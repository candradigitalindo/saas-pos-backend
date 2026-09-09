package database

import (
	"log/slog"

	"candra/backend-api/models"

	"github.com/shopspring/decimal"
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
	if err := seedPlans(); err != nil {
		slog.Error("seeder plans gagal — sudah `go run ./cmd/migrate up`?", slog.Any("error", err))
		return
	}
	if err := seedPartnerTiers(); err != nil {
		slog.Error("seeder partner_tiers gagal — sudah `go run ./cmd/migrate up`?", slog.Any("error", err))
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

// seedPlans mengisi katalog `plans` & tangga `plan_term_discounts` (§5.13).
//
// ON CONFLICT DO NOTHING → idempoten & bebas race. Tabel platform, tanpa RLS.
// Harga yang sudah diubah admin TIDAK ditimpa (DO NOTHING, bukan upsert).
func seedPlans() error {
	plans := make([]models.Plan, 0, len(planCatalog))
	for _, p := range planCatalog {
		plans = append(plans, models.Plan{
			Code: p.Code, Name: p.Name, MonthlyPrice: p.Monthly,
			Features: []byte(p.Features), IsActive: true,
		})
	}
	if err := DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "code"}},
		DoNothing: true,
	}).Create(&plans).Error; err != nil {
		return err
	}

	discounts := make([]models.PlanTermDiscount, 0, len(termDiscountCatalog))
	for _, d := range termDiscountCatalog {
		rate, err := decimal.NewFromString(d.Rate)
		if err != nil {
			return err
		}
		discounts = append(discounts, models.PlanTermDiscount{
			TermMonths: d.Term, DiscountRate: rate, IsActive: true,
		})
	}
	return DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "term_months"}},
		DoNothing: true,
	}).Create(&discounts).Error
}

// seedPartnerTiers mengisi katalog `partner_tiers` (Fase 12, blueprint G.1/G.2).
// ON CONFLICT (code) DO NOTHING → idempoten; angka yang sudah diubah admin tidak
// ditimpa. Tabel platform, tanpa RLS.
func seedPartnerTiers() error {
	rows := make([]models.PartnerTier, 0, len(partnerTierCatalog))
	for _, t := range partnerTierCatalog {
		rate, err := decimal.NewFromString(t.Rate)
		if err != nil {
			return err
		}
		rows = append(rows, models.PartnerTier{
			Name: t.Name, Kind: t.Kind, RecurringRate: rate,
			RecurringMonths: t.RecurringMonths,
			ActivationBonus: t.ActivationBonus, MinActiveMerchants: t.MinActiveMerchants,
			ActivationMinTxn: t.ActMinTxn, ActivationMinDays: t.ActMinDays,
			AttributionDays: t.AttributionDays, ClawbackDays: t.ClawbackDays,
		})
	}
	return DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "name"}},
		DoNothing: true,
	}).Create(&rows).Error
}
