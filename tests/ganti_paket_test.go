package tests

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"candra/backend-api/database"
	"candra/backend-api/helpers"
	"candra/backend-api/internal/reqctx"
	"candra/backend-api/internal/ulid"
	"candra/backend-api/services"
)

// Uji ganti paket (migrasi 000040) dan masa coba sekali per tenant.
//
// Dulu POST /subscription/change-plan langsung memindahkan langganan ke paket
// baru saat tagihannya TERBIT: tenant Basic bisa pindah ke Multi-Outlet,
// memakai fiturnya, dan tidak pernah membayar. Masa paket barunya pun dimulai
// setelah periode lama habis walau sisa bulan lama sudah dikreditkan.

// langgananAktif: tenant polos dengan langganan berbayar yang sudah lunas,
// periodenya mulai hari ini (masa coba dihabiskan dulu).
func langgananAktif(t *testing.T, slug, plan string, term int) (tenantFixture, map[string]any) {
	t.Helper()
	f := registerTenantPolos(t, slug)
	call(t, "POST", "/api/v1/subscription", f.token, map[string]any{
		"plan_code": plan, "term_months": term,
	}).mustCode(t, "mulai "+plan, 201)
	lewat := time.Now().UTC().Add(-time.Hour)
	aturLangganan(t, f.tenantID, map[string]any{"trial_ends_at": lewat, "current_period_end": lewat})
	inv := call(t, "POST", "/api/v1/subscription/invoices", f.token, nil).mustCode(t, "tagihan", 201).data(t)
	lunas := paySub(t, f.token, ulid.New(), inv["id"].(string), int64(inv["total_amount"].(float64))).
		mustCode(t, "bayar", 201).data(t)
	return f, lunas
}

// mundurkanPeriode menggeser awal periode tagihan & langganan N bulan ke
// belakang — N bulan dianggap sudah terpakai.
func mundurkanPeriode(t *testing.T, f tenantFixture, invID string, bulan int) {
	t.Helper()
	t0 := time.Now().UTC()
	ps := time.Date(t0.Year(), t0.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -bulan, 0)
	database.DB.Exec(`UPDATE subscription_invoices SET period_start = ? WHERE id = ?`, ps, invID)
	database.DB.Exec(`UPDATE subscriptions SET current_period_start = ? WHERE tenant_id = ?`, ps, f.tenantID)
}

func jumlahDiakuiNanti(t *testing.T, invID any) int64 {
	t.Helper()
	var n int64
	database.DB.Raw(`SELECT COALESCE(SUM(amount), 0) FROM deferred_revenue_entries WHERE subscription_invoice_id = ?`, invID).
		Row().Scan(&n)
	return n
}

func statusTagihan(t *testing.T, invID any) string {
	t.Helper()
	var s string
	database.DB.Raw(`SELECT status FROM subscription_invoices WHERE id = ?`, invID).Row().Scan(&s)
	return s
}

func TestGantiPaketBerlakuSetelahDibayar(t *testing.T) {
	requireDB(t)
	f, lama := langgananAktif(t, "gantibayar", "basic", 3)
	lamaID, lamaBayar := lama["id"].(string), int64(lama["paid_amount"].(float64))
	mundurkanPeriode(t, f, lamaID, 1) // 1 bulan terpakai → sisa 2 × 79.000

	inv := call(t, "POST", "/api/v1/subscription/change-plan", f.token, map[string]any{
		"plan_code": "pro", "term_months": 3,
	}).mustCode(t, "ganti ke Pro", 201).data(t)
	if inv["kind"] != "plan_change" || inv["plan_code"] != "pro" || inv["status"] != "open" {
		t.Fatalf("tagihan ganti paket = %v/%v/%v", inv["kind"], inv["plan_code"], inv["status"])
	}
	assertI64(t, inv, "credit_amount", 158000)

	// BELUM dibayar → paket belum berpindah, dan tagihan lama belum disentuh.
	if p := paketMe(t, f.token); p["code"] != "basic" {
		t.Fatalf("paket sebelum tagihan ganti dibayar = %v, mau tetap basic", p["code"])
	}
	if res := call(t, "POST", "/api/v1/channels", f.token, map[string]any{"name": "GoFood"}); res.Code != 402 {
		t.Fatalf("kanal online (fitur Pro) sebelum bayar: kode %d, mau 402", res.Code)
	}
	if n := jumlahDiakuiNanti(t, lamaID); n != lamaBayar {
		t.Fatalf("pengakuan tagihan lama berubah sebelum ganti paket dibayar: %d, mau %d", n, lamaBayar)
	}

	total := int64(inv["total_amount"].(float64))
	paySub(t, f.token, ulid.New(), inv["id"].(string), total).mustCode(t, "bayar ganti paket", 201)

	if p := paketMe(t, f.token); p["code"] != "pro" || p["status"] != "active" {
		t.Fatalf("paket setelah bayar = %v/%v, mau pro/active", p["code"], p["status"])
	}
	s := call(t, "GET", "/api/v1/subscription", f.token, nil).mustOK(t, "ringkasan").data(t)["subscription"].(map[string]any)
	hariIni := time.Now().UTC()
	if tanggal(s["current_period_start"]) != hariIni.Format("2006-01-02") ||
		tanggal(s["current_period_end"]) != hariIni.AddDate(0, 3, 0).Format("2006-01-02") {
		t.Fatalf("periode setelah ganti paket %v–%v, mau mulai hari ini selama 3 bulan",
			s["current_period_start"], s["current_period_end"])
	}
	// Nilai berpindah, tidak hilang: tagihan lama tetap diakui (bayar − kredit),
	// tagihan baru mengakui tunai + kredit.
	if n := jumlahDiakuiNanti(t, lamaID); n != lamaBayar-158000 {
		t.Fatalf("pengakuan tagihan lama = %d, mau %d", n, lamaBayar-158000)
	}
	if n := jumlahDiakuiNanti(t, inv["id"]); n != total+158000 {
		t.Fatalf("pengakuan tagihan ganti paket = %d, mau %d", n, total+158000)
	}
}

func TestGantiPaketBelumDibayarBisaDigantiAtauDibatalkan(t *testing.T) {
	requireDB(t)
	f, _ := langgananAktif(t, "gantibatal", "basic", 1)

	a := call(t, "POST", "/api/v1/subscription/change-plan", f.token, map[string]any{
		"plan_code": "pro", "term_months": 1,
	}).mustCode(t, "ganti ke Pro", 201).data(t)
	// Berubah pikiran sebelum membayar → tagihan pertama digantikan.
	b := call(t, "POST", "/api/v1/subscription/change-plan", f.token, map[string]any{
		"plan_code": "multi", "term_months": 1,
	}).mustCode(t, "ganti ke Multi", 201).data(t)
	if s := statusTagihan(t, a["id"]); s != "void" {
		t.Fatalf("tagihan ganti paket pertama = %s, mau void", s)
	}

	// Batal sama sekali → paket tetap Basic, tidak ada tagihan terbuka.
	call(t, "POST", "/api/v1/subscription/invoices/"+b["id"].(string)+"/void", f.token, nil).mustOK(t, "batalkan")
	if p := paketMe(t, f.token); p["code"] != "basic" {
		t.Fatalf("paket setelah ganti paket dibatalkan = %v, mau basic", p["code"])
	}
	if ov := call(t, "GET", "/api/v1/subscription", f.token, nil).mustOK(t, "ringkasan").data(t); ov["open_invoice"] != nil {
		t.Fatalf("masih ada tagihan terbuka: %v", ov["open_invoice"])
	}

	// Tagihan biasa (perpanjangan) tidak dibatalkan lewat jalur ini.
	per := call(t, "POST", "/api/v1/subscription/invoices", f.token, nil).mustCode(t, "perpanjangan", 201).data(t)
	call(t, "POST", "/api/v1/subscription/invoices/"+per["id"].(string)+"/void", f.token, nil).
		mustCode(t, "batalkan perpanjangan", 409)
}

func TestGantiPaketSaatKonfirmasiMenunggu(t *testing.T) {
	requireDB(t)
	f, _ := langgananAktif(t, "gantitunggu", "basic", 1)
	inv := call(t, "POST", "/api/v1/subscription/change-plan", f.token, map[string]any{
		"plan_code": "pro", "term_months": 1,
	}).mustCode(t, "ganti ke Pro", 201).data(t)
	klaim := kirimKonfirmasi(t, f.token, ulid.New(), inv["id"].(string), int64(inv["total_amount"].(float64))).
		mustCode(t, "konfirmasi", 201).data(t)

	// Uangnya sedang diverifikasi: tagihannya tidak boleh digantikan/dibatalkan.
	call(t, "POST", "/api/v1/subscription/change-plan", f.token, map[string]any{
		"plan_code": "multi", "term_months": 1,
	}).mustCode(t, "ganti lagi saat konfirmasi menunggu", 409)
	call(t, "POST", "/api/v1/subscription/invoices/"+inv["id"].(string)+"/void", f.token, nil).
		mustCode(t, "batalkan saat konfirmasi menunggu", 409)

	call(t, "POST", "/api/v1/platform/subscription-payment-claims/"+klaim["id"].(string)+"/approve",
		tokenKeuanganPlatform(t), nil).mustCode(t, "setujui", 201)
	if p := paketMe(t, f.token); p["code"] != "pro" {
		t.Fatalf("paket setelah disetujui = %v, mau pro", p["code"])
	}
}

func TestGantiPaketKeGratisDitolak(t *testing.T) {
	requireDB(t)
	f, _ := langgananAktif(t, "gantigratis", "basic", 1)
	res := call(t, "POST", "/api/v1/subscription/change-plan", f.token, map[string]any{
		"plan_code": "free", "term_months": 1,
	})
	// Dulu: tagihan Rp0 yang tidak pernah bisa dibayar, menggantung dan
	// menghalangi perpanjangan.
	if res.Code != 422 {
		t.Fatalf("ganti ke Gratis: kode %d, mau 422 (%s)", res.Code, res.Raw)
	}
	if ov := call(t, "GET", "/api/v1/subscription", f.token, nil).mustOK(t, "ringkasan").data(t); ov["open_invoice"] != nil {
		t.Fatalf("ganti ke Gratis menerbitkan tagihan: %v", ov["open_invoice"])
	}
}

// Kredit melebihi biaya paket baru → ditolak dengan jalan keluar, bukan
// diam-diam menghanguskan sisanya. Kredit tepat menutup biaya → langsung
// berlaku tanpa tagihan yang perlu dibayar.
func TestTurunPaketDenganKredit(t *testing.T) {
	requireDB(t)
	f, lama := langgananAktif(t, "gantiturun", "basic", 3)
	res := call(t, "POST", "/api/v1/subscription/change-plan", f.token, map[string]any{
		"plan_code": "basic", "term_months": 1,
	})
	if res.Code != 422 || !strings.Contains(res.Raw, "lebih panjang") {
		t.Fatalf("kredit > biaya: kode %d, %s", res.Code, res.Raw)
	}

	// 2 bulan terpakai → sisa 1 × 79.000 = biaya Basic 1 bulan persis.
	mundurkanPeriode(t, f, lama["id"].(string), 2)
	inv := call(t, "POST", "/api/v1/subscription/change-plan", f.token, map[string]any{
		"plan_code": "basic", "term_months": 1,
	}).mustCode(t, "Basic 3 → 1 bulan", 201).data(t)
	if inv["status"] != "paid" {
		t.Fatalf("tagihan yang tertutup kredit = %v, mau langsung paid", inv["status"])
	}
	assertI64(t, inv, "total_amount", 0)
	s := call(t, "GET", "/api/v1/subscription", f.token, nil).mustOK(t, "ringkasan").data(t)["subscription"].(map[string]any)
	if s["term_months"] != float64(1) || s["status"] != "active" {
		t.Fatalf("langganan setelah turun masa = %v bulan/%v", s["term_months"], s["status"])
	}
	lamaBayar := int64(lama["paid_amount"].(float64))
	if n := jumlahDiakuiNanti(t, lama["id"]); n != lamaBayar-79000 {
		t.Fatalf("pengakuan tagihan lama = %d, mau %d", n, lamaBayar-79000)
	}
	if n := jumlahDiakuiNanti(t, inv["id"]); n != 79000 {
		t.Fatalf("pengakuan tagihan yang tertutup kredit = %d, mau 79000", n)
	}
}

// Masa coba hanya sekali per tenant. Dulu berhenti lalu mulai lagi memberi 14
// hari gratis yang BARU — bisa diulang tanpa batas.
func TestMasaCobaTidakBisaDiulang(t *testing.T) {
	requireDB(t)
	f := registerTenantPolos(t, "cobaulang")
	sub := call(t, "POST", "/api/v1/subscription", f.token, map[string]any{
		"plan_code": "basic", "term_months": 1,
	}).mustCode(t, "mulai", 201).data(t)
	akhir := tanggal(sub["trial_ends_at"])

	// Berhenti di tengah masa coba lalu mulai lagi → sisa masa coba lanjut,
	// tanggalnya TIDAK maju.
	call(t, "POST", "/api/v1/subscription/cancel", f.token, map[string]any{"reason": "coba"}).mustOK(t, "berhenti")
	lagi := call(t, "POST", "/api/v1/subscription", f.token, map[string]any{
		"plan_code": "basic", "term_months": 1,
	}).mustCode(t, "mulai lagi", 201).data(t)
	if tanggal(lagi["trial_ends_at"]) != akhir {
		t.Fatalf("mulai lagi di masa coba: berakhir %v, mau tetap %s", lagi["trial_ends_at"], akhir)
	}
	if p := paketMe(t, f.token); p["code"] != "basic" {
		t.Fatalf("sisa masa coba harus lanjut; paket = %v", p["code"])
	}

	// Masa coba sudah habis, berhenti, mulai lagi → TANPA masa coba baru.
	lewat := time.Now().UTC().Add(-time.Hour)
	aturLangganan(t, f.tenantID, map[string]any{"trial_ends_at": lewat, "current_period_end": lewat})
	call(t, "POST", "/api/v1/subscription/cancel", f.token, map[string]any{"reason": "coba"}).mustOK(t, "berhenti")
	call(t, "POST", "/api/v1/subscription", f.token, map[string]any{
		"plan_code": "pro", "term_months": 1,
	}).mustCode(t, "mulai lagi setelah masa coba habis", 201)
	if p := paketMe(t, f.token); p["code"] != "free" {
		t.Fatalf("mulai lagi memberi masa coba baru; paket = %v, mau free", p["code"])
	}

	// Jalan kembalinya tetap ada: tagihan → bayar → aktif.
	inv := call(t, "POST", "/api/v1/subscription/invoices", f.token, nil).mustCode(t, "tagihan", 201).data(t)
	paySub(t, f.token, ulid.New(), inv["id"].(string), int64(inv["total_amount"].(float64))).mustCode(t, "bayar", 201)
	if p := paketMe(t, f.token); p["code"] != "pro" || p["status"] != "active" {
		t.Fatalf("setelah bayar paket = %v/%v, mau pro/active", p["code"], p["status"])
	}
}

// Di masa coba, memilih paket lain membatalkan tagihan paket lama yang belum
// dibayar — membayarnya akan mengaktifkan paket yang sudah tidak dipilih.
func TestGantiPilihanMasaCobaMembatalkanTagihanLama(t *testing.T) {
	requireDB(t)
	f := registerTenantPolos(t, "cobaganti")
	call(t, "POST", "/api/v1/subscription", f.token, map[string]any{
		"plan_code": "basic", "term_months": 1,
	}).mustCode(t, "mulai Basic", 201)
	lama := call(t, "POST", "/api/v1/subscription/invoices", f.token, nil).mustCode(t, "tagihan Basic", 201).data(t)

	call(t, "POST", "/api/v1/subscription", f.token, map[string]any{
		"plan_code": "pro", "term_months": 1,
	}).mustCode(t, "pilih Pro", 201)
	if s := statusTagihan(t, lama["id"]); s != "void" {
		t.Fatalf("tagihan Basic setelah pilih Pro = %s, mau void", s)
	}
	baru := call(t, "POST", "/api/v1/subscription/invoices", f.token, nil).mustCode(t, "tagihan Pro", 201).data(t)
	if baru["plan_code"] != "pro" {
		t.Fatalf("tagihan baru untuk paket %v, mau pro", baru["plan_code"])
	}

	// Sedang dikonfirmasi → pilihan tidak boleh berganti.
	kirimKonfirmasi(t, f.token, ulid.New(), baru["id"].(string), int64(baru["total_amount"].(float64))).
		mustCode(t, "konfirmasi", 201)
	call(t, "POST", "/api/v1/subscription", f.token, map[string]any{
		"plan_code": "multi", "term_months": 1,
	}).mustCode(t, "pilih Multi saat konfirmasi menunggu", 409)
	if p := paketMe(t, f.token); p["code"] != "pro" {
		t.Fatalf("paket masa coba = %v, mau tetap pro", p["code"])
	}
}

// Konfirmasi dan pembatalan tagihan yang BERSAMAAN: tidak boleh berakhir
// dengan konfirmasi menunggu pada tagihan yang sudah batal (staf akan
// menyetujui uang untuk tagihan yang tidak ada). Keduanya mengunci baris
// tagihan yang sama. Diuji di lapisan layanan — jalur HTTP tes berurutan.
func TestKonfirmasiDanPembatalanSerentak(t *testing.T) {
	requireDB(t)
	f, _ := langgananAktif(t, "gantiserentak", "basic", 1)
	ctx := reqctx.WithUserID(reqctx.WithTenantID(context.Background(), f.tenantID), f.ownerID)

	const putaran = 8
	for i := 0; i < putaran; i++ {
		plan := []string{"pro", "multi"}[i%2]
		inv := call(t, "POST", "/api/v1/subscription/change-plan", f.token, map[string]any{
			"plan_code": plan, "term_months": 1,
		}).mustCode(t, "ganti paket", 201).data(t)
		id, total := inv["id"].(string), int64(inv["total_amount"].(float64))

		var wg sync.WaitGroup
		mulai := make(chan struct{})
		var galatKlaim, galatBatal error
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-mulai
			_, _, galatKlaim = services.SubmitPaymentClaim(ctx, services.SubmitClaimInput{
				InvoiceID: id, Amount: total, Method: "transfer", Reference: "Uji serentak",
				IdempotencyKey: ulid.New(), RequestHash: ulid.New(),
			})
		}()
		go func() {
			defer wg.Done()
			<-mulai
			_, galatBatal = services.VoidPlanChangeInvoice(ctx, id)
		}()
		close(mulai)
		wg.Wait()

		for _, err := range []error{galatKlaim, galatBatal} {
			if err != nil && !errors.Is(err, helpers.ErrConflict) {
				t.Fatalf("putaran %d: galat tak terduga %v", i, err)
			}
		}
		var menunggu int
		database.DB.Raw(`SELECT count(*) FROM subscription_payment_claims c
			JOIN subscription_invoices i ON i.id = c.subscription_invoice_id
			WHERE c.subscription_invoice_id = ? AND c.status = 'pending' AND i.status = 'void'`, id).Row().Scan(&menunggu)
		if menunggu != 0 {
			t.Fatalf("putaran %d: konfirmasi menunggu pada tagihan yang batal", i)
		}
		// Bersihkan untuk putaran berikutnya: tolak konfirmasi yang menang.
		database.DB.Exec(`UPDATE subscription_payment_claims SET status = 'rejected', reject_reason = 'uji'
			WHERE subscription_invoice_id = ? AND status = 'pending'`, id)
	}
}

// Kredit dinilai harga bulanan NORMAL, tapi tagihan prabayar dibayar dengan
// diskon: tanpa batas, pindah paket sesaat setelah membayar 12 bulan memberi
// kredit 12 × 79.000 = 948.000 padahal yang dibayar 789.684.
func TestKreditTidakMelebihiYangDibayar(t *testing.T) {
	requireDB(t)
	f, lama := langgananAktif(t, "gantikredit", "basic", 12)
	assertI64(t, lama, "paid_amount", 789684)
	inv := call(t, "POST", "/api/v1/subscription/change-plan", f.token, map[string]any{
		"plan_code": "pro", "term_months": 12,
	}).mustCode(t, "ganti ke Pro 12 bulan", 201).data(t)
	assertI64(t, inv, "credit_amount", 789684)
}

// Memilih Gratis bukan "berlangganan": dulu di tengah masa coba ia menjadi
// masa coba paket Gratis, lalu "Bayar sekarang" menerbitkan tagihan Rp0 yang
// tidak mungkin dibayar — toko tersangkut.
func TestPaketGratisTidakDipilihAtauDitagih(t *testing.T) {
	requireDB(t)
	f := registerTenantPolos(t, "pilihgratis")
	call(t, "POST", "/api/v1/subscription", f.token, map[string]any{
		"plan_code": "basic", "term_months": 1,
	}).mustCode(t, "mulai Basic", 201)
	call(t, "POST", "/api/v1/subscription", f.token, map[string]any{
		"plan_code": "free", "term_months": 1,
	}).mustCode(t, "pilih Gratis di masa coba", 422)

	// Data lama yang sudah terlanjur "masa coba paket Gratis": tagihannya ditolak.
	database.DB.Exec(`UPDATE subscriptions SET plan_id = (SELECT id FROM plans WHERE code = 'free') WHERE tenant_id = ?`, f.tenantID)
	call(t, "POST", "/api/v1/subscription/invoices", f.token, nil).mustCode(t, "tagihan paket Gratis", 422)
	// Jalan keluarnya: pilih paket berbayar.
	call(t, "POST", "/api/v1/subscription", f.token, map[string]any{
		"plan_code": "pro", "term_months": 1,
	}).mustCode(t, "pilih Pro", 201)
	inv := call(t, "POST", "/api/v1/subscription/invoices", f.token, nil).mustCode(t, "tagihan Pro", 201).data(t)
	if inv["total_amount"].(float64) <= 0 {
		t.Fatalf("tagihan Pro = %v", inv["total_amount"])
	}
}
