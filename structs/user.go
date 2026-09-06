package structs

// Struct ini digunakan untuk menampilkan data user sebagai response API
type UserResponse struct {
	Id        string  `json:"id"`
	Name      string  `json:"name"`
	Username  string  `json:"username"`
	Email     string  `json:"email"`
	RoleName  string  `json:"role_name"` // Menampilkan nama role, bukan ID
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
	Token     *string `json:"token,omitempty"`
}

// Struct ini digunakan untuk menerima data saat proses registrasi mandiri.
// Catatan: role_id sengaja tidak ada di sini — registrasi selalu mendapat role
// default 'user' untuk mencegah privilege escalation.
type UserCreateRequest struct {
	Name     string `json:"name" binding:"required"`
	Username string `json:"username" binding:"required,min=3,max=50"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

// Struct ini digunakan untuk menerima data saat proses update user
type UserUpdateRequest struct {
	Name     string `json:"name" binding:"omitempty,min=1"`
	Username string `json:"username" binding:"omitempty,min=1"`
	Email    string `json:"email" binding:"omitempty,email"`
	Password string `json:"password" binding:"omitempty,min=6"`
	RoleID   string `json:"role_id" binding:"omitempty"` // Validasi keberadaan ULID dilakukan di logic, bukan di binding
}

// Struct ini digunakan saat user melakukan proses login.
// DeviceName opsional: label perangkat yang ikut disimpan di refresh token
// sehingga user bisa mengenali sesinya di daftar "perangkat aktif".
type UserLoginRequest struct {
	Username   string `json:"username" binding:"required"`
	Password   string `json:"password" binding:"required"`
	DeviceName string `json:"device_name" binding:"omitempty,max=100"`
}
