package tests

import (
	"testing"
	"time"

	"candra/backend-api/internal/timez"
)

// konfigurasiUjiHariUsaha memilih zona waktu outlet + jam tutup buku yang
// MENJAMIN hari usaha outlet berbeda dari tanggal UTC, berapa pun jam tes
// dijalankan.
//
// Kenapa perlu dipilih, bukan dipatok? Tes yang membuktikan "business_date
// dihitung dari zona outlet, bukan UTC" tidak membuktikan apa pun bila kedua
// tanggal itu kebetulan sama. Jadi tesnya butuh keadaan di mana keduanya pasti
// beda — dan keadaan itu berubah sepanjang hari.
//
// Versi sebelumnya memakai satu resep tetap: "geser jam tutup buku ke satu jam
// setelah waktu Jakarta sekarang", sehingga sekarang jatuh di hari usaha
// sebelumnya. Resep itu justru BATAL antara 00:00–07:00 WIB, karena di jam-jam
// itu tanggal Jakarta sudah satu hari di depan tanggal UTC — digeser mundur satu
// hari usaha, ia mendarat kembali tepat di tanggal UTC. Tesnya gagal tiap pagi.
//
// Aturan yang dipakai timez.BusinessDate adalah GESER MUNDUR:
//
//	business_date = tanggal( waktu_lokal − jam_tutup_buku )
//
// Dari situ ada dua keadaan, dan keduanya cukup:
//
//	UTC ≥ 15:00 → Asia/Jayapura (+9) sudah lewat tengah malam, jadi tanggal
//	              lokalnya = tanggal UTC + 1. Jam tutup buku 00:00 (tanpa
//	              geseran) membuat hari usaha = tanggal UTC + 1. Beda. ✓
//
//	UTC < 15:00 → Asia/Jakarta (+7) berada di 07:00–22:00 pada tanggal UTC yang
//	              sama. Jam tutup buku 23:59 menggeser mundur hampir sehari
//	              penuh, jadi hari usaha = tanggal UTC − 1. Beda. ✓
//
// Batas 15:00 UTC dipilih karena di situlah Jayapura tepat berganti hari.
func konfigurasiUjiHariUsaha(now time.Time) (zona, jamTutupBuku string) {
	if now.UTC().Hour() >= 15 {
		return "Asia/Jayapura", "00:00"
	}
	return "Asia/Jakarta", "23:59"
}

// Menyapu SETIAP MENIT dalam sehari. Bug yang diperbaiki di sini hanya muncul
// pada rentang jam tertentu, jadi memeriksa "jam sekarang" saja persis cara
// bug itu lolos pertama kali.
func TestKonfigurasiUjiHariUsahaBerlakuSepanjangHari(t *testing.T) {
	// Tanggal sembarang; yang diuji jamnya, bukan harinya.
	awal := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)

	for menit := 0; menit < 24*60; menit++ {
		now := awal.Add(time.Duration(menit) * time.Minute)
		zona, jam := konfigurasiUjiHariUsaha(now)

		durasi, err := timez.ParseDayStart(jam)
		if err != nil {
			t.Fatalf("%s: jam tutup buku %q tidak terbaca: %v", now.Format(time.RFC3339), jam, err)
		}

		bd, err := timez.BusinessDate(now, zona, durasi)
		if err != nil {
			t.Fatalf("%s: business date: %v", now.Format(time.RFC3339), err)
		}

		utc := now.Format("2006-01-02")
		if bd.Format("2006-01-02") == utc {
			t.Fatalf("pukul %s UTC: hari usaha (%s) SAMA dengan tanggal UTC (%s) "+
				"memakai zona %s jam tutup buku %s — tes business_date jadi tidak membuktikan apa pun",
				now.Format("15:04"), bd.Format("2006-01-02"), utc, zona, jam)
		}
	}
}
