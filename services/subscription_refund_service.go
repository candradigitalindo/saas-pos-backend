package services

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"candra/backend-api/helpers"
	"candra/backend-api/internal/reqctx"
	"candra/backend-api/models"
	"candra/backend-api/repositories"
	"candra/backend-api/structs"

	"gorm.io/gorm"
)

// Antrean pengembalian dana langganan (migrasi 000041).
//
// Pengembalian dihitung saat pemilik menghentikan langganan (CancelSubscription)
// dan dulu berhenti di situ: tercatat, tapi tidak ada yang mentransfernya,
// tidak ada rekening tujuan, dan pemilik tidak tahu uangnya sudah dikirim atau
// belum. Kini ia menunggu di panel; staf keuangan (billing.refund) mentransfer
// ke rekening yang diisi pemilik lalu menandainya dengan nomor referensi —
// pemilik diberi tahu lewat WhatsApp dan melihat statusnya di menu Langganan.

// PlatformListRefunds: antrean lintas tenant. status: pending (bawaan) | paid | all.
func PlatformListRefunds(ctx context.Context, status string) ([]structs.PlatformRefundResponse, error) {
	switch status {
	case "":
		status = "pending"
	case "all":
		status = ""
	case "pending", "paid":
	default:
		return nil, fmt.Errorf("%w: status harus pending, paid, atau all", helpers.ErrValidation)
	}
	rows, err := repositories.PlatformListRefunds(ctx, status, 200)
	if err != nil {
		return nil, err
	}
	out := make([]structs.PlatformRefundResponse, len(rows))
	for i, r := range rows {
		out[i] = structs.PlatformRefundResponse{
			SubscriptionRefundResponse: refundToResponse(r.SubscriptionRefund),
			TenantID:                   r.TenantID,
			BusinessName:               r.BusinessName,
			TenantPhone:                r.TenantPhone,
			InvoiceNumber:              r.InvoiceNumber,
			InvoicePaid:                r.InvoicePaid,
			PlanName:                   r.PlanName,
			Reason:                     r.Reason,
		}
	}
	return out, nil
}

// PlatformMarkRefundPaid menandai pengembalian SUDAH ditransfer, dengan nomor
// referensi transfernya. Baris dikunci: tanda kedua (dua staf, atau tombol
// tertekan dua kali) ditolak, bukan mencatat dua kali.
func PlatformMarkRefundPaid(ctx context.Context, refundID, reference string) (structs.SubscriptionRefundResponse, error) {
	var out structs.SubscriptionRefundResponse
	reference = strings.TrimSpace(reference)
	if len([]rune(reference)) < 3 {
		return out, fmt.Errorf("%w: isi nomor referensi transfer — bukti bahwa uangnya sudah dikirim", helpers.ErrValidation)
	}
	r0, err := repositories.FindRefundAnyTenant(ctx, refundID)
	if err != nil {
		return out, err
	}
	adminID := reqctx.PlatformAdminID(ctx)
	tctx := reqctx.WithTenantID(ctx, r0.TenantID)
	err = repositories.WithTenant(tctx, func(tx *gorm.DB) error {
		r, err := repositories.LockRefund(tctx, tx, refundID)
		if err != nil {
			return err
		}
		if r.Status != "pending" {
			return fmt.Errorf("%w: pengembalian ini tidak menunggu ditransfer (%s)", helpers.ErrConflict, r.Status)
		}
		if err := repositories.MarkRefundPaid(tctx, tx, &r, adminID, potongRune(reference, 100), time.Now().UTC()); err != nil {
			return err
		}
		var tenant models.Tenant
		if err := repositories.FindTenantByID(tctx, tx, r.TenantID, &tenant); err == nil && tenant.Phone != "" {
			tid := r.TenantID
			_ = EnqueueNotification(tctx, tx, &tid, "subscription.refund_paid", map[string]any{
				"channel":    "whatsapp",
				"to":         tenant.Phone,
				"nama_usaha": tenant.BusinessName,
				"jumlah":     helpers.FormatRupiah(r.Amount),
				"rekening":   r.DestinationBank + " " + r.DestinationAccount,
				"referensi":  r.PayoutReference,
			})
		}
		out = refundToResponse(r)
		return nil
	})
	if err != nil {
		return out, err
	}
	tabel := "subscription_refunds"
	tid := r0.TenantID
	if err := repositories.WriteAuditLogs(ctx, []repositories.AuditEntry{{
		TenantID: &tid, ActorType: "admin", ActorID: &adminID,
		Action: "subscription.refund.paid", TargetTable: &tabel, TargetID: &refundID,
	}}); err != nil {
		helpers.LoggerFromContext(ctx).Error("mencatat audit pengembalian dana", slog.Any("error", err))
	}
	return out, nil
}

func refundToResponse(r models.SubscriptionRefund) structs.SubscriptionRefundResponse {
	out := structs.SubscriptionRefundResponse{
		ID: r.ID, Amount: r.Amount, MonthsUsed: r.MonthsUsed, Status: r.Status,
		DestinationBank: r.DestinationBank, DestinationAccount: r.DestinationAccount,
		DestinationHolder: r.DestinationHolder, PayoutReference: r.PayoutReference,
		CreatedAt: r.CreatedAt.UTC().Format(saleTimeLayout),
	}
	if r.PaidAt != nil {
		out.PaidAt = r.PaidAt.UTC().Format(saleTimeLayout)
	}
	return out
}
