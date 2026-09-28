package controllers

import (
	"errors"
	"net/http"

	"candra/backend-api/repositories"
	"candra/backend-api/services"
	"candra/backend-api/structs"

	"github.com/gin-gonic/gin"
)

// Varian barang — lihat services/product_variant_service.go.

func balasGalatVarian(c *gin.Context, err error) {
	switch {
	case errors.Is(err, repositories.ErrProductNotFound):
		notFound(c, "Produk tidak ditemukan")
	case errors.Is(err, repositories.ErrVariantNotFound):
		notFound(c, "Varian tidak ditemukan")
	default:
		respondServiceError(c, err)
	}
}

// ListProductVariants: GET /api/v1/products/:id/variants.
func ListProductVariants(c *gin.Context) {
	res, err := services.ListProductVariants(c.Request.Context(), c.Param("id"))
	if err != nil {
		balasGalatVarian(c, err)
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[[]structs.ProductVariantResponse]{Success: true, Message: "Varian barang", Data: res})
}

func simpanVarian(c *gin.Context, variantID string, kode int) {
	var req structs.ProductVariantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	res, err := services.SaveProductVariant(c.Request.Context(), c.Param("id"), variantID, req)
	if err != nil {
		balasGalatVarian(c, err)
		return
	}
	c.JSON(kode, structs.SuccessResponse[structs.ProductVariantResponse]{Success: true, Message: "Varian disimpan", Data: res})
}

// CreateProductVariant: POST /api/v1/products/:id/variants.
func CreateProductVariant(c *gin.Context) { simpanVarian(c, "", http.StatusCreated) }

// UpdateProductVariant: PUT /api/v1/products/:id/variants/:vid.
func UpdateProductVariant(c *gin.Context) { simpanVarian(c, c.Param("vid"), http.StatusOK) }

// DeleteProductVariant: DELETE /api/v1/products/:id/variants/:vid.
func DeleteProductVariant(c *gin.Context) {
	if err := services.DeleteProductVariant(c.Request.Context(), c.Param("id"), c.Param("vid")); err != nil {
		balasGalatVarian(c, err)
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[any]{Success: true, Message: "Varian dihapus", Data: nil})
}
