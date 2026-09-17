package tests

import (
	"fmt"
	"testing"
	"time"

	"candra/backend-api/internal/ulid"
)

// Uji integrasi Fase 10 — CRM sales lapangan (§5.9, blueprint E.3).

// fieldSalesRole membuat peran sales lapangan lengkap dan mengembalikan id-nya.
func fieldSalesRole(t *testing.T, f tenantFixture) string {
	t.Helper()
	return call(t, "POST", "/api/v1/roles", f.token, map[string]any{
		"name": "Sales Lapangan " + ulid.New()[:6],
		"permission_codes": []string{
			"crm.visit.checkin", "sale.create", "sale.discount",
			"customer.view", "customer.edit",
			"shift.open", "shift.close", "cash.movement", "receivable.manage",
		},
	}).mustCode(t, "buat peran sales", 201).data(t)["id"].(string)
}

// visitOp membangun satu operasi 'visit.upsert' untuk /sync/push.
func visitOp(visitID, customerID string, order bool) map[string]any {
	now := time.Now().UTC()
	payload := map[string]any{
		"id":          visitID,
		"customer_id": customerID,
		"checkin_at":  now.Add(-30 * time.Minute).Format(time.RFC3339),
		"checkout_at": now.Format(time.RFC3339),
		"checkin_lat": "-6.200000",
		"checkin_lng": "106.816666",
		"photo_url":   "https://cdn/toko.jpg",
		"result":      "no_order",
	}
	if order {
		payload["result"] = "closed"
	} else {
		payload["no_order_reason"] = "stok masih penuh"
	}
	return map[string]any{"op": "visit.upsert", "id": visitID, "payload": payload}
}

// TestFieldSalesRouteSyncNoDuplicates — DoD Fase 10: rute 20 toko tanpa sinyal
// lalu tersinkron utuh tanpa duplikat.
func TestFieldSalesRouteSyncNoDuplicates(t *testing.T) {
	requireDB(t)
	f := registerTenant(t, "fsdod")
	sales := staffToken(t, f, fieldSalesRole(t, f), "sales_fsdod")

	const n = 20
	custIDs := make([]string, n)
	for i := range custIDs {
		custIDs[i] = makeCustomer(t, sales, fmt.Sprintf("Toko %02d", i))
	}

	plan := call(t, "POST", "/api/v1/visit-plans", sales, map[string]any{
		"plan_date":    time.Now().UTC().Format("2006-01-02"),
		"customer_ids": custIDs,
	}).mustCode(t, "buat rencana", 201).data(t)
	planVisits := plan["visits"].([]any)
	if len(planVisits) != n {
		t.Fatalf("rencana punya %d kunjungan, mau %d", len(planVisits), n)
	}

	// Rute selesai offline → dorong 20 kunjungan sekaligus.
	ops := make([]map[string]any, n)
	for i, pv := range planVisits {
		m := pv.(map[string]any)
		ops[i] = visitOp(m["id"].(string), m["customer_id"].(string), i == 0)
	}
	first := push(t, sales, ops...)
	assertI64(t, first, "applied", n)

	// Sinyal putus lalu perangkat mencoba lagi — kirim ulang batch yang sama.
	push(t, sales, ops...)

	// Tetap 20 kunjungan, tanpa duplikat, semua sudah check-out.
	//
	// Tanggalnya diambil dari server, bukan dihitung sendiri dari UTC: jalur
	// sinkronisasi menulis ulang business_date memakai zona waktu outlet, jadi
	// tanggal UTC bisa meleset satu hari antara tengah malam dan pagi.
	semua := call(t, "GET", "/api/v1/visits?limit=100", sales, nil).
		mustOK(t, "daftar kunjungan tanpa saringan").data(t)["data"].([]any)
	if len(semua) == 0 {
		t.Fatal("tidak ada kunjungan tersimpan")
	}
	bd := semua[0].(map[string]any)["business_date"].(string)
	list := call(t, "GET", "/api/v1/visits?business_date="+bd+"&limit=100", sales, nil).
		mustOK(t, "daftar kunjungan").data(t)
	rows := list["data"].([]any)
	if len(rows) != n {
		t.Fatalf("jumlah kunjungan = %d, mau %d (push ulang tidak boleh menggandakan)", len(rows), n)
	}
	done := 0
	for _, r := range rows {
		if r.(map[string]any)["checkout_at"] != nil {
			done++
		}
	}
	if done != n {
		t.Fatalf("%d kunjungan ter-check-out, mau %d", done, n)
	}
}

// TestVisitCheckinCheckoutFlow — jalur API: check-in (GPS+foto) lalu check-out.
func TestVisitCheckinCheckoutFlow(t *testing.T) {
	requireDB(t)
	f := registerTenant(t, "fsflow")
	sales := staffToken(t, f, fieldSalesRole(t, f), "sales_fsflow")
	cust := makeCustomer(t, sales, "Toko Flow")

	v := call(t, "POST", "/api/v1/visits", sales, map[string]any{
		"customer_id": cust,
		"checkin_at":  time.Now().UTC().Format(time.RFC3339),
		"checkin_lat": "-6.2", "checkin_lng": "106.8",
		"photo_url": "https://cdn/x.jpg",
	}).mustOK(t, "check-in").data(t)
	vID := v["id"].(string)
	if v["result"] != "pending" || v["checkin_lat"] == nil {
		t.Fatalf("check-in salah: %v", v)
	}

	// no_order tanpa alasan → 422.
	call(t, "POST", "/api/v1/visits/"+vID+"/checkout", sales, map[string]any{"result": "no_order"}).
		mustCode(t, "checkout tanpa alasan", 422)

	out := call(t, "POST", "/api/v1/visits/"+vID+"/checkout", sales, map[string]any{
		"result": "no_order", "no_order_reason": "toko tutup",
	}).mustOK(t, "check-out").data(t)
	if out["result"] != "no_order" || out["checkout_at"] == nil {
		t.Fatalf("check-out salah: %v", out)
	}

	// Check-out kedua → 409.
	call(t, "POST", "/api/v1/visits/"+vID+"/checkout", sales, map[string]any{
		"result": "closed",
	}).mustCode(t, "check-out kedua", 409)
}

// TestVisitOwnerVisibility — sales tak melihat kunjungan sales lain; pengawas melihat semua.
func TestVisitOwnerVisibility(t *testing.T) {
	requireDB(t)
	f := registerTenant(t, "fsvis")
	role := fieldSalesRole(t, f)
	x := staffToken(t, f, role, "sales_x_fsvis")
	y := staffToken(t, f, role, "sales_y_fsvis")

	custX := makeCustomer(t, x, "Toko X")
	vX := call(t, "POST", "/api/v1/visits", x, map[string]any{"customer_id": custX}).
		mustOK(t, "visit X").data(t)["id"].(string)

	call(t, "GET", "/api/v1/visits/"+vX, y, nil).mustCode(t, "Y baca visit X", 404)
	if n := len(call(t, "GET", "/api/v1/visits", y, nil).mustOK(t, "visits Y").data(t)["data"].([]any)); n != 0 {
		t.Fatalf("sales Y melihat %d kunjungan sales X", n)
	}
	// Pemilik (crm.lead.view.all) melihat.
	call(t, "GET", "/api/v1/visits/"+vX, f.token, nil).mustOK(t, "owner baca visit X")
}

// TestCommissionOnCollectedNotShipped — dasar komisi = nilai TERTAGIH
// (pembayaran non-kredit + setoran piutang), bukan terkirim.
func TestCommissionOnCollectedNotShipped(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "fscomm") // outlet + 2 produk + shift terbuka
	sales := staffToken(t, f.tenantFixture, fieldSalesRole(t, f.tenantFixture), "sales_fscomm")

	// Sales bertransaksi memakai shift terbuka outlet — created_by = sales.
	cust := call(t, "POST", "/api/v1/customers", sales, map[string]any{
		"name": "Toko Kredit", "phone": "0812fscomm", "credit_limit": 1000000,
	}).mustCode(t, "pelanggan", 201).data(t)["id"].(string)

	// 1. Penjualan tunai penuh: 2 * prodA (15000) = 30000, semua tertagih.
	checkout(t, sales, "FC-1", map[string]any{
		"outlet_id": f.outletID,
		"items":     []map[string]any{{"product_id": f.prodA, "qty": "2"}},
		"payments":  []map[string]any{{"method": "cash", "amount": 30000}},
	}).mustCode(t, "jual tunai", 201)

	// 2. Penjualan campur: 4 * prodA = 60000; tunai 20000 + kredit 40000.
	sale2 := checkout(t, sales, "FC-2", map[string]any{
		"outlet_id":   f.outletID,
		"customer_id": cust,
		"items":       []map[string]any{{"product_id": f.prodA, "qty": "4"}},
		"payments": []map[string]any{
			{"method": "cash", "amount": 20000},
			{"method": "credit", "amount": 40000},
		},
	}).mustCode(t, "jual campur", 201).data(t)
	bd := sale2["business_date"].(string)

	// 3. Setor piutang 15000 di lapangan (collected_by = sales).
	recID := call(t, "GET", "/api/v1/receivables?customer_id="+cust, sales, nil).
		mustOK(t, "piutang").data(t)["data"].([]any)[0].(map[string]any)["id"].(string)
	call(t, "POST", "/api/v1/receivable-payments", sales, map[string]any{
		"receivable_id": recID, "amount": 15000, "method": "cash",
	}).mustCode(t, "setor piutang", 201)

	// Tertagih = 30000 (tunai) + 20000 (tunai) + 15000 (setoran) = 65000.
	// Kredit 40000 yang belum disetor (sisa 25000) TIDAK dihitung.
	salesUserID := call(t, "GET", "/api/v1/me", sales, nil).mustOK(t, "me").data(t)["user"].(map[string]any)["id"].(string)

	com := call(t, "POST", "/api/v1/commissions", f.token, map[string]any{
		"user_id": salesUserID, "period_start": bd, "period_end": bd, "rate": "0.05",
	}).mustOK(t, "hitung komisi").data(t)
	assertI64(t, com, "base_amount", 65000)
	assertI64(t, com, "amount", 3250) // 65000 * 5%
	if com["status"] != "draft" {
		t.Fatalf("status komisi = %v, mau draft", com["status"])
	}
	comID := com["id"].(string)

	call(t, "POST", "/api/v1/commissions/"+comID+"/approve", f.token, nil).mustOK(t, "setujui komisi")
	// Hitung ulang setelah disetujui → 409.
	call(t, "POST", "/api/v1/commissions", f.token, map[string]any{
		"user_id": salesUserID, "period_start": bd, "period_end": bd, "rate": "0.1",
	}).mustCode(t, "hitung ulang setelah setuju", 409)
	paid := call(t, "POST", "/api/v1/commissions/"+comID+"/pay", f.token, nil).mustOK(t, "bayar komisi").data(t)
	if paid["status"] != "paid" {
		t.Fatalf("status komisi = %v, mau paid", paid["status"])
	}
}

// TestSalesTargetAchievement — target + pencapaian (kunjungan selesai & tertagih).
func TestSalesTargetAchievement(t *testing.T) {
	requireDB(t)
	f := registerTenant(t, "fstarget")
	sales := staffToken(t, f, fieldSalesRole(t, f), "sales_fstarget")
	salesUserID := call(t, "GET", "/api/v1/me", sales, nil).mustOK(t, "me").data(t)["user"].(map[string]any)["id"].(string)

	// Dua kunjungan selesai.
	var bd string
	for i := 0; i < 2; i++ {
		cust := makeCustomer(t, sales, fmt.Sprintf("Toko T%d", i))
		v := call(t, "POST", "/api/v1/visits", sales, map[string]any{
			"customer_id": cust, "checkin_at": time.Now().UTC().Format(time.RFC3339),
		}).mustOK(t, "check-in").data(t)
		bd = v["business_date"].(string)
		call(t, "POST", "/api/v1/visits/"+v["id"].(string)+"/checkout", sales, map[string]any{"result": "closed"}).
			mustOK(t, "check-out")
	}

	// Periode target memakai HARI USAHA dari server, bukan tanggal UTC yang
	// dihitung sendiri: keduanya berbeda setiap hari antara tengah malam dan
	// pagi di zona Indonesia, dan tes ini pernah gagal persis karena itu.
	today := bd
	call(t, "POST", "/api/v1/sales-targets", f.token, map[string]any{
		"user_id": salesUserID, "period_start": today, "period_end": today,
		"target_amount": 1000000, "target_visits": 10,
	}).mustOK(t, "set target")

	got := call(t, "GET", "/api/v1/sales-targets?user_id="+salesUserID, f.token, nil).
		mustOK(t, "lihat target").data(t)["data"].([]any)[0].(map[string]any)
	assertI64(t, got, "achieved_visits", 2)
	assertI64(t, got, "target_visits", 10)
}

// TestFieldSalesPermission — kunjungan butuh crm.visit.checkin; target & komisi butuh crm.commission.view.
func TestFieldSalesPermission(t *testing.T) {
	requireDB(t)
	f := registerTenant(t, "fsperm")
	kasir := staffToken(t, f, roleID(t, f, "Kasir"), "kasir_fsperm")

	call(t, "GET", "/api/v1/visits", kasir, nil).mustCode(t, "kasir lihat kunjungan", 403)
	call(t, "POST", "/api/v1/visit-plans", kasir, map[string]any{
		"plan_date": "2026-12-01", "customer_ids": []string{ulid.New()},
	}).mustCode(t, "kasir buat rencana", 403)
	call(t, "GET", "/api/v1/sales-targets", kasir, nil).mustCode(t, "kasir lihat target", 403)
	call(t, "POST", "/api/v1/commissions", kasir, map[string]any{
		"user_id": ulid.New(), "period_start": "2026-01-01", "period_end": "2026-01-31", "rate": "0.05",
	}).mustCode(t, "kasir hitung komisi", 403)
}

// TestFieldSalesIsolation — tenant B tak melihat kunjungan tenant A.
func TestFieldSalesIsolation(t *testing.T) {
	requireDB(t)
	a := registerTenant(t, "fsisoA")
	b := registerTenant(t, "fsisoB")
	sa := staffToken(t, a, fieldSalesRole(t, a), "sales_fsisoA")

	custA := makeCustomer(t, sa, "Toko A")
	vA := call(t, "POST", "/api/v1/visits", sa, map[string]any{"customer_id": custA}).
		mustOK(t, "visit A").data(t)["id"].(string)

	call(t, "GET", "/api/v1/visits/"+vA, b.token, nil).mustCode(t, "B baca visit A", 404)
	if n := len(call(t, "GET", "/api/v1/visits", b.token, nil).mustOK(t, "visits B").data(t)["data"].([]any)); n != 0 {
		t.Fatalf("tenant B melihat %d kunjungan tenant A", n)
	}
}

// TestReceivablePaymentUsesOutletBusinessDate — setoran piutang harus memakai
// hari usaha OUTLET, bukan tanggal UTC.
//
// Sebelumnya controller mengisi `business_date` dengan `time.Now().UTC()`. Di
// Indonesia (UTC+7..+9) tanggal UTC tertinggal dari tanggal setempat setiap
// hari antara tengah malam dan pagi, jadi setoran pukul 06.00 WIB tercatat
// sebagai hari kemarin — dan komisi sales, yang dihitung dari business_date,
// ikut hilang.
//
// Tes ini TIDAK bergantung pada jam berapa ia dijalankan: jam tutup buku outlet
// digeser ke satu jam setelah waktu setempat sekarang, sehingga "sekarang"
// dipastikan masih masuk hari usaha SEBELUMNYA. Dengan begitu tanggal UTC dan
// hari usaha outlet dijamin berbeda, kapan pun tes ini jalan.
func TestReceivablePaymentUsesOutletBusinessDate(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "recbizdate")

	// Geser jam tutup buku ke 1 jam setelah waktu Jakarta sekarang.
	jkt, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		t.Fatalf("zona waktu: %v", err)
	}
	dayStart := time.Now().In(jkt).Add(time.Hour).Format("15:04")
	call(t, "PUT", "/api/v1/outlets/"+f.outletID, f.token, map[string]any{
		"timezone": "Asia/Jakarta", "business_day_start": dayStart,
	}).mustOK(t, "geser jam tutup buku")

	cust := call(t, "POST", "/api/v1/customers", f.token, map[string]any{
		"name": "Toko Kasbon", "phone": "0812recbiz", "credit_limit": 1000000,
	}).mustCode(t, "pelanggan", 201).data(t)["id"].(string)

	// Penjualan kasbon: 2 × prodA (15000) = 30000, seluruhnya kredit.
	sale := checkout(t, f.token, "RB-1", map[string]any{
		"outlet_id":   f.outletID,
		"customer_id": cust,
		"items":       []map[string]any{{"product_id": f.prodA, "qty": "2"}},
		"payments":    []map[string]any{{"method": "credit", "amount": 30000}},
	}).mustCode(t, "jual kasbon", 201).data(t)
	bd := sale["business_date"].(string)

	// Hari usaha outlet memang BEDA dari tanggal UTC — kalau tidak, tes ini
	// tidak membuktikan apa pun.
	if utc := time.Now().UTC().Format("2006-01-02"); bd == utc {
		t.Fatalf("persiapan gagal: business_date (%s) sama dengan tanggal UTC (%s)", bd, utc)
	}

	// Setor piutangnya.
	recID := call(t, "GET", "/api/v1/receivables?customer_id="+cust, f.token, nil).
		mustOK(t, "piutang").data(t)["data"].([]any)[0].(map[string]any)["id"].(string)
	call(t, "POST", "/api/v1/receivable-payments", f.token, map[string]any{
		"receivable_id": recID, "amount": 12000, "method": "cash",
	}).mustCode(t, "setor piutang", 201)

	// Komisi pada hari usaha penjualannya HARUS memuat setoran tadi. Bila
	// business_date setoran memakai tanggal UTC, ia jatuh di luar rentang dan
	// base_amount-nya jadi 0.
	me := call(t, "GET", "/api/v1/me", f.token, nil).mustOK(t, "me").data(t)
	uid := me["user"].(map[string]any)["id"].(string)

	com := call(t, "POST", "/api/v1/commissions", f.token, map[string]any{
		"user_id": uid, "period_start": bd, "period_end": bd, "rate": "0.1",
	}).mustOK(t, "hitung komisi").data(t)
	assertI64(t, com, "base_amount", 12000)
}
