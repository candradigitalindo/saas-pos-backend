-- Migrasi 000006 — pencarian produk cepat (Fase 2).
--
-- Target blueprint: cari produk dari 500+ katalog dalam < 200 ms. Index GIN
-- trigram membuat `name ILIKE '%kata%'` tidak perlu full scan.
--
-- Index ini sengaja TIDAK diawali tenant_id (berbeda dari §5.15 aturan 1):
-- itulah bentuk yang ditulis di §5.4, dan GIN trigram tidak bisa dipimpin kolom
-- btree biasa tanpa extension btree_gin. Query tetap menyaring tenant_id
-- (lapisan 1) + RLS (lapisan 2); trigram hanya mempercepat pencocokan nama.
--
-- pg_trgm adalah extension "trusted" sejak PostgreSQL 13 — pemilik database bisa
-- memasangnya tanpa hak superuser.
--
-- Runner membungkus berkas ini dalam satu transaksi; jangan tulis BEGIN/COMMIT.

CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX idx_products_tenant_name_trgm ON products USING gin (name gin_trgm_ops);
