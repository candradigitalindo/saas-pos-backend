package tests

import (
	"fmt"
	"testing"
	"time"
)

// Uji integrasi Fase 11b — pipeline peristiwa kanal provider-agnostik
// (§5.10, blueprint F.6/F.8): webhook → channel_events → pekerja → sales +
// channel_fees + antrean channel_stock_syncs + rekonsiliasi channel_settlements.

// makeChannelWithRef membuat kanal ber-merchant_ref (kunci routing webhook).
func makeChannelWithRef(t *testing.T, f posFixture, provider, rate, merchantRef string) string {
	t.Helper()
	return call(t, "POST", "/api/v1/channels", f.token, map[string]any{
		"outlet_id": f.outletID, "kind": "delivery_app",
		"provider": provider, "name": provider, "commission_rate": rate,
		"merchant_ref": merchantRef,
	}).mustCode(t, "buat kanal "+provider, 201).data(t)["id"].(string)
}

// sendChannelWebhook mengirim payload generik ke webhook TANPA auth.
func sendChannelWebhook(t *testing.T, provider, merchantRef string, payload map[string]any) apiResp {
	t.Helper()
	return call(t, "POST", "/webhooks/channels/"+provider+"?merchant_ref="+merchantRef, "", payload)
}

// processChannelEvents memicu pekerja pemroses lewat endpoint admin.
func processChannelEvents(t *testing.T, token string) map[string]any {
	t.Helper()
	return call(t, "POST", "/api/v1/channel-events/process", token, nil).
		mustOK(t, "proses peristiwa kanal").data(t)
}

// TestChannelWebhookDedupAndProcess — DoD Fase 11b: pesanan dikirim kanal dua
// kali → satu baris channel_events → satu penjualan; proses ulang tetap satu.
func TestChannelWebhookDedupAndProcess(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "chwh") // prodA sell 15000 cost 6000, stok 100

	chID := makeChannelWithRef(t, f, "gofood", "0.20", "MERCH-chwh")
	payload := map[string]any{
		"event_type":        "order.created",
		"external_order_id": "WH-1",
		"buyer_name":        "Budi",
		"items":             []map[string]any{{"product_id": f.prodA, "qty": "2"}},
		"fees":              []map[string]any{{"kind": "commission", "amount": 6000}},
	}

	// Kirim webhook DUA kali — kanal biasa mengirim ganda.
	w1 := sendChannelWebhook(t, "gofood", "MERCH-chwh", payload).mustCode(t, "webhook #1", 200).data(t)
	if w1["accepted"] != true || w1["duplicate"] != false {
		t.Fatalf("webhook #1: accepted/duplicate salah: %v", w1)
	}
	w2 := sendChannelWebhook(t, "gofood", "MERCH-chwh", payload).mustCode(t, "webhook #2", 200).data(t)
	if w2["accepted"] != true || w2["duplicate"] != true {
		t.Fatalf("webhook #2 harus terdeteksi duplikat: %v", w2)
	}

	// Inbox: tepat SATU baris (dedup lewat UNIQUE).
	ev := call(t, "GET", "/api/v1/channels/"+chID+"/events", f.token, nil).
		mustOK(t, "list events").data(t)["data"].([]any)
	if len(ev) != 1 {
		t.Fatalf("channel_events = %d, mau 1 (dedup)", len(ev))
	}
	if ev[0].(map[string]any)["status"] != "pending" {
		t.Fatalf("status awal = %v, mau pending", ev[0].(map[string]any)["status"])
	}
	// Waktu terima = saat webhook datang (antrean pekerja diurutkan olehnya),
	// bukan time.Time nol "0001-01-01" yang menimpa DEFAULT now().
	if at, _ := time.Parse(time.RFC3339, fmt.Sprint(ev[0].(map[string]any)["received_at"])); time.Since(at) > time.Minute {
		t.Fatalf("received_at = %v, mau baru saja", ev[0].(map[string]any)["received_at"])
	}

	// Proses.
	p := processChannelEvents(t, f.token)
	assertI64(t, p, "events_done", 1)
	assertI64(t, p, "events_failed", 0)

	done := call(t, "GET", "/api/v1/channels/"+chID+"/events?status=done", f.token, nil).
		mustOK(t, "list done").data(t)["data"].([]any)
	if len(done) != 1 {
		t.Fatalf("peristiwa done = %d, mau 1", len(done))
	}

	// Tepat satu penjualan, stok terpotong sekali (100 - 2).
	sales := call(t, "GET", "/api/v1/sales?status=completed", f.token, nil).
		mustOK(t, "list sales").data(t)["data"].([]any)
	if len(sales) != 1 {
		t.Fatalf("penjualan = %d, mau 1", len(sales))
	}
	if q := stockQty(t, f.tenantFixture, f.outletID, f.prodA); q != "98" {
		t.Fatalf("stok prodA = %s, mau 98", q)
	}

	// Satu pesanan kanal.
	co := call(t, "GET", "/api/v1/channel-orders?channel_id="+chID, f.token, nil).
		mustOK(t, "channel orders").data(t)["data"].([]any)
	if len(co) != 1 {
		t.Fatalf("channel_orders = %d, mau 1", len(co))
	}
	saleID := co[0].(map[string]any)["sale_id"].(string)

	// Proses ULANG → tak ada yang diproses, penjualan tetap satu.
	p2 := processChannelEvents(t, f.token)
	assertI64(t, p2, "events_done", 0)
	sales2 := call(t, "GET", "/api/v1/sales?status=completed", f.token, nil).
		mustOK(t, "list sales lagi").data(t)["data"].([]any)
	if len(sales2) != 1 {
		t.Fatalf("penjualan setelah proses ulang = %d, mau 1", len(sales2))
	}

	// /reports/profit by_channel menampilkan biaya kanal dari channel_fees.
	bd := call(t, "GET", "/api/v1/sales/"+saleID, f.token, nil).
		mustOK(t, "get sale").data(t)["business_date"].(string)
	prof := call(t, "GET", fmt.Sprintf("/api/v1/reports/profit?from=%s&to=%s", bd, bd), f.token, nil).
		mustOK(t, "laporan laba").data(t)
	var row map[string]any
	for _, r := range prof["by_channel"].([]any) {
		m := r.(map[string]any)
		if m["channel_id"] == chID {
			row = m
		}
	}
	if row == nil {
		t.Fatalf("laporan laba tak memuat kanal: %v", prof["by_channel"])
	}
	assertI64(t, row, "omzet", 30000)
	assertI64(t, row, "biaya_kanal", 6000)
	assertI64(t, row, "laba_bersih", 30000-12000-6000) // 30000 - modal 2*6000 - fee 6000
}

// TestChannelWebhookDoesNotBlockPOS — DoD: mematikan kanal (atau kanal apa pun
// yang rewel) TIDAK mengganggu kasir. Checkout POS tetap 201.
func TestChannelWebhookDoesNotBlockPOS(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "chwhpos")
	chID := makeChannelWithRef(t, f, "grabfood", "0.15", "MERCH-chwhpos")

	// Pesanan kanal masuk lewat webhook lalu kanal dinonaktifkan.
	sendChannelWebhook(t, "grabfood", "MERCH-chwhpos", map[string]any{
		"event_type": "order.created", "external_order_id": "GB-1",
		"items": []map[string]any{{"product_id": f.prodA, "qty": "1"}},
	}).mustCode(t, "webhook", 200)
	call(t, "DELETE", "/api/v1/channels/"+chID, f.token, nil).mustOK(t, "nonaktifkan kanal")

	// Kasir tetap bisa jualan.
	checkout(t, f.token, "idem-chwhpos-1", map[string]any{
		"outlet_id": f.outletID,
		"items":     []map[string]any{{"product_id": f.prodB, "qty": "1"}},
		"payments":  []map[string]any{{"method": "cash", "amount": 8000}},
	}).mustCode(t, "checkout setelah kanal mati", 201)

	// Pekerja pemroses tetap menyelesaikan pesanan yang terlanjur masuk.
	p := processChannelEvents(t, f.token)
	assertI64(t, p, "events_done", 1)
}

// TestChannelSettlementReconcile — rekonsiliasi pencairan: hitung nilai periode
// dari penjualan + channel_fees, lalu cocokkan dengan uang yang benar-benar
// masuk (matched / mismatch).
func TestChannelSettlementReconcile(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "chset")
	chID := makeChannelWithRef(t, f, "shopeefood", "0", "MERCH-chset")

	sendChannelWebhook(t, "shopeefood", "MERCH-chset", map[string]any{
		"event_type": "order.created", "external_order_id": "SET-1",
		"items": []map[string]any{{"product_id": f.prodA, "qty": "2"}}, // 2 x 15000 = 30000
		"fees":  []map[string]any{{"kind": "commission", "amount": 4000}},
	}).mustCode(t, "webhook", 200)
	processChannelEvents(t, f.token)

	co := call(t, "GET", "/api/v1/channel-orders?channel_id="+chID, f.token, nil).
		mustOK(t, "channel orders").data(t)["data"].([]any)
	saleID := co[0].(map[string]any)["sale_id"].(string)
	bd := call(t, "GET", "/api/v1/sales/"+saleID, f.token, nil).
		mustOK(t, "get sale").data(t)["business_date"].(string)

	// Hitung settlement periode.
	s := call(t, "POST", "/api/v1/channels/"+chID+"/settlements", f.token, map[string]any{
		"period_start": bd, "period_end": bd,
	}).mustOK(t, "hitung settlement").data(t)
	assertI64(t, s, "gross_amount", 30000)
	assertI64(t, s, "fee_amount", 4000)
	assertI64(t, s, "net_amount", 26000)
	if s["status"] != "open" {
		t.Fatalf("status = %v, mau open", s["status"])
	}

	// Uang masuk = net → matched.
	m := call(t, "POST", "/api/v1/channels/"+chID+"/settlements/receipt", f.token, map[string]any{
		"period_start": bd, "received_amount": 26000,
	}).mustOK(t, "receipt cocok").data(t)
	if m["status"] != "matched" {
		t.Fatalf("status = %v, mau matched", m["status"])
	}
	assertI64(t, m, "difference", 0)

	// Uang masuk kurang → mismatch, selisih terlihat.
	mm := call(t, "POST", "/api/v1/channels/"+chID+"/settlements/receipt", f.token, map[string]any{
		"period_start": bd, "received_amount": 25000,
	}).mustOK(t, "receipt selisih").data(t)
	if mm["status"] != "mismatch" {
		t.Fatalf("status = %v, mau mismatch", mm["status"])
	}
	assertI64(t, mm, "difference", -1000)

	// Daftar: satu settlement (upsert, bukan ganda).
	lst := call(t, "GET", "/api/v1/channels/"+chID+"/settlements", f.token, nil).
		mustOK(t, "list settlement").Body["data"].([]any)
	if len(lst) != 1 {
		t.Fatalf("settlements = %d, mau 1", len(lst))
	}

	// Receipt untuk periode yang belum dihitung → 404.
	call(t, "POST", "/api/v1/channels/"+chID+"/settlements/receipt", f.token, map[string]any{
		"period_start": "2099-01-01", "received_amount": 0,
	}).mustCode(t, "receipt tanpa settlement", 404)
}

// TestChannelStockSyncHanyaKanalTersambung — pesanan dari kanal yang TIDAK
// tersambung ke API penyedia (webhook generik) tetap memotong stok, tetapi
// tidak mengantre kiriman stok: tidak ada API untuk menerimanya. Kiriman stok
// ke marketplace diuji di sinkron_stok_test.go.
func TestChannelStockSyncHanyaKanalTersambung(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "chss")
	chID := makeChannelWithRef(t, f, "tokopedia", "0", "MERCH-chss")

	sendChannelWebhook(t, "tokopedia", "MERCH-chss", map[string]any{
		"event_type": "order.created", "external_order_id": "SS-1",
		"items": []map[string]any{{"product_id": f.prodA, "qty": "3"}},
	}).mustCode(t, "webhook", 200)
	p := processChannelEvents(t, f.token)
	assertI64(t, p, "events_done", 1)
	assertI64(t, p, "stock_syncs_sent", 0)

	ss := call(t, "GET", "/api/v1/channels/"+chID+"/stock-syncs", f.token, nil).
		mustOK(t, "antrean sinkron stok").Body["data"].([]any)
	if len(ss) != 0 {
		t.Fatalf("channel_stock_syncs = %d, mau 0 (kanal tanpa API)", len(ss))
	}
	st := call(t, "GET", "/api/v1/channels/"+chID+"/stock-status", f.token, nil).mustOK(t, "status stok").data(t)
	if st["supported"] != false {
		t.Fatalf("kanal tanpa sambungan API tidak mendukung kirim stok: %v", st)
	}
}

// TestChannelEventPermission — endpoint pipeline butuh izin; webhook TIDAK butuh
// auth.
func TestChannelEventPermission(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "chevperm")
	chID := makeChannelWithRef(t, f, "gofood", "0", "MERCH-chevperm")
	kasir := staffToken(t, f.tenantFixture, roleID(t, f.tenantFixture, "Kasir"), "kasir_chevperm")

	call(t, "GET", "/api/v1/channels/"+chID+"/events", kasir, nil).
		mustCode(t, "kasir lihat events", 403)
	call(t, "POST", "/api/v1/channel-events/process", kasir, nil).
		mustCode(t, "kasir proses", 403)
	call(t, "GET", "/api/v1/channels/"+chID+"/settlements", kasir, nil).
		mustCode(t, "kasir lihat settlement", 403)
	call(t, "GET", "/api/v1/channels/"+chID+"/stock-syncs", kasir, nil).
		mustCode(t, "kasir lihat stock-syncs", 403)

	// Webhook tanpa token TIDAK boleh ditolak karena auth.
	w := sendChannelWebhook(t, "gofood", "MERCH-chevperm", map[string]any{
		"event_type": "order.created", "external_order_id": "PERM-1",
		"items": []map[string]any{{"product_id": f.prodA, "qty": "1"}},
	})
	if w.Code == 401 || w.Code == 403 {
		t.Fatalf("webhook menolak tanpa auth: %d — %s", w.Code, w.Raw)
	}
}

// TestChannelEventIsolation — tenant B tak melihat inbox / antrean kanal
// tenant A; webhook menautkan peristiwa ke tenant yang benar lewat merchant_ref.
func TestChannelEventIsolation(t *testing.T) {
	requireDB(t)
	a := setupPOS(t, "chevisoA")
	b := setupPOS(t, "chevisoB")
	chA := makeChannelWithRef(t, a, "gofood", "0", "MERCH-chevisoA")

	sendChannelWebhook(t, "gofood", "MERCH-chevisoA", map[string]any{
		"event_type": "order.created", "external_order_id": "ISOA-1",
		"items": []map[string]any{{"product_id": a.prodA, "qty": "1"}},
	}).mustCode(t, "webhook A", 200)
	processChannelEvents(t, a.token)

	// A melihat 1 peristiwa; B melihat 0 pada kanal A.
	evA := call(t, "GET", "/api/v1/channels/"+chA+"/events", a.token, nil).
		mustOK(t, "A lihat events").data(t)["data"].([]any)
	if len(evA) != 1 {
		t.Fatalf("A: events = %d, mau 1", len(evA))
	}
	evB := call(t, "GET", "/api/v1/channels/"+chA+"/events", b.token, nil).
		mustOK(t, "B lihat events A").data(t)["data"].([]any)
	if len(evB) != 0 {
		t.Fatalf("B melihat %d peristiwa kanal A — bocor", len(evB))
	}
	ssB := call(t, "GET", "/api/v1/channels/"+chA+"/stock-syncs", b.token, nil).
		mustOK(t, "B lihat stock-syncs A").Body["data"].([]any)
	if len(ssB) != 0 {
		t.Fatalf("B melihat %d antrean sinkron kanal A — bocor", len(ssB))
	}
}

// TestChannelWebhookRateLimited — webhook publik dibatasi per-IP; banjir
// permintaan menabrak 429. Ditaruh TERAKHIR di berkas ini agar tidak menghabiskan
// token untuk test webhook lain (limiter global per proses, isi ulang 20/dtk).
func TestChannelWebhookRateLimited(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "chwhrl")
	makeChannelWithRef(t, f, "gofood", "0", "MERCH-chwhrl")
	payload := map[string]any{
		"event_type": "order.created", "external_order_id": "RL-1",
		"items": []map[string]any{{"product_id": f.prodA, "qty": "1"}},
	}

	got429, got2xx := 0, 0
	for i := 0; i < 60; i++ { // burst 40 → sisanya kena 429
		code := sendChannelWebhook(t, "gofood", "MERCH-chwhrl", payload).Code
		switch {
		case code == 429:
			got429++
		case code >= 200 && code < 300:
			got2xx++
		}
	}
	if got2xx == 0 {
		t.Fatalf("tak satu pun permintaan lolos — limiter terlalu ketat")
	}
	if got429 == 0 {
		t.Fatalf("60 permintaan beruntun tak ada yang 429 — limiter tidak terpasang")
	}
}
