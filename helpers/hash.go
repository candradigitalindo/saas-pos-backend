package helpers

import (
	"candra/backend-api/config"

	"golang.org/x/crypto/bcrypt"
)

// DummyPasswordHash adalah hash bcrypt valid dari string acak, dipakai untuk
// menjalankan CheckPassword saat user tidak ditemukan agar waktu respon login
// tetap konstan (mencegah timing-based user enumeration).
const DummyPasswordHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"

// HashPassword mengubah password teks biasa menjadi hash bcrypt yang aman.
// Cost factor diambil dari environment variable untuk fleksibilitas.
func HashPassword(password string) (string, error) {
	// Ambil cost dari .env, gunakan bcrypt.DefaultCost (10) jika tidak diset.
	// Ini memungkinkan kita meningkatkan keamanan di masa depan tanpa mengubah kode.
	cost := config.GetIntEnv("BCRYPT_COST", bcrypt.DefaultCost)
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	return string(hashed), err
}

// CheckPassword membandingkan password teks biasa dengan hash yang ada.
func CheckPassword(password, hashed string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashed), []byte(password))
}
