package controllers

import (
	"candra/backend-api/helpers"
	"candra/backend-api/models"
	"candra/backend-api/repositories"
	"candra/backend-api/structs"
	"net/http"

	"github.com/gin-gonic/gin"
)

func GetAllUsers(c *gin.Context) {
	page, limit, offset := helpers.ParsePaginationParams(c)
	search := c.Query("search")

	// Satu pemanggilan repository mengambil data halaman + total sekaligus.
	// Context request diteruskan agar query dibatalkan bila klien memutus koneksi,
	// sehingga koneksi database segera dilepas kembali ke pool.
	users, total, err := repositories.ListUsers(c.Request.Context(), search, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, structs.ErrorResponse{
			Success: false,
			Message: "Gagal mengambil data user",
			Errors:  helpers.TranslateErrorMessage(err),
		})
		return
	}

	userResponses := make([]structs.UserResponse, len(users))
	for i, user := range users {
		userResponses[i] = structs.UserResponse{
			Id:        user.ID,
			Name:      user.Name,
			Username:  user.Username,
			Email:     user.Email,
			RoleName:  user.Role.Name,
			CreatedAt: user.CreatedAt.Format("2006-01-02 15:04:05"),
			UpdatedAt: user.UpdatedAt.Format("2006-01-02 15:04:05"),
		}
	}

	pagination := helpers.BuildPaginationResponse(c, page, limit, total, userResponses)

	c.JSON(http.StatusOK, structs.SuccessResponse[structs.PaginatedResponse[structs.UserResponse]]{
		Success: true,
		Message: "Berhasil mengambil data user",
		Data:    pagination,
	})
}

// GetUserByID
func GetUserByID(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, structs.ErrorResponse{
			Success: false,
			Message: "ID user tidak boleh kosong",
			Errors:  map[string]string{"id": "ID user tidak boleh kosong"},
		})
		return
	}

	var user models.User
	if err := repositories.FindUserByID(c.Request.Context(), id, &user); err != nil {
		c.JSON(http.StatusNotFound, structs.ErrorResponse{
			Success: false,
			Message: "User tidak ditemukan",
			Errors:  helpers.TranslateErrorMessage(err),
		})
		return
	}

	userResponse := structs.UserResponse{
		Id:        user.ID,
		Name:      user.Name,
		Username:  user.Username,
		Email:     user.Email,
		RoleName:  user.Role.Name,
		CreatedAt: user.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt: user.UpdatedAt.Format("2006-01-02 15:04:05"),
	}

	c.JSON(http.StatusOK, structs.SuccessResponse[structs.UserResponse]{
		Success: true,
		Message: "Berhasil mengambil data user",
		Data:    userResponse,
	})
}

// UpdateUser
func UpdateUser(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, structs.ErrorResponse{
			Success: false,
			Message: "ID user tidak boleh kosong",
			Errors:  map[string]string{"id": "ID user tidak boleh kosong"},
		})
		return
	}

	// 1. Bind and validate the request using the dedicated request struct
	var req structs.UserUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, structs.ErrorResponse{
			Success: false,
			Message: "Data yang dikirim tidak valid",
			Errors:  helpers.TranslateErrorMessage(err),
		})
		return
	}

	// 2. Fetch the existing user model from the database
	var user models.User
	if err := repositories.FindUserByID(c.Request.Context(), id, &user); err != nil {
		c.JSON(http.StatusNotFound, structs.ErrorResponse{
			Success: false,
			Message: "User tidak ditemukan",
			Errors:  helpers.TranslateErrorMessage(err),
		})
		return
	}

	var roleCheck models.Role // Declare here to use it later for the response
	// 3. Selectively update the model fields if they were provided in the request
	if req.Name != "" {
		user.Name = req.Name
	}
	if req.Username != "" {
		user.Username = req.Username
	}
	if req.Email != "" {
		user.Email = req.Email
	}
	if req.RoleID != "" {
		// Validasi tambahan: Pastikan RoleID yang baru ada di database sebelum di-assign
		if err := repositories.FindRoleByID(c.Request.Context(), req.RoleID, &roleCheck); err != nil {
			c.JSON(http.StatusUnprocessableEntity, structs.ErrorResponse{
				Success: false,
				Message: "Validasi gagal",
				Errors:  map[string]string{"role_id": "Role dengan ID tersebut tidak ditemukan"},
			})
			return
		}
		user.RoleID = req.RoleID
	}
	if req.Password != "" {
		hashedPassword, err := helpers.HashPassword(req.Password)
		if err != nil {
			c.JSON(http.StatusInternalServerError, structs.ErrorResponse{
				Success: false,
				Message: "Gagal memproses password",
				Errors:  map[string]string{"password": "Terjadi kesalahan saat hashing"},
			})
			return
		}
		user.Password = hashedPassword
	}

	// 4. Save the updated model
	if err := repositories.UpdateUser(c.Request.Context(), &user); err != nil {
		c.JSON(http.StatusInternalServerError, structs.ErrorResponse{
			Success: false,
			Message: "Gagal memperbarui data user",
			Errors:  helpers.TranslateErrorMessage(err),
		})
		return
	}

	// 5. Buat response tanpa query tambahan.
	// Jika role diubah, perbarui model 'user' di memori untuk response yang akurat.
	if req.RoleID != "" {
		user.Role = roleCheck
	}

	userResponse := structs.UserResponse{
		Id:        user.ID,
		Name:      user.Name,
		Username:  user.Username,
		Email:     user.Email,
		RoleName:  user.Role.Name, // Ini akan akurat karena pembaruan di atas
		CreatedAt: user.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt: user.UpdatedAt.Format("2006-01-02 15:04:05"),
	}

	c.JSON(http.StatusOK, structs.SuccessResponse[structs.UserResponse]{
		Success: true,
		Message: "Berhasil memperbarui data user",
		Data:    userResponse,
	})
}

// DeleteUser
func DeleteUser(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, structs.ErrorResponse{
			Success: false,
			Message: "ID user tidak boleh kosong",
			Errors:  map[string]string{"id": "ID user tidak boleh kosong"},
		})
		return
	}

	var user models.User
	if err := repositories.FindUserByID(c.Request.Context(), id, &user); err != nil {
		c.JSON(http.StatusNotFound, structs.ErrorResponse{
			Success: false,
			Message: "User tidak ditemukan",
			Errors:  helpers.TranslateErrorMessage(err),
		})
		return
	}

	if err := repositories.DeleteUser(c.Request.Context(), &user); err != nil {
		c.JSON(http.StatusInternalServerError, structs.ErrorResponse{
			Success: false,
			Message: "Gagal menghapus data user",
			Errors:  helpers.TranslateErrorMessage(err),
		})
		return
	}

	c.JSON(http.StatusOK, structs.SuccessResponse[any]{
		Success: true,
		Message: "Berhasil menghapus data user",
		Data:    nil,
	})
}
