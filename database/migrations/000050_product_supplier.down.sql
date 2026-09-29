-- Membatalkan 000050. Pemasok utama tiap barang hilang.
DROP INDEX IF EXISTS idx_products_tenant_supplier;
ALTER TABLE products DROP CONSTRAINT IF EXISTS products_supplier_fk;
ALTER TABLE products DROP COLUMN IF EXISTS supplier_id;
