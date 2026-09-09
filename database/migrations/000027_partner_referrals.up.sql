-- Migrasi 000027 — Program Mitra: prospek & atribusi referral (Fase 12,
-- blueprint G.7).
--
-- `partner_leads`: prospek yang didaftarkan mitra; masa atribusi mulai berjalan
-- saat prospek dibuat (blueprint G.2 #5).
--
-- `partner_referrals`: kaitan mitra ↔ tenant. Dibuat SEKALI saat tenant
-- mendaftar memakai kode referral yang valid; `UNIQUE (tenant_id)` = satu mitra
-- per tenant, tidak boleh berubah (inilah dasar seluruh perhitungan komisi).
--
-- `tenants.referred_by_partner_id` / `referral_code_used` kolomnya sudah ada
-- sejak migrasi 000003; di sini FK-nya baru dipasang (targetnya baru ada).
--
-- Modul PLATFORM — tanpa RLS.
--
-- Runner membungkus berkas ini dalam satu transaksi; jangan tulis BEGIN/COMMIT.

CREATE TABLE partner_leads (
    id                   CHAR(26)    PRIMARY KEY,
    partner_id           CHAR(26)    NOT NULL REFERENCES partners (id) ON DELETE CASCADE,
    business_name        TEXT        NOT NULL,
    contact_name         TEXT,
    contact_phone        TEXT,
    city                 TEXT,
    business_type        TEXT,
    status               TEXT        NOT NULL DEFAULT 'baru'
                         CHECK (status IN ('baru','dihubungi','demo','daftar','gagal')),
    registered_tenant_id CHAR(26) REFERENCES tenants (id) ON DELETE SET NULL,
    attribution_expires_at DATE      NOT NULL,   -- created + tier.attribution_days
    note                 TEXT,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_partner_leads_partner ON partner_leads (partner_id, status);

CREATE TABLE partner_referrals (
    id                 CHAR(26)    PRIMARY KEY,
    partner_id         CHAR(26)    NOT NULL REFERENCES partners (id) ON DELETE RESTRICT,
    tenant_id          CHAR(26)    NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    lead_id            CHAR(26) REFERENCES partner_leads (id) ON DELETE SET NULL,
    referral_code      TEXT        NOT NULL,
    attributed_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    attribution_status TEXT        NOT NULL DEFAULT 'active'
                       CHECK (attribution_status IN ('active','lapsed','disputed','revoked')),
    activated_at       TIMESTAMPTZ,   -- saat ambang aktivasi merchant terpenuhi
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id)
);
CREATE INDEX idx_partner_referrals_partner ON partner_referrals (partner_id, attribution_status);

-- FK yang ditangguhkan dari 000003 (§5.16): targetnya baru ada sekarang.
-- Rapikan dulu nilai yang tak menunjuk mitra nyata sebelum memasang FK:
--   - '' → CHAR(26) di-blank-pad jadi 26 spasi (model lama menyimpannya sebagai
--     string biasa; GORM menyisipkan '').
--   - ULID yatim yang tertinggal dari down/up migrasi ini sebelumnya.
-- Keduanya dikembalikan ke NULL.
UPDATE tenants SET referred_by_partner_id = NULL
    WHERE referred_by_partner_id IS NOT NULL
      AND btrim(referred_by_partner_id) NOT IN (SELECT id FROM partners);

ALTER TABLE tenants
    ADD CONSTRAINT fk_tenants_referred_by_partner
    FOREIGN KEY (referred_by_partner_id) REFERENCES partners (id) ON DELETE SET NULL;
