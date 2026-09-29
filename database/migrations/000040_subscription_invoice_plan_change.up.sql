-- Migrasi 000040 — ganti paket berlaku SETELAH dibayar.
--
-- Sampai migrasi ini, POST /subscription/change-plan langsung memindahkan
-- langganan ke paket baru lalu menerbitkan tagihannya. Sejak kunci paket
-- ditegakkan, itu berarti tenant Basic bisa pindah ke Multi-Outlet, memakai
-- fiturnya, dan tidak pernah membayar. Masa paket barunya pun baru dimulai
-- setelah periode lama habis, padahal sisa bulan lama sudah dikreditkan —
-- bulan-bulan itu terpakai dua kali.
--
-- Kini tagihan membawa paket yang DIBAYARNYA (plan_id) dan jenisnya: tagihan
-- 'plan_change' baru memindahkan paket saat lunas, masa berbayarnya mulai hari
-- pelunasan, dan kredit sisa paket lama (credit_amount, dari
-- credit_from_invoice_id) baru diperhitungkan pada tagihan lama saat itu juga.
--
-- Runner membungkus berkas ini dalam satu transaksi; jangan tulis BEGIN/COMMIT.

ALTER TABLE subscription_invoices
    ADD COLUMN plan_id                CHAR(26) REFERENCES plans (id) ON DELETE RESTRICT,
    ADD COLUMN kind                   TEXT     NOT NULL DEFAULT 'regular'
                                               CHECK (kind IN ('regular', 'plan_change')),
    ADD COLUMN credit_amount          BIGINT   NOT NULL DEFAULT 0 CHECK (credit_amount >= 0),
    ADD COLUMN credit_from_invoice_id CHAR(26) REFERENCES subscription_invoices (id) ON DELETE RESTRICT;

-- Tagihan lama: paketnya = paket langganan saat ini (satu-satunya petunjuk
-- yang ada; tagihan ganti paket lama sudah memindahkan langganan saat terbit).
UPDATE subscription_invoices i
   SET plan_id = s.plan_id
  FROM subscriptions s
 WHERE s.id = i.subscription_id AND i.plan_id IS NULL;

CREATE INDEX idx_sub_invoices_plan        ON subscription_invoices (plan_id);
CREATE INDEX idx_sub_invoices_credit_from ON subscription_invoices (credit_from_invoice_id);
