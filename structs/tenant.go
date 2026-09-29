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
	// Plan = paket yang SEDANG BERLAKU (bukan sekadar yang dipilih): klien
	// menyembunyikan QRIS, menandai menu terkunci, dan menampilkan ajakan naik
	// paket dari sini. Penegakan sebenarnya tetap di server.
	Plan PlanEntitlementResponse `json:"plan"`
}

// PlanEntitlementResponse adalah paket yang berlaku bagi tenant.
type PlanEntitlementResponse struct {
	Code string `json:"code"`
	Name string `json:"name"`
	// Status: "trial" | "active" | "past_due" bila langganan berlaku; "none"
	// bila tenant memakai paket Gratis (belum/tidak lagi berlangganan).
	Status      string          `json:"status"`
	ActiveUntil string          `json:"active_until,omitempty"` // RFC3339 UTC; kosong untuk Gratis
	Features    map[string]bool `json:"features"`
	MaxOutlets  *int            `json:"max_outlets"`
	MaxUsers    *int            `json:"max_users"`
	MaxProducts *int            `json:"max_products"`
	// UpgradeFor: untuk setiap fitur katalog yang TIDAK termasuk paket ini,
	// nama paket termurah yang membukanya — klien menulis "tersedia di paket
	// Basic" tanpa perlu mengambil katalog (termasuk saat kasir offline).
	UpgradeFor map[string]string `json:"upgrade_for"`
}
