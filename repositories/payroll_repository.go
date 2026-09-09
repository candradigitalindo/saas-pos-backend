package repositories

import (
	"context"
	"errors"
	"time"

	"candra/backend-api/internal/reqctx"
	"candra/backend-api/internal/ulid"
	"candra/backend-api/models"

	"gorm.io/gorm"
)

// Repositori penggajian (§5.11, §13.6). Semua tulis lewat WithTenant.

var (
	ErrPayrollRuleNotFound   = errors.New("aturan gaji tidak ditemukan")
	ErrPayrollPeriodNotFound = errors.New("periode gaji tidak ditemukan")
	ErrPayslipNotFound       = errors.New("slip gaji tidak ditemukan")
	ErrAdvanceNotFound       = errors.New("kasbon tidak ditemukan")
)

// ── Payroll rule ──────────────────────────────────────────────────────────

func CreatePayrollRule(ctx context.Context, tx *gorm.DB, r *models.PayrollRule) error {
	r.TenantID = currentTenantID(ctx)
	return tenantDB(ctx, tx).Create(r).Error
}

func ListPayrollRules(ctx context.Context) ([]models.PayrollRule, error) {
	var rows []models.PayrollRule
	err := scopeTenant(ctx, tenantDB(ctx, nil)).Order("category, priority, code").Find(&rows).Error
	return rows, err
}

func FindPayrollRule(ctx context.Context, tx *gorm.DB, id string) (models.PayrollRule, error) {
	var r models.PayrollRule
	err := scopeTenant(ctx, tenantDB(ctx, tx)).First(&r, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return r, ErrPayrollRuleNotFound
	}
	return r, err
}

func SavePayrollRule(ctx context.Context, tx *gorm.DB, r *models.PayrollRule) error {
	return scopeTenant(ctx, tenantDB(ctx, tx)).Model(&models.PayrollRule{}).Where("id = ?", r.ID).
		Updates(map[string]any{
			"name": r.Name, "params": r.Params, "priority": r.Priority,
			"effective_to": r.EffectiveTo, "is_active": r.IsActive, "updated_at": gorm.Expr("now()"),
		}).Error
}

// EffectiveRulesForEmployee mengembalikan aturan aktif yang berlaku pada rentang
// periode & menyasar karyawan (target_type all, atau employee/role yang cocok),
// urut kategori lalu priority lalu code (deterministik).
func EffectiveRulesForEmployee(ctx context.Context, tx *gorm.DB, emp models.Employee, roleID string, start, end time.Time) ([]models.PayrollRule, error) {
	var rows []models.PayrollRule
	err := scopeTenant(ctx, tenantDB(ctx, tx)).
		Where("is_active = ? AND effective_from <= ? AND (effective_to IS NULL OR effective_to >= ?)", true, end, start).
		Where("target_type = 'all' OR (target_type = 'employee' AND target_id = ?) OR (target_type = 'role' AND target_id = ?)",
			emp.ID, roleID).
		Order("category, priority, code").
		Find(&rows).Error
	return rows, err
}

// ── Payroll period ────────────────────────────────────────────────────────

func CreatePayrollPeriod(ctx context.Context, tx *gorm.DB, p *models.PayrollPeriod) error {
	p.TenantID = currentTenantID(ctx)
	p.CreatedBy = reqctx.UserID(ctx)
	return tenantDB(ctx, tx).Create(p).Error
}

func ListPayrollPeriods(ctx context.Context, limit, offset int) ([]models.PayrollPeriod, int64, error) {
	q := scopeTenant(ctx, tenantDB(ctx, nil).Model(&models.PayrollPeriod{}))
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []models.PayrollPeriod
	err := q.Order("start_date DESC").Limit(limit).Offset(offset).Find(&rows).Error
	return rows, total, err
}

// LockPayrollPeriod memuat periode dengan penguncian baris (SELECT ... FOR UPDATE).
func LockPayrollPeriod(ctx context.Context, tx *gorm.DB, id string) (models.PayrollPeriod, error) {
	var p models.PayrollPeriod
	err := scopeTenant(ctx, tenantDB(ctx, tx)).Clauses(lockForUpdate()).First(&p, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return p, ErrPayrollPeriodNotFound
	}
	return p, err
}

func FindPayrollPeriod(ctx context.Context, tx *gorm.DB, id string) (models.PayrollPeriod, error) {
	var p models.PayrollPeriod
	err := scopeTenant(ctx, tenantDB(ctx, tx)).First(&p, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return p, ErrPayrollPeriodNotFound
	}
	return p, err
}

func SavePayrollPeriod(ctx context.Context, tx *gorm.DB, p *models.PayrollPeriod) error {
	return scopeTenant(ctx, tenantDB(ctx, tx)).Model(&models.PayrollPeriod{}).Where("id = ?", p.ID).
		Updates(map[string]any{
			"status": p.Status, "total_gross": p.TotalGross, "total_deduction": p.TotalDeduction,
			"total_net": p.TotalNet, "calculated_at": p.CalculatedAt, "locked_at": p.LockedAt,
			"locked_by": p.LockedBy, "pay_date": p.PayDate, "paid_at": p.PaidAt, "updated_at": gorm.Expr("now()"),
		}).Error
}

// LockedPeriodCovering mencari periode gaji locked/paid yang mencakup `date`
// (outlet spesifik atau semua outlet).
func LockedPeriodCovering(ctx context.Context, tx *gorm.DB, outletID string, date time.Time) (models.PayrollPeriod, bool, error) {
	var p models.PayrollPeriod
	err := scopeTenant(ctx, tenantDB(ctx, tx)).
		Where("status IN ('locked','paid') AND start_date <= ? AND end_date >= ? AND (outlet_id IS NULL OR outlet_id = ?)",
			date, date, outletID).
		Order("start_date DESC").First(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return p, false, nil
	}
	if err != nil {
		return p, false, err
	}
	return p, true, nil
}

// NextPeriodOnOrAfter mencari periode karyawan berikutnya (start_date > origin
// period start) yang belum terkunci — target default penyesuaian.
func NextOpenPeriodAfter(ctx context.Context, tx *gorm.DB, originStart time.Time, periodType string) (models.PayrollPeriod, bool, error) {
	var p models.PayrollPeriod
	err := scopeTenant(ctx, tenantDB(ctx, tx)).
		Where("period_type = ? AND start_date > ? AND status IN ('draft','calculated')", periodType, originStart).
		Order("start_date").
		First(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return p, false, nil
	}
	if err != nil {
		return p, false, err
	}
	return p, true, nil
}

// ── Payslip ───────────────────────────────────────────────────────────────

// ReplacePayslip menghapus slip lama (+ baris) satu karyawan pada periode lalu
// menulis yang baru — inti determinisme: hitung ulang = tulis ulang bersih.
func ReplacePayslip(ctx context.Context, tx *gorm.DB, s *models.Payslip, lines []models.PayslipLine) error {
	tid := currentTenantID(ctx)
	if err := tx.WithContext(ctx).
		Where("tenant_id = ? AND payroll_period_id = ? AND employee_id = ?", tid, s.PayrollPeriodID, s.EmployeeID).
		Delete(&models.Payslip{}).Error; err != nil {
		return err
	}
	s.ID = ulid.New()
	s.TenantID = tid
	for i := range lines {
		lines[i].ID = ulid.New()
		lines[i].TenantID = tid
		lines[i].PayslipID = s.ID
	}
	s.Lines = lines
	return tx.WithContext(ctx).Create(s).Error
}

func FindPayslip(ctx context.Context, tx *gorm.DB, id string) (models.Payslip, error) {
	var s models.Payslip
	err := scopeTenant(ctx, tenantDB(ctx, tx)).
		Preload("Lines", func(db *gorm.DB) *gorm.DB { return db.Order("payslip_lines.sort_order") }).
		First(&s, "payslips.id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return s, ErrPayslipNotFound
	}
	return s, err
}

func PayslipsForPeriod(ctx context.Context, tx *gorm.DB, periodID string) ([]models.Payslip, error) {
	var rows []models.Payslip
	err := scopeTenant(ctx, tenantDB(ctx, tx)).
		Preload("Lines", func(db *gorm.DB) *gorm.DB { return db.Order("payslip_lines.sort_order") }).
		Where("payroll_period_id = ?", periodID).
		Order("employee_id").
		Find(&rows).Error
	return rows, err
}

func MarkPayslipsStatus(ctx context.Context, tx *gorm.DB, periodID, status string, paidAt *time.Time) error {
	upd := map[string]any{"status": status, "updated_at": gorm.Expr("now()")}
	if paidAt != nil {
		upd["paid_at"] = *paidAt
	}
	return scopeTenant(ctx, tenantDB(ctx, tx)).Model(&models.Payslip{}).
		Where("payroll_period_id = ?", periodID).Updates(upd).Error
}

// ── Payroll adjustment ────────────────────────────────────────────────────

func CreatePayrollAdjustment(ctx context.Context, tx *gorm.DB, a *models.PayrollAdjustment) error {
	a.TenantID = currentTenantID(ctx)
	a.CreatedBy = reqctx.UserID(ctx)
	return tenantDB(ctx, tx).Create(a).Error
}

// PendingAdjustmentsForEmployee mengembalikan penyesuaian yang belum terpakai
// untuk karyawan, yang menyasar periode ini (target NULL = periode mana pun berikut).
func PendingAdjustmentsForEmployee(ctx context.Context, tx *gorm.DB, employeeID, periodID string) ([]models.PayrollAdjustment, error) {
	var rows []models.PayrollAdjustment
	err := scopeTenant(ctx, tenantDB(ctx, tx)).
		Where("employee_id = ? AND applied_payslip_id IS NULL AND (target_period_id IS NULL OR target_period_id = ?)",
			employeeID, periodID).
		Order("created_at").
		Find(&rows).Error
	return rows, err
}

func MarkAdjustmentApplied(ctx context.Context, tx *gorm.DB, adjustmentID, payslipID string) error {
	return scopeTenant(ctx, tenantDB(ctx, tx)).Model(&models.PayrollAdjustment{}).
		Where("id = ?", adjustmentID).
		Update("applied_payslip_id", payslipID).Error
}

func ClearAppliedAdjustmentsForPeriod(ctx context.Context, tx *gorm.DB, periodID string) error {
	// Saat hitung ulang, lepas penyesuaian yang tadinya terikat slip periode ini.
	return tenantDB(ctx, tx).Exec(`
		UPDATE payroll_adjustments SET applied_payslip_id = NULL
		WHERE tenant_id = ? AND applied_payslip_id IN (
			SELECT id FROM payslips WHERE tenant_id = ? AND payroll_period_id = ?
		)`, currentTenantID(ctx), currentTenantID(ctx), periodID).Error
}

// ── Employee advance (kasbon) ─────────────────────────────────────────────

func CreateAdvance(ctx context.Context, tx *gorm.DB, a *models.EmployeeAdvance) error {
	a.TenantID = currentTenantID(ctx)
	return tenantDB(ctx, tx).Create(a).Error
}

func FindAdvance(ctx context.Context, tx *gorm.DB, id string) (models.EmployeeAdvance, error) {
	var a models.EmployeeAdvance
	err := scopeTenant(ctx, tenantDB(ctx, tx)).First(&a, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return a, ErrAdvanceNotFound
	}
	return a, err
}

func SaveAdvance(ctx context.Context, tx *gorm.DB, a *models.EmployeeAdvance) error {
	return scopeTenant(ctx, tenantDB(ctx, tx)).Model(&models.EmployeeAdvance{}).Where("id = ?", a.ID).
		Updates(map[string]any{
			"remaining": a.Remaining, "status": a.Status, "approved_by": a.ApprovedBy,
			"approved_at": a.ApprovedAt, "disbursed_at": a.DisbursedAt, "cash_movement_id": a.CashMovementID,
			"updated_at": gorm.Expr("now()"),
		}).Error
}

func ListAdvances(ctx context.Context, employeeID, status string, limit, offset int) ([]models.EmployeeAdvance, int64, error) {
	build := func() *gorm.DB {
		q := scopeTenant(ctx, tenantDB(ctx, nil).Model(&models.EmployeeAdvance{}))
		if employeeID != "" {
			q = q.Where("employee_id = ?", employeeID)
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
	var rows []models.EmployeeAdvance
	err := build().Order("created_at DESC").Limit(limit).Offset(offset).Find(&rows).Error
	return rows, total, err
}

// OpenAdvanceForEmployee mengembalikan kasbon berjalan (disbursed, sisa > 0)
// tertua milik karyawan.
func OpenAdvanceForEmployee(ctx context.Context, tx *gorm.DB, employeeID string) (models.EmployeeAdvance, bool, error) {
	var a models.EmployeeAdvance
	err := scopeTenant(ctx, tenantDB(ctx, tx)).
		Where("employee_id = ? AND status = 'disbursed' AND remaining > 0", employeeID).
		Order("created_at").
		First(&a).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return a, false, nil
	}
	if err != nil {
		return a, false, err
	}
	return a, true, nil
}

func CreateAdvanceRepayment(ctx context.Context, tx *gorm.DB, r *models.AdvanceRepayment) error {
	r.TenantID = currentTenantID(ctx)
	return tenantDB(ctx, tx).Create(r).Error
}

// RepaymentExistsForPayslip cek apakah kasbon sudah dipotong oleh slip ini
// (UNIQUE tenant, advance, payslip) — jaga determinisme hitung-ulang.
func RepaymentExistsForPayslip(ctx context.Context, tx *gorm.DB, advanceID, payslipID string) (bool, error) {
	var n int64
	err := scopeTenant(ctx, tenantDB(ctx, tx).Model(&models.AdvanceRepayment{})).
		Where("advance_id = ? AND payslip_id = ?", advanceID, payslipID).Count(&n).Error
	return n > 0, err
}

// DeleteRepaymentsForPeriod melepas cicilan kasbon yang tercatat dari slip
// periode ini + mengembalikan `remaining` — dipanggil sebelum hitung ulang.
func DeleteRepaymentsForPeriod(ctx context.Context, tx *gorm.DB, periodID string) error {
	tid := currentTenantID(ctx)
	// Kembalikan remaining untuk tiap kasbon yang cicilannya berasal dari slip periode ini.
	if err := tx.WithContext(ctx).Exec(`
		UPDATE employee_advances ea SET remaining = LEAST(ea.amount, ea.remaining + agg.total), status = 'disbursed'
		FROM (
			SELECT ar.advance_id, SUM(ar.amount) AS total
			FROM advance_repayments ar
			JOIN payslips p ON p.id = ar.payslip_id
			WHERE ar.tenant_id = ? AND p.payroll_period_id = ?
			GROUP BY ar.advance_id
		) agg
		WHERE ea.tenant_id = ? AND ea.id = agg.advance_id`, tid, periodID, tid).Error; err != nil {
		return err
	}
	return tx.WithContext(ctx).Exec(`
		DELETE FROM advance_repayments
		WHERE tenant_id = ? AND payslip_id IN (SELECT id FROM payslips WHERE tenant_id = ? AND payroll_period_id = ?)`,
		tid, tid, periodID).Error
}

// ── Cash movement (kas keluar non-shift) ──────────────────────────────────

// InsertPayrollCashOut mencatat kas keluar untuk gaji/kasbon — TANPA shift_id.
// Mengembalikan id cash_movement.
func InsertPayrollCashOut(ctx context.Context, tx *gorm.DB, outletID string, amount int64, reason, refTable, refID string, bizDate time.Time) (string, error) {
	id := ulid.New()
	err := tx.WithContext(ctx).Exec(`
		INSERT INTO cash_movements
			(id, tenant_id, outlet_id, shift_id, direction, amount, reason,
			 occurred_at, business_date, ref_table, ref_id, created_by, created_at)
		VALUES (?, ?, ?, NULL, 'out', ?, ?, now(), ?, ?, ?, ?, now())`,
		id, currentTenantID(ctx), outletID, amount, reason, bizDate, refTable, refID, reqctx.UserID(ctx),
	).Error
	return id, err
}
