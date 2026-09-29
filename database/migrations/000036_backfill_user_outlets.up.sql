-- Migrasi 000036 — isi akses cabang (user_outlets) untuk staf yang belum punya.
--
-- Sampai migrasi ini, `user_outlets` hanya diisi SEKALI: untuk pemilik saat
-- pendaftaran usaha. Staf yang dibuat lewat POST /users tidak pernah mendapat
-- baris di sini, sehingga:
--   - /me mengembalikan outlet_ids kosong dan layar kasir web tidak punya toko
--     aktif untuk siapa pun selain pemilik;
--   - pagar akses outlet (§9, §13.1 langkah 0) tidak bisa dipasang tanpa
--     mengunci seluruh staf lama keluar dari kasir.
--
-- Mulai versi ini staf baru tanpa `outlet_ids` otomatis mendapat SEMUA cabang
-- aktif, dan layanan menegakkan akses per outlet. Backfill ini memberi staf
-- lama perlakuan yang sama, jadi tidak ada yang kehilangan akses yang de facto
-- sudah ia pakai. Staf yang SUDAH punya baris (pemilik) tidak disentuh.
--
-- Runner membungkus berkas ini dalam satu transaksi; jangan tulis BEGIN/COMMIT.

INSERT INTO user_outlets (tenant_id, user_id, outlet_id)
SELECT u.tenant_id, u.id, o.id
  FROM users u
  JOIN outlets o ON o.tenant_id = u.tenant_id
                AND o.deleted_at IS NULL
                AND o.is_active
 WHERE u.tenant_id IS NOT NULL
   AND u.deleted_at IS NULL
   AND NOT EXISTS (SELECT 1 FROM user_outlets uo WHERE uo.user_id = u.id)
ON CONFLICT DO NOTHING;
