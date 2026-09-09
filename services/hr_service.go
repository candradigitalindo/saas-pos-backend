package services

import (
	"context"
	"fmt"
	"time"

	"candra/backend-api/helpers"
	"candra/backend-api/internal/reqctx"
	"candra/backend-api/internal/timez"
	"candra/backend-api/models"
	"candra/backend-api/repositories"
	"candra/backend-api/structs"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// Layanan SDM — karyawan, jadwal, libur, cuti, kasbon (Fase 13, §5.11).

func parseHRDate(field, s string, required bool) (*time.Time, error) {
	if s == "" {
		if required {
			return nil, fmt.Errorf("%w: %s wajib (YYYY-MM-DD)", helpers.ErrValidation, field)
		}
		return nil, nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return nil, fmt.Errorf("%w: %s harus YYYY-MM-DD", helpers.ErrValidation, field)
	}
	return &t, nil
}

// ── Employee ──────────────────────────────────────────────────────────────

func CreateEmployee(ctx context.Context, in structs.EmployeeCreateRequest) (structs.EmployeeResponse, error) {
	var out structs.EmployeeResponse
	joined, err := parseHRDate("joined_at", in.JoinedAt, true)
	if err != nil {
		return out, err
	}
	err = repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		if err := repositories.FindOutletByID(ctx, tx, in.OutletID, &models.Outlet{}); err != nil {
			return fmt.Errorf("%w: outlet tidak ditemukan", helpers.ErrValidation)
		}
		e := models.Employee{
			OutletID: in.OutletID, EmployeeNo: in.EmployeeNo, FullName: in.FullName,
			Phone: in.Phone, Email: in.Email, Position: in.Position,
			EmploymentStatus: orDefault(in.EmploymentStatus, "permanent"),
			WageType:         in.WageType, BaseWage: in.BaseWage,
			PayrollPeriodType: orDefault(in.PayrollPeriodType, "monthly"),
			BankName:          in.BankName, BankAccountNo: in.BankAccountNo, BankAccountName: in.BankAccountName,
			JoinedAt: *joined, IsActive: true,
		}
		if in.UserID != "" {
			e.UserID = &in.UserID
		}
		if err := repositories.CreateEmployee(ctx, tx, &e); err != nil {
			if helpers.IsDuplicateEntryError(err) {
				return fmt.Errorf("%w: nomor karyawan atau akun sudah terpakai", helpers.ErrConflict)
			}
			return err
		}
		out = employeeToResponse(e)
		return nil
	})
	return out, err
}

func UpdateEmployee(ctx context.Context, id string, in structs.EmployeeUpdateRequest) (structs.EmployeeResponse, error) {
	var out structs.EmployeeResponse
	err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		e, err := repositories.FindEmployee(ctx, tx, id)
		if err != nil {
			return err
		}
		if in.FullName != nil {
			e.FullName = *in.FullName
		}
		if in.Phone != nil {
			e.Phone = *in.Phone
		}
		if in.Email != nil {
			e.Email = *in.Email
		}
		if in.Position != nil {
			e.Position = *in.Position
		}
		if in.EmploymentStatus != nil {
			e.EmploymentStatus = *in.EmploymentStatus
		}
		if in.WageType != nil {
			e.WageType = *in.WageType
		}
		if in.BaseWage != nil {
			e.BaseWage = *in.BaseWage
		}
		if in.PayrollPeriodType != nil {
			e.PayrollPeriodType = *in.PayrollPeriodType
		}
		if in.BankName != nil {
			e.BankName = *in.BankName
		}
		if in.BankAccountNo != nil {
			e.BankAccountNo = *in.BankAccountNo
		}
		if in.BankAccountName != nil {
			e.BankAccountName = *in.BankAccountName
		}
		if in.ResignedAt != nil {
			t, derr := parseHRDate("resigned_at", *in.ResignedAt, false)
			if derr != nil {
				return derr
			}
			e.ResignedAt = t
		}
		if in.IsActive != nil {
			e.IsActive = *in.IsActive
		}
		if err := repositories.SaveEmployee(ctx, tx, &e); err != nil {
			return err
		}
		reloaded, err := repositories.FindEmployee(ctx, tx, id)
		if err != nil {
			return err
		}
		out = employeeToResponse(reloaded)
		return nil
	})
	return out, err
}

// ── Work schedule ─────────────────────────────────────────────────────────

func SetWorkSchedule(ctx context.Context, employeeID string, in structs.WorkScheduleRequest) (structs.WorkScheduleResponse, error) {
	var out structs.WorkScheduleResponse
	ef, err := parseHRDate("effective_from", in.EffectiveFrom, true)
	if err != nil {
		return out, err
	}
	err = repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		if _, ferr := repositories.FindEmployee(ctx, tx, employeeID); ferr != nil {
			return ferr
		}
		s := models.WorkSchedule{
			EmployeeID: employeeID, Weekday: in.Weekday, IsWorkingDay: true,
			BreakMinutes: in.BreakMinutes, LateToleranceMinutes: in.LateToleranceMinutes,
			EffectiveFrom: *ef,
		}
		if in.IsWorkingDay != nil {
			s.IsWorkingDay = *in.IsWorkingDay
		}
		if in.StartTime != "" {
			c, cerr := timez.ParseClock(in.StartTime)
			if cerr != nil {
				return fmt.Errorf("%w: start_time harus HH:MM", helpers.ErrValidation)
			}
			s.StartTime = &c
		}
		if in.EndTime != "" {
			c, cerr := timez.ParseClock(in.EndTime)
			if cerr != nil {
				return fmt.Errorf("%w: end_time harus HH:MM", helpers.ErrValidation)
			}
			s.EndTime = &c
		}
		if err := repositories.UpsertWorkSchedule(ctx, tx, &s); err != nil {
			return err
		}
		out = workScheduleToResponse(s)
		return nil
	})
	return out, err
}

// ── Holiday ───────────────────────────────────────────────────────────────

func CreateHoliday(ctx context.Context, in structs.HolidayRequest) (structs.HolidayResponse, error) {
	var out structs.HolidayResponse
	hd, err := parseHRDate("holiday_date", in.HolidayDate, true)
	if err != nil {
		return out, err
	}
	err = repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		h := models.Holiday{HolidayDate: *hd, Name: in.Name, IsPaid: true}
		if in.OutletID != "" {
			h.OutletID = &in.OutletID
		}
		if in.IsPaid != nil {
			h.IsPaid = *in.IsPaid
		}
		if err := repositories.CreateHoliday(ctx, tx, &h); err != nil {
			if helpers.IsDuplicateEntryError(err) {
				return fmt.Errorf("%w: libur untuk tanggal itu sudah ada", helpers.ErrConflict)
			}
			return err
		}
		out = holidayToResponse(h)
		return nil
	})
	return out, err
}

// ── Leave ─────────────────────────────────────────────────────────────────

func CreateLeaveRequest(ctx context.Context, in structs.LeaveRequestCreate) (structs.LeaveRequestResponse, error) {
	var out structs.LeaveRequestResponse
	sd, err := parseHRDate("start_date", in.StartDate, true)
	if err != nil {
		return out, err
	}
	ed, err := parseHRDate("end_date", in.EndDate, true)
	if err != nil {
		return out, err
	}
	if ed.Before(*sd) {
		return out, fmt.Errorf("%w: end_date lebih awal dari start_date", helpers.ErrValidation)
	}
	days, err := decimal.NewFromString(in.Days)
	if err != nil || days.LessThanOrEqual(decimal.Zero) {
		return out, fmt.Errorf("%w: days tidak valid", helpers.ErrValidation)
	}

	err = repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		if _, ferr := repositories.FindEmployee(ctx, tx, in.EmployeeID); ferr != nil {
			return ferr
		}
		l := models.LeaveRequest{
			EmployeeID: in.EmployeeID, Kind: in.Kind, StartDate: *sd, EndDate: *ed,
			Days: days, Reason: in.Reason, IsPaid: in.Kind != "unpaid", Status: "pending",
		}
		if err := repositories.CreateLeaveRequest(ctx, tx, &l); err != nil {
			return err
		}
		out = leaveToResponse(l)
		return nil
	})
	return out, err
}

func ApproveLeaveRequest(ctx context.Context, id string, in structs.LeaveApproveRequest, approve bool) (structs.LeaveRequestResponse, error) {
	var out structs.LeaveRequestResponse
	err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		l, err := repositories.FindLeaveRequest(ctx, tx, id)
		if err != nil {
			return err
		}
		if l.Status != "pending" {
			return fmt.Errorf("%w: pengajuan sudah diproses", helpers.ErrConflict)
		}
		now := time.Now().UTC()
		uid := reqctx.UserID(ctx)
		l.ApprovedBy = &uid
		l.ApprovedAt = &now
		if !approve {
			l.Status = "rejected"
			l.RejectReason = in.RejectReason
		} else {
			l.Status = "approved"
			if in.IsPaid != nil {
				l.IsPaid = *in.IsPaid
			}
			if l.IsPaid && l.Kind == "leave" {
				if err := repositories.UpsertLeaveBalanceUsage(ctx, tx, l.EmployeeID, l.StartDate.Year(), l.Days.String()); err != nil {
					return err
				}
			}
		}
		if err := repositories.SaveLeaveRequest(ctx, tx, &l); err != nil {
			return err
		}
		out = leaveToResponse(l)
		return nil
	})
	return out, err
}

// ── Employee advance (kasbon) ─────────────────────────────────────────────

func CreateAdvance(ctx context.Context, in structs.AdvanceCreateRequest) (structs.AdvanceResponse, error) {
	var out structs.AdvanceResponse
	if in.InstallmentAmount > in.Amount {
		return out, fmt.Errorf("%w: cicilan melebihi nilai kasbon", helpers.ErrValidation)
	}
	err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		if _, ferr := repositories.FindEmployee(ctx, tx, in.EmployeeID); ferr != nil {
			return ferr
		}
		a := models.EmployeeAdvance{
			EmployeeID: in.EmployeeID, Amount: in.Amount, Remaining: in.Amount,
			InstallmentAmount: in.InstallmentAmount, Reason: in.Reason, Status: "pending",
		}
		if err := repositories.CreateAdvance(ctx, tx, &a); err != nil {
			return err
		}
		out = advanceToResponse(a)
		return nil
	})
	return out, err
}

// DisburseAdvance menyetujui & mencairkan kasbon: kas keluar
// (ref_table='employee_advances') lalu status → disbursed.
func DisburseAdvance(ctx context.Context, id string) (structs.AdvanceResponse, error) {
	var out structs.AdvanceResponse
	err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		a, err := repositories.FindAdvance(ctx, tx, id)
		if err != nil {
			return err
		}
		if a.Status != "pending" && a.Status != "approved" {
			return fmt.Errorf("%w: kasbon tidak dalam status yang bisa dicairkan", helpers.ErrConflict)
		}
		emp, err := repositories.FindEmployee(ctx, tx, a.EmployeeID)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		cmID, err := repositories.InsertPayrollCashOut(ctx, tx, emp.OutletID, a.Amount,
			"Pencairan kasbon "+emp.FullName, "employee_advances", a.ID, now)
		if err != nil {
			return err
		}
		uid := reqctx.UserID(ctx)
		a.Status = "disbursed"
		a.ApprovedBy = &uid
		a.ApprovedAt = &now
		a.DisbursedAt = &now
		a.CashMovementID = &cmID
		if err := repositories.SaveAdvance(ctx, tx, &a); err != nil {
			return err
		}
		reloaded, err := repositories.FindAdvance(ctx, tx, id)
		if err != nil {
			return err
		}
		out = advanceToResponse(reloaded)
		return nil
	})
	return out, err
}

// ── DTO ───────────────────────────────────────────────────────────────────

func employeeToResponse(e models.Employee) structs.EmployeeResponse {
	r := structs.EmployeeResponse{
		ID: e.ID, OutletID: e.OutletID, EmployeeNo: e.EmployeeNo, FullName: e.FullName,
		Phone: e.Phone, Position: e.Position, EmploymentStatus: e.EmploymentStatus,
		WageType: e.WageType, BaseWage: e.BaseWage, PayrollPeriodType: e.PayrollPeriodType,
		JoinedAt: e.JoinedAt.Format("2006-01-02"), IsActive: e.IsActive,
	}
	if e.UserID != nil {
		r.UserID = *e.UserID
	}
	if e.ResignedAt != nil {
		r.ResignedAt = e.ResignedAt.Format("2006-01-02")
	}
	return r
}

func workScheduleToResponse(s models.WorkSchedule) structs.WorkScheduleResponse {
	r := structs.WorkScheduleResponse{
		ID: s.ID, Weekday: s.Weekday, IsWorkingDay: s.IsWorkingDay,
		BreakMinutes: s.BreakMinutes, LateToleranceMinutes: s.LateToleranceMinutes,
		EffectiveFrom: s.EffectiveFrom.Format("2006-01-02"),
	}
	if s.StartTime != nil {
		r.StartTime = s.StartTime.String()
	}
	if s.EndTime != nil {
		r.EndTime = s.EndTime.String()
	}
	return r
}

func holidayToResponse(h models.Holiday) structs.HolidayResponse {
	r := structs.HolidayResponse{
		ID: h.ID, HolidayDate: h.HolidayDate.Format("2006-01-02"), Name: h.Name, IsPaid: h.IsPaid,
	}
	if h.OutletID != nil {
		r.OutletID = *h.OutletID
	}
	return r
}

func leaveToResponse(l models.LeaveRequest) structs.LeaveRequestResponse {
	return structs.LeaveRequestResponse{
		ID: l.ID, EmployeeID: l.EmployeeID, Kind: l.Kind,
		StartDate: l.StartDate.Format("2006-01-02"), EndDate: l.EndDate.Format("2006-01-02"),
		Days: l.Days.String(), IsPaid: l.IsPaid, Status: l.Status, Reason: l.Reason,
	}
}

func advanceToResponse(a models.EmployeeAdvance) structs.AdvanceResponse {
	return structs.AdvanceResponse{
		ID: a.ID, EmployeeID: a.EmployeeID, Amount: a.Amount, Remaining: a.Remaining,
		InstallmentAmount: a.InstallmentAmount, Status: a.Status, Reason: a.Reason,
	}
}

// EmployeeToResponse / AdvanceToResponse — untuk controller list.
func EmployeeToResponse(e models.Employee) structs.EmployeeResponse      { return employeeToResponse(e) }
func AdvanceToResponse(a models.EmployeeAdvance) structs.AdvanceResponse { return advanceToResponse(a) }
