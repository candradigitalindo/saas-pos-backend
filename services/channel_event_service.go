package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"candra/backend-api/helpers"
	"candra/backend-api/internal/reqctx"
	"candra/backend-api/models"
	"candra/backend-api/repositories"
	"candra/backend-api/structs"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// Pipeline peristiwa kanal — Fase 11b (§5.10, blueprint F.6/F.8).
//
// Alur (blueprint F.6):
//
//	kanal → webhook/polling → channel_events (MENTAH, belum diproses)
//	                               ↓ pekerja asinkron (cmd/process-channel-events)
//	                       adaptor kanal (antarmuka seragam)
//	                               ↓
//	              sales + sale_items + stock_movements + channel_fees
//	                               ↓
//	                     antrean channel_stock_syncs
//
// Aturan yang mengikat (blueprint F.6):
//  1. Webhook TIDAK pernah memproses langsung — simpan mentah, balas cepat.
//  2. Idempoten berdasarkan external_order_id — kanal mengirim ganda itu normal.
//  3. Kegagalan kanal TIDAK menghentikan kasir — pipeline di luar jalur checkout.
//  4. Percobaan ulang bertahap + antrean mati (status 'dead') yang bisa dilihat
//     pemilik.
//
// Adaptor spesifik-provider (gofood, shopee, ...) menyusul seiring kemitraan API
// disetujui (blueprint F.9). Sampai itu ada, semua provider memakai
// `genericAdapter` yang menerima payload yang SUDAH ternormalisasi — cukup untuk
// klien webhook internal / jembatan pihak ketiga.

// ── Adaptor kanal ─────────────────────────────────────────────────────────

// ChannelAdapter menerjemahkan payload MENTAH sebuah kanal menjadi bentuk
// seragam yang dimengerti pipeline. Satu kanal baru = satu implementasi baru,
// tanpa mengubah inti.
type ChannelAdapter interface {
	// Normalize mengurai satu payload peristiwa mentah.
	Normalize(raw []byte) (NormalizedEvent, error)
}

// NormalizedEvent adalah peristiwa kanal dalam bentuk seragam.
type NormalizedEvent struct {
	EventType       string // order.created | order.status | order.canceled
	ExternalOrderID string
	ExternalStatus  string
	BuyerName       string
	BuyerPhone      string
	ShippingAddress string
	Courier         string
	Reason          string
	OccurredAt      *time.Time
	Items           []NormalizedItem
	Fees            []NormalizedFee
	// IgnoreIfMissing: status/pembatalan untuk pesanan yang belum pernah
	// tercatat diabaikan, bukan dicoba ulang sampai mati. Di aplikator itu
	// wajar — pesanan yang ditolak/kedaluwarsa sebelum diterima toko tetap
	// mengirim "cancelled".
	IgnoreIfMissing bool
}

// NormalizedItem adalah satu baris pesanan ternormalisasi. `ProductID` boleh
// kosong bila `SKU` diisi — pipeline memetakannya lewat channel_products.
type NormalizedItem struct {
	SKU       string
	ProductID string
	VariantID string
	Qty       decimal.Decimal
	UnitPrice *int64 // override harga; nil → harga kanal/master
}

// NormalizedFee adalah satu komponen potongan kanal (komisi, layanan, ongkir,
// promo, pajak). Total masuk `sale_payments.fee_amount`; rincian ke channel_fees.
type NormalizedFee struct {
	Kind   string
	Amount int64
	Note   string
}

// adapterFor memilih adaptor untuk MEMPROSES payload tersimpan. Selalu generik:
// adaptor penyedia nyata (channel_provider.go) sudah menerjemahkan payloadnya
// ke bentuk generik saat webhook diterima, dengan kredensial milik tenant.
func adapterFor(string) ChannelAdapter {
	return genericAdapter{}
}

// genericAdapter menerima payload yang SUDAH berbentuk normalized JSON.
type genericAdapter struct{}

// genericPayload adalah kontrak JSON adaptor generik. Field asing diabaikan.
type genericPayload struct {
	EventType       string `json:"event_type"`
	ExternalOrderID string `json:"external_order_id"`
	ExternalStatus  string `json:"external_status"`
	BuyerName       string `json:"buyer_name"`
	BuyerPhone      string `json:"buyer_phone"`
	ShippingAddress string `json:"shipping_address"`
	Courier         string `json:"courier"`
	Reason          string `json:"reason"`
	OccurredAt      string `json:"occurred_at"` // RFC3339
	IgnoreIfMissing bool   `json:"ignore_if_missing"`
	Items           []struct {
		SKU       string `json:"sku"`
		ProductID string `json:"product_id"`
		VariantID string `json:"variant_id"`
		Qty       string `json:"qty"`
		UnitPrice *int64 `json:"unit_price"`
	} `json:"items"`
	Fees []struct {
		Kind   string `json:"kind"`
		Amount int64  `json:"amount"`
		Note   string `json:"note"`
	} `json:"fees"`
}

// Normalize mengurai payload generik.
func (genericAdapter) Normalize(raw []byte) (NormalizedEvent, error) {
	var p genericPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return NormalizedEvent{}, fmt.Errorf("json tidak valid: %w", err)
	}
	ev := NormalizedEvent{
		EventType:       p.EventType,
		ExternalOrderID: strings.TrimSpace(p.ExternalOrderID),
		ExternalStatus:  strings.TrimSpace(p.ExternalStatus),
		BuyerName:       p.BuyerName,
		BuyerPhone:      p.BuyerPhone,
		ShippingAddress: p.ShippingAddress,
		Courier:         p.Courier,
		Reason:          p.Reason,
		IgnoreIfMissing: p.IgnoreIfMissing,
	}
	if s := strings.TrimSpace(p.OccurredAt); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			ev.OccurredAt = &t
		}
	}
	for i, it := range p.Items {
		qty := decimal.Zero
		if s := strings.TrimSpace(it.Qty); s != "" {
			q, err := decimal.NewFromString(s)
			if err != nil {
				return NormalizedEvent{}, fmt.Errorf("items[%d].qty bukan angka: %q", i, it.Qty)
			}
			qty = q
		}
		ev.Items = append(ev.Items, NormalizedItem{
			SKU:       strings.TrimSpace(it.SKU),
			ProductID: strings.TrimSpace(it.ProductID),
			VariantID: strings.TrimSpace(it.VariantID),
			Qty:       qty,
			UnitPrice: it.UnitPrice,
		})
	}
	for _, f := range p.Fees {
		ev.Fees = append(ev.Fees, NormalizedFee{Kind: f.Kind, Amount: f.Amount, Note: f.Note})
	}
	return ev, nil
}

// normalizeEventType menyeragamkan nama tipe peristiwa dari beragam ejaan kanal.
func normalizeEventType(s string) string {
	switch strings.ToLower(strings.TrimSpace(strings.ReplaceAll(s, "_", "."))) {
	case "", "order.created", "order.create", "order.new", "order":
		return "order.created"
	case "order.status", "order.status.changed", "order.updated", "status":
		return "order.status"
	case "order.canceled", "order.cancelled", "order.cancel", "cancel":
		return "order.canceled"
	default:
		return strings.ToLower(strings.TrimSpace(s))
	}
}

// normalizeFeeKind memetakan label biaya kanal ke enum channel_fees.kind;
// yang tak dikenal jadi 'other'.
func normalizeFeeKind(s string) string {
	k := strings.ToLower(strings.TrimSpace(s))
	for _, v := range models.ChannelFeeKinds {
		if v == k {
			return k
		}
	}
	return "other"
}

// ── Webhook: ingest (jalur cepat, TIDAK memproses) ───────────────────────

// IngestChannelEvent menyimpan satu peristiwa kanal MENTAH ke channel_events
// lalu langsung kembali. Tidak ada pemrosesan di sini (blueprint F.6 aturan 1).
//
// LINTAS-tenant: kanal ditautkan lewat (provider, merchant_ref) yang unik
// global (indeks ux_channels_provider_merchant_ref, migrasi 000024).
//
// Selalu mengembalikan error nil untuk kegagalan "bisnis" (kanal tak dikenal,
// payload rusak, ref kosong, duplikat) — controller membalas HTTP 200 agar
// kanal tidak menonaktifkan webhook. error non-nil hanya untuk kegagalan
// infrastruktur (→ 500, kanal akan mengirim ulang).
func IngestChannelEvent(ctx context.Context, provider, merchantRef string, raw []byte) (structs.WebhookIngestResult, error) {
	var res structs.WebhookIngestResult
	provider = strings.ToLower(strings.TrimSpace(provider))
	merchantRef = strings.TrimSpace(merchantRef)
	if provider == "" || merchantRef == "" {
		res.Reason = "provider & merchant_ref wajib"
		return res, nil
	}
	if len(raw) == 0 {
		res.Reason = "payload kosong"
		return res, nil
	}

	ch, err := repositories.FindChannelByProviderRef(ctx, provider, merchantRef)
	if errors.Is(err, repositories.ErrChannelNotFound) {
		res.Reason = "kanal tak dikenal untuk (provider, merchant_ref) itu"
		return res, nil
	}
	if err != nil {
		return res, err
	}
	// Jalur lama ini tidak bertanda tangan. Kanal yang sudah tersambung API
	// hanya menerima webhook di alamatnya sendiri yang diverifikasi — tanpa
	// penjaga ini siapa pun yang tahu nomor/ID tokonya bisa menyuntik pesanan.
	if len(ch.CredentialsEncrypted) > 0 {
		res.Reason = "kanal ini tersambung API; kirim ke alamat webhook di pengaturan kanal"
		return res, nil
	}

	norm, nerr := adapterFor(ch.Provider).Normalize(raw)
	if nerr != nil {
		res.Reason = "payload tidak dapat diurai: " + nerr.Error()
		return res, nil
	}
	evType := normalizeEventType(norm.EventType)
	extRef := strings.TrimSpace(norm.ExternalOrderID)
	if extRef == "" {
		res.Reason = "external_order_id wajib ada di payload"
		return res, nil
	}

	created, err := repositories.InsertChannelEvent(ctx, ch.TenantID, ch.ID, evType, extRef, raw)
	if err != nil {
		return res, err
	}
	res.Accepted = true
	res.Duplicate = !created
	res.EventType = evType
	res.ExternalRef = extRef
	return res, nil
}

// ── Pekerja: proses antrean ──────────────────────────────────────────────

// ProcessChannelEvents memproses peristiwa kanal yang menunggu, satu per
// transaksi, dengan FOR UPDATE SKIP LOCKED sehingga aman dijalankan beberapa
// instance. GLOBAL (semua tenant) — dipanggil cmd/process-channel-events.
//
// Idempoten: RecordChannelOrder mengenali pesanan yang sudah tercatat, jadi
// peristiwa yang diproses ulang (mis. setelah crash sebelum status tersimpan)
// tidak membuat penjualan kedua.
func ProcessChannelEvents(ctx context.Context) (structs.ChannelWorkerResult, error) {
	var res structs.ChannelWorkerResult
	const maxIter = 1000 // pengaman: jangan berputar tanpa batas dalam satu panggilan
	for i := 0; i < maxIter; i++ {
		handled := false
		// Satu peristiwa per transaksi. Baris channel_events terkunci
		// (FOR UPDATE SKIP LOCKED) selama penerapan; penerapan sendiri membuka
		// transaksi ber-tenant sendiri (koneksi lain dari pool) untuk menulis
		// sales dkk — tabelnya lepas dari channel_events, jadi tak ada siklus
		// kunci. Bila transaksi ini batal setelah penjualan tercatat, peristiwa
		// tetap 'pending' dan diproses ulang — RecordChannelOrder idempoten.
		err := repositories.Transaction(ctx, func(tx *gorm.DB) error {
			evs, err := repositories.ClaimPendingEvents(ctx, tx, 1)
			if err != nil {
				return err
			}
			if len(evs) == 0 {
				return nil
			}
			handled = true
			ev := evs[0]
			status, lastErr := dispatchChannelEvent(ctx, ev)
			switch status {
			case "done":
				res.EventsDone++
			case "dead":
				res.EventsDead++
			default:
				res.EventsFailed++
			}
			return repositories.SaveChannelEventResult(ctx, tx, ev.ID, status, lastErr, ev.Attempts+1)
		})
		if err != nil {
			return res, err
		}
		if !handled {
			break
		}
	}

	sent, failed, err := ProcessStockSyncs(ctx)
	res.StockSyncsSent = sent
	res.StockSyncsFail = failed
	return res, err
}

// ProcessStockSyncs mengirim permintaan sinkron stok yang menunggu. Adaptor
// generik = no-op (belum ada API kanal) → langsung ditandai 'sent'. Adaptor
// provider nyata akan mendorong stok di sini. GLOBAL.
func ProcessStockSyncs(ctx context.Context) (sent, failed int, err error) {
	const maxIter = 1000
	for i := 0; i < maxIter; i++ {
		n := 0
		e := repositories.Transaction(ctx, func(tx *gorm.DB) error {
			rows, err := repositories.ClaimPendingStockSyncs(ctx, tx, 50)
			if err != nil {
				return err
			}
			n = len(rows)
			for _, r := range rows {
				if err := repositories.SaveStockSyncResult(ctx, tx, r.ID, "sent", "", r.Attempts+1); err != nil {
					return err
				}
				sent++
			}
			return nil
		})
		if e != nil {
			return sent, failed, e
		}
		if n == 0 {
			break
		}
	}
	return sent, failed, nil
}

// dispatchChannelEvent menerapkan satu peristiwa. Mengembalikan status akhir
// ('done'/'failed'/'dead') + pesan error. Tidak mengembalikan error ke pemanggil
// agar status TETAP tersimpan meski penerapannya gagal.
func dispatchChannelEvent(ctx context.Context, ev models.ChannelEvent) (status, lastErr string) {
	next := ev.Attempts + 1

	uid, err := repositories.AnyTenantUserID(ctx, nil, ev.TenantID)
	if err != nil {
		return classifyEventError(err, next)
	}
	// Konteks ber-tenant untuk seluruh penerapan (RLS lapis 2 + created_by).
	ectx := reqctx.WithUserID(reqctx.WithTenantID(ctx, ev.TenantID), uid)

	ch, err := repositories.FindChannel(ectx, nil, ev.ChannelID)
	if err != nil {
		return classifyEventError(err, next)
	}
	norm, err := adapterFor(ch.Provider).Normalize(ev.Payload)
	if err != nil {
		return "dead", "normalisasi payload: " + err.Error() // rusak → permanen
	}
	if strings.TrimSpace(norm.ExternalOrderID) == "" {
		return "dead", "external_order_id kosong di payload"
	}

	switch normalizeEventType(norm.EventType) {
	case "order.created":
		err = applyOrderCreated(ectx, ch, norm)
	case "order.status":
		err = applyOrderStatus(ectx, ch, norm)
	case "order.canceled":
		err = applyOrderCanceled(ectx, ch, norm)
	default:
		return "dead", "tipe peristiwa tak dikenal: " + norm.EventType
	}
	return classifyEventError(err, next)
}

// classifyEventError memetakan error penerapan ke status antrean.
func classifyEventError(err error, nextAttempts int) (string, string) {
	if err == nil {
		return "done", ""
	}
	if errors.Is(err, helpers.ErrValidation) {
		return "dead", err.Error() // input tak membaik dengan diulang
	}
	if nextAttempts >= repositories.MaxEventAttempts {
		return "dead", err.Error()
	}
	return "failed", err.Error()
}

// applyOrderCreated mencatat pesanan kanal sebagai penjualan, menyimpan rincian
// biaya, lalu mengantre sinkron stok tiap produk.
func applyOrderCreated(ctx context.Context, ch models.Channel, norm NormalizedEvent) error {
	in := ChannelOrderInput{
		ChannelID:       ch.ID,
		ExternalOrderID: norm.ExternalOrderID,
		BuyerName:       norm.BuyerName,
		BuyerPhone:      norm.BuyerPhone,
		ShippingAddress: norm.ShippingAddress,
		Courier:         norm.Courier,
		OccurredAt:      norm.OccurredAt,
	}
	for _, it := range norm.Items {
		pid, vid := it.ProductID, it.VariantID
		if pid == "" && it.SKU != "" {
			p, v, err := resolveChannelSKU(ctx, ch.ID, it.SKU)
			if err != nil {
				return err
			}
			pid = p
			if vid == "" {
				vid = v
			}
		}
		if pid == "" {
			return fmt.Errorf("%w: item tanpa product_id maupun sku", helpers.ErrValidation)
		}
		in.Items = append(in.Items, ChannelOrderItemInput{
			ProductID: pid, VariantID: vid, Qty: it.Qty, UnitPrice: it.UnitPrice,
		})
	}

	var feeSum int64
	for _, f := range norm.Fees {
		feeSum += f.Amount
	}
	if len(norm.Fees) > 0 {
		in.FeeAmount = &feeSum
	}

	resp, created, err := RecordChannelOrder(ctx, in)
	if err != nil {
		return err
	}
	if !created {
		return nil // sudah tercatat pada pemrosesan sebelumnya — idempoten
	}

	// Rincian biaya + antrean sinkron stok dalam satu transaksi ber-tenant.
	return repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		if len(norm.Fees) > 0 {
			fees := make([]models.ChannelFee, 0, len(norm.Fees))
			for _, f := range norm.Fees {
				fees = append(fees, models.ChannelFee{
					SaleID: resp.SaleID, Kind: normalizeFeeKind(f.Kind), Amount: f.Amount, Note: f.Note,
				})
			}
			if err := repositories.ReplaceChannelFees(ctx, tx, resp.SaleID, fees); err != nil {
				return err
			}
		}
		seen := map[string]bool{}
		for _, it := range in.Items {
			if seen[it.ProductID] {
				continue
			}
			seen[it.ProductID] = true
			qty, err := repositories.CurrentStockQty(ctx, tx, ch.OutletID, it.ProductID, "")
			if err != nil {
				return err
			}
			if err := repositories.QueueStockSync(ctx, tx, ch.ID, it.ProductID, qty.String()); err != nil {
				return err
			}
		}
		return nil
	})
}

// applyOrderStatus memutakhirkan status pesanan kanal. Bila pesanannya belum
// tercatat (peristiwa create menyusul), kembalikan ErrNotFound → dicoba lagi.
func applyOrderStatus(ctx context.Context, ch models.Channel, norm NormalizedEvent) error {
	if strings.TrimSpace(norm.ExternalStatus) == "" {
		return fmt.Errorf("%w: external_status kosong", helpers.ErrValidation)
	}
	co, ok, err := repositories.FindChannelOrderByExternal(ctx, nil, ch.ID, norm.ExternalOrderID)
	if err != nil {
		return err
	}
	if !ok {
		if norm.IgnoreIfMissing {
			return nil
		}
		return fmt.Errorf("%w: pesanan %s belum tercatat", helpers.ErrNotFound, norm.ExternalOrderID)
	}
	_, err = UpdateChannelOrderStatus(ctx, co.ID, structs.ChannelOrderStatusRequest{
		ExternalStatus: norm.ExternalStatus, Courier: norm.Courier,
	})
	return err
}

// applyOrderCanceled membatalkan pesanan kanal (void penjualan). Idempoten:
// pesanan yang sudah 'canceled' dianggap selesai.
func applyOrderCanceled(ctx context.Context, ch models.Channel, norm NormalizedEvent) error {
	co, ok, err := repositories.FindChannelOrderByExternal(ctx, nil, ch.ID, norm.ExternalOrderID)
	if err != nil {
		return err
	}
	if !ok {
		if norm.IgnoreIfMissing {
			return nil // mis. ditolak/kedaluwarsa sebelum diterima — tak ada penjualan untuk dibatalkan
		}
		return fmt.Errorf("%w: pesanan %s belum tercatat", helpers.ErrNotFound, norm.ExternalOrderID)
	}
	reason := strings.TrimSpace(norm.Reason)
	if reason == "" {
		reason = "dibatalkan oleh kanal"
	}
	if _, err := CancelChannelOrder(ctx, co.ID, reason); err != nil {
		if errors.Is(err, helpers.ErrConflict) {
			return nil // sudah dibatalkan — idempoten
		}
		return err
	}
	return nil
}

// ── Rekonsiliasi pencairan (settlement) ─────────────────────────────────

// RecomputeSettlement menghitung ulang nilai kotor & potongan sebuah periode
// dari penjualan 'completed' + channel_fees, lalu menyimpannya (upsert).
func RecomputeSettlement(ctx context.Context, channelID string, in structs.SettlementRecomputeRequest) (structs.ChannelSettlementResponse, error) {
	var out structs.ChannelSettlementResponse
	ps, pe, err := parsePeriod(in.PeriodStart, in.PeriodEnd)
	if err != nil {
		return out, err
	}
	err = repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		if _, e := repositories.FindChannel(ctx, tx, channelID); e != nil {
			return e
		}
		gross, fee, e := repositories.SettlementTotalsForChannel(ctx, tx, channelID, in.PeriodStart, in.PeriodEnd)
		if e != nil {
			return e
		}
		s := models.ChannelSettlement{
			ChannelID: channelID, PeriodStart: ps, PeriodEnd: pe,
			GrossAmount: gross, FeeAmount: fee, NetAmount: gross - fee,
			Status: "open", Note: strings.TrimSpace(in.Note),
		}
		// Pertahankan uang yang sudah tercatat bila settlement periode ini ada.
		if existing, ok, e := repositories.FindSettlement(ctx, tx, channelID, in.PeriodStart); e != nil {
			return e
		} else if ok {
			s.ReceivedAmount = existing.ReceivedAmount
			s.ReceivedAt = existing.ReceivedAt
			if s.Note == "" {
				s.Note = existing.Note
			}
			s.Status = settlementStatus(s.NetAmount, existing.ReceivedAmount)
		}
		if e := repositories.UpsertSettlement(ctx, tx, &s); e != nil {
			return e
		}
		out = settlementToResponse(s)
		return nil
	})
	return out, err
}

// RecordSettlementReceipt mencatat uang yang benar-benar masuk untuk sebuah
// periode; status jadi 'matched' bila = net, 'mismatch' bila selisih.
func RecordSettlementReceipt(ctx context.Context, channelID string, in structs.SettlementReceiptRequest) (structs.ChannelSettlementResponse, error) {
	var out structs.ChannelSettlementResponse
	if _, err := time.Parse("2006-01-02", in.PeriodStart); err != nil {
		return out, fmt.Errorf("%w: period_start harus YYYY-MM-DD", helpers.ErrValidation)
	}
	recAt := time.Now().UTC()
	if s := strings.TrimSpace(in.ReceivedAt); s != "" {
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			return out, fmt.Errorf("%w: received_at harus RFC3339", helpers.ErrValidation)
		}
		recAt = t
	}
	err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		existing, ok, e := repositories.FindSettlement(ctx, tx, channelID, in.PeriodStart)
		if e != nil {
			return e
		}
		if !ok {
			return repositories.ErrChannelSettlementNotFound
		}
		// Segarkan totals agar akurat pada saat pencatatan.
		gross, fee, e := repositories.SettlementTotalsForChannel(ctx, tx, channelID,
			existing.PeriodStart.Format("2006-01-02"), existing.PeriodEnd.Format("2006-01-02"))
		if e != nil {
			return e
		}
		recv := in.ReceivedAmount
		s := existing
		s.GrossAmount = gross
		s.FeeAmount = fee
		s.NetAmount = gross - fee
		s.ReceivedAmount = &recv
		s.ReceivedAt = &recAt
		if n := strings.TrimSpace(in.Note); n != "" {
			s.Note = n
		}
		s.Status = settlementStatus(s.NetAmount, &recv)
		if e := repositories.UpsertSettlement(ctx, tx, &s); e != nil {
			return e
		}
		out = settlementToResponse(s)
		return nil
	})
	return out, err
}

// ListChannelSettlements mengembalikan settlement sebuah kanal (terbaru dulu).
func ListChannelSettlements(ctx context.Context, channelID string) ([]structs.ChannelSettlementResponse, error) {
	rows, err := repositories.ListSettlements(ctx, channelID)
	if err != nil {
		return nil, err
	}
	out := make([]structs.ChannelSettlementResponse, len(rows))
	for i := range rows {
		out[i] = settlementToResponse(rows[i])
	}
	return out, nil
}

// ── Daftar (inbox & antrean) ────────────────────────────────────────────

// ListChannelEvents mengembalikan satu halaman peristiwa kanal milik tenant.
func ListChannelEvents(ctx context.Context, channelID, status string, limit, offset int) ([]structs.ChannelEventResponse, int64, error) {
	rows, total, err := repositories.ListChannelEvents(ctx, channelID, status, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	out := make([]structs.ChannelEventResponse, len(rows))
	for i := range rows {
		out[i] = channelEventToResponse(rows[i])
	}
	return out, total, nil
}

// ListChannelStockSyncs mengembalikan antrean sinkron stok sebuah kanal, lengkap
// dengan umur keterlambatan untuk indikator "stok kanal X terlambat N menit".
func ListChannelStockSyncs(ctx context.Context, channelID, status string) ([]structs.ChannelStockSyncResponse, error) {
	rows, err := repositories.ListStockSyncs(ctx, channelID, status)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	out := make([]structs.ChannelStockSyncResponse, len(rows))
	for i, r := range rows {
		resp := structs.ChannelStockSyncResponse{
			ID: r.ID, ChannelID: r.ChannelID, ProductID: r.ProductID,
			RequestedQty: r.RequestedQty.String(), Status: r.Status,
			Attempts: r.Attempts, LastError: r.LastError,
			QueuedAt: r.QueuedAt.UTC().Format(saleTimeLayout),
		}
		if r.SentAt != nil {
			resp.SentAt = r.SentAt.UTC().Format(saleTimeLayout)
		} else {
			resp.LateMinutes = int64(now.Sub(r.QueuedAt).Minutes())
		}
		out[i] = resp
	}
	return out, nil
}

// ── Helper ──────────────────────────────────────────────────────────────

// parsePeriod mengurai sepasang tanggal YYYY-MM-DD dan memastikan urutannya.
func parsePeriod(from, to string) (time.Time, time.Time, error) {
	ps, err := time.Parse("2006-01-02", from)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("%w: period_start harus YYYY-MM-DD", helpers.ErrValidation)
	}
	pe, err := time.Parse("2006-01-02", to)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("%w: period_end harus YYYY-MM-DD", helpers.ErrValidation)
	}
	if pe.Before(ps) {
		return time.Time{}, time.Time{}, fmt.Errorf("%w: period_end sebelum period_start", helpers.ErrValidation)
	}
	return ps, pe, nil
}

// settlementStatus menilai kecocokan uang masuk terhadap net.
func settlementStatus(net int64, received *int64) string {
	if received == nil {
		return "open"
	}
	if *received == net {
		return "matched"
	}
	return "mismatch"
}

func settlementToResponse(s models.ChannelSettlement) structs.ChannelSettlementResponse {
	r := structs.ChannelSettlementResponse{
		ID: s.ID, ChannelID: s.ChannelID,
		PeriodStart: s.PeriodStart.Format("2006-01-02"),
		PeriodEnd:   s.PeriodEnd.Format("2006-01-02"),
		GrossAmount: s.GrossAmount, FeeAmount: s.FeeAmount, NetAmount: s.NetAmount,
		ReceivedAmount: s.ReceivedAmount, Status: s.Status, Note: s.Note,
	}
	if s.ReceivedAmount != nil {
		r.Difference = *s.ReceivedAmount - s.NetAmount
	}
	if s.ReceivedAt != nil {
		r.ReceivedAt = s.ReceivedAt.UTC().Format(saleTimeLayout)
	}
	return r
}

func channelEventToResponse(e models.ChannelEvent) structs.ChannelEventResponse {
	r := structs.ChannelEventResponse{
		ID: e.ID, ChannelID: e.ChannelID, EventType: e.EventType, ExternalRef: e.ExternalRef,
		Status: e.Status, Attempts: e.Attempts, LastError: e.LastError,
		ReceivedAt: e.ReceivedAt.UTC().Format(saleTimeLayout),
	}
	if e.ProcessedAt != nil {
		r.ProcessedAt = e.ProcessedAt.UTC().Format(saleTimeLayout)
	}
	return r
}
