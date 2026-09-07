package structs

// TenantResponse adalah bentuk publik data usaha. Tidak memuat kolom internal
// seperti referral / status pembayaran mentah.
type TenantResponse struct {
	ID           string `json:"id"`
	BusinessName string `json:"business_name"`
	BusinessType string `json:"business_type"`
	OwnerName    string `json:"owner_name"`
	Phone        string `json:"phone"`
	Email        string `json:"email,omitempty"`
	Status       string `json:"status"`
	CreatedAt    string `json:"created_at"`
}

// MeResponse adalah profil user permintaan: identitas, tenant, peran, izin, dan
// outlet yang boleh diaksesnya. Dipakai klien untuk menyusun menu/izin UI.
type MeResponse struct {
	User        UserResponse   `json:"user"`
	Tenant      TenantResponse `json:"tenant"`
	Permissions []string       `json:"permissions"`
	OutletIDs   []string       `json:"outlet_ids"`
}
