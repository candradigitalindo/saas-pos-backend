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

// resolveParentCategory memvalidasi calon induk kategori: harus ada di tenant
// dan bukan sub-kategori (batas dua tingkat, §5.4). Mengembalikan pointer id
// yang siap disimpan, atau error yang sudah dibalas ke klien (return true).
func resolveParentCategory(c *gin.Context, parentID string, selfID string) (*string, bool) {
	if parentID == "" {
		return nil, false
	}
	if parentID == selfID {
		badRequest(c, "parent_id", "Kategori tidak bisa menjadi induk dirinya sendiri")
		return nil, true
	}
	var parent models.Category
	if err := repositories.FindCategoryInTenant(c.Request.Context(), nil, parentID, &parent); err != nil {
		badRequest(c, "parent_id", "Kategori induk tidak ditemukan")
		return nil, true
	}
	if parent.ParentID != nil {
		badRequest(c, "parent_id", "Kategori hanya boleh dua tingkat")
		return nil, true
	}
	return &parentID, false
}

// ListCategories mengembalikan kategori tenant, berpaginasi.
func ListCategories(c *gin.Context) {
	page, limit, offset := helpers.ParsePaginationParams(c)
	rows, total, err := repositories.ListCategories(c.Request.Context(), c.Query("search"), limit, offset)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	items := make([]structs.CategoryResponse, len(rows))
	for i, r := range rows {
		items[i] = categoryToResponse(r)
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.PaginatedResponse[structs.CategoryResponse]]{
		Success: true, Message: "Berhasil mengambil data kategori",
		Data: helpers.BuildPaginationResponse(c, page, limit, total, items),
	})
}

// GetCategory mengembalikan satu kategori tenant.
func GetCategory(c *gin.Context) {
	var row models.Category
	if err := repositories.FindCategoryInTenant(c.Request.Context(), nil, c.Param("id"), &row); err != nil {
		notFoundOr(c, err, repositories.ErrCategoryNotFound, "Kategori tidak ditemukan")
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.CategoryResponse]{
		Success: true, Message: "Berhasil mengambil data kategori", Data: categoryToResponse(row),
	})
}

// CreateCategory menambah kategori.
func CreateCategory(c *gin.Context) {
	var req structs.CategoryCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	parent, handled := resolveParentCategory(c, req.ParentID, "")
	if handled {
		return
	}

	ctx := c.Request.Context()
	row := models.Category{
		TenantID:  reqctx.TenantID(ctx),
		ParentID:  parent,
		Name:      req.Name,
		SortOrder: req.SortOrder,
	}
	if err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		return repositories.CreateCategory(ctx, tx, &row)
	}); err != nil {
		if helpers.IsDuplicateEntryError(err) {
			conflict(c, "name", "Nama kategori sudah dipakai pada tingkat yang sama")
			return
		}
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, structs.SuccessResponse[structs.CategoryResponse]{
		Success: true, Message: "Kategori berhasil dibuat", Data: categoryToResponse(row),
	})
}

// UpdateCategory mengubah kategori.
func UpdateCategory(c *gin.Context) {
	id := c.Param("id")
	var req structs.CategoryUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}

	ctx := c.Request.Context()
	var row models.Category
	if err := repositories.FindCategoryInTenant(ctx, nil, id, &row); err != nil {
		notFoundOr(c, err, repositories.ErrCategoryNotFound, "Kategori tidak ditemukan")
		return
	}

	if req.Name != nil {
		row.Name = *req.Name
	}
	if req.SortOrder != nil {
		row.SortOrder = *req.SortOrder
	}
	if req.ParentID != nil {
		parent, handled := resolveParentCategory(c, *req.ParentID, id)
		if handled {
			return
		}
		row.ParentID = parent
	}

	if err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		return repositories.UpdateCategory(ctx, tx, &row)
	}); err != nil {
		if helpers.IsDuplicateEntryError(err) {
			conflict(c, "name", "Nama kategori sudah dipakai pada tingkat yang sama")
			return
		}
		notFoundOr(c, err, repositories.ErrCategoryNotFound, "Kategori tidak ditemukan")
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.CategoryResponse]{
		Success: true, Message: "Kategori berhasil diperbarui", Data: categoryToResponse(row),
	})
}

// DeleteCategory menghapus (soft delete) kategori yang tidak dipakai.
func DeleteCategory(c *gin.Context) {
	err := repositories.DeleteCategory(c.Request.Context(), c.Param("id"))
	switch {
	case errors.Is(err, repositories.ErrCategoryNotFound):
		notFound(c, "Kategori tidak ditemukan")
	case errors.Is(err, repositories.ErrCategoryInUse):
		conflict(c, "category", "Kategori masih dipakai produk atau punya sub-kategori")
	case err != nil:
		respondServiceError(c, err)
	default:
		c.JSON(http.StatusOK, structs.SuccessResponse[any]{Success: true, Message: "Kategori berhasil dihapus", Data: nil})
	}
}
