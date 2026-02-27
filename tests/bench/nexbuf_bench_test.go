package bench

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/nexus-protocol/nexus/nexapi/pkg/nexbuf"
	"github.com/nexus-protocol/nexus/nexapi/pkg/protocol"
	"github.com/nexus-protocol/nexus/nexapi/pkg/sad"
	"github.com/nexus-protocol/nexus/nexapi/pkg/transport"
)

// --------------------------------------------------------------------------
// NexBuf encode benchmarks
// --------------------------------------------------------------------------

// BenchmarkNexBufEncodeSmall benchmarks encoding a small InferenceRequest.
func BenchmarkNexBufEncodeSmall(b *testing.B) {
	req := &protocol.InferenceRequest{
		ID:          [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		Prompt:      "What is the capital of France?",
		MaxTokens:   100,
		Temperature: 0.7,
		Metadata:    map[string]string{"user": "bench"},
	}

	buf := nexbuf.NewBuffer(256)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		req.Encode(buf)
	}
	b.SetBytes(int64(buf.Len()))
}

// BenchmarkNexBufEncodeLarge benchmarks encoding a large TensorTransfer.
func BenchmarkNexBufEncodeLarge(b *testing.B) {
	data := make([]byte, 1024*1024) // 1 MiB tensor
	rng := rand.New(rand.NewSource(42))
	rng.Read(data)

	tensor := &protocol.TensorTransfer{
		ID:    [16]byte{0xAA, 0xBB, 0xCC, 0xDD},
		DType: 5,
		Shape: []uint32{256, 256, 16},
		Data:  data,
	}

	buf := nexbuf.NewBuffer(len(data) + 256)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		tensor.Encode(buf)
	}
	b.SetBytes(int64(buf.Len()))
}

// --------------------------------------------------------------------------
// NexBuf decode benchmarks
// --------------------------------------------------------------------------

// BenchmarkNexBufDecode benchmarks decoding an InferenceRequest.
func BenchmarkNexBufDecode(b *testing.B) {
	req := &protocol.InferenceRequest{
		ID:          [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		ModelSAD:    []byte("sad:llm:gpt4:128k"),
		Prompt:      "Explain quantum computing in simple terms.",
		MaxTokens:   2048,
		Temperature: 0.8,
		Metadata: map[string]string{
			"user":    "bench",
			"session": "sess-abc123",
			"model":   "gpt-4",
		},
	}

	buf := nexbuf.NewBuffer(512)
	req.Encode(buf)
	encoded := make([]byte, buf.Len())
	copy(encoded, buf.Bytes())

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		decoded := &protocol.InferenceRequest{}
		reader := nexbuf.NewReader(encoded)
		if err := decoded.Decode(reader); err != nil {
			b.Fatalf("decode: %v", err)
		}
	}
	b.SetBytes(int64(len(encoded)))
}

// BenchmarkNexBufDecodeResponse benchmarks decoding an InferenceResponse.
func BenchmarkNexBufDecodeResponse(b *testing.B) {
	resp := &protocol.InferenceResponse{
		ID:               [16]byte{0xFF, 0xFE},
		Text:             "Paris is the capital of France. It is known for the Eiffel Tower and rich cultural heritage.",
		FinishReason:     "stop",
		PromptTokens:     15,
		CompletionTokens: 20,
	}

	buf := nexbuf.NewBuffer(256)
	resp.Encode(buf)
	encoded := make([]byte, buf.Len())
	copy(encoded, buf.Bytes())

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		decoded := &protocol.InferenceResponse{}
		reader := nexbuf.NewReader(encoded)
		if err := decoded.Decode(reader); err != nil {
			b.Fatalf("decode: %v", err)
		}
	}
	b.SetBytes(int64(len(encoded)))
}

// --------------------------------------------------------------------------
// Overlay transport roundtrip
// --------------------------------------------------------------------------

// BenchmarkOverlayRoundtrip benchmarks a full send/recv cycle through the
// overlay transport on loopback.
func BenchmarkOverlayRoundtrip(b *testing.B) {
	// Bind a listener.
	listener, err := transport.ListenOverlay("127.0.0.1:0")
	if err != nil {
		b.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	addr := listener.LocalAddr().String()

	sender, err := transport.DialOverlay(addr)
	if err != nil {
		b.Fatalf("dial: %v", err)
	}
	defer sender.Close()

	ctx := context.Background()
	payload := []byte("benchmark payload data for round-trip test")

	// Warm up: send one message so the listener captures the remote addr.
	if err := sender.Send(ctx, protocol.OpInferenceRequest, payload); err != nil {
		b.Fatalf("warmup send: %v", err)
	}
	_, _, err = listener.Recv(ctx)
	if err != nil {
		b.Fatalf("warmup recv: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := sender.Send(ctx, protocol.OpInferenceRequest, payload); err != nil {
			b.Fatalf("send: %v", err)
		}

		_, recvPayload, err := listener.Recv(ctx)
		if err != nil {
			b.Fatalf("recv: %v", err)
		}
		_ = recvPayload
	}
	b.SetBytes(int64(len(payload)))
}

// --------------------------------------------------------------------------
// SAD benchmarks
// --------------------------------------------------------------------------

// BenchmarkSADBuild benchmarks building a SAD descriptor via the builder.
func BenchmarkSADBuild(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := sad.NewSADBuilder().
			ModelType("llm").
			WithCapability(sad.TextGen).
			WithCapability(sad.CodeGen).
			WithCapability(sad.ToolUse).
			ContextWindow(128000).
			LatencySLA(100).
			Build()
		if err != nil {
			b.Fatalf("build: %v", err)
		}
	}
}

// BenchmarkSADParse benchmarks parsing a SAD from binary bytes.
func BenchmarkSADParse(b *testing.B) {
	sadBytes, err := sad.NewSADBuilder().
		ModelType("llm").
		WithCapability(sad.TextGen | sad.CodeGen | sad.ToolUse | sad.Vision).
		ContextWindow(128000).
		LatencySLA(100).
		Build()
	if err != nil {
		b.Fatalf("build: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		s := &sad.SAD{}
		reader := nexbuf.NewReader(sadBytes)
		if err := s.Decode(reader); err != nil {
			b.Fatalf("decode: %v", err)
		}
	}
	b.SetBytes(int64(len(sadBytes)))
}

// --------------------------------------------------------------------------
// Frame write/read benchmark
// --------------------------------------------------------------------------

// BenchmarkFrameWriteRead benchmarks framing a payload using WriteFrame/ReadFrame.
func BenchmarkFrameWriteRead(b *testing.B) {
	payload := make([]byte, 1024)
	for i := range payload {
		payload[i] = byte(i)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		if err := protocol.WriteFrame(&buf, protocol.OpInferenceRequest, payload); err != nil {
			b.Fatalf("write: %v", err)
		}
		_, _, err := protocol.ReadFrame(&buf)
		if err != nil {
			b.Fatalf("read: %v", err)
		}
	}
	b.SetBytes(int64(len(payload)))
}

// --------------------------------------------------------------------------
// Benchmark various message sizes
// --------------------------------------------------------------------------

func BenchmarkNexBufEncodeVarySizes(b *testing.B) {
	sizes := []int{64, 1024, 64 * 1024}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("payload_%d", size), func(b *testing.B) {
			data := make([]byte, size)
			tensor := &protocol.TensorTransfer{
				DType: 1,
				Shape: []uint32{uint32(size)},
				Data:  data,
			}
			buf := nexbuf.NewBuffer(size + 128)

			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				buf.Reset()
				tensor.Encode(buf)
			}
			b.SetBytes(int64(buf.Len()))
		})
	}
}

// Ensure transport package is used (prevents import error if benchmark
// functions above are conditionally excluded).
var _ = transport.OverlayMagic
var _ = time.Second
