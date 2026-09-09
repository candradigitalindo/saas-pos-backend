package repositories

import (
	"context"
	"errors"
	"time"

	"candra/backend-api/database"
	"candra/backend-api/internal/ulid"
	"candra/backend-api/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Repositori Program Mitra Penjual (Fase 12, §5.12, blueprint Bagian G).
//
// Tabel PLATFORM tanpa RLS — semua akses memakai database.DB langsung dengan
// filter `partner_id` / `tenant_id` EKSPLISIT dan FK tunggal (§5.17). Tidak ada
// scopeTenant di sini (mitra bukan tenant). Fungsi mesin komisi lintas-tenant.

var (
	ErrPartnerNotFound           = errors.New("mitra tidak ditemukan")
	ErrPartnerUserNotFound       = errors.New("akun mitra tidak ditemukan")
	ErrPartnerTierNotFound       = errors.New("tingkat mitra tidak ditemukan")
	ErrPartnerLeadNotFound       = errors.New("prospek mitra tidak ditemukan")
	ErrPartnerPayoutNotFound     = errors.New("pencairan mitra tidak ditemukan")
	ErrPartnerCommissionNotFound = errors.New("komisi mitra tidak ditemukan")
)

// ── Tier ─────────────────────────────────────────────────────────────────

func CreatePartnerTier(ctx context.Context, t *models.PartnerTier) error {
	return database.DB.WithContext(ctx).Create(t).Error
}

// FindPartnerTierByName mencari tingkat berdasarkan nama (kunci alami §5.12).
func FindPartnerTierByName(ctx context.Context, name string) (models.PartnerTier, error) {
	var t models.PartnerTier
	err := database.DB.WithContext(ctx).First(&t, "name = ?", name).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return t, ErrPartnerTierNotFound
	}
	return t, err
}

func ListPartnerTiers(ctx context.Context) ([]models.PartnerTier, error) {
	var rows []models.PartnerTier
	err := database.DB.WithContext(ctx).Order("name").Find(&rows).Error
	return rows, err
}

// ── Partner ──────────────────────────────────────────────────────────────

func CreatePartner(ctx context.Context, p *models.Partner) error {
	return database.DB.WithContext(ctx).Create(p).Error
}

func FindPartnerByID(ctx context.Context, id string) (models.Partner, error) {
	var p models.Partner
	err := database.DB.WithContext(ctx).Preload("Tier").First(&p, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return p, ErrPartnerNotFound
	}
	return p, err
}

// PartnersByIDs memuat banyak mitra + tingkatnya SEKALI JALAN — dipakai mesin
// komisi agar tidak N+1 saat menelusuri ribuan referral.
func PartnersByIDs(ctx context.Context, ids []string) (map[string]models.Partner, error) {
	out := map[string]models.Partner{}
	if len(ids) == 0 {
		return out, nil
	}
	var rows []models.Partner
	if err := database.DB.WithContext(ctx).Preload("Tier").
		Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, p := range rows {
		out[p.ID] = p
	}
	return out, nil
}

// FindActivePartnerByReferralCode dipakai saat pendaftaran tenant. Hanya mitra
// berstatus 'active' yang menautkan referral — kode mitra pending/suspended
// disimpan mentah di tenants.referral_code_used tapi tak membuat atribusi.
func FindActivePartnerByReferralCode(ctx context.Context, tx *gorm.DB, code string) (models.Partner, bool, error) {
	var p models.Partner
	db := database.DB
	if tx != nil {
		db = tx
	}
	err := db.WithContext(ctx).Preload("Tier").
		First(&p, "referral_code = ? AND status = 'active'", code).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return p, false, nil
	}
	if err != nil {
		return p, false, err
	}
	return p, true, nil
}

func ListPartners(ctx context.Context, status string, limit, offset int) ([]models.Partner, int64, error) {
	base := func() *gorm.DB {
		q := database.DB.WithContext(ctx).Model(&models.Partner{})
		if status != "" {
			q = q.Where("status = ?", status)
		}
		return q
	}
	var total int64
	if err := base().Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []models.Partner
	err := base().Preload("Tier").Order("created_at DESC").Limit(limit).Offset(offset).Find(&rows).Error
	return rows, total, err
}

// ActivatePartner memverifikasi & mengaktifkan mitra sekaligus (G.6 langkah 2 &
// 4): status → 'active', verified_at & joined_at diisi sekali.
func ActivatePartner(ctx context.Context, id string, at time.Time) error {
	res := database.DB.WithContext(ctx).Model(&models.Partner{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":      "active",
			"verified_at": gorm.Expr("COALESCE(verified_at, ?)", at),
			"joined_at":   gorm.Expr("COALESCE(joined_at, ?)", at),
			"updated_at":  gorm.Expr("now()"),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrPartnerNotFound
	}
	return nil
}

func SetPartnerStatus(ctx context.Context, id, status string) error {
	res := database.DB.WithContext(ctx).Model(&models.Partner{}).
		Where("id = ?", id).
		Updates(map[string]any{"status": status, "updated_at": gorm.Expr("now()")})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrPartnerNotFound
	}
	return nil
}

// ── Partner user (akun login) ────────────────────────────────────────────

func CreatePartnerUser(ctx context.Context, u *models.PartnerUser) error {
	return database.DB.WithContext(ctx).Create(u).Error
}

func FindPartnerUserByID(ctx context.Context, id string) (models.PartnerUser, error) {
	var u models.PartnerUser
	err := database.DB.WithContext(ctx).First(&u, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return u, ErrPartnerUserNotFound
	}
	return u, err
}

// FindPartnerUserByEmail — login portal mitra memakai EMAIL (§5.12; unik selama
// belum dihapus lewat partial index uq_partner_users_email).
func FindPartnerUserByEmail(ctx context.Context, email string) (models.PartnerUser, error) {
	var u models.PartnerUser
	err := database.DB.WithContext(ctx).First(&u, "email = ?", email).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return u, ErrPartnerUserNotFound
	}
	return u, err
}

func TouchPartnerUserLogin(ctx context.Context, id string) error {
	return database.DB.WithContext(ctx).Model(&models.PartnerUser{}).
		Where("id = ?", id).UpdateColumn("last_login_at", time.Now().UTC()).Error
}

// ── Lead ─────────────────────────────────────────────────────────────────

func CreatePartnerLead(ctx context.Context, l *models.PartnerLead) error {
	return database.DB.WithContext(ctx).Create(l).Error
}

func ListPartnerLeads(ctx context.Context, partnerID, status string) ([]models.PartnerLead, error) {
	q := database.DB.WithContext(ctx).Where("partner_id = ?", partnerID)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var rows []models.PartnerLead
	err := q.Order("created_at DESC").Find(&rows).Error
	return rows, err
}

// MatchOpenLeadForPartner mencari prospek mitra yang masih terbuka & belum lewat
// masa atribusi, cocok berdasarkan nomor telepon (paling andal) untuk ditautkan
// saat merchant-nya benar-benar mendaftar.
func MatchOpenLeadForPartner(ctx context.Context, tx *gorm.DB, partnerID, phone string) (models.PartnerLead, bool, error) {
	if phone == "" {
		return models.PartnerLead{}, false, nil
	}
	db := database.DB
	if tx != nil {
		db = tx
	}
	var l models.PartnerLead
	err := db.WithContext(ctx).
		Where("partner_id = ? AND phone = ? AND status NOT IN ('registered','activated','lost') AND attribution_expires_at >= now()", partnerID, phone).
		Order("created_at DESC").First(&l).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return l, false, nil
	}
	if err != nil {
		return l, false, err
	}
	return l, true, nil
}

func MarkLeadRegistered(ctx context.Context, tx *gorm.DB, leadID, tenantID string) error {
	db := database.DB
	if tx != nil {
		db = tx
	}
	return db.WithContext(ctx).Model(&models.PartnerLead{}).
		Where("id = ?", leadID).
		Updates(map[string]any{"status": "registered", "converted_tenant_id": tenantID, "updated_at": gorm.Expr("now()")}).Error
}

// MarkLeadActivated dipanggil mesin komisi saat merchant memenuhi ambang
// aktivasi (blueprint G.6 langkah 8).
func MarkLeadActivated(ctx context.Context, leadID string) error {
	return database.DB.WithContext(ctx).Model(&models.PartnerLead{}).
		Where("id = ? AND status = 'registered'", leadID).
		Updates(map[string]any{"status": "activated", "updated_at": gorm.Expr("now()")}).Error
}

// ── Referral (kaitan mitra ↔ tenant) ────────────────────────────────────

func CreatePartnerReferral(ctx context.Context, tx *gorm.DB, r *models.PartnerReferral) error {
	db := database.DB
	if tx != nil {
		db = tx
	}
	return db.WithContext(ctx).Create(r).Error
}

func FindReferralByTenant(ctx context.Context, tenantID string) (models.PartnerReferral, bool, error) {
	var r models.PartnerReferral
	err := database.DB.WithContext(ctx).First(&r, "tenant_id = ?", tenantID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return r, false, nil
	}
	if err != nil {
		return r, false, err
	}
	return r, true, nil
}

func ListReferralsByPartner(ctx context.Context, partnerID string) ([]models.PartnerReferral, error) {
	var rows []models.PartnerReferral
	err := database.DB.WithContext(ctx).
		Where("partner_id = ?", partnerID).Order("attributed_at DESC").Find(&rows).Error
	return rows, err
}

// ListLiveReferrals mengembalikan referral yang masih berhak komisi
// ('pending' = belum aktivasi, 'active' = sudah). partnerID kosong = SELURUH
// mitra (cron bulanan); diisi = satu mitra (hitung ulang bertarget / test).
func ListLiveReferrals(ctx context.Context, partnerID string) ([]models.PartnerReferral, error) {
	q := database.DB.WithContext(ctx).Where("status IN ('pending','active')")
	if partnerID != "" {
		q = q.Where("partner_id = ?", partnerID)
	}
	var rows []models.PartnerReferral
	err := q.Order("attributed_at").Find(&rows).Error
	return rows, err
}

// ActivateReferral menandai referral aktif + awal masa komisi. `endsAt` nil
// untuk tier tanpa batas bulan.
func ActivateReferral(ctx context.Context, id string, at time.Time, endsAt *time.Time) error {
	return database.DB.WithContext(ctx).Model(&models.PartnerReferral{}).
		Where("id = ? AND activated_at IS NULL", id).
		Updates(map[string]any{
			"activated_at": at, "commission_starts_at": at,
			"commission_ends_at": endsAt, "status": "active",
		}).Error
}

func SetReferralStatus(ctx context.Context, id, status string) error {
	return database.DB.WithContext(ctx).Model(&models.PartnerReferral{}).
		Where("id = ?", id).Update("status", status).Error
}

// ── Sinyal aktivasi merchant (lintas tenant) ────────────────────────────

// CountCompletedSalesForTenant menghitung transaksi selesai sebuah tenant —
// salah satu ambang aktivasi komisi (blueprint G.2 #3).
func CountCompletedSalesForTenant(ctx context.Context, tenantID string) (int64, error) {
	var n int64
	err := database.DB.WithContext(ctx).Table("sales").
		Where("tenant_id = ? AND status = 'completed'", tenantID).Count(&n).Error
	return n, err
}

// PaidSubInvoicesForTenant mengembalikan faktur langganan yang DIBAYAR pada
// rentang tanggal (berdasarkan paid_at) — dasar komisi (blueprint G.2 #2).
func PaidSubInvoicesForTenant(ctx context.Context, tenantID, from, to string) ([]models.SubscriptionInvoice, error) {
	var rows []models.SubscriptionInvoice
	err := database.DB.WithContext(ctx).
		Where("tenant_id = ? AND status = 'paid' AND paid_at::date BETWEEN ? AND ?", tenantID, from, to).
		Order("paid_at").Find(&rows).Error
	return rows, err
}

// SubscriptionStateForTenant mengembalikan status & tanggal batal langganan
// tenant (untuk clawback & tampilan merchant di portal).
func SubscriptionStateForTenant(ctx context.Context, tenantID string) (models.Subscription, bool, error) {
	var s models.Subscription
	err := database.DB.WithContext(ctx).First(&s, "tenant_id = ?", tenantID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return s, false, nil
	}
	if err != nil {
		return s, false, err
	}
	return s, true, nil
}

// ── Commission ───────────────────────────────────────────────────────────

// UpsertHeldCommission menyisipkan/memperbarui satu komisi. Baris yang sudah
// 'approved'/'paid'/'clawed_back'/'canceled' TIDAK disentuh (deterministik:
// hanya 'held' yang dihitung ulang).
func UpsertHeldCommission(ctx context.Context, c *models.PartnerCommission) error {
	if c.ID == "" {
		c.ID = ulid.New()
	}
	return database.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "referral_id"}, {Name: "subscription_invoice_id"}, {Name: "period_month"},
		},
		DoUpdates: clause.Assignments(map[string]any{
			"base_amount": c.BaseAmount, "rate": c.Rate, "amount": c.Amount,
			"updated_at": gorm.Expr("now()"),
		}),
		Where: clause.Where{Exprs: []clause.Expression{
			gorm.Expr("partner_commissions.status = 'held'"),
		}},
	}).Create(c).Error
}

func ListPartnerCommissions(ctx context.Context, partnerID, status, from, to string) ([]models.PartnerCommission, error) {
	q := database.DB.WithContext(ctx).Where("partner_id = ?", partnerID)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if from != "" && to != "" {
		q = q.Where("period_month BETWEEN date_trunc('month', ?::date) AND ?::date", from, to)
	}
	var rows []models.PartnerCommission
	err := q.Order("period_month DESC, created_at DESC").Find(&rows).Error
	return rows, err
}

func SumPartnerCommissionsByStatus(ctx context.Context, partnerID string) (map[string]int64, error) {
	type row struct {
		Status string
		Total  int64
	}
	var rs []row
	err := database.DB.WithContext(ctx).Model(&models.PartnerCommission{}).
		Select("status, COALESCE(SUM(amount),0) AS total").
		Where("partner_id = ?", partnerID).Group("status").Scan(&rs).Error
	if err != nil {
		return nil, err
	}
	out := map[string]int64{}
	for _, r := range rs {
		out[r.Status] = r.Total
	}
	return out, nil
}

func SetCommissionStatus(ctx context.Context, id, status string) error {
	res := database.DB.WithContext(ctx).Model(&models.PartnerCommission{}).
		Where("id = ?", id).
		Updates(map[string]any{"status": status, "updated_at": gorm.Expr("now()")})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrPartnerCommissionNotFound
	}
	return nil
}

// ClawbackReferralCommissions menandai komisi 'held'/'approved' sebuah referral
// jadi 'clawed_back' (langganan berhenti dalam masa clawback). Komisi yang sudah
// 'paid' diperhitungkan sebagai pengurang di pencairan berikutnya.
func ClawbackReferralCommissions(ctx context.Context, referralID string) (int64, error) {
	res := database.DB.WithContext(ctx).Model(&models.PartnerCommission{}).
		Where("referral_id = ? AND status IN ('held','approved')", referralID).
		Updates(map[string]any{"status": "clawed_back", "updated_at": gorm.Expr("now()")})
	return res.RowsAffected, res.Error
}

// RevokedReferralIDsForPartner mengembalikan referral mitra yang atribusinya
// dicabut (dipakai menghitung clawback pada pencairan).
func RevokedReferralIDsForPartner(ctx context.Context, partnerID string) ([]string, error) {
	var ids []string
	err := database.DB.WithContext(ctx).Model(&models.PartnerReferral{}).
		Where("partner_id = ? AND status = 'revoked'", partnerID).
		Pluck("id", &ids).Error
	return ids, err
}

// PaidCommissionsToClawback menjumlahkan komisi yang SUDAH DIBAYAR pada referral
// yang dicabut dan belum pernah diperhitungkan di pencairan mana pun.
func PaidCommissionsToClawback(ctx context.Context, referralIDs []string) (int64, error) {
	if len(referralIDs) == 0 {
		return 0, nil
	}
	var total int64
	err := database.DB.WithContext(ctx).Model(&models.PartnerCommission{}).
		Where("referral_id IN ? AND status = 'paid'", referralIDs).
		Select("COALESCE(SUM(amount),0)").Scan(&total).Error
	return total, err
}

// MarkPaidCommissionsClawedBack menutup komisi 'paid' yang sudah dikurangkan di
// sebuah pencairan agar tidak dihitung dua kali pada pencairan berikutnya.
func MarkPaidCommissionsClawedBack(ctx context.Context, tx *gorm.DB, referralIDs []string) error {
	if len(referralIDs) == 0 {
		return nil
	}
	db := database.DB
	if tx != nil {
		db = tx
	}
	return db.WithContext(ctx).Model(&models.PartnerCommission{}).
		Where("referral_id IN ? AND status = 'paid'", referralIDs).
		Updates(map[string]any{"status": "clawed_back", "updated_at": gorm.Expr("now()")}).Error
}

// ── Payout ───────────────────────────────────────────────────────────────

func ListApprovedCommissionsForPayout(ctx context.Context, partnerID, from, to string) ([]models.PartnerCommission, error) {
	var rows []models.PartnerCommission
	err := database.DB.WithContext(ctx).
		Where("partner_id = ? AND status = 'approved' AND payout_id IS NULL AND period_month BETWEEN date_trunc('month', ?::date) AND ?::date",
			partnerID, from, to).
		Find(&rows).Error
	return rows, err
}

func CreatePartnerPayout(ctx context.Context, tx *gorm.DB, p *models.PartnerPayout) error {
	db := database.DB
	if tx != nil {
		db = tx
	}
	return db.WithContext(ctx).Create(p).Error
}

func AssignCommissionsToPayout(ctx context.Context, tx *gorm.DB, payoutID string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	db := database.DB
	if tx != nil {
		db = tx
	}
	return db.WithContext(ctx).Model(&models.PartnerCommission{}).
		Where("id IN ?", ids).
		Updates(map[string]any{"status": "paid", "payout_id": payoutID, "updated_at": gorm.Expr("now()")}).Error
}

func ListPartnerPayouts(ctx context.Context, partnerID string) ([]models.PartnerPayout, error) {
	var rows []models.PartnerPayout
	err := database.DB.WithContext(ctx).
		Where("partner_id = ?", partnerID).Order("period_start DESC").Find(&rows).Error
	return rows, err
}

func FindPartnerPayout(ctx context.Context, id string) (models.PartnerPayout, error) {
	var p models.PartnerPayout
	err := database.DB.WithContext(ctx).First(&p, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return p, ErrPartnerPayoutNotFound
	}
	return p, err
}

// MarkPartnerPayoutPaid menandai pencairan 'draft'/'approved' → 'paid' dengan
// bukti transfer & bukti potong pajak.
func MarkPartnerPayoutPaid(ctx context.Context, id, proofURL, taxSlipURL string) error {
	res := database.DB.WithContext(ctx).Model(&models.PartnerPayout{}).
		Where("id = ? AND status IN ('draft','approved')", id).
		Updates(map[string]any{
			"status": "paid", "transfer_proof_url": proofURL, "tax_slip_url": taxSlipURL,
			"paid_at": time.Now().UTC(), "updated_at": gorm.Expr("now()"),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrPartnerPayoutNotFound
	}
	return nil
}

// ── Tampilan merchant TERBATAS untuk portal mitra (blueprint G.8) ───────

// MerchantStatusRow adalah SATU-SATUNYA data merchant yang boleh dilihat mitra:
// nama usaha, status langganan, tanggal jatuh tempo. TIDAK ada omzet, produk,
// harga, pelanggan, atau transaksi.
type MerchantStatusRow struct {
	TenantID     string     `gorm:"column:tenant_id"`
	BusinessName string     `gorm:"column:business_name"`
	SubStatus    string     `gorm:"column:sub_status"`
	PeriodEnd    *time.Time `gorm:"column:period_end"`
}

// MerchantStatusForTenants mengambil baris terbatas itu untuk sekumpulan tenant.
// SELECT hanya 4 kolom yang diizinkan — tidak menyentuh tabel operasional.
func MerchantStatusForTenants(ctx context.Context, tenantIDs []string) (map[string]MerchantStatusRow, error) {
	out := map[string]MerchantStatusRow{}
	if len(tenantIDs) == 0 {
		return out, nil
	}
	var rows []MerchantStatusRow
	err := database.DB.WithContext(ctx).
		Table("tenants t").
		Select("t.id AS tenant_id, t.business_name AS business_name, COALESCE(s.status, 'none') AS sub_status, s.current_period_end AS period_end").
		Joins("LEFT JOIN subscriptions s ON s.tenant_id = t.id").
		Where("t.id IN ?", tenantIDs).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.TenantID] = r
	}
	return out, nil
}
