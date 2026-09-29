package services

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"candra/backend-api/helpers"
	"candra/backend-api/internal/reqctx"
	"candra/backend-api/models"
	"candra/backend-api/repositories"
	"candra/backend-api/structs"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// importMaxRows membatasi jumlah baris data agar satu unggahan tidak bisa
// memuat berkas raksasa ke memori.
const importMaxRows = 20000

// requiredImportColumns adalah kolom header yang wajib ada.
var requiredImportColumns = []string{"name", "unit"}

// ImportProductsCSV memvalidasi dan (bila bukan dry-run) menyisipkan produk dari
// CSV. Bersifat "impor sebagian": baris yang lolos validasi tetap dimasukkan,
// baris yang gagal dilaporkan per baris — tidak membatalkan yang lain.
//
// Validasi di sini bersifat OTORITATIF: satuan & kategori diresolusi dari nama,
// SKU/barcode dicek duplikat terhadap data yang ada DAN sesama baris berkas.
// Insert massal setelahnya seharusnya tidak lagi gagal karena data (kecuali
// balapan tulis yang sangat jarang → dilaporkan sebagai galat internal).
func ImportProductsCSV(ctx context.Context, raw []byte, dryRun bool) (*structs.ProductImportResult, error) {
	if reqctx.TenantID(ctx) == "" {
		return nil, repositories.ErrNoTenantInContext
	}

	r := csv.NewReader(strings.NewReader(string(raw)))
	r.FieldsPerRecord = -1 // toleransi jumlah kolom tak konsisten; divalidasi manual
	r.TrimLeadingSpace = true

	header, err := r.Read()
	if err == io.EOF {
		return nil, fmt.Errorf("%w: berkas CSV kosong", helpers.ErrValidation)
	}
	if err != nil {
		return nil, fmt.Errorf("%w: gagal membaca header CSV", helpers.ErrValidation)
	}

	col := map[string]int{}
	for i, h := range header {
		col[strings.ToLower(strings.TrimSpace(h))] = i
	}
	for _, need := range requiredImportColumns {
		if _, ok := col[need]; !ok {
			return nil, fmt.Errorf("%w: kolom wajib '%s' tidak ada di header", helpers.ErrValidation, need)
		}
	}

	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("%w: format CSV tidak valid", helpers.ErrValidation)
	}
	if len(records) > importMaxRows {
		return nil, fmt.Errorf("%w: maksimal %d baris per unggahan", helpers.ErrValidation, importMaxRows)
	}

	unitByName, err := repositories.UnitLookup(ctx)
	if err != nil {
		return nil, err
	}
	catByName, err := repositories.CategoryLookup(ctx)
	if err != nil {
		return nil, err
	}

	// Kumpulkan SKU & barcode dari berkas untuk cek duplikat terhadap data
	// yang sudah ada (satu query).
	fileSKUs, fileBarcodes := collectCodes(records, col)
	existSKU, existBarcode, err := repositories.ExistingProductCodes(ctx, fileSKUs, fileBarcodes)
	if err != nil {
		return nil, err
	}

	tenantID := reqctx.TenantID(ctx)
	seenSKU := map[string]bool{}
	seenBarcode := map[string]bool{}

	result := &structs.ProductImportResult{DryRun: dryRun, Total: len(records)}
	valid := make([]models.Product, 0, len(records))

	for i, rec := range records {
		rowNo := i + 1 // baris data ke-, 1-indexed (header tidak dihitung)
		field := func(name string) string {
			idx, ok := col[name]
			if !ok || idx >= len(rec) {
				return ""
			}
			return strings.TrimSpace(rec[idx])
		}
		fail := func(f, msg string) {
			result.Errors = append(result.Errors, structs.ProductImportRowError{Row: rowNo, Field: f, Message: msg})
		}

		name := field("name")
		if name == "" {
			fail("name", "nama produk wajib diisi")
			continue
		}

		unitName := field("unit")
		unitID, ok := unitByName[unitName]
		if !ok {
			fail("unit", fmt.Sprintf("satuan %q tidak ditemukan", unitName))
			continue
		}

		var categoryID *string
		if cn := field("category"); cn != "" {
			cid, ok := catByName[cn]
			if !ok {
				fail("category", fmt.Sprintf("kategori %q tidak ditemukan", cn))
				continue
			}
			categoryID = &cid
		}

		sku := field("sku")
		if sku != "" {
			if existSKU[sku] || seenSKU[sku] {
				fail("sku", fmt.Sprintf("SKU %q sudah dipakai", sku))
				continue
			}
			seenSKU[sku] = true
		}
		barcode := field("barcode")
		if barcode != "" {
			if existBarcode[barcode] || seenBarcode[barcode] {
				fail("barcode", fmt.Sprintf("barcode %q sudah dipakai", barcode))
				continue
			}
			seenBarcode[barcode] = true
		}

		sellPrice, err := parseMoney(field("sell_price"))
		if err != nil {
			fail("sell_price", "harga jual harus bilangan bulat ≥ 0")
			continue
		}
		costPrice, err := parseMoney(field("cost_price"))
		if err != nil {
			fail("cost_price", "harga modal harus bilangan bulat ≥ 0")
			continue
		}
		minStock, err := parseDecimalOrZero(field("min_stock"))
		if err != nil {
			fail("min_stock", "stok minimum bukan angka yang valid")
			continue
		}

		valid = append(valid, models.Product{
			TenantID:   tenantID,
			CategoryID: categoryID,
			UnitID:     unitID,
			Name:       name,
			SKU:        nilIfEmpty(sku),
			Barcode:    nilIfEmpty(barcode),
			SellPrice:  sellPrice,
			CostPrice:  costPrice,
			TrackStock: parseBoolDefault(field("track_stock"), true),
			MinStock:   minStock,
			IsActive:   parseBoolDefault(field("is_active"), true),
		})
	}

	result.Failed = len(result.Errors)
	result.Imported = len(valid)

	if !dryRun && len(valid) > 0 {
		kuota := func(tx *gorm.DB) error { return EnsureQuota(ctx, tx, KuotaBarang, len(valid)) }
		if err := repositories.CreateProductsBulk(ctx, valid, kuota); err != nil {
			if errors.Is(err, helpers.ErrPlanRequired) {
				return nil, err
			}
			return nil, fmt.Errorf("gagal menyimpan produk terimpor: %w", err)
		}
	}
	if result.Errors == nil {
		result.Errors = []structs.ProductImportRowError{}
	}
	return result, nil
}

// collectCodes mengumpulkan seluruh nilai kolom sku & barcode non-kosong dari
// records untuk keperluan cek duplikat sekali jalan.
func collectCodes(records [][]string, col map[string]int) (skus, barcodes []string) {
	get := func(rec []string, name string) string {
		idx, ok := col[name]
		if !ok || idx >= len(rec) {
			return ""
		}
		return strings.TrimSpace(rec[idx])
	}
	for _, rec := range records {
		if s := get(rec, "sku"); s != "" {
			skus = append(skus, s)
		}
		if b := get(rec, "barcode"); b != "" {
			barcodes = append(barcodes, b)
		}
	}
	return skus, barcodes
}

// parseMoney mengurai rupiah bulat. Kosong → 0. Menolak desimal & negatif.
func parseMoney(s string) (int64, error) {
	if s == "" {
		return 0, nil
	}
	s = strings.ReplaceAll(s, ".", "") // toleransi pemisah ribuan "1.500"
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("nilai uang tidak valid: %q", s)
	}
	return n, nil
}

// parseDecimalOrZero mengurai kuantitas desimal. Kosong → 0. Menerima koma
// sebagai pemisah desimal ("0,25").
func parseDecimalOrZero(s string) (decimal.Decimal, error) {
	if s == "" {
		return decimal.Zero, nil
	}
	return decimal.NewFromString(strings.ReplaceAll(s, ",", "."))
}

// parseBoolDefault mengurai boolean longgar (id/en). Kosong / tak dikenal → def.
func parseBoolDefault(s string, def bool) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "ya", "y", "yes", "aktif":
		return true
	case "0", "false", "tidak", "n", "no", "nonaktif":
		return false
	default:
		return def
	}
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
