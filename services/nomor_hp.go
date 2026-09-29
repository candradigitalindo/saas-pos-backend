package services

import (
	"fmt"
	"strings"
)

// KodeNegaraBawaan dipakai saat nomor ditulis dengan awalan lokal ("0812…").
const KodeNegaraBawaan = "62"

// NormalkanNomorWA mengubah nomor yang DIKETIK MANUSIA menjadi MSISDN angka
// saja yang bisa dipakai WhatsApp (mis. "6281234567890").
//
// Ini bukan kerapian belaka. Nomor tujuan notifikasi berasal dari kolom yang
// diisi pemilik warung sendiri (lihat `tenant.Phone` di subscription_service),
// dan di Indonesia orang menulis nomornya dengan cara yang berbeda-beda:
//
//	0812-3456-7890      (0)812 3456 7890      +62 812 3456 7890
//	62 812 3456 7890    0062812345 6789       812-3456-7890
//
// WhatsApp hanya mengenali satu bentuk. Dikirim apa adanya, "081234567890"
// dibaca sebagai nomor asing dan pesannya tidak pernah sampai — diam-diam,
// tanpa galat yang jelas.
//
// Nomor luar negeri dibiarkan apa adanya: yang ditambahi kode negara HANYA
// bentuk lokal (awalan "0") dan nomor seluler Indonesia tanpa awalan ("8…").
// Menempelkan "62" ke sembarang nomor akan merusak nomor Malaysia atau Amerika
// yang sah.
//
// Galat dibungkus ErrNotifPermanen: nomor yang bentuknya salah tidak akan
// berubah benar karena dicoba ulang.
func NormalkanNomorWA(mentah, kodeNegara string) (string, error) {
	asli := strings.TrimSpace(mentah)
	if kodeNegara == "" {
		kodeNegara = KodeNegaraBawaan
	}

	// "+" harus dikenali SEBELUM dibuang: ia satu-satunya penanda bahwa nomornya
	// sudah lengkap dengan kode negara.
	internasional := strings.HasPrefix(asli, "+")

	var b strings.Builder
	for _, r := range asli {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	angka := b.String()

	if angka == "" {
		return "", fmt.Errorf("%w: nomor tujuan kosong atau tidak memuat angka (%q)", ErrNotifPermanen, mentah)
	}

	switch {
	case internasional:
		// Sudah internasional, tidak diapa-apakan.
	case strings.HasPrefix(angka, "00"):
		// Awalan panggilan internasional gaya lama: 0062… → 62…
		angka = strings.TrimPrefix(angka, "00")
	case strings.HasPrefix(angka, "0"):
		// Bentuk lokal: 0812… → 62812…
		angka = kodeNegara + angka[1:]
	case strings.HasPrefix(angka, kodeNegara):
		// Sudah memakai kode negara tanpa "+".
	case strings.HasPrefix(angka, "8"):
		// Seluler Indonesia yang awalan nolnya hilang — sering terjadi saat nomor
		// disalin dari spreadsheet, yang membuang nol di depan.
		angka = kodeNegara + angka
	}

	// Batas E.164: nomor terpanjang di dunia 15 digit. Batas bawah 8 digit
	// menolak sisa ketikan seperti "0812" yang jelas belum selesai.
	if len(angka) < 8 || len(angka) > 15 {
		return "", fmt.Errorf("%w: nomor %q menghasilkan %d digit, di luar batas wajar 8–15", ErrNotifPermanen, mentah, len(angka))
	}
	return angka, nil
}
