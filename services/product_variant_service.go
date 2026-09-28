package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"candra/backend-api/helpers"
	"candra/backend-api/models"
	"candra/backend-api/repositories"
	"candra/backend-api/structs"

	"gorm.io/gorm"
)

// Varian barang = PILIHAN dengan selisih harga (ukuran, es/panas, level
// pedas). Stok tetap dihitung di tingkat BARANG: layar stok (barang masuk,
// penyesuaian, opname, transfer) belum mengenal varian, jadi penjualan
// bervarian memotong stok barangnya. Nama varian ikut tercetak di struk
// ("Kopi Susu (Besar)") lewat snapshot nama di isi penjualan.

func variantToResponse(v models.ProductVariant, hargaBarang int64) structs.ProductVariantResponse {
	r := structs.ProductVariantResponse{
		ID: v.ID, ProductID: v.ProductID, Name: v.Name, PriceDelta: v.PriceDelta,
		Price: hargaBarang + v.PriceDelta, IsActive: v.IsActive,
	}
	if v.SKU != nil {
		r.SKU = *v.SKU
	}
	if v.Barcode != nil {
		r.Barcode = *v.Barcode
	}
	return r
}

func ListProductVariants(ctx context.Context, productID string) ([]structs.ProductVariantResponse, error) {
	var p models.Product
	if err := repositories.FindProductInTenant(ctx, nil, productID, &p); err != nil {
		return nil, err
	}
	vs, err := repositories.ListProductVariants(ctx, nil, productID)
	if err != nil {
		return nil, err
	}
	out := make([]structs.ProductVariantResponse, 0, len(vs))
	for _, v := range vs {
		out = append(out, variantToResponse(v, p.SellPrice))
	}
	return out, nil
}

func kosongJadiNil(s string) *string {
	if s = strings.TrimSpace(s); s == "" {
		return nil
	}
	return &s
}

// SaveProductVariant membuat (variantID kosong) atau mengganti satu varian.
func SaveProductVariant(ctx context.Context, productID, variantID string, in structs.ProductVariantRequest) (structs.ProductVariantResponse, error) {
	var out structs.ProductVariantResponse
	err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		var p models.Product
		if err := repositories.FindProductInTenant(ctx, tx, productID, &p); err != nil {
			return err
		}
		nama := strings.TrimSpace(in.Name)
		if nama == "" {
			return fmt.Errorf("%w: nama varian wajib diisi", helpers.ErrValidation)
		}
		if p.SellPrice+in.PriceDelta < 0 {
			return fmt.Errorf("%w: harga varian jadi minus (harga jual Rp %d, selisih %d)", helpers.ErrValidation, p.SellPrice, in.PriceDelta)
		}
		ada, err := repositories.ListProductVariants(ctx, tx, productID)
		if err != nil {
			return err
		}
		for _, v := range ada {
			if v.ID != variantID && strings.EqualFold(strings.TrimSpace(v.Name), nama) {
				return fmt.Errorf("%w: varian %q sudah ada di barang ini", helpers.ErrConflict, nama)
			}
		}
		for _, kode := range []string{strings.TrimSpace(in.SKU), strings.TrimSpace(in.Barcode)} {
			if kode == "" {
				continue
			}
			pid, err := repositories.FindProductIDByCode(ctx, tx, kode)
			if errors.Is(err, repositories.ErrAmbiguousProductCode) || (err == nil && pid != "") {
				return fmt.Errorf("%w: kode %q sudah dipakai sebuah barang", helpers.ErrConflict, kode)
			}
			if err != nil && !errors.Is(err, repositories.ErrAmbiguousProductCode) {
				return err
			}
			if dipakai, err := repositories.VariantCodeTaken(ctx, tx, kode, variantID); err != nil {
				return err
			} else if dipakai {
				return fmt.Errorf("%w: kode %q sudah dipakai varian lain", helpers.ErrConflict, kode)
			}
		}
		v := models.ProductVariant{
			ProductID: productID, Name: nama, PriceDelta: in.PriceDelta,
			SKU: kosongJadiNil(in.SKU), Barcode: kosongJadiNil(in.Barcode), IsActive: in.IsActive == nil || *in.IsActive,
		}
		if variantID == "" {
			if err := repositories.CreateProductVariant(ctx, tx, &v); err != nil {
				return err
			}
		} else {
			lama, err := repositories.FindProductVariant(ctx, tx, productID, variantID)
			if err != nil {
				return err
			}
			v.ID, v.TenantID, v.CreatedAt = lama.ID, lama.TenantID, lama.CreatedAt
			if err := repositories.UpdateProductVariant(ctx, tx, &v); err != nil {
				return err
			}
		}
		out = variantToResponse(v, p.SellPrice)
		return nil
	})
	return out, err
}

func DeleteProductVariant(ctx context.Context, productID, variantID string) error {
	return repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		return repositories.DeleteProductVariant(ctx, tx, productID, variantID)
	})
}
