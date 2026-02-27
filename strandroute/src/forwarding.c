/*
 * forwarding.c - Software dataplane forwarding engine
 *
 * Receive a NexLink frame -> extract SAD from options -> resolve via
 * routing table -> select next hop (weighted random from top matches)
 * -> rewrite dst_node_id -> forward via send callback.
 */

#include "nexroute/types.h"
#include "nexroute/sad.h"
#include "nexroute/routing_table.h"

#include <string.h>
#include <stdlib.h>
#include <stdatomic.h>

/* --------------------------------------------------------------------------
 * Forward declarations from other modules
 * -------------------------------------------------------------------------- */

extern int resolver_resolve(const routing_table_t *rt,
                            const sad_t *query,
                            resolve_result_t *results,
                            int max_results);

/* --------------------------------------------------------------------------
 * Forwarding engine state
 * -------------------------------------------------------------------------- */

#define FWD_MAX_NEXT_HOPS 8

typedef struct {
    uint8_t          self_id[NEXLINK_NODE_ID_LEN];
    routing_table_t *routing_table;   /* owned externally */
    nexlink_send_fn  send_fn;
    void            *send_ctx;
    int              max_multipath;   /* top-K results to consider */

    /* Statistics (atomic for lock-free reads) */
    _Atomic uint64_t frames_forwarded;
    _Atomic uint64_t frames_dropped;
    _Atomic uint64_t frames_resolved;
    _Atomic uint64_t resolve_failures;
} forwarding_engine_t;

/* --------------------------------------------------------------------------
 * forwarding_engine_init
 * -------------------------------------------------------------------------- */

void forwarding_engine_init(forwarding_engine_t *eng,
                            const uint8_t self_id[NEXLINK_NODE_ID_LEN],
                            routing_table_t *rt,
                            nexlink_send_fn send_fn,
                            void *send_ctx)
{
    memset(eng, 0, sizeof(*eng));
    node_id_copy(eng->self_id, self_id);
    eng->routing_table = rt;
    eng->send_fn       = send_fn;
    eng->send_ctx      = send_ctx;
    eng->max_multipath = 3;  /* default top-K */
    atomic_init(&eng->frames_forwarded, 0);
    atomic_init(&eng->frames_dropped, 0);
    atomic_init(&eng->frames_resolved, 0);
    atomic_init(&eng->resolve_failures, 0);
}

/* --------------------------------------------------------------------------
 * Simple PRNG for weighted random selection (xorshift32)
 * -------------------------------------------------------------------------- */

static uint32_t fwd_rand_state = 0;

static uint32_t fwd_rand(void)
{
    if (fwd_rand_state == 0)
        fwd_rand_state = 0xCAFEBABE;
    fwd_rand_state ^= fwd_rand_state << 13;
    fwd_rand_state ^= fwd_rand_state >> 17;
    fwd_rand_state ^= fwd_rand_state << 5;
    return fwd_rand_state;
}

/* --------------------------------------------------------------------------
 * Weighted random selection from top-K results
 *
 * Weight for each result is proportional to its match score.
 * -------------------------------------------------------------------------- */

static int select_next_hop(const resolve_result_t *results, int count)
{
    if (count <= 0)  return -1;
    if (count == 1)  return 0;

    /* Sum scores */
    float total = 0.0f;
    for (int i = 0; i < count; i++) {
        total += results[i].score;
    }

    if (total <= 0.0f)
        return 0;

    /* Random value in [0, total) */
    float r = ((float)(fwd_rand() % 10000) / 10000.0f) * total;
    float acc = 0.0f;
    for (int i = 0; i < count; i++) {
        acc += results[i].score;
        if (r < acc)
            return i;
    }

    return count - 1;  /* fallback */
}

/* --------------------------------------------------------------------------
 * Extract SAD from NexLink frame options area
 *
 * The SAD is stored in the frame's options region (pointed to by
 * options_offset and options_length in the header).
 * -------------------------------------------------------------------------- */

static int extract_sad_from_frame(const nexlink_frame_t *frame, sad_t *sad)
{
    const nexlink_frame_header_t *hdr = &frame->header;

    uint16_t opt_off = hdr->options_offset;
    uint16_t opt_len = hdr->options_length;

    if (opt_len == 0)
        return -1;  /* no options / no SAD */

    /* Options are stored in the payload area, offset from start of payload */
    if (opt_off + opt_len > hdr->payload_length)
        return -1;

    const uint8_t *sad_data = &frame->payload[opt_off];
    return sad_decode(sad_data, opt_len, sad);
}

/* --------------------------------------------------------------------------
 * forwarding_engine_process_frame
 *
 * Main forwarding hot path:
 *   1. Extract SAD from frame options
 *   2. Resolve SAD against routing table
 *   3. Select next hop via weighted random
 *   4. Rewrite dst_node_id in frame header
 *   5. Forward via send callback
 *
 * Returns 0 on success, -1 if frame was dropped.
 * -------------------------------------------------------------------------- */

int forwarding_engine_process_frame(forwarding_engine_t *eng,
                                    nexlink_frame_t *frame,
                                    nexlink_port_t ingress_port)
{
    (void)ingress_port;

    if (!eng || !frame)
        return -1;

    /* If the frame is destined for us, do not forward */
    if (node_id_equal(frame->header.dst_node_id, eng->self_id))
        return 0;

    /* Check TTL */
    if (frame->header.ttl == 0) {
        atomic_fetch_add(&eng->frames_dropped, 1);
        return -1;
    }
    frame->header.ttl--;

    /* Extract SAD from options */
    sad_t query;
    int rc = extract_sad_from_frame(frame, &query);
    if (rc < 0) {
        /* No SAD in frame -- cannot route semantically.
         * In a real system we'd fall back to exact node_id forwarding.
         * For now, drop. */
        atomic_fetch_add(&eng->frames_dropped, 1);
        return -1;
    }

    /* Resolve: find top matches */
    resolve_result_t results[FWD_MAX_NEXT_HOPS];
    int k = eng->max_multipath;
    if (k > FWD_MAX_NEXT_HOPS) k = FWD_MAX_NEXT_HOPS;

    int num_results = resolver_resolve(eng->routing_table, &query, results, k);
    if (num_results <= 0) {
        atomic_fetch_add(&eng->resolve_failures, 1);
        atomic_fetch_add(&eng->frames_dropped, 1);
        return -1;
    }

    atomic_fetch_add(&eng->frames_resolved, 1);

    /* Select next hop via weighted random */
    int hop_idx = select_next_hop(results, num_results);
    if (hop_idx < 0) {
        atomic_fetch_add(&eng->frames_dropped, 1);
        return -1;
    }

    /* Rewrite destination node ID */
    node_id_copy(frame->header.dst_node_id,
                 results[hop_idx].entry.node_id);

    /* Forward */
    if (eng->send_fn) {
        /* Use port 0 (the send_fn implementation can do its own mapping) */
        rc = eng->send_fn(0, frame, eng->send_ctx);
        if (rc < 0) {
            atomic_fetch_add(&eng->frames_dropped, 1);
            return -1;
        }
    }

    atomic_fetch_add(&eng->frames_forwarded, 1);
    return 0;
}

/* --------------------------------------------------------------------------
 * Stats getters
 * -------------------------------------------------------------------------- */

uint64_t forwarding_engine_frames_forwarded(const forwarding_engine_t *eng)
{
    return atomic_load(&eng->frames_forwarded);
}

uint64_t forwarding_engine_frames_dropped(const forwarding_engine_t *eng)
{
    return atomic_load(&eng->frames_dropped);
}
