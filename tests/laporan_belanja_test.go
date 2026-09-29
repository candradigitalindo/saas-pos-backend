package tests

import (
	"encoding/csv"
	"fmt"
	"strings"
	"testing"
	"time"

	"candra/backend-api/database"
	"candra/backend-api/internal/ulid"
)

// Laporan belanja & utang pemasok (GET /reports/purchases): nota menurut
// tanggal usaha notanya, pembayaran menurut tanggal usaha pembayarannya —
// pelunasan hari ini untuk nota bulan lalu tetap terhitung "uang keluar"
// periode ini. Butuh report.view DAN stock.view.
func TestLaporanBelanja(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "lapbelanja") // shift terbuka; A modal 6.000, B 3.000
	loc, _ := time.LoadLocation("Asia/Jakarta")
	hariIni := time.Now().In(loc)
	dari, sampai := hariIni.AddDate(0, 0, -6).Format("2006-01-02"), hariIni.Format("2006-01-02")

	pemasok := func(nama string) string {
		t.Helper()
		return call(t, "POST", "/api/v1/suppliers", f.token, map[string]any{"name": nama}).
			mustCode(t, "pemasok", 201).data(t)["id"].(string)
	}
	s1, s2 := pemasok("Grosir Satu"), pemasok("Grosir Dua")
	beli := func(body map[string]any) string {
		t.Helper()
		body["outlet_id"] = f.outletID
		return checkoutLike(t, f.token, ulid.New(), "/api/v1/purchases", body).mustCode(t, "beli", 201).data(t)["id"].(string)
	}
	// Nota lama (40 hari lalu, di luar rentang): 6.000 belum dibayar.
	lama := beli(map[string]any{"supplier_id": s1, "items": []map[string]any{{"product_id": f.prodB, "qty": "2", "unit_cost": 3000}}})
	database.DB.Exec(`UPDATE purchases SET business_date = business_date - 40, occurred_at = occurred_at - interval '40 days' WHERE id = ?`, lama)
	// Di rentang:
	beli(map[string]any{"supplier_id": s1, "paid_amount": 20000, "payment_source": "drawer",
		"items": []map[string]any{{"product_id": f.prodA, "qty": "10", "unit_cost": 6000}}}) // 60.000, sisa 40.000
	beli(map[string]any{"supplier_id": s2, "paid_amount": 15000,
		"items": []map[string]any{{"product_id": f.prodB, "qty": "5", "unit_cost": 3000}}}) // 15.000 lunas
	beli(map[string]any{"items": []map[string]any{{"product_id": f.prodA, "qty": "1", "unit_cost": 6000}}}) // 6.000, tanpa pemasok
	// Pelunasan sebagian nota lama HARI INI.
	checkoutLike(t, f.token, ulid.New(), "/api/v1/purchases/"+lama+"/payments", map[string]any{"amount": 1000, "source": "other"}).
		mustCode(t, "cicil nota lama", 201)

	r := call(t, "GET", "/api/v1/reports/purchases?outlet_id="+f.outletID+"&from="+dari+"&to="+sampai, f.token, nil).
		mustOK(t, "laporan belanja").data(t)
	tot := r["totals"].(map[string]any)
	assertI64(t, tot, "nota_count", 3)
	assertI64(t, tot, "belanja", 81000)
	assertI64(t, tot, "sisa_nota", 46000) // 40.000 + 0 + 6.000
	assertI64(t, tot, "dibayar", 36000)   // 20.000 laci + 15.000 + 1.000 (nota lama, dibayar hari ini)
	assertI64(t, tot, "dari_laci", 20000)
	assertI64(t, tot, "dari_lain", 16000)
	assertI64(t, tot, "utang_kini", 51000) // 40.000 + 6.000 + 5.000 (nota lama)

	sup := r["suppliers"].([]any)
	if len(sup) != 3 || sup[0].(map[string]any)["supplier_name"] != "Grosir Satu" {
		t.Fatalf("per pemasok: %v", sup)
	}
	assertI64(t, sup[0].(map[string]any), "belanja", 60000)
	if last := sup[2].(map[string]any); last["supplier_id"] != nil || last["belanja"] != float64(6000) {
		t.Fatalf("tanpa pemasok: %v", last)
	}
	barang := r["products"].([]any)
	if a := barang[0].(map[string]any); a["product_id"] != f.prodA || a["qty"] != "11" || a["nilai"] != float64(66000) {
		t.Fatalf("barang terbesar: %v", a)
	}
	if hari := r["days"].([]any); len(hari) != 1 || hari[0].(map[string]any)["belanja"] != float64(81000) {
		t.Fatalf("per hari: %v", hari)
	}

	// Ekspor untuk pembukuan: baris barang per nota, dan pembayaran.
	baca := func(tipe string) [][]string {
		t.Helper()
		code, ctype, body := rawResponse(t, "/api/v1/reports/export?type="+tipe+"&format=csv&outlet_id="+f.outletID+
			"&from="+dari+"&to="+sampai, f.token)
		if code != 200 || !strings.HasPrefix(ctype, "text/csv") {
			t.Fatalf("ekspor %s: %d %q %s", tipe, code, ctype, body)
		}
		rows, err := csv.NewReader(strings.NewReader(body)).ReadAll()
		if err != nil {
			t.Fatalf("csv %s: %v", tipe, err)
		}
		return rows
	}
	nota := baca("purchases")
	if len(nota) != 4 || nota[0][0] != "Tanggal" || nota[0][10] != "Sisa Nota" {
		t.Fatalf("ekspor nota: %v", nota)
	}
	var sisaSatu string
	for _, r := range nota[1:] {
		if r[2] == "Grosir Satu" {
			sisaSatu = r[10]
		}
	}
	if sisaSatu != "40000" {
		t.Fatalf("sisa nota Grosir Satu di ekspor: %q, mau 40000", sisaSatu)
	}
	bayar := baca("purchase_payments")
	if len(bayar) != 5 || bayar[4][0] != "TOTAL" || bayar[4][4] != "36000" {
		t.Fatalf("ekspor pembayaran: %v", bayar)
	}
	if !strings.Contains(fmt.Sprint(bayar), "Laci kasir") {
		t.Fatalf("sumber laci tidak tertulis: %v", bayar)
	}

	// Kedua izin wajib.
	for _, izin := range [][]string{{"report.view"}, {"stock.view"}} {
		rid := call(t, "POST", "/api/v1/roles", f.token, map[string]any{
			"name": "Uji " + izin[0], "permission_codes": izin,
		}).mustCode(t, "peran", 201).data(t)["id"].(string)
		tok := staffToken(t, f.tenantFixture, rid, "lapbelanja_"+izin[0][:4])
		call(t, "GET", "/api/v1/reports/purchases?from="+dari+"&to="+sampai, tok, nil).mustCode(t, "hanya "+izin[0], 403)
	}
	// Izin ekspor saja tidak cukup untuk berkas belanja.
	rid := call(t, "POST", "/api/v1/roles", f.token, map[string]any{
		"name": "Uji ekspor saja", "permission_codes": []string{"report.export"},
	}).mustCode(t, "peran ekspor", 201).data(t)["id"].(string)
	tok := staffToken(t, f.tenantFixture, rid, "lapbelanja_ekspor")
	if code, _, _ := rawResponse(t, "/api/v1/reports/export?type=purchases&format=csv&from="+dari+"&to="+sampai, tok); code != 403 {
		t.Fatalf("ekspor belanja tanpa stock.view: %d, mau 403", code)
	}
}
