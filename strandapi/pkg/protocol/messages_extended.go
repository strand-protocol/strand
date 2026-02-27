package protocol

import (
	"fmt"

	"github.com/strand-protocol/strand/strandapi/pkg/strandbuf"
)

// Maximum context data size (4 MiB) to prevent allocation bombs.
const maxContextDataSize = 4 << 20

// ContextShare transfers multi-turn conversation context to a server so it
// can be cached and reused across subsequent inference requests.
//
// Wire layout (StrandBuf):
//
//	[16 bytes] RequestID
//	[bytes]    ContextData  (length-prefixed opaque context blob)
type ContextShare struct {
	RequestID   [16]byte
	ContextData []byte
}

func (m *ContextShare) Encode(buf *strandbuf.Buffer) {
	for i := 0; i < 16; i++ {
		buf.WriteUint8(m.RequestID[i])
	}
	buf.WriteBytes(m.ContextData)
}

func (m *ContextShare) Decode(r *strandbuf.Reader) error {
	for i := 0; i < 16; i++ {
		b, err := r.ReadUint8()
		if err != nil {
			return err
		}
		m.RequestID[i] = b
	}
	data, err := r.ReadBytes()
	if err != nil {
		return err
	}
	if len(data) > maxContextDataSize {
		return fmt.Errorf("strandapi: context data %d bytes exceeds max %d", len(data), maxContextDataSize)
	}
	m.ContextData = make([]byte, len(data))
	copy(m.ContextData, data)
	return nil
}

// ContextAck acknowledges that a shared context has been cached.
//
// Wire layout (StrandBuf):
//
//	[16 bytes] RequestID
type ContextAck struct {
	RequestID [16]byte
}

func (m *ContextAck) Encode(buf *strandbuf.Buffer) {
	for i := 0; i < 16; i++ {
		buf.WriteUint8(m.RequestID[i])
	}
}

func (m *ContextAck) Decode(r *strandbuf.Reader) error {
	for i := 0; i < 16; i++ {
		b, err := r.ReadUint8()
		if err != nil {
			return err
		}
		m.RequestID[i] = b
	}
	return nil
}

// ToolInvoke is sent by a model to request the client to execute a tool.
//
// Wire layout (StrandBuf):
//
//	[16 bytes] RequestID
//	[string]   ToolName    (length-prefixed tool identifier)
//	[bytes]    Arguments   (length-prefixed JSON or opaque arguments)
type ToolInvoke struct {
	RequestID [16]byte
	ToolName  string
	Arguments []byte
}

func (m *ToolInvoke) Encode(buf *strandbuf.Buffer) {
	for i := 0; i < 16; i++ {
		buf.WriteUint8(m.RequestID[i])
	}
	buf.WriteString(m.ToolName)
	buf.WriteBytes(m.Arguments)
}

func (m *ToolInvoke) Decode(r *strandbuf.Reader) error {
	for i := 0; i < 16; i++ {
		b, err := r.ReadUint8()
		if err != nil {
			return err
		}
		m.RequestID[i] = b
	}
	var err error
	m.ToolName, err = r.ReadString()
	if err != nil {
		return err
	}
	args, err := r.ReadBytes()
	if err != nil {
		return err
	}
	m.Arguments = make([]byte, len(args))
	copy(m.Arguments, args)
	return nil
}

// ToolResult carries the result of a tool execution back to the server.
//
// Wire layout (StrandBuf):
//
//	[16 bytes] RequestID
//	[bytes]    ResultPayload  (length-prefixed result data)
//	[uint16]   ErrorCode      (0x0000 = success)
type ToolResult struct {
	RequestID     [16]byte
	ResultPayload []byte
	ErrorCode     uint16
}

func (m *ToolResult) Encode(buf *strandbuf.Buffer) {
	for i := 0; i < 16; i++ {
		buf.WriteUint8(m.RequestID[i])
	}
	buf.WriteBytes(m.ResultPayload)
	buf.WriteUint16(m.ErrorCode)
}

func (m *ToolResult) Decode(r *strandbuf.Reader) error {
	for i := 0; i < 16; i++ {
		b, err := r.ReadUint8()
		if err != nil {
			return err
		}
		m.RequestID[i] = b
	}
	payload, err := r.ReadBytes()
	if err != nil {
		return err
	}
	m.ResultPayload = make([]byte, len(payload))
	copy(m.ResultPayload, payload)
	m.ErrorCode, err = r.ReadUint16()
	return err
}

// HealthCheck is a lightweight probe sent to verify a node is alive.
//
// Wire layout (StrandBuf):
//
//	[16 bytes] NodeID
type HealthCheck struct {
	NodeID [16]byte
}

func (m *HealthCheck) Encode(buf *strandbuf.Buffer) {
	for i := 0; i < 16; i++ {
		buf.WriteUint8(m.NodeID[i])
	}
}

func (m *HealthCheck) Decode(r *strandbuf.Reader) error {
	for i := 0; i < 16; i++ {
		b, err := r.ReadUint8()
		if err != nil {
			return err
		}
		m.NodeID[i] = b
	}
	return nil
}

// HealthStatus is the response to a HealthCheck probe.
//
// Wire layout (StrandBuf):
//
//	[16 bytes] NodeID
//	[uint8]    Status   (0=unknown, 1=healthy, 2=degraded, 3=unhealthy)
//	[uint64]   Uptime   (seconds since node started)
type HealthStatus struct {
	NodeID [16]byte
	Status uint8
	Uptime uint64
}

func (m *HealthStatus) Encode(buf *strandbuf.Buffer) {
	for i := 0; i < 16; i++ {
		buf.WriteUint8(m.NodeID[i])
	}
	buf.WriteUint8(m.Status)
	buf.WriteUint64(m.Uptime)
}

func (m *HealthStatus) Decode(r *strandbuf.Reader) error {
	for i := 0; i < 16; i++ {
		b, err := r.ReadUint8()
		if err != nil {
			return err
		}
		m.NodeID[i] = b
	}
	var err error
	m.Status, err = r.ReadUint8()
	if err != nil {
		return err
	}
	m.Uptime, err = r.ReadUint64()
	return err
}

// Cancel requests cancellation of an in-flight request.
//
// Wire layout (StrandBuf):
//
//	[16 bytes] RequestID
type Cancel struct {
	RequestID [16]byte
}

func (m *Cancel) Encode(buf *strandbuf.Buffer) {
	for i := 0; i < 16; i++ {
		buf.WriteUint8(m.RequestID[i])
	}
}

func (m *Cancel) Decode(r *strandbuf.Reader) error {
	for i := 0; i < 16; i++ {
		b, err := r.ReadUint8()
		if err != nil {
			return err
		}
		m.RequestID[i] = b
	}
	return nil
}
