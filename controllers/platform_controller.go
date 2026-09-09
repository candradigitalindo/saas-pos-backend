package controllers

import (
	"net/http"

	"candra/backend-api/helpers"
	"candra/backend-api/repositories"
	"candra/backend-api/services"
	"candra/backend-api/structs"

	"github.com/gin-gonic/gin"
)

// Handler panel internal penyedia SaaS (blueprint G.5) — realm ketiga.
// Semua handler di bawah PlatformAuth kecuali PlatformLogin.

func PlatformLogin(c *gin.Context) {
	var req structs.PlatformLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	res, err := services.PlatformLogin(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.PlatformAuthResponse]{
		Success: true, Message: "Berhasil masuk panel internal", Data: res,
	})
}

func PlatformMe(c *gin.Context) {
	res, err := services.PlatformProfile(c.Request.Context())
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.PlatformAdminResponse]{
		Success: true, Message: "Profil admin", Data: res,
	})
}

// ── Akun staf internal (superadmin) ─────────────────────────────────────

func PlatformListAdmins(c *gin.Context) {
	res, err := services.ListPlatformAdmins(c.Request.Context())
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[[]structs.PlatformAdminResponse]{
		Success: true, Message: "Daftar admin platform", Data: res,
	})
}

func PlatformCreateAdmin(c *gin.Context) {
	var req structs.PlatformAdminCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	res, err := services.CreatePlatformAdmin(c.Request.Context(), req)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, structs.SuccessResponse[structs.PlatformAdminCreateResponse]{
		Success: true, Message: "Admin dibuat", Data: res,
	})
}

func PlatformSetAdminActive(c *gin.Context) {
	var req structs.PlatformAdminActiveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	err := services.SetPlatformAdminActive(c.Request.Context(), c.Param("id"), *req.IsActive)
	if err != nil {
		notFoundOr(c, err, repositories.ErrPlatformAdminNotFound, "Admin tidak ditemukan")
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[any]{
		Success: true, Message: "Status admin diperbarui", Data: nil,
	})
}

// ── Mitra: verifikasi & tingkat ─────────────────────────────────────────

func PlatformListPartners(c *gin.Context) {
	page, limit, offset := helpers.ParsePaginationParams(c)
	rows, total, err := services.ListPartners(c.Request.Context(), c.Query("status"), limit, offset)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.PaginatedResponse[structs.PartnerResponse]]{
		Success: true, Message: "Daftar mitra",
		Data: helpers.BuildPaginationResponse(c, page, limit, total, rows),
	})
}

func PlatformCreatePartner(c *gin.Context) {
	var req structs.PartnerCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	res, err := services.CreatePartner(c.Request.Context(), req)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	services.AuditPlatformAction(c.Request.Context(), "partner.create", "partners", res.Partner.ID)
	c.JSON(http.StatusCreated, structs.SuccessResponse[structs.PartnerCreateResponse]{
		Success: true, Message: "Mitra dibuat — verifikasi identitas & rekening sebelum diaktifkan", Data: res,
	})
}

// PlatformApprovePartner memverifikasi & mengaktifkan mitra (blueprint G.6 #2/#4).
func PlatformApprovePartner(c *gin.Context) {
	id := c.Param("id")
	if err := services.ApprovePartner(c.Request.Context(), id); err != nil {
		notFoundOr(c, err, repositories.ErrPartnerNotFound, "Mitra tidak ditemukan")
		return
	}
	services.AuditPlatformAction(c.Request.Context(), "partner.approve", "partners", id)
	c.JSON(http.StatusOK, structs.SuccessResponse[any]{
		Success: true, Message: "Mitra disetujui & diaktifkan", Data: nil,
	})
}

func PlatformSuspendPartner(c *gin.Context) {
	id := c.Param("id")
	if err := services.SuspendPartner(c.Request.Context(), id); err != nil {
		notFoundOr(c, err, repositories.ErrPartnerNotFound, "Mitra tidak ditemukan")
		return
	}
	services.AuditPlatformAction(c.Request.Context(), "partner.suspend", "partners", id)
	c.JSON(http.StatusOK, structs.SuccessResponse[any]{
		Success: true, Message: "Mitra dinonaktifkan", Data: nil,
	})
}

func PlatformListTiers(c *gin.Context) {
	res, err := services.ListPartnerTiers(c.Request.Context())
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[[]structs.PartnerTierResponse]{
		Success: true, Message: "Tingkat mitra", Data: res,
	})
}

func PlatformCreateTier(c *gin.Context) {
	var req structs.PartnerTierRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	res, err := services.CreatePartnerTier(c.Request.Context(), req)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	services.AuditPlatformAction(c.Request.Context(), "partner.tier.create", "partner_tiers", res.ID)
	c.JSON(http.StatusCreated, structs.SuccessResponse[structs.PartnerTierResponse]{
		Success: true, Message: "Tingkat mitra dibuat", Data: res,
	})
}

// ── Komisi & pencairan (finance) ────────────────────────────────────────

// PlatformRunCommissions menjalankan siklus komisi dari panel — pengganti
// cmd/partner-commissions untuk pemakaian manual.
func PlatformRunCommissions(c *gin.Context) {
	var req structs.PlatformCommissionRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	ctx := c.Request.Context()

	// Satu mitra saja → hitung bertarget; tanpa partner_id → siklus penuh.
	if req.PartnerID != "" {
		run, err := services.ComputePartnerCommissions(ctx, req.PartnerID, req.PeriodStart, req.PeriodEnd)
		if err != nil {
			respondServiceError(c, err)
			return
		}
		services.AuditPlatformAction(ctx, "partner.commission.run", "partners", req.PartnerID)
		c.JSON(http.StatusOK, structs.SuccessResponse[structs.PartnerCommissionRunResult]{
			Success: true, Message: "Komisi dihitung", Data: run,
		})
		return
	}

	res, err := services.RunPartnerCommissionCycle(ctx, req.PeriodStart, req.PeriodEnd, req.Approve, req.Payout)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	services.AuditPlatformAction(ctx, "partner.commission.run", "partner_commissions", "")
	c.JSON(http.StatusOK, structs.SuccessResponse[services.PartnerCycleResult]{
		Success: true, Message: "Siklus komisi selesai", Data: res,
	})
}

func PlatformApproveCommission(c *gin.Context) {
	id := c.Param("id")
	if err := services.ApprovePartnerCommission(c.Request.Context(), id); err != nil {
		notFoundOr(c, err, repositories.ErrPartnerCommissionNotFound, "Komisi tidak ditemukan")
		return
	}
	services.AuditPlatformAction(c.Request.Context(), "partner.commission.approve", "partner_commissions", id)
	c.JSON(http.StatusOK, structs.SuccessResponse[any]{
		Success: true, Message: "Komisi disetujui", Data: nil,
	})
}

func PlatformCreatePayout(c *gin.Context) {
	var req structs.PlatformPayoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	res, err := services.CreatePartnerPayout(c.Request.Context(), req.PartnerID, req.PeriodStart, req.PeriodEnd)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	services.AuditPlatformAction(c.Request.Context(), "partner.payout.create", "partner_payouts", res.ID)
	c.JSON(http.StatusCreated, structs.SuccessResponse[structs.PartnerPayoutResponse]{
		Success: true, Message: "Pencairan dibuat (draf)", Data: res,
	})
}

func PlatformMarkPayoutPaid(c *gin.Context) {
	var req structs.PartnerPayoutMarkPaidRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	id := c.Param("id")
	err := services.MarkPartnerPayoutPaid(c.Request.Context(), id, req.TransferProofURL, req.TaxSlipURL)
	if err != nil {
		notFoundOr(c, err, repositories.ErrPartnerPayoutNotFound, "Pencairan tidak ditemukan atau sudah dibayar")
		return
	}
	services.AuditPlatformAction(c.Request.Context(), "partner.payout.paid", "partner_payouts", id)
	c.JSON(http.StatusOK, structs.SuccessResponse[any]{
		Success: true, Message: "Pencairan ditandai sudah dibayar", Data: nil,
	})
}
