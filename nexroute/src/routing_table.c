/*
 * routing_table.c - RCU-based lock-free capability routing table
 *
 * Two arrays + atomic pointer swap.  Readers dereference the current
 * pointer (lock-free).  Writers acquire a mutex, copy the live array,
 * mutate the copy, atomically swap, then wait for readers to drain
 * using a simple epoch counter.
 *
 * This avoids a dependency on liburcu for portability; the technique
 * is equivalent to a double-buffered RCU for moderate write rates.
 */

#include "nexroute/routing_table.h"
#include "nexroute/sad.h"

#include <stdlib.h>
#include <string.h>
#include <stdatomic.h>
#include <pthread.h>
#include <sched.h>

/* --------------------------------------------------------------------------
 * Internal snapshot - immutable array read by concurrent readers
 * -------------------------------------------------------------------------- */

typedef struct rt_snapshot {
    route_entry_t *entries;
    uint32_t       count;
    uint32_t       capacity;
    _Atomic uint32_t readers;   /* active reader count */
} rt_snapshot_t;

struct routing_table {
    _Atomic(rt_snapshot_t *) current;   /* readers load this atomically */
    rt_snapshot_t           *standby;   /* writers prepare updates here */
    pthread_mutex_t          write_lock;
    scoring_weights_t        weights;
};

/* --------------------------------------------------------------------------
 * Snapshot allocation helpers
 * -------------------------------------------------------------------------- */

static rt_snapshot_t *snapshot_alloc(uint32_t capacity)
{
    rt_snapshot_t *s = calloc(1, sizeof(rt_snapshot_t));
    if (!s) return NULL;
    s->entries  = calloc(capacity, sizeof(route_entry_t));
    if (!s->entries) { free(s); return NULL; }
    s->count    = 0;
    s->capacity = capacity;
    atomic_init(&s->readers, 0);
    return s;
}

static void snapshot_free(rt_snapshot_t *s)
{
    if (!s) return;
    free(s->entries);
    free(s);
}

static rt_snapshot_t *snapshot_clone(const rt_snapshot_t *src, uint32_t new_cap)
{
    if (new_cap < src->count)
        new_cap = src->count;
    rt_snapshot_t *dst = snapshot_alloc(new_cap);
    if (!dst) return NULL;
    memcpy(dst->entries, src->entries, src->count * sizeof(route_entry_t));
    dst->count = src->count;
    return dst;
}

/* --------------------------------------------------------------------------
 * Wait for readers to drain from a retired snapshot
 * -------------------------------------------------------------------------- */

static void wait_for_readers(rt_snapshot_t *old)
{
    /* Spin-wait until all readers that held a reference have released it.
     * sched_yield() prevents busy-spin from starving readers on single-core
     * or heavily loaded systems, which would otherwise deadlock. */
    while (atomic_load_explicit(&old->readers, memory_order_acquire) > 0) {
        sched_yield();
    }
}

/* --------------------------------------------------------------------------
 * Public API
 * -------------------------------------------------------------------------- */

routing_table_t *routing_table_create(uint32_t initial_capacity)
{
    if (initial_capacity == 0)
        initial_capacity = 64;

    routing_table_t *rt = calloc(1, sizeof(routing_table_t));
    if (!rt) return NULL;

    rt_snapshot_t *snap = snapshot_alloc(initial_capacity);
    if (!snap) { free(rt); return NULL; }

    atomic_init(&rt->current, snap);
    rt->standby = NULL;
    pthread_mutex_init(&rt->write_lock, NULL);
    rt->weights = scoring_weights_default();

    return rt;
}

void routing_table_destroy(routing_table_t *rt)
{
    if (!rt) return;
    rt_snapshot_t *cur = atomic_load(&rt->current);
    snapshot_free(cur);
    snapshot_free(rt->standby);
    pthread_mutex_destroy(&rt->write_lock);
    free(rt);
}

/* --------------------------------------------------------------------------
 * Read path - lock-free
 * -------------------------------------------------------------------------- */

static rt_snapshot_t *reader_acquire(const routing_table_t *rt)
{
    rt_snapshot_t *snap = atomic_load_explicit(
        ((_Atomic(rt_snapshot_t *) *)&rt->current), memory_order_acquire);
    atomic_fetch_add_explicit(&snap->readers, 1, memory_order_acq_rel);
    return snap;
}

static void reader_release(rt_snapshot_t *snap)
{
    atomic_fetch_sub_explicit(&snap->readers, 1, memory_order_acq_rel);
}

/* --------------------------------------------------------------------------
 * Write helpers (caller must hold write_lock)
 * -------------------------------------------------------------------------- */

/* Find entry index by node_id within a snapshot, returns -1 if not found */
static int find_entry(const rt_snapshot_t *snap, const uint8_t node_id[16])
{
    for (uint32_t i = 0; i < snap->count; i++) {
        if (node_id_equal(snap->entries[i].node_id, node_id))
            return (int)i;
    }
    return -1;
}

/* Swap current with new snapshot, wait for readers on old, then recycle old */
static void publish_and_reclaim(routing_table_t *rt, rt_snapshot_t *new_snap)
{
    rt_snapshot_t *old = atomic_exchange_explicit(
        &rt->current, new_snap, memory_order_acq_rel);

    wait_for_readers(old);

    /* Recycle old as the standby buffer */
    snapshot_free(rt->standby);
    rt->standby = old;
}

/* --------------------------------------------------------------------------
 * routing_table_insert
 * -------------------------------------------------------------------------- */

int routing_table_insert(routing_table_t *rt, const route_entry_t *entry)
{
    if (!rt || !entry) return -1;

    pthread_mutex_lock(&rt->write_lock);

    rt_snapshot_t *cur = atomic_load_explicit(&rt->current, memory_order_acquire);

    /* Determine if we need more capacity */
    uint32_t new_cap = cur->capacity;
    if (cur->count >= cur->capacity)
        new_cap = cur->capacity * 2;

    rt_snapshot_t *next = snapshot_clone(cur, new_cap);
    if (!next) {
        pthread_mutex_unlock(&rt->write_lock);
        return -1;
    }

    /* Check if entry already exists (update in-place in copy) */
    int idx = find_entry(next, entry->node_id);
    if (idx >= 0) {
        next->entries[idx] = *entry;
    } else {
        next->entries[next->count] = *entry;
        next->count++;
    }

    publish_and_reclaim(rt, next);
    pthread_mutex_unlock(&rt->write_lock);
    return 0;
}

/* --------------------------------------------------------------------------
 * routing_table_remove
 * -------------------------------------------------------------------------- */

int routing_table_remove(routing_table_t *rt, const uint8_t node_id[16])
{
    if (!rt || !node_id) return -1;

    pthread_mutex_lock(&rt->write_lock);

    rt_snapshot_t *cur = atomic_load_explicit(&rt->current, memory_order_acquire);

    int idx = find_entry(cur, node_id);
    if (idx < 0) {
        pthread_mutex_unlock(&rt->write_lock);
        return -1;
    }

    rt_snapshot_t *next = snapshot_clone(cur, cur->capacity);
    if (!next) {
        pthread_mutex_unlock(&rt->write_lock);
        return -1;
    }

    /* Remove by swapping with last element */
    if ((uint32_t)idx < next->count - 1) {
        next->entries[idx] = next->entries[next->count - 1];
    }
    next->count--;

    publish_and_reclaim(rt, next);
    pthread_mutex_unlock(&rt->write_lock);
    return 0;
}

/* --------------------------------------------------------------------------
 * routing_table_lookup  (lock-free read path)
 * -------------------------------------------------------------------------- */

/* Forward declaration from sad_match.c */
extern int sad_find_best(const sad_t *query, const route_entry_t *table,
                         int table_size, const scoring_weights_t *weights,
                         int top_k, resolve_result_t *results);

int routing_table_lookup(const routing_table_t *rt,
                         const sad_t *query,
                         resolve_result_t *results,
                         int max_results)
{
    if (!rt || !query || !results || max_results <= 0)
        return -1;

    rt_snapshot_t *snap = reader_acquire(rt);

    int n = sad_find_best(query,
                          snap->entries,
                          (int)snap->count,
                          &rt->weights,
                          max_results,
                          results);

    reader_release(snap);
    return n;
}

/* --------------------------------------------------------------------------
 * routing_table_update_metrics
 * -------------------------------------------------------------------------- */

int routing_table_update_metrics(routing_table_t *rt,
                                 const uint8_t node_id[16],
                                 uint32_t latency_us,
                                 float load_factor)
{
    if (!rt || !node_id) return -1;

    pthread_mutex_lock(&rt->write_lock);

    rt_snapshot_t *cur = atomic_load_explicit(&rt->current, memory_order_acquire);

    int idx = find_entry(cur, node_id);
    if (idx < 0) {
        pthread_mutex_unlock(&rt->write_lock);
        return -1;
    }

    rt_snapshot_t *next = snapshot_clone(cur, cur->capacity);
    if (!next) {
        pthread_mutex_unlock(&rt->write_lock);
        return -1;
    }

    next->entries[idx].latency_us  = latency_us;
    next->entries[idx].load_factor = load_factor;

    publish_and_reclaim(rt, next);
    pthread_mutex_unlock(&rt->write_lock);
    return 0;
}

/* --------------------------------------------------------------------------
 * routing_table_size
 * -------------------------------------------------------------------------- */

uint32_t routing_table_size(const routing_table_t *rt)
{
    if (!rt) return 0;
    rt_snapshot_t *snap = reader_acquire(rt);
    uint32_t n = snap->count;
    reader_release(snap);
    return n;
}

/* --------------------------------------------------------------------------
 * routing_table_snapshot
 * -------------------------------------------------------------------------- */

int routing_table_snapshot(const routing_table_t *rt,
                           route_entry_t *out,
                           int max)
{
    if (!rt || !out || max <= 0) return 0;

    rt_snapshot_t *snap = reader_acquire(rt);
    int n = (int)snap->count;
    if (n > max) n = max;
    memcpy(out, snap->entries, (size_t)n * sizeof(route_entry_t));
    reader_release(snap);
    return n;
}

/* --------------------------------------------------------------------------
 * routing_table_gc — TTL-based garbage collection (spec NR-RT-003)
 *
 * Removes entries where (now_ns - last_updated) > ttl_ns.
 * Entries with ttl_ns == 0 are permanent and never expired.
 * Prevents stale route poisoning from lingering unreachable nodes.
 * -------------------------------------------------------------------------- */

int routing_table_gc(routing_table_t *rt, uint64_t now_ns)
{
    if (!rt) return -1;

    pthread_mutex_lock(&rt->write_lock);

    rt_snapshot_t *cur = atomic_load_explicit(&rt->current, memory_order_acquire);

    /* Count how many entries will survive GC */
    uint32_t survivors = 0;
    for (uint32_t i = 0; i < cur->count; i++) {
        const route_entry_t *e = &cur->entries[i];
        if (e->ttl_ns == 0 || (now_ns - e->last_updated) <= e->ttl_ns)
            survivors++;
    }

    int expired = (int)(cur->count - survivors);
    if (expired == 0) {
        pthread_mutex_unlock(&rt->write_lock);
        return 0;
    }

    /* Build a new snapshot containing only live entries */
    rt_snapshot_t *next = snapshot_alloc(cur->capacity);
    if (!next) {
        pthread_mutex_unlock(&rt->write_lock);
        return -1;
    }

    for (uint32_t i = 0; i < cur->count; i++) {
        const route_entry_t *e = &cur->entries[i];
        if (e->ttl_ns == 0 || (now_ns - e->last_updated) <= e->ttl_ns)
            next->entries[next->count++] = *e;
    }

    publish_and_reclaim(rt, next);
    pthread_mutex_unlock(&rt->write_lock);
    return expired;
}
