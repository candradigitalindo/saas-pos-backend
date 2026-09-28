package services

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"strings"
	"time"

	"candra/backend-api/config"
	"candra/backend-api/helpers"
	"candra/backend-api/internal/rahasia"
	"candra/backend-api/internal/reqctx"
	"candra/backend-api/models"
	"candra/backend-api/repositories"
	"candra/backend-api/structs"

	"gorm.io/gorm"
)

// Layanan sambungan API kanal — lihat channel_provider.go.

// ErrWebhookUnauthorized: tanda tangan webhook tidak cocok (bukan dari penyedia).
var ErrWebhookUnauthorized = errors.New("webhook tidak sah")

func bukaKredensial(ch models.Channel) (ChannelCredentials, error) {
	cred := ChannelCredentials{}
	if len(ch.CredentialsEncrypted) == 0 {
		return cred, nil
	}
	polos, err := rahasia.Buka(ch.CredentialsEncrypted)
	if err != nil {
		return nil, fmt.Errorf("kredensial kanal tidak terbaca (kunci enkripsi berubah?): %w", err)
	}
	if err := json.Unmarshal(polos, &cred); err != nil {
		return nil, err
	}
	return cred, nil
}

func webhookURL(provider, token string) string {
	if token == "" || provider == "" {
		return ""
	}
	base := strings.TrimRight(config.GetEnv("APP_URL", ""), "/")
	return base + "/webhooks/channels/" + provider + "/" + token
}

func pratinjau(s string) string {
	r := []rune(s)
	if len(r) <= 4 {
		return strings.Repeat("•", len(r))
	}
	return "••••" + string(r[len(r)-4:])
}

func connectionResponse(ctx context.Context, ch models.Channel, cred ChannelCredentials) structs.ChannelConnectionResponse {
	out := structs.ChannelConnectionResponse{
		Status: ch.ConnectionStatus, Error: ch.ConnectionError,
		Fields: map[string]structs.ChannelConnectionField{},
	}
	if out.Status == "" {
		out.Status = "none"
	}
	if ch.ConnectionCheckedAt != nil {
		out.CheckedAt = ch.ConnectionCheckedAt.UTC().Format(saleTimeLayout)
	}
	ad, ada := providerAdapters[ch.Provider]
	if !ada || len(ch.CredentialsEncrypted) == 0 {
		return out
	}
	out.Provider = ch.Provider
	for _, f := range ad.Info().Fields {
		v := cred[f.Key]
		field := structs.ChannelConnectionField{Set: v != ""}
		if f.Secret {
			if v != "" {
				field.Preview = pratinjau(v)
			}
		} else {
			field.Value = v
		}
		out.Fields[f.Key] = field
	}
	if ch.WebhookToken != nil {
		out.WebhookURL = webhookURL(ch.Provider, *ch.WebhookToken)
	}
	out.WebhookValues = ad.WebhookValues(cred, out.WebhookURL)
	if auth, ok := ad.(authorizer); ok {
		out.NeedsAuthorization = true
		out.Authorized = auth.Diotorisasi(cred)
	}
	if t, err := repositories.LastChannelEventAt(ctx, ch.ID); err == nil && t != nil {
		out.LastEventAt = t.UTC().Format(saleTimeLayout)
	}
	return out
}

// GetChannelConnection: status sambungan & isian (rahasia hanya pratinjau).
func GetChannelConnection(ctx context.Context, channelID string) (structs.ChannelConnectionResponse, error) {
	ch, err := repositories.FindChannel(ctx, nil, channelID)
	if err != nil {
		return structs.ChannelConnectionResponse{}, err
	}
	cred, err := bukaKredensial(ch)
	if err != nil {
		return structs.ChannelConnectionResponse{}, err
	}
	return connectionResponse(ctx, ch, cred), nil
}

// SaveChannelConnection menyimpan kredensial tenant (terenkripsi). Isian rahasia
// yang dikosongkan memakai nilai tersimpan — kecuali penyedianya berganti.
// Status kembali "none": kredensial baru harus dites ulang.
func SaveChannelConnection(ctx context.Context, channelID string, in structs.ChannelConnectionRequest) (structs.ChannelConnectionResponse, error) {
	var out structs.ChannelConnectionResponse
	code := strings.ToLower(strings.TrimSpace(in.Provider))
	ad, ada := providerAdapters[code]
	if !ada {
		return out, fmt.Errorf("%w: sambungan API untuk %q belum tersedia", helpers.ErrValidation, in.Provider)
	}
	err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		ch, err := repositories.FindChannel(ctx, tx, channelID)
		if err != nil {
			return err
		}
		lama, err := bukaKredensial(ch)
		if err != nil || ch.Provider != code {
			lama = ChannelCredentials{} // penyedia berganti / tak terbaca → mulai bersih
		}
		cred := ChannelCredentials{}
		for k, v := range lama {
			cred[k] = v
		}
		for _, f := range ad.Info().Fields {
			if v := strings.TrimSpace(in.Fields[f.Key]); v != "" {
				cred[f.Key] = v
			}
			if cred[f.Key] == "" && !f.Optional {
				return fmt.Errorf("%w: %s wajib diisi", helpers.ErrValidation, f.Label)
			}
		}
		ad.Prepare(cred)
		// Akun/toko/lingkungan berganti → alamat webhook harus didaftarkan ulang,
		// dan token hasil otorisasi (Shopee) tidak berlaku untuk aplikasi baru.
		for _, k := range []string{"client_id", "outlet_id", "merchant_id", "environment", "phone_number_id", "partner_id", "partner_key", "app_key"} {
			if lama[k] != cred[k] {
				delete(cred, "subscribed_url")
				if sa, ok := ad.(statefulAuth); ok {
					for _, s := range sa.StateKeys() {
						delete(cred, s)
					}
				}
				break
			}
		}
		polos, err := json.Marshal(cred)
		if err != nil {
			return err
		}
		sandi, err := rahasia.Tutup(polos)
		if err != nil {
			return err
		}
		ch.Provider = code
		ch.MerchantRef = ad.MerchantRef(cred)
		ch.IntegrationMode = "api"
		ch.CredentialsEncrypted = sandi
		if ch.WebhookToken == nil {
			t, err := tokenAcak()
			if err != nil {
				return err
			}
			ch.WebhookToken = &t
		}
		ch.ConnectionStatus, ch.ConnectionError, ch.ConnectionCheckedAt = "none", "", nil
		if err := repositories.SaveChannelConnection(ctx, tx, &ch); err != nil {
			if helpers.IsDuplicateEntryError(err) {
				return fmt.Errorf("%w: akun %s ini sudah tersambung ke kanal lain", helpers.ErrConflict, ad.Info().Name)
			}
			return err
		}
		out = connectionResponse(ctx, ch, cred)
		return nil
	})
	return out, err
}

// denganKredensialTerkunci menjalankan fn atas kredensial kanal dengan baris
// kanalnya TERKUNCI (FOR UPDATE), lalu menyimpan kredensial bila fn
// mengubahnya (mis. token baru) beserta status sambungan pada ch. Pembaruan
// token yang bersamaan antre di sini — refresh token Shopee sekali pakai.
func denganKredensialTerkunci(ctx context.Context, channelID string, fn func(ch *models.Channel, ad ProviderAdapter, cred ChannelCredentials) error) error {
	return repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		ch, err := repositories.FindChannelForUpdate(ctx, tx, channelID)
		if err != nil {
			return err
		}
		ad, ada := providerAdapters[ch.Provider]
		if !ada || len(ch.CredentialsEncrypted) == 0 {
			return fmt.Errorf("%w: kanal ini belum disambungkan", helpers.ErrValidation)
		}
		cred, err := bukaKredensial(ch)
		if err != nil {
			return err
		}
		sebelum, _ := json.Marshal(cred)
		lamaStatus, lamaGalat, lamaCek := ch.ConnectionStatus, ch.ConnectionError, ch.ConnectionCheckedAt
		ferr := fn(&ch, ad, cred)
		sesudah, _ := json.Marshal(cred)
		berubah := string(sebelum) != string(sesudah)
		if berubah {
			sandi, err := rahasia.Tutup(sesudah)
			if err != nil {
				return err
			}
			ch.CredentialsEncrypted = sandi
			ch.MerchantRef = ad.MerchantRef(cred) // mis. shop_id Shopee baru diketahui setelah otorisasi
		}
		if berubah || ch.ConnectionStatus != lamaStatus || ch.ConnectionError != lamaGalat || ch.ConnectionCheckedAt != lamaCek {
			if err := repositories.SaveChannelConnection(ctx, tx, &ch); err != nil {
				if helpers.IsDuplicateEntryError(err) {
					return fmt.Errorf("%w: toko ini sudah tersambung ke kanal lain", helpers.ErrConflict)
				}
				return err
			}
		}
		return ferr
	})
}

// TestChannelConnection memanggil API penyedia dengan kredensial tersimpan dan
// mencatat hasilnya — kegagalan penyedia BUKAN galat HTTP; statusnya "error"
// dengan pesan dari penyedia supaya tenant tahu isian mana yang salah.
func TestChannelConnection(ctx context.Context, channelID string) (structs.ChannelConnectionResponse, error) {
	var out structs.ChannelConnectionResponse
	var info string
	var akhir models.Channel
	var credAkhir ChannelCredentials
	err := denganKredensialTerkunci(ctx, channelID, func(ch *models.Channel, ad ProviderAdapter, cred ChannelCredentials) error {
		tctx, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
		i, terr := ad.Test(tctx, cred)
		now := time.Now().UTC()
		ch.ConnectionCheckedAt = &now
		if terr != nil {
			ch.ConnectionStatus, ch.ConnectionError = "error", terr.Error()
		} else {
			ch.ConnectionStatus, ch.ConnectionError = "connected", ""
			i = daftarkanWebhook(tctx, ad, ch, cred, i)
		}
		info, akhir, credAkhir = i, *ch, cred
		return nil
	})
	if err != nil {
		return out, err
	}
	out = connectionResponse(ctx, akhir, credAkhir)
	out.Info = info
	return out, nil
}

// AuthorizeChannelConnection: alamat otorisasi toko di penyedia (Shopee),
// dengan state acak yang dicatat untuk mencocokkan callback-nya.
func AuthorizeChannelConnection(ctx context.Context, channelID string) (string, error) {
	var alamat string
	err := denganKredensialTerkunci(ctx, channelID, func(ch *models.Channel, ad ProviderAdapter, cred ChannelCredentials) error {
		auth, bisa := ad.(authorizer)
		if !bisa {
			return fmt.Errorf("%w: penyedia ini tidak memakai otorisasi toko", helpers.ErrValidation)
		}
		if ch.WebhookToken == nil {
			return fmt.Errorf("%w: simpan kredensialnya dulu", helpers.ErrValidation)
		}
		base := webhookURL(ch.Provider, *ch.WebhookToken)
		if !strings.HasPrefix(base, "http") {
			return fmt.Errorf("%w: alamat publik server (APP_URL) belum diatur — penyedia tidak bisa mengembalikan toko ke sini", helpers.ErrValidation)
		}
		alamat = auth.AuthorizeURL(cred, base+"/oauth/callback")
		return nil
	})
	return alamat, err
}

// ambilRincianPesanan: rincian pesanan dari API penyedia untuk peristiwa
// ber-NeedsFetch — dipanggil PEKERJA (token diperbarui & disimpan bila perlu).
func ambilRincianPesanan(ctx context.Context, channelID, orderID string) (*NormalizedEvent, error) {
	var ev *NormalizedEvent
	err := denganKredensialTerkunci(ctx, channelID, func(_ *models.Channel, ad ProviderAdapter, cred ChannelCredentials) error {
		f, bisa := ad.(orderFetcher)
		if !bisa {
			return fmt.Errorf("%w: penyedia tidak mendukung pengambilan rincian pesanan", helpers.ErrValidation)
		}
		tctx, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
		var err error
		ev, err = f.FetchOrder(tctx, cred, orderID)
		return err
	})
	return ev, err
}

// daftarkanWebhook: untuk penyedia yang mendukungnya, alamat webhook kanal
// didaftarkan lewat API — sekali per alamat (dicatat di kredensial). Gagal
// mendaftar tidak menggagalkan tes koneksi; alasannya ikut di info supaya
// tenant bisa mendaftarkannya manual.
func daftarkanWebhook(ctx context.Context, ad ProviderAdapter, ch *models.Channel, cred ChannelCredentials, info string) string {
	sub, bisa := ad.(webhookSubscriber)
	if !bisa || ch.WebhookToken == nil {
		return info
	}
	alamat := webhookURL(ch.Provider, *ch.WebhookToken)
	tambah := func(s string) string { return strings.Trim(info+" · "+s, " ·") }
	if !strings.HasPrefix(alamat, "https://") {
		return tambah("webhook belum didaftarkan: alamat publik HTTPS server (APP_URL) belum diatur")
	}
	if cred["subscribed_url"] == alamat {
		return tambah("webhook terdaftar")
	}
	if err := sub.SubscribeWebhook(ctx, cred, alamat); err != nil {
		return tambah("webhook belum terdaftar: " + err.Error())
	}
	cred["subscribed_url"] = alamat
	if polos, err := json.Marshal(cred); err == nil {
		if sandi, err := rahasia.Tutup(polos); err == nil {
			ch.CredentialsEncrypted = sandi
		}
	}
	return tambah("webhook didaftarkan")
}

// DisconnectChannel menghapus kredensial & alamat webhook; kanal kembali manual.
// Pesanan yang sudah tercatat tidak disentuh.
func DisconnectChannel(ctx context.Context, channelID string) error {
	return repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		ch, err := repositories.FindChannel(ctx, tx, channelID)
		if err != nil {
			return err
		}
		ch.MerchantRef, ch.IntegrationMode = "", "manual"
		ch.CredentialsEncrypted, ch.WebhookToken = nil, nil
		ch.ConnectionStatus, ch.ConnectionError, ch.ConnectionCheckedAt = "none", "", nil
		return repositories.SaveChannelConnection(ctx, tx, &ch)
	})
}

func tokenAcak() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// kanalWebhook menemukan kanal tersambung dari alamat webhook-nya.
func kanalWebhook(ctx context.Context, provider, token string) (models.Channel, ProviderAdapter, ChannelCredentials, error) {
	ch, err := repositories.FindChannelByWebhookToken(ctx, token)
	if err != nil {
		return ch, nil, nil, err
	}
	ad, ada := providerAdapters[ch.Provider]
	if !ada || ch.Provider != strings.ToLower(provider) || !ch.IsActive {
		return ch, nil, nil, repositories.ErrChannelNotFound
	}
	cred, err := bukaKredensial(ch)
	if err != nil {
		return ch, nil, nil, err
	}
	return ch, ad, cred, nil
}

// jalankanAksi: sub-jalur yang ditangani adaptor sendiri. Perubahan kredensial
// (mis. token hasil otorisasi) disimpan dengan baris kanal terkunci.
func jalankanAksi(ctx context.Context, ch models.Channel, ad ProviderAdapter, cred ChannelCredentials, p WebhookPermintaan) (*WebhookBalasan, error) {
	act, bisa := ad.(webhookActor)
	if !bisa {
		return nil, nil
	}
	sebelum, _ := json.Marshal(cred)
	balasan, ditangani := act.WebhookAction(ctx, p, cred)
	if !ditangani {
		return nil, nil
	}
	if sesudah, _ := json.Marshal(cred); string(sebelum) != string(sesudah) {
		// Hanya SELISIH yang diterapkan ke baris terkunci: kunci yang tidak
		// disentuh aksi tidak menimpa pembaruan pekerja (token baru), dan kunci
		// yang dihapus aksi (state otorisasi sekali pakai) ikut terhapus.
		var awal ChannelCredentials
		_ = json.Unmarshal(sebelum, &awal)
		tctx := reqctx.WithTenantID(ctx, ch.TenantID)
		err := denganKredensialTerkunci(tctx, ch.ID, func(c *models.Channel, _ ProviderAdapter, simpan ChannelCredentials) error {
			for k, v := range cred {
				if lama, ada := awal[k]; !ada || lama != v {
					simpan[k] = v
				}
			}
			for k := range awal {
				if _, ada := cred[k]; !ada {
					delete(simpan, k)
				}
			}
			if balasan.Status == http.StatusOK {
				now := time.Now().UTC()
				c.ConnectionStatus, c.ConnectionError, c.ConnectionCheckedAt = "connected", "", &now
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return &balasan, nil
}

// ProviderWebhookGet: GET ke alamat webhook — verifikasi alamat (WhatsApp) atau
// sub-jalur adaptor (callback otorisasi Shopee). ok=false → 403.
func ProviderWebhookGet(ctx context.Context, provider, token, aksi string, q url.Values, h http.Header) (*WebhookBalasan, string, bool) {
	ch, ad, cred, err := kanalWebhook(ctx, provider, token)
	if err != nil {
		return nil, "", false
	}
	p := WebhookPermintaan{Metode: http.MethodGet, Aksi: strings.Trim(aksi, "/"), Query: q, Header: h,
		Alamat: webhookURL(ch.Provider, token)}
	if balasan, err := jalankanAksi(ctx, ch, ad, cred, p); err != nil {
		return &WebhookBalasan{Status: http.StatusInternalServerError, HTML: halamanPesan("Gagal menyimpan", err.Error())}, "", true
	} else if balasan != nil {
		return balasan, "", true
	}
	body, ok := ad.VerifyChallenge(q, cred)
	return nil, body, ok
}

// IngestProviderWebhook: tanda tangan diperiksa dengan rahasia milik kanal itu,
// lalu tiap peristiwa disimpan (idempoten) untuk diproses pekerja — tidak ada
// pemrosesan di jalur webhook (blueprint F.6 aturan 1). Sub-jalur yang
// ditangani adaptor sendiri (mis. token OAuth GrabFood) dibalas langsung.
func IngestProviderWebhook(ctx context.Context, provider, token, aksi string, h http.Header, body []byte) (structs.ProviderWebhookResult, *WebhookBalasan, error) {
	var res structs.ProviderWebhookResult
	ch, ad, cred, err := kanalWebhook(ctx, provider, token)
	if errors.Is(err, repositories.ErrChannelNotFound) {
		return res, nil, ErrWebhookUnauthorized
	}
	if err != nil {
		return res, nil, err
	}
	alamat := webhookURL(ch.Provider, token)
	p := WebhookPermintaan{Metode: http.MethodPost, Aksi: strings.Trim(aksi, "/"), Header: h, Body: body, Alamat: alamat}
	if balasan, err := jalankanAksi(ctx, ch, ad, cred, p); err != nil || balasan != nil {
		return res, balasan, err
	}
	if err := ad.VerifySignature(h, body, cred, alamat); err != nil {
		return res, nil, ErrWebhookUnauthorized
	}
	var kosong *WebhookBalasan
	if a, bisa := ad.(ackKosong); bisa && a.AckKosong() {
		kosong = &WebhookBalasan{Status: http.StatusOK, Kosong: true}
	}
	if ic, bisa := ad.(izinDicabut); bisa {
		if pesan, dicabut := ic.IzinDicabut(body, cred); dicabut {
			tctx := reqctx.WithTenantID(ctx, ch.TenantID)
			err := denganKredensialTerkunci(tctx, ch.ID, func(c *models.Channel, _ ProviderAdapter, simpan ChannelCredentials) error {
				for _, k := range []string{"access_token", "refresh_token", "access_expires", "subscribed_url"} {
					delete(simpan, k)
				}
				now := time.Now().UTC()
				c.ConnectionStatus, c.ConnectionError, c.ConnectionCheckedAt = "error", pesan, &now
				return nil
			})
			if err != nil {
				return res, nil, err
			}
			res.Ignored = "izin toko dicabut"
			return res, kosong, nil
		}
	}
	evs, err := ad.Events(body, cred)
	if err != nil {
		res.Ignored = "payload tidak dapat diurai: " + err.Error()
		return res, kosong, nil
	}
	if len(evs) == 0 {
		res.Ignored = "bukan pesanan"
		return res, kosong, nil
	}
	for _, ev := range evs {
		payload, err := genericFromEvent(ev, body)
		if err != nil {
			return res, nil, err
		}
		tipe := normalizeEventType(ev.EventType)
		// Status berurutan (dikirim, diterima, selesai) untuk pesanan yang
		// sama adalah peristiwa berbeda — tanpa akhiran status, yang kedua
		// dan seterusnya dibuang sebagai duplikat.
		ref := ev.ExternalOrderID
		if tipe == "order.status" && ev.ExternalStatus != "" {
			ref += "#" + ev.ExternalStatus
		}
		created, err := repositories.InsertChannelEvent(ctx, ch.TenantID, ch.ID, tipe, ref, payload)
		if err != nil {
			return res, nil, err
		}
		if created {
			res.Received++
		} else {
			res.Duplicate++
		}
	}
	return res, kosong, nil
}

// halamanPesan: halaman HTML singkat untuk peramban yang kembali dari penyedia.
func halamanPesan(judul, isi string) string {
	return "<!doctype html><html lang=\"id\"><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width,initial-scale=1\">" +
		"<title>" + html.EscapeString(judul) + "</title><style>body{font-family:system-ui,sans-serif;background:#f7f7f5;color:#1a1a1a;display:grid;place-items:center;min-height:100vh;margin:0;padding:16px}" +
		"main{background:#fff;border:1px solid #e5e5e0;border-radius:12px;padding:24px;max-width:420px;text-align:center}h1{font-size:20px;margin:0 0 8px}p{margin:0;color:#555;line-height:1.5}</style></head>" +
		"<body><main><h1>" + html.EscapeString(judul) + "</h1><p>" + html.EscapeString(isi) + "</p></main></body></html>"
}
