package controllers

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"candra/backend-api/helpers"
	"candra/backend-api/middlewares"
	"candra/backend-api/models"
	"candra/backend-api/repositories"
	"candra/backend-api/structs"

	"github.com/gin-gonic/gin"
)

// Register menangani registrasi user baru (endpoint publik).
//
// KEAMANAN: registrasi mandiri SELALU mendapat role default 'user'. role_id dari
// input SENGAJA diabaikan agar penyerang anonim tidak bisa mendaftar sebagai
// admin (§4 CONVENTIONS). Penetapan role lain hanya lewat endpoint admin.
func Register(c *gin.Context) {
	var req structs.UserCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, structs.ErrorResponse{
			Success: false,
			Message: "Validasi gagal",
			Errors:  helpers.TranslateErrorMessage(err),
		})
		return
	}

	const defaultRoleName = "user"
	var roleCheck models.Role
	if err := repositories.FindRoleByName(c.Request.Context(), defaultRoleName, &roleCheck); err != nil {
		helpers.LoggerFromContext(c.Request.Context()).Error("role default tidak tersedia", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, structs.ErrorResponse{
			Success: false,
			Message: "Registrasi belum bisa diproses",
			Errors:  map[string]string{"role": "Role default 'user' belum di-seed"},
		})
		return
	}

	hashedPassword, err := helpers.HashPassword(req.Password)
	if err != nil {
		helpers.LoggerFromContext(c.Request.Context()).Error("gagal hash password", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, structs.ErrorResponse{
			Success: false,
			Message: "Registrasi belum bisa diproses",
			Errors:  map[string]string{"password": "Gagal memproses password"},
		})
		return
	}

	user := models.User{
		Name:     req.Name,
		Username: req.Username,
		Email:    req.Email,
		RoleID:   roleCheck.ID, // selalu role default, bukan dari input
		Password: hashedPassword,
	}
	if err := repositories.CreateUser(c.Request.Context(), &user); err != nil {
		status, message := http.StatusInternalServerError, "Gagal membuat user"
		if helpers.IsDuplicateEntryError(err) {
			status, message = http.StatusConflict, "Username atau email sudah terdaftar"
		}
		c.JSON(status, structs.ErrorResponse{Success: false, Message: message, Errors: helpers.TranslateErrorMessage(err)})
		return
	}

	user.Role = roleCheck // agar userToResponse punya nama role tanpa query ulang
	c.JSON(http.StatusCreated, structs.SuccessResponse[structs.UserResponse]{
		Success: true,
		Message: "User berhasil dibuat",
		Data:    userToResponse(user),
	})
}

// Login memverifikasi kredensial lalu menerbitkan access token (15 mnt) +
// refresh token (30 hari). Username tidak ditemukan dan password salah
// menghasilkan pesan DAN waktu respon yang sama (§4): bcrypt tetap dijalankan
// dengan hash dummy saat user tidak ada.
func Login(c *gin.Context) {
	var req structs.UserLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, structs.ErrorResponse{
			Success: false,
			Message: "Validasi gagal",
			Errors:  helpers.TranslateErrorMessage(err),
		})
		return
	}

	var user models.User
	err := repositories.FindUserByUsername(c.Request.Context(), req.Username, &user)
	if err != nil {
		helpers.CheckPassword(req.Password, helpers.DummyPasswordHash)
		unauthorized(c, "Username atau password salah")
		return
	}
	if err := helpers.CheckPassword(req.Password, user.Password); err != nil {
		unauthorized(c, "Username atau password salah")
		return
	}

	auth, err := issueFreshSession(c.Request.Context(), user, req.DeviceName)
	if err != nil {
		helpers.LoggerFromContext(c.Request.Context()).Error("gagal menerbitkan sesi", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, structs.ErrorResponse{
			Success: false,
			Message: "Gagal membuat sesi login",
			Errors:  map[string]string{"token": "Gagal membuat token"},
		})
		return
	}

	c.JSON(http.StatusOK, structs.SuccessResponse[structs.AuthResponse]{
		Success: true,
		Message: "Login berhasil",
		Data:    auth,
	})
}

// Refresh menukar refresh token dengan sepasang token baru (rotasi).
//
// Deteksi pencurian: bila refresh token yang SUDAH dicabut/dirotasi dipakai
// lagi, seluruh sesi user itu dicabut dan permintaan ditolak — pemilik sah harus
// login ulang, penyerang kehilangan akses (§9).
func Refresh(c *gin.Context) {
	var req structs.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, structs.ErrorResponse{
			Success: false,
			Message: "Validasi gagal",
			Errors:  helpers.TranslateErrorMessage(err),
		})
		return
	}

	ctx := c.Request.Context()
	log := helpers.LoggerFromContext(ctx)

	var current models.RefreshToken
	err := repositories.FindRefreshTokenByHash(ctx, helpers.HashRefreshToken(req.RefreshToken), &current)
	if errors.Is(err, repositories.ErrRefreshTokenNotFound) {
		unauthorized(c, "Refresh token tidak valid")
		return
	}
	if err != nil {
		log.Error("gagal membaca refresh token", slog.Any("error", err))
		internalError(c)
		return
	}

	if current.RevokedAt != nil {
		// Token mati dipakai lagi → indikasi pencurian.
		if e := repositories.RevokeAllUserRefreshTokens(ctx, current.UserID); e != nil {
			log.Error("gagal mencabut seluruh sesi setelah deteksi pemakaian ulang", slog.Any("error", e))
		}
		log.Warn("refresh token dipakai ulang — seluruh sesi user dicabut", slog.String("user_id", current.UserID))
		unauthorized(c, "Sesi tidak valid. Silakan login ulang.")
		return
	}
	if !current.IsActive(time.Now().UTC()) {
		unauthorized(c, "Sesi kedaluwarsa. Silakan login ulang.")
		return
	}

	var user models.User
	if err := repositories.FindUserByID(ctx, current.UserID, &user); err != nil {
		// User hilang / non-aktif: matikan token ini, minta login ulang.
		_ = repositories.RevokeRefreshToken(ctx, current.ID)
		unauthorized(c, "Sesi tidak valid. Silakan login ulang.")
		return
	}

	access, expiry, err := helpers.GenerateAccessToken(user.ID)
	if err != nil {
		log.Error("gagal membuat access token saat refresh", slog.Any("error", err))
		internalError(c)
		return
	}
	newRow, raw, err := newRefreshTokenRow(user.ID, current.DeviceName)
	if err != nil {
		log.Error("gagal membuat refresh token saat rotasi", slog.Any("error", err))
		internalError(c)
		return
	}
	if err := repositories.RotateRefreshToken(ctx, current.ID, newRow); err != nil {
		if errors.Is(err, repositories.ErrRefreshTokenReused) {
			if e := repositories.RevokeAllUserRefreshTokens(ctx, user.ID); e != nil {
				log.Error("gagal mencabut seluruh sesi setelah rotasi ganda", slog.Any("error", e))
			}
			log.Warn("rotasi refresh token bertabrakan — seluruh sesi user dicabut", slog.String("user_id", user.ID))
			unauthorized(c, "Sesi tidak valid. Silakan login ulang.")
			return
		}
		log.Error("gagal merotasi refresh token", slog.Any("error", err))
		internalError(c)
		return
	}

	c.JSON(http.StatusOK, structs.SuccessResponse[structs.AuthResponse]{
		Success: true,
		Message: "Token diperbarui",
		Data:    buildAuthResponse(user, access, expiry, raw),
	})
}

// Logout mencabut refresh token yang dikirim. Dijalankan di balik AuthMiddleware.
// Idempoten dan tidak menjadi oracle: selalu membalas 200 walau token tidak
// ditemukan atau bukan milik pemanggil.
func Logout(c *gin.Context) {
	var req structs.LogoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, structs.ErrorResponse{
			Success: false,
			Message: "Validasi gagal",
			Errors:  helpers.TranslateErrorMessage(err),
		})
		return
	}

	ctx := c.Request.Context()
	callerID := middlewares.UserIDFromGin(c)

	var rt models.RefreshToken
	if err := repositories.FindRefreshTokenByHash(ctx, helpers.HashRefreshToken(req.RefreshToken), &rt); err == nil {
		if rt.UserID == callerID {
			if e := repositories.RevokeRefreshToken(ctx, rt.ID); e != nil {
				helpers.LoggerFromContext(ctx).Error("gagal mencabut refresh token saat logout", slog.Any("error", e))
			}
		}
	}

	c.JSON(http.StatusOK, structs.SuccessResponse[any]{
		Success: true,
		Message: "Logout berhasil",
		Data:    nil,
	})
}

// unauthorized membalas 401 dengan pesan seragam.
func unauthorized(c *gin.Context, msg string) {
	c.JSON(http.StatusUnauthorized, structs.ErrorResponse{
		Success: false,
		Message: msg,
		Errors:  map[string]string{"auth": msg},
	})
}

// internalError membalas 500 generik tanpa membocorkan detail.
func internalError(c *gin.Context) {
	c.JSON(http.StatusInternalServerError, structs.ErrorResponse{
		Success: false,
		Message: "Terjadi kesalahan internal",
		Errors:  map[string]string{"server": "Terjadi kesalahan internal"},
	})
}
