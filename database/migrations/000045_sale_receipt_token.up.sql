-- Migrasi 000045 — tautan struk digital.
--
-- Struk dibagikan ke pembeli lewat WhatsApp dari perangkat kasir (keputusan
-- pemilik produk 2026-09-28: pengirimnya nomor TOKO, bukan nomor platform),
-- berisi teks struk + tautan ke halaman struk publik. Tautan itu memakai token
-- acak (128 bit) — bukan id penjualan — supaya struk orang lain tidak bisa
-- ditebak dengan menggeser angka.
--
-- Token dibuat saat struk pertama kali dibagikan, bukan untuk setiap
-- penjualan: kebanyakan struk tidak pernah dibagikan. Unik GLOBAL karena
-- halaman publik mencarinya tanpa tahu tenant.
--
-- Runner membungkus berkas ini dalam satu transaksi; jangan tulis BEGIN/COMMIT.

ALTER TABLE sales ADD COLUMN receipt_token TEXT;
CREATE UNIQUE INDEX uq_sales_receipt_token ON sales (receipt_token) WHERE receipt_token IS NOT NULL;
