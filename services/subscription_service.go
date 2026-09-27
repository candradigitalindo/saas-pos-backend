package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
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

const idempotencyScopeSubClaim = "subscription.payment_claim"

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
		if plan.MonthlyPrice <= 0 {
			// Dulu memilih Gratis di tengah masa coba menghasilkan "masa coba
			// paket Gratis" — dan "Bayar sekarang" lalu menerbitkan tagihan Rp0
			// yang tidak mungkin dibayar, menggantung selamanya.
			return fmt.Errorf("%w: paket %s tidak perlu dipilih — toko memakainya dengan sendirinya bila masa coba atau langganan tidak dibayar. Untuk berhenti sekarang, pakai \"Berhenti berlangganan\"", helpers.ErrValidation, plan.Name)
		}
		rate, err := repositories.TermDiscountRate(ctx, tx, term)
		if err != nil {
			return err
		}

		now := time.Now().UTC()
		trialEnds := now.AddDate(0, 0, subscriptionTrialDays())

		existing, ferr := repositories.FindSubscriptionByTenant(ctx, tx)
		if ferr == nil && (existing.PlanID != plan.ID || existing.TermMonths != term) {
			// Tagihan terbuka menagih paket/masa yang LAMA; membayarnya setelah
			// berganti pilihan akan mengaktifkan paket yang tidak dipilih lagi.
			// Batalkan (kalau belum ada uang/konfirmasi), tenant menerbitkan
			// tagihan baru untuk pilihan barunya.
			if open, ada, err := repositories.OpenSubInvoiceForTenant(ctx, tx); err != nil {
				return err
			} else if ada {
				if err := batalkanTagihanBelumDibayar(ctx, tx, open.ID); err != nil {
					return err
				}
			}
		}
		switch {
		case ferr == nil && existing.Status == "trial":
			// Masih (atau sudah selesai) masa coba: GANTI PAKET TANPA mengubah
			// tanggal masa cobanya. Dulu tenant dalam masa coba tidak bisa
			// berpindah paket sama sekali — memilih paket ditolak ("langganan
			// berjalan") dan ganti paket butuh status "active". Lebih parah
			// setelah masa coba habis (status tetap "trial" karena belum ada
			// pekerjaan terjadwal yang memindahkannya): sejak kunci paket
			// ditegakkan, tenant itu jatuh ke Gratis dan terjebak di sana.
			// Tanggal tidak diubah, jadi memilih ulang tidak pernah memberi masa
			// coba baru; langkah berikutnya adalah tagihan & bayar.
			existing.PlanID = plan.ID
			existing.TermMonths = term
			existing.DiscountRate = rate
			if err := repositories.SaveSubscription(ctx, tx, &existing); err != nil {
				return err
			}
		case ferr == nil:
			if existing.Status == "trial" || existing.Status == "active" || existing.Status == "past_due" {
				return fmt.Errorf("%w: tenant sudah punya langganan berjalan", helpers.ErrConflict)
			}
			// Berlangganan lagi setelah berhenti: TANPA masa coba baru. Dulu
			// langganan yang dihentikan lalu dimulai lagi mendapat 14 hari
			// gratis yang baru — berhenti-mulai bisa diulang tanpa batas.
			// Masa coba hanya sekali per tenant; yang berhenti DI TENGAH masa
			// coba boleh melanjutkan sisanya (tanggal aslinya dipertahankan).
			existing.PlanID = plan.ID
			existing.TermMonths = term
			existing.DiscountRate = rate
			existing.Status = "trial"
			if existing.TrialEndsAt == nil {
				existing.TrialEndsAt = &now
			}
			existing.CurrentPeriodStart = now
			existing.CurrentPeriodEnd = now
			if existing.TrialEndsAt.After(now) {
				existing.CurrentPeriodEnd = *existing.TrialEndsAt
			}
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
		// Dikunci: pekerjaan harian perpanjangan dan pemilik yang menekan
		// "Bayar sekarang" bersamaan tidak boleh menerbitkan dua tagihan.
		sub, err := repositories.LockSubscriptionByTenant(ctx, tx)
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
		if plan.MonthlyPrice <= 0 {
			return fmt.Errorf("%w: paket %s tidak ditagih — pilih paket berbayar dulu", helpers.ErrValidation, plan.Name)
		}

		inv, err := buildInvoice(ctx, tx, sub, plan, sub.TermMonths, sub.DiscountRate, 0, models.SubInvoiceRegular)
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

// buildInvoice merakit sebuah SubscriptionInvoice (belum disimpan). `credit`
// adalah potongan sisa paket lama saat ganti paket, diterapkan setelah diskon
// masa langganan dan dibatasi sebesar sisa tagihannya (CreditAmount mencatat
// yang benar-benar terpakai).
func buildInvoice(ctx context.Context, tx *gorm.DB, sub models.Subscription, plan models.Plan, term int, rate decimal.Decimal, credit int64, kind string) (models.SubscriptionInvoice, error) {
	now := time.Now().UTC()
	today := firstOfDay(now)

	periodStart := awalPeriodeBerbayar(sub, now)
	if kind == models.SubInvoicePlanChange {
		// Ganti paket berlaku sejak lunas (dihitung ulang saat itu), bukan
		// setelah periode lama habis — sisa periode lama sudah dikreditkan.
		periodStart = today
	}
	periodEnd := periodStart.AddDate(0, term, 0)

	gross := plan.MonthlyPrice * int64(term)
	termDiscount := helpers.RoundHalfUpToInt(decimal.NewFromInt(gross).Mul(rate))
	if termDiscount > gross {
		termDiscount = gross
	}
	if credit > gross-termDiscount {
		credit = gross - termDiscount
	}
	if credit < 0 {
		credit = 0
	}
	discount := termDiscount + credit
	total := gross - discount

	number, err := repositories.NextSubInvoiceNumber(ctx, tx)
	if err != nil {
		return models.SubscriptionInvoice{}, err
	}
	planID := plan.ID
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
		PlanID:         &planID,
		Kind:           kind,
		CreditAmount:   credit,
		Plan:           &plan,
	}, nil
}

// awalPeriodeBerbayar menentukan kapan masa berbayar sebuah tagihan dimulai:
//   - perpanjangan langganan berbayar yang belum lewat masa tenggang → tepat
//     saat periode berjalan berakhir (hari-hari tenggang ikut terbayar, karena
//     fiturnya memang tetap jalan);
//   - masih dalam masa coba → saat masa coba berakhir, supaya membayar lebih
//     awal tidak menghanguskan sisa masa coba (dulu masa berbayar dimulai hari
//     itu juga, jadi pemilik yang rajin membayar di hari pertama kehilangan 14
//     hari gratisnya);
//   - selain itu (masa coba habis, langganan lewat tenggang) → hari ini.
func awalPeriodeBerbayar(sub models.Subscription, now time.Time) time.Time {
	switch {
	case (sub.Status == "active" || sub.Status == "past_due") &&
		sub.CurrentPeriodEnd.AddDate(0, 0, subscriptionGraceDays()).After(now):
		return firstOfDay(sub.CurrentPeriodEnd)
	case sub.Status == "trial" && sub.TrialEndsAt != nil && sub.TrialEndsAt.After(now):
		return firstOfDay(*sub.TrialEndsAt)
	default:
		return firstOfDay(now)
	}
}

// firstOfDay memangkas t ke tengah malam UTC.
//
// t diubah ke UTC DULU: tanggal diambil dari zona t sendiri, dan time.Now() di
// server berzona WIB berbeda hari dengan UTC antara 00.00–07.00 WIB — periode
// langganan sempat bergeser sehari tergantung jam berapa tagihan terbit
// (nilai dari basis data sudah UTC, nilai dari time.Now() belum).
func firstOfDay(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// terapkanPembayaran mencatat SATU pembayaran atas tagihan milik tenant
// konteks, di dalam tx pemanggil. Saat tagihan LUNAS: langganan menjadi
// 'active', periodenya maju ke periode tagihan, dan N baris
// deferred_revenue_entries dibuat (§13.4).
//
// Sengaja tidak diekspor dan tidak punya rute tenant: dulu tenant sendiri yang
// memanggil jalur ini (POST /subscription-payments) — menandai tagihannya
// lunas tanpa uang yang diterima siapa pun. Kini satu-satunya pemanggilnya
// adalah persetujuan konfirmasi pembayaran oleh staf keuangan platform
// (PlatformApprovePaymentClaim); jalur gerbang pembayaran kelak memakainya juga.
func terapkanPembayaran(ctx context.Context, tx *gorm.DB, invoiceID string, amount int64, method, reference string) (models.SubscriptionInvoice, models.SubscriptionPayment, error) {
	var bayar models.SubscriptionPayment
	if amount <= 0 {
		return models.SubscriptionInvoice{}, bayar, fmt.Errorf("%w: nominal pembayaran harus > 0", helpers.ErrValidation)
	}
	inv, err := repositories.LockSubInvoiceForTenant(ctx, tx, invoiceID)
	if err != nil {
		return inv, bayar, err
	}
	if inv.Status != "open" && inv.Status != "overdue" {
		return inv, bayar, fmt.Errorf("%w: tagihan tidak dalam status yang bisa dibayar", helpers.ErrConflict)
	}
	if remaining := inv.TotalAmount - inv.PaidAmount; amount > remaining {
		return inv, bayar, fmt.Errorf("%w: pembayaran melebihi sisa tagihan (%s)", helpers.ErrConflict, helpers.FormatRupiah(remaining))
	}

	now := time.Now().UTC()
	bayar = models.SubscriptionPayment{
		SubscriptionInvoiceID: inv.ID,
		Amount:                amount,
		Method:                method,
		Reference:             reference,
		PaidAt:                now,
	}
	if err := repositories.CreateSubPayment(ctx, tx, &bayar); err != nil {
		return inv, bayar, err
	}

	inv.PaidAmount += amount
	if inv.PaidAmount >= inv.TotalAmount {
		if err := lunaskan(ctx, tx, &inv, now); err != nil {
			return inv, bayar, err
		}
	}
	if err := repositories.SaveSubInvoice(ctx, tx, &inv); err != nil {
		return inv, bayar, err
	}
	return inv, bayar, nil
}

// lunaskan menjalankan akibat sebuah tagihan LUNAS (pemanggil menyimpan
// tagihannya): langganan aktif pada paket & periode tagihan itu, dan N baris
// deferred_revenue_entries dibuat (§13.4).
//
// Periode dihitung ulang saat lunas untuk dua jenis tagihan:
//   - ganti paket → mulai hari pelunasan; paket, masa, dan tarif diskon
//     langganan baru berpindah SEKARANG (dulu saat tagihan terbit — paket
//     baru terbuka tanpa dibayar), dan kredit sisa paket lama baru
//     diperhitungkan pada tagihan lamanya sekarang juga;
//   - masa coba → mulai saat masa coba berakhir, atau hari ini bila sudah
//     lewat (hari-hari di paket Gratis tidak ditagihkan).
func lunaskan(ctx context.Context, tx *gorm.DB, inv *models.SubscriptionInvoice, now time.Time) error {
	inv.Status = "paid"
	inv.PaidAt = &now

	sub, err := repositories.FindSubscriptionByTenant(ctx, tx)
	if err != nil {
		return err
	}
	switch {
	case inv.Kind == models.SubInvoicePlanChange:
		inv.PeriodStart = firstOfDay(now)
		inv.PeriodEnd = inv.PeriodStart.AddDate(0, inv.TermMonths, 0)
		rate, err := repositories.TermDiscountRate(ctx, tx, inv.TermMonths)
		if err != nil {
			return err
		}
		sub.TermMonths = inv.TermMonths
		sub.DiscountRate = rate
		if inv.CreditFromInvoiceID != nil {
			lama, err := repositories.FindSubInvoiceForTenant(ctx, tx, *inv.CreditFromInvoiceID)
			if err != nil {
				return err
			}
			// Nilai tagihan lama yang tetap diakui = yang dibayar − yang
			// dipindahkan ke tagihan ini sebagai kredit.
			tetap := lama.PaidAmount - inv.CreditAmount
			if tetap < 0 {
				tetap = 0
			}
			if err := settleInvoiceDeferred(ctx, tx, lama.ID, tetap, now); err != nil {
				return err
			}
		}
	case sub.Status == "trial":
		inv.PeriodStart = awalPeriodeBerbayar(sub, now)
		inv.PeriodEnd = inv.PeriodStart.AddDate(0, inv.TermMonths, 0)
	case now.After(inv.PeriodStart.AddDate(0, 0, subscriptionGraceDays())):
		// Perpanjangan yang baru dibayar setelah masa tenggangnya habis:
		// selama itu toko memakai paket Gratis, jadi hari-hari itu tidak
		// ditagihkan — periode mulai hari pembayaran.
		inv.PeriodStart = firstOfDay(now)
		inv.PeriodEnd = inv.PeriodStart.AddDate(0, inv.TermMonths, 0)
	}
	// Yang dibayar menentukan paketnya.
	if inv.PlanID != nil {
		sub.PlanID = *inv.PlanID
	}
	sub.Status = "active"
	sub.CurrentPeriodStart = inv.PeriodStart
	sub.CurrentPeriodEnd = inv.PeriodEnd
	if err := repositories.SaveSubscription(ctx, tx, &sub); err != nil {
		return err
	}
	if err := repositories.CreateDeferredEntries(ctx, tx, deferredEntriesFor(*inv)); err != nil {
		return err
	}
	return setTenantStatus(ctx, tx, "active")
}

// batalkanTagihanBelumDibayar membatalkan (void) tagihan terbuka yang belum
// menerima uang dan tidak sedang dikonfirmasi. Baris tagihannya dikunci —
// jalur kirim konfirmasi & persetujuan melewati kunci yang sama.
func batalkanTagihanBelumDibayar(ctx context.Context, tx *gorm.DB, invoiceID string) error {
	inv, err := repositories.LockSubInvoiceForTenant(ctx, tx, invoiceID)
	if err != nil {
		return err
	}
	if inv.Status != "open" && inv.Status != "overdue" {
		return fmt.Errorf("%w: tagihan %s tidak lagi terbuka", helpers.ErrConflict, inv.Number)
	}
	if inv.PaidAmount > 0 {
		return fmt.Errorf("%w: tagihan %s sudah dibayar sebagian — hubungi kami untuk menyelesaikannya", helpers.ErrConflict, inv.Number)
	}
	switch k, err := repositories.LatestSubClaimForInvoice(ctx, tx, inv.ID); {
	case err == nil && k.Status == "pending":
		return fmt.Errorf("%w: pembayaran tagihan %s sedang diverifikasi — tunggu hasilnya dulu", helpers.ErrConflict, inv.Number)
	case err != nil && !errors.Is(err, repositories.ErrSubClaimNotFound):
		return err
	}
	inv.Status = "void"
	return repositories.SaveSubInvoice(ctx, tx, &inv)
}

// VoidPlanChangeInvoice membatalkan tagihan GANTI PAKET yang belum dibayar —
// tenant berubah pikiran. Paketnya memang belum berpindah (baru berpindah saat
// lunas), jadi cukup tagihannya yang dibatalkan.
func VoidPlanChangeInvoice(ctx context.Context, invoiceID string) (structs.SubInvoiceResponse, error) {
	var out structs.SubInvoiceResponse
	err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		inv, err := repositories.FindSubInvoiceForTenant(ctx, tx, invoiceID)
		if err != nil {
			return err
		}
		if inv.Kind != models.SubInvoicePlanChange {
			return fmt.Errorf("%w: hanya tagihan ganti paket yang bisa dibatalkan", helpers.ErrConflict)
		}
		if err := batalkanTagihanBelumDibayar(ctx, tx, inv.ID); err != nil {
			return err
		}
		inv, err = repositories.FindSubInvoiceForTenant(ctx, tx, invoiceID)
		if err != nil {
			return err
		}
		out = subInvoiceToResponse(inv)
		return nil
	})
	return out, err
}

// deferredEntriesFor memecah total tagihan menjadi N baris pengakuan bulanan
// (§13.4): masing-masing total/N, sisa pembulatan ditaruh di bulan terakhir agar
// jumlahnya persis.
func deferredEntriesFor(inv models.SubscriptionInvoice) []models.DeferredRevenueEntry {
	n := inv.TermMonths
	if n < 1 {
		n = 1
	}
	// Nilai layanan tagihan ini = uang tunainya + kredit yang dipindahkan dari
	// tagihan lama (yang di sana sudah tidak diakui lagi). Tanpa kredit, nilai
	// itu hilang dari pengakuan walau uangnya tetap kita pegang.
	nilai := inv.TotalAmount + inv.CreditAmount
	per := nilai / int64(n)
	remainder := nilai - per*int64(n)
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

// CancelInput: alasan berhenti + rekening tujuan pengembalian dana (wajib bila
// ada uang yang dikembalikan — staf keuangan mentransfernya dari panel).
type CancelInput struct {
	Reason, Bank, Account, Holder string
}

// hitungPengembalian menghitung pengembalian dana bila langganan dihentikan
// sekarang, dari tagihan berbayar terbaru (blueprint aturan 3):
//
//	refund = maks(0, dibayar − bulan_terpakai × harga_bulanan_normal)
//
// ada=false bila belum pernah ada tagihan berbayar (mis. masih masa coba).
func hitungPengembalian(ctx context.Context, tx *gorm.DB, sub models.Subscription, now time.Time) (inv models.SubscriptionInvoice, ada bool, monthsUsed int, refund, earned int64, err error) {
	inv, err = repositories.LatestPaidSubInvoice(ctx, tx)
	if errors.Is(err, repositories.ErrSubInvoiceNotFound) {
		return inv, false, 0, 0, 0, nil
	}
	if err != nil {
		return inv, false, 0, 0, 0, err
	}
	harga := int64(0)
	if inv.Plan != nil {
		harga = inv.Plan.MonthlyPrice
	} else {
		plan, err := repositories.FindPlanByID(ctx, tx, sub.PlanID)
		if err != nil {
			return inv, false, 0, 0, 0, err
		}
		harga = plan.MonthlyPrice
	}
	monthsUsed = wholeMonthsBetween(inv.PeriodStart, now)
	switch {
	case now.Before(inv.PeriodStart):
		// Dibayar di masa coba lalu berhenti sebelum masa berbayarnya
		// mulai: belum ada bulan yang terpakai, uangnya kembali utuh.
		monthsUsed = 0
	case monthsUsed < 1:
		monthsUsed = 1
	}
	if monthsUsed > inv.TermMonths {
		monthsUsed = inv.TermMonths
	}
	refund = inv.PaidAmount - int64(monthsUsed)*harga
	if refund < 0 {
		refund = 0
	}
	return inv, true, monthsUsed, refund, inv.PaidAmount - refund, nil
}

// CancelPreview: angka pengembalian bila langganan dihentikan SEKARANG —
// ditampilkan sebelum pemilik menekan "Hentikan".
func CancelPreview(ctx context.Context) (structs.SubscriptionCancelPreview, error) {
	var out structs.SubscriptionCancelPreview
	err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		sub, err := repositories.FindSubscriptionByTenant(ctx, tx)
		if err != nil {
			return err
		}
		if sub.Status == "canceled" || sub.Status == "expired" {
			return fmt.Errorf("%w: langganan sudah berakhir", helpers.ErrConflict)
		}
		inv, ada, bulan, refund, _, err := hitungPengembalian(ctx, tx, sub, time.Now().UTC())
		if err != nil {
			return err
		}
		out = structs.SubscriptionCancelPreview{RefundAmount: refund, MonthsUsed: bulan}
		if ada {
			out.PaidAmount = inv.PaidAmount
			out.TermMonths = inv.TermMonths
			if inv.Plan != nil {
				out.MonthlyPrice = inv.Plan.MonthlyPrice
			}
		}
		return nil
	})
	return out, err
}

// CancelSubscription menghentikan langganan sekarang: paket kembali ke Gratis,
// tagihan terbuka yang belum dibayar dibatalkan, dan pengembalian dana (bila
// ada) masuk ANTREAN staf keuangan ke rekening yang diisi pemilik. Pengakuan
// pendapatan disamakan agar total yang diakui = dibayar − refund.
func CancelSubscription(ctx context.Context, in CancelInput) (structs.SubscriptionCancelResponse, error) {
	var out structs.SubscriptionCancelResponse
	in.Reason, in.Bank = strings.TrimSpace(in.Reason), strings.TrimSpace(in.Bank)
	in.Account, in.Holder = strings.TrimSpace(in.Account), strings.TrimSpace(in.Holder)
	err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		sub, err := repositories.LockSubscriptionByTenant(ctx, tx)
		if err != nil {
			return err
		}
		if sub.Status == "canceled" || sub.Status == "expired" {
			return fmt.Errorf("%w: langganan sudah berakhir", helpers.ErrConflict)
		}
		now := time.Now().UTC()

		// Tagihan yang masih terbuka tidak akan dibayar lagi. Yang sedang
		// dikonfirmasi menahan penghentian — uangnya mungkin sudah dikirim.
		for {
			open, ada, err := repositories.OpenSubInvoiceForTenant(ctx, tx)
			if err != nil {
				return err
			}
			if !ada {
				break
			}
			if err := batalkanTagihanBelumDibayar(ctx, tx, open.ID); err != nil {
				return err
			}
		}

		inv, ada, monthsUsed, refund, earned, err := hitungPengembalian(ctx, tx, sub, now)
		if err != nil {
			return err
		}
		if ada {
			if refund > 0 && (in.Bank == "" || in.Account == "" || in.Holder == "") {
				return fmt.Errorf("%w: isi bank, nomor rekening, dan nama pemilik rekening untuk pengembalian %s",
					helpers.ErrValidation, helpers.FormatRupiah(refund))
			}
			// Yang dikembalikan hanya uang tunai; kredit dari paket lama yang
			// terpakai di tagihan ini tetap nilai yang diakui.
			if err := settleInvoiceDeferred(ctx, tx, inv.ID, earned+inv.CreditAmount, now); err != nil {
				return err
			}
			inv.PaidAmount = earned
			inv.Status = "refunded"
			if err := repositories.SaveSubInvoice(ctx, tx, &inv); err != nil {
				return err
			}
			r := models.SubscriptionRefund{
				SubscriptionInvoiceID: inv.ID,
				Amount:                refund,
				MonthsUsed:            monthsUsed,
				Reason:                in.Reason,
				RefundedAt:            now,
				Status:                "not_needed",
			}
			if refund > 0 {
				r.Status = "pending"
				r.DestinationBank, r.DestinationAccount, r.DestinationHolder = in.Bank, in.Account, in.Holder
			}
			if err := repositories.CreateSubRefund(ctx, tx, &r); err != nil {
				return err
			}
			if refund > 0 {
				tid := reqctx.TenantID(ctx)
				var tenant models.Tenant
				if err := repositories.FindTenantByID(ctx, tx, tid, &tenant); err == nil && tenant.Phone != "" {
					_ = EnqueueNotification(ctx, tx, &tid, "subscription.refund_requested", map[string]any{
						"channel":    "whatsapp",
						"to":         tenant.Phone,
						"nama_usaha": tenant.BusinessName,
						"jumlah":     helpers.FormatRupiah(refund),
						"rekening":   in.Bank + " " + in.Account + " a.n. " + in.Holder,
					})
				}
			}
		}

		sub.Status = "canceled"
		sub.CanceledAt = &now
		sub.CancelReason = in.Reason
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

// ChangePlan menerbitkan tagihan GANTI PAKET dengan kredit atas sisa masa
// paket lama (dihitung pada harga bulanan normal, tidak pernah melebihi yang
// dibayar). Langganan TIDAK berpindah di sini: paket, masa, dan periodenya
// berpindah saat tagihan ini lunas (lihat lunaskan). Dulu paket langsung
// berpindah saat tagihan terbit — sejak kunci paket ditegakkan, itu membuka
// paket yang lebih mahal tanpa membayar.
//
// Tagihan ganti paket yang belum dibayar boleh digantikan (memilih paket lain
// lagi) atau dibatalkan (VoidPlanChangeInvoice). Bila biayanya habis tertutup
// kredit, perpindahannya langsung berlaku.
func ChangePlan(ctx context.Context, planCode string, term int) (structs.SubInvoiceResponse, error) {
	var out structs.SubInvoiceResponse
	if !validTerm(term) {
		return out, fmt.Errorf("%w: masa langganan harus 1, 3, 6, 9, atau 12 bulan", helpers.ErrValidation)
	}
	err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		sub, err := repositories.LockSubscriptionByTenant(ctx, tx)
		if err != nil {
			return err
		}
		// past_due = masa bayar sudah lewat, tagihan perpanjangan belum lunas:
		// pindah paket saat itu justru wajar (kreditnya nol).
		if sub.Status != "active" && sub.Status != "past_due" {
			return fmt.Errorf("%w: hanya langganan berbayar yang berjalan yang bisa ganti paket", helpers.ErrConflict)
		}
		newPlan, err := repositories.FindPlanByCode(ctx, tx, planCode)
		if err != nil {
			return err
		}
		if newPlan.MonthlyPrice <= 0 {
			// Tagihan Rp0 tidak pernah bisa dibayar — dulu ia menggantung
			// sebagai tagihan terbuka dan menghalangi perpanjangan.
			return fmt.Errorf("%w: paket %s tidak perlu dibeli — bila masa berjalan tidak diperpanjang, toko kembali ke paket %s dengan sendirinya", helpers.ErrValidation, newPlan.Name, newPlan.Name)
		}
		if newPlan.ID == sub.PlanID && term == sub.TermMonths {
			return fmt.Errorf("%w: paket %s %d bulan sedang dipakai", helpers.ErrConflict, newPlan.Name, term)
		}
		if open, ada, err := repositories.OpenSubInvoiceForTenant(ctx, tx); err != nil {
			return err
		} else if ada {
			if open.Kind != models.SubInvoicePlanChange {
				return fmt.Errorf("%w: selesaikan tagihan yang belum dibayar dulu", helpers.ErrConflict)
			}
			// Berubah pikiran sebelum membayar: tagihan ganti paket yang lama
			// digantikan.
			if err := batalkanTagihanBelumDibayar(ctx, tx, open.ID); err != nil {
				return err
			}
		}

		newRate, err := repositories.TermDiscountRate(ctx, tx, term)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		var credit int64
		var dari *string
		if paidInv, ierr := repositories.LatestPaidSubInvoice(ctx, tx); ierr == nil {
			hargaLama := int64(0)
			if paidInv.Plan != nil {
				hargaLama = paidInv.Plan.MonthlyPrice
			} else if oldPlan, err := repositories.FindPlanByID(ctx, tx, sub.PlanID); err == nil {
				hargaLama = oldPlan.MonthlyPrice
			} else {
				return err
			}
			used := wholeMonthsBetween(paidInv.PeriodStart, now)
			remaining := paidInv.TermMonths - used
			if remaining < 0 {
				remaining = 0
			}
			credit = int64(remaining) * hargaLama
			// Tidak pernah lebih dari uang yang benar-benar diterima (+ kredit
			// yang dulu dipindahkan ke tagihan itu): tagihan prabayar berdiskon
			// dinilai harga normal, jadi tanpa batas ini ganti paket sesaat
			// setelah membayar memberi kredit lebih besar dari pembayarannya.
			if batas := paidInv.PaidAmount + paidInv.CreditAmount; credit > batas {
				credit = batas
			}
			if credit > 0 {
				id := paidInv.ID
				dari = &id
			}
		} else if ierr != repositories.ErrSubInvoiceNotFound {
			return ierr
		}

		inv, err := buildInvoice(ctx, tx, sub, newPlan, term, newRate, credit, models.SubInvoicePlanChange)
		if err != nil {
			return err
		}
		if inv.CreditAmount < credit {
			// Kredit melebihi biaya paket baru: sisanya akan hangus. Jangan
			// diam-diam — minta masa yang lebih panjang atau tunggu periode
			// berjalan selesai.
			return fmt.Errorf("%w: sisa masa paket sekarang bernilai %s, lebih besar dari biaya paket %s %d bulan (%s) — pilih masa langganan yang lebih panjang, atau pindah paket setelah masa berjalan berakhir",
				helpers.ErrValidation, helpers.FormatRupiah(credit), newPlan.Name, term, helpers.FormatRupiah(inv.GrossAmount-(inv.DiscountAmount-inv.CreditAmount)))
		}
		inv.CreditFromInvoiceID = dari
		if err := repositories.CreateSubInvoice(ctx, tx, &inv); err != nil {
			return err
		}
		if inv.TotalAmount == 0 {
			// Biaya habis tertutup kredit — tidak ada yang perlu dibayar.
			if err := lunaskan(ctx, tx, &inv, now); err != nil {
				return err
			}
			if err := repositories.SaveSubInvoice(ctx, tx, &inv); err != nil {
				return err
			}
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
		k, err := repositories.LatestSubClaimForInvoice(ctx, nil, inv.ID)
		switch {
		case err == nil:
			kr := claimToResponse(k, inv.Number)
			out.PaymentClaim = &kr
		case !errors.Is(err, repositories.ErrSubClaimNotFound):
			return out, err
		}
	}
	out.PaymentInstructions = PaymentInstructions()
	switch r, err := repositories.LatestRefundForTenant(ctx, nil); {
	case err == nil && r.Status != "not_needed":
		rr := refundToResponse(r)
		out.Refund = &rr
	case err != nil && !errors.Is(err, repositories.ErrSubRefundNotFound):
		return out, err
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
		Kind: i.Kind, CreditAmount: i.CreditAmount,
	}
	if i.Plan != nil {
		r.PlanCode = i.Plan.Code
		r.PlanName = i.Plan.Name
	}
	if i.PaidAt != nil {
		r.PaidAt = i.PaidAt.UTC().Format(saleTimeLayout)
	}
	return r
}
