-- Migrasi 000002 — refresh token (Fase 0).
--
-- Menutup utang teknis "tidak ada logout / pencabutan token" (CONVENTIONS §7):
-- access token berumur pendek (15 mnt), refresh token berumur panjang (30 hari)
-- yang bisa dicabut dan dirotasi. Yang disimpan adalah HASH token, bukan token
-- mentahnya — bila tabel bocor, token tetap tidak bisa dipakai (§9, §5.3).
--
-- Runner membungkus berkas ini dalam satu transaksi; jangan tulis BEGIN/COMMIT.

CREATE TABLE refresh_tokens (
    id          CHAR(26)    PRIMARY KEY,
    user_id     CHAR(26)    NOT NULL,
    token_hash  TEXT        NOT NULL,   -- SHA-256 hex dari token mentah
    device_name TEXT,                   -- label perangkat, untuk daftar "sesi aktif"
    expires_at  TIMESTAMPTZ NOT NULL,
    revoked_at  TIMESTAMPTZ,            -- non-NULL = sudah dicabut / sudah dirotasi
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT fk_refresh_tokens_user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
);

-- Pencarian token saat refresh selalu lewat hash-nya: wajib unik & terindeks.
CREATE UNIQUE INDEX uq_refresh_tokens_token_hash ON refresh_tokens (token_hash);

-- Partial index: hanya token aktif yang relevan untuk pencabutan massal
-- ("cabut semua sesi user X") dan penghitungan sesi.
CREATE INDEX idx_refresh_tokens_user ON refresh_tokens (user_id) WHERE revoked_at IS NULL;
