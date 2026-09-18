package tests

import (
	"strings"
	"testing"
	"time"

	"candra/backend-api/database"
)

// Uji laporan per JAM (group_by=hour) — "jam berapa warung saya paling ramai".
//
// Yang dijaga di sini cuma satu hal, tapi hal itu yang menentukan seluruh
// gunanya: jam dihitung di ZONA WAKTU OUTLET, bukan UTC. `occurred_at` disimpan
// UTC, Indonesia di UTC+7..+9, jadi dibaca mentah penjualan pukul 07.00 WIB
// tercatat pukul 00.00 — dan grafiknya akan memberi tahu pemilik warung bahwa
// ia paling ramai tengah malam.

// setelWaktuPenjualan memaksa occurred_at & business_date sebuah penjualan ke
// nilai tetap, supaya hasilnya tidak bergantung pada jam tes dijalankan.
func setelWaktuPenjualan(t *testing.T, saleID string, utc time.Time, bisnis string) {
	t.Helper()
	if err := database.DB.Table("sales").Where("id = ?", saleID).
		Updates(map[string]any{"occurred_at": utc, "business_date": bisnis}).Error; err != nil {
		t.Fatalf("menyetel waktu penjualan: %v", err)
	}
}

func TestLaporanPerJamMemakaiZonaOutlet(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "jamramai")

	call(t, "PUT", "/api/v1/outlets/"+f.outletID, f.token, map[string]any{
		"timezone": "Asia/Jakarta", "business_day_start": "00:00",
	}).mustOK(t, "setel zona outlet")

	sale := checkout(t, f.token, "JAM-1", map[string]any{
		"outlet_id": f.outletID,
		"items":     []map[string]any{{"product_id": f.prodA, "qty": "2"}},
		"payments":  []map[string]any{{"method": "cash", "amount": 30000}},
	}).mustCode(t, "jual", 201).data(t)

	// 00:30 UTC = 07:30 WIB. Jam UTC ("00") dan jam WIB ("07") sengaja berbeda
	// jauh, supaya kekeliruan tidak bisa lolos karena kebetulan.
	const hari = "2026-09-18"
	setelWaktuPenjualan(t, sale["id"].(string),
		time.Date(2026, 9, 18, 0, 30, 0, 0, time.UTC), hari)

	baris := barisPerJam(t, f, hari)
	if len(baris) != 1 {
		t.Fatalf("baris = %d, mau 1; isi: %+v", len(baris), baris)
	}
	if got := baris[0]["key"].(string); got != "07" {
		t.Fatalf("jam = %q, mau \"07\" (07.30 WIB). %q berarti jamnya dibaca dari UTC — "+
			"grafik jam teramai akan menunjuk waktu yang salah", got, got)
	}
	if n := baris[0]["sales_count"].(float64); n != 1 {
		t.Fatalf("jumlah transaksi = %v, mau 1", n)
	}
}

// barisPerJam memanggil endpoint laporan dengan group_by=hour.
func barisPerJam(t *testing.T, f posFixture, hari string) []map[string]any {
	t.Helper()
	d := call(t, "GET",
		"/api/v1/reports/sales?from="+hari+"&to="+hari+"&group_by=hour&outlet_id="+f.outletID,
		f.token, nil).mustOK(t, "laporan per jam").data(t)
	var out []map[string]any
	for _, r := range d["rows"].([]any) {
		out = append(out, r.(map[string]any))
	}
	return out
}

// Zona diambil dari OUTLET-nya, bukan dari satu zona tetap: penjualan yang sama
// persis harus melaporkan jam berbeda saat outletnya berpindah zona. Inilah yang
// membuat tenant dengan cabang di Jakarta dan Jayapura tetap benar.
func TestLaporanPerJamIkutZonaTiapOutlet(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "jamzona")

	sale := checkout(t, f.token, "JAM-2", map[string]any{
		"outlet_id": f.outletID,
		"items":     []map[string]any{{"product_id": f.prodA, "qty": "1"}},
		"payments":  []map[string]any{{"method": "cash", "amount": 15000}},
	}).mustCode(t, "jual", 201).data(t)

	const hari = "2026-09-18"
	setelWaktuPenjualan(t, sale["id"].(string),
		time.Date(2026, 9, 18, 3, 0, 0, 0, time.UTC), hari)

	jamDi := func(zona string) string {
		t.Helper()
		call(t, "PUT", "/api/v1/outlets/"+f.outletID, f.token, map[string]any{
			"timezone": zona, "business_day_start": "00:00",
		}).mustOK(t, "setel zona "+zona)

		baris := barisPerJam(t, f, hari)
		if len(baris) != 1 {
			t.Fatalf("baris = %d, mau 1 (%s)", len(baris), zona)
		}
		return baris[0]["key"].(string)
	}

	// 03:00 UTC → 10.00 WIB (+7) → 12.00 WIT (+9).
	if got := jamDi("Asia/Jakarta"); got != "10" {
		t.Fatalf("Asia/Jakarta: jam = %q, mau \"10\"", got)
	}
	if got := jamDi("Asia/Jayapura"); got != "12" {
		t.Fatalf("Asia/Jayapura: jam = %q, mau \"12\" — zona tidak diambil dari outlet", got)
	}
}

// group_by yang tidak dikenal tetap ditolak, dan pesannya menyebut "hour".
func TestLaporanGroupByTidakDikenal(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "jamsalah")

	res := call(t, "GET",
		"/api/v1/reports/sales?from=2026-09-18&to=2026-09-18&group_by=menit&outlet_id="+f.outletID,
		f.token, nil)
	if res.Code < 400 {
		t.Fatalf("group_by tak dikenal seharusnya ditolak, dapat %d", res.Code)
	}
	if !strings.Contains(res.Raw, "hour") {
		t.Fatalf("pesan galat tidak menyebut pilihan \"hour\": %s", res.Raw)
	}
}
