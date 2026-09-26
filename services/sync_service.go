package services

import (
	"context"

	"candra/backend-api/config"
	"candra/backend-api/helpers"
	"candra/backend-api/internal/reqctx"
	"candra/backend-api/repositories"
	"candra/backend-api/structs"

	"github.com/gin-gonic/gin/binding"
)

// Layanan sinkronisasi offline (Fase 6, §10).
//
// PUSH (klien → server): batch operasi yang hanya menambah (penjualan). Tiap
// operasi idempoten lewat ULID + Idempotency-Key; satu operasi buruk TIDAK
// menggagalkan yang lain (§10 aturan 1). business_date selalu dihitung ULANG di
// server dari zona outlet — jam perangkat tidak dipercaya (§10 aturan 3).
//
// PULL (server → klien): master data + saldo stok + batu nisan, dipandu kursor
// `sync_version`.

// syncPullDefaultLimit / syncPullMaxLimit membatasi ukuran halaman pull.
func syncPullDefaultLimit() int { return config.GetIntEnv("SYNC_PULL_DEFAULT_LIMIT", 500) }
func syncPullMaxLimit() int     { return config.GetIntEnv("SYNC_PULL_MAX_LIMIT", 1000) }

// SyncCursorSafetyLag adalah saran jeda aman (jumlah nomor `sync_version`) yang
// SEBAIKNYA dikurangkan klien dari kursor sebelum menyimpannya — menutup lubang
// transaksi bernomor kecil yang commit terlambat (§10). Disertakan di response
// pull agar klien tak perlu menebak.
func SyncCursorSafetyLag() int64 { return int64(config.GetIntEnv("SYNC_CURSOR_SAFETY_LAG", 1000)) }

// SyncPullResult adalah PullChanges repositori + saran jeda aman untuk klien.
type SyncPullResult struct {
	repositories.PullChanges
	SafetyLag int64 `json:"safety_lag"`
}

// SyncPull mengembalikan satu halaman perubahan sejak kursor `since`. limit ≤ 0
// memakai default; limit di atas batas dipangkas.
func SyncPull(ctx context.Context, outletID string, since int64, limit int) (SyncPullResult, error) {
	if limit <= 0 {
		limit = syncPullDefaultLimit()
	}
	if max := syncPullMaxLimit(); limit > max {
		limit = max
	}
	if since < 0 {
		since = 0
	}
	changes, err := repositories.GetPullChanges(ctx, outletID, since, limit)
	if err != nil {
		return SyncPullResult{}, err
	}
	return SyncPullResult{PullChanges: changes, SafetyLag: SyncCursorSafetyLag()}, nil
}

// SyncPush menerapkan satu batch operasi. Selalu mengembalikan ringkasan
// (HTTP 200); hanya kesalahan internal (≥ 500) yang membatalkan seluruh batch
// agar klien mengulang — pengulangan aman karena tiap operasi idempoten.
func SyncPush(ctx context.Context, req structs.SyncPushRequest) (structs.SyncPushResponse, error) {
	out := structs.SyncPushResponse{
		DeviceID: req.DeviceID,
		Results:  make([]structs.SyncOpResult, 0, len(req.Operations)),
	}
	for _, op := range req.Operations {
		res, fatal := applySyncOp(ctx, op)
		if fatal != nil {
			return out, fatal
		}
		out.Results = append(out.Results, res)
		switch res.Status {
		case "applied":
			out.Applied++
		case "duplicate":
			out.Duplicate++
		default:
			out.Rejected++
		}
	}
	return out, nil
}

// applySyncOp menjalankan satu operasi. Mengembalikan (hasil, error fatal).
// error fatal (≥ 500, mis. gangguan DB) membatalkan batch; kesalahan biasa
// (validasi/konflik/tak ditemukan) menjadi hasil "rejected".
func applySyncOp(ctx context.Context, op structs.SyncOperation) (structs.SyncOpResult, error) {
	reject := func(reason string) structs.SyncOpResult {
		return structs.SyncOpResult{ID: op.ID, Status: "rejected", Reason: reason}
	}

	// Rute /sync dibuka untuk siapa pun yang boleh berjualan ATAU mencatat
	// kunjungan, jadi izin WAJIB diperiksa lagi per operasi — sama dengan izin
	// endpoint HTTP padanannya (POST /sales, POST /visits). Tanpa ini kasir
	// bisa menulis kunjungan sales lewat pintu belakang ini.
	if perm, ok := syncOpPermission[op.Op]; ok && !reqctx.HasPermission(ctx, perm) {
		return reject("Anda tidak punya izin untuk operasi " + op.Op), nil
	}

	switch op.Op {
	case "sale.create":
		var payload structs.CheckoutRequest
		if err := binding.JSON.BindBody(op.Payload, &payload); err != nil {
			return reject("payload tidak valid: " + err.Error()), nil
		}
		if payload.ID == "" {
			payload.ID = op.ID // ULID operasi menjadi ULID transaksi (mode offline)
		}
		idemKey := op.IdempotencyKey
		if idemKey == "" {
			idemKey = op.ID
		}
		in, err := BuildCheckoutInput(payload, idemKey, helpers.SHA256Hex(op.Payload))
		if err != nil {
			return reject(cleanReason(err)), nil
		}

		_, _, replayed, err := Checkout(ctx, in)
		switch {
		case err != nil && helpers.StatusForError(err) >= 500:
			return structs.SyncOpResult{}, err // fatal
		case err != nil:
			return reject(cleanReason(err)), nil
		case replayed:
			return structs.SyncOpResult{ID: op.ID, Status: "duplicate"}, nil
		default:
			return structs.SyncOpResult{ID: op.ID, Status: "applied"}, nil
		}

	case "visit.upsert":
		var payload structs.VisitUpsertRequest
		if err := binding.JSON.BindBody(op.Payload, &payload); err != nil {
			return reject("payload tidak valid: " + err.Error()), nil
		}
		if payload.ID == "" {
			payload.ID = op.ID // ULID operasi = ULID kunjungan
		}
		_, err := UpsertVisit(ctx, VisitUpsertInput{
			ID: payload.ID, VisitPlanID: payload.VisitPlanID, CustomerID: payload.CustomerID,
			CheckinAt: payload.CheckinAt, CheckoutAt: payload.CheckoutAt,
			CheckinLat: payload.CheckinLat, CheckinLng: payload.CheckinLng,
			PhotoURL: payload.PhotoURL, Result: payload.Result,
			NoOrderReason: payload.NoOrderReason, SaleID: payload.SaleID,
		})
		switch {
		case err != nil && helpers.StatusForError(err) >= 500:
			return structs.SyncOpResult{}, err // fatal
		case err != nil:
			return reject(cleanReason(err)), nil
		default:
			// Upsert idempoten: kirim ulang kunjungan yang sama = satu baris.
			return structs.SyncOpResult{ID: op.ID, Status: "applied"}, nil
		}

	default:
		return reject("operasi tidak dikenal: " + op.Op), nil
	}
}

// syncOpPermission memetakan jenis operasi /sync/push ke izin yang sama dengan
// endpoint HTTP padanannya.
var syncOpPermission = map[string]string{
	"sale.create":  "sale.create",
	"visit.upsert": "crm.visit.checkin",
}

// cleanReason menampilkan pesan error yang aman untuk klien.
func cleanReason(err error) string {
	if helpers.StatusForError(err) >= 500 {
		return "kesalahan internal"
	}
	return err.Error()
}
