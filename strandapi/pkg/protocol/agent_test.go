package protocol

import (
	"bytes"
	"testing"

	"github.com/nexus-protocol/nexus/nexapi/pkg/nexbuf"
)

// ---------------------------------------------------------------------------
// AgentNegotiate
// ---------------------------------------------------------------------------

func TestAgentNegotiateRoundTrip(t *testing.T) {
	orig := &AgentNegotiate{
		SessionID:    0xDEADBEEF,
		Capabilities: []string{"inference", "tensor-transfer", "stream"},
		Version:      1,
	}

	buf := nexbuf.NewBuffer(128)
	orig.Encode(buf)

	decoded := &AgentNegotiate{}
	reader := nexbuf.NewReader(buf.Bytes())
	if err := decoded.Decode(reader); err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if orig.SessionID != decoded.SessionID {
		t.Errorf("SessionID: %d != %d", orig.SessionID, decoded.SessionID)
	}
	if len(orig.Capabilities) != len(decoded.Capabilities) {
		t.Fatalf("Capabilities len: %d != %d", len(orig.Capabilities), len(decoded.Capabilities))
	}
	for i, c := range orig.Capabilities {
		if decoded.Capabilities[i] != c {
			t.Errorf("Capabilities[%d]: %q != %q", i, decoded.Capabilities[i], c)
		}
	}
	if orig.Version != decoded.Version {
		t.Errorf("Version: %d != %d", orig.Version, decoded.Version)
	}
}

func TestAgentNegotiateEmptyCapabilities(t *testing.T) {
	orig := &AgentNegotiate{
		SessionID:    1,
		Capabilities: []string{},
		Version:      1,
	}

	buf := nexbuf.NewBuffer(32)
	orig.Encode(buf)

	decoded := &AgentNegotiate{}
	reader := nexbuf.NewReader(buf.Bytes())
	if err := decoded.Decode(reader); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(decoded.Capabilities) != 0 {
		t.Errorf("expected empty capabilities, got %v", decoded.Capabilities)
	}
}

func TestAgentNegotiateViaFrame(t *testing.T) {
	orig := &AgentNegotiate{
		SessionID:    42,
		Capabilities: []string{"tool-use"},
		Version:      1,
	}

	// Encode into a NexBuf buffer, then wrap in a protocol frame.
	payload := nexbuf.NewBuffer(64)
	orig.Encode(payload)

	var frameBuf bytes.Buffer
	if err := WriteFrame(&frameBuf, OpAgentNegotiate, payload.Bytes()); err != nil {
		t.Fatalf("WriteFrame: %v", err)
	}

	opcode, raw, err := ReadFrame(&frameBuf)
	if err != nil {
		t.Fatalf("ReadFrame: %v", err)
	}
	if opcode != OpAgentNegotiate {
		t.Errorf("opcode = 0x%02x, want 0x%02x", opcode, OpAgentNegotiate)
	}

	decoded := &AgentNegotiate{}
	if err := decoded.Decode(nexbuf.NewReader(raw)); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if decoded.SessionID != orig.SessionID {
		t.Errorf("SessionID mismatch")
	}
}

// ---------------------------------------------------------------------------
// AgentDelegate
// ---------------------------------------------------------------------------

func TestAgentDelegateRoundTrip(t *testing.T) {
	orig := &AgentDelegate{
		SessionID:    0xCAFEBABE,
		TargetNodeID: [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		TaskPayload:  []byte(`{"model":"gpt-4","prompt":"Translate to French"}`),
		TimeoutMS:    5000,
	}

	buf := nexbuf.NewBuffer(128)
	orig.Encode(buf)

	decoded := &AgentDelegate{}
	reader := nexbuf.NewReader(buf.Bytes())
	if err := decoded.Decode(reader); err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if orig.SessionID != decoded.SessionID {
		t.Errorf("SessionID: %d != %d", orig.SessionID, decoded.SessionID)
	}
	if orig.TargetNodeID != decoded.TargetNodeID {
		t.Errorf("TargetNodeID mismatch: %v != %v", orig.TargetNodeID, decoded.TargetNodeID)
	}
	if !bytes.Equal(orig.TaskPayload, decoded.TaskPayload) {
		t.Errorf("TaskPayload mismatch")
	}
	if orig.TimeoutMS != decoded.TimeoutMS {
		t.Errorf("TimeoutMS: %d != %d", orig.TimeoutMS, decoded.TimeoutMS)
	}
}

func TestAgentDelegateEmptyPayload(t *testing.T) {
	orig := &AgentDelegate{
		SessionID:    7,
		TargetNodeID: [16]byte{0xFF},
		TaskPayload:  []byte{},
		TimeoutMS:    0,
	}

	buf := nexbuf.NewBuffer(64)
	orig.Encode(buf)

	decoded := &AgentDelegate{}
	if err := decoded.Decode(nexbuf.NewReader(buf.Bytes())); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(decoded.TaskPayload) != 0 {
		t.Errorf("expected empty TaskPayload, got %d bytes", len(decoded.TaskPayload))
	}
	if decoded.TimeoutMS != 0 {
		t.Errorf("TimeoutMS: %d != 0", decoded.TimeoutMS)
	}
}

func TestAgentDelegateViaFrame(t *testing.T) {
	orig := &AgentDelegate{
		SessionID:    99,
		TargetNodeID: [16]byte{0xAB},
		TaskPayload:  []byte("hello"),
		TimeoutMS:    1000,
	}

	payload := nexbuf.NewBuffer(128)
	orig.Encode(payload)

	var frameBuf bytes.Buffer
	if err := WriteFrame(&frameBuf, OpAgentDelegate, payload.Bytes()); err != nil {
		t.Fatalf("WriteFrame: %v", err)
	}

	opcode, raw, err := ReadFrame(&frameBuf)
	if err != nil {
		t.Fatalf("ReadFrame: %v", err)
	}
	if opcode != OpAgentDelegate {
		t.Errorf("opcode = 0x%02x, want 0x%02x", opcode, OpAgentDelegate)
	}

	decoded := &AgentDelegate{}
	if err := decoded.Decode(nexbuf.NewReader(raw)); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if !bytes.Equal(orig.TaskPayload, decoded.TaskPayload) {
		t.Errorf("TaskPayload mismatch after frame round-trip")
	}
}

// ---------------------------------------------------------------------------
// AgentResult
// ---------------------------------------------------------------------------

func TestAgentResultRoundTripSuccess(t *testing.T) {
	orig := &AgentResult{
		SessionID:     0xCAFEBABE,
		ResultPayload: []byte(`{"translated":"Bonjour le monde"}`),
		ErrorCode:     ErrOK,
		ErrorMsg:      "",
	}

	buf := nexbuf.NewBuffer(128)
	orig.Encode(buf)

	decoded := &AgentResult{}
	if err := decoded.Decode(nexbuf.NewReader(buf.Bytes())); err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if orig.SessionID != decoded.SessionID {
		t.Errorf("SessionID: %d != %d", orig.SessionID, decoded.SessionID)
	}
	if !bytes.Equal(orig.ResultPayload, decoded.ResultPayload) {
		t.Errorf("ResultPayload mismatch")
	}
	if orig.ErrorCode != decoded.ErrorCode {
		t.Errorf("ErrorCode: 0x%04x != 0x%04x", orig.ErrorCode, decoded.ErrorCode)
	}
	if orig.ErrorMsg != decoded.ErrorMsg {
		t.Errorf("ErrorMsg: %q != %q", orig.ErrorMsg, decoded.ErrorMsg)
	}
}

func TestAgentResultRoundTripError(t *testing.T) {
	orig := &AgentResult{
		SessionID:     1,
		ResultPayload: nil,
		ErrorCode:     ErrTimeout,
		ErrorMsg:      "upstream model timed out after 5000ms",
	}

	buf := nexbuf.NewBuffer(64)
	orig.Encode(buf)

	decoded := &AgentResult{}
	if err := decoded.Decode(nexbuf.NewReader(buf.Bytes())); err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if decoded.ErrorCode != ErrTimeout {
		t.Errorf("ErrorCode: 0x%04x != 0x%04x", decoded.ErrorCode, ErrTimeout)
	}
	if decoded.ErrorMsg != orig.ErrorMsg {
		t.Errorf("ErrorMsg: %q != %q", decoded.ErrorMsg, orig.ErrorMsg)
	}
	// nil payload encodes as zero-length; decoded should be empty, not nil.
	if len(decoded.ResultPayload) != 0 {
		t.Errorf("expected empty ResultPayload, got %d bytes", len(decoded.ResultPayload))
	}
}

func TestAgentResultViaFrame(t *testing.T) {
	orig := &AgentResult{
		SessionID:     5,
		ResultPayload: []byte{0x01, 0x02, 0x03},
		ErrorCode:     ErrOK,
		ErrorMsg:      "",
	}

	payload := nexbuf.NewBuffer(64)
	orig.Encode(payload)

	var frameBuf bytes.Buffer
	if err := WriteFrame(&frameBuf, OpAgentResult, payload.Bytes()); err != nil {
		t.Fatalf("WriteFrame: %v", err)
	}

	opcode, raw, err := ReadFrame(&frameBuf)
	if err != nil {
		t.Fatalf("ReadFrame: %v", err)
	}
	if opcode != OpAgentResult {
		t.Errorf("opcode = 0x%02x, want 0x%02x", opcode, OpAgentResult)
	}

	decoded := &AgentResult{}
	if err := decoded.Decode(nexbuf.NewReader(raw)); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if !bytes.Equal(orig.ResultPayload, decoded.ResultPayload) {
		t.Errorf("ResultPayload mismatch after frame round-trip")
	}
}

// ---------------------------------------------------------------------------
// Opcode table coverage
// ---------------------------------------------------------------------------

func TestAgentOpcodeNames(t *testing.T) {
	for _, op := range []byte{OpAgentNegotiate, OpAgentDelegate, OpAgentResult} {
		name, ok := OpcodeNames[op]
		if !ok {
			t.Errorf("opcode 0x%02x missing from OpcodeNames", op)
		}
		if name == "" {
			t.Errorf("opcode 0x%02x has empty name", op)
		}
	}
}
