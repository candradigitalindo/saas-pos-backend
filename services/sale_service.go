package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"candra/backend-api/config"
	"candra/backend-api/helpers"
	"candra/backend-api/internal/reqctx"
	"candra/backend-api/internal/timez"
	"candra/backend-api/models"
	"candra/backend-api/repositories"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// orDefault mengembalikan v bila tidak kosong, selain itu def.
func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

// idempotencyScope untuk operasi checkout.
const idempotencyScopeSale = "sale.create"

// idempotencyTTL diambil dari IDEMPOTENCY_TTL_DAYS (default 7 hari, §15).
func idempotencyTTL() time.Duration {
	return time.Duration(config.GetIntEnv("IDEMPOTENCY_TTL_DAYS", 7)) * 24 * time.Hour
}

// CheckoutItem adalah satu baris keranjang dari controller.
type CheckoutItem struct {
	ProductID      string
	VariantID      string // "" = tanpa varian
	ProductUnitID  string // "" = satuan dasar; selain itu kemasan (qty dalam kemasan)
	Qty            decimal.Decimal
	DiscountAmount int64
	Note           string
}

// CheckoutPayment adalah satu tender.
type CheckoutPayment struct {
	Method    string
	Amount    int64
	Reference string
}

// CheckoutInput adalah masukan lengkap Checkout, sudah diparse & dinormalkan
// oleh controller.
type CheckoutInput struct {
	SaleID          string // ULID klien opsional
	OutletID        string
	ShiftID         string // "" = pakai shift terbuka outlet
	CustomerID      string
	OrderType       string
	OrderDiscount   int64
	Note            string
	OpenBillID      string // "" = bukan dari tagihan terbuka
	ClientCreatedAt *time.Time
	Items           []CheckoutItem
	Payments        []CheckoutPayment

	IdempotencyKey string // dari header Idempotency-Key (wajib)
	RequestHash    string // sha256 dari body mentah
}

// Checkout menjalankan transaksi kasir (§13.1): satu transaksi database menulis
// sale + item + payment + gerakan stok + (opsional) piutang, idempoten lewat
// Idempotency-Key.
//
// Mengembalikan (status HTTP, body JSON siap kirim, replayed, error). `replayed`
// true berarti Idempotency-Key ini sudah pernah dipakai dan body lama
// dikembalikan tanpa mengerjakan ulang — dipakai /sync/push untuk menandai
// operasi sebagai "duplicate", bukan "applied".
func Checkout(ctx context.Context, in CheckoutInput) (int, []byte, bool, error) {
	if in.IdempotencyKey == "" {
		return 0, nil, false, fmt.Errorf("%w: header Idempotency-Key wajib untuk checkout", helpers.ErrValidation)
	}
	if len(in.Items) == 0 {
		return 0, nil, false, fmt.Errorf("%w: keranjang kosong", helpers.ErrValidation)
	}
	for _, it := range in.Items {
		if it.Qty.LessThanOrEqual(decimal.Zero) {
			return 0, nil, false, fmt.Errorf("%w: qty harus lebih besar dari 0", helpers.ErrValidation)
		}
		if it.DiscountAmount < 0 {
			return 0, nil, false, fmt.Errorf("%w: diskon baris tidak boleh negatif", helpers.ErrValidation)
		}
	}
	if in.OrderDiscount < 0 {
		return 0, nil, false, fmt.Errorf("%w: diskon transaksi tidak boleh negatif", helpers.ErrValidation)
	}
	// Izin sale.discount ditegakkan DI SINI, bukan hanya dengan menyembunyikan
	// kolom diskon di layar: klien bisa dibuat siapa saja, dan /sync/push
	// membawa badan checkout yang sama.
	if hasDiscount(in) && reqctx.UserID(ctx) != "" && !reqctx.HasPermission(ctx, "sale.discount") {
		return 0, nil, false, fmt.Errorf("%w: Anda tidak punya izin memberi diskon", helpers.ErrForbidden)
	}

	// 0. Di luar transaksi: hak akses outlet, outlet (butuh zona & batas hari),
	//    waktu, tanggal usaha.
	if err := ensureOutletAccess(ctx, in.OutletID); err != nil {
		return 0, nil, false, err
	}
	var outlet models.Outlet
	if err := repositories.FindOutletByID(ctx, nil, in.OutletID, &outlet); err != nil {
		return 0, nil, false, fmt.Errorf("%w: outlet tidak ditemukan", helpers.ErrValidation)
	}
	now := time.Now().UTC()
	bizDate, err := timez.BusinessDate(now, outlet.Timezone, outlet.DayStartOffset())
	if err != nil {
		return 0, nil, false, fmt.Errorf("zona waktu outlet tidak valid: %w", err)
	}

	var (
		outStatus int
		outBody   []byte
		replayed  bool
	)
	txErr := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		// 1. Idempotensi.
		m, err := repositories.LookupIdempotency(ctx, tx, idempotencyScopeSale, in.IdempotencyKey, in.RequestHash)
		if err != nil {
			return err
		}
		if m.Found {
			if !m.SameRequest {
				return fmt.Errorf("%w: Idempotency-Key sudah dipakai untuk permintaan berbeda", helpers.ErrConflict)
			}
			outStatus, outBody, replayed = m.ResponseStatus, m.ResponseBody, true
			return nil
		}

		// 1b. Shift.
		shift, err := resolveShift(ctx, tx, in.OutletID, in.ShiftID)
		if err != nil {
			return err
		}

		// 1c. Tagihan terbuka yang dilunasi (bila ada) dikunci SEBELUM menulis
		//     apa pun: dua kasir yang menagih meja yang sama antre di sini, dan
		//     yang kedua ditolak karena tagihannya sudah dibayar.
		var tagihan *models.OpenBill
		if in.OpenBillID != "" {
			if tagihan, err = lockOpenBillForCheckout(ctx, tx, in.OpenBillID, in.OutletID); err != nil {
				return err
			}
		}

		// 2. Muat produk, varian, dan resep (F&B) → SNAPSHOT. Harga dari master,
		//    bukan klien. Penguncian baris stok dilakukan ApplyStockDeltas di
		//    langkah 8 (URUT product_id MENAIK atas gabungan produk + bahan baku).
		productIDs := sortedUniqueProductIDs(in.Items)
		products, err := repositories.ProductsByIDs(ctx, tx, productIDs)
		if err != nil {
			return err
		}
		variants, err := repositories.ProductVariantsByIDs(ctx, tx, variantIDs(in.Items))
		if err != nil {
			return err
		}
		recipes, err := repositories.RecipesByProductIDs(ctx, tx, productIDs)
		if err != nil {
			return err
		}
		kemasan, err := repositories.ProductUnitsByIDs(ctx, tx, productUnitIDs(in.Items))
		if err != nil {
			return err
		}
		grosir, err := repositories.WholesaleTiers(ctx, tx, productIDs)
		if err != nil {
			return err
		}
		khusus, err := customerListTiers(ctx, tx, in.CustomerID, productIDs)
		if err != nil {
			return err
		}

		// 4. Hitung per baris lalu jumlahkan.
		priced, totals, err := priceCheckout(in, outlet, products, variants, kemasan, grosir, khusus)
		if err != nil {
			return err
		}

		// 5. Pembayaran & (opsional) piutang kasbon.
		pay, err := resolvePayments(ctx, tx, in, totals.total)
		if err != nil {
			return err
		}

		// 6. Nomor struk (baris penghitung dikunci).
		seq, err := repositories.NextReceiptSeq(ctx, tx, in.OutletID, bizDate)
		if err != nil {
			return err
		}
		receiptNo := formatReceiptNo(outlet, bizDate, seq)

		// 7. Tulis sale + item + payment.
		sale := buildSale(ctx, in, shift, bizDate, now, receiptNo, priced, totals, pay)
		if err := repositories.CreateSale(ctx, tx, &sale); err != nil {
			return err
		}
		if tagihan != nil {
			if err := closeOpenBillForSale(ctx, tx, tagihan, sale.ID); err != nil {
				return err
			}
		}

		// 8. Gerakan stok: 'sale' untuk produk ber-track_stock + 'recipe' untuk
		//    bahan baku menu (F&B). Satu penguncian, saldo minus diizinkan (§13.1).
		deltas, derr := stockDeltasForSale(priced, recipes, sale.OutletID)
		if derr != nil {
			return derr
		}
		if _, err := repositories.ApplyStockDeltas(ctx, tx, sale.OutletID, deltas, repositories.MovementMeta{
			Kind: "sale", RefTable: "sales", RefID: sale.ID, OccurredAt: now, BusinessDate: bizDate,
		}); err != nil {
			return err
		}

		// 9. Piutang bila ada pembayaran kasbon.
		if pay.creditAmount > 0 {
			rec := models.Receivable{
				CustomerID:  in.CustomerID,
				SourceTable: "sales",
				SourceID:    sale.ID,
				Amount:      pay.creditAmount,
				Status:      "open",
			}
			if err := repositories.CreateReceivable(ctx, tx, &rec); err != nil {
				return err
			}
		}

		// 10. Perbarui agregat laporan harian (UPSERT inkremental, §13.1 langkah 10).
		if err := repositories.ApplySaleToSummary(ctx, tx, &sale); err != nil {
			return err
		}

		// 11. Bentuk response, simpan ke idempotency_keys, kembalikan.
		body, err := marshalSaleResponse(&sale, "Transaksi berhasil")
		if err != nil {
			return err
		}
		if err := repositories.SaveIdempotency(ctx, tx, idempotencyScopeSale, in.IdempotencyKey, in.RequestHash,
			http.StatusCreated, body, idempotencyTTL()); err != nil {
			return err
		}
		outStatus, outBody = http.StatusCreated, body
		return nil
	})
	if txErr != nil {
		return 0, nil, false, txErr
	}
	return outStatus, outBody, replayed, nil
}

// resolveShift memilih shift untuk transaksi: yang diminta (harus 'open' & milik
// outlet), atau shift 'open' outlet bila tidak diminta.
func resolveShift(ctx context.Context, tx *gorm.DB, outletID, shiftID string) (models.Shift, error) {
	var sh models.Shift
	if shiftID != "" {
		if err := repositories.FindShiftInTenant(ctx, tx, shiftID, &sh); err != nil {
			return sh, fmt.Errorf("%w: shift tidak ditemukan", helpers.ErrValidation)
		}
		if sh.OutletID != outletID || sh.Status != "open" {
			return sh, fmt.Errorf("%w: shift tidak terbuka untuk outlet ini", helpers.ErrValidation)
		}
		return sh, nil
	}
	if err := repositories.FindOpenShift(ctx, tx, outletID, &sh); err != nil {
		return sh, fmt.Errorf("%w: belum ada shift terbuka — buka shift dulu", helpers.ErrValidation)
	}
	return sh, nil
}

// pricedItem adalah satu baris yang sudah di-snapshot & dihitung.
type pricedItem struct {
	in          CheckoutItem
	productName string
	unitName    string
	conversion  decimal.Decimal // isi kemasan dalam satuan dasar (1 = tanpa kemasan)
	baseCost    int64           // modal per SATUAN DASAR (untuk gerakan stok)
	unitPrice   int64
	unitCost    int64
	lineGross   int64 // round(qty * unit_price), sebelum diskon/pajak
	lineTax     int64
	lineTotal   int64 // qty*price - diskon (+ pajak bila eksklusif)
	lineCost    int64 // round(qty * unit_cost)
	trackStock  bool
}

type saleTotals struct {
	subtotal, discountAmount, taxAmount, serviceAmount, roundingAmount, total, costTotal int64
	sumLineDiscount                                                                      int64
}

// priceCheckout membuat SNAPSHOT tiap baris dan menghitung total. Harga jual &
// modal diambil dari master produk (+ price_delta varian), TIDAK dari klien.
func priceCheckout(in CheckoutInput, outlet models.Outlet, products map[string]models.Product, variants map[string]models.ProductVariant, kemasan map[string]models.ProductUnit, grosir, khusus map[string][]models.ProductPrice) ([]pricedItem, saleTotals, error) {
	taxRate := outlet.TaxRate
	taxOn := outlet.TaxEnabled && taxRate.GreaterThan(decimal.Zero)

	out := make([]pricedItem, 0, len(in.Items))
	var t saleTotals

	// Harga grosir memakai TOTAL jumlah barang itu di transaksi ini — "Kopi
	// (Besar) ×6 + Kopi (Kecil) ×6" = 12 kopi, sama dengan kasir menghitungnya.
	// Hanya baris SATUAN DASAR yang dihitung: kemasan punya harganya sendiri.
	totalQty := map[string]decimal.Decimal{}
	for _, it := range in.Items {
		if it.ProductUnitID == "" {
			totalQty[it.ProductID] = totalQty[it.ProductID].Add(it.Qty)
		}
	}

	for _, it := range in.Items {
		p, ok := products[it.ProductID]
		if !ok {
			return nil, t, fmt.Errorf("%w: produk %s tidak ditemukan", helpers.ErrValidation, it.ProductID)
		}
		unitName := ""
		if p.Unit != nil {
			unitName = p.Unit.Name
		}

		// Harga khusus pelanggan (member/reseller) menang bila barang ini punya
		// harga di daftarnya; selain itu harga umum + grosir per jumlah.
		unitPrice := hargaDasarGrosir(p.SellPrice, grosir[p.ID], totalQty[p.ID])
		if h, ok := hargaDaftar(khusus[p.ID], totalQty[p.ID]); ok {
			unitPrice = h
		}
		// Kemasan (dus): harganya sendiri, modal = isi × modal satuan dasar,
		// nama satuan di struk = nama kemasan.
		konversi := decimal.NewFromInt(1)
		unitCost := p.CostPrice
		if it.ProductUnitID != "" {
			k, ok := kemasan[it.ProductUnitID]
			if !ok || k.ProductID != it.ProductID {
				return nil, t, fmt.Errorf("%w: kemasan %s tidak cocok dengan produk", helpers.ErrValidation, it.ProductUnitID)
			}
			konversi = k.Conversion
			unitPrice = k.HargaKemasan(p.SellPrice)
			unitCost = helpers.LineAmount(konversi, p.CostPrice)
			if k.Unit != nil {
				unitName = k.Unit.Name
			}
		}
		nama := p.Name
		if it.VariantID != "" {
			v, ok := variants[it.VariantID]
			if !ok || v.ProductID != it.ProductID {
				return nil, t, fmt.Errorf("%w: varian %s tidak cocok dengan produk", helpers.ErrValidation, it.VariantID)
			}
			// Selisih varian berlaku per satuan dasar — satu dus berisi
			// sekian satuan, jadi selisihnya ikut dikali isinya.
			unitPrice += helpers.LineAmount(konversi, v.PriceDelta)
			// Snapshot nama di isi penjualan → struk & riwayat menyebut variannya.
			nama = namaBervarian(p.Name, v.Name)
		}

		lineGross := helpers.LineAmount(it.Qty, unitPrice)
		lineCost := helpers.LineAmount(it.Qty, unitCost)
		if it.DiscountAmount > lineGross {
			return nil, t, fmt.Errorf("%w: diskon baris melebihi nilai baris", helpers.ErrValidation)
		}
		baseAfterDisc := lineGross - it.DiscountAmount

		var lineTax, lineTotal int64
		switch {
		case !taxOn:
			lineTotal = baseAfterDisc
		case outlet.TaxInclusive:
			lineTax = helpers.InclusiveTax(baseAfterDisc, taxRate)
			lineTotal = baseAfterDisc
		default: // eksklusif
			lineTax = helpers.ApplyRate(baseAfterDisc, taxRate)
			lineTotal = baseAfterDisc + lineTax
		}

		out = append(out, pricedItem{
			in: it, productName: nama, unitName: unitName, conversion: konversi, baseCost: p.CostPrice,
			unitPrice: unitPrice, unitCost: unitCost,
			lineGross: lineGross, lineTax: lineTax, lineTotal: lineTotal, lineCost: lineCost,
			trackStock: p.TrackStock,
		})

		t.subtotal += lineGross
		t.sumLineDiscount += it.DiscountAmount
		t.taxAmount += lineTax
		t.costTotal += lineCost
		t.total += lineTotal
	}

	// Diskon transaksi dipotong dari Σ total baris. Melebihinya membuat total
	// negatif — dan pembayaran tunai berapa pun lalu tercatat "berkembalian"
	// lebih besar dari uang yang diterima.
	if in.OrderDiscount > t.total {
		return nil, t, fmt.Errorf("%w: diskon transaksi (%d) melebihi total belanja (%d)",
			helpers.ErrValidation, in.OrderDiscount, t.total)
	}

	t.discountAmount = t.sumLineDiscount + in.OrderDiscount
	baseForService := t.subtotal - t.discountAmount
	if outlet.ServiceChargeRate.GreaterThan(decimal.Zero) && baseForService > 0 {
		t.serviceAmount = helpers.ApplyRate(baseForService, outlet.ServiceChargeRate)
	}
	// total = Σ line_total - diskon order + service + rounding
	t.total = t.total - in.OrderDiscount + t.serviceAmount + t.roundingAmount
	return out, t, nil
}

// paymentResolution memilah pembayaran non-kredit vs kredit (kasbon).
type paymentResolution struct {
	rows         []models.SalePayment
	paidAmount   int64
	changeAmount int64
	creditAmount int64
}

// resolvePayments memvalidasi pembayaran terhadap total: non-kredit boleh
// melebihi (kembalian) — tapi kembalian hanya sebesar uang TUNAI yang
// diterima; bila ada kredit, harus pas dan butuh customer_id + tidak
// melampaui batas kredit.
func resolvePayments(ctx context.Context, tx *gorm.DB, in CheckoutInput, total int64) (paymentResolution, error) {
	var r paymentResolution
	now := time.Now().UTC()

	// Pelanggan diperiksa untuk SEMUA metode bayar, bukan hanya kasbon. Tanpa
	// ini id yang tidak dikenal baru ditolak foreign key saat INSERT sebagai
	// galat 500 — dan di /sync/push galat 500 membatalkan seluruh batch, jadi
	// satu transaksi offline yang merujuk pelanggan terhapus memacetkan
	// antrean perangkat selamanya.
	var cust models.Customer
	if in.CustomerID != "" {
		if err := repositories.FindCustomerInTenant(ctx, tx, in.CustomerID, &cust); err != nil {
			if errors.Is(err, repositories.ErrCustomerNotFound) {
				return r, fmt.Errorf("%w: pelanggan tidak ditemukan", helpers.ErrValidation)
			}
			return r, err
		}
	}

	var nonCredit, tunai int64
	for _, p := range in.Payments {
		if p.Amount <= 0 {
			return r, fmt.Errorf("%w: nominal pembayaran harus > 0", helpers.ErrValidation)
		}
		if p.Method == "credit" {
			r.creditAmount += p.Amount
		} else {
			nonCredit += p.Amount
		}
		if p.Method == "cash" {
			tunai += p.Amount
		}
		r.rows = append(r.rows, models.SalePayment{
			Method: p.Method, Amount: p.Amount, Reference: p.Reference, PaidAt: now,
		})
	}
	r.paidAmount = nonCredit + r.creditAmount

	if r.creditAmount > 0 {
		if in.CustomerID == "" {
			return r, fmt.Errorf("%w: pembayaran kasbon membutuhkan pelanggan", helpers.ErrValidation)
		}
		if nonCredit+r.creditAmount != total {
			return r, fmt.Errorf("%w: total pembayaran (termasuk kasbon) harus sama persis dengan total transaksi", helpers.ErrValidation)
		}
		if cust.CreditLimit > 0 {
			outstanding, err := repositories.OutstandingReceivableTotal(ctx, tx, in.CustomerID)
			if err != nil {
				return r, err
			}
			if outstanding+r.creditAmount > cust.CreditLimit {
				return r, fmt.Errorf("%w: melebihi batas kredit pelanggan", helpers.ErrValidation)
			}
		}
		r.changeAmount = 0
		return r, nil
	}

	if r.paidAmount < total {
		return r, fmt.Errorf("%w: pembayaran kurang dari total", helpers.ErrValidation)
	}
	r.changeAmount = r.paidAmount - total
	// Kembalian keluar dari laci, jadi hanya bisa berasal dari uang tunai.
	// QRIS Rp 70.000 untuk belanja Rp 64.000 akan tercatat "kembalian Rp 6.000"
	// yang tak pernah ada — laci shift (tunai − kembalian) lalu tampak kurang
	// dan laporan tunai per cara bayar bisa minus. Bayar gabungan membuat
	// kesalahan ini mudah terjadi, jadi ditolak di sini.
	if r.changeAmount > tunai {
		return r, fmt.Errorf("%w: pembayaran non-tunai melebihi sisa belanja — kembalian hanya bisa dari uang tunai", helpers.ErrValidation)
	}
	return r, nil
}

// buildSale merangkai model Sale + item + payment (belum disimpan).
func buildSale(ctx context.Context, in CheckoutInput, shift models.Shift, bizDate, now time.Time, receiptNo string, priced []pricedItem, t saleTotals, pay paymentResolution) models.Sale {
	shiftID := shift.ID
	sale := models.Sale{
		ID:             in.SaleID,
		OutletID:       in.OutletID,
		ShiftID:        &shiftID,
		ReceiptNo:      receiptNo,
		IdempotencyKey: in.IdempotencyKey,
		OrderType:      orDefault(in.OrderType, "dine_in"),
		Status:         "completed",
		Subtotal:       t.subtotal,
		DiscountAmount: t.discountAmount,
		TaxAmount:      t.taxAmount,
		ServiceAmount:  t.serviceAmount,
		RoundingAmount: t.roundingAmount,
		Total:          t.total,
		PaidAmount:     pay.paidAmount,
		ChangeAmount:   pay.changeAmount,
		CostTotal:      t.costTotal,
		Note:           in.Note,
		OccurredAt:     now,
		BusinessDate:   bizDate,
		CreatedBy:      reqctx.UserID(ctx),
	}
	if in.CustomerID != "" {
		sale.CustomerID = &in.CustomerID
	}
	if in.ClientCreatedAt != nil {
		sale.ClientCreatedAt = in.ClientCreatedAt
	}

	for _, p := range priced {
		item := models.SaleItem{
			ProductID:      p.in.ProductID,
			ProductName:    p.productName,
			UnitName:       p.unitName,
			Qty:            p.in.Qty,
			UnitPrice:      p.unitPrice,
			UnitCost:       p.unitCost,
			DiscountAmount: p.in.DiscountAmount,
			TaxAmount:      p.lineTax,
			LineTotal:      p.lineTotal,
			Note:           p.in.Note,
			UnitConversion: p.conversion,
		}
		if p.in.VariantID != "" {
			vid := p.in.VariantID
			item.VariantID = &vid
		}
		if p.in.ProductUnitID != "" {
			kid := p.in.ProductUnitID
			item.ProductUnitID = &kid
		}
		sale.Items = append(sale.Items, item)
	}
	sale.Payments = pay.rows
	return sale
}

// stockDeltasForSale menyusun perubahan saldo untuk sebuah checkout:
//   - kind='sale'   : produk ber-track_stock → −qty
//   - kind='recipe' : bahan baku menu ber-resep → −(qty_bahan × qty_jual / yield)
//
// Satu menu F&B biasanya track_stock=false sehingga hanya bahannya yang
// berkurang; produk retail track_stock=true dan tak punya resep.
func stockDeltasForSale(priced []pricedItem, recipes map[string]repositories.RecipeWithItems, outletID string) ([]repositories.StockDelta, error) {
	var deltas []repositories.StockDelta

	for _, p := range priced {
		// Stok selalu dalam SATUAN DASAR: 2 dus isi 40 = 80.
		konversi := p.conversion
		if konversi.IsZero() {
			konversi = decimal.NewFromInt(1)
		}
		qtyDasar := p.in.Qty.Mul(konversi)
		if p.trackStock {
			// Stok barang bervarian dihitung di tingkat BARANG (varian =
			// pilihan harga) — layar stok belum mengenal stok per varian.
			// Modal per satuan dasar supaya nilai persediaan tetap benar.
			deltas = append(deltas, repositories.StockDelta{
				ProductID: p.in.ProductID,
				Delta:     qtyDasar.Neg(), UnitCost: p.baseCost, Kind: "sale",
			})
		}

		rec, ok := recipes[p.in.ProductID]
		if !ok || len(rec.Items) == 0 {
			continue
		}
		yield := rec.Recipe.YieldQty
		if yield.LessThanOrEqual(decimal.Zero) {
			yield = decimal.NewFromInt(1)
		}
		factor := qtyDasar.Div(yield) // porsi terjual relatif terhadap hasil resep
		for _, ing := range rec.Items {
			deltas = append(deltas, repositories.StockDelta{
				ProductID: ing.IngredientProductID,
				Delta:     ing.Qty.Mul(factor).Neg(),
				Kind:      "recipe",
			})
		}
	}
	return deltas, nil
}

// formatReceiptNo: <KODE>-<YYMMDD>-<URUT4>, YYMMDD dari business_date (§13.7).
func formatReceiptNo(outlet models.Outlet, bizDate time.Time, seq int64) string {
	code := ""
	if outlet.Code != nil {
		code = strings.TrimSpace(*outlet.Code)
	}
	if code == "" {
		code = strings.ToUpper(outlet.ID[:4])
	}
	return fmt.Sprintf("%s-%s-%04d", code, bizDate.Format("060102"), seq)
}

// hasDiscount melaporkan apakah checkout memuat diskon apa pun (baris atau
// transaksi) — penentu perlu-tidaknya izin sale.discount.
func hasDiscount(in CheckoutInput) bool {
	if in.OrderDiscount > 0 {
		return true
	}
	for _, it := range in.Items {
		if it.DiscountAmount > 0 {
			return true
		}
	}
	return false
}

func sortedUniqueProductIDs(items []CheckoutItem) []string {
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

// namaBervarian: "Kopi Susu (Besar)".
func namaBervarian(barang, varian string) string {
	if varian = strings.TrimSpace(varian); varian == "" {
		return barang
	}
	return barang + " (" + varian + ")"
}

func variantIDs(items []CheckoutItem) []string {
	var ids []string
	for _, it := range items {
		if it.VariantID != "" {
			ids = append(ids, it.VariantID)
		}
	}
	return ids
}
