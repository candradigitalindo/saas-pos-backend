package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"candra/backend-api/config"
	"candra/backend-api/helpers"
	"candra/backend-api/internal/reqctx"
	"candra/backend-api/models"
	"candra/backend-api/repositories"
	"candra/backend-api/structs"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// Layanan langganan & tagihan platform (Fase 7, §5.13, §13.4, blueprint
// "Diskon langganan dibayar di muka").
//
// Empat aturan yang wajib ada di kode:
//  1. Pendapatan prabayar diakui BULANAN (deferred_revenue_entries), bukan
//     sekaligus saat uang masuk.
//  2. Komisi mitra bertahap per bulan — Fase 12; di sini `paid_amount` tagihan
//     dijaga = uang yang benar-benar jadi hak platform agar dasar komisi benar.
//  3. Pembatalan di tengah masa dihitung ULANG pada harga bulanan NORMAL, baru
//     sisanya dikembalikan.
//  4. Naik paket di tengah masa memakai prorata; sisa nilai jadi kredit.

const idempotencyScopeSubPayment = "subscription.payment"

func subscriptionTrialDays() int { return config.GetIntEnv("SUBSCRIPTION_TRIAL_DAYS", 14) }
func subscriptionDueDays() int   { return config.GetIntEnv("SUBSCRIPTION_INVOICE_DUE_DAYS", 7) }

// validTerm memastikan masa langganan salah satu dari 1/3/6/9/12 bulan.
func validTerm(term int) bool {
	for _, t := range models.SubscriptionTerms {
		if t == term {
			return true
		}
	}
	return false
}

// firstOfMonth mengembalikan tanggal 1 (UTC) dari bulan t.
func firstOfMonth(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// wholeMonthsBetween menghitung jumlah BULAN PENUH yang telah berlalu dari start
// ke end (mis. 5 Jan → 5 Jun = 5; 5 Jan → 4 Jun = 4). Minimal 0.
func wholeMonthsBetween(start, end time.Time) int {
	m := (end.Year()-start.Year())*12 + int(end.Month()) - int(start.Month())
	if end.Day() < start.Day() {
		m--
	}
	if m < 0 {
		m = 0
	}
	return m
}

// StartSubscription memulai (atau mengaktifkan ulang) langganan tenant dalam
// status trial. Idempoten terhadap UNIQUE(tenant_id): langganan yang masih hidup
// menolak; yang sudah canceled/expired diaktifkan ulang.
func StartSubscription(ctx context.Context, planCode string, term int) (structs.SubscriptionResponse, error) {
	var out structs.SubscriptionResponse
	if !validTerm(term) {
		return out, fmt.Errorf("%w: masa langganan harus 1, 3, 6, 9, atau 12 bulan", helpers.ErrValidation)
	}

	err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		plan, err := repositories.FindPlanByCode(ctx, tx, planCode)
		if err != nil {
			return err
		}
		rate, err := repositories.TermDiscountRate(ctx, tx, term)
		if err != nil {
			return err
		}

		now := time.Now().UTC()
		trialEnds := now.AddDate(0, 0, subscriptionTrialDays())

		existing, ferr := repositories.FindSubscriptionByTenant(ctx, tx)
		switch {
		case ferr == nil:
			if existing.Status == "trial" || existing.Status == "active" || existing.Status == "past_due" {
				return fmt.Errorf("%w: tenant sudah punya langganan berjalan", helpers.ErrConflict)
			}
			existing.PlanID = plan.ID
			existing.TermMonths = term
			existing.DiscountRate = rate
			existing.Status = "trial"
			existing.TrialEndsAt = &trialEnds
			existing.CurrentPeriodStart = now
			existing.CurrentPeriodEnd = trialEnds
			existing.AutoRenew = true
			existing.CanceledAt = nil
			existing.CancelReason = ""
			if err := repositories.SaveSubscription(ctx, tx, &existing); err != nil {
				return err
			}
		case ferr == repositories.ErrSubscriptionNotFound:
			sub := models.Subscription{
				PlanID:             plan.ID,
				TermMonths:         term,
				DiscountRate:       rate,
				Status:             "trial",
				TrialEndsAt:        &trialEnds,
				CurrentPeriodStart: now,
				CurrentPeriodEnd:   trialEnds,
				AutoRenew:          true,
			}
			if err := repositories.CreateSubscription(ctx, tx, &sub); err != nil {
				return err
			}
		default:
			return ferr
		}

		sub, err := repositories.FindSubscriptionByTenant(ctx, tx)
		if err != nil {
			return err
		}
		out = subscriptionToResponse(sub)
		return nil
	})
	return out, err
}

// GenerateInvoice menerbitkan tagihan untuk paket + masa langganan berjalan.
// Menolak bila masih ada tagihan yang belum lunas.
func GenerateInvoice(ctx context.Context) (structs.SubInvoiceResponse, error) {
	var out structs.SubInvoiceResponse
	err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		sub, err := repositories.FindSubscriptionByTenant(ctx, tx)
		if err != nil {
			return err
		}
		if _, open, err := repositories.OpenSubInvoiceForTenant(ctx, tx); err != nil {
			return err
		} else if open {
			return fmt.Errorf("%w: masih ada tagihan yang belum dibayar", helpers.ErrConflict)
		}

		plan, err := repositories.FindPlanByID(ctx, tx, sub.PlanID)
		if err != nil {
			return err
		}

		inv, err := buildInvoice(ctx, tx, sub, plan, sub.TermMonths, sub.DiscountRate, 0)
		if err != nil {
			return err
		}
		if err := repositories.CreateSubInvoice(ctx, tx, &inv); err != nil {
			return err
		}

		// Pemberitahuan tagihan — pola OUTBOX (§5.14): peristiwanya ditulis di
		// dalam transaksi yang SAMA dengan tagihannya. Kalau transaksi ini batal,
		// pemberitahuannya ikut batal; kalau berhasil, pengirimannya dijamin
		// tersimpan walau penyedia notifikasi sedang mati. Pengiriman sungguhan
		// dilakukan cmd/process-outbox belakangan.
		tid := reqctx.TenantID(ctx)
		var tenant models.Tenant
		penerima := ""
		if err := repositories.FindTenantByID(ctx, tx, tid, &tenant); err == nil {
			penerima = tenant.Phone
		}
		if penerima != "" {
			_ = EnqueueNotification(ctx, tx, &tid, "invoice.issued", map[string]any{
				"channel":     "whatsapp",
				"to":          penerima,
				"nama_usaha":  tenant.BusinessName,
				"nomor":       inv.Number,
				"total":       helpers.FormatRupiah(inv.TotalAmount),
				"jatuh_tempo": inv.DueDate.Format("02 Jan 2006"),
			})
		}

		out = subInvoiceToResponse(inv)
		return nil
	})
	return out, err
}

// buildInvoice merakit sebuah SubscriptionInvoice (belum disimpan). `extraCredit`
// adalah potongan tambahan (kredit prorata saat ganti paket), diterapkan setelah
// diskon masa langganan.
func buildInvoice(ctx context.Context, tx *gorm.DB, sub models.Subscription, plan models.Plan, term int, rate decimal.Decimal, extraCredit int64) (models.SubscriptionInvoice, error) {
	now := time.Now().UTC()
	today := firstOfDay(now)

	periodStart := today
	if sub.Status == "active" && sub.CurrentPeriodEnd.After(now) {
		periodStart = firstOfDay(sub.CurrentPeriodEnd)
	}
	periodEnd := periodStart.AddDate(0, term, 0)

	gross := plan.MonthlyPrice * int64(term)
	discount := helpers.RoundHalfUpToInt(decimal.NewFromInt(gross).Mul(rate)) + extraCredit
	if discount > gross {
		discount = gross
	}
	total := gross - discount

	number, err := repositories.NextSubInvoiceNumber(ctx, tx)
	if err != nil {
		return models.SubscriptionInvoice{}, err
	}
	return models.SubscriptionInvoice{
		SubscriptionID: sub.ID,
		Number:         number,
		TermMonths:     term,
		PeriodStart:    periodStart,
		PeriodEnd:      periodEnd,
		GrossAmount:    gross,
		DiscountAmount: discount,
		TotalAmount:    total,
		DueDate:        today.AddDate(0, 0, subscriptionDueDays()),
		Status:         "open",
	}, nil
}

// firstOfDay memangkas t ke tengah malam UTC.
func firstOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// PaySubInput adalah masukan pembayaran tagihan.
type PaySubInput struct {
	InvoiceID      string
	Amount         int64
	Method         string
	Reference      string
	IdempotencyKey string
	RequestHash    string
}

// PaySubscriptionInvoice mencatat pembayaran tagihan langganan. Idempoten lewat
// Idempotency-Key. Saat tagihan LUNAS: langganan menjadi 'active', periode maju,
// dan N baris deferred_revenue_entries dibuat (§13.4).
//
// Mengembalikan (status HTTP, body JSON, error).
func PaySubscriptionInvoice(ctx context.Context, in PaySubInput) (int, []byte, error) {
	if in.IdempotencyKey == "" {
		return 0, nil, fmt.Errorf("%w: header Idempotency-Key wajib", helpers.ErrValidation)
	}
	if in.Amount <= 0 {
		return 0, nil, fmt.Errorf("%w: nominal pembayaran harus > 0", helpers.ErrValidation)
	}

	var (
		outStatus int
		outBody   []byte
	)
	txErr := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		m, err := repositories.LookupIdempotency(ctx, tx, idempotencyScopeSubPayment, in.IdempotencyKey, in.RequestHash)
		if err != nil {
			return err
		}
		if m.Found {
			if !m.SameRequest {
				return fmt.Errorf("%w: Idempotency-Key sudah dipakai untuk permintaan berbeda", helpers.ErrConflict)
			}
			outStatus, outBody = m.ResponseStatus, m.ResponseBody
			return nil
		}

		inv, err := repositories.FindSubInvoiceForTenant(ctx, tx, in.InvoiceID)
		if err != nil {
			return err
		}
		if inv.Status != "open" && inv.Status != "overdue" {
			return fmt.Errorf("%w: tagihan tidak dalam status yang bisa dibayar", helpers.ErrConflict)
		}
		remaining := inv.TotalAmount - inv.PaidAmount
		if in.Amount > remaining {
			return fmt.Errorf("%w: pembayaran melebihi sisa tagihan (%d)", helpers.ErrValidation, remaining)
		}

		now := time.Now().UTC()
		if err := repositories.CreateSubPayment(ctx, tx, &models.SubscriptionPayment{
			SubscriptionInvoiceID: inv.ID,
			Amount:                in.Amount,
			Method:                in.Method,
			Reference:             in.Reference,
			PaidAt:                now,
		}); err != nil {
			return err
		}

		inv.PaidAmount += in.Amount
		if inv.PaidAmount >= inv.TotalAmount {
			inv.Status = "paid"
			inv.PaidAt = &now

			sub, err := repositories.FindSubscriptionByTenant(ctx, tx)
			if err != nil {
				return err
			}
			sub.Status = "active"
			sub.CurrentPeriodStart = inv.PeriodStart
			sub.CurrentPeriodEnd = inv.PeriodEnd
			if err := repositories.SaveSubscription(ctx, tx, &sub); err != nil {
				return err
			}
			if err := repositories.CreateDeferredEntries(ctx, tx, deferredEntriesFor(inv)); err != nil {
				return err
			}
			if err := setTenantStatus(ctx, tx, "active"); err != nil {
				return err
			}
		}
		if err := repositories.SaveSubInvoice(ctx, tx, &inv); err != nil {
			return err
		}

		body, err := json.Marshal(structs.SuccessResponse[structs.SubInvoiceResponse]{
			Success: true, Message: "Pembayaran diterima", Data: subInvoiceToResponse(inv),
		})
		if err != nil {
			return err
		}
		if err := repositories.SaveIdempotency(ctx, tx, idempotencyScopeSubPayment, in.IdempotencyKey, in.RequestHash,
			http.StatusCreated, body, idempotencyTTL()); err != nil {
			return err
		}
		outStatus, outBody = http.StatusCreated, body
		return nil
	})
	if txErr != nil {
		return 0, nil, txErr
	}
	return outStatus, outBody, nil
}

// deferredEntriesFor memecah total tagihan menjadi N baris pengakuan bulanan
// (§13.4): masing-masing total/N, sisa pembulatan ditaruh di bulan terakhir agar
// jumlahnya persis.
func deferredEntriesFor(inv models.SubscriptionInvoice) []models.DeferredRevenueEntry {
	n := inv.TermMonths
	if n < 1 {
		n = 1
	}
	per := inv.TotalAmount / int64(n)
	remainder := inv.TotalAmount - per*int64(n)
	month0 := firstOfMonth(inv.PeriodStart)

	rows := make([]models.DeferredRevenueEntry, 0, n)
	for i := 0; i < n; i++ {
		amt := per
		if i == n-1 {
			amt += remainder
		}
		rows = append(rows, models.DeferredRevenueEntry{
			SubscriptionInvoiceID: inv.ID,
			RecognitionMonth:      month0.AddDate(0, i, 0),
			Amount:                amt,
		})
	}
	return rows
}

// CancelSubscription membatalkan langganan di tengah masa. Refund dihitung pada
// harga bulanan NORMAL (blueprint aturan 3):
//
//	refund = maks(0, dibayar − bulan_terpakai × harga_bulanan_normal)
//
// Pengakuan pendapatan disamakan agar total yang diakui = dibayar − refund.
func CancelSubscription(ctx context.Context, reason string) (structs.SubscriptionCancelResponse, error) {
	var out structs.SubscriptionCancelResponse
	err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		sub, err := repositories.FindSubscriptionByTenant(ctx, tx)
		if err != nil {
			return err
		}
		if sub.Status == "canceled" || sub.Status == "expired" {
			return fmt.Errorf("%w: langganan sudah berakhir", helpers.ErrConflict)
		}
		now := time.Now().UTC()

		var refund, earned int64
		var monthsUsed int

		inv, ierr := repositories.LatestPaidSubInvoice(ctx, tx)
		if ierr == nil {
			plan, err := repositories.FindPlanByID(ctx, tx, sub.PlanID)
			if err != nil {
				return err
			}
			monthsUsed = wholeMonthsBetween(inv.PeriodStart, now)
			if monthsUsed < 1 {
				monthsUsed = 1
			}
			if monthsUsed > inv.TermMonths {
				monthsUsed = inv.TermMonths
			}
			refund = inv.PaidAmount - int64(monthsUsed)*plan.MonthlyPrice
			if refund < 0 {
				refund = 0
			}
			earned = inv.PaidAmount - refund

			if err := settleInvoiceDeferred(ctx, tx, inv.ID, earned, now); err != nil {
				return err
			}
			inv.PaidAmount = earned
			inv.Status = "refunded"
			if err := repositories.SaveSubInvoice(ctx, tx, &inv); err != nil {
				return err
			}
			if err := repositories.CreateSubRefund(ctx, tx, &models.SubscriptionRefund{
				SubscriptionInvoiceID: inv.ID,
				Amount:                refund,
				MonthsUsed:            monthsUsed,
				Reason:                reason,
				RefundedAt:            now,
			}); err != nil {
				return err
			}
		} else if ierr != repositories.ErrSubInvoiceNotFound {
			return ierr
		}

		sub.Status = "canceled"
		sub.CanceledAt = &now
		sub.CancelReason = reason
		sub.AutoRenew = false
		if err := repositories.SaveSubscription(ctx, tx, &sub); err != nil {
			return err
		}
		if err := setTenantStatus(ctx, tx, "closed"); err != nil {
			return err
		}

		reloaded, err := repositories.FindSubscriptionByTenant(ctx, tx)
		if err != nil {
			return err
		}
		out = structs.SubscriptionCancelResponse{
			RefundAmount: refund, EarnedAmount: earned, MonthsUsed: monthsUsed,
			Subscription: subscriptionToResponse(reloaded),
		}
		return nil
	})
	return out, err
}

// ChangePlan menerbitkan tagihan paket baru dengan KREDIT prorata atas sisa masa
// paket lama (dihitung pada harga bulanan normal), lalu mengarahkan langganan ke
// paket baru. Berlaku setelah tagihan barunya dibayar.
func ChangePlan(ctx context.Context, planCode string, term int) (structs.SubInvoiceResponse, error) {
	var out structs.SubInvoiceResponse
	if !validTerm(term) {
		return out, fmt.Errorf("%w: masa langganan harus 1, 3, 6, 9, atau 12 bulan", helpers.ErrValidation)
	}
	err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		sub, err := repositories.FindSubscriptionByTenant(ctx, tx)
		if err != nil {
			return err
		}
		if sub.Status != "active" {
			return fmt.Errorf("%w: hanya langganan aktif yang bisa ganti paket di tengah masa", helpers.ErrConflict)
		}
		if _, open, err := repositories.OpenSubInvoiceForTenant(ctx, tx); err != nil {
			return err
		} else if open {
			return fmt.Errorf("%w: selesaikan tagihan yang belum dibayar dulu", helpers.ErrConflict)
		}

		newPlan, err := repositories.FindPlanByCode(ctx, tx, planCode)
		if err != nil {
			return err
		}
		oldPlan, err := repositories.FindPlanByID(ctx, tx, sub.PlanID)
		if err != nil {
			return err
		}
		newRate, err := repositories.TermDiscountRate(ctx, tx, term)
		if err != nil {
			return err
		}

		now := time.Now().UTC()
		var credit int64
		if paidInv, ierr := repositories.LatestPaidSubInvoice(ctx, tx); ierr == nil {
			used := wholeMonthsBetween(paidInv.PeriodStart, now)
			remaining := paidInv.TermMonths - used
			if remaining < 0 {
				remaining = 0
			}
			credit = int64(remaining) * oldPlan.MonthlyPrice
			earnedOld := paidInv.PaidAmount - credit
			if earnedOld < 0 {
				earnedOld = 0
			}
			if err := settleInvoiceDeferred(ctx, tx, paidInv.ID, earnedOld, now); err != nil {
				return err
			}
		} else if ierr != repositories.ErrSubInvoiceNotFound {
			return ierr
		}

		inv, err := buildInvoice(ctx, tx, sub, newPlan, term, newRate, credit)
		if err != nil {
			return err
		}
		if err := repositories.CreateSubInvoice(ctx, tx, &inv); err != nil {
			return err
		}

		sub.PlanID = newPlan.ID
		sub.TermMonths = term
		sub.DiscountRate = newRate
		if err := repositories.SaveSubscription(ctx, tx, &sub); err != nil {
			return err
		}
		out = subInvoiceToResponse(inv)
		return nil
	})
	return out, err
}

// settleInvoiceDeferred menghapus pengakuan yang belum diakui untuk sebuah
// tagihan lalu menulis satu baris penyesuaian agar total yang diakui persis =
// `earned`.
func settleInvoiceDeferred(ctx context.Context, tx *gorm.DB, invoiceID string, earned int64, now time.Time) error {
	if _, err := repositories.DeleteUnrecognizedDeferred(ctx, tx, invoiceID); err != nil {
		return err
	}
	recognized, err := repositories.SumRecognizedDeferred(ctx, tx, invoiceID)
	if err != nil {
		return err
	}
	if adjust := earned - recognized; adjust != 0 {
		return repositories.UpsertDeferredAdjustment(ctx, tx, invoiceID, firstOfMonth(now), adjust)
	}
	return nil
}

// setTenantStatus memperbarui kolom status di baris tenants konteks.
func setTenantStatus(ctx context.Context, tx *gorm.DB, status string) error {
	return tx.WithContext(ctx).Exec(
		"UPDATE tenants SET status = ?, updated_at = now() WHERE id = ?",
		status, repositories.TenantIDFromCtx(ctx),
	).Error
}

// GetSubscriptionOverview mengembalikan langganan + tagihan terbuka bila ada.
func GetSubscriptionOverview(ctx context.Context) (structs.SubscriptionOverviewResponse, error) {
	var out structs.SubscriptionOverviewResponse
	sub, err := repositories.FindSubscriptionByTenant(ctx, nil)
	if err != nil {
		return out, err
	}
	out.Subscription = subscriptionToResponse(sub)
	if inv, open, err := repositories.OpenSubInvoiceForTenant(ctx, nil); err != nil {
		return out, err
	} else if open {
		r := subInvoiceToResponse(inv)
		out.OpenInvoice = &r
	}
	return out, nil
}

// ListPlans mengembalikan katalog paket dengan tabel harga per masa langganan.
func ListPlans(ctx context.Context) ([]structs.PlanResponse, error) {
	plans, err := repositories.ListActivePlans(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]structs.PlanResponse, 0, len(plans))
	for _, p := range plans {
		pr := structs.PlanResponse{
			Code: p.Code, Name: p.Name, MonthlyPrice: p.MonthlyPrice,
			MaxOutlets: p.MaxOutlets, MaxUsers: p.MaxUsers, MaxProducts: p.MaxProducts,
			MaxMonthlyTransactions: p.MaxMonthlyTransactions, Features: p.Features,
		}
		for _, term := range models.SubscriptionTerms {
			rate, err := repositories.TermDiscountRate(ctx, nil, term)
			if err != nil {
				return nil, err
			}
			gross := p.MonthlyPrice * int64(term)
			disc := helpers.RoundHalfUpToInt(decimal.NewFromInt(gross).Mul(rate))
			pr.TermPrices = append(pr.TermPrices, structs.PlanTermPrice{
				TermMonths: term, DiscountRate: rate.String(),
				GrossAmount: gross, DiscountAmount: disc, TotalAmount: gross - disc,
			})
		}
		out = append(out, pr)
	}
	return out, nil
}

// RecognizeDueRevenue mengakui semua pendapatan diterima di muka yang bulannya
// sudah tiba (§13.4). GLOBAL — dipanggil pekerjaan harian (cmd/recognize-revenue).
func RecognizeDueRevenue(ctx context.Context) (int64, error) {
	return repositories.RecognizeDueRevenue(ctx)
}

// ── Pemetaan DTO ──────────────────────────────────────────────────────────

func subscriptionToResponse(s models.Subscription) structs.SubscriptionResponse {
	r := structs.SubscriptionResponse{
		ID:                 s.ID,
		TermMonths:         s.TermMonths,
		DiscountRate:       s.DiscountRate.String(),
		Status:             s.Status,
		CurrentPeriodStart: s.CurrentPeriodStart.UTC().Format(saleTimeLayout),
		CurrentPeriodEnd:   s.CurrentPeriodEnd.UTC().Format(saleTimeLayout),
		AutoRenew:          s.AutoRenew,
		CancelReason:       s.CancelReason,
	}
	if s.Plan != nil {
		r.PlanCode = s.Plan.Code
		r.PlanName = s.Plan.Name
	}
	if s.TrialEndsAt != nil {
		r.TrialEndsAt = s.TrialEndsAt.UTC().Format(saleTimeLayout)
	}
	if s.CanceledAt != nil {
		r.CanceledAt = s.CanceledAt.UTC().Format(saleTimeLayout)
	}
	return r
}

// SubInvoiceToResponse memetakan model tagihan ke DTO (diekspor untuk controller
// daftar tagihan).
func SubInvoiceToResponse(i models.SubscriptionInvoice) structs.SubInvoiceResponse {
	return subInvoiceToResponse(i)
}

func subInvoiceToResponse(i models.SubscriptionInvoice) structs.SubInvoiceResponse {
	r := structs.SubInvoiceResponse{
		ID: i.ID, Number: i.Number, TermMonths: i.TermMonths,
		PeriodStart: i.PeriodStart.Format("2006-01-02"),
		PeriodEnd:   i.PeriodEnd.Format("2006-01-02"),
		GrossAmount: i.GrossAmount, DiscountAmount: i.DiscountAmount,
		TotalAmount: i.TotalAmount, PaidAmount: i.PaidAmount,
		DueDate: i.DueDate.Format("2006-01-02"), Status: i.Status,
	}
	if i.PaidAt != nil {
		r.PaidAt = i.PaidAt.UTC().Format(saleTimeLayout)
	}
	return r
}
