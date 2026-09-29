# 04 — Peta Layar

Setiap layar dipetakan ke endpoint yang **sudah ada** di backend dan izin yang
menjaganya. Prefiks `/api/v1` dihilangkan agar tabel ringkas.

Kalau sebuah layar ingin ditambahkan tapi kolom endpointnya kosong — berarti
backendnya belum ada, dan itu harus dibicarakan dulu, bukan dikarang di UI.

---

## Navigasi menurut peran

Menu **dibangun dari izin**, bukan dari daftar tetap. Pengguna hanya melihat
yang bisa ia kerjakan — inilah cara utama menjaga aplikasi terasa sederhana
bagi pengguna gaptek.

### Pemilik (HP) — navigasi bawah

```
┌──────────────────────────────────────────────┐
│                                              │
│              (isi halaman)                   │
│                                              │
├──────────────────────────────────────────────┤
│  🏠      🧾       📦       📊       ⋯        │
│ Beranda  Kasir   Stok   Laporan  Lainnya     │
└──────────────────────────────────────────────┘
```

### Kasir (tablet lanskap) — tanpa navigasi, satu layar penuh

Kasir sengaja dibuat mode fokus: hanya ada Kasir, dan tombol keluar shift.
Semakin sedikit pintu, semakin kecil peluang tersesat.

### Layar lebar — navigasi samping ala SaaS (`app/layouts/navigasi-samping.tsx`)

- **Kepala:** pemilih toko · tombol utama **Buka Kasir** · kotak **Cari…** (palet perintah,
  Ctrl/⌘ K — lompat ke halaman atau aksi cepat; kata lain ikut dicocokkan: "piutang" → Kasbon,
  "kulakan" → Catat barang masuk).
- **Daftar:** `Beranda` · `Laporan`, lalu kelompok yang bisa dilipat — `Penjualan` ·
  `Barang & Stok` · `Pelanggan` · `CRM` · `SDM` (kelompok berisi halaman aktif selalu terbuka) —
  dan di ujungnya `Langganan` · `Pengaturan`. Menu aktif: latar `sorot` + garis aksen; lencana
  jumlah pada Stok (hampir habis + habis + minus); gembok pada fitur yang terkunci paket.
- **Kaki:** kartu paket (Gratis / masa coba, hanya `billing.manage`) · status sinkron · profil ·
  tombol ciutkan (lajur ikon 72px, diingat per perangkat) · keluar.
- Baris 48px di layar sentuh, 40px dengan penunjuk halus (ui/01 §3).

---

## Masuk & pendaftaran

| Layar | Endpoint | Izin |
|---|---|---|
| Daftar usaha baru | `POST /auth/register` | — (publik) |
| Masuk | `POST /auth/login` | — (publik); kolom `username` menerima nama pengguna atau email |
| Perbarui sesi (di balik layar) | `POST /auth/refresh` | — |
| Keluar | `POST /auth/logout` | wajib masuk |
| Profil & izin saya | `GET /me` | wajib masuk |

> Pendaftaran menerima `referral_code` opsional — kolomnya disembunyikan di balik
> tautan kecil "Punya kode dari agen?" supaya tidak membingungkan mayoritas
> pengguna yang mendaftar sendiri.

---

## Beranda (pemilik)

| Bagian | Endpoint | Izin |
|---|---|---|
| Kartu uang masuk, untung, jumlah transaksi | `GET /reports/dashboard` | `report.view` |
| Tren uang masuk 7/30 hari + pembanding periode sebelumnya | `GET /reports/sales?group_by=day` (satu rentang ganda) | `report.view` |
| Barang terlaris (5 teratas, periode yang sama) | `GET /reports/sales?group_by=product` | `report.view` |
| Transaksi terakhir (5 nota, tautan ke Riwayat pada tanggalnya) | `GET /sales?limit=5` | `sale.create` |
| Peringatan stok menipis | `GET /stocks?low=true` | `stock.view` |
| Pintasan aksi harian | — | sesuai izin |

Navigasi samping (≥1024px): lihat "Layar lebar" di atas. **Pemilih toko** muncul sebagai
tombol bila cabang > 1. Tepat satu menu menyala — yang jalurnya paling spesifik (`menuAktif`) — dan
menu aktif selalu digulir ke dalam pandangan.

---

## Kasir

| Layar | Endpoint | Izin |
|---|---|---|
| Buka shift | `POST /shifts/open` | `shift.open` |
| Daftar produk (grid) | `GET /products` · cache Dexie | `product.view` |
| Pilih varian — kartu barang bervarian menampilkan "mulai Rp…" + "N pilihan"; ketukan membuka dialog wajib-pilih-satu (nama varian 2 baris, harga jadi, lencana jumlah di keranjang). Pindai barcode/SKU varian langsung masuk sebagai "Barang (Varian)"; barcode barang induknya membuka dialog | varian dari `GET /sync/pull` (`product_variants`, Dexie v3) → checkout `items[].variant_id` | `sale.create` |
| Catatan & diskon di keranjang — tombol pena per baris membuka dialog: Catatan (≤200, ikut tercetak di struk & riwayat) + Diskon Rp/% (tombol cepat 5/10/15/20%, pratinjau "Rp 20.000 → Rp 18.000 (hemat …)"); "+ Diskon transaksi" satu baris di atas Total (Rp/%, persen dari belanja setelah diskon barang, sebelum pajak & layanan). Diskon persen disimpan sebagai persen (tetap 10% saat qty berubah); nominal dijepit ke batas server. Tanpa izin diskon: dialog hanya Catatan, baris diskon transaksi tidak tampil | `items[].discount_amount` · `items[].note` · `order_discount` di `POST /sales` | `sale.create` (+ `sale.discount` untuk diskon) |
| Bayar / checkout (tetap jalan saat browser offline — masuk antrean, struk "Menunggu dikirim"). **Bayar gabungan** tanpa mode terpisah: tunai kurang → "Sisanya pakai cara lain" (bagian tunai disimpan, pindah ke QRIS); QRIS → "Sebagian saja" (isi jumlah < sisa → "Tambahkan — sisanya cara lain"); bagian berbaris di bawah Total dengan ✕ untuk membatalkan dan **Sisa** jadi angka utama; kasbon selalu melunasi sisa (pas). Pintasan uang satu baris (Pas · 20.000 · 50.000 · 100.000), kotak KEMBALIAN/kurang ringkas — SELESAI terlihat tanpa gulir di 360×640 untuk semua keadaan. Dialog selalu mulai bersih saat dibuka | `POST /sales` + header `Idempotency-Key` (`payments[]` berisi beberapa baris; server menolak kembalian yang melebihi uang tunai) | `sale.create` |
| Kirim struk lewat WhatsApp — struk setelah bayar punya "Cetak" & "WhatsApp" berdampingan; rincian nota di Riwayat punya "Kirim WhatsApp" (bukan untuk yang dibatalkan). Dialog: nomor pembeli (terisi dari pelanggan transaksi; boleh kosong → pilih kontak di WhatsApp; 08…/+62…/8… dinormalkan), tautan struk disiapkan saat dialog dibuka (supaya jendela WhatsApp dibuka langsung dari ketukan), "Buka WhatsApp" membuka `wa.me` dengan teks struk + "Lihat struk: <tautan>". Offline/belum terkirim: teks saja, dijelaskan di dialog. Pengirimnya WhatsApp TOKO di perangkat itu | `POST /sales/:id/receipt-link` | `sale.create` |
| Struk digital publik `/struk/:token` — tanpa akun, satu kolom seperti struk kertas (nama & alamat cabang, kepala/kaki struk, nota & jam zona cabang, rincian, total, pembayaran, kembalian, "DIBATALKAN" bila batal), "Cetak / Simpan PDF" (tidak ikut tercetak); token salah → "Struk tidak ditemukan" | `GET /public/receipts/:token` | publik |
| Tagihan terbuka (open bill / tahan transaksi) — tombol **Tagihan** di kepala kasir (HP: ikon + lencana jumlah) membuka daftar tagihan cabang dari semua perangkat (label, jumlah barang, pratinjau total, pencatat & jam, lencana "Belum terkirim"); ketuk → dimuat ke keranjang (kepala keranjang menyebut labelnya); batal dua ketukan. Kepala keranjang punya **Tahan** (tagihan baru → dialog nama bebas "Meja 5 / Pak Budi") atau **Simpan** (tagihan yang sedang dibuka). Membuka tagihan saat keranjang berisi ditahan dengan pesan. Daftar disegarkan tiap 20 dtk selama online; offline tetap bisa tahan/ubah/batal (diantre, versi lokal naik) | `GET /open-bills?outlet_id=` · `PUT /open-bills/:id` (id ULID dari klien, `base_version`) · `POST /open-bills/:id/cancel` · bayar = `POST /sales` dengan `open_bill_id` · offline: `/sync/push` op `open_bill.upsert` / `open_bill.cancel` | `sale.create` |
| Riwayat transaksi (ringkasan hari, cari nota, saring cara bayar/status, muat bertahap) | `GET /sales?search=&method=` · `GET /sales/day-summary` | `sale.create` |
| Detail struk | `GET /sales/:id` | `sale.create` |
| Batalkan transaksi | `POST /sales/:id/void` | `sale.void` |
| Retur barang | `POST /sales/:id/refund` | `sale.refund` |
| Uang masuk/keluar laci (masuk/keluar/seharusnya di laci, akibat pada laci sebelum dicatat, keterangan sekali ketuk, siapa pencatatnya; formulir muat satu layar) | `POST /cash-movements` · `GET /cash-movements` · `GET /shifts/:id` | `cash.movement` |
| Tutup Shift — satu layar tanpa digulir (≥320×568), pola sama dengan Ganti Shift: satu angka "Hasil hitung laci" dengan pembanding "Seharusnya" di baris label, per pecahan di dialog, selisih satu baris bernada menenangkan, "+ Catatan"; rincian penjualan & rumus laci di dialog "Rincian shift" (layar lebar: kolom kiri); layar hasil (penjualan, dihitung, seharusnya, selisih) alih-alih toast. Terkunci selama ada penjualan offline belum terkirim (+ tombol "Kirim Sekarang") — server menolak penjualan untuk shift yang sudah ditutup | `GET /shifts/:id` · `POST /shifts/:id/close` | `shift.close` |
| Ganti Shift — satu layar tanpa digulir (≥360×640): siapa → siapa, hitung laci (total, atau per pecahan di dialog), tinggal semua/setor sebagian, catatan opsional; rincian penjualan & rumus laci di dialog "Rincian shift" (di layar lebar rumusnya tampil di kolom kiri); layar hasil; "Masuk sebagai kasir lain" kembali ke layar ini setelah masuk. Terkunci selama ada penjualan offline belum terkirim, sama seperti Tutup Shift | `GET /shifts/:id` · `POST /shifts/:id/handover` | `shift.close` + `shift.open` |
| Daftar & detail shift | `GET /shifts` · `GET /shifts/:id` | `shift.open` / `shift.close` |

---

## Barang & stok

| Layar | Endpoint | Izin |
|---|---|---|
| Daftar & cari barang (gambar mini, margin, saring kategori, halaman) | `GET /products` · kolom stok: `GET /stocks?product_ids=` | `product.view` (stok: `stock.view`) |
| Tambah / ubah barang | `POST /products` · `PUT /products/:id` | `product.edit` |
| Kemasan (konversi satuan) — form barang: "+ Kemasan (dus, pak)" → baris "[satuan] isi [40] pcs ✕" + "Rp [harga, placeholder isi × harga jual] [barcode]" (≤5; butuh satuan kemasan dibuat dulu di Master › Satuan). Kasir: baris keranjang barang berkemasan punya pilihan satuan jual "pcs | dus isi 40" (ganti satuan menggabung baris kembar), stepper menyebut satuannya, pindai barcode dus → "1 dus"; lencana kartu & stok dalam satuan dasar. Barang Masuk: baris barang berkemasan punya pilihan satuan beli; "Harga beli per dus" dengan keterangan "Stok masuk 40 pcs · modal Rp 5.000 per pcs". Data kemasan ikut tersalin ke perangkat (Dexie v7) | `packagings` di `/products` · `items[].product_unit_id` di `POST /sales` & `POST /purchases` | `product.edit` · `sale.create` · `stock.adjust` |
| Harga khusus pelanggan — Barang › Master punya tab **Daftar Harga** (buat/hapus "Member", "Reseller"); form barang: "+ Harga khusus (Member, …)" → satu kolom Rp per daftar (kosong = harga umum, peringatan rugi); form pelanggan: "Daftar harga" (tampil bila ada daftar). Kasir: baris **Pelanggan** di bawah kepala keranjang ("Umum" / "Bu Rina · harga Member") → dialog pilih pelanggan dari data lokal (cari nama/HP, sebut daftar harganya, "Umum"); memilih mengubah harga semua baris (tanda "· harga Member"); kasbon terkunci atas nama pelanggan keranjang ("Ganti pelanggan dari keranjang"); tagihan terbuka menyimpan & memulihkan pelanggannya; struk offline ikut harga khusus | `GET/POST/DELETE /price-lists` · `special_prices` di `/products` · `price_list_id` di `/customers` · `customer_id` di `POST /sales` | `product.edit` · `customer.edit` · `sale.create` |
| Harga grosir per jumlah — di bawah untung/margin: "+ Harga grosir (beli banyak lebih murah)"; bila diisi jadi kotak "Harga grosir" berisi baris "≥ [jumlah] (satuan) Rp [harga] ✕" (satuan disembunyikan di bawah 400px), "Tambah tingkat" (maks 5), peringatan per baris "Di bawah harga beli — rugi" / "Tidak lebih murah dari harga jual". Kasir: harga turun otomatis dari TOTAL barang itu di keranjang (varian dijumlah), baris keranjang bertanda "· harga grosir"; data harga ikut tersalin ke perangkat (Dexie v5) sehingga pratinjau & struk offline sama dengan server | `wholesale_prices` di `POST/PUT /products` · `GET /products/:id` · pull `price_lists`/`product_prices` | `product.edit` |
| Varian barang (kartu "Varian" di halaman ubah barang: daftar nama · kode · harga jadi, "Tidak tampil di kasir" untuk nonaktif; dialog nama, **harga jadi** — disimpan sebagai selisih terhadap harga jual tersimpan dan disebutkan "+Rp… dari harga jual", SKU & barcode opsional, "Tampil di kasir", hapus dua ketukan). Barang baru: "Varian bisa ditambahkan setelah barang disimpan" | `GET\|POST /products/:id/variants` · `PUT\|DELETE /products/:id/variants/:vid` | `product.view` / `product.edit` |
| Hapus barang | `DELETE /products/:id` | `product.delete` |
| Impor dari Excel/CSV | `POST /products/import` (dukung `dry_run`) | `product.import` |
| Resep (F&B) | `GET\|PUT /products/:id/recipe` | `product.view` / `product.edit` |
| Kategori, satuan, pemasok | `GET\|POST\|PUT\|DELETE /categories[/:id]` · `/units[/:id]` · `/suppliers[/:id]` | `product.view` / `product.edit` |
| Stok — kartu kesehatan (nilai stok, bilah proporsi aman/hampir habis/habis/perlu dicocokkan yang sekaligus tombol saring, "Rp X modal tertahan di N barang tidak laku 30 hari"), cari + urutan (paling mendesak [bawaan]/paling laku/nilai terbesar/nama); baris: foto, kategori, keadaan + meteran sisa-terhadap-batas, sisa, laku 30 hari + "cukup ±N hari" (≤ 3 hari ditandai), nilai stok, Koreksi/Cocokkan. Saringan & urutan di URL. HP: kartu membuka Riwayat Stok, yang kini menampilkan saldo sekarang + tombol Koreksi/Cocokkan | `GET /stocks/summary` · `GET /stocks?outlet_id=&status=&sort=&q=` | `stock.view` |
| Ringkasan stok (aman/hampir habis/habis/minus + nilai stok) | `GET /stocks/summary` | `stock.view` |
| Kartu stok (riwayat keluar-masuk) | `GET /stock-movements?product_id=` | `stock.view` |
| Koreksi Stok — tidak lagi membuka pemilih di atas layar kosong: "Pilih barang" + panel "Perlu dicocokkan" (barang tercatat minus, ketuk untuk mengoreksi) + "Koreksi terakhir" (barang, alasan, ±jumlah → jadi X, waktu & pencatat; ketuk → Riwayat Stok). Formulir: foto, "Catatan sekarang" + lencana keadaan, "Jumlah sebenarnya di rak" DIMULAI dari catatan (minus → 0) dan bisa diketik, pratinjau "13 → 11 pcs · kurang 2 · ≈ −Rp 10.000" (nilai stok, catatan minus = 0), alasan + pilihan cepat (Barang rusak, Kedaluwarsa, Hilang, Salah hitung, Dipakai sendiri); simpan terkunci bila jumlah sama dengan catatan. Dibuka dengan `?product_id=` → kembali ke asal setelah simpan; tanpa itu tetap di layar untuk koreksi berikutnya. `?onboarding=1` = "Isi Stok Awal" (tanpa panel & pilihan cepat) | `POST /stock-adjustments` + `Idempotency-Key` · `GET /stock-adjustments?outlet_id=` · `GET /stocks?status=negative` | `stock.adjust` / `stock.view` |
| Barang Masuk — "Perlu dibeli": saran belanja (di bawah batas / habis < seminggu) dengan jumlah untuk dua minggu, ketuk = masuk daftar ("+ Semua"); terlipat bila formulir diisi lewat pemilih, tetap terbuka saat orang mengambil dari saran. Baris: foto, "sisa X → jadi Y", perubahan harga modal "Rp A → Rp B (naik n%)" (≥ 50% → "periksa lagi harganya"). Kartu total menyebut berapa barang yang modalnya berubah, lalu "Pembayaran": Lunas / Sebagian ("Dibayar sekarang") / Belum bayar; bila ada yang dibayar, "Sumber uang": Uang lain (dompet/rekening) atau Dari laci kasir (uang keluar shift yang sedang buka — nonaktif dengan alasan bila kasir belum dibuka / tanpa izin uang masuk & keluar); bila ada sisa: "Sisa Rp X dicatat sebagai utang ke …", peringatan bila tanpa pemasok, jatuh tempo (tanggal + 7/14/30 hari). Pemilih barang menampilkan sisa stok + modal. "Riwayat" (dialog): nota terbaru dulu — pemasok, nota, cuplikan barang, total, waktu & pencatat; ketuk untuk rincian — tiap nota "lunas" / "sisa Rp X", rincian memuat jejak pembayaran, retur ("Diretur · alasan · barang · pemasok mengembalikan Rp X ke laci/uang lain"), "Retur barang" (dialog: jumlah per baris dibatasi sisa yang bisa diretur, alasan cepat Rusak/Kedaluwarsa/Salah kirim/Kelebihan kirim, akibat ditulis — "Utang nota turun dari A jadi B" atau "pemasok mengembalikan Rp X" + pilihan Uang lain / Laci kasir), dan tautan "Bayar di Utang Pemasok" | `GET /stocks?status=restock` · `POST /purchases` + `Idempotency-Key` · `GET /purchases?outlet_id=` · `GET /purchases/:id` | `stock.adjust` / `stock.view` |
| Pemasok (`/pemasok`, menu Barang & Stok) — cari; kartu per pemasok (terakhir dibeli dulu): inisial, nama, nomor · alamat, tombol WhatsApp, "Belanja 30 hr" / "Utang" (merah bila ada yang lewat jatuh tempo, "Lunas") / "Terakhir"; "+ Pemasok" → dialog (nama, nomor WhatsApp dengan pemeriksaan format, alamat, catatan). Rincian (`/pemasok/:id`): kontak + WhatsApp / Telepon / "Catat barang masuk" (Barang Masuk dengan `?pemasok=` terisi), ubah & hapus (ikon); angka Belanja 30 hari · Nota belanja · Utang (→ Utang Pemasok); "Pesan lagi": barang yang biasa dibeli — yang perlu dibeli (di bawah batas / habis ≤ 7 hari) sudah tercentang dengan saran ±2 minggu dalam SATUAN BELI terakhir (dus dibulatkan ke atas), sisa stok & harga terakhir, "Kirim lewat WhatsApp (N)" (teks bernomor ke `wa.me`) atau "Salin teks"; riwayat belanja (per nota, lunas/sisa). Tab Pemasok di Kategori & Satuan menautkan ke sini | `GET /suppliers[/:id]` · `POST/PUT/DELETE /suppliers[/:id]` · `GET /supplier-stats` · `GET /suppliers/:id/products` · `GET /stocks?product_ids=` · `GET /purchases?supplier_id=` | `product.view` / `product.edit` / `stock.view` |
| Utang Pemasok (`/stok/utang`, menu Barang & Stok) — total utang, "N nota · M pemasok", penanda "K lewat · Rp" & "L jatuh tempo ≤ 3 hari · Rp"; menu samping berlencana K + L; Beranda menampilkan kartu "Utang pemasok: …" (merah bila ada yang lewat, jingga bila hanya segera) HANYA saat ada yang mendesak; pemilik menerima ringkasan WhatsApp harian saat ada nota BARU lewat / segera jatuh tempo (cmd/payable-reminders); kartu per pemasok (terbesar dulu; "Tanpa pemasok" tersendiri) dengan jatuh tempo terdekat (lewat X hari / hari ini / besok / N hari lagi / tanggal), terbuka otomatis bila ≤ 3 pemasok; tiap nota: tanggal, nomor nota, cuplikan barang, sisa (dari total), jatuh tempo, "Bayar". Dialog bayar: nominal dimulai dari sisa (+ "Lunasi semua"), sumber uang, catatan, akibat ("Setelah ini … LUNAS" / "Sisa setelah ini Rp X"). Pembelian sebelum 000047 dianggap lunas | `GET /payables/summary` · `GET /purchases?unpaid=true&supplier_id=` · `POST /purchases/:id/payments` + `Idempotency-Key` | `stock.view` / `stock.adjust` (+ `cash.movement` untuk laci) |
| Hitung Fisik (opname), 3 langkah — (1) pilih: cari & saring DI SERVER (Semua / Minus / Habis / Hampir habis, dengan jumlahnya), muat 50 per kali + "Muat lebih banyak", baris berfoto & berkategori, catatan minus "tercatat −10 pcs"; kotak "N barang tercatat minus — Pilih N barang ini" memilih SEMUA yang minus; pilihan bertahan lintas saringan; "Riwayat" (dialog). (2) hitung: satu barang satu layar (muat 360×640), foto, catatan sistem ditampilkan, hitungan dimulai dari catatan (minus → 0), angka bisa diketik, selisih "Kurang 2 pcs ≈ Rp 10.000" / "Cocok dengan catatan". (3) tinjau: Dihitung/Berubah/Dilewati, "Nilai stok berubah ≈ ±Rp" (catatan minus dihitung nol), baris berubah bisa diketuk untuk menghitung ulang barang itu. Riwayat: sesi terbaru dulu — jumlah dihitung/berubah, ≈ nilai, waktu & pencatat, "Tidak selesai" untuk draf; ketuk untuk rincian per barang | `GET /stocks?status=&q=&sort=urgent` · `POST /stock-opnames` → `/:id/items` → `/:id/post` · `GET /stock-opnames?outlet_id=` · `GET /stock-opnames/:id` | `stock.opname` / `stock.view` |
| Kirim Antar Toko — kotak "N kiriman sedang di jalan ke toko ini" paling atas (dengan "Terima Barang"). Formulir: Dari (toko aktif) → Kirim ke, barang berfoto dengan "sisa di sini X" / "melebihi sisa di sini" + peringatan stok akan minus, catatan (mis. kurir). Daftar "Pengiriman" (Semua/Keluar/Masuk, hanya yang melibatkan toko ini): "Toko ini → Cabang X", cuplikan nama barang, waktu & pencatat, status Disiapkan/Di jalan/Diterima; "Rincian N barang" (nama & jumlah, catatan, waktu disiapkan/berangkat/diterima); tombol hanya untuk toko yang berwenang (asal: "Barang Sudah Berangkat"; tujuan: "Terima Barang"). Satu toko saja → keadaan kosong "Tambah Cabang" | `POST /stock-transfers` → `/:id/send` → `/:id/receive` · `GET /stock-transfers?outlet_id=` · `GET /stock-transfers/:id` · `GET /stocks?product_ids=` | `stock.transfer` / `stock.view` |
| Cocokkan ulang stok | `POST /stock-reconcile` | `stock.opname` |

---

## Pelanggan & piutang

| Layar | Endpoint | Izin |
|---|---|---|
| Daftar & detail pelanggan (avatar, kedatangan, belanja, terakhir datang, sisa kasbon) | `GET /customers` (`stats`) · `GET /customers/:id` | `customer.view` (sisa kasbon: `receivable.manage`) |
| Tambah / ubah / hapus | `POST\|PUT\|DELETE /customers[/:id]` | `customer.edit` |
| Daftar kasbon | `GET /receivables` · `GET /receivables/:id` | `receivable.manage` |
| Terima setoran kasbon | `POST /receivable-payments` + `Idempotency-Key` | `receivable.manage` |

---

## Laporan

| Layar | Endpoint | Izin |
|---|---|---|
| Ringkasan dashboard | `GET /reports/dashboard` | `report.view` |
| Laporan penjualan (per hari/jam/barang/kanal/kasir/metode bayar) | `GET /reports/sales?group_by=` | `report.view` |
| Laporan untung-rugi | `GET /reports/profit?from=&to=` | `report.profit` |
| Laporan Belanja (`/laporan/belanja`; tab Penjualan · Belanja di kedua laporan, tab tersembunyi tanpa izin stok) — rentang sama dengan Laporan Penjualan (bawaan 30 hari); Belanja (N nota · X% dari uang masuk), Uang keluar ke pemasok (dari laci · uang lain; menurut tanggal bayar), Utang saat ini (→ Utang Pemasok; sisa dari nota periode ini); grafik belanja per hari (hari kosong = 0); per pemasok (batang, nota, sisa/lunas; → rincian pemasok); 10 barang dengan belanja terbesar (qty masuk); dengan `report.export`: "Unduh nota belanja (Excel)" & "Unduh pembayaran (Excel)". Pembanding "naik/turun" sengaja tidak dipakai — belanja naik belum tentu buruk | `GET /reports/purchases` · `GET /reports/sales?group_by=day` · `GET /reports/export?type=purchases\|purchase_payments` | `report.view` + `stock.view` (+ `report.export`) |
| Unduh CSV | `GET /reports/export?type=` | `report.export` |
| Hitung ulang ringkasan | `POST /reports/rebuild-summaries` | `report.view` **dan** `outlet.manage` |

> `report.profit` dipisah dari `report.view` — pemilik sering ingin kasir melihat
> omzet tapi **tidak** melihat margin. UI harus menghormati ini: kalau tidak
> berhak, tab "Untung" tidak muncul sama sekali.

---

## Kanal penjualan online

| Layar | Endpoint | Izin |
|---|---|---|
| Kanal Online: ringkasan 30 hari (diterima bersih, pesanan, penjualan, komisi), kartu per kanal dengan kinerjanya & saklar aktif, catat pesanan (pilih kanal, perkiraan komisi), impor CSV per SKU, daftar pesanan + rincian & pembatalan | `GET\|POST\|PUT /channels[/:id]` · `POST /channels/:id/orders/import` | `channel.manage` |
| Pemetaan SKU kanal ↔ produk | `GET\|POST /channels/:id/products` · `DELETE /channels/:id/products/:pid` | `channel.manage` |
| Pesanan kanal | `GET /channel-orders` · `GET /channel-orders/:id` | `channel.order.accept` |
| Catat pesanan manual (WA/IG) — barang bervarian memuat variannya (`GET /products/:id/variants`) dan baris itu wajib dipilih variannya lewat pilihan "Reguler \| Besar" (tombol Catat terkunci + "Pilih varian X dulu"); barang bervarian boleh ditambah lagi untuk varian lain, varian kembar digabung; perkiraan harga = harga jual + selisih | `POST /channel-orders` (`items[].variant_id`) | `channel.order.accept` |
| Ubah status / batalkan | `POST /channel-orders/:id/status` · `POST /channel-orders/:id/cancel` | `channel.order.accept` |
| Impor laporan harian (CSV) | `POST /channels/:id/orders/import` | `channel.order.accept` |
| Antrean peristiwa & sinkron stok | `GET /channels/:id/events` · `GET /channels/:id/stock-syncs` | `channel.manage` |
| Proses antrean manual | `POST /channel-events/process` | `channel.manage` |
| Rekonsiliasi pencairan | `GET\|POST /channels/:id/settlements` · `POST /channels/:id/settlements/receipt` | `channel.settlement.view` |

---

## CRM & sales lapangan

| Layar | Endpoint | Izin |
|---|---|---|
| Pipeline & tahapan | `GET\|POST /pipelines` · `GET\|POST /lead-sources` | `crm.lead.view.*` / `crm.deal.edit` |
| Prospek (deal) | `GET\|POST /deals` · `GET\|PUT /deals/:id` · `POST /deals/:id/win` · `POST /deals/:id/lose` | `crm.deal.edit` |
| Aktivitas follow-up | `GET\|POST /activities` · `POST /activities/:id/complete` · `POST /activities/:id/cancel` | `crm.deal.edit` |
| Penawaran | `GET\|POST /quotations` · `GET /quotations/:id` · `POST /quotations/:id/send` · `POST /quotations/:id/reject` | `crm.deal.edit` |
| Setujui penawaran → proyek | `POST /quotations/:id/accept` | `quotation.approve` |
| Proyek, tugas, biaya | `GET\|POST /projects` · `GET\|PUT /projects/:id` · `POST /projects/:id/tasks` · `POST /projects/:id/expenses` | `crm.deal.edit` |
| Invoice | `GET\|POST /invoices` · `GET /invoices/:id` · `POST /invoices/:id/send` | `invoice.issue` |
| Batalkan invoice | `POST /invoices/:id/void` | `invoice.void` |
| Bayar invoice | `POST /invoice-payments` + `Idempotency-Key` | `invoice.issue` |
| Rencana kunjungan | `GET\|POST /visit-plans` · `GET /visit-plans/:id` | `crm.visit.checkin` |
| Kunjungan (check-in/out + GPS/foto) | `GET\|POST /visits` · `GET /visits/:id` · `POST /visits/:id/checkout` | `crm.visit.checkin` |
| Target & pencapaian sales | `GET\|POST /sales-targets` | `crm.commission.view` |
| Komisi sales | `GET\|POST /commissions` · `POST /commissions/:id/approve` · `POST /commissions/:id/pay` | `crm.commission.view` |

> **Visibilitas kepemilikan:** sales hanya melihat prospek miliknya sendiri
> (`crm.lead.view.own`); atasan dengan `crm.lead.view.all` melihat semuanya.
> UI tidak perlu menyaring — backend sudah melakukannya. Yang perlu UI lakukan:
> **jangan menampilkan filter "semua sales"** kalau izinnya tidak ada.

---

## SDM: absensi & gaji

| Layar | Endpoint | Izin |
|---|---|---|
| Daftar & detail karyawan | `GET /employees` · `GET /employees/:id` | `hr.employee.view` |
| Tambah / ubah karyawan | `POST /employees` · `PUT /employees/:id` | `hr.employee.edit` |
| Jadwal kerja mingguan | `POST /employees/:id/schedule` | `hr.employee.edit` |
| Hari libur | `GET\|POST /holidays` | `hr.attendance.view` / `hr.employee.edit` |
| Absensi | `GET\|POST /attendances` | `hr.attendance.view` |
| Ajukan & setujui koreksi absen | `POST /attendance-corrections` · `POST /attendance-corrections/:id/approve` | `hr.attendance.view` / `hr.attendance.correct` |
| Cuti & izin | `POST /leave-requests` · `POST /leave-requests/:id/approve` · `POST /leave-requests/:id/reject` | `hr.leave.request` / `hr.leave.approve` |
| Komponen gaji | `GET\|POST /payroll-rules` | `hr.payroll.run` |
| Periode gaji | `GET\|POST /payroll-periods` | `hr.payroll.run` |
| Hitung gaji | `POST /payroll-periods/:id/calculate` | `hr.payroll.run` |
| Kunci periode | `POST /payroll-periods/:id/lock` | `hr.payroll.lock` |
| Bayar gaji | `POST /payroll-periods/:id/pay` | `hr.payroll.pay` |
| Slip gaji | `GET /payroll-periods/:id/payslips` · `GET /payslips/:id` | `hr.salary.view` |
| Kasbon karyawan | `GET\|POST /employee-advances` · `POST /employee-advances/:id/disburse` | `hr.advance.approve` / `hr.salary.view` |

> `hr.salary.view` adalah izin **paling sensitif** di aplikasi. Layar slip gaji
> tidak boleh bisa dicapai dari mana pun tanpa izin ini — termasuk dari tautan
> di notifikasi.

---

## Langganan (tagihan aplikasi ke pemilik)

| Layar | Endpoint | Izin |
|---|---|---|
| Katalog paket | `GET /plans` | wajib masuk |
| Status langganan saya | `GET /subscription` | `billing.manage` |
| Mulai berlangganan | `POST /subscription` | `billing.manage` |
| Tagihan | `GET\|POST /subscription/invoices` | `billing.manage` |
| Konfirmasi sudah bayar | `POST /subscription-payment-claims` + `Idempotency-Key` | `billing.manage` |
| Ganti paket / berhenti | `POST /subscription/change-plan` · `POST /subscription/cancel` | `billing.manage` |
| Batalkan pindah paket | `POST /subscription/invoices/:id/void` | `billing.manage` |
| Berhenti berlangganan | `GET /subscription/cancel-preview` · `POST /subscription/cancel` | `billing.manage` |
| Bayar sekarang (masa coba / habis / tenggang) | `POST /subscription/invoices` lalu konfirmasi | `billing.manage` |

**Pembayaran langganan diverifikasi, bukan dinyatakan sendiri.** Dialog "Konfirmasi Pembayaran"
menampilkan rekening tujuan (`payment_instructions`; bila kosong, arahan menghubungi lewat
WhatsApp), lalu meminta cara bayar + nama pengirim/nomor referensi. Setelah terkirim, kartu tagihan
berganti "Menunggu verifikasi" (tanpa tombol bayar); bila ditolak, alasan dari staf tampil beserta
tombol "Kirim Konfirmasi Baru". Paket berubah HANYA setelah staf keuangan menyetujui di panel.
Membayar di tengah masa coba tidak menghanguskan sisanya: masa berbayar dimulai saat masa coba
berakhir. Kartu status menyebutnya ("Sudah dibayar. Masa berbayar mulai …"), dan kartu tagihan
menampilkan periode yang dibayar.

**Pindah paket berlaku setelah dibayar.** "Pindah ke Paket Ini" menerbitkan tagihan pindah paket;
kartu tagihannya berjudul "Tagihan pindah ke paket …", menyebut bahwa paket sekarang tetap
berjalan sampai pembayarannya diverifikasi (plus potongan sisa masa paket lama), dan punya tombol
"Batalkan Pindah Paket". Kartu paket tujuannya bertuliskan "Menunggu pembayaran". Kartu Gratis tidak
punya tombol untuk pelanggan berbayar — toko kembali ke Gratis sendiri bila tidak diperpanjang.

**Pindah ke Gratis** (tombol di kartu Gratis selama ada masa coba/langganan berjalan, dan tautan
"Tetap pakai Gratis" di kartu masa coba yang habis) membuka dialog berhenti yang sama. Setelahnya
kartu atas menampilkan "Anda memakai paket Gratis" — tanpa masa coba, tanggal berlaku, atau tombol
bayar — plus sisa masa coba yang masih bisa dilanjutkan. Dulu kartu Gratis memulai "masa coba paket
Gratis" dan tagihan Rp0 yang tidak bisa dibayar (000041 merapikan data yang terlanjur).

**Berhenti berlangganan** adalah tautan kecil di kartu status (jalan keluar harus ada dan jujur,
tapi bukan tombol besar). Dialognya membaca angka dari `cancel-preview` — uang kembali = dibayar −
bulan terpakai × harga normal — dan meminta bank, nomor rekening, dan nama pemilik bila ada uang
kembali. Setelah berhenti, kartu status menampilkan status pengembaliannya ("sedang kami proses" →
"sudah ditransfer … referensi …").

**Kunci paket di layar** (dari `plan` di `GET /me`; penegakannya tetap di server, 402):

- Kasir: petak **QRIS** tetap tampil tapi terkunci, bertuliskan paket yang membukanya ("Paket
  Basic"). Setoran kasbon: opsi QRIS tertulis dengan paketnya dan tidak bisa dipilih.
- Kanal Online, Prospek & Kunjungan: menu bergembok; halamannya terbuka dan data lama terbaca, tombol
  "tambah"/"catat pesanan" disembunyikan, `BannerKunciFitur` menjelaskan paketnya (tombol "Lihat
  paket" hanya untuk `billing.manage`).
- Toko & Cabang: "Tambah Cabang" hilang saat batas cabang paket tercapai.
- Langganan: kartu status mengikuti paket yang BERLAKU — masa coba, masa coba habis, tenggang,
  berhenti — dengan tombol bayar yang menerbitkan tagihan bila belum ada.

---

## Pengaturan

| Layar | Endpoint | Izin |
|---|---|---|
| Toko / cabang — termasuk pajak & biaya layanan per cabang | `GET\|POST\|PUT\|DELETE /outlets[/:id]` | `outlet.manage` (daftar `GET /outlets` juga `stock.transfer`, untuk memilih cabang tujuan kiriman) |
| Pengguna | `GET\|POST\|PUT\|DELETE /users[/:id]` — `outlet_ids` untuk membatasi cabang | `user.manage` |
| Peran & hak akses | `GET\|POST /roles` · `GET\|PUT\|DELETE /roles/:id` · `PUT /roles/:id/permissions` | `role.manage` |
| Katalog izin (untuk layar peran) | `GET /permissions` | `role.manage` |

### Layar peran — perhatian khusus

Backend mendukung **peran dinamis**: 4 peran bawaan dibuat saat pendaftaran,
lalu pemilik bebas menambah/mengubah. **Satu pengguna boleh memegang beberapa
peran** (`role_ids`), dan izinnya adalah gabungan.

UI harus menampilkan itu dengan jujur dan aman:

- Izin dikelompokkan per grup (`Penjualan`, `Stok`, `Laporan`, …) sesuai
  `group_name` dari `GET /permissions` — jangan tampilkan 50 kotak centang polos.
- Setiap izin diberi **penjelasan dalam bahasa awam**, bukan kodenya:
  `sale.void` → "Boleh membatalkan transaksi yang sudah selesai".
- Peran bawaan pemilik: kotak `role.manage` **dikunci dan diberi keterangan**
  — backend akan menolaknya (422), jadi lebih baik dicegah sebelum ditekan:
  > 🔒 Izin ini tidak bisa dilepas. Tanpa ini, tidak ada lagi yang bisa
  > mengatur hak akses di usaha Anda.
- Saat memilih peran untuk pengguna, tampilkan **pratinjau izin gabungan**
  supaya pemilik paham akibat merangkap peran.

### Pajak & biaya layanan

Diatur per cabang di dialog Toko. Pemilik mengetik **persen** (boleh berkoma,
`7,5`); web mengubahnya ke pecahan (`0.075`) dengan decimal.js, dan server
menolak tarif negatif, ≥ 100%, atau lebih dari 4 desimal pecahan (400).
Dialog menampilkan **contoh hitungan dalam rupiah** memakai kalkulator yang
sama dengan kasir, sebelum disimpan. Kasir memuat ulang tarif setiap membuka
layar bayar, jadi perubahan dari HP pemilik langsung berlaku di tablet kasir
yang terbuka seharian.

### Akses cabang

Kasir, pembukaan shift, kas laci, void/retur, dan seluruh gerakan stok
**ditolak (403)** di cabang yang bukan milik penggunanya. Aturannya:

- Pemegang `outlet.manage` bekerja di **semua** cabang; `GET /me` mengembalikan
  seluruh cabang aktif untuknya, termasuk yang baru dibuat.
- Staf lain hanya di cabang pada `outlet_ids`-nya (`user_outlets`). Staf baru
  yang dibuat tanpa `outlet_ids` otomatis mendapat semua cabang aktif — usaha
  satu cabang tidak perlu melihat pilihan ini sama sekali.
- Form pengguna baru menanyakan cabang bila usahanya punya **lebih dari satu**
  cabang aktif. Minimal satu harus dicentang.
- Toko aktif di web selalu dipilih dari `outlet_ids` di `/me`; jangan pernah
  menawarkan cabang di luar daftar itu. `/me` juga membawa `outlets[]`
  (rincian cabang: pajak, biaya layanan, zona) — sumber total kasir.
- **Membaca** juga dibatasi: daftar penjualan, shift, kas, stok, kartu stok,
  pembelian, opname, transfer, laporan, dan `sync/pull` hanya memuat cabang
  staf itu (filter `outlet_id` cabang lain → hasil kosong); detail dokumen
  cabang lain → 404, seperti data CRM milik sales lain.
- Profil `/me` terakhir dan shift aktif disimpan di perangkat
  (`lib/offline/ingatan.ts`) supaya kasir yang dibuka ulang TANPA sinyal tetap
  masuk ke layar jualan, bukan ke layar masuk.

---

## Portal mitra — aplikasi terpisah

Realm autentikasi berbeda; token tenant ditolak di sini dan sebaliknya.
**Tata letak, warna aksen, dan menu dibuat berbeda** supaya tidak ada mitra yang
mengira sedang melihat data operasional toko.

| Layar | Endpoint | Catatan |
|---|---|---|
| Masuk mitra | `POST /partner/auth/login` | memakai **email** |
| Profil mitra | `GET /partner/me` → `{partner, user}` (bentuk sama dengan respons masuk) | |
| Dashboard | `GET /partner/dashboard` | prospek, merchant aktif, komisi, pencairan terakhir |
| Prospek | `GET\|POST /partner/leads` | |
| Merchant binaan | `GET /partner/merchants` | **hanya** nama usaha, status langganan, jatuh tempo, aktif |
| Rincian komisi | `GET /partner/commissions` | |
| Riwayat pencairan | `GET /partner/payouts` | |

> **Batas privasi wajib terlihat di UI.** Di halaman merchant binaan, tampilkan
> keterangan permanen:
> "Anda hanya dapat melihat status langganan merchant. Data penjualan, produk,
> dan pelanggan mereka bersifat rahasia."
> Ini melindungi merchant sekaligus menjelaskan ke mitra kenapa datanya sedikit —
> mencegah mereka mengira aplikasinya rusak.

---

## Panel internal penyedia SaaS — aplikasi terpisah

Realm **ketiga**; token panel ditolak di rute tenant maupun portal mitra.
Menunya dibangun dari `capabilities` yang dikembalikan `GET /platform/me`, jadi
staf hanya melihat yang boleh ia kerjakan.

| Layar | Endpoint | Peran |
|---|---|---|
| Masuk panel | `POST /platform/auth/login` | — (email + password) |
| Profil & kemampuan | `GET /platform/me` | semua |
| Akun staf internal | `GET\|POST /platform/admins` · `PUT /platform/admins/:id/active` | superadmin |
| Daftar mitra | `GET /platform/partners` | semua (baca) |
| Buat mitra | `POST /platform/partners` | superadmin, operator |
| Verifikasi & aktifkan | `POST /platform/partners/:id/approve` · `POST /platform/partners/:id/suspend` | superadmin, operator |
| Tingkat komisi | `GET /platform/partner-tiers` · `POST /platform/partner-tiers` | baca: semua · tulis: operator |
| Jalankan komisi | `POST /platform/partner-commissions/run` | superadmin, finance |
| Setujui komisi | `POST /platform/partner-commissions/:id/approve` | superadmin, finance |
| Pencairan | `POST /platform/partner-payouts` · `POST /platform/partner-payouts/:id/paid` | superadmin, finance |
| Target mitra | `GET /platform/partners/:id/targets` · `POST /platform/partner-targets` | baca: semua · tulis: operator |
| Materi jualan | `GET\|POST /platform/partner-materials` | baca: semua · tulis: operator |
| Pelatihan | `GET\|POST /platform/partner-trainings` | baca: semua · tulis: operator |
| Sengketa atribusi | `GET /platform/partner-disputes` · `POST /platform/partner-disputes/:id/resolve` | baca: semua · putus: operator |
| Konfirmasi pembayaran langganan | `GET /platform/subscription-payment-claims` · `POST …/:id/approve` · `POST …/:id/reject` | superadmin, finance (`billing.verify`) |
| Pengembalian dana langganan | `GET /platform/subscription-refunds` · `POST …/:id/paid` | superadmin, finance (`billing.refund`) |
| Antrean notifikasi | `GET /platform/outbox` · `POST /platform/outbox/:id/retry` | baca: semua · ulang: superadmin |
| Template pesan | `POST /platform/notification-templates` | superadmin |

**Dua hal yang wajib terlihat di UI panel:**

1. **Pemisahan wewenang bukan sekadar teknis.** Tampilkan peran pengguna di
   kepala halaman. Orang yang memverifikasi mitra memang tidak boleh
   mencairkan uangnya — kalau tombolnya sekadar hilang tanpa penjelasan, staf
   akan mengira aplikasinya rusak.
2. **Antrean mati harus mencolok.** Notifikasi yang gagal 10 kali hanya
   terlihat di sini. Beri lencana berisi jumlahnya di menu, lengkap dengan
   alasan gagal dan tombol "Coba kirim lagi".
3. **Konfirmasi pembayaran yang menunggu juga berlencana**, dan dialog "Setujui"
   menyebut nominal, nama usaha, dan nama pengirim — staf mencocokkannya
   dengan mutasi rekening sebelum menekan. Menolak wajib beralasan karena
   tenant membacanya.

---

## Yang belum ada di backend

Jangan dirancang di UI sampai backendnya tersedia:

| Fitur | Status backend |
|---|---|
| Sambungan API kanal dengan kredensial MILIK TENANT | `GET /channel-providers` · `GET\|PUT\|DELETE /channels/:id/connection` · `POST .../test` — dialog "Sambungan API" di kartu kanal. Tersedia: **WhatsApp Cloud API** (pesanan katalog) **GoFood** (GoBiz Direct Integration: pesanan diterima, status pengemudi, pembatalan; webhook didaftarkan otomatis), **GrabFood** (Partner API: Grab memanggil server partner — OAuth per kanal, Submit Order & Push Order State), dan **Shopee** (Open Platform v2, aplikasi Seller In House System milik toko: simpan Partner ID/Key → tombol "Otorisasi Toko Shopee" membuka halaman izin di tab baru, dialog memeriksa sendiri tiap 3 detik sampai nama toko tercatat; push status pesanan → rincian diambil pekerja), dan **Tokopedia & Shop** (TikTok Shop Open API — satu API untuk toko Tokopedia & TikTok Shop di Indonesia; nama "TikTok" tidak dipakai sesuai aturan merek platform: Custom App milik toko, izin toko seperti Shopee, webhook didaftarkan otomatis per toko, pencabutan izin di Seller Center menampilkan "Izin toko perlu diberikan ulang"), dan **Lazada** (aplikasi Seller In-house milik toko + Authorized Seller Whitelist; izin toko seperti Shopee; push Message Service ditambah tarikan berkala tiap 15 menit sebagai cadangan karena push Lazada mewajibkan sertifikat OV/EV). Semua penyedia di katalog kini tersedia. Untuk Shopee, Tokopedia & Shop, dan Lazada, dialog yang tersambung menampilkan kotak **Stok ke {penyedia}**: tombol "Cocokkan Barang" (hasil: cocok/baru, SKU penyedia tanpa pasangan, listing tanpa Seller SKU), ringkasan "N barang tertaut · M menunggu dikirim · stok terakhir dikirim …", dan galat terakhir beserta SKU-nya (`GET .../stock-status`, jajak 5 dtk selama ada yang menunggu). Rincian pesanan GoFood/GrabFood yang tersambung API punya tombol utama lebar penuh **"Tandai Siap Diambil"** (`POST /channel-orders/:id/ready`) → lencana "Siap diambil sejak HH.MM — pengemudi sudah diberi tahu"; daftar pesanan menampilkan lencana "Siap HH.MM". GoFood/GrabFood yang tersambung punya kotak **Menu di {penyedia}** (jumlah barang di menu, kapan terakhir dikirim, galat ketersediaan) dengan "Atur Menu" (dialog pilih barang per kategori + cari + pilih/lepas semua; baris menampilkan harga kanal/jual) dan "Kirim Menu" yang meminta konfirmasi tegas (penjelasan penggantian + centang "Saya mengerti" sebelum tombol merah "Ganti Menu di {penyedia}" aktif). Deskripsi item menu berasal dari kolom **Deskripsi** di form barang ("Detail lainnya", penghitung x/250) |
| Pengiriman email sungguhan | Alur outbox sudah utuh & teruji. WhatsApp **sudah** terkirim sungguhan lewat sidecar `wa-gateway/` (bila `WA_GATEWAY_URL` diisi); email masih mencatat ke log sampai penyedianya dipilih |
| Deteksi kejanggalan mitra (merchant fiktif, pendaftaran beruntun) | Blueprint G.5 P1 — belum dibangun |
| Laporan biaya akuisisi per mitra & wilayah | Blueprint G.5 P1 — belum dibangun |
