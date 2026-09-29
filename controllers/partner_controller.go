package controllers

import (
	"net/http"

	"candra/backend-api/internal/reqctx"

	"candra/backend-api/services"
	"candra/backend-api/structs"

	"github.com/gin-gonic/gin"
)

// Handler portal mitra — realm auth TERPISAH (Fase 12, blueprint G.4/G.8).
// Semua handler di bawah PartnerAuth kecuali PartnerLogin.

// PartnerLogin menukar kredensial akun mitra dengan access token realm "partner".
func PartnerLogin(c *gin.Context) {
	var req structs.PartnerLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	res, err := services.PartnerLogin(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.PartnerAuthResponse]{
		Success: true, Message: "Berhasil masuk", Data: res,
	})
}

// PartnerMe mengembalikan profil mitra permintaan ini.
func PartnerMe(c *gin.Context) {
	res, err := services.PartnerProfile(c.Request.Context())
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.PartnerMeResponse]{
		Success: true, Message: "Profil mitra", Data: res,
	})
}

// PartnerDashboard merangkum prospek, merchant aktif, komisi, pencairan terakhir.
func PartnerDashboard(c *gin.Context) {
	res, err := services.PartnerDashboard(c.Request.Context())
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.PartnerDashboardResponse]{
		Success: true, Message: "Dashboard mitra", Data: res,
	})
}

// PartnerCreateLead mendaftarkan prospek baru milik mitra permintaan ini.
func PartnerCreateLead(c *gin.Context) {
	var req structs.PartnerLeadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	res, err := services.CreatePartnerLead(c.Request.Context(), req)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, structs.SuccessResponse[structs.PartnerLeadResponse]{
		Success: true, Message: "Prospek dicatat", Data: res,
	})
}

// PartnerListLeads mengembalikan prospek milik mitra permintaan ini.
func PartnerListLeads(c *gin.Context) {
	res, err := services.ListPartnerLeads(c.Request.Context(), c.Query("status"))
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[[]structs.PartnerLeadResponse]{
		Success: true, Message: "Daftar prospek", Data: res,
	})
}

// PartnerListMerchants mengembalikan merchant binaan — TAMPILAN TERBATAS
// (blueprint G.8). Setiap panggilan tercatat di jejak audit.
func PartnerListMerchants(c *gin.Context) {
	res, err := services.ListPartnerMerchants(c.Request.Context())
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[[]structs.PartnerMerchantResponse]{
		Success: true, Message: "Merchant binaan", Data: res,
	})
}

// PartnerListCommissions mengembalikan komisi milik mitra permintaan ini.
func PartnerListCommissions(c *gin.Context) {
	res, err := services.ListPartnerCommissions(c.Request.Context(), c.Query("status"), c.Query("from"), c.Query("to"))
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[[]structs.PartnerCommissionResponse]{
		Success: true, Message: "Rincian komisi", Data: res,
	})
}

// PartnerListPayouts mengembalikan riwayat pencairan mitra permintaan ini.
func PartnerListPayouts(c *gin.Context) {
	res, err := services.ListPartnerPayouts(c.Request.Context())
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[[]structs.PartnerPayoutResponse]{
		Success: true, Message: "Riwayat pencairan", Data: res,
	})
}

// ── Pelengkap portal mitra: materi, pelatihan, target, sengketa ─────────

// PartnerListMaterials mengembalikan materi jualan yang boleh dilihat tingkat
// mitra ini (blueprint G.4 — mencegah mitra mengarang janji fitur sendiri).
func PartnerListMaterials(c *gin.Context) {
	res, err := services.ListMaterialsForCurrentPartner(c.Request.Context())
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[[]structs.PartnerMaterialResponse]{
		Success: true, Message: "Materi jualan", Data: res,
	})
}

// PartnerListTrainings mengembalikan modul pelatihan + status penyelesaiannya.
func PartnerListTrainings(c *gin.Context) {
	res, err := services.ListTrainingsForCurrentPartnerUser(c.Request.Context())
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[[]structs.PartnerTrainingResponse]{
		Success: true, Message: "Modul pelatihan", Data: res,
	})
}

func PartnerCompleteTraining(c *gin.Context) {
	var req structs.PartnerTrainingCompleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	if err := services.CompleteTraining(c.Request.Context(), c.Param("id"), req.Score); err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[any]{
		Success: true, Message: "Pelatihan ditandai selesai", Data: nil,
	})
}

// PartnerListTargets mengembalikan target mitra ini; pencapaian dihitung dari
// merchant yang langganannya masih hidup, bukan jumlah pendaftaran.
func PartnerListTargets(c *gin.Context) {
	res, err := services.ListPartnerTargets(c.Request.Context(), reqctx.PartnerID(c.Request.Context()))
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[[]structs.PartnerTargetResponse]{
		Success: true, Message: "Target & pencapaian", Data: res,
	})
}

func PartnerListDisputes(c *gin.Context) {
	res, err := services.ListDisputes(c.Request.Context(), reqctx.PartnerID(c.Request.Context()), c.Query("status"))
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[[]structs.PartnerDisputeResponse]{
		Success: true, Message: "Sengketa atribusi", Data: res,
	})
}

func PartnerCreateDispute(c *gin.Context) {
	var req structs.PartnerDisputeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	res, err := services.CreatePartnerDispute(c.Request.Context(), req)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, structs.SuccessResponse[structs.PartnerDisputeResponse]{
		Success: true, Message: "Sengketa diajukan — menunggu keputusan admin", Data: res,
	})
}
