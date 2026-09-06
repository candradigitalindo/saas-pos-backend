// Command migrate menjalankan migrasi skema berversi.
//
// Ini adalah LANGKAH DEPLOY TERPISAH, bukan bagian dari start aplikasi
// (docs/TECHNICAL-BACKEND.md §4 aturan 6): dua instance aplikasi yang naik
// bersamaan tidak boleh berebut mengubah skema.
//
// Pemakaian:
//
//	go run ./cmd/migrate up            # terapkan semua migrasi yang tertunda
//	go run ./cmd/migrate down [n]      # batalkan n migrasi terakhir (default 1)
//	go run ./cmd/migrate status        # tampilkan versi terpasang & jumlah tertunda
//
// Konfigurasi database dibaca dari environment / .env yang sama dengan aplikasi
// (DB_HOST, DB_PORT, DB_USER, DB_PASS, DB_NAME, DB_SSLMODE, DB_TZ).
package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"candra/backend-api/config"
	"candra/backend-api/database"
)

func main() {
	config.LoadEnv()

	args := os.Args[1:]
	if len(args) == 0 {
		usage()
		os.Exit(2)
	}

	switch args[0] {
	case "up":
		fmt.Println("Menerapkan migrasi:")
		if err := database.MigrateUp(); err != nil {
			fail("%v", err)
		}
		fmt.Println("Selesai. Database terkini.")

	case "down":
		steps := 1
		if len(args) > 1 {
			n, err := strconv.Atoi(args[1])
			if err != nil || n < 1 {
				fail("jumlah langkah tidak valid: %q", args[1])
			}
			steps = n
		}
		if !confirm(fmt.Sprintf("Batalkan %d migrasi terakhir? Ini bisa menghapus data. [y/N] ", steps)) {
			fmt.Println("Dibatalkan.")
			return
		}
		fmt.Println("Membatalkan migrasi:")
		if err := database.MigrateDown(steps); err != nil {
			fail("%v", err)
		}
		fmt.Println("Selesai.")

	case "status":
		current, pending, err := database.MigrationStatus()
		if err != nil {
			fail("%v", err)
		}
		if current == 0 {
			fmt.Println("Belum ada migrasi yang diterapkan.")
		} else {
			fmt.Printf("Versi terpasang: %d\n", current)
		}
		fmt.Printf("Migrasi tertunda: %d\n", pending)

	default:
		usage()
		os.Exit(2)
	}
}

func fail(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "migrate: "+format+"\n", a...)
	os.Exit(1)
}

func confirm(prompt string) bool {
	fmt.Print(prompt)
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes"
}

func usage() {
	fmt.Fprint(os.Stderr, `Pemakaian: go run ./cmd/migrate <perintah>

Perintah:
  up             Terapkan semua migrasi yang tertunda.
  down [n]       Batalkan n migrasi terakhir (default 1). Meminta konfirmasi.
  status         Tampilkan versi migrasi terpasang dan jumlah yang tertunda.
`)
}
