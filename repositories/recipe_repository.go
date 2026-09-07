package repositories

import (
	"context"
	"errors"

	"candra/backend-api/models"

	"gorm.io/gorm"
)

var ErrRecipeNotFound = errors.New("resep tidak ditemukan")

// RecipeWithItems adalah resep beserta bahan bakunya.
type RecipeWithItems struct {
	Recipe models.Recipe
	Items  []models.RecipeItem
}

// FindRecipeByProduct memuat resep sebuah produk (menu jadi) beserta bahannya.
// tx opsional. Mengembalikan ErrRecipeNotFound bila produk belum punya resep.
func FindRecipeByProduct(ctx context.Context, tx *gorm.DB, productID string, out *RecipeWithItems) error {
	err := scopeTenant(ctx, tenantDB(ctx, tx)).First(&out.Recipe, "product_id = ?", productID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrRecipeNotFound
	}
	if err != nil {
		return err
	}
	return scopeTenant(ctx, tenantDB(ctx, tx)).
		Where("recipe_id = ?", out.Recipe.ID).
		Order("id ASC").
		Find(&out.Items).Error
}

// RecipesByProductIDs memuat resep untuk sekumpulan produk yang punya resep
// (produk tanpa resep tidak muncul di hasil). tx wajib (dipakai di dalam
// transaksi checkout). Mengembalikan peta product_id → resep+bahan.
func RecipesByProductIDs(ctx context.Context, tx *gorm.DB, productIDs []string) (map[string]RecipeWithItems, error) {
	out := map[string]RecipeWithItems{}
	if len(productIDs) == 0 {
		return out, nil
	}

	var recipes []models.Recipe
	if err := scopeTenant(ctx, tenantDB(ctx, tx)).
		Where("product_id IN ?", productIDs).Find(&recipes).Error; err != nil {
		return nil, err
	}
	if len(recipes) == 0 {
		return out, nil
	}

	recipeIDs := make([]string, 0, len(recipes))
	byRecipeID := map[string]string{} // recipe_id → product_id
	for _, r := range recipes {
		recipeIDs = append(recipeIDs, r.ID)
		byRecipeID[r.ID] = r.ProductID
	}

	var items []models.RecipeItem
	if err := scopeTenant(ctx, tenantDB(ctx, tx)).
		Where("recipe_id IN ?", recipeIDs).Find(&items).Error; err != nil {
		return nil, err
	}

	for _, r := range recipes {
		out[r.ProductID] = RecipeWithItems{Recipe: r}
	}
	for _, it := range items {
		pid := byRecipeID[it.RecipeID]
		e := out[pid]
		e.Items = append(e.Items, it)
		out[pid] = e
	}
	return out, nil
}

// ReplaceRecipe mengganti resep sebuah produk (hapus lalu buat ulang) dalam satu
// transaksi. items boleh kosong (menghapus resep).
func ReplaceRecipe(ctx context.Context, productID string, yield models.Recipe, items []models.RecipeItem) (RecipeWithItems, error) {
	var result RecipeWithItems
	err := WithTenant(ctx, func(tx *gorm.DB) error {
		// Hapus resep lama (CASCADE menghapus recipe_items).
		if err := scopeTenant(ctx, tx).Where("product_id = ?", productID).
			Delete(&models.Recipe{}).Error; err != nil {
			return err
		}
		if len(items) == 0 {
			return nil
		}

		rec := models.Recipe{ProductID: productID, YieldQty: yield.YieldQty}
		if err := createTenant(ctx, tx, &rec); err != nil {
			return err
		}
		for i := range items {
			items[i].RecipeID = rec.ID
			if err := createTenant(ctx, tx, &items[i]); err != nil {
				return err
			}
		}
		result = RecipeWithItems{Recipe: rec, Items: items}
		return nil
	})
	return result, err
}
