// Package ulid adalah satu-satunya sumber pembuatan dan validasi ULID di seluruh
// backend. Semua primary key tabel bertipe CHAR(26) dan berisi ULID (lihat
// docs/TECHNICAL-BACKEND.md §3.1).
//
// Kenapa dibungkus, bukan memakai github.com/oklog/ulid/v2 langsung di mana-mana:
//
//   - Satu titik ganti bila implementasi/entropi perlu diubah.
//   - Validasi bentuk ID dari klien (mode offline) harus konsisten — kasir boleh
//     membuat ID transaksi sendiri, jadi server WAJIB memvalidasinya sebelum
//     percaya. Aturan validasi tidak boleh tersebar dan berbeda-beda.
//   - Entropi yang aman untuk dipakai banyak goroutine sekaligus disiapkan sekali
//     di sini, bukan diulang di tiap pemanggil.
package ulid

import (
	"crypto/rand"
	"sync"
	"time"

	oklid "github.com/oklog/ulid/v2"
)

// Len adalah panjang tetap sebuah ULID dalam representasi teks Base32 Crockford.
const Len = 26

// entropy adalah sumber acak untuk komponen 80-bit ULID.
//
// oklid.Monotonic membungkus crypto/rand sehingga dua ULID yang dibuat pada
// milidetik yang sama tetap terurut menaik (bit acak dinaikkan, bukan diacak
// ulang). Ini penting supaya "ORDER BY id" setara dengan "ORDER BY created_at"
// untuk baris yang dibuat server — dipakai pada paginasi kursor sinkronisasi.
//
// oklid.Monotonic tidak aman dipakai dari banyak goroutine, karena itu setiap
// pemanggilan New() menyalurkannya lewat lock pada level paket (lihat New).
var entropy = oklid.Monotonic(rand.Reader, 0)

// mu menjaga entropy agar aman dipanggil dari banyak goroutine sekaligus.
// Alternatif tanpa lock (pool entropi per goroutine) tidak sepadan: pembuatan
// ULID bukan jalur panas, dan biaya lock di sini terukur dalam nanodetik.
var mu sync.Mutex

// New membuat ULID baru memakai waktu sekarang (UTC) dan entropi kriptografis.
// Dipakai server untuk seluruh entitas master & platform, biasanya lewat hook
// GORM BeforeCreate.
//
// Fungsi ini tidak pernah gagal dalam praktik: satu-satunya sumber error dari
// library adalah kehabisan entropi monotonic dalam 1 ms yang sama (lebih dari
// 2^80 ID), yang tidak mungkin terjadi. Bila itu tetap terjadi, kode jatuh ke
// oklid.MustNew yang meng-acak ulang tanpa jaminan monotonic — lebih baik ID
// yang sedikit tidak terurut daripada panik di jalur penulisan.
func New() string {
	t := oklid.Timestamp(time.Now().UTC())

	mu.Lock()
	id, err := oklid.New(t, entropy)
	mu.Unlock()

	if err != nil {
		return oklid.MustNew(t, rand.Reader).String()
	}
	return id.String()
}

// Parse mengurai teks ULID menjadi nilai oklid.ULID. Mengembalikan error bila
// panjang atau alfabetnya salah. Dipakai saat butuh komponen waktu/entropi,
// bukan sekadar memeriksa bentuk (untuk itu pakai IsValid).
func Parse(s string) (oklid.ULID, error) {
	return oklid.ParseStrict(s)
}

// IsValid melaporkan apakah s berbentuk ULID yang sah: tepat 26 karakter,
// seluruhnya dalam alfabet Base32 Crockford, dan tidak melebihi batas waktu ULID.
//
// Ini pemeriksaan yang dipakai di lapisan request untuk ID kiriman klien
// (validator tag `ulid`). ID dari klien TIDAK BOLEH dipercaya bentuknya —
// lihat docs/TECHNICAL-BACKEND.md §3.1.
func IsValid(s string) bool {
	if len(s) != Len {
		return false
	}
	_, err := oklid.ParseStrict(s)
	return err == nil
}

// TimeOf mengembalikan stempel waktu pembuatan yang tertanam di 48 bit pertama
// ULID, dalam UTC. Berguna untuk diagnosa ("kapan transaksi ini dibuat di
// perangkat"), BUKAN untuk otorisasi: ULID bukan token dan komponen waktunya
// bukan rahasia.
func TimeOf(s string) (time.Time, error) {
	id, err := oklid.ParseStrict(s)
	if err != nil {
		return time.Time{}, err
	}
	return oklid.Time(id.Time()).UTC(), nil
}
