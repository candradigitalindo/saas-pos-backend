# 01 — Prinsip Desain

Sepuluh aturan di bawah **mengikat**. Kalau sebuah rancangan melanggarnya,
rancangan itu yang diubah, bukan aturannya.

---

## 1. Satu layar, satu pekerjaan

Setiap layar menjawab **satu** pertanyaan atau menyelesaikan **satu** tugas.
Kalau sebuah layar butuh judul dengan kata "dan", pecah jadi dua.

- ✅ "Kasir" — hanya untuk menjual
- ✅ "Tutup Shift" — hanya menghitung uang laci
- ❌ "Manajemen Transaksi & Kas" — dua pekerjaan berbeda dipaksa jadi satu

## 2. Bahasa orang, bukan bahasa sistem

Istilah teknis diterjemahkan **sekali di seluruh aplikasi**, tidak setengah-setengah.

| Jangan tulis | Tulis |
|---|---|
| Sinkronisasi | Kirim data ke server |
| Idempotency / duplikat request | (jangan ditampilkan sama sekali — tangani diam-diam) |
| Void transaksi | Batalkan transaksi |
| Stock opname | Hitung fisik barang |
| Gross profit | Untung kotor |
| Net amount | Uang bersih |
| Receivable | Utang pelanggan (kasbon) |
| Outlet | Toko / Cabang |
| Tenant | (jangan pernah muncul di UI) |
| Reconcile | Cocokkan |
| Payload / request gagal | Data belum terkirim |

**Aturan uji:** bacakan kalimat di layar ke orang yang belum pernah pakai
aplikasi ini. Kalau dia bertanya balik "maksudnya apa?", ganti kalimatnya.

## 3. Tombol besar, dan jelas mana yang utama

- Target sentuh **minimal 48 × 48 px**, tombol utama di kasir **minimal 64 px** tinggi.
- **Satu tombol utama per layar**, berwarna penuh. Sisanya abu-abu atau teks saja.
- Tombol berbahaya (Batalkan, Hapus) **tidak pernah** bersebelahan dengan tombol utama.
- Label tombol memakai **kata kerja**: "Simpan Barang", bukan "OK". "Bayar Rp 45.000", bukan "Submit".

## 4. Angka adalah isi utama, perlakukan sebagai bintang

- Uang selalu lengkap: **Rp 1.250.000** — dengan titik pemisah ribuan, tanpa desimal.
- Angka penting ditulis besar (32–40px), labelnya kecil di atasnya.
- Selalu sertakan **pembanding**: "Rp 1.250.000 · naik 12% dari kemarin".
  Angka tanpa pembanding tidak memberi tahu apa pun.
- Angka gunakan *tabular numerals* supaya kolom lurus dan tidak "goyang" saat berubah.

## 5. Selalu tampilkan apa yang sedang terjadi

Tidak boleh ada ketukan yang "sunyi". Setiap aksi punya tiga keadaan yang terlihat:

```
 [ Simpan ]  →  [ ⟳ Menyimpan… ]  →  [ ✓ Tersimpan ]
```

- Proses < 1 detik: cukup perubahan tombol.
- Proses > 1 detik: tampilkan *skeleton*, **bukan** spinner kosong di tengah layar.
- Selesai: notifikasi singkat di bawah layar, hilang sendiri 4 detik.

## 6. Kesalahan harus bisa dipulihkan, bukan dicegah dengan konfirmasi berlapis

Dialog "Anda yakin?" akan diklik "Ya" tanpa dibaca setelah kali kelima.

- Aksi ringan (hapus draf, hapus baris) → **langsung jalan + tombol "Urungkan" 8 detik**.
- Aksi berat & tidak bisa dibalik (batalkan transaksi, kunci gaji) → dialog yang
  **menjelaskan akibatnya dengan angka**, bukan sekadar bertanya:

  > **Batalkan transaksi #0001?**
  > Stok 3 pcs Kopi Susu akan dikembalikan.
  > Omzet hari ini turun Rp 45.000.
  > Transaksi ini tetap tercatat di riwayat sebagai "dibatalkan".

## 7. Tidak ada jalan buntu

Setiap layar kosong, error, dan hasil pencarian nihil **wajib** punya jalan keluar.

```
┌──────────────────────────────────┐
│            🗂                     │
│   Belum ada barang di sini       │
│   Tambahkan barang dulu supaya   │
│   bisa mulai berjualan.          │
│                                  │
│   [ + Tambah Barang ]            │
│   atau  Impor dari Excel         │
└──────────────────────────────────┘
```

Pesan error juga begitu — sebutkan **apa yang harus dilakukan**, bukan kode error.

| Jangan | Lakukan |
|---|---|
| "Error 409: conflict" | "Nomor struk ini sudah tersimpan. Cek di riwayat penjualan." |
| "Validation failed" | "Harga jual belum diisi." (langsung di bawah kolomnya) |
| "Network error" | "Belum ada internet. Data disimpan di HP dan dikirim otomatis nanti." |

## 8. Aplikasi tetap bekerja tanpa sinyal

Backend sudah menyiapkan `/sync/push` dan `/sync/pull` — UI wajib memanfaatkannya.

- Kasir **selalu** bisa menjual, walau internet mati total.
- Status koneksi ditampilkan sebagai **fakta yang menenangkan**, bukan peringatan merah:
  - `✓ Semua data tersimpan` (hijau, kecil)
  - `⏳ 3 transaksi menunggu dikirim` (jingga, kecil) — **bukan** dialog, bukan alarm
- Yang **tidak boleh** dilakukan offline dibuat abu-abu dengan alasannya:
  "Laporan butuh internet".

## 9. Warna punya arti tetap di seluruh aplikasi

Sekali dipelajari, berlaku di mana saja — ini yang membuat pengguna gaptek cepat hafal.

| Warna | Artinya, selalu |
|---|---|
| 🟢 Hijau | Maju, untung, berhasil, aman |
| 🔴 Merah | Rugi, batal, hilang, bahaya |
| 🟡 Jingga | Perlu perhatian, hampir habis, menunggu |
| 🔵 Biru | Keterangan / informasi netral |

Hijau **tidak pernah** dipakai untuk tombol hapus. Merah **tidak pernah** untuk
angka yang bagus. Tidak ada pengecualian "demi estetika".

## 10. Maksimal tiga ketukan ke pekerjaan harian

Diukur dari layar pertama setelah masuk:

| Pekerjaan | Jalur | Ketukan |
|---|---|---|
| Jual barang | Beranda → Kasir → pilih barang | 2 |
| Lihat untung hari ini | Beranda (langsung terlihat) | 0 |
| Tambah stok masuk | Beranda → Stok → Barang Masuk | 2 |
| Tutup kasir | Beranda → Kasir → Tutup Shift | 3 |

Kalau sebuah pekerjaan harian butuh lebih dari 3 ketukan, letakkan pintasannya
di Beranda.

---

## Anti-pola yang dilarang

| Dilarang | Sebabnya |
|---|---|
| Tabel lebar yang harus digeser ke samping di HP | Kolom penting hilang di luar layar. Gunakan **kartu bertumpuk** di layar kecil |
| Menu bertingkat lebih dari 2 level | Pengguna tersesat dan tidak bisa kembali |
| Ikon tanpa tulisan di navigasi utama | Ikon tidak universal. Selalu ikon **+** teks |
| Istilah bahasa Inggris di layar utama | Memaksa pengguna menerka |
| Form panjang satu halaman (>7 kolom) | Pecah jadi beberapa langkah dengan indikator "Langkah 1 dari 3" |
| Warna sebagai satu-satunya penanda | 1 dari 12 pria buta warna merah-hijau. Selalu tambah ikon atau teks |
| Menyembunyikan aksi di balik tekan-lama atau geser | Tidak akan pernah ditemukan. Sediakan tombol yang terlihat |
| *Auto-save* diam-diam pada data uang | Pengguna harus sadar kapan uang tercatat. Simpan harus disengaja |
| Nomor halaman kecil-kecil untuk daftar panjang | Gunakan "Muat lebih banyak" atau gulir tak terbatas di HP |
