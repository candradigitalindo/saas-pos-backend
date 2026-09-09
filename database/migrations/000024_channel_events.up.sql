-- Migrasi 000024 — kanal: kotak masuk peristiwa & rincian biaya (Fase 11b,
-- §5.10, blueprint F.6).
--
-- Arsitektur: webhook/polling kanal → `channel_events` (MENTAH, belum diproses)
-- → pekerja asinkron → adaptor kanal (antarmuka seragam) → sales + stok.
-- Webhook TIDAK memproses langsung; ia hanya menaruh baris lalu balas 200.
--
-- `UNIQUE (tenant, channel, event_type, external_ref)` adalah pengaman
-- idempotensi: kanal yang mengirim peristiwa yang sama dua kali tetap satu
-- baris → satu penjualan.
--
-- Nomor 000024–000028 sempat sebelumnya dilewati Fase 13; dipakai sekarang
-- untuk 11b. Runner tidak menuntut nomor berurutan.
--
-- Runner membungkus berkas ini dalam satu transaksi; jangan tulis BEGIN/COMMIT.

CREATE TABLE channel_events (
    id           CHAR(26)     PRIMARY KEY,
    tenant_id    CHAR(26)     NOT NULL,
    channel_id   CHAR(26)     NOT NULL,
    event_type   TEXT         NOT NULL,
    external_ref TEXT,
    payload      JSONB        NOT NULL,
    status       TEXT         NOT NULL DEFAULT 'pending'
                 CHECK (status IN ('pending','processing','done','failed','dead')),
    attempts     INT          NOT NULL DEFAULT 0,
    last_error   TEXT,
    received_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    processed_at TIMESTAMPTZ,
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, channel_id, event_type, external_ref),
    FOREIGN KEY (tenant_id, channel_id) REFERENCES channels (tenant_id, id) ON DELETE CASCADE
);
CREATE INDEX idx_channel_events_pending ON channel_events (status, received_at) WHERE status IN ('pending','failed');

CREATE TABLE channel_fees (
    id        CHAR(26) PRIMARY KEY,
    tenant_id CHAR(26) NOT NULL,
    sale_id   CHAR(26) NOT NULL,
    kind      TEXT     NOT NULL CHECK (kind IN ('commission','service','shipping_subsidy','merchant_promo','tax','other')),
    amount    BIGINT   NOT NULL,   -- positif = potongan bagi kita
    note      TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, sale_id) REFERENCES sales (tenant_id, id) ON DELETE CASCADE
);
CREATE INDEX idx_channel_fees_tenant_sale ON channel_fees (tenant_id, sale_id);

-- Kunci ROUTING webhook: (provider, merchant_ref) wajib unik LINTAS tenant —
-- webhook tanpa auth memakainya untuk menautkan peristiwa ke tenant yang benar
-- (repositories.FindChannelByProviderRef). Indeks unik tetap dipaksakan di
-- level storage meski tabel channels ber-RLS FORCE. Channel tanpa merchant_ref
-- (integrasi manual/CSV) dikecualikan.
CREATE UNIQUE INDEX ux_channels_provider_merchant_ref
    ON channels (provider, merchant_ref)
    WHERE merchant_ref IS NOT NULL AND merchant_ref <> '';

DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['channel_events','channel_fees'] LOOP
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('ALTER TABLE %I FORCE  ROW LEVEL SECURITY', t);
    EXECUTE format($f$
      CREATE POLICY tenant_isolation ON %I
        USING      (COALESCE(current_setting('app.tenant_id', true), '') = '' OR tenant_id = current_setting('app.tenant_id', true))
        WITH CHECK (COALESCE(current_setting('app.tenant_id', true), '') = '' OR tenant_id = current_setting('app.tenant_id', true))
    $f$, t);
  END LOOP;
END $$;
