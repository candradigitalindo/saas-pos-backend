package tests

import "testing"

// tenantFixture menampung hasil pendaftaran satu tenant untuk dipakai test.
type tenantFixture struct {
	token    string
	refresh  string
	tenantID string
	outletID string
	ownerID  string
}

// registerTenant mendaftarkan satu usaha baru lewat endpoint publik dan
// mengembalikan token + id entitas yang lahir. slug harus unik per test.
// registerTenant mendaftarkan tenant uji DENGAN masa coba paket Multi-Outlet —
// semua fitur berbayar terbuka. Sejak kunci paket ditegakkan, tenant tanpa
// langganan jatuh ke paket Gratis (tanpa QRIS, kanal online, CRM, cabang
// kedua); tes fitur-fitur itu menguji fiturnya, bukan paketnya, jadi mereka
// mendapat paket penuh. Tes paket & langganan memakai registerTenantPolos.
func registerTenant(t *testing.T, slug string) tenantFixture {
	t.Helper()
	f := registerTenantPolos(t, slug)
	call(t, "POST", "/api/v1/subscription", f.token, map[string]any{
		"plan_code": "multi", "term_months": 1,
	}).mustCode(t, "masa coba multi "+slug, 201)
	return f
}

// registerTenantPolos mendaftarkan tenant uji TANPA langganan (paket Gratis).
func registerTenantPolos(t *testing.T, slug string) tenantFixture {
	t.Helper()
	payload := map[string]any{
		"business_name": "Usaha " + slug,
		"business_type": "retail",
		"phone":         "08110000" + slug,
		"outlet_name":   "Outlet " + slug,
		"timezone":      "Asia/Jakarta",
		"owner": map[string]any{
			"name":     "Owner " + slug,
			"username": "owner_" + slug,
			"email":    "owner_" + slug + "@example.com",
			"password": "rahasia123",
		},
	}
	d := call(t, "POST", "/api/v1/auth/register", "", payload).
		mustCode(t, "register "+slug, 201).data(t)

	return tenantFixture{
		token:    get[string](t, d, "auth.access_token"),
		refresh:  get[string](t, d, "auth.refresh_token"),
		tenantID: get[string](t, d, "tenant.id"),
		outletID: get[string](t, d, "outlet.id"),
		ownerID:  get[string](t, d, "auth.user.id"),
	}
}

// TestTenantIsolation adalah test yang diwajibkan docs/TECHNICAL-BACKEND.md §14:
// tenant A tidak boleh membaca ATAU mengubah data tenant B di endpoint mana pun,
// termasuk dengan menebak id di URL.
func TestTenantIsolation(t *testing.T) {
	requireDB(t)

	a := registerTenant(t, "isoA")
	b := registerTenant(t, "isoB")

	if a.tenantID == b.tenantID {
		t.Fatal("dua pendaftaran menghasilkan tenant_id yang sama")
	}

	// --- /me hanya mengembalikan tenant sendiri ---
	meA := call(t, "GET", "/api/v1/me", a.token, nil).mustOK(t, "GET /me A").data(t)
	if got := get[string](t, meA, "tenant.id"); got != a.tenantID {
		t.Fatalf("/me A: tenant.id = %s, mau %s", got, a.tenantID)
	}
	if got := get[string](t, meA, "user.id"); got != a.ownerID {
		t.Fatalf("/me A: user.id = %s, mau %s", got, a.ownerID)
	}

	// --- daftar outlet: A tidak melihat outlet B ---
	listA := call(t, "GET", "/api/v1/outlets", a.token, nil).mustOK(t, "GET /outlets A").data(t)
	items, _ := listA["data"].([]any)
	if len(items) != 1 {
		t.Fatalf("A melihat %d outlet, mau 1 (miliknya sendiri). body: %v", len(items), listA)
	}
	first, _ := items[0].(map[string]any)
	if id, _ := first["id"].(string); id != a.outletID {
		t.Fatalf("outlet yang terlihat A = %s, mau %s", id, a.outletID)
	}

	// --- menebak id milik B: baca/ubah/hapus semuanya 404 ---
	for _, tc := range []struct{ method, path string }{
		{"GET", "/api/v1/outlets/" + b.outletID},
		{"PUT", "/api/v1/outlets/" + b.outletID},
		{"DELETE", "/api/v1/outlets/" + b.outletID},
		{"GET", "/api/v1/users/" + b.ownerID},
		{"PUT", "/api/v1/users/" + b.ownerID},
		{"DELETE", "/api/v1/users/" + b.ownerID},
	} {
		var payload any
		if tc.method == "PUT" {
			payload = map[string]any{"name": "diretas"}
		}
		call(t, tc.method, tc.path, a.token, payload).
			mustCode(t, tc.method+" "+tc.path+" (A menembak data B)", 404)
	}

	// --- role B tidak terlihat oleh A ---
	rolesB := call(t, "GET", "/api/v1/roles", b.token, nil).mustOK(t, "GET /roles B").data(t)
	rItems, _ := rolesB["data"].([]any)
	if len(rItems) == 0 {
		t.Fatal("B tidak punya role sama sekali")
	}
	someRoleB, _ := rItems[0].(map[string]any)
	roleBID, _ := someRoleB["id"].(string)
	call(t, "GET", "/api/v1/roles/"+roleBID, a.token, nil).
		mustCode(t, "A membaca role B", 404)

	// --- B tetap utuh setelah semua percobaan A ---
	meB := call(t, "GET", "/api/v1/me", b.token, nil).mustOK(t, "GET /me B").data(t)
	if got := get[string](t, meB, "tenant.id"); got != b.tenantID {
		t.Fatalf("/me B berubah: tenant.id = %s, mau %s", got, b.tenantID)
	}
	usersB := call(t, "GET", "/api/v1/users", b.token, nil).mustOK(t, "GET /users B").data(t)
	uItems, _ := usersB["data"].([]any)
	if len(uItems) != 1 {
		t.Fatalf("jumlah user B = %d, mau 1 (percobaan A seharusnya tidak menambah/menghapus)", len(uItems))
	}

	// --- tanpa / dengan token rusak: 401 ---
	call(t, "GET", "/api/v1/me", "", nil).mustCode(t, "GET /me tanpa token", 401)
	call(t, "GET", "/api/v1/me", "token.tidak.valid", nil).mustCode(t, "GET /me token rusak", 401)
}

// TestRegistrationCreatesDefaults memverifikasi pendaftaran membuat peran bawaan
// lengkap dan menautkan pemilik ke outlet pertama.
func TestRegistrationCreatesDefaults(t *testing.T) {
	requireDB(t)

	f := registerTenant(t, "defaults")

	me := call(t, "GET", "/api/v1/me", f.token, nil).mustOK(t, "GET /me").data(t)

	// Pemilik memegang SEMUA permission katalog.
	perms := jsonArray(me["permissions"])
	if len(perms) < 38 {
		t.Fatalf("pemilik punya %d permission, mau ≥ 38 (seluruh katalog)", len(perms))
	}
	for _, need := range []string{"user.manage", "role.manage", "outlet.manage", "sale.void", "report.profit"} {
		if !containsStr(perms, need) {
			t.Fatalf("permission %q tidak dimiliki pemilik", need)
		}
	}

	// Pemilik tertaut ke outlet pertama.
	outletIDs := jsonArray(me["outlet_ids"])
	if len(outletIDs) != 1 || outletIDs[0] != f.outletID {
		t.Fatalf("outlet_ids pemilik = %v, mau [%s]", outletIDs, f.outletID)
	}

	// Empat peran bawaan ada, dengan nama yang benar.
	roles := call(t, "GET", "/api/v1/roles?limit=50", f.token, nil).mustOK(t, "GET /roles").data(t)
	rItems, _ := roles["data"].([]any)
	names := map[string]bool{}
	for _, ri := range rItems {
		if m, ok := ri.(map[string]any); ok {
			names[m["name"].(string)] = true
		}
	}
	for _, want := range []string{"Pemilik", "Manajer", "Kasir", "Gudang"} {
		if !names[want] {
			t.Fatalf("peran bawaan %q tidak dibuat. yang ada: %v", want, names)
		}
	}

	// Role pemilik = Pemilik, dan is_system.
	if got := get[string](t, me, "user.role_name"); got != "Pemilik" {
		t.Fatalf("role pemilik = %q, mau Pemilik", got)
	}
}

// TestPermissionEnforcement memverifikasi Require() menolak aksi yang tidak
// diizinkan peran, dan mengizinkan yang diizinkan.
func TestPermissionEnforcement(t *testing.T) {
	requireDB(t)

	owner := registerTenant(t, "permzz")

	// Ambil id role "Kasir" milik tenant ini.
	roles := call(t, "GET", "/api/v1/roles?limit=50", owner.token, nil).mustOK(t, "GET /roles").data(t)
	rItems, _ := roles["data"].([]any)
	var kasirRoleID string
	for _, ri := range rItems {
		if m, ok := ri.(map[string]any); ok && m["name"] == "Kasir" {
			kasirRoleID = m["id"].(string)
		}
	}
	if kasirRoleID == "" {
		t.Fatal("role Kasir tidak ditemukan")
	}

	// Owner membuat user staf berperan Kasir.
	staff := call(t, "POST", "/api/v1/users", owner.token, map[string]any{
		"name":     "Kasir Satu",
		"username": "kasir_permzz",
		"email":    "kasir_permzz@example.com",
		"password": "rahasia123",
		"role_id":  kasirRoleID,
	}).mustCode(t, "POST /users kasir", 201)
	_ = staff

	// Kasir login.
	login := call(t, "POST", "/api/v1/auth/login", "", map[string]any{
		"username": "kasir_permzz",
		"password": "rahasia123",
	}).mustOK(t, "login kasir").data(t)
	kasirToken := get[string](t, login, "access_token")

	// Kasir BOLEH: /me.
	call(t, "GET", "/api/v1/me", kasirToken, nil).mustOK(t, "kasir GET /me")

	// Kasir TIDAK BOLEH: kelola user, kelola role, kelola outlet.
	call(t, "GET", "/api/v1/users", kasirToken, nil).
		mustCode(t, "kasir GET /users (butuh user.manage)", 403)
	call(t, "GET", "/api/v1/roles", kasirToken, nil).
		mustCode(t, "kasir GET /roles (butuh role.manage)", 403)
	call(t, "POST", "/api/v1/outlets", kasirToken, map[string]any{"name": "cabang gelap"}).
		mustCode(t, "kasir POST /outlets (butuh outlet.manage)", 403)
}

// TestRegisterValidation memverifikasi input pendaftaran yang buruk ditolak
// (422) tanpa menyentuh database.
func TestRegisterValidation(t *testing.T) {
	requireDB(t)

	// business_type di luar enum.
	call(t, "POST", "/api/v1/auth/register", "", map[string]any{
		"business_name": "X",
		"business_type": "kelontong", // tidak valid
		"phone":         "0811999",
		"outlet_name":   "Y",
		"owner": map[string]any{
			"name": "Z", "username": "z_val", "email": "z@example.com", "password": "rahasia123",
		},
	}).mustCode(t, "register business_type invalid", 422)

	// password terlalu pendek.
	call(t, "POST", "/api/v1/auth/register", "", map[string]any{
		"business_name": "X2",
		"business_type": "retail",
		"phone":         "0811999",
		"outlet_name":   "Y2",
		"owner": map[string]any{
			"name": "Z2", "username": "z2_val", "email": "z2@example.com", "password": "pendek",
		},
	}).mustCode(t, "register password pendek", 422)

	// username duplikat → 409 (setelah satu pendaftaran sukses).
	registerTenant(t, "dupuser")
	call(t, "POST", "/api/v1/auth/register", "", map[string]any{
		"business_name": "Lain",
		"business_type": "fnb",
		"phone":         "0822000",
		"outlet_name":   "Lain-1",
		"owner": map[string]any{
			"name": "Kembar", "username": "owner_dupuser", "email": "beda@example.com", "password": "rahasia123",
		},
	}).mustCode(t, "register username duplikat", 409)
}

func containsStr(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
