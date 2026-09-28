package services

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"

	"candra/backend-api/helpers"
	"candra/backend-api/models"
	"candra/backend-api/repositories"
	"candra/backend-api/structs"

	"gorm.io/gorm"
)

// Struk digital: tautan publik ke satu struk, untuk dibagikan lewat WhatsApp
// dari perangkat kasir (pengirimnya nomor toko sendiri).

// ReceiptLinkToken mengembalikan token tautan struk sebuah penjualan,
// membuatnya bila belum ada. Penjualan cabang lain diperlakukan tidak ada.
func ReceiptLinkToken(ctx context.Context, saleID string) (string, error) {
	var sale models.Sale
	if err := repositories.FindSaleInTenant(ctx, nil, saleID, &sale); err != nil {
		if errors.Is(err, repositories.ErrSaleNotFound) {
			return "", fmt.Errorf("%w: transaksi tidak ditemukan", helpers.ErrNotFound)
		}
		return "", err
	}
	if !repositories.OutletVisible(ctx, sale.OutletID) {
		return "", fmt.Errorf("%w: transaksi tidak ditemukan", helpers.ErrNotFound)
	}
	if sale.ReceiptToken != nil && *sale.ReceiptToken != "" {
		return *sale.ReceiptToken, nil
	}
	token, err := tokenStruk()
	if err != nil {
		return "", err
	}
	var hasil string
	err = repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		n, err := repositories.SetSaleReceiptToken(ctx, tx, saleID, token)
		if err != nil {
			return err
		}
		if n == 1 {
			hasil = token
			return nil
		}
		// Kalah balapan: kasir lain baru saja membagikan struk yang sama.
		var s models.Sale
		if err := repositories.FindSaleInTenant(ctx, tx, saleID, &s); err != nil {
			return err
		}
		if s.ReceiptToken != nil {
			hasil = *s.ReceiptToken
		}
		return nil
	})
	return hasil, err
}

// tokenStruk: 128 bit acak, base64url tanpa padding (22 karakter).
func tokenStruk() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// PublicReceipt memuat struk untuk halaman publik. Yang dibuka hanya isi
// struk — tanpa modal, pelanggan, kasir, atau id internal.
func PublicReceipt(ctx context.Context, token string) (structs.PublicReceiptResponse, error) {
	var out structs.PublicReceiptResponse
	if len(token) != 22 {
		return out, fmt.Errorf("%w: struk tidak ditemukan", helpers.ErrNotFound)
	}
	s, o, err := repositories.PublicReceiptByToken(ctx, token)
	if errors.Is(err, repositories.ErrSaleNotFound) {
		return out, fmt.Errorf("%w: struk tidak ditemukan", helpers.ErrNotFound)
	}
	if err != nil {
		return out, err
	}
	out = structs.PublicReceiptResponse{
		StoreName: o.Name, Address: o.Address, Phone: o.Phone,
		Header: o.ReceiptHeader, Footer: o.ReceiptFooter, Timezone: o.Timezone,
		ReceiptNo: s.ReceiptNo, Status: s.Status, OccurredAt: s.OccurredAt.UTC().Format(saleTimeLayout),
		Subtotal: s.Subtotal, Discount: s.DiscountAmount, Tax: s.TaxAmount, Service: s.ServiceAmount,
		Rounding: s.RoundingAmount, Total: s.Total, Paid: s.PaidAmount, Change: s.ChangeAmount,
		Items: []structs.PublicReceiptItem{}, Payments: []structs.PublicReceiptPayment{},
	}
	for _, it := range s.Items {
		out.Items = append(out.Items, structs.PublicReceiptItem{
			Name: it.ProductName, Qty: it.Qty.String(), Unit: it.UnitName, UnitPrice: it.UnitPrice,
			Discount: it.DiscountAmount, LineTotal: it.LineTotal, Note: it.Note,
		})
	}
	for _, p := range s.Payments {
		out.Payments = append(out.Payments, structs.PublicReceiptPayment{Method: p.Method, Amount: p.Amount})
	}
	return out, nil
}
