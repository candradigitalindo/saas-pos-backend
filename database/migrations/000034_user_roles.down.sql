-- Membatalkan 000034. `users.role_id` tetap menyimpan peran utama, jadi hak
-- akses kembali ke perilaku satu-peran tanpa kehilangan data.
DROP TABLE IF EXISTS user_roles;
