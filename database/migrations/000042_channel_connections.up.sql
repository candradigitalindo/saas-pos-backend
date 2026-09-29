-- Migrasi 000042 — sambungan API kanal milik TENANT.
--
-- Setiap tenant mendaftar sendiri ke penyedia (Meta/WhatsApp, GoBiz, Grab,
-- Shopee, ...) dan memegang kredensialnya sendiri; platform hanya
-- menyambungkan. Kredensial disimpan terenkripsi di channels.credentials_encrypted
-- (kolom sudah ada sejak 000022; lihat internal/rahasia).
--
-- webhook_token: bagian acak di alamat webhook PER KANAL
-- (/webhooks/channels/:provider/:token). Alamat per kanal membuat routing tidak
-- bergantung pada isi payload, dan tanda tangan webhook diverifikasi dengan
-- rahasia milik kanal itu — webhook lama berbasis (provider, merchant_ref)
-- tidak menandatangani apa pun.
--
-- connection_status: none | connected | error, dari "Tes koneksi" terakhir.
--
-- Runner membungkus berkas ini dalam satu transaksi; jangan tulis BEGIN/COMMIT.

ALTER TABLE channels
    ADD COLUMN webhook_token         TEXT,
    ADD COLUMN connection_status     TEXT        NOT NULL DEFAULT 'none',
    ADD COLUMN connection_checked_at TIMESTAMPTZ,
    ADD COLUMN connection_error      TEXT        NOT NULL DEFAULT '';

ALTER TABLE channels
    ADD CONSTRAINT channels_connection_status_check
    CHECK (connection_status IN ('none', 'connected', 'error'));

-- Dicari LINTAS tenant oleh webhook (seperti ux_channels_provider_merchant_ref).
CREATE UNIQUE INDEX ux_channels_webhook_token
    ON channels (webhook_token)
    WHERE webhook_token IS NOT NULL;
