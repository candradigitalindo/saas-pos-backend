package controllers

import (
	"net/http"

	"candra/backend-api/models"
	"candra/backend-api/repositories"
	"candra/backend-api/services"
	"candra/backend-api/structs"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

// GetProductRecipe mengembalikan resep sebuah produk (menu jadi).
func GetProductRecipe(c *gin.Context) {
	productID := c.Param("id")
	var rw repositories.RecipeWithItems
	if err := repositories.FindRecipeByProduct(c.Request.Context(), nil, productID, &rw); err != nil {
		notFoundOr(c, err, repositories.ErrRecipeNotFound, "Produk ini belum punya resep")
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.RecipeResponse]{
		Success: true, Message: "Detail resep", Data: services.RecipeToResponse(rw),
	})
}

// UpsertProductRecipe mengganti seluruh resep sebuah produk. items kosong =
// hapus resep.
func UpsertProductRecipe(c *gin.Context) {
	productID := c.Param("id")
	var req structs.RecipeUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}

	ctx := c.Request.Context()
	// Produk harus ada di tenant.
	var prod models.Product
	if err := repositories.FindProductInTenant(ctx, nil, productID, &prod); err != nil {
		notFoundOr(c, err, repositories.ErrProductNotFound, "Produk tidak ditemukan")
		return
	}

	yield := decimal.NewFromInt(1)
	if req.YieldQty != "" {
		y, e := decimal.NewFromString(req.YieldQty)
		if e != nil || y.LessThanOrEqual(decimal.Zero) {
			badRequest(c, "yield_qty", "yield_qty harus angka lebih besar dari 0")
			return
		}
		yield = y
	}

	items := make([]models.RecipeItem, 0, len(req.Items))
	for _, it := range req.Items {
		if it.IngredientProductID == productID {
			badRequest(c, "items", "Bahan baku tidak boleh produk itu sendiri")
			return
		}
		q, e := decimal.NewFromString(it.Qty)
		if e != nil || q.LessThanOrEqual(decimal.Zero) {
			badRequest(c, "items.qty", "qty bahan harus angka lebih besar dari 0")
			return
		}
		// Bahan harus produk milik tenant.
		var ing models.Product
		if err := repositories.FindProductInTenant(ctx, nil, it.IngredientProductID, &ing); err != nil {
			badRequest(c, "items.ingredient_product_id", "Bahan baku tidak ditemukan")
			return
		}
		items = append(items, models.RecipeItem{IngredientProductID: it.IngredientProductID, Qty: q})
	}

	rw, err := repositories.ReplaceRecipe(ctx, productID, models.Recipe{YieldQty: yield}, items)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	if len(items) == 0 {
		c.JSON(http.StatusOK, structs.SuccessResponse[any]{Success: true, Message: "Resep dihapus", Data: nil})
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.RecipeResponse]{
		Success: true, Message: "Resep disimpan", Data: services.RecipeToResponse(rw),
	})
}
