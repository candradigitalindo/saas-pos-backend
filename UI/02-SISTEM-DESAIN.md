# 02 — Sistem Desain

Token di sini adalah sumber kebenaran. Nilai warna yang tertulis sudah
**diverifikasi rasio kontrasnya** (WCAG 2.1), bukan dikira-kira.

---

## Filosofi warna

Tema aplikasi ini adalah **usaha yang tumbuh**. Palet dibangun dari satu ide:
warna harus terasa seperti panen dan pagi hari — bukan seperti laporan bank.

- **Hijau** jadi warna utama karena maknanya paling langsung untuk pedagang:
  tumbuh, subur, panen, untung. Hijau juga warna yang **paling luas diterima
  lintas usia dan kalangan**, dan di Indonesia tidak membawa konotasi politik
  atau golongan tertentu — penting untuk produk yang dijual ke siapa saja.
- **Jingga hangat** jadi aksen: energi, optimisme, keramahan pasar tradisional.
  Dipakai **hemat** — hanya untuk momen pencapaian. Kalau semua disorot, tidak
  ada yang tersorot.
- **Netral hangat (batu)**, bukan abu-abu kebiruan. Abu kebiruan terasa
  korporat dan dingin; netral hangat membuat aplikasi terasa ramah.

---

## Palet

### Hijau Tumbuh — warna utama

| Token | Hex | Dipakai untuk |
|---|---|---|
| `hijau-50` | `#ECFDF5` | Latar sorotan lembut, baris terpilih |
| `hijau-100` | `#D1FAE5` | Latar lencana "berhasil" |
| `hijau-500` | `#10B981` | Grafik, ikon dekoratif |
| `hijau-600` | `#059669` | Garis tepi, ikon di latar terang |
| **`hijau-700`** | **`#047857`** | **Isian tombol utama, tautan, nav aktif** |
| `hijau-800` | `#065F46` | Tombol ditekan, teks di latar hijau muda |

> ⚠️ **Isian tombol memakai `hijau-700`, bukan `hijau-600`.**
> Teks putih di atas `#059669` hanya 3.77:1 — **gagal** untuk teks kecil.
> Di atas `#047857` menjadi **5.48:1** dan lulus. Ini bukan selera, ini keterbacaan.

### Jingga Semangat — aksen

| Token | Hex | Dipakai untuk |
|---|---|---|
| `jingga-100` | `#FEF3C7` | Latar lencana "perlu perhatian" |
| **`jingga-400`** | **`#FBBF24`** | **Sorotan pencapaian — WAJIB teks gelap di atasnya** |
| `jingga-600` | `#D97706` | Ikon peringatan, garis tepi |
| `jingga-700` | `#B45309` | Teks peringatan di latar terang |

> ⚠️ **Jingga tidak pernah memakai teks putih.** Putih di atas `#D97706` hanya
> 3.19:1 — gagal. Teks gelap di atas `#FBBF24` mencapai **10.48:1** dan justru
> paling terbaca di seluruh palet. Jadikan jingga selalu "gelap di atas terang".

### Semantik

| Peran | Isian | Teks di latar terang | Kontras teks |
|---|---|---|---|
| Berhasil / untung | `#047857` | `#047857` | 5.48:1 ✅ |
| Bahaya / rugi | `#DC2626` (teks putih 4.83:1 ✅) | `#B91C1C` | 6.47:1 ✅ |
| Peringatan | `#FBBF24` (teks gelap) | `#B45309` | 5.02:1 ✅ |
| Informasi | `#2563EB` (teks putih 5.17:1 ✅) | `#1D4ED8` | 6.70:1 ✅ |

### Netral hangat

| Token | Hex | Dipakai untuk | Kontras di `#FAFAF9` |
|---|---|---|---|
| `latar` | `#FAFAF9` | Latar halaman | — |
| `permukaan` | `#FFFFFF` | Kartu, panel |  — |
| `garis` | `#E7E5E4` | Garis pemisah (dekoratif, bukan teks) | 1.20 — tidak untuk teks |
| `teks-redup` | `#6F6963` | Keterangan, placeholder | 5.19:1 ✅ |
| `teks-sekunder` | `#57534E` | Label, sub-judul | 7.30:1 ✅ |
| `teks-utama` | `#1C1917` | Isi utama, angka | 16.74:1 ✅ |

### Warna kategori grafik

Untuk laporan per kanal / per metode bayar. Dipilih agar tetap terbedakan oleh
mata dengan buta warna merah-hijau, dan **selalu** disertai label — warna tidak
pernah jadi satu-satunya penanda.

`#047857` · `#2563EB` · `#B45309` · `#7C3AED` · `#0891B2` · `#BE185D`

**Urutannya tetap dan tidak pernah diputar.** Warna mengikuti *entitas*, bukan
peringkatnya: kalau sebuah penyaring menghilangkan satu kanal, kanal yang
tersisa tidak boleh berganti warna. Deret ketujuh dan seterusnya digabung
menjadi "Lainnya", bukan diberi warna baru hasil generate.

> ⚠️ Slot kelima sebelumnya `#0E7490`. Diubah ke `#0891B2` setelah rasio
> warnanya dihitung, bukan dikira-kira: chroma `#0E7490` hanya 0,094 — di bawah
> ambang, sehingga di grafik ia terbaca **abu-abu** dan justru melanggar tujuan
> palet ini sendiri. `#0891B2` menaikkan chroma di atas ambang sekaligus
> memperbaiki jarak buta-warna terhadap `#BE185D` dari ΔE 10,5 → 13,0.
> Keduanya lulus di mode terang maupun gelap.

Pemisahan untuk buta warna biru-kuning (tritan) ada di angka 7,5 — di bawah
ambang aman 8. Itu sebabnya aturan "selalu disertai label" di atas **bukan
anjuran**: tanpa label langsung, dua kanal bisa tertukar oleh sebagian pembaca.

---

## Mode gelap

Dipakai kasir malam hari dan warung dengan pencahayaan redup. Bukan pembalikan
warna, melainkan palet tersendiri:

| Token | Terang | Gelap |
|---|---|---|
| Latar | `#FAFAF9` | `#1C1917` |
| Permukaan | `#FFFFFF` | `#292524` |
| Teks utama | `#1C1917` | `#FAFAF9` |
| Hijau utama | `#047857` | `#34D399` (lebih terang agar tetap terbaca di latar gelap) |

### Teks semantik di mode gelap

Warna teks semantik **berganti** di mode gelap. Nilai versi terang dipilih untuk
latar putih dan gagal total di atas `#292524`:

| Peran | Terang | Kontras | Gelap | Kontras |
|---|---|---|---|---|
| Bahaya / rugi | `#B91C1C` | 6.47:1 | `#F87171` | 5.48:1 |
| Peringatan | `#B45309` | 5.02:1 | `#FBBF24` | 9.09:1 |
| Informasi | `#1D4ED8` | 6.70:1 | `#60A5FA` | 5.97:1 |
| Berhasil / untung | `#047857` | 5.48:1 | `#34D399` | 7.89:1 |
| Berhasil (pekat) | `#065F46` | 7.68:1 | `#6EE7B7` | 9.95:1 |

> ⚠️ **Ini ditemukan dengan mengukur, bukan dengan melihat.** Versi pertama mode
> gelap hanya mengganti latar, permukaan, teks utama, dan hijau — warna teks
> semantik dibiarkan memakai nilai terang. Akibatnya lencana **"HABIS" merah
> `#B91C1C` di atas `#292524` hanya 2,34:1** dan praktis tidak terbaca — persis
> untuk kasir malam hari, satu-satunya alasan mode gelap ini ada. Seluruh nilai
> di tabel sudah diukur ≥ 4,5:1 terhadap `permukaan` **dan** `permukaan-2`.
>
> `teks-redup` juga diturunkan dari `#78716C` ke `#6F6963`: nilai lama
> diverifikasi terhadap `latar` (4,59:1) tapi jatuh ke **4,40:1** di atas
> `permukaan-2` yang sedikit lebih gelap.

Aturan: **jangan** memakai hitam murni `#000` — kontras ekstrem melelahkan mata
saat dipakai berjam-jam.

---

## Tipografi

**Plus Jakarta Sans** (SIL Open Font License, gratis). Dipilih karena buatan
Indonesia, angkanya tegas dan tidak ambigu (`1`, `7`, `0` mudah dibedakan) —
penting untuk aplikasi yang isinya uang.

Cadangan: `-apple-system, Segoe UI, Roboto, sans-serif`.

| Peran | Ukuran / tinggi baris | Bobot | Catatan |
|---|---|---|---|
| Angka sorotan | 40 / 44 | 800 | Uang di beranda |
| Judul halaman | 24 / 32 | 700 | |
| Judul kartu | 18 / 26 | 600 | |
| Isi | 16 / 24 | 400 | **Jangan di bawah 16px** — banyak pengguna berumur 40+ |
| Label | 14 / 20 | 500 | |
| Keterangan | 13 / 18 | 400 | Minimum absolut |

**Semua angka memakai `font-variant-numeric: tabular-nums`** supaya kolom lurus
dan angka tidak "bergoyang" saat berubah.

---

## Jarak, sudut, bayangan

- **Kelipatan 4px**: 4, 8, 12, 16, 24, 32, 48, 64. Tidak ada nilai di luar ini.
- **Sudut**: kontrol 8px · kartu 12px · dialog 16px · lencana penuh.
- **Bayangan**: hanya 3 tingkat. Kartu nyaris rata (`0 1px 2px rgba(0,0,0,.05)`),
  menu melayang, dialog paling tinggi. Bayangan tebal bikin UI terasa berat.

---

## Komponen inti

Dibangun di atas **shadcn/ui** (Radix) — disalin ke repo sehingga bebas diubah.

### Tombol

| Jenis | Tampilan | Kapan |
|---|---|---|
| Utama | Isian `hijau-700`, teks putih | Satu per layar |
| Kedua | Garis tepi `garis`, teks `teks-utama` | Aksi pendamping |
| Bahaya | Isian `#DC2626`, teks putih | Batalkan, hapus |
| Teks | Tanpa latar | Aksi tersier |

Tinggi: 40px (padat) · 48px (normal) · **64px (kasir)**.

> ⚠️ **"Padat" 40px hanya untuk penunjuk halus (tetikus).** Angka ini sempat
> bertabrakan dengan [01 §3](01-PRINSIP-DESAIN.md) yang mewajibkan target sentuh
> minimal 48×48 px — dan prinsip di dokumen 01 yang mengikat. Penyelesaiannya:
> tombol padat dirender 48px secara bawaan dan baru menyusut ke 40px pada
> `pointer: fine`. Di HP dan tablet tidak ada tombol di bawah 48px.
Saat memuat: label berganti "Menyimpan…" + tombol dinonaktifkan — **lebarnya
tidak berubah** supaya tata letak tidak melompat.

> **Tombol utama yang nonaktif memakai permukaan NETRAL, bukan hijau yang
> diredupkan.** Hijau pekat pada 50% opasitas di atas latar terang menjadi hijau
> keruh yang terbaca sebagai "rusak", bukan "belum bisa ditekan" — paling
> kentara pada tombol BAYAR saat keranjang kosong, tombol terbesar di layar
> tersibuk. Varian lain tetap memakai opasitas.

### Kolom isian

```
Harga Jual                          ← label selalu di atas, tidak pernah placeholder
┌────────────────────────────────┐
│ Rp  15.000                     │  ← awalan satuan menyatu di kolom
└────────────────────────────────┘
Harga yang dibayar pembeli.         ← bantuan singkat, selalu ada untuk kolom uang

Harga Jual                          ← keadaan salah
┌────────────────────────────────┐
│ Rp                             │  garis tepi merah
└────────────────────────────────┘
⚠ Harga jual belum diisi.           ← pesan di bawah kolom, BUKAN di atas layar
```

- Kolom uang: papan tik angka (`inputmode="numeric"`), format ribuan otomatis saat mengetik.
- Kolom jumlah: tombol `−` dan `+` besar di kiri-kanan, karena mengetik angka di HP sambil berdiri itu susah.

### Kartu ringkasan (dipakai di beranda)

```
┌──────────────────────────────────┐
│ Uang masuk hari ini              │  ← label dulu, baru angka
│                                  │
│ Rp 1.250.000                     │  ← 40px, bobot 800
│ ▲ 12% dibanding kemarin          │  ← hijau + ikon panah (bukan warna saja)
└──────────────────────────────────┘
```

### Kartu sorotan — satu per layar

Satu angka terpenting, di atas latar berwarna penuh.

```
┌────────────────────────────────────────────────────┐
│ ▓▓▓ permukaan-sorotan ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓ │
│ UANG MASUK HARI INI                     Laporan ›  │ ← huruf besar berjarak
│ Rp 324.000                               ▁▃▂▅▁█▆   │ ← 40px/800 · grafik kanan
│ ( ↓ 67% dibanding kemarin )                        │ ← lencana BERGARIS
│ ──────────────────────────────────────────────     │
│ Transaksi              Rata-rata belanja           │ ← angka pendukung
│ 9                      Rp 36.000                   │
└────────────────────────────────────────────────────┘
```

**Kenapa ada.** Beranda versi pertama menaruh semua angka di kartu putih yang
seragam. Diukur dengan `npm run periksa:warna`, liputan Hijau Tumbuh di beranda
ternyata **0,0% di HP** dan 1,3% di desktop: 95–99% layar putih dan abu-abu.
Akibatnya dua, dan keduanya merugikan:

1. Aplikasinya tidak terasa punya identitas — warna utama yang dipilih
   susah-payah di dokumen ini praktis tidak pernah terlihat.
2. Satu-satunya warna kuat di layar justru kabar buruk: merah "turun 67%" dan
   jingga "habis" lima baris. Pemilik warung yang hari itu menerima Rp 324.000
   disambut layar yang terasa seperti peringatan.

Setelah kartu sorotan dipakai, liputannya **16–33%** di semua lebar 320–1440px.

**Kerajinan yang membuatnya terasa layak jual, bukan sekadar berwarna:**

| Unsur | Aturan | Kenapa |
|---|---|---|
| Permukaan | `.permukaan-sorotan` — gradien `hijau-800` → `utama`, 135° | Kedalaman tanpa pasangan warna baru: kedua ujungnya sudah diverifikasi |
| Bayangan | `melayang`, bukan `kartu` | Satu-satunya elemen yang boleh terasa terangkat |
| Label | Huruf besar, `tracking-wider`, 13px | Ia penanda, bukan kalimat — supaya tidak bersaing dengan angkanya |
| Lencana tren | **Bergaris**, `border-current/30`, isian transparan | Garis tidak mengubah warna di baliknya, jadi rasio teks tetap utuh |
| Grafik | Di kanan angka, lebar dibatasi ~176px | Dibiarkan selebar kartu, tujuh batang di atas 1000px jadi balok 140px yang terbaca sebagai hiasan |
| Grafik di HP | Turun ke bawah angka, maks 200px | Disandingkan, ia menyisakan <210px dan "Rp 324.000" pecah dua baris |

**Kartu angka pendamping** memakai kelengkapan yang sama tingkatnya lebih rendah:
keping ikon `bg-utama/10` (tinta 10% — cukup terbaca, terlalu tipis untuk
menggeser kontras), dan perubahan sebagai **lencana berisi `permukaan-2`** dengan
teks semantik. Pasangan itu bukan pilihan bebas: `bahaya-teks`, `hijau-700`, dan
`jingga-700` di atas `permukaan-2` justru pasangan yang sudah diukur di audit
kontras. Teks merah telanjang di beberapa kartu sekaligus membuat halaman
terbaca seperti peringatan; lencana memberinya batas sehingga terbaca sebagai
keterangan.

**Aturan:**

- **Satu per layar.** Kalau dua angka sama-sama disorot, tidak ada yang tersorot.
- Memakai pasangan token `utama` / `utama-teks` apa adanya — keduanya sudah
  dirancang berpasangan dan membalik sendiri di mode gelap (`#047857` + putih →
  `#34D399` + `#1C1917`). Jangan membuat token baru untuk ini.
- **Arah perubahan ditandai panah, bukan warna.** Merah "turun" mustahil dibuat
  terbaca di atas hijau penuh — dan tidak perlu: yang jadi berita adalah angka
  di atasnya, bukan alarmnya.
- Angka pendamping di layar yang sama memakai **24px** (`ringkas`), bukan 40px.
  Perbedaan ukuran itulah yang membentuk susunannya.

Grafik mungilnya digambar dengan elemen biasa berbasis flex, bukan Recharts: beranda adalah
layar pertama tiap pagi dan tidak boleh menunggu bundel grafik. Versi SVG-nya
sempat dicoba, tapi `preserveAspectRatio="none"` yang membuatnya bisa melar juga
melarkan sudut membulat batangnya jadi elips picak. Ia **menyembunyikan
diri bila kurang dari dua hari berisi** — satu batang di antara garis tipis
terlihat seperti komponen rusak, dan warung yang baru mulai justru persis ada di
keadaan itu.

### Kartu barang di kasir

```
┌──────────────────────┐(1)  ← lencana jumlah, muncul saat ada di keranjang
│ Gula Pasir 1kg       │     ← NAMA: 14px, teks-sekunder
│                      │
│ Rp 16.000            │     ← HARGA: 18px/800, teks-utama
│ sisa 4 pcs           │
└──────────────────────┘
```

**Harga lebih besar daripada nama.** Nama dipakai untuk MENEMUKAN barang, harga
untuk MEMASTIKAN — dan yang dibaca berulang-ulang sepanjang hari adalah harganya.

- Barang habis tetap terlihat, hanya tidak bisa ditekan. Disembunyikan, kasir
  mengira barangnya lenyap dari sistem lalu menelepon pemilik.
- "Habis" ditulis sebagai **lencana**, bukan teks merah telanjang: teks merah di
  dalam kartu mudah tertukar dengan pesan galat.
- Ditekan → kartu menyusut `scale-[0.98]`, dihormati `motion-reduce`. Di layar
  yang diketuk bertubi-tubi, inilah yang mengabarkan "ketukan tadi masuk".
- **Tidak ada toast saat barang masuk keranjang.** Ketukannya sudah dijawab tiga
  kali dan seketika — lencana jumlah, baris di keranjang, dan totalnya berubah.
  Toast tambahan hanya mengulang yang sudah terlihat, dan karena kasir menekan
  bertubi-tubi ia menumpuk sampai menutupi tombol bayar di baliknya. Pengecualian:
  jalur PINDAI tetap bertoast, karena di sana dialog kamera menutupi keranjang
  dan lencananya.

### Kontrol segmen

Sekelompok pilihan saling-tolak dalam SATU wadah.

```
┌─────────────────────────────────────────────────┐
│ ( Hari ini )  7 hari   30 hari   Bulan ini   …  │ ← wadah permukaan-2
└─────────────────────────────────────────────────┘
      ▲ terpilih: keping `permukaan` + shadow-kartu + teks `utama`
```

- **Wadah `permukaan-2`, terpilih naik sebagai keping `permukaan`.** Pilihan
  mengambang satu-satu dengan garis tepi masing-masing terbaca sebagai lima
  tombol yang kebetulan berdekatan; wadah membuatnya terbaca sebagai satu
  kendali dengan satu nilai — dan itu memang yang terjadi.
- **Menggeser mendatar, TIDAK membungkus.** Pilihan yang membungkus ke baris
  kedua membuat tinggi halaman melompat saat labelnya berubah, dan di HP
  mendorong isi halaman turun tanpa alasan yang terlihat.
- Tinggi **48px, menyusut ke 40px pada `pointer: fine`** — aturan yang sama
  dengan tombol varian "padat" ([01 §3](01-PRINSIP-DESAIN.md)).
- `teks-sekunder` dan `utama` di atas `permukaan-2` adalah pasangan yang sudah
  diukur di audit kontras; jangan menggantinya tanpa mengukur ulang.

### Daftar: tabel di layar lebar, kartu di layar kecil

Komponen yang sama, tampilan beda — **bukan tabel yang digeser ke samping**.

```
≥1024px (tabel)                    <768px (kartu)
┌──────┬────────┬──────┬────────┐  ┌────────────────────────────┐
│ Nama │ Stok   │ Harga│ Aksi   │  │ Kopi Susu                  │
├──────┼────────┼──────┼────────┤  │ Rp 18.000 · sisa 44 pcs    │
│ Kopi │ 44 pcs │ 18rb │  ⋯     │  │                     [ Ubah ]│
└──────┴────────┴──────┴────────┘  └────────────────────────────┘
```

### Lencana status

Selalu **ikon + teks + warna** — tidak pernah warna saja.

`✓ Lunas` (hijau) · `⏳ Menunggu` (jingga) · `✕ Dibatalkan` (merah) · `● Draf` (netral)

---

## Ikon

**Lucide** — garis 2px, konsisten, gratis. Ukuran 20px (dalam teks) / 24px (navigasi).

Di navigasi utama dan tombol penting, **ikon selalu ditemani teks**. Ikon
sendirian hanya boleh untuk aksi yang sudah universal (✕ tutup, ← kembali).

---

## Gerak

Secukupnya, dan selalu punya alasan.

- Transisi: **150–200ms**, `ease-out`.
- Dialog masuk dari bawah di HP (mengikuti kebiasaan aplikasi ponsel), memudar di desktop.
  Terpasang sebagai `.gerak-dialog` / `.gerak-lapis` di `tokens.css`, memakai
  `data-state` dari Radix. Di layar besar SENGAJA hanya memudar tanpa geser:
  dialognya dipusatkan dengan `translate(-50%, -50%)`, jadi keyframe yang ikut
  menyentuh `transform` membatalkan pemusatan itu dan dialognya melompat ke
  pojok saat animasi mulai.
- **Wajib menghormati `prefers-reduced-motion`** — matikan animasi bila pengguna memintanya.
- Tidak ada animasi pada angka uang yang sedang berubah: uang harus terbaca, bukan bergerak.
- **Toast mengambang DI ATAS bilah bawah, tidak menimpanya** (`--sela-bilah-bawah`).
  Diukur di layar kasir: notifikasi "barang ditambahkan" menutupi tombol bayar
  selama lima detik — menyembunyikan persis hal yang ingin ditekan pengguna
  berikutnya, dan mengabarkan hal yang sudah terlihat dari lencana di kartu
  barangnya.

---

## Responsif

| Rentang | Perangkat | Tata letak |
|---|---|---|
| < 640px | HP | Satu kolom, navigasi bawah 5 ikon, dialog dari bawah |
| 640–1023px | Tablet potret | Dua kolom untuk daftar |
| **1024–1439px** | **Tablet lanskap — layar kasir utama** | Kiri: produk (grid). Kanan: keranjang tetap terlihat |
| ≥ 1440px | Desktop | Navigasi samping tetap + isi maksimum 1280px agar tidak terlalu lebar |

Dirancang **mobile-first**: tulis gaya untuk HP dulu, baru tambahkan untuk layar besar.
