package controllers

import (
	"net/http"
	"strconv"

	"candra/backend-api/services"
	"candra/backend-api/structs"

	"github.com/gin-gonic/gin"
)

// Handler sinkronisasi offline (Fase 6, §10). Kedua endpoint dijaga permission
// `sale.create` — perangkat yang berjualan adalah perangkat yang menyinkron.

// SyncPush: POST /api/v1/sync/push
// Menerima batch operasi dari perangkat offline. Selalu 200; hasil per operasi
// (applied / duplicate / rejected) ada di body — satu operasi buruk tidak
// menggagalkan yang lain (§10).
func SyncPush(c *gin.Context) {
	var req structs.SyncPushRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationFailed(c, err)
		return
	}
	res, err := services.SyncPush(c.Request.Context(), req)
	if err != nil {
		respondServiceError(c, err) // hanya kesalahan internal sampai sini
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[structs.SyncPushResponse]{
		Success: true, Message: "Sinkronisasi diterima", Data: res,
	})
}

// SyncPull: GET /api/v1/sync/pull?since=&outlet_id=&limit=
// Mengembalikan master data + snapshot stok + batu nisan yang lebih baru dari
// kursor `since`. Klien menyimpan `cursor` DIKURANGI `safety_lag`.
func SyncPull(c *gin.Context) {
	since, _ := strconv.ParseInt(c.Query("since"), 10, 64)
	limit, _ := strconv.Atoi(c.Query("limit"))

	res, err := services.SyncPull(c.Request.Context(), c.Query("outlet_id"), since, limit)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, structs.SuccessResponse[services.SyncPullResult]{
		Success: true, Message: "Perubahan sinkronisasi", Data: res,
	})
}
