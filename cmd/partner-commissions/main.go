// Command partner-commissions menjalankan siklus komisi Program Mitra
// (docs/TECHNICAL-BACKEND.md §16 Fase 12, blueprint G.2/G.6).
//
// PEKERJAAN TERJADWAL — dijalankan cron sebulan sekali (mis. tanggal 1 untuk
// bulan sebelumnya). Idempoten: hanya baris komisi 'held' yang dihitung ulang;
// 'approved'/'paid'/'clawed_back' tak tersentuh.
//
// Pemakaian:
//
//	go run ./cmd/partner-commissions                     # hitung komisi bulan lalu
//	go run ./cmd/partner-commissions -from 2026-08-01 -to 2026-08-31
//	go run ./cmd/partner-commissions -approve            # + setujui semua 'held'
//	go run ./cmd/partner-commissions -approve -payout    # + buat pencairan draft per mitra
//
// Konfigurasi database dibaca dari environment / .env yang sama dengan aplikasi.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"candra/backend-api/config"
	"candra/backend-api/database"
	"candra/backend-api/helpers"
	"candra/backend-api/services"
)

func main() {
	var from, to string
	prevStart, prevEnd := previousMonth(time.Now().UTC())
	flag.StringVar(&from, "from", prevStart, "awal periode pembayaran diterima (YYYY-MM-DD)")
	flag.StringVar(&to, "to", prevEnd, "akhir periode (YYYY-MM-DD)")
	approve := flag.Bool("approve", false, "setujui semua komisi 'held' setelah dihitung")
	payout := flag.Bool("payout", false, "buat pencairan draft per mitra dari komisi disetujui")
	flag.Parse()

	config.LoadEnv()
	helpers.InitLogger()
	database.InitDatabase()
	defer database.Close()

	res, err := services.RunPartnerCommissionCycle(context.Background(), from, to, *approve, *payout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "partner-commissions: %v\n", err)
		os.Exit(1)
	}
	c := res.Compute
	fmt.Printf(
		"Komisi mitra %s..%s: referral=%d aktivasi=%d dihitung=%d ditarik=%d belum-aktif=%d; disetujui=%d pencairan=%d\n",
		c.From, c.To, c.ReferralsSeen, c.Activated, c.Computed, c.ClawedBack, c.SkippedNoActivation,
		res.Approved, res.Payouts,
	)
}

// previousMonth mengembalikan tanggal 1 dan tanggal terakhir bulan SEBELUM t.
func previousMonth(t time.Time) (string, string) {
	firstThis := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
	lastPrev := firstThis.AddDate(0, 0, -1)
	firstPrev := time.Date(lastPrev.Year(), lastPrev.Month(), 1, 0, 0, 0, 0, time.UTC)
	return firstPrev.Format("2006-01-02"), lastPrev.Format("2006-01-02")
}
