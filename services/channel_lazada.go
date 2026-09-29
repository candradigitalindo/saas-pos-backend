package services

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
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

// Adaptor Lazada — Lazada Open Platform (open.lazada.com, dokumen resmi),
// toko Lazada Indonesia.
//
// Toko memakai aplikasi MILIKNYA sendiri (kategori Seller In-house APP: App
// Key & App Secret); hanya akun penjual di "Authorized Seller Whitelist"
// aplikasi itu yang bisa memberi izin:
//   - Tautan izin: auth.lazada.com/oauth/authorize?response_type=code&force_auth
//     &redirect_uri&client_id&state&country=id; redirect_uri WAJIB sama dengan
//     App Callback URL aplikasi = {webhook}/oauth/callback.
//   - code (sekali pakai, 30 menit) → /auth/token/create → access token
//     (±30 hari) + refresh token (±180 hari), diperbarui lewat
//     /auth/token/refresh dengan baris kanal terkunci.
//   - sign = HEX_BESAR(HMAC-SHA256(secret, api + {k}{v} terurut tanpa sign &
//     nilai kosong)); access_token IKUT ditandatangani; timestamp milidetik.
//   - Push (Lazada Push Mechanism): Authorization = hex(HMAC-SHA256(secret,
//     app_key + body)); balas 200 dalam 500 md. Push dikirim per BARIS pesanan
//     dan hanya ke HTTPS bersertifikat OV/EV — karena itu pesanan juga DITARIK
//     tiap 15 menit (/orders/get) sebagai cadangan (channel_tarik_service.go).
//   - Lazada tidak punya status pesanan, hanya status per barang; setiap barang
//     objek sendiri (satu unit). Rincian (/order/get + /order/items/get)
//     diambil PEKERJA.

type lazadaAdapter struct{}

var lazadaState = []string{
	"seller_id", "short_code", "shop_name", "account",
	"access_token", "refresh_token", "access_expires", "auth_state",
	"authorized_at", "pulled_at", "pulled_until",
}

func (lazadaAdapter) Info() structs.ChannelProviderInfo {
	return structs.ChannelProviderInfo{
		Code: "lazada", Name: "Lazada", Kind: "marketplace", Available: true, RequiresAuthorization: true,
		DocsURL:      "https://open.lazada.com/apps/doc/doc?nodeId=10777&docId=108260",
		WebhookLabel: "Push URL (Message Service)",
		Steps: []string{
			"Daftar di Lazada Open Platform (open.lazada.com) dan buat aplikasi kategori Seller In-house APP.",
			"Isi App Callback URL aplikasi dengan nilai dari layar ini, lalu salin App Key & App Secret ke sini dan Simpan.",
			"Di App Management → Auth Management, masukkan akun penjual toko ke Authorized Seller Whitelist. Lalu tekan Otorisasi Toko dan masuk dengan akun penjual itu.",
			"Di tab Message Service, isi Push URL dari layar ini, tekan Verify, dan langganan pesan Trade Order. Push Lazada mewajibkan HTTPS bersertifikat OV/EV — tanpa itu pesanan tetap masuk lewat tarikan berkala tiap 15 menit.",
			"Samakan Seller SKU produk dengan SKU barang toko — atau petakan di pemetaan SKU kanal — supaya stok ikut terpotong.",
		},
		Fields: []structs.ChannelProviderField{
			{Key: "app_key", Label: "App Key"},
			{Key: "app_secret", Label: "App Secret", Secret: true},
		},
		Capabilities: []string{
			"Pesanan yang sudah dibayar masuk otomatis — seketika lewat push, dengan tarikan tiap 15 menit sebagai cadangan",
			"Pembatalan seluruh pesanan membatalkan penjualan dan mengembalikan stok",
			"Status pengiriman ikut tercatat",
			"Token diperbarui otomatis — toko cukup memberi izin sekali",
			"Stok toko dikirim ke Lazada setiap berubah (barang yang sudah dicocokkan)",
		},
	}
}

func (lazadaAdapter) Prepare(ChannelCredentials) {}

func (lazadaAdapter) StateKeys() []string { return lazadaState }

func (lazadaAdapter) MerchantRef(cred ChannelCredentials) string {
	return strings.TrimSpace(cred["app_key"]) + ":" + strings.TrimSpace(cred["seller_id"])
}

// WebhookValues: App Callback URL aplikasi = alamat balik otorisasi.
func (lazadaAdapter) WebhookValues(_ ChannelCredentials, alamat string) []structs.LabelValue {
	return []structs.LabelValue{{Label: "App Callback URL", Value: alamat + "/oauth/callback"}}
}

func (lazadaAdapter) VerifyChallenge(url.Values, ChannelCredentials) (string, bool) {
	return "", false
}

func (lazadaAdapter) Diotorisasi(cred ChannelCredentials) string {
	if cred["refresh_token"] == "" {
		return ""
	}
	return firstNonEmpty(cred["shop_name"], cred["short_code"], "toko "+cred["seller_id"])
}

// lazadaURL: gerbang API Indonesia, gerbang token, dan halaman izin; bisa
// diarahkan ke tiruan saat uji.
func lazadaURL() (api, token, izin string) {
	return strings.TrimRight(config.GetEnv("CHANNEL_LAZADA_API_URL", "https://api.lazada.co.id/rest"), "/"),
		strings.TrimRight(config.GetEnv("CHANNEL_LAZADA_TOKEN_URL", "https://auth.lazada.com/rest"), "/"),
		config.GetEnv("CHANNEL_LAZADA_AUTH_URL", "https://auth.lazada.com/oauth/authorize")
}

// lazadaSign: dokumen "Signature algorithm" (contoh resmi di uji unit).
func lazadaSign(secret, api string, q url.Values) string {
	kunci := make([]string, 0, len(q))
	for k := range q {
		if k != "sign" && q.Get(k) != "" {
			kunci = append(kunci, k)
		}
	}
	sort.Strings(kunci)
	var b strings.Builder
	b.WriteString(api)
	for _, k := range kunci {
		b.WriteString(k)
		b.WriteString(q.Get(k))
	}
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(b.String()))
	return strings.ToUpper(hex.EncodeToString(m.Sum(nil)))
}

func (lazadaAdapter) AuthorizeURL(cred ChannelCredentials, redirect string) string {
	cred["auth_state"] = acakHex(16)
	_, _, izin := lazadaURL()
	return izin + "?" + url.Values{
		"response_type": {"code"}, "force_auth": {"true"}, "redirect_uri": {redirect},
		"client_id": {cred["app_key"]}, "state": {cred["auth_state"]}, "country": {"id"},
	}.Encode()
}

func (lazadaAdapter) VerifySignature(h http.Header, body []byte, cred ChannelCredentials, _ string) error {
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

type lazadaPush struct {
	SellerID    json.RawMessage `json:"seller_id"`
	MessageType json.Number     `json:"message_type"`
	Data        struct {
		OrderStatus      string          `json:"order_status"`
		TradeOrderID     json.RawMessage `json:"trade_order_id"`
		TradeOrderLineID json.RawMessage `json:"trade_order_line_id"`
		ReverseOrderID   json.RawMessage `json:"reverse_order_id"`
		StatusUpdateTime json.Number     `json:"status_update_time"`
	} `json:"data"`
}

// lazadaAktif: status barang yang berarti sudah dibayar & masih berjalan.
func lazadaAktif(s string) bool {
	switch s {
	case "", "unpaid", "canceled", "cancelled":
		return false
	}
	return true
}

func (lazadaAdapter) Events(body []byte, cred ChannelCredentials) ([]NormalizedEvent, error) {
	var p lazadaPush
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	if err := dec.Decode(&p); err != nil {
		return nil, fmt.Errorf("json tidak valid: %w", err)
	}
	// Hanya pesan pesanan (0); 8 = peringatan izin hampir habis, pesan uji
	// "Verify" konsol, dll. → 200 tanpa peristiwa.
	pesanan := teksJSON(p.Data.TradeOrderID)
	if p.MessageType.String() != "0" || pesanan == "" {
		return nil, nil
	}
	if toko, penjual := cred["seller_id"], teksJSON(p.SellerID); toko != "" && penjual != "" && penjual != toko {
		return nil, nil
	}
	dasar := NormalizedEvent{ExternalOrderID: pesanan}
	if sec, err := p.Data.StatusUpdateTime.Int64(); err == nil && sec > 0 {
		t := time.Unix(sec, 0).UTC()
		dasar.OccurredAt = &t
	}
	status := strings.ToLower(strings.TrimSpace(p.Data.OrderStatus))
	catat := func(s string) NormalizedEvent {
		ev := dasar
		ev.EventType, ev.ExternalStatus, ev.IgnoreIfMissing = "order.status", s, true
		return ev
	}
	// Pengembalian/refund: dicatat sebagai status, penjualan tidak diubah.
	if teksJSON(p.Data.ReverseOrderID) != "" {
		if status == "" {
			return nil, nil
		}
		return []NormalizedEvent{catat("reverse_" + status)}, nil
	}
	switch {
	case status == "canceled" || status == "cancelled":
		// Status per BARANG: satu barang batal belum tentu seluruh pesanan —
		// pekerja memeriksa rinciannya. Dedup per baris supaya pembatalan
		// barang berikutnya tidak terbuang sebagai duplikat.
		ev := dasar
		ev.EventType, ev.NeedsFetch, ev.IgnoreIfMissing = "order.canceled", true, true
		ev.DedupKey = "batal:" + teksJSON(p.Data.TradeOrderLineID)
		return []NormalizedEvent{ev}, nil
	case !lazadaAktif(status):
		return nil, nil // belum dibayar
	default:
		buat := dasar
		buat.EventType, buat.NeedsFetch = "order.created", true
		return []NormalizedEvent{buat, catat(status)}, nil
	}
}

// panggil: API bertanda tangan. Galat memuat pesan asli Lazada.
func (lazadaAdapter) panggil(ctx context.Context, cred ChannelCredentials, base, api string, q url.Values, pakaiToken bool) (map[string]json.RawMessage, error) {
	if q == nil {
		q = url.Values{}
	}
	q.Set("app_key", cred["app_key"])
	q.Set("timestamp", strconv.FormatInt(time.Now().UnixMilli(), 10))
	q.Set("sign_method", "sha256")
	if pakaiToken {
		q.Set("access_token", cred["access_token"])
	}
	q.Set("sign", lazadaSign(cred["app_secret"], api, q))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+api+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	res, err := httpKanal.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tidak bisa menghubungi Lazada: %w", err)
	}
	defer res.Body.Close()
	isi, _ := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	var out map[string]json.RawMessage
	dec := json.NewDecoder(bytes.NewReader(isi))
	dec.UseNumber()
	if err := dec.Decode(&out); err != nil {
		return nil, fmt.Errorf("balasan Lazada tidak dapat dibaca (HTTP %d)", res.StatusCode)
	}
	kode, pesan := teksJSON(out["code"]), teksJSON(out["message"])
	if res.StatusCode != http.StatusOK || kode != "0" {
		if kode != "" && kode != "0" {
			pesan = strings.TrimSpace(pesan + " (" + kode + ")")
		}
		return nil, fmt.Errorf("Lazada menolak: %s", firstNonEmpty(pesan, fmt.Sprintf("HTTP %d", res.StatusCode)))
	}
	return out, nil
}

// token menukar code (token/create) atau refresh token (token/refresh).
func (a lazadaAdapter) token(ctx context.Context, cred ChannelCredentials, api string, q url.Values) error {
	_, base, _ := lazadaURL()
	out, err := a.panggil(ctx, cred, base, api, q, false)
	if err != nil {
		return err
	}
	akses, segar := teksJSON(out["access_token"]), teksJSON(out["refresh_token"])
	if akses == "" {
		return fmt.Errorf("Lazada tidak mengirim token")
	}
	umur, err := strconv.ParseInt(teksJSON(out["expires_in"]), 10, 64)
	if err != nil || umur <= 0 {
		umur = 7 * 24 * 3600
	}
	cred["access_token"], cred["access_expires"] = akses, strconv.FormatInt(time.Now().Unix()+umur, 10)
	if segar != "" {
		cred["refresh_token"] = segar
	}
	if akun := teksJSON(out["account"]); akun != "" {
		cred["account"] = akun
	}
	var info []struct {
		Country   string          `json:"country"`
		SellerID  json.RawMessage `json:"seller_id"`
		ShortCode string          `json:"short_code"`
	}
	_ = json.Unmarshal(out["country_user_info"], &info)
	for _, u := range info {
		if strings.EqualFold(u.Country, "id") {
			cred["seller_id"], cred["short_code"] = teksJSON(u.SellerID), u.ShortCode
			return nil
		}
	}
	if cred["seller_id"] == "" {
		return fmt.Errorf("akun ini bukan toko Lazada Indonesia")
	}
	return nil
}

// pastikanToken memperbarui access token bila tinggal kurang dari sejam.
// Dipanggil dengan baris kanal terkunci.
func (a lazadaAdapter) pastikanToken(ctx context.Context, cred ChannelCredentials) error {
	if cred["refresh_token"] == "" {
		return fmt.Errorf("toko belum diotorisasi — tekan Otorisasi Toko")
	}
	if exp, err := strconv.ParseInt(cred["access_expires"], 10, 64); err == nil && cred["access_token"] != "" &&
		time.Now().Unix() < exp-3600 {
		return nil
	}
	if err := a.token(ctx, cred, "/auth/token/refresh", url.Values{"refresh_token": {cred["refresh_token"]}}); err != nil {
		return fmt.Errorf("izin toko Lazada kedaluwarsa atau dicabut — otorisasi ulang (%v)", err)
	}
	return nil
}

// toko: profil penjual (GetSeller) — nama toko untuk ditampilkan.
func (a lazadaAdapter) toko(ctx context.Context, cred ChannelCredentials) (nama, kode string, err error) {
	api, _, _ := lazadaURL()
	out, err := a.panggil(ctx, cred, api, "/seller/get", nil, true)
	if err != nil {
		return "", "", err
	}
	var d struct {
		Name      string `json:"name"`
		ShortCode string `json:"short_code"`
	}
	_ = json.Unmarshal(out["data"], &d)
	return d.Name, d.ShortCode, nil
}

// Test: token (diperbarui bila perlu) + profil penjual terbaca.
func (a lazadaAdapter) Test(ctx context.Context, cred ChannelCredentials) (string, error) {
	if cred["refresh_token"] == "" {
		return "", fmt.Errorf("kredensial tersimpan — lanjutkan dengan Otorisasi Toko")
	}
	if err := a.pastikanToken(ctx, cred); err != nil {
		return "", err
	}
	nama, kode, err := a.toko(ctx, cred)
	if err != nil {
		return "", err
	}
	if nama != "" {
		cred["shop_name"] = nama
	}
	if kode != "" {
		cred["short_code"] = kode
	}
	info := firstNonEmpty(nama, "Toko") + " · " + firstNonEmpty(cred["short_code"], "seller "+cred["seller_id"])
	if t, err := time.Parse(time.RFC3339, cred["pulled_at"]); err == nil {
		info += " · ditarik " + t.In(time.FixedZone("WIB", 7*3600)).Format("15.04") + " WIB"
	}
	return info, nil
}

// WebhookAction: GET .../oauth/callback — penjual kembali dari halaman izin.
func (a lazadaAdapter) WebhookAction(ctx context.Context, p WebhookPermintaan, cred ChannelCredentials) (WebhookBalasan, bool) {
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
	code := q.Get("code")
	if code == "" {
		return gagal("Otorisasi dibatalkan", "Toko belum memberi izin. Ulangi dari aplikasi bila ingin menyambungkan.")
	}
	lamaPenjual := cred["seller_id"]
	delete(cred, "seller_id")
	if err := a.token(ctx, cred, "/auth/token/create", url.Values{"code": {code}}); err != nil {
		cred["seller_id"] = lamaPenjual
		return gagal("Otorisasi gagal", err.Error())
	}
	delete(cred, "auth_state")
	if lamaPenjual != cred["seller_id"] {
		// Toko lain → mulai tarikan dari awal lagi.
		delete(cred, "pulled_until")
		delete(cred, "shop_name")
	}
	cred["authorized_at"] = time.Now().UTC().Format(time.RFC3339)
	if nama, _, err := a.toko(ctx, cred); err == nil && nama != "" {
		cred["shop_name"] = nama
	}
	return WebhookBalasan{Status: http.StatusOK, HTML: halamanPesan("Toko Lazada tersambung",
		firstNonEmpty(cred["shop_name"], "Toko")+" sudah memberi izin. Pesanan baru akan masuk otomatis. Tutup tab ini dan kembali ke aplikasi.")}, true
}

// lazadaWaktu: "2014-10-15 18:36:05 +0800" (GetOrder) atau RFC3339 (GetOrders).
func lazadaWaktu(s string) (time.Time, bool) {
	for _, f := range []string{"2006-01-02 15:04:05 -0700", time.RFC3339} {
		if t, err := time.Parse(f, strings.TrimSpace(s)); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}

// FetchOrder: rincian pesanan (GetOrder + GetOrderItems) — dipanggil pekerja,
// baik untuk pesanan baru maupun pembatalan (status per barang).
func (a lazadaAdapter) FetchOrder(ctx context.Context, cred ChannelCredentials, orderID string) (*NormalizedEvent, error) {
	if err := a.pastikanToken(ctx, cred); err != nil {
		return nil, err
	}
	api, _, _ := lazadaURL()
	o, err := a.panggil(ctx, cred, api, "/order/get", url.Values{"order_id": {orderID}}, true)
	if err != nil {
		return nil, err
	}
	var ord struct {
		CreatedAt string `json:"created_at"`
		First     string `json:"customer_first_name"`
		Last      string `json:"customer_last_name"`
		AddrShip  struct {
			First    string `json:"first_name"`
			Last     string `json:"last_name"`
			Phone    string `json:"phone"`
			Address1 string `json:"address1"`
			Address3 string `json:"address3"`
			Address4 string `json:"address4"`
			City     string `json:"city"`
			PostCode string `json:"post_code"`
		} `json:"address_shipping"`
	}
	_ = json.Unmarshal(o["data"], &ord)
	it, err := a.panggil(ctx, cred, api, "/order/items/get", url.Values{"order_id": {orderID}}, true)
	if err != nil {
		return nil, err
	}
	var items []struct {
		SKU           string `json:"sku"`
		ShopSKU       string `json:"shop_sku"`
		SkuID         string `json:"sku_id"`
		ItemPrice     string `json:"item_price"`
		PaidPrice     string `json:"paid_price"`
		VoucherSeller string `json:"voucher_seller"`
		Currency      string `json:"currency"`
		Status        string `json:"status"`
		Kurir         string `json:"shipment_provider"`
	}
	if err := json.Unmarshal(it["data"], &items); err != nil || len(items) == 0 {
		return nil, fmt.Errorf("barang pesanan %s tidak ada di balasan Lazada", orderID)
	}
	ev := &NormalizedEvent{ExternalOrderID: orderID}
	if t, ok := lazadaWaktu(ord.CreatedAt); ok {
		ev.OccurredAt = &t
	}
	// Setiap barang objek sendiri (satu unit): kelompokkan per SKU & harga.
	type kunci struct {
		sku   string
		harga int64
	}
	var urutan []kunci
	jumlah := map[kunci]int64{}
	batal := 0
	for _, b := range items {
		s := strings.ToLower(b.Status)
		if s == "canceled" || s == "cancelled" {
			batal++
			continue
		}
		if !lazadaAktif(s) {
			continue
		}
		if cur := strings.ToUpper(b.Currency); cur != "" && cur != "IDR" {
			return nil, fmt.Errorf("mata uang %s belum didukung (hanya IDR)", cur)
		}
		// Harga toko = harga barang dikurangi voucher yang ditanggung penjual
		// (voucher Lazada dibayar Lazada, bukan potongan toko).
		harga, err := decimal.NewFromString(firstNonEmpty(b.ItemPrice, b.PaidPrice))
		if err != nil {
			return nil, fmt.Errorf("harga barang bukan angka")
		}
		if v, err := decimal.NewFromString(b.VoucherSeller); err == nil {
			harga = harga.Sub(v)
		}
		k := kunci{sku: firstNonEmpty(b.SKU, b.ShopSKU, b.SkuID), harga: harga.Round(0).IntPart()}
		if k.sku == "" {
			return nil, fmt.Errorf("barang pesanan tanpa SKU")
		}
		if _, ada := jumlah[k]; !ada {
			urutan = append(urutan, k)
		}
		jumlah[k]++
		if ev.Courier == "" {
			ev.Courier = b.Kurir
		}
	}
	if len(urutan) == 0 {
		if batal > 0 {
			ev.EventType, ev.Reason, ev.IgnoreIfMissing = "order.canceled", "Lazada: seluruh barang dibatalkan", true
			return ev, nil
		}
		return nil, nil // belum dibayar
	}
	ev.EventType = "order.created"
	ev.BuyerName = strings.TrimSpace(firstNonEmpty(ord.AddrShip.First+" "+ord.AddrShip.Last, ord.First+" "+ord.Last))
	ev.BuyerPhone = ord.AddrShip.Phone
	var alamat []string
	for _, s := range []string{ord.AddrShip.Address1, firstNonEmpty(ord.AddrShip.Address4, ord.AddrShip.City), ord.AddrShip.Address3, ord.AddrShip.PostCode} {
		if s = strings.TrimSpace(s); s != "" {
			alamat = append(alamat, s)
		}
	}
	ev.ShippingAddress = strings.Join(alamat, ", ")
	for _, k := range urutan {
		h := k.harga
		ev.Items = append(ev.Items, NormalizedItem{SKU: k.sku, Qty: decimal.NewFromInt(jumlah[k]), UnitPrice: &h})
	}
	return ev, nil
}

// PullOrders: pesanan yang berubah sejak `sejak` (GetOrders, urut
// updated_at) → peristiwa yang sama dengan push. Kursor berikutnya = saat
// tarikan dimulai (jendela berikutnya mundur beberapa menit, dedup menangani
// ulangannya).
func (a lazadaAdapter) PullOrders(ctx context.Context, cred ChannelCredentials, sejak time.Time) ([]NormalizedEvent, time.Time, error) {
	mulai := time.Now().UTC()
	if err := a.pastikanToken(ctx, cred); err != nil {
		return nil, time.Time{}, err
	}
	api, _, _ := lazadaURL()
	var evs []NormalizedEvent
	for offset := 0; offset <= 5000; offset += 100 {
		out, err := a.panggil(ctx, cred, api, "/orders/get", url.Values{
			"update_after": {sejak.UTC().Format(time.RFC3339)}, "sort_by": {"updated_at"}, "sort_direction": {"ASC"},
			"offset": {strconv.Itoa(offset)}, "limit": {"100"},
		}, true)
		if err != nil {
			return nil, time.Time{}, err
		}
		var d struct {
			Orders []struct {
				OrderID   json.RawMessage `json:"order_id"`
				Statuses  []string        `json:"statuses"`
				UpdatedAt string          `json:"updated_at"`
			} `json:"orders"`
		}
		_ = json.Unmarshal(out["data"], &d)
		for _, o := range d.Orders {
			id := teksJSON(o.OrderID)
			if id == "" {
				continue
			}
			dasar := NormalizedEvent{ExternalOrderID: id}
			if t, ok := lazadaWaktu(o.UpdatedAt); ok {
				dasar.OccurredAt = &t
			}
			var aktif []string
			batal := false
			for _, s := range o.Statuses {
				s = strings.ToLower(s)
				switch {
				case s == "canceled" || s == "cancelled":
					batal = true
				case lazadaAktif(s):
					aktif = append(aktif, s)
				}
			}
			if len(aktif) == 0 {
				if batal {
					ev := dasar
					ev.EventType, ev.NeedsFetch, ev.IgnoreIfMissing, ev.DedupKey = "order.canceled", true, true, "batal:tarik"
					evs = append(evs, ev)
				}
				continue
			}
			buat := dasar
			buat.EventType, buat.NeedsFetch = "order.created", true
			evs = append(evs, buat)
			for _, s := range aktif {
				st := dasar
				st.EventType, st.ExternalStatus, st.IgnoreIfMissing = "order.status", s, true
				evs = append(evs, st)
			}
		}
		if len(d.Orders) < 100 {
			break
		}
	}
	return evs, mulai, nil
}

// ── Stok ───────────────────────────────────────────────────────────────────

// Ref stok Lazada: "item_id:sku_id:SellerSku" — SellerSku terakhir karena
// boleh berisi titik dua ("39817:01:01" di contoh resmi).

// ListListings: GetProducts (filter=all), 50 per halaman.
func (a lazadaAdapter) ListListings(ctx context.Context, cred ChannelCredentials) ([]ListingPenyedia, error) {
	if err := a.pastikanToken(ctx, cred); err != nil {
		return nil, err
	}
	api, _, _ := lazadaURL()
	var hasil []ListingPenyedia
	for offset := 0; offset <= 10000; offset += 50 {
		out, err := a.panggil(ctx, cred, api, "/products/get", url.Values{
			"filter": {"all"}, "offset": {strconv.Itoa(offset)}, "limit": {"50"},
		}, true)
		if err != nil {
			return nil, err
		}
		var d struct {
			Products []struct {
				ItemID     json.RawMessage `json:"item_id"`
				Attributes struct {
					Name string `json:"name"`
				} `json:"attributes"`
				Skus []struct {
					SellerSKU string          `json:"SellerSku"`
					SkuID     json.RawMessage `json:"SkuId"`
				} `json:"skus"`
			} `json:"products"`
		}
		_ = json.Unmarshal(out["data"], &d)
		for _, p := range d.Products {
			item := teksJSON(p.ItemID)
			for _, s := range p.Skus {
				if item == "" || teksJSON(s.SkuID) == "" {
					continue
				}
				hasil = append(hasil, ListingPenyedia{SKU: s.SellerSKU, Ref: item + ":" + teksJSON(s.SkuID) + ":" + s.SellerSKU, Nama: p.Attributes.Name})
			}
		}
		if len(d.Products) < 50 {
			break
		}
	}
	return hasil, nil
}

// PushStock: UpdateSellableQuantity (payload XML), 20 SKU per panggilan
// (anjuran dokumen; batas 50).
func (a lazadaAdapter) PushStock(ctx context.Context, cred ChannelCredentials, stok []StokKirim) map[string]error {
	galat := map[string]error{}
	if err := a.pastikanToken(ctx, cred); err != nil {
		for _, s := range stok {
			galat[s.Ref] = err
		}
		return galat
	}
	var sah []StokKirim
	for _, s := range stok {
		if tanpaAngka(s) {
			continue
		}
		if b := strings.SplitN(s.Ref, ":", 3); len(b) != 3 || b[0] == "" || b[1] == "" {
			galat[s.Ref] = fmt.Errorf("pengenal listing tidak dikenal — cocokkan barang lagi")
			continue
		}
		sah = append(sah, s)
	}
	api, _, _ := lazadaURL()
	for awal := 0; awal < len(sah); awal += 20 {
		bagian := sah[awal:min(awal+20, len(sah))]
		var b strings.Builder
		b.WriteString("<Request><Product><Skus>")
		for _, s := range bagian {
			p := strings.SplitN(s.Ref, ":", 3)
			b.WriteString("<Sku><ItemId>" + xmlTeks(p[0]) + "</ItemId><SkuId>" + xmlTeks(p[1]) + "</SkuId><SellerSku>" + xmlTeks(p[2]) +
				"</SellerSku><SellableQuantity>" + strconv.FormatInt(s.Qty, 10) + "</SellableQuantity></Sku>")
		}
		b.WriteString("</Skus></Product></Request>")
		if _, err := a.panggil(ctx, cred, api, "/product/stock/sellable/update", url.Values{"payload": {b.String()}}, true); err != nil {
			for _, s := range bagian {
				galat[s.Ref] = err
			}
		}
	}
	return galat
}

func xmlTeks(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}
