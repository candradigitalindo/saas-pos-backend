package controllers

import (
	"net/http"

	"candra/backend-api/internal/reqctx"
	"candra/backend-api/middlewares"
	"candra/backend-api/models"
	"candra/backend-api/repositories"
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

	outletIDs, err := repositories.OutletIDsForUser(ctx, user.ID)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	if outletIDs == nil {
		outletIDs = []string{}
	}

	perms := reqctx.Permissions(ctx)
	if perms == nil {
		perms = []string{}
	}

	c.JSON(http.StatusOK, structs.SuccessResponse[structs.MeResponse]{
		Success: true,
		Message: "Profil pengguna",
		Data: structs.MeResponse{
			User:        userToResponse(user),
			Tenant:      tenantToResponse(tenant),
			Permissions: perms,
			OutletIDs:   outletIDs,
		},
	})
}
