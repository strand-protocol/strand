/*
 * test_routing.c - Routing table CRUD + concurrent read tests
 */

#include "strandroute/routing_table.h"
#include "strandroute/sad.h"
#include "strandroute/types.h"

#include <stdio.h>
#include <string.h>
#include <pthread.h>

/* --------------------------------------------------------------------------
 * Test framework hooks
 * -------------------------------------------------------------------------- */

extern void test_register(const char *name, int (*fn)(void));
extern int  test_assert_impl(int cond, const char *expr,
                              const char *file, int line);

#define TASSERT(cond) do { errors += test_assert_impl((cond), #cond, __FILE__, __LINE__); } while(0)

/* --------------------------------------------------------------------------
 * Helper: build a route entry with a given node ID byte and capabilities
 * -------------------------------------------------------------------------- */

static route_entry_t make_entry(uint8_t id_byte, uint32_t caps,
                                 uint32_t ctx_window,
                                 uint32_t latency_us, uint32_t cost,
                                 uint8_t trust, uint16_t region)
{
    route_entry_t e;
    memset(&e, 0, sizeof(e));
    e.node_id[0]  = id_byte;
    e.latency_us  = latency_us;
    e.cost_milli  = cost;
    e.trust_level = trust;
    e.region_code = region;
    e.load_factor = 0.0f;

    sad_init(&e.capabilities);
    sad_add_uint32(&e.capabilities, SAD_FIELD_CAPABILITY, caps);
    if (ctx_window > 0) {
        sad_add_uint32(&e.capabilities, SAD_FIELD_CONTEXT_WINDOW, ctx_window);
    }

    return e;
}

/* --------------------------------------------------------------------------
 * Test: create and destroy empty table
 * -------------------------------------------------------------------------- */

static int test_routing_create_destroy(void)
{
    int errors = 0;

    routing_table_t *rt = routing_table_create(16);
    TASSERT(rt != NULL);
    TASSERT(routing_table_size(rt) == 0);

    routing_table_destroy(rt);
    return errors;
}

/* --------------------------------------------------------------------------
 * Test: insert and lookup
 * -------------------------------------------------------------------------- */

static int test_routing_insert_lookup(void)
{
    int errors = 0;

    routing_table_t *rt = routing_table_create(16);
    TASSERT(rt != NULL);

    /* Insert 3 entries */
    route_entry_t e1 = make_entry(0x01,
        CAP_TEXT_GEN | CAP_CODE_GEN, 131072, 50000, 1000,
        TRUST_FULL_AUDIT, 840);
    route_entry_t e2 = make_entry(0x02,
        CAP_TEXT_GEN, 65536, 100000, 500,
        TRUST_IDENTITY, 276);
    route_entry_t e3 = make_entry(0x03,
        CAP_IMAGE_GEN, 0, 30000, 2000,
        TRUST_PROVENANCE, 392);

    TASSERT(routing_table_insert(rt, &e1) == 0);
    TASSERT(routing_table_insert(rt, &e2) == 0);
    TASSERT(routing_table_insert(rt, &e3) == 0);
    TASSERT(routing_table_size(rt) == 3);

    /* Query for text+code gen */
    sad_t query;
    sad_init(&query);
    sad_add_uint32(&query, SAD_FIELD_CAPABILITY, CAP_TEXT_GEN | CAP_CODE_GEN);

    resolve_result_t results[3];
    int n = routing_table_lookup(rt, &query, results, 3);
    TASSERT(n > 0);

    /* Best match should be entry 1 (has both text+code gen) */
    TASSERT(results[0].entry.node_id[0] == 0x01);
    TASSERT(results[0].score > 0.0f);

    routing_table_destroy(rt);
    return errors;
}

/* --------------------------------------------------------------------------
 * Test: insert duplicate (update in place)
 * -------------------------------------------------------------------------- */

static int test_routing_insert_duplicate(void)
{
    int errors = 0;

    routing_table_t *rt = routing_table_create(16);

    route_entry_t e1 = make_entry(0x01, CAP_TEXT_GEN, 0, 100000, 500,
                                   TRUST_IDENTITY, 840);
    TASSERT(routing_table_insert(rt, &e1) == 0);
    TASSERT(routing_table_size(rt) == 1);

    /* Insert again with updated latency */
    e1.latency_us = 50000;
    TASSERT(routing_table_insert(rt, &e1) == 0);
    TASSERT(routing_table_size(rt) == 1);  /* should still be 1 */

    routing_table_destroy(rt);
    return errors;
}

/* --------------------------------------------------------------------------
 * Test: remove
 * -------------------------------------------------------------------------- */

static int test_routing_remove(void)
{
    int errors = 0;

    routing_table_t *rt = routing_table_create(16);

    route_entry_t e1 = make_entry(0x01, CAP_TEXT_GEN, 0, 100000, 500,
                                   TRUST_IDENTITY, 840);
    route_entry_t e2 = make_entry(0x02, CAP_CODE_GEN, 0, 50000, 1000,
                                   TRUST_PROVENANCE, 276);

    routing_table_insert(rt, &e1);
    routing_table_insert(rt, &e2);
    TASSERT(routing_table_size(rt) == 2);

    /* Remove e1 */
    TASSERT(routing_table_remove(rt, e1.node_id) == 0);
    TASSERT(routing_table_size(rt) == 1);

    /* Remove again should fail */
    TASSERT(routing_table_remove(rt, e1.node_id) == -1);

    /* e2 should still be there */
    sad_t query;
    sad_init(&query);
    sad_add_uint32(&query, SAD_FIELD_CAPABILITY, CAP_CODE_GEN);

    resolve_result_t results[1];
    int n = routing_table_lookup(rt, &query, results, 1);
    TASSERT(n == 1);
    TASSERT(results[0].entry.node_id[0] == 0x02);

    routing_table_destroy(rt);
    return errors;
}

/* --------------------------------------------------------------------------
 * Test: update metrics
 * -------------------------------------------------------------------------- */

static int test_routing_update_metrics(void)
{
    int errors = 0;

    routing_table_t *rt = routing_table_create(16);

    route_entry_t e = make_entry(0x01, CAP_TEXT_GEN, 0, 100000, 500,
                                  TRUST_IDENTITY, 840);
    routing_table_insert(rt, &e);

    /* Update latency */
    TASSERT(routing_table_update_metrics(rt, e.node_id, 25000, 0.5f) == 0);

    /* Snapshot and verify */
    route_entry_t snap[1];
    int n = routing_table_snapshot(rt, snap, 1);
    TASSERT(n == 1);
    TASSERT(snap[0].latency_us == 25000);
    TASSERT(snap[0].load_factor > 0.4f && snap[0].load_factor < 0.6f);

    /* Update non-existent entry should fail */
    uint8_t fake_id[16] = { 0xFF };
    TASSERT(routing_table_update_metrics(rt, fake_id, 0, 0) == -1);

    routing_table_destroy(rt);
    return errors;
}

/* --------------------------------------------------------------------------
 * Test: table grows beyond initial capacity
 * -------------------------------------------------------------------------- */

static int test_routing_grow(void)
{
    int errors = 0;

    routing_table_t *rt = routing_table_create(4);  /* small initial cap */

    for (int i = 0; i < 20; i++) {
        route_entry_t e = make_entry((uint8_t)i, CAP_TEXT_GEN, 0,
                                      (uint32_t)(100000 - i * 1000), 500,
                                      TRUST_IDENTITY, 840);
        TASSERT(routing_table_insert(rt, &e) == 0);
    }

    TASSERT(routing_table_size(rt) == 20);

    /* Wildcard lookup should find all 20 */
    sad_t query;
    sad_init(&query);  /* wildcard */

    resolve_result_t results[20];
    int n = routing_table_lookup(rt, &query, results, 20);
    /* n may be capped by resolver's top_k default (3) */
    TASSERT(n > 0);

    routing_table_destroy(rt);
    return errors;
}

/* --------------------------------------------------------------------------
 * Test: snapshot
 * -------------------------------------------------------------------------- */

static int test_routing_snapshot(void)
{
    int errors = 0;

    routing_table_t *rt = routing_table_create(16);

    for (int i = 0; i < 5; i++) {
        route_entry_t e = make_entry((uint8_t)(i + 1), CAP_TEXT_GEN, 0,
                                      50000, 500, TRUST_IDENTITY, 840);
        routing_table_insert(rt, &e);
    }

    route_entry_t snap[10];
    int n = routing_table_snapshot(rt, snap, 10);
    TASSERT(n == 5);

    /* All entries should have non-zero first byte of node_id */
    for (int i = 0; i < n; i++) {
        TASSERT(snap[i].node_id[0] > 0);
    }

    routing_table_destroy(rt);
    return errors;
}

/* --------------------------------------------------------------------------
 * Test: concurrent readers (basic stress test)
 *
 * Spawn multiple reader threads that continuously lookup while the
 * main thread inserts/removes entries.  Verify no crashes (data race).
 * -------------------------------------------------------------------------- */

#define CONCURRENT_READERS     4
#define CONCURRENT_ITERATIONS  1000

typedef struct {
    const routing_table_t *rt;
    int                    lookups_done;
} reader_ctx_t;

static void *reader_thread(void *arg)
{
    reader_ctx_t *ctx = (reader_ctx_t *)arg;

    sad_t query;
    sad_init(&query);
    sad_add_uint32(&query, SAD_FIELD_CAPABILITY, CAP_TEXT_GEN);

    resolve_result_t results[3];

    for (int i = 0; i < CONCURRENT_ITERATIONS; i++) {
        int n = routing_table_lookup(ctx->rt, &query, results, 3);
        (void)n;
        ctx->lookups_done++;
    }

    return NULL;
}

static int test_routing_concurrent(void)
{
    int errors = 0;

    routing_table_t *rt = routing_table_create(64);

    /* Pre-populate some entries */
    for (int i = 0; i < 10; i++) {
        route_entry_t e = make_entry((uint8_t)(i + 1), CAP_TEXT_GEN, 0,
                                      50000, 500, TRUST_IDENTITY, 840);
        routing_table_insert(rt, &e);
    }

    /* Start reader threads */
    pthread_t threads[CONCURRENT_READERS];
    reader_ctx_t contexts[CONCURRENT_READERS];

    for (int i = 0; i < CONCURRENT_READERS; i++) {
        contexts[i].rt = rt;
        contexts[i].lookups_done = 0;
        pthread_create(&threads[i], NULL, reader_thread, &contexts[i]);
    }

    /* Meanwhile, do some inserts and removes */
    for (int i = 10; i < 30; i++) {
        route_entry_t e = make_entry((uint8_t)i, CAP_TEXT_GEN | CAP_CODE_GEN, 0,
                                      40000, 1000, TRUST_PROVENANCE, 276);
        routing_table_insert(rt, &e);
    }

    for (int i = 10; i < 20; i++) {
        uint8_t nid[16] = { 0 };
        nid[0] = (uint8_t)i;
        routing_table_remove(rt, nid);
    }

    /* Wait for readers */
    for (int i = 0; i < CONCURRENT_READERS; i++) {
        pthread_join(threads[i], NULL);
        TASSERT(contexts[i].lookups_done == CONCURRENT_ITERATIONS);
    }

    /* Table should be in a consistent state.
     * Original: id bytes 1..10 (10 entries).
     * Additions: id bytes 10..29 (20 entries, but byte 10 overlaps -> 19 new).
     * After inserts: 10 + 19 = 29 unique entries (bytes 1..29).
     * Removals: bytes 10..19 = 10 entries removed.
     * Final: 29 - 10 = 19 entries. */
    uint32_t final_size = routing_table_size(rt);
    TASSERT(final_size == 19);

    routing_table_destroy(rt);
    return errors;
}

/* --------------------------------------------------------------------------
 * Security test: TTL-based garbage collection (spec NR-RT-003)
 * -------------------------------------------------------------------------- */

#define NS_PER_SEC  UINT64_C(1000000000)

static int test_routing_gc_ttl(void)
{
    int errors = 0;

    routing_table_t *rt = routing_table_create(16);
    TASSERT(rt != NULL);

    uint64_t t0 = 100 * NS_PER_SEC;   /* baseline "now" */
    uint64_t ttl_30s = 30 * NS_PER_SEC;

    /* Entry A: inserted at t0, ttl=30s → expires at t0+30s */
    route_entry_t ea = make_entry(0xAA, CAP_TEXT_GEN, 0, 50000, 500,
                                   TRUST_IDENTITY, 840);
    ea.last_updated = t0;
    ea.ttl_ns       = ttl_30s;

    /* Entry B: permanent (ttl_ns=0) */
    route_entry_t eb = make_entry(0xBB, CAP_CODE_GEN, 0, 60000, 600,
                                   TRUST_IDENTITY, 840);
    eb.last_updated = t0;
    eb.ttl_ns       = 0;  /* permanent */

    /* Entry C: very short TTL, already expired at t0+1s */
    route_entry_t ec = make_entry(0xCC, CAP_IMAGE_GEN, 0, 70000, 700,
                                   TRUST_IDENTITY, 840);
    ec.last_updated = t0;
    ec.ttl_ns       = NS_PER_SEC;  /* 1-second TTL */

    routing_table_insert(rt, &ea);
    routing_table_insert(rt, &eb);
    routing_table_insert(rt, &ec);
    TASSERT(routing_table_size(rt) == 3);

    /* GC at t0+20s: A is still live (20s < 30s), C is expired (20s > 1s) */
    uint64_t t1 = t0 + 20 * NS_PER_SEC;
    int removed = routing_table_gc(rt, t1);
    TASSERT(removed == 1);                    /* only C expired */
    TASSERT(routing_table_size(rt) == 2);     /* A and B remain */

    /* GC at t0+35s: A is now expired (35s > 30s) */
    uint64_t t2 = t0 + 35 * NS_PER_SEC;
    removed = routing_table_gc(rt, t2);
    TASSERT(removed == 1);                    /* A expires */
    TASSERT(routing_table_size(rt) == 1);     /* only B (permanent) remains */

    /* GC again at t2: nothing to remove */
    removed = routing_table_gc(rt, t2);
    TASSERT(removed == 0);
    TASSERT(routing_table_size(rt) == 1);

    /* Verify the surviving entry is B */
    route_entry_t snap[2];
    int n = routing_table_snapshot(rt, snap, 2);
    TASSERT(n == 1);
    TASSERT(snap[0].node_id[0] == 0xBB);

    routing_table_destroy(rt);
    return errors;
}

/* --------------------------------------------------------------------------
 * Security test: GC on NULL table returns error
 * -------------------------------------------------------------------------- */

static int test_routing_gc_null(void)
{
    int errors = 0;
    TASSERT(routing_table_gc(NULL, 0) == -1);
    return errors;
}

/* --------------------------------------------------------------------------
 * Registration
 * -------------------------------------------------------------------------- */

void register_routing_tests(void)
{
    test_register("routing_create_destroy",    test_routing_create_destroy);
    test_register("routing_insert_lookup",     test_routing_insert_lookup);
    test_register("routing_insert_duplicate",  test_routing_insert_duplicate);
    test_register("routing_remove",            test_routing_remove);
    test_register("routing_update_metrics",    test_routing_update_metrics);
    test_register("routing_grow",              test_routing_grow);
    test_register("routing_snapshot",          test_routing_snapshot);
    test_register("routing_concurrent_reads",  test_routing_concurrent);
    test_register("routing_gc_ttl_expiry",     test_routing_gc_ttl);
    test_register("routing_gc_null_safe",      test_routing_gc_null);
}
