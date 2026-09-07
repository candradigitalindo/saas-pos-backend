package repositories

import (
	"context"
	"errors"
	"fmt"

	"candra/backend-api/models"

	"gorm.io/gorm"
)

// Repositori dokumen CRM — penawaran, proyek, invoice pelanggan (§5.9,
// blueprint E.2). Semua ber-owner_id → kena scopeVisibility (lapis 3).

var (
	ErrQuotationNotFound = errors.New("penawaran tidak ditemukan")
	ErrProjectNotFound   = errors.New("proyek tidak ditemukan")
	ErrInvoiceNotFound   = errors.New("invoice tidak ditemukan")
	ErrNoOutlet          = errors.New("tenant belum punya outlet")
)

// NextDocumentNumber mengambil nomor dokumen berikutnya untuk (tenant, jenis),
// mengunci baris penghitung SELECT ... FOR UPDATE lalu menaikkannya. Format
// `<prefix>-000001`. Mirip NextReceiptSeq (§13.7): bukan COUNT(*).
func NextDocumentNumber(ctx context.Context, tx *gorm.DB, docType, prefix string) (string, error) {
	tid := currentTenantID(ctx)

	var c models.CRMDocumentCounter
	err := tx.WithContext(ctx).
		Clauses(lockForUpdate()).
		Where("tenant_id = ? AND doc_type = ?", tid, docType).
		First(&c).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		c = models.CRMDocumentCounter{TenantID: tid, DocType: docType, NextSeq: 2}
		if err := tx.WithContext(ctx).Create(&c).Error; err != nil {
			return "", err
		}
		return fmt.Sprintf("%s-%06d", prefix, 1), nil
	}
	if err != nil {
		return "", err
	}

	seq := c.NextSeq
	if err := tx.WithContext(ctx).Model(&models.CRMDocumentCounter{}).
		Where("tenant_id = ? AND doc_type = ?", tid, docType).
		UpdateColumn("next_seq", seq+1).Error; err != nil {
		return "", err
	}
	return fmt.Sprintf("%s-%06d", prefix, seq), nil
}

// FirstOutletID mengembalikan id outlet pertama tenant — dipakai sebagai outlet
// pencatatan penjualan sintetis saat invoice lunas (invoice CRM tak punya
// outlet). Outlet aktif diutamakan; bila tak ada yang aktif, tetap ambil yang
// pertama supaya pencatatan penjualan tidak gagal.
func FirstOutletID(ctx context.Context, tx *gorm.DB) (string, error) {
	var id string
	err := scopeTenant(ctx, tenantDB(ctx, tx).Model(&models.Outlet{})).
		Order("is_active DESC, created_at").
		Limit(1).
		Pluck("id", &id).Error
	if err != nil {
		return "", err
	}
	if id == "" {
		return "", ErrNoOutlet
	}
	return id, nil
}

// ── Quotation ─────────────────────────────────────────────────────────────

// QuotationFilter menyaring daftar penawaran.
type QuotationFilter struct {
	CustomerID string
	Status     string
}

func quotationScope(ctx context.Context) *gorm.DB {
	return scopeVisibility(ctx, scopeTenant(ctx, tenantDB(ctx, nil).Model(&models.Quotation{})), "quotations")
}

// ListQuotations mengembalikan satu halaman penawaran yang terlihat user konteks.
func ListQuotations(ctx context.Context, f QuotationFilter, limit, offset int) ([]models.Quotation, int64, error) {
	build := func() *gorm.DB {
		q := quotationScope(ctx)
		if f.CustomerID != "" {
			q = q.Where("customer_id = ?", f.CustomerID)
		}
		if f.Status != "" {
			q = q.Where("status = ?", f.Status)
		}
		return q
	}
	var total int64
	if err := build().Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []models.Quotation
	err := build().Order("created_at DESC").Limit(limit).Offset(offset).Find(&rows).Error
	return rows, total, err
}

// FindQuotation memuat satu penawaran (+ item) yang terlihat user konteks.
func FindQuotation(ctx context.Context, tx *gorm.DB, id string) (models.Quotation, error) {
	var q models.Quotation
	err := scopeVisibility(ctx, scopeTenant(ctx, tenantDB(ctx, tx)), "quotations").
		Preload("Items").
		First(&q, "quotations.id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return q, ErrQuotationNotFound
	}
	return q, err
}

// CreateQuotation menyimpan penawaran + itemnya (tenant_id di-stempel).
func CreateQuotation(ctx context.Context, tx *gorm.DB, q *models.Quotation) error {
	tid := currentTenantID(ctx)
	q.TenantID = tid
	for i := range q.Items {
		q.Items[i].TenantID = tid
	}
	return tx.WithContext(ctx).Create(q).Error
}

// SaveQuotation menyimpan kolom status/akseptasi penawaran.
func SaveQuotation(ctx context.Context, tx *gorm.DB, q *models.Quotation) error {
	res := scopeTenant(ctx, tenantDB(ctx, tx)).Model(&models.Quotation{}).
		Where("id = ?", q.ID).
		Updates(map[string]any{
			"status":      q.Status,
			"accepted_at": q.AcceptedAt,
			"note":        q.Note,
			"updated_at":  gorm.Expr("now()"),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrQuotationNotFound
	}
	return nil
}

// ── Project ───────────────────────────────────────────────────────────────

// ProjectFilter menyaring daftar proyek.
type ProjectFilter struct {
	CustomerID string
	Status     string
}

// ListProjects mengembalikan satu halaman proyek yang terlihat user konteks.
func ListProjects(ctx context.Context, f ProjectFilter, limit, offset int) ([]models.Project, int64, error) {
	build := func() *gorm.DB {
		q := scopeVisibility(ctx, scopeTenant(ctx, tenantDB(ctx, nil).Model(&models.Project{})), "projects")
		if f.CustomerID != "" {
			q = q.Where("customer_id = ?", f.CustomerID)
		}
		if f.Status != "" {
			q = q.Where("status = ?", f.Status)
		}
		return q
	}
	var total int64
	if err := build().Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []models.Project
	err := build().Order("created_at DESC").Limit(limit).Offset(offset).Find(&rows).Error
	return rows, total, err
}

// FindProject memuat satu proyek (+ tugas & biaya) yang terlihat user konteks.
func FindProject(ctx context.Context, tx *gorm.DB, id string) (models.Project, error) {
	var p models.Project
	err := scopeVisibility(ctx, scopeTenant(ctx, tenantDB(ctx, tx)), "projects").
		Preload("Tasks", func(db *gorm.DB) *gorm.DB { return db.Order("project_tasks.sort_order") }).
		Preload("Expenses").
		First(&p, "projects.id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return p, ErrProjectNotFound
	}
	return p, err
}

// CreateProject menyimpan proyek baru.
func CreateProject(ctx context.Context, tx *gorm.DB, p *models.Project) error {
	p.TenantID = currentTenantID(ctx)
	return tx.WithContext(ctx).Create(p).Error
}

// FindProjectByQuotation memuat proyek yang lahir dari sebuah penawaran, bila ada.
func FindProjectByQuotation(ctx context.Context, tx *gorm.DB, quotationID string) (models.Project, bool, error) {
	var p models.Project
	err := scopeTenant(ctx, tenantDB(ctx, tx)).First(&p, "quotation_id = ?", quotationID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return p, false, nil
	}
	if err != nil {
		return p, false, err
	}
	return p, true, nil
}

// SaveProject menyimpan kolom proyek yang berubah.
func SaveProject(ctx context.Context, tx *gorm.DB, p *models.Project) error {
	res := scopeTenant(ctx, tenantDB(ctx, tx)).Model(&models.Project{}).
		Where("id = ?", p.ID).
		Updates(map[string]any{
			"name":           p.Name,
			"start_date":     p.StartDate,
			"due_date":       p.DueDate,
			"status":         p.Status,
			"contract_value": p.ContractValue,
			"updated_at":     gorm.Expr("now()"),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrProjectNotFound
	}
	return nil
}

// CreateProjectTask & CreateProjectExpense menyimpan sub-baris proyek.
func CreateProjectTask(ctx context.Context, tx *gorm.DB, t *models.ProjectTask) error {
	t.TenantID = currentTenantID(ctx)
	return tx.WithContext(ctx).Create(t).Error
}

func CreateProjectExpense(ctx context.Context, tx *gorm.DB, e *models.ProjectExpense) error {
	e.TenantID = currentTenantID(ctx)
	return tx.WithContext(ctx).Create(e).Error
}

// SumProjectExpenses menjumlahkan biaya sebuah proyek (→ cost_total penjualan
// saat invoice proyek lunas).
func SumProjectExpenses(ctx context.Context, tx *gorm.DB, projectID string) (int64, error) {
	var total int64
	err := scopeTenant(ctx, tenantDB(ctx, tx).Model(&models.ProjectExpense{})).
		Where("project_id = ?", projectID).
		Select("COALESCE(SUM(amount), 0)").
		Scan(&total).Error
	return total, err
}

// ── Invoice ───────────────────────────────────────────────────────────────

// InvoiceFilter menyaring daftar invoice.
type InvoiceFilter struct {
	CustomerID string
	ProjectID  string
	Status     string
}

// ListInvoices mengembalikan satu halaman invoice yang terlihat user konteks.
func ListInvoices(ctx context.Context, f InvoiceFilter, limit, offset int) ([]models.Invoice, int64, error) {
	build := func() *gorm.DB {
		q := scopeVisibility(ctx, scopeTenant(ctx, tenantDB(ctx, nil).Model(&models.Invoice{})), "invoices")
		if f.CustomerID != "" {
			q = q.Where("customer_id = ?", f.CustomerID)
		}
		if f.ProjectID != "" {
			q = q.Where("project_id = ?", f.ProjectID)
		}
		if f.Status != "" {
			q = q.Where("status = ?", f.Status)
		}
		return q
	}
	var total int64
	if err := build().Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []models.Invoice
	err := build().Order("created_at DESC").Limit(limit).Offset(offset).Find(&rows).Error
	return rows, total, err
}

// FindInvoice memuat satu invoice (+ item & pembayaran) yang terlihat user konteks.
func FindInvoice(ctx context.Context, tx *gorm.DB, id string) (models.Invoice, error) {
	var in models.Invoice
	err := scopeVisibility(ctx, scopeTenant(ctx, tenantDB(ctx, tx)), "invoices").
		Preload("Items").
		Preload("Payments").
		First(&in, "invoices.id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return in, ErrInvoiceNotFound
	}
	return in, err
}

// CreateInvoice menyimpan invoice + itemnya (tenant_id di-stempel).
func CreateInvoice(ctx context.Context, tx *gorm.DB, in *models.Invoice) error {
	tid := currentTenantID(ctx)
	in.TenantID = tid
	for i := range in.Items {
		in.Items[i].TenantID = tid
	}
	return tx.WithContext(ctx).Create(in).Error
}

// SaveInvoice menyimpan kolom invoice yang berubah (status, paid, sale_id).
func SaveInvoice(ctx context.Context, tx *gorm.DB, in *models.Invoice) error {
	res := scopeTenant(ctx, tenantDB(ctx, tx)).Model(&models.Invoice{}).
		Where("id = ?", in.ID).
		Updates(map[string]any{
			"status":      in.Status,
			"paid_amount": in.PaidAmount,
			"sale_id":     in.SaleID,
			"updated_at":  gorm.Expr("now()"),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrInvoiceNotFound
	}
	return nil
}

// CreateInvoicePayment menyimpan satu pembayaran bertahap.
func CreateInvoicePayment(ctx context.Context, tx *gorm.DB, p *models.InvoicePayment) error {
	p.TenantID = currentTenantID(ctx)
	return tx.WithContext(ctx).Create(p).Error
}
