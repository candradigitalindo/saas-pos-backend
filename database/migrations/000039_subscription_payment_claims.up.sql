-- Migrasi 000039 — konfirmasi pembayaran langganan (subscription_payment_claims).
--
-- Sampai migrasi ini, tagihan langganan ditandai LUNAS oleh pemilik toko
-- sendiri: POST /subscription-payments (izin billing.manage milik tenant)
-- langsung mencatat pembayaran dan mengaktifkan paket — tanpa uang yang
-- benar-benar diterima siapa pun. Sejak kunci paket ditegakkan, itu berarti
-- siapa pun bisa membuka paket berbayar tanpa membayar.
--
-- Kini tenant hanya MENGONFIRMASI pembayaran (berapa, lewat apa, nama pengirim
-- / nomor referensi). Konfirmasi menunggu diverifikasi staf keuangan platform
-- di panel internal; baru saat DISETUJUI pembayaran sungguhan dicatat
-- (subscription_payments) dan paket aktif. Ditolak → alasannya ditampilkan ke
-- tenant, yang boleh mengirim konfirmasi baru.
--
-- Runner membungkus berkas ini dalam satu transaksi; jangan tulis BEGIN/COMMIT.

CREATE TABLE subscription_payment_claims (
    id                      CHAR(26)    PRIMARY KEY,
    tenant_id               CHAR(26)    NOT NULL REFERENCES tenants (id) ON DELETE RESTRICT,
    subscription_invoice_id CHAR(26)    NOT NULL REFERENCES subscription_invoices (id) ON DELETE RESTRICT,
    amount                  BIGINT      NOT NULL CHECK (amount > 0),
    method                  TEXT        NOT NULL CHECK (method IN ('transfer', 'qris', 'ewallet', 'card', 'cash')),
    reference               TEXT        NOT NULL DEFAULT '',   -- nama pengirim / nomor referensi
    note                    TEXT        NOT NULL DEFAULT '',
    status                  TEXT        NOT NULL DEFAULT 'pending'
                                        CHECK (status IN ('pending', 'approved', 'rejected')),
    submitted_by            CHAR(26),
    reviewed_by             CHAR(26)    REFERENCES platform_admins (id) ON DELETE SET NULL,
    reviewed_at             TIMESTAMPTZ,
    reject_reason           TEXT        NOT NULL DEFAULT '',
    subscription_payment_id CHAR(26)    REFERENCES subscription_payments (id) ON DELETE SET NULL,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Pengirim konfirmasi harus pengguna tenant yang SAMA (FK komposit, pola
    -- tabel bertenant lain); bila penggunanya dihapus, hanya kolom ini yang
    -- dikosongkan, bukan tenant_id.
    FOREIGN KEY (tenant_id, submitted_by) REFERENCES users (tenant_id, id)
        ON DELETE SET NULL (submitted_by)
);

-- Satu konfirmasi MENUNGGU per tagihan: kiriman kedua sebelum yang pertama
-- diputus hanya menggandakan pekerjaan verifikasi (dan membuka peluang
-- disetujui dua kali).
CREATE UNIQUE INDEX uq_sub_claims_pending_invoice
    ON subscription_payment_claims (subscription_invoice_id) WHERE status = 'pending';

-- Antrean panel (status + urutan masuk) dan riwayat per tenant; FK diberi
-- indeks sesuai 000033.
CREATE INDEX idx_sub_claims_status  ON subscription_payment_claims (status, created_at);
CREATE INDEX idx_sub_claims_tenant  ON subscription_payment_claims (tenant_id, created_at DESC);
CREATE INDEX idx_sub_claims_invoice ON subscription_payment_claims (subscription_invoice_id);
CREATE INDEX idx_sub_claims_submitted_by ON subscription_payment_claims (submitted_by);
CREATE INDEX idx_sub_claims_reviewed_by  ON subscription_payment_claims (reviewed_by);
CREATE INDEX idx_sub_claims_payment      ON subscription_payment_claims (subscription_payment_id);

-- Row Level Security, sama seperti tabel bertenant lainnya (§6). Panel
-- internal membaca lintas tenant TANPA GUC — kebijakan permisif saat GUC
-- kosong (lihat catatan keputusan RLS).
ALTER TABLE subscription_payment_claims ENABLE ROW LEVEL SECURITY;
ALTER TABLE subscription_payment_claims FORCE  ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON subscription_payment_claims
    USING      (COALESCE(current_setting('app.tenant_id', true), '') = '' OR tenant_id = current_setting('app.tenant_id', true))
    WITH CHECK (COALESCE(current_setting('app.tenant_id', true), '') = '' OR tenant_id = current_setting('app.tenant_id', true));
