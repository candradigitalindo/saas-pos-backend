package services

import (
	"context"
	"fmt"

	"candra/backend-api/helpers"
	"candra/backend-api/internal/reqctx"
	"candra/backend-api/repositories"
)

// permOutletManage adalah izin yang membebaskan pemegangnya dari batas outlet:
// ia mengelola SEMUA cabang tenant (pemilik, manajer area).
const permOutletManage = "outlet.manage"

// ensureOutletAccess menolak (ErrForbidden) bila user permintaan tidak boleh
// bekerja di outlet `outletID` (§9: "untuk sumber daya per outlet, periksa
// user_outlets"; §13.1 langkah 0: "validasi hak akses outlet").
//
// Aturannya:
//   - pemegang outlet.manage → selalu boleh;
//   - selain itu → harus punya baris user_outlets untuk outlet itu.
//
// Konteks tanpa user (pekerja latar yang memanggil layanan langsung) tidak
// diperiksa — pagar ini untuk ORANG yang masuk lewat HTTP, dan setiap
// permintaan HTTP bertenant selalu membawa user dari middleware TenantScope.
//
// Kenapa di layanan, bukan middleware RequireOutletAccess: sebagian besar
// pintu tulis membawa outlet_id di BADAN JSON (checkout, buka shift, kas) atau
// hanya lewat id dokumen (void, tutup shift), dan /sync/push membawa banyak
// outlet dalam satu permintaan. Hanya layanan yang tahu outlet mana yang
// sebenarnya disentuh.
func ensureOutletAccess(ctx context.Context, outletID string) error {
	if reqctx.HasPermission(ctx, permOutletManage) {
		return nil
	}
	uid := reqctx.UserID(ctx)
	if uid == "" {
		return nil
	}
	// Jalur biasa (HTTP): batas cabang sudah dimuat middleware TenantScope.
	if _, terbatas := reqctx.OutletScope(ctx); terbatas {
		if !reqctx.OutletAllowed(ctx, outletID) {
			return errBukanCabangAnda
		}
		return nil
	}
	// Konteks tanpa batas termuat (layanan dipanggil langsung): tanya database.
	ok, err := repositories.UserCanAccessOutlet(ctx, uid, outletID)
	if err != nil {
		return err
	}
	if !ok {
		return errBukanCabangAnda
	}
	return nil
}

// errBukanCabangAnda dikembalikan pagar akses cabang.
var errBukanCabangAnda = fmt.Errorf("%w: Anda tidak punya akses ke outlet ini", helpers.ErrForbidden)
