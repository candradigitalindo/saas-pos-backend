package database

import (
	"log/slog"

	"candra/backend-api/models"

	"gorm.io/gorm/clause"
)

// SeedData menjalankan semua seeder data awal. Aman dijalankan berulang
// (idempoten). Tidak fatal bila gagal: kegagalan paling umum adalah tabel belum
// ada karena migrasi belum dijalankan — dan itu sudah diperingatkan terpisah
// oleh WarnIfMigrationsPending. Server tetap boleh naik agar /health bisa
// dipakai untuk diagnosa.
func SeedData() {
	if err := seedRoles(); err != nil {
		slog.Error("seeder roles gagal — sudah menjalankan `go run ./cmd/migrate up`?", slog.Any("error", err))
		return
	}
	slog.Info("seeding database selesai")
}

// seedRoles memastikan peran default ('admin', 'user') ada.
//
// ON CONFLICT (name) WHERE deleted_at IS NULL DO NOTHING dipilih agar:
//   - idempoten & bebas race saat beberapa instance start bersamaan;
//   - cocok dengan partial unique index uq_roles_name (yang juga ber-predikat
//     deleted_at IS NULL) — ON CONFLICT tanpa predikat akan ditolak PostgreSQL;
//   - satu query untuk semua role, bukan satu per role.
//
// ID di-generate hook BeforeCreate tiap model.
func seedRoles() error {
	roles := []models.Role{
		{Name: "admin"},
		{Name: "user"},
	}

	return DB.Clauses(clause.OnConflict{
		Columns:     []clause.Column{{Name: "name"}},
		TargetWhere: clause.Where{Exprs: []clause.Expression{clause.Expr{SQL: "deleted_at IS NULL"}}},
		DoNothing:   true,
	}).Create(&roles).Error
}
