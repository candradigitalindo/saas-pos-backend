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
	if err := ensureOutletAccess(ctx, outletID); err != nil {
		return nil, err
	}
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
		if err := ensureOutletAccess(ctx, sh.OutletID); err != nil {
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

// HandoverShift menyerahkan kasir dari satu orang ke orang berikutnya: shift
// berjalan ditutup dan shift baru dibuka, DALAM SATU TRANSAKSI.
//
// Kenapa satu endpoint, bukan dua panggilan dari klien. Ada batasan satu shift
// terbuka per outlet, jadi "buka" akan selalu ditolak selama yang lama masih
// terbuka — urutannya wajib tutup dulu. Dan kalau urutan itu dijalankan klien,
// kegagalan di tengah meninggalkan kasir TANPA shift terbuka: uang laci sudah
// dihitung dan dibukukan, tapi tidak ada tempat mencatat penjualan berikutnya,
// dan kasir berikutnya menghadapi layar yang menolak melayani pembeli. Satu
// transaksi membuat keadaan setengah jadi itu mustahil.
//
// `openingCash` boleh berbeda dari `countedCash`: lazimnya sebagian uang laci
// disetor ke brankas saat pergantian, dan yang ditinggal hanya uang kembalian.
// Bila tidak diisi, seluruh uang yang dihitung diteruskan ke shift berikutnya.
//
// Yang membuka shift baru adalah PENGGUNA YANG SEDANG MASUK — orang yang
// berdiri di depan mesin saat ini, yang juga akan bertanggung jawab atas
// lacinya mulai detik ini.
func HandoverShift(
	ctx context.Context,
	shiftID string,
	countedCash int64,
	openingCash *int64,
	note string,
) (*models.Shift, *models.Shift, error) {
	if countedCash < 0 {
		return nil, nil, fmt.Errorf("%w: uang laci tidak boleh negatif", helpers.ErrValidation)
	}
	modalBaru := countedCash
	if openingCash != nil {
		if *openingCash < 0 {
			return nil, nil, fmt.Errorf("%w: modal awal tidak boleh negatif", helpers.ErrValidation)
		}
		if *openingCash > countedCash {
			return nil, nil, fmt.Errorf(
				"%w: modal awal (%d) tidak boleh lebih besar dari uang laci yang dihitung (%d)",
				helpers.ErrValidation, *openingCash, countedCash)
		}
		modalBaru = *openingCash
	}

	var lama, baru models.Shift
	err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		if err := repositories.FindShiftInTenant(ctx, tx, shiftID, &lama); err != nil {
			return err
		}
		if err := ensureOutletAccess(ctx, lama.OutletID); err != nil {
			return err
		}
		if lama.Status != "open" {
			return fmt.Errorf("%w: shift sudah ditutup", helpers.ErrConflict)
		}

		var outlet models.Outlet
		if err := repositories.FindOutletByID(ctx, tx, lama.OutletID, &outlet); err != nil {
			return err
		}

		cashSales, cashIn, cashOut, err := repositories.ShiftCashTotals(ctx, tx, shiftID)
		if err != nil {
			return err
		}
		expected := lama.OpeningCash + cashSales + cashIn - cashOut
		selisih := countedCash - expected
		now := time.Now().UTC()
		uid := reqctx.UserID(ctx)

		lama.Status = "closed"
		lama.ClosedBy = &uid
		lama.ClosedAt = &now
		lama.ExpectedCash = expected
		lama.CountedCash = &countedCash
		lama.Difference = &selisih
		if note != "" {
			lama.Note = note
		}
		if err := repositories.UpdateShift(ctx, tx, &lama); err != nil {
			return err
		}

		// Hari usaha dihitung ulang dari zona outlet, bukan diwarisi dari shift
		// lama: pergantian shift malam kerap melewati batas hari usaha, dan
		// mewarisinya akan membukukan penjualan besok ke hari kemarin.
		bizDate, err := timez.BusinessDate(now, outlet.Timezone, outlet.DayStartOffset())
		if err != nil {
			return err
		}
		baru = models.Shift{
			OutletID:     lama.OutletID,
			OpenedBy:     uid,
			OpenedAt:     now,
			BusinessDate: bizDate,
			OpeningCash:  modalBaru,
			Status:       "open",
		}
		return repositories.CreateShift(ctx, tx, &baru)
	})
	if err != nil {
		return nil, nil, err
	}
	return &lama, &baru, nil
}

// CreateCashMovement mencatat kas masuk/keluar non-penjualan pada shift terbuka
// outlet. shiftID opsional (kosong = shift terbuka outlet).
func CreateCashMovement(ctx context.Context, outletID, shiftID, direction string, amount int64, reason string) (*models.CashMovement, error) {
	if amount <= 0 {
		return nil, fmt.Errorf("%w: nominal harus lebih besar dari 0", helpers.ErrValidation)
	}
	if err := ensureOutletAccess(ctx, outletID); err != nil {
		return nil, err
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
