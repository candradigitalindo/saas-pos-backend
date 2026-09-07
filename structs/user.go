package structs

// UserResponse adalah bentuk publik data user. RoleName, bukan RoleID, agar
// klien tidak perlu resolve terpisah. Token diisi hanya pada endpoint auth lama
// (dipertahankan untuk kompatibilitas; alur baru memakai AuthResponse).
type UserResponse struct {
	Id        string  `json:"id"`
	Name      string  `json:"name"`
	Username  string  `json:"username"`
	Email     string  `json:"email"`
	RoleID    string  `json:"role_id,omitempty"`
	RoleName  string  `json:"role_name"`
	IsActive  bool    `json:"is_active"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
	Token     *string `json:"token,omitempty"`
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
	RoleID   string `json:"role_id" binding:"required,ulid"` // harus role milik tenant yang sama (dicek di controller)
}

// UserUpdateRequest — perubahan user staf. omitempty: field yang tidak dikirim
// tidak diubah.
type UserUpdateRequest struct {
	Name     string `json:"name" binding:"omitempty,min=2,max=100"`
	Username string `json:"username" binding:"omitempty,min=3,max=50"`
	Email    string `json:"email" binding:"omitempty,email"`
	Password string `json:"password" binding:"omitempty,min=8"`
	RoleID   string `json:"role_id" binding:"omitempty,ulid"`
	IsActive *bool  `json:"is_active" binding:"omitempty"`
}

// Struct ini digunakan saat user melakukan proses login.
// DeviceName opsional: label perangkat yang ikut disimpan di refresh token
// sehingga user bisa mengenali sesinya di daftar "perangkat aktif".
type UserLoginRequest struct {
	Username   string `json:"username" binding:"required"`
	Password   string `json:"password" binding:"required"`
	DeviceName string `json:"device_name" binding:"omitempty,max=100"`
}
