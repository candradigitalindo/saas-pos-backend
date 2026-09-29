-- Migrasi 000037 — satu baris hitungan per barang dalam satu opname, termasuk
-- barang TANPA varian.
--
-- 000011 memasang UNIQUE (tenant_id, opname_id, product_id, variant_id) sebagai
-- sasaran upsert repositories.UpsertOpnameItem. Untuk barang tanpa varian,
-- variant_id bernilai NULL — dan di PostgreSQL NULL tidak pernah sama dengan
-- NULL, jadi constraint itu TIDAK PERNAH bentrok dan ON CONFLICT tidak pernah
-- terpicu. Setiap kali hitungan barang dikoreksi, opname mendapat baris BARU,
-- dan saat diposting SEMUA selisihnya diterapkan: hitung 95 lalu dikoreksi 97
-- (stok sistem 100) berakhir di stok 92, bukan 97.
--
-- UNIQUE NULLS NOT DISTINCT (PostgreSQL ≥ 15; proyek ini memakai 16 di lokal,
-- docker-compose, dan CI) memperlakukan NULL sebagai nilai yang sama, sehingga
-- upsert yang sudah ada bekerja sebagaimana mestinya tanpa mengubah kodenya.
--
-- Baris ganda yang sudah telanjur ada dirapikan dulu: yang dipertahankan adalah
-- hitungan TERAKHIR (id ULID terbesar), karena itulah yang dimaksud penghitung.
-- Gerakan stok dari opname yang SUDAH diposting tidak disentuh — buku besar
-- tetap mencatat apa yang benar-benar terjadi.
--
-- Runner membungkus berkas ini dalam satu transaksi; jangan tulis BEGIN/COMMIT.

DELETE FROM stock_opname_items a
 USING stock_opname_items b
 WHERE a.tenant_id  = b.tenant_id
   AND a.opname_id  = b.opname_id
   AND a.product_id = b.product_id
   AND a.variant_id IS NOT DISTINCT FROM b.variant_id
   AND a.id < b.id;

ALTER TABLE stock_opname_items
    DROP CONSTRAINT stock_opname_items_tenant_id_opname_id_product_id_variant_i_key;

ALTER TABLE stock_opname_items
    ADD CONSTRAINT uq_stock_opname_items_product
    UNIQUE NULLS NOT DISTINCT (tenant_id, opname_id, product_id, variant_id);
