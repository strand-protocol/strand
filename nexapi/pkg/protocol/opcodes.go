// Package protocol defines the NexAPI wire protocol: opcodes, message types,
// and framing for AI-native application communication.
package protocol

// Opcode constants identify NexAPI message types on the wire.
const (
	OpInferenceRequest  byte = 0x01
	OpInferenceResponse byte = 0x02
	OpTokenStreamStart  byte = 0x03
	OpTokenStreamChunk  byte = 0x04
	OpTokenStreamEnd    byte = 0x05
	OpTensorTransfer    byte = 0x06
	OpAgentNegotiation  byte = 0x07 // Kept for backwards compatibility; prefer OpAgentNegotiate.
	OpHeartbeat         byte = 0x08
	// Agent delegation opcodes (spec §2.5, message types 0x000B–0x000D).
	// Within the single-byte opcode space used by this framing layer they are
	// assigned the next available values after 0x08.
	OpAgentNegotiate byte = 0x09 // AGENT_NEGOTIATE  — capability exchange proposal
	OpAgentDelegate  byte = 0x0A // AGENT_DELEGATE   — delegate a task to a peer
	OpAgentResult    byte = 0x0B // AGENT_RESULT     — result of a delegated task
	OpError          byte = 0xFF
)

// OpcodeNames maps opcodes to human-readable names for logging and diagnostics.
var OpcodeNames = map[byte]string{
	OpInferenceRequest:  "INFERENCE_REQUEST",
	OpInferenceResponse: "INFERENCE_RESPONSE",
	OpTokenStreamStart:  "TOKEN_STREAM_START",
	OpTokenStreamChunk:  "TOKEN_STREAM_CHUNK",
	OpTokenStreamEnd:    "TOKEN_STREAM_END",
	OpTensorTransfer:    "TENSOR_TRANSFER",
	OpAgentNegotiation:  "AGENT_NEGOTIATION",
	OpHeartbeat:         "HEARTBEAT",
	OpAgentNegotiate:    "AGENT_NEGOTIATE",
	OpAgentDelegate:     "AGENT_DELEGATE",
	OpAgentResult:       "AGENT_RESULT",
	OpError:             "ERROR",
}
