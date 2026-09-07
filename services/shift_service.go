package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"candra/backend-api/helpers"
	"candra/backend-api/internal/reqctx"
	"candra/backend-api/internal/timez"
	"candra/backend-api/models"
	"candra/backend-api/repositories"

	"gorm.io/gorm"
)

// OpenShift membuka shift kas untuk sebuah outlet. Menolak (ErrConflict) bila
// outlet sudah punya shift terbuka — dijamin ganda oleh partial unique index.
func OpenShift(ctx context.Context, outletID string, openingCash int64, note string) (*models.Shift, error) {
	var outlet models.Outlet
	if err := repositories.FindOutletByID(ctx, nil, outletID, &outlet); err != nil {
		return nil, fmt.Errorf("%w: outlet tidak ditemukan", helpers.ErrValidation)
	}
	now := time.Now().UTC()
	bizDate, err := timez.BusinessDate(now, outlet.Timezone, outlet.DayStartOffset())
	if err != nil {
		return nil, err
	}

	shift := models.Shift{
		OutletID:     outletID,
		OpenedBy:     reqctx.UserID(ctx),
		OpenedAt:     now,
		BusinessDate: bizDate,
		OpeningCash:  openingCash,
		Note:         note,
		Status:       "open",
	}
	err = repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		return repositories.CreateShift(ctx, tx, &shift)
	})
	if err != nil {
		if helpers.IsDuplicateEntryError(err) {
			return nil, fmt.Errorf("%w: sudah ada shift terbuka untuk outlet ini", helpers.ErrConflict)
		}
		return nil, err
	}
	return &shift, nil
}

// CloseShift menutup shift: menghitung expected_cash dari transaksi & gerakan
// kas, menyimpan hasil hitung fisik (counted) dan selisihnya.
//
//	expected = opening_cash + Σ(pembayaran tunai) + Σ(kas masuk) - Σ(kas keluar)
//	difference = counted - expected
func CloseShift(ctx context.Context, shiftID string, countedCash int64, note string) (*models.Shift, error) {
	var out models.Shift
	err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		var sh models.Shift
		if err := repositories.FindShiftInTenant(ctx, tx, shiftID, &sh); err != nil {
			return err
		}
		if sh.Status != "open" {
			return fmt.Errorf("%w: shift sudah ditutup", helpers.ErrConflict)
		}

		cashSales, cashIn, cashOut, err := repositories.ShiftCashTotals(ctx, tx, shiftID)
		if err != nil {
			return err
		}
		expected := sh.OpeningCash + cashSales + cashIn - cashOut
		difference := countedCash - expected
		now := time.Now().UTC()
		uid := reqctx.UserID(ctx)

		sh.Status = "closed"
		sh.ClosedBy = &uid
		sh.ClosedAt = &now
		sh.ExpectedCash = expected
		sh.CountedCash = &countedCash
		sh.Difference = &difference
		if note != "" {
			sh.Note = note
		}
		if err := repositories.UpdateShift(ctx, tx, &sh); err != nil {
			return err
		}
		out = sh
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateCashMovement mencatat kas masuk/keluar non-penjualan pada shift terbuka
// outlet. shiftID opsional (kosong = shift terbuka outlet).
func CreateCashMovement(ctx context.Context, outletID, shiftID, direction string, amount int64, reason string) (*models.CashMovement, error) {
	if amount <= 0 {
		return nil, fmt.Errorf("%w: nominal harus lebih besar dari 0", helpers.ErrValidation)
	}
	var mv models.CashMovement
	err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		var sh models.Shift
		if shiftID != "" {
			if err := repositories.FindShiftInTenant(ctx, tx, shiftID, &sh); err != nil {
				return err
			}
			if sh.OutletID != outletID || sh.Status != "open" {
				return fmt.Errorf("%w: shift tidak terbuka untuk outlet ini", helpers.ErrValidation)
			}
		} else if err := repositories.FindOpenShift(ctx, tx, outletID, &sh); err != nil {
			if errors.Is(err, repositories.ErrNoOpenShift) {
				return fmt.Errorf("%w: belum ada shift terbuka untuk outlet ini", helpers.ErrValidation)
			}
			return err
		}

		var outlet models.Outlet
		if err := repositories.FindOutletByID(ctx, tx, outletID, &outlet); err != nil {
			return err
		}
		now := time.Now().UTC()
		bizDate, err := timez.BusinessDate(now, outlet.Timezone, outlet.DayStartOffset())
		if err != nil {
			return err
		}

		mv = models.CashMovement{
			OutletID:     outletID,
			ShiftID:      sh.ID,
			Direction:    direction,
			Amount:       amount,
			Reason:       reason,
			OccurredAt:   now,
			BusinessDate: bizDate,
			CreatedBy:    reqctx.UserID(ctx),
		}
		return repositories.CreateCashMovement(ctx, tx, &mv)
	})
	if err != nil {
		return nil, err
	}
	return &mv, nil
}
