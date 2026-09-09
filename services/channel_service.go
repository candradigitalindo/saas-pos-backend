package services

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"candra/backend-api/helpers"
	"candra/backend-api/internal/reqctx"
	"candra/backend-api/internal/timez"
	"candra/backend-api/models"
	"candra/backend-api/repositories"
	"candra/backend-api/structs"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// Layanan kanal pesanan online — fondasi (Fase 11a, §5.10, blueprint Bagian F).
//
// 11a TIDAK menyentuh API kanal. Pesanan masuk lewat ENTRI MANUAL atau IMPOR
// CSV; masing-masing dicatat sebagai SATU `sales` bertanda `channel_id` +
// `external_order_id`, dengan komisi kanal masuk ke `sale_payments.fee_amount`
// sehingga `daily_sales_summaries` + `/reports/profit` menampilkan laba bersih
// per kanal SETELAH komisi — tanpa infrastruktur baru.
//
// `UNIQUE (tenant, channel, external_order_id)` + cek eksplisit membuat pesanan
// yang dientri dua kali tetap satu penjualan.

// ── Channel CRUD ──────────────────────────────────────────────────────────

// ListChannels mengembalikan kanal tenant.
func ListChannels(ctx context.Context) ([]structs.ChannelResponse, error) {
	rows, err := repositories.ListChannels(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]structs.ChannelResponse, len(rows))
	for i := range rows {
		out[i] = channelToResponse(rows[i])
	}
	return out, nil
}

// CreateChannel mendaftarkan kanal baru untuk sebuah outlet.
func CreateChannel(ctx context.Context, in structs.ChannelCreateRequest) (structs.ChannelResponse, error) {
	var out structs.ChannelResponse
	rate, err := parseRateOrZero(in.CommissionRate)
	if err != nil {
		return out, err
	}
	err = repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		if err := repositories.FindOutletByID(ctx, tx, in.OutletID, &models.Outlet{}); err != nil {
			return fmt.Errorf("%w: outlet tidak ditemukan", helpers.ErrValidation)
		}
		ch := models.Channel{
			OutletID: in.OutletID, Kind: in.Kind, Provider: in.Provider, Name: in.Name,
			MerchantRef: in.MerchantRef, CommissionRate: rate,
			IntegrationMode: orDefault(in.IntegrationMode, "manual"), IsActive: true,
		}
		if in.PriceListID != "" {
			ch.PriceListID = &in.PriceListID
		}
		if err := repositories.CreateChannel(ctx, tx, &ch); err != nil {
			if helpers.IsDuplicateEntryError(err) {
				return fmt.Errorf("%w: kanal untuk provider itu sudah ada di outlet tsb", helpers.ErrConflict)
			}
			return err
		}
		out = channelToResponse(ch)
		return nil
	})
	return out, err
}

// UpdateChannel mengubah kanal (nama, komisi, mode, aktif/nonaktif).
func UpdateChannel(ctx context.Context, id string, in structs.ChannelUpdateRequest) (structs.ChannelResponse, error) {
	var out structs.ChannelResponse
	err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		ch, err := repositories.FindChannel(ctx, tx, id)
		if err != nil {
			return err
		}
		if in.Name != nil {
			ch.Name = *in.Name
		}
		if in.MerchantRef != nil {
			ch.MerchantRef = *in.MerchantRef
		}
		if in.CommissionRate != nil {
			r, perr := parseRateOrZero(*in.CommissionRate)
			if perr != nil {
				return perr
			}
			ch.CommissionRate = r
		}
		if in.PriceListID != nil {
			ch.PriceListID = nilIfBlank(*in.PriceListID)
		}
		if in.IntegrationMode != nil {
			ch.IntegrationMode = *in.IntegrationMode
		}
		if in.IsActive != nil {
			ch.IsActive = *in.IsActive
		}
		if err := repositories.SaveChannel(ctx, tx, &ch); err != nil {
			return err
		}
		reloaded, err := repositories.FindChannel(ctx, tx, id)
		if err != nil {
			return err
		}
		out = channelToResponse(reloaded)
		return nil
	})
	return out, err
}

// DeactivateChannel menonaktifkan kanal (mematikan kanal tak mengganggu kasir).
func DeactivateChannel(ctx context.Context, id string) error {
	return repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		return repositories.DeactivateChannel(ctx, tx, id)
	})
}

// ── Channel product (pemetaan SKU) ────────────────────────────────────────

// UpsertChannelProduct membuat/memperbarui pemetaan SKU kanal (unik per
// channel+external_sku).
func UpsertChannelProduct(ctx context.Context, channelID string, in structs.ChannelProductRequest) (structs.ChannelProductResponse, error) {
	var out structs.ChannelProductResponse
	buffer, err := parseDecOrZero(in.StockBuffer)
	if err != nil {
		return out, err
	}
	err = repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		if _, err := repositories.FindChannel(ctx, tx, channelID); err != nil {
			return err
		}
		if err := repositories.FindProductInTenant(ctx, tx, in.ProductID, &models.Product{}); err != nil {
			return fmt.Errorf("%w: produk tidak ditemukan", helpers.ErrValidation)
		}

		existing, found, ferr := repositories.FindChannelProductBySKU(ctx, tx, channelID, in.ExternalSKU)
		if ferr != nil {
			return ferr
		}
		cp := models.ChannelProduct{
			ChannelID: channelID, ProductID: in.ProductID, ExternalSKU: in.ExternalSKU,
			ChannelPrice: in.ChannelPrice, IsAvailable: true, StockBuffer: buffer,
		}
		if in.VariantID != "" {
			cp.VariantID = &in.VariantID
		}
		if in.IsAvailable != nil {
			cp.IsAvailable = *in.IsAvailable
		}
		if found {
			cp.ID = existing.ID
			if err := repositories.SaveChannelProduct(ctx, tx, &cp); err != nil {
				return err
			}
		} else if err := repositories.CreateChannelProduct(ctx, tx, &cp); err != nil {
			return err
		}
		out = channelProductToResponse(cp)
		return nil
	})
	return out, err
}

// DeleteChannelProduct menghapus satu pemetaan.
func DeleteChannelProduct(ctx context.Context, id string) error {
	return repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		return repositories.DeleteChannelProduct(ctx, tx, id)
	})
}

// ── Impor CSV laporan harian kanal ────────────────────────────────────────

// ChannelOrderImportResult merangkum hasil impor CSV.
type ChannelOrderImportResult struct {
	Imported int      `json:"imported"`
	Skipped  int      `json:"skipped"` // sudah ada (idempoten)
	Failed   int      `json:"failed"`
	Errors   []string `json:"errors,omitempty"`
}

// csvOrderRow satu baris CSV: header
// external_order_id,date,product_id,qty,unit_price,fee_amount
type csvOrderRow struct {
	ExternalOrderID string
	Date            string
	ProductID       string
	Qty             string
	UnitPrice       string
	FeeAmount       string
}

// ImportChannelOrdersCSV mengimpor laporan harian kanal: satu baris = satu baris
// pesanan. Baris dengan `external_order_id` yang sama digabung jadi satu pesanan.
// Idempoten: pesanan yang sudah tercatat dilewati.
func ImportChannelOrdersCSV(ctx context.Context, channelID string, raw []byte) (ChannelOrderImportResult, error) {
	var res ChannelOrderImportResult

	r := csv.NewReader(bytes.NewReader(raw))
	r.TrimLeadingSpace = true
	r.FieldsPerRecord = -1

	head, err := r.Read()
	if err != nil {
		return res, fmt.Errorf("%w: CSV kosong atau rusak", helpers.ErrValidation)
	}
	idx := map[string]int{}
	for i, h := range head {
		idx[strings.ToLower(strings.TrimSpace(h))] = i
	}
	for _, need := range []string{"external_order_id", "date", "product_id", "qty", "unit_price"} {
		if _, ok := idx[need]; !ok {
			return res, fmt.Errorf("%w: kolom %q wajib ada di header CSV", helpers.ErrValidation, need)
		}
	}
	get := func(rec []string, key string) string {
		i, ok := idx[key]
		if !ok || i >= len(rec) {
			return ""
		}
		return strings.TrimSpace(rec[i])
	}

	// Kumpulkan baris per pesanan, jaga urutan kemunculan.
	order := []string{}
	byOrder := map[string][]csvOrderRow{}
	line := 1
	for {
		rec, rerr := r.Read()
		if rerr == io.EOF {
			break
		}
		line++
		if rerr != nil {
			res.Failed++
			res.Errors = append(res.Errors, fmt.Sprintf("baris %d: %v", line, rerr))
			continue
		}
		row := csvOrderRow{
			ExternalOrderID: get(rec, "external_order_id"),
			Date:            get(rec, "date"),
			ProductID:       get(rec, "product_id"),
			Qty:             get(rec, "qty"),
			UnitPrice:       get(rec, "unit_price"),
			FeeAmount:       get(rec, "fee_amount"),
		}
		if row.ExternalOrderID == "" || row.ProductID == "" {
			res.Failed++
			res.Errors = append(res.Errors, fmt.Sprintf("baris %d: external_order_id & product_id wajib", line))
			continue
		}
		if _, seen := byOrder[row.ExternalOrderID]; !seen {
			order = append(order, row.ExternalOrderID)
		}
		byOrder[row.ExternalOrderID] = append(byOrder[row.ExternalOrderID], row)
	}

	for _, extID := range order {
		rows := byOrder[extID]
		in := ChannelOrderInput{ChannelID: channelID, ExternalOrderID: extID}

		var feeSum int64
		var feeSet bool
		var bad string
		for _, row := range rows {
			qty, qerr := decimal.NewFromString(row.Qty)
			if qerr != nil || qty.LessThanOrEqual(decimal.Zero) {
				bad = "qty tidak valid"
				break
			}
			up, uerr := strconv.ParseInt(row.UnitPrice, 10, 64)
			if uerr != nil || up < 0 {
				bad = "unit_price tidak valid"
				break
			}
			upCopy := up
			in.Items = append(in.Items, ChannelOrderItemInput{ProductID: row.ProductID, Qty: qty, UnitPrice: &upCopy})
			if row.FeeAmount != "" {
				f, ferr := strconv.ParseInt(row.FeeAmount, 10, 64)
				if ferr != nil || f < 0 {
					bad = "fee_amount tidak valid"
					break
				}
				feeSum += f
				feeSet = true
			}
			if row.Date != "" {
				if d, derr := time.Parse("2006-01-02", row.Date); derr == nil {
					dd := d
					in.OccurredAt = &dd
				}
			}
		}
		if bad != "" {
			res.Failed++
			res.Errors = append(res.Errors, fmt.Sprintf("pesanan %s: %s", extID, bad))
			continue
		}
		if feeSet {
			in.FeeAmount = &feeSum
		}

		_, created, oerr := RecordChannelOrder(ctx, in)
		if oerr != nil {
			res.Failed++
			res.Errors = append(res.Errors, fmt.Sprintf("pesanan %s: %s", extID, oerr.Error()))
			continue
		}
		if created {
			res.Imported++
		} else {
			res.Skipped++ // sudah tercatat pada impor sebelumnya
		}
	}
	return res, nil
}

// ── Channel order ─────────────────────────────────────────────────────────

// ChannelOrderInput adalah masukan pencatatan satu pesanan kanal (entri manual
// atau baris CSV).
type ChannelOrderInput struct {
	ChannelID       string
	ExternalOrderID string
	BuyerName       string
	BuyerPhone      string
	ShippingAddress string
	Courier         string
	OrderDiscount   int64
	FeeAmount       *int64
	OccurredAt      *time.Time
	Items           []ChannelOrderItemInput
}

// ChannelOrderItemInput satu baris pesanan kanal.
type ChannelOrderItemInput struct {
	ProductID string
	VariantID string
	Qty       decimal.Decimal
	UnitPrice *int64
}

// RecordChannelOrder mencatat satu pesanan kanal sebagai penjualan 'completed'
// bertanda kanal: item ber-snapshot harga, stok terpotong (+ resep), satu
// pembayaran ber-`fee_amount` = komisi kanal, lalu `daily_sales_summaries`
// dimutakhirkan. Idempoten lewat (channel, external_order_id).
//
// Mengembalikan (channelOrder, created, error). created=false berarti pesanan
// itu sudah tercatat sebelumnya (idempoten).
func RecordChannelOrder(ctx context.Context, in ChannelOrderInput) (structs.ChannelOrderResponse, bool, error) {
	var out structs.ChannelOrderResponse
	created := false
	if len(in.Items) == 0 {
		return out, false, fmt.Errorf("%w: pesanan tanpa item", helpers.ErrValidation)
	}
	for _, it := range in.Items {
		if it.Qty.LessThanOrEqual(decimal.Zero) {
			return out, false, fmt.Errorf("%w: qty harus > 0", helpers.ErrValidation)
		}
	}

	err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		ch, err := repositories.FindChannel(ctx, tx, in.ChannelID)
		if err != nil {
			return err
		}

		// Idempotensi: pesanan yang sama → kembalikan yang sudah ada.
		if existing, ok, ferr := repositories.FindChannelOrderByExternal(ctx, tx, ch.ID, in.ExternalOrderID); ferr != nil {
			return ferr
		} else if ok {
			out = channelOrderToResponse(existing, existingSaleAmounts(ctx, tx, existing.SaleID))
			return nil
		}
		created = true

		var outlet models.Outlet
		if err := repositories.FindOutletByID(ctx, tx, ch.OutletID, &outlet); err != nil {
			return err
		}
		occurred := time.Now().UTC()
		if in.OccurredAt != nil {
			occurred = in.OccurredAt.UTC()
		}
		bizDate, err := timez.BusinessDate(occurred, outlet.Timezone, outlet.DayStartOffset())
		if err != nil {
			return err
		}

		productIDs := channelProductIDs(in.Items)
		products, err := repositories.ProductsByIDs(ctx, tx, productIDs)
		if err != nil {
			return err
		}
		variants, err := repositories.ProductVariantsByIDs(ctx, tx, channelVariantIDs(in.Items))
		if err != nil {
			return err
		}
		recipes, err := repositories.RecipesByProductIDs(ctx, tx, productIDs)
		if err != nil {
			return err
		}
		channelPrices, err := repositories.ChannelProductPriceMap(ctx, tx, ch.ID)
		if err != nil {
			return err
		}

		var subtotal, costTotal int64
		saleItems := make([]models.SaleItem, 0, len(in.Items))
		deltas := make([]repositories.StockDelta, 0, len(in.Items))

		for _, it := range in.Items {
			p, ok := products[it.ProductID]
			if !ok {
				return fmt.Errorf("%w: produk %s tidak ditemukan", helpers.ErrValidation, it.ProductID)
			}
			unitPrice := p.SellPrice
			if v, cp := channelPrices[it.ProductID]; cp {
				unitPrice = v
			}
			if it.VariantID != "" {
				vr, vok := variants[it.VariantID]
				if !vok || vr.ProductID != it.ProductID {
					return fmt.Errorf("%w: varian tidak cocok", helpers.ErrValidation)
				}
				unitPrice += vr.PriceDelta
			}
			if it.UnitPrice != nil {
				unitPrice = *it.UnitPrice
			}

			lineGross := helpers.LineAmount(it.Qty, unitPrice)
			lineCost := helpers.LineAmount(it.Qty, p.CostPrice)
			subtotal += lineGross
			costTotal += lineCost

			unitName := ""
			if p.Unit != nil {
				unitName = p.Unit.Name
			}
			si := models.SaleItem{
				ProductID: it.ProductID, ProductName: p.Name, UnitName: unitName,
				Qty: it.Qty, UnitPrice: unitPrice, UnitCost: p.CostPrice, LineTotal: lineGross,
			}
			if it.VariantID != "" {
				vid := it.VariantID
				si.VariantID = &vid
			}
			saleItems = append(saleItems, si)

			if p.TrackStock {
				deltas = append(deltas, repositories.StockDelta{
					ProductID: it.ProductID, VariantID: it.VariantID,
					Delta: it.Qty.Neg(), UnitCost: p.CostPrice, Kind: "sale",
				})
			}
			if rec, has := recipes[it.ProductID]; has && len(rec.Items) > 0 {
				yield := rec.Recipe.YieldQty
				if yield.LessThanOrEqual(decimal.Zero) {
					yield = decimal.NewFromInt(1)
				}
				factor := it.Qty.Div(yield)
				for _, ing := range rec.Items {
					deltas = append(deltas, repositories.StockDelta{
						ProductID: ing.IngredientProductID,
						Delta:     ing.Qty.Mul(factor).Neg(), Kind: "recipe",
					})
				}
			}
		}

		if in.OrderDiscount > subtotal {
			return fmt.Errorf("%w: diskon melebihi subtotal", helpers.ErrValidation)
		}
		total := subtotal - in.OrderDiscount

		fee := int64(0)
		if in.FeeAmount != nil {
			fee = *in.FeeAmount
		} else {
			fee = helpers.RoundHalfUpToInt(decimal.NewFromInt(total).Mul(ch.CommissionRate))
		}

		seq, err := repositories.NextReceiptSeq(ctx, tx, ch.OutletID, bizDate)
		if err != nil {
			return err
		}
		extID := in.ExternalOrderID
		sale := models.Sale{
			OutletID:        ch.OutletID,
			ChannelID:       &ch.ID,
			ExternalOrderID: &extID,
			ReceiptNo:       formatReceiptNo(outlet, bizDate, seq),
			IdempotencyKey:  "channel:" + ch.ID + ":" + in.ExternalOrderID,
			OrderType:       "delivery",
			Status:          "completed",
			Subtotal:        subtotal,
			DiscountAmount:  in.OrderDiscount,
			Total:           total,
			PaidAmount:      total,
			CostTotal:       costTotal,
			Note:            "Pesanan " + ch.Provider + " " + in.ExternalOrderID,
			OccurredAt:      occurred,
			BusinessDate:    bizDate,
			CreatedBy:       reqctx.UserID(ctx),
			Items:           saleItems,
			Payments: []models.SalePayment{
				{Method: "transfer", Amount: total, FeeAmount: fee, PaidAt: occurred},
			},
		}
		if err := repositories.CreateSale(ctx, tx, &sale); err != nil {
			if helpers.IsDuplicateEntryError(err) {
				return fmt.Errorf("%w: pesanan kanal ini sudah tercatat", helpers.ErrConflict)
			}
			return err
		}

		if len(deltas) > 0 {
			if _, err := repositories.ApplyStockDeltas(ctx, tx, ch.OutletID, deltas, repositories.MovementMeta{
				Kind: "sale", RefTable: "sales", RefID: sale.ID, OccurredAt: occurred, BusinessDate: bizDate,
			}); err != nil {
				return err
			}
		}
		if err := repositories.ApplySaleToSummary(ctx, tx, &sale); err != nil {
			return err
		}

		co := models.ChannelOrder{
			ChannelID: ch.ID, SaleID: sale.ID, ExternalOrderID: in.ExternalOrderID,
			ExternalStatus: "completed",
			BuyerName:      in.BuyerName, BuyerPhone: in.BuyerPhone,
			ShippingAddress: in.ShippingAddress, Courier: in.Courier,
		}
		if err := repositories.CreateChannelOrder(ctx, tx, &co); err != nil {
			return err
		}
		out = channelOrderToResponse(co, channelSaleAmounts{gross: subtotal, fee: fee, net: total - fee})
		return nil
	})
	return out, created, err
}

// UpdateChannelOrderStatus memutakhirkan status & jejak logistik pesanan kanal.
func UpdateChannelOrderStatus(ctx context.Context, id string, in structs.ChannelOrderStatusRequest) (structs.ChannelOrderResponse, error) {
	var out structs.ChannelOrderResponse
	err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		co, err := repositories.FindChannelOrder(ctx, tx, id)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		co.ExternalStatus = in.ExternalStatus
		if in.Courier != "" {
			co.Courier = in.Courier
		}
		if in.TrackingNo != "" {
			co.TrackingNo = in.TrackingNo
		}
		if in.DriverName != "" {
			co.DriverName = in.DriverName
		}
		switch in.ExternalStatus {
		case "accepted", "preparing":
			if co.AcceptedAt == nil {
				co.AcceptedAt = &now
			}
		case "ready", "shipped":
			if co.ReadyAt == nil {
				co.ReadyAt = &now
			}
		case "completed", "delivered":
			if co.CompletedAt == nil {
				co.CompletedAt = &now
			}
		}
		if err := repositories.SaveChannelOrderStatus(ctx, tx, &co); err != nil {
			return err
		}
		out = channelOrderToResponse(co, existingSaleAmounts(ctx, tx, co.SaleID))
		return nil
	})
	return out, err
}

// CancelChannelOrder membatalkan pesanan kanal: penjualannya di-void (stok
// dikembalikan, laporan disesuaikan) dan status kanal ditandai 'canceled'.
func CancelChannelOrder(ctx context.Context, id, reason string) (structs.ChannelOrderResponse, error) {
	var out structs.ChannelOrderResponse
	err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		co, err := repositories.FindChannelOrder(ctx, tx, id)
		if err != nil {
			return err
		}
		if co.ExternalStatus == "canceled" {
			return fmt.Errorf("%w: pesanan sudah dibatalkan", helpers.ErrConflict)
		}
		if err := voidSaleInTx(ctx, tx, co.SaleID, reason); err != nil {
			return err
		}
		co.ExternalStatus = "canceled"
		if err := repositories.SaveChannelOrderStatus(ctx, tx, &co); err != nil {
			return err
		}
		out = channelOrderToResponse(co, existingSaleAmounts(ctx, tx, co.SaleID))
		return nil
	})
	return out, err
}

// ── Helper ────────────────────────────────────────────────────────────────

type channelSaleAmounts struct{ gross, fee, net int64 }

// existingSaleAmounts membaca ringkasan uang dari sebuah penjualan (untuk DTO
// pesanan kanal yang sudah ada / berubah status).
func existingSaleAmounts(ctx context.Context, tx *gorm.DB, saleID string) channelSaleAmounts {
	var s models.Sale
	if err := repositories.FindSaleInTenant(ctx, tx, saleID, &s); err != nil {
		return channelSaleAmounts{}
	}
	var fee int64
	for _, p := range s.Payments {
		fee += p.FeeAmount
	}
	return channelSaleAmounts{gross: s.Subtotal, fee: fee, net: s.Total - fee}
}

func channelProductIDs(items []ChannelOrderItemInput) []string {
	set := map[string]struct{}{}
	for _, it := range items {
		set[it.ProductID] = struct{}{}
	}
	ids := make([]string, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func channelVariantIDs(items []ChannelOrderItemInput) []string {
	var ids []string
	for _, it := range items {
		if it.VariantID != "" {
			ids = append(ids, it.VariantID)
		}
	}
	return ids
}

func parseRateOrZero(s string) (decimal.Decimal, error) {
	if s == "" {
		return decimal.Zero, nil
	}
	d, err := decimal.NewFromString(s)
	if err != nil || d.IsNegative() {
		return decimal.Zero, fmt.Errorf("%w: commission_rate tidak valid", helpers.ErrValidation)
	}
	return d, nil
}

func parseDecOrZero(s string) (decimal.Decimal, error) {
	if s == "" {
		return decimal.Zero, nil
	}
	d, err := decimal.NewFromString(s)
	if err != nil {
		return decimal.Zero, fmt.Errorf("%w: stock_buffer bukan angka", helpers.ErrValidation)
	}
	return d, nil
}

func nilIfBlank(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// ── DTO ───────────────────────────────────────────────────────────────────

func channelToResponse(c models.Channel) structs.ChannelResponse {
	r := structs.ChannelResponse{
		ID: c.ID, OutletID: c.OutletID, Kind: c.Kind, Provider: c.Provider, Name: c.Name,
		MerchantRef: c.MerchantRef, CommissionRate: c.CommissionRate.String(),
		IntegrationMode: c.IntegrationMode, IsActive: c.IsActive,
		CreatedAt: c.CreatedAt.Format(saleTimeLayout),
	}
	if c.PriceListID != nil {
		r.PriceListID = *c.PriceListID
	}
	return r
}

func channelProductToResponse(p models.ChannelProduct) structs.ChannelProductResponse {
	r := structs.ChannelProductResponse{
		ID: p.ID, ChannelID: p.ChannelID, ProductID: p.ProductID,
		ExternalSKU: p.ExternalSKU, ChannelPrice: p.ChannelPrice,
		IsAvailable: p.IsAvailable, StockBuffer: p.StockBuffer.String(),
	}
	if p.VariantID != nil {
		r.VariantID = *p.VariantID
	}
	return r
}

func channelOrderToResponse(o models.ChannelOrder, amt channelSaleAmounts) structs.ChannelOrderResponse {
	return structs.ChannelOrderResponse{
		ID: o.ID, ChannelID: o.ChannelID, SaleID: o.SaleID,
		ExternalOrderID: o.ExternalOrderID, ExternalStatus: o.ExternalStatus,
		BuyerName: o.BuyerName, BuyerPhone: o.BuyerPhone, ShippingAddress: o.ShippingAddress,
		Courier: o.Courier, TrackingNo: o.TrackingNo,
		GrossAmount: amt.gross, FeeAmount: amt.fee, NetAmount: amt.net,
		CreatedAt: o.CreatedAt.Format(saleTimeLayout),
	}
}

// ChannelOrderToResponse — untuk controller list (tanpa ringkasan uang detail).
func ChannelOrderToResponse(o models.ChannelOrder) structs.ChannelOrderResponse {
	return channelOrderToResponse(o, channelSaleAmounts{})
}

// ChannelToResponse / ChannelProductToResponse — untuk controller list.
func ChannelToResponse(c models.Channel) structs.ChannelResponse { return channelToResponse(c) }
func ChannelProductToResponse(p models.ChannelProduct) structs.ChannelProductResponse {
	return channelProductToResponse(p)
}
