package repositories

import (
	"context"
	"errors"

	"candra/backend-api/database"
	"candra/backend-api/internal/reqctx"

	"gorm.io/gorm"
)

// ErrNoTenantInContext dikembalikan WithTenant bila dipanggil tanpa tenant_id di
// context. Untuk permintaan HTTP ini seharusnya tidak pernah terjadi (middleware
// TenantScope menjaminnya); untuk pekerja latar, pemanggil wajib memasang
// tenant_id lebih dulu lewat reqctx.WithTenantID.
var ErrNoTenantInContext = errors.New("tenant_id tidak ada di context")

// Lapisan 1 isolasi tenant (docs/TECHNICAL-BACKEND.md §6): tidak ada query ke
// tabel bertenant yang tidak melewati scopeTenant.

// scopeTenant menambahkan `WHERE tenant_id = <tenant context>` pada query TANPA
// JOIN (kolom tenant_id tidak ambigu).
//
// PANIC bila context tidak punya tenant_id — itu berarti sebuah rute
// tenant-scoped dipasang tanpa middleware TenantScope, sebuah bug yang harus
// terlihat keras saat dikembangkan, bukan diam-diam mengembalikan data semua
// tenant.
func scopeTenant(ctx context.Context, db *gorm.DB) *gorm.DB {
	return db.Where("tenant_id = ?", currentTenantID(ctx))
}

// scopeTenantOn sama seperti scopeTenant tetapi mengkualifikasi kolom dengan
// nama tabel — dipakai pada query yang punya JOIN, di mana kolom `tenant_id`
// muncul di lebih dari satu tabel (CONVENTIONS §2).
func scopeTenantOn(ctx context.Context, db *gorm.DB, table string) *gorm.DB {
	return db.Where(table+".tenant_id = ?", currentTenantID(ctx))
}

// currentTenantID mengambil tenant_id dari context, panik bila kosong.
func currentTenantID(ctx context.Context) string {
	tid := reqctx.TenantID(ctx)
	if tid == "" {
		panic("repositories: operasi tenant-scoped dipanggil tanpa tenant_id di context — rute tidak dipasangi middleware TenantScope?")
	}
	return tid
}

// tenantDB memilih handle database untuk sebuah operasi repo:
//   - tx != nil  → pakai transaksi milik pemanggil (service di dalam WithTenant).
//   - tx == nil  → pakai DB global (jalur baca biasa, tanpa transaksi).
//
// Semua dibungkus WithContext agar query batal saat klien memutus koneksi.
func tenantDB(ctx context.Context, tx *gorm.DB) *gorm.DB {
	if tx != nil {
		return tx.WithContext(ctx)
	}
	return database.DB.WithContext(ctx)
}

// WithTenant menjalankan fn di dalam satu transaksi yang GUC `app.tenant_id`-nya
// sudah disetel ke tenant context — sehingga lapisan 2 (Row Level Security,
// migrasi 000004) ikut menjaga setiap tulisan di dalamnya.
//
// Dipakai oleh SERVICE yang membuka proses bisnis lintas tabel (§7), dan oleh
// controller untuk operasi tulis satu-langkah. Jangan dipakai membungkus
// pembacaan biasa — jalur baca cukup scopeTenant (lapisan 1).
//
// set_config(..., true) dipakai, bukan `SET LOCAL ... = ?`: `SET` tidak menerima
// parameter terikat, sedangkan set_config menerimanya dan `true` membuatnya
// transaction-scoped (setara SET LOCAL) — aman dengan connection pool / PgBouncer.
func WithTenant(ctx context.Context, fn func(tx *gorm.DB) error) error {
	tid := reqctx.TenantID(ctx)
	if tid == "" {
		return ErrNoTenantInContext
	}
	return database.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT set_config('app.tenant_id', ?, true)", tid).Error; err != nil {
			return err
		}
		return fn(tx)
	})
}

// Transaction menjalankan fn dalam satu transaksi TANPA menyetel GUC tenant.
// Dipakai proses yang belum punya tenant di context — terutama REGISTRASI, yang
// membuat baris tenants lebih dulu lalu memanggil SetTenantGUC di tengah jalan.
// Menjaga service tidak perlu mengimpor paket database langsung.
func Transaction(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return database.DB.WithContext(ctx).Transaction(fn)
}

// SetTenantGUC menyetel GUC app.tenant_id (transaction-scoped) di dalam transaksi
// tx yang sedang berjalan. Dipakai registration service setelah membuat baris
// tenants, agar insert berikutnya (outlets, roles, users) lolos WITH CHECK RLS.
func SetTenantGUC(tx *gorm.DB, tid string) error {
	return tx.Exec("SELECT set_config('app.tenant_id', ?, true)", tid).Error
}
