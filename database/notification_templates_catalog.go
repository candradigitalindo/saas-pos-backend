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
