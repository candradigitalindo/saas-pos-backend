package services

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"candra/backend-api/helpers"
	"candra/backend-api/internal/reqctx"
	"candra/backend-api/models"
	"candra/backend-api/repositories"
	"candra/backend-api/structs"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// Layanan penggajian (§5.11, §13.6).
//
// Yang membuatnya akurat: URUTAN YANG TETAP (base wage → aturan earning →
// aturan deduction → penyesuaian periode lalu → cicilan kasbon), SNAPSHOT
// nama/tipe/params di tiap baris slip, dan PENGUNCIAN. Menghitung periode yang
// sama dua kali dari data masukan yang sama → angka & baris IDENTIK.

// monthlyHoursDivisor: konstanta jam kerja bulanan untuk konversi upah/jam dari
// upah bulanan (aturan upah_lembur mode multiplier).
const monthlyHoursDivisor = 173

// ── Payroll rule CRUD ─────────────────────────────────────────────────────

// CreatePayrollRule menyimpan aturan gaji; params divalidasi per tipe.
func CreatePayrollRule(ctx context.Context, in structs.PayrollRuleRequest) (structs.PayrollRuleResponse, error) {
	var out structs.PayrollRuleResponse
	if !contains(models.PayrollRuleTypes, in.Type) {
		return out, fmt.Errorf("%w: tipe aturan tidak dikenal", helpers.ErrValidation)
	}
	if in.Category != "earning" && in.Category != "deduction" {
		return out, fmt.Errorf("%w: category harus earning/deduction", helpers.ErrValidation)
	}
	if err := validateRuleParams(in.Type, in.Params); err != nil {
		return out, err
	}
	ef, err := time.Parse("2006-01-02", in.EffectiveFrom)
	if err != nil {
		return out, fmt.Errorf("%w: effective_from harus YYYY-MM-DD", helpers.ErrValidation)
	}

	err = repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		r := models.PayrollRule{
			Code: in.Code, Name: in.Name, Type: in.Type, Category: in.Category,
			Params: in.Params, TargetType: orDefault(in.TargetType, "all"),
			Priority: in.Priority, EffectiveFrom: ef, IsActive: true,
		}
		if r.Priority == 0 {
			r.Priority = 100
		}
		if in.TargetID != "" {
			r.TargetID = &in.TargetID
		}
		if in.EffectiveTo != "" {
			et, perr := time.Parse("2006-01-02", in.EffectiveTo)
			if perr != nil {
				return fmt.Errorf("%w: effective_to harus YYYY-MM-DD", helpers.ErrValidation)
			}
			r.EffectiveTo = &et
		}
		if err := repositories.CreatePayrollRule(ctx, tx, &r); err != nil {
			if helpers.IsDuplicateEntryError(err) {
				return fmt.Errorf("%w: aturan dengan code & effective_from itu sudah ada", helpers.ErrConflict)
			}
			return err
		}
		out = payrollRuleToResponse(r)
		return nil
	})
	return out, err
}

// validateRuleParams memvalidasi bentuk params per tipe (kontrak §5.11).
func validateRuleParams(ruleType string, raw json.RawMessage) error {
	var m map[string]any
	if len(raw) == 0 || json.Unmarshal(raw, &m) != nil {
		return fmt.Errorf("%w: params harus objek JSON", helpers.ErrValidation)
	}
	need := func(keys ...string) error {
		for _, k := range keys {
			if _, ok := m[k]; !ok {
				return fmt.Errorf("%w: params.%s wajib untuk tipe %s", helpers.ErrValidation, k, ruleType)
			}
		}
		return nil
	}
	switch ruleType {
	case "tunjangan_tetap":
		return need("amount", "per")
	case "potongan_telat":
		return need("threshold_minutes", "mode", "amount")
	case "potongan_alpa":
		return need("mode")
	case "upah_lembur":
		return need("mode")
	case "bonus_kehadiran":
		return need("max_absent", "max_late", "amount")
	case "bonus_target", "potongan_kasbon", "komponen_manual":
		return nil
	}
	return nil
}

// ── Payroll period lifecycle ──────────────────────────────────────────────

// CreatePayrollPeriod membuat periode gaji draft.
func CreatePayrollPeriod(ctx context.Context, in structs.PayrollPeriodRequest) (structs.PayrollPeriodResponse, error) {
	var out structs.PayrollPeriodResponse
	sd, err := time.Parse("2006-01-02", in.StartDate)
	if err != nil {
		return out, fmt.Errorf("%w: start_date harus YYYY-MM-DD", helpers.ErrValidation)
	}
	ed, err := time.Parse("2006-01-02", in.EndDate)
	if err != nil {
		return out, fmt.Errorf("%w: end_date harus YYYY-MM-DD", helpers.ErrValidation)
	}
	if ed.Before(sd) {
		return out, fmt.Errorf("%w: end_date lebih awal dari start_date", helpers.ErrValidation)
	}
	if !contains(models.PayrollPeriodTypes, in.PeriodType) {
		return out, fmt.Errorf("%w: period_type tidak dikenal", helpers.ErrValidation)
	}

	err = repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		p := models.PayrollPeriod{
			PeriodType: in.PeriodType, StartDate: sd, EndDate: ed, Status: "draft",
		}
		if in.OutletID != "" {
			p.OutletID = &in.OutletID
		}
		if err := repositories.CreatePayrollPeriod(ctx, tx, &p); err != nil {
			if helpers.IsDuplicateEntryError(err) {
				return fmt.Errorf("%w: periode dengan tipe & tanggal mulai itu sudah ada", helpers.ErrConflict)
			}
			return err
		}
		out = payrollPeriodToResponse(p)
		return nil
	})
	return out, err
}

// CalculatePayroll menghitung (atau menghitung ULANG) seluruh slip periode.
// Ditolak bila periode sudah locked/paid (§13.6).
func CalculatePayroll(ctx context.Context, periodID string) (structs.PayrollPeriodResponse, error) {
	var out structs.PayrollPeriodResponse
	err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		p, err := repositories.LockPayrollPeriod(ctx, tx, periodID)
		if err != nil {
			return err
		}
		if p.Status == "locked" || p.Status == "paid" {
			return fmt.Errorf("%w: periode sudah dikunci — tidak bisa dihitung ulang", helpers.ErrConflict)
		}

		// Reset agar hitung ulang bersih & deterministik.
		if err := repositories.ClearAppliedAdjustmentsForPeriod(ctx, tx, p.ID); err != nil {
			return err
		}
		if err := repositories.DeleteRepaymentsForPeriod(ctx, tx, p.ID); err != nil {
			return err
		}

		outletID := ""
		if p.OutletID != nil {
			outletID = *p.OutletID
		}
		if err := RebuildAttendanceDays(ctx, tx, outletID, p.StartDate, p.EndDate); err != nil {
			return err
		}

		emps, err := repositories.ActiveEmployeesForPeriod(ctx, tx, outletID, p.StartDate, p.EndDate)
		if err != nil {
			return err
		}
		sort.Slice(emps, func(i, j int) bool { return emps[i].ID < emps[j].ID })

		var totGross, totDed, totNet int64
		for _, emp := range emps {
			slip, lines, err := calcEmployeeSlip(ctx, tx, p, emp)
			if err != nil {
				return err
			}
			if err := repositories.ReplacePayslip(ctx, tx, slip, lines); err != nil {
				return err
			}
			// Tandai penyesuaian terpakai + catat cicilan kasbon.
			if err := applyPostSlipEffects(ctx, tx, p, emp, slip, lines); err != nil {
				return err
			}
			totGross += slip.GrossAmount
			totDed += slip.DeductionAmount
			totNet += slip.NetAmount
		}

		now := time.Now().UTC()
		p.Status = "calculated"
		p.CalculatedAt = &now
		p.TotalGross, p.TotalDeduction, p.TotalNet = totGross, totDed, totNet
		if err := repositories.SavePayrollPeriod(ctx, tx, &p); err != nil {
			return err
		}
		out = payrollPeriodToResponse(p)
		return nil
	})
	return out, err
}

// LockPayroll mengunci periode & seluruh slipnya. Setelah ini hitung ulang ditolak.
func LockPayroll(ctx context.Context, periodID string) (structs.PayrollPeriodResponse, error) {
	var out structs.PayrollPeriodResponse
	err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		p, err := repositories.LockPayrollPeriod(ctx, tx, periodID)
		if err != nil {
			return err
		}
		if p.Status != "calculated" {
			return fmt.Errorf("%w: hanya periode berstatus calculated yang bisa dikunci", helpers.ErrConflict)
		}
		now := time.Now().UTC()
		uid := reqctx.UserID(ctx)
		p.Status = "locked"
		p.LockedAt = &now
		p.LockedBy = &uid
		if err := repositories.SavePayrollPeriod(ctx, tx, &p); err != nil {
			return err
		}
		if err := repositories.MarkPayslipsStatus(ctx, tx, p.ID, "locked", nil); err != nil {
			return err
		}
		out = payrollPeriodToResponse(p)
		return nil
	})
	return out, err
}

// PayPayroll membayar periode terkunci: kas keluar (ref_table='payroll_periods')
// + slip → paid.
func PayPayroll(ctx context.Context, periodID string) (structs.PayrollPeriodResponse, error) {
	var out structs.PayrollPeriodResponse
	err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		p, err := repositories.LockPayrollPeriod(ctx, tx, periodID)
		if err != nil {
			return err
		}
		if p.Status != "locked" {
			return fmt.Errorf("%w: hanya periode terkunci yang bisa dibayar", helpers.ErrConflict)
		}
		outletID := ""
		if p.OutletID != nil {
			outletID = *p.OutletID
		}
		if outletID == "" {
			oid, oerr := repositories.FirstOutletID(ctx, tx)
			if oerr != nil {
				return oerr
			}
			outletID = oid
		}
		now := time.Now().UTC()
		if p.TotalNet > 0 {
			if _, err := repositories.InsertPayrollCashOut(ctx, tx, outletID, p.TotalNet,
				"Pembayaran gaji periode "+p.StartDate.Format("2006-01-02"), "payroll_periods", p.ID, now); err != nil {
				return err
			}
		}
		p.Status = "paid"
		p.PaidAt = &now
		if err := repositories.SavePayrollPeriod(ctx, tx, &p); err != nil {
			return err
		}
		if err := repositories.MarkPayslipsStatus(ctx, tx, p.ID, "paid", &now); err != nil {
			return err
		}
		out = payrollPeriodToResponse(p)
		return nil
	})
	return out, err
}

// ── Perhitungan satu slip ─────────────────────────────────────────────────

func calcEmployeeSlip(ctx context.Context, tx *gorm.DB, p models.PayrollPeriod, emp models.Employee) (*models.Payslip, []models.PayslipLine, error) {
	days, err := repositories.AttendanceDaysForRange(ctx, tx, emp.ID, p.StartDate, p.EndDate)
	if err != nil {
		return nil, nil, err
	}
	roleID := ""
	if emp.UserID != nil {
		var u models.User
		if uerr := repositories.FindUserInTenant(ctx, tx, *emp.UserID, &u); uerr == nil {
			roleID = u.RoleID
		}
	}
	rules, err := repositories.EffectiveRulesForEmployee(ctx, tx, emp, roleID, p.StartDate, p.EndDate)
	if err != nil {
		return nil, nil, err
	}

	st := summariseDays(days, p)
	var lines []models.PayslipLine
	sort_ := 0
	push := func(l models.PayslipLine) {
		if l.Amount == 0 && l.RuleType != "base" {
			return // baris nol tidak ditampilkan
		}
		l.SortOrder = sort_
		sort_++
		lines = append(lines, l)
	}

	// 1. Upah dasar.
	push(basicWageLine(emp, st, p))

	// 2. Aturan earning lalu deduction (rules sudah urut category, priority, code).
	hasKasbonRule := false
	for _, r := range rules {
		if r.Type == "potongan_kasbon" {
			hasKasbonRule = true
			continue
		}
		if l, ok := applyRule(r, emp, st, p); ok {
			push(l)
		}
	}

	// 3. Penyesuaian dari periode sebelumnya.
	adjs, err := repositories.PendingAdjustmentsForEmployee(ctx, tx, emp.ID, p.ID)
	if err != nil {
		return nil, nil, err
	}
	for _, a := range adjs {
		push(models.PayslipLine{
			Name: a.Name, Category: a.Category, RuleType: "adjustment",
			BasisNote: "Penyesuaian periode sebelumnya", Amount: a.Amount,
		})
	}

	// 4. Cicilan kasbon berjalan (hanya bila ada aturan potongan_kasbon aktif).
	if hasKasbonRule {
		if adv, ok, aerr := repositories.OpenAdvanceForEmployee(ctx, tx, emp.ID); aerr != nil {
			return nil, nil, aerr
		} else if ok {
			inst := adv.InstallmentAmount
			if inst > adv.Remaining {
				inst = adv.Remaining
			}
			push(models.PayslipLine{
				Name: "Potongan kasbon", Category: "deduction", RuleType: "potongan_kasbon",
				BasisNote: fmt.Sprintf("cicilan Rp%d dari sisa Rp%d", inst, adv.Remaining), Amount: inst,
			})
		}
	}

	gross, ded := int64(0), int64(0)
	for _, l := range lines {
		if l.Category == "earning" {
			gross += l.Amount
		} else {
			ded += l.Amount
		}
	}
	net, carried := gross-ded, int64(0)
	if net < 0 {
		carried, net = -net, 0
	}

	slip := &models.Payslip{
		PayrollPeriodID: p.ID, EmployeeID: emp.ID,
		GrossAmount: gross, DeductionAmount: ded, NetAmount: net, CarriedDebt: carried,
		PresentDays:     decimal.NewFromInt(int64(st.presentDays)),
		LateCount:       st.lateCount,
		AbsentDays:      decimal.NewFromInt(int64(st.absentDays)),
		LeaveDays:       decimal.NewFromInt(int64(st.leaveDays)),
		OvertimeMinutes: st.overtimeMinutes,
		Status:          "draft",
	}
	return slip, lines, nil
}

// dayStats merangkum attendance_days satu karyawan pada periode.
type dayStats struct {
	presentDays     int // present + late
	lateCount       int
	lateMinutes     int
	absentDays      int
	leaveDays       int
	paidLeaveDays   int
	holidayDays     int
	overtimeMinutes int
	workingDays     int // hari yang dijadwalkan kerja (present+late+absent)
	perDayLate      []int
}

func summariseDays(days []models.AttendanceDay, p models.PayrollPeriod) dayStats {
	var s dayStats
	for _, d := range days {
		switch d.Status {
		case "present":
			s.presentDays++
			s.workingDays++
		case "late":
			s.presentDays++
			s.lateCount++
			s.lateMinutes += d.LateMinutes
			s.perDayLate = append(s.perDayLate, d.LateMinutes)
			s.workingDays++
		case "absent":
			s.absentDays++
			s.workingDays++
		case "leave", "sick", "permit":
			s.leaveDays++
		case "holiday":
			s.holidayDays++
		}
		s.overtimeMinutes += d.OvertimeMinutes
	}
	return s
}

// periodCalendarDays menghitung jumlah hari kalender periode.
func periodCalendarDays(p models.PayrollPeriod) int {
	return int(p.EndDate.Sub(p.StartDate).Hours()/24) + 1
}

// basicWageLine — upah dasar sesuai wage_type; bulanan diprorata bila karyawan
// masuk/keluar di tengah periode.
func basicWageLine(emp models.Employee, st dayStats, p models.PayrollPeriod) models.PayslipLine {
	l := models.PayslipLine{Name: "Upah dasar", Category: "earning", RuleType: "base"}
	switch emp.WageType {
	case "daily":
		amt := emp.BaseWage * int64(st.presentDays)
		l.Amount = amt
		q := decimal.NewFromInt(int64(st.presentDays))
		l.Quantity = &q
		l.BasisNote = fmt.Sprintf("%d hari hadir x Rp%d", st.presentDays, emp.BaseWage)
	case "hourly":
		worked := st.overtimeMinutes // fallback; hourly rarely used — pakai present days x 8j bila tak ada work_minutes
		_ = worked
		hours := decimal.NewFromInt(int64(st.presentDays * 8))
		l.Amount = helpers.RoundHalfUpToInt(hours.Mul(decimal.NewFromInt(emp.BaseWage)))
		l.Quantity = &hours
		l.BasisNote = fmt.Sprintf("%s jam x Rp%d", hours.String(), emp.BaseWage)
	default: // monthly
		total := periodCalendarDays(p)
		active := activeCalendarDays(emp, p)
		if active >= total {
			l.Amount = emp.BaseWage
			l.BasisNote = "gaji bulanan penuh"
		} else {
			l.Amount = helpers.RoundHalfUpToInt(decimal.NewFromInt(emp.BaseWage).Mul(decimal.NewFromInt(int64(active))).Div(decimal.NewFromInt(int64(total))))
			l.BasisNote = fmt.Sprintf("prorata %d/%d hari", active, total)
		}
	}
	return l
}

func activeCalendarDays(emp models.Employee, p models.PayrollPeriod) int {
	start := p.StartDate
	if emp.JoinedAt.After(start) {
		start = emp.JoinedAt
	}
	end := p.EndDate
	if emp.ResignedAt != nil && emp.ResignedAt.Before(end) {
		end = *emp.ResignedAt
	}
	if end.Before(start) {
		return 0
	}
	return int(end.Sub(start).Hours()/24) + 1
}

// dailyWage menghitung upah harian acuan (untuk potongan alpa).
func dailyWage(emp models.Employee, st dayStats, p models.PayrollPeriod) int64 {
	if emp.WageType == "daily" || emp.WageType == "hourly" {
		return emp.BaseWage
	}
	div := st.workingDays
	if div == 0 {
		div = 22 // asumsi hari kerja bila jadwal belum ada
	}
	return helpers.RoundHalfUpToInt(decimal.NewFromInt(emp.BaseWage).Div(decimal.NewFromInt(int64(div))))
}

// applyRule menghitung satu baris dari sebuah aturan. ok=false → aturan tak
// menghasilkan baris (nilai nol / tak berlaku).
func applyRule(r models.PayrollRule, emp models.Employee, st dayStats, p models.PayrollPeriod) (models.PayslipLine, bool) {
	var m map[string]any
	_ = json.Unmarshal(r.Params, &m)
	num := func(k string) decimal.Decimal {
		switch v := m[k].(type) {
		case float64:
			return decimal.NewFromFloat(v)
		case string:
			d, _ := decimal.NewFromString(v)
			return d
		}
		return decimal.Zero
	}
	line := models.PayslipLine{
		Name: r.Name, Category: r.Category, RuleType: r.Type,
		ParamsSnapshot: r.Params,
	}
	rid := r.ID
	line.RuleID = &rid

	switch r.Type {
	case "tunjangan_tetap":
		amount := num("amount").IntPart()
		per, _ := m["per"].(string)
		reqPresent, _ := m["require_present"].(bool)
		if per == "period" {
			line.Amount = amount
			line.BasisNote = "tunjangan per periode"
		} else {
			days := st.presentDays
			if !reqPresent {
				days = st.workingDays
			}
			line.Amount = amount * int64(days)
			q := decimal.NewFromInt(int64(days))
			line.Quantity = &q
			line.BasisNote = fmt.Sprintf("%d hari x Rp%d", days, amount)
		}

	case "potongan_telat":
		threshold := int(num("threshold_minutes").IntPart())
		mode, _ := m["mode"].(string)
		amount := num("amount").IntPart()
		var total int64
		var events int
		for _, lm := range st.perDayLate {
			if lm <= threshold {
				continue
			}
			events++
			switch mode {
			case "per_minute":
				total += int64(lm-threshold) * amount
			default: // per_event / tiered (tiered disederhanakan ke per_event)
				total += amount
			}
		}
		line.Amount = total
		q := decimal.NewFromInt(int64(events))
		line.Quantity = &q
		line.BasisNote = fmt.Sprintf("%d kali telat > %d menit x Rp%d", events, threshold, amount)

	case "potongan_alpa":
		mode, _ := m["mode"].(string)
		if mode == "daily_wage_percent" {
			pct := num("percent")
			dw := dailyWage(emp, st, p)
			line.Amount = helpers.RoundHalfUpToInt(decimal.NewFromInt(dw).Mul(pct).Mul(decimal.NewFromInt(int64(st.absentDays))))
			line.BasisNote = fmt.Sprintf("%d hari alpa x %s x upah harian Rp%d", st.absentDays, pct.String(), dw)
		} else {
			amount := num("amount").IntPart()
			line.Amount = amount * int64(st.absentDays)
			line.BasisNote = fmt.Sprintf("%d hari alpa x Rp%d", st.absentDays, amount)
		}
		q := decimal.NewFromInt(int64(st.absentDays))
		line.Quantity = &q

	case "upah_lembur":
		mode, _ := m["mode"].(string)
		ot := decimal.NewFromInt(int64(st.overtimeMinutes)).Div(decimal.NewFromInt(60))
		if mode == "hourly_rate" {
			line.Amount = helpers.RoundHalfUpToInt(ot.Mul(num("rate")))
		} else {
			hourlyBase := decimal.NewFromInt(emp.BaseWage).Div(decimal.NewFromInt(monthlyHoursDivisor))
			line.Amount = helpers.RoundHalfUpToInt(ot.Mul(hourlyBase).Mul(num("multiplier")))
		}
		line.Quantity = &ot
		line.BasisNote = fmt.Sprintf("%s jam lembur", ot.StringFixed(2))

	case "bonus_kehadiran":
		maxAbsent := int(num("max_absent").IntPart())
		maxLate := int(num("max_late").IntPart())
		if st.absentDays <= maxAbsent && st.lateCount <= maxLate {
			line.Amount = num("amount").IntPart()
			line.BasisNote = "target kehadiran tercapai"
		}

	default: // bonus_target, komponen_manual — belum didukung di fondasi 11a payroll
		return line, false
	}

	return line, line.Amount != 0
}

// applyPostSlipEffects menandai penyesuaian terpakai & mencatat cicilan kasbon.
func applyPostSlipEffects(ctx context.Context, tx *gorm.DB, p models.PayrollPeriod, emp models.Employee, slip *models.Payslip, lines []models.PayslipLine) error {
	adjs, err := repositories.PendingAdjustmentsForEmployee(ctx, tx, emp.ID, p.ID)
	if err != nil {
		return err
	}
	for _, a := range adjs {
		if err := repositories.MarkAdjustmentApplied(ctx, tx, a.ID, slip.ID); err != nil {
			return err
		}
	}
	for _, l := range lines {
		if l.RuleType != "potongan_kasbon" || l.Amount <= 0 {
			continue
		}
		adv, ok, aerr := repositories.OpenAdvanceForEmployee(ctx, tx, emp.ID)
		if aerr != nil {
			return aerr
		}
		if !ok {
			continue
		}
		pid := slip.ID
		if err := repositories.CreateAdvanceRepayment(ctx, tx, &models.AdvanceRepayment{
			AdvanceID: adv.ID, PayslipID: &pid, Amount: l.Amount,
			PaidAt: time.Now().UTC(), BusinessDate: p.EndDate,
		}); err != nil {
			return err
		}
		adv.Remaining -= l.Amount
		if adv.Remaining <= 0 {
			adv.Remaining = 0
			adv.Status = "settled"
		}
		if err := repositories.SaveAdvance(ctx, tx, &adv); err != nil {
			return err
		}
	}
	return nil
}

// ── DTO ───────────────────────────────────────────────────────────────────

func payrollRuleToResponse(r models.PayrollRule) structs.PayrollRuleResponse {
	res := structs.PayrollRuleResponse{
		ID: r.ID, Code: r.Code, Name: r.Name, Type: r.Type, Category: r.Category,
		Params: r.Params, TargetType: r.TargetType, Priority: r.Priority,
		EffectiveFrom: r.EffectiveFrom.Format("2006-01-02"), IsActive: r.IsActive,
	}
	if r.TargetID != nil {
		res.TargetID = *r.TargetID
	}
	if r.EffectiveTo != nil {
		res.EffectiveTo = r.EffectiveTo.Format("2006-01-02")
	}
	return res
}

func payrollPeriodToResponse(p models.PayrollPeriod) structs.PayrollPeriodResponse {
	res := structs.PayrollPeriodResponse{
		ID: p.ID, PeriodType: p.PeriodType,
		StartDate: p.StartDate.Format("2006-01-02"), EndDate: p.EndDate.Format("2006-01-02"),
		Status: p.Status, TotalGross: p.TotalGross, TotalDeduction: p.TotalDeduction, TotalNet: p.TotalNet,
	}
	if p.OutletID != nil {
		res.OutletID = *p.OutletID
	}
	if p.CalculatedAt != nil {
		res.CalculatedAt = p.CalculatedAt.Format(saleTimeLayout)
	}
	if p.LockedAt != nil {
		res.LockedAt = p.LockedAt.Format(saleTimeLayout)
	}
	if p.PaidAt != nil {
		res.PaidAt = p.PaidAt.Format(saleTimeLayout)
	}
	return res
}

// PayslipToResponse memetakan slip + baris ke DTO.
func PayslipToResponse(s models.Payslip) structs.PayslipResponse {
	r := structs.PayslipResponse{
		ID: s.ID, PayrollPeriodID: s.PayrollPeriodID, EmployeeID: s.EmployeeID,
		GrossAmount: s.GrossAmount, DeductionAmount: s.DeductionAmount, NetAmount: s.NetAmount,
		CarriedDebt: s.CarriedDebt, PresentDays: s.PresentDays.String(), LateCount: s.LateCount,
		AbsentDays: s.AbsentDays.String(), LeaveDays: s.LeaveDays.String(),
		OvertimeMinutes: s.OvertimeMinutes, Status: s.Status,
	}
	for _, l := range s.Lines {
		lr := structs.PayslipLineResponse{
			Name: l.Name, Category: l.Category, RuleType: l.RuleType,
			BasisNote: l.BasisNote, Amount: l.Amount, SortOrder: l.SortOrder,
		}
		if l.Quantity != nil {
			lr.Quantity = l.Quantity.String()
		}
		r.Lines = append(r.Lines, lr)
	}
	return r
}
