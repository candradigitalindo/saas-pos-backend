package tests

import (
	"testing"

	"candra/backend-api/internal/ulid"
)

// Harga khusus pelanggan (member / reseller).
//
// Yang dijaga: daftar harga khusus dikelola (nama unik; yang default tidak
// tampil & tidak bisa dipakai sebagai harga khusus); harga khusus disimpan
// per barang; checkout atas nama pelanggan berdaftar harga memakai harga
// khususnya, pelanggan lain & tanpa pelanggan tidak; barang tanpa harga
// khusus tetap memakai harga umum + grosir; daftar yang dihapus diabaikan.
func TestHargaKhusus(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "khusus") // prodA 15.000, prodB 8.000

	member := call(t, "POST", "/api/v1/price-lists", f.token, map[string]any{"name": "Member"}).
		mustCode(t, "buat daftar", 201).data(t)["id"].(string)
	call(t, "POST", "/api/v1/price-lists", f.token, map[string]any{"name": " member "}).mustCode(t, "nama ganda", 409)

	// Harga khusus prodA untuk Member; prodB hanya punya harga grosir ≥6.
	call(t, "PUT", "/api/v1/products/"+f.prodA, f.token, map[string]any{
		"special_prices": []map[string]any{{"price_list_id": member, "price": 12000}},
	}).mustOK(t, "harga khusus")
	call(t, "PUT", "/api/v1/products/"+f.prodB, f.token, map[string]any{
		"wholesale_prices": []map[string]any{{"min_qty": "6", "price": 7000}},
	}).mustOK(t, "grosir B")
	if s := call(t, "GET", "/api/v1/products/"+f.prodA, f.token, nil).mustOK(t, "ambil A").data(t)["special_prices"].([]any); len(s) != 1 {
		t.Fatalf("harga khusus di GET: %v", s)
	}
	// Daftar default (tempat harga grosir) tidak tampil & tidak bisa dipakai.
	daftar := call(t, "GET", "/api/v1/price-lists", f.token, nil).mustOK(t, "daftar").Body["data"].([]any)
	if len(daftar) != 1 {
		t.Fatalf("daftar harga = %v, mau hanya Member", daftar)
	}
	pull := call(t, "GET", "/api/v1/sync/pull?since=0&outlet_id="+f.outletID, f.token, nil).mustOK(t, "pull").data(t)
	var bawaan string
	for _, pl := range pull["price_lists"].([]any) {
		if m := pl.(map[string]any); m["is_default"] == true {
			bawaan = m["id"].(string)
		}
	}
	call(t, "PUT", "/api/v1/products/"+f.prodA, f.token, map[string]any{
		"special_prices": []map[string]any{{"price_list_id": bawaan, "price": 1}},
	}).mustCode(t, "daftar default bukan harga khusus", 422)

	// Pelanggan.
	pelanggan := func(nama string, daftar string) string {
		isi := map[string]any{"name": nama}
		if daftar != "" {
			isi["price_list_id"] = daftar
		}
		return call(t, "POST", "/api/v1/customers", f.token, isi).mustCode(t, "pelanggan "+nama, 201).data(t)["id"].(string)
	}
	rina := pelanggan("Bu Rina Member", member)
	andi := pelanggan("Pak Andi Umum", "")
	call(t, "POST", "/api/v1/customers", f.token, map[string]any{"name": "Salah", "price_list_id": ulid.New()}).
		mustCode(t, "daftar tak dikenal", 422)
	if d := call(t, "GET", "/api/v1/customers/"+rina, f.token, nil).mustOK(t, "ambil rina").data(t); d["price_list_id"] != member {
		t.Fatalf("price_list_id pelanggan: %v", d)
	}

	harga := func(cust string, prod string, qty string) float64 {
		t.Helper()
		isi := map[string]any{
			"outlet_id": f.outletID, "items": []map[string]any{{"product_id": prod, "qty": qty}},
			"payments": []map[string]any{{"method": "cash", "amount": 500000}},
		}
		if cust != "" {
			isi["customer_id"] = cust
		}
		it := checkout(t, f.token, ulid.New(), isi).mustCode(t, "jual", 201).data(t)["items"].([]any)[0]
		return it.(map[string]any)["unit_price"].(float64)
	}
	if h := harga(rina, f.prodA, "1"); h != 12000 {
		t.Fatalf("member membeli A: %v, mau 12.000", h)
	}
	if h := harga(andi, f.prodA, "1"); h != 15000 {
		t.Fatalf("pelanggan umum membeli A: %v", h)
	}
	if h := harga("", f.prodA, "1"); h != 15000 {
		t.Fatalf("tanpa pelanggan membeli A: %v", h)
	}
	// Barang tanpa harga khusus → harga umum + grosir, juga untuk member.
	if h := harga(rina, f.prodB, "6"); h != 7000 {
		t.Fatalf("member membeli 6 B (grosir): %v", h)
	}

	// Daftar dihapus → member kembali ke harga umum.
	call(t, "DELETE", "/api/v1/price-lists/"+member, f.token, nil).mustOK(t, "hapus daftar")
	if h := harga(rina, f.prodA, "1"); h != 15000 {
		t.Fatalf("setelah daftar dihapus: %v", h)
	}
	call(t, "DELETE", "/api/v1/price-lists/"+bawaan, f.token, nil).mustCode(t, "hapus default", 404)
}
