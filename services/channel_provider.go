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
	// WebhookValues: nilai selain URL utama yang perlu disalin ke konsol
	// penyedia; alamat = URL webhook kanal (untuk alamat turunan).
	WebhookValues(cred ChannelCredentials, alamat string) []structs.LabelValue
	// VerifyChallenge menjawab GET verifikasi alamat webhook. ok=false → 403.
	VerifyChallenge(q url.Values, cred ChannelCredentials) (body string, ok bool)
	// VerifySignature memeriksa tanda tangan webhook (bukan dari penyedia → galat).
	// alamat = URL publik webhook kanal (Shopee menandatangani URL|body).
	VerifySignature(h http.Header, body []byte, cred ChannelCredentials, alamat string) error
	// Events mengurai satu webhook jadi nol atau lebih peristiwa seragam.
	Events(body []byte, cred ChannelCredentials) ([]NormalizedEvent, error)
	// Test memanggil API penyedia dengan kredensial ini; info singkat bila berhasil.
	Test(ctx context.Context, cred ChannelCredentials) (string, error)
}

// providerAdapters: penyedia yang SUDAH bisa disambungkan.
var providerAdapters = map[string]ProviderAdapter{
	"whatsapp":  whatsappAdapter{},
	"gofood":    gofoodAdapter{},
	"grabfood":  grabfoodAdapter{},
	"shopee":    shopeeAdapter{},
	"tokopedia": tokopediaAdapter{},
}

// WebhookBalasan: balasan langsung untuk sub-jalur webhook yang ditangani
// adaptor sendiri (bukan peristiwa pesanan). HTML diisi untuk halaman yang
// dibuka peramban (mis. kembali dari otorisasi toko); Kosong = 200 tanpa isi.
type WebhookBalasan struct {
	Status int
	Body   any
	HTML   string
	Kosong bool
}

// WebhookPermintaan: satu panggilan penyedia ke alamat webhook kanal.
type WebhookPermintaan struct {
	Metode string
	Aksi   string // sub-jalur tanpa garis miring tepi, mis. "oauth/token"
	Query  url.Values
	Header http.Header
	Body   []byte
	Alamat string // URL publik webhook kanal (tanpa sub-jalur)
}

// webhookActor: penyedia yang punya sub-jalur di bawah alamat webhook — mis.
// GrabFood meminta token OAuth dari "server partner"; Shopee mengembalikan
// penjual ke alamat callback setelah otorisasi. handled=false → diperlakukan
// sebagai webhook pesanan biasa. Perubahan pada cred (token baru) disimpan.
type webhookActor interface {
	WebhookAction(ctx context.Context, p WebhookPermintaan, cred ChannelCredentials) (WebhookBalasan, bool)
}

// ackKosong: penyedia yang menganggap balasan ber-isi sebagai kegagalan
// (Shopee: "2xx dan body kosong").
type ackKosong interface{ AckKosong() bool }

// orderFetcher: penyedia yang webhook-nya hanya membawa nomor pesanan (Shopee)
// — rinciannya diambil PEKERJA, bukan di jalur webhook. nil = tidak dicatat
// (mis. belum dibayar / sudah batal).
type orderFetcher interface {
	FetchOrder(ctx context.Context, cred ChannelCredentials, orderID string) (*NormalizedEvent, error)
}

// authorizer: penyedia yang tokonya harus memberi izin lewat peramban (OAuth).
type authorizer interface {
	AuthorizeURL(cred ChannelCredentials, redirect string) string
	// Diotorisasi: nama/ID toko yang sudah memberi izin ("" = belum).
	Diotorisasi(cred ChannelCredentials) string
}

// statefulAuth: kunci kredensial yang merupakan hasil otorisasi (token) —
// dihapus bila identitas aplikasinya berganti.
type statefulAuth interface{ StateKeys() []string }

// izinDicabut: penyedia yang memberi tahu lewat webhook bahwa toko mencabut
// izin aplikasi (Tokopedia & Shop: SELLER_DEAUTHORIZATION) — token dihapus dan
// sambungan ditandai galat supaya tenant tahu harus mengotorisasi ulang.
type izinDicabut interface {
	IzinDicabut(body []byte, cred ChannelCredentials) (pesan string, dicabut bool)
}

// webhookSubscriber: penyedia yang alamat webhook-nya bisa didaftarkan lewat
// API (GoBiz, Tokopedia & Shop) — tenant tidak perlu menempelkannya sendiri.
type webhookSubscriber interface {
	SubscribeWebhook(ctx context.Context, cred ChannelCredentials, alamat string) error
}

// providerAliases: nama bebas yang diketik saat membuat kanal → kode penyedia.
var providerAliases = map[string]string{
	"whatsapp": "whatsapp", "wa": "whatsapp", "whatsapp business": "whatsapp",
	"gofood": "gofood", "gobiz": "gofood",
	"grabfood": "grabfood", "grab": "grabfood",
	"shopee": "shopee",
	// Tokopedia & TikTok Shop di Indonesia = satu toko, satu API.
	"tokopedia": "tokopedia", "tokopedia & shop": "tokopedia",
	"tiktok shop": "tokopedia", "tiktokshop": "tokopedia", "tiktok": "tokopedia",
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
	out := []structs.ChannelProviderInfo{
		whatsappAdapter{}.Info(), gofoodAdapter{}.Info(), grabfoodAdapter{}.Info(), shopeeAdapter{}.Info(),
		tokopediaAdapter{}.Info(),
	}
	out = append(out,
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
		IgnoreIfMissing bool            `json:"ignore_if_missing,omitempty"`
		NeedsFetch      bool            `json:"_fetch,omitempty"`
		Items           []item          `json:"items,omitempty"`
		Fees            []fee           `json:"fees,omitempty"`
		Raw             json.RawMessage `json:"_raw,omitempty"`
	}{
		EventType: ev.EventType, ExternalOrderID: ev.ExternalOrderID, ExternalStatus: ev.ExternalStatus,
		BuyerName: ev.BuyerName, BuyerPhone: ev.BuyerPhone, ShippingAddress: ev.ShippingAddress,
		Courier: ev.Courier, Reason: ev.Reason, IgnoreIfMissing: ev.IgnoreIfMissing, NeedsFetch: ev.NeedsFetch,
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
