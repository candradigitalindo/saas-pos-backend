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

// roleToResponse memetakan model Role + kode permission-nya (opsional) ke DTO.
func roleToResponse(r models.Role, permCodes []string) structs.RoleResponse {
	return structs.RoleResponse{
		ID:              r.ID,
		Name:            r.Name,
		Description:     r.Description,
		IsSystem:        r.IsSystem,
		PermissionCodes: permCodes,
		CreatedAt:       r.CreatedAt.Format(timeLayout),
		UpdatedAt:       r.UpdatedAt.Format(timeLayout),
	}
}

// GetAllRoles mengembalikan peran milik tenant, berpaginasi (tanpa daftar
// permission per baris agar hemat query).
func GetAllRoles(c *gin.Context) {
	page, limit, offset := helpers.ParsePaginationParams(c)
	search := c.Query("search")

	roles, total, err := repositories.ListRoles(c.Request.Context(), search, limit, offset)
	if err != nil {
		respondServiceError(c, err)
		return
	}

	items := make([]structs.RoleResponse, len(roles))
	for i, r := range roles {
		items[i] = roleToResponse(r, nil)
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.PaginatedResponse[structs.RoleResponse]]{
		Success: true,
		Message: "Berhasil mengambil data role",
		Data:    helpers.BuildPaginationResponse(c, page, limit, total, items),
	})
}

// GetRoleByID mengembalikan satu peran beserta kode permission-nya.
func GetRoleByID(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		badRequest(c, "id", "ID role tidak boleh kosong")
		return
	}
	ctx := c.Request.Context()

	var role models.Role
	if err := repositories.FindRoleInTenant(ctx, nil, id, &role); err != nil {
		if errors.Is(err, repositories.ErrRoleNotFound) {
			notFound(c, "Role tidak ditemukan")
			return
		}
		respondServiceError(c, err)
		return
	}

	codes, err := repositories.EffectivePermissionCodes(ctx, role.ID)
	if err != nil {
		respondServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, structs.SuccessResponse[structs.RoleResponse]{
		Success: true,
		Message: "Berhasil mengambil data role",
		Data:    roleToResponse(role, codes),
	})
}

// CreateRole menambah peran baru dalam tenant, opsional langsung dengan
// pemetaan permission.
func CreateRole(c *gin.Context) {
	var req structs.RoleCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	ctx := c.Request.Context()

	permIDs, err := repositories.PermissionIDsByCode(ctx, nil, req.PermissionCodes)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	if len(permIDs) != len(req.PermissionCodes) {
		badRequest(c, "permission_codes", "Ada kode permission yang tidak dikenal")
		return
	}

	role := models.Role{
		TenantID:    reqctx.TenantID(ctx),
		Name:        req.Name,
		Description: req.Description,
		IsSystem:    false,
	}
	if err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		if err := repositories.CreateRole(ctx, tx, &role); err != nil {
			return err
		}
		return repositories.AssignRolePermissions(ctx, tx, role.ID, permIDs)
	}); err != nil {
		if helpers.IsDuplicateEntryError(err) {
			c.JSON(http.StatusConflict, structs.ErrorResponse{
				Success: false,
				Message: "Nama role sudah dipakai",
				Errors:  map[string]string{"name": "sudah dipakai"},
			})
			return
		}
		respondServiceError(c, err)
		return
	}

	c.JSON(http.StatusCreated, structs.SuccessResponse[structs.RoleResponse]{
		Success: true,
		Message: "Role berhasil dibuat",
		Data:    roleToResponse(role, req.PermissionCodes),
	})
}

// UpdateRole mengubah nama/deskripsi peran. Peran bawaan (is_system) tidak boleh
// diganti namanya.
func UpdateRole(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		badRequest(c, "id", "ID role tidak boleh kosong")
		return
	}

	var req structs.RoleUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	ctx := c.Request.Context()

	var role models.Role
	if err := repositories.FindRoleInTenant(ctx, nil, id, &role); err != nil {
		if errors.Is(err, repositories.ErrRoleNotFound) {
			notFound(c, "Role tidak ditemukan")
			return
		}
		respondServiceError(c, err)
		return
	}

	if role.IsSystem && req.Name != "" && req.Name != role.Name {
		badRequest(c, "name", "Nama peran bawaan tidak boleh diubah")
		return
	}
	if req.Name != "" {
		role.Name = req.Name
	}
	if req.Description != "" {
		role.Description = req.Description
	}

	if err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		return repositories.UpdateRole(ctx, tx, &role)
	}); err != nil {
		if errors.Is(err, repositories.ErrRoleNotFound) {
			notFound(c, "Role tidak ditemukan")
			return
		}
		if helpers.IsDuplicateEntryError(err) {
			c.JSON(http.StatusConflict, structs.ErrorResponse{
				Success: false,
				Message: "Nama role sudah dipakai",
				Errors:  map[string]string{"name": "sudah dipakai"},
			})
			return
		}
		respondServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, structs.SuccessResponse[structs.RoleResponse]{
		Success: true,
		Message: "Role berhasil diperbarui",
		Data:    roleToResponse(role, nil),
	})
}

// SetRolePermissions mengganti SELURUH pemetaan permission sebuah peran.
func SetRolePermissions(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		badRequest(c, "id", "ID role tidak boleh kosong")
		return
	}

	var req structs.RolePermissionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	ctx := c.Request.Context()

	var role models.Role
	if err := repositories.FindRoleInTenant(ctx, nil, id, &role); err != nil {
		if errors.Is(err, repositories.ErrRoleNotFound) {
			notFound(c, "Role tidak ditemukan")
			return
		}
		respondServiceError(c, err)
		return
	}

	permIDs, err := repositories.PermissionIDsByCode(ctx, nil, req.PermissionCodes)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	if len(permIDs) != len(req.PermissionCodes) {
		badRequest(c, "permission_codes", "Ada kode permission yang tidak dikenal")
		return
	}

	if err := repositories.ReplaceRolePermissions(ctx, role.ID, permIDs); err != nil {
		respondServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, structs.SuccessResponse[structs.RoleResponse]{
		Success: true,
		Message: "Hak akses role berhasil diperbarui",
		Data:    roleToResponse(role, req.PermissionCodes),
	})
}

// DeleteRole menghapus (soft delete) peran non-bawaan yang tidak sedang dipakai.
func DeleteRole(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		badRequest(c, "id", "ID role tidak boleh kosong")
		return
	}

	err := repositories.DeleteRole(c.Request.Context(), id)
	switch {
	case errors.Is(err, repositories.ErrRoleNotFound):
		notFound(c, "Role tidak ditemukan")
	case errors.Is(err, repositories.ErrRoleInUse):
		c.JSON(http.StatusConflict, structs.ErrorResponse{
			Success: false,
			Message: "Role tidak dapat dihapus: masih dipakai user atau merupakan peran bawaan",
			Errors:  map[string]string{"role": "sedang dipakai"},
		})
	case err != nil:
		respondServiceError(c, err)
	default:
		c.JSON(http.StatusOK, structs.SuccessResponse[any]{
			Success: true,
			Message: "Role berhasil dihapus",
			Data:    nil,
		})
	}
}

// ListPermissions mengembalikan katalog permission untuk UI pengaturan peran.
func ListPermissions(c *gin.Context) {
	perms, err := repositories.ListPermissions(c.Request.Context())
	if err != nil {
		respondServiceError(c, err)
		return
	}
	items := make([]structs.PermissionResponse, len(perms))
	for i, p := range perms {
		items[i] = structs.PermissionResponse{Code: p.Code, GroupName: p.GroupName, Description: p.Description}
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[[]structs.PermissionResponse]{
		Success: true,
		Message: "Katalog permission",
		Data:    items,
	})
}
