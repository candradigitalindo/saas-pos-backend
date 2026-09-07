// Command recognize-revenue mengakui pendapatan diterima di muka yang bulan
// pengakuannya sudah tiba (docs/TECHNICAL-BACKEND.md §13.4).
//
// Ini PEKERJAAN TERJADWAL (§11: "Pengakuan pendapatan diterima di muka | harian,
// awal bulan"), dijalankan oleh cron sistem — bukan bagian dari start aplikasi.
// Idempoten: hanya menyentuh baris `recognized_at IS NULL` yang bulannya ≤ bulan
// berjalan, jadi aman dijalankan berkali-kali.
//
// Pemakaian:
//
//	go run ./cmd/recognize-revenue
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

	n, err := services.RecognizeDueRevenue(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "recognize-revenue: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Pengakuan pendapatan selesai: %d baris diakui.\n", n)
}
