package repositories

import "strings"

// escapeLike meng-escape karakter wildcard LIKE/ILIKE ('%', '_') dan backslash
// dari input pencarian pengguna.
//
// Tanpa ini, input seperti "%" akan cocok dengan SELURUH baris di tabel dan
// memaksa full table scan yang mahal (bukan SQL injection — query tetap
// parameterized — melainkan "wildcard injection" yang berdampak ke performa).
// PostgreSQL memakai backslash sebagai escape character default untuk LIKE.
func escapeLike(search string) string {
	return strings.NewReplacer(
		`\`, `\\`,
		`%`, `\%`,
		`_`, `\_`,
	).Replace(search)
}
