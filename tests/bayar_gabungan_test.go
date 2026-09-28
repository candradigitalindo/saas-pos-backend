package tests

import (
	"testing"

	"candra/backend-api/internal/ulid"
)

// Bayar gabungan: satu penjualan dengan beberapa baris pembayaran.
//
// Yang dijaga: kembalian dihitung dari seluruh yang dibayar tapi hanya boleh
// sebesar uang TUNAI (non-tunai yang melebihi sisa ditolak — kembalian dari
// QRIS tidak pernah ada di laci); tunai + kasbon harus pas; laci shift dan
// ringkasan per cara bayar menghitung tunai bersih (diterima − kembalian).
func TestBayarGabungan(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "gabung")
	pelanggan := makeCustomer(t, f.token, "Bu Sari Gabung")
	jual := func(payload map[string]any) apiResp {
		t.Helper()
		payload["outlet_id"] = f.outletID
		return checkout(t, f.token, ulid.New(), payload)
	}
	duaA := []map[string]any{{"product_id": f.prodA, "qty": "2"}} // 30.000

	// QRIS 20.000 + tunai 20.000 untuk 30.000 → kembalian 10.000 (dari tunai).
	s := jual(map[string]any{
		"items": duaA,
		"payments": []map[string]any{
			{"method": "qris", "amount": 20000}, {"method": "cash", "amount": 20000},
		},
	}).mustCode(t, "qris + tunai", 201).data(t)
	assertI64(t, s, "paid_amount", 40000)
	assertI64(t, s, "change_amount", 10000)
	if n := len(s["payments"].([]any)); n != 2 {
		t.Fatalf("baris pembayaran = %d, mau 2", n)
	}

	// QRIS melebihi total: "kembalian" 5.000 bukan dari tunai → ditolak.
	jual(map[string]any{
		"items":    duaA,
		"payments": []map[string]any{{"method": "qris", "amount": 35000}},
	}).mustCode(t, "qris berlebih", 422)
	// Tunai 2.000 tidak cukup menutup kembalian 7.000 dari QRIS berlebih.
	jual(map[string]any{
		"items": duaA,
		"payments": []map[string]any{
			{"method": "qris", "amount": 35000}, {"method": "cash", "amount": 2000},
		},
	}).mustCode(t, "kembalian melebihi tunai", 422)

	// Tunai sebagian + kasbon pelunas sisa (harus pas).
	k := jual(map[string]any{
		"customer_id": pelanggan,
		"items":       duaA,
		"payments": []map[string]any{
			{"method": "cash", "amount": 10000}, {"method": "credit", "amount": 20000},
		},
	}).mustCode(t, "tunai + kasbon", 201).data(t)
	assertI64(t, k, "change_amount", 0)
	jual(map[string]any{
		"customer_id": pelanggan,
		"items":       duaA,
		"payments": []map[string]any{
			{"method": "cash", "amount": 15000}, {"method": "credit", "amount": 20000},
		},
	}).mustCode(t, "tunai + kasbon berlebih", 422)

	// Laci: modal 100.000 + tunai bersih (20.000 − 10.000) + 10.000.
	d := call(t, "GET", "/api/v1/shifts/"+f.shiftID, f.token, nil).mustOK(t, "rincian shift").data(t)
	assertI64(t, d, "expected_cash", 120000)
	perCara := map[string]int64{}
	for _, c := range d["sales"].(map[string]any)["by_method"].([]any) {
		m := c.(map[string]any)
		perCara[m["method"].(string)] = int64(m["amount"].(float64))
	}
	if perCara["cash"] != 20000 || perCara["qris"] != 20000 || perCara["credit"] != 20000 {
		t.Fatalf("per cara bayar = %v", perCara)
	}
}
