package http3

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gospider007/tools"
	"github.com/quic-go/qpack"
	"golang.org/x/net/http/httpguts"
)

func (obj *Client) writeHeaders(str *stream, req *http.Request, orderHeaders []interface {
	Key() string
	Val() any
}) error {
	defer obj.encoder.Close()
	defer obj.headerBuf.Reset()
	if err := obj.encodeHeaders(req, orderHeaders); err != nil {
		return err
	}
	b := make([]byte, 0, 128)
	b = (&headersFrame{Length: uint64(obj.headerBuf.Len())}).Append(b)
	if _, err := str.str.Write(b); err != nil {
		return err
	}
	_, err := str.str.Write(obj.headerBuf.Bytes())
	return err
}
func (obj *Client) encodeHeaders(req *http.Request, orderHeaders []interface {
	Key() string
	Val() any
}) error {
	host := req.Host
	if host == "" {
		host = req.URL.Host
	}
	host, err := httpguts.PunycodeHostPort(host)
	if err != nil {
		return err
	}
	var path string
	if req.Method != http.MethodConnect {
		path = req.URL.RequestURI()
		if !validPseudoPath(path) {
			path = strings.TrimPrefix(path, req.URL.Scheme+"://"+host)
		}
	}
	enumerateHeaders := func(replaceF func(name, value string)) {
		gospiderHeaders := [][2]string{}
		f := func(name, value string) {
			gospiderHeaders = append(gospiderHeaders, [2]string{strings.ToLower(name), value})
		}
		f(":method", req.Method)
		f(":authority", host)
		if req.Method != http.MethodConnect {
			f(":scheme", req.URL.Scheme)
			f(":path", path)
		}
		for k, vv := range req.Header {
			switch strings.ToLower(k) {
			case "host", "content-length", "connection", "proxy-connection", "transfer-encoding", "upgrade", "keep-alive":
			case "cookie":
				for _, v := range vv {
					for _, c := range strings.Split(v, "; ") {
						f("cookie", c)
					}
				}
			default:
				for _, v := range vv {
					f(k, v)
				}
			}
		}

		if contentLength, _ := tools.GetContentLength(req); contentLength >= 0 {
			f("content-length", strconv.FormatInt(contentLength, 10))
		}

		for _, kv := range tools.NewHeadersWithH2(orderHeaders, gospiderHeaders) {
			replaceF(kv[0], kv[1])
		}
	}
	enumerateHeaders(func(name, value string) {
		name = strings.ToLower(name)
		obj.encoder.WriteField(qpack.HeaderField{Name: name, Value: value})
	})
	return nil
}
func validPseudoPath(v string) bool {
	return (len(v) > 0 && v[0] == '/') || v == "*"
}

func responseFromHeaders(headerFields []qpack.HeaderField) (*http.Response, error) {
	hdr, err := parseHeaders(headerFields)
	if err != nil {
		return nil, err
	}
	rsp := &http.Response{
		Proto:         "HTTP/3.0",
		ProtoMajor:    3,
		Header:        hdr.Headers,
		ContentLength: hdr.ContentLength,
	}
	status, err := strconv.Atoi(hdr.Status)
	if err != nil {
		return nil, fmt.Errorf("invalid status code: %w", err)
	}
	rsp.StatusCode = status
	rsp.Status = hdr.Status + " " + http.StatusText(status)
	return rsp, nil
}

type header struct {
	Headers       http.Header
	Path          string
	Method        string
	Authority     string
	Scheme        string
	Status        string
	Protocol      string
	ContentLength int64
}

func parseHeaders(headers []qpack.HeaderField) (header, error) {
	hdr := header{Headers: make(http.Header, len(headers))}
	for _, h := range headers {
		h.Name = strings.ToLower(h.Name)
		if h.IsPseudo() {
			switch h.Name {
			case ":path":
				hdr.Path = h.Value
			case ":method":
				hdr.Method = h.Value
			case ":authority":
				hdr.Authority = h.Value
			case ":protocol":
				hdr.Protocol = h.Value
			case ":scheme":
				hdr.Scheme = h.Value
			case ":status":
				hdr.Status = h.Value
			default:
				goto defaultSet
			}
			continue
		}
	defaultSet:
		hdr.Headers.Add(h.Name, h.Value)
		if h.Name == "content-length" {
			cl, err := strconv.ParseInt(h.Value, 10, 64)
			if err != nil {
				return header{}, fmt.Errorf("invalid content length: %w", err)
			}
			hdr.ContentLength = cl
		}
	}
	return hdr, nil
}
