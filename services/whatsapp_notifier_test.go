package services

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"candra/backend-api/repositories"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sidecarTiruan berpura-pura jadi wa-gateway dengan jawaban yang ditentukan.
func sidecarTiruan(t *testing.T, kode int, badan string, tangkap func(*http.Request, []byte)) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		isi, _ := io.ReadAll(r.Body)
		if tangkap != nil {
			tangkap(r, isi)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(kode)
		_, _ = w.Write([]byte(badan))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// Nomor harus sudah dinormalkan SEBELUM dikirim ke sidecar, dan token ikut.
func TestWhatsAppNotifierMengirimNomorTernormalkan(t *testing.T) {
	var terlihat map[string]string
	var auth string

	srv := sidecarTiruan(t, 200, `{"status":"terkirim","pesan_id":"ABC123"}`,
		func(r *http.Request, isi []byte) {
			auth = r.Header.Get("Authorization")
			_ = json.Unmarshal(isi, &terlihat)
			assert.Equal(t, "/kirim", r.URL.Path)
			assert.Equal(t, http.MethodPost, r.Method)
		})

	n := NewWhatsAppNotifier(srv.URL, "rahasia", KodeNegaraBawaan, 5*time.Second)
	err := n.Kirim(context.Background(), NotifMessage{
		Ke: "0812-3456-7890", Isi: "Tagihan Anda sudah terbit.", TopikID: "outbox-1",
	})

	require.NoError(t, err)
	assert.Equal(t, "6281234567890", terlihat["to"], "sidecar harus menerima MSISDN, bukan bentuk lokal")
	assert.Equal(t, "Tagihan Anda sudah terbit.", terlihat["isi"])
	assert.Equal(t, "outbox-1", terlihat["topik_id"])
	assert.Equal(t, "Bearer rahasia", auth)
}

// Nomor cacat ditolak DI GO, tanpa membebani sidecar sama sekali.
func TestWhatsAppNotifierNomorCacatTidakMenyentuhSidecar(t *testing.T) {
	dipanggil := false
	srv := sidecarTiruan(t, 200, `{}`, func(*http.Request, []byte) { dipanggil = true })

	n := NewWhatsAppNotifier(srv.URL, "rahasia", KodeNegaraBawaan, 5*time.Second)
	err := n.Kirim(context.Background(), NotifMessage{Ke: "bukan nomor", Isi: "halo"})

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNotifPermanen))
	assert.False(t, dipanggil, "nomor yang jelas cacat tidak perlu dikirim ke sidecar")
}

// Pemetaan jawaban sidecar ke "boleh diulang" vs "sudah selesai, gagal".
func TestWhatsAppNotifierMemetakanKegagalan(t *testing.T) {
	kasus := []struct {
		nama     string
		kode     int
		badan    string
		permanen bool
	}{
		{"nomor tak terdaftar di WhatsApp", 422, `{"galat":"nomor tidak terdaftar","permanen":true}`, true},
		{"permintaan cacat", 400, `{"galat":"body tidak sah"}`, true},
		{"WhatsApp belum tertaut", 503, `{"galat":"belum siap","permanen":false}`, false},
		{"sidecar error", 500, `{"galat":"panik"}`, false},
		{"token salah", 401, `{"galat":"token tidak sah"}`, false},
		{"token ditolak", 403, `{"galat":"terlarang"}`, false},
	}

	for _, k := range kasus {
		t.Run(k.nama, func(t *testing.T) {
			srv := sidecarTiruan(t, k.kode, k.badan, nil)
			n := NewWhatsAppNotifier(srv.URL, "rahasia", KodeNegaraBawaan, 5*time.Second)

			err := n.Kirim(context.Background(), NotifMessage{Ke: "081234567890", Isi: "halo"})
			require.Error(t, err)
			assert.Equal(t, k.permanen, errors.Is(err, ErrNotifPermanen),
				"HTTP %d seharusnya permanen=%v, dapat galat: %v", k.kode, k.permanen, err)
		})
	}
}

// Sidecar mati adalah keadaan yang PALING pantas dicoba ulang — jangan sampai
// notifikasi hangus hanya karena proses Node sedang direstart.
func TestWhatsAppNotifierSidecarMatiBolehDiulang(t *testing.T) {
	// Port yang sengaja tidak ada yang mendengarkan.
	n := NewWhatsAppNotifier("http://127.0.0.1:1", "rahasia", KodeNegaraBawaan, 2*time.Second)
	err := n.Kirim(context.Background(), NotifMessage{Ke: "081234567890", Isi: "halo"})

	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrNotifPermanen),
		"sidecar yang sedang mati harus dicoba ulang, bukan dibuang")
}

// Inti dari ErrNotifPermanen: langsung ke antrean mati, tanpa menghabiskan
// jatah sepuluh percobaan.
func TestKlasifikasiOutboxPermanenLangsungMati(t *testing.T) {
	status, pesan := klasifikasiOutbox(errors.New("jaringan goyah"), 1)
	assert.Equal(t, "failed", status, "galat biasa pada percobaan pertama harus dicoba ulang")
	assert.Contains(t, pesan, "jaringan goyah")

	status, _ = klasifikasiOutbox(errors.New("jaringan goyah"), repositories.MaxOutboxAttempts)
	assert.Equal(t, "dead", status, "galat biasa mati setelah jatah percobaan habis")

	status, _ = klasifikasiOutbox(ErrNotifPermanen, 1)
	assert.Equal(t, "dead", status, "galat permanen mati pada percobaan PERTAMA")

	status, _ = klasifikasiOutbox(NotifMessage{}.galatPermanenContoh(), 1)
	assert.Equal(t, "dead", status, "galat permanen yang dibungkus %w tetap terdeteksi")

	status, _ = klasifikasiOutbox(nil, 1)
	assert.Equal(t, "done", status)
}

// galatPermanenContoh membungkus sentinel seperti kode sungguhan melakukannya,
// untuk memastikan errors.Is menembus pembungkusnya.
func (NotifMessage) galatPermanenContoh() error {
	_, err := NormalkanNomorWA("0812", KodeNegaraBawaan)
	return err
}
