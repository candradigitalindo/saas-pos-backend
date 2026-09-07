package controllers

import (
	"errors"
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

	items := make([]structs.UserResponse, len(users))
	for i, u := range users {
		items[i] = userToResponse(u)
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

	c.JSON(http.StatusOK, structs.SuccessResponse[structs.UserResponse]{
		Success: true,
		Message: "Berhasil mengambil data user",
		Data:    userToResponse(user),
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
		return repositories.CreateUser(ctx, tx, &user)
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
	c.JSON(http.StatusCreated, structs.SuccessResponse[structs.UserResponse]{
		Success: true,
		Message: "User berhasil dibuat",
		Data:    userToResponse(user),
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
	if req.RoleID != "" && req.RoleID != user.RoleID {
		if err := repositories.FindRoleInTenant(ctx, nil, req.RoleID, &newRole); err != nil {
			badRequest(c, "role_id", "Role tidak ditemukan di usaha ini")
			return
		}
		user.RoleID = newRole.ID
		roleChanged = true
	}
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
		return repositories.UpdateUser(ctx, tx, &user)
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

	c.JSON(http.StatusOK, structs.SuccessResponse[any]{
		Success: true,
		Message: "Berhasil menghapus data user",
		Data:    nil,
	})
}
