package services

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Nomor tujuan notifikasi diketik manusia, bukan dipilih dari daftar. Tes ini
// mengumpulkan bentuk-bentuk yang benar-benar muncul di lapangan.
func TestNormalkanNomorWA(t *testing.T) {
	kasus := []struct {
		nama  string
		masuk string
		mau   string
	}{
		{"bentuk lokal biasa", "081234567890", "6281234567890"},
		{"lokal dengan strip", "0812-3456-7890", "6281234567890"},
		{"lokal dengan spasi", "0812 3456 7890", "6281234567890"},
		{"lokal dalam kurung", "(0812) 3456-7890", "6281234567890"},
		{"plus kode negara", "+6281234567890", "6281234567890"},
		{"plus dengan spasi", "+62 812-3456-7890", "6281234567890"},
		{"kode negara tanpa plus", "6281234567890", "6281234567890"},
		{"awalan internasional lama", "006281234567890", "6281234567890"},
		{"nol hilang saat disalin dari spreadsheet", "81234567890", "6281234567890"},
		{"sudah bersih", "628123456789", "628123456789"},
		{"nomor asing dibiarkan", "+14155550123", "14155550123"},
		{"spasi di ujung", "  081234567890  ", "6281234567890"},
	}

	for _, k := range kasus {
		t.Run(k.nama, func(t *testing.T) {
			dapat, err := NormalkanNomorWA(k.masuk, KodeNegaraBawaan)
			require.NoError(t, err)
			assert.Equal(t, k.mau, dapat)
		})
	}
}

// Nomor yang bentuknya salah harus GAGAL PERMANEN. Kalau tidak, satu kesalahan
// ketik membuat antrean mencoba ulang sepuluh kali selama berjam-jam sebelum
// akhirnya menyerah — dan operator baru tahu setelah lupa konteksnya.
func TestNormalkanNomorWATolakYangCacat(t *testing.T) {
	kasus := []struct{ nama, masuk string }{
		{"kosong", ""},
		{"hanya spasi", "   "},
		{"tanpa angka sama sekali", "hubungi saya"},
		{"terlalu pendek", "0812"},
		{"terlalu panjang", "0812345678901234567"},
	}

	for _, k := range kasus {
		t.Run(k.nama, func(t *testing.T) {
			_, err := NormalkanNomorWA(k.masuk, KodeNegaraBawaan)
			require.Error(t, err)
			assert.True(t, errors.Is(err, ErrNotifPermanen),
				"nomor cacat harus permanen supaya tidak diulang sia-sia, dapat: %v", err)
		})
	}
}

// Kode negara bisa diganti — SaaS ini Indonesia, tapi aturannya tidak
// dipaku ke "62".
func TestNormalkanNomorWAKodeNegaraLain(t *testing.T) {
	dapat, err := NormalkanNomorWA("0123456789", "60") // Malaysia
	require.NoError(t, err)
	assert.Equal(t, "60123456789", dapat)
}
