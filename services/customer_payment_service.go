package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"candra/backend-api/config"
	"candra/backend-api/helpers"
	"candra/backend-api/internal/reqctx"
	"candra/backend-api/internal/timez"
	"candra/backend-api/models"
	"candra/backend-api/repositories"
	"candra/backend-api/structs"

	"gorm.io/gorm"
)

// ReceivableDueSoonDays: batas "kasbon segera jatuh tempo" (hari) di layar
// Kasbon & Beranda.
func ReceivableDueSoonDays() int { return config.GetIntEnv("RECEIVABLE_DUE_SOON_DAYS", 3) }

const idempotencyScopeCustomerPayment = "receivable.payment.customer"

// CustomerPaymentInput: setoran seorang pelanggan atas kasbonnya.
type CustomerPaymentInput struct {
	CustomerID     string
	Amount         int64
	Method         string
	Source         string // tunai: SumberLaci | SumberLain; non-tunai diabaikan
	OutletID       string // toko tempat setoran diterima (wajib bila ke laci)
	Note           string
	IdempotencyKey string
	RequestHash    string
}

// PayCustomerReceivables mencatat setoran pelanggan atas SEMUA kasbonnya
// yang belum lunas, dipakai melunasi yang TERLAMA dulu. Pelanggan berpikir
// "utang saya Rp 250.000, saya bayar Rp 100.000" — bukan per nota; kasir
// tidak perlu membagi uangnya sendiri ke nota-nota.
//
// Setoran TUNAI yang diterima di kasir bisa masuk LACI: dicatat sebagai SATU
// uang masuk pada shift yang sedang buka di toko itu, supaya hitungan laci
// saat tutup shift tetap cocok. Tidak butuh izin uang masuk & keluar — uang
// bertambah, tidak ada yang diambil dari laci; izin kasbon (rute) sudah cukup.
//
// Seluruh kasbon pelanggan DIKUNCI: dua setoran bersamaan tidak boleh
// sama-sama melunasi sisa yang sama. Wajib Idempotency-Key.
func PayCustomerReceivables(ctx context.Context, in CustomerPaymentInput) (int, []byte, error) {
	if in.Amount <= 0 {
		return 0, nil, fmt.Errorf("%w: nominal setoran harus lebih dari 0", helpers.ErrValidation)
	}
	source := SumberLain
	if in.Method == "cash" && in.Source != "" {
		if !SumberBayarSah(in.Source) {
			return 0, nil, fmt.Errorf("%w: sumber harus drawer atau other", helpers.ErrValidation)
		}
		source = in.Source
	} else if in.Source == SumberLaci {
		return 0, nil, fmt.Errorf("%w: hanya setoran tunai yang bisa masuk laci kasir", helpers.ErrValidation)
	}
	if source == SumberLaci {
		if in.OutletID == "" {
			return 0, nil, fmt.Errorf("%w: pilih toko yang menerima setoran", helpers.ErrValidation)
		}
		if err := ensureOutletAccess(ctx, in.OutletID); err != nil {
			return 0, nil, err
		}
	}

	status, body, _, err := jalankanIdempoten(ctx, idempotencyScopeCustomerPayment, in.IdempotencyKey, in.RequestHash,
		func(tx *gorm.DB) (int, []byte, error) {
			var cust models.Customer
			if err := repositories.FindCustomerInTenant(ctx, tx, in.CustomerID, &cust); err != nil {
				return 0, nil, err
			}
			recs, err := repositories.LockUnpaidReceivablesOfCustomer(ctx, tx, cust.ID)
			if err != nil {
				return 0, nil, err
			}
			var sisa int64
			for _, r := range recs {
				sisa += r.Outstanding()
			}
			if sisa <= 0 {
				return 0, nil, fmt.Errorf("%w: %s tidak punya kasbon yang belum lunas", helpers.ErrConflict, cust.Name)
			}
			if in.Amount > sisa {
				return 0, nil, fmt.Errorf("%w: setoran %s melebihi sisa kasbon %s",
					helpers.ErrValidation, helpers.FormatRupiah(in.Amount), helpers.FormatRupiah(sisa))
			}

			now := time.Now().UTC()
			bizDate, err := tanggalUsahaSetoran(ctx, tx, in.OutletID, recs[0].ID, now)
			if err != nil {
				return 0, nil, err
			}
			var kasID *string
			if source == SumberLaci {
				mv, err := catatKasTx(ctx, tx, in.OutletID, "", "in", in.Amount, "Setoran kasbon "+cust.Name)
				if err != nil {
					if strings.Contains(err.Error(), "belum ada shift terbuka") {
						return 0, nil, fmt.Errorf("%w: kasir di toko ini belum dibuka — buka kasir dulu, atau catat sebagai uang lain", helpers.ErrValidation)
					}
					return 0, nil, err
				}
				kasID = &mv.ID
			}

			saleIDs := make([]string, 0, len(recs))
			for _, r := range recs {
				if r.SourceTable == "sales" {
					saleIDs = append(saleIDs, r.SourceID)
				}
			}
			nota, err := repositories.ReceivableSaleRefs(ctx, saleIDs)
			if err != nil {
				return 0, nil, err
			}

			penerima := reqctx.UserID(ctx)
			out := structs.CustomerPaymentResponse{Amount: in.Amount, ToDrawer: kasID != nil,
				Allocations: []structs.CustomerPaymentAllocation{}}
			tersisa := in.Amount
			for _, r := range recs {
				if tersisa == 0 {
					break
				}
				bayar := min(tersisa, r.Outstanding())
				if bayar <= 0 {
					continue
				}
				pay := models.ReceivablePayment{
					ReceivableID: r.ID, Amount: bayar, Method: in.Method, PaidAt: now, BusinessDate: bizDate,
					CollectedBy: &penerima, CashMovementID: kasID, Note: strings.TrimSpace(in.Note),
				}
				if err := repositories.CreateReceivablePayment(ctx, tx, &pay); err != nil {
					return 0, nil, err
				}
				dibayar, st := r.PaidAmount+bayar, "partial"
				if dibayar >= r.Amount {
					st = "paid"
				}
				if err := repositories.SetReceivablePaid(ctx, tx, r.ID, dibayar, st); err != nil {
					return 0, nil, err
				}
				out.Allocations = append(out.Allocations, structs.CustomerPaymentAllocation{
					ReceivableID: r.ID, ReceiptNo: nota[r.SourceID].ReceiptNo, Amount: bayar, Settled: st == "paid",
				})
				tersisa -= bayar
			}
			out.Outstanding = sisa - in.Amount

			body, err := json.Marshal(structs.SuccessResponse[structs.CustomerPaymentResponse]{
				Success: true, Message: "Setoran kasbon dicatat", Data: out,
			})
			return http.StatusCreated, body, err
		})
	if errors.Is(err, repositories.ErrCustomerNotFound) {
		return 0, nil, fmt.Errorf("%w: pelanggan tidak ditemukan", helpers.ErrNotFound)
	}
	return status, body, err
}

// tanggalUsahaSetoran: hari usaha setoran — menurut zona & jam tutup buku
// TOKO YANG MENERIMA bila diketahui; bila tidak, toko asal kasbon terlama
// (aturan AddReceivablePayment). Komisi penagih dihitung dari tanggal ini.
func tanggalUsahaSetoran(ctx context.Context, tx *gorm.DB, outletID, receivableID string, now time.Time) (time.Time, error) {
	if outletID != "" {
		var o models.Outlet
		if err := repositories.FindOutletByID(ctx, tx, outletID, &o); err != nil {
			if errors.Is(err, repositories.ErrOutletNotFound) {
				return time.Time{}, fmt.Errorf("%w: toko tidak ditemukan", helpers.ErrValidation)
			}
			return time.Time{}, err
		}
		return timez.BusinessDate(now, o.Timezone, o.DayStartOffset())
	}
	return receivableBusinessDate(ctx, receivableID, now)
}

// SetReceivableDueDate mengatur jatuh tempo satu kasbon belum lunas — mis.
// "janji bayar tanggal 25". due kosong = tanpa jatuh tempo.
func SetReceivableDueDate(ctx context.Context, id, due string) (models.Receivable, error) {
	var tgl *time.Time
	if due = strings.TrimSpace(due); due != "" {
		t, err := time.Parse("2006-01-02", due)
		if err != nil {
			return models.Receivable{}, fmt.Errorf("%w: jatuh tempo harus berformat YYYY-MM-DD", helpers.ErrValidation)
		}
		tgl = &t
	}
	var rec models.Receivable
	err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		var err error
		rec, err = repositories.SetReceivableDueDate(ctx, tx, id, tgl)
		return err
	})
	return rec, err
}
