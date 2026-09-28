package services

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"candra/backend-api/config"
	"candra/backend-api/helpers"
	"candra/backend-api/models"
	"candra/backend-api/repositories"
	"candra/backend-api/structs"

	"gorm.io/gorm"
)

// Menu aplikasi antar DARI POS (GoFood, GrabFood).
//
// Ketersediaan item di aplikasi antar hanya bisa diubah lewat API untuk item
// yang ber-ID partner — dan ID itu hanya ada bila menunya diunggah partner.
// Karena itu menu dikelola di POS: barang yang dipilih untuk kanal (satu baris
// channel_products per item; ID item = SKU barang, atau "P-<id>" bila kosong)
// dikirim utuh ke penyedia — GoFood menerima katalog lewat PUT, Grab diberi tahu
// lalu mengambil menu dari alamat webhook kanal. Mengirim menu MENGGANTI menu
// di aplikasi antar, jadi wajib dikonfirmasi. Setelah itu habis/tersedia (dan
// jumlah stok di Grab) ikut otomatis lewat sinkron stok.

// ItemMenu: satu item menu yang dikirim.
type ItemMenu struct {
	ID, Nama, Foto string
	Harga          int64 // rupiah
	Tersedia       bool
	Stok           *int64 // nil = stok tidak dilacak (mis. masakan dibuat saat dipesan)
}

type KategoriMenu struct {
	ID, Nama string
	Item     []ItemMenu
}

type MenuKanal struct{ Kategori []KategoriMenu }

func (m MenuKanal) JumlahItem() int {
	n := 0
	for _, k := range m.Kategori {
		n += len(k.Item)
	}
	return n
}

// menuPublisher: aplikasi antar yang menunya bisa dikirim dari POS.
type menuPublisher interface {
	PublishMenu(ctx context.Context, cred ChannelCredentials, menu MenuKanal) error
}

// menuServer: penyedia yang MENGAMBIL menu dari alamat webhook kanal (Grab).
type menuServer interface {
	MenuAksi(aksi string) bool
	CekAksesMenu(h http.Header, cred ChannelCredentials) bool
	ServeMenu(q url.Values, cred ChannelCredentials, menu MenuKanal) any
}

// idItemMenu: ID item di penyedia — SKU barang (juga dipakai memetakan pesanan
// yang masuk), atau "P-<id barang>" bila SKU kosong.
func idItemMenu(p models.Product) string {
	if p.SKU != nil && strings.TrimSpace(*p.SKU) != "" {
		return strings.TrimSpace(*p.SKU)
	}
	return "P-" + p.ID
}

// fotoPublik: URL foto yang bisa diunduh penyedia (alamat relatif dilengkapi
// APP_URL; selain http(s) tidak dikirim).
func fotoPublik(u string) string {
	u = strings.TrimSpace(u)
	if strings.HasPrefix(u, "/") {
		u = strings.TrimRight(config.GetEnv("APP_URL", ""), "/") + u
	}
	if strings.HasPrefix(u, "https://") || strings.HasPrefix(u, "http://") {
		return u
	}
	return ""
}

const kategoriLain = "Lainnya"

// bangunMenu menyusun menu kanal dari pemetaannya, barang, dan stok outlet.
func bangunMenu(ctx context.Context, ch models.Channel) (MenuKanal, error) {
	var menu MenuKanal
	cps, err := repositories.ChannelProductsAll(ctx, nil, ch.ID)
	if err != nil {
		return menu, err
	}
	ids := make([]string, 0, len(cps))
	for _, cp := range cps {
		ids = append(ids, cp.ProductID)
	}
	prods, err := repositories.ProductsByIDs(ctx, nil, ids)
	if err != nil {
		return menu, err
	}
	kat, err := repositories.CategoryNames(ctx)
	if err != nil {
		return menu, err
	}
	grup := map[string]*KategoriMenu{}
	for _, cp := range cps {
		p, ada := prods[cp.ProductID]
		if !ada || !p.IsActive {
			continue
		}
		item := ItemMenu{ID: cp.ExternalSKU, Nama: p.Name, Harga: p.SellPrice, Foto: fotoPublik(p.ImageURL)}
		if cp.ChannelPrice != nil {
			item.Harga = *cp.ChannelPrice
		}
		if p.TrackStock {
			vid := ""
			if cp.VariantID != nil {
				vid = *cp.VariantID
			}
			stok, err := repositories.CurrentStockQty(ctx, nil, ch.OutletID, p.ID, vid)
			if err != nil {
				return menu, err
			}
			q := qtyKirim(stok, cp)
			item.Stok, item.Tersedia = &q, q > 0
		} else {
			item.Tersedia = cp.IsAvailable
		}
		kid, nama := "lainnya", kategoriLain
		if p.CategoryID != nil && kat[*p.CategoryID] != "" {
			kid, nama = *p.CategoryID, kat[*p.CategoryID]
		}
		if grup[kid] == nil {
			grup[kid] = &KategoriMenu{ID: kid, Nama: nama}
		}
		grup[kid].Item = append(grup[kid].Item, item)
	}
	for _, k := range grup {
		sort.Slice(k.Item, func(i, j int) bool { return k.Item[i].Nama < k.Item[j].Nama })
		menu.Kategori = append(menu.Kategori, *k)
	}
	sort.Slice(menu.Kategori, func(i, j int) bool {
		a, b := menu.Kategori[i], menu.Kategori[j]
		if (a.Nama == kategoriLain) != (b.Nama == kategoriLain) {
			return b.Nama == kategoriLain
		}
		return a.Nama < b.Nama
	})
	return menu, nil
}

func menuDidukung(ch models.Channel) bool {
	_, bisa := providerAdapters[ch.Provider].(menuPublisher)
	return bisa && ch.IntegrationMode == "api" && len(ch.CredentialsEncrypted) > 0
}

// MenuKanalUntukAtur: GET /channels/:id/menu — barang aktif + tanda di menu.
func MenuKanalUntukAtur(ctx context.Context, channelID string) (structs.ChannelMenuResponse, error) {
	var out structs.ChannelMenuResponse
	ch, err := repositories.FindChannel(ctx, nil, channelID)
	if err != nil {
		return out, err
	}
	out.Supported = menuDidukung(ch)
	if cred, err := bukaKredensial(ch); err == nil {
		out.PublishedAt = cred["menu_published_at"]
		out.PublishedCount, _ = strconv.Atoi(cred["menu_published_count"])
	}
	prods, err := repositories.ActiveProductsForMenu(ctx)
	if err != nil {
		return out, err
	}
	kat, err := repositories.CategoryNames(ctx)
	if err != nil {
		return out, err
	}
	cps, err := repositories.ChannelProductsAll(ctx, nil, channelID)
	if err != nil {
		return out, err
	}
	diMenu := map[string]models.ChannelProduct{}
	for _, cp := range cps {
		diMenu[cp.ProductID] = cp
	}
	out.Items = []structs.ChannelMenuItem{}
	for _, p := range prods {
		cp, ada := diMenu[p.ID]
		it := structs.ChannelMenuItem{
			ProductID: p.ID, Name: p.Name, Category: kategoriLain, Price: p.SellPrice, ImageURL: p.ImageURL,
			InMenu: ada, Available: !ada || cp.IsAvailable, TrackStock: p.TrackStock,
		}
		if p.SKU != nil {
			it.SKU = *p.SKU
		}
		if p.CategoryID != nil && kat[*p.CategoryID] != "" {
			it.Category = kat[*p.CategoryID]
		}
		if ada && cp.ChannelPrice != nil {
			it.Price = *cp.ChannelPrice
		}
		out.Items = append(out.Items, it)
	}
	sort.SliceStable(out.Items, func(i, j int) bool {
		a, b := out.Items[i], out.Items[j]
		if a.Category != b.Category {
			if (a.Category == kategoriLain) != (b.Category == kategoriLain) {
				return b.Category == kategoriLain
			}
			return a.Category < b.Category
		}
		return a.Name < b.Name
	})
	return out, nil
}

// SimpanMenuKanal menetapkan isi menu kanal: barang terpilih mendapat
// pemetaan (ID item = SKU), yang tidak dipilih dilepas dari menu. Belum
// dikirim — kirim menu adalah langkah terpisah yang dikonfirmasi.
func SimpanMenuKanal(ctx context.Context, channelID string, productIDs []string) (structs.ChannelMenuResponse, error) {
	err := repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		ch, err := repositories.FindChannel(ctx, tx, channelID)
		if err != nil {
			return err
		}
		if !menuDidukung(ch) {
			return fmt.Errorf("%w: menu hanya untuk GoFood/GrabFood yang tersambung API", helpers.ErrValidation)
		}
		mau := map[string]bool{}
		for _, id := range productIDs {
			mau[id] = true
		}
		prods, err := repositories.ProductsByIDs(ctx, tx, productIDs)
		if err != nil {
			return err
		}
		for id := range mau {
			if p, ada := prods[id]; !ada || !p.IsActive {
				return fmt.Errorf("%w: barang %s tidak ditemukan atau nonaktif", helpers.ErrValidation, id)
			}
		}
		cps, err := repositories.ChannelProductsAll(ctx, tx, channelID)
		if err != nil {
			return err
		}
		sudah := map[string]bool{}
		dipakai := map[string]bool{}
		for _, cp := range cps {
			if !mau[cp.ProductID] {
				if err := repositories.DeleteChannelProduct(ctx, tx, cp.ID); err != nil {
					return err
				}
				continue
			}
			sudah[cp.ProductID] = true
			dipakai[cp.ExternalSKU] = true
		}
		for _, id := range productIDs {
			if sudah[id] {
				continue
			}
			p := prods[id]
			kode := idItemMenu(p)
			if dipakai[kode] {
				kode = "P-" + p.ID
			}
			cp := models.ChannelProduct{ChannelID: channelID, ProductID: id, ExternalSKU: kode, IsAvailable: true}
			if err := repositories.CreateChannelProduct(ctx, tx, &cp); err != nil {
				return err
			}
			sudah[id], dipakai[kode] = true, true
		}
		return nil
	})
	if err != nil {
		return structs.ChannelMenuResponse{}, err
	}
	return MenuKanalUntukAtur(ctx, channelID)
}

// TerbitkanMenuKanal mengirim menu ke aplikasi antar — MENGGANTI menu di sana.
func TerbitkanMenuKanal(ctx context.Context, channelID string, konfirmasi bool) (structs.ChannelMenuPublishResult, error) {
	var out structs.ChannelMenuPublishResult
	if !konfirmasi {
		return out, fmt.Errorf("%w: konfirmasi dulu — menu di aplikasi antar akan diganti", helpers.ErrValidation)
	}
	ch, err := repositories.FindChannel(ctx, nil, channelID)
	if err != nil {
		return out, err
	}
	pub, bisa := providerAdapters[ch.Provider].(menuPublisher)
	if !bisa || !menuDidukung(ch) {
		return out, fmt.Errorf("%w: menu hanya untuk GoFood/GrabFood yang tersambung API", helpers.ErrValidation)
	}
	if ch.ConnectionStatus != "connected" {
		return out, fmt.Errorf("%w: sambungan API belum tersambung — tes koneksi dulu", helpers.ErrValidation)
	}
	menu, err := bangunMenu(ctx, ch)
	if err != nil {
		return out, err
	}
	if menu.JumlahItem() == 0 {
		return out, fmt.Errorf("%w: pilih barang untuk menu dulu", helpers.ErrValidation)
	}
	sekarang := time.Now().UTC()
	err = denganKredensialTerkunci(ctx, ch.ID, func(_ *models.Channel, _ ProviderAdapter, cred ChannelCredentials) error {
		tctx, cancel := context.WithTimeout(ctx, time.Minute)
		defer cancel()
		if err := pub.PublishMenu(tctx, cred, menu); err != nil {
			return fmt.Errorf("%w: %v", helpers.ErrValidation, err)
		}
		cred["menu_published_at"] = sekarang.Format(time.RFC3339)
		cred["menu_published_count"] = strconv.Itoa(menu.JumlahItem())
		return nil
	})
	if err != nil {
		return out, err
	}
	// Item kini ada di penyedia: pengenal kirim stok = ID item. Stok yang baru
	// ikut terkirim bersama menu dicatat sebagai kiriman, supaya pekerja tidak
	// langsung mengirimnya lagi (di Grab, menu baru diambil beberapa saat
	// kemudian — kiriman terlalu cepat justru gagal "item tidak ada").
	err = repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		cps, err := repositories.ChannelProductsAll(ctx, tx, ch.ID)
		if err != nil {
			return err
		}
		var ids []string
		for _, cp := range cps {
			if cp.ExternalProductID != cp.ExternalSKU {
				if err := repositories.SetChannelProductRef(ctx, tx, cp.ID, cp.ExternalSKU); err != nil {
					return err
				}
			}
			ids = append(ids, cp.ID)
			if err := repositories.RecordStockSyncSent(ctx, ch.TenantID, ch.ID, cp.ProductID); err != nil {
				return err
			}
		}
		return repositories.MarkChannelProductSynced(ctx, tx, ids, sekarang)
	})
	if err != nil {
		return out, err
	}
	out.Items, out.Categories = menu.JumlahItem(), len(menu.Kategori)
	out.PublishedAt = sekarang.Format(saleTimeLayout)
	return out, nil
}

// layaniMenu: GET menu yang diambil penyedia dari alamat webhook kanal.
func layaniMenu(ctx context.Context, ch models.Channel, ad ProviderAdapter, cred ChannelCredentials, p WebhookPermintaan) (*WebhookBalasan, bool) {
	ms, bisa := ad.(menuServer)
	if !bisa || p.Metode != http.MethodGet || !ms.MenuAksi(p.Aksi) {
		return nil, false
	}
	if !ms.CekAksesMenu(p.Header, cred) {
		return &WebhookBalasan{Status: http.StatusUnauthorized, Body: map[string]string{"message": "token tidak sah"}}, true
	}
	menu, err := bangunMenu(ctx, ch)
	if err != nil {
		return &WebhookBalasan{Status: http.StatusInternalServerError, Body: map[string]string{"message": "menu gagal disusun"}}, true
	}
	return &WebhookBalasan{Status: http.StatusOK, Body: ms.ServeMenu(p.Query, cred, menu)}, true
}
