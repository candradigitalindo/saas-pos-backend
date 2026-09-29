-- Membatalkan 000049. Retur yang sudah tercatat hilang catatannya; total nota,
-- paid_amount, dan stok yang sudah berubah TIDAK dikembalikan. Gerakan stok
-- kind='purchase_return' yang sudah ada membuat constraint lama gagal dipasang
-- — hapus/ubah dulu bila benar-benar harus turun.
DROP TABLE IF EXISTS purchase_return_items;
DROP TABLE IF EXISTS purchase_returns;
ALTER TABLE stock_movements DROP CONSTRAINT stock_movements_kind_check;
ALTER TABLE stock_movements ADD CONSTRAINT stock_movements_kind_check CHECK (kind IN (
    'sale', 'void', 'refund', 'purchase', 'adjustment', 'transfer_in', 'transfer_out',
    'opname', 'recipe', 'initial'));
ALTER TABLE purchases DROP COLUMN IF EXISTS returned_amount;
