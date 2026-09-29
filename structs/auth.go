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

// RegisterOwner adalah blok data akun pemilik di dalam RegisterRequest.
type RegisterOwner struct {
	Name       string `json:"name" binding:"required,min=2,max=100"`
	Username   string `json:"username" binding:"required,min=3,max=50"`
	Email      string `json:"email" binding:"required,email"`
	Password   string `json:"password" binding:"required,min=8"`
	DeviceName string `json:"device_name" binding:"omitempty,max=100"`
}

// RegisterRequest adalah body untuk POST /api/v1/auth/register — pendaftaran
// usaha baru: membuat tenant + outlet pertama + peran bawaan + user pemilik.
type RegisterRequest struct {
	BusinessName     string `json:"business_name" binding:"required,min=2,max=150"`
	BusinessType     string `json:"business_type" binding:"required,oneof=retail fnb service wholesale other"`
	Phone            string `json:"phone" binding:"required,min=5,max=30"`
	OutletName       string `json:"outlet_name" binding:"required,min=1,max=100"`
	Timezone         string `json:"timezone" binding:"omitempty,oneof=Asia/Jakarta Asia/Makassar Asia/Jayapura"`
	BusinessDayStart string `json:"business_day_start" binding:"omitempty"`
	ReferralCode     string `json:"referral_code" binding:"omitempty,max=50"`

	Owner RegisterOwner `json:"owner"`
}

// RegisterResponse adalah hasil pendaftaran usaha: entitas yang lahir + sesi
// login untuk pemilik (sudah langsung masuk).
type RegisterResponse struct {
	Tenant TenantResponse `json:"tenant"`
	Outlet OutletResponse `json:"outlet"`
	Auth   AuthResponse   `json:"auth"`
}

// RefreshRequest adalah body untuk POST /auth/refresh.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// LogoutRequest adalah body untuk POST /auth/logout.
type LogoutRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}
