package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"candra/backend-api/helpers"
	"candra/backend-api/models"
	"candra/backend-api/repositories"
	"candra/backend-api/structs"

	"gorm.io/gorm"
)

// Mesin komisi & pencairan Program Mitra (Fase 12, §5.12, blueprint G.2/G.6).
//
// Enam aturan yang saling mengunci (blueprint G.2):
//  1. Komisi BERULANG selama merchant berlangganan (`tier.recurring_months` nil);
//     tier berjangka berhenti setelah N bulan sejak aktivasi.
//  2. Dihitung dari `subscription_invoices.paid_amount` — uang yang BENAR-BENAR
//     diterima, bukan tagihan yang diterbitkan.
//  3. Ambang aktivasi: merchant harus dipakai nyata (≥ tier.activation_min_txn
//     transaksi selesai, ATAU ≥ tier.activation_min_days sejak daftar) sebelum
//     komisi pertama.
//  4. Clawback: langganan berhenti dalam `tier.clawback_days` sejak aktivasi →
//     komisi 'held'/'approved' referral itu ditarik; yang sudah 'paid'
//     dikurangkan di pencairan berikutnya.
//  5. Masa atribusi diputuskan saat pendaftaran (baris partner_referrals ada =
//     konversi tepat waktu). Sengketa diputus admin lewat partner_disputes.
//  6. Pajak: potongan `partner.tax_withholding_rate` dipisah di pencairan.
//
// Deterministik: hanya baris komisi 'held' yang dihitung ulang (UPSERT dengan
// WHERE status='held'); 'approved'/'paid'/'clawed_back'/'canceled' tak tersentuh
// — sepadan dengan mesin gaji Fase 13. GLOBAL (lintas tenant).

// ComputePartnerCommissions menjalankan satu putaran perhitungan komisi untuk
// pembayaran langganan yang diterima pada rentang [from, to] (YYYY-MM-DD).
// partnerID kosong = semua mitra (cron bulanan); diisi = satu mitra saja.
func ComputePartnerCommissions(ctx context.Context, partnerID, from, to string) (structs.PartnerCommissionRunResult, error) {
	res := structs.PartnerCommissionRunResult{From: from, To: to}
	if _, err := time.Parse("2006-01-02", from); err != nil {
		return res, fmt.Errorf("%w: from harus YYYY-MM-DD", helpers.ErrValidation)
	}
	if _, err := time.Parse("2006-01-02", to); err != nil {
		return res, fmt.Errorf("%w: to harus YYYY-MM-DD", helpers.ErrValidation)
	}

	refs, err := repositories.ListLiveReferrals(ctx, partnerID)
	if err != nil {
		return res, err
	}
	res.ReferralsSeen = len(refs)
	if len(refs) == 0 {
		return res, nil
	}

	// Muat semua mitra + tingkatnya SEKALI (hindari N+1 pada ribuan referral).
	pids := make([]string, 0, len(refs))
	seen := map[string]bool{}
	for _, r := range refs {
		if !seen[r.PartnerID] {
			seen[r.PartnerID] = true
			pids = append(pids, r.PartnerID)
		}
	}
	partners, err := repositories.PartnersByIDs(ctx, pids)
	if err != nil {
		return res, err
	}

	now := time.Now().UTC()
	for _, ref := range refs {
		partner, ok := partners[ref.PartnerID]
		if !ok || partner.Status != "active" || partner.Tier == nil {
			continue
		}
		tier := *partner.Tier

		// (3) Aktivasi.
		activatedAt := ref.ActivatedAt
		if activatedAt == nil {
			ready, err := merchantActivated(ctx, ref.TenantID, tier, now)
			if err != nil {
				return res, err
			}
			if !ready {
				res.SkippedNoActivation++
				continue
			}
			endsAt := commissionEndsAt(now, tier)
			if err := repositories.ActivateReferral(ctx, ref.ID, now, endsAt); err != nil {
				return res, err
			}
			if ref.LeadID != nil {
				_ = repositories.MarkLeadActivated(ctx, *ref.LeadID)
			}
			t := now
			activatedAt = &t
			ref.CommissionEndsAt = endsAt
			res.Activated++
		}

		// (4) Clawback: langganan berhenti dalam masa clawback sejak aktivasi.
		clawed, err := maybeClawback(ctx, ref, tier, *activatedAt)
		if err != nil {
			return res, err
		}
		if clawed {
			res.ClawedBack++
			continue
		}

		// (1) Jendela efektif: tidak sebelum aktivasi; tier berjangka dibatasi.
		effFrom := from
		if a := activatedAt.Format("2006-01-02"); a > effFrom {
			effFrom = a
		}
		effTo := to
		if ref.CommissionEndsAt != nil {
			if e := ref.CommissionEndsAt.Format("2006-01-02"); e < effTo {
				effTo = e
			}
		}
		if effFrom > effTo {
			continue
		}

		// (2) Faktur langganan DIBAYAR pada jendela → satu komisi per faktur.
		invoices, err := repositories.PaidSubInvoicesForTenant(ctx, ref.TenantID, effFrom, effTo)
		if err != nil {
			return res, err
		}
		for _, inv := range invoices {
			if inv.PaidAmount <= 0 {
				continue
			}
			paidAt := now
			if inv.PaidAt != nil {
				paidAt = *inv.PaidAt
			}
			c := models.PartnerCommission{
				PartnerID:             partner.ID,
				ReferralID:            ref.ID,
				SubscriptionInvoiceID: inv.ID,
				PeriodMonth:           firstOfMonth(paidAt),
				BaseAmount:            inv.PaidAmount,
				Rate:                  tier.RecurringRate,
				Amount:                helpers.ApplyRate(inv.PaidAmount, tier.RecurringRate),
				Status:                "held",
			}
			if err := repositories.UpsertHeldCommission(ctx, &c); err != nil {
				return res, err
			}
			res.Computed++
		}
	}
	return res, nil
}

// commissionEndsAt menghitung akhir masa komisi untuk tier berjangka.
// nil = selama merchant masih berlangganan (tier.recurring_months NULL).
func commissionEndsAt(activatedAt time.Time, tier models.PartnerTier) *time.Time {
	if tier.RecurringMonths == nil || *tier.RecurringMonths <= 0 {
		return nil
	}
	end := activatedAt.AddDate(0, *tier.RecurringMonths, 0)
	return &end
}

// merchantActivated melapor apakah merchant sudah memenuhi ambang aktivasi
// (blueprint G.2 #3): cukup salah satu — transaksi nyata ATAU umur berlangganan.
func merchantActivated(ctx context.Context, tenantID string, tier models.PartnerTier, now time.Time) (bool, error) {
	txns, err := repositories.CountCompletedSalesForTenant(ctx, tenantID)
	if err != nil {
		return false, err
	}
	if txns >= int64(tier.ActivationMinTxn) {
		return true, nil
	}
	var t models.Tenant
	if err := repositories.FindTenantByID(ctx, nil, tenantID, &t); err != nil {
		return false, err
	}
	daysActive := int(now.Sub(t.CreatedAt).Hours() / 24)
	return daysActive >= tier.ActivationMinDays, nil
}

// maybeClawback menarik komisi bila langganan merchant berhenti dalam masa
// clawback sejak aktivasi. Mengembalikan true bila clawback dilakukan.
func maybeClawback(ctx context.Context, ref models.PartnerReferral, tier models.PartnerTier, activatedAt time.Time) (bool, error) {
	sub, ok, err := repositories.SubscriptionStateForTenant(ctx, ref.TenantID)
	if err != nil {
		return false, err
	}
	if !ok || sub.Status != "canceled" || sub.CanceledAt == nil {
		return false, nil
	}
	if sub.CanceledAt.After(activatedAt.AddDate(0, 0, tier.ClawbackDays)) {
		// Berhenti di luar masa clawback — komisi tetap, atribusi ditutup.
		return false, repositories.SetReferralStatus(ctx, ref.ID, "ended")
	}
	if _, err := repositories.ClawbackReferralCommissions(ctx, ref.ID); err != nil {
		return false, err
	}
	if err := repositories.SetReferralStatus(ctx, ref.ID, "revoked"); err != nil {
		return false, err
	}
	return true, nil
}

// ── Persetujuan & pencairan ────────────────────────────────────────────

// ApprovePartnerCommission memindahkan satu komisi 'held' → 'approved' (tinjauan
// admin sebelum pembayaran, blueprint G.6 langkah 9).
func ApprovePartnerCommission(ctx context.Context, id string) error {
	return repositories.SetCommissionStatus(ctx, id, "approved")
}

// ApproveHeldCommissionsForPartner menyetujui SEMUA komisi 'held' seorang mitra
// pada rentang periode — dipakai CLI pencairan massal.
func ApproveHeldCommissionsForPartner(ctx context.Context, partnerID, from, to string) (int, error) {
	held, err := repositories.ListPartnerCommissions(ctx, partnerID, "held", from, to)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, c := range held {
		if err := repositories.SetCommissionStatus(ctx, c.ID, "approved"); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

// CreatePartnerPayout menagih SEMUA komisi 'approved' seorang mitra pada rentang
// periode menjadi satu pencairan: bruto − clawback − pajak = neto. Komisi yang
// diambil ditandai 'paid' + `payout_id`.
func CreatePartnerPayout(ctx context.Context, partnerID, from, to string) (structs.PartnerPayoutResponse, error) {
	var out structs.PartnerPayoutResponse
	ps, err := time.Parse("2006-01-02", from)
	if err != nil {
		return out, fmt.Errorf("%w: from harus YYYY-MM-DD", helpers.ErrValidation)
	}
	pe, err := time.Parse("2006-01-02", to)
	if err != nil {
		return out, fmt.Errorf("%w: to harus YYYY-MM-DD", helpers.ErrValidation)
	}

	partner, err := repositories.FindPartnerByID(ctx, partnerID)
	if err != nil {
		return out, err
	}
	commissions, err := repositories.ListApprovedCommissionsForPayout(ctx, partnerID, from, to)
	if err != nil {
		return out, err
	}
	if len(commissions) == 0 {
		return out, fmt.Errorf("%w: tak ada komisi disetujui pada periode itu", helpers.ErrValidation)
	}

	var gross int64
	ids := make([]string, 0, len(commissions))
	for _, c := range commissions {
		gross += c.Amount
		ids = append(ids, c.ID)
	}

	// Clawback komisi yang sudah dibayar pada referral yang atribusinya dicabut.
	revokedRefs, err := repositories.RevokedReferralIDsForPartner(ctx, partnerID)
	if err != nil {
		return out, err
	}
	clawback, err := repositories.PaidCommissionsToClawback(ctx, revokedRefs)
	if err != nil {
		return out, err
	}

	taxable := gross - clawback
	if taxable < 0 {
		taxable = 0
	}
	tax := helpers.ApplyRate(taxable, partner.TaxWithholdingRate)
	net := taxable - tax

	payout := models.PartnerPayout{
		PartnerID: partnerID, PeriodStart: ps, PeriodEnd: pe,
		GrossAmount: gross, ClawbackAmount: clawback, TaxAmount: tax, NetAmount: net,
		Status: "draft",
	}
	err = repositories.Transaction(ctx, func(tx *gorm.DB) error {
		if e := repositories.CreatePartnerPayout(ctx, tx, &payout); e != nil {
			if helpers.IsDuplicateEntryError(e) {
				return fmt.Errorf("%w: pencairan periode itu sudah ada", helpers.ErrConflict)
			}
			return e
		}
		if e := repositories.AssignCommissionsToPayout(ctx, tx, payout.ID, ids); e != nil {
			return e
		}
		// Tutup komisi 'paid' yang barusan dikurangkan agar tidak dihitung dua kali.
		return repositories.MarkPaidCommissionsClawedBack(ctx, tx, revokedRefs)
	})
	if err != nil {
		return out, err
	}
	return payoutToResponse(payout), nil
}

// MarkPartnerPayoutPaid menandai pencairan → 'paid' dengan bukti transfer dan
// bukti potong pajak (blueprint G.4).
func MarkPartnerPayoutPaid(ctx context.Context, id, proofURL, taxSlipURL string) error {
	return repositories.MarkPartnerPayoutPaid(ctx, id,
		strings.TrimSpace(proofURL), strings.TrimSpace(taxSlipURL))
}

// PartnerCycleResult merangkum satu putaran cmd/partner-commissions.
type PartnerCycleResult struct {
	Compute  structs.PartnerCommissionRunResult
	Approved int
	Payouts  int
}

// RunPartnerCommissionCycle menjalankan siklus komisi periodik (blueprint G.6
// langkah 9): hitung → (opsional) setujui semua 'held' → (opsional) buat
// pencairan draft per mitra aktif. GLOBAL. Dipakai cmd/partner-commissions.
func RunPartnerCommissionCycle(ctx context.Context, from, to string, approve, payout bool) (PartnerCycleResult, error) {
	var out PartnerCycleResult
	run, err := ComputePartnerCommissions(ctx, "", from, to)
	if err != nil {
		return out, err
	}
	out.Compute = run

	if !approve && !payout {
		return out, nil
	}

	partners, _, err := repositories.ListPartners(ctx, "active", 10000, 0)
	if err != nil {
		return out, err
	}
	for _, p := range partners {
		if approve {
			n, err := ApproveHeldCommissionsForPartner(ctx, p.ID, from, to)
			if err != nil {
				return out, err
			}
			out.Approved += n
		}
		if payout {
			_, err := CreatePartnerPayout(ctx, p.ID, from, to)
			if err != nil {
				// Tak ada komisi disetujui / pencairan periode itu sudah ada —
				// bukan kegagalan siklus; lewati mitra ini.
				if errors.Is(err, helpers.ErrValidation) || errors.Is(err, helpers.ErrConflict) {
					continue
				}
				return out, err
			}
			out.Payouts++
		}
	}
	return out, nil
}
