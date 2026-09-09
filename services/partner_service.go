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

// Layanan Program Mitra Penjual (Fase 12, blueprint Bagian G).
//
// Modul PLATFORM: mitra bukan tenant, masuk lewat realm auth terpisah, dan tidak
// pernah bisa melihat data operasional tenant mana pun (blueprint G.8). Panel
// internal (buat/verifikasi mitra, jalankan komisi, cairkan) belum punya realm
// admin platform sendiri — untuk sekarang dijalankan lewat cmd/partner-admin &
// cmd/partner-commissions, dan fungsi servicenya diekspor agar bisa diuji.

const partnerTimeLayout = "2006-01-02 15:04:05"

// ── Auth mitra ───────────────────────────────────────────────────────────

// PartnerLogin memverifikasi kredensial akun mitra dan menerbitkan access token
// ber-realm "partner" (ditolak di semua rute tenant).
func PartnerLogin(ctx context.Context, username, password string) (structs.PartnerAuthResponse, error) {
	var out structs.PartnerAuthResponse
	pu, err := repositories.FindPartnerUserByUsername(ctx, strings.TrimSpace(username))
	if err != nil {
		return out, fmt.Errorf("%w: username atau password salah", helpers.ErrUnauthorized)
	}
	if !pu.IsActive || helpers.CheckPassword(password, pu.PasswordHash) != nil {
		return out, fmt.Errorf("%w: username atau password salah", helpers.ErrUnauthorized)
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
			ID: pu.ID, Name: pu.Name, Email: pu.Email, Username: pu.Username,
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
		ContactPhone:         strings.TrimSpace(in.ContactPhone),
		City:                 strings.TrimSpace(in.City),
		BusinessType:         strings.TrimSpace(in.BusinessType),
		Status:               "baru",
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
// Setiap panggilan dicatat di jejak audit.
func ListPartnerMerchants(ctx context.Context) ([]structs.PartnerMerchantResponse, error) {
	pid := reqctx.PartnerID(ctx)
	refs, err := repositories.ListReferralsByPartner(ctx, pid)
	if err != nil {
		return nil, err
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
	out := make([]structs.PartnerMerchantResponse, 0, len(refs))
	for _, r := range refs {
		st := status[r.TenantID]
		_ = repositories.LogPartnerMerchantAccess(ctx, pid, puid, r.TenantID, "view_merchant_status")
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
	rate, err := decimal.NewFromString(strings.TrimSpace(in.CommissionRate))
	if err != nil || rate.IsNegative() {
		return structs.PartnerTierResponse{}, fmt.Errorf("%w: commission_rate tidak valid", helpers.ErrValidation)
	}
	recurring := true
	if in.Recurring != nil {
		recurring = *in.Recurring
	}
	if !recurring && in.OneTimeMonths <= 0 {
		return structs.PartnerTierResponse{}, fmt.Errorf("%w: one_time_months wajib > 0 bila tidak berulang", helpers.ErrValidation)
	}
	t := models.PartnerTier{
		Code: strings.TrimSpace(in.Code), Name: strings.TrimSpace(in.Name), Kind: in.Kind,
		CommissionRate: rate, Recurring: recurring, OneTimeMonths: in.OneTimeMonths,
		ActivationMinTxn: orInt(in.ActivationMinTxn, 30), ActivationMinDays: orInt(in.ActivationMinDays, 30),
		AttributionDays: orInt(in.AttributionDays, 60), ClawbackDays: orInt(in.ClawbackDays, 90),
		IsActive: true,
	}
	if err := repositories.CreatePartnerTier(ctx, &t); err != nil {
		if helpers.IsDuplicateEntryError(err) {
			return structs.PartnerTierResponse{}, fmt.Errorf("%w: kode tingkat sudah ada", helpers.ErrConflict)
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
	tier, err := repositories.FindPartnerTierByCode(ctx, strings.TrimSpace(in.TierCode))
	if err != nil {
		return out, fmt.Errorf("%w: tingkat %q tidak dikenal", helpers.ErrValidation, in.TierCode)
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
		Region: strings.TrimSpace(in.Region), ReferralCode: code, Status: "pending",
		BankAccount: strings.TrimSpace(in.BankAccount), TaxID: strings.TrimSpace(in.TaxID),
		TaxWithholdingRate: twr,
	}
	if err := repositories.CreatePartner(ctx, &partner); err != nil {
		if helpers.IsDuplicateEntryError(err) {
			return out, fmt.Errorf("%w: kode referral sudah dipakai", helpers.ErrConflict)
		}
		return out, err
	}

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
		PartnerID: partner.ID, Name: strings.TrimSpace(in.UserName),
		Email:    strings.ToLower(strings.TrimSpace(in.UserEmail)),
		Username: strings.TrimSpace(in.UserUsername), PasswordHash: hash, IsActive: true,
	}
	if err := repositories.CreatePartnerUser(ctx, &pu); err != nil {
		if helpers.IsDuplicateEntryError(err) {
			return out, fmt.Errorf("%w: username akun mitra sudah dipakai", helpers.ErrConflict)
		}
		return out, err
	}

	out = structs.PartnerCreateResponse{
		Partner: partnerToResponse(partner), UserUsername: pu.Username, GeneratedPassword: generated,
	}
	return out, nil
}

// ApprovePartner memverifikasi mitra: pending → active, joined_at = hari ini
// (blueprint G.6 langkah 2 & 4).
func ApprovePartner(ctx context.Context, id string) error {
	partner, err := repositories.FindPartnerByID(ctx, id)
	if err != nil {
		return err
	}
	if partner.Status == "active" {
		return nil
	}
	if partner.Status != "pending" {
		return fmt.Errorf("%w: mitra berstatus %s tidak bisa disetujui", helpers.ErrConflict, partner.Status)
	}
	if err := repositories.SetPartnerStatus(ctx, id, "active"); err != nil {
		return err
	}
	return repositories.SetPartnerJoinedAt(ctx, id, time.Now().UTC())
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
		ID: p.ID, Kind: p.Kind, Name: p.Name, Region: p.Region,
		ReferralCode: p.ReferralCode, Status: p.Status,
	}
	if p.Tier != nil {
		r.TierCode = p.Tier.Code
	}
	if p.JoinedAt != nil {
		r.JoinedAt = p.JoinedAt.Format("2006-01-02")
	}
	return r
}

func tierToResponse(t models.PartnerTier) structs.PartnerTierResponse {
	return structs.PartnerTierResponse{
		ID: t.ID, Code: t.Code, Name: t.Name, Kind: t.Kind,
		CommissionRate: t.CommissionRate.String(), Recurring: t.Recurring,
		OneTimeMonths: t.OneTimeMonths, ActivationMinTxn: t.ActivationMinTxn,
		ActivationMinDays: t.ActivationMinDays, AttributionDays: t.AttributionDays,
		ClawbackDays: t.ClawbackDays, IsActive: t.IsActive,
	}
}

func leadToResponse(l models.PartnerLead) structs.PartnerLeadResponse {
	r := structs.PartnerLeadResponse{
		ID: l.ID, BusinessName: l.BusinessName, ContactName: l.ContactName,
		ContactPhone: l.ContactPhone, City: l.City, Status: l.Status,
		AttributionExpiresAt: l.AttributionExpiresAt.Format("2006-01-02"),
		CreatedAt:            l.CreatedAt.Format(partnerTimeLayout),
	}
	if l.RegisteredTenantID != nil {
		r.RegisteredTenantID = *l.RegisteredTenantID
	}
	return r
}

func partnerCommissionToResponse(c models.PartnerCommission) structs.PartnerCommissionResponse {
	r := structs.PartnerCommissionResponse{
		ID: c.ID, TenantID: c.TenantID, SubscriptionInvoiceID: c.SubscriptionInvoiceID,
		PeriodStart: c.PeriodStart.Format("2006-01-02"), PeriodEnd: c.PeriodEnd.Format("2006-01-02"),
		BaseAmount: c.BaseAmount, Rate: c.Rate.String(), Amount: c.Amount, Status: c.Status,
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
		NetAmount: p.NetAmount, Status: p.Status, TransferProof: p.TransferProof,
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

// generateReferralCode membuat kode acak 8 karakter alfanumerik huruf besar.
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
