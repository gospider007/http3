package http3

import (
	"bytes"
	"net/http"

	"github.com/quic-go/qpack"
	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
)

type roundTripperOpts struct {
	DisableCompression bool
	EnableDatagram     bool
	MaxHeaderBytes     int64
	AdditionalSettings map[uint64]uint64
}

type client struct {
	conn      quic.EarlyConnection
	opts      roundTripperOpts
	decoder   *qpack.Decoder
	encoder   *qpack.Encoder
	headerBuf *bytes.Buffer
}

func (obj *client) RoundTrip(req *http.Request) (*http.Response, error) {
	str, err := obj.conn.OpenStreamSync(req.Context())
	if err != nil {
		return nil, err
	}
	return obj.doRequest(req, str)
}

type RoundTripOpt = http3.RoundTripOpt

var NextProtoH3 = http3.NextProtoH3

func NewClient(conn quic.EarlyConnection) *client {
	headerBuf := bytes.NewBuffer(nil)
	return &client{
		conn:      conn,
		opts:      roundTripperOpts{},
		decoder:   qpack.NewDecoder(func(hf qpack.HeaderField) {}),
		encoder:   qpack.NewEncoder(headerBuf),
		headerBuf: headerBuf,
	}
}
