-- Membatalkan 000001. Menghapus tabel autentikasi dan sequence sinkronisasi.
-- Urutan: `users` dulu (mereferensikan `roles`), lalu `roles`, lalu sequence.
--
-- Runner membungkus berkas ini dalam satu transaksi; jangan tulis BEGIN/COMMIT.

DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS roles;
DROP SEQUENCE IF EXISTS sync_version_seq;
