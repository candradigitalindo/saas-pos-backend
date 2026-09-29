package services

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"candra/backend-api/config"
	"candra/backend-api/helpers"
	"candra/backend-api/internal/reqctx"
	"candra/backend-api/models"
	"candra/backend-api/repositories"

	"gorm.io/gorm"
)

// Pekerjaan harian perpanjangan & pengingat langganan (cmd/subscription-renewals).
//
// Sampai kini tidak ada yang menerbitkan tagihan perpanjangan atau mengingatkan
// apa pun: langganan berbayar diam-diam lewat masa tenggang lalu terkunci, dan
// pemilik baru tahu saat QRIS di kasir menolak. "Diperpanjang otomatis" di layar
// Langganan tidak ada yang menjalankan.
//
// Setiap langkah idempoten, jadi aman dijalankan berkali-kali sehari atau dua
// instans bersamaan: tagihan diterbitkan lewat GenerateInvoice (mengunci baris
// langganan, menolak bila sudah ada tagihan terbuka), dan setiap pengingat
// dicatat di subscription_notices dengan UNIQUE (tenant, jenis, ref) — di
// transaksi yang SAMA dengan pesannya di outbox.

func subscriptionRenewalLeadDays() int { return config.GetIntEnv("SUBSCRIPTION_RENEWAL_LEAD_DAYS", 7) }
func subscriptionTrialReminderDays() int {
	return config.GetIntEnv("SUBSCRIPTION_TRIAL_REMINDER_DAYS", 3)
}
func subscriptionGraceReminderDays() int {
	return config.GetIntEnv("SUBSCRIPTION_GRACE_REMINDER_DAYS", 2)
}

// RenewalOptions: TenantID kosong = semua tenant; diisi = satu tenant saja.
type RenewalOptions struct {
	TenantID string
}

// RenewalReport merangkum satu putaran pekerjaan harian.
type RenewalReport struct {
	Issued           int // tagihan perpanjangan yang diterbitkan
	Overdue          int // tagihan yang ditandai lewat jatuh tempo
	PastDue          int // langganan yang ditandai past_due
	TrialReminders   int
	OverdueReminders int
	GraceReminders   int
	Failed           int // tenant yang langkahnya gagal (lihat log); tenant lain tetap diproses
}

// RunSubscriptionRenewals menjalankan satu putaran. Kegagalan satu tenant
// dicatat dan dilewati — satu data rusak tidak boleh menghentikan tagihan
// tenant lain. Galat yang dikembalikan hanya galat pencarian kandidat.
func RunSubscriptionRenewals(ctx context.Context, opsi RenewalOptions) (RenewalReport, error) {
	var rep RenewalReport
	now := time.Now().UTC()
	log := helpers.LoggerFromContext(ctx)
	gagal := func(tenantID, langkah string, err error) {
		rep.Failed++
		log.Error("perpanjangan langganan", slog.String("tenant_id", tenantID),
			slog.String("langkah", langkah), slog.Any("error", err))
	}

	// 1. Tagihan perpanjangan, beberapa hari sebelum masa berjalan habis.
	tenants, err := repositories.RenewalDueTenants(ctx, now.AddDate(0, 0, subscriptionRenewalLeadDays()), opsi.TenantID)
	if err != nil {
		return rep, err
	}
	for _, tid := range tenants {
		switch _, err := GenerateInvoice(reqctx.WithTenantID(ctx, tid)); {
		case err == nil:
			rep.Issued++
		case errors.Is(err, helpers.ErrConflict):
			// Sudah diterbitkan jalur lain (pemilik, atau instans lain) — beres.
		default:
			gagal(tid, "tagihan perpanjangan", err)
		}
	}

	// 2. Tagihan yang lewat jatuh tempo → overdue + pengingat sekali.
	lewat, err := repositories.OpenInvoicesPastDue(ctx, firstOfDay(now), opsi.TenantID)
	if err != nil {
		return rep, err
	}
	for _, r := range lewat {
		ditandai, diingatkan, err := tandaiLewatJatuhTempo(ctx, r.TenantID, r.ID)
		if err != nil {
			gagal(r.TenantID, "tagihan lewat jatuh tempo", err)
			continue
		}
		if ditandai {
			rep.Overdue++
		}
		if diingatkan {
			rep.OverdueReminders++
		}
	}

	// 3. Status jujur: masa bayar lewat → past_due (hak paket tetap dihitung
	// dari waktu, termasuk tenggang).
	n, err := repositories.MarkLapsedPastDue(ctx, now, opsi.TenantID)
	if err != nil {
		return rep, err
	}
	rep.PastDue = int(n)

	// 4. Masa coba hampir habis.
	coba, err := repositories.TrialsEndingBetween(ctx, now, now.AddDate(0, 0, subscriptionTrialReminderDays()), opsi.TenantID)
	if err != nil {
		return rep, err
	}
	for _, c := range coba {
		terkirim, err := kirimPengingat(ctx, c.TenantID, models.NoticeTrialEnding,
			c.TrialEndsAt.UTC().Format("2006-01-02"), "subscription.trial_ending", map[string]any{
				"paket":  c.PlanName,
				"sampai": c.TrialEndsAt.UTC().Format("02 Jan 2006"),
			})
		if err != nil {
			gagal(c.TenantID, "pengingat masa coba", err)
		} else if terkirim {
			rep.TrialReminders++
		}
	}

	// 5. Masa tenggang hampir habis dan perpanjangan belum dibayar.
	tenggang := subscriptionGraceDays()
	habis, err := repositories.GraceEndingBetween(ctx, tenggang, now, now.AddDate(0, 0, subscriptionGraceReminderDays()), opsi.TenantID)
	if err != nil {
		return rep, err
	}
	for _, g := range habis {
		terkirim, err := kirimPengingat(ctx, g.TenantID, models.NoticeGraceEnding,
			g.CurrentPeriodEnd.UTC().Format("2006-01-02"), "subscription.grace_ending", map[string]any{
				"nomor":  g.InvoiceNumber,
				"jumlah": helpers.FormatRupiah(g.InvoiceTotal - g.InvoicePaid),
				"sampai": g.CurrentPeriodEnd.UTC().AddDate(0, 0, tenggang).Format("02 Jan 2006"),
			})
		if err != nil {
			gagal(g.TenantID, "pengingat masa tenggang", err)
		} else if terkirim {
			rep.GraceReminders++
		}
	}
	return rep, nil
}

// tandaiLewatJatuhTempo: open → overdue (baris dikunci; yang sudah berubah
// dilewati) dan pengingat sekali per tagihan.
func tandaiLewatJatuhTempo(ctx context.Context, tenantID, invoiceID string) (ditandai, diingatkan bool, err error) {
	tctx := reqctx.WithTenantID(ctx, tenantID)
	err = repositories.WithTenant(tctx, func(tx *gorm.DB) error {
		inv, err := repositories.LockSubInvoiceForTenant(tctx, tx, invoiceID)
		if err != nil {
			return err
		}
		if inv.Status != "open" {
			return nil
		}
		inv.Status = "overdue"
		if err := repositories.SaveSubInvoice(tctx, tx, &inv); err != nil {
			return err
		}
		ditandai = true
		diingatkan, err = catatDanKirim(tctx, tx, tenantID, models.NoticeInvoiceOverdue, inv.ID,
			"subscription.invoice_overdue", map[string]any{
				"nomor":       inv.Number,
				"jumlah":      helpers.FormatRupiah(inv.TotalAmount - inv.PaidAmount),
				"jatuh_tempo": inv.DueDate.Format("02 Jan 2006"),
			})
		return err
	})
	return ditandai, diingatkan, err
}

// kirimPengingat mencatat & mengirim satu pengingat di transaksi tenantnya.
func kirimPengingat(ctx context.Context, tenantID, jenis, ref, topik string, data map[string]any) (bool, error) {
	tctx := reqctx.WithTenantID(ctx, tenantID)
	var terkirim bool
	err := repositories.WithTenant(tctx, func(tx *gorm.DB) error {
		var err error
		terkirim, err = catatDanKirim(tctx, tx, tenantID, jenis, ref, topik, data)
		return err
	})
	return terkirim, err
}

// catatDanKirim: false bila pengingat yang sama sudah pernah tercatat. Tenant
// tanpa nomor HP tetap dicatat (supaya tidak dicoba ulang tiap hari).
func catatDanKirim(ctx context.Context, tx *gorm.DB, tenantID, jenis, ref, topik string, data map[string]any) (bool, error) {
	baru, err := repositories.InsertNoticeOnce(ctx, tx, &models.SubscriptionNotice{Kind: jenis, Ref: ref})
	if err != nil || !baru {
		return false, err
	}
	var tenant models.Tenant
	if err := repositories.FindTenantByID(ctx, tx, tenantID, &tenant); err != nil {
		return false, err
	}
	if tenant.Phone == "" {
		return true, nil
	}
	payload := map[string]any{"channel": "whatsapp", "to": tenant.Phone, "nama_usaha": tenant.BusinessName}
	for k, v := range data {
		payload[k] = v
	}
	tid := tenantID
	return true, EnqueueNotification(ctx, tx, &tid, topik, payload)
}
