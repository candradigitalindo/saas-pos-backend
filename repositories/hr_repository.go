package repositories

import (
	"context"
	"errors"
	"time"

	"candra/backend-api/models"

	"gorm.io/gorm"
)

// Repositori SDM — karyawan, jadwal, libur, absensi, koreksi, cache harian, cuti
// (§5.11). Tabel bertenant + RLS; hanya lapis 1 (scopeTenant).

var (
	ErrEmployeeNotFound   = errors.New("karyawan tidak ditemukan")
	ErrAttendanceNotFound = errors.New("absensi tidak ditemukan")
	ErrCorrectionNotFound = errors.New("koreksi absensi tidak ditemukan")
	ErrLeaveNotFound      = errors.New("pengajuan cuti tidak ditemukan")
)

// ── Employee ──────────────────────────────────────────────────────────────

func ListEmployees(ctx context.Context, outletID string, activeOnly bool, limit, offset int) ([]models.Employee, int64, error) {
	build := func() *gorm.DB {
		q := scopeTenant(ctx, tenantDB(ctx, nil).Model(&models.Employee{}))
		if outletID != "" {
			q = q.Where("outlet_id = ?", outletID)
		}
		if activeOnly {
			q = q.Where("is_active = ?", true)
		}
		return q
	}
	var total int64
	if err := build().Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []models.Employee
	err := build().Order("full_name").Limit(limit).Offset(offset).Find(&rows).Error
	return rows, total, err
}

func FindEmployee(ctx context.Context, tx *gorm.DB, id string) (models.Employee, error) {
	var e models.Employee
	err := scopeTenant(ctx, tenantDB(ctx, tx)).First(&e, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return e, ErrEmployeeNotFound
	}
	return e, err
}

// ActiveEmployeesForPeriod mengembalikan karyawan yang aktif pada rentang
// periode (joined_at <= end, resigned_at NULL atau >= start), opsional per outlet.
func ActiveEmployeesForPeriod(ctx context.Context, tx *gorm.DB, outletID string, start, end time.Time) ([]models.Employee, error) {
	q := scopeTenant(ctx, tenantDB(ctx, tx).Model(&models.Employee{})).
		Where("joined_at <= ? AND (resigned_at IS NULL OR resigned_at >= ?)", end, start)
	if outletID != "" {
		q = q.Where("outlet_id = ?", outletID)
	}
	var rows []models.Employee
	err := q.Order("id").Find(&rows).Error
	return rows, err
}

func CreateEmployee(ctx context.Context, tx *gorm.DB, e *models.Employee) error {
	e.TenantID = currentTenantID(ctx)
	return tenantDB(ctx, tx).Create(e).Error
}

func SaveEmployee(ctx context.Context, tx *gorm.DB, e *models.Employee) error {
	res := scopeTenant(ctx, tenantDB(ctx, tx)).Model(&models.Employee{}).Where("id = ?", e.ID).
		Updates(map[string]any{
			"full_name": e.FullName, "phone": e.Phone, "email": e.Email, "position": e.Position,
			"employment_status": e.EmploymentStatus, "wage_type": e.WageType, "base_wage": e.BaseWage,
			"payroll_period_type": e.PayrollPeriodType, "bank_name": e.BankName,
			"bank_account_no": e.BankAccountNo, "bank_account_name": e.BankAccountName,
			"resigned_at": e.ResignedAt, "is_active": e.IsActive, "updated_at": gorm.Expr("now()"),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrEmployeeNotFound
	}
	return nil
}

// ── Work schedule ─────────────────────────────────────────────────────────

func UpsertWorkSchedule(ctx context.Context, tx *gorm.DB, s *models.WorkSchedule) error {
	s.TenantID = currentTenantID(ctx)
	var existing models.WorkSchedule
	err := scopeTenant(ctx, tenantDB(ctx, tx)).
		First(&existing, "employee_id = ? AND weekday = ? AND effective_from = ?", s.EmployeeID, s.Weekday, s.EffectiveFrom).Error
	if err == nil {
		s.ID = existing.ID
		return tenantDB(ctx, tx).Model(&models.WorkSchedule{}).Where("id = ?", existing.ID).Updates(map[string]any{
			"is_working_day": s.IsWorkingDay, "start_time": s.StartTime, "end_time": s.EndTime,
			"break_minutes": s.BreakMinutes, "late_tolerance_minutes": s.LateToleranceMinutes,
			"effective_to": s.EffectiveTo,
		}).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return tenantDB(ctx, tx).Create(s).Error
}

// SchedulesForEmployee memuat seluruh baris jadwal karyawan (semua weekday).
func SchedulesForEmployee(ctx context.Context, tx *gorm.DB, employeeID string) ([]models.WorkSchedule, error) {
	var rows []models.WorkSchedule
	err := scopeTenant(ctx, tenantDB(ctx, tx)).
		Where("employee_id = ?", employeeID).
		Order("weekday, effective_from DESC").
		Find(&rows).Error
	return rows, err
}

// ── Holiday ───────────────────────────────────────────────────────────────

func CreateHoliday(ctx context.Context, tx *gorm.DB, h *models.Holiday) error {
	h.TenantID = currentTenantID(ctx)
	return tenantDB(ctx, tx).Create(h).Error
}

func ListHolidays(ctx context.Context, from, to string) ([]models.Holiday, error) {
	q := scopeTenant(ctx, tenantDB(ctx, nil))
	if from != "" && to != "" {
		q = q.Where("holiday_date BETWEEN ? AND ?", from, to)
	}
	var rows []models.Holiday
	err := q.Order("holiday_date").Find(&rows).Error
	return rows, err
}

// HolidaysInRange mengembalikan libur (outlet spesifik + global) pada rentang.
func HolidaysInRange(ctx context.Context, tx *gorm.DB, outletID string, start, end time.Time) ([]models.Holiday, error) {
	var rows []models.Holiday
	err := scopeTenant(ctx, tenantDB(ctx, tx)).
		Where("holiday_date BETWEEN ? AND ? AND (outlet_id IS NULL OR outlet_id = ?)", start, end, outletID).
		Find(&rows).Error
	return rows, err
}

// ── Attendance ────────────────────────────────────────────────────────────

func FindAttendanceInTenant(ctx context.Context, tx *gorm.DB, id string) (models.Attendance, bool, error) {
	var a models.Attendance
	err := scopeTenant(ctx, tenantDB(ctx, tx)).First(&a, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return a, false, nil
	}
	if err != nil {
		return a, false, err
	}
	return a, true, nil
}

func CreateAttendance(ctx context.Context, tx *gorm.DB, a *models.Attendance) error {
	a.TenantID = currentTenantID(ctx)
	return tx.WithContext(ctx).Create(a).Error
}

func ListAttendances(ctx context.Context, employeeID, businessDate string, limit, offset int) ([]models.Attendance, int64, error) {
	build := func() *gorm.DB {
		q := scopeTenant(ctx, tenantDB(ctx, nil).Model(&models.Attendance{}))
		if employeeID != "" {
			q = q.Where("employee_id = ?", employeeID)
		}
		if businessDate != "" {
			q = q.Where("business_date = ?", businessDate)
		}
		return q
	}
	var total int64
	if err := build().Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []models.Attendance
	err := build().Order("occurred_at DESC").Limit(limit).Offset(offset).Find(&rows).Error
	return rows, total, err
}

// AttendancesForEmployeeRange memuat ketukan absen karyawan pada rentang, urut waktu.
func AttendancesForEmployeeRange(ctx context.Context, tx *gorm.DB, employeeID string, start, end time.Time) ([]models.Attendance, error) {
	var rows []models.Attendance
	err := scopeTenant(ctx, tenantDB(ctx, tx)).
		Where("employee_id = ? AND business_date BETWEEN ? AND ?", employeeID, start, end).
		Order("occurred_at").
		Find(&rows).Error
	return rows, err
}

// ── Attendance correction ─────────────────────────────────────────────────

func CreateCorrection(ctx context.Context, tx *gorm.DB, c *models.AttendanceCorrection) error {
	c.TenantID = currentTenantID(ctx)
	return tenantDB(ctx, tx).Create(c).Error
}

func FindCorrection(ctx context.Context, tx *gorm.DB, id string) (models.AttendanceCorrection, error) {
	var c models.AttendanceCorrection
	err := scopeTenant(ctx, tenantDB(ctx, tx)).First(&c, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return c, ErrCorrectionNotFound
	}
	return c, err
}

func SaveCorrection(ctx context.Context, tx *gorm.DB, c *models.AttendanceCorrection) error {
	return tenantDB(ctx, tx).Model(&models.AttendanceCorrection{}).Where("id = ? AND tenant_id = ?", c.ID, currentTenantID(ctx)).
		Updates(map[string]any{"status": c.Status, "approved_by": c.ApprovedBy, "approved_at": c.ApprovedAt}).Error
}

// ApprovedCorrectionsForRange mengembalikan koreksi disetujui atas absensi
// karyawan pada rentang, dipetakan attendance_id → new_occurred_at.
func ApprovedCorrectionsForRange(ctx context.Context, tx *gorm.DB, employeeID string, start, end time.Time) (map[string]time.Time, error) {
	rows := []struct {
		AttendanceID  string
		NewOccurredAt time.Time
	}{}
	err := tenantDB(ctx, tx).
		Table("attendance_corrections ac").
		Joins("JOIN attendances a ON a.id = ac.attendance_id").
		Where("ac.tenant_id = ? AND ac.status = 'approved' AND a.employee_id = ? AND a.business_date BETWEEN ? AND ?",
			currentTenantID(ctx), employeeID, start, end).
		Select("ac.attendance_id, ac.new_occurred_at").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make(map[string]time.Time, len(rows))
	for _, r := range rows {
		out[r.AttendanceID] = r.NewOccurredAt
	}
	return out, nil
}

// ── Attendance day (cache) ────────────────────────────────────────────────

func UpsertAttendanceDay(ctx context.Context, tx *gorm.DB, d *models.AttendanceDay) error {
	d.TenantID = currentTenantID(ctx)
	return tx.WithContext(ctx).Exec(`
		INSERT INTO attendance_days
			(tenant_id, employee_id, business_date, status, scheduled_start, scheduled_end,
			 first_in, last_out, late_minutes, early_leave_minutes, work_minutes, overtime_minutes,
			 leave_request_id, computed_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?, now())
		ON CONFLICT (tenant_id, employee_id, business_date) DO UPDATE SET
			status = EXCLUDED.status, scheduled_start = EXCLUDED.scheduled_start,
			scheduled_end = EXCLUDED.scheduled_end, first_in = EXCLUDED.first_in,
			last_out = EXCLUDED.last_out, late_minutes = EXCLUDED.late_minutes,
			early_leave_minutes = EXCLUDED.early_leave_minutes, work_minutes = EXCLUDED.work_minutes,
			overtime_minutes = EXCLUDED.overtime_minutes, leave_request_id = EXCLUDED.leave_request_id,
			computed_at = now()`,
		d.TenantID, d.EmployeeID, d.BusinessDate, d.Status, d.ScheduledStart, d.ScheduledEnd,
		d.FirstIn, d.LastOut, d.LateMinutes, d.EarlyLeaveMinutes, d.WorkMinutes, d.OvertimeMinutes,
		d.LeaveRequestID,
	).Error
}

// AttendanceDaysForRange memuat cache harian karyawan pada rentang, urut tanggal.
func AttendanceDaysForRange(ctx context.Context, tx *gorm.DB, employeeID string, start, end time.Time) ([]models.AttendanceDay, error) {
	var rows []models.AttendanceDay
	err := scopeTenant(ctx, tenantDB(ctx, tx)).
		Where("employee_id = ? AND business_date BETWEEN ? AND ?", employeeID, start, end).
		Order("business_date").
		Find(&rows).Error
	return rows, err
}

// ── Leave ─────────────────────────────────────────────────────────────────

func CreateLeaveRequest(ctx context.Context, tx *gorm.DB, l *models.LeaveRequest) error {
	l.TenantID = currentTenantID(ctx)
	return tenantDB(ctx, tx).Create(l).Error
}

func FindLeaveRequest(ctx context.Context, tx *gorm.DB, id string) (models.LeaveRequest, error) {
	var l models.LeaveRequest
	err := scopeTenant(ctx, tenantDB(ctx, tx)).First(&l, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return l, ErrLeaveNotFound
	}
	return l, err
}

func SaveLeaveRequest(ctx context.Context, tx *gorm.DB, l *models.LeaveRequest) error {
	return tenantDB(ctx, tx).Model(&models.LeaveRequest{}).Where("id = ? AND tenant_id = ?", l.ID, currentTenantID(ctx)).
		Updates(map[string]any{
			"is_paid": l.IsPaid, "status": l.Status, "approved_by": l.ApprovedBy,
			"approved_at": l.ApprovedAt, "reject_reason": l.RejectReason, "updated_at": gorm.Expr("now()"),
		}).Error
}

// ApprovedLeaveForRange memuat cuti disetujui yang beririsan dengan rentang.
func ApprovedLeaveForRange(ctx context.Context, tx *gorm.DB, employeeID string, start, end time.Time) ([]models.LeaveRequest, error) {
	var rows []models.LeaveRequest
	err := scopeTenant(ctx, tenantDB(ctx, tx)).
		Where("employee_id = ? AND status = 'approved' AND start_date <= ? AND end_date >= ?", employeeID, end, start).
		Find(&rows).Error
	return rows, err
}

func UpsertLeaveBalanceUsage(ctx context.Context, tx *gorm.DB, employeeID string, year int, deltaUsed string) error {
	tid := currentTenantID(ctx)
	return tx.WithContext(ctx).Exec(`
		INSERT INTO leave_balances (tenant_id, employee_id, year, used_days)
		VALUES (?, ?, ?, ?)
		ON CONFLICT (tenant_id, employee_id, year) DO UPDATE SET used_days = leave_balances.used_days + ?`,
		tid, employeeID, year, deltaUsed, deltaUsed,
	).Error
}
