package timez

// Menyertakan salinan database zona waktu IANA ke dalam binary.
//
// Kenapa wajib: seluruh model waktu produk ini (docs/TECHNICAL-BACKEND.md §3.2)
// bergantung pada time.LoadLocation("Asia/Jayapura") dan kawan-kawannya selalu
// berhasil. Di image container minimal (distroless, scratch, alpine tanpa
// tzdata) file zoneinfo sistem tidak ada, sehingga LoadLocation gagal dan
// perhitungan business_date runtuh diam-diam. Impor blank ini membuat Go memakai
// tzdata bawaannya sebagai fallback — menambah ~450 KB ke binary, harga yang
// murah untuk laporan harian yang benar di WITA/WIT.
import _ "time/tzdata"
