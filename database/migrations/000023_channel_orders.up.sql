-- Migrasi 000023 — kanal pesanan online: metadata pesanan per kanal
-- (Fase 11a, §5.10).
--
-- `channel_orders` menyimpan data KHAS KANAL atas sebuah `sales` (id pesanan
-- eksternal, pembeli, alamat, kurir). Satu pesanan kanal = satu `sales` =
-- satu baris `channel_orders`. `UNIQUE (tenant, channel, external_order_id)`
-- adalah pengaman dedup — kanal yang mengirim pesanan yang sama dua kali tetap
-- menghasilkan satu penjualan.
--
-- Runner membungkus berkas ini dalam satu transaksi; jangan tulis BEGIN/COMMIT.

CREATE TABLE channel_orders (
    id                CHAR(26)    PRIMARY KEY,
    tenant_id         CHAR(26)    NOT NULL,
    channel_id        CHAR(26)    NOT NULL,
    sale_id           CHAR(26)    NOT NULL,
    external_order_id TEXT        NOT NULL,
    external_status   TEXT,
    buyer_name        TEXT,
    buyer_phone       TEXT,
    shipping_address  TEXT,
    courier           TEXT,
    tracking_no       TEXT,
    driver_name       TEXT,
    accepted_at       TIMESTAMPTZ,
    ready_at          TIMESTAMPTZ,
    completed_at      TIMESTAMPTZ,
    raw_payload       JSONB,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, channel_id, external_order_id),
    FOREIGN KEY (tenant_id, channel_id) REFERENCES channels (tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, sale_id)    REFERENCES sales    (tenant_id, id) ON DELETE CASCADE
);
CREATE INDEX idx_channel_orders_tenant_channel ON channel_orders (tenant_id, channel_id);

ALTER TABLE channel_orders ENABLE ROW LEVEL SECURITY;
ALTER TABLE channel_orders FORCE  ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON channel_orders
    USING      (COALESCE(current_setting('app.tenant_id', true), '') = '' OR tenant_id = current_setting('app.tenant_id', true))
    WITH CHECK (COALESCE(current_setting('app.tenant_id', true), '') = '' OR tenant_id = current_setting('app.tenant_id', true));
