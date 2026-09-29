-- Migrasi 000038 — fitur paket `crm_sales` (sales lapangan: rencana
-- kunjungan, check-in, target, komisi).
--
-- Modul ini dulu terbuka untuk semua tenant, termasuk paket Gratis, padahal
-- pasarnya grosir/distributor dengan tim sales dan model bisnis menaruhnya
-- sebagai modul berbayar. Keputusan pemilik produk 2026-09-27: termasuk paket
-- Pro & Multi-Outlet, terkunci di Gratis & Basic (kunci paket hanya menutup
-- MEMULAI hal baru — lihat services/plan_entitlement_service.go).
--
-- Seeder hanya MENYISIPKAN paket yang belum ada (ON CONFLICT DO NOTHING), jadi
-- basis data yang sudah berjalan butuh migrasi ini. Hanya kunci yang BELUM ada
-- yang ditambahkan: nilai yang sudah diatur admin platform tidak ditimpa.
-- (Tanpa operator jsonb `?` supaya tidak disangka placeholder.)
--
-- Runner membungkus berkas ini dalam satu transaksi; jangan tulis BEGIN/COMMIT.

UPDATE plans
   SET features = features || '{"crm_sales": true}'::jsonb, updated_at = now()
 WHERE code IN ('pro', 'multi')
   AND features -> 'crm_sales' IS NULL;

UPDATE plans
   SET features = features || '{"crm_sales": false}'::jsonb, updated_at = now()
 WHERE code IN ('free', 'basic')
   AND features -> 'crm_sales' IS NULL;
