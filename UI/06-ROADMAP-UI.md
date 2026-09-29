# 06 — Roadmap UI

Urutan membangun frontend. Prinsipnya sama dengan backend: **setiap tahap
menghasilkan sesuatu yang benar-benar bisa dipakai**, bukan setengah jadi yang
menunggu tahap berikutnya.

Backend seluruh fase §16 sudah selesai, jadi tidak ada tahap UI yang terhalang API —
kecuali yang ditandai di [04](04-PETA-LAYAR.md#yang-belum-ada-di-backend).

---

## U0 — Fondasi

Belum ada layar bisnis; ini yang membuat sisanya cepat.

- Vite + React + TS + Tailwind; token warna dari [02](02-SISTEM-DESAIN.md) masuk `tokens.css`
- `api-client.ts`: amplop respons, penerjemah error 401/403/404/409/422/429/5xx
- Autentikasi: masuk, perbarui token otomatis, keluar
- `useIzin()` + penjagaan rute
- Kerangka layout (HP, tablet, desktop) + navigasi dari izin
- Komponen dasar: Tombol, Kolom, Kartu, Dialog, Toast, KeadaanKosong, LencanaStatus
- Util **uang bilangan bulat** dan format Rupiah + tes unitnya

**Selesai bila:** bisa masuk, melihat `/me`, dan menu menyesuaikan izin.

---

## U1 — Kasir bisa jualan

Tahap paling menentukan. Kalau berhenti di sini pun, produknya sudah berguna.

- Buka/tutup shift
- Layar kasir: grid produk, pencarian, penyaring kategori, keranjang, bayar
  (tunai/QRIS/kasbon)
- Pindai barcode lewat kamera — `BarcodeDetector` bawaan di Chrome Android
  (nol byte tambahan), pustaka WASM ditarik saat dibutuhkan di peramban lain.
  Jalan penuh saat offline, dan kolom ketik manual selalu tersedia di layar yang
  sama karena barcode bisa saja sobek
- `Idempotency-Key` pada checkout
- Struk di layar + cetak
- Riwayat transaksi, batalkan, retur
- Uang masuk/keluar laci

**Selesai bila:** satu hari penuh berjualan bisa dijalankan dari aplikasi, dan
angka tutup shift cocok dengan uang di laci.

---

## U2 — Barang & stok

- CRUD barang (form pendek + "Detail lainnya" yang tertutup)
- Impor CSV dengan pratinjau `dry_run` dan laporan baris gagal
- Kategori, satuan, pemasok
- Saldo stok + kartu stok
- Koreksi stok, barang masuk
- Alur onboarding 3 langkah

**Selesai bila:** pemilik baru bisa mandiri dari daftar sampai transaksi pertama.

---

## U3 — Offline

- PWA (dapat dipasang, aset di-cache)
- Dexie: master data dari `GET /sync/pull` + antrean transaksi
- Kirim antrean lewat `POST /sync/push`, tangani `applied` / `duplicate` / gagal
- Indikator koneksi yang menenangkan + halaman "perlu diperiksa"

**Selesai bila:** mode pesawat dinyalakan, 10 transaksi dibuat, internet
dinyalakan lagi → semuanya masuk tanpa duplikat.

---

## U4 — Laporan

- Beranda pemilik dengan kartu angka + pembanding
- Laporan penjualan (per hari/kanal/kasir/metode bayar)
- Laporan untung-rugi (hormati `report.profit`)
- Unduh CSV
- Grafik dengan Recharts

**Selesai bila:** pemilik bisa menjawab "hari ini untung berapa" dalam 0 ketukan.

---

## U5 — Operasional lanjutan

- Opname bertahap (satu barang satu layar)
- Transfer antar toko
- Pelanggan & kasbon
- Pengaturan: outlet, pengguna, **peran dinamis + peran ganda**

---

## U6 — Modul berbayar

Dikerjakan sesuai permintaan pasar, tidak harus berurutan.

| Modul | Catatan |
|---|---|
| Kanal online | Entri manual & impor CSV dulu; webhook menyusul kemitraan |
| CRM & sales lapangan | Kunjungan wajib jalan offline (dipakai di jalan) |
| SDM & gaji | Empat langkah: hitung → periksa → kunci → bayar |
| Langganan | Paket, tagihan, pembayaran |
| Portal mitra | Aplikasi terpisah, realm berbeda |

---

## UX — Panel internal (jalur terpisah)

Bukan bagian dari urutan U0–U6: penggunanya staf kita sendiri, bukan pelanggan.
Dibangun saat perekrutan mitra benar-benar dimulai, karena sebelum itu
`cmd/platform-admin` sudah cukup.

- Desktop saja — tidak perlu offline, tidak perlu PWA
- Menu dibentuk dari `capabilities` di `GET /platform/me`; jangan hardcode peran
- Prioritas layar: verifikasi mitra → jalankan & setujui komisi → pencairan →
  antrean notifikasi mati
- Boleh memakai token desain yang sama, tapi **tanpa nada motivasional**:
  ini alat kerja, bukan aplikasi yang harus menyemangati penggunanya

---

## Cara menguji bahwa UI-nya benar-benar mudah

Tes teknis tidak bisa mengukur "gaptek-friendly". Yang ini harus dilakukan
dengan orang sungguhan sebelum tahap dianggap selesai:

1. **Uji lima menit.** Beri HP ke pemilik warung yang belum pernah melihat
   aplikasi ini. Minta dia menjual satu barang. **Tanpa dibantu bicara.**
   Kalau gagal dalam 5 menit, alurnya yang salah.
2. **Uji sambil berisik.** Jalankan alur kasir sambil ada yang mengajak bicara —
   meniru antrean. Alur yang butuh konsentrasi penuh akan gagal di dunia nyata.
3. **Uji satu tangan.** Semua alur di HP harus bisa diselesaikan dengan satu ibu jari.
4. **Uji sinyal jelek.** Nyalakan mode pesawat di tengah transaksi. Tidak boleh
   ada data hilang dan tidak boleh ada pesan menakutkan.
5. **Uji layar silau.** Bawa tablet ke luar ruangan. Teks abu-abu muda yang
   terlihat elegan di dalam ruangan sering hilang total di bawah matahari.

---

## Yang sengaja TIDAK dibangun dulu

Menahan diri sama pentingnya dengan membangun.

| Ditunda | Alasan |
|---|---|
| Kustomisasi tema per tenant | Menambah beban perawatan besar, manfaatnya kecil di awal |
| Dashboard yang bisa diatur sendiri | Pengguna gaptek justru bingung. Beranda yang sudah tepat lebih berguna |
| Notifikasi push | Menunggu `outbox_events` punya pengirim di backend |
| Aplikasi native | PWA sudah cukup. Tinjau ulang bila cetak struk Bluetooth jadi penghalang |
| Mode multi-bahasa | Fokus Bahasa Indonesia dulu; struktur teks tetap disiapkan agar mudah ditambah |
