package services

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"candra/backend-api/helpers"
	"candra/backend-api/internal/reqctx"
	"candra/backend-api/models"
	"candra/backend-api/repositories"
	"candra/backend-api/structs"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// Sinkron stok ke marketplace — penangkal overselling.
//
// Stok toko berubah di banyak tempat (kasir, retur, penyesuaian, transfer,
// opname, pesanan kanal lain). Alih-alih menyisipkan kode di setiap alur,
// pekerja mencocokkan dari stock_movements: barang TERPETAKAN yang stoknya
// bergerak setelah antrean terakhirnya diantre, lalu stok terkininya dikirim ke
// API penyedia (channel_stock_syncs → status sent/failed, terlihat pemilik).
//
// Hanya barang yang sudah dipetakan (channel_products ber-ref) yang dikirim —
// toko dengan 500 barang mungkin hanya memajang 50 di satu marketplace, dan
// 450 lainnya tidak boleh menjadi galat "SKU tidak ada". Pemetaan dibuat
// otomatis lewat "Cocokkan barang": daftar produk penyedia ditarik sekali lalu
// dicocokkan dengan SKU/barcode barang toko.
//
// Yang dikirim = stok outlet kanal − penyangga (stock_buffer), dibulatkan ke
// bawah, minimal 0; barang yang ditandai tidak tersedia dikirim 0.

// ListingPenyedia: satu SKU yang dijual toko di penyedia.
type ListingPenyedia struct {
	SKU  string
	Ref  string // pengenal kirim stok, dibentuk adaptor (mis. "item:model")
	Nama string
}

// StokKirim: stok untuk satu ref penyedia.
type StokKirim struct {
	Ref string
	Qty int64
	// Lacak=false: barang tanpa pelacakan stok (mis. masakan dibuat saat
	// dipesan) — tidak punya angka stok; yang berarti hanya Tersedia.
	Lacak    bool
	Tersedia bool
}

// tanpaAngka: barang tak dilacak yang tersedia — marketplace (yang butuh
// angka stok) melewatinya; mengirim 0 justru menutup barang yang masih dijual.
func tanpaAngka(s StokKirim) bool { return !s.Lacak && s.Tersedia }

// stockPusher: penyedia yang bisa menerima stok/ketersediaan dari toko.
type stockPusher interface {
	// PushStock mengirim stok; galat per ref (tidak ada entri = berhasil).
	PushStock(ctx context.Context, cred ChannelCredentials, stok []StokKirim) map[string]error
}

// listingLister: penyedia yang daftar produknya bisa ditarik untuk
// dicocokkan (marketplace). Aplikasi antar memakai menu dari POS.
type listingLister interface {
	ListListings(ctx context.Context, cred ChannelCredentials) ([]ListingPenyedia, error)
}

// pemisahRef: satu SKU bisa ada di lebih dari satu listing penyedia.
const pemisahRef = ";"

// maksPercobaanStok: sesudahnya antrean ditandai gagal (terlihat pemilik) dan
// baru diantre lagi saat stok barangnya bergerak.
const maksPercobaanStok = 5

// CocokkanBarangKanal menarik daftar produk penyedia lalu memetakan setiap
// SKU-nya ke barang toko (SKU/barcode yang sama). Pemetaan manual yang sudah
// ada dipertahankan (hanya ref-nya diperbarui). Barang yang terpetakan langsung
// diantre untuk sinkron stok pertama.
func CocokkanBarangKanal(ctx context.Context, channelID string) (structs.ChannelProductMatchResult, error) {
	var out structs.ChannelProductMatchResult
	tanpaPasangan := func(sku string) {
		out.UnmatchedCount++
		if len(out.Unmatched) < 20 {
			out.Unmatched = append(out.Unmatched, sku)
		}
	}
	var listings []ListingPenyedia
	var ch models.Channel
	err := denganKredensialTerkunci(ctx, channelID, func(c *models.Channel, ad ProviderAdapter, cred ChannelCredentials) error {
		ss, bisa := ad.(listingLister)
		if !bisa {
			return fmt.Errorf("%w: %s tidak mendukung pencocokan barang — atur menunya dari POS", helpers.ErrValidation, ad.Info().Name)
		}
		if c.ConnectionStatus != "connected" {
			return fmt.Errorf("%w: sambungan API belum tersambung — selesaikan otorisasi & tes koneksi dulu", helpers.ErrValidation)
		}
		tctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		var err error
		listings, err = ss.ListListings(tctx, cred)
		ch = *c
		if err != nil {
			return fmt.Errorf("%w: %v", helpers.ErrValidation, err)
		}
		return nil
	})
	if err != nil {
		return out, err
	}
	// SKU → ref (gabungan bila SKU yang sama ada di beberapa listing).
	refs := map[string][]string{}
	var urut []string
	for _, l := range listings {
		sku := strings.TrimSpace(l.SKU)
		if sku == "" || l.Ref == "" {
			out.WithoutSKU++
			continue
		}
		if _, ada := refs[sku]; !ada {
			urut = append(urut, sku)
		}
		refs[sku] = append(refs[sku], l.Ref)
	}
	out.Listings = len(urut)
	var antre []string
	err = repositories.WithTenant(ctx, func(tx *gorm.DB) error {
		for _, sku := range urut {
			ref := strings.Join(refs[sku], pemisahRef)
			if cp, ada, err := repositories.FindChannelProductBySKU(ctx, tx, ch.ID, sku); err != nil {
				return err
			} else if ada {
				if cp.ExternalProductID != ref {
					if err := repositories.SetChannelProductRef(ctx, tx, cp.ID, ref); err != nil {
						return err
					}
				}
				out.Matched++
				antre = append(antre, cp.ProductID)
				continue
			}
			pid, err := repositories.FindProductIDByCode(ctx, tx, sku)
			if errors.Is(err, repositories.ErrAmbiguousProductCode) {
				tanpaPasangan(sku + " (dipakai >1 barang)")
				continue
			}
			if err != nil {
				return err
			}
			if pid == "" {
				tanpaPasangan(sku)
				continue
			}
			cp := models.ChannelProduct{ChannelID: ch.ID, ProductID: pid, ExternalSKU: sku, ExternalProductID: ref, IsAvailable: true}
			if err := repositories.CreateChannelProduct(ctx, tx, &cp); err != nil {
				return err
			}
			out.Matched++
			out.Created++
			antre = append(antre, pid)
		}
		return nil
	})
	if err != nil {
		return out, err
	}
	// Sinkron pertama untuk semua yang terpetakan — stok marketplace bisa jauh
	// berbeda dari stok toko saat baru disambungkan.
	for _, pid := range antre {
		if err := antreStokBarang(ctx, ch, pid); err != nil {
			return out, err
		}
	}
	return out, nil
}

func antreStokBarang(ctx context.Context, ch models.Channel, productID string) error {
	qty, err := repositories.CurrentStockQty(ctx, nil, ch.OutletID, productID, "")
	if err != nil {
		return err
	}
	return repositories.QueueStockSyncOnce(ctx, ch.TenantID, ch.ID, productID, qty.String())
}

// AntrekanSinkronStok: untuk setiap kanal tersambung yang penyedianya bisa
// menerima stok, antre barang terpetakan yang stoknya bergerak.
func AntrekanSinkronStok(ctx context.Context) (int, error) {
	var kode []string
	for k, ad := range providerAdapters {
		if _, bisa := ad.(stockPusher); bisa {
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
	n := 0
	for _, ch := range chs {
		ids, err := repositories.ProductsNeedingStockSync(ctx, ch.TenantID, ch.ID, ch.OutletID)
		if err != nil {
			return n, err
		}
		tctx := reqctx.WithTenantID(ctx, ch.TenantID)
		for _, pid := range ids {
			if err := antreStokBarang(tctx, ch, pid); err != nil {
				return n, err
			}
			n++
		}
	}
	return n, nil
}

// qtyKirim: stok yang ditawarkan ke kanal untuk satu pemetaan.
func qtyKirim(stok decimal.Decimal, cp models.ChannelProduct) int64 {
	if !cp.IsAvailable {
		return 0
	}
	q := stok.Sub(cp.StockBuffer).Floor().IntPart()
	if q < 0 {
		return 0
	}
	return q
}

// hasilStok: hasil kirim satu baris antrean.
type hasilStok struct {
	status, galat, qty string
}

// kirimStokKanal mengirim sekumpulan baris antrean satu kanal.
func kirimStokKanal(ctx context.Context, channelID string, rows []models.ChannelStockSync) map[string]hasilStok {
	out := make(map[string]hasilStok, len(rows))
	semua := func(status, galat string) map[string]hasilStok {
		for _, r := range rows {
			out[r.ID] = hasilStok{status: status, galat: galat}
		}
		return out
	}
	ch, err := repositories.ChannelByIDAnyTenant(ctx, channelID)
	if err != nil {
		return semua("failed", "kanal tidak ditemukan")
	}
	ad := providerAdapters[ch.Provider]
	ss, bisa := ad.(stockPusher)
	if !bisa || ch.IntegrationMode != "api" || len(ch.CredentialsEncrypted) == 0 {
		return semua("failed", "kanal ini tidak tersambung ke API yang menerima stok")
	}
	if ch.ConnectionStatus != "connected" {
		return semua("failed", "sambungan API sedang bermasalah — perbaiki di Sambungan API, lalu stok dikirim lagi saat berubah")
	}
	tctx := reqctx.WithTenantID(ctx, ch.TenantID)
	type bagian struct {
		cpIDs []string
		refs  []string
		qty   int64
	}
	rinci := map[string]*bagian{}
	var kiriman []StokKirim
	dibaca := time.Now().UTC()
	ids := make([]string, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ProductID)
	}
	prods, err := repositories.ProductsByIDs(tctx, nil, ids)
	if err != nil {
		return semua("pending", err.Error())
	}
	for _, r := range rows {
		cps, err := repositories.ChannelProductsForProduct(tctx, nil, ch.ID, r.ProductID)
		if err != nil {
			out[r.ID] = hasilStok{status: "pending", galat: err.Error()}
			continue
		}
		lacak := prods[r.ProductID].TrackStock
		b := &bagian{}
		for _, cp := range cps {
			if cp.ExternalProductID == "" {
				continue
			}
			vid := ""
			if cp.VariantID != nil {
				vid = *cp.VariantID
			}
			stok, err := repositories.CurrentStockQty(tctx, nil, ch.OutletID, cp.ProductID, vid)
			if err != nil {
				out[r.ID] = hasilStok{status: "pending", galat: err.Error()}
				b = nil
				break
			}
			q := qtyKirim(stok, cp)
			k := StokKirim{Qty: q, Lacak: lacak, Tersedia: q > 0}
			if !lacak {
				k.Qty, k.Tersedia = 0, cp.IsAvailable
			}
			b.qty = k.Qty
			b.cpIDs = append(b.cpIDs, cp.ID)
			for _, ref := range strings.Split(cp.ExternalProductID, pemisahRef) {
				if ref = strings.TrimSpace(ref); ref != "" {
					b.refs = append(b.refs, ref)
					k.Ref = ref
					kiriman = append(kiriman, k)
				}
			}
		}
		if b == nil {
			continue
		}
		if len(b.refs) == 0 {
			out[r.ID] = hasilStok{status: "failed", galat: "barang belum dicocokkan dengan listing di " + ad.Info().Name}
			continue
		}
		rinci[r.ID] = b
	}
	if len(kiriman) == 0 {
		return out
	}
	var galat map[string]error
	err = denganKredensialTerkunci(tctx, ch.ID, func(_ *models.Channel, _ ProviderAdapter, cred ChannelCredentials) error {
		pctx, cancel := context.WithTimeout(ctx, time.Minute)
		defer cancel()
		galat = ss.PushStock(pctx, cred, kiriman)
		return nil
	})
	if err != nil {
		for id := range rinci {
			out[id] = hasilStok{status: "pending", galat: err.Error()}
		}
		return out
	}
	var berhasil []string
	for id, b := range rinci {
		var pesan []string
		for _, ref := range b.refs {
			if e := galat[ref]; e != nil {
				pesan = append(pesan, e.Error())
			}
		}
		if len(pesan) > 0 {
			out[id] = hasilStok{status: "pending", galat: strings.Join(pesan, "; "), qty: fmt.Sprint(b.qty)}
			continue
		}
		out[id] = hasilStok{status: "sent", qty: fmt.Sprint(b.qty)}
		berhasil = append(berhasil, b.cpIDs...)
	}
	if err := repositories.MarkChannelProductSynced(tctx, nil, berhasil, dibaca); err != nil {
		helpers.LoggerFromContext(ctx).Warn("gagal mencatat waktu sinkron stok", "error", err)
	}
	return out
}

// ProcessStockSyncs mengantre barang yang stoknya bergerak lalu mengirim
// antrean sinkron stok per kanal. GLOBAL (pekerja). Galat penyedia dicoba
// ulang sampai maksPercobaanStok, lalu ditandai gagal dengan pesan aslinya.
func ProcessStockSyncs(ctx context.Context) (sent, failed int, err error) {
	if _, err := AntrekanSinkronStok(ctx); err != nil {
		return 0, 0, err
	}
	const maxIter = 200
	var dicoba []string // sekali per putaran; percobaan ulang di putaran berikutnya
	for i := 0; i < maxIter; i++ {
		n := 0
		e := repositories.Transaction(ctx, func(tx *gorm.DB) error {
			rows, err := repositories.ClaimPendingStockSyncs(ctx, tx, 50, dicoba...)
			if err != nil {
				return err
			}
			n = len(rows)
			for _, r := range rows {
				dicoba = append(dicoba, r.ID)
			}
			perKanal := map[string][]models.ChannelStockSync{}
			var urut []string
			for _, r := range rows {
				if _, ada := perKanal[r.ChannelID]; !ada {
					urut = append(urut, r.ChannelID)
				}
				perKanal[r.ChannelID] = append(perKanal[r.ChannelID], r)
			}
			for _, chID := range urut {
				hasil := kirimStokKanal(ctx, chID, perKanal[chID])
				for _, r := range perKanal[chID] {
					h := hasil[r.ID]
					coba := r.Attempts + 1
					status := h.status
					if status == "pending" && coba >= maksPercobaanStok {
						status = "failed"
					}
					switch status {
					case "sent":
						sent++
					case "failed":
						failed++
					}
					if err := repositories.SaveStockSyncResult(ctx, tx, r.ID, status, h.galat, coba, h.qty); err != nil {
						return err
					}
				}
			}
			return nil
		})
		if e != nil {
			return sent, failed, e
		}
		if n == 0 {
			break
		}
	}
	return sent, failed, nil
}

// StatusStokKanal: ringkasan untuk dialog sambungan.
func StatusStokKanal(ctx context.Context, channelID string) (structs.ChannelStockStatusResponse, error) {
	var out structs.ChannelStockStatusResponse
	ch, err := repositories.FindChannel(ctx, nil, channelID)
	if err != nil {
		return out, err
	}
	// Didukung = penyedianya menerima stok DAN kanal ini tersambung ke API-nya
	// (kanal bernama "Tokopedia" yang dicatat manual tidak ikut).
	_, bisa := providerAdapters[ch.Provider].(stockPusher)
	out.Supported = bisa && ch.IntegrationMode == "api" && len(ch.CredentialsEncrypted) > 0
	r, err := repositories.ChannelStockSummary(ctx, channelID)
	if err != nil {
		return out, err
	}
	out.Mapped, out.Linked, out.Pending, out.Failed = r.Terpetakan, r.BerRef, r.Menunggu, r.Gagal
	out.LastError, out.LastErrorSKU = r.GalatAkhir, r.SKUGalat
	if r.TerakhirAt != nil {
		out.LastSentAt = r.TerakhirAt.UTC().Format(saleTimeLayout)
	}
	return out, nil
}
