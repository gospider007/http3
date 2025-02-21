package http3

import (
	"bytes"
	"context"
	"io"
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

	closeCtx context.Context
	closeCnl context.CancelCauseFunc
}

func (obj *Client) Stream() io.ReadWriteCloser {
	return nil
}
func (obj *Client) CloseCtx() context.Context {
	return obj.closeCtx
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
func (obj *Client) DoRequest(req *http.Request, orderHeaders []string) (*http.Response, context.Context, error) {
	str, err := obj.conn.OpenStreamSync(req.Context())
	if err != nil {
		return nil, nil, err
	}
	response, err := obj.doRequest(req, &stream{str: str}, orderHeaders)
	return response, cancelCtx{}, err
}
func (obj *Client) CloseWithError(err error) error {
	obj.closeCnl(err)
	var errStr string
	if err == nil {
		errStr = "Client closed"
	} else {
		errStr = err.Error()
	}
	if obj.closeFunc != nil {
		obj.closeFunc()
	}
	return obj.conn.CloseWithError(errStr)
}

var NextProtoH3 = http3.NextProtoH3

type Conn interface {
	CloseWithError(err error) error
	DoRequest(req *http.Request, orderHeaders []string) (*http.Response, context.Context, error)
	CloseCtx() context.Context
	Stream() io.ReadWriteCloser
}

type gconn struct {
	conn quic.EarlyConnection
}

func (obj *gconn) OpenStreamSync(ctx context.Context) (io.ReadWriteCloser, error) {
	return obj.conn.OpenStreamSync(ctx)
}
func (obj *gconn) CloseWithError(reason string) error {
	return obj.conn.CloseWithError(0, reason)
}

type guconn struct {
	conn uquic.EarlyConnection
}

func (obj *guconn) OpenStreamSync(ctx context.Context) (io.ReadWriteCloser, error) {
	return obj.conn.OpenStreamSync(ctx)
}
func (obj *guconn) CloseWithError(reason string) error {
	return obj.conn.CloseWithError(0, reason)
}

func NewClient(conn quic.EarlyConnection, closeFunc func()) Conn {
	headerBuf := bytes.NewBuffer(nil)
	closeCtx, closeCnl := context.WithCancelCause(context.TODO())
	return &Client{
		closeCtx: closeCtx,
		closeCnl: closeCnl,

		closeFunc: closeFunc,
		conn:      &gconn{conn: conn},
		decoder:   qpack.NewDecoder(func(hf qpack.HeaderField) {}),
		encoder:   qpack.NewEncoder(headerBuf),
		headerBuf: headerBuf,
	}
}
func NewUClient(conn uquic.EarlyConnection, closeFunc func()) Conn {
	headerBuf := bytes.NewBuffer(nil)
	return &Client{
		closeFunc: closeFunc,
		conn:      &guconn{conn: conn},
		decoder:   qpack.NewDecoder(func(hf qpack.HeaderField) {}),
		encoder:   qpack.NewEncoder(headerBuf),
		headerBuf: headerBuf,
	}
}
