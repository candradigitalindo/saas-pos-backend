package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Sambungan GrabFood (Partner API) dengan kredensial MILIK TENANT.
//
// Arahnya berkebalikan dengan GoFood: Grab meminta token ke "server partner"
// (sub-jalur /oauth/token alamat webhook kanal, dengan Partner Client ID &
// Secret buatan platform), lalu mengirim pesanan & status dengan token itu.
// Yang dijaga: token hanya untuk kredensial yang benar; pesanan/status tanpa
// token sah ditolak; harga minor unit (eksponen 2) jadi rupiah; status
// pengemudi tercatat; CANCELLED membatalkan penjualannya.
func TestSambunganGrabFoodDariKredensialTenant(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "sambung-grab")
	call(t, "PUT", "/api/v1/products/"+f.prodA, f.token, map[string]any{"sku": "GF-A"}).mustOK(t, "sku A")
	ch := makeChannel(t, f, "GrabFood", "0.25")

	grab := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/grabid/v1/oauth2/token":
			var b map[string]string
			_ = json.NewDecoder(r.Body).Decode(&b)
			if b["client_id"] != "grab-klien" || b["client_secret"] != "grab-rahasia" || b["scope"] != "food.partner_api" {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"invalid_client","error_description":"Bad client credentials"}`))
				return
			}
			_, _ = w.Write([]byte(`{"access_token":"tok-grab","token_type":"Bearer","expires_in":604799}`))
		case r.Header.Get("Authorization") == "Bearer tok-grab" && r.URL.Path == "/partner/v1/merchants/1-CYNGRUNGSBAAAA/store/status":
			_, _ = w.Write([]byte(`{"closeReason":"","isInSpecialOpeningHourRange":false,"isOpen":true}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer grab.Close()
	t.Setenv("CHANNEL_GRAB_API_URL", grab.URL)
	t.Setenv("CHANNEL_GRAB_OAUTH_URL", grab.URL)

	jalur := "/api/v1/channels/" + ch + "/connection"
	call(t, "PUT", jalur, f.token, map[string]any{"provider": "grabfood", "fields": map[string]any{
		"client_id": "grab-klien", "client_secret": "grab-rahasia", "merchant_id": "1-CYNGRUNGSBAAAA",
	}}).mustOK(t, "simpan grab")
	uji := call(t, "POST", jalur+"/test", f.token, nil).mustOK(t, "tes").data(t)
	if uji["status"] != "connected" || !strings.Contains(uji["info"].(string), "toko buka") {
		t.Fatalf("tes grab: %v", uji)
	}
	nilai := map[string]string{}
	for _, x := range uji["webhook_values"].([]any) {
		m := x.(map[string]any)
		nilai[m["label"].(string)] = m["value"].(string)
	}
	webhook := uji["webhook_url"].(string)
	lokal := webhook[strings.Index(webhook, "/webhooks/"):]
	if nilai["Partner OAuth URL"] != webhook+"/oauth/token" || nilai["Partner Client ID"] == "" || len(nilai["Partner Client Secret"]) < 32 {
		t.Fatalf("nilai untuk konsol Grab: %v", nilai)
	}

	kirim := func(sub, auth, body string) apiResp {
		req := httptest.NewRequest("POST", lokal+sub, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		req.RemoteAddr = "203.0.113.52:4431" // IP sendiri: batas laju webhook per IP
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		out := apiResp{Code: rec.Code, Raw: rec.Body.String()}
		_ = json.Unmarshal(rec.Body.Bytes(), &out.Body)
		return out
	}

	// Grab meminta token ke "server partner".
	kirim("/oauth/token", "", `{"client_id":"`+nilai["Partner Client ID"]+`","client_secret":"salah","grant_type":"client_credentials"}`).
		mustCode(t, "token rahasia salah", 401)
	tok := kirim("/oauth/token", "", `{"client_id":"`+nilai["Partner Client ID"]+`","client_secret":"`+nilai["Partner Client Secret"]+
		`","grant_type":"client_credentials","scope":"food.partner_api"}`).mustOK(t, "token")
	akses, _ := tok.Body["access_token"].(string)
	if akses == "" || tok.Body["token_type"] != "Bearer" {
		t.Fatalf("balasan token: %v", tok.Raw)
	}

	pesanan := `{"orderID":"123-CYNKLPCVRN5ZMY","shortOrderNumber":"GF-123","merchantID":"1-CYNGRUNGSBAAAA",
		"paymentType":"CASHLESS","cutlery":false,"orderTime":"2026-09-28T03:04:05Z",
		"currency":{"code":"IDR","symbol":"Rp","exponent":2},
		"featureFlags":{"orderAcceptedType":"AUTO","orderType":"Delivery"},
		"items":[{"id":"GF-A","grabItemID":"IDITE20190101","quantity":2,"price":1500000,"tax":0}],
		"price":{"subtotal":3000000},
		"receiver":{"name":"Budi Grab","phones":"6281234000111","address":{"address":"Jl. Uji No. 1"}}}`
	kirim("", "", pesanan).mustCode(t, "tanpa token", 401)
	kirim("", "Bearer gf1.9999999999.00", pesanan).mustCode(t, "token palsu", 401)
	assertI64(t, kirim("/order", "Bearer "+akses, pesanan).mustOK(t, "submit order").data(t), "received", 1)
	status := func(s string) string {
		return `{"merchantID":"1-CYNGRUNGSBAAAA","orderID":"123-CYNKLPCVRN5ZMY","state":"` + s + `","message":"uji"}`
	}
	assertI64(t, kirim("/order", "Bearer "+akses, status("DRIVER_ALLOCATED")).mustOK(t, "status").data(t), "received", 1)
	processChannelEvents(t, f.token)

	var p map[string]any
	for _, x := range call(t, "GET", "/api/v1/channel-orders?channel_id="+ch, f.token, nil).mustOK(t, "pesanan").data(t)["data"].([]any) {
		p = x.(map[string]any)
	}
	if p == nil || p["external_order_id"] != "123-CYNKLPCVRN5ZMY" || p["buyer_name"] != "Budi Grab" ||
		p["courier"] != "GrabFood · #GF-123" || p["shipping_address"] != "Jl. Uji No. 1" || p["external_status"] != "driver_allocated" {
		t.Fatalf("pesanan grab: %v", p)
	}
	assertI64(t, p, "gross_amount", 30000) // 2 × 1.500.000 minor (eksponen 2) = 2 × Rp 15.000
	assertI64(t, p, "fee_amount", 7500)    // komisi kanal 25%

	kirim("/order", "Bearer "+akses, status("CANCELLED")).mustOK(t, "batal")
	processChannelEvents(t, f.token)
	for _, x := range call(t, "GET", "/api/v1/channel-orders?channel_id="+ch, f.token, nil).mustOK(t, "pesanan").data(t)["data"].([]any) {
		p = x.(map[string]any)
	}
	if p["sale_status"] != "canceled" {
		t.Fatalf("CANCELLED dari Grab tidak membatalkan penjualan: %v", p)
	}
}
