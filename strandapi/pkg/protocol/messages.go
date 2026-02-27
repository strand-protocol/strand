package protocol

import (
	"fmt"

	"github.com/strand-protocol/strand/strandapi/pkg/strandbuf"
)

// Allocation-bomb guards: cap collection sizes read from the wire so a
// malicious peer cannot exhaust heap memory with a single crafted message.
const (
	maxMetadataEntries = 256
	maxShapeDimensions = 8
)

// InferenceRequest is the primary message sent by a client to request model
// inference. It carries a 128-bit request ID, an optional SAD-encoded model
// selector, the prompt text, generation parameters, and arbitrary metadata.
type InferenceRequest struct {
	ID          [16]byte          // Unique 128-bit request identifier
	ModelSAD    []byte            // Semantic Address Descriptor (StrandRoute binary)
	Prompt      string            // User prompt / input text
	MaxTokens   uint32            // Maximum tokens to generate
	Temperature float32           // Sampling temperature
	Metadata    map[string]string // Custom key-value metadata
}

// Encode serialises the InferenceRequest into buf using StrandBuf wire format.
func (m *InferenceRequest) Encode(buf *strandbuf.Buffer) {
	off := buf.Len()
	_ = off
	// ID: 16 raw bytes
	for i := 0; i < 16; i++ {
		buf.WriteUint8(m.ID[i])
	}
	// ModelSAD: length-prefixed bytes
	buf.WriteBytes(m.ModelSAD)
	// Prompt: length-prefixed string
	buf.WriteString(m.Prompt)
	// MaxTokens: uint32
	buf.WriteUint32(m.MaxTokens)
	// Temperature: float32
	buf.WriteFloat32(m.Temperature)
	// Metadata: map<string,string>
	buf.WriteMapLen(uint32(len(m.Metadata)))
	for k, v := range m.Metadata {
		buf.WriteString(k)
		buf.WriteString(v)
	}
}

// Decode reads an InferenceRequest from r. Returns an error if the data is
// incomplete or malformed.
func (m *InferenceRequest) Decode(r *strandbuf.Reader) error {
	// ID
	for i := 0; i < 16; i++ {
		b, err := r.ReadUint8()
		if err != nil {
			return err
		}
		m.ID[i] = b
	}
	// ModelSAD
	sad, err := r.ReadBytes()
	if err != nil {
		return err
	}
	m.ModelSAD = make([]byte, len(sad))
	copy(m.ModelSAD, sad)
	// Prompt
	m.Prompt, err = r.ReadString()
	if err != nil {
		return err
	}
	// MaxTokens
	m.MaxTokens, err = r.ReadUint32()
	if err != nil {
		return err
	}
	// Temperature
	m.Temperature, err = r.ReadFloat32()
	if err != nil {
		return err
	}
	// Metadata — cap to prevent allocation-bomb DoS.
	count, err := r.ReadMapLen()
	if err != nil {
		return err
	}
	if count > maxMetadataEntries {
		return fmt.Errorf("strandapi: metadata count %d exceeds max %d", count, maxMetadataEntries)
	}
	m.Metadata = make(map[string]string, count)
	for i := uint32(0); i < count; i++ {
		k, err := r.ReadString()
		if err != nil {
			return err
		}
		v, err := r.ReadString()
		if err != nil {
			return err
		}
		m.Metadata[k] = v
	}
	return nil
}

// InferenceResponse is the complete (non-streaming) response to an
// InferenceRequest.
type InferenceResponse struct {
	ID               [16]byte // Matches the request ID
	Text             string   // Generated text
	FinishReason     string   // "stop", "length", "tool_use", etc.
	PromptTokens     uint32   // Tokens consumed by the prompt
	CompletionTokens uint32   // Tokens generated
}

// Encode serialises the InferenceResponse into buf.
func (m *InferenceResponse) Encode(buf *strandbuf.Buffer) {
	for i := 0; i < 16; i++ {
		buf.WriteUint8(m.ID[i])
	}
	buf.WriteString(m.Text)
	buf.WriteString(m.FinishReason)
	buf.WriteUint32(m.PromptTokens)
	buf.WriteUint32(m.CompletionTokens)
}

// Decode reads an InferenceResponse from r.
func (m *InferenceResponse) Decode(r *strandbuf.Reader) error {
	for i := 0; i < 16; i++ {
		b, err := r.ReadUint8()
		if err != nil {
			return err
		}
		m.ID[i] = b
	}
	var err error
	m.Text, err = r.ReadString()
	if err != nil {
		return err
	}
	m.FinishReason, err = r.ReadString()
	if err != nil {
		return err
	}
	m.PromptTokens, err = r.ReadUint32()
	if err != nil {
		return err
	}
	m.CompletionTokens, err = r.ReadUint32()
	if err != nil {
		return err
	}
	return nil
}

// TokenStreamChunk represents a single token (or small batch of tokens)
// delivered during streaming inference.
type TokenStreamChunk struct {
	RequestID [16]byte // Links back to the originating request
	SeqNum    uint32   // Sequence number for client-side reassembly
	Token     string   // The generated token text
	Logprob   float32  // Log probability of this token
}

// Encode serialises the TokenStreamChunk into buf.
func (m *TokenStreamChunk) Encode(buf *strandbuf.Buffer) {
	for i := 0; i < 16; i++ {
		buf.WriteUint8(m.RequestID[i])
	}
	buf.WriteUint32(m.SeqNum)
	buf.WriteString(m.Token)
	buf.WriteFloat32(m.Logprob)
}

// Decode reads a TokenStreamChunk from r.
func (m *TokenStreamChunk) Decode(r *strandbuf.Reader) error {
	for i := 0; i < 16; i++ {
		b, err := r.ReadUint8()
		if err != nil {
			return err
		}
		m.RequestID[i] = b
	}
	var err error
	m.SeqNum, err = r.ReadUint32()
	if err != nil {
		return err
	}
	m.Token, err = r.ReadString()
	if err != nil {
		return err
	}
	m.Logprob, err = r.ReadFloat32()
	if err != nil {
		return err
	}
	return nil
}

// TensorTransfer carries bulk tensor data (model weights, activations,
// gradients, embeddings) between endpoints.
type TensorTransfer struct {
	ID    [16]byte // Unique tensor transfer identifier
	DType uint8    // Data type code (maps to StrandLink tensor_dtype)
	Shape []uint32 // Tensor dimensions
	Data  []byte   // Raw tensor data
}

// Encode serialises the TensorTransfer into buf.
func (m *TensorTransfer) Encode(buf *strandbuf.Buffer) {
	for i := 0; i < 16; i++ {
		buf.WriteUint8(m.ID[i])
	}
	buf.WriteUint8(m.DType)
	buf.WriteList(uint32(len(m.Shape)))
	for _, d := range m.Shape {
		buf.WriteUint32(d)
	}
	buf.WriteBytes(m.Data)
}

// Decode reads a TensorTransfer from r.
func (m *TensorTransfer) Decode(r *strandbuf.Reader) error {
	for i := 0; i < 16; i++ {
		b, err := r.ReadUint8()
		if err != nil {
			return err
		}
		m.ID[i] = b
	}
	var err error
	m.DType, err = r.ReadUint8()
	if err != nil {
		return err
	}
	count, err := r.ReadList()
	if err != nil {
		return err
	}
	// Cap to prevent allocation-bomb DoS.
	if count > maxShapeDimensions {
		return fmt.Errorf("strandapi: shape dimension count %d exceeds max %d", count, maxShapeDimensions)
	}
	m.Shape = make([]uint32, count)
	for i := uint32(0); i < count; i++ {
		m.Shape[i], err = r.ReadUint32()
		if err != nil {
			return err
		}
	}
	m.Data, err = r.ReadBytes()
	if err != nil {
		return err
	}
	// Copy data out of reader buffer to avoid aliasing
	dataCopy := make([]byte, len(m.Data))
	copy(dataCopy, m.Data)
	m.Data = dataCopy
	return nil
}
