package controllers

import (
	"net/http"

	"candra/backend-api/services"
	"candra/backend-api/structs"

	"github.com/gin-gonic/gin"
)

// Daftar harga khusus (member, reseller).

// ListPriceLists: GET /api/v1/price-lists.
func ListPriceLists(c *gin.Context) {
	res, err := services.ListPriceLists(c.Request.Context())
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[[]structs.PriceListResponse]{Success: true, Message: "Daftar harga", Data: res})
}

// CreatePriceList: POST /api/v1/price-lists.
func CreatePriceList(c *gin.Context) {
	var req structs.PriceListRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	res, err := services.CreatePriceList(c.Request.Context(), req.Name)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, structs.SuccessResponse[structs.PriceListResponse]{Success: true, Message: "Daftar harga dibuat", Data: res})
}

// DeletePriceList: DELETE /api/v1/price-lists/:id.
func DeletePriceList(c *gin.Context) {
	if err := services.DeletePriceList(c.Request.Context(), c.Param("id")); err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[any]{Success: true, Message: "Daftar harga dihapus"})
}
