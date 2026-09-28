// Command process-channel-events menjalankan pekerja pemroses antrean kanal
// (docs/TECHNICAL-BACKEND.md §5.10, §11; blueprint F.6).
//
// §11 menyebut "Pemrosesan channel_events | tiap 10 detik". Dua cara memakainya:
//
//   - SEKALI JALAN (default) — dipanggil cron/systemd timer sesering itu:
//     go run ./cmd/process-channel-events
//
//   - DAEMON — proses menetap, memproses lalu tidur `-interval`, berulang;
//     cocok untuk systemd service:
//     go run ./cmd/process-channel-events -loop -interval 10s
//
// Sekali jalan: menguras seluruh peristiwa 'pending'/'failed' (satu per
// transaksi, FOR UPDATE SKIP LOCKED — aman beberapa instance), lalu mengosongkan
// antrean channel_stock_syncs.
//
// Idempoten: peristiwa yang diproses ulang tidak membuat penjualan kedua
// (RecordChannelOrder mengenali external_order_id yang sudah tercatat).
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
	loop := flag.Bool("loop", false, "jalan menetap: proses lalu tidur -interval, berulang")
	interval := flag.Duration("interval", 10*time.Second, "jeda antar putaran saat -loop")
	flag.Parse()

	config.LoadEnv()
	helpers.InitLogger()
	database.InitDatabase()
	defer database.Close()

	if !*loop {
		if err := runOnce(context.Background()); err != nil {
			fmt.Fprintf(os.Stderr, "process-channel-events: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Mode daemon: berhenti rapi saat SIGINT/SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log := helpers.LoggerFromContext(ctx)
	log.Info("process-channel-events daemon mulai", "interval", interval.String())
	ticker := time.NewTicker(*interval)
	defer ticker.Stop()
	for {
		if err := runOnce(ctx); err != nil {
			log.Error("putaran pemrosesan gagal", "error", err)
		}
		select {
		case <-ctx.Done():
			log.Info("process-channel-events daemon berhenti")
			return
		case <-ticker.C:
		}
	}
}

// runOnce menjalankan satu putaran penuh: peristiwa + antrean sinkron stok.
func runOnce(ctx context.Context) error {
	// Cadangan push: kanal yang penyedianya mendukung ditarik tiap 15 menit
	// (dilewati bila belum waktunya), lalu hasilnya diproses di putaran ini.
	tarik, err := services.TarikPesananKanal(ctx, time.Now())
	if err != nil {
		return err
	}
	res, err := services.ProcessChannelEvents(ctx)
	if err != nil {
		return err
	}
	fmt.Printf(
		"Pemrosesan kanal: tarikan baru=%d; peristiwa selesai=%d gagal=%d mati=%d; sinkron stok terkirim=%d gagal=%d.\n",
		tarik, res.EventsDone, res.EventsFailed, res.EventsDead, res.StockSyncsSent, res.StockSyncsFail,
	)
	return nil
}
