package structs

// RoleCreateRequest — pembuatan peran baru dalam tenant.
type RoleCreateRequest struct {
	Name            string   `json:"name" binding:"required,min=2,max=50"`
	Description     string   `json:"description" binding:"omitempty,max=255"`
	PermissionCodes []string `json:"permission_codes" binding:"omitempty,dive,min=1"`
}

// RoleUpdateRequest — perubahan nama/deskripsi peran. Pemetaan permission diubah
// lewat endpoint terpisah (PUT /roles/:id/permissions).
type RoleUpdateRequest struct {
	Name        string `json:"name" binding:"omitempty,min=2,max=50"`
	Description string `json:"description" binding:"omitempty,max=255"`
}

// RolePermissionsRequest — mengganti SELURUH pemetaan permission sebuah peran.
type RolePermissionsRequest struct {
	PermissionCodes []string `json:"permission_codes" binding:"required,dive,min=1"`
}

// RoleResponse adalah bentuk publik data peran.
type RoleResponse struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Description     string   `json:"description,omitempty"`
	IsSystem        bool     `json:"is_system"`
	PermissionCodes []string `json:"permission_codes,omitempty"`
	CreatedAt       string   `json:"created_at"`
	UpdatedAt       string   `json:"updated_at"`
}

// PermissionResponse adalah satu entri katalog permission untuk UI pengaturan.
type PermissionResponse struct {
	Code        string `json:"code"`
	GroupName   string `json:"group_name"`
	Description string `json:"description"`
}
