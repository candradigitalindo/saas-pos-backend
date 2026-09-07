-- Migrasi 000004 — Row Level Security (Fase 1, lapisan 2 isolasi tenant §6).
--
-- Jaring pengaman bila sebuah query lolos dari lapisan 1 (scopeTenant). Kebijakan
-- membandingkan kolom tenant dengan GUC `app.tenant_id` yang disetel `SET LOCAL`
-- di dalam repositories.WithTenant.
--
-- Bentuk kebijakan — PERMISIF saat GUC belum disetel:
--
--   COALESCE(current_setting('app.tenant_id', true), '') = ''      -- GUC kosong → izinkan semua
--   OR <kolom tenant> = current_setting('app.tenant_id', true)     -- GUC terisi  → paksa cocok
--
-- Kenapa permisif saat kosong, bukan bentuk ketat di §6:
--   1. Bootstrap auth: middleware memuat user berdasarkan id SEBELUM tenant-nya
--      diketahui — query itu tidak mungkin menyetel GUC lebih dulu.
--   2. Jalur baca biasa tidak dibungkus transaksi (hemat round-trip); isolasinya
--      dijamin lapisan 1.
-- Akibatnya RLS efektif menjaga SEMUA jalur tulis (lewat WithTenant, GUC selalu
-- terisi) — persis jalur yang menyentuh uang & stok. Bisa diperketat nanti bila
-- seluruh jalur baca sudah transaksional.
--
-- FORCE diperlukan karena aplikasi konek sebagai pemilik tabel, dan pemilik
-- otomatis melewati RLS tanpa FORCE.
--
-- Runner membungkus berkas ini dalam satu transaksi; jangan tulis BEGIN/COMMIT.

-- tenants: dikunci berdasarkan id-nya sendiri (bukan tenant_id).
ALTER TABLE tenants ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenants FORCE  ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON tenants
    USING      (COALESCE(current_setting('app.tenant_id', true), '') = '' OR id = current_setting('app.tenant_id', true))
    WITH CHECK (COALESCE(current_setting('app.tenant_id', true), '') = '' OR id = current_setting('app.tenant_id', true));

-- outlets, users, roles, user_outlets: dikunci berdasarkan tenant_id.
ALTER TABLE outlets ENABLE ROW LEVEL SECURITY;
ALTER TABLE outlets FORCE  ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON outlets
    USING      (COALESCE(current_setting('app.tenant_id', true), '') = '' OR tenant_id = current_setting('app.tenant_id', true))
    WITH CHECK (COALESCE(current_setting('app.tenant_id', true), '') = '' OR tenant_id = current_setting('app.tenant_id', true));

ALTER TABLE users ENABLE ROW LEVEL SECURITY;
ALTER TABLE users FORCE  ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON users
    USING      (COALESCE(current_setting('app.tenant_id', true), '') = '' OR tenant_id = current_setting('app.tenant_id', true))
    WITH CHECK (COALESCE(current_setting('app.tenant_id', true), '') = '' OR tenant_id = current_setting('app.tenant_id', true));

ALTER TABLE roles ENABLE ROW LEVEL SECURITY;
ALTER TABLE roles FORCE  ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON roles
    USING      (COALESCE(current_setting('app.tenant_id', true), '') = '' OR tenant_id = current_setting('app.tenant_id', true))
    WITH CHECK (COALESCE(current_setting('app.tenant_id', true), '') = '' OR tenant_id = current_setting('app.tenant_id', true));

ALTER TABLE user_outlets ENABLE ROW LEVEL SECURITY;
ALTER TABLE user_outlets FORCE  ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON user_outlets
    USING      (COALESCE(current_setting('app.tenant_id', true), '') = '' OR tenant_id = current_setting('app.tenant_id', true))
    WITH CHECK (COALESCE(current_setting('app.tenant_id', true), '') = '' OR tenant_id = current_setting('app.tenant_id', true));
