package controllers

import (
	"context"
	"time"

	"candra/backend-api/helpers"
	"candra/backend-api/models"
	"candra/backend-api/repositories"
	"candra/backend-api/structs"
)

// userToResponse memetakan model User ke DTO response. Terpusat di sini supaya
// tidak ada handler yang tak sengaja membocorkan field sensitif (Password) dan
// format waktunya seragam.
func userToResponse(user models.User) structs.UserResponse {
	return structs.UserResponse{
		Id:        user.ID,
		Name:      user.Name,
		Username:  user.Username,
		Email:     user.Email,
		RoleID:    user.RoleID,
		RoleName:  user.Role.Name,
		IsActive:  user.IsActive,
		CreatedAt: user.CreatedAt.Format(timeLayout),
		UpdatedAt: user.UpdatedAt.Format(timeLayout),
	}
}

// newRefreshTokenRow membuat baris refresh token baru (belum disimpan) beserta
// nilai MENTAH yang harus dikembalikan ke klien. Hanya hash-nya yang tersimpan.
func newRefreshTokenRow(userID, deviceName string) (row *models.RefreshToken, raw string, err error) {
	raw, hash, err := helpers.GenerateRefreshToken()
	if err != nil {
		return nil, "", err
	}
	row = &models.RefreshToken{
		UserID:     userID,
		TokenHash:  hash,
		DeviceName: deviceName,
		ExpiresAt:  time.Now().UTC().Add(helpers.RefreshTokenTTL()),
	}
	return row, raw, nil
}

// buildAuthResponse merangkai AuthResponse dari access token yang sudah dibuat
// dan refresh token mentah. Tidak menyentuh database.
func buildAuthResponse(user models.User, accessToken string, accessExpiry time.Time, rawRefresh string) structs.AuthResponse {
	return structs.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: rawRefresh,
		TokenType:    "Bearer",
		ExpiresIn:    int(time.Until(accessExpiry).Seconds()),
		User:         userToResponse(user),
	}
}

// issueFreshSession membuat access token + refresh token BARU untuk user (jalur
// login). Menyimpan baris refresh token, lalu mengembalikan AuthResponse.
func issueFreshSession(ctx context.Context, user models.User, deviceName string) (structs.AuthResponse, error) {
	access, expiry, err := helpers.GenerateAccessToken(user.ID)
	if err != nil {
		return structs.AuthResponse{}, err
	}
	row, raw, err := newRefreshTokenRow(user.ID, deviceName)
	if err != nil {
		return structs.AuthResponse{}, err
	}
	if err := repositories.CreateRefreshToken(ctx, row); err != nil {
		return structs.AuthResponse{}, err
	}
	return buildAuthResponse(user, access, expiry, raw), nil
}
