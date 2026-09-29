package services

import (
	"archive/zip"
	"bytes"
	"fmt"
	"strings"
	"unicode/utf8"
)

// Penulis XLSX minimal — satu lembar, tanpa pustaka luar.
//
// Kenapa bukan CSV saja: Excel berbahasa Indonesia memakai TITIK KOMA sebagai
// pemisah daftar (koma dipakai untuk desimal), jadi CSV berpemisah koma
// terbuka menumpuk di satu kolom; sebaliknya CSV titik koma rusak di Excel
// berbahasa Inggris. XLSX tidak bergantung pada pengaturan wilayah, dan angka
// tersimpan sebagai ANGKA — bisa langsung dijumlahkan.
//
// Kenapa bukan pustaka (excelize): yang dibutuhkan hanya satu tabel dengan
// judul tebal & baris judul dibekukan. XLSX adalah zip berisi beberapa XML
// (Office Open XML); menulisnya sendiri ±100 baris, tanpa dependensi berat.

// Angka adalah nilai sel numerik yang ditulis apa adanya ("11.5", "-3000").
// Nilai lain (string) ditulis sebagai teks.
type Angka string

// tulisXLSX menyusun berkas .xlsx satu lembar. Baris pertama = judul kolom
// (tebal, dibekukan saat menggulir).
func tulisXLSX(namaLembar string, baris [][]any) ([]byte, error) {
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	tulis := func(nama, isi string) error {
		w, err := z.Create(nama)
		if err != nil {
			return err
		}
		_, err = w.Write([]byte(isi))
		return err
	}

	berkas := []struct{ nama, isi string }{
		{"[Content_Types].xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
<Default Extension="xml" ContentType="application/xml"/>
<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
<Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
<Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>
</Types>`},
		{"_rels/.rels", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>
</Relationships>`},
		{"xl/workbook.xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
<sheets><sheet name="` + escXML(potongNamaLembar(namaLembar)) + `" sheetId="1" r:id="rId1"/></sheets>
</workbook>`},
		{"xl/_rels/workbook.xml.rels", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>
</Relationships>`},
		// Gaya 0 = biasa, 1 = tebal (judul), 2 = rupiah ribuan "#,##0".
		{"xl/styles.xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
<fonts count="2"><font><sz val="11"/><name val="Calibri"/></font><font><b/><sz val="11"/><name val="Calibri"/></font></fonts>
<fills count="2"><fill><patternFill patternType="none"/></fill><fill><patternFill patternType="gray125"/></fill></fills>
<borders count="1"><border><left/><right/><top/><bottom/><diagonal/></border></borders>
<cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs>
<cellXfs count="3"><xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/><xf numFmtId="0" fontId="1" fillId="0" borderId="0" xfId="0" applyFont="1"/><xf numFmtId="3" fontId="0" fillId="0" borderId="0" xfId="0" applyNumberFormat="1"/></cellXfs>
<cellStyles count="1"><cellStyle name="Normal" xfId="0" builtinId="0"/></cellStyles>
</styleSheet>`},
		{"xl/worksheets/sheet1.xml", lembarXML(baris)},
	}
	for _, b := range berkas {
		if err := tulis(b.nama, b.isi); err != nil {
			return nil, err
		}
	}
	if err := z.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// lembarXML menyusun isi sheet1.xml: lebar kolom menurut isi terpanjang,
// baris judul dibekukan, teks sebagai inlineStr, angka sebagai <v>.
func lembarXML(baris [][]any) string {
	var lebar []int
	for _, r := range baris {
		for j, v := range r {
			for len(lebar) <= j {
				lebar = append(lebar, 8)
			}
			if n := utf8.RuneCountInString(fmt.Sprint(v)) + 2; n > lebar[j] {
				lebar[j] = min(n, 60)
			}
		}
	}

	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
<sheetViews><sheetView workbookViewId="0"><pane ySplit="1" topLeftCell="A2" activePane="bottomLeft" state="frozen"/></sheetView></sheetViews>
<cols>`)
	for j, w := range lebar {
		fmt.Fprintf(&b, `<col min="%d" max="%d" width="%d" customWidth="1"/>`, j+1, j+1, w)
	}
	b.WriteString("</cols><sheetData>")
	for i, r := range baris {
		fmt.Fprintf(&b, `<row r="%d">`, i+1)
		for j, v := range r {
			ref := kolomXLSX(j) + fmt.Sprint(i+1)
			switch x := v.(type) {
			case Angka:
				// Jumlah (bisa desimal, 2.5 kg): format umum — "#,##0" akan
				// menampilkannya sebagai 3.
				if x == "" {
					continue
				}
				fmt.Fprintf(&b, `<c r="%s"><v>%s</v></c>`, ref, escXML(string(x)))
			case int64: // rupiah & hitungan: ribuan
				fmt.Fprintf(&b, `<c r="%s" s="2"><v>%d</v></c>`, ref, x)
			default:
				s := fmt.Sprint(x)
				if s == "" {
					continue
				}
				gaya := ""
				if i == 0 {
					gaya = ` s="1"`
				}
				fmt.Fprintf(&b, `<c r="%s" t="inlineStr"%s><is><t xml:space="preserve">%s</t></is></c>`, ref, gaya, escXML(s))
			}
		}
		b.WriteString("</row>")
	}
	b.WriteString("</sheetData></worksheet>")
	return b.String()
}

// kolomXLSX: indeks kolom (0 = A) → huruf kolom ("A", "Z", "AA").
func kolomXLSX(i int) string {
	s := ""
	for i++; i > 0; i = (i - 1) / 26 {
		s = string(rune('A'+(i-1)%26)) + s
	}
	return s
}

// potongNamaLembar: nama lembar Excel ≤31 karakter & tanpa []:*?/\.
func potongNamaLembar(s string) string {
	s = strings.Map(func(r rune) rune {
		if strings.ContainsRune(`[]:*?/\`, r) {
			return '-'
		}
		return r
	}, s)
	if utf8.RuneCountInString(s) > 31 {
		s = string([]rune(s)[:31])
	}
	return s
}

// escXML meloloskan teks untuk isi elemen/atribut XML dan membuang karakter
// kendali yang tidak sah di XML 1.0 (kecuali tab & baris baru).
func escXML(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		case '"':
			b.WriteString("&quot;")
		default:
			if r < 0x20 && r != '\t' && r != '\n' && r != '\r' {
				continue
			}
			b.WriteRune(r)
		}
	}
	return b.String()
}
