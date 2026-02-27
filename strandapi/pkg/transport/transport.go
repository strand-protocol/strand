// Package transport defines the Transport interface used by StrandAPI clients
// and servers to exchange framed messages, along with concrete implementations
// for the pure-Go UDP overlay mode.
package transport

import "context"

// Transport is the abstract message-level transport used by the StrandAPI client
// and server. Each call to Send/Recv operates on one complete StrandAPI frame
// (opcode + payload). Implementations handle framing, buffering, and
// connection management internally.
type Transport interface {
	// Send transmits a single StrandAPI frame identified by opcode with the
	// given payload. The context may carry deadlines or cancellation.
	Send(ctx context.Context, opcode byte, payload []byte) error

	// Recv blocks until a complete StrandAPI frame arrives. It returns the
	// opcode and payload. The context may carry deadlines or cancellation.
	Recv(ctx context.Context) (opcode byte, payload []byte, err error)

	// Close shuts down the transport, releasing all resources. It is safe
	// to call Close concurrently with Send/Recv; blocked operations will
	// return an error.
	Close() error
}
