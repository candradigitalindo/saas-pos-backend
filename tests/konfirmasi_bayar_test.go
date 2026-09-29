package tests

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"candra/backend-api/database"
	"candra/backend-api/helpers"
	"candra/backend-api/internal/reqctx"
	"candra/backend-api/internal/ulid"
	"candra/backend-api/services"
)

// Uji konfirmasi pembayaran langganan (migrasi 000039).
//
// Dulu tenant sendiri yang mencatat pembayaran langganannya (POST
// /subscription-payments) dan paket langsung aktif — sejak kunci paket
// ditegakkan, itu pintu memakai paket berbayar tanpa membayar. Kini tenant
// hanya MENGONFIRMASI; staf keuangan platform yang menyetujui.

// tagihanTerbuka: tenant polos → masa coba Basic → masa coba habis → tagihan.
func tagihanTerbuka(t *testing.T, slug string) (tenantFixture, string, int64) {
	t.Helper()
	f := registerTenantPolos(t, slug)
	// Belum berlangganan = 404, bukan galat lain: halaman Langganan membedakan
	// keduanya (404 → "Anda memakai paket Gratis"; selain itu → galat + coba lagi).
	call(t, "GET", "/api/v1/subscription", f.token, nil).mustCode(t, "ringkasan tanpa langganan", 404)
	call(t, "POST", "/api/v1/subscription", f.token, map[string]any{
		"plan_code": "basic", "term_months": 1,
	}).mustCode(t, "mulai Basic", 201)
	lewat := time.Now().UTC().Add(-time.Hour)
	aturLangganan(t, f.tenantID, map[string]any{"trial_ends_at": lewat, "current_period_end": lewat})
	inv := call(t, "POST", "/api/v1/subscription/invoices", f.token, nil).mustCode(t, "tagihan", 201).data(t)
	return f, inv["id"].(string), int64(inv["total_amount"].(float64))
}

func TestTenantTidakBisaMenandaiTagihanSendiriLunas(t *testing.T) {
	requireDB(t)
	f, invID, total := tagihanTerbuka(t, "konfirmsendiri")

	// Jalur lama sudah tidak ada.
	req := jsonRequest(t, "POST", "/api/v1/subscription-payments", f.token, map[string]any{
		"invoice_id": invID, "amount": total, "method": "transfer",
	})
	req.Header.Set("Idempotency-Key", ulid.New())
	if res := serve(t, req); res.Code != 404 {
		t.Fatalf("POST /subscription-payments masih ada: kode %d", res.Code)
	}

	// Konfirmasi TIDAK mengaktifkan apa pun.
	klaim := kirimKonfirmasi(t, f.token, ulid.New(), invID, total).mustCode(t, "konfirmasi", 201).data(t)
	if klaim["status"] != "pending" {
		t.Fatalf("status konfirmasi = %v, mau pending", klaim["status"])
	}
	if p := paketMe(t, f.token); p["code"] != "free" {
		t.Fatalf("paket setelah konfirmasi (belum disetujui) = %v, mau tetap free", p["code"])
	}
	ov := call(t, "GET", "/api/v1/subscription", f.token, nil).mustOK(t, "ringkasan").data(t)
	if k := ov["payment_claim"].(map[string]any); k["status"] != "pending" {
		t.Fatalf("ringkasan tidak menampilkan konfirmasi yang menunggu: %v", ov["payment_claim"])
	}

	// Tenant tidak bisa menyetujui konfirmasinya sendiri — rute panel memakai
	// token realm lain.
	res := call(t, "POST", "/api/v1/platform/subscription-payment-claims/"+klaim["id"].(string)+"/approve", f.token, nil)
	if res.Code != 401 && res.Code != 403 {
		t.Fatalf("token tenant di rute panel: kode %d", res.Code)
	}

	// Staf keuangan menyetujui → lunas, paket aktif, tenant diberi tahu,
	// keputusannya tercatat di jejak audit tenant itu.
	call(t, "POST", "/api/v1/platform/subscription-payment-claims/"+klaim["id"].(string)+"/approve",
		tokenKeuanganPlatform(t), nil).mustCode(t, "setujui", 201)
	if p := paketMe(t, f.token); p["code"] != "basic" || p["status"] != "active" {
		t.Fatalf("paket setelah disetujui = %v/%v, mau basic/active", p["code"], p["status"])
	}
	var pesan, audit int64
	database.DB.Table("outbox_events").
		Where("tenant_id = ? AND topic = 'subscription.payment_approved'", f.tenantID).Count(&pesan)
	database.DB.Table("audit_logs").
		Where("tenant_id = ? AND action = 'subscription.payment_claim.approve' AND actor_type = 'admin'", f.tenantID).Count(&audit)
	if pesan != 1 || audit != 1 {
		t.Fatalf("pemberitahuan %d, audit %d; mau 1 & 1", pesan, audit)
	}
}

func TestKonfirmasiDitolakBolehDikirimUlang(t *testing.T) {
	requireDB(t)
	f, invID, total := tagihanTerbuka(t, "konfirmtolak")
	klaim := kirimKonfirmasi(t, f.token, ulid.New(), invID, total).mustCode(t, "konfirmasi", 201).data(t)
	jalur := "/api/v1/platform/subscription-payment-claims/" + klaim["id"].(string)

	// Tolak tanpa alasan → 422: tenant perlu tahu apa yang harus diperbaiki.
	call(t, "POST", jalur+"/reject", tokenKeuanganPlatform(t), map[string]any{"reason": ""}).
		mustCode(t, "tolak tanpa alasan", 422)
	call(t, "POST", jalur+"/reject", tokenKeuanganPlatform(t), map[string]any{
		"reason": "Nominal tidak ditemukan di mutasi rekening",
	}).mustOK(t, "tolak")

	ov := call(t, "GET", "/api/v1/subscription", f.token, nil).mustOK(t, "ringkasan").data(t)
	k := ov["payment_claim"].(map[string]any)
	if k["status"] != "rejected" || !strings.Contains(k["reject_reason"].(string), "mutasi") {
		t.Fatalf("tenant tidak melihat penolakan & alasannya: %v", k)
	}
	if ov["open_invoice"] == nil {
		t.Fatal("tagihan harus tetap terbuka setelah konfirmasi ditolak")
	}
	var pesan int64
	database.DB.Table("outbox_events").
		Where("tenant_id = ? AND topic = 'subscription.payment_rejected'", f.tenantID).Count(&pesan)
	if pesan != 1 {
		t.Fatalf("pemberitahuan penolakan = %d, mau 1", pesan)
	}

	// Yang sudah diputus tidak bisa disetujui belakangan.
	call(t, "POST", jalur+"/approve", tokenKeuanganPlatform(t), nil).mustCode(t, "setujui yang ditolak", 409)
	// Kirim ulang → konfirmasi baru, menunggu lagi.
	kirimKonfirmasi(t, f.token, ulid.New(), invID, total).mustCode(t, "konfirmasi ulang", 201)
}

// Wewenang: hanya finance & superadmin (billing.verify) yang melihat &
// memutus konfirmasi. Operator memverifikasi MITRA, bukan uang masuk.
func TestWewenangVerifikasiPembayaran(t *testing.T) {
	requireDB(t)
	_, emailOp, passOp := makePlatformAdmin(t, "operator-bayar", "operator")
	_, emailSup, passSup := makePlatformAdmin(t, "support-bayar", "support")
	operator := platformToken(t, emailOp, passOp)
	support := platformToken(t, emailSup, passSup)

	f, invID, total := tagihanTerbuka(t, "konfirmwewenang")
	klaim := kirimKonfirmasi(t, f.token, ulid.New(), invID, total).mustCode(t, "konfirmasi", 201).data(t)
	jalur := "/api/v1/platform/subscription-payment-claims/" + klaim["id"].(string)

	call(t, "GET", "/api/v1/platform/subscription-payment-claims", support, nil).mustCode(t, "support membaca", 403)
	call(t, "POST", jalur+"/approve", operator, nil).mustCode(t, "operator menyetujui", 403)

	daftar := call(t, "GET", "/api/v1/platform/subscription-payment-claims", tokenKeuanganPlatform(t), nil).
		mustOK(t, "finance membaca").Body["data"].([]any)
	var ketemu map[string]any
	for _, d := range daftar {
		if m := d.(map[string]any); m["id"] == klaim["id"] {
			ketemu = m
		}
	}
	if ketemu == nil || ketemu["business_name"] != "Usaha konfirmwewenang" || ketemu["invoice_number"] == "" {
		t.Fatalf("antrean panel tanpa konteks usaha/tagihan: %v", ketemu)
	}
}

// Dua staf menekan "Setujui" BERSAMAAN: tepat satu yang mencatat pembayaran.
// Diuji di lapisan layanan — di jalur HTTP tes, middleware membuat permintaan
// berangkat berurutan sehingga balapannya tidak pernah terjadi.
func TestPersetujuanSerentakTidakMencatatDuaKali(t *testing.T) {
	requireDB(t)
	adminID, _, _ := makePlatformAdmin(t, "keuangan-serentak", "finance")
	f, invID, total := tagihanTerbuka(t, "konfirmserentak")
	klaim := kirimKonfirmasi(t, f.token, ulid.New(), invID, total).mustCode(t, "konfirmasi", 201).data(t)

	ctx := reqctx.WithPlatformAdmin(context.Background(), adminID, "finance")
	const serentak = 16
	var wg sync.WaitGroup
	mulai := make(chan struct{})
	hasil := make([]error, serentak)
	for i := range hasil {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-mulai
			_, hasil[i] = services.PlatformApprovePaymentClaim(ctx, klaim["id"].(string))
		}(i)
	}
	close(mulai)
	wg.Wait()

	berhasil := 0
	for _, err := range hasil {
		switch {
		case err == nil:
			berhasil++
		case errors.Is(err, helpers.ErrConflict):
		default:
			t.Fatalf("persetujuan serentak: galat tak terduga %v", err)
		}
	}
	var bayar int
	database.DB.Raw(`SELECT count(*) FROM subscription_payments WHERE subscription_invoice_id = ?`, invID).Row().Scan(&bayar)
	if berhasil != 1 || bayar != 1 {
		t.Fatalf("%d persetujuan serentak: %d berhasil, %d pembayaran; mau 1 & 1", serentak, berhasil, bayar)
	}
}
