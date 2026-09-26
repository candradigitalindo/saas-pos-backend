package tests

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"candra/backend-api/database"
	"candra/backend-api/database/migrations"
	"candra/backend-api/internal/ulid"
)

// Uji regresi hasil audit menyeluruh (2026-09-26). Setiap test di sini
// dibuktikan GAGAL pada kode sebelum perbaikannya — bukan sekadar mengunci
// perilaku yang sudah benar.

// TestCheckoutRejectsOrderDiscountAboveTotal — diskon transaksi yang melebihi
// nilai belanja dulu diterima: total jadi negatif dan "kembalian" yang tercatat
// lebih besar dari uang yang diterima, sehingga laci terlihat harus
// mengeluarkan uang yang tidak pernah masuk.
func TestCheckoutRejectsOrderDiscountAboveTotal(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "odisc")

	checkout(t, f.token, "OD-1", map[string]any{
		"outlet_id":      f.outletID,
		"items":          []map[string]any{{"product_id": f.prodA, "qty": "1"}}, // 15000
		"order_discount": 50000,
		"payments":       []map[string]any{{"method": "cash", "amount": 1000}},
	}).mustCode(t, "diskon melebihi total", 422)

	// Diskon tepat sebesar total tetap sah (gratis), dan tidak ada kembalian palsu.
	d := checkout(t, f.token, "OD-2", map[string]any{
		"outlet_id":      f.outletID,
		"items":          []map[string]any{{"product_id": f.prodA, "qty": "1"}},
		"order_discount": 14000,
		"payments":       []map[string]any{{"method": "cash", "amount": 1000}},
	}).mustCode(t, "diskon wajar", 201).data(t)
	assertI64(t, d, "total", 1000)
	assertI64(t, d, "change_amount", 0)
}

// TestCheckoutDiscountNeedsPermission — izin sale.discount dulu hanya
// menyembunyikan kolom diskon di layar; server menerima diskon dari peran
// mana pun yang boleh berjualan.
func TestCheckoutDiscountNeedsPermission(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "discperm")

	role := call(t, "POST", "/api/v1/roles", f.token, map[string]any{
		"name":             "Kasir Tanpa Diskon",
		"permission_codes": []string{"sale.create", "shift.open", "shift.close", "product.view"},
	}).mustCode(t, "buat peran", 201).data(t)["id"].(string)
	kasir := staffToken(t, f.tenantFixture, role, "kasir_discperm")

	checkout(t, kasir, "DP-1", map[string]any{
		"outlet_id": f.outletID,
		"items":     []map[string]any{{"product_id": f.prodA, "qty": "1", "discount_amount": 5000}},
		"payments":  []map[string]any{{"method": "cash", "amount": 10000}},
	}).mustCode(t, "diskon baris tanpa izin", 403)

	checkout(t, kasir, "DP-2", map[string]any{
		"outlet_id":      f.outletID,
		"items":          []map[string]any{{"product_id": f.prodA, "qty": "1"}},
		"order_discount": 5000,
		"payments":       []map[string]any{{"method": "cash", "amount": 10000}},
	}).mustCode(t, "diskon transaksi tanpa izin", 403)

	// Tanpa diskon, peran yang sama tetap bisa berjualan.
	checkout(t, kasir, "DP-3", map[string]any{
		"outlet_id": f.outletID,
		"items":     []map[string]any{{"product_id": f.prodA, "qty": "1"}},
		"payments":  []map[string]any{{"method": "cash", "amount": 15000}},
	}).mustCode(t, "tanpa diskon", 201)
}

// TestUnknownCustomerDoesNotPoisonSyncBatch — customer_id yang tidak ada dulu
// lolos ke INSERT dan ditolak foreign key sebagai galat 500. Di /sync/push,
// galat 500 membatalkan SELURUH batch, sehingga satu transaksi offline yang
// merujuk pelanggan terhapus membuat seluruh antrean perangkat macet.
func TestUnknownCustomerDoesNotPoisonSyncBatch(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "custfk")

	checkout(t, f.token, "CF-1", map[string]any{
		"outlet_id":   f.outletID,
		"customer_id": ulid.New(),
		"items":       []map[string]any{{"product_id": f.prodA, "qty": "1"}},
		"payments":    []map[string]any{{"method": "cash", "amount": 15000}},
	}).mustCode(t, "pelanggan tak dikenal", 422)

	buruk := saleOp(f, 1)
	buruk["payload"].(map[string]any)["customer_id"] = ulid.New()
	baik := saleOp(f, 2)

	res := push(t, f.token, buruk, baik)
	assertInt(t, res, "applied", 1)
	assertInt(t, res, "rejected", 1)
}

// TestVoidAfterRefundRejected — penjualan yang sudah diretur tetap berstatus
// 'completed' (returnya baris terpisah), dan void dulu mengizinkannya. Stok
// dikembalikan DUA kali dan omzet dikurangi dua kali.
func TestVoidAfterRefundRejected(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "voidref")

	sale := checkout(t, f.token, "VR-1", map[string]any{
		"outlet_id": f.outletID,
		"items":     []map[string]any{{"product_id": f.prodA, "qty": "4"}},
		"payments":  []map[string]any{{"method": "cash", "amount": 60000}},
	}).mustCode(t, "checkout", 201).data(t)
	id := sale["id"].(string)

	call(t, "POST", "/api/v1/sales/"+id+"/refund", f.token, map[string]any{"reason": "rusak"}).
		mustOK(t, "retur")
	if q := stockQty(t, f.tenantFixture, f.outletID, f.prodA); q != "100" {
		t.Fatalf("stok setelah retur = %s, mau 100", q)
	}

	call(t, "POST", "/api/v1/sales/"+id+"/void", f.token, map[string]any{"reason": "salah"}).
		mustCode(t, "void setelah retur", 409)
	if q := stockQty(t, f.tenantFixture, f.outletID, f.prodA); q != "100" {
		t.Fatalf("stok setelah void-setelah-retur = %s, mau tetap 100", q)
	}
}

// TestRefundReducesDrawerCash — retur tunai adalah uang yang KELUAR dari laci,
// tapi dulu tidak mengurangi "uang yang seharusnya ada", sehingga setiap retur
// muncul sebagai kekurangan kas kasir saat tutup shift.
func TestRefundReducesDrawerCash(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "refcash") // modal awal 100000

	sale := checkout(t, f.token, "RC-1", map[string]any{
		"outlet_id": f.outletID,
		"items":     []map[string]any{{"product_id": f.prodA, "qty": "2"}}, // 30000
		"payments":  []map[string]any{{"method": "cash", "amount": 50000}}, // kembali 20000
	}).mustCode(t, "checkout", 201).data(t)

	sh := call(t, "GET", "/api/v1/shifts/"+f.shiftID, f.token, nil).mustOK(t, "shift").data(t)
	assertI64(t, sh, "expected_cash", 130000)

	call(t, "POST", "/api/v1/sales/"+sale["id"].(string)+"/refund", f.token, map[string]any{}).
		mustOK(t, "retur")

	sh = call(t, "GET", "/api/v1/shifts/"+f.shiftID, f.token, nil).mustOK(t, "shift").data(t)
	assertI64(t, sh, "expected_cash", 100000)

	// Retur penjualan dari shift yang SUDAH ditutup dibayar dari laci shift
	// yang sedang berjalan.
	sale2 := checkout(t, f.token, "RC-2", map[string]any{
		"outlet_id": f.outletID,
		"items":     []map[string]any{{"product_id": f.prodB, "qty": "1"}}, // 8000
		"payments":  []map[string]any{{"method": "cash", "amount": 8000}},
	}).mustCode(t, "checkout 2", 201).data(t)
	call(t, "POST", "/api/v1/shifts/"+f.shiftID+"/close", f.token, map[string]any{"counted_cash": 108000}).
		mustOK(t, "tutup shift")
	baru := call(t, "POST", "/api/v1/shifts/open", f.token, map[string]any{
		"outlet_id": f.outletID, "opening_cash": 50000,
	}).mustCode(t, "buka shift baru", 201).data(t)["id"].(string)

	ret := call(t, "POST", "/api/v1/sales/"+sale2["id"].(string)+"/refund", f.token, map[string]any{}).
		mustOK(t, "retur 2").data(t)
	if ret["shift_id"] != baru {
		t.Fatalf("retur dicatat di shift %v, mau shift berjalan %s", ret["shift_id"], baru)
	}
	sh = call(t, "GET", "/api/v1/shifts/"+baru, f.token, nil).mustOK(t, "shift baru").data(t)
	assertI64(t, sh, "expected_cash", 42000)
}

// TestFirstStockMovementConcurrent — barang yang belum punya baris saldo dulu
// tidak terkunci sama sekali: dua gerakan pertama yang bersamaan sama-sama
// menganggap saldo awal 0 lalu saling menimpa cache stok. Checkout di outlet
// yang sama kebetulan terserialisasi oleh kunci nomor struk, jadi balapan ini
// diuji lewat pembelian (barang masuk), yang tidak memakai kunci itu.
func TestFirstStockMovementConcurrent(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "firstmv")

	// Balapan bergantung pada jadwal goroutine, jadi diulang beberapa putaran
	// dengan barang baru tiap putaran — satu putaran saja sering lolos.
	const putaran, n = 8, 12
	for r := 0; r < putaran; r++ {
		baru := call(t, "POST", "/api/v1/products", f.token, map[string]any{
			"name": fmt.Sprintf("Barang Tanpa Saldo %d", r), "unit_id": f.unitID, "sell_price": 1000, "track_stock": true,
		}).mustCode(t, "produk", 201).data(t)["id"].(string)

		var wg sync.WaitGroup
		codes := make([]int, n)
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				req := jsonRequest(t, "POST", "/api/v1/purchases", f.token, map[string]any{
					"outlet_id": f.outletID,
					"items":     []map[string]any{{"product_id": baru, "qty": "1", "unit_cost": 500}},
				})
				req.Header.Set("Idempotency-Key", fmt.Sprintf("FM-%d-%d", r, i))
				codes[i] = serve(t, req).Code
			}(i)
		}
		wg.Wait()
		for i, c := range codes {
			if c != http.StatusCreated {
				t.Fatalf("putaran %d pembelian #%d = %d, mau 201", r, i, c)
			}
		}
		if q := stockQty(t, f.tenantFixture, f.outletID, baru); q != fmt.Sprintf("%d", n) {
			t.Fatalf("putaran %d: stok = %s, mau %d (ada pembaruan yang hilang)", r, q, n)
		}
	}
}

// TestStockDocsPostOnceUnderConcurrency — posting opname dan kirim transfer
// dulu membaca status tanpa mengunci baris dokumennya. Dua klik yang
// bersamaan sama-sama melihat 'draft' dan menulis gerakan stok dua kali.
func TestStockDocsPostOnceUnderConcurrency(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "docrace") // prodA 100

	op := call(t, "POST", "/api/v1/stock-opnames", f.token, map[string]any{"outlet_id": f.outletID}).
		mustCode(t, "opname", 201).data(t)["id"].(string)
	call(t, "POST", "/api/v1/stock-opnames/"+op+"/items", f.token, map[string]any{
		"items": []map[string]any{{"product_id": f.prodA, "counted_qty": "90"}},
	}).mustOK(t, "isi opname")

	berhasil := serentak(t, 6, func() int {
		return call(t, "POST", "/api/v1/stock-opnames/"+op+"/post", f.token, nil).Code
	})
	if berhasil != 1 {
		t.Fatalf("posting opname berhasil %d kali, mau tepat 1", berhasil)
	}
	if q := stockQty(t, f.tenantFixture, f.outletID, f.prodA); q != "90" {
		t.Fatalf("stok setelah opname = %s, mau 90", q)
	}

	tujuan := makeOutlet(t, f.tenantFixture, "DR2")
	tr := call(t, "POST", "/api/v1/stock-transfers", f.token, map[string]any{
		"from_outlet_id": f.outletID, "to_outlet_id": tujuan,
		"items": []map[string]any{{"product_id": f.prodA, "qty": "10"}},
	}).mustCode(t, "transfer", 201).data(t)["id"].(string)

	berhasil = serentak(t, 6, func() int {
		return call(t, "POST", "/api/v1/stock-transfers/"+tr+"/send", f.token, nil).Code
	})
	if berhasil != 1 {
		t.Fatalf("kirim transfer berhasil %d kali, mau tepat 1", berhasil)
	}
	if q := stockQty(t, f.tenantFixture, f.outletID, f.prodA); q != "80" {
		t.Fatalf("stok asal setelah kirim = %s, mau 80", q)
	}
}

// serentak menjalankan fn n kali bersamaan dan mengembalikan berapa yang
// membalas 200.
func serentak(t *testing.T, n int, fn func() int) int {
	t.Helper()
	var wg sync.WaitGroup
	codes := make([]int, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			codes[i] = fn()
		}(i)
	}
	wg.Wait()
	ok := 0
	for _, c := range codes {
		switch c {
		case http.StatusOK:
			ok++
		case http.StatusConflict:
		default:
			t.Fatalf("status tak terduga %d (mau 200 atau 409)", c)
		}
	}
	return ok
}

// TestSyncVisitNeedsVisitPermission — /sync/push dijaga sale.create saja, dan
// operasi visit.upsert di dalamnya dulu tidak memeriksa crm.visit.checkin.
// Kasir biasa bisa menulis kunjungan sales lewat pintu belakang ini; sebaliknya
// sales lapangan tanpa izin berjualan tidak bisa menyinkron kunjungannya sama
// sekali.
func TestSyncVisitNeedsVisitPermission(t *testing.T) {
	requireDB(t)
	f := registerTenant(t, "syncvis")
	cust := makeCustomer(t, f.token, "Toko Uji")

	kasir := staffToken(t, f, roleID(t, f, "Kasir"), "kasir_syncvis")
	res := push(t, kasir, visitOp(ulid.New(), cust, false))
	assertInt(t, res, "rejected", 1)

	role := call(t, "POST", "/api/v1/roles", f.token, map[string]any{
		"name":             "Sales Murni",
		"permission_codes": []string{"crm.visit.checkin", "customer.view"},
	}).mustCode(t, "peran sales", 201).data(t)["id"].(string)
	sales := staffToken(t, f, role, "sales_syncvis")
	res = push(t, sales, visitOp(ulid.New(), cust, false))
	assertInt(t, res, "applied", 1)

	// ...tapi tetap tidak boleh mendorong penjualan.
	pf := setupPOS(t, "syncvis2")
	roleB := call(t, "POST", "/api/v1/roles", pf.token, map[string]any{
		"name":             "Sales Murni",
		"permission_codes": []string{"crm.visit.checkin", "customer.view"},
	}).mustCode(t, "peran sales B", 201).data(t)["id"].(string)
	salesB := staffToken(t, pf.tenantFixture, roleB, "sales_syncvis2")
	res = push(t, salesB, saleOp(pf, 1))
	assertInt(t, res, "rejected", 1)
}

// TestOutletAccessForStaff — user_outlets dulu hanya diisi untuk pemilik saat
// pendaftaran. Staf baru mendapat outlet_ids kosong dari /me (layar kasir web
// tidak punya toko aktif sama sekali), cabang kedua tak pernah muncul bagi
// pemiliknya, dan RequireOutletAccess tidak dipasang di rute mana pun sehingga
// kasir cabang A bebas berjualan di cabang B.
func TestOutletAccessForStaff(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "outacc")
	cabangB := makeOutlet(t, f.tenantFixture, "OB")

	// Pemilik (outlet.manage) melihat semua cabang, termasuk yang baru dibuat.
	me := call(t, "GET", "/api/v1/me", f.token, nil).mustOK(t, "me pemilik").data(t)
	if ids := jsonArray(me["outlet_ids"]); !containsStr(ids, f.outletID) || !containsStr(ids, cabangB) {
		t.Fatalf("outlet_ids pemilik = %v, mau memuat kedua cabang", ids)
	}

	// Staf tanpa outlet_ids eksplisit mendapat seluruh cabang aktif.
	kasirSemua := staffToken(t, f.tenantFixture, roleID(t, f.tenantFixture, "Kasir"), "kasir_outacc_all")
	me = call(t, "GET", "/api/v1/me", kasirSemua, nil).mustOK(t, "me kasir").data(t)
	if ids := jsonArray(me["outlet_ids"]); len(ids) != 2 {
		t.Fatalf("outlet_ids kasir = %v, mau 2 cabang", ids)
	}

	// Staf yang dibatasi ke cabang A.
	call(t, "POST", "/api/v1/users", f.token, map[string]any{
		"name": "Kasir A", "username": "kasir_outacc_a", "email": "kasir_outacc_a@example.com",
		"password": "rahasia123", "role_id": roleID(t, f.tenantFixture, "Kasir"),
		"outlet_ids": []string{f.outletID},
	}).mustCode(t, "buat kasir A", 201)
	kasirA := get[string](t, call(t, "POST", "/api/v1/auth/login", "", map[string]any{
		"username": "kasir_outacc_a", "password": "rahasia123",
	}).mustOK(t, "login kasir A").data(t), "access_token")

	me = call(t, "GET", "/api/v1/me", kasirA, nil).mustOK(t, "me kasir A").data(t)
	if ids := jsonArray(me["outlet_ids"]); len(ids) != 1 || ids[0] != f.outletID {
		t.Fatalf("outlet_ids kasir A = %v, mau hanya [%s]", ids, f.outletID)
	}

	// Cabang sendiri: boleh.
	checkout(t, kasirA, "OA-1", map[string]any{
		"outlet_id": f.outletID,
		"items":     []map[string]any{{"product_id": f.prodA, "qty": "1"}},
		"payments":  []map[string]any{{"method": "cash", "amount": 15000}},
	}).mustCode(t, "jual di cabang sendiri", 201)

	// Cabang lain: ditolak di setiap pintu yang menyentuh uang atau stok.
	call(t, "POST", "/api/v1/shifts/open", kasirA, map[string]any{
		"outlet_id": cabangB, "opening_cash": 0,
	}).mustCode(t, "buka shift cabang lain", 403)
	call(t, "POST", "/api/v1/shifts/open", f.token, map[string]any{
		"outlet_id": cabangB, "opening_cash": 0,
	}).mustCode(t, "pemilik buka shift cabang B", 201)
	checkout(t, kasirA, "OA-2", map[string]any{
		"outlet_id": cabangB,
		"items":     []map[string]any{{"product_id": f.prodA, "qty": "1"}},
		"payments":  []map[string]any{{"method": "cash", "amount": 15000}},
	}).mustCode(t, "jual di cabang lain", 403)
	call(t, "POST", "/api/v1/cash-movements", kasirA, map[string]any{
		"outlet_id": cabangB, "direction": "out", "amount": 1000, "reason": "coba",
	}).mustCode(t, "kas cabang lain", 403)

	// Penjualan cabang B tidak bisa dibatalkan kasir A.
	jualB := checkout(t, f.token, "OA-3", map[string]any{
		"outlet_id": cabangB,
		"items":     []map[string]any{{"product_id": f.prodA, "qty": "1"}},
		"payments":  []map[string]any{{"method": "cash", "amount": 15000}},
	}).mustCode(t, "pemilik jual di B", 201).data(t)
	manajerA := staffToken(t, f.tenantFixture, rolePenuhTanpaOutlet(t, f.tenantFixture), "manajer_outacc_a")
	call(t, "PUT", "/api/v1/users/"+userIDByUsername(t, f.tenantFixture, "manajer_outacc_a"), f.token, map[string]any{
		"outlet_ids": []string{f.outletID},
	}).mustOK(t, "batasi manajer ke A")
	call(t, "POST", "/api/v1/sales/"+jualB["id"].(string)+"/void", manajerA, map[string]any{"reason": "coba"}).
		mustCode(t, "void cabang lain", 403)
}

// rolePenuhTanpaOutlet membuat peran yang boleh membatalkan penjualan tapi
// TIDAK memegang outlet.manage.
func rolePenuhTanpaOutlet(t *testing.T, f tenantFixture) string {
	t.Helper()
	return call(t, "POST", "/api/v1/roles", f.token, map[string]any{
		"name":             "Supervisor Cabang",
		"permission_codes": []string{"sale.create", "sale.void", "sale.refund", "shift.open", "shift.close"},
	}).mustCode(t, "peran supervisor", 201).data(t)["id"].(string)
}

// userIDByUsername mencari id user staf lewat daftar user.
func userIDByUsername(t *testing.T, f tenantFixture, username string) string {
	t.Helper()
	d := call(t, "GET", "/api/v1/users?limit=100", f.token, nil).mustOK(t, "daftar user").data(t)
	for _, it := range d["data"].([]any) {
		if m := it.(map[string]any); m["username"] == username {
			return m["id"].(string)
		}
	}
	t.Fatalf("user %s tidak ditemukan", username)
	return ""
}

// TestResponseTimestampsAreUTC — waktu di response dulu berformat
// "2006-01-02 15:04:05" TANPA zona, dan nilainya bercampur: waktu yang baru
// dibuat (time.Now().UTC()) keluar dalam UTC, waktu yang dibaca ulang dari
// database keluar dalam zona lokal proses. Browser menafsirkan string tanpa
// zona sebagai jam lokal perangkat, jadi di server produksi (UTC) seluruh jam
// di layar WIB mundur 7 jam — termasuk jam di struk.
func TestResponseTimestampsAreUTC(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "tsutc")

	d := checkout(t, f.token, "TS-1", map[string]any{
		"outlet_id": f.outletID,
		"items":     []map[string]any{{"product_id": f.prodA, "qty": "1"}},
		"payments":  []map[string]any{{"method": "cash", "amount": 15000}},
	}).mustCode(t, "checkout", 201).data(t)

	baca := call(t, "GET", "/api/v1/sales/"+d["id"].(string), f.token, nil).mustOK(t, "detail").data(t)
	for _, sumber := range []map[string]any{d, baca} {
		for _, kolom := range []string{"occurred_at", "created_at"} {
			s, _ := sumber[kolom].(string)
			w, err := time.Parse(time.RFC3339, s)
			if err != nil || !strings.HasSuffix(s, "Z") {
				t.Fatalf("%s = %q, mau RFC 3339 UTC berakhiran Z", kolom, s)
			}
			if selisih := time.Since(w); selisih < -time.Minute || selisih > time.Minute {
				t.Fatalf("%s = %s meleset %s dari sekarang", kolom, s, selisih)
			}
		}
	}
	if d["occurred_at"] != baca["occurred_at"] {
		t.Fatalf("occurred_at checkout %v ≠ saat dibaca ulang %v", d["occurred_at"], baca["occurred_at"])
	}
}

// TestPasswordChangeRevokesSessions — mengganti password atau menonaktifkan
// staf dulu tidak memutus sesinya: refresh token lama tetap bisa ditukar
// sampai 30 hari.
func TestPasswordChangeRevokesSessions(t *testing.T) {
	requireDB(t)
	f := registerTenant(t, "revsesi")

	call(t, "POST", "/api/v1/users", f.token, map[string]any{
		"name": "Kasir", "username": "kasir_revsesi", "email": "kasir_revsesi@example.com",
		"password": "rahasia123", "role_id": roleID(t, f, "Kasir"),
	}).mustCode(t, "buat staf", 201)
	sesi := call(t, "POST", "/api/v1/auth/login", "", map[string]any{
		"username": "kasir_revsesi", "password": "rahasia123",
	}).mustOK(t, "login").data(t)
	refresh := get[string](t, sesi, "refresh_token")
	uid := userIDByUsername(t, f, "kasir_revsesi")

	call(t, "PUT", "/api/v1/users/"+uid, f.token, map[string]any{"password": "gantibaru123"}).
		mustOK(t, "ganti password")
	call(t, "POST", "/api/v1/auth/refresh", "", map[string]any{"refresh_token": refresh}).
		mustCode(t, "refresh setelah ganti password", 401)
}

// TestBackfillUserOutletsMigration — SQL migrasi 000036 memberi staf lama yang
// belum punya akses cabang SELURUH cabang aktif, tanpa menyentuh user yang
// sudah punya akses.
func TestBackfillUserOutletsMigration(t *testing.T) {
	requireDB(t)
	f := registerTenant(t, "backfill")
	cabangB := makeOutlet(t, f, "BF2")

	call(t, "POST", "/api/v1/users", f.token, map[string]any{
		"name": "Staf Lama", "username": "staf_backfill", "email": "staf_backfill@example.com",
		"password": "rahasia123", "role_id": roleID(t, f, "Kasir"),
	}).mustCode(t, "buat staf", 201)
	staf := userIDByUsername(t, f, "staf_backfill")

	// Tiru keadaan sebelum perbaikan: staf tanpa baris user_outlets.
	if err := database.DB.Exec("DELETE FROM user_outlets WHERE user_id = ?", staf).Error; err != nil {
		t.Fatal(err)
	}
	sqlUp, err := migrations.FS.ReadFile("000036_backfill_user_outlets.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if err := database.DB.Exec(string(sqlUp)).Error; err != nil {
		t.Fatalf("SQL backfill gagal: %v", err)
	}

	var jumlahStaf, jumlahPemilik int64
	database.DB.Raw("SELECT count(*) FROM user_outlets WHERE user_id = ?", staf).Scan(&jumlahStaf)
	database.DB.Raw("SELECT count(*) FROM user_outlets WHERE user_id = ?", f.ownerID).Scan(&jumlahPemilik)
	if jumlahStaf != 2 {
		t.Fatalf("akses staf setelah backfill = %d cabang, mau 2 (termasuk %s)", jumlahStaf, cabangB)
	}
	if jumlahPemilik != 1 {
		t.Fatalf("akses pemilik berubah jadi %d, mau tetap 1 (sudah punya akses, tak disentuh)", jumlahPemilik)
	}
}

// TestOpnameRecountReplacesItem — hitungan ulang barang yang sama dulu
// MENAMBAH baris opname, bukan menggantinya. Unique (…, product_id,
// variant_id) tidak pernah bentrok untuk barang tanpa varian karena
// variant_id-nya NULL, dan NULL tidak sama dengan NULL. Saat diposting kedua
// selisihnya diterapkan: hitung 95 lalu dikoreksi 97 berakhir di stok 92.
func TestOpnameRecountReplacesItem(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "recount") // prodA 100

	op := call(t, "POST", "/api/v1/stock-opnames", f.token, map[string]any{"outlet_id": f.outletID}).
		mustCode(t, "opname", 201).data(t)["id"].(string)
	for _, hitung := range []string{"95", "97"} {
		call(t, "POST", "/api/v1/stock-opnames/"+op+"/items", f.token, map[string]any{
			"items": []map[string]any{{"product_id": f.prodA, "counted_qty": hitung}},
		}).mustOK(t, "hitung "+hitung)
	}
	d := call(t, "GET", "/api/v1/stock-opnames/"+op, f.token, nil).mustOK(t, "detail opname").data(t)
	if n := len(d["items"].([]any)); n != 1 {
		t.Fatalf("baris opname = %d, mau 1 (hitungan ulang menggantikan)", n)
	}

	// Barang yang sama dua kali dalam SATU kiriman: yang terakhir berlaku,
	// bukan galat 500 dari upsert yang menyentuh baris sama dua kali.
	d = call(t, "POST", "/api/v1/stock-opnames/"+op+"/items", f.token, map[string]any{
		"items": []map[string]any{
			{"product_id": f.prodB, "counted_qty": "40"},
			{"product_id": f.prodB, "counted_qty": "45"},
		},
	}).mustOK(t, "hitung ganda satu kiriman").data(t)
	if n := len(d["items"].([]any)); n != 2 {
		t.Fatalf("baris opname = %d, mau 2 (A dan B)", n)
	}

	call(t, "POST", "/api/v1/stock-opnames/"+op+"/post", f.token, nil).mustOK(t, "posting")
	if q := stockQty(t, f.tenantFixture, f.outletID, f.prodA); q != "97" {
		t.Fatalf("stok setelah opname = %s, mau 97 (hitungan terakhir)", q)
	}
	if q := stockQty(t, f.tenantFixture, f.outletID, f.prodB); q != "45" {
		t.Fatalf("stok B setelah opname = %s, mau 45 (hitungan terakhir dalam kiriman)", q)
	}
}

// TestProductImageURLCannotPointAtUploads — image_url dulu boleh diisi bebas
// lewat form barang. Tenant A bisa mengarahkannya ke berkas foto milik tenant
// B, lalu "hapus foto" menghapus berkas B dari disk.
func TestProductImageURLCannotPointAtUploads(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "imgurl")

	call(t, "PUT", "/api/v1/products/"+f.prodA, f.token, map[string]any{
		"image_url": "/uploads/barang/01ZZZZZZZZZZZZZZZZZZZZZZZZ.jpg",
	}).mustCode(t, "arahkan ke berkas unggahan", 400)
	call(t, "POST", "/api/v1/products", f.token, map[string]any{
		"name": "Barang Foto Curian", "unit_id": f.unitID, "sell_price": 1000,
		"image_url": "/uploads/barang/01ZZZZZZZZZZZZZZZZZZZZZZZZ.jpg",
	}).mustCode(t, "buat dengan berkas unggahan", 400)

	// URL luar tetap boleh.
	call(t, "PUT", "/api/v1/products/"+f.prodA, f.token, map[string]any{
		"image_url": "https://cdn.example.com/kopi.jpg",
	}).mustOK(t, "url luar")
}

// TestReceivablePaymentIdempotent — setoran kasbon dulu tanpa Idempotency-Key:
// tombol yang ditekan dua kali, atau dikirim ulang setelah sinyal putus,
// mencatat cicilan DUA kali selama sisanya masih cukup.
func TestReceivablePaymentIdempotent(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "recidem")
	cust := makeCustomer(t, f.token, "Pelanggan Kasbon")

	checkout(t, f.token, "RI-1", map[string]any{
		"outlet_id": f.outletID, "customer_id": cust,
		"items":    []map[string]any{{"product_id": f.prodA, "qty": "2"}}, // 30000
		"payments": []map[string]any{{"method": "credit", "amount": 30000}},
	}).mustCode(t, "checkout kasbon", 201)
	recID := call(t, "GET", "/api/v1/receivables?customer_id="+cust, f.token, nil).
		mustOK(t, "piutang").data(t)["data"].([]any)[0].(map[string]any)["id"].(string)

	setor := map[string]any{"receivable_id": recID, "amount": 10000, "method": "cash"}
	call(t, "POST", "/api/v1/receivable-payments", f.token, setor).
		mustCode(t, "tanpa Idempotency-Key", 422)

	r1 := checkoutLike(t, f.token, "SETOR-1", "/api/v1/receivable-payments", setor).
		mustCode(t, "setor #1", 201).data(t)
	r2 := checkoutLike(t, f.token, "SETOR-1", "/api/v1/receivable-payments", setor).
		mustCode(t, "setor #1 dikirim ulang", 201).data(t)
	if r1["paid_amount"] != r2["paid_amount"] {
		t.Fatalf("balasan kiriman ulang berbeda: %v vs %v", r1["paid_amount"], r2["paid_amount"])
	}
	d := call(t, "GET", "/api/v1/receivables/"+recID, f.token, nil).mustOK(t, "detail piutang").data(t)
	assertI64(t, d, "paid_amount", 10000)
	assertI64(t, d, "outstanding", 20000)

	// Kunci sama, isi berbeda → 409.
	setor["amount"] = 5000
	checkoutLike(t, f.token, "SETOR-1", "/api/v1/receivable-payments", setor).
		mustCode(t, "kunci sama isi beda", 409)
}

// TestStockAdjustIdempotent — "tambah 10" yang terkirim dua kali dulu
// menambah 20.
func TestStockAdjustIdempotent(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "adjidem") // prodA 100

	geser := map[string]any{"outlet_id": f.outletID, "product_id": f.prodA, "delta": "10", "reason": "temuan gudang"}
	call(t, "POST", "/api/v1/stock-adjustments", f.token, geser).
		mustCode(t, "tanpa Idempotency-Key", 422)
	for i := 0; i < 2; i++ {
		checkoutLike(t, f.token, "GESER-1", "/api/v1/stock-adjustments", geser).
			mustCode(t, fmt.Sprintf("geser #%d", i+1), 201)
	}
	if q := stockQty(t, f.tenantFixture, f.outletID, f.prodA); q != "110" {
		t.Fatalf("stok = %s, mau 110 (kiriman ulang tidak boleh menambah lagi)", q)
	}
}

// TestRebuildSummariesNeedsOutletManage — bangun ulang ringkasan dulu cukup
// dengan izin BACA laporan, padahal ia menulis ulang tabel laporan untuk
// rentang sampai 366 hari di semua cabang.
func TestRebuildSummariesNeedsOutletManage(t *testing.T) {
	requireDB(t)
	f := registerTenant(t, "rebuildperm")
	role := call(t, "POST", "/api/v1/roles", f.token, map[string]any{
		"name": "Pembaca Laporan", "permission_codes": []string{"report.view"},
	}).mustCode(t, "peran", 201).data(t)["id"].(string)
	pembaca := staffToken(t, f, role, "pembaca_rebuild")

	hari := time.Now().UTC().Format("2006-01-02")
	call(t, "GET", "/api/v1/reports/dashboard", pembaca, nil).mustOK(t, "baca laporan tetap boleh")
	call(t, "POST", "/api/v1/reports/rebuild-summaries?from="+hari+"&to="+hari, pembaca, nil).
		mustCode(t, "rebuild tanpa outlet.manage", 403)
	call(t, "POST", "/api/v1/reports/rebuild-summaries?from="+hari+"&to="+hari, f.token, nil).
		mustOK(t, "pemilik boleh rebuild")
}

// TestOutletReadScope — batas cabang dulu hanya ada di jalur TULIS: kasir
// cabang A tetap bisa membaca penjualan, shift, stok, dan laporan cabang B.
func TestOutletReadScope(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "readscope") // cabang A: shift terbuka, stok A/B
	cabangB := makeOutlet(t, f.tenantFixture, "RB")

	// Satu penjualan di tiap cabang (oleh pemilik).
	jualA := checkout(t, f.token, "RS-A", map[string]any{
		"outlet_id": f.outletID,
		"items":     []map[string]any{{"product_id": f.prodA, "qty": "1"}},
		"payments":  []map[string]any{{"method": "cash", "amount": 15000}},
	}).mustCode(t, "jual di A", 201).data(t)
	shiftB := call(t, "POST", "/api/v1/shifts/open", f.token, map[string]any{
		"outlet_id": cabangB, "opening_cash": 0,
	}).mustCode(t, "buka shift B", 201).data(t)["id"].(string)
	jualB := checkout(t, f.token, "RS-B", map[string]any{
		"outlet_id": cabangB,
		"items":     []map[string]any{{"product_id": f.prodB, "qty": "2"}},
		"payments":  []map[string]any{{"method": "cash", "amount": 16000}},
	}).mustCode(t, "jual di B", 201).data(t)
	hari := jualA["business_date"].(string)

	// Kepala cabang A: boleh berjualan & membaca laporan, TANPA outlet.manage.
	role := call(t, "POST", "/api/v1/roles", f.token, map[string]any{
		"name": "Kepala Cabang",
		"permission_codes": []string{
			"sale.create", "shift.open", "shift.close", "stock.view", "report.view",
		},
	}).mustCode(t, "peran", 201).data(t)["id"].(string)
	call(t, "POST", "/api/v1/users", f.token, map[string]any{
		"name": "Kepala A", "username": "kepala_readscope", "email": "kepala_readscope@example.com",
		"password": "rahasia123", "role_id": role, "outlet_ids": []string{f.outletID},
	}).mustCode(t, "buat kepala cabang", 201)
	kepala := get[string](t, call(t, "POST", "/api/v1/auth/login", "", map[string]any{
		"username": "kepala_readscope", "password": "rahasia123",
	}).mustOK(t, "login").data(t), "access_token")

	// /me membawa rincian cabangnya (pajak dsb.) untuk total kasir.
	me := call(t, "GET", "/api/v1/me", kepala, nil).mustOK(t, "me").data(t)
	outlets := me["outlets"].([]any)
	if len(outlets) != 1 || outlets[0].(map[string]any)["id"] != f.outletID {
		t.Fatalf("outlets di /me = %v, mau hanya cabang A", outlets)
	}
	if _, ada := outlets[0].(map[string]any)["tax_enabled"]; !ada {
		t.Fatal("rincian cabang di /me tidak memuat pengaturan pajak")
	}

	// Daftar penjualan tanpa filter: hanya cabang A.
	daftar := call(t, "GET", "/api/v1/sales?limit=100", kepala, nil).mustOK(t, "daftar jual").data(t)
	for _, it := range daftar["data"].([]any) {
		if o := it.(map[string]any)["outlet_id"]; o != f.outletID {
			t.Fatalf("kepala cabang A melihat penjualan cabang %v", o)
		}
	}
	// Meminta cabang B terang-terangan: kosong, bukan bocor.
	d := call(t, "GET", "/api/v1/sales?outlet_id="+cabangB, kepala, nil).mustOK(t, "jual B").data(t)
	if n := len(d["data"].([]any)); n != 0 {
		t.Fatalf("filter cabang B mengembalikan %d penjualan, mau 0", n)
	}
	// Detail penjualan & shift cabang B: seperti tidak ada.
	call(t, "GET", "/api/v1/sales/"+jualB["id"].(string), kepala, nil).mustCode(t, "detail jual B", 404)
	call(t, "GET", "/api/v1/sales/"+jualA["id"].(string), kepala, nil).mustOK(t, "detail jual A")
	call(t, "GET", "/api/v1/shifts/"+shiftB, kepala, nil).mustCode(t, "detail shift B", 404)
	sh := call(t, "GET", "/api/v1/shifts?limit=100", kepala, nil).mustOK(t, "daftar shift").data(t)
	for _, it := range sh["data"].([]any) {
		if o := it.(map[string]any)["outlet_id"]; o != f.outletID {
			t.Fatalf("kepala cabang A melihat shift cabang %v", o)
		}
	}

	// Stok: hanya baris cabang A.
	st := call(t, "GET", "/api/v1/stocks?limit=100", kepala, nil).mustOK(t, "stok").data(t)
	for _, it := range st["data"].([]any) {
		if o := it.(map[string]any)["outlet_id"]; o != f.outletID {
			t.Fatalf("kepala cabang A melihat stok cabang %v", o)
		}
	}

	// Laporan tanpa filter: hanya omzet cabang A (15000), bukan A+B (31000).
	dash := call(t, "GET", "/api/v1/reports/dashboard?date="+hari, kepala, nil).mustOK(t, "dashboard").data(t)
	assertI64(t, dash["today"].(map[string]any), "net_amount", 15000)
	dashPemilik := call(t, "GET", "/api/v1/reports/dashboard?date="+hari, f.token, nil).mustOK(t, "dashboard pemilik").data(t)
	assertI64(t, dashPemilik["today"].(map[string]any), "net_amount", 31000)

	// Petugas gudang (stock.transfer, tanpa outlet.manage) bisa melihat daftar
	// cabang untuk memilih tujuan kiriman.
	gudang := staffToken(t, f.tenantFixture, roleID(t, f.tenantFixture, "Gudang"), "gudang_readscope")
	cabang := call(t, "GET", "/api/v1/outlets", gudang, nil).mustOK(t, "daftar cabang gudang").data(t)
	if n := len(cabang["data"].([]any)); n != 2 {
		t.Fatalf("gudang melihat %d cabang, mau 2", n)
	}
	call(t, "POST", "/api/v1/outlets", gudang, map[string]any{"name": "X"}).
		mustCode(t, "gudang tetap tak boleh menambah cabang", 403)
}
