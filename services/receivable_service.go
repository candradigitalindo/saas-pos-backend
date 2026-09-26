package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"candra/backend-api/helpers"
	"candra/backend-api/internal/timez"
	"candra/backend-api/models"
	"candra/backend-api/repositories"
	"candra/backend-api/structs"

	"gorm.io/gorm"
)

// AddReceivablePayment mencatat setoran atas piutang pelanggan.
//
// Tugas utamanya yang tidak boleh dilewatkan: menghitung `business_date` dari
// ZONA WAKTU OUTLET, bukan dari UTC.
//
// Sebelumnya kolom itu diisi `time.Now().UTC()` langsung di controller. Di
// Indonesia (UTC+7 sampai UTC+9) tanggal UTC tertinggal dari tanggal setempat
// setiap hari antara tengah malam dan pagi, sehingga setoran yang diterima
// pukul 06.00 WIB tercatat sebagai HARI KEMARIN. Akibatnya berantai:
//
//   - Komisi sales dihitung dari `business_date` (lihat CollectedByUser), jadi
//     uang yang benar-benar ditagih pagi hari jatuh di luar periode dan
//     komisinya hilang.
//   - Jam tutup buku outlet (`business_day_start`) diabaikan sama sekali,
//     padahal warung yang tutup pukul 02.00 memang ingin setoran larut malam
//     masuk ke hari jualan sebelumnya.
//
// Outlet diambil dari penjualan asal piutangnya bila ada — satu tenant boleh
// punya cabang di zona waktu berbeda (Asia/Jakarta, Asia/Makassar,
// Asia/Jayapura), jadi memakai "outlet pertama" akan salah untuk cabang di
// zona lain. Piutang dari invoice tidak terikat outlet, jadi di situ barulah
// outlet pertama dipakai sebagai perkiraan terbaik.
//
// Idempoten lewat Idempotency-Key (scope "receivable.payment"): setoran adalah
// uang yang masuk, dan tombol "Terima setoran" yang ditekan dua kali — atau
// dikirim ulang setelah sinyal putus — dulu mencatat cicilan DUA kali selama
// sisanya masih cukup. Mengembalikan (status HTTP, body JSON siap kirim).
func AddReceivablePayment(
	ctx context.Context,
	in *models.ReceivablePayment,
	idempotencyKey, requestHash string,
) (int, []byte, error) {
	if idempotencyKey == "" {
		return 0, nil, fmt.Errorf("%w: header Idempotency-Key wajib", helpers.ErrValidation)
	}
	now := time.Now().UTC()

	bizDate, err := receivableBusinessDate(ctx, in.ReceivableID, now)
	if err != nil {
		return 0, nil, err
	}

	in.PaidAt = now
	in.BusinessDate = bizDate
	status, body, _, err := jalankanIdempoten(ctx, idempotencyScopeReceivablePayment, idempotencyKey, requestHash,
		func(tx *gorm.DB) (int, []byte, error) {
			rec, err := repositories.AddReceivablePayment(ctx, tx, in)
			if err != nil {
				return 0, nil, err
			}
			body, err := json.Marshal(structs.SuccessResponse[structs.ReceivableResponse]{
				Success: true, Message: "Pembayaran piutang dicatat", Data: ReceivableToResponse(rec),
			})
			return http.StatusCreated, body, err
		})
	return status, body, err
}

// idempotencyScopeReceivablePayment untuk setoran piutang.
const idempotencyScopeReceivablePayment = "receivable.payment"

// receivableBusinessDate menentukan hari usaha untuk sebuah setoran piutang.
func receivableBusinessDate(
	ctx context.Context,
	receivableID string,
	now time.Time,
) (time.Time, error) {
	outlet, err := outletForReceivable(ctx, receivableID)
	if err != nil {
		return time.Time{}, err
	}
	return timez.BusinessDate(now, outlet.Timezone, outlet.DayStartOffset())
}

// outletForReceivable mencari outlet yang paling tepat mewakili sebuah piutang:
// outlet penjualan asalnya, atau outlet pertama tenant bila piutang itu lahir
// dari invoice (yang memang tidak terikat outlet).
func outletForReceivable(ctx context.Context, receivableID string) (models.Outlet, error) {
	var outlet models.Outlet

	var rec models.Receivable
	if err := repositories.FindReceivableInTenant(ctx, nil, receivableID, &rec); err != nil {
		return outlet, err
	}

	if rec.SourceTable == "sales" {
		var sale models.Sale
		if err := repositories.FindSaleInTenant(ctx, nil, rec.SourceID, &sale); err == nil {
			if err := repositories.FindOutletByID(ctx, nil, sale.OutletID, &outlet); err == nil {
				return outlet, nil
			}
		}
		// Penjualan asalnya tidak terbaca (mis. sudah dibersihkan) — jangan
		// gagalkan setorannya; jatuh ke outlet pertama seperti jalur invoice.
	}

	outletID, err := repositories.FirstOutletID(ctx, nil)
	if err != nil {
		return outlet, err
	}
	err = repositories.FindOutletByID(ctx, nil, outletID, &outlet)
	return outlet, err
}
