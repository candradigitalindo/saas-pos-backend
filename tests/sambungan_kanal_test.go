package tests

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Sambungan API kanal MILIK TENANT — WhatsApp Cloud API (pesanan katalog).
//
// Yang dijaga: kredensial tersimpan & tidak pernah dikirim balik utuh; tes
// koneksi memakai token tenant ke Graph API; webhook hanya diterima di alamat
// kanal itu dengan tanda tangan App Secret tenant; pesanan keranjang WhatsApp
// menjadi penjualan dengan SKU dicocokkan ke barang toko; jalur webhook lama
// yang tak bertanda tangan menolak kanal yang sudah tersambung.
func TestSambunganWhatsAppDariKredensialTenant(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "sambung-wa")
	call(t, "PUT", "/api/v1/products/"+f.prodA, f.token, map[string]any{"sku": "KOPI-A"}).mustOK(t, "sku A")
	ch := call(t, "POST", "/api/v1/channels", f.token, map[string]any{
		"outlet_id": f.outletID, "kind": "conversation", "provider": "WhatsApp", "name": "WhatsApp",
		"commission_rate": "0",
	}).mustCode(t, "buat kanal", 201).data(t)["id"].(string)

	// Graph API tiruan: hanya "token-benar" yang diterima.
	graph := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("Authorization") != "Bearer token-benar" || !strings.HasSuffix(r.URL.Path, "/10987654321") {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":{"message":"Invalid OAuth access token."}}`))
			return
		}
		_, _ = w.Write([]byte(`{"display_phone_number":"+62 812-0000-1111","verified_name":"Warung Uji","id":"10987654321"}`))
	}))
	defer graph.Close()
	t.Setenv("CHANNEL_WA_GRAPH_URL", graph.URL)

	// Katalog penyedia.
	var wa map[string]any
	for _, x := range call(t, "GET", "/api/v1/channel-providers", f.token, nil).mustOK(t, "penyedia").Body["data"].([]any) {
		if m := x.(map[string]any); m["code"] == "whatsapp" {
			wa = m
		}
	}
	if wa == nil || wa["available"] != true {
		t.Fatalf("whatsapp harus tersedia: %v", wa)
	}

	jalur := "/api/v1/channels/" + ch + "/connection"
	call(t, "PUT", jalur, f.token, map[string]any{
		"provider": "whatsapp", "fields": map[string]any{"phone_number_id": "10987654321", "access_token": "token-salah"},
	}).mustCode(t, "tanpa app secret", 422)

	sim := call(t, "PUT", jalur, f.token, map[string]any{
		"provider": "whatsapp",
		"fields":   map[string]any{"phone_number_id": "10987654321", "access_token": "token-salah", "app_secret": "rahasia-app"},
	}).mustOK(t, "simpan").data(t)
	if strings.Contains(jsonTeks(sim), "token-salah") || strings.Contains(jsonTeks(sim), "rahasia-app") {
		t.Fatalf("rahasia terkirim balik utuh: %v", sim)
	}
	url, _ := sim["webhook_url"].(string)
	if !strings.Contains(url, "/webhooks/channels/whatsapp/") {
		t.Fatalf("webhook_url = %q", url)
	}
	token := url[strings.LastIndex(url, "/")+1:]
	verify := sim["webhook_values"].([]any)[0].(map[string]any)["value"].(string)
	if verify == "" || sim["status"] != "none" {
		t.Fatalf("verify token / status awal: %v", sim)
	}

	// Tes koneksi: token salah → status error dengan pesan Meta.
	uji := call(t, "POST", jalur+"/test", f.token, nil).mustOK(t, "tes gagal").data(t)
	if uji["status"] != "error" || !strings.Contains(uji["error"].(string), "Invalid OAuth") {
		t.Fatalf("tes token salah: %v", uji)
	}
	// Ganti token saja — isian lain tetap tersimpan.
	call(t, "PUT", jalur, f.token, map[string]any{
		"provider": "whatsapp", "fields": map[string]any{"access_token": "token-benar"},
	}).mustOK(t, "ganti token")
	uji = call(t, "POST", jalur+"/test", f.token, nil).mustOK(t, "tes berhasil").data(t)
	if uji["status"] != "connected" || uji["info"] != "Warung Uji · +62 812-0000-1111" {
		t.Fatalf("tes token benar: %v", uji)
	}

	// Verifikasi alamat oleh Meta (GET hub.challenge).
	tantang := func(vt string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", "/webhooks/channels/whatsapp/"+token+
			"?hub.mode=subscribe&hub.verify_token="+vt+"&hub.challenge=tantangan123", nil)
		req.RemoteAddr = ipPenyedia
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	if rec := tantang(verify); rec.Code != 200 || rec.Body.String() != "tantangan123" {
		t.Fatalf("challenge benar: %d %q", rec.Code, rec.Body.String())
	}
	if rec := tantang("salah"); rec.Code != 403 {
		t.Fatalf("challenge salah harus 403, dapat %d", rec.Code)
	}

	pesanan := `{"object":"whatsapp_business_account","entry":[{"id":"WABA","changes":[{"field":"messages","value":{
		"messaging_product":"whatsapp","metadata":{"display_phone_number":"6281200001111","phone_number_id":"10987654321"},
		"contacts":[{"profile":{"name":"Kerry"},"wa_id":"6281399990000"}],
		"messages":[{"from":"6281399990000","id":"wamid.PESANAN1","timestamp":"1790500000","type":"order",
		"order":{"catalog_id":"CAT1","text":"tanpa gula","product_items":[
			{"product_retailer_id":"KOPI-A","quantity":2,"item_price":18000,"currency":"IDR"}]}}]}}]}]}`
	kirim := func(body, secret string) apiResp {
		m := hmac.New(sha256.New, []byte(secret))
		m.Write([]byte(body))
		req := httptest.NewRequest("POST", "/webhooks/channels/whatsapp/"+token, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Hub-Signature-256", "sha256="+hex.EncodeToString(m.Sum(nil)))
		req.RemoteAddr = ipPenyedia
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		out := apiResp{Code: rec.Code, Raw: rec.Body.String()}
		_ = json.Unmarshal(rec.Body.Bytes(), &out.Body)
		return out
	}

	kirim(pesanan, "bukan-rahasianya").mustCode(t, "tanda tangan palsu", 401)
	r1 := kirim(pesanan, "rahasia-app").mustOK(t, "webhook pesanan").data(t)
	assertI64(t, r1, "received", 1)
	r2 := kirim(pesanan, "rahasia-app").mustOK(t, "webhook ulang").data(t)
	assertI64(t, r2, "duplicate", 1)
	status := `{"object":"whatsapp_business_account","entry":[{"changes":[{"field":"messages","value":{
		"metadata":{"phone_number_id":"10987654321"},"statuses":[{"id":"wamid.X","status":"read"}]}}]}]}`
	if r := kirim(status, "rahasia-app").mustOK(t, "status kiriman").data(t); r["ignored"] != "bukan pesanan" {
		t.Fatalf("status kiriman harus diabaikan: %v", r)
	}

	// Jalur lama tanpa tanda tangan menolak kanal yang sudah tersambung API.
	susup, _ := json.Marshal(map[string]any{
		"event_type": "order.created", "external_order_id": "SUSUP-1",
		"items": []map[string]any{{"product_id": f.prodA, "qty": "9"}},
	})
	reqLama := httptest.NewRequest("POST", "/webhooks/channels/whatsapp?merchant_ref=10987654321", strings.NewReader(string(susup)))
	reqLama.Header.Set("Content-Type", "application/json")
	reqLama.RemoteAddr = ipPenyedia
	recLama := httptest.NewRecorder()
	router.ServeHTTP(recLama, reqLama)
	resLama := apiResp{Code: recLama.Code, Raw: recLama.Body.String()}
	_ = json.Unmarshal(recLama.Body.Bytes(), &resLama.Body)
	lama := resLama.mustOK(t, "jalur lama").data(t)
	if lama["accepted"] != false {
		t.Fatalf("jalur lama harus menolak kanal ber-API: %v", lama)
	}

	// Pekerja memproses → penjualan kanal dengan SKU dicocokkan ke barang toko.
	// Pekerjanya global (semua tenant) — yang diperiksa hasil untuk kanal ini.
	processChannelEvents(t, f.token)
	if ev := call(t, "GET", "/api/v1/channels/"+ch+"/events?status=done", f.token, nil).
		mustOK(t, "peristiwa").data(t)["data"].([]any); len(ev) != 1 {
		t.Fatalf("peristiwa selesai kanal ini = %d, mau 1", len(ev))
	}
	daftar := call(t, "GET", "/api/v1/channel-orders?channel_id="+ch, f.token, nil).
		mustOK(t, "pesanan").data(t)["data"].([]any)
	if len(daftar) != 1 {
		t.Fatalf("pesanan kanal = %d, mau 1", len(daftar))
	}
	p := daftar[0].(map[string]any)
	if p["buyer_name"] != "Kerry" || p["buyer_phone"] != "+6281399990000" || p["external_order_id"] != "wamid.PESANAN1" {
		t.Fatalf("pembeli/nomor pesanan: %v", p)
	}
	assertI64(t, p, "gross_amount", 36000)

	// Putus: kredensial & alamat hilang; webhook ke alamat lama ditolak.
	call(t, "DELETE", jalur, f.token, nil).mustOK(t, "putus")
	g := call(t, "GET", jalur, f.token, nil).mustOK(t, "setelah putus").data(t)
	if g["status"] != "none" || g["webhook_url"] != nil {
		t.Fatalf("setelah putus: %v", g)
	}
	kirim(pesanan, "rahasia-app").mustCode(t, "alamat lama", 401)
}

// ipPenyedia: alamat sendiri untuk permintaan webhook di uji ini — pembatas laju
// webhook per-IP sudah dihabiskan TestChannelWebhookRateLimited.
const ipPenyedia = "203.0.113.42:4431"

func jsonTeks(m map[string]any) string {
	b, _ := json.Marshal(m)
	return string(b)
}
