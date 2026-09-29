package rahasia

import (
	"bytes"
	"testing"
)

func TestTutupBukaBolakBalik(t *testing.T) {
	t.Setenv("JWT_SECRET", "rahasia-uji-yang-cukup-panjang-32-karakter")
	polos := []byte(`{"access_token":"EAAG-uji"}`)
	a, err := Tutup(polos)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(a, []byte("EAAG")) {
		t.Fatal("sandi masih memuat teks aslinya")
	}
	b, _ := Tutup(polos)
	if bytes.Equal(a, b) {
		t.Fatal("dua kali Tutup menghasilkan sandi sama — nonce tidak acak")
	}
	hasil, err := Buka(a)
	if err != nil || !bytes.Equal(hasil, polos) {
		t.Fatalf("Buka = %q, %v", hasil, err)
	}
}

func TestBukaMenolakSandiDiubah(t *testing.T) {
	t.Setenv("JWT_SECRET", "rahasia-uji-yang-cukup-panjang-32-karakter")
	a, _ := Tutup([]byte("token"))
	a[len(a)-1] ^= 1
	if _, err := Buka(a); err != ErrRusak {
		t.Fatalf("sandi diubah harus ditolak, dapat %v", err)
	}
	if _, err := Buka([]byte{9, 9, 9}); err != ErrRusak {
		t.Fatalf("sandi pendek harus ditolak, dapat %v", err)
	}
}
