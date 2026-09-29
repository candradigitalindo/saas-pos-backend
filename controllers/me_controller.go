package controllers

import (
	"net/http"

	"candra/backend-api/internal/reqctx"
	"candra/backend-api/middlewares"
	"candra/backend-api/models"
	"candra/backend-api/repositories"
	"candra/backend-api/services"
	"candra/backend-api/structs"

	"github.com/gin-gonic/gin"
)

// Me mengembalikan profil user permintaan: identitas, usaha, peran, seluruh kode
// permission efektif, dan id outlet yang boleh diakses. Klien memakainya untuk
// menyusun menu & menyembunyikan aksi yang tak berizin.
//
// Semua data sudah dimuat middleware TenantScope (user & permission ada di
// context); hanya tenant & daftar outlet yang perlu satu query tambahan.
func Me(c *gin.Context) {
	ctx := c.Request.Context()

	v, exists := c.Get(middlewares.CurrentUserKey)
	user, ok := v.(models.User)
	if !exists || !ok {
		internalError(c)
		return
	}

	var tenant models.Tenant
	if err := repositories.FindTenantByID(ctx, nil, user.TenantID, &tenant); err != nil {
		respondServiceError(c, err)
		return
	}

	// Pemegang outlet.manage bekerja di SEMUA cabang — termasuk cabang yang
	// baru dibuat, yang tidak pernah punya baris user_outlets. Dulu daftar ini
	// selalu dari user_outlets saja, sehingga pemilik tidak bisa berpindah ke
	// cabang barunya di aplikasi ("Pakai toko ini" tidak berefek).
	var (
		outletIDs []string
		err       error
	)
	if reqctx.HasPermission(ctx, "outlet.manage") {
		outletIDs, err = repositories.ActiveOutletIDs(ctx, nil)
	} else {
		outletIDs, err = repositories.OutletIDsForUser(ctx, user.ID)
	}
	if err != nil {
		respondServiceError(c, err)
		return
	}
	if outletIDs == nil {
		outletIDs = []string{}
	}
	rincian, err := repositories.OutletsByIDs(ctx, nil, outletIDs)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	outlets := make([]structs.OutletResponse, len(rincian))
	for i, o := range rincian {
		outlets[i] = outletToResponse(o)
	}

	perms := reqctx.Permissions(ctx)
	if perms == nil {
		perms = []string{}
	}

	hak, err := services.CurrentEntitlement(ctx)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	paket := structs.PlanEntitlementResponse{
		Code: hak.PlanCode, Name: hak.PlanName, Status: hak.Status,
		Features: hak.Features, MaxOutlets: hak.OutletLimit(),
		MaxUsers: hak.MaxUsers, MaxProducts: hak.MaxProducts,
		UpgradeFor: services.UpgradeFor(ctx, hak),
	}
	if hak.BerlakuSampai != nil {
		paket.ActiveUntil = hak.BerlakuSampai.UTC().Format(timeLayout)
	}

	c.JSON(http.StatusOK, structs.SuccessResponse[structs.MeResponse]{
		Success: true,
		Message: "Profil pengguna",
		Data: structs.MeResponse{
			User:        userToResponse(user),
			Tenant:      tenantToResponse(tenant),
			Permissions: perms,
			OutletIDs:   outletIDs,
			Outlets:     outlets,
			Plan:        paket,
		},
	})
}
