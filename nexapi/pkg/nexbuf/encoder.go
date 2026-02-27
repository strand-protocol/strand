package nexbuf

import (
	"encoding/binary"
	"math"
)

// Buffer is a growable byte buffer used for NexBuf binary encoding.
// All multi-byte integers are written in little-endian byte order.
type Buffer struct {
	data []byte
}

// NewBuffer returns a Buffer pre-allocated with the given capacity.
func NewBuffer(cap int) *Buffer {
	return &Buffer{data: make([]byte, 0, cap)}
}

// Bytes returns the accumulated encoded bytes.
func (b *Buffer) Bytes() []byte {
	return b.data
}

// Len returns the number of bytes written so far.
func (b *Buffer) Len() int {
	return len(b.data)
}

// Reset clears the buffer for reuse.
func (b *Buffer) Reset() {
	b.data = b.data[:0]
}

// grow ensures room for n additional bytes, returning the write offset.
func (b *Buffer) grow(n int) int {
	off := len(b.data)
	need := off + n
	if need <= cap(b.data) {
		b.data = b.data[:need]
		return off
	}
	newCap := cap(b.data) * 2
	if newCap < need {
		newCap = need
	}
	tmp := make([]byte, need, newCap)
	copy(tmp, b.data)
	b.data = tmp
	return off
}

// WriteUint8 appends a single byte.
func (b *Buffer) WriteUint8(v uint8) {
	off := b.grow(1)
	b.data[off] = v
}

// WriteUint16 appends a 16-bit unsigned integer in little-endian order.
func (b *Buffer) WriteUint16(v uint16) {
	off := b.grow(2)
	binary.LittleEndian.PutUint16(b.data[off:], v)
}

// WriteUint32 appends a 32-bit unsigned integer in little-endian order.
func (b *Buffer) WriteUint32(v uint32) {
	off := b.grow(4)
	binary.LittleEndian.PutUint32(b.data[off:], v)
}

// WriteUint64 appends a 64-bit unsigned integer in little-endian order.
func (b *Buffer) WriteUint64(v uint64) {
	off := b.grow(8)
	binary.LittleEndian.PutUint64(b.data[off:], v)
}

// WriteFloat32 appends a 32-bit IEEE 754 float in little-endian order.
func (b *Buffer) WriteFloat32(v float32) {
	b.WriteUint32(math.Float32bits(v))
}

// WriteFloat64 appends a 64-bit IEEE 754 float in little-endian order.
func (b *Buffer) WriteFloat64(v float64) {
	b.WriteUint64(math.Float64bits(v))
}

// WriteString appends a length-prefixed UTF-8 string (uint32 length + bytes).
func (b *Buffer) WriteString(s string) {
	b.WriteUint32(uint32(len(s)))
	off := b.grow(len(s))
	copy(b.data[off:], s)
}

// WriteBytes appends a length-prefixed byte slice (uint32 length + bytes).
func (b *Buffer) WriteBytes(p []byte) {
	b.WriteUint32(uint32(len(p)))
	off := b.grow(len(p))
	copy(b.data[off:], p)
}

// WriteList writes a uint32 element count header. The caller is responsible
// for encoding each element immediately after this call.
func (b *Buffer) WriteList(count uint32) {
	b.WriteUint32(count)
}

// WriteMapLen writes a uint32 entry count header. The caller is responsible
// for encoding each key-value pair immediately after this call.
func (b *Buffer) WriteMapLen(count uint32) {
	b.WriteUint32(count)
}
