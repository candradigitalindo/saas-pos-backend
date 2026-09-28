package services

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"candra/backend-api/config"
	"candra/backend-api/structs"

	"github.com/shopspring/decimal"
)

// Adaptor Tokopedia & Shop — TikTok Shop Open API (partner.tokopedia.com,
// dokumen resmi). Sejak penggabungan, toko Tokopedia dan TikTok Shop di
// Indonesia memakai SATU set API (Tokopedia Open API lama dihentikan
// 30 Sep 2025); pesanan membawa commerce_platform TOKOPEDIA / TIKTOK_SHOP.
// Aturan merek platform di Indonesia: layanan pihak ketiga memakai nama
// "Tokopedia & Shop", bukan "TikTok" — begitu pula layar & kode penyedia ini.
//
// Toko memakai Custom App MILIKNYA sendiri (seller developer: App Key, App
// Secret, Service ID), lalu memberi izin lewat peramban:
//   - Tautan izin: {authorize}?service_id&state → penjual kembali ke Redirect URL
//     aplikasi = {webhook}/oauth/callback?code&state (code sekali pakai, 30 menit).
//   - Token: GET {auth}/api/v2/token/get?app_key&app_secret&auth_code&grant_type=authorized_code
//     → access token (±7 hari; kedaluwarsa berupa unix timestamp) + refresh
//     token; diperbarui lewat /api/v2/token/refresh dengan baris kanal terkunci.
//   - API: query app_key, timestamp, shop_cipher, sign + header x-tts-access-token;
//     sign = hex(HMAC-SHA256(secret, secret+path+{k}{v} terurut+body+secret)),
//     tanpa sign & access_token.
//   - Webhook didaftarkan PER TOKO lewat API (PUT /event/202309/webhooks);
//     header Authorization = hex(HMAC-SHA256(app_secret, app_key+body)).
//   - Webhook status pesanan hanya membawa nomor & status — rinciannya (Get
//     Order Detail) diambil PEKERJA. Tidak ada kolom jumlah: SATU line_item =
//     SATU unit, jadi baris ber-SKU & harga sama dijumlahkan.

type tokopediaAdapter struct{}

// Kunci kredensial hasil otorisasi (bukan isian tenant).
var tokopediaState = []string{
	"shop_id", "shop_name", "shop_cipher", "shop_region", "seller_name",
	"access_token", "refresh_token", "access_expires", "auth_state",
}

// Topik webhook yang didaftarkan otomatis untuk toko yang memberi izin.
var tokopediaTopik = []string{"ORDER_STATUS_CHANGE", "CANCELLATION_STATUS_CHANGE", "SELLER_DEAUTHORIZATION"}

func (tokopediaAdapter) Info() structs.ChannelProviderInfo {
	return structs.ChannelProviderInfo{
		Code: "tokopedia", Name: "Tokopedia & Shop", Kind: "marketplace", Available: true, RequiresAuthorization: true,
		DocsURL:      "https://partner.tokopedia.com/docv2/page/6789f743b59cf903096fac16",
		WebhookLabel: "Webhook URL",
		Steps: []string{
			"Masuk ke Partner Center (partner.tokopedia.com) dengan akun PEMILIK toko dan daftar sebagai seller developer. Syarat dari platform: toko lolos verifikasi (KYC/KYB) dan sudah punya Account Manager.",
			"Buat Custom App, lalu salin App Key, App Secret, dan Service ID dari halaman detail aplikasinya. Isi di sini, lalu Simpan.",
			"Di pengaturan API aplikasi, isi Redirect URL dari layar ini dan aktifkan cakupan API pesanan & webhook.",
			"Tekan Otorisasi Toko dan setujui dengan akun penjual. Webhook pesanan didaftarkan otomatis setelahnya (butuh alamat HTTPS server).",
			"Samakan Seller SKU produk dengan SKU barang toko — atau petakan di pemetaan SKU kanal — supaya stok ikut terpotong.",
		},
		Fields: []structs.ChannelProviderField{
			{Key: "app_key", Label: "App Key"},
			{Key: "app_secret", Label: "App Secret", Secret: true},
			{Key: "service_id", Label: "Service ID", Help: "Ada di halaman detail aplikasi, juga di ujung tautan otorisasinya (…?service_id=…)."},
		},
		Capabilities: []string{
			"Pesanan yang sudah dibayar masuk otomatis sebagai penjualan kanal — dari aplikasi mana pun pembeli berbelanja",
			"Pembatalan membatalkan penjualan dan mengembalikan stok",
			"Status pengiriman ikut tercatat",
			"Webhook didaftarkan & token diperbarui otomatis — toko cukup memberi izin sekali",
			"Stok toko dikirim ke Tokopedia & Shop setiap berubah (barang yang sudah dicocokkan)",
		},
	}
}

func (tokopediaAdapter) Prepare(ChannelCredentials) {}

func (tokopediaAdapter) StateKeys() []string { return tokopediaState }

func (tokopediaAdapter) MerchantRef(cred ChannelCredentials) string {
	return strings.TrimSpace(cred["app_key"]) + ":" + strings.TrimSpace(cred["shop_id"])
}

// WebhookValues: Redirect URL diisi tenant di pengaturan aplikasinya. Webhook
// URL tidak wajib ditempel — didaftarkan lewat API setelah otorisasi.
func (tokopediaAdapter) WebhookValues(_ ChannelCredentials, alamat string) []structs.LabelValue {
	return []structs.LabelValue{{Label: "Redirect URL", Value: alamat + "/oauth/callback"}}
}

func (tokopediaAdapter) VerifyChallenge(url.Values, ChannelCredentials) (string, bool) {
	return "", false
}

func (tokopediaAdapter) Diotorisasi(cred ChannelCredentials) string {
	if cred["refresh_token"] == "" {
		return ""
	}
	return firstNonEmpty(cred["shop_name"], cred["seller_name"], "toko "+cred["shop_id"])
}

// tokopediaURL: host API, host token, dan halaman izin; bisa diarahkan ke
// tiruan saat uji.
func tokopediaURL() (api, token, izin string) {
	return strings.TrimRight(config.GetEnv("CHANNEL_TOKOPEDIA_API_URL", "https://open-api.tiktokglobalshop.com"), "/"),
		strings.TrimRight(config.GetEnv("CHANNEL_TOKOPEDIA_TOKEN_URL", "https://auth.tiktok-shops.com"), "/"),
		config.GetEnv("CHANNEL_TOKOPEDIA_AUTH_URL", "https://services.tiktokshop.com/open/authorize")
}

// tokopediaSign: tanda tangan panggilan API (dokumen "Sign your API request").
func tokopediaSign(secret, path string, q url.Values, body []byte) string {
	kunci := make([]string, 0, len(q))
	for k := range q {
		if k != "sign" && k != "access_token" {
			kunci = append(kunci, k)
		}
	}
	sort.Strings(kunci)
	var b strings.Builder
	b.WriteString(secret)
	b.WriteString(path)
	for _, k := range kunci {
		b.WriteString(k)
		b.WriteString(q.Get(k))
	}
	b.Write(body)
	b.WriteString(secret)
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(b.String()))
	return hex.EncodeToString(m.Sum(nil))
}

func (tokopediaAdapter) AuthorizeURL(cred ChannelCredentials, _ string) string {
	// Redirect URL diatur di aplikasi, bukan di tautan.
	cred["auth_state"] = acakHex(16)
	_, _, izin := tokopediaURL()
	return izin + "?" + url.Values{"service_id": {cred["service_id"]}, "state": {cred["auth_state"]}}.Encode()
}

func (tokopediaAdapter) VerifySignature(h http.Header, body []byte, cred ChannelCredentials, _ string) error {
	got := strings.ToLower(strings.TrimSpace(h.Get("Authorization")))
	m := hmac.New(sha256.New, []byte(cred["app_secret"]))
	m.Write([]byte(cred["app_key"]))
	m.Write(body)
	want := hex.EncodeToString(m.Sum(nil))
	if got == "" || cred["app_secret"] == "" || subtle.ConstantTimeCompare([]byte(got), []byte(want)) != 1 {
		return errTandaTangan
	}
	return nil
}

// teksJSON: nilai JSON string ATAU angka sebagai teks (shop_id, order_id).
func teksJSON(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var n json.Number
	if json.Unmarshal(raw, &n) == nil {
		return n.String()
	}
	return ""
}

type tokopediaPush struct {
	Type   json.Number     `json:"type"`
	ShopID json.RawMessage `json:"shop_id"`
	Data   struct {
		OrderID      json.RawMessage `json:"order_id"`
		OrderStatus  string          `json:"order_status"`
		CancelStatus string          `json:"cancel_status"`
		UpdateTime   json.Number     `json:"update_time"`
		CreateTime   json.Number     `json:"create_time"`
		Message      string          `json:"message"`
	} `json:"data"`
}

func (tokopediaAdapter) urai(body []byte, cred ChannelCredentials) (tokopediaPush, bool, error) {
	var p tokopediaPush
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	if err := dec.Decode(&p); err != nil {
		return p, false, fmt.Errorf("json tidak valid: %w", err)
	}
	// Webhook toko lain (satu aplikasi bisa dipakai lebih dari satu toko) diabaikan.
	toko := teksJSON(p.ShopID)
	cocok := cred["shop_id"] == "" || toko == "" || toko == cred["shop_id"]
	return p, cocok, nil
}

// IzinDicabut: SELLER_DEAUTHORIZATION (type 6) — toko mencabut izin aplikasi.
func (a tokopediaAdapter) IzinDicabut(body []byte, cred ChannelCredentials) (string, bool) {
	p, cocok, err := a.urai(body, cred)
	if err != nil || !cocok || p.Type.String() != "6" {
		return "", false
	}
	return "Toko mencabut izin aplikasi di Seller Center — tekan Otorisasi Toko untuk menyambungkan lagi.", true
}

func (a tokopediaAdapter) Events(body []byte, cred ChannelCredentials) ([]NormalizedEvent, error) {
	p, cocok, err := a.urai(body, cred)
	if err != nil {
		return nil, err
	}
	pesanan := teksJSON(p.Data.OrderID)
	if !cocok || pesanan == "" {
		return nil, nil
	}
	dasar := NormalizedEvent{ExternalOrderID: pesanan}
	waktu := p.Data.UpdateTime
	if waktu == "" {
		waktu = p.Data.CreateTime
	}
	if sec, err := waktu.Int64(); err == nil && sec > 0 {
		t := time.Unix(sec, 0).UTC()
		dasar.OccurredAt = &t
	}
	status := func(s string) NormalizedEvent {
		ev := dasar
		ev.EventType, ev.ExternalStatus, ev.IgnoreIfMissing = "order.status", s, true
		return ev
	}
	batal := func() []NormalizedEvent {
		ev := dasar
		ev.EventType, ev.Reason, ev.IgnoreIfMissing = "order.canceled", "Tokopedia & Shop: pesanan dibatalkan", true
		return []NormalizedEvent{ev}
	}
	switch p.Type.String() {
	case "1": // ORDER_STATUS_CHANGE
		s := strings.ToUpper(p.Data.OrderStatus)
		switch s {
		case "", "UNPAID", "ON_HOLD":
			// Belum dibayar / masih bisa dibatalkan pembeli tanpa persetujuan
			// toko — belum menjadi penjualan.
			return nil, nil
		case "CANCEL", "CANCELLED":
			return batal(), nil
		default:
			// AWAITING_SHIPMENT dan seterusnya. Pencatatan idempoten, jadi
			// status berikutnya menjadi cadangan bila push sebelumnya terlewat.
			buat := dasar
			buat.EventType, buat.NeedsFetch = "order.created", true
			return []NormalizedEvent{buat, status(strings.ToLower(s))}, nil
		}
	case "11": // CANCELLATION_STATUS_CHANGE
		switch strings.ToUpper(p.Data.CancelStatus) {
		case "CANCELLATION_REQUEST_SUCCESS", "CANCELLATION_REQUEST_COMPLETE":
			return batal(), nil
		case "CANCELLATION_REQUEST_PENDING":
			return []NormalizedEvent{status("cancel_requested")}, nil
		case "CANCELLATION_REQUEST_CANCELLED":
			return []NormalizedEvent{status("cancel_request_closed")}, nil
		}
	}
	return nil, nil
}

// panggil: API toko (shop_cipher) atau API aplikasi (daftar toko). Galat
// memuat pesan asli platform supaya tenant tahu isian mana yang salah.
func (tokopediaAdapter) panggil(ctx context.Context, cred ChannelCredentials, method, path string, query url.Values, badan any, toko bool) (json.RawMessage, error) {
	api, _, _ := tokopediaURL()
	if query == nil {
		query = url.Values{}
	}
	query.Set("app_key", cred["app_key"])
	query.Set("timestamp", strconv.FormatInt(time.Now().Unix(), 10))
	if toko {
		query.Set("shop_cipher", cred["shop_cipher"])
	}
	var raw []byte
	if badan != nil {
		b, err := json.Marshal(badan)
		if err != nil {
			return nil, err
		}
		raw = b
	}
	query.Set("sign", tokopediaSign(cred["app_secret"], path, query, raw))
	req, err := http.NewRequestWithContext(ctx, method, api+path+"?"+query.Encode(), bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-tts-access-token", cred["access_token"])
	return tokopediaBalasan(req)
}

func tokopediaBalasan(req *http.Request) (json.RawMessage, error) {
	res, err := httpKanal.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tidak bisa menghubungi Tokopedia & Shop: %w", err)
	}
	defer res.Body.Close()
	isi, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	var out struct {
		Code    *int            `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	dec := json.NewDecoder(bytes.NewReader(isi))
	dec.UseNumber()
	_ = dec.Decode(&out)
	if res.StatusCode != http.StatusOK || out.Code == nil || *out.Code != 0 {
		pesan := firstNonEmpty(out.Message, fmt.Sprintf("HTTP %d", res.StatusCode))
		if out.Code != nil && *out.Code != 0 {
			pesan += fmt.Sprintf(" (kode %d)", *out.Code)
		}
		return nil, fmt.Errorf("Tokopedia & Shop menolak: %s", pesan)
	}
	return out.Data, nil
}

// token menukar kode izin (token/get) atau refresh token (token/refresh).
func (tokopediaAdapter) token(ctx context.Context, cred ChannelCredentials, jalur string, q url.Values) error {
	_, base, _ := tokopediaURL()
	q.Set("app_key", cred["app_key"])
	q.Set("app_secret", cred["app_secret"])
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+jalur+"?"+q.Encode(), nil)
	if err != nil {
		return err
	}
	data, err := tokopediaBalasan(req)
	if err != nil {
		return err
	}
	var t struct {
		AccessToken  string      `json:"access_token"`
		AccessExpire json.Number `json:"access_token_expire_in"`
		RefreshToken string      `json:"refresh_token"`
		SellerName   string      `json:"seller_name"`
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&t); err != nil || t.AccessToken == "" {
		return fmt.Errorf("Tokopedia & Shop tidak mengirim token")
	}
	// access_token_expire_in adalah WAKTU kedaluwarsa (unix), bukan durasi.
	exp, err := t.AccessExpire.Int64()
	if err != nil || exp <= time.Now().Unix() {
		exp = time.Now().Add(7 * 24 * time.Hour).Unix()
	}
	cred["access_token"], cred["access_expires"] = t.AccessToken, strconv.FormatInt(exp, 10)
	if t.RefreshToken != "" {
		cred["refresh_token"] = t.RefreshToken
	}
	if t.SellerName != "" {
		cred["seller_name"] = t.SellerName
	}
	return nil
}

// pastikanToken memperbarui access token bila (hampir) kedaluwarsa. Dipanggil
// dengan baris kanal terkunci.
func (a tokopediaAdapter) pastikanToken(ctx context.Context, cred ChannelCredentials) error {
	if cred["refresh_token"] == "" {
		return fmt.Errorf("toko belum diotorisasi — tekan Otorisasi Toko")
	}
	if exp, err := strconv.ParseInt(cred["access_expires"], 10, 64); err == nil && cred["access_token"] != "" &&
		time.Now().Unix() < exp-300 {
		return nil
	}
	err := a.token(ctx, cred, "/api/v2/token/refresh",
		url.Values{"refresh_token": {cred["refresh_token"]}, "grant_type": {"refresh_token"}})
	if err != nil {
		return fmt.Errorf("izin toko kedaluwarsa atau dicabut — otorisasi ulang (%v)", err)
	}
	return nil
}

type tokopediaToko struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Region string `json:"region"`
	Cipher string `json:"cipher"`
	Code   string `json:"code"`
}

// toko: toko yang mengizinkan aplikasi ini. shopID kosong → toko Indonesia
// yang pertama (Custom App seller developer hanya untuk toko sendiri).
func (a tokopediaAdapter) toko(ctx context.Context, cred ChannelCredentials, shopID string) (tokopediaToko, error) {
	data, err := a.panggil(ctx, cred, http.MethodGet, "/authorization/202309/shops", nil, nil, false)
	if err != nil {
		return tokopediaToko{}, err
	}
	var r struct {
		Shops []tokopediaToko `json:"shops"`
	}
	_ = json.Unmarshal(data, &r)
	var pilih *tokopediaToko
	for i := range r.Shops {
		t := &r.Shops[i]
		switch {
		case shopID != "":
			if t.ID == shopID {
				return *t, nil
			}
		case strings.EqualFold(t.Region, "ID"):
			return *t, nil
		case pilih == nil:
			pilih = t
		}
	}
	if pilih != nil && shopID == "" {
		return *pilih, nil
	}
	if shopID != "" {
		return tokopediaToko{}, fmt.Errorf("toko %s tidak lagi mengizinkan aplikasi ini — otorisasi ulang", shopID)
	}
	return tokopediaToko{}, fmt.Errorf("akun ini belum memberi izin toko mana pun")
}

func simpanToko(cred ChannelCredentials, t tokopediaToko) {
	cred["shop_id"], cred["shop_cipher"], cred["shop_region"] = t.ID, t.Cipher, t.Region
	if t.Name != "" {
		cred["shop_name"] = t.Name
	}
}

// Test: token (diperbarui bila perlu) + toko masih mengizinkan aplikasi.
func (a tokopediaAdapter) Test(ctx context.Context, cred ChannelCredentials) (string, error) {
	if cred["refresh_token"] == "" {
		return "", fmt.Errorf("kredensial tersimpan — lanjutkan dengan Otorisasi Toko")
	}
	if err := a.pastikanToken(ctx, cred); err != nil {
		return "", err
	}
	t, err := a.toko(ctx, cred, cred["shop_id"])
	if err != nil {
		return "", err
	}
	simpanToko(cred, t)
	info := firstNonEmpty(t.Name, "Toko") + " · " + firstNonEmpty(t.Region, "ID")
	if t.Code != "" {
		info += " · kode " + t.Code
	}
	return info, nil
}

// SubscribeWebhook mendaftarkan alamat kanal untuk topik pesanan, pembatalan,
// dan pencabutan izin di toko yang memberi izin.
func (a tokopediaAdapter) SubscribeWebhook(ctx context.Context, cred ChannelCredentials, alamat string) error {
	if cred["shop_cipher"] == "" {
		return fmt.Errorf("toko belum diotorisasi")
	}
	if err := a.pastikanToken(ctx, cred); err != nil {
		return err
	}
	for _, topik := range tokopediaTopik {
		if _, err := a.panggil(ctx, cred, http.MethodPut, "/event/202309/webhooks", nil,
			map[string]string{"address": alamat, "event_type": topik}, true); err != nil {
			return fmt.Errorf("%s: %w", topik, err)
		}
	}
	return nil
}

// WebhookAction: GET .../oauth/callback — penjual kembali dari halaman izin.
// code ditukar token, toko dicatat, webhook didaftarkan; hasilnya halaman
// singkat di tab peramban itu.
func (a tokopediaAdapter) WebhookAction(ctx context.Context, p WebhookPermintaan, cred ChannelCredentials) (WebhookBalasan, bool) {
	if p.Metode != http.MethodGet || p.Aksi != "oauth/callback" {
		return WebhookBalasan{}, false
	}
	gagal := func(judul, isi string) (WebhookBalasan, bool) {
		return WebhookBalasan{Status: http.StatusBadRequest, HTML: halamanPesan(judul, isi)}, true
	}
	q := p.Query
	if cred["auth_state"] == "" || subtle.ConstantTimeCompare([]byte(q.Get("state")), []byte(cred["auth_state"])) != 1 {
		return gagal("Tautan otorisasi tidak cocok", "Ulangi dari tombol Otorisasi Toko di aplikasi.")
	}
	// Ditolak penjual: code=null&error=auth_denied.
	code := q.Get("code")
	if code == "" || code == "null" || q.Get("error") != "" {
		return gagal("Otorisasi dibatalkan", "Toko belum memberi izin. Ulangi dari aplikasi bila ingin menyambungkan.")
	}
	if err := a.token(ctx, cred, "/api/v2/token/get", url.Values{"auth_code": {code}, "grant_type": {"authorized_code"}}); err != nil {
		return gagal("Otorisasi gagal", err.Error())
	}
	delete(cred, "auth_state")
	t, err := a.toko(ctx, cred, "")
	if err != nil {
		return gagal("Otorisasi gagal", err.Error())
	}
	simpanToko(cred, t)
	catatan := " Pesanan baru akan masuk otomatis."
	delete(cred, "subscribed_url")
	if !strings.HasPrefix(p.Alamat, "https://") {
		catatan = " Webhook pesanan belum didaftarkan: alamat publik HTTPS server (APP_URL) belum diatur."
	} else if err := a.SubscribeWebhook(ctx, cred, p.Alamat); err != nil {
		catatan = " Namun webhook pesanan belum terdaftar (" + err.Error() + ") — tekan Simpan & Tes Ulang di aplikasi."
	} else {
		cred["subscribed_url"] = p.Alamat
	}
	return WebhookBalasan{Status: http.StatusOK, HTML: halamanPesan("Toko Tokopedia & Shop tersambung",
		firstNonEmpty(t.Name, "Toko")+" sudah memberi izin."+catatan+" Tutup tab ini dan kembali ke aplikasi.")}, true
}

type tokopediaPesanan struct {
	ID               string      `json:"id"`
	Status           string      `json:"status"`
	CreateTime       json.Number `json:"create_time"`
	PaidTime         json.Number `json:"paid_time"`
	ShippingProvider string      `json:"shipping_provider"`
	Payment          struct {
		Currency string `json:"currency"`
	} `json:"payment"`
	Recipient struct {
		Name        string `json:"name"`
		Phone       string `json:"phone_number"`
		FullAddress string `json:"full_address"`
	} `json:"recipient_address"`
	LineItems []struct {
		SkuID         string `json:"sku_id"`
		SellerSKU     string `json:"seller_sku"`
		SalePrice     string `json:"sale_price"`
		OriginalPrice string `json:"original_price"`
		Currency      string `json:"currency"`
	} `json:"line_items"`
}

// FetchOrder mengambil rincian pesanan (Get Order Detail) — dipanggil pekerja.
func (a tokopediaAdapter) FetchOrder(ctx context.Context, cred ChannelCredentials, orderID string) (*NormalizedEvent, error) {
	if err := a.pastikanToken(ctx, cred); err != nil {
		return nil, err
	}
	data, err := a.panggil(ctx, cred, http.MethodGet, "/order/202507/orders", url.Values{"ids": {orderID}}, nil, true)
	if err != nil {
		return nil, err
	}
	var r struct {
		Orders []tokopediaPesanan `json:"orders"`
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&r); err != nil || len(r.Orders) == 0 {
		return nil, fmt.Errorf("rincian pesanan %s tidak ada di balasan Tokopedia & Shop", orderID)
	}
	o := r.Orders[0]
	switch strings.ToUpper(o.Status) {
	case "UNPAID", "CANCELLED", "CANCEL":
		return nil, nil
	case "ON_HOLD":
		// Push-nya sudah melewati ON_HOLD; rincian yang tertinggal → coba lagi.
		return nil, fmt.Errorf("pesanan %s masih ON_HOLD di Tokopedia & Shop — dicoba lagi", orderID)
	}
	if cur := strings.ToUpper(o.Payment.Currency); cur != "" && cur != "IDR" {
		return nil, fmt.Errorf("mata uang %s belum didukung (hanya IDR)", cur)
	}
	ev := &NormalizedEvent{
		EventType: "order.created", ExternalOrderID: firstNonEmpty(o.ID, orderID), Courier: o.ShippingProvider,
		BuyerName: o.Recipient.Name, BuyerPhone: o.Recipient.Phone, ShippingAddress: o.Recipient.FullAddress,
	}
	waktu := o.PaidTime
	if waktu == "" || waktu == "0" {
		waktu = o.CreateTime
	}
	if sec, err := waktu.Int64(); err == nil && sec > 0 {
		t := time.Unix(sec, 0).UTC()
		ev.OccurredAt = &t
	}
	// Satu line_item = satu unit: kelompokkan per SKU & harga.
	type kunci struct {
		sku   string
		harga int64
	}
	var urutan []kunci
	jumlah := map[kunci]int64{}
	for _, li := range o.LineItems {
		if cur := strings.ToUpper(li.Currency); cur != "" && cur != "IDR" {
			return nil, fmt.Errorf("mata uang %s belum didukung (hanya IDR)", cur)
		}
		harga, err := decimal.NewFromString(firstNonEmpty(li.SalePrice, li.OriginalPrice))
		if err != nil {
			return nil, fmt.Errorf("harga barang bukan angka")
		}
		// Seller SKU yang diisi penjual = SKU barang toko; tanpa itu ID SKU
		// platform untuk pemetaan SKU kanal.
		k := kunci{sku: firstNonEmpty(li.SellerSKU, li.SkuID), harga: harga.Round(0).IntPart()}
		if k.sku == "" {
			return nil, fmt.Errorf("baris pesanan tanpa SKU")
		}
		if _, ada := jumlah[k]; !ada {
			urutan = append(urutan, k)
		}
		jumlah[k]++
	}
	for _, k := range urutan {
		h := k.harga
		ev.Items = append(ev.Items, NormalizedItem{SKU: k.sku, Qty: decimal.NewFromInt(jumlah[k]), UnitPrice: &h})
	}
	if len(ev.Items) == 0 {
		return nil, fmt.Errorf("pesanan %s tanpa barang", orderID)
	}
	return ev, nil
}

// ── Stok ───────────────────────────────────────────────────────────────────

// Ref stok Tokopedia & Shop: "product_id:sku_id:warehouse_id" (gudang kosong =
// satu gudang default); "*" = SKU di lebih dari satu gudang.
func tokopediaRef(produk, sku, gudang string) string { return produk + ":" + sku + ":" + gudang }

// ListListings: Search Products (semua status selain dihapus), 100 per halaman.
func (a tokopediaAdapter) ListListings(ctx context.Context, cred ChannelCredentials) ([]ListingPenyedia, error) {
	if err := a.pastikanToken(ctx, cred); err != nil {
		return nil, err
	}
	var out []ListingPenyedia
	halaman := ""
	for i := 0; i < 200; i++ {
		q := url.Values{"page_size": {"100"}}
		if halaman != "" {
			q.Set("page_token", halaman)
		}
		data, err := a.panggil(ctx, cred, http.MethodPost, "/product/202502/products/search", q, map[string]string{"status": "ALL"}, true)
		if err != nil {
			return nil, err
		}
		var r struct {
			Products []struct {
				ID     string `json:"id"`
				Title  string `json:"title"`
				Status string `json:"status"`
				Skus   []struct {
					ID        string `json:"id"`
					SellerSKU string `json:"seller_sku"`
					Inventory []struct {
						WarehouseID string `json:"warehouse_id"`
					} `json:"inventory"`
				} `json:"skus"`
			} `json:"products"`
			Next string `json:"next_page_token"`
		}
		if err := json.Unmarshal(data, &r); err != nil {
			return nil, fmt.Errorf("daftar produk Tokopedia & Shop tidak terbaca")
		}
		for _, p := range r.Products {
			switch strings.ToUpper(p.Status) {
			case "DELETED", "FREEZE":
				continue
			}
			for _, s := range p.Skus {
				gudang := ""
				switch len(s.Inventory) {
				case 1:
					gudang = s.Inventory[0].WarehouseID
				case 0:
				default:
					gudang = "*"
				}
				out = append(out, ListingPenyedia{SKU: s.SellerSKU, Ref: tokopediaRef(p.ID, s.ID, gudang), Nama: p.Title})
			}
		}
		if r.Next == "" || len(r.Products) == 0 {
			break
		}
		halaman = r.Next
	}
	return out, nil
}

// PushStock: Update Inventory per produk (SKU satu produk sekali panggil).
// Kode 0 belum tentu semua berhasil — data.errors dibaca per SKU.
func (a tokopediaAdapter) PushStock(ctx context.Context, cred ChannelCredentials, stok []StokKirim) map[string]error {
	galat := map[string]error{}
	if err := a.pastikanToken(ctx, cred); err != nil {
		for _, s := range stok {
			galat[s.Ref] = err
		}
		return galat
	}
	type skuKirim struct {
		ref, sku, gudang string
		qty              int64
	}
	perProduk := map[string][]skuKirim{}
	var urut []string
	for _, s := range stok {
		if tanpaAngka(s) {
			continue
		}
		b := strings.SplitN(s.Ref, ":", 3)
		if len(b) != 3 || b[0] == "" || b[1] == "" {
			galat[s.Ref] = fmt.Errorf("pengenal listing tidak dikenal — cocokkan barang lagi")
			continue
		}
		if b[2] == "*" {
			galat[s.Ref] = fmt.Errorf("SKU ini tersebar di lebih dari satu gudang — atur stoknya di Seller Center")
			continue
		}
		if _, ada := perProduk[b[0]]; !ada {
			urut = append(urut, b[0])
		}
		perProduk[b[0]] = append(perProduk[b[0]], skuKirim{ref: s.Ref, sku: b[1], gudang: b[2], qty: s.Qty})
	}
	for _, pid := range urut {
		var skus []map[string]any
		refSKU := map[string]string{}
		for _, k := range perProduk[pid] {
			inv := map[string]any{"quantity": k.qty}
			if k.gudang != "" {
				inv["warehouse_id"] = k.gudang
			}
			skus = append(skus, map[string]any{"id": k.sku, "inventory": []any{inv}})
			refSKU[k.sku] = k.ref
		}
		data, err := a.panggil(ctx, cred, http.MethodPost, "/product/202309/products/"+pid+"/inventory/update", nil,
			map[string]any{"skus": skus}, true)
		if err != nil {
			for _, k := range perProduk[pid] {
				galat[k.ref] = err
			}
			continue
		}
		var r struct {
			Errors []struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
				Detail  struct {
					SkuID       string `json:"sku_id"`
					ExtraErrors []struct {
						Message string `json:"message"`
					} `json:"extra_errors"`
				} `json:"detail"`
			} `json:"errors"`
		}
		_ = json.Unmarshal(data, &r)
		for _, e := range r.Errors {
			pesan := e.Message
			if len(e.Detail.ExtraErrors) > 0 && e.Detail.ExtraErrors[0].Message != "" {
				pesan = e.Detail.ExtraErrors[0].Message
			}
			err := fmt.Errorf("Tokopedia & Shop menolak stok: %s (kode %d)", pesan, e.Code)
			if ref, ada := refSKU[e.Detail.SkuID]; ada {
				galat[ref] = err
				continue
			}
			for _, k := range perProduk[pid] { // galat tanpa SKU → seluruh produk
				galat[k.ref] = err
			}
		}
	}
	return galat
}
