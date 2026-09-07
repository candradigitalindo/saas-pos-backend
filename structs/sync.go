package structs

import "encoding/json"

// DTO sinkronisasi offline (Fase 6, §10).

// ── Push: klien → server ───────────────────────────────────────────────────

// SyncOperation adalah satu operasi dalam kiriman batch. `payload` dibiarkan
// mentah dan diparse sesuai `op`.
type SyncOperation struct {
	Op             string          `json:"op" binding:"required"`               // mis. "sale.create"
	ID             string          `json:"id" binding:"required,ulid"`          // ULID entitas, dibuat klien
	IdempotencyKey string          `json:"idempotency_key" binding:"omitempty"` // kosong → pakai ID
	Payload        json.RawMessage `json:"payload" binding:"required"`
}

// SyncPushRequest adalah body POST /api/v1/sync/push.
type SyncPushRequest struct {
	DeviceID   string          `json:"device_id" binding:"required"`
	Operations []SyncOperation `json:"operations" binding:"required,min=1,dive"`
}

// SyncOpResult adalah hasil satu operasi. Status:
//   - "applied"   : diterapkan
//   - "duplicate" : sudah pernah diterapkan (idempoten) — bukan error
//   - "rejected"  : ditolak; `reason` menjelaskan (tidak menghentikan operasi lain)
type SyncOpResult struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

// SyncPushResponse merangkum hasil batch. Selalu HTTP 200 — kegagalan satu
// operasi tidak menggagalkan seluruh kiriman (§10 aturan 1).
type SyncPushResponse struct {
	DeviceID  string         `json:"device_id"`
	Applied   int            `json:"applied"`
	Duplicate int            `json:"duplicate"`
	Rejected  int            `json:"rejected"`
	Results   []SyncOpResult `json:"results"`
}
