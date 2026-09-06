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

- **Autentikasi JWT** — access token pendek (15 mnt) + refresh token (30 hari) dengan rotasi & deteksi pemakaian ulang
- **Manajemen User** (CRUD + Pagination ala Laravel)
- **Manajemen Role** (CRUD + Pagination)
- **Middleware Auth + Role**
- **Migrasi skema berversi** (`cmd/migrate`) — `AutoMigrate` dimatikan
- **Soft delete** + partial unique index (baris terhapus tidak memblokir pendaftaran ulang)
- **Health check** — `/health` (liveness) & `/health/ready` (readiness)
- **Log terstruktur** (slog) dengan `request_id` per permintaan
- **ULID** sebagai primary key, dibuat terpusat di `internal/ulid`
- **Zona waktu** — server & DB selalu UTC; `business_date` dihitung per zona outlet (`internal/timez`)
- **Struktur folder modular (MVC + Repository)**
- **Konfigurasi via .env**
- **Response JSON konsisten dan standar**
- **Pagination response mirip Laravel**

---

## Struktur Folder

```
/cmd/migrate         # Runner migrasi skema (up / down / status)
/config              # Konfigurasi aplikasi (baca .env)
/controllers         # Handler endpoint
/database            # Koneksi, connection pool, runner migrasi, seeder
/database/migrations # Berkas SQL migrasi berversi (NNNNNN_judul.up/.down.sql)
/helpers             # Fungsi bantu (hash, jwt, token, logger, pagination, dsb)
/internal/ulid       # Pembuatan & validasi ULID
/internal/timez      # Konversi zona waktu & perhitungan business_date
/middlewares         # Middleware (auth, role, rate limit, observability)
/models              # Model database (GORM)
/repositories        # Query ke database
/routes              # Routing API
/structs             # Struct untuk request/response
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
> `DROP TABLE users, roles CASCADE;` lalu `go run ./cmd/migrate up`. Untuk DB berisi data,
> samakan skema manual ke migrasi `000001`/`000002` lalu catat versinya di tabel
> `schema_migrations`.

---

## Contoh Endpoint

Semua endpoint bisnis di bawah prefiks `/api/v1`.

| Method & Path | Keterangan |
|---|---|
| `GET  /health` | Liveness — proses hidup |
| `GET  /health/ready` | Readiness — termasuk cek database |
| `POST /api/v1/auth/register` | Registrasi user (role dipaksa `user`) |
| `POST /api/v1/auth/login` | → access token + refresh token |
| `POST /api/v1/auth/refresh` | Tukar refresh token (rotasi) |
| `POST /api/v1/auth/logout` | Cabut refresh token (butuh access token) |
| `GET  /api/v1/users?page=1&limit=10` | Daftar user (admin) |
| `GET  /api/v1/users/:id` | Detail user (admin) |
| `POST /api/v1/roles` | Buat role (admin) |
| `GET  /api/v1/roles?page=1&limit=10` | Daftar role (admin) |

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