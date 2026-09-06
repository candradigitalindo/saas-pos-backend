// Package timez memusatkan konversi zona waktu dan perhitungan "tanggal usaha"
// (business_date). Ini adalah implementasi aturan mengikat #4 dan #5 di
// docs/TECHNICAL-BACKEND.md: semua waktu disimpan UTC, tetapi "penjualan hari
// ini" dihitung menurut zona waktu OUTLET, bukan zona server.
//
// Aturan pemakaian:
//
//   - Di dalam proses, time.Time selalu UTC. Konversi ke zona lokal hanya terjadi
//     di sini, sesaat, untuk menghitung tanggal — hasilnya kembali sebagai
//     tanggal murni (tengah malam UTC), bukan waktu berzona.
//   - Nama zona SELALU berupa string IANA ("Asia/Makassar"), tidak pernah offset
//     angka ("+08:00"). Offset bisa berubah bila aturan negara berubah; nama IANA
//     tetap benar.
package timez

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Zona waktu Indonesia yang didukung, dalam nama IANA.
//
//   - Asia/Jakarta   → WIB  (UTC+7)
//   - Asia/Makassar  → WITA (UTC+8)
//   - Asia/Jayapura  → WIT  (UTC+9)
//
// Daftar ini sengaja pendek: outlet baru hanya boleh memilih dari sini
// (divalidasi di service saat membuat/mengubah outlet). Menambah zona = menambah
// satu baris, bukan mengubah logika.
const (
	WIB  = "Asia/Jakarta"
	WITA = "Asia/Makassar"
	WIT  = "Asia/Jayapura"
)

// supported memetakan nama IANA yang diizinkan ke *time.Location yang sudah
// dimuat sekali saat paket diinisialisasi. LoadLocation relatif mahal (baca &
// parse file zoneinfo); melakukannya sekali di sini menghindari biaya itu di
// jalur checkout.
var supported = func() map[string]*time.Location {
	m := make(map[string]*time.Location, 3)
	for _, name := range []string{WIB, WITA, WIT} {
		loc, err := time.LoadLocation(name)
		if err != nil {
			// Hanya mungkin bila tzdata benar-benar tidak tersedia — dan
			// tzdata.go seharusnya mencegahnya. Panik saat start lebih baik
			// daripada business_date yang salah diam-diam saat runtime.
			panic(fmt.Sprintf("timez: gagal memuat zona %q: %v", name, err))
		}
		m[name] = loc
	}
	return m
}()

// SupportedTimezones mengembalikan salinan daftar nama zona IANA yang didukung,
// untuk dipakai lapisan validasi dan endpoint yang menampilkan pilihan ke UI.
func SupportedTimezones() []string {
	return []string{WIB, WITA, WIT}
}

// IsSupportedTimezone melaporkan apakah tz adalah salah satu zona Indonesia yang
// didukung. Dipakai saat memvalidasi input pembuatan/perubahan outlet.
func IsSupportedTimezone(tz string) bool {
	_, ok := supported[tz]
	return ok
}

// LoadLocation mengembalikan *time.Location untuk tz bila tz termasuk zona yang
// didukung. Berbeda dari time.LoadLocation: menolak zona di luar daftar (mis.
// "America/New_York" atau "Local") supaya data outlet tidak pernah menyimpan
// zona yang tak bermakna untuk produk ini.
func LoadLocation(tz string) (*time.Location, error) {
	loc, ok := supported[tz]
	if !ok {
		return nil, fmt.Errorf("timez: zona waktu tidak didukung: %q", tz)
	}
	return loc, nil
}

// BusinessDate menghitung tanggal usaha dari sebuah stempel waktu.
//
// Parameter:
//
//   - occurredAt: waktu kejadian menurut server, dalam zona apa pun (akan
//     dikonversi). Sumbernya adalah satu time.Now().UTC() per transaksi.
//   - tz:         nama IANA zona outlet (lihat konstanta WIB/WITA/WIT).
//   - dayStart:   pergeseran awal hari usaha dari tengah malam, mis. 4*time.Hour
//     untuk warung yang tutup pukul 02:00 dan menganggap transaksi dini hari
//     sebagai "kemarin". 0 untuk mayoritas usaha. Wajib di rentang [0, 24 jam).
//
// Hasil: time.Time pada pukul 00:00:00 UTC dari tanggal usaha tersebut. Nilai
// ini yang disimpan apa adanya ke kolom `business_date DATE`. Komponen jam/menit
// nol dan lokasi UTC dipilih sengaja supaya perbandingan tanggal tidak pernah
// terpengaruh zona.
//
// Contoh yang wajib benar (docs/TECHNICAL-BACKEND.md §14):
//
//	Outlet Jayapura (Asia/Jayapura, UTC+9), dayStart 0.
//	occurredAt = 2026-09-07T07:00:00+09:00  = 2026-09-06T22:00:00Z
//	BusinessDate → 2026-09-07  (bukan 2026-09-06)
func BusinessDate(occurredAt time.Time, tz string, dayStart time.Duration) (time.Time, error) {
	if dayStart < 0 || dayStart >= 24*time.Hour {
		return time.Time{}, fmt.Errorf("timez: dayStart harus di rentang [0, 24 jam), dapat %s", dayStart)
	}
	loc, err := LoadLocation(tz)
	if err != nil {
		return time.Time{}, err
	}

	// Geser mundur sebesar dayStart lalu ambil komponen tanggalnya di zona
	// lokal. Transaksi pukul 03:00 dengan dayStart 04:00 menjadi 23:00 hari
	// sebelumnya → tanggal usaha ikut mundur satu hari, sesuai maksud.
	local := occurredAt.In(loc).Add(-dayStart)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC), nil
}

// DayRangeUTC mengembalikan rentang [from, to) dalam UTC yang mencakup satu
// tanggal usaha lokal pada sebuah outlet. Dipakai endpoint laporan: klien
// mengirim tanggal lokal, server menerjemahkannya ke rentang UTC untuk query
// `occurred_at >= from AND occurred_at < to`.
//
//   - localDate: tanggal lokal; hanya komponen tahun-bulan-hari yang dipakai.
//   - to bersifat eksklusif (awal hari usaha berikutnya).
//
// Untuk laporan lintas outlet beda zona, JANGAN menggabungkan rentang UTC dari
// fungsi ini — gabungkan berdasarkan kolom business_date, sesuai §3.2.
func DayRangeUTC(localDate time.Time, tz string, dayStart time.Duration) (from, to time.Time, err error) {
	if dayStart < 0 || dayStart >= 24*time.Hour {
		return time.Time{}, time.Time{}, fmt.Errorf("timez: dayStart harus di rentang [0, 24 jam), dapat %s", dayStart)
	}
	loc, err := LoadLocation(tz)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}

	y, m, d := localDate.Date()
	// Awal hari usaha = tengah malam lokal + dayStart, ditafsirkan di zona
	// outlet, lalu dinormalkan ke UTC.
	from = time.Date(y, m, d, 0, 0, 0, 0, loc).Add(dayStart).UTC()
	to = time.Date(y, m, d+1, 0, 0, 0, 0, loc).Add(dayStart).UTC()
	return from, to, nil
}

// ParseDayStart mengubah nilai kolom `business_day_start` (format "HH:MM" atau
// "HH:MM:SS", seperti dikembalikan PostgreSQL untuk tipe TIME) menjadi
// time.Duration yang bisa dipakai BusinessDate dan DayRangeUTC.
func ParseDayStart(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	parts := strings.Split(s, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, fmt.Errorf("timez: format waktu tidak valid: %q", s)
	}

	hh, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, fmt.Errorf("timez: jam tidak valid pada %q: %w", s, err)
	}
	mm, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, fmt.Errorf("timez: menit tidak valid pada %q: %w", s, err)
	}
	ss := 0
	if len(parts) == 3 {
		if ss, err = strconv.Atoi(parts[2]); err != nil {
			return 0, fmt.Errorf("timez: detik tidak valid pada %q: %w", s, err)
		}
	}

	if hh < 0 || hh > 23 || mm < 0 || mm > 59 || ss < 0 || ss > 59 {
		return 0, fmt.Errorf("timez: komponen waktu di luar rentang: %q", s)
	}
	return time.Duration(hh)*time.Hour + time.Duration(mm)*time.Minute + time.Duration(ss)*time.Second, nil
}
