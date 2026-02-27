/*
 * nexlink_compat.h - Minimal NexLink stub types for NexRoute
 *
 * This header provides the subset of NexLink types that NexRoute depends on.
 * It will be replaced by the real nexlink/nexlink.h once NexLink is built.
 */

#ifndef NEXLINK_COMPAT_H
#define NEXLINK_COMPAT_H

#include <stdint.h>
#include <stddef.h>

#ifdef __cplusplus
extern "C" {
#endif

/* NexLink Node ID - 16 bytes (128-bit) as specified in the routing table */
#define NEXLINK_NODE_ID_LEN  16

typedef struct {
    uint8_t bytes[NEXLINK_NODE_ID_LEN];
} nexlink_node_id_t;

/* NexLink frame types */
typedef enum {
    NEXLINK_FRAME_DATA       = 0x01,
    NEXLINK_FRAME_CONTROL    = 0x02,
    NEXLINK_FRAME_HEARTBEAT  = 0x03,
    NEXLINK_FRAME_DISCOVERY  = 0x04,
    NEXLINK_FRAME_GOSSIP     = 0x10,
} nexlink_frame_type_t;

/* NexLink frame header - 64 bytes fixed header */
typedef struct __attribute__((packed)) {
    uint8_t  version;                              /*  1 byte  */
    uint8_t  frame_type;                           /*  1 byte  */
    uint16_t payload_length;                       /*  2 bytes */
    uint32_t sequence;                             /*  4 bytes */
    uint8_t  src_node_id[NEXLINK_NODE_ID_LEN];     /* 16 bytes */
    uint8_t  dst_node_id[NEXLINK_NODE_ID_LEN];     /* 16 bytes */
    uint8_t  stream_id[8];                         /*  8 bytes */
    uint16_t options_offset;                       /*  2 bytes */
    uint16_t options_length;                       /*  2 bytes */
    uint8_t  ttl;                                  /*  1 byte  */
    uint8_t  priority;                             /*  1 byte  */
    uint8_t  flags;                                /*  1 byte  */
    uint8_t  _reserved[9];                         /*  9 bytes */
    /* Total: 64 bytes */
} nexlink_frame_header_t;

_Static_assert(sizeof(nexlink_frame_header_t) == 64,
               "NexLink frame header must be exactly 64 bytes");

/* NexLink frame with header + payload buffer */
#define NEXLINK_MAX_FRAME_SIZE 9216

typedef struct {
    nexlink_frame_header_t header;
    uint8_t                payload[NEXLINK_MAX_FRAME_SIZE - 64];
} nexlink_frame_t;

/* Port abstraction for forwarding */
typedef uint16_t nexlink_port_t;

#define NEXLINK_PORT_INVALID  ((nexlink_port_t)0xFFFF)

/* Stub send/receive callbacks (to be provided by NexLink integration) */
typedef int (*nexlink_send_fn)(nexlink_port_t port,
                               const nexlink_frame_t *frame,
                               void *ctx);

typedef int (*nexlink_recv_fn)(nexlink_port_t port,
                               nexlink_frame_t *frame,
                               void *ctx);

#ifdef __cplusplus
}
#endif

#endif /* NEXLINK_COMPAT_H */
