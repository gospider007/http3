package http3

import (
	"errors"
	"io"
	"net/http"

	"github.com/quic-go/quic-go"
)

func sendRequestBody(str quic.Stream, body io.ReadCloser) error {
	defer body.Close()
	_, err := io.Copy(str, body)
	return err
}

func (obj *client) sendRequest(req *http.Request, str quic.Stream) error {
	defer str.Close()
	var requestGzip bool
	if !obj.opts.DisableCompression && req.Method != "HEAD" && req.Header.Get("Accept-Encoding") == "" && req.Header.Get("Range") == "" {
		requestGzip = true
	}
	if err := obj.WriteRequestHeader(str, req, requestGzip); err != nil {
		return err
	}
	if req.Body != nil {
		return sendRequestBody(str, req.Body)
	}
	return nil
}
func (obj *client) readResponse(req *http.Request, str quic.Stream) (*http.Response, error) {
	defer str.Close()
	frame, err := parseNextFrame(str)
	if err != nil {
		return nil, err
	}
	headFrame, ok := frame.(*headersFrame)
	if !ok {
		return nil, errors.New("not head Frames")
	}
	headerBlock := make([]byte, headFrame.Length)
	if _, err := io.ReadFull(str, headerBlock); err != nil {
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
	connState := obj.conn.ConnectionState().TLS
	res.TLS = &connState
	res.Request = req
	res.Body = str
	return res, nil
}
func (obj *client) doRequest(req *http.Request, str quic.Stream) (*http.Response, error) {
	err := obj.sendRequest(req, str)
	if err != nil {
		return nil, err
	}
	return obj.readResponse(req, str)
}
