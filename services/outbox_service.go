package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"candra/backend-api/models"
	"candra/backend-api/repositories"
	"candra/backend-api/structs"

	"gorm.io/gorm"
)

// Pengiriman notifikasi keluar dengan pola OUTBOX (§5.14, migrasi 000032).
//
// Peristiwa ditulis ke `outbox_events` DI DALAM transaksi bisnis yang sama
// dengan perubahan datanya, lalu dikirim belakangan oleh pekerja
// (cmd/process-outbox). Itu yang membuat dua kegagalan klasik mustahil terjadi:
// "tagihan tersimpan tapi notifikasinya tidak pernah terkirim", dan
// "notifikasi terkirim padahal transaksinya batal".
//
// Penyedia WhatsApp/email sungguhan menyusul; sampai itu ada, pengirim bawaan
// hanya MENCATAT ke log — persis pola genericAdapter di pipeline kanal:
// alurnya sudah utuh dan teruji, tinggal menukar satu implementasi.

// Notifier mengirim satu pesan yang sudah dirender. Satu penyedia = satu
// implementasi; inti aplikasi tidak berubah.
type Notifier interface {
	// Kirim mengirim pesan. Error apa pun akan membuat peristiwa dicoba ulang
	// dengan penundaan bertahap sampai batas percobaan.
	Kirim(ctx context.Context, msg NotifMessage) error
	// Saluran yang ditangani: "whatsapp" atau "email".
	Saluran() string
}

// ErrNotifPermanen menandai kegagalan pengiriman yang TIDAK akan membaik bila
// diulang: nomor tujuan tidak terdaftar di WhatsApp, alamat email cacat, dan
// sejenisnya. Pengirim membungkusnya dengan %w agar antrean bisa membedakannya.
//
// Tanpa penanda ini setiap kegagalan diperlakukan sama: 10 percobaan dengan
// penundaan bertambah, jadi kesalahan ketik satu digit pada nomor pelanggan
// baru terlihat di antrean mati berjam-jam kemudian. Yang dibutuhkan operator
// justru sebaliknya — tahu SEKARANG bahwa nomornya salah, selagi ia masih ingat
// tagihan mana yang baru saja diterbitkan.
var ErrNotifPermanen = errors.New("notifikasi gagal permanen")

// NotifMessage adalah pesan siap kirim.
type NotifMessage struct {
	Ke      string // nomor HP atau alamat email
	Subjek  string // kosong untuk WhatsApp
	Isi     string
	TopikID string // id peristiwa outbox, untuk korelasi log
}

// logNotifier adalah pengirim bawaan: mencatat, tidak benar-benar mengirim.
// Membuat seluruh alur bisa dijalankan & diuji tanpa penyedia berbayar.
type logNotifier struct{ saluran string }

func (n logNotifier) Saluran() string { return n.saluran }
func (n logNotifier) Kirim(ctx context.Context, msg NotifMessage) error {
	slog.Info("notifikasi (mode catat saja — penyedia belum dipasang)",
		"saluran", n.saluran, "ke", msg.Ke, "subjek", msg.Subjek, "outbox_id", msg.TopikID)
	return nil
}

// notifiers dipilih berdasarkan saluran. Ditukar lewat RegisterNotifier saat
// penyedia sungguhan sudah tersedia.
var notifiers = map[string]Notifier{
	"whatsapp": logNotifier{saluran: "whatsapp"},
	"email":    logNotifier{saluran: "email"},
}

// RegisterNotifier memasang pengirim sungguhan untuk sebuah saluran, dan
// mengembalikan pengirim yang digantikannya — supaya pemanggil bisa
// memulihkannya (dipakai tes, dan berguna saat menukar penyedia sementara).
func RegisterNotifier(n Notifier) Notifier {
	sebelumnya := notifiers[n.Saluran()]
	notifiers[n.Saluran()] = n
	return sebelumnya
}

// ── Penulisan peristiwa ─────────────────────────────────────────────────

// EnqueueNotification menulis peristiwa notifikasi. tx WAJIB diisi bila
// dipanggil dari dalam transaksi bisnis — itulah inti pola outbox.
func EnqueueNotification(ctx context.Context, tx *gorm.DB, tenantID *string, topic string, payload map[string]any) error {
	return repositories.EnqueueOutbox(ctx, tx, tenantID, topic, payload)
}

// ── Pekerja ─────────────────────────────────────────────────────────────

// ProcessOutbox mengirim peristiwa yang sudah waktunya, satu per transaksi
// (FOR UPDATE SKIP LOCKED — aman beberapa instance). GLOBAL, lintas tenant.
func ProcessOutbox(ctx context.Context) (structs.OutboxRunResult, error) {
	var res structs.OutboxRunResult
	const maxIter = 1000 // pengaman: jangan berputar tanpa batas dalam satu panggilan
	for i := 0; i < maxIter; i++ {
		ditangani := false
		err := repositories.Transaction(ctx, func(tx *gorm.DB) error {
			evs, err := repositories.ClaimDueOutbox(ctx, tx, 1)
			if err != nil {
				return err
			}
			if len(evs) == 0 {
				return nil
			}
			ditangani = true
			ev := evs[0]
			status, lastErr := kirimSatuPeristiwa(ctx, ev)
			switch status {
			case "done":
				res.Sent++
			case "dead":
				res.Dead++
			default:
				res.Failed++
			}
			return repositories.SaveOutboxResult(ctx, tx, ev.ID, status, lastErr, ev.Attempts+1)
		})
		if err != nil {
			return res, err
		}
		if !ditangani {
			break
		}
	}
	return res, nil
}

// kirimSatuPeristiwa merender template lalu mengirimnya. Mengembalikan status
// akhir + pesan error; tidak mengembalikan error ke pemanggil agar statusnya
// TETAP tersimpan meski pengiriman gagal.
func kirimSatuPeristiwa(ctx context.Context, ev models.OutboxEvent) (status, lastErr string) {
	berikutnya := ev.Attempts + 1

	var data map[string]any
	if err := json.Unmarshal(ev.Payload, &data); err != nil {
		return "dead", "payload bukan JSON objek: " + err.Error() // rusak → permanen
	}

	saluran, _ := data["channel"].(string)
	if saluran == "" {
		saluran = "whatsapp"
	}
	tujuan, _ := data["to"].(string)
	if strings.TrimSpace(tujuan) == "" {
		return "dead", "payload tidak memuat tujuan (`to`)" // tak akan membaik
	}

	tmpl, ada, err := repositories.FindNotificationTemplate(ctx, ev.TenantID, ev.Topic, saluran)
	if err != nil {
		return klasifikasiOutbox(err, berikutnya)
	}
	if !ada {
		// Template belum dibuat — retry tidak menolong sampai ada yang membuatnya.
		return "dead", fmt.Sprintf("template %q saluran %q belum ada", ev.Topic, saluran)
	}

	n, ok := notifiers[saluran]
	if !ok {
		return "dead", "saluran tidak dikenal: " + saluran
	}
	if err := n.Kirim(ctx, NotifMessage{
		Ke:      tujuan,
		Subjek:  renderTemplate(tmpl.Subject, data),
		Isi:     renderTemplate(tmpl.Body, data),
		TopikID: ev.ID,
	}); err != nil {
		return klasifikasiOutbox(err, berikutnya)
	}
	return "done", ""
}

// klasifikasiOutbox memetakan kegagalan pengiriman ke status antrean.
func klasifikasiOutbox(err error, berikutnya int) (string, string) {
	if err == nil {
		return "done", ""
	}
	// Kegagalan permanen tidak menunggu jatah percobaan habis — lihat
	// ErrNotifPermanen di atas.
	if errors.Is(err, ErrNotifPermanen) {
		return "dead", err.Error()
	}
	if berikutnya >= repositories.MaxOutboxAttempts {
		return "dead", err.Error()
	}
	return "failed", err.Error()
}

// renderTemplate mengganti penanda {{nama}} dengan nilai dari payload.
//
// Sengaja SESEDERHANA ini, bukan text/template: isi template ditulis operator
// lewat panel, dan mesin template penuh membuka pintu ke data/perilaku yang
// tidak diniatkan. Penanda yang tidak dikenal dibiarkan apa adanya supaya
// kesalahan ketik terlihat di pesan, bukan hilang diam-diam.
func renderTemplate(s string, data map[string]any) string {
	for k, v := range data {
		s = strings.ReplaceAll(s, "{{"+k+"}}", fmt.Sprintf("%v", v))
	}
	return s
}

// ── Panel internal ──────────────────────────────────────────────────────

// ListOutboxEvents menampilkan antrean & antrean mati.
func ListOutboxEvents(ctx context.Context, status, topic string, limit int) ([]structs.OutboxEventResponse, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := repositories.ListOutboxEvents(ctx, status, topic, limit)
	if err != nil {
		return nil, err
	}
	out := make([]structs.OutboxEventResponse, len(rows))
	for i, e := range rows {
		r := structs.OutboxEventResponse{
			ID: e.ID, Topic: e.Topic, Status: e.Status,
			Attempts: e.Attempts, LastError: e.LastError,
			AvailableAt: e.AvailableAt.Format(partnerTimeLayout),
			CreatedAt:   e.CreatedAt.Format(partnerTimeLayout),
		}
		if e.TenantID != nil {
			r.TenantID = *e.TenantID
		}
		if e.ProcessedAt != nil {
			r.ProcessedAt = e.ProcessedAt.Format(partnerTimeLayout)
		}
		out[i] = r
	}
	return out, nil
}

// RetryOutboxEvent mengembalikan peristiwa mati ke antrean setelah masalahnya
// diperbaiki — tanpa ini antrean mati jadi kuburan.
func RetryOutboxEvent(ctx context.Context, id string) error {
	if err := repositories.RetryOutboxEvent(ctx, id); err != nil {
		return err
	}
	AuditPlatformAction(ctx, "outbox.retry", "outbox_events", id)
	return nil
}

// UpsertNotificationTemplate menyimpan template pesan.
func UpsertNotificationTemplate(ctx context.Context, in structs.NotificationTemplateRequest) error {
	if !contains(models.NotificationChannels, in.Channel) {
		return fmt.Errorf("channel harus salah satu dari %v", models.NotificationChannels)
	}
	t := models.NotificationTemplate{
		Code: strings.TrimSpace(in.Code), Channel: in.Channel,
		Subject: in.Subject, Body: in.Body,
	}
	if in.TenantID != "" {
		t.TenantID = &in.TenantID
	}
	return repositories.UpsertNotificationTemplate(ctx, &t)
}
