package helpers

import "errors"

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
	default:
		return 500
	}
}
