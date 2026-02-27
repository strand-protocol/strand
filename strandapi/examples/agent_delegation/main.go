// Command agent_delegation demonstrates the StrandAPI agent delegation protocol:
// AgentNegotiate → AgentDelegate → AgentResult. An orchestrator agent
// negotiates capabilities with a worker agent, delegates a task, and
// receives the result.
//
// Usage:
//
//	go run ./strandapi/examples/agent_delegation
package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/strand-protocol/strand/strandapi/pkg/client"
	"github.com/strand-protocol/strand/strandapi/pkg/strandbuf"
	"github.com/strand-protocol/strand/strandapi/pkg/protocol"
	"github.com/strand-protocol/strand/strandapi/pkg/server"
)

// workerHandler handles agent delegation requests.
func workerHandler(_ context.Context, msg *protocol.AgentDelegate) (*protocol.AgentResult, error) {
	task := string(msg.TaskPayload)
	fmt.Printf("  [worker] Received task: %q\n", task)
	fmt.Printf("  [worker] Processing...\n")
	time.Sleep(200 * time.Millisecond) // simulate work

	// Produce a result based on the task.
	result := fmt.Sprintf("Completed: %s (processed %d chars)", task, len(task))
	fmt.Printf("  [worker] Done: %q\n", result)

	return &protocol.AgentResult{
		SessionID:     msg.SessionID,
		ResultPayload: []byte(result),
		ErrorCode:     protocol.ErrOK,
		ErrorMsg:      "",
	}, nil
}

// dummyHandler satisfies the Handler interface (not used for delegation).
type dummyHandler struct{}

func (h *dummyHandler) HandleInference(_ context.Context, req *protocol.InferenceRequest) (*protocol.InferenceResponse, error) {
	return &protocol.InferenceResponse{ID: req.ID, Text: "not used"}, nil
}

func main() {
	const addr = "127.0.0.1:6479"

	fmt.Println("=== Strand Protocol — Agent Delegation Demo ===")
	fmt.Println()

	// ---------------------------------------------------------------
	// 1. Start the worker server with an agent handler
	// ---------------------------------------------------------------
	srv := server.New(&dummyHandler{}, server.WithAgentHandler(workerHandler))
	go func() {
		if err := srv.ListenAndServe(addr); err != nil {
			log.Printf("worker server stopped: %v", err)
		}
	}()
	time.Sleep(100 * time.Millisecond)

	fmt.Println("[orchestrator] Worker server started on", addr)
	fmt.Println()

	// ---------------------------------------------------------------
	// 2. Connect the orchestrator client
	// ---------------------------------------------------------------
	c, err := client.Dial(addr)
	if err != nil {
		log.Fatalf("dial: %v", err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// ---------------------------------------------------------------
	// 3. Agent Negotiate — exchange capabilities
	// ---------------------------------------------------------------
	fmt.Println("[orchestrator] Sending AgentNegotiate...")
	neg := &protocol.AgentNegotiate{
		SessionID:    1001,
		Capabilities: []string{"text_gen", "code_gen", "tool_use"},
		Version:      1,
	}
	buf := strandbuf.NewBuffer(128)
	neg.Encode(buf)
	if err := c.RawSend(ctx, protocol.OpAgentNegotiate, buf.Bytes()); err != nil {
		log.Fatalf("send negotiate: %v", err)
	}

	// Read the negotiate response.
	opcode, payload, err := c.RawRecv(ctx)
	if err != nil {
		log.Fatalf("recv negotiate: %v", err)
	}
	if opcode == protocol.OpAgentNegotiate {
		resp := &protocol.AgentNegotiate{}
		if err := resp.Decode(strandbuf.NewReader(payload)); err == nil {
			caps := "(none)"
			if len(resp.Capabilities) > 0 {
				caps = strings.Join(resp.Capabilities, ", ")
			}
			fmt.Printf("[orchestrator] Worker capabilities: [%s] (session %d)\n\n", caps, resp.SessionID)
		}
	}

	// ---------------------------------------------------------------
	// 4. Agent Delegate — send a task
	// ---------------------------------------------------------------
	fmt.Println("[orchestrator] Delegating task...")
	del := &protocol.AgentDelegate{
		SessionID:    1001,
		TargetNodeID: [16]byte{0xAA, 0xBB, 0xCC, 0xDD},
		TaskPayload:  []byte("Analyze the Strand Protocol architecture and summarize key components"),
		TimeoutMS:    5000,
	}
	buf.Reset()
	del.Encode(buf)
	if err := c.RawSend(ctx, protocol.OpAgentDelegate, buf.Bytes()); err != nil {
		log.Fatalf("send delegate: %v", err)
	}

	// ---------------------------------------------------------------
	// 5. Agent Result — receive the result
	// ---------------------------------------------------------------
	opcode, payload, err = c.RawRecv(ctx)
	if err != nil {
		log.Fatalf("recv result: %v", err)
	}
	if opcode == protocol.OpAgentResult {
		result := &protocol.AgentResult{}
		if err := result.Decode(strandbuf.NewReader(payload)); err == nil {
			fmt.Println()
			fmt.Printf("[orchestrator] Received result (session %d):\n", result.SessionID)
			fmt.Printf("  Payload: %s\n", string(result.ResultPayload))
			fmt.Printf("  Error code: 0x%04X (%s)\n", result.ErrorCode, protocol.ErrCodeNames[result.ErrorCode])
		}
	}

	fmt.Println()
	fmt.Println("=== Agent delegation demo complete ===")
	srv.Stop()
}
