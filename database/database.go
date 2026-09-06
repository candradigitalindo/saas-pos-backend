package database

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"candra/backend-api/config"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// quoteDSNValue membungkus nilai keyword libpq dalam tanda kutip tunggal dan
// meng-escape karakter khusus (\ dan '). pgx akan melepas kutip ini saat parsing.
func quoteDSNValue(value string) string {
	escaped := strings.ReplaceAll(value, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `'`, `\'`)
	return "'" + escaped + "'"
}

// buildDSN menyusun connection string PostgreSQL (format keyword/value).
// Keyword libpq standar di-quote agar password yang mengandung spasi/karakter
// khusus (mis. @ : / ? spasi ') atau bahkan kosong tidak merusak parsing DSN dan
// tidak bisa dipakai untuk "parameter injection". TimeZone adalah runtime
// parameter (nilai terkendali dari config) sehingga dibiarkan tanpa quote.
func buildDSN(host, user, pass, dbname, port, sslmode, tz string, statementTimeoutMs int) string {
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=%s",
		quoteDSNValue(host), quoteDSNValue(user), quoteDSNValue(pass),
		quoteDSNValue(dbname), quoteDSNValue(port), quoteDSNValue(sslmode), tz,
	)

	// statement_timeout membuat PostgreSQL sendiri membatalkan query yang berjalan
	// melewati batas waktu. Ini jaring pengaman terakhir agar satu query bermasalah
	// tidak menahan koneksi (session) selamanya. Set 0 untuk menonaktifkan.
	if statementTimeoutMs > 0 {
		dsn += fmt.Sprintf(" statement_timeout=%d", statementTimeoutMs)
	}
	return dsn
}

// DSNFromEnv menyusun connection string PostgreSQL dari variabel environment.
//
// Dipakai bersama oleh InitDatabase (koneksi aplikasi) dan cmd/migrate (runner
// migrasi), sehingga keduanya menuju database yang sama dengan aturan quoting
// yang sama. Format keyword/value; kompatibel dengan driver pgx maupun libpq.
//
// DB_TZ default-nya UTC — seluruh waktu disimpan UTC (docs/TECHNICAL-BACKEND.md
// aturan #4). Jangan kembalikan ke Asia/Jakarta.
func DSNFromEnv() string {
	return buildDSN(
		config.GetEnv("DB_HOST", "localhost"),
		config.GetEnv("DB_USER", "root"),
		config.GetEnv("DB_PASS", ""),
		config.GetEnv("DB_NAME", "pos_db"),
		config.GetEnv("DB_PORT", "5432"),
		config.GetEnv("DB_SSLMODE", "disable"),
		config.GetEnv("DB_TZ", "UTC"),
		config.GetIntEnv("DB_STATEMENT_TIMEOUT_MS", 30000),
	)
}

// MigrationDSNFromEnv sama seperti DSNFromEnv tetapi TANPA statement_timeout.
//
// Migrasi berat (mis. CREATE INDEX CONCURRENTLY pada tabel besar) bisa berjalan
// jauh lebih lama dari 30 detik. Batas waktu di jalur runtime jangan sampai
// membatalkan migrasi di tengah jalan dan meninggalkan skema setengah jadi.
func MigrationDSNFromEnv() string {
	return buildDSN(
		config.GetEnv("DB_HOST", "localhost"),
		config.GetEnv("DB_USER", "root"),
		config.GetEnv("DB_PASS", ""),
		config.GetEnv("DB_NAME", "pos_db"),
		config.GetEnv("DB_PORT", "5432"),
		config.GetEnv("DB_SSLMODE", "disable"),
		config.GetEnv("DB_TZ", "UTC"),
		0, // tanpa statement_timeout
	)
}

// Close menutup connection pool database. Dipanggil saat shutdown agar seluruh
// session di sisi PostgreSQL dilepas dengan rapi, bukan ditinggal menggantung.
func Close() error {
	if DB == nil {
		return nil
	}
	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// Ping memeriksa apakah database masih menjawab, dengan menghormati batas waktu
// pada ctx. Dipakai endpoint readiness (/health/ready) untuk load balancer.
func Ping(ctx context.Context) error {
	if DB == nil {
		return context.Canceled
	}
	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}

func InitDatabase() {
	// DSN dibangun terpusat di DSNFromEnv agar aplikasi dan cmd/migrate selalu
	// menuju database yang sama. Setiap nilai di-quote & di-escape sehingga aman
	// terhadap password berspasi/berkarakter khusus dan tidak bisa dipakai untuk
	// "parameter injection" pada DSN.
	dsn := DSNFromEnv()

	// Konfigurasi Logger GORM
	var gormLogger logger.Interface
	if config.GetEnv("APP_ENV", "development") == "production" {
		gormLogger = logger.Default.LogMode(logger.Warn) // Hanya log warning & error di production
	} else {
		gormLogger = logger.Default.LogMode(logger.Info) // Log semua query di development
	}

	// Koneksi ke database
	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: gormLogger,
		// GORM secara default membungkus SETIAP operasi tulis dalam transaksi.
		// Operasi tulis di aplikasi ini berupa statement tunggal (yang sudah atomik
		// dengan sendirinya), sehingga transaksi implisit tersebut hanya menambah
		// dua round-trip (BEGIN/COMMIT) per write. Transaksi eksplisit tetap bisa
		// dipakai lewat DB.Transaction(...) bila memang dibutuhkan.
		SkipDefaultTransaction: true,
		// Cache prepared statement per koneksi: query yang sama tidak perlu
		// di-parse & di-plan ulang oleh PostgreSQL setiap kali dipanggil.
		// Catatan: matikan bila memakai PgBouncer dalam mode transaction/statement.
		PrepareStmt: config.GetBoolEnv("DB_PREPARE_STMT", true),
	})
	if err != nil {
		log.Fatal("Gagal terhubung ke database:", err)
	}
	fmt.Println("Berhasil terhubung ke database!")

	// Konfigurasi Connection Pool
	sqlDB, err := DB.DB()
	if err != nil {
		log.Fatal("Gagal mendapatkan instance *sql.DB:", err)
	}

	// Jumlah koneksi terbuka maksimum. Default 25 (bukan 100) agar beberapa
	// instance aplikasi tidak menghabiskan kuota max_connections PostgreSQL
	// yang secara default hanya 100.
	maxOpen := config.GetIntEnv("DB_MAX_OPEN_CONNS", 25)
	if maxOpen < 1 {
		maxOpen = 1
	}

	// Jumlah koneksi idle yang dipertahankan. Dibuat sama dengan maxOpen agar
	// koneksi dipakai ulang, bukan terus dibuka-tutup saat lalu lintas tinggi
	// (membuka session PostgreSQL baru itu mahal). Nilai idle tidak boleh
	// melebihi maxOpen, jika tidak koneksi akan langsung ditutup setelah dipakai.
	maxIdle := config.GetIntEnv("DB_MAX_IDLE_CONNS", maxOpen)
	if maxIdle > maxOpen {
		maxIdle = maxOpen
	}
	if maxIdle < 1 {
		maxIdle = 1
	}

	sqlDB.SetMaxOpenConns(maxOpen)
	sqlDB.SetMaxIdleConns(maxIdle)
	// Waktu hidup maksimum sebuah koneksi sebelum dibuat ulang.
	sqlDB.SetConnMaxLifetime(time.Duration(config.GetIntEnv("DB_CONN_MAX_LIFETIME_MINUTES", 60)) * time.Minute)
	// Lepaskan koneksi yang menganggur terlalu lama agar session di sisi
	// PostgreSQL tidak menumpuk saat aplikasi sedang sepi.
	sqlDB.SetConnMaxIdleTime(time.Duration(config.GetIntEnv("DB_CONN_MAX_IDLE_MINUTES", 5)) * time.Minute)

	// Skema HANYA berubah lewat migrasi berversi (docs/TECHNICAL-BACKEND.md
	// aturan #10). AutoMigrate sengaja tidak dipanggil di sini: menjalankannya
	// saat aplikasi start membuat dua instance yang naik bersamaan berebut
	// mengubah skema. Jalankan migrasi sebagai langkah deploy terpisah:
	//
	//	go run ./cmd/migrate up
	//
	// Peringatkan lebih awal bila database tertinggal dari versi migrasi.
	if err := WarnIfMigrationsPending(); err != nil {
		log.Printf("Peringatan migrasi: %v", err)
	}
}
