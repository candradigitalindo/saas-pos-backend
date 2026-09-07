-- Migrasi 000003 — multi-tenancy & otorisasi granular (Fase 1).
--
-- Menambah entitas inti isolasi tenant (docs/TECHNICAL-BACKEND.md §5.2–§5.3):
--   tenants, outlets, kolom tenant_id di users/roles, katalog permissions,
--   role_permissions, user_outlets.
--
-- Aturan yang ditegakkan di sini:
--   - Setiap FK antar tabel bertenant berbentuk KOMPOSIT (tenant_id, x_id) →
--     relasi lintas tenant ditolak database, bukan sekadar dicegah kode (§5.1).
--   - Setiap tabel bertenant punya UNIQUE (tenant_id, id) agar bisa jadi target
--     FK komposit.
--   - deleted_at pada users/roles sudah ada sejak migrasi 000001.
--
-- Runner membungkus berkas ini dalam satu transaksi; jangan tulis BEGIN/COMMIT.

-- ────────────────────────────────────────────────────────────────────────────
-- tenants — akar semua data operasional. Bukan tabel "bertenant": tidak punya
-- kolom tenant_id, aksesnya dibatasi RLS berbasis id (migrasi 000004).
-- ────────────────────────────────────────────────────────────────────────────
CREATE TABLE tenants (
    id            CHAR(26)    PRIMARY KEY,
    business_name TEXT        NOT NULL,
    business_type TEXT        NOT NULL
                  CHECK (business_type IN ('retail','fnb','service','wholesale','other')),
    owner_name    TEXT        NOT NULL,
    phone         TEXT        NOT NULL,
    email         TEXT,
    npwp          TEXT,
    nib           TEXT,
    status        TEXT        NOT NULL DEFAULT 'trial'
                  CHECK (status IN ('trial','active','past_due','suspended','closed')),
    -- FK ke partners menyusul di Fase 12 (§5.16); kolomnya dibuat sekarang.
    referred_by_partner_id CHAR(26),
    referral_code_used     TEXT,   -- diisi sekali saat daftar, tidak boleh berubah
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at    TIMESTAMPTZ
);

-- ────────────────────────────────────────────────────────────────────────────
-- outlets — cabang/gerai. Zona waktu ADA DI SINI, bukan di tenant: satu usaha
-- bisa punya cabang di Makassar dan Jayapura (§5.2).
-- ────────────────────────────────────────────────────────────────────────────
CREATE TABLE outlets (
    id                  CHAR(26)     PRIMARY KEY,
    tenant_id           CHAR(26)     NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    name                TEXT         NOT NULL,
    type                TEXT         NOT NULL DEFAULT 'store'
                        CHECK (type IN ('store','kitchen','warehouse','vehicle')),
    address             TEXT,
    phone               TEXT,
    timezone            TEXT         NOT NULL DEFAULT 'Asia/Jakarta',  -- nama IANA; divalidasi di service
    business_day_start  TIME         NOT NULL DEFAULT '00:00',         -- batas hari usaha
    currency            TEXT         NOT NULL DEFAULT 'IDR',
    tax_enabled         BOOLEAN      NOT NULL DEFAULT false,
    tax_rate            NUMERIC(7,4) NOT NULL DEFAULT 0,               -- pecahan, bukan persen
    tax_inclusive       BOOLEAN      NOT NULL DEFAULT true,
    service_charge_rate NUMERIC(7,4) NOT NULL DEFAULT 0,
    receipt_header      TEXT,
    receipt_footer      TEXT,
    is_active           BOOLEAN      NOT NULL DEFAULT true,
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ  NOT NULL DEFAULT now(),
    deleted_at          TIMESTAMPTZ,
    UNIQUE (tenant_id, id)   -- target FK komposit
);
CREATE INDEX idx_outlets_tenant ON outlets (tenant_id) WHERE deleted_at IS NULL;

-- ────────────────────────────────────────────────────────────────────────────
-- users / roles — tambah dimensi tenant.
-- ────────────────────────────────────────────────────────────────────────────
ALTER TABLE users
    ADD COLUMN tenant_id     CHAR(26) REFERENCES tenants(id) ON DELETE RESTRICT,
    ADD COLUMN pin_hash      TEXT,                        -- PIN ganti kasir cepat, bcrypt
    ADD COLUMN is_active     BOOLEAN NOT NULL DEFAULT true,
    ADD COLUMN last_login_at TIMESTAMPTZ;
ALTER TABLE users ADD CONSTRAINT uq_users_tenant_id UNIQUE (tenant_id, id);
CREATE INDEX idx_users_tenant ON users (tenant_id) WHERE deleted_at IS NULL;

ALTER TABLE roles
    ADD COLUMN tenant_id   CHAR(26) REFERENCES tenants(id) ON DELETE CASCADE,  -- NULL = peran bawaan sistem
    ADD COLUMN description TEXT,
    ADD COLUMN is_system   BOOLEAN NOT NULL DEFAULT false;   -- true = tidak boleh dihapus/ganti nama oleh tenant
ALTER TABLE roles ADD CONSTRAINT uq_roles_tenant_id UNIQUE (tenant_id, id);

-- Nama role unik per tenant (COALESCE menyatukan NULL agar peran sistem juga
-- unik di antara sesamanya). WHERE deleted_at IS NULL: mengikuti pola partial
-- unique §3.5 — role terhapus tidak memblokir nama yang sama.
DROP INDEX uq_roles_name;
CREATE UNIQUE INDEX uq_roles_tenant_name
    ON roles (COALESCE(tenant_id, ''), name) WHERE deleted_at IS NULL;

-- ────────────────────────────────────────────────────────────────────────────
-- permissions — katalog global (diisi seeder). Bukan tabel bertenant.
-- ────────────────────────────────────────────────────────────────────────────
CREATE TABLE permissions (
    id          CHAR(26) PRIMARY KEY,
    code        TEXT NOT NULL UNIQUE,   -- 'sale.void', 'report.view', ...
    group_name  TEXT NOT NULL,          -- pengelompokan untuk UI pengaturan peran
    description TEXT NOT NULL
);

CREATE TABLE role_permissions (
    role_id       CHAR(26) NOT NULL REFERENCES roles(id)       ON DELETE CASCADE,
    permission_id CHAR(26) NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);
-- FK di sisi permission_id butuh index tersendiri (PK hanya menutupi role_id).
CREATE INDEX idx_role_permissions_permission ON role_permissions (permission_id);

-- ────────────────────────────────────────────────────────────────────────────
-- user_outlets — akses user ke beberapa outlet. Tabel penghubung murni: PK-nya
-- pasangan kunci, tanpa kolom id. FK komposit menutup relasi lintas tenant.
-- ────────────────────────────────────────────────────────────────────────────
CREATE TABLE user_outlets (
    tenant_id CHAR(26) NOT NULL,
    user_id   CHAR(26) NOT NULL,
    outlet_id CHAR(26) NOT NULL,
    PRIMARY KEY (user_id, outlet_id),
    FOREIGN KEY (tenant_id, user_id)   REFERENCES users   (tenant_id, id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, outlet_id) REFERENCES outlets (tenant_id, id) ON DELETE CASCADE
);
CREATE INDEX idx_user_outlets_tenant_outlet ON user_outlets (tenant_id, outlet_id);
