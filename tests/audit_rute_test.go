package tests

import (
	"net/http/httptest"
	"strings"
	"testing"

	"candra/backend-api/routes"
)

// Audit: setiap rute terdaftar harus menolak permintaan TANPA token.
//
// Memeriksa ini dari teks routes/api.go tidak bisa diandalkan — izin bisa
// dipasang di level grup, dan parser baris-per-baris melaporkan puluhan
// positif palsu. Yang dijalankan di sini adalah routernya sendiri.
func TestAuditSemuaRuteMenolakTanpaToken(t *testing.T) {
	// Jalur yang memang TERBUKA, beserta alasannya.
	terbuka := map[string]string{
		"/health":                      "liveness, tanpa rahasia",
		"/health/ready":                "readiness, tanpa rahasia",
		"/api/v1/auth/login":           "pintu masuk",
		"/api/v1/auth/register":        "pendaftaran tenant baru",
		"/api/v1/auth/refresh":         "menukar refresh token",
		"/api/v1/partner/auth/login":   "pintu masuk mitra",
		"/api/v1/platform/auth/login":  "pintu masuk staf internal",
		"/webhooks/channels/:provider": "kanal tidak membawa token kita",
		// Tanpa sesi, tapi berpagar tanda tangan milik kanal: salah → 401
		// (diuji di sambungan_kanal_test.go).
		"/webhooks/channels/:provider/:token":       "diamankan tanda tangan penyedia per kanal",
		"/webhooks/channels/:provider/:token/*aksi": "sub-jalur webhook per kanal (token OAuth GrabFood, pesanan)",
		"/uploads/*filepath":                        "foto barang; pagarnya nama ULID tak tertebak",
		// Dibuka pembeli dari tautan WhatsApp, tanpa akun; pagarnya token
		// 128 bit acak (diuji di struk_digital_test.go).
		"/api/v1/public/receipts/:token": "struk digital untuk pembeli",
	}

	r := routes.SetupRouter()
	var bocor []string
	for _, rt := range r.Routes() {
		if _, ok := terbuka[rt.Path]; ok {
			continue
		}
		// Ganti parameter dengan nilai contoh agar rutenya cocok.
		jalur := strings.NewReplacer(":id", "x", ":pid", "x", ":provider", "x").Replace(rt.Path)
		req := httptest.NewRequest(rt.Method, jalur, strings.NewReader("{}"))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != 401 && rec.Code != 403 {
			bocor = append(bocor, rt.Method+" "+rt.Path+" → "+rec.Result().Status)
		}
	}
	if len(bocor) > 0 {
		t.Fatalf("%d rute menjawab tanpa token:\n  %s", len(bocor), strings.Join(bocor, "\n  "))
	}
	t.Logf("✓ %d rute diperiksa, semua berpagar", len(r.Routes())-len(terbuka))
}
