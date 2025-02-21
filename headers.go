package http3

import (
	"fmt"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/quic-go/qpack"
	"golang.org/x/net/http/httpguts"
	"golang.org/x/net/http2/hpack"
)

func (obj *Client) writeHeaders(str *stream, req *http.Request, orderHeaders []string) error {
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
func (obj *Client) encodeHeaders(req *http.Request, orderHeaders []string) error {
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
		f(":authority", host)
		f(":method", req.Method)
		if req.Method != http.MethodConnect {
			f(":path", path)
			f(":scheme", req.URL.Scheme)
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
		if contentLength := actualContentLength(req); shouldSendReqContentLength(req.Method, contentLength) {
			f("content-length", strconv.FormatInt(contentLength, 10))
		}
		sort.Slice(gospiderHeaders, func(x, y int) bool {
			xI := slices.Index(orderHeaders, gospiderHeaders[x][0])
			yI := slices.Index(orderHeaders, gospiderHeaders[y][0])
			if xI < 0 {
				return false
			}
			if yI < 0 {
				return true
			}
			if xI <= yI {
				return true
			}
			return false
		})
		for _, kv := range gospiderHeaders {
			replaceF(kv[0], kv[1])
		}
	}
	hlSize := uint64(0)
	enumerateHeaders(func(name, value string) {
		hf := hpack.HeaderField{Name: name, Value: value}
		hlSize += uint64(hf.Size())
	})
	enumerateHeaders(func(name, value string) {
		name = strings.ToLower(name)
		obj.encoder.WriteField(qpack.HeaderField{Name: name, Value: value})
	})
	return nil
}
func validPseudoPath(v string) bool {
	return (len(v) > 0 && v[0] == '/') || v == "*"
}
func actualContentLength(req *http.Request) int64 {
	if req.Body == nil {
		return 0
	}
	if req.ContentLength != 0 {
		return req.ContentLength
	}
	return -1
}

// shouldSendReqContentLength reports whether the http2.Transport should send
// a "content-length" request header. This logic is basically a copy of the net/http
// transferWriter.shouldSendContentLength.
// The contentLength is the corrected contentLength (so 0 means actually 0, not unknown).
// -1 means unknown.
func shouldSendReqContentLength(method string, contentLength int64) bool {
	if contentLength > 0 {
		return true
	}
	if contentLength < 0 {
		return false
	}
	// For zero bodies, whether we send a content-length depends on the method.
	// It also kinda doesn't matter for http2 either way, with END_STREAM.
	switch method {
	case "POST", "PUT", "PATCH":
		return true
	default:
		return false
	}
}
func responseFromHeaders(headerFields []qpack.HeaderField) (*http.Response, error) {
	hdr, err := parseHeaders(headerFields, false)
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
	// all non-pseudo headers
	Headers http.Header
	// Pseudo header fields defined in RFC 9114
	Path      string
	Method    string
	Authority string
	Scheme    string
	Status    string
	// for Extended connect
	Protocol string
	// parsed and deduplicated
	ContentLength int64
}

func parseHeaders(headers []qpack.HeaderField, isRequest bool) (header, error) {
	hdr := header{Headers: make(http.Header, len(headers))}
	for _, h := range headers {
		h.Name = strings.ToLower(h.Name)
		if h.IsPseudo() {
			var isResponsePseudoHeader bool // pseudo headers are either valid for requests or for responses
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
				isResponsePseudoHeader = true
			default:
				return header{}, fmt.Errorf("unknown pseudo header: %s", h.Name)
			}
			if isRequest && isResponsePseudoHeader {
				return header{}, fmt.Errorf("invalid request pseudo header: %s", h.Name)
			}
			if !isRequest && !isResponsePseudoHeader {
				return header{}, fmt.Errorf("invalid response pseudo header: %s", h.Name)
			}
		} else {
			hdr.Headers.Add(h.Name, h.Value)
			if h.Name == "content-length" {
				cl, err := strconv.ParseInt(h.Value, 10, 64)
				if err != nil {
					return header{}, fmt.Errorf("invalid content length: %w", err)
				}
				hdr.ContentLength = cl
			}
		}
	}
	return hdr, nil
}
