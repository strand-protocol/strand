/*
 * multipath.c - Maglev consistent hashing for weighted multipath
 *
 * Implements the Maglev hashing algorithm (Google, 2016) for consistent
 * selection of backends.  Builds a lookup table of size M (prime) using
 * per-backend offset/skip values.  Hash flow to index, select backend.
 * Supports weighted endpoints by giving higher-weight backends more
 * entries in the permutation generation.
 *
 * Reference: Eisenbud et al., "Maglev: A Fast and Reliable Software
 * Network Load Balancer", NSDI 2016.
 */

#include "strandroute/types.h"

#include <stdint.h>
#include <stdlib.h>
#include <string.h>
#include <stdbool.h>

/* --------------------------------------------------------------------------
 * Constants
 * -------------------------------------------------------------------------- */

/* Lookup table size -- must be prime.  65537 is a common choice for
 * Maglev; we use 5003 for smaller deployments (configurable). */
#define MAGLEV_TABLE_SIZE  5003
#define MAGLEV_MAX_BACKENDS 128

/* --------------------------------------------------------------------------
 * Backend descriptor
 * -------------------------------------------------------------------------- */

typedef struct {
    uint8_t  node_id[STRANDLINK_NODE_ID_LEN];
    uint32_t weight;        /* relative weight (higher = more traffic) */
    bool     active;
} maglev_backend_t;

/* --------------------------------------------------------------------------
 * Maglev lookup table
 * -------------------------------------------------------------------------- */

typedef struct {
    int32_t           table[MAGLEV_TABLE_SIZE];  /* backend index per slot */
    maglev_backend_t  backends[MAGLEV_MAX_BACKENDS];
    int               num_backends;
    bool              built;     /* true after populate() */
} maglev_t;

/* --------------------------------------------------------------------------
 * Hash helpers
 * -------------------------------------------------------------------------- */

/* DJB2 hash on raw bytes */
static uint32_t hash_djb2(const uint8_t *data, size_t len)
{
    uint32_t h = 5381;
    for (size_t i = 0; i < len; i++) {
        h = ((h << 5) + h) + data[i];
    }
    return h;
}

/* FNV-1a hash on raw bytes */
static uint32_t hash_fnv1a(const uint8_t *data, size_t len)
{
    uint32_t h = 2166136261u;
    for (size_t i = 0; i < len; i++) {
        h ^= data[i];
        h *= 16777619u;
    }
    return h;
}

/* --------------------------------------------------------------------------
 * maglev_init
 * -------------------------------------------------------------------------- */

void maglev_init(maglev_t *m)
{
    memset(m, 0, sizeof(*m));
    for (int i = 0; i < MAGLEV_TABLE_SIZE; i++) {
        m->table[i] = -1;
    }
    m->built = false;
}

/* --------------------------------------------------------------------------
 * maglev_add_backend
 * -------------------------------------------------------------------------- */

int maglev_add_backend(maglev_t *m,
                       const uint8_t node_id[STRANDLINK_NODE_ID_LEN],
                       uint32_t weight)
{
    if (!m || m->num_backends >= MAGLEV_MAX_BACKENDS)
        return -1;
    if (weight == 0) weight = 1;

    maglev_backend_t *b = &m->backends[m->num_backends];
    node_id_copy(b->node_id, node_id);
    b->weight = weight;
    b->active = true;
    m->num_backends++;
    m->built = false;  /* need rebuild */
    return 0;
}

/* --------------------------------------------------------------------------
 * maglev_remove_backend
 * -------------------------------------------------------------------------- */

int maglev_remove_backend(maglev_t *m,
                          const uint8_t node_id[STRANDLINK_NODE_ID_LEN])
{
    if (!m) return -1;

    for (int i = 0; i < m->num_backends; i++) {
        if (node_id_equal(m->backends[i].node_id, node_id)) {
            /* Swap with last */
            if (i < m->num_backends - 1) {
                m->backends[i] = m->backends[m->num_backends - 1];
            }
            m->num_backends--;
            m->built = false;
            return 0;
        }
    }
    return -1;
}

/* --------------------------------------------------------------------------
 * maglev_populate - build the lookup table
 *
 * For each backend i, compute:
 *   offset_i = hash1(backend_i.node_id) % M
 *   skip_i   = hash2(backend_i.node_id) % (M-1) + 1
 *
 * Weights are handled by giving each backend w_i "turns" per round:
 * a backend with weight 3 gets 3 consecutive tries before moving to
 * the next backend.
 * -------------------------------------------------------------------------- */

int maglev_populate(maglev_t *m)
{
    if (!m || m->num_backends == 0)
        return -1;

    const int M = MAGLEV_TABLE_SIZE;
    int N = m->num_backends;

    /* Reset table */
    for (int i = 0; i < M; i++) {
        m->table[i] = -1;
    }

    /* Compute offset/skip per backend */
    uint32_t *offset = malloc((size_t)N * sizeof(uint32_t));
    uint32_t *skip   = malloc((size_t)N * sizeof(uint32_t));
    uint32_t *next   = calloc((size_t)N, sizeof(uint32_t));  /* next candidate */

    if (!offset || !skip || !next) {
        free(offset); free(skip); free(next);
        return -1;
    }

    for (int i = 0; i < N; i++) {
        offset[i] = hash_djb2(m->backends[i].node_id, STRANDLINK_NODE_ID_LEN) % (uint32_t)M;
        skip[i]   = (hash_fnv1a(m->backends[i].node_id, STRANDLINK_NODE_ID_LEN) % (uint32_t)(M - 1)) + 1;
    }

    /* Fill the table */
    int filled = 0;
    int round  = 0;

    while (filled < M) {
        for (int i = 0; i < N && filled < M; i++) {
            /* Each backend gets weight[i] tries per round */
            uint32_t turns = m->backends[i].weight;
            for (uint32_t t = 0; t < turns && filled < M; t++) {
                /* Find next empty slot for backend i */
                uint32_t c = (offset[i] + next[i] * skip[i]) % (uint32_t)M;
                while (m->table[c] >= 0) {
                    next[i]++;
                    c = (offset[i] + next[i] * skip[i]) % (uint32_t)M;
                }
                m->table[c] = i;
                next[i]++;
                filled++;
            }
        }
        round++;
        /* Safety: avoid infinite loop if weights are all 0 somehow */
        if (round > M) break;
    }

    free(offset);
    free(skip);
    free(next);

    m->built = true;
    return 0;
}

/* --------------------------------------------------------------------------
 * maglev_lookup - select a backend for a given flow hash
 *
 * @param flow_key  Pointer to flow key data (e.g., stream ID, src+dst)
 * @param key_len   Length of flow key
 * @return          Backend index, or -1 if table not populated
 * -------------------------------------------------------------------------- */

int maglev_lookup(const maglev_t *m,
                  const uint8_t *flow_key, size_t key_len)
{
    if (!m || !m->built || m->num_backends == 0)
        return -1;

    uint32_t h = hash_fnv1a(flow_key, key_len);
    int slot = (int)(h % MAGLEV_TABLE_SIZE);
    return m->table[slot];
}

/* --------------------------------------------------------------------------
 * maglev_lookup_node_id - lookup and return the selected backend's node ID
 * -------------------------------------------------------------------------- */

int maglev_lookup_node_id(const maglev_t *m,
                          const uint8_t *flow_key, size_t key_len,
                          uint8_t out_node_id[STRANDLINK_NODE_ID_LEN])
{
    int idx = maglev_lookup(m, flow_key, key_len);
    if (idx < 0) return -1;
    node_id_copy(out_node_id, m->backends[idx].node_id);
    return 0;
}

/* --------------------------------------------------------------------------
 * maglev_get_backend_count / maglev_table_size
 * -------------------------------------------------------------------------- */

int maglev_get_backend_count(const maglev_t *m)
{
    return m ? m->num_backends : 0;
}

int maglev_get_table_size(void)
{
    return MAGLEV_TABLE_SIZE;
}
