-- Migrasi 000050 — pemasok utama per barang.
--
-- Satu barang biasa dibeli dari satu pemasok langganan. Dengan mencatatnya,
-- saran belanja bisa dikelompokkan per pemasok dan dipesan sekaligus lewat
-- WhatsApp (layar Pemasok › "Perlu dipesan" & "Pesan lagi").
--
-- Diisi: (a) di formulir barang; (b) OTOMATIS dari barang masuk pertama yang
-- menyebut pemasok, bila barangnya belum punya pemasok utama; (c) sekali di
-- sini, dari pemasok barang masuk TERAKHIR yang memuat barang itu.

ALTER TABLE products ADD COLUMN supplier_id CHAR(26);
ALTER TABLE products ADD CONSTRAINT products_supplier_fk
    FOREIGN KEY (tenant_id, supplier_id) REFERENCES suppliers (tenant_id, id) ON DELETE RESTRICT;
CREATE INDEX idx_products_tenant_supplier ON products (tenant_id, supplier_id) WHERE supplier_id IS NOT NULL;

UPDATE products p SET supplier_id = t.supplier_id
FROM (
    SELECT DISTINCT ON (pi.tenant_id, pi.product_id) pi.tenant_id, pi.product_id, pu.supplier_id
    FROM purchase_items pi
    JOIN purchases pu ON pu.tenant_id = pi.tenant_id AND pu.id = pi.purchase_id
    JOIN suppliers s ON s.tenant_id = pu.tenant_id AND s.id = pu.supplier_id AND s.deleted_at IS NULL
    WHERE pu.supplier_id IS NOT NULL AND pu.status = 'received'
    ORDER BY pi.tenant_id, pi.product_id, pu.occurred_at DESC
) t
WHERE p.tenant_id = t.tenant_id AND p.id = t.product_id AND p.supplier_id IS NULL;
