-- Migrasi 000026 — Program Mitra Penjual: mitra, tingkat, akun login
-- (Fase 12, §16, blueprint Bagian G).
--
-- Ini modul PLATFORM (milik penyedia SaaS untuk merekrut tenant), sejajar
-- dengan `plans`/`subscriptions` — BUKAN data operasional tenant. Karena itu
-- TIDAK ada Row Level Security di sini; isolasi antar mitra dijaga di lapisan
-- repo/service dengan filter `partner_id` eksplisit.
--
-- Mitra masuk lewat JALUR AUTENTIKASI TERPISAH (`partner_users`) dan sama sekali
-- bukan user tenant mana pun (blueprint G.8). Yang boleh mereka lihat tentang
-- merchant binaannya hanya: status langganan, tanggal jatuh tempo, penanda
-- aktif — tidak pernah omzet, produk, harga, pelanggan, atau isi transaksi.
--
-- Satu tingkat saja, tidak ada jaringan berjenjang (blueprint G.3).
--
-- Runner membungkus berkas ini dalam satu transaksi; jangan tulis BEGIN/COMMIT.

-- Tingkat mitra + aturan komisinya. Dapat diubah tanpa menyentuh kode
-- (blueprint G.5). Global.
CREATE TABLE partner_tiers (
    id                  CHAR(26)     PRIMARY KEY,
    code                TEXT         NOT NULL UNIQUE,   -- 'afiliasi','agen','agen_senior'
    name                TEXT         NOT NULL,
    kind                TEXT         NOT NULL CHECK (kind IN ('agen','afiliasi')),
    commission_rate     NUMERIC(7,4) NOT NULL DEFAULT 0,   -- pecahan atas pembayaran DITERIMA
    recurring           BOOLEAN      NOT NULL DEFAULT true, -- berulang selama merchant aktif
    one_time_months     INT          NOT NULL DEFAULT 0,    -- bila NOT recurring: batas bulan sejak atribusi
    activation_min_txn  INT          NOT NULL DEFAULT 30,   -- syarat aktivasi (blueprint G.2 #3)
    activation_min_days INT          NOT NULL DEFAULT 30,
    attribution_days    INT          NOT NULL DEFAULT 60,   -- masa atribusi prospek (G.2 #5)
    clawback_days       INT          NOT NULL DEFAULT 90,   -- clawback bila merchant berhenti (G.2 #4)
    is_active           BOOLEAN      NOT NULL DEFAULT true,
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CHECK (recurring OR one_time_months > 0)
);

-- Mitra. `referral_code` unik global — dipakai calon tenant saat mendaftar.
CREATE TABLE partners (
    id                   CHAR(26)     PRIMARY KEY,
    tier_id              CHAR(26)     NOT NULL REFERENCES partner_tiers (id) ON DELETE RESTRICT,
    kind                 TEXT         NOT NULL CHECK (kind IN ('agen','afiliasi')),
    name                 TEXT         NOT NULL,
    region               TEXT,
    referral_code        TEXT         NOT NULL UNIQUE,
    status               TEXT         NOT NULL DEFAULT 'pending'
                         CHECK (status IN ('pending','active','suspended','terminated')),
    bank_account         TEXT,
    tax_id               TEXT,                                -- NPWP
    tax_withholding_rate NUMERIC(7,4) NOT NULL DEFAULT 0,     -- potongan pajak di pencairan (G.2 #6)
    joined_at            DATE,
    created_at           TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX idx_partners_tier ON partners (tier_id);

-- Akun login mitra — satu mitra boleh punya beberapa orang (blueprint G.7).
CREATE TABLE partner_users (
    id            CHAR(26)    PRIMARY KEY,
    partner_id    CHAR(26)    NOT NULL REFERENCES partners (id) ON DELETE CASCADE,
    name          TEXT        NOT NULL,
    email         TEXT        NOT NULL,
    username      TEXT        NOT NULL UNIQUE,
    password_hash TEXT        NOT NULL,
    is_active     BOOLEAN     NOT NULL DEFAULT true,
    last_login_at TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_partner_users_partner ON partner_users (partner_id);
