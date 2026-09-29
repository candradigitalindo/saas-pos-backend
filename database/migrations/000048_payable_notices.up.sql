-- Migrasi 000048 — pengingat utang pemasok (cmd/payable-reminders).
--
-- Pekerjaan harian mengirim SATU ringkasan WhatsApp ke pemilik saat ada nota
-- pembelian yang BARU masuk masa "jatuh tempo sebentar lagi" atau BARU lewat
-- jatuh tempo — bukan setiap hari selama utangnya ada (itu derau yang membuat
-- pesan berikutnya diabaikan). Tiap (pembelian, jenis) dicatat sekali di sini,
-- di transaksi yang SAMA dengan pesannya di outbox; UNIQUE-nya yang membuat
-- pekerjaan ini aman dijalankan ulang atau dua instans bersamaan.

CREATE TABLE payable_notices (
    id          CHAR(26)    PRIMARY KEY,
    tenant_id   CHAR(26)    NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    purchase_id CHAR(26)    NOT NULL,
    kind        TEXT        NOT NULL CHECK (kind IN ('due_soon', 'overdue')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, purchase_id, kind),
    FOREIGN KEY (tenant_id, purchase_id) REFERENCES purchases (tenant_id, id) ON DELETE CASCADE
);

ALTER TABLE payable_notices ENABLE ROW LEVEL SECURITY;
ALTER TABLE payable_notices FORCE  ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON payable_notices
    USING      (COALESCE(current_setting('app.tenant_id', true), '') = '' OR tenant_id = current_setting('app.tenant_id', true))
    WITH CHECK (COALESCE(current_setting('app.tenant_id', true), '') = '' OR tenant_id = current_setting('app.tenant_id', true));
