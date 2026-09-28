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
> Skema 104 tabel beserta relasinya, konvensi ULID & zona waktu (UTC + WIB/WITA/WIT),
> kontrak API, protokol sinkronisasi offline, dan urutan implementasi per fase.

---

## Fitur Utama

- **Multi-tenancy** — isolasi data per usaha di tiga lapisan: `scopeTenant` (repo), Row Level Security PostgreSQL, dan visibilitas kepemilikan (menyusul)
- **Pendaftaran usaha 1 transaksi** — `POST /api/v1/auth/register` membuat tenant + outlet + peran bawaan + user pemilik sekaligus
- **Otorisasi granular** — 50 permission, peran per-tenant, middleware `Require(...)` per-endpoint
- **Master data** — kategori, satuan, produk (+ pencarian trigram < 200 ms), supplier
- **Varian barang** — pilihan dengan selisih harga (ukuran, es/panas, level pedas); stok tetap di tingkat barang; struk mencatat "Barang (Varian)"; SKU/barcode varian unik lintas barang & varian (dipindai kasir, dikenali pesanan kanal, dikirim sebagai pilihan wajib di menu GoFood/GrabFood)
- **Impor produk CSV** — pratinjau (`dry_run`), impor sebagian, laporan baris gagal per baris
- **Kasir** — checkout 1 transaksi (harga dari server, hitung & bulat per baris), idempoten via `Idempotency-Key`, penomoran struk terkunci, void & retur, kasbon → piutang; bayar gabungan (beberapa baris `payments`; kasbon harus pas, kembalian hanya dari uang tunai); tagihan terbuka / open bill (disimpan di server untuk semua perangkat, versi mencegah saling timpa, ditutup di transaksi checkout yang sama sehingga tidak bisa dibayar dua kali; migrasi 000044)
- **Buku besar stok** — `stock_movements` sumber kebenaran, `stocks` cache; satu jalur `ApplyStockDeltas` (kunci urut `product_id`, saldo berjalan) dipakai penjualan/void/retur/opname/transfer/pembelian; rekonsiliasi cache dari buku besar
- **Shift & kas** — buka/tutup shift, rekonsiliasi `expected_cash`, gerakan kas non-penjualan
- **Pembelian** — stok masuk (idempoten), harga modal = harga beli terakhir
- **Opname & transfer** — hitung fisik (draft → post), transfer antar outlet (draft → kirim → terima)
- **Resep F&B** — `PUT /products/:id/recipe`; bahan baku otomatis terpotong saat menu terjual
- **Dashboard & laporan** — agregat `daily_sales_summaries` (dipelihara inkremental saat checkout, dihitung ulang saat void/retur); dashboard, laporan penjualan (`group_by` day/hour/channel/cashier/payment/product), laba bersih per kanal, ekspor CSV; dashboard < 1 detik pada 100.000 transaksi
- **Sinkronisasi offline** — `POST /sync/push` (batch penjualan dari perangkat offline, idempoten per ULID + Idempotency-Key, satu operasi gagal tidak menjatuhkan batch, `business_date` dihitung ulang di server) + `GET /sync/pull` (master data + stok + batu nisan, kursor `sync_version` lewat pemicu database)
- **Langganan platform** — paket + tangga diskon prabayar (1/3/6/9/12 bulan), trial, tagihan, **konfirmasi pembayaran yang diverifikasi staf keuangan platform** (tenant tidak bisa mengaktifkan paketnya sendiri), pendapatan diterima di muka diakui bulanan (`deferred_revenue_entries` + `cmd/recognize-revenue`), pembatalan di tengah masa dihitung ulang pada harga bulanan normal dan **pengembaliannya diantre ke staf keuangan** (rekening diisi pemilik, ditandai dengan nomor referensi), ganti paket yang berlaku setelah dibayar, **perpanjangan & pengingat otomatis** (`cmd/subscription-renewals`: tagihan H-7, jatuh tempo, masa coba/tenggang hampir habis — masing-masing sekali)
- **CRM tenant** — pipeline & tahap yang bisa diatur, deal + alasan menang/kalah, aktivitas follow-up; penawaran → proyek otomatis → invoice bertermin → pembayaran parsial; **pelunasan invoice tercatat sebagai satu penjualan di tabel `sales` yang sama dengan POS** (omzet & laba satu pintu). Visibilitas kepemilikan (lapis 3, `scopeVisibility`): sales hanya melihat datanya sendiri
- **CRM sales lapangan** — rencana kunjungan (call plan), check-in/out dengan GPS + foto (direkam **hanya** saat check-in/out), kunjungan tanpa pesanan + alasan; kunjungan offline disinkron idempoten lewat `/sync/push` (`op: visit.upsert`); target sales + pencapaian; **komisi berbasis nilai TERTAGIH** (pembayaran non-kredit + setoran piutang lapangan), bukan terkirim
- **Kanal pesanan online (fondasi, tanpa API)** — definisi kanal per outlet + tarif komisi, pemetaan SKU kanal ↔ produk, entri pesanan manual (WhatsApp/Instagram) + impor CSV laporan harian kanal; setiap pesanan = **satu `sales` bertanda `channel_id`** (potong stok + resep, komisi masuk `sale_payments.fee_amount`) → **laba bersih per kanal setelah komisi** lewat `/reports/profit`; idempoten per `external_order_id`; batal pesanan mengembalikan stok
- **Pipeline peristiwa kanal (provider-agnostik)** — webhook `POST /webhooks/channels/:provider` **tanpa auth** (kanal ditautkan lewat `(provider, merchant_ref)` unik global) menyimpan payload **mentah** ke `channel_events` lalu balas 200 cepat; pekerja `cmd/process-channel-events` memprosesnya **satu per transaksi** (`FOR UPDATE SKIP LOCKED`) lewat adaptor seragam (`genericAdapter` menerima payload ternormalisasi; adaptor per-provider menyusul seiring kemitraan API) → `order.created` jadi penjualan + rincian `channel_fees`; stok ke marketplace lewat antrean `channel_stock_syncs` yang diisi pekerja dari `stock_movements` (barang terpetakan saja); **pengiriman ganda oleh kanal → tetap satu penjualan** (idempoten `external_order_id`); percobaan ulang bertahap + antrean mati (`status='dead'`) yang bisa dilihat pemilik; **rekonsiliasi pencairan** `channel_settlements` (nilai periode dari penjualan + fee vs. uang yang benar-benar masuk → `matched`/`mismatch`); kegagalan kanal **tidak** menghentikan kasir
- **Program Mitra Penjual (agen & afiliasi)** — modul **platform** untuk merekrut tenant, dengan **realm autentikasi terpisah** (`/api/v1/partner/*`, token realm `partner` ditolak di semua rute tenant dan sebaliknya). Kode referral valid saat pendaftaran → kaitan `partner_referrals` sekali seumur hidup (dasar komisi). **Mesin komisi deterministik** (`cmd/partner-commissions`): berulang selama merchant berlangganan, dihitung dari `subscription_invoices.paid_amount` (uang yang benar-benar diterima), **ambang aktivasi** (≥N transaksi / N hari) sebelum komisi pertama, **clawback** bila merchant berhenti dalam masa tertentu, **potong pajak** dipisah di pencairan (bruto − clawback − pajak = neto); hitung ulang idempoten (hanya baris `held`). Portal mitra: dashboard, daftar prospek, **status merchant binaan SAJA** (status langganan + jatuh tempo + aktif — tidak pernah omzet/produk/pelanggan/transaksi, blueprint G.8), rincian komisi & pencairan. Setiap akses mitra ke data merchant dicatat di `audit_logs` (§5.14, `actor_type='partner_user'`) — satu insert batch per pembukaan halaman, dan kegagalan mencatatnya menggagalkan permintaan
- **Panel internal penyedia SaaS** — realm autentikasi **ketiga** (`/api/v1/platform/*`), terpisah penuh dari tenant maupun mitra. Peran TETAP (superadmin/operator/finance/support) dengan wewenang dipisah supaya yang memverifikasi mitra bukan yang mencairkan uangnya; setiap tindakan tulis tercatat di `audit_logs` (`actor_type='admin'`). Superadmin aktif terakhir tidak bisa dinonaktifkan. Verifikasi mitra, verifikasi pembayaran langganan, pengembalian dana langganan, tingkat komisi, jalankan komisi, pencairan, keputusan sengketa, dan antrean notifikasi kini lewat panel — bukan lagi hanya CLI
- **Notifikasi keluar (pola outbox)** — peristiwa ditulis ke `outbox_events` **di dalam transaksi bisnis yang sama** dengan perubahan datanya, lalu dikirim `cmd/process-outbox`. Itu yang membuat "tagihan tersimpan tapi notifikasinya hilang" — atau sebaliknya — mustahil. Gagal → penundaan bertahap; 10 kali gagal → antrean mati yang terlihat di panel dan bisa dikembalikan ke antrean. Template `{{penanda}}` per tenant menimpa bawaan sistem. Pengirim WhatsApp/email sungguhan tinggal ditukar lewat antarmuka `Notifier`
- **Absensi & penggajian** — karyawan + jadwal kerja mingguan, `attendances` buku besar (koreksi tak mengubah baris asli) + `attendance_days` cache yang dibangun ulang (urutan status TETAP: libur > cuti > absen > alpa), hari libur, cuti/izin + saldo; **mesin gaji deterministik** (urutan tetap: upah dasar → aturan earning → deduction → penyesuaian periode lalu → cicilan kasbon; snapshot nama/tipe/params tiap baris slip) → hitung → kunci → bayar (kas keluar `ref_table='payroll_periods'`); hitung ulang dari data sama = angka identik; koreksi setelah kunci → `payroll_adjustments` di periode berikutnya; kasbon dipotong bertahap
- **Autentikasi JWT** — access token pendek (15 mnt) + refresh token (30 hari) dengan rotasi & deteksi pemakaian ulang
- **Peran dinamis per tenant** — pendaftaran menyiapkan 4 peran bawaan (Pemilik/Manajer/Kasir/Gudang), lalu tenant bebas menambah, mengubah izin, dan menghapus peran lewat `role.manage`. **Satu user boleh memegang beberapa peran** (`user_roles`); izin efektifnya = gabungan izin seluruh perannya. Dua batas yang dikunci: peran bawaan pemilik wajib mempertahankan `role.manage` (kalau tidak, tenant terkunci permanen), dan peran yang masih dipegang seseorang tidak bisa dihapus
- **Manajemen Outlet / User** — CRUD tenant-scoped + Pagination ala Laravel
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
/cmd/subscription-renewals # Pekerjaan harian: tagihan perpanjangan, jatuh tempo, pengingat masa coba/tenggang
/cmd/process-channel-events # Pekerja: tarik pesanan berkala (cadangan push) + proses antrean channel_events + kirim stok ke marketplace (§11)
/cmd/partner-commissions # Pekerjaan bulanan: hitung komisi mitra → setujui → cairkan (Fase 12)
/cmd/partner-admin   # CLI mitra (pelengkap panel internal)
/cmd/platform-admin  # Bootstrap akun staf internal (admin pertama)
/cmd/seed-demo       # Isi data contoh untuk menjelajah & peragaan (bukan production)
/cmd/process-outbox  # Pekerja: kirim notifikasi yang mengantre (§5.14)
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
/deploy              # postgres-init.sql (role non-superuser untuk RLS)
/wa-gateway          # Sidecar Node: jembatan ke WhatsApp lewat Baileys (opsional)
/.github/workflows   # CI: build + vet + gofmt + test -race + docker build
main.go              # Bootstrap aplikasi
Dockerfile           # Build multi-tahap → distroless (server + semua cmd/)
docker-compose.yml   # Dev lokal: PostgreSQL + migrate + server
Makefile             # make help | build | test | lint | migrate-up | docker-build
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

   Untuk menjelajahi aplikasi, isi data contoh — barang, stok, transaksi hari
   ini, kasbon, karyawan (server harus sudah jalan):
   ```sh
   go run ./cmd/seed-demo
   ```
   Aman dijalankan berulang: master data yang sudah ada dipakai lagi, hanya
   transaksi hari ini yang ditambah. Masuk sebagai `sari` / `rahasia123`.
6. **Pekerjaan terjadwal** (cron/systemd timer — bukan bagian dari start aplikasi, semuanya idempoten):
   ```sh
   go run ./cmd/recognize-revenue                     # harian, awal bulan — akui pendapatan diterima di muka (§13.4)
   go run ./cmd/subscription-renewals                 # harian — tagihan perpanjangan, jatuh tempo, pengingat WhatsApp (sekali per kejadian)
   go run ./cmd/process-channel-events                # sekali jalan (§11: panggil tiap ~10 dtk) — tarik pesanan (Lazada, tiap 15 mnt per toko) + proses channel_events + sinkron stok
   go run ./cmd/process-channel-events -loop -interval 10s   # atau: daemon menetap (systemd), berhenti rapi di SIGTERM
   go run ./cmd/partner-commissions -approve -payout  # bulanan (tanggal 1) — komisi mitra bulan lalu → setujui → pencairan draft
   go run ./cmd/process-outbox                        # sering (tiap ~30 dtk) — kirim notifikasi yang mengantre
   go run ./cmd/process-outbox -loop -interval 30s    # atau: daemon menetap
   ```

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

Atau lewat `make`: `make test` (butuh PostgreSQL) · `make test-unit` (tanpa DB) ·
`make lint` (vet + cek format). CI (`.github/workflows/ci.yml`) berjalan pada
setiap pull request dan push ke `main`, dengan tiga job:

- `test` — gofmt, vet, build, dan seluruh tes Go `-race` pada PostgreSQL 16 +
  role non-superuser;
- `web` — `npm ci`, periksa tipe, vitest, dan build produksi PWA di `web/`.
  Job ini juga menjaga sisi web kontrak total kasir
  (`web/src/bersama/util/kasus-total.json`); sisi server-nya di job `test`;
- `docker` — membangun image runtime.

---

## Deployment

Aplikasi **tidak** menjalankan migrasi saat start (aturan #10) dan **tidak**
punya fallback konfigurasi — ia sengaja gagal-cepat bila setelan tidak aman.

### 1. Prasyarat wajib di production

| Hal | Kenapa |
|---|---|
| **Role database NON-SUPERUSER** | RLS (termasuk `FORCE ROW LEVEL SECURITY`) **tidak berlaku untuk superuser** — konek sebagai `postgres` mematikan seluruh isolasi antar-tenant tanpa error. Jalankan [deploy/postgres-init.sql](deploy/postgres-init.sql) sekali (atau setara di PostgreSQL terkelola). |
| `JWT_SECRET` ≥ 32 karakter | `openssl rand -hex 32`. App fatal bila lebih pendek. |
| `APP_URL` diisi | Tanpa ini, tautan paginasi mengambil `Host` dari klien (bisa dipalsukan). |
| `APP_ENV=production` | Gin release mode + log level lebih tenang. |
| `ALLOWED_ORIGINS` & `TRUSTED_PROXIES` | CORS ketat + `X-Forwarded-For` hanya dipercaya dari proxy yang benar. |
| `DB_SSLMODE=require` (atau lebih ketat) | |
| Ganti `DB_PASS` dari nilai contoh | Nilai di `.env` repo lama pernah bocor — wajib dirotasi di PostgreSQL. |

### 2. Urutan rilis (zero-downtime)

```sh
docker build --build-arg VERSION=$(git describe --tags --always) -t saas-pos:$TAG .

# Migrasi = langkah TERPISAH, dijalankan SEKALI sebelum menaikkan instance baru.
docker run --rm --env-file .env --entrypoint /usr/local/bin/migrate saas-pos:$TAG up

# Baru rilis server. main.go sudah punya graceful shutdown (SIGTERM → drain → tutup pool).
docker run -d --env-file .env -p 8080:8080 saas-pos:$TAG
```

Migrasi dirancang aditif (kolom baru nullable / tabel baru), jadi versi lama dan
baru bisa berjalan berdampingan selama rollout. `GET /health/ready` mengecek
koneksi DB; `GET /health` untuk liveness.

### 3. Pekerjaan terjadwal (cron host / scheduler)

| Jadwal | Perintah |
|---|---|
| harian, awal bulan | `/usr/local/bin/recognize-revenue` |
| harian (mis. 07.00 WIB) | `/usr/local/bin/subscription-renewals` |
| tiap ~30 detik (atau daemon `-loop`) | `/usr/local/bin/process-outbox` |
| tiap ~10 detik (atau daemon `-loop`) | `/usr/local/bin/process-channel-events` |
| bulanan, tanggal 1 | `/usr/local/bin/partner-commissions -approve -payout` |

`docker compose up --build` menyiapkan semuanya untuk **pengembangan lokal**
(PostgreSQL + role non-superuser + migrate + server); `--profile jobs` menambah
kontainer pekerjaan terjadwal.

### 3b. Notifikasi WhatsApp (opsional)

`process-outbox` mengirim notifikasi yang mengantre. Tanpa konfigurasi tambahan
ia hanya **mencatat ke log** — alur outbox tetap utuh dan bisa diuji tanpa
menautkan nomor WhatsApp sungguhan.

Untuk benar-benar mengirim, jalankan sidecar di [`wa-gateway/`](wa-gateway/)
(Node + [Baileys](https://baileys.wiki/)) lalu isi dua variabel:

```bash
WA_GATEWAY_URL=http://127.0.0.1:8090
WA_GATEWAY_TOKEN=<rahasia bersama, openssl rand -hex 32>
```

Backend tidak bisa memanggil Baileys langsung: itu pustaka Node yang bicara
WebSocket ke WhatsApp Web, bukan HTTP API. Sidecar-lah yang memegang sambungan.

**Baileys tidak resmi.** Ia meniru WhatsApp Web, bisa rusak saat WhatsApp
berubah, dan nomor yang berkelakuan seperti robot berisiko diblokir. Bila
notifikasi tagihan tidak boleh gagal, WhatsApp Business API resmi lebih tenang —
menukarnya cukup satu implementasi `Notifier` baru, inti aplikasi tidak berubah.
Rinciannya di [`wa-gateway/README.md`](wa-gateway/README.md).

### 4. Rate limiter — memori vs Redis

`middlewares/rate_limit_middleware.go` punya dua backend, dipilih sekali saat
start:

| `RATELIMIT_REDIS_URL` | Backend | Catatan |
|---|---|---|
| kosong (default) | memori proses | Cukup untuk 1 instance. Dengan N replika batas efektif jadi N×; restart mereset bucket. |
| `redis://host:6379/0` | Redis (token-bucket Lua atomik) | Penegakan lintas-instance. Tak terjangkau **saat start** → app fatal. Blip **saat runtime** → fail-open (izinkan + warning), agar satu gangguan Redis tidak mengunci semua orang. |

Deploy multi-replika (mis. di belakang load balancer) sebaiknya memakai Redis
untuk endpoint sensitif brute-force (`/auth/*`, `/partner/auth/login`).

---

## Contoh Endpoint

Semua endpoint bisnis di bawah prefiks `/api/v1`.

| Method & Path | Izin | Keterangan |
|---|---|---|
| `GET  /health`, `/health/ready` | — | Liveness / readiness (cek DB) |
| `POST /api/v1/auth/register` | publik | Daftar USAHA: tenant + outlet + peran + pemilik (1 transaksi) |
| `POST /api/v1/auth/login` | publik | Kolom `username` diisi nama pengguna **atau email** → access + refresh token |
| `POST /api/v1/auth/refresh` | publik | Tukar refresh token (rotasi + deteksi reuse) |
| `POST /api/v1/auth/logout` | token | Cabut refresh token |
| `GET  /api/v1/me` | token | Profil: user, tenant, permission, `outlet_ids` + `outlets` (rincian cabang: pajak, biaya layanan — dipakai total kasir) |
| `GET/POST/PUT/DELETE /api/v1/outlets[/:id]` | `outlet.manage` | CRUD outlet. `tax_rate` & `service_charge_rate` = pecahan desimal string, 0 ≤ tarif < 1, maks. 4 desimal (`"0.11"` = 11%). `GET /outlets` juga untuk `stock.transfer` (memilih cabang tujuan) |
| `GET/POST/PUT/DELETE /api/v1/categories[/:id]` | `product.view` / `product.edit` | CRUD kategori (maks 2 tingkat) |
| `GET/POST/PUT/DELETE /api/v1/units[/:id]` | `product.view` / `product.edit` | CRUD satuan |
| `GET/POST/PUT/DELETE /api/v1/suppliers[/:id]` | `product.view` / `product.edit` | CRUD supplier |
| `GET  /api/v1/products?q=&category_id=&is_active=` | `product.view` | Cari produk (index trigram) |
| `POST /api/v1/products` · `PUT /:id` | `product.edit` | Buat / ubah produk (termasuk `description` ≤500 karakter untuk menu aplikasi antar — GoFood menampilkan 250 pertama; migrasi 000043). `wholesale_prices: [{min_qty, price}]` = harga grosir per jumlah (≤5 tingkat, min_qty > 1, unik; PUT: tidak dikirim = tidak diubah, `[]` = dihapus) — disimpan sebagai `product_prices` pada daftar harga default ("Harga umum", dibuat otomatis). `GET /products/:id` & balasan buat/ubah membawa `wholesale_prices`. Checkout memakai tingkat ber-min_qty terbesar yang terpenuhi oleh TOTAL jumlah barang itu di transaksi (varian dijumlah), varian tetap menambah selisihnya |
| `DELETE /api/v1/products/:id` | `product.delete` | Hapus produk |
| `GET /api/v1/products/:id/variants` · `POST` · `PUT /:vid` · `DELETE /:vid` | `product.view` / `product.edit` | Varian barang: `{name, price_delta, sku, barcode, is_active}` (PUT mengganti utuh); nama unik per barang (409), harga jual + selisih tidak boleh minus (422), SKU/barcode tidak boleh dipakai barang atau varian lain (409 — arah sebaliknya juga dijaga di buat/ubah/impor barang); hapus = lunak, penjualan lama utuh. Checkout `items[].variant_id` → harga + selisih, nama "Barang (Varian)", stok berkurang di tingkat barang |
| `POST /api/v1/products/import?dry_run=` | `product.import` | Impor CSV (pratinjau + laporan baris gagal) |
| `GET/POST/PUT/DELETE /api/v1/customers[/:id]` | `customer.view` / `customer.edit` | CRUD pelanggan; daftar membawa `stats` (kedatangan, belanja bersih, terakhir datang; `receivable_outstanding` hanya untuk `receivable.manage`) |
| `POST /api/v1/sales` (header `Idempotency-Key`) | `sale.create` | Checkout. `open_bill_id` (opsional) menutup tagihan terbuka di transaksi yang sama — tagihan yang sudah dibayar → 409; yang tidak dikenal diabaikan (penjualan tetap dicatat) |
| `POST /api/v1/sales/:id/receipt-link` · `GET /api/v1/public/receipts/:token` | `sale.create` · **publik** | Struk digital: token acak 128 bit (dibuat sekali saat struk pertama dibagikan, dipakai ulang; migrasi 000045). Halaman publik `/struk/:token` (tanpa akun, rate limit per IP `STRUK_PUBLIK_RATELIMIT_RPS/_BURST`) hanya berisi isi struk — tanpa modal, pelanggan, kasir, atau id internal; struk yang dibatalkan tetap terbuka dengan statusnya. Struk dikirim lewat WhatsApp TOKO di perangkat kasir (`wa.me`), bukan nomor platform |
| `GET /api/v1/open-bills?outlet_id=` · `PUT /api/v1/open-bills/:id` · `POST .../:id/cancel` | `sale.create` | Tagihan terbuka (open bill): id ULID dari klien; `PUT` mengganti utuh `{outlet_id, label, items[{product_id, variant_id, qty, discount_amount \| discount_percent, note}], order_discount{kind, value}, base_version}` — `base_version` 0 = baru (201), selain itu harus sama dengan versi server (basi → 409). Batal juga dengan `base_version`. Offline: `/sync/push` op `open_bill.upsert` / `open_bill.cancel` (id operasi ≠ id tagihan; kirim ulang = `duplicate`) |
| `GET  /api/v1/sales` · `GET /api/v1/sales/:id` | `sale.create` | Daftar / detail transaksi. Daftar bisa disaring `search` (nomor nota) & `method` (cara bayar); tiap baris membawa `items`, `payments`, `customer_name`, `cashier_name`, serta `return_of_receipt_no` / `returned_by_receipt_no` (retur ↔ nota asal) |
| `GET  /api/v1/sales/day-summary?outlet_id=&business_date=` | `sale.create` | Ringkasan satu hari untuk layar riwayat: penjualan, rata-rata, retur, batal, bersih, per cara bayar (tunai dikurangi kembalian) |
| `POST /api/v1/sales/:id/void` · `.../refund` | `sale.void` / `sale.refund` | Batal / retur penuh |
| `GET  /api/v1/sales-summary?from=&to=` | `report.view` | Ringkasan omzet/laba (langsung dari `sales`) |
| `GET  /api/v1/reports/dashboard?outlet_id=&date=` | `report.view` | Ringkasan hari + bulan berjalan + per kanal (dari agregat) |
| `GET  /api/v1/reports/sales?from=&to=&group_by=` | `report.view` | Laporan penjualan; `group_by` = day\|hour\|channel\|cashier\|payment\|product (per barang: baris membawa `label`, `unit`, `qty`; bersih dari void & retur) |
| `GET  /api/v1/reports/profit?from=&to=` | `report.profit` | Laba bersih per kanal (§13.5) |
| `GET  /api/v1/reports/export?type=&format=csv` | `report.export` | Ekspor CSV (`type` = sales\|profit\|dashboard) |
| `POST /api/v1/reports/rebuild-summaries?from=&to=&outlet_id=` | `report.view` + `outlet.manage` | Bangun ulang `daily_sales_summaries` dari `sales` |
| `POST /api/v1/sync/push` | `sale.create` atau `crm.visit.checkin` | Batch operasi offline; hasil per operasi (applied/duplicate/rejected). Izin diperiksa lagi **per operasi**: `sale.create` untuk `op: sale.create`, `crm.visit.checkin` untuk `op: visit.upsert` |
| `GET  /api/v1/sync/pull?since=&outlet_id=&limit=` | `sale.create` atau `crm.visit.checkin` | Master data + stok + batu nisan sejak kursor `sync_version` |
| `GET  /api/v1/plans` | token | Katalog paket + harga per masa langganan |
| `GET/POST /api/v1/subscription` | `billing.manage` | Status langganan / mulai (trial). Selama masa coba (atau setelah habis) POST mengganti paket TANPA menggeser tanggal masa coba, dan membatalkan tagihan paket lama yang belum dibayar (409 bila sedang dikonfirmasi). **Masa coba sekali per tenant**: mulai lagi setelah berhenti tidak memberi masa coba baru (yang berhenti di tengah masa coba melanjutkan sisanya). Paket harga 0 → 422: pindah ke Gratis = `POST /subscription/cancel` |
| `POST /api/v1/subscription/invoices` · `GET` | `billing.manage` | Terbitkan / daftar tagihan langganan. Periode: perpanjangan mulai di akhir periode berjalan; **di masa coba mulai saat masa coba berakhir** (dihitung ulang saat lunas — dibayar setelah masa coba habis → mulai hari bayar), jadi membayar lebih awal tidak menghanguskan sisa masa coba |
| `POST /api/v1/subscription-payment-claims` (header `Idempotency-Key`) · `GET` | `billing.manage` | Konfirmasi sudah membayar (nominal, cara, nama pengirim/ref). **Tidak** mengaktifkan apa pun; satu konfirmasi menunggu per tagihan (kedua → 409). `GET /subscription` memuat `payment_claim` terakhir + `payment_instructions` (rekening dari `SUBSCRIPTION_BANK_*`) |
| `GET /api/v1/platform/subscription-payment-claims?status=` | panel: `billing.verify` (superadmin, finance) | Antrean verifikasi (`pending` bawaan, terlama dulu; `approved`\|`rejected`\|`all`) dengan nama usaha, nomor & sisa tagihan |
| `POST /api/v1/platform/subscription-payment-claims/:id/approve` · `/reject` (`reason` ≥ 3 huruf) | panel: `billing.verify` | Setujui → pembayaran dicatat, lunas → aktif + `deferred_revenue_entries`. Tolak → tagihan tetap terbuka, tenant boleh mengirim ulang. Keduanya memberi tahu tenant (WhatsApp) dan tercatat di `audit_logs`; yang sudah diputus → 409 |
| `GET /api/v1/subscription/cancel-preview` | `billing.manage` | Angka pengembalian bila berhenti sekarang (dibayar, bulan terpakai, harga normal) |
| `POST /api/v1/subscription/cancel` | `billing.manage` | Berhenti sekarang (paket → Gratis) + refund (harga bulanan normal; berhenti sebelum masa berbayar dimulai → 0 bulan terpakai). Ada uang kembali → `refund_bank`/`refund_account`/`refund_holder` wajib (422), pengembalian masuk antrean panel. Tagihan terbuka yang belum dibayar dibatalkan; 409 bila ada konfirmasi yang sedang diverifikasi |
| `GET /api/v1/platform/subscription-refunds?status=` · `POST …/:id/paid` (`reference`) | panel: `billing.refund` (superadmin, finance) | Antrean pengembalian dana (`pending` bawaan, terlama dulu; `paid`\|`all`); tandai sudah ditransfer dengan nomor referensi → tenant diberi tahu (WhatsApp), `audit_logs`; kedua kalinya → 409 |
| `POST /api/v1/subscription/change-plan` | `billing.manage` | Terbitkan tagihan **pindah paket** (`kind: plan_change`) dengan kredit sisa masa paket lama (harga bulanan normal, maks. yang dibayar). Paket **baru berpindah saat lunas**, masa berbayarnya mulai hari pelunasan; kredit menutup seluruh biaya → langsung berlaku. Ke Gratis → 422; kredit > biaya → 422 (pilih masa lebih panjang). Tagihan pindah paket yang belum dibayar digantikan bila memilih lagi |
| `POST /api/v1/subscription/invoices/:id/void` | `billing.manage` | Batalkan tagihan pindah paket yang belum dibayar (paket tidak berubah); 409 bila sudah dibayar sebagian atau sedang dikonfirmasi |

**Kunci paket** (`services/plan_entitlement_service.go`). Paket yang BERLAKU dihitung dari waktu:
langganan `trial` s.d. `trial_ends_at`, `active`/`past_due` s.d. `current_period_end` +
`SUBSCRIPTION_GRACE_DAYS`; tanpa langganan / berhenti / lewat → paket **Gratis**. `GET /me` memuat
`plan` (kode, status, `features`, batas, `upgrade_for`). Pelanggaran → **402** dengan pesan yang
menyebut paket termurah yang membukanya.

| Fitur / kuota | Yang dikunci | Yang TIDAK dikunci |
|---|---|---|
| `qris` | QRIS di `POST /sales` & `POST /receivable-payments` | `/sync/push` (transaksi offline sudah terjadi) |
| `online_channel` | tulis `/channels/*`, `POST /channel-orders` | baca; proses/batal pesanan lama; pencairan |
| `crm_freelance` | MEMBUAT prospek, aktivitas, penawaran, proyek, invoice, sumber & alur | baca; ubah/menangkan/kirim; bayar invoice lama |
| `crm_sales` (Pro & Multi-Outlet, migrasi 000038) | rencana kunjungan, check-in langsung (`POST /visits`), target baru | kunjungan offline via `/sync/push`; check-out kunjungan berjalan; hitung/setujui/bayar komisi |
| `multi_outlet` + `max_outlets` | `POST /outlets` (cabang ke-2 dst.) | cabang yang sudah ada |
| `max_users` · `max_products` | `POST /users` · `POST /products` & impor CSV (dikunci per tenant → tidak bisa dibalap) | — |
| `max_monthly_transactions` | — sengaja tidak ditegakkan: penjualan tidak pernah diblokir paket | |
| `GET/POST /api/v1/pipelines` · `/lead-sources` | `crm.lead.view.*` / `crm.deal.edit` | Konfigurasi pipeline & sumber prospek |
| `GET/POST/PUT /api/v1/deals[/:id]` · `.../win` · `.../lose` | `crm.lead.view.*` / `crm.deal.edit` | Deal + menang/kalah (alasan wajib) |
| `GET/POST /api/v1/activities` · `.../complete` · `.../cancel` | `crm.lead.view.*` / `crm.deal.edit` | Aktivitas follow-up |
| `POST /api/v1/quotations` · `.../send` · `.../accept` | `crm.deal.edit` / `quotation.approve` | Penawaran; accept → proyek otomatis |
| `GET/POST/PUT /api/v1/projects[/:id]` · `.../tasks` · `.../expenses` | `crm.lead.view.*` / `crm.deal.edit` | Proyek, tugas, biaya (→ modal saat lunas) |
| `POST /api/v1/invoices` · `.../send` · `.../void` | `invoice.issue` / `invoice.void` | Invoice pelanggan bertermin |
| `POST /api/v1/invoice-payments` (header `Idempotency-Key`) | `invoice.issue` | Pembayaran parsial; lunas → penjualan di `sales` |
| `GET/POST /api/v1/visit-plans[/:id]` | `crm.visit.checkin` | Rencana kunjungan harian + kunjungan `pending` per toko |
| `GET/POST /api/v1/visits[/:id]` · `.../checkout` | `crm.visit.checkin` | Check-in (GPS+foto) & check-out; idempoten per id klien |
| `POST /api/v1/sync/push` `op: visit.upsert` | `crm.visit.checkin` | Sinkron kunjungan offline (tanpa duplikat) |
| `GET/POST /api/v1/sales-targets` | `crm.commission.view` | Target sales + pencapaian (kunjungan & tertagih) |
| `GET/POST /api/v1/commissions` · `.../approve` · `.../pay` | `crm.commission.view` | Hitung komisi dari nilai tertagih → setujui → bayar |
| `GET/POST/PUT/DELETE /api/v1/channels[/:id]` | `channel.manage` | Kanal per outlet + tarif komisi. `GET /channels` mengirim **larik** (bukan halaman) dengan `stats` 30 hari per kanal (pesanan, kotor, komisi, bersih, dibatalkan, pesanan terakhir). `DELETE` hanya menonaktifkan |
| `GET/POST /api/v1/channels/:id/products` · `DELETE .../:pid` | `channel.manage` | Pemetaan SKU kanal ↔ produk |
| `POST /api/v1/channels/:id/orders/import` | `channel.manage` | Impor CSV laporan harian kanal: `external_order_id,date,sku,qty,unit_price,fee_amount` — `sku` dicocokkan ke pemetaan SKU kanal lalu SKU/barcode barang (`product_id` masih diterima). Hasil `{imported, skipped, failed, errors}` |
| `GET/POST/PUT /api/v1/channel-orders[/:id]` · `.../status` · `.../cancel` | `channel.order.accept` | Entri pesanan manual, status, pembatalan. Daftar membawa kotor/komisi/bersih, `receipt_no`, `sale_status`, `occurred_at`, `items` |
| `POST /api/v1/channel-orders/:id/ready` | `channel.order.accept` | **Tandai siap diambil** untuk pesanan GoFood/GrabFood yang tersambung API: GoFood `PUT …/orders/{delivery|pickup}/{order_number}/food-prepared` (jenis dari `service_type` di `raw_payload` pesanan), Grab `POST /partner/v1/orders/mark` (`markStatus: 1`); mencatat `ready_at` & status `ready`, idempoten (tanda kedua tidak diteruskan), 409 bila pesanan sudah batal |
| `POST /webhooks/channels/:provider` (`?merchant_ref=` / `X-Merchant-Ref`) | — (tanpa auth) | Terima webhook kanal → simpan mentah ke `channel_events`, balas 200. Menolak kanal yang sudah tersambung API (jalur ini tak bertanda tangan) |
| `GET/POST /webhooks/channels/:provider/:token[/*aksi]` | tanda tangan / token penyedia | Alamat webhook **per kanal** untuk sambungan API milik tenant. GET = verifikasi alamat (WhatsApp `hub.challenge`); POST diperiksa dengan rahasia kanal itu (WhatsApp: `X-Hub-Signature-256` dengan App Secret tenant; GoFood: `X-Go-Signature` dengan Notification Secret Key tenant) → 401 bila tidak cocok. GoFood: alamat ini didaftarkan otomatis ke GoBiz saat tes koneksi (butuh `APP_URL` HTTPS). GrabFood: Grab meminta token ke sub-jalur `/oauth/token` (Partner Client ID & Secret buatan platform per kanal) lalu mengirim pesanan & status dengan `Authorization: Bearer` token itu. Shopee: push diperiksa dengan `Authorization` = HMAC(Partner Key, URL + "\|" + body) dan dibalas 200 **tanpa isi**; push hanya membawa nomor & status pesanan, rinciannya diambil pekerja lewat `get_order_detail`. GET sub-jalur `/oauth/callback` = penjual kembali dari halaman izin Shopee (state sekali pakai) → kode ditukar token, balasan halaman HTML. Tokopedia & Shop: `Authorization` = HMAC(App Secret, App Key + body); alamat ini didaftarkan otomatis per toko (status pesanan, pembatalan, pencabutan izin) setelah toko memberi izin di `/oauth/callback`; pencabutan izin memutus sambungan. Lazada: `Authorization` = HMAC(App Secret, App Key + body), push per baris pesanan (batal sebagian tidak membatalkan penjualan; batal seluruh barang membatalkannya); push Lazada hanya ke HTTPS bersertifikat OV/EV, jadi pekerja juga menarik `/orders/get` tiap 15 menit per toko sebagai cadangan (dedup dengan push) |
| `GET /api/v1/channel-providers` | `channel.manage` | Katalog penyedia: isian kredensial, langkah mendapatkannya, kemampuan, tersedia/belum |
| `GET/PUT/DELETE /api/v1/channels/:id/connection` · `POST .../test` · `POST .../authorize` | `channel.manage` (+ paket `online_channel` untuk menulis) | Kredensial API milik tenant (terenkripsi; rahasia hanya pratinjau 4 karakter), alamat webhook, tes koneksi ke penyedia, putus. `authorize` (penyedia ber-OAuth: Shopee, Tokopedia & Shop, Lazada) = alamat halaman izin toko dengan state baru; token hasil izin diperbarui otomatis dengan baris kanal terkunci (refresh token Shopee sekali pakai) |
| `GET /api/v1/channels/:id/events?status=` · `GET .../stock-syncs` | `channel.manage` | Inbox peristiwa kanal + antrean sinkron stok (umur keterlambatan) |
| `POST /api/v1/channels/:id/products/match` · `GET .../stock-status` | `channel.manage` (+ paket `online_channel` untuk menulis) | Sinkron stok ke marketplace (Shopee, Tokopedia & Shop, Lazada): **Cocokkan barang** menarik listing penyedia dan memetakan Seller SKU ke SKU/barcode barang toko (`channel_products` + ref penyedia; tanpa pasangan dilaporkan), lalu pekerja mengirim stok outlet − `stock_buffer` (0 bila `is_available=false`) untuk barang terpetakan yang stoknya bergerak (dicocokkan dari `stock_movements` — kasir, retur, penyesuaian, pesanan kanal); gagal dicoba ulang per putaran pekerja lalu ditandai `failed` dengan pesan penyedia |
| `GET/PUT /api/v1/channels/:id/menu` · `POST .../menu/publish` | `channel.manage` (+ paket `online_channel` untuk menulis) | **Menu aplikasi antar dari POS** (GoFood, GrabFood): isi menu = pemetaan `channel_products` (ID item = SKU, `P-<id>` bila kosong; ID yang sama memetakan pesanan masuk); kirim menu wajib `{"confirm":true}` karena MENGGANTI menu di aplikasi antar — GoFood `PUT …/v1/catalog` (katalog utuh, token cakupan `gofood:catalog`), Grab `POST /partner/v1/merchant/menu/notification` lalu Grab mengambil `GET {webhook}/merchant/menu` dengan token partner (409 bila terlalu sering). Sesudahnya habis/tersedia lewat sinkron stok: GoFood `menu_item_stocks`, Grab `batch/menu` (`maxStock`); barang yang stoknya tidak dilacak selalu tersedia |
| `POST /api/v1/channel-events/process` | `channel.manage` | Picu pekerja pemroses manual (selain `cmd/process-channel-events`) |
| `GET/POST /api/v1/channels/:id/settlements` · `.../receipt` | `channel.settlement.view` | Rekonsiliasi pencairan: hitung nilai periode, catat uang masuk (`matched`/`mismatch`) |
| `POST /api/v1/partner/auth/login` | — (realm mitra, bukan tenant) | Masuk portal mitra dengan **email** + password; token realm `partner` ditolak di semua rute tenant |
| `GET /api/v1/partner/{me,dashboard,leads,merchants,commissions,payouts}` · `POST .../leads` | realm `partner` | Portal mitra; `/merchants` = status langganan merchant binaan SAJA (blueprint G.8), akses dicatat |
| `GET/POST/PUT /api/v1/employees[/:id]` · `.../schedule` | `hr.employee.*` | Karyawan + jadwal kerja mingguan |
| `GET/POST /api/v1/attendances` · `/attendance-corrections[/:id/approve]` | `hr.attendance.*` | Absensi (buku besar) + koreksi |
| `POST /api/v1/leave-requests[/:id/approve\|reject]` | `hr.leave.*` | Cuti/izin + persetujuan (snapshot `is_paid`) |
| `GET/POST /api/v1/payroll-rules` | `hr.payroll.run` | Komponen gaji (params per tipe) |
| `POST /api/v1/payroll-periods/:id/{calculate,lock,pay}` | `hr.payroll.{run,lock,pay}` | Siklus gaji deterministik |
| `GET /api/v1/payroll-periods/:id/payslips` · `/payslips/:id` | `hr.salary.view` | Slip gaji + rincian baris (izin paling sensitif) |
| `POST /api/v1/employee-advances[/:id/disburse]` | `hr.advance.approve` | Kasbon → cair (kas keluar) → potong bertahap di gaji |
| `POST /api/v1/shifts/open` · `/shifts/:id/close` | `shift.open` / `shift.close` | Buka / tutup shift |
| `POST /api/v1/shifts/:id/handover` | `shift.close` + `shift.open` | Serah terima: tutup + buka shift baru dalam satu transaksi (shift baru atas nama akun yang masuk) |
| `GET  /api/v1/shifts` · `GET /api/v1/shifts/:id` | `shift.open` / `shift.close` | Daftar / detail shift. Detail membawa rincian laci (`cash_sales`, `cash_in`, `cash_out`), `opened_by_name`/`closed_by_name`, dan `sales` (ringkasan penjualan shift: jumlah, total, retur, batal, per cara bayar) |
| `POST/GET /api/v1/cash-movements` | `cash.movement` | Kas masuk/keluar; daftar membawa `created_by_name` (pencatatnya) |
| `GET  /api/v1/stocks?outlet_id=&low=&q=&product_ids=` | `stock.view` | Saldo stok; `q` cari nama/SKU/barcode, `product_ids` (dipisah koma) untuk barang tertentu |
| `GET  /api/v1/stocks/summary?outlet_id=` | `stock.view` | Ringkasan SELURUH barang: `total`, `safe`, `low`, `out`, `negative`, `stock_value` (harga modal) |
| `GET  /api/v1/stock-movements?product_id=` | `stock.view` | Kartu stok |
| `POST /api/v1/stock-adjustments` | `stock.adjust` | Saldo awal / koreksi stok. Wajib `Idempotency-Key` |
| `POST /api/v1/stock-reconcile?outlet_id=` | `stock.opname` | Hitung ulang cache stok dari buku besar |
| `POST /api/v1/purchases` (header `Idempotency-Key`) | `stock.adjust` | Terima barang (stok masuk) |
| `POST /api/v1/stock-opnames` · `/:id/items` · `/:id/post` | `stock.opname` | Hitung fisik → posting selisih |
| `POST /api/v1/stock-transfers` · `/:id/send` · `/:id/receive` | `stock.transfer` | Transfer antar outlet |
| `GET/PUT /api/v1/products/:id/recipe` | `product.view` / `product.edit` | Resep menu F&B |
| `GET  /api/v1/receivables` · `POST /api/v1/receivable-payments` | `receivable.manage` | Piutang & pelunasan. Setoran wajib `Idempotency-Key` |
| `GET/POST/PUT/DELETE /api/v1/users[/:id]` | `user.manage` | CRUD user staf. `outlet_ids` membatasi cabang tempat staf boleh bekerja (tidak dikirim saat membuat = semua cabang aktif). Ganti password / nonaktifkan / hapus mencabut seluruh sesinya |
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