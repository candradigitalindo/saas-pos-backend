package tests

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// Sambungan Tokopedia & Shop (TikTok Shop Open API) dengan Custom App MILIK
// TOKO.
//
// Platform tiruan memeriksa tanda tangan setiap panggilan dengan algoritma
// dokumen resmi. Yang dijaga: izin toko lewat callback ber-state (penolakan
// penjual = code=null); token dari token/get dan diperbarui (kedaluwarsa =
// unix timestamp); webhook didaftarkan otomatis per toko; webhook hanya
// diterima dengan Authorization = HMAC(app_secret, app_key+body); rincian
// diambil pekerja dan SATU line_item = SATU unit; pembatalan dari platform
// membatalkan penjualan; pencabutan izin memutus sambungan.
func TestSambunganTokopediaDariAplikasiTenant(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "sambung-tokopedia")
	call(t, "PUT", "/api/v1/products/"+f.prodA, f.token, map[string]any{"sku": "TT-A"}).mustOK(t, "sku A")
	ch := makeChannel(t, f, "Tokopedia", "0.05")

	const kunciApp, rahasiaApp = "kunci-app-toko", "rahasia-app-toko"
	tandaAPI := func(r *http.Request, body []byte) string {
		q := r.URL.Query()
		var ks []string
		for k := range q {
			if k != "sign" && k != "access_token" {
				ks = append(ks, k)
			}
		}
		sort.Strings(ks)
		s := r.URL.Path
		for _, k := range ks {
			s += k + q.Get(k)
		}
		m := hmac.New(sha256.New, []byte(rahasiaApp))
		m.Write([]byte(rahasiaApp + s + string(body) + rahasiaApp))
		return hex.EncodeToString(m.Sum(nil))
	}
	var mu sync.Mutex
	jumlahRefresh := 0
	topik := map[string]string{}
	platform := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		q := r.URL.Query()
		balas := func(data string) {
			_, _ = w.Write([]byte(`{"code":0,"message":"Success","request_id":"r1","data":` + data + `}`))
		}
		tolak := func(kode int, pesan string) {
			_, _ = w.Write([]byte(`{"code":` + strconv.Itoa(kode) + `,"message":"` + pesan + `","request_id":"r1"}`))
		}
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		defer mu.Unlock()
		switch r.URL.Path {
		case "/api/v2/token/get", "/api/v2/token/refresh":
			if q.Get("app_key") != kunciApp || q.Get("app_secret") != rahasiaApp {
				tolak(36004004, "invalid app")
				return
			}
			if r.URL.Path == "/api/v2/token/get" {
				if q.Get("auth_code") != "kode-izin" || q.Get("grant_type") != "authorized_code" {
					tolak(36004005, "invalid auth_code")
					return
				}
				// Kedaluwarsa 1 detik lagi: panggilan berikutnya WAJIB memperbarui.
				balas(`{"access_token":"akses-1","access_token_expire_in":` + strconv.FormatInt(time.Now().Unix()+1, 10) +
					`,"refresh_token":"segar-1","refresh_token_expire_in":1893456000,"seller_name":"Bu Sari","user_type":0}`)
				return
			}
			if q.Get("refresh_token") != "segar-1" || q.Get("grant_type") != "refresh_token" {
				tolak(36004006, "invalid refresh_token")
				return
			}
			jumlahRefresh++
			balas(`{"access_token":"akses-2","access_token_expire_in":` + strconv.FormatInt(time.Now().Unix()+7*86400, 10) +
				`,"refresh_token":"segar-1","refresh_token_expire_in":1893456000,"seller_name":"Bu Sari","user_type":0}`)
			return
		}
		if q.Get("app_key") != kunciApp || q.Get("sign") != tandaAPI(r, body) {
			tolak(106001, "Invalid signature")
			return
		}
		at := r.Header.Get("x-tts-access-token")
		switch {
		case r.URL.Path == "/authorization/202309/shops" && (at == "akses-1" || at == "akses-2"):
			balas(`{"shops":[{"id":"7494049642642441621","name":"Toko Uji Tokopedia","region":"ID","seller_type":"LOCAL","cipher":"ROW_uji","code":"IDLCUJI"}]}`)
		case at != "akses-2":
			tolak(105002, "Expired credentials") // token 1 detik harus sudah diperbarui
		case q.Get("shop_cipher") != "ROW_uji":
			tolak(106011, "Invalid shop_cipher")
		case r.URL.Path == "/event/202309/webhooks" && r.Method == http.MethodPut:
			var b struct {
				Address   string `json:"address"`
				EventType string `json:"event_type"`
			}
			_ = json.Unmarshal(body, &b)
			topik[b.EventType] = b.Address
			balas(`{}`)
		case r.URL.Path == "/order/202507/orders":
			li := func(harga string) string {
				return `{"id":"li` + harga + `","sku_id":"1729","seller_sku":"TT-A","product_name":"Kopi","sale_price":"` + harga + `","original_price":"16000","currency":"IDR"}`
			}
			balas(`{"orders":[{"id":"` + q.Get("ids") + `","status":"AWAITING_SHIPMENT","create_time":1790500000,"paid_time":1790500060,
				"shipping_provider":"JNE","commerce_platform":"TOKOPEDIA","payment":{"currency":"IDR","sub_total":"44000"},
				"recipient_address":{"name":"Rina","phone_number":"(+62)812****88","full_address":"Jl. Toko No. 2"},
				"line_items":[` + li("15000") + `,` + li("15000") + `,` + li("14000") + `]}]}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer platform.Close()
	t.Setenv("CHANNEL_TOKOPEDIA_API_URL", platform.URL)
	t.Setenv("CHANNEL_TOKOPEDIA_TOKEN_URL", platform.URL)
	t.Setenv("CHANNEL_TOKOPEDIA_AUTH_URL", "https://services.contoh.id/open/authorize")
	t.Setenv("APP_URL", "https://pos.contoh.id")

	jalur := "/api/v1/channels/" + ch + "/connection"
	s := call(t, "PUT", jalur, f.token, map[string]any{"provider": "tokopedia", "fields": map[string]any{
		"app_key": kunciApp, "app_secret": rahasiaApp, "service_id": "7001234",
	}}).mustOK(t, "simpan").data(t)
	webhook := s["webhook_url"].(string)
	if s["needs_authorization"] != true || !strings.Contains(webhook, "/webhooks/channels/tokopedia/") {
		t.Fatalf("sebelum otorisasi: %v", s)
	}
	if v, _ := s["webhook_values"].([]any); len(v) != 1 || v[0].(map[string]any)["value"] != webhook+"/oauth/callback" {
		t.Fatalf("Redirect URL: %v", s["webhook_values"])
	}
	lokal := webhook[strings.Index(webhook, "/webhooks/"):]
	if u := call(t, "POST", jalur+"/test", f.token, nil).mustOK(t, "tes awal").data(t); u["status"] != "error" ||
		!strings.Contains(u["error"].(string), "Otorisasi") {
		t.Fatalf("tes sebelum otorisasi: %v", u)
	}

	alamat := call(t, "POST", jalur+"/authorize", f.token, nil).mustOK(t, "alamat otorisasi").data(t)["url"].(string)
	au, _ := url.Parse(alamat)
	state := au.Query().Get("state")
	if au.Host != "services.contoh.id" || au.Query().Get("service_id") != "7001234" || state == "" {
		t.Fatalf("alamat otorisasi: %s", alamat)
	}
	callback := func(kueri string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", lokal+"/oauth/callback?"+kueri, nil)
		req.RemoteAddr = ipPenyedia
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	if rec := callback("code=kode-izin&state=palsu"); rec.Code != 400 || !strings.Contains(rec.Body.String(), "tidak cocok") {
		t.Fatalf("state palsu: %d %s", rec.Code, rec.Body.String())
	}
	if rec := callback("code=null&error=auth_denied&state=" + state); rec.Code != 400 || !strings.Contains(rec.Body.String(), "dibatalkan") {
		t.Fatalf("penjual menolak: %d %s", rec.Code, rec.Body.String())
	}
	if rec := callback("code=kode-izin&state=" + state); rec.Code != 200 || !strings.Contains(rec.Body.String(), "Toko Uji Tokopedia") ||
		!strings.Contains(rec.Body.String(), "masuk otomatis") {
		t.Fatalf("callback: %d %s", rec.Code, rec.Body.String())
	}
	if rec := callback("code=kode-izin&state=" + state); rec.Code != 400 {
		t.Fatalf("callback diputar ulang harus ditolak, dapat %d", rec.Code)
	}
	for _, tp := range []string{"ORDER_STATUS_CHANGE", "CANCELLATION_STATUS_CHANGE", "SELLER_DEAUTHORIZATION"} {
		if topik[tp] != webhook {
			t.Fatalf("webhook %s tidak didaftarkan ke %s: %v", tp, webhook, topik)
		}
	}
	g := call(t, "GET", jalur, f.token, nil).mustOK(t, "setelah otorisasi").data(t)
	if g["status"] != "connected" || g["authorized"] != "Toko Uji Tokopedia" {
		t.Fatalf("setelah otorisasi: %v", g)
	}

	kirim := func(body, rahasia string) *httptest.ResponseRecorder {
		m := hmac.New(sha256.New, []byte(rahasia))
		m.Write([]byte(kunciApp + body))
		req := httptest.NewRequest("POST", lokal, bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", hex.EncodeToString(m.Sum(nil)))
		req.RemoteAddr = ipPenyedia
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	status := func(sn, st string) string {
		return `{"type":1,"tts_notification_id":"n` + st + `","shop_id":"7494049642642441621","timestamp":1790500100,"data":{"order_id":"` + sn +
			`","order_status":"` + st + `","is_on_hold_order":false,"update_time":1790500100}}`
	}
	if rec := kirim(status("TT1", "AWAITING_SHIPMENT"), "bukan-rahasianya"); rec.Code != 401 {
		t.Fatalf("webhook palsu harus 401, dapat %d", rec.Code)
	}
	for _, st := range []string{"UNPAID", "ON_HOLD"} {
		if rec := kirim(status("TT1", st), rahasiaApp); rec.Code != 200 || !strings.Contains(rec.Body.String(), "bukan pesanan") {
			t.Fatalf("%s: %d %s", st, rec.Code, rec.Body.String())
		}
	}
	if rec := kirim(status("TT1", "AWAITING_SHIPMENT"), rahasiaApp); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"received":2`) {
		t.Fatalf("siap kirim: %d %s", rec.Code, rec.Body.String())
	}
	processChannelEvents(t, f.token)

	pesanan := func() map[string]any {
		for _, x := range call(t, "GET", "/api/v1/channel-orders?channel_id="+ch, f.token, nil).mustOK(t, "pesanan").data(t)["data"].([]any) {
			return x.(map[string]any)
		}
		return nil
	}
	p := pesanan()
	if p == nil || p["external_order_id"] != "TT1" || p["buyer_name"] != "Rina" || p["shipping_address"] != "Jl. Toko No. 2" ||
		p["courier"] != "JNE" || p["external_status"] != "awaiting_shipment" {
		t.Fatalf("pesanan: %v", p)
	}
	assertI64(t, p, "gross_amount", 44000) // 2 × 15.000 + 1 × 14.000 (satu line_item = satu unit)
	assertI64(t, p, "fee_amount", 2200)    // komisi kanal 5%
	if items, _ := p["items"].([]any); len(items) != 2 {
		t.Fatalf("baris barang = %v, mau 2 (per SKU & harga)", p["items"])
	}
	if jumlahRefresh != 1 {
		t.Fatalf("refresh token = %d kali, mau 1", jumlahRefresh)
	}
	// "Pesanan otomatis terakhir …" = waktu webhook terakhir diterima, bukan waktu nol.
	if akhir, _ := call(t, "GET", jalur, f.token, nil).mustOK(t, "sambungan").data(t)["last_event_at"].(string); !strings.HasPrefix(akhir, time.Now().UTC().Format("2006-01-02")) {
		t.Fatalf("last_event_at = %q, mau hari ini", akhir)
	}

	// Pembatalan disetujui → penjualan batal; status CANCEL susulan = duplikat.
	batal := `{"type":11,"tts_notification_id":"nbatal","shop_id":"7494049642642441621","timestamp":1790500200,"data":{"order_id":"TT1",` +
		`"cancellations_role":"BUYER","cancel_status":"CANCELLATION_REQUEST_SUCCESS","cancel_id":"40353","create_time":1790500150}}`
	kirim(batal, rahasiaApp)
	if rec := kirim(status("TT1", "CANCEL"), rahasiaApp); !strings.Contains(rec.Body.String(), `"duplicate":1`) {
		t.Fatalf("CANCEL susulan: %s", rec.Body.String())
	}
	processChannelEvents(t, f.token)
	if p = pesanan(); p["sale_status"] != "canceled" {
		t.Fatalf("pembatalan tidak membatalkan penjualan: %v", p)
	}
	if u := call(t, "POST", jalur+"/test", f.token, nil).mustOK(t, "tes").data(t); u["status"] != "connected" ||
		!strings.Contains(u["info"].(string), "Toko Uji Tokopedia · ID · kode IDLCUJI") || !strings.Contains(u["info"].(string), "webhook terdaftar") {
		t.Fatalf("tes setelah otorisasi: %v", u)
	}

	// Toko mencabut izin → sambungan galat, izin harus diberikan ulang.
	cabut := `{"type":6,"tts_notification_id":"ncabut","shop_id":"7494049642642441621","timestamp":1790500300,"data":{"message":"Shop_id 7494049642642441621 is deauthorized from your APP by merchant."}}`
	if rec := kirim(cabut, rahasiaApp); rec.Code != 200 || !strings.Contains(rec.Body.String(), "izin toko dicabut") {
		t.Fatalf("pencabutan: %d %s", rec.Code, rec.Body.String())
	}
	g = call(t, "GET", jalur, f.token, nil).mustOK(t, "setelah dicabut").data(t)
	if g["status"] != "error" || !strings.Contains(g["error"].(string), "mencabut izin") || g["authorized"] != nil {
		t.Fatalf("setelah dicabut: %v", g)
	}
	if mati := call(t, "GET", "/api/v1/channels/"+ch+"/events?status=dead", f.token, nil).mustOK(t, "mati").data(t)["data"].([]any); len(mati) != 0 {
		t.Fatalf("peristiwa mati: %v", mati)
	}
}
