package repositories

import (
	"context"
	"errors"

	"candra/backend-api/models"

	"gorm.io/gorm"
)

// Repositori CRM sales lapangan — rencana kunjungan, kunjungan, target, komisi
// (§5.9, blueprint E.3). `visits`/`visit_plans` ber-owner_id → scopeVisibility
// (lapis 3). `sales_targets`/`commissions` di-scope lewat `user_id`
// (scopeOwnUnless "crm.commission.view").

var (
	ErrVisitPlanNotFound  = errors.New("rencana kunjungan tidak ditemukan")
	ErrVisitNotFound      = errors.New("kunjungan tidak ditemukan")
	ErrCommissionNotFound = errors.New("komisi tidak ditemukan")
)

// ── Visit plan ────────────────────────────────────────────────────────────

// FindVisitPlanByOwnerDate memuat rencana kunjungan (owner, tanggal), bila ada.
func FindVisitPlanByOwnerDate(ctx context.Context, tx *gorm.DB, ownerID, planDate string) (models.VisitPlan, bool, error) {
	var p models.VisitPlan
	err := scopeTenant(ctx, tenantDB(ctx, tx)).
		First(&p, "owner_id = ? AND plan_date = ?", ownerID, planDate).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return p, false, nil
	}
	if err != nil {
		return p, false, err
	}
	return p, true, nil
}

// CreateVisitPlan menyimpan rencana kunjungan baru.
func CreateVisitPlan(ctx context.Context, tx *gorm.DB, p *models.VisitPlan) error {
	p.TenantID = currentTenantID(ctx)
	return tx.WithContext(ctx).Create(p).Error
}

// FindVisitPlan memuat satu rencana (+ kunjungan) yang terlihat user konteks.
func FindVisitPlan(ctx context.Context, tx *gorm.DB, id string) (models.VisitPlan, error) {
	var p models.VisitPlan
	err := scopeVisibility(ctx, scopeTenant(ctx, tenantDB(ctx, tx)), "visit_plans").
		Preload("Visits").
		First(&p, "visit_plans.id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return p, ErrVisitPlanNotFound
	}
	return p, err
}

// ListVisitPlans mengembalikan rencana kunjungan yang terlihat user konteks.
func ListVisitPlans(ctx context.Context, planDate string, limit, offset int) ([]models.VisitPlan, int64, error) {
	build := func() *gorm.DB {
		q := scopeVisibility(ctx, scopeTenant(ctx, tenantDB(ctx, nil).Model(&models.VisitPlan{})), "visit_plans")
		if planDate != "" {
			q = q.Where("plan_date = ?", planDate)
		}
		return q
	}
	var total int64
	if err := build().Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []models.VisitPlan
	err := build().Order("plan_date DESC").Limit(limit).Offset(offset).Find(&rows).Error
	return rows, total, err
}

// SaveVisitPlanStatus memperbarui status rencana kunjungan.
func SaveVisitPlanStatus(ctx context.Context, tx *gorm.DB, id, status string) error {
	res := scopeTenant(ctx, tenantDB(ctx, tx)).Model(&models.VisitPlan{}).
		Where("id = ?", id).UpdateColumn("status", status)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrVisitPlanNotFound
	}
	return nil
}

// ── Visit ─────────────────────────────────────────────────────────────────

// CreateVisit menyimpan kunjungan baru.
func CreateVisit(ctx context.Context, tx *gorm.DB, v *models.Visit) error {
	v.TenantID = currentTenantID(ctx)
	return tx.WithContext(ctx).Create(v).Error
}

// CreateVisits menyimpan sekumpulan kunjungan (mis. saat membuat rencana).
func CreateVisits(ctx context.Context, tx *gorm.DB, rows []models.Visit) error {
	if len(rows) == 0 {
		return nil
	}
	tid := currentTenantID(ctx)
	for i := range rows {
		rows[i].TenantID = tid
	}
	return tx.WithContext(ctx).Create(&rows).Error
}

// FindVisit memuat satu kunjungan yang terlihat user konteks.
func FindVisit(ctx context.Context, tx *gorm.DB, id string) (models.Visit, error) {
	var v models.Visit
	err := scopeVisibility(ctx, scopeTenant(ctx, tenantDB(ctx, tx)), "visits").
		First(&v, "visits.id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return v, ErrVisitNotFound
	}
	return v, err
}

// FindVisitInTenant memuat satu kunjungan TANPA lapis visibilitas — dipakai
// jalur sync (pemilik kunjungan = perangkat pendorong, sudah terautentikasi).
func FindVisitInTenant(ctx context.Context, tx *gorm.DB, id string) (models.Visit, bool, error) {
	var v models.Visit
	err := scopeTenant(ctx, tenantDB(ctx, tx)).First(&v, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return v, false, nil
	}
	if err != nil {
		return v, false, err
	}
	return v, true, nil
}

// VisitFilter menyaring daftar kunjungan.
type VisitFilter struct {
	BusinessDate string
	Result       string
	CustomerID   string
	VisitPlanID  string
}

// ListVisits mengembalikan satu halaman kunjungan yang terlihat user konteks.
func ListVisits(ctx context.Context, f VisitFilter, limit, offset int) ([]models.Visit, int64, error) {
	build := func() *gorm.DB {
		q := scopeVisibility(ctx, scopeTenant(ctx, tenantDB(ctx, nil).Model(&models.Visit{})), "visits")
		if f.BusinessDate != "" {
			q = q.Where("business_date = ?", f.BusinessDate)
		}
		if f.Result != "" {
			q = q.Where("result = ?", f.Result)
		}
		if f.CustomerID != "" {
			q = q.Where("customer_id = ?", f.CustomerID)
		}
		if f.VisitPlanID != "" {
			q = q.Where("visit_plan_id = ?", f.VisitPlanID)
		}
		return q
	}
	var total int64
	if err := build().Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []models.Visit
	err := build().Order("COALESCE(checkin_at, created_at) DESC").Limit(limit).Offset(offset).Find(&rows).Error
	return rows, total, err
}

// SaveVisit menyimpan kolom kunjungan yang berubah.
func SaveVisit(ctx context.Context, tx *gorm.DB, v *models.Visit) error {
	res := scopeTenant(ctx, tenantDB(ctx, tx)).Model(&models.Visit{}).
		Where("id = ?", v.ID).
		Updates(map[string]any{
			"visit_plan_id":   v.VisitPlanID,
			"checkin_at":      v.CheckinAt,
			"checkout_at":     v.CheckoutAt,
			"checkin_lat":     v.CheckinLat,
			"checkin_lng":     v.CheckinLng,
			"photo_url":       v.PhotoURL,
			"result":          v.Result,
			"no_order_reason": v.NoOrderReason,
			"sale_id":         v.SaleID,
			"updated_at":      gorm.Expr("now()"),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrVisitNotFound
	}
	return nil
}

// CountVisits menghitung kunjungan seorang user pada rentang; doneOnly → hanya
// yang sudah check-out.
func CountVisits(ctx context.Context, tx *gorm.DB, userID, from, to string, doneOnly bool) (int64, error) {
	q := scopeTenant(ctx, tenantDB(ctx, tx).Model(&models.Visit{})).
		Where("owner_id = ? AND business_date BETWEEN ? AND ?", userID, from, to)
	if doneOnly {
		q = q.Where("checkout_at IS NOT NULL")
	}
	var n int64
	err := q.Count(&n).Error
	return n, err
}

// ── Sales target ──────────────────────────────────────────────────────────

// FindTargetByUserPeriod memuat target (user, awal periode), bila ada.
func FindTargetByUserPeriod(ctx context.Context, tx *gorm.DB, userID, periodStart string) (models.SalesTarget, bool, error) {
	var t models.SalesTarget
	err := scopeTenant(ctx, tenantDB(ctx, tx)).
		First(&t, "user_id = ? AND period_start = ?", userID, periodStart).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return t, false, nil
	}
	if err != nil {
		return t, false, err
	}
	return t, true, nil
}

// CreateTarget & SaveTarget.
func CreateTarget(ctx context.Context, tx *gorm.DB, t *models.SalesTarget) error {
	t.TenantID = currentTenantID(ctx)
	return tx.WithContext(ctx).Create(t).Error
}

func SaveTarget(ctx context.Context, tx *gorm.DB, t *models.SalesTarget) error {
	return scopeTenant(ctx, tenantDB(ctx, tx)).Model(&models.SalesTarget{}).
		Where("id = ?", t.ID).
		Updates(map[string]any{
			"period_end":    t.PeriodEnd,
			"target_amount": t.TargetAmount,
			"target_visits": t.TargetVisits,
			"updated_at":    gorm.Expr("now()"),
		}).Error
}

// ListTargets mengembalikan target yang terlihat user konteks (sendiri kecuali
// pemegang crm.commission.view).
func ListTargets(ctx context.Context, userID string, limit, offset int) ([]models.SalesTarget, int64, error) {
	build := func() *gorm.DB {
		q := scopeOwnUnless(ctx, scopeTenant(ctx, tenantDB(ctx, nil).Model(&models.SalesTarget{})),
			"user_id", "crm.commission.view")
		if userID != "" {
			q = q.Where("user_id = ?", userID)
		}
		return q
	}
	var total int64
	if err := build().Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []models.SalesTarget
	err := build().Order("period_start DESC").Limit(limit).Offset(offset).Find(&rows).Error
	return rows, total, err
}

// ── Commission ────────────────────────────────────────────────────────────

// FindCommissionByUserPeriod memuat komisi (user, awal periode), bila ada.
func FindCommissionByUserPeriod(ctx context.Context, tx *gorm.DB, userID, periodStart string) (models.Commission, bool, error) {
	var c models.Commission
	err := scopeTenant(ctx, tenantDB(ctx, tx)).
		First(&c, "user_id = ? AND period_start = ?", userID, periodStart).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return c, false, nil
	}
	if err != nil {
		return c, false, err
	}
	return c, true, nil
}

func CreateCommission(ctx context.Context, tx *gorm.DB, c *models.Commission) error {
	c.TenantID = currentTenantID(ctx)
	return tx.WithContext(ctx).Create(c).Error
}

func SaveCommission(ctx context.Context, tx *gorm.DB, c *models.Commission) error {
	res := scopeTenant(ctx, tenantDB(ctx, tx)).Model(&models.Commission{}).
		Where("id = ?", c.ID).
		Updates(map[string]any{
			"period_end":  c.PeriodEnd,
			"base_amount": c.BaseAmount,
			"rate":        c.Rate,
			"amount":      c.Amount,
			"status":      c.Status,
			"updated_at":  gorm.Expr("now()"),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrCommissionNotFound
	}
	return nil
}

// FindCommission memuat satu komisi yang terlihat user konteks.
func FindCommission(ctx context.Context, tx *gorm.DB, id string) (models.Commission, error) {
	var c models.Commission
	err := scopeOwnUnless(ctx, scopeTenant(ctx, tenantDB(ctx, tx)), "user_id", "crm.commission.view").
		First(&c, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return c, ErrCommissionNotFound
	}
	return c, err
}

// ListCommissions mengembalikan komisi yang terlihat user konteks.
func ListCommissions(ctx context.Context, userID, status string, limit, offset int) ([]models.Commission, int64, error) {
	build := func() *gorm.DB {
		q := scopeOwnUnless(ctx, scopeTenant(ctx, tenantDB(ctx, nil).Model(&models.Commission{})),
			"user_id", "crm.commission.view")
		if userID != "" {
			q = q.Where("user_id = ?", userID)
		}
		if status != "" {
			q = q.Where("status = ?", status)
		}
		return q
	}
	var total int64
	if err := build().Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []models.Commission
	err := build().Order("period_start DESC").Limit(limit).Offset(offset).Find(&rows).Error
	return rows, total, err
}

// CollectedByUser menghitung nilai TERTAGIH (uang benar-benar masuk) yang
// diatribusikan ke `userID` pada rentang [from, to]:
//
//	Σ sale_payments.amount (method != 'credit', sale.created_by = user,
//	                        sale.status='completed', sale.business_date ∈ rentang)
//	+ Σ receivable_payments.amount (collected_by = user, business_date ∈ rentang)
//
// Bukan nilai TERKIRIM: bagian kasbon dari penjualan kredit tidak dihitung
// sampai benar-benar disetor.
func CollectedByUser(ctx context.Context, tx *gorm.DB, userID, from, to string) (int64, error) {
	tid := currentTenantID(ctx)

	var fromSales int64
	if err := tenantDB(ctx, tx).
		Table("sale_payments sp").
		Joins("JOIN sales s ON s.id = sp.sale_id").
		Where("sp.tenant_id = ? AND sp.method <> 'credit' AND s.created_by = ? AND s.status = 'completed' AND s.business_date BETWEEN ? AND ?",
			tid, userID, from, to).
		Select("COALESCE(SUM(sp.amount), 0)").
		Scan(&fromSales).Error; err != nil {
		return 0, err
	}

	var fromReceivables int64
	if err := scopeTenant(ctx, tenantDB(ctx, tx).Model(&models.ReceivablePayment{})).
		Where("collected_by = ? AND business_date BETWEEN ? AND ?", userID, from, to).
		Select("COALESCE(SUM(amount), 0)").
		Scan(&fromReceivables).Error; err != nil {
		return 0, err
	}

	return fromSales + fromReceivables, nil
}
