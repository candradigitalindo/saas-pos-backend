package structs

// AuthResponse adalah bentuk baku hasil login dan refresh.
//
// access_token dipakai di header Authorization untuk permintaan berikutnya.
// refresh_token disimpan aman oleh klien dan HANYA dikirim ke /auth/refresh.
// expires_in adalah sisa umur access token dalam detik — klien memakainya untuk
// menjadwalkan refresh sebelum kedaluwarsa.
type AuthResponse struct {
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	TokenType    string       `json:"token_type"` // selalu "Bearer"
	ExpiresIn    int          `json:"expires_in"` // detik
	User         UserResponse `json:"user"`
}

// RefreshRequest adalah body untuk POST /auth/refresh.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// LogoutRequest adalah body untuk POST /auth/logout.
type LogoutRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}
