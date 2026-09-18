# wa-gateway — sidecar WhatsApp

Menjembatani backend Go ke WhatsApp memakai [Baileys](https://baileys.wiki/).

Backend tidak bisa memanggil Baileys langsung: Baileys adalah pustaka **Node**
yang bicara WebSocket ke WhatsApp Web, bukan HTTP API. Jadi proses Node inilah
yang memegang sambungan WhatsApp, dan Go memanggilnya lewat HTTP lokal.

```
process-outbox (Go) ──HTTP──> wa-gateway (Node) ──WebSocket──> WhatsApp
```

## Baca ini dulu

Baileys **tidak resmi**. Ia meniru WhatsApp Web; tidak berafiliasi dengan
WhatsApp. Tiga akibat yang nyata:

1. **Nomor bisa diblokir.** Akun yang berkelakuan seperti robot — mengirim
   serentak, ke nomor yang tidak menyimpan kontak Anda — berisiko diblokir
   WhatsApp. Sidecar ini menyerialkan pengiriman dengan jeda minimum
   (`WA_JEDA_KIRIM_MS`, bawaan 2 detik) justru untuk mengurangi risiko itu, tapi
   risikonya tidak bisa dihapus.
2. **Bisa rusak sewaktu-waktu.** Saat WhatsApp mengubah protokolnya, Baileys
   ikut rusak sampai ada versi baru.
3. **Sesi butuh manusia.** Penautan lewat pindai QR, dan sesi bisa diakhiri dari
   ponsel. Saat itu terjadi, notifikasi berhenti sampai ada yang memindai ulang.

Kalau notifikasi tagihan tidak boleh gagal, WhatsApp Business API resmi atau
gateway berbayar lebih tenang. Menukarnya cukup satu implementasi `Notifier`
baru di Go — lihat `services/outbox_service.go`; inti aplikasi tidak berubah.

## Menjalankan

```bash
npm install
export WA_GATEWAY_TOKEN=$(openssl rand -hex 32)   # simpan, backend butuh yang sama
npm start
```

Saat pertama jalan, QR muncul di terminal. Pindai lewat WhatsApp di ponsel:
**Setelan → Perangkat tertaut → Tautkan perangkat**. Setelah tertaut,
kredensial tersimpan di `WA_AUTH_DIR` dan tidak diminta lagi.

Lalu arahkan backend ke sini:

```bash
# .env di akar repo
WA_GATEWAY_URL=http://127.0.0.1:8090
WA_GATEWAY_TOKEN=<token yang sama persis>
```

Tanpa `WA_GATEWAY_URL`, backend tetap di **mode catat saja**: notifikasi ditulis
ke log, tidak dikirim. Alur outbox tetap utuh dan bisa diuji tanpa menautkan
nomor sungguhan.

## Konfigurasi

| Variabel | Bawaan | Keterangan |
|---|---|---|
| `WA_GATEWAY_TOKEN` | — | **Wajib**, minimal 32 karakter. Tanpa ini layanan menolak jalan. |
| `WA_GATEWAY_HOST` | `127.0.0.1` | Hanya localhost. Ubah ke `0.0.0.0` hanya di dalam kontainer. |
| `WA_GATEWAY_PORT` | `8090` | |
| `WA_AUTH_DIR` | `./auth` | Folder kredensial. **Harus persisten.** |
| `WA_JEDA_KIRIM_MS` | `2000` | Jeda minimum antar pesan. |
| `WA_LOG_LEVEL` | `info` | |
| `WA_LOG_FORMAT` | `text` | `json` untuk produksi. |

## Dua hal yang paling mudah membuat celaka

1. **Folder `auth/` setara kunci akun WhatsApp usaha Anda.** Siapa pun yang
   menyalinnya bisa mengirim pesan sebagai nomor itu, dan membaca yang masuk.
   Sudah masuk `.gitignore`; cadangkan di luar git, jangan kirim lewat chat.

2. **Port ini tidak boleh terbuka ke publik.** Tidak ada pembatasan siapa yang
   boleh dikirimi pesan — hanya token yang menjaganya. Bawaannya mengikat ke
   `127.0.0.1`; bila perlu dijangkau mesin lain, taruh di belakang reverse proxy
   ber-TLS.

## API

Semua jalur kecuali `/health` butuh `Authorization: Bearer <WA_GATEWAY_TOKEN>`.

| Jalur | Guna |
|---|---|
| `GET /health` | Liveness. Tanpa token — tidak membocorkan apa pun. |
| `GET /status` | Status sambungan, nomor tertaut, apakah perlu pindai QR. |
| `GET /qr` | Isi QR saat menunggu penautan. Berpagar token: yang memindainya jadi perangkat tertaut. |
| `POST /kirim` | `{"to":"628…","isi":"…","topik_id":"…"}` |

Kode balasan `/kirim` yang menentukan nasib peristiwa di antrean outbox:

| Kode | Arti | Yang dilakukan backend |
|---|---|---|
| `200` | Terkirim | Selesai |
| `400` | Permintaan cacat | Antrean mati — tidak akan membaik |
| `422` | Nomor tidak terdaftar di WhatsApp | Antrean mati — operator perlu memperbaiki nomornya |
| `401` | Token salah | **Dicoba ulang.** Ini salah konfigurasi; kalau dimatikan permanen, seluruh antrean hangus dalam hitungan detik |
| `503` | Belum tertaut / sambungan putus | Dicoba ulang dengan penundaan bertambah |

Nomor dikirim sudah dalam bentuk MSISDN (`628…`) — backend yang
menormalkannya dari apa pun yang diketik pemilik warung (`0812-3456-7890`,
`+62 812…`, `812…`). Lihat `NormalkanNomorWA` di `services/nomor_hp.go`.

## Saat notifikasi berhenti terkirim

```bash
curl -H "Authorization: Bearer $WA_GATEWAY_TOKEN" http://127.0.0.1:8090/status
```

| `status` | Artinya | Tindakan |
|---|---|---|
| `tersambung` | Sehat | Periksa antrean di panel internal |
| `menunggu-pindai` | Belum tertaut | Pindai QR dari log |
| `terputus` | Sedang menyambung ulang sendiri | Tunggu; kalau menetap, periksa jaringan |
| `keluar` | **Sesi diakhiri dari ponsel** | Hapus folder `auth/`, jalankan ulang, pindai QR baru |

Peristiwa yang gagal tidak hilang: ia mengendap di `outbox_events` dan bisa
dicoba ulang dari panel internal setelah masalahnya beres.
