package helpers

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

const (
	defaultPerPage = 10
	// maxPerPage membatasi jumlah item per halaman agar satu request tidak bisa
	// memaksa memuat seluruh tabel ke memori (perlindungan DoS).
	maxPerPage = 100
)

// ParsePaginationParams mengekstrak dan memvalidasi parameter 'page' dan 'limit' dari query URL.
// Ini membantu menjaga kode tetap DRY (Don't Repeat Yourself) di seluruh controller.
func ParsePaginationParams(c *gin.Context) (page, limit, offset int) {
	page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ = strconv.Atoi(c.DefaultQuery("limit", strconv.Itoa(defaultPerPage)))

	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = defaultPerPage
	}
	if limit > maxPerPage {
		limit = maxPerPage
	}
	offset = (page - 1) * limit
	return
}
