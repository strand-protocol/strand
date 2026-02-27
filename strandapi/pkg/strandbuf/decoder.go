package nexbuf

import (
	"encoding/binary"
	"errors"
	"math"
)

var (
	// ErrShortBuffer is returned when the Reader has fewer bytes than required.
	ErrShortBuffer = errors.New("nexbuf: insufficient data in buffer")
)

// Reader provides sequential, zero-copy decoding of NexBuf-encoded data.
type Reader struct {
	data   []byte
	offset int
}

// NewReader wraps an existing byte slice for decoding.
func NewReader(data []byte) *Reader {
	return &Reader{data: data}
}

// Remaining returns the number of unread bytes.
func (r *Reader) Remaining() int {
	return len(r.data) - r.offset
}

// Offset returns the current read position.
func (r *Reader) Offset() int {
	return r.offset
}

// need checks that at least n bytes remain and returns the current offset.
func (r *Reader) need(n int) (int, error) {
	if r.offset+n > len(r.data) {
		return 0, ErrShortBuffer
	}
	off := r.offset
	r.offset += n
	return off, nil
}

// ReadUint8 reads a single byte.
func (r *Reader) ReadUint8() (uint8, error) {
	off, err := r.need(1)
	if err != nil {
		return 0, err
	}
	return r.data[off], nil
}

// ReadUint16 reads a 16-bit unsigned integer in little-endian order.
func (r *Reader) ReadUint16() (uint16, error) {
	off, err := r.need(2)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint16(r.data[off:]), nil
}

// ReadUint32 reads a 32-bit unsigned integer in little-endian order.
func (r *Reader) ReadUint32() (uint32, error) {
	off, err := r.need(4)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(r.data[off:]), nil
}

// ReadUint64 reads a 64-bit unsigned integer in little-endian order.
func (r *Reader) ReadUint64() (uint64, error) {
	off, err := r.need(8)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(r.data[off:]), nil
}

// ReadFloat32 reads a 32-bit IEEE 754 float in little-endian order.
func (r *Reader) ReadFloat32() (float32, error) {
	v, err := r.ReadUint32()
	if err != nil {
		return 0, err
	}
	return math.Float32frombits(v), nil
}

// ReadFloat64 reads a 64-bit IEEE 754 float in little-endian order.
func (r *Reader) ReadFloat64() (float64, error) {
	v, err := r.ReadUint64()
	if err != nil {
		return 0, err
	}
	return math.Float64frombits(v), nil
}

// ReadString reads a length-prefixed UTF-8 string. The returned string holds
// its own copy of the data (safe after the Reader is discarded).
func (r *Reader) ReadString() (string, error) {
	length, err := r.ReadUint32()
	if err != nil {
		return "", err
	}
	off, err := r.need(int(length))
	if err != nil {
		return "", err
	}
	return string(r.data[off : off+int(length)]), nil
}

// ReadBytes reads a length-prefixed byte slice. The returned slice is a
// sub-slice of the Reader's underlying buffer (zero-copy).
func (r *Reader) ReadBytes() ([]byte, error) {
	length, err := r.ReadUint32()
	if err != nil {
		return nil, err
	}
	off, err := r.need(int(length))
	if err != nil {
		return nil, err
	}
	return r.data[off : off+int(length)], nil
}

// ReadList reads a uint32 list element count. The caller must then read
// that many elements sequentially.
func (r *Reader) ReadList() (uint32, error) {
	return r.ReadUint32()
}

// ReadMapLen reads a uint32 map entry count. The caller must then read
// that many key-value pairs sequentially.
func (r *Reader) ReadMapLen() (uint32, error) {
	return r.ReadUint32()
}
