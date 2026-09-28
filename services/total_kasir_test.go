package services

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"candra/backend-api/models"

	"github.com/shopspring/decimal"
)

// Kontrak total kasir lintas bahasa.
//
// Kasir web menghitung total SENDIRI — saat offline tidak ada server yang bisa
// ditanya, dan untuk QRIS/kasbon total itulah yang dibayar pas (server menolak
// selisih serupiah pun). Maka kalkulatornya wajib menghasilkan angka yang sama
// persis dengan priceCheckout.
//
// Kasusnya tinggal di SATU berkas yang diuji dua sisi: tes ini memastikan
// server menghasilkan angka `harapan`, dan web/src/bersama/util/total.test.ts
// memastikan klien menghasilkan angka yang sama. Mengubah rumus di satu sisi
// tanpa sisi lain membuat salah satu tes gagal.
//
// Menambah kasus: tulis kasusnya tanpa `harapan`, lalu jalankan
//
//	PERBARUI_KASUS_TOTAL=1 go test ./services -run TestKontrakTotalKasir
//
// dan periksa angka yang ditulis sebelum di-commit.

const berkasKasusTotal = "../web/src/bersama/util/kasus-total.json"

type kasusTotal struct {
	Nama   string `json:"nama"`
	Aturan struct {
		TaxEnabled        bool   `json:"tax_enabled"`
		TaxRate           string `json:"tax_rate"`
		TaxInclusive      bool   `json:"tax_inclusive"`
		ServiceChargeRate string `json:"service_charge_rate"`
	} `json:"aturan"`
	Baris []struct {
		Harga  int64  `json:"harga"`
		Qty    string `json:"qty"`
		Diskon int64  `json:"diskon"`
	} `json:"baris"`
	DiskonTransaksi int64         `json:"diskon_transaksi"`
	Harapan         *harapanTotal `json:"harapan,omitempty"`
}

type harapanTotal struct {
	Subtotal       int64 `json:"subtotal"`
	DiscountAmount int64 `json:"discount_amount"`
	TaxAmount      int64 `json:"tax_amount"`
	ServiceAmount  int64 `json:"service_amount"`
	Total          int64 `json:"total"`
}

func TestKontrakTotalKasir(t *testing.T) {
	mentah, err := os.ReadFile(berkasKasusTotal)
	if err != nil {
		t.Fatalf("baca %s: %v", berkasKasusTotal, err)
	}
	var kasus []kasusTotal
	if err := json.Unmarshal(mentah, &kasus); err != nil {
		t.Fatalf("urai %s: %v", berkasKasusTotal, err)
	}
	perbarui := os.Getenv("PERBARUI_KASUS_TOTAL") == "1"

	for i := range kasus {
		k := &kasus[i]
		outlet := models.Outlet{
			TaxEnabled:        k.Aturan.TaxEnabled,
			TaxRate:           decimal.RequireFromString(k.Aturan.TaxRate),
			TaxInclusive:      k.Aturan.TaxInclusive,
			ServiceChargeRate: decimal.RequireFromString(k.Aturan.ServiceChargeRate),
		}
		in := CheckoutInput{OrderDiscount: k.DiskonTransaksi}
		produk := map[string]models.Product{}
		for j, b := range k.Baris {
			id := fmt.Sprintf("P%02d", j)
			produk[id] = models.Product{ID: id, SellPrice: b.Harga}
			in.Items = append(in.Items, CheckoutItem{
				ProductID: id, Qty: decimal.RequireFromString(b.Qty), DiscountAmount: b.Diskon,
			})
		}
		_, tot, err := priceCheckout(in, outlet, produk, nil, nil, nil)
		if err != nil {
			t.Fatalf("%s: %v", k.Nama, err)
		}
		dapat := harapanTotal{
			Subtotal: tot.subtotal, DiscountAmount: tot.discountAmount, TaxAmount: tot.taxAmount,
			ServiceAmount: tot.serviceAmount, Total: tot.total,
		}
		if perbarui {
			k.Harapan = &dapat
			continue
		}
		if k.Harapan == nil {
			t.Fatalf("%s: belum punya `harapan` — jalankan dengan PERBARUI_KASUS_TOTAL=1", k.Nama)
		}
		if *k.Harapan != dapat {
			t.Errorf("%s: server menghitung %+v, berkas kasus menulis %+v", k.Nama, dapat, *k.Harapan)
		}
	}

	if perbarui {
		keluaran, err := json.MarshalIndent(kasus, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(berkasKasusTotal, append(keluaran, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("harapan %d kasus ditulis ulang ke %s", len(kasus), berkasKasusTotal)
	}
}
