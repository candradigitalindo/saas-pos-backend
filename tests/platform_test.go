package tests

import (
	"testing"

	"candra/backend-api/database"
	"candra/backend-api/services"
	"candra/backend-api/structs"
)

// Uji panel internal penyedia SaaS (blueprint G.5, migrasi 000035) — realm
// KETIGA, terpisah dari tenant dan mitra.

// makePlatformAdmin membuat akun staf internal dan mengembalikan (id, email, password).
func makePlatformAdmin(t *testing.T, slug, role string) (string, string, string) {
	t.Helper()
	email := "admin_" + slug + "@platform.test"
	res, err := services.CreatePlatformAdmin(bg(), structs.PlatformAdminCreateRequest{
		Name: "Admin " + slug, Email: email, Role: role, Password: "rahasiaplatform123",
	})
	if err != nil {
		t.Fatalf("buat admin %s: %v", slug, err)
	}
	return res.Admin.ID, email, "rahasiaplatform123"
}

func platformToken(t *testing.T, email, password string) string {
	t.Helper()
	d := call(t, "POST", "/api/v1/platform/auth/login", "", map[string]any{
		"email": email, "password": password,
	}).mustOK(t, "login panel "+email).data(t)
	return get[string](t, d, "access_token")
}

// TestPlatformRealmIsolated — token panel ditolak di rute tenant & portal mitra,
// dan sebaliknya. Ini batas yang sama pentingnya dengan batas mitra (G.8).
func TestPlatformRealmIsolated(t *testing.T) {
	requireDB(t)
	_, email, pass := makePlatformAdmin(t, "iso", "superadmin")
	tok := platformToken(t, email, pass)

	f := registerTenant(t, "pfiso")
	makePartnerTier(t, "Uji Platform Iso", "0.10", 1, 1, 3650)
	_, _, pEmail, pPass := makeActivePartner(t, "Uji Platform Iso", "pfiso")
	pTok := partnerToken(t, pEmail, pPass)

	// Token panel ditolak di rute tenant.
	for _, path := range []string{"/api/v1/me", "/api/v1/products", "/api/v1/sales?status=completed"} {
		call(t, "GET", path, tok, nil).mustCode(t, "panel di rute tenant "+path, 401)
	}
	// Token panel ditolak di portal mitra.
	call(t, "GET", "/api/v1/partner/me", tok, nil).mustCode(t, "panel di portal mitra", 401)

	// Token tenant & mitra ditolak di panel.
	call(t, "GET", "/api/v1/platform/me", f.token, nil).mustCode(t, "tenant di panel", 401)
	call(t, "GET", "/api/v1/platform/me", pTok, nil).mustCode(t, "mitra di panel", 401)

	// Admin sendiri bisa masuk panel.
	me := call(t, "GET", "/api/v1/platform/me", tok, nil).mustOK(t, "admin lihat profil").data(t)
	if me["role"] != "superadmin" {
		t.Fatalf("role = %v, mau superadmin", me["role"])
	}
}

// TestPlatformRolesSeparateDuties — yang memverifikasi mitra bukan yang
// mencairkan uangnya (blueprint G.5).
func TestPlatformRolesSeparateDuties(t *testing.T) {
	requireDB(t)
	_, opEmail, opPass := makePlatformAdmin(t, "op", "operator")
	_, fiEmail, fiPass := makePlatformAdmin(t, "fi", "finance")
	_, suEmail, suPass := makePlatformAdmin(t, "su", "support")
	op, fi, su := platformToken(t, opEmail, opPass), platformToken(t, fiEmail, fiPass), platformToken(t, suEmail, suPass)

	// operator boleh verifikasi mitra, TIDAK boleh menyentuh uang.
	call(t, "GET", "/api/v1/platform/partners", op, nil).mustOK(t, "operator lihat mitra")
	call(t, "POST", "/api/v1/platform/partner-payouts", op, map[string]any{
		"partner_id": "01M22NYJF9PG3ZMFN817GWMZXY", "period_start": "2026-09-01", "period_end": "2026-09-30",
	}).mustCode(t, "operator mencairkan uang", 403)

	// finance boleh urusan uang, TIDAK boleh menyetujui mitra.
	call(t, "POST", "/api/v1/platform/partners/01M22NYJF9PG3ZMFN817GWMZXY/approve", fi, nil).
		mustCode(t, "finance menyetujui mitra", 403)

	// support hanya membaca.
	call(t, "GET", "/api/v1/platform/partner-tiers", su, nil).mustOK(t, "support membaca")
	call(t, "POST", "/api/v1/platform/partner-tiers", su, map[string]any{
		"name": "Coba", "kind": "agent", "recurring_rate": "0.1",
	}).mustCode(t, "support menulis", 403)

	// Kelola akun admin hanya superadmin.
	call(t, "GET", "/api/v1/platform/admins", op, nil).mustCode(t, "operator kelola admin", 403)
}

// TestPlatformPartnerLifecycle — mitra dibuat, diverifikasi, lalu dicairkan
// komisinya SELURUHNYA lewat panel, tanpa CLI.
func TestPlatformPartnerLifecycle(t *testing.T) {
	requireDB(t)
	_, email, pass := makePlatformAdmin(t, "cycle", "superadmin")
	tok := platformToken(t, email, pass)

	tier := call(t, "POST", "/api/v1/platform/partner-tiers", tok, map[string]any{
		"name": "Uji Panel", "kind": "agent", "recurring_rate": "0.20",
		"activation_min_txn": 1, "activation_min_days": 1,
		"attribution_days": 3650, "clawback_days": 3650,
	}).mustCode(t, "buat tingkat", 201).data(t)
	if tier["name"] != "Uji Panel" {
		t.Fatalf("tingkat = %v", tier)
	}

	created := call(t, "POST", "/api/v1/platform/partners", tok, map[string]any{
		"tier_name": "Uji Panel", "name": "Agen Panel", "phone": "0811-panel",
		"user_name": "Panel", "user_email": "agen_panel@mitra.test",
		"user_password": "rahasiamitra123",
	}).mustCode(t, "buat mitra", 201).data(t)
	partner := created["partner"].(map[string]any)
	pid, code := partner["id"].(string), partner["referral_code"].(string)
	if partner["status"] != "pending" {
		t.Fatalf("mitra baru harus pending, dapat %v", partner["status"])
	}

	// Belum disetujui → mitra belum bisa masuk portalnya.
	call(t, "POST", "/api/v1/partner/auth/login", "", map[string]any{
		"email": "agen_panel@mitra.test", "password": "rahasiamitra123",
	}).mustCode(t, "mitra pending masuk portal", 403)

	call(t, "POST", "/api/v1/platform/partners/"+pid+"/approve", tok, nil).
		mustOK(t, "setujui mitra")

	// Merchant mendaftar pakai kode, bertransaksi, bayar langganan.
	f := referredPOS(t, "pfcycle", code, "0899-pfcycle")
	sellOnce(t, f, "pfcycle-1")
	inv := startBasic12(t, f.tenantFixture)
	paySub(t, f.token, "PAY-pfcycle", inv["id"].(string), 789684).mustCode(t, "bayar langganan", 201)

	// Jalankan komisi dari panel — bukan dari CLI.
	from, to := widePeriod()
	run := call(t, "POST", "/api/v1/platform/partner-commissions/run", tok, map[string]any{
		"partner_id": pid, "period_start": from, "period_end": to,
	}).mustOK(t, "jalankan komisi").data(t)
	assertI64(t, run, "computed", 1)

	// Setujui lalu cairkan.
	pTok := partnerToken(t, "agen_panel@mitra.test", "rahasiamitra123")
	comms := call(t, "GET", "/api/v1/partner/commissions", pTok, nil).
		mustOK(t, "komisi mitra").Body["data"].([]any)
	cid := comms[0].(map[string]any)["id"].(string)
	call(t, "POST", "/api/v1/platform/partner-commissions/"+cid+"/approve", tok, nil).
		mustOK(t, "setujui komisi")

	payout := call(t, "POST", "/api/v1/platform/partner-payouts", tok, map[string]any{
		"partner_id": pid, "period_start": from, "period_end": to,
	}).mustCode(t, "buat pencairan", 201).data(t)
	assertI64(t, payout, "gross_amount", 157937)
	if payout["status"] != "draft" {
		t.Fatalf("pencairan baru harus draft, dapat %v", payout["status"])
	}

	call(t, "POST", "/api/v1/platform/partner-payouts/"+payout["id"].(string)+"/paid", tok, map[string]any{
		"transfer_proof_url": "https://bukti.test/tf-1.jpg",
	}).mustOK(t, "tandai sudah dibayar")

	// Tindakan admin tercatat di jejak audit dengan actor_type='admin'.
	n, err := repositoriesCountAudit(t, "admin", "partner.approve")
	if err != nil || n < 1 {
		t.Fatalf("tindakan admin tidak tercatat di audit_logs (n=%d err=%v)", n, err)
	}
}

// TestPlatformLastSuperadminProtected — superadmin aktif terakhir tidak boleh
// dinonaktifkan; kalau bisa, panel internal terkunci total.
func TestPlatformLastSuperadminProtected(t *testing.T) {
	requireDB(t)
	id, email, pass := makePlatformAdmin(t, "last", "superadmin")
	tok := platformToken(t, email, pass)

	// Sisakan HANYA admin ini sebagai superadmin aktif. Dinonaktifkan langsung
	// lewat database karena service-nya justru menolak mematikan yang terakhir —
	// itulah yang sedang diuji.
	if err := database.DB.Table("platform_admins").
		Where("role = 'superadmin' AND id <> ?", id).
		Update("is_active", false).Error; err != nil {
		t.Fatalf("siapkan kondisi: %v", err)
	}

	call(t, "PUT", "/api/v1/platform/admins/"+id+"/active", tok, map[string]any{"is_active": false}).
		mustCode(t, "matikan superadmin terakhir", 409)

	// Masih bisa dipakai — tidak terkunci.
	call(t, "GET", "/api/v1/platform/me", tok, nil).mustOK(t, "panel masih terbuka")
}

// repositoriesCountAudit menghitung baris audit_logs untuk pengujian.
func repositoriesCountAudit(t *testing.T, actorType, action string) (int64, error) {
	t.Helper()
	var n int64
	err := database.DB.Table("audit_logs").
		Where("actor_type = ? AND action = ?", actorType, action).Count(&n).Error
	return n, err
}
