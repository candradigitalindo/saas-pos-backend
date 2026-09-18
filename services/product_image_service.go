package services

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"candra/backend-api/config"
	"candra/backend-api/helpers"
	"candra/backend-api/internal/ulid"
	"candra/backend-api/models"
	"candra/backend-api/repositories"
)

// Penyimpanan foto barang.
//
// Berkasnya disimpan di disk server, bukan di basis data. Gambar di dalam
// basis data membuat setiap cadangan ikut membengkak dan setiap query yang
// tidak sengaja memilih kolomnya menarik megabita percuma.
//
// Nama berkas SELALU dibuat server dari ULID, tidak pernah dari nama kiriman
// klien. Nama dari klien adalah masukan yang tidak dipercaya: "../../.env"
// atau "foto.php" sama-sama bisa dikirim, dan keduanya berbahaya dengan cara
// yang berbeda. ULID sekaligus membuat alamat fotonya tidak bisa ditebak —
// berkasnya disajikan tanpa autentikasi, jadi ketidakterdugaan itulah
// pagarnya.

// jenisGambarDiizinkan memetakan tipe yang DITERIMA ke ekstensi berkasnya.
var jenisGambarDiizinkan = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
}

// FolderUnggah mengembalikan direktori penyimpanan berkas unggahan.
func FolderUnggah() string {
	return config.GetEnv("UPLOAD_DIR", "./uploads")
}

// batasUnggahByte adalah ukuran maksimum satu foto yang diterima server.
//
// Klien sudah mengecilkan gambar sebelum mengirim, jadi batas ini bukan
// ukuran yang diharapkan melainkan PAGAR: yang melewatinya berarti bukan
// datang dari aplikasi kita.
func batasUnggahByte() int64 {
	return int64(config.GetIntEnv("UPLOAD_MAX_KB", 2048)) * 1024
}

// SimpanFotoBarang menyimpan satu foto untuk sebuah barang, menggantikan yang
// lama bila ada, lalu mengembalikan alamat publiknya.
func SimpanFotoBarang(ctx context.Context, productID string, sumber io.Reader) (string, error) {
	var barang models.Product
	if err := repositories.FindProductInTenant(ctx, nil, productID, &barang); err != nil {
		return "", err
	}

	// Dibaca dengan BATAS, bukan dibaca habis lalu diukur: berkas 2 GB tidak
	// boleh sempat masuk memori hanya untuk kemudian ditolak.
	isi, err := io.ReadAll(io.LimitReader(sumber, batasUnggahByte()+1))
	if err != nil {
		return "", fmt.Errorf("%w: gagal membaca berkas: %v", helpers.ErrValidation, err)
	}
	if int64(len(isi)) > batasUnggahByte() {
		return "", fmt.Errorf("%w: foto terlalu besar, maksimal %d KB",
			helpers.ErrValidation, batasUnggahByte()/1024)
	}
	if len(isi) == 0 {
		return "", fmt.Errorf("%w: berkas kosong", helpers.ErrValidation)
	}

	// Jenis ditentukan dari ISI berkas, bukan dari header Content-Type maupun
	// ekstensi namanya — keduanya diisi klien dan bisa berbohong. Berkas PHP
	// yang diberi nama .jpg akan tertangkap di sini.
	ekstensi, ok := jenisGambarDiizinkan[strings.Split(http.DetectContentType(isi), ";")[0]]
	if !ok {
		return "", fmt.Errorf("%w: berkas harus berupa gambar JPG, PNG, atau WebP",
			helpers.ErrValidation)
	}

	folder := filepath.Join(FolderUnggah(), "barang")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		return "", fmt.Errorf("gagal menyiapkan folder unggahan: %w", err)
	}

	nama := ulid.New() + ekstensi
	if err := os.WriteFile(filepath.Join(folder, nama), isi, 0o644); err != nil {
		return "", fmt.Errorf("gagal menyimpan foto: %w", err)
	}
	alamat := "/uploads/barang/" + nama

	lama := barang.ImageURL
	if err := repositories.SetProductImageURL(ctx, productID, alamat); err != nil {
		// Basis data gagal diperbarui — berkasnya dibuang lagi supaya tidak
		// menumpuk sebagai sampah yang tidak dirujuk siapa pun.
		_ = os.Remove(filepath.Join(folder, nama))
		return "", err
	}

	hapusFotoLama(lama)
	return alamat, nil
}

// HapusFotoBarang melepas foto sebuah barang.
func HapusFotoBarang(ctx context.Context, productID string) error {
	var barang models.Product
	if err := repositories.FindProductInTenant(ctx, nil, productID, &barang); err != nil {
		return err
	}
	if err := repositories.SetProductImageURL(ctx, productID, ""); err != nil {
		return err
	}
	hapusFotoLama(barang.ImageURL)
	return nil
}

// hapusFotoLama membuang berkas yang sudah tidak dirujuk.
//
// Hanya menyentuh berkas DI DALAM folder unggahan kita. `image_url` bisa saja
// berisi alamat luar (kolomnya memang menerima URL sebelum unggahan ada), dan
// menghapus berdasarkan nilai kolom tanpa memeriksa lokasinya adalah cara
// mengubah kolom teks menjadi perintah hapus berkas sembarangan.
func hapusFotoLama(alamat string) {
	const awalan = "/uploads/barang/"
	if !strings.HasPrefix(alamat, awalan) {
		return
	}
	nama := filepath.Base(strings.TrimPrefix(alamat, awalan))
	if nama == "." || nama == ".." || nama == string(filepath.Separator) {
		return
	}
	_ = os.Remove(filepath.Join(FolderUnggah(), "barang", nama))
}
