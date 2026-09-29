-- Membatalkan 000046. Transaksi per kemasan kehilangan jejak kemasannya
-- (stok yang sudah bergerak tidak diubah).
ALTER TABLE purchase_items DROP COLUMN IF EXISTS unit_name,
    DROP COLUMN IF EXISTS unit_conversion, DROP COLUMN IF EXISTS product_unit_id;
ALTER TABLE sale_items DROP COLUMN IF EXISTS unit_conversion, DROP COLUMN IF EXISTS product_unit_id;
DROP TABLE IF EXISTS product_units;
