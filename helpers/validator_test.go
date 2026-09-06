package helpers

import (
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestTranslateErrorMessagePostgres(t *testing.T) {
	// Error PostgreSQL selain 23505 tidak boleh menghasilkan map kosong.
	fk := TranslateErrorMessage(&pgconn.PgError{Code: "23503", ConstraintName: "fk_users_role"})
	if len(fk) == 0 {
		t.Fatal("error 23503 menghasilkan map kosong")
	}

	// Nama kolom yang mengandung underscore harus utuh, bukan terpotong.
	cases := []struct {
		constraint string
		table      string
		wantField  string
	}{
		{"uni_users_username", "users", "username"},
		{"uni_products_sku_code", "products", "sku_code"},
		{"uni_roles_name", "roles", "name"},
	}
	for _, tc := range cases {
		got := TranslateErrorMessage(&pgconn.PgError{
			Code: "23505", ConstraintName: tc.constraint, TableName: tc.table,
		})
		if _, ok := got[tc.wantField]; !ok {
			t.Errorf("%s -> %#v, ingin field %q", tc.constraint, got, tc.wantField)
		}
	}
}
