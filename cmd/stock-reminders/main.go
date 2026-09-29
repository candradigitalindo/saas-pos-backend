// Command stock-reminders mengirim pengingat stok menipis: satu ringkasan
// WhatsApp ke pemilik saat ada barang yang BARU habis atau BARU di bawah batas
// minimum sejak pemeriksaan terakhir. Pesannya masuk outbox;
// cmd/process-outbox yang mengirim.
//
// Ini PEKERJAAN TERJADWAL (cron sistem, sekali sehari — pagi, sebelum
// belanja), bukan bagian dari start aplikasi. Idempoten: tiap (toko, barang,
// jenis) diingatkan sekali sampai barangnya pulih (stock_notices).
//
// Pemakaian:
//
//	go run ./cmd/stock-reminders             # semua tenant
//	go run ./cmd/stock-reminders -tenant ID  # satu tenant saja
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"candra/backend-api/config"
	"candra/backend-api/database"
	"candra/backend-api/helpers"
	"candra/backend-api/services"
)

func main() {
	tenant := flag.String("tenant", "", "jalankan untuk satu tenant saja (id)")
	flag.Parse()

	config.LoadEnv()
	helpers.InitLogger()
	database.InitDatabase()
	defer database.Close()

	rep, err := services.RunStockReminders(context.Background(), services.StockReminderOptions{TenantID: *tenant})
	if err != nil {
		fmt.Fprintf(os.Stderr, "stock-reminders: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Pengingat stok selesai: %d barang baru diingatkan, %d ringkasan WhatsApp.\n", rep.Notices, rep.Sent)
	if rep.Failed > 0 {
		fmt.Fprintf(os.Stderr, "stock-reminders: %d tenant gagal — lihat log.\n", rep.Failed)
		os.Exit(2)
	}
}
