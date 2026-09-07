-- Membatalkan 000006. Extension pg_trgm dibiarkan terpasang (mungkin dipakai
-- objek lain); hanya index yang dilepas.
-- Runner membungkus berkas ini dalam satu transaksi; jangan tulis BEGIN/COMMIT.

DROP INDEX IF EXISTS idx_products_tenant_name_trgm;
