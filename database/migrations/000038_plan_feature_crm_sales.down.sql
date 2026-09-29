-- Membatalkan 000038: buang kunci `crm_sales` dari katalog paket bawaan —
-- tanpa kunci itu modul sales lapangan kembali terkunci untuk SEMUA paket
-- (fitur yang tidak disebut dianggap tidak termasuk), jadi hanya jalankan
-- bersama pembatalan kode kuncinya.
UPDATE plans
   SET features = features - 'crm_sales', updated_at = now()
 WHERE code IN ('free', 'basic', 'pro', 'multi');
