package server

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/nexus-protocol/nexus/nexapi/pkg/nexbuf"
	"github.com/nexus-protocol/nexus/nexapi/pkg/protocol"
	"github.com/nexus-protocol/nexus/nexapi/pkg/transport"
)

// ServerOption configures a Server.
type ServerOption func(*Server)

// WithStreamHandler registers a StreamHandler for token-streaming inference.
func WithStreamHandler(sh StreamHandler) ServerOption {
	return func(s *Server) {
		s.streamHandler = sh
	}
}

// Server listens for NexAPI frames on an overlay transport, dispatches
// requests to registered handlers, and writes responses back.
type Server struct {
	handler       Handler
	streamHandler StreamHandler
	transport     *transport.OverlayTransport
	mu            sync.Mutex
	done          chan struct{}
}

// New creates a Server with the given inference handler and options.
func New(handler Handler, opts ...ServerOption) *Server {
	s := &Server{
		handler: handler,
		done:    make(chan struct{}),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// ListenAndServe binds to addr and processes incoming NexAPI frames until
// the server is stopped or a fatal error occurs.
func (s *Server) ListenAndServe(addr string) error {
	t, err := transport.ListenOverlay(addr)
	if err != nil {
		return fmt.Errorf("nexapi server: listen: %w", err)
	}
	s.transport = t

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		<-s.done
		cancel()
		t.Close()
	}()

	for {
		opcode, payload, err := t.Recv(ctx)
		if err != nil {
			select {
			case <-s.done:
				return nil // graceful shutdown
			default:
				log.Printf("nexapi server: recv error: %v", err)
				return err
			}
		}
		// Dispatch in a goroutine so we can keep receiving.
		go s.handleFrame(ctx, opcode, payload)
	}
}

// Stop signals the server to shut down gracefully.
func (s *Server) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	select {
	case <-s.done:
	default:
		close(s.done)
	}
}

// handleFrame dispatches a single NexAPI frame to the appropriate handler.
func (s *Server) handleFrame(ctx context.Context, opcode byte, payload []byte) {
	switch opcode {
	case protocol.OpInferenceRequest:
		s.handleInference(ctx, payload)
	case protocol.OpHeartbeat:
		s.handleHeartbeat(ctx)
	default:
		log.Printf("nexapi server: unhandled opcode 0x%02x", opcode)
	}
}

func (s *Server) handleInference(ctx context.Context, payload []byte) {
	req := &protocol.InferenceRequest{}
	reader := nexbuf.NewReader(payload)
	if err := req.Decode(reader); err != nil {
		s.sendError(ctx, fmt.Sprintf("decode error: %v", err))
		return
	}

	// Try stream handler first if registered, otherwise fall back to
	// synchronous handler.
	if s.streamHandler != nil {
		s.handleStreamInference(ctx, req)
		return
	}

	if s.handler == nil {
		s.sendError(ctx, "no handler registered")
		return
	}

	resp, err := s.handler.HandleInference(ctx, req)
	if err != nil {
		s.sendError(ctx, err.Error())
		return
	}

	buf := nexbuf.NewBuffer(256)
	resp.Encode(buf)
	if err := s.transport.Send(ctx, protocol.OpInferenceResponse, buf.Bytes()); err != nil {
		log.Printf("nexapi server: send response error: %v", err)
	}
}

func (s *Server) handleStreamInference(ctx context.Context, req *protocol.InferenceRequest) {
	// Send stream start
	if err := s.transport.Send(ctx, protocol.OpTokenStreamStart, nil); err != nil {
		log.Printf("nexapi server: send stream start error: %v", err)
		return
	}

	sender := &overlayTokenSender{transport: s.transport, ctx: ctx}
	if err := s.streamHandler.HandleTokenStream(ctx, req, sender); err != nil {
		s.sendError(ctx, err.Error())
		return
	}

	// Send stream end
	if err := s.transport.Send(ctx, protocol.OpTokenStreamEnd, nil); err != nil {
		log.Printf("nexapi server: send stream end error: %v", err)
	}
}

func (s *Server) handleHeartbeat(ctx context.Context) {
	// Reply with a heartbeat.
	_ = s.transport.Send(ctx, protocol.OpHeartbeat, nil)
}

func (s *Server) sendError(ctx context.Context, msg string) {
	_ = s.transport.Send(ctx, protocol.OpError, []byte(msg))
}

// overlayTokenSender implements TokenSender over an OverlayTransport.
type overlayTokenSender struct {
	transport *transport.OverlayTransport
	ctx       context.Context
}

func (s *overlayTokenSender) Send(chunk *protocol.TokenStreamChunk) error {
	buf := nexbuf.NewBuffer(128)
	chunk.Encode(buf)
	return s.transport.Send(s.ctx, protocol.OpTokenStreamChunk, buf.Bytes())
}
