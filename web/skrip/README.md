# Skrip pemeriksaan UI

Tiga pemeriksaan yang **mengukur**, bukan menilai dari pandangan mata. Ketiganya
butuh backend di `:8080` dan dev server di `:5173` yang sudah berisi data.

| Perintah | Yang diperiksa |
|---|---|
| `npm run periksa:kontras` | Rasio kontras seluruh pasangan warna teks/latar di mode terang DAN gelap. Keluar dengan kode ≠ 0 bila ada yang di bawah 4,5:1 |
| `npm run periksa:tataletak` | Target sentuh < 48px, teks < 13px, gulir mendatar, ID mentah yang bocor ke layar — di seluruh halaman |
| `npm run tangkap-layar` | Tangkapan layar tiap halaman pada tiga rentang (HP 390 · tablet 1024 · desktop 1440) |

## Kenapa diukur, bukan dilihat

Tiga bug ini lolos dari pemeriksaan mata dan baru ketahuan setelah diukur:

- **Ukuran huruf hilang diam-diam.** `tailwind-merge` menganggap `text-angka`
  (ukuran) dan `text-teks-utama` (warna) satu kelompok, lalu membuang salah
  satunya. Angka "Untung bersih" mengecil dari 40px ke ukuran biasa tanpa ada
  yang menyadari.
- **Mode gelap tidak terbaca.** Warna teks semantik dibiarkan memakai nilai
  versi terang; merah `#B91C1C` di atas `#292524` cuma 2,34:1.
- **Muat ulang halaman menghabiskan jatah rate limit.** Access token yang hanya
  di memori memaksa satu `/auth/refresh` setiap muat ulang, dan batasnya
  per alamat IP.
