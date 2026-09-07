package services

import (
	"context"
	"fmt"
	"time"

	"candra/backend-api/helpers"
	"candra/backend-api/internal/reqctx"
	"candra/backend-api/models"
	"candra/backend-api/repositories"
	"candra/backend-api/structs"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// Layanan CRM inti — pipeline, deal, aktivitas (Fase 9, §5.9, blueprint E.1).
//
// owner_id di-stempel dari user konteks saat membuat; visibilitas (lapis 3)
// dijaga di repository lewat scopeVisibility.

// defaultPipelineStages adalah tahap bawaan pipeline "Umum" (dibuat malas saat
// deal pertama). Tenant boleh menggantinya lewat POST /pipelines.
var defaultPipelineStages = []struct {
	Name        string
	Probability string
	IsWon       bool
	IsLost      bool
}{
	{"Baru", "0.1", false, false},
	{"Kontak", "0.25", false, false},
	{"Penawaran", "0.5", false, false},
	{"Negosiasi", "0.75", false, false},
	{"Menang", "1", true, false},
	{"Kalah", "0", false, true},
}

var defaultLeadSources = []string{"WhatsApp", "Instagram", "Marketplace", "Referral", "Walk-in"}

// ensureCRMDefaults membuat pipeline "Umum" + tahap + sumber prospek bila tenant
// belum punya pipeline. Dipanggil di dalam transaksi.
func ensureCRMDefaults(ctx context.Context, tx *gorm.DB) (models.Pipeline, error) {
	if n, err := repositories.CountPipelines(ctx, tx); err != nil {
		return models.Pipeline{}, err
	} else if n > 0 {
		p, _, err := repositories.DefaultPipeline(ctx, tx)
		return p, err
	}

	p := models.Pipeline{Name: "Umum", Kind: "general", IsDefault: true}
	if err := repositories.CreatePipeline(ctx, tx, &p); err != nil {
		return p, err
	}
	stages := make([]models.PipelineStage, 0, len(defaultPipelineStages))
	for i, s := range defaultPipelineStages {
		prob, _ := decimal.NewFromString(s.Probability)
		stages = append(stages, models.PipelineStage{
			PipelineID: p.ID, Name: s.Name, SortOrder: i + 1,
			Probability: prob.InexactFloat64(), IsWon: s.IsWon, IsLost: s.IsLost,
		})
	}
	if err := repositories.CreatePipelineStages(ctx, tx, stages); err != nil {
		return p, err
	}
	for _, name := range defaultLeadSources {
		ls := models.LeadSource{Name: name, IsActive: true}
		if err := repositories.CreateLeadSource(ctx, tx, &ls); err != nil {
			return p, err
		}
	}
	p.Stages = stages
	return p, nil
}

// ── Lead source ───────────────────────────────────────────────────────────

// ListLeadSources mengembalikan sumber prospek tenant.
func ListLeadSources(ctx context.Context) ([]structs.LeadSourceResponse, error) {
	rows, err := repositories.ListLeadSources(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]structs.LeadSourceResponse, len(rows))
	for i, r := range rows {
		out[i] = structs.LeadSourceResponse{ID: r.ID, Name: r.Name, IsActive: r.IsActive}
	}
	return out, nil
}

// CreateLeadSource menambah sumber prospek.
func CreateLeadSource(ctx context.Context, name string) (structs.LeadSourceResponse, error) {
	var out structs.LeadSourceResponse
	err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		ls := models.LeadSource{Name: name, IsActive: true}
		if err := repositories.CreateLeadSource(ctx, tx, &ls); err != nil {
			return err
		}
		out = structs.LeadSourceResponse{ID: ls.ID, Name: ls.Name, IsActive: ls.IsActive}
		return nil
	})
	return out, err
}

// ── Pipeline ──────────────────────────────────────────────────────────────

// ListPipelines mengembalikan pipeline tenant; membuat pipeline default bila
// belum ada (agar klien selalu punya tahap untuk dipilih).
func ListPipelines(ctx context.Context) ([]structs.PipelineResponse, error) {
	if err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		_, err := ensureCRMDefaults(ctx, tx)
		return err
	}); err != nil {
		return nil, err
	}
	rows, err := repositories.ListPipelines(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]structs.PipelineResponse, len(rows))
	for i := range rows {
		out[i] = pipelineToResponse(rows[i])
	}
	return out, nil
}

// CreatePipeline menyimpan pipeline kustom + tahap-tahapnya.
func CreatePipeline(ctx context.Context, in structs.PipelineRequest) (structs.PipelineResponse, error) {
	var out structs.PipelineResponse
	err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		p := models.Pipeline{Name: in.Name, Kind: in.Kind}
		if err := repositories.CreatePipeline(ctx, tx, &p); err != nil {
			return err
		}
		stages := make([]models.PipelineStage, 0, len(in.Stages))
		for i, s := range in.Stages {
			prob := decimal.Zero
			if s.Probability != "" {
				var perr error
				if prob, perr = decimal.NewFromString(s.Probability); perr != nil {
					return fmt.Errorf("%w: probability bukan angka", helpers.ErrValidation)
				}
			}
			stages = append(stages, models.PipelineStage{
				PipelineID: p.ID, Name: s.Name, SortOrder: i + 1,
				Probability: prob.InexactFloat64(), IsWon: s.IsWon, IsLost: s.IsLost,
			})
		}
		if err := repositories.CreatePipelineStages(ctx, tx, stages); err != nil {
			return err
		}
		p.Stages = stages
		out = pipelineToResponse(p)
		return nil
	})
	return out, err
}

// ── Deal ──────────────────────────────────────────────────────────────────

// CreateDeal membuat deal baru. pipeline_id/stage_id kosong → pakai pipeline
// default & tahap pertamanya.
func CreateDeal(ctx context.Context, in structs.DealCreateRequest) (structs.DealResponse, error) {
	var out structs.DealResponse
	err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		var pipeline models.Pipeline
		var err error
		if in.PipelineID != "" {
			pipeline, err = repositories.FindPipeline(ctx, tx, in.PipelineID)
		} else {
			pipeline, err = ensureCRMDefaults(ctx, tx)
		}
		if err != nil {
			return err
		}
		if len(pipeline.Stages) == 0 {
			return fmt.Errorf("%w: pipeline belum punya tahap", helpers.ErrValidation)
		}

		stageID := pipeline.Stages[0].ID
		if in.StageID != "" {
			s, serr := repositories.FindStage(ctx, tx, pipeline.ID, in.StageID)
			if serr != nil {
				return fmt.Errorf("%w: tahap tidak cocok dengan pipeline", helpers.ErrValidation)
			}
			stageID = s.ID
		}

		d := models.Deal{
			PipelineID: pipeline.ID,
			StageID:    stageID,
			OwnerID:    reqctx.UserID(ctx),
			Title:      in.Title,
			Value:      in.Value,
			Status:     "open",
		}
		if in.CustomerID != "" {
			d.CustomerID = &in.CustomerID
		}
		if in.LeadSourceID != "" {
			d.LeadSourceID = &in.LeadSourceID
		}
		if in.ExpectedCloseDate != "" {
			t, derr := time.Parse("2006-01-02", in.ExpectedCloseDate)
			if derr != nil {
				return fmt.Errorf("%w: expected_close_date harus YYYY-MM-DD", helpers.ErrValidation)
			}
			d.ExpectedCloseDate = &t
		}
		if err := repositories.CreateDeal(ctx, tx, &d); err != nil {
			return err
		}
		out = dealToResponse(d)
		return nil
	})
	return out, err
}

// UpdateDeal mengubah field deal (termasuk pindah tahap).
func UpdateDeal(ctx context.Context, id string, in structs.DealUpdateRequest) (structs.DealResponse, error) {
	var out structs.DealResponse
	err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		d, err := repositories.FindDeal(ctx, tx, id)
		if err != nil {
			return err
		}
		if d.Status != "open" {
			return fmt.Errorf("%w: deal yang sudah ditutup tidak bisa diubah", helpers.ErrConflict)
		}
		if in.StageID != nil {
			s, serr := repositories.FindStage(ctx, tx, d.PipelineID, *in.StageID)
			if serr != nil {
				return fmt.Errorf("%w: tahap tidak cocok dengan pipeline", helpers.ErrValidation)
			}
			d.StageID = s.ID
		}
		if in.CustomerID != nil {
			d.CustomerID = in.CustomerID
		}
		if in.LeadSourceID != nil {
			d.LeadSourceID = in.LeadSourceID
		}
		if in.Title != nil {
			d.Title = *in.Title
		}
		if in.Value != nil {
			d.Value = *in.Value
		}
		if in.ExpectedCloseDate != nil {
			t, derr := time.Parse("2006-01-02", *in.ExpectedCloseDate)
			if derr != nil {
				return fmt.Errorf("%w: expected_close_date harus YYYY-MM-DD", helpers.ErrValidation)
			}
			d.ExpectedCloseDate = &t
		}
		if err := repositories.SaveDeal(ctx, tx, &d); err != nil {
			return err
		}
		reloaded, err := repositories.FindDeal(ctx, tx, id)
		if err != nil {
			return err
		}
		out = dealToResponse(reloaded)
		return nil
	})
	return out, err
}

// closeDeal menandai deal won/lost dan memindahkannya ke tahap menang/kalah bila ada.
func closeDeal(ctx context.Context, id, status, lostReason string) (structs.DealResponse, error) {
	var out structs.DealResponse
	err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		d, err := repositories.FindDeal(ctx, tx, id)
		if err != nil {
			return err
		}
		if d.Status != "open" {
			return fmt.Errorf("%w: deal sudah ditutup", helpers.ErrConflict)
		}
		pipeline, err := repositories.FindPipeline(ctx, tx, d.PipelineID)
		if err != nil {
			return err
		}
		for _, s := range pipeline.Stages {
			if (status == "won" && s.IsWon) || (status == "lost" && s.IsLost) {
				d.StageID = s.ID
				break
			}
		}
		now := time.Now().UTC()
		d.Status = status
		d.ClosedAt = &now
		d.LostReason = lostReason
		if err := repositories.SaveDeal(ctx, tx, &d); err != nil {
			return err
		}
		reloaded, err := repositories.FindDeal(ctx, tx, id)
		if err != nil {
			return err
		}
		out = dealToResponse(reloaded)
		return nil
	})
	return out, err
}

// WinDeal menutup deal sebagai menang.
func WinDeal(ctx context.Context, id string) (structs.DealResponse, error) {
	return closeDeal(ctx, id, "won", "")
}

// LoseDeal menutup deal sebagai kalah, mencatat alasannya.
func LoseDeal(ctx context.Context, id, reason string) (structs.DealResponse, error) {
	return closeDeal(ctx, id, "lost", reason)
}

// ── Activity ──────────────────────────────────────────────────────────────

// CreateActivity menjadwalkan aktivitas follow-up.
func CreateActivity(ctx context.Context, in structs.ActivityCreateRequest) (structs.ActivityResponse, error) {
	var out structs.ActivityResponse
	err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		a := models.Activity{
			Kind:    in.Kind,
			Subject: in.Subject,
			Body:    in.Body,
			OwnerID: reqctx.UserID(ctx),
			Status:  "planned",
		}
		if in.CustomerID != "" {
			a.CustomerID = &in.CustomerID
		}
		if in.DealID != "" {
			a.DealID = &in.DealID
		}
		if in.DueAt != "" {
			t, derr := time.Parse(time.RFC3339, in.DueAt)
			if derr != nil {
				return fmt.Errorf("%w: due_at harus RFC3339", helpers.ErrValidation)
			}
			a.DueAt = &t
		}
		if err := repositories.CreateActivity(ctx, tx, &a); err != nil {
			return err
		}
		out = activityToResponse(a)
		return nil
	})
	return out, err
}

// setActivityStatus menandai aktivitas selesai / batal.
func setActivityStatus(ctx context.Context, id, status string) (structs.ActivityResponse, error) {
	var out structs.ActivityResponse
	err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		a, err := repositories.FindActivity(ctx, tx, id)
		if err != nil {
			return err
		}
		if a.Status != "planned" {
			return fmt.Errorf("%w: aktivitas sudah tidak berstatus terjadwal", helpers.ErrConflict)
		}
		a.Status = status
		if status == "done" {
			now := time.Now().UTC()
			a.CompletedAt = &now
		}
		if err := repositories.SaveActivity(ctx, tx, &a); err != nil {
			return err
		}
		out = activityToResponse(a)
		return nil
	})
	return out, err
}

// CompleteActivity menandai aktivitas selesai.
func CompleteActivity(ctx context.Context, id string) (structs.ActivityResponse, error) {
	return setActivityStatus(ctx, id, "done")
}

// CancelActivity membatalkan aktivitas.
func CancelActivity(ctx context.Context, id string) (structs.ActivityResponse, error) {
	return setActivityStatus(ctx, id, "canceled")
}

// ── Pemetaan DTO ──────────────────────────────────────────────────────────

func pipelineToResponse(p models.Pipeline) structs.PipelineResponse {
	r := structs.PipelineResponse{
		ID: p.ID, Name: p.Name, Kind: p.Kind, IsDefault: p.IsDefault,
		Stages: make([]structs.PipelineStageResponse, len(p.Stages)),
	}
	for i, s := range p.Stages {
		r.Stages[i] = structs.PipelineStageResponse{
			ID: s.ID, Name: s.Name, SortOrder: s.SortOrder,
			Probability: decimal.NewFromFloat(s.Probability).String(),
			IsWon:       s.IsWon, IsLost: s.IsLost,
		}
	}
	return r
}

func dealToResponse(d models.Deal) structs.DealResponse {
	r := structs.DealResponse{
		ID: d.ID, PipelineID: d.PipelineID, StageID: d.StageID,
		OwnerID: d.OwnerID, Title: d.Title, Value: d.Value,
		Status: d.Status, LostReason: d.LostReason,
		CreatedAt: d.CreatedAt.Format(saleTimeLayout),
		UpdatedAt: d.UpdatedAt.Format(saleTimeLayout),
	}
	if d.CustomerID != nil {
		r.CustomerID = *d.CustomerID
	}
	if d.LeadSourceID != nil {
		r.LeadSourceID = *d.LeadSourceID
	}
	if d.ExpectedCloseDate != nil {
		r.ExpectedCloseDate = d.ExpectedCloseDate.Format("2006-01-02")
	}
	if d.ClosedAt != nil {
		r.ClosedAt = d.ClosedAt.Format(saleTimeLayout)
	}
	return r
}

func activityToResponse(a models.Activity) structs.ActivityResponse {
	r := structs.ActivityResponse{
		ID: a.ID, Kind: a.Kind, Subject: a.Subject, Body: a.Body,
		OwnerID: a.OwnerID, Status: a.Status,
		CreatedAt: a.CreatedAt.Format(saleTimeLayout),
	}
	if a.CustomerID != nil {
		r.CustomerID = *a.CustomerID
	}
	if a.DealID != nil {
		r.DealID = *a.DealID
	}
	if a.DueAt != nil {
		r.DueAt = a.DueAt.Format(saleTimeLayout)
	}
	if a.CompletedAt != nil {
		r.CompletedAt = a.CompletedAt.Format(saleTimeLayout)
	}
	return r
}

// DealToResponse / ActivityToResponse diekspor untuk controller list.
func DealToResponse(d models.Deal) structs.DealResponse             { return dealToResponse(d) }
func ActivityToResponse(a models.Activity) structs.ActivityResponse { return activityToResponse(a) }
