-- Migrasi 000026 — Program Mitra Penjual: tingkat, mitra, akun login
-- (Fase 12, §5.12, blueprint Bagian G).
--
-- Skema mengikuti DDL §5.12 apa adanya (nama kolom, nilai enum, constraint).
--
-- Ini modul PLATFORM (milik penyedia SaaS untuk merekrut tenant), sejajar
-- dengan `plans`/`subscriptions` — BUKAN data operasional tenant. Karena itu
-- TIDAK ada Row Level Security dan FK-nya TUNGGAL, bukan komposit (§5.17);
-- isolasi antar mitra dijaga filter `partner_id` eksplisit di repo.
--
-- Mitra masuk lewat JALUR AUTENTIKASI TERPISAH (`partner_users`, login pakai
-- EMAIL) dan sama sekali bukan user tenant mana pun (blueprint G.8).
--
-- Satu tingkat saja, tidak ada jaringan berjenjang: TIDAK ADA kolom
-- `upline_id`/`parent_partner_id` di `partners` (§5.12 catatan penutup).
--
-- Runner membungkus berkas ini dalam satu transaksi; jangan tulis BEGIN/COMMIT.

CREATE TABLE partner_tiers (
    id                   CHAR(26)     PRIMARY KEY,
    name                 TEXT         NOT NULL UNIQUE,   -- 'Afiliasi','Agen','Agen Utama'
    kind                 TEXT         NOT NULL CHECK (kind IN ('affiliate','agent')),
    recurring_rate       NUMERIC(7,4) NOT NULL,          -- 0.1500 = 15%
    recurring_months     INT,                            -- NULL = selama merchant aktif
    activation_bonus     BIGINT       NOT NULL DEFAULT 0,
    min_active_merchants INT          NOT NULL DEFAULT 0,
    -- TAMBAHAN di luar §5.12, disengaja: blueprint G.2 #3 & #4 mewajibkan ambang
    -- aktivasi dan masa clawback, tetapi DDL §5.12 tidak memberi mereka tempat.
    -- Ditaruh per-tingkat agar bisa diubah tanpa menyentuh kode (blueprint G.5).
    activation_min_txn   INT          NOT NULL DEFAULT 30,
    activation_min_days  INT          NOT NULL DEFAULT 30,
    attribution_days     INT          NOT NULL DEFAULT 60,
    clawback_days        INT          NOT NULL DEFAULT 90,
    created_at           TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE TABLE partners (
    id                   CHAR(26)     PRIMARY KEY,
    tier_id              CHAR(26)     NOT NULL REFERENCES partner_tiers (id) ON DELETE RESTRICT,
    kind                 TEXT         NOT NULL CHECK (kind IN ('affiliate','agent')),
    name                 TEXT         NOT NULL,
    phone                TEXT         NOT NULL,
    email                TEXT,
    region               TEXT,
    referral_code        TEXT         NOT NULL UNIQUE,   -- dipakai merchant saat mendaftar
    id_number            TEXT,                           -- KTP, untuk verifikasi
    npwp                 TEXT,                           -- untuk bukti potong pajak
    bank_name            TEXT,
    bank_account_no      TEXT,
    bank_account_name    TEXT,
    status               TEXT         NOT NULL DEFAULT 'pending'
                         CHECK (status IN ('pending','verified','active','suspended','terminated')),
    verified_at          TIMESTAMPTZ,
    joined_at            TIMESTAMPTZ,
    -- TAMBAHAN di luar §5.12: tarif potong pajak dipisah di pencairan
    -- (blueprint G.2 #6). §5.12 menyimpan npwp tapi tidak tarifnya.
    tax_withholding_rate NUMERIC(7,4) NOT NULL DEFAULT 0,
    created_at           TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX idx_partners_tier   ON partners (tier_id);
CREATE INDEX idx_partners_status ON partners (status);

CREATE TABLE partner_users (   -- jalur autentikasi TERPISAH dari users tenant
    id            CHAR(26)    PRIMARY KEY,
    partner_id    CHAR(26)    NOT NULL REFERENCES partners (id) ON DELETE CASCADE,
    name          TEXT        NOT NULL,
    email         TEXT        NOT NULL,
    phone         TEXT,
    password_hash TEXT        NOT NULL,
    is_active     BOOLEAN     NOT NULL DEFAULT true,
    last_login_at TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at    TIMESTAMPTZ
);
CREATE UNIQUE INDEX uq_partner_users_email ON partner_users (email) WHERE deleted_at IS NULL;
CREATE INDEX idx_partner_users_partner ON partner_users (partner_id);
