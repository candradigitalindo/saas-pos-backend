package tests

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// jsonRequest membangun *http.Request JSON tanpa langsung mengirimnya — untuk
// handler yang butuh header tambahan (mis. Idempotency-Key). Kirim dengan serve.
func jsonRequest(t *testing.T, method, path, token string, payload any) *http.Request {
	t.Helper()
	var body io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("marshal payload: %v", err)
		}
		body = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, body)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req
}

// serve mengirim req ke router dan mendecode responsnya.
func serve(t *testing.T, req *http.Request) apiResp {
	t.Helper()
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	raw := rec.Body.String()
	out := apiResp{Code: rec.Code, Raw: raw}
	if raw != "" {
		if err := json.Unmarshal([]byte(raw), &out.Body); err != nil {
			t.Fatalf("%s %s: response bukan JSON valid (%d): %s", req.Method, req.URL.Path, rec.Code, raw)
		}
	}
	return out
}

// apiResp adalah hasil satu permintaan HTTP ke router, sudah didecode.
type apiResp struct {
	Code int
	Body map[string]any
	Raw  string
}

// call mengirim permintaan JSON ke router aplikasi dan mengembalikan response
// yang sudah didecode. token kosong = tanpa header Authorization. payload nil =
// tanpa body.
func call(t *testing.T, method, path, token string, payload any) apiResp {
	t.Helper()

	var body io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("marshal payload: %v", err)
		}
		body = bytes.NewReader(b)
	}

	req := httptest.NewRequest(method, path, body)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	raw := rec.Body.String()
	out := apiResp{Code: rec.Code, Raw: raw}
	if raw != "" {
		if err := json.Unmarshal([]byte(raw), &out.Body); err != nil {
			t.Fatalf("%s %s: response bukan JSON valid (%d): %s", method, path, rec.Code, raw)
		}
	}
	return out
}

// callRaw mengirim permintaan dengan body mentah dan Content-Type tertentu
// (dipakai uji impor CSV: text/csv).
func callRaw(t *testing.T, method, path, token, contentType, body string) apiResp {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", contentType)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	raw := rec.Body.String()
	out := apiResp{Code: rec.Code, Raw: raw}
	if raw != "" {
		if err := json.Unmarshal([]byte(raw), &out.Body); err != nil {
			t.Fatalf("%s %s: response bukan JSON valid (%d): %s", method, path, rec.Code, raw)
		}
	}
	return out
}

// mustOK menggagalkan test bila status di luar rentang 2xx.
func (r apiResp) mustOK(t *testing.T, ctx string) apiResp {
	t.Helper()
	if r.Code < 200 || r.Code >= 300 {
		t.Fatalf("%s: status %d, mau 2xx. body: %s", ctx, r.Code, r.Raw)
	}
	return r
}

// mustCode menggagalkan test bila status tidak sama dengan want.
func (r apiResp) mustCode(t *testing.T, ctx string, want int) apiResp {
	t.Helper()
	if r.Code != want {
		t.Fatalf("%s: status %d, mau %d. body: %s", ctx, r.Code, want, r.Raw)
	}
	return r
}

// data mengambil sub-objek "data" dari response.
func (r apiResp) data(t *testing.T) map[string]any {
	t.Helper()
	d, ok := r.Body["data"].(map[string]any)
	if !ok {
		t.Fatalf("response tidak punya objek 'data': %s", r.Raw)
	}
	return d
}

// str mengambil field string bersarang lewat path titik, mis. get("auth.access_token").
func get[T any](t *testing.T, m map[string]any, path string) T {
	t.Helper()
	var cur any = m
	segs := splitDots(path)
	for i, s := range segs {
		obj, ok := cur.(map[string]any)
		if !ok {
			t.Fatalf("path %q: segmen %q bukan objek", path, s)
		}
		cur = obj[s]
		if i == len(segs)-1 {
			v, ok := cur.(T)
			if !ok {
				t.Fatalf("path %q: tipe %T, mau %T (nilai: %v)", path, cur, *new(T), cur)
			}
			return v
		}
	}
	var zero T
	return zero
}

func splitDots(s string) []string {
	out := []string{}
	cur := ""
	for _, r := range s {
		if r == '.' {
			out = append(out, cur)
			cur = ""
			continue
		}
		cur += string(r)
	}
	return append(out, cur)
}

// jsonArray mengubah []any menjadi []string (untuk field permissions/outlet_ids).
func jsonArray(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, e := range arr {
		if s, ok := e.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
