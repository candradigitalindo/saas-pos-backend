# Frontend — SaaS POS UMKM

Aplikasi web untuk backend di repositori ini. Rancangannya ada di
[`../ui/`](../ui/) — **dokumen itu yang mengikat**, kode ini mengikutinya.

Kalau sebuah keputusan di sini terasa aneh, kemungkinan besar alasannya
tertulis di salah satu dokumen tersebut. Ubah dokumennya dulu, baru kodenya.

## Menjalankan

```bash
npm install
npm run dev          # http://localhost:5173
```

Backend harus jalan di `http://localhost:8080` (`go run .` dari akar repo).
Dev server mem-proxy `/api` ke sana, jadi frontend dan API berbagi origin dan
CORS tidak ikut campur saat pengembangan.

```bash
npm run build        # tsc + bundel produksi
npm test             # unit test (Vitest)
npm run lint         # typecheck saja
```

## Struktur

```
src/
├─ app/            rute, penjaga izin, layout, definisi navigasi
├─ fitur/          satu folder = satu modul backend (auth, kasir, produk, …)
├─ bersama/        ui/ komponen/ hooks/ util/ — dipakai lintas fitur
├─ lib/            api-client, penyimpanan sesi, konstanta izin
└─ styles/         tokens.css — sumber kebenaran warna & tipografi
```

**Aturan:** modul di `fitur/` tidak boleh saling mengimpor. Kalau butuh
berbagi, naikkan ke `bersama/`.

## Tiga hal yang paling gampang dilanggar

1. **Uang selalu bilangan bulat rupiah.** Tidak pernah pecahan JavaScript, dan
   pembulatan tidak pernah dilakukan di frontend — server membulatkan per baris
   lalu menjumlahkan. Perhitungan di klien hanya untuk pratinjau; angka final
   selalu dari balasan server. Lihat `bersama/util/uang.ts`.

2. **Izin menyembunyikan, bukan menonaktifkan.** Tombol abu-abu yang tidak bisa
   diklik membuat pengguna menelepon dukungan. Kalau tidak berhak, tombolnya
   tidak ada. Lihat `useIzin()` di `bersama/hooks/use-sesi.tsx`.

3. **Kasir membaca katalog dari Dexie, bukan dari server.** Bukan sekadar
   optimasi: kasir wajib tetap bisa menjual saat internet mati, dan sasaran
   "cari barang < 100 ms" tidak mungkin lewat jaringan. Layar kasir karena itu
   tidak pernah "beralih mode" saat sinyal hilang — sumbernya memang selalu
   lokal, dan mesin sinkronisasi yang menyegarkan di belakang.

4. **`Idempotency-Key` wajib** pada `POST /sales`, `/purchases`,
   `/invoice-payments`, dan `/subscription-payments`. Kuncinya dibuat SEKALI
   saat pengguna menekan Bayar, dan kunci yang sama dipakai untuk semua
   percobaan ulang — inilah yang membuat tombol tertekan dua kali tidak
   menghasilkan dua transaksi.

## Kemajuan menurut [roadmap](../ui/06-ROADMAP-UI.md)

| Tahap | Isi | Status |
|---|---|---|
| U0 | Fondasi: token desain, api-client, autentikasi, izin, layout, komponen dasar | ✅ Selesai |
| U1 | Kasir: buka/tutup shift, grid barang, keranjang, bayar, struk, riwayat, batalkan, kas laci | ✅ Selesai |
| U2 | Barang & stok: CRUD barang, impor CSV, kategori/satuan/pemasok, saldo & kartu stok, koreksi, barang masuk | ✅ Selesai |
| U3 | Offline: PWA, Dexie, antrean kirim, halaman "belum terkirim", pecah kode per rute | ✅ Selesai |
| U4 | Laporan: beranda berangka, untung-rugi, per hari/kanal/kasir/metode bayar, grafik, unduh CSV | ✅ Selesai |
| U5 | Operasional lanjutan: hitung fisik, kirim antar toko, pelanggan & kasbon, pengaturan (toko, pengguna, peran dinamis + peran ganda) | ✅ Selesai |
| U6 | Modul berbayar (kanal, CRM, SDM & gaji, langganan, portal mitra) | ⏳ Berikutnya |
| UX | Panel internal penyedia SaaS | — |
