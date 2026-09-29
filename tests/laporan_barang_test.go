package tests

import (
	"strings"
	"testing"
)

// Uji laporan per BARANG (group_by=product) — "barang apa yang paling laku".
//
// Yang dijaga: angka per barang harus BERSIH — penjualan yang dibatalkan (void)
// tidak ikut, dan retur mengurangi jumlah maupun uangnya. Tanpa itu, barang
// yang sering dikembalikan pembeli justru tampil sebagai "terlaris", dan
// pemilik warung menyetok ulang barang yang salah.
func TestLaporanPerBarangBersihDariVoidDanRetur(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "terlaris")

	jual := func(kunci string, items []map[string]any, bayar int64) map[string]any {
		t.Helper()
		return checkout(t, f.token, kunci, map[string]any{
			"outlet_id": f.outletID,
			"items":     items,
			"payments":  []map[string]any{{"method": "cash", "amount": bayar}},
		}).mustCode(t, "jual "+kunci, 201).data(t)
	}

	// A: 5×A (75.000, modal 30.000) + 2×B (16.000, modal 6.000)
	a := jual("TL-A", []map[string]any{
		{"product_id": f.prodA, "qty": "5"},
		{"product_id": f.prodB, "qty": "2"},
	}, 91000)
	// B: 3×A — dibatalkan, harus hilang seluruhnya.
	b := jual("TL-B", []map[string]any{{"product_id": f.prodA, "qty": "3"}}, 45000)
	// C: 4×B — diretur, pasangan jual+retur saling menghapus.
	c := jual("TL-C", []map[string]any{{"product_id": f.prodB, "qty": "4"}}, 32000)
	// D: 1×B (8.000, modal 3.000)
	jual("TL-D", []map[string]any{{"product_id": f.prodB, "qty": "1"}}, 8000)

	call(t, "POST", "/api/v1/sales/"+b["id"].(string)+"/void", f.token,
		map[string]any{"reason": "salah input"}).mustOK(t, "void B")
	call(t, "POST", "/api/v1/sales/"+c["id"].(string)+"/refund", f.token,
		map[string]any{"reason": "barang rusak"}).mustOK(t, "retur C")

	hari := a["business_date"].(string)
	d := call(t, "GET",
		"/api/v1/reports/sales?from="+hari+"&to="+hari+"&group_by=product&outlet_id="+f.outletID,
		f.token, nil).mustOK(t, "laporan per barang").data(t)

	if d["group_by"] != "product" {
		t.Fatalf("group_by = %v, mau \"product\"", d["group_by"])
	}
	baris := d["rows"].([]any)
	if len(baris) != 2 {
		t.Fatalf("baris = %d, mau 2 (A & B); isi: %v", len(baris), baris)
	}

	// Diurutkan menurut uang masuk, terbesar dulu: A (75.000) lalu B (24.000).
	pertama, kedua := baris[0].(map[string]any), baris[1].(map[string]any)
	if pertama["key"] != f.prodA || kedua["key"] != f.prodB {
		t.Fatalf("urutan = %v, %v; mau A lalu B (menurut uang masuk)", pertama["key"], kedua["key"])
	}

	// A: hanya penjualan A — void B tidak ikut.
	if pertama["qty"] != "5" {
		t.Fatalf("qty A = %v, mau \"5\" — 3 dari transaksi yang dibatalkan ikut terhitung?", pertama["qty"])
	}
	assertI64(t, pertama, "net_amount", 75000)
	assertI64(t, pertama, "cost_amount", 30000)
	assertI64(t, pertama, "gross_profit", 45000)
	assertI64(t, pertama, "sales_count", 1)
	if pertama["label"] != "Produk A terlaris" || pertama["unit"] != "pcs" {
		t.Fatalf("label/unit A = %v/%v, mau \"Produk A terlaris\"/\"pcs\"", pertama["label"], pertama["unit"])
	}

	// B: 2 (A) + 4 (C) − 4 (retur C) + 1 (D) = 3; uang 16.000 + 8.000.
	if kedua["qty"] != "3" {
		t.Fatalf("qty B = %v, mau \"3\" — retur tidak mengurangi jumlah terjual?", kedua["qty"])
	}
	assertI64(t, kedua, "net_amount", 24000)
	assertI64(t, kedua, "cost_amount", 9000)
	assertI64(t, kedua, "gross_profit", 15000)

	// Ekspor CSV per barang memuat NAMA & jumlahnya — bukan cuma ULID yang tak
	// berarti apa-apa di spreadsheet.
	kode, _, isi := rawResponse(t,
		"/api/v1/reports/export?type=sales&format=csv&group_by=product&from="+hari+"&to="+hari+
			"&outlet_id="+f.outletID, f.token)
	if kode != 200 {
		t.Fatalf("ekspor CSV per barang: kode %d, isi %s", kode, isi)
	}
	for _, mau := range []string{
		"key,product_name,unit,qty,sales_count",
		f.prodA + ",Produk A terlaris,pcs,5,1,",
		"TOTAL,,,,",
	} {
		if !strings.Contains(isi, mau) {
			t.Fatalf("CSV tidak memuat %q:\n%s", mau, isi)
		}
	}
}

// Laporan per barang tunduk pada izin laporan yang sama dengan pengelompokan
// lain, dan pesan galat group_by yang tak dikenal kini menyebut "product".
func TestLaporanPerBarangDisebutDiPesanGalat(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "terlarissalah")

	res := call(t, "GET",
		"/api/v1/reports/sales?from=2026-09-18&to=2026-09-18&group_by=sku&outlet_id="+f.outletID,
		f.token, nil)
	if res.Code != 422 && res.Code != 400 {
		t.Fatalf("group_by tak dikenal: kode %d, mau 4xx validasi", res.Code)
	}
	if !strings.Contains(res.Raw, "product") {
		t.Fatalf("pesan galat tidak menyebut pilihan \"product\": %s", res.Raw)
	}
}
