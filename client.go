package http3

import (
	"bytes"
	"net/http"

	"github.com/quic-go/qpack"
	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
)

type Client struct {
	conn      quic.EarlyConnection
	decoder   *qpack.Decoder
	encoder   *qpack.Encoder
	headerBuf *bytes.Buffer
}

func (obj *Client) RoundTrip(req *http.Request) (*http.Response, error) {
	str, err := obj.conn.OpenStreamSync(req.Context())
	if err != nil {
		return nil, err
	}
	return obj.doRequest(req, &stream{str: str})
}
func (obj *Client) Close(err string) error {
	if err == "" {
		err = "Client closed"
	}
	return obj.conn.CloseWithError(0, err)
}

var NextProtoH3 = http3.NextProtoH3

type SingleDestinationRoundTripper = http3.SingleDestinationRoundTripper

type Clilent2 struct {
	r *http3.SingleDestinationRoundTripper
}

func (c *Clilent2) RoundTrip(req *http.Request) (*http.Response, error) {
	return c.r.RoundTrip(req)
}
func (c *Clilent2) Close(err string) error {
	return c.r.Connection.CloseWithError(0, err)
}

type RoundTripper interface {
	Close(err string) error
	RoundTrip(req *http.Request) (*http.Response, error)
}

func NewClient(conn quic.EarlyConnection) RoundTripper {
	headerBuf := bytes.NewBuffer(nil)
	return &Client{
		conn:      conn,
		decoder:   qpack.NewDecoder(func(hf qpack.HeaderField) {}),
		encoder:   qpack.NewEncoder(headerBuf),
		headerBuf: headerBuf,
	}
}
