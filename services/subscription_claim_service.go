package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"candra/backend-api/config"
	"candra/backend-api/helpers"
	"candra/backend-api/internal/reqctx"
	"candra/backend-api/models"
	"candra/backend-api/repositories"
	"candra/backend-api/structs"

	"gorm.io/gorm"
)

// ── Konfirmasi pembayaran langganan ─────────────────────────────────────────
//
// Alur uang masuk langganan (migrasi 000039):
//
//  1. Tenant membayar di luar aplikasi (transfer, QRIS, ...) lalu MENGIRIM
//     KONFIRMASI: berapa, lewat apa, nama pengirim / nomor referensi.
//     Konfirmasi berstatus 'pending'; paket BELUM berubah.
//  2. Staf keuangan platform mencocokkannya dengan mutasi rekening di panel
//     internal, lalu MENYETUJUI — baru di sini pembayaran dicatat
//     (terapkanPembayaran) dan paket aktif — atau MENOLAK dengan alasan.
//  3. Tenant diberi tahu lewat WhatsApp (outbox), dan melihat keputusannya di
//     halaman Langganan; konfirmasi yang ditolak boleh dikirim ulang.
//
// Dulu tenant sendiri yang mencatat pembayaran dan paket langsung aktif —
// sejak kunci paket ditegakkan, itu pintu untuk memakai paket berbayar tanpa
// membayar.

// SubmitClaimInput adalah masukan konfirmasi pembayaran dari tenant.
type SubmitClaimInput struct {
	InvoiceID      string
	Amount         int64
	Method         string
	Reference      string
	Note           string
	IdempotencyKey string
	RequestHash    string
}

// SubmitPaymentClaim menyimpan konfirmasi pembayaran tenant (status pending).
// Idempoten lewat Idempotency-Key: tombol "Kirim" yang tertekan dua kali atau
// dikirim ulang setelah sinyal putus tidak membuat dua konfirmasi.
//
// Mengembalikan (status HTTP, body JSON, error).
func SubmitPaymentClaim(ctx context.Context, in SubmitClaimInput) (int, []byte, error) {
	if in.IdempotencyKey == "" {
		return 0, nil, fmt.Errorf("%w: header Idempotency-Key wajib", helpers.ErrValidation)
	}
	var (
		outStatus int
		outBody   []byte
	)
	err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		m, err := repositories.LookupIdempotency(ctx, tx, idempotencyScopeSubClaim, in.IdempotencyKey, in.RequestHash)
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

		// Dikunci: pembatalan tagihan (ganti pilihan paket) melewati kunci yang
		// sama, jadi konfirmasi tidak pernah tertinggal di tagihan yang batal.
		inv, err := repositories.LockSubInvoiceForTenant(ctx, tx, in.InvoiceID)
		if err != nil {
			return err
		}
		if inv.Status != "open" && inv.Status != "overdue" {
			return fmt.Errorf("%w: tagihan ini tidak menunggu pembayaran", helpers.ErrConflict)
		}
		if remaining := inv.TotalAmount - inv.PaidAmount; in.Amount > remaining {
			return fmt.Errorf("%w: nominal melebihi sisa tagihan (%s)", helpers.ErrValidation, helpers.FormatRupiah(remaining))
		}

		uid := reqctx.UserID(ctx)
		klaim := models.SubscriptionPaymentClaim{
			SubscriptionInvoiceID: inv.ID,
			Amount:                in.Amount,
			Method:                in.Method,
			Reference:             strings.TrimSpace(in.Reference),
			Note:                  strings.TrimSpace(in.Note),
			Status:                "pending",
		}
		if uid != "" {
			klaim.SubmittedBy = &uid
		}
		if err := repositories.CreateSubClaim(ctx, tx, &klaim); err != nil {
			if helpers.IsDuplicateEntryError(err) {
				return fmt.Errorf("%w: masih ada konfirmasi untuk tagihan ini yang menunggu diverifikasi", helpers.ErrConflict)
			}
			return err
		}

		body, err := json.Marshal(structs.SuccessResponse[structs.PaymentClaimResponse]{
			Success: true,
			Message: "Konfirmasi pembayaran terkirim. Paket aktif setelah pembayarannya diverifikasi.",
			Data:    claimToResponse(klaim, inv.Number),
		})
		if err != nil {
			return err
		}
		if err := repositories.SaveIdempotency(ctx, tx, idempotencyScopeSubClaim, in.IdempotencyKey, in.RequestHash,
			http.StatusCreated, body, idempotencyTTL()); err != nil {
			return err
		}
		outStatus, outBody = http.StatusCreated, body
		return nil
	})
	if err != nil {
		return 0, nil, err
	}
	return outStatus, outBody, nil
}

// ListPaymentClaims: riwayat konfirmasi milik tenant konteks.
func ListPaymentClaims(ctx context.Context) ([]structs.PaymentClaimResponse, error) {
	rows, err := repositories.ListSubClaimsForTenant(ctx, 50)
	if err != nil {
		return nil, err
	}
	out := make([]structs.PaymentClaimResponse, len(rows))
	for i, r := range rows {
		out[i] = claimToResponse(r, "")
	}
	return out, nil
}

// PaymentInstructions: rekening tujuan pembayaran langganan, dari konfigurasi
// (SUBSCRIPTION_BANK_*). nil bila belum diatur — layar lalu menyuruh
// menghubungi penyedia alih-alih menampilkan rekening kosong.
func PaymentInstructions() *structs.PaymentInstructions {
	bank := config.GetEnv("SUBSCRIPTION_BANK_NAME", "")
	norek := config.GetEnv("SUBSCRIPTION_BANK_ACCOUNT", "")
	if bank == "" || norek == "" {
		return nil
	}
	return &structs.PaymentInstructions{
		BankName:      bank,
		AccountNumber: norek,
		AccountHolder: config.GetEnv("SUBSCRIPTION_BANK_HOLDER", ""),
	}
}

// ── Panel internal ──────────────────────────────────────────────────────────

// PlatformListPaymentClaims: antrean konfirmasi lintas tenant untuk staf
// keuangan. status: pending (bawaan) | approved | rejected | all.
func PlatformListPaymentClaims(ctx context.Context, status string) ([]structs.PlatformPaymentClaimResponse, error) {
	switch status {
	case "":
		status = "pending"
	case "all":
		status = ""
	case "pending", "approved", "rejected":
	default:
		return nil, fmt.Errorf("%w: status harus pending, approved, rejected, atau all", helpers.ErrValidation)
	}
	rows, err := repositories.PlatformListSubClaims(ctx, status, 200)
	if err != nil {
		return nil, err
	}
	out := make([]structs.PlatformPaymentClaimResponse, len(rows))
	for i, r := range rows {
		out[i] = structs.PlatformPaymentClaimResponse{
			PaymentClaimResponse: claimToResponse(r.SubscriptionPaymentClaim, r.InvoiceNumber),
			TenantID:             r.TenantID,
			BusinessName:         r.BusinessName,
			TenantPhone:          r.TenantPhone,
			InvoiceTotal:         r.InvoiceTotal,
			InvoicePaid:          r.InvoicePaid,
			PlanName:             r.PlanName,
		}
	}
	return out, nil
}

// PlatformApprovePaymentClaim menyetujui konfirmasi: pembayaran dicatat
// (terapkanPembayaran — tagihan lunas → paket aktif), konfirmasi ditandai
// disetujui & ditautkan ke pembayarannya, tenant diberi tahu lewat WhatsApp.
// Semuanya dalam SATU transaksi bertenant milik konfirmasi itu.
//
// Baris konfirmasi dikunci lebih dulu: persetujuan kedua (dua staf menekan
// bersamaan, atau tombol tertekan dua kali) ditolak "sudah diputus", bukan
// mencatat pembayaran dua kali.
func PlatformApprovePaymentClaim(ctx context.Context, claimID string) (structs.SubInvoiceResponse, error) {
	var out structs.SubInvoiceResponse
	klaim, err := repositories.FindSubClaimAnyTenant(ctx, claimID)
	if err != nil {
		return out, err
	}
	adminID := reqctx.PlatformAdminID(ctx)
	tctx := reqctx.WithTenantID(ctx, klaim.TenantID)

	err = repositories.WithTenant(tctx, func(tx *gorm.DB) error {
		k, err := repositories.LockSubClaim(tctx, tx, claimID)
		if err != nil {
			return err
		}
		if k.Status != "pending" {
			return fmt.Errorf("%w: konfirmasi ini sudah diputus (%s)", helpers.ErrConflict, k.Status)
		}
		ref := k.Reference
		if k.Note != "" {
			ref = strings.TrimSpace(ref + " · " + k.Note)
		}
		inv, bayar, err := terapkanPembayaran(tctx, tx, k.SubscriptionInvoiceID, k.Amount, k.Method, potongRune(ref, 100))
		if err != nil {
			return err
		}
		if err := repositories.ReviewSubClaim(tctx, tx, &k, "approved", adminID, "", &bayar.ID); err != nil {
			return err
		}
		beritahuTenant(tctx, tx, "subscription.payment_approved", inv, k, "")
		out = subInvoiceToResponse(inv)
		return nil
	})
	if err != nil {
		return out, err
	}
	auditKeputusan(ctx, klaim.TenantID, adminID, "subscription.payment_claim.approve", claimID)
	return out, nil
}

// PlatformRejectPaymentClaim menolak konfirmasi dengan alasan yang akan
// dibaca tenant ("nominal tidak ditemukan di mutasi", "nama pengirim
// berbeda", ...). Tagihannya tetap terbuka; tenant boleh mengirim ulang.
func PlatformRejectPaymentClaim(ctx context.Context, claimID, reason string) error {
	reason = strings.TrimSpace(reason)
	if len([]rune(reason)) < 3 {
		return fmt.Errorf("%w: alasan penolakan wajib diisi — tenant perlu tahu apa yang harus diperbaiki", helpers.ErrValidation)
	}
	klaim, err := repositories.FindSubClaimAnyTenant(ctx, claimID)
	if err != nil {
		return err
	}
	adminID := reqctx.PlatformAdminID(ctx)
	tctx := reqctx.WithTenantID(ctx, klaim.TenantID)

	err = repositories.WithTenant(tctx, func(tx *gorm.DB) error {
		k, err := repositories.LockSubClaim(tctx, tx, claimID)
		if err != nil {
			return err
		}
		if k.Status != "pending" {
			return fmt.Errorf("%w: konfirmasi ini sudah diputus (%s)", helpers.ErrConflict, k.Status)
		}
		inv, err := repositories.FindSubInvoiceForTenant(tctx, tx, k.SubscriptionInvoiceID)
		if err != nil {
			return err
		}
		if err := repositories.ReviewSubClaim(tctx, tx, &k, "rejected", adminID, reason, nil); err != nil {
			return err
		}
		beritahuTenant(tctx, tx, "subscription.payment_rejected", inv, k, reason)
		return nil
	})
	if err != nil {
		return err
	}
	auditKeputusan(ctx, klaim.TenantID, adminID, "subscription.payment_claim.reject", claimID)
	return nil
}

// beritahuTenant mengantrekan pemberitahuan WhatsApp ke nomor usaha — di
// dalam tx yang sama dengan keputusannya (pola outbox, §5.14). Tenant tanpa
// nomor HP dilewati: keputusannya tetap terlihat di halaman Langganan.
func beritahuTenant(ctx context.Context, tx *gorm.DB, topik string, inv models.SubscriptionInvoice, k models.SubscriptionPaymentClaim, alasan string) {
	tid := reqctx.TenantID(ctx)
	var tenant models.Tenant
	if err := repositories.FindTenantByID(ctx, tx, tid, &tenant); err != nil || tenant.Phone == "" {
		return
	}
	payload := map[string]any{
		"channel":    "whatsapp",
		"to":         tenant.Phone,
		"nama_usaha": tenant.BusinessName,
		"nomor":      inv.Number,
		"jumlah":     helpers.FormatRupiah(k.Amount),
		"alasan":     alasan,
	}
	if inv.Status == "paid" {
		payload["sampai"] = inv.PeriodEnd.Format("02 Jan 2006")
	}
	_ = EnqueueNotification(ctx, tx, &tid, topik, payload)
}

// auditKeputusan mencatat siapa memutus konfirmasi mana, bertanda tenant
// pemiliknya — jejak uang masuk harus bisa ditelusuri dari kedua sisi.
// Gagal mencatat tidak membatalkan keputusan (uangnya sudah tercatat); ia
// masuk log sebagai galat supaya kelihatan.
func auditKeputusan(ctx context.Context, tenantID, adminID, aksi, claimID string) {
	tabel := "subscription_payment_claims"
	if err := repositories.WriteAuditLogs(ctx, []repositories.AuditEntry{{
		TenantID: &tenantID, ActorType: "admin", ActorID: &adminID,
		Action: aksi, TargetTable: &tabel, TargetID: &claimID,
	}}); err != nil {
		helpers.LoggerFromContext(ctx).Error("mencatat audit konfirmasi pembayaran", slog.Any("error", err))
	}
}

func claimToResponse(c models.SubscriptionPaymentClaim, nomorTagihan string) structs.PaymentClaimResponse {
	r := structs.PaymentClaimResponse{
		ID: c.ID, InvoiceID: c.SubscriptionInvoiceID, InvoiceNumber: nomorTagihan,
		Amount: c.Amount, Method: c.Method, Reference: c.Reference, Note: c.Note,
		Status: c.Status, RejectReason: c.RejectReason,
		CreatedAt: c.CreatedAt.UTC().Format(saleTimeLayout),
	}
	if c.ReviewedAt != nil {
		r.ReviewedAt = c.ReviewedAt.UTC().Format(saleTimeLayout)
	}
	return r
}

// potongRune memangkas s menjadi paling banyak n karakter (bukan byte) —
// kolom referensi pembayaran dibatasi 100, dan nama pengirim boleh beraksen.
func potongRune(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
