-- Membatalkan 000003. Urutan kebalikan pembuatan; FK ON DELETE CASCADE ikut
-- membereskan baris anak, tapi tabel tetap di-drop eksplisit.
--
-- Runner membungkus berkas ini dalam satu transaksi; jangan tulis BEGIN/COMMIT.

DROP TABLE IF EXISTS user_outlets;
DROP TABLE IF EXISTS role_permissions;
DROP TABLE IF EXISTS permissions;

-- Kembalikan unique index roles ke bentuk Fase 0.
DROP INDEX IF EXISTS uq_roles_tenant_name;
CREATE UNIQUE INDEX uq_roles_name ON roles (name) WHERE deleted_at IS NULL;

ALTER TABLE roles
    DROP CONSTRAINT IF EXISTS uq_roles_tenant_id,
    DROP COLUMN IF EXISTS tenant_id,
    DROP COLUMN IF EXISTS description,
    DROP COLUMN IF EXISTS is_system;

DROP INDEX IF EXISTS idx_users_tenant;
ALTER TABLE users
    DROP CONSTRAINT IF EXISTS uq_users_tenant_id,
    DROP COLUMN IF EXISTS tenant_id,
    DROP COLUMN IF EXISTS pin_hash,
    DROP COLUMN IF EXISTS is_active,
    DROP COLUMN IF EXISTS last_login_at;

DROP TABLE IF EXISTS outlets;
DROP TABLE IF EXISTS tenants;
