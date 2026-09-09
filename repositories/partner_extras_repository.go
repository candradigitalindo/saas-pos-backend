package repositories

import (
	"context"
	"errors"
	"time"

	"candra/backend-api/database"
	"candra/backend-api/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Repositori pelengkap Program Mitra (migrasi 000028): target, materi jualan,
// pelatihan, dan sengketa atribusi. Tabelnya sudah ada sejak awal Fase 12;
// di sini jalan masuknya.
//
// Tabel PLATFORM — tanpa RLS, FK tunggal (§5.17).

var (
	ErrPartnerTargetNotFound   = errors.New("target mitra tidak ditemukan")
	ErrPartnerMaterialNotFound = errors.New("materi mitra tidak ditemukan")
	ErrPartnerDisputeNotFound  = errors.New("sengketa mitra tidak ditemukan")
)

// ── Target ───────────────────────────────────────────────────────────────

// UpsertPartnerTarget menyimpan target periode; diulang = diperbarui
// (UNIQUE partner_id, period_start).
func UpsertPartnerTarget(ctx context.Context, t *models.PartnerTarget) error {
	return database.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "partner_id"}, {Name: "period_start"}},
		DoUpdates: clause.AssignmentColumns([]string{"period_end", "target_merchants"}),
	}).Create(t).Error
}

func ListPartnerTargets(ctx context.Context, partnerID string) ([]models.PartnerTarget, error) {
	var rows []models.PartnerTarget
	err := database.DB.WithContext(ctx).
		Where("partner_id = ?", partnerID).Order("period_start DESC").Find(&rows).Error
	return rows, err
}

// CountActiveMerchantsForPartner menghitung merchant binaan yang langganannya
// masih hidup — dasar pencapaian target (blueprint G.4: papan peringkat
// BERBASIS MERCHANT AKTIF, bukan jumlah pendaftaran).
func CountActiveMerchantsForPartner(ctx context.Context, partnerID string) (int64, error) {
	var n int64
	err := database.DB.WithContext(ctx).
		Table("partner_referrals pr").
		Joins("JOIN subscriptions s ON s.tenant_id = pr.tenant_id").
		Where("pr.partner_id = ? AND pr.status IN ('pending','active') AND s.status IN ('active','trial')", partnerID).
		Count(&n).Error
	return n, err
}

// SetTargetAchieved menyimpan pencapaian yang sudah dihitung.
func SetTargetAchieved(ctx context.Context, id string, achieved int) error {
	return database.DB.WithContext(ctx).Model(&models.PartnerTarget{}).
		Where("id = ?", id).UpdateColumn("achieved_merchants", achieved).Error
}

// ── Materi jualan ────────────────────────────────────────────────────────

func CreatePartnerMaterial(ctx context.Context, m *models.PartnerMaterial) error {
	return database.DB.WithContext(ctx).Create(m).Error
}

// ListMaterialsForTier mengembalikan materi yang boleh dilihat sebuah tingkat:
// materi tanpa batas tingkat, atau yang batasnya tepat tingkat itu.
func ListMaterialsForTier(ctx context.Context, tierID string) ([]models.PartnerMaterial, error) {
	var rows []models.PartnerMaterial
	err := database.DB.WithContext(ctx).
		Where("is_active = true AND (min_tier_id IS NULL OR min_tier_id = ?)", tierID).
		Order("kind, title").Find(&rows).Error
	return rows, err
}

func ListAllPartnerMaterials(ctx context.Context) ([]models.PartnerMaterial, error) {
	var rows []models.PartnerMaterial
	err := database.DB.WithContext(ctx).Order("kind, title").Find(&rows).Error
	return rows, err
}

func SetPartnerMaterialActive(ctx context.Context, id string, active bool) error {
	res := database.DB.WithContext(ctx).Model(&models.PartnerMaterial{}).
		Where("id = ?", id).UpdateColumn("is_active", active)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrPartnerMaterialNotFound
	}
	return nil
}

// ── Pelatihan ────────────────────────────────────────────────────────────

func CreatePartnerTraining(ctx context.Context, t *models.PartnerTraining) error {
	return database.DB.WithContext(ctx).Create(t).Error
}

func ListPartnerTrainings(ctx context.Context) ([]models.PartnerTraining, error) {
	var rows []models.PartnerTraining
	err := database.DB.WithContext(ctx).Order("sort_order, title").Find(&rows).Error
	return rows, err
}

// TrainingRecordsForUser mengembalikan catatan penyelesaian pelatihan seorang
// akun mitra, dipetakan per training_id.
func TrainingRecordsForUser(ctx context.Context, partnerUserID string) (map[string]models.PartnerTrainingRecord, error) {
	var rows []models.PartnerTrainingRecord
	if err := database.DB.WithContext(ctx).
		Where("partner_user_id = ?", partnerUserID).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[string]models.PartnerTrainingRecord, len(rows))
	for _, r := range rows {
		out[r.TrainingID] = r
	}
	return out, nil
}

// MarkTrainingCompleted mencatat penyelesaian sebuah modul; diulang = diperbarui.
func MarkTrainingCompleted(ctx context.Context, partnerUserID, trainingID string, score *int) error {
	rec := models.PartnerTrainingRecord{
		PartnerUserID: partnerUserID, TrainingID: trainingID, Score: score,
	}
	now := time.Now().UTC()
	rec.CompletedAt = &now
	return database.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "partner_user_id"}, {Name: "training_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"completed_at", "score"}),
	}).Create(&rec).Error
}

// RequiredTrainingsDone melapor apakah seluruh pelatihan WAJIB sudah selesai —
// syarat naik tingkat agen (blueprint G.6 #3).
func RequiredTrainingsDone(ctx context.Context, partnerUserID string) (bool, error) {
	var kurang int64
	err := database.DB.WithContext(ctx).
		Table("partner_trainings t").
		Joins("LEFT JOIN partner_training_records r ON r.training_id = t.id AND r.partner_user_id = ?", partnerUserID).
		Where("t.is_required = true AND r.completed_at IS NULL").
		Count(&kurang).Error
	return kurang == 0, err
}

// ── Sengketa atribusi ────────────────────────────────────────────────────

func CreatePartnerDispute(ctx context.Context, d *models.PartnerDispute) error {
	return database.DB.WithContext(ctx).Create(d).Error
}

func FindPartnerDispute(ctx context.Context, id string) (models.PartnerDispute, error) {
	var d models.PartnerDispute
	err := database.DB.WithContext(ctx).First(&d, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return d, ErrPartnerDisputeNotFound
	}
	return d, err
}

// ListPartnerDisputes: partnerID kosong = seluruh sengketa (panel internal).
func ListPartnerDisputes(ctx context.Context, partnerID, status string) ([]models.PartnerDispute, error) {
	q := database.DB.WithContext(ctx)
	if partnerID != "" {
		q = q.Where("claimant_partner_id = ?", partnerID)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var rows []models.PartnerDispute
	err := q.Order("created_at DESC").Find(&rows).Error
	return rows, err
}

// ResolvePartnerDispute mencatat keputusan admin beserta alasannya — blueprint
// G.2 #5 mewajibkan keputusannya terekam, bukan hanya statusnya berubah.
func ResolvePartnerDispute(ctx context.Context, id, status, decidedBy, note string) error {
	res := database.DB.WithContext(ctx).Model(&models.PartnerDispute{}).
		Where("id = ? AND status = 'open'", id).
		Updates(map[string]any{
			// partner_disputes memang tidak punya updated_at (§5.12) —
			// decided_at yang menjadi penanda waktu keputusan.
			"status": status, "decided_by": decidedBy, "decision_note": note,
			"decided_at": time.Now().UTC(),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrPartnerDisputeNotFound
	}
	return nil
}
