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

// Harga khusus pelanggan: pemilik membuat daftar harga ("Member",
// "Reseller"), mengisi harga khusus per barang, lalu mengaitkan pelanggan ke
// satu daftar. Di kasir, harga khusus berlaku bila transaksinya atas nama
// pelanggan itu DAN barangnya punya harga di daftarnya; selain itu harga umum
// (termasuk harga grosir per jumlah).

// ListPriceLists: daftar harga KHUSUS (yang default tidak ditampilkan —
// isinya harga grosir per jumlah, diatur dari form barang).
func ListPriceLists(ctx context.Context) ([]structs.PriceListResponse, error) {
	rows, err := repositories.ListPriceLists(ctx)
	if err != nil {
		return nil, err
	}
	out := []structs.PriceListResponse{}
	for _, r := range rows {
		if !r.IsDefault {
			out = append(out, structs.PriceListResponse{ID: r.ID, Name: r.Name, Kind: r.Kind})
		}
	}
	return out, nil
}

// CreatePriceList membuat daftar harga khusus. Nama unik (tanpa beda huruf).
func CreatePriceList(ctx context.Context, name string) (structs.PriceListResponse, error) {
	nama := strings.TrimSpace(name)
	if nama == "" {
		return structs.PriceListResponse{}, fmt.Errorf("%w: nama daftar harga wajib diisi", helpers.ErrValidation)
	}
	ada, err := repositories.ListPriceLists(ctx)
	if err != nil {
		return structs.PriceListResponse{}, err
	}
	for _, a := range ada {
		if strings.EqualFold(a.Name, nama) {
			return structs.PriceListResponse{}, fmt.Errorf("%w: daftar harga %q sudah ada", helpers.ErrConflict, nama)
		}
	}
	pl := models.PriceList{Name: nama, Kind: "member"}
	err = repositories.WithTenant(ctx, func(tx *gorm.DB) error { return repositories.CreatePriceList(ctx, tx, &pl) })
	return structs.PriceListResponse{ID: pl.ID, Name: pl.Name, Kind: pl.Kind}, err
}

func DeletePriceList(ctx context.Context, id string) error {
	err := repositories.WithTenant(ctx, func(tx *gorm.DB) error { return repositories.DeletePriceList(ctx, tx, id) })
	if errors.Is(err, repositories.ErrPriceListNotFound) {
		return fmt.Errorf("%w: daftar harga tidak ditemukan", helpers.ErrNotFound)
	}
	return err
}

// NormalSpecialPrices memeriksa harga khusus dari form barang: daftar harga
// harus ada & khusus, tiap daftar paling banyak satu harga.
func NormalSpecialPrices(ctx context.Context, tx *gorm.DB, in []structs.SpecialPriceRequest) ([]models.ProductPrice, error) {
	out := make([]models.ProductPrice, 0, len(in))
	sudah := map[string]bool{}
	for _, s := range in {
		if sudah[s.PriceListID] {
			return nil, fmt.Errorf("%w: satu daftar harga diisi dua kali", helpers.ErrValidation)
		}
		sudah[s.PriceListID] = true
		if _, err := repositories.FindSpecialPriceList(ctx, tx, s.PriceListID); err != nil {
			if errors.Is(err, repositories.ErrPriceListNotFound) {
				return nil, fmt.Errorf("%w: daftar harga tidak ditemukan", helpers.ErrValidation)
			}
			return nil, err
		}
		if s.Price <= 0 {
			return nil, fmt.Errorf("%w: harga khusus harus lebih dari 0", helpers.ErrValidation)
		}
		out = append(out, models.ProductPrice{PriceListID: s.PriceListID, MinQty: decimal.NewFromInt(1), Price: s.Price})
	}
	return out, nil
}

// SaveSpecialPrices memeriksa lalu mengganti harga khusus satu barang (di tx pemanggil).
func SaveSpecialPrices(ctx context.Context, tx *gorm.DB, productID string, in []structs.SpecialPriceRequest) error {
	rows, err := NormalSpecialPrices(ctx, tx, in)
	if err != nil {
		return err
	}
	return repositories.ReplaceSpecialPrices(ctx, tx, productID, rows)
}

func SpecialPricesResponse(ctx context.Context, productID string) ([]structs.SpecialPriceResponse, error) {
	rows, err := repositories.SpecialPrices(ctx, nil, productID)
	if err != nil {
		return nil, err
	}
	out := make([]structs.SpecialPriceResponse, 0, len(rows))
	for _, r := range rows {
		out = append(out, structs.SpecialPriceResponse{PriceListID: r.PriceListID, Price: r.Price})
	}
	return out, nil
}

// ValidateCustomerPriceList memastikan daftar harga pelanggan ada & khusus.
func ValidateCustomerPriceList(ctx context.Context, id string) error {
	if id == "" {
		return nil
	}
	if _, err := repositories.FindSpecialPriceList(ctx, nil, id); err != nil {
		if errors.Is(err, repositories.ErrPriceListNotFound) {
			return fmt.Errorf("%w: daftar harga tidak ditemukan", helpers.ErrValidation)
		}
		return err
	}
	return nil
}

// customerListTiers: tingkat harga daftar khusus milik pelanggan transaksi
// (nil bila tanpa pelanggan / pelanggan tanpa daftar). Pelanggan yang tidak
// dikenal tidak ditolak di sini — resolvePayments yang menolaknya dengan
// pesan yang jelas.
func customerListTiers(ctx context.Context, tx *gorm.DB, customerID string, productIDs []string) (map[string][]models.ProductPrice, error) {
	if customerID == "" {
		return nil, nil
	}
	var c models.Customer
	if err := repositories.FindCustomerInTenant(ctx, tx, customerID, &c); err != nil {
		if errors.Is(err, repositories.ErrCustomerNotFound) {
			return nil, nil
		}
		return nil, err
	}
	if c.PriceListID == nil {
		return nil, nil
	}
	return repositories.ListTiers(ctx, tx, *c.PriceListID, productIDs)
}
