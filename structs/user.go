package structs

// UserResponse adalah bentuk publik data user. RoleName, bukan RoleID, agar
// klien tidak perlu resolve terpisah. Token diisi hanya pada endpoint auth lama
// (dipertahankan untuk kompatibilitas; alur baru memakai AuthResponse).
type UserResponse struct {
	Id       string `json:"id"`
	Name     string `json:"name"`
	Username string `json:"username"`
	Email    string `json:"email"`
	RoleID   string `json:"role_id,omitempty"` // peran UTAMA
	RoleName string `json:"role_name"`
	// RoleIDs = SELURUH peran yang dipegang (termasuk peran utama). Izin
	// efektifnya adalah gabungan izin peran-peran ini.
	RoleIDs []string `json:"role_ids,omitempty"`
	// OutletIDs = cabang yang boleh dipakai user ini (user_outlets). Pemegang
	// outlet.manage tetap boleh bekerja di SEMUA cabang walau daftar ini
	// lebih sempit.
	OutletIDs []string `json:"outlet_ids,omitempty"`
	IsActive  bool     `json:"is_active"`
	CreatedAt string   `json:"created_at"`
	UpdatedAt string   `json:"updated_at"`
	Token     *string  `json:"token,omitempty"`
}

// UserCreateRequest — pembuatan user staf oleh pemilik/manajer
// (POST /api/v1/users, butuh permission user.manage). role_id di sini AMAN
// karena endpoint-nya ber-otorisasi; berbeda dengan registrasi publik yang tidak
// pernah menerima role dari input.
type UserCreateRequest struct {
	Name     string `json:"name" binding:"required,min=2,max=100"`
	Username string `json:"username" binding:"required,min=3,max=50"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
	RoleID   string `json:"role_id" binding:"required,ulid"` // peran UTAMA; harus milik tenant yang sama (dicek di controller)
	// RoleIDs = peran TAMBAHAN, opsional. Izin efektif = gabungan peran utama +
	// peran tambahan (migrasi 000034). Kosong = hanya peran utama.
	RoleIDs []string `json:"role_ids" binding:"omitempty,dive,ulid"`
	// OutletIDs = cabang tempat staf ini boleh bekerja. Kosong/tidak dikirim =
	// SEMUA cabang aktif saat ini — usaha satu cabang (mayoritas UMKM) tidak
	// perlu memikirkannya, dan staf barunya langsung bisa membuka kasir.
	OutletIDs []string `json:"outlet_ids" binding:"omitempty,dive,ulid"`
}

// UserUpdateRequest — perubahan user staf. omitempty: field yang tidak dikirim
// tidak diubah.
type UserUpdateRequest struct {
	Name     string `json:"name" binding:"omitempty,min=2,max=100"`
	Username string `json:"username" binding:"omitempty,min=3,max=50"`
	Email    string `json:"email" binding:"omitempty,email"`
	Password string `json:"password" binding:"omitempty,min=8"`
	RoleID   string `json:"role_id" binding:"omitempty,ulid"`
	// RoleIDs dikirim = ganti SELURUH peran tambahan. Tidak dikirim = peran
	// tidak diubah sama sekali (pakai pointer agar "kirim array kosong" —
	// artinya hapus semua peran tambahan — bisa dibedakan dari "tidak dikirim").
	RoleIDs  *[]string `json:"role_ids" binding:"omitempty,dive,ulid"`
	IsActive *bool     `json:"is_active" binding:"omitempty"`
	// OutletIDs dikirim = ganti SELURUH akses cabang (minimal satu). Tidak
	// dikirim = tidak diubah.
	OutletIDs *[]string `json:"outlet_ids" binding:"omitempty,min=1,dive,ulid"`
}

// Struct ini digunakan saat user melakukan proses login.
// Username boleh diisi NAMA PENGGUNA ATAU EMAIL — namanya tetap "username"
// supaya klien lama tidak perlu diubah (lihat repositories.FindUserForLogin).
// DeviceName opsional: label perangkat yang ikut disimpan di refresh token
// sehingga user bisa mengenali sesinya di daftar "perangkat aktif".
type UserLoginRequest struct {
	Username   string `json:"username" binding:"required"`
	Password   string `json:"password" binding:"required"`
	DeviceName string `json:"device_name" binding:"omitempty,max=100"`
}
