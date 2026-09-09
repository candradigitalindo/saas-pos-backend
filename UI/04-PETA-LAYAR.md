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

### Layar lebar — navigasi samping, dikelompokkan

`Beranda` · `Penjualan` · `Barang & Stok` · `Pelanggan` · `Laporan` ·
`CRM` · `Kanal` · `SDM` · `Pengaturan`

---

## Masuk & pendaftaran

| Layar | Endpoint | Izin |
|---|---|---|
| Daftar usaha baru | `POST /auth/register` | — (publik) |
| Masuk | `POST /auth/login` | — (publik) |
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
| Ringkasan per hari / kanal | `GET /sales-summary?from=&to=` | `report.view` |
| Peringatan stok menipis | `GET /stocks?low=true` | `stock.view` |
| Pintasan aksi harian | — | sesuai izin |

---

## Kasir

| Layar | Endpoint | Izin |
|---|---|---|
| Buka shift | `POST /shifts/open` | `shift.open` |
| Daftar produk (grid) | `GET /products` · cache Dexie | `product.view` |
| Bayar / checkout | `POST /sales` + header `Idempotency-Key` | `sale.create` |
| Riwayat transaksi | `GET /sales` | `sale.create` |
| Detail struk | `GET /sales/:id` | `sale.create` |
| Batalkan transaksi | `POST /sales/:id/void` | `sale.void` |
| Retur barang | `POST /sales/:id/refund` | `sale.refund` |
| Uang masuk/keluar laci | `POST /cash-movements` · `GET /cash-movements` | `cash.movement` |
| Tutup shift | `POST /shifts/:id/close` | `shift.close` |
| Daftar & detail shift | `GET /shifts` · `GET /shifts/:id` | `shift.open` / `shift.close` |

---

## Barang & stok

| Layar | Endpoint | Izin |
|---|---|---|
| Daftar & cari barang | `GET /products` | `product.view` |
| Tambah / ubah barang | `POST /products` · `PUT /products/:id` | `product.edit` |
| Hapus barang | `DELETE /products/:id` | `product.delete` |
| Impor dari Excel/CSV | `POST /products/import` (dukung `dry_run`) | `product.import` |
| Resep (F&B) | `GET\|PUT /products/:id/recipe` | `product.view` / `product.edit` |
| Kategori, satuan, pemasok | `GET\|POST\|PUT\|DELETE /categories[/:id]` · `/units[/:id]` · `/suppliers[/:id]` | `product.view` / `product.edit` |
| Saldo stok | `GET /stocks?outlet_id=` | `stock.view` |
| Kartu stok (riwayat keluar-masuk) | `GET /stock-movements?product_id=` | `stock.view` |
| Koreksi stok | `POST /stock-adjustments` | `stock.adjust` |
| Barang masuk (pembelian) | `POST /purchases` + `Idempotency-Key` · `GET /purchases` · `GET /purchases/:id` | `stock.adjust` / `stock.view` |
| Hitung fisik (opname) | `POST /stock-opnames` → `POST /stock-opnames/:id/items` → `POST /stock-opnames/:id/post` · `GET /stock-opnames[/:id]` | `stock.opname` / `stock.view` |
| Kirim barang antar toko | `POST /stock-transfers` → `POST /stock-transfers/:id/send` → `POST /stock-transfers/:id/receive` · `GET /stock-transfers[/:id]` | `stock.transfer` / `stock.view` |
| Cocokkan ulang stok | `POST /stock-reconcile` | `stock.opname` |

---

## Pelanggan & piutang

| Layar | Endpoint | Izin |
|---|---|---|
| Daftar & detail pelanggan | `GET /customers` · `GET /customers/:id` | `customer.view` |
| Tambah / ubah / hapus | `POST\|PUT\|DELETE /customers[/:id]` | `customer.edit` |
| Daftar kasbon | `GET /receivables` · `GET /receivables/:id` | `receivable.manage` |
| Terima setoran kasbon | `POST /receivable-payments` | `receivable.manage` |

---

## Laporan

| Layar | Endpoint | Izin |
|---|---|---|
| Ringkasan dashboard | `GET /reports/dashboard` | `report.view` |
| Laporan penjualan (per hari/kanal/kasir/metode bayar) | `GET /reports/sales?group_by=` | `report.view` |
| Laporan untung-rugi | `GET /reports/profit?from=&to=` | `report.profit` |
| Unduh CSV | `GET /reports/export?type=` | `report.export` |
| Hitung ulang ringkasan | `POST /reports/rebuild-summaries` | `report.view` |

> `report.profit` dipisah dari `report.view` — pemilik sering ingin kasir melihat
> omzet tapi **tidak** melihat margin. UI harus menghormati ini: kalau tidak
> berhak, tab "Untung" tidak muncul sama sekali.

---

## Kanal penjualan online

| Layar | Endpoint | Izin |
|---|---|---|
| Daftar & pengaturan kanal | `GET\|POST\|PUT\|DELETE /channels[/:id]` | `channel.manage` |
| Pemetaan SKU kanal ↔ produk | `GET\|POST /channels/:id/products` · `DELETE /channels/:id/products/:pid` | `channel.manage` |
| Pesanan kanal | `GET /channel-orders` · `GET /channel-orders/:id` | `channel.order.accept` |
| Catat pesanan manual (WA/IG) | `POST /channel-orders` | `channel.order.accept` |
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
| Bayar | `POST /subscription-payments` + `Idempotency-Key` | `billing.manage` |
| Ganti paket / berhenti | `POST /subscription/change-plan` · `POST /subscription/cancel` | `billing.manage` |

---

## Pengaturan

| Layar | Endpoint | Izin |
|---|---|---|
| Toko / cabang | `GET\|POST\|PUT\|DELETE /outlets[/:id]` | `outlet.manage` |
| Pengguna | `GET\|POST\|PUT\|DELETE /users[/:id]` | `user.manage` |
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

---

## Portal mitra — aplikasi terpisah

Realm autentikasi berbeda; token tenant ditolak di sini dan sebaliknya.
**Tata letak, warna aksen, dan menu dibuat berbeda** supaya tidak ada mitra yang
mengira sedang melihat data operasional toko.

| Layar | Endpoint | Catatan |
|---|---|---|
| Masuk mitra | `POST /partner/auth/login` | memakai **email** |
| Profil mitra | `GET /partner/me` | |
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

---

## Yang belum ada di backend

Jangan dirancang di UI sampai backendnya tersedia:

| Fitur | Status backend |
|---|---|
| Adaptor API kanal per-provider (GoFood, Shopee, dll.) | **Terkunci pihak luar.** Menunggu kemitraan; blueprint F.9 melarang menjanjikannya sebelum disetujui. Sekarang lewat entri manual / CSV / webhook generik |
| Pengiriman WhatsApp/email sungguhan | Alur outbox sudah utuh & teruji; pengirim bawaan baru mencatat ke log. Tinggal menukar satu implementasi `Notifier` saat penyedia dipilih |
| Deteksi kejanggalan mitra (merchant fiktif, pendaftaran beruntun) | Blueprint G.5 P1 — belum dibangun |
| Laporan biaya akuisisi per mitra & wilayah | Blueprint G.5 P1 — belum dibangun |
