package tests

import (
	"strings"
	"testing"

	"candra/backend-api/database"
)

// Uji ringkasan stok (GET /stocks/summary) dan penyaring daftar stok (?q=,
// ?product_ids=).
//
// Ringkasan dihitung di SERVER atas seluruh barang, bukan dari halaman yang
// kebetulan termuat: toko dengan 300 barang hanya menerima 100 baris per
// halaman, dan "5 barang habis" yang dihitung dari halaman pertama saja adalah
// angka yang salah dengan wajah meyakinkan.
func TestRingkasanDanPenyaringStok(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "stokringkas") // A: 100 (modal 6.000), B: 50 (modal 3.000)
	unit := makeUnit(t, f.tenantFixture, "botol")

	barang := func(nama string, modal int64, min string) string {
		t.Helper()
		return call(t, "POST", "/api/v1/products", f.token, map[string]any{
			"name": nama, "unit_id": unit, "sell_price": modal * 2, "cost_price": modal,
			"track_stock": true, "min_stock": min,
		}).mustCode(t, "barang "+nama, 201).data(t)["id"].(string)
	}
	tipis := barang("Kecap Tipis", 4000, "5")
	habis := barang("Saus Habis", 2000, "0")
	minus := barang("Sirup Minus", 5000, "0")

	adjust(t, f.tenantFixture, f.outletID, tipis, "3", "saldo awal") // hampir habis (3 ≤ 5)
	adjust(t, f.tenantFixture, f.outletID, habis, "0", "saldo awal") // habis
	adjust(t, f.tenantFixture, f.outletID, minus, "0", "saldo awal")
	// Penjualan tidak pernah diblokir stok — saldo boleh minus dan harus
	// terhitung sebagai "perlu dicocokkan".
	checkout(t, f.token, "SR-1", map[string]any{
		"outlet_id": f.outletID,
		"items":     []map[string]any{{"product_id": minus, "qty": "2"}},
		"payments":  []map[string]any{{"method": "cash", "amount": 20000}},
	}).mustCode(t, "jual sampai minus", 201)

	r := call(t, "GET", "/api/v1/stocks/summary?outlet_id="+f.outletID, f.token, nil).
		mustOK(t, "ringkasan stok").data(t)
	assertI64(t, r, "total", 5)
	assertI64(t, r, "safe", 2) // A (100) & B (50), min_stock 0
	assertI64(t, r, "low", 1)
	assertI64(t, r, "out", 1)
	assertI64(t, r, "negative", 1)
	// Nilai stok (harga modal), saldo minus dihitung nol — bukan mengurangi:
	// 100×6.000 + 50×3.000 + 3×4.000 = 762.000
	assertI64(t, r, "stock_value", 762000)

	// ?q= menyaring menurut nama barang, tanpa peduli huruf besar.
	cari := call(t, "GET", "/api/v1/stocks?outlet_id="+f.outletID+"&q=kecap", f.token, nil).
		mustOK(t, "cari stok").data(t)["data"].([]any)
	if len(cari) != 1 || cari[0].(map[string]any)["product_id"] != tipis {
		t.Fatalf("cari \"kecap\": %v, mau hanya Kecap Tipis", cari)
	}

	// ?product_ids= mengambil saldo barang-barang tertentu saja — dipakai
	// daftar Barang untuk kolom stok halaman yang sedang tampil.
	pilih := call(t, "GET", "/api/v1/stocks?outlet_id="+f.outletID+"&product_ids="+
		strings.Join([]string{f.prodA, habis}, ","), f.token, nil).
		mustOK(t, "stok terpilih").data(t)["data"].([]any)
	if len(pilih) != 2 {
		t.Fatalf("product_ids: %d baris, mau 2", len(pilih))
	}

	// Ringkasan tunduk pada izin yang sama dengan daftar stok (stock.view).
	rid := call(t, "POST", "/api/v1/roles", f.token, map[string]any{
		"name": "Tanpa Stok", "permission_codes": []string{"customer.view"},
	}).mustCode(t, "buat peran", 201).data(t)["id"].(string)
	tanpaStok := staffToken(t, f.tenantFixture, rid, "tanpastok_ringkas")
	call(t, "GET", "/api/v1/stocks/summary?outlet_id="+f.outletID, tanpaStok, nil).
		mustCode(t, "ringkasan tanpa stock.view", 403)
}

// Daftar stok yang informatif: saring per keadaan (?status=), urutan
// (?sort=), dan per baris nilai stok, laku 30 hari, serta terakhir terjual.
//
// Batas tiap ?status= harus SAMA dengan hitungan ringkasan: kartu "Habis 3"
// yang ditekan lalu menampilkan 4 baris merusak kepercayaan pada kedua angka.
func TestDaftarStokInformatif(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "stokinfo") // A: 100 (modal 6.000), B: 50 (modal 3.000)
	unit := makeUnit(t, f.tenantFixture, "botol")
	kat := makeCategory(t, f.tenantFixture, "Bumbu")

	barang := func(nama string, modal int64, min string) string {
		t.Helper()
		return call(t, "POST", "/api/v1/products", f.token, map[string]any{
			"name": nama, "unit_id": unit, "category_id": kat, "sku": "SKU-" + nama[:3],
			"sell_price": modal * 2, "cost_price": modal, "track_stock": true, "min_stock": min,
		}).mustCode(t, "barang "+nama, 201).data(t)["id"].(string)
	}
	tipis := barang("Kecap Tipis", 4000, "5")
	habis := barang("Saus Habis", 2000, "0")
	minus := barang("Sirup Minus", 5000, "0")
	lama := barang("Teh Lama", 2500, "0")

	adjust(t, f.tenantFixture, f.outletID, tipis, "3", "saldo awal")
	adjust(t, f.tenantFixture, f.outletID, habis, "0", "saldo awal")
	adjust(t, f.tenantFixture, f.outletID, minus, "0", "saldo awal")
	adjust(t, f.tenantFixture, f.outletID, lama, "10", "saldo awal")

	jual := func(kunci, produk, qty string, bayar int) string {
		t.Helper()
		return checkout(t, f.token, kunci, map[string]any{
			"outlet_id": f.outletID,
			"items":     []map[string]any{{"product_id": produk, "qty": qty}},
			"payments":  []map[string]any{{"method": "cash", "amount": bayar}},
		}).mustCode(t, "jual "+kunci, 201).data(t)["id"].(string)
	}
	jual("SI-A", f.prodA, "6", 90000)
	jual("SI-M", minus, "2", 20000)
	jual("SI-T", lama, "1", 5000)
	// Penjualan yang dibatalkan tidak dihitung laku.
	batal := jual("SI-B", f.prodB, "4", 40000)
	call(t, "POST", "/api/v1/sales/"+batal+"/void", f.token, map[string]any{"reason": "salah input"}).mustOK(t, "void B")
	// Teh Lama: masuk & terakhir laku 40 hari lalu → modal tertahan.
	if err := database.DB.Exec(`UPDATE stock_movements SET occurred_at = now() - interval '40 days'
		WHERE tenant_id = ? AND product_id = ?`, f.tenantID, lama).Error; err != nil {
		t.Fatal(err)
	}

	daftar := func(query string) []map[string]any {
		t.Helper()
		var out []map[string]any
		for _, x := range call(t, "GET", "/api/v1/stocks?outlet_id="+f.outletID+query, f.token, nil).
			mustOK(t, "stok "+query).data(t)["data"].([]any) {
			out = append(out, x.(map[string]any))
		}
		return out
	}
	ids := func(rows []map[string]any) []string {
		var out []string
		for _, r := range rows {
			out = append(out, r["product_id"].(string))
		}
		return out
	}

	// Saring per keadaan — sama dengan ringkasan.
	r := call(t, "GET", "/api/v1/stocks/summary?outlet_id="+f.outletID, f.token, nil).mustOK(t, "ringkasan").data(t)
	for status, mau := range map[string][]string{
		"negative": {minus},
		"out":      {habis},
		"low":      {tipis},
		"idle":     {lama},
	} {
		if got := ids(daftar("&status=" + status)); strings.Join(got, ",") != strings.Join(mau, ",") {
			t.Fatalf("status=%s: %v, mau %v", status, got, mau)
		}
	}
	if n := len(daftar("&status=safe")); int64(n) != int64(r["safe"].(float64)) || n != 3 {
		t.Fatalf("status=safe: %d baris, ringkasan %v, mau 3 (A, B, Teh Lama)", n, r["safe"])
	}
	assertI64(t, r, "idle", 1)
	assertI64(t, r, "idle_value", 22500) // 9 × 2.500

	// Urutan mendesak: minus → habis → hampir habis → sisanya.
	if got := ids(daftar("&sort=urgent"))[:3]; strings.Join(got, ",") != strings.Join([]string{minus, habis, tipis}, ",") {
		t.Fatalf("sort=urgent: %v", got)
	}
	if got := daftar("&sort=value")[0]["product_id"]; got != f.prodA { // 94 × 6.000
		t.Fatalf("sort=value pertama: %v, mau A", got)
	}
	if got := daftar("&sort=sold")[0]["product_id"]; got != f.prodA {
		t.Fatalf("sort=sold pertama: %v, mau A", got)
	}

	// Isi baris.
	per := map[string]map[string]any{}
	for _, b := range daftar("") {
		per[b["product_id"].(string)] = b
	}
	a := per[f.prodA]
	if a["sold_30d"] != "6" || a["last_sold_at"] == nil || a["cost_price"] != float64(6000) {
		t.Fatalf("baris A: %v", a)
	}
	assertI64(t, a, "stock_value", 564000)
	if b := per[f.prodB]; b["sold_30d"] != "0" {
		t.Fatalf("B dibatalkan, laku mau 0: %v", b["sold_30d"])
	}
	if m := per[minus]; m["sold_30d"] != "2" || m["stock_value"] != float64(0) {
		t.Fatalf("baris minus: laku %v nilai %v, mau 2 & 0", m["sold_30d"], m["stock_value"])
	}
	if k := per[tipis]; k["category_name"] != "Bumbu" || k["sku"] != "SKU-Kec" {
		t.Fatalf("baris kecap: kategori %v sku %v", k["category_name"], k["sku"])
	}
	if l := per[lama]; l["sold_30d"] != "0" || l["last_sold_at"] == nil {
		t.Fatalf("baris teh lama: %v", l)
	}

	call(t, "GET", "/api/v1/stocks?outlet_id="+f.outletID+"&status=hilang", f.token, nil).mustCode(t, "status asing", 400)
	call(t, "GET", "/api/v1/stocks?outlet_id="+f.outletID+"&sort=acak", f.token, nil).mustCode(t, "sort asing", 400)
}
