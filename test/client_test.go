package main

import (
	"context"
	"crypto/tls"
	"log"
	"net/http"
	"net/url"
	"testing"

	"github.com/gospider007/http3"
)

func TestMain(t *testing.T) {
	href := "https://cloudflare-quic.com/"
	// href = "https://sparrow.cloudflare.com/api/v1/identify"
	u, err := url.Parse(href)
	if err != nil {
		log.Panic(err)
	}
	req := &http.Request{
		// Method: http.MethodGet,
		Method: http.MethodPost,

		URL:  u,
		Host: "cloudflare-quic.com",
		// Body: io.NopCloser(cc),
		// Proto:  "HTTP/1.1",
		// ProtoMajor: 1, // 1
		// ProtoMinor: 1, // 0
		Header: http.Header{
			"User-Agent": []string{"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/112.0.0.0 Safari/537.36"},
		},
	}
	conn, err := http3.DialEarly(context.TODO(), u.Host+":443", &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         u.Host,
		NextProtos:         []string{http3.NextProtoH3},
	}, nil)
	if err != nil {
		log.Print("超时了: ", err)
		// log.Panic(err)
		return
	}
	log.Print("good")
	// client2 := http3.NewClient2(conn)
	// resp, err := client2.RoundTrip(req)
	// if err != nil {
	// 	log.Panic(err)
	// }
	// log.Print(resp.Status)

	client := http3.NewClient(conn)
	for i := 0; i < 2; i++ {
		resp, err := client.RoundTrip(req)
		if err != nil {
			log.Panic(err)
		}
		log.Print(resp.StatusCode)
	}
}
