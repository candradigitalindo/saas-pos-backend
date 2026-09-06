package helpers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

// fieldFromConstraint menebak nama field dari error unique constraint PostgreSQL.
//
// GORM menamai constraint dengan pola "<prefix>_<tabel>_<kolom>", mis.
// "uni_users_username". Mengambil potongan terakhir setelah "_" saja tidak cukup:
// untuk "uni_products_sku_code" hasilnya "code", padahal kolomnya "sku_code".
// Karena itu prefix "<prefix>_<tabel>_" dipangkas terlebih dahulu.
func fieldFromConstraint(pgErr *pgconn.PgError) string {
	// PostgreSQL kadang menyertakan nama kolom secara langsung.
	if pgErr.ColumnName != "" {
		return pgErr.ColumnName
	}

	constraint := pgErr.ConstraintName
	if constraint == "" {
		return "field"
	}

	// Pangkas "<prefix>_<tabel>_" bila nama tabel diketahui, mis.
	// "uni_users_username" + tabel "users" -> "username".
	if pgErr.TableName != "" {
		if idx := strings.Index(constraint, "_"+pgErr.TableName+"_"); idx != -1 {
			if field := constraint[idx+len(pgErr.TableName)+2:]; field != "" {
				return field
			}
		}
	}

	// Fallback: ambil potongan terakhir.
	if parts := strings.Split(constraint, "_"); len(parts) > 1 {
		return parts[len(parts)-1]
	}
	return "field"
}

// TranslateErrorMessage menerjemahkan error dari validator atau database (GORM/PostgreSQL)
// menjadi map[string]string yang lebih ramah pengguna untuk response API.
func TranslateErrorMessage(err error) map[string]string {
	pesanError := make(map[string]string)

	// 1. Handle error dari go-playground/validator/v10
	var validationErrs validator.ValidationErrors
	if errors.As(err, &validationErrs) {
		for _, fieldErr := range validationErrs {
			field := fieldErr.Field() // Gunakan nama field langsung dari validator (yang sudah membaca tag 'json')
			switch fieldErr.Tag() {
			case "required":
				pesanError[field] = fmt.Sprintf("%s wajib diisi", field)
			case "email":
				pesanError[field] = "Format email tidak valid"
			case "min":
				pesanError[field] = fmt.Sprintf("%s minimal %s karakter", field, fieldErr.Param())
			case "max":
				pesanError[field] = fmt.Sprintf("%s maksimal %s karakter", field, fieldErr.Param())
			case "numeric":
				pesanError[field] = fmt.Sprintf("%s harus berupa angka", field)
			default:
				pesanError[field] = fmt.Sprintf("Input untuk %s tidak valid", field)
			}
		}
		return pesanError
	}

	// 2. Handle error spesifik dari PostgreSQL
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		// Kode '23505' adalah unique_violation untuk PostgreSQL
		if pgErr.Code == "23505" {
			field := fieldFromConstraint(pgErr)
			pesanError[field] = fmt.Sprintf("%s sudah terdaftar", field)
			return pesanError
		}
		// Kode PostgreSQL lain (mis. '23503' foreign_key_violation) tidak
		// diterjemahkan per field. Jangan return map kosong di sini — biarkan
		// jatuh ke fallback di langkah 5 agar klien tetap menerima pesan error.
	}

	// 3. Handle error umum dari GORM
	if errors.Is(err, gorm.ErrRecordNotFound) {
		pesanError["error"] = "Data tidak ditemukan"
		return pesanError
	}

	// 4. Handle JSON binding errors
	var jsonErr *json.SyntaxError
	if errors.As(err, &jsonErr) {
		pesanError["error"] = fmt.Sprintf("Request body mengandung format JSON yang tidak valid pada karakter %d", jsonErr.Offset)
		return pesanError
	}
	if errors.Is(err, io.EOF) {
		pesanError["error"] = "Request body tidak boleh kosong"
		return pesanError
	}
	var unmarshalTypeErr *json.UnmarshalTypeError
	if errors.As(err, &unmarshalTypeErr) {
		pesanError[unmarshalTypeErr.Field] = fmt.Sprintf("Tipe data tidak valid untuk field '%s', seharusnya '%s'", unmarshalTypeErr.Field, unmarshalTypeErr.Type.String())
		return pesanError
	}

	// 5. Fallback untuk error lainnya
	if err != nil {
		pesanError["error"] = "Terjadi kesalahan internal"
	}

	return pesanError
}

// IsDuplicateEntryError mendeteksi error duplikasi entri pada database secara andal
// dengan memeriksa kode error spesifik PostgreSQL ('23505').
func IsDuplicateEntryError(err error) bool {
	var pgErr *pgconn.PgError
	// errors.As akan mencari error dengan tipe *pgconn.PgError di dalam chain error
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505" // 23505 = unique_violation
	}
	return false
}
