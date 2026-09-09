package services

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"time"

	"candra/backend-api/helpers"
	"candra/backend-api/internal/reqctx"
	"candra/backend-api/models"
	"candra/backend-api/repositories"
	"candra/backend-api/structs"

	"github.com/shopspring/decimal"
)

// Layanan Program Mitra Penjual (Fase 12, §5.12, blueprint Bagian G).
//
// Modul PLATFORM: mitra bukan tenant, masuk lewat realm auth terpisah, dan tidak
// pernah bisa melihat data operasional tenant mana pun (blueprint G.8). Panel
// internal (buat/verifikasi mitra, jalankan komisi, cairkan) belum punya realm
// admin platform sendiri — untuk sekarang dijalankan lewat cmd/partner-admin &
// cmd/partner-commissions, dan fungsi servicenya diekspor agar bisa diuji.

const partnerTimeLayout = "2006-01-02 15:04:05"

// ── Auth mitra ───────────────────────────────────────────────────────────

// PartnerLogin memverifikasi kredensial akun mitra (EMAIL + password) dan
// menerbitkan access token ber-realm "partner" (ditolak di rute tenant).
func PartnerLogin(ctx context.Context, email, password string) (structs.PartnerAuthResponse, error) {
	var out structs.PartnerAuthResponse
	pu, err := repositories.FindPartnerUserByEmail(ctx, strings.ToLower(strings.TrimSpace(email)))
	if err != nil {
		return out, fmt.Errorf("%w: email atau password salah", helpers.ErrUnauthorized)
	}
	if !pu.IsActive || helpers.CheckPassword(password, pu.PasswordHash) != nil {
		return out, fmt.Errorf("%w: email atau password salah", helpers.ErrUnauthorized)
	}
	partner, err := repositories.FindPartnerByID(ctx, pu.PartnerID)
	if err != nil {
		return out, err
	}
	if partner.Status != "active" {
		return out, fmt.Errorf("%w: mitra belum aktif", helpers.ErrForbidden)
	}

	token, expiresAt, err := helpers.GeneratePartnerAccessToken(pu.ID)
	if err != nil {
		return out, err
	}
	_ = repositories.TouchPartnerUserLogin(ctx, pu.ID)

	out = structs.PartnerAuthResponse{
		AccessToken: token,
		ExpiresAt:   expiresAt.Format(partnerTimeLayout),
		Partner:     partnerToResponse(partner),
		User: structs.PartnerUserResponse{
			ID: pu.ID, Name: pu.Name, Email: pu.Email, Phone: pu.Phone,
		},
	}
	return out, nil
}

// ── Portal mitra (realm partner) ────────────────────────────────────────

// PartnerProfile mengembalikan profil mitra permintaan ini.
func PartnerProfile(ctx context.Context) (structs.PartnerResponse, error) {
	partner, err := repositories.FindPartnerByID(ctx, reqctx.PartnerID(ctx))
	if err != nil {
		return structs.PartnerResponse{}, err
	}
	return partnerToResponse(partner), nil
}

// PartnerDashboard merangkum prospek, merchant aktif, komisi, dan pencairan
// terakhir — "kapan cair dan berapa" (blueprint G.4).
func PartnerDashboard(ctx context.Context) (structs.PartnerDashboardResponse, error) {
	pid := reqctx.PartnerID(ctx)
	var out structs.PartnerDashboardResponse
	out.LeadsByStatus = map[string]int{}

	leads, err := repositories.ListPartnerLeads(ctx, pid, "")
	if err != nil {
		return out, err
	}
	for _, l := range leads {
		out.LeadsByStatus[l.Status]++
	}

	merchants, err := ListPartnerMerchants(ctx)
	if err != nil {
		return out, err
	}
	out.MerchantsTotal = len(merchants)
	for _, m := range merchants {
		if m.IsActive {
			out.MerchantsActive++
		}
	}

	sums, err := repositories.SumPartnerCommissionsByStatus(ctx, pid)
	if err != nil {
		return out, err
	}
	out.CommissionHeld = sums["held"]
	out.CommissionApproved = sums["approved"]
	out.CommissionPaid = sums["paid"]

	payouts, err := repositories.ListPartnerPayouts(ctx, pid)
	if err != nil {
		return out, err
	}
	if len(payouts) > 0 {
		r := payoutToResponse(payouts[0])
		out.LastPayout = &r
	}
	return out, nil
}

// CreatePartnerLead mendaftarkan prospek baru milik mitra permintaan ini. Masa
// atribusi = sekarang + tier.attribution_days (blueprint G.2 #5).
func CreatePartnerLead(ctx context.Context, in structs.PartnerLeadRequest) (structs.PartnerLeadResponse, error) {
	pid := reqctx.PartnerID(ctx)
	partner, err := repositories.FindPartnerByID(ctx, pid)
	if err != nil {
		return structs.PartnerLeadResponse{}, err
	}
	days := 60
	if partner.Tier != nil && partner.Tier.AttributionDays > 0 {
		days = partner.Tier.AttributionDays
	}
	lead := models.PartnerLead{
		PartnerID:            pid,
		BusinessName:         strings.TrimSpace(in.BusinessName),
		ContactName:          strings.TrimSpace(in.ContactName),
		Phone:                strings.TrimSpace(in.Phone),
		City:                 strings.TrimSpace(in.City),
		BusinessType:         strings.TrimSpace(in.BusinessType),
		Status:               "new",
		AttributionExpiresAt: time.Now().UTC().AddDate(0, 0, days),
		Note:                 strings.TrimSpace(in.Note),
	}
	if err := repositories.CreatePartnerLead(ctx, &lead); err != nil {
		return structs.PartnerLeadResponse{}, err
	}
	return leadToResponse(lead), nil
}

// ListPartnerLeads mengembalikan prospek milik mitra permintaan ini.
func ListPartnerLeads(ctx context.Context, status string) ([]structs.PartnerLeadResponse, error) {
	rows, err := repositories.ListPartnerLeads(ctx, reqctx.PartnerID(ctx), status)
	if err != nil {
		return nil, err
	}
	out := make([]structs.PartnerLeadResponse, len(rows))
	for i := range rows {
		out[i] = leadToResponse(rows[i])
	}
	return out, nil
}

// ListPartnerMerchants mengembalikan merchant binaan mitra dengan TAMPILAN
// TERBATAS (blueprint G.8): nama usaha, status langganan, jatuh tempo, aktif.
//
// Setiap pembukaan dicatat di `audit_logs` — SATU insert batch untuk seluruh
// daftar, bukan satu per merchant. Kegagalan menulis audit membuat permintaan
// GAGAL: akses data merchant yang tidak tercatat melanggar G.8.
func ListPartnerMerchants(ctx context.Context) ([]structs.PartnerMerchantResponse, error) {
	pid := reqctx.PartnerID(ctx)
	refs, err := repositories.ListReferralsByPartner(ctx, pid)
	if err != nil {
		return nil, err
	}
	if len(refs) == 0 {
		return []structs.PartnerMerchantResponse{}, nil
	}

	ids := make([]string, 0, len(refs))
	for _, r := range refs {
		ids = append(ids, r.TenantID)
	}
	status, err := repositories.MerchantStatusForTenants(ctx, ids)
	if err != nil {
		return nil, err
	}

	puid := reqctx.PartnerUserID(ctx)
	entries := make([]repositories.AuditEntry, 0, len(refs))
	out := make([]structs.PartnerMerchantResponse, 0, len(refs))
	for _, r := range refs {
		st := status[r.TenantID]
		tenantID, targetTable := r.TenantID, "tenants"
		entries = append(entries, repositories.AuditEntry{
			TenantID: &tenantID, ActorType: "partner_user", ActorID: &puid,
			Action: "partner.merchant.view", TargetTable: &targetTable, TargetID: &tenantID,
		})
		m := structs.PartnerMerchantResponse{
			TenantID:           r.TenantID,
			BusinessName:       st.BusinessName,
			SubscriptionStatus: st.SubStatus,
			IsActive:           st.SubStatus == "active" || st.SubStatus == "trial",
			AttributedAt:       r.AttributedAt.Format(partnerTimeLayout),
			Activated:          r.ActivatedAt != nil,
		}
		if st.PeriodEnd != nil {
			m.CurrentPeriodEnd = st.PeriodEnd.Format("2006-01-02")
		}
		out = append(out, m)
	}
	if err := repositories.WriteAuditLogs(ctx, entries); err != nil {
		return nil, fmt.Errorf("gagal mencatat jejak audit akses merchant: %w", err)
	}
	return out, nil
}

// ListPartnerCommissions mengembalikan komisi milik mitra permintaan ini.
func ListPartnerCommissions(ctx context.Context, status, from, to string) ([]structs.PartnerCommissionResponse, error) {
	rows, err := repositories.ListPartnerCommissions(ctx, reqctx.PartnerID(ctx), status, from, to)
	if err != nil {
		return nil, err
	}
	out := make([]structs.PartnerCommissionResponse, len(rows))
	for i := range rows {
		out[i] = partnerCommissionToResponse(rows[i])
	}
	return out, nil
}

// ListPartnerPayouts mengembalikan riwayat pencairan mitra permintaan ini.
func ListPartnerPayouts(ctx context.Context) ([]structs.PartnerPayoutResponse, error) {
	rows, err := repositories.ListPartnerPayouts(ctx, reqctx.PartnerID(ctx))
	if err != nil {
		return nil, err
	}
	out := make([]structs.PartnerPayoutResponse, len(rows))
	for i := range rows {
		out[i] = payoutToResponse(rows[i])
	}
	return out, nil
}

// ── Panel internal (dipanggil cmd/partner-admin & test) ────────────────

// CreatePartnerTier membuat tingkat mitra baru.
func CreatePartnerTier(ctx context.Context, in structs.PartnerTierRequest) (structs.PartnerTierResponse, error) {
	if strings.TrimSpace(in.Name) == "" {
		return structs.PartnerTierResponse{}, fmt.Errorf("%w: name wajib diisi", helpers.ErrValidation)
	}
	if !contains(models.PartnerKinds, in.Kind) {
		return structs.PartnerTierResponse{}, fmt.Errorf("%w: kind harus salah satu dari %v", helpers.ErrValidation, models.PartnerKinds)
	}
	rate, err := decimal.NewFromString(strings.TrimSpace(in.RecurringRate))
	if err != nil || rate.IsNegative() {
		return structs.PartnerTierResponse{}, fmt.Errorf("%w: recurring_rate tidak valid", helpers.ErrValidation)
	}
	if in.RecurringMonths != nil && *in.RecurringMonths <= 0 {
		return structs.PartnerTierResponse{}, fmt.Errorf("%w: recurring_months harus > 0 atau kosong", helpers.ErrValidation)
	}
	t := models.PartnerTier{
		Name: strings.TrimSpace(in.Name), Kind: in.Kind,
		RecurringRate: rate, RecurringMonths: in.RecurringMonths,
		ActivationBonus: in.ActivationBonus, MinActiveMerchants: in.MinActiveMerchants,
		ActivationMinTxn: orInt(in.ActivationMinTxn, 30), ActivationMinDays: orInt(in.ActivationMinDays, 30),
		AttributionDays: orInt(in.AttributionDays, 60), ClawbackDays: orInt(in.ClawbackDays, 90),
	}
	if err := repositories.CreatePartnerTier(ctx, &t); err != nil {
		if helpers.IsDuplicateEntryError(err) {
			return structs.PartnerTierResponse{}, fmt.Errorf("%w: nama tingkat sudah ada", helpers.ErrConflict)
		}
		return structs.PartnerTierResponse{}, err
	}
	return tierToResponse(t), nil
}

// ListPartnerTiers mengembalikan seluruh tingkat.
func ListPartnerTiers(ctx context.Context) ([]structs.PartnerTierResponse, error) {
	rows, err := repositories.ListPartnerTiers(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]structs.PartnerTierResponse, len(rows))
	for i := range rows {
		out[i] = tierToResponse(rows[i])
	}
	return out, nil
}

// CreatePartner membuat mitra + akun login pertamanya. Referral code & password
// dibuatkan bila kosong; keduanya dikembalikan SEKALI.
func CreatePartner(ctx context.Context, in structs.PartnerCreateRequest) (structs.PartnerCreateResponse, error) {
	var out structs.PartnerCreateResponse

	// Divalidasi di SINI, bukan hanya lewat tag binding: service ini juga
	// dipanggil langsung oleh cmd/partner-admin dan test, yang tidak melewati
	// validator HTTP. `phone` NOT NULL di §5.12 dan dipakai mencocokkan prospek
	// saat merchant mendaftar — mitra tanpa nomor telepon tidak berguna.
	for _, f := range []struct{ name, val string }{
		{"name", in.Name}, {"phone", in.Phone},
		{"user_name", in.UserName}, {"user_email", in.UserEmail},
	} {
		if strings.TrimSpace(f.val) == "" {
			return out, fmt.Errorf("%w: %s wajib diisi", helpers.ErrValidation, f.name)
		}
	}
	if !strings.Contains(in.UserEmail, "@") {
		return out, fmt.Errorf("%w: user_email tidak valid", helpers.ErrValidation)
	}

	tier, err := repositories.FindPartnerTierByName(ctx, strings.TrimSpace(in.TierName))
	if err != nil {
		return out, fmt.Errorf("%w: tingkat %q tidak dikenal", helpers.ErrValidation, in.TierName)
	}
	twr := decimal.Zero
	if s := strings.TrimSpace(in.TaxWithholdingRate); s != "" {
		twr, err = decimal.NewFromString(s)
		if err != nil || twr.IsNegative() {
			return out, fmt.Errorf("%w: tax_withholding_rate tidak valid", helpers.ErrValidation)
		}
	}
	code := strings.ToUpper(strings.TrimSpace(in.ReferralCode))
	if code == "" {
		code = generateReferralCode()
	}
	partner := models.Partner{
		TierID: tier.ID, Kind: tier.Kind, Name: strings.TrimSpace(in.Name),
		Phone: strings.TrimSpace(in.Phone), Email: strings.ToLower(strings.TrimSpace(in.Email)),
		Region: strings.TrimSpace(in.Region), ReferralCode: code, Status: "pending",
		IDNumber: strings.TrimSpace(in.IDNumber), NPWP: strings.TrimSpace(in.NPWP),
		BankName: strings.TrimSpace(in.BankName), BankAccountNo: strings.TrimSpace(in.BankAccountNo),
		BankAccountName: strings.TrimSpace(in.BankAccountName), TaxWithholdingRate: twr,
	}
	if err := repositories.CreatePartner(ctx, &partner); err != nil {
		if helpers.IsDuplicateEntryError(err) {
			return out, fmt.Errorf("%w: kode referral sudah dipakai", helpers.ErrConflict)
		}
		return out, err
	}
	partner.Tier = &tier

	password := strings.TrimSpace(in.UserPassword)
	generated := ""
	if password == "" {
		password = generatePassword()
		generated = password
	}
	hash, err := helpers.HashPassword(password)
	if err != nil {
		return out, err
	}
	pu := models.PartnerUser{
		PartnerID:    partner.ID,
		Name:         strings.TrimSpace(in.UserName),
		Email:        strings.ToLower(strings.TrimSpace(in.UserEmail)),
		Phone:        strings.TrimSpace(in.UserPhone),
		PasswordHash: hash,
		IsActive:     true,
	}
	if err := repositories.CreatePartnerUser(ctx, &pu); err != nil {
		if helpers.IsDuplicateEntryError(err) {
			return out, fmt.Errorf("%w: email akun mitra sudah dipakai", helpers.ErrConflict)
		}
		return out, err
	}

	out = structs.PartnerCreateResponse{
		Partner: partnerToResponse(partner), UserEmail: pu.Email, GeneratedPassword: generated,
	}
	return out, nil
}

// ApprovePartner memverifikasi mitra: pending/verified → active, verified_at &
// joined_at diisi (blueprint G.6 langkah 2 & 4).
func ApprovePartner(ctx context.Context, id string) error {
	partner, err := repositories.FindPartnerByID(ctx, id)
	if err != nil {
		return err
	}
	if partner.Status == "active" {
		return nil
	}
	if partner.Status != "pending" && partner.Status != "verified" {
		return fmt.Errorf("%w: mitra berstatus %s tidak bisa disetujui", helpers.ErrConflict, partner.Status)
	}
	return repositories.ActivatePartner(ctx, id, time.Now().UTC())
}

// SuspendPartner menonaktifkan mitra (active → suspended).
func SuspendPartner(ctx context.Context, id string) error {
	return repositories.SetPartnerStatus(ctx, id, "suspended")
}

// ListPartners mengembalikan mitra (panel internal).
func ListPartners(ctx context.Context, status string, limit, offset int) ([]structs.PartnerResponse, int64, error) {
	rows, total, err := repositories.ListPartners(ctx, status, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	out := make([]structs.PartnerResponse, len(rows))
	for i := range rows {
		out[i] = partnerToResponse(rows[i])
	}
	return out, total, nil
}

// ── DTO & util ──────────────────────────────────────────────────────────

func partnerToResponse(p models.Partner) structs.PartnerResponse {
	r := structs.PartnerResponse{
		ID: p.ID, Kind: p.Kind, Name: p.Name, Phone: p.Phone, Email: p.Email,
		Region: p.Region, ReferralCode: p.ReferralCode, Status: p.Status,
	}
	if p.Tier != nil {
		r.TierName = p.Tier.Name
	}
	if p.VerifiedAt != nil {
		r.VerifiedAt = p.VerifiedAt.Format(partnerTimeLayout)
	}
	if p.JoinedAt != nil {
		r.JoinedAt = p.JoinedAt.Format(partnerTimeLayout)
	}
	return r
}

func tierToResponse(t models.PartnerTier) structs.PartnerTierResponse {
	return structs.PartnerTierResponse{
		ID: t.ID, Name: t.Name, Kind: t.Kind,
		RecurringRate: t.RecurringRate.String(), RecurringMonths: t.RecurringMonths,
		ActivationBonus: t.ActivationBonus, MinActiveMerchants: t.MinActiveMerchants,
		ActivationMinTxn: t.ActivationMinTxn, ActivationMinDays: t.ActivationMinDays,
		AttributionDays: t.AttributionDays, ClawbackDays: t.ClawbackDays,
	}
}

func leadToResponse(l models.PartnerLead) structs.PartnerLeadResponse {
	r := structs.PartnerLeadResponse{
		ID: l.ID, BusinessName: l.BusinessName, ContactName: l.ContactName,
		Phone: l.Phone, City: l.City, Status: l.Status,
		AttributionExpiresAt: l.AttributionExpiresAt.Format("2006-01-02"),
		CreatedAt:            l.CreatedAt.Format(partnerTimeLayout),
	}
	if l.ConvertedTenantID != nil {
		r.ConvertedTenantID = *l.ConvertedTenantID
	}
	return r
}

func partnerCommissionToResponse(c models.PartnerCommission) structs.PartnerCommissionResponse {
	r := structs.PartnerCommissionResponse{
		ID: c.ID, ReferralID: c.ReferralID, SubscriptionInvoiceID: c.SubscriptionInvoiceID,
		PeriodMonth: c.PeriodMonth.Format("2006-01-02"),
		BaseAmount:  c.BaseAmount, Rate: c.Rate.String(), Amount: c.Amount, Status: c.Status,
	}
	if c.PayoutID != nil {
		r.PayoutID = *c.PayoutID
	}
	return r
}

func payoutToResponse(p models.PartnerPayout) structs.PartnerPayoutResponse {
	r := structs.PartnerPayoutResponse{
		ID: p.ID, PeriodStart: p.PeriodStart.Format("2006-01-02"), PeriodEnd: p.PeriodEnd.Format("2006-01-02"),
		GrossAmount: p.GrossAmount, ClawbackAmount: p.ClawbackAmount, TaxAmount: p.TaxAmount,
		NetAmount: p.NetAmount, Status: p.Status,
		TransferProofURL: p.TransferProofURL, TaxSlipURL: p.TaxSlipURL,
	}
	if p.PaidAt != nil {
		r.PaidAt = p.PaidAt.Format(partnerTimeLayout)
	}
	return r
}

func orInt(v, def int) int {
	if v <= 0 {
		return def
	}
	return v
}

// generateReferralCode membuat kode acak alfanumerik huruf besar.
func generateReferralCode() string { return "MTR-" + randCode(6) }

func generatePassword() string { return randCode(12) }

func randCode(n int) string {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // tanpa I/O/0/1 (mudah keliru)
	b := make([]byte, n)
	for i := range b {
		idx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		b[i] = alphabet[idx.Int64()]
	}
	return string(b)
}
