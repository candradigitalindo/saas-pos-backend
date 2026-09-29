package services

import (
	"context"
	"fmt"
	"strings"

	"candra/backend-api/helpers"
	"candra/backend-api/internal/reqctx"
	"candra/backend-api/models"
	"candra/backend-api/repositories"
	"candra/backend-api/structs"
)

// Layanan panel internal penyedia SaaS (blueprint G.5, migrasi 000035).
//
// Realm KETIGA: staf kita sendiri. Semua tindakan tulis di sini dicatat ke
// `audit_logs` dengan actor_type='admin' — nilai yang memang sudah disediakan
// §5.14 untuk keperluan ini.

// PlatformLogin memverifikasi kredensial staf internal dan menerbitkan token
// ber-realm "platform".
func PlatformLogin(ctx context.Context, email, password string) (structs.PlatformAuthResponse, error) {
	var out structs.PlatformAuthResponse
	admin, err := repositories.FindPlatformAdminByEmail(ctx, strings.ToLower(strings.TrimSpace(email)))
	if err != nil {
		return out, fmt.Errorf("%w: email atau password salah", helpers.ErrUnauthorized)
	}
	if !admin.IsActive || helpers.CheckPassword(password, admin.PasswordHash) != nil {
		return out, fmt.Errorf("%w: email atau password salah", helpers.ErrUnauthorized)
	}

	token, expiresAt, err := helpers.GeneratePlatformAccessToken(admin.ID)
	if err != nil {
		return out, err
	}
	_ = repositories.TouchPlatformAdminLogin(ctx, admin.ID)
	_ = auditPlatform(ctx, admin.ID, "platform.login", "platform_admins", admin.ID)

	return structs.PlatformAuthResponse{
		AccessToken: token,
		ExpiresAt:   expiresAt.UTC().Format(partnerTimeLayout),
		Admin:       platformAdminToResponse(admin),
	}, nil
}

// PlatformProfile mengembalikan profil admin permintaan ini beserta kemampuannya.
func PlatformProfile(ctx context.Context) (structs.PlatformAdminResponse, error) {
	admin, err := repositories.FindPlatformAdminByID(ctx, reqctx.PlatformAdminID(ctx))
	if err != nil {
		return structs.PlatformAdminResponse{}, err
	}
	return platformAdminToResponse(admin), nil
}

// CreatePlatformAdmin membuat akun staf internal baru. Hanya superadmin.
// Password dibuatkan bila kosong dan dikembalikan SEKALI.
func CreatePlatformAdmin(ctx context.Context, in structs.PlatformAdminCreateRequest) (structs.PlatformAdminCreateResponse, error) {
	var out structs.PlatformAdminCreateResponse
	if strings.TrimSpace(in.Name) == "" || strings.TrimSpace(in.Email) == "" {
		return out, fmt.Errorf("%w: name & email wajib diisi", helpers.ErrValidation)
	}
	if !contains(models.PlatformAdminRoles, in.Role) {
		return out, fmt.Errorf("%w: role harus salah satu dari %v", helpers.ErrValidation, models.PlatformAdminRoles)
	}

	password := strings.TrimSpace(in.Password)
	generated := ""
	if password == "" {
		password = randCode(14)
		generated = password
	}
	hash, err := helpers.HashPassword(password)
	if err != nil {
		return out, err
	}
	admin := models.PlatformAdmin{
		Name: strings.TrimSpace(in.Name), Email: strings.ToLower(strings.TrimSpace(in.Email)),
		Role: in.Role, PasswordHash: hash, IsActive: true,
	}
	if err := repositories.CreatePlatformAdmin(ctx, &admin); err != nil {
		if helpers.IsDuplicateEntryError(err) {
			return out, fmt.Errorf("%w: email admin sudah dipakai", helpers.ErrConflict)
		}
		return out, err
	}
	_ = auditPlatform(ctx, reqctx.PlatformAdminID(ctx), "platform.admin.create", "platform_admins", admin.ID)

	return structs.PlatformAdminCreateResponse{
		Admin: platformAdminToResponse(admin), GeneratedPassword: generated,
	}, nil
}

// ListPlatformAdmins mengembalikan seluruh staf internal.
func ListPlatformAdmins(ctx context.Context) ([]structs.PlatformAdminResponse, error) {
	rows, err := repositories.ListPlatformAdmins(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]structs.PlatformAdminResponse, len(rows))
	for i := range rows {
		out[i] = platformAdminToResponse(rows[i])
	}
	return out, nil
}

// SetPlatformAdminActive menyalakan/mematikan akun staf internal.
//
// Menolak mematikan SUPERADMIN TERAKHIR yang masih aktif — tanpa penjaga ini
// panel internal bisa terkunci total, persis seperti peran pemilik di tenant.
func SetPlatformAdminActive(ctx context.Context, id string, active bool) error {
	target, err := repositories.FindPlatformAdminByID(ctx, id)
	if err != nil {
		return err
	}
	if !active && target.Role == "superadmin" {
		lain, err := repositories.CountActiveSuperadmins(ctx, id)
		if err != nil {
			return err
		}
		if lain == 0 {
			return fmt.Errorf("%w: ini superadmin aktif terakhir — panel akan terkunci bila dinonaktifkan", helpers.ErrConflict)
		}
	}
	if err := repositories.SetPlatformAdminActive(ctx, id, active); err != nil {
		return err
	}
	aksi := "platform.admin.deactivate"
	if active {
		aksi = "platform.admin.activate"
	}
	_ = auditPlatform(ctx, reqctx.PlatformAdminID(ctx), aksi, "platform_admins", id)
	return nil
}

// ── util ────────────────────────────────────────────────────────────────

// auditPlatform mencatat satu tindakan staf internal ke audit_logs (§5.14).
// tenant_id sengaja nil: ini aksi tingkat platform, bukan milik tenant mana pun.
func auditPlatform(ctx context.Context, adminID, action, targetTable, targetID string) error {
	tt, ti := targetTable, targetID
	return repositories.WriteAuditLogs(ctx, []repositories.AuditEntry{{
		ActorType: "admin", ActorID: &adminID, Action: action,
		TargetTable: &tt, TargetID: &ti,
	}})
}

// AuditPlatformAction diekspor agar controller bisa mencatat tindakan yang
// logikanya sudah ada di service lain (mis. menyetujui mitra, mencairkan komisi).
func AuditPlatformAction(ctx context.Context, action, targetTable, targetID string) {
	_ = auditPlatform(ctx, reqctx.PlatformAdminID(ctx), action, targetTable, targetID)
}

func platformAdminToResponse(a models.PlatformAdmin) structs.PlatformAdminResponse {
	r := structs.PlatformAdminResponse{
		ID: a.ID, Name: a.Name, Email: a.Email, Role: a.Role,
		Capabilities: models.PlatformCapabilities(a.Role), IsActive: a.IsActive,
	}
	if a.LastLoginAt != nil {
		r.LastLoginAt = a.LastLoginAt.UTC().Format(partnerTimeLayout)
	}
	return r
}
