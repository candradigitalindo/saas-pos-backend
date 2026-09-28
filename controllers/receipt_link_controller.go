package controllers

import (
	"net/http"

	"candra/backend-api/services"
	"candra/backend-api/structs"

	"github.com/gin-gonic/gin"
)

// CreateReceiptLink: POST /api/v1/sales/:id/receipt-link — token tautan struk
// digital (dibuat sekali, dipakai ulang).
func CreateReceiptLink(c *gin.Context) {
	token, err := services.ReceiptLinkToken(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.ReceiptLinkResponse]{
		Success: true, Message: "Tautan struk", Data: structs.ReceiptLinkResponse{Token: token},
	})
}

// PublicReceipt: GET /api/v1/public/receipts/:token — TANPA auth; tokennya
// yang menjadi kunci. Boleh disimpan cache sebentar oleh peramban pembeli.
func PublicReceipt(c *gin.Context) {
	res, err := services.PublicReceipt(c.Request.Context(), c.Param("token"))
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.Header("Cache-Control", "private, max-age=60")
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.PublicReceiptResponse]{Success: true, Message: "Struk", Data: res})
}
