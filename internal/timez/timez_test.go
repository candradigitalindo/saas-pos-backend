package timez

import (
	"testing"
	"time"
)

// mustTime mengurai RFC 3339 atau menggagalkan test.
func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	ts, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse waktu %q: %v", s, err)
	}
	return ts
}

// TestBusinessDateAcrossTimezones adalah test yang diwajibkan
// docs/TECHNICAL-BACKEND.md §14: WIB/WITA/WIT dengan dan tanpa batas hari 04:00.
func TestBusinessDateAcrossTimezones(t *testing.T) {
	cases := []struct {
		name       string
		occurredAt string // RFC 3339, boleh berzona apa pun
		tz         string
		dayStart   time.Duration
		want       string // YYYY-MM-DD
	}{
		{
			name:       "Jayapura dini hari masih hari yang sama (kasus contoh di dokumen)",
			occurredAt: "2026-09-07T07:00:00+09:00", // = 2026-09-06T22:00:00Z
			tz:         WIT,
			dayStart:   0,
			want:       "2026-09-07",
		},
		{
			name:       "WIB pagi, UTC masih kemarin",
			occurredAt: "2026-09-07T08:00:00+07:00", // = 2026-09-07T01:00:00Z
			tz:         WIB,
			dayStart:   0,
			want:       "2026-09-07",
		},
		{
			name:       "WIT malam, UTC hari yang sama",
			occurredAt: "2026-09-07T23:30:00+09:00", // = 2026-09-07T14:30:00Z
			tz:         WIT,
			dayStart:   0,
			want:       "2026-09-07",
		},
		{
			name:       "input UTC dikonversi ke WITA",
			occurredAt: "2026-09-07T16:10:00Z", // = 2026-09-08T00:10:00 WITA
			tz:         WITA,
			dayStart:   0,
			want:       "2026-09-08",
		},
		{
			name:       "batas hari 04:00 — transaksi 02:00 lokal masuk hari sebelumnya",
			occurredAt: "2026-09-07T02:00:00+07:00",
			tz:         WIB,
			dayStart:   4 * time.Hour,
			want:       "2026-09-06",
		},
		{
			name:       "batas hari 04:00 — transaksi 05:00 lokal masuk hari itu",
			occurredAt: "2026-09-07T05:00:00+07:00",
			tz:         WIB,
			dayStart:   4 * time.Hour,
			want:       "2026-09-07",
		},
		{
			name:       "batas hari 04:00 — tepat 04:00 lokal masuk hari itu",
			occurredAt: "2026-09-07T04:00:00+08:00",
			tz:         WITA,
			dayStart:   4 * time.Hour,
			want:       "2026-09-07",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := BusinessDate(mustTime(t, tc.occurredAt), tc.tz, tc.dayStart)
			if err != nil {
				t.Fatalf("BusinessDate() error: %v", err)
			}
			if got.Location() != time.UTC {
				t.Fatalf("hasil bukan UTC: %v", got.Location())
			}
			if h, m, s := got.Clock(); h != 0 || m != 0 || s != 0 {
				t.Fatalf("hasil bukan tengah malam: %v", got)
			}
			if got.Format("2006-01-02") != tc.want {
				t.Fatalf("BusinessDate = %s, mau %s", got.Format("2006-01-02"), tc.want)
			}
		})
	}
}

// TestBusinessDateRejectsBadInput menutup jalur error.
func TestBusinessDateRejectsBadInput(t *testing.T) {
	now := time.Now()
	if _, err := BusinessDate(now, "America/New_York", 0); err == nil {
		t.Fatal("zona di luar Indonesia seharusnya ditolak")
	}
	if _, err := BusinessDate(now, WIB, -time.Hour); err == nil {
		t.Fatal("dayStart negatif seharusnya ditolak")
	}
	if _, err := BusinessDate(now, WIB, 24*time.Hour); err == nil {
		t.Fatal("dayStart >= 24 jam seharusnya ditolak")
	}
}

// TestDayRangeUTC memastikan rentang laporan sepanjang tepat 24 jam dan
// batasnya cocok dengan BusinessDate.
func TestDayRangeUTC(t *testing.T) {
	localDate := mustTime(t, "2026-09-07T00:00:00Z")

	from, to, err := DayRangeUTC(localDate, WIT, 0)
	if err != nil {
		t.Fatalf("DayRangeUTC() error: %v", err)
	}
	if to.Sub(from) != 24*time.Hour {
		t.Fatalf("panjang rentang = %v, mau 24 jam", to.Sub(from))
	}
	// 2026-09-07 00:00 WIT = 2026-09-06 15:00Z
	if want := mustTime(t, "2026-09-06T15:00:00Z"); !from.Equal(want) {
		t.Fatalf("from = %v, mau %v", from, want)
	}

	// Sebuah waktu tepat di dalam rentang harus menghasilkan business_date
	// yang sama dengan tanggal yang diminta.
	mid := from.Add(12 * time.Hour)
	bd, err := BusinessDate(mid, WIT, 0)
	if err != nil {
		t.Fatalf("BusinessDate() error: %v", err)
	}
	if bd.Format("2006-01-02") != "2026-09-07" {
		t.Fatalf("business_date titik tengah = %s, mau 2026-09-07", bd.Format("2006-01-02"))
	}
}

// TestDayRangeUTCWithDayStart memastikan pergeseran awal hari ikut tercermin di
// rentang laporan.
func TestDayRangeUTCWithDayStart(t *testing.T) {
	localDate := mustTime(t, "2026-09-07T00:00:00Z")
	from, to, err := DayRangeUTC(localDate, WIB, 4*time.Hour)
	if err != nil {
		t.Fatalf("DayRangeUTC() error: %v", err)
	}
	// 2026-09-07 04:00 WIB = 2026-09-06 21:00Z
	if want := mustTime(t, "2026-09-06T21:00:00Z"); !from.Equal(want) {
		t.Fatalf("from = %v, mau %v", from, want)
	}
	if to.Sub(from) != 24*time.Hour {
		t.Fatalf("panjang rentang = %v, mau 24 jam", to.Sub(from))
	}
}

func TestParseDayStart(t *testing.T) {
	cases := []struct {
		in      string
		want    time.Duration
		wantErr bool
	}{
		{"00:00", 0, false},
		{"04:00", 4 * time.Hour, false},
		{"04:00:00", 4 * time.Hour, false},
		{"23:59:59", 23*time.Hour + 59*time.Minute + 59*time.Second, false},
		{"  06:30  ", 6*time.Hour + 30*time.Minute, false},
		{"", 0, false},
		{"24:00", 0, true},
		{"12", 0, true},
		{"12:60", 0, true},
		{"aa:bb", 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, err := ParseDayStart(tc.in)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ParseDayStart(%q) err = %v, wantErr %v", tc.in, err, tc.wantErr)
			}
			if err == nil && got != tc.want {
				t.Fatalf("ParseDayStart(%q) = %v, mau %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestIsSupportedTimezone(t *testing.T) {
	for _, tz := range SupportedTimezones() {
		if !IsSupportedTimezone(tz) {
			t.Fatalf("%q seharusnya didukung", tz)
		}
	}
	for _, tz := range []string{"", "UTC", "Local", "Asia/Bangkok", "asia/jakarta"} {
		if IsSupportedTimezone(tz) {
			t.Fatalf("%q seharusnya TIDAK didukung", tz)
		}
	}
}
