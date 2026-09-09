package structs

// DTO panel internal penyedia SaaS (blueprint G.5, migrasi 000035).
// Realm ketiga — terpisah dari tenant dan mitra.

type PlatformLoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type PlatformAuthResponse struct {
	AccessToken string                `json:"access_token"`
	ExpiresAt   string                `json:"expires_at"`
	Admin       PlatformAdminResponse `json:"admin"`
}

type PlatformAdminResponse struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Email        string   `json:"email"`
	Role         string   `json:"role"`
	Capabilities []string `json:"capabilities"` // supaya panel bisa menyembunyikan menu
	IsActive     bool     `json:"is_active"`
	LastLoginAt  string   `json:"last_login_at,omitempty"`
}

type PlatformAdminCreateRequest struct {
	Name     string `json:"name" binding:"required,min=2,max=120"`
	Email    string `json:"email" binding:"required,email"`
	Role     string `json:"role" binding:"required,oneof=superadmin operator finance support"`
	Password string `json:"password" binding:"omitempty,min=10"` // kosong → dibuatkan
}

type PlatformAdminCreateResponse struct {
	Admin             PlatformAdminResponse `json:"admin"`
	GeneratedPassword string                `json:"generated_password,omitempty"`
}

type PlatformAdminActiveRequest struct {
	IsActive *bool `json:"is_active" binding:"required"`
}

// PlatformCommissionRunRequest menjalankan siklus komisi dari panel.
type PlatformCommissionRunRequest struct {
	PeriodStart string `json:"period_start" binding:"required"` // YYYY-MM-DD
	PeriodEnd   string `json:"period_end" binding:"required"`
	PartnerID   string `json:"partner_id" binding:"omitempty,ulid"` // kosong = semua mitra
	Approve     bool   `json:"approve"`
	Payout      bool   `json:"payout"`
}

type PlatformPayoutRequest struct {
	PartnerID   string `json:"partner_id" binding:"required,ulid"`
	PeriodStart string `json:"period_start" binding:"required"`
	PeriodEnd   string `json:"period_end" binding:"required"`
}
