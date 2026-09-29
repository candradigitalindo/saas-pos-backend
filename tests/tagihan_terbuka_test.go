package tests

import (
	"testing"

	"candra/backend-api/internal/ulid"
)

// Tagihan terbuka (open bill): pesanan yang disimpan di server, dibayar nanti.
//
// Yang dijaga: simpan utuh dengan pemeriksaan versi (versi basi → 409, bukan
// menimpa); dibayar lewat checkout ber-open_bill_id → tertutup di transaksi
// yang sama dan TIDAK bisa dibayar dua kali; dibatalkan tetap tercatat;
// operasi antrean offline idempoten (dikirim ulang = duplikat, versi basi =
// ditolak); tenant lain tidak melihat tagihan kita.
func TestTagihanTerbuka(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "tagihan")
	jalur := func(id string) string { return "/api/v1/open-bills/" + id }
	isi := func(label string, versi int, items ...map[string]any) map[string]any {
		return map[string]any{"outlet_id": f.outletID, "label": label, "items": items, "base_version": versi}
	}
	kopi := func(qty string) map[string]any { return map[string]any{"product_id": f.prodA, "qty": qty} }
	daftar := func() []any {
		t.Helper()
		return call(t, "GET", "/api/v1/open-bills?outlet_id="+f.outletID, f.token, nil).mustOK(t, "daftar").Body["data"].([]any)
	}

	// Buat & ubah dengan versi.
	meja5 := ulid.New()
	b := call(t, "PUT", jalur(meja5), f.token, isi(" Meja 5 ", 0, kopi("1"))).mustCode(t, "buat", 201).data(t)
	if b["label"] != "Meja 5" || b["version"] != float64(1) || b["created_by_name"] == "" {
		t.Fatalf("tagihan baru: %v", b)
	}
	tambah := isi("Meja 5", 1, kopi("2"), map[string]any{"product_id": f.prodB, "qty": "1", "note": "tanpa es", "discount_percent": 10})
	b = call(t, "PUT", jalur(meja5), f.token, tambah).mustOK(t, "ubah").data(t)
	if b["version"] != float64(2) || len(b["items"].([]any)) != 2 {
		t.Fatalf("tagihan diubah: %v", b)
	}
	// Perangkat lain masih memegang versi 1 → ditolak, bukan menimpa.
	call(t, "PUT", jalur(meja5), f.token, isi("Meja 5", 1)).mustCode(t, "versi basi", 409)
	call(t, "PUT", jalur(ulid.New()), f.token, isi("  ", 0)).mustCode(t, "label kosong", 422)
	call(t, "PUT", jalur(ulid.New()), f.token, isi("X", 0, kopi("0"))).mustCode(t, "qty nol", 422)
	if n := len(daftar()); n != 1 {
		t.Fatalf("daftar tagihan = %d, mau 1", n)
	}

	// Dibayar: tertutup dan hilang dari daftar; dibayar kedua kali → 409.
	bayar := func(bill string) apiResp {
		return checkout(t, f.token, ulid.New(), map[string]any{
			"outlet_id": f.outletID, "open_bill_id": bill,
			"items":    []map[string]any{kopi("2")},
			"payments": []map[string]any{{"method": "cash", "amount": 30000}},
		})
	}
	jual := bayar(meja5).mustCode(t, "bayar tagihan", 201).data(t)
	if len(daftar()) != 0 {
		t.Fatal("tagihan yang dibayar masih di daftar")
	}
	bayar(meja5).mustCode(t, "bayar dua kali", 409)
	call(t, "PUT", jalur(meja5), f.token, isi("Meja 5", 3)).mustCode(t, "ubah tagihan terbayar", 409)
	_ = jual

	// Dibatalkan: versi wajib cocok; tetap boleh dibayar (uang nyata menang).
	batal := ulid.New()
	call(t, "PUT", jalur(batal), f.token, isi("Pak Budi", 0, kopi("1"))).mustCode(t, "buat 2", 201)
	call(t, "POST", jalur(batal)+"/cancel", f.token, map[string]any{"base_version": 2}).mustCode(t, "batal basi", 409)
	call(t, "POST", jalur(batal)+"/cancel", f.token, map[string]any{"base_version": 1}).mustOK(t, "batal")
	call(t, "POST", jalur(batal)+"/cancel", f.token, map[string]any{"base_version": 2}).mustCode(t, "batal lagi", 409)
	bayar(batal).mustCode(t, "bayar yang dibatalkan", 201)
	// Tagihan yang tidak ada (dibuat offline lalu ditolak) tidak menggagalkan penjualan.
	bayar(ulid.New()).mustCode(t, "tagihan tak dikenal", 201)

	// Antrean offline: buat, kirim ulang (duplikat), ubah dari versi basi (ditolak).
	offline := ulid.New()
	opBuat := map[string]any{"op": "open_bill.upsert", "id": ulid.New(), "payload": map[string]any{
		"id": offline, "outlet_id": f.outletID, "label": "Gojek 2", "items": []map[string]any{kopi("1")}, "base_version": 0,
	}}
	if r := push(t, f.token, opBuat); r["applied"] != float64(1) {
		t.Fatalf("push buat: %v", r)
	}
	if r := push(t, f.token, opBuat); r["duplicate"] != float64(1) {
		t.Fatalf("push ulang harus duplikat: %v", r)
	}
	opUbah := func(versi int) map[string]any {
		return map[string]any{"op": "open_bill.upsert", "id": ulid.New(), "payload": map[string]any{
			"id": offline, "outlet_id": f.outletID, "label": "Gojek 2", "items": []map[string]any{kopi("3")}, "base_version": versi,
		}}
	}
	if r := push(t, f.token, opUbah(1), opUbah(1)); r["applied"] != float64(1) || r["rejected"] != float64(1) {
		t.Fatalf("push ubah (kedua basi): %v", r)
	}
	opBatal := map[string]any{"op": "open_bill.cancel", "id": ulid.New(), "payload": map[string]any{"id": offline, "base_version": 2}}
	if r := push(t, f.token, opBatal); r["applied"] != float64(1) {
		t.Fatalf("push batal: %v", r)
	}
	if r := push(t, f.token, opBatal); r["duplicate"] != float64(1) {
		t.Fatalf("push batal ulang harus duplikat: %v", r)
	}

	// Tenant lain tidak melihat & tidak bisa menimpa.
	g := setupPOS(t, "tagihan-lain")
	sisa := ulid.New()
	call(t, "PUT", jalur(sisa), f.token, isi("Meja 9", 0)).mustCode(t, "buat 3", 201)
	if d := call(t, "GET", "/api/v1/open-bills?outlet_id="+g.outletID, g.token, nil).mustOK(t, "daftar lain").Body["data"].([]any); len(d) != 0 {
		t.Fatalf("tenant lain melihat tagihan: %v", d)
	}
	call(t, "PUT", jalur(sisa), g.token, map[string]any{"outlet_id": g.outletID, "label": "Curang", "base_version": 0}).
		mustCode(t, "id milik tenant lain", 409)
}
