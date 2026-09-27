package tests

import (
	"testing"
	"time"

	"candra/backend-api/database"
	"candra/backend-api/internal/ulid"
)

// Membayar di tengah masa coba tidak menghanguskan sisa masa cobanya: masa
// berbayar dimulai saat masa coba berakhir. Dulu masa berbayar dimulai hari
// pembayaran, jadi pemilik yang membayar di hari pertama kehilangan 14 hari
// gratis — yang menunda pembayaran justru diuntungkan.

// tanggal mengambil bagian YYYY-MM-DD dari cap waktu RFC3339 atau tanggal saja.
func tanggal(v any) string {
	s, _ := v.(string)
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}

func TestBayarSaatMasaCobaMulaiSetelahMasaCobaHabis(t *testing.T) {
	requireDB(t)
	f := registerTenantPolos(t, "bayarcoba")
	sub := call(t, "POST", "/api/v1/subscription", f.token, map[string]any{
		"plan_code": "basic", "term_months": 1,
	}).mustCode(t, "mulai Basic", 201).data(t)
	akhirCoba, err := time.Parse(time.RFC3339, sub["trial_ends_at"].(string))
	if err != nil {
		t.Fatalf("trial_ends_at: %v", err)
	}
	mulai := akhirCoba.UTC().Format("2006-01-02")
	selesai := akhirCoba.UTC().AddDate(0, 1, 0).Format("2006-01-02")

	// Tagihan yang terbit di masa coba sudah menyebut periode sesudahnya.
	inv := call(t, "POST", "/api/v1/subscription/invoices", f.token, nil).mustCode(t, "tagihan", 201).data(t)
	if tanggal(inv["period_start"]) != mulai || tanggal(inv["period_end"]) != selesai {
		t.Fatalf("periode tagihan %v–%v, mau %s–%s", inv["period_start"], inv["period_end"], mulai, selesai)
	}
	total := int64(inv["total_amount"].(float64))
	paySub(t, f.token, ulid.New(), inv["id"].(string), total).mustCode(t, "bayar", 201)

	// Paket langsung aktif (tidak ada jeda), periodenya mulai saat masa coba habis.
	if p := paketMe(t, f.token); p["code"] != "basic" || p["status"] != "active" {
		t.Fatalf("paket setelah bayar = %v/%v, mau basic/active", p["code"], p["status"])
	}
	s := call(t, "GET", "/api/v1/subscription", f.token, nil).mustOK(t, "ringkasan").data(t)["subscription"].(map[string]any)
	if tanggal(s["current_period_start"]) != mulai || tanggal(s["current_period_end"]) != selesai {
		t.Fatalf("periode langganan %v–%v, mau %s–%s", s["current_period_start"], s["current_period_end"], mulai, selesai)
	}
	// Pendapatan diakui mulai bulan masa berbayarnya, bukan bulan uangnya masuk.
	var bulan time.Time
	database.DB.Raw(`SELECT min(recognition_month) FROM deferred_revenue_entries WHERE subscription_invoice_id = ?`,
		inv["id"]).Row().Scan(&bulan)
	if got, want := bulan.Format("2006-01"), akhirCoba.UTC().Format("2006-01"); got != want {
		t.Fatalf("bulan pengakuan pertama %s, mau %s", got, want)
	}

	// Berhenti sebelum masa berbayarnya mulai → belum ada bulan terpakai,
	// uang kembali utuh (dulu minimal 1 bulan dianggap terpakai).
	res := call(t, "POST", "/api/v1/subscription/cancel", f.token, map[string]any{"reason": "batal"}).
		mustOK(t, "berhenti").data(t)
	assertI64(t, res, "months_used", 0)
	assertI64(t, res, "refund_amount", total)
	assertI64(t, res, "earned_amount", 0)
}

// Tagihan terbit di masa coba, tapi baru dibayar setelah masa coba habis:
// masa berbayar mulai hari pembayaran — hari-hari di antaranya tenant memakai
// paket Gratis, jadi tidak ikut ditagihkan.
func TestBayarSetelahMasaCobaHabisMulaiHariItu(t *testing.T) {
	requireDB(t)
	f := registerTenantPolos(t, "bayarlewat")
	call(t, "POST", "/api/v1/subscription", f.token, map[string]any{
		"plan_code": "basic", "term_months": 1,
	}).mustCode(t, "mulai Basic", 201)
	inv := call(t, "POST", "/api/v1/subscription/invoices", f.token, nil).mustCode(t, "tagihan", 201).data(t)

	lewat := time.Now().UTC().Add(-time.Hour)
	aturLangganan(t, f.tenantID, map[string]any{"trial_ends_at": lewat, "current_period_end": lewat})
	paid := paySub(t, f.token, ulid.New(), inv["id"].(string), int64(inv["total_amount"].(float64))).
		mustCode(t, "bayar", 201).data(t)

	hariIni := time.Now().UTC().Format("2006-01-02")
	if tanggal(paid["period_start"]) != hariIni {
		t.Fatalf("periode tagihan mulai %v, mau hari ini %s", paid["period_start"], hariIni)
	}
	if p := paketMe(t, f.token); p["code"] != "basic" || p["status"] != "active" {
		t.Fatalf("paket setelah bayar = %v/%v, mau basic/active", p["code"], p["status"])
	}
}
