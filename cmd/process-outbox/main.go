// Command process-outbox mengirim notifikasi yang mengantre di `outbox_events`
// (docs/TECHNICAL-BACKEND.md §5.14).
//
// PEKERJAAN TERJADWAL. Peristiwanya ditulis di dalam transaksi bisnis
// (mis. saat tagihan langganan terbit); pekerja inilah yang mengirimnya.
// Pemisahan itu yang membuat "tagihan tersimpan tapi notifikasinya hilang" —
// atau sebaliknya — tidak mungkin terjadi.
//
// Idempoten & aman beberapa instance: FOR UPDATE SKIP LOCKED, satu peristiwa
// per transaksi, gagal → penundaan bertahap, 10 kali gagal → antrean mati yang
// bisa dilihat & dicoba ulang dari panel internal.
//
// Pemakaian:
//
//	go run ./cmd/process-outbox                         # sekali jalan (cron)
//	go run ./cmd/process-outbox -loop -interval 30s     # daemon (systemd)
//
// Konfigurasi database dibaca dari environment / .env yang sama dengan aplikasi.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"candra/backend-api/config"
	"candra/backend-api/database"
	"candra/backend-api/helpers"
	"candra/backend-api/services"
)

func main() {
	loop := flag.Bool("loop", false, "jalan menetap: kirim lalu tidur -interval, berulang")
	interval := flag.Duration("interval", 30*time.Second, "jeda antar putaran saat -loop")
	flag.Parse()

	config.LoadEnv()
	helpers.InitLogger()
	database.InitDatabase()
	defer database.Close()

	// Pengirim sungguhan dipasang DI SINI, bukan di main.go server: hanya pekerja
	// ini yang mengirim notifikasi, jadi hanya ia yang perlu tahu soal penyedia.
	services.PasangNotifierDariEnv()

	if !*loop {
		if err := sekaliJalan(context.Background()); err != nil {
			fmt.Fprintf(os.Stderr, "process-outbox: %v\n", err)
			os.Exit(1)
		}
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log := helpers.LoggerFromContext(ctx)
	log.Info("process-outbox daemon mulai", "interval", interval.String())
	ticker := time.NewTicker(*interval)
	defer ticker.Stop()
	for {
		if err := sekaliJalan(ctx); err != nil {
			log.Error("putaran pengiriman gagal", "error", err)
		}
		select {
		case <-ctx.Done():
			log.Info("process-outbox daemon berhenti")
			return
		case <-ticker.C:
		}
	}
}

func sekaliJalan(ctx context.Context) error {
	res, err := services.ProcessOutbox(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("Outbox: terkirim=%d gagal=%d mati=%d\n", res.Sent, res.Failed, res.Dead)
	return nil
}
