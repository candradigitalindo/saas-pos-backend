-- Inisialisasi PostgreSQL untuk SaaS POS UMKM.
--
-- PENTING (docs/TECHNICAL-BACKEND.md §6): aplikasi WAJIB terhubung sebagai role
-- NON-SUPERUSER. Row Level Security (termasuk FORCE ROW LEVEL SECURITY di
-- migrasi) TIDAK berlaku untuk superuser — bila aplikasi login sebagai
-- `postgres`, seluruh isolasi antar-tenant lenyap tanpa error apa pun.
--
-- Skrip ini dijalankan sekali oleh entrypoint image `postgres` (file di
-- /docker-entrypoint-initdb.d/). Untuk deploy ke PostgreSQL yang dikelola
-- (RDS/Cloud SQL/dll), jalankan isinya sekali secara manual sebagai admin.
--
-- Password di sini HANYA untuk pengembangan lokal (compose). Di production,
-- buat role-nya terpisah dengan password kuat dan JANGAN commit.

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'pos_app') THEN
    CREATE ROLE pos_app LOGIN PASSWORD 'pos_app_dev' NOSUPERUSER NOCREATEROLE CREATEDB;
  END IF;
END
$$;

-- Basis data aplikasi & basis data test, dimiliki role non-superuser itu.
-- (CREATE DATABASE tidak bisa di dalam blok DO; jalankan lewat \gexec.)
SELECT 'CREATE DATABASE saas_pos       OWNER pos_app'
 WHERE NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = 'saas_pos')\gexec
SELECT 'CREATE DATABASE saas_pos_test  OWNER pos_app'
 WHERE NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = 'saas_pos_test')\gexec

-- Pastikan role bisa membuat objek di skema public tiap basis data
-- (PostgreSQL 15+ mencabut hak CREATE default pada public).
\connect saas_pos
GRANT ALL ON SCHEMA public TO pos_app;
ALTER SCHEMA public OWNER TO pos_app;

\connect saas_pos_test
GRANT ALL ON SCHEMA public TO pos_app;
ALTER SCHEMA public OWNER TO pos_app;
