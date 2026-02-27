// Package nexbuf implements the NexBuf binary serialization format for the
// NexAPI protocol. It provides a compact, little-endian, length-prefixed wire
// format inspired by FlatBuffers and CBOR.
package nexbuf

// Wire type constants identify the data type of each encoded field.
const (
	TypeUint8   byte = 1
	TypeUint16  byte = 2
	TypeUint32  byte = 3
	TypeUint64  byte = 4
	TypeFloat32 byte = 5
	TypeFloat64 byte = 6
	TypeString  byte = 7
	TypeBytes   byte = 8
	TypeList    byte = 9
	TypeMap     byte = 10
)
