package tests

import (
	"fmt"
	"testing"

	"candra/backend-api/database"
	"candra/backend-api/internal/ulid"
)

// Uji integrasi Fase 6 — sinkronisasi offline (§10).

// saleOp membangun satu operasi "sale.create" untuk /sync/push: jual prodA
// sebanyak qty, bayar tunai pas.
func saleOp(f posFixture, qty int) map[string]any {
	id := ulid.New()
	return map[string]any{
		"op":              "sale.create",
		"id":              id,
		"idempotency_key": id,
		"payload": map[string]any{
			"outlet_id": f.outletID,
			"items":     []map[string]any{{"product_id": f.prodA, "qty": fmt.Sprintf("%d", qty)}},
			"payments":  []map[string]any{{"method": "cash", "amount": qty * 15000}},
		},
	}
}

// push mengirim batch operasi ke /sync/push.
func push(t *testing.T, token string, ops ...map[string]any) map[string]any {
	t.Helper()
	return call(t, "POST", "/api/v1/sync/push", token, map[string]any{
		"device_id": "dev-" + ulid.New(), "operations": ops,
	}).mustOK(t, "sync push").data(t)
}

// TestSyncPushAppliesAndDeduplicates — DoD Fase 6: perangkat berjualan lama
// tanpa internet lalu menyinkron; kiriman yang diulang tidak menggandakan apa
// pun.
func TestSyncPushAppliesAndDeduplicates(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "syncdod") // prodA 100

	const n = 10
	ops := make([]map[string]any, n)
	for i := range ops {
		ops[i] = saleOp(f, 2) // 10 x 2 = 20 unit
	}

	first := push(t, f.token, ops...)
	assertI64(t, first, "applied", n)
	assertI64(t, first, "duplicate", 0)
	assertI64(t, first, "rejected", 0)
	for _, r := range first["results"].([]any) {
		if r.(map[string]any)["status"] != "applied" {
			t.Fatalf("hasil bukan applied: %v", r)
		}
	}
	if q := stockQty(t, f.tenantFixture, f.outletID, f.prodA); q != "80" {
		t.Fatalf("stok setelah push = %s, mau 80", q)
	}

	// Kirim ULANG batch yang sama (koneksi putus lalu perangkat mencoba lagi).
	again := push(t, f.token, ops...)
	assertI64(t, again, "applied", 0)
	assertI64(t, again, "duplicate", n)
	if q := stockQty(t, f.tenantFixture, f.outletID, f.prodA); q != "80" {
		t.Fatalf("stok setelah push ulang = %s, mau tetap 80 (tanpa duplikat)", q)
	}

	// Jumlah transaksi tetap n.
	ls := call(t, "GET", "/api/v1/sales?outlet_id="+f.outletID+"&status=completed", f.token, nil).
		mustOK(t, "list sales").data(t)
	if got := len(ls["data"].([]any)); got != n {
		t.Fatalf("jumlah transaksi = %d, mau %d", got, n)
	}
}

// TestSyncPushPartialFailure — satu operasi buruk tidak menghentikan yang lain
// (§10 aturan 1).
func TestSyncPushPartialFailure(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "syncpart")

	bad := saleOp(f, 1)
	bad["payload"].(map[string]any)["items"] = []map[string]any{
		{"product_id": ulid.New(), "qty": "1"}, // produk tak ada
	}

	res := push(t, f.token, saleOp(f, 1), bad, saleOp(f, 1))
	assertI64(t, res, "applied", 2)
	assertI64(t, res, "rejected", 1)
	rows := res["results"].([]any)
	if rows[0].(map[string]any)["status"] != "applied" ||
		rows[1].(map[string]any)["status"] != "rejected" ||
		rows[2].(map[string]any)["status"] != "applied" {
		t.Fatalf("urutan hasil salah: %v", rows)
	}
	if rows[1].(map[string]any)["reason"] == "" {
		t.Fatal("operasi rejected tanpa alasan")
	}
}

// TestSyncPushRecomputesBusinessDate — business_date dihitung ulang di server,
// jam perangkat diabaikan (§10 aturan 3).
func TestSyncPushRecomputesBusinessDate(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "syncbd")

	// business_date server "hari ini" — ambil lewat satu checkout biasa.
	today := saleDay(t, f, "BD-REF", 1)

	op := saleOp(f, 1)
	op["payload"].(map[string]any)["client_created_at"] = "2020-01-01T02:00:00+09:00" // jauh di masa lalu
	res := push(t, f.token, op)
	assertI64(t, res, "applied", 1)

	saleID := res["results"].([]any)[0].(map[string]any)["id"].(string)
	sale := call(t, "GET", "/api/v1/sales/"+saleID, f.token, nil).mustOK(t, "get synced sale").data(t)
	if sale["business_date"] != today {
		t.Fatalf("business_date = %v, mau %s (dihitung server, bukan dari client_created_at)", sale["business_date"], today)
	}
}

// TestSyncPullDeltaAndTombstone — pull hanya membawa yang lebih baru dari kursor;
// baris soft-delete muncul di `deleted`.
func TestSyncPullDeltaAndTombstone(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "syncpull") // 1 unit + 2 produk

	full := call(t, "GET", "/api/v1/sync/pull?since=0&outlet_id="+f.outletID, f.token, nil).
		mustOK(t, "pull awal").data(t)
	if len(full["products"].([]any)) < 2 || len(full["units"].([]any)) < 1 {
		t.Fatalf("pull awal kurang lengkap: %d produk, %d unit", len(full["products"].([]any)), len(full["units"].([]any)))
	}
	cursor := int64(full["cursor"].(float64))
	if cursor <= 0 {
		t.Fatalf("cursor = %d, mau > 0", cursor)
	}
	if full["has_more"].(bool) {
		t.Fatal("has_more true untuk dataset kecil")
	}

	// Produk baru → hanya itu yang muncul di pull berikutnya.
	np := call(t, "POST", "/api/v1/products", f.token, map[string]any{
		"name": "Produk Baru syncpull", "unit_id": f.unitID, "sell_price": 1000, "cost_price": 500, "track_stock": true,
	}).mustCode(t, "buat produk", 201).data(t)["id"].(string)

	delta := call(t, "GET", fmt.Sprintf("/api/v1/sync/pull?since=%d&outlet_id=%s", cursor, f.outletID), f.token, nil).
		mustOK(t, "pull delta").data(t)
	dp := delta["products"].([]any)
	if len(dp) != 1 || dp[0].(map[string]any)["id"] != np {
		t.Fatalf("pull delta: mau tepat 1 produk baru (%s), dapat %v", np, dp)
	}
	cursor2 := int64(delta["cursor"].(float64))

	// Soft-delete → id masuk ke `deleted`.
	call(t, "DELETE", "/api/v1/products/"+np, f.token, nil).mustOK(t, "hapus produk")
	afterDel := call(t, "GET", fmt.Sprintf("/api/v1/sync/pull?since=%d&outlet_id=%s", cursor2, f.outletID), f.token, nil).
		mustOK(t, "pull setelah hapus").data(t)
	if !containsStr(jsonArray(afterDel["deleted"].(map[string]any)["products"]), np) {
		t.Fatalf("produk terhapus %s tidak ada di daftar deleted: %v", np, afterDel["deleted"])
	}
}

// TestSyncPullHardDeleteTombstone — DELETE keras pun terkabar lewat sync_tombstones.
func TestSyncPullHardDeleteTombstone(t *testing.T) {
	requireDB(t)
	f := registerTenant(t, "synchard")

	cat := makeCategory(t, f, "Kategori Fana")
	cur := int64(call(t, "GET", "/api/v1/sync/pull?since=0", f.token, nil).mustOK(t, "pull").data(t)["cursor"].(float64))

	if err := database.DB.Exec("DELETE FROM categories WHERE id = ?", cat).Error; err != nil {
		t.Fatalf("hard delete: %v", err)
	}

	res := call(t, "GET", fmt.Sprintf("/api/v1/sync/pull?since=%d", cur), f.token, nil).
		mustOK(t, "pull setelah hard delete").data(t)
	del, _ := res["deleted"].(map[string]any)
	if !containsStr(jsonArray(del["categories"]), cat) {
		t.Fatalf("kategori hard-deleted %s tidak ada di batu nisan: %v", cat, res["deleted"])
	}
}

// TestSyncPullPaging — kursor maju melewati beberapa halaman kecil dan berhenti.
func TestSyncPullPaging(t *testing.T) {
	requireDB(t)
	f := registerTenant(t, "syncpage")

	made := map[string]bool{}
	for i := 0; i < 5; i++ {
		made[makeCategory(t, f, fmt.Sprintf("Kat %d", i))] = true
	}

	seen := map[string]bool{}
	var since int64
	for step := 0; step < 20; step++ {
		d := call(t, "GET", fmt.Sprintf("/api/v1/sync/pull?since=%d&limit=2", since), f.token, nil).
			mustOK(t, "pull halaman").data(t)
		for _, c := range d["categories"].([]any) {
			seen[c.(map[string]any)["id"].(string)] = true
		}
		next := int64(d["cursor"].(float64))
		if !d["has_more"].(bool) {
			since = next
			break
		}
		if next <= since {
			t.Fatalf("kursor tidak maju: %d → %d", since, next)
		}
		since = next
	}
	for id := range made {
		if !seen[id] {
			t.Fatalf("kategori %s tak terlihat setelah paging seluruh halaman", id)
		}
	}
}

// TestSyncIsolation — tenant B tak melihat data A lewat pull, dan tak bisa
// mendorong penjualan ke outlet A.
func TestSyncIsolation(t *testing.T) {
	requireDB(t)
	a := setupPOS(t, "syncisoA")
	b := setupPOS(t, "syncisoB")

	pb := call(t, "GET", "/api/v1/sync/pull?since=0", b.token, nil).mustOK(t, "pull B").data(t)
	for _, p := range pb["products"].([]any) {
		if p.(map[string]any)["id"] == a.prodA {
			t.Fatal("tenant B melihat produk tenant A lewat pull")
		}
	}

	// B mendorong penjualan yang menunjuk outlet A → ditolak.
	op := saleOp(a, 1) // payload memakai outlet & produk milik A
	res := push(t, b.token, op)
	assertI64(t, res, "rejected", 1)
	assertI64(t, res, "applied", 0)
}

// TestSyncPermission — endpoint sinkron butuh sale.create.
func TestSyncPermission(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "syncperm")

	gudang := staffToken(t, f.tenantFixture, roleID(t, f.tenantFixture, "Gudang"), "gudang_syncperm")
	call(t, "GET", "/api/v1/sync/pull?since=0", gudang, nil).mustCode(t, "gudang pull", 403)
	call(t, "POST", "/api/v1/sync/push", gudang, map[string]any{
		"device_id": "d", "operations": []map[string]any{saleOp(f, 1)},
	}).mustCode(t, "gudang push", 403)

	kasir := staffToken(t, f.tenantFixture, roleID(t, f.tenantFixture, "Kasir"), "kasir_syncperm")
	call(t, "GET", "/api/v1/sync/pull?since=0", kasir, nil).mustOK(t, "kasir pull")
}
