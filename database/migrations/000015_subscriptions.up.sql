-- Migrasi 000015 — langganan & tagihan platform (Fase 7, §5.13).
--
-- Ini tabel PLATFORM: milik penyedia SaaS, bukan data operasional tenant.
-- `tenant_id` di sini adalah PENUNJUK PELANGGAN, bukan pembatas akses (§5 tabel
-- klasifikasi) — jadi TIDAK ada Row Level Security di sini. Isolasi untuk
-- endpoint yang menghadap tenant dijaga di lapisan repo/service dengan filter
-- `tenant_id = <konteks>` eksplisit.
--
-- Penamaan `subscription_invoices` / `subscription_payments` sengaja dipakai
-- agar tak bentrom dengan `invoices` pelanggan di modul CRM (§5.13, blueprint).
--
-- Runner membungkus berkas ini dalam satu transaksi; jangan tulis BEGIN/COMMIT.

-- Katalog paket. Tanpa tenant_id — global.
CREATE TABLE plans (
    id                       CHAR(26)    PRIMARY KEY,
    code                     TEXT        NOT NULL UNIQUE,   -- 'free','basic','pro','multi'
    name                     TEXT        NOT NULL,
    monthly_price            BIGINT      NOT NULL,
    max_outlets              INT,
    max_users                INT,
    max_products             INT,
    max_monthly_transactions INT,
    features                 JSONB       NOT NULL DEFAULT '{}',
    is_active                BOOLEAN     NOT NULL DEFAULT true,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Tangga diskon prabayar 3/6/9/12 bulan. Global.
CREATE TABLE plan_term_discounts (
    id            CHAR(26)     PRIMARY KEY,
    term_months   INT          NOT NULL CHECK (term_months IN (1,3,6,9,12)),
    discount_rate NUMERIC(7,4) NOT NULL,   -- pecahan: 0.1670 = 16,7%
    is_active     BOOLEAN      NOT NULL DEFAULT true,
    UNIQUE (term_months)
);

-- Satu langganan aktif per tenant (UNIQUE tenant_id).
CREATE TABLE subscriptions (
    id                   CHAR(26)     PRIMARY KEY,
    tenant_id            CHAR(26)     NOT NULL REFERENCES tenants (id) ON DELETE RESTRICT,
    plan_id              CHAR(26)     NOT NULL REFERENCES plans (id) ON DELETE RESTRICT,
    term_months          INT          NOT NULL DEFAULT 1 CHECK (term_months IN (1,3,6,9,12)),
    discount_rate        NUMERIC(7,4) NOT NULL DEFAULT 0,
    status               TEXT         NOT NULL DEFAULT 'trial'
                         CHECK (status IN ('trial','active','past_due','canceled','expired')),
    trial_ends_at        TIMESTAMPTZ,
    current_period_start TIMESTAMPTZ  NOT NULL,
    current_period_end   TIMESTAMPTZ  NOT NULL,
    auto_renew           BOOLEAN      NOT NULL DEFAULT true,
    canceled_at          TIMESTAMPTZ,
    cancel_reason        TEXT,
    created_at           TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ  NOT NULL DEFAULT now(),
    UNIQUE (tenant_id)
);

CREATE TABLE subscription_addons (
    id              CHAR(26)    PRIMARY KEY,
    subscription_id CHAR(26)    NOT NULL REFERENCES subscriptions (id) ON DELETE CASCADE,
    addon_code      TEXT        NOT NULL CHECK (addon_code IN ('crm_freelance','crm_sales','online_channel')),
    quantity        INT         NOT NULL DEFAULT 1,
    unit_price      BIGINT      NOT NULL,
    started_at      TIMESTAMPTZ NOT NULL,
    ended_at        TIMESTAMPTZ,
    UNIQUE (subscription_id, addon_code)
);

-- Nomor faktur langganan berurutan global: SUB-000001, SUB-000002, ...
CREATE SEQUENCE subscription_invoice_seq AS BIGINT START 1;

CREATE TABLE subscription_invoices (
    id              CHAR(26)    PRIMARY KEY,
    tenant_id       CHAR(26)    NOT NULL REFERENCES tenants (id) ON DELETE RESTRICT,
    subscription_id CHAR(26)    NOT NULL REFERENCES subscriptions (id) ON DELETE RESTRICT,
    number          TEXT        NOT NULL UNIQUE,
    term_months     INT         NOT NULL,
    period_start    DATE        NOT NULL,
    period_end      DATE        NOT NULL,
    gross_amount    BIGINT      NOT NULL,          -- sebelum diskon
    discount_amount BIGINT      NOT NULL DEFAULT 0,
    total_amount    BIGINT      NOT NULL,          -- yang ditagihkan
    paid_amount     BIGINT      NOT NULL DEFAULT 0, -- dasar komisi mitra (§13.3)
    due_date        DATE        NOT NULL,
    status          TEXT        NOT NULL DEFAULT 'open'
                    CHECK (status IN ('open','paid','overdue','void','refunded')),
    paid_at         TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_sub_invoices_status_due ON subscription_invoices (status, due_date);
CREATE INDEX idx_sub_invoices_tenant     ON subscription_invoices (tenant_id);

CREATE TABLE subscription_payments (
    id                      CHAR(26)    PRIMARY KEY,
    subscription_invoice_id CHAR(26)    NOT NULL REFERENCES subscription_invoices (id) ON DELETE RESTRICT,
    amount                  BIGINT      NOT NULL CHECK (amount > 0),
    method                  TEXT        NOT NULL,
    reference               TEXT,
    gateway_fee             BIGINT      NOT NULL DEFAULT 0,
    paid_at                 TIMESTAMPTZ NOT NULL,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_sub_payments_invoice ON subscription_payments (subscription_invoice_id);
