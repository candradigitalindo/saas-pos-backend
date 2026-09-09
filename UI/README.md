# Konsep UI — SaaS POS UMKM

Dokumen rancangan antarmuka untuk backend di repositori ini. Isinya **keputusan
dan alasannya**, bukan sekadar daftar keinginan — supaya siapa pun yang
mengerjakan frontend nanti tahu apa yang boleh diubah dan apa yang tidak.

> **Backend adalah kontraknya.** Setiap layar di dokumen ini dipetakan ke
> endpoint dan izin yang **benar-benar sudah ada** (lihat
> [04-PETA-LAYAR.md](04-PETA-LAYAR.md)). Tidak ada layar yang menjanjikan data
> yang belum bisa disediakan server.

---

## Siapa yang akan memakai aplikasi ini

Ini yang menentukan hampir semua keputusan di bawah.

| Persona | Perangkat | Kondisi nyata di lapangan |
|---|---|---|
| **Pemilik warung/toko** | HP Android kelas menengah | Bukan orang IT. Sering hanya sempat melihat aplikasi 2 menit sambil melayani pembeli. Yang dicari: "hari ini masuk berapa, untungnya berapa" |
| **Kasir** | Tablet 10" di meja kasir, kadang HP | Tangan sibuk, antrean panjang, layar kena silau. Butuh tombol besar dan alur yang bisa dihafal |
| **Staf gudang** | HP sambil berdiri di rak | Satu tangan memegang barang. Butuh input cepat, kamera untuk barcode |
| **Sales lapangan** | HP di jalan, **sinyal putus-putus** | Aplikasi wajib tetap jalan tanpa internet |
| **Mitra penjual (agen)** | HP/laptop | Pertanyaannya cuma satu: "komisi saya kapan cair dan berapa" |
| **Staf internal (kita)** | Laptop | Panel internal: verifikasi mitra, jalankan komisi, cairkan, pantau antrean notifikasi. Realm terpisah, aplikasi terpisah |

**Asumsi yang dipegang sepanjang dokumen: pengguna belum tentu melek teknologi.**
Kalau sebuah alur butuh dijelaskan lewat telepon, alur itu gagal.

---

## Daftar isi

| Dokumen | Isi |
|---|---|
| [01-PRINSIP-DESAIN.md](01-PRINSIP-DESAIN.md) | Sepuluh prinsip yang mengikat + anti-pola yang dilarang |
| [02-SISTEM-DESAIN.md](02-SISTEM-DESAIN.md) | Warna (dengan kontras terverifikasi), tipografi, jarak, komponen |
| [03-ARSITEKTUR-FRONTEND.md](03-ARSITEKTUR-FRONTEND.md) | Pilihan framework + alasannya, struktur folder, offline, kontrak API |
| [04-PETA-LAYAR.md](04-PETA-LAYAR.md) | Semua layar → endpoint → izin |
| [05-ALUR-UTAMA.md](05-ALUR-UTAMA.md) | Alur inti beserta sketsa layar |
| [06-ROADMAP-UI.md](06-ROADMAP-UI.md) | Urutan membangun, menyusul fase backend |

---

## Ringkasan keputusan

| Aspek | Keputusan | Alasan singkat |
|---|---|---|
| Framework | **React 18 + TypeScript + Vite** | Kolam talenta terbesar di Indonesia — gampang cari penerus. Vite = build cepat, konfigurasi sedikit |
| Styling | **Tailwind CSS + shadcn/ui** | Komponennya **disalin ke repo**, bukan dependensi. Bisa diubah bebas, tidak terkunci vendor. Radix di bawahnya sudah aksesibel |
| Data server | **TanStack Query** | Cache, retry, dan status "sedang mengirim" ditangani otomatis — cocok untuk sinyal jelek |
| Offline | **PWA + IndexedDB (Dexie)** | Backend sudah punya `/sync/push` & `/sync/pull`. Kasir harus tetap jualan saat internet mati |
| Warna utama | **Hijau Tumbuh** `#047857` | Hijau = tumbuh, panen, cuan. Disukai lintas kalangan dan tidak berkonotasi politik |
| Aksen | **Jingga Semangat** `#FBBF24` | Kehangatan & optimisme. Hanya untuk momen pencapaian, tidak untuk aksi biasa |
| Font | **Plus Jakarta Sans** | Buatan Indonesia, angkanya jelas, gratis, lengkap sampai bobot tebal |
| Bahasa | **Bahasa Indonesia sehari-hari** | "Uang masuk hari ini", bukan "Total Gross Revenue" |
| Target sentuh | **Minimal 48 × 48 px** | Jari, bukan kursor. Sering dipakai sambil buru-buru |

---

## Satu kalimat yang memandu semuanya

> **Aplikasi ini harus terasa seperti buku catatan yang lebih pintar — bukan
> seperti software akuntansi.**

Kalau sebuah rancangan membuat pemilik warung merasa bodoh, rancangan itu salah,
sebagus apa pun secara teknis.
