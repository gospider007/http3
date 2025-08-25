package http3

import (
	"context"
	"errors"
	"io"
	"net/http"

	"github.com/gospider007/http1"
)

func (obj *Client) sendRequest(req *http.Request, str *stream, orderHeaders []interface {
	Key() string
	Val() any
}) error {
	defer str.Close()
	if err := obj.writeHeaders(str, req, orderHeaders); err != nil {
		return err
	}
	if req.Body != nil {
		_, err := io.CopyBuffer(str, req.Body, make([]byte, bodyCopyBufferSize))
		req.Body.Close()
		return err
	}
	return nil
}

func (obj *Client) readResponse(str *stream) (*http.Response, error) {
	t, l, err := str.parseNextFrame()
	if err != nil {
		return nil, err
	}
	if t != frameTypeHeaders {
		return nil, errors.New("not headers Frames")
	}
	headerBlock := make([]byte, l)
	if _, err := io.ReadFull(str.str, headerBlock); err != nil {
		return nil, err
	}
	hfs, err := obj.decoder.DecodeFull(headerBlock)
	if err != nil {
		return nil, err
	}
	res, err := responseFromHeaders(hfs)
	if err != nil {
		return nil, err
	}
	ctx, cnl := context.WithCancelCause(obj.ctx)
	res.Body = http1.NewBody(str, obj, ctx, cnl, false, nil)
	return res, nil
}

func (obj *Client) doRequest(ctx context.Context, req *http.Request, str *stream, orderHeaders []interface {
	Key() string
	Val() any
}) (*http.Response, error) {
	var readErr error
	var resp *http.Response
	writeDone := make(chan struct{})
	readDone := make(chan struct{})
	go func() {
		defer close(writeDone)
		writeErr := obj.sendRequest(req, str, orderHeaders)
		if writeErr != nil {
			obj.CloseWithError(writeErr)
		}
	}()
	go func() {
		defer close(readDone)
		resp, readErr = obj.readResponse(str)
		if readErr != nil {
			obj.CloseWithError(readErr)
		}
	}()
	select {
	case <-readDone:
		if readErr == nil {
			resp.Body.(*http1.Body).SetWriteDone(writeDone)
		}
		return resp, readErr
	case <-ctx.Done():
		obj.CloseWithError(ctx.Err())
		return nil, ctx.Err()
	case <-obj.ctx.Done():
		obj.CloseWithError(obj.ctx.Err())
		return nil, obj.ctx.Err()
	}
}
