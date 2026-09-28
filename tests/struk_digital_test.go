package tests

import (
	"encoding/json"
	"strings"
	"testing"

	"candra/backend-api/internal/ulid"
)

// Struk digital: tautan publik ke satu struk.
//
// Yang dijaga: token acak dibuat sekali lalu dipakai ulang; halaman publik
// terbuka TANPA akun tapi hanya berisi isi struk (tanpa modal, pelanggan,
// kasir, id internal); token asal/tenant lain → tidak ditemukan; struk yang
// dibatalkan tetap terbuka dengan statusnya.
func TestStrukDigital(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "struk-digital")
	pelanggan := makeCustomer(t, f.token, "Bu Ani Rahasia")
	jual := checkout(t, f.token, ulid.New(), map[string]any{
		"outlet_id": f.outletID, "customer_id": pelanggan,
		"items":    []map[string]any{{"product_id": f.prodA, "qty": "2", "note": "tanpa gula"}},
		"payments": []map[string]any{{"method": "cash", "amount": 50000}},
	}).mustCode(t, "jual", 201).data(t)
	id := jual["id"].(string)

	token := call(t, "POST", "/api/v1/sales/"+id+"/receipt-link", f.token, nil).mustOK(t, "tautan").data(t)["token"].(string)
	if len(token) != 22 {
		t.Fatalf("token = %q, mau 22 karakter", token)
	}
	if lagi := call(t, "POST", "/api/v1/sales/"+id+"/receipt-link", f.token, nil).mustOK(t, "tautan lagi").data(t)["token"]; lagi != token {
		t.Fatalf("token berubah: %v → %v", token, lagi)
	}

	// Terbuka tanpa akun; hanya isi struk.
	r := call(t, "GET", "/api/v1/public/receipts/"+token, "", nil).mustOK(t, "struk publik")
	d := r.data(t)
	if d["receipt_no"] != jual["receipt_no"] || d["total"] != float64(30000) || d["change"] != float64(20000) || d["store_name"] == "" {
		t.Fatalf("struk publik: %v", d)
	}
	it := d["items"].([]any)[0].(map[string]any)
	if it["note"] != "tanpa gula" || it["line_total"] != float64(30000) {
		t.Fatalf("baris struk publik: %v", it)
	}
	mentah, _ := json.Marshal(d)
	for _, bocor := range []string{"cost", "customer", "Bu Ani", id, f.outletID, "created_by", "product_id"} {
		if strings.Contains(string(mentah), bocor) {
			t.Fatalf("struk publik membocorkan %q: %s", bocor, mentah)
		}
	}

	call(t, "GET", "/api/v1/public/receipts/"+strings.Repeat("A", 22), "", nil).mustCode(t, "token asal", 404)
	call(t, "GET", "/api/v1/public/receipts/pendek", "", nil).mustCode(t, "token pendek", 404)

	// Tenant lain tidak bisa membuat tautan untuk penjualan kita.
	g := setupPOS(t, "struk-digital-lain")
	call(t, "POST", "/api/v1/sales/"+id+"/receipt-link", g.token, nil).mustCode(t, "tenant lain", 404)

	// Dibatalkan: tautan tetap terbuka, statusnya ikut.
	call(t, "POST", "/api/v1/sales/"+id+"/void", f.token, map[string]any{"reason": "salah input"}).mustOK(t, "batalkan")
	if s := call(t, "GET", "/api/v1/public/receipts/"+token, "", nil).mustOK(t, "struk batal").data(t)["status"]; s != "canceled" {
		t.Fatalf("status struk batal = %v", s)
	}
}
