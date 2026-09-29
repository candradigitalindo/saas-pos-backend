-- Migrasi 000027 — Program Mitra: prospek & atribusi referral (Fase 12, §5.12).
--
-- `partner_leads`: prospek yang didaftarkan mitra; masa atribusi mulai berjalan
-- saat prospek dibuat (blueprint G.2 #5).
--
-- `partner_referrals`: kaitan mitra ↔ tenant. Dibuat SEKALI saat tenant
-- mendaftar memakai kode referral yang valid; `UNIQUE (tenant_id)` = satu mitra
-- per tenant, tidak boleh berubah (dasar seluruh perhitungan komisi).
--
-- `tenants.referred_by_partner_id` / `referral_code_used` kolomnya sudah ada
-- sejak migrasi 000003; di sini FK-nya baru dipasang — constraint menyusul
-- sesuai tabel §5.16.
--
-- Modul PLATFORM — tanpa RLS, FK tunggal (§5.17).
--
-- Runner membungkus berkas ini dalam satu transaksi; jangan tulis BEGIN/COMMIT.

CREATE TABLE partner_leads (
    id                     CHAR(26)    PRIMARY KEY,
    partner_id             CHAR(26)    NOT NULL REFERENCES partners (id) ON DELETE CASCADE,
    business_name          TEXT        NOT NULL,
    contact_name           TEXT,
    phone                  TEXT        NOT NULL,
    city                   TEXT,
    business_type          TEXT,
    note                   TEXT,
    status                 TEXT        NOT NULL DEFAULT 'new'
                           CHECK (status IN ('new','contacted','demo','registered','activated','lost')),
    attribution_expires_at TIMESTAMPTZ NOT NULL,   -- default +tier.attribution_days
    converted_tenant_id    CHAR(26) REFERENCES tenants (id) ON DELETE SET NULL,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_partner_leads_phone     ON partner_leads (phone);
CREATE INDEX idx_partner_leads_partner   ON partner_leads (partner_id, status);
CREATE INDEX idx_partner_leads_converted ON partner_leads (converted_tenant_id);

CREATE TABLE partner_referrals (   -- kaitan mitra ↔ tenant, dasar seluruh komisi
    id                   CHAR(26)    PRIMARY KEY,
    partner_id           CHAR(26)    NOT NULL REFERENCES partners (id) ON DELETE RESTRICT,
    tenant_id            CHAR(26)    NOT NULL REFERENCES tenants (id) ON DELETE RESTRICT,
    lead_id              CHAR(26) REFERENCES partner_leads (id) ON DELETE SET NULL,
    referral_code        TEXT        NOT NULL,
    attributed_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    activated_at         TIMESTAMPTZ,               -- saat ambang aktivasi terpenuhi
    commission_starts_at TIMESTAMPTZ,
    commission_ends_at   TIMESTAMPTZ,               -- untuk tier berjangka
    status               TEXT        NOT NULL DEFAULT 'pending'
                         CHECK (status IN ('pending','active','ended','disputed','revoked')),
    UNIQUE (tenant_id)                              -- satu tenant → satu mitra
);
CREATE INDEX idx_partner_referrals_partner ON partner_referrals (partner_id, status);
CREATE INDEX idx_partner_referrals_lead    ON partner_referrals (lead_id);

-- FK yang ditangguhkan dari 000003 (§5.16): targetnya baru ada sekarang.
-- Rapikan dulu nilai yang tak menunjuk mitra nyata sebelum memasang FK:
--   - '' → CHAR(26) di-blank-pad jadi 26 spasi (model lama menyimpannya sebagai
--     string biasa; GORM menyisipkan '').
--   - ULID yatim yang tertinggal dari down/up migrasi ini sebelumnya.
UPDATE tenants SET referred_by_partner_id = NULL
    WHERE referred_by_partner_id IS NOT NULL
      AND btrim(referred_by_partner_id) NOT IN (SELECT id FROM partners);

ALTER TABLE tenants
    ADD CONSTRAINT fk_tenants_referred_by_partner
    FOREIGN KEY (referred_by_partner_id) REFERENCES partners (id) ON DELETE SET NULL;
CREATE INDEX idx_tenants_referred_by ON tenants (referred_by_partner_id);
