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

	"candra/backend-api/internal/ulid"
)

// Menu aplikasi antar dari POS.
//
// Yang dijaga: isi menu dipilih di POS (ID item = SKU, "P-<id>" bila SKU
// kosong); mengirim menu wajib dikonfirmasi karena MENGGANTI menu di aplikasi
// antar; GoFood menerima katalog utuh, Grab diberi tahu lalu mengambil menu
// dari alamat kanal dengan token partner; barang yang stoknya dilacak ikut
// habis/tersedia (Grab: maxStock) sedangkan masakan tanpa pelacakan stok tetap
// tersedia; dan ID item yang sama memetakan pesanan yang masuk.

func stokJadi(t *testing.T, f posFixture, productID, qty string) {
	t.Helper()
	checkoutLike(t, f.token, ulid.New(), "/api/v1/stock-adjustments", map[string]any{
		"outlet_id": f.outletID, "product_id": productID, "new_qty": qty, "reason": "uji menu",
	}).mustCode(t, "penyesuaian stok", 201)
}

func TestMenuGoFoodDariPOS(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "menu-gofood")
	panjang := strings.Repeat("Kopi susu gula aren dengan es. ", 10) // 310 karakter
	if d := call(t, "PUT", "/api/v1/products/"+f.prodA, f.token, map[string]any{"sku": "MN-A", "description": panjang}).
		mustOK(t, "sku A").data(t); d["description"] != strings.TrimSpace(panjang) {
		t.Fatalf("deskripsi barang: %v", d["description"])
	}
	call(t, "PUT", "/api/v1/products/"+f.prodB, f.token, map[string]any{"track_stock": false}).mustOK(t, "B tanpa stok")
	call(t, "POST", "/api/v1/products/"+f.prodA+"/variants", f.token, map[string]any{"name": "Besar", "price_delta": 5000, "sku": "MN-A-L"}).
		mustCode(t, "varian besar", 201)
	vKecil := call(t, "POST", "/api/v1/products/"+f.prodA+"/variants", f.token, map[string]any{"name": "Kecil", "price_delta": -2000}).
		mustCode(t, "varian kecil", 201).data(t)
	ch := makeChannel(t, f, "GoFood", "0.20")

	var mu sync.Mutex
	var katalog map[string]any
	var ketersediaan []string
	gobiz := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		raw, _ := io.ReadAll(r.Body)
		mu.Lock()
		defer mu.Unlock()
		switch {
		case r.URL.Path == "/oauth2/token":
			_ = r.ParseForm()
			tok := "tok-pesanan"
			if strings.Contains(string(raw), "catalog") {
				tok = "tok-katalog"
			}
			_, _ = w.Write([]byte(`{"access_token":"` + tok + `","expires_in":3599}`))
		case r.URL.Path == "/integrations/partner/outlets/G1/v1":
			_, _ = w.Write([]byte(`{"success":true,"data":{"outlet":{"name":"Warung Menu"}}}`))
		case strings.HasSuffix(r.URL.Path, "/notification-subscriptions"):
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"success":true}`))
		case r.Header.Get("Authorization") != "Bearer tok-katalog":
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"errors":[{"message":"scope katalog tidak ada"}]}`))
		case r.Method == "PUT" && r.URL.Path == "/integrations/gofood/outlets/G1/v1/catalog":
			_ = json.Unmarshal(raw, &katalog)
			_, _ = w.Write([]byte(`{"success":true,"data":{}}`))
		case r.Method == "PATCH" && r.URL.Path == "/integrations/gofood/outlets/G1/v2/menu_item_stocks":
			ketersediaan = append(ketersediaan, strings.TrimSpace(string(raw)))
			_, _ = w.Write([]byte(`{"success":true,"data":{}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer gobiz.Close()
	t.Setenv("CHANNEL_GOBIZ_API_URL", gobiz.URL)
	t.Setenv("CHANNEL_GOBIZ_OAUTH_URL", gobiz.URL)
	t.Setenv("APP_URL", "https://pos.contoh.id")

	jalur := "/api/v1/channels/" + ch
	s := call(t, "PUT", jalur+"/connection", f.token, map[string]any{"provider": "gofood", "fields": map[string]any{
		"client_id": "k", "client_secret": "r", "outlet_id": "G1", "notification_secret": "rahasia-notif",
	}}).mustOK(t, "simpan").data(t)
	webhook := s["webhook_url"].(string)
	if u := call(t, "POST", jalur+"/connection/test", f.token, nil).mustOK(t, "tes").data(t); u["status"] != "connected" {
		t.Fatalf("tes: %v", u)
	}

	m := call(t, "GET", jalur+"/menu", f.token, nil).mustOK(t, "menu").data(t)
	if m["supported"] != true {
		t.Fatalf("menu tidak didukung: %v", m)
	}
	call(t, "POST", jalur+"/menu/publish", f.token, map[string]any{"confirm": true}).mustCode(t, "menu kosong", 422)
	m = call(t, "PUT", jalur+"/menu", f.token, map[string]any{"product_ids": []string{f.prodA, f.prodB}}).mustOK(t, "isi menu").data(t)
	diMenu := 0
	for _, x := range m["items"].([]any) {
		if x.(map[string]any)["in_menu"] == true {
			diMenu++
		}
	}
	if diMenu != 2 {
		t.Fatalf("di menu = %d, mau 2: %v", diMenu, m["items"])
	}
	call(t, "POST", jalur+"/menu/publish", f.token, map[string]any{"confirm": false}).mustCode(t, "tanpa konfirmasi", 422)
	p := call(t, "POST", jalur+"/menu/publish", f.token, map[string]any{"confirm": true}).mustOK(t, "kirim menu").data(t)
	assertI64(t, p, "items", 2)

	item := map[string]map[string]any{}
	for _, mn := range katalog["menus"].([]any) {
		for _, it := range mn.(map[string]any)["menu_items"].([]any) {
			x := it.(map[string]any)
			item[x["external_id"].(string)] = x
		}
	}
	idB := "P-" + f.prodB
	// Harga dasar = varian termurah (15.000 − 2.000); pilihan menambah dari situ.
	if len(item) != 2 || item["MN-A"]["in_stock"] != true || item[idB] == nil || item["MN-A"]["price"] != float64(13000) {
		t.Fatalf("katalog GoFood: %v", katalog)
	}
	if d, _ := item["MN-A"]["description"].(string); len([]rune(d)) != 250 || item[idB]["description"] != nil {
		t.Fatalf("deskripsi GoFood (dipotong 250, kosong tidak dikirim): %q / %v", d, item[idB]["description"])
	}
	vc, _ := katalog["variant_categories"].([]any)
	if len(vc) != 1 {
		t.Fatalf("variant_categories: %v", katalog["variant_categories"])
	}
	kat := vc[0].(map[string]any)
	pilihan := map[string]float64{}
	for _, v := range kat["variants"].([]any) {
		pilihan[v.(map[string]any)["external_id"].(string)] = v.(map[string]any)["price"].(float64)
	}
	if kat["external_id"] != "VC-MN-A" || pilihan["MN-A-L"] != 7000 || pilihan["V-"+vKecil["id"].(string)] != 0 ||
		item["MN-A"]["variant_category_external_ids"].([]any)[0] != "VC-MN-A" {
		t.Fatalf("pilihan varian GoFood: %v / %v", kat, item["MN-A"])
	}

	// Sesudah kirim menu tidak ada kiriman stok ulang; stok A habis → habis di
	// GoFood; B tanpa pelacakan stok tidak ikut ditutup.
	processChannelEvents(t, f.token)
	if len(ketersediaan) != 0 {
		t.Fatalf("stok dikirim ulang tepat setelah menu: %v", ketersediaan)
	}
	stokJadi(t, f, f.prodA, "0")
	processChannelEvents(t, f.token)
	if len(ketersediaan) != 1 || ketersediaan[0] != `[{"external_id":"MN-A","in_stock":false}]` {
		t.Fatalf("ketersediaan GoFood: %v", ketersediaan)
	}

	// Pesanan GoFood dengan ID item menu → barang yang benar.
	body := `{"header":{"event_name":"gofood.order.merchant_accepted","event_id":"e-menu","version":1,"timestamp":"2026-09-28T10:15:22+07:00"},
		"body":{"customer":{"name":"Pembeli Menu"},"service_type":"gofood","outlet":{"id":"G1"},
		"order":{"status":"X","order_number":"F-MENU","currency":"IDR","created_at":"2026-09-28T10:14:00+07:00",
		"order_items":[{"quantity":1,"price":8000,"name":"B","external_id":"` + idB + `"}]}}}`
	req := httptest.NewRequest("POST", webhook[strings.Index(webhook, "/webhooks/"):], strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Go-Signature", tandaGoBiz("rahasia-notif", body))
	req.RemoteAddr = "203.0.113.61:4431"
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("webhook: %d %s", rec.Code, rec.Body.String())
	}
	// Pesanan dengan pilihan varian (SKU varian) → "(Besar)".
	bodyVar := `{"header":{"event_name":"gofood.order.merchant_accepted","event_id":"e-menu-v","version":1,"timestamp":"2026-09-28T10:15:22+07:00"},
		"body":{"customer":{"name":"Pembeli Varian"},"service_type":"gofood","outlet":{"id":"G1"},
		"order":{"status":"X","order_number":"F-VAR","currency":"IDR","created_at":"2026-09-28T10:14:00+07:00",
		"order_items":[{"quantity":1,"price":20000,"name":"A","external_id":"MN-A","variants":[{"id":"x","name":"Besar","external_id":"MN-A-L"}]}]}}}`
	req = httptest.NewRequest("POST", webhook[strings.Index(webhook, "/webhooks/"):], strings.NewReader(bodyVar))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Go-Signature", tandaGoBiz("rahasia-notif", bodyVar))
	req.RemoteAddr = "203.0.113.61:4431"
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	processChannelEvents(t, f.token)
	var ketemu bool
	for _, x := range call(t, "GET", "/api/v1/channel-orders?channel_id="+ch, f.token, nil).mustOK(t, "pesanan").data(t)["data"].([]any) {
		o := x.(map[string]any)
		if o["external_order_id"] == "F-MENU" {
			ketemu = true
			assertI64(t, o, "gross_amount", 8000)
		}
		if o["external_order_id"] == "F-VAR" {
			if n := o["items"].([]any)[0].(map[string]any)["product_name"].(string); !strings.HasSuffix(n, "(Besar)") {
				t.Fatalf("pesanan varian GoFood: %q", n)
			}
		}
	}
	if !ketemu {
		t.Fatal("pesanan dengan ID item menu tidak tercatat")
	}
}

func TestMenuGrabDariPOS(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "menu-grab")
	kat := call(t, "POST", "/api/v1/categories", f.token, map[string]any{"name": "Minuman"}).mustCode(t, "kategori", 201).data(t)["id"].(string)
	call(t, "PUT", "/api/v1/products/"+f.prodA, f.token, map[string]any{"sku": "GM-A", "category_id": kat,
		"description": "Es kopi susu gula aren"}).mustOK(t, "sku A")
	call(t, "PUT", "/api/v1/products/"+f.prodB, f.token, map[string]any{"sku": "GM-B", "track_stock": false}).mustOK(t, "B")
	vDingin := call(t, "POST", "/api/v1/products/"+f.prodB+"/variants", f.token, map[string]any{"name": "Dingin", "price_delta": 2000}).
		mustCode(t, "varian dingin", 201).data(t)
	call(t, "POST", "/api/v1/products/"+f.prodB+"/variants", f.token, map[string]any{"name": "Panas", "price_delta": 0}).
		mustCode(t, "varian panas", 201)
	ch := makeChannel(t, f, "GrabFood", "0.25")

	var mu sync.Mutex
	var notif, batch []string
	grab := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		raw, _ := io.ReadAll(r.Body)
		mu.Lock()
		defer mu.Unlock()
		switch {
		case r.URL.Path == "/grabid/v1/oauth2/token":
			_, _ = w.Write([]byte(`{"access_token":"tok-grab","token_type":"Bearer","expires_in":604799}`))
		case r.Header.Get("Authorization") != "Bearer tok-grab":
			w.WriteHeader(http.StatusUnauthorized)
		case strings.HasSuffix(r.URL.Path, "/store/status"):
			_, _ = w.Write([]byte(`{"isOpen":true}`))
		case r.URL.Path == "/partner/v1/merchant/menu/notification":
			notif = append(notif, strings.TrimSpace(string(raw)))
			if len(notif) > 1 {
				w.WriteHeader(http.StatusConflict)
				_, _ = w.Write([]byte(`{"reason":"invalid_argument","message":"sync menu too frequently, retry after 120 seconds"}`))
				return
			}
			w.WriteHeader(http.StatusNoContent)
		case r.Method == "PUT" && r.URL.Path == "/partner/v1/batch/menu":
			batch = append(batch, strings.TrimSpace(string(raw)))
			_, _ = w.Write([]byte(`{"merchantID":"1-GM","status":"SUCCESS"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer grab.Close()
	t.Setenv("CHANNEL_GRAB_API_URL", grab.URL)
	t.Setenv("CHANNEL_GRAB_OAUTH_URL", grab.URL)
	t.Setenv("APP_URL", "https://pos.contoh.id")

	jalur := "/api/v1/channels/" + ch
	call(t, "PUT", jalur+"/connection", f.token, map[string]any{"provider": "grabfood", "fields": map[string]any{
		"client_id": "g", "client_secret": "r", "merchant_id": "1-GM",
	}}).mustOK(t, "simpan")
	u := call(t, "POST", jalur+"/connection/test", f.token, nil).mustOK(t, "tes").data(t)
	nilai := map[string]string{}
	for _, x := range u["webhook_values"].([]any) {
		nilai[x.(map[string]any)["label"].(string)] = x.(map[string]any)["value"].(string)
	}
	webhook := u["webhook_url"].(string)
	lokal := webhook[strings.Index(webhook, "/webhooks/"):]
	panggil := func(method, sub, auth, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, lokal+sub, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		req.RemoteAddr = "203.0.113.62:4431"
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	var tok struct {
		AccessToken string `json:"access_token"`
	}
	_ = json.Unmarshal(panggil("POST", "/oauth/token", "", `{"client_id":"`+nilai["Partner Client ID"]+`","client_secret":"`+
		nilai["Partner Client Secret"]+`","grant_type":"client_credentials"}`).Body.Bytes(), &tok)

	call(t, "PUT", jalur+"/menu", f.token, map[string]any{"product_ids": []string{f.prodA, f.prodB}}).mustOK(t, "isi menu")
	call(t, "POST", jalur+"/menu/publish", f.token, map[string]any{"confirm": true}).mustOK(t, "kirim menu")
	if len(notif) != 1 || notif[0] != `{"merchantID":"1-GM"}` {
		t.Fatalf("notifikasi menu: %v", notif)
	}
	r := call(t, "POST", jalur+"/menu/publish", f.token, map[string]any{"confirm": true})
	if r.Code != 422 || !strings.Contains(r.Raw, "2 menit") {
		t.Fatalf("kirim ulang terlalu cepat: %d %s", r.Code, r.Raw)
	}

	// Grab mengambil menu dari alamat kanal dengan token partner.
	jalurMenu := "/merchant/menu?merchantID=1-GM&partnerMerchantID=P-GM&BusinessType=0"
	if rec := panggil("GET", jalurMenu, "", ""); rec.Code != 401 {
		t.Fatalf("menu tanpa token harus 401, dapat %d", rec.Code)
	}
	rec := panggil("GET", jalurMenu, "Bearer "+tok.AccessToken, "")
	var menu struct {
		MerchantID        string `json:"merchantID"`
		PartnerMerchantID string `json:"partnerMerchantID"`
		Currency          struct {
			Code     string `json:"code"`
			Exponent int    `json:"exponent"`
		} `json:"currency"`
		SellingTimes []map[string]any `json:"sellingTimes"`
		Categories   []struct {
			Name          string           `json:"name"`
			SellingTimeID string           `json:"sellingTimeID"`
			Items         []map[string]any `json:"items"`
		} `json:"categories"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &menu); err != nil || rec.Code != 200 {
		t.Fatalf("menu Grab: %d %s", rec.Code, rec.Body.String())
	}
	item := map[string]map[string]any{}
	kategoriItem := map[string]string{}
	for _, k := range menu.Categories {
		for _, it := range k.Items {
			item[it["id"].(string)] = it
			kategoriItem[it["id"].(string)] = k.Name
		}
	}
	if kategoriItem["GM-A"] != "Minuman" || kategoriItem["GM-B"] != "Lainnya" {
		t.Fatalf("kategori menu: %v", kategoriItem)
	}
	if menu.MerchantID != "1-GM" || menu.PartnerMerchantID != "P-GM" || menu.Currency.Code != "IDR" || menu.Currency.Exponent != 2 ||
		len(menu.SellingTimes) != 1 || menu.Categories[0].SellingTimeID != menu.SellingTimes[0]["id"] {
		t.Fatalf("kerangka menu Grab: %s", rec.Body.String())
	}
	if a := item["GM-A"]; a == nil || a["price"] != float64(1500000) || a["maxStock"] != float64(100) || a["availableStatus"] != "AVAILABLE" ||
		a["description"] != "Es kopi susu gula aren" {
		t.Fatalf("item A (stok dilacak): %v", item["GM-A"])
	}
	if b := item["GM-B"]; b == nil || b["availableStatus"] != "AVAILABLE" || b["maxStock"] != nil || b["price"] != float64(800000) {
		t.Fatalf("item B (tanpa pelacakan stok, tanpa maxStock, harga dasar = varian termurah): %v", item["GM-B"])
	}
	grup := item["GM-B"]["modifierGroups"].([]any)[0].(map[string]any)
	mods := grup["modifiers"].([]any)
	if grup["selectionRangeMin"] != float64(1) || grup["selectionRangeMax"] != float64(1) || len(mods) != 2 ||
		mods[0].(map[string]any)["id"] != "V-"+vDingin["id"].(string) || mods[0].(map[string]any)["price"] != float64(200000) {
		t.Fatalf("modifier Grab: %v", grup)
	}

	stokJadi(t, f, f.prodA, "0")
	processChannelEvents(t, f.token)
	if len(batch) != 1 || !strings.Contains(batch[0], `{"availableStatus":"UNAVAILABLE","id":"GM-A","maxStock":0}`) ||
		!strings.Contains(batch[0], `"field":"ITEM"`) {
		t.Fatalf("stok ke Grab: %v", batch)
	}
}

func tandaGoBiz(rahasia, body string) string {
	m := hmac.New(sha256.New, []byte(rahasia))
	m.Write([]byte(body))
	return hex.EncodeToString(m.Sum(nil))
}
