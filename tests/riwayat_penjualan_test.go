package tests

import (
	"testing"

	"candra/backend-api/internal/ulid"
)

// Riwayat penjualan: daftar memuat isi belanja, cara bayar, pelanggan, dan
// kasir (dulu hanya nomor nota, jam, total); bisa dicari per nomor nota dan
// disaring per cara bayar; ringkasan harian memisahkan penjualan, retur, dan
// batal, dengan uang tunai sudah dikurangi kembalian.
func TestRiwayatPenjualanLengkapDanRingkasan(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "riwayat")
	pelanggan := makeCustomer(t, f.token, "Bu Rina Riwayat")

	jual := func(nama string, payload map[string]any) map[string]any {
		t.Helper()
		payload["outlet_id"] = f.outletID
		return checkout(t, f.token, ulid.New(), payload).mustCode(t, nama, 201).data(t)
	}
	// Tunai: 2×A + 1×B = 38.000, dibayar 50.000 → kembalian 12.000.
	tunai := jual("tunai", map[string]any{
		"items": []map[string]any{
			{"product_id": f.prodA, "qty": "2"}, {"product_id": f.prodB, "qty": "1"},
		},
		"payments": []map[string]any{{"method": "cash", "amount": 50000}},
	})
	jual("qris", map[string]any{
		"items":    []map[string]any{{"product_id": f.prodA, "qty": "1"}},
		"payments": []map[string]any{{"method": "qris", "amount": 15000}},
	})
	jual("kasbon", map[string]any{
		"customer_id": pelanggan,
		"items":       []map[string]any{{"product_id": f.prodB, "qty": "1"}},
		"payments":    []map[string]any{{"method": "credit", "amount": 8000}},
	})
	batal := jual("akan dibatalkan", map[string]any{
		"items":    []map[string]any{{"product_id": f.prodB, "qty": "1"}},
		"payments": []map[string]any{{"method": "cash", "amount": 8000}},
	})
	retur := jual("akan diretur", map[string]any{
		"items":    []map[string]any{{"product_id": f.prodA, "qty": "1"}},
		"payments": []map[string]any{{"method": "cash", "amount": 15000}},
	})
	call(t, "POST", "/api/v1/sales/"+batal["id"].(string)+"/void", f.token, map[string]any{"reason": "salah input"}).
		mustOK(t, "batalkan")
	call(t, "POST", "/api/v1/sales/"+retur["id"].(string)+"/refund", f.token, map[string]any{"reason": "rusak"}).
		mustOK(t, "retur")
	hari := tunai["business_date"].(string)

	// Daftar: isi belanja, cara bayar, pelanggan, kasir — tanpa membuka satu per satu.
	daftar := call(t, "GET", "/api/v1/sales?outlet_id="+f.outletID+"&business_date="+hari+"&limit=50", f.token, nil).
		mustOK(t, "daftar").data(t)["data"].([]any)
	var baris map[string]any
	for _, d := range daftar {
		if m := d.(map[string]any); m["id"] == tunai["id"] {
			baris = m
		}
	}
	if baris == nil {
		t.Fatal("transaksi tunai tidak ada di daftar")
	}
	if items, _ := baris["items"].([]any); len(items) != 2 {
		t.Fatalf("items di daftar = %v, mau 2 baris", baris["items"])
	}
	if pays, _ := baris["payments"].([]any); len(pays) != 1 || pays[0].(map[string]any)["method"] != "cash" {
		t.Fatalf("payments di daftar = %v", baris["payments"])
	}
	if baris["cashier_name"] == "" || baris["cashier_name"] == nil {
		t.Fatalf("nama kasir kosong: %v", baris)
	}
	var kasbon map[string]any
	for _, d := range daftar {
		m := d.(map[string]any)
		if pays, _ := m["payments"].([]any); len(pays) == 1 && pays[0].(map[string]any)["method"] == "credit" {
			kasbon = m
		}
	}
	if kasbon == nil || kasbon["customer_name"] != "Bu Rina Riwayat" {
		t.Fatalf("kasbon tanpa nama pelanggan: %v", kasbon)
	}

	// Retur ↔ penjualan asal saling menyebut nomor notanya.
	var asal, barisRetur map[string]any
	for _, d := range daftar {
		m := d.(map[string]any)
		switch {
		case m["id"] == retur["id"]:
			asal = m
		case m["status"] == "returned":
			barisRetur = m
		}
	}
	if asal == nil || barisRetur == nil {
		t.Fatalf("penjualan asal / baris retur tidak ada di daftar")
	}
	if barisRetur["return_of_receipt_no"] != retur["receipt_no"] {
		t.Fatalf("baris retur menyebut nota %v, mau %v", barisRetur["return_of_receipt_no"], retur["receipt_no"])
	}
	if asal["returned_by_receipt_no"] != barisRetur["receipt_no"] {
		t.Fatalf("penjualan asal menyebut retur %v, mau %v", asal["returned_by_receipt_no"], barisRetur["receipt_no"])
	}
	if _, ada := baris["returned_by_receipt_no"]; ada {
		t.Fatalf("penjualan yang tidak diretur ikut ditandai: %v", baris)
	}

	// Saring per cara bayar & cari per nomor nota.
	qris := call(t, "GET", "/api/v1/sales?outlet_id="+f.outletID+"&business_date="+hari+"&method=qris", f.token, nil).
		mustOK(t, "saring qris").data(t)["data"].([]any)
	if len(qris) != 1 {
		t.Fatalf("saring QRIS: %d transaksi, mau 1", len(qris))
	}
	nota := tunai["receipt_no"].(string)
	cari := call(t, "GET", "/api/v1/sales?outlet_id="+f.outletID+"&business_date="+hari+"&search="+nota[len(nota)-4:], f.token, nil).
		mustOK(t, "cari nota").data(t)["data"].([]any)
	if len(cari) == 0 || cari[0].(map[string]any)["receipt_no"] != nota {
		t.Fatalf("cari nota %s: %v", nota, cari)
	}

	// Ringkasan: selesai = tunai 38.000 + QRIS 15.000 + kasbon 8.000 + yang
	// diretur 15.000 (tetap selesai; returnya baris sendiri) = 76.000 dari 4.
	r := call(t, "GET", "/api/v1/sales/day-summary?outlet_id="+f.outletID+"&business_date="+hari, f.token, nil).
		mustOK(t, "ringkasan").data(t)
	assertI64(t, r, "sales_count", 4)
	assertI64(t, r, "sales_total", 76000)
	assertI64(t, r, "average_sale", 19000)
	assertI64(t, r, "returns_count", 1)
	assertI64(t, r, "returns_total", 15000)
	assertI64(t, r, "canceled_count", 1)
	assertI64(t, r, "canceled_total", 8000)
	assertI64(t, r, "net_total", 61000)
	perCara := map[string]int64{}
	for _, c := range r["by_method"].([]any) {
		m := c.(map[string]any)
		perCara[m["method"].(string)] = int64(m["amount"].(float64))
	}
	// Tunai: 50.000 + 15.000 − kembalian 12.000 = 53.000 (yang dibatalkan tidak ikut).
	if perCara["cash"] != 53000 || perCara["qris"] != 15000 || perCara["credit"] != 8000 {
		t.Fatalf("per cara bayar = %v", perCara)
	}

	call(t, "GET", "/api/v1/sales/day-summary?business_date=kemarin", f.token, nil).mustCode(t, "tanggal salah", 422)
}
