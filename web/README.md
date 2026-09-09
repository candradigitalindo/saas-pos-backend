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

3. **`Idempotency-Key` wajib** pada `POST /sales`, `/purchases`,
   `/invoice-payments`, dan `/subscription-payments`. Kuncinya dibuat SEKALI
   saat pengguna menekan Bayar, dan kunci yang sama dipakai untuk semua
   percobaan ulang — inilah yang membuat tombol tertekan dua kali tidak
   menghasilkan dua transaksi.

## Kemajuan menurut [roadmap](../ui/06-ROADMAP-UI.md)

| Tahap | Isi | Status |
|---|---|---|
| U0 | Fondasi: token desain, api-client, autentikasi, izin, layout, komponen dasar | ✅ Selesai |
| U1 | Kasir bisa jualan | ⏳ Berikutnya |
| U2 | Barang & stok | — |
| U3 | Offline (PWA + Dexie + sinkronisasi) | — |
| U4 | Laporan | — |
| U5 | Operasional lanjutan | — |
| U6 | Modul berbayar | — |
