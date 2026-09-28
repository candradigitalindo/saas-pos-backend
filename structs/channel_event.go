package structs

// DTO pipeline peristiwa kanal (Fase 11b, §5.10, blueprint F.6/F.8).
//
// Alur: webhook/polling → channel_events (mentah) → pekerja asinkron → adaptor
// → sales + channel_fees + antrean channel_stock_syncs. Rekonsiliasi pencairan
// lewat channel_settlements.

// ── Webhook ───────────────────────────────────────────────────────────────

// WebhookIngestResult adalah balasan cepat webhook. Selalu HTTP 200 kecuali
// kegagalan infrastruktur — kanal akan mengirim ulang bila balasan bukan 2xx.
type WebhookIngestResult struct {
	Accepted    bool   `json:"accepted"`               // peristiwa tersimpan (atau sudah ada)
	Duplicate   bool   `json:"duplicate"`              // sudah pernah diterima (idempoten)
	EventType   string `json:"event_type,omitempty"`   // tipe ternormalisasi
	ExternalRef string `json:"external_ref,omitempty"` // id pesanan eksternal
	Reason      string `json:"reason,omitempty"`       // alasan bila accepted=false
}

// ── Pekerja ───────────────────────────────────────────────────────────────

// ChannelProductMatchResult: hasil "Cocokkan barang" dengan listing penyedia.
type ChannelProductMatchResult struct {
	Listings       int      `json:"listings"`    // SKU unik di penyedia
	Matched        int      `json:"matched"`     // terpetakan ke barang toko (lama + baru)
	Created        int      `json:"created"`     // pemetaan baru
	WithoutSKU     int      `json:"without_sku"` // listing tanpa Seller SKU
	Unmatched      []string `json:"unmatched"`   // SKU penyedia tanpa pasangan (maks 20)
	UnmatchedCount int      `json:"unmatched_count"`
}

// ChannelStockStatusResponse: ringkasan sinkron stok satu kanal.
type ChannelStockStatusResponse struct {
	Supported    bool   `json:"supported"`
	Mapped       int64  `json:"mapped"` // pemetaan barang di kanal ini
	Linked       int64  `json:"linked"` // … yang sudah bertaut ke listing penyedia
	Pending      int64  `json:"pending"`
	Failed       int64  `json:"failed"`
	LastSentAt   string `json:"last_sent_at,omitempty"`
	LastError    string `json:"last_error,omitempty"`
	LastErrorSKU string `json:"last_error_sku,omitempty"`
}

// ChannelWorkerResult merangkum satu putaran pekerja pemroses.
type ChannelWorkerResult struct {
	EventsDone     int `json:"events_done"`
	EventsFailed   int `json:"events_failed"`
	EventsDead     int `json:"events_dead"` // mencapai batas percobaan → antrean mati
	StockSyncsSent int `json:"stock_syncs_sent"`
	StockSyncsFail int `json:"stock_syncs_failed"`
}

// ── Channel event (inbox) ─────────────────────────────────────────────────

type ChannelEventResponse struct {
	ID          string `json:"id"`
	ChannelID   string `json:"channel_id"`
	EventType   string `json:"event_type"`
	ExternalRef string `json:"external_ref,omitempty"`
	Status      string `json:"status"`
	Attempts    int    `json:"attempts"`
	LastError   string `json:"last_error,omitempty"`
	ReceivedAt  string `json:"received_at"`
	ProcessedAt string `json:"processed_at,omitempty"`
}

// ── Channel settlement (rekonsiliasi pencairan) ───────────────────────────

// SettlementRecomputeRequest menghitung ulang nilai kotor & potongan sebuah
// periode dari penjualan + channel_fees.
type SettlementRecomputeRequest struct {
	PeriodStart string `json:"period_start" binding:"required"` // YYYY-MM-DD
	PeriodEnd   string `json:"period_end" binding:"required"`   // YYYY-MM-DD
	Note        string `json:"note" binding:"omitempty,max=200"`
}

// SettlementReceiptRequest mencatat uang yang benar-benar masuk rekening untuk
// sebuah periode; status jadi 'matched'/'mismatch' terhadap net.
type SettlementReceiptRequest struct {
	PeriodStart    string `json:"period_start" binding:"required"` // YYYY-MM-DD
	ReceivedAmount int64  `json:"received_amount" binding:"gte=0"`
	ReceivedAt     string `json:"received_at" binding:"omitempty"` // RFC3339
	Note           string `json:"note" binding:"omitempty,max=200"`
}

type ChannelSettlementResponse struct {
	ID             string `json:"id"`
	ChannelID      string `json:"channel_id"`
	PeriodStart    string `json:"period_start"`
	PeriodEnd      string `json:"period_end"`
	GrossAmount    int64  `json:"gross_amount"`
	FeeAmount      int64  `json:"fee_amount"`
	NetAmount      int64  `json:"net_amount"`
	ReceivedAmount *int64 `json:"received_amount"`
	Difference     int64  `json:"difference"` // received - net (0 = cocok)
	Status         string `json:"status"`
	ReceivedAt     string `json:"received_at,omitempty"`
	Note           string `json:"note,omitempty"`
}

// ── Channel stock sync (antrean) ─────────────────────────────────────────

type ChannelStockSyncResponse struct {
	ID           string `json:"id"`
	ChannelID    string `json:"channel_id"`
	ProductID    string `json:"product_id"`
	RequestedQty string `json:"requested_qty"`
	Status       string `json:"status"`
	Attempts     int    `json:"attempts"`
	LastError    string `json:"last_error,omitempty"`
	QueuedAt     string `json:"queued_at"`
	SentAt       string `json:"sent_at,omitempty"`
	LateMinutes  int64  `json:"late_minutes"` // umur antrean bila belum terkirim
}
