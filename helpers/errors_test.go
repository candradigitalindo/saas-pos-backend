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
