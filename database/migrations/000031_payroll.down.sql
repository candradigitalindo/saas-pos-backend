-- Membatalkan 000031.
ALTER TABLE cash_movements
    DROP COLUMN IF EXISTS ref_table,
    DROP COLUMN IF EXISTS ref_id;
-- Catatan: shift_id dibiarkan NULLABLE — mengembalikannya ke NOT NULL bisa gagal
-- bila sudah ada baris gaji/kasbon ber-shift_id NULL. Aman: kolom nullable
-- tidak melanggar apa pun.

DROP TABLE IF EXISTS advance_repayments;
DROP TABLE IF EXISTS employee_advances;
DROP TABLE IF EXISTS payroll_adjustments;
DROP TABLE IF EXISTS payslip_lines;
DROP TABLE IF EXISTS payslips;
DROP TABLE IF EXISTS payroll_periods;
DROP TABLE IF EXISTS payroll_rules;
