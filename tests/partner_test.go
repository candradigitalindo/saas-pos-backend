package tests

import (
	"context"
	"testing"
	"time"

	"candra/backend-api/database"
	"candra/backend-api/internal/ulid"
	"candra/backend-api/services"
	"candra/backend-api/structs"
)

// Uji integrasi Fase 12 — Program Mitra Penjual (§16, blueprint Bagian G).
//
// DoD: (1) siklus komisi berjalan tanpa hitungan manual; (2) akun mitra TIDAK
// bisa menyentuh data operasional tenant mana pun.
//
// Panel internal (buat/verifikasi mitra, jalankan komisi) belum punya realm
// admin platform → dijalankan lewat fungsi service langsung di sini (sepadan
// dengan cmd/partner-admin & cmd/partner-commissions).

func bg() context.Context { return context.Background() }

// makePartnerTier membuat tingkat mitra dengan ambang yang mudah diuji.
func makePartnerTier(t *testing.T, code, rate string, minTxn, minDays, clawbackDays int) {
	t.Helper()
	recurring := true
	_, err := services.CreatePartnerTier(bg(), structs.PartnerTierRequest{
		Code: code, Name: "Tier " + code, Kind: "agen", CommissionRate: rate,
		Recurring: &recurring, ActivationMinTxn: minTxn, ActivationMinDays: minDays,
		AttributionDays: 3650, ClawbackDays: clawbackDays,
	})
	if err != nil {
		t.Fatalf("buat tier %s: %v", code, err)
	}
}

// makeActivePartner membuat mitra + akun login, lalu menyetujuinya (→ active).
// Mengembalikan (partnerID, referralCode, username, password).
func makeActivePartner(t *testing.T, tierCode, slug string) (string, string, string, string) {
	t.Helper()
	res, err := services.CreatePartner(bg(), structs.PartnerCreateRequest{
		TierCode: tierCode, Name: "Mitra " + slug,
		UserName: "Mitra " + slug, UserEmail: "mitra_" + slug + "@example.com",
		UserUsername: "mitra_" + slug, UserPassword: "rahasiamitra123",
	})
	if err != nil {
		t.Fatalf("buat mitra %s: %v", slug, err)
	}
	if err := services.ApprovePartner(bg(), res.Partner.ID); err != nil {
		t.Fatalf("setujui mitra %s: %v", slug, err)
	}
	return res.Partner.ID, res.Partner.ReferralCode, res.UserUsername, "rahasiamitra123"
}

// partnerToken login ke portal mitra dan mengembalikan access token realm partner.
func partnerToken(t *testing.T, username, password string) string {
	t.Helper()
	d := call(t, "POST", "/api/v1/partner/auth/login", "", map[string]any{
		"username": username, "password": password,
	}).mustOK(t, "login mitra "+username).data(t)
	return get[string](t, d, "access_token")
}

// registerReferredTenant mendaftarkan tenant dengan kode referral + nomor telepon
// tertentu.
func registerReferredTenant(t *testing.T, slug, referralCode, phone string) tenantFixture {
	t.Helper()
	payload := map[string]any{
		"business_name": "Usaha " + slug,
		"business_type": "retail",
		"phone":         phone,
		"outlet_name":   "Outlet " + slug,
		"timezone":      "Asia/Jakarta",
		"referral_code": referralCode,
		"owner": map[string]any{
			"name": "Owner " + slug, "username": "owner_" + slug,
			"email": "owner_" + slug + "@example.com", "password": "rahasia123",
		},
	}
	d := call(t, "POST", "/api/v1/auth/register", "", payload).
		mustCode(t, "register "+slug, 201).data(t)
	return tenantFixture{
		token:    get[string](t, d, "auth.access_token"),
		tenantID: get[string](t, d, "tenant.id"),
		outletID: get[string](t, d, "outlet.id"),
		ownerID:  get[string](t, d, "auth.user.id"),
	}
}

// referredPOS = tenant ber-referral + unit + prodA berstok + shift terbuka.
func referredPOS(t *testing.T, slug, referralCode, phone string) posFixture {
	t.Helper()
	f := registerReferredTenant(t, slug, referralCode, phone)
	unit := makeUnit(t, f, "pcs")
	prodA := call(t, "POST", "/api/v1/products", f.token, map[string]any{
		"name": "Produk A " + slug, "unit_id": unit, "sell_price": 15000, "cost_price": 6000, "track_stock": true,
	}).mustCode(t, "buat produk", 201).data(t)["id"].(string)
	pf := posFixture{tenantFixture: f, unitID: unit, prodA: prodA}
	adjust(t, f, pf.outletID, prodA, "1000", "saldo awal")
	sh := call(t, "POST", "/api/v1/shifts/open", f.token, map[string]any{
		"outlet_id": pf.outletID, "opening_cash": 100000,
	}).mustCode(t, "buka shift", 201).data(t)
	pf.shiftID = sh["id"].(string)
	return pf
}

func sellOnce(t *testing.T, f posFixture, key string) {
	t.Helper()
	checkout(t, f.token, key, map[string]any{
		"outlet_id": f.outletID,
		"items":     []map[string]any{{"product_id": f.prodA, "qty": "1"}},
		"payments":  []map[string]any{{"method": "cash", "amount": 15000}},
	}).mustCode(t, "checkout "+key, 201)
}

func widePeriod() (string, string) {
	now := time.Now().UTC()
	return now.AddDate(0, 0, -2).Format("2006-01-02"), now.AddDate(0, 0, 2).Format("2006-01-02")
}

// TestPartnerCommissionCycle — DoD Fase 12: dari referral → aktivasi → faktur
// langganan dibayar → komisi terhitung otomatis → disetujui → dicairkan, tanpa
// satu pun hitungan manual, dan hitung ulang tetap idempoten.
func TestPartnerCommissionCycle(t *testing.T) {
	requireDB(t)
	makePartnerTier(t, "tcyc", "0.20", 1, 1, 3650) // komisi 20%, aktif pada 1 transaksi
	pid, code, user, pass := makeActivePartner(t, "tcyc", "cyc")

	f := referredPOS(t, "pcyc", code, "0899-pcyc")
	sellOnce(t, f, "pcyc-1") // penuhi ambang aktivasi

	inv := startBasic12(t, f.tenantFixture) // total 789684
	invID := inv["id"].(string)
	paySub(t, f.token, "PAY-"+ulid.New(), invID, 789684).mustCode(t, "bayar langganan", 201)

	from, to := widePeriod()
	run, err := services.ComputePartnerCommissions(bg(), pid, from, to)
	if err != nil {
		t.Fatalf("hitung komisi: %v", err)
	}
	if run.Computed != 1 || run.Activated != 1 {
		t.Fatalf("run = %+v, mau computed=1 activated=1", run)
	}

	tok := partnerToken(t, user, pass)
	comms := call(t, "GET", "/api/v1/partner/commissions", tok, nil).
		mustOK(t, "komisi mitra").Body["data"].([]any)
	if len(comms) != 1 {
		t.Fatalf("komisi = %d, mau 1", len(comms))
	}
	c := comms[0].(map[string]any)
	assertI64(t, c, "base_amount", 789684)
	assertI64(t, c, "amount", 157937) // round(789684 * 0.20)
	if c["status"] != "held" {
		t.Fatalf("status komisi = %v, mau held", c["status"])
	}

	// Setujui + cairkan (panel internal / CLI).
	n, err := services.ApproveHeldCommissionsForPartner(bg(), pid, from, to)
	if err != nil || n != 1 {
		t.Fatalf("setujui: n=%d err=%v", n, err)
	}
	payout, err := services.CreatePartnerPayout(bg(), pid, from, to)
	if err != nil {
		t.Fatalf("cairkan: %v", err)
	}
	if payout.GrossAmount != 157937 || payout.NetAmount != 157937 {
		t.Fatalf("pencairan = %+v, mau gross/net 157937 (pajak 0)", payout)
	}

	payouts := call(t, "GET", "/api/v1/partner/payouts", tok, nil).
		mustOK(t, "pencairan mitra").Body["data"].([]any)
	if len(payouts) != 1 {
		t.Fatalf("pencairan mitra = %d, mau 1", len(payouts))
	}

	// Hitung ulang → komisi tetap SATU dan tidak kembali ke 'held'.
	if _, err := services.ComputePartnerCommissions(bg(), pid, from, to); err != nil {
		t.Fatalf("hitung ulang: %v", err)
	}
	paid := call(t, "GET", "/api/v1/partner/commissions?status=paid", tok, nil).
		mustOK(t, "komisi paid").Body["data"].([]any)
	if len(paid) != 1 {
		t.Fatalf("komisi paid setelah hitung ulang = %d, mau 1", len(paid))
	}
}

// TestPartnerActivationThreshold — komisi tidak dihitung sebelum merchant
// benar-benar dipakai (blueprint G.2 #3).
func TestPartnerActivationThreshold(t *testing.T) {
	requireDB(t)
	makePartnerTier(t, "tact", "0.10", 5, 3650, 3650) // butuh 5 transaksi
	pid, code, user, pass := makeActivePartner(t, "tact", "act")

	f := referredPOS(t, "pact", code, "0899-pact")
	inv := startBasic12(t, f.tenantFixture)
	paySub(t, f.token, "PAY-"+ulid.New(), inv["id"].(string), 789684).mustCode(t, "bayar", 201)

	for i := 0; i < 2; i++ { // < 5 → belum aktif
		sellOnce(t, f, "pact-a-"+ulid.New())
	}
	from, to := widePeriod()
	run, err := services.ComputePartnerCommissions(bg(), pid, from, to)
	if err != nil {
		t.Fatalf("hitung: %v", err)
	}
	if run.Computed != 0 || run.SkippedNoActivation < 1 {
		t.Fatalf("run = %+v, mau computed=0 skipped>=1", run)
	}
	tok := partnerToken(t, user, pass)
	if n := len(call(t, "GET", "/api/v1/partner/commissions", tok, nil).mustOK(t, "komisi").Body["data"].([]any)); n != 0 {
		t.Fatalf("komisi sebelum aktivasi = %d, mau 0", n)
	}

	for i := 0; i < 3; i++ { // total 5 → aktif
		sellOnce(t, f, "pact-b-"+ulid.New())
	}
	run2, err := services.ComputePartnerCommissions(bg(), pid, from, to)
	if err != nil {
		t.Fatalf("hitung lagi: %v", err)
	}
	if run2.Activated != 1 || run2.Computed != 1 {
		t.Fatalf("run2 = %+v, mau activated=1 computed=1", run2)
	}
}

// TestPartnerCannotTouchTenantData — DoD Fase 12: akun mitra ditolak di SEMUA
// rute tenant; token tenant ditolak di rute mitra; mitra hanya melihat status
// merchant (bukan omzet/produk/pelanggan), dan aksesnya tercatat di jejak audit.
func TestPartnerCannotTouchTenantData(t *testing.T) {
	requireDB(t)
	makePartnerTier(t, "tiso", "0.10", 1, 1, 3650)
	pid, code, user, pass := makeActivePartner(t, "tiso", "iso")
	f := referredPOS(t, "piso", code, "0899-piso")
	tok := partnerToken(t, user, pass)

	// Token mitra ditolak di rute tenant.
	for _, path := range []string{
		"/api/v1/me", "/api/v1/products", "/api/v1/sales?status=completed",
		"/api/v1/reports/dashboard", "/api/v1/customers", "/api/v1/stocks?outlet_id=" + f.outletID,
	} {
		call(t, "GET", path, tok, nil).mustCode(t, "mitra "+path, 401)
	}
	// Token tenant ditolak di portal mitra.
	for _, path := range []string{"/api/v1/partner/me", "/api/v1/partner/merchants", "/api/v1/partner/commissions"} {
		call(t, "GET", path, f.token, nil).mustCode(t, "tenant "+path, 401)
	}

	// Mitra HANYA melihat status merchant — tak ada field operasional.
	merchants := call(t, "GET", "/api/v1/partner/merchants", tok, nil).
		mustOK(t, "merchant binaan").Body["data"].([]any)
	if len(merchants) != 1 {
		t.Fatalf("merchant binaan = %d, mau 1", len(merchants))
	}
	m := merchants[0].(map[string]any)
	if m["business_name"] == "" || m["tenant_id"] == "" {
		t.Fatalf("baris merchant kurang identitas: %v", m)
	}
	for _, forbidden := range []string{"omzet", "revenue", "products", "customers", "sales", "gross_profit", "total"} {
		if _, leaked := m[forbidden]; leaked {
			t.Fatalf("baris merchant membocorkan field %q: %v", forbidden, m)
		}
	}

	// Akses tercatat di jejak audit.
	var n int64
	database.DB.Table("partner_merchant_access_log").
		Where("partner_id = ? AND tenant_id = ?", pid, f.tenantID).Count(&n)
	if n < 1 {
		t.Fatalf("akses merchant tidak tercatat di jejak audit (n=%d)", n)
	}
}

// TestPartnerReferralWiring — kode referral valid → kaitan mitra↔tenant + prospek
// yang cocok jadi 'daftar'; kode tak dikenal tidak menggagalkan pendaftaran.
func TestPartnerReferralWiring(t *testing.T) {
	requireDB(t)
	makePartnerTier(t, "twire", "0.10", 1, 1, 3650)
	_, code, user, pass := makeActivePartner(t, "twire", "wire")
	tok := partnerToken(t, user, pass)

	phone := "0899-wire-lead"
	call(t, "POST", "/api/v1/partner/leads", tok, map[string]any{
		"business_name": "Calon Toko", "contact_phone": phone,
	}).mustCode(t, "catat prospek", 201)

	registerReferredTenant(t, "pwire", code, phone)

	merchants := call(t, "GET", "/api/v1/partner/merchants", tok, nil).
		mustOK(t, "merchant").Body["data"].([]any)
	if len(merchants) != 1 {
		t.Fatalf("merchant binaan = %d, mau 1", len(merchants))
	}
	daftar := call(t, "GET", "/api/v1/partner/leads?status=daftar", tok, nil).
		mustOK(t, "prospek daftar").Body["data"].([]any)
	if len(daftar) != 1 {
		t.Fatalf("prospek 'daftar' = %d, mau 1", len(daftar))
	}

	// Kode tak dikenal → pendaftaran tetap sukses, tak ada merchant baru.
	registerReferredTenant(t, "pwire-bad", "KODE-NGACO-XYZ", "0899-bad")
	merchants2 := call(t, "GET", "/api/v1/partner/merchants", tok, nil).
		mustOK(t, "merchant lagi").Body["data"].([]any)
	if len(merchants2) != 1 {
		t.Fatalf("merchant binaan setelah kode ngaco = %d, mau tetap 1", len(merchants2))
	}
}

// TestPartnerClawback — langganan berhenti dalam masa clawback → komisi ditarik.
func TestPartnerClawback(t *testing.T) {
	requireDB(t)
	makePartnerTier(t, "tclaw", "0.20", 1, 1, 3650) // masa clawback sangat lebar
	pid, code, user, pass := makeActivePartner(t, "tclaw", "claw")

	f := referredPOS(t, "pclaw", code, "0899-pclaw")
	sellOnce(t, f, "pclaw-1")
	inv := startBasic12(t, f.tenantFixture)
	paySub(t, f.token, "PAY-"+ulid.New(), inv["id"].(string), 789684).mustCode(t, "bayar", 201)

	from, to := widePeriod()
	if run, err := services.ComputePartnerCommissions(bg(), pid, from, to); err != nil || run.Computed != 1 {
		t.Fatalf("hitung awal: run=%+v err=%v", run, err)
	}

	call(t, "POST", "/api/v1/subscription/cancel", f.token, map[string]any{
		"reason": "tutup usaha",
	}).mustOK(t, "batalkan langganan")

	run2, err := services.ComputePartnerCommissions(bg(), pid, from, to)
	if err != nil {
		t.Fatalf("hitung ulang: %v", err)
	}
	if run2.ClawedBack != 1 {
		t.Fatalf("run2 = %+v, mau clawed_back=1", run2)
	}
	tok := partnerToken(t, user, pass)
	clawed := call(t, "GET", "/api/v1/partner/commissions?status=clawed_back", tok, nil).
		mustOK(t, "komisi clawed_back").Body["data"].([]any)
	if len(clawed) != 1 {
		t.Fatalf("komisi clawed_back = %d, mau 1", len(clawed))
	}
}
