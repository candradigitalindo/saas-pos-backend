package repositories

import (
	"strings"

	"gorm.io/gorm/clause"
)

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

// onConflictDoNothing membuat klausa "ON CONFLICT (cols) DO NOTHING" untuk
// insert idempoten pada tabel dengan unique/primary key di kolom-kolom tsb.
func onConflictDoNothing(cols ...string) clause.OnConflict {
	columns := make([]clause.Column, len(cols))
	for i, c := range cols {
		columns[i] = clause.Column{Name: c}
	}
	return clause.OnConflict{Columns: columns, DoNothing: true}
}

// lockForUpdate mengembalikan klausa "FOR UPDATE" untuk mengunci baris terpilih
// sampai akhir transaksi. Dipakai penomoran struk & penguncian saldo stok.
func lockForUpdate() clause.Locking {
	return clause.Locking{Strength: "UPDATE"}
}
