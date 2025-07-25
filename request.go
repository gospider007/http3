package http3

import (
	"context"
	"errors"
	"io"
	"net/http"

	"github.com/gospider007/http1"
)

func sendRequestBody(str *stream, body io.ReadCloser) error {
	defer body.Close()
	_, err := io.CopyBuffer(str, body, make([]byte, bodyCopyBufferSize))
	return err
}

func (obj *Client) sendRequest(req *http.Request, str *stream, orderHeaders []interface {
	Key() string
	Val() any
}) error {
	defer str.Close()
	if err := obj.writeHeaders(str, req, orderHeaders); err != nil {
		return err
	}
	if req.Body != nil {
		return sendRequestBody(str, req.Body)
	}
	return nil
}

func (obj *Client) readResponse(str *stream) (*http.Response, context.Context, error) {
	t, l, err := str.parseNextFrame()
	if err != nil {
		return nil, nil, err
	}
	if t != frameTypeHeaders {
		return nil, nil, errors.New("not headers Frames")
	}
	headerBlock := make([]byte, l)
	if _, err := io.ReadFull(str.str, headerBlock); err != nil {
		return nil, nil, err
	}
	hfs, err := obj.decoder.DecodeFull(headerBlock)
	if err != nil {
		return nil, nil, err
	}
	res, err := responseFromHeaders(hfs)
	if err != nil {
		return nil, nil, err
	}
	ctx, cnl := context.WithCancelCause(obj.ctx)
	res.Body = http1.NewClientBody(str, obj, cnl, nil)
	return res, ctx, nil
}

func (obj *Client) doRequest(req *http.Request, str *stream, orderHeaders []interface {
	Key() string
	Val() any
}) (*http.Response, context.Context, error) {
	var writeErr error
	var readErr error
	var ctx context.Context
	var resp *http.Response
	writeDone := make(chan struct{})
	readDone := make(chan struct{})
	go func() {
		writeErr = obj.sendRequest(req, str, orderHeaders)
		close(writeDone)
	}()
	go func() {
		resp, ctx, readErr = obj.readResponse(str)
		close(readDone)
	}()
	select {
	case <-writeDone:
		if writeErr != nil {
			return nil, ctx, writeErr
		}
		select {
		case <-readDone:
			return resp, ctx, readErr
		case <-obj.ctx.Done():
			return nil, ctx, obj.ctx.Err()
		case <-req.Context().Done():
			return nil, ctx, req.Context().Err()
		}
	case <-readDone:
		if readErr == nil {
			resp.Body.(*http1.ClientBody).SetWriteDone(writeDone)
		}
		return resp, ctx, readErr
	case <-obj.ctx.Done():
		return nil, ctx, obj.ctx.Err()
	case <-req.Context().Done():
		return nil, ctx, req.Context().Err()
	}
}
