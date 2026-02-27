package integration

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/strand-protocol/strand/strandapi/pkg/strandbuf"
	"github.com/strand-protocol/strand/strandapi/pkg/protocol"
	"github.com/strand-protocol/strand/strandapi/pkg/sad"
)

// TestStrandBufEncodingDeterministic verifies that encoding the same message
// twice produces identical byte sequences.
func TestStrandBufEncodingDeterministic(t *testing.T) {
	req := &protocol.InferenceRequest{
		ID:          [16]byte{0xAA, 0xBB, 0xCC, 0xDD, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C},
		ModelSAD:    []byte{0x01, 0x02, 0x03},
		Prompt:      "deterministic test",
		MaxTokens:   512,
		Temperature: 0.9,
		// NOTE: empty metadata map to ensure deterministic encoding (maps
		// with entries would iterate in random order in Go).
		Metadata: map[string]string{},
	}

	buf1 := strandbuf.NewBuffer(256)
	req.Encode(buf1)

	buf2 := strandbuf.NewBuffer(256)
	req.Encode(buf2)

	if !bytes.Equal(buf1.Bytes(), buf2.Bytes()) {
		t.Errorf("StrandBuf encoding is NOT deterministic:\n  first:  %x\n  second: %x", buf1.Bytes(), buf2.Bytes())
	}
}

// TestStrandBufEncodingDeterministicNoMetadata ensures a message with no
// metadata entries encodes deterministically.
func TestStrandBufEncodingDeterministicNoMetadata(t *testing.T) {
	req := &protocol.InferenceRequest{
		Prompt:   "no metadata",
		Metadata: map[string]string{},
	}

	buf1 := strandbuf.NewBuffer(128)
	req.Encode(buf1)

	buf2 := strandbuf.NewBuffer(128)
	req.Encode(buf2)

	if !bytes.Equal(buf1.Bytes(), buf2.Bytes()) {
		t.Error("encoding without metadata is not deterministic")
	}
}

// TestStrandBufEncodeDecodeRoundtrip verifies encode -> decode produces the
// same logical message.
func TestStrandBufEncodeDecodeRoundtrip(t *testing.T) {
	original := &protocol.InferenceRequest{
		ID:          [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		ModelSAD:    []byte("sad:llm:gpt4:128k"),
		Prompt:      "Hello, world!",
		MaxTokens:   1024,
		Temperature: 0.7,
		Metadata: map[string]string{
			"user":    "test",
			"session": "abc123",
		},
	}

	buf := strandbuf.NewBuffer(256)
	original.Encode(buf)

	decoded := &protocol.InferenceRequest{}
	reader := strandbuf.NewReader(buf.Bytes())
	if err := decoded.Decode(reader); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if decoded.ID != original.ID {
		t.Errorf("ID mismatch: %v != %v", decoded.ID, original.ID)
	}
	if !bytes.Equal(decoded.ModelSAD, original.ModelSAD) {
		t.Errorf("ModelSAD mismatch")
	}
	if decoded.Prompt != original.Prompt {
		t.Errorf("Prompt mismatch: %q != %q", decoded.Prompt, original.Prompt)
	}
	if decoded.MaxTokens != original.MaxTokens {
		t.Errorf("MaxTokens: %d != %d", decoded.MaxTokens, original.MaxTokens)
	}
	if decoded.Temperature != original.Temperature {
		t.Errorf("Temperature: %f != %f", decoded.Temperature, original.Temperature)
	}
	if len(decoded.Metadata) != len(original.Metadata) {
		t.Errorf("Metadata length: %d != %d", len(decoded.Metadata), len(original.Metadata))
	}
	for k, v := range original.Metadata {
		if decoded.Metadata[k] != v {
			t.Errorf("Metadata[%q]: %q != %q", k, decoded.Metadata[k], v)
		}
	}
}

// TestInferenceResponseRoundtrip verifies encode/decode for InferenceResponse.
func TestInferenceResponseRoundtrip(t *testing.T) {
	original := &protocol.InferenceResponse{
		ID:               [16]byte{0xFF, 0xFE, 0xFD},
		Text:             "The answer is 42.",
		FinishReason:     "stop",
		PromptTokens:     100,
		CompletionTokens: 7,
	}

	buf := strandbuf.NewBuffer(128)
	original.Encode(buf)

	decoded := &protocol.InferenceResponse{}
	reader := strandbuf.NewReader(buf.Bytes())
	if err := decoded.Decode(reader); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if decoded.ID != original.ID {
		t.Errorf("ID mismatch")
	}
	if decoded.Text != original.Text {
		t.Errorf("Text: %q != %q", decoded.Text, original.Text)
	}
	if decoded.FinishReason != original.FinishReason {
		t.Errorf("FinishReason: %q != %q", decoded.FinishReason, original.FinishReason)
	}
	if decoded.PromptTokens != original.PromptTokens {
		t.Errorf("PromptTokens: %d != %d", decoded.PromptTokens, original.PromptTokens)
	}
	if decoded.CompletionTokens != original.CompletionTokens {
		t.Errorf("CompletionTokens: %d != %d", decoded.CompletionTokens, original.CompletionTokens)
	}
}

// TestTensorTransferRoundtrip verifies encode/decode for TensorTransfer.
func TestTensorTransferRoundtrip(t *testing.T) {
	data := make([]byte, 4096)
	for i := range data {
		data[i] = byte(i % 256)
	}

	original := &protocol.TensorTransfer{
		ID:    [16]byte{0x10, 0x20, 0x30, 0x40},
		DType: 3,
		Shape: []uint32{32, 128},
		Data:  data,
	}

	buf := strandbuf.NewBuffer(8192)
	original.Encode(buf)

	decoded := &protocol.TensorTransfer{}
	reader := strandbuf.NewReader(buf.Bytes())
	if err := decoded.Decode(reader); err != nil {
		t.Fatalf("decode tensor: %v", err)
	}

	if decoded.ID != original.ID {
		t.Errorf("ID mismatch")
	}
	if decoded.DType != original.DType {
		t.Errorf("DType: %d != %d", decoded.DType, original.DType)
	}
	if len(decoded.Shape) != len(original.Shape) {
		t.Fatalf("Shape length: %d != %d", len(decoded.Shape), len(original.Shape))
	}
	for i := range original.Shape {
		if decoded.Shape[i] != original.Shape[i] {
			t.Errorf("Shape[%d]: %d != %d", i, decoded.Shape[i], original.Shape[i])
		}
	}
	if !bytes.Equal(decoded.Data, original.Data) {
		t.Errorf("Data mismatch (len %d vs %d)", len(decoded.Data), len(original.Data))
	}
}

// TestSADBinaryFormatLayout verifies that the SAD binary encoding follows the
// specified wire format: [2B version][4B caps][4B ctx_window][4B latency][string model_type].
func TestSADBinaryFormatLayout(t *testing.T) {
	s := &sad.SAD{
		Version:       1,
		Capabilities:  sad.TextGen | sad.CodeGen,
		ContextWindow: 128000,
		LatencySLA:    500,
		ModelType:     "llm",
	}

	buf := strandbuf.NewBuffer(64)
	s.Encode(buf)
	data := buf.Bytes()

	offset := 0

	// 2 bytes: version (little-endian uint16)
	if len(data) < offset+2 {
		t.Fatalf("data too short for version")
	}
	version := binary.LittleEndian.Uint16(data[offset:])
	if version != 1 {
		t.Errorf("version: got %d, want 1", version)
	}
	offset += 2

	// 4 bytes: capabilities (little-endian uint32)
	if len(data) < offset+4 {
		t.Fatalf("data too short for capabilities")
	}
	caps := binary.LittleEndian.Uint32(data[offset:])
	if caps != (sad.TextGen | sad.CodeGen) {
		t.Errorf("capabilities: got 0x%x, want 0x%x", caps, sad.TextGen|sad.CodeGen)
	}
	offset += 4

	// 4 bytes: context window
	ctxWin := binary.LittleEndian.Uint32(data[offset:])
	if ctxWin != 128000 {
		t.Errorf("context_window: got %d, want 128000", ctxWin)
	}
	offset += 4

	// 4 bytes: latency SLA
	latency := binary.LittleEndian.Uint32(data[offset:])
	if latency != 500 {
		t.Errorf("latency_sla: got %d, want 500", latency)
	}
	offset += 4

	// string: 4 bytes length + payload
	strLen := binary.LittleEndian.Uint32(data[offset:])
	offset += 4
	modelType := string(data[offset : offset+int(strLen)])
	if modelType != "llm" {
		t.Errorf("model_type: got %q, want %q", modelType, "llm")
	}
}

// TestSADRoundtrip verifies SAD encode/decode round-trip.
func TestSADRoundtrip(t *testing.T) {
	original := &sad.SAD{
		Version:       1,
		Capabilities:  sad.TextGen | sad.Vision | sad.ToolUse,
		ContextWindow: 32000,
		LatencySLA:    200,
		ModelType:     "multimodal",
	}

	buf := strandbuf.NewBuffer(64)
	original.Encode(buf)

	decoded := &sad.SAD{}
	reader := strandbuf.NewReader(buf.Bytes())
	if err := decoded.Decode(reader); err != nil {
		t.Fatalf("SAD decode: %v", err)
	}

	if decoded.Version != original.Version {
		t.Errorf("Version: %d != %d", decoded.Version, original.Version)
	}
	if decoded.Capabilities != original.Capabilities {
		t.Errorf("Capabilities: 0x%x != 0x%x", decoded.Capabilities, original.Capabilities)
	}
	if decoded.ContextWindow != original.ContextWindow {
		t.Errorf("ContextWindow: %d != %d", decoded.ContextWindow, original.ContextWindow)
	}
	if decoded.LatencySLA != original.LatencySLA {
		t.Errorf("LatencySLA: %d != %d", decoded.LatencySLA, original.LatencySLA)
	}
	if decoded.ModelType != original.ModelType {
		t.Errorf("ModelType: %q != %q", decoded.ModelType, original.ModelType)
	}
}

// TestFrameWriteReadRoundtrip verifies the protocol framing (WriteFrame/ReadFrame).
func TestFrameWriteReadRoundtrip(t *testing.T) {
	testCases := []struct {
		name    string
		opcode  byte
		payload []byte
	}{
		{"empty payload", protocol.OpHeartbeat, nil},
		{"small payload", protocol.OpInferenceRequest, []byte("hello")},
		{"opcode error", protocol.OpError, []byte("something went wrong")},
		{"large payload", protocol.OpTensorTransfer, make([]byte, 8192)},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := protocol.WriteFrame(&buf, tc.opcode, tc.payload); err != nil {
				t.Fatalf("WriteFrame: %v", err)
			}

			opcode, payload, err := protocol.ReadFrame(&buf)
			if err != nil {
				t.Fatalf("ReadFrame: %v", err)
			}

			if opcode != tc.opcode {
				t.Errorf("opcode: got 0x%02x, want 0x%02x", opcode, tc.opcode)
			}
			if !bytes.Equal(payload, tc.payload) {
				t.Errorf("payload mismatch (len %d vs %d)", len(payload), len(tc.payload))
			}
		})
	}
}

// TestFramingConsistency verifies that multiple frames can be written to and
// read from a single stream correctly.
func TestFramingConsistency(t *testing.T) {
	var buf bytes.Buffer

	frames := []struct {
		opcode  byte
		payload []byte
	}{
		{protocol.OpInferenceRequest, []byte("first request")},
		{protocol.OpInferenceResponse, []byte("first response")},
		{protocol.OpHeartbeat, nil},
		{protocol.OpError, []byte("test error")},
	}

	// Write all frames.
	for _, f := range frames {
		if err := protocol.WriteFrame(&buf, f.opcode, f.payload); err != nil {
			t.Fatalf("WriteFrame: %v", err)
		}
	}

	// Read all frames back.
	for i, f := range frames {
		opcode, payload, err := protocol.ReadFrame(&buf)
		if err != nil {
			t.Fatalf("ReadFrame[%d]: %v", i, err)
		}
		if opcode != f.opcode {
			t.Errorf("frame %d opcode: 0x%02x != 0x%02x", i, opcode, f.opcode)
		}
		if !bytes.Equal(payload, f.payload) {
			t.Errorf("frame %d payload mismatch", i)
		}
	}
}

// TestSADBuilderRoundtrip tests the SAD builder -> encode -> decode cycle.
func TestSADBuilderRoundtrip(t *testing.T) {
	sadBytes, err := sad.NewSADBuilder().
		ModelType("llm").
		WithCapability(sad.TextGen).
		WithCapability(sad.CodeGen).
		ContextWindow(128000).
		LatencySLA(100).
		Build()
	if err != nil {
		t.Fatalf("SAD build: %v", err)
	}

	// Decode.
	decoded := &sad.SAD{}
	reader := strandbuf.NewReader(sadBytes)
	if err := decoded.Decode(reader); err != nil {
		t.Fatalf("SAD decode: %v", err)
	}

	if decoded.ModelType != "llm" {
		t.Errorf("ModelType: %q", decoded.ModelType)
	}
	if decoded.Capabilities != (sad.TextGen | sad.CodeGen) {
		t.Errorf("Capabilities: 0x%x", decoded.Capabilities)
	}
	if decoded.ContextWindow != 128000 {
		t.Errorf("ContextWindow: %d", decoded.ContextWindow)
	}
	if decoded.LatencySLA != 100 {
		t.Errorf("LatencySLA: %d", decoded.LatencySLA)
	}
}
