# Dokumen Teknis Pengembangan Backend

Spesifikasi implementasi untuk backend SaaS POS UMKM. Dokumen ini menerjemahkan
[BLUEPRINT-SAAS-POS.md](BLUEPRINT-SAAS-POS.md) (apa yang dibangun) dan
[BUSINESS-MODEL-CANVAS.md](BUSINESS-MODEL-CANVAS.md) (kenapa dibangun) menjadi keputusan teknis yang
bisa langsung dikerjakan: skema tabel, relasi, kontrak API, dan urutan pengerjaan.

Aturan gaya penulisan kode tetap di [CONVENTIONS.md](../CONVENTIONS.md). Bila dokumen ini dan
CONVENTIONS bertentangan, CONVENTIONS menang untuk gaya kode, dokumen ini menang untuk skema dan
kontrak.

**Stack:** Go 1.24 · Gin · GORM · PostgreSQL 15+ · ULID sebagai primary key · seluruh waktu disimpan UTC.

---

## Daftar Isi

- [1. Sepuluh aturan yang mengikat](#1-sepuluh-aturan-yang-mengikat)
- [2. Struktur proyek](#2-struktur-proyek)
- [3. Konvensi dasar](#3-konvensi-dasar)
  - [3.1 ULID sebagai primary key](#31-ulid-sebagai-primary-key)
  - [3.2 Waktu: simpan UTC, tampilkan per zona](#32-waktu-simpan-utc-tampilkan-per-zona)
  - [3.3 Uang, kuantitas, dan tarif](#33-uang-kuantitas-dan-tarif)
  - [3.4 Penamaan & kolom baku](#34-penamaan--kolom-baku)
  - [3.5 Soft delete & keunikan](#35-soft-delete--keunikan)
- [4. Migrasi berversi](#4-migrasi-berversi)
- [5. Model data lengkap](#5-model-data-lengkap)
- [6. Isolasi tenant](#6-isolasi-tenant)
- [7. Lapisan aplikasi](#7-lapisan-aplikasi)
- [8. Rancangan API](#8-rancangan-api)
- [9. Autentikasi & otorisasi](#9-autentikasi--otorisasi)
- [10. Sinkronisasi offline](#10-sinkronisasi-offline)
- [11. Pekerjaan latar & outbox](#11-pekerjaan-latar--outbox)
- [12. Integrasi eksternal](#12-integrasi-eksternal)
- [13. Algoritma bisnis kritis](#13-algoritma-bisnis-kritis)
- [14. Pengujian](#14-pengujian)
- [15. Observabilitas & konfigurasi](#15-observabilitas--konfigurasi)
- [16. Urutan implementasi bertahap](#16-urutan-implementasi-bertahap)

---

## 1. Sepuluh aturan yang mengikat

Aturan yang bila dilanggar menimbulkan kerusakan yang tidak bisa diperbaiki belakangan tanpa migrasi
data besar. Semua sisanya bisa dinegosiasikan.

| # | Aturan | Akibat bila dilanggar |
|---|---|---|
| 1 | **Setiap tabel bisnis punya `tenant_id`, dan setiap index dimulai darinya** | Kebocoran data antar tenant; query lambat saat tenant bertambah |
| 2 | **Foreign key antar tabel bertenant selalu komposit** `(tenant_id, x_id)` | Relasi lintas tenant bisa terbentuk tanpa terdeteksi |
| 3 | **Primary key ULID `CHAR(26)`, dibuat aplikasi/klien** | Mode offline mustahil; urutan index acak |
| 4 | **Semua waktu `TIMESTAMPTZ` disimpan UTC** | Laporan salah hari untuk outlet di luar WIB |
| 5 | **Tanggal usaha (`business_date`) dihitung dari zona waktu outlet** | "Penjualan hari ini" salah untuk WITA/WIT dan untuk usaha yang tutup lewat tengah malam |
| 6 | **Uang `BIGINT` dalam rupiah bulat** | Selisih pembulatan pada uang pelanggan |
| 7 | **Checkout, retur, dan mutasi stok dalam satu `DB.Transaction`** | Stok dan penjualan tidak cocok, dan tidak bisa direkonsiliasi |
| 8 | **Endpoint yang mengubah uang/stok wajib idempoten** | Transaksi ganda saat jaringan buruk |
| 9 | **`stock_movements` adalah kebenaran, `stocks` hanya cache** | Angka stok melenceng dan tidak bisa diperbaiki dengan jujur |
| 10 | **Skema hanya berubah lewat migrasi berversi** | Perubahan skema di produksi tanpa jejak dan tanpa jalan mundur |

---

## 2. Struktur proyek

Struktur saat ini dipertahankan, dengan penambahan yang dibutuhkan modul-modul baru.

```
/config          # konfigurasi env (sudah ada)
/controllers     # handler HTTP, tipis (sudah ada)
/services        # ← BARU: logika bisnis lintas tabel & transaksi
/repositories    # akses data, semua menerima ctx (sudah ada)
/models          # struct GORM (sudah ada)
/structs         # request/response DTO (sudah ada)
/middlewares     # auth, tenant, permission, rate limit (sudah ada)
/helpers         # utilitas murni (sudah ada)
/database
  /migrations    # ← BARU: berkas migrasi berversi
/jobs            # ← BARU: pekerja latar (outbox, sinkronisasi, komisi)
/adapters        # ← BARU: integrasi luar (kanal, pembayaran, WhatsApp)
/internal/ulid   # ← BARU: pembuatan & validasi ULID
/internal/timez  # ← BARU: konversi zona waktu & business_date
```

**Kenapa `services/` menjadi wajib.** Checkout menyentuh enam tabel dalam satu transaksi. Menaruhnya di
controller membuat transaksi bocor ke lapisan HTTP; menaruhnya di repository membuat repository saling
memanggil. Aturannya: **repository tahu satu tabel, service tahu satu proses bisnis, controller tahu
satu endpoint.**

Repository menerima `*gorm.DB` opsional agar bisa ikut transaksi milik service:

```go
// repositories/stock_repository.go
func AdjustStock(ctx context.Context, tx *gorm.DB, tenantID, outletID, productID string, delta decimal.Decimal) error
// service memanggil dengan tx dari DB.Transaction; pemanggil biasa mengirim nil → pakai DB global
```

---

## 3. Konvensi dasar

### 3.1 ULID sebagai primary key

**Tipe kolom: `CHAR(26) NOT NULL`.** Sudah dipakai `users` dan `roles`; dipertahankan untuk seluruh
tabel agar seragam.

**Kenapa ULID, bukan UUIDv4 atau bigserial:**

| Sifat | Manfaat langsung di produk ini |
|---|---|
| Terurut menurut waktu | Index B-tree menulis di ujung, bukan menyebar — penting untuk tabel `sales` dan `stock_movements` yang tumbuh cepat |
| Bisa dibuat di klien | Kasir offline bisa membuat ID transaksi tanpa server. **Ini yang membuat mode offline mungkin** |
| Tidak berurutan tebak-tebakan | Tidak seperti bigserial yang membocorkan jumlah transaksi dan mengundang enumerasi |
| Ramah manusia | 26 karakter Base32 Crockford, aman disalin di WhatsApp saat pelanggan melapor |

**Yang harus disadari:** ULID membawa stempel waktu pembuatan di 10 karakter pertama. Itu bukan rahasia
yang perlu dilindungi di sini, tetapi jangan pernah menganggap ULID sebagai token akses atau kunci
tebakan-aman.

**Pembuatan ID:**

```go
// internal/ulid/ulid.go
func New() string                    // dibuat server
func Parse(s string) (ulid.ULID, error)
func IsValid(s string) bool          // 26 karakter, alfabet Crockford Base32
func TimeOf(s string) (time.Time, error)
```

- **Server membuat ID** untuk semua entitas master dan platform, lewat hook `BeforeCreate` seperti pola
  yang sudah ada di [models/user.go](../models/user.go).
- **Klien membuat ID** untuk entitas yang lahir di perangkat: `sales`, `sale_items`, `sale_payments`,
  `stock_movements` dari penjualan, `visits`, dan `activities`. Server **menerima** ID dari klien
  setelah divalidasi.

**Validasi wajib di lapisan request.** ID dari klien tidak boleh dipercaya bentuknya:

```go
type SaleCreateRequest struct {
    ID       string `json:"id" binding:"required,ulid"`        // validator kustom
    OutletID string `json:"outlet_id" binding:"required,ulid"`
    // ...
}
```

Daftarkan validator `ulid` sekali di `routes/api.go`, bersama `RegisterTagNameFunc` yang sudah ada.

**Aturan index:** ULID acak-terurut membuat `ORDER BY id` setara `ORDER BY created_at` untuk baris yang
dibuat server. Pakai itu untuk paginasi kursor (lihat [8](#8-rancangan-api)). Untuk baris yang dibuat
klien, urutan ID mengikuti jam perangkat — **jangan** memakai `id` sebagai urutan kejadian di sana;
pakai `created_at` server.

### 3.2 Waktu: simpan UTC, tampilkan per zona

Indonesia memakai tiga zona — **WIB (UTC+7), WITA (UTC+8), WIT (UTC+9)** — dan satu tenant bisa punya
outlet di lebih dari satu zona. Tidak ada perubahan waktu musim panas, tetapi jangan pernah menuliskan
offset `+7` di kode: simpan **nama zona IANA** supaya tetap benar bila aturan berubah.

#### Aturan penyimpanan

| Hal | Aturan |
|---|---|
| Tipe kolom | `TIMESTAMPTZ` untuk semua stempel waktu. Tidak ada `TIMESTAMP` polos |
| Nilai tersimpan | Selalu UTC |
| Sesi database | `DB_TZ=UTC`. **Ubah dari `Asia/Jakarta` yang ada di [.env.example](../.env.example) sekarang** — sebelum ada data transaksi |
| Zona waktu usaha | `outlets.timezone` berisi nama IANA: `Asia/Jakarta`, `Asia/Makassar`, `Asia/Jayapura` |
| Batas hari usaha | `outlets.business_day_start TIME` (default `00:00`), untuk usaha yang tutup lewat tengah malam |
| Format di API | RFC 3339 dengan offset, contoh `2026-09-07T14:30:00+07:00`. Waktu tanpa offset **ditolak** |
| Waktu di Go | `time.Time` selalu dalam UTC di dalam proses; konversi hanya saat menyajikan |

#### Masalah yang diselesaikan: "penjualan hari ini"

Transaksi pukul 23:30 WIT (Jayapura) terjadi pada 14:30 UTC — **hari yang sama** di UTC. Tetapi transaksi
pukul 08:00 WIB terjadi 01:00 UTC, dan transaksi 07:00 WIT terjadi 22:00 UTC **hari sebelumnya**.
Menghitung laporan harian langsung dari kolom UTC akan salah untuk sebagian besar outlet.

Karena itu setiap baris yang masuk laporan harian menyimpan **tanggal usaha yang sudah dihitung**:

```sql
-- kolom pada sales, stock_movements, cash_movements, shifts
business_date DATE NOT NULL
```

Dihitung sekali saat penulisan, di service, bukan saat membaca:

```go
// internal/timez/business.go
func BusinessDate(occurredAt time.Time, tz string, dayStart time.Duration) (time.Time, error) {
    loc, err := time.LoadLocation(tz)          // "Asia/Jayapura"
    if err != nil { return time.Time{}, err }
    local := occurredAt.In(loc).Add(-dayStart) // geser bila hari usaha mulai jam 04:00
    return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC), nil
}
```

Padanan SQL-nya, dipakai saat migrasi mengisi ulang data lama:

```sql
UPDATE sales s SET business_date =
  (((s.occurred_at AT TIME ZONE o.timezone) - o.business_day_start))::date
FROM outlets o WHERE o.id = s.outlet_id;
```

#### Rentang laporan

Endpoint laporan menerima **tanggal lokal** (`from`, `to`) plus `outlet_id`, lalu server menghitung
rentang UTC-nya:

```sql
-- rentang UTC untuk satu hari usaha di sebuah outlet
SELECT
  ((($1::date) + o.business_day_start) AT TIME ZONE o.timezone)     AS from_utc,
  ((($1::date + 1) + o.business_day_start) AT TIME ZONE o.timezone) AS to_utc
FROM outlets o WHERE o.id = $2;
```

Untuk laporan lintas outlet beda zona, **jangan** menggabungkan rentang UTC — gabungkan berdasarkan
`business_date`, karena kolom itu sudah bermakna "hari usaha di outlet masing-masing".

#### Hal lain yang sering salah

- **Shift melewati tengah malam.** Laporan shift memakai `shifts.opened_at`–`closed_at`, bukan tanggal.
  `business_date` pada shift diisi dari `opened_at`.
- **Pekerjaan terjadwal** (rekap sore, pengingat jatuh tempo) dijadwalkan **per zona outlet**, bukan
  sekali untuk semua. Penjadwal menghitung waktu UTC berikutnya untuk tiap zona yang dipakai tenant.
- **`client_created_at`** dari perangkat offline disimpan apa adanya (dengan offset), terpisah dari
  `created_at` server. Jam perangkat sering salah; yang dipercaya untuk urutan adalah waktu server.
- **Jangan** memakai `now()` PostgreSQL dan `time.Now()` Go bergantian untuk satu proses — pilih satu
  sumber waktu per transaksi agar tidak ada selisih milidetik yang membingungkan saat audit.

### 3.3 Uang, kuantitas, dan tarif

| Jenis | Tipe | Aturan |
|---|---|---|
| Uang | `BIGINT` | Rupiah bulat. Tidak ada sen. Tidak pernah `FLOAT`/`NUMERIC` untuk uang |
| Kuantitas | `NUMERIC(14,3)` | Timbangan menjual 0,25 kg |
| Tarif/persentase | `NUMERIC(7,4)` | `0.1500` = 15%. Disimpan sebagai pecahan, bukan angka persen |
| Konversi satuan | `NUMERIC(14,6)` | 1 dus = 24 pcs |

**Aturan pembulatan:** hitung dan bulatkan **per baris** (`ROUND(setengah ke atas)`), lalu jumlahkan
baris untuk mendapat total. Jangan menghitung total dari persentase agregat — hasilnya akan berbeda
beberapa rupiah dari jumlah baris, dan pelanggan akan menemukannya.

Setiap dokumen bernilai uang menyimpan **hasil hitung**, bukan hanya aturannya:

```sql
-- pada sale_items
unit_price      BIGINT NOT NULL,  -- snapshot harga jual saat transaksi
unit_cost       BIGINT NOT NULL,  -- snapshot harga modal saat transaksi
discount_amount BIGINT NOT NULL DEFAULT 0,
tax_amount      BIGINT NOT NULL DEFAULT 0,
line_total      BIGINT NOT NULL   -- qty*unit_price - discount + tax, sudah dibulatkan
```

Tambahkan pemeriksaan konsistensi di service (bukan CHECK constraint, agar pesan errornya bisa
dijelaskan): `sales.total = Σ sale_items.line_total - sales.discount_amount + sales.tax_amount`.

### 3.4 Penamaan & kolom baku

- Tabel: `snake_case` jamak (`sale_items`). Kolom: `snake_case`.
- Primary key selalu bernama `id`.
- Foreign key: `<tabel_tunggal>_id` (`product_id`, `outlet_id`).
- Index: `idx_<tabel>_<kolom>`, unik: `uq_<tabel>_<kolom>`, FK: `fk_<tabel>_<tabel_tujuan>`.
- Enum disimpan sebagai `TEXT` + `CHECK (kolom IN (...))`, bukan tipe ENUM PostgreSQL — menambah nilai
  baru pada tipe ENUM mengunci tabel, sedangkan mengubah CHECK cukup satu migrasi ringan.

Kolom baku pada **semua** tabel bertenant:

```sql
id          CHAR(26)    PRIMARY KEY,
tenant_id   CHAR(26)    NOT NULL,
created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
deleted_at  TIMESTAMPTZ,                        -- hanya pada tabel yang boleh dihapus lunak
created_by  CHAR(26),                           -- users.id, pada tabel transaksional
sync_version BIGINT     NOT NULL DEFAULT nextval('sync_version_seq')
```

`sync_version` dipakai untuk sinkronisasi offline — lihat [10](#10-sinkronisasi-offline).

### 3.5 Soft delete & keunikan

Data transaksional **tidak pernah** dihapus keras. Data master dihapus lunak dengan `deleted_at`.

Ini menyelesaikan utang teknis yang tercatat di CONVENTIONS bagian 7 — unique constraint yang bentrok
dengan baris terhapus — memakai **partial unique index**:

```sql
-- SKU unik per tenant, tapi hanya di antara baris yang masih hidup
CREATE UNIQUE INDEX uq_products_tenant_sku
  ON products (tenant_id, sku) WHERE deleted_at IS NULL;

-- username & email global, pola yang sama
CREATE UNIQUE INDEX uq_users_username ON users (username) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX uq_users_email    ON users (email)    WHERE deleted_at IS NULL;
```

GORM: pakai `gorm.DeletedAt` pada model master, dan **jangan** dipasang pada tabel transaksional supaya
tidak ada penghapusan diam-diam. Pembatalan transaksi memakai kolom `status`, bukan `deleted_at`.

---

## 4. Migrasi berversi

`AutoMigrate` dimatikan sebelum tenant pertama masuk. Penggantinya
[golang-migrate](https://github.com/golang-migrate/migrate) dengan berkas SQL biasa.

```
database/migrations/
  000001_init_platform.up.sql      000001_init_platform.down.sql
  000002_tenancy_and_authz.up.sql  000002_tenancy_and_authz.down.sql
  000003_master_data.up.sql        ...
```

**Aturan:**

1. Satu migrasi = satu perubahan logis. Jangan menggabungkan penambahan tabel dengan pengisian data.
2. Setiap `.up.sql` wajib punya `.down.sql` yang benar-benar diuji, bukan sekadar ada.
3. Migrasi **tidak boleh** memuat logika bisnis. Pengisian ulang data besar dijalankan sebagai perintah
   terpisah yang bisa diulang.
4. Perubahan yang mengunci tabel besar (`ALTER TABLE ... ADD COLUMN NOT NULL DEFAULT`) dipecah:
   tambah nullable → isi bertahap → pasang NOT NULL.
5. Index dibuat `CONCURRENTLY` di produksi, dan migrasi yang memuatnya dijalankan di luar transaksi.
6. Migrasi dijalankan sebagai **langkah deploy terpisah**, bukan saat aplikasi start — agar dua instance
   yang naik bersamaan tidak berebut.

```sh
migrate -path database/migrations -database "$DATABASE_URL" up
migrate -path database/migrations -database "$DATABASE_URL" down 1
migrate -path database/migrations -database "$DATABASE_URL" version
```

Sequence yang dipakai kolom `sync_version` dibuat di migrasi pertama:

```sql
CREATE SEQUENCE sync_version_seq AS BIGINT START 1;
```

---

## 5. Model data lengkap

### 5.1 Peta relasi

```
PLATFORM (tanpa tenant_id)
  plans ──< subscriptions >── tenants
  subscriptions ──< subscription_invoices ──< subscription_payments
                └──< deferred_revenue_entries
  partners ──< partner_users
           ├──< partner_leads
           ├──< partner_referrals ──> tenants
           ├──< partner_commissions ──> subscription_invoices
           └──< partner_payouts
  partner_tiers ──< partners

TENANT
  tenants ──< outlets ──< users (via user_outlets)
          ├──< roles ──< role_permissions >── permissions
          ├──< categories, units, products, suppliers, customers
          ├──< sales ──< sale_items, sale_payments
          ├──< shifts ──< cash_movements
          ├──< stock_movements, stocks
          ├──< purchases ──< purchase_items
          ├──< channels ──< channel_products, channel_orders
          ├──< deals, activities, quotations, invoices, projects, visits
          └──< employees ──< work_schedules
                         ├──< attendances ──< attendance_corrections
                         ├──< attendance_days      (cache, dihitung ulang dari attendances)
                         ├──< leave_requests, leave_balances
                         ├──< employee_advances ──< advance_repayments
                         └──< payslips ──< payslip_lines
              payroll_periods ──< payslips
              payroll_rules  ──> payslip_lines.rule_id (snapshot parameter ikut disimpan)
```

**Aturan relasi yang berlaku di seluruh skema:**

1. Setiap tabel bertenant punya `UNIQUE (tenant_id, id)` — bukan karena `id` kurang unik, melainkan
   supaya bisa menjadi target **foreign key komposit**.
2. Setiap FK ke tabel bertenant memakai bentuk komposit:
   `FOREIGN KEY (tenant_id, product_id) REFERENCES products (tenant_id, id)`.
   Dengan ini, menghubungkan produk tenant A ke penjualan tenant B **ditolak database**, bukan sekadar
   dicegah kode.
3. FK ke tabel platform (`plans`, `partners`) memakai bentuk tunggal biasa.
4. `ON DELETE RESTRICT` untuk semua relasi transaksional. `ON DELETE CASCADE` hanya untuk baris anak
   yang tidak punya makna sendiri (`sale_items`, `role_permissions`, `quotation_items`).

### 5.2 Platform & tenancy

```sql
CREATE TABLE tenants (
  id                CHAR(26) PRIMARY KEY,
  business_name     TEXT NOT NULL,
  business_type     TEXT NOT NULL CHECK (business_type IN ('retail','fnb','service','wholesale','other')),
  owner_name        TEXT NOT NULL,
  phone             TEXT NOT NULL,
  email             TEXT,
  npwp              TEXT,
  nib               TEXT,
  status            TEXT NOT NULL DEFAULT 'trial'
                    CHECK (status IN ('trial','active','past_due','suspended','closed')),
  referred_by_partner_id CHAR(26) REFERENCES partners(id) ON DELETE SET NULL,
  referral_code_used     TEXT,          -- diisi sekali saat daftar, tidak boleh berubah
  created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at        TIMESTAMPTZ
);

CREATE TABLE outlets (
  id                 CHAR(26) PRIMARY KEY,
  tenant_id          CHAR(26) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
  name               TEXT NOT NULL,
  type               TEXT NOT NULL DEFAULT 'store'
                     CHECK (type IN ('store','kitchen','warehouse','vehicle')),  -- vehicle = stok kanvas
  address            TEXT,
  phone              TEXT,
  timezone           TEXT NOT NULL DEFAULT 'Asia/Jakarta',   -- nama IANA, WIB/WITA/WIT
  business_day_start TIME NOT NULL DEFAULT '00:00',          -- untuk usaha yang tutup lewat tengah malam
  currency           TEXT NOT NULL DEFAULT 'IDR',
  tax_enabled        BOOLEAN NOT NULL DEFAULT false,
  tax_rate           NUMERIC(7,4) NOT NULL DEFAULT 0,        -- PPN/PB1, konfigurasi — tidak pernah hard-code
  tax_inclusive      BOOLEAN NOT NULL DEFAULT true,
  service_charge_rate NUMERIC(7,4) NOT NULL DEFAULT 0,
  receipt_header     TEXT,
  receipt_footer     TEXT,
  is_active          BOOLEAN NOT NULL DEFAULT true,
  created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at         TIMESTAMPTZ,
  UNIQUE (tenant_id, id)
);
CREATE INDEX idx_outlets_tenant ON outlets (tenant_id) WHERE deleted_at IS NULL;

-- validasi zona waktu saat insert/update dilakukan di service:
-- time.LoadLocation(tz) harus berhasil, dan tz harus ada di daftar zona Indonesia yang didukung
```

> **Zona waktu ada di outlet, bukan di tenant.** Satu usaha bisa punya cabang di Makassar dan Jayapura.
> Menyimpannya di tenant akan membuat laporan kedua cabang itu salah sejak hari pertama.

### 5.3 Identitas & otorisasi

```sql
ALTER TABLE users
  ADD COLUMN tenant_id CHAR(26) REFERENCES tenants(id) ON DELETE RESTRICT,
  ADD COLUMN pin_hash  TEXT,                    -- PIN cepat ganti kasir, bcrypt
  ADD COLUMN is_active BOOLEAN NOT NULL DEFAULT true,
  ADD COLUMN last_login_at TIMESTAMPTZ,
  ADD COLUMN deleted_at TIMESTAMPTZ;
ALTER TABLE users ADD CONSTRAINT uq_users_tenant_id UNIQUE (tenant_id, id);

ALTER TABLE roles
  ADD COLUMN tenant_id   CHAR(26) REFERENCES tenants(id) ON DELETE CASCADE,  -- NULL = peran bawaan sistem
  ADD COLUMN description TEXT,
  ADD COLUMN is_system   BOOLEAN NOT NULL DEFAULT false;
CREATE UNIQUE INDEX uq_roles_tenant_name ON roles (COALESCE(tenant_id,''), name);

CREATE TABLE permissions (              -- katalog global, diisi seeder
  id    CHAR(26) PRIMARY KEY,
  code  TEXT NOT NULL UNIQUE,           -- 'sale.void', 'report.view', 'product.edit'
  group_name TEXT NOT NULL,
  description TEXT NOT NULL
);

CREATE TABLE role_permissions (
  role_id       CHAR(26) NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
  permission_id CHAR(26) NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
  PRIMARY KEY (role_id, permission_id)
);

CREATE TABLE user_outlets (             -- user boleh diberi akses beberapa outlet
  tenant_id CHAR(26) NOT NULL,
  user_id   CHAR(26) NOT NULL,
  outlet_id CHAR(26) NOT NULL,
  PRIMARY KEY (user_id, outlet_id),
  FOREIGN KEY (tenant_id, user_id)   REFERENCES users   (tenant_id, id) ON DELETE CASCADE,
  FOREIGN KEY (tenant_id, outlet_id) REFERENCES outlets (tenant_id, id) ON DELETE CASCADE
);

CREATE TABLE refresh_tokens (
  id          CHAR(26) PRIMARY KEY,
  user_id     CHAR(26) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash  TEXT NOT NULL,            -- simpan hash, bukan token
  device_name TEXT,
  expires_at  TIMESTAMPTZ NOT NULL,
  revoked_at  TIMESTAMPTZ,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_refresh_tokens_user ON refresh_tokens (user_id) WHERE revoked_at IS NULL;
```

Daftar permission awal (seeder), dikelompokkan supaya UI pengaturan peran mudah dibaca:

```
sale.create  sale.void  sale.refund  sale.discount  sale.price_override
product.view product.edit product.delete product.import
stock.view   stock.adjust stock.opname stock.transfer
report.view  report.profit report.export
shift.open   shift.close  shift.reconcile  cash.movement
customer.view customer.edit receivable.manage
user.manage  role.manage  outlet.manage  setting.manage
crm.lead.view.own  crm.lead.view.all  crm.deal.edit  crm.visit.checkin  crm.commission.view
quotation.approve  invoice.issue  invoice.void
channel.manage  channel.order.accept  channel.settlement.view
```

### 5.4 Master data

```sql
CREATE TABLE categories (
  id CHAR(26) PRIMARY KEY, tenant_id CHAR(26) NOT NULL,
  parent_id CHAR(26),                     -- maksimal 2 tingkat, dijaga di service
  name TEXT NOT NULL, sort_order INT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ, sync_version BIGINT NOT NULL DEFAULT nextval('sync_version_seq'),
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE RESTRICT,
  FOREIGN KEY (tenant_id, parent_id) REFERENCES categories (tenant_id, id) ON DELETE RESTRICT
);

CREATE TABLE units (
  id CHAR(26) PRIMARY KEY, tenant_id CHAR(26) NOT NULL,
  name TEXT NOT NULL,                      -- 'pcs', 'dus', 'kg'
  base_unit_id CHAR(26),                   -- NULL bila ini satuan dasar
  conversion NUMERIC(14,6) NOT NULL DEFAULT 1,  -- 1 dus = 24 pcs → conversion = 24
  allow_decimal BOOLEAN NOT NULL DEFAULT false, -- kg boleh 0,25; pcs tidak
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ, sync_version BIGINT NOT NULL DEFAULT nextval('sync_version_seq'),
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, base_unit_id) REFERENCES units (tenant_id, id) ON DELETE RESTRICT
);

CREATE TABLE products (
  id CHAR(26) PRIMARY KEY, tenant_id CHAR(26) NOT NULL,
  category_id CHAR(26), unit_id CHAR(26) NOT NULL,
  name TEXT NOT NULL,
  sku  TEXT,
  barcode TEXT,
  sell_price BIGINT NOT NULL DEFAULT 0,
  cost_price BIGINT NOT NULL DEFAULT 0,    -- wajib diisi agar laba bisa dihitung
  track_stock BOOLEAN NOT NULL DEFAULT true,
  min_stock NUMERIC(14,3) NOT NULL DEFAULT 0,
  is_active BOOLEAN NOT NULL DEFAULT true,
  image_url TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ, sync_version BIGINT NOT NULL DEFAULT nextval('sync_version_seq'),
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, category_id) REFERENCES categories (tenant_id, id) ON DELETE RESTRICT,
  FOREIGN KEY (tenant_id, unit_id)     REFERENCES units      (tenant_id, id) ON DELETE RESTRICT
);
CREATE UNIQUE INDEX uq_products_tenant_sku     ON products (tenant_id, sku)     WHERE deleted_at IS NULL AND sku IS NOT NULL;
CREATE UNIQUE INDEX uq_products_tenant_barcode ON products (tenant_id, barcode) WHERE deleted_at IS NULL AND barcode IS NOT NULL;
CREATE INDEX idx_products_tenant_active ON products (tenant_id, is_active) WHERE deleted_at IS NULL;
-- pencarian nama cepat (< 200 ms sesuai target blueprint)
CREATE INDEX idx_products_tenant_name_trgm ON products USING gin (name gin_trgm_ops);

CREATE TABLE product_variants (
  id CHAR(26) PRIMARY KEY, tenant_id CHAR(26) NOT NULL, product_id CHAR(26) NOT NULL,
  name TEXT NOT NULL,                      -- 'Besar', 'Pedas'
  sku TEXT, barcode TEXT,
  price_delta BIGINT NOT NULL DEFAULT 0,   -- selisih terhadap harga produk
  is_active BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ, sync_version BIGINT NOT NULL DEFAULT nextval('sync_version_seq'),
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, product_id) REFERENCES products (tenant_id, id) ON DELETE CASCADE
);

CREATE TABLE price_lists (               -- eceran / grosir / member / per kanal
  id CHAR(26) PRIMARY KEY, tenant_id CHAR(26) NOT NULL,
  name TEXT NOT NULL,
  kind TEXT NOT NULL CHECK (kind IN ('retail','wholesale','member','channel')),
  channel_id CHAR(26),                   -- diisi bila kind='channel'
  is_default BOOLEAN NOT NULL DEFAULT false,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ, sync_version BIGINT NOT NULL DEFAULT nextval('sync_version_seq'),
  UNIQUE (tenant_id, id)
);

CREATE TABLE product_prices (
  id CHAR(26) PRIMARY KEY, tenant_id CHAR(26) NOT NULL,
  product_id CHAR(26) NOT NULL, variant_id CHAR(26), price_list_id CHAR(26) NOT NULL,
  min_qty NUMERIC(14,3) NOT NULL DEFAULT 1,   -- harga bertingkat per jumlah
  price BIGINT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  sync_version BIGINT NOT NULL DEFAULT nextval('sync_version_seq'),
  UNIQUE (tenant_id, id),
  UNIQUE (tenant_id, product_id, variant_id, price_list_id, min_qty),
  FOREIGN KEY (tenant_id, product_id)    REFERENCES products    (tenant_id, id) ON DELETE CASCADE,
  FOREIGN KEY (tenant_id, variant_id)    REFERENCES product_variants (tenant_id, id) ON DELETE CASCADE,
  FOREIGN KEY (tenant_id, price_list_id) REFERENCES price_lists (tenant_id, id) ON DELETE CASCADE
);

CREATE TABLE recipes (                   -- F&B: potong bahan baku saat menu terjual
  id CHAR(26) PRIMARY KEY, tenant_id CHAR(26) NOT NULL,
  product_id CHAR(26) NOT NULL,          -- menu jadi
  yield_qty NUMERIC(14,3) NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id), UNIQUE (tenant_id, product_id),
  FOREIGN KEY (tenant_id, product_id) REFERENCES products (tenant_id, id) ON DELETE CASCADE
);

CREATE TABLE recipe_items (
  id CHAR(26) PRIMARY KEY, tenant_id CHAR(26) NOT NULL,
  recipe_id CHAR(26) NOT NULL, ingredient_product_id CHAR(26) NOT NULL,
  qty NUMERIC(14,3) NOT NULL,
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, recipe_id) REFERENCES recipes (tenant_id, id) ON DELETE CASCADE,
  FOREIGN KEY (tenant_id, ingredient_product_id) REFERENCES products (tenant_id, id) ON DELETE RESTRICT
);

CREATE TABLE suppliers (
  id CHAR(26) PRIMARY KEY, tenant_id CHAR(26) NOT NULL,
  name TEXT NOT NULL, phone TEXT, address TEXT, note TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ, sync_version BIGINT NOT NULL DEFAULT nextval('sync_version_seq'),
  UNIQUE (tenant_id, id)
);

CREATE TABLE dining_tables (             -- F&B: nomor meja & open bill
  id CHAR(26) PRIMARY KEY, tenant_id CHAR(26) NOT NULL, outlet_id CHAR(26) NOT NULL,
  name TEXT NOT NULL, capacity INT,
  status TEXT NOT NULL DEFAULT 'free' CHECK (status IN ('free','occupied','reserved')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ, sync_version BIGINT NOT NULL DEFAULT nextval('sync_version_seq'),
  UNIQUE (tenant_id, id), UNIQUE (tenant_id, outlet_id, name),
  FOREIGN KEY (tenant_id, outlet_id) REFERENCES outlets (tenant_id, id) ON DELETE RESTRICT
);
```

### 5.5 Transaksi kasir

Tabel inti seluruh produk. Setiap kolom di sini punya alasan; jangan ada yang dihapus tanpa membaca
[13.1](#131-checkout).

```sql
CREATE TABLE sales (
  id                CHAR(26) PRIMARY KEY,          -- dibuat KLIEN (ULID) agar offline mungkin
  tenant_id         CHAR(26) NOT NULL,
  outlet_id         CHAR(26) NOT NULL,
  shift_id          CHAR(26),
  channel_id        CHAR(26),                      -- NULL = kasir; lihat 5.10
  customer_id       CHAR(26),
  table_id          CHAR(26),                      -- F&B
  receipt_no        TEXT NOT NULL,                 -- bernomor per outlet
  external_order_id TEXT,                          -- id pesanan di kanal luar
  idempotency_key   TEXT NOT NULL,
  order_type        TEXT NOT NULL DEFAULT 'dine_in'
                    CHECK (order_type IN ('dine_in','takeaway','delivery','pickup')),
  status            TEXT NOT NULL DEFAULT 'completed'
                    CHECK (status IN ('draft','pending','accepted','preparing','ready',
                                      'shipped','completed','rejected','canceled','returned')),
  subtotal          BIGINT NOT NULL DEFAULT 0,
  discount_amount   BIGINT NOT NULL DEFAULT 0,
  tax_amount        BIGINT NOT NULL DEFAULT 0,
  service_amount    BIGINT NOT NULL DEFAULT 0,
  rounding_amount   BIGINT NOT NULL DEFAULT 0,
  total             BIGINT NOT NULL DEFAULT 0,
  paid_amount       BIGINT NOT NULL DEFAULT 0,
  change_amount     BIGINT NOT NULL DEFAULT 0,
  cost_total        BIGINT NOT NULL DEFAULT 0,     -- Σ snapshot modal, agar laba tak perlu JOIN produk
  note              TEXT,
  occurred_at       TIMESTAMPTZ NOT NULL,          -- waktu transaksi menurut server (UTC)
  client_created_at TIMESTAMPTZ,                   -- waktu menurut perangkat, apa adanya
  business_date     DATE NOT NULL,                 -- dihitung dari zona & batas hari outlet
  voided_at         TIMESTAMPTZ,
  voided_by         CHAR(26),
  void_reason       TEXT,
  created_by        CHAR(26) NOT NULL,
  created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
  sync_version      BIGINT NOT NULL DEFAULT nextval('sync_version_seq'),
  UNIQUE (tenant_id, id),
  UNIQUE (tenant_id, outlet_id, receipt_no),
  UNIQUE (tenant_id, idempotency_key),
  FOREIGN KEY (tenant_id, outlet_id)   REFERENCES outlets  (tenant_id, id) ON DELETE RESTRICT,
  FOREIGN KEY (tenant_id, shift_id)    REFERENCES shifts   (tenant_id, id) ON DELETE RESTRICT,
  FOREIGN KEY (tenant_id, customer_id) REFERENCES customers(tenant_id, id) ON DELETE RESTRICT,
  FOREIGN KEY (tenant_id, created_by)  REFERENCES users    (tenant_id, id) ON DELETE RESTRICT
);
CREATE INDEX idx_sales_tenant_outlet_date ON sales (tenant_id, outlet_id, business_date);
CREATE INDEX idx_sales_tenant_occurred    ON sales (tenant_id, occurred_at DESC);
CREATE INDEX idx_sales_tenant_shift       ON sales (tenant_id, shift_id);
CREATE INDEX idx_sales_tenant_status      ON sales (tenant_id, status) WHERE status <> 'completed';
CREATE UNIQUE INDEX uq_sales_channel_external
  ON sales (tenant_id, channel_id, external_order_id) WHERE external_order_id IS NOT NULL;

CREATE TABLE sale_items (
  id              CHAR(26) PRIMARY KEY,
  tenant_id       CHAR(26) NOT NULL,
  sale_id         CHAR(26) NOT NULL,
  product_id      CHAR(26) NOT NULL,
  variant_id      CHAR(26),
  product_name    TEXT NOT NULL,          -- SNAPSHOT: nama saat transaksi
  unit_name       TEXT NOT NULL,          -- SNAPSHOT
  qty             NUMERIC(14,3) NOT NULL CHECK (qty > 0),
  unit_price      BIGINT NOT NULL,        -- SNAPSHOT harga jual
  unit_cost       BIGINT NOT NULL,        -- SNAPSHOT harga modal
  discount_amount BIGINT NOT NULL DEFAULT 0,
  tax_amount      BIGINT NOT NULL DEFAULT 0,
  line_total      BIGINT NOT NULL,
  note            TEXT,                   -- 'tanpa es'
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, sale_id)    REFERENCES sales    (tenant_id, id) ON DELETE CASCADE,
  FOREIGN KEY (tenant_id, product_id) REFERENCES products (tenant_id, id) ON DELETE RESTRICT
);
CREATE INDEX idx_sale_items_tenant_sale    ON sale_items (tenant_id, sale_id);
CREATE INDEX idx_sale_items_tenant_product ON sale_items (tenant_id, product_id);

CREATE TABLE sale_payments (
  id           CHAR(26) PRIMARY KEY,
  tenant_id    CHAR(26) NOT NULL,
  sale_id      CHAR(26) NOT NULL,
  method       TEXT NOT NULL CHECK (method IN ('cash','qris','transfer','card','ewallet','credit')),
  amount       BIGINT NOT NULL CHECK (amount > 0),
  reference    TEXT,                      -- nomor referensi QRIS/EDC
  fee_amount   BIGINT NOT NULL DEFAULT 0, -- MDR, agar laba bersih benar
  paid_at      TIMESTAMPTZ NOT NULL,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, sale_id) REFERENCES sales (tenant_id, id) ON DELETE CASCADE
);
CREATE INDEX idx_sale_payments_tenant_sale ON sale_payments (tenant_id, sale_id);
```

> **`method = 'credit'` berarti kasbon** — penjualan tetap tercatat lunas nilainya, tetapi menimbulkan
> baris di `receivables`. Lihat [5.8](#58-pelanggan--piutang).

### 5.6 Stok

```sql
CREATE TABLE stock_movements (           -- BUKU BESAR STOK: sumber kebenaran
  id            CHAR(26) PRIMARY KEY,
  tenant_id     CHAR(26) NOT NULL,
  outlet_id     CHAR(26) NOT NULL,
  product_id    CHAR(26) NOT NULL,
  variant_id    CHAR(26),
  kind          TEXT NOT NULL CHECK (kind IN ('sale','void','refund','purchase','adjustment',
                                              'transfer_in','transfer_out','opname','recipe','initial')),
  qty_delta     NUMERIC(14,3) NOT NULL,  -- negatif untuk keluar
  balance_after NUMERIC(14,3) NOT NULL,  -- saldo setelah gerakan ini, untuk kartu stok
  unit_cost     BIGINT NOT NULL DEFAULT 0,
  ref_table     TEXT,                    -- 'sales', 'purchases', 'stock_opnames'
  ref_id        CHAR(26),
  reason        TEXT,                    -- wajib untuk kind='adjustment'
  occurred_at   TIMESTAMPTZ NOT NULL,
  business_date DATE NOT NULL,
  created_by    CHAR(26),
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  sync_version  BIGINT NOT NULL DEFAULT nextval('sync_version_seq'),
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, outlet_id)  REFERENCES outlets  (tenant_id, id) ON DELETE RESTRICT,
  FOREIGN KEY (tenant_id, product_id) REFERENCES products (tenant_id, id) ON DELETE RESTRICT
);
CREATE INDEX idx_stock_mov_tenant_product ON stock_movements (tenant_id, outlet_id, product_id, occurred_at DESC);
CREATE INDEX idx_stock_mov_tenant_ref     ON stock_movements (tenant_id, ref_table, ref_id);
CREATE INDEX idx_stock_mov_tenant_date    ON stock_movements (tenant_id, business_date);

CREATE TABLE stocks (                    -- CACHE saldo, boleh dihitung ulang kapan saja
  tenant_id   CHAR(26) NOT NULL,
  outlet_id   CHAR(26) NOT NULL,
  product_id  CHAR(26) NOT NULL,
  variant_id  CHAR(26) NOT NULL DEFAULT '',   -- '' = tanpa varian, agar PK tetap sederhana
  qty         NUMERIC(14,3) NOT NULL DEFAULT 0,
  reserved_qty NUMERIC(14,3) NOT NULL DEFAULT 0,  -- alokasi kanal online
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, outlet_id, product_id, variant_id),
  FOREIGN KEY (tenant_id, outlet_id)  REFERENCES outlets  (tenant_id, id) ON DELETE RESTRICT,
  FOREIGN KEY (tenant_id, product_id) REFERENCES products (tenant_id, id) ON DELETE RESTRICT
);
```

Perintah rekonsiliasi wajib ada sejak hari pertama — tanpa ini angka stok yang melenceng tidak bisa
diperbaiki dengan jujur:

```sql
-- jobs/reconcile_stock.go menjalankan ini per outlet
INSERT INTO stocks (tenant_id, outlet_id, product_id, variant_id, qty, updated_at)
SELECT tenant_id, outlet_id, product_id, COALESCE(variant_id,''), SUM(qty_delta), now()
FROM stock_movements WHERE tenant_id = $1 AND outlet_id = $2
GROUP BY tenant_id, outlet_id, product_id, COALESCE(variant_id,'')
ON CONFLICT (tenant_id, outlet_id, product_id, variant_id)
DO UPDATE SET qty = EXCLUDED.qty, updated_at = now();
```

```sql
CREATE TABLE purchases (
  id CHAR(26) PRIMARY KEY, tenant_id CHAR(26) NOT NULL,
  outlet_id CHAR(26) NOT NULL, supplier_id CHAR(26),
  invoice_no TEXT, status TEXT NOT NULL DEFAULT 'received'
    CHECK (status IN ('draft','received','canceled')),
  subtotal BIGINT NOT NULL DEFAULT 0, discount_amount BIGINT NOT NULL DEFAULT 0,
  tax_amount BIGINT NOT NULL DEFAULT 0, total BIGINT NOT NULL DEFAULT 0,
  paid_amount BIGINT NOT NULL DEFAULT 0, due_date DATE,
  occurred_at TIMESTAMPTZ NOT NULL, business_date DATE NOT NULL,
  created_by CHAR(26) NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  sync_version BIGINT NOT NULL DEFAULT nextval('sync_version_seq'),
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, outlet_id)   REFERENCES outlets   (tenant_id, id) ON DELETE RESTRICT,
  FOREIGN KEY (tenant_id, supplier_id) REFERENCES suppliers (tenant_id, id) ON DELETE RESTRICT
);

CREATE TABLE purchase_items (
  id CHAR(26) PRIMARY KEY, tenant_id CHAR(26) NOT NULL,
  purchase_id CHAR(26) NOT NULL, product_id CHAR(26) NOT NULL, variant_id CHAR(26),
  qty NUMERIC(14,3) NOT NULL CHECK (qty > 0),
  unit_cost BIGINT NOT NULL, line_total BIGINT NOT NULL,
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, purchase_id) REFERENCES purchases (tenant_id, id) ON DELETE CASCADE,
  FOREIGN KEY (tenant_id, product_id)  REFERENCES products  (tenant_id, id) ON DELETE RESTRICT
);

CREATE TABLE stock_opnames (
  id CHAR(26) PRIMARY KEY, tenant_id CHAR(26) NOT NULL, outlet_id CHAR(26) NOT NULL,
  status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','posted','canceled')),
  note TEXT, counted_at TIMESTAMPTZ, business_date DATE NOT NULL,
  created_by CHAR(26) NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, outlet_id) REFERENCES outlets (tenant_id, id) ON DELETE RESTRICT
);

CREATE TABLE stock_opname_items (
  id CHAR(26) PRIMARY KEY, tenant_id CHAR(26) NOT NULL,
  opname_id CHAR(26) NOT NULL, product_id CHAR(26) NOT NULL, variant_id CHAR(26),
  system_qty NUMERIC(14,3) NOT NULL,   -- saldo sistem saat hitung dimulai
  counted_qty NUMERIC(14,3) NOT NULL,
  diff_qty NUMERIC(14,3) NOT NULL,
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, opname_id)  REFERENCES stock_opnames (tenant_id, id) ON DELETE CASCADE,
  FOREIGN KEY (tenant_id, product_id) REFERENCES products      (tenant_id, id) ON DELETE RESTRICT
);

CREATE TABLE stock_transfers (
  id CHAR(26) PRIMARY KEY, tenant_id CHAR(26) NOT NULL,
  from_outlet_id CHAR(26) NOT NULL, to_outlet_id CHAR(26) NOT NULL,
  status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','sent','received','canceled')),
  sent_at TIMESTAMPTZ, received_at TIMESTAMPTZ, business_date DATE NOT NULL,
  created_by CHAR(26) NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id),
  CHECK (from_outlet_id <> to_outlet_id),
  FOREIGN KEY (tenant_id, from_outlet_id) REFERENCES outlets (tenant_id, id) ON DELETE RESTRICT,
  FOREIGN KEY (tenant_id, to_outlet_id)   REFERENCES outlets (tenant_id, id) ON DELETE RESTRICT
);

CREATE TABLE stock_transfer_items (
  id CHAR(26) PRIMARY KEY, tenant_id CHAR(26) NOT NULL,
  transfer_id CHAR(26) NOT NULL, product_id CHAR(26) NOT NULL, variant_id CHAR(26),
  qty NUMERIC(14,3) NOT NULL CHECK (qty > 0),
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, transfer_id) REFERENCES stock_transfers (tenant_id, id) ON DELETE CASCADE,
  FOREIGN KEY (tenant_id, product_id)  REFERENCES products        (tenant_id, id) ON DELETE RESTRICT
);
```

### 5.7 Kas & shift

```sql
CREATE TABLE shifts (
  id CHAR(26) PRIMARY KEY, tenant_id CHAR(26) NOT NULL, outlet_id CHAR(26) NOT NULL,
  opened_by CHAR(26) NOT NULL, closed_by CHAR(26),
  opened_at TIMESTAMPTZ NOT NULL, closed_at TIMESTAMPTZ,
  business_date DATE NOT NULL,               -- diisi dari opened_at
  opening_cash BIGINT NOT NULL DEFAULT 0,
  expected_cash BIGINT NOT NULL DEFAULT 0,   -- dihitung sistem saat tutup
  counted_cash BIGINT,                       -- hasil hitung fisik
  difference BIGINT,                         -- counted - expected; boleh negatif
  note TEXT,
  status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open','closed')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  sync_version BIGINT NOT NULL DEFAULT nextval('sync_version_seq'),
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, outlet_id) REFERENCES outlets (tenant_id, id) ON DELETE RESTRICT,
  FOREIGN KEY (tenant_id, opened_by) REFERENCES users   (tenant_id, id) ON DELETE RESTRICT
);
-- satu outlet hanya boleh punya satu shift terbuka
CREATE UNIQUE INDEX uq_shifts_open_per_outlet ON shifts (tenant_id, outlet_id) WHERE status = 'open';

CREATE TABLE cash_movements (
  id CHAR(26) PRIMARY KEY, tenant_id CHAR(26) NOT NULL,
  outlet_id CHAR(26) NOT NULL, shift_id CHAR(26) NOT NULL,
  direction TEXT NOT NULL CHECK (direction IN ('in','out')),
  amount BIGINT NOT NULL CHECK (amount > 0),
  reason TEXT NOT NULL,
  occurred_at TIMESTAMPTZ NOT NULL, business_date DATE NOT NULL,
  created_by CHAR(26) NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  sync_version BIGINT NOT NULL DEFAULT nextval('sync_version_seq'),
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, shift_id) REFERENCES shifts (tenant_id, id) ON DELETE RESTRICT
);
```

### 5.8 Pelanggan & piutang

```sql
CREATE TABLE customers (
  id CHAR(26) PRIMARY KEY, tenant_id CHAR(26) NOT NULL,
  code TEXT, name TEXT NOT NULL, phone TEXT, email TEXT, address TEXT,
  type TEXT NOT NULL DEFAULT 'person' CHECK (type IN ('person','company','store')),
  price_list_id CHAR(26),                 -- harga khusus pelanggan ini
  owner_id CHAR(26),                      -- CRM: sales pemilik data — lihat 6
  credit_limit BIGINT NOT NULL DEFAULT 0,
  latitude NUMERIC(9,6), longitude NUMERIC(9,6),   -- kunjungan sales
  note TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ, sync_version BIGINT NOT NULL DEFAULT nextval('sync_version_seq'),
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, price_list_id) REFERENCES price_lists (tenant_id, id) ON DELETE SET NULL,
  FOREIGN KEY (tenant_id, owner_id)      REFERENCES users       (tenant_id, id) ON DELETE SET NULL
);
CREATE UNIQUE INDEX uq_customers_tenant_phone ON customers (tenant_id, phone) WHERE deleted_at IS NULL AND phone IS NOT NULL;
CREATE INDEX idx_customers_tenant_owner ON customers (tenant_id, owner_id) WHERE deleted_at IS NULL;

CREATE TABLE contact_persons (
  id CHAR(26) PRIMARY KEY, tenant_id CHAR(26) NOT NULL, customer_id CHAR(26) NOT NULL,
  name TEXT NOT NULL, position TEXT, phone TEXT, email TEXT,
  is_primary BOOLEAN NOT NULL DEFAULT false,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, customer_id) REFERENCES customers (tenant_id, id) ON DELETE CASCADE
);

CREATE TABLE receivables (
  id CHAR(26) PRIMARY KEY, tenant_id CHAR(26) NOT NULL,
  customer_id CHAR(26) NOT NULL,
  source_table TEXT NOT NULL CHECK (source_table IN ('sales','invoices')),
  source_id CHAR(26) NOT NULL,
  amount BIGINT NOT NULL CHECK (amount > 0),
  paid_amount BIGINT NOT NULL DEFAULT 0,
  due_date DATE,
  status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open','partial','paid','written_off')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id), UNIQUE (tenant_id, source_table, source_id),
  FOREIGN KEY (tenant_id, customer_id) REFERENCES customers (tenant_id, id) ON DELETE RESTRICT
);
CREATE INDEX idx_receivables_tenant_due ON receivables (tenant_id, due_date) WHERE status IN ('open','partial');

CREATE TABLE receivable_payments (
  id CHAR(26) PRIMARY KEY, tenant_id CHAR(26) NOT NULL, receivable_id CHAR(26) NOT NULL,
  amount BIGINT NOT NULL CHECK (amount > 0),
  method TEXT NOT NULL CHECK (method IN ('cash','qris','transfer','card','ewallet')),
  paid_at TIMESTAMPTZ NOT NULL, business_date DATE NOT NULL,
  collected_by CHAR(26),                   -- sales yang menagih di lapangan
  proof_url TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, receivable_id) REFERENCES receivables (tenant_id, id) ON DELETE RESTRICT
);
```

### 5.9 CRM tenant (Bagian E blueprint)

```sql
CREATE TABLE lead_sources (
  id CHAR(26) PRIMARY KEY, tenant_id CHAR(26) NOT NULL,
  name TEXT NOT NULL,                    -- 'WhatsApp', 'Instagram', 'Marketplace', 'Referral'
  is_active BOOLEAN NOT NULL DEFAULT true,
  UNIQUE (tenant_id, id), UNIQUE (tenant_id, name)
);

CREATE TABLE pipelines (
  id CHAR(26) PRIMARY KEY, tenant_id CHAR(26) NOT NULL,
  name TEXT NOT NULL,
  kind TEXT NOT NULL CHECK (kind IN ('freelance','field_sales','general')),
  is_default BOOLEAN NOT NULL DEFAULT false,
  UNIQUE (tenant_id, id)
);

CREATE TABLE pipeline_stages (
  id CHAR(26) PRIMARY KEY, tenant_id CHAR(26) NOT NULL, pipeline_id CHAR(26) NOT NULL,
  name TEXT NOT NULL, sort_order INT NOT NULL,
  probability NUMERIC(7,4) NOT NULL DEFAULT 0,   -- 0..1
  is_won BOOLEAN NOT NULL DEFAULT false, is_lost BOOLEAN NOT NULL DEFAULT false,
  UNIQUE (tenant_id, id), UNIQUE (tenant_id, pipeline_id, sort_order),
  FOREIGN KEY (tenant_id, pipeline_id) REFERENCES pipelines (tenant_id, id) ON DELETE CASCADE
);

CREATE TABLE deals (
  id CHAR(26) PRIMARY KEY, tenant_id CHAR(26) NOT NULL,
  pipeline_id CHAR(26) NOT NULL, stage_id CHAR(26) NOT NULL,
  customer_id CHAR(26), lead_source_id CHAR(26),
  owner_id CHAR(26) NOT NULL,            -- lapisan visibilitas ketiga, lihat 6
  title TEXT NOT NULL,
  value BIGINT NOT NULL DEFAULT 0,
  expected_close_date DATE,
  status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open','won','lost')),
  lost_reason TEXT,                      -- data paling berharga & paling sering dilupakan
  closed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ, sync_version BIGINT NOT NULL DEFAULT nextval('sync_version_seq'),
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, pipeline_id)    REFERENCES pipelines       (tenant_id, id) ON DELETE RESTRICT,
  FOREIGN KEY (tenant_id, stage_id)       REFERENCES pipeline_stages (tenant_id, id) ON DELETE RESTRICT,
  FOREIGN KEY (tenant_id, customer_id)    REFERENCES customers       (tenant_id, id) ON DELETE RESTRICT,
  FOREIGN KEY (tenant_id, lead_source_id) REFERENCES lead_sources    (tenant_id, id) ON DELETE SET NULL,
  FOREIGN KEY (tenant_id, owner_id)       REFERENCES users           (tenant_id, id) ON DELETE RESTRICT
);
CREATE INDEX idx_deals_tenant_owner_status ON deals (tenant_id, owner_id, status);
CREATE INDEX idx_deals_tenant_stage ON deals (tenant_id, stage_id) WHERE status = 'open';

CREATE TABLE activities (
  id CHAR(26) PRIMARY KEY, tenant_id CHAR(26) NOT NULL,
  kind TEXT NOT NULL CHECK (kind IN ('call','chat','meeting','visit','task','note')),
  subject TEXT NOT NULL, body TEXT,
  owner_id CHAR(26) NOT NULL,
  customer_id CHAR(26), deal_id CHAR(26),
  due_at TIMESTAMPTZ, completed_at TIMESTAMPTZ,
  status TEXT NOT NULL DEFAULT 'planned' CHECK (status IN ('planned','done','canceled')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  sync_version BIGINT NOT NULL DEFAULT nextval('sync_version_seq'),
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, owner_id)    REFERENCES users     (tenant_id, id) ON DELETE RESTRICT,
  FOREIGN KEY (tenant_id, customer_id) REFERENCES customers (tenant_id, id) ON DELETE CASCADE,
  FOREIGN KEY (tenant_id, deal_id)     REFERENCES deals     (tenant_id, id) ON DELETE CASCADE
);
CREATE INDEX idx_activities_tenant_owner_due ON activities (tenant_id, owner_id, due_at) WHERE status = 'planned';

CREATE TABLE quotations (
  id CHAR(26) PRIMARY KEY, tenant_id CHAR(26) NOT NULL,
  number TEXT NOT NULL, customer_id CHAR(26) NOT NULL, deal_id CHAR(26), owner_id CHAR(26) NOT NULL,
  valid_until DATE,
  status TEXT NOT NULL DEFAULT 'draft'
         CHECK (status IN ('draft','sent','accepted','rejected','expired')),
  subtotal BIGINT NOT NULL DEFAULT 0, discount_amount BIGINT NOT NULL DEFAULT 0,
  tax_amount BIGINT NOT NULL DEFAULT 0, total BIGINT NOT NULL DEFAULT 0,
  note TEXT, accepted_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id), UNIQUE (tenant_id, number),
  FOREIGN KEY (tenant_id, customer_id) REFERENCES customers (tenant_id, id) ON DELETE RESTRICT,
  FOREIGN KEY (tenant_id, deal_id)     REFERENCES deals     (tenant_id, id) ON DELETE SET NULL
);

CREATE TABLE quotation_items (
  id CHAR(26) PRIMARY KEY, tenant_id CHAR(26) NOT NULL, quotation_id CHAR(26) NOT NULL,
  product_id CHAR(26), description TEXT NOT NULL,
  qty NUMERIC(14,3) NOT NULL, unit_price BIGINT NOT NULL,
  discount_amount BIGINT NOT NULL DEFAULT 0, line_total BIGINT NOT NULL,
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, quotation_id) REFERENCES quotations (tenant_id, id) ON DELETE CASCADE,
  FOREIGN KEY (tenant_id, product_id)   REFERENCES products   (tenant_id, id) ON DELETE SET NULL
);

CREATE TABLE projects (
  id CHAR(26) PRIMARY KEY, tenant_id CHAR(26) NOT NULL,
  customer_id CHAR(26) NOT NULL, quotation_id CHAR(26), owner_id CHAR(26) NOT NULL,
  name TEXT NOT NULL, start_date DATE, due_date DATE,
  status TEXT NOT NULL DEFAULT 'active'
         CHECK (status IN ('active','on_hold','completed','canceled')),
  contract_value BIGINT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, customer_id)  REFERENCES customers  (tenant_id, id) ON DELETE RESTRICT,
  FOREIGN KEY (tenant_id, quotation_id) REFERENCES quotations (tenant_id, id) ON DELETE SET NULL
);

CREATE TABLE project_tasks (
  id CHAR(26) PRIMARY KEY, tenant_id CHAR(26) NOT NULL, project_id CHAR(26) NOT NULL,
  title TEXT NOT NULL, due_date DATE, done_at TIMESTAMPTZ, sort_order INT NOT NULL DEFAULT 0,
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, project_id) REFERENCES projects (tenant_id, id) ON DELETE CASCADE
);

CREATE TABLE project_expenses (
  id CHAR(26) PRIMARY KEY, tenant_id CHAR(26) NOT NULL, project_id CHAR(26) NOT NULL,
  description TEXT NOT NULL, amount BIGINT NOT NULL CHECK (amount > 0),
  spent_at DATE NOT NULL, receipt_url TEXT,
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, project_id) REFERENCES projects (tenant_id, id) ON DELETE CASCADE
);

CREATE TABLE invoices (                  -- invoice PELANGGAN (bukan tagihan langganan platform)
  id CHAR(26) PRIMARY KEY, tenant_id CHAR(26) NOT NULL,
  number TEXT NOT NULL, customer_id CHAR(26) NOT NULL,
  project_id CHAR(26), quotation_id CHAR(26), owner_id CHAR(26) NOT NULL,
  issue_date DATE NOT NULL, due_date DATE NOT NULL,
  status TEXT NOT NULL DEFAULT 'draft'
         CHECK (status IN ('draft','sent','partial','paid','overdue','void')),
  subtotal BIGINT NOT NULL DEFAULT 0, discount_amount BIGINT NOT NULL DEFAULT 0,
  tax_amount BIGINT NOT NULL DEFAULT 0, total BIGINT NOT NULL DEFAULT 0,
  paid_amount BIGINT NOT NULL DEFAULT 0,
  term_label TEXT,                       -- 'DP 50%', 'Pelunasan'
  sale_id CHAR(26),                      -- diisi saat lunas → tercatat sebagai penjualan
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id), UNIQUE (tenant_id, number),
  FOREIGN KEY (tenant_id, customer_id) REFERENCES customers (tenant_id, id) ON DELETE RESTRICT,
  FOREIGN KEY (tenant_id, project_id)  REFERENCES projects  (tenant_id, id) ON DELETE SET NULL,
  FOREIGN KEY (tenant_id, sale_id)     REFERENCES sales     (tenant_id, id) ON DELETE SET NULL
);
CREATE INDEX idx_invoices_tenant_due ON invoices (tenant_id, due_date) WHERE status IN ('sent','partial','overdue');

CREATE TABLE invoice_items (
  id CHAR(26) PRIMARY KEY, tenant_id CHAR(26) NOT NULL, invoice_id CHAR(26) NOT NULL,
  product_id CHAR(26), description TEXT NOT NULL,
  qty NUMERIC(14,3) NOT NULL, unit_price BIGINT NOT NULL, line_total BIGINT NOT NULL,
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, invoice_id) REFERENCES invoices (tenant_id, id) ON DELETE CASCADE
);

CREATE TABLE invoice_payments (
  id CHAR(26) PRIMARY KEY, tenant_id CHAR(26) NOT NULL, invoice_id CHAR(26) NOT NULL,
  amount BIGINT NOT NULL CHECK (amount > 0),
  method TEXT NOT NULL CHECK (method IN ('cash','qris','transfer','card','ewallet')),
  paid_at TIMESTAMPTZ NOT NULL, business_date DATE NOT NULL, proof_url TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, invoice_id) REFERENCES invoices (tenant_id, id) ON DELETE RESTRICT
);

CREATE TABLE visit_plans (
  id CHAR(26) PRIMARY KEY, tenant_id CHAR(26) NOT NULL,
  owner_id CHAR(26) NOT NULL, plan_date DATE NOT NULL,
  status TEXT NOT NULL DEFAULT 'planned' CHECK (status IN ('planned','running','done')),
  UNIQUE (tenant_id, id), UNIQUE (tenant_id, owner_id, plan_date),
  FOREIGN KEY (tenant_id, owner_id) REFERENCES users (tenant_id, id) ON DELETE CASCADE
);

CREATE TABLE visits (
  id CHAR(26) PRIMARY KEY,               -- dibuat KLIEN, dibuat offline di lapangan
  tenant_id CHAR(26) NOT NULL, visit_plan_id CHAR(26), customer_id CHAR(26) NOT NULL,
  owner_id CHAR(26) NOT NULL,
  checkin_at TIMESTAMPTZ, checkout_at TIMESTAMPTZ,
  checkin_lat NUMERIC(9,6), checkin_lng NUMERIC(9,6),   -- HANYA saat check-in/out, bukan pelacakan terus-menerus
  photo_url TEXT,
  result TEXT NOT NULL DEFAULT 'pending'
         CHECK (result IN ('pending','order','no_order','closed','rejected')),
  no_order_reason TEXT,
  sale_id CHAR(26),                      -- pesanan yang diambil di lokasi
  business_date DATE NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  sync_version BIGINT NOT NULL DEFAULT nextval('sync_version_seq'),
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, customer_id)   REFERENCES customers   (tenant_id, id) ON DELETE RESTRICT,
  FOREIGN KEY (tenant_id, owner_id)      REFERENCES users       (tenant_id, id) ON DELETE RESTRICT,
  FOREIGN KEY (tenant_id, visit_plan_id) REFERENCES visit_plans (tenant_id, id) ON DELETE SET NULL,
  FOREIGN KEY (tenant_id, sale_id)       REFERENCES sales       (tenant_id, id) ON DELETE SET NULL
);
CREATE INDEX idx_visits_tenant_owner_date ON visits (tenant_id, owner_id, business_date);

CREATE TABLE sales_targets (
  id CHAR(26) PRIMARY KEY, tenant_id CHAR(26) NOT NULL,
  user_id CHAR(26) NOT NULL, period_start DATE NOT NULL, period_end DATE NOT NULL,
  target_amount BIGINT NOT NULL DEFAULT 0, target_visits INT NOT NULL DEFAULT 0,
  UNIQUE (tenant_id, id), UNIQUE (tenant_id, user_id, period_start),
  FOREIGN KEY (tenant_id, user_id) REFERENCES users (tenant_id, id) ON DELETE CASCADE
);

CREATE TABLE commissions (               -- komisi SALES TENANT (bukan mitra penjual aplikasi)
  id CHAR(26) PRIMARY KEY, tenant_id CHAR(26) NOT NULL,
  user_id CHAR(26) NOT NULL, period_start DATE NOT NULL, period_end DATE NOT NULL,
  base_amount BIGINT NOT NULL DEFAULT 0, -- dasar: nilai TERTAGIH, bukan terkirim
  rate NUMERIC(7,4) NOT NULL, amount BIGINT NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','approved','paid')),
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, user_id) REFERENCES users (tenant_id, id) ON DELETE RESTRICT
);
```

### 5.10 Kanal pesanan online (Bagian F blueprint)

```sql
CREATE TABLE channels (
  id CHAR(26) PRIMARY KEY, tenant_id CHAR(26) NOT NULL, outlet_id CHAR(26) NOT NULL,
  kind TEXT NOT NULL CHECK (kind IN ('pos','marketplace','delivery_app','conversation')),
  provider TEXT NOT NULL,                -- 'gofood','grabfood','shopeefood','tokopedia','shopee','tiktok','whatsapp'
  name TEXT NOT NULL,
  merchant_ref TEXT,                     -- identitas merchant kita di kanal itu
  credentials_encrypted BYTEA,           -- terenkripsi di aplikasi, kunci dari env
  commission_rate NUMERIC(7,4) NOT NULL DEFAULT 0,   -- konfigurasi, wajib diverifikasi ke perjanjian
  price_list_id CHAR(26),                -- harga khusus kanal
  integration_mode TEXT NOT NULL DEFAULT 'manual'
         CHECK (integration_mode IN ('manual','csv','api')),
  is_active BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id), UNIQUE (tenant_id, outlet_id, provider),
  FOREIGN KEY (tenant_id, outlet_id)     REFERENCES outlets     (tenant_id, id) ON DELETE RESTRICT,
  FOREIGN KEY (tenant_id, price_list_id) REFERENCES price_lists (tenant_id, id) ON DELETE SET NULL
);

CREATE TABLE channel_products (
  id CHAR(26) PRIMARY KEY, tenant_id CHAR(26) NOT NULL,
  channel_id CHAR(26) NOT NULL, product_id CHAR(26) NOT NULL, variant_id CHAR(26),
  external_sku TEXT NOT NULL,            -- SKU di kanal, hampir tidak pernah sama
  external_product_id TEXT,
  channel_price BIGINT,                  -- bila berbeda dari price_list
  is_available BOOLEAN NOT NULL DEFAULT true,
  stock_buffer NUMERIC(14,3) NOT NULL DEFAULT 0,   -- cadangan penyangga anti-overselling
  last_synced_at TIMESTAMPTZ,
  UNIQUE (tenant_id, id), UNIQUE (tenant_id, channel_id, external_sku),
  FOREIGN KEY (tenant_id, channel_id) REFERENCES channels (tenant_id, id) ON DELETE CASCADE,
  FOREIGN KEY (tenant_id, product_id) REFERENCES products (tenant_id, id) ON DELETE RESTRICT
);

CREATE TABLE channel_events (            -- kotak masuk mentah; webhook TIDAK memproses langsung
  id CHAR(26) PRIMARY KEY, tenant_id CHAR(26) NOT NULL, channel_id CHAR(26) NOT NULL,
  event_type TEXT NOT NULL,
  external_ref TEXT,
  payload JSONB NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending'
         CHECK (status IN ('pending','processing','done','failed','dead')),
  attempts INT NOT NULL DEFAULT 0, last_error TEXT,
  received_at TIMESTAMPTZ NOT NULL DEFAULT now(), processed_at TIMESTAMPTZ,
  UNIQUE (tenant_id, id),
  UNIQUE (tenant_id, channel_id, event_type, external_ref),   -- dedup pengiriman ganda
  FOREIGN KEY (tenant_id, channel_id) REFERENCES channels (tenant_id, id) ON DELETE CASCADE
);
CREATE INDEX idx_channel_events_pending ON channel_events (status, received_at) WHERE status IN ('pending','failed');

CREATE TABLE channel_orders (            -- data khas kanal atas sebuah sales
  id CHAR(26) PRIMARY KEY, tenant_id CHAR(26) NOT NULL,
  channel_id CHAR(26) NOT NULL, sale_id CHAR(26) NOT NULL,
  external_order_id TEXT NOT NULL, external_status TEXT,
  buyer_name TEXT, buyer_phone TEXT, shipping_address TEXT,
  courier TEXT, tracking_no TEXT, driver_name TEXT,
  accepted_at TIMESTAMPTZ, ready_at TIMESTAMPTZ, completed_at TIMESTAMPTZ,
  raw_payload JSONB,
  UNIQUE (tenant_id, id), UNIQUE (tenant_id, channel_id, external_order_id),
  FOREIGN KEY (tenant_id, channel_id) REFERENCES channels (tenant_id, id) ON DELETE RESTRICT,
  FOREIGN KEY (tenant_id, sale_id)    REFERENCES sales    (tenant_id, id) ON DELETE CASCADE
);

CREATE TABLE channel_fees (
  id CHAR(26) PRIMARY KEY, tenant_id CHAR(26) NOT NULL, sale_id CHAR(26) NOT NULL,
  kind TEXT NOT NULL CHECK (kind IN ('commission','service','shipping_subsidy','merchant_promo','tax','other')),
  amount BIGINT NOT NULL,                -- positif = potongan bagi kita
  note TEXT,
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, sale_id) REFERENCES sales (tenant_id, id) ON DELETE CASCADE
);
CREATE INDEX idx_channel_fees_tenant_sale ON channel_fees (tenant_id, sale_id);

CREATE TABLE channel_settlements (
  id CHAR(26) PRIMARY KEY, tenant_id CHAR(26) NOT NULL, channel_id CHAR(26) NOT NULL,
  period_start DATE NOT NULL, period_end DATE NOT NULL,
  gross_amount BIGINT NOT NULL DEFAULT 0, fee_amount BIGINT NOT NULL DEFAULT 0,
  net_amount BIGINT NOT NULL DEFAULT 0, received_amount BIGINT,
  status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open','matched','mismatch','closed')),
  received_at TIMESTAMPTZ, note TEXT,
  UNIQUE (tenant_id, id), UNIQUE (tenant_id, channel_id, period_start),
  FOREIGN KEY (tenant_id, channel_id) REFERENCES channels (tenant_id, id) ON DELETE RESTRICT
);

CREATE TABLE channel_stock_syncs (
  id CHAR(26) PRIMARY KEY, tenant_id CHAR(26) NOT NULL,
  channel_id CHAR(26) NOT NULL, product_id CHAR(26) NOT NULL,
  requested_qty NUMERIC(14,3) NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','sent','failed')),
  attempts INT NOT NULL DEFAULT 0, last_error TEXT,
  queued_at TIMESTAMPTZ NOT NULL DEFAULT now(), sent_at TIMESTAMPTZ,
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, channel_id) REFERENCES channels (tenant_id, id) ON DELETE CASCADE
);
CREATE INDEX idx_channel_stock_syncs_pending ON channel_stock_syncs (status, queued_at) WHERE status <> 'sent';
```

### 5.11 Absensi & penggajian (Bagian H blueprint)

Modul ini memakai disiplin yang sama dengan stok: **`attendances` adalah kebenaran, `attendance_days`
hanyalah cache yang boleh dihitung ulang kapan saja.**

```sql
CREATE TABLE employees (
  id CHAR(26) PRIMARY KEY, tenant_id CHAR(26) NOT NULL,
  user_id CHAR(26),                        -- NULL: karyawan tanpa akun aplikasi (mis. juru masak)
  outlet_id CHAR(26) NOT NULL,
  employee_no TEXT, full_name TEXT NOT NULL, phone TEXT, email TEXT,
  id_number TEXT, npwp TEXT, address TEXT, birth_date DATE,   -- data pribadi: simpan seperlunya
  position TEXT,
  employment_status TEXT NOT NULL DEFAULT 'permanent'
    CHECK (employment_status IN ('permanent','contract','probation','daily')),
  wage_type TEXT NOT NULL CHECK (wage_type IN ('monthly','daily','hourly')),
  base_wage BIGINT NOT NULL DEFAULT 0,
  payroll_period_type TEXT NOT NULL DEFAULT 'monthly'
    CHECK (payroll_period_type IN ('daily','weekly','biweekly','monthly')),
  bank_name TEXT, bank_account_no TEXT, bank_account_name TEXT,
  joined_at DATE NOT NULL, resigned_at DATE,
  is_active BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ, sync_version BIGINT NOT NULL DEFAULT nextval('sync_version_seq'),
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, outlet_id) REFERENCES outlets (tenant_id, id) ON DELETE RESTRICT,
  FOREIGN KEY (tenant_id, user_id)   REFERENCES users   (tenant_id, id) ON DELETE SET NULL
);
-- satu akun aplikasi hanya boleh terikat ke satu karyawan
CREATE UNIQUE INDEX uq_employees_user ON employees (tenant_id, user_id)
  WHERE user_id IS NOT NULL AND deleted_at IS NULL;
CREATE UNIQUE INDEX uq_employees_no   ON employees (tenant_id, employee_no)
  WHERE employee_no IS NOT NULL AND deleted_at IS NULL;

CREATE TABLE work_schedules (
  id CHAR(26) PRIMARY KEY, tenant_id CHAR(26) NOT NULL, employee_id CHAR(26) NOT NULL,
  weekday SMALLINT NOT NULL CHECK (weekday BETWEEN 0 AND 6),   -- 0 = Minggu
  is_working_day BOOLEAN NOT NULL DEFAULT true,
  start_time TIME, end_time TIME,          -- end < start berarti melewati tengah malam
  break_minutes INT NOT NULL DEFAULT 0,
  late_tolerance_minutes INT NOT NULL DEFAULT 0,
  effective_from DATE NOT NULL, effective_to DATE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id),
  UNIQUE (tenant_id, employee_id, weekday, effective_from),
  CHECK (effective_to IS NULL OR effective_to >= effective_from),
  FOREIGN KEY (tenant_id, employee_id) REFERENCES employees (tenant_id, id) ON DELETE CASCADE
);

CREATE TABLE attendances (               -- BUKU BESAR ABSENSI, hanya tambah
  id CHAR(26) PRIMARY KEY,               -- dibuat KLIEN: absen bisa terjadi saat offline
  tenant_id CHAR(26) NOT NULL, employee_id CHAR(26) NOT NULL, outlet_id CHAR(26) NOT NULL,
  kind TEXT NOT NULL CHECK (kind IN ('in','out')),
  occurred_at TIMESTAMPTZ NOT NULL,      -- UTC
  business_date DATE NOT NULL,           -- dihitung server dari zona & batas hari outlet
  source TEXT NOT NULL DEFAULT 'manual'
    CHECK (source IN ('manual','shift','import','correction')),
  photo_url TEXT, latitude NUMERIC(9,6), longitude NUMERIC(9,6),  -- opsional per tenant
  device_id TEXT, client_created_at TIMESTAMPTZ,
  reason TEXT,                           -- wajib bila source='correction'
  approved_by CHAR(26),                  -- wajib bila source='correction'
  created_by CHAR(26) NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  sync_version BIGINT NOT NULL DEFAULT nextval('sync_version_seq'),
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, employee_id) REFERENCES employees (tenant_id, id) ON DELETE RESTRICT,
  FOREIGN KEY (tenant_id, outlet_id)   REFERENCES outlets   (tenant_id, id) ON DELETE RESTRICT
);
CREATE INDEX idx_attendances_emp_date ON attendances (tenant_id, employee_id, business_date);
CREATE INDEX idx_attendances_outlet_date ON attendances (tenant_id, outlet_id, business_date);

CREATE TABLE attendance_corrections (    -- koreksi waktu; baris asli TIDAK diubah
  id CHAR(26) PRIMARY KEY, tenant_id CHAR(26) NOT NULL,
  attendance_id CHAR(26) NOT NULL,
  new_occurred_at TIMESTAMPTZ NOT NULL,
  reason TEXT NOT NULL,
  requested_by CHAR(26) NOT NULL, approved_by CHAR(26), approved_at TIMESTAMPTZ,
  status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','approved','rejected')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, attendance_id) REFERENCES attendances (tenant_id, id) ON DELETE RESTRICT
);
-- satu baris absensi hanya boleh punya satu koreksi disetujui
CREATE UNIQUE INDEX uq_attendance_corr_approved
  ON attendance_corrections (tenant_id, attendance_id) WHERE status = 'approved';

CREATE TABLE attendance_days (           -- CACHE status harian, boleh dihitung ulang
  tenant_id CHAR(26) NOT NULL,
  employee_id CHAR(26) NOT NULL,
  business_date DATE NOT NULL,
  status TEXT NOT NULL CHECK (status IN ('present','late','leave','sick','permit','holiday','off','absent')),
  scheduled_start TIMESTAMPTZ, scheduled_end TIMESTAMPTZ,
  first_in TIMESTAMPTZ, last_out TIMESTAMPTZ,
  late_minutes INT NOT NULL DEFAULT 0,
  early_leave_minutes INT NOT NULL DEFAULT 0,
  work_minutes INT NOT NULL DEFAULT 0,
  overtime_minutes INT NOT NULL DEFAULT 0,
  leave_request_id CHAR(26),
  computed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, employee_id, business_date),
  FOREIGN KEY (tenant_id, employee_id) REFERENCES employees (tenant_id, id) ON DELETE CASCADE
);

CREATE TABLE holidays (
  id CHAR(26) PRIMARY KEY, tenant_id CHAR(26) NOT NULL,
  outlet_id CHAR(26),                    -- NULL = berlaku semua outlet
  holiday_date DATE NOT NULL, name TEXT NOT NULL,
  is_paid BOOLEAN NOT NULL DEFAULT true,
  UNIQUE (tenant_id, id)
);
CREATE UNIQUE INDEX uq_holidays_date
  ON holidays (tenant_id, COALESCE(outlet_id,''), holiday_date);

CREATE TABLE leave_requests (
  id CHAR(26) PRIMARY KEY, tenant_id CHAR(26) NOT NULL, employee_id CHAR(26) NOT NULL,
  kind TEXT NOT NULL CHECK (kind IN ('permit','sick','leave','unpaid')),
  start_date DATE NOT NULL, end_date DATE NOT NULL,
  days NUMERIC(5,1) NOT NULL,            -- 0.5 untuk setengah hari
  reason TEXT, attachment_url TEXT,
  is_paid BOOLEAN NOT NULL,              -- SNAPSHOT kebijakan saat disetujui
  status TEXT NOT NULL DEFAULT 'pending'
    CHECK (status IN ('pending','approved','rejected','canceled')),
  approved_by CHAR(26), approved_at TIMESTAMPTZ, reject_reason TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  sync_version BIGINT NOT NULL DEFAULT nextval('sync_version_seq'),
  UNIQUE (tenant_id, id),
  CHECK (end_date >= start_date),
  FOREIGN KEY (tenant_id, employee_id) REFERENCES employees (tenant_id, id) ON DELETE CASCADE
);
CREATE INDEX idx_leave_emp_range ON leave_requests (tenant_id, employee_id, start_date, end_date)
  WHERE status = 'approved';

CREATE TABLE leave_balances (
  tenant_id CHAR(26) NOT NULL, employee_id CHAR(26) NOT NULL, year SMALLINT NOT NULL,
  quota_days NUMERIC(5,1) NOT NULL DEFAULT 12,
  used_days NUMERIC(5,1) NOT NULL DEFAULT 0,
  carried_over_days NUMERIC(5,1) NOT NULL DEFAULT 0,
  PRIMARY KEY (tenant_id, employee_id, year),
  FOREIGN KEY (tenant_id, employee_id) REFERENCES employees (tenant_id, id) ON DELETE CASCADE
);

CREATE TABLE payroll_rules (
  id CHAR(26) PRIMARY KEY, tenant_id CHAR(26) NOT NULL,
  code TEXT NOT NULL, name TEXT NOT NULL,
  type TEXT NOT NULL CHECK (type IN ('tunjangan_tetap','potongan_telat','potongan_alpa',
        'upah_lembur','bonus_kehadiran','bonus_target','potongan_kasbon','komponen_manual')),
  category TEXT NOT NULL CHECK (category IN ('earning','deduction')),
  params JSONB NOT NULL,                 -- parameter khas tipe; divalidasi per tipe di service
  target_type TEXT NOT NULL DEFAULT 'all' CHECK (target_type IN ('all','role','employee')),
  target_id CHAR(26),                    -- roles.id atau employees.id sesuai target_type
  priority INT NOT NULL DEFAULT 100,     -- urutan di dalam kelompoknya
  effective_from DATE NOT NULL, effective_to DATE,
  is_active BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id),
  UNIQUE (tenant_id, code, effective_from),
  CHECK (effective_to IS NULL OR effective_to >= effective_from),
  CHECK ((target_type = 'all') = (target_id IS NULL))
);
CREATE INDEX idx_payroll_rules_active ON payroll_rules (tenant_id, effective_from, effective_to)
  WHERE is_active;
```

> **Kenapa `params` boleh JSONB.** Ini melanggar aturan "JSONB hanya untuk muatan pihak luar" di
> [5.15](#515-aturan-index--constraint-yang-berlaku-umum) secara sengaja, dengan tiga syarat: bentuknya
> divalidasi per `type` di service (bukan bebas), nilainya **tidak pernah** dicari atau dijumlahkan lewat
> SQL, dan salinannya dibekukan ke `payslip_lines.params_snapshot` saat dipakai. Alternatifnya — satu
> kolom per parameter untuk delapan tipe aturan — menghasilkan tabel dengan tiga puluh kolom yang
> sebagian besar selalu NULL.

Bentuk `params` per tipe, sebagai kontrak yang divalidasi service:

```jsonc
tunjangan_tetap  { "amount": 15000, "per": "day"|"period", "require_present": true }
potongan_telat   { "threshold_minutes": 15, "mode": "per_minute"|"per_event"|"tiered",
                   "amount": 10000, "tiers": [{"from_minutes":30,"amount":25000}] }
potongan_alpa    { "mode": "amount"|"daily_wage_percent", "amount": 0, "percent": 1.0 }
upah_lembur      { "after_minutes": 480, "mode": "hourly_rate"|"multiplier",
                   "rate": 15000, "multiplier": 1.5 }
bonus_kehadiran  { "max_absent": 0, "max_late": 2, "amount": 200000 }
bonus_target     { "basis": "outlet_sales"|"own_sales", "threshold": 50000000,
                   "mode": "percent"|"amount", "percent": 0.01, "amount": 0 }
potongan_kasbon  { }                     // nominal diambil dari employee_advances
komponen_manual  { }                     // nominal diisi saat penggajian
```

```sql
CREATE TABLE payroll_periods (
  id CHAR(26) PRIMARY KEY, tenant_id CHAR(26) NOT NULL,
  outlet_id CHAR(26),                    -- NULL = seluruh outlet
  period_type TEXT NOT NULL CHECK (period_type IN ('daily','weekly','biweekly','monthly')),
  start_date DATE NOT NULL, end_date DATE NOT NULL, pay_date DATE,
  status TEXT NOT NULL DEFAULT 'draft'
    CHECK (status IN ('draft','calculated','locked','paid','canceled')),
  total_gross BIGINT NOT NULL DEFAULT 0,
  total_deduction BIGINT NOT NULL DEFAULT 0,
  total_net BIGINT NOT NULL DEFAULT 0,
  calculated_at TIMESTAMPTZ,
  locked_at TIMESTAMPTZ, locked_by CHAR(26),
  paid_at TIMESTAMPTZ,
  created_by CHAR(26) NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id),
  CHECK (end_date >= start_date)
);
CREATE UNIQUE INDEX uq_payroll_periods
  ON payroll_periods (tenant_id, COALESCE(outlet_id,''), period_type, start_date);

CREATE TABLE payslips (
  id CHAR(26) PRIMARY KEY, tenant_id CHAR(26) NOT NULL,
  payroll_period_id CHAR(26) NOT NULL, employee_id CHAR(26) NOT NULL,
  gross_amount BIGINT NOT NULL DEFAULT 0,
  deduction_amount BIGINT NOT NULL DEFAULT 0,
  net_amount BIGINT NOT NULL DEFAULT 0 CHECK (net_amount >= 0),  -- tidak pernah minus
  carried_debt BIGINT NOT NULL DEFAULT 0,   -- potongan yang tidak tertutup, terbawa periode berikutnya
  present_days NUMERIC(5,1) NOT NULL DEFAULT 0,
  late_count INT NOT NULL DEFAULT 0,
  absent_days NUMERIC(5,1) NOT NULL DEFAULT 0,
  leave_days NUMERIC(5,1) NOT NULL DEFAULT 0,
  overtime_minutes INT NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','locked','paid')),
  paid_at TIMESTAMPTZ, payment_method TEXT, note TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id),
  UNIQUE (tenant_id, payroll_period_id, employee_id),
  FOREIGN KEY (tenant_id, payroll_period_id) REFERENCES payroll_periods (tenant_id, id) ON DELETE CASCADE,
  FOREIGN KEY (tenant_id, employee_id)       REFERENCES employees       (tenant_id, id) ON DELETE RESTRICT
);

CREATE TABLE payslip_lines (
  id CHAR(26) PRIMARY KEY, tenant_id CHAR(26) NOT NULL, payslip_id CHAR(26) NOT NULL,
  rule_id CHAR(26),                      -- NULL untuk komponen manual & penyesuaian
  name TEXT NOT NULL,                    -- SNAPSHOT nama komponen
  category TEXT NOT NULL CHECK (category IN ('earning','deduction')),
  rule_type TEXT,                        -- SNAPSHOT tipe aturan
  params_snapshot JSONB,                 -- SNAPSHOT parameter saat dihitung
  basis_note TEXT,                       -- "3 kali telat x Rp10.000" — dibaca karyawan
  quantity NUMERIC(14,3),
  amount BIGINT NOT NULL,
  sort_order INT NOT NULL DEFAULT 0,
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, payslip_id) REFERENCES payslips     (tenant_id, id) ON DELETE CASCADE,
  FOREIGN KEY (tenant_id, rule_id)    REFERENCES payroll_rules(tenant_id, id) ON DELETE SET NULL
);
CREATE INDEX idx_payslip_lines_payslip ON payslip_lines (tenant_id, payslip_id);

CREATE TABLE payroll_adjustments (       -- koreksi terlambat: muncul di periode BERIKUTNYA
  id CHAR(26) PRIMARY KEY, tenant_id CHAR(26) NOT NULL, employee_id CHAR(26) NOT NULL,
  origin_period_id CHAR(26) NOT NULL,    -- periode yang salah
  target_period_id CHAR(26),             -- periode tempat penyesuaian dibebankan; NULL = periode berikutnya
  name TEXT NOT NULL,
  category TEXT NOT NULL CHECK (category IN ('earning','deduction')),
  amount BIGINT NOT NULL CHECK (amount > 0),
  reason TEXT NOT NULL,
  applied_payslip_id CHAR(26),           -- diisi saat sudah terpakai
  created_by CHAR(26) NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, employee_id)      REFERENCES employees       (tenant_id, id) ON DELETE CASCADE,
  FOREIGN KEY (tenant_id, origin_period_id) REFERENCES payroll_periods (tenant_id, id) ON DELETE RESTRICT
);
CREATE INDEX idx_payroll_adj_pending ON payroll_adjustments (tenant_id, employee_id)
  WHERE applied_payslip_id IS NULL;

CREATE TABLE employee_advances (         -- kasbon
  id CHAR(26) PRIMARY KEY, tenant_id CHAR(26) NOT NULL, employee_id CHAR(26) NOT NULL,
  amount BIGINT NOT NULL CHECK (amount > 0),
  remaining BIGINT NOT NULL CHECK (remaining >= 0),
  installment_amount BIGINT NOT NULL CHECK (installment_amount > 0),
  reason TEXT,
  status TEXT NOT NULL DEFAULT 'pending'
    CHECK (status IN ('pending','approved','disbursed','settled','canceled')),
  approved_by CHAR(26), approved_at TIMESTAMPTZ, disbursed_at TIMESTAMPTZ,
  cash_movement_id CHAR(26),             -- pencairan sebagai kas keluar
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id),
  CHECK (remaining <= amount),
  FOREIGN KEY (tenant_id, employee_id)      REFERENCES employees      (tenant_id, id) ON DELETE RESTRICT,
  FOREIGN KEY (tenant_id, cash_movement_id) REFERENCES cash_movements (tenant_id, id) ON DELETE SET NULL
);
CREATE INDEX idx_advances_open ON employee_advances (tenant_id, employee_id)
  WHERE status = 'disbursed' AND remaining > 0;

CREATE TABLE advance_repayments (
  id CHAR(26) PRIMARY KEY, tenant_id CHAR(26) NOT NULL,
  advance_id CHAR(26) NOT NULL,
  payslip_id CHAR(26),                   -- NULL bila dibayar tunai di luar gaji
  amount BIGINT NOT NULL CHECK (amount > 0),
  paid_at TIMESTAMPTZ NOT NULL, business_date DATE NOT NULL,
  UNIQUE (tenant_id, id),
  UNIQUE (tenant_id, advance_id, payslip_id),   -- satu slip memotong satu kasbon sekali
  FOREIGN KEY (tenant_id, advance_id) REFERENCES employee_advances (tenant_id, id) ON DELETE RESTRICT,
  FOREIGN KEY (tenant_id, payslip_id) REFERENCES payslips          (tenant_id, id) ON DELETE RESTRICT
);
```

Penyesuaian tabel yang sudah ada:

```sql
ALTER TABLE cash_movements
  ADD COLUMN ref_table TEXT CHECK (ref_table IN ('payroll_periods','employee_advances')),
  ADD COLUMN ref_id CHAR(26);            -- pembayaran gaji & pencairan kasbon masuk arus kas
```

Izin baru untuk seeder `permissions`:

```
hr.employee.view   hr.employee.edit
hr.attendance.view hr.attendance.correct
hr.leave.request   hr.leave.approve
hr.payroll.run     hr.payroll.lock      hr.payroll.pay
hr.salary.view     hr.advance.approve
```

> **`hr.salary.view` adalah izin paling sensitif di seluruh produk.** Secara bawaan hanya peran Owner
> yang memilikinya. Karyawan melihat slipnya sendiri lewat endpoint terpisah yang menyaring dengan
> `employee_id` miliknya, bukan lewat izin ini.

### 5.12 Program mitra penjual (Bagian G blueprint) — lingkup platform

```sql
CREATE TABLE partner_tiers (
  id CHAR(26) PRIMARY KEY,
  name TEXT NOT NULL UNIQUE,             -- 'Afiliasi', 'Agen', 'Agen Utama'
  kind TEXT NOT NULL CHECK (kind IN ('affiliate','agent')),
  recurring_rate NUMERIC(7,4) NOT NULL,  -- 0.1500 = 15%
  recurring_months INT,                  -- NULL = selama merchant aktif
  activation_bonus BIGINT NOT NULL DEFAULT 0,
  min_active_merchants INT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE partners (
  id CHAR(26) PRIMARY KEY,
  tier_id CHAR(26) NOT NULL REFERENCES partner_tiers(id) ON DELETE RESTRICT,
  kind TEXT NOT NULL CHECK (kind IN ('affiliate','agent')),
  name TEXT NOT NULL, phone TEXT NOT NULL, email TEXT,
  region TEXT,
  referral_code TEXT NOT NULL UNIQUE,    -- dipakai saat merchant mendaftar
  id_number TEXT, npwp TEXT,             -- untuk bukti potong pajak
  bank_name TEXT, bank_account_no TEXT, bank_account_name TEXT,
  status TEXT NOT NULL DEFAULT 'pending'
         CHECK (status IN ('pending','verified','active','suspended','terminated')),
  verified_at TIMESTAMPTZ, joined_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE partner_users (             -- jalur autentikasi TERPISAH dari users tenant
  id CHAR(26) PRIMARY KEY,
  partner_id CHAR(26) NOT NULL REFERENCES partners(id) ON DELETE CASCADE,
  name TEXT NOT NULL, email TEXT NOT NULL, phone TEXT,
  password_hash TEXT NOT NULL,
  is_active BOOLEAN NOT NULL DEFAULT true, last_login_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX uq_partner_users_email ON partner_users (email) WHERE deleted_at IS NULL;

CREATE TABLE partner_leads (
  id CHAR(26) PRIMARY KEY,
  partner_id CHAR(26) NOT NULL REFERENCES partners(id) ON DELETE CASCADE,
  business_name TEXT NOT NULL, contact_name TEXT, phone TEXT NOT NULL,
  city TEXT, business_type TEXT, note TEXT,
  status TEXT NOT NULL DEFAULT 'new'
         CHECK (status IN ('new','contacted','demo','registered','activated','lost')),
  attribution_expires_at TIMESTAMPTZ NOT NULL,   -- masa atribusi, default +60 hari
  converted_tenant_id CHAR(26) REFERENCES tenants(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_partner_leads_phone ON partner_leads (phone);

CREATE TABLE partner_referrals (         -- kaitan mitra ↔ tenant, dasar seluruh komisi
  id CHAR(26) PRIMARY KEY,
  partner_id CHAR(26) NOT NULL REFERENCES partners(id) ON DELETE RESTRICT,
  tenant_id  CHAR(26) NOT NULL REFERENCES tenants(id)  ON DELETE RESTRICT,
  lead_id CHAR(26) REFERENCES partner_leads(id) ON DELETE SET NULL,
  referral_code TEXT NOT NULL,
  attributed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  activated_at TIMESTAMPTZ,              -- saat ambang aktivasi terpenuhi
  commission_starts_at TIMESTAMPTZ,
  commission_ends_at TIMESTAMPTZ,        -- untuk tier berjangka
  status TEXT NOT NULL DEFAULT 'pending'
         CHECK (status IN ('pending','active','ended','disputed','revoked')),
  UNIQUE (tenant_id)                     -- satu tenant hanya boleh diatribusikan ke satu mitra
);

CREATE TABLE partner_commissions (
  id CHAR(26) PRIMARY KEY,
  partner_id CHAR(26) NOT NULL REFERENCES partners(id) ON DELETE RESTRICT,
  referral_id CHAR(26) NOT NULL REFERENCES partner_referrals(id) ON DELETE RESTRICT,
  subscription_invoice_id CHAR(26) NOT NULL REFERENCES subscription_invoices(id) ON DELETE RESTRICT,
  period_month DATE NOT NULL,            -- bulan yang dikomisikan (tanggal 1)
  base_amount BIGINT NOT NULL,           -- nilai yang BENAR-BENAR diterima, setelah diskon
  rate NUMERIC(7,4) NOT NULL,
  amount BIGINT NOT NULL,
  status TEXT NOT NULL DEFAULT 'held'
         CHECK (status IN ('held','approved','paid','clawed_back','canceled')),
  payout_id CHAR(26),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (referral_id, subscription_invoice_id, period_month)   -- idempoten: tidak bisa dobel
);
CREATE INDEX idx_partner_commissions_partner_status ON partner_commissions (partner_id, status);

CREATE TABLE partner_payouts (
  id CHAR(26) PRIMARY KEY,
  partner_id CHAR(26) NOT NULL REFERENCES partners(id) ON DELETE RESTRICT,
  period_start DATE NOT NULL, period_end DATE NOT NULL,
  gross_amount BIGINT NOT NULL, tax_amount BIGINT NOT NULL DEFAULT 0,
  clawback_amount BIGINT NOT NULL DEFAULT 0,
  net_amount BIGINT NOT NULL,
  status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','approved','paid','failed')),
  paid_at TIMESTAMPTZ, transfer_proof_url TEXT, tax_slip_url TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (partner_id, period_start)
);
ALTER TABLE partner_commissions
  ADD CONSTRAINT fk_partner_commissions_payout
  FOREIGN KEY (payout_id) REFERENCES partner_payouts(id) ON DELETE SET NULL;

CREATE TABLE partner_targets (
  id CHAR(26) PRIMARY KEY,
  partner_id CHAR(26) NOT NULL REFERENCES partners(id) ON DELETE CASCADE,
  period_start DATE NOT NULL, period_end DATE NOT NULL,
  target_merchants INT NOT NULL DEFAULT 0, achieved_merchants INT NOT NULL DEFAULT 0,
  UNIQUE (partner_id, period_start)
);

CREATE TABLE partner_materials (
  id CHAR(26) PRIMARY KEY,
  title TEXT NOT NULL, kind TEXT NOT NULL CHECK (kind IN ('brochure','video','template','pricelist')),
  file_url TEXT NOT NULL, version INT NOT NULL DEFAULT 1,
  min_tier_id CHAR(26) REFERENCES partner_tiers(id) ON DELETE SET NULL,
  is_active BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE partner_trainings (
  id CHAR(26) PRIMARY KEY, title TEXT NOT NULL, content_url TEXT,
  is_required BOOLEAN NOT NULL DEFAULT false, sort_order INT NOT NULL DEFAULT 0
);

CREATE TABLE partner_training_records (
  id CHAR(26) PRIMARY KEY,
  partner_user_id CHAR(26) NOT NULL REFERENCES partner_users(id) ON DELETE CASCADE,
  training_id CHAR(26) NOT NULL REFERENCES partner_trainings(id) ON DELETE CASCADE,
  completed_at TIMESTAMPTZ, score INT,
  UNIQUE (partner_user_id, training_id)
);

CREATE TABLE partner_disputes (
  id CHAR(26) PRIMARY KEY,
  claimant_partner_id CHAR(26) NOT NULL REFERENCES partners(id) ON DELETE CASCADE,
  tenant_id CHAR(26) REFERENCES tenants(id) ON DELETE SET NULL,
  lead_id CHAR(26) REFERENCES partner_leads(id) ON DELETE SET NULL,
  reason TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open','accepted','rejected')),
  decided_by CHAR(26), decided_at TIMESTAMPTZ, decision_note TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

> **Batas satu tingkat dikunci di skema, bukan hanya di perjanjian.** Tidak ada kolom `upline_id` atau
> `parent_partner_id` di tabel `partners`. Skema berjenjang menjadi tidak mungkin dibangun tanpa migrasi
> yang terlihat jelas di tinjauan kode.

### 5.13 Langganan, tagihan, dan pendapatan diterima di muka

```sql
CREATE TABLE plans (
  id CHAR(26) PRIMARY KEY,
  code TEXT NOT NULL UNIQUE,             -- 'free','basic','pro','multi'
  name TEXT NOT NULL,
  monthly_price BIGINT NOT NULL,
  max_outlets INT, max_users INT, max_products INT, max_monthly_transactions INT,
  features JSONB NOT NULL DEFAULT '{}',  -- {"qris":true,"crm_freelance":false}
  is_active BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE plan_term_discounts (       -- tangga diskon prabayar 3/6/9/12 bulan
  id CHAR(26) PRIMARY KEY,
  term_months INT NOT NULL CHECK (term_months IN (1,3,6,9,12)),
  discount_rate NUMERIC(7,4) NOT NULL,   -- 0.0500, 0.1000, 0.1250, 0.1670
  is_active BOOLEAN NOT NULL DEFAULT true,
  UNIQUE (term_months)
);

CREATE TABLE subscriptions (
  id CHAR(26) PRIMARY KEY,
  tenant_id CHAR(26) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
  plan_id CHAR(26) NOT NULL REFERENCES plans(id) ON DELETE RESTRICT,
  term_months INT NOT NULL DEFAULT 1 CHECK (term_months IN (1,3,6,9,12)),
  discount_rate NUMERIC(7,4) NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT 'trial'
         CHECK (status IN ('trial','active','past_due','canceled','expired')),
  trial_ends_at TIMESTAMPTZ,
  current_period_start TIMESTAMPTZ NOT NULL,
  current_period_end   TIMESTAMPTZ NOT NULL,
  auto_renew BOOLEAN NOT NULL DEFAULT true,
  canceled_at TIMESTAMPTZ, cancel_reason TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id)                     -- satu langganan aktif per tenant
);

CREATE TABLE subscription_addons (       -- CRM Freelance, CRM Sales, Kanal Online
  id CHAR(26) PRIMARY KEY,
  subscription_id CHAR(26) NOT NULL REFERENCES subscriptions(id) ON DELETE CASCADE,
  addon_code TEXT NOT NULL CHECK (addon_code IN ('crm_freelance','crm_sales','online_channel')),
  quantity INT NOT NULL DEFAULT 1,       -- per pengguna sales / per kanal aktif
  unit_price BIGINT NOT NULL,
  started_at TIMESTAMPTZ NOT NULL, ended_at TIMESTAMPTZ,
  UNIQUE (subscription_id, addon_code)
);

CREATE TABLE subscription_invoices (
  id CHAR(26) PRIMARY KEY,
  tenant_id CHAR(26) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
  subscription_id CHAR(26) NOT NULL REFERENCES subscriptions(id) ON DELETE RESTRICT,
  number TEXT NOT NULL UNIQUE,
  term_months INT NOT NULL,
  period_start DATE NOT NULL, period_end DATE NOT NULL,
  gross_amount BIGINT NOT NULL,          -- sebelum diskon
  discount_amount BIGINT NOT NULL DEFAULT 0,
  total_amount BIGINT NOT NULL,          -- yang ditagihkan
  paid_amount BIGINT NOT NULL DEFAULT 0, -- dasar komisi mitra
  due_date DATE NOT NULL,
  status TEXT NOT NULL DEFAULT 'open'
         CHECK (status IN ('open','paid','overdue','void','refunded')),
  paid_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_sub_invoices_status_due ON subscription_invoices (status, due_date);

CREATE TABLE subscription_payments (
  id CHAR(26) PRIMARY KEY,
  subscription_invoice_id CHAR(26) NOT NULL REFERENCES subscription_invoices(id) ON DELETE RESTRICT,
  amount BIGINT NOT NULL CHECK (amount > 0),
  method TEXT NOT NULL, reference TEXT, gateway_fee BIGINT NOT NULL DEFAULT 0,
  paid_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE deferred_revenue_entries (  -- uang di muka BUKAN pendapatan bulan itu
  id CHAR(26) PRIMARY KEY,
  subscription_invoice_id CHAR(26) NOT NULL REFERENCES subscription_invoices(id) ON DELETE CASCADE,
  tenant_id CHAR(26) NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
  recognition_month DATE NOT NULL,       -- tanggal 1 bulan pengakuan
  amount BIGINT NOT NULL,
  recognized_at TIMESTAMPTZ,             -- NULL = belum diakui
  UNIQUE (subscription_invoice_id, recognition_month)
);
CREATE INDEX idx_deferred_pending ON deferred_revenue_entries (recognition_month) WHERE recognized_at IS NULL;
```

### 5.14 Tabel sistem

```sql
CREATE TABLE idempotency_keys (
  id CHAR(26) PRIMARY KEY,
  tenant_id CHAR(26),
  scope TEXT NOT NULL,                   -- 'sale.create', 'stock.adjust'
  key TEXT NOT NULL,
  request_hash TEXT NOT NULL,            -- hash body; kunci sama + body beda = 409
  response_status INT, response_body JSONB,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  expires_at TIMESTAMPTZ NOT NULL        -- dibersihkan setelah 7 hari
);
-- UNIQUE dengan ekspresi harus berupa index, bukan constraint di dalam CREATE TABLE
CREATE UNIQUE INDEX uq_idempotency_scope_key
  ON idempotency_keys (COALESCE(tenant_id,''), scope, key);
CREATE INDEX idx_idempotency_expires ON idempotency_keys (expires_at);

CREATE TABLE audit_logs (
  id CHAR(26) PRIMARY KEY,
  tenant_id CHAR(26),                    -- NULL untuk aksi platform/mitra
  actor_type TEXT NOT NULL CHECK (actor_type IN ('user','partner_user','system','admin')),
  actor_id CHAR(26),
  action TEXT NOT NULL,                  -- 'sale.void', 'price.change', 'partner.commission.approve'
  target_table TEXT, target_id CHAR(26),
  before_data JSONB, after_data JSONB,
  ip_address INET, user_agent TEXT,
  occurred_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_audit_tenant_time ON audit_logs (tenant_id, occurred_at DESC);
CREATE INDEX idx_audit_target ON audit_logs (target_table, target_id);

CREATE TABLE outbox_events (             -- pengiriman andal: notifikasi, webhook, sinkronisasi kanal
  id CHAR(26) PRIMARY KEY,
  tenant_id CHAR(26),
  topic TEXT NOT NULL,                   -- 'invoice.due', 'sale.completed', 'stock.sync'
  payload JSONB NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending'
         CHECK (status IN ('pending','processing','done','failed','dead')),
  attempts INT NOT NULL DEFAULT 0, last_error TEXT,
  available_at TIMESTAMPTZ NOT NULL DEFAULT now(),   -- untuk penundaan bertahap
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(), processed_at TIMESTAMPTZ
);
CREATE INDEX idx_outbox_ready ON outbox_events (status, available_at) WHERE status IN ('pending','failed');

CREATE TABLE daily_sales_summaries (     -- agregat laporan; dashboard tidak pernah SUM tabel penuh
  tenant_id CHAR(26) NOT NULL,
  outlet_id CHAR(26) NOT NULL,
  business_date DATE NOT NULL,
  channel_id CHAR(26) NOT NULL DEFAULT '',
  sales_count INT NOT NULL DEFAULT 0,
  gross_amount BIGINT NOT NULL DEFAULT 0,
  discount_amount BIGINT NOT NULL DEFAULT 0,
  tax_amount BIGINT NOT NULL DEFAULT 0,
  net_amount BIGINT NOT NULL DEFAULT 0,
  cost_amount BIGINT NOT NULL DEFAULT 0,
  fee_amount BIGINT NOT NULL DEFAULT 0,  -- MDR + komisi kanal
  gross_profit BIGINT NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, outlet_id, business_date, channel_id)
);

CREATE TABLE notification_templates (
  id CHAR(26) PRIMARY KEY, tenant_id CHAR(26),   -- NULL = template bawaan sistem
  code TEXT NOT NULL, channel TEXT NOT NULL CHECK (channel IN ('whatsapp','email')),
  subject TEXT, body TEXT NOT NULL
);
CREATE UNIQUE INDEX uq_notif_templates
  ON notification_templates (COALESCE(tenant_id,''), code, channel);
```

### 5.15 Aturan index & constraint yang berlaku umum

1. **Setiap index pada tabel bertenant dimulai dari `tenant_id`.** Index yang tidak diawali `tenant_id`
   akan memindai baris tenant lain sebelum menyaring.
2. **Setiap FK punya index di sisi anak.** PostgreSQL tidak membuatnya otomatis, dan tanpa itu
   `ON DELETE RESTRICT` memindai seluruh tabel.
3. **Kolom status yang dipakai untuk antrean** memakai partial index (`WHERE status = 'pending'`) —
   jauh lebih kecil dan tetap tepat sasaran.
4. **Tabel besar dipartisi per bulan** setelah melewati sekitar 10 juta baris: `sales`,
   `stock_movements`, `audit_logs`, `channel_events`. Rancang nama partisi sejak awal walau belum
   dipasang.
5. **`JSONB` hanya untuk muatan pihak luar** (`payload`, `raw_payload`) dan pengaturan bebas
   (`features`). Data yang dicari atau dijumlahkan harus jadi kolom.

### 5.16 Constraint menyusul & urutan pembuatan

Beberapa relasi menunjuk tabel yang lahir di fase berikutnya — `sales` sudah ada di Fase 3, sedangkan
`channels` baru di Fase 11. Relasi seperti itu **tetap dipasang**, hanya waktunya belakangan lewat
`ALTER TABLE`, supaya tidak ada relasi yang hilang diam-diam.

| Constraint | Dipasang di fase | Alasan ditunda |
|---|---|---|
| `sales.shift_id → shifts` | 3 | `shifts` dibuat dalam migrasi yang sama, setelah `sales` |
| `sales.customer_id → customers` | 3 | idem |
| `sales.table_id → dining_tables` | 3 | hanya relevan untuk F&B |
| `sale_items.variant_id → product_variants` | 3 | |
| `stock_movements.variant_id → product_variants` | 4 | |
| `price_lists.channel_id → channels` | 11a | `channels` belum ada sebelum Fase 11 |
| `sales.channel_id → channels` | 11a | idem |
| `tenants.referred_by_partner_id → partners` | 12 | `partners` belum ada sebelum Fase 12 |
| `partner_commissions.subscription_invoice_id` | 12 | `subscription_invoices` sudah ada sejak Fase 7 |
| `cash_movements.ref_table/ref_id` → `payroll_periods`, `employee_advances` | 13 | Polimorfik, dijaga service + `CHECK` |
| `employee_advances.cash_movement_id → cash_movements` | 13 | `cash_movements` sudah ada sejak Fase 3 |

```sql
-- contoh, di migrasi Fase 11a
ALTER TABLE sales
  ADD CONSTRAINT fk_sales_channel
  FOREIGN KEY (tenant_id, channel_id) REFERENCES channels (tenant_id, id) ON DELETE RESTRICT;
ALTER TABLE price_lists
  ADD CONSTRAINT fk_price_lists_channel
  FOREIGN KEY (tenant_id, channel_id) REFERENCES channels (tenant_id, id) ON DELETE SET NULL;
```

**Satu-satunya relasi yang sengaja TIDAK berupa foreign key:**

| Kolom | Kenapa |
|---|---|
| `stocks.variant_id` | Memakai `''` (bukan NULL) agar primary key komposit tetap sederhana dan `ON CONFLICT` bekerja. FK ke `''` mustahil, jadi keutuhannya dijaga service |
| `stock_movements.ref_table` / `ref_id` | Referensi polimorfik ke `sales`/`purchases`/`stock_opnames`. Dijaga service, dan setiap `kind` menentukan `ref_table` yang sah |
| `audit_logs.target_table` / `target_id` | Idem — log harus tetap ada walau baris rujukannya dipartisi atau diarsipkan |
| `receivables.source_table` / `source_id` | Idem, dengan `CHECK` yang membatasi nilainya |

Referensi polimorfik dibatasi lewat `CHECK` pada kolom penunjuk tabelnya, dan diuji di test service —
bukan dibiarkan bebas.

### 5.17 Dua lingkup, dua aturan relasi

Tidak semua tabel yang memuat `tenant_id` adalah tabel bertenant.

| Lingkup | Contoh | Aturan relasi |
|---|---|---|
| **Bertenant** — data operasional milik satu usaha | `sales`, `products`, `deals`, `channels` | Wajib `UNIQUE (tenant_id, id)`, dan semua FK antar tabel ini **komposit** |
| **Platform** — data milik kita, sebagian menunjuk tenant | `subscriptions`, `subscription_invoices`, `deferred_revenue_entries`, seluruh `partner_*` | FK biasa (tunggal). `tenant_id` di sini adalah **penunjuk pelanggan**, bukan pembatas akses |

Tabel platform tidak memakai FK komposit karena pemakainya adalah kita, bukan tenant, dan tidak pernah
disentuh lewat `scopeTenant`. Aksesnya dibatasi peran admin platform dan `partner_merchant_view`.

Tabel penghubung murni (`user_outlets`, `role_permissions`, `partner_training_records`) tidak punya
kolom `id` sama sekali — primary key-nya adalah pasangan kunci asing, dan tidak ada yang merujuknya.

### 5.18 Ringkasan jumlah tabel

| Domain | Tabel baru | Fase |
|---|---|---|
| Platform & tenancy | 2 | 1 |
| Identitas & otorisasi | 4 *(+`users`, `roles` yang sudah ada, di-ALTER)* | 0–1 |
| Master data | 10 | 2 |
| Transaksi kasir | 3 | 3 |
| Kas & shift | 2 | 3 |
| Stok | 8 | 4 |
| Pelanggan & piutang | 4 | 3–4 |
| CRM tenant | 17 | 9–10 |
| Kanal online | 7 | 11 |
| **Absensi & penggajian** | **15** | **13** |
| Program mitra | 12 | 12 |
| Langganan & tagihan | 7 | 7 |
| Sistem | 5 | 0–5 |
| **Total** | **96 tabel baru** + 2 tabel lama | |

---

## 6. Isolasi tenant

Tiga lapisan, masing-masing menutup celah yang lolos dari lapisan sebelumnya.

### Lapisan 1 — scope wajib di repository

```go
// repositories/helper.go
func scopeTenant(ctx context.Context, db *gorm.DB) *gorm.DB {
    tid := middlewares.TenantIDFromContext(ctx)   // panic bila kosong: bug, bukan kondisi normal
    return db.Where("tenant_id = ?", tid)
}
```

**Aturan:** tidak ada query ke tabel bertenant tanpa melewati `scopeTenant`. Tegakkan dengan pemeriksaan
statis sederhana di CI:

```sh
# gagal bila ada Find/First/Take pada tabel bertenant tanpa scopeTenant di berkas yang sama
grep -rn "DB\.\(Model\|Table\|Find\|First\)" repositories/ | grep -v "scopeTenant" && exit 1
```

### Lapisan 2 — Row Level Security

Jaring pengaman bila satu query lolos dari lapisan 1.

```sql
ALTER TABLE sales ENABLE ROW LEVEL SECURITY;
ALTER TABLE sales FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON sales
  USING (tenant_id = current_setting('app.tenant_id', true));
-- ulangi untuk seluruh tabel bertenant
```

Nilainya dipasang **di dalam transaksi** agar aman dengan connection pool:

```go
func WithTenant(ctx context.Context, db *gorm.DB, fn func(tx *gorm.DB) error) error {
    return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
        tid := middlewares.TenantIDFromContext(ctx)
        if err := tx.Exec("SET LOCAL app.tenant_id = ?", tid).Error; err != nil {
            return err
        }
        return fn(tx)
    })
}
```

> **Catatan PgBouncer:** `SET LOCAL` aman pada mode transaction pooling karena hidup hanya selama
> transaksi. Jangan pernah memakai `SET` biasa — nilainya akan menempel di koneksi dan bocor ke tenant
> berikutnya yang memakai koneksi itu.

### Lapisan 3 — visibilitas kepemilikan (CRM)

Di dalam satu tenant, sales hanya boleh melihat datanya sendiri:

```go
func scopeVisibility(ctx context.Context, db *gorm.DB, table string) *gorm.DB {
    if middlewares.HasPermission(ctx, "crm.lead.view.all") {
        return db
    }
    return db.Where(table+".owner_id = ?", middlewares.UserIDFromContext(ctx))
}
```

Berlaku untuk `customers`, `deals`, `activities`, `visits`, `quotations`, `invoices`.

### Lapisan mitra

Akun `partner_users` **bukan** user tenant. Mereka tidak pernah mendapat `tenant_id` di token, dan
endpoint tenant menolaknya. Yang boleh dibaca mitra tentang merchant binaannya hanya tiga kolom —
disajikan lewat view khusus, bukan akses tabel langsung:

```sql
CREATE VIEW partner_merchant_view AS
SELECT r.partner_id, t.id AS tenant_id, t.business_name, s.status AS subscription_status,
       s.current_period_end AS due_date,
       (SELECT max(business_date) FROM daily_sales_summaries d WHERE d.tenant_id = t.id) AS last_active_date
FROM partner_referrals r
JOIN tenants t ON t.id = r.tenant_id
LEFT JOIN subscriptions s ON s.tenant_id = t.id;
```

Omzet, produk, harga, pelanggan, dan isi transaksi **tidak pernah** masuk ke view ini.

---

## 7. Lapisan aplikasi

### Alur permintaan

```
HTTP → middleware (auth → tenant → permission → rate limit)
     → controller  (bind, validasi, panggil service, bentuk response)
     → service     (aturan bisnis, buka transaksi, panggil beberapa repository)
     → repository  (satu tabel, selalu menerima ctx dan tx opsional)
     → PostgreSQL
```

### Aturan transaksi

- **Service yang membuka transaksi**, bukan repository, bukan controller.
- Satu permintaan HTTP = paling banyak satu transaksi database.
- Panggilan ke pihak luar (WhatsApp, kanal, gateway) **tidak boleh** ada di dalam transaksi. Tulis ke
  `outbox_events` di dalam transaksi, kirim di luar oleh pekerja latar.
- Urutan penguncian baris ditetapkan global untuk mencegah deadlock: **`stocks` dikunci menurut
  `product_id` menaik**.

### Bentuk error

```go
// helpers/errors.go
var (
    ErrNotFound      = errors.New("data tidak ditemukan")
    ErrConflict      = errors.New("data bentrok")
    ErrInsufficient  = errors.New("stok tidak mencukupi")
    ErrForbidden     = errors.New("tidak punya akses")
    ErrValidation    = errors.New("input tidak valid")
)
```

Service mengembalikan error sentinel; controller memetakannya ke kode HTTP. **Detail error internal
tidak pernah dikirim ke klien** — sesuai CONVENTIONS bagian 4; yang dikirim adalah pesan yang sudah
disiapkan, sedangkan detailnya masuk log bersama request ID.

---

## 8. Rancangan API

### Konvensi

| Hal | Aturan |
|---|---|
| Prefiks | `/api/v1` — versi naik hanya bila ada perubahan yang merusak |
| Penamaan | Jamak, kebab-case: `/sales`, `/stock-movements`, `/partner-payouts` |
| Waktu | RFC 3339 dengan offset di request dan response |
| Uang | Bilangan bulat rupiah, bukan string, bukan desimal |
| Kuantitas | String desimal (`"0.250"`) agar tidak rusak oleh float JavaScript |
| Paginasi daftar | Gaya Laravel yang sudah ada (`page`, `per_page`) |
| Paginasi sinkronisasi | Kursor ULID (`after_id`, `limit`) — memanfaatkan keterurutan ULID |

### Bentuk response baku

```jsonc
// sukses
{ "success": true, "message": "OK", "data": { } }

// gagal validasi
{ "success": false, "message": "Input tidak valid",
  "errors": { "qty": ["harus lebih besar dari 0"] } }
```

### Idempotensi

Wajib pada semua endpoint yang menciptakan uang atau memindahkan stok:
`POST /sales`, `POST /sales/:id/refund`, `POST /stock-adjustments`, `POST /purchases`,
`POST /invoice-payments`, `POST /subscription-payments`.

```
Idempotency-Key: 01J9Z8Y7X6W5V4U3T2S1R0Q9P8
```

Perilaku server:

1. Cari `(scope, key)` di `idempotency_keys`.
2. Ditemukan **dan** `request_hash` sama → kembalikan response tersimpan, jangan kerjakan ulang.
3. Ditemukan **tapi** `request_hash` beda → `409 Conflict`. Kunci yang sama tidak boleh dipakai untuk
   dua permintaan berbeda.
4. Tidak ditemukan → kerjakan di dalam transaksi, simpan response, kembalikan.

### Peta endpoint utama

```
POST   /api/v1/auth/register           daftar tenant + outlet + owner + peran bawaan (1 transaksi)
POST   /api/v1/auth/login              → access token (15 mnt) + refresh token (30 hari)
POST   /api/v1/auth/refresh            tukar refresh token, rotasi
POST   /api/v1/auth/logout             cabut refresh token
POST   /api/v1/auth/pin-login          ganti kasir cepat di perangkat yang sama

GET    /api/v1/outlets                 CRUD outlet, termasuk timezone & business_day_start
GET    /api/v1/products?q=&category_id=&page=
POST   /api/v1/products/import         impor CSV/Excel, pratinjau + laporan baris gagal

POST   /api/v1/sales                   checkout (idempoten) — lihat 13.1
GET    /api/v1/sales?business_date=&outlet_id=&status=
POST   /api/v1/sales/:id/void          batal + alasan (permission sale.void)
POST   /api/v1/sales/:id/refund        retur sebagian/penuh

POST   /api/v1/shifts/open             buka kas
POST   /api/v1/shifts/:id/close        tutup kas + hitung fisik + selisih
POST   /api/v1/cash-movements          kas masuk/keluar non-penjualan

GET    /api/v1/stocks?outlet_id=&low=true
POST   /api/v1/stock-adjustments       penyesuaian + alasan wajib
GET    /api/v1/stock-movements?product_id=   kartu stok
POST   /api/v1/purchases               stok masuk
POST   /api/v1/stock-opnames/:id/post  posting hasil hitung fisik

GET    /api/v1/reports/dashboard?outlet_id=&date=      dari daily_sales_summaries
GET    /api/v1/reports/sales?from=&to=&group_by=day|hour|channel|cashier|payment
       # group_by=hour: jam dinding DI ZONA OUTLET (key "00".."23"), bukan UTC.
       #   occurred_at disimpan UTC (§3.2) dan Indonesia di UTC+7..+9, jadi
       #   dibaca mentah penjualan 07.00 WIB tercatat 00.00 dan grafik "jam
       #   teramai" menunjuk tengah malam. Zona diambil per-outlet lewat join,
       #   karena satu tenant boleh punya cabang di zona berbeda.
GET    /api/v1/reports/profit?from=&to=
GET    /api/v1/reports/export?type=&format=csv|xlsx|pdf

POST   /api/v1/sync/push               kiriman batch dari perangkat offline
GET    /api/v1/sync/pull?since=&outlet_id=

POST   /api/v1/channels/:id/orders/:oid/accept   terima pesanan kanal
POST   /webhooks/channels/:provider              masuk ke channel_events, tanpa proses

# jalur mitra — autentikasi terpisah, tanpa tenant_id
POST   /api/v1/partner/auth/login
GET    /api/v1/partner/dashboard
POST   /api/v1/partner/leads
GET    /api/v1/partner/merchants        hanya status langganan & aktivitas, lihat 6
GET    /api/v1/partner/commissions?period=
GET    /api/v1/partner/payouts
```

---

## 9. Autentikasi & otorisasi

### Token

| Token | Umur | Isi |
|---|---|---|
| Access | 15 menit | `sub` (user id), `typ`, `jti`. **Tidak ada** `tenant_id` atau daftar permission |
| Refresh | 30 hari, dirotasi | Disimpan sebagai hash di `refresh_tokens` |

**`tenant_id` dan permission diambil dari database setiap permintaan**, bukan dari token — sesuai
CONVENTIONS bagian 4 ("JWT hanya membawa user ID"). Konsekuensinya satu query per permintaan; cache
5 detik di memori proses bila terbukti membebani, dengan invalidasi saat peran berubah.

Rotasi refresh token: setiap penukaran mencabut token lama dan menerbitkan yang baru. Bila token yang
sudah dicabut dipakai lagi, **cabut seluruh sesi user itu** — itu tanda token dicuri.

### Middleware berantai

```go
api := r.Group("/api/v1")
protected := api.Group("", middlewares.Auth())               // token → user
tenantScoped := protected.Group("", middlewares.TenantScope()) // user → tenant_id ke context
tenantScoped.POST("/sales", middlewares.Require("sale.create"), controllers.CreateSale)
```

`middlewares.Require(codes ...string)` memeriksa permission efektif user. Untuk sumber daya per outlet,
tambahkan `middlewares.RequireOutletAccess()` yang memeriksa `user_outlets`.

### PIN kasir

PIN 6 digit di-hash bcrypt, **hanya berlaku** bila perangkat sudah punya sesi tenant yang sah — PIN
menukar sesi antar user di dalam tenant yang sama, bukan menggantikan login. Batasi 5 percobaan salah
per 15 menit per outlet.

---

## 10. Sinkronisasi offline

### Prinsip

| Jenis data | Arah | Penyelesaian konflik |
|---|---|---|
| Penjualan, kunjungan, gerakan stok dari penjualan | Klien → server, hanya tambah | Tidak ada konflik; idempoten lewat ULID + `idempotency_key` |
| Master data (produk, harga, pelanggan) | Server → klien | Server berwenang |
| Saldo stok | Server → klien | Server berwenang; angka di klien adalah perkiraan |

### Penanda kemajuan: `sync_version`

Setiap tabel yang disinkronkan punya `sync_version BIGINT` yang diisi dari satu sequence global pada
setiap insert/update, lewat trigger:

```sql
CREATE OR REPLACE FUNCTION bump_sync_version() RETURNS trigger AS $$
BEGIN
  NEW.sync_version := nextval('sync_version_seq');
  RETURN NEW;
END $$ LANGUAGE plpgsql;

CREATE TRIGGER trg_products_sync BEFORE INSERT OR UPDATE ON products
  FOR EACH ROW EXECUTE FUNCTION bump_sync_version();
```

Kenapa bukan `updated_at`: jam bisa mundur dan dua penulisan dalam milidetik yang sama tidak bisa
dibedakan. Sequence selalu naik.

> **Keterbatasan yang harus diakui:** transaksi yang mengambil nomor lebih kecil bisa *commit* setelah
> transaksi bernomor lebih besar, sehingga satu baris bisa terlewat bila klien langsung memakai nilai
> tertinggi. Penawarnya sederhana: klien menyimpan kursor **dikurangi jeda aman** (misal 1.000 nomor
> atau 30 detik) dan menerapkan hasil tarikan secara idempoten. Menarik ulang beberapa baris jauh lebih
> murah daripada kehilangan satu.

### Kontrak endpoint

```jsonc
// POST /api/v1/sync/push
{
  "device_id": "01J9...",
  "operations": [
    { "op": "sale.create", "id": "01J9...", "idempotency_key": "01J9...",
      "payload": { "outlet_id": "01J8...", "client_created_at": "2026-09-07T21:14:03+09:00",
                   "items": [ ], "payments": [ ] } }
  ]
}

// response — per operasi, tidak pernah gagal seluruhnya karena satu operasi buruk
{ "success": true, "data": { "results": [
    { "id": "01J9...", "status": "applied" },
    { "id": "01J9...", "status": "duplicate" },
    { "id": "01J9...", "status": "rejected", "reason": "produk tidak ditemukan" }
]}}
```

```jsonc
// GET /api/v1/sync/pull?since=184023&outlet_id=01J8...&limit=500
{ "success": true, "data": {
    "cursor": 184530,
    "products":  [ ], "product_prices": [ ], "customers": [ ],
    "stocks":    [ ],
    "deleted":   { "products": ["01J7..."] }        // batu nisan, agar klien ikut menghapus
}}
```

### Aturan penerapan

1. Operasi yang ditolak **tidak** menghentikan operasi lain dalam satu kiriman.
2. Penjualan offline yang membuat stok minus **diterima**, lalu ditandai untuk ditinjau pemilik.
   Menolaknya berarti menyuruh kasir berhenti berjualan — lebih buruk daripada stok minus.
3. `business_date` dihitung **ulang di server** memakai zona outlet, bukan diambil dari klien. Jam
   perangkat tidak dipercaya.
4. Klien menampilkan jumlah operasi yang belum terkirim, dan memberi peringatan setelah 24 jam atau
   200 transaksi.

---

## 11. Pekerjaan latar & outbox

Semua efek samping yang menyentuh dunia luar melewati `outbox_events`, ditulis **di dalam transaksi
bisnis yang sama**. Ini yang membuat "penjualan tercatat tapi notifikasi hilang" tidak mungkin terjadi.

```go
// jobs/outbox_worker.go — pola dasar
rows := SELECT * FROM outbox_events
        WHERE status IN ('pending','failed') AND available_at <= now()
        ORDER BY available_at LIMIT 100
        FOR UPDATE SKIP LOCKED;          // aman dijalankan banyak instance
```

- **Backoff bertahap:** `available_at = now() + (2^attempts) menit`, maksimal 6 jam.
- **Dead letter:** setelah 10 percobaan → `status='dead'`, muncul di panel admin untuk ditinjau manusia.
- **Kegagalan permanen langsung `dead`,** tanpa menunggu jatah percobaan habis: nomor tujuan
  tidak terdaftar di WhatsApp, alamat cacat, template tidak ada. Mencoba ulang tidak mengubah
  hasilnya, dan menundanya sepuluh kali hanya membuat operator baru tahu berjam-jam kemudian —
  saat ia sudah lupa tagihan mana yang bermasalah. Pengirim menandainya dengan
  `services.ErrNotifPermanen`.
- **`FOR UPDATE SKIP LOCKED`** membuat beberapa pekerja bisa berjalan tanpa saling menunggu.

### Daftar pekerjaan terjadwal

| Pekerjaan | Jadwal | Catatan |
|---|---|---|
| Pengiriman outbox | tiap 10 detik | |
| Pemrosesan `channel_events` | tiap 10 detik | |
| Sinkronisasi stok ke kanal | tiap 1 menit | dari `channel_stock_syncs` |
| Penyegaran `daily_sales_summaries` | tiap 5 menit + saat tulis | |
| Pengingat jatuh tempo invoice | harian **per zona outlet** | H-3, H, H+3, H+7 |
| Rekap harian ke supervisor | sore **per zona outlet** | |
| Pengakuan pendapatan diterima di muka | harian, awal bulan | lihat 13.4 |
| Perhitungan komisi mitra | harian | lihat 13.3 |
| Rekonsiliasi `stocks` dari `stock_movements` | mingguan + manual | |
| Pembangunan ulang `attendance_days` | harian **per zona outlet**, setelah hari usaha tutup | sumbernya `attendances`, sama seperti rekonsiliasi stok |
| Pengingat periode gaji jatuh tempo | harian | ke pemilik, H-2 sebelum `pay_date` |
| Pembersihan `idempotency_keys` kedaluwarsa | harian | |

> **Pekerjaan berjadwal lokal wajib memperhitungkan zona outlet.** "Kirim rekap jam 5 sore" berarti
> tiga waktu UTC berbeda untuk WIB, WITA, dan WIT. Penjadwal mengelompokkan outlet menurut `timezone`,
> lalu menghitung waktu UTC berikutnya untuk tiap kelompok.

---

## 12. Integrasi eksternal

Semua integrasi memakai antarmuka seragam supaya menambah kanal berarti menambah satu implementasi,
bukan mengubah inti.

```go
// adapters/channel/adapter.go
type Adapter interface {
    FetchOrders(ctx context.Context, ch Channel, since time.Time) ([]ExternalOrder, error)
    AcceptOrder(ctx context.Context, ch Channel, externalID string) error
    RejectOrder(ctx context.Context, ch Channel, externalID, reason string) error
    PushStatus(ctx context.Context, ch Channel, externalID, status string) error
    SyncStock(ctx context.Context, ch Channel, items []StockItem) error
    SetItemAvailability(ctx context.Context, ch Channel, sku string, available bool) error
}
```

Implementasi awal: `ManualAdapter` (entri tangan) dan `CSVAdapter` (impor laporan harian) — keduanya
**tidak bergantung pada persetujuan pihak mana pun** dan sudah cukup untuk menyelesaikan keluhan
"laporan saya pecah". Adaptor API dipasang belakangan seiring kemitraan disetujui.

Aturan untuk semua adaptor:

1. Timeout keras (10 detik) dan pemutus sirkuit per kanal.
2. Batas laju dihormati; token disegarkan otomatis.
3. Kredensial dienkripsi di aplikasi sebelum masuk `channels.credentials_encrypted`.
4. Kegagalan satu kanal **tidak boleh** merambat — pekerja per kanal terpisah.

---

## 13. Algoritma bisnis kritis

### 13.1 Checkout

Satu transaksi database, tidak boleh dipecah. Ini implementasi dari urutan yang ditetapkan di
[Bagian D blueprint](BLUEPRINT-SAAS-POS.md#bagian-d--model-data-inti).

```go
func (s *SaleService) Checkout(ctx context.Context, req SaleCreateRequest) (*SaleResponse, error) {
    // 0. Di luar transaksi: validasi bentuk, ULID, dan hak akses outlet.
    outlet, err := s.outlets.Get(ctx, req.OutletID)          // butuh timezone & business_day_start
    now := time.Now().UTC()
    bizDate, err := timez.BusinessDate(now, outlet.Timezone, outlet.BusinessDayStart)

    var resp *SaleResponse
    err = WithTenant(ctx, s.db, func(tx *gorm.DB) error {
        // 1. Idempotensi — kunci yang sama mengembalikan hasil lama
        if prev, ok := s.idem.Lookup(ctx, tx, "sale.create", req.IdempotencyKey, req.Hash()); ok {
            resp = prev; return nil
        }

        // 2. Kunci baris stok, URUT product_id MENAIK (mencegah deadlock)
        ids := sortedProductIDs(req.Items)
        stocks, err := s.stocks.LockForUpdate(ctx, tx, req.OutletID, ids)

        // 3. Ambil produk + harga berlaku (per pelanggan / per kanal), buat SNAPSHOT
        //    Harga TIDAK diambil dari request klien — klien hanya mengirim product_id & qty.
        items := s.pricing.Resolve(ctx, tx, req)             // isi product_name, unit_price, unit_cost

        // 4. Hitung per baris lalu jumlahkan (jangan hitung total dari persentase agregat)
        totals := calcTotals(items, req.Discount, outlet.TaxRate, outlet.TaxInclusive)

        // 5. Nomor struk per outlet, diambil dengan penguncian baris outlet
        receiptNo := s.numbering.Next(ctx, tx, req.OutletID, bizDate)

        // 6. Tulis sales + sale_items + sale_payments
        // 7. Tulis stock_movements untuk tiap item ber-track_stock (dan bahan baku via resep)
        //    balance_after dihitung dari stok terkunci di langkah 2
        // 8. Perbarui stocks (cache)
        // 9. Perbarui shift berjalan
        //10. Perbarui daily_sales_summaries (UPSERT)
        //11. audit_logs + outbox_events ('sale.completed')
        //12. Simpan hasil ke idempotency_keys

        return nil
    })
    return resp, err
}
```

**Yang tidak boleh ada di dalam transaksi ini:** cetak struk, kirim WhatsApp, panggilan ke kanal atau
gateway. Semuanya lewat `outbox_events`.

**Stok minus:** bila `balance_after < 0`, transaksi **tetap dilanjutkan** dan ditandai untuk ditinjau.
Kasir tidak boleh dihentikan karena data stok yang tidak sinkron.

### 13.2 Void & retur

```
void   : status → 'voided', catat voided_by/voided_at/void_reason
         stock_movements kind='void' dengan qty_delta berlawanan (JANGAN hapus baris asli)
         daily_sales_summaries dikurangi, shift disesuaikan, audit_logs ditulis
retur  : sale baru bertipe retur yang menunjuk sale asal, atau retur sebagian per item
         stock_movements kind='refund'
```

Baris asli tidak pernah dihapus atau diubah nilainya. Pembalikan selalu berupa baris baru — itu yang
membuat kartu stok dan audit tetap bisa dipercaya.

### 13.3 Komisi mitra

Dijalankan harian, idempoten berkat `UNIQUE (referral_id, subscription_invoice_id, period_month)`.

```
untuk setiap partner_referrals berstatus 'active':
  1. Syarat aktivasi: tenant sudah mencapai ambang (mis. ≥30 transaksi ATAU ≥30 hari aktif)?
     belum → lewati, jangan buat baris komisi
  2. Ambil subscription_invoices yang paid_at-nya di dalam periode & paid_amount > 0
     → DASAR komisi adalah paid_amount (yang benar-benar diterima, setelah diskon)
  3. Untuk pembayaran di muka N bulan: pecah menjadi N baris partner_commissions,
     satu per period_month — komisi dibayar bertahap meski uang diterima sekaligus
  4. rate diambil dari partner_tiers; hormati recurring_months bila berjangka
  5. status awal 'held'; berubah 'approved' setelah tinjauan admin
  6. Bila invoice di-refund/void → baris terkait menjadi 'clawed_back',
     nilainya mengurangi partner_payouts periode berikutnya
```

### 13.4 Pengakuan pendapatan diterima di muka

```
Saat subscription_invoice berstatus 'paid' untuk term_months = N:
  buat N baris deferred_revenue_entries, masing-masing total_amount/N
  (sisa pembulatan ditaruh di bulan terakhir agar jumlahnya persis)

Pekerjaan harian:
  UPDATE deferred_revenue_entries SET recognized_at = now()
  WHERE recognition_month <= date_trunc('month', now()) AND recognized_at IS NULL;
```

Laporan pendapatan platform membaca baris yang **sudah diakui**, bukan `subscription_payments`. Tanpa
disiplin ini, bulan penjualan terlihat sangat untung dan bulan-bulan berikutnya terlihat rugi.

### 13.5 Laba bersih per kanal

```sql
SELECT s.channel_id,
       SUM(s.total)                                   AS omzet,
       SUM(s.cost_total)                              AS modal,
       COALESCE(SUM(f.amount), 0)                     AS biaya_kanal,
       SUM(s.total) - SUM(s.cost_total) - COALESCE(SUM(f.amount),0) AS laba_bersih
FROM sales s
LEFT JOIN (SELECT tenant_id, sale_id, SUM(amount) amount FROM channel_fees GROUP BY 1,2) f
  ON f.tenant_id = s.tenant_id AND f.sale_id = s.id
WHERE s.tenant_id = $1 AND s.business_date BETWEEN $2 AND $3 AND s.status = 'completed'
GROUP BY s.channel_id;
```

### 13.6 Penggajian

Dijalankan sebagai satu proses per periode. Yang membuatnya akurat bukan rumusnya, melainkan
**urutan yang tetap, snapshot di setiap baris, dan penguncian.**

```go
func (s *PayrollService) Calculate(ctx context.Context, periodID string) error {
    return WithTenant(ctx, s.db, func(tx *gorm.DB) error {
        p := s.periods.Lock(ctx, tx, periodID)         // SELECT ... FOR UPDATE
        if p.Status == "locked" || p.Status == "paid" {
            return ErrConflict                          // periode terkunci tidak pernah dihitung ulang
        }

        // 1. Bangun ulang attendance_days untuk seluruh karyawan pada rentang periode.
        //    Urutan status TETAP: libur > cuti/izin/sakit disetujui > ada absensi > alpa
        s.attendance.RebuildDays(ctx, tx, p.TenantID, p.OutletID, p.StartDate, p.EndDate)

        for _, emp := range s.employees.ActiveIn(ctx, tx, p) {
            days := s.attendance.Days(ctx, tx, emp.ID, p.StartDate, p.EndDate)

            // 2. Pendapatan dasar
            //    bulanan → base_wage penuh (prorata bila masuk/keluar di tengah periode)
            //    harian  → base_wage x hari hadir
            lines := []Line{ basicWage(emp, days, p) }

            // 3. Aturan yang BERLAKU pada periode ini, urut kategori lalu priority
            rules := s.rules.Effective(ctx, tx, emp, p.StartDate, p.EndDate)
            for _, r := range earningsOf(rules) { lines = append(lines, apply(r, emp, days, p)) }
            for _, r := range deductionsOf(rules) { lines = append(lines, apply(r, emp, days, p)) }

            // 4. Penyesuaian dari periode sebelumnya yang jatuh ke periode ini
            lines = append(lines, s.adjustments.PendingFor(ctx, tx, emp.ID, p.ID)...)

            // 5. Cicilan kasbon berjalan
            lines = append(lines, s.advances.InstallmentFor(ctx, tx, emp.ID)...)

            // 6. Jumlahkan: bulatkan PER BARIS lalu jumlahkan (aturan uang di 3.3)
            gross, deduction := sumByCategory(lines)
            net, carried := gross-deduction, int64(0)
            if net < 0 { carried, net = -net, 0 }       // slip tidak pernah minus

            // 7. Tulis payslip + payslip_lines dengan SNAPSHOT nama, tipe, dan params tiap aturan
            s.payslips.Upsert(ctx, tx, emp, p, lines, gross, deduction, net, carried)
        }
        return s.periods.MarkCalculated(ctx, tx, p)
    })
}
```

**Penguncian** mengubah status periode dan seluruh slipnya menjadi `locked`, mencatat `locked_by` dan
`locked_at`, lalu menulis `audit_logs`. Setelah itu:

- Menghitung ulang periode terkunci **ditolak** (`ErrConflict`).
- Koreksi absensi yang disetujui setelah penguncian tidak mengubah slip lama — ia menghasilkan baris
  `payroll_adjustments` dengan `origin_period_id` periode lama.
- Pembayaran menulis `cash_movements` keluar dengan `ref_table='payroll_periods'`, memperbarui
  `advance_repayments`, dan menurunkan `employee_advances.remaining`.

**Sifat yang wajib diuji:** menghitung periode yang sama dua kali dari data masukan yang sama harus
menghasilkan angka yang identik sampai rupiah terakhir — termasuk urutan dan isi `payslip_lines`.

### 13.7 Penomoran struk

Format `<KODE_OUTLET>-<YYMMDD>-<URUT>`, dengan `YYMMDD` dari **`business_date`**, bukan tanggal UTC.
Nomor urut diambil dengan `SELECT ... FOR UPDATE` pada baris penghitung per outlet per hari — bukan
`COUNT(*)`, yang akan menghasilkan nomor ganda saat dua kasir menutup transaksi bersamaan.

---

## 14. Pengujian

| Lapisan | Yang diuji | Alat |
|---|---|---|
| Helper murni | pembulatan uang, `BusinessDate`, validasi ULID, `escapeLike` | test tabel biasa |
| Repository | query, filter, paginasi, partial unique index | PostgreSQL sungguhan lewat `dockertest` |
| Service | checkout, void, komisi, pengakuan pendapatan | transaksi rollback per test |
| HTTP | otorisasi, bentuk response, idempotensi | `httptest` |
| **Isolasi** | kebocoran tenant, kebocoran pemilik, akses mitra | **wajib, jalan di CI** |
| Konkurensi | dua checkout produk sama, dua tutup shift | `-race` + goroutine |

**Test yang tidak boleh absen:**

```go
func TestTenantIsolation(t *testing.T)   // tenant A tidak bisa membaca/mengubah data tenant B
                                         // di SETIAP endpoint, termasuk dengan menebak ID di URL
func TestOwnerVisibility(t *testing.T)   // sales X tidak melihat pelanggan sales Y
func TestPartnerCannotReadTenantData(t *testing.T)
func TestCheckoutIdempotent(t *testing.T)      // kirim dua kali → satu penjualan
func TestConcurrentCheckoutSameProduct(t *testing.T)  // stok akhir tepat, tanpa deadlock
func TestBusinessDateAcrossTimezones(t *testing.T)    // WIB/WITA/WIT + batas hari 04:00
func TestMoneyRoundingSumsExactly(t *testing.T)
func TestPayrollDeterministic(t *testing.T)      // hitung dua kali → angka & baris identik
func TestPayrollNeverNegative(t *testing.T)      // potongan > pendapatan → net 0 + carried_debt
func TestLockedPayrollImmutable(t *testing.T)    // hitung ulang periode terkunci → ErrConflict
func TestLateCorrectionBecomesAdjustment(t *testing.T)  // koreksi setelah kunci → periode berikutnya
func TestAttendanceShiftCrossesMidnight(t *testing.T)   // masuk 22:00, pulang 06:00
```

Contoh yang wajib lulus untuk zona waktu:

```go
// Outlet Jayapura (Asia/Jayapura, UTC+9), batas hari 00:00
// Transaksi 2026-09-07T07:00:00+09:00 = 2026-09-06T22:00:00Z
// business_date HARUS 2026-09-07, bukan 2026-09-06
```

Perintah verifikasi sebelum commit, melengkapi CONVENTIONS bagian 8:

```sh
go build ./... && go vet ./... && gofmt -l . && go test ./... -race
```

---

## 15. Observabilitas & konfigurasi

### Log & metrik

- Log terstruktur JSON dengan `request_id`, `tenant_id`, `user_id`, `route`, `latency_ms`.
- **Jangan pernah** menulis token, password, PIN, kredensial kanal, atau isi `credentials_encrypted`.
- Metrik minimum: laju permintaan & error per rute, durasi checkout p95, kedalaman `outbox_events`
  pending, umur `channel_events` tertua, jumlah operasi sinkronisasi tertunda per perangkat.
- Peringatan: error 5xx naik, outbox `dead` > 0, sinkronisasi stok kanal terlambat > 15 menit,
  pekerjaan terjadwal gagal.

### Konfigurasi baru di `.env.example`

```sh
# --- Waktu ---
DB_TZ=UTC                       # WAJIB UTC (sebelumnya Asia/Jakarta — ubah sekarang)
DEFAULT_TIMEZONE=Asia/Jakarta   # default untuk outlet baru saja

# --- Token ---
JWT_ACCESS_MINUTES=15
JWT_REFRESH_DAYS=30

# --- Idempotensi & sinkronisasi ---
IDEMPOTENCY_TTL_DAYS=7
SYNC_PULL_MAX_LIMIT=1000
SYNC_CURSOR_SAFETY_LAG=1000     # jeda aman nomor sync_version

# --- Pekerja latar ---
OUTBOX_WORKER_COUNT=2
OUTBOX_MAX_ATTEMPTS=10
CHANNEL_WORKER_TIMEOUT_SECONDS=10

# --- Enkripsi kredensial kanal ---
CREDENTIALS_ENCRYPTION_KEY=     # 32 byte hex, WAJIB di produksi

# --- Redis (rate limit lintas instance & cache permission) ---
REDIS_URL=
```

---

## 16. Urutan implementasi bertahap

Setiap tahap menghasilkan sesuatu yang bisa dijalankan dan diuji. Nomor fase mengikuti
[roadmap blueprint](BLUEPRINT-SAAS-POS.md#bagian-i--roadmap-bertahap).

| Fase | Keluaran teknis | Migrasi | Definisi selesai |
|---|---|---|---|
| **0** | `golang-migrate` terpasang, `AutoMigrate` mati, `DB_TZ=UTC`, `/health`, log terstruktur, refresh token, `internal/ulid`, `internal/timez`, rate limit Redis, soft delete + partial unique index | `000001`–`000002` | Migrasi naik-turun bersih; test `helpers` & `config` hijau |
| **1** | `tenants`, `outlets` (timezone!), `tenant_id` di `users`/`roles`, `permissions`, `scopeTenant`, RLS, `Require()`, pendaftaran tenant dalam satu transaksi | `000003`–`000004` | `TestTenantIsolation` lulus di semua endpoint |
| **2** | Master data lengkap + impor CSV + index trigram pencarian | `000005`–`000006` | 500 produk terimpor, dicari < 200 ms |
| **3** | `sales`, `sale_items`, `sale_payments`, `shifts`, `cash_movements`, `idempotency_keys`, `SaleService.Checkout`, penomoran struk, void/retur | `000007`–`000009` | 100 transaksi berurutan: stok, kas, laporan cocok sampai rupiah terakhir |
| **4** | `stock_movements` + `stocks` + rekonsiliasi, `purchases`, opname, transfer, resep | `000010`–`000012` | Hitung ulang `stocks` dari `stock_movements` menghasilkan angka sama persis |
| **5** | `daily_sales_summaries` + endpoint laporan + ekspor | `000013` | Dashboard < 1 detik pada 100.000 transaksi |
| **6** | `sync_version` + trigger, `/sync/push`, `/sync/pull`, batu nisan | `000014` | Perangkat berjualan 8 jam tanpa internet lalu tersinkron tanpa duplikat |
| **7** | `plans`, `plan_term_discounts`, `subscriptions`, `subscription_invoices`, `deferred_revenue_entries`, prorata & pembatalan | `000015`–`000016` | Daftar → trial → bayar 12 bulan → batal di tengah: angka pengembalian sama dengan hitungan tangan |
| **9** | CRM: pipeline, deals, activities, quotations, invoices, projects, `scopeVisibility` | `000017`–`000019` | Siklus prospek → penawaran → DP → pelunasan muncul di laporan omzet yang sama dengan POS |
| **10** | `visits`, `visit_plans`, `sales_targets`, `commissions`, antrean foto & GPS | `000020`–`000021` | Rute 20 toko tanpa sinyal tersinkron utuh |
| **11a** | `channels`, `channel_products`, harga per kanal, status pesanan diperluas, entri manual + CSV | `000022`–`000023` | Laba bersih per kanal keluar benar tanpa satu pun API kanal |
| **11b** | `channel_events`, pekerja, adaptor API, `channel_fees`, `channel_settlements`, `channel_stock_syncs` | `000024`–`000025` | Pesanan dikirim dua kali oleh kanal → tetap satu penjualan |
| **12** | `partners` dkk, portal mitra, mesin komisi, pencairan, `partner_merchant_view` | `000026`–`000028` | Siklus komisi berjalan tanpa hitungan manual; akun mitra tidak bisa menyentuh data operasional tenant |
| **13** | `employees`, `work_schedules`, `attendances` + koreksi, `attendance_days`, izin & libur, `payroll_rules`, siklus penggajian + penguncian, kasbon, izin `hr.*` | `000029`–`000031` | Satu periode dihitung, dikunci, dibayar; hitung ulang dari data yang sama menghasilkan angka identik; koreksi terlambat muncul sebagai penyesuaian periode berikutnya |

### Tiga langkah pertama, konkret

1. **Migrasi & waktu.** Pasang `golang-migrate`, pindahkan skema `users`/`roles` ke `000001`, matikan
   `AutoMigrate` di [database/database.go](../database/database.go), ubah `DB_TZ` menjadi `UTC`, buat
   `internal/timez` beserta testnya. Belum ada tabel bisnis — jadi belum ada data yang perlu dipindah.
2. **Tenancy.** `tenants` + `outlets` (dengan `timezone` dan `business_day_start`), `tenant_id` pada
   `users`/`roles`, `scopeTenant`, RLS, middleware tenant.
3. **Test isolasi.** `TestTenantIsolation` yang membuat dua tenant dan menembak seluruh rute. Test ini
   yang akan menjaga sepuluh fase berikutnya.

---

*Dokumen hidup — perbarui bersama skema. Pasangannya: [BLUEPRINT-SAAS-POS.md](BLUEPRINT-SAAS-POS.md)
untuk produk, [BUSINESS-MODEL-CANVAS.md](BUSINESS-MODEL-CANVAS.md) untuk model bisnis,
[CONVENTIONS.md](../CONVENTIONS.md) untuk gaya kode.*
