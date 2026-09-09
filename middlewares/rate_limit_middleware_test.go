package middlewares

import (
	"testing"
	"time"
)

// Uji unit backend memori pembatas laju (tanpa database, tanpa HTTP).

func TestMemoryStoreBurstThenBlock(t *testing.T) {
	m := &memoryStore{visitors: map[string]*visitor{}, ttl: time.Minute}

	// burst 3, laju 0 → tepat 3 permintaan lolos, sisanya ditolak.
	for i := 0; i < 3; i++ {
		if !m.allow("auth", "1.2.3.4", 0, 3) {
			t.Fatalf("permintaan ke-%d seharusnya lolos", i+1)
		}
	}
	if m.allow("auth", "1.2.3.4", 0, 3) {
		t.Fatal("permintaan ke-4 seharusnya ditolak (bucket kosong)")
	}
}

func TestMemoryStoreRefill(t *testing.T) {
	m := &memoryStore{visitors: map[string]*visitor{}, ttl: time.Minute}

	// Habiskan bucket burst 1.
	if !m.allow("auth", "9.9.9.9", 100, 1) {
		t.Fatal("permintaan pertama harus lolos")
	}
	if m.allow("auth", "9.9.9.9", 100, 1) {
		t.Fatal("permintaan kedua langsung harus ditolak")
	}
	// Curangi waktu lastSeen ke masa lalu → laju 100/dtk mengisi >1 token.
	m.visitors["auth\x009.9.9.9"].lastSeen = time.Now().Add(-time.Second)
	if !m.allow("auth", "9.9.9.9", 100, 1) {
		t.Fatal("setelah isi ulang, permintaan harus lolos lagi")
	}
}

func TestMemoryStoreIsolatesByNameAndIP(t *testing.T) {
	m := &memoryStore{visitors: map[string]*visitor{}, ttl: time.Minute}

	// Grup & IP yang berbeda tidak berbagi kuota.
	if !m.allow("auth", "1.1.1.1", 0, 1) || !m.allow("webhook", "1.1.1.1", 0, 1) ||
		!m.allow("auth", "2.2.2.2", 0, 1) {
		t.Fatal("bucket antar grup/IP tidak boleh berbagi kuota")
	}
	if m.allow("auth", "1.1.1.1", 0, 1) {
		t.Fatal("bucket (auth,1.1.1.1) sudah kosong, harus ditolak")
	}
}
