package database

import (
	"log/slog"

	"candra/backend-api/models"

	"gorm.io/gorm/clause"
)

// SeedData menjalankan semua seeder data awal. Idempoten (aman dijalankan
// berulang). Tidak fatal bila gagal: kegagalan paling umum adalah tabel belum
// ada karena migrasi belum dijalankan — sudah diperingatkan oleh
// WarnIfMigrationsPending. Server tetap boleh naik agar /health bisa dipakai
// untuk diagnosa.
func SeedData() {
	if err := seedPermissions(); err != nil {
		slog.Error("seeder permissions gagal — sudah `go run ./cmd/migrate up`?", slog.Any("error", err))
		return
	}
	slog.Info("seeding database selesai")
}

// seedPermissions mengisi katalog `permissions` dari permissionCatalog.
//
// ON CONFLICT (code) DO NOTHING → idempoten & bebas race saat beberapa instance
// start bersamaan. `permissions` bukan tabel bertenant, jadi tidak terkena RLS.
// ULID di-generate lewat hook BeforeCreate tiap baris.
func seedPermissions() error {
	rows := make([]models.Permission, 0, len(permissionCatalog))
	for _, p := range permissionCatalog {
		rows = append(rows, models.Permission{
			Code:        p.Code,
			GroupName:   p.Group,
			Description: p.Description,
		})
	}

	return DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "code"}},
		DoNothing: true,
	}).Create(&rows).Error
}
