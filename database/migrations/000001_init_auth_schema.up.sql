-- Migrasi 000001 — skema awal autentikasi (Fase 0).
--
-- Menggantikan AutoMigrate GORM untuk tabel `users` dan `roles` yang sudah ada,
-- ditambah fondasi yang dipakai seluruh skema berikutnya:
--   - sequence `sync_version_seq` untuk penanda sinkronisasi offline (§10)
--   - soft delete + partial unique index (§3.5) — baris terhapus tidak lagi
--     memblokir pendaftaran ulang username/email yang sama
--
-- Belum ada `tenant_id` di sini: multi-tenancy masuk di Fase 1 (migrasi 000003+).
--
-- Runner membungkus berkas ini dalam satu transaksi; jangan tulis BEGIN/COMMIT.

-- Satu sumber nomor urut global untuk kolom `sync_version` di semua tabel yang
-- disinkronkan. Dibuat sekarang walau pemakainya baru muncul di fase berikutnya,
-- supaya migrasi yang menambahkannya tinggal memakai, bukan mendefinisikan ulang.
CREATE SEQUENCE IF NOT EXISTS sync_version_seq AS BIGINT START 1;

CREATE TABLE roles (
    id         CHAR(26)    PRIMARY KEY,
    name       TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

-- Nama role unik hanya di antara baris yang masih hidup. Bentuk final
-- (per-tenant) menyusul di Fase 1.
CREATE UNIQUE INDEX uq_roles_name ON roles (name) WHERE deleted_at IS NULL;

CREATE TABLE users (
    id         CHAR(26)    PRIMARY KEY,
    name       TEXT        NOT NULL,
    username   TEXT        NOT NULL,
    email      TEXT        NOT NULL,
    password   TEXT        NOT NULL,
    role_id    CHAR(26),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT fk_users_role FOREIGN KEY (role_id) REFERENCES roles (id) ON DELETE RESTRICT
);

-- Partial unique: menggantikan unique constraint polos bawaan AutoMigrate yang
-- bentrok dengan soft delete (utang teknis di CONVENTIONS §7).
CREATE UNIQUE INDEX uq_users_username ON users (username) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX uq_users_email    ON users (email)    WHERE deleted_at IS NULL;

-- FK butuh index di sisi anak: PostgreSQL tidak membuatnya otomatis, dan tanpa
-- itu ON DELETE RESTRICT memindai seluruh tabel `users` (§5.15).
CREATE INDEX idx_users_role_id ON users (role_id);
