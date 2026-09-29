// Package tests memuat uji integrasi lintas-lapisan: HTTP → middleware →
// controller → service → repository → PostgreSQL sungguhan.
//
// Butuh database PostgreSQL. Konfigurasinya diambil dari environment
// TEST_DB_* (lihat testDSNFromEnv); bila database tidak bisa dihubungi, SELURUH
// test di paket ini di-skip — bukan gagal — supaya `go test ./...` tetap hijau
// di lingkungan tanpa Postgres.
//
// PERINGATAN: TestMain menjalankan DROP SCHEMA public CASCADE pada database
// target lalu memigrasikannya dari nol. Jangan arahkan ke database berisi data.
package tests

import (
	"database/sql"
	"fmt"
	"net/http/httptest"
	"os"
	"testing"

	"candra/backend-api/config"
	"candra/backend-api/database"
	"candra/backend-api/helpers"
	"candra/backend-api/routes"

	"github.com/gin-gonic/gin"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// dbReady menandakan apakah database test tersedia. Dipakai tiap test untuk
// memutuskan skip.
var dbReady bool

// router adalah instance http.Handler aplikasi yang diuji.
var router *gin.Engine

// testEnv menyetel variabel environment yang dibaca lapisan config/database/jwt.
func testEnv() map[string]string {
	return map[string]string{
		"APP_ENV":              "test",
		"LOG_LEVEL":            "error", // kurangi bising saat test
		"LOG_FORMAT":           "text",
		"DB_HOST":              envOr("TEST_DB_HOST", "localhost"),
		"DB_PORT":              envOr("TEST_DB_PORT", "5432"),
		"DB_USER":              envOr("TEST_DB_USER", "root"),
		"DB_PASS":              os.Getenv("TEST_DB_PASS"),
		"DB_NAME":              envOr("TEST_DB_NAME", "saas_pos_test"),
		"DB_SSLMODE":           envOr("TEST_DB_SSLMODE", "disable"),
		"DB_TZ":                "UTC",
		"DB_PREPARE_STMT":      "false",
		"JWT_SECRET":           "integration-test-secret-yang-cukup-panjang-0123456789",
		"JWT_ACCESS_MINUTES":   "15",
		"JWT_REFRESH_DAYS":     "30",
		"BCRYPT_COST":          "4",    // paling cepat, cukup untuk test
		"AUTH_RATELIMIT_RPS":   "1000", // longgar: test menembak banyak endpoint auth
		"AUTH_RATELIMIT_BURST": "1000",
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// TestMain menyiapkan database test (drop → migrate → seed), membangun router,
// lalu menjalankan seluruh test. Bila DB tidak tersedia, dbReady tetap false dan
// tiap test akan men-skip dirinya.
func TestMain(m *testing.M) {
	for k, v := range testEnv() {
		_ = os.Setenv(k, v)
	}
	gin.SetMode(gin.TestMode)
	config.LoadEnv()
	helpers.InitLogger()
	helpers.InitJWT()

	if err := resetSchema(); err != nil {
		fmt.Printf("tests: database test tidak tersedia, semua test di-skip: %v\n", err)
		os.Exit(m.Run()) // biarkan test skip sendiri
	}

	if err := database.MigrateUp(); err != nil {
		fmt.Printf("tests: migrasi gagal: %v\n", err)
		os.Exit(1)
	}
	database.InitDatabase()
	database.SeedData()
	router = routes.SetupRouter()
	dbReady = true

	code := m.Run()
	_ = database.Close()
	os.Exit(code)
}

// resetSchema mengosongkan skema public database test. Mengembalikan error bila
// database tidak bisa dihubungi (→ test di-skip).
func resetSchema() error {
	db, err := sql.Open("pgx", database.MigrationDSNFromEnv())
	if err != nil {
		return err
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		return err
	}
	_, err = db.Exec("DROP SCHEMA public CASCADE; CREATE SCHEMA public;")
	return err
}

// newServer mengembalikan httptest.Server untuk router (dipanggil per test bila
// perlu URL nyata; kebanyakan test memakai httptest.NewRecorder langsung).
func newServer() *httptest.Server { return httptest.NewServer(router) }

// requireDB men-skip test bila database test tidak tersedia.
func requireDB(t *testing.T) {
	t.Helper()
	if !dbReady {
		t.Skip("database test tidak tersedia (set TEST_DB_* dan jalankan PostgreSQL)")
	}
}
