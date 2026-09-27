-- Membatalkan 000042. Kredensial terenkripsi (kolom lama) tidak disentuh.
DROP INDEX IF EXISTS ux_channels_webhook_token;
ALTER TABLE channels DROP CONSTRAINT IF EXISTS channels_connection_status_check;
ALTER TABLE channels
    DROP COLUMN IF EXISTS connection_error,
    DROP COLUMN IF EXISTS connection_checked_at,
    DROP COLUMN IF EXISTS connection_status,
    DROP COLUMN IF EXISTS webhook_token;
