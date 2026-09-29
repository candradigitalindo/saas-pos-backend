package services

import (
	"net/url"
	"testing"
)

// Contoh resmi "HTTP request sample" Lazada Open Platform: GetOrder dengan App
// Secret "helloworld".
func TestTandaTanganLazadaContohResmi(t *testing.T) {
	q := url.Values{
		"app_key": {"123456"}, "access_token": {"test"}, "timestamp": {"1517820392000"},
		"sign_method": {"sha256"}, "order_id": {"1234"},
	}
	const want = "4190D32361CFB9581350222F345CB77F3B19F0E31D162316848A2C1FFD5FAB4A"
	if got := lazadaSign("helloworld", "/order/get", q); got != want {
		t.Fatalf("sign = %s, contoh resmi = %s", got, want)
	}
	// sign lama & parameter kosong tidak ikut ditandatangani.
	q.Set("sign", "LAMA")
	q.Set("status", "")
	if got := lazadaSign("helloworld", "/order/get", q); got != want {
		t.Fatalf("sign dengan sign lama/nilai kosong = %s", got)
	}
}

// Push per BARIS pesanan: batal satu baris diperiksa pekerja (bisa jadi hanya
// sebagian), dan dua baris batal tidak saling membuang sebagai duplikat.
func TestPushLazadaBatalPerBaris(t *testing.T) {
	cred := ChannelCredentials{"seller_id": "100200"}
	body := func(status, baris string) []byte {
		return []byte(`{"seller_id":"100200","message_type":0,"data":{"order_status":"` + status +
			`","trade_order_id":"LZ1","trade_order_line_id":"` + baris + `","status_update_time":1790500000},"timestamp":1790500001000,"site":"lazada_id"}`)
	}
	a, _ := (lazadaAdapter{}).Events(body("canceled", "L1"), cred)
	b, _ := (lazadaAdapter{}).Events(body("canceled", "L2"), cred)
	if len(a) != 1 || a[0].EventType != "order.canceled" || !a[0].NeedsFetch || a[0].DedupKey == b[0].DedupKey {
		t.Fatalf("batal per baris: %+v / %+v", a, b)
	}
	if ev, _ := (lazadaAdapter{}).Events(body("unpaid", "L1"), cred); len(ev) != 0 {
		t.Fatalf("belum dibayar harus diabaikan: %+v", ev)
	}
	if ev, _ := (lazadaAdapter{}).Events(body("pending", "L1"), ChannelCredentials{"seller_id": "lain"}); len(ev) != 0 {
		t.Fatalf("push penjual lain harus diabaikan: %+v", ev)
	}
}
