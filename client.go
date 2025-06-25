package http3

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/quic-go/qpack"
	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"

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
	OpenStreamSync(context.Context) (io.ReadWriteCloser, error)
}
type Client struct {
	closeFunc func()
	conn      uconn
	decoder   udeocder
	encoder   uencoder
	headerBuf *bytes.Buffer
}

func (obj *Client) Stream() io.ReadWriteCloser {
	return nil
}

type cancelCtx struct {
}

func (obj cancelCtx) Done() <-chan struct{} {
	c := make(chan struct{})
	close(c)
	return c
}
func (obj cancelCtx) Err() error {
	return context.Canceled
}
func (obj cancelCtx) Deadline() (time.Time, bool) {
	return time.Time{}, false
}
func (obj cancelCtx) Value(key interface{}) interface{} {
	return nil
}
func (obj *Client) DoRequest(req *http.Request, orderHeaders []interface {
	Key() string
	Val() any
}) (*http.Response, context.Context, error) {
	str, err := obj.conn.OpenStreamSync(req.Context())
	if err != nil {
		return nil, nil, err
	}
	response, err := obj.doRequest(req, &stream{str: str}, orderHeaders)
	return response, cancelCtx{}, err
}
func (obj *Client) CloseWithError(err error) error {
	if obj.closeFunc != nil {
		obj.closeFunc()
	}
	var errStr string
	if err == nil {
		errStr = "Client closed"
	} else {
		errStr = err.Error()
	}
	return obj.conn.CloseWithError(errStr)
}

var NextProtoH3 = http3.NextProtoH3

type Conn interface {
	CloseWithError(err error) error
	DoRequest(*http.Request, []interface {
		Key() string
		Val() any
	}) (*http.Response, context.Context, error)
	Stream() io.ReadWriteCloser
}

type gconn struct {
	conn    *quic.Conn
	udpConn net.PacketConn
}

func (obj *gconn) OpenStreamSync(ctx context.Context) (io.ReadWriteCloser, error) {
	return obj.conn.OpenStreamSync(ctx)
}
func (obj *gconn) CloseWithError(reason string) error {
	obj.conn.CloseWithError(0, reason)
	return obj.udpConn.Close()
}

type guconn struct {
	conn    uquic.EarlyConnection
	udpConn net.PacketConn
}

func (obj *guconn) OpenStreamSync(ctx context.Context) (io.ReadWriteCloser, error) {
	return obj.conn.OpenStreamSync(ctx)
}
func (obj *guconn) CloseWithError(reason string) error {
	obj.conn.CloseWithError(0, reason)
	return obj.udpConn.Close()
}

func newClient(conn uconn, closeFunc func()) Conn {
	headerBuf := bytes.NewBuffer(nil)
	return &Client{
		closeFunc: closeFunc,
		conn:      conn,
		decoder:   qpack.NewDecoder(func(hf qpack.HeaderField) {}),
		encoder:   qpack.NewEncoder(headerBuf),
		headerBuf: headerBuf,
	}
}
func NewClient(conn any, udpConn net.PacketConn, closeFunc func()) (Conn, error) {
	var wrapCon uconn
	switch conn := conn.(type) {
	case uquic.EarlyConnection:
		wrapCon = &guconn{conn: conn, udpConn: udpConn}
	case *quic.Conn:
		wrapCon = &gconn{conn: conn, udpConn: udpConn}
	default:
		return nil, errors.New("unsupported connection type")
	}
	return newClient(wrapCon, closeFunc), nil
}
