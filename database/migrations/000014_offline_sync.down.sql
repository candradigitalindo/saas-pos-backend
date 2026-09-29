-- Membatalkan 000014.
DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY[
    'categories','units','products','product_variants','price_lists','product_prices',
    'suppliers','dining_tables','customers','stock_movements','shifts','cash_movements',
    'sales','purchases'
  ] LOOP
    EXECUTE format('DROP TRIGGER IF EXISTS trg_%1$s_sync_version ON %1$I', t);
    EXECUTE format('DROP TRIGGER IF EXISTS trg_%1$s_tombstone ON %1$I', t);
  END LOOP;
END $$;

DROP FUNCTION IF EXISTS bump_sync_version();
DROP FUNCTION IF EXISTS record_sync_tombstone();
DROP TABLE IF EXISTS sync_tombstones;
