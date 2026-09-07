package structs

// DTO CRM inti — pipeline, deal, aktivitas (Fase 9, §5.9). Nilai uang int64
// rupiah bulat; qty & probabilitas string desimal.

// ── Lead source ───────────────────────────────────────────────────────────

type LeadSourceRequest struct {
	Name string `json:"name" binding:"required,min=1,max=80"`
}

type LeadSourceResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	IsActive bool   `json:"is_active"`
}

// ── Pipeline ──────────────────────────────────────────────────────────────

type PipelineStageRequest struct {
	Name        string `json:"name" binding:"required,min=1,max=60"`
	Probability string `json:"probability" binding:"omitempty"` // "0".."1"
	IsWon       bool   `json:"is_won"`
	IsLost      bool   `json:"is_lost"`
}

type PipelineRequest struct {
	Name   string                 `json:"name" binding:"required,min=1,max=80"`
	Kind   string                 `json:"kind" binding:"required,oneof=freelance field_sales general"`
	Stages []PipelineStageRequest `json:"stages" binding:"required,min=2,dive"`
}

type PipelineStageResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	SortOrder   int    `json:"sort_order"`
	Probability string `json:"probability"`
	IsWon       bool   `json:"is_won"`
	IsLost      bool   `json:"is_lost"`
}

type PipelineResponse struct {
	ID        string                  `json:"id"`
	Name      string                  `json:"name"`
	Kind      string                  `json:"kind"`
	IsDefault bool                    `json:"is_default"`
	Stages    []PipelineStageResponse `json:"stages"`
}

// ── Deal ──────────────────────────────────────────────────────────────────

type DealCreateRequest struct {
	PipelineID        string `json:"pipeline_id" binding:"omitempty,ulid"` // kosong → pipeline default
	StageID           string `json:"stage_id" binding:"omitempty,ulid"`    // kosong → tahap pertama
	CustomerID        string `json:"customer_id" binding:"omitempty,ulid"`
	LeadSourceID      string `json:"lead_source_id" binding:"omitempty,ulid"`
	Title             string `json:"title" binding:"required,min=1,max=200"`
	Value             int64  `json:"value" binding:"omitempty,gte=0"`
	ExpectedCloseDate string `json:"expected_close_date" binding:"omitempty"` // YYYY-MM-DD
}

type DealUpdateRequest struct {
	StageID           *string `json:"stage_id" binding:"omitempty,ulid"`
	CustomerID        *string `json:"customer_id" binding:"omitempty,ulid"`
	LeadSourceID      *string `json:"lead_source_id" binding:"omitempty,ulid"`
	Title             *string `json:"title" binding:"omitempty,min=1,max=200"`
	Value             *int64  `json:"value" binding:"omitempty,gte=0"`
	ExpectedCloseDate *string `json:"expected_close_date" binding:"omitempty"`
}

type DealLoseRequest struct {
	LostReason string `json:"lost_reason" binding:"required,min=1,max=255"`
}

type DealResponse struct {
	ID                string `json:"id"`
	PipelineID        string `json:"pipeline_id"`
	StageID           string `json:"stage_id"`
	CustomerID        string `json:"customer_id,omitempty"`
	LeadSourceID      string `json:"lead_source_id,omitempty"`
	OwnerID           string `json:"owner_id"`
	Title             string `json:"title"`
	Value             int64  `json:"value"`
	ExpectedCloseDate string `json:"expected_close_date,omitempty"`
	Status            string `json:"status"`
	LostReason        string `json:"lost_reason,omitempty"`
	ClosedAt          string `json:"closed_at,omitempty"`
	CreatedAt         string `json:"created_at"`
	UpdatedAt         string `json:"updated_at"`
}

// ── Activity ──────────────────────────────────────────────────────────────

type ActivityCreateRequest struct {
	Kind       string `json:"kind" binding:"required,oneof=call chat meeting visit task note"`
	Subject    string `json:"subject" binding:"required,min=1,max=200"`
	Body       string `json:"body" binding:"omitempty,max=2000"`
	CustomerID string `json:"customer_id" binding:"omitempty,ulid"`
	DealID     string `json:"deal_id" binding:"omitempty,ulid"`
	DueAt      string `json:"due_at" binding:"omitempty"` // RFC3339
}

type ActivityResponse struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	Subject     string `json:"subject"`
	Body        string `json:"body,omitempty"`
	OwnerID     string `json:"owner_id"`
	CustomerID  string `json:"customer_id,omitempty"`
	DealID      string `json:"deal_id,omitempty"`
	DueAt       string `json:"due_at,omitempty"`
	CompletedAt string `json:"completed_at,omitempty"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
}
