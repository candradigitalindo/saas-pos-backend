-- Migrasi 000014 — sinkronisasi offline (Fase 6, §10).
--
-- Menambahkan mesin penanda kemajuan sinkronisasi:
--   1. pemicu `bump_sync_version()` — setiap INSERT/UPDATE pada tabel yang
--      disinkronkan menaikkan `sync_version` dari satu sequence global. Kenapa
--      bukan `updated_at`: jam bisa mundur dan dua penulisan di milidetik yang
--      sama tak bisa dibedakan; sequence selalu naik (§10). Kolom `sync_version`
--      sendiri sudah ada di semua tabel target sejak migrasi pembuatannya —
--      selama ini hanya terisi lewat DEFAULT saat INSERT, tidak naik saat UPDATE;
--   2. tabel batu nisan `sync_tombstones` + pemicu AFTER DELETE — baris yang
--      DIHAPUS KERAS tetap terkabar ke klien lewat pull agar salinan lokalnya
--      ikut dihapus. (Baris SOFT-delete tidak butuh ini: mengisi `deleted_at`
--      adalah UPDATE, jadi `sync_version`-nya ikut naik dan pull tetap
--      mengembalikannya — klien melihatnya lewat daftar `deleted`.)
--
-- Runner membungkus berkas ini dalam satu transaksi; jangan tulis BEGIN/COMMIT.

-- 1. Fungsi pemicu penanda kemajuan.
CREATE OR REPLACE FUNCTION bump_sync_version() RETURNS trigger AS $fn$
BEGIN
    NEW.sync_version := nextval('sync_version_seq');
    RETURN NEW;
END;
$fn$ LANGUAGE plpgsql;

-- 2. Batu nisan hard-delete. PK BIGINT identity (log internal, bukan entitas
--    domain — sejalan dengan `schema_migrations` yang juga ber-PK BIGINT).
CREATE TABLE sync_tombstones (
    id           BIGINT      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id    CHAR(26)    NOT NULL,
    table_name   TEXT        NOT NULL,
    row_id       CHAR(26)    NOT NULL,
    sync_version BIGINT      NOT NULL DEFAULT nextval('sync_version_seq'),
    deleted_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_sync_tombstones_tenant_ver ON sync_tombstones (tenant_id, sync_version);

ALTER TABLE sync_tombstones ENABLE ROW LEVEL SECURITY;
ALTER TABLE sync_tombstones FORCE  ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON sync_tombstones
    USING      (COALESCE(current_setting('app.tenant_id', true), '') = '' OR tenant_id = current_setting('app.tenant_id', true))
    WITH CHECK (COALESCE(current_setting('app.tenant_id', true), '') = '' OR tenant_id = current_setting('app.tenant_id', true));

-- 3. Fungsi pemicu batu nisan.
CREATE OR REPLACE FUNCTION record_sync_tombstone() RETURNS trigger AS $fn$
BEGIN
    INSERT INTO sync_tombstones (tenant_id, table_name, row_id)
    VALUES (OLD.tenant_id, TG_TABLE_NAME, OLD.id);
    RETURN OLD;
END;
$fn$ LANGUAGE plpgsql;

-- 4. Pasang kedua pemicu pada setiap tabel yang punya kolom `sync_version`.
DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY[
    'categories','units','products','product_variants','price_lists','product_prices',
    'suppliers','dining_tables','customers','stock_movements','shifts','cash_movements',
    'sales','purchases'
  ] LOOP
    EXECUTE format('DROP TRIGGER IF EXISTS trg_%1$s_sync_version ON %1$I', t);
    EXECUTE format(
      'CREATE TRIGGER trg_%1$s_sync_version BEFORE INSERT OR UPDATE ON %1$I '
      'FOR EACH ROW EXECUTE FUNCTION bump_sync_version()', t);

    EXECUTE format('DROP TRIGGER IF EXISTS trg_%1$s_tombstone ON %1$I', t);
    EXECUTE format(
      'CREATE TRIGGER trg_%1$s_tombstone AFTER DELETE ON %1$I '
      'FOR EACH ROW EXECUTE FUNCTION record_sync_tombstone()', t);
  END LOOP;
END $$;
