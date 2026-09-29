package tests

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"candra/backend-api/database"
	"candra/backend-api/helpers"
	"candra/backend-api/internal/reqctx"
	"candra/backend-api/internal/ulid"
	"candra/backend-api/models"
	"candra/backend-api/repositories"
	"candra/backend-api/services"

	"gorm.io/gorm"
)

// Uji kunci paket langganan (services/plan_entitlement_service.go).
//
// Dulu katalog paket menjanjikan perbedaan (QRIS, kanal online, CRM, banyak
// cabang) yang tidak ditegakkan di mana pun. Tes-tes ini menjaga tiga hal:
// fitur berbayar benar-benar tertutup di paket yang tidak memuatnya; hak itu
// terbuka & tertutup sendiri mengikuti masa langganan; dan kunci paket TIDAK
// PERNAH menghapus penjualan yang sudah terjadi.

// bayarQRIS membangun badan checkout 1×A (15.000) dibayar QRIS.
func bayarQRIS(f posFixture) map[string]any {
	return map[string]any{
		"outlet_id": f.outletID,
		"items":     []map[string]any{{"product_id": f.prodA, "qty": "1"}},
		"payments":  []map[string]any{{"method": "qris", "amount": 15000}},
	}
}

// aturLangganan memaksa status & batas waktu langganan tenant di basis data —
// satu-satunya cara menguji "masa coba habis" tanpa menunggu 14 hari.
func aturLangganan(t *testing.T, tenantID string, kolom map[string]any) {
	t.Helper()
	if err := database.DB.Table("subscriptions").Where("tenant_id = ?", tenantID).
		Updates(kolom).Error; err != nil {
		t.Fatalf("mengatur langganan: %v", err)
	}
}

func paketMe(t *testing.T, token string) map[string]any {
	t.Helper()
	return call(t, "GET", "/api/v1/me", token, nil).mustOK(t, "me").data(t)["plan"].(map[string]any)
}

func TestPaketGratisMengunciFiturBerbayar(t *testing.T) {
	requireDB(t)
	f := setupPOSDengan(t, "kuncigratis", registerTenantPolos)

	p := paketMe(t, f.token)
	if p["code"] != "free" || p["status"] != "none" {
		t.Fatalf("paket tanpa langganan = %v/%v, mau free/none", p["code"], p["status"])
	}
	if p["features"].(map[string]any)["qris"] != false {
		t.Fatalf("paket Gratis memuat QRIS? %v", p["features"])
	}
	// Klien menulis "tersedia di paket …" dari sini, termasuk saat offline.
	naik := p["upgrade_for"].(map[string]any)
	if naik["qris"] != "Basic" || naik["online_channel"] != "Pro" || naik["multi_outlet"] != "Multi-Outlet" ||
		naik["crm_sales"] != "Pro" {
		t.Fatalf("upgrade_for = %v, mau qris→Basic, online_channel & crm_sales→Pro, multi_outlet→Multi-Outlet", naik)
	}

	// QRIS di checkout langsung → 402, menyebut paket termurah yang memuatnya.
	res := checkout(t, f.token, "KG-QRIS", bayarQRIS(f))
	if res.Code != http.StatusPaymentRequired {
		t.Fatalf("checkout QRIS di paket Gratis: kode %d, mau 402. %s", res.Code, res.Raw)
	}
	if !strings.Contains(res.Raw, "Pembayaran QRIS tersedia mulai paket Basic") {
		t.Fatalf("pesan tidak menyebut paket Basic: %s", res.Raw)
	}
	// Tunai tetap jalan — penjualan biasa tidak pernah dikunci paket.
	checkout(t, f.token, "KG-TUNAI", map[string]any{
		"outlet_id": f.outletID,
		"items":     []map[string]any{{"product_id": f.prodA, "qty": "1"}},
		"payments":  []map[string]any{{"method": "cash", "amount": 15000}},
	}).mustCode(t, "checkout tunai", 201)

	// Transaksi QRIS yang terjadi OFFLINE tetap diterima saat disinkron:
	// pembelinya sudah membayar, menolak hanya menghapus catatannya.
	op := saleOp(f, 1)
	op["payload"].(map[string]any)["payments"] = []map[string]any{{"method": "qris", "amount": 15000}}
	hasil := push(t, f.token, op)
	assertI64(t, hasil, "applied", 1)
	assertI64(t, hasil, "rejected", 0)

	// Cabang kedua → 402 menyebut Multi-Outlet.
	res = call(t, "POST", "/api/v1/outlets", f.token, map[string]any{"name": "Cabang Dua"})
	if res.Code != http.StatusPaymentRequired || !strings.Contains(res.Raw, "Multi-Outlet") {
		t.Fatalf("cabang kedua di paket Gratis: kode %d, %s", res.Code, res.Raw)
	}

	// Kanal online & CRM: memulai hal baru dikunci, MEMBACA tetap boleh.
	if res = call(t, "POST", "/api/v1/channels", f.token, map[string]any{"name": "GoFood"}); res.Code != 402 {
		t.Fatalf("buat kanal di paket Gratis: kode %d, mau 402", res.Code)
	}
	call(t, "GET", "/api/v1/channels", f.token, nil).mustOK(t, "baca kanal tetap boleh")
	// Otorisasi toko (Shopee) menulis state baru → ikut dikunci walau tanpa isi.
	call(t, "POST", "/api/v1/channels/01KANALTIDAKADA0000000000/connection/authorize", f.token, nil).
		mustCode(t, "otorisasi toko di paket Gratis", 402)
	if res = call(t, "POST", "/api/v1/deals", f.token, map[string]any{"title": "Proyek"}); res.Code != 402 {
		t.Fatalf("buat deal di paket Gratis: kode %d, mau 402", res.Code)
	}
	call(t, "GET", "/api/v1/deals", f.token, nil).mustOK(t, "baca deal tetap boleh")

	// Setoran kasbon lewat QRIS ikut terkunci; tunai tidak.
	cust := makeCustomer(t, f.token, "Pelanggan Kunci")
	checkout(t, f.token, "KG-KASBON", map[string]any{
		"outlet_id": f.outletID, "customer_id": cust,
		"items":    []map[string]any{{"product_id": f.prodA, "qty": "2"}},
		"payments": []map[string]any{{"method": "credit", "amount": 30000}},
	}).mustCode(t, "jual kasbon", 201)
	rec := call(t, "GET", "/api/v1/receivables?customer_id="+cust, f.token, nil).
		mustOK(t, "piutang").data(t)["data"].([]any)[0].(map[string]any)["id"].(string)
	checkoutLike(t, f.token, ulid.New(), "/api/v1/receivable-payments", map[string]any{
		"receivable_id": rec, "amount": 10000, "method": "qris",
	}).mustCode(t, "setor kasbon QRIS", 402)
	checkoutLike(t, f.token, ulid.New(), "/api/v1/receivable-payments", map[string]any{
		"receivable_id": rec, "amount": 10000, "method": "cash",
	}).mustCode(t, "setor kasbon tunai", 201)

	// Sales lapangan: check-in langsung, rencana kunjungan, dan target baru
	// terkunci; riwayat tetap terbaca.
	res = call(t, "POST", "/api/v1/visits", f.token, map[string]any{"id": ulid.New(), "customer_id": cust})
	// Pesan dibaca dari JSON terurai: di badan mentah "&" tertulis \u0026.
	if pesan, _ := res.Body["message"].(string); res.Code != 402 ||
		!strings.Contains(pesan, "Sales lapangan (kunjungan, target & komisi) tersedia mulai paket Pro") {
		t.Fatalf("check-in di paket Gratis: kode %d, %s", res.Code, res.Raw)
	}
	call(t, "POST", "/api/v1/visit-plans", f.token, map[string]any{
		"plan_date": "2026-09-28", "customer_ids": []string{cust},
	}).mustCode(t, "rencana kunjungan di Gratis", 402)
	call(t, "POST", "/api/v1/sales-targets", f.token, map[string]any{
		"user_id": f.ownerID, "period_start": "2026-10-01", "period_end": "2026-10-31", "target_visits": 10,
	}).mustCode(t, "target baru di Gratis", 402)
	call(t, "GET", "/api/v1/visits", f.token, nil).mustOK(t, "riwayat kunjungan tetap terbaca")

	// Kunjungan yang terjadi OFFLINE tetap diterima saat disinkron — sama
	// seperti penjualan QRIS offline: kunjungannya sudah terjadi di lapangan.
	kunjungan := push(t, f.token, visitOp(ulid.New(), cust, false))
	assertI64(t, kunjungan, "applied", 1)
	assertI64(t, kunjungan, "rejected", 0)
}

func TestMasaCobaMembukaFiturDanBerakhirSendiri(t *testing.T) {
	requireDB(t)
	f := setupPOSDengan(t, "kuncitrial", registerTenantPolos)

	call(t, "POST", "/api/v1/subscription", f.token, map[string]any{
		"plan_code": "basic", "term_months": 1,
	}).mustCode(t, "mulai masa coba Basic", 201)

	if p := paketMe(t, f.token); p["code"] != "basic" || p["status"] != "trial" || p["active_until"] == "" {
		t.Fatalf("paket masa coba = %v", p)
	}
	checkout(t, f.token, "KT-1", bayarQRIS(f)).mustCode(t, "QRIS di masa coba Basic", 201)

	// Basic tidak memuat kanal online → tetap 402, menyebut paket Pro.
	res := call(t, "POST", "/api/v1/channels", f.token, map[string]any{"name": "GoFood"})
	if res.Code != 402 || !strings.Contains(res.Raw, "paket Pro") {
		t.Fatalf("kanal di Basic: kode %d, %s", res.Code, res.Raw)
	}

	// Di tengah masa coba boleh pindah paket — tanggal masa cobanya TIDAK
	// bergeser (memilih ulang bukan cara mendapat masa coba baru).
	sebelum := paketMe(t, f.token)["active_until"]
	call(t, "POST", "/api/v1/subscription", f.token, map[string]any{
		"plan_code": "multi", "term_months": 1,
	}).mustCode(t, "pindah ke Multi-Outlet di masa coba", 201)
	if p := paketMe(t, f.token); p["code"] != "multi" || p["active_until"] != sebelum {
		t.Fatalf("pindah paket di masa coba: %v s.d. %v, mau multi s.d. %v", p["code"], p["active_until"], sebelum)
	}

	// Masa coba habis — tanpa pekerjaan terjadwal apa pun, status di basis
	// data tetap "trial". Hak dihitung dari waktunya: kembali ke Gratis.
	lewat := time.Now().UTC().Add(-time.Hour)
	aturLangganan(t, f.tenantID, map[string]any{"trial_ends_at": lewat, "current_period_end": lewat})
	if p := paketMe(t, f.token); p["code"] != "free" {
		t.Fatalf("setelah masa coba habis paket = %v, mau free", p["code"])
	}
	checkout(t, f.token, "KT-2", bayarQRIS(f)).mustCode(t, "QRIS setelah masa coba habis", 402)

	// Jalan kembali: dulu tenant ini TERJEBAK — memilih paket ditolak
	// ("langganan berjalan") dan ganti paket butuh status active. Kini paketnya
	// boleh diganti (tanpa masa coba baru), lalu tagihan → bayar → aktif.
	sub := call(t, "POST", "/api/v1/subscription", f.token, map[string]any{
		"plan_code": "pro", "term_months": 1,
	}).mustCode(t, "pilih Pro setelah masa coba habis", 201).data(t)
	if sub["plan_code"] != "pro" {
		t.Fatalf("paket setelah dipilih ulang = %v, mau pro", sub["plan_code"])
	}
	if p := paketMe(t, f.token); p["code"] != "free" {
		t.Fatalf("memilih ulang TIDAK boleh memberi masa coba baru; paket = %v", p["code"])
	}
	inv := call(t, "POST", "/api/v1/subscription/invoices", f.token, nil).
		mustCode(t, "tagihan", 201).data(t)
	paySub(t, f.token, "PAY-"+ulid.New(), inv["id"].(string), int64(inv["total_amount"].(float64))).
		mustCode(t, "bayar", 201)
	if p := paketMe(t, f.token); p["code"] != "pro" || p["status"] != "active" {
		t.Fatalf("setelah bayar paket = %v/%v, mau pro/active", p["code"], p["status"])
	}
	checkout(t, f.token, "KT-3", bayarQRIS(f)).mustCode(t, "QRIS setelah bayar", 201)
}

func TestLanggananBerbayarDapatTenggang(t *testing.T) {
	requireDB(t)
	f := setupPOSDengan(t, "kuncitenggang", registerTenantPolos)
	call(t, "POST", "/api/v1/subscription", f.token, map[string]any{
		"plan_code": "pro", "term_months": 1,
	}).mustCode(t, "mulai Pro", 201)

	// Masa bayar habis 3 hari lalu: masih dalam tenggang 7 hari — kasir tidak
	// kehilangan QRIS hanya karena tagihan perpanjangan belum dibayar hari ini.
	aturLangganan(t, f.tenantID, map[string]any{
		"status": "active", "trial_ends_at": nil,
		"current_period_end": time.Now().UTC().AddDate(0, 0, -3),
	})
	if p := paketMe(t, f.token); p["code"] != "pro" {
		t.Fatalf("dalam tenggang paket = %v, mau pro", p["code"])
	}
	checkout(t, f.token, "KP-1", bayarQRIS(f)).mustCode(t, "QRIS dalam tenggang", 201)
	pelanggan := makeCustomer(t, f.token, "Toko Kunjungan")
	if res := call(t, "POST", "/api/v1/visits", f.token, map[string]any{
		"id": ulid.New(), "customer_id": pelanggan,
	}); res.Code == 402 {
		t.Fatalf("check-in di paket Pro ditolak paket: %s", res.Raw)
	}

	// Lewat tenggang → Gratis.
	aturLangganan(t, f.tenantID, map[string]any{"current_period_end": time.Now().UTC().AddDate(0, 0, -8)})
	checkout(t, f.token, "KP-2", bayarQRIS(f)).mustCode(t, "QRIS lewat tenggang", 402)

	// Langganan yang dihentikan tidak memberi hak apa pun, berapa pun sisa
	// masanya.
	aturLangganan(t, f.tenantID, map[string]any{
		"status": "canceled", "current_period_end": time.Now().UTC().AddDate(0, 1, 0),
	})
	checkout(t, f.token, "KP-3", bayarQRIS(f)).mustCode(t, "QRIS setelah berhenti", 402)
}

// Kuota (max_outlets/max_users/max_products) ditegakkan saat membuat data,
// termasuk impor CSV — dan DUA permintaan bersamaan di ambang batas tidak
// boleh sama-sama lolos.
func TestKuotaPaketDitegakkanTanpaBalapan(t *testing.T) {
	requireDB(t)
	dua := 2
	paket := models.Plan{
		Code: "uji-kuota-" + strings.ToLower(ulid.New()), Name: "Uji Kuota", MonthlyPrice: 1,
		MaxOutlets: &dua, MaxUsers: &dua, MaxProducts: &dua,
		Features: json.RawMessage(`{"multi_outlet":true}`),
		// Tidak aktif: tidak muncul di katalog, tapi tetap bisa dilanggani tes.
		IsActive: false,
	}
	if err := database.DB.Create(&paket).Error; err != nil {
		t.Fatalf("membuat paket uji: %v", err)
	}
	database.DB.Model(&paket).Update("is_active", false)

	f := registerTenantPolos(t, "kuncikuota")
	call(t, "POST", "/api/v1/subscription", f.token, map[string]any{
		"plan_code": paket.Code, "term_months": 1,
	}).mustCode(t, "berlangganan paket uji", 201)
	unit := makeUnit(t, f, "pcs")

	barang := func(nama string) map[string]any {
		return map[string]any{"name": nama, "unit_id": unit, "sell_price": 1000, "cost_price": 500}
	}
	call(t, "POST", "/api/v1/products", f.token, barang("Barang 1")).mustCode(t, "barang 1", 201)

	// Delapan transaksi BERSAMAAN memperebutkan satu kursi terakhir: tepat
	// satu yang boleh lolos. Diuji di lapisan layanan, bukan lewat HTTP: di
	// jalur HTTP tes, middleware membuat permintaan berangkat berurutan
	// sehingga balapannya tidak pernah terjadi (tes versi HTTP tetap hijau
	// meski kuncinya dicabut). Di sini setiap goroutine membuka transaksinya
	// sendiri dan menghitung pada saat yang sama.
	ctx := reqctx.WithTenantID(context.Background(), f.tenantID)
	const serentak = 8
	var wg sync.WaitGroup
	mulai := make(chan struct{})
	hasil := make([]error, serentak)
	for i := range hasil {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-mulai
			hasil[i] = repositories.WithTenant(ctx, func(tx *gorm.DB) error {
				if err := services.EnsureQuota(ctx, tx, services.KuotaBarang, 1); err != nil {
					return err
				}
				// Beri jeda antara menghitung dan menyimpan, seperti yang
				// terjadi di jalur sungguhan (validasi, satuan, kategori):
				// tanpa kunci, transaksi lain menghitung di celah ini.
				time.Sleep(20 * time.Millisecond)
				return repositories.CreateProduct(ctx, tx, &models.Product{
					TenantID: f.tenantID, UnitID: unit, Name: "Barang serentak " + ulid.New(),
					SellPrice: 1000, CostPrice: 500, TrackStock: true, IsActive: true,
				})
			})
		}(i)
	}
	close(mulai)
	wg.Wait()
	lolos := 0
	for _, err := range hasil {
		switch {
		case err == nil:
			lolos++
		case errors.Is(err, helpers.ErrPlanRequired):
		default:
			t.Fatalf("transaksi serentak: galat tak terduga %v", err)
		}
	}
	if lolos != 1 {
		t.Fatalf("%d transaksi serentak di batas 2 barang: %d lolos, mau tepat 1", serentak, lolos)
	}

	res := call(t, "POST", "/api/v1/products", f.token, barang("Barang lebih"))
	if res.Code != 402 || !strings.Contains(res.Raw, "dibatasi 2 barang") {
		t.Fatalf("barang melebihi kuota: kode %d, %s", res.Code, res.Raw)
	}
	csv := "name,unit,sell_price\nBarang Impor,pcs,1000\n"
	if imp := callRaw(t, "POST", "/api/v1/products/import", f.token, "text/csv", csv); imp.Code != 402 {
		t.Fatalf("impor melebihi kuota: kode %d, %s", imp.Code, imp.Raw)
	}

	// Cabang: 1 dari pendaftaran + 1 lagi boleh, yang ketiga tidak.
	call(t, "POST", "/api/v1/outlets", f.token, map[string]any{"name": "Cabang 2"}).mustCode(t, "cabang 2", 201)
	res = call(t, "POST", "/api/v1/outlets", f.token, map[string]any{"name": "Cabang 3"})
	if res.Code != 402 || !strings.Contains(res.Raw, "dibatasi 2 cabang") {
		t.Fatalf("cabang ketiga: kode %d, %s", res.Code, res.Raw)
	}

	// Pengguna: pemilik + 1 staf boleh, staf kedua tidak.
	staf := func(nama string) apiResp {
		return call(t, "POST", "/api/v1/users", f.token, map[string]any{
			"name": nama, "username": nama + "_kuota", "email": nama + "@kuota.test",
			"password": "rahasia123", "role_id": roleID(t, f, "Kasir"),
		})
	}
	staf("staf1").mustCode(t, "staf 1", 201)
	if res = staf("staf2"); res.Code != 402 || !strings.Contains(res.Raw, "dibatasi 2 pengguna") {
		t.Fatalf("staf kedua: kode %d, %s", res.Code, res.Raw)
	}
}

// Keputusan produk 2026-09-27: paket Gratis berlaku SELAMANYA dan yang
// dibatasi hanya FITUR — tidak ada kuota pengguna, barang, maupun transaksi.
// (Asumsi awal di model bisnis, "1 pengguna, 50 produk, 300 transaksi",
// ditinggalkan.) Pendaftar baru langsung memakai paket ini tanpa masa coba.
func TestPaketGratisSelamanyaTanpaKuota(t *testing.T) {
	requireDB(t)
	f := registerTenantPolos(t, "gratisselamanya")

	// Katalog: paket Gratis tanpa batas apa pun.
	var gratis map[string]any
	for _, p := range call(t, "GET", "/api/v1/plans", f.token, nil).mustOK(t, "katalog").Body["data"].([]any) {
		if m := p.(map[string]any); m["code"] == "free" {
			gratis = m
		}
	}
	if gratis == nil {
		t.Fatal("paket Gratis tidak ada di katalog")
	}
	for _, k := range []string{"max_outlets", "max_users", "max_products", "max_monthly_transactions"} {
		if gratis[k] != nil {
			t.Fatalf("paket Gratis punya kuota %s = %v; Gratis hanya membatasi fitur", k, gratis[k])
		}
	}

	// Pendaftar baru langsung di Gratis, tanpa masa coba dan tanpa kedaluwarsa.
	p := paketMe(t, f.token)
	if p["code"] != "free" || p["status"] != "none" {
		t.Fatalf("paket pendaftar baru = %v/%v, mau free/none", p["code"], p["status"])
	}
	if v, ada := p["active_until"]; ada && v != "" {
		t.Fatalf("paket Gratis tidak boleh berakhir; active_until = %v", v)
	}
	if p["max_users"] != nil || p["max_products"] != nil {
		t.Fatalf("paket Gratis berkuota: pengguna %v, barang %v", p["max_users"], p["max_products"])
	}

	// Lewat asumsi lama (50 barang) tanpa hambatan.
	makeUnit(t, f, "pcs") // satuan yang dirujuk kolom unit di CSV
	csv := "name,unit,sell_price\n"
	for i := 1; i <= 60; i++ {
		csv += fmt.Sprintf("Barang %d,pcs,1000\n", i)
	}
	imp := callRaw(t, "POST", "/api/v1/products/import", f.token, "text/csv", csv).mustOK(t, "impor 60 barang").data(t)
	if n := imp["imported"].(float64); n != 60 {
		t.Fatalf("barang terimpor = %v, mau 60", n)
	}

	// Lewat asumsi lama (1 pengguna): pemilik + dua staf.
	for _, nama := range []string{"staf_gratis1", "staf_gratis2"} {
		call(t, "POST", "/api/v1/users", f.token, map[string]any{
			"name": nama, "username": nama, "email": nama + "@gratis.test",
			"password": "rahasia123", "role_id": roleID(t, f, "Kasir"),
		}).mustCode(t, "staf "+nama, 201)
	}
}

// Migrasi 000038 menambah fitur crm_sales ke basis data yang SUDAH berjalan
// (seeder hanya menyisipkan paket baru): Pro & Multi-Outlet mendapat true,
// Gratis & Basic false — tanpa menimpa nilai yang sudah diatur admin.
func TestMigrasiFiturSalesLapangan(t *testing.T) {
	requireDB(t)
	sqlNaik, err := os.ReadFile("../database/migrations/000038_plan_feature_crm_sales.up.sql")
	if err != nil {
		t.Fatalf("membaca migrasi: %v", err)
	}

	fitur := func(kode string) map[string]any {
		t.Helper()
		var mentah string
		if err := database.DB.Raw("SELECT features::text FROM plans WHERE code = ?", kode).Scan(&mentah).Error; err != nil {
			t.Fatalf("membaca fitur %s: %v", kode, err)
		}
		var m map[string]any
		_ = json.Unmarshal([]byte(mentah), &m)
		return m
	}
	// Simpan & pulihkan katalog: tabel plans milik seluruh tes.
	asli := map[string]string{}
	for _, k := range []string{"free", "basic", "pro", "multi"} {
		b, _ := json.Marshal(fitur(k))
		asli[k] = string(b)
	}
	defer func() {
		for k, v := range asli {
			database.DB.Exec("UPDATE plans SET features = ?::jsonb WHERE code = ?", v, k)
		}
	}()

	// Keadaan sebelum migrasi: kunci belum ada — kecuali Basic, yang sudah
	// diberi TRUE oleh admin (tidak boleh ditimpa jadi false).
	database.DB.Exec("UPDATE plans SET features = features - 'crm_sales' WHERE code IN ('free','pro','multi')")
	database.DB.Exec(`UPDATE plans SET features = features || '{"crm_sales": true}'::jsonb WHERE code = 'basic'`)

	if err := database.DB.Exec(string(sqlNaik)).Error; err != nil {
		t.Fatalf("menjalankan migrasi: %v", err)
	}
	for kode, mau := range map[string]bool{"free": false, "basic": true, "pro": true, "multi": true} {
		if got := fitur(kode)["crm_sales"]; got != mau {
			t.Fatalf("crm_sales paket %s = %v, mau %v", kode, got, mau)
		}
	}
}
