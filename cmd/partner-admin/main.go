// Command partner-admin adalah panel internal Program Mitra sebagai CLI
// (docs/TECHNICAL-BACKEND.md §16 Fase 12, blueprint G.5).
//
// Belum ada realm autentikasi ADMIN PLATFORM di aplikasi (sama seperti
// cmd/recognize-revenue di Fase 7), jadi verifikasi mitra & pengaturan tingkat
// dijalankan operator lewat perintah ini. Portal MITRA sendiri sudah punya
// jalur auth-nya (/api/v1/partner/*).
//
// Pemakaian:
//
//	go run ./cmd/partner-admin list-tiers
//	go run ./cmd/partner-admin list-partners [status]
//	go run ./cmd/partner-admin create-partner -tier agen -name "Budi" -region "Bandung" \
//	    -user-name "Budi" -user-email budi@example.com -user-username budi
//	go run ./cmd/partner-admin approve-partner <partner_id>
//
// Konfigurasi database dibaca dari environment / .env yang sama dengan aplikasi.
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
	"candra/backend-api/structs"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	config.LoadEnv()
	helpers.InitLogger()
	database.InitDatabase()
	defer database.Close()

	ctx := context.Background()
	cmd, args := os.Args[1], os.Args[2:]

	switch cmd {
	case "list-tiers":
		tiers, err := services.ListPartnerTiers(ctx)
		fatal(err)
		for _, t := range tiers {
			months := "selama aktif"
			if t.RecurringMonths != nil {
				months = fmt.Sprintf("%d bulan", *t.RecurringMonths)
			}
			fmt.Printf("%-14s  %-10s  rate=%-8s  komisi=%s\n", t.Name, t.Kind, t.RecurringRate, months)
		}

	case "list-partners":
		status := ""
		if len(args) > 0 {
			status = args[0]
		}
		partners, _, err := services.ListPartners(ctx, status, 500, 0)
		fatal(err)
		for _, p := range partners {
			fmt.Printf("%s  %-10s  %-12s  %-24s  code=%s\n", p.ID, p.Status, p.TierName, p.Name, p.ReferralCode)
		}

	case "approve-partner":
		if len(args) < 1 {
			usage()
		}
		fatal(services.ApprovePartner(ctx, args[0]))
		fmt.Println("Mitra disetujui & diaktifkan.")

	case "create-partner":
		fs := flag.NewFlagSet("create-partner", flag.ExitOnError)
		var in structs.PartnerCreateRequest
		fs.StringVar(&in.TierName, "tier", "", "nama tingkat (list-tiers)")
		fs.StringVar(&in.Name, "name", "", "nama mitra")
		fs.StringVar(&in.Phone, "phone", "", "telepon mitra (wajib)")
		fs.StringVar(&in.Email, "email", "", "email mitra")
		fs.StringVar(&in.Region, "region", "", "wilayah")
		fs.StringVar(&in.IDNumber, "id-number", "", "nomor KTP")
		fs.StringVar(&in.BankName, "bank-name", "", "nama bank")
		fs.StringVar(&in.BankAccountName, "bank-account-name", "", "nama pemilik rekening")
		fs.StringVar(&in.ReferralCode, "code", "", "kode referral (kosong → dibuatkan)")
		fs.StringVar(&in.BankAccountNo, "bank", "", "nomor rekening")
		fs.StringVar(&in.NPWP, "npwp", "", "NPWP")
		fs.StringVar(&in.TaxWithholdingRate, "tax-rate", "", "tarif potong pajak, mis. 0.025")
		fs.StringVar(&in.UserName, "user-name", "", "nama akun login")
		fs.StringVar(&in.UserEmail, "user-email", "", "email akun login")
		fs.StringVar(&in.UserPhone, "user-phone", "", "telepon akun login")
		fs.StringVar(&in.UserPassword, "user-password", "", "password (kosong → dibuatkan)")
		_ = fs.Parse(args)

		res, err := services.CreatePartner(ctx, in)
		fatal(err)
		fmt.Printf("Mitra dibuat: id=%s status=%s referral_code=%s\n", res.Partner.ID, res.Partner.Status, res.Partner.ReferralCode)
		fmt.Printf("Akun login: %s\n", res.UserEmail)
		if res.GeneratedPassword != "" {
			fmt.Printf("Password (SEKALI ini): %s\n", res.GeneratedPassword)
		}
		fmt.Println("Jalankan `approve-partner` setelah verifikasi identitas & rekening.")

	default:
		usage()
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "perintah: list-tiers | list-partners [status] | create-partner -tier ... | approve-partner <id>")
	os.Exit(2)
}

func fatal(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "partner-admin: %v\n", err)
		os.Exit(1)
	}
}
