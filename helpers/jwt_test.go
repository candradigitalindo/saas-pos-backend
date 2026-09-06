package helpers

import (
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// setTestSecret memasang JWT_SECRET sementara untuk satu test lalu memulihkannya.
func setTestSecret(t *testing.T) {
	t.Helper()
	t.Setenv("JWT_SECRET", "test-secret-yang-cukup-panjang-32chars!!")
	InitJWT()
}

// TestGenerateAndParseAccessToken menguji jalur normal: token yang dibuat bisa
// diverifikasi kembali, membawa subject yang benar, dan punya jti + typ.
func TestGenerateAndParseAccessToken(t *testing.T) {
	setTestSecret(t)

	const uid = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	token, expiresAt, err := GenerateAccessToken(uid)
	if err != nil {
		t.Fatalf("GenerateAccessToken() error: %v", err)
	}
	if time.Until(expiresAt) <= 0 || time.Until(expiresAt) > AccessTokenTTL()+time.Minute {
		t.Fatalf("expiresAt tidak masuk akal: %v", expiresAt)
	}

	claims, err := ParseAccessToken(token)
	if err != nil {
		t.Fatalf("ParseAccessToken() error: %v", err)
	}
	if claims.Subject != uid {
		t.Fatalf("subject = %q, mau %q", claims.Subject, uid)
	}
	if claims.Typ != tokenTypeAccess {
		t.Fatalf("typ = %q, mau %q", claims.Typ, tokenTypeAccess)
	}
	if claims.ID == "" {
		t.Fatal("jti kosong")
	}
}

// TestParseAccessTokenRejectsWrongSecret memastikan token yang ditandatangani
// kunci lain ditolak.
func TestParseAccessTokenRejectsWrongSecret(t *testing.T) {
	setTestSecret(t)
	token, _, err := GenerateAccessToken("01ARZ3NDEKTSV4RRFFQ69G5FAV")
	if err != nil {
		t.Fatalf("GenerateAccessToken() error: %v", err)
	}

	t.Setenv("JWT_SECRET", "kunci-lain-yang-sama-sama-32-karakter!!!")
	InitJWT()

	if _, err := ParseAccessToken(token); err == nil {
		t.Fatal("token dengan secret berbeda seharusnya ditolak")
	}
}

// TestParseAccessTokenRejectsNonAccessType memastikan token dengan typ selain
// "access" ditolak walau tanda tangannya sah.
func TestParseAccessTokenRejectsNonAccessType(t *testing.T) {
	setTestSecret(t)

	claims := AccessClaims{
		Typ: "refresh",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "01ARZ3NDEKTSV4RRFFQ69G5FAV",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(GetJWTKey())
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	if _, err := ParseAccessToken(signed); err != ErrWrongTokenType {
		t.Fatalf("error = %v, mau ErrWrongTokenType", err)
	}
}

// TestParseAccessTokenRejectsExpired memastikan token kedaluwarsa terdeteksi
// lewat errors.Is(err, jwt.ErrTokenExpired).
func TestParseAccessTokenRejectsExpired(t *testing.T) {
	setTestSecret(t)

	claims := AccessClaims{
		Typ: tokenTypeAccess,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "01ARZ3NDEKTSV4RRFFQ69G5FAV",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Minute)),
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(GetJWTKey())
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	_, err = ParseAccessToken(signed)
	if err == nil {
		t.Fatal("token kedaluwarsa seharusnya ditolak")
	}
	if !errors.Is(err, jwt.ErrTokenExpired) {
		t.Fatalf("error kedaluwarsa tidak terdeteksi lewat errors.Is: %v", err)
	}
}
