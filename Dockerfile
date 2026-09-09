# syntax=docker/dockerfile:1

# ── Build ────────────────────────────────────────────────────────────────
# Satu tahap build menghasilkan SEMUA binari: server + runner migrasi +
# pekerjaan terjadwal. CGO dimatikan (murni Go — pgx tidak butuh libpq), jadi
# binarinya statis dan bisa jalan di image tanpa glibc.
FROM golang:1.24 AS build

WORKDIR /src

# Lapisan cache dependensi: hanya diunduh ulang bila go.mod/go.sum berubah.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
ENV CGO_ENABLED=0 GOOS=linux
RUN go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /out/pos-server . && \
    go build -trimpath -ldflags="-s -w" -o /out/migrate               ./cmd/migrate && \
    go build -trimpath -ldflags="-s -w" -o /out/recognize-revenue     ./cmd/recognize-revenue && \
    go build -trimpath -ldflags="-s -w" -o /out/process-channel-events ./cmd/process-channel-events && \
    go build -trimpath -ldflags="-s -w" -o /out/partner-commissions    ./cmd/partner-commissions && \
    go build -trimpath -ldflags="-s -w" -o /out/partner-admin          ./cmd/partner-admin && \
    go build -trimpath -ldflags="-s -w" -o /out/platform-admin         ./cmd/platform-admin && \
    go build -trimpath -ldflags="-s -w" -o /out/process-outbox         ./cmd/process-outbox

# ── Runtime ──────────────────────────────────────────────────────────────
# distroless: hanya CA certs + tzdata + user non-root. Tidak ada shell, tidak
# ada package manager — permukaan serangan minimal.
FROM gcr.io/distroless/static-debian12:nonroot

# Zona waktu: server & DB selalu UTC, tapi internal/timez butuh tzdata untuk
# menghitung business_date per outlet (Asia/Jakarta dst).
COPY --from=build /usr/share/zoneinfo /usr/share/zoneinfo
ENV TZ=UTC

COPY --from=build /out/ /usr/local/bin/

# Migrasi TIDAK dijalankan saat start (docs/TECHNICAL-BACKEND.md aturan #10) —
# jalankan `docker run --entrypoint /usr/local/bin/migrate <image> up` sebagai
# langkah deploy terpisah sebelum merilis versi baru.
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/pos-server"]
