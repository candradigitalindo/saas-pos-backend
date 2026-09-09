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
| `teks-redup` | `#78716C` | Keterangan, placeholder | 4.59:1 ✅ |
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
Saat memuat: label berganti "Menyimpan…" + tombol dinonaktifkan — **lebarnya
tidak berubah** supaya tata letak tidak melompat.

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
- **Wajib menghormati `prefers-reduced-motion`** — matikan animasi bila pengguna memintanya.
- Tidak ada animasi pada angka uang yang sedang berubah: uang harus terbaca, bukan bergerak.

---

## Responsif

| Rentang | Perangkat | Tata letak |
|---|---|---|
| < 640px | HP | Satu kolom, navigasi bawah 5 ikon, dialog dari bawah |
| 640–1023px | Tablet potret | Dua kolom untuk daftar |
| **1024–1439px** | **Tablet lanskap — layar kasir utama** | Kiri: produk (grid). Kanan: keranjang tetap terlihat |
| ≥ 1440px | Desktop | Navigasi samping tetap + isi maksimum 1280px agar tidak terlalu lebar |

Dirancang **mobile-first**: tulis gaya untuk HP dulu, baru tambahkan untuk layar besar.
