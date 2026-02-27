/*
 * sad_forward.p4 - NexRoute P4_16 compilation entry point
 *
 * This file is the top-level include aggregator for the NexRoute P4 pipeline.
 * It includes all component files in dependency order and serves as the single
 * target passed to the P4 compiler (p4c / BMv2).
 *
 * Compilation:
 *   # For BMv2 behavioral model:
 *   p4c --target bmv2 --arch v1model --std p4-16 sad_forward.p4 -o sad_forward.json
 *
 *   # For Tofino (future):
 *   p4c --target tofino --arch tna sad_forward.p4
 *
 * Pipeline overview:
 *   Parser    : Ethernet -> NexLink (64B) -> SAD header + first 3 fields
 *   Ingress   : SADLookup (ternary + exact TCAM) -> NexRouteForwarding
 *   Egress    : NexRouteEgress (placeholder for QoS / mirroring)
 *   Deparser  : Reassemble headers on wire
 *
 * File dependency graph:
 *   sad_forward.p4       (this file)
 *     headers.p4         -- all struct/header type definitions
 *     parser.p4          -- includes headers.p4; defines Parser + Deparser
 *     sad_lookup.p4      -- includes headers.p4; defines SADLookup control
 *     forwarding.p4      -- includes headers.p4; defines NexRouteForwarding,
 *                           NexRouteIngress, NexRouteEgress,
 *                           NexRouteVerifyChecksum, NexRouteComputeChecksum,
 *                           and the V1Switch main instantiation.
 *
 * Note: forwarding.p4 already contains the V1Switch main declaration, so
 * this file is purely an include aggregator — no additional declarations
 * are needed here.
 */

/* v1model architecture provided by the BMv2 / p4c backend */
#include <core.p4>
#include <v1model.p4>

/*
 * Include component files in dependency order.
 *
 * headers.p4 is included first (directly) so that its include-guard
 * prevents duplicate definitions if any subsequent file also includes it.
 */
#include "headers.p4"
#include "parser.p4"
#include "sad_lookup.p4"
#include "forwarding.p4"

/*
 * EOF: forwarding.p4 contains the V1Switch instantiation:
 *
 *   V1Switch(
 *       NexRouteParser(),
 *       NexRouteVerifyChecksum(),
 *       NexRouteIngress(),
 *       NexRouteEgress(),
 *       NexRouteComputeChecksum(),
 *       NexRouteDeparser()
 *   ) main;
 *
 * No further declarations are required in this file.
 */
