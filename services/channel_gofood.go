package services

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"candra/backend-api/config"
	"candra/backend-api/models"
	"candra/backend-api/structs"

	"github.com/shopspring/decimal"
)

// Adaptor GoFood — GoBiz Open API, model Direct Integration.
//
// Toko membuat aplikasi sendiri di GoBiz Developer Portal dan menautkan outlet
// GoFood-nya; platform memakai kredensial itu (client credentials) untuk
// mengetes sambungan dan mendaftarkan webhook pesanan ke alamat per kanal.
//
// Kontrak (developer.gobiz.com):
//   - Token: POST {oauth}/oauth2/token, Basic client_id:client_secret,
//     grant_type=client_credentials. Umur ±1 jam.
//   - Webhook: {header:{event_name,...}, body:{order:{order_number, order_items,
//     ...}}}, ditandatangani X-Go-Signature = hex(HMAC-SHA256(notification
//     secret key, body)).
//   - Langganan: POST /integrations/partner/outlets/{outlet_id}/v1/notification-subscriptions
//     {event, url, active}.
//
// Penjualan dicatat saat pesanan DITERIMA toko (merchant_accepted; completed
// sebagai cadangan, idempoten), bukan saat masih menunggu diterima — pesanan
// yang ditolak/kedaluwarsa tidak boleh memotong stok.

type gofoodAdapter struct{}

// Peristiwa yang didaftarkan otomatis.
var gofoodEvents = []string{
	"gofood.order.merchant_accepted",
	"gofood.order.driver_otw_pickup",
	"gofood.order.driver_arrived",
	"gofood.order.completed",
	"gofood.order.cancelled",
}

func (gofoodAdapter) Info() structs.ChannelProviderInfo {
	return structs.ChannelProviderInfo{
		Code: "gofood", Name: "GoFood (GoBiz)", Kind: "delivery_app", Available: true,
		DocsURL: "https://developer.gobiz.com/docs/docs/food-integration/direct-integration/",
		Steps: []string{
			"Masuk ke GoBiz Developer Portal dengan akun GoBiz toko, buat aplikasi Direct Integration, lalu tautkan outlet GoFood Anda.",
			"Salin Client ID, Client Secret, Outlet ID (mis. G123456789), dan Notification Secret Key aplikasi itu.",
			"Isi di sini, pilih Sandbox untuk uji coba atau Produksi, lalu Simpan & Tes. Webhook pesanan didaftarkan otomatis bila alamat publik server sudah diatur; bila belum, daftarkan Callback URL di bawah secara manual.",
			"Paling mudah: kelola menu di POS (Atur Menu → Kirim Menu) — ID menu otomatis sama dengan barang toko, dan habis/tersedia ikut otomatis. Aktifkan cakupan katalog (gofood:catalog) di aplikasi GoBiz untuk itu. Bila menu tetap dikelola di GoBiz, petakan nama menunya di pemetaan SKU kanal.",
		},
		Fields: []structs.ChannelProviderField{
			{Key: "client_id", Label: "Client ID"},
			{Key: "client_secret", Label: "Client Secret", Secret: true},
			{Key: "outlet_id", Label: "Outlet ID", Help: "ID outlet GoBiz, diawali huruf G."},
			{Key: "notification_secret", Label: "Notification Secret Key", Secret: true,
				Help: "Untuk memeriksa tanda tangan X-Go-Signature bahwa webhook benar dari GoBiz."},
			{Key: "environment", Label: "Lingkungan", Optional: true, Options: []structs.LabelValue{
				{Value: "production", Label: "Produksi"}, {Value: "sandbox", Label: "Sandbox (uji)"},
			}},
		},
		Capabilities: []string{
			"Pesanan GoFood yang diterima toko masuk otomatis sebagai penjualan kanal",
			"Pembatalan dari GoFood membatalkan penjualannya dan mengembalikan stok",
			"Status pengemudi (menuju, tiba, selesai) ikut tercatat",
			"Webhook pesanan didaftarkan otomatis ke GoBiz",
			"Menu bisa dikirim dari POS (butuh cakupan katalog di aplikasi GoBiz); item yang stoknya habis otomatis ditutup",
			"Tandai pesanan siap diambil dari rincian pesanan",
		},
	}
}

func (gofoodAdapter) Prepare(cred ChannelCredentials) {
	if cred["environment"] != "sandbox" {
		cred["environment"] = "production"
	}
}

func (gofoodAdapter) MerchantRef(cred ChannelCredentials) string {
	return strings.TrimSpace(cred["outlet_id"])
}

func (gofoodAdapter) WebhookValues(ChannelCredentials, string) []structs.LabelValue { return nil }

// GoBiz tidak memverifikasi alamat lewat GET.
func (gofoodAdapter) VerifyChallenge(url.Values, ChannelCredentials) (string, bool) { return "", false }

func (gofoodAdapter) VerifySignature(h http.Header, body []byte, cred ChannelCredentials, _ string) error {
	want, err := hex.DecodeString(strings.TrimSpace(h.Get("X-Go-Signature")))
	if err != nil || len(want) == 0 || cred["notification_secret"] == "" {
		return errTandaTangan
	}
	m := hmac.New(sha256.New, []byte(cred["notification_secret"]))
	m.Write(body)
	if !hmac.Equal(m.Sum(nil), want) {
		return errTandaTangan
	}
	return nil
}

type gofoodWebhook struct {
	Header struct {
		EventName string `json:"event_name"`
		EventID   string `json:"event_id"`
		Timestamp string `json:"timestamp"`
	} `json:"header"`
	Body struct {
		Customer struct {
			Name string `json:"name"`
		} `json:"customer"`
		Driver struct {
			Name string `json:"name"`
		} `json:"driver"`
		Outlet struct {
			ID string `json:"id"`
		} `json:"outlet"`
		Order struct {
			Status      string `json:"status"`
			OrderNumber string `json:"order_number"`
			Currency    string `json:"currency"`
			CreatedAt   string `json:"created_at"`
			OrderItems  []struct {
				ExternalID string      `json:"external_id"`
				Name       string      `json:"name"`
				Quantity   json.Number `json:"quantity"`
				Price      json.Number `json:"price"`
				Variants   []struct {
					ExternalID string `json:"external_id"`
				} `json:"variants"`
			} `json:"order_items"`
			CancellationDetail struct {
				Reason string `json:"reason"`
			} `json:"cancellation_detail"`
		} `json:"order"`
	} `json:"body"`
}

func (gofoodAdapter) Events(body []byte, cred ChannelCredentials) ([]NormalizedEvent, error) {
	var w gofoodWebhook
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	if err := dec.Decode(&w); err != nil {
		return nil, fmt.Errorf("json tidak valid: %w", err)
	}
	o := w.Body.Order
	if o.OrderNumber == "" {
		return nil, nil // peristiwa katalog/pembayaran — bukan pesanan
	}
	if outlet := strings.TrimSpace(cred["outlet_id"]); outlet != "" && w.Body.Outlet.ID != "" && w.Body.Outlet.ID != outlet {
		return nil, nil // outlet lain di aplikasi yang sama
	}
	kurir := "GoFood"
	if n := strings.TrimSpace(w.Body.Driver.Name); n != "" {
		kurir += " · " + n
	}
	dasar := NormalizedEvent{
		ExternalOrderID: o.OrderNumber, BuyerName: w.Body.Customer.Name, Courier: kurir,
	}
	for _, s := range []string{o.CreatedAt, w.Header.Timestamp} {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			tt := t.UTC()
			dasar.OccurredAt = &tt
			break
		}
	}

	buat := func() (NormalizedEvent, error) {
		ev := dasar
		ev.EventType = "order.created"
		if cur := strings.ToUpper(o.Currency); cur != "" && cur != "IDR" {
			return ev, fmt.Errorf("mata uang %s belum didukung (hanya IDR)", cur)
		}
		for _, it := range o.OrderItems {
			qty, err := decimal.NewFromString(it.Quantity.String())
			if err != nil {
				return ev, fmt.Errorf("jumlah %q bukan angka", it.Quantity)
			}
			harga, err := decimal.NewFromString(it.Price.String())
			if err != nil {
				return ev, fmt.Errorf("harga %q bukan angka", it.Price)
			}
			h := harga.Round(0).IntPart()
			// External ID = ID menu dari sinkron katalog; tanpa itu nama menu,
			// yang bisa dipetakan di pemetaan SKU kanal.
			sku := strings.TrimSpace(it.ExternalID)
			if sku == "" {
				sku = strings.TrimSpace(it.Name)
			}
			ni := NormalizedItem{SKU: sku, Qty: qty, UnitPrice: &h}
			for _, v := range it.Variants {
				if id := strings.TrimSpace(v.ExternalID); id != "" {
					ni.VariantSKU = id // pilihan varian dari menu yang dikirim POS
					break
				}
			}
			ev.Items = append(ev.Items, ni)
		}
		return ev, nil
	}
	status := func(s string) NormalizedEvent {
		ev := dasar
		ev.EventType, ev.ExternalStatus, ev.IgnoreIfMissing = "order.status", s, true
		return ev
	}

	switch w.Header.EventName {
	case "gofood.order.merchant_accepted":
		ev, err := buat()
		if err != nil {
			return nil, err
		}
		return []NormalizedEvent{ev}, nil
	case "gofood.order.completed":
		// Cadangan bila peristiwa "diterima" terlewat — pencatatannya idempoten.
		ev, err := buat()
		if err != nil {
			return nil, err
		}
		return []NormalizedEvent{ev, status("completed")}, nil
	case "gofood.order.driver_otw_pickup":
		return []NormalizedEvent{status("driver_otw_pickup")}, nil
	case "gofood.order.driver_arrived":
		return []NormalizedEvent{status("driver_arrived")}, nil
	case "gofood.order.placed":
		return []NormalizedEvent{status("picked_up")}, nil
	case "gofood.order.cancelled":
		ev := dasar
		ev.EventType, ev.IgnoreIfMissing = "order.canceled", true
		ev.Reason = strings.TrimSpace("GoFood: " + o.CancellationDetail.Reason)
		return []NormalizedEvent{ev}, nil
	default:
		// awaiting_merchant_acceptance, created, katalog, pembayaran — belum
		// menjadi penjualan.
		return nil, nil
	}
}

// Alamat GoBiz; bisa diarahkan ke server tiruan saat pengujian.
func gobizURL(cred ChannelCredentials) (api, oauth string) {
	if cred["environment"] == "sandbox" {
		api, oauth = "https://api.partner-sandbox.gobiz.co.id", "https://integration-goauth.gojekapi.com"
	} else {
		api, oauth = "https://api.gobiz.co.id", "https://accounts.go-jek.com"
	}
	api = strings.TrimRight(config.GetEnv("CHANNEL_GOBIZ_API_URL", api), "/")
	oauth = strings.TrimRight(config.GetEnv("CHANNEL_GOBIZ_OAUTH_URL", oauth), "/")
	return api, oauth
}

func gobizToken(ctx context.Context, cred ChannelCredentials) (string, error) {
	return gobizTokenScope(ctx, cred, "partner:outlet:read gofood:order:read gofood:order:write")
}

// gobizTokenScope: token dengan cakupan tertentu. Katalog memakai token
// tersendiri — aplikasi yang belum diberi cakupan katalog tetap bisa menerima
// pesanan; hanya menu yang gagal, dengan pesan GoBiz apa adanya.
func gobizTokenScope(ctx context.Context, cred ChannelCredentials, scope string) (string, error) {
	_, oauth := gobizURL(cred)
	form := url.Values{
		"grant_type": {"client_credentials"},
		"scope":      {scope},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, oauth+"/oauth2/token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(cred["client_id"], cred["client_secret"])
	res, err := httpKanal.Do(req)
	if err != nil {
		return "", fmt.Errorf("tidak bisa menghubungi GoBiz: %w", err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 64<<10))
	var b struct {
		AccessToken      string `json:"access_token"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	_ = json.Unmarshal(raw, &b)
	if res.StatusCode != http.StatusOK || b.AccessToken == "" {
		pesan := strings.TrimSpace(b.ErrorDescription)
		if pesan == "" {
			pesan = strings.TrimSpace(b.Error)
		}
		if pesan == "" {
			pesan = fmt.Sprintf("HTTP %d", res.StatusCode)
		}
		return "", fmt.Errorf("GoBiz menolak Client ID/Secret: %s", pesan)
	}
	return b.AccessToken, nil
}

// gobizPanggil mengirim permintaan ber-token ke API GoBiz.
func gobizPanggil(ctx context.Context, cred ChannelCredentials, token, method, jalur string, badan any) (int, []byte, error) {
	api, _ := gobizURL(cred)
	var rd io.Reader
	if badan != nil {
		b, err := json.Marshal(badan)
		if err != nil {
			return 0, nil, err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, api+jalur, rd)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	res, err := httpKanal.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("tidak bisa menghubungi GoBiz: %w", err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 256<<10))
	return res.StatusCode, raw, nil
}

// pesanGoBiz mengambil pesan galat dari balasan GoBiz bila ada.
func pesanGoBiz(raw []byte, kode int) string {
	var b struct {
		Message string `json:"message"`
		Errors  []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	_ = json.Unmarshal(raw, &b)
	if len(b.Errors) > 0 && b.Errors[0].Message != "" {
		return b.Errors[0].Message
	}
	if b.Message != "" {
		return b.Message
	}
	return fmt.Sprintf("HTTP %d", kode)
}

// Test: token berhasil + outlet terbaca = kredensial & Outlet ID cocok.
func (gofoodAdapter) Test(ctx context.Context, cred ChannelCredentials) (string, error) {
	token, err := gobizToken(ctx, cred)
	if err != nil {
		return "", err
	}
	outlet := strings.TrimSpace(cred["outlet_id"])
	kode, raw, err := gobizPanggil(ctx, cred, token, http.MethodGet,
		"/integrations/partner/outlets/"+url.PathEscape(outlet)+"/v1", nil)
	if err != nil {
		return "", err
	}
	if kode != http.StatusOK {
		return "", fmt.Errorf("outlet %s tidak bisa dibaca: %s", outlet, pesanGoBiz(raw, kode))
	}
	// Nama outlet dicari di beberapa bentuk balasan yang lazim.
	var b struct {
		Data struct {
			Name   string `json:"name"`
			Outlet struct {
				Name string `json:"name"`
			} `json:"outlet"`
		} `json:"data"`
		Name string `json:"name"`
	}
	_ = json.Unmarshal(raw, &b)
	nama := firstNonEmpty(b.Data.Outlet.Name, b.Data.Name, b.Name)
	return strings.Trim(nama+" · "+outlet, " ·"), nil
}

// SubscribeWebhook mendaftarkan alamat webhook kanal untuk tiap peristiwa
// pesanan (langganan khusus outlet).
func (gofoodAdapter) SubscribeWebhook(ctx context.Context, cred ChannelCredentials, alamat string) error {
	token, err := gobizToken(ctx, cred)
	if err != nil {
		return err
	}
	jalur := "/integrations/partner/outlets/" + url.PathEscape(strings.TrimSpace(cred["outlet_id"])) + "/v1/notification-subscriptions"
	for _, ev := range gofoodEvents {
		kode, raw, err := gobizPanggil(ctx, cred, token, http.MethodPost, jalur,
			map[string]any{"event": ev, "url": alamat, "active": true})
		if err != nil {
			return err
		}
		if kode != http.StatusOK && kode != http.StatusCreated {
			return fmt.Errorf("langganan %s ditolak: %s", ev, pesanGoBiz(raw, kode))
		}
	}
	return nil
}

func firstNonEmpty(xs ...string) string {
	for _, x := range xs {
		if strings.TrimSpace(x) != "" {
			return strings.TrimSpace(x)
		}
	}
	return ""
}

// MarkReady: Mark Food Ready — pengemudi (diantar) atau pembeli (ambil sendiri)
// diberi tahu makanan siap. Jenis pesanan dari service_type webhook yang
// tersimpan di raw_payload pesanan: gofood → delivery, gofood_pickup → pickup.
func (gofoodAdapter) MarkReady(ctx context.Context, cred ChannelCredentials, co models.ChannelOrder) error {
	var p struct {
		Raw struct {
			Body struct {
				ServiceType string `json:"service_type"`
			} `json:"body"`
		} `json:"_raw"`
	}
	_ = json.Unmarshal(co.RawPayload, &p)
	jenis := "delivery"
	if strings.EqualFold(p.Raw.Body.ServiceType, "gofood_pickup") {
		jenis = "pickup"
	}
	token, err := gobizToken(ctx, cred)
	if err != nil {
		return err
	}
	jalur := "/integrations/gofood/outlets/" + url.PathEscape(strings.TrimSpace(cred["outlet_id"])) +
		"/v1/orders/" + jenis + "/" + url.PathEscape(co.ExternalOrderID) + "/food-prepared"
	kode, raw, err := gobizPanggil(ctx, cred, token, http.MethodPut, jalur, map[string]string{"country_code": "ID"})
	if err != nil {
		return err
	}
	if kode != http.StatusOK {
		return fmt.Errorf("GoFood menolak: %s", pesanGoBiz(raw, kode))
	}
	return nil
}

// ── Menu & ketersediaan ────────────────────────────────────────────────────

const gobizScopeKatalog = "gofood:catalog:read gofood:catalog:write"

func potong(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) > n {
		return string(r[:n])
	}
	return string(r)
}

// PublishMenu: Update GoFood Outlet Catalog — seluruh menu dikirim sekaligus
// (GoBiz tidak menerima pembaruan sebagian) dan MENGGANTI menu outlet.
func (gofoodAdapter) PublishMenu(ctx context.Context, cred ChannelCredentials, menu MenuKanal) error {
	token, err := gobizTokenScope(ctx, cred, gobizScopeKatalog)
	if err != nil {
		return err
	}
	menus := make([]map[string]any, 0, len(menu.Kategori))
	pilihan := []any{}
	for _, k := range menu.Kategori {
		items := make([]map[string]any, 0, len(k.Item))
		for _, it := range k.Item {
			if it.Harga > 2_000_000 {
				return fmt.Errorf("harga %s melebihi batas GoFood Rp 2.000.000", it.Nama)
			}
			m := map[string]any{"external_id": potong(it.ID, 200), "name": potong(it.Nama, 150), "price": it.Harga, "in_stock": it.Tersedia}
			if it.Foto != "" {
				m["image"] = it.Foto
			}
			if it.Deskripsi != "" {
				m["description"] = potong(it.Deskripsi, 250)
			}
			if len(it.Varian) > 0 {
				// Satu kategori varian per item: wajib pilih satu.
				idKat := potong("VC-"+it.ID, 200)
				varian := make([]map[string]any, 0, len(it.Varian))
				for _, v := range it.Varian {
					varian = append(varian, map[string]any{
						"external_id": potong(v.ID, 200), "name": potong(v.Nama, 150), "price": v.Tambahan, "in_stock": v.Tersedia,
					})
				}
				pilihan = append(pilihan, map[string]any{
					"external_id": idKat, "internal_name": potong(it.Nama+" · pilihan", 150), "name": "Pilihan",
					"rules": map[string]any{"selection": map[string]int{"min_quantity": 1, "max_quantity": 1}}, "variants": varian,
				})
				m["variant_category_external_ids"] = []string{idKat}
			}
			items = append(items, m)
		}
		menus = append(menus, map[string]any{"name": potong(k.Nama, 150), "menu_items": items})
	}
	jalur := "/integrations/gofood/outlets/" + url.PathEscape(strings.TrimSpace(cred["outlet_id"])) + "/v1/catalog"
	kode, raw, err := gobizPanggil(ctx, cred, token, http.MethodPut, jalur,
		map[string]any{"request_id": acakHex(16), "menus": menus, "variant_categories": pilihan})
	if err != nil {
		return err
	}
	if kode != http.StatusOK {
		return fmt.Errorf("GoFood menolak menu: %s", pesanGoBiz(raw, kode))
	}
	return nil
}

// PushStock: Update Menu Items OOS — GoFood hanya mengenal tersedia/habis.
func (gofoodAdapter) PushStock(ctx context.Context, cred ChannelCredentials, stok []StokKirim) map[string]error {
	galat := map[string]error{}
	semua := func(err error) map[string]error {
		for _, s := range stok {
			galat[s.Ref] = err
		}
		return galat
	}
	token, err := gobizTokenScope(ctx, cred, gobizScopeKatalog)
	if err != nil {
		return semua(err)
	}
	daftar := make([]map[string]any, 0, len(stok))
	for _, s := range stok {
		daftar = append(daftar, map[string]any{"external_id": s.Ref, "in_stock": s.Tersedia})
	}
	jalur := "/integrations/gofood/outlets/" + url.PathEscape(strings.TrimSpace(cred["outlet_id"])) + "/v2/menu_item_stocks"
	kode, raw, err := gobizPanggil(ctx, cred, token, http.MethodPatch, jalur, daftar)
	if err != nil {
		return semua(err)
	}
	if kode != http.StatusOK {
		return semua(fmt.Errorf("GoFood menolak ketersediaan: %s", pesanGoBiz(raw, kode)))
	}
	return galat
}
