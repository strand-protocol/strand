// Package sad implements the Semantic Address Descriptor (SAD) type used by
// NexRoute for AI-model-aware routing. A SAD is a compact binary descriptor
// that encodes model capabilities, context window size, latency SLA, and other
// routing-relevant properties.
package sad

import (
	"github.com/nexus-protocol/nexus/nexapi/pkg/nexbuf"
)

// Capability bit flags used in a SAD to advertise or request model abilities.
const (
	TextGen  uint32 = 1 << 0 // Text generation
	CodeGen  uint32 = 1 << 1 // Code generation
	Embedding uint32 = 1 << 2 // Embedding / vector generation
	ImageGen uint32 = 1 << 3 // Image generation
	AudioGen uint32 = 1 << 4 // Audio generation
	ToolUse  uint32 = 1 << 5 // Function/tool calling
	Vision   uint32 = 1 << 6 // Image/visual understanding
)

// SAD is the in-memory representation of a Semantic Address Descriptor.
type SAD struct {
	ModelType     string // e.g. "llm", "embedding", "diffusion"
	Capabilities  uint32 // Bitmask of capability flags
	ContextWindow uint32 // Maximum context window in tokens
	LatencySLA    uint32 // Target latency in milliseconds
	Version       uint16 // SAD format version (currently 1)
}

// Encode writes the binary representation of the SAD to buf. The format is:
//
//	[2B version][4B capabilities][4B context_window][4B latency_sla][string model_type]
func (s *SAD) Encode(buf *nexbuf.Buffer) {
	buf.WriteUint16(s.Version)
	buf.WriteUint32(s.Capabilities)
	buf.WriteUint32(s.ContextWindow)
	buf.WriteUint32(s.LatencySLA)
	buf.WriteString(s.ModelType)
}

// Decode populates the SAD from a NexBuf reader.
func (s *SAD) Decode(r *nexbuf.Reader) error {
	var err error
	s.Version, err = r.ReadUint16()
	if err != nil {
		return err
	}
	s.Capabilities, err = r.ReadUint32()
	if err != nil {
		return err
	}
	s.ContextWindow, err = r.ReadUint32()
	if err != nil {
		return err
	}
	s.LatencySLA, err = r.ReadUint32()
	if err != nil {
		return err
	}
	s.ModelType, err = r.ReadString()
	if err != nil {
		return err
	}
	return nil
}
