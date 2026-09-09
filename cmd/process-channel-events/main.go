// Command process-channel-events menjalankan pekerja pemroses antrean kanal
// (docs/TECHNICAL-BACKEND.md §5.10, §11; blueprint F.6).
//
// Ini PEKERJAAN TERJADWAL — §11 menyebut "Pemrosesan channel_events | tiap 10
// detik". Dijalankan cron/systemd timer sesering itu, BUKAN bagian dari start
// aplikasi. Sekali jalan: menguras seluruh peristiwa 'pending'/'failed' (satu
// per transaksi, FOR UPDATE SKIP LOCKED — aman beberapa instance), lalu
// mengosongkan antrean channel_stock_syncs, lalu keluar.
//
// Idempoten: peristiwa yang diproses ulang tidak membuat penjualan kedua
// (RecordChannelOrder mengenali external_order_id yang sudah tercatat).
//
// Pemakaian:
//
//	go run ./cmd/process-channel-events
//
// Konfigurasi database dibaca dari environment / .env yang sama dengan aplikasi.
package main

import (
	"context"
	"fmt"
	"os"

	"candra/backend-api/config"
	"candra/backend-api/database"
	"candra/backend-api/helpers"
	"candra/backend-api/services"
)

func main() {
	config.LoadEnv()
	helpers.InitLogger()
	database.InitDatabase()
	defer database.Close()

	res, err := services.ProcessChannelEvents(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "process-channel-events: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf(
		"Pemrosesan kanal selesai: peristiwa selesai=%d gagal=%d mati=%d; sinkron stok terkirim=%d gagal=%d.\n",
		res.EventsDone, res.EventsFailed, res.EventsDead, res.StockSyncsSent, res.StockSyncsFail,
	)
}
