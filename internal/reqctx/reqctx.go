// Package reqctx menyimpan data per-permintaan di context.Context: tenant_id
// efektif, user_id, dan himpunan permission.
//
// Kenapa paket terpisah (paket daun, tanpa dependensi): baik `middlewares`
// maupun `repositories` perlu membaca nilai-nilai ini. Kalau helper-nya taruh di
// salah satu paket itu, yang lain harus mengimpornya — dan `middlewares` sudah
// mengimpor `repositories` untuk memuat user, sehingga terjadi import cycle.
// reqctx memutus simpul itu.
//
// Semua nilai di sini di-set sekali oleh middleware TenantScope dan hanya dibaca
// setelahnya.
package reqctx

import "context"

// ctxKey adalah tipe kunci privat agar tidak bertabrakan dengan kunci context
// paket lain.
type ctxKey int

const (
	tenantIDKey ctxKey = iota
	userIDKey
	permissionsKey
	partnerIDKey
	partnerUserIDKey
	platformAdminIDKey
	platformRoleKey
)

// WithTenantID menautkan tenant_id efektif ke context.
func WithTenantID(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, tenantIDKey, tenantID)
}

// TenantID mengembalikan tenant_id efektif, atau "" bila permintaan belum
// di-scope ke tenant (mis. endpoint publik, pekerja latar tanpa tenant).
func TenantID(ctx context.Context) string {
	s, _ := ctx.Value(tenantIDKey).(string)
	return s
}

// WithUserID menautkan user_id terautentikasi ke context.
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

// UserID mengembalikan user_id terautentikasi, atau "".
func UserID(ctx context.Context) string {
	s, _ := ctx.Value(userIDKey).(string)
	return s
}

// WithPermissions menautkan himpunan kode permission efektif user ke context.
// Nilai disalin agar pemanggil tidak bisa mengubahnya belakangan.
func WithPermissions(ctx context.Context, codes []string) context.Context {
	set := make(map[string]struct{}, len(codes))
	for _, c := range codes {
		set[c] = struct{}{}
	}
	return context.WithValue(ctx, permissionsKey, set)
}

// HasPermission melaporkan apakah user permintaan ini memiliki `code`.
// Mengembalikan false bila himpunan permission belum di-set.
func HasPermission(ctx context.Context, code string) bool {
	set, _ := ctx.Value(permissionsKey).(map[string]struct{})
	_, ok := set[code]
	return ok
}

// HasAnyPermission melaporkan apakah user memiliki setidaknya satu dari `codes`.
// Daftar kosong berarti "tidak ada syarat" → true.
func HasAnyPermission(ctx context.Context, codes ...string) bool {
	if len(codes) == 0 {
		return true
	}
	set, _ := ctx.Value(permissionsKey).(map[string]struct{})
	for _, c := range codes {
		if _, ok := set[c]; ok {
			return true
		}
	}
	return false
}

// Permissions mengembalikan salinan daftar kode permission efektif (urutan tidak
// dijamin). Untuk endpoint yang menampilkan hak akses user ke UI.
func Permissions(ctx context.Context) []string {
	set, _ := ctx.Value(permissionsKey).(map[string]struct{})
	out := make([]string, 0, len(set))
	for c := range set {
		out = append(out, c)
	}
	return out
}

// ── Realm mitra (Fase 12) ────────────────────────────────────────────────
//
// Di-set oleh middleware PartnerAuth. Populasi akun yang TERPISAH dari
// tenant/user di atas — sebuah permintaan tidak pernah punya keduanya.

// WithPartnerID menautkan id mitra ke context.
func WithPartnerID(ctx context.Context, partnerID string) context.Context {
	return context.WithValue(ctx, partnerIDKey, partnerID)
}

// PartnerID mengembalikan id mitra permintaan ini, atau "".
func PartnerID(ctx context.Context) string {
	s, _ := ctx.Value(partnerIDKey).(string)
	return s
}

// WithPartnerUserID menautkan id akun-login mitra ke context.
func WithPartnerUserID(ctx context.Context, partnerUserID string) context.Context {
	return context.WithValue(ctx, partnerUserIDKey, partnerUserID)
}

// PartnerUserID mengembalikan id akun-login mitra permintaan ini, atau "".
func PartnerUserID(ctx context.Context) string {
	s, _ := ctx.Value(partnerUserIDKey).(string)
	return s
}

// ── Realm platform (panel internal, blueprint G.5) ───────────────────────
//
// Di-set oleh middleware PlatformAuth. Populasi akun KETIGA, terpisah dari
// tenant maupun mitra — sebuah permintaan tidak pernah punya lebih dari satu.

// WithPlatformAdmin menautkan id & peran admin platform ke context.
func WithPlatformAdmin(ctx context.Context, adminID, role string) context.Context {
	return context.WithValue(context.WithValue(ctx, platformAdminIDKey, adminID), platformRoleKey, role)
}

// PlatformAdminID mengembalikan id admin platform permintaan ini, atau "".
func PlatformAdminID(ctx context.Context) string {
	s, _ := ctx.Value(platformAdminIDKey).(string)
	return s
}

// PlatformRole mengembalikan peran admin platform permintaan ini, atau "".
func PlatformRole(ctx context.Context) string {
	s, _ := ctx.Value(platformRoleKey).(string)
	return s
}
