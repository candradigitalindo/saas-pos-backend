package helpers

import (
	"fmt"
	"strconv"
	"strings"

	"candra/backend-api/config"
	"candra/backend-api/structs"

	"github.com/gin-gonic/gin"
)

// BuildPaginationResponse membuat struktur respons paginasi yang mirip dengan Laravel.
// Fitur utamanya adalah "sliding window" untuk tautan halaman, yang lebih efisien
// dan ramah pengguna untuk dataset yang besar.
func BuildPaginationResponse[T any](
	c *gin.Context,
	page, limit int,
	total int64,
	data []T,
) structs.PaginatedResponse[T] {
	req := c.Request

	// Tentukan base URL (scheme://host) untuk tautan paginasi.
	//
	// Utamakan APP_URL dari konfigurasi. Header Host dan X-Forwarded-Proto
	// dikirim oleh klien, jadi tanpa APP_URL penyerang bisa menyetel
	// "Host: evil.com" dan membuat seluruh tautan di response menunjuk ke
	// domain miliknya. Set APP_URL di production.
	baseURL := strings.TrimRight(config.GetEnv("APP_URL", ""), "/")
	if baseURL == "" {
		scheme := "http"
		// Cek header X-Forwarded-Proto yang biasa digunakan oleh reverse proxy (Nginx, Caddy, dll)
		if proto := req.Header.Get("X-Forwarded-Proto"); proto == "https" {
			scheme = "https"
		} else if req.TLS != nil {
			// Jika koneksi langsung ke server Go menggunakan TLS
			scheme = "https"
		}
		baseURL = fmt.Sprintf("%s://%s", scheme, req.Host)
	}

	basePath := baseURL + req.URL.Path

	// Helper untuk membuat URL sambil mempertahankan query parameter yang ada
	buildURL := func(p int) string {
		url := basePath
		// Ambil semua query parameter yang sudah ada
		query := req.URL.Query()
		// Set/update parameter 'page'
		query.Set("page", strconv.Itoa(p))
		// Pastikan parameter 'limit' juga ada di URL link
		query.Set("limit", strconv.Itoa(limit))
		// Gabungkan kembali menjadi URL lengkap
		return fmt.Sprintf("%s?%s", url, query.Encode())
	}

	lastPage := int((total + int64(limit) - 1) / int64(limit))
	if lastPage <= 0 {
		lastPage = 1
	}

	// from/to hanya bermakna bila halaman ini benar-benar berisi data.
	// Tanpa pengecekan len(data), permintaan halaman di luar jangkauan
	// (mis. total=5, limit=10, page=3) menghasilkan from=21 & to=20 —
	// angka yang saling bertolak belakang dan melebihi total.
	from := 0
	to := 0
	if len(data) > 0 {
		from = (page-1)*limit + 1
		to = (page-1)*limit + len(data)
	}

	// --- Logika untuk membuat "Sliding Window" Links ---
	const onEachSide = 2 // Jumlah link di setiap sisi halaman saat ini
	var links []structs.PaginationLink

	// Previous link
	var prevURL *string
	if page > 1 {
		url := buildURL(page - 1)
		prevURL = &url
	}
	links = append(links, structs.PaginationLink{URL: prevURL, Label: "&laquo; Previous", Active: false})

	// Kalkulasi jendela halaman
	windowStart := page - onEachSide
	windowEnd := page + onEachSide

	if windowEnd > lastPage {
		windowStart -= (windowEnd - lastPage)
		windowEnd = lastPage
	}
	if windowStart < 1 {
		windowEnd += (1 - windowStart)
		windowStart = 1
	}
	if windowEnd > lastPage {
		windowEnd = lastPage
	}

	// Tambah halaman pertama jika di luar jendela
	if windowStart > 1 {
		url := buildURL(1)
		links = append(links, structs.PaginationLink{URL: &url, Label: "1", Active: false})
	}
	// Tambah elipsis "..." jika ada jeda
	if windowStart > 2 {
		links = append(links, structs.PaginationLink{URL: nil, Label: "...", Active: false})
	}

	// Tambah link halaman di dalam jendela
	for i := windowStart; i <= windowEnd; i++ {
		url := buildURL(i)
		links = append(links, structs.PaginationLink{URL: &url, Label: strconv.Itoa(i), Active: i == page})
	}

	// Tambah elipsis "..." jika ada jeda
	if windowEnd < lastPage-1 {
		links = append(links, structs.PaginationLink{URL: nil, Label: "...", Active: false})
	}
	// Tambah halaman terakhir jika di luar jendela
	if windowEnd < lastPage {
		url := buildURL(lastPage)
		links = append(links, structs.PaginationLink{URL: &url, Label: strconv.Itoa(lastPage), Active: false})
	}

	// Next link
	var nextURL *string
	if page < lastPage {
		url := buildURL(page + 1)
		nextURL = &url
	}
	links = append(links, structs.PaginationLink{URL: nextURL, Label: "Next &raquo;", Active: false})

	return structs.PaginatedResponse[T]{
		CurrentPage:  page,
		Data:         data,
		FirstPageURL: buildURL(1),
		From:         from,
		LastPage:     lastPage,
		LastPageURL:  buildURL(lastPage),
		Links:        links,
		NextPageURL:  nextURL,
		Path:         basePath,
		PerPage:      limit,
		PrevPageURL:  prevURL,
		To:           to,
		Total:        total,
	}
}
