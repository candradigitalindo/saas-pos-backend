-- 000043: deskripsi barang — ditampilkan di menu aplikasi antar (GoFood
-- memotong di 250 karakter, Grab menerima lebih panjang).
ALTER TABLE products ADD COLUMN description TEXT NOT NULL DEFAULT '';
