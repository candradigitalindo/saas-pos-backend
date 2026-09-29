package tests

import (
	"testing"

	"candra/backend-api/internal/ulid"
)

// Kemasan barang: jual & beli per "dus isi 10", stok & laporan dalam satuan dasar.
//
// Yang dijaga: kemasan disimpan per barang (id tetap saat diubah), harga
// tersendiri atau isi × harga jual; validasi (satuan dasar, isi > 1, barcode
// bentrok); jual 2 dus → harga & nama satuan dus, stok −20, modal 10×; baris
// campuran; laporan per barang dalam satuan dasar; batal mengembalikan stok;
// beli per dus → stok + isi, harga modal per satuan dasar; ikut tersinkron.
func TestKemasan(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "kemasan") // prodA 15.000 / modal 6.000, stok 100
	dus := makeUnit(t, f.tenantFixture, "dus")
	pak := makeUnit(t, f.tenantFixture, "pak")

	kemasan := func(items []map[string]any) apiResp {
		return call(t, "PUT", "/api/v1/products/"+f.prodA, f.token, map[string]any{"packagings": items})
	}
	d := kemasan([]map[string]any{
		{"unit_id": dus, "conversion": "10", "sell_price": 140000, "barcode": "DUS-A"},
		{"unit_id": pak, "conversion": "4"},
	}).mustOK(t, "kemasan").data(t)
	k := d["packagings"].([]any)
	if len(k) != 2 {
		t.Fatalf("kemasan di balasan: %v", k)
	}
	pakRow, dusRow := k[0].(map[string]any), k[1].(map[string]any) // isi terkecil dulu
	if pakRow["price"] != float64(60000) || dusRow["price"] != float64(140000) || dusRow["unit_name"] != "dus" {
		t.Fatalf("harga kemasan: %v / %v", pakRow, dusRow)
	}
	idDus := dusRow["id"].(string)

	// Validasi.
	kemasan([]map[string]any{{"unit_id": f.unitID, "conversion": "10"}}).mustCode(t, "satuan dasar", 422)
	kemasan([]map[string]any{{"unit_id": dus, "conversion": "1"}}).mustCode(t, "isi 1", 422)
	call(t, "PUT", "/api/v1/products/"+f.prodB, f.token, map[string]any{"sku": "B-SKU"}).mustOK(t, "sku B")
	kemasan([]map[string]any{{"unit_id": dus, "conversion": "10", "barcode": "B-SKU"}}).mustCode(t, "barcode dipakai barang", 409)
	call(t, "PUT", "/api/v1/products/"+f.prodB, f.token, map[string]any{"barcode": "DUS-A"}).mustCode(t, "barang pakai barcode dus", 409)

	// Mengubah kemasan tidak mengganti id-nya.
	d = kemasan([]map[string]any{
		{"unit_id": dus, "conversion": "10", "sell_price": 140000, "barcode": "DUS-A"},
	}).mustOK(t, "kemasan ubah").data(t)
	if k := d["packagings"].([]any); len(k) != 1 || k[0].(map[string]any)["id"] != idDus {
		t.Fatalf("id kemasan berubah / pak tidak terhapus: %v", k)
	}

	// Jual 2 dus + 3 pcs.
	awal := stockQty(t, f.tenantFixture, f.outletID, f.prodA)
	jual := checkout(t, f.token, ulid.New(), map[string]any{
		"outlet_id": f.outletID,
		"items": []map[string]any{
			{"product_id": f.prodA, "product_unit_id": idDus, "qty": "2"},
			{"product_id": f.prodA, "qty": "3"},
		},
		"payments": []map[string]any{{"method": "cash", "amount": 400000}},
	}).mustCode(t, "jual dus", 201).data(t)
	b := jual["items"].([]any)[0].(map[string]any)
	if b["unit_price"] != float64(140000) || b["unit_name"] != "dus" || b["unit_cost"] != float64(60000) ||
		b["product_unit_id"] != idDus || b["unit_conversion"] != "10" {
		t.Fatalf("baris dus: %v", b)
	}
	assertI64(t, jual, "total", 2*140000+3*15000)
	if got := stockQty(t, f.tenantFixture, f.outletID, f.prodA); !sama(got, kurang(awal, "23")) {
		t.Fatalf("stok %s → %s, mau berkurang 23 (2 dus × 10 + 3)", awal, got)
	}

	// Laporan per barang: jumlah dalam satuan dasar.
	hari := jual["business_date"].(string)
	lap := call(t, "GET", "/api/v1/reports/sales?from="+hari+"&to="+hari+"&group_by=product&outlet_id="+f.outletID,
		f.token, nil).mustOK(t, "laporan").data(t)["rows"].([]any)
	if q := lap[0].(map[string]any)["qty"]; q != "23" {
		t.Fatalf("qty laporan = %v, mau 23 satuan dasar", q)
	}

	// Batal → stok kembali.
	call(t, "POST", "/api/v1/sales/"+jual["id"].(string)+"/void", f.token, map[string]any{"reason": "salah input"}).mustOK(t, "batal")
	if got := stockQty(t, f.tenantFixture, f.outletID, f.prodA); !sama(got, awal) {
		t.Fatalf("stok setelah batal = %s, mau %s", got, awal)
	}

	// Kemasan milik barang lain ditolak.
	checkout(t, f.token, ulid.New(), map[string]any{
		"outlet_id": f.outletID,
		"items":     []map[string]any{{"product_id": f.prodB, "product_unit_id": idDus, "qty": "1"}},
		"payments":  []map[string]any{{"method": "cash", "amount": 500000}},
	}).mustCode(t, "kemasan barang lain", 422)

	// Beli 3 dus @ 70.000 → stok +30, modal per pcs 7.000.
	beli := checkoutLike(t, f.token, ulid.New(), "/api/v1/purchases", map[string]any{
		"outlet_id": f.outletID,
		"items":     []map[string]any{{"product_id": f.prodA, "product_unit_id": idDus, "qty": "3", "unit_cost": 70000}},
	}).mustCode(t, "beli dus", 201).data(t)
	if it := beli["items"].([]any)[0].(map[string]any); it["unit_name"] != "dus" || it["unit_conversion"] != "10" {
		t.Fatalf("baris pembelian: %v", it)
	}
	assertI64(t, beli, "total", 210000)
	if got := stockQty(t, f.tenantFixture, f.outletID, f.prodA); !sama(got, kurang(awal, "-30")) {
		t.Fatalf("stok setelah beli 3 dus = %s", got)
	}
	if c := call(t, "GET", "/api/v1/products/"+f.prodA, f.token, nil).mustOK(t, "ambil A").data(t)["cost_price"]; c != float64(7000) {
		t.Fatalf("harga modal per pcs = %v, mau 7.000", c)
	}

	// Tersinkron ke perangkat.
	pull := call(t, "GET", "/api/v1/sync/pull?since=0&outlet_id="+f.outletID, f.token, nil).mustOK(t, "pull").data(t)
	n := 0
	for _, r := range pull["product_units"].([]any) {
		if r.(map[string]any)["id"] == idDus {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("kemasan di pull = %d", n)
	}
}
