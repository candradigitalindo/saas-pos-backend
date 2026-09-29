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

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// Kemasan barang: jual & beli per "dus isi 40", stok & laporan tetap dalam
// satuan dasar barang. Harga grosir per jumlah & harga khusus pelanggan hanya
// berlaku untuk satuan dasar — kemasan punya harganya sendiri.

// SavePackagings memeriksa lalu menyamakan kemasan satu barang (di tx pemanggil).
func SavePackagings(ctx context.Context, tx *gorm.DB, p models.Product, in []structs.PackagingRequest) error {
	if len(in) > 5 {
		return fmt.Errorf("%w: kemasan paling banyak 5", helpers.ErrValidation)
	}
	rows := make([]models.ProductUnit, 0, len(in))
	sudah := map[string]bool{}
	for _, k := range in {
		if k.UnitID == p.UnitID {
			return fmt.Errorf("%w: kemasan tidak boleh memakai satuan dasar barang", helpers.ErrValidation)
		}
		if sudah[k.UnitID] {
			return fmt.Errorf("%w: satu satuan kemasan diisi dua kali", helpers.ErrValidation)
		}
		sudah[k.UnitID] = true
		var u models.Unit
		if err := repositories.FindUnitInTenant(ctx, tx, k.UnitID, &u); err != nil {
			return fmt.Errorf("%w: satuan kemasan tidak ditemukan", helpers.ErrValidation)
		}
		isi, err := decimal.NewFromString(strings.TrimSpace(k.Conversion))
		if err != nil || !isi.GreaterThan(decimal.NewFromInt(1)) {
			return fmt.Errorf("%w: isi %s harus lebih dari 1", helpers.ErrValidation, u.Name)
		}
		row := models.ProductUnit{UnitID: k.UnitID, Conversion: isi, SellPrice: k.SellPrice}
		if kode := strings.TrimSpace(k.Barcode); kode != "" {
			if err := cekKodeKemasan(ctx, tx, kode, p.ID, k.UnitID); err != nil {
				return err
			}
			row.Barcode = &kode
		}
		rows = append(rows, row)
	}
	return repositories.SaveProductUnits(ctx, tx, p.ID, rows)
}

// cekKodeKemasan: barcode kemasan tidak boleh dipakai barang, varian, atau
// kemasan lain — pemindai kasir mencari ketiganya dari kode yang sama.
func cekKodeKemasan(ctx context.Context, tx *gorm.DB, kode, productID, unitID string) error {
	pid, err := repositories.FindProductIDByCode(ctx, tx, kode)
	if errors.Is(err, repositories.ErrAmbiguousProductCode) || (err == nil && pid != "") {
		return fmt.Errorf("%w: barcode %q sudah dipakai sebuah barang", helpers.ErrConflict, kode)
	}
	if err != nil {
		return err
	}
	if dipakai, err := repositories.VariantCodeTaken(ctx, tx, kode, ""); err != nil {
		return err
	} else if dipakai {
		return fmt.Errorf("%w: barcode %q sudah dipakai varian", helpers.ErrConflict, kode)
	}
	if dipakai, err := repositories.PackagingCodeTaken(ctx, tx, kode, productID, unitID); err != nil {
		return err
	} else if dipakai {
		return fmt.Errorf("%w: barcode %q sudah dipakai kemasan lain", helpers.ErrConflict, kode)
	}
	return nil
}

// PackagingsResponse: kemasan satu barang untuk form & GET barang.
func PackagingsResponse(ctx context.Context, p models.Product) ([]structs.PackagingResponse, error) {
	m, err := repositories.ProductUnitsFor(ctx, nil, []string{p.ID})
	if err != nil {
		return nil, err
	}
	out := make([]structs.PackagingResponse, 0, len(m[p.ID]))
	for _, k := range m[p.ID] {
		r := structs.PackagingResponse{
			ID: k.ID, UnitID: k.UnitID, Conversion: k.Conversion.String(),
			SellPrice: k.SellPrice, Price: k.HargaKemasan(p.SellPrice),
		}
		if k.Unit != nil {
			r.UnitName = k.Unit.Name
		}
		if k.Barcode != nil {
			r.Barcode = *k.Barcode
		}
		out = append(out, r)
	}
	return out, nil
}

// productUnitIDs: id kemasan unik dari baris checkout.
func productUnitIDs(items []CheckoutItem) []string {
	set := map[string]bool{}
	var out []string
	for _, it := range items {
		if it.ProductUnitID != "" && !set[it.ProductUnitID] {
			set[it.ProductUnitID] = true
			out = append(out, it.ProductUnitID)
		}
	}
	return out
}
