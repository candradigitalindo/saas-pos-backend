# Business Model Canvas — SaaS POS UMKM

Model bisnis di balik produk yang dirancang di [BLUEPRINT-SAAS-POS.md](BLUEPRINT-SAAS-POS.md).
Blueprint menjawab *apa yang dibangun*; dokumen ini menjawab *bagaimana usahanya hidup*.

> **Semua angka di dokumen ini adalah asumsi kerja, bukan hasil riset pasar.** Angka ditulis eksplisit
> supaya bisa dibantah dan diuji, bukan supaya dipercaya. Setiap asumsi punya cara pembuktiannya di
> [bagian 8](#8-asumsi-yang-harus-dibuktikan).

**Kalimat model bisnis dalam satu tarikan napas:**
Menjual langganan bulanan aplikasi kasir kepada UMKM Indonesia yang selama ini mencatat manual,
lewat komunitas dan agen daerah alih-alih iklan, dengan biaya melayani yang sangat rendah karena
satu binary Go melayani ribuan tenant — lalu memperdalam pendapatan lewat modul add-on dan bagi
hasil pembayaran digital setelah kepercayaan terbentuk.

**Beachhead:** toko retail kecil satu outlet, pemilik yang sudah pakai HP Android dan sudah merasa
kesulitan menghitung stok serta laba harian. Bukan warung yang belum pegang smartphone, bukan
jaringan waralaba.

---

## Daftar Isi

- [1. Kanvas — sembilan blok](#1-kanvas--sembilan-blok)
- [2. Rincian pendapatan](#2-rincian-pendapatan)
- [3. Struktur biaya](#3-struktur-biaya)
- [4. Unit economics](#4-unit-economics)
- [5. Modul yang mengunci: absensi & penggajian](#5-modul-yang-mengunci-absensi--penggajian)
- [6. Ekonomi program mitra penjual](#6-ekonomi-program-mitra-penjual)
- [7. Risiko yang bisa membatalkan model](#7-risiko-yang-bisa-membatalkan-model)
- [8. Asumsi yang harus dibuktikan](#8-asumsi-yang-harus-dibuktikan)
- [9. Tahapan pembuktian model](#9-tahapan-pembuktian-model)

---

## 1. Kanvas — sembilan blok

### 1.1 Segmen Pelanggan

| Segmen | Ciri | Status |
|---|---|---|
| **Retail kecil 1 outlet** | Kelontong, toko bangunan, toko baju, toko kosmetik. Omzet Rp10–150 juta/bulan, 1–3 karyawan | **Beachhead** — semua energi awal ke sini |
| **F&B kecil** | Warung makan, kedai kopi, kedai minuman. Transaksi banyak, nilai kecil | Kedua |
| **Grosir & agen dengan tim sales** | Punya 2–20 sales lapangan, piutang besar | Ketiga — pintu masuk modul CRM Sales |
| **Freelance & jasa profesional** | Tanpa toko, tanpa stok, uang masuk bertermin | Keempat — produk berbeda dengan fondasi sama |
| **Usaha multi-outlet** | 3–10 cabang, sudah punya manajer | Naik kelas dari segmen 1 & 2, ARPU tertinggi |

Sekitar setengah segmen di atas punya karyawan, dan mereka semua mengerjakan absensi serta gaji secara
manual. Modul absensi & penggajian ([Bagian H blueprint](BLUEPRINT-SAAS-POS.md#bagian-h--absensi--penggajian-sederhana))
melayani irisan itu — lihat [bagian 5](#5-modul-yang-mengunci-absensi--penggajian).

**Bukan segmen kita:** jaringan waralaba besar (butuh kustomisasi dan tender), pedagang tanpa
smartphone, dan usaha yang butuh akuntansi penuh. Menolak ini sama pentingnya dengan memilih yang di atas.

### 1.2 Proposisi Nilai

Berbeda per orang yang menyentuh aplikasi — inilah sebabnya satu pesan pemasaran tidak cukup.

| Untuk | Masalah nyata hari ini | Yang kita janjikan |
|---|---|---|
| **Pemilik toko** | Tidak tahu untung hari ini tanpa menghitung manual; curiga ada kebocoran di kasir tapi tak bisa dibuktikan | Angka untung hari ini di satu layar, dan rekonsiliasi kas yang membuat selisih terlihat |
| **Pemilik yang punya karyawan** | Absensi di buku tulis, gaji dihitung kalkulator tiap akhir bulan, kasbon tercatat di kertas yang hilang | Absensi dari HP yang sudah ada, gaji terhitung sendiri menurut aturan usahanya, slip yang bisa dijelaskan ke karyawan |
| **Kasir** | Aplikasi lain lambat, ribet, dan mati saat internet putus | Transaksi selesai di bawah 10 detik, tetap jalan tanpa internet |
| **Pemilik grosir** | Tidak tahu sales benar-benar berkunjung atau tidak; piutang tercecer | Bukti kunjungan (GPS + foto), pesanan dari lokasi, komisi dari tagihan yang benar-benar tertagih |
| **Freelancer** | Penawaran tercecer di chat, invoice telat ditagih, tidak tahu proyek mana yang untung | Penawaran → invoice → pengingat tagih otomatis, dan laba per proyek |
| **Semua** | Takut data disandera dan takut harga naik diam-diam | Ekspor data di semua paket termasuk gratis, harga transparan, Bahasa Indonesia, dukungan WhatsApp |

**Tiga hal yang membuat kita berbeda dari POS gratisan bawaan bank/dompet digital:** benar-benar jalan
offline, punya manajemen stok yang serius, dan datanya bisa dibawa pergi kapan saja.

### 1.3 Saluran

| Saluran | Peran | Catatan |
|---|---|---|
| **Komunitas UMKM, koperasi, asosiasi pedagang** | Utama di awal | Satu sesi demo di paguyuban pedagang lebih efektif daripada ribuan tayangan iklan |
| **Agen/reseller daerah** | Skala | Mereka yang datang ke toko, memasang printer, dan mendampingi onboarding; dibayar komisi **berulang**, bukan sekali |
| **Freelance sales / afiliasi** | Jangkauan murah | Merujuk lewat kode/tautan tanpa kewajiban pendampingan; komisi lebih kecil dan berjangka |
| **Rujukan pelanggan** | Termurah | Pemilik toko saling bercerita; sediakan insentif rujukan yang sederhana |
| **Konten edukasi (WhatsApp, TikTok, YouTube)** | Menumbuhkan permintaan | Bukan iklan produk — "cara hitung stok", "cara tahu untung", lalu produk muncul sebagai alat |
| **Kemitraan PJP/bank penyedia QRIS** | Distribusi + pendapatan | Mereka butuh merchant, kita butuh jangkauan |
| **Dinas Koperasi & UMKM daerah** | Legitimasi | Program pembinaan digitalisasi UMKM |
| ~~Iklan digital berbayar~~ | Ditunda | Biaya akuisisi tidak sebanding dengan ARPU sebelum retensi terbukti |

**Roda gila yang paling murah:** pemilik toko yang puas adalah calon mitra afiliasi terbaik — mereka sudah
punya kredibilitas di antara sesama pedagang dan tidak perlu diyakinkan soal produknya. Jalur "pelanggan
puas → mitra afiliasi" sebaiknya dibuka sejak mitra pertama, bukan belakangan.

Perkakas untuk mengelola kedua jenis mitra — kode referral, komisi, atribusi, pencairan — dirancang di
[Bagian G blueprint](BLUEPRINT-SAAS-POS.md#bagian-g--program-mitra-penjual-agen--freelance-sales).

### 1.4 Hubungan Pelanggan

- **Onboarding terpandu, gratis, untuk semua pelanggan baru:** impor produk dari catatan lama dan
  pasang printer. Ini bukan biaya dukungan — ini investasi aktivasi, karena tenant yang produknya
  belum masuk tidak akan pernah bertransaksi.
- **Pendampingan langsung untuk 10–50 pelanggan pertama.** Datang ke tokonya. Di fase ini kita menjual
  sedikit dan belajar banyak.
- **Dukungan WhatsApp dengan SLA tertulis** (misal: dibalas < 2 jam pada jam kerja). Segmen ini tidak
  membuka email dan tidak akan mengisi tiket dukungan.
- **Layan-mandiri untuk hal rutin:** panduan singkat, video pendek, FAQ di dalam aplikasi.
- **Dukungan tingkat pertama oleh agen** untuk merchant yang mereka bawa: pertanyaan dasar diselesaikan
  di lapangan, yang rumit naik ke kita. Ini menurunkan biaya dukungan kita sekaligus mempercepat respons.
  **Syaratnya satu:** hubungan langsung kita dengan merchant tetap ada — merchant yang hanya kenal
  agennya akan ikut pergi saat agennya pergi.
- **Komunitas pengguna** (grup WhatsApp per daerah) — pengguna lama menjawab pengguna baru, dan kita
  dapat masukan produk tanpa riset formal.
- **Transparansi produk:** catatan pembaruan yang bisa dibaca orang awam.

### 1.5 Arus Pendapatan

Lima sumber, dengan urutan waktu yang disengaja — jangan aktifkan semuanya sekaligus.

| Sumber | Kapan | Sifat |
|---|---|---|
| **Langganan bulanan/tahunan POS** | Sejak awal | Berulang, tulang punggung |
| **Add-on modul CRM** (Freelance / Sales Lapangan) | Setelah Fase 9–10 | Berulang, memperdalam ARPU |
| **Add-on kanal pesanan online** (marketplace & aplikator) | Setelah Fase 11 | Berulang, ditagih per kanal aktif karena biaya melayaninya ikut naik per kanal |
| **Add-on absensi & penggajian** | Setelah Fase 13 | Berulang, ditagih per karyawan aktif. **Sumber retensi, bukan sekadar sumber pendapatan** — lihat [bagian 5](#5-modul-yang-mengunci-absensi--penggajian) |
| **Bagi hasil pembayaran digital (QRIS)** | Setelah integrasi PJP | Berulang, tumbuh mengikuti omzet tenant |
| **Jasa setup & migrasi data berbayar** | Sejak awal, opsional | Sekali, menutup biaya onboarding tenant besar |
| **Penjualan/bundling perangkat** (printer, scanner) | Lewat mitra | Sekali, margin tipis — tujuannya menghapus hambatan pasang |

**Yang sengaja tidak kita jual:** data transaksi tenant, dalam bentuk apa pun, teragregasi maupun tidak.
Kepercayaan adalah aset utama produk keuangan; sekali dijual, tidak bisa dibeli kembali.

**Peluang yang ditunda, bukan ditolak:** rujukan pembiayaan modal usaha berbasis riwayat transaksi.
Nilainya besar, tetapi menyentuh wilayah yang diawasi OJK dan menuntut kematangan data serta izin —
jangan disentuh sebelum model dasar terbukti.

### 1.6 Sumber Daya Kunci

| Sumber daya | Kenapa kunci |
|---|---|
| **Basis kode Go + PostgreSQL** | Satu binary melayani ribuan tenant; biaya melayani per tenant sangat rendah — ini yang membuat harga murah tetap sehat |
| **Arsitektur offline-first** | Sulit ditiru belakangan; harus dirancang sejak awal (lihat blueprint C.5) |
| **Jaringan agen & komunitas daerah** | Saluran distribusi yang tidak bisa dibeli dengan uang iklan |
| **Kepercayaan atas data pelanggan** | Aset paling rapuh dan paling menentukan |
| **Tim kecil yang paham lapangan** | Keputusan produk yang benar lahir dari melihat kasir bekerja, bukan dari rapat |
| **Konten edukasi berbahasa Indonesia** | Aset yang menumpuk nilainya, tidak habis seperti iklan |

### 1.7 Aktivitas Kunci

1. **Membangun produk sesuai roadmap** — Fase 0–7 sebelum jualan serius.
2. **Onboarding & migrasi data pelanggan baru** — aktivitas kunci, bukan pekerjaan sampingan. Metrik
   aktivasi ditentukan di sini.
3. **Dukungan pelanggan lewat WhatsApp** — sekaligus saluran riset produk termurah.
4. **Menjaga keandalan**: backup teruji, pemantauan, penanganan insiden. Satu hari data hilang cukup
   untuk membunuh reputasi di komunitas yang saling bercerita.
5. **Menjalankan program mitra** — rekrut dan verifikasi, latih dan sertifikasi, hitung komisi,
   **bayar tepat waktu**, dan putuskan sengketa atribusi. Pencairan yang telat sekali saja merusak
   kepercayaan seluruh jaringan, karena mitra saling bercerita persis seperti pedagang.
6. **Memproduksi konten edukasi** secara rutin.
7. **Menjaga kepatuhan** — UU PDP, kebijakan privasi, keamanan (lihat CONVENTIONS bagian 4).

### 1.8 Mitra Kunci

| Mitra | Kita dapat | Mereka dapat |
|---|---|---|
| **PJP / payment gateway QRIS** (Midtrans, Xendit, Doku) | Integrasi pembayaran + bagi hasil MDR | Merchant baru dan volume transaksi |
| **Penyedia WhatsApp Business API resmi** | Kanal OTP, struk, pengingat tagihan | Volume pesan |
| **Distributor perangkat** (printer termal, scanner) | Bundling yang menghapus hambatan pasang | Penjualan unit |
| **Agen/reseller daerah** | Distribusi, pendampingan lokal, dukungan tingkat pertama | Komisi berulang selama merchant aktif + bonus aktivasi |
| **Freelance sales / afiliasi** | Jangkauan tanpa biaya tetap | Komisi berjangka atas rujukan yang benar-benar aktif |
| **Koperasi, asosiasi pedagang, dinas UMKM** | Akses ke kumpulan calon pelanggan | Program digitalisasi anggota |
| **Penyedia cloud & database terkelola** | Infrastruktur tanpa tim infra | Biaya langganan |
| **Konsultan pajak / akuntan** | Rujukan dua arah, masukan fitur laporan | Klien yang pembukuannya sudah rapi |

### 1.9 Struktur Biaya

Sifat biaya lebih penting daripada besarnya: yang **tetap** menentukan titik impas, yang **variabel**
menentukan margin.

| Biaya | Sifat | Catatan |
|---|---|---|
| Gaji/waktu tim (developer, dukungan) | Tetap | Porsi terbesar di tahun pertama |
| Infrastruktur (VPS, DB terkelola, storage, backup) | Semi-variabel | Tumbuh bertahap, bukan per tenant |
| Pesan WhatsApp (OTP, struk, pengingat) | **Variabel per tenant** | Sering diremehkan; bisa menggerus margin bila pengingat dikirim tanpa batas |
| **Komisi mitra (berulang)** | Variabel per tenant, **selamanya** | Bukan biaya akuisisi sekali bayar — ini pemotong margin permanen untuk tenant hasil mitra. Lihat [bagian 6](#6-ekonomi-program-mitra-penjual) |
| Bonus aktivasi mitra | Variabel per tenant, sekali | Cair setelah merchant benar-benar dipakai, bukan saat mendaftar |
| Materi, pelatihan, dan administrasi mitra | Semi-tetap | Naik bertahap seiring jumlah mitra |
| Dukungan pelanggan | Variabel per tenant | Berbanding lurus dengan buruknya onboarding |
| Akuisisi (acara komunitas, transport, materi) | Variabel per tenant | |
| Legal & kepatuhan | Tetap | Kebijakan privasi, syarat layanan, konsultasi PDP |
| Perangkat untuk bundling | Variabel | Bila stok sendiri; hindari — serahkan ke mitra |

---

## 2. Rincian pendapatan

**Contoh struktur harga (asumsi, wajib divalidasi lewat wawancara harga):**

| Paket | Harga/bulan | Untuk siapa |
|---|---|---|
| Gratis | Rp0 | Coba-coba; batas 1 pengguna, 50 produk, 300 transaksi/bulan |
| Basic | Rp79.000 /outlet | Toko 1 outlet yang sudah serius |
| Pro | Rp199.000 /outlet | Butuh laporan penuh + integrasi QRIS |
| Multi-Outlet | Rp399.000 (3 outlet) + Rp99.000/outlet tambahan | Usaha bercabang |
| Add-on CRM Freelance | Rp99.000 | Tanpa perlu paket POS |
| Add-on CRM Sales | Rp59.000 /pengguna sales | Grosir & distribusi |
| Add-on Kanal Online | Rp49.000 /kanal aktif | Marketplace & aplikator layanan antar |
| Add-on Absensi & Gaji | Rp5.000 /karyawan aktif, minimum Rp25.000 | Usaha yang punya karyawan |

### Diskon langganan dibayar di muka

Tenant boleh membayar di muka untuk 3, 6, 9, atau 12 bulan. Semakin panjang masanya, semakin besar
diskonnya — dan lompatan terbesar sengaja ditaruh di 12 bulan agar ke sanalah orang diarahkan.

| Masa | Diskon | Basic (Rp79.000/bln) | Pro (Rp199.000/bln) | Setara |
|---|---|---|---|---|
| Bulanan | — | Rp79.000 | Rp199.000 | — |
| **3 bulan** | **5%** | Rp225.000 | Rp565.000 | hemat ±½ bulan |
| **6 bulan** | **10%** | Rp425.000 | Rp1.075.000 | hemat ±½ bulan lebih |
| **9 bulan** | **12,5%** | Rp620.000 | Rp1.565.000 | hemat ±1 bulan |
| **12 bulan** | **16,7%** | Rp790.000 | Rp1.990.000 | **2 bulan gratis** |

Harga dibulatkan ke ribuan terdekat agar mudah diucapkan saat menawarkan — pedagang menyebut angka,
bukan persentase.

**Kenapa diskon ini menguntungkan, bukan sekadar mengurangi pendapatan.** Tenant bulanan tidak pasti
bertahan. Pada asumsi churn 5%/bulan, seorang tenant bulanan rata-rata hanya membayar **±8,7 bulan**
dari 12 bulan ke depan. Membayar di muka 12 bulan seharga 10 bulan berarti kita menerima **10 bulan,
hari ini, tunai**. Setiap anak tangga lolos uji yang sama:

| Masa | Yang kita terima di muka | Perkiraan diterima bila bulanan (churn 5%) | Selisih |
|---|---|---|---|
| 3 bulan | 2,85 bulan | 2,71 bulan | +0,14 |
| 6 bulan | 5,40 bulan | 5,03 bulan | +0,37 |
| 9 bulan | 7,88 bulan | 7,02 bulan | +0,86 |
| 12 bulan | 10,00 bulan | 8,73 bulan | **+1,27** |

> **Batas amannya: churn ±3%/bulan.** Bila retensi kelak membaik sampai churn turun di bawah itu, diskon
> 12 bulan mulai merugikan — tenant memang akan bertahan sendiri tanpa perlu dibayar untuk tinggal.
> Tinjau ulang tangga diskon ini setiap kali churn berubah lebih dari 1 poin.

**Empat aturan yang menjaga skema ini tidak bocor:**

1. **Uang di muka bukan pendapatan bulan itu.** Catat sebagai pendapatan diterima di muka dan akui
   sebulan demi sebulan. Kalau tidak, laporan laba terlihat besar di bulan penjualan, lalu kas terpakai
   untuk melayani bulan-bulan yang belum dijalani.
2. **Komisi mitra dibayar bertahap per bulan berjalan**, meski uangnya kita terima sekaligus. Ini
   melindungi kas dan membuat penarikan kembali tetap mungkin bila tenant membatalkan di tengah masa.
   Dasar perhitungannya tetap nilai yang benar-benar diterima — yaitu setelah diskon.
3. **Pembatalan di tengah masa dihitung ulang pada harga bulanan normal.** Sisa dana dikembalikan
   setelah bulan yang telanjur dipakai ditagih tanpa diskon. Adil bagi kedua pihak, dan mencegah orang
   membeli 12 bulan lalu minta uang kembali di bulan kedua.
4. **Naik paket di tengah masa memakai prorata**, sisa nilai menjadi kredit — jangan paksa tenant
   menunggu masa habis untuk membayar kita lebih banyak.

**Bagi hasil QRIS:** pendapatan mengikuti omzet tenant, bukan jumlah tenant. Satu toko dengan omzet
Rp100 juta/bulan yang 40% transaksinya QRIS menghasilkan bagi hasil yang bisa melampaui langganannya
sendiri. Ini alasan integrasi pembayaran layak dikerjakan meski bukan fitur yang diminta pengguna.
Besaran bagi hasil ditentukan negosiasi dengan PJP dan berubah-ubah — perlakukan sebagai variabel,
jangan sebagai janji.

**Bagi hasil QRIS:** pendapatan mengikuti omzet tenant, bukan jumlah tenant. Satu toko dengan omzet
Rp100 juta/bulan yang 40% transaksinya QRIS menghasilkan bagi hasil yang bisa melampaui langganannya
sendiri. Ini alasan integrasi pembayaran layak dikerjakan meski bukan fitur yang diminta pengguna.
Besaran bagi hasil ditentukan negosiasi dengan PJP dan berubah-ubah — perlakukan sebagai variabel,
jangan sebagai janji.

---

## 3. Struktur biaya

**Contoh biaya tetap bulanan pada tahap awal (1 developer + 1 dukungan paruh waktu):**

| Pos | Perkiraan/bulan |
|---|---|
| Tim (2 orang, sebagian waktu) | Rp20.000.000 |
| Infrastruktur (VPS + DB terkelola + storage + backup) | Rp1.500.000 |
| Layanan pendukung (pemantauan, error tracking, domain) | Rp500.000 |
| Legal & administrasi | Rp1.000.000 |
| **Total tetap** | **± Rp23.000.000** |

**Biaya variabel per tenant berbayar per bulan:**

| Pos | Perkiraan |
|---|---|
| Infrastruktur terpakai | Rp3.000 |
| Pesan WhatsApp | Rp7.000 |
| Dukungan (rata-rata) | Rp10.000 |
| **Total variabel** | **± Rp20.000** |

Prinsip dari blueprint tetap berlaku: **biaya melayani satu tenant harus di bawah 10% harga langganannya.**
Pada Basic Rp79.000, batas itu Rp7.900 — artinya biaya WhatsApp dan dukungan harus ditekan lewat
pembatasan pengingat dan onboarding yang benar, bukan dibiarkan.

---

## 4. Unit economics

Ilustrasi dengan asumsi campuran paket: ARPU **Rp120.000**/tenant/bulan.

> **Catatan tangga diskon.** Bila sebagian tenant beralih ke pembayaran di muka, ARPU bulanan efektif
> turun sekitar 6–7% (menjadi ±Rp112.000 pada campuran 40% bulanan / 20% tiga bulan / 15% enam bulan /
> 5% sembilan bulan / 20% setahun). Margin kotor per tenant turun ke ±Rp92.000 dan titik impas naik ke
> ±250 tenant — **tetapi churn efektif ikut turun** karena tenant prabayar tidak bisa berhenti di tengah
> masa, dan LTV justru naik. Diskon ini menukar sedikit margin dengan kepastian dan uang tunai lebih awal.

| Besaran | Nilai | Cara hitung |
|---|---|---|
| ARPU | Rp120.000 | campuran Basic/Pro/Multi + add-on |
| Biaya variabel per tenant | Rp20.000 | tabel di atas |
| **Margin kotor per tenant** | **Rp100.000 (83%)** | ARPU − biaya variabel |
| Biaya akuisisi (CAC) | Rp300.000 | waktu demo + pendampingan + komisi agen |
| **Payback CAC** | **± 3 bulan** | CAC ÷ margin kotor |
| Churn bulanan (asumsi) | 5% | UMKM tutup, ganti aplikasi, berhenti pakai |
| Umur pelanggan | ± 20 bulan | 1 ÷ churn |
| **LTV** | **± Rp2.000.000** | margin kotor × umur |
| **LTV : CAC** | **± 6–7×** | sehat bila churn tertahan di 5% |
| **Titik impas** | **± 230 tenant berbayar** | biaya tetap ÷ margin kotor per tenant |

**Yang paling menentukan model ini bukan harga, melainkan churn.** Pada churn 10%/bulan, umur pelanggan
turun jadi 10 bulan, LTV jadi Rp1.000.000, dan LTV:CAC turun ke ±3× — masih hidup tapi rapuh. Pada
churn 15%, model berhenti masuk akal. Karena itu aktivasi dan onboarding diperlakukan sebagai aktivitas
kunci, bukan pekerjaan dukungan.

---

## 5. Modul yang mengunci: absensi & penggajian

Seluruh model bisnis ini bergantung pada satu angka — churn ([bagian 4](#4-unit-economics)). Karena itu
modul yang **menurunkan churn** bernilai lebih besar daripada pendapatan langganannya sendiri, dan
absensi-penggajian adalah kandidat terkuat untuk itu.

### Kenapa modul ini paling melekat

| Modul | Biaya pindah bagi tenant |
|---|---|
| Kasir | Sedang — produk bisa diimpor ulang ke aplikasi lain |
| Stok | Sedang — saldo awal bisa dihitung ulang |
| CRM | Agak tinggi — riwayat prospek hilang |
| **Absensi & gaji** | **Sangat tinggi** — riwayat kehadiran, slip gaji yang sudah dikunci, saldo kasbon berjalan, dan aturan potongan yang sudah disepakati dengan karyawan |

Pemilik yang sudah menjalankan tiga periode gaji di aplikasi ini tidak akan pindah hanya karena
kompetitor menawarkan diskon. Berhenti berlangganan berarti kembali ke buku tulis dan kehilangan bukti
saat karyawan menyanggah potongan.

### Nilainya dalam angka

Ilustrasi untuk tenant yang memakai modul ini, dengan 5 karyawan:

| Besaran | Tanpa modul HR | Dengan modul HR |
|---|---|---|
| ARPU | Rp120.000 | Rp145.000 *(+Rp25.000 add-on)* |
| Margin kotor | Rp100.000 | Rp124.000 |
| Churn bulanan (asumsi) | 5% | **4%** |
| Umur pelanggan | 20 bulan | 25 bulan |
| **LTV** | **Rp2.000.000** | **Rp3.100.000** |

Kenaikan LTV sekitar **55%** datang dari dua arah sekaligus: ARPU naik, dan umur pelanggan memanjang.
Yang kedua jauh lebih berharga daripada yang pertama.

> **Asumsi churn 4% belum terbukti.** Turunnya churn adalah tebakan yang masuk akal, bukan hasil
> pengamatan — dan justru asumsi inilah yang menentukan apakah modul ini layak dibangun. Cara mengujinya
> ada di [bagian 8](#8-asumsi-yang-harus-dibuktikan), dan bisa dilakukan **sebelum** modulnya jadi:
> tanyakan pada tenant yang sudah ada berapa yang mengelola gaji manual, dan berapa yang bersedia
> membayar untuk berhenti melakukannya.

### Kenapa ditagih per karyawan

Nilai modul ini tumbuh mengikuti jumlah karyawan — begitu pula biaya melayaninya (penyimpanan absensi
harian, pengiriman slip lewat WhatsApp). Usaha berkaryawan dua orang tidak boleh membayar sama dengan
yang berkaryawan dua puluh. Batas bawah Rp25.000 menjaga tenant kecil tetap menutupi biaya melayaninya.

### Risiko khusus modul ini

**Kesalahan di sini lebih mahal daripada di modul mana pun.** Salah menghitung stok membuat pemilik
kesal; salah memotong gaji membuat pemilik kehilangan muka di depan karyawannya, dan itu diceritakan ke
sesama pedagang. Konsekuensinya untuk pengembangan: modul ini tidak dirilis sebelum satu periode gaji
nyata dihitung ulang secara manual dan hasilnya cocok sampai rupiah terakhir.

Kehati-hatian kedua: **jangan menjadi penasihat ketenagakerjaan.** Sistem menghitung menurut kebijakan
yang diisi tenant, tidak menyarankan besaran upah, dan tidak menghitung pajak karyawan — sikap yang sama
dengan pajak usaha di modul kasir.

---

## 6. Ekonomi program mitra penjual

Komisi berulang mengubah sifat biaya akuisisi: sebagian dari CAC berhenti menjadi ongkos sekali bayar
dan berubah menjadi **pemotong margin permanen**. Menukar margin dengan CAC yang jauh lebih murah bisa
sangat menguntungkan — tetapi hanya bila retensinya tidak ikut memburuk.

### Contoh skema komisi (asumsi)

| Jenis mitra | Bonus aktivasi | Komisi berulang | Syarat cair |
|---|---|---|---|
| **Agen daerah** | Rp50.000, sekali | **15%** dari langganan, selama merchant aktif | Merchant mencapai ambang aktivasi (misal 30 transaksi atau 30 hari aktif) |
| **Freelance sales / afiliasi** | — | **7%** dari langganan, dibatasi 12 bulan | Sama |

Aturan yang membuat skema ini tidak bocor — selengkapnya di
[G.2 blueprint](BLUEPRINT-SAAS-POS.md#g2-aturan-komisi--tempat-kesalahan-paling-mahal): komisi dihitung
atas pembayaran yang **benar-benar diterima**, ada syarat aktivasi sebelum rupiah pertama cair, ada
penarikan kembali bila merchant batal, dan **tidak ada skema berjenjang**.

### Dua jalur akuisisi, dibandingkan

Memakai ARPU Rp120.000 dan asumsi agen daerah dengan komisi 15%:

| Besaran | Pendaftaran mandiri | **Lewat agen daerah** |
|---|---|---|
| ARPU | Rp120.000 | Rp120.000 |
| Biaya melayani | Rp20.000 | **Rp15.000** — dukungan tingkat pertama ditangani agen |
| Komisi berulang | — | **Rp18.000** |
| **Margin kotor** | **Rp100.000 (83%)** | **Rp87.000 (73%)** |
| CAC | Rp300.000 | **Rp100.000** — bonus aktivasi + materi & pelatihan teralokasi |
| **Payback CAC** | 3 bulan | **± 1,2 bulan** |
| LTV pada churn 5% | Rp2.000.000 | Rp1.740.000 |
| **LTV : CAC** | ± 6–7× | **± 17×** |

**Cara membacanya:** margin per tenant turun 10 poin, tetapi biaya akuisisi menjadi sepertiga dan modal
kembali dua kali lebih cepat. Untuk usaha yang uang tunainya terbatas, jalur mitra jauh lebih ramah —
selama retensinya tidak lebih buruk.

### Titik impas bergeser

| Komposisi tenant | Margin kotor rata-rata | Titik impas |
|---|---|---|
| Semua pendaftaran mandiri | Rp100.000 | ± 230 tenant |
| Separuh dari mitra | Rp93.500 | ± 250 tenant |
| Semua dari mitra | Rp87.000 | ± 265 tenant |

Kenaikan titik impas ini murah dibayar bila jalur mitra membuat tenant bertambah jauh lebih cepat
daripada yang bisa kita capai sendiri — dan itulah taruhannya.

### Satu angka yang menentukan segalanya

**Retensi merchant hasil mitra dibanding pendaftaran mandiri.**

- **Bila lebih baik** (agen mendampingi onboarding, menjawab pertanyaan dasar, mengingatkan perpanjangan)
  — jalur mitra unggul di semua sisi dan layak diperbesar.
- **Bila sama** — tetap layak, karena payback tiga kali lebih cepat.
- **Bila lebih buruk** (agen mengejar bonus, menjual ke siapa saja, lalu menghilang) — kita membayar
  komisi selamanya untuk pelanggan yang cepat pergi. Skema komisi harus segera diubah, bukan ditambal.

Karena itu papan peringkat dan evaluasi tingkat mitra **wajib berbasis merchant aktif, bukan jumlah
pendaftaran** — ukuran yang salah di sini akan langsung menghasilkan perilaku yang salah.

### Biaya program yang sering terlupa

Selain komisi: materi jualan, pelatihan dan sertifikasi, waktu admin untuk verifikasi dan pencairan,
biaya transfer, serta penanganan sengketa. Pada 20 mitra ini masih ringan; pada 100 mitra ia menjadi
peran tersendiri. Otomatiskan perhitungan komisi sebelum jumlah mitra membuatnya mustahil dikerjakan
tangan — pemicunya ada di
[G.11 blueprint](BLUEPRINT-SAAS-POS.md#g11-kapan-ini-dibangun).

---

## 7. Risiko yang bisa membatalkan model

| Risiko | Dampak | Mitigasi |
|---|---|---|
| **POS gratis dari bank/dompet digital** (mereka mensubsidi demi volume pembayaran) | Harga kita terlihat mahal | Menangkan di tempat yang mereka abaikan: mode offline sungguhan, stok serius, multi-outlet, CRM, dan portabilitas data. Jangan bertanding di harga |
| **Churn tinggi karena UMKM tutup** | LTV runtuh | Segmen beachhead adalah toko yang sudah berjalan, bukan usaha baru; pantau kesehatan pemakaian sebagai peringatan dini |
| **Aktivasi rendah** (daftar tapi tak pernah transaksi) | CAC terbuang | Impor produk gratis dan terpandu; ukur transaksi pertama dalam 24 jam |
| **Biaya dukungan membengkak** | Margin tergerus | Perbaiki produk di titik yang paling sering ditanyakan; konten edukasi; komunitas |
| **Ketergantungan pada satu PJP** | Pendapatan bagi hasil rapuh | Rancang lapisan pembayaran agar bisa berganti penyedia |
| **Insiden kehilangan/kebocoran data** | Fatal di komunitas yang saling bercerita | Backup teruji bulanan, isolasi tenant berlapis, jejak audit — semuanya sudah jadi syarat go-live di blueprint |
| **Pendiri terpecah antara POS dan CRM** | Dua produk setengah jadi | Urutan fase di blueprint; CRM baru setelah POS matang |
| **Merchant hasil mitra churn lebih cepat** | Komisi dibayar selamanya untuk pelanggan yang cepat pergi | Bandingkan retensi per jalur sejak 50 tenant pertama; evaluasi tingkat mitra berbasis merchant aktif |
| **Merchant fiktif demi mengejar bonus** | Angka pertumbuhan palsu, uang keluar percuma | Syarat aktivasi berbasis pemakaian nyata + audit acak |
| **Ketergantungan pada segelintir mitra besar** | Satu mitra pergi, sebagian jaringan ikut goyah | Pantau sebaran; jangan biarkan satu mitra melebihi porsi tertentu dari tenant baru |
| **Mitra membanting harga** | Harga berantakan antar wilayah, margin tergerus | Harga resmi terkunci; diskon hanya lewat kode promo tercatat |
| **Pencairan komisi telat** | Kepercayaan jaringan rusak — mitra saling bercerita | Otomatiskan perhitungan lebih awal; jadwal pencairan tetap dan diumumkan |
| **Salah hitung gaji karyawan tenant** | Kepercayaan hancur paling cepat — menyangkut uang orang, dan diceritakan ke sesama pedagang | Slip dikunci dan bisa direproduksi persis; uji dengan satu periode nyata sebelum rilis; setiap baris slip menyimpan dasar hitungnya |
| **Terseret sengketa ketenagakerjaan tenant** | Beban hukum di luar kendali | Aplikasi mencatat kebijakan tenant, tidak memberi nasihat, tidak menghitung pajak karyawan; jejak audit lengkap |
| **Perubahan regulasi pajak/pembayaran** | Fitur usang mendadak | Semua tarif sebagai konfigurasi, tidak pernah hard-code |

---

## 8. Asumsi yang harus dibuktikan

Urut dari yang paling mematikan bila salah.

| # | Asumsi | Cara membuktikan | Ambang lulus |
|---|---|---|---|
| 1 | Pemilik toko mau **membayar** untuk kerapian catatan, bukan hanya mau yang gratis | Tawarkan berbayar ke 20 toko percontohan setelah masa gratis | ≥ 30% lanjut berbayar |
| 2 | Onboarding bisa menghasilkan transaksi pertama dalam 24 jam | Ukur aktivasi pada 50 tenant pertama | ≥ 60% aktif dalam 24 jam |
| 3 | Churn bulanan tertahan di bawah 5% | Amati kohor 3 bulan | Retensi bulan-3 ≥ 85% |
| 4 | Komunitas & agen benar-benar menghasilkan pelanggan | Uji 3 komunitas dan 3 agen | CAC < Rp300.000 |
| 5 | Biaya melayani tetap di bawah 10% harga langganan | Ukur biaya WhatsApp + dukungan aktual | Terpenuhi pada 100 tenant |
| 6 | Bagi hasil QRIS material, bukan receh | Negosiasi dengan 2 PJP sebelum membangun integrasi | Proyeksi ≥ 20% ARPU |
| 7 | **Retensi merchant hasil mitra tidak lebih buruk** daripada pendaftaran mandiri | Bandingkan kohor per jalur akuisisi sejak 50 tenant pertama | Retensi bulan-3 mitra ≥ mandiri |
| 8 | **Mitra sendiri bertahan**, bukan berhenti setelah beberapa bulan | Amati mitra aktif per kuartal | ≥ 60% mitra masih membawa merchant baru di kuartal ke-2 |
| 9 | **Modul absensi & gaji menurunkan churn**, bukan sekadar menambah ARPU | Bandingkan retensi tenant pemakai vs bukan pemakai, kohor 6 bulan | Churn pemakai ≥ 1 poin lebih rendah |
| 10 | Tenant berkaryawan mau memindahkan urusan gaji ke aplikasi kasir | Tanyakan ke 30 tenant berjalan sebelum Fase 13 | ≥ 40% bersedia membayar add-on |
| 11 | Segmen freelance cukup besar untuk dilayani modul terpisah | 20 wawancara sebelum Fase 9 | ≥ 10 bersedia bayar di muka |

---

## 9. Tahapan pembuktian model

| Tahap | Target | Yang dibuktikan | Kaitan roadmap |
|---|---|---|---|
| **Uji pakai** | 5–10 toko, gratis | Produk dipakai setiap hari tanpa didampingi | Setelah Fase 5 |
| **Uji bayar** | 20–30 toko | Ada yang mau membayar (asumsi 1) | Setelah Fase 7 |
| **Uji saluran** | 100 tenant berbayar, 5–10 mitra | CAC dan churn masuk akal (asumsi 3 & 4); retensi per jalur akuisisi terbandingkan (asumsi 7) | Fase 8 berjalan, program mitra masih manual |
| **Uji program mitra** | 15+ mitra aktif | Skema komisi tidak bocor dan mitra sendiri bertahan (asumsi 8) | Memicu Fase 12 |
| **Uji skala** | 230+ tenant berbayar | Titik impas tercapai | — |
| **Uji kedalaman** | Add-on & QRIS aktif | ARPU naik tanpa menaikkan churn | Setelah Fase 9–10 |
| **Uji kelekatan** | 20+ tenant memakai modul gaji | Churn pemakai lebih rendah daripada bukan pemakai | Setelah Fase 13 |

Jangan lompat tahap. Menambah saluran sebelum churn terkendali hanya memperbesar kebocoran ember.

---

*Dokumen hidup — perbarui setiap kali satu asumsi terbukti atau terbantah.
Pasangannya: [BLUEPRINT-SAAS-POS.md](BLUEPRINT-SAAS-POS.md) untuk produk dan roadmap.*
