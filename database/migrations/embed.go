// Package migrations menyimpan berkas SQL migrasi berversi dan menyematkannya ke
// dalam binary lewat go:embed.
//
// Kenapa disematkan, bukan dibaca dari disk saat runtime:
//
//   - `cmd/migrate` bisa dijalankan dari direktori mana pun tanpa perlu
//     menyertakan folder database/migrations di sampingnya.
//   - Versi migrasi yang dijalankan dijamin sama persis dengan versi kode yang
//     di-deploy — tidak ada celah "berkas migrasi di server berbeda".
//
// Aturan berkas (lihat docs/TECHNICAL-BACKEND.md §4):
//   - Nama: <versi 6 digit>_<judul>.up.sql dan pasangan .down.sql-nya.
//   - Satu migrasi = satu perubahan logis.
//   - Setiap .up.sql WAJIB punya .down.sql yang benar-benar diuji.
package migrations

import "embed"

// FS berisi seluruh berkas *.sql di direktori ini. Dikonsumsi oleh
// golang-migrate lewat sumber "iofs".
//
//go:embed *.sql
var FS embed.FS
