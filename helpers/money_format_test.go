package helpers

import "testing"

func TestFormatRupiah(t *testing.T) {
	kasus := []struct {
		in   int64
		want string
	}{
		{0, "Rp 0"},
		{5, "Rp 5"},
		{999, "Rp 999"},
		{1000, "Rp 1.000"},
		{45000, "Rp 45.000"},
		{1250000, "Rp 1.250.000"},
		{789684, "Rp 789.684"},
		{1000000000, "Rp 1.000.000.000"},
		{-45000, "-Rp 45.000"},
	}
	for _, k := range kasus {
		if got := FormatRupiah(k.in); got != k.want {
			t.Errorf("FormatRupiah(%d) = %q, mau %q", k.in, got, k.want)
		}
	}
}
