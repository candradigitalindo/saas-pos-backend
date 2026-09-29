package tests

import "testing"

// roleID mencari id peran bernama `name` milik tenant f.
func roleID(t *testing.T, f tenantFixture, name string) string {
	t.Helper()
	d := call(t, "GET", "/api/v1/roles?limit=50", f.token, nil).mustOK(t, "GET /roles").data(t)
	for _, ri := range d["data"].([]any) {
		if m, ok := ri.(map[string]any); ok && m["name"] == name {
			return m["id"].(string)
		}
	}
	t.Fatalf("peran %q tidak ditemukan", name)
	return ""
}

// makeUnit membuat satuan dan mengembalikan id-nya.
func makeUnit(t *testing.T, f tenantFixture, name string) string {
	t.Helper()
	d := call(t, "POST", "/api/v1/units", f.token, map[string]any{"name": name}).
		mustCode(t, "POST /units "+name, 201).data(t)
	return d["id"].(string)
}

// makeCategory membuat kategori dan mengembalikan id-nya.
func makeCategory(t *testing.T, f tenantFixture, name string) string {
	t.Helper()
	d := call(t, "POST", "/api/v1/categories", f.token, map[string]any{"name": name}).
		mustCode(t, "POST /categories "+name, 201).data(t)
	return d["id"].(string)
}

// TestMasterDataIsolation memastikan kategori/satuan/produk tenant A tak terlihat
// oleh tenant B.
func TestMasterDataIsolation(t *testing.T) {
	requireDB(t)

	a := registerTenant(t, "mdA")
	b := registerTenant(t, "mdB")

	unitA := makeUnit(t, a, "pcs")
	catA := makeCategory(t, a, "Minuman")
	prodA := call(t, "POST", "/api/v1/products", a.token, map[string]any{
		"name": "Kopi Hitam", "unit_id": unitA, "category_id": catA, "sell_price": 15000, "cost_price": 6000,
	}).mustCode(t, "POST /products A", 201).data(t)
	prodAID := prodA["id"].(string)

	// B tidak melihat data A.
	for _, path := range []string{"/api/v1/units", "/api/v1/categories", "/api/v1/products"} {
		d := call(t, "GET", path, b.token, nil).mustOK(t, "GET "+path+" B").data(t)
		if n := len(d["data"].([]any)); n != 0 {
			t.Fatalf("B melihat %d entri di %s, mau 0", n, path)
		}
	}

	// B menebak id milik A → 404.
	for _, tc := range []struct{ method, path string }{
		{"GET", "/api/v1/units/" + unitA},
		{"GET", "/api/v1/categories/" + catA},
		{"GET", "/api/v1/products/" + prodAID},
		{"PUT", "/api/v1/products/" + prodAID},
		{"DELETE", "/api/v1/products/" + prodAID},
	} {
		var payload any
		if tc.method == "PUT" {
			payload = map[string]any{"name": "diretas"}
		}
		call(t, tc.method, tc.path, b.token, payload).
			mustCode(t, tc.method+" "+tc.path+" (B menembak data A)", 404)
	}

	// A tetap utuh.
	still := call(t, "GET", "/api/v1/products/"+prodAID, a.token, nil).mustOK(t, "GET product A").data(t)
	if still["name"] != "Kopi Hitam" {
		t.Fatalf("produk A berubah: %v", still["name"])
	}
}

// TestProductCRUDAndSearch menguji siklus produk + pencarian trigram.
func TestProductCRUDAndSearch(t *testing.T) {
	requireDB(t)

	f := registerTenant(t, "prodcrud")
	unit := makeUnit(t, f, "pcs")

	names := []string{"Teh Manis", "Teh Tawar", "Kopi Susu", "Air Mineral", "Jus Alpukat"}
	for _, n := range names {
		call(t, "POST", "/api/v1/products", f.token, map[string]any{
			"name": n, "unit_id": unit, "sell_price": 10000, "cost_price": 4000,
		}).mustCode(t, "POST /products "+n, 201)
	}

	// Cari "teh" → 2 hasil.
	d := call(t, "GET", "/api/v1/products?q=teh", f.token, nil).mustOK(t, "search teh").data(t)
	items := d["data"].([]any)
	if len(items) != 2 {
		t.Fatalf("cari 'teh' → %d hasil, mau 2. %v", len(items), items)
	}
	for _, it := range items {
		m := it.(map[string]any)
		if m["unit_name"] != "pcs" {
			t.Fatalf("unit_name = %v, mau pcs (JOIN gagal?)", m["unit_name"])
		}
	}

	// Update satu produk.
	pid := items[0].(map[string]any)["id"].(string)
	upd := call(t, "PUT", "/api/v1/products/"+pid, f.token, map[string]any{
		"sell_price": 12000, "is_active": false,
	}).mustOK(t, "PUT product").data(t)
	if int64(upd["sell_price"].(float64)) != 12000 || upd["is_active"].(bool) {
		t.Fatalf("update tidak tersimpan: %v", upd)
	}

	// Filter is_active=false → minimal 1.
	da := call(t, "GET", "/api/v1/products?is_active=false", f.token, nil).mustOK(t, "filter inactive").data(t)
	if len(da["data"].([]any)) < 1 {
		t.Fatal("filter is_active=false tidak mengembalikan produk nonaktif")
	}

	// Delete → lalu 404.
	call(t, "DELETE", "/api/v1/products/"+pid, f.token, nil).mustOK(t, "DELETE product")
	call(t, "GET", "/api/v1/products/"+pid, f.token, nil).mustCode(t, "GET produk terhapus", 404)
}

// TestProductImportCSV menguji impor CSV: dry-run, impor nyata, laporan baris
// gagal, deteksi duplikat.
func TestProductImportCSV(t *testing.T) {
	requireDB(t)

	f := registerTenant(t, "import1")
	makeUnit(t, f, "pcs")
	makeUnit(t, f, "kg")
	makeCategory(t, f, "Sembako")

	csv := "name,unit,category,sku,sell_price,cost_price,min_stock\n" +
		"Beras Premium,kg,Sembako,BRS-01,13000,11000,5\n" +
		"Gula Pasir,kg,Sembako,GLA-01,15000,13000,3\n" +
		"Minyak Goreng,liter,Sembako,MYK-01,18000,16000,2\n" + // satuan 'liter' tidak ada → gagal
		"Telur,kg,Sembako,BRS-01,28000,25000,1\n" + // SKU duplikat dengan baris 1 → gagal
		"Kopi Sachet,pcs,,KPI-01,2000,1500,20\n"

	// Dry-run: 5 total, 3 valid, 2 gagal, TIDAK menyimpan.
	dry := callRaw(t, "POST", "/api/v1/products/import?dry_run=true", f.token, "text/csv", csv).
		mustOK(t, "import dry-run").data(t)
	if !dry["dry_run"].(bool) {
		t.Fatal("dry_run seharusnya true")
	}
	assertInt(t, dry, "total", 5)
	assertInt(t, dry, "imported", 3)
	assertInt(t, dry, "failed", 2)
	errs := dry["errors"].([]any)
	if len(errs) != 2 {
		t.Fatalf("errors = %d, mau 2: %v", len(errs), errs)
	}

	// Belum ada produk tersimpan.
	pre := call(t, "GET", "/api/v1/products", f.token, nil).mustOK(t, "list sebelum impor").data(t)
	if n := len(pre["data"].([]any)); n != 0 {
		t.Fatalf("dry-run seharusnya tidak menyimpan, tapi ada %d produk", n)
	}

	// Impor nyata.
	real := callRaw(t, "POST", "/api/v1/products/import", f.token, "text/csv", csv).
		mustOK(t, "import nyata").data(t)
	assertInt(t, real, "imported", 3)
	assertInt(t, real, "failed", 2)

	post := call(t, "GET", "/api/v1/products?limit=50", f.token, nil).mustOK(t, "list setelah impor").data(t)
	if n := len(post["data"].([]any)); n != 3 {
		t.Fatalf("setelah impor ada %d produk, mau 3", n)
	}

	// Impor ulang berkas yang sama → semua baris ber-SKU jadi duplikat.
	again := callRaw(t, "POST", "/api/v1/products/import", f.token, "text/csv", csv).
		mustOK(t, "import ulang").data(t)
	if again["imported"].(float64) != 0 {
		t.Fatalf("impor ulang seharusnya 0 baris (semua SKU duplikat), dapat %v", again["imported"])
	}
}

// TestMasterDataPermissions memastikan product.view bisa membaca tapi tidak
// menulis/impor.
func TestMasterDataPermissions(t *testing.T) {
	requireDB(t)

	owner := registerTenant(t, "mdperm")
	unit := makeUnit(t, owner, "pcs")
	_ = unit

	// Buat peran "Lihat Saja" hanya dengan product.view, lalu user-nya.
	roleResp := call(t, "POST", "/api/v1/roles", owner.token, map[string]any{
		"name": "Lihat Saja", "permission_codes": []string{"product.view"},
	}).mustCode(t, "POST /roles", 201).data(t)
	viewRoleID := roleResp["id"].(string)

	call(t, "POST", "/api/v1/users", owner.token, map[string]any{
		"name": "Via", "username": "via_mdperm", "email": "via_mdperm@example.com",
		"password": "rahasia123", "role_id": viewRoleID,
	}).mustCode(t, "POST /users", 201)

	login := call(t, "POST", "/api/v1/auth/login", "", map[string]any{
		"username": "via_mdperm", "password": "rahasia123",
	}).mustOK(t, "login via").data(t)
	viaToken := get[string](t, login, "access_token")

	call(t, "GET", "/api/v1/products", viaToken, nil).mustOK(t, "via GET /products")
	call(t, "POST", "/api/v1/products", viaToken, map[string]any{"name": "X", "unit_id": unit}).
		mustCode(t, "via POST /products (butuh product.edit)", 403)
	call(t, "GET", "/api/v1/categories", viaToken, nil).mustOK(t, "via GET /categories")
	call(t, "POST", "/api/v1/categories", viaToken, map[string]any{"name": "Y"}).
		mustCode(t, "via POST /categories (butuh product.edit)", 403)
	call(t, "POST", "/api/v1/products/import", viaToken, map[string]any{}).
		mustCode(t, "via import (butuh product.import)", 403)
}

func assertInt(t *testing.T, m map[string]any, key string, want int) {
	t.Helper()
	got, ok := m[key].(float64)
	if !ok || int(got) != want {
		t.Fatalf("%s = %v, mau %d", key, m[key], want)
	}
}
