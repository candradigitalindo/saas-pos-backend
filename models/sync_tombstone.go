package models

import "time"

// SyncTombstone adalah jejak satu baris yang DIHAPUS KERAS dari tabel yang
// disinkronkan. Ditulis pemicu `record_sync_tombstone` (migrasi 000014, §10)
// AFTER DELETE. Pull mengembalikan tombstone ber-`sync_version` di atas kursor
// klien supaya klien ikut menghapus salinan lokalnya.
//
// Baris SOFT-delete tidak menghasilkan tombstone di sini — mengisi `deleted_at`
// adalah UPDATE, jadi `sync_version` barisnya ikut naik dan pull tetap
// membawanya (klien mengenalinya dari `deleted_at` yang terisi).
type SyncTombstone struct {
	ID          int64     `json:"id" gorm:"primaryKey"`
	TenantID    string    `json:"tenant_id" gorm:"type:char(26);not null"`
	TargetTable string    `json:"table_name" gorm:"column:table_name;not null"` // nama tabel asal baris
	RowID       string    `json:"row_id" gorm:"type:char(26);not null"`
	SyncVersion int64     `json:"sync_version" gorm:"not null"`
	DeletedAt   time.Time `json:"deleted_at"`
}

// TableName memaksa nama tabel PostgreSQL — GORM tidak diberi kesempatan menebak.
func (SyncTombstone) TableName() string { return "sync_tombstones" }
