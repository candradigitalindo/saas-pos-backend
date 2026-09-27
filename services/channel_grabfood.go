package services

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"candra/backend-api/config"
	"candra/backend-api/structs"

	"github.com/shopspring/decimal"
)

// Adaptor GrabFood — Partner API (developer.grab.com, SDK resmi grab/grabfood-api-sdk-go).
//
// Arahnya berkebalikan dengan GoFood: GRAB yang memanggil "server partner".
//   1. Grab meminta token ke endpoint OAuth partner (PartnerOauthRequest:
//      client_id/client_secret/grant_type/scope) — di sini sub-jalur
//      /oauth/token di bawah alamat webhook kanal, dengan client ID & secret
//      yang DIBUAT platform untuk kanal itu dan ditempel tenant di konsol Grab.
//   2. Grab mengirim pesanan (SubmitOrderRequest) & perubahan status
//      (OrderStateRequest) dengan Authorization: Bearer <token tadi>.
// Token dibuat tanpa disimpan: "gf1.<kedaluwarsa>.<hex HMAC(partner secret)>",
// jadi memverifikasinya cukup menghitung ulang tanda tangannya.
//
// Ke arah Grab, platform memakai kredensial Partner API milik tenant (client
// credentials, scope food.partner_api) — dipakai tes koneksi (status toko).
//
// Harga Grab dalam MINOR UNIT dengan currency.exponent (2 untuk Indonesia:
// 2000000 = Rp 20.000). Penjualan dicatat saat pesanan masuk (tenant memakai
// terima otomatis); CANCELLED/FAILED membatalkannya.

type grabfoodAdapter struct{}

const grabTokenAwalan = "gf1."

func (grabfoodAdapter) Info() structs.ChannelProviderInfo {
	return structs.ChannelProviderInfo{
		Code: "grabfood", Name: "GrabFood", Kind: "delivery_app", Available: true,
		DocsURL:      "https://developer.grab.com/docs/grabfood/",
		WebhookLabel: "Order URL (Submit Order & Push Order State)",
		Steps: []string{
			"Daftarkan integrasi GrabFood toko di Grab Developer dan minta kredensial Partner API (Client ID & Client Secret) untuk Merchant ID toko Anda.",
			"Isi Client ID, Client Secret, dan Merchant ID di sini, pilih Sandbox atau Produksi, lalu Simpan & Tes.",
			"Di konsol Grab, isi Partner OAuth URL beserta Partner Client ID & Secret dari layar ini, lalu arahkan endpoint Submit Order dan Push Order State ke Order URL.",
			"Aktifkan terima pesanan otomatis (auto-accept) — pesanan dicatat saat masuk, dan dibatalkan bila Grab membatalkannya.",
			"Isi External ID tiap menu di Grab dengan SKU barang toko — atau petakan ID item Grab di pemetaan SKU kanal — supaya stok ikut terpotong.",
		},
		Fields: []structs.ChannelProviderField{
			{Key: "client_id", Label: "Client ID (dari Grab)"},
			{Key: "client_secret", Label: "Client Secret (dari Grab)", Secret: true},
			{Key: "merchant_id", Label: "Merchant ID GrabFood", Help: "ID toko di GrabFood, mis. 1-C3JXXXXXXX."},
			{Key: "environment", Label: "Lingkungan", Optional: true, Options: []structs.LabelValue{
				{Value: "production", Label: "Produksi"}, {Value: "sandbox", Label: "Sandbox (uji)"},
			}},
		},
		Capabilities: []string{
			"Pesanan GrabFood masuk otomatis sebagai penjualan kanal",
			"Pembatalan atau pesanan gagal dari Grab membatalkan penjualan dan mengembalikan stok",
			"Status pengemudi (ditugaskan, tiba, diambil, terkirim) ikut tercatat",
			"Grab memanggil server ini dengan token OAuth khusus kanal, bukan alamat terbuka",
		},
	}
}

func acakHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (grabfoodAdapter) Prepare(cred ChannelCredentials) {
	if cred["environment"] != "sandbox" {
		cred["environment"] = "production"
	}
	// Kredensial yang dipakai GRAB untuk memanggil server ini — dibuat
	// platform, ditempel tenant di konsol Grab.
	if cred["partner_client_id"] == "" {
		cred["partner_client_id"] = "pos-" + acakHex(8)
	}
	if cred["partner_client_secret"] == "" {
		cred["partner_client_secret"] = acakHex(24)
	}
}

func (grabfoodAdapter) MerchantRef(cred ChannelCredentials) string {
	return strings.TrimSpace(cred["merchant_id"])
}

func (grabfoodAdapter) WebhookValues(cred ChannelCredentials, alamat string) []structs.LabelValue {
	return []structs.LabelValue{
		{Label: "Partner OAuth URL", Value: alamat + "/oauth/token"},
		{Label: "Partner Client ID", Value: cred["partner_client_id"]},
		{Label: "Partner Client Secret", Value: cred["partner_client_secret"]},
	}
}

func (grabfoodAdapter) VerifyChallenge(url.Values, ChannelCredentials) (string, bool) {
	return "", false
}

// grabTokenBaru membuat token berumur `umur` yang bisa diverifikasi tanpa disimpan.
func grabTokenBaru(secret string, umur time.Duration) string {
	isi := grabTokenAwalan + strconv.FormatInt(time.Now().Add(umur).Unix(), 10)
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(isi))
	return isi + "." + hex.EncodeToString(m.Sum(nil))
}

func grabTokenSah(token, secret string) bool {
	i := strings.LastIndex(token, ".")
	if secret == "" || i <= len(grabTokenAwalan) || !strings.HasPrefix(token, grabTokenAwalan) {
		return false
	}
	isi, tanda := token[:i], token[i+1:]
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(isi))
	want, err := hex.DecodeString(tanda)
	if err != nil || !hmac.Equal(m.Sum(nil), want) {
		return false
	}
	exp, err := strconv.ParseInt(strings.TrimPrefix(isi, grabTokenAwalan), 10, 64)
	return err == nil && time.Now().Unix() < exp
}

// WebhookAction menangani POST .../oauth/token: Grab menukar Partner Client
// ID & Secret (buatan platform) dengan token untuk memanggil Order URL.
func (grabfoodAdapter) WebhookAction(_ context.Context, p WebhookPermintaan, cred ChannelCredentials) (WebhookBalasan, bool) {
	if p.Metode != http.MethodPost || p.Aksi != "oauth/token" {
		return WebhookBalasan{}, false
	}
	body := p.Body
	var req struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
		GrantType    string `json:"grant_type"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		// Beberapa klien OAuth mengirim form; terima keduanya.
		if q, perr := url.ParseQuery(string(body)); perr == nil {
			req.ClientID, req.ClientSecret, req.GrantType = q.Get("client_id"), q.Get("client_secret"), q.Get("grant_type")
		}
	}
	idCocok := subtle.ConstantTimeCompare([]byte(req.ClientID), []byte(cred["partner_client_id"])) == 1
	rahasiaCocok := subtle.ConstantTimeCompare([]byte(req.ClientSecret), []byte(cred["partner_client_secret"])) == 1
	if !idCocok || !rahasiaCocok || cred["partner_client_secret"] == "" {
		return WebhookBalasan{Status: http.StatusUnauthorized, Body: map[string]string{
			"error": "invalid_client", "error_description": "client_id/client_secret tidak cocok",
		}}, true
	}
	if req.GrantType != "" && req.GrantType != "client_credentials" {
		return WebhookBalasan{Status: http.StatusBadRequest, Body: map[string]string{"error": "unsupported_grant_type"}}, true
	}
	const umur = time.Hour
	return WebhookBalasan{Status: http.StatusOK, Body: map[string]any{
		"access_token": grabTokenBaru(cred["partner_client_secret"], umur),
		"token_type":   "Bearer",
		"expires_in":   int(umur.Seconds()),
	}}, true
}

func (grabfoodAdapter) VerifySignature(h http.Header, _ []byte, cred ChannelCredentials, _ string) error {
	auth := strings.TrimSpace(h.Get("Authorization"))
	if len(auth) < 7 || !strings.EqualFold(auth[:7], "bearer ") || !grabTokenSah(strings.TrimSpace(auth[7:]), cred["partner_client_secret"]) {
		return errTandaTangan
	}
	return nil
}

// Satu bentuk untuk SubmitOrderRequest & OrderStateRequest — dibedakan dari
// isinya (pesanan punya items; perubahan status punya state).
type grabWebhook struct {
	OrderID          string `json:"orderID"`
	ShortOrderNumber string `json:"shortOrderNumber"`
	MerchantID       string `json:"merchantID"`
	OrderTime        string `json:"orderTime"`
	Currency         struct {
		Code     string `json:"code"`
		Exponent int32  `json:"exponent"`
	} `json:"currency"`
	Items []struct {
		ID         string      `json:"id"`
		GrabItemID string      `json:"grabItemID"`
		Quantity   json.Number `json:"quantity"`
		Price      json.Number `json:"price"`
	} `json:"items"`
	Receiver *struct {
		Name    string `json:"name"`
		Phones  string `json:"phones"`
		Address *struct {
			Address string `json:"address"`
		} `json:"address"`
	} `json:"receiver"`
	State   string `json:"state"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (grabfoodAdapter) Events(body []byte, cred ChannelCredentials) ([]NormalizedEvent, error) {
	var w grabWebhook
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	if err := dec.Decode(&w); err != nil {
		return nil, fmt.Errorf("json tidak valid: %w", err)
	}
	if w.OrderID == "" {
		return nil, nil
	}
	if m := strings.TrimSpace(cred["merchant_id"]); m != "" && w.MerchantID != "" && w.MerchantID != m {
		return nil, nil // toko lain di aplikasi yang sama
	}

	if w.State != "" && len(w.Items) == 0 {
		ev := NormalizedEvent{ExternalOrderID: w.OrderID, IgnoreIfMissing: true}
		switch strings.ToUpper(w.State) {
		case "CANCELLED", "FAILED":
			ev.EventType = "order.canceled"
			ev.Reason = strings.TrimSpace("GrabFood: " + firstNonEmpty(w.Message, w.Code, strings.ToLower(w.State)))
		default:
			ev.EventType, ev.ExternalStatus = "order.status", strings.ToLower(w.State)
		}
		return []NormalizedEvent{ev}, nil
	}

	if cur := strings.ToUpper(w.Currency.Code); cur != "" && cur != "IDR" {
		return nil, fmt.Errorf("mata uang %s belum didukung (hanya IDR)", cur)
	}
	pembagi := decimal.New(1, w.Currency.Exponent) // minor unit → rupiah
	ev := NormalizedEvent{
		EventType: "order.created", ExternalOrderID: w.OrderID,
		Courier: strings.TrimSpace("GrabFood · #" + w.ShortOrderNumber),
	}
	if w.ShortOrderNumber == "" {
		ev.Courier = "GrabFood"
	}
	if w.Receiver != nil {
		ev.BuyerName, ev.BuyerPhone = w.Receiver.Name, w.Receiver.Phones
		if w.Receiver.Address != nil {
			ev.ShippingAddress = w.Receiver.Address.Address
		}
	}
	if t, err := time.Parse(time.RFC3339, w.OrderTime); err == nil {
		tt := t.UTC()
		ev.OccurredAt = &tt
	}
	for _, it := range w.Items {
		qty, err := decimal.NewFromString(it.Quantity.String())
		if err != nil {
			return nil, fmt.Errorf("jumlah %q bukan angka", it.Quantity)
		}
		minor, err := decimal.NewFromString(it.Price.String())
		if err != nil {
			return nil, fmt.Errorf("harga %q bukan angka", it.Price)
		}
		h := minor.Div(pembagi).Round(0).IntPart()
		// ID = External ID menu di Grab (diisi toko = SKU barang); tanpa itu ID
		// item Grab (tanpa akhiran "#…" varian modifier) untuk pemetaan SKU kanal.
		sku := strings.TrimSpace(it.ID)
		if sku == "" {
			sku, _, _ = strings.Cut(strings.TrimSpace(it.GrabItemID), "#")
		}
		ev.Items = append(ev.Items, NormalizedItem{SKU: sku, Qty: qty, UnitPrice: &h})
	}
	return []NormalizedEvent{ev}, nil
}

// Alamat Grab; bisa diarahkan ke server tiruan saat pengujian.
func grabURL(cred ChannelCredentials) (api, oauth string) {
	api = "https://partner-api.grab.com/grabfood"
	if cred["environment"] == "sandbox" {
		api = "https://partner-api.grab.com/grabfood-sandbox"
	}
	api = strings.TrimRight(config.GetEnv("CHANNEL_GRAB_API_URL", api), "/")
	oauth = strings.TrimRight(config.GetEnv("CHANNEL_GRAB_OAUTH_URL", "https://api.grab.com"), "/")
	return api, oauth
}

func grabToken(ctx context.Context, cred ChannelCredentials) (string, error) {
	_, oauth := grabURL(cred)
	b, _ := json.Marshal(map[string]string{
		"client_id": cred["client_id"], "client_secret": cred["client_secret"],
		"grant_type": "client_credentials", "scope": "food.partner_api",
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, oauth+"/grabid/v1/oauth2/token", bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := httpKanal.Do(req)
	if err != nil {
		return "", fmt.Errorf("tidak bisa menghubungi Grab: %w", err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 64<<10))
	var t struct {
		AccessToken      string `json:"access_token"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	_ = json.Unmarshal(raw, &t)
	if res.StatusCode != http.StatusOK || t.AccessToken == "" {
		pesan := firstNonEmpty(t.ErrorDescription, t.Error, fmt.Sprintf("HTTP %d", res.StatusCode))
		return "", fmt.Errorf("Grab menolak Client ID/Secret: %s", pesan)
	}
	return t.AccessToken, nil
}

// Test: token berhasil + status toko terbaca = kredensial & Merchant ID cocok.
func (grabfoodAdapter) Test(ctx context.Context, cred ChannelCredentials) (string, error) {
	token, err := grabToken(ctx, cred)
	if err != nil {
		return "", err
	}
	api, _ := grabURL(cred)
	merchant := strings.TrimSpace(cred["merchant_id"])
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		api+"/partner/v1/merchants/"+url.PathEscape(merchant)+"/store/status", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := httpKanal.Do(req)
	if err != nil {
		return "", fmt.Errorf("tidak bisa menghubungi Grab: %w", err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 64<<10))
	var s struct {
		IsOpen      bool   `json:"isOpen"`
		CloseReason string `json:"closeReason"`
		Message     string `json:"message"`
	}
	_ = json.Unmarshal(raw, &s)
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("toko %s tidak bisa dibaca: %s", merchant, firstNonEmpty(s.Message, fmt.Sprintf("HTTP %d", res.StatusCode)))
	}
	status := "toko buka"
	if !s.IsOpen {
		status = strings.TrimSpace("toko tutup " + s.CloseReason)
	}
	return "GrabFood " + merchant + " · " + status, nil
}
