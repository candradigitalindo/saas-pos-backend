// Package services memuat logika bisnis yang menyentuh beberapa tabel dalam satu
// transaksi. Aturannya (docs/TECHNICAL-BACKEND.md §2, §7): repository tahu satu
// tabel, service tahu satu proses bisnis, controller tahu satu endpoint. Service
// yang membuka transaksi — bukan controller, bukan repository.
package services

import (
	"context"
	"fmt"
	"strings"

	"candra/backend-api/helpers"
	"candra/backend-api/internal/timez"
	"candra/backend-api/models"
	"candra/backend-api/repositories"

	"gorm.io/gorm"
)

// RegisterTenantInput adalah masukan pendaftaran usaha baru. Divalidasi ulang di
// service (tidak hanya mengandalkan tag binding controller) supaya pemakai lain
// — test, CLI — ikut terlindungi.
type RegisterTenantInput struct {
	BusinessName     string
	BusinessType     string // salah satu models.TenantBusinessTypes
	Phone            string
	OutletName       string
	Timezone         string // nama IANA; salah satu timez.SupportedTimezones()
	BusinessDayStart string // "HH:MM" / "HH:MM:SS"; kosong = "00:00"
	ReferralCode     string // opsional

	OwnerName     string
	OwnerUsername string
	OwnerEmail    string
	OwnerPassword string
}

// RegisterTenantResult adalah entitas yang lahir dari pendaftaran. Controller
// yang memetakannya ke response + menerbitkan token.
type RegisterTenantResult struct {
	Tenant models.Tenant
	Outlet models.Outlet
	Owner  models.User
}

// defaultRoles adalah peran bawaan yang dibuatkan untuk SETIAP tenant baru,
// beserta permission-nya. Urutan slice (bukan map) dipertahankan agar hasil
// pendaftaran deterministik dan mudah diuji.
//
// Pemilik: permissions == nil berarti "semua permission di katalog".
var defaultRoles = []struct {
	Name        string
	Description string
	IsSystem    bool
	Permissions []string // nil = semua
}{
	{
		Name:        "Pemilik",
		Description: "Akses penuh atas seluruh fitur. Peran bawaan pemilik usaha, tidak bisa dihapus.",
		IsSystem:    true,
		Permissions: nil,
	},
	{
		Name:        "Manajer",
		Description: "Mengelola operasional harian: penjualan, stok, laporan, kas, pelanggan, outlet.",
		Permissions: []string{
			"sale.create", "sale.void", "sale.refund", "sale.discount", "sale.price_override",
			"product.view", "product.edit", "product.import",
			"stock.view", "stock.adjust", "stock.opname", "stock.transfer",
			"report.view", "report.profit", "report.export",
			"shift.open", "shift.close", "shift.reconcile", "cash.movement",
			"customer.view", "customer.edit", "receivable.manage",
			"outlet.manage",
		},
	},
	{
		Name:        "Kasir",
		Description: "Melayani transaksi di kasir dan mengelola shift-nya sendiri.",
		Permissions: []string{
			"sale.create", "sale.discount",
			"product.view", "stock.view",
			"shift.open", "shift.close", "cash.movement",
			"customer.view", "customer.edit",
		},
	},
	{
		Name:        "Gudang",
		Description: "Mengelola stok dan data produk.",
		Permissions: []string{
			"product.view", "product.edit", "product.import",
			"stock.view", "stock.adjust", "stock.opname", "stock.transfer",
		},
	},
}

// ownerRoleName adalah nama peran yang dipakai untuk user pemilik saat daftar.
const ownerRoleName = "Pemilik"

// RegisterTenant mendaftarkan usaha baru: tenant + outlet pertama + peran bawaan
// + user pemilik + akses outlet, SEMUANYA dalam satu transaksi (§8). Bila satu
// langkah gagal, tidak ada yang tersisa.
//
// Password di-hash di LUAR transaksi (bcrypt lambat, jangan menahan transaksi).
// GUC app.tenant_id disetel tepat setelah baris tenants dibuat, agar insert
// berikutnya lolos WITH CHECK Row Level Security.
func RegisterTenant(ctx context.Context, in RegisterTenantInput) (*RegisterTenantResult, error) {
	in = in.normalized()
	if err := in.validate(); err != nil {
		return nil, err
	}

	passwordHash, err := helpers.HashPassword(in.OwnerPassword)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	dayStart, _ := timez.ParseClock(in.BusinessDayStart) // sudah divalidasi di validate()

	var result RegisterTenantResult
	txErr := repositories.Transaction(ctx, func(tx *gorm.DB) error {
		// 1. Tenant (GUC belum disetel → RLS permisif, insert diizinkan).
		tenant := models.Tenant{
			BusinessName:     in.BusinessName,
			BusinessType:     in.BusinessType,
			OwnerName:        in.OwnerName,
			Phone:            in.Phone,
			Email:            in.OwnerEmail,
			Status:           "trial",
			ReferralCodeUsed: in.ReferralCode,
		}
		if err := repositories.CreateTenant(ctx, tx, &tenant); err != nil {
			return err
		}

		// 2. Mulai sekarang, batasi transaksi ke tenant ini (RLS aktif).
		if err := repositories.SetTenantGUC(tx, tenant.ID); err != nil {
			return err
		}

		// 3. Outlet pertama.
		outlet := models.Outlet{
			TenantID:         tenant.ID,
			Name:             in.OutletName,
			Type:             "store",
			Timezone:         in.Timezone,
			BusinessDayStart: dayStart,
			Currency:         "IDR",
			IsActive:         true,
		}
		if err := repositories.CreateOutlet(ctx, tx, &outlet); err != nil {
			return err
		}

		// 4. Peran bawaan + pemetaan permission.
		allPermIDs, err := repositories.AllPermissionIDs(ctx, tx)
		if err != nil {
			return err
		}
		if len(allPermIDs) == 0 {
			return fmt.Errorf("katalog permissions kosong — jalankan seeder")
		}

		var ownerRoleID string
		for _, def := range defaultRoles {
			role := models.Role{
				TenantID:    tenant.ID,
				Name:        def.Name,
				Description: def.Description,
				IsSystem:    def.IsSystem,
			}
			if err := repositories.CreateRole(ctx, tx, &role); err != nil {
				return err
			}
			if def.Name == ownerRoleName {
				ownerRoleID = role.ID
			}

			permIDs := allPermIDs
			if def.Permissions != nil {
				permIDs, err = repositories.PermissionIDsByCode(ctx, tx, def.Permissions)
				if err != nil {
					return err
				}
			}
			if err := repositories.AssignRolePermissions(ctx, tx, role.ID, permIDs); err != nil {
				return err
			}
		}

		// 5. User pemilik.
		owner := models.User{
			TenantID: tenant.ID,
			Name:     in.OwnerName,
			Username: in.OwnerUsername,
			Email:    in.OwnerEmail,
			Password: passwordHash,
			RoleID:   ownerRoleID,
			IsActive: true,
		}
		if err := repositories.CreateUser(ctx, tx, &owner); err != nil {
			return err
		}

		// 6. Akses ke outlet pertama.
		if err := repositories.LinkUserOutlet(ctx, tx, models.UserOutlet{
			TenantID: tenant.ID,
			UserID:   owner.ID,
			OutletID: outlet.ID,
		}); err != nil {
			return err
		}

		// Muat role ke owner agar controller punya nama role tanpa query ulang.
		owner.Role = models.Role{ID: ownerRoleID, TenantID: tenant.ID, Name: ownerRoleName, IsSystem: true}
		result = RegisterTenantResult{Tenant: tenant, Outlet: outlet, Owner: owner}
		return nil
	})
	if txErr != nil {
		if helpers.IsDuplicateEntryError(txErr) {
			return nil, fmt.Errorf("%w: username atau email sudah terpakai", helpers.ErrConflict)
		}
		return nil, txErr
	}
	return &result, nil
}

// normalized merapikan spasi & huruf pada field yang perlu.
func (in RegisterTenantInput) normalized() RegisterTenantInput {
	in.BusinessName = strings.TrimSpace(in.BusinessName)
	in.BusinessType = strings.ToLower(strings.TrimSpace(in.BusinessType))
	in.Phone = strings.TrimSpace(in.Phone)
	in.OutletName = strings.TrimSpace(in.OutletName)
	in.Timezone = strings.TrimSpace(in.Timezone)
	in.BusinessDayStart = strings.TrimSpace(in.BusinessDayStart)
	in.ReferralCode = strings.TrimSpace(in.ReferralCode)
	in.OwnerName = strings.TrimSpace(in.OwnerName)
	in.OwnerUsername = strings.TrimSpace(in.OwnerUsername)
	in.OwnerEmail = strings.ToLower(strings.TrimSpace(in.OwnerEmail))
	if in.BusinessDayStart == "" {
		in.BusinessDayStart = "00:00"
	}
	if in.Timezone == "" {
		in.Timezone = timez.WIB
	}
	return in
}

// validate memeriksa nilai yang tidak bisa dijamin tag binding: enum tipe usaha,
// zona waktu yang didukung, format batas hari usaha.
func (in RegisterTenantInput) validate() error {
	if !contains(models.TenantBusinessTypes, in.BusinessType) {
		return fmt.Errorf("%w: jenis usaha harus salah satu dari %s",
			helpers.ErrValidation, strings.Join(models.TenantBusinessTypes, ", "))
	}
	if !timez.IsSupportedTimezone(in.Timezone) {
		return fmt.Errorf("%w: zona waktu tidak didukung (%s)",
			helpers.ErrValidation, strings.Join(timez.SupportedTimezones(), ", "))
	}
	if _, err := timez.ParseDayStart(in.BusinessDayStart); err != nil {
		return fmt.Errorf("%w: format jam mulai hari usaha tidak valid", helpers.ErrValidation)
	}
	return nil
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
