package tests

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"candra/backend-api/internal/ulid"
)

// Ekspor XLSX: berkas .xlsx sungguhan (zip Office Open XML) — terbuka benar
// di Excel berbahasa apa pun, angka tersimpan sebagai angka, judul tebal &
// dibekukan. CSV tetap tersedia untuk program.
func TestEksporXLSX(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "eksporxlsx")
	loc, _ := time.LoadLocation("Asia/Jakarta")
	hari := time.Now().In(loc).Format("2006-01-02")
	sup := call(t, "POST", "/api/v1/suppliers", f.token, map[string]any{"name": "Toko A & B <Grosir>"}).
		mustCode(t, "pemasok", 201).data(t)["id"].(string)
	checkoutLike(t, f.token, ulid.New(), "/api/v1/purchases", map[string]any{
		"outlet_id": f.outletID, "supplier_id": sup, "invoice_no": "X-1", "paid_amount": 10000,
		"items": []map[string]any{{"product_id": f.prodA, "qty": "2.5", "unit_cost": 6000}},
	}).mustCode(t, "beli", 201)

	code, ctype, body := rawResponse(t, "/api/v1/reports/export?type=purchases&format=xlsx&outlet_id="+f.outletID+
		"&from="+hari+"&to="+hari, f.token)
	if code != 200 || !strings.HasPrefix(ctype, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet") {
		t.Fatalf("xlsx: %d %q", code, ctype)
	}
	z, err := zip.NewReader(bytes.NewReader([]byte(body)), int64(len(body)))
	if err != nil {
		t.Fatalf("bukan zip: %v", err)
	}
	isi := map[string]string{}
	for _, berkas := range z.File {
		r, _ := berkas.Open()
		b, _ := io.ReadAll(r)
		isi[berkas.Name] = string(b)
	}
	for _, wajib := range []string{"[Content_Types].xml", "xl/workbook.xml", "xl/styles.xml", "xl/worksheets/sheet1.xml"} {
		if _, ada := isi[wajib]; !ada {
			t.Fatalf("xlsx tanpa %s", wajib)
		}
	}
	lembar := isi["xl/worksheets/sheet1.xml"]
	for _, mau := range []string{
		`<c r="A1" t="inlineStr" s="1"><is><t xml:space="preserve">Tanggal</t>`, // judul tebal
		`state="frozen"`,                   // baris judul dibekukan
		`Toko A &amp; B &lt;Grosir&gt;`,    // teks diloloskan
		`<c r="E2"><v>2.5</v></c>`,         // jumlah desimal sebagai angka
		`<c r="H2" s="2"><v>15000</v></c>`, // subtotal sebagai angka
		`<c r="K2" s="2"><v>5000</v></c>`,  // sisa nota
	} {
		if !strings.Contains(lembar, mau) {
			t.Fatalf("lembar tidak memuat %s:\n%s", mau, lembar)
		}
	}
	if !strings.Contains(isi["xl/workbook.xml"], `name="Belanja"`) {
		t.Fatalf("nama lembar: %s", isi["xl/workbook.xml"])
	}

	// Laporan penjualan juga bisa .xlsx; nama berkasnya berakhiran .xlsx.
	req := httptest.NewRequest("GET", "/api/v1/reports/export?type=sales&format=xlsx&from="+hari+"&to="+hari, nil)
	req.Header.Set("Authorization", "Bearer "+f.token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != 200 || !strings.Contains(rec.Header().Get("Content-Disposition"), `.xlsx"`) {
		t.Fatalf("penjualan xlsx: %d %q", rec.Code, rec.Header().Get("Content-Disposition"))
	}
}
