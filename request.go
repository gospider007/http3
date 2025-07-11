package http3

import (
	"errors"
	"io"
	"net/http"
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

func (obj *Client) readResponse(str *stream) (*http.Response, error) {
	defer str.Close()
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
	res.Body = str
	return res, nil
}

func (obj *Client) doRequest(req *http.Request, str *stream, orderHeaders []interface {
	Key() string
	Val() any
}) (*http.Response, error) {
	var writeErr error
	var readErr error
	var resp *http.Response
	writeDone := make(chan struct{})
	readDone := make(chan struct{})
	go func() {
		writeErr = obj.sendRequest(req, str, orderHeaders)
		close(writeDone)
	}()
	go func() {
		resp, readErr = obj.readResponse(str)
		close(readDone)
	}()
	select {
	case <-writeDone:
		if writeErr != nil {
			return nil, writeErr
		}
		select {
		case <-readDone:
			return resp, readErr
		case <-obj.ctx.Done():
			return nil, obj.ctx.Err()
		case <-req.Context().Done():
			return nil, req.Context().Err()
		}
	case <-readDone:
		return resp, readErr
	case <-obj.ctx.Done():
		return nil, obj.ctx.Err()
	case <-req.Context().Done():
		return nil, req.Context().Err()
	}
}
