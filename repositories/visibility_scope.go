package repositories

import (
	"context"

	"candra/backend-api/internal/reqctx"

	"gorm.io/gorm"
)

// Lapis 3 isolasi (docs/TECHNICAL-BACKEND.md §6, blueprint E.5): di dalam SATU
// tenant, seorang sales hanya boleh melihat datanya sendiri; supervisor/pemilik
// yang memegang `crm.lead.view.all` melihat semua.
//
// Dipakai untuk `customers`, `deals`, `activities`, `quotations`, `projects`,
// `invoices` — sesudah scopeTenant (lapis 1), sebelum filter query lainnya.

// scopeVisibility menambahkan batas kepemilikan pada query. Baris ber-owner_id
// NULL (pelanggan lama / walk-in yang dibuat di kasir sebelum modul CRM) tetap
// terlihat semua orang — tak pernah "hilang" karena aturan ini. Baris CRM baru
// selalu ber-owner_id (kolomnya NOT NULL), jadi bagian NULL hanya menyentuh
// `customers`.
//
// Parentheses eksplisit di string penting: tanpa itu presedensi AND/OR membuat
// klausa `tenant_id = ?` ikut ter-OR dan membocorkan baris tenant lain.
func scopeVisibility(ctx context.Context, db *gorm.DB, table string) *gorm.DB {
	if reqctx.HasPermission(ctx, "crm.lead.view.all") {
		return db
	}
	return db.Where("("+table+".owner_id = ? OR "+table+".owner_id IS NULL)", reqctx.UserID(ctx))
}

// currentUserID mengambil user_id dari context (mis. untuk stempel owner_id saat
// membuat data CRM). Kosong hanya bila rute tak melewati middleware Auth.
func currentUserID(ctx context.Context) string {
	return reqctx.UserID(ctx)
}
