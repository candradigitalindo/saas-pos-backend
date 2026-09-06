package structs

// SuccessResponse adalah struct generik untuk response API yang berhasil.
// Penggunaan generics [T any] memastikan type-safety untuk field Data.
type SuccessResponse[T any] struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}
