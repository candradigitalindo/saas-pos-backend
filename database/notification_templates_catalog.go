package database

// Template pesan bawaan sistem (§5.14, migrasi 000032). `tenant_id` NULL =
// berlaku untuk semua tenant; tenant boleh menimpanya dengan template sendiri.
//
// Penanda {{nama}} diganti nilai dari payload peristiwa outbox. Bahasanya
// sengaja bahasa sehari-hari — penerimanya pemilik warung, bukan staf IT.

type notifTemplateSeed struct {
	Code, Channel, Subject, Body string
}

var notifTemplateCatalog = []notifTemplateSeed{
	{
		// Pengingat stok menipis (cmd/stock-reminders): {{ringkasan}} disusun
		// services.ringkasanStok.
		Code: "stock.low_digest", Channel: "whatsapp", Subject: "",
		Body: "Halo {{nama_usaha}},\n\nStok yang perlu dibeli:\n{{ringkasan}}\n\n" +
			"Catat barang masuk atau pesan ke pemasok lewat menu Pemasok di aplikasi.",
	},
	{
		// Pengingat utang pemasok (cmd/payable-reminders): {{ringkasan}}
		// disusun services.ringkasanUtang.
		Code: "payable.due_digest", Channel: "whatsapp", Subject: "",
		Body: "Halo {{nama_usaha}},\n\nPengingat utang ke pemasok:\n{{ringkasan}}\n\n" +
			"Bayar lewat menu Utang Pemasok di aplikasi — sebagian atau sekaligus.",
	},
	{
		Code: "invoice.issued", Channel: "whatsapp", Subject: "",
		Body: "Halo {{nama_usaha}} 👋\n\nTagihan langganan {{nomor}} sudah terbit.\n" +
			"Jumlah: {{total}}\nJatuh tempo: {{jatuh_tempo}}\n\n" +
			"Terima kasih sudah berjualan bersama kami.",
	},
	{
		Code: "invoice.issued", Channel: "email",
		Subject: "Tagihan {{nomor}} — {{nama_usaha}}",
		Body: "Halo {{nama_usaha}},\n\nTagihan langganan {{nomor}} sudah terbit.\n" +
			"Jumlah: {{total}}\nJatuh tempo: {{jatuh_tempo}}\n\nTerima kasih.",
	},
	{
		Code: "subscription.trial_ending", Channel: "whatsapp", Subject: "",
		Body: "Halo {{nama_usaha}} 👋\n\nMasa coba paket {{paket}} berakhir {{sampai}}. Supaya QRIS dan fitur " +
			"paket lainnya tetap jalan, bayar lewat menu Langganan — masa berbayar baru dimulai setelah masa " +
			"coba berakhir, jadi tidak ada hari yang hilang.",
	},
	{
		Code: "subscription.invoice_overdue", Channel: "whatsapp", Subject: "",
		Body: "Halo {{nama_usaha}},\n\nTagihan langganan {{nomor}} sebesar {{jumlah}} sudah lewat jatuh tempo " +
			"({{jatuh_tempo}}). Fitur paket tetap jalan selama masa tenggang — bayar lewat menu Langganan " +
			"supaya tidak terkunci.",
	},
	{
		Code: "subscription.grace_ending", Channel: "whatsapp", Subject: "",
		Body: "Halo {{nama_usaha}},\n\nMasa tenggang langganan Anda berakhir {{sampai}}. Setelah itu fitur paket " +
			"berbayar (QRIS, kanal online, CRM, cabang tambahan) terkunci sampai tagihan {{nomor}} ({{jumlah}}) " +
			"dibayar. Penjualan biasa tetap jalan.",
	},
	{
		Code: "subscription.refund_requested", Channel: "whatsapp", Subject: "",
		Body: "Halo {{nama_usaha}},\n\nLangganan Anda sudah dihentikan. Pengembalian dana {{jumlah}} akan kami " +
			"transfer ke {{rekening}}. Kami kabari lagi begitu uangnya terkirim.",
	},
	{
		Code: "subscription.refund_paid", Channel: "whatsapp", Subject: "",
		Body: "Halo {{nama_usaha}},\n\nPengembalian dana {{jumlah}} sudah kami transfer ke {{rekening}} " +
			"(referensi {{referensi}}). Terima kasih sudah memakai aplikasi kami.",
	},
	{
		Code: "subscription.payment_approved", Channel: "whatsapp", Subject: "",
		Body: "Halo {{nama_usaha}} 👋\n\nPembayaran {{jumlah}} untuk tagihan {{nomor}} sudah kami terima. " +
			"Terima kasih!\n\nFitur paket Anda sudah aktif — buka menu Langganan untuk melihat masa berlakunya.",
	},
	{
		Code: "subscription.payment_rejected", Channel: "whatsapp", Subject: "",
		Body: "Halo {{nama_usaha}},\n\nKonfirmasi pembayaran {{jumlah}} untuk tagihan {{nomor}} belum bisa kami terima:\n" +
			"{{alasan}}\n\nSilakan kirim konfirmasi baru dari menu Langganan, atau balas pesan ini bila ada pertanyaan.",
	},
	{
		Code: "invoice.due", Channel: "whatsapp", Subject: "",
		Body: "Pengingat: tagihan {{nomor}} sebesar {{total}} jatuh tempo {{jatuh_tempo}}.\n" +
			"Abaikan pesan ini bila sudah dibayar.",
	},
}
