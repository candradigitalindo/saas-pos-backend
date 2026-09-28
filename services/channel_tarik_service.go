package services

import (
	"context"
	"sort"
	"time"

	"candra/backend-api/helpers"
	"candra/backend-api/internal/reqctx"
	"candra/backend-api/models"
	"candra/backend-api/repositories"
)

// Tarikan berkala — CADANGAN push, bukan penggantinya.
//
// Lazada mengirim push hanya ke HTTPS bersertifikat OV/EV (DV seperti
// Let's Encrypt ditolak) dan menganjurkan "terima push, tarik berfrekuensi
// rendah supaya tidak ada yang terlewat". Penyedia yang mendukungnya
// (orderPuller) ditarik tiap JedaTarikPesanan oleh pekerja
// process-channel-events; hasilnya masuk antrean yang SAMA dengan webhook
// (dedup per nomor pesanan), jadi pesanan yang datang lewat keduanya tercatat
// sekali.

// orderPuller: penyedia yang pesanan berubahnya bisa ditarik sejak waktu
// tertentu. sampai = batas kursor berikutnya.
type orderPuller interface {
	PullOrders(ctx context.Context, cred ChannelCredentials, sejak time.Time) (evs []NormalizedEvent, sampai time.Time, err error)
}

// JedaTarikPesanan: frekuensi rendah — penyedia melarang menarik daftar
// pesanan terus-menerus.
const JedaTarikPesanan = 15 * time.Minute

// tumpangTarik: jendela tarikan mundur sedikit dari kursor supaya pesanan
// yang berubah tepat di batas tidak terlewat (dedup menangani ulangannya).
const tumpangTarik = 5 * time.Minute

// TarikPesananKanal menarik pesanan yang berubah sejak tarikan terakhir untuk
// setiap kanal tersambung yang penyedianya mendukung. Kanal yang ditarik
// kurang dari JedaTarikPesanan lalu dilewati. Galat penyedia dicatat di log,
// tidak menghentikan kanal lain.
func TarikPesananKanal(ctx context.Context, now time.Time) (masuk int, err error) {
	var kode []string
	for k, ad := range providerAdapters {
		if _, bisa := ad.(orderPuller); bisa {
			kode = append(kode, k)
		}
	}
	if len(kode) == 0 {
		return 0, nil
	}
	sort.Strings(kode)
	chs, err := repositories.ChannelsForPull(ctx, kode)
	if err != nil {
		return 0, err
	}
	log := helpers.LoggerFromContext(ctx)
	for _, ch := range chs {
		tctx := reqctx.WithTenantID(ctx, ch.TenantID)
		e := denganKredensialTerkunci(tctx, ch.ID, func(c *models.Channel, ad ProviderAdapter, cred ChannelCredentials) error {
			p, bisa := ad.(orderPuller)
			if !bisa || cred["refresh_token"] == "" {
				return nil
			}
			if t, err := time.Parse(time.RFC3339, cred["pulled_at"]); err == nil && now.Sub(t) < JedaTarikPesanan {
				return nil
			}
			// Tarikan pertama mulai dari saat toko memberi izin — pesanan
			// sebelum itu mungkin sudah dicatat manual/CSV.
			sejak, err := time.Parse(time.RFC3339, cred["pulled_until"])
			if err != nil {
				if sejak, err = time.Parse(time.RFC3339, cred["authorized_at"]); err != nil {
					sejak = now.Add(-time.Hour)
				}
			}
			pctx, cancel := context.WithTimeout(ctx, time.Minute)
			defer cancel()
			evs, sampai, perr := p.PullOrders(pctx, cred, sejak.Add(-tumpangTarik))
			cred["pulled_at"] = now.UTC().Format(time.RFC3339)
			if perr != nil {
				log.Warn("tarik pesanan kanal gagal", "channel_id", c.ID, "provider", c.Provider, "error", perr)
				return nil
			}
			for _, ev := range evs {
				created, err := simpanPeristiwa(ctx, *c, ev, nil)
				if err != nil {
					return err
				}
				if created {
					masuk++
				}
			}
			if !sampai.IsZero() {
				cred["pulled_until"] = sampai.UTC().Format(time.RFC3339)
			}
			return nil
		})
		if e != nil {
			log.Warn("tarik pesanan kanal gagal disimpan", "channel_id", ch.ID, "error", e)
		}
	}
	return masuk, nil
}
