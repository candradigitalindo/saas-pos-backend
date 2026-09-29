package helpers

import (
	"errors"
	"fmt"
	"testing"
)

func TestPesanUntukPengguna(t *testing.T) {
	kasus := []struct {
		err error
		mau string
	}{
		{fmt.Errorf("%w: username atau email sudah terpakai", ErrConflict), "Username atau email sudah terpakai"},
		{fmt.Errorf("%w: diskon transaksi melebihi total", ErrValidation), "Diskon transaksi melebihi total"},
		{fmt.Errorf("%w: Anda tidak punya akses ke outlet ini", ErrForbidden), "Anda tidak punya akses ke outlet ini"},
		{fmt.Errorf("%w: QRIS tersedia mulai paket Basic", ErrPlanRequired), "QRIS tersedia mulai paket Basic"},
		{ErrNotFound, "Data tidak ditemukan"},                         // sentinel tanpa konteks
		{fmt.Errorf("gagal: %w", ErrConflict), "Gagal: data bentrok"}, // konteks di depan: tidak dipotong
		{errors.New("pesan biasa"), "Pesan biasa"},
		{nil, ""},
	}
	for _, k := range kasus {
		if dapat := PesanUntukPengguna(k.err); dapat != k.mau {
			t.Errorf("PesanUntukPengguna(%v) = %q, mau %q", k.err, dapat, k.mau)
		}
	}
}

// ErrPlanRequired → 402, bukan 403: jalan keluarnya naik paket, bukan minta
// izin peran — klien membedakan keduanya dari kode statusnya.
func TestStatusPaket(t *testing.T) {
	if got := StatusForError(fmt.Errorf("%w: x", ErrPlanRequired)); got != 402 {
		t.Fatalf("StatusForError(ErrPlanRequired) = %d, mau 402", got)
	}
}
