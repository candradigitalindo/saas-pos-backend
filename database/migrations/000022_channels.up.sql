-- Migrasi 000022 — kanal pesanan online: fondasi (Fase 11a, §5.10,
-- blueprint Bagian F).
--
-- 11a TIDAK menyentuh API kanal mana pun. Yang dibuat: definisi kanal,
-- pemetaan SKU, dan (di 000023) metadata pesanan per kanal. Adaptor asinkron
-- + `channel_events`/`channel_settlements`/`channel_stock_syncs` menyusul 11b.
--
-- `sales` sudah channel-ready sejak Fase 3: kolom `channel_id`,
-- `external_order_id`, status pesanan diperluas, dan
-- `uq_sales_channel_external` — tidak ada perubahan `sales` di sini.
--
-- `credentials_encrypted` disiapkan kolomnya tapi belum dipakai di 11a
-- (kredensial baru relevan saat adaptor API 11b).
--
-- Runner membungkus berkas ini dalam satu transaksi; jangan tulis BEGIN/COMMIT.

CREATE TABLE channels (
    id                    CHAR(26)     PRIMARY KEY,
    tenant_id             CHAR(26)     NOT NULL,
    outlet_id             CHAR(26)     NOT NULL,
    kind                  TEXT         NOT NULL CHECK (kind IN ('pos','marketplace','delivery_app','conversation')),
    provider              TEXT         NOT NULL,   -- 'gofood','shopee','whatsapp',...
    name                  TEXT         NOT NULL,
    merchant_ref          TEXT,
    credentials_encrypted BYTEA,
    commission_rate       NUMERIC(7,4) NOT NULL DEFAULT 0,   -- verifikasi ke perjanjian
    price_list_id         CHAR(26),
    integration_mode      TEXT         NOT NULL DEFAULT 'manual'
                          CHECK (integration_mode IN ('manual','csv','api')),
    is_active             BOOLEAN      NOT NULL DEFAULT true,
    created_at            TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ  NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, outlet_id, provider),
    FOREIGN KEY (tenant_id, outlet_id)     REFERENCES outlets     (tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, price_list_id) REFERENCES price_lists (tenant_id, id) ON DELETE SET NULL
);
CREATE INDEX idx_channels_tenant_outlet ON channels (tenant_id, outlet_id);

CREATE TABLE channel_products (
    id                  CHAR(26)      PRIMARY KEY,
    tenant_id           CHAR(26)      NOT NULL,
    channel_id          CHAR(26)      NOT NULL,
    product_id          CHAR(26)      NOT NULL,
    variant_id          CHAR(26),
    external_sku        TEXT          NOT NULL,   -- SKU di kanal, hampir tak pernah sama
    external_product_id TEXT,
    channel_price       BIGINT,                   -- bila berbeda dari daftar harga
    is_available        BOOLEAN       NOT NULL DEFAULT true,
    stock_buffer        NUMERIC(14,3) NOT NULL DEFAULT 0,   -- penyangga anti-overselling
    last_synced_at      TIMESTAMPTZ,
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, channel_id, external_sku),
    FOREIGN KEY (tenant_id, channel_id) REFERENCES channels (tenant_id, id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, product_id) REFERENCES products (tenant_id, id) ON DELETE RESTRICT
);
CREATE INDEX idx_channel_products_tenant_channel ON channel_products (tenant_id, channel_id);
CREATE INDEX idx_channel_products_tenant_product ON channel_products (tenant_id, product_id);

DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['channels','channel_products'] LOOP
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('ALTER TABLE %I FORCE  ROW LEVEL SECURITY', t);
    EXECUTE format($f$
      CREATE POLICY tenant_isolation ON %I
        USING      (COALESCE(current_setting('app.tenant_id', true), '') = '' OR tenant_id = current_setting('app.tenant_id', true))
        WITH CHECK (COALESCE(current_setting('app.tenant_id', true), '') = '' OR tenant_id = current_setting('app.tenant_id', true))
    $f$, t);
  END LOOP;
END $$;
