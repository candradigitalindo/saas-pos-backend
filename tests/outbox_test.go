package tests

import (
	"testing"

	"candra/backend-api/database"
	"candra/backend-api/services"
)

// Uji outbox notifikasi (§5.14, migrasi 000032): peristiwa ditulis di dalam
// transaksi bisnis, dikirim belakangan oleh pekerja, gagal → dicoba ulang
// bertahap, dan yang mati bisa dilihat serta dikembalikan ke antrean.

// TestOutboxEnqueuedWithBusinessTransaction — DoD pola outbox: menerbitkan
// tagihan langganan JUGA menuliskan peristiwa notifikasinya, dalam transaksi
// yang sama. Tidak ada tagihan tanpa pemberitahuan.
func TestOutboxEnqueuedWithBusinessTransaction(t *testing.T) {
	requireDB(t)
	f := registerTenant(t, "outbox1")

	var sebelum int64
	database.DB.Table("outbox_events").
		Where("tenant_id = ? AND topic = 'invoice.issued'", f.tenantID).Count(&sebelum)

	startBasic12(t, f) // menerbitkan tagihan

	var sesudah int64
	database.DB.Table("outbox_events").
		Where("tenant_id = ? AND topic = 'invoice.issued' AND status = 'pending'", f.tenantID).
		Count(&sesudah)
	if sesudah != sebelum+1 {
		t.Fatalf("peristiwa outbox = %d, mau %d (satu per tagihan terbit)", sesudah, sebelum+1)
	}
}

// TestOutboxWorkerSendsAndIsIdempotent — pekerja mengirim yang tertunda, dan
// putaran berikutnya tidak mengirim ulang yang sudah selesai.
func TestOutboxWorkerSendsAndIsIdempotent(t *testing.T) {
	requireDB(t)
	f := registerTenant(t, "outbox2")
	startBasic12(t, f)

	res, err := services.ProcessOutbox(bg())
	if err != nil {
		t.Fatalf("proses outbox: %v", err)
	}
	if res.Sent < 1 {
		t.Fatalf("hasil = %+v, mau minimal satu terkirim", res)
	}

	var terkirim int64
	database.DB.Table("outbox_events").
		Where("tenant_id = ? AND status = 'done' AND processed_at IS NOT NULL", f.tenantID).
		Count(&terkirim)
	if terkirim < 1 {
		t.Fatalf("peristiwa berstatus done = %d, mau >= 1", terkirim)
	}

	// Putaran kedua: tidak ada lagi yang tertunda untuk tenant ini.
	res2, err := services.ProcessOutbox(bg())
	if err != nil {
		t.Fatalf("proses outbox kedua: %v", err)
	}
	var masihPending int64
	database.DB.Table("outbox_events").
		Where("tenant_id = ? AND status IN ('pending','failed')", f.tenantID).Count(&masihPending)
	if masihPending != 0 {
		t.Fatalf("masih ada %d peristiwa tertunda setelah putaran kedua (%+v)", masihPending, res2)
	}
}

// TestOutboxTemplateRendered — penanda {{...}} diganti nilai payload, dan
// template milik TENANT menang atas template bawaan sistem.
func TestOutboxTemplateRendered(t *testing.T) {
	requireDB(t)
	_, email, pass := makePlatformAdmin(t, "outbox", "superadmin")
	adm := platformToken(t, email, pass)
	f := registerTenant(t, "outbox3")

	// Template khusus tenant ini.
	call(t, "POST", "/api/v1/platform/notification-templates", adm, map[string]any{
		"tenant_id": f.tenantID, "code": "invoice.issued", "channel": "whatsapp",
		"body": "Khusus {{nama_usaha}}: tagihan {{nomor}} sebesar {{total}}.",
	}).mustOK(t, "simpan template tenant")

	startBasic12(t, f)
	if _, err := services.ProcessOutbox(bg()); err != nil {
		t.Fatalf("proses outbox: %v", err)
	}

	// Pengirim bawaan hanya mencatat, jadi yang diverifikasi: peristiwanya
	// selesai (template ketemu & render sukses), bukan isi pesannya.
	var done int64
	database.DB.Table("outbox_events").
		Where("tenant_id = ? AND topic = 'invoice.issued' AND status = 'done'", f.tenantID).Count(&done)
	if done < 1 {
		t.Fatal("peristiwa tidak selesai — template tenant seharusnya ditemukan")
	}
}

// TestOutboxDeadLetterVisibleAndRetryable — peristiwa yang tak mungkin berhasil
// masuk antrean mati, terlihat di panel, dan bisa dikembalikan ke antrean.
func TestOutboxDeadLetterVisibleAndRetryable(t *testing.T) {
	requireDB(t)
	_, email, pass := makePlatformAdmin(t, "dead", "superadmin")
	adm := platformToken(t, email, pass)

	// Topik tanpa template → tidak akan pernah berhasil → langsung mati.
	if err := services.EnqueueNotification(bg(), nil, nil, "topik.tidak.ada", map[string]any{
		"channel": "whatsapp", "to": "0812000111",
	}); err != nil {
		t.Fatalf("antre peristiwa: %v", err)
	}
	res, err := services.ProcessOutbox(bg())
	if err != nil {
		t.Fatalf("proses: %v", err)
	}
	if res.Dead < 1 {
		t.Fatalf("hasil = %+v, mau minimal satu masuk antrean mati", res)
	}

	// Terlihat di panel internal.
	mati := call(t, "GET", "/api/v1/platform/outbox?status=dead", adm, nil).
		mustOK(t, "lihat antrean mati").Body["data"].([]any)
	if len(mati) == 0 {
		t.Fatal("antrean mati kosong di panel — kegagalan jadi tak terlihat")
	}
	var target map[string]any
	for _, e := range mati {
		if m := e.(map[string]any); m["topic"] == "topik.tidak.ada" {
			target = m
		}
	}
	if target == nil {
		t.Fatalf("peristiwa mati tidak ditemukan di panel: %v", mati)
	}
	if target["last_error"] == "" {
		t.Fatal("alasan kegagalan tidak tercatat")
	}

	// Bisa dikembalikan ke antrean setelah masalahnya diperbaiki.
	call(t, "POST", "/api/v1/platform/outbox/"+target["id"].(string)+"/retry", adm, nil).
		mustOK(t, "coba ulang")
	var pending int64
	database.DB.Table("outbox_events").
		Where("id = ? AND status = 'pending' AND attempts = 0", target["id"]).Count(&pending)
	if pending != 1 {
		t.Fatal("peristiwa tidak kembali ke antrean")
	}
}

// TestOutboxPanelPermission — antrean notifikasi hanya untuk staf internal.
func TestOutboxPanelPermission(t *testing.T) {
	requireDB(t)
	f := registerTenant(t, "outboxperm")
	call(t, "GET", "/api/v1/platform/outbox", f.token, nil).
		mustCode(t, "tenant lihat antrean notifikasi", 401)

	_, email, pass := makePlatformAdmin(t, "outboxsup", "support")
	sup := platformToken(t, email, pass)
	call(t, "GET", "/api/v1/platform/outbox", sup, nil).mustOK(t, "support boleh membaca")
	call(t, "POST", "/api/v1/platform/notification-templates", sup, map[string]any{
		"code": "x.y", "channel": "email", "body": "halo",
	}).mustCode(t, "support ubah template", 403)
}
