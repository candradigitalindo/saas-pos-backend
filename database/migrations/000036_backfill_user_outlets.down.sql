-- Membatalkan 000036: sengaja TIDAK menghapus apa pun. Baris hasil backfill
-- tidak bisa dibedakan dari akses yang diberikan pemilik sesudahnya, dan
-- menghapus akses cabang yang sah jauh lebih merugikan daripada membiarkan
-- baris tambahan. Skema tidak berubah, jadi tidak ada yang perlu dibalik.
SELECT 1;
