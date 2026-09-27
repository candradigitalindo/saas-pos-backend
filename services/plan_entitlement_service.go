package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"candra/backend-api/config"
	"candra/backend-api/helpers"
	"candra/backend-api/models"
	"candra/backend-api/repositories"

	"gorm.io/gorm"
)

// ── Kunci paket langganan ──────────────────────────────────────────────────
//
// Katalog paket (tabel plans) menyimpan `features` dan batas pemakaian, dan
// halaman Langganan menjanjikan perbedaan antarpaket — tetapi sampai modul ini
// ada, TIDAK ADA satu pun yang menegakkannya: tenant tanpa langganan bisa
// memakai semuanya. Modul ini menentukan PAKET YANG BERLAKU bagi tenant dan
// menjaga fitur & kuotanya.
//
// Prinsip yang dipegang:
//
//  1. Penjualan TIDAK PERNAH diblokir karena paket. Transaksi offline yang
//     disinkron (/sync/push) sudah terjadi di dunia nyata — uangnya sudah
//     diterima; menolaknya hanya menghilangkan catatan. Batas transaksi per
//     bulan karena itu juga tidak ditegakkan di sini.
//  2. Data tidak disandera. Turun paket menutup TINDAKAN baru (membuat,
//     mengubah); data lama tetap bisa DIBACA.
//  3. Yang sudah ada tetap jalan. Tenant yang turun ke paket satu cabang
//     tetap memakai cabang-cabangnya; ia hanya tidak bisa menambah lagi.

// Kode fitur di plans.features.
const (
	FeatureQRIS          = "qris"
	FeatureOnlineChannel = "online_channel"
	FeatureCRMFreelance  = "crm_freelance"
	FeatureMultiOutlet   = "multi_outlet"
)

// namaFitur: nama fitur untuk pesan galat, dalam bahasa pemilik warung.
var namaFitur = map[string]string{
	FeatureQRIS:          "Pembayaran QRIS",
	FeatureOnlineChannel: "Kanal jualan online",
	FeatureCRMFreelance:  "CRM (penawaran, proyek & invoice)",
	FeatureMultiOutlet:   "Menambah cabang",
}

// planGratis adalah kode paket yang berlaku bila tenant tidak punya langganan
// yang masih berlaku.
const planGratis = "free"

// subscriptionGraceDays: hari tenggang setelah masa langganan BERBAYAR habis
// sebelum turun ke paket Gratis — ruang untuk membayar tagihan perpanjangan
// tanpa kasir mendadak kehilangan QRIS di tengah hari. Masa coba (trial) tidak
// diberi tenggang.
func subscriptionGraceDays() int { return config.GetIntEnv("SUBSCRIPTION_GRACE_DAYS", 7) }

// Entitlement adalah paket yang sedang BERLAKU bagi tenant beserta hak-haknya.
type Entitlement struct {
	PlanCode string
	PlanName string
	// Status: status langganan ("trial", "active", "past_due") bila
	// langganannya berlaku; "none" bila tenant memakai paket Gratis karena
	// belum/tidak lagi berlangganan.
	Status string
	// BerlakuSampai: batas akhir hak paket ini (termasuk tenggang); nil untuk
	// paket Gratis yang tidak berakhir.
	BerlakuSampai *time.Time
	Features      map[string]bool
	MaxOutlets    *int
	MaxUsers      *int
	MaxProducts   *int
}

// Has melaporkan apakah fitur `kode` termasuk paket yang berlaku.
func (e Entitlement) Has(kode string) bool { return e.Features[kode] }

// OutletLimit: batas jumlah cabang. Tanpa fitur multi_outlet, satu cabang;
// dengan fitur itu, max_outlets paket (nil = tanpa batas).
func (e Entitlement) OutletLimit() *int {
	if !e.Has(FeatureMultiOutlet) {
		satu := 1
		if e.MaxOutlets != nil && *e.MaxOutlets < 1 {
			return e.MaxOutlets
		}
		return &satu
	}
	return e.MaxOutlets
}

// entitlementFromPlan membangun Entitlement dari satu baris paket.
func entitlementFromPlan(p models.Plan) Entitlement {
	fitur := map[string]bool{}
	if len(p.Features) > 0 {
		// Nilai non-boolean diabaikan (dianggap tidak aktif), bukan galat:
		// katalog diubah admin platform dan satu salah ketik tidak boleh
		// menjatuhkan seluruh pemeriksaan.
		var mentah map[string]any
		if err := json.Unmarshal(p.Features, &mentah); err == nil {
			for k, v := range mentah {
				b, _ := v.(bool)
				fitur[k] = b
			}
		}
	}
	return Entitlement{
		PlanCode:    p.Code,
		PlanName:    p.Name,
		Features:    fitur,
		MaxOutlets:  p.MaxOutlets,
		MaxUsers:    p.MaxUsers,
		MaxProducts: p.MaxProducts,
	}
}

// batasBerlaku mengembalikan saat terakhir langganan `s` masih memberi hak
// paketnya, atau nil bila statusnya sudah tidak memberi hak sama sekali.
//
// Dihitung dari WAKTU, bukan hanya dari kolom status: belum ada pekerjaan
// terjadwal yang memindahkan trial → expired atau active → past_due, jadi
// status "trial" yang trial_ends_at-nya sudah lewat harus diperlakukan habis.
func batasBerlaku(s models.Subscription) *time.Time {
	switch s.Status {
	case "trial":
		akhir := s.CurrentPeriodEnd
		if s.TrialEndsAt != nil {
			akhir = *s.TrialEndsAt
		}
		return &akhir
	case "active", "past_due":
		akhir := s.CurrentPeriodEnd.AddDate(0, 0, subscriptionGraceDays())
		return &akhir
	default: // canceled, expired
		return nil
	}
}

// CurrentEntitlement menentukan paket yang berlaku bagi tenant konteks,
// sekarang. Langganan yang masih berlaku → paketnya; selain itu → Gratis.
func CurrentEntitlement(ctx context.Context) (Entitlement, error) {
	return entitlementAt(ctx, nil, time.Now().UTC())
}

func entitlementAt(ctx context.Context, tx *gorm.DB, now time.Time) (Entitlement, error) {
	sub, err := repositories.FindSubscriptionByTenant(ctx, tx)
	switch {
	case err == nil:
		if akhir := batasBerlaku(sub); akhir != nil && now.Before(*akhir) && sub.Plan != nil {
			e := entitlementFromPlan(*sub.Plan)
			e.Status = sub.Status
			e.BerlakuSampai = akhir
			return e, nil
		}
	case !errors.Is(err, repositories.ErrSubscriptionNotFound):
		return Entitlement{}, err
	}

	gratis, err := repositories.FindPlanByCode(ctx, tx, planGratis)
	if err != nil {
		if errors.Is(err, helpers.ErrValidation) {
			// Katalog tanpa paket Gratis: tidak ada fitur berbayar sama sekali.
			return Entitlement{PlanCode: planGratis, PlanName: "Gratis", Status: "none", Features: map[string]bool{}}, nil
		}
		return Entitlement{}, err
	}
	e := entitlementFromPlan(gratis)
	e.Status = "none"
	return e, nil
}

// RequireFeature menolak (ErrPlanRequired → 402) bila fitur `kode` tidak
// termasuk paket yang berlaku. Pesannya menyebut paket TERMURAH yang punya
// fitur itu, supaya pemilik tahu persis apa yang harus dipilih.
func RequireFeature(ctx context.Context, kode string) error {
	e, err := CurrentEntitlement(ctx)
	if err != nil {
		return err
	}
	if e.Has(kode) {
		return nil
	}
	return fmt.Errorf("%w: %s", helpers.ErrPlanRequired, pesanButuhFitur(ctx, kode))
}

// pesanButuhFitur: "Pembayaran QRIS tersedia mulai paket Basic. Pilih paket
// di menu Langganan."
func pesanButuhFitur(ctx context.Context, kode string) string {
	nama := namaFitur[kode]
	if nama == "" {
		nama = "Fitur ini"
	}
	if paket := paketTermurahDengan(ctx, kode); paket != "" {
		return fmt.Sprintf("%s tersedia mulai paket %s. Pilih paket di menu Langganan.", nama, paket)
	}
	return nama + " tidak termasuk paket Anda. Lihat pilihan paket di menu Langganan."
}

// paketTermurahDengan mengembalikan nama paket aktif termurah yang memuat
// fitur `kode`, atau "" bila tidak ada / katalog gagal dibaca (pesan galat
// tetap dikirim tanpa nama paket — tidak ada gunanya menggagalkan penolakan).
func paketTermurahDengan(ctx context.Context, kode string) string {
	paket, err := repositories.ListActivePlans(ctx)
	if err != nil {
		return ""
	}
	for _, p := range paket { // sudah urut termurah dulu
		if entitlementFromPlan(p).Has(kode) {
			return p.Name
		}
	}
	return ""
}

// UpgradeFor memetakan setiap fitur katalog yang tidak dimiliki `e` ke nama
// paket aktif termurah yang memuatnya. Galat membaca katalog → peta kosong
// (klien lalu menulis ajakan umum tanpa nama paket).
func UpgradeFor(ctx context.Context, e Entitlement) map[string]string {
	out := map[string]string{}
	paket, err := repositories.ListActivePlans(ctx)
	if err != nil {
		return out
	}
	for _, p := range paket { // termurah dulu: yang pertama ditemukan menang
		for k, ada := range entitlementFromPlan(p).Features {
			if _, sudah := out[k]; ada && !sudah && !e.Has(k) {
				out[k] = p.Name
			}
		}
	}
	return out
}

// Jenis kuota yang ditegakkan saat membuat data.
const (
	KuotaCabang   = "outlets"
	KuotaPengguna = "users"
	KuotaBarang   = "products"
)

// EnsureQuota memastikan tenant masih boleh menambah `tambah` baris `jenis`
// (cabang, pengguna, barang). WAJIB dipanggil di dalam transaksi `tx` yang
// juga membuat datanya: tenant dikunci dulu (LockTenantRow) supaya dua
// permintaan bersamaan tidak sama-sama lolos dari batas yang sama.
//
// Batas nil = tanpa batas; pemeriksaan selesai tanpa menghitung apa pun.
func EnsureQuota(ctx context.Context, tx *gorm.DB, jenis string, tambah int) error {
	e, err := entitlementAt(ctx, tx, time.Now().UTC())
	if err != nil {
		return err
	}

	var batas *int
	var hitung func(context.Context, *gorm.DB) (int64, error)
	var label string
	switch jenis {
	case KuotaCabang:
		batas, hitung, label = e.OutletLimit(), repositories.CountOutlets, "cabang"
	case KuotaPengguna:
		batas, hitung, label = e.MaxUsers, repositories.CountUsers, "pengguna"
	case KuotaBarang:
		batas, hitung, label = e.MaxProducts, repositories.CountProducts, "barang"
	default:
		return fmt.Errorf("kuota tak dikenal: %s", jenis)
	}
	if batas == nil {
		return nil
	}

	if err := repositories.LockTenantRow(ctx, tx); err != nil {
		return err
	}
	ada, err := hitung(ctx, tx)
	if err != nil {
		return err
	}
	if ada+int64(tambah) <= int64(*batas) {
		return nil
	}

	// Menambah cabang tanpa fitur multi_outlet: sebut paket yang membukanya.
	if jenis == KuotaCabang && !e.Has(FeatureMultiOutlet) {
		return fmt.Errorf("%w: %s", helpers.ErrPlanRequired, pesanButuhFitur(ctx, FeatureMultiOutlet))
	}
	return fmt.Errorf("%w: paket %s dibatasi %d %s (sekarang %d). Naikkan paket di menu Langganan untuk menambah lagi.",
		helpers.ErrPlanRequired, e.PlanName, *batas, label, ada)
}
