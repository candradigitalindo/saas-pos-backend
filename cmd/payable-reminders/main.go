// Command payable-reminders mengirim pengingat utang pemasok: satu ringkasan
// WhatsApp ke pemilik saat ada nota pembelian yang BARU masuk masa jatuh tempo
// sebentar lagi (PAYABLE_DUE_SOON_DAYS, bawaan 3 hari) atau BARU lewat jatuh
// tempo. Pesannya masuk outbox; cmd/process-outbox yang mengirim.
//
// Ini PEKERJAAN TERJADWAL (cron sistem, sekali sehari cukup — pagi hari),
// bukan bagian dari start aplikasi. Idempoten: tiap (nota, jenis) diingatkan
// sekali (payable_notices), aman dijalankan ulang atau dua instans bersamaan.
//
// Pemakaian:
//
//	go run ./cmd/payable-reminders             # semua tenant
//	go run ./cmd/payable-reminders -tenant ID  # satu tenant saja
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

	rep, err := services.RunPayableReminders(context.Background(), services.PayableReminderOptions{TenantID: *tenant})
	if err != nil {
		fmt.Fprintf(os.Stderr, "payable-reminders: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Pengingat utang pemasok selesai: %d nota baru diingatkan, %d ringkasan WhatsApp.\n", rep.Notices, rep.Sent)
	if rep.Failed > 0 {
		fmt.Fprintf(os.Stderr, "payable-reminders: %d tenant gagal — lihat log.\n", rep.Failed)
		os.Exit(2)
	}
}
