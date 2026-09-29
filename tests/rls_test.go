package tests

import (
	"context"
	"testing"

	"candra/backend-api/internal/reqctx"
	"candra/backend-api/internal/timez"
	"candra/backend-api/models"
	"candra/backend-api/repositories"

	"gorm.io/gorm"
)

// TestRLSBlocksCrossTenantWrite membuktikan lapisan 2 (Row Level Security):
// walau lapisan 1 (scopeTenant) dilewati, PostgreSQL sendiri menolak menulis
// baris milik tenant lain selama transaksi berjalan di dalam WithTenant.
func TestRLSBlocksCrossTenantWrite(t *testing.T) {
	requireDB(t)

	a := registerTenant(t, "rlsA")
	b := registerTenant(t, "rlsB")

	ctxA := reqctx.WithTenantID(context.Background(), a.tenantID)

	// Negatif: menyisipkan outlet milik tenant B saat GUC = tenant A → ditolak
	// oleh WITH CHECK policy.
	err := repositories.WithTenant(ctxA, func(tx *gorm.DB) error {
		return tx.Create(&models.Outlet{
			TenantID:         b.tenantID,
			Name:             "outlet selundupan",
			Type:             "store",
			Timezone:         "Asia/Jakarta",
			Currency:         "IDR",
			BusinessDayStart: timez.NewClock(0),
		}).Error
	})
	if err == nil {
		t.Fatal("RLS seharusnya menolak INSERT outlet milik tenant lain")
	}

	// Positif: menyisipkan outlet milik tenant A sendiri → berhasil.
	err = repositories.WithTenant(ctxA, func(tx *gorm.DB) error {
		return tx.Create(&models.Outlet{
			TenantID:         a.tenantID,
			Name:             "gudang A",
			Type:             "warehouse",
			Timezone:         "Asia/Jakarta",
			Currency:         "IDR",
			BusinessDayStart: timez.NewClock(0),
		}).Error
	})
	if err != nil {
		t.Fatalf("INSERT outlet milik tenant sendiri seharusnya berhasil: %v", err)
	}
}

// TestRLSFiltersReadWithoutScope membuktikan: di dalam WithTenant, sebuah SELECT
// yang LUPA memakai scopeTenant tetap hanya melihat baris tenant konteks —
// karena USING policy RLS ikut menyaring.
func TestRLSFiltersReadWithoutScope(t *testing.T) {
	requireDB(t)

	a := registerTenant(t, "rlsreadA")
	_ = registerTenant(t, "rlsreadB")

	ctxA := reqctx.WithTenantID(context.Background(), a.tenantID)

	var rows []models.Outlet
	err := repositories.WithTenant(ctxA, func(tx *gorm.DB) error {
		// Sengaja TANPA scopeTenant.
		return tx.Find(&rows).Error
	})
	if err != nil {
		t.Fatalf("SELECT error: %v", err)
	}
	for _, o := range rows {
		if o.TenantID != a.tenantID {
			t.Fatalf("melihat outlet tenant %s padahal scope = %s", o.TenantID, a.tenantID)
		}
	}
	if len(rows) == 0 {
		t.Fatal("harusnya melihat minimal outlet milik tenant A sendiri")
	}
}

// TestWithTenantRequiresTenant memastikan WithTenant menolak dipanggil tanpa
// tenant_id di context (bukan diam-diam jalan tanpa proteksi).
func TestWithTenantRequiresTenant(t *testing.T) {
	requireDB(t)

	err := repositories.WithTenant(context.Background(), func(tx *gorm.DB) error {
		return nil
	})
	if err != repositories.ErrNoTenantInContext {
		t.Fatalf("err = %v, mau ErrNoTenantInContext", err)
	}
}
