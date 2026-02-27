// Package client provides the high-level StrandAPI client SDK. It wraps a
// Transport with typed methods for inference, streaming, and tensor transfer.
package client

import (
	"context"
	"fmt"
	"sync"

	"github.com/strand-protocol/strand/strandapi/pkg/strandbuf"
	"github.com/strand-protocol/strand/strandapi/pkg/protocol"
	"github.com/strand-protocol/strand/strandapi/pkg/transport"
)

// Option configures a Client during construction.
type Option func(*Client)

// WithTransport overrides the transport used by the Client. This is useful
// for testing or when a custom transport is needed.
func WithTransport(t transport.Transport) Option {
	return func(c *Client) {
		c.transport = t
	}
}

// Client is the primary entry point for StrandAPI consumers. It manages the
// underlying transport and provides typed helpers for every StrandAPI operation.
type Client struct {
	transport transport.Transport
	mu        sync.Mutex
	closed    bool
}

// Dial creates a new Client connected to the overlay transport at addr.
func Dial(addr string, opts ...Option) (*Client, error) {
	c := &Client{}
	for _, opt := range opts {
		opt(c)
	}
	if c.transport == nil {
		t, err := transport.DialOverlay(addr)
		if err != nil {
			return nil, fmt.Errorf("strandapi client: dial: %w", err)
		}
		c.transport = t
	}
	return c, nil
}

// Infer sends a synchronous inference request and blocks until the complete
// response arrives. For streaming use StreamTokens instead.
func (c *Client) Infer(ctx context.Context, req *protocol.InferenceRequest) (*protocol.InferenceResponse, error) {
	buf := strandbuf.NewBuffer(256)
	req.Encode(buf)

	if err := c.transport.Send(ctx, protocol.OpInferenceRequest, buf.Bytes()); err != nil {
		return nil, fmt.Errorf("strandapi client: send inference request: %w", err)
	}

	opcode, payload, err := c.transport.Recv(ctx)
	if err != nil {
		return nil, fmt.Errorf("strandapi client: recv inference response: %w", err)
	}
	if opcode == protocol.OpError {
		return nil, fmt.Errorf("strandapi client: server error: %s", string(payload))
	}
	if opcode != protocol.OpInferenceResponse {
		return nil, fmt.Errorf("strandapi client: unexpected opcode 0x%02x, want 0x%02x", opcode, protocol.OpInferenceResponse)
	}

	resp := &protocol.InferenceResponse{}
	reader := strandbuf.NewReader(payload)
	if err := resp.Decode(reader); err != nil {
		return nil, fmt.Errorf("strandapi client: decode inference response: %w", err)
	}
	return resp, nil
}

// StreamTokens sends a streaming inference request and returns a channel that
// yields TokenStreamChunk messages as they arrive. The channel is closed when
// the stream ends (OpTokenStreamEnd) or an error occurs.
func (c *Client) StreamTokens(ctx context.Context, req *protocol.InferenceRequest) (<-chan *protocol.TokenStreamChunk, error) {
	buf := strandbuf.NewBuffer(256)
	req.Encode(buf)

	if err := c.transport.Send(ctx, protocol.OpInferenceRequest, buf.Bytes()); err != nil {
		return nil, fmt.Errorf("strandapi client: send stream request: %w", err)
	}

	ch := make(chan *protocol.TokenStreamChunk, 64)
	go func() {
		defer close(ch)
		for {
			opcode, payload, err := c.transport.Recv(ctx)
			if err != nil {
				return
			}
			switch opcode {
			case protocol.OpTokenStreamStart:
				// Stream has started; continue reading chunks.
				continue
			case protocol.OpTokenStreamChunk:
				chunk := &protocol.TokenStreamChunk{}
				reader := strandbuf.NewReader(payload)
				if err := chunk.Decode(reader); err != nil {
					return
				}
				select {
				case ch <- chunk:
				case <-ctx.Done():
					return
				}
			case protocol.OpTokenStreamEnd:
				return
			case protocol.OpError:
				return
			default:
				// Unexpected opcode -- ignore and keep reading.
				continue
			}
		}
	}()

	return ch, nil
}

// RawSend transmits a single StrandAPI frame with the given opcode and payload.
// Use this for protocol messages not covered by the typed helpers (e.g. agent
// delegation, tool invocation, health checks).
func (c *Client) RawSend(ctx context.Context, opcode byte, payload []byte) error {
	return c.transport.Send(ctx, opcode, payload)
}

// RawRecv blocks until a complete StrandAPI frame arrives and returns the raw
// opcode and payload. Use this for protocol messages not covered by the typed
// helpers.
func (c *Client) RawRecv(ctx context.Context) (byte, []byte, error) {
	return c.transport.Recv(ctx)
}

// Close shuts down the client transport.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	return c.transport.Close()
}
