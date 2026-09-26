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
	// Outlets = rincian cabang pada OutletIDs, urutan sama. Kasir butuh
	// pengaturan harga cabangnya (pajak, biaya layanan) untuk menghitung total
	// yang PERSIS sama dengan server — termasuk saat offline, ketika total
	// itulah yang dibayar pas lewat QRIS/kasbon — padahal kasir tidak boleh
	// membaca GET /outlets (butuh outlet.manage).
	Outlets []OutletResponse `json:"outlets"`
}
