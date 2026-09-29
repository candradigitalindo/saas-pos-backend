-- Migrasi 000025 — kanal: rekonsiliasi pencairan & antrean sinkron stok
-- (Fase 11b, §5.10, blueprint F.4/F.8).
--
-- `channel_settlements`: cocokkan nilai pesanan sebuah periode dengan uang yang
-- benar-benar masuk rekening (selalu lebih kecil — komisi, biaya layanan,
-- subsidi ongkir). Selisih harus terlihat.
--
-- `channel_stock_syncs`: antrean permintaan pembaruan stok ke kanal. Adaptor
-- API yang mengirimnya sungguhan menyusul per kemitraan; antrean & indikator
-- keterlambatan / antrean mati bisa dilihat pemilik sekarang.
--
-- Runner membungkus berkas ini dalam satu transaksi; jangan tulis BEGIN/COMMIT.

CREATE TABLE channel_settlements (
    id              CHAR(26)    PRIMARY KEY,
    tenant_id       CHAR(26)    NOT NULL,
    channel_id      CHAR(26)    NOT NULL,
    period_start    DATE        NOT NULL,
    period_end      DATE        NOT NULL,
    gross_amount    BIGINT      NOT NULL DEFAULT 0,
    fee_amount      BIGINT      NOT NULL DEFAULT 0,
    net_amount      BIGINT      NOT NULL DEFAULT 0,
    received_amount BIGINT,
    status          TEXT        NOT NULL DEFAULT 'open'
                    CHECK (status IN ('open','matched','mismatch','closed')),
    received_at     TIMESTAMPTZ,
    note            TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, channel_id, period_start),
    FOREIGN KEY (tenant_id, channel_id) REFERENCES channels (tenant_id, id) ON DELETE RESTRICT
);

CREATE TABLE channel_stock_syncs (
    id            CHAR(26)      PRIMARY KEY,
    tenant_id     CHAR(26)      NOT NULL,
    channel_id    CHAR(26)      NOT NULL,
    product_id    CHAR(26)      NOT NULL,
    requested_qty NUMERIC(14,3) NOT NULL,
    status        TEXT          NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','sent','failed')),
    attempts      INT           NOT NULL DEFAULT 0,
    last_error    TEXT,
    queued_at     TIMESTAMPTZ   NOT NULL DEFAULT now(),
    sent_at       TIMESTAMPTZ,
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, channel_id) REFERENCES channels (tenant_id, id) ON DELETE CASCADE
);
CREATE INDEX idx_channel_stock_syncs_pending ON channel_stock_syncs (status, queued_at) WHERE status <> 'sent';

DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['channel_settlements','channel_stock_syncs'] LOOP
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('ALTER TABLE %I FORCE  ROW LEVEL SECURITY', t);
    EXECUTE format($f$
      CREATE POLICY tenant_isolation ON %I
        USING      (COALESCE(current_setting('app.tenant_id', true), '') = '' OR tenant_id = current_setting('app.tenant_id', true))
        WITH CHECK (COALESCE(current_setting('app.tenant_id', true), '') = '' OR tenant_id = current_setting('app.tenant_id', true))
    $f$, t);
  END LOOP;
END $$;
