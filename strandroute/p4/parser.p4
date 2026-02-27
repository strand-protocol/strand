/*
 * parser.p4 - P4_16 parser for StrandLink frames with StrandRoute SAD extraction
 *
 * Parses:
 *   Ethernet -> StrandLink (64B fixed) -> SAD header + first 3 fields
 */

#ifndef __STRANDROUTE_PARSER_P4__
#define __STRANDROUTE_PARSER_P4__

#include "headers.p4"

/* --------------------------------------------------------------------------
 * StrandLink frame type constants
 * -------------------------------------------------------------------------- */

const bit<8> STRANDLINK_FRAME_DATA      = 0x01;
const bit<8> STRANDLINK_FRAME_CONTROL   = 0x02;
const bit<8> STRANDLINK_FRAME_HEARTBEAT = 0x03;
const bit<8> STRANDLINK_FRAME_DISCOVERY = 0x04;
const bit<8> STRANDLINK_FRAME_GOSSIP    = 0x10;

/* SAD field type constants */
const bit<8> SAD_FIELD_MODEL_ARCH     = 0x01;
const bit<8> SAD_FIELD_CAPABILITY     = 0x02;
const bit<8> SAD_FIELD_CONTEXT_WINDOW = 0x03;

/* --------------------------------------------------------------------------
 * Parser
 * -------------------------------------------------------------------------- */

parser StrandRouteParser(packet_in packet,
                      out headers_t hdr,
                      inout strandroute_metadata_t meta,
                      inout standard_metadata_t standard_metadata) {

    state start {
        packet.extract(hdr.ethernet);
        transition select(hdr.ethernet.ether_type) {
            ETHERTYPE_STRANDLINK: parse_strandlink;
            default:           accept;
        }
    }

    state parse_strandlink {
        packet.extract(hdr.strandlink);
        transition select(hdr.strandlink.frame_type) {
            STRANDLINK_FRAME_DATA: check_options;
            default:            accept;
        }
    }

    /*
     * Check if the StrandLink frame carries options (SAD data).
     * If options_length > 0, parse the SAD header.
     */
    state check_options {
        transition select(hdr.strandlink.options_length) {
            0:       accept;
            default: parse_sad_header;
        }
    }

    state parse_sad_header {
        packet.extract(hdr.sad_header);
        meta.has_sad = 1;
        transition select(hdr.sad_header.num_fields) {
            0:       accept;
            default: parse_sad_field0;
        }
    }

    /*
     * Parse first SAD field (expected: MODEL_ARCH, 4-byte value).
     * We extract a fixed sad_field_t (type:8, length:16, value:32).
     */
    state parse_sad_field0 {
        packet.extract(hdr.sad_field0);
        meta.model_arch = hdr.sad_field0.field_value;
        transition select(hdr.sad_header.num_fields) {
            1:       accept;
            default: parse_sad_field1;
        }
    }

    /* Second field: CAPABILITY */
    state parse_sad_field1 {
        packet.extract(hdr.sad_field1);
        meta.capability_flags = hdr.sad_field1.field_value;
        transition select(hdr.sad_header.num_fields) {
            2:       accept;
            default: parse_sad_field2;
        }
    }

    /* Third field: CONTEXT_WINDOW */
    state parse_sad_field2 {
        packet.extract(hdr.sad_field2);
        meta.context_window = hdr.sad_field2.field_value;
        transition accept;
    }
}

/* --------------------------------------------------------------------------
 * Deparser - reassemble headers on egress
 * -------------------------------------------------------------------------- */

control StrandRouteDeparser(packet_out packet, in headers_t hdr) {
    apply {
        packet.emit(hdr.ethernet);
        packet.emit(hdr.strandlink);
        packet.emit(hdr.sad_header);
        packet.emit(hdr.sad_field0);
        packet.emit(hdr.sad_field1);
        packet.emit(hdr.sad_field2);
    }
}

#endif /* __STRANDROUTE_PARSER_P4__ */
