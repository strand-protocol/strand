/*
 * headers.p4 - P4_16 header definitions for NexRoute
 *
 * Defines NexLink frame header and NexRoute SAD overlay header
 * for programmable switch dataplanes.
 */

#ifndef __NEXROUTE_HEADERS_P4__
#define __NEXROUTE_HEADERS_P4__

/* --------------------------------------------------------------------------
 * Ethernet header (for physical port ingress)
 * -------------------------------------------------------------------------- */

header ethernet_t {
    bit<48> dst_addr;
    bit<48> src_addr;
    bit<16> ether_type;
}

/* EtherType for NexLink frames */
const bit<16> ETHERTYPE_NEXLINK = 0x9100;

/* --------------------------------------------------------------------------
 * NexLink frame header - 64 bytes fixed
 *
 * This must match nexlink_frame_header_t exactly.
 * -------------------------------------------------------------------------- */

header nexlink_t {
    bit<8>   version;
    bit<8>   frame_type;
    bit<16>  payload_length;
    bit<32>  sequence;
    bit<128> src_node_id;
    bit<128> dst_node_id;
    bit<64>  stream_id;
    bit<16>  options_offset;
    bit<16>  options_length;
    bit<8>   ttl;
    bit<8>   priority;
    bit<8>   flags;
    bit<72>  _reserved;     /* 9 bytes = 72 bits */
}

/* --------------------------------------------------------------------------
 * NexRoute SAD header - extracted from NexLink options area
 *
 * The SAD sits inside the NexLink options region.  We parse a fixed
 * header (4 bytes) and then up to 3 "first fields" for TCAM matching.
 * Full SAD parsing for variable-length fields is deferred to the
 * control plane via digest/clone.
 *
 * Wire format:
 *   [version:8][flags:8][num_fields:16]
 *   per field: [type:8][length:16][value:variable]
 *
 * For hardware TCAM we extract the first 3 fields as fixed-width:
 *   field0: MODEL_ARCH  (type=0x01, 4 bytes -> 32 bits)
 *   field1: CAPABILITY  (type=0x02, 4 bytes -> 32 bits)
 *   field2: CONTEXT_WINDOW (type=0x03, 4 bytes -> 32 bits)
 * -------------------------------------------------------------------------- */

header nexroute_sad_t {
    bit<8>  version;
    bit<8>  flags;
    bit<16> num_fields;
}

/* Individual SAD field header (parsed in a loop or as fixed positions) */
header sad_field_t {
    bit<8>  field_type;
    bit<16> field_length;
    bit<32> field_value;    /* first 4 bytes of value; sufficient for
                               MODEL_ARCH, CAPABILITY, CONTEXT_WINDOW,
                               LATENCY, COST fields */
}

/* --------------------------------------------------------------------------
 * Metadata for NexRoute processing
 * -------------------------------------------------------------------------- */

struct nexroute_metadata_t {
    bit<1>   has_sad;
    bit<32>  model_arch;
    bit<32>  capability_flags;
    bit<32>  context_window;
    bit<128> resolved_node_id;
    bit<9>   egress_port;
    bit<1>   do_forward;
    bit<1>   do_drop;
}

/* --------------------------------------------------------------------------
 * All headers struct
 * -------------------------------------------------------------------------- */

struct headers_t {
    ethernet_t       ethernet;
    nexlink_t        nexlink;
    nexroute_sad_t   sad_header;
    sad_field_t      sad_field0;    /* MODEL_ARCH */
    sad_field_t      sad_field1;    /* CAPABILITY */
    sad_field_t      sad_field2;    /* CONTEXT_WINDOW */
}

#endif /* __NEXROUTE_HEADERS_P4__ */
