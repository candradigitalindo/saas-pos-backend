# 05 — Alur Utama

Sketsa di bawah adalah **rancangan tata letak dan urutan**, bukan gambar akhir.
Yang mengikat: jumlah langkah, urutannya, dan kata-kata di layar.

---

## 1. Dari mendaftar sampai bisa jualan — target: di bawah 5 menit

Ini momen paling menentukan. Kalau pemilik warung tidak berhasil menjual barang
pertamanya di hari pertama, dia tidak akan kembali.

```
Daftar (1 layar, 6 kolom)
   │  Nama usaha · Jenis usaha · No. HP
   │  Nama Anda · Username · Kata sandi
   │  ▸ Punya kode dari agen?   ← tertutup, agar tidak membingungkan
   ▼
Selamat datang, Bu Sari! 🎉
   │  Usaha Anda sudah siap. Tinggal 3 langkah lagi:
   │
   │  ┌──────────────────────────────────────────┐
   │  │ ① Tambah barang pertama      [ Mulai ]   │
   │  │ ② Isi stok awal              terkunci    │
   │  │ ③ Coba transaksi pertama     terkunci    │
   │  └──────────────────────────────────────────┘
   │  Lewati dulu →
   ▼
Tambah barang (3 kolom saja: Nama · Harga jual · Harga beli)
   │  Kolom lain (SKU, barcode, kategori) disembunyikan di "Detail lainnya"
   ▼
Isi stok awal  →  Coba transaksi  →  🎉 "Penjualan pertama Anda tercatat!"
```

**Keputusan penting:**
- Pendaftaran **tidak** menanyakan alamat, NPWP, logo. Semua itu bisa menyusul.
- Langkah onboarding **bisa dilewati**, tapi tetap ditampilkan di Beranda sampai selesai.
- Perayaan kecil di transaksi pertama — ini satu-satunya tempat **Jingga Semangat**
  dipakai penuh. Momen ini yang membuat orang merasa "saya bisa".

---

## 2. Kasir — layar yang paling sering dilihat

Tablet lanskap. Keranjang **selalu terlihat** — tidak pernah bersembunyi di balik ikon.

```
┌────────────────────────────────────────┬───────────────────────────────┐
│ 🔍 Cari barang…              [Scan 📷] │  Keranjang                    │
├────────────────────────────────────────┤                               │
│  Semua  Minuman  Makanan  Rokok        │  Kopi Susu                    │
├────────────────────────────────────────┤  Rp 18.000 × 2   = Rp 36.000  │
│ ┌────────┐ ┌────────┐ ┌────────┐       │        [ − ]  2  [ + ]   🗑   │
│ │ Kopi   │ │ Teh    │ │ Roti   │       │  ─────────────────────────    │
│ │ Susu   │ │ Manis  │ │ Bakar  │       │  Teh Manis                    │
│ │18.000  │ │ 8.000  │ │12.000  │       │  Rp 8.000 × 1    = Rp  8.000  │
│ │sisa 44 │ │sisa 12 │ │ HABIS  │       │        [ − ]  1  [ + ]   🗑   │
│ └────────┘ └────────┘ └────────┘       │                               │
│              (kartu 120×120,           ├───────────────────────────────┤
│               target sentuh besar)     │  Total        Rp 44.000       │
│                                        │                               │
│                                        │  ┌─────────────────────────┐  │
│                                        │  │   BAYAR  Rp 44.000      │  │  64px
│                                        │  └─────────────────────────┘  │
└────────────────────────────────────────┴───────────────────────────────┘
   ✓ Semua data tersimpan          Kasir: Sari · Shift dibuka 08:00
```

**Yang membuat ini ramah untuk pengguna gaptek:**
- Barang habis **tetap terlihat** tapi tidak bisa ditekan — kalau disembunyikan,
  kasir mengira barangnya hilang dari sistem lalu menelepon pemilik.
- Jumlah diubah dengan tombol `−`/`+` besar, bukan mengetik.
- Tombol Bayar **menampilkan nominalnya** — kasir membaca ulang sebelum menekan.
- Status koneksi ada di pojok, kecil dan tenang. Bukan alarm.

### Layar bayar

```
┌───────────────────────────────────────┐
│ Total belanja        Rp 44.000        │
│                                       │
│ Cara bayar                            │
│  ┌────────┐ ┌────────┐ ┌────────┐     │
│  │ 💵     │ │ 📱     │ │ 📝     │     │
│  │ Tunai  │ │ QRIS   │ │ Kasbon │     │
│  └────────┘ └────────┘ └────────┘     │
│                                       │
│ Uang diterima                         │
│ ┌───────────────────────────────────┐ │
│ │ Rp 50.000                         │ │
│ └───────────────────────────────────┘ │
│  [ Pas ] [ 50rb ] [ 100rb ]           │  ← pintasan nominal umum
│                                       │
│ ╔═══════════════════════════════════╗ │
│ ║ KEMBALIAN        Rp 6.000         ║ │  ← paling besar di layar
│ ╚═══════════════════════════════════╝ │
│                                       │
│ ┌───────────────────────────────────┐ │
│ │        SELESAI & CETAK            │ │
│ └───────────────────────────────────┘ │
└───────────────────────────────────────┘
```

Kembalian adalah angka **terbesar** di layar — itu yang dibutuhkan kasir dalam
tekanan antrean, bukan nomor struk.

---

## 3. Tutup shift — momen paling rawan salah

```
┌───────────────────────────────────────────────┐
│  Tutup Shift · Sari · 08:00 – 17:30           │
│                                               │
│  Uang yang seharusnya ada di laci             │
│                                               │
│    Modal awal              Rp   100.000       │
│    Penjualan tunai         Rp 1.240.000       │
│    Uang masuk lain         Rp    50.000       │
│    Uang keluar             Rp  − 75.000       │
│    ─────────────────────────────────────      │
│    Seharusnya              Rp 1.315.000       │
│                                               │
│  Hitung uang di laci sekarang, lalu isi:      │
│  ┌─────────────────────────────────────────┐  │
│  │ Rp 1.310.000                            │  │
│  └─────────────────────────────────────────┘  │
│                                               │
│  ┌─────────────────────────────────────────┐  │
│  │ ⚠ Kurang Rp 5.000                       │  │  ← jingga, bukan merah
│  │ Tidak apa-apa — selisih kecil biasa     │  │
│  │ terjadi. Beri catatan bila tahu         │  │
│  │ sebabnya.                               │  │
│  │ ┌─────────────────────────────────────┐ │  │
│  │ │ Catatan (boleh dikosongkan)         │ │  │
│  │ └─────────────────────────────────────┘ │  │
│  └─────────────────────────────────────────┘  │
│                                               │
│         [ Tutup Shift ]                       │
└───────────────────────────────────────────────┘
```

**Nada bahasa sengaja menenangkan.** Selisih kecil itu wajar; kalau UI-nya
menuduh (merah, "SELISIH!"), kasir akan berhenti jujur dan mulai memaksakan
angka agar pas. Itu jauh lebih merugikan daripada selisih Rp 5.000.

---

## 4. Barang masuk

```
Stok → Barang Masuk
   │
   ▼
┌────────────────────────────────────────┐
│ Dari pemasok                           │
│ ┌────────────────────────────────────┐ │
│ │ Toko Grosir Jaya            ▾      │ │
│ └────────────────────────────────────┘ │
│                                        │
│ Barang                    [ + Tambah ] │
│ ┌────────────────────────────────────┐ │
│ │ Kopi Susu                          │ │
│ │ Jumlah [ − ] 24 [ + ]  pcs         │ │
│ │ Harga beli satuan  Rp 12.000       │ │
│ │ ──────────────────────────         │ │
│ │ Subtotal           Rp 288.000      │ │
│ └────────────────────────────────────┘ │
│                                        │
│ Total bayar          Rp 288.000        │
│         [ Simpan Barang Masuk ]        │
└────────────────────────────────────────┘
   │
   ▼
✓ 24 pcs Kopi Susu masuk. Stok sekarang 68 pcs.
  ℹ Harga modal diperbarui otomatis jadi Rp 12.000.
```

Baris terakhir itu penting: backend memperbarui `cost_price` dari pembelian
terakhir. Kalau tidak diberitahu, pemilik akan bingung kenapa angka untungnya
berubah.

---

## 5. Hitung fisik (opname) — dirancang untuk dikerjakan sambil berdiri

```
Langkah 1 dari 3 · Pilih barang yang mau dihitung
Langkah 2 dari 3 · Hitung
┌────────────────────────────────────────┐
│  Kopi Susu                             │
│  Catatan sistem: 44 pcs                │  ← ditampilkan; jujur lebih baik
│                                        │     daripada "menguji" petugas
│  Hitungan Anda                         │
│      [ − ]     42      [ + ]           │
│                                        │
│  ⚠ Selisih −2 pcs                      │
│                                        │
│      [ Lewati ]      [ Berikutnya → ]  │
│                                        │
│  ●●●○○○○  3 dari 7 barang              │
└────────────────────────────────────────┘
Langkah 3 dari 3 · Tinjau & simpan
   │  Ringkasan seluruh selisih + nilai rupiahnya
   ▼
Dialog: "Stok akan disesuaikan mengikuti hitungan Anda.
         7 barang berubah, nilai selisih Rp −36.000.
         Tindakan ini tercatat dan tidak bisa dibatalkan."
```

Satu barang satu layar, ada indikator kemajuan. Petugas gudang bisa berhenti
di tengah dan melanjutkan nanti tanpa kehilangan hitungan.

---

## 6. Laporan untuk pemilik — jawab dulu, rinci belakangan

```
┌──────────────────────────────────────────────┐
│  Laporan          [ Hari ini ▾ ]             │
│                                              │
│  Untung bersih                               │
│  Rp 412.000                                  │  ← 40px
│  ▲ 12% dibanding kemarin                     │
│                                              │
│  ┌────────────────────────────────────────┐  │
│  │ Uang masuk        Rp 1.250.000         │  │
│  │ Modal barang    − Rp   780.000         │  │
│  │ Biaya kanal     − Rp    58.000         │  │
│  │ ────────────────────────────────       │  │
│  │ Untung bersih     Rp   412.000         │  │
│  └────────────────────────────────────────┘  │
│                                              │
│  Dari mana penjualannya?                     │
│  Kasir langsung  ████████████░░  Rp 890.000  │
│  GoFood          ████░░░░░░░░░░  Rp 260.000  │
│  Shopee          ██░░░░░░░░░░░░  Rp 100.000  │
│                                              │
│  💡 GoFood ramai, tapi setelah komisi 20%    │
│     untungnya paling tipis.                  │
│                                              │
│  [ Unduh Excel ]        Lihat rincian →      │
└──────────────────────────────────────────────┘
```

- Angka besarnya adalah **untung**, bukan omzet — itu pertanyaan sebenarnya.
- Rumusnya ditulis terbuka sebagai pengurangan bertingkat, supaya pemilik
  paham dari mana angkanya, bukan disuruh percaya.
- Baris 💡 muncul hanya kalau memang ada pola yang layak disebut. Kalau
  dipaksakan setiap hari, akan diabaikan.

---

## 7. Saat internet mati

```
Normal                          Terputus
┌──────────────────────┐        ┌──────────────────────────────────┐
│ ✓ Semua data         │        │ ⏳ 3 transaksi menunggu dikirim  │
│   tersimpan          │        │    Tetap bisa jualan seperti     │
└──────────────────────┘        │    biasa.                        │
                                └──────────────────────────────────┘
```

- Kasir **tidak** melihat dialog, tidak ada bunyi, tidak ada merah.
- Menu yang butuh internet (Laporan) diberi ikon awan-dicoret + keterangan
  "Butuh internet" — bukan hilang, supaya pengguna tidak mengira fiturnya lenyap.
- Saat tersambung lagi: pengiriman berjalan sendiri, lalu notifikasi kecil
  "✓ 3 transaksi berhasil dikirim".
- Kalau ada yang gagal setelah beberapa percobaan, muncul satu tempat khusus:
  "2 transaksi perlu diperiksa" → daftar + tombol "Coba kirim lagi".
  **Tidak pernah** menyalahkan pengguna atas kegagalan jaringan.

---

## 8. Gaji — modul paling rumit, harus terasa paling sederhana

Backend punya mesin gaji deterministik dengan urutan tetap dan penguncian.
Kerumitannya disembunyikan di balik empat langkah yang jelas.

```
Periode Gaji September 2026
┌──────────────────────────────────────────────┐
│  ① Hitung  →  ② Periksa  →  ③ Kunci  →  ④ Bayar │
│  ●            ○             ○           ○     │
└──────────────────────────────────────────────┘

Setelah ② Periksa:
┌──────────────────────────────────────────────┐
│  8 karyawan · Total Rp 24.500.000            │
│                                              │
│  Budi Santoso                Rp 3.200.000 →  │
│  Sari Dewi                   Rp 2.850.000 →  │
│  …                                           │
│                                              │
│  ⚠ 1 slip perlu diperiksa                    │
│     Ahmad: potongan lebih besar dari gaji     │
│                                              │
│         [ Kunci Periode Ini ]                │
└──────────────────────────────────────────────┘

Dialog kunci:
  "Setelah dikunci, angka gaji tidak bisa diubah lagi.
   Koreksi absensi yang datang setelah ini akan otomatis
   masuk sebagai penyesuaian di gaji bulan depan.
   Kunci periode September 2026?"
```

Kalimat terakhir itu menjelaskan perilaku backend yang sebenarnya rumit
(penyesuaian periode berikutnya) dengan satu kalimat yang bisa dipahami — dan
menghilangkan rasa takut "kalau saya salah bagaimana".

Slip gaji menampilkan **rincian per baris** dengan nama komponen yang
di-*snapshot* backend, bukan hanya angka akhir — supaya karyawan bisa memeriksa
sendiri dan tidak perlu bertanya.

---

## 9. Portal mitra — satu pertanyaan, satu jawaban

```
┌──────────────────────────────────────────────┐
│  Halo, Agen Bandung                          │
│                                              │
│  Komisi bulan ini                            │
│  Rp 1.840.000                                │
│  Cair sekitar 5 Oktober                      │  ← inilah yang dicari
│                                              │
│  ┌────────────┬────────────┬──────────────┐  │
│  │ Prospek    │ Merchant   │ Merchant     │  │
│  │ 12         │ aktif 8    │ baru 2       │  │
│  └────────────┴────────────┴──────────────┘  │
│                                              │
│  Merchant binaan                             │
│  ┌────────────────────────────────────────┐  │
│  │ Warung Bu Sari      ● Aktif            │  │
│  │ Jatuh tempo 12 Okt 2026                │  │
│  └────────────────────────────────────────┘  │
│                                              │
│  ℹ Anda hanya dapat melihat status           │
│    langganan merchant. Data penjualan,       │
│    produk, dan pelanggan mereka rahasia.     │
│                                              │
│  [ + Daftarkan Prospek ]                     │
└──────────────────────────────────────────────┘
```

Keterangan privasi ditulis permanen, bukan disembunyikan. Ia melindungi merchant
sekaligus menjawab lebih dulu pertanyaan mitra "kok datanya cuma segini?".
