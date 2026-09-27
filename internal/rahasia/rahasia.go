// Package rahasia mengenkripsi data rahasia tenant yang HARUS bisa dibaca lagi
// oleh server — kredensial API kanal (token WhatsApp, kunci Shopee, dll.).
//
// Kenapa bukan hash seperti kata sandi: server memakai kredensial ini untuk
// memanggil API penyedia atas nama tenant, jadi nilainya perlu dikembalikan
// utuh. Yang dijaga: basis data yang bocor (cadangan, salinan dev) tidak ikut
// membocorkan akses ke toko GoFood/Shopee milik pelanggan.
//
// AES-256-GCM: sekaligus rahasia dan tahan ubah — sandi yang diubah satu bit
// ditolak saat dibuka, bukan menghasilkan kredensial sampah.
//
// Kunci dari CHANNEL_SECRET_KEY (base64 32 byte: `openssl rand -base64 32`).
// Tanpa itu, kunci diturunkan dari JWT_SECRET — cukup untuk pengembangan, tapi
// artinya mengganti JWT_SECRET membuat kredensial lama tak terbaca. Set kunci
// sendiri di production.
package rahasia

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"sync"

	"candra/backend-api/config"
)

// versi1 menandai format "v1 || nonce || ciphertext" — satu byte di depan
// supaya format/kunci bisa diganti kelak tanpa menebak isi lama.
const versi1 byte = 1

// ErrRusak: sandi tidak bisa dibuka (dirusak, kunci berbeda, atau bukan buatan paket ini).
var ErrRusak = errors.New("data rahasia tidak dapat dibuka")

var (
	sekali sync.Once
	aead   cipher.AEAD
	galat  error
)

func siapkan() (cipher.AEAD, error) {
	sekali.Do(func() {
		kunci, err := kunciAktif()
		if err != nil {
			galat = err
			return
		}
		blok, err := aes.NewCipher(kunci)
		if err != nil {
			galat = err
			return
		}
		aead, galat = cipher.NewGCM(blok)
	})
	return aead, galat
}

func kunciAktif() ([]byte, error) {
	if s := config.GetEnv("CHANNEL_SECRET_KEY", ""); s != "" {
		k, err := base64.StdEncoding.DecodeString(s)
		if err != nil || len(k) != 32 {
			return nil, fmt.Errorf("CHANNEL_SECRET_KEY harus base64 dari 32 byte (openssl rand -base64 32)")
		}
		return k, nil
	}
	jwt := config.GetEnv("JWT_SECRET", "")
	if jwt == "" {
		return nil, errors.New("CHANNEL_SECRET_KEY dan JWT_SECRET kosong — tidak ada kunci enkripsi")
	}
	k := sha256.Sum256([]byte("rahasia-kanal/v1:" + jwt))
	return k[:], nil
}

// Tutup mengenkripsi data.
func Tutup(polos []byte) ([]byte, error) {
	a, err := siapkan()
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, a.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	out := make([]byte, 0, 1+len(nonce)+len(polos)+a.Overhead())
	out = append(out, versi1)
	out = append(out, nonce...)
	return a.Seal(out, nonce, polos, []byte{versi1}), nil
}

// Buka mendekripsi hasil Tutup.
func Buka(sandi []byte) ([]byte, error) {
	a, err := siapkan()
	if err != nil {
		return nil, err
	}
	n := a.NonceSize()
	if len(sandi) < 1+n+a.Overhead() || sandi[0] != versi1 {
		return nil, ErrRusak
	}
	polos, err := a.Open(nil, sandi[1:1+n], sandi[1+n:], []byte{versi1})
	if err != nil {
		return nil, ErrRusak
	}
	return polos, nil
}
