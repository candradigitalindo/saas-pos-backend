package tests

import (
	"testing"

	"candra/backend-api/internal/ulid"
)

// Harga grosir per jumlah: beli minimal N (total barang itu dalam satu
// transaksi, varian dijumlah), harga satuannya turun.
//
// Yang dijaga: tingkat disimpan & dibaca urut jumlah; checkout memilih tingkat
// terbesar yang terpenuhi; varian menambah selisih di atas harga grosir;
// dikosongkan → harga jual lagi; validasi; ikut tersinkron ke perangkat.
func TestHargaGrosir(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "grosir")
	tingkat := []map[string]any{{"min_qty": "12", "price": 9000}, {"min_qty": "6", "price": 9500}}
	p := call(t, "POST", "/api/v1/products", f.token, map[string]any{
		"name": "Air Galon", "unit_id": f.unitID, "sell_price": 10000, "cost_price": 7000, "track_stock": false,
		"wholesale_prices": tingkat,
	}).mustCode(t, "buat barang grosir", 201).data(t)
	id := p["id"].(string)
	if w := p["wholesale_prices"].([]any); len(w) != 2 || w[0].(map[string]any)["min_qty"] != "6" {
		t.Fatalf("tingkat di balasan buat: %v", w)
	}
	if w := call(t, "GET", "/api/v1/products/"+id, f.token, nil).mustOK(t, "ambil").data(t)["wholesale_prices"].([]any); len(w) != 2 {
		t.Fatalf("tingkat di GET: %v", w)
	}

	harga := func(items ...map[string]any) []any {
		t.Helper()
		return checkout(t, f.token, ulid.New(), map[string]any{
			"outlet_id": f.outletID, "items": items,
			"payments": []map[string]any{{"method": "cash", "amount": 1000000}},
		}).mustCode(t, "jual", 201).data(t)["items"].([]any)
	}
	satuan := func(it any) float64 { return it.(map[string]any)["unit_price"].(float64) }
	for _, k := range []struct {
		qty  string
		mau  float64
		nama string
	}{{"5", 10000, "di bawah tingkat"}, {"6", 9500, "tingkat 6"}, {"11", 9500, "masih tingkat 6"}, {"12", 9000, "tingkat 12"}} {
		if got := satuan(harga(map[string]any{"product_id": id, "qty": k.qty})[0]); got != k.mau {
			t.Fatalf("%s: qty %s → %v, mau %v", k.nama, k.qty, got, k.mau)
		}
	}

	// Varian: 6 Besar + 6 Kecil = 12 → dasar 9.000, selisih varian tetap ditambah.
	besar := call(t, "POST", "/api/v1/products/"+id+"/variants", f.token, map[string]any{"name": "Besar", "price_delta": 2000}).
		mustCode(t, "varian", 201).data(t)["id"]
	kecil := call(t, "POST", "/api/v1/products/"+id+"/variants", f.token, map[string]any{"name": "Kecil", "price_delta": 0}).
		mustCode(t, "varian 2", 201).data(t)["id"]
	baris := harga(
		map[string]any{"product_id": id, "variant_id": besar, "qty": "6"},
		map[string]any{"product_id": id, "variant_id": kecil, "qty": "6"},
	)
	if satuan(baris[0]) != 11000 || satuan(baris[1]) != 9000 {
		t.Fatalf("grosir + varian: %v / %v", satuan(baris[0]), satuan(baris[1]))
	}

	// Validasi.
	for _, salah := range [][]map[string]any{
		{{"min_qty": "1", "price": 9000}},
		{{"min_qty": "6", "price": 9000}, {"min_qty": "6.000", "price": 8000}},
		{{"min_qty": "abc", "price": 9000}},
	} {
		call(t, "PUT", "/api/v1/products/"+id, f.token, map[string]any{"wholesale_prices": salah}).mustCode(t, "tingkat salah", 422)
	}

	// Dikosongkan → harga jual lagi; mengubah kolom lain tidak menyentuh tingkat.
	call(t, "PUT", "/api/v1/products/"+id, f.token, map[string]any{"name": "Air Galon 19L"}).mustOK(t, "ubah nama")
	if satuan(harga(map[string]any{"product_id": id, "qty": "12"})[0]) != 9000 {
		t.Fatal("mengubah nama menghapus tingkat grosir")
	}
	call(t, "PUT", "/api/v1/products/"+id, f.token, map[string]any{"wholesale_prices": []any{}}).mustOK(t, "kosongkan")
	if satuan(harga(map[string]any{"product_id": id, "qty": "12"})[0]) != 10000 {
		t.Fatal("tingkat yang dikosongkan masih dipakai")
	}

	// Tersinkron ke perangkat (pratinjau keranjang offline).
	call(t, "PUT", "/api/v1/products/"+id, f.token, map[string]any{"wholesale_prices": tingkat}).mustOK(t, "isi lagi")
	pull := call(t, "GET", "/api/v1/sync/pull?since=0&outlet_id="+f.outletID, f.token, nil).mustOK(t, "pull").data(t)
	if pl := pull["price_lists"].([]any); len(pl) != 1 || pl[0].(map[string]any)["is_default"] != true {
		t.Fatalf("daftar harga di pull: %v", pl)
	}
	n := 0
	for _, r := range pull["product_prices"].([]any) {
		if r.(map[string]any)["product_id"] == id {
			n++
		}
	}
	if n != 2 {
		t.Fatalf("harga grosir di pull = %d, mau 2", n)
	}
}
