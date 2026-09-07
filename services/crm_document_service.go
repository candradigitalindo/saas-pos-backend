package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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

// Layanan dokumen CRM — penawaran → proyek → invoice bertermin (Fase 9, §5.9,
// blueprint E.2).
//
// Aturan integrasi kunci: invoice yang LUNAS dicatat sebagai SATU penjualan di
// tabel `sales` yang sama dengan POS (`invoices.sale_id` terisi) — omzet & laba
// tetap satu pintu, tanpa entri ganda.

const idempotencyScopeInvoicePay = "invoice.payment"

const crmDateLayout = "2006-01-02"

// parseCRMDate membaca tanggal YYYY-MM-DD opsional.
func parseCRMDate(field, s string, required bool) (*time.Time, error) {
	if s == "" {
		if required {
			return nil, fmt.Errorf("%w: %s wajib (YYYY-MM-DD)", helpers.ErrValidation, field)
		}
		return nil, nil
	}
	t, err := time.Parse(crmDateLayout, s)
	if err != nil {
		return nil, fmt.Errorf("%w: %s harus berformat YYYY-MM-DD", helpers.ErrValidation, field)
	}
	return &t, nil
}

// ── Quotation ─────────────────────────────────────────────────────────────

// CreateQuotation menyusun penawaran baru dengan item ber-snapshot harga.
func CreateQuotation(ctx context.Context, in structs.QuotationCreateRequest) (structs.QuotationResponse, error) {
	var out structs.QuotationResponse
	err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		validUntil, err := parseCRMDate("valid_until", in.ValidUntil, false)
		if err != nil {
			return err
		}
		if err := repositories.FindCustomerInTenant(ctx, tx, in.CustomerID, &models.Customer{}); err != nil {
			return fmt.Errorf("%w: pelanggan tidak ditemukan", helpers.ErrValidation)
		}

		q := models.Quotation{
			CustomerID: in.CustomerID,
			OwnerID:    reqctx.UserID(ctx),
			ValidUntil: validUntil,
			Status:     "draft",
			TaxAmount:  in.TaxAmount,
			Note:       in.Note,
		}
		if in.DealID != "" {
			q.DealID = &in.DealID
		}

		var subtotal, discTotal int64
		for _, it := range in.Items {
			qty, derr := decimal.NewFromString(it.Qty)
			if derr != nil || qty.LessThanOrEqual(decimal.Zero) {
				return fmt.Errorf("%w: qty item tidak valid", helpers.ErrValidation)
			}
			gross := helpers.LineAmount(qty, it.UnitPrice)
			if it.DiscountAmount > gross {
				return fmt.Errorf("%w: diskon baris melebihi nilai baris", helpers.ErrValidation)
			}
			line := gross - it.DiscountAmount
			item := models.QuotationItem{
				Description: it.Description, Qty: qty, UnitPrice: it.UnitPrice,
				DiscountAmount: it.DiscountAmount, LineTotal: line,
			}
			if it.ProductID != "" {
				item.ProductID = &it.ProductID
			}
			q.Items = append(q.Items, item)
			subtotal += gross
			discTotal += it.DiscountAmount
		}
		q.Subtotal = subtotal
		q.DiscountAmount = discTotal
		q.Total = subtotal - discTotal + in.TaxAmount

		q.Number, err = repositories.NextDocumentNumber(ctx, tx, "quotation", "QUO")
		if err != nil {
			return err
		}
		if err := repositories.CreateQuotation(ctx, tx, &q); err != nil {
			return err
		}
		reloaded, err := repositories.FindQuotation(ctx, tx, q.ID)
		if err != nil {
			return err
		}
		out = quotationToResponse(reloaded, "")
		return nil
	})
	return out, err
}

// SendQuotation mengubah status penawaran draft → sent.
func SendQuotation(ctx context.Context, id string) (structs.QuotationResponse, error) {
	return transitionQuotation(ctx, id, "draft", "sent", nil)
}

// RejectQuotation menandai penawaran ditolak.
func RejectQuotation(ctx context.Context, id string) (structs.QuotationResponse, error) {
	return transitionQuotation(ctx, id, "sent", "rejected", nil)
}

// transitionQuotation menerapkan perpindahan status sederhana.
func transitionQuotation(ctx context.Context, id, from, to string, extra func(*models.Quotation)) (structs.QuotationResponse, error) {
	var out structs.QuotationResponse
	err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		q, err := repositories.FindQuotation(ctx, tx, id)
		if err != nil {
			return err
		}
		if q.Status != from {
			return fmt.Errorf("%w: penawaran harus berstatus %q", helpers.ErrConflict, from)
		}
		q.Status = to
		if extra != nil {
			extra(&q)
		}
		if err := repositories.SaveQuotation(ctx, tx, &q); err != nil {
			return err
		}
		reloaded, err := repositories.FindQuotation(ctx, tx, id)
		if err != nil {
			return err
		}
		out = quotationToResponse(reloaded, "")
		return nil
	})
	return out, err
}

// AcceptQuotation menyetujui penawaran dan OTOMATIS membuat proyek (tanpa
// mengetik ulang). Idempoten: penawaran yang sudah diterima mengembalikan proyek
// yang sudah ada.
func AcceptQuotation(ctx context.Context, id string) (structs.QuotationResponse, error) {
	var out structs.QuotationResponse
	err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		q, err := repositories.FindQuotation(ctx, tx, id)
		if err != nil {
			return err
		}

		if q.Status == "accepted" {
			if p, ok, perr := repositories.FindProjectByQuotation(ctx, tx, q.ID); perr != nil {
				return perr
			} else if ok {
				out = quotationToResponse(q, p.ID)
				return nil
			}
		} else if q.Status != "sent" && q.Status != "draft" {
			return fmt.Errorf("%w: hanya penawaran draft/terkirim yang bisa disetujui", helpers.ErrConflict)
		}

		now := time.Now().UTC()
		q.Status = "accepted"
		q.AcceptedAt = &now
		if err := repositories.SaveQuotation(ctx, tx, &q); err != nil {
			return err
		}

		project := models.Project{
			CustomerID:    q.CustomerID,
			QuotationID:   &q.ID,
			OwnerID:       q.OwnerID,
			Name:          "Proyek " + q.Number,
			Status:        "active",
			ContractValue: q.Total,
		}
		if err := repositories.CreateProject(ctx, tx, &project); err != nil {
			return err
		}

		reloaded, err := repositories.FindQuotation(ctx, tx, id)
		if err != nil {
			return err
		}
		out = quotationToResponse(reloaded, project.ID)
		return nil
	})
	return out, err
}

// ── Project ───────────────────────────────────────────────────────────────

// CreateProject membuat proyek langsung (tanpa lewat penawaran).
func CreateProject(ctx context.Context, in structs.ProjectCreateRequest) (structs.ProjectResponse, error) {
	var out structs.ProjectResponse
	err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		start, err := parseCRMDate("start_date", in.StartDate, false)
		if err != nil {
			return err
		}
		due, err := parseCRMDate("due_date", in.DueDate, false)
		if err != nil {
			return err
		}
		p := models.Project{
			CustomerID:    in.CustomerID,
			OwnerID:       reqctx.UserID(ctx),
			Name:          in.Name,
			StartDate:     start,
			DueDate:       due,
			Status:        "active",
			ContractValue: in.ContractValue,
		}
		if in.QuotationID != "" {
			p.QuotationID = &in.QuotationID
		}
		if err := repositories.CreateProject(ctx, tx, &p); err != nil {
			return err
		}
		reloaded, err := repositories.FindProject(ctx, tx, p.ID)
		if err != nil {
			return err
		}
		out = projectToResponse(reloaded)
		return nil
	})
	return out, err
}

// UpdateProject mengubah nama/tanggal/status/nilai kontrak proyek.
func UpdateProject(ctx context.Context, id string, in structs.ProjectUpdateRequest) (structs.ProjectResponse, error) {
	var out structs.ProjectResponse
	err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		p, err := repositories.FindProject(ctx, tx, id)
		if err != nil {
			return err
		}
		if in.Name != nil {
			p.Name = *in.Name
		}
		if in.Status != nil {
			p.Status = *in.Status
		}
		if in.ContractValue != nil {
			p.ContractValue = *in.ContractValue
		}
		if in.StartDate != nil {
			t, derr := parseCRMDate("start_date", *in.StartDate, false)
			if derr != nil {
				return derr
			}
			p.StartDate = t
		}
		if in.DueDate != nil {
			t, derr := parseCRMDate("due_date", *in.DueDate, false)
			if derr != nil {
				return derr
			}
			p.DueDate = t
		}
		if err := repositories.SaveProject(ctx, tx, &p); err != nil {
			return err
		}
		reloaded, err := repositories.FindProject(ctx, tx, id)
		if err != nil {
			return err
		}
		out = projectToResponse(reloaded)
		return nil
	})
	return out, err
}

// AddProjectTask menambah tugas/deliverable ke proyek.
func AddProjectTask(ctx context.Context, projectID string, in structs.ProjectTaskRequest) (structs.ProjectResponse, error) {
	return mutateProject(ctx, projectID, func(tx *gorm.DB, p models.Project) error {
		due, err := parseCRMDate("due_date", in.DueDate, false)
		if err != nil {
			return err
		}
		return repositories.CreateProjectTask(ctx, tx, &models.ProjectTask{
			ProjectID: p.ID, Title: in.Title, DueDate: due, SortOrder: len(p.Tasks) + 1,
		})
	})
}

// AddProjectExpense mencatat biaya proyek (→ cost_total saat invoice lunas).
func AddProjectExpense(ctx context.Context, projectID string, in structs.ProjectExpenseRequest) (structs.ProjectResponse, error) {
	return mutateProject(ctx, projectID, func(tx *gorm.DB, p models.Project) error {
		spent, err := parseCRMDate("spent_at", in.SpentAt, true)
		if err != nil {
			return err
		}
		return repositories.CreateProjectExpense(ctx, tx, &models.ProjectExpense{
			ProjectID: p.ID, Description: in.Description, Amount: in.Amount,
			SpentAt: *spent, ReceiptURL: in.ReceiptURL,
		})
	})
}

// mutateProject memuat proyek (cek visibilitas), menjalankan fn, lalu mengembalikan bentuk terbaru.
func mutateProject(ctx context.Context, id string, fn func(tx *gorm.DB, p models.Project) error) (structs.ProjectResponse, error) {
	var out structs.ProjectResponse
	err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		p, err := repositories.FindProject(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := fn(tx, p); err != nil {
			return err
		}
		reloaded, err := repositories.FindProject(ctx, tx, id)
		if err != nil {
			return err
		}
		out = projectToResponse(reloaded)
		return nil
	})
	return out, err
}

// ── Invoice ───────────────────────────────────────────────────────────────

// CreateInvoice menyusun invoice bertermin dengan item ber-snapshot harga.
func CreateInvoice(ctx context.Context, in structs.InvoiceCreateRequest) (structs.InvoiceResponse, error) {
	var out structs.InvoiceResponse
	err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		due, err := parseCRMDate("due_date", in.DueDate, true)
		if err != nil {
			return err
		}
		if err := repositories.FindCustomerInTenant(ctx, tx, in.CustomerID, &models.Customer{}); err != nil {
			return fmt.Errorf("%w: pelanggan tidak ditemukan", helpers.ErrValidation)
		}
		if in.ProjectID != "" {
			if _, perr := repositories.FindProject(ctx, tx, in.ProjectID); perr != nil {
				return fmt.Errorf("%w: proyek tidak ditemukan", helpers.ErrValidation)
			}
		}

		now := time.Now().UTC()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

		inv := models.Invoice{
			CustomerID:     in.CustomerID,
			OwnerID:        reqctx.UserID(ctx),
			IssueDate:      today,
			DueDate:        *due,
			Status:         "draft",
			DiscountAmount: in.DiscountAmount,
			TaxAmount:      in.TaxAmount,
			TermLabel:      in.TermLabel,
		}
		if in.ProjectID != "" {
			inv.ProjectID = &in.ProjectID
		}
		if in.QuotationID != "" {
			inv.QuotationID = &in.QuotationID
		}

		var subtotal int64
		for _, it := range in.Items {
			qty, derr := decimal.NewFromString(it.Qty)
			if derr != nil || qty.LessThanOrEqual(decimal.Zero) {
				return fmt.Errorf("%w: qty item tidak valid", helpers.ErrValidation)
			}
			line := helpers.LineAmount(qty, it.UnitPrice)
			item := models.InvoiceItem{
				Description: it.Description, Qty: qty, UnitPrice: it.UnitPrice, LineTotal: line,
			}
			if it.ProductID != "" {
				item.ProductID = &it.ProductID
			}
			inv.Items = append(inv.Items, item)
			subtotal += line
		}
		if in.DiscountAmount > subtotal {
			return fmt.Errorf("%w: diskon melebihi subtotal", helpers.ErrValidation)
		}
		inv.Subtotal = subtotal
		inv.Total = subtotal - in.DiscountAmount + in.TaxAmount

		inv.Number, err = repositories.NextDocumentNumber(ctx, tx, "invoice", "INV")
		if err != nil {
			return err
		}
		if err := repositories.CreateInvoice(ctx, tx, &inv); err != nil {
			return err
		}
		reloaded, err := repositories.FindInvoice(ctx, tx, inv.ID)
		if err != nil {
			return err
		}
		out = invoiceToResponse(reloaded)
		return nil
	})
	return out, err
}

// SendInvoice mengubah status invoice draft → sent.
func SendInvoice(ctx context.Context, id string) (structs.InvoiceResponse, error) {
	var out structs.InvoiceResponse
	err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		inv, err := repositories.FindInvoice(ctx, tx, id)
		if err != nil {
			return err
		}
		if inv.Status != "draft" {
			return fmt.Errorf("%w: hanya invoice draft yang bisa dikirim", helpers.ErrConflict)
		}
		inv.Status = "sent"
		if err := repositories.SaveInvoice(ctx, tx, &inv); err != nil {
			return err
		}
		reloaded, err := repositories.FindInvoice(ctx, tx, id)
		if err != nil {
			return err
		}
		out = invoiceToResponse(reloaded)
		return nil
	})
	return out, err
}

// VoidInvoice membatalkan invoice yang belum ada pembayaran.
func VoidInvoice(ctx context.Context, id string) (structs.InvoiceResponse, error) {
	var out structs.InvoiceResponse
	err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		inv, err := repositories.FindInvoice(ctx, tx, id)
		if err != nil {
			return err
		}
		if inv.Status == "paid" || inv.Status == "void" {
			return fmt.Errorf("%w: invoice tidak bisa dibatalkan", helpers.ErrConflict)
		}
		if inv.PaidAmount > 0 {
			return fmt.Errorf("%w: invoice sudah menerima pembayaran", helpers.ErrConflict)
		}
		inv.Status = "void"
		if err := repositories.SaveInvoice(ctx, tx, &inv); err != nil {
			return err
		}
		reloaded, err := repositories.FindInvoice(ctx, tx, id)
		if err != nil {
			return err
		}
		out = invoiceToResponse(reloaded)
		return nil
	})
	return out, err
}

// InvoicePayInput adalah masukan pembayaran invoice.
type InvoicePayInput struct {
	InvoiceID      string
	Amount         int64
	Method         string
	ProofURL       string
	IdempotencyKey string
	RequestHash    string
}

// PayInvoice mencatat pembayaran bertahap. Idempoten lewat Idempotency-Key.
// Saat LUNAS, invoice dicatat sebagai satu penjualan di `sales` (§13, blueprint
// E.2) dan ikut agregat laporan — omzet & laba satu pintu, tanpa entri ganda.
//
// Mengembalikan (status HTTP, body JSON, error).
func PayInvoice(ctx context.Context, in InvoicePayInput) (int, []byte, error) {
	if in.IdempotencyKey == "" {
		return 0, nil, fmt.Errorf("%w: header Idempotency-Key wajib", helpers.ErrValidation)
	}
	if in.Amount <= 0 {
		return 0, nil, fmt.Errorf("%w: nominal pembayaran harus > 0", helpers.ErrValidation)
	}

	var (
		outStatus int
		outBody   []byte
	)
	txErr := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		m, err := repositories.LookupIdempotency(ctx, tx, idempotencyScopeInvoicePay, in.IdempotencyKey, in.RequestHash)
		if err != nil {
			return err
		}
		if m.Found {
			if !m.SameRequest {
				return fmt.Errorf("%w: Idempotency-Key sudah dipakai untuk permintaan berbeda", helpers.ErrConflict)
			}
			outStatus, outBody = m.ResponseStatus, m.ResponseBody
			return nil
		}

		inv, err := repositories.FindInvoice(ctx, tx, in.InvoiceID)
		if err != nil {
			return err
		}
		if inv.Status != "sent" && inv.Status != "partial" && inv.Status != "overdue" {
			return fmt.Errorf("%w: invoice tidak dalam status yang bisa dibayar", helpers.ErrConflict)
		}
		outstanding := inv.Total - inv.PaidAmount
		if in.Amount > outstanding {
			return fmt.Errorf("%w: pembayaran melebihi sisa tagihan (%d)", helpers.ErrValidation, outstanding)
		}

		outletID, err := repositories.FirstOutletID(ctx, tx)
		if err != nil {
			return err
		}
		var outlet models.Outlet
		if err := repositories.FindOutletByID(ctx, tx, outletID, &outlet); err != nil {
			return err
		}
		now := time.Now().UTC()
		bizDate, err := timez.BusinessDate(now, outlet.Timezone, outlet.DayStartOffset())
		if err != nil {
			return err
		}

		if err := repositories.CreateInvoicePayment(ctx, tx, &models.InvoicePayment{
			InvoiceID: inv.ID, Amount: in.Amount, Method: in.Method,
			PaidAt: now, BusinessDate: bizDate, ProofURL: in.ProofURL,
		}); err != nil {
			return err
		}

		inv.PaidAmount += in.Amount
		if inv.PaidAmount >= inv.Total {
			inv.Status = "paid"
			saleID, serr := recordInvoiceSale(ctx, tx, inv, outletID, bizDate, now)
			if serr != nil {
				return serr
			}
			inv.SaleID = &saleID
		} else {
			inv.Status = "partial"
		}
		if err := repositories.SaveInvoice(ctx, tx, &inv); err != nil {
			return err
		}

		reloaded, err := repositories.FindInvoice(ctx, tx, inv.ID)
		if err != nil {
			return err
		}
		body, err := json.Marshal(structs.SuccessResponse[structs.InvoiceResponse]{
			Success: true, Message: "Pembayaran diterima", Data: invoiceToResponse(reloaded),
		})
		if err != nil {
			return err
		}
		if err := repositories.SaveIdempotency(ctx, tx, idempotencyScopeInvoicePay, in.IdempotencyKey, in.RequestHash,
			http.StatusCreated, body, idempotencyTTL()); err != nil {
			return err
		}
		outStatus, outBody = http.StatusCreated, body
		return nil
	})
	if txErr != nil {
		return 0, nil, txErr
	}
	return outStatus, outBody, nil
}

// recordInvoiceSale membuat SATU penjualan sintetis di `sales` untuk invoice yang
// lunas: header bernilai penuh (item TIDAK dipindah — baris invoice bisa berupa
// jasa bebas teks tanpa product_id), pembayaran = seluruh invoice_payments,
// cost_total = total biaya proyek terkait. Lalu memutakhirkan
// daily_sales_summaries agar muncul di laporan yang sama dengan POS.
func recordInvoiceSale(ctx context.Context, tx *gorm.DB, inv models.Invoice, outletID string, bizDate, now time.Time) (string, error) {
	var costTotal int64
	if inv.ProjectID != nil {
		c, err := repositories.SumProjectExpenses(ctx, tx, *inv.ProjectID)
		if err != nil {
			return "", err
		}
		costTotal = c
	}

	full, err := repositories.FindInvoice(ctx, tx, inv.ID)
	if err != nil {
		return "", err
	}

	sale := models.Sale{
		OutletID:       outletID,
		CustomerID:     &inv.CustomerID,
		ReceiptNo:      inv.Number,
		IdempotencyKey: "invoice:" + inv.ID,
		OrderType:      "pickup",
		Status:         "completed",
		Subtotal:       inv.Subtotal,
		DiscountAmount: inv.DiscountAmount,
		TaxAmount:      inv.TaxAmount,
		Total:          inv.Total,
		PaidAmount:     inv.Total,
		CostTotal:      costTotal,
		Note:           "Invoice " + inv.Number,
		OccurredAt:     now,
		BusinessDate:   bizDate,
		CreatedBy:      inv.OwnerID,
	}
	for _, p := range full.Payments {
		sale.Payments = append(sale.Payments, models.SalePayment{
			Method: p.Method, Amount: p.Amount, PaidAt: p.PaidAt,
		})
	}
	if err := repositories.CreateSale(ctx, tx, &sale); err != nil {
		return "", err
	}
	if err := repositories.ApplySaleToSummary(ctx, tx, &sale); err != nil {
		return "", err
	}
	return sale.ID, nil
}

// ── Pemetaan DTO ──────────────────────────────────────────────────────────

func quotationToResponse(q models.Quotation, projectID string) structs.QuotationResponse {
	r := structs.QuotationResponse{
		ID: q.ID, Number: q.Number, CustomerID: q.CustomerID, OwnerID: q.OwnerID,
		Status: q.Status, Subtotal: q.Subtotal, DiscountAmount: q.DiscountAmount,
		TaxAmount: q.TaxAmount, Total: q.Total, Note: q.Note, ProjectID: projectID,
		CreatedAt: q.CreatedAt.Format(saleTimeLayout),
	}
	if q.DealID != nil {
		r.DealID = *q.DealID
	}
	if q.ValidUntil != nil {
		r.ValidUntil = q.ValidUntil.Format(crmDateLayout)
	}
	if q.AcceptedAt != nil {
		r.AcceptedAt = q.AcceptedAt.Format(saleTimeLayout)
	}
	for _, it := range q.Items {
		ir := structs.QuotationItemResponse{
			ID: it.ID, Description: it.Description, Qty: it.Qty.String(),
			UnitPrice: it.UnitPrice, DiscountAmount: it.DiscountAmount, LineTotal: it.LineTotal,
		}
		if it.ProductID != nil {
			ir.ProductID = *it.ProductID
		}
		r.Items = append(r.Items, ir)
	}
	return r
}

func projectToResponse(p models.Project) structs.ProjectResponse {
	r := structs.ProjectResponse{
		ID: p.ID, CustomerID: p.CustomerID, OwnerID: p.OwnerID, Name: p.Name,
		Status: p.Status, ContractValue: p.ContractValue,
		CreatedAt: p.CreatedAt.Format(saleTimeLayout),
	}
	if p.QuotationID != nil {
		r.QuotationID = *p.QuotationID
	}
	if p.StartDate != nil {
		r.StartDate = p.StartDate.Format(crmDateLayout)
	}
	if p.DueDate != nil {
		r.DueDate = p.DueDate.Format(crmDateLayout)
	}
	for _, t := range p.Tasks {
		tr := structs.ProjectTaskResponse{ID: t.ID, Title: t.Title, SortOrder: t.SortOrder}
		if t.DueDate != nil {
			tr.DueDate = t.DueDate.Format(crmDateLayout)
		}
		if t.DoneAt != nil {
			tr.DoneAt = t.DoneAt.Format(saleTimeLayout)
		}
		r.Tasks = append(r.Tasks, tr)
	}
	for _, e := range p.Expenses {
		r.TotalExpense += e.Amount
		r.Expenses = append(r.Expenses, structs.ProjectExpenseResponse{
			ID: e.ID, Description: e.Description, Amount: e.Amount,
			SpentAt: e.SpentAt.Format(crmDateLayout), ReceiptURL: e.ReceiptURL,
		})
	}
	return r
}

func invoiceToResponse(in models.Invoice) structs.InvoiceResponse {
	r := structs.InvoiceResponse{
		ID: in.ID, Number: in.Number, CustomerID: in.CustomerID, OwnerID: in.OwnerID,
		IssueDate: in.IssueDate.Format(crmDateLayout), DueDate: in.DueDate.Format(crmDateLayout),
		Status: in.Status, Subtotal: in.Subtotal, DiscountAmount: in.DiscountAmount,
		TaxAmount: in.TaxAmount, Total: in.Total, PaidAmount: in.PaidAmount,
		Outstanding: in.Total - in.PaidAmount, TermLabel: in.TermLabel,
		CreatedAt: in.CreatedAt.Format(saleTimeLayout),
	}
	if in.ProjectID != nil {
		r.ProjectID = *in.ProjectID
	}
	if in.QuotationID != nil {
		r.QuotationID = *in.QuotationID
	}
	if in.SaleID != nil {
		r.SaleID = *in.SaleID
	}
	for _, it := range in.Items {
		ir := structs.InvoiceItemResponse{
			ID: it.ID, Description: it.Description, Qty: it.Qty.String(),
			UnitPrice: it.UnitPrice, LineTotal: it.LineTotal,
		}
		if it.ProductID != nil {
			ir.ProductID = *it.ProductID
		}
		r.Items = append(r.Items, ir)
	}
	for _, p := range in.Payments {
		r.Payments = append(r.Payments, structs.InvoicePaymentResponse{
			ID: p.ID, Amount: p.Amount, Method: p.Method,
			PaidAt: p.PaidAt.Format(saleTimeLayout), BusinessDate: p.BusinessDate.Format(crmDateLayout),
			ProofURL: p.ProofURL,
		})
	}
	return r
}

// QuotationToResponse / ProjectToResponse / InvoiceToResponse — untuk controller list.
func QuotationToResponse(q models.Quotation) structs.QuotationResponse {
	return quotationToResponse(q, "")
}
func ProjectToResponse(p models.Project) structs.ProjectResponse { return projectToResponse(p) }
func InvoiceToResponse(i models.Invoice) structs.InvoiceResponse { return invoiceToResponse(i) }
