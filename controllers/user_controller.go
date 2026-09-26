package controllers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"candra/backend-api/helpers"
	"candra/backend-api/internal/reqctx"
	"candra/backend-api/models"
	"candra/backend-api/repositories"
	"candra/backend-api/structs"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Semua handler di sini tenant-scoped: repositories.* memakai scopeTenant,
// sehingga user tenant lain tidak pernah terlihat/terubah walau id-nya ditebak.

// GetAllUsers mengembalikan user milik tenant, berpaginasi.
func GetAllUsers(c *gin.Context) {
	page, limit, offset := helpers.ParsePaginationParams(c)
	search := c.Query("search")

	users, total, err := repositories.ListUsers(c.Request.Context(), search, limit, offset)
	if err != nil {
		respondServiceError(c, err)
		return
	}

	ids := make([]string, len(users))
	for i, u := range users {
		ids[i] = u.ID
	}
	rolesByUser, err := repositories.RoleIDsForUsers(c.Request.Context(), ids)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	outletsByUser, err := repositories.OutletIDsForUsers(c.Request.Context(), ids)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	items := make([]structs.UserResponse, len(users))
	for i, u := range users {
		items[i] = userToResponseWithRoles(u, rolesByUser[u.ID])
		items[i].OutletIDs = outletsByUser[u.ID]
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.PaginatedResponse[structs.UserResponse]]{
		Success: true,
		Message: "Berhasil mengambil data user",
		Data:    helpers.BuildPaginationResponse(c, page, limit, total, items),
	})
}

// GetUserByID mengembalikan satu user milik tenant.
func GetUserByID(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		badRequest(c, "id", "ID user tidak boleh kosong")
		return
	}

	var user models.User
	if err := repositories.FindUserInTenant(c.Request.Context(), nil, id, &user); err != nil {
		if errors.Is(err, repositories.ErrUserNotFound) {
			notFound(c, "User tidak ditemukan")
			return
		}
		respondServiceError(c, err)
		return
	}

	roleIDs, err := repositories.RoleIDsForUser(c.Request.Context(), nil, user.ID)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	outletIDs, err := repositories.OutletIDsForUser(c.Request.Context(), user.ID)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	res := userToResponseWithRoles(user, roleIDs)
	res.OutletIDs = outletIDs
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.UserResponse]{
		Success: true,
		Message: "Berhasil mengambil data user",
		Data:    res,
	})
}

// CreateUser menambah user staf. Butuh permission user.manage. role_id wajib dan
// harus role milik tenant yang sama.
func CreateUser(c *gin.Context) {
	var req structs.UserCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}

	ctx := c.Request.Context()

	var role models.Role
	if err := repositories.FindRoleInTenant(ctx, nil, req.RoleID, &role); err != nil {
		badRequest(c, "role_id", "Role tidak ditemukan di usaha ini")
		return
	}
	// Peran tambahan juga harus milik tenant ini — kalau tidak, seorang manajer
	// bisa menempelkan peran tenant lain lewat tebakan id. FK komposit
	// (tenant_id, role_id) di user_roles adalah jaring pengaman terakhirnya.
	if err := ensureRolesInTenant(ctx, req.RoleIDs); err != nil {
		badRequest(c, "role_ids", err.Error())
		return
	}
	// Akses cabang. Dulu tidak pernah diisi untuk staf, sehingga /me
	// mengembalikan outlet_ids kosong dan layar kasir web tidak punya toko
	// aktif sama sekali untuk siapa pun selain pemilik.
	outletIDs := req.OutletIDs
	if len(outletIDs) == 0 {
		semua, err := repositories.ActiveOutletIDs(ctx, nil)
		if err != nil {
			respondServiceError(c, err)
			return
		}
		outletIDs = semua
	} else if err := ensureOutletsInTenant(ctx, outletIDs); err != nil {
		badRequest(c, "outlet_ids", err.Error())
		return
	}

	hash, err := helpers.HashPassword(req.Password)
	if err != nil {
		internalError(c)
		return
	}

	user := models.User{
		TenantID: reqctx.TenantID(ctx),
		Name:     req.Name,
		Username: req.Username,
		Email:    req.Email,
		Password: hash,
		RoleID:   role.ID,
		IsActive: true,
	}
	if err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		if err := repositories.CreateUser(ctx, tx, &user); err != nil {
			return err
		}
		if err := repositories.SetUserOutlets(ctx, tx, reqctx.TenantID(ctx), user.ID, outletIDs); err != nil {
			return err
		}
		return repositories.SetUserRoles(ctx, tx, reqctx.TenantID(ctx), user.ID, role.ID, req.RoleIDs)
	}); err != nil {
		if helpers.IsDuplicateEntryError(err) {
			c.JSON(http.StatusConflict, structs.ErrorResponse{
				Success: false,
				Message: "Username atau email sudah terdaftar",
				Errors:  map[string]string{"username": "sudah terpakai"},
			})
			return
		}
		respondServiceError(c, err)
		return
	}

	user.Role = role
	res := userToResponse(user)
	res.OutletIDs = outletIDs
	c.JSON(http.StatusCreated, structs.SuccessResponse[structs.UserResponse]{
		Success: true,
		Message: "User berhasil dibuat",
		Data:    res,
	})
}

// UpdateUser mengubah user staf milik tenant.
func UpdateUser(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		badRequest(c, "id", "ID user tidak boleh kosong")
		return
	}

	var req structs.UserUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}

	ctx := c.Request.Context()

	var user models.User
	if err := repositories.FindUserInTenant(ctx, nil, id, &user); err != nil {
		if errors.Is(err, repositories.ErrUserNotFound) {
			notFound(c, "User tidak ditemukan")
			return
		}
		respondServiceError(c, err)
		return
	}

	var newRole models.Role
	roleChanged := false
	oldPrimaryRoleID := user.RoleID // sebelum diganti; dipakai saat menyusun peran tambahan
	if req.RoleID != "" && req.RoleID != user.RoleID {
		if err := repositories.FindRoleInTenant(ctx, nil, req.RoleID, &newRole); err != nil {
			badRequest(c, "role_id", "Role tidak ditemukan di usaha ini")
			return
		}
		user.RoleID = newRole.ID
		roleChanged = true
	}
	if req.RoleIDs != nil {
		if err := ensureRolesInTenant(ctx, *req.RoleIDs); err != nil {
			badRequest(c, "role_ids", err.Error())
			return
		}
		roleChanged = true
	}
	if req.OutletIDs != nil {
		if err := ensureOutletsInTenant(ctx, *req.OutletIDs); err != nil {
			badRequest(c, "outlet_ids", err.Error())
			return
		}
	}
	// Ganti password atau penonaktifan WAJIB memutus sesi yang sedang
	// berjalan. Tanpa ini perangkat yang tertinggal di tangan orang lain (atau
	// token yang dicuri) tetap bisa memperbarui aksesnya sampai 30 hari,
	// walau pemilik sudah mengganti password-nya.
	cabutSesi := req.Password != "" || (req.IsActive != nil && !*req.IsActive)
	if req.Name != "" {
		user.Name = req.Name
	}
	if req.Username != "" {
		user.Username = req.Username
	}
	if req.Email != "" {
		user.Email = req.Email
	}
	if req.IsActive != nil {
		user.IsActive = *req.IsActive
	}
	if req.Password != "" {
		hash, err := helpers.HashPassword(req.Password)
		if err != nil {
			internalError(c)
			return
		}
		user.Password = hash
	}

	if err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		if err := repositories.UpdateUser(ctx, tx, &user); err != nil {
			return err
		}
		if req.OutletIDs != nil {
			if err := repositories.SetUserOutlets(ctx, tx, reqctx.TenantID(ctx), user.ID, *req.OutletIDs); err != nil {
				return err
			}
		}
		if !roleChanged {
			return nil
		}
		// Peran tambahan hanya diganti bila klien mengirim role_ids; bila hanya
		// role_id yang berubah, peran tambahan yang sudah ada dipertahankan.
		//
		// Peran utama LAMA dibuang dari daftar itu: ia ada di user_roles karena
		// menjadi peran utama, bukan karena ditambahkan terpisah. Tanpa ini,
		// memindahkan staf ke peran lain akan menyisakan peran lamanya menempel
		// selamanya (dan peran itu jadi tak pernah bisa dihapus).
		extra := req.RoleIDs
		if extra == nil {
			existing, err := repositories.RoleIDsForUser(ctx, tx, user.ID)
			if err != nil {
				return err
			}
			kept := make([]string, 0, len(existing))
			for _, id := range existing {
				if id != oldPrimaryRoleID {
					kept = append(kept, id)
				}
			}
			extra = &kept
		}
		return repositories.SetUserRoles(ctx, tx, reqctx.TenantID(ctx), user.ID, user.RoleID, *extra)
	}); err != nil {
		if errors.Is(err, repositories.ErrUserNotFound) {
			notFound(c, "User tidak ditemukan")
			return
		}
		if helpers.IsDuplicateEntryError(err) {
			c.JSON(http.StatusConflict, structs.ErrorResponse{
				Success: false,
				Message: "Username atau email sudah terdaftar",
				Errors:  map[string]string{"username": "sudah terpakai"},
			})
			return
		}
		respondServiceError(c, err)
		return
	}

	if cabutSesi {
		if err := repositories.RevokeAllUserRefreshTokens(ctx, user.ID); err != nil {
			helpers.LoggerFromContext(ctx).Error("gagal mencabut sesi setelah perubahan akun",
				slog.Any("error", err), slog.String("user_id", user.ID))
		}
	}
	if roleChanged {
		user.Role = newRole
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.UserResponse]{
		Success: true,
		Message: "Berhasil memperbarui data user",
		Data:    userToResponse(user),
	})
}

// DeleteUser menonaktifkan (soft delete) user staf. Tidak bisa menghapus diri
// sendiri — mencegah owner mengunci dirinya keluar tanpa sengaja.
func DeleteUser(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		badRequest(c, "id", "ID user tidak boleh kosong")
		return
	}
	ctx := c.Request.Context()
	if id == reqctx.UserID(ctx) {
		badRequest(c, "id", "Tidak bisa menghapus akun Anda sendiri")
		return
	}

	if err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		return repositories.SoftDeleteUser(ctx, tx, id)
	}); err != nil {
		if errors.Is(err, repositories.ErrUserNotFound) {
			notFound(c, "User tidak ditemukan")
			return
		}
		respondServiceError(c, err)
		return
	}
	if err := repositories.RevokeAllUserRefreshTokens(ctx, id); err != nil {
		helpers.LoggerFromContext(ctx).Error("gagal mencabut sesi user yang dihapus",
			slog.Any("error", err), slog.String("user_id", id))
	}

	c.JSON(http.StatusOK, structs.SuccessResponse[any]{
		Success: true,
		Message: "Berhasil menghapus data user",
		Data:    nil,
	})
}

// ensureRolesInTenant memastikan setiap id peran benar-benar milik tenant
// permintaan ini. Dipakai sebelum menempelkan peran tambahan ke seorang user.
func ensureRolesInTenant(ctx context.Context, roleIDs []string) error {
	for _, id := range roleIDs {
		if id == "" {
			continue
		}
		var r models.Role
		if err := repositories.FindRoleInTenant(ctx, nil, id, &r); err != nil {
			return fmt.Errorf("role %s tidak ditemukan di usaha ini", id)
		}
	}
	return nil
}

// ensureOutletsInTenant memastikan setiap id outlet milik tenant permintaan
// ini, sebelum ditautkan ke seorang user.
func ensureOutletsInTenant(ctx context.Context, outletIDs []string) error {
	for _, id := range outletIDs {
		var o models.Outlet
		if err := repositories.FindOutletByID(ctx, nil, id, &o); err != nil {
			return fmt.Errorf("outlet %s tidak ditemukan di usaha ini", id)
		}
	}
	return nil
}
