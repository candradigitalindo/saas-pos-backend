package services

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"mime"
	"net/http"
	"net/url"
	"sort"
	"testing"
)

// Vektor resmi dari dokumen webhook Tokopedia & Shop ("Header and signature
// verification"): app key "abcdef", app secret "123".
func TestTandaTanganWebhookTokopediaVektorResmi(t *testing.T) {
	body := []byte(`{"type":1,"tts_notification_id":"7380066284010030890","shop_id":"7495540735365777507","timestamp":1718305585,"data":{"is_on_hold_order":true,"order_id":"576653688135258178","order_status":"UNPAID","update_time":1718305585}}`)
	cred := ChannelCredentials{"app_key": "abcdef", "app_secret": "123"}
	h := http.Header{}
	h.Set("Authorization", "5dec0f11ec2f6783b8deee53c9ffbf8d024302f7c7e7fa55a35d17629031ac05")
	if err := (tokopediaAdapter{}).VerifySignature(h, body, cred, ""); err != nil {
		t.Fatalf("vektor resmi ditolak: %v", err)
	}
	h.Set("Authorization", "Bearer 5dec0f11ec2f6783b8deee53c9ffbf8d024302f7c7e7fa55a35d17629031ac05")
	if (tokopediaAdapter{}).VerifySignature(h, body, cred, "") == nil {
		t.Fatal("awalan Bearer bukan format platform — harus ditolak")
	}
	if (tokopediaAdapter{}).VerifySignature(h, append(body, ' '), cred, "") == nil {
		t.Fatal("body berubah tetap diterima")
	}
}

// calSignDokumen: contoh Go resmi "Sign your API request", apa adanya.
func calSignDokumen(req *http.Request, secret string) string {
	queries := req.URL.Query()
	keys := make([]string, 0, len(queries))
	for k := range queries {
		if k != "sign" && k != "access_token" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	input := ""
	for _, key := range keys {
		input = input + key + queries.Get(key)
	}
	input = req.URL.Path + input
	mediaType, _, _ := mime.ParseMediaType(req.Header.Get("Content-type"))
	if mediaType != "multipart/form-data" {
		body, _ := io.ReadAll(req.Body)
		input = input + string(body)
		req.Body = io.NopCloser(bytes.NewReader(body))
	}
	input = secret + input + secret
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(input))
	return hex.EncodeToString(h.Sum(nil))
}

func TestTandaTanganAPITokopediaSamaDenganContohResmi(t *testing.T) {
	q := url.Values{"app_key": {"38abcd"}, "timestamp": {"1623812664"}, "shop_cipher": {"ROW_x"}, "access_token": {"tidak-ikut"}}
	body := []byte(`{"address":"https://pos.contoh.id/webhooks/channels/tokopedia/t","event_type":"ORDER_STATUS_CHANGE"}`)
	mine := tokopediaSign("rahasia", "/event/202309/webhooks", q, body)
	req, _ := http.NewRequest(http.MethodPut, "https://open-api.contoh/event/202309/webhooks?"+q.Encode(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if want := calSignDokumen(req, "rahasia"); mine != want {
		t.Fatalf("sign = %s, contoh resmi = %s", mine, want)
	}
}
