// Command seed-demo mengisi database dengan data contoh yang masuk akal untuk
// sebuah warung: barang, stok, transaksi hari ini, kasbon, karyawan, dan
// prospek.
//
// Gunanya untuk menjelajahi aplikasi dan menyiapkan peragaan — BUKAN untuk
// production.
//
// Sengaja lewat HTTP API, bukan langsung ke repositori, supaya datanya menempuh
// jalur yang sama dengan pemakaian sungguhan: validasi, izin, kunci
// idempotensi, dan yang terpenting `business_date` yang dihitung server dari
// zona waktu outlet. Data contoh yang dibuat lewat jalan pintas gampang
// menghasilkan baris yang mustahil muncul dari aplikasi.
//
// Transaksinya dicatat SEKARANG, jadi selalu jatuh pada hari usaha berjalan —
// jalankan ulang kapan saja untuk mendapat data "hari ini".
//
// Pemakaian:
//
//	go run . &                      # server harus hidup
//	go run ./cmd/seed-demo
//	go run ./cmd/seed-demo -base http://localhost:8080 -sandi rahasia123
//
// Aman dijalankan berulang: usaha, barang, dan pengguna yang sudah ada dipakai
// lagi, bukan digandakan. Yang selalu ditambah hanyalah transaksi hari ini.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"candra/backend-api/internal/ulid"
)

type opsi struct {
	base     string
	pengguna string
	sandi    string
}

func main() {
	var o opsi
	flag.StringVar(&o.base, "base", "http://localhost:8080", "alamat server")
	flag.StringVar(&o.pengguna, "pengguna", "sari", "nama pengguna pemilik")
	flag.StringVar(&o.sandi, "sandi", "rahasia123", "kata sandi pemilik")
	flag.Parse()

	k := &klien{base: strings.TrimRight(o.base, "/")}

	if err := k.pastikanServerHidup(); err != nil {
		gagal("server tidak menjawab di %s — jalankan `go run .` dulu\n  %v", o.base, err)
	}

	if err := k.masukAtauDaftar(o); err != nil {
		gagal("masuk/daftar: %v", err)
	}
	fmt.Printf("usaha      : %s (outlet %s)\n", k.namaUsaha, potong(k.outletID))

	satuan, err := k.pastikanSatuan()
	if err != nil {
		gagal("satuan: %v", err)
	}
	kategori, err := k.pastikanKategori()
	if err != nil {
		gagal("kategori: %v", err)
	}
	produk, err := k.pastikanBarang(satuan, kategori)
	if err != nil {
		gagal("barang: %v", err)
	}
	fmt.Printf("barang     : %d siap\n", len(produk))

	pelanggan, err := k.pastikanPelanggan()
	if err != nil {
		gagal("pelanggan: %v", err)
	}

	shift, err := k.pastikanShift()
	if err != nil {
		gagal("shift: %v", err)
	}

	n, omzet, err := k.catatPenjualanHariIni(shift, produk, pelanggan)
	if err != nil {
		gagal("penjualan: %v", err)
	}
	fmt.Printf("transaksi  : %d hari ini, Rp %s\n", n, rupiah(omzet))

	if err := k.catatKas(shift); err != nil {
		gagal("gerakan kas: %v", err)
	}
	if err := k.catatBarangMasuk(produk); err != nil {
		gagal("barang masuk: %v", err)
	}
	if err := k.pastikanKaryawan(); err != nil {
		gagal("karyawan: %v", err)
	}
	if err := k.pastikanStaf(); err != nil {
		gagal("pengguna staf: %v", err)
	}

	fmt.Println()
	fmt.Println("selesai. Masuk sebagai:")
	fmt.Printf("  %-6s / %s   (pemilik, semua izin)\n", o.pengguna, o.sandi)
	fmt.Printf("  %-6s / %s   (kasir + gudang, peran ganda)\n", "budi", o.sandi)
	fmt.Printf("  %-6s / %s   (gudang saja)\n", "ahmad", o.sandi)
}

// ── Klien HTTP ─────────────────────────────────────────────────────────────

type klien struct {
	base      string
	token     string
	outletID  string
	namaUsaha string
}

func (k *klien) pastikanServerHidup() error {
	res, err := http.Get(k.base + "/health")
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("health menjawab %d", res.StatusCode)
	}
	return nil
}

// minta mengirim satu permintaan dan membuka amplop respons.
func (k *klien) minta(metode, jalur string, badan any, hasil any, extra map[string]string) error {
	var r io.Reader
	if badan != nil {
		b, err := json.Marshal(badan)
		if err != nil {
			return err
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequest(metode, k.base+"/api/v1"+jalur, r)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if k.token != "" {
		req.Header.Set("Authorization", "Bearer "+k.token)
	}
	for kk, vv := range extra {
		req.Header.Set(kk, vv)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	isi, _ := io.ReadAll(res.Body)

	if res.StatusCode >= 400 {
		var e struct {
			Message string            `json:"message"`
			Errors  map[string]string `json:"errors"`
		}
		_ = json.Unmarshal(isi, &e)
		return &galatAPI{status: res.StatusCode, pesan: e.Message, kolom: e.Errors}
	}
	if hasil == nil {
		return nil
	}
	var amplop struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(isi, &amplop); err != nil {
		return err
	}
	return json.Unmarshal(amplop.Data, hasil)
}

type galatAPI struct {
	status int
	pesan  string
	kolom  map[string]string
}

func (e *galatAPI) Error() string {
	if len(e.kolom) > 0 {
		return fmt.Sprintf("HTTP %d: %s %v", e.status, e.pesan, e.kolom)
	}
	return fmt.Sprintf("HTTP %d: %s", e.status, e.pesan)
}

func statusnya(err error) int {
	if e, ok := err.(*galatAPI); ok {
		return e.status
	}
	return 0
}

// ── Langkah-langkah ────────────────────────────────────────────────────────

func (k *klien) masukAtauDaftar(o opsi) error {
	var auth struct {
		AccessToken string `json:"access_token"`
	}
	err := k.minta("POST", "/auth/login",
		map[string]any{"username": o.pengguna, "password": o.sandi}, &auth, nil)

	if err != nil {
		if statusnya(err) != http.StatusUnauthorized {
			return err
		}
		// Belum ada — daftarkan usahanya.
		var reg struct {
			Auth struct {
				AccessToken string `json:"access_token"`
			} `json:"auth"`
		}
		if err := k.minta("POST", "/auth/register", map[string]any{
			"business_name": "Warung Bu Sari", "business_type": "retail",
			"phone": "081234567890", "outlet_name": "Warung Bu Sari",
			"owner": map[string]any{
				"name": "Sari Dewi", "username": o.pengguna,
				"email": o.pengguna + "@contoh.id", "password": o.sandi,
			},
		}, &reg, nil); err != nil {
			return err
		}
		auth.AccessToken = reg.Auth.AccessToken
	}
	k.token = auth.AccessToken

	var me struct {
		Tenant struct {
			BusinessName string `json:"business_name"`
		} `json:"tenant"`
		OutletIDs []string `json:"outlet_ids"`
	}
	if err := k.minta("GET", "/me", nil, &me, nil); err != nil {
		return err
	}
	if len(me.OutletIDs) == 0 {
		return fmt.Errorf("pemilik tidak punya outlet")
	}
	k.namaUsaha, k.outletID = me.Tenant.BusinessName, me.OutletIDs[0]
	return nil
}

type baris struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// daftarNama mengambil koleksi dan memetakan nama → id.
func (k *klien) daftarNama(jalur string) (map[string]string, error) {
	var hal struct {
		Data []baris `json:"data"`
	}
	if err := k.minta("GET", jalur+"?limit=100", nil, &hal, nil); err != nil {
		return nil, err
	}
	m := make(map[string]string, len(hal.Data))
	for _, b := range hal.Data {
		m[b.Name] = b.ID
	}
	return m, nil
}

// pastikanAda membuat baris bila namanya belum ada, lalu mengembalikan id-nya.
func (k *klien) pastikanAda(jalur string, ada map[string]string, nama string, badan any) (string, error) {
	if id, ok := ada[nama]; ok {
		return id, nil
	}
	var b baris
	if err := k.minta("POST", jalur, badan, &b, nil); err != nil {
		return "", err
	}
	ada[nama] = b.ID
	return b.ID, nil
}

func (k *klien) pastikanSatuan() (map[string]string, error) {
	ada, err := k.daftarNama("/units")
	if err != nil {
		return nil, err
	}
	for _, n := range []string{"pcs", "botol", "bungkus", "kg"} {
		if _, err := k.pastikanAda("/units", ada, n, map[string]any{"name": n}); err != nil {
			return nil, err
		}
	}
	return ada, nil
}

func (k *klien) pastikanKategori() (map[string]string, error) {
	ada, err := k.daftarNama("/categories")
	if err != nil {
		return nil, err
	}
	for _, n := range []string{"Minuman", "Makanan", "Rokok", "Sembako"} {
		if _, err := k.pastikanAda("/categories", ada, n, map[string]any{"name": n}); err != nil {
			return nil, err
		}
	}
	return ada, nil
}

type barang struct {
	nama, kategori    string
	jual, beli        int64
	stokAwal, minimal string
	barcode           string
}

// katalog sengaja memakai barang warung sungguhan beserta harga yang masuk
// akal — angka acak membuat layar laporan terlihat tidak meyakinkan.
var katalog = []barang{
	{"Kopi Susu Gula Aren", "Minuman", 18000, 12000, "44", "5", "8991002101234"},
	{"Teh Manis Dingin", "Minuman", 8000, 4000, "12", "5", ""},
	{"Air Mineral 600ml", "Minuman", 4000, 2200, "6", "12", "8886008101053"},
	{"Kopi Hitam", "Minuman", 10000, 5000, "28", "5", ""},
	{"Nasi Goreng Spesial", "Makanan", 22000, 13000, "25", "3", ""},
	{"Roti Bakar Coklat", "Makanan", 12000, 7000, "9", "5", ""},
	{"Indomie Goreng", "Makanan", 6000, 3200, "60", "10", "8998866200035"},
	{"Rokok Sampoerna", "Rokok", 32000, 29000, "14", "10", ""},
	{"Gula Pasir 1kg", "Sembako", 16000, 13500, "15", "4", ""},
	{"Minyak Goreng 1L", "Sembako", 18000, 15500, "8", "4", ""},
}

func (k *klien) pastikanBarang(satuan, kategori map[string]string) (map[string]string, error) {
	ada, err := k.daftarNama("/products")
	if err != nil {
		return nil, err
	}
	for _, b := range katalog {
		baru := false
		if _, sudah := ada[b.nama]; !sudah {
			baru = true
		}
		id, err := k.pastikanAda("/products", ada, b.nama, map[string]any{
			"name": b.nama, "unit_id": satuan["pcs"], "category_id": kategori[b.kategori],
			"sell_price": b.jual, "cost_price": b.beli,
			"track_stock": true, "min_stock": b.minimal,
			"barcode": nilBila(b.barcode),
		})
		if err != nil {
			return nil, err
		}
		// Stok awal hanya untuk barang yang memang baru dibuat — menimpanya
		// tiap kali dijalankan akan menghapus jejak penjualan sebelumnya.
		if baru {
			if err := k.minta("POST", "/stock-adjustments", map[string]any{
				"outlet_id": k.outletID, "product_id": id,
				"new_qty": b.stokAwal, "reason": "Stok awal",
			}, nil, nil); err != nil {
				return nil, err
			}
		}
	}
	return ada, nil
}

func (k *klien) pastikanPelanggan() (map[string]string, error) {
	ada, err := k.daftarNama("/customers")
	if err != nil {
		return nil, err
	}
	for _, p := range []struct {
		nama, hp string
		batas    int64
	}{
		{"Bu Rina", "08111000111", 500000},
		{"Pak Andi", "08222000222", 300000},
		{"Warung Sebelah", "08333000333", 1000000},
		{"Ibu Kos Melati", "08444000444", 0},
	} {
		if _, err := k.pastikanAda("/customers", ada, p.nama, map[string]any{
			"name": p.nama, "phone": p.hp, "credit_limit": p.batas,
		}); err != nil {
			return nil, err
		}
	}
	return ada, nil
}

func (k *klien) pastikanShift() (string, error) {
	var hal struct {
		Data []struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"data"`
	}
	if err := k.minta("GET", "/shifts?outlet_id="+k.outletID+"&limit=20", nil, &hal, nil); err != nil {
		return "", err
	}
	for _, s := range hal.Data {
		if s.Status == "open" {
			return s.ID, nil
		}
	}
	var sh struct {
		ID string `json:"id"`
	}
	err := k.minta("POST", "/shifts/open", map[string]any{
		"outlet_id": k.outletID, "opening_cash": 200000,
	}, &sh, nil)
	return sh.ID, err
}

// keranjang: daftar (nama barang, jumlah).
type isi struct {
	nama string
	qty  int
}

var keranjangHariIni = [][]isi{
	{{"Kopi Susu Gula Aren", 2}, {"Teh Manis Dingin", 1}},
	{{"Nasi Goreng Spesial", 1}, {"Air Mineral 600ml", 2}},
	{{"Indomie Goreng", 3}, {"Kopi Hitam", 1}},
	{{"Rokok Sampoerna", 1}},
	{{"Gula Pasir 1kg", 2}, {"Minyak Goreng 1L", 1}},
	{{"Kopi Susu Gula Aren", 1}},
	{{"Nasi Goreng Spesial", 2}, {"Teh Manis Dingin", 2}},
	{{"Kopi Hitam", 2}, {"Roti Bakar Coklat", 1}},
}

func (k *klien) catatPenjualanHariIni(shift string, produk, pelanggan map[string]string) (int, int64, error) {
	harga := map[string]int64{}
	for _, b := range katalog {
		harga[b.nama] = b.jual
	}

	var jumlah int
	var omzet int64

	for i, kj := range keranjangHariIni {
		var items []map[string]any
		var total int64
		for _, it := range kj {
			id, ok := produk[it.nama]
			if !ok {
				continue
			}
			items = append(items, map[string]any{"product_id": id, "qty": fmt.Sprint(it.qty)})
			total += harga[it.nama] * int64(it.qty)
		}
		if len(items) == 0 {
			continue
		}

		// Sebagian QRIS, sisanya tunai dengan uang dibulatkan ke atas supaya
		// ada kembalian — laporan jadi menyerupai hari jualan sungguhan.
		metode, dibayar := "cash", ((total/5000)+1)*5000
		if i%3 == 1 {
			metode, dibayar = "qris", total
		}

		var s struct {
			Total int64 `json:"total"`
		}
		err := k.minta("POST", "/sales", map[string]any{
			"outlet_id": k.outletID, "shift_id": shift,
			"items":    items,
			"payments": []map[string]any{{"method": metode, "amount": dibayar}},
		}, &s, map[string]string{"Idempotency-Key": ulid.New()})
		if err != nil {
			// Stok habis itu wajar bila skrip dijalankan berkali-kali —
			// lewati, jangan gagalkan seluruh peragaan.
			if statusnya(err) == http.StatusUnprocessableEntity {
				continue
			}
			return jumlah, omzet, err
		}
		jumlah++
		omzet += s.Total
	}

	// Satu penjualan kasbon supaya layar Kasbon ada isinya.
	if id, ok := produk["Indomie Goreng"]; ok && pelanggan["Bu Rina"] != "" {
		err := k.minta("POST", "/sales", map[string]any{
			"outlet_id": k.outletID, "shift_id": shift,
			"customer_id": pelanggan["Bu Rina"],
			"items":       []map[string]any{{"product_id": id, "qty": "5"}},
			"payments":    []map[string]any{{"method": "credit", "amount": 30000}},
		}, nil, map[string]string{"Idempotency-Key": ulid.New()})
		if err != nil && statusnya(err) != http.StatusUnprocessableEntity {
			return jumlah, omzet, err
		}
		if err == nil {
			jumlah++
		}
	}

	return jumlah, omzet, nil
}

func (k *klien) catatKas(shift string) error {
	for _, g := range []struct {
		arah, alasan string
		jumlah       int64
	}{
		{"out", "Beli plastik kresek & tisu", 75000},
		{"in", "Tambahan modal dari pemilik", 50000},
	} {
		if err := k.minta("POST", "/cash-movements", map[string]any{
			"outlet_id": k.outletID, "shift_id": shift,
			"direction": g.arah, "amount": g.jumlah, "reason": g.alasan,
		}, nil, nil); err != nil {
			return err
		}
	}
	return nil
}

func (k *klien) catatBarangMasuk(produk map[string]string) error {
	pemasok, err := k.daftarNama("/suppliers")
	if err != nil {
		return err
	}
	id, err := k.pastikanAda("/suppliers", pemasok, "Toko Grosir Jaya", map[string]any{
		"name": "Toko Grosir Jaya", "phone": "0812999888",
	})
	if err != nil {
		return err
	}
	pid, ok := produk["Kopi Susu Gula Aren"]
	if !ok {
		return nil
	}
	return k.minta("POST", "/purchases", map[string]any{
		"outlet_id": k.outletID, "supplier_id": id,
		"items": []map[string]any{{"product_id": pid, "qty": "24", "unit_cost": 12000}},
	}, nil, map[string]string{"Idempotency-Key": ulid.New()})
}

func (k *klien) pastikanKaryawan() error {
	var hal struct {
		Data []struct {
			FullName string `json:"full_name"`
		} `json:"data"`
	}
	if err := k.minta("GET", "/employees?limit=100", nil, &hal, nil); err != nil {
		return err
	}
	ada := map[string]bool{}
	for _, e := range hal.Data {
		ada[e.FullName] = true
	}
	for _, e := range []struct {
		nama, jabatan string
		upah          int64
	}{
		{"Budi Santoso", "Kasir", 3200000},
		{"Ahmad Fauzi", "Gudang", 2800000},
		{"Siti Aminah", "Kasir", 3000000},
	} {
		if ada[e.nama] {
			continue
		}
		if err := k.minta("POST", "/employees", map[string]any{
			"outlet_id": k.outletID, "full_name": e.nama, "position": e.jabatan,
			"wage_type": "monthly", "base_wage": e.upah,
			"joined_at": time.Now().AddDate(0, -8, 0).Format("2006-01-02"),
		}, nil, nil); err != nil {
			return err
		}
	}
	return nil
}

// pastikanStaf membuat dua pengguna: satu dengan PERAN GANDA (kasir + gudang)
// supaya izin gabungan bisa dicoba langsung di layar.
func (k *klien) pastikanStaf() error {
	peran, err := k.daftarNama("/roles")
	if err != nil {
		return err
	}
	var hal struct {
		Data []struct {
			Username string `json:"username"`
		} `json:"data"`
	}
	if err := k.minta("GET", "/users?limit=100", nil, &hal, nil); err != nil {
		return err
	}
	ada := map[string]bool{}
	for _, u := range hal.Data {
		ada[u.Username] = true
	}

	buat := func(nama, user string, utama string, tambahan []string) error {
		if ada[user] {
			return nil
		}
		badan := map[string]any{
			"name": nama, "username": user, "email": user + "@contoh.id",
			"password": "rahasia123", "role_id": peran[utama],
		}
		if len(tambahan) > 0 {
			ids := make([]string, 0, len(tambahan))
			for _, t := range tambahan {
				if id := peran[t]; id != "" {
					ids = append(ids, id)
				}
			}
			badan["role_ids"] = ids
		}
		return k.minta("POST", "/users", badan, nil, nil)
	}

	if err := buat("Budi Santoso", "budi", "Kasir", []string{"Gudang"}); err != nil {
		return err
	}
	return buat("Ahmad Fauzi", "ahmad", "Gudang", nil)
}

// ── Bantuan kecil ──────────────────────────────────────────────────────────

func nilBila(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func potong(id string) string {
	if len(id) > 8 {
		return id[:8] + "…"
	}
	return id
}

// rupiah memberi pemisah ribuan titik, seperti tampilan di layar.
func rupiah(n int64) string {
	s := fmt.Sprint(n)
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(c)
	}
	return b.String()
}

func gagal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "seed-demo: "+format+"\n", args...)
	os.Exit(1)
}
