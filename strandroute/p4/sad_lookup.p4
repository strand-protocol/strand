/*
 * sad_lookup.p4 - P4_16 SAD field matching via TCAM tables
 *
 * Two tables:
 *   1. sad_ternary_match: ternary match on (model_arch, capability_flags,
 *      context_window) -> resolved_node_id.  Populated by control plane
 *      from the routing table.
 *   2. sad_exact_match:   exact match on full (model_arch, capability)
 *      for common hot-path queries.
 *
 * The control plane pre-computes match entries from the routing table
 * and installs them in TCAM.  This allows line-rate SAD resolution
 * for the most common SAD queries.  Uncommon queries are sent to the
 * CPU via digest for software resolution.
 */

#ifndef __STRANDROUTE_SAD_LOOKUP_P4__
#define __STRANDROUTE_SAD_LOOKUP_P4__

#include "headers.p4"

/* --------------------------------------------------------------------------
 * SAD Lookup Control Block
 * -------------------------------------------------------------------------- */

control SADLookup(inout headers_t hdr,
                  inout strandroute_metadata_t meta,
                  inout standard_metadata_t standard_metadata) {

    /* ---- Actions ---- */

    action set_resolved_node(bit<128> node_id) {
        meta.resolved_node_id = node_id;
        meta.do_forward = 1;
    }

    action send_to_cpu() {
        /* Clone/digest to CPU for software-path resolution */
        standard_metadata.egress_spec = 64;  /* CPU port */
        meta.do_forward = 0;
    }

    action sad_drop() {
        meta.do_drop = 1;
        meta.do_forward = 0;
    }

    /* ---- Ternary match table ---- */
    /*
     * Ternary match allows wildcard masks on each field:
     *   model_arch = 0x01 &&& 0xFF   -> exact match on transformer
     *   capability = 0x03 &&& 0x03   -> must have text_gen + code_gen (other bits don't care)
     *   context_window >= threshold   -> encoded as ternary range
     *
     * Control plane populates this from pre-resolved SAD->node_id mappings.
     */
    table sad_ternary_match {
        key = {
            meta.model_arch       : ternary;
            meta.capability_flags : ternary;
            meta.context_window   : ternary;
        }
        actions = {
            set_resolved_node;
            send_to_cpu;
            sad_drop;
        }
        size = 4096;
        default_action = send_to_cpu();
    }

    /* ---- Exact match table for common queries ---- */
    table sad_exact_match {
        key = {
            meta.model_arch       : exact;
            meta.capability_flags : exact;
        }
        actions = {
            set_resolved_node;
            send_to_cpu;
        }
        size = 1024;
        default_action = send_to_cpu();
    }

    /* ---- Apply logic ---- */
    apply {
        if (meta.has_sad == 1) {
            /* Try exact match first (faster, smaller table) */
            if (!sad_exact_match.apply().hit) {
                /* Fall back to ternary match */
                sad_ternary_match.apply();
            }
        }
    }
}

#endif /* __STRANDROUTE_SAD_LOOKUP_P4__ */
