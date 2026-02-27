/**
 * strandlink.h — C FFI header for StrandLink L1 Frame Protocol
 *
 * Provides C-compatible function declarations for StrandLink frame encoding/decoding,
 * ring buffer operations, and CRC-32C computation. Link against the strandlink
 * static/shared library built by the Zig build system.
 *
 * All multi-byte fields use big-endian (network byte order) on the wire.
 */

#ifndef STRANDLINK_H
#define STRANDLINK_H

#include <stdint.h>
#include <stddef.h>

#ifdef __cplusplus
extern "C" {
#endif

/* ── Constants ── */

#define STRANDLINK_HEADER_SIZE      64
#define STRANDLINK_MAX_OPTIONS_SIZE 256
#define STRANDLINK_MAX_FRAME_SIZE   65535
#define STRANDLINK_MIN_FRAME_SIZE   68   /* header(64) + CRC(4) */
#define STRANDLINK_CRC_SIZE         4
#define STRANDLINK_VERSION          1

#define STRANDLINK_OVERLAY_MAGIC    0x504C
#define STRANDLINK_OVERLAY_PORT     6477
#define STRANDLINK_OVERLAY_HDR_SIZE 8

/* ── Frame types ── */

#define STRANDLINK_FRAME_DATA               0x0001
#define STRANDLINK_FRAME_CONTROL            0x0002
#define STRANDLINK_FRAME_HEARTBEAT          0x0003
#define STRANDLINK_FRAME_ROUTE_ADVERTISEMENT 0x0004
#define STRANDLINK_FRAME_TRUST_HANDSHAKE    0x0005
#define STRANDLINK_FRAME_TENSOR_TRANSFER    0x0006
#define STRANDLINK_FRAME_STREAM_CONTROL     0x0007

/* ── Option types ── */

#define STRANDLINK_OPT_FRAGMENT_INFO  0x01
#define STRANDLINK_OPT_COMPRESSION_ALG 0x02
#define STRANDLINK_OPT_ENCRYPTION_TAG 0x03
#define STRANDLINK_OPT_TENSOR_SHAPE   0x04
#define STRANDLINK_OPT_TRACE_ID       0x05
#define STRANDLINK_OPT_HOP_COUNT      0x06
#define STRANDLINK_OPT_SEMANTIC_ADDR  0x07
#define STRANDLINK_OPT_GPU_HINT       0x08

/* ── Opaque types ── */

typedef struct strandlink_ring_buffer strandlink_ring_buffer_t;

/* ── Frame operations ── */

/**
 * Encode a StrandLink frame.
 *
 * @param hdr_buf        Pointer to a serialized 64-byte frame header
 * @param options        Pointer to TLV-encoded options (may be NULL)
 * @param options_len    Length of options in bytes
 * @param payload        Pointer to payload data (may be NULL)
 * @param payload_len    Length of payload in bytes
 * @param out_buf        Output buffer for the encoded frame
 * @param out_buf_len    Size of the output buffer
 * @param out_frame_len  [out] Actual length of the encoded frame
 *
 * @return 0 on success, negative on error (-1 = invalid header, -2 = buffer too small)
 */
int strandlink_frame_encode(
    const uint8_t *hdr_buf,
    const uint8_t *options,
    uint16_t options_len,
    const uint8_t *payload,
    uint32_t payload_len,
    uint8_t *out_buf,
    uint32_t out_buf_len,
    uint32_t *out_frame_len
);

/**
 * Decode a StrandLink frame.
 *
 * @param buf             Input buffer containing the encoded frame
 * @param buf_len         Length of the input buffer
 * @param out_header_buf  [out] Buffer to receive the 64-byte serialized header
 * @param out_payload_ptr [out] Pointer to payload within the input buffer
 * @param out_payload_len [out] Length of the payload
 *
 * @return 0 on success, negative on error (-1 = decode error, -2 = header serialize error)
 */
int strandlink_frame_decode(
    const uint8_t *buf,
    uint32_t buf_len,
    uint8_t *out_header_buf,
    const uint8_t **out_payload_ptr,
    uint32_t *out_payload_len
);

/* ── Ring buffer operations ── */

/**
 * Create a new ring buffer.
 *
 * @param num_slots  Number of slots (must be a power of 2)
 * @param slot_size  Size of each slot in bytes
 * @return Opaque ring buffer pointer, or NULL on failure
 */
strandlink_ring_buffer_t *strandlink_ring_buffer_create(uint32_t num_slots, uint32_t slot_size);

/**
 * Destroy a ring buffer and free its resources.
 */
void strandlink_ring_buffer_destroy(strandlink_ring_buffer_t *rb);

/**
 * Reserve a slot for writing.
 * @return Pointer to the slot buffer, or NULL if the ring is full
 */
uint8_t *strandlink_ring_buffer_reserve(strandlink_ring_buffer_t *rb);

/**
 * Commit a previously reserved slot, making it visible to the consumer.
 */
void strandlink_ring_buffer_commit(strandlink_ring_buffer_t *rb);

/**
 * Peek at the next readable slot.
 * @return Pointer to the slot buffer, or NULL if the ring is empty
 */
const uint8_t *strandlink_ring_buffer_peek(strandlink_ring_buffer_t *rb);

/**
 * Release a consumed slot back to the ring.
 */
void strandlink_ring_buffer_release(strandlink_ring_buffer_t *rb);

/* ── Utility ── */

/**
 * Compute CRC-32C (Castagnoli) over a buffer.
 *
 * @param data  Pointer to data
 * @param len   Length of data in bytes
 * @return CRC-32C checksum
 */
uint32_t strandlink_crc32c(const uint8_t *data, uint32_t len);

#ifdef __cplusplus
}
#endif

#endif /* STRANDLINK_H */
