-- Migrasi 000034 — satu user boleh memegang BEBERAPA peran.
--
-- PERLUASAN di luar DDL §5.3, disengaja: §5.3 hanya mengenal `users.role_id`
-- (satu peran per user). Kebutuhan nyata UMKM tidak sesempit itu — di toko
-- kecil satu orang lazim merangkap kasir sekaligus gudang, dan memaksa pemilik
-- membuat peran gabungan "Kasir+Gudang" untuk tiap kombinasi membuat daftar
-- peran meledak. Ini melengkapi peran dinamis per tenant, bukan menggantinya.
--
-- Pembagian peran setelah migrasi ini:
--   - `user_roles`   = SUMBER KEBENARAN hak akses. Izin efektif seorang user =
--                      GABUNGAN izin seluruh peran miliknya.
--   - `users.role_id` = peran UTAMA: yang ditampilkan di UI sebagai jabatan
--                      utama, dan default saat klien lama hanya mengirim satu
--                      peran. Nilainya WAJIB ikut ada di `user_roles` — dijaga
--                      di satu tempat (repositories.SetUserRoles).
--
-- Tabel penghubung murni: primary key-nya pasangan kunci asing, tanpa kolom
-- `id` (§5.17), sama seperti `user_outlets`. FK komposit karena `users` dan
-- `roles` sama-sama tabel bertenant (§5.17).
--
-- Runner membungkus berkas ini dalam satu transaksi; jangan tulis BEGIN/COMMIT.

CREATE TABLE user_roles (
    tenant_id CHAR(26) NOT NULL,
    user_id   CHAR(26) NOT NULL,
    role_id   CHAR(26) NOT NULL,
    PRIMARY KEY (user_id, role_id),
    FOREIGN KEY (tenant_id, user_id) REFERENCES users (tenant_id, id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, role_id) REFERENCES roles (tenant_id, id) ON DELETE RESTRICT
);
CREATE INDEX idx_user_roles_user ON user_roles (tenant_id, user_id);
CREATE INDEX idx_user_roles_role ON user_roles (tenant_id, role_id);

-- Backfill: setiap user yang sudah punya peran utama mendapat baris di sini,
-- sehingga hak aksesnya persis sama seperti sebelum migrasi.
INSERT INTO user_roles (tenant_id, user_id, role_id)
SELECT tenant_id, id, role_id FROM users
 WHERE role_id IS NOT NULL AND tenant_id IS NOT NULL
ON CONFLICT DO NOTHING;

-- Row Level Security, sama seperti tabel bertenant lainnya (§6).
ALTER TABLE user_roles ENABLE ROW LEVEL SECURITY;
ALTER TABLE user_roles FORCE  ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON user_roles
    USING      (COALESCE(current_setting('app.tenant_id', true), '') = '' OR tenant_id = current_setting('app.tenant_id', true))
    WITH CHECK (COALESCE(current_setting('app.tenant_id', true), '') = '' OR tenant_id = current_setting('app.tenant_id', true));
