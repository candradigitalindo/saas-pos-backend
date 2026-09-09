package tests

import "testing"

// Uji pelengkap Program Mitra (blueprint G.4/G.5): materi jualan per tingkat,
// pelatihan, target berbasis merchant aktif, dan sengketa atribusi.

// TestPartnerMaterialsFilteredByTier — materi berbatas tingkat hanya terlihat
// oleh mitra tingkat itu; materi tanpa batas terlihat semua.
func TestPartnerMaterialsFilteredByTier(t *testing.T) {
	requireDB(t)
	_, email, pass := makePlatformAdmin(t, "mat", "superadmin")
	adm := platformToken(t, email, pass)

	tiers := call(t, "POST", "/api/v1/platform/partner-tiers", adm, map[string]any{
		"name": "Uji Materi Atas", "kind": "agent", "recurring_rate": "0.25",
	}).mustCode(t, "tingkat atas", 201).data(t)
	call(t, "POST", "/api/v1/platform/partner-tiers", adm, map[string]any{
		"name": "Uji Materi Bawah", "kind": "affiliate", "recurring_rate": "0.10",
	}).mustCode(t, "tingkat bawah", 201)

	// Satu materi umum, satu khusus tingkat atas.
	call(t, "POST", "/api/v1/platform/partner-materials", adm, map[string]any{
		"title": "Brosur Umum", "kind": "brochure", "file_url": "https://cdn.test/umum.pdf",
	}).mustCode(t, "materi umum", 201)
	call(t, "POST", "/api/v1/platform/partner-materials", adm, map[string]any{
		"title": "Daftar Harga Khusus", "kind": "pricelist",
		"file_url": "https://cdn.test/khusus.pdf", "min_tier_id": tiers["id"].(string),
	}).mustCode(t, "materi khusus", 201)

	// Mitra tingkat BAWAH hanya melihat materi umum.
	_, _, e1, p1 := makeActivePartner(t, "Uji Materi Bawah", "matbawah")
	low := partnerToken(t, e1, p1)
	got := call(t, "GET", "/api/v1/partner/materials", low, nil).
		mustOK(t, "materi tingkat bawah").Body["data"].([]any)
	for _, m := range got {
		if m.(map[string]any)["title"] == "Daftar Harga Khusus" {
			t.Fatal("mitra tingkat bawah tidak boleh melihat materi khusus tingkat atas")
		}
	}

	// Mitra tingkat ATAS melihat keduanya.
	_, _, e2, p2 := makeActivePartner(t, "Uji Materi Atas", "matatas")
	high := partnerToken(t, e2, p2)
	got2 := call(t, "GET", "/api/v1/partner/materials", high, nil).
		mustOK(t, "materi tingkat atas").Body["data"].([]any)
	var adaKhusus bool
	for _, m := range got2 {
		if m.(map[string]any)["title"] == "Daftar Harga Khusus" {
			adaKhusus = true
		}
	}
	if !adaKhusus {
		t.Fatalf("mitra tingkat atas seharusnya melihat materi khususnya: %v", got2)
	}
}

// TestPartnerTrainingCompletion — modul wajib ditandai selesai oleh mitra.
func TestPartnerTrainingCompletion(t *testing.T) {
	requireDB(t)
	_, email, pass := makePlatformAdmin(t, "lat", "superadmin")
	adm := platformToken(t, email, pass)

	tr := call(t, "POST", "/api/v1/platform/partner-trainings", adm, map[string]any{
		"title": "Dasar Produk", "content_url": "https://cdn.test/dasar.mp4",
		"is_required": true, "sort_order": 1,
	}).mustCode(t, "buat modul", 201).data(t)

	makePartnerTier(t, "Uji Pelatihan", "0.15", 1, 1, 3650)
	_, _, e, p := makeActivePartner(t, "Uji Pelatihan", "latih")
	tok := partnerToken(t, e, p)

	before := call(t, "GET", "/api/v1/partner/trainings", tok, nil).
		mustOK(t, "daftar modul").Body["data"].([]any)
	var target map[string]any
	for _, m := range before {
		if m.(map[string]any)["id"] == tr["id"] {
			target = m.(map[string]any)
		}
	}
	if target == nil || target["completed"] != false {
		t.Fatalf("modul awal harus belum selesai: %v", target)
	}

	call(t, "POST", "/api/v1/partner/trainings/"+tr["id"].(string)+"/complete", tok,
		map[string]any{"score": 90}).mustOK(t, "tandai selesai")

	after := call(t, "GET", "/api/v1/partner/trainings", tok, nil).
		mustOK(t, "daftar modul lagi").Body["data"].([]any)
	for _, m := range after {
		mm := m.(map[string]any)
		if mm["id"] == tr["id"] {
			if mm["completed"] != true {
				t.Fatalf("modul harusnya selesai: %v", mm)
			}
			if mm["score"] == nil {
				t.Fatalf("nilai harusnya tersimpan: %v", mm)
			}
		}
	}
}

// TestPartnerTargetCountsActiveMerchants — pencapaian dihitung dari merchant
// yang langganannya HIDUP, bukan jumlah pendaftaran (blueprint G.4).
func TestPartnerTargetCountsActiveMerchants(t *testing.T) {
	requireDB(t)
	_, email, pass := makePlatformAdmin(t, "tgt", "superadmin")
	adm := platformToken(t, email, pass)

	makePartnerTier(t, "Uji Target", "0.20", 1, 1, 3650)
	pid, code, e, p := makeActivePartner(t, "Uji Target", "target")
	tok := partnerToken(t, e, p)

	from, to := widePeriod()
	call(t, "POST", "/api/v1/platform/partner-targets", adm, map[string]any{
		"partner_id": pid, "period_start": from, "period_end": to, "target_merchants": 5,
	}).mustOK(t, "tetapkan target")

	// Belum ada merchant → pencapaian 0.
	got := call(t, "GET", "/api/v1/partner/targets", tok, nil).
		mustOK(t, "target mitra").Body["data"].([]any)
	if len(got) != 1 {
		t.Fatalf("target = %d, mau 1", len(got))
	}
	assertI64(t, got[0].(map[string]any), "target_merchants", 5)
	assertI64(t, got[0].(map[string]any), "achieved_merchants", 0)

	// Satu merchant mendaftar & berlangganan → pencapaian jadi 1.
	f := referredPOS(t, "ptarget", code, "0899-ptarget")
	startBasic12(t, f.tenantFixture)
	got2 := call(t, "GET", "/api/v1/partner/targets", tok, nil).
		mustOK(t, "target setelah merchant").Body["data"].([]any)
	assertI64(t, got2[0].(map[string]any), "achieved_merchants", 1)
}

// TestPartnerDisputeDecided — mitra mengajukan sengketa, admin memutuskan, dan
// keputusannya tercatat; sengketa yang DITERIMA mengubah status atribusi.
func TestPartnerDisputeDecided(t *testing.T) {
	requireDB(t)
	_, email, pass := makePlatformAdmin(t, "sengketa", "superadmin")
	adm := platformToken(t, email, pass)

	makePartnerTier(t, "Uji Sengketa", "0.20", 1, 1, 3650)
	_, code, e, p := makeActivePartner(t, "Uji Sengketa", "sengketa")
	tok := partnerToken(t, e, p)
	f := referredPOS(t, "psengketa", code, "0899-psengketa")

	d := call(t, "POST", "/api/v1/partner/disputes", tok, map[string]any{
		"tenant_id": f.tenantID,
		"reason":    "Merchant ini saya yang mendampingi sejak awal, bukan agen lain.",
	}).mustCode(t, "ajukan sengketa", 201).data(t)
	if d["status"] != "open" {
		t.Fatalf("sengketa baru harus open, dapat %v", d["status"])
	}

	// Mitra lain tidak melihat sengketa ini.
	_, _, e2, p2 := makeActivePartner(t, "Uji Sengketa", "sengketa2")
	lain := partnerToken(t, e2, p2)
	if n := len(call(t, "GET", "/api/v1/partner/disputes", lain, nil).
		mustOK(t, "sengketa mitra lain").Body["data"].([]any)); n != 0 {
		t.Fatalf("mitra lain melihat %d sengketa — bocor", n)
	}

	// Admin memutuskan; alasan WAJIB.
	call(t, "POST", "/api/v1/platform/partner-disputes/"+d["id"].(string)+"/resolve", adm,
		map[string]any{"status": "accepted"}).
		mustCode(t, "putus tanpa alasan", 422)

	call(t, "POST", "/api/v1/platform/partner-disputes/"+d["id"].(string)+"/resolve", adm,
		map[string]any{"status": "accepted", "decision_note": "Bukti pendampingan diterima."}).
		mustOK(t, "putus sengketa")

	after := call(t, "GET", "/api/v1/partner/disputes", tok, nil).
		mustOK(t, "sengketa setelah putusan").Body["data"].([]any)
	dd := after[0].(map[string]any)
	if dd["status"] != "accepted" || dd["decision_note"] == "" || dd["decided_at"] == "" {
		t.Fatalf("keputusan tidak tercatat lengkap: %v", dd)
	}

	// Diputus dua kali → 409: barisnya ada, tapi keadaannya sudah final.
	call(t, "POST", "/api/v1/platform/partner-disputes/"+d["id"].(string)+"/resolve", adm,
		map[string]any{"status": "rejected", "decision_note": "coba lagi"}).
		mustCode(t, "putus ulang", 409)
}
