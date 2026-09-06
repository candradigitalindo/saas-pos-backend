# Konvensi Pengembangan

Catatan ini mengunci pola yang dipakai di codebase. **Setiap fitur baru wajib mengikuti skema di sini** agar konsisten, aman, dan tidak mengulang bug yang sudah pernah diperbaiki.

Setiap aturan disertai alasannya. Kalau suatu saat aturan terasa menghalangi, baca dulu alasannya sebelum melanggar.

---

## 1. Alur & Tanggung Jawab Lapisan

```
routes/  →  middlewares/  →  controllers/  →  repositories/  →  models/
                                  ↕
                          structs/ + helpers/
```

| Lapisan | Boleh | Tidak boleh |
|---|---|---|
| `routes/` | Daftar endpoint, pasang middleware | Logika bisnis |
| `middlewares/` | Auth, otorisasi, rate limit | Query domain (selain ambil user untuk cek role) |
| `controllers/` | Bind & validasi request, susun response | Panggil `database.DB` langsung |
| `repositories/` | **Semua** akses `database.DB` | Menulis response HTTP / sentuh `gin.Context` |
| `models/` | Skema tabel + GORM hook | Logika bisnis |
| `structs/` | Bentuk request & response API | Dipakai sebagai model tabel |

**Aturan mati:** controller tidak pernah menyentuh `database.DB`. Kalau butuh query baru, buat fungsi di `repositories/`.

---

## 2. Pola Repository

### Signature wajib: `ctx` sebagai parameter pertama

```go
func FindUserByID(ctx context.Context, id string, user *models.User) error {
	return database.DB.WithContext(ctx).
		Joins("Role").
		Where("users.id = ?", id).
		First(user).Error
}
```

Controller meneruskan `c.Request.Context()`.

**Kenapa:** tanpa `WithContext`, query tetap jalan walau klien sudah memutus koneksi — koneksi database tertahan sia-sia dan session menumpuk. Ini penyebab paling umum "koneksi habis" saat trafik naik.

### Listing: satu fungsi mengembalikan `(data, total, error)`

Jangan bikin `GetAllX` dan `CountAllX` terpisah. Gabung seperti [`ListUsers`](repositories/user_repository.go):

```go
func ListUsers(ctx context.Context, search string, limit, offset int) ([]models.User, int64, error)
```

**Kenapa:** dulu terpisah, akibatnya kondisi filter pencarian diduplikasi di dua tempat (rawan tidak sinkron) dan controller butuh dua blok error handling.

### Aturan query yang wajib dipatuhi

| Aturan | Contoh | Alasan |
|---|---|---|
| **Selalu `ORDER BY`** pada query berpaginasi | `Order("users.id DESC")` | PostgreSQL **tidak menjamin** urutan baris tanpa `ORDER BY`. Tanpa ini data bisa terlewat atau muncul dua kali antar halaman. Pakai `id` (ULID) — terurut kronologis sekaligus memakai index primary key. |
| **`Joins`, bukan `Preload`**, untuk relasi belongs-to | `Joins("Role")` | `Preload` = 2 round-trip. `Joins` = 1 query LEFT JOIN. |
| **Kualifikasi nama kolom** saat ada JOIN | `users.name`, bukan `name` | `roles` juga punya kolom `name` → error ambiguous column. |
| **Escape input pencarian** | `escapeLike(search)` | `%` dan `_` adalah wildcard LIKE. Input `%` akan cocok dengan seluruh tabel dan memaksa full scan. |
| **Count tanpa JOIN** bila filter hanya menyentuh satu tabel | lihat `ListUsers` | JOIN untuk menghitung itu pemborosan. |
| **Lewati query halaman bila `total == 0`** | lihat `ListUsers` | Hemat satu round-trip. |
| **`Omit("Relasi")` saat `Save`** | `Omit("Role").Save(user)` | GORM ikut meng-upsert association yang ter-load → menulis ke tabel `roles` tanpa diminta. |

### Integritas data: serahkan ke constraint database

`DeleteRole` **tidak** melakukan COUNT lalu DELETE. Ia langsung DELETE dan menerjemahkan error FK:

```go
result := database.DB.WithContext(ctx).Delete(&models.Role{}, "id = ?", id)
if err := result.Error; err != nil {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23503" { // foreign_key_violation
		return ErrRoleInUse
	}
	return err
}
if result.RowsAffected == 0 {
	return gorm.ErrRecordNotFound
}
```

**Kenapa:** pola "cek dulu baru hapus" punya celah race — baris baru bisa menyelip di antara kedua query. Constraint database dievaluasi atomik, sekaligus menghemat satu round-trip.

**Kode SQLSTATE PostgreSQL yang dipakai:**
- `23505` unique_violation → `helpers.IsDuplicateEntryError(err)` → HTTP 409
- `23503` foreign_key_violation → data masih direferensikan → HTTP 409

### Error sentinel

Error domain dideklarasikan sebagai `var` di package `repositories` dan dicek dengan `errors.Is`, bukan dengan membandingkan string:

```go
var ErrRoleInUse = errors.New("...")
// di controller:
if errors.Is(err, repositories.ErrRoleInUse) { ... }
```

---

## 3. Pola Controller

Urutan baku setiap handler:

1. Ambil path param, tolak bila kosong → **400**
2. `ShouldBindJSON` ke struct request → gagal: **422** + `helpers.TranslateErrorMessage(err)`
3. Ambil entity yang ada (untuk update/delete) → tidak ketemu: **404**
4. Validasi relasi (mis. `role_id` ada di DB) → gagal: **422**
5. Panggil repository dengan `c.Request.Context()`
6. Susun response dari data yang **sudah ada di memori** — jangan query ulang hanya untuk menyusun response

Contoh langkah 6: `UpdateUser` menyimpan hasil `FindRoleByID` ke variabel dan memakainya kembali untuk `role_name`, bukan me-reload user.

### Response selalu memakai wrapper generic

```go
c.JSON(http.StatusOK, structs.SuccessResponse[structs.UserResponse]{
	Success: true,
	Message: "Berhasil mengambil data user",
	Data:    userResponse,
})
```

- Sukses: `structs.SuccessResponse[T]`
- Gagal: `structs.ErrorResponse` (`Errors` berupa `map[string]string` per field)
- Berpaginasi: `structs.SuccessResponse[structs.PaginatedResponse[T]]`
- Tanpa data: `structs.SuccessResponse[any]` dengan `Data: nil`

**Jangan** kembalikan `gin.H{}` mentah — klien mengandalkan bentuk yang konsisten.

### Model tidak pernah jadi response

Selalu petakan ke struct di `structs/`. Model `User` punya `Password`; membocorkannya lewat response adalah insiden keamanan, bukan sekadar kerapian.

Format waktu yang dipakai: `Format("2006-01-02 15:04:05")`.

---

## 4. Keamanan — Tidak Bisa Ditawar

Aturan berikut lahir dari celah yang **sudah pernah ada** di codebase ini dan sudah ditutup. Jangan dibuka lagi.

### Registrasi tidak boleh menerima `role_id` dari input

`POST /api/register` bersifat publik. Dulu ia menerima `role_id` dari body, sehingga siapa pun bisa mendaftar sebagai admin. Sekarang role **dipaksa** ke `user`:

```go
const defaultRoleName = "user"
repositories.FindRoleByName(c.Request.Context(), defaultRoleName, &roleCheck)
user := models.User{ ..., RoleID: roleCheck.ID }
```

**Aturan umum:** field yang menentukan hak akses **tidak boleh** berasal dari input endpoint publik. Perubahan role hanya lewat endpoint ber-`RoleMiddleware("admin")`.

### Secret tidak boleh punya nilai default

`InitJWT()` sengaja `log.Fatal` bila `JWT_SECRET` kosong atau < 32 karakter. Jangan tambahkan fallback "biar jalan dulu" — secret default yang ada di source publik sama saja dengan tidak ada autentikasi.

Pola ini berlaku untuk setiap secret baru: **fail fast, jangan fallback.**

### JWT hanya membawa user ID

Subject token = `user.ID` (ULID), bukan username. Username bisa berubah; ID tidak. Jangan menaruh role di dalam token — role dibaca dari database tiap request supaya pencabutan hak akses langsung berlaku.

### Jangan bocorkan detail error internal

Ke klien: pesan generik. Ke log: detail lengkap.

```go
Errors: map[string]string{"token": errMsg},   // benar
Errors: map[string]string{"token": err.Error()}, // salah — bocorkan internal
```

`helpers.TranslateErrorMessage` sudah menangani ini: error yang tidak dikenali jadi `"Terjadi kesalahan internal"`.

### Response autentikasi harus seragam

Username tidak ditemukan dan password salah harus menghasilkan pesan **dan waktu respon** yang sama. Karena itu `Login` tetap menjalankan bcrypt memakai `helpers.DummyPasswordHash` walau user tidak ada — kalau tidak, selisih waktu respon membocorkan username mana yang terdaftar.

### Endpoint publik wajib di-rate limit

```go
authLimiter := middlewares.RateLimit(0.2, 5) // 0.2 req/detik, burst 5
api.POST("/login", authLimiter, controllers.Login)
```

Rate limiter berbasis IP dan **in-memory** — kalau nanti aplikasi jalan multi-instance, ganti penyimpanannya ke Redis.

`TRUSTED_PROXIES` default kosong (tidak percaya proxy mana pun) supaya `X-Forwarded-For` tidak bisa dipalsukan untuk menembus limiter. Isi hanya bila memang di belakang reverse proxy.

### Validasi input

Aturan validasi ditulis di tag `binding` pada struct `structs/`, bukan sebagai `if` manual di controller:

```go
Username string `json:"username" binding:"required,min=3,max=50"`
Email    string `json:"email" binding:"required,email"`
Password string `json:"password" binding:"required,min=8"`
```

Untuk request update, pakai `omitempty` agar field yang tidak dikirim tidak dianggap kosong.

**Jaga konsistensi antara create dan update.** Bug nyata yang pernah terjadi: `email` divalidasi di update tapi tidak di create, sehingga email ngawur bisa masuk lewat register.

Tag `json` otomatis dipakai sebagai nama field di pesan error (diatur di `SetupRouter`), jadi pesan error cocok dengan payload.

### Jangan percaya header dari klien

`Host`, `X-Forwarded-Proto`, dan `X-Forwarded-For` semuanya dikirim klien dan bisa dipalsukan. Bila nilainya dipakai untuk menyusun URL, membatasi akses, atau mengidentifikasi pemanggil, ambil dari konfigurasi (`APP_URL`) atau dari proxy yang memang dipercaya (`TRUSTED_PROXIES`).

### Yang tidak boleh masuk git

`.env`, binary, `tmp/`, `.DS_Store` — semua sudah di `.gitignore`. `.env.example` hanya berisi placeholder, **tidak pernah** nilai asli. Kalau secret pernah ter-commit, menghapusnya dari tracking tidak cukup — **secret itu harus dirotasi.**

---

## 5. Database & Konfigurasi

### Connection pool

Nilai default di [`database.go`](database/database.go) sudah disetel dan sebaiknya tidak diturunkan tanpa alasan:

| Setting | Default | Alasan |
|---|---|---|
| `DB_MAX_OPEN_CONNS` | 25 | `max_connections` PostgreSQL defaultnya 100. Beberapa instance × 100 = kuota habis. |
| `DB_MAX_IDLE_CONNS` | = max open | Kalau idle jauh lebih kecil dari open, koneksi terus dibuka-tutup saat ramai. Membuka session PostgreSQL itu mahal. |
| `DB_CONN_MAX_IDLE_MINUTES` | 5 | Melepas koneksi menganggur agar session tidak menumpuk saat sepi. |
| `DB_STATEMENT_TIMEOUT_MS` | 30000 | Jaring pengaman: PostgreSQL sendiri membatalkan query yang nyangkut. |

`PrepareStmt` aktif secara default. **Set `DB_PREPARE_STMT=false` bila memakai PgBouncer** mode transaction/statement.

### Menambah konfigurasi baru

Selalu lewat helper di `config/` dengan default yang aman, lalu **catat di `.env.example`**. Jangan hardcode nilai lingkungan di source — CORS dulu di-hardcode ke `localhost` sehingga production harus mengubah kode.

| Helper | Untuk |
|---|---|
| `config.GetEnv(key, default)` | string |
| `config.GetIntEnv(key, default)` | integer |
| `config.GetBoolEnv(key, default)` | boolean |
| `config.GetStringSliceEnv(key, default)` | daftar dipisah koma |

Variabel yang **di-set tetapi kosong** (baris `ALLOWED_ORIGINS=` di `.env`) diperlakukan seperti tidak di-set, sehingga default tetap dipakai. Jangan pakai `os.Getenv` langsung atau `strings.Split` manual — `strings.Split("", ",")` menghasilkan `[]string{""}`, dan itu pernah membuat konfigurasi CORS berisi satu origin kosong.

**`APP_URL` wajib diisi di production.** Tautan paginasi dibangun darinya; bila kosong, base URL diambil dari header `Host` kiriman klien dan tautan di response bisa diarahkan ke domain penyerang.

### DSN

Nilai DSN dibangun lewat `buildDSN` dan di-quote oleh `quoteDSNValue`. Jangan kembali ke `fmt.Sprintf` polos: password kosong atau mengandung spasi akan **menelan parameter berikutnya** dan menghasilkan error koneksi yang membingungkan.

Pengecualian: `TimeZone` dan `statement_timeout` adalah runtime parameter — tidak di-quote.

### Migrasi

Saat ini memakai `AutoMigrate`. Tambahkan model baru ke daftar di `InitDatabase`.

`AutoMigrate` tidak bisa menghapus/mengganti nama kolom dan tidak reversible. **Sebelum rilis ke production, pindah ke migrasi berversi** (golang-migrate atau goose).

### Model

```go
type Product struct {
	ID        string    `json:"id" gorm:"primaryKey;type:char(26)"`
	Name      string    `json:"name" gorm:"not null"`
	CategoryID string   `json:"category_id" gorm:"index"`  // index setiap FK
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (p *Product) BeforeCreate(tx *gorm.DB) (err error) {
	p.ID = ulid.Make().String()
	return
}
```

- Primary key **ULID** `char(26)`, di-generate di hook `BeforeCreate`
- **Index setiap kolom foreign key** — tanpa index, JOIN dan pengecekan FK memindai seluruh tabel
- Field sensitif diberi tag `json:"-"`

---

## 6. Resep: Menambah Resource Baru

Misal menambah `Product`. Urutannya:

1. **`models/product.go`** — struct + hook `BeforeCreate` ULID + index pada FK
2. **`database.go`** — daftarkan di `AutoMigrate(&models.User{}, &models.Role{}, &models.Product{})`
3. **`structs/product.go`** — `ProductCreateRequest`, `ProductUpdateRequest` (pakai `omitempty`), `ProductResponse`
4. **`repositories/product_repository.go`** — semua fungsi menerima `ctx`; sediakan `ListProducts(ctx, search, limit, offset) ([]models.Product, int64, error)` beserta `productSearchCondition` yang memakai `escapeLike`
5. **`controllers/product_controller.go`** — ikuti urutan langkah di bagian 3
6. **`routes/api.go`** — daftarkan di grup yang sesuai dengan middleware yang benar

### Checklist sebelum dianggap selesai

- [ ] Semua fungsi repository menerima dan meneruskan `ctx`
- [ ] Query berpaginasi punya `ORDER BY` yang deterministik
- [ ] Input pencarian melewati `escapeLike`
- [ ] Relasi diambil dengan `Joins`, bukan `Preload`
- [ ] Kolom dikualifikasi (`products.name`) bila ada JOIN
- [ ] FK punya index di model
- [ ] Response memakai struct `structs/`, bukan model — tidak ada field sensitif yang bocor
- [ ] Aturan validasi konsisten antara create dan update
- [ ] Field penentu hak akses tidak berasal dari input publik
- [ ] Endpoint terpasang middleware yang benar di `routes/api.go`
- [ ] Konfigurasi baru tercatat di `.env.example`
- [ ] `go build ./... && go vet ./... && gofmt -l .` bersih

---

## 7. Utang Teknis yang Belum Dibayar

Jangan dianggap sudah beres:

| Item | Catatan |
|---|---|
| **Cakupan test masih tipis** | Baru ada regression test untuk `config` dan `helpers`. Kode auth, `repositories`, dan `middlewares` belum tersentuh — prioritaskan sebelum menambah tabel transaksi. |
| **Tidak ada logout / pencabutan token** | Token berlaku sampai kedaluwarsa. Butuh daftar cabut (jti + Redis) atau refresh token berumur pendek. |
| **Belum ada multi-tenancy** | Ini SaaS POS, tapi belum ada konsep tenant/outlet. Menambahkannya belakangan jauh lebih mahal — putuskan sebelum tabel transaksi dibuat. |
| **RBAC masih berbasis nama role** | `RoleMiddleware("admin")`. POS butuh permission granular (kasir/manajer/owner per fitur). |
| **Hard delete** | Belum ada soft delete. Catatan: menambah `gorm.DeletedAt` bentrok dengan unique constraint `username`/`email` — baris terhapus tetap memblokir pendaftaran ulang. Perlu partial unique index bila jadi diterapkan. |
| **`AutoMigrate` di production** | Ganti dengan migrasi berversi sebelum rilis. |
| **Rate limiter in-memory** | Tidak berlaku lintas instance. |
| **Belum ada health check** | Butuh `/health` untuk load balancer / Kubernetes. |
| **Belum ada transaksi lintas tabel** | Checkout POS (kurangi stok + buat order + catat pembayaran) wajib dalam satu `DB.Transaction`. Rancang di lapisan repository/service. |

---

## 8. Perintah Verifikasi

Jalankan sebelum commit:

```sh
go build ./...
go vet ./...
gofmt -l .        # keluaran kosong berarti rapi
```

Menjalankan aplikasi saat development:

```sh
air              # hot reload (konfigurasi di .air.toml)
go run main.go   # tanpa hot reload
```
