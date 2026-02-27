/*
 * routing_table.h - RCU-based capability routing table
 *
 * Lock-free readers via atomic pointer swap.  Writers copy the current
 * array, modify the copy, then atomically publish it.  Old copies are
 * reclaimed after an epoch-based grace period.
 */

#ifndef STRANDROUTE_ROUTING_TABLE_H
#define STRANDROUTE_ROUTING_TABLE_H

#include "strandroute/types.h"

#ifdef __cplusplus
extern "C" {
#endif

/* Opaque handle */
typedef struct routing_table routing_table_t;

/**
 * Create a new routing table with the given initial capacity.
 * Returns NULL on allocation failure.
 */
routing_table_t *routing_table_create(uint32_t initial_capacity);

/**
 * Destroy the routing table and free all associated memory.
 * The caller must ensure no readers or writers are active.
 */
void routing_table_destroy(routing_table_t *rt);

/**
 * Insert or update a route entry.
 * If an entry with the same node_id already exists it is replaced.
 *
 * @return 0 on success, -1 on error (table full, allocation failure).
 */
int routing_table_insert(routing_table_t *rt, const route_entry_t *entry);

/**
 * Remove the entry matching node_id.
 *
 * @return 0 on success, -1 if not found.
 */
int routing_table_remove(routing_table_t *rt, const uint8_t node_id[16]);

/**
 * Resolve: find the best matching entries for a SAD query.
 * Thread-safe for concurrent readers (lock-free read path).
 *
 * @param rt          Routing table.
 * @param query       SAD query to match against.
 * @param results     Output array of resolve_result_t.
 * @param max_results Maximum number of results to return.
 * @return            Number of results written, or -1 on error.
 */
int routing_table_lookup(const routing_table_t *rt,
                         const sad_t *query,
                         resolve_result_t *results,
                         int max_results);

/**
 * Update live metrics for an existing entry (latency, load).
 *
 * @return 0 on success, -1 if not found.
 */
int routing_table_update_metrics(routing_table_t *rt,
                                 const uint8_t node_id[16],
                                 uint32_t latency_us,
                                 float load_factor);

/**
 * Return the current number of entries.
 */
uint32_t routing_table_size(const routing_table_t *rt);

/**
 * Get a snapshot of all entries (copies them into out, up to max).
 * Returns the number of entries copied.
 */
int routing_table_snapshot(const routing_table_t *rt,
                           route_entry_t *out,
                           int max);

/**
 * TTL-based garbage collection: remove entries where
 * (now_ns - last_updated) > ttl_ns.
 *
 * Entries with ttl_ns == 0 are considered permanent and are never removed.
 * Should be called periodically (e.g., from gossip_tick).
 *
 * @param rt     Routing table.
 * @param now_ns Current monotonic time in nanoseconds.
 * @return       Number of expired entries removed, or -1 on error.
 */
int routing_table_gc(routing_table_t *rt, uint64_t now_ns);

#ifdef __cplusplus
}
#endif

#endif /* STRANDROUTE_ROUTING_TABLE_H */
