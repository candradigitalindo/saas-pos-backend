package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"candra/backend-api/config"
	"candra/backend-api/database"
	"candra/backend-api/helpers"
	"candra/backend-api/middlewares"
	"candra/backend-api/routes"
)

// version diisi saat build lewat -ldflags "-X main.version=<tag>" (lihat
// Dockerfile / Makefile). "dev" saat `go run`.
var version = "dev"

func main() {
	config.LoadEnv()              // Muat konfigurasi dari .env
	helpers.InitLogger()          // Pasang logger terstruktur (slog) sebagai default proses
	helpers.InitJWT()             // Inisialisasi JWT Secret Key (fatal bila tidak aman)
	middlewares.InitRateLimiter() // Pilih backend pembatas laju (memori / Redis)
	database.InitDatabase()       // Koneksi + connection pool (TIDAK menjalankan migrasi)
	database.SeedData()           // Seeder data awal (idempoten, non-fatal)

	log.Printf("pos-server %s — APP_ENV=%s", version, config.GetEnv("APP_ENV", "development"))

	r := routes.SetupRouter() // Setup router

	// Konfigurasi server dengan graceful shutdown.
	//
	// Timeout wajib diset secara eksplisit: nilai default net/http adalah "tanpa
	// batas", sehingga koneksi yang mengirim header/body sangat lambat bisa
	// menahan resource server tanpa henti (serangan Slowloris).
	srv := &http.Server{
		Addr:    ":" + config.GetEnv("APP_PORT", "3000"),
		Handler: r,
		// Batas waktu membaca header request.
		ReadHeaderTimeout: time.Duration(config.GetIntEnv("SERVER_READ_HEADER_TIMEOUT_SECONDS", 10)) * time.Second,
		// Batas waktu membaca seluruh request (header + body).
		ReadTimeout: time.Duration(config.GetIntEnv("SERVER_READ_TIMEOUT_SECONDS", 15)) * time.Second,
		// Batas waktu menulis response.
		WriteTimeout: time.Duration(config.GetIntEnv("SERVER_WRITE_TIMEOUT_SECONDS", 30)) * time.Second,
		// Berapa lama koneksi keep-alive boleh menganggur.
		IdleTimeout: time.Duration(config.GetIntEnv("SERVER_IDLE_TIMEOUT_SECONDS", 60)) * time.Second,
		// Batas ukuran header untuk mencegah header raksasa menghabiskan memori.
		MaxHeaderBytes: config.GetIntEnv("SERVER_MAX_HEADER_BYTES", 1<<20), // 1 MB
	}

	go func() {
		// Jalankan server dalam goroutine agar tidak memblokir
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	// Tunggu sinyal interupsi (misalnya, Ctrl+C)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	// Beri waktu 5 detik untuk menyelesaikan request yang sedang berjalan
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown: ", err)
	}

	// Tutup connection pool setelah semua request selesai, agar session di sisi
	// PostgreSQL dilepas dengan rapi.
	if err := database.Close(); err != nil {
		log.Printf("Gagal menutup koneksi database: %v", err)
	}

	log.Println("Server exiting")
}
