# Blueprint SaaS POS untuk UMKM

Dokumen ini menjawab dua hal:

1. **Kebutuhan apa saja** yang harus dipenuhi sebuah POS agar bisa dipakai UMKM Indonesia — dari warung sampai grosir.
2. **Langkah pembangunannya**, berurut, dari kondisi repo saat ini sampai layak dijual sebagai SaaS.

Dokumen ini bersifat *product & engineering plan*. Aturan penulisan kode tetap mengacu ke
[CONVENTIONS.md](../CONVENTIONS.md) — blueprint ini menentukan **apa** yang dibangun, CONVENTIONS menentukan **bagaimana** menuliskannya.

**Kondisi awal (per dokumen ini dibuat):** backend Go + Gin + GORM + PostgreSQL dengan autentikasi JWT,
manajemen user & role, pagination, dan pengerasan keamanan dasar. Belum ada tenant, produk, maupun transaksi.

---

## Daftar Isi

- [Bagian A — Prinsip Produk](#bagian-a--prinsip-produk)
- [Bagian B — Kebutuhan UMKM](#bagian-b--kebutuhan-umkm)
  - [B.1 Kebutuhan universal (semua jenis usaha)](#b1-kebutuhan-universal-semua-jenis-usaha)
  - [B.2 Kebutuhan per jenis usaha](#b2-kebutuhan-per-jenis-usaha)
  - [B.3 Kebutuhan regulasi & konteks Indonesia](#b3-kebutuhan-regulasi--konteks-indonesia)
  - [B.4 Kebutuhan perangkat & lingkungan kerja](#b4-kebutuhan-perangkat--lingkungan-kerja)
  - [B.5 Yang sengaja TIDAK dikerjakan dulu](#b5-yang-sengaja-tidak-dikerjakan-dulu)
- [Bagian C — Keputusan Arsitektur](#bagian-c--keputusan-arsitektur)
- [Bagian D — Model Data Inti](#bagian-d--model-data-inti)
- [Bagian E — Modul CRM: Freelance & Sales Lapangan](#bagian-e--modul-crm-freelance--sales-lapangan)
- [Bagian F — Kanal Pesanan Online: Marketplace & Aplikator](#bagian-f--kanal-pesanan-online-marketplace--aplikator)
- [Bagian G — Program Mitra Penjual: Agen & Freelance Sales](#bagian-g--program-mitra-penjual-agen--freelance-sales)
- [Bagian H — Absensi & Penggajian Sederhana](#bagian-h--absensi--penggajian-sederhana)
- [Bagian I — Roadmap Bertahap](#bagian-i--roadmap-bertahap)
- [Bagian J — Kebutuhan Non-Fungsional](#bagian-j--kebutuhan-non-fungsional)
- [Bagian K — Model Bisnis SaaS](#bagian-k--model-bisnis-saas)
- [Bagian L — Operasional & Checklist Go-Live](#bagian-l--operasional--checklist-go-live)
- [Bagian M — Langkah Konkret Berikutnya di Repo Ini](#bagian-m--langkah-konkret-berikutnya-di-repo-ini)

---

## Bagian A — Prinsip Produk

Lima prinsip ini dipakai untuk memutuskan saat ada konflik fitur. Kalau sebuah usulan melanggar
salah satunya, usulan itu ditolak atau ditunda.

| Prinsip | Konsekuensi teknis |
|---|---|
| **Transaksi harus selesai < 10 detik** | Layar kasir satu halaman, tanpa navigasi bertingkat. Pencarian produk < 200 ms. Checkout satu tombol. |
| **Tidak boleh berhenti saat internet mati** | Client offline-first. Transaksi tersimpan lokal lalu disinkronkan. Server tidak boleh jadi titik gagal tunggal untuk berjualan. |
| **Kasir bukan akuntan** | Istilah sehari-hari ("Modal", "Untung", "Uang Masuk"), bukan istilah akuntansi ("HPP", "Debit/Kredit"). Laporan akuntansi disembunyikan di menu pemilik. |
| **Data pemilik adalah milik pemilik** | Ekspor Excel/CSV tersedia di semua paket, termasuk gratis. Tidak ada penyanderaan data. |
| **Murah untuk dijalankan** | Satu binary Go + PostgreSQL. Hindari dependensi berbayar sampai ada pendapatan. Biaya infrastruktur per tenant harus di bawah 10% harga langganan. |

**Target segmen awal (urut prioritas):**

1. **Retail kecil** — toko kelontong, toko bangunan kecil, apotek non-resep, toko baju. Kebutuhan paling standar, paling mudah dilayani.
2. **F&B kecil** — warung makan, kedai kopi, kedai minuman. Volume transaksi tinggi, butuh modul dapur.
3. **Jasa** — barbershop, salon, laundry, servis. Butuh jadwal & tiket kerja, stok minim.
4. **Grosir/agen** — butuh harga bertingkat, piutang, dan surat jalan. Paling kompleks, kerjakan terakhir.

Dua persona lain — **freelance/jasa profesional** dan **sales lapangan** — tidak memakai layar kasir sama
sekali. Keduanya dilayani modul terpisah di [Bagian E](#bagian-e--modul-crm-freelance--sales-lapangan),
dibangun di atas fondasi yang sama.

---

## Bagian B — Kebutuhan UMKM

Kolom **Prioritas**: `P0` = tanpa ini produk tidak bisa dipakai berjualan; `P1` = alasan orang mau
membayar; `P2` = pembeda kompetitif; `P3` = nanti.

### B.1 Kebutuhan universal (semua jenis usaha)

Ini himpunan minimum yang dibutuhkan **semua** UMKM tanpa memandang jenis usaha.

#### 1. Pendaftaran & profil usaha

| Kebutuhan | Prioritas | Catatan |
|---|---|---|
| Daftar mandiri (self-service) dengan nomor HP / email | P0 | Verifikasi OTP WhatsApp lebih dipercaya daripada email di segmen ini |
| Profil usaha: nama, jenis usaha, alamat, logo, nomor kontak | P0 | Dipakai di header struk |
| Multi-outlet / cabang dalam satu akun | P1 | Rancang skemanya sejak awal walau UI-nya belakangan |
| Pengaturan mata uang, zona waktu, format tanggal | P0 | Default `IDR`, `Asia/Jakarta` |
| Pengaturan pajak & biaya layanan per outlet | P0 | Tarif **harus** konfigurasi, jangan pernah di-hard-code |

#### 2. Produk & master data

| Kebutuhan | Prioritas | Catatan |
|---|---|---|
| Produk: nama, SKU, barcode, kategori, satuan, harga jual, harga modal | P0 | Harga modal wajib — tanpa itu laba tidak bisa dihitung |
| Kategori bertingkat (maksimal 2 level) | P1 | Lebih dalam dari 2 level membingungkan pengguna |
| Satuan & konversi satuan (dus → pak → pcs) | P1 | Wajib untuk grosir & toko kelontong |
| Varian (ukuran, warna, level pedas) | P1 | Jangan pakai matriks varian penuh; cukup daftar varian datar |
| Produk paket / bundling | P2 | Potong stok komponen saat terjual |
| Produk non-stok (jasa, ongkos kirim) | P0 | Tidak ikut perhitungan stok |
| Harga bertingkat (eceran / grosir / member) | P1 | |
| Impor produk dari Excel/CSV | P1 | **Penentu adopsi.** Pengguna pindahan punya ratusan produk; tanpa impor mereka batal pakai |
| Foto produk | P2 | Wajib untuk F&B, opsional untuk retail |

#### 3. Transaksi kasir (jantung produk)

| Kebutuhan | Prioritas | Catatan |
|---|---|---|
| Cari produk: ketik nama, scan barcode, atau tekan tombol favorit | P0 | Scanner barcode = input keyboard, tidak butuh driver khusus |
| Keranjang: ubah jumlah, hapus, catatan per item | P0 | |
| Diskon per item & per transaksi (nominal & persen) | P0 | |
| Pajak & biaya layanan otomatis | P0 | Bisa dimatikan per transaksi |
| Pembayaran: tunai, QRIS, transfer, kartu debit/kredit, e-wallet | P0 | Minimal catat metodenya walau belum terintegrasi gateway |
| Pembayaran gabungan (sebagian tunai, sebagian QRIS) | P1 | |
| Kembalian & saran pecahan uang | P0 | Hitung otomatis, tampilkan besar |
| Tahan transaksi / *open bill* | P1 | Wajib untuk F&B dan antrean ramai |
| Batal & retur transaksi (dengan alasan + jejak audit) | P0 | Stok dikembalikan, laporan menyesuaikan |
| Kasbon / bayar nanti (piutang pelanggan) | P1 | Sangat umum di warung & grosir |
| Cetak struk termal 58 mm & 80 mm | P0 | |
| Struk digital (PDF / tautan / kirim WhatsApp) | P1 | Menghemat kertas, jadi jalur pemasaran |
| Mode offline penuh + sinkron otomatis | P0 | Lihat [C.5](#c5-offline-first--sinkronisasi) |

#### 4. Kas & shift kasir

| Kebutuhan | Prioritas | Catatan |
|---|---|---|
| Buka kas (modal awal laci) & tutup kas | P0 | |
| Rekonsiliasi: uang fisik vs catatan sistem, selisih tercatat | P0 | Ini alasan utama pemilik membeli POS: mengontrol kasir |
| Kas masuk / kas keluar non-penjualan (bayar listrik, ambil uang) | P1 | |
| Laporan per shift & per kasir | P1 | |

#### 5. Stok / inventori

| Kebutuhan | Prioritas | Catatan |
|---|---|---|
| Stok otomatis berkurang saat penjualan | P0 | Dalam satu transaksi database dengan checkout |
| Stok masuk / pembelian ke supplier | P1 | |
| Penyesuaian stok manual (rusak, hilang, koreksi) + alasan | P0 | |
| Kartu stok / riwayat pergerakan per produk | P1 | Sumber kebenaran saat angka stok diperdebatkan |
| Peringatan stok menipis | P1 | Notifikasi harian, bukan per detik |
| Stok opname (hitung fisik & selisih) | P2 | |
| Transfer stok antar outlet | P2 | |
| Data supplier | P1 | |

#### 6. Pelanggan

| Kebutuhan | Prioritas | Catatan |
|---|---|---|
| Data pelanggan: nama, HP, alamat | P1 | |
| Riwayat pembelian per pelanggan | P2 | |
| Piutang pelanggan & pelunasan | P1 | |
| Poin / membership sederhana | P2 | Jangan buat aturan poin yang rumit |

#### 7. Laporan

Aturannya: **pemilik membuka aplikasi untuk melihat angka, bukan untuk berjualan.** Dashboard harus
menjawab "hari ini untung berapa" dalam satu layar tanpa filter.

| Kebutuhan | Prioritas | Catatan |
|---|---|---|
| Dashboard: omzet hari ini, jumlah transaksi, rata-rata per transaksi, laba kotor | P0 | |
| Laporan penjualan per periode / outlet / kasir / metode bayar | P0 | |
| Produk terlaris & produk mati | P1 | |
| Laba kotor per produk (harga jual − harga modal) | P1 | Alasan utama harga modal wajib diisi |
| Laporan arus kas sederhana | P1 | |
| Laporan stok & nilai persediaan | P1 | |
| Rekap pajak | P2 | Untuk pemilik yang sudah PKP |
| Ekspor Excel/CSV & PDF | P0 | Termasuk paket gratis |

#### 8. Pengguna & hak akses

| Kebutuhan | Prioritas | Catatan |
|---|---|---|
| Peran: Pemilik, Manajer/Supervisor, Kasir | P0 | |
| Izin granular per fitur (lihat laporan, beri diskon, batalkan transaksi, ubah harga) | P1 | RBAC berbasis nama role saat ini belum cukup |
| PIN cepat untuk ganti kasir di perangkat yang sama | P1 | Login email+password terlalu lambat di kasir |
| Jejak audit untuk aksi sensitif (batal, diskon besar, ubah harga, hapus data) | P0 | Perlindungan hukum dan bukti saat ada kecurangan |

#### 9. Keandalan data

| Kebutuhan | Prioritas | Catatan |
|---|---|---|
| Cadangan (backup) otomatis + uji pemulihan | P0 | |
| Ekspor seluruh data tenant | P1 | Kewajiban etis dan sering jadi syarat calon pelanggan besar |
| Soft delete untuk data transaksional | P1 | Data keuangan tidak boleh benar-benar hilang |

### B.2 Kebutuhan per jenis usaha

Modul tambahan di atas himpunan universal. Bangun sebagai modul opsional yang bisa dinyalakan per
tenant, jangan dicampur ke inti.

| Jenis usaha | Kebutuhan khas | Prioritas |
|---|---|---|
| **Retail / kelontong** | Barcode & timbangan, satuan bertingkat, harga grosir, produk kedaluwarsa | P1 |
| **F&B** | Nomor meja, open bill, cetak pesanan ke dapur/bar, catatan pesanan ("tanpa es"), takeaway vs dine-in, resep & potong bahan baku, PB1 (pajak restoran), pesanan dari aplikator layanan antar ([Bagian F](#bagian-f--kanal-pesanan-online-marketplace--aplikator)) | P1 |
| **Jasa (salon, laundry, servis)** | Jadwal & janji temu, tiket kerja + status pengerjaan, komisi karyawan, harga per layanan/durasi/kg | P2 |
| **Grosir / agen** | Harga bertingkat per jumlah, piutang & jatuh tempo, surat jalan, sales/kanvasing, minimum order | P2 |
| **Produksi kecil** | Bill of Material, biaya produksi, stok bahan baku vs barang jadi | P3 |
| **Apotek / toko dengan izin** | Nomor batch, tanggal kedaluwarsa, kategori obat | P3 |
| **Freelance / jasa profesional** | Penawaran, proyek bertermin, invoice + pengingat, laba per proyek — lihat [E.2](#e2-crm-freelance--jasa-profesional-solo) | P2 |
| **Grosir dengan tim sales** | Rute kunjungan, ambil pesanan di lokasi, penagihan lapangan, target & komisi — lihat [E.3](#e3-crm-sales-lapangan--kanvasing--distribusi) | P2 |

> **Peringatan cakupan:** godaan terbesar SaaS POS adalah melayani semua jenis usaha sekaligus di
> versi pertama. Selesaikan **retail** sampai benar-benar matang, baru F&B. Modul jasa dan grosir
> menyusul setelah ada pelanggan yang membayar.

### B.3 Kebutuhan regulasi & konteks Indonesia

Semua nilai di bawah ini disimpan sebagai **konfigurasi per tenant**, tidak pernah ditulis tetap di
kode — tarif dan aturan berubah, dan sebagian ditentukan peraturan daerah.

| Aspek | Yang perlu disediakan sistem |
|---|---|
| **QRIS** | Terima pembayaran QRIS (statis atau dinamis via PJP seperti Midtrans/Xendit/Doku). Catat biaya MDR agar laba bersih tidak salah hitung — tarif MDR berbeda untuk merchant mikro dan reguler, dan berubah dari waktu ke waktu, jadi jadikan kolom konfigurasi dan verifikasi tarif ke penyedia. |
| **PPN** | Untuk tenant yang sudah PKP: tarif PPN sebagai konfigurasi, pilihan harga sudah/belum termasuk pajak, dan nomor seri faktur bila diperlukan. Jangan pernah hard-code tarif. |
| **Pajak final UMKM** | Sebagian besar UMKM dikenai PPh final berbasis peredaran bruto (PP 55/2022). Sistem cukup menyediakan **laporan omzet bulanan** yang rapi agar pemilik bisa menyetor sendiri. Jangan menghitungkan pajak terutang untuk mereka — itu tanggung jawab profesional pajak. |
| **PB1 / pajak restoran** | Untuk F&B, pajak daerah dengan tarif yang ditentukan perda (umumnya sampai 10%). Wajib per-outlet karena berbeda antar kota. |
| **Identitas usaha** | Simpan NPWP dan NIB di profil usaha untuk keperluan cetak dokumen. |
| **UU PDP (UU No. 27 Tahun 2022)** | Data pelanggan (nama, HP) adalah data pribadi. Wajib: dasar pemrosesan yang jelas, kemampuan menghapus data atas permintaan, enkripsi saat transit, kontrol akses, dan pemberitahuan bila terjadi kebocoran. Siapkan Kebijakan Privasi & Syarat Layanan sebelum tenant pertama masuk. |
| **Bukti transaksi** | Struk memuat identitas usaha, tanggal-waktu, rincian barang, total, pajak, metode bayar, dan nomor transaksi unik. |
| **Retensi data keuangan** | Pembukuan umumnya wajib disimpan bertahun-tahun. Jangan hapus permanen data transaksi; pakai soft delete dan arsip. |

> Tarif dan pasal berubah. Cantumkan tanggal verifikasi terakhir di halaman pengaturan pajak, dan
> jangan menempatkan aplikasi sebagai pemberi nasihat pajak.

### B.4 Kebutuhan perangkat & lingkungan kerja

Ini yang paling sering diabaikan pengembang, dan paling sering jadi alasan POS ditinggalkan.

| Kenyataan di lapangan | Konsekuensi desain |
|---|---|
| Perangkat berupa HP Android murah atau tablet, bukan PC | UI harus jalan mulus di layar 7–10 inci, RAM 2 GB, dan browser lawas |
| Internet putus-putus atau kuota habis | Offline-first bukan fitur tambahan, melainkan syarat |
| Listrik mati | Data yang belum tersinkron harus selamat: simpan ke penyimpanan lokal **sebelum** mengonfirmasi ke pengguna |
| Printer termal Bluetooth 58 mm (paling murah) | Dukung ESC/POS; sediakan juga struk PDF/gambar untuk printer yang tidak kompatibel |
| Scanner barcode USB/Bluetooth murah | Bekerja sebagai keyboard — pastikan fokus input di layar kasir tidak pernah lepas |
| Laci uang (cash drawer) terhubung ke printer | Buka laci lewat perintah kick-out ESC/POS |
| Kasir berganti-ganti orang, sebagian gaptek | Alur singkat, tombol besar, teks Bahasa Indonesia, kesalahan dapat dibatalkan |
| Pemilik memantau dari rumah | Dashboard harus enak dibuka di HP |

### B.5 Yang sengaja TIDAK dikerjakan dulu

Ditulis agar tidak diam-diam masuk cakupan:

- Akuntansi berpasangan penuh (jurnal, neraca, buku besar) — cukup sediakan ekspor untuk akuntan.
- ~~Payroll / HR~~ — **keputusan ini berubah.** Absensi dan penggajian sederhana kini masuk cakupan
  sebagai modul tersendiri di [Bagian H](#bagian-h--absensi--penggajian-sederhana). Yang tetap ditolak
  adalah payroll tingkat perusahaan: perhitungan pajak otomatis, struktur jabatan berlapis, dan
  manajemen kinerja.
- ~~Integrasi marketplace~~ — **keputusan ini berubah.** Menerima pesanan dari marketplace dan aplikator
  kini masuk cakupan sebagai modul tersendiri di [Bagian F](#bagian-f--kanal-pesanan-online-marketplace--aplikator).
  Yang tetap ditunda hanyalah integrasi API penuh ke tiap kanal, karena bergantung pada persetujuan
  kemitraan di luar kendali kita — fondasi multi-kanal dan jalur manual/CSV dikerjakan lebih dulu.
- Aplikasi mobile native — PWA lebih dulu, native menyusul kalau memang terbukti perlu.
- Modul produksi/manufaktur.
- Kustomisasi struk tingkat lanjut & desainer template.

---

## Bagian C — Keputusan Arsitektur

### C.1 Keputusan yang wajib diambil SEBELUM tabel transaksi dibuat

Sesuai catatan utang teknis di [CONVENTIONS.md](../CONVENTIONS.md) bagian 7, keputusan berikut jauh
lebih mahal bila ditunda:

1. Model multi-tenancy.
2. Tipe data uang.
3. Sumber ID transaksi (server atau client) — menentukan apakah offline mungkin.
4. Strategi migrasi berversi menggantikan `AutoMigrate`.

### C.2 Model multi-tenancy

**Pilihan: satu database, satu skema, kolom `tenant_id` di setiap tabel.**

| Opsi | Nilai | Alasan |
|---|---|---|
| Database per tenant | ✗ | Biaya operasional dan migrasi tidak masuk akal untuk harga langganan UMKM |
| Skema per tenant | ✗ | Ribuan skema membuat migrasi dan connection pool berat |
| **Kolom `tenant_id` bersama** | ✓ | Termurah, paling sederhana, cukup sampai puluhan ribu tenant bila indeksnya benar |

Aturan yang mengikuti pilihan ini:

- Setiap tabel bisnis punya `tenant_id char(26) NOT NULL`.
- **Setiap** index dan foreign key dimulai dari `tenant_id`: `(tenant_id, created_at)`, `(tenant_id, sku)`.
- Penyaringan tenant terjadi di lapisan repository, **bukan** di controller — supaya tidak bisa lupa.
  Buat helper wajib, misalnya `scopeTenant(ctx, db)`, dan larang query tabel bisnis tanpa helper itu.
- Lapisan kedua: aktifkan **Row Level Security** PostgreSQL sebagai jaring pengaman. Kalau ada satu
  query yang lupa memfilter, database yang menolaknya, bukan pelanggan yang menemukan data orang lain.
- Uji kebocoran tenant adalah **test wajib**: buat dua tenant, pastikan tenant A tidak pernah bisa
  membaca atau mengubah data tenant B lewat jalur mana pun (termasuk tebak ID di URL).

Hierarki: `Tenant (usaha) → Outlet (cabang) → User`. Seorang user terikat pada satu tenant dan boleh
diberi akses ke satu atau banyak outlet.

### C.3 Uang, angka, dan waktu

- **Uang disimpan sebagai `BIGINT` dalam satuan terkecil (rupiah bulat).** Jangan `float`, jangan
  `double`. Selisih pembulatan pada uang orang lain adalah bug yang menghancurkan kepercayaan.
- Kuantitas memakai `NUMERIC(14,3)` — timbangan menjual 0,25 kg.
- Diskon disimpan sebagai **hasil hitung nominal** plus keterangan aturannya, supaya struk lama tetap
  bisa dicetak ulang persis walau aturan diskon berubah.
- Semua stempel waktu `TIMESTAMPTZ` dalam UTC; konversi ke zona waktu outlet saat ditampilkan.
- **Snapshot harga:** baris item transaksi menyimpan nama, harga jual, dan harga modal **saat itu**.
  Jangan mengambil harga dari tabel produk saat mencetak laporan lama.

### C.4 Otorisasi (RBAC granular)

Ganti `RoleMiddleware("admin")` dengan izin berbasis kemampuan:

```
permission := "sale.void" | "sale.discount" | "report.view" | "product.edit" | "user.manage" | ...
role       := kumpulan permission, milik satu tenant (tenant boleh membuat role sendiri)
middleware := RequirePermission("sale.void")
```

Peran bawaan yang dibuat otomatis saat tenant baru: **Owner** (semua izin), **Manager**, **Cashier**.

### C.5 Offline-first & sinkronisasi

Ini bagian tersulit sekaligus pembeda utama produk. Rancangannya:

1. **ID dibuat di client.** ULID digenerate di perangkat kasir (sudah sejalan dengan pemakaian ULID
   di repo ini). Transaksi punya identitas sebelum menyentuh server.
2. **Kunci idempoten.** Endpoint checkout menerima `Idempotency-Key`; pengiriman ulang mengembalikan
   hasil yang sama, tidak membuat transaksi ganda. Ini syarat mutlak untuk jaringan buruk.
3. **Antrean lokal.** Transaksi masuk IndexedDB dulu, tampilkan struk, lalu dikirim di latar belakang
   dengan percobaan ulang bertahap (*exponential backoff*).
4. **Arah sinkronisasi berbeda per jenis data:**
   - *Penjualan*: hanya tambah (append-only) dari client ke server → tidak ada konflik.
   - *Master data (produk, harga)*: server yang berwenang → client menarik perubahan.
   - *Stok*: server yang berwenang. Client menampilkan angka perkiraan; stok minus akibat penjualan
     offline **diterima**, lalu ditandai untuk ditinjau pemilik. Menolak transaksi offline karena
     stok tidak sinkron akan membuat kasir berhenti berjualan — itu lebih buruk daripada stok minus.
5. **Batas offline yang jujur.** Tampilkan indikator "X transaksi belum tersinkron" dan beri
   peringatan setelah ambang tertentu (misalnya > 24 jam atau > 200 transaksi).

### C.6 Integrasi eksternal (urutan pengerjaan)

| Integrasi | Kapan | Catatan |
|---|---|---|
| Payment gateway QRIS (Midtrans / Xendit / Doku) | Fase 8 | Mulai dari QRIS statis (cukup catat manual) sebelum integrasi penuh |
| WhatsApp (OTP + kirim struk) | Fase 1 & 4 | Pakai penyedia resmi; hindari solusi tidak resmi yang berisiko diblokir |
| Ekspor untuk akuntansi | Fase 5 | CSV standar sudah cukup |
| Marketplace & aplikator layanan antar | Fase 11 | Fondasi multi-kanal + entri manual/CSV lebih dulu; adaptor API menyusul seiring kemitraan disetujui — lihat [Bagian F](#bagian-f--kanal-pesanan-online-marketplace--aplikator) |

### C.7 Bentuk aplikasi client

**PWA (Progressive Web App)** dulu: satu basis kode, jalan di Android/iOS/desktop, pemasangan tanpa
app store, pembaruan langsung. Kelemahan yang harus diakui: pencetakan Bluetooth di iOS terbatas —
untuk itu sediakan jalur cetak lewat aplikasi pendamping Android atau printer jaringan.

---

## Bagian D — Model Data Inti

Daftar tabel minimum untuk POS yang berfungsi. Semua memakai ULID `char(26)` dan `tenant_id` (kecuali
`tenants` dan tabel platform).

**Tenancy & akses**

| Tabel | Isi penting |
|---|---|
| `tenants` | nama usaha, jenis usaha, status langganan, zona waktu, tanggal daftar |
| `outlets` | `tenant_id`, nama, alamat, telepon, pengaturan pajak, header/footer struk |
| `users` | *(sudah ada)* + `tenant_id`, `pin_hash`, status aktif |
| `roles` | *(sudah ada)* + `tenant_id` (nullable untuk peran bawaan sistem) |
| `permissions`, `role_permissions`, `user_outlets` | otorisasi granular |

**Master data**

| Tabel | Isi penting |
|---|---|
| `categories` | `tenant_id`, nama, induk |
| `units` | `tenant_id`, nama, konversi ke satuan dasar |
| `products` | `tenant_id`, nama, SKU, barcode, kategori, satuan, harga jual, **harga modal**, lacak stok (bool), aktif |
| `product_variants` | `product_id`, nama, selisih harga, SKU sendiri |
| `product_prices` | harga bertingkat (eceran/grosir/member) |
| `suppliers`, `customers` | data mitra, `tenant_id` |

**Transaksi**

| Tabel | Isi penting |
|---|---|
| `sales` | `tenant_id`, `outlet_id`, `shift_id`, nomor struk, subtotal, diskon, pajak, total, status (`paid`/`void`/`refunded`), `created_by`, `client_created_at`, `idempotency_key` |
| `sale_items` | `sale_id`, `product_id`, **snapshot** nama/harga jual/harga modal, qty, diskon, pajak |
| `sale_payments` | `sale_id`, metode, jumlah, referensi, biaya MDR |
| `shifts` | buka/tutup kas, modal awal, kas akhir sistem, kas akhir fisik, selisih |
| `cash_movements` | kas masuk/keluar non-penjualan + alasan |
| `receivables` | piutang & pelunasan |

**Stok**

| Tabel | Isi penting |
|---|---|
| `stock_movements` | **buku besar stok**: `product_id`, `outlet_id`, jenis (`sale`/`purchase`/`adjustment`/`transfer`/`void`), qty (+/−), referensi dokumen, sisa stok setelah gerakan |
| `stocks` | saldo stok per produk per outlet (agregat cepat, boleh dihitung ulang dari `stock_movements`) |
| `purchases`, `purchase_items` | pembelian ke supplier |
| `stock_opnames`, `stock_opname_items` | hitung fisik |

> **Aturan stok:** `stock_movements` adalah sumber kebenaran, `stocks` hanyalah cache. Sediakan
> perintah rekonsiliasi yang menghitung ulang `stocks` dari `stock_movements`. Tanpa itu, angka stok
> yang melenceng tidak akan pernah bisa diperbaiki dengan jujur.

**Platform (SaaS)**

| Tabel | Isi penting |
|---|---|
| `plans` | paket, harga, kuota (jumlah outlet, user, produk, transaksi/bulan) |
| `subscriptions` | `tenant_id`, paket, status, masa trial, periode berjalan, **`term_months`** (1/3/6/9/12) dan diskon yang dipakai |
| `deferred_revenue` | pendapatan diterima di muka yang belum diakui, per langganan per bulan |
| `subscription_invoices`, `subscription_payments` | tagihan langganan platform (dinamai jelas agar tidak bentrok dengan invoice pelanggan di modul CRM) |
| `audit_logs` | siapa, kapan, aksi apa, data sebelum/sesudah, IP |
| `outbox` | kejadian untuk pengiriman notifikasi/webhook yang andal |

**Aturan transaksi database untuk checkout** (satu `DB.Transaction`, tidak boleh dipecah):

```
BEGIN
  cek idempotency_key → kalau sudah ada, kembalikan hasil lama
  buat sales + sale_items + sale_payments
  buat stock_movements untuk tiap item berstok
  perbarui stocks (kunci baris: SELECT ... FOR UPDATE)
  perbarui shift berjalan
  tulis audit_log
COMMIT
```

---

## Bagian E — Modul CRM: Freelance & Sales Lapangan

POS mencatat transaksi **yang sudah terjadi**. CRM mengurus yang terjadi sebelumnya: prospek masuk,
follow-up, penawaran, penagihan, dan hubungan jangka panjang. Dua persona di bawah memakai inti yang
sama tetapi alur kerjanya berbeda jauh — dan keduanya **bukan** pengguna layar kasir.

> **Ini keputusan produk, bukan sekadar tambahan fitur.** Melayani freelance dan sales lapangan berarti
> menambah dua persona di luar pemilik toko. Kerjakan setelah POS retail matang (Fase 0–7), kecuali
> segmen freelance memang dijadikan target utama — dalam hal itu justru POS yang ditunda. Mengerjakan
> keduanya bersamaan adalah cara tercepat menghasilkan dua produk setengah jadi.

> **Jangan tertukar dengan [Bagian G](#bagian-g--program-mitra-penjual-agen--freelance-sales).** Bagian E
> adalah CRM yang kita **jual** kepada tenant untuk mengelola pelanggan *mereka*. Bagian G adalah CRM
> yang kita **pakai sendiri** untuk mengelola agen dan freelance sales yang menjual aplikasi *kita*.
> Mesinnya mirip, penggunanya sama sekali berbeda.

Alasan modul ini layak berdiri di atas fondasi yang sama: tenancy, RBAC, data pelanggan, produk &
harga bertingkat, penomoran dokumen, laporan, dan antrean sinkronisasi offline **sudah ada**. Yang
benar-benar baru hanya pipeline prospek, dokumen bertermin (penawaran → invoice), dan aktivitas lapangan.

### E.1 Inti CRM (dipakai kedua persona)

| Kebutuhan | Prioritas | Catatan |
|---|---|---|
| Kontak & perusahaan, dengan penanggung jawab lebih dari satu | P0 | Perluasan tabel `customers`, bukan tabel identitas baru |
| Sumber prospek (WhatsApp, Instagram, marketplace, referral, walk-in) | P0 | Tanpa ini pemilik tidak tahu kanal mana yang menghasilkan |
| Pipeline & tahap yang bisa diatur sendiri per tenant | P0 | Tahap freelance dan tahap sales berbeda — jangan dipaksa seragam |
| Deal/peluang: nilai, perkiraan tanggal tutup, alasan menang/kalah | P0 | Alasan kalah adalah data paling berharga dan paling sering dilupakan |
| Aktivitas & pengingat follow-up (telepon, chat, kunjungan, kirim penawaran) | P0 | Follow-up yang lupa dikerjakan adalah kebocoran omzet terbesar di segmen ini |
| Catatan & lampiran per kontak/deal | P1 | Foto, PDF penawaran, tangkapan layar chat |
| Riwayat interaksi dalam satu lini waktu | P1 | Termasuk transaksi POS pelanggan yang sama |
| Kepemilikan data (`owner_id`) + serah terima | P0 | Lihat [E.5](#e5-kepemilikan-data--lapisan-ketiga-setelah-tenant) |
| WhatsApp sebagai kanal utama, dengan template pesan | P1 | Email hampir tidak dipakai di segmen ini |
| Impor kontak dari CSV / kontak HP | P1 | Sama seperti impor produk: penentu adopsi |

### E.2 CRM Freelance — jasa profesional solo

Persona: desainer, fotografer, videografer, MUA, konsultan, developer, penulis, guru les, kontraktor
kecil. Ciri khasnya: nilai per transaksi besar, jumlah transaksi sedikit, uang masuk **bertahap**, dan
tidak ada stok.

| Kebutuhan | Prioritas | Catatan |
|---|---|---|
| Penawaran (quotation) dengan template, masa berlaku, dan status | P0 | Disetujui → otomatis menjadi proyek, tanpa mengetik ulang |
| Proyek dengan termin pembayaran (DP / progres / pelunasan) | P0 | Ini pembeda utama dari POS: satu penjualan, banyak pembayaran |
| Invoice bernomor, jatuh tempo, dan pengingat otomatis | P0 | Pengingat H-3, hari-H, H+3, H+7 lewat WhatsApp |
| Pencatatan pembayaran parsial & sisa tagihan | P0 | |
| Daftar pekerjaan / deliverable per proyek + tenggat | P1 | Cukup daftar tugas sederhana, bukan manajemen proyek penuh |
| Biaya proyek (vendor, transport, cetak, sewa alat) | P1 | Tanpa ini "laba per proyek" hanya tebakan |
| Retainer / langganan bulanan klien | P2 | Invoice berulang otomatis |
| Pelacakan waktu kerja | P2 | Hanya untuk yang menagih per jam |
| Kontrak & tanda tangan sederhana | P2 | Cukup PDF + persetujuan lewat tautan |
| Portofolio / galeri hasil kerja | P3 | |

> **Aturan integrasi:** invoice yang lunas **dicatat sebagai penjualan** di tabel yang sama dengan POS.
> Dengan begitu omzet, laba, dan rekap pajak tetap satu pintu — pemilik tidak perlu menjumlahkan dua
> laporan yang berbeda. Termin pembayaran dicatat sebagai pembayaran bertahap atas penjualan tersebut.

### E.3 CRM Sales Lapangan — kanvasing & distribusi

Persona: sales grosir, agen distributor, sales kanvas yang membawa barang di kendaraan. Ciri khasnya:
bekerja di jalan, sinyal buruk, dan pemilik butuh bukti bahwa kunjungan benar-benar terjadi.

| Kebutuhan | Prioritas | Catatan |
|---|---|---|
| Database toko pelanggan + titik lokasi | P0 | Satu sales bisa memegang ratusan toko |
| Rencana kunjungan harian / rute (call plan) | P0 | Disusun supervisor atau sales sendiri |
| Check-in & check-out dengan GPS + foto toko | P0 | Bukti kunjungan — alasan utama pemilik membeli modul ini |
| Ambil pesanan di lokasi dengan harga tingkat pelanggan | P0 | Memakai produk & harga yang sama dengan POS |
| Kunjungan tanpa pesanan + alasan (tutup, stok masih penuh, tolak) | P0 | Data ini yang menjelaskan kenapa target tidak tercapai |
| Penagihan piutang di lapangan + bukti setoran | P1 | Uang tunai yang dipegang sales harus terekonsiliasi |
| Stok kanvas (barang di kendaraan) + rekonsiliasi akhir hari | P1 | Outlet bertipe "kendaraan" — memakai `stock_movements` yang sama |
| Target vs pencapaian per sales & per produk | P1 | |
| Komisi berdasarkan penjualan tertagih, bukan hanya terkirim | P1 | Membayar komisi atas piutang macet adalah kesalahan mahal |
| Laporan aktivitas harian otomatis ke supervisor | P1 | Dikirim sore hari lewat WhatsApp |
| Survei / foto display & harga kompetitor | P2 | |

> **Offline mutlak.** Sales bekerja di area yang sinyalnya paling buruk. Semua bukti — foto, GPS,
> pesanan — masuk antrean lokal dulu, persis seperti transaksi kasir di [C.5](#c5-offline-first--sinkronisasi).
> Foto dikompres di perangkat sebelum diantre agar kuota sales tidak habis.

> **Privasi karyawan.** Titik lokasi adalah data pribadi. Rekam **hanya pada saat check-in/check-out**,
> bukan pelacakan terus-menerus; beri tahu sales bahwa lokasinya direkam dan untuk apa. Selain
> kewajiban UU PDP, pelacakan diam-diam adalah alasan paling umum aplikasi sales dibuang karyawannya.

### E.4 Tambahan model data

Semua tabel mengikuti aturan yang sama: ULID, `tenant_id`, index dimulai dari `tenant_id`.

| Tabel | Isi penting |
|---|---|
| `contact_persons` | orang di dalam satu `customers`: nama, jabatan, HP |
| `lead_sources` | kanal asal prospek, dapat diatur tenant |
| `pipelines`, `pipeline_stages` | tahap yang bisa diatur sendiri, punya urutan & probabilitas |
| `deals` | `customer_id`, nilai, tahap, `owner_id`, perkiraan tutup, status menang/kalah + alasan |
| `activities` | jenis (telepon/chat/kunjungan/tugas), jadwal, status, relasi ke deal/kontak |
| `quotations`, `quotation_items` | penawaran + masa berlaku + status; item memakai snapshot harga |
| `invoices`, `invoice_items` | invoice pelanggan, nomor, jatuh tempo, status |
| `invoice_payments` | pembayaran bertahap; pelunasan memicu pencatatan penjualan |
| `projects`, `project_tasks`, `project_expenses` | proyek freelance, tugas, biaya |
| `visit_plans`, `visits` | rencana & realisasi kunjungan: GPS, foto, waktu, hasil |
| `sales_targets` | target per sales / periode / produk |
| `commissions` | aturan & perhitungan komisi, dasar tertagih |

Catatan penyesuaian pada tabel yang sudah dirancang di [Bagian D](#bagian-d--model-data-inti):

- `customers` mendapat `owner_id`, `type` (perorangan/perusahaan/toko), dan titik lokasi.
- `outlets` mendapat `type` agar kendaraan kanvas bisa jadi lokasi stok tanpa tabel baru.
- Tabel tagihan langganan platform dinamai `subscription_invoices` / `subscription_payments` supaya
  tidak tertukar dengan invoice pelanggan di modul ini.

### E.5 Kepemilikan data — lapisan ketiga setelah tenant

Isolasi tenant saja tidak cukup di sini: dalam satu tenant, seorang sales **tidak boleh** melihat
pelanggan milik sales lain, sementara supervisor melihat timnya dan pemilik melihat semuanya.

- Tambahkan `owner_id` pada `customers`, `deals`, `activities`, `visits`, `quotations`, `invoices`.
- Terapkan sebagai scope wajib di repository, satu pola dengan `scopeTenant`: `scopeVisibility(ctx, db)`.
- Izin baru: `crm.lead.view.all` / `crm.lead.view.own`, `crm.deal.edit`, `crm.visit.checkin`,
  `crm.commission.view`, `invoice.issue`, `invoice.void`, `quotation.approve`.
- Test kebocoran diperluas: selain lintas tenant, uji juga lintas pemilik di dalam satu tenant.

### E.6 Otomasi & notifikasi

Semua pengiriman melewati tabel `outbox` yang sudah dirancang, supaya pesan tidak hilang saat penyedia
WhatsApp sedang gagal.

| Pemicu | Pesan |
|---|---|
| Aktivitas follow-up jatuh tempo | Pengingat ke pemilik aktivitas (H-1 dan hari-H) |
| Penawaran mendekati masa berlaku habis | Pengingat ke sales/freelancer |
| Invoice jatuh tempo | Pengingat ke pelanggan: H-3, hari-H, H+3, H+7 |
| Deal diam terlalu lama | Peringatan "prospek dingin" setelah N hari tanpa aktivitas |
| Akhir hari | Rekap aktivitas sales ke supervisor |

Template pesan wajib bisa disunting tenant — nada bahasa penagihan sangat berbeda antar usaha.

### E.7 Laporan khas modul

**Freelance:** nilai pipeline per tahap · konversi penawaran → proyek · umur piutang (aging) ·
laba per proyek setelah biaya · proyek melewati tenggat.

**Sales lapangan:** pencapaian vs target per sales · kunjungan efektif (rasio kunjungan yang
menghasilkan pesanan) · pelanggan aktif vs tidur · piutang per sales · komisi berjalan ·
peta sebaran kunjungan.

---

## Bagian F — Kanal Pesanan Online: Marketplace & Aplikator

Dua keluhan yang paling sering terdengar dari UMKM yang sudah jualan online:

- **F&B:** "tablet berjejer di kasir." Setiap aplikator layanan antar punya aplikasi merchant sendiri.
  Pesanan yang masuk lewat sana tidak tercatat di POS, sehingga laporan penjualan pecah dan stok bahan
  baku tidak pernah cocok.
- **Retail:** *overselling*. Barang terjual di toko fisik, stok di marketplace belum berkurang, pesanan
  masuk untuk barang yang sudah habis, lalu dibatalkan — dan skor toko ikut turun.

Modul ini menyelesaikan keduanya dengan satu prinsip: **semua pesanan dari kanal mana pun bermuara ke
satu tabel penjualan dan satu buku besar stok.**

> **Catatan perubahan keputusan.** Di [B.5](#b5-yang-sengaja-tidak-dikerjakan-dulu) integrasi marketplace
> semula ditandai "jangan dikerjakan dulu". Keputusan itu dicabut. Yang tetap berlaku dari kehati-hatian
> lama: **jangan menjanjikan integrasi API sebelum kemitraan dengan kanalnya disetujui.** Cara aman
> dijelaskan di [F.9](#f9-kenyataan-kemitraan-api--mulai-dari-jalur-manual).

### F.1 Tiga jenis kanal, tiga masalah berbeda

| Kanal | Contoh | Masalah utama yang diselesaikan | Kesulitan integrasi |
|---|---|---|---|
| **Aplikator layanan antar** | GoFood, GrabFood, ShopeeFood | Pesanan tidak masuk POS; harga & komisi berbeda; dapur kebanjiran tanpa antrean tunggal | Tinggi — akses API terbatas |
| **Marketplace** | Tokopedia, Shopee, TikTok Shop, Lazada | Stok tidak sinkron (overselling); pesanan & resi dikelola terpisah | Sedang — ada program open API |
| **Sosial & percakapan** | WhatsApp, Instagram, katalog sendiri | Pesanan tercecer di chat; tidak ada rekap | Rendah — cukup entri manual bertanda kanal |

Kanal ketiga sering diremehkan, padahal untuk banyak UMKM justru paling besar volumenya — dan biaya
mendukungnya nyaris nol.

### F.2 Aturan inti: satu tabel penjualan untuk semua kanal

Godaan terbesar adalah membuat tabel pesanan terpisah per kanal. Sekali itu dilakukan, laporan pecah
selamanya dan tidak ada satu pun angka omzet yang bisa dipercaya.

- Tabel `sales` mendapat `channel_id` dan `external_order_id`.
- Siklus hidup pesanan online lebih panjang daripada transaksi kasir, jadi `sales.status` diperluas:
  `pending` → `accepted` → `preparing` → `ready` / `shipped` → `completed`, dengan cabang `rejected`,
  `canceled`, `returned`. Transaksi kasir langsung lahir sebagai `completed`.
- Laporan omzet hanya menghitung status final. Pesanan batal tetap tersimpan — tingkat pembatalan per
  kanal adalah data yang berharga.
- Stok bergerak lewat `stock_movements` yang sama, dengan referensi ke pesanan kanal.

### F.3 Kebutuhan — aplikator layanan antar (F&B)

| Kebutuhan | Prioritas | Catatan |
|---|---|---|
| Pesanan masuk tampil di satu antrean bersama pesanan kasir | P0 | Menghapus alasan tablet berjejer |
| Terima / tolak pesanan + alasan tolak | P0 | Batas waktu respons tiap aplikator berbeda |
| Cetak otomatis ke printer dapur saat pesanan diterima | P0 | Tanpa ini dapur tetap bekerja dari tablet |
| Status pesanan dikirim balik ke aplikator (siap diambil) | P0 | Kalau tidak, driver menunggu tanpa kabar |
| Daftar harga khusus per kanal | P0 | Harga di aplikator dinaikkan untuk menutup komisi — lihat [F.8](#f8-harga-komisi-dan-rekonsiliasi-pencairan) |
| Tandai item habis / buka-tutup toko dari POS | P0 | Mencegah pesanan masuk untuk barang yang sudah habis |
| Potong stok bahan baku lewat resep | P1 | Memakai modul resep F&B yang sama |
| Rekonsiliasi pencairan dana mingguan | P1 | Cocokkan pesanan dengan pencairan; selisih harus terlihat |
| Suara & notifikasi pesanan baru | P0 | Kasir sedang melayani antrean; pesanan tidak boleh terlewat |
| Laporan laba bersih per kanal setelah komisi | P1 | Sering jadi kejutan: kanal yang ramai belum tentu untung |

### F.4 Kebutuhan — marketplace (retail)

| Kebutuhan | Prioritas | Catatan |
|---|---|---|
| Pemetaan SKU marketplace ↔ produk internal | P0 | SKU di marketplace hampir tidak pernah sama; pemetaan wajib bisa disunting massal |
| Sinkronisasi stok otomatis setelah setiap penjualan | P0 | Inti dari pencegahan overselling |
| Tarik pesanan baru + rincian pembeli & alamat | P0 | |
| Cetak label pengiriman / nomor resi | P1 | Ambil dari kanal bila API menyediakan |
| Perbarui status: diproses, dikirim, selesai | P1 | |
| Retur & pembatalan mengembalikan stok | P0 | |
| Rekonsiliasi pencairan: komisi, biaya layanan, subsidi ongkir | P1 | Angka yang masuk rekening selalu lebih kecil dari nilai pesanan |
| Unggah produk dari POS ke marketplace | P2 | Arah sebaliknya; kerjakan setelah tarik pesanan stabil |
| Alokasi stok per kanal | P2 | Untuk pedagang bervolume tinggi — lihat [F.7](#f7-stok-lintas-kanal) |

### F.5 Kanal percakapan — murah, jangan dilewatkan

Pesanan lewat WhatsApp dan Instagram cukup ditangani dengan **entri pesanan manual yang diberi tanda
kanal**, ditambah katalog produk yang bisa dibagikan sebagai tautan. Tidak butuh integrasi apa pun,
langsung memberi pemilik satu hal yang selama ini tidak ada: rekap penjualan per kanal yang jujur.

Bila modul CRM ([Bagian E](#bagian-e--modul-crm-freelance--sales-lapangan)) aktif, pesanan percakapan
otomatis terhubung ke kontak dan riwayat pelanggannya.

### F.6 Arsitektur: adaptor, kotak masuk, idempotensi

```
Kanal → webhook / polling → channel_events (mentah, belum diproses)
                                   ↓ pekerja asinkron
                        adaptor kanal (antarmuka seragam)
                                   ↓
                   sales + sale_items + stock_movements
```

Aturan yang mengikat:

1. **Antarmuka adaptor seragam** untuk semua kanal: `FetchOrders`, `AcceptOrder`, `RejectOrder`,
   `PushStatus`, `SyncStock`, `SetItemAvailability`. Kanal baru berarti satu implementasi baru, bukan
   perubahan di inti.
2. **Webhook tidak pernah memproses langsung.** Simpan mentah ke `channel_events`, balas cepat, proses
   di pekerja terpisah. Kanal akan mengirim ulang bila balasan lambat.
3. **Idempoten berdasarkan `external_order_id`.** Kanal mengirim pesan ganda — itu normal, bukan kasus
   langka. Pesanan yang sama tidak boleh menghasilkan dua penjualan.
4. **Kegagalan kanal tidak boleh menghentikan kasir.** Integrasi berjalan di luar jalur checkout; kanal
   yang mati hanya membuat pesanannya tertunda, bukan membuat toko berhenti berjualan.
5. **Percobaan ulang bertahap + antrean mati (dead letter)** untuk pesanan yang gagal diproses, dengan
   tampilan agar pemilik bisa melihat dan memperbaikinya sendiri.
6. **Batas laju (rate limit) tiap kanal dihormati**, dan penyegaran token dilakukan otomatis.

### F.7 Stok lintas kanal

| Strategi | Untuk siapa | Cara kerja |
|---|---|---|
| **Stok bersama + cadangan penyangga** | Mayoritas UMKM | Semua kanal melihat stok yang sama dikurangi penyangga (misal 2 pcs). Sederhana, cukup untuk sebagian besar kasus |
| **Alokasi per kanal** | Volume tinggi | Stok dibagi eksplisit per kanal; sisa yang tidak terpakai dikembalikan berkala |

Dua kejadian yang wajib ditangani sejak awal:

- **Kasir sedang offline dan marketplace tetap menjual.** Server tetap berwenang atas stok. Saat POS
  tersinkron, stok bisa menjadi minus — terima, lalu tandai untuk ditinjau, konsisten dengan
  [C.5](#c5-offline-first--sinkronisasi). Jangan menolak penjualan yang sudah terjadi.
- **Sinkronisasi stok gagal terkirim.** Antrekan dan coba lagi; tampilkan indikator "stok kanal X
  terlambat N menit" supaya pemilik tahu risikonya sebelum overselling terjadi.

### F.8 Harga, komisi, dan rekonsiliasi pencairan

Ini bagian yang paling sering salah dan paling mahal akibatnya.

- **Harga berbeda per kanal.** Pedagang menaikkan harga di aplikator untuk menutup komisi. Perluas
  `product_prices` menjadi daftar harga per kanal, jangan buat tabel harga terpisah.
- **Komisi & biaya kanal dicatat sebagai biaya pada penjualan itu**, bukan diabaikan. Komisi aplikator
  umumnya berkisar sekitar 20% ditambah PPN atas jasanya, sementara marketplace memungut komisi yang
  berbeda-beda per kategori dan tingkat toko — semuanya **konfigurasi per kanal**, dan angkanya wajib
  diverifikasi ke perjanjian merchant masing-masing.
- **Promo yang dibiayai merchant** (potongan yang ditanggung pedagang, bukan platform) dicatat terpisah
  dari diskon biasa. Tanpa ini, laba per kanal salah hitung.
- **Rekonsiliasi pencairan.** Uang yang masuk ke rekening selalu lebih kecil daripada nilai pesanan.
  Sediakan pencocokan antara pesanan, potongan, dan pencairan; tampilkan selisih yang tidak cocok.
- **Laporan laba bersih per kanal** adalah hasil akhir dari semua ini — dan sering menjadi temuan paling
  berharga bagi pemilik: kanal yang paling ramai belum tentu kanal yang paling menguntungkan.

### F.9 Kenyataan kemitraan API — mulai dari jalur manual

Akses API kanal tidak terbuka bebas. Marketplace umumnya menyediakan program open platform yang bisa
didaftari, sedangkan aplikator layanan antar cenderung membuka integrasi hanya untuk mitra terpilih,
dengan proses persetujuan yang panjang dan syarat yang berubah-ubah. **Verifikasi ketersediaan sebelum
menjanjikan apa pun ke pelanggan.**

Karena itu modul ini dikerjakan dalam dua langkah yang bisa dijual terpisah:

| Langkah | Isi | Bergantung pada pihak lain? |
|---|---|---|
| **F-a — Fondasi multi-kanal** | `channels`, harga per kanal, status pesanan, entri pesanan manual, impor laporan harian kanal (CSV), laporan & laba per kanal | **Tidak.** Bisa dirilis kapan saja |
| **F-b — Adaptor API** | Satu adaptor per kanal, dipasang seiring kemitraan disetujui | Ya |

Langkah F-a saja sudah menyelesaikan keluhan "laporan saya pecah" — pemilik menyalin pesanan sekali
sehari, dan seluruh angka omzet, stok, serta laba akhirnya berada di satu tempat. Ini juga cara paling
murah untuk membuktikan bahwa fiturnya memang dibutuhkan sebelum menghabiskan waktu mengurus kemitraan.

### F.10 Tambahan model data

| Tabel | Isi penting |
|---|---|
| `channels` | jenis (aplikator/marketplace/percakapan/pos), nama, status aktif, kredensial terenkripsi, outlet terkait |
| `channel_products` | pemetaan produk internal ↔ SKU/ID kanal, harga kanal, status ketersediaan |
| `channel_events` | muatan mentah dari webhook/polling, status proses, jumlah percobaan |
| `channel_orders` | data khas kanal atas sebuah `sales`: id eksternal, status kanal, driver/kurir, resi, muatan asli |
| `channel_fees` | komisi, biaya layanan, subsidi ongkir, promo tanggungan merchant per pesanan |
| `channel_settlements` | pencairan dana: periode, nilai, potongan, status pencocokan |
| `channel_stock_syncs` | riwayat & status sinkronisasi stok per kanal, untuk indikator keterlambatan |

Penyesuaian tabel yang sudah ada di [Bagian D](#bagian-d--model-data-inti):

- `sales` mendapat `channel_id`, `external_order_id`, dan status yang diperluas.
- `product_prices` diperluas agar bisa berbasis kanal, bukan hanya tingkat pelanggan.
- `outlets` menyimpan pemetaan ke identitas merchant di tiap kanal.

### F.11 Laporan khas modul

Penjualan per kanal · **laba bersih per kanal setelah komisi dan promo** · produk terlaris per kanal ·
tingkat pembatalan & penolakan per kanal · waktu rata-rata terima → siap (F&B) · kejadian overselling
yang tercegah · umur keterlambatan sinkronisasi stok.

### F.12 Risiko yang harus diakui sejak awal

| Risiko | Mitigasi |
|---|---|
| API kanal berubah sepihak | Adaptor terisolasi; uji kontrak per kanal; pantau tingkat kegagalan |
| Kemitraan ditolak atau dicabut | Jalur manual/CSV tetap berfungsi tanpa API |
| Batas laju & token kedaluwarsa | Antrean dengan percobaan bertahap, penyegaran token otomatis |
| Data pembeli dari kanal | Tunduk UU PDP sama seperti data pelanggan lain; simpan seperlunya |
| Ketergantungan pada satu kanal besar | Rancang agar kanal bisa dimatikan tanpa mengganggu operasi lain |

---

## Bagian G — Program Mitra Penjual: Agen & Freelance Sales

Di [model bisnis](BUSINESS-MODEL-CANVAS.md), agen dan reseller daerah adalah saluran utama untuk
tumbuh — mereka yang datang ke toko, memasang printer, dan mendampingi impor produk. Program itu tidak
bisa dijalankan dari spreadsheet selamanya: begitu mitra lewat dari belasan orang, perhitungan komisi
mulai memakan waktu berhari-hari dan sengketa atribusi mulai muncul.

Modul ini adalah **CRM untuk kita sendiri**, bukan untuk tenant.

| | [Bagian E](#bagian-e--modul-crm-freelance--sales-lapangan) — CRM tenant | **Bagian G — Program mitra** |
|---|---|---|
| Siapa penggunanya | Freelancer & sales milik tenant | Agen & freelance sales milik **kita** |
| Apa yang dijual | Barang/jasa tenant | **Langganan aplikasi ini** |
| Siapa pelanggannya | Pelanggan tenant | Calon tenant |
| Lingkup data | Dalam satu tenant | **Tingkat platform**, lintas tenant |
| Status | Modul berbayar untuk tenant | Perkakas internal |

**Peluang penggunaan ulang:** mesin pipeline, aktivitas, dan `owner_id` di Bagian E sudah menyediakan
80% kebutuhan di sini. Cara termurah membangunnya adalah menjalankan modul CRM itu untuk **tenant
internal kita sendiri** — kita memakai produk kita sendiri untuk berjualan — lalu menambahkan yang
benar-benar khas program mitra: kode referral, komisi, dan pencairan.

### G.1 Dua jenis mitra, dua perjanjian berbeda

| | **Agen / reseller daerah** | **Freelance sales / afiliasi** |
|---|---|---|
| Cara kerja | Datang ke toko, demo, pasang printer, dampingi onboarding | Merujuk lewat kode/tautan, sebagian ikut demo |
| Kewajiban | Pendampingan & dukungan tingkat pertama di wilayahnya | Tidak ada kewajiban pendampingan |
| Komisi | Berulang, lebih besar, selama merchant aktif | Berulang lebih kecil, atau sekali dengan masa tertentu |
| Syarat masuk | Verifikasi identitas, perjanjian tertulis, pelatihan + sertifikasi | Verifikasi identitas, persetujuan syarat |
| Cocok untuk | Kota/kabupaten dengan banyak toko | Siapa saja: pegiat komunitas, konsultan, teman pedagang |

Keduanya memakai portal yang sama; yang membedakan hanya **tingkat (tier)** dan aturan komisinya.

### G.2 Aturan komisi — tempat kesalahan paling mahal

Enam aturan berikut saling mengunci. Melanggar satu saja membuat program ini merugi atau mengundang
kecurangan.

1. **Komisi berulang, bukan sekali di awal.** Mitra dibayar selama merchant masih berlangganan.
   Alasannya bukan kemurahan hati: churn adalah pembunuh utama model bisnis ini, dan komisi berulang
   membuat mitra ikut berkepentingan menjaga merchant tetap hidup.
2. **Dihitung dari pembayaran yang benar-benar diterima**, bukan dari nilai kontrak atau tagihan yang
   diterbitkan. Aturan yang sama persis dengan komisi sales lapangan di [E.3](#e3-crm-sales-lapangan--kanvasing--distribusi).
3. **Syarat aktivasi sebelum komisi pertama cair.** Merchant harus benar-benar dipakai — misalnya
   minimal 30 transaksi atau 30 hari aktif. Tanpa syarat ini, mitra akan mendaftarkan toko fiktif untuk
   mengejar bonus, dan angka pertumbuhan kita jadi bohong.
4. **Penarikan kembali (clawback).** Bila merchant menuntut pengembalian dana atau berhenti dalam masa
   tertentu, komisi yang telanjur dibayar ditarik dari pencairan berikutnya. Aturannya harus tertulis
   di perjanjian sejak awal, bukan diberitahukan saat pertama terjadi.
5. **Masa atribusi yang jelas.** Prospek yang didaftarkan mitra dan baru mendaftar berbulan-bulan
   kemudian tetap miliknya selama masih dalam masa atribusi (misal 60 hari sejak pendaftaran prospek).
   Lewat dari itu, atribusi gugur. Sengketa diputus manual oleh admin, dan keputusannya dicatat.
6. **Pajak dan bukti potong.** Komisi ke perorangan menimbulkan kewajiban pemotongan pajak. Simpan NPWP
   mitra, hasilkan bukti potong, dan pisahkan nilai bruto/neto di pencairan. Konfirmasikan perlakuannya
   ke konsultan pajak sebelum pencairan pertama — jangan mengarang sendiri.

### G.3 Batas yang tidak boleh dilewati

> **Satu tingkat saja. Tidak ada jaringan berjenjang.**
> Skema di mana mitra mendapat komisi dari hasil penjualan mitra yang direkrutnya — apalagi bertingkat
> ke bawah — menempatkan usaha ini pada wilayah penjualan langsung berjenjang yang perizinannya ketat,
> dan mengundang orang yang tertarik merekrut, bukan berjualan. Bonus pengenalan mitra baru boleh ada,
> tetapi **satu kali, nilainya tetap, dan tidak berlanjut ke bawah**.

Tiga batas lain yang perlu tertulis dalam perjanjian:

- **Harga resmi terkunci.** Mitra tidak boleh membanting harga sendiri; diskon hanya lewat kode promo
  resmi yang tercatat, supaya harga tidak berantakan antar wilayah.
- **Wilayah bukan hak eksklusif.** Menjanjikan monopoli wilayah akan menyandera pertumbuhan kita
  sendiri. Yang diberikan adalah prioritas prospek, bukan kepemilikan.
- **Hubungan langsung dengan merchant tetap ada.** Dukungan resmi datang dari kita, bukan hanya lewat
  mitra. Merchant yang hanya kenal agennya akan ikut pergi saat agennya pergi.

### G.4 Portal mitra — yang dilihat mereka

| Kebutuhan | Prioritas | Catatan |
|---|---|---|
| Dashboard: prospek, merchant aktif, komisi berjalan, komisi tertahan, jadwal pencairan | P0 | Pertanyaan pertama mitra selalu "kapan cair dan berapa" |
| Daftarkan prospek + kode/tautan referral pribadi | P0 | Kode dipakai saat merchant mendaftar sendiri |
| Daftar merchant binaan + status langganan + tanggal jatuh tempo | P0 | |
| **Kesehatan pemakaian merchant binaan** (aktif/tidak, terakhir bertransaksi) | P1 | Ini yang mengubah mitra dari penjual menjadi penjaga retensi |
| Rincian perhitungan komisi per merchant per periode | P0 | Komisi yang tidak bisa ditelusuri akan selalu diributkan |
| Riwayat pencairan + bukti transfer + bukti potong pajak | P0 | |
| Materi jualan: brosur, video, template pesan WhatsApp, daftar harga resmi | P1 | Mencegah mitra mengarang janji fitur sendiri |
| Pelatihan & sertifikasi singkat | P1 | Syarat naik tingkat; menjaga mutu pendampingan |
| Aktivitas & pengingat follow-up | P1 | Memakai mesin yang sama dengan [E.1](#e1-inti-crm-dipakai-kedua-persona) |
| Papan peringkat | P2 | **Berbasis merchant aktif, bukan jumlah pendaftaran** — kalau salah, ini justru memicu pendaftaran fiktif |
| Pengajuan sengketa atribusi | P2 | |

### G.5 Panel internal — yang dilihat kita

| Kebutuhan | Prioritas |
|---|---|
| Verifikasi & persetujuan mitra baru (identitas, perjanjian, rekening) | P0 |
| Aturan komisi per tingkat, dapat diubah tanpa menyentuh kode | P0 |
| Perhitungan komisi periodik + tinjauan sebelum pembayaran | P0 |
| Proses pencairan massal + rekonsiliasi dengan mutasi bank | P0 |
| Deteksi kejanggalan: merchant fiktif, pendaftaran beruntun dari satu perangkat, aktivasi palsu | P1 |
| Penyelesaian sengketa atribusi + jejak audit keputusan | P1 |
| Evaluasi tingkat mitra per kuartal | P2 |
| Laporan biaya akuisisi per mitra dan per wilayah | P1 |

### G.6 Alur kerja, dari rekrut sampai bayar

1. **Pendaftaran mitra** — isi data, unggah identitas, setujui perjanjian.
2. **Verifikasi** oleh admin: identitas, rekening, NPWP bila ada.
3. **Pelatihan & sertifikasi singkat** — wajib untuk tingkat agen, opsional untuk afiliasi.
4. **Aktivasi**: kode referral, tautan pendaftaran, dan materi jualan terbuka.
5. **Mendaftarkan prospek** — masuk pipeline, masa atribusi mulai berjalan.
6. **Merchant mendaftar** memakai kode/tautan → terhubung otomatis ke mitra.
7. **Onboarding & aktivasi merchant** — impor produk, pasang printer, transaksi pertama.
8. **Ambang aktivasi terpenuhi** → komisi mulai dihitung.
9. **Perhitungan periodik** → tinjauan admin → **pencairan** + bukti potong.
10. **Evaluasi tingkat** tiap kuartal berdasarkan merchant aktif dan retensinya, bukan jumlah pendaftaran.

### G.7 Tambahan model data

Seluruhnya di **lingkup platform**, sejajar dengan `plans` dan `subscriptions` — bukan di dalam tenant.

| Tabel | Isi penting |
|---|---|
| `partners` | jenis (agen/afiliasi), nama, wilayah, tingkat, status, rekening, NPWP, tanggal bergabung, pengenal mitra |
| `partner_users` | akun login mitra — satu mitra boleh punya beberapa orang |
| `partner_tiers` | tingkat + aturan komisi + syarat naik/turun |
| `partner_leads` | prospek yang didaftarkan mitra: nama usaha, kontak, kota, jenis usaha, status |
| `partner_referrals` | kaitan mitra ↔ tenant: kode yang dipakai, tanggal, masa atribusi, status atribusi |
| `partner_commissions` | perhitungan per periode per langganan: dasar, tarif, nilai, status (tertahan/disetujui/dibayar/ditarik) |
| `partner_payouts` | pencairan: periode, bruto, potongan pajak, neto, bukti transfer, status |
| `partner_targets` | target & pencapaian per periode |
| `partner_materials` | materi jualan berversi |
| `partner_trainings` | modul pelatihan & status sertifikasi |
| `partner_disputes` | sengketa atribusi + keputusan + alasan |

Penyesuaian pada tabel yang sudah ada:

- `tenants` mendapat `referred_by_partner_id` dan `referral_code_used` — **diisi sekali saat pendaftaran
  dan tidak boleh berubah**, karena inilah dasar seluruh perhitungan komisi.
- `subscription_invoices` menjadi sumber angka "yang benar-benar tertagih" untuk aturan komisi nomor 2.

### G.8 Otorisasi: mitra bukan tenant, dan tidak boleh melihat isi dagangan orang

Ini batas privasi yang paling penting di modul ini, dan paling mudah dilanggar tanpa sengaja.

- Mitra masuk lewat **jalur autentikasi terpisah** dengan perannya sendiri; mereka bukan user tenant
  mana pun dan tidak boleh bisa masuk ke aplikasi kasir merchant.
- Yang boleh dilihat mitra tentang merchant binaannya: **status langganan, tanggal jatuh tempo, dan
  penanda aktif/tidak aktif.**
- Yang **tidak** boleh dilihat: omzet, daftar produk, harga, pelanggan, dan isi transaksi apa pun.
  Merchant tidak pernah setuju datanya dibaca agen penjual.
- Semua akses mitra ke data merchant dicatat di jejak audit.
- Test kebocoran diperluas sekali lagi: selain lintas tenant dan lintas pemilik, uji juga bahwa akun
  mitra tidak bisa menyentuh data operasional tenant mana pun.

### G.9 Metrik program

Per mitra: merchant aktif · tingkat retensi merchant binaannya · biaya akuisisi efektif · rasio prospek
menjadi merchant · rata-rata waktu prospek → aktivasi.

Per program: porsi tenant baru yang datang dari mitra · perbandingan retensi merchant hasil mitra vs
pendaftaran mandiri · total komisi terhadap pendapatan · jumlah sengketa dan kecepatan penyelesaiannya.

> Metrik yang paling menentukan: **retensi merchant per mitra.** Mitra yang mendatangkan banyak merchant
> tetapi semuanya berhenti dalam tiga bulan sebenarnya menghabiskan uang kita, bukan menghasilkan.

### G.10 Risiko & mitigasi

| Risiko | Mitigasi |
|---|---|
| Merchant fiktif demi bonus | Syarat aktivasi berbasis pemakaian nyata + deteksi kejanggalan + audit acak |
| Mitra membanting harga | Harga resmi terkunci; diskon hanya lewat kode promo tercatat |
| Sengketa atribusi | Masa atribusi tertulis, catatan waktu yang jelas, keputusan admin yang terekam |
| Mitra membawa kabur merchant ke produk lain | Hubungan langsung dengan merchant; kualitas produk; komisi berulang yang membuat bertahan lebih menguntungkan |
| Ketergantungan pada segelintir mitra besar | Pantau sebaran; jangan biarkan satu mitra melebihi porsi tertentu dari tenant baru |
| Skema berjenjang merembet | Aturan satu tingkat dikunci di aturan komisi, bukan hanya di perjanjian |
| Beban administrasi pencairan | Otomatiskan perhitungan sejak awal; pencairan massal, bukan satu per satu |

### G.11 Kapan ini dibangun

**Jangan bangun portalnya untuk lima mitra pertama.** Kelola manual, pelajari sengketa yang benar-benar
muncul, dan biarkan aturan komisi diuji di dunia nyata sebelum dibekukan menjadi kode.

Pemicu untuk mulai membangun — mana yang lebih dulu tercapai:

- Mitra aktif lebih dari **15 orang**, atau
- Perhitungan komisi bulanan memakan lebih dari **satu hari kerja**, atau
- Muncul **sengketa atribusi kedua** yang tidak bisa diselesaikan dari catatan yang ada.

---

## Bagian H — Absensi & Penggajian Sederhana

Setiap UMKM yang punya karyawan sudah mengerjakan ini — dengan buku tulis, grup WhatsApp, dan hitungan
kalkulator di akhir bulan. Kesalahannya mahal bukan karena nilainya besar, melainkan karena menyangkut
gaji orang: **satu potongan yang salah merusak kepercayaan lebih cepat daripada bug apa pun di kasir.**

> **Catatan perubahan keputusan.** Di [B.5](#b5-yang-sengaja-tidak-dikerjakan-dulu), "Payroll / HR"
> semula ditandai jangan dikerjakan. Keputusan itu dicabut untuk lingkup **sederhana**. Yang tetap
> ditolak ada di [H.1](#h1-batas-cakupan--apa-yang-sengaja-tidak-dibangun).

Modul ini berdiri di atas yang sudah ada: `users` sudah menjadi karyawan, `outlets` sudah membawa zona
waktu dan batas hari usaha, shift kasir sudah mencatat siapa membuka dan menutup kas, dan komisi sales
sudah dihitung di [Bagian E](#bagian-e--modul-crm-freelance--sales-lapangan). Yang benar-benar baru
hanya catatan kehadiran, mesin aturan, dan slip gaji.

### H.1 Batas cakupan — apa yang sengaja tidak dibangun

| Dibangun | Tidak dibangun |
|---|---|
| Absensi masuk/pulang dari HP yang sudah dipakai kasir | Integrasi mesin sidik jari / wajah |
| Jadwal kerja sederhana per karyawan | Penjadwalan shift otomatis & optimasi tenaga kerja |
| Izin, sakit, cuti, alpa | Manajemen cuti berjenjang dengan kuota tahunan rumit |
| Gaji harian, mingguan, dua mingguan, bulanan | Struktur jabatan & jenjang karier |
| Komponen pendapatan & potongan yang bisa diatur sendiri | Bahasa formula bebas |
| Kasbon karyawan & cicilannya | Pinjaman berbunga |
| **Laporan** untuk pajak & BPJS | **Perhitungan** PPh 21 otomatis |
| Slip gaji yang bisa dicetak & dikirim WhatsApp | Manajemen kinerja, KPI, penilaian 360 |

Alasan penolakan perhitungan pajak sama dengan sikap di [B.3](#b3-kebutuhan-regulasi--konteks-indonesia):
tarif dan aturan berubah, dan salah hitung pajak orang lain adalah tanggung jawab yang tidak pantas
dipikul aplikasi kasir. Sediakan angkanya rapi, biarkan pemilik atau konsultannya yang menyetor.

### H.2 Absensi

| Kebutuhan | Prioritas | Catatan |
|---|---|---|
| Tombol masuk & pulang di perangkat yang sudah ada | P0 | Tidak ada UMKM yang mau beli mesin absen untuk 4 karyawan |
| Absensi otomatis dari buka/tutup shift kasir | P1 | Kasir yang membuka kas jelas sedang hadir — jangan suruh dia absen dua kali |
| Jadwal kerja per karyawan (jam masuk & pulang per hari) | P0 | Tanpa jadwal, "terlambat" tidak punya arti |
| Toleransi keterlambatan | P0 | Misal 10 menit, diatur per tenant |
| Absensi melewati tengah malam | P0 | Memakai `business_date` outlet, sama seperti transaksi kasir |
| Zona waktu outlet | P0 | Karyawan di Jayapura tidak boleh dinilai terlambat memakai jam Jakarta |
| Absensi saat internet mati | P0 | Masuk antrean lokal, sama seperti transaksi |
| Foto & titik lokasi saat absen | P2 | **Opsional per tenant** — lihat catatan privasi di bawah |
| Koreksi absensi oleh atasan | P0 | Wajib beralasan, tercatat di jejak audit, tidak menimpa baris asli |
| Rekap kehadiran per periode | P0 | Hadir, terlambat, lembur, izin, alpa |

> **Baris absensi tidak pernah diubah.** Koreksi selalu berupa baris baru yang menunjuk baris asal,
> lengkap dengan alasan dan siapa yang menyetujui. Ini syarat agar slip gaji bisa dipertanggungjawabkan
> saat karyawan bertanya "kenapa saya dipotong".

> **Privasi karyawan.** Foto dan titik lokasi hanya direkam **pada saat absen**, tidak pernah menjadi
> pelacakan sepanjang hari — aturan yang sama dengan kunjungan sales di [E.3](#e3-crm-sales-lapangan--kanvasing--distribusi).
> Fitur ini mati secara bawaan dan hanya menyala bila tenant menyalakannya serta memberi tahu karyawannya.

### H.3 Izin, sakit, cuti, dan alpa

| Jenis | Berbayar? | Catatan |
|---|---|---|
| Izin | Diatur tenant | Keperluan pribadi, biasanya potong |
| Sakit | Diatur tenant | Dengan atau tanpa surat dokter, sesuai kebijakan |
| Cuti | Ya, dengan kuota | Kuota sederhana per tahun, sisa terlihat karyawan |
| Libur/hari besar | Ya | Kalender libur per tenant, karena tiap usaha berbeda |
| Alpa | Tidak | Tanpa kabar — potongan terberat |

Alurnya sengaja pendek: karyawan mengajukan dari HP → atasan menyetujui atau menolak → status langsung
memengaruhi perhitungan gaji periode itu. Tidak ada persetujuan berlapis.

**Yang menentukan akurasi:** hari yang sudah disetujui sebagai izin/sakit/cuti **tidak boleh lagi**
dihitung sebagai alpa oleh mesin aturan. Urutan penentuan status harian ditetapkan tetap:

```
libur tenant  >  cuti/izin/sakit disetujui  >  ada absensi  >  alpa
```

### H.4 Komponen gaji

Gaji tersusun dari **baris komponen**, bukan satu angka. Inilah yang membuat setiap usaha bisa punya
aturannya sendiri tanpa mengubah kode.

| Kelompok | Contoh komponen |
|---|---|
| **Pendapatan dasar** | Gaji pokok (bulanan) atau upah harian × hari hadir |
| **Tunjangan** | Uang makan per hari hadir, transport, tunjangan jabatan |
| **Variabel** | Lembur, bonus kehadiran, bonus target, komisi penjualan |
| **Potongan** | Keterlambatan, alpa, cicilan kasbon, potongan lain bernama |
| **Wajib** | BPJS Ketenagakerjaan & Kesehatan porsi karyawan — bila tenant memakainya |

Setiap baris slip menyimpan **nama komponen, dasar hitung, dan hasilnya**, bukan hanya totalnya.
Karyawan yang bertanya "kenapa gaji saya berkurang 30 ribu" harus bisa dijawab dari slip itu sendiri,
tanpa membuka aplikasi.

### H.5 Mesin aturan dinamis

Ini inti permintaan "dinamis tergantung kebijakan usaha masing-masing", dan sekaligus tempat modul
seperti ini biasanya menjadi rumit tak terkendali.

**Keputusan rancangan: katalog aturan berparameter, bukan bahasa formula bebas.**

| Tipe aturan | Parameter | Contoh nyata |
|---|---|---|
| `tunjangan_tetap` | nominal, periode (harian/bulanan), syarat hadir | Uang makan Rp15.000 per hari hadir |
| `potongan_telat` | ambang menit, cara hitung (per menit / per kejadian / bertingkat), nominal | Lewat 15 menit → potong Rp10.000 |
| `potongan_alpa` | nominal atau persen upah harian | Alpa → potong satu hari upah |
| `upah_lembur` | ambang jam mulai, tarif per jam atau pengali upah/jam | Setelah jam ke-8 → 1,5× upah per jam |
| `bonus_kehadiran` | syarat (alpa = 0, telat ≤ N), nominal | Hadir penuh sebulan → Rp200.000 |
| `bonus_target` | dasar (omzet outlet / penjualan pribadi), ambang, persen atau nominal | Omzet outlet > Rp50 juta → bonus 1% |
| `potongan_kasbon` | nominal cicilan per periode | Otomatis dari saldo kasbon |
| `komponen_manual` | nama, nominal, alasan | THR, bonus lebaran, potongan kerusakan |

Enam aturan yang menjaga mesin ini tetap sederhana **dan** akurat:

1. **Aturan punya masa berlaku** (`berlaku_dari`, `berlaku_sampai`), tidak pernah disunting di tempat.
   Mengubah kebijakan berarti menutup aturan lama dan membuka yang baru — sehingga slip bulan lalu tetap
   bisa dihitung ulang dan menghasilkan angka yang sama persis.
2. **Aturan bisa dipasang ke semua karyawan, satu peran, atau satu orang.** Cukup tiga tingkat; jangan
   tambahkan grup di atas grup.
3. **Urutan hitung ditetapkan tetap:** pendapatan dasar → tunjangan → lembur → bonus → potongan →
   pembulatan. Urutan yang bisa diatur pengguna adalah pintu masuk kerumitan tanpa ujung.
4. **Setiap baris slip menyimpan `rule_id` dan salinan parameternya.** Kalau tarif lembur berubah tahun
   depan, slip tahun ini tetap bisa dijelaskan.
5. **Potongan tidak boleh membuat gaji bersih negatif.** Bila total potongan melebihi pendapatan,
   sisanya menjadi utang karyawan yang terbawa ke periode berikutnya — bukan angka minus di slip.
6. **Kebutuhan di luar katalog diselesaikan dengan `komponen_manual`**, bukan dengan menambah bahasa
   formula. Bila satu jenis komponen manual dipakai berulang oleh banyak tenant, itulah tanda ia layak
   naik menjadi tipe aturan baru — keputusan produk, bukan keputusan pengguna.

### H.6 Siklus penggajian & penguncian

```
1. Tentukan periode      harian / mingguan / dua mingguan / bulanan, diatur per karyawan
2. Tarik data            absensi, izin, lembur, komisi, saldo kasbon
3. Hitung                jalankan aturan yang berlaku pada periode itu
4. Tinjau                pemilik melihat rincian per karyawan, boleh menambah komponen manual
5. KUNCI                 slip dibekukan — angka tidak berubah lagi selamanya
6. Bayar                 tandai terbayar; menimbulkan kas keluar di modul kas
7. Kirim slip            cetak atau kirim WhatsApp
```

**Setelah dikunci, slip tidak pernah dihitung ulang.** Koreksi yang datang belakangan — absensi salah,
lembur terlewat — masuk sebagai **penyesuaian di periode berikutnya** dengan keterangan periode asalnya.
Ini aturan yang sama dengan pembalikan transaksi kasir di [Bagian D](#bagian-d--model-data-inti): baris
lama tidak diubah, koreksi selalu berupa baris baru.

Pembayaran gaji menimbulkan **kas keluar** di modul kas, sehingga arus kas pemilik tetap utuh tanpa
pencatatan ganda.

### H.7 Kasbon karyawan

Sangat umum di UMKM dan hampir selalu dicatat di buku terpisah yang akhirnya hilang.

- Pengajuan kasbon → persetujuan → pencairan (kas keluar).
- Saldo dicicil otomatis pada setiap penggajian sebesar nominal yang disepakati.
- Karyawan berhenti dengan saldo tersisa → sisa muncul di penyelesaian akhir, tidak menguap.
- Kasbon **tidak berbunga**. Menambahkan bunga mengubah usaha ini menjadi pemberi pinjaman, dengan
  konsekuensi hukum yang tidak diinginkan.

### H.8 Kepatuhan & kehati-hatian

Sikapnya sama dengan pajak di [B.3](#b3-kebutuhan-regulasi--konteks-indonesia): **sediakan catatan yang
rapi, jangan menjadi penasihat.**

| Hal | Yang disediakan sistem |
|---|---|
| Upah minimum daerah | Peringatan bila upah bulanan di bawah nilai yang **diisi tenant sendiri**. Jangan menanam angka UMP di kode — nilainya berbeda tiap daerah dan berubah tiap tahun |
| Lembur | Aturan lembur sebagai konfigurasi, dengan tarif yang diisi tenant |
| THR | Komponen manual dengan pengingat menjelang hari raya, bukan perhitungan otomatis |
| BPJS | Komponen potongan & iuran sebagai konfigurasi, plus laporan rekap |
| PPh 21 | **Hanya laporan** penghasilan bruto per karyawan per tahun, untuk diserahkan ke konsultan pajak |
| Data pribadi karyawan | Tunduk UU PDP: NIK, rekening, dan foto absensi adalah data pribadi. Simpan seperlunya, batasi aksesnya, dan hapus atas permintaan setelah kewajiban penyimpanan selesai |

> Cantumkan di antarmuka bahwa perhitungan mengikuti kebijakan yang **diisi tenant**, dan aplikasi tidak
> memberi nasihat ketenagakerjaan. Ini melindungi kedua pihak.

### H.9 Tambahan model data

| Tabel | Isi penting |
|---|---|
| `employees` | perluasan `users`: tanggal masuk, jenis upah (harian/bulanan), nominal dasar, jabatan, status kerja, NIK, rekening |
| `work_schedules` | jadwal per karyawan: hari dalam minggu, jam masuk, jam pulang, toleransi |
| `attendances` | satu baris per kejadian absen: jenis (masuk/pulang), waktu, outlet, sumber (manual/shift/koreksi), foto, lokasi, `business_date` |
| `attendance_corrections` | koreksi yang menunjuk baris asal + alasan + penyetuju |
| `leave_requests` | jenis (izin/sakit/cuti), tanggal, status, penyetuju, lampiran |
| `leave_balances` | kuota cuti per karyawan per tahun |
| `holidays` | kalender libur per tenant |
| `payroll_rules` | tipe aturan + parameter + masa berlaku + sasaran (semua/peran/karyawan) |
| `payroll_periods` | periode penggajian, status (draft/terkunci/terbayar) |
| `payslips` | slip per karyawan per periode: bruto, potongan, neto, status |
| `payslip_lines` | baris komponen: nama, `rule_id`, salinan parameter, dasar hitung, nominal |
| `employee_advances` | kasbon: nominal, saldo, cicilan per periode |
| `advance_repayments` | pembayaran cicilan, terhubung ke slip |

Penyesuaian tabel yang sudah ada:

- `users` mendapat kaitan satu-ke-satu ke `employees` — akun aplikasi dan data kepegawaian dipisah,
  karena ada karyawan yang tidak punya akun (misalnya juru masak) dan ada akun yang bukan karyawan.
- `cash_movements` menerima referensi ke `payroll_periods` untuk pembayaran gaji dan ke
  `employee_advances` untuk pencairan kasbon.
- Izin baru: `hr.attendance.view`, `hr.attendance.correct`, `hr.leave.approve`, `hr.payroll.run`,
  `hr.payroll.lock`, `hr.salary.view` — **`hr.salary.view` adalah izin paling sensitif di seluruh
  produk**; secara bawaan hanya pemilik yang memilikinya.

### H.10 Laporan

Rekap kehadiran per periode · keterlambatan per karyawan · rekap lembur · biaya gaji per outlet dan
per periode · **porsi biaya gaji terhadap omzet** (angka yang paling sering mengejutkan pemilik) ·
saldo kasbon berjalan · rekap penghasilan tahunan per karyawan untuk keperluan pajak.

### H.11 Risiko

| Risiko | Mitigasi |
|---|---|
| Salah hitung gaji | Slip dikunci dan bisa direproduksi; setiap baris menyimpan dasar hitungnya; uji dengan kasus nyata sebelum rilis |
| Absensi dititipkan ke teman | Foto & lokasi opsional; absensi otomatis dari shift kasir; laporan kejanggalan pola |
| Modul menjadi rumit tak terkendali | Katalog aturan tertutup; kebutuhan aneh diselesaikan lewat komponen manual |
| Kebocoran data gaji antar karyawan | `hr.salary.view` dibatasi ketat; karyawan hanya melihat slipnya sendiri; setiap akses tercatat |
| Tenant menuntut fitur payroll perusahaan | Batas cakupan di [H.1](#h1-batas-cakupan--apa-yang-sengaja-tidak-dibangun) ditulis agar bisa ditunjukkan, bukan diperdebatkan ulang |
| Sengketa ketenagakerjaan menyeret kita | Aplikasi mencatat kebijakan tenant, tidak memberi nasihat; jejak audit lengkap |

---

## Bagian I — Roadmap Bertahap

Estimasi mengasumsikan **satu pengembang penuh waktu**. Skalakan sesuai tim. Setiap fase harus
berakhir dengan sesuatu yang bisa didemokan.

### Fase 0 — Membayar utang teknis fondasi (2–3 minggu)

Kerjakan sebelum satu tabel transaksi pun dibuat. Semua item diambil dari daftar utang teknis di
[CONVENTIONS.md](../CONVENTIONS.md) bagian 7.

| Pekerjaan | Alasan tidak bisa ditunda |
|---|---|
| Ganti `AutoMigrate` dengan migrasi berversi (`golang-migrate` / `goose`) | Setelah ada data pelanggan, perubahan skema tanpa migrasi berversi berbahaya |
| Endpoint `/health` (liveness + readiness) | Dibutuhkan load balancer sejak deploy pertama |
| Test untuk auth, repository, middleware | Cakupan test masih tipis; menambah modul di atas fondasi tak teruji akan berlipat biayanya |
| Refresh token + pencabutan token (jti) | Perangkat kasir hilang/dicuri harus bisa diputus aksesnya |
| Logging terstruktur + request ID | Tanpa ini, menelusuri keluhan pelanggan mustahil |
| Rate limiter lintas instance (Redis) | Limiter in-memory tidak berlaku saat instance lebih dari satu |
| Soft delete + partial unique index | Data keuangan tidak boleh hilang permanen |

**Selesai bila:** pipeline migrasi jalan, `/health` hijau, `go build ./... && go vet ./... && gofmt -l .` bersih, dan test auth lulus.

### Fase 1 — Multi-tenancy & RBAC (2–3 minggu)

- Tabel `tenants`, `outlets`, penyesuaian `users`/`roles`.
- Pendaftaran tenant mandiri: satu permintaan membuat tenant + outlet + user Owner + peran bawaan.
- JWT membawa `tenant_id` **dari database**, tidak pernah dari input klien.
- Helper `scopeTenant` wajib di repository + Row Level Security PostgreSQL.
- `RequirePermission(...)` menggantikan pengecekan berbasis nama role.
- **Test kebocoran tenant** (dua tenant, lintas akses ditolak di semua endpoint).

**Selesai bila:** dua tenant hidup berdampingan dan tidak ada satu pun jalur yang membocorkan data antar tenant.

### Fase 2 — Master data (2 minggu)

- Kategori, satuan, produk, varian, harga bertingkat, supplier, pelanggan.
- Impor Excel/CSV dengan pratinjau dan laporan baris gagal.
- Ikuti resep di [CONVENTIONS.md](../CONVENTIONS.md) bagian 6 untuk tiap resource.

**Selesai bila:** 500 produk bisa diimpor, dicari (< 200 ms), dan disunting.

### Fase 3 — Kasir & transaksi (3–4 minggu) — *fase paling kritis*

- Checkout dalam satu `DB.Transaction` (lihat [Bagian D](#bagian-d--model-data-inti)).
- Idempotency key, snapshot harga, penomoran struk per outlet.
- Diskon, pajak, pembayaran gabungan, kembalian.
- Batal & retur dengan jejak audit dan pembalikan stok.
- Shift kasir: buka/tutup kas + rekonsiliasi.
- Cetak struk ESC/POS 58/80 mm + struk PDF.

**Selesai bila:** 100 transaksi berurutan menghasilkan stok, kas, dan laporan yang cocok sampai rupiah terakhir.

### Fase 4 — Stok & pembelian (2 minggu)

- `stock_movements` sebagai buku besar, `stocks` sebagai cache + perintah rekonsiliasi.
- Pembelian, penyesuaian, kartu stok, peringatan stok menipis.

**Selesai bila:** menghitung ulang `stocks` dari `stock_movements` menghasilkan angka yang sama persis.

### Fase 5 — Laporan & dashboard (2 minggu)

- Dashboard harian, laporan penjualan/produk/laba/kas/stok, ekspor Excel & PDF.
- Query laporan berat memakai tabel ringkasan harian (`daily_sales_summary`) yang diperbarui saat
  transaksi ditulis. Jangan pernah menghitung `SUM` seluruh tabel penjualan saat pemilik membuka dashboard.

**Selesai bila:** dashboard tampil < 1 detik pada data 100.000 transaksi.

### Fase 6 — Client offline-first (3–4 minggu)

- PWA kasir: IndexedDB, antrean sinkronisasi, indikator status, penyelesaian sesuai [C.5](#c5-offline-first--sinkronisasi).
- Uji dengan mematikan jaringan di tengah transaksi, menutup paksa aplikasi, dan mengirim ulang antrean.

**Selesai bila:** perangkat bisa berjualan 8 jam tanpa internet lalu tersinkron utuh tanpa duplikat.

### Fase 7 — Billing & operasi SaaS (2–3 minggu)

- Paket, langganan, trial, kuota, penurunan fitur saat langganan lewat tempo
  (**jangan kunci total** — pemilik tetap harus bisa membaca dan mengekspor datanya).
- **Masa langganan 1/3/6/9/12 bulan** dengan tangga diskon, prorata saat naik paket, dan perhitungan
  ulang saat batal di tengah masa.
- **Pendapatan diterima di muka** dicatat terpisah dan diakui bulanan — jangan mengakui 12 bulan
  sekaligus, karena laporan laba akan bohong dan kas terpakai untuk bulan yang belum dilayani.
- Faktur, pembayaran, pengingat via WhatsApp/email.
- Panel admin platform: daftar tenant, pemakaian, dukungan.

**Selesai bila:** satu tenant bisa daftar, trial, membayar untuk masa berapa pun, dan diperpanjang tanpa
campur tangan manual; dan pembatalan di tengah masa prabayar menghasilkan angka pengembalian yang sama
persis dengan hitungan tangan.

### Fase 8 — Penguatan & pembeda (berkelanjutan)

Integrasi QRIS, modul F&B (meja, dapur, open bill), multi-outlet lanjutan, program loyalitas, modul
jasa, aplikasi pendamping cetak, dan modul grosir.

### Fase 9 — CRM inti + Freelance (3–4 minggu)

Spesifikasinya di [Bagian E](#bagian-e--modul-crm-freelance--sales-lapangan).

- Kontak, sumber prospek, pipeline yang bisa diatur, deal, aktivitas & pengingat.
- Penawaran → proyek → invoice bertermin → pembayaran parsial.
- Pelunasan invoice tercatat sebagai penjualan di tabel yang sama dengan POS.
- `owner_id` + `scopeVisibility` di repository, izin CRM baru, test kebocoran lintas pemilik.
- Pengingat follow-up & jatuh tempo lewat `outbox`.

**Selesai bila:** satu freelancer bisa menjalankan siklus penuh — prospek masuk, penawaran, DP,
pelunasan — dan angkanya muncul di laporan omzet yang sama dengan POS, tanpa entri ganda.

### Fase 10 — CRM Sales Lapangan (3–4 minggu)

- Rencana kunjungan, check-in/out dengan GPS + foto, ambil pesanan di lokasi.
- Kunjungan tanpa pesanan + alasan, penagihan lapangan, stok kanvas sebagai outlet bertipe kendaraan.
- Target, komisi berbasis tagihan tertagih, rekap harian ke supervisor.
- Antrean offline untuk foto & GPS, dengan kompresi di perangkat.

**Selesai bila:** seorang sales bisa menyelesaikan rute 20 toko tanpa sinyal, lalu seluruh kunjungan,
pesanan, dan foto tersinkron utuh tanpa duplikat.

### Fase 11 — Kanal pesanan online (3–4 minggu + 1–2 minggu per kanal)

Spesifikasinya di [Bagian F](#bagian-f--kanal-pesanan-online-marketplace--aplikator). Dikerjakan dalam
dua langkah karena hanya langkah kedua yang bergantung pada persetujuan pihak lain.

**11a — Fondasi multi-kanal (3–4 minggu, tanpa ketergantungan eksternal)**

- `channels`, `channel_products`, harga per kanal, status pesanan yang diperluas pada `sales`.
- Entri pesanan manual bertanda kanal (WhatsApp, Instagram) dan impor laporan harian kanal dari CSV.
- Antrean pesanan gabungan di layar kasir + cetak dapur.
- Laporan penjualan dan **laba bersih per kanal setelah komisi**.

**11b — Adaptor API per kanal (1–2 minggu per kanal, setelah kemitraan disetujui)**

- `channel_events` + pekerja asinkron, idempoten berdasarkan `external_order_id`.
- Sinkronisasi stok dengan penyangga, indikator keterlambatan, antrean mati yang bisa dilihat pemilik.
- Rekonsiliasi pencairan dana dan pencatatan komisi.

**Selesai bila:** satu pesanan dari kanal mana pun — termasuk yang dikirim ulang dua kali oleh
kanalnya — menghasilkan tepat satu penjualan, satu pergerakan stok, dan muncul di laporan yang sama
dengan penjualan kasir; dan mematikan satu kanal tidak mengganggu operasi kasir sama sekali.

### Fase 12 — Program mitra penjual (2–3 minggu)

Spesifikasinya di [Bagian G](#bagian-g--program-mitra-penjual-agen--freelance-sales). **Jangan
dikerjakan sebelum pemicunya tercapai** — lihat [G.11](#g11-kapan-ini-dibangun); lima mitra pertama
dikelola manual.

- `partners`, `partner_referrals`, kode referral pada pendaftaran tenant (diisi sekali, tidak berubah).
- Mesin komisi: berulang, atas yang tertagih, dengan syarat aktivasi dan penarikan kembali.
- Portal mitra: prospek, merchant binaan, rincian komisi, riwayat pencairan, materi jualan.
- Panel internal: verifikasi mitra, tinjauan komisi, pencairan massal, penyelesaian sengketa.
- Jalur autentikasi mitra terpisah + batas data: mitra hanya melihat status langganan merchant
  binaannya, tidak pernah isi transaksinya.

**Selesai bila:** satu siklus penuh berjalan tanpa perhitungan manual — mitra mendaftarkan prospek,
merchant aktif, komisi terhitung sendiri, pencairan terbit dengan bukti potong, dan akun mitra terbukti
tidak bisa menyentuh data operasional tenant mana pun.

### Fase 13 — Absensi & penggajian (3–4 minggu)

Spesifikasinya di [Bagian H](#bagian-h--absensi--penggajian-sederhana).

- `employees`, `work_schedules`, `attendances` + koreksi beralasan, `leave_requests`, `holidays`.
- Absensi otomatis dari buka/tutup shift kasir, dan antrean offline seperti transaksi.
- Mesin aturan berparameter (`payroll_rules`) dengan masa berlaku, bukan bahasa formula bebas.
- Siklus penggajian: hitung → tinjau → **kunci** → bayar (kas keluar) → kirim slip.
- Kasbon karyawan dan cicilannya.
- Izin `hr.salary.view` dibatasi ketat sejak hari pertama.

**Selesai bila:** satu periode gaji dihitung, dikunci, dan dibayar; menghitung ulang periode yang sudah
dikunci menghasilkan angka yang sama persis; dan koreksi absensi yang datang terlambat muncul sebagai
penyesuaian di periode berikutnya, bukan mengubah slip lama.

### Ringkasan waktu

| Fase | Durasi | Kumulatif |
|---|---|---|
| 0 — Fondasi | 2–3 mgg | ~3 mgg |
| 1 — Tenancy & RBAC | 2–3 mgg | ~6 mgg |
| 2 — Master data | 2 mgg | ~8 mgg |
| 3 — Kasir | 3–4 mgg | ~12 mgg |
| 4 — Stok | 2 mgg | ~14 mgg |
| 5 — Laporan | 2 mgg | ~16 mgg |
| 6 — Offline | 3–4 mgg | ~20 mgg |
| 7 — Billing | 2–3 mgg | **~23 mgg (≈5–6 bulan) siap jual** |
| 9 — CRM inti + Freelance | 3–4 mgg | ~27 mgg *(modul add-on)* |
| 10 — CRM Sales Lapangan | 3–4 mgg | ~31 mgg *(modul add-on)* |
| 11a — Fondasi multi-kanal | 3–4 mgg | ~35 mgg *(modul add-on)* |
| 11b — Adaptor API per kanal | 1–2 mgg/kanal | mengikuti kemitraan |
| 12 — Program mitra penjual | 2–3 mgg | mengikuti pemicu di [G.11](#g11-kapan-ini-dibangun) |
| 13 — Absensi & penggajian | 3–4 mgg | modul add-on |

Fase 12 tidak punya posisi tetap dalam urutan: ia dikerjakan saat jumlah mitra membuat pengelolaan
manual tidak lagi masuk akal — bisa lebih awal maupun lebih lambat dari nomornya.

Fase 13 layak dinaikkan lebih awal dari nomornya bila pelanggan awal banyak yang punya karyawan:
absensi dan gaji adalah modul yang paling melekat, dan tenant yang riwayat gajinya sudah ada di sini
jauh lebih kecil kemungkinannya pindah aplikasi.

Fase 8 berjalan berkelanjutan dan tidak masuk hitungan kumulatif. Fase 9–11 adalah modul add-on yang
urutannya mengikuti permintaan pasar, bukan nomornya: bila pelanggan awal didominasi F&B yang sudah
berjualan di aplikator, **Fase 11a layak dinaikkan mendahului Fase 9–10**. Semuanya tetap menunggu
Fase 1 (tenancy & RBAC), Fase 2 (master data), dan Fase 4 (buku besar stok) selesai — khusus Fase 11,
sinkronisasi stok tidak punya arti tanpa Fase 4.

**Uji pasar lebih awal:** setelah Fase 5 (~4 bulan) produk sudah bisa dipakai 5–10 toko percontohan
secara gratis. Masukan dari mereka lebih berharga daripada menebak fitur Fase 6–8.

---

## Bagian J — Kebutuhan Non-Fungsional

| Aspek | Target | Cara mencapainya |
|---|---|---|
| **Waktu respons** | Cari produk < 200 ms, checkout < 500 ms, laporan < 1 dtk | Index dari `tenant_id`, tabel ringkasan, hindari N+1 (pakai `Joins`, bukan `Preload`) |
| **Ketersediaan** | 99,5% pada tahun pertama | Client offline-first membuat gangguan server tidak menghentikan penjualan |
| **Cadangan** | Backup otomatis harian + PITR, simpan 30 hari | **Uji pemulihan setiap bulan.** Backup yang belum pernah diuji bukan backup |
| **Keamanan** | Sesuai bagian 4 CONVENTIONS + isolasi tenant | Enkripsi saat transit (TLS), hash password bcrypt, jejak audit, prinsip hak akses minimum, rahasia tidak pernah masuk git |
| **Observabilitas** | Tahu ada masalah sebelum pelanggan menelepon | Log terstruktur + request ID, metrik (Prometheus), pelacakan error (Sentry), peringatan pada tingkat error & antrean sinkronisasi |
| **Skalabilitas** | 1.000 tenant di satu instans PostgreSQL yang wajar | Partisi tabel `sales`/`stock_movements` per bulan bila sudah besar |
| **Kepatuhan** | UU PDP | Kebijakan privasi, penghapusan data atas permintaan, catatan pemrosesan |
| **Pemulihan bencana** | RPO < 1 jam, RTO < 4 jam | Backup lintas region + prosedur pemulihan yang tertulis dan pernah dijalankan |

---

## Bagian K — Model Bisnis SaaS

### Struktur paket (contoh, sesuaikan dengan riset harga)

| | **Gratis** | **Basic** | **Pro** | **Multi-Outlet** |
|---|---|---|---|---|
| Outlet | 1 | 1 | 1 | 3+ |
| Pengguna | 1 | 3 | 10 | tak terbatas |
| Produk | 50 | tak terbatas | tak terbatas | tak terbatas |
| Transaksi/bulan | 300 | tak terbatas | tak terbatas | tak terbatas |
| Laporan lanjutan | – | dasar | penuh | penuh |
| Ekspor data | ✓ | ✓ | ✓ | ✓ |
| Mode offline | ✓ | ✓ | ✓ | ✓ |
| Integrasi QRIS | – | – | ✓ | ✓ |
| Modul CRM Freelance | add-on | add-on | ✓ | ✓ |
| Modul CRM Sales Lapangan | – | add-on | add-on | ✓ |
| Kanal pesanan online | – | add-on | add-on | ✓ |
| Absensi & penggajian | – | add-on | add-on | add-on |

Modul absensi & penggajian ditagih **per karyawan aktif** dengan batas bawah, karena nilainya tumbuh
mengikuti jumlah karyawan dan usaha berkaryawan dua orang tidak boleh membayar sama dengan yang
berkaryawan dua puluh.

Modul kanal online ditagih per kanal aktif — biaya melayaninya nyata (pekerja sinkronisasi, penyimpanan
muatan mentah, pemantauan) dan bertambah seiring jumlah kanal, sehingga harga flat akan merugikan.

Modul CRM dijual sebagai add-on berbayar terpisah, bukan dibundel ke paket POS: penggunanya berbeda
(freelancer tanpa toko, atau tim sales dengan tarif per pengguna), dan mencampurnya ke paket POS akan
menaikkan harga bagi mayoritas pelanggan yang tidak membutuhkannya.

Prinsip penetapan paket: **mode offline dan ekspor data ada di semua paket.** Menyandera keduanya
merusak kepercayaan pada segmen yang justru paling sensitif soal itu.

### Diskon langganan dibayar di muka

Tenant boleh membayar di muka untuk 3, 6, 9, atau 12 bulan, dengan lompatan diskon terbesar di 12 bulan
supaya ke sanalah orang diarahkan.

| Masa | Diskon | Basic (Rp79.000/bln) | Pro (Rp199.000/bln) |
|---|---|---|---|
| Bulanan | — | Rp79.000 | Rp199.000 |
| 3 bulan | 5% | Rp225.000 | Rp565.000 |
| 6 bulan | 10% | Rp425.000 | Rp1.075.000 |
| 9 bulan | 12,5% | Rp620.000 | Rp1.565.000 |
| 12 bulan | 16,7% | Rp790.000 | Rp1.990.000 (**2 bulan gratis**) |

Diskon ini menguntungkan selama churn bulanan masih di atas ±3%: pada churn 5%, tenant bulanan
rata-rata hanya membayar ±8,7 bulan dari 12 bulan ke depan, sedangkan prabayar setahun memberi 10 bulan
tunai hari ini. Perhitungan lengkapnya di
[BUSINESS-MODEL-CANVAS.md](BUSINESS-MODEL-CANVAS.md#2-rincian-pendapatan) — tinjau ulang tangga ini
setiap kali churn bergerak lebih dari 1 poin.

Empat aturan yang wajib ikut diterapkan di kode, bukan hanya di brosur:

1. **Pendapatan diterima di muka diakui bulanan**, bukan sekaligus saat uang masuk.
2. **Komisi mitra dibayar bertahap per bulan berjalan**, meski pembayaran diterima sekaligus — melindungi
   kas dan menjaga penarikan kembali tetap mungkin.
3. **Pembatalan di tengah masa dihitung ulang pada harga bulanan normal**, baru sisanya dikembalikan.
4. **Naik paket di tengah masa memakai prorata**, sisa nilai menjadi kredit.

### Metrik yang dipantau sejak tenant pertama

- **Aktivasi:** berapa persen tenant yang menyelesaikan transaksi pertama dalam 24 jam pertama.
  Ini metrik terpenting di awal — impor produk dan onboarding menentukannya.
- **Retensi:** tenant yang masih bertransaksi di minggu ke-4 dan ke-12.
- **MRR, churn bulanan, biaya infrastruktur per tenant.**
- **Kesehatan pemakaian:** transaksi per tenant per hari (turun tajam = akan berhenti berlangganan).

### Jalur masuk pasar

Komunitas UMKM lokal, koperasi, dan asosiasi pedagang lebih efektif daripada iklan digital untuk
segmen ini. Pendampingan langsung pada 10 pelanggan pertama (impor data, atur printer) menghasilkan
retensi jauh lebih tinggi daripada onboarding mandiri sepenuhnya.

Skalanya kemudian bertumpu pada agen dan freelance sales daerah. Perkakas untuk mengelola mereka —
komisi, atribusi, dan pencairan — dirancang di
[Bagian G](#bagian-g--program-mitra-penjual-agen--freelance-sales), sedangkan model bisnisnya diuraikan
di [BUSINESS-MODEL-CANVAS.md](BUSINESS-MODEL-CANVAS.md).

---

## Bagian L — Operasional & Checklist Go-Live

### Infrastruktur minimum

- Satu VPS (2 vCPU / 4 GB) untuk API + PostgreSQL terkelola — cukup untuk ratusan tenant pertama.
- Reverse proxy (Caddy/Nginx) dengan TLS otomatis; set `TRUSTED_PROXIES` sesuai [.env.example](../.env.example).
- Object storage untuk foto produk & lampiran.
- CI: `go build ./... && go vet ./... && gofmt -l . && go test ./...` pada setiap push.

### Checklist sebelum tenant berbayar pertama

- [ ] `APP_ENV=production`, `APP_URL` terisi, `JWT_SECRET` acak ≥ 32 karakter
- [ ] `DB_SSLMODE=require` (atau `verify-full`), kredensial tidak ada di git
- [ ] `ALLOWED_ORIGINS` hanya domain produksi
- [ ] Migrasi berversi berjalan otomatis saat deploy, `AutoMigrate` sudah dimatikan
- [ ] Backup otomatis aktif **dan pemulihannya sudah pernah diuji**
- [ ] `/health` terhubung ke load balancer & monitor uptime
- [ ] Pelacakan error dan peringatan (alert) aktif
- [ ] Test kebocoran tenant lulus di CI
- [ ] Jejak audit aktif untuk void, diskon, ubah harga, hapus data
- [ ] Rate limit aktif di semua endpoint publik
- [ ] Kebijakan Privasi & Syarat Layanan terbit
- [ ] Jalur dukungan pelanggan siap (WhatsApp) + SLA balasan tertulis
- [ ] Prosedur ekspor data tenant terdokumentasi
- [ ] Runbook insiden: siapa dihubungi, langkah pemulihan, cara komunikasi ke pelanggan

---

## Bagian M — Langkah Konkret Berikutnya di Repo Ini

Tiga pekerjaan pertama, berurutan, dimulai dari kode yang sudah ada:

**1. Migrasi berversi (menggantikan `AutoMigrate`)**
Tambahkan `golang-migrate`, pindahkan skema `users` & `roles` ke berkas migrasi awal, matikan
`AutoMigrate` di [database/database.go](../database/database.go), jalankan migrasi saat startup atau lewat perintah terpisah.

**2. Tenant & outlet**
Ikuti resep [CONVENTIONS.md](../CONVENTIONS.md) bagian 6 untuk `models/tenant.go` dan `models/outlet.go`, tambahkan
`tenant_id` ke `users` dan `roles`, lalu buat helper `scopeTenant` di [repositories/helper.go](../repositories/helper.go) dan
middleware yang menaruh `tenant_id` ke dalam context dari klaim JWT (diambil ulang dari database, bukan dari input).

**3. Test kebocoran tenant**
Sebelum menambah tabel bisnis apa pun: test yang membuat dua tenant dan memastikan semua endpoint
menolak akses lintas tenant. Test ini akan terus menjaga setiap fase berikutnya.

Setelah tiga langkah itu selesai, sisa roadmap tinggal mengulang resep yang sama untuk tiap resource baru.

---

*Dokumen hidup — perbarui saat keputusan berubah. Pasangannya: [README.md](../README.md) untuk cara menjalankan,
[CONVENTIONS.md](../CONVENTIONS.md) untuk aturan menulis kode.*
