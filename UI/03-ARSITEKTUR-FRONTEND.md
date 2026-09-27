# 03 — Arsitektur Frontend

---

## Pilihan teknologi, beserta alasannya

| Lapisan | Pilihan | Kenapa ini |
|---|---|---|
| Bahasa | **TypeScript** | Bentuk data dari backend banyak dan berlapis. Salah nama field ketahuan saat menulis kode, bukan saat kasir sedang melayani pembeli |
| Kerangka | **React 18** | Kolam talenta terbesar di Indonesia — pemilik proyek solo harus bisa mencari penerus dengan mudah |
| Build | **Vite** | Start dev < 1 detik, konfigurasi hampir nol. Tanpa ini, iterasi UI jadi lambat |
| Styling | **Tailwind CSS** | Token desain (warna/jarak) hidup di satu berkas config, bukan tersebar di ratusan file CSS |
| Komponen | **shadcn/ui** (di atas Radix) | **Kodenya disalin ke repo**, bukan dependensi. Bisa diubah bebas tanpa melawan library. Radix menangani aksesibilitas (fokus, papan tik, pembaca layar) yang mahal kalau dibuat sendiri |
| Data server | **TanStack Query** | Cache, coba-ulang, dan status "sedang mengirim" sudah jadi. Sangat cocok untuk sinyal jelek |
| Rute | **React Router v6** | Sederhana, stabil, dokumentasinya melimpah |
| Form | **React Hook Form + Zod** | Validasi di klien memakai skema yang sama bentuknya dengan kontrak backend |
| Offline | **Dexie (IndexedDB)** | Antrean transaksi offline butuh penyimpanan berstruktur, bukan localStorage |
| PWA | **vite-plugin-pwa** | Bisa dipasang di layar depan HP/tablet, jalan tanpa internet |
| Grafik | **Recharts** | Cukup untuk grafik batang/garis laporan; ringan |
| Uang | **dinero.js** atau bilangan bulat sendiri | **Wajib** — lihat bagian "Uang" di bawah |
| Tes | **Vitest + Testing Library + Playwright** | Unit + alur kritis (kasir, tutup shift) |

### Yang dipertimbangkan lalu ditolak

| Alternatif | Kenapa tidak |
|---|---|
| **Next.js** | Kekuatannya di SSR/SEO. Aplikasi ini di balik login dan **wajib jalan offline** — SSR justru menambah rumit tanpa manfaat. SPA + PWA lebih tepat |
| **Vue / Svelte** | Bagus secara teknis, tapi kolam talenta di Indonesia jauh lebih kecil. Menyulitkan regenerasi pengembang |
| **MUI / Ant Design** | Tampilannya khas dan sulit dilepas. Menyesuaikan warna & rasa "hangat" jadi perang melawan library |
| **Redux Toolkit** | 90% state di sini adalah data server. TanStack Query sudah menanganinya; Redux jadi lapisan tambahan tanpa hasil |
| **Aplikasi native (Flutter/RN)** | Menambah jalur rilis dan tim. PWA sudah cukup: bisa dipasang, offline, akses kamera untuk barcode. Native dipertimbangkan lagi kalau butuh cetak struk Bluetooth yang dalam |

---

## Struktur folder

Dipisah **per fitur**, bukan per jenis berkas — supaya menambah modul tidak
berarti menyentuh sepuluh folder berbeda.

```
src/
├─ app/
│  ├─ router.tsx              # definisi rute + penjaga izin
│  ├─ providers.tsx           # QueryClient, tema, toast
│  └─ layouts/                # LayoutTenant, LayoutKasir, LayoutMitra
├─ fitur/                     # satu folder = satu modul backend
│  ├─ kasir/
│  │  ├─ api.ts               # pemanggilan endpoint modul ini
│  │  ├─ hooks.ts             # useCheckout, useShiftAktif
│  │  ├─ komponen/
│  │  └─ halaman/
│  ├─ produk/
│  ├─ stok/
│  ├─ laporan/
│  ├─ pelanggan/
│  ├─ crm/
│  ├─ kanal/
│  ├─ sdm/                    # absensi & gaji
│  ├─ langganan/
│  ├─ pengaturan/             # outlet, pengguna, peran
│  └─ mitra/                  # portal mitra (realm terpisah)
├─ bersama/
│  ├─ ui/                     # komponen shadcn (hasil salin)
│  ├─ komponen/               # KartuAngka, KeadaanKosong, LencanaStatus
│  ├─ hooks/                  # useIzin, useOnline, useUang
│  └─ util/                   # format uang, tanggal, desimal
├─ lib/
│  ├─ api-client.ts           # fetch + token + penerjemah error
│  ├─ offline/                # skema Dexie, antrean, mesin sinkronisasi
│  └─ izin.ts                 # konstanta kode izin
└─ styles/
   └─ tokens.css              # variabel warna (terang & gelap)
```

**Aturan:** modul di `fitur/` tidak boleh saling mengimpor. Kalau butuh berbagi,
naikkan ke `bersama/`. Ini yang menjaga proyek tetap mudah dikembangkan saat besar.

---

## Kontrak dengan backend

### Bentuk balasan

Backend selalu mengembalikan amplop yang seragam:

```jsonc
// berhasil
{ "success": true, "message": "…", "data": { … } }

// berhasil, berpaginasi
{ "success": true, "message": "…",
  "data": { "current_page": 1, "data": [ … ], "last_page": 5, "total": 87, … } }

// gagal
{ "success": false, "message": "Validasi gagal",
  "errors": { "sell_price": "wajib diisi" } }
```

`api-client.ts` membuka amplop ini **satu kali**, sehingga komponen hanya
berurusan dengan `data` — tidak ada `res.data.data.data` bertebaran.

### Peta kode status → perilaku UI

Ini yang membuat pesan error terasa manusiawi.

| Kode | Arti backend | Yang dilakukan UI |
|---|---|---|
| **401** | Token mati / salah realm | Perbarui token diam-diam; kalau tetap gagal → ke halaman masuk. **Jangan** tampilkan "401" |
| **403** | Izin kurang | **Sembunyikan tombolnya sejak awal**. Kalau tetap tembus, tampilkan "Fitur ini tidak tersedia untuk akun Anda" |
| **404** | Tidak ditemukan / milik tenant lain | "Data tidak ditemukan." + tombol kembali |
| **409** | Bentrok (mis. sudah dibatalkan, kunci ganda) | Dialog penjelas + muat ulang data. **Bukan** toast merah sekilas |
| **422** | Validasi gagal, ada `errors` per field | Tempelkan pesan **di bawah kolom yang bersangkutan**, gulir ke kolom pertama yang salah |
| **429** | Terlalu sering | "Terlalu banyak percobaan. Coba lagi sebentar lagi." + hitung mundur |
| **5xx / jaringan** | Server/koneksi | Untuk aksi yang bisa diantre → simpan ke antrean offline. Selain itu → tombol "Coba lagi" |

### Autentikasi

- Masuk → `POST /api/v1/auth/login` → simpan `access_token` (15 menit) + `refresh_token` (30 hari).
- **Keduanya disimpan, lengkap dengan waktu kedaluwarsa access token.**

  > ⚠️ Rancangan awal menahan access token **di memori saja**. Setelah
  > dijalankan sungguhan, aturan itu dicabut karena dua alasan:
  >
  > 1. **Manfaat keamanannya nyaris nol.** Refresh token — kredensial yang jauh
  >    lebih berkuasa karena bisa mencetak access token berkali-kali — memang
  >    harus menetap di penyimpanan supaya kasir tidak diminta masuk ulang tiap
  >    pagi. Penyerang yang bisa membacanya sudah mendapat yang lebih besar.
  > 2. **Ongkosnya nyata.** Memori hilang tiap halaman dimuat ulang, jadi setiap
  >    muat ulang memaksa satu `/auth/refresh`. Endpoint itu dibatasi ~0,2
  >    permintaan/detik burst 5 **per alamat IP** (`middlewares.RateLimit`
  >    memakai `c.ClientIP()`). Satu warung dengan tablet kasir + HP pemilik +
  >    HP gudang berbagi satu IP: mereka saling menghabiskan jatah lalu
  >    terlempar ke layar masuk bersamaan.
  >
  > Pembaruan token kini hanya dijalankan saat token benar-benar kedaluwarsa
  > atau saat server membalas 401.
- Satu *interceptor* memperbarui token saat 401, **dan mengantre permintaan lain**
  supaya tidak terjadi lima kali refresh bersamaan.
- **Portal mitra adalah realm terpisah** (`POST /api/v1/partner/auth/login`, masuk
  dengan **email**). Token mitra ditolak di rute tenant dan sebaliknya — maka
  aplikasinya juga dipisah: `/mitra/*` punya layout, penyimpanan token, dan menu sendiri.

### Izin di UI

Backend mengembalikan izin efektif pengguna (gabungan seluruh perannya).

```ts
const { boleh } = useIzin();

{boleh('sale.void') && <TombolBatalkan />}
```

**Aturan: izin menyembunyikan, bukan menonaktifkan.** Tombol abu-abu yang tidak
bisa diklik membuat pengguna bertanya-tanya dan menelepon dukungan. Kalau tidak
berhak, tombolnya tidak ada.

Penjagaan izin juga dipasang di tingkat rute, sehingga URL yang diketik langsung
tetap tertolak — tapi ini hanya kenyamanan; **penegakan sesungguhnya tetap di backend**.

---

## Uang dan angka — bagian paling rawan

Backend menyimpan **uang sebagai bilangan bulat rupiah** (`int64`) dan
**jumlah barang sebagai string desimal** (agar 0,5 kg tetap presisi).

**Jangan pernah memakai bilangan pecahan JavaScript untuk uang.**

```ts
// ❌ SALAH — 0.1 + 0.2 = 0.30000000000000004
const total = harga * 0.1;

// ✅ BENAR — tetap bilangan bulat rupiah dari server sampai layar
const totalRp: number = data.total;          // 45000 (bilangan bulat)
formatRupiah(totalRp);                        // "Rp 45.000"

// ✅ Jumlah barang tetap string, diolah dengan decimal.js bila perlu
const qty: string = item.qty;                 // "1.5"
```

- **Frontend tidak mengarang aturan pembulatannya sendiri.** Backend
  membulatkan **per baris** (setengah ke atas) lalu menjumlahkan; aturan lain
  di frontend membuat total beda beberapa rupiah dari struk — dan itu
  menghancurkan kepercayaan.
- **Satu pengecualian yang dijaga kontrak: total kasir** (`bersama/util/total.ts`).
  Untuk QRIS dan kasbon, total itulah yang DIBAYAR PAS, dan saat offline tidak
  ada server yang bisa ditanya — jadi kasir menghitung subtotal, pajak
  (inklusif/eksklusif), biaya layanan, dan diskon **persis** seperti
  `services.priceCheckout`, memakai pengaturan cabang dari `GET /me`
  (`outlets[]`). Kesamaannya dijaga satu berkas kasus
  (`bersama/util/kasus-total.json`) yang diuji DUA sisi:
  `services/total_kasir_test.go` dan `bersama/util/total.test.ts`. Mengubah
  rumus di satu sisi tanpa sisi lain membuat salah satu tes gagal.
- Di luar itu frontend menghitung **hanya untuk pratinjau**, dan angka yang
  dicetak di struk **selalu diambil dari balasan server**.

## Waktu

- Server dan database seluruhnya UTC.
- **`business_date` dihitung server** berdasarkan zona waktu outlet dan jam
  tutup buku. Frontend **tidak pernah** menghitungnya sendiri — jam HP kasir
  sering salah.
- Untuk ditampilkan, waktu diubah ke zona outlet, bukan zona perangkat.

## Idempotency-Key

Aksi yang menciptakan uang atau stok **wajib** mengirim header ini:
`POST /sales`, `POST /purchases`, `POST /invoice-payments`, `POST /subscription-payment-claims`.

```ts
const kunci = ulid();            // dibuat SEKALI saat pengguna menekan Bayar
// kunci yang sama dipakai untuk semua percobaan ulang
```

Inilah yang membuat "tombol Bayar tertekan dua kali" atau "kirim ulang setelah
sinyal putus" tidak menghasilkan dua transaksi.

---

## Offline & sinkronisasi

Backend menyediakan `POST /sync/push` (operasi `sale.create`, `visit.upsert`) dan
`GET /sync/pull` (kategori, satuan, produk, varian, daftar harga, harga produk,
pelanggan, stok + batu nisan penghapusan).

### Cara kerja di sisi UI

```
Kasir menekan "Bayar"
        │
        ├─ Ada internet ──→ POST /sales (+ Idempotency-Key) ──→ struk tampil
        │
        └─ Tidak ada ────→ simpan ke antrean Dexie ──→ struk tampil "menunggu dikirim"
                                    │
                          internet kembali (atau tiap 30 dtk)
                                    │
                          POST /sync/push satu batch
                                    │
                     ┌──────────────┴──────────────┐
                  applied                      duplicate
              tandai terkirim            tandai terkirim (aman, bukan error)
```

Poin penting yang harus dipatuhi:

1. **Kasir tidak pernah diblokir** oleh status koneksi.
2. Satu operasi gagal **tidak menjatuhkan** seluruh batch — backend membalas
   per operasi; UI menandai yang gagal saja dan menampilkannya di satu tempat
   ("2 transaksi perlu diperiksa").
3. Balasan `duplicate` **bukan error** — artinya sudah pernah masuk. Tandai
   terkirim, jangan tampilkan apa pun ke pengguna.
4. Data master ditarik dengan kursor `sync_version`, disimpan di Dexie, dan
   dipakai kasir saat offline.
5. **Stok saat offline hanya perkiraan.** Beri label "perkiraan" di layar
   offline — server tetap pemegang kebenaran, dan stok minus diterima lalu
   ditandai untuk ditinjau (sesuai aturan backend).

---

## Kinerja

| Sasaran | Angka | Cara |
|---|---|---|
| Muat pertama di 3G | < 3 detik | Pecah kode per rute, pramuat rute kasir |
| Berpindah layar | < 200 ms | Data sudah di cache TanStack Query |
| Cari produk di kasir | < 100 ms | Cari di indeks Dexie lokal, bukan ke server tiap ketikan |
| Ukuran bundel awal | < 200 KB (terkompresi) | Grafik & modul berat dimuat saat dibutuhkan |

---

## Aksesibilitas

Bukan pelengkap — banyak pengguna berumur 40+ dan bekerja di bawah cahaya terik.

- Kontras **minimal 4.5:1** untuk teks (sudah diverifikasi di [02](02-SISTEM-DESAIN.md)).
- Seluruh aplikasi bisa dijalankan dengan papan tik (kasir sering pakai keyboard eksternal).
- Fokus selalu terlihat: cincin 2px `hijau-700`.
- Ukuran teks bisa diperbesar sampai 200% tanpa tata letak rusak.
- Label form terikat resmi ke kolomnya (`<label for>`).
- Menghormati `prefers-reduced-motion` dan `prefers-color-scheme`.
