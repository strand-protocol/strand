/*
 * resolver.c - Multi-constraint SAD resolver
 *
 * Given a SAD query and a routing table, compute match scores for all
 * entries, apply scoring weights, and return the top-K sorted results.
 * This is the high-level resolution API that ties sad_match + routing_table.
 */

#include "nexroute/routing_table.h"
#include "nexroute/sad.h"
#include "nexroute/types.h"

#include <stdlib.h>
#include <string.h>

/* --------------------------------------------------------------------------
 * Resolver context - holds weights and any per-query state
 * -------------------------------------------------------------------------- */

typedef struct {
    scoring_weights_t weights;
    int               top_k;        /* max results to return */
} resolver_config_t;

static resolver_config_t g_resolver_config = {
    .weights = {
        .capability     = 0.30f,
        .latency        = 0.25f,
        .cost           = 0.20f,
        .context_window = 0.15f,
        .trust          = 0.10f,
    },
    .top_k = 3,
};

/* --------------------------------------------------------------------------
 * resolver_set_weights / resolver_set_top_k
 * -------------------------------------------------------------------------- */

void resolver_set_weights(const scoring_weights_t *w)
{
    if (w) g_resolver_config.weights = *w;
}

void resolver_set_top_k(int k)
{
    if (k > 0) g_resolver_config.top_k = k;
}

/* --------------------------------------------------------------------------
 * resolver_resolve
 *
 * Main resolve function:
 *   1. Take a snapshot of the routing table (lock-free read)
 *   2. Score each candidate entry against the query
 *   3. Return top-K results sorted by score descending
 *
 * Returns number of results, or -1 on error.
 * -------------------------------------------------------------------------- */

int resolver_resolve(const routing_table_t *rt,
                     const sad_t *query,
                     resolve_result_t *results,
                     int max_results)
{
    if (!rt || !query || !results)
        return -1;

    int k = max_results;
    if (k > g_resolver_config.top_k)
        k = g_resolver_config.top_k;
    if (k <= 0)
        k = 1;

    return routing_table_lookup(rt, query, results, k);
}

/* --------------------------------------------------------------------------
 * resolver_resolve_with_weights
 *
 * Like resolver_resolve but with explicit weights for this query.
 * We snapshot, run sad_find_best directly with the given weights.
 * -------------------------------------------------------------------------- */

/* Forward declaration from sad_match.c */
extern float sad_match_score(const sad_t *query,
                             const route_entry_t *candidate,
                             const scoring_weights_t *weights);

extern int sad_find_best(const sad_t *query,
                         const route_entry_t *table,
                         int table_size,
                         const scoring_weights_t *weights,
                         int top_k,
                         resolve_result_t *results);

int resolver_resolve_with_weights(const routing_table_t *rt,
                                  const sad_t *query,
                                  const scoring_weights_t *weights,
                                  resolve_result_t *results,
                                  int max_results)
{
    if (!rt || !query || !results || max_results <= 0)
        return -1;

    /* Get a snapshot of entries */
    int snap_max = 4096;  /* reasonable upper bound for stack alloc */
    uint32_t size = routing_table_size(rt);
    if ((int)size < snap_max) snap_max = (int)size;
    if (snap_max <= 0) return 0;

    route_entry_t *entries = malloc((size_t)snap_max * sizeof(route_entry_t));
    if (!entries) return -1;

    int n = routing_table_snapshot(rt, entries, snap_max);
    if (n <= 0) {
        free(entries);
        return 0;
    }

    int k = max_results;
    int found = sad_find_best(query, entries, n,
                              weights ? weights : &g_resolver_config.weights,
                              k, results);

    free(entries);
    return found;
}
