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

// Layanan absensi (§5.11). `attendances` adalah kebenaran; `attendance_days`
// hanya cache yang dibangun ulang dari attendances + koreksi disetujui + jadwal
// + libur + cuti — dengan URUTAN STATUS TETAP: libur > cuti > ada absensi > alpa.

// RecordAttendance mencatat satu ketukan absen. Idempoten per ULID klien
// (dipakai POST /attendances & op sync 'attendance.upsert'). business_date
// dihitung ULANG di server dari zona outlet karyawan.
func RecordAttendance(ctx context.Context, in structs.AttendanceRecordRequest) (structs.AttendanceResponse, error) {
	var out structs.AttendanceResponse
	occurred := time.Now().UTC()
	if in.OccurredAt != "" {
		t, err := time.Parse(time.RFC3339, in.OccurredAt)
		if err != nil {
			return out, fmt.Errorf("%w: occurred_at harus RFC3339", helpers.ErrValidation)
		}
		occurred = t.UTC()
	}

	err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		emp, err := repositories.FindEmployee(ctx, tx, in.EmployeeID)
		if err != nil {
			return err
		}

		if in.ID != "" {
			if existing, ok, ferr := repositories.FindAttendanceInTenant(ctx, tx, in.ID); ferr != nil {
				return ferr
			} else if ok {
				out = attendanceToResponse(existing)
				return nil // sudah ada → idempoten, ketukan tak diubah
			}
		}

		var outlet models.Outlet
		if err := repositories.FindOutletByID(ctx, tx, emp.OutletID, &outlet); err != nil {
			return err
		}
		bizDate, err := timez.BusinessDate(occurred, outlet.Timezone, outlet.DayStartOffset())
		if err != nil {
			return err
		}

		a := models.Attendance{
			EmployeeID: emp.ID, OutletID: emp.OutletID, Kind: in.Kind,
			OccurredAt: occurred, BusinessDate: bizDate,
			Source: "manual", PhotoURL: in.PhotoURL, DeviceID: in.DeviceID,
			CreatedBy: reqctx.UserID(ctx),
		}
		if in.ID != "" {
			a.ID = in.ID
		}
		if in.Latitude != "" {
			if d, e := decimal.NewFromString(in.Latitude); e == nil {
				a.Latitude = &d
			}
		}
		if in.Longitude != "" {
			if d, e := decimal.NewFromString(in.Longitude); e == nil {
				a.Longitude = &d
			}
		}
		if err := repositories.CreateAttendance(ctx, tx, &a); err != nil {
			return err
		}
		out = attendanceToResponse(a)
		return nil
	})
	return out, err
}

// RequestCorrection mengajukan koreksi waktu sebuah absensi (baris asli tak diubah).
func RequestCorrection(ctx context.Context, in structs.CorrectionRequest) (structs.CorrectionResponse, error) {
	var out structs.CorrectionResponse
	newAt, err := time.Parse(time.RFC3339, in.NewOccurredAt)
	if err != nil {
		return out, fmt.Errorf("%w: new_occurred_at harus RFC3339", helpers.ErrValidation)
	}
	err = repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		if _, ok, ferr := repositories.FindAttendanceInTenant(ctx, tx, in.AttendanceID); ferr != nil {
			return ferr
		} else if !ok {
			return repositories.ErrAttendanceNotFound
		}
		c := models.AttendanceCorrection{
			AttendanceID: in.AttendanceID, NewOccurredAt: newAt.UTC(),
			Reason: in.Reason, RequestedBy: reqctx.UserID(ctx), Status: "pending",
		}
		if err := repositories.CreateCorrection(ctx, tx, &c); err != nil {
			return err
		}
		out = correctionToResponse(c)
		return nil
	})
	return out, err
}

// ApproveCorrection menyetujui koreksi. Bila hari yang dikoreksi berada di
// periode gaji yang SUDAH DIKUNCI dan penyetuju menyertakan `adjustment`, sebuah
// `payroll_adjustment` dibuat (origin = periode terkunci) agar selisihnya muncul
// di periode BERIKUTNYA (§13.6).
func ApproveCorrection(ctx context.Context, id string, adj *structs.CorrectionAdjustment) (structs.CorrectionResponse, error) {
	var out structs.CorrectionResponse
	err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		c, err := repositories.FindCorrection(ctx, tx, id)
		if err != nil {
			return err
		}
		if c.Status != "pending" {
			return fmt.Errorf("%w: koreksi sudah tidak berstatus pending", helpers.ErrConflict)
		}
		att, _, err := repositories.FindAttendanceInTenant(ctx, tx, c.AttendanceID)
		if err != nil {
			return err
		}

		now := time.Now().UTC()
		uid := reqctx.UserID(ctx)
		c.Status = "approved"
		c.ApprovedBy = &uid
		c.ApprovedAt = &now
		if err := repositories.SaveCorrection(ctx, tx, &c); err != nil {
			return err
		}

		if adj != nil {
			lp, locked, lerr := repositories.LockedPeriodCovering(ctx, tx, att.OutletID, att.BusinessDate)
			if lerr != nil {
				return lerr
			}
			if !locked {
				return fmt.Errorf("%w: tidak ada periode gaji terkunci yang mencakup tanggal itu", helpers.ErrValidation)
			}
			if err := repositories.CreatePayrollAdjustment(ctx, tx, &models.PayrollAdjustment{
				EmployeeID: att.EmployeeID, OriginPeriodID: lp.ID,
				Name: adj.Name, Category: adj.Category, Amount: adj.Amount,
				Reason: "Koreksi absensi disetujui: " + c.Reason,
			}); err != nil {
				return err
			}
		}
		out = correctionToResponse(c)
		return nil
	})
	return out, err
}

// ── Rebuild attendance_days ───────────────────────────────────────────────

// dayCalc adalah hasil hitung satu hari (dipakai juga oleh payroll).
type dayCalc struct {
	Status          string
	LateMinutes     int
	WorkMinutes     int
	OvertimeMinutes int
	FirstIn         *time.Time
	LastOut         *time.Time
	LeaveIsPaid     *bool
}

// RebuildAttendanceDays menghitung ulang attendance_days seluruh karyawan aktif
// pada [start, end] untuk sebuah outlet (kosong = semua). Idempoten.
func RebuildAttendanceDays(ctx context.Context, tx *gorm.DB, outletID string, start, end time.Time) error {
	emps, err := repositories.ActiveEmployeesForPeriod(ctx, tx, outletID, start, end)
	if err != nil {
		return err
	}
	for _, emp := range emps {
		var outlet models.Outlet
		if err := repositories.FindOutletByID(ctx, tx, emp.OutletID, &outlet); err != nil {
			return err
		}
		loc, err := timez.LoadLocation(outlet.Timezone)
		if err != nil {
			return err
		}

		schedules, err := repositories.SchedulesForEmployee(ctx, tx, emp.ID)
		if err != nil {
			return err
		}
		holidays, err := repositories.HolidaysInRange(ctx, tx, emp.OutletID, start, end)
		if err != nil {
			return err
		}
		leaves, err := repositories.ApprovedLeaveForRange(ctx, tx, emp.ID, start, end)
		if err != nil {
			return err
		}
		atts, err := repositories.AttendancesForEmployeeRange(ctx, tx, emp.ID, start, end)
		if err != nil {
			return err
		}
		corr, err := repositories.ApprovedCorrectionsForRange(ctx, tx, emp.ID, start, end)
		if err != nil {
			return err
		}

		holidaySet := map[string]bool{}
		for _, h := range holidays {
			holidaySet[h.HolidayDate.Format("2006-01-02")] = true
		}

		for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
			key := d.Format("2006-01-02")
			dc, leaveID := computeDay(d, key, loc, schedules, holidaySet, leaves, atts, corr)

			day := models.AttendanceDay{
				EmployeeID: emp.ID, BusinessDate: d, Status: dc.Status,
				FirstIn: dc.FirstIn, LastOut: dc.LastOut,
				LateMinutes: dc.LateMinutes, WorkMinutes: dc.WorkMinutes, OvertimeMinutes: dc.OvertimeMinutes,
			}
			if leaveID != "" {
				lid := leaveID
				day.LeaveRequestID = &lid
			}
			if ss, se, ok := scheduledInstants(d, loc, schedules, key); ok {
				day.ScheduledStart = &ss
				day.ScheduledEnd = &se
			}
			if err := repositories.UpsertAttendanceDay(ctx, tx, &day); err != nil {
				return err
			}
		}
	}
	return nil
}

// scheduleForWeekday mengembalikan jadwal yang berlaku pada tanggal `d`.
func scheduleForWeekday(d time.Time, schedules []models.WorkSchedule, dateKey string) (models.WorkSchedule, bool) {
	wd := int(d.Weekday())
	var best models.WorkSchedule
	found := false
	for _, s := range schedules {
		if s.Weekday != wd {
			continue
		}
		if s.EffectiveFrom.Format("2006-01-02") > dateKey {
			continue
		}
		if s.EffectiveTo != nil && s.EffectiveTo.Format("2006-01-02") < dateKey {
			continue
		}
		if !found || s.EffectiveFrom.After(best.EffectiveFrom) {
			best, found = s, true
		}
	}
	return best, found
}

// scheduledInstants membangun waktu mulai/selesai terjadwal (UTC) untuk tanggal d.
func scheduledInstants(d time.Time, loc *time.Location, schedules []models.WorkSchedule, dateKey string) (time.Time, time.Time, bool) {
	s, ok := scheduleForWeekday(d, schedules, dateKey)
	if !ok || !s.IsWorkingDay || s.StartTime == nil || s.EndTime == nil {
		return time.Time{}, time.Time{}, false
	}
	midnight := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, loc)
	ss := midnight.Add(s.StartTime.Duration()).UTC()
	se := midnight.Add(s.EndTime.Duration()).UTC()
	if !se.After(ss) { // lewat tengah malam
		se = se.AddDate(0, 0, 1)
	}
	return ss, se, true
}

// computeDay menentukan status & menit satu hari, urutan TETAP:
// libur > cuti disetujui > ada absensi (present/late) > alpa/off.
func computeDay(d time.Time, key string, loc *time.Location, schedules []models.WorkSchedule,
	holidaySet map[string]bool, leaves []models.LeaveRequest, atts []models.Attendance,
	corr map[string]time.Time) (dayCalc, string) {

	if holidaySet[key] {
		return dayCalc{Status: "holiday"}, ""
	}
	for _, l := range leaves {
		if l.StartDate.Format("2006-01-02") <= key && l.EndDate.Format("2006-01-02") >= key {
			st := map[string]string{"leave": "leave", "sick": "sick", "permit": "permit", "unpaid": "permit"}[l.Kind]
			paid := l.IsPaid
			return dayCalc{Status: st, LeaveIsPaid: &paid}, l.ID
		}
	}

	var firstIn, lastOut *time.Time
	for _, a := range atts {
		if a.BusinessDate.Format("2006-01-02") != key {
			continue
		}
		at := a.OccurredAt
		if nc, ok := corr[a.ID]; ok {
			at = nc
		}
		if a.Kind == "in" {
			if firstIn == nil || at.Before(*firstIn) {
				v := at
				firstIn = &v
			}
		} else {
			if lastOut == nil || at.After(*lastOut) {
				v := at
				lastOut = &v
			}
		}
	}

	sched, hasSched := scheduleForWeekday(d, schedules, key)
	ss, se, schedOK := scheduledInstants(d, loc, schedules, key)

	if firstIn == nil && lastOut == nil {
		if hasSched && !sched.IsWorkingDay {
			return dayCalc{Status: "off"}, ""
		}
		if !schedOK {
			return dayCalc{Status: "off"}, "" // tak ada jadwal → tak bisa dianggap alpa
		}
		return dayCalc{Status: "absent"}, ""
	}

	dc := dayCalc{Status: "present", FirstIn: firstIn, LastOut: lastOut}
	if firstIn != nil && lastOut != nil {
		wm := int(lastOut.Sub(*firstIn).Minutes()) - sched.BreakMinutes
		if wm < 0 {
			wm = 0
		}
		dc.WorkMinutes = wm
	}
	if schedOK && firstIn != nil {
		tol := time.Duration(sched.LateToleranceMinutes) * time.Minute
		if firstIn.After(ss.Add(tol)) {
			dc.Status = "late"
			dc.LateMinutes = int(firstIn.Sub(ss).Minutes())
		}
		schedMinutes := int(se.Sub(ss).Minutes()) - sched.BreakMinutes
		if dc.WorkMinutes > schedMinutes && schedMinutes > 0 {
			dc.OvertimeMinutes = dc.WorkMinutes - schedMinutes
		}
	}
	return dc, ""
}

// ── DTO ───────────────────────────────────────────────────────────────────

func attendanceToResponse(a models.Attendance) structs.AttendanceResponse {
	return structs.AttendanceResponse{
		ID: a.ID, EmployeeID: a.EmployeeID, OutletID: a.OutletID, Kind: a.Kind,
		OccurredAt: a.OccurredAt.Format(saleTimeLayout), BusinessDate: a.BusinessDate.Format("2006-01-02"),
		Source: a.Source, PhotoURL: a.PhotoURL,
	}
}

func correctionToResponse(c models.AttendanceCorrection) structs.CorrectionResponse {
	r := structs.CorrectionResponse{
		ID: c.ID, AttendanceID: c.AttendanceID, Reason: c.Reason,
		NewOccurredAt: c.NewOccurredAt.Format(saleTimeLayout), Status: c.Status,
	}
	if c.ApprovedAt != nil {
		r.ApprovedAt = c.ApprovedAt.Format(saleTimeLayout)
	}
	return r
}

// AttendanceToResponse — untuk controller list.
func AttendanceToResponse(a models.Attendance) structs.AttendanceResponse {
	return attendanceToResponse(a)
}
