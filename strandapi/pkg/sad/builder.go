package sad

import (
	"errors"

	"github.com/strand-protocol/strand/strandapi/pkg/strandbuf"
)

// Builder provides a fluent interface for constructing SAD descriptors.
type Builder struct {
	sad SAD
}

// NewSADBuilder returns a new Builder with sensible defaults.
func NewSADBuilder() *Builder {
	return &Builder{
		sad: SAD{
			Version: 1,
		},
	}
}

// ModelType sets the model type string (e.g. "llm", "embedding").
func (b *Builder) ModelType(t string) *Builder {
	b.sad.ModelType = t
	return b
}

// Capabilities sets the capability bitmask directly.
func (b *Builder) Capabilities(c uint32) *Builder {
	b.sad.Capabilities = c
	return b
}

// WithCapability sets one or more capability flags using bitwise OR.
func (b *Builder) WithCapability(flags uint32) *Builder {
	b.sad.Capabilities |= flags
	return b
}

// ContextWindow sets the required minimum context window size (in tokens).
func (b *Builder) ContextWindow(tokens uint32) *Builder {
	b.sad.ContextWindow = tokens
	return b
}

// LatencySLA sets the target latency in milliseconds.
func (b *Builder) LatencySLA(ms uint32) *Builder {
	b.sad.LatencySLA = ms
	return b
}

// Build encodes the SAD into its binary wire representation and returns the
// bytes. Returns an error if the descriptor is incomplete.
func (b *Builder) Build() ([]byte, error) {
	if b.sad.ModelType == "" {
		return nil, errors.New("sad: model type is required")
	}
	buf := strandbuf.NewBuffer(64)
	b.sad.Encode(buf)
	return buf.Bytes(), nil
}

// BuildSAD returns the SAD struct directly (useful when you want the typed
// value rather than the wire bytes).
func (b *Builder) BuildSAD() (*SAD, error) {
	if b.sad.ModelType == "" {
		return nil, errors.New("sad: model type is required")
	}
	s := b.sad // copy
	return &s, nil
}
