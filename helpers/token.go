package helpers

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

// refreshTokenBytes adalah panjang entropi refresh token mentah sebelum
// di-encode. 32 byte = 256 bit: tidak bisa ditebak, dan tidak perlu lebih.
const refreshTokenBytes = 32

// GenerateRefreshToken membuat refresh token acak.
//
// Mengembalikan:
//   - raw:  nilai yang DIKIRIM ke klien sekali saja. Klien menyimpannya dan
//     mengirimkannya kembali saat menukar token.
//   - hash: SHA-256 hex dari raw. HANYA ini yang disimpan di tabel
//     refresh_tokens — bila tabel bocor, token tetap tak berguna (§5.3).
//
// SHA-256 (bukan bcrypt) memang disengaja: nilainya sudah berentropi tinggi dan
// acak penuh, jadi tidak rentan brute-force kamus; yang dibutuhkan hanyalah
// pencarian cepat dan setara-waktu lewat index unik pada kolom hash.
func GenerateRefreshToken() (raw, hash string, err error) {
	buf := make([]byte, refreshTokenBytes)
	if _, err = rand.Read(buf); err != nil {
		return "", "", err
	}
	raw = base64.RawURLEncoding.EncodeToString(buf)
	return raw, HashRefreshToken(raw), nil
}

// HashRefreshToken menghitung SHA-256 hex dari refresh token mentah. Dipakai
// saat menukar/mencabut token: cari baris berdasarkan hash ini.
func HashRefreshToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
