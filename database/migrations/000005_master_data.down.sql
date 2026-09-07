-- Membatalkan 000005. Drop berurutan mundur; FK CASCADE membereskan anak.
-- Runner membungkus berkas ini dalam satu transaksi; jangan tulis BEGIN/COMMIT.

DROP TABLE IF EXISTS dining_tables;
DROP TABLE IF EXISTS recipe_items;
DROP TABLE IF EXISTS recipes;
DROP TABLE IF EXISTS product_prices;
DROP TABLE IF EXISTS price_lists;
DROP TABLE IF EXISTS product_variants;
DROP TABLE IF EXISTS products;
DROP TABLE IF EXISTS suppliers;
DROP TABLE IF EXISTS units;
DROP TABLE IF EXISTS categories;
