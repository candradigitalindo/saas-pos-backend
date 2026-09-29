package tests

import (
	"strings"
	"testing"

	"candra/backend-api/internal/ulid"

	"github.com/shopspring/decimal"
)

// Varian barang = pilihan dengan selisih harga.
//
// Yang dijaga: varian dikelola per barang (nama unik, harga tidak boleh minus,
// SKU/barcode tidak bentrok dengan barang atau varian lain); penjualan varian
// berharga barang + selisih dan struknya menyebut variannya; stok dihitung di
// tingkat BARANG; varian yang dihapus lunak tetap terbaca di penjualan lama;
// pesanan kanal ber-SKU varian terpetakan ke variannya.
func TestVarianBarang(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "varian")
	jalur := "/api/v1/products/" + f.prodA + "/variants"

	besar := call(t, "POST", jalur, f.token, map[string]any{"name": "Besar", "price_delta": 5000, "sku": "KOPI-L"}).
		mustCode(t, "varian besar", 201).data(t)
	if besar["price"] != float64(20000) || besar["is_active"] != true {
		t.Fatalf("varian besar: %v", besar)
	}
	kecil := call(t, "POST", jalur, f.token, map[string]any{"name": "Kecil", "price_delta": -3000}).
		mustCode(t, "varian kecil", 201).data(t)
	call(t, "POST", jalur, f.token, map[string]any{"name": " besar ", "price_delta": 0}).mustCode(t, "nama ganda", 409)
	call(t, "POST", jalur, f.token, map[string]any{"name": "Gratis", "price_delta": -20000}).mustCode(t, "harga minus", 422)
	call(t, "POST", jalur, f.token, map[string]any{"name": "Sedang", "sku": "KOPI-L"}).mustCode(t, "SKU dipakai varian", 409)
	call(t, "PUT", "/api/v1/products/"+f.prodB, f.token, map[string]any{"sku": "TEH-1"}).mustOK(t, "sku B")
	call(t, "POST", jalur, f.token, map[string]any{"name": "Sedang", "sku": "TEH-1"}).mustCode(t, "SKU dipakai barang", 409)
	call(t, "PUT", "/api/v1/products/"+f.prodB, f.token, map[string]any{"barcode": "KOPI-L"}).mustCode(t, "barang pakai kode varian", 409)

	// Ubah (ganti utuh) & daftar.
	call(t, "PUT", jalur+"/"+kecil["id"].(string), f.token, map[string]any{"name": "Kecil", "price_delta": -2000, "is_active": true}).
		mustOK(t, "ubah kecil")
	daftar := call(t, "GET", jalur, f.token, nil).mustOK(t, "daftar").Body["data"].([]any)
	if len(daftar) != 2 || daftar[1].(map[string]any)["price"] != float64(13000) {
		t.Fatalf("daftar varian: %v", daftar)
	}

	// Jual 2 × Besar: harga 20.000, nama "… (Besar)", stok barang berkurang 2.
	stokAwal := stockQty(t, f.tenantFixture, f.outletID, f.prodA)
	jual := checkout(t, f.token, ulid.New(), map[string]any{
		"outlet_id": f.outletID,
		"items":     []map[string]any{{"product_id": f.prodA, "variant_id": besar["id"], "qty": "2"}},
		"payments":  []map[string]any{{"method": "cash", "amount": 40000}},
	}).mustCode(t, "jual varian", 201).data(t)
	it := jual["items"].([]any)[0].(map[string]any)
	if it["unit_price"] != float64(20000) || !strings.HasSuffix(it["product_name"].(string), "(Besar)") || it["variant_id"] != besar["id"] {
		t.Fatalf("baris penjualan varian: %v", it)
	}
	if got := stockQty(t, f.tenantFixture, f.outletID, f.prodA); !sama(got, kurang(stokAwal, "2")) {
		t.Fatalf("stok barang %s → %s, mau berkurang 2 (stok tingkat barang)", stokAwal, got)
	}

	// Hapus lunak: tidak ada di daftar, penjualan lama utuh.
	call(t, "DELETE", jalur+"/"+besar["id"].(string), f.token, nil).mustOK(t, "hapus")
	if d := call(t, "GET", jalur, f.token, nil).mustOK(t, "daftar").Body["data"].([]any); len(d) != 1 {
		t.Fatalf("setelah hapus: %v", d)
	}
	call(t, "GET", "/api/v1/sales/"+jual["id"].(string), f.token, nil).mustOK(t, "penjualan lama")

	// Pesanan kanal ber-SKU varian → variannya.
	call(t, "POST", jalur, f.token, map[string]any{"name": "Jumbo", "price_delta": 9000, "sku": "KOPI-XL"}).mustCode(t, "jumbo", 201)
	ch := makeChannelWithRef(t, f, "shopee", "0", "MERCH-varian")
	sendChannelWebhook(t, "shopee", "MERCH-varian", map[string]any{
		"event_type": "order.created", "external_order_id": "VR-1",
		"items": []map[string]any{{"sku": "KOPI-XL", "qty": "1"}},
	}).mustCode(t, "webhook", 200)
	processChannelEvents(t, f.token)
	for _, x := range call(t, "GET", "/api/v1/channel-orders?channel_id="+ch, f.token, nil).mustOK(t, "pesanan").data(t)["data"].([]any) {
		p := x.(map[string]any)
		assertI64(t, p, "gross_amount", 24000)
		if n := p["items"].([]any)[0].(map[string]any)["product_name"].(string); !strings.HasSuffix(n, "(Jumbo)") {
			t.Fatalf("nama barang pesanan kanal: %q", n)
		}
	}

	// Pesanan kanal yang dicatat MANUAL dengan variant_id (layar Kanal).
	kecilID := kecil["id"].(string)
	manual := call(t, "POST", "/api/v1/channel-orders", f.token, map[string]any{
		"channel_id": ch, "external_order_id": "MAN-VAR-1",
		"items": []map[string]any{{"product_id": f.prodA, "variant_id": kecilID, "qty": "2"}},
	}).mustCode(t, "pesanan manual bervarian", 201).data(t)
	assertI64(t, manual, "gross_amount", 26000) // 2 × (15.000 − 2.000)
	jualManual := call(t, "GET", "/api/v1/sales/"+manual["sale_id"].(string), f.token, nil).mustOK(t, "penjualan manual").data(t)
	if n := jualManual["items"].([]any)[0].(map[string]any)["product_name"].(string); !strings.HasSuffix(n, "(Kecil)") {
		t.Fatalf("nama barang pesanan manual: %q", n)
	}
}

func kurang(a, b string) string {
	x, _ := decimal.NewFromString(a)
	y, _ := decimal.NewFromString(b)
	return x.Sub(y).String()
}

func sama(a, b string) bool {
	x, _ := decimal.NewFromString(a)
	y, _ := decimal.NewFromString(b)
	return x.Equal(y)
}
