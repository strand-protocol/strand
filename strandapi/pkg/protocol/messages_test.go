package protocol

import (
	"bytes"
	"testing"

	"github.com/nexus-protocol/nexus/nexapi/pkg/nexbuf"
)

func TestInferenceRequestRoundTrip(t *testing.T) {
	orig := &InferenceRequest{
		ID:          [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		ModelSAD:    []byte{0xAA, 0xBB, 0xCC},
		Prompt:      "What is the meaning of life?",
		MaxTokens:   4096,
		Temperature: 0.7,
		Metadata: map[string]string{
			"user":    "alice",
			"session": "abc123",
		},
	}

	buf := nexbuf.NewBuffer(256)
	orig.Encode(buf)

	decoded := &InferenceRequest{}
	reader := nexbuf.NewReader(buf.Bytes())
	if err := decoded.Decode(reader); err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if orig.ID != decoded.ID {
		t.Errorf("ID mismatch: %v != %v", orig.ID, decoded.ID)
	}
	if !bytes.Equal(orig.ModelSAD, decoded.ModelSAD) {
		t.Errorf("ModelSAD mismatch")
	}
	if orig.Prompt != decoded.Prompt {
		t.Errorf("Prompt: %q != %q", orig.Prompt, decoded.Prompt)
	}
	if orig.MaxTokens != decoded.MaxTokens {
		t.Errorf("MaxTokens: %d != %d", orig.MaxTokens, decoded.MaxTokens)
	}
	if orig.Temperature != decoded.Temperature {
		t.Errorf("Temperature: %v != %v", orig.Temperature, decoded.Temperature)
	}
	if len(orig.Metadata) != len(decoded.Metadata) {
		t.Fatalf("Metadata len: %d != %d", len(orig.Metadata), len(decoded.Metadata))
	}
	for k, v := range orig.Metadata {
		if decoded.Metadata[k] != v {
			t.Errorf("Metadata[%q]: %q != %q", k, decoded.Metadata[k], v)
		}
	}
}

func TestInferenceResponseRoundTrip(t *testing.T) {
	orig := &InferenceResponse{
		ID:               [16]byte{0xDE, 0xAD},
		Text:             "The meaning of life is 42.",
		FinishReason:     "stop",
		PromptTokens:     10,
		CompletionTokens: 7,
	}

	buf := nexbuf.NewBuffer(128)
	orig.Encode(buf)

	decoded := &InferenceResponse{}
	reader := nexbuf.NewReader(buf.Bytes())
	if err := decoded.Decode(reader); err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if orig.ID != decoded.ID {
		t.Errorf("ID mismatch")
	}
	if orig.Text != decoded.Text {
		t.Errorf("Text: %q != %q", orig.Text, decoded.Text)
	}
	if orig.FinishReason != decoded.FinishReason {
		t.Errorf("FinishReason: %q != %q", orig.FinishReason, decoded.FinishReason)
	}
	if orig.PromptTokens != decoded.PromptTokens {
		t.Errorf("PromptTokens: %d != %d", orig.PromptTokens, decoded.PromptTokens)
	}
	if orig.CompletionTokens != decoded.CompletionTokens {
		t.Errorf("CompletionTokens: %d != %d", orig.CompletionTokens, decoded.CompletionTokens)
	}
}

func TestTokenStreamChunkRoundTrip(t *testing.T) {
	orig := &TokenStreamChunk{
		RequestID: [16]byte{0x01, 0x02, 0x03},
		SeqNum:    42,
		Token:     "Hello",
		Logprob:   -1.5,
	}

	buf := nexbuf.NewBuffer(64)
	orig.Encode(buf)

	decoded := &TokenStreamChunk{}
	reader := nexbuf.NewReader(buf.Bytes())
	if err := decoded.Decode(reader); err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if orig.RequestID != decoded.RequestID {
		t.Errorf("RequestID mismatch")
	}
	if orig.SeqNum != decoded.SeqNum {
		t.Errorf("SeqNum: %d != %d", orig.SeqNum, decoded.SeqNum)
	}
	if orig.Token != decoded.Token {
		t.Errorf("Token: %q != %q", orig.Token, decoded.Token)
	}
	if orig.Logprob != decoded.Logprob {
		t.Errorf("Logprob: %v != %v", orig.Logprob, decoded.Logprob)
	}
}

func TestTensorTransferRoundTrip(t *testing.T) {
	data := make([]byte, 256)
	for i := range data {
		data[i] = byte(i)
	}

	orig := &TensorTransfer{
		ID:    [16]byte{0xFF, 0xEE, 0xDD},
		DType: 5, // float32
		Shape: []uint32{2, 3, 4},
		Data:  data,
	}

	buf := nexbuf.NewBuffer(512)
	orig.Encode(buf)

	decoded := &TensorTransfer{}
	reader := nexbuf.NewReader(buf.Bytes())
	if err := decoded.Decode(reader); err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if orig.ID != decoded.ID {
		t.Errorf("ID mismatch")
	}
	if orig.DType != decoded.DType {
		t.Errorf("DType: %d != %d", orig.DType, decoded.DType)
	}
	if len(orig.Shape) != len(decoded.Shape) {
		t.Fatalf("Shape len: %d != %d", len(orig.Shape), len(decoded.Shape))
	}
	for i, v := range orig.Shape {
		if decoded.Shape[i] != v {
			t.Errorf("Shape[%d]: %d != %d", i, decoded.Shape[i], v)
		}
	}
	if !bytes.Equal(orig.Data, decoded.Data) {
		t.Errorf("Data mismatch (len %d vs %d)", len(orig.Data), len(decoded.Data))
	}
}

func TestFrameRoundTrip(t *testing.T) {
	payload := []byte("test payload data")

	var buf bytes.Buffer
	if err := WriteFrame(&buf, OpInferenceRequest, payload); err != nil {
		t.Fatalf("WriteFrame: %v", err)
	}

	opcode, got, err := ReadFrame(&buf)
	if err != nil {
		t.Fatalf("ReadFrame: %v", err)
	}
	if opcode != OpInferenceRequest {
		t.Errorf("opcode = 0x%02x, want 0x%02x", opcode, OpInferenceRequest)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("payload mismatch")
	}
}

func TestFrameEmptyPayload(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteFrame(&buf, OpHeartbeat, nil); err != nil {
		t.Fatalf("WriteFrame: %v", err)
	}

	opcode, payload, err := ReadFrame(&buf)
	if err != nil {
		t.Fatalf("ReadFrame: %v", err)
	}
	if opcode != OpHeartbeat {
		t.Errorf("opcode = 0x%02x, want 0x%02x", opcode, OpHeartbeat)
	}
	if len(payload) != 0 {
		t.Errorf("expected empty payload, got %d bytes", len(payload))
	}
}

func TestInferenceRequestEmptyMetadata(t *testing.T) {
	orig := &InferenceRequest{
		Prompt:      "hello",
		MaxTokens:   100,
		Temperature: 0.5,
		Metadata:    map[string]string{},
	}

	buf := nexbuf.NewBuffer(64)
	orig.Encode(buf)

	decoded := &InferenceRequest{}
	reader := nexbuf.NewReader(buf.Bytes())
	if err := decoded.Decode(reader); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(decoded.Metadata) != 0 {
		t.Errorf("expected empty metadata, got %d entries", len(decoded.Metadata))
	}
}

func TestTensorTransferEmptyShape(t *testing.T) {
	orig := &TensorTransfer{
		DType: 1,
		Shape: []uint32{},
		Data:  []byte{0x42},
	}

	buf := nexbuf.NewBuffer(64)
	orig.Encode(buf)

	decoded := &TensorTransfer{}
	reader := nexbuf.NewReader(buf.Bytes())
	if err := decoded.Decode(reader); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(decoded.Shape) != 0 {
		t.Errorf("expected empty shape, got %v", decoded.Shape)
	}
	if !bytes.Equal(decoded.Data, orig.Data) {
		t.Errorf("data mismatch")
	}
}
