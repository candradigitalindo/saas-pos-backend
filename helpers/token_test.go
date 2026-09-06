package helpers

import "testing"

// TestGenerateRefreshTokenUnique memastikan setiap token acak dan hash-nya
// konsisten dengan HashRefreshToken.
func TestGenerateRefreshTokenUnique(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 500; i++ {
		raw, hash, err := GenerateRefreshToken()
		if err != nil {
			t.Fatalf("GenerateRefreshToken() error: %v", err)
		}
		if raw == "" || hash == "" {
			t.Fatal("raw/hash kosong")
		}
		if seen[raw] {
			t.Fatalf("token mentah berulang: %q", raw)
		}
		seen[raw] = true

		if got := HashRefreshToken(raw); got != hash {
			t.Fatalf("HashRefreshToken(raw) = %q, tidak cocok dengan hash saat generate %q", got, hash)
		}
	}
}

// TestHashRefreshTokenStableAndSensitive memastikan hash deterministik untuk
// input sama dan berbeda untuk input berbeda (SHA-256 hex = 64 karakter).
func TestHashRefreshTokenStableAndSensitive(t *testing.T) {
	a1 := HashRefreshToken("token-abc")
	a2 := HashRefreshToken("token-abc")
	b := HashRefreshToken("token-abd")

	if a1 != a2 {
		t.Fatal("hash tidak deterministik untuk input yang sama")
	}
	if a1 == b {
		t.Fatal("hash sama untuk input berbeda")
	}
	if len(a1) != 64 {
		t.Fatalf("panjang hash = %d, mau 64 (sha256 hex)", len(a1))
	}
}
