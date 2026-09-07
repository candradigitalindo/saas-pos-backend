package reqctx

import (
	"context"
	"testing"
)

func TestTenantAndUserIDRoundTrip(t *testing.T) {
	ctx := context.Background()
	if TenantID(ctx) != "" || UserID(ctx) != "" {
		t.Fatal("context kosong seharusnya mengembalikan string kosong")
	}

	ctx = WithTenantID(ctx, "01TENANT")
	ctx = WithUserID(ctx, "01USER")
	if got := TenantID(ctx); got != "01TENANT" {
		t.Fatalf("TenantID = %q", got)
	}
	if got := UserID(ctx); got != "01USER" {
		t.Fatalf("UserID = %q", got)
	}
}

func TestPermissions(t *testing.T) {
	ctx := WithPermissions(context.Background(), []string{"sale.create", "sale.void"})

	if !HasPermission(ctx, "sale.create") {
		t.Fatal("sale.create seharusnya ada")
	}
	if HasPermission(ctx, "user.manage") {
		t.Fatal("user.manage seharusnya tidak ada")
	}
	if !HasAnyPermission(ctx, "user.manage", "sale.void") {
		t.Fatal("HasAnyPermission harus true bila salah satu cocok")
	}
	if HasAnyPermission(ctx, "user.manage", "role.manage") {
		t.Fatal("HasAnyPermission harus false bila tidak ada yang cocok")
	}
	if !HasAnyPermission(ctx) {
		t.Fatal("HasAnyPermission tanpa argumen = tidak ada syarat = true")
	}
	if len(Permissions(ctx)) != 2 {
		t.Fatalf("Permissions() = %v, mau 2 entri", Permissions(ctx))
	}
}

// TestPermissionsUnset memastikan pembacaan aman saat himpunan belum di-set.
func TestPermissionsUnset(t *testing.T) {
	ctx := context.Background()
	if HasPermission(ctx, "anything") {
		t.Fatal("tanpa set, HasPermission harus false")
	}
	if HasAnyPermission(ctx, "a", "b") {
		t.Fatal("tanpa set, HasAnyPermission(codes) harus false")
	}
	if len(Permissions(ctx)) != 0 {
		t.Fatal("tanpa set, Permissions harus kosong")
	}
}

// TestWithPermissionsCopies memastikan slice sumber yang diubah setelahnya tidak
// mempengaruhi context.
func TestWithPermissionsCopies(t *testing.T) {
	src := []string{"sale.create"}
	ctx := WithPermissions(context.Background(), src)
	src[0] = "diubah"
	if !HasPermission(ctx, "sale.create") {
		t.Fatal("context seharusnya menyimpan salinan, bukan referensi slice")
	}
}
