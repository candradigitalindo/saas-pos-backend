package services

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"candra/backend-api/config"
	"candra/backend-api/helpers"
	"candra/backend-api/internal/reqctx"
	"candra/backend-api/internal/timez"
	"candra/backend-api/models"
	"candra/backend-api/repositories"

	"gorm.io/gorm"
)

// Pengingat utang pemasok (cmd/payable-reminders, migrasi 000048).
//
// Sekali sehari: pembelian belum lunas yang jatuh temponya BARU masuk masa
// "sebentar lagi" (≤ PAYABLE_DUE_SOON_DAYS hari, bawaan 3) atau BARU lewat,
// dicatat di payable_notices — lalu pemilik menerima SATU ringkasan WhatsApp
// yang memuat semua utang yang sedang mendesak. Tidak ada yang baru → tidak
// ada pesan: pengingat yang datang setiap hari untuk utang yang sama berhenti
// dibaca pada hari ketiga.

// PayableDueSoonDays: batas "jatuh tempo sebentar lagi" (hari), dipakai juga
// ringkasan di Beranda supaya angka layar & pesan sama.
func PayableDueSoonDays() int { return config.GetIntEnv("PAYABLE_DUE_SOON_DAYS", 3) }

// PayableReminderOptions: TenantID kosong = semua tenant.
type PayableReminderOptions struct{ TenantID string }

// PayableReminderReport merangkum satu putaran.
type PayableReminderReport struct {
	Notices int // (pembelian, jenis) yang baru dicatat
	Sent    int // ringkasan WhatsApp yang masuk outbox
	Failed  int // tenant yang gagal (lihat log); tenant lain tetap diproses
}

// RunPayableReminders menjalankan satu putaran pengingat utang pemasok.
func RunPayableReminders(ctx context.Context, opsi PayableReminderOptions) (PayableReminderReport, error) {
	var rep PayableReminderReport
	now := time.Now().UTC()
	segera := PayableDueSoonDays()
	// Longgar sehari: tanggal usaha outlet bisa sudah "besok" dibanding UTC.
	kandidat, err := repositories.PayableReminderCandidates(ctx, now.AddDate(0, 0, segera+1), opsi.TenantID)
	if err != nil {
		return rep, err
	}
	perTenant := map[string][]repositories.PayableCandidate{}
	var urutan []string
	for _, k := range kandidat {
		if _, ada := perTenant[k.TenantID]; !ada {
			urutan = append(urutan, k.TenantID)
		}
		perTenant[k.TenantID] = append(perTenant[k.TenantID], k)
	}
	for _, tid := range urutan {
		baru, terkirim, err := ingatkanUtangTenant(ctx, tid, perTenant[tid], now, segera)
		if err != nil {
			rep.Failed++
			helpers.LoggerFromContext(ctx).Error("pengingat utang pemasok",
				slog.String("tenant_id", tid), slog.Any("error", err))
			continue
		}
		rep.Notices += baru
		if terkirim {
			rep.Sent++
		}
	}
	return rep, nil
}

// utangMendesak: satu nota di ringkasan pesan.
type utangMendesak struct {
	repositories.PayableCandidate
	selisih int // hari dari tanggal usaha outlet ke jatuh tempo (<0 = lewat)
}

func ingatkanUtangTenant(ctx context.Context, tenantID string, daftar []repositories.PayableCandidate, now time.Time, segera int) (int, bool, error) {
	tctx := reqctx.WithTenantID(ctx, tenantID)
	baru, terkirim := 0, false
	err := repositories.WithTenant(tctx, func(tx *gorm.DB) error {
		hariIni := map[string]time.Time{} // tanggal usaha per outlet
		var mendesak []utangMendesak
		for _, k := range daftar {
			h, ada := hariIni[k.OutletID]
			if !ada {
				var o models.Outlet
				if err := repositories.FindOutletByID(tctx, tx, k.OutletID, &o); err != nil {
					return err
				}
				d, err := timez.BusinessDate(now, o.Timezone, o.DayStartOffset())
				if err != nil {
					return err
				}
				h = time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC)
				hariIni[k.OutletID] = h
			}
			due := time.Date(k.DueDate.Year(), k.DueDate.Month(), k.DueDate.Day(), 0, 0, 0, 0, time.UTC)
			selisih := int(due.Sub(h).Hours() / 24)
			jenis := ""
			switch {
			case selisih < 0:
				jenis = "overdue"
			case selisih <= segera:
				jenis = "due_soon"
			default:
				continue
			}
			mendesak = append(mendesak, utangMendesak{k, selisih})
			ok, err := repositories.InsertPayableNoticeOnce(tctx, tx, k.ID, jenis)
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
			"ringkasan": ringkasanUtang(mendesak),
		}
		tid := tenantID
		if err := EnqueueNotification(tctx, tx, &tid, "payable.due_digest", payload); err != nil {
			return err
		}
		terkirim = true
		return nil
	})
	return baru, terkirim, err
}

// ringkasanUtang menyusun isi pesan: jumlah & total yang lewat dan yang
// segera jatuh tempo, lalu paling banyak lima nota terdekat.
func ringkasanUtang(daftar []utangMendesak) string {
	sort.SliceStable(daftar, func(i, j int) bool { return daftar[i].selisih < daftar[j].selisih })
	var nLewat, nSegera int
	var tLewat, tSegera int64
	for _, d := range daftar {
		if d.selisih < 0 {
			nLewat++
			tLewat += d.Outstanding
		} else {
			nSegera++
			tSegera += d.Outstanding
		}
	}
	var b strings.Builder
	if nLewat > 0 {
		fmt.Fprintf(&b, "• Lewat jatuh tempo: %d nota, %s\n", nLewat, helpers.FormatRupiah(tLewat))
	}
	if nSegera > 0 {
		fmt.Fprintf(&b, "• Jatuh tempo sebentar lagi: %d nota, %s\n", nSegera, helpers.FormatRupiah(tSegera))
	}
	b.WriteString("\nYang paling dekat:\n")
	for i, d := range daftar {
		if i == 5 {
			fmt.Fprintf(&b, "… dan %d nota lagi\n", len(daftar)-5)
			break
		}
		nama := d.SupplierName
		if nama == "" {
			nama = "Tanpa pemasok"
		}
		if d.InvoiceNo != "" {
			nama += " (nota " + d.InvoiceNo + ")"
		}
		var kapan string
		switch {
		case d.selisih < 0:
			kapan = fmt.Sprintf("lewat %d hari", -d.selisih)
		case d.selisih == 0:
			kapan = "hari ini"
		case d.selisih == 1:
			kapan = "besok"
		default:
			kapan = fmt.Sprintf("%d hari lagi", d.selisih)
		}
		fmt.Fprintf(&b, "– %s %s, %s\n", nama, helpers.FormatRupiah(d.Outstanding), kapan)
	}
	return strings.TrimRight(b.String(), "\n")
}
