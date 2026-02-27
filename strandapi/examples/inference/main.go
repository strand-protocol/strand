// Command inference is a mock StrandAPI inference server that splits the prompt
// into word-level "tokens" and streams them back one at a time, simulating a
// real LLM token-by-token generation process.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/strand-protocol/strand/strandapi/pkg/protocol"
	"github.com/strand-protocol/strand/strandapi/pkg/server"
)

// mockStreamHandler splits the prompt into words and sends each as a token.
type mockStreamHandler struct{}

func (h *mockStreamHandler) HandleTokenStream(_ context.Context, req *protocol.InferenceRequest, sender server.TokenSender) error {
	words := strings.Fields(req.Prompt)
	for i, word := range words {
		// Add a leading space for all words except the first.
		token := word
		if i > 0 {
			token = " " + word
		}
		chunk := &protocol.TokenStreamChunk{
			RequestID: req.ID,
			SeqNum:    uint32(i),
			Token:     token,
			Logprob:   -0.1 * float32(i+1),
		}
		if err := sender.Send(chunk); err != nil {
			return fmt.Errorf("send token %d: %w", i, err)
		}
	}
	return nil
}

// noopHandler satisfies the Handler interface for the server constructor.
// Synchronous inference falls through to the stream handler when registered.
type noopHandler struct{}

func (h *noopHandler) HandleInference(_ context.Context, _ *protocol.InferenceRequest) (*protocol.InferenceResponse, error) {
	return nil, fmt.Errorf("use streaming mode")
}

func main() {
	addr := "127.0.0.1:6478"
	if len(os.Args) > 1 {
		addr = os.Args[1]
	}

	srv := server.New(
		&noopHandler{},
		server.WithStreamHandler(&mockStreamHandler{}),
	)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		fmt.Println("\nshutting down...")
		srv.Stop()
	}()

	log.Printf("StrandAPI mock inference server listening on %s", addr)
	if err := srv.ListenAndServe(addr); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
