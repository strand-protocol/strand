package protocol

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// Maximum payload size (16 MiB) to prevent allocation of absurd buffers.
const maxPayloadSize = 16 << 20

var (
	// ErrPayloadTooLarge is returned when a frame's declared length exceeds
	// the maximum allowed payload size.
	ErrPayloadTooLarge = errors.New("nexapi: payload exceeds maximum size")
)

// WriteFrame writes a single NexAPI frame to w.
//
// Frame layout:
//
//	[4 bytes] payload length (little-endian uint32)
//	[1 byte]  opcode
//	[N bytes] payload
func WriteFrame(w io.Writer, opcode byte, payload []byte) error {
	// Header: 4-byte length + 1-byte opcode = 5 bytes
	var hdr [5]byte
	binary.LittleEndian.PutUint32(hdr[0:4], uint32(len(payload)))
	hdr[4] = opcode
	if _, err := w.Write(hdr[:]); err != nil {
		return fmt.Errorf("nexapi: write frame header: %w", err)
	}
	if len(payload) > 0 {
		if _, err := w.Write(payload); err != nil {
			return fmt.Errorf("nexapi: write frame payload: %w", err)
		}
	}
	return nil
}

// ReadFrame reads a single NexAPI frame from r, returning the opcode and
// payload. Returns io.EOF when the reader is exhausted cleanly.
func ReadFrame(r io.Reader) (opcode byte, payload []byte, err error) {
	var hdr [5]byte
	if _, err = io.ReadFull(r, hdr[:]); err != nil {
		return 0, nil, err
	}
	length := binary.LittleEndian.Uint32(hdr[0:4])
	opcode = hdr[4]

	if length > maxPayloadSize {
		return 0, nil, ErrPayloadTooLarge
	}

	payload = make([]byte, length)
	if length > 0 {
		if _, err = io.ReadFull(r, payload); err != nil {
			return 0, nil, fmt.Errorf("nexapi: read frame payload: %w", err)
		}
	}
	return opcode, payload, nil
}
