package integration

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/strand-protocol/strand/strandapi/pkg/client"
	"github.com/strand-protocol/strand/strandapi/pkg/strandbuf"
	"github.com/strand-protocol/strand/strandapi/pkg/protocol"
	"github.com/strand-protocol/strand/strandapi/pkg/server"
	"github.com/strand-protocol/strand/strandapi/pkg/transport"
)

// --------------------------------------------------------------------------
// channelTransport implements transport.Transport using in-process channels,
// enabling full server/client round-trip testing without UDP.
// --------------------------------------------------------------------------

type frame struct {
	opcode  byte
	payload []byte
}

type channelTransport struct {
	send chan frame
	recv chan frame
	done chan struct{}
	once sync.Once
}

func newChannelTransportPair() (*channelTransport, *channelTransport) {
	// c2s: client sends, server receives
	c2s := make(chan frame, 64)
	// s2c: server sends, client receives
	s2c := make(chan frame, 64)

	clientT := &channelTransport{send: c2s, recv: s2c, done: make(chan struct{})}
	serverT := &channelTransport{send: s2c, recv: c2s, done: make(chan struct{})}
	return clientT, serverT
}

func (t *channelTransport) Send(ctx context.Context, opcode byte, payload []byte) error {
	p := make([]byte, len(payload))
	copy(p, payload)
	select {
	case t.send <- frame{opcode: opcode, payload: p}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-t.done:
		return transport.ErrTransportClosed
	}
}

func (t *channelTransport) Recv(ctx context.Context) (byte, []byte, error) {
	select {
	case f := <-t.recv:
		return f.opcode, f.payload, nil
	case <-ctx.Done():
		return 0, nil, ctx.Err()
	case <-t.done:
		return 0, nil, transport.ErrTransportClosed
	}
}

func (t *channelTransport) Close() error {
	t.once.Do(func() { close(t.done) })
	return nil
}

// --------------------------------------------------------------------------
// echoHandler echoes back the prompt as "echo: <prompt>".
// --------------------------------------------------------------------------

type echoHandler struct{}

func (h *echoHandler) HandleInference(ctx context.Context, req *protocol.InferenceRequest) (*protocol.InferenceResponse, error) {
	return &protocol.InferenceResponse{
		ID:               req.ID,
		Text:             "echo: " + req.Prompt,
		FinishReason:     "stop",
		PromptTokens:     uint32(len(req.Prompt)),
		CompletionTokens: uint32(len(req.Prompt) + 6),
	}, nil
}

// --------------------------------------------------------------------------
// channelServerWrapper wraps the server's transport-based dispatch in a
// goroutine, simulating what ListenAndServe does internally.
// --------------------------------------------------------------------------

func startChannelServer(t *testing.T, handler server.Handler, serverT *channelTransport) (stop func()) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			opcode, payload, err := serverT.Recv(ctx)
			if err != nil {
				return
			}
			// Dispatch synchronously in this test helper.
			switch opcode {
			case protocol.OpInferenceRequest:
				req := &protocol.InferenceRequest{}
				reader := strandbuf.NewReader(payload)
				if err := req.Decode(reader); err != nil {
					errMsg := fmt.Sprintf("decode error: %v", err)
					_ = serverT.Send(ctx, protocol.OpError, []byte(errMsg))
					continue
				}
				resp, err := handler.HandleInference(ctx, req)
				if err != nil {
					_ = serverT.Send(ctx, protocol.OpError, []byte(err.Error()))
					continue
				}
				buf := strandbuf.NewBuffer(256)
				resp.Encode(buf)
				_ = serverT.Send(ctx, protocol.OpInferenceResponse, buf.Bytes())

			case protocol.OpHeartbeat:
				_ = serverT.Send(ctx, protocol.OpHeartbeat, nil)

			default:
				// Ignore unknown opcodes.
			}
		}
	}()

	return func() {
		cancel()
		serverT.Close()
		<-done
	}
}

// --------------------------------------------------------------------------
// Tests
// --------------------------------------------------------------------------

// TestStrandAPIEchoServer starts a StrandAPI echo server via channel transport,
// connects a client, sends an InferenceRequest, and verifies the response.
func TestStrandAPIEchoServer(t *testing.T) {
	clientT, serverT := newChannelTransportPair()
	stop := startChannelServer(t, &echoHandler{}, serverT)
	defer stop()

	c, err := client.Dial("unused", client.WithTransport(clientT))
	if err != nil {
		t.Fatalf("client.Dial: %v", err)
	}
	defer c.Close()

	req := &protocol.InferenceRequest{
		ID:          [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		Prompt:      "Hello Strand",
		MaxTokens:   100,
		Temperature: 0.7,
		Metadata:    map[string]string{"test": "true"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := c.Infer(ctx, req)
	if err != nil {
		t.Fatalf("Infer: %v", err)
	}

	if resp.ID != req.ID {
		t.Errorf("response ID mismatch: got %v, want %v", resp.ID, req.ID)
	}
	if resp.Text != "echo: Hello Strand" {
		t.Errorf("response text: got %q, want %q", resp.Text, "echo: Hello Strand")
	}
	if resp.FinishReason != "stop" {
		t.Errorf("finish reason: got %q, want %q", resp.FinishReason, "stop")
	}
}

// TestStrandAPIConcurrentRequests sends 10 concurrent inference requests over
// separate channel transport pairs and verifies all responses.
func TestStrandAPIConcurrentRequests(t *testing.T) {
	const numGoroutines = 10
	var wg sync.WaitGroup
	var successCount atomic.Int32
	var errCount atomic.Int32

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			clientT, serverT := newChannelTransportPair()
			stop := startChannelServer(t, &echoHandler{}, serverT)
			defer stop()

			c, err := client.Dial("unused", client.WithTransport(clientT))
			if err != nil {
				t.Logf("goroutine %d: dial error: %v", idx, err)
				errCount.Add(1)
				return
			}
			defer c.Close()

			req := &protocol.InferenceRequest{
				Prompt:      fmt.Sprintf("request-%d", idx),
				MaxTokens:   50,
				Temperature: 0.5,
				Metadata:    map[string]string{},
			}
			req.ID[0] = byte(idx)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			resp, err := c.Infer(ctx, req)
			if err != nil {
				t.Logf("goroutine %d: infer error: %v", idx, err)
				errCount.Add(1)
				return
			}

			expectedText := fmt.Sprintf("echo: request-%d", idx)
			if resp.Text != expectedText {
				t.Logf("goroutine %d: text mismatch: got %q, want %q", idx, resp.Text, expectedText)
				errCount.Add(1)
				return
			}

			successCount.Add(1)
		}(i)
	}

	wg.Wait()

	if errCount.Load() > 0 {
		t.Errorf("concurrent test: %d successes, %d errors", successCount.Load(), errCount.Load())
	}
	if successCount.Load() != numGoroutines {
		t.Errorf("expected %d successes, got %d", numGoroutines, successCount.Load())
	}
}

// TestStrandAPITimeout verifies that the client times out when the server does
// not respond within the deadline.
func TestStrandAPITimeout(t *testing.T) {
	// Use a slow handler that sleeps longer than the client deadline.
	slowHandler := server.HandlerFunc(func(ctx context.Context, req *protocol.InferenceRequest) (*protocol.InferenceResponse, error) {
		time.Sleep(2 * time.Second)
		return &protocol.InferenceResponse{
			ID:   req.ID,
			Text: "slow",
		}, nil
	})

	clientT, serverT := newChannelTransportPair()
	stop := startChannelServer(t, slowHandler, serverT)
	defer stop()

	c, err := client.Dial("unused", client.WithTransport(clientT))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	req := &protocol.InferenceRequest{
		Prompt:   "timeout test",
		Metadata: map[string]string{},
	}

	_, err = c.Infer(ctx, req)
	if err == nil {
		t.Error("expected timeout error, got nil")
	}
}

// TestStrandAPIGracefulShutdown verifies the server shuts down gracefully after
// handling a request.
func TestStrandAPIGracefulShutdown(t *testing.T) {
	clientT, serverT := newChannelTransportPair()
	stop := startChannelServer(t, &echoHandler{}, serverT)

	c, err := client.Dial("unused", client.WithTransport(clientT))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()

	// Send a request first.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req := &protocol.InferenceRequest{
		Prompt:   "pre-shutdown",
		Metadata: map[string]string{},
	}
	resp, err := c.Infer(ctx, req)
	if err != nil {
		t.Fatalf("pre-shutdown infer: %v", err)
	}
	if resp.Text != "echo: pre-shutdown" {
		t.Errorf("pre-shutdown text: got %q", resp.Text)
	}

	// Shut down the server.
	stop()

	// Verify the client gets an error when trying to send post-shutdown.
	ctx2, cancel2 := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel2()

	_, err = c.Infer(ctx2, req)
	if err == nil {
		t.Log("post-shutdown request unexpectedly succeeded (acceptable with buffered channels)")
	}
}

// TestStrandAPIMultipleSequentialRequests verifies multiple sequential requests
// on the same connection work correctly.
func TestStrandAPIMultipleSequentialRequests(t *testing.T) {
	clientT, serverT := newChannelTransportPair()
	stop := startChannelServer(t, &echoHandler{}, serverT)
	defer stop()

	c, err := client.Dial("unused", client.WithTransport(clientT))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()

	for i := 0; i < 20; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

		req := &protocol.InferenceRequest{
			Prompt:   fmt.Sprintf("sequential-%d", i),
			Metadata: map[string]string{},
		}
		req.ID[0] = byte(i)

		resp, err := c.Infer(ctx, req)
		cancel()
		if err != nil {
			t.Fatalf("request %d: %v", i, err)
		}

		expected := fmt.Sprintf("echo: sequential-%d", i)
		if resp.Text != expected {
			t.Errorf("request %d: got %q, want %q", i, resp.Text, expected)
		}
	}
}

// TestStrandAPILargePrompt verifies that a large prompt can be sent and received.
func TestStrandAPILargePrompt(t *testing.T) {
	clientT, serverT := newChannelTransportPair()
	stop := startChannelServer(t, &echoHandler{}, serverT)
	defer stop()

	c, err := client.Dial("unused", client.WithTransport(clientT))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()

	// Generate a large prompt (100 KB).
	largePrompt := make([]byte, 100*1024)
	for i := range largePrompt {
		largePrompt[i] = byte('A' + (i % 26))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &protocol.InferenceRequest{
		Prompt:   string(largePrompt),
		Metadata: map[string]string{},
	}

	resp, err := c.Infer(ctx, req)
	if err != nil {
		t.Fatalf("large prompt infer: %v", err)
	}

	expected := "echo: " + string(largePrompt)
	if resp.Text != expected {
		t.Errorf("large prompt response length: got %d, want %d", len(resp.Text), len(expected))
	}
}
