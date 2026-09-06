package ulid

import (
	"sync"
	"testing"
	"time"
)

// TestNewProducesValidSortableIDs memastikan New() menghasilkan ULID yang lolos
// IsValid dan terurut menaik ketika dibuat berurutan — sifat yang diandalkan
// paginasi kursor sinkronisasi.
func TestNewProducesValidSortableIDs(t *testing.T) {
	const n = 1000
	prev := ""
	for i := 0; i < n; i++ {
		id := New()
		if !IsValid(id) {
			t.Fatalf("New() menghasilkan ID tidak valid: %q", id)
		}
		if len(id) != Len {
			t.Fatalf("panjang ID = %d, mau %d", len(id), Len)
		}
		if prev != "" && id <= prev {
			t.Fatalf("ID tidak monoton: %q <= %q", id, prev)
		}
		prev = id
	}
}

// TestNewConcurrent menjalankan New() dari banyak goroutine untuk memastikan
// tidak ada data race pada entropy (jalankan test dengan -race).
func TestNewConcurrent(t *testing.T) {
	const goroutines = 50
	const perGoroutine = 200

	var wg sync.WaitGroup
	seen := sync.Map{}
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				id := New()
				if _, dup := seen.LoadOrStore(id, struct{}{}); dup {
					t.Errorf("ULID duplikat: %q", id)
				}
			}
		}()
	}
	wg.Wait()
}

// TestIsValid menutup kasus batas yang penting untuk validasi ID kiriman klien.
func TestIsValid(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"ULID sah", "01ARZ3NDEKTSV4RRFFQ69G5FAV", true},
		{"huruf kecil dinormalkan Crockford", "01arz3ndektsv4rrffq69g5fav", true},
		{"kosong", "", false},
		{"terlalu pendek", "01ARZ3NDEKTSV4RRFFQ69G5FA", false},
		{"terlalu panjang", "01ARZ3NDEKTSV4RRFFQ69G5FAVX", false},
		{"karakter di luar alfabet (I O U L)", "01ARZ3NDEKTSV4RRFFQ69G5FAI", false},
		{"spasi", "01ARZ3NDEKTSV4RRFFQ69G5FA ", false},
		{"overflow waktu (7ZZ...)", "7ZZZZZZZZZZZZZZZZZZZZZZZZZZ", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsValid(tc.in); got != tc.want {
				t.Fatalf("IsValid(%q) = %v, mau %v", tc.in, got, tc.want)
			}
		})
	}
}

// TestTimeOfRoundTrip memastikan komponen waktu yang tertanam bisa dibaca
// kembali dengan presisi milidetik (ULID hanya menyimpan ms).
func TestTimeOfRoundTrip(t *testing.T) {
	id := New()
	got, err := TimeOf(id)
	if err != nil {
		t.Fatalf("TimeOf() error: %v", err)
	}
	if got.Location() != time.UTC {
		t.Fatalf("TimeOf() lokasi = %v, mau UTC", got.Location())
	}
	if d := time.Since(got); d < 0 || d > 5*time.Second {
		t.Fatalf("selisih waktu tidak masuk akal: %v", d)
	}
}

// TestParseRejectsMalformed memastikan Parse mengembalikan error, bukan panik,
// untuk masukan buruk.
func TestParseRejectsMalformed(t *testing.T) {
	if _, err := Parse("bukan-ulid"); err == nil {
		t.Fatal("Parse() menerima string yang jelas tidak valid")
	}
}
