package tests

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"candra/backend-api/database"
	"candra/backend-api/services"
)

// Uji ujung-ke-ujung jalur notifikasi WhatsApp: tagihan terbit → peristiwa
// outbox → pekerja → sidecar.
//
// Tes unit di services/ sudah memastikan tiap bagiannya benar. Yang dijaga DI
// SINI adalah SAMBUNGANNYA — bahwa nomor yang tersimpan di kolom tenant benar
// -benar sampai ke sidecar dalam bentuk yang dimengerti WhatsApp, dan bahwa
// kegagalan permanen benar-benar mengubah baris di tabel `outbox_events`.
//
// SATU JEBAKAN yang membuat tes ini harus ditulis hati-hati: ProcessOutbox
// bekerja LINTAS TENANT — ia mengambil peristiwa siapa pun yang sudah waktunya,
// termasuk sisa dari tes lain di paket ini. Karena itu sidecar tiruan di bawah
// menjawab BERDASARKAN NOMOR TUJUAN, bukan seragam: tanpa itu, tes yang
// mensimulasikan kegagalan ikut mematikan antrean tenant lain, dan tes yang
// memeriksa nomor bisa menangkap pesan milik tenant lain.

// sidecarTiruan mencatat setiap panggilan dan menjawab sesuai keputusan
// pemanggil. Notifier semula dipulihkan setelah tes selesai.
type sidecarTiruan struct {
	mu       sync.Mutex
	diterima []map[string]string
}

// pasangSidecarTiruan memasang notifier WhatsApp yang menunjuk sidecar palsu.
// `jawab` menentukan balasan untuk satu nomor tujuan; kembalikan (0, "") untuk
// memakai balasan sukses bawaan.
func pasangSidecarTiruan(t *testing.T, jawab func(to string) (int, string)) *sidecarTiruan {
	t.Helper()
	s := &sidecarTiruan{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		isi, _ := io.ReadAll(r.Body)
		var m map[string]string
		_ = json.Unmarshal(isi, &m)

		s.mu.Lock()
		s.diterima = append(s.diterima, m)
		s.mu.Unlock()

		kode, badan := 0, ""
		if jawab != nil {
			kode, badan = jawab(m["to"])
		}
		if kode == 0 {
			kode, badan = 200, `{"status":"terkirim","pesan_id":"X1"}`
		}
		w.WriteHeader(kode)
		_, _ = w.Write([]byte(badan))
	}))

	semula := services.RegisterNotifier(
		services.NewWhatsAppNotifier(srv.URL, "token-uji", services.KodeNegaraBawaan, 5*time.Second),
	)
	t.Cleanup(func() {
		srv.Close()
		if semula != nil {
			services.RegisterNotifier(semula)
		}
	})
	return s
}

// pesanUntuk mencari pesan yang dikirim ke satu nomor tertentu.
func (s *sidecarTiruan) pesanUntuk(to string) (map[string]string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, m := range s.diterima {
		if m["to"] == to {
			return m, true
		}
	}
	return nil, false
}

// setelNomorTenant menuliskan nomor telepon tenant apa adanya, seperti yang
// diketik pemiliknya.
func setelNomorTenant(t *testing.T, tenantID, nomor string) {
	t.Helper()
	if err := database.DB.Table("tenants").
		Where("id = ?", tenantID).Update("phone", nomor).Error; err != nil {
		t.Fatalf("menyetel nomor tenant: %v", err)
	}
}

// peristiwaTerakhir membaca peristiwa outbox terbaru milik satu tenant.
func peristiwaTerakhir(t *testing.T, tenantID string) (status string, attempts int, lastErr string) {
	t.Helper()
	var baris struct {
		Status    string
		Attempts  int
		LastError string
	}
	if err := database.DB.Table("outbox_events").
		Select("status, attempts, last_error").
		Where("tenant_id = ? AND topic = 'invoice.issued'", tenantID).
		Order("created_at DESC").Limit(1).Scan(&baris).Error; err != nil {
		t.Fatalf("membaca peristiwa outbox: %v", err)
	}
	return baris.Status, baris.Attempts, baris.LastError
}

// Nomor tenant harus tiba di sidecar dalam bentuk MSISDN, bukan bentuk lokal
// yang diketik pemiliknya.
func TestWhatsAppNotifikasiSampaiKeSidecar(t *testing.T) {
	requireDB(t)

	const diketik = "0812-7000-0001"
	const diharap = "6281270000001"

	s := pasangSidecarTiruan(t, nil) // semua berhasil

	f := registerTenantPolos(t, "wanotif1")
	setelNomorTenant(t, f.tenantID, diketik)
	startBasic12(t, f) // menerbitkan tagihan → peristiwa outbox

	if _, err := services.ProcessOutbox(bg()); err != nil {
		t.Fatalf("proses outbox: %v", err)
	}

	m, ada := s.pesanUntuk(diharap)
	if !ada {
		t.Fatalf("sidecar tidak pernah menerima %q (bentuk ternormalkan dari %q); yang diterima: %v",
			diharap, diketik, s.diterima)
	}
	if m["isi"] == "" {
		t.Fatal("isi pesan kosong — template tidak terender")
	}
	if m["topik_id"] == "" {
		t.Fatal("topik_id kosong — korelasi log ke peristiwa outbox hilang")
	}
}

// Nomor yang tidak terdaftar di WhatsApp harus langsung masuk antrean mati pada
// percobaan PERTAMA — bukan setelah sepuluh percobaan selama berjam-jam.
func TestWhatsAppNomorTidakTerdaftarLangsungMati(t *testing.T) {
	requireDB(t)

	const diketik = "0812-7000-0002"
	const diharap = "6281270000002"

	// Hanya nomor tenant INI yang ditolak; peristiwa tenant lain yang kebetulan
	// ikut terambil tetap dianggap berhasil.
	pasangSidecarTiruan(t, func(to string) (int, string) {
		if to == diharap {
			return 422, `{"galat":"nomor tidak terdaftar","permanen":true}`
		}
		return 0, ""
	})

	f := registerTenantPolos(t, "wanotif2")
	setelNomorTenant(t, f.tenantID, diketik)
	startBasic12(t, f)

	if _, err := services.ProcessOutbox(bg()); err != nil {
		t.Fatalf("proses outbox: %v", err)
	}

	status, attempts, lastErr := peristiwaTerakhir(t, f.tenantID)
	if status != "dead" {
		t.Fatalf("status = %q (percobaan %d), mau \"dead\" pada percobaan pertama; galat: %s",
			status, attempts, lastErr)
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, mau 1 — kegagalan permanen tidak boleh menghabiskan jatah percobaan",
			attempts)
	}
}

// Sidecar yang sedang mati TIDAK boleh membuat notifikasi hangus: peristiwanya
// harus tetap di antrean untuk dicoba lagi nanti.
func TestWhatsAppSidecarMatiTetapDiantrekan(t *testing.T) {
	requireDB(t)

	const diketik = "0812-7000-0003"
	const diharap = "6281270000003"

	pasangSidecarTiruan(t, func(to string) (int, string) {
		if to == diharap {
			return 503, `{"galat":"belum tertaut","permanen":false}`
		}
		return 0, ""
	})

	f := registerTenantPolos(t, "wanotif3")
	setelNomorTenant(t, f.tenantID, diketik)
	startBasic12(t, f)

	if _, err := services.ProcessOutbox(bg()); err != nil {
		t.Fatalf("proses outbox: %v", err)
	}

	status, attempts, _ := peristiwaTerakhir(t, f.tenantID)
	if status != "failed" {
		t.Fatalf("status = %q, mau \"failed\" (masih akan dicoba lagi)", status)
	}
	if attempts >= 10 {
		t.Fatalf("attempts = %d — masih harus punya sisa percobaan", attempts)
	}
}
