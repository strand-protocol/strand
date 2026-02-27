package transport

import (
	"bytes"
	"context"
	"testing"
	"time"
)

func TestOverlayLoopback(t *testing.T) {
	// Start a listener on a random port.
	listener, err := ListenOverlay("127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenOverlay: %v", err)
	}
	defer listener.Close()

	laddr := listener.LocalAddr().String()

	// Dial the listener.
	sender, err := DialOverlay(laddr)
	if err != nil {
		t.Fatalf("DialOverlay: %v", err)
	}
	defer sender.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Send a frame from sender to listener.
	opcode := byte(0x01)
	payload := []byte("hello nexapi overlay")
	if err := sender.Send(ctx, opcode, payload); err != nil {
		t.Fatalf("Send: %v", err)
	}

	// Receive on the listener side.
	gotOp, gotPayload, err := listener.Recv(ctx)
	if err != nil {
		t.Fatalf("Recv: %v", err)
	}
	if gotOp != opcode {
		t.Errorf("opcode = 0x%02x, want 0x%02x", gotOp, opcode)
	}
	if !bytes.Equal(gotPayload, payload) {
		t.Errorf("payload = %q, want %q", gotPayload, payload)
	}
}

func TestOverlayMultipleFrames(t *testing.T) {
	listener, err := ListenOverlay("127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenOverlay: %v", err)
	}
	defer listener.Close()

	sender, err := DialOverlay(listener.LocalAddr().String())
	if err != nil {
		t.Fatalf("DialOverlay: %v", err)
	}
	defer sender.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Send several frames.
	frames := []struct {
		opcode  byte
		payload []byte
	}{
		{0x01, []byte("first")},
		{0x02, []byte("second")},
		{0x03, []byte{}},
		{0x04, []byte("fourth with more data")},
	}

	for i, f := range frames {
		if err := sender.Send(ctx, f.opcode, f.payload); err != nil {
			t.Fatalf("Send[%d]: %v", i, err)
		}
	}

	for i, want := range frames {
		gotOp, gotPayload, err := listener.Recv(ctx)
		if err != nil {
			t.Fatalf("Recv[%d]: %v", i, err)
		}
		if gotOp != want.opcode {
			t.Errorf("frame[%d] opcode = 0x%02x, want 0x%02x", i, gotOp, want.opcode)
		}
		if !bytes.Equal(gotPayload, want.payload) {
			t.Errorf("frame[%d] payload = %q, want %q", i, gotPayload, want.payload)
		}
	}
}

func TestOverlayEmptyPayload(t *testing.T) {
	listener, err := ListenOverlay("127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenOverlay: %v", err)
	}
	defer listener.Close()

	sender, err := DialOverlay(listener.LocalAddr().String())
	if err != nil {
		t.Fatalf("DialOverlay: %v", err)
	}
	defer sender.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := sender.Send(ctx, 0x08, nil); err != nil {
		t.Fatalf("Send: %v", err)
	}

	gotOp, gotPayload, err := listener.Recv(ctx)
	if err != nil {
		t.Fatalf("Recv: %v", err)
	}
	if gotOp != 0x08 {
		t.Errorf("opcode = 0x%02x, want 0x08", gotOp)
	}
	if len(gotPayload) != 0 {
		t.Errorf("expected empty payload, got %d bytes", len(gotPayload))
	}
}

func TestOverlayClose(t *testing.T) {
	listener, err := ListenOverlay("127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenOverlay: %v", err)
	}

	if err := listener.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Double-close should not error.
	if err := listener.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}

	// Send after close should error.
	ctx := context.Background()
	if err := listener.Send(ctx, 0x01, nil); err != ErrTransportClosed {
		t.Errorf("Send after close: got %v, want ErrTransportClosed", err)
	}
}

func TestOverlayLargePayload(t *testing.T) {
	listener, err := ListenOverlay("127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenOverlay: %v", err)
	}
	defer listener.Close()

	sender, err := DialOverlay(listener.LocalAddr().String())
	if err != nil {
		t.Fatalf("DialOverlay: %v", err)
	}
	defer sender.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Send a reasonably large payload. macOS limits localhost UDP to ~9KB,
	// so we stay within that to avoid "message too long" errors.
	payload := make([]byte, 8000)
	for i := range payload {
		payload[i] = byte(i % 256)
	}

	if err := sender.Send(ctx, 0x06, payload); err != nil {
		t.Fatalf("Send: %v", err)
	}

	gotOp, gotPayload, err := listener.Recv(ctx)
	if err != nil {
		t.Fatalf("Recv: %v", err)
	}
	if gotOp != 0x06 {
		t.Errorf("opcode = 0x%02x, want 0x06", gotOp)
	}
	if !bytes.Equal(gotPayload, payload) {
		t.Errorf("large payload mismatch (got %d bytes, want %d)", len(gotPayload), len(payload))
	}
}
