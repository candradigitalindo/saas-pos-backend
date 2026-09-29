package tests

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"candra/backend-api/database"
	"candra/backend-api/helpers"
	"candra/backend-api/internal/reqctx"
	"candra/backend-api/internal/ulid"
	"candra/backend-api/services"
)

// Uji pekerjaan harian perpanjangan & pengingat (cmd/subscription-renewals)
// dan antrean pengembalian dana (migrasi 000041). Pekerjaannya dijalankan
// untuk SATU tenant (RenewalOptions.TenantID) supaya tidak menyentuh tenant
// uji lain.

func jalankanPerpanjangan(t *testing.T, tenantID string) services.RenewalReport {
	t.Helper()
	rep, err := services.RunSubscriptionRenewals(context.Background(), services.RenewalOptions{TenantID: tenantID})
	if err != nil {
		t.Fatalf("pekerjaan perpanjangan: %v", err)
	}
	if rep.Failed > 0 {
		t.Fatalf("pekerjaan perpanjangan: %d langkah gagal", rep.Failed)
	}
	return rep
}

func jumlahPesan(t *testing.T, tenantID, topik string) int64 {
	t.Helper()
	var n int64
	database.DB.Table("outbox_events").Where("tenant_id = ? AND topic = ?", tenantID, topik).Count(&n)
	return n
}

func tagihanTerbukaTenant(t *testing.T, tenantID string) (id, status string, periodStart time.Time, jumlah int) {
	t.Helper()
	database.DB.Raw(`SELECT count(*) FROM subscription_invoices WHERE tenant_id = ? AND status IN ('open','overdue')`, tenantID).
		Row().Scan(&jumlah)
	database.DB.Raw(`SELECT id, status, period_start FROM subscription_invoices
		WHERE tenant_id = ? AND status IN ('open','overdue') ORDER BY created_at DESC LIMIT 1`, tenantID).
		Row().Scan(&id, &status, &periodStart)
	return
}

func TestPerpanjanganTerbitSekaliSebelumMasaHabis(t *testing.T) {
	requireDB(t)
	f, _ := langgananAktif(t, "panjangterbit", "basic", 1)
	sebelum := jumlahPesan(t, f.tenantID, "invoice.issued")

	// Masa habis sebulan lagi → belum waktunya.
	if rep := jalankanPerpanjangan(t, f.tenantID); rep.Issued != 0 {
		t.Fatalf("tagihan terbit %d bulan sebelum masa habis, mau 0", rep.Issued)
	}

	akhir := time.Now().UTC().AddDate(0, 0, 5)
	aturLangganan(t, f.tenantID, map[string]any{"current_period_end": akhir})
	if rep := jalankanPerpanjangan(t, f.tenantID); rep.Issued != 1 {
		t.Fatalf("tagihan perpanjangan terbit %d, mau 1", rep.Issued)
	}
	_, status, mulai, n := tagihanTerbukaTenant(t, f.tenantID)
	if n != 1 || status != "open" || mulai.Format("2006-01-02") != akhir.Format("2006-01-02") {
		t.Fatalf("tagihan perpanjangan: %d terbuka, %s, mulai %s; mau 1, open, mulai %s",
			n, status, mulai.Format("2006-01-02"), akhir.Format("2006-01-02"))
	}
	if d := jumlahPesan(t, f.tenantID, "invoice.issued") - sebelum; d != 1 {
		t.Fatalf("pemberitahuan tagihan terbit %d, mau 1", d)
	}

	// Dijalankan lagi (cron dua kali sehari, atau ulang manual) → tidak dobel.
	if rep := jalankanPerpanjangan(t, f.tenantID); rep.Issued != 0 {
		t.Fatalf("putaran kedua menerbitkan %d tagihan lagi", rep.Issued)
	}
	if _, _, _, n := tagihanTerbukaTenant(t, f.tenantID); n != 1 {
		t.Fatalf("tagihan terbuka = %d, mau 1", n)
	}
}

func TestJatuhTempoTenggangDanPengingatSekali(t *testing.T) {
	requireDB(t)
	f, _ := langgananAktif(t, "panjangtempo", "basic", 1)
	aturLangganan(t, f.tenantID, map[string]any{"current_period_end": time.Now().UTC().AddDate(0, 0, 5)})
	jalankanPerpanjangan(t, f.tenantID)
	invID, _, _, _ := tagihanTerbukaTenant(t, f.tenantID)

	// Jatuh tempo lewat, masa bayar lewat 6 hari (tenggang 7 → habis besok).
	database.DB.Exec(`UPDATE subscription_invoices SET due_date = CURRENT_DATE - 2 WHERE id = ?`, invID)
	// Waktu maju: periode lama (dan awal periode tagihan perpanjangannya)
	// berakhir 6 hari lalu.
	akhirLama := time.Now().UTC().AddDate(0, 0, -6)
	aturLangganan(t, f.tenantID, map[string]any{"current_period_end": akhirLama})
	database.DB.Exec(`UPDATE subscription_invoices SET period_start = ?, period_end = ? WHERE id = ?`,
		akhirLama, akhirLama.AddDate(0, 1, 0), invID)

	rep := jalankanPerpanjangan(t, f.tenantID)
	if rep.Overdue != 1 || rep.OverdueReminders != 1 || rep.PastDue != 1 || rep.GraceReminders != 1 {
		t.Fatalf("putaran pertama: %+v; mau 1 overdue, 1 pengingat jatuh tempo, 1 past_due, 1 pengingat tenggang", rep)
	}
	if _, status, _, _ := tagihanTerbukaTenant(t, f.tenantID); status != "overdue" {
		t.Fatalf("status tagihan = %s, mau overdue", status)
	}
	// Hak paket tetap jalan selama tenggang walau status past_due.
	if p := paketMe(t, f.token); p["code"] != "basic" {
		t.Fatalf("paket dalam tenggang = %v, mau basic", p["code"])
	}

	// Cron berjalan lagi → tidak ada pesan dobel.
	rep = jalankanPerpanjangan(t, f.tenantID)
	if rep.OverdueReminders+rep.GraceReminders+rep.Overdue+rep.PastDue != 0 {
		t.Fatalf("putaran kedua mengulang: %+v", rep)
	}
	if a, b := jumlahPesan(t, f.tenantID, "subscription.invoice_overdue"), jumlahPesan(t, f.tenantID, "subscription.grace_ending"); a != 1 || b != 1 {
		t.Fatalf("pesan jatuh tempo %d, tenggang %d; mau 1 & 1", a, b)
	}

	// Dibayar di dalam tenggang → periode bersambung dari akhir yang lama
	// (hari-hari tenggang memang dipakai), status kembali active.
	var total int64
	database.DB.Raw(`SELECT total_amount FROM subscription_invoices WHERE id = ?`, invID).Row().Scan(&total)
	paid := paySub(t, f.token, ulid.New(), invID, total).mustCode(t, "bayar dalam tenggang", 201).data(t)
	if tanggal(paid["period_start"]) != akhirLama.Format("2006-01-02") {
		t.Fatalf("periode setelah bayar dalam tenggang mulai %v, mau %s", paid["period_start"], akhirLama.Format("2006-01-02"))
	}
	if p := paketMe(t, f.token); p["status"] != "active" {
		t.Fatalf("status setelah bayar = %v, mau active", p["status"])
	}
}

// Perpanjangan yang baru dibayar jauh setelah tenggang habis: selama itu toko
// memakai Gratis, jadi periodenya mulai hari pembayaran.
func TestPerpanjanganDibayarSetelahTenggangMulaiHariBayar(t *testing.T) {
	requireDB(t)
	f, _ := langgananAktif(t, "panjangtelat", "basic", 1)
	aturLangganan(t, f.tenantID, map[string]any{"current_period_end": time.Now().UTC().AddDate(0, 0, 5)})
	jalankanPerpanjangan(t, f.tenantID)
	invID, _, _, _ := tagihanTerbukaTenant(t, f.tenantID)

	lama := time.Now().UTC().AddDate(0, 0, -20)
	aturLangganan(t, f.tenantID, map[string]any{"current_period_end": lama})
	database.DB.Exec(`UPDATE subscription_invoices SET period_start = ?, period_end = ? WHERE id = ?`,
		lama, lama.AddDate(0, 1, 0), invID)
	if p := paketMe(t, f.token); p["code"] != "free" {
		t.Fatalf("lewat tenggang paket = %v, mau free", p["code"])
	}

	var total int64
	database.DB.Raw(`SELECT total_amount FROM subscription_invoices WHERE id = ?`, invID).Row().Scan(&total)
	paid := paySub(t, f.token, ulid.New(), invID, total).mustCode(t, "bayar telat", 201).data(t)
	if hariIni := time.Now().UTC().Format("2006-01-02"); tanggal(paid["period_start"]) != hariIni {
		t.Fatalf("periode setelah bayar telat mulai %v, mau hari ini %s", paid["period_start"], hariIni)
	}
}

func TestPengingatMasaCobaSekali(t *testing.T) {
	requireDB(t)
	f := registerTenantPolos(t, "panjangcoba")
	call(t, "POST", "/api/v1/subscription", f.token, map[string]any{
		"plan_code": "pro", "term_months": 1,
	}).mustCode(t, "mulai Pro", 201)

	if rep := jalankanPerpanjangan(t, f.tenantID); rep.TrialReminders != 0 {
		t.Fatalf("pengingat masa coba 14 hari sebelum habis: %d, mau 0", rep.TrialReminders)
	}
	aturLangganan(t, f.tenantID, map[string]any{"trial_ends_at": time.Now().UTC().AddDate(0, 0, 2)})
	if rep := jalankanPerpanjangan(t, f.tenantID); rep.TrialReminders != 1 {
		t.Fatalf("pengingat masa coba: %d, mau 1", rep.TrialReminders)
	}
	jalankanPerpanjangan(t, f.tenantID)
	if n := jumlahPesan(t, f.tenantID, "subscription.trial_ending"); n != 1 {
		t.Fatalf("pesan masa coba = %d, mau 1", n)
	}
	// Masa coba tidak diterbitkan tagihan otomatis — pemilik yang memutuskan.
	if _, _, _, n := tagihanTerbukaTenant(t, f.tenantID); n != 0 {
		t.Fatalf("masa coba diterbitkan %d tagihan otomatis, mau 0", n)
	}
}

// Pekerjaan harian yang berjalan dua kali bersamaan, dan pemilik yang menekan
// "Bayar sekarang" di saat yang sama: tepat SATU tagihan per putaran. Diuji di
// lapisan layanan — jalur HTTP tes berurutan. Beberapa putaran, karena
// balapan tanpa kunci tidak selalu muncul di putaran pertama.
func TestPerpanjanganSerentakSatuTagihan(t *testing.T) {
	requireDB(t)
	f, _ := langgananAktif(t, "panjangserentak", "basic", 1)
	aturLangganan(t, f.tenantID, map[string]any{"current_period_end": time.Now().UTC().AddDate(0, 0, 3)})
	ctx := reqctx.WithUserID(reqctx.WithTenantID(context.Background(), f.tenantID), f.ownerID)

	const putaran, serentak = 6, 16
	for p := 0; p < putaran; p++ {
		var wg sync.WaitGroup
		mulai := make(chan struct{})
		galat := make([]error, serentak)
		for i := range galat {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-mulai
				if i%4 == 0 {
					_, galat[i] = services.RunSubscriptionRenewals(context.Background(), services.RenewalOptions{TenantID: f.tenantID})
					return
				}
				if _, err := services.GenerateInvoice(ctx); err != nil && !errors.Is(err, helpers.ErrConflict) {
					galat[i] = err
				}
			}(i)
		}
		close(mulai)
		wg.Wait()
		for _, err := range galat {
			if err != nil {
				t.Fatalf("putaran %d: galat tak terduga: %v", p, err)
			}
		}
		if _, _, _, n := tagihanTerbukaTenant(t, f.tenantID); n != 1 {
			t.Fatalf("putaran %d: %d penerbitan serentak menghasilkan %d tagihan terbuka, mau 1", p, serentak, n)
		}
		database.DB.Exec(`UPDATE subscription_invoices SET status = 'void' WHERE tenant_id = ? AND status = 'open'`, f.tenantID)
	}
}

// ── Berhenti & pengembalian dana ───────────────────────────────────────────

func TestBerhentiDenganPengembalianDana(t *testing.T) {
	requireDB(t)
	f, lama := langgananAktif(t, "berhentirefund", "basic", 12)
	dibayar := int64(lama["paid_amount"].(float64))

	// Pratinjau = angka yang sama dengan saat benar-benar berhenti.
	pra := call(t, "GET", "/api/v1/subscription/cancel-preview", f.token, nil).mustOK(t, "pratinjau").data(t)
	mau := dibayar - 79000 // bulan pertama dihitung terpakai, harga normal
	assertI64(t, pra, "refund_amount", mau)

	// Ada uang kembali → rekening tujuan wajib.
	call(t, "POST", "/api/v1/subscription/cancel", f.token, map[string]any{"reason": "tutup"}).
		mustCode(t, "berhenti tanpa rekening", 422)
	res := call(t, "POST", "/api/v1/subscription/cancel", f.token, berhentiDengan("tutup")).mustOK(t, "berhenti").data(t)
	assertI64(t, res, "refund_amount", mau)
	if p := paketMe(t, f.token); p["code"] != "free" {
		t.Fatalf("paket setelah berhenti = %v, mau free", p["code"])
	}
	ov := call(t, "GET", "/api/v1/subscription", f.token, nil).mustOK(t, "ringkasan").data(t)
	r := ov["refund"].(map[string]any)
	if r["status"] != "pending" || r["destination_account"] != "1234567890" {
		t.Fatalf("pengembalian di ringkasan = %v", r)
	}
	if n := jumlahPesan(t, f.tenantID, "subscription.refund_requested"); n != 1 {
		t.Fatalf("pesan pengembalian diproses = %d, mau 1", n)
	}

	// Panel: hanya keuangan; tanpa nomor referensi ditolak; sekali saja.
	_, emailSup, passSup := makePlatformAdmin(t, "support-refund", "support")
	call(t, "GET", "/api/v1/platform/subscription-refunds", platformToken(t, emailSup, passSup), nil).
		mustCode(t, "support membaca refund", 403)
	daftar := call(t, "GET", "/api/v1/platform/subscription-refunds", tokenKeuanganPlatform(t), nil).
		mustOK(t, "antrean refund").Body["data"].([]any)
	var baris map[string]any
	for _, d := range daftar {
		if m := d.(map[string]any); m["id"] == r["id"] {
			baris = m
		}
	}
	if baris == nil || baris["business_name"] != "Usaha berhentirefund" || baris["destination_holder"] != "Pemilik Uji" {
		t.Fatalf("antrean refund tanpa konteks: %v", baris)
	}
	jalur := "/api/v1/platform/subscription-refunds/" + r["id"].(string) + "/paid"
	call(t, "POST", jalur, tokenKeuanganPlatform(t), map[string]any{"reference": ""}).mustCode(t, "tanpa referensi", 422)
	call(t, "POST", jalur, tokenKeuanganPlatform(t), map[string]any{"reference": "TRF-BCA-0927"}).mustOK(t, "tandai ditransfer")
	call(t, "POST", jalur, tokenKeuanganPlatform(t), map[string]any{"reference": "TRF-BCA-0927"}).mustCode(t, "tandai dua kali", 409)

	r = call(t, "GET", "/api/v1/subscription", f.token, nil).mustOK(t, "ringkasan").data(t)["refund"].(map[string]any)
	if r["status"] != "paid" || r["payout_reference"] != "TRF-BCA-0927" {
		t.Fatalf("tenant tidak melihat pengembalian terkirim: %v", r)
	}
	var audit int64
	database.DB.Table("audit_logs").Where("tenant_id = ? AND action = 'subscription.refund.paid'", f.tenantID).Count(&audit)
	if n := jumlahPesan(t, f.tenantID, "subscription.refund_paid"); n != 1 || audit != 1 {
		t.Fatalf("pesan terkirim %d, audit %d; mau 1 & 1", n, audit)
	}
}

// Berhenti membatalkan tagihan terbuka yang belum dibayar — kecuali yang
// pembayarannya sedang diverifikasi (uangnya mungkin sudah dikirim).
func TestBerhentiMembatalkanTagihanTerbuka(t *testing.T) {
	requireDB(t)
	f, _ := langgananAktif(t, "berhentitagihan", "basic", 1)
	aturLangganan(t, f.tenantID, map[string]any{"current_period_end": time.Now().UTC().AddDate(0, 0, 3)})
	jalankanPerpanjangan(t, f.tenantID)
	invID, _, _, _ := tagihanTerbukaTenant(t, f.tenantID)
	var total int64
	database.DB.Raw(`SELECT total_amount FROM subscription_invoices WHERE id = ?`, invID).Row().Scan(&total)

	kirimKonfirmasi(t, f.token, ulid.New(), invID, total).mustCode(t, "konfirmasi", 201)
	call(t, "POST", "/api/v1/subscription/cancel", f.token, berhentiDengan("tutup")).
		mustCode(t, "berhenti saat konfirmasi menunggu", 409)

	database.DB.Exec(`UPDATE subscription_payment_claims SET status = 'rejected', reject_reason = 'uji'
		WHERE subscription_invoice_id = ?`, invID)
	call(t, "POST", "/api/v1/subscription/cancel", f.token, berhentiDengan("tutup")).mustOK(t, "berhenti")
	if s := statusTagihan(t, invID); s != "void" {
		t.Fatalf("tagihan terbuka setelah berhenti = %s, mau void", s)
	}
	// Berhenti = tidak ada lagi yang diperpanjang otomatis.
	if rep := jalankanPerpanjangan(t, f.tenantID); rep.Issued != 0 {
		t.Fatalf("langganan yang berhenti diterbitkan %d tagihan", rep.Issued)
	}
}

// Dua staf menandai "sudah ditransfer" bersamaan: tepat satu yang tercatat.
func TestTandaiPengembalianSerentak(t *testing.T) {
	requireDB(t)
	adminID, _, _ := makePlatformAdmin(t, "keuangan-refund", "finance")
	f, _ := langgananAktif(t, "berhentiserentak", "basic", 3)
	call(t, "POST", "/api/v1/subscription/cancel", f.token, berhentiDengan("tutup")).mustOK(t, "berhenti")
	id := call(t, "GET", "/api/v1/subscription", f.token, nil).mustOK(t, "ringkasan").data(t)["refund"].(map[string]any)["id"].(string)

	ctx := reqctx.WithPlatformAdmin(context.Background(), adminID, "finance")
	const serentak = 12
	var wg sync.WaitGroup
	mulai := make(chan struct{})
	hasil := make([]error, serentak)
	for i := range hasil {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-mulai
			_, hasil[i] = services.PlatformMarkRefundPaid(ctx, id, "TRF-SERENTAK")
		}(i)
	}
	close(mulai)
	wg.Wait()
	berhasil := 0
	for _, err := range hasil {
		switch {
		case err == nil:
			berhasil++
		case errors.Is(err, helpers.ErrConflict):
		default:
			t.Fatalf("galat tak terduga: %v", err)
		}
	}
	if n := jumlahPesan(t, f.tenantID, "subscription.refund_paid"); berhasil != 1 || n != 1 {
		t.Fatalf("%d tanda serentak: %d berhasil, %d pesan; mau 1 & 1", serentak, berhasil, n)
	}
}

// Tagihan perpanjangan yang baru terbit di tengah masa tenggang (pekerjaan
// harian sempat mati, atau pemilik menekan "Bayar sekarang"): periodenya tetap
// bersambung dari akhir periode lama — dulu mulai hari ini, jadi hari-hari
// tenggang yang dipakai tidak pernah tertagih.
func TestPerpanjanganDalamTenggangTetapBersambung(t *testing.T) {
	requireDB(t)
	f, _ := langgananAktif(t, "panjangsambung", "basic", 1)
	akhir := time.Now().UTC().AddDate(0, 0, -3)
	aturLangganan(t, f.tenantID, map[string]any{"current_period_end": akhir})
	if rep := jalankanPerpanjangan(t, f.tenantID); rep.Issued != 1 {
		t.Fatalf("tagihan perpanjangan dalam tenggang: %d, mau 1", rep.Issued)
	}
	if _, _, mulai, _ := tagihanTerbukaTenant(t, f.tenantID); mulai.Format("2006-01-02") != akhir.Format("2006-01-02") {
		t.Fatalf("periode perpanjangan mulai %s, mau %s", mulai.Format("2006-01-02"), akhir.Format("2006-01-02"))
	}
}

// Laporan pengguna: di tengah masa coba memilih Gratis, tapi kartu atas tetap
// menampilkan masa coba dan pembayaran. Pindah ke Gratis = menghentikan masa
// coba (tanpa body — tidak ada uang yang dikembalikan): paket langsung Gratis,
// tagihan terbuka batal, dan sisa masa cobanya masih bisa dilanjutkan.
func TestPindahKeGratisDiMasaCoba(t *testing.T) {
	requireDB(t)
	f := registerTenantPolos(t, "cobakegratis")
	sub := call(t, "POST", "/api/v1/subscription", f.token, map[string]any{
		"plan_code": "basic", "term_months": 1,
	}).mustCode(t, "mulai Basic", 201).data(t)
	inv := call(t, "POST", "/api/v1/subscription/invoices", f.token, nil).mustCode(t, "tagihan", 201).data(t)

	pra := call(t, "GET", "/api/v1/subscription/cancel-preview", f.token, nil).mustOK(t, "pratinjau").data(t)
	assertI64(t, pra, "refund_amount", 0)
	call(t, "POST", "/api/v1/subscription/cancel", f.token, nil).mustOK(t, "pindah ke Gratis")

	if p := paketMe(t, f.token); p["code"] != "free" {
		t.Fatalf("paket setelah pindah ke Gratis = %v, mau free", p["code"])
	}
	ov := call(t, "GET", "/api/v1/subscription", f.token, nil).mustOK(t, "ringkasan").data(t)
	if s := ov["subscription"].(map[string]any); s["status"] != "canceled" || ov["open_invoice"] != nil || ov["refund"] != nil {
		t.Fatalf("setelah pindah ke Gratis: status %v, tagihan %v, pengembalian %v", s["status"], ov["open_invoice"], ov["refund"])
	}
	if st := statusTagihan(t, inv["id"]); st != "void" {
		t.Fatalf("tagihan masa coba = %s, mau void", st)
	}
	// Berubah pikiran sebelum masa coba habis → sisa masa cobanya lanjut.
	lagi := call(t, "POST", "/api/v1/subscription", f.token, map[string]any{
		"plan_code": "basic", "term_months": 1,
	}).mustCode(t, "pilih Basic lagi", 201).data(t)
	if tanggal(lagi["trial_ends_at"]) != tanggal(sub["trial_ends_at"]) {
		t.Fatalf("masa coba lanjut s.d. %v, mau %v", lagi["trial_ends_at"], sub["trial_ends_at"])
	}
	if p := paketMe(t, f.token); p["code"] != "basic" {
		t.Fatalf("paket setelah memilih Basic lagi = %v, mau basic", p["code"])
	}
}

// Data lama "berlangganan" paket Gratis tidak pernah ditagih atau diingatkan
// — dulu pekerjaan harian akan gagal menagihnya tiap hari.
func TestPerpanjanganMelewatiPaketGratis(t *testing.T) {
	requireDB(t)
	f := registerTenantPolos(t, "panjanggratis")
	call(t, "POST", "/api/v1/subscription", f.token, map[string]any{
		"plan_code": "basic", "term_months": 1,
	}).mustCode(t, "mulai Basic", 201)
	database.DB.Exec(`UPDATE subscriptions SET plan_id = (SELECT id FROM plans WHERE code = 'free'),
		trial_ends_at = now() + interval '2 days' WHERE tenant_id = ?`, f.tenantID)
	if rep := jalankanPerpanjangan(t, f.tenantID); rep.TrialReminders != 0 {
		t.Fatalf("pengingat masa coba paket Gratis: %d, mau 0", rep.TrialReminders)
	}
	aturLangganan(t, f.tenantID, map[string]any{
		"status": "active", "trial_ends_at": nil, "current_period_end": time.Now().UTC().AddDate(0, 0, 3),
	})
	if rep := jalankanPerpanjangan(t, f.tenantID); rep.Issued != 0 || rep.Failed != 0 {
		t.Fatalf("paket Gratis ditagih: %+v", rep)
	}
}
