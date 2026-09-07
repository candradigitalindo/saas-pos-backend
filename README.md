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
- **Otorisasi granular** — 38 permission, peran per-tenant, middleware `Require(...)` per-endpoint
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