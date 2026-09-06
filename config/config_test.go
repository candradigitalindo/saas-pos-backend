package config

import (
	"os"
	"testing"
)

// Variabel yang di-set tapi kosong harus diperlakukan seperti tidak di-set,
// agar baris kosong di .env tidak menimpa nilai default.
func TestGetEnvTreatsEmptyAsUnset(t *testing.T) {
	t.Setenv("TEST_KEY", "")
	if got := GetEnv("TEST_KEY", "default"); got != "default" {
		t.Fatalf("GetEnv = %q, ingin %q", got, "default")
	}
	t.Setenv("TEST_KEY", "  spasi  ")
	if got := GetEnv("TEST_KEY", "default"); got != "spasi" {
		t.Fatalf("GetEnv tidak melakukan trim: %q", got)
	}
}

func TestGetStringSliceEnv(t *testing.T) {
	cases := []struct {
		name  string
		value string
		set   bool
		want  int
	}{
		{"tidak di-set", "", false, 2},
		{"di-set tapi kosong", "", true, 2},
		{"ada entri kosong", "https://a.com, ,https://b.com,", true, 2},
		{"satu nilai", "https://a.com", true, 1},
	}
	def := []string{"http://localhost:5173", "http://localhost:3000"}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			os.Unsetenv("TEST_LIST")
			if tc.set {
				t.Setenv("TEST_LIST", tc.value)
			}
			got := GetStringSliceEnv("TEST_LIST", def)
			if len(got) != tc.want {
				t.Fatalf("len = %d, ingin %d (%#v)", len(got), tc.want, got)
			}
			for _, v := range got {
				if v == "" {
					t.Fatalf("ada entri kosong: %#v", got)
				}
			}
		})
	}
}

func TestGetBoolEnv(t *testing.T) {
	t.Setenv("TEST_BOOL", "false")
	if GetBoolEnv("TEST_BOOL", true) {
		t.Fatal("ingin false")
	}
	t.Setenv("TEST_BOOL", "bukan-bool")
	if !GetBoolEnv("TEST_BOOL", true) {
		t.Fatal("nilai tidak valid harus jatuh ke default")
	}
}
