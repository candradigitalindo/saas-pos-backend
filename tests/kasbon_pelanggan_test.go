package tests

import (
	"testing"
	"time"

	"candra/backend-api/database"
	"candra/backend-api/internal/ulid"
)

// Kasbon per pelanggan (000052): tempo kasbon → jatuh tempo; ringkasan per
// pelanggan (yang lewat jatuh tempo di atas); setoran pelanggan melunasi kasbon
// TERLAMA dulu, dan setoran tunai bisa masuk laci shift; kasbon yang baru
// dicicil tidak hilang dari "belum lunas".
func TestKasbonPelanggan(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "kasbonplg") // shift terbuka di outlet utama

	pelanggan := func(nama, hp string, tempo int) string {
		t.Helper()
		return call(t, "POST", "/api/v1/customers", f.token, map[string]any{
			"name": nama, "phone": hp, "credit_term_days": tempo,
		}).mustCode(t, "pelanggan "+nama, 201).data(t)["id"].(string)
	}
	sari := pelanggan("Bu Sari", "081200001111", 14)
	budi := pelanggan("Pak Budi", "081200002222", 0)

	kasbon := func(kunci, cust, prod, qty string, jumlah int64) {
		t.Helper()
		checkout(t, f.token, kunci, map[string]any{
			"outlet_id": f.outletID, "customer_id": cust,
			"items":    []map[string]any{{"product_id": prod, "qty": qty}},
			"payments": []map[string]any{{"method": "credit", "amount": jumlah}},
		}).mustCode(t, "kasbon "+kunci, 201)
	}
	kasbon("KP-1", sari, f.prodA, "2", 30000) // terlama
	kasbon("KP-2", sari, f.prodB, "1", 8000)
	kasbon("KP-3", budi, f.prodA, "1", 15000)

	sum := call(t, "GET", "/api/v1/receivable-summary?outlet_id="+f.outletID, f.token, nil).
		mustOK(t, "ringkasan").data(t)
	hariIni, _ := time.Parse("2006-01-02", sum["today"].(string))
	assertI64(t, sum, "outstanding", 53000)
	assertI64(t, sum, "count", 3)
	assertI64(t, sum, "overdue_count", 0)

	// Tempo 14 hari → jatuh tempo = tanggal usaha + 14; tanpa tempo → kosong.
	daftar := func(cust string) []map[string]any {
		t.Helper()
		d := call(t, "GET", "/api/v1/receivables?status=unpaid&customer_id="+cust, f.token, nil).
			mustOK(t, "kasbon "+cust).data(t)["data"].([]any)
		out := make([]map[string]any, len(d))
		for i, x := range d {
			out[i] = x.(map[string]any)
		}
		return out
	}
	ks := daftar(sari)
	if len(ks) != 2 || ks[0]["amount"].(float64) != 30000 {
		t.Fatalf("kasbon Bu Sari (terlama dulu): %v", ks)
	}
	if mau := hariIni.AddDate(0, 0, 14).Format("2006-01-02"); ks[0]["due_date"] != mau {
		t.Fatalf("jatuh tempo = %v, mau %s", ks[0]["due_date"], mau)
	}
	if ks[0]["receipt_no"] == "" || ks[0]["receipt_no"] == nil {
		t.Fatalf("nomor nota kasbon kosong: %v", ks[0])
	}
	kb := daftar(budi)
	if kb[0]["due_date"] != nil {
		t.Fatalf("kasbon tanpa tempo punya jatuh tempo: %v", kb[0])
	}

	// Janji bayar Pak Budi: kemarin → lewat jatuh tempo; ia naik ke atas.
	kemarin := hariIni.AddDate(0, 0, -1).Format("2006-01-02")
	call(t, "PUT", "/api/v1/receivables/"+kb[0]["id"].(string), f.token, map[string]any{"due_date": kemarin}).
		mustOK(t, "atur jatuh tempo")
	call(t, "PUT", "/api/v1/receivables/"+kb[0]["id"].(string), f.token, map[string]any{"due_date": "25-09-2026"}).
		mustCode(t, "format tanggal salah", 422)
	sum = call(t, "GET", "/api/v1/receivable-summary?outlet_id="+f.outletID, f.token, nil).mustOK(t, "ringkasan 2").data(t)
	assertI64(t, sum, "overdue_count", 1)
	assertI64(t, sum, "overdue_amount", 15000)
	pertama := sum["customers"].([]any)[0].(map[string]any)
	if pertama["customer_id"] != budi {
		t.Fatalf("yang lewat jatuh tempo harus di atas: %v", pertama)
	}

	// Statistik pelanggan: sisa & yang lewat jatuh tempo.
	pb := call(t, "GET", "/api/v1/customers/"+budi+"?outlet_id="+f.outletID, f.token, nil).mustOK(t, "pelanggan").data(t)
	st := pb["stats"].(map[string]any)
	assertI64(t, st, "receivable_outstanding", 15000)
	assertI64(t, st, "receivable_overdue", 15000)

	// Setoran lewat endpoint lama (per kasbon) → 'partial' — tetap belum lunas.
	checkoutLike(t, f.token, ulid.New(), "/api/v1/receivable-payments", map[string]any{
		"receivable_id": kb[0]["id"], "amount": 5000, "method": "transfer",
	}).mustCode(t, "cicil budi", 201)
	if kb = daftar(budi); len(kb) != 1 || kb[0]["outstanding"].(float64) != 10000 {
		t.Fatalf("kasbon dicicil hilang dari 'belum lunas': %v", kb)
	}
	pb = call(t, "GET", "/api/v1/customers/"+budi, f.token, nil).mustOK(t, "pelanggan 2").data(t)
	assertI64(t, pb["stats"].(map[string]any), "receivable_outstanding", 10000)

	// Setoran Bu Sari Rp 35.000 tunai ke laci: KP-1 (30.000) lunas, KP-2 sisa 3.000.
	laciMasuk := func() int64 {
		t.Helper()
		var n int64
		database.DB.Raw(`SELECT COALESCE(SUM(amount), 0) FROM cash_movements
			WHERE tenant_id = ? AND shift_id = ? AND direction = 'in' AND reason = 'Setoran kasbon Bu Sari'`,
			f.tenantID, f.shiftID).Scan(&n)
		return n
	}
	kunci := ulid.New()
	setor := map[string]any{"amount": 35000, "method": "cash", "source": "drawer", "outlet_id": f.outletID}
	d := checkoutLike(t, f.token, kunci, "/api/v1/customers/"+sari+"/receivable-payments", setor).
		mustCode(t, "setoran sari", 201).data(t)
	assertI64(t, d, "outstanding", 3000)
	al := d["allocations"].([]any)
	if len(al) != 2 || al[0].(map[string]any)["settled"] != true || al[1].(map[string]any)["amount"].(float64) != 5000 {
		t.Fatalf("pembagian setoran: %v", al)
	}
	if d["to_drawer"] != true || laciMasuk() != 35000 {
		t.Fatalf("setoran tunai tidak masuk laci: to_drawer=%v laci=%d", d["to_drawer"], laciMasuk())
	}
	// Kirim ulang dengan kunci sama → jawaban sama, tidak tercatat dua kali.
	checkoutLike(t, f.token, kunci, "/api/v1/customers/"+sari+"/receivable-payments", setor).mustCode(t, "ulang", 201)
	if laciMasuk() != 35000 {
		t.Fatalf("setoran ulang tercatat dua kali: %d", laciMasuk())
	}
	// Laci shift ikut: uang masuk (cash_in) memuat setoran.
	sh := call(t, "GET", "/api/v1/shifts/"+f.shiftID, f.token, nil).mustOK(t, "shift").data(t)
	assertI64(t, sh, "cash_in", 35000)

	// Melebihi sisa ditolak; non-tunai tidak bisa ke laci.
	checkoutLike(t, f.token, ulid.New(), "/api/v1/customers/"+sari+"/receivable-payments",
		map[string]any{"amount": 3001, "method": "cash", "source": "other"}).mustCode(t, "lebih", 422)
	checkoutLike(t, f.token, ulid.New(), "/api/v1/customers/"+sari+"/receivable-payments",
		map[string]any{"amount": 1000, "method": "qris", "source": "drawer", "outlet_id": f.outletID}).
		mustCode(t, "qris ke laci", 422)

	// Lunasi sisanya dengan uang lain → laci tetap; tidak ada kasbon lagi.
	checkoutLike(t, f.token, ulid.New(), "/api/v1/customers/"+sari+"/receivable-payments",
		map[string]any{"amount": 3000, "method": "transfer", "note": "via BCA"}).mustCode(t, "lunas", 201)
	if laciMasuk() != 35000 || len(daftar(sari)) != 0 {
		t.Fatalf("setelah lunas: laci %d, kasbon %d", laciMasuk(), len(daftar(sari)))
	}
	checkoutLike(t, f.token, ulid.New(), "/api/v1/customers/"+sari+"/receivable-payments",
		map[string]any{"amount": 1000, "method": "cash"}).mustCode(t, "sudah lunas", 409)

	// Riwayat setoran: 3 baris (2 dari setoran laci, 1 transfer), terbaru dulu.
	rw := call(t, "GET", "/api/v1/customers/"+sari+"/receivable-payments", f.token, nil).mustOK(t, "riwayat setoran")
	baris := rw.Body["data"].([]any)
	if len(baris) != 3 || baris[0].(map[string]any)["note"] != "via BCA" || baris[1].(map[string]any)["to_drawer"] != true {
		t.Fatalf("riwayat setoran: %v", baris)
	}

	// Setoran tunai ke laci saat kasir tutup → ditolak dengan arahan.
	call(t, "POST", "/api/v1/shifts/"+f.shiftID+"/close", f.token, map[string]any{"counted_cash": 0}).mustOK(t, "tutup shift")
	checkoutLike(t, f.token, ulid.New(), "/api/v1/customers/"+budi+"/receivable-payments",
		map[string]any{"amount": 1000, "method": "cash", "source": "drawer", "outlet_id": f.outletID}).
		mustCode(t, "laci tutup", 422)

	// Riwayat belanja & barang yang sering dibeli.
	rs := call(t, "GET", "/api/v1/sales?customer_id="+sari, f.token, nil).mustOK(t, "belanja sari").data(t)
	assertI64(t, rs, "total", 2)
	tp := call(t, "GET", "/api/v1/customers/"+sari+"/top-products", f.token, nil).mustOK(t, "sering dibeli")
	top := tp.Body["data"].([]any)
	if len(top) != 2 || top[0].(map[string]any)["times"].(float64) != 1 {
		t.Fatalf("sering dibeli: %v", top)
	}
}
