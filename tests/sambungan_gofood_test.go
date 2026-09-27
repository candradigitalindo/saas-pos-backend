package tests

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// Sambungan GoFood (GoBiz Direct Integration) dengan kredensial MILIK TENANT.
//
// GoBiz tiruan memeriksa token client credentials (Basic auth), membaca
// outlet, dan menerima langganan webhook. Yang dijaga: webhook didaftarkan
// otomatis sekali saja; X-Go-Signature diperiksa dengan Notification Secret
// Key tenant; penjualan baru dicatat saat pesanan DITERIMA toko (menunggu
// diterima tidak memotong stok); pembatalan membatalkan penjualannya; batal
// untuk pesanan yang tak pernah diterima diabaikan, tidak mati di antrean.
func TestSambunganGoFoodDariKredensialTenant(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "sambung-gf")
	call(t, "PUT", "/api/v1/products/"+f.prodA, f.token, map[string]any{"sku": "1773640"}).mustOK(t, "sku A")
	ch := makeChannel(t, f, "GoFood", "0.20")

	var mu sync.Mutex
	var langganan []map[string]any
	gobiz := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/oauth2/token":
			id, rahasia, _ := r.BasicAuth()
			_ = r.ParseForm()
			if id != "klien-toko" || rahasia != "rahasia-klien" || r.Form.Get("grant_type") != "client_credentials" {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"invalid_client","error_description":"Client authentication failed"}`))
				return
			}
			_, _ = w.Write([]byte(`{"access_token":"tok-gobiz","expires_in":3599,"token_type":"bearer"}`))
		case r.Header.Get("Authorization") != "Bearer tok-gobiz":
			w.WriteHeader(http.StatusUnauthorized)
		case r.Method == "GET" && r.URL.Path == "/integrations/partner/outlets/G123456789/v1":
			_, _ = w.Write([]byte(`{"success":true,"data":{"outlet":{"id":"G123456789","name":"Warung Uji GoFood"}}}`))
		case r.Method == "POST" && r.URL.Path == "/integrations/partner/outlets/G123456789/v1/notification-subscriptions":
			var b map[string]any
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &b)
			mu.Lock()
			langganan = append(langganan, b)
			mu.Unlock()
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"success":true,"data":{"subscription":{"id":"x","active":true}}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errors":[{"message":"outlet tidak ditemukan"}]}`))
		}
	}))
	defer gobiz.Close()
	t.Setenv("CHANNEL_GOBIZ_API_URL", gobiz.URL)
	t.Setenv("CHANNEL_GOBIZ_OAUTH_URL", gobiz.URL)
	t.Setenv("APP_URL", "https://pos.contoh.id")

	jalur := "/api/v1/channels/" + ch + "/connection"
	simpan := func(fields map[string]any) map[string]any {
		return call(t, "PUT", jalur, f.token, map[string]any{"provider": "gofood", "fields": fields}).
			mustOK(t, "simpan gofood").data(t)
	}
	simpan(map[string]any{
		"client_id": "klien-toko", "client_secret": "salah", "outlet_id": "G123456789",
		"notification_secret": "rahasia-notif", "environment": "sandbox",
	})
	uji := call(t, "POST", jalur+"/test", f.token, nil).mustOK(t, "tes gagal").data(t)
	if uji["status"] != "error" || !strings.Contains(uji["error"].(string), "Client authentication failed") {
		t.Fatalf("secret salah: %v", uji)
	}
	if len(langganan) != 0 {
		t.Fatal("webhook didaftarkan padahal kredensial salah")
	}

	s := simpan(map[string]any{"client_secret": "rahasia-klien"})
	url := s["webhook_url"].(string)
	token := url[strings.LastIndex(url, "/")+1:]
	uji = call(t, "POST", jalur+"/test", f.token, nil).mustOK(t, "tes berhasil").data(t)
	if uji["status"] != "connected" || !strings.Contains(uji["info"].(string), "Warung Uji GoFood") ||
		!strings.Contains(uji["info"].(string), "webhook didaftarkan") {
		t.Fatalf("tes berhasil: %v", uji)
	}
	if len(langganan) != 5 || langganan[0]["url"] != url || langganan[0]["active"] != true {
		t.Fatalf("langganan webhook = %v", langganan)
	}
	// Tes ulang tidak mendaftarkan ganda.
	uji = call(t, "POST", jalur+"/test", f.token, nil).mustOK(t, "tes ulang").data(t)
	if len(langganan) != 5 || !strings.Contains(uji["info"].(string), "webhook terdaftar") {
		t.Fatalf("tes ulang mendaftar lagi: %d, %v", len(langganan), uji["info"])
	}

	kirim := func(event, nomor, rahasia string) apiResp {
		body := `{"header":{"event_name":"` + event + `","event_id":"e-` + event + nomor + `","version":1,"timestamp":"2026-09-28T10:15:22.557+07:00"},
			"body":{"customer":{"id":"C1","name":"Pelanggan GoFood"},"driver":{"name":"Pak Ojol"},"service_type":"gofood",
			"outlet":{"id":"G123456789","external_outlet_id":"x"},
			"order":{"status":"X","pin":"1234","order_number":"` + nomor + `","order_total":3.0e4,"currency":"IDR",
			"created_at":"2026-09-28T10:14:00.000+07:00","cancellation_detail":{"reason":"Dibatalkan resto - ada menu yang habis"},
			"order_items":[{"quantity":2,"price":1.5e4,"notes":"pedas","name":"Hamburger","id":"i1","external_id":"1773640"}]}}}`
		m := hmac.New(sha256.New, []byte(rahasia))
		m.Write([]byte(body))
		req := httptest.NewRequest("POST", "/webhooks/channels/gofood/"+token, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Go-Signature", hex.EncodeToString(m.Sum(nil)))
		req.RemoteAddr = ipPenyedia
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		out := apiResp{Code: rec.Code, Raw: rec.Body.String()}
		_ = json.Unmarshal(rec.Body.Bytes(), &out.Body)
		return out
	}

	kirim("gofood.order.merchant_accepted", "F-100", "bukan-rahasianya").mustCode(t, "tanda tangan palsu", 401)
	if r := kirim("gofood.order.awaiting_merchant_acceptance", "F-100", "rahasia-notif").mustOK(t, "menunggu").data(t); r["ignored"] != "bukan pesanan" {
		t.Fatalf("menunggu diterima tidak boleh jadi penjualan: %v", r)
	}
	assertI64(t, kirim("gofood.order.merchant_accepted", "F-100", "rahasia-notif").mustOK(t, "diterima").data(t), "received", 1)
	assertI64(t, kirim("gofood.order.driver_otw_pickup", "F-100", "rahasia-notif").mustOK(t, "otw").data(t), "received", 1)
	processChannelEvents(t, f.token)

	pesanan := func(nomor string) map[string]any {
		for _, x := range call(t, "GET", "/api/v1/channel-orders?channel_id="+ch, f.token, nil).
			mustOK(t, "pesanan").data(t)["data"].([]any) {
			if m := x.(map[string]any); m["external_order_id"] == nomor {
				return m
			}
		}
		return nil
	}
	p := pesanan("F-100")
	if p == nil || p["buyer_name"] != "Pelanggan GoFood" || p["courier"] != "GoFood · Pak Ojol" {
		t.Fatalf("pesanan F-100: %v", p)
	}
	assertI64(t, p, "gross_amount", 30000)
	assertI64(t, p, "fee_amount", 6000) // komisi kanal 20%
	if p["external_status"] != "driver_otw_pickup" {
		t.Fatalf("status pengemudi tidak tercatat: %v", p["external_status"])
	}

	// Batal dari GoFood → penjualan dibatalkan; batal untuk pesanan yang tak
	// pernah diterima (F-999) diabaikan tanpa mati di antrean.
	kirim("gofood.order.cancelled", "F-100", "rahasia-notif").mustOK(t, "batal")
	kirim("gofood.order.cancelled", "F-999", "rahasia-notif").mustOK(t, "batal tak dikenal")
	processChannelEvents(t, f.token)
	if p = pesanan("F-100"); p["sale_status"] != "canceled" {
		t.Fatalf("pembatalan GoFood tidak membatalkan penjualan: %v", p)
	}
	mati := call(t, "GET", "/api/v1/channels/"+ch+"/events?status=dead", f.token, nil).
		mustOK(t, "antrean mati").data(t)["data"].([]any)
	if len(mati) != 0 {
		t.Fatalf("ada peristiwa mati: %v", mati)
	}
}
