// Command subscription-renewals menjalankan pekerjaan harian langganan:
// menerbitkan tagihan perpanjangan (SUBSCRIPTION_RENEWAL_LEAD_DAYS, bawaan 7
// hari sebelum masa habis), menandai tagihan lewat jatuh tempo, menandai
// langganan yang masa bayarnya lewat sebagai past_due, dan mengirim pengingat
// WhatsApp — masa coba hampir habis, tagihan lewat jatuh tempo, masa tenggang
// hampir habis. Pesannya masuk outbox; cmd/process-outbox yang mengirim.
//
// Ini PEKERJAAN TERJADWAL (cron sistem, sekali sehari cukup), bukan bagian dari
// start aplikasi. Idempoten: setiap pengingat terkirim sekali, tagihan tidak
// terbit dua kali — aman dijalankan ulang atau dua instans bersamaan.
//
// Pemakaian:
//
//	go run ./cmd/subscription-renewals             # semua tenant
//	go run ./cmd/subscription-renewals -tenant ID  # satu tenant saja
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

	rep, err := services.RunSubscriptionRenewals(context.Background(), services.RenewalOptions{TenantID: *tenant})
	if err != nil {
		fmt.Fprintf(os.Stderr, "subscription-renewals: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Perpanjangan langganan selesai: %d tagihan terbit, %d lewat jatuh tempo, %d past_due; "+
		"pengingat: %d masa coba, %d jatuh tempo, %d tenggang.\n",
		rep.Issued, rep.Overdue, rep.PastDue, rep.TrialReminders, rep.OverdueReminders, rep.GraceReminders)
	if rep.Failed > 0 {
		fmt.Fprintf(os.Stderr, "subscription-renewals: %d langkah tenant gagal — lihat log.\n", rep.Failed)
		os.Exit(2)
	}
}
