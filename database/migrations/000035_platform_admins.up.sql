-- Migrasi 000035 — akun ADMIN PLATFORM (panel internal, blueprint G.5).
--
-- PERLUASAN di luar DDL §5: dokumen menyebut "peran admin platform" (§5.17) dan
-- `audit_logs.actor_type = 'admin'` (§5.14), tetapi tabel akunnya belum pernah
-- didefinisikan. Sampai sekarang verifikasi mitra & siklus komisi dijalankan
-- lewat CLI (cmd/partner-admin, cmd/partner-commissions) — jalan keluar
-- sementara yang sudah dicatat di komentar service Fase 12.
--
-- Ini realm KETIGA, sejajar dan terpisah penuh dari dua yang sudah ada:
--   users          → realm tenant   (rlm kosong)  — orang di dalam usaha
--   partner_users  → realm partner  (rlm partner) — mitra penjual
--   platform_admins→ realm platform (rlm platform)— kita, penyedia SaaS
-- Token satu realm SELALU ditolak di realm lain.
--
-- Perannya SENGAJA tetap (bukan dinamis seperti peran tenant): penggunanya
-- segelintir staf internal, dan aturan siapa boleh mencairkan uang mitra tidak
-- boleh bisa diubah lewat UI oleh salah satu dari mereka sendiri.
--
--   superadmin → semua, termasuk mengelola admin lain
--   operator   → verifikasi mitra, tingkat mitra, sengketa
--   finance    → jalankan komisi, setujui, cairkan
--   support    → hanya membaca
--
-- Tabel PLATFORM — tanpa RLS, tanpa tenant_id (§5.17).
--
-- Runner membungkus berkas ini dalam satu transaksi; jangan tulis BEGIN/COMMIT.

CREATE TABLE platform_admins (
    id            CHAR(26)    PRIMARY KEY,
    name          TEXT        NOT NULL,
    email         TEXT        NOT NULL,
    password_hash TEXT        NOT NULL,
    role          TEXT        NOT NULL
                  CHECK (role IN ('superadmin','operator','finance','support')),
    is_active     BOOLEAN     NOT NULL DEFAULT true,
    last_login_at TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at    TIMESTAMPTZ
);
CREATE UNIQUE INDEX uq_platform_admins_email ON platform_admins (email) WHERE deleted_at IS NULL;
CREATE INDEX idx_platform_admins_role ON platform_admins (role) WHERE deleted_at IS NULL;
