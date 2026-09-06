package helpers

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func testContext(target string) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", target, nil)
	return c
}

// Halaman di luar jangkauan tidak boleh menghasilkan from/to yang saling
// bertolak belakang (dulu: total=5, limit=10, page=3 -> from=21, to=20).
func TestBuildPaginationResponseFromTo(t *testing.T) {
	empty := BuildPaginationResponse(testContext("/api/user?page=3&limit=10"), 3, 10, 5, []string{})
	if empty.From != 0 || empty.To != 0 {
		t.Fatalf("halaman kosong: from=%d to=%d, ingin 0/0", empty.From, empty.To)
	}

	filled := BuildPaginationResponse(testContext("/api/user?page=1&limit=10"), 1, 10, 5, []string{"a", "b"})
	if filled.From != 1 || filled.To != 2 {
		t.Fatalf("halaman berisi: from=%d to=%d, ingin 1/2", filled.From, filled.To)
	}
}

// APP_URL harus mengalahkan header Host yang dikirim klien, agar tautan di
// response tidak bisa diarahkan ke domain penyerang.
func TestBuildPaginationResponseUsesAppURL(t *testing.T) {
	t.Setenv("APP_URL", "https://api.tokosaya.com")
	c := testContext("/api/user")
	c.Request.Host = "evil.example.com"

	res := BuildPaginationResponse(c, 1, 10, 5, []string{"a"})
	if res.Path != "https://api.tokosaya.com/api/user" {
		t.Fatalf("Path = %q, ingin memakai APP_URL", res.Path)
	}
}
