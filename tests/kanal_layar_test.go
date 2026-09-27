package tests

import (
	"strings"
	"testing"
)

// Layar Kanal Online: daftar kanal membawa kinerja 30 hari (pesanan, kotor,
// komisi, bersih, dibatalkan, pesanan terakhir); daftar pesanan membawa uang
// yang sebenarnya — dulu nol semua sehingga layar menampilkan "Rp 0" — beserta
// nomor nota, status penjualan, dan isinya.
func TestKanalLayarRingkasanDanPesanan(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "kanal-layar")
	ch := makeChannel(t, f, "GoFood", "0.20")

	// Selesai: 2×A = 30.000, komisi 20% = 6.000 → bersih 24.000.
	recordChannelOrder(t, f.token, ch, "GF-1", []map[string]any{{"product_id": f.prodA, "qty": "2"}})
	// Dibatalkan: tidak ikut kotor/bersih, dihitung terpisah.
	batal := recordChannelOrder(t, f.token, ch, "GF-2", []map[string]any{{"product_id": f.prodB, "qty": "1"}})
	call(t, "POST", "/api/v1/channel-orders/"+batal["id"].(string)+"/cancel", f.token, map[string]any{
		"reason": "pembeli membatalkan",
	}).mustOK(t, "batalkan")

	var kanal map[string]any
	for _, k := range call(t, "GET", "/api/v1/channels", f.token, nil).mustOK(t, "daftar kanal").Body["data"].([]any) {
		if m := k.(map[string]any); m["id"] == ch {
			kanal = m
		}
	}
	if kanal == nil {
		t.Fatal("kanal tidak ada di daftar")
	}
	st, _ := kanal["stats"].(map[string]any)
	if st == nil {
		t.Fatalf("kanal tanpa stats: %v", kanal)
	}
	assertI64(t, st, "days", 30)
	assertI64(t, st, "order_count", 1)
	assertI64(t, st, "gross_amount", 30000)
	assertI64(t, st, "fee_amount", 6000)
	assertI64(t, st, "net_amount", 24000)
	assertI64(t, st, "canceled_count", 1)
	if s, _ := st["last_order_at"].(string); s == "" {
		t.Fatalf("last_order_at kosong: %v", st)
	}

	pesanan := call(t, "GET", "/api/v1/channel-orders?channel_id="+ch, f.token, nil).
		mustOK(t, "daftar pesanan").data(t)["data"].([]any)
	if len(pesanan) != 2 {
		t.Fatalf("pesanan = %d, mau 2", len(pesanan))
	}
	for _, x := range pesanan {
		p := x.(map[string]any)
		items, _ := p["items"].([]any)
		if len(items) != 1 || p["receipt_no"] == "" || p["occurred_at"] == "" {
			t.Fatalf("pesanan tanpa isi/nota/waktu: %v", p)
		}
		switch p["external_order_id"] {
		case "GF-1":
			assertI64(t, p, "gross_amount", 30000)
			assertI64(t, p, "fee_amount", 6000)
			assertI64(t, p, "net_amount", 24000)
			if p["sale_status"] != "completed" {
				t.Fatalf("GF-1 status penjualan = %v", p["sale_status"])
			}
		case "GF-2":
			if p["sale_status"] != "canceled" || p["external_status"] != "canceled" {
				t.Fatalf("GF-2 harus dibatalkan: %v", p)
			}
		}
	}
}

// Impor laporan kanal memakai SKU — laporan marketplace tidak mengenal ID
// internal barang. SKU dicocokkan ke pemetaan SKU kanal dulu, lalu ke SKU
// barang di toko; SKU yang tidak dikenal ditolak per pesanan dengan alasan.
func TestKanalImporCSVDenganSKU(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "kanal-sku")
	ch := makeChannel(t, f, "Shopee", "0.10")

	call(t, "PUT", "/api/v1/products/"+f.prodA, f.token, map[string]any{"sku": "KOPI-A"}).mustOK(t, "sku barang A")
	call(t, "POST", "/api/v1/channels/"+ch+"/products", f.token, map[string]any{
		"product_id": f.prodB, "external_sku": "SHP-B-01",
	}).mustCode(t, "pemetaan SKU B", 201)

	csv := "external_order_id,date,sku,qty,unit_price,fee_amount\n" +
		"SP-1,2026-09-20,KOPI-A,2,15000,3000\n" + // SKU barang toko
		"SP-2,2026-09-20,SHP-B-01,1,8000,800\n" + // SKU pemetaan kanal
		"SP-3,2026-09-20,TIDAK-ADA,1,5000,0\n" // tidak dikenal
	r := callRaw(t, "POST", "/api/v1/channels/"+ch+"/orders/import", f.token, "text/csv", csv).
		mustOK(t, "impor sku").data(t)
	assertI64(t, r, "imported", 2)
	assertI64(t, r, "failed", 1)
	errs, _ := r["errors"].([]any)
	if len(errs) != 1 || !strings.Contains(errs[0].(string), "TIDAK-ADA") {
		t.Fatalf("galat impor harus menyebut SKU-nya: %v", errs)
	}

	// Header tanpa sku maupun product_id → 422.
	callRaw(t, "POST", "/api/v1/channels/"+ch+"/orders/import", f.token, "text/csv",
		"external_order_id,date,qty,unit_price\nX,2026-09-20,1,1000\n").mustCode(t, "tanpa sku", 422)
}
