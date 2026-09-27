-- Migrasi 000041 — perpanjangan & pengingat langganan, antrean pengembalian dana.
--
-- 1. subscription_notices: jejak pengingat yang SUDAH dikirim pekerjaan
--    harian cmd/subscription-renewals (masa coba hampir habis, tagihan lewat
--    jatuh tempo, masa tenggang hampir habis). UNIQUE (tenant_id, kind, ref)
--    membuat setiap pengingat terkirim sekali saja walau pekerjaannya jalan
--    tiap hari — atau dua kali bersamaan.
--
-- 2. subscription_refunds sampai kini hanya CATATAN hitungan pengembalian
--    saat berhenti; tidak ada yang memprosesnya, tidak ada rekening tujuan,
--    dan tenant tidak tahu uangnya sudah dikirim atau belum. Kini baris
--    ber-status: pending (menunggu ditransfer staf keuangan di panel) → paid;
--    not_needed untuk yang nilainya nol.
--
-- Tabel platform — tanpa RLS, sama seperti subscription_* lain (lihat 000015).
-- Runner membungkus berkas ini dalam satu transaksi; jangan tulis BEGIN/COMMIT.

CREATE TABLE subscription_notices (
    id         CHAR(26)    PRIMARY KEY,
    tenant_id  CHAR(26)    NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    kind       TEXT        NOT NULL CHECK (kind IN ('trial_ending', 'invoice_overdue', 'grace_ending')),
    ref        TEXT        NOT NULL,   -- id tagihan / tanggal berakhir yang diingatkan
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, kind, ref)
);

ALTER TABLE subscription_refunds
    ADD COLUMN status              TEXT     NOT NULL DEFAULT 'pending'
                                            CHECK (status IN ('pending', 'paid', 'not_needed')),
    ADD COLUMN destination_bank    TEXT     NOT NULL DEFAULT '',
    ADD COLUMN destination_account TEXT     NOT NULL DEFAULT '',
    ADD COLUMN destination_holder  TEXT     NOT NULL DEFAULT '',
    ADD COLUMN paid_at             TIMESTAMPTZ,
    ADD COLUMN paid_by             CHAR(26) REFERENCES platform_admins (id) ON DELETE SET NULL,
    ADD COLUMN payout_reference    TEXT     NOT NULL DEFAULT '';

-- Pengembalian bernilai nol tidak perlu diproses. Yang bernilai (dari sebelum
-- migrasi ini) tetap 'pending' supaya muncul di antrean — belum pernah ada
-- jalur yang mencairkannya.
UPDATE subscription_refunds SET status = 'not_needed' WHERE amount = 0;

-- Bug lama: memilih kartu Gratis di tengah masa coba menghasilkan "masa coba
-- paket Gratis" — layar tetap menampilkan masa coba dan tombol bayar, lalu
-- "Bayar sekarang" menerbitkan tagihan Rp0 yang tidak pernah bisa dibayar
-- (konfirmasi wajib > 0) dan menghalangi tagihan berikutnya. Kini memilih
-- Gratis = menghentikan masa coba. Rapikan data yang terlanjur:
--   a. tagihan Rp0 yang masih terbuka → void;
--   b. masa coba paket berharga 0 → dihentikan (tanpa uang yang terlibat;
--      trial_ends_at dipertahankan, jadi sisa masa coba bisa dilanjutkan
--      dengan memilih paket berbayar).
UPDATE subscription_invoices
   SET status = 'void', updated_at = now()
 WHERE status IN ('open', 'overdue') AND total_amount = 0 AND paid_amount = 0;

UPDATE subscriptions s
   SET status = 'canceled', canceled_at = now(), auto_renew = false,
       cancel_reason = 'Pindah ke paket Gratis', updated_at = now()
  FROM plans p
 WHERE p.id = s.plan_id AND p.monthly_price = 0 AND s.status = 'trial';

CREATE INDEX idx_sub_refunds_status  ON subscription_refunds (status, created_at);
CREATE INDEX idx_sub_refunds_paid_by ON subscription_refunds (paid_by);
