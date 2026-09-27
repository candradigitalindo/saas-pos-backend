package services

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"candra/backend-api/config"
	"candra/backend-api/helpers"
	"candra/backend-api/internal/rahasia"
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
	out.WebhookValues = ad.WebhookValues(cred)
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
		// Akun/toko/lingkungan berganti → alamat webhook harus didaftarkan ulang.
		for _, k := range []string{"client_id", "outlet_id", "environment", "phone_number_id"} {
			if lama[k] != cred[k] {
				delete(cred, "subscribed_url")
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

// TestChannelConnection memanggil API penyedia dengan kredensial tersimpan dan
// mencatat hasilnya — kegagalan penyedia BUKAN galat HTTP; statusnya "error"
// dengan pesan dari penyedia supaya tenant tahu isian mana yang salah.
func TestChannelConnection(ctx context.Context, channelID string) (structs.ChannelConnectionResponse, error) {
	var out structs.ChannelConnectionResponse
	ch, err := repositories.FindChannel(ctx, nil, channelID)
	if err != nil {
		return out, err
	}
	ad, ada := providerAdapters[ch.Provider]
	if !ada || len(ch.CredentialsEncrypted) == 0 {
		return out, fmt.Errorf("%w: kanal ini belum disambungkan", helpers.ErrValidation)
	}
	cred, err := bukaKredensial(ch)
	if err != nil {
		return out, err
	}
	tctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	info, terr := ad.Test(tctx, cred)
	now := time.Now().UTC()
	ch.ConnectionCheckedAt = &now
	if terr != nil {
		ch.ConnectionStatus, ch.ConnectionError = "error", terr.Error()
	} else {
		ch.ConnectionStatus, ch.ConnectionError = "connected", ""
		info = daftarkanWebhook(tctx, ad, &ch, cred, info)
	}
	err = repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		return repositories.SaveChannelConnection(ctx, tx, &ch)
	})
	if err != nil {
		return out, err
	}
	out = connectionResponse(ctx, ch, cred)
	out.Info = info
	return out, nil
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

// ProviderWebhookChallenge menjawab GET verifikasi alamat webhook.
func ProviderWebhookChallenge(ctx context.Context, provider, token string, q url.Values) (string, bool) {
	_, ad, cred, err := kanalWebhook(ctx, provider, token)
	if err != nil {
		return "", false
	}
	return ad.VerifyChallenge(q, cred)
}

// IngestProviderWebhook: tanda tangan diperiksa dengan rahasia milik kanal itu,
// lalu tiap peristiwa disimpan (idempoten) untuk diproses pekerja — tidak ada
// pemrosesan di jalur webhook (blueprint F.6 aturan 1).
func IngestProviderWebhook(ctx context.Context, provider, token string, h http.Header, body []byte) (structs.ProviderWebhookResult, error) {
	var res structs.ProviderWebhookResult
	ch, ad, cred, err := kanalWebhook(ctx, provider, token)
	if errors.Is(err, repositories.ErrChannelNotFound) {
		return res, ErrWebhookUnauthorized
	}
	if err != nil {
		return res, err
	}
	if err := ad.VerifySignature(h, body, cred); err != nil {
		return res, ErrWebhookUnauthorized
	}
	evs, err := ad.Events(body, cred)
	if err != nil {
		res.Ignored = "payload tidak dapat diurai: " + err.Error()
		return res, nil
	}
	if len(evs) == 0 {
		res.Ignored = "bukan pesanan"
		return res, nil
	}
	for _, ev := range evs {
		payload, err := genericFromEvent(ev, body)
		if err != nil {
			return res, err
		}
		created, err := repositories.InsertChannelEvent(ctx, ch.TenantID, ch.ID,
			normalizeEventType(ev.EventType), ev.ExternalOrderID, payload)
		if err != nil {
			return res, err
		}
		if created {
			res.Received++
		} else {
			res.Duplicate++
		}
	}
	return res, nil
}
