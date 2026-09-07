-- Migrasi 000005 — master data (Fase 2, docs/TECHNICAL-BACKEND.md §5.4).
--
-- Kategori, satuan, produk (+ varian, daftar harga, harga bertingkat), resep
-- F&B, supplier, meja. Semua tabel bertenant:
--   - UNIQUE (tenant_id, id) sebagai target FK komposit
--   - setiap FK antar tabel bertenant BERBENTUK KOMPOSIT (tenant_id, x_id)
--   - sync_version dari sequence yang dibuat di 000001 (trigger bump menyusul
--     Fase 6; untuk sekarang hanya nilai awal saat insert)
--   - Row Level Security dipasang di berkas ini juga (lapisan 2, §6)
--
-- price_lists.channel_id: kolom saja; FK ke channels menyusul Fase 11a (§5.16).
--
-- Runner membungkus berkas ini dalam satu transaksi; jangan tulis BEGIN/COMMIT.

-- ── categories ──────────────────────────────────────────────────────────────
CREATE TABLE categories (
    id           CHAR(26)    PRIMARY KEY,
    tenant_id    CHAR(26)    NOT NULL,
    parent_id    CHAR(26),                 -- maksimal 2 tingkat, dijaga di service
    name         TEXT        NOT NULL,
    sort_order   INT         NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at   TIMESTAMPTZ,
    sync_version BIGINT      NOT NULL DEFAULT nextval('sync_version_seq'),
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id)            REFERENCES tenants (id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, parent_id) REFERENCES categories (tenant_id, id) ON DELETE RESTRICT
);
CREATE INDEX idx_categories_tenant ON categories (tenant_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_categories_tenant_parent ON categories (tenant_id, parent_id);
CREATE UNIQUE INDEX uq_categories_tenant_name
    ON categories (tenant_id, COALESCE(parent_id, ''), name) WHERE deleted_at IS NULL;

-- ── units ───────────────────────────────────────────────────────────────────
CREATE TABLE units (
    id            CHAR(26)      PRIMARY KEY,
    tenant_id     CHAR(26)      NOT NULL,
    name          TEXT          NOT NULL,          -- 'pcs', 'dus', 'kg'
    base_unit_id  CHAR(26),                        -- NULL bila ini satuan dasar
    conversion    NUMERIC(14,6) NOT NULL DEFAULT 1, -- 1 dus = 24 pcs → 24
    allow_decimal BOOLEAN       NOT NULL DEFAULT false,
    created_at    TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ   NOT NULL DEFAULT now(),
    deleted_at    TIMESTAMPTZ,
    sync_version  BIGINT        NOT NULL DEFAULT nextval('sync_version_seq'),
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id)              REFERENCES tenants (id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, base_unit_id) REFERENCES units (tenant_id, id) ON DELETE RESTRICT
);
CREATE INDEX idx_units_tenant ON units (tenant_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_units_tenant_base ON units (tenant_id, base_unit_id);
CREATE UNIQUE INDEX uq_units_tenant_name ON units (tenant_id, name) WHERE deleted_at IS NULL;

-- ── products ────────────────────────────────────────────────────────────────
CREATE TABLE products (
    id           CHAR(26)      PRIMARY KEY,
    tenant_id    CHAR(26)      NOT NULL,
    category_id  CHAR(26),
    unit_id      CHAR(26)      NOT NULL,
    name         TEXT          NOT NULL,
    sku          TEXT,
    barcode      TEXT,
    sell_price   BIGINT        NOT NULL DEFAULT 0,
    cost_price   BIGINT        NOT NULL DEFAULT 0,   -- wajib diisi agar laba bisa dihitung
    track_stock  BOOLEAN       NOT NULL DEFAULT true,
    min_stock    NUMERIC(14,3) NOT NULL DEFAULT 0,
    is_active    BOOLEAN       NOT NULL DEFAULT true,
    image_url    TEXT,
    created_at   TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ   NOT NULL DEFAULT now(),
    deleted_at   TIMESTAMPTZ,
    sync_version BIGINT        NOT NULL DEFAULT nextval('sync_version_seq'),
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id)               REFERENCES tenants (id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, category_id)  REFERENCES categories (tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, unit_id)      REFERENCES units (tenant_id, id) ON DELETE RESTRICT
);
CREATE UNIQUE INDEX uq_products_tenant_sku
    ON products (tenant_id, sku) WHERE deleted_at IS NULL AND sku IS NOT NULL;
CREATE UNIQUE INDEX uq_products_tenant_barcode
    ON products (tenant_id, barcode) WHERE deleted_at IS NULL AND barcode IS NOT NULL;
CREATE INDEX idx_products_tenant_active ON products (tenant_id, is_active) WHERE deleted_at IS NULL;
CREATE INDEX idx_products_tenant_category ON products (tenant_id, category_id);

-- ── product_variants ────────────────────────────────────────────────────────
CREATE TABLE product_variants (
    id           CHAR(26)    PRIMARY KEY,
    tenant_id    CHAR(26)    NOT NULL,
    product_id   CHAR(26)    NOT NULL,
    name         TEXT        NOT NULL,            -- 'Besar', 'Pedas'
    sku          TEXT,
    barcode      TEXT,
    price_delta  BIGINT      NOT NULL DEFAULT 0,  -- selisih terhadap harga produk
    is_active    BOOLEAN     NOT NULL DEFAULT true,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at   TIMESTAMPTZ,
    sync_version BIGINT      NOT NULL DEFAULT nextval('sync_version_seq'),
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, product_id) REFERENCES products (tenant_id, id) ON DELETE CASCADE
);
CREATE INDEX idx_product_variants_tenant_product ON product_variants (tenant_id, product_id);

-- ── price_lists ─────────────────────────────────────────────────────────────
CREATE TABLE price_lists (
    id           CHAR(26)    PRIMARY KEY,
    tenant_id    CHAR(26)    NOT NULL,
    name         TEXT        NOT NULL,
    kind         TEXT        NOT NULL CHECK (kind IN ('retail','wholesale','member','channel')),
    channel_id   CHAR(26),                        -- diisi bila kind='channel' (FK menyusul Fase 11a)
    is_default   BOOLEAN     NOT NULL DEFAULT false,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at   TIMESTAMPTZ,
    sync_version BIGINT      NOT NULL DEFAULT nextval('sync_version_seq'),
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE RESTRICT
);
CREATE INDEX idx_price_lists_tenant ON price_lists (tenant_id) WHERE deleted_at IS NULL;
-- hanya satu daftar harga default per tenant
CREATE UNIQUE INDEX uq_price_lists_tenant_default
    ON price_lists (tenant_id) WHERE is_default AND deleted_at IS NULL;

-- ── product_prices ──────────────────────────────────────────────────────────
CREATE TABLE product_prices (
    id            CHAR(26)      PRIMARY KEY,
    tenant_id     CHAR(26)      NOT NULL,
    product_id    CHAR(26)      NOT NULL,
    variant_id    CHAR(26),
    price_list_id CHAR(26)      NOT NULL,
    min_qty       NUMERIC(14,3) NOT NULL DEFAULT 1,  -- harga bertingkat per jumlah
    price         BIGINT        NOT NULL,
    created_at    TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ   NOT NULL DEFAULT now(),
    sync_version  BIGINT        NOT NULL DEFAULT nextval('sync_version_seq'),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, product_id, variant_id, price_list_id, min_qty),
    FOREIGN KEY (tenant_id, product_id)    REFERENCES products (tenant_id, id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, variant_id)    REFERENCES product_variants (tenant_id, id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, price_list_id) REFERENCES price_lists (tenant_id, id) ON DELETE CASCADE
);
CREATE INDEX idx_product_prices_tenant_product ON product_prices (tenant_id, product_id);
CREATE INDEX idx_product_prices_tenant_list ON product_prices (tenant_id, price_list_id);

-- ── recipes / recipe_items ──────────────────────────────────────────────────
CREATE TABLE recipes (
    id         CHAR(26)      PRIMARY KEY,
    tenant_id  CHAR(26)      NOT NULL,
    product_id CHAR(26)      NOT NULL,          -- menu jadi
    yield_qty  NUMERIC(14,3) NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ   NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, product_id),
    FOREIGN KEY (tenant_id, product_id) REFERENCES products (tenant_id, id) ON DELETE CASCADE
);

CREATE TABLE recipe_items (
    id                    CHAR(26)      PRIMARY KEY,
    tenant_id             CHAR(26)      NOT NULL,
    recipe_id             CHAR(26)      NOT NULL,
    ingredient_product_id CHAR(26)      NOT NULL,
    qty                   NUMERIC(14,3) NOT NULL,
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, recipe_id)             REFERENCES recipes (tenant_id, id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, ingredient_product_id) REFERENCES products (tenant_id, id) ON DELETE RESTRICT
);
CREATE INDEX idx_recipe_items_tenant_recipe ON recipe_items (tenant_id, recipe_id);

-- ── suppliers ───────────────────────────────────────────────────────────────
CREATE TABLE suppliers (
    id           CHAR(26)    PRIMARY KEY,
    tenant_id    CHAR(26)    NOT NULL,
    name         TEXT        NOT NULL,
    phone        TEXT,
    address      TEXT,
    note         TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at   TIMESTAMPTZ,
    sync_version BIGINT      NOT NULL DEFAULT nextval('sync_version_seq'),
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE RESTRICT
);
CREATE INDEX idx_suppliers_tenant ON suppliers (tenant_id) WHERE deleted_at IS NULL;

-- ── dining_tables ───────────────────────────────────────────────────────────
CREATE TABLE dining_tables (
    id           CHAR(26)    PRIMARY KEY,
    tenant_id    CHAR(26)    NOT NULL,
    outlet_id    CHAR(26)    NOT NULL,
    name         TEXT        NOT NULL,
    capacity     INT,
    status       TEXT        NOT NULL DEFAULT 'free' CHECK (status IN ('free','occupied','reserved')),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at   TIMESTAMPTZ,
    sync_version BIGINT      NOT NULL DEFAULT nextval('sync_version_seq'),
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, outlet_id) REFERENCES outlets (tenant_id, id) ON DELETE RESTRICT
);
CREATE UNIQUE INDEX uq_dining_tables_tenant_outlet_name
    ON dining_tables (tenant_id, outlet_id, name) WHERE deleted_at IS NULL;

-- ── Row Level Security (lapisan 2) ──────────────────────────────────────────
-- Bentuk kebijakan identik dengan migrasi 000004: permisif saat GUC belum
-- disetel, memaksa cocok saat disetel di dalam repositories.WithTenant.
DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY[
    'categories','units','products','product_variants','price_lists',
    'product_prices','recipes','recipe_items','suppliers','dining_tables'
  ] LOOP
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('ALTER TABLE %I FORCE  ROW LEVEL SECURITY', t);
    EXECUTE format($f$
      CREATE POLICY tenant_isolation ON %I
        USING      (COALESCE(current_setting('app.tenant_id', true), '') = '' OR tenant_id = current_setting('app.tenant_id', true))
        WITH CHECK (COALESCE(current_setting('app.tenant_id', true), '') = '' OR tenant_id = current_setting('app.tenant_id', true))
    $f$, t);
  END LOOP;
END $$;
