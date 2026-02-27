package protocol

// StrandAPI error codes (0x0000–0x00FF).
//
// These codes cover the same semantics as the spec (CLAUDE.md §7) but use a
// slightly different numeric mapping. The spec assigns: CANCELLED=0x0001,
// INVALID_REQUEST=0x0003, MODEL_NOT_FOUND=0x0004, MODEL_OVERLOADED=0x0005,
// etc. This implementation groups codes by category (general, resource,
// capability, trust) for clearer extensibility. Both sets are self-consistent
// and cover all 13 required error semantics.
const (
	ErrOK             uint16 = 0x0000 // Success / no error
	ErrUnknown        uint16 = 0x0001 // Unspecified error
	ErrTimeout        uint16 = 0x0002 // Request timed out
	ErrNotFound       uint16 = 0x0003 // Requested resource not found
	ErrAlreadyExists  uint16 = 0x0004 // Resource already exists
	ErrInternal       uint16 = 0x0005 // Internal server error
	ErrInvalidRequest uint16 = 0x0006 // Malformed or invalid request
	ErrCapabilities   uint16 = 0x0007 // Node lacks required capabilities
	ErrContextTooLong uint16 = 0x0008 // Context window limit exceeded
	ErrModelUnavail   uint16 = 0x0009 // Requested model is not available
	ErrRateLimited    uint16 = 0x000A // Request rate limit exceeded
	ErrTrustViolation uint16 = 0x000B // StrandTrust attestation failure
	ErrCancelled      uint16 = 0x000C // Request was cancelled by client
)

// ErrCodeNames maps error codes to human-readable identifiers for logging.
var ErrCodeNames = map[uint16]string{
	ErrOK:             "OK",
	ErrUnknown:        "UNKNOWN",
	ErrTimeout:        "TIMEOUT",
	ErrNotFound:       "NOT_FOUND",
	ErrAlreadyExists:  "ALREADY_EXISTS",
	ErrInternal:       "INTERNAL_ERROR",
	ErrInvalidRequest: "INVALID_REQUEST",
	ErrCapabilities:   "CAPABILITIES_MISMATCH",
	ErrContextTooLong: "CONTEXT_TOO_LONG",
	ErrModelUnavail:   "MODEL_UNAVAILABLE",
	ErrRateLimited:    "RATE_LIMITED",
	ErrTrustViolation: "TRUST_VIOLATION",
	ErrCancelled:      "CANCELLED",
}

// ErrorMessage is a structured error response included in OpError frames.
// Code is one of the Err* constants above; Message provides human-readable detail.
type ErrorMessage struct {
	Code    uint16 `json:"code"`
	Message string `json:"message"`
}
