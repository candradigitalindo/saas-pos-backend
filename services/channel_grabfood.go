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
	"candra/backend-api/models"
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
			"Paling mudah: kelola menu di POS (Atur Menu → Kirim Menu) dan isi Get Menu URL dari layar ini di konsol Grab — ID item otomatis sama dengan barang toko. Bila menu tetap dikelola di Grab, isi External ID tiap menu dengan SKU barang.",
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
			"Menu bisa dikirim dari POS; item yang stoknya habis otomatis ditutup dan jumlah stoknya ikut dikirim",
			"Tandai pesanan siap diambil dari rincian pesanan",
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
		// Grab mengambil menu dari sini setelah "Kirim Menu" di POS.
		{Label: "Get Menu URL", Value: alamat + "/merchant/menu"},
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
		Modifiers  []struct {
			ID string `json:"id"`
		} `json:"modifiers"`
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
		// Harga item sudah termasuk modifier; modifier = pilihan varian yang
		// dikirim bersama menu dari POS.
		ni := NormalizedItem{SKU: sku, Qty: qty, UnitPrice: &h}
		for _, m := range it.Modifiers {
			if id := strings.TrimSpace(m.ID); id != "" {
				ni.VariantSKU = id
				break
			}
		}
		ev.Items = append(ev.Items, ni)
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

// MarkReady: POST /partner/v1/orders/mark {orderID, markStatus: 1} — Grab
// memberi tahu pengemudi pesanan siap diambil. Balasan sukses tanpa isi.
func (grabfoodAdapter) MarkReady(ctx context.Context, cred ChannelCredentials, co models.ChannelOrder) error {
	token, err := grabToken(ctx, cred)
	if err != nil {
		return err
	}
	api, _ := grabURL(cred)
	b, _ := json.Marshal(map[string]any{"orderID": co.ExternalOrderID, "markStatus": 1})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, api+"/partner/v1/orders/mark", bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	res, err := httpKanal.Do(req)
	if err != nil {
		return fmt.Errorf("tidak bisa menghubungi Grab: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode/100 == 2 {
		return nil
	}
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 64<<10))
	var g struct {
		Message string `json:"message"`
		Reason  string `json:"reason"`
	}
	_ = json.Unmarshal(raw, &g)
	return fmt.Errorf("Grab menolak: %s", firstNonEmpty(g.Message, g.Reason, fmt.Sprintf("HTTP %d", res.StatusCode)))
}

// ── Menu & stok ────────────────────────────────────────────────────────────

// PublishMenu: Update menu notification — Grab lalu mengambil menu dari
// {webhook}/merchant/menu (ServeMenu) secara asinkron.
func (grabfoodAdapter) PublishMenu(ctx context.Context, cred ChannelCredentials, _ MenuKanal) error {
	kode, raw, err := grabPanggil(ctx, cred, http.MethodPost, "/partner/v1/merchant/menu/notification",
		map[string]string{"merchantID": strings.TrimSpace(cred["merchant_id"])})
	if err != nil {
		return err
	}
	switch {
	case kode/100 == 2:
		return nil
	case kode == http.StatusConflict:
		return fmt.Errorf("Grab menolak: menu baru saja dikirim — coba lagi 2 menit lagi")
	case kode == http.StatusForbidden:
		return fmt.Errorf("Grab menolak: integrasi toko belum aktif (%s)", pesanGrab(raw, kode))
	}
	return fmt.Errorf("Grab menolak: %s", pesanGrab(raw, kode))
}

func (grabfoodAdapter) MenuAksi(aksi string) bool { return aksi == "merchant/menu" }

func (a grabfoodAdapter) CekAksesMenu(h http.Header, cred ChannelCredentials) bool {
	return a.VerifySignature(h, nil, cred, "") == nil
}

// ServeMenu: Get food menu webhook — satu waktu jual sepanjang hari (jam buka
// toko diatur di GrabMerchant), harga dalam satuan minor (eksponen 2).
func (grabfoodAdapter) ServeMenu(q url.Values, cred ChannelCredentials, menu MenuKanal) any {
	hari := map[string]any{}
	for _, h := range []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"} {
		hari[h] = map[string]string{"openPeriodType": "OpenAllDay"}
	}
	const waktuJual = "sepanjang-hari"
	kategori := make([]map[string]any, 0, len(menu.Kategori))
	for i, k := range menu.Kategori {
		items := make([]map[string]any, 0, len(k.Item))
		for j, it := range k.Item {
			m := map[string]any{"id": it.ID, "name": it.Nama, "price": it.Harga * 100, "sequence": j + 1}
			grabStatus(m, it.Tersedia, it.Stok)
			if it.Foto != "" {
				m["photos"] = []string{it.Foto}
			}
			if it.Deskripsi != "" {
				m["description"] = it.Deskripsi
			}
			if len(it.Varian) > 0 {
				mods := make([]map[string]any, 0, len(it.Varian))
				for n, v := range it.Varian {
					status := "AVAILABLE"
					if !v.Tersedia {
						status = "UNAVAILABLE"
					}
					mods = append(mods, map[string]any{
						"id": v.ID, "name": v.Nama, "availableStatus": status, "price": v.Tambahan * 100, "sequence": n + 1,
					})
				}
				m["modifierGroups"] = []any{map[string]any{
					"id": "MG-" + it.ID, "name": "Pilihan", "availableStatus": "AVAILABLE",
					"selectionRangeMin": 1, "selectionRangeMax": 1, "sequence": 1, "modifiers": mods,
				}}
			}
			items = append(items, m)
		}
		kategori = append(kategori, map[string]any{
			"id": k.ID, "name": k.Nama, "availableStatus": "AVAILABLE", "sellingTimeID": waktuJual,
			"sequence": i + 1, "items": items,
		})
	}
	return map[string]any{
		"merchantID":        firstNonEmpty(q.Get("merchantID"), cred["merchant_id"]),
		"partnerMerchantID": q.Get("partnerMerchantID"),
		"currency":          map[string]any{"code": "IDR", "symbol": "Rp", "exponent": 2},
		"sellingTimes":      []any{map[string]any{"id": waktuJual, "name": "Sepanjang hari", "serviceHours": hari}},
		"categories":        kategori,
	}
}

// grabStatus: UNAVAILABLE wajib disertai maxStock 0; stok yang dilacak
// dikirim sebagai maxStock (berkurang sendiri tiap ada pesanan di Grab).
func grabStatus(m map[string]any, tersedia bool, stok *int64) {
	if !tersedia {
		m["availableStatus"], m["maxStock"] = "UNAVAILABLE", 0
		return
	}
	m["availableStatus"] = "AVAILABLE"
	if stok != nil {
		m["maxStock"] = min(*stok, 9_999_999)
	}
}

// PushStock: Batch Update Menu (field ITEM) — ketersediaan + maxStock.
func (grabfoodAdapter) PushStock(ctx context.Context, cred ChannelCredentials, stok []StokKirim) map[string]error {
	galat := map[string]error{}
	entitas := make([]map[string]any, 0, len(stok))
	for _, s := range stok {
		m := map[string]any{"id": s.Ref}
		var jumlah *int64
		if s.Lacak {
			q := s.Qty
			jumlah = &q
		}
		grabStatus(m, s.Tersedia, jumlah)
		entitas = append(entitas, m)
	}
	kode, raw, err := grabPanggil(ctx, cred, http.MethodPut, "/partner/v1/batch/menu", map[string]any{
		"merchantID": strings.TrimSpace(cred["merchant_id"]), "field": "ITEM", "menuEntities": entitas,
	})
	if err == nil && kode/100 != 2 {
		err = fmt.Errorf("Grab menolak stok: %s", pesanGrab(raw, kode))
	}
	if err != nil {
		for _, s := range stok {
			galat[s.Ref] = err
		}
		return galat
	}
	var r struct {
		Errors []struct {
			ID     string `json:"id"`
			Errors []struct {
				Message string `json:"message"`
			} `json:"errors"`
			Message string `json:"message"`
		} `json:"errors"`
	}
	_ = json.Unmarshal(raw, &r)
	for _, e := range r.Errors {
		pesan := e.Message
		if len(e.Errors) > 0 && e.Errors[0].Message != "" {
			pesan = e.Errors[0].Message
		}
		galat[e.ID] = fmt.Errorf("Grab menolak stok: %s", firstNonEmpty(pesan, "alasan tidak disebut"))
	}
	return galat
}

// grabPanggil: API Grab dengan token client credentials.
func grabPanggil(ctx context.Context, cred ChannelCredentials, method, jalur string, badan any) (int, []byte, error) {
	token, err := grabToken(ctx, cred)
	if err != nil {
		return 0, nil, err
	}
	api, _ := grabURL(cred)
	b, _ := json.Marshal(badan)
	req, err := http.NewRequestWithContext(ctx, method, api+jalur, bytes.NewReader(b))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	res, err := httpKanal.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("tidak bisa menghubungi Grab: %w", err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 256<<10))
	return res.StatusCode, raw, nil
}

func pesanGrab(raw []byte, kode int) string {
	var g struct {
		Message string `json:"message"`
		Reason  string `json:"reason"`
	}
	_ = json.Unmarshal(raw, &g)
	return firstNonEmpty(g.Message, g.Reason, fmt.Sprintf("HTTP %d", kode))
}
