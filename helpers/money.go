package helpers

import (
	"strconv"
	"strings"

	"github.com/shopspring/decimal"
)

// Aturan uang (docs/TECHNICAL-BACKEND.md §3.3): rupiah bulat (int64), hitung &
// bulatkan PER BARIS (setengah ke atas), lalu jumlahkan baris. Jangan pernah
// menghitung total dari persentase agregat.

// RoundHalfUpToInt membulatkan desimal ke bilangan bulat terdekat, dengan .5
// dibulatkan menjauhi nol ("setengah ke atas" untuk nilai positif).
func RoundHalfUpToInt(d decimal.Decimal) int64 {
	return d.Round(0).IntPart()
}

// LineAmount menghitung qty * unitPrice lalu membulatkannya ke rupiah bulat.
// Dipakai untuk nilai kotor per baris (harga jual maupun harga modal).
func LineAmount(qty decimal.Decimal, unitPrice int64) int64 {
	return RoundHalfUpToInt(qty.Mul(decimal.NewFromInt(unitPrice)))
}

// ApplyRate mengalikan base (rupiah bulat) dengan rate pecahan (mis. 0.11) lalu
// membulatkan. Untuk pajak eksklusif & service charge.
func ApplyRate(base int64, rate decimal.Decimal) int64 {
	return RoundHalfUpToInt(decimal.NewFromInt(base).Mul(rate))
}

// InclusiveTax mengeluarkan komponen pajak yang SUDAH termasuk di dalam
// grossWithTax, untuk tarif `rate`:  tax = gross - round(gross / (1 + rate)).
func InclusiveTax(grossWithTax int64, rate decimal.Decimal) int64 {
	if rate.IsZero() {
		return 0
	}
	divisor := decimal.NewFromInt(1).Add(rate)
	net := RoundHalfUpToInt(decimal.NewFromInt(grossWithTax).Div(divisor))
	return grossWithTax - net
}

// FormatRupiah memformat rupiah bulat menjadi teks siap tampil: 1250000 →
// "Rp 1.250.000". Pemisah ribuan titik, tanpa desimal — sen tidak dipakai.
//
// Dipakai untuk teks yang dilihat manusia (notifikasi, struk). Perhitungan
// TETAP memakai int64; fungsi ini hanya lapisan tampilan.
func FormatRupiah(v int64) string {
	tanda := ""
	if v < 0 {
		tanda = "-"
		v = -v
	}
	digit := strconv.FormatInt(v, 10)

	var b strings.Builder
	b.WriteString(tanda)
	b.WriteString("Rp ")
	for i, r := range digit {
		if i > 0 && (len(digit)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(r)
	}
	return b.String()
}
