/*
 * forwarding.p4 - P4_16 StrandRoute forwarding pipeline
 *
 * After SAD lookup resolves a destination node_id, this control block
 * looks up the egress port for that node_id and rewrites the StrandLink
 * header.
 *
 * Tables:
 *   1. node_id_forward: exact match on dst_node_id -> egress port
 *   2. Per-stream counters for packet/byte statistics
 */

#ifndef __STRANDROUTE_FORWARDING_P4__
#define __STRANDROUTE_FORWARDING_P4__

#include "headers.p4"

/* --------------------------------------------------------------------------
 * Forwarding Control Block
 * -------------------------------------------------------------------------- */

control StrandRouteForwarding(inout headers_t hdr,
                           inout strandroute_metadata_t meta,
                           inout standard_metadata_t standard_metadata) {

    /* ---- Counters ---- */
    counter(1024, CounterType.packets_and_bytes) stream_counter;

    /* ---- Actions ---- */

    action forward_to_port(bit<9> port) {
        standard_metadata.egress_spec = port;
        /* Rewrite destination node ID if SAD resolution provided one */
        if (meta.do_forward == 1) {
            hdr.strandlink.dst_node_id = meta.resolved_node_id;
        }
        /* Decrement TTL */
        hdr.strandlink.ttl = hdr.strandlink.ttl - 1;
    }

    action drop_frame() {
        mark_to_drop(standard_metadata);
    }

    action send_to_controller() {
        standard_metadata.egress_spec = 64;  /* CPU port */
    }

    /* ---- Node ID forwarding table ---- */
    /*
     * Exact match on the 128-bit destination node ID.
     * Populated by the control plane from the StrandLink neighbor table.
     * Maps node_id -> physical egress port.
     */
    table node_id_forward {
        key = {
            hdr.strandlink.dst_node_id : exact;
        }
        actions = {
            forward_to_port;
            send_to_controller;
            drop_frame;
        }
        size = 65536;
        default_action = send_to_controller();
    }

    /* ---- Apply logic ---- */
    apply {
        /* Drop if explicitly marked or TTL expired */
        if (meta.do_drop == 1 || hdr.strandlink.ttl == 0) {
            drop_frame();
            return;
        }

        /* If SAD resolution gave us a node_id, overwrite dst before lookup */
        if (meta.do_forward == 1) {
            hdr.strandlink.dst_node_id = meta.resolved_node_id;
        }

        /* Look up egress port for the destination node ID */
        node_id_forward.apply();

        /* Update per-stream counter */
        stream_counter.count((bit<32>)hdr.strandlink.stream_id[31:0]);
    }
}

/* --------------------------------------------------------------------------
 * Egress processing (minimal - just emit)
 * -------------------------------------------------------------------------- */

control StrandRouteEgress(inout headers_t hdr,
                       inout strandroute_metadata_t meta,
                       inout standard_metadata_t standard_metadata) {
    apply {
        /* Future: per-port QoS, mirroring, etc. */
    }
}

/* --------------------------------------------------------------------------
 * Checksum verification / computation (placeholder)
 * -------------------------------------------------------------------------- */

control StrandRouteVerifyChecksum(inout headers_t hdr,
                               inout strandroute_metadata_t meta) {
    apply {
        /* StrandLink does not use L3 checksums (L2-only) */
    }
}

control StrandRouteComputeChecksum(inout headers_t hdr,
                                inout strandroute_metadata_t meta) {
    apply {
        /* No checksums to recompute */
    }
}

/* --------------------------------------------------------------------------
 * Top-level ingress pipeline: SAD lookup -> forwarding
 * -------------------------------------------------------------------------- */

control StrandRouteIngress(inout headers_t hdr,
                        inout strandroute_metadata_t meta,
                        inout standard_metadata_t standard_metadata) {

    SADLookup()          sad_lookup;
    StrandRouteForwarding() forwarding;

    apply {
        /* Stage 1: SAD resolution (if SAD present) */
        sad_lookup.apply(hdr, meta, standard_metadata);

        /* Stage 2: Node ID -> port forwarding */
        forwarding.apply(hdr, meta, standard_metadata);
    }
}

/* --------------------------------------------------------------------------
 * V1Model switch instantiation
 * -------------------------------------------------------------------------- */

V1Switch(
    StrandRouteParser(),
    StrandRouteVerifyChecksum(),
    StrandRouteIngress(),
    StrandRouteEgress(),
    StrandRouteComputeChecksum(),
    StrandRouteDeparser()
) main;

#endif /* __STRANDROUTE_FORWARDING_P4__ */
