/*
 * gossip.c - HyParView gossip protocol for capability advertisement
 *
 * Maintains an active view (small, fully connected) and a passive view
 * (larger, used for recovery).  Messages: Join, ForwardJoin, Disconnect,
 * Shuffle.  Periodic shuffle timer rotates passive view entries.
 *
 * Reference: Leitao et al., "HyParView: A Membership Protocol for
 * Reliable Gossip-Based Broadcast", DSN 2007.
 */

#include "nexroute/types.h"
#include "nexroute/routing_table.h"

#include <stdint.h>
#include <stdbool.h>
#include <string.h>
#include <stdlib.h>
#include <time.h>
#include <fcntl.h>
#include <unistd.h>

/* --------------------------------------------------------------------------
 * Constants
 * -------------------------------------------------------------------------- */

#define GOSSIP_MAX_ACTIVE    5
#define GOSSIP_MAX_PASSIVE   30
#define GOSSIP_SHUFFLE_LEN   3    /* number of entries per shuffle round */
#define GOSSIP_ARWL          6    /* Active Random Walk Length for ForwardJoin */
#define GOSSIP_PRWL          3    /* Passive Random Walk Length for ForwardJoin */
#define GOSSIP_DEFAULT_TTL   30   /* seconds */
#define GOSSIP_DEFAULT_INTERVAL_MS 1000

/* --------------------------------------------------------------------------
 * Gossip message types
 * -------------------------------------------------------------------------- */

typedef enum {
    GOSSIP_MSG_JOIN          = 0x01,
    GOSSIP_MSG_FORWARD_JOIN  = 0x02,
    GOSSIP_MSG_DISCONNECT    = 0x03,
    GOSSIP_MSG_SHUFFLE       = 0x04,
    GOSSIP_MSG_SHUFFLE_REPLY = 0x05,
    GOSSIP_MSG_ADVERTISE     = 0x06,  /* capability advertisement */
} gossip_msg_type_t;

/* --------------------------------------------------------------------------
 * Peer descriptor
 * -------------------------------------------------------------------------- */

typedef struct {
    uint8_t  node_id[NEXLINK_NODE_ID_LEN];
    uint16_t port;              /* overlay port for gossip */
    uint64_t last_seen;         /* monotonic ns timestamp */
    bool     active;            /* true if slot is in use */
} gossip_peer_t;

/* --------------------------------------------------------------------------
 * Gossip message (on-wire format, packed)
 * -------------------------------------------------------------------------- */

typedef struct __attribute__((packed)) {
    uint8_t  msg_type;
    uint8_t  ttl;
    uint8_t  sender_id[NEXLINK_NODE_ID_LEN];
    uint8_t  origin_id[NEXLINK_NODE_ID_LEN];  /* original initiator */
    uint16_t payload_len;
    /* Ed25519 signature over all preceding fields (msg_type..payload_len).
     * Populated when gossip_set_auth_fn() has been called.
     * Zero-filled when no signing callback is configured. */
    uint8_t  signature[64];
    /* payload follows */
} gossip_msg_header_t;

/* --------------------------------------------------------------------------
 * Gossip state
 * -------------------------------------------------------------------------- */

typedef struct {
    uint8_t         self_id[NEXLINK_NODE_ID_LEN];
    gossip_peer_t   active_view[GOSSIP_MAX_ACTIVE];
    gossip_peer_t   passive_view[GOSSIP_MAX_PASSIVE];
    int             active_count;
    int             passive_count;
    uint32_t        shuffle_timer_ms;
    uint32_t        advertise_interval_ms;
    uint64_t        last_shuffle_ts;
    uint64_t        last_advertise_ts;
    routing_table_t *routing_table;   /* owned externally */

    /* Callback for sending gossip messages */
    int (*send_fn)(const uint8_t dst_node_id[NEXLINK_NODE_ID_LEN],
                   const void *msg, size_t msg_len, void *ctx);
    void *send_ctx;

    /* Authentication callbacks for NexTrust integration (spec NR-G-005).
     * When set, outgoing messages are signed and incoming messages are
     * verified before processing.  A failed verification causes rejection. */
    int (*sign_fn)(const void *msg, size_t msg_len,
                   uint8_t sig[64], void *ctx);
    int (*verify_fn)(const void *msg, size_t msg_len,
                     const uint8_t sig[64], void *ctx);
    void *auth_ctx;
} gossip_state_t;

/* --------------------------------------------------------------------------
 * Helpers: pseudo-random via xorshift
 * -------------------------------------------------------------------------- */

static uint32_t gossip_rand_state = 0;

/* Seed PRNG from /dev/urandom to prevent predictable peer selection.
 * Predictable seeds allow targeted gossip poisoning attacks. */
static void gossip_seed_prng(void)
{
    uint32_t seed = 0;
    int fd = open("/dev/urandom", O_RDONLY | O_CLOEXEC);
    if (fd >= 0) {
        ssize_t n = read(fd, &seed, sizeof(seed));
        close(fd);
        if (n != (ssize_t)sizeof(seed))
            seed = 0;
    }
    /* Fallback: mix time with PID to reduce predictability */
    if (seed == 0)
        seed = (uint32_t)time(NULL) ^ (uint32_t)getpid();
    /* xorshift requires non-zero state */
    gossip_rand_state = (seed == 0) ? 0x12345678u : seed;
}

static uint32_t gossip_rand(void)
{
    if (gossip_rand_state == 0) {
        gossip_seed_prng();
    }
    gossip_rand_state ^= gossip_rand_state << 13;
    gossip_rand_state ^= gossip_rand_state >> 17;
    gossip_rand_state ^= gossip_rand_state << 5;
    return gossip_rand_state;
}

static int gossip_rand_range(int max)
{
    if (max <= 0) return 0;
    return (int)(gossip_rand() % (uint32_t)max);
}

/* --------------------------------------------------------------------------
 * View manipulation helpers
 * -------------------------------------------------------------------------- */

static gossip_peer_t *find_peer(gossip_peer_t *view, int count,
                                 const uint8_t node_id[NEXLINK_NODE_ID_LEN])
{
    for (int i = 0; i < count; i++) {
        if (view[i].active && node_id_equal(view[i].node_id, node_id))
            return &view[i];
    }
    return NULL;
}

static bool view_contains(gossip_peer_t *view, int count,
                           const uint8_t node_id[NEXLINK_NODE_ID_LEN])
{
    return find_peer(view, count, node_id) != NULL;
}

static int view_add(gossip_peer_t *view, int *count, int max_count,
                     const uint8_t node_id[NEXLINK_NODE_ID_LEN],
                     uint16_t port)
{
    if (*count >= max_count) return -1;
    if (view_contains(view, *count, node_id)) return 0;  /* already present */

    gossip_peer_t *p = &view[*count];
    node_id_copy(p->node_id, node_id);
    p->port      = port;
    p->last_seen = 0;
    p->active    = true;
    (*count)++;
    return 0;
}

static void view_remove_at(gossip_peer_t *view, int *count, int idx)
{
    if (idx < 0 || idx >= *count) return;
    /* Swap with last */
    if (idx < *count - 1) {
        view[idx] = view[*count - 1];
    }
    view[*count - 1].active = false;
    (*count)--;
}

static int find_peer_idx(gossip_peer_t *view, int count,
                          const uint8_t node_id[NEXLINK_NODE_ID_LEN])
{
    for (int i = 0; i < count; i++) {
        if (view[i].active && node_id_equal(view[i].node_id, node_id))
            return i;
    }
    return -1;
}

/* --------------------------------------------------------------------------
 * gossip_sign_header
 *
 * Sign a gossip header in-place when a signing callback is configured.
 * The signature covers all header fields up to (but not including) the
 * signature field itself -- the same region verified in gossip_handle_message.
 * Returns 0 on success or when no sign_fn is set, -1 on signing failure.
 * -------------------------------------------------------------------------- */

static int gossip_sign_header(gossip_state_t *gs, gossip_msg_header_t *hdr)
{
    if (!gs->sign_fn)
        return 0;
    static const size_t signed_len = offsetof(gossip_msg_header_t, signature);
    return gs->sign_fn(hdr, signed_len, hdr->signature, gs->auth_ctx);
}

/* --------------------------------------------------------------------------
 * gossip_init
 * -------------------------------------------------------------------------- */

void gossip_init(gossip_state_t *gs,
                 const uint8_t self_id[NEXLINK_NODE_ID_LEN],
                 routing_table_t *rt)
{
    memset(gs, 0, sizeof(*gs));
    node_id_copy(gs->self_id, self_id);
    gs->routing_table        = rt;
    gs->shuffle_timer_ms     = GOSSIP_DEFAULT_INTERVAL_MS * 10;
    gs->advertise_interval_ms = GOSSIP_DEFAULT_INTERVAL_MS;
}

void gossip_set_send_fn(gossip_state_t *gs,
                        int (*fn)(const uint8_t *, const void *, size_t, void *),
                        void *ctx)
{
    gs->send_fn  = fn;
    gs->send_ctx = ctx;
}

/* Install NexTrust authentication callbacks (spec NR-G-005).
 * sign_fn:   called to sign outgoing message headers.
 * verify_fn: called to verify incoming message headers; returns 0 on success.
 * ctx:       opaque context passed to both callbacks.
 * Pass NULL for all parameters to disable authentication.
 */
void gossip_set_auth_fn(gossip_state_t *gs,
                        int (*sign_fn)(const void *, size_t, uint8_t[64], void *),
                        int (*verify_fn)(const void *, size_t, const uint8_t[64], void *),
                        void *ctx)
{
    gs->sign_fn   = sign_fn;
    gs->verify_fn = verify_fn;
    gs->auth_ctx  = ctx;
}

/* --------------------------------------------------------------------------
 * gossip_handle_join
 *
 * A new node wants to join.  Add it to our active view; if full, drop a
 * random peer (moving it to passive view) and add the newcomer.  Then
 * forward the join to all active peers.
 * -------------------------------------------------------------------------- */

int gossip_handle_join(gossip_state_t *gs,
                       const uint8_t new_node[NEXLINK_NODE_ID_LEN],
                       uint16_t port)
{
    if (node_id_equal(gs->self_id, new_node))
        return 0;

    /* If active view is full, evict a random peer to passive */
    if (gs->active_count >= GOSSIP_MAX_ACTIVE) {
        int evict = gossip_rand_range(gs->active_count);
        gossip_peer_t evicted = gs->active_view[evict];
        view_remove_at(gs->active_view, &gs->active_count, evict);
        view_add(gs->passive_view, &gs->passive_count,
                 GOSSIP_MAX_PASSIVE, evicted.node_id, evicted.port);

        /* Send disconnect to evicted peer */
        if (gs->send_fn) {
            gossip_msg_header_t hdr;
            memset(&hdr, 0, sizeof(hdr));
            hdr.msg_type = GOSSIP_MSG_DISCONNECT;
            node_id_copy(hdr.sender_id, gs->self_id);
            if (gossip_sign_header(gs, &hdr) == 0)
                gs->send_fn(evicted.node_id, &hdr, sizeof(hdr), gs->send_ctx);
        }
    }

    /* Add new node to active view */
    view_add(gs->active_view, &gs->active_count,
             GOSSIP_MAX_ACTIVE, new_node, port);

    /* Forward join to all active peers (with TTL=ARWL) */
    if (gs->send_fn) {
        gossip_msg_header_t hdr;
        memset(&hdr, 0, sizeof(hdr));
        hdr.msg_type = GOSSIP_MSG_FORWARD_JOIN;
        hdr.ttl      = GOSSIP_ARWL;
        node_id_copy(hdr.sender_id, gs->self_id);
        node_id_copy(hdr.origin_id, new_node);

        if (gossip_sign_header(gs, &hdr) == 0) {
            for (int i = 0; i < gs->active_count; i++) {
                if (!node_id_equal(gs->active_view[i].node_id, new_node)) {
                    gs->send_fn(gs->active_view[i].node_id,
                                &hdr, sizeof(hdr), gs->send_ctx);
                }
            }
        }
    }

    return 0;
}

/* --------------------------------------------------------------------------
 * gossip_handle_forward_join
 *
 * Received a ForwardJoin from a peer.  If TTL == 0 or active view is
 * small, add the origin to active view.  Else if TTL == PRWL, add to
 * passive view.  Decrement TTL and forward to a random active peer.
 * -------------------------------------------------------------------------- */

int gossip_handle_forward_join(gossip_state_t *gs,
                               const uint8_t sender[NEXLINK_NODE_ID_LEN],
                               const uint8_t origin[NEXLINK_NODE_ID_LEN],
                               uint8_t ttl)
{
    (void)sender;

    if (node_id_equal(gs->self_id, origin))
        return 0;

    if (ttl == 0 || gs->active_count <= 1) {
        /* Add origin to active view */
        view_add(gs->active_view, &gs->active_count,
                 GOSSIP_MAX_ACTIVE, origin, 0);
        return 0;
    }

    if (ttl == GOSSIP_PRWL) {
        /* Add origin to passive view */
        view_add(gs->passive_view, &gs->passive_count,
                 GOSSIP_MAX_PASSIVE, origin, 0);
    }

    /* Forward to a random active peer (not the sender or origin) */
    if (gs->active_count > 0 && gs->send_fn) {
        int attempts = 0;
        int idx = gossip_rand_range(gs->active_count);
        while (attempts < gs->active_count &&
               (node_id_equal(gs->active_view[idx].node_id, origin) ||
                node_id_equal(gs->active_view[idx].node_id, gs->self_id))) {
            idx = (idx + 1) % gs->active_count;
            attempts++;
        }

        if (attempts < gs->active_count) {
            gossip_msg_header_t hdr;
            memset(&hdr, 0, sizeof(hdr));
            hdr.msg_type = GOSSIP_MSG_FORWARD_JOIN;
            hdr.ttl      = ttl - 1;
            node_id_copy(hdr.sender_id, gs->self_id);
            node_id_copy(hdr.origin_id, origin);
            if (gossip_sign_header(gs, &hdr) == 0)
                gs->send_fn(gs->active_view[idx].node_id,
                            &hdr, sizeof(hdr), gs->send_ctx);
        }
    }

    return 0;
}

/* --------------------------------------------------------------------------
 * gossip_handle_disconnect
 *
 * A peer has disconnected.  Remove from active view, promote a random
 * passive peer to active.
 * -------------------------------------------------------------------------- */

int gossip_handle_disconnect(gossip_state_t *gs,
                             const uint8_t peer_id[NEXLINK_NODE_ID_LEN])
{
    int idx = find_peer_idx(gs->active_view, gs->active_count, peer_id);
    if (idx >= 0) {
        view_remove_at(gs->active_view, &gs->active_count, idx);
    }

    /* Promote a random passive peer to fill the gap */
    if (gs->passive_count > 0 && gs->active_count < GOSSIP_MAX_ACTIVE) {
        int pidx = gossip_rand_range(gs->passive_count);
        gossip_peer_t promoted = gs->passive_view[pidx];
        view_remove_at(gs->passive_view, &gs->passive_count, pidx);
        view_add(gs->active_view, &gs->active_count,
                 GOSSIP_MAX_ACTIVE, promoted.node_id, promoted.port);
    }

    return 0;
}

/* --------------------------------------------------------------------------
 * gossip_do_shuffle
 *
 * Periodic shuffle: select SHUFFLE_LEN random peers from active + passive,
 * send them to a random active peer, and expect a reply with its own set.
 * -------------------------------------------------------------------------- */

int gossip_do_shuffle(gossip_state_t *gs)
{
    if (gs->active_count == 0)
        return 0;

    /* Pick a random active peer as the shuffle target */
    int target_idx = gossip_rand_range(gs->active_count);
    gossip_peer_t *target = &gs->active_view[target_idx];

    /* Build shuffle set: pick from passive view */
    uint8_t shuffle_set[GOSSIP_SHUFFLE_LEN][NEXLINK_NODE_ID_LEN];
    int shuffle_count = 0;

    for (int i = 0; i < GOSSIP_SHUFFLE_LEN && i < gs->passive_count; i++) {
        int pidx = gossip_rand_range(gs->passive_count);
        node_id_copy(shuffle_set[shuffle_count], gs->passive_view[pidx].node_id);
        shuffle_count++;
    }

    /* Always include self */
    if (shuffle_count < GOSSIP_SHUFFLE_LEN) {
        node_id_copy(shuffle_set[shuffle_count], gs->self_id);
        shuffle_count++;
    }

    /* Send shuffle message */
    if (gs->send_fn && shuffle_count > 0) {
        /* Build message: header + array of node IDs */
        size_t payload_len = (size_t)shuffle_count * NEXLINK_NODE_ID_LEN;
        size_t msg_len = sizeof(gossip_msg_header_t) + payload_len;
        uint8_t *msg = malloc(msg_len);
        if (msg) {
            gossip_msg_header_t *hdr = (gossip_msg_header_t *)msg;
            memset(hdr, 0, sizeof(*hdr));
            hdr->msg_type    = GOSSIP_MSG_SHUFFLE;
            hdr->ttl         = GOSSIP_ARWL;
            node_id_copy(hdr->sender_id, gs->self_id);
            node_id_copy(hdr->origin_id, gs->self_id);
            hdr->payload_len = (uint16_t)payload_len;

            memcpy(msg + sizeof(gossip_msg_header_t),
                   shuffle_set, payload_len);

            if (gossip_sign_header(gs, hdr) == 0)
                gs->send_fn(target->node_id, msg, msg_len, gs->send_ctx);
            free(msg);
        }
    }

    return 0;
}

/* --------------------------------------------------------------------------
 * gossip_handle_shuffle
 *
 * Received a shuffle from a peer.  Incorporate their entries into our
 * passive view (replacing oldest entries if full).  Send back our own
 * shuffle set as a reply.
 * -------------------------------------------------------------------------- */

int gossip_handle_shuffle(gossip_state_t *gs,
                          const uint8_t sender[NEXLINK_NODE_ID_LEN],
                          const uint8_t *payload, uint16_t payload_len)
{
    int num_entries = payload_len / NEXLINK_NODE_ID_LEN;

    /* Incorporate received entries into passive view */
    for (int i = 0; i < num_entries; i++) {
        const uint8_t *nid = &payload[i * NEXLINK_NODE_ID_LEN];
        if (node_id_equal(nid, gs->self_id))
            continue;

        if (gs->passive_count >= GOSSIP_MAX_PASSIVE) {
            /* Replace a random passive entry */
            int rep = gossip_rand_range(gs->passive_count);
            node_id_copy(gs->passive_view[rep].node_id, nid);
            gs->passive_view[rep].port = 0;
        } else {
            view_add(gs->passive_view, &gs->passive_count,
                     GOSSIP_MAX_PASSIVE, nid, 0);
        }
    }

    /* Send shuffle reply with our own entries */
    if (gs->send_fn) {
        int reply_count = 0;
        uint8_t reply_set[GOSSIP_SHUFFLE_LEN][NEXLINK_NODE_ID_LEN];

        for (int i = 0; i < GOSSIP_SHUFFLE_LEN && i < gs->passive_count; i++) {
            int pidx = gossip_rand_range(gs->passive_count);
            node_id_copy(reply_set[reply_count],
                         gs->passive_view[pidx].node_id);
            reply_count++;
        }

        if (reply_count > 0) {
            size_t rpl = (size_t)reply_count * NEXLINK_NODE_ID_LEN;
            size_t msg_len = sizeof(gossip_msg_header_t) + rpl;
            uint8_t *msg = malloc(msg_len);
            if (msg) {
                gossip_msg_header_t *hdr = (gossip_msg_header_t *)msg;
                memset(hdr, 0, sizeof(*hdr));
                hdr->msg_type    = GOSSIP_MSG_SHUFFLE_REPLY;
                node_id_copy(hdr->sender_id, gs->self_id);
                node_id_copy(hdr->origin_id, gs->self_id);
                hdr->payload_len = (uint16_t)rpl;
                memcpy(msg + sizeof(gossip_msg_header_t), reply_set, rpl);
                if (gossip_sign_header(gs, hdr) == 0)
                    gs->send_fn(sender, msg, msg_len, gs->send_ctx);
                free(msg);
            }
        }
    }

    return 0;
}

/* --------------------------------------------------------------------------
 * gossip_handle_message - dispatch incoming gossip messages
 * -------------------------------------------------------------------------- */

int gossip_handle_message(gossip_state_t *gs,
                          const void *msg, size_t msg_len)
{
    if (!gs || !msg || msg_len < sizeof(gossip_msg_header_t))
        return -1;

    const gossip_msg_header_t *hdr = (const gossip_msg_header_t *)msg;

    /* If a verify callback is installed, authenticate the message before
     * processing.  The signature covers all header fields up to (but not
     * including) the signature field itself. */
    if (gs->verify_fn) {
        /* signed_len = offset of signature field within the header */
        static const size_t signed_len =
            offsetof(gossip_msg_header_t, signature);
        if (gs->verify_fn(msg, signed_len, hdr->signature, gs->auth_ctx) != 0)
            return -1;   /* reject: bad or missing signature */
    }

    switch (hdr->msg_type) {
    case GOSSIP_MSG_JOIN:
        return gossip_handle_join(gs, hdr->origin_id, 0);

    case GOSSIP_MSG_FORWARD_JOIN:
        return gossip_handle_forward_join(gs, hdr->sender_id,
                                          hdr->origin_id, hdr->ttl);

    case GOSSIP_MSG_DISCONNECT:
        return gossip_handle_disconnect(gs, hdr->sender_id);

    case GOSSIP_MSG_SHUFFLE: {
        const uint8_t *payload = (const uint8_t *)msg + sizeof(*hdr);
        uint16_t pl = hdr->payload_len;
        if (sizeof(*hdr) + pl > msg_len) return -1;
        return gossip_handle_shuffle(gs, hdr->sender_id, payload, pl);
    }

    case GOSSIP_MSG_SHUFFLE_REPLY: {
        /* Incorporate shuffle reply into passive view same as shuffle */
        const uint8_t *payload = (const uint8_t *)msg + sizeof(*hdr);
        uint16_t pl = hdr->payload_len;
        if (sizeof(*hdr) + pl > msg_len) return -1;
        int num = pl / NEXLINK_NODE_ID_LEN;
        for (int i = 0; i < num; i++) {
            const uint8_t *nid = &payload[i * NEXLINK_NODE_ID_LEN];
            if (!node_id_equal(nid, gs->self_id)) {
                if (gs->passive_count < GOSSIP_MAX_PASSIVE) {
                    view_add(gs->passive_view, &gs->passive_count,
                             GOSSIP_MAX_PASSIVE, nid, 0);
                }
            }
        }
        return 0;
    }

    default:
        return -1;  /* unknown message type */
    }
}

/* --------------------------------------------------------------------------
 * gossip_tick - called periodically (e.g., every 100ms) to drive timers
 * -------------------------------------------------------------------------- */

void gossip_tick(gossip_state_t *gs, uint64_t now_ms)
{
    if (!gs) return;

    /* Shuffle timer */
    if (now_ms - gs->last_shuffle_ts >= gs->shuffle_timer_ms) {
        gossip_do_shuffle(gs);
        gs->last_shuffle_ts = now_ms;
    }
}
