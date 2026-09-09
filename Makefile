# SaaS POS UMKM — perintah pengembangan & rilis.
# `make help` menampilkan daftar target.

GO      ?= go
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
IMAGE   ?= saas-pos:$(VERSION)

# TEST_DB_* dipakai suite integrasi di ./tests (butuh PostgreSQL non-superuser).
TEST_DB_HOST ?= localhost
TEST_DB_PORT ?= 5432
TEST_DB_USER ?= pos_app
TEST_DB_NAME ?= saas_pos_test
export TEST_DB_HOST TEST_DB_PORT TEST_DB_USER TEST_DB_NAME

.PHONY: help build run test test-unit lint vet fmt fmt-check tidy \
        migrate-up migrate-down migrate-status docker-build ci

help: ## Tampilkan bantuan ini
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | \
		awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

build: ## Kompilasi seluruh paket
	$(GO) build -trimpath -ldflags "-X main.version=$(VERSION)" ./...

run: ## Jalankan server (baca .env)
	$(GO) run .

test: ## Seluruh test + race detector (butuh PostgreSQL untuk ./tests)
	$(GO) test ./... -race -count=1

test-unit: ## Hanya test yang tidak butuh database
	$(GO) test ./config/... ./helpers/... ./internal/... -race -count=1

vet: ## go vet
	$(GO) vet ./...

fmt: ## Rapikan format (gofmt -w)
	gofmt -w .

fmt-check: ## Gagal bila ada berkas belum ter-format
	@test -z "$$(gofmt -l .)" || { echo "gofmt: berkas belum rapi:"; gofmt -l .; exit 1; }

lint: vet fmt-check ## vet + cek format

tidy: ## go mod tidy
	$(GO) mod tidy

migrate-up: ## Terapkan semua migrasi tertunda
	$(GO) run ./cmd/migrate up

migrate-down: ## Batalkan 1 migrasi terakhir (minta konfirmasi)
	$(GO) run ./cmd/migrate down 1

migrate-status: ## Versi migrasi terpasang + jumlah tertunda
	$(GO) run ./cmd/migrate status

docker-build: ## Bangun image runtime (multi-binari)
	docker build --build-arg VERSION=$(VERSION) -t $(IMAGE) .

ci: lint build test ## Yang dijalankan pipeline CI
