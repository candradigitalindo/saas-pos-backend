// Command platform-admin membuat & mengelola akun staf internal penyedia SaaS
// (panel internal, blueprint G.5; migrasi 000035).
//
// Ada karena masalah ayam-telur: admin PERTAMA tidak bisa dibuat lewat panel
// yang untuk masuknya butuh admin. Sesudah admin pertama ada, pengelolaan
// selanjutnya sebaiknya lewat panel (/api/v1/platform/admins) agar tercatat di
// jejak audit.
//
// Pemakaian:
//
//	go run ./cmd/platform-admin create -name "Candra" -email admin@contoh.id -role superadmin
//	go run ./cmd/platform-admin list
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

	switch os.Args[1] {
	case "list":
		admins, err := services.ListPlatformAdmins(ctx)
		fatal(err)
		for _, a := range admins {
			aktif := "aktif"
			if !a.IsActive {
				aktif = "nonaktif"
			}
			fmt.Printf("%s  %-12s  %-9s  %-28s  %s\n", a.ID, a.Role, aktif, a.Email, a.Name)
		}

	case "create":
		fs := flag.NewFlagSet("create", flag.ExitOnError)
		var in structs.PlatformAdminCreateRequest
		fs.StringVar(&in.Name, "name", "", "nama admin")
		fs.StringVar(&in.Email, "email", "", "email (dipakai untuk masuk)")
		fs.StringVar(&in.Role, "role", "superadmin", "superadmin | operator | finance | support")
		fs.StringVar(&in.Password, "password", "", "kosong → dibuatkan")
		_ = fs.Parse(os.Args[2:])

		res, err := services.CreatePlatformAdmin(ctx, in)
		fatal(err)
		fmt.Printf("Admin dibuat: id=%s peran=%s email=%s\n", res.Admin.ID, res.Admin.Role, res.Admin.Email)
		if res.GeneratedPassword != "" {
			fmt.Printf("Password (SEKALI ini, simpan sekarang): %s\n", res.GeneratedPassword)
		}
		fmt.Println("Masuk lewat POST /api/v1/platform/auth/login")

	default:
		usage()
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "perintah: create -name ... -email ... [-role ...] | list")
	os.Exit(2)
}

func fatal(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "platform-admin: %v\n", err)
		os.Exit(1)
	}
}
