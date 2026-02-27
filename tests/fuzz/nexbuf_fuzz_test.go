package fuzz

import (
	"bytes"
	"testing"

	"github.com/strand-protocol/strand/strandapi/pkg/strandbuf"
	"github.com/strand-protocol/strand/strandapi/pkg/protocol"
	"github.com/strand-protocol/strand/strandapi/pkg/sad"
)

// FuzzStrandBufRoundtrip feeds random bytes to the InferenceRequest decoder.
// If decoding succeeds, re-encode and verify the output matches the decoded
// state (encode -> decode -> encode must be idempotent).
func FuzzStrandBufRoundtrip(f *testing.F) {
	// Add seed corpus with a valid encoded InferenceRequest.
	req := &protocol.InferenceRequest{
		ID:          [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		Prompt:      "seed prompt",
		MaxTokens:   100,
		Temperature: 0.5,
		Metadata:    map[string]string{},
	}
	buf := strandbuf.NewBuffer(256)
	req.Encode(buf)
	f.Add(buf.Bytes())

	// Add an empty input.
	f.Add([]byte{})

	// Add random-looking data.
	f.Add([]byte{0xFF, 0xFE, 0x00, 0x01, 0x02, 0x03, 0x04, 0x05})

	f.Fuzz(func(t *testing.T, data []byte) {
		decoded := &protocol.InferenceRequest{}
		reader := strandbuf.NewReader(data)
		err := decoded.Decode(reader)
		if err != nil {
			// Decoding failed on random input -- that is perfectly fine.
			return
		}

		// If decoding succeeded, re-encode the decoded message.
		buf1 := strandbuf.NewBuffer(len(data) + 64)
		decoded.Encode(buf1)

		// Decode the re-encoded bytes.
		decoded2 := &protocol.InferenceRequest{}
		reader2 := strandbuf.NewReader(buf1.Bytes())
		if err := decoded2.Decode(reader2); err != nil {
			t.Fatalf("re-decode failed after successful decode+encode: %v", err)
		}

		// Re-encode the second decode and compare.
		buf2 := strandbuf.NewBuffer(len(data) + 64)
		decoded2.Encode(buf2)

		if !bytes.Equal(buf1.Bytes(), buf2.Bytes()) {
			t.Errorf("encode is not idempotent after decode:\n  first:  %x\n  second: %x", buf1.Bytes(), buf2.Bytes())
		}
	})
}

// FuzzFrameRead feeds random bytes to protocol.ReadFrame to ensure it never
// panics, regardless of input.
func FuzzFrameRead(f *testing.F) {
	// Valid frame: 4 bytes length (LE) + 1 byte opcode + payload.
	validFrame := []byte{
		0x05, 0x00, 0x00, 0x00, // length = 5
		0x01,                   // opcode = InferenceRequest
		'h', 'e', 'l', 'l', 'o', // payload
	}
	f.Add(validFrame)

	// Empty.
	f.Add([]byte{})

	// Truncated header.
	f.Add([]byte{0x01, 0x00})

	// Header with zero length.
	f.Add([]byte{0x00, 0x00, 0x00, 0x00, 0x08})

	// Absurdly large length (should hit maxPayloadSize check).
	f.Add([]byte{0xFF, 0xFF, 0xFF, 0xFF, 0x01})

	f.Fuzz(func(t *testing.T, data []byte) {
		reader := bytes.NewReader(data)
		// Must not panic.
		_, _, _ = protocol.ReadFrame(reader)
	})
}

// FuzzSADParse feeds random bytes to the SAD decoder to ensure it never panics.
func FuzzSADParse(f *testing.F) {
	// Valid SAD.
	s := &sad.SAD{
		Version:       1,
		Capabilities:  sad.TextGen | sad.CodeGen,
		ContextWindow: 128000,
		LatencySLA:    500,
		ModelType:     "llm",
	}
	buf := strandbuf.NewBuffer(64)
	s.Encode(buf)
	f.Add(buf.Bytes())

	// Empty.
	f.Add([]byte{})

	// Short data.
	f.Add([]byte{0x01, 0x00})

	// Random.
	f.Add([]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF})

	f.Fuzz(func(t *testing.T, data []byte) {
		decoded := &sad.SAD{}
		reader := strandbuf.NewReader(data)
		// Must not panic.
		_ = decoded.Decode(reader)
	})
}

// FuzzInferenceResponseDecode feeds random bytes to InferenceResponse.Decode.
func FuzzInferenceResponseDecode(f *testing.F) {
	resp := &protocol.InferenceResponse{
		ID:               [16]byte{0xFF},
		Text:             "test",
		FinishReason:     "stop",
		PromptTokens:     10,
		CompletionTokens: 5,
	}
	buf := strandbuf.NewBuffer(128)
	resp.Encode(buf)
	f.Add(buf.Bytes())

	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, data []byte) {
		decoded := &protocol.InferenceResponse{}
		reader := strandbuf.NewReader(data)
		_ = decoded.Decode(reader)
	})
}

// FuzzAgentNegotiateDecode feeds random bytes to AgentNegotiate.Decode.
func FuzzAgentNegotiateDecode(f *testing.F) {
	msg := &protocol.AgentNegotiate{
		SessionID:    42,
		Capabilities: []string{"text_gen", "code_gen"},
		Version:      1,
	}
	buf := strandbuf.NewBuffer(128)
	msg.Encode(buf)
	f.Add(buf.Bytes())

	f.Add([]byte{})
	// Oversized capability count.
	f.Add([]byte{0x01, 0x00, 0x00, 0x00, 0xFF, 0xFF, 0xFF, 0xFF})
	// Truncated after session ID.
	f.Add([]byte{0x01, 0x00, 0x00, 0x00})

	f.Fuzz(func(t *testing.T, data []byte) {
		decoded := &protocol.AgentNegotiate{}
		reader := strandbuf.NewReader(data)
		err := decoded.Decode(reader)
		if err != nil {
			return
		}
		// Re-encode and verify idempotency.
		buf1 := strandbuf.NewBuffer(len(data) + 64)
		decoded.Encode(buf1)
		decoded2 := &protocol.AgentNegotiate{}
		reader2 := strandbuf.NewReader(buf1.Bytes())
		if err := decoded2.Decode(reader2); err != nil {
			t.Fatalf("re-decode failed: %v", err)
		}
		buf2 := strandbuf.NewBuffer(len(data) + 64)
		decoded2.Encode(buf2)
		if !bytes.Equal(buf1.Bytes(), buf2.Bytes()) {
			t.Errorf("encode not idempotent:\n  first:  %x\n  second: %x", buf1.Bytes(), buf2.Bytes())
		}
	})
}

// FuzzAgentDelegateDecode feeds random bytes to AgentDelegate.Decode.
func FuzzAgentDelegateDecode(f *testing.F) {
	msg := &protocol.AgentDelegate{
		SessionID:    1,
		TargetNodeID: [16]byte{0xAA, 0xBB},
		TaskPayload:  []byte("task data"),
		TimeoutMS:    5000,
	}
	buf := strandbuf.NewBuffer(128)
	msg.Encode(buf)
	f.Add(buf.Bytes())

	f.Add([]byte{})
	f.Add([]byte{0xFF, 0x00, 0x00, 0x00})

	f.Fuzz(func(t *testing.T, data []byte) {
		decoded := &protocol.AgentDelegate{}
		reader := strandbuf.NewReader(data)
		err := decoded.Decode(reader)
		if err != nil {
			return
		}
		buf1 := strandbuf.NewBuffer(len(data) + 64)
		decoded.Encode(buf1)
		decoded2 := &protocol.AgentDelegate{}
		reader2 := strandbuf.NewReader(buf1.Bytes())
		if err := decoded2.Decode(reader2); err != nil {
			t.Fatalf("re-decode failed: %v", err)
		}
		buf2 := strandbuf.NewBuffer(len(data) + 64)
		decoded2.Encode(buf2)
		if !bytes.Equal(buf1.Bytes(), buf2.Bytes()) {
			t.Errorf("encode not idempotent:\n  first:  %x\n  second: %x", buf1.Bytes(), buf2.Bytes())
		}
	})
}

// FuzzAgentResultDecode feeds random bytes to AgentResult.Decode.
func FuzzAgentResultDecode(f *testing.F) {
	msg := &protocol.AgentResult{
		SessionID:     1,
		ResultPayload: []byte("result"),
		ErrorCode:     0,
		ErrorMsg:      "",
	}
	buf := strandbuf.NewBuffer(128)
	msg.Encode(buf)
	f.Add(buf.Bytes())

	f.Add([]byte{})
	f.Add([]byte{0x01, 0x00, 0x00, 0x00, 0xFF, 0xFF, 0xFF, 0xFF})

	f.Fuzz(func(t *testing.T, data []byte) {
		decoded := &protocol.AgentResult{}
		reader := strandbuf.NewReader(data)
		err := decoded.Decode(reader)
		if err != nil {
			return
		}
		buf1 := strandbuf.NewBuffer(len(data) + 64)
		decoded.Encode(buf1)
		decoded2 := &protocol.AgentResult{}
		reader2 := strandbuf.NewReader(buf1.Bytes())
		if err := decoded2.Decode(reader2); err != nil {
			t.Fatalf("re-decode failed: %v", err)
		}
		buf2 := strandbuf.NewBuffer(len(data) + 64)
		decoded2.Encode(buf2)
		if !bytes.Equal(buf1.Bytes(), buf2.Bytes()) {
			t.Errorf("encode not idempotent:\n  first:  %x\n  second: %x", buf1.Bytes(), buf2.Bytes())
		}
	})
}

// FuzzTensorTransferDecode feeds random bytes to TensorTransfer.Decode.
func FuzzTensorTransferDecode(f *testing.F) {
	tensor := &protocol.TensorTransfer{
		DType: 1,
		Shape: []uint32{4, 4},
		Data:  []byte{1, 2, 3, 4},
	}
	buf := strandbuf.NewBuffer(128)
	tensor.Encode(buf)
	f.Add(buf.Bytes())

	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, data []byte) {
		decoded := &protocol.TensorTransfer{}
		reader := strandbuf.NewReader(data)
		_ = decoded.Decode(reader)
	})
}
