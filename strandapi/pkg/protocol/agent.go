package protocol

import (
	"fmt"

	"github.com/strand-protocol/strand/strandapi/pkg/strandbuf"
)

// maxCapabilities caps the number of capability strings in an AgentNegotiate
// message, preventing allocation-bomb DoS from a malicious peer.
const maxCapabilities = 128

// AgentNegotiate is sent to propose a capability exchange with a peer agent.
// Both sides send this message at the start of a delegation session so each
// knows what the other can do before any work is delegated.
//
// Wire layout (StrandBuf):
//
//	[uint32] SessionID
//	[uint32] capability count
//	[string] Capabilities[0..n-1]  (each length-prefixed)
//	[uint8]  Version
type AgentNegotiate struct {
	SessionID    uint32   // Identifies the delegation session across messages
	Capabilities []string // List of capability identifiers advertised by this agent
	Version      uint8    // Protocol version spoken by the sender (currently 1)
}

// Encode serialises AgentNegotiate into buf using StrandBuf wire format.
func (m *AgentNegotiate) Encode(buf *strandbuf.Buffer) {
	buf.WriteUint32(m.SessionID)
	buf.WriteList(uint32(len(m.Capabilities)))
	for _, c := range m.Capabilities {
		buf.WriteString(c)
	}
	buf.WriteUint8(m.Version)
}

// Decode reads an AgentNegotiate from r. Returns an error if the data is
// incomplete or malformed.
func (m *AgentNegotiate) Decode(r *strandbuf.Reader) error {
	var err error
	m.SessionID, err = r.ReadUint32()
	if err != nil {
		return err
	}
	count, err := r.ReadList()
	if err != nil {
		return err
	}
	// Cap to prevent allocation-bomb DoS.
	if count > maxCapabilities {
		return fmt.Errorf("strandapi: capability count %d exceeds max %d", count, maxCapabilities)
	}
	m.Capabilities = make([]string, count)
	for i := uint32(0); i < count; i++ {
		m.Capabilities[i], err = r.ReadString()
		if err != nil {
			return err
		}
	}
	m.Version, err = r.ReadUint8()
	return err
}

// AgentDelegate delegates a task (payload) to a target node/agent. The sender
// transfers ownership of the described work to the node identified by
// TargetNodeID.
//
// Wire layout (StrandBuf):
//
//	[uint32]   SessionID
//	[16 bytes] TargetNodeID  (raw 128-bit node identifier)
//	[bytes]    TaskPayload   (length-prefixed opaque task data)
//	[uint32]   TimeoutMS     (0 = no timeout)
type AgentDelegate struct {
	SessionID    uint32   // Delegation session identifier
	TargetNodeID [16]byte // 128-bit StrandLink node ID of the target agent
	TaskPayload  []byte   // Opaque task encoding (caller-defined serialisation)
	TimeoutMS    uint32   // Deadline in milliseconds; 0 means no deadline
}

// Encode serialises AgentDelegate into buf using StrandBuf wire format.
func (m *AgentDelegate) Encode(buf *strandbuf.Buffer) {
	buf.WriteUint32(m.SessionID)
	for i := 0; i < 16; i++ {
		buf.WriteUint8(m.TargetNodeID[i])
	}
	buf.WriteBytes(m.TaskPayload)
	buf.WriteUint32(m.TimeoutMS)
}

// Decode reads an AgentDelegate from r.
func (m *AgentDelegate) Decode(r *strandbuf.Reader) error {
	var err error
	m.SessionID, err = r.ReadUint32()
	if err != nil {
		return err
	}
	for i := 0; i < 16; i++ {
		m.TargetNodeID[i], err = r.ReadUint8()
		if err != nil {
			return err
		}
	}
	payload, err := r.ReadBytes()
	if err != nil {
		return err
	}
	// Copy out of reader buffer to avoid aliasing after the reader is discarded.
	m.TaskPayload = make([]byte, len(payload))
	copy(m.TaskPayload, payload)
	m.TimeoutMS, err = r.ReadUint32()
	return err
}

// AgentResult carries the result of a delegated task back to the originating
// agent. ErrorCode is zero on success.
//
// Wire layout (StrandBuf):
//
//	[uint32] SessionID
//	[bytes]  ResultPayload  (length-prefixed opaque result data; may be empty)
//	[uint16] ErrorCode      (0x0000 = success; see Err* constants in errors.go)
//	[string] ErrorMsg       (empty on success; human-readable detail on failure)
type AgentResult struct {
	SessionID     uint32 // Matches the SessionID from the corresponding AgentDelegate
	ResultPayload []byte // Opaque result encoding (caller-defined serialisation)
	ErrorCode     uint16 // Zero on success; one of the ErrXxx protocol error codes
	ErrorMsg      string // Human-readable error detail; empty on success
}

// Encode serialises AgentResult into buf using StrandBuf wire format.
func (m *AgentResult) Encode(buf *strandbuf.Buffer) {
	buf.WriteUint32(m.SessionID)
	buf.WriteBytes(m.ResultPayload)
	buf.WriteUint16(m.ErrorCode)
	buf.WriteString(m.ErrorMsg)
}

// Decode reads an AgentResult from r.
func (m *AgentResult) Decode(r *strandbuf.Reader) error {
	var err error
	m.SessionID, err = r.ReadUint32()
	if err != nil {
		return err
	}
	result, err := r.ReadBytes()
	if err != nil {
		return err
	}
	// Copy out of reader buffer to avoid aliasing.
	m.ResultPayload = make([]byte, len(result))
	copy(m.ResultPayload, result)
	m.ErrorCode, err = r.ReadUint16()
	if err != nil {
		return err
	}
	m.ErrorMsg, err = r.ReadString()
	return err
}
