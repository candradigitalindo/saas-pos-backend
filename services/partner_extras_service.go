package services

import (
	"context"
	"fmt"
	"strings"

	"candra/backend-api/helpers"
	"candra/backend-api/internal/reqctx"
	"candra/backend-api/models"
	"candra/backend-api/repositories"
	"candra/backend-api/structs"
)

// Layanan pelengkap Program Mitra (blueprint G.4/G.5): target, materi jualan,
// pelatihan, sengketa atribusi. Tabelnya lahir bersama Fase 12; ini jalan
// masuknya.

// ── Target ───────────────────────────────────────────────────────────────

// SetPartnerTarget menetapkan target merchant aktif sebuah periode (panel internal).
func SetPartnerTarget(ctx context.Context, in structs.PartnerTargetRequest) (structs.PartnerTargetResponse, error) {
	var out structs.PartnerTargetResponse
	ps, pe, err := parsePeriod(in.PeriodStart, in.PeriodEnd)
	if err != nil {
		return out, err
	}
	if _, err := repositories.FindPartnerByID(ctx, in.PartnerID); err != nil {
		return out, err
	}
	t := models.PartnerTarget{
		PartnerID: in.PartnerID, PeriodStart: ps, PeriodEnd: pe,
		TargetMerchants: in.TargetMerchants,
	}
	if err := repositories.UpsertPartnerTarget(ctx, &t); err != nil {
		return out, err
	}
	return partnerTargetToResponse(t), nil
}

// ListPartnerTargets mengembalikan target sebuah mitra, dengan pencapaian
// DIHITUNG ULANG dari merchant yang langganannya masih hidup — bukan dari
// jumlah pendaftaran (blueprint G.4: papan peringkat berbasis merchant aktif,
// "kalau salah, ini justru memicu pendaftaran fiktif").
func ListPartnerTargets(ctx context.Context, partnerID string) ([]structs.PartnerTargetResponse, error) {
	rows, err := repositories.ListPartnerTargets(ctx, partnerID)
	if err != nil {
		return nil, err
	}
	aktif, err := repositories.CountActiveMerchantsForPartner(ctx, partnerID)
	if err != nil {
		return nil, err
	}
	out := make([]structs.PartnerTargetResponse, len(rows))
	for i, r := range rows {
		if int64(r.AchievedMerchants) != aktif {
			_ = repositories.SetTargetAchieved(ctx, r.ID, int(aktif))
			r.AchievedMerchants = int(aktif)
		}
		out[i] = partnerTargetToResponse(r)
	}
	return out, nil
}

// ── Materi jualan ────────────────────────────────────────────────────────

func CreatePartnerMaterial(ctx context.Context, in structs.PartnerMaterialRequest) (structs.PartnerMaterialResponse, error) {
	m := models.PartnerMaterial{
		Title: strings.TrimSpace(in.Title), Kind: in.Kind,
		FileURL: strings.TrimSpace(in.FileURL), Version: orInt(in.Version, 1), IsActive: true,
	}
	if in.MinTierID != "" {
		m.MinTierID = &in.MinTierID
	}
	if err := repositories.CreatePartnerMaterial(ctx, &m); err != nil {
		return structs.PartnerMaterialResponse{}, err
	}
	return materialToResponse(m), nil
}

// ListMaterialsForCurrentPartner mengembalikan materi yang boleh dilihat mitra
// permintaan ini — disaring menurut tingkatnya.
func ListMaterialsForCurrentPartner(ctx context.Context) ([]structs.PartnerMaterialResponse, error) {
	partner, err := repositories.FindPartnerByID(ctx, reqctx.PartnerID(ctx))
	if err != nil {
		return nil, err
	}
	rows, err := repositories.ListMaterialsForTier(ctx, partner.TierID)
	if err != nil {
		return nil, err
	}
	out := make([]structs.PartnerMaterialResponse, len(rows))
	for i := range rows {
		out[i] = materialToResponse(rows[i])
	}
	return out, nil
}

func ListAllPartnerMaterials(ctx context.Context) ([]structs.PartnerMaterialResponse, error) {
	rows, err := repositories.ListAllPartnerMaterials(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]structs.PartnerMaterialResponse, len(rows))
	for i := range rows {
		out[i] = materialToResponse(rows[i])
	}
	return out, nil
}

// ── Pelatihan ────────────────────────────────────────────────────────────

func CreatePartnerTraining(ctx context.Context, in structs.PartnerTrainingRequest) (structs.PartnerTrainingResponse, error) {
	t := models.PartnerTraining{
		Title: strings.TrimSpace(in.Title), ContentURL: strings.TrimSpace(in.ContentURL),
		IsRequired: in.IsRequired, SortOrder: in.SortOrder,
	}
	if err := repositories.CreatePartnerTraining(ctx, &t); err != nil {
		return structs.PartnerTrainingResponse{}, err
	}
	return structs.PartnerTrainingResponse{
		ID: t.ID, Title: t.Title, ContentURL: t.ContentURL,
		IsRequired: t.IsRequired, SortOrder: t.SortOrder,
	}, nil
}

// ListTrainingsForCurrentPartnerUser mengembalikan seluruh modul pelatihan
// beserta status penyelesaiannya untuk akun mitra permintaan ini.
func ListTrainingsForCurrentPartnerUser(ctx context.Context) ([]structs.PartnerTrainingResponse, error) {
	trainings, err := repositories.ListPartnerTrainings(ctx)
	if err != nil {
		return nil, err
	}
	recs, err := repositories.TrainingRecordsForUser(ctx, reqctx.PartnerUserID(ctx))
	if err != nil {
		return nil, err
	}
	out := make([]structs.PartnerTrainingResponse, len(trainings))
	for i, t := range trainings {
		r := structs.PartnerTrainingResponse{
			ID: t.ID, Title: t.Title, ContentURL: t.ContentURL,
			IsRequired: t.IsRequired, SortOrder: t.SortOrder,
		}
		if rec, ok := recs[t.ID]; ok && rec.CompletedAt != nil {
			r.Completed = true
			r.CompletedAt = rec.CompletedAt.Format(partnerTimeLayout)
			r.Score = rec.Score
		}
		out[i] = r
	}
	return out, nil
}

// CompleteTraining menandai satu modul selesai untuk akun mitra permintaan ini.
func CompleteTraining(ctx context.Context, trainingID string, score *int) error {
	return repositories.MarkTrainingCompleted(ctx, reqctx.PartnerUserID(ctx), trainingID, score)
}

// ── Sengketa atribusi ────────────────────────────────────────────────────

// CreatePartnerDispute diajukan MITRA atas atribusi yang ia rasa keliru
// (blueprint G.4 P2). Keputusannya nanti oleh admin, dan dicatat.
func CreatePartnerDispute(ctx context.Context, in structs.PartnerDisputeRequest) (structs.PartnerDisputeResponse, error) {
	d := models.PartnerDispute{
		ClaimantPartnerID: reqctx.PartnerID(ctx),
		Reason:            strings.TrimSpace(in.Reason),
		Status:            "open",
	}
	if in.TenantID != "" {
		d.TenantID = &in.TenantID
	}
	if in.LeadID != "" {
		d.LeadID = &in.LeadID
	}
	if err := repositories.CreatePartnerDispute(ctx, &d); err != nil {
		return structs.PartnerDisputeResponse{}, err
	}
	return disputeToResponse(d), nil
}

// ListDisputes: partnerID kosong = seluruh sengketa (panel internal).
func ListDisputes(ctx context.Context, partnerID, status string) ([]structs.PartnerDisputeResponse, error) {
	rows, err := repositories.ListPartnerDisputes(ctx, partnerID, status)
	if err != nil {
		return nil, err
	}
	out := make([]structs.PartnerDisputeResponse, len(rows))
	for i := range rows {
		out[i] = disputeToResponse(rows[i])
	}
	return out, nil
}

// ResolveDispute mencatat keputusan admin. Bila diterima ('accepted') dan
// menyangkut sebuah tenant, atribusi tenant itu DICABUT — sengketa yang
// dimenangkan harus benar-benar mengubah keadaan, bukan sekadar berganti label.
func ResolveDispute(ctx context.Context, id string, in structs.PartnerDisputeResolveRequest) error {
	d, err := repositories.FindPartnerDispute(ctx, id)
	if err != nil {
		return err
	}
	if d.Status != "open" {
		return fmt.Errorf("%w: sengketa ini sudah diputus", helpers.ErrConflict)
	}
	adminID := reqctx.PlatformAdminID(ctx)
	if err := repositories.ResolvePartnerDispute(ctx, id, in.Status, adminID, strings.TrimSpace(in.DecisionNote)); err != nil {
		return err
	}
	if in.Status == "accepted" && d.TenantID != nil {
		if ref, ok, ferr := repositories.FindReferralByTenant(ctx, *d.TenantID); ferr == nil && ok {
			_ = repositories.SetReferralStatus(ctx, ref.ID, "disputed")
		}
	}
	AuditPlatformAction(ctx, "partner.dispute."+in.Status, "partner_disputes", id)
	return nil
}

// ── DTO ─────────────────────────────────────────────────────────────────

func partnerTargetToResponse(t models.PartnerTarget) structs.PartnerTargetResponse {
	return structs.PartnerTargetResponse{
		ID: t.ID, PeriodStart: t.PeriodStart.Format("2006-01-02"),
		PeriodEnd:       t.PeriodEnd.Format("2006-01-02"),
		TargetMerchants: t.TargetMerchants, AchievedMerchants: t.AchievedMerchants,
	}
}

func materialToResponse(m models.PartnerMaterial) structs.PartnerMaterialResponse {
	return structs.PartnerMaterialResponse{
		ID: m.ID, Title: m.Title, Kind: m.Kind, FileURL: m.FileURL,
		Version: m.Version, IsActive: m.IsActive,
	}
}

func disputeToResponse(d models.PartnerDispute) structs.PartnerDisputeResponse {
	r := structs.PartnerDisputeResponse{
		ID: d.ID, ClaimantPartnerID: d.ClaimantPartnerID, Reason: d.Reason,
		Status: d.Status, DecisionNote: d.DecisionNote,
		CreatedAt: d.CreatedAt.Format(partnerTimeLayout),
	}
	if d.TenantID != nil {
		r.TenantID = *d.TenantID
	}
	if d.LeadID != nil {
		r.LeadID = *d.LeadID
	}
	if d.DecidedAt != nil {
		r.DecidedAt = d.DecidedAt.Format(partnerTimeLayout)
	}
	return r
}

// ListAllTrainings mengembalikan seluruh modul pelatihan tanpa status
// penyelesaian — dipakai panel internal.
func ListAllTrainings(ctx context.Context) ([]structs.PartnerTrainingResponse, error) {
	rows, err := repositories.ListPartnerTrainings(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]structs.PartnerTrainingResponse, len(rows))
	for i, t := range rows {
		out[i] = structs.PartnerTrainingResponse{
			ID: t.ID, Title: t.Title, ContentURL: t.ContentURL,
			IsRequired: t.IsRequired, SortOrder: t.SortOrder,
		}
	}
	return out, nil
}
