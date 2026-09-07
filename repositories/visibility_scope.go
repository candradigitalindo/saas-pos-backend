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

// scopeOwnUnless membatasi query ke `col = user konteks` KECUALI user memegang
// salah satu `bypassPerms`. Dipakai untuk tabel yang di-scope ke pemakainya
// lewat kolom selain `owner_id` (mis. sales_targets/commissions → `user_id`),
// di mana pengawas dengan izin tertentu boleh melihat seluruh tim.
func scopeOwnUnless(ctx context.Context, db *gorm.DB, col string, bypassPerms ...string) *gorm.DB {
	if reqctx.HasAnyPermission(ctx, bypassPerms...) {
		return db
	}
	return db.Where(col+" = ?", reqctx.UserID(ctx))
}

// currentUserID mengambil user_id dari context (mis. untuk stempel owner_id saat
// membuat data CRM). Kosong hanya bila rute tak melewati middleware Auth.
func currentUserID(ctx context.Context) string {
	return reqctx.UserID(ctx)
}
