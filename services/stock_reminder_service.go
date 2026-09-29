package services

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"candra/backend-api/helpers"
	"candra/backend-api/internal/reqctx"
	"candra/backend-api/models"
	"candra/backend-api/repositories"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// Pengingat stok menipis (cmd/stock-reminders, migrasi 000051).
//
// Sekali sehari: barang yang BARU habis atau BARU di bawah batas minimum
// dicatat di stock_notices, lalu pemilik menerima SATU ringkasan WhatsApp yang
// memuat semua barang yang sedang habis / hampir habis — per toko, dengan
// pemasok utamanya supaya bisa langsung dipesan. Barang yang pulih (dibeli
// lagi) catatannya dihapus, jadi penurunan berikutnya diingatkan lagi. Tidak
// ada yang baru → tidak ada pesan.

// StockReminderOptions: TenantID kosong = semua tenant.
type StockReminderOptions struct{ TenantID string }

// StockReminderReport merangkum satu putaran.
type StockReminderReport struct {
	Notices int // (toko, barang, jenis) yang baru dicatat
	Sent    int // ringkasan WhatsApp yang masuk outbox
	Failed  int
}

// Paling banyak barang yang disebut per jenis di pesan.
const batasBarisPengingatStok = 10

// RunStockReminders menjalankan satu putaran pengingat stok.
func RunStockReminders(ctx context.Context, opsi StockReminderOptions) (StockReminderReport, error) {
	var rep StockReminderReport
	kandidat, err := repositories.StockAlertCandidates(ctx, opsi.TenantID)
	if err != nil {
		return rep, err
	}
	perTenant := map[string][]repositories.StockAlert{}
	urutan := []string{}
	for _, k := range kandidat {
		if _, ada := perTenant[k.TenantID]; !ada {
			urutan = append(urutan, k.TenantID)
		}
		perTenant[k.TenantID] = append(perTenant[k.TenantID], k)
	}
	// Tenant tanpa kandidat tetap perlu dibersihkan catatannya (semua pulih);
	// untuk satu tenant yang diminta, pastikan ia ikut diproses.
	if opsi.TenantID != "" && len(perTenant[opsi.TenantID]) == 0 {
		urutan = append(urutan, opsi.TenantID)
	}
	for _, tid := range urutan {
		baru, terkirim, err := ingatkanStokTenant(ctx, tid, perTenant[tid])
		if err != nil {
			rep.Failed++
			helpers.LoggerFromContext(ctx).Error("pengingat stok", slog.String("tenant_id", tid), slog.Any("error", err))
			continue
		}
		rep.Notices += baru
		if terkirim {
			rep.Sent++
		}
	}
	return rep, nil
}

func ingatkanStokTenant(ctx context.Context, tenantID string, daftar []repositories.StockAlert) (int, bool, error) {
	tctx := reqctx.WithTenantID(ctx, tenantID)
	baru, terkirim := 0, false
	err := repositories.WithTenant(tctx, func(tx *gorm.DB) error {
		if err := repositories.ClearRecoveredStockNotices(tctx, tx); err != nil {
			return err
		}
		for _, a := range daftar {
			jenis := "low"
			if a.Qty.LessThanOrEqual(decimal.Zero) {
				jenis = "out"
			}
			ok, err := repositories.InsertStockNoticeOnce(tctx, tx, a.OutletID, a.ProductID, jenis)
			if err != nil {
				return err
			}
			if ok {
				baru++
			}
		}
		if baru == 0 {
			return nil
		}
		var tenant models.Tenant
		if err := repositories.FindTenantByID(tctx, tx, tenantID, &tenant); err != nil {
			return err
		}
		if tenant.Phone == "" {
			return nil // tetap tercatat, supaya tidak dicoba ulang tiap hari
		}
		payload := map[string]any{
			"channel": "whatsapp", "to": tenant.Phone, "nama_usaha": tenant.BusinessName,
			"ringkasan": ringkasanStok(daftar),
		}
		tid := tenantID
		if err := EnqueueNotification(tctx, tx, &tid, "stock.low_digest", payload); err != nil {
			return err
		}
		terkirim = true
		return nil
	})
	return baru, terkirim, err
}

// ringkasanStok: per toko (bila lebih dari satu), "Habis" lalu "Hampir habis",
// tiap baris dengan sisa/batas dan pemasok utamanya.
func ringkasanStok(daftar []repositories.StockAlert) string {
	var toko []string
	perToko := map[string][]repositories.StockAlert{}
	for _, a := range daftar {
		if _, ada := perToko[a.OutletName]; !ada {
			toko = append(toko, a.OutletName)
		}
		perToko[a.OutletName] = append(perToko[a.OutletName], a)
	}
	var b strings.Builder
	for i, t := range toko {
		if len(toko) > 1 {
			if i > 0 {
				b.WriteString("\n")
			}
			fmt.Fprintf(&b, "*%s*\n", t)
		}
		var habis, tipis []repositories.StockAlert
		for _, a := range perToko[t] {
			if a.Qty.LessThanOrEqual(decimal.Zero) {
				habis = append(habis, a)
			} else {
				tipis = append(tipis, a)
			}
		}
		tulisDaftarStok(&b, "Habis", habis, func(a repositories.StockAlert) string { return "" })
		tulisDaftarStok(&b, "Hampir habis", tipis, func(a repositories.StockAlert) string {
			return fmt.Sprintf(" (sisa %s %s, batas %s)", a.Qty.String(), a.UnitName, a.MinStock.String())
		})
	}
	return strings.TrimRight(b.String(), "\n")
}

func tulisDaftarStok(b *strings.Builder, judul string, daftar []repositories.StockAlert, rinci func(repositories.StockAlert) string) {
	if len(daftar) == 0 {
		return
	}
	fmt.Fprintf(b, "%s (%d):\n", judul, len(daftar))
	for i, a := range daftar {
		if i == batasBarisPengingatStok {
			fmt.Fprintf(b, "… dan %d lagi\n", len(daftar)-batasBarisPengingatStok)
			break
		}
		pemasok := ""
		if a.SupplierName != "" {
			pemasok = " — " + a.SupplierName
		}
		fmt.Fprintf(b, "• %s%s%s\n", a.ProductName, rinci(a), pemasok)
	}
}
