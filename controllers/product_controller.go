package controllers

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"

	"candra/backend-api/helpers"
	"candra/backend-api/internal/reqctx"
	"candra/backend-api/models"
	"candra/backend-api/repositories"
	"candra/backend-api/services"
	"candra/backend-api/structs"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// importMaxBytes membatasi ukuran berkas impor yang dibaca ke memori.
const importMaxBytes = 8 << 20 // 8 MB

// errKodeDipakaiVarian: SKU/barcode barang sudah dipakai varian. Diperiksa
// manual karena keunikan lintas tabel tidak bisa dijaga indeks — padahal
// pemindai kasir & pesanan kanal mencari barang DAN varian dari kode yang sama.
var errKodeDipakaiVarian = errors.New("kode dipakai varian")

func cekKodeVarian(ctx context.Context, tx *gorm.DB, kode ...*string) error {
	for _, k := range kode {
		if k == nil || *k == "" {
			continue
		}
		dipakai, err := repositories.VariantCodeTaken(ctx, tx, *k, "")
		if err != nil {
			return err
		}
		if dipakai {
			return errKodeDipakaiVarian
		}
	}
	return nil
}

// validateProductRefs memastikan unit_id ada dan category_id (bila diisi) ada,
// keduanya milik tenant. Mengembalikan Unit & Category yang termuat (untuk
// response) atau (_, _, true) bila sudah membalas error.
func validateProductRefs(c *gin.Context, unitID, categoryID string) (models.Unit, *models.Category, bool) {
	ctx := c.Request.Context()

	var unit models.Unit
	if err := repositories.FindUnitInTenant(ctx, nil, unitID, &unit); err != nil {
		badRequest(c, "unit_id", "Satuan tidak ditemukan")
		return unit, nil, true
	}

	if categoryID == "" {
		return unit, nil, false
	}
	var cat models.Category
	if err := repositories.FindCategoryInTenant(ctx, nil, categoryID, &cat); err != nil {
		badRequest(c, "category_id", "Kategori tidak ditemukan")
		return unit, nil, true
	}
	return unit, &cat, false
}

// ListProducts mengembalikan produk tenant dengan filter q / category_id /
// is_active, berpaginasi. Pencarian nama memakai index trigram.
func ListProducts(c *gin.Context) {
	page, limit, offset := helpers.ParsePaginationParams(c)

	f := repositories.ProductFilter{
		Search:     c.Query("q"),
		CategoryID: c.Query("category_id"),
	}
	switch c.Query("is_active") {
	case "true", "1":
		v := true
		f.IsActive = &v
	case "false", "0":
		v := false
		f.IsActive = &v
	}

	rows, total, err := repositories.ListProducts(c.Request.Context(), f, limit, offset)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	items := make([]structs.ProductResponse, len(rows))
	for i, r := range rows {
		items[i] = productToResponse(r)
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.PaginatedResponse[structs.ProductResponse]]{
		Success: true, Message: "Berhasil mengambil data produk",
		Data: helpers.BuildPaginationResponse(c, page, limit, total, items),
	})
}

// GetProduct mengembalikan satu produk tenant (dengan nama unit & kategori).
func GetProduct(c *gin.Context) {
	var row models.Product
	if err := repositories.FindProductInTenant(c.Request.Context(), nil, c.Param("id"), &row); err != nil {
		notFoundOr(c, err, repositories.ErrProductNotFound, "Produk tidak ditemukan")
		return
	}
	res := productToResponse(row)
	tiers, err := services.WholesaleTiersResponse(c.Request.Context(), row.ID)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	res.WholesalePrices = tiers
	if res.SpecialPrices, err = services.SpecialPricesResponse(c.Request.Context(), row.ID); err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.ProductResponse]{
		Success: true, Message: "Berhasil mengambil data produk", Data: res,
	})
}

// CreateProduct menambah produk.
func CreateProduct(c *gin.Context) {
	var req structs.ProductCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}

	minStock, err := decimalOrZero(req.MinStock)
	if err != nil {
		badRequest(c, "min_stock", "Stok minimum bukan angka yang valid")
		return
	}
	unit, cat, handled := validateProductRefs(c, req.UnitID, req.CategoryID)
	if handled {
		return
	}
	if alamatUnggahanKita(req.ImageURL) {
		badRequest(c, "image_url", pesanFotoLewatUnggah)
		return
	}
	grosir, err := services.NormalWholesaleTiers(req.WholesalePrices)
	if err != nil {
		respondServiceError(c, err)
		return
	}

	ctx := c.Request.Context()
	row := models.Product{
		TenantID:    reqctx.TenantID(ctx),
		CategoryID:  nilIfEmpty(req.CategoryID),
		UnitID:      req.UnitID,
		Name:        req.Name,
		SKU:         nilIfEmpty(req.SKU),
		Barcode:     nilIfEmpty(req.Barcode),
		SellPrice:   req.SellPrice,
		CostPrice:   req.CostPrice,
		TrackStock:  boolOr(req.TrackStock, true),
		MinStock:    minStock,
		IsActive:    boolOr(req.IsActive, true),
		ImageURL:    req.ImageURL,
		Description: strings.TrimSpace(req.Description),
	}
	if err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		if err := services.EnsureQuota(ctx, tx, services.KuotaBarang, 1); err != nil {
			return err
		}
		if err := cekKodeVarian(ctx, tx, row.SKU, row.Barcode); err != nil {
			return err
		}
		if err := repositories.CreateProduct(ctx, tx, &row); err != nil {
			return err
		}
		if len(req.SpecialPrices) > 0 {
			if err := services.SaveSpecialPrices(ctx, tx, row.ID, req.SpecialPrices); err != nil {
				return err
			}
		}
		if len(grosir) == 0 {
			return nil
		}
		return services.SaveWholesaleTiers(ctx, tx, row.ID, grosir)
	}); err != nil {
		if helpers.IsDuplicateEntryError(err) || errors.Is(err, errKodeDipakaiVarian) {
			conflict(c, "sku", "SKU atau barcode sudah dipakai")
			return
		}
		respondServiceError(c, err)
		return
	}

	row.Unit = &unit
	row.Category = cat
	res := productToResponse(row)
	res.WholesalePrices = wholesaleToResponse(grosir)
	for _, s := range req.SpecialPrices {
		res.SpecialPrices = append(res.SpecialPrices, structs.SpecialPriceResponse{PriceListID: s.PriceListID, Price: s.Price})
	}
	c.JSON(http.StatusCreated, structs.SuccessResponse[structs.ProductResponse]{
		Success: true, Message: "Produk berhasil dibuat", Data: res,
	})
}

// UpdateProduct mengubah produk. Hanya field yang dikirim yang diubah.
func UpdateProduct(c *gin.Context) {
	id := c.Param("id")
	var req structs.ProductUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}

	ctx := c.Request.Context()
	var row models.Product
	if err := repositories.FindProductInTenant(ctx, nil, id, &row); err != nil {
		notFoundOr(c, err, repositories.ErrProductNotFound, "Produk tidak ditemukan")
		return
	}

	if req.Name != nil {
		row.Name = *req.Name
	}
	if req.UnitID != nil {
		var unit models.Unit
		if err := repositories.FindUnitInTenant(ctx, nil, *req.UnitID, &unit); err != nil {
			badRequest(c, "unit_id", "Satuan tidak ditemukan")
			return
		}
		row.UnitID = unit.ID
		row.Unit = &unit
	}
	if req.CategoryID != nil {
		if *req.CategoryID == "" {
			row.CategoryID = nil
			row.Category = nil
		} else {
			var cat models.Category
			if err := repositories.FindCategoryInTenant(ctx, nil, *req.CategoryID, &cat); err != nil {
				badRequest(c, "category_id", "Kategori tidak ditemukan")
				return
			}
			row.CategoryID = &cat.ID
			row.Category = &cat
		}
	}
	if req.SKU != nil {
		row.SKU = nilIfEmpty(*req.SKU)
	}
	if req.Barcode != nil {
		row.Barcode = nilIfEmpty(*req.Barcode)
	}
	if req.SellPrice != nil {
		row.SellPrice = *req.SellPrice
	}
	if req.CostPrice != nil {
		row.CostPrice = *req.CostPrice
	}
	if req.TrackStock != nil {
		row.TrackStock = *req.TrackStock
	}
	if req.IsActive != nil {
		row.IsActive = *req.IsActive
	}
	if req.ImageURL != nil && *req.ImageURL != row.ImageURL {
		if alamatUnggahanKita(*req.ImageURL) {
			badRequest(c, "image_url", pesanFotoLewatUnggah)
			return
		}
		row.ImageURL = *req.ImageURL
	}
	if req.MinStock != nil {
		d, err := decimalOrZero(*req.MinStock)
		if err != nil {
			badRequest(c, "min_stock", "Stok minimum bukan angka yang valid")
			return
		}
		row.MinStock = d
	}
	if req.Description != nil {
		row.Description = strings.TrimSpace(*req.Description)
	}
	var grosir []models.ProductPrice
	if req.WholesalePrices != nil {
		var err error
		if grosir, err = services.NormalWholesaleTiers(*req.WholesalePrices); err != nil {
			respondServiceError(c, err)
			return
		}
	}

	if err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		var kode []*string
		if req.SKU != nil {
			kode = append(kode, row.SKU)
		}
		if req.Barcode != nil {
			kode = append(kode, row.Barcode)
		}
		if err := cekKodeVarian(ctx, tx, kode...); err != nil {
			return err
		}
		if err := repositories.UpdateProduct(ctx, tx, &row); err != nil {
			return err
		}
		if req.SpecialPrices != nil {
			if err := services.SaveSpecialPrices(ctx, tx, row.ID, *req.SpecialPrices); err != nil {
				return err
			}
		}
		if req.WholesalePrices == nil {
			return nil
		}
		return services.SaveWholesaleTiers(ctx, tx, row.ID, grosir)
	}); err != nil {
		if helpers.IsDuplicateEntryError(err) || errors.Is(err, errKodeDipakaiVarian) {
			conflict(c, "sku", "SKU atau barcode sudah dipakai")
			return
		}
		notFoundOr(c, err, repositories.ErrProductNotFound, "Produk tidak ditemukan")
		return
	}
	res := productToResponse(row)
	tiers, err := services.WholesaleTiersResponse(ctx, row.ID)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	res.WholesalePrices = tiers
	if res.SpecialPrices, err = services.SpecialPricesResponse(ctx, row.ID); err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.ProductResponse]{
		Success: true, Message: "Produk berhasil diperbarui", Data: res,
	})
}

func wholesaleToResponse(tiers []models.ProductPrice) []structs.WholesalePriceResponse {
	out := make([]structs.WholesalePriceResponse, 0, len(tiers))
	for _, t := range tiers {
		out = append(out, structs.WholesalePriceResponse{MinQty: t.MinQty.String(), Price: t.Price})
	}
	return out
}

// DeleteProduct menghapus (soft delete) produk.
func DeleteProduct(c *gin.Context) {
	ctx := c.Request.Context()
	if err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		return repositories.DeleteProduct(ctx, tx, c.Param("id"))
	}); err != nil {
		notFoundOr(c, err, repositories.ErrProductNotFound, "Produk tidak ditemukan")
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[any]{Success: true, Message: "Produk berhasil dihapus", Data: nil})
}

// ImportProducts mengimpor produk dari CSV. Menerima berkas lewat
// multipart/form-data (field "file") ATAU body mentah (text/csv). Query
// ?dry_run=true hanya memvalidasi tanpa menyimpan.
func ImportProducts(c *gin.Context) {
	dryRun := c.Query("dry_run") == "true" || c.Query("dry_run") == "1"

	raw, ok := readImportBody(c)
	if !ok {
		return
	}

	result, err := services.ImportProductsCSV(c.Request.Context(), raw, dryRun)
	if err != nil {
		respondServiceError(c, err)
		return
	}

	msg := "Impor selesai"
	if dryRun {
		msg = "Pratinjau impor selesai"
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.ProductImportResult]{
		Success: true, Message: msg, Data: *result,
	})
}

// readImportBody mengambil isi CSV dari request. Mengembalikan (nil, false) dan
// sudah membalas error bila gagal.
func readImportBody(c *gin.Context) ([]byte, bool) {
	if strings.HasPrefix(c.ContentType(), "multipart/form-data") {
		fh, err := c.FormFile("file")
		if err != nil {
			badRequest(c, "file", "Sertakan berkas CSV pada field 'file'")
			return nil, false
		}
		if fh.Size > importMaxBytes {
			badRequest(c, "file", "Berkas terlalu besar")
			return nil, false
		}
		f, err := fh.Open()
		if err != nil {
			internalError(c)
			return nil, false
		}
		defer f.Close()
		b, err := io.ReadAll(io.LimitReader(f, importMaxBytes))
		if err != nil {
			internalError(c)
			return nil, false
		}
		return b, true
	}

	b, err := io.ReadAll(io.LimitReader(c.Request.Body, importMaxBytes))
	if err != nil || len(b) == 0 {
		badRequest(c, "body", "Body CSV kosong atau tidak terbaca")
		return nil, false
	}
	return b, true
}

// decimalOrZero mengurai string desimal; kosong → 0.
func decimalOrZero(s string) (decimal.Decimal, error) {
	if s == "" {
		return decimal.Zero, nil
	}
	return decimal.NewFromString(s)
}

// UploadProductImage: POST /api/v1/products/:id/image
//
// Menerima satu foto (multipart, field "file") dan menyimpannya di server.
// Jenis berkas ditentukan dari ISINYA, bukan dari nama atau Content-Type yang
// dikirim klien — lihat services.SimpanFotoBarang.
func UploadProductImage(c *gin.Context) {
	berkas, err := c.FormFile("file")
	if err != nil {
		badRequest(c, "file", "Foto belum dipilih.")
		return
	}

	f, err := berkas.Open()
	if err != nil {
		respondServiceError(c, err)
		return
	}
	defer f.Close()

	alamat, err := services.SimpanFotoBarang(c.Request.Context(), c.Param("id"), f)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[gin.H]{
		Success: true, Message: "Foto barang tersimpan",
		Data: gin.H{"image_url": alamat},
	})
}

// DeleteProductImage: DELETE /api/v1/products/:id/image
func DeleteProductImage(c *gin.Context) {
	if err := services.HapusFotoBarang(c.Request.Context(), c.Param("id")); err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[gin.H]{
		Success: true, Message: "Foto barang dihapus", Data: gin.H{"image_url": ""},
	})
}

// pesanFotoLewatUnggah menjelaskan kenapa alamat berkas unggahan ditolak di
// form barang.
const pesanFotoLewatUnggah = "Foto hasil unggahan hanya bisa dipasang lewat tombol unggah foto"

// alamatUnggahanKita melaporkan apakah sebuah image_url menunjuk berkas di
// folder unggahan server ini.
//
// Alamat seperti itu HANYA boleh ditulis oleh endpoint unggah foto, yang
// membuat nama berkasnya sendiri. Bila form barang boleh mengisinya bebas,
// satu tenant bisa mengarahkan fotonya ke berkas milik tenant lain lalu
// menekan "hapus foto" — dan hapusFotoLama menghapus berkas tenant lain itu
// dari disk. URL luar (CDN, dsb.) tetap diterima seperti sebelumnya.
func alamatUnggahanKita(alamat string) bool {
	return strings.HasPrefix(strings.TrimSpace(alamat), "/uploads/")
}
