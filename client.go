package http3

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"log"
	"net"
	"net/http"

	"github.com/quic-go/qpack"
	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"

	uquic "github.com/refraction-networking/uquic"
	utls "github.com/refraction-networking/utls"
)

type udeocder interface {
	DecodeFull(p []byte) ([]qpack.HeaderField, error)
}

type HeaderField struct {
	Name  string
	Value string
}
type uencoder interface {
	WriteField(f qpack.HeaderField) error
	Close() error
}
type uconn interface {
	CloseWithError(uint64, string) error
	OpenStreamSync(context.Context) (io.ReadWriteCloser, error)
}
type Client struct {
	conn      uconn
	decoder   udeocder
	encoder   uencoder
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

type RoundTripper interface {
	Close(err string) error
	RoundTrip(req *http.Request) (*http.Response, error)
}

func Dial(ctx context.Context, addr string, tlsCfg *tls.Config, cfg *quic.Config) (quic.EarlyConnection, error) {
	udpConn, err := net.ListenUDP("udp", nil)
	if err != nil {
		log.Panic(err)
	}
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}
	return quic.DialEarly(ctx, udpConn, udpAddr, tlsCfg, cfg)
}
func UDial(ctx context.Context, addr string, tlsCfg *utls.Config, cfg *uquic.Config) (uquic.EarlyConnection, error) {
	udpConn, err := net.ListenUDP("udp", nil)
	if err != nil {
		log.Panic(err)
	}
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}
	return uquic.DialEarly(ctx, udpConn, udpAddr, tlsCfg, cfg)
}

type gconn struct {
	conn quic.EarlyConnection
}

func (obj *gconn) OpenStreamSync(ctx context.Context) (io.ReadWriteCloser, error) {
	return obj.conn.OpenStreamSync(ctx)
}
func (obj *gconn) CloseWithError(code uint64, reason string) error {
	return obj.conn.CloseWithError(0, reason)
}

type guconn struct {
	conn uquic.EarlyConnection
}

func (obj *guconn) OpenStreamSync(ctx context.Context) (io.ReadWriteCloser, error) {
	return obj.conn.OpenStreamSync(ctx)
}
func (obj *guconn) CloseWithError(code uint64, reason string) error {
	return obj.conn.CloseWithError(0, reason)
}

func NewClient(conn quic.EarlyConnection) RoundTripper {
	headerBuf := bytes.NewBuffer(nil)
	return &Client{
		conn:      &gconn{conn: conn},
		decoder:   qpack.NewDecoder(func(hf qpack.HeaderField) {}),
		encoder:   qpack.NewEncoder(headerBuf),
		headerBuf: headerBuf,
	}
}
func NewUClient(conn uquic.EarlyConnection) RoundTripper {
	headerBuf := bytes.NewBuffer(nil)
	return &Client{
		conn:      &guconn{conn: conn},
		decoder:   qpack.NewDecoder(func(hf qpack.HeaderField) {}),
		encoder:   qpack.NewEncoder(headerBuf),
		headerBuf: headerBuf,
	}
}
