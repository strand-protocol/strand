// Package transport provides the StrandAPI transport layer.
//
// # Pure-Go Overlay Transport
//
// The OverlayTransport in overlay.go implements a pure-Go, zero-CGo UDP
// transport for StrandAPI frames. It exists so that StrandAPI works immediately
// with "go get" — no Zig, Rust, or CGo build toolchains required.
//
// The overlay transport currently implements:
//   - Custom 8-byte overlay frame header (2B magic + 1B version + 1B flags + 4B length)
//   - UDP send/recv with context cancellation and deadline support
//   - Magic byte and version validation
//
// Per the full spec (CLAUDE.md §3.6), the overlay path should also implement:
//   - StrandLink 64-byte frame header encode/decode
//   - StrandStream Reliable-Ordered mode (simplified)
//   - StrandTrust handshake + AEAD encryption (Go crypto/ed25519 + golang.org/x/crypto)
//
// These are P2 enhancements. The CGo path (linking libstrandstream + libstrandtrust)
// is the production-optimized path that implements the full spec.
package transport
