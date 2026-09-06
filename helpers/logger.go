package helpers

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"candra/backend-api/config"
)

// loggerCtxKey adalah kunci privat untuk menyimpan *slog.Logger di context.
// Tipe khusus (bukan string) mencegah tabrakan dengan kunci context paket lain.
type loggerCtxKey struct{}

// InitLogger memasang logger default proses: JSON terstruktur ke stdout dengan
// level dari LOG_LEVEL (docs/TECHNICAL-BACKEND.md §15).
//
// Format:
//   - LOG_FORMAT=json (default) — untuk agregator log di produksi.
//   - LOG_FORMAT=text          — lebih enak dibaca manusia saat development.
//
// Dipanggil sekali di main.go setelah LoadEnv(). Setelah ini, slog.Info/Error di
// mana pun akan memakai konfigurasi ini.
func InitLogger() {
	opts := &slog.HandlerOptions{Level: parseLevel(config.GetEnv("LOG_LEVEL", "info"))}

	var handler slog.Handler
	if strings.EqualFold(config.GetEnv("LOG_FORMAT", "json"), "text") {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	slog.SetDefault(slog.New(handler))
}

// parseLevel menerjemahkan string level ke slog.Level. Nilai tak dikenal jatuh
// ke Info — logging tidak boleh gagal hanya karena salah ketik konfigurasi.
func parseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// ContextWithLogger menautkan logger (biasanya sudah diberi atribut request_id,
// tenant_id, user_id) ke context, agar lapisan di bawah controller bisa
// mengambilnya lewat LoggerFromContext tanpa meneruskan *slog.Logger manual.
func ContextWithLogger(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerCtxKey{}, l)
}

// LoggerFromContext mengambil logger request-scoped dari context. Bila tidak ada
// (mis. dipanggil dari pekerja latar atau test), mengembalikan slog.Default()
// sehingga pemanggil tidak perlu memeriksa nil.
func LoggerFromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(loggerCtxKey{}).(*slog.Logger); ok && l != nil {
		return l
	}
	return slog.Default()
}
