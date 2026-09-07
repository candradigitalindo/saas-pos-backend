package repositories

import (
	"context"
	"errors"

	"candra/backend-api/models"

	"gorm.io/gorm"
)

// Repositori CRM tenant — pipeline, deal, aktivitas (§5.9, blueprint Bagian E).
//
// Tiga lapis isolasi: scopeTenant (lapis 1) selalu; scopeVisibility (lapis 3)
// pada tabel ber-owner_id (deals, activities, quotations, projects, invoices).

var (
	ErrDealNotFound     = errors.New("deal tidak ditemukan")
	ErrActivityNotFound = errors.New("aktivitas tidak ditemukan")
	ErrPipelineNotFound = errors.New("pipeline tidak ditemukan")
	ErrStageNotFound    = errors.New("tahap pipeline tidak ditemukan")
)

// ── Lead source ───────────────────────────────────────────────────────────

// ListLeadSources mengembalikan seluruh sumber prospek tenant.
func ListLeadSources(ctx context.Context) ([]models.LeadSource, error) {
	var rows []models.LeadSource
	err := scopeTenant(ctx, tenantDB(ctx, nil)).Order("name").Find(&rows).Error
	return rows, err
}

// CreateLeadSource menyimpan sumber prospek baru.
func CreateLeadSource(ctx context.Context, tx *gorm.DB, s *models.LeadSource) error {
	s.TenantID = currentTenantID(ctx)
	return tenantDB(ctx, tx).Create(s).Error
}

// FindLeadSource memuat satu sumber prospek milik tenant.
func FindLeadSource(ctx context.Context, tx *gorm.DB, id string) (models.LeadSource, error) {
	var s models.LeadSource
	err := scopeTenant(ctx, tenantDB(ctx, tx)).First(&s, "id = ?", id).Error
	return s, err
}

// ── Pipeline ──────────────────────────────────────────────────────────────

// ListPipelines mengembalikan pipeline tenant beserta tahap-tahapnya (urut).
func ListPipelines(ctx context.Context) ([]models.Pipeline, error) {
	var rows []models.Pipeline
	err := scopeTenant(ctx, tenantDB(ctx, nil)).
		Preload("Stages", func(db *gorm.DB) *gorm.DB { return db.Order("pipeline_stages.sort_order") }).
		Order("is_default DESC, id").
		Find(&rows).Error
	return rows, err
}

// CountPipelines menghitung pipeline tenant (untuk pembuatan default malas).
func CountPipelines(ctx context.Context, tx *gorm.DB) (int64, error) {
	var n int64
	err := scopeTenant(ctx, tenantDB(ctx, tx).Model(&models.Pipeline{})).Count(&n).Error
	return n, err
}

// CreatePipeline menyimpan pipeline baru.
func CreatePipeline(ctx context.Context, tx *gorm.DB, p *models.Pipeline) error {
	p.TenantID = currentTenantID(ctx)
	return tenantDB(ctx, tx).Create(p).Error
}

// CreatePipelineStages menyimpan sekumpulan tahap.
func CreatePipelineStages(ctx context.Context, tx *gorm.DB, rows []models.PipelineStage) error {
	if len(rows) == 0 {
		return nil
	}
	tid := currentTenantID(ctx)
	for i := range rows {
		rows[i].TenantID = tid
	}
	return tenantDB(ctx, tx).Create(&rows).Error
}

// FindPipeline memuat satu pipeline (+ tahap) milik tenant.
func FindPipeline(ctx context.Context, tx *gorm.DB, id string) (models.Pipeline, error) {
	var p models.Pipeline
	err := scopeTenant(ctx, tenantDB(ctx, tx)).
		Preload("Stages", func(db *gorm.DB) *gorm.DB { return db.Order("pipeline_stages.sort_order") }).
		First(&p, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return p, ErrPipelineNotFound
	}
	return p, err
}

// DefaultPipeline mengembalikan pipeline default tenant (is_default), atau yang
// pertama bila belum ada yang ditandai. found=false bila tenant belum punya.
func DefaultPipeline(ctx context.Context, tx *gorm.DB) (models.Pipeline, bool, error) {
	var p models.Pipeline
	err := scopeTenant(ctx, tenantDB(ctx, tx)).
		Preload("Stages", func(db *gorm.DB) *gorm.DB { return db.Order("pipeline_stages.sort_order") }).
		Order("is_default DESC, id").
		First(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return p, false, nil
	}
	if err != nil {
		return p, false, err
	}
	return p, true, nil
}

// FindStage memuat satu tahap milik tenant, memastikan tahap itu milik pipeline
// yang diberikan.
func FindStage(ctx context.Context, tx *gorm.DB, pipelineID, stageID string) (models.PipelineStage, error) {
	var s models.PipelineStage
	err := scopeTenant(ctx, tenantDB(ctx, tx)).
		First(&s, "id = ? AND pipeline_id = ?", stageID, pipelineID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return s, ErrStageNotFound
	}
	return s, err
}

// ── Deal ──────────────────────────────────────────────────────────────────

// DealFilter menyaring daftar deal.
type DealFilter struct {
	PipelineID string
	StageID    string
	Status     string
	CustomerID string
}

// ListDeals mengembalikan satu halaman deal yang terlihat oleh user konteks
// (lapis 3).
func ListDeals(ctx context.Context, f DealFilter, limit, offset int) ([]models.Deal, int64, error) {
	base := func() *gorm.DB {
		q := scopeVisibility(ctx, scopeTenant(ctx, tenantDB(ctx, nil).Model(&models.Deal{})), "deals")
		if f.PipelineID != "" {
			q = q.Where("pipeline_id = ?", f.PipelineID)
		}
		if f.StageID != "" {
			q = q.Where("stage_id = ?", f.StageID)
		}
		if f.Status != "" {
			q = q.Where("status = ?", f.Status)
		}
		if f.CustomerID != "" {
			q = q.Where("customer_id = ?", f.CustomerID)
		}
		return q
	}
	var total int64
	if err := base().Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []models.Deal
	err := base().Order("created_at DESC").Limit(limit).Offset(offset).Find(&rows).Error
	return rows, total, err
}

// FindDeal memuat satu deal yang terlihat oleh user konteks.
func FindDeal(ctx context.Context, tx *gorm.DB, id string) (models.Deal, error) {
	var d models.Deal
	err := scopeVisibility(ctx, scopeTenant(ctx, tenantDB(ctx, tx)), "deals").
		First(&d, "deals.id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return d, ErrDealNotFound
	}
	return d, err
}

// CreateDeal menyimpan deal baru (owner_id di-stempel pemanggil).
func CreateDeal(ctx context.Context, tx *gorm.DB, d *models.Deal) error {
	d.TenantID = currentTenantID(ctx)
	return tenantDB(ctx, tx).Create(d).Error
}

// SaveDeal menyimpan kolom deal yang berubah.
func SaveDeal(ctx context.Context, tx *gorm.DB, d *models.Deal) error {
	res := scopeTenant(ctx, tenantDB(ctx, tx)).Model(&models.Deal{}).
		Where("id = ?", d.ID).
		Updates(map[string]any{
			"stage_id":            d.StageID,
			"customer_id":         d.CustomerID,
			"lead_source_id":      d.LeadSourceID,
			"title":               d.Title,
			"value":               d.Value,
			"expected_close_date": d.ExpectedCloseDate,
			"status":              d.Status,
			"lost_reason":         d.LostReason,
			"closed_at":           d.ClosedAt,
			"updated_at":          gorm.Expr("now()"),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrDealNotFound
	}
	return nil
}

// ── Activity ──────────────────────────────────────────────────────────────

// ActivityFilter menyaring daftar aktivitas.
type ActivityFilter struct {
	DealID     string
	CustomerID string
	Status     string
	Kind       string
}

// ListActivities mengembalikan satu halaman aktivitas yang terlihat user konteks.
func ListActivities(ctx context.Context, f ActivityFilter, limit, offset int) ([]models.Activity, int64, error) {
	base := func() *gorm.DB {
		q := scopeVisibility(ctx, scopeTenant(ctx, tenantDB(ctx, nil).Model(&models.Activity{})), "activities")
		if f.DealID != "" {
			q = q.Where("deal_id = ?", f.DealID)
		}
		if f.CustomerID != "" {
			q = q.Where("customer_id = ?", f.CustomerID)
		}
		if f.Status != "" {
			q = q.Where("status = ?", f.Status)
		}
		if f.Kind != "" {
			q = q.Where("kind = ?", f.Kind)
		}
		return q
	}
	var total int64
	if err := base().Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []models.Activity
	err := base().Order("COALESCE(due_at, created_at) DESC").Limit(limit).Offset(offset).Find(&rows).Error
	return rows, total, err
}

// FindActivity memuat satu aktivitas yang terlihat user konteks.
func FindActivity(ctx context.Context, tx *gorm.DB, id string) (models.Activity, error) {
	var a models.Activity
	err := scopeVisibility(ctx, scopeTenant(ctx, tenantDB(ctx, tx)), "activities").
		First(&a, "activities.id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return a, ErrActivityNotFound
	}
	return a, err
}

// CreateActivity menyimpan aktivitas baru.
func CreateActivity(ctx context.Context, tx *gorm.DB, a *models.Activity) error {
	a.TenantID = currentTenantID(ctx)
	return tenantDB(ctx, tx).Create(a).Error
}

// SaveActivity menyimpan kolom aktivitas yang berubah.
func SaveActivity(ctx context.Context, tx *gorm.DB, a *models.Activity) error {
	res := scopeTenant(ctx, tenantDB(ctx, tx)).Model(&models.Activity{}).
		Where("id = ?", a.ID).
		Updates(map[string]any{
			"subject":      a.Subject,
			"body":         a.Body,
			"due_at":       a.DueAt,
			"completed_at": a.CompletedAt,
			"status":       a.Status,
			"updated_at":   gorm.Expr("now()"),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrActivityNotFound
	}
	return nil
}
