package helpers

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Sentinel error lintas-lapisan (docs/TECHNICAL-BACKEND.md §7).
//
// Service mengembalikan salah satu dari ini (dibungkus dengan fmt.Errorf("%w",
// ...) bila perlu konteks); controller memetakannya ke kode HTTP lewat
// StatusForError. Detail internal TIDAK pernah dikirim ke klien — pesan yang
// dikirim adalah pesan yang sudah disiapkan di sini, detailnya masuk log.
var (
	ErrNotFound     = errors.New("data tidak ditemukan")
	ErrConflict     = errors.New("data bentrok")
	ErrInsufficient = errors.New("stok tidak mencukupi")
	ErrForbidden    = errors.New("tidak punya akses")
	ErrValidation   = errors.New("input tidak valid")
	ErrUnauthorized = errors.New("kredensial tidak valid")
)

// StatusForError memetakan sentinel di atas ke kode status HTTP. Error yang tidak
// dikenali → 500 (dan pemanggil sebaiknya mencatat detailnya ke log).
func StatusForError(err error) int {
	switch {
	case err == nil:
		return 200
	case errors.Is(err, ErrNotFound):
		return 404
	case errors.Is(err, ErrConflict):
		return 409
	case errors.Is(err, ErrValidation):
		return 422
	case errors.Is(err, ErrInsufficient):
		return 409
	case errors.Is(err, ErrForbidden):
		return 403
	case errors.Is(err, ErrUnauthorized):
		return 401
	default:
		return 500
	}
}

// PesanUntukPengguna mengubah error layanan menjadi kalimat yang layak tampil
// di layar: awalan sentinel dibuang dan huruf pertamanya dikapitalkan.
//
// Layanan membungkus sentinel dengan konteks —
// fmt.Errorf("%w: username atau email sudah terpakai", ErrConflict) — sehingga
// err.Error() berbunyi "data bentrok: username atau email sudah terpakai".
// Awalan itu label internal untuk memetakan kode status, bukan bahasa orang
// (ui/01 §2), tapi dulu ikut terkirim apa adanya dan tampil di layar pendaftaran,
// riwayat penjualan, kasbon, dan halaman "perlu diperiksa".
func PesanUntukPengguna(err error) string {
	if err == nil {
		return ""
	}
	pesan := err.Error()
	for _, s := range []error{ErrNotFound, ErrConflict, ErrInsufficient, ErrForbidden, ErrValidation, ErrUnauthorized} {
		if errors.Is(err, s) && strings.HasPrefix(pesan, s.Error()+": ") {
			pesan = strings.TrimPrefix(pesan, s.Error()+": ")
			break
		}
	}
	r, ukuran := utf8.DecodeRuneInString(pesan)
	if r == utf8.RuneError {
		return pesan
	}
	return string(unicode.ToUpper(r)) + pesan[ukuran:]
}
