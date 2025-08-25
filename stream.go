package http3

import (
	"errors"
	"io"

	"github.com/quic-go/quic-go/quicvarint"
)

const bodyCopyBufferSize int = 8 * 1024

type stream struct {
	str     io.ReadWriteCloser
	buf     []byte
	readLen int
}

func (obj *stream) Write(b []byte) (int, error) {
	obj.buf = obj.buf[:0]
	obj.buf = (&dataFrame{Length: uint64(len(b))}).Append(obj.buf)
	if _, err := obj.str.Write(obj.buf); err != nil {
		return 0, err
	}
	return obj.str.Write(b)
}
func (obj *stream) Close() error {
	return obj.str.Close()
}
func (obj *stream) CloseWithError(err error) error {
	return obj.str.Close()
}
func (obj *stream) Read(p []byte) (n int, err error) {
	if obj.readLen == 0 {
		t, l, err := obj.parseNextFrame()
		if err != nil {
			return 0, err
		}
		if t != frameTypeData {
			return 0, errors.New("not data Frames")
		}
		obj.readLen = int(l)
		if obj.readLen == 0 {
			obj.Close()
			return 0, io.EOF
		}
	}
	if len(p) > obj.readLen {
		n, err = obj.str.Read(p[:obj.readLen])
	} else {
		n, err = obj.str.Read(p)
	}
	obj.readLen -= n
	return
}

type dataFrame struct {
	Length uint64
}
type headersFrame struct {
	Length uint64
}

func (f *headersFrame) Append(b []byte) []byte {
	b = quicvarint.Append(b, 0x1)
	return quicvarint.Append(b, f.Length)
}

func (f *dataFrame) Append(b []byte) []byte {
	b = quicvarint.Append(b, 0x0)
	return quicvarint.Append(b, f.Length)
}

const (
	frameTypeData    = 0x0
	frameTypeHeaders = 0x1
)

func (obj *stream) parseNextFrame() (uint64, uint64, error) {
	qr := quicvarint.NewReader(obj.str)
	for {
		t, err := quicvarint.Read(qr)
		if err != nil {
			return 0, 0, err
		}
		l, err := quicvarint.Read(qr)
		if err != nil {
			return 0, 0, err
		}
		switch t {
		case frameTypeData, frameTypeHeaders:
			return t, l, nil
		case 0x4:
		case 0x3: // CANCEL_PUSH
		case 0x5: // PUSH_PROMISE
		case 0x7: // GOAWAY
		case 0xd: // MAX_PUSH_ID
		}
		// skip over unknown frames
		if _, err := io.CopyN(io.Discard, qr, int64(l)); err != nil {
			return t, l, err
		}
	}
}
