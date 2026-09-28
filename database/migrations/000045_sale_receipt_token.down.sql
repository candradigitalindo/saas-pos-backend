-- Membatalkan 000045. Tautan struk yang sudah dibagikan berhenti bisa dibuka.
DROP INDEX IF EXISTS uq_sales_receipt_token;
ALTER TABLE sales DROP COLUMN IF EXISTS receipt_token;
