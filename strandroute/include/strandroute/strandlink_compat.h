/*
 * strandlink_compat.h - Minimal StrandLink stub types for StrandRoute
 *
 * This header provides the subset of StrandLink types that StrandRoute depends on.
 * It will be replaced by the real strandlink/strandlink.h once StrandLink is built.
 */

#ifndef STRANDLINK_COMPAT_H
#define STRANDLINK_COMPAT_H

#include <stdint.h>
#include <stddef.h>

#ifdef __cplusplus
extern "C" {
#endif

/* StrandLink Node ID - 16 bytes (128-bit) as specified in the routing table */
#define STRANDLINK_NODE_ID_LEN  16

typedef struct {
    uint8_t bytes[STRANDLINK_NODE_ID_LEN];
} strandlink_node_id_t;

/* StrandLink frame types */
typedef enum {
    STRANDLINK_FRAME_DATA       = 0x01,
    STRANDLINK_FRAME_CONTROL    = 0x02,
    STRANDLINK_FRAME_HEARTBEAT  = 0x03,
    STRANDLINK_FRAME_DISCOVERY  = 0x04,
    STRANDLINK_FRAME_GOSSIP     = 0x10,
} strandlink_frame_type_t;

/* StrandLink frame header - 64 bytes fixed header */
typedef struct __attribute__((packed)) {
    uint8_t  version;                              /*  1 byte  */
    uint8_t  frame_type;                           /*  1 byte  */
    uint16_t payload_length;                       /*  2 bytes */
    uint32_t sequence;                             /*  4 bytes */
    uint8_t  src_node_id[STRANDLINK_NODE_ID_LEN];     /* 16 bytes */
    uint8_t  dst_node_id[STRANDLINK_NODE_ID_LEN];     /* 16 bytes */
    uint8_t  stream_id[8];                         /*  8 bytes */
    uint16_t options_offset;                       /*  2 bytes */
    uint16_t options_length;                       /*  2 bytes */
    uint8_t  ttl;                                  /*  1 byte  */
    uint8_t  priority;                             /*  1 byte  */
    uint8_t  flags;                                /*  1 byte  */
    uint8_t  _reserved[9];                         /*  9 bytes */
    /* Total: 64 bytes */
} strandlink_frame_header_t;

_Static_assert(sizeof(strandlink_frame_header_t) == 64,
               "StrandLink frame header must be exactly 64 bytes");

/* StrandLink frame with header + payload buffer */
#define STRANDLINK_MAX_FRAME_SIZE 9216

typedef struct {
    strandlink_frame_header_t header;
    uint8_t                payload[STRANDLINK_MAX_FRAME_SIZE - 64];
} strandlink_frame_t;

/* Port abstraction for forwarding */
typedef uint16_t strandlink_port_t;

#define STRANDLINK_PORT_INVALID  ((strandlink_port_t)0xFFFF)

/* Stub send/receive callbacks (to be provided by StrandLink integration) */
typedef int (*strandlink_send_fn)(strandlink_port_t port,
                               const strandlink_frame_t *frame,
                               void *ctx);

typedef int (*strandlink_recv_fn)(strandlink_port_t port,
                               strandlink_frame_t *frame,
                               void *ctx);

#ifdef __cplusplus
}
#endif

#endif /* STRANDLINK_COMPAT_H */
