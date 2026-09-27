package tests

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
)

// Sambungan Shopee (Open Platform v2) dengan aplikasi MILIK TENANT.
//
// Shopee tiruan memeriksa tanda tangan setiap panggilan. Yang dijaga: toko
// memberi izin lewat callback ber-state; kode ditukar token; push hanya
// diterima dengan Authorization = HMAC(partner_key, URL|body) dan dibalas 200
// TANPA isi; rincian pesanan diambil pekerja (push hanya membawa nomor);
// refresh token yang SEKALI PAKAI diperbarui dan yang baru tersimpan;
// pembatalan dari Shopee membatalkan penjualannya.
func TestSambunganShopeeDariAplikasiTenant(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "sambung-shopee")
	call(t, "PUT", "/api/v1/products/"+f.prodA, f.token, map[string]any{"sku": "SP-A"}).mustOK(t, "sku A")
	ch := makeChannel(t, f, "Shopee", "0.08")

	const kunci = "kunci-partner-toko"
	tanda := func(bagian ...string) string {
		m := hmac.New(sha256.New, []byte(kunci))
		m.Write([]byte(strings.Join(bagian, "")))
		return hex.EncodeToString(m.Sum(nil))
	}
	var mu sync.Mutex
	refreshTerpakai := map[string]bool{}
	jumlahRefresh := 0
	shopee := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		q := r.URL.Query()
		tolak := func(pesan string) {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"error":"error_auth","message":"` + pesan + `"}`))
		}
		var body map[string]any
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		switch r.URL.Path {
		case "/api/v2/auth/token/get", "/api/v2/auth/access_token/get":
			if q.Get("partner_id") != "123456" || q.Get("sign") != tanda("123456", r.URL.Path, q.Get("timestamp")) {
				tolak("Wrong sign")
				return
			}
			mu.Lock()
			defer mu.Unlock()
			if r.URL.Path == "/api/v2/auth/token/get" {
				if body["code"] != "kode-otorisasi" || body["shop_id"] != float64(777888) {
					tolak("Invalid code")
					return
				}
				// Umur 1 detik: panggilan berikutnya WAJIB memperbarui token.
				_, _ = w.Write([]byte(`{"error":"","message":"","access_token":"akses-1","refresh_token":"segar-1","expire_in":1}`))
				return
			}
			rt, _ := body["refresh_token"].(string)
			if rt != "segar-1" || refreshTerpakai[rt] {
				tolak("Invalid refresh_token")
				return
			}
			refreshTerpakai[rt] = true
			jumlahRefresh++
			_, _ = w.Write([]byte(`{"error":"","message":"","access_token":"akses-2","refresh_token":"segar-2","expire_in":14400}`))
		case "/api/v2/shop/get_shop_info", "/api/v2/order/get_order_detail":
			at := q.Get("access_token")
			if q.Get("shop_id") != "777888" || q.Get("sign") != tanda("123456", r.URL.Path, q.Get("timestamp"), at, "777888") {
				tolak("Wrong sign")
				return
			}
			if r.URL.Path == "/api/v2/shop/get_shop_info" {
				_, _ = w.Write([]byte(`{"error":"","message":"","shop_name":"Toko Uji Shopee","region":"ID","status":"NORMAL"}`))
				return
			}
			if at != "akses-2" {
				tolak("Invalid access_token") // token berumur 1 detik harus sudah diperbarui
				return
			}
			_, _ = w.Write([]byte(`{"error":"","message":"","response":{"order_list":[{"order_sn":"` + q.Get("order_sn_list") + `",
				"order_status":"READY_TO_SHIP","currency":"IDR","create_time":1790500000,"buyer_username":"rani_s",
				"recipient_address":{"name":"Rani","phone":"6281277778888","full_address":"Jl. Shopee No. 1"},
				"item_list":[{"item_id":111,"model_id":0,"item_sku":"SP-A","model_sku":"","model_quantity_purchased":2,
				"model_discounted_price":15000.0,"model_original_price":16000.0}]}]}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer shopee.Close()
	t.Setenv("CHANNEL_SHOPEE_API_URL", shopee.URL)
	t.Setenv("CHANNEL_SHOPEE_AUTH_URL", "https://open.contoh.id/auth")
	t.Setenv("APP_URL", "https://pos.contoh.id")

	jalur := "/api/v1/channels/" + ch + "/connection"
	s := call(t, "PUT", jalur, f.token, map[string]any{"provider": "shopee", "fields": map[string]any{
		"partner_id": "123456", "partner_key": kunci,
	}}).mustOK(t, "simpan shopee").data(t)
	if s["needs_authorization"] != true || s["authorized"] != nil {
		t.Fatalf("sebelum otorisasi: %v", s)
	}
	webhook := s["webhook_url"].(string)
	if v, _ := s["webhook_values"].([]any); len(v) != 1 || v[0].(map[string]any)["value"] != "https://pos.contoh.id" {
		t.Fatalf("Redirect URL Domain: %v", s["webhook_values"])
	}
	lokal := webhook[strings.Index(webhook, "/webhooks/"):]
	if u := call(t, "POST", jalur+"/test", f.token, nil).mustOK(t, "tes awal").data(t); u["status"] != "error" ||
		!strings.Contains(u["error"].(string), "Otorisasi") {
		t.Fatalf("tes sebelum otorisasi: %v", u)
	}

	alamat := call(t, "POST", jalur+"/authorize", f.token, nil).mustOK(t, "alamat otorisasi").data(t)["url"].(string)
	au, _ := url.Parse(alamat)
	if au.Host != "open.contoh.id" || au.Query().Get("partner_id") != "123456" || au.Query().Get("auth_type") != "seller" ||
		au.Query().Get("redirect_uri") != webhook+"/oauth/callback" || au.Query().Get("state") == "" {
		t.Fatalf("alamat otorisasi: %s", alamat)
	}
	callback := func(state string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", lokal+"/oauth/callback?code=kode-otorisasi&shop_id=777888&state="+state, nil)
		req.RemoteAddr = ipPenyedia
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	if rec := callback("state-palsu"); rec.Code != 400 || !strings.Contains(rec.Body.String(), "tidak cocok") {
		t.Fatalf("callback state palsu: %d %s", rec.Code, rec.Body.String())
	}
	if rec := callback(au.Query().Get("state")); rec.Code != 200 || !strings.Contains(rec.Body.String(), "Toko Uji Shopee") {
		t.Fatalf("callback: %d %s", rec.Code, rec.Body.String())
	}
	// State sekali pakai: tautan callback yang sama tidak bisa diputar ulang.
	if rec := callback(au.Query().Get("state")); rec.Code != 400 || !strings.Contains(rec.Body.String(), "tidak cocok") {
		t.Fatalf("callback diputar ulang: %d %s", rec.Code, rec.Body.String())
	}
	g := call(t, "GET", jalur, f.token, nil).mustOK(t, "setelah otorisasi").data(t)
	if g["status"] != "connected" || g["authorized"] != "Toko Uji Shopee" {
		t.Fatalf("setelah otorisasi: %v", g)
	}

	push := func(sn, status, rahasia string) *httptest.ResponseRecorder {
		body := `{"data":{"items":[],"ordersn":"` + sn + `","status":"` + status + `","completed_scenario":"","update_time":1790500100},"shop_id":777888,"code":3,"timestamp":1790500100}`
		m := hmac.New(sha256.New, []byte(rahasia))
		m.Write([]byte(webhook + "|" + body))
		req := httptest.NewRequest("POST", lokal, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", hex.EncodeToString(m.Sum(nil)))
		req.RemoteAddr = ipPenyedia
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	if rec := push("SN1", "READY_TO_SHIP", "bukan-kuncinya"); rec.Code != 401 {
		t.Fatalf("push palsu harus 401, dapat %d", rec.Code)
	}
	if rec := push("SN1", "UNPAID", kunci); rec.Code != 200 || rec.Body.Len() != 0 {
		t.Fatalf("push belum dibayar: %d %q (Shopee mewajibkan body kosong)", rec.Code, rec.Body.String())
	}
	if rec := push("SN1", "READY_TO_SHIP", kunci); rec.Code != 200 || rec.Body.Len() != 0 {
		t.Fatalf("push siap kirim: %d %q", rec.Code, rec.Body.String())
	}
	processChannelEvents(t, f.token)

	pesanan := func() map[string]any {
		for _, x := range call(t, "GET", "/api/v1/channel-orders?channel_id="+ch, f.token, nil).mustOK(t, "pesanan").data(t)["data"].([]any) {
			return x.(map[string]any)
		}
		return nil
	}
	p := pesanan()
	if p == nil || p["external_order_id"] != "SN1" || p["buyer_name"] != "Rani" || p["shipping_address"] != "Jl. Shopee No. 1" ||
		p["external_status"] != "ready_to_ship" {
		t.Fatalf("pesanan shopee: %v", p)
	}
	assertI64(t, p, "gross_amount", 30000)
	assertI64(t, p, "fee_amount", 2400) // komisi kanal 8%
	if jumlahRefresh != 1 {
		t.Fatalf("refresh token = %d kali, mau 1", jumlahRefresh)
	}

	// Batal dari Shopee → penjualan dibatalkan.
	push("SN1", "CANCELLED", kunci)
	processChannelEvents(t, f.token)
	if p = pesanan(); p["sale_status"] != "canceled" {
		t.Fatalf("pembatalan Shopee tidak membatalkan penjualan: %v", p)
	}
	// Token baru (segar-2) tersimpan: tes koneksi tidak memakai refresh token lama lagi.
	if u := call(t, "POST", jalur+"/test", f.token, nil).mustOK(t, "tes akhir").data(t); u["status"] != "connected" {
		t.Fatalf("tes setelah refresh: %v", u)
	}
	if mati := call(t, "GET", "/api/v1/channels/"+ch+"/events?status=dead", f.token, nil).mustOK(t, "mati").data(t)["data"].([]any); len(mati) != 0 {
		t.Fatalf("peristiwa mati: %v", mati)
	}
}
