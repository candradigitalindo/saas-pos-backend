package tests

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"candra/backend-api/services"
)

// Sambungan Lazada (Open Platform) dengan aplikasi Seller In-house MILIK TOKO.
//
// Lazada tiruan memeriksa tanda tangan setiap panggilan (hex besar, token ikut
// ditandatangani). Yang dijaga: izin toko lewat callback ber-state dengan
// redirect_uri = App Callback URL; token/create (expires_in berupa TEKS) lalu
// token/refresh; push hanya diterima dengan Authorization = HMAC(App Secret,
// App Key + body); status per BARANG — batal sebagian tidak membatalkan
// penjualan, batal seluruhnya membatalkan; pesanan yang tidak ter-push tetap
// masuk lewat tarikan berkala, yang menghormati jeda 15 menit.
func TestSambunganLazadaDariAplikasiTenant(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "sambung-lazada")
	call(t, "PUT", "/api/v1/products/"+f.prodA, f.token, map[string]any{"sku": "LZ-A"}).mustOK(t, "sku A")
	ch := makeChannel(t, f, "Lazada", "0.10")

	const kunciApp, rahasiaApp = "123789", "rahasia-lazada-toko"
	tanda := func(api string, q url.Values) string {
		var ks []string
		for k := range q {
			if k != "sign" && q.Get(k) != "" {
				ks = append(ks, k)
			}
		}
		sort.Strings(ks)
		s := api
		for _, k := range ks {
			s += k + q.Get(k)
		}
		m := hmac.New(sha256.New, []byte(rahasiaApp))
		m.Write([]byte(s))
		return strings.ToUpper(hex.EncodeToString(m.Sum(nil)))
	}
	var mu sync.Mutex
	jumlahRefresh, tarikan := 0, 0
	semuaBatal := false
	barang := func(status string) string {
		return `{"order_item_id":"` + status + `","sku":"LZ-A","shop_sku":"123_ID-9","item_price":"20000.00","paid_price":"17000.00",` +
			`"voucher_seller":"2000.00","voucher_platform":"1000.00","currency":"IDR","status":"` + status + `","shipment_provider":"LEX ID"}`
	}
	lazada := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		q := r.URL.Query()
		tulis := func(s string) { _, _ = w.Write([]byte(s)) }
		mu.Lock()
		defer mu.Unlock()
		if q.Get("app_key") != kunciApp || q.Get("sign_method") != "sha256" || q.Get("sign") != tanda(r.URL.Path, q) {
			tulis(`{"type":"ISV","code":"IncompleteSignature","message":"The request signature does not conform to platform standards","request_id":"r"}`)
			return
		}
		at := q.Get("access_token")
		switch {
		case r.URL.Path == "/auth/token/create":
			if q.Get("code") != "kode-lzd" {
				tulis(`{"type":"ISV","code":"InvalidCode","message":"Invalid authorization code","request_id":"r"}`)
				return
			}
			// expires_in TEKS & singkat: panggilan berikutnya WAJIB memperbarui.
			tulis(`{"access_token":"akses-1","refresh_token":"segar-1","expires_in":"10","refresh_expires_in":"15552000","country":"id",` +
				`"account":"sari@contoh.id","account_platform":"seller_center","code":"0","request_id":"r",` +
				`"country_user_info":[{"country":"ID","seller_id":"100200","user_id":"9","short_code":"IDLZ1"}]}`)
		case r.URL.Path == "/auth/token/refresh":
			if q.Get("refresh_token") != "segar-1" {
				tulis(`{"type":"ISV","code":"InvalidRefreshToken","message":"refresh token invalid","request_id":"r"}`)
				return
			}
			jumlahRefresh++
			tulis(`{"access_token":"akses-2","refresh_token":"segar-1","expires_in":2592000,"country":"id","code":"0","request_id":"r",` +
				`"country_user_info":[{"country":"ID","seller_id":"100200","user_id":"9","short_code":"IDLZ1"}]}`)
		case r.URL.Path == "/seller/get" && (at == "akses-1" || at == "akses-2"):
			tulis(`{"code":"0","data":{"name":"Toko Lazada Uji","short_code":"IDLZ1","seller_id":100200},"request_id":"r"}`)
		case at != "akses-2":
			tulis(`{"type":"ISP","code":"IllegalAccessToken","message":"The specified access token is invalid or expired","request_id":"r"}`)
		case r.URL.Path == "/order/get":
			tulis(`{"code":"0","data":{"order_id":"` + q.Get("order_id") + `","created_at":"2026-09-28 09:00:00 +0700","customer_first_name":"Wati",` +
				`"address_shipping":{"first_name":"Wati","last_name":"S","phone":"6281233","address1":"Jl. Kenari 3","address3":"Sumatera Utara",` +
				`"address4":"Medan","city":"Medan","post_code":"20111"},"statuses":["pending"]},"request_id":"r"}`)
		case r.URL.Path == "/order/items/get":
			switch {
			case q.Get("order_id") == "LZ2":
				tulis(`{"code":"0","data":[` + barang("pending") + `],"request_id":"r"}`)
			case semuaBatal:
				tulis(`{"code":"0","data":[` + barang("canceled") + `,` + barang("canceled") + `,` + barang("canceled") + `],"request_id":"r"}`)
			default: // dua barang aktif, satu sudah dibatalkan
				tulis(`{"code":"0","data":[` + barang("pending") + `,` + barang("pending") + `,` + barang("canceled") + `],"request_id":"r"}`)
			}
		case r.URL.Path == "/orders/get":
			if _, err := time.Parse(time.RFC3339, q.Get("update_after")); err != nil || q.Get("sort_by") != "updated_at" {
				tulis(`{"type":"ISV","code":"MissingParameter","message":"update_after","request_id":"r"}`)
				return
			}
			tarikan++
			tulis(`{"code":"0","data":{"count":"1","countTotal":"1","orders":[{"order_id":"LZ2","statuses":["pending"],` +
				`"updated_at":"2026-09-28T09:30:00+07:00","created_at":"2026-09-28T09:29:00+07:00"}]},"request_id":"r"}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer lazada.Close()
	t.Setenv("CHANNEL_LAZADA_API_URL", lazada.URL)
	t.Setenv("CHANNEL_LAZADA_TOKEN_URL", lazada.URL)
	t.Setenv("CHANNEL_LAZADA_AUTH_URL", "https://auth.contoh.id/oauth/authorize")
	t.Setenv("APP_URL", "https://pos.contoh.id")

	jalur := "/api/v1/channels/" + ch + "/connection"
	s := call(t, "PUT", jalur, f.token, map[string]any{"provider": "lazada", "fields": map[string]any{
		"app_key": kunciApp, "app_secret": rahasiaApp,
	}}).mustOK(t, "simpan").data(t)
	webhook := s["webhook_url"].(string)
	if s["needs_authorization"] != true {
		t.Fatalf("sebelum otorisasi: %v", s)
	}
	if v, _ := s["webhook_values"].([]any); len(v) != 1 || v[0].(map[string]any)["value"] != webhook+"/oauth/callback" {
		t.Fatalf("App Callback URL: %v", s["webhook_values"])
	}
	lokal := webhook[strings.Index(webhook, "/webhooks/"):]

	alamat := call(t, "POST", jalur+"/authorize", f.token, nil).mustOK(t, "alamat otorisasi").data(t)["url"].(string)
	au, _ := url.Parse(alamat)
	state := au.Query().Get("state")
	if au.Host != "auth.contoh.id" || au.Query().Get("client_id") != kunciApp || au.Query().Get("redirect_uri") != webhook+"/oauth/callback" ||
		au.Query().Get("response_type") != "code" || au.Query().Get("country") != "id" || state == "" {
		t.Fatalf("alamat otorisasi: %s", alamat)
	}
	callback := func(kueri string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", lokal+"/oauth/callback?"+kueri, nil)
		req.RemoteAddr = "203.0.113.53:4431" // IP sendiri: batas laju webhook per IP
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	if rec := callback("code=kode-lzd&state=palsu"); rec.Code != 400 {
		t.Fatalf("state palsu harus 400, dapat %d", rec.Code)
	}
	if rec := callback("state=" + state); rec.Code != 400 || !strings.Contains(rec.Body.String(), "dibatalkan") {
		t.Fatalf("tanpa code: %d %s", rec.Code, rec.Body.String())
	}
	if rec := callback("code=kode-lzd&state=" + state); rec.Code != 200 || !strings.Contains(rec.Body.String(), "Toko Lazada Uji") {
		t.Fatalf("callback: %d %s", rec.Code, rec.Body.String())
	}
	g := call(t, "GET", jalur, f.token, nil).mustOK(t, "setelah otorisasi").data(t)
	if g["status"] != "connected" || g["authorized"] != "Toko Lazada Uji" {
		t.Fatalf("setelah otorisasi: %v", g)
	}

	push := func(status, baris, rahasia string) *httptest.ResponseRecorder {
		body := `{"seller_id":"100200","message_type":0,"data":{"order_status":"` + status + `","trade_order_id":"LZ1","trade_order_line_id":"` +
			baris + `","status_update_time":1790560000},"timestamp":1790560001000,"site":"lazada_id"}`
		m := hmac.New(sha256.New, []byte(rahasia))
		m.Write([]byte(kunciApp + body))
		req := httptest.NewRequest("POST", lokal, bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", hex.EncodeToString(m.Sum(nil)))
		req.RemoteAddr = "203.0.113.53:4431" // IP sendiri: batas laju webhook per IP
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	if rec := push("pending", "L1", "bukan-rahasianya"); rec.Code != 401 {
		t.Fatalf("push palsu harus 401, dapat %d", rec.Code)
	}
	if rec := push("unpaid", "L1", rahasiaApp); !strings.Contains(rec.Body.String(), "bukan pesanan") {
		t.Fatalf("belum dibayar: %s", rec.Body.String())
	}
	for _, baris := range []string{"L1", "L2"} { // push per baris: yang kedua duplikat
		push("pending", baris, rahasiaApp)
	}
	processChannelEvents(t, f.token)

	pesanan := func(id string) map[string]any {
		for _, x := range call(t, "GET", "/api/v1/channel-orders?channel_id="+ch, f.token, nil).mustOK(t, "pesanan").data(t)["data"].([]any) {
			if p := x.(map[string]any); p["external_order_id"] == id {
				return p
			}
		}
		return nil
	}
	p := pesanan("LZ1")
	if p == nil || p["buyer_name"] != "Wati S" || p["shipping_address"] != "Jl. Kenari 3, Medan, Sumatera Utara, 20111" ||
		p["courier"] != "LEX ID" || p["external_status"] != "pending" {
		t.Fatalf("pesanan LZ1: %v", p)
	}
	// Dua barang aktif × (20.000 − voucher penjual 2.000); barang batal tidak ikut.
	assertI64(t, p, "gross_amount", 36000)
	assertI64(t, p, "fee_amount", 3600)
	if jumlahRefresh != 1 {
		t.Fatalf("refresh token = %d kali, mau 1", jumlahRefresh)
	}

	// Satu barang batal → penjualan tetap; seluruh barang batal → penjualan batal.
	push("canceled", "L3", rahasiaApp)
	processChannelEvents(t, f.token)
	if p = pesanan("LZ1"); p["sale_status"] == "canceled" {
		t.Fatalf("batal sebagian tidak boleh membatalkan penjualan: %v", p)
	}
	mu.Lock()
	semuaBatal = true
	mu.Unlock()
	if rec := push("canceled", "L1", rahasiaApp); !strings.Contains(rec.Body.String(), `"received":1`) {
		t.Fatalf("batal baris berikutnya harus peristiwa baru: %s", rec.Body.String())
	}
	processChannelEvents(t, f.token)
	if p = pesanan("LZ1"); p["sale_status"] != "canceled" {
		t.Fatalf("seluruh barang batal harus membatalkan penjualan: %v", p)
	}

	// Pesanan LZ2 tidak pernah di-push (mis. sertifikat server bukan OV/EV):
	// tetap masuk lewat tarikan berkala.
	ctx := context.Background()
	now := time.Now()
	if _, err := services.TarikPesananKanal(ctx, now); err != nil {
		t.Fatal(err)
	}
	if n, _ := services.TarikPesananKanal(ctx, now.Add(time.Minute)); n != 0 || tarikan != 1 {
		t.Fatalf("tarikan sebelum 15 menit harus dilewati: masuk=%d tarikan=%d", n, tarikan)
	}
	processChannelEvents(t, f.token)
	if p = pesanan("LZ2"); p == nil || p["external_status"] != "pending" {
		t.Fatalf("pesanan hasil tarikan: %v", p)
	}
	assertI64(t, p, "gross_amount", 18000)
	if n, _ := services.TarikPesananKanal(ctx, now.Add(16*time.Minute)); n != 0 || tarikan != 2 {
		t.Fatalf("tarikan berikutnya: masuk=%d (mau 0, sudah tercatat) tarikan=%d", n, tarikan)
	}
	if u := call(t, "POST", jalur+"/test", f.token, nil).mustOK(t, "tes").data(t); u["status"] != "connected" ||
		!strings.Contains(u["info"].(string), "Toko Lazada Uji · IDLZ1 · ditarik") {
		t.Fatalf("tes koneksi: %v", u)
	}
	if mati := call(t, "GET", "/api/v1/channels/"+ch+"/events?status=dead", f.token, nil).mustOK(t, "mati").data(t)["data"].([]any); len(mati) != 0 {
		t.Fatalf("peristiwa mati: %v", mati)
	}
}
