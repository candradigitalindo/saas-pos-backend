package structs

type RoleCreateRequest struct {
	Name string `json:"name" binding:"required,min=2,max=50"`
}

// RoleUpdateRequest digunakan untuk menerima data saat proses update role
type RoleUpdateRequest struct {
	Name string `json:"name" binding:"required,min=2,max=50"`
}

type RoleResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}
