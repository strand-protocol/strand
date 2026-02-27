// Command e2e_demo demonstrates the full Strand Protocol MVP flow:
// a Go client sends an InferenceRequest to a Go server over the StrandAPI
// overlay transport, receives streamed tokens, and prints the assembled
// response. This is the "MVP demo flow" described in CLAUDE.md §5.
//
// Usage:
//
//	go run ./strandapi/examples/e2e_demo
package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/strand-protocol/strand/strandapi/pkg/client"
	"github.com/strand-protocol/strand/strandapi/pkg/protocol"
	"github.com/strand-protocol/strand/strandapi/pkg/sad"
	"github.com/strand-protocol/strand/strandapi/pkg/server"
)

// mockStreamHandler splits the prompt into words and streams them as tokens.
type mockStreamHandler struct{}

func (h *mockStreamHandler) HandleTokenStream(_ context.Context, req *protocol.InferenceRequest, sender server.TokenSender) error {
	// Simulate model inference: echo back the prompt word by word.
	words := strings.Fields(req.Prompt)
	for i, word := range words {
		token := word
		if i > 0 {
			token = " " + word
		}
		if err := sender.Send(&protocol.TokenStreamChunk{
			RequestID: req.ID,
			SeqNum:    uint32(i),
			Token:     token,
			Logprob:   -0.05 * float32(i+1),
		}); err != nil {
			return err
		}
		time.Sleep(50 * time.Millisecond) // simulate generation delay
	}
	return nil
}

func main() {
	const addr = "127.0.0.1:6477"

	// ---------------------------------------------------------------
	// 1. Start the StrandAPI server
	// ---------------------------------------------------------------
	srv := server.New(nil, server.WithStreamHandler(&mockStreamHandler{}))
	go func() {
		if err := srv.ListenAndServe(addr); err != nil {
			log.Printf("server stopped: %v", err)
		}
	}()
	time.Sleep(100 * time.Millisecond) // let the server bind

	fmt.Println("=== Strand Protocol E2E Demo ===")
	fmt.Println()

	// ---------------------------------------------------------------
	// 2. Build a Semantic Address Descriptor (SAD) for the request
	// ---------------------------------------------------------------
	modelSAD, err := sad.NewSADBuilder().
		ModelType("llm").
		WithCapability(sad.TextGen).
		WithCapability(sad.CodeGen).
		ContextWindow(128000).
		LatencySLA(500).
		Build()
	if err != nil {
		log.Fatalf("build SAD: %v", err)
	}
	fmt.Printf("Model selector (SAD): %d bytes\n", len(modelSAD))

	// ---------------------------------------------------------------
	// 3. Connect the client
	// ---------------------------------------------------------------
	c, err := client.Dial(addr)
	if err != nil {
		log.Fatalf("dial: %v", err)
	}
	defer c.Close()

	fmt.Printf("Connected to %s\n\n", addr)

	// ---------------------------------------------------------------
	// 4. Send InferenceRequest with streaming tokens
	// ---------------------------------------------------------------
	req := &protocol.InferenceRequest{
		ID:          [16]byte{0x01, 0x02, 0x03, 0x04},
		ModelSAD:    modelSAD,
		Prompt:      "The Strand Protocol replaces TCP/IP with an AI-native networking stack",
		MaxTokens:   512,
		Temperature: 0.7,
		Metadata:    map[string]string{"demo": "e2e"},
	}

	fmt.Printf("Prompt: %q\n\n", req.Prompt)
	fmt.Print("Streaming response: ")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tokenCh, err := c.StreamTokens(ctx, req)
	if err != nil {
		log.Fatalf("stream tokens: %v", err)
	}

	var assembled strings.Builder
	tokenCount := 0
	for chunk := range tokenCh {
		assembled.WriteString(chunk.Token)
		fmt.Print(chunk.Token)
		tokenCount++
	}
	fmt.Println()
	fmt.Println()

	// ---------------------------------------------------------------
	// 5. Print results
	// ---------------------------------------------------------------
	fmt.Printf("Assembled response: %q\n", assembled.String())
	fmt.Printf("Total tokens: %d\n", tokenCount)
	fmt.Println()
	fmt.Println("=== Demo complete ===")

	// ---------------------------------------------------------------
	// 6. Shut down
	// ---------------------------------------------------------------
	srv.Stop()
}
