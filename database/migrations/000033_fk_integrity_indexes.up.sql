-- Migrasi 000033 — keutuhan relasi & index sisi anak setiap foreign key
-- (§5.15 aturan 2, §5.17).
--
-- DUA hal yang diperbaiki:
--
-- 1. `users.role_id` sebelumnya FK TUNGGAL ke `roles(id)`. Keduanya tabel
--    BERTENANT, jadi §5.17 mewajibkan FK KOMPOSIT: tanpa itu, secara database
--    sebuah user tenant A masih bisa menunjuk peran milik tenant B. Lapisan
--    aplikasi sudah menjaganya (FindRoleInTenant), tetapi jaring pengaman
--    terakhir tetap harus ada di skema.
--
-- 2. 47 foreign key belum punya index di sisi anak. PostgreSQL tidak
--    membuatnya otomatis; tanpa index itu setiap ON DELETE RESTRICT/CASCADE
--    pada baris induk memindai SELURUH tabel anak. Semua index diawali
--    `tenant_id` untuk tabel bertenant (§5.15 aturan 1).
--
-- Runner membungkus berkas ini dalam satu transaksi; jangan tulis BEGIN/COMMIT.

-- ── 1. FK komposit users.role_id ─────────────────────────────────────────
-- Rapikan dulu bila ada user yang menunjuk peran lintas tenant (seharusnya
-- tidak ada; UPDATE ini membuat migrasi aman dijalankan di data lama).
UPDATE users u SET role_id = NULL
 WHERE role_id IS NOT NULL
   AND NOT EXISTS (SELECT 1 FROM roles r WHERE r.id = u.role_id AND r.tenant_id = u.tenant_id);

ALTER TABLE users DROP CONSTRAINT IF EXISTS fk_users_role;
ALTER TABLE users
  ADD CONSTRAINT fk_users_role
  FOREIGN KEY (tenant_id, role_id) REFERENCES roles (tenant_id, id) ON DELETE RESTRICT;

-- ── 2. Index sisi anak untuk setiap FK yang belum punya ──────────────────
CREATE INDEX IF NOT EXISTS idx_activities_customer
    ON activities (tenant_id,customer_id);
CREATE INDEX IF NOT EXISTS idx_activities_deal
    ON activities (tenant_id,deal_id);
CREATE INDEX IF NOT EXISTS idx_advance_repayments_payslip
    ON advance_repayments (tenant_id,payslip_id);
CREATE INDEX IF NOT EXISTS idx_channel_orders_sale
    ON channel_orders (tenant_id,sale_id);
CREATE INDEX IF NOT EXISTS idx_channel_stock_syncs_channel
    ON channel_stock_syncs (tenant_id,channel_id);
CREATE INDEX IF NOT EXISTS idx_channels_price_list
    ON channels (tenant_id,price_list_id);
CREATE INDEX IF NOT EXISTS idx_customers_price_list
    ON customers (tenant_id,price_list_id);
CREATE INDEX IF NOT EXISTS idx_deals_customer
    ON deals (tenant_id,customer_id);
CREATE INDEX IF NOT EXISTS idx_deals_lead_source
    ON deals (tenant_id,lead_source_id);
CREATE INDEX IF NOT EXISTS idx_deals_pipeline
    ON deals (tenant_id,pipeline_id);
CREATE INDEX IF NOT EXISTS idx_deferred_revenue_entries_tenant
    ON deferred_revenue_entries (tenant_id);
CREATE INDEX IF NOT EXISTS idx_employee_advances_cash_movement
    ON employee_advances (tenant_id,cash_movement_id);
CREATE INDEX IF NOT EXISTS idx_invoice_items_invoice
    ON invoice_items (tenant_id,invoice_id);
CREATE INDEX IF NOT EXISTS idx_invoice_items_product
    ON invoice_items (tenant_id,product_id);
CREATE INDEX IF NOT EXISTS idx_invoices_customer
    ON invoices (tenant_id,customer_id);
CREATE INDEX IF NOT EXISTS idx_invoices_project
    ON invoices (tenant_id,project_id);
CREATE INDEX IF NOT EXISTS idx_invoices_quotation
    ON invoices (tenant_id,quotation_id);
CREATE INDEX IF NOT EXISTS idx_invoices_sale
    ON invoices (tenant_id,sale_id);
CREATE INDEX IF NOT EXISTS idx_payroll_adjustments_origin_period
    ON payroll_adjustments (tenant_id,origin_period_id);
CREATE INDEX IF NOT EXISTS idx_payslip_lines_rule
    ON payslip_lines (tenant_id,rule_id);
CREATE INDEX IF NOT EXISTS idx_payslips_employee
    ON payslips (tenant_id,employee_id);
CREATE INDEX IF NOT EXISTS idx_product_prices_variant
    ON product_prices (tenant_id,variant_id);
CREATE INDEX IF NOT EXISTS idx_products_unit
    ON products (tenant_id,unit_id);
CREATE INDEX IF NOT EXISTS idx_project_expenses_project
    ON project_expenses (tenant_id,project_id);
CREATE INDEX IF NOT EXISTS idx_project_tasks_project
    ON project_tasks (tenant_id,project_id);
CREATE INDEX IF NOT EXISTS idx_projects_customer
    ON projects (tenant_id,customer_id);
CREATE INDEX IF NOT EXISTS idx_projects_quotation
    ON projects (tenant_id,quotation_id);
CREATE INDEX IF NOT EXISTS idx_quotation_items_product
    ON quotation_items (tenant_id,product_id);
CREATE INDEX IF NOT EXISTS idx_quotation_items_quotation
    ON quotation_items (tenant_id,quotation_id);
CREATE INDEX IF NOT EXISTS idx_quotations_customer
    ON quotations (tenant_id,customer_id);
CREATE INDEX IF NOT EXISTS idx_quotations_deal
    ON quotations (tenant_id,deal_id);
CREATE INDEX IF NOT EXISTS idx_recipe_items_ingredient_product
    ON recipe_items (tenant_id,ingredient_product_id);
CREATE INDEX IF NOT EXISTS idx_sale_items_variant
    ON sale_items (tenant_id,variant_id);
CREATE INDEX IF NOT EXISTS idx_sales_created_by
    ON sales (tenant_id,created_by);
CREATE INDEX IF NOT EXISTS idx_sales_customer
    ON sales (tenant_id,customer_id);
CREATE INDEX IF NOT EXISTS idx_shifts_opened_by
    ON shifts (tenant_id,opened_by);
CREATE INDEX IF NOT EXISTS idx_stock_movements_product
    ON stock_movements (tenant_id,product_id);
CREATE INDEX IF NOT EXISTS idx_stock_movements_variant
    ON stock_movements (tenant_id,variant_id);
CREATE INDEX IF NOT EXISTS idx_stock_opname_items_product
    ON stock_opname_items (tenant_id,product_id);
CREATE INDEX IF NOT EXISTS idx_stock_transfer_items_product
    ON stock_transfer_items (tenant_id,product_id);
CREATE INDEX IF NOT EXISTS idx_stocks_product
    ON stocks (tenant_id,product_id);
CREATE INDEX IF NOT EXISTS idx_subscription_invoices_subscription
    ON subscription_invoices (subscription_id);
CREATE INDEX IF NOT EXISTS idx_subscription_refunds_tenant
    ON subscription_refunds (tenant_id);
CREATE INDEX IF NOT EXISTS idx_subscriptions_plan
    ON subscriptions (plan_id);
CREATE INDEX IF NOT EXISTS idx_user_outlets_user
    ON user_outlets (tenant_id,user_id);
CREATE INDEX IF NOT EXISTS idx_visits_customer
    ON visits (tenant_id,customer_id);
CREATE INDEX IF NOT EXISTS idx_visits_sale
    ON visits (tenant_id,sale_id);

-- Index untuk FK komposit users.role_id yang baru dipasang di atas.
CREATE INDEX IF NOT EXISTS idx_users_role ON users (tenant_id, role_id);
