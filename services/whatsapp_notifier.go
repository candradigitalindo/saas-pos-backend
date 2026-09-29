package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"candra/backend-api/config"
)

// Pengirim WhatsApp lewat sidecar Baileys (lihat ../wa-gateway/).
//
// KENAPA LEWAT SIDECAR. Baileys adalah pustaka Node yang bicara WebSocket ke
// WhatsApp Web; tidak ada cara memanggilnya dari Go. Jadi Node memegang
// sambungan WhatsApp-nya, dan berkas ini hanya memanggil HTTP lokal. Yang
// berpindah lewat kabel cuma "kirim teks ini ke nomor itu" — seluruh kerumitan
// sesi, QR, dan sambung-ulang tinggal di sisi Node.
//
// YANG PERLU DIKETAHUI SEBELUM DIPAKAI PRODUKSI. Baileys tidak resmi: ia meniru
// WhatsApp Web dan bisa rusak saat WhatsApp berubah, dan nomor yang berkelakuan
// seperti robot berisiko diblokir. Bila notifikasi tagihan menjadi hal yang
// tidak boleh gagal, WhatsApp Business API resmi atau gateway berbayar adalah
// pilihan yang lebih tenang — dan menukarnya cukup satu implementasi Notifier
// baru, tanpa menyentuh inti.

// WhatsAppNotifier mengirim pesan lewat sidecar Baileys.
type WhatsAppNotifier struct {
	baseURL    string
	token      string
	kodeNegara string
	klien      *http.Client
}

// NewWhatsAppNotifier merakit pengirim. `baseURL` menunjuk sidecar, mis.
// "http://127.0.0.1:8090".
func NewWhatsAppNotifier(baseURL, token, kodeNegara string, timeout time.Duration) *WhatsAppNotifier {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &WhatsAppNotifier{
		baseURL:    strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		token:      token,
		kodeNegara: kodeNegara,
		klien:      &http.Client{Timeout: timeout},
	}
}

// Saluran memenuhi Notifier.
func (n *WhatsAppNotifier) Saluran() string { return "whatsapp" }

// balasanSidecar adalah bentuk JSON yang dikembalikan sidecar.
type balasanSidecar struct {
	Status   string `json:"status"`
	PesanID  string `json:"pesan_id"`
	Galat    string `json:"galat"`
	Permanen bool   `json:"permanen"`
}

// Kirim mengirim satu pesan.
//
// Pembagian galat permanen vs sementara yang menentukan nasib peristiwa di
// antrean: permanen → langsung antrean mati (operator perlu memperbaiki data),
// sementara → dicoba ulang dengan penundaan bertambah.
func (n *WhatsAppNotifier) Kirim(ctx context.Context, msg NotifMessage) error {
	nomor, err := NormalkanNomorWA(msg.Ke, n.kodeNegara)
	if err != nil {
		return err // sudah dibungkus ErrNotifPermanen
	}

	badan, err := json.Marshal(map[string]string{
		"to":       nomor,
		"isi":      msg.Isi,
		"topik_id": msg.TopikID,
	})
	if err != nil {
		return fmt.Errorf("%w: gagal menyusun permintaan: %v", ErrNotifPermanen, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.baseURL+"/kirim", bytes.NewReader(badan))
	if err != nil {
		return fmt.Errorf("%w: URL sidecar tidak sah (%q): %v", ErrNotifPermanen, n.baseURL, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+n.token)

	res, err := n.klien.Do(req)
	if err != nil {
		// Sidecar mati atau jaringan lokal bermasalah — justru keadaan yang
		// paling pantas dicoba ulang.
		return fmt.Errorf("sidecar WhatsApp tidak terjangkau di %s: %w", n.baseURL, err)
	}
	defer res.Body.Close()

	// Dibatasi supaya balasan yang tak terduga besar tidak menyedot memori.
	mentah, _ := io.ReadAll(io.LimitReader(res.Body, 32<<10))
	var b balasanSidecar
	_ = json.Unmarshal(mentah, &b)

	pesan := b.Galat
	if pesan == "" {
		pesan = strings.TrimSpace(string(mentah))
	}

	switch {
	case res.StatusCode == http.StatusOK:
		slog.Info("WhatsApp terkirim", "ke", nomor, "pesan_id", b.PesanID, "outbox_id", msg.TopikID)
		return nil

	case res.StatusCode == http.StatusUnauthorized, res.StatusCode == http.StatusForbidden:
		// Token salah adalah salah KONFIGURASI, bukan salah data. Sengaja
		// diperlakukan sementara: kalau permanen, seluruh antrean mati dalam
		// hitungan detik dan notifikasinya hilang. Dicoba ulang memberi
		// operator beberapa jam untuk memperbaiki token.
		slog.Error("sidecar WhatsApp menolak token — periksa WA_GATEWAY_TOKEN di kedua sisi",
			"status", res.StatusCode, "outbox_id", msg.TopikID)
		return fmt.Errorf("sidecar WhatsApp menolak token (HTTP %d)", res.StatusCode)

	case b.Permanen, res.StatusCode == http.StatusUnprocessableEntity, res.StatusCode == http.StatusBadRequest:
		return fmt.Errorf("%w: %s", ErrNotifPermanen, pesan)

	default:
		// Termasuk 503 saat WhatsApp belum tertaut atau sambungan sedang putus.
		return fmt.Errorf("sidecar WhatsApp gagal (HTTP %d): %s", res.StatusCode, pesan)
	}
}

// PasangNotifierDariEnv memasang pengirim sungguhan bila dikonfigurasi.
//
// Tanpa WA_GATEWAY_URL, aplikasi tetap jalan dengan pengirim bawaan yang hanya
// MENCATAT ke log. Itu disengaja: alur outbox harus bisa dijalankan dan diuji
// oleh siapa pun tanpa lebih dulu menautkan nomor WhatsApp sungguhan.
func PasangNotifierDariEnv() {
	url := config.GetEnv("WA_GATEWAY_URL", "")
	if url == "" {
		slog.Info("WA_GATEWAY_URL kosong — notifikasi WhatsApp tetap mode catat saja")
		return
	}
	token := config.GetEnv("WA_GATEWAY_TOKEN", "")
	if token == "" {
		// Berjalan tanpa token berarti setiap pesan ditolak 401 lalu dicoba
		// ulang sampai mati. Lebih baik berhenti sekarang, saat masih terbaca.
		slog.Error("WA_GATEWAY_URL diisi tetapi WA_GATEWAY_TOKEN kosong — notifikasi WhatsApp TIDAK dipasang")
		return
	}

	RegisterNotifier(NewWhatsAppNotifier(
		url,
		token,
		config.GetEnv("WA_KODE_NEGARA", KodeNegaraBawaan),
		time.Duration(config.GetIntEnv("WA_GATEWAY_TIMEOUT_SECONDS", 30))*time.Second,
	))
	slog.Info("notifikasi WhatsApp memakai sidecar", "url", url)
}
