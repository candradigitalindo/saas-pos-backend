package tests

import (
	"strings"
	"testing"
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
