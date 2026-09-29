package tests

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"candra/backend-api/database"
	"candra/backend-api/internal/ulid"
	"candra/backend-api/services"
)

// Pengingat utang pemasok (cmd/payable-reminders): satu ringkasan WhatsApp
// saat ada nota yang BARU masuk masa jatuh tempo sebentar lagi atau BARU
// lewat — bukan setiap hari untuk utang yang sama. Angka ringkasan Beranda
// (GET /payables/summary) memakai batas hari yang sama.
func TestPengingatUtangPemasok(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "ingatutang")
	sup := call(t, "POST", "/api/v1/suppliers", f.token, map[string]any{"name": "UD Sentosa"}).
		mustCode(t, "pemasok", 201).data(t)["id"].(string)

	loc, _ := time.LoadLocation("Asia/Jakarta")
	hari := func(n int) string { return time.Now().In(loc).AddDate(0, 0, n).Format("2006-01-02") }
	beli := func(jatuhTempo string, dibayar int64) string {
		t.Helper()
		return checkoutLike(t, f.token, ulid.New(), "/api/v1/purchases", map[string]any{
			"outlet_id": f.outletID, "supplier_id": sup, "due_date": jatuhTempo, "paid_amount": dibayar,
			"items": []map[string]any{{"product_id": f.prodA, "qty": "1", "unit_cost": 10000}},
		}).mustCode(t, "beli", 201).data(t)["id"].(string)
	}
	beli(hari(-2), 0)         // lewat 2 hari
	beli(hari(1), 4000)       // besok, sisa 6.000
	jauh := beli(hari(10), 0) // masih jauh — belum diingatkan
	beli(hari(-5), 10000)     // lewat tapi LUNAS — tidak ikut

	r := call(t, "GET", "/api/v1/payables/summary?outlet_id="+f.outletID, f.token, nil).mustOK(t, "ringkasan").data(t)
	assertI64(t, r, "overdue_count", 1)
	assertI64(t, r, "overdue_amount", 10000)
	assertI64(t, r, "due_soon_count", 1)
	assertI64(t, r, "due_soon_amount", 6000)

	jalan := func() services.PayableReminderReport {
		t.Helper()
		rep, err := services.RunPayableReminders(context.Background(), services.PayableReminderOptions{TenantID: f.tenantID})
		if err != nil || rep.Failed != 0 {
			t.Fatalf("RunPayableReminders: %v, gagal %d", err, rep.Failed)
		}
		return rep
	}
	pesan := func() []string {
		t.Helper()
		var payloads []string
		database.DB.Raw(`SELECT payload::text FROM outbox_events WHERE tenant_id = ? AND topic = 'payable.due_digest'
			ORDER BY created_at`, f.tenantID).Scan(&payloads)
		return payloads
	}

	if rep := jalan(); rep.Notices != 2 || rep.Sent != 1 {
		t.Fatalf("putaran pertama: %+v, mau 2 nota & 1 pesan", rep)
	}
	p := pesan()
	if len(p) != 1 {
		t.Fatalf("pesan: %d, mau 1", len(p))
	}
	var isi map[string]any
	_ = json.Unmarshal([]byte(p[0]), &isi)
	ringkasan, _ := isi["ringkasan"].(string)
	for _, mau := range []string{"Lewat jatuh tempo: 1 nota", "sebentar lagi: 1 nota", "UD Sentosa", "lewat 2 hari", "besok"} {
		if !strings.Contains(ringkasan, mau) {
			t.Fatalf("ringkasan tidak memuat %q:\n%s", mau, ringkasan)
		}
	}
	if isi["to"] == nil || isi["to"] == "" {
		t.Fatalf("pesan tanpa tujuan: %v", isi)
	}

	// Dijalankan lagi tanpa yang baru → tidak ada pesan.
	if rep := jalan(); rep.Notices != 0 || rep.Sent != 0 || len(pesan()) != 1 {
		t.Fatalf("putaran kedua: %+v, pesan %d — mau tidak ada yang baru", rep, len(pesan()))
	}
	// Nota yang jauh kini jatuh tempo besok → satu pesan baru.
	database.DB.Exec(`UPDATE purchases SET due_date = ?::date WHERE id = ?`, hari(1), jauh)
	if rep := jalan(); rep.Notices != 1 || rep.Sent != 1 || len(pesan()) != 2 {
		t.Fatalf("putaran ketiga: %+v, pesan %d — mau 1 nota baru & pesan kedua", rep, len(pesan()))
	}
}
