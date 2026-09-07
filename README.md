# Backend API (Go + Gin)

Sebuah framework backend REST API berbasis Golang dan Gin, dengan struktur modular dan siap dikembangkan sesuai kebutuhan, dalam framework ini sudah termasuk manajemen user, role, dan autentikasi JWT.

> **Sebelum menambah fitur, baca [CONVENTIONS.md](CONVENTIONS.md).**
> Berisi pola baku untuk repository, controller, query database, dan aturan keamanan
> yang wajib diikuti — beserta alasan di balik setiap aturan.

> **Rencana produk & roadmap: [docs/BLUEPRINT-SAAS-POS.md](docs/BLUEPRINT-SAAS-POS.md).**
> Kebutuhan UMKM, keputusan arsitektur (multi-tenancy, offline-first), model data,
> dan urutan fase pembangunan sampai siap dijual.

> **Model bisnis: [docs/BUSINESS-MODEL-CANVAS.md](docs/BUSINESS-MODEL-CANVAS.md).**
> Sembilan blok BMC, struktur harga, unit economics, risiko, dan asumsi yang
> harus dibuktikan sebelum menambah saluran.

> **Spesifikasi teknis: [docs/TECHNICAL-BACKEND.md](docs/TECHNICAL-BACKEND.md).**
> Skema 81 tabel beserta relasinya, konvensi ULID & zona waktu (UTC + WIB/WITA/WIT),
> kontrak API, protokol sinkronisasi offline, dan urutan implementasi per fase.

---

## Fitur Utama

- **Multi-tenancy** — isolasi data per usaha di tiga lapisan: `scopeTenant` (repo), Row Level Security PostgreSQL, dan visibilitas kepemilikan (menyusul)
- **Pendaftaran usaha 1 transaksi** — `POST /api/v1/auth/register` membuat tenant + outlet + peran bawaan + user pemilik sekaligus
- **Otorisasi granular** — 39 permission, peran per-tenant, middleware `Require(...)` per-endpoint
- **Master data** — kategori, satuan, produk (+ pencarian trigram < 200 ms), supplier
- **Impor produk CSV** — pratinjau (`dry_run`), impor sebagian, laporan baris gagal per baris
- **Kasir** — checkout 1 transaksi (harga dari server, hitung & bulat per baris), idempoten via `Idempotency-Key`, penomoran struk terkunci, void & retur, kasbon → piutang
- **Buku besar stok** — `stock_movements` sumber kebenaran, `stocks` cache; satu jalur `ApplyStockDeltas` (kunci urut `product_id`, saldo berjalan) dipakai penjualan/void/retur/opname/transfer/pembelian; rekonsiliasi cache dari buku besar
- **Shift & kas** — buka/tutup shift, rekonsiliasi `expected_cash`, gerakan kas non-penjualan
- **Pembelian** — stok masuk (idempoten), harga modal = harga beli terakhir
- **Opname & transfer** — hitung fisik (draft → post), transfer antar outlet (draft → kirim → terima)
- **Resep F&B** — `PUT /products/:id/recipe`; bahan baku otomatis terpotong saat menu terjual
- **Dashboard & laporan** — agregat `daily_sales_summaries` (dipelihara inkremental saat checkout, dihitung ulang saat void/retur); dashboard, laporan penjualan (`group_by` day/channel/cashier/payment), laba bersih per kanal, ekspor CSV; dashboard < 1 detik pada 100.000 transaksi
- **Sinkronisasi offline** — `POST /sync/push` (batch penjualan dari perangkat offline, idempoten per ULID + Idempotency-Key, satu operasi gagal tidak menjatuhkan batch, `business_date` dihitung ulang di server) + `GET /sync/pull` (master data + stok + batu nisan, kursor `sync_version` lewat pemicu database)
- **Langganan platform** — paket + tangga diskon prabayar (1/3/6/9/12 bulan), trial, tagihan & pembayaran idempoten, pendapatan diterima di muka diakui bulanan (`deferred_revenue_entries` + `cmd/recognize-revenue`), pembatalan di tengah masa dihitung ulang pada harga bulanan normal, prorata ganti paket
- **Autentikasi JWT** — access token pendek (15 mnt) + refresh token (30 hari) dengan rotasi & deteksi pemakaian ulang
- **Manajemen Outlet / User / Role** — CRUD tenant-scoped + Pagination ala Laravel
- **Migrasi skema berversi** (`cmd/migrate`) — `AutoMigrate` dimatikan
- **Soft delete** + partial unique index (baris terhapus tidak memblokir pendaftaran ulang)
- **Health check** — `/health` (liveness) & `/health/ready` (readiness)
- **Log terstruktur** (slog) dengan `request_id` per permintaan
- **ULID** sebagai primary key, dibuat terpusat di `internal/ulid`
- **Zona waktu** — server & DB selalu UTC; `business_date` per zona outlet (`internal/timez`)
- **Struktur folder modular** (routes → middleware → controller → service → repository → model)
- **Response JSON konsisten dan standar**

---

## Struktur Folder

```
/cmd/migrate         # Runner migrasi skema (up / down / status)
/cmd/recognize-revenue # Pekerjaan harian: akui pendapatan diterima di muka (§13.4)
/config              # Konfigurasi aplikasi (baca .env)
/controllers         # Handler endpoint (tipis)
/database            # Koneksi, connection pool, runner migrasi, seeder
/database/migrations # Berkas SQL migrasi berversi (NNNNNN_judul.up/.down.sql)
/helpers             # Fungsi bantu (hash, jwt, token, logger, errors, pagination)
/internal/reqctx     # Kunci context lintas-lapisan (tenant_id, user_id, permission)
/internal/timez      # Zona waktu, business_date, tipe Clock (kolom TIME)
/internal/ulid       # Pembuatan & validasi ULID
/middlewares         # auth, tenant scope, permission (Require), rate limit, observability
/models              # Model database (GORM)
/repositories        # Query ke database (satu tabel per repo) + scopeTenant / WithTenant
/routes              # Routing API
/services            # Logika bisnis lintas tabel + transaksi (mis. pendaftaran tenant)
/structs             # Struct request/response
/tests               # Uji integrasi (butuh PostgreSQL; skip otomatis bila tak ada)
main.go              # Bootstrap aplikasi
.env                 # Konfigurasi environment
```

---

## Instalasi & Menjalankan

1. **Clone repo ini**
2. **Copy `.env.example` ke `.env`** dan sesuaikan konfigurasi (DB, `JWT_SECRET`, dsb).
   `JWT_SECRET` wajib ≥ 32 karakter — `openssl rand -hex 32`.
3. **Install dependency**
   ```sh
   go mod tidy
   ```
4. **Jalankan migrasi database** — langkah terpisah, tidak jalan saat aplikasi start:
   ```sh
   go run ./cmd/migrate up        # terapkan semua migrasi tertunda
   go run ./cmd/migrate status    # lihat versi terpasang & jumlah tertunda
   go run ./cmd/migrate down 1    # batalkan 1 migrasi terakhir (minta konfirmasi)
   ```
5. **Jalankan aplikasi**
   ```sh
   go run main.go   # atau: air (hot reload)
   ```
   Cek: `curl localhost:8080/health` dan `curl localhost:8080/health/ready`.

> **Upgrade dari versi ber-`AutoMigrate`:** database lama yang tabel `users`/`roles`-nya
> dibuat `AutoMigrate` perlu disiapkan sekali. Untuk DB dev yang datanya boleh hilang:
> `DROP SCHEMA public CASCADE; CREATE SCHEMA public;` lalu `go run ./cmd/migrate up`.

### Menjalankan Test

```sh
go test ./...            # unit test selalu jalan
go test ./... -race
```

Uji integrasi di `/tests` butuh PostgreSQL. Jika tak tersedia, test itu **di-skip**
(bukan gagal). Untuk menjalankannya, siapkan database kosong lalu:

```sh
createdb saas_pos_test                       # dimiliki role non-superuser (RLS wajib berlaku)
TEST_DB_NAME=saas_pos_test TEST_DB_USER=<role> go test ./tests/ -v
```

> Peringatan: `TestMain` menjalankan `DROP SCHEMA public CASCADE` pada database target.
> Jangan arahkan ke database berisi data.

---

## Contoh Endpoint

Semua endpoint bisnis di bawah prefiks `/api/v1`.

| Method & Path | Izin | Keterangan |
|---|---|---|
| `GET  /health`, `/health/ready` | — | Liveness / readiness (cek DB) |
| `POST /api/v1/auth/register` | publik | Daftar USAHA: tenant + outlet + peran + pemilik (1 transaksi) |
| `POST /api/v1/auth/login` | publik | → access + refresh token |
| `POST /api/v1/auth/refresh` | publik | Tukar refresh token (rotasi + deteksi reuse) |
| `POST /api/v1/auth/logout` | token | Cabut refresh token |
| `GET  /api/v1/me` | token | Profil: user, tenant, permission, outlet |
| `GET/POST/PUT/DELETE /api/v1/outlets[/:id]` | `outlet.manage` | CRUD outlet |
| `GET/POST/PUT/DELETE /api/v1/categories[/:id]` | `product.view` / `product.edit` | CRUD kategori (maks 2 tingkat) |
| `GET/POST/PUT/DELETE /api/v1/units[/:id]` | `product.view` / `product.edit` | CRUD satuan |
| `GET/POST/PUT/DELETE /api/v1/suppliers[/:id]` | `product.view` / `product.edit` | CRUD supplier |
| `GET  /api/v1/products?q=&category_id=&is_active=` | `product.view` | Cari produk (index trigram) |
| `POST /api/v1/products` · `PUT /:id` | `product.edit` | Buat / ubah produk |
| `DELETE /api/v1/products/:id` | `product.delete` | Hapus produk |
| `POST /api/v1/products/import?dry_run=` | `product.import` | Impor CSV (pratinjau + laporan baris gagal) |
| `GET/POST/PUT/DELETE /api/v1/customers[/:id]` | `customer.view` / `customer.edit` | CRUD pelanggan |
| `POST /api/v1/sales` (header `Idempotency-Key`) | `sale.create` | Checkout |
| `GET  /api/v1/sales` · `GET /api/v1/sales/:id` | `sale.create` | Daftar / detail transaksi |
| `POST /api/v1/sales/:id/void` · `.../refund` | `sale.void` / `sale.refund` | Batal / retur penuh |
| `GET  /api/v1/sales-summary?from=&to=` | `report.view` | Ringkasan omzet/laba (langsung dari `sales`) |
| `GET  /api/v1/reports/dashboard?outlet_id=&date=` | `report.view` | Ringkasan hari + bulan berjalan + per kanal (dari agregat) |
| `GET  /api/v1/reports/sales?from=&to=&group_by=` | `report.view` | Laporan penjualan; `group_by` = day\|channel\|cashier\|payment |
| `GET  /api/v1/reports/profit?from=&to=` | `report.profit` | Laba bersih per kanal (§13.5) |
| `GET  /api/v1/reports/export?type=&format=csv` | `report.export` | Ekspor CSV (`type` = sales\|profit\|dashboard) |
| `POST /api/v1/reports/rebuild-summaries?from=&to=&outlet_id=` | `report.view` | Bangun ulang `daily_sales_summaries` dari `sales` |
| `POST /api/v1/sync/push` | `sale.create` | Batch penjualan offline; hasil per operasi (applied/duplicate/rejected) |
| `GET  /api/v1/sync/pull?since=&outlet_id=&limit=` | `sale.create` | Master data + stok + batu nisan sejak kursor `sync_version` |
| `GET  /api/v1/plans` | token | Katalog paket + harga per masa langganan |
| `GET/POST /api/v1/subscription` | `billing.manage` | Status langganan / mulai (trial) |
| `POST /api/v1/subscription/invoices` · `GET` | `billing.manage` | Terbitkan / daftar tagihan langganan |
| `POST /api/v1/subscription-payments` (header `Idempotency-Key`) | `billing.manage` | Bayar tagihan; lunas → aktif + `deferred_revenue_entries` |
| `POST /api/v1/subscription/cancel` | `billing.manage` | Batal + refund (harga bulanan normal) |
| `POST /api/v1/subscription/change-plan` | `billing.manage` | Ganti paket dengan kredit prorata |
| `POST /api/v1/shifts/open` · `/shifts/:id/close` | `shift.open` / `shift.close` | Buka / tutup shift |
| `POST/GET /api/v1/cash-movements` | `cash.movement` | Kas masuk/keluar |
| `GET  /api/v1/stocks?outlet_id=&low=` | `stock.view` | Saldo stok |
| `GET  /api/v1/stock-movements?product_id=` | `stock.view` | Kartu stok |
| `POST /api/v1/stock-adjustments` | `stock.adjust` | Saldo awal / koreksi stok |
| `POST /api/v1/stock-reconcile?outlet_id=` | `stock.opname` | Hitung ulang cache stok dari buku besar |
| `POST /api/v1/purchases` (header `Idempotency-Key`) | `stock.adjust` | Terima barang (stok masuk) |
| `POST /api/v1/stock-opnames` · `/:id/items` · `/:id/post` | `stock.opname` | Hitung fisik → posting selisih |
| `POST /api/v1/stock-transfers` · `/:id/send` · `/:id/receive` | `stock.transfer` | Transfer antar outlet |
| `GET/PUT /api/v1/products/:id/recipe` | `product.view` / `product.edit` | Resep menu F&B |
| `GET  /api/v1/receivables` · `POST /api/v1/receivable-payments` | `receivable.manage` | Piutang & pelunasan |
| `GET/POST/PUT/DELETE /api/v1/users[/:id]` | `user.manage` | CRUD user staf |
| `GET/POST/PUT/DELETE /api/v1/roles[/:id]` | `role.manage` | CRUD peran |
| `PUT  /api/v1/roles/:id/permissions` | `role.manage` | Ganti pemetaan permission peran |
| `GET  /api/v1/permissions` | `role.manage` | Katalog permission |

Semua rute `/api/v1` selain `/auth/*` butuh rantai `Auth → TenantScope → Require(...)`.

---

## Standar Response

```json
{
  "success": true,
  "message": "Berhasil mengambil data user",
  "data": {
    "current_page": 1,
    "per_page": 10,
    "total": 25,
    "last_page": 3,
    "from": 1,
    "to": 10,
    "data": [
      { "id": 1, "name": "John Doe", ... }
    ]
  }
}
```

---

## Kontribusi

- Pull request dan issue sangat terbuka!
- Ikuti standar penamaan dan struktur folder yang sudah ada.

---

## Lisensi

MIT License

---

## Database & Dependency

### Database

- **PostgreSQL**  
  Framework ini menggunakan PostgreSQL sebagai database utama.  
  Koneksi database diatur melalui file `.env`:
  ```
  DB_HOST=localhost
  DB_PORT=5432
  DB_USER=postgres
  DB_PASS=password
  DB_NAME=pos_db
  ```
- ORM yang digunakan: **GORM** (https://gorm.io/)  
  Anda dapat menyesuaikan koneksi di file konfigurasi sesuai kebutuhan.

### Dependency Utama

- [Gin](https://github.com/gin-gonic/gin) — HTTP web framework
- [GORM](https://github.com/go-gorm/gorm) — ORM untuk Golang
- [pgx/v5](https://github.com/jackc/pgx) — driver PostgreSQL (dipakai GORM & runner migrasi)
- [GoDotEnv](https://github.com/joho/godotenv) — Loader file `.env`
- [golang-jwt/jwt/v5](https://github.com/golang-jwt/jwt) — Library JWT untuk autentikasi
- [oklog/ulid/v2](https://github.com/oklog/ulid) — Pembuatan ULID
- [bcrypt](https://pkg.go.dev/golang.org/x/crypto/bcrypt) — Untuk hash password
- [validator](https://github.com/go-playground/validator) — Validasi struct request
- `log/slog` (stdlib) — log terstruktur; runner migrasi ditulis sendiri (tanpa library)

### Cara Install Dependency

Semua dependency akan otomatis terinstall saat menjalankan:
```sh
go mod tidy
```

---

**Catatan:**  
- Pastikan PostgreSQL sudah berjalan dan kredensial di `.env` sudah benar sebelum menjalankan aplikasi.
- Untuk development/testing, Anda bisa menggunakan database lain yang didukung GORM dengan sedikit penyesuaian

---

**Dikembangkan dengan ❤️ oleh Candra