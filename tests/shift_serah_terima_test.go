package tests

import (
	"testing"

	"candra/backend-api/database"
)

// Uji serah terima shift: tutup + buka dalam SATU transaksi.
//
// Yang dijaga adalah rantai uangnya. Uang laci yang dihitung saat pergantian
// harus jadi modal awal shift berikutnya, dan shift lama harus tercatat
// selisihnya. Kalau rantai itu putus, uang berpindah tangan tanpa jejak — dan
// itu justru satu-satunya alasan fitur ini ada.

// serahTerima memanggil endpoint serah terima.
func serahTerima(t *testing.T, f posFixture, shiftID string, payload map[string]any) apiResp {
	t.Helper()
	return call(t, "POST", "/api/v1/shifts/"+shiftID+"/handover", f.token, payload)
}

// shiftTerbukaDi menghitung shift yang masih terbuka di sebuah outlet.
func shiftTerbukaDi(t *testing.T, outletID string) int64 {
	t.Helper()
	var n int64
	if err := database.DB.Table("shifts").
		Where("outlet_id = ? AND status = 'open'", outletID).Count(&n).Error; err != nil {
		t.Fatalf("menghitung shift terbuka: %v", err)
	}
	return n
}

func TestSerahTerimaMeneruskanUangLaci(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "serah1")

	// Modal awal dibaca dari shift-nya, tidak diasumsikan: nilai fixture bisa
	// berubah, dan tes yang memaku angkanya akan gagal karena alasan yang
	// tidak ada hubungannya dengan serah terima.
	modalAwal := int64(call(t, "GET", "/api/v1/shifts/"+f.shiftID, f.token, nil).
		mustOK(t, "baca shift").data(t)["opening_cash"].(float64))

	// Satu penjualan tunai supaya laci tidak cuma berisi modal awal.
	const tunai = 30000
	checkout(t, f.token, "ST-1", map[string]any{
		"outlet_id": f.outletID,
		"items":     []map[string]any{{"product_id": f.prodA, "qty": "2"}},
		"payments":  []map[string]any{{"method": "cash", "amount": tunai}},
	}).mustCode(t, "jual tunai", 201)

	diLaci := modalAwal + tunai
	d := serahTerima(t, f, f.shiftID, map[string]any{
		"counted_cash": diLaci,
		"note":         "Ganti ke shift sore",
	}).mustOK(t, "serah terima").data(t)

	ditutup := d["ditutup"].(map[string]any)
	dibuka := d["dibuka"].(map[string]any)

	if ditutup["status"] != "closed" {
		t.Fatalf("shift lama status = %v, mau closed", ditutup["status"])
	}
	if got := int64(ditutup["difference"].(float64)); got != 0 {
		t.Fatalf("selisih = %d, mau 0 (%d dihitung, modal %d + tunai %d)",
			got, diLaci, modalAwal, tunai)
	}
	if dibuka["status"] != "open" {
		t.Fatalf("shift baru status = %v, mau open", dibuka["status"])
	}
	// Inti fiturnya: uang yang dihitung menjadi modal shift berikutnya.
	if got := int64(dibuka["opening_cash"].(float64)); got != diLaci {
		t.Fatalf("modal awal shift baru = %d, mau %d — rantai uangnya putus", got, diLaci)
	}
	if dibuka["outlet_id"] != f.outletID {
		t.Fatalf("shift baru di outlet %v, mau %s", dibuka["outlet_id"], f.outletID)
	}
	if dibuka["id"] == ditutup["id"] {
		t.Fatal("shift baru dan lama beridentitas sama")
	}
	if n := shiftTerbukaDi(t, f.outletID); n != 1 {
		t.Fatalf("%d shift terbuka di outlet, mau tepat 1", n)
	}
}

// Sebagian uang disetor ke brankas: yang ditinggal hanya uang kembalian.
func TestSerahTerimaBolehMeninggalkanSebagian(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "serah2")

	d := serahTerima(t, f, f.shiftID, map[string]any{
		"counted_cash": 200000,
		"opening_cash": 50000, // sisanya masuk brankas
	}).mustOK(t, "serah terima sebagian").data(t)

	if got := int64(d["dibuka"].(map[string]any)["opening_cash"].(float64)); got != 50000 {
		t.Fatalf("modal awal = %d, mau 50000", got)
	}
	if got := int64(d["ditutup"].(map[string]any)["counted_cash"].(float64)); got != 200000 {
		t.Fatalf("uang dihitung = %d, mau 200000", got)
	}
}

// Modal yang ditinggal tidak boleh melebihi uang yang ada di laci — itu
// menciptakan uang dari ketiadaan.
func TestSerahTerimaTolakModalMelebihiLaci(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "serah3")

	res := serahTerima(t, f, f.shiftID, map[string]any{
		"counted_cash": 100000,
		"opening_cash": 150000,
	})
	if res.Code < 400 {
		t.Fatalf("status %d — modal melebihi isi laci seharusnya ditolak", res.Code)
	}
	if n := shiftTerbukaDi(t, f.outletID); n != 1 {
		t.Fatalf("%d shift terbuka setelah penolakan, mau tetap 1", n)
	}
}

// Shift yang sudah ditutup tidak bisa diserahterimakan lagi, dan penolakannya
// TIDAK boleh menyisakan shift baru yang terlanjur dibuka.
func TestSerahTerimaTolakShiftYangSudahDitutup(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "serah4")

	call(t, "POST", "/api/v1/shifts/"+f.shiftID+"/close", f.token,
		map[string]any{"counted_cash": 200000}).mustOK(t, "tutup shift")

	res := serahTerima(t, f, f.shiftID, map[string]any{"counted_cash": 200000})
	if res.Code < 400 {
		t.Fatalf("status %d — shift tertutup seharusnya ditolak", res.Code)
	}
	if n := shiftTerbukaDi(t, f.outletID); n != 0 {
		t.Fatalf("%d shift terbuka setelah penolakan, mau 0 — transaksi tidak dibatalkan utuh", n)
	}
}
