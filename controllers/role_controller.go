package controllers

import (
	"candra/backend-api/helpers"
	"candra/backend-api/models"
	"candra/backend-api/repositories"
	"candra/backend-api/structs"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Tampilkan semua role dengan pagination
func GetAllRoles(c *gin.Context) {
	page, limit, offset := helpers.ParsePaginationParams(c)
	search := c.Query("search")

	// Satu pemanggilan repository mengambil data halaman + total sekaligus,
	// dengan context request agar query batal saat klien memutus koneksi.
	roles, total, err := repositories.ListRoles(c.Request.Context(), search, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, structs.ErrorResponse{
			Success: false,
			Message: "Gagal mengambil data role",
			Errors:  helpers.TranslateErrorMessage(err),
		})
		return
	}

	roleResponses := make([]structs.RoleResponse, len(roles))
	for i, role := range roles {
		roleResponses[i] = structs.RoleResponse{
			ID:        role.ID,
			Name:      role.Name,
			CreatedAt: role.CreatedAt.Format("2006-01-02 15:04:05"),
			UpdatedAt: role.UpdatedAt.Format("2006-01-02 15:04:05"),
		}
	}

	pagination := helpers.BuildPaginationResponse(c, page, limit, total, roleResponses)

	c.JSON(http.StatusOK, structs.SuccessResponse[structs.PaginatedResponse[structs.RoleResponse]]{
		Success: true,
		Message: "Berhasil mengambil data role",
		Data:    pagination,
	})
}

// Fungsi untuk membuat role baru
func CreateRole(c *gin.Context) {
	var req structs.RoleCreateRequest

	// Validasi input
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, structs.ErrorResponse{
			Success: false,
			Message: "Validasi gagal",
			Errors:  helpers.TranslateErrorMessage(err),
		})
		return
	}

	role := models.Role{Name: req.Name}

	if err := repositories.CreateRole(c.Request.Context(), &role); err != nil {
		// Tentukan status HTTP dan pesan umum berdasarkan jenis error
		status := http.StatusInternalServerError
		message := "Gagal membuat role"

		if helpers.IsDuplicateEntryError(err) {
			status = http.StatusConflict
			message = "Role dengan nama ini sudah ada"
		}

		// Selalu gunakan TranslateErrorMessage untuk mendapatkan detail error yang konsisten
		c.JSON(status, structs.ErrorResponse{Success: false, Message: message, Errors: helpers.TranslateErrorMessage(err)})
		return
	}

	c.JSON(http.StatusCreated, structs.SuccessResponse[structs.RoleResponse]{
		Success: true,
		Message: "Role berhasil dibuat",
		Data: structs.RoleResponse{
			ID:        role.ID,
			Name:      role.Name,
			CreatedAt: role.CreatedAt.Format("2006-01-02 15:04:05"),
			UpdatedAt: role.UpdatedAt.Format("2006-01-02 15:04:05"),
		},
	})
}

// Fungsi untuk mengambil detail role berdasarkan ID
func GetRoleByID(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, structs.ErrorResponse{
			Success: false,
			Message: "ID role tidak boleh kosong",
			Errors:  map[string]string{"id": "ID role tidak boleh kosong"},
		})
		return
	}

	var role models.Role
	if err := repositories.FindRoleByID(c.Request.Context(), id, &role); err != nil {
		c.JSON(http.StatusNotFound, structs.ErrorResponse{
			Success: false,
			Message: "Data role tidak ditemukan",
			Errors:  helpers.TranslateErrorMessage(err),
		})
		return
	}

	c.JSON(http.StatusOK, structs.SuccessResponse[structs.RoleResponse]{
		Success: true,
		Message: "Berhasil mengambil data role",
		Data: structs.RoleResponse{
			ID:        role.ID,
			Name:      role.Name,
			CreatedAt: role.CreatedAt.Format("2006-01-02 15:04:05"),
			UpdatedAt: role.UpdatedAt.Format("2006-01-02 15:04:05"),
		},
	})
}

// Fungsi untuk mengupdate role
func UpdateRole(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, structs.ErrorResponse{
			Success: false,
			Message: "ID role tidak boleh kosong",
			Errors:  map[string]string{"id": "ID role tidak boleh kosong"},
		})
		return
	}

	var req structs.RoleUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, structs.ErrorResponse{
			Success: false,
			Message: "Validasi gagal",
			Errors:  helpers.TranslateErrorMessage(err),
		})
		return
	}

	// 1. Ambil model role yang ada dari database
	var role models.Role
	if err := repositories.FindRoleByID(c.Request.Context(), id, &role); err != nil {
		c.JSON(http.StatusNotFound, structs.ErrorResponse{
			Success: false,
			Message: "Data role tidak ditemukan",
			Errors:  helpers.TranslateErrorMessage(err),
		})
		return
	}

	// 2. Perbarui field pada model dari request
	role.Name = req.Name

	// 3. Simpan model yang sudah diperbarui
	if err := repositories.UpdateRoleModel(c.Request.Context(), &role); err != nil {
		c.JSON(http.StatusInternalServerError, structs.ErrorResponse{
			Success: false,
			Message: "Gagal mengupdate role",
			Errors:  helpers.TranslateErrorMessage(err),
		})
		return
	}

	c.JSON(http.StatusOK, structs.SuccessResponse[structs.RoleResponse]{
		Success: true,
		Message: "Role berhasil diupdate",
		Data: structs.RoleResponse{
			ID:        role.ID,
			Name:      role.Name,
			CreatedAt: role.CreatedAt.Format("2006-01-02 15:04:05"),
			UpdatedAt: role.UpdatedAt.Format("2006-01-02 15:04:05"),
		},
	})
}

// Fungsi untuk menghapus role
func DeleteRole(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, structs.ErrorResponse{
			Success: false,
			Message: "ID role tidak boleh kosong",
			Errors:  map[string]string{"id": "ID role tidak boleh kosong"},
		})
		return
	}

	// Tambahkan pengecekan: Pastikan role ada sebelum mencoba menghapus.
	// Ini untuk memberikan response 404 yang benar jika ID tidak ditemukan.
	var role models.Role
	if err := repositories.FindRoleByID(c.Request.Context(), id, &role); err != nil {
		c.JSON(http.StatusNotFound, structs.ErrorResponse{
			Success: false,
			Message: "Data role tidak ditemukan",
			Errors:  helpers.TranslateErrorMessage(err),
		})
		return
	}

	err := repositories.DeleteRole(c.Request.Context(), id)
	if err != nil {
		// Periksa custom error secara spesifik untuk penanganan yang lebih andal
		if errors.Is(err, repositories.ErrRoleInUse) {
			c.JSON(http.StatusConflict, structs.ErrorResponse{
				Success: false,
				Message: "Role tidak dapat dihapus karena sedang digunakan oleh user.",
				Errors:  map[string]string{"role": err.Error()},
			})
			return
		}
		// Tangani error potensial lainnya
		c.JSON(http.StatusInternalServerError, structs.ErrorResponse{
			Success: false,
			Message: "Gagal menghapus role",
			Errors:  helpers.TranslateErrorMessage(err),
		})
		return
	}

	c.JSON(http.StatusOK, structs.SuccessResponse[any]{
		Success: true,
		Message: "Role berhasil dihapus",
		Data:    nil, // Menggunakan 'any' untuk response tanpa data
	})
}
