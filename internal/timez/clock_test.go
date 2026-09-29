package timez

import (
	"encoding/json"
	"testing"
	"time"
)

func TestClockParseAndString(t *testing.T) {
	cases := []struct {
		in      string
		wantDur time.Duration
		wantHM  string
		wantErr bool
	}{
		{"00:00", 0, "00:00", false},
		{"04:00", 4 * time.Hour, "04:00", false},
		{"04:00:00", 4 * time.Hour, "04:00", false},
		{"23:59:59", 23*time.Hour + 59*time.Minute + 59*time.Second, "23:59", false},
		{"6:5", 6*time.Hour + 5*time.Minute, "06:05", false},
		{"24:00", 0, "", true},
		{"aa:bb", 0, "", true},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			c, err := ParseClock(tc.in)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ParseClock(%q) err=%v wantErr=%v", tc.in, err, tc.wantErr)
			}
			if err != nil {
				return
			}
			if c.Duration() != tc.wantDur {
				t.Fatalf("Duration = %v, mau %v", c.Duration(), tc.wantDur)
			}
			if c.String() != tc.wantHM {
				t.Fatalf("String = %q, mau %q", c.String(), tc.wantHM)
			}
		})
	}
}

// TestClockScanValueRoundTrip meniru siklus baca/tulis database di kedua mode
// driver: string (protokol sederhana) dan time.Time (protokol biner).
func TestClockScanValueRoundTrip(t *testing.T) {
	orig := NewClock(4*time.Hour + 30*time.Minute)

	v, err := orig.Value()
	if err != nil {
		t.Fatalf("Value() error: %v", err)
	}
	if v.(string) != "04:30:00" {
		t.Fatalf("Value = %q, mau 04:30:00", v)
	}

	// Scan dari string (yang dikembalikan pgx pada protokol sederhana).
	var fromStr Clock
	if err := fromStr.Scan("04:30:00"); err != nil {
		t.Fatalf("Scan(string) error: %v", err)
	}
	if fromStr != orig {
		t.Fatalf("round-trip string: %v != %v", fromStr, orig)
	}

	// Scan dari []byte.
	var fromBytes Clock
	if err := fromBytes.Scan([]byte("04:30:00")); err != nil {
		t.Fatalf("Scan([]byte) error: %v", err)
	}
	if fromBytes != orig {
		t.Fatalf("round-trip []byte: %v != %v", fromBytes, orig)
	}

	// Scan dari time.Time (protokol biner).
	var fromTime Clock
	if err := fromTime.Scan(time.Date(1, 1, 1, 4, 30, 0, 0, time.UTC)); err != nil {
		t.Fatalf("Scan(time.Time) error: %v", err)
	}
	if fromTime != orig {
		t.Fatalf("round-trip time.Time: %v != %v", fromTime, orig)
	}

	// Scan nil → nol.
	var fromNil Clock
	_ = fromNil.Scan(NewClock(time.Hour)) // set dulu bukan nol
	fromNil = Clock{}
	if err := fromNil.Scan(nil); err != nil {
		t.Fatalf("Scan(nil) error: %v", err)
	}
	if fromNil.Duration() != 0 {
		t.Fatalf("Scan(nil) → %v, mau 0", fromNil.Duration())
	}
}

func TestClockJSON(t *testing.T) {
	type wrap struct {
		Start Clock `json:"start"`
	}
	in := wrap{Start: NewClock(9*time.Hour + 15*time.Minute)}

	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(b) != `{"start":"09:15"}` {
		t.Fatalf("JSON = %s, mau {\"start\":\"09:15\"}", b)
	}

	var out wrap
	if err := json.Unmarshal([]byte(`{"start":"09:15:00"}`), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Start != in.Start {
		t.Fatalf("JSON round-trip: %v != %v", out.Start, in.Start)
	}
}
