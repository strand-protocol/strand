package server

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/strand-protocol/strand/strandapi/pkg/strandbuf"
	"github.com/strand-protocol/strand/strandapi/pkg/protocol"
	"github.com/strand-protocol/strand/strandapi/pkg/transport"
)

// defaultShutdownTimeout is how long Stop waits for in-flight frames before
// forcibly closing the transport.
const defaultShutdownTimeout = 5 * time.Second

// ServerOption configures a Server.
type ServerOption func(*Server)

// WithStreamHandler registers a StreamHandler for token-streaming inference.
func WithStreamHandler(sh StreamHandler) ServerOption {
	return func(s *Server) {
		s.streamHandler = sh
	}
}

// WithAgentHandler registers a handler for OpAgentDelegate frames. When a
// peer delegates a task to this server the handler is invoked with the decoded
// AgentDelegate and must return an AgentResult (or an error). On error the
// server sends back an AgentResult with ErrInternal and the error string.
func WithAgentHandler(fn func(ctx context.Context, msg *protocol.AgentDelegate) (*protocol.AgentResult, error)) ServerOption {
	return func(s *Server) {
		s.agentHandler = fn
	}
}

// WithShutdownTimeout configures how long Stop waits for in-flight frame
// handlers to finish before forcibly closing the transport.
func WithShutdownTimeout(d time.Duration) ServerOption {
	return func(s *Server) {
		s.shutdownTimeout = d
	}
}

// maxConcurrentFrames limits the number of goroutines processing frames
// simultaneously, preventing goroutine exhaustion under burst traffic.
const maxConcurrentFrames = 1000

// Server listens for StrandAPI frames on an overlay transport, dispatches
// requests to registered handlers, and writes responses back.
type Server struct {
	handler       Handler
	streamHandler StreamHandler
	// agentHandler handles OpAgentDelegate frames (optional).
	agentHandler func(ctx context.Context, msg *protocol.AgentDelegate) (*protocol.AgentResult, error)
	transport        *transport.OverlayTransport
	mu               sync.Mutex
	done             chan struct{}
	shutdownTimeout  time.Duration
	// sem bounds the number of in-flight frame handler goroutines.
	sem chan struct{}
	// wg tracks in-flight frame handlers so Stop can drain gracefully.
	wg sync.WaitGroup
}

// New creates a Server with the given inference handler and options.
func New(handler Handler, opts ...ServerOption) *Server {
	s := &Server{
		handler:         handler,
		done:            make(chan struct{}),
		sem:             make(chan struct{}, maxConcurrentFrames),
		shutdownTimeout: defaultShutdownTimeout,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// ListenAndServe binds to addr and processes incoming StrandAPI frames until
// the server is stopped or a fatal error occurs.
func (s *Server) ListenAndServe(addr string) error {
	t, err := transport.ListenOverlay(addr)
	if err != nil {
		return fmt.Errorf("strandapi server: listen: %w", err)
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
				log.Printf("strandapi server: recv error: %v", err)
				return err
			}
		}
		// Dispatch in a goroutine bounded by the semaphore to prevent
		// goroutine exhaustion under burst traffic.
		select {
		case s.sem <- struct{}{}:
			s.wg.Add(1)
			go func(op byte, pl []byte) {
				defer s.wg.Done()
				defer func() { <-s.sem }()
				s.handleFrame(ctx, op, pl)
			}(opcode, payload)
		default:
			log.Printf("strandapi server: overloaded, dropping frame opcode=0x%02x", opcode)
		}
	}
}

// Stop signals the server to shut down gracefully. It stops accepting new
// frames and waits up to ShutdownTimeout for in-flight handlers to finish.
func (s *Server) Stop() {
	s.mu.Lock()
	select {
	case <-s.done:
		s.mu.Unlock()
		return // already stopped
	default:
		close(s.done)
	}
	s.mu.Unlock()

	// Wait for in-flight frame handlers to complete, bounded by the
	// configured shutdown timeout so we don't block indefinitely.
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		log.Printf("strandapi server: all in-flight handlers drained")
	case <-time.After(s.shutdownTimeout):
		log.Printf("strandapi server: shutdown timeout (%v) exceeded, forcing close", s.shutdownTimeout)
	}
}

// handleFrame dispatches a single StrandAPI frame to the appropriate handler.
func (s *Server) handleFrame(ctx context.Context, opcode byte, payload []byte) {
	switch opcode {
	case protocol.OpInferenceRequest:
		s.handleInference(ctx, payload)
	case protocol.OpHeartbeat:
		s.handleHeartbeat(ctx)
	case protocol.OpAgentNegotiate:
		s.handleAgentNegotiate(ctx, payload)
	case protocol.OpAgentDelegate:
		s.handleAgentDelegate(ctx, payload)
	default:
		log.Printf("strandapi server: unhandled opcode 0x%02x", opcode)
	}
}

func (s *Server) handleInference(ctx context.Context, payload []byte) {
	req := &protocol.InferenceRequest{}
	reader := strandbuf.NewReader(payload)
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

	buf := strandbuf.NewBuffer(256)
	resp.Encode(buf)
	if err := s.transport.Send(ctx, protocol.OpInferenceResponse, buf.Bytes()); err != nil {
		log.Printf("strandapi server: send response error: %v", err)
	}
}

func (s *Server) handleStreamInference(ctx context.Context, req *protocol.InferenceRequest) {
	// Send stream start
	if err := s.transport.Send(ctx, protocol.OpTokenStreamStart, nil); err != nil {
		log.Printf("strandapi server: send stream start error: %v", err)
		return
	}

	sender := &overlayTokenSender{transport: s.transport, ctx: ctx}
	if err := s.streamHandler.HandleTokenStream(ctx, req, sender); err != nil {
		s.sendError(ctx, err.Error())
		return
	}

	// Send stream end
	if err := s.transport.Send(ctx, protocol.OpTokenStreamEnd, nil); err != nil {
		log.Printf("strandapi server: send stream end error: %v", err)
	}
}

// handleAgentNegotiate responds to an AGENT_NEGOTIATE frame by echoing back
// an AGENT_NEGOTIATE with this server's own capabilities (currently empty —
// callers may extend this by wrapping the server).
func (s *Server) handleAgentNegotiate(ctx context.Context, payload []byte) {
	req := &protocol.AgentNegotiate{}
	reader := strandbuf.NewReader(payload)
	if err := req.Decode(reader); err != nil {
		log.Printf("strandapi server: agent negotiate decode error: %v", err)
		return
	}
	// Echo back with the same SessionID so the peer can correlate the reply.
	resp := &protocol.AgentNegotiate{
		SessionID:    req.SessionID,
		Capabilities: []string{}, // Extend via higher-level server wrappers.
		Version:      1,
	}
	buf := strandbuf.NewBuffer(64)
	resp.Encode(buf)
	if err := s.transport.Send(ctx, protocol.OpAgentNegotiate, buf.Bytes()); err != nil {
		log.Printf("strandapi server: send agent negotiate response error: %v", err)
	}
}

// handleAgentDelegate dispatches an AGENT_DELEGATE frame to the registered
// agentHandler. If no handler is registered, it replies with ErrCapabilities.
func (s *Server) handleAgentDelegate(ctx context.Context, payload []byte) {
	req := &protocol.AgentDelegate{}
	reader := strandbuf.NewReader(payload)
	if err := req.Decode(reader); err != nil {
		s.sendAgentResult(ctx, req.SessionID, nil, protocol.ErrInvalidRequest, fmt.Sprintf("decode error: %v", err))
		return
	}

	if s.agentHandler == nil {
		s.sendAgentResult(ctx, req.SessionID, nil, protocol.ErrCapabilities, "no agent handler registered")
		return
	}

	result, err := s.agentHandler(ctx, req)
	if err != nil {
		s.sendAgentResult(ctx, req.SessionID, nil, protocol.ErrInternal, err.Error())
		return
	}

	// Use the result as returned by the handler; set SessionID from request
	// if the handler did not populate it.
	if result.SessionID == 0 {
		result.SessionID = req.SessionID
	}
	buf := strandbuf.NewBuffer(256)
	result.Encode(buf)
	if err := s.transport.Send(ctx, protocol.OpAgentResult, buf.Bytes()); err != nil {
		log.Printf("strandapi server: send agent result error: %v", err)
	}
}

// sendAgentResult is a helper that encodes and sends an AgentResult frame.
func (s *Server) sendAgentResult(ctx context.Context, sessionID uint32, payload []byte, code uint16, msg string) {
	result := &protocol.AgentResult{
		SessionID:     sessionID,
		ResultPayload: payload,
		ErrorCode:     code,
		ErrorMsg:      msg,
	}
	buf := strandbuf.NewBuffer(128)
	result.Encode(buf)
	if err := s.transport.Send(ctx, protocol.OpAgentResult, buf.Bytes()); err != nil {
		log.Printf("strandapi server: send agent result error: %v", err)
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
	buf := strandbuf.NewBuffer(128)
	chunk.Encode(buf)
	return s.transport.Send(s.ctx, protocol.OpTokenStreamChunk, buf.Bytes())
}
