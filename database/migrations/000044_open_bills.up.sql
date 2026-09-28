-- Migrasi 000044 — tagihan terbuka (open bill / tahan transaksi).
--
-- Pesanan yang belum dibayar: "Meja 5", "Pak Budi". Dicatat pelayan atau
-- kasir, ditambah bertahap, lalu dibayar sekali di akhir. Tersimpan di server
-- supaya terlihat dari semua perangkat toko (keputusan pemilik produk
-- 2026-09-28: "bersama via server", label nama bebas — daftar meja menyusul).
--
-- Isi tagihan disimpan UTUH sebagai JSON (items): tagihan selalu diubah
-- sebagai satu dokumen dari keranjang kasir, bukan per baris, dan HARGA tidak
-- disimpan — harga selalu dihitung server saat checkout (§13.1).
--
-- `version` menjaga dua perangkat yang mengubah tagihan yang sama: setiap
-- perubahan menyebut versi yang ia baca (base_version); yang basi ditolak
-- (409) alih-alih diam-diam menimpa pesanan perangkat lain. `last_op_id`
-- membuat operasi dari antrean offline idempoten: dikirim ulang = duplikat.
--
-- Tagihan TIDAK pernah dihapus: dibayar → status 'paid' + sale_id, dibatalkan
-- → 'canceled' + siapa yang membatalkan (jejak audit: pesanan yang lenyap
-- sebelum dibayar adalah pola kecurangan kasir yang klasik).
--
-- Runner membungkus berkas ini dalam satu transaksi; jangan tulis BEGIN/COMMIT.

CREATE TABLE open_bills (
    id                 CHAR(26)    PRIMARY KEY,               -- ULID dibuat KLIEN (offline)
    tenant_id          CHAR(26)    NOT NULL REFERENCES tenants (id) ON DELETE RESTRICT,
    outlet_id          CHAR(26)    NOT NULL,
    label              TEXT        NOT NULL CHECK (length(btrim(label)) BETWEEN 1 AND 60),
    customer_id        CHAR(26),
    order_type         TEXT        NOT NULL DEFAULT 'dine_in'
                                   CHECK (order_type IN ('dine_in', 'takeaway', 'delivery', 'pickup')),
    items              JSONB       NOT NULL DEFAULT '[]'::jsonb,
    order_discount     JSONB,                                 -- {"kind":"nominal|percent","value":n}
    note               TEXT        NOT NULL DEFAULT '',
    status             TEXT        NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'paid', 'canceled')),
    version            INT         NOT NULL DEFAULT 1,
    last_op_id         CHAR(26),
    sale_id            CHAR(26),
    created_by         CHAR(26)    NOT NULL,
    updated_by         CHAR(26)    NOT NULL,
    closed_by          CHAR(26),
    closed_at          TIMESTAMPTZ,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, outlet_id)   REFERENCES outlets   (tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, customer_id) REFERENCES customers (tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, sale_id)     REFERENCES sales     (tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, created_by)  REFERENCES users     (tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, updated_by)  REFERENCES users     (tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, closed_by)   REFERENCES users     (tenant_id, id) ON DELETE RESTRICT
);

-- Daftar tagihan terbuka per cabang (layar kasir), sekaligus indeks FK outlet.
CREATE INDEX idx_open_bills_outlet ON open_bills (tenant_id, outlet_id, status, updated_at DESC);
-- Satu penjualan menutup paling banyak satu tagihan.
CREATE UNIQUE INDEX uq_open_bills_sale ON open_bills (tenant_id, sale_id) WHERE sale_id IS NOT NULL;
-- FK diberi indeks (lihat 000033).
CREATE INDEX idx_open_bills_customer   ON open_bills (tenant_id, customer_id);
CREATE INDEX idx_open_bills_created_by ON open_bills (tenant_id, created_by);
CREATE INDEX idx_open_bills_updated_by ON open_bills (tenant_id, updated_by);
CREATE INDEX idx_open_bills_closed_by  ON open_bills (tenant_id, closed_by);

ALTER TABLE open_bills ENABLE ROW LEVEL SECURITY;
ALTER TABLE open_bills FORCE  ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON open_bills
    USING      (COALESCE(current_setting('app.tenant_id', true), '') = '' OR tenant_id = current_setting('app.tenant_id', true))
    WITH CHECK (COALESCE(current_setting('app.tenant_id', true), '') = '' OR tenant_id = current_setting('app.tenant_id', true));
