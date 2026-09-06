package database

import (
	"database/sql"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"

	"candra/backend-api/database/migrations"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

// Runner migrasi berversi buatan sendiri.
//
// Kenapa tidak memakai library golang-migrate: paket driver-nya menyeret grafik
// dependensi test (Docker, OpenTelemetry) yang besar hanya untuk kebutuhan yang
// sederhana. Kebutuhan kita cukup jelas — berkas SQL bernomor, satu tabel
// pencatat versi, terapkan berurutan — dan muat dalam ~150 baris yang bisa
// dibaca utuh. Penamaan berkas tetap kompatibel dengan CLI golang-migrate
// (`NNNNNN_judul.up.sql` / `.down.sql`) bila suatu saat perlu.
//
// Jaminan: setiap migrasi (kecuali yang ditandai no-transaction) dijalankan
// dalam SATU transaksi bersama pencatatan versinya. Bila ada satu statement
// gagal, seluruhnya batal dan tidak ada versi setengah jadi — jadi tidak perlu
// konsep "dirty" seperti golang-migrate.

// migrationsTable adalah tabel pencatat: satu baris per migrasi yang sudah
// diterapkan (bukan satu baris tunggal), sehingga riwayat penerapan terlihat.
const migrationsTable = "schema_migrations"

// noTxMarker menandai berkas .up.sql yang TIDAK boleh dibungkus transaksi —
// mis. berisi CREATE INDEX CONCURRENTLY (docs/TECHNICAL-BACKEND.md §4 aturan 5).
// Letakkan sebagai baris pertama berkas.
const noTxMarker = "-- migrate:no-transaction"

// migration adalah sepasang skrip naik/turun untuk satu versi.
type migration struct {
	version int64
	name    string
	upSQL   string
	downSQL string
	noTx    bool
}

// openMigrationDB membuka koneksi database khusus migrasi memakai protokol
// sederhana pgx, sehingga satu Exec boleh memuat banyak statement (DDL migrasi)
// dan blok dollar-quote ($$ ... $$) aman. Tanpa statement_timeout.
func openMigrationDB() (*sql.DB, error) {
	cfg, err := pgx.ParseConfig(MigrationDSNFromEnv())
	if err != nil {
		return nil, fmt.Errorf("parse konfigurasi koneksi migrasi: %w", err)
	}
	cfg.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol

	db := sql.OpenDB(stdlib.GetConnector(*cfg))
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("menghubungi database untuk migrasi: %w", err)
	}
	return db, nil
}

// loadMigrations membaca seluruh pasangan berkas dari migrations.FS, memvalidasi
// bahwa setiap .up.sql punya pasangan .down.sql, dan mengembalikannya terurut
// menaik menurut versi.
func loadMigrations() ([]migration, error) {
	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		return nil, fmt.Errorf("membaca direktori migrasi tersemat: %w", err)
	}

	byVersion := make(map[int64]*migration)
	order := []int64{}

	get := func(version int64, name string) *migration {
		m, ok := byVersion[version]
		if !ok {
			m = &migration{version: version, name: name}
			byVersion[version] = m
			order = append(order, version)
		}
		return m
	}

	for _, e := range entries {
		fn := e.Name()
		if e.IsDir() || !strings.HasSuffix(fn, ".sql") {
			continue
		}

		var direction string
		switch {
		case strings.HasSuffix(fn, ".up.sql"):
			direction = "up"
		case strings.HasSuffix(fn, ".down.sql"):
			direction = "down"
		default:
			return nil, fmt.Errorf("berkas migrasi %q harus berakhiran .up.sql atau .down.sql", fn)
		}

		base := strings.TrimSuffix(fn, "."+direction+".sql")
		underscore := strings.IndexByte(base, '_')
		if underscore <= 0 {
			return nil, fmt.Errorf("nama berkas migrasi %q harus berpola <versi>_<judul>", fn)
		}
		version, err := strconv.ParseInt(base[:underscore], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("versi tidak valid pada berkas %q: %w", fn, err)
		}

		body, err := fs.ReadFile(migrations.FS, fn)
		if err != nil {
			return nil, fmt.Errorf("membaca %q: %w", fn, err)
		}
		content := string(body)

		m := get(version, base[underscore+1:])
		if direction == "up" {
			m.upSQL = content
			m.noTx = strings.HasPrefix(strings.TrimSpace(content), noTxMarker)
		} else {
			m.downSQL = content
		}
	}

	sort.Slice(order, func(i, j int) bool { return order[i] < order[j] })
	result := make([]migration, 0, len(order))
	for _, v := range order {
		m := byVersion[v]
		if m.upSQL == "" {
			return nil, fmt.Errorf("migrasi %d (%s) tidak punya berkas .up.sql", m.version, m.name)
		}
		if m.downSQL == "" {
			return nil, fmt.Errorf("migrasi %d (%s) tidak punya berkas .down.sql", m.version, m.name)
		}
		result = append(result, *m)
	}
	return result, nil
}

// ensureMigrationsTable membuat tabel pencatat bila belum ada. Idempoten.
func ensureMigrationsTable(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS ` + migrationsTable + ` (
			version    BIGINT      PRIMARY KEY,
			name       TEXT        NOT NULL,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`)
	if err != nil {
		return fmt.Errorf("membuat tabel %s: %w", migrationsTable, err)
	}
	return nil
}

// appliedVersions mengembalikan himpunan versi yang sudah diterapkan.
func appliedVersions(db *sql.DB) (map[int64]bool, error) {
	rows, err := db.Query(`SELECT version FROM ` + migrationsTable + ` ORDER BY version`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := map[int64]bool{}
	for rows.Next() {
		var v int64
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		applied[v] = true
	}
	return applied, rows.Err()
}

// runOne menjalankan satu skrip migrasi (naik atau turun) beserta pencatatan
// versinya. Untuk migrasi transaksional, keduanya dibungkus BEGIN/COMMIT dalam
// satu Exec sehingga atomik.
func runOne(db *sql.DB, script string, bookkeeping string, noTx bool) error {
	if noTx {
		if _, err := db.Exec(script); err != nil {
			return err
		}
		_, err := db.Exec(bookkeeping)
		return err
	}
	_, err := db.Exec("BEGIN;\n" + script + "\n" + bookkeeping + "\nCOMMIT;")
	return err
}

// MigrateUp menerapkan seluruh migrasi yang belum diterapkan, berurutan menaik.
// Aman dipanggil berulang: bila sudah terkini, tidak melakukan apa-apa.
func MigrateUp() error {
	db, err := openMigrationDB()
	if err != nil {
		return err
	}
	defer db.Close()

	if err := ensureMigrationsTable(db); err != nil {
		return err
	}
	all, err := loadMigrations()
	if err != nil {
		return err
	}
	applied, err := appliedVersions(db)
	if err != nil {
		return err
	}

	pending := 0
	for _, m := range all {
		if applied[m.version] {
			continue
		}
		bookkeeping := fmt.Sprintf(
			"INSERT INTO %s (version, name) VALUES (%d, %s);",
			migrationsTable, m.version, quoteLiteral(m.name),
		)
		if err := runOne(db, m.upSQL, bookkeeping, m.noTx); err != nil {
			return fmt.Errorf("migrasi %d (%s) gagal: %w", m.version, m.name, err)
		}
		fmt.Printf("  ✓ %06d_%s\n", m.version, m.name)
		pending++
	}
	if pending == 0 {
		fmt.Println("  (tidak ada migrasi tertunda)")
	}
	return nil
}

// MigrateDown membatalkan `steps` migrasi terakhir yang diterapkan, berurutan
// menurun. `steps` harus > 0.
func MigrateDown(steps int) error {
	if steps <= 0 {
		return fmt.Errorf("jumlah langkah harus > 0, dapat %d", steps)
	}
	db, err := openMigrationDB()
	if err != nil {
		return err
	}
	defer db.Close()

	if err := ensureMigrationsTable(db); err != nil {
		return err
	}
	all, err := loadMigrations()
	if err != nil {
		return err
	}
	applied, err := appliedVersions(db)
	if err != nil {
		return err
	}

	// Susun daftar versi terpasang, menurun.
	descending := make([]migration, 0, len(all))
	for i := len(all) - 1; i >= 0; i-- {
		if applied[all[i].version] {
			descending = append(descending, all[i])
		}
	}
	if len(descending) == 0 {
		fmt.Println("  (tidak ada migrasi untuk dibatalkan)")
		return nil
	}
	if steps > len(descending) {
		steps = len(descending)
	}

	for _, m := range descending[:steps] {
		bookkeeping := fmt.Sprintf("DELETE FROM %s WHERE version = %d;", migrationsTable, m.version)
		if err := runOne(db, m.downSQL, bookkeeping, m.noTx); err != nil {
			return fmt.Errorf("pembatalan migrasi %d (%s) gagal: %w", m.version, m.name, err)
		}
		fmt.Printf("  ✓ turun %06d_%s\n", m.version, m.name)
	}
	return nil
}

// MigrationStatus melaporkan versi tertinggi yang terpasang dan jumlah migrasi
// yang masih tertunda.
func MigrationStatus() (current int64, pending int, err error) {
	db, err := openMigrationDB()
	if err != nil {
		return 0, 0, err
	}
	defer db.Close()

	if err := ensureMigrationsTable(db); err != nil {
		return 0, 0, err
	}
	all, err := loadMigrations()
	if err != nil {
		return 0, 0, err
	}
	applied, err := appliedVersions(db)
	if err != nil {
		return 0, 0, err
	}

	for _, m := range all {
		if applied[m.version] {
			if m.version > current {
				current = m.version
			}
		} else {
			pending++
		}
	}
	return current, pending, nil
}

// WarnIfMigrationsPending mengembalikan error deskriptif bila database tertinggal
// dari berkas migrasi yang di-deploy. SENGAJA hanya memperingatkan — InitDatabase
// tidak boleh mengubah skema saat aplikasi start (§4 aturan 6).
func WarnIfMigrationsPending() error {
	current, pending, err := MigrationStatus()
	if err != nil {
		return fmt.Errorf("tidak bisa memeriksa status migrasi: %w", err)
	}
	if pending > 0 {
		return fmt.Errorf("ada %d migrasi tertunda (versi terpasang: %d) — jalankan `go run ./cmd/migrate up`", pending, current)
	}
	return nil
}

// quoteLiteral membungkus s sebagai string literal PostgreSQL yang aman
// (menggandakan tanda kutip tunggal). Dipakai hanya untuk nama migrasi yang
// berasal dari nama berkas kita sendiri, bukan input pengguna — ini lapis
// pengaman, bukan pertahanan utama.
func quoteLiteral(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
