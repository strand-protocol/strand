// Command tool_use demonstrates the StrandAPI tool invocation protocol:
// a model server emits a ToolInvoke during inference, the client handles it
// (e.g. runs a calculator), returns a ToolResult, and the server completes
// the response using the tool output.
//
// Usage:
//
//	go run ./strandapi/examples/tool_use
package main

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/strand-protocol/strand/strandapi/pkg/strandbuf"
	"github.com/strand-protocol/strand/strandapi/pkg/protocol"
	"github.com/strand-protocol/strand/strandapi/pkg/transport"
)

func main() {
	const addr = "127.0.0.1:6480"

	fmt.Println("=== Strand Protocol — Tool Use Demo ===")
	fmt.Println()

	// ---------------------------------------------------------------
	// 1. Start the "model server" that uses tool calls
	// ---------------------------------------------------------------
	srvTransport, err := transport.ListenOverlay(addr)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	defer srvTransport.Close()

	go func() {
		ctx := context.Background()
		// Wait for an inference request.
		opcode, payload, err := srvTransport.Recv(ctx)
		if err != nil {
			log.Printf("server recv: %v", err)
			return
		}
		if opcode != protocol.OpInferenceRequest {
			log.Printf("server: unexpected opcode 0x%02x", opcode)
			return
		}

		req := &protocol.InferenceRequest{}
		if err := req.Decode(strandbuf.NewReader(payload)); err != nil {
			log.Printf("server: decode request: %v", err)
			return
		}
		fmt.Printf("[server] Received request: %q\n", req.Prompt)

		// Step 1: Start streaming some initial text.
		srvTransport.Send(ctx, protocol.OpTokenStreamStart, nil)
		sendToken := func(seq uint32, text string) {
			chunk := &protocol.TokenStreamChunk{RequestID: req.ID, SeqNum: seq, Token: text}
			buf := strandbuf.NewBuffer(64)
			chunk.Encode(buf)
			srvTransport.Send(ctx, protocol.OpTokenStreamChunk, buf.Bytes())
		}
		sendToken(0, "Let me calculate that")
		sendToken(1, " for you. ")
		time.Sleep(100 * time.Millisecond)

		// Step 2: Invoke a tool.
		fmt.Println("[server] Invoking tool: calculator")
		invoke := &protocol.ToolInvoke{
			RequestID: req.ID,
			ToolName:  "calculator",
			Arguments: []byte(`{"expression": "42 * 137"}`),
		}
		buf := strandbuf.NewBuffer(128)
		invoke.Encode(buf)
		srvTransport.Send(ctx, protocol.OpToolInvoke, buf.Bytes())

		// Step 3: Wait for the tool result.
		opcode, payload, err = srvTransport.Recv(ctx)
		if err != nil {
			log.Printf("server: recv tool result: %v", err)
			return
		}
		if opcode == protocol.OpToolResult {
			result := &protocol.ToolResult{}
			if err := result.Decode(strandbuf.NewReader(payload)); err == nil {
				fmt.Printf("[server] Tool result: %s\n", string(result.ResultPayload))
				// Step 4: Continue streaming with the tool output.
				sendToken(2, "The result is ")
				sendToken(3, string(result.ResultPayload))
				sendToken(4, ".")
			}
		}

		srvTransport.Send(ctx, protocol.OpTokenStreamEnd, nil)
		fmt.Println("[server] Stream complete")
	}()

	time.Sleep(100 * time.Millisecond)

	// ---------------------------------------------------------------
	// 2. Client sends inference request and handles tool calls
	// ---------------------------------------------------------------
	clientTransport, err := transport.DialOverlay(addr)
	if err != nil {
		log.Fatalf("dial: %v", err)
	}
	defer clientTransport.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := &protocol.InferenceRequest{
		ID:       [16]byte{0xDE, 0xAD},
		Prompt:   "What is 42 times 137?",
		Metadata: map[string]string{},
	}
	buf := strandbuf.NewBuffer(128)
	req.Encode(buf)
	clientTransport.Send(ctx, protocol.OpInferenceRequest, buf.Bytes())

	fmt.Println("[client] Sent request, awaiting response...")
	fmt.Println()

	var response strings.Builder
	for {
		opcode, payload, err := clientTransport.Recv(ctx)
		if err != nil {
			break
		}

		switch opcode {
		case protocol.OpTokenStreamStart:
			fmt.Print("[client] Streaming: ")

		case protocol.OpTokenStreamChunk:
			chunk := &protocol.TokenStreamChunk{}
			if err := chunk.Decode(strandbuf.NewReader(payload)); err == nil {
				fmt.Print(chunk.Token)
				response.WriteString(chunk.Token)
			}

		case protocol.OpToolInvoke:
			invoke := &protocol.ToolInvoke{}
			if err := invoke.Decode(strandbuf.NewReader(payload)); err == nil {
				fmt.Printf("\n[client] Tool invoked: %s(%s)\n", invoke.ToolName, string(invoke.Arguments))

				// Handle the tool call: simple calculator.
				result := handleCalculator(invoke.Arguments)
				fmt.Printf("[client] Tool result: %s\n", result)
				fmt.Print("[client] Streaming: ")

				tr := &protocol.ToolResult{
					RequestID:     invoke.RequestID,
					ResultPayload: []byte(result),
					ErrorCode:     protocol.ErrOK,
				}
				buf := strandbuf.NewBuffer(64)
				tr.Encode(buf)
				clientTransport.Send(ctx, protocol.OpToolResult, buf.Bytes())
			}

		case protocol.OpTokenStreamEnd:
			fmt.Println()
			fmt.Println()
			fmt.Printf("[client] Full response: %q\n", response.String())
			goto done

		case protocol.OpError:
			fmt.Printf("\n[client] Error: %s\n", string(payload))
			goto done
		}
	}

done:
	fmt.Println()
	fmt.Println("=== Tool use demo complete ===")
}

// handleCalculator is a mock tool that evaluates simple "A * B" expressions.
func handleCalculator(args []byte) string {
	// Very simple parser for {"expression": "42 * 137"}
	s := string(args)
	if idx := strings.Index(s, "\"expression\""); idx >= 0 {
		// Extract the expression value.
		rest := s[idx+len("\"expression\""):]
		if start := strings.Index(rest, "\""); start >= 0 {
			rest = rest[start+1:]
			if end := strings.Index(rest, "\""); end >= 0 {
				expr := rest[:end]
				parts := strings.Split(expr, "*")
				if len(parts) == 2 {
					a, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
					b, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
					return strconv.Itoa(a * b)
				}
			}
		}
	}
	return "error: could not evaluate"
}
