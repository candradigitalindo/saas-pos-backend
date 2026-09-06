package middlewares

import (
	"net/http"
	"sync"
	"time"

	"candra/backend-api/structs"

	"github.com/gin-gonic/gin"
)

// visitor melacak sisa token dan waktu isi ulang terakhir untuk satu IP.
type visitor struct {
	tokens   float64
	lastSeen time.Time
}

// rateLimiter adalah token-bucket per-IP sederhana yang tersimpan di memori.
// Cocok untuk melindungi endpoint auth dari brute-force pada deployment single-instance.
// Untuk multi-instance, ganti dengan store terpusat (mis. Redis).
type rateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	rate     float64       // token yang diisi ulang per detik
	burst    float64       // kapasitas maksimum bucket
	ttl      time.Duration // durasi idle sebelum entri dibersihkan
}

func newRateLimiter(rate, burst float64, ttl time.Duration) *rateLimiter {
	rl := &rateLimiter{
		visitors: make(map[string]*visitor),
		rate:     rate,
		burst:    burst,
		ttl:      ttl,
	}
	go rl.cleanupLoop()
	return rl
}

// allow mengurangi satu token untuk IP tertentu; mengembalikan false bila kuota habis.
func (rl *rateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	v, exists := rl.visitors[ip]
	if !exists {
		rl.visitors[ip] = &visitor{tokens: rl.burst - 1, lastSeen: now}
		return true
	}

	// Isi ulang token berdasarkan waktu yang berlalu.
	elapsed := now.Sub(v.lastSeen).Seconds()
	v.tokens += elapsed * rl.rate
	if v.tokens > rl.burst {
		v.tokens = rl.burst
	}
	v.lastSeen = now

	if v.tokens < 1 {
		return false
	}
	v.tokens--
	return true
}

// cleanupLoop membuang entri IP yang sudah lama tidak aktif agar map tidak tumbuh tanpa batas.
func (rl *rateLimiter) cleanupLoop() {
	ticker := time.NewTicker(rl.ttl)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		for ip, v := range rl.visitors {
			if time.Since(v.lastSeen) > rl.ttl {
				delete(rl.visitors, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// RateLimit membuat middleware pembatas laju berdasarkan IP klien.
//   - rate  : token yang diisi ulang per detik (laju permintaan berkelanjutan).
//   - burst : jumlah maksimum permintaan beruntun yang diizinkan.
func RateLimit(rate, burst float64) gin.HandlerFunc {
	limiter := newRateLimiter(rate, burst, 10*time.Minute)
	return func(c *gin.Context) {
		if !limiter.allow(c.ClientIP()) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, structs.ErrorResponse{
				Success: false,
				Message: "Terlalu banyak permintaan. Silakan coba lagi nanti.",
				Errors:  map[string]string{"rate_limit": "Batas permintaan terlampaui"},
			})
			return
		}
		c.Next()
	}
}
