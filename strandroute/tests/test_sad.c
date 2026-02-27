/*
 * test_sad.c - SAD encode/decode roundtrip + matching tests
 */

#include "strandroute/sad.h"
#include "strandroute/types.h"

#include <stdio.h>
#include <string.h>
#include <math.h>

/* --------------------------------------------------------------------------
 * Test framework hooks (defined in test_main.c)
 * -------------------------------------------------------------------------- */

extern void test_register(const char *name, int (*fn)(void));
extern int  test_assert_impl(int cond, const char *expr,
                              const char *file, int line);

#define TASSERT(cond) do { errors += test_assert_impl((cond), #cond, __FILE__, __LINE__); } while(0)

/* From sad_match.c */
extern float sad_match_score(const sad_t *query,
                             const route_entry_t *candidate,
                             const scoring_weights_t *weights);

extern int sad_find_best(const sad_t *query,
                         const route_entry_t *table,
                         int table_size,
                         const scoring_weights_t *weights,
                         int top_k,
                         resolve_result_t *results);

/* --------------------------------------------------------------------------
 * Test: SAD init and add fields
 * -------------------------------------------------------------------------- */

static int test_sad_init(void)
{
    int errors = 0;
    sad_t sad;
    sad_init(&sad);

    TASSERT(sad.version == SAD_VERSION);
    TASSERT(sad.num_fields == 0);
    TASSERT(sad.flags == 0);

    int rc = sad_add_uint32(&sad, SAD_FIELD_MODEL_ARCH, MODEL_ARCH_TRANSFORMER);
    TASSERT(rc == 0);
    TASSERT(sad.num_fields == 1);

    rc = sad_add_uint32(&sad, SAD_FIELD_CAPABILITY,
                        CAP_TEXT_GEN | CAP_CODE_GEN | CAP_TOOL_USE);
    TASSERT(rc == 0);
    TASSERT(sad.num_fields == 2);

    rc = sad_add_uint32(&sad, SAD_FIELD_CONTEXT_WINDOW, 131072);
    TASSERT(rc == 0);

    rc = sad_add_uint32(&sad, SAD_FIELD_MAX_LATENCY_MS, 200);
    TASSERT(rc == 0);

    rc = sad_add_uint8(&sad, SAD_FIELD_TRUST_LEVEL, TRUST_PROVENANCE);
    TASSERT(rc == 0);
    TASSERT(sad.num_fields == 5);

    /* Verify field lookup */
    uint32_t arch = sad_get_uint32(&sad, SAD_FIELD_MODEL_ARCH);
    TASSERT(arch == MODEL_ARCH_TRANSFORMER);

    uint32_t caps = sad_get_uint32(&sad, SAD_FIELD_CAPABILITY);
    TASSERT(caps == (CAP_TEXT_GEN | CAP_CODE_GEN | CAP_TOOL_USE));

    uint32_t ctx = sad_get_uint32(&sad, SAD_FIELD_CONTEXT_WINDOW);
    TASSERT(ctx == 131072);

    uint8_t trust = sad_get_uint8(&sad, SAD_FIELD_TRUST_LEVEL);
    TASSERT(trust == TRUST_PROVENANCE);

    return errors;
}

/* --------------------------------------------------------------------------
 * Test: SAD regions
 * -------------------------------------------------------------------------- */

static int test_sad_regions(void)
{
    int errors = 0;
    sad_t sad;
    sad_init(&sad);

    uint16_t regions[] = { 276, 250, 528 };  /* DE, FR, NL */
    int rc = sad_add_regions(&sad, SAD_FIELD_REGION_PREFER, regions, 3);
    TASSERT(rc == 0);
    TASSERT(sad.num_fields == 1);

    const sad_field_t *f = sad_find_field(&sad, SAD_FIELD_REGION_PREFER);
    TASSERT(f != NULL);
    TASSERT(f->length == 6);  /* 3 * 2 bytes */

    return errors;
}

/* --------------------------------------------------------------------------
 * Test: SAD encode/decode roundtrip
 * -------------------------------------------------------------------------- */

static int test_sad_encode_decode(void)
{
    int errors = 0;

    /* Build a SAD */
    sad_t original;
    sad_init(&original);
    sad_add_uint32(&original, SAD_FIELD_MODEL_ARCH, MODEL_ARCH_TRANSFORMER);
    sad_add_uint32(&original, SAD_FIELD_CAPABILITY,
                   CAP_TEXT_GEN | CAP_CODE_GEN);
    sad_add_uint32(&original, SAD_FIELD_CONTEXT_WINDOW, 65536);
    sad_add_uint32(&original, SAD_FIELD_MAX_LATENCY_MS, 100);
    sad_add_uint32(&original, SAD_FIELD_MAX_COST_MILLI, 5000);
    sad_add_uint8(&original, SAD_FIELD_TRUST_LEVEL, TRUST_IDENTITY);

    uint16_t regions[] = { 840, 124 };  /* US, CA */
    sad_add_regions(&original, SAD_FIELD_REGION_PREFER, regions, 2);

    /* Encode */
    uint8_t buf[SAD_MAX_SIZE];
    int encoded_len = sad_encode(&original, buf, sizeof(buf));
    TASSERT(encoded_len > 0);

    /* Validate */
    int valid = sad_validate(buf, (size_t)encoded_len);
    TASSERT(valid == 0);

    /* Decode */
    sad_t decoded;
    int decoded_len = sad_decode(buf, (size_t)encoded_len, &decoded);
    TASSERT(decoded_len == encoded_len);
    TASSERT(decoded.version == original.version);
    TASSERT(decoded.num_fields == original.num_fields);

    /* Verify all fields match */
    TASSERT(sad_get_uint32(&decoded, SAD_FIELD_MODEL_ARCH) ==
            MODEL_ARCH_TRANSFORMER);
    TASSERT(sad_get_uint32(&decoded, SAD_FIELD_CAPABILITY) ==
            (CAP_TEXT_GEN | CAP_CODE_GEN));
    TASSERT(sad_get_uint32(&decoded, SAD_FIELD_CONTEXT_WINDOW) == 65536);
    TASSERT(sad_get_uint32(&decoded, SAD_FIELD_MAX_LATENCY_MS) == 100);
    TASSERT(sad_get_uint32(&decoded, SAD_FIELD_MAX_COST_MILLI) == 5000);
    TASSERT(sad_get_uint8(&decoded, SAD_FIELD_TRUST_LEVEL) == TRUST_IDENTITY);

    /* Check region field */
    const sad_field_t *rf = sad_find_field(&decoded, SAD_FIELD_REGION_PREFER);
    TASSERT(rf != NULL);
    TASSERT(rf->length == 4);  /* 2 regions * 2 bytes */

    return errors;
}

/* --------------------------------------------------------------------------
 * Test: SAD empty (wildcard)
 * -------------------------------------------------------------------------- */

static int test_sad_empty_roundtrip(void)
{
    int errors = 0;

    sad_t empty;
    sad_init(&empty);

    uint8_t buf[SAD_MAX_SIZE];
    int enc = sad_encode(&empty, buf, sizeof(buf));
    TASSERT(enc == 4);  /* just the header */

    sad_t decoded;
    int dec = sad_decode(buf, (size_t)enc, &decoded);
    TASSERT(dec == 4);
    TASSERT(decoded.num_fields == 0);

    return errors;
}

/* --------------------------------------------------------------------------
 * Test: SAD validate rejects bad data
 * -------------------------------------------------------------------------- */

static int test_sad_validate_bad(void)
{
    int errors = 0;

    /* Too short */
    uint8_t short_buf[] = { 1, 0 };
    TASSERT(sad_validate(short_buf, 2) != 0);

    /* Bad version */
    uint8_t bad_ver[] = { 99, 0, 0, 0 };
    TASSERT(sad_validate(bad_ver, 4) != 0);

    /* Claims 1 field but no field data */
    uint8_t trunc[] = { 1, 0, 0, 1 };
    TASSERT(sad_validate(trunc, 4) != 0);

    /* Field with wrong length for MODEL_ARCH (should be 4, we say 2) */
    uint8_t bad_flen[] = { 1, 0, 0, 1,   0x01, 0, 2,   0xAA, 0xBB };
    TASSERT(sad_validate(bad_flen, 9) != 0);

    return errors;
}

/* --------------------------------------------------------------------------
 * Test: SAD overflow protection
 * -------------------------------------------------------------------------- */

static int test_sad_overflow(void)
{
    int errors = 0;

    sad_t sad;
    sad_init(&sad);

    /* Fill all 16 fields */
    for (int i = 0; i < SAD_MAX_FIELDS; i++) {
        int rc = sad_add_uint32(&sad, SAD_FIELD_CAPABILITY, (uint32_t)i);
        TASSERT(rc == 0);
    }

    /* 17th should fail */
    int rc = sad_add_uint32(&sad, SAD_FIELD_CAPABILITY, 99);
    TASSERT(rc == -1);

    return errors;
}

/* --------------------------------------------------------------------------
 * Test: SAD match score - perfect match
 * -------------------------------------------------------------------------- */

static int test_match_score_perfect(void)
{
    int errors = 0;

    /* Build query: transformer, text+code gen, 128K ctx, 200ms latency */
    sad_t query;
    sad_init(&query);
    sad_add_uint32(&query, SAD_FIELD_MODEL_ARCH, MODEL_ARCH_TRANSFORMER);
    sad_add_uint32(&query, SAD_FIELD_CAPABILITY, CAP_TEXT_GEN | CAP_CODE_GEN);
    sad_add_uint32(&query, SAD_FIELD_CONTEXT_WINDOW, 131072);
    sad_add_uint32(&query, SAD_FIELD_MAX_LATENCY_MS, 200);

    /* Build candidate that matches perfectly */
    route_entry_t candidate;
    memset(&candidate, 0, sizeof(candidate));
    candidate.node_id[0] = 0x01;
    candidate.latency_us = 50000;   /* 50ms < 200ms */
    candidate.cost_milli = 1000;
    candidate.trust_level = TRUST_FULL_AUDIT;
    candidate.region_code = 840;

    sad_init(&candidate.capabilities);
    sad_add_uint32(&candidate.capabilities, SAD_FIELD_MODEL_ARCH,
                   MODEL_ARCH_TRANSFORMER);
    sad_add_uint32(&candidate.capabilities, SAD_FIELD_CAPABILITY,
                   CAP_TEXT_GEN | CAP_CODE_GEN | CAP_REASONING);
    sad_add_uint32(&candidate.capabilities, SAD_FIELD_CONTEXT_WINDOW, 262144);

    scoring_weights_t w = scoring_weights_default();
    float score = sad_match_score(&query, &candidate, &w);
    TASSERT(score > 0.5f);  /* should be a good match */

    return errors;
}

/* --------------------------------------------------------------------------
 * Test: SAD match score - hard constraint violation (context window)
 * -------------------------------------------------------------------------- */

static int test_match_score_hard_fail(void)
{
    int errors = 0;

    sad_t query;
    sad_init(&query);
    sad_add_uint32(&query, SAD_FIELD_CONTEXT_WINDOW, 131072);

    route_entry_t candidate;
    memset(&candidate, 0, sizeof(candidate));
    candidate.node_id[0] = 0x02;
    sad_init(&candidate.capabilities);
    sad_add_uint32(&candidate.capabilities, SAD_FIELD_CONTEXT_WINDOW, 8192);

    float score = sad_match_score(&query, &candidate, NULL);
    TASSERT(score < 0.0f);  /* hard constraint violated */

    return errors;
}

/* --------------------------------------------------------------------------
 * Test: SAD match score - trust level violation
 * -------------------------------------------------------------------------- */

static int test_match_score_trust_fail(void)
{
    int errors = 0;

    sad_t query;
    sad_init(&query);
    sad_add_uint8(&query, SAD_FIELD_TRUST_LEVEL, TRUST_SAFETY_EVAL);

    route_entry_t candidate;
    memset(&candidate, 0, sizeof(candidate));
    candidate.node_id[0] = 0x03;
    candidate.trust_level = TRUST_IDENTITY;  /* too low */
    sad_init(&candidate.capabilities);

    float score = sad_match_score(&query, &candidate, NULL);
    TASSERT(score < 0.0f);

    return errors;
}

/* --------------------------------------------------------------------------
 * Test: SAD match score - region exclude
 * -------------------------------------------------------------------------- */

static int test_match_score_region_exclude(void)
{
    int errors = 0;

    sad_t query;
    sad_init(&query);
    uint16_t exclude[] = { 156 };  /* exclude China */
    sad_add_regions(&query, SAD_FIELD_REGION_EXCLUDE, exclude, 1);

    route_entry_t candidate;
    memset(&candidate, 0, sizeof(candidate));
    candidate.node_id[0] = 0x04;
    candidate.region_code = 156;  /* in excluded region */
    sad_init(&candidate.capabilities);

    float score = sad_match_score(&query, &candidate, NULL);
    TASSERT(score < 0.0f);

    /* Non-excluded region should pass */
    candidate.region_code = 840;  /* US */
    score = sad_match_score(&query, &candidate, NULL);
    TASSERT(score >= 0.0f);

    return errors;
}

/* --------------------------------------------------------------------------
 * Test: SAD wildcard query matches all
 * -------------------------------------------------------------------------- */

static int test_match_wildcard(void)
{
    int errors = 0;

    sad_t wildcard;
    sad_init(&wildcard);  /* zero fields = wildcard */

    route_entry_t candidate;
    memset(&candidate, 0, sizeof(candidate));
    candidate.node_id[0] = 0x05;
    sad_init(&candidate.capabilities);

    float score = sad_match_score(&wildcard, &candidate, NULL);
    TASSERT(score == 1.0f);  /* wildcard matches everything */

    return errors;
}

/* --------------------------------------------------------------------------
 * Test: sad_find_best - top-K selection
 * -------------------------------------------------------------------------- */

static int test_find_best(void)
{
    int errors = 0;

    /* Build query */
    sad_t query;
    sad_init(&query);
    sad_add_uint32(&query, SAD_FIELD_CAPABILITY, CAP_TEXT_GEN | CAP_CODE_GEN);
    sad_add_uint32(&query, SAD_FIELD_MAX_LATENCY_MS, 500);

    /* Build table of 4 candidates */
    route_entry_t table[4];
    memset(table, 0, sizeof(table));

    /* Candidate 0: has text+code+reasoning, low latency -> best */
    table[0].node_id[0] = 0x10;
    table[0].latency_us = 50000;
    table[0].cost_milli = 1000;
    table[0].trust_level = TRUST_FULL_AUDIT;
    sad_init(&table[0].capabilities);
    sad_add_uint32(&table[0].capabilities, SAD_FIELD_CAPABILITY,
                   CAP_TEXT_GEN | CAP_CODE_GEN | CAP_REASONING);

    /* Candidate 1: has text only, medium latency -> decent */
    table[1].node_id[0] = 0x11;
    table[1].latency_us = 200000;
    table[1].cost_milli = 500;
    table[1].trust_level = TRUST_IDENTITY;
    sad_init(&table[1].capabilities);
    sad_add_uint32(&table[1].capabilities, SAD_FIELD_CAPABILITY, CAP_TEXT_GEN);

    /* Candidate 2: has all caps, very low latency -> also good */
    table[2].node_id[0] = 0x12;
    table[2].latency_us = 30000;
    table[2].cost_milli = 3000;
    table[2].trust_level = TRUST_PROVENANCE;
    sad_init(&table[2].capabilities);
    sad_add_uint32(&table[2].capabilities, SAD_FIELD_CAPABILITY,
                   CAP_TEXT_GEN | CAP_CODE_GEN | CAP_IMAGE_GEN);

    /* Candidate 3: has code only, over latency SLA -> so-so */
    table[3].node_id[0] = 0x13;
    table[3].latency_us = 450000;
    table[3].cost_milli = 200;
    table[3].trust_level = TRUST_NONE;
    sad_init(&table[3].capabilities);
    sad_add_uint32(&table[3].capabilities, SAD_FIELD_CAPABILITY, CAP_CODE_GEN);

    /* Find top 2 */
    resolve_result_t results[2];
    int n = sad_find_best(&query, table, 4, NULL, 2, results);
    TASSERT(n == 2);

    /* First result should be one of the two best (0 or 2) */
    TASSERT(results[0].score >= results[1].score);
    TASSERT(results[0].score > 0.0f);
    TASSERT(results[1].score > 0.0f);

    /* The best should be candidate 0 or 2 (both have text+code, low latency) */
    TASSERT(results[0].entry.node_id[0] == 0x10 ||
            results[0].entry.node_id[0] == 0x12);

    return errors;
}

/* --------------------------------------------------------------------------
 * Registration
 * -------------------------------------------------------------------------- */

void register_sad_tests(void)
{
    test_register("sad_init_and_add_fields",       test_sad_init);
    test_register("sad_regions",                   test_sad_regions);
    test_register("sad_encode_decode_roundtrip",   test_sad_encode_decode);
    test_register("sad_empty_roundtrip",           test_sad_empty_roundtrip);
    test_register("sad_validate_bad_data",         test_sad_validate_bad);
    test_register("sad_overflow_protection",       test_sad_overflow);
    test_register("match_score_perfect",           test_match_score_perfect);
    test_register("match_score_hard_fail_ctx",     test_match_score_hard_fail);
    test_register("match_score_trust_fail",        test_match_score_trust_fail);
    test_register("match_score_region_exclude",    test_match_score_region_exclude);
    test_register("match_wildcard",                test_match_wildcard);
    test_register("find_best_top_k",              test_find_best);
}
