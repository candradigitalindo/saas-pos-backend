package helpers

import (
	"errors"
	"log"
	"time"

	"candra/backend-api/config"
	"candra/backend-api/internal/ulid"

	"github.com/golang-jwt/jwt/v5"
)

// jwtKey menyimpan secret key untuk menandatangani & memverifikasi token.
var jwtKey []byte

// tokenTypeAccess adalah nilai klaim `typ` untuk access token. Memberi label
// jenis token mencegah sebuah token dipakai di tempat yang bukan peruntukannya.
const tokenTypeAccess = "access"

// ErrWrongTokenType dikembalikan ParseAccessToken bila token sah secara tanda
// tangan tetapi bukan access token.
var ErrWrongTokenType = errors.New("jenis token tidak sesuai")

// AccessClaims adalah isi access token JWT.
//
//   - Subject (sub): USER ID (ULID), bukan username — username bisa berubah,
//     ID tidak (§4 CONVENTIONS).
//   - ID (jti): ULID unik per token, untuk korelasi log dan — nanti — daftar
//     pencabutan.
//   - Typ: selalu "access". TIDAK ADA tenant_id atau daftar permission di sini;
//     keduanya dibaca dari database setiap permintaan (§9).
type AccessClaims struct {
	Typ string `json:"typ"`
	jwt.RegisteredClaims
}

// InitJWT harus dipanggil di main.go setelah LoadEnv().
// Aplikasi SENGAJA berhenti (fatal) bila JWT_SECRET tidak diset atau terlalu
// pendek, agar tidak ada fallback secret yang bisa ditebak dari kode sumber
// (§4 "fail fast, jangan fallback").
func InitJWT() {
	secret := config.GetEnv("JWT_SECRET", "")
	if len(secret) < 32 {
		log.Fatal("Konfigurasi tidak aman: JWT_SECRET wajib diset dan minimal 32 karakter")
	}
	jwtKey = []byte(secret)
}

// AccessTokenTTL mengembalikan umur access token dari JWT_ACCESS_MINUTES
// (default 15 menit).
func AccessTokenTTL() time.Duration {
	return time.Duration(config.GetIntEnv("JWT_ACCESS_MINUTES", 15)) * time.Minute
}

// RefreshTokenTTL mengembalikan umur refresh token dari JWT_REFRESH_DAYS
// (default 30 hari).
func RefreshTokenTTL() time.Duration {
	return time.Duration(config.GetIntEnv("JWT_REFRESH_DAYS", 30)) * 24 * time.Hour
}

// GenerateAccessToken membuat access token berumur pendek untuk userID.
// Mengembalikan token yang sudah ditandatangani beserta waktu kedaluwarsanya
// (dipakai controller untuk mengisi field expires_in di response).
func GenerateAccessToken(userID string) (token string, expiresAt time.Time, err error) {
	now := time.Now()
	expiresAt = now.Add(AccessTokenTTL())

	claims := AccessClaims{
		Typ: tokenTypeAccess,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			ID:        ulid.New(),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}

	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(jwtKey)
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, expiresAt, nil
}

// ParseAccessToken memverifikasi tanda tangan dan masa berlaku token, memaksa
// algoritma HS256 (mencegah serangan downgrade/`alg=none`), dan menolak token
// yang klaim `typ`-nya bukan "access".
//
// Error kedaluwarsa tetap bisa dideteksi pemanggil dengan
// errors.Is(err, jwt.ErrTokenExpired).
func ParseAccessToken(tokenStr string) (*AccessClaims, error) {
	claims := &AccessClaims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("metode signing tidak terduga")
		}
		return jwtKey, nil
	})
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, jwt.ErrTokenInvalidClaims
	}
	if claims.Typ != tokenTypeAccess {
		return nil, ErrWrongTokenType
	}
	return claims, nil
}

// GetJWTKey mengembalikan kunci JWT yang sudah diinisialisasi.
// Dipertahankan untuk kompatibilitas; kode baru sebaiknya lewat ParseAccessToken.
func GetJWTKey() []byte {
	return jwtKey
}
