package tests

import (
	"fmt"
	"testing"
)

// Uji integrasi Fase 11a — kanal pesanan online (fondasi, §5.10, blueprint F).

// makeChannel membuat kanal pada outlet fixture dan mengembalikan id-nya.
func makeChannel(t *testing.T, f posFixture, provider, rate string) string {
	t.Helper()
	return call(t, "POST", "/api/v1/channels", f.token, map[string]any{
		"outlet_id": f.outletID, "kind": "delivery_app",
		"provider": provider, "name": provider, "commission_rate": rate,
	}).mustCode(t, "buat kanal "+provider, 201).data(t)["id"].(string)
}

// recordChannelOrder mencatat pesanan kanal manual.
func recordChannelOrder(t *testing.T, token, channelID, extID string, items []map[string]any) map[string]any {
	t.Helper()
	return call(t, "POST", "/api/v1/channel-orders", token, map[string]any{
		"channel_id": channelID, "external_order_id": extID, "items": items,
	}).data(t)
}

// TestChannelProfitAfterCommission — DoD Fase 11a: laba bersih per kanal keluar
// benar (setelah komisi) tanpa satu pun API kanal.
func TestChannelProfitAfterCommission(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "chprofit") // prodA sell 15000 cost 6000; prodB sell 8000 cost 3000

	gofood := makeChannel(t, f, "gofood", "0.20") // komisi 20%
	shopee := makeChannel(t, f, "shopeefood", "0.05")

	// GoFood: 4 x prodA = 60000 kotor; komisi 12000; laba = 60000-24000-12000 = 24000.
	og := recordChannelOrder(t, f.token, gofood, "GF-1",
		[]map[string]any{{"product_id": f.prodA, "qty": "4"}})
	assertI64(t, og, "gross_amount", 60000)
	assertI64(t, og, "fee_amount", 12000)
	assertI64(t, og, "net_amount", 48000)
	bd := call(t, "GET", "/api/v1/sales/"+og["sale_id"].(string), f.token, nil).
		mustOK(t, "get sale gofood").data(t)["business_date"].(string)

	// ShopeeFood: 2 x prodB = 16000 kotor; komisi 800; laba = 16000-6000-800 = 9200.
	os := recordChannelOrder(t, f.token, shopee, "SP-1",
		[]map[string]any{{"product_id": f.prodB, "qty": "2"}})
	assertI64(t, os, "fee_amount", 800)

	// Stok terpotong.
	if q := stockQty(t, f.tenantFixture, f.outletID, f.prodA); q != "96" {
		t.Fatalf("stok prodA = %s, mau 96", q)
	}

	// /reports/profit → laba bersih per kanal SETELAH komisi.
	prof := call(t, "GET", fmt.Sprintf("/api/v1/reports/profit?from=%s&to=%s", bd, bd), f.token, nil).
		mustOK(t, "laporan laba").data(t)
	byCh := map[string]map[string]any{}
	for _, r := range prof["by_channel"].([]any) {
		m := r.(map[string]any)
		byCh[m["channel_id"].(string)] = m
	}
	g, s := byCh[gofood], byCh[shopee]
	if g == nil || s == nil {
		t.Fatalf("laporan laba tidak memuat kedua kanal: %v", prof["by_channel"])
	}
	assertI64(t, g, "omzet", 60000)
	assertI64(t, g, "modal", 24000)
	assertI64(t, g, "biaya_kanal", 12000)
	assertI64(t, g, "laba_bersih", 24000)
	assertI64(t, s, "omzet", 16000)
	assertI64(t, s, "biaya_kanal", 800)
	assertI64(t, s, "laba_bersih", 9200)

	// Total = dua kanal, dua penjualan (satu pintu dengan POS).
	assertI64(t, prof["totals"].(map[string]any), "laba_bersih", 33200)
	ls := call(t, "GET", "/api/v1/sales?status=completed", f.token, nil).mustOK(t, "list sales").data(t)
	if n := len(ls["data"].([]any)); n != 2 {
		t.Fatalf("jumlah penjualan = %d, mau 2", n)
	}
}

// TestChannelOrderIdempotent — pesanan yang sama dientri dua kali tetap satu penjualan.
func TestChannelOrderIdempotent(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "chidem")
	ch := makeChannel(t, f, "tokopedia", "0.08")

	body := map[string]any{
		"channel_id": ch, "external_order_id": "TP-99",
		"items": []map[string]any{{"product_id": f.prodA, "qty": "3"}},
	}
	first := call(t, "POST", "/api/v1/channel-orders", f.token, body).mustCode(t, "entri #1", 201).data(t)
	second := call(t, "POST", "/api/v1/channel-orders", f.token, body).mustCode(t, "entri #2 (idempoten)", 200).data(t)
	if first["sale_id"] != second["sale_id"] {
		t.Fatalf("entri kedua membuat penjualan baru: %v vs %v", first["sale_id"], second["sale_id"])
	}
	// Stok berkurang sekali (100 - 3).
	if q := stockQty(t, f.tenantFixture, f.outletID, f.prodA); q != "97" {
		t.Fatalf("stok = %s, mau 97 (entri ulang tak boleh memotong lagi)", q)
	}
	if n := len(call(t, "GET", "/api/v1/sales?status=completed", f.token, nil).mustOK(t, "sales").data(t)["data"].([]any)); n != 1 {
		t.Fatalf("jumlah penjualan = %d, mau 1", n)
	}
}

// TestChannelOrderCancel — batal pesanan kanal → penjualan di-void, stok kembali.
func TestChannelOrderCancel(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "chcancel")
	ch := makeChannel(t, f, "grabfood", "0.15")

	o := recordChannelOrder(t, f.token, ch, "GB-1",
		[]map[string]any{{"product_id": f.prodA, "qty": "5"}})
	if q := stockQty(t, f.tenantFixture, f.outletID, f.prodA); q != "95" {
		t.Fatalf("stok setelah pesanan = %s, mau 95", q)
	}

	coID := o["id"].(string)
	res := call(t, "POST", "/api/v1/channel-orders/"+coID+"/cancel", f.token, map[string]any{
		"reason": "pembeli batal",
	}).mustOK(t, "batalkan").data(t)
	if res["external_status"] != "canceled" {
		t.Fatalf("external_status = %v, mau canceled", res["external_status"])
	}
	if q := stockQty(t, f.tenantFixture, f.outletID, f.prodA); q != "100" {
		t.Fatalf("stok setelah batal = %s, mau 100 (dikembalikan)", q)
	}
	sale := call(t, "GET", "/api/v1/sales/"+o["sale_id"].(string), f.token, nil).mustOK(t, "get sale").data(t)
	if sale["status"] != "canceled" {
		t.Fatalf("status penjualan = %v, mau canceled", sale["status"])
	}
	// Batal kedua → 409.
	call(t, "POST", "/api/v1/channel-orders/"+coID+"/cancel", f.token, map[string]any{"reason": "lagi"}).
		mustCode(t, "batal kedua", 409)
}

// TestChannelSKUMappingPrice — harga kanal dipakai bila item tak mengirim unit_price.
func TestChannelSKUMappingPrice(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "chsku")
	ch := makeChannel(t, f, "shopee", "0")

	call(t, "POST", "/api/v1/channels/"+ch+"/products", f.token, map[string]any{
		"product_id": f.prodA, "external_sku": "SHP-A-001", "channel_price": 22000,
	}).mustCode(t, "petakan SKU", 201)

	o := recordChannelOrder(t, f.token, ch, "SHP-1",
		[]map[string]any{{"product_id": f.prodA, "qty": "2"}})
	// 2 x 22000 (harga kanal), bukan 2 x 15000 (harga master).
	assertI64(t, o, "gross_amount", 44000)

	item := call(t, "GET", "/api/v1/sales/"+o["sale_id"].(string), f.token, nil).
		mustOK(t, "get sale").data(t)["items"].([]any)[0].(map[string]any)
	assertI64(t, item, "unit_price", 22000)
}

// TestChannelOrdersCSVImport — impor laporan harian kanal dari CSV; idempoten.
func TestChannelOrdersCSVImport(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "chcsv")
	ch := makeChannel(t, f, "tiktok", "0.10")

	csvBody := "external_order_id,date,product_id,qty,unit_price,fee_amount\n" +
		"TT-1,2026-09-08," + f.prodA + ",2,15000,3000\n" +
		"TT-2,2026-09-08," + f.prodB + ",1,8000,800\n" +
		"TT-2,2026-09-08," + f.prodA + ",1,15000,1500\n" // baris kedua TT-2 → digabung

	r := callRaw(t, "POST", "/api/v1/channels/"+ch+"/orders/import", f.token, "text/csv", csvBody).
		mustOK(t, "impor csv").data(t)
	assertI64(t, r, "imported", 2)
	assertI64(t, r, "failed", 0)

	// Impor ulang berkas yang sama → semua dilewati (idempoten).
	r2 := callRaw(t, "POST", "/api/v1/channels/"+ch+"/orders/import", f.token, "text/csv", csvBody).
		mustOK(t, "impor ulang").data(t)
	assertI64(t, r2, "imported", 0)
	assertI64(t, r2, "skipped", 2)

	// Header kurang kolom → 422.
	callRaw(t, "POST", "/api/v1/channels/"+ch+"/orders/import", f.token, "text/csv", "external_order_id,qty\nX,1\n").
		mustCode(t, "header buruk", 422)
}

// TestChannelPermission — endpoint kanal butuh izin channel.*.
func TestChannelPermission(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "chperm")
	kasir := staffToken(t, f.tenantFixture, roleID(t, f.tenantFixture, "Kasir"), "kasir_chperm")

	call(t, "GET", "/api/v1/channels", kasir, nil).mustCode(t, "kasir lihat kanal", 403)
	call(t, "POST", "/api/v1/channels", kasir, map[string]any{
		"outlet_id": f.outletID, "kind": "marketplace", "provider": "x", "name": "x",
	}).mustCode(t, "kasir buat kanal", 403)
	call(t, "GET", "/api/v1/channel-orders", kasir, nil).mustCode(t, "kasir lihat pesanan kanal", 403)
}

// TestChannelIsolation — tenant B tak melihat kanal / pesanan tenant A.
func TestChannelIsolation(t *testing.T) {
	requireDB(t)
	a := setupPOS(t, "chisoA")
	b := setupPOS(t, "chisoB")

	chA := makeChannel(t, a, "gofood", "0.2")
	oA := recordChannelOrder(t, a.token, chA, "A-1",
		[]map[string]any{{"product_id": a.prodA, "qty": "1"}})

	call(t, "GET", "/api/v1/channels/"+chA, b.token, nil).mustCode(t, "B baca kanal A", 404)
	call(t, "GET", "/api/v1/channel-orders/"+oA["id"].(string), b.token, nil).mustCode(t, "B baca pesanan A", 404)
	chansB, _ := call(t, "GET", "/api/v1/channels", b.token, nil).mustOK(t, "kanal B").Body["data"].([]any)
	if len(chansB) != 0 {
		t.Fatalf("tenant B melihat %d kanal tenant A", len(chansB))
	}
}
