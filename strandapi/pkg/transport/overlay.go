package transport

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

// Overlay transport wire constants.
const (
	OverlayMagic   uint16 = 0x504C // "PL"
	OverlayVersion byte   = 1
	overlayHdrSize        = 8 // 2B magic + 1B version + 1B flags + 4B length
	maxUDPPayload         = 65507
)

var (
	ErrInvalidMagic   = errors.New("strandapi overlay: invalid magic bytes")
	ErrVersionMismatch = errors.New("strandapi overlay: unsupported version")
	ErrMessageTooLarge = errors.New("strandapi overlay: message exceeds maximum UDP payload")
	ErrTransportClosed = errors.New("strandapi overlay: transport is closed")
)

// OverlayTransport is a pure-Go transport that frames StrandAPI messages over
// UDP. It requires no CGo, no StrandLink, and no StrandStream -- it exists to
// provide full StrandAPI functionality with zero native dependencies.
//
// Frame layout on the wire:
//
//	[2B magic 0x504C][1B version][1B flags][4B length][1B opcode][payload...]
type OverlayTransport struct {
	conn   *net.UDPConn
	remote *net.UDPAddr // non-nil for client (dialled) connections
	mu     sync.Mutex
	closed bool
}

// DialOverlay connects to a remote StrandAPI overlay endpoint.
func DialOverlay(addr string) (*OverlayTransport, error) {
	raddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, fmt.Errorf("strandapi overlay: resolve %s: %w", addr, err)
	}
	conn, err := net.DialUDP("udp", nil, raddr)
	if err != nil {
		return nil, fmt.Errorf("strandapi overlay: dial %s: %w", addr, err)
	}
	return &OverlayTransport{conn: conn, remote: raddr}, nil
}

// ListenOverlay creates a listening overlay transport bound to addr.
func ListenOverlay(addr string) (*OverlayTransport, error) {
	laddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, fmt.Errorf("strandapi overlay: resolve %s: %w", addr, err)
	}
	conn, err := net.ListenUDP("udp", laddr)
	if err != nil {
		return nil, fmt.Errorf("strandapi overlay: listen %s: %w", addr, err)
	}
	return &OverlayTransport{conn: conn}, nil
}

// Send transmits a single StrandAPI frame over the overlay.
func (t *OverlayTransport) Send(ctx context.Context, opcode byte, payload []byte) error {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return ErrTransportClosed
	}
	t.mu.Unlock()

	// Total wire frame: header + 1B opcode + payload
	totalLen := overlayHdrSize + 1 + len(payload)
	if totalLen > maxUDPPayload {
		return ErrMessageTooLarge
	}

	frame := make([]byte, totalLen)
	// Magic
	binary.BigEndian.PutUint16(frame[0:2], OverlayMagic)
	// Version
	frame[2] = OverlayVersion
	// Flags (reserved)
	frame[3] = 0
	// Length of (opcode + payload)
	binary.LittleEndian.PutUint32(frame[4:8], uint32(1+len(payload)))
	// Opcode
	frame[8] = opcode
	// Payload
	copy(frame[9:], payload)

	// Respect context deadline.
	if deadline, ok := ctx.Deadline(); ok {
		if err := t.conn.SetWriteDeadline(deadline); err != nil {
			return err
		}
	}

	_, err := t.conn.Write(frame)
	return err
}

// Recv blocks until a complete StrandAPI overlay frame arrives.
func (t *OverlayTransport) Recv(ctx context.Context) (byte, []byte, error) {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return 0, nil, ErrTransportClosed
	}
	t.mu.Unlock()

	// Return immediately if the context is already done.
	if err := ctx.Err(); err != nil {
		return 0, nil, err
	}

	buf := make([]byte, maxUDPPayload)

	// Respect context deadline.
	if deadline, ok := ctx.Deadline(); ok {
		if err := t.conn.SetReadDeadline(deadline); err != nil {
			return 0, nil, err
		}
	}

	// Monitor context cancellation. When ctx is cancelled (with or without a
	// deadline), set an expired read deadline so ReadFromUDP unblocks promptly.
	// The goroutine exits cleanly when the read finishes normally.
	readDone := make(chan struct{})
	defer close(readDone)
	go func() {
		select {
		case <-ctx.Done():
			_ = t.conn.SetReadDeadline(time.Now())
		case <-readDone:
		}
	}()

	n, remoteAddr, err := t.conn.ReadFromUDP(buf)
	if err != nil {
		return 0, nil, err
	}
	if n < overlayHdrSize+1 {
		return 0, nil, fmt.Errorf("strandapi overlay: frame too short (%d bytes)", n)
	}

	// Save the remote address for listener-mode transports so that
	// subsequent Send calls know where to reply.
	if t.remote == nil && remoteAddr != nil {
		t.remote = remoteAddr
	}

	// Validate magic
	magic := binary.BigEndian.Uint16(buf[0:2])
	if magic != OverlayMagic {
		return 0, nil, ErrInvalidMagic
	}

	// Validate version
	if buf[2] != OverlayVersion {
		return 0, nil, ErrVersionMismatch
	}

	// Parse length
	length := binary.LittleEndian.Uint32(buf[4:8])
	if overlayHdrSize+int(length) > n {
		return 0, nil, fmt.Errorf("strandapi overlay: declared length %d exceeds received %d", length, n-overlayHdrSize)
	}

	opcode := buf[8]
	payload := make([]byte, length-1)
	copy(payload, buf[9:9+length-1])

	return opcode, payload, nil
}

// Close shuts down the overlay transport.
func (t *OverlayTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return nil
	}
	t.closed = true
	return t.conn.Close()
}

// LocalAddr returns the local network address of the underlying connection.
func (t *OverlayTransport) LocalAddr() net.Addr {
	return t.conn.LocalAddr()
}
