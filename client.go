package http3

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"

	"github.com/gospider007/http1"
	"github.com/quic-go/qpack"
	"github.com/quic-go/quic-go"

	uquic "github.com/refraction-networking/uquic"
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
	CloseWithError(string) error
	OpenStream() (io.ReadWriteCloser, error)
}
type Client struct {
	ctx       context.Context
	cnl       context.CancelCauseFunc
	conn      uconn
	decoder   udeocder
	encoder   uencoder
	headerBuf *bytes.Buffer
}

func (obj *Client) Context() context.Context {
	return obj.ctx
}

func (obj *Client) Stream() io.ReadWriteCloser {
	return nil
}

func (obj *Client) DoRequest(ctx context.Context, req *http.Request, option *http1.Option) (*http.Response, error) {
	str, err := obj.conn.OpenStream()
	if err != nil {
		return nil, err
	}
	response, err := obj.doRequest(ctx, req, &stream{str: str}, option.OrderHeaders)
	if err != nil {
		obj.CloseWithError(err)
	}
	return response, err
}

func (obj *Client) CloseWithError(err error) error {
	obj.cnl(err)
	var errStr string
	if err == nil {
		errStr = "Client closed"
	} else {
		errStr = err.Error()
	}
	return obj.conn.CloseWithError(errStr)
}

var NextProtoH3 = "h3"

type gconn struct {
	conn    *quic.Conn
	udpConn net.PacketConn
}

func (obj *gconn) OpenStream() (io.ReadWriteCloser, error) {
	return obj.conn.OpenStream()
}
func (obj *gconn) CloseWithError(reason string) error {
	obj.conn.CloseWithError(0, reason)
	return obj.udpConn.Close()
}

type guconn struct {
	conn    uquic.EarlyConnection
	udpConn net.PacketConn
}

func (obj *guconn) OpenStream() (io.ReadWriteCloser, error) {
	return obj.conn.OpenStream()
}
func (obj *guconn) CloseWithError(reason string) error {
	obj.conn.CloseWithError(0, reason)
	return obj.udpConn.Close()
}

func newClient(preCtx context.Context, conn uconn) http1.Conn {
	headerBuf := bytes.NewBuffer(nil)
	ctx, cnl := context.WithCancelCause(preCtx)
	return &Client{
		ctx:       ctx,
		cnl:       cnl,
		conn:      conn,
		decoder:   qpack.NewDecoder(func(hf qpack.HeaderField) {}),
		encoder:   qpack.NewEncoder(headerBuf),
		headerBuf: headerBuf,
	}
}
func NewConn(ctx context.Context, conn any, udpConn net.PacketConn) http1.Conn {
	var wrapCon uconn
	switch conn := conn.(type) {
	case uquic.EarlyConnection:
		wrapCon = &guconn{conn: conn, udpConn: udpConn}
	case *quic.Conn:
		wrapCon = &gconn{conn: conn, udpConn: udpConn}
	default:
		return nil
	}
	return newClient(ctx, wrapCon)
}
