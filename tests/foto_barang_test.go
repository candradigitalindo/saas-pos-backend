package tests

import (
	"bytes"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Uji unggah foto barang.
//
// Yang dijaga di sini bukan kenyamanan, melainkan bahwa folder unggahan TIDAK
// PERNAH memuat apa pun selain gambar. Berkasnya disajikan apa adanya dari
// disk tanpa autentikasi; satu berkas HTML atau skrip yang lolos ke sana
// menjadi halaman yang dijalankan browser di bawah domain aplikasi ini.

// PNG 1×1 piksel yang sah, sebagai bahan uji.
var pngSatuPiksel = []byte{
	0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a,
	0x00, 0x00, 0x00, 0x0d, 'I', 'H', 'D', 'R',
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4, 0x89,
	0x00, 0x00, 0x00, 0x0a, 'I', 'D', 'A', 'T',
	0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00, 0x05, 0x00, 0x01,
	0x0d, 0x0a, 0x2d, 0xb4,
	0x00, 0x00, 0x00, 0x00, 'I', 'E', 'N', 'D', 0xae, 0x42, 0x60, 0x82,
}

// unggahFoto mengirim satu berkas sebagai multipart ke endpoint foto barang.
func unggahFoto(t *testing.T, token, productID, namaBerkas string, isi []byte) apiResp {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	bagian, err := w.CreateFormFile("file", namaBerkas)
	if err != nil {
		t.Fatalf("menyusun multipart: %v", err)
	}
	if _, err := bagian.Write(isi); err != nil {
		t.Fatalf("menulis isi: %v", err)
	}
	w.Close()

	req := httptest.NewRequest("POST", "/api/v1/products/"+productID+"/image", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	return serve(t, req)
}

func TestUnggahFotoBarangMenerimaGambar(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "fotobrg1")
	t.Setenv("UPLOAD_DIR", t.TempDir())

	res := unggahFoto(t, f.token, f.prodA, "kopi.png", pngSatuPiksel)
	res.mustOK(t, "unggah foto")

	alamat, _ := res.data(t)["image_url"].(string)
	if !strings.HasPrefix(alamat, "/uploads/barang/") || !strings.HasSuffix(alamat, ".png") {
		t.Fatalf("image_url = %q, mau /uploads/barang/<ulid>.png", alamat)
	}
	// Nama berkas TIDAK boleh mengandung nama kiriman klien.
	if strings.Contains(alamat, "kopi") {
		t.Fatalf("image_url = %q memuat nama berkas dari klien — nama harus dibuat server", alamat)
	}
	if _, err := os.Stat(filepath.Join(os.Getenv("UPLOAD_DIR"), "barang", filepath.Base(alamat))); err != nil {
		t.Fatalf("berkas tidak ada di disk: %v", err)
	}
}

// Berkas yang BUKAN gambar harus ditolak walau namanya .png dan Content-Type
// -nya mengaku gambar. Keduanya diisi klien dan bisa berbohong.
func TestUnggahFotoBarangMenolakYangBukanGambar(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "fotobrg2")
	dir := t.TempDir()
	t.Setenv("UPLOAD_DIR", dir)

	kasus := []struct {
		nama string
		isi  []byte
	}{
		{"html.png", []byte("<html><script>alert(1)</script></html>")},
		{"php.png", []byte("<?php system($_GET['c']); ?>")},
		{"svg.png", []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script/></svg>`)},
		{"kosong.png", []byte{}},
	}
	for _, k := range kasus {
		t.Run(k.nama, func(t *testing.T) {
			res := unggahFoto(t, f.token, f.prodA, k.nama, k.isi)
			if res.Code < 400 {
				t.Fatalf("status %d — berkas %q seharusnya ditolak", res.Code, k.nama)
			}
		})
	}

	// Tidak satu pun berkas boleh mendarat di disk.
	folder := filepath.Join(dir, "barang")
	if isi, err := os.ReadDir(folder); err == nil && len(isi) > 0 {
		var nama []string
		for _, e := range isi {
			nama = append(nama, e.Name())
		}
		t.Fatalf("folder unggahan memuat %v — berkas ditolak tapi tetap tersimpan", nama)
	}
}

// Foto lama dibuang saat diganti, supaya disk tidak dipenuhi berkas yatim.
func TestUnggahFotoBarangMenggantiYangLama(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "fotobrg3")
	dir := t.TempDir()
	t.Setenv("UPLOAD_DIR", dir)

	pertama, _ := unggahFoto(t, f.token, f.prodA, "a.png", pngSatuPiksel).
		mustOK(t, "unggah pertama").data(t)["image_url"].(string)
	kedua, _ := unggahFoto(t, f.token, f.prodA, "b.png", pngSatuPiksel).
		mustOK(t, "unggah kedua").data(t)["image_url"].(string)

	if pertama == kedua {
		t.Fatal("dua unggahan menghasilkan nama berkas yang sama")
	}
	folder := filepath.Join(dir, "barang")
	isi, _ := os.ReadDir(folder)
	if len(isi) != 1 {
		t.Fatalf("ada %d berkas di disk, mau 1 — foto lama tidak dibuang", len(isi))
	}
}
