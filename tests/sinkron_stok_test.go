package tests

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"candra/backend-api/internal/ulid"
)

// Sinkron stok ke marketplace (Tokopedia & Shop tiruan).
//
// Yang dijaga: "Cocokkan barang" memetakan listing ber-Seller SKU yang sama
// dengan SKU barang toko (yang tanpa pasangan dilaporkan, bukan dipaksa);
// kiriman pertama membawa stok toko; penjualan di KASIR (bukan pesanan kanal)
// ikut terkirim; penyangga stok & "tidak tersedia" dihormati; SKU yang tersebar
// di dua gudang gagal dengan alasan jelas setelah beberapa putaran, dan tidak
// diantre ulang terus-menerus selama stoknya tidak bergerak.
func TestSinkronStokKeMarketplace(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "sinkron-stok")
	call(t, "PUT", "/api/v1/products/"+f.prodA, f.token, map[string]any{"sku": "SS-A"}).mustOK(t, "sku A")
	call(t, "PUT", "/api/v1/products/"+f.prodB, f.token, map[string]any{"sku": "SS-B"}).mustOK(t, "sku B")
	ch := makeChannel(t, f, "Tokopedia", "0.05")

	const kunciApp, rahasiaApp = "app-stok", "rahasia-stok"
	var mu sync.Mutex
	kiriman := []string{} // "sku_id=qty@gudang"
	platform := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		balas := func(data string) {
			_, _ = w.Write([]byte(`{"code":0,"message":"Success","request_id":"r","data":` + data + `}`))
		}
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		defer mu.Unlock()
		switch {
		case r.URL.Path == "/api/v2/token/get" || r.URL.Path == "/api/v2/token/refresh":
			balas(`{"access_token":"akses","access_token_expire_in":` + strconv.FormatInt(time.Now().Unix()+86400*7, 10) +
				`,"refresh_token":"segar","seller_name":"Toko Stok"}`)
		case r.URL.Path == "/authorization/202309/shops":
			balas(`{"shops":[{"id":"55","name":"Toko Stok","region":"ID","cipher":"ROW_stok","code":"IDSTOK"}]}`)
		case r.URL.Path == "/product/202502/products/search":
			if r.URL.Query().Get("shop_cipher") != "ROW_stok" || !strings.Contains(string(body), `"status":"ALL"`) {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			balas(`{"total_count":4,"products":[
				{"id":"P1","title":"Kopi","status":"ACTIVATE","skus":[{"id":"S1","seller_sku":"SS-A","inventory":[{"warehouse_id":"W1","quantity":5}]}]},
				{"id":"P2","title":"Teh","status":"ACTIVATE","skus":[{"id":"S2","seller_sku":"SS-B","inventory":[{"warehouse_id":"W1","quantity":1},{"warehouse_id":"W2","quantity":1}]}]},
				{"id":"P3","title":"Lain","status":"ACTIVATE","skus":[{"id":"S3","seller_sku":"TIDAK-ADA","inventory":[{"warehouse_id":"W1","quantity":1}]}]},
				{"id":"P4","title":"Tanpa SKU","status":"ACTIVATE","skus":[{"id":"S4","seller_sku":"","inventory":[]}]}]}`)
		case strings.HasSuffix(r.URL.Path, "/inventory/update"):
			var b struct {
				Skus []struct {
					ID        string `json:"id"`
					Inventory []struct {
						WarehouseID string `json:"warehouse_id"`
						Quantity    int64  `json:"quantity"`
					} `json:"inventory"`
				} `json:"skus"`
			}
			_ = json.Unmarshal(body, &b)
			for _, s := range b.Skus {
				for _, inv := range s.Inventory {
					kiriman = append(kiriman, s.ID+"="+strconv.FormatInt(inv.Quantity, 10)+"@"+inv.WarehouseID)
				}
			}
			balas(`{}`)
		default:
			balas(`{}`) // pendaftaran webhook
		}
	}))
	defer platform.Close()
	t.Setenv("CHANNEL_TOKOPEDIA_API_URL", platform.URL)
	t.Setenv("CHANNEL_TOKOPEDIA_TOKEN_URL", platform.URL)
	t.Setenv("CHANNEL_TOKOPEDIA_AUTH_URL", "https://services.contoh.id/open/authorize")
	t.Setenv("APP_URL", "https://pos.contoh.id")

	jalur := "/api/v1/channels/" + ch + "/connection"
	webhook := call(t, "PUT", jalur, f.token, map[string]any{"provider": "tokopedia", "fields": map[string]any{
		"app_key": kunciApp, "app_secret": rahasiaApp, "service_id": "9",
	}}).mustOK(t, "simpan").data(t)["webhook_url"].(string)
	alamat, _ := url.Parse(call(t, "POST", jalur+"/authorize", f.token, nil).mustOK(t, "otorisasi").data(t)["url"].(string))
	req := httptest.NewRequest("GET", webhook[strings.Index(webhook, "/webhooks/"):]+"/oauth/callback?code=k&state="+alamat.Query().Get("state"), nil)
	req.RemoteAddr = "203.0.113.56:4431" // IP sendiri: batas laju webhook per IP
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("callback: %d %s", rec.Code, rec.Body.String())
	}

	// Sebelum dicocokkan: tidak ada yang dikirim.
	processChannelEvents(t, f.token)
	if len(kiriman) != 0 {
		t.Fatalf("stok terkirim sebelum barang dicocokkan: %v", kiriman)
	}
	m := call(t, "POST", "/api/v1/channels/"+ch+"/products/match", f.token, nil).mustOK(t, "cocokkan").data(t)
	assertI64(t, m, "listings", 3)
	assertI64(t, m, "matched", 2)
	assertI64(t, m, "created", 2)
	assertI64(t, m, "without_sku", 1)
	if u, _ := m["unmatched"].([]any); len(u) != 1 || u[0] != "TIDAK-ADA" {
		t.Fatalf("tanpa pasangan: %v", m["unmatched"])
	}

	ambil := func() []string {
		mu.Lock()
		defer mu.Unlock()
		k := kiriman
		kiriman = nil
		return k
	}
	processChannelEvents(t, f.token)
	if k := ambil(); len(k) != 1 || k[0] != "S1=100@W1" {
		t.Fatalf("kiriman pertama = %v, mau [S1=100@W1] (SS-B di dua gudang tidak dikirim)", k)
	}

	// Penjualan di KASIR memotong stok → ikut terkirim ke marketplace.
	checkout(t, f.token, ulid.New(), map[string]any{
		"outlet_id": f.outletID,
		"items":     []map[string]any{{"product_id": f.prodA, "qty": "3"}},
		"payments":  []map[string]any{{"method": "cash", "amount": 45000}},
	}).mustCode(t, "checkout", 201)
	processChannelEvents(t, f.token)
	if k := ambil(); len(k) != 1 || k[0] != "S1=97@W1" {
		t.Fatalf("setelah penjualan kasir = %v, mau [S1=97@W1]", k)
	}

	// Penyangga 10 → 87; tidak tersedia → 0.
	call(t, "POST", "/api/v1/channels/"+ch+"/products", f.token, map[string]any{
		"product_id": f.prodA, "external_sku": "SS-A", "stock_buffer": "10",
	}).mustOK(t, "penyangga")
	processChannelEvents(t, f.token)
	if k := ambil(); len(k) != 1 || k[0] != "S1=87@W1" {
		t.Fatalf("dengan penyangga = %v, mau [S1=87@W1]", k)
	}
	call(t, "POST", "/api/v1/channels/"+ch+"/products", f.token, map[string]any{
		"product_id": f.prodA, "external_sku": "SS-A", "stock_buffer": "10", "is_available": false,
	}).mustOK(t, "tidak tersedia")
	processChannelEvents(t, f.token)
	if k := ambil(); len(k) != 1 || k[0] != "S1=0@W1" {
		t.Fatalf("tidak tersedia = %v, mau [S1=0@W1]", k)
	}

	// SS-B (dua gudang) gagal setelah beberapa putaran, dengan alasannya.
	for i := 0; i < 5; i++ {
		processChannelEvents(t, f.token)
	}
	st := call(t, "GET", "/api/v1/channels/"+ch+"/stock-status", f.token, nil).mustOK(t, "status").data(t)
	if st["supported"] != true || st["last_error_sku"] != "SS-B" || !strings.Contains(st["last_error"].(string), "gudang") {
		t.Fatalf("status stok: %v", st)
	}
	assertI64(t, st, "mapped", 2)
	assertI64(t, st, "linked", 2)
	assertI64(t, st, "failed", 1)
	assertI64(t, st, "pending", 0)
	antre := func() int {
		return len(call(t, "GET", "/api/v1/channels/"+ch+"/stock-syncs", f.token, nil).mustOK(t, "antrean").Body["data"].([]any))
	}
	sebelum := antre()
	processChannelEvents(t, f.token)
	processChannelEvents(t, f.token)
	if sesudah := antre(); sesudah != sebelum {
		t.Fatalf("antrean bertambah tanpa gerakan stok: %d → %d", sebelum, sesudah)
	}
}
