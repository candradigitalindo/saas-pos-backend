package timez

import (
	"database/sql/driver"
	"fmt"
	"time"
)

// Clock adalah waktu-hari (jam:menit:detik) tanpa tanggal, sebagai pergeseran
// dari tengah malam di rentang [0, 24 jam). Dipakai untuk kolom PostgreSQL
// bertipe TIME seperti outlets.business_day_start.
//
// Kenapa tipe khusus, bukan time.Time: driver pgx mengembalikan TIME sebagai
// string pada protokol sederhana dan sebagai nilai lain pada protokol biner —
// keduanya gagal di-scan langsung ke *time.Time. Clock mengimplementasikan
// sql.Scanner & driver.Valuer sehingga round-trip-nya andal di kedua mode.
type Clock struct {
	d time.Duration
}

// NewClock membuat Clock dari pergeseran terhadap tengah malam. Nilai di luar
// [0, 24 jam) dinormalkan.
func NewClock(d time.Duration) Clock {
	d %= 24 * time.Hour
	if d < 0 {
		d += 24 * time.Hour
	}
	return Clock{d: d}
}

// ParseClock mengurai "HH:MM" atau "HH:MM:SS".
func ParseClock(s string) (Clock, error) {
	d, err := ParseDayStart(s)
	if err != nil {
		return Clock{}, err
	}
	return NewClock(d), nil
}

// Duration mengembalikan pergeseran dari tengah malam.
func (c Clock) Duration() time.Duration { return c.d }

// String mengembalikan "HH:MM" (untuk response API).
func (c Clock) String() string {
	h := int(c.d / time.Hour)
	m := int((c.d % time.Hour) / time.Minute)
	return fmt.Sprintf("%02d:%02d", h, m)
}

// hms mengembalikan "HH:MM:SS" (untuk disimpan ke kolom TIME).
func (c Clock) hms() string {
	h := int(c.d / time.Hour)
	m := int((c.d % time.Hour) / time.Minute)
	s := int((c.d % time.Minute) / time.Second)
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}

// Value mengimplementasikan driver.Valuer — selalu menulis "HH:MM:SS".
func (c Clock) Value() (driver.Value, error) {
	return c.hms(), nil
}

// Scan mengimplementasikan sql.Scanner. Menerima nil, string/[]byte ("HH:MM:SS"
// atau "HH:MM"), maupun time.Time (mode biner pgx) dan pgtype yang punya
// representasi time.Time.
func (c *Clock) Scan(src any) error {
	switch v := src.(type) {
	case nil:
		*c = Clock{}
		return nil
	case string:
		parsed, err := ParseClock(v)
		if err != nil {
			return err
		}
		*c = parsed
		return nil
	case []byte:
		parsed, err := ParseClock(string(v))
		if err != nil {
			return err
		}
		*c = parsed
		return nil
	case time.Time:
		h, m, s := v.Clock()
		*c = NewClock(time.Duration(h)*time.Hour + time.Duration(m)*time.Minute + time.Duration(s)*time.Second)
		return nil
	default:
		return fmt.Errorf("timez: tidak bisa Scan %T ke Clock", src)
	}
}

// MarshalJSON menyajikan Clock sebagai string "HH:MM" di JSON.
func (c Clock) MarshalJSON() ([]byte, error) {
	return []byte(`"` + c.String() + `"`), nil
}

// UnmarshalJSON menerima string "HH:MM" / "HH:MM:SS".
func (c *Clock) UnmarshalJSON(b []byte) error {
	if len(b) < 2 || b[0] != '"' {
		return fmt.Errorf("timez: Clock JSON harus string")
	}
	parsed, err := ParseClock(string(b[1 : len(b)-1]))
	if err != nil {
		return err
	}
	*c = parsed
	return nil
}
