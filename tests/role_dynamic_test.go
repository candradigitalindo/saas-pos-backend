package tests

import "testing"

// Uji peran dinamis per tenant: sistem menyiapkan peran bawaan saat pendaftaran,
// dan tenant bebas menambah/mengubah/menghapus peran sesuai kebutuhannya —
// dengan satu batas yang tidak boleh dilanggar: peran bawaan pemilik harus tetap
// bisa mengelola peran, supaya tenant tidak pernah bisa mengunci dirinya sendiri.

// roleByName mencari peran tenant berdasarkan nama.
func roleByName(t *testing.T, f tenantFixture, name string) map[string]any {
	t.Helper()
	d := call(t, "GET", "/api/v1/roles?limit=50", f.token, nil).mustOK(t, "daftar peran").data(t)
	for _, ri := range d["data"].([]any) {
		if m, ok := ri.(map[string]any); ok && m["name"] == name {
			return m
		}
	}
	t.Fatalf("peran %q tidak ditemukan", name)
	return nil
}

// TestTenantDefaultRolesSeeded — pendaftaran membuat empat peran bawaan, dan
// hanya peran pemilik yang ditandai bawaan sistem.
func TestTenantDefaultRolesSeeded(t *testing.T) {
	requireDB(t)
	f := registerTenant(t, "roledef")

	for _, name := range []string{"Pemilik", "Manajer", "Kasir", "Gudang"} {
		r := roleByName(t, f, name)
		if name == "Pemilik" && r["is_system"] != true {
			t.Fatalf("peran Pemilik seharusnya is_system=true, dapat %v", r["is_system"])
		}
		if name != "Pemilik" && r["is_system"] != false {
			t.Fatalf("peran %s seharusnya is_system=false, dapat %v", name, r["is_system"])
		}
	}

	// Pemilik memegang SELURUH permission katalog.
	owner := roleByName(t, f, "Pemilik")
	detail := call(t, "GET", "/api/v1/roles/"+owner["id"].(string), f.token, nil).
		mustOK(t, "detail Pemilik").data(t)
	katalog := call(t, "GET", "/api/v1/permissions", f.token, nil).
		mustOK(t, "katalog permission").Body["data"].([]any)
	if got, want := len(detail["permission_codes"].([]any)), len(katalog); got != want {
		t.Fatalf("permission Pemilik = %d, mau %d (seluruh katalog)", got, want)
	}
}

// TestTenantCustomRoleLifecycle — tenant menambah peran sendiri, mengatur
// izinnya, memakainya untuk staf, lalu menghapusnya setelah tak dipakai.
func TestTenantCustomRoleLifecycle(t *testing.T) {
	requireDB(t)
	f := registerTenant(t, "rolecustom")

	// 1. Buat peran khusus "Supervisor Gudang".
	created := call(t, "POST", "/api/v1/roles", f.token, map[string]any{
		"name":             "Supervisor Gudang",
		"description":      "Pantau stok, tanpa akses kasir",
		"permission_codes": []string{"stock.view", "stock.opname", "product.view"},
	}).mustCode(t, "buat peran", 201).data(t)
	rid := created["id"].(string)
	if created["is_system"] != false {
		t.Fatalf("peran buatan tenant tidak boleh is_system")
	}

	// 2. Ubah izinnya (ganti seluruh pemetaan).
	call(t, "PUT", "/api/v1/roles/"+rid+"/permissions", f.token, map[string]any{
		"permission_codes": []string{"stock.view", "stock.adjust", "report.view"},
	}).mustOK(t, "ganti izin peran")
	after := call(t, "GET", "/api/v1/roles/"+rid, f.token, nil).mustOK(t, "detail peran").data(t)
	codes := jsonArray(after["permission_codes"])
	if len(codes) != 3 {
		t.Fatalf("izin setelah diganti = %v, mau 3 kode", codes)
	}

	// 3. Pakai untuk staf → izin baru benar-benar berlaku.
	staf := staffToken(t, f, rid, "spv_rolecustom")
	call(t, "GET", "/api/v1/stocks?outlet_id="+f.outletID, staf, nil).
		mustOK(t, "staf boleh lihat stok")
	call(t, "POST", "/api/v1/sales", staf, map[string]any{}).
		mustCode(t, "staf TIDAK boleh kasir", 403)

	// 4. Peran yang masih dipakai tidak boleh dihapus.
	call(t, "DELETE", "/api/v1/roles/"+rid, f.token, nil).
		mustCode(t, "hapus peran yang dipakai", 409)

	// 5. Pindahkan staf ke peran lain, baru peran boleh dihapus.
	kasir := roleByName(t, f, "Kasir")["id"].(string)
	users := call(t, "GET", "/api/v1/users?limit=50", f.token, nil).mustOK(t, "daftar user").data(t)
	var stafID string
	for _, ui := range users["data"].([]any) {
		if m := ui.(map[string]any); m["username"] == "spv_rolecustom" {
			stafID = m["id"].(string)
		}
	}
	if stafID == "" {
		t.Fatal("user staf tidak ditemukan")
	}
	call(t, "PUT", "/api/v1/users/"+stafID, f.token, map[string]any{"role_id": kasir}).
		mustOK(t, "pindah peran staf")
	call(t, "DELETE", "/api/v1/roles/"+rid, f.token, nil).
		mustOK(t, "hapus peran yang sudah kosong")
}

// TestOwnerRoleCannotLockItselfOut — batas yang menjaga tenant dari mengunci
// dirinya sendiri: peran bawaan pemilik tidak boleh kehilangan `role.manage`
// (tanpa itu tak ada seorang pun yang bisa mengembalikan izin apa pun), tidak
// boleh diganti nama, dan tidak boleh dihapus.
func TestOwnerRoleCannotLockItselfOut(t *testing.T) {
	requireDB(t)
	f := registerTenant(t, "rolelock")
	owner := roleByName(t, f, "Pemilik")
	rid := owner["id"].(string)

	// Mengosongkan izin peran pemilik harus DITOLAK.
	call(t, "PUT", "/api/v1/roles/"+rid+"/permissions", f.token, map[string]any{
		"permission_codes": []string{"product.view"},
	}).mustCode(t, "kosongkan izin Pemilik", 422)

	// Ganti nama & hapus peran bawaan juga ditolak.
	call(t, "PUT", "/api/v1/roles/"+rid, f.token, map[string]any{"name": "Bos"}).
		mustCode(t, "ganti nama peran bawaan", 400)
	call(t, "DELETE", "/api/v1/roles/"+rid, f.token, nil).
		mustCode(t, "hapus peran bawaan", 409)

	// Pemilik masih bisa mengelola peran — tidak terkunci.
	call(t, "GET", "/api/v1/roles", f.token, nil).mustOK(t, "pemilik masih bisa kelola peran")

	// Menambah izin ke peran bawaan tetap boleh (selama role.manage dipertahankan).
	all := call(t, "GET", "/api/v1/permissions", f.token, nil).
		mustOK(t, "katalog").Body["data"].([]any)
	codes := make([]string, 0, len(all))
	for _, p := range all {
		codes = append(codes, p.(map[string]any)["code"].(string))
	}
	call(t, "PUT", "/api/v1/roles/"+rid+"/permissions", f.token, map[string]any{
		"permission_codes": codes,
	}).mustOK(t, "set ulang seluruh izin Pemilik")
}
