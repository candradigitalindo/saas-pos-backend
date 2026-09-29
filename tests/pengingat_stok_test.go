package tests

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"candra/backend-api/database"
	"candra/backend-api/services"
)

// Pengingat stok menipis (cmd/stock-reminders): satu ringkasan WhatsApp saat
// ada barang yang BARU habis / di bawah batas; tidak diulang setiap hari; dan
// barang yang pulih lalu menipis lagi diingatkan LAGI.
func TestPengingatStokMenipis(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "ingatstok") // A: 100, B: 50 — aman
	unit := makeUnit(t, f.tenantFixture, "btl")
	sup := call(t, "POST", "/api/v1/suppliers", f.token, map[string]any{"name": "UD Kecap"}).
		mustCode(t, "pemasok", 201).data(t)["id"].(string)
	barang := func(nama, min, pemasok string) string {
		t.Helper()
		body := map[string]any{"name": nama, "unit_id": unit, "sell_price": 5000, "cost_price": 3000,
			"track_stock": true, "min_stock": min}
		if pemasok != "" {
			body["supplier_id"] = pemasok
		}
		return call(t, "POST", "/api/v1/products", f.token, body).mustCode(t, "barang "+nama, 201).data(t)["id"].(string)
	}
	tipis := barang("Kecap Manis", "5", sup)
	habis := barang("Saus Sambal", "0", "")
	minus := barang("Sirup Jeruk", "0", "")
	adjust(t, f.tenantFixture, f.outletID, tipis, "3", "awal")
	adjust(t, f.tenantFixture, f.outletID, habis, "0", "awal")
	adjust(t, f.tenantFixture, f.outletID, minus, "0", "awal")
	checkout(t, f.token, "IS-1", map[string]any{
		"outlet_id": f.outletID,
		"items":     []map[string]any{{"product_id": minus, "qty": "2"}},
		"payments":  []map[string]any{{"method": "cash", "amount": 10000}},
	}).mustCode(t, "jual sampai minus", 201)

	jalan := func() services.StockReminderReport {
		t.Helper()
		rep, err := services.RunStockReminders(context.Background(), services.StockReminderOptions{TenantID: f.tenantID})
		if err != nil || rep.Failed != 0 {
			t.Fatalf("RunStockReminders: %v, gagal %d", err, rep.Failed)
		}
		return rep
	}
	pesan := func() []string {
		t.Helper()
		var p []string
		database.DB.Raw(`SELECT payload->>'ringkasan' FROM outbox_events WHERE tenant_id = ? AND topic = 'stock.low_digest'
			ORDER BY created_at`, f.tenantID).Scan(&p)
		return p
	}

	if rep := jalan(); rep.Notices != 3 || rep.Sent != 1 {
		t.Fatalf("putaran pertama: %+v, mau 3 barang & 1 pesan", rep)
	}
	isi := pesan()[0]
	for _, mau := range []string{"Habis (2)", "Saus Sambal", "Sirup Jeruk", "Hampir habis (1)",
		"Kecap Manis (sisa 3 btl, batas 5) — UD Kecap"} {
		if !strings.Contains(isi, mau) {
			t.Fatalf("ringkasan tidak memuat %q:\n%s", mau, isi)
		}
	}
	if strings.Contains(isi, "Produk A") {
		t.Fatalf("barang aman ikut disebut:\n%s", isi)
	}

	// Tidak ada yang baru → tidak ada pesan.
	if rep := jalan(); rep.Notices != 0 || rep.Sent != 0 {
		t.Fatalf("putaran kedua: %+v", rep)
	}
	// Kecap dibeli lagi → catatannya dihapus, tanpa pesan.
	adjust(t, f.tenantFixture, f.outletID, tipis, "20", "barang masuk")
	if rep := jalan(); rep.Sent != 0 {
		t.Fatalf("setelah pulih: %+v", rep)
	}
	var sisa int64
	database.DB.Raw(`SELECT COUNT(*) FROM stock_notices WHERE tenant_id = ? AND product_id = ?`, f.tenantID, tipis).Scan(&sisa)
	if sisa != 0 {
		t.Fatalf("catatan kecap masih ada setelah pulih: %d", sisa)
	}
	// Menipis lagi → diingatkan lagi.
	adjust(t, f.tenantFixture, f.outletID, tipis, "2", "terjual")
	if rep := jalan(); rep.Notices != 1 || rep.Sent != 1 || len(pesan()) != 2 {
		t.Fatalf("menipis lagi: %+v, pesan %d", rep, len(pesan()))
	}
	var payload map[string]any
	var raw string
	database.DB.Raw(`SELECT payload::text FROM outbox_events WHERE tenant_id = ? AND topic = 'stock.low_digest'
		ORDER BY created_at DESC LIMIT 1`, f.tenantID).Scan(&raw)
	_ = json.Unmarshal([]byte(raw), &payload)
	if payload["to"] == nil || payload["to"] == "" {
		t.Fatalf("pesan tanpa tujuan: %v", payload)
	}
}
