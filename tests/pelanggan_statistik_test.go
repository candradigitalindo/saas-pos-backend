package tests

import (
	"testing"
)

// Uji ringkasan pelanggan di daftar pelanggan: berapa kali datang, total
// belanja, kapan terakhir datang, dan sisa kasbon.
//
// Yang dijaga: angkanya BERSIH (void tak ikut, retur mengurangi — aturan yang
// sama dengan laporan), dan sisa kasbon hanya terlihat oleh yang berhak
// mengurus kasbon. Pelanggan yang belum pernah belanja tetap punya ringkasan
// bernilai nol, bukan kosong.
func TestDaftarPelangganMemuatRingkasanBelanja(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "pelstat")
	rina := makeCustomer(t, f.token, "Bu Rina")
	baru := makeCustomer(t, f.token, "Pak Baru")

	jual := func(kunci string, qty string, bayar []map[string]any) map[string]any {
		t.Helper()
		return checkout(t, f.token, kunci, map[string]any{
			"outlet_id": f.outletID, "customer_id": rina,
			"items":    []map[string]any{{"product_id": f.prodA, "qty": qty}},
			"payments": bayar,
		}).mustCode(t, "jual "+kunci, 201).data(t)
	}
	tunai := func(n int64) []map[string]any { return []map[string]any{{"method": "cash", "amount": n}} }

	jual("PS-1", "2", tunai(30000))                                            // 30.000
	batal := jual("PS-2", "1", tunai(15000))                                   // dibatalkan
	retur := jual("PS-3", "1", tunai(15000))                                   // diretur
	jual("PS-4", "2", []map[string]any{{"method": "credit", "amount": 30000}}) // kasbon 30.000

	call(t, "POST", "/api/v1/sales/"+batal["id"].(string)+"/void", f.token,
		map[string]any{"reason": "salah"}).mustOK(t, "void")
	call(t, "POST", "/api/v1/sales/"+retur["id"].(string)+"/refund", f.token,
		map[string]any{"reason": "rusak"}).mustOK(t, "retur")

	pelanggan := func(token string) map[string]map[string]any {
		t.Helper()
		out := map[string]map[string]any{}
		for _, r := range call(t, "GET", "/api/v1/customers", token, nil).
			mustOK(t, "daftar pelanggan").data(t)["data"].([]any) {
			c := r.(map[string]any)
			out[c["id"].(string)] = c
		}
		return out
	}

	semua := pelanggan(f.token)
	st, ok := semua[rina]["stats"].(map[string]any)
	if !ok {
		t.Fatalf("pelanggan tidak membawa stats: %v", semua[rina])
	}
	// Datang: PS-1, PS-3, PS-4 (PS-2 dibatalkan). Retur tidak dihitung kedatangan.
	assertI64(t, st, "visit_count", 3)
	// Belanja: 30.000 + 15.000 − 15.000 (retur) + 30.000 = 60.000.
	assertI64(t, st, "total_spent", 60000)
	if st["last_visit_at"] == nil || st["last_visit_at"] == "" {
		t.Fatalf("last_visit_at kosong: %v", st)
	}
	assertI64(t, st, "receivable_outstanding", 30000)

	kosong := semua[baru]["stats"].(map[string]any)
	assertI64(t, kosong, "visit_count", 0)
	assertI64(t, kosong, "total_spent", 0)
	if _, ada := kosong["last_visit_at"]; ada {
		t.Fatalf("pelanggan tanpa belanja tidak boleh punya last_visit_at: %v", kosong)
	}

	// Peran tanpa receivable.manage: ringkasan belanja tetap ada, SISA KASBON
	// tidak dikirim sama sekali. (crm.lead.view.all supaya pelanggan milik
	// pemilik terlihat — lapis visibilitas bukan yang diuji di sini.)
	rid := call(t, "POST", "/api/v1/roles", f.token, map[string]any{
		"name":             "Lihat Pelanggan",
		"permission_codes": []string{"customer.view", "crm.lead.view.all"},
	}).mustCode(t, "buat peran", 201).data(t)["id"].(string)
	staf := staffToken(t, f.tenantFixture, rid, "staf_pelstat")

	stStaf := pelanggan(staf)[rina]["stats"].(map[string]any)
	assertI64(t, stStaf, "total_spent", 60000)
	if _, ada := stStaf["receivable_outstanding"]; ada {
		t.Fatalf("sisa kasbon bocor ke peran tanpa receivable.manage: %v", stStaf)
	}
}
