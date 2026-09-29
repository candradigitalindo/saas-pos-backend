-- Membatalkan 000037: kembali ke UNIQUE biasa (NULL dianggap berbeda). Baris
-- ganda yang sudah dirapikan tidak dikembalikan.
ALTER TABLE stock_opname_items DROP CONSTRAINT uq_stock_opname_items_product;

ALTER TABLE stock_opname_items
    ADD CONSTRAINT stock_opname_items_tenant_id_opname_id_product_id_variant_i_key
    UNIQUE (tenant_id, opname_id, product_id, variant_id);
