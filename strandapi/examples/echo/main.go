// Command echo is a StrandAPI echo server. It receives inference requests and
// returns the prompt text verbatim as the response, useful for testing the
// protocol round-trip.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/strand-protocol/strand/strandapi/pkg/protocol"
	"github.com/strand-protocol/strand/strandapi/pkg/server"
)

// echoHandler returns the prompt text back as the inference result.
type echoHandler struct{}

func (h *echoHandler) HandleInference(_ context.Context, req *protocol.InferenceRequest) (*protocol.InferenceResponse, error) {
	return &protocol.InferenceResponse{
		ID:               req.ID,
		Text:             req.Prompt,
		FinishReason:     "stop",
		PromptTokens:     uint32(len(req.Prompt)),
		CompletionTokens: uint32(len(req.Prompt)),
	}, nil
}

func main() {
	addr := "127.0.0.1:6477"
	if len(os.Args) > 1 {
		addr = os.Args[1]
	}

	srv := server.New(&echoHandler{})

	// Graceful shutdown on SIGINT/SIGTERM.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		fmt.Println("\nshutting down...")
		srv.Stop()
	}()

	log.Printf("StrandAPI echo server listening on %s", addr)
	if err := srv.ListenAndServe(addr); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
