package models

import (
	"encoding/json"
	"time"
)

// OpenBill adalah tagihan terbuka (open bill / tahan transaksi): pesanan yang
// belum dibayar, mis. "Meja 5". Isinya dokumen utuh (Items, JSON) yang diganti
// seluruhnya setiap kali disimpan dari keranjang kasir; harga TIDAK disimpan —
// dihitung server saat checkout. Lihat migrasi 000044.
type OpenBill struct {
	ID            string          `json:"id" gorm:"primaryKey;type:char(26)"`
	TenantID      string          `json:"tenant_id" gorm:"type:char(26);not null"`
	OutletID      string          `json:"outlet_id" gorm:"type:char(26);not null"`
	Label         string          `json:"label" gorm:"not null"`
	CustomerID    *string         `json:"customer_id" gorm:"type:char(26)"`
	OrderType     string          `json:"order_type" gorm:"not null;default:dine_in"`
	Items         json.RawMessage `json:"items" gorm:"type:jsonb;not null"`
	OrderDiscount json.RawMessage `json:"order_discount" gorm:"type:jsonb"`
	Note          string          `json:"note" gorm:"not null;default:''"`
	Status        string          `json:"status" gorm:"not null;default:open"` // open | paid | canceled
	// Version naik setiap perubahan; penyimpan wajib menyebut versi yang ia
	// baca (base_version) supaya perubahan perangkat lain tidak tertimpa diam-diam.
	Version int `json:"version" gorm:"not null;default:1"`
	// LastOpID: id operasi antrean offline terakhir yang diterapkan — operasi
	// yang sama dikirim ulang dikenali sebagai duplikat.
	LastOpID  *string    `json:"-" gorm:"type:char(26)"`
	SaleID    *string    `json:"sale_id" gorm:"type:char(26)"`
	CreatedBy string     `json:"created_by" gorm:"type:char(26);not null"`
	UpdatedBy string     `json:"updated_by" gorm:"type:char(26);not null"`
	ClosedBy  *string    `json:"closed_by" gorm:"type:char(26)"`
	ClosedAt  *time.Time `json:"closed_at"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// OpenBillItem adalah satu baris tagihan (bentuk isi kolom items).
type OpenBillItem struct {
	ProductID      string `json:"product_id"`
	VariantID      string `json:"variant_id,omitempty"`
	ProductUnitID  string `json:"product_unit_id,omitempty"` // kemasan (qty dalam kemasan)
	Qty            string `json:"qty"`                       // desimal string, sama dengan checkout
	DiscountAmount int64  `json:"discount_amount,omitempty"`
	// DiscountPercent: diskon persen (1–100) — disimpan sebagai persen supaya
	// tetap persen saat qty diubah di perangkat lain. Bila diisi, DiscountAmount diabaikan.
	DiscountPercent *int   `json:"discount_percent,omitempty"`
	Note            string `json:"note,omitempty"`
}

// OpenBillDiscount adalah diskon transaksi pada tagihan: nominal rupiah, atau
// persen dari belanja setelah diskon barang.
type OpenBillDiscount struct {
	Kind  string `json:"kind"` // nominal | percent
	Value int64  `json:"value"`
}
