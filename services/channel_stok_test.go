package services

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func tokenAwet() string { return strconv.FormatInt(time.Now().Add(24*time.Hour).Unix(), 10) }

// Shopee: SKU induk (tanpa variasi → model 0) dan SKU variasi (get_model_list)
// menjadi ref "item:model"; update_stock per item; failure_list per model.
func TestStokShopeeListingDanKiriman(t *testing.T) {
	var mu sync.Mutex
	var kiriman []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		switch r.URL.Path {
		case "/api/v2/product/get_item_list":
			if st := q["item_status"]; len(st) != 2 {
				t.Errorf("item_status = %v, mau NORMAL & UNLIST", st)
			}
			_, _ = w.Write([]byte(`{"error":"","response":{"item":[{"item_id":10},{"item_id":11}],"has_next_page":false}}`))
		case "/api/v2/product/get_item_base_info":
			if q.Get("item_id_list") != "10,11" {
				t.Errorf("item_id_list = %q", q.Get("item_id_list"))
			}
			_, _ = w.Write([]byte(`{"error":"","response":{"item_list":[
				{"item_id":10,"item_name":"Kopi","item_sku":"SH-A","has_model":false},
				{"item_id":11,"item_name":"Kaos","item_sku":"INDUK","has_model":true}]}}`))
		case "/api/v2/product/get_model_list":
			_, _ = w.Write([]byte(`{"error":"","response":{"model":[{"model_id":21,"model_sku":"SH-B-M"},{"model_id":22,"model_sku":"SH-B-L"}]}}`))
		case "/api/v2/product/update_stock":
			b, _ := io.ReadAll(r.Body)
			mu.Lock()
			kiriman = append(kiriman, string(b))
			mu.Unlock()
			if strings.Contains(string(b), `"item_id":11`) {
				_, _ = w.Write([]byte(`{"error":"","response":{"failure_list":[{"model_id":22,"failed_reason":"stock below reserved"}],"success_list":[{"model_id":21,"stock":7}]}}`))
				return
			}
			_, _ = w.Write([]byte(`{"error":"","response":{"failure_list":[],"success_list":[{"model_id":0,"stock":5}]}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	t.Setenv("CHANNEL_SHOPEE_API_URL", srv.URL)
	cred := ChannelCredentials{"partner_id": "1", "partner_key": "k", "shop_id": "9", "access_token": "a",
		"refresh_token": "r", "access_expires": tokenAwet()}

	l, err := (shopeeAdapter{}).ListListings(context.Background(), cred)
	if err != nil {
		t.Fatal(err)
	}
	got := []string{}
	for _, x := range l {
		got = append(got, x.SKU+"="+x.Ref)
	}
	if strings.Join(got, ",") != "SH-A=10:0,SH-B-M=11:21,SH-B-L=11:22" {
		t.Fatalf("listing = %v", got)
	}
	galat := (shopeeAdapter{}).PushStock(context.Background(), cred, []StokKirim{
		{Ref: "10:0", Qty: 5, Lacak: true, Tersedia: true}, {Ref: "11:21", Qty: 7, Lacak: true, Tersedia: true},
		{Ref: "11:22", Qty: 0, Lacak: true}, {Ref: "rusak", Qty: 1, Lacak: true, Tersedia: true},
		{Ref: "10:99", Lacak: false, Tersedia: true}, // tak dilacak & tersedia → tidak dikirim angka
	})
	if len(galat) != 2 || galat["11:22"] == nil || !strings.Contains(galat["11:22"].Error(), "reserved") || galat["rusak"] == nil {
		t.Fatalf("galat = %v", galat)
	}
	if len(kiriman) != 2 || !strings.Contains(kiriman[0], `"item_id":10`) || !strings.Contains(kiriman[1], `"model_id":21`) ||
		!strings.Contains(kiriman[1], `"stock":0`) {
		t.Fatalf("kiriman = %v", kiriman)
	}
}

// Lazada: ref "item:sku:SellerSku" (SellerSku boleh bertitik dua), kiriman
// XML per 20 SKU dengan SellerSku di-escape.
func TestStokLazadaListingDanKiriman(t *testing.T) {
	var mu sync.Mutex
	var payload []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		switch r.URL.Path {
		case "/products/get":
			if q.Get("filter") != "all" {
				t.Errorf("filter = %q", q.Get("filter"))
			}
			_, _ = w.Write([]byte(`{"code":"0","data":{"total_products":"1","products":[{"item_id":180226526,"attributes":{"name":"Kopi"},
				"skus":[{"SellerSku":"39817:01:01","SkuId":314525867},{"SellerSku":"A&B","SkuId":314525868}]}]}}`))
		case "/product/stock/sellable/update":
			mu.Lock()
			payload = append(payload, q.Get("payload"))
			mu.Unlock()
			_, _ = w.Write([]byte(`{"code":"0","data":{},"request_id":"r"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	t.Setenv("CHANNEL_LAZADA_API_URL", srv.URL)
	cred := ChannelCredentials{"app_key": "1", "app_secret": "s", "access_token": "a", "refresh_token": "r", "access_expires": tokenAwet()}

	l, err := (lazadaAdapter{}).ListListings(context.Background(), cred)
	if err != nil {
		t.Fatal(err)
	}
	if len(l) != 2 || l[0].SKU != "39817:01:01" || l[0].Ref != "180226526:314525867:39817:01:01" {
		t.Fatalf("listing = %+v", l)
	}
	stok := []StokKirim{{Ref: l[1].Ref, Qty: 3, Lacak: true, Tersedia: true}}
	for i := 0; i < 24; i++ {
		stok = append(stok, StokKirim{Ref: "1:" + strconv.Itoa(i) + ":S" + strconv.Itoa(i), Qty: int64(i), Lacak: true, Tersedia: i > 0})
	}
	if galat := (lazadaAdapter{}).PushStock(context.Background(), cred, stok); len(galat) != 0 {
		t.Fatalf("galat = %v", galat)
	}
	if len(payload) != 2 || strings.Count(payload[0], "<Sku>") != 20 || strings.Count(payload[1], "<Sku>") != 5 {
		t.Fatalf("pembagian kiriman salah: %d panggilan", len(payload))
	}
	if !strings.Contains(payload[0], "<SellerSku>A&amp;B</SellerSku><SellableQuantity>3</SellableQuantity>") {
		t.Fatalf("SellerSku tidak di-escape: %s", payload[0][:200])
	}
}
