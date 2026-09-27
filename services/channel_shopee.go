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
	"strconv"
	"strings"
	"time"

	"candra/backend-api/config"
	"candra/backend-api/structs"

	"github.com/shopspring/decimal"
)

// Adaptor Shopee — Shopee Open Platform v2 (open.shopee.com, dokumen resmi).
//
// Toko memakai aplikasi Open Platform MILIKNYA sendiri (Partner ID & Partner
// Key, tipe "Seller In House System"), lalu memberi izin tokonya lewat
// peramban (OAuth):
//   - Tautan otorisasi: {auth}?partner_id&auth_type=seller&redirect_uri&response_type=code&state
//     → Shopee mengembalikan penjual ke {webhook}/oauth/callback?code&shop_id&state.
//   - code → POST /api/v2/auth/token/get → access_token (±4 jam) + refresh_token
//     (30 hari, SEKALI PAKAI — pembaruan dilakukan dengan baris kanal terkunci).
//   - Tanda tangan API toko: hex(HMAC-SHA256(partner_key, partner_id+path+timestamp+access_token+shop_id)).
//   - Push (webhook): header Authorization = hex(HMAC-SHA256(partner_key, URL+"|"+body));
//     balasan WAJIB 2xx tanpa isi dalam 3 detik.
//   - Push status pesanan (code 3) hanya membawa ordersn & status — rinciannya
//     (get_order_detail) diambil PEKERJA, bukan di jalur webhook.

type shopeeAdapter struct{}

// Kunci kredensial hasil otorisasi (bukan isian tenant).
var shopeeState = []string{"shop_id", "shop_name", "access_token", "refresh_token", "access_expires", "auth_state"}

func (shopeeAdapter) Info() structs.ChannelProviderInfo {
	return structs.ChannelProviderInfo{
		Code: "shopee", Name: "Shopee", Kind: "marketplace", Available: true, RequiresAuthorization: true,
		DocsURL:      "https://open.shopee.com/developer-guide/20",
		WebhookLabel: "Push callback URL",
		Steps: []string{
			"Di Shopee Open Platform, daftarkan akun developer toko dan buat aplikasi bertipe Seller In House System.",
			"Salin Partner ID dan Partner Key aplikasi itu (Live untuk Produksi, Test untuk Sandbox), isi di sini, lalu Simpan.",
			"Di pengaturan aplikasi Shopee, isi Redirect URL Domain dengan domain server ini, lalu tekan Otorisasi Toko Shopee dan setujui dengan akun penjual.",
			"Di menu Push Mechanism aplikasi, isi Push callback URL dari layar ini, tekan Verify, lalu aktifkan Order Status Push.",
			"Samakan SKU produk/variasi di Shopee dengan SKU barang toko — atau petakan di pemetaan SKU kanal — supaya stok ikut terpotong.",
		},
		Fields: []structs.ChannelProviderField{
			{Key: "partner_id", Label: "Partner ID"},
			{Key: "partner_key", Label: "Partner Key", Secret: true},
			{Key: "environment", Label: "Lingkungan", Optional: true, Options: []structs.LabelValue{
				{Value: "production", Label: "Produksi"}, {Value: "sandbox", Label: "Sandbox (uji)"},
			}},
		},
		Capabilities: []string{
			"Pesanan Shopee yang sudah dibayar masuk otomatis sebagai penjualan kanal",
			"Pembatalan dari Shopee membatalkan penjualan dan mengembalikan stok",
			"Status pengiriman (diproses, dikirim, selesai) ikut tercatat",
			"Token akses diperbarui otomatis — toko cukup memberi izin sekali",
		},
	}
}

func (shopeeAdapter) Prepare(cred ChannelCredentials) {
	if cred["environment"] != "sandbox" {
		cred["environment"] = "production"
	}
}

func (shopeeAdapter) StateKeys() []string { return shopeeState }

func (shopeeAdapter) MerchantRef(cred ChannelCredentials) string {
	// Satu aplikasi bisa dipakai beberapa toko; toko ditautkan setelah otorisasi.
	return strings.TrimSpace(cred["partner_id"]) + ":" + strings.TrimSpace(cred["shop_id"])
}

// WebhookValues: domain yang wajib didaftarkan sebagai Redirect URL Domain —
// Shopee menolak redirect_uri di luar domain itu.
func (shopeeAdapter) WebhookValues(_ ChannelCredentials, alamat string) []structs.LabelValue {
	u, err := url.Parse(alamat)
	if err != nil || u.Host == "" {
		return nil
	}
	return []structs.LabelValue{{Label: "Redirect URL Domain", Value: u.Scheme + "://" + u.Host}}
}

func (shopeeAdapter) VerifyChallenge(url.Values, ChannelCredentials) (string, bool) { return "", false }

func (shopeeAdapter) AckKosong() bool { return true }

func (shopeeAdapter) Diotorisasi(cred ChannelCredentials) string {
	if cred["refresh_token"] == "" {
		return ""
	}
	return firstNonEmpty(cred["shop_name"], "toko "+cred["shop_id"])
}

// shopeeURL: host API & halaman otorisasi; bisa diarahkan ke tiruan saat uji.
func shopeeURL(cred ChannelCredentials) (api, auth string) {
	if cred["environment"] == "sandbox" {
		api, auth = "https://openplatform.sandbox.test-stable.shopee.sg", "https://open.sandbox.test-stable.shopee.com/auth"
	} else {
		api, auth = "https://partner.shopeemobile.com", "https://open.shopee.com/auth"
	}
	return strings.TrimRight(config.GetEnv("CHANNEL_SHOPEE_API_URL", api), "/"),
		config.GetEnv("CHANNEL_SHOPEE_AUTH_URL", auth)
}

func shopeeSign(key string, bagian ...string) string {
	m := hmac.New(sha256.New, []byte(key))
	m.Write([]byte(strings.Join(bagian, "")))
	return hex.EncodeToString(m.Sum(nil))
}

func (shopeeAdapter) AuthorizeURL(cred ChannelCredentials, redirect string) string {
	cred["auth_state"] = acakHex(16)
	_, auth := shopeeURL(cred)
	q := url.Values{
		"partner_id": {cred["partner_id"]}, "auth_type": {"seller"}, "redirect_uri": {redirect},
		"response_type": {"code"}, "state": {cred["auth_state"]},
	}
	return auth + "?" + q.Encode()
}

func (shopeeAdapter) VerifySignature(h http.Header, body []byte, cred ChannelCredentials, alamat string) error {
	got := strings.TrimSpace(h.Get("Authorization"))
	want := shopeeSign(cred["partner_key"], alamat, "|", string(body))
	if got == "" || cred["partner_key"] == "" || subtle.ConstantTimeCompare([]byte(got), []byte(want)) != 1 {
		return errTandaTangan
	}
	return nil
}

type shopeePush struct {
	Code   int         `json:"code"`
	ShopID json.Number `json:"shop_id"`
	Data   struct {
		OrderSN    string      `json:"ordersn"`
		Status     string      `json:"status"`
		UpdateTime json.Number `json:"update_time"`
	} `json:"data"`
}

func (shopeeAdapter) Events(body []byte, cred ChannelCredentials) ([]NormalizedEvent, error) {
	var p shopeePush
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	if err := dec.Decode(&p); err != nil {
		return nil, fmt.Errorf("json tidak valid: %w", err)
	}
	if p.Code != 3 || p.Data.OrderSN == "" {
		return nil, nil // otorisasi, pembaruan Shopee, produk, dll.
	}
	if toko := cred["shop_id"]; toko != "" && p.ShopID.String() != "" && p.ShopID.String() != toko {
		return nil, nil
	}
	dasar := NormalizedEvent{ExternalOrderID: p.Data.OrderSN}
	if sec, err := p.Data.UpdateTime.Int64(); err == nil && sec > 0 {
		t := time.Unix(sec, 0).UTC()
		dasar.OccurredAt = &t
	}
	status := strings.ToUpper(p.Data.Status)
	perubahan := dasar
	perubahan.EventType, perubahan.ExternalStatus, perubahan.IgnoreIfMissing = "order.status", strings.ToLower(status), true
	switch status {
	case "", "UNPAID":
		return nil, nil // belum dibayar — belum menjadi penjualan
	case "CANCELLED":
		ev := dasar
		ev.EventType, ev.Reason, ev.IgnoreIfMissing = "order.canceled", "Shopee: pesanan dibatalkan", true
		return []NormalizedEvent{ev}, nil
	case "IN_CANCEL", "TO_RETURN":
		return []NormalizedEvent{perubahan}, nil
	default:
		// READY_TO_SHIP dan seterusnya: sudah dibayar. Pencatatan idempoten,
		// jadi status berikutnya menjadi cadangan bila push sebelumnya terlewat.
		buat := dasar
		buat.EventType, buat.NeedsFetch = "order.created", true
		return []NormalizedEvent{buat, perubahan}, nil
	}
}

// shopeePanggil memanggil API Shopee. Toko: tanda tangan menyertakan
// access_token & shop_id; publik (token): hanya partner_id+path+timestamp.
func shopeePanggil(ctx context.Context, cred ChannelCredentials, method, path string, query url.Values, badan any, toko bool) (map[string]json.RawMessage, error) {
	api, _ := shopeeURL(cred)
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	if query == nil {
		query = url.Values{}
	}
	query.Set("partner_id", cred["partner_id"])
	query.Set("timestamp", ts)
	if toko {
		query.Set("access_token", cred["access_token"])
		query.Set("shop_id", cred["shop_id"])
		query.Set("sign", shopeeSign(cred["partner_key"], cred["partner_id"], path, ts, cred["access_token"], cred["shop_id"]))
	} else {
		query.Set("sign", shopeeSign(cred["partner_key"], cred["partner_id"], path, ts))
	}
	var rd io.Reader
	if badan != nil {
		b, err := json.Marshal(badan)
		if err != nil {
			return nil, err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, api+path+"?"+query.Encode(), rd)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := httpKanal.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tidak bisa menghubungi Shopee: %w", err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	var out map[string]json.RawMessage
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	_ = dec.Decode(&out)
	var galat, pesan string
	_ = json.Unmarshal(out["error"], &galat)
	_ = json.Unmarshal(out["message"], &pesan)
	if res.StatusCode != http.StatusOK || galat != "" {
		return nil, fmt.Errorf("Shopee menolak: %s", firstNonEmpty(pesan, galat, fmt.Sprintf("HTTP %d", res.StatusCode)))
	}
	return out, nil
}

// simpanToken mencatat hasil token/get atau access_token/get ke kredensial.
func simpanToken(cred ChannelCredentials, out map[string]json.RawMessage) error {
	var t struct {
		AccessToken  string      `json:"access_token"`
		RefreshToken string      `json:"refresh_token"`
		ExpireIn     json.Number `json:"expire_in"`
	}
	b, _ := json.Marshal(out)
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	if err := dec.Decode(&t); err != nil || t.AccessToken == "" {
		return fmt.Errorf("Shopee tidak mengirim token")
	}
	umur, err := t.ExpireIn.Int64()
	if err != nil || umur <= 0 || umur > 7*24*3600 {
		umur = 4 * 3600
	}
	cred["access_token"], cred["refresh_token"] = t.AccessToken, t.RefreshToken
	cred["access_expires"] = strconv.FormatInt(time.Now().Unix()+umur, 10)
	return nil
}

// pastikanToken memperbarui access token bila (hampir) kedaluwarsa. Dipanggil
// dengan baris kanal terkunci — refresh token lama tidak berlaku lagi setelah dipakai.
func pastikanToken(ctx context.Context, cred ChannelCredentials) error {
	if cred["refresh_token"] == "" {
		return fmt.Errorf("toko Shopee belum diotorisasi — tekan Otorisasi Toko Shopee")
	}
	if exp, err := strconv.ParseInt(cred["access_expires"], 10, 64); err == nil && cred["access_token"] != "" &&
		time.Now().Unix() < exp-300 {
		return nil
	}
	pid, _ := strconv.ParseInt(cred["partner_id"], 10, 64)
	sid, _ := strconv.ParseInt(cred["shop_id"], 10, 64)
	out, err := shopeePanggil(ctx, cred, http.MethodPost, "/api/v2/auth/access_token/get", nil,
		map[string]any{"refresh_token": cred["refresh_token"], "partner_id": pid, "shop_id": sid}, false)
	if err != nil {
		return fmt.Errorf("izin toko Shopee kedaluwarsa atau dicabut — otorisasi ulang (%v)", err)
	}
	return simpanToken(cred, out)
}

func shopeeInfoToko(ctx context.Context, cred ChannelCredentials) (string, string, error) {
	out, err := shopeePanggil(ctx, cred, http.MethodGet, "/api/v2/shop/get_shop_info", nil, nil, true)
	if err != nil {
		return "", "", err
	}
	var nama, status string
	_ = json.Unmarshal(out["shop_name"], &nama)
	_ = json.Unmarshal(out["status"], &status)
	return nama, status, nil
}

// Test: token (diperbarui bila perlu) + profil toko terbaca.
func (shopeeAdapter) Test(ctx context.Context, cred ChannelCredentials) (string, error) {
	if cred["refresh_token"] == "" {
		return "", fmt.Errorf("kredensial tersimpan — lanjutkan dengan Otorisasi Toko Shopee")
	}
	if err := pastikanToken(ctx, cred); err != nil {
		return "", err
	}
	nama, status, err := shopeeInfoToko(ctx, cred)
	if err != nil {
		return "", err
	}
	if nama != "" {
		cred["shop_name"] = nama
	}
	info := firstNonEmpty(nama, "Toko") + " · shop " + cred["shop_id"]
	if status != "" && status != "NORMAL" {
		info += " · status " + status
	}
	return info, nil
}

// WebhookAction: GET .../oauth/callback — penjual kembali dari halaman izin
// Shopee. code ditukar token; hasilnya halaman singkat di tab peramban itu.
func (shopeeAdapter) WebhookAction(ctx context.Context, p WebhookPermintaan, cred ChannelCredentials) (WebhookBalasan, bool) {
	if p.Metode != http.MethodGet || p.Aksi != "oauth/callback" {
		return WebhookBalasan{}, false
	}
	gagal := func(judul, isi string) (WebhookBalasan, bool) {
		return WebhookBalasan{Status: http.StatusBadRequest, HTML: halamanPesan(judul, isi)}, true
	}
	q := p.Query
	state := q.Get("state")
	if cred["auth_state"] == "" || subtle.ConstantTimeCompare([]byte(state), []byte(cred["auth_state"])) != 1 {
		return gagal("Tautan otorisasi tidak cocok", "Ulangi dari tombol Otorisasi Toko Shopee di aplikasi.")
	}
	code, shop := q.Get("code"), q.Get("shop_id")
	if code == "" {
		return gagal("Otorisasi dibatalkan", "Toko belum memberi izin. Ulangi dari aplikasi bila ingin menyambungkan.")
	}
	if shop == "" {
		return gagal("Pilih satu toko", "Otorisasi lewat akun utama (main account) belum didukung — pilih tokonya langsung.")
	}
	pid, _ := strconv.ParseInt(cred["partner_id"], 10, 64)
	sid, _ := strconv.ParseInt(shop, 10, 64)
	out, err := shopeePanggil(ctx, cred, http.MethodPost, "/api/v2/auth/token/get", nil,
		map[string]any{"code": code, "partner_id": pid, "shop_id": sid}, false)
	if err != nil {
		return gagal("Otorisasi gagal", err.Error())
	}
	cred["shop_id"] = shop
	if err := simpanToken(cred, out); err != nil {
		return gagal("Otorisasi gagal", err.Error())
	}
	delete(cred, "auth_state")
	if nama, _, err := shopeeInfoToko(ctx, cred); err == nil && nama != "" {
		cred["shop_name"] = nama
	}
	return WebhookBalasan{Status: http.StatusOK, HTML: halamanPesan("Toko Shopee tersambung",
		firstNonEmpty(cred["shop_name"], "Toko")+" sudah memberi izin. Tutup tab ini dan kembali ke aplikasi — pesanan baru akan masuk otomatis.")}, true
}

type shopeeOrder struct {
	OrderSN     string      `json:"order_sn"`
	OrderStatus string      `json:"order_status"`
	Currency    string      `json:"currency"`
	CreateTime  json.Number `json:"create_time"`
	BuyerName   string      `json:"buyer_username"`
	Recipient   struct {
		Name        string `json:"name"`
		Phone       string `json:"phone"`
		FullAddress string `json:"full_address"`
	} `json:"recipient_address"`
	Items []struct {
		ItemID    json.Number `json:"item_id"`
		ModelID   json.Number `json:"model_id"`
		ItemSKU   string      `json:"item_sku"`
		ModelSKU  string      `json:"model_sku"`
		Qty       json.Number `json:"model_quantity_purchased"`
		Harga     json.Number `json:"model_discounted_price"`
		HargaAsli json.Number `json:"model_original_price"`
	} `json:"item_list"`
}

// FetchOrder mengambil rincian pesanan (get_order_detail) — dipanggil pekerja.
func (shopeeAdapter) FetchOrder(ctx context.Context, cred ChannelCredentials, orderSN string) (*NormalizedEvent, error) {
	if err := pastikanToken(ctx, cred); err != nil {
		return nil, err
	}
	out, err := shopeePanggil(ctx, cred, http.MethodGet, "/api/v2/order/get_order_detail", url.Values{
		"order_sn_list":            {orderSN},
		"response_optional_fields": {"buyer_username,item_list,recipient_address,total_amount"},
	}, nil, true)
	if err != nil {
		return nil, err
	}
	var r struct {
		OrderList []shopeeOrder `json:"order_list"`
	}
	dec := json.NewDecoder(bytes.NewReader(out["response"]))
	dec.UseNumber()
	if err := dec.Decode(&r); err != nil || len(r.OrderList) == 0 {
		return nil, fmt.Errorf("rincian pesanan %s tidak ada di balasan Shopee", orderSN)
	}
	o := r.OrderList[0]
	switch strings.ToUpper(o.OrderStatus) {
	case "UNPAID", "CANCELLED":
		return nil, nil
	}
	if cur := strings.ToUpper(o.Currency); cur != "" && cur != "IDR" {
		return nil, fmt.Errorf("mata uang %s belum didukung (hanya IDR)", cur)
	}
	ev := &NormalizedEvent{
		EventType: "order.created", ExternalOrderID: o.OrderSN, Courier: "Shopee",
		BuyerName: firstNonEmpty(o.Recipient.Name, o.BuyerName), BuyerPhone: o.Recipient.Phone,
		ShippingAddress: o.Recipient.FullAddress,
	}
	if sec, err := o.CreateTime.Int64(); err == nil && sec > 0 {
		t := time.Unix(sec, 0).UTC()
		ev.OccurredAt = &t
	}
	for _, it := range o.Items {
		qty, err := decimal.NewFromString(it.Qty.String())
		if err != nil {
			return nil, fmt.Errorf("jumlah %q bukan angka", it.Qty)
		}
		harga, err := decimal.NewFromString(firstNonEmpty(it.Harga.String(), it.HargaAsli.String()))
		if err != nil {
			return nil, fmt.Errorf("harga barang bukan angka")
		}
		h := harga.Round(0).IntPart()
		// SKU variasi/produk yang diisi penjual = SKU barang toko; tanpa itu
		// "item_id[:model_id]" untuk pemetaan SKU kanal.
		sku := firstNonEmpty(it.ModelSKU, it.ItemSKU)
		if sku == "" {
			sku = it.ItemID.String()
			if m := it.ModelID.String(); m != "" && m != "0" {
				sku += ":" + m
			}
		}
		ev.Items = append(ev.Items, NormalizedItem{SKU: sku, Qty: qty, UnitPrice: &h})
	}
	return ev, nil
}
