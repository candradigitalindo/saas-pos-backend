package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"candra/backend-api/structs"
)

// Sambungan API kanal MILIK TENANT.
//
// Setiap tenant mendaftar sendiri ke penyedia (Meta untuk WhatsApp, GoBiz untuk
// GoFood, Grab, Shopee, ...) dan memegang kredensialnya sendiri. Platform tidak
// mengurus izin atau kemitraan — tugasnya menyambungkan: menyimpan kredensial
// terenkripsi, memberi alamat webhook per kanal, memeriksa tanda tangannya, dan
// menerjemahkan payload penyedia ke peristiwa seragam pipeline kanal.
//
// Satu penyedia = satu ProviderAdapter. Menambah penyedia tidak mengubah inti:
// daftarkan adaptornya di providerAdapters dan isi katalognya.

// ChannelCredentials: kredensial kanal, kunci sesuai ProviderInfo.Fields.
type ChannelCredentials map[string]string

// ProviderAdapter menerjemahkan satu penyedia nyata.
type ProviderAdapter interface {
	Info() structs.ChannelProviderInfo
	// Prepare melengkapi nilai yang dibuat platform (mis. verify token) saat disimpan.
	Prepare(cred ChannelCredentials)
	// MerchantRef: pengenal toko/nomor di penyedia — unik lintas tenant.
	MerchantRef(cred ChannelCredentials) string
	// WebhookValues: nilai selain URL yang perlu disalin ke konsol penyedia.
	WebhookValues(cred ChannelCredentials) []structs.LabelValue
	// VerifyChallenge menjawab GET verifikasi alamat webhook. ok=false → 403.
	VerifyChallenge(q url.Values, cred ChannelCredentials) (body string, ok bool)
	// VerifySignature memeriksa tanda tangan webhook (bukan dari penyedia → galat).
	VerifySignature(h http.Header, body []byte, cred ChannelCredentials) error
	// Events mengurai satu webhook jadi nol atau lebih peristiwa seragam.
	Events(body []byte, cred ChannelCredentials) ([]NormalizedEvent, error)
	// Test memanggil API penyedia dengan kredensial ini; info singkat bila berhasil.
	Test(ctx context.Context, cred ChannelCredentials) (string, error)
}

// providerAdapters: penyedia yang SUDAH bisa disambungkan.
var providerAdapters = map[string]ProviderAdapter{
	"whatsapp": whatsappAdapter{},
}

// providerAliases: nama bebas yang diketik saat membuat kanal → kode penyedia.
var providerAliases = map[string]string{
	"whatsapp": "whatsapp", "wa": "whatsapp", "whatsapp business": "whatsapp",
	"gofood": "gofood", "gobiz": "gofood",
	"grabfood": "grabfood", "grab": "grabfood",
	"shopee":      "shopee",
	"tiktok shop": "tiktokshop", "tiktokshop": "tiktokshop", "tokopedia": "tiktokshop",
	"lazada": "lazada",
}

// ProviderCode menebak kode penyedia dari nama/provider kanal ("" bila tak dikenal).
func ProviderCode(s string) string {
	return providerAliases[strings.ToLower(strings.TrimSpace(s))]
}

// segera: penyedia yang adaptornya belum dibuat — tetap ditampilkan supaya
// tenant tahu yang disiapkan & cara mendaftarnya, tanpa janji tanggal.
func segera(code, name, kind, docs, catatan string) structs.ChannelProviderInfo {
	return structs.ChannelProviderInfo{
		Code: code, Name: name, Kind: kind, DocsURL: docs, Note: catatan,
		Steps: []string{}, Fields: []structs.ChannelProviderField{}, Capabilities: []string{},
	}
}

// ListChannelProviders: katalog penyedia untuk layar "Hubungkan API".
func ListChannelProviders() []structs.ChannelProviderInfo {
	out := []structs.ChannelProviderInfo{whatsappAdapter{}.Info()}
	out = append(out,
		segera("gofood", "GoFood (GoBiz)", "delivery_app", "https://developer.gobiz.com/",
			"Butuh akun GoBiz Developer (Client ID, Client Secret, Partner ID, Outlet ID) milik toko."),
		segera("grabfood", "GrabFood", "delivery_app", "https://developer.grab.com/",
			"Butuh kredensial GrabFood Partner API (client ID & secret) milik toko."),
		segera("shopee", "Shopee", "marketplace", "https://open.shopee.com/",
			"Butuh aplikasi Shopee Open Platform (Partner ID & Partner Key) milik toko."),
		segera("tiktokshop", "TikTok Shop & Tokopedia", "marketplace", "https://partner.tiktokshop.com/",
			"Tokopedia kini lewat API TikTok Shop — butuh App Key & App Secret milik toko."),
		segera("lazada", "Lazada", "marketplace", "https://open.lazada.com/",
			"Butuh aplikasi Lazada Open Platform (App Key & App Secret) milik toko."),
	)
	return out
}

// genericFromEvent menyimpan peristiwa penyedia dalam bentuk payload adaptor
// generik — pekerja memprosesnya dengan jalur yang sama seperti peristiwa lain.
// Payload asli penyedia ikut di "_raw" untuk penelusuran.
func genericFromEvent(ev NormalizedEvent, raw []byte) ([]byte, error) {
	type item struct {
		SKU       string `json:"sku,omitempty"`
		ProductID string `json:"product_id,omitempty"`
		VariantID string `json:"variant_id,omitempty"`
		Qty       string `json:"qty"`
		UnitPrice *int64 `json:"unit_price,omitempty"`
	}
	type fee struct {
		Kind   string `json:"kind"`
		Amount int64  `json:"amount"`
		Note   string `json:"note,omitempty"`
	}
	p := struct {
		EventType       string          `json:"event_type"`
		ExternalOrderID string          `json:"external_order_id"`
		ExternalStatus  string          `json:"external_status,omitempty"`
		BuyerName       string          `json:"buyer_name,omitempty"`
		BuyerPhone      string          `json:"buyer_phone,omitempty"`
		ShippingAddress string          `json:"shipping_address,omitempty"`
		Courier         string          `json:"courier,omitempty"`
		Reason          string          `json:"reason,omitempty"`
		OccurredAt      string          `json:"occurred_at,omitempty"`
		Items           []item          `json:"items,omitempty"`
		Fees            []fee           `json:"fees,omitempty"`
		Raw             json.RawMessage `json:"_raw,omitempty"`
	}{
		EventType: ev.EventType, ExternalOrderID: ev.ExternalOrderID, ExternalStatus: ev.ExternalStatus,
		BuyerName: ev.BuyerName, BuyerPhone: ev.BuyerPhone, ShippingAddress: ev.ShippingAddress,
		Courier: ev.Courier, Reason: ev.Reason,
	}
	if ev.OccurredAt != nil {
		p.OccurredAt = ev.OccurredAt.UTC().Format(time.RFC3339)
	}
	for _, it := range ev.Items {
		p.Items = append(p.Items, item{
			SKU: it.SKU, ProductID: it.ProductID, VariantID: it.VariantID, Qty: it.Qty.String(), UnitPrice: it.UnitPrice,
		})
	}
	for _, f := range ev.Fees {
		p.Fees = append(p.Fees, fee{Kind: f.Kind, Amount: f.Amount, Note: f.Note})
	}
	if json.Valid(raw) {
		p.Raw = raw
	}
	return json.Marshal(p)
}
