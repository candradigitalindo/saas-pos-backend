package repositories

import (
	"context"
	"strings"

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

// ── Batas cabang (user_outlets) untuk query BACA ───────────────────────────
//
// Staf yang tidak memegang outlet.manage hanya boleh melihat data cabang
// tempat ia bekerja — penjualan, shift, kas, stok, pembelian, opname,
// transfer, dan laporan. Daftarnya dimuat middleware TenantScope ke context
// (reqctx.OutletScope); tanpa batas → query tidak disentuh.
//
// Kueri yang MEMINTA cabang lain secara eksplisit (outlet_id=B) tidak
// ditolak di sini — hasilnya kosong, karena `outlet_id = B AND outlet_id IN
// (A)` tidak pernah benar. Detail dokumen diperiksa OutletVisible.

// scopeOutlet membatasi query ke cabang yang boleh dilihat user permintaan.
// `col` harus dikualifikasi nama tabel bila query-nya memakai JOIN.
func scopeOutlet(ctx context.Context, db *gorm.DB, col string) *gorm.DB {
	ids, terbatas := reqctx.OutletScope(ctx)
	if !terbatas {
		return db
	}
	if len(ids) == 0 {
		return db.Where("1 = 0")
	}
	return db.Where(col+" IN ?", ids)
}

// whereOutlet = scopeOutlet untuk pemanggil paginateTenant yang menyusun
// klausa WHERE sebagai teks: menambahkan batas cabang ke `where`/`args`.
// `cols` lebih dari satu = cukup salah satu kolom cocok (transfer stok
// terlihat oleh cabang asal MAUPUN tujuan).
func whereOutlet(ctx context.Context, where string, args []any, cols ...string) (string, []any) {
	ids, terbatas := reqctx.OutletScope(ctx)
	if !terbatas {
		return where, args
	}
	kondisi := "1 = 0"
	if len(ids) > 0 {
		bagian := make([]string, len(cols))
		for i, c := range cols {
			bagian[i] = c + " IN ?"
			args = append(args, ids)
		}
		kondisi = "(" + strings.Join(bagian, " OR ") + ")"
	}
	if where == "" {
		return kondisi, args
	}
	return "(" + where + ") AND " + kondisi, args
}

// OutletVisible melaporkan apakah dokumen milik outlet-outlet ini boleh
// dilihat user permintaan (cukup salah satu). Dipakai endpoint detail —
// penjualan, shift, pembelian, opname, transfer — yang memuat dokumennya lewat
// fungsi Find*InTenant bersama jalur tulis.
func OutletVisible(ctx context.Context, outletIDs ...string) bool {
	for _, id := range outletIDs {
		if reqctx.OutletAllowed(ctx, id) {
			return true
		}
	}
	return false
}

// currentUserID mengambil user_id dari context (mis. untuk stempel owner_id saat
// membuat data CRM). Kosong hanya bila rute tak melewati middleware Auth.
func currentUserID(ctx context.Context) string {
	return reqctx.UserID(ctx)
}
