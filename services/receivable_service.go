package services

import (
	"context"
	"time"

	"candra/backend-api/internal/timez"
	"candra/backend-api/models"
	"candra/backend-api/repositories"
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
func AddReceivablePayment(
	ctx context.Context,
	in *models.ReceivablePayment,
) (models.Receivable, error) {
	now := time.Now().UTC()

	bizDate, err := receivableBusinessDate(ctx, in.ReceivableID, now)
	if err != nil {
		return models.Receivable{}, err
	}

	in.PaidAt = now
	in.BusinessDate = bizDate
	return repositories.AddReceivablePayment(ctx, in)
}

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
