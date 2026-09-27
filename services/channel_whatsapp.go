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
	"errors"
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

// Adaptor WhatsApp Cloud API (Meta) — pesanan dari KERANJANG KATALOG WhatsApp.
//
// Tenant memakai aplikasi Meta & nomor WhatsApp Business miliknya sendiri.
// Pembeli memilih barang di katalog WA lalu mengirim keranjang; Meta mengirim
// webhook `messages` bertipe "order" berisi SKU (product_retailer_id), jumlah,
// dan harga — itu yang dicatat sebagai pesanan kanal.
//
// Keamanan: setiap POST webhook ditandatangani Meta dengan App Secret tenant
// (header X-Hub-Signature-256 = "sha256=" + HMAC-SHA256(app_secret, body)).
// Verifikasi alamat (GET) memakai verify token yang dibuat platform.
//
// Ini TIDAK berkaitan dengan notifikasi WA platform (sidecar Baileys di
// wa-gateway/) — itu nomor platform, ini nomor milik toko.

type whatsappAdapter struct{}

func (whatsappAdapter) Info() structs.ChannelProviderInfo {
	return structs.ChannelProviderInfo{
		Code: "whatsapp", Name: "WhatsApp Business (katalog)", Kind: "conversation", Available: true,
		DocsURL: "https://developers.facebook.com/docs/whatsapp/cloud-api",
		Steps: []string{
			"Di Meta for Developers, buat aplikasi tipe Business dan tambahkan produk WhatsApp. Hubungkan nomor WhatsApp Business toko dan katalognya (Commerce Manager).",
			"Salin Phone Number ID (WhatsApp › API Setup) dan App Secret (App settings › Basic). Buat token akses permanen lewat System User di Business Settings dengan izin whatsapp_business_messaging dan whatsapp_business_management.",
			"Isi ketiganya di sini, simpan, lalu tekan Tes koneksi.",
			"Di WhatsApp › Configuration, isi Callback URL dan Verify token dari layar ini, lalu langganan (Subscribe) kolom \"messages\".",
			"Samakan Content ID produk di katalog dengan SKU barang di toko — atau petakan SKU kanal — supaya stok ikut terpotong.",
		},
		Fields: []structs.ChannelProviderField{
			{Key: "phone_number_id", Label: "Phone Number ID", Help: "Angka di WhatsApp › API Setup, bukan nomor teleponnya."},
			{Key: "access_token", Label: "Token akses permanen", Secret: true, Help: "Dari System User di Business Settings, bukan token sementara 24 jam."},
			{Key: "app_secret", Label: "App Secret", Secret: true, Help: "Untuk memeriksa bahwa webhook benar dari Meta."},
		},
		Capabilities: []string{
			"Pesanan dari keranjang katalog WhatsApp masuk otomatis sebagai penjualan kanal",
			"Stok barang terpotong; SKU katalog dicocokkan ke barang toko",
			"Nama & nomor pembeli ikut tercatat",
		},
	}
}

func (whatsappAdapter) Prepare(cred ChannelCredentials) {
	if cred["verify_token"] == "" {
		b := make([]byte, 18)
		_, _ = rand.Read(b)
		cred["verify_token"] = hex.EncodeToString(b)
	}
}

func (whatsappAdapter) MerchantRef(cred ChannelCredentials) string {
	return strings.TrimSpace(cred["phone_number_id"])
}

func (whatsappAdapter) WebhookValues(cred ChannelCredentials, _ string) []structs.LabelValue {
	return []structs.LabelValue{{Label: "Verify token", Value: cred["verify_token"]}}
}

func (whatsappAdapter) VerifyChallenge(q url.Values, cred ChannelCredentials) (string, bool) {
	want := cred["verify_token"]
	got := q.Get("hub.verify_token")
	if q.Get("hub.mode") != "subscribe" || want == "" ||
		subtle.ConstantTimeCompare([]byte(got), []byte(want)) != 1 {
		return "", false
	}
	return q.Get("hub.challenge"), true
}

var errTandaTangan = errors.New("tanda tangan webhook tidak cocok")

func (whatsappAdapter) VerifySignature(h http.Header, body []byte, cred ChannelCredentials, _ string) error {
	sig := strings.TrimPrefix(h.Get("X-Hub-Signature-256"), "sha256=")
	want, err := hex.DecodeString(sig)
	if err != nil || len(want) == 0 || cred["app_secret"] == "" {
		return errTandaTangan
	}
	m := hmac.New(sha256.New, []byte(cred["app_secret"]))
	m.Write(body)
	if !hmac.Equal(m.Sum(nil), want) {
		return errTandaTangan
	}
	return nil
}

// Bentuk webhook `messages` WhatsApp Cloud API — hanya bagian yang dipakai.
type waWebhook struct {
	Object string `json:"object"`
	Entry  []struct {
		Changes []struct {
			Field string `json:"field"`
			Value struct {
				Metadata struct {
					PhoneNumberID string `json:"phone_number_id"`
				} `json:"metadata"`
				Contacts []struct {
					WaID    string `json:"wa_id"`
					Profile struct {
						Name string `json:"name"`
					} `json:"profile"`
				} `json:"contacts"`
				Messages []struct {
					From      string `json:"from"`
					ID        string `json:"id"`
					Timestamp string `json:"timestamp"`
					Type      string `json:"type"`
					Order     *struct {
						CatalogID    string `json:"catalog_id"`
						Text         string `json:"text"`
						ProductItems []struct {
							ProductRetailerID string      `json:"product_retailer_id"`
							Quantity          json.Number `json:"quantity"`
							ItemPrice         json.Number `json:"item_price"`
							Currency          string      `json:"currency"`
						} `json:"product_items"`
					} `json:"order"`
				} `json:"messages"`
			} `json:"value"`
		} `json:"changes"`
	} `json:"entry"`
}

// Events mengambil pesan bertipe "order" saja; status kiriman, chat biasa, dan
// pesan untuk nomor lain di aplikasi yang sama diabaikan.
func (whatsappAdapter) Events(body []byte, cred ChannelCredentials) ([]NormalizedEvent, error) {
	var w waWebhook
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	if err := dec.Decode(&w); err != nil {
		return nil, fmt.Errorf("json tidak valid: %w", err)
	}
	nomor := strings.TrimSpace(cred["phone_number_id"])
	var out []NormalizedEvent
	for _, e := range w.Entry {
		for _, c := range e.Changes {
			if c.Field != "messages" || (nomor != "" && c.Value.Metadata.PhoneNumberID != nomor) {
				continue
			}
			nama := map[string]string{}
			for _, k := range c.Value.Contacts {
				nama[k.WaID] = k.Profile.Name
			}
			for _, m := range c.Value.Messages {
				if m.Type != "order" || m.Order == nil {
					continue
				}
				ev := NormalizedEvent{
					EventType:       "order.created",
					ExternalOrderID: m.ID,
					BuyerName:       nama[m.From],
					BuyerPhone:      "+" + strings.TrimPrefix(m.From, "+"),
				}
				if sec, err := strconv.ParseInt(m.Timestamp, 10, 64); err == nil {
					t := time.Unix(sec, 0).UTC()
					ev.OccurredAt = &t
				}
				for _, it := range m.Order.ProductItems {
					if cur := strings.ToUpper(it.Currency); cur != "" && cur != "IDR" {
						return nil, fmt.Errorf("mata uang %s belum didukung (hanya IDR)", cur)
					}
					qty, err := decimal.NewFromString(it.Quantity.String())
					if err != nil {
						return nil, fmt.Errorf("jumlah %q bukan angka", it.Quantity)
					}
					harga, err := decimal.NewFromString(it.ItemPrice.String())
					if err != nil {
						return nil, fmt.Errorf("harga %q bukan angka", it.ItemPrice)
					}
					h := harga.Round(0).IntPart()
					ev.Items = append(ev.Items, NormalizedItem{
						SKU: strings.TrimSpace(it.ProductRetailerID), Qty: qty, UnitPrice: &h,
					})
				}
				out = append(out, ev)
			}
		}
	}
	return out, nil
}

// graphURL: alamat Graph API; bisa diarahkan ke server tiruan saat pengujian.
func graphURL() string {
	base := strings.TrimRight(config.GetEnv("CHANNEL_WA_GRAPH_URL", "https://graph.facebook.com"), "/")
	return base + "/" + config.GetEnv("CHANNEL_WA_GRAPH_VERSION", "v23.0")
}

var httpKanal = &http.Client{Timeout: 10 * time.Second}

// Test membaca profil nomor lewat Graph API — sekaligus membuktikan token &
// Phone Number ID cocok.
func (whatsappAdapter) Test(ctx context.Context, cred ChannelCredentials) (string, error) {
	id := strings.TrimSpace(cred["phone_number_id"])
	u := graphURL() + "/" + url.PathEscape(id) + "?fields=display_phone_number,verified_name"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+cred["access_token"])
	res, err := httpKanal.Do(req)
	if err != nil {
		return "", fmt.Errorf("tidak bisa menghubungi Meta: %w", err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 64<<10))
	var b struct {
		DisplayPhoneNumber string `json:"display_phone_number"`
		VerifiedName       string `json:"verified_name"`
		Error              *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.Unmarshal(raw, &b)
	if res.StatusCode != http.StatusOK {
		if b.Error != nil && b.Error.Message != "" {
			return "", fmt.Errorf("Meta menolak: %s", b.Error.Message)
		}
		return "", fmt.Errorf("Meta membalas HTTP %d", res.StatusCode)
	}
	info := strings.TrimSpace(b.VerifiedName)
	if b.DisplayPhoneNumber != "" {
		info = strings.TrimSpace(info + " · " + b.DisplayPhoneNumber)
	}
	return strings.Trim(info, " ·"), nil
}
