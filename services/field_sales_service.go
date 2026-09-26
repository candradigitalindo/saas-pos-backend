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

// Layanan CRM sales lapangan — rencana kunjungan, kunjungan, target, komisi
// (Fase 10, §5.9, blueprint E.3).
//
// Kunjungan dibuat perangkat (ULID) dan sering OFFLINE → UpsertVisit idempoten
// per id, dipakai baik oleh POST /visits maupun op sync 'visit.upsert'.
// business_date SELALU dihitung ulang di server dari zona outlet.

// visitBusinessDate menghitung tanggal usaha untuk waktu `at` memakai zona
// outlet pertama tenant.
func visitBusinessDate(ctx context.Context, tx *gorm.DB, at time.Time) (time.Time, error) {
	outletID, err := repositories.FirstOutletID(ctx, tx)
	if err != nil {
		return time.Time{}, err
	}
	var outlet models.Outlet
	if err := repositories.FindOutletByID(ctx, tx, outletID, &outlet); err != nil {
		return time.Time{}, err
	}
	return timez.BusinessDate(at.UTC(), outlet.Timezone, outlet.DayStartOffset())
}

// resolveVisitOwner menentukan pemilik data kunjungan: diri sendiri, atau orang
// lain hanya bila pemanggil pengawas (crm.commission.view).
func resolveVisitOwner(ctx context.Context, requested string) (string, error) {
	me := reqctx.UserID(ctx)
	if requested == "" || requested == me {
		return me, nil
	}
	if !reqctx.HasPermission(ctx, "crm.commission.view") {
		return "", fmt.Errorf("%w: tidak boleh membuat data untuk sales lain", helpers.ErrForbidden)
	}
	return requested, nil
}

// ── Visit plan ────────────────────────────────────────────────────────────

// CreateVisitPlan membuat rencana kunjungan + satu kunjungan 'pending' per toko.
func CreateVisitPlan(ctx context.Context, in structs.VisitPlanCreateRequest) (structs.VisitPlanResponse, error) {
	var out structs.VisitPlanResponse
	planDate, err := time.Parse("2006-01-02", in.PlanDate)
	if err != nil {
		return out, fmt.Errorf("%w: plan_date harus YYYY-MM-DD", helpers.ErrValidation)
	}

	err = repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		ownerID, oerr := resolveVisitOwner(ctx, in.OwnerID)
		if oerr != nil {
			return oerr
		}
		if _, ok, ferr := repositories.FindVisitPlanByOwnerDate(ctx, tx, ownerID, in.PlanDate); ferr != nil {
			return ferr
		} else if ok {
			return fmt.Errorf("%w: rencana kunjungan untuk tanggal itu sudah ada", helpers.ErrConflict)
		}

		plan := models.VisitPlan{OwnerID: ownerID, PlanDate: planDate, Status: "planned"}
		if err := repositories.CreateVisitPlan(ctx, tx, &plan); err != nil {
			return err
		}

		visits := make([]models.Visit, 0, len(in.CustomerIDs))
		for _, cid := range in.CustomerIDs {
			if err := repositories.FindCustomerInTenant(ctx, tx, cid, &models.Customer{}); err != nil {
				return fmt.Errorf("%w: pelanggan %s tidak ditemukan", helpers.ErrValidation, cid)
			}
			pid := plan.ID
			visits = append(visits, models.Visit{
				VisitPlanID: &pid, CustomerID: cid, OwnerID: ownerID,
				Result: "pending", BusinessDate: planDate,
			})
		}
		if err := repositories.CreateVisits(ctx, tx, visits); err != nil {
			return err
		}

		reloaded, err := repositories.FindVisitPlan(ctx, tx, plan.ID)
		if err != nil {
			return err
		}
		out = visitPlanToResponse(reloaded)
		return nil
	})
	return out, err
}

// ── Visit ─────────────────────────────────────────────────────────────────

// VisitUpsertInput adalah masukan UpsertVisit (dari POST /visits atau sync).
type VisitUpsertInput struct {
	ID            string
	VisitPlanID   string
	CustomerID    string
	CheckinAt     string
	CheckoutAt    string
	CheckinLat    string
	CheckinLng    string
	PhotoURL      string
	Result        string
	NoOrderReason string
	SaleID        string
}

// UpsertVisit membuat atau memperbarui satu kunjungan, idempoten per id klien.
// Push berulang atas kunjungan yang sama → satu baris (DoD Fase 10).
func UpsertVisit(ctx context.Context, in VisitUpsertInput) (structs.VisitResponse, error) {
	var out structs.VisitResponse
	err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		checkinAt, err := parseOptTime("checkin_at", in.CheckinAt)
		if err != nil {
			return err
		}
		checkoutAt, err := parseOptTime("checkout_at", in.CheckoutAt)
		if err != nil {
			return err
		}
		lat, err := parseOptDecimal("checkin_lat", in.CheckinLat)
		if err != nil {
			return err
		}
		lng, err := parseOptDecimal("checkin_lng", in.CheckinLng)
		if err != nil {
			return err
		}

		// Sudah ada? → update (jalur sync: tanpa lapis visibilitas).
		if in.ID != "" {
			if existing, ok, ferr := repositories.FindVisitInTenant(ctx, tx, in.ID); ferr != nil {
				return ferr
			} else if ok {
				applyVisitPatch(&existing, in, checkinAt, checkoutAt, lat, lng)
				if err := repositories.SaveVisit(ctx, tx, &existing); err != nil {
					return err
				}
				out = visitToResponse(existing)
				return nil
			}
		}

		// Baru.
		if err := repositories.FindCustomerInTenant(ctx, tx, in.CustomerID, &models.Customer{}); err != nil {
			return fmt.Errorf("%w: pelanggan tidak ditemukan", helpers.ErrValidation)
		}
		at := time.Now().UTC()
		if checkinAt != nil {
			at = *checkinAt
		}
		bizDate, err := visitBusinessDate(ctx, tx, at)
		if err != nil {
			return err
		}
		v := models.Visit{
			CustomerID:    in.CustomerID,
			OwnerID:       reqctx.UserID(ctx),
			CheckinAt:     checkinAt,
			CheckoutAt:    checkoutAt,
			CheckinLat:    lat,
			CheckinLng:    lng,
			PhotoURL:      in.PhotoURL,
			Result:        orDefault(in.Result, "pending"),
			NoOrderReason: in.NoOrderReason,
			BusinessDate:  bizDate,
		}
		if in.ID != "" {
			v.ID = in.ID
		}
		if in.VisitPlanID != "" {
			v.VisitPlanID = &in.VisitPlanID
		}
		if in.SaleID != "" {
			v.SaleID = &in.SaleID
		}
		if err := repositories.CreateVisit(ctx, tx, &v); err != nil {
			return err
		}
		out = visitToResponse(v)
		return nil
	})
	return out, err
}

// applyVisitPatch menimpa kolom mutable kunjungan yang ada dengan nilai `in`
// yang terisi.
func applyVisitPatch(v *models.Visit, in VisitUpsertInput, checkinAt, checkoutAt *time.Time, lat, lng *decimal.Decimal) {
	if in.VisitPlanID != "" {
		v.VisitPlanID = &in.VisitPlanID
	}
	if checkinAt != nil {
		v.CheckinAt = checkinAt
	}
	if checkoutAt != nil {
		v.CheckoutAt = checkoutAt
	}
	if lat != nil {
		v.CheckinLat = lat
	}
	if lng != nil {
		v.CheckinLng = lng
	}
	if in.PhotoURL != "" {
		v.PhotoURL = in.PhotoURL
	}
	if in.Result != "" {
		v.Result = in.Result
	}
	if in.NoOrderReason != "" {
		v.NoOrderReason = in.NoOrderReason
	}
	if in.SaleID != "" {
		v.SaleID = &in.SaleID
	}
}

// CheckoutVisit menutup kunjungan dengan hasilnya.
func CheckoutVisit(ctx context.Context, id string, in structs.VisitCheckoutRequest) (structs.VisitResponse, error) {
	var out structs.VisitResponse
	if in.Result == "no_order" && in.NoOrderReason == "" {
		return out, fmt.Errorf("%w: alasan wajib untuk kunjungan tanpa pesanan", helpers.ErrValidation)
	}
	err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		v, err := repositories.FindVisit(ctx, tx, id)
		if err != nil {
			return err
		}
		if v.CheckoutAt != nil {
			return fmt.Errorf("%w: kunjungan sudah di-check-out", helpers.ErrConflict)
		}
		at, err := parseOptTime("checkout_at", in.CheckoutAt)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		if at == nil {
			at = &now
		}
		v.CheckoutAt = at
		v.Result = in.Result
		v.NoOrderReason = in.NoOrderReason
		if in.PhotoURL != "" {
			v.PhotoURL = in.PhotoURL
		}
		if in.SaleID != "" {
			v.SaleID = &in.SaleID
		}
		if err := repositories.SaveVisit(ctx, tx, &v); err != nil {
			return err
		}
		reloaded, err := repositories.FindVisit(ctx, tx, id)
		if err != nil {
			return err
		}
		out = visitToResponse(reloaded)
		return nil
	})
	return out, err
}

// ── Sales target ──────────────────────────────────────────────────────────

// SetSalesTarget membuat/memperbarui target seorang sales untuk satu periode.
func SetSalesTarget(ctx context.Context, in structs.SalesTargetRequest) (structs.SalesTargetResponse, error) {
	var out structs.SalesTargetResponse
	ps, err := time.Parse("2006-01-02", in.PeriodStart)
	if err != nil {
		return out, fmt.Errorf("%w: period_start harus YYYY-MM-DD", helpers.ErrValidation)
	}
	pe, err := time.Parse("2006-01-02", in.PeriodEnd)
	if err != nil {
		return out, fmt.Errorf("%w: period_end harus YYYY-MM-DD", helpers.ErrValidation)
	}
	if pe.Before(ps) {
		return out, fmt.Errorf("%w: period_end lebih awal dari period_start", helpers.ErrValidation)
	}

	err = repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		existing, ok, ferr := repositories.FindTargetByUserPeriod(ctx, tx, in.UserID, in.PeriodStart)
		if ferr != nil {
			return ferr
		}
		t := models.SalesTarget{
			UserID: in.UserID, PeriodStart: ps, PeriodEnd: pe,
			TargetAmount: in.TargetAmount, TargetVisits: in.TargetVisits,
		}
		if ok {
			t.ID = existing.ID
			if err := repositories.SaveTarget(ctx, tx, &t); err != nil {
				return err
			}
		} else {
			if err := repositories.CreateTarget(ctx, tx, &t); err != nil {
				return err
			}
		}
		resp, err := targetToResponse(ctx, tx, t)
		if err != nil {
			return err
		}
		out = resp
		return nil
	})
	return out, err
}

// ListSalesTargets mengembalikan target (+ pencapaian) yang terlihat user konteks.
func ListSalesTargets(ctx context.Context, userID string, limit, offset int) ([]structs.SalesTargetResponse, int64, error) {
	rows, total, err := repositories.ListTargets(ctx, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	out := make([]structs.SalesTargetResponse, len(rows))
	for i, r := range rows {
		resp, rerr := targetToResponse(ctx, nil, r)
		if rerr != nil {
			return nil, 0, rerr
		}
		out[i] = resp
	}
	return out, total, nil
}

// ── Commission ────────────────────────────────────────────────────────────

// ComputeCommission menghitung komisi seorang sales untuk satu periode dari
// nilai TERTAGIH. Idempoten per (user, periode): memperbarui baris draft yang
// ada; menolak bila sudah 'approved'/'paid'.
func ComputeCommission(ctx context.Context, in structs.CommissionComputeRequest) (structs.CommissionResponse, error) {
	var out structs.CommissionResponse
	rate, err := decimal.NewFromString(in.Rate)
	if err != nil || rate.IsNegative() {
		return out, fmt.Errorf("%w: rate tidak valid", helpers.ErrValidation)
	}
	ps, err := time.Parse("2006-01-02", in.PeriodStart)
	if err != nil {
		return out, fmt.Errorf("%w: period_start harus YYYY-MM-DD", helpers.ErrValidation)
	}
	pe, err := time.Parse("2006-01-02", in.PeriodEnd)
	if err != nil {
		return out, fmt.Errorf("%w: period_end harus YYYY-MM-DD", helpers.ErrValidation)
	}

	err = repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		base, berr := repositories.CollectedByUser(ctx, tx, in.UserID, in.PeriodStart, in.PeriodEnd)
		if berr != nil {
			return berr
		}
		amount := helpers.RoundHalfUpToInt(decimal.NewFromInt(base).Mul(rate))

		existing, ok, ferr := repositories.FindCommissionByUserPeriod(ctx, tx, in.UserID, in.PeriodStart)
		if ferr != nil {
			return ferr
		}
		c := models.Commission{
			UserID: in.UserID, PeriodStart: ps, PeriodEnd: pe,
			BaseAmount: base, Rate: rate, Amount: amount, Status: "draft",
		}
		if ok {
			if existing.Status != "draft" {
				return fmt.Errorf("%w: komisi periode ini sudah %s", helpers.ErrConflict, existing.Status)
			}
			c.ID = existing.ID
			if err := repositories.SaveCommission(ctx, tx, &c); err != nil {
				return err
			}
		} else if err := repositories.CreateCommission(ctx, tx, &c); err != nil {
			return err
		}
		out = commissionToResponse(c)
		return nil
	})
	return out, err
}

// setCommissionStatus memindahkan status komisi (draft→approved→paid).
func setCommissionStatus(ctx context.Context, id, from, to string) (structs.CommissionResponse, error) {
	var out structs.CommissionResponse
	err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		c, err := repositories.FindCommission(ctx, tx, id)
		if err != nil {
			return err
		}
		if c.Status != from {
			return fmt.Errorf("%w: komisi harus berstatus %q", helpers.ErrConflict, from)
		}
		c.Status = to
		if err := repositories.SaveCommission(ctx, tx, &c); err != nil {
			return err
		}
		out = commissionToResponse(c)
		return nil
	})
	return out, err
}

// ApproveCommission & PayCommission.
func ApproveCommission(ctx context.Context, id string) (structs.CommissionResponse, error) {
	return setCommissionStatus(ctx, id, "draft", "approved")
}

func PayCommission(ctx context.Context, id string) (structs.CommissionResponse, error) {
	return setCommissionStatus(ctx, id, "approved", "paid")
}

// ── Helper parse ──────────────────────────────────────────────────────────

func parseOptTime(field, s string) (*time.Time, error) {
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil, fmt.Errorf("%w: %s harus RFC3339", helpers.ErrValidation, field)
	}
	u := t.UTC()
	return &u, nil
}

func parseOptDecimal(field, s string) (*decimal.Decimal, error) {
	if s == "" {
		return nil, nil
	}
	d, err := decimal.NewFromString(s)
	if err != nil {
		return nil, fmt.Errorf("%w: %s bukan angka", helpers.ErrValidation, field)
	}
	return &d, nil
}

// ── Pemetaan DTO ──────────────────────────────────────────────────────────

func visitPlanToResponse(p models.VisitPlan) structs.VisitPlanResponse {
	r := structs.VisitPlanResponse{
		ID: p.ID, OwnerID: p.OwnerID,
		PlanDate: p.PlanDate.Format("2006-01-02"), Status: p.Status,
	}
	for _, v := range p.Visits {
		r.Visits = append(r.Visits, visitToResponse(v))
	}
	return r
}

func visitToResponse(v models.Visit) structs.VisitResponse {
	r := structs.VisitResponse{
		ID: v.ID, CustomerID: v.CustomerID, OwnerID: v.OwnerID,
		PhotoURL: v.PhotoURL, Result: v.Result, NoOrderReason: v.NoOrderReason,
		BusinessDate: v.BusinessDate.Format("2006-01-02"),
	}
	if v.VisitPlanID != nil {
		r.VisitPlanID = *v.VisitPlanID
	}
	if v.CheckinAt != nil {
		r.CheckinAt = v.CheckinAt.UTC().Format(saleTimeLayout)
	}
	if v.CheckoutAt != nil {
		r.CheckoutAt = v.CheckoutAt.UTC().Format(saleTimeLayout)
	}
	if v.CheckinLat != nil {
		r.CheckinLat = v.CheckinLat.String()
	}
	if v.CheckinLng != nil {
		r.CheckinLng = v.CheckinLng.String()
	}
	if v.SaleID != nil {
		r.SaleID = *v.SaleID
	}
	return r
}

func targetToResponse(ctx context.Context, tx *gorm.DB, t models.SalesTarget) (structs.SalesTargetResponse, error) {
	from := t.PeriodStart.Format("2006-01-02")
	to := t.PeriodEnd.Format("2006-01-02")
	collected, err := repositories.CollectedByUser(ctx, tx, t.UserID, from, to)
	if err != nil {
		return structs.SalesTargetResponse{}, err
	}
	visits, err := repositories.CountVisits(ctx, tx, t.UserID, from, to, true)
	if err != nil {
		return structs.SalesTargetResponse{}, err
	}
	return structs.SalesTargetResponse{
		ID: t.ID, UserID: t.UserID, PeriodStart: from, PeriodEnd: to,
		TargetAmount: t.TargetAmount, TargetVisits: t.TargetVisits,
		AchievedAmount: collected, AchievedVisits: int(visits),
	}, nil
}

func commissionToResponse(c models.Commission) structs.CommissionResponse {
	return structs.CommissionResponse{
		ID: c.ID, UserID: c.UserID,
		PeriodStart: c.PeriodStart.Format("2006-01-02"), PeriodEnd: c.PeriodEnd.Format("2006-01-02"),
		BaseAmount: c.BaseAmount, Rate: c.Rate.String(), Amount: c.Amount, Status: c.Status,
	}
}

// VisitToResponse / VisitPlanToResponse / CommissionToResponse — untuk controller list.
func VisitToResponse(v models.Visit) structs.VisitResponse             { return visitToResponse(v) }
func VisitPlanToResponse(p models.VisitPlan) structs.VisitPlanResponse { return visitPlanToResponse(p) }
func CommissionToResponse(c models.Commission) structs.CommissionResponse {
	return commissionToResponse(c)
}
