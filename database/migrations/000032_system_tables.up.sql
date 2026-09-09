-- Migrasi 000032 — tabel sistem §5.14 yang belum dibuat: audit_logs,
-- outbox_events, notification_templates.
--
-- `audit_logs` LINTAS LINGKUP: `tenant_id` boleh NULL untuk aksi platform/mitra,
-- dan `actor_type` membedakan user tenant, akun mitra, sistem, atau admin.
-- Karena itulah jejak audit akses mitra ke data merchant (blueprint G.8) memakai
-- tabel ini dengan actor_type='partner_user' — bukan tabel khusus sendiri.
--
-- TANPA RLS: `audit_logs` harus tetap terbaca oleh proses platform meski GUC
-- tenant tidak disetel, dan barisnya sengaja abadi (§5.16: referensi
-- target_table/target_id polimorfik, tidak ber-FK, agar log tetap ada walau
-- baris rujukannya diarsipkan). Endpoint yang menghadap tenant WAJIB memfilter
-- `tenant_id` eksplisit.
--
-- `outbox_events` = pola outbox untuk pengiriman andal (notifikasi/webhook
-- keluar). Belum ada produsen/konsumen di kode; tabelnya dibuat sekarang agar
-- relasi & indexnya tidak "hilang diam-diam" (§5.16).
--
-- Runner membungkus berkas ini dalam satu transaksi; jangan tulis BEGIN/COMMIT.

CREATE TABLE audit_logs (
    id           CHAR(26)    PRIMARY KEY,
    tenant_id    CHAR(26),                  -- NULL untuk aksi platform/mitra
    actor_type   TEXT        NOT NULL CHECK (actor_type IN ('user','partner_user','system','admin')),
    actor_id     CHAR(26),
    action       TEXT        NOT NULL,      -- 'sale.void', 'partner.merchant.view'
    target_table TEXT,
    target_id    CHAR(26),
    before_data  JSONB,
    after_data   JSONB,
    ip_address   INET,
    user_agent   TEXT,
    occurred_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_audit_tenant_time ON audit_logs (tenant_id, occurred_at DESC);
CREATE INDEX idx_audit_target      ON audit_logs (target_table, target_id);
CREATE INDEX idx_audit_actor       ON audit_logs (actor_type, actor_id, occurred_at DESC);

CREATE TABLE outbox_events (
    id           CHAR(26)    PRIMARY KEY,
    tenant_id    CHAR(26),
    topic        TEXT        NOT NULL,      -- 'invoice.due', 'sale.completed'
    payload      JSONB       NOT NULL,
    status       TEXT        NOT NULL DEFAULT 'pending'
                 CHECK (status IN ('pending','processing','done','failed','dead')),
    attempts     INT         NOT NULL DEFAULT 0,
    last_error   TEXT,
    available_at TIMESTAMPTZ NOT NULL DEFAULT now(),   -- penundaan bertahap
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    processed_at TIMESTAMPTZ
);
CREATE INDEX idx_outbox_ready ON outbox_events (status, available_at)
    WHERE status IN ('pending','failed');

CREATE TABLE notification_templates (
    id        CHAR(26) PRIMARY KEY,
    tenant_id CHAR(26),                     -- NULL = template bawaan sistem
    code      TEXT     NOT NULL,
    channel   TEXT     NOT NULL CHECK (channel IN ('whatsapp','email')),
    subject   TEXT,
    body      TEXT     NOT NULL
);
CREATE UNIQUE INDEX uq_notif_templates
    ON notification_templates (COALESCE(tenant_id, ''), code, channel);
