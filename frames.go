package http3

import (
	"io"

	"github.com/quic-go/quic-go/quicvarint"
)

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
