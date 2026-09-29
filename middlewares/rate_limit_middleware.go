package middlewares

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"candra/backend-api/config"
	"candra/backend-api/structs"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// Pembatas laju per-IP dengan algoritma token-bucket.
//
// Dua backend, dipilih SEKALI saat start lewat InitRateLimiter():
//   - memori (default) — cukup untuk satu instance; batas jadi N× nilai
//     konfigurasi bila ada N replika, dan restart mereset bucket.
//   - Redis (RATELIMIT_REDIS_URL diisi) — penegakan lintas-instance. Bila Redis
//     tak terjangkau SAAT permintaan, middleware fail-open (mengizinkan) dan
//     mencatat warning: satu blip Redis tidak boleh mengunci semua orang.
//     Bila URL diisi tapi tak terjangkau SAAT START, aplikasi fatal (Anda minta
//     Redis; ia tidak ada).

// limiterStore adalah backend penyimpanan bucket. name memisahkan grup pembatas
// (auth / channel-webhook / partner-login) agar tidak berbagi kuota.
type limiterStore interface {
	allow(name, ip string, rate, burst float64) bool
}

// store dipilih InitRateLimiter(); default memori agar test & cmd tetap jalan
// tanpa memanggil init.
var store limiterStore = newMemoryStore(10 * time.Minute)

// InitRateLimiter memilih backend pembatas laju. Dipanggil main.go setelah
// LoadEnv()+InitLogger(). cmd/* dan test tidak memanggilnya (tetap memori).
func InitRateLimiter() {
	url := config.GetEnv("RATELIMIT_REDIS_URL", "")
	if url == "" {
		slog.Info("rate limiter: backend memori (per-instance)")
		return
	}
	opt, err := redis.ParseURL(url)
	if err != nil {
		log.Fatalf("RATELIMIT_REDIS_URL tidak valid: %v", err)
	}
	client := redis.NewClient(opt)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		log.Fatalf("rate limiter: Redis tak terjangkau (%s): %v", opt.Addr, err)
	}
	store = &redisStore{client: client}
	slog.Info("rate limiter: backend Redis", "addr", opt.Addr)
}

// RateLimit membuat middleware pembatas laju per-IP untuk grup bernama `name`.
//   - rate  : token diisi ulang per detik (laju berkelanjutan).
//   - burst : jumlah maksimum permintaan beruntun.
func RateLimit(name string, rate, burst float64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !store.allow(name, c.ClientIP(), rate, burst) {
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

// ── Backend memori ───────────────────────────────────────────────────────

type visitor struct {
	tokens   float64
	lastSeen time.Time
}

type memoryStore struct {
	mu       sync.Mutex
	visitors map[string]*visitor // key: name + "\x00" + ip
	ttl      time.Duration
}

func newMemoryStore(ttl time.Duration) *memoryStore {
	m := &memoryStore{visitors: make(map[string]*visitor), ttl: ttl}
	go m.cleanupLoop()
	return m
}

func (m *memoryStore) allow(name, ip string, rate, burst float64) bool {
	key := name + "\x00" + ip
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	v, ok := m.visitors[key]
	if !ok {
		m.visitors[key] = &visitor{tokens: burst - 1, lastSeen: now}
		return true
	}
	v.tokens += now.Sub(v.lastSeen).Seconds() * rate
	if v.tokens > burst {
		v.tokens = burst
	}
	v.lastSeen = now
	if v.tokens < 1 {
		return false
	}
	v.tokens--
	return true
}

func (m *memoryStore) cleanupLoop() {
	ticker := time.NewTicker(m.ttl)
	defer ticker.Stop()
	for range ticker.C {
		m.mu.Lock()
		for k, v := range m.visitors {
			if time.Since(v.lastSeen) > m.ttl {
				delete(m.visitors, k)
			}
		}
		m.mu.Unlock()
	}
}

// ── Backend Redis ────────────────────────────────────────────────────────

type redisStore struct{ client *redis.Client }

// tokenBucketScript: token-bucket atomik dalam satu round-trip. Semantik sama
// dengan backend memori (mulai penuh di `burst`, isi ulang `rate`/detik,
// konsumsi 1). Balikan 1 = diizinkan, 0 = ditolak.
var tokenBucketScript = redis.NewScript(`
local rate  = tonumber(ARGV[1])
local burst = tonumber(ARGV[2])
local now   = tonumber(ARGV[3])
local ttl   = tonumber(ARGV[4])
local d = redis.call('HMGET', KEYS[1], 'tokens', 'ts')
local tokens = tonumber(d[1])
local ts = tonumber(d[2])
if tokens == nil then tokens = burst; ts = now end
tokens = math.min(burst, tokens + math.max(0, now - ts) * rate)
local allowed = 0
if tokens >= 1 then tokens = tokens - 1; allowed = 1 end
redis.call('HMSET', KEYS[1], 'tokens', tokens, 'ts', now)
redis.call('EXPIRE', KEYS[1], ttl)
return allowed
`)

func (r *redisStore) allow(name, ip string, rate, burst float64) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	key := "ratelimit:" + name + ":" + ip
	// TTL bucket: cukup untuk terisi penuh kembali dari kosong, minimal 60 dtk.
	ttl := int(burst/rate) + 1
	if ttl < 60 {
		ttl = 60
	}
	res, err := tokenBucketScript.Run(ctx, r.client, []string{key},
		rate, burst, float64(time.Now().UnixNano())/1e9, ttl).Int()
	if err != nil {
		// Redis bermasalah → jangan kunci pengguna; izinkan + catat.
		slog.Warn("rate limiter: Redis error, fail-open", "error", err, "key", key)
		return true
	}
	return res == 1
}
