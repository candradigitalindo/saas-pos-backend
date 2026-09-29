package services

import (
	"context"
	"fmt"
	"sort"

	"candra/backend-api/helpers"
	"candra/backend-api/models"
	"candra/backend-api/repositories"
	"candra/backend-api/structs"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// Harga grosir per jumlah: beli minimal N (TOTAL barang itu dalam satu
// transaksi, semua varian dijumlah), harga satuannya turun. Berlaku untuk
// semua pembeli — harga khusus per pelanggan (member) adalah fitur terpisah.
// Varian tetap menambah selisihnya di atas harga grosir.

// NormalWholesaleTiers memeriksa & mengurutkan tingkat grosir dari form.
func NormalWholesaleTiers(in []structs.WholesalePriceRequest) ([]models.ProductPrice, error) {
	if len(in) > 5 {
		return nil, fmt.Errorf("%w: harga grosir paling banyak 5 tingkat", helpers.ErrValidation)
	}
	out := make([]models.ProductPrice, 0, len(in))
	sudah := map[string]bool{}
	for _, t := range in {
		q, err := decimal.NewFromString(t.MinQty)
		if err != nil || !q.GreaterThan(decimal.NewFromInt(1)) {
			return nil, fmt.Errorf("%w: jumlah minimal harga grosir harus lebih dari 1 (tertulis %q)", helpers.ErrValidation, t.MinQty)
		}
		if t.Price <= 0 {
			return nil, fmt.Errorf("%w: harga grosir harus lebih dari 0", helpers.ErrValidation)
		}
		if sudah[q.String()] {
			return nil, fmt.Errorf("%w: jumlah minimal %s tertulis dua kali", helpers.ErrValidation, q.String())
		}
		sudah[q.String()] = true
		out = append(out, models.ProductPrice{MinQty: q, Price: t.Price})
	}
	sort.Slice(out, func(a, b int) bool { return out[a].MinQty.LessThan(out[b].MinQty) })
	return out, nil
}

// SaveWholesaleTiers mengganti tingkat grosir satu barang (di dalam tx pemanggil).
func SaveWholesaleTiers(ctx context.Context, tx *gorm.DB, productID string, tiers []models.ProductPrice) error {
	pl, err := repositories.EnsureDefaultPriceList(ctx, tx)
	if err != nil {
		return err
	}
	return repositories.ReplaceWholesaleTiers(ctx, tx, productID, pl.ID, tiers)
}

// WholesaleTiersResponse: tingkat grosir satu barang, jumlah terkecil dulu.
func WholesaleTiersResponse(ctx context.Context, productID string) ([]structs.WholesalePriceResponse, error) {
	m, err := repositories.WholesaleTiers(ctx, nil, []string{productID})
	if err != nil {
		return nil, err
	}
	rows := m[productID]
	out := make([]structs.WholesalePriceResponse, 0, len(rows))
	for i := len(rows) - 1; i >= 0; i-- {
		out = append(out, structs.WholesalePriceResponse{MinQty: rows[i].MinQty.String(), Price: rows[i].Price})
	}
	return out, nil
}

// hargaDaftar: harga dari tingkat daftar harga khusus yang terpenuhi, bila ada.
func hargaDaftar(tiers []models.ProductPrice, total decimal.Decimal) (int64, bool) {
	for _, t := range tiers {
		if total.GreaterThanOrEqual(t.MinQty) {
			return t.Price, true
		}
	}
	return 0, false
}

// hargaDasarGrosir: harga satuan dasar barang untuk total jumlah `total` —
// tingkat ber-min_qty terbesar yang terpenuhi, selain itu harga jual.
// `tiers` urut min_qty menurun (repositories.WholesaleTiers).
func hargaDasarGrosir(hargaJual int64, tiers []models.ProductPrice, total decimal.Decimal) int64 {
	for _, t := range tiers {
		if total.GreaterThanOrEqual(t.MinQty) {
			return t.Price
		}
	}
	return hargaJual
}
