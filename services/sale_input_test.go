package services

import (
	"testing"
	"time"

	"candra/backend-api/structs"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func TestBuildCheckoutInput(t *testing.T) {
	// Contoh data dari request
	req := structs.CheckoutRequest{
		ID:            "01ARZ3NDEKTSV4RRFFQ69G5FAV", // ULID Contoh
		OutletID:      "01ARZ3NDEKTSV4RRFFQ69G5FBW",
		ShiftID:       "01ARZ3NDEKTSV4RRFFQ69G5FBX",
		CustomerID:    "01ARZ3NDEKTSV4RRFFQ69G5FCY",
		OrderType:     "dine_in",
		OrderDiscount: 500,
		Note:          "Pesanan Spesial",
		ClientCreatedAt: time.Now().UTC().Format(time.RFC3339),
		Items: []structs.CheckoutItemRequest{
			{
				ProductID:      "01ARZ3NDEKTSV4RRFFQ69G5FDZ",
				VariantID:      "01ARZ3NDEKTSV4RRFFQ69G5FEA",
				Qty:            "2.5",
				DiscountAmount: 100,
				Note:           "Tanpa cabe",
			},
		},
		Payments: []structs.CheckoutPaymentRequest{
			{
				Method:    "cash",
				Amount:    10000,
				Reference: "REF-12345",
			},
			{
				Method: "qris",
				Amount: 5000,
			},
		},
	}
	idempotencyKey := "test-idempotency-key-12345"
	requestHash := "sha256-of-the-original-request-body"

	// Panggil fungsi yang akan di-test
	input, err := BuildCheckoutInput(req, idempotencyKey, requestHash)

	// Assert hasil
	assert.NoError(t, err)
	assert.Equal(t, req.ID, input.SaleID)
	assert.Equal(t, req.OutletID, input.OutletID)
	assert.Equal(t, req.ShiftID, input.ShiftID)
	assert.Equal(t, req.CustomerID, input.CustomerID)
	assert.Equal(t, req.OrderType, input.OrderType)
	assert.Equal(t, req.OrderDiscount, input.OrderDiscount)
	assert.Equal(t, req.Note, input.Note)

	// Validasi Items
	assert.Len(t, input.Items, 1)
	assert.Equal(t, req.Items[0].ProductID, input.Items[0].ProductID)
	assert.Equal(t, req.Items[0].VariantID, input.Items[0].VariantID)
	assert.Equal(t, decimal.RequireFromString(req.Items[0].Qty), input.Items[0].Qty)
	assert.Equal(t, req.Items[0].DiscountAmount, input.Items[0].DiscountAmount)
	assert.Equal(t, req.Items[0].Note, input.Items[0].Note)

	// Validasi Payments
	assert.Len(t, input.Payments, 2)
	assert.Equal(t, req.Payments[0].Method, input.Payments[0].Method)
	assert.Equal(t, req.Payments[0].Amount, input.Payments[0].Amount)
	assert.Equal(t, req.Payments[0].Reference, input.Payments[0].Reference)
	assert.Equal(t, req.Payments[1].Method, input.Payments[1].Method)
	assert.Equal(t, req.Payments[1].Amount, input.Payments[1].Amount)

	assert.Equal(t, idempotencyKey, input.IdempotencyKey)
	assert.Equal(t, requestHash, input.RequestHash)

	// Test dengan ClientCreatedAt kosong
	req.ClientCreatedAt = ""
	input, err = BuildCheckoutInput(req, idempotencyKey, requestHash)
	assert.NoError(t, err)
	assert.Nil(t, input.ClientCreatedAt)
}