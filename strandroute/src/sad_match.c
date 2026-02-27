/*
 * sad_match.c - SAD matching engine
 *
 * Scores a candidate route_entry against a SAD query using weighted
 * multi-constraint scoring per the StrandRoute spec (Section 4.3).
 */

#include "strandroute/sad.h"
#include "strandroute/types.h"

#include <math.h>
#include <string.h>

/* --------------------------------------------------------------------------
 * Byte-order helpers
 * -------------------------------------------------------------------------- */

static inline uint16_t get_be16(const uint8_t *p)
{
    return (uint16_t)((uint16_t)p[0] << 8 | (uint16_t)p[1]);
}

static inline uint32_t get_be32(const uint8_t *p)
{
    return ((uint32_t)p[0] << 24) | ((uint32_t)p[1] << 16) |
           ((uint32_t)p[2] << 8)  | ((uint32_t)p[3]);
}

/* --------------------------------------------------------------------------
 * popcount helper
 * -------------------------------------------------------------------------- */

static inline int popcount32(uint32_t x)
{
    /* Use built-in if available, otherwise Hamming weight */
#if defined(__GNUC__) || defined(__clang__)
    return __builtin_popcount(x);
#else
    x = x - ((x >> 1) & 0x55555555u);
    x = (x & 0x33333333u) + ((x >> 2) & 0x33333333u);
    return (int)(((x + (x >> 4)) & 0x0F0F0F0Fu) * 0x01010101u >> 24);
#endif
}

/* --------------------------------------------------------------------------
 * Field extraction helpers
 * -------------------------------------------------------------------------- */

static uint32_t field_get_u32(const sad_field_t *f)
{
    if (!f || f->length < 4) return 0;
    return get_be32(f->value);
}

static uint8_t field_get_u8(const sad_field_t *f)
{
    if (!f || f->length < 1) return 0;
    return f->value[0];
}

/* Check if a region code appears in a region list field */
static bool region_in_field(uint16_t region, const sad_field_t *f)
{
    if (!f || f->length < 2) return false;
    uint16_t count = f->length / 2;
    for (uint16_t i = 0; i < count; i++) {
        if (get_be16(&f->value[i * 2]) == region)
            return true;
    }
    return false;
}

/* --------------------------------------------------------------------------
 * Per-field match functions
 * -------------------------------------------------------------------------- */

/*
 * MODEL_ARCH: exact match -- 1.0 if equal, 0.0 otherwise.
 * If query does not specify, score is 1.0 (no constraint).
 */
static float match_model_arch(const sad_t *query, const sad_t *candidate)
{
    const sad_field_t *qf = sad_find_field(query, SAD_FIELD_MODEL_ARCH);
    if (!qf) return 1.0f;  /* no constraint */

    const sad_field_t *cf = sad_find_field(candidate, SAD_FIELD_MODEL_ARCH);
    if (!cf) return 0.0f;  /* query requires, candidate doesn't have */

    return (field_get_u32(qf) == field_get_u32(cf)) ? 1.0f : 0.0f;
}

/*
 * CAPABILITY: popcount(candidate & query) / popcount(query)
 * Measures what fraction of required capabilities are present.
 */
static float match_capability(const sad_t *query, const sad_t *candidate)
{
    const sad_field_t *qf = sad_find_field(query, SAD_FIELD_CAPABILITY);
    if (!qf) return 1.0f;

    uint32_t q_caps = field_get_u32(qf);
    if (q_caps == 0) return 1.0f;

    const sad_field_t *cf = sad_find_field(candidate, SAD_FIELD_CAPABILITY);
    if (!cf) return 0.0f;

    uint32_t c_caps = field_get_u32(cf);
    uint32_t matched = c_caps & q_caps;

    int q_pop = popcount32(q_caps);
    int m_pop = popcount32(matched);

    return (float)m_pop / (float)q_pop;
}

/*
 * CONTEXT_WINDOW: hard constraint.
 * 1.0 if candidate >= query, 0.0 otherwise.
 */
static float match_context_window(const sad_t *query, const sad_t *candidate)
{
    const sad_field_t *qf = sad_find_field(query, SAD_FIELD_CONTEXT_WINDOW);
    if (!qf) return 1.0f;

    const sad_field_t *cf = sad_find_field(candidate, SAD_FIELD_CONTEXT_WINDOW);
    if (!cf) return 0.0f;

    uint32_t q_ctx = field_get_u32(qf);
    uint32_t c_ctx = field_get_u32(cf);

    return (c_ctx >= q_ctx) ? 1.0f : 0.0f;
}

/*
 * LATENCY: max(0, 1.0 - (candidate_latency / query_max_latency))
 * Uses the route entry's measured latency_us (converted to ms) against
 * the query's MAX_LATENCY_MS constraint.
 */
static float match_latency(const sad_t *query, uint32_t candidate_latency_us)
{
    const sad_field_t *qf = sad_find_field(query, SAD_FIELD_MAX_LATENCY_MS);
    if (!qf) return 1.0f;

    uint32_t max_ms = field_get_u32(qf);
    if (max_ms == 0) return 0.0f;

    float cand_ms = (float)candidate_latency_us / 1000.0f;
    float score = 1.0f - (cand_ms / (float)max_ms);
    return (score > 0.0f) ? score : 0.0f;
}

/*
 * COST: max(0, 1.0 - (candidate_cost / query_max_cost))
 */
static float match_cost(const sad_t *query, uint32_t candidate_cost_milli)
{
    const sad_field_t *qf = sad_find_field(query, SAD_FIELD_MAX_COST_MILLI);
    if (!qf) return 1.0f;

    uint32_t max_cost = field_get_u32(qf);
    if (max_cost == 0) return 0.0f;

    float score = 1.0f - ((float)candidate_cost_milli / (float)max_cost);
    return (score > 0.0f) ? score : 0.0f;
}

/*
 * TRUST_LEVEL: hard constraint.
 * 1.0 if candidate >= query, 0.0 otherwise.
 */
static float match_trust(const sad_t *query, uint8_t candidate_trust)
{
    const sad_field_t *qf = sad_find_field(query, SAD_FIELD_TRUST_LEVEL);
    if (!qf) return 1.0f;

    uint8_t required = field_get_u8(qf);
    return (candidate_trust >= required) ? 1.0f : 0.0f;
}

/*
 * REGION_PREFER: 1.0 if candidate's region is in preferred list, 0.5 otherwise.
 */
static float match_region_prefer(const sad_t *query, uint16_t candidate_region)
{
    const sad_field_t *qf = sad_find_field(query, SAD_FIELD_REGION_PREFER);
    if (!qf) return 1.0f;

    return region_in_field(candidate_region, qf) ? 1.0f : 0.5f;
}

/*
 * REGION_EXCLUDE: hard constraint.
 * Returns -INFINITY if candidate's region is in exclude list, 1.0 otherwise.
 */
static float match_region_exclude(const sad_t *query, uint16_t candidate_region)
{
    const sad_field_t *qf = sad_find_field(query, SAD_FIELD_REGION_EXCLUDE);
    if (!qf) return 1.0f;

    return region_in_field(candidate_region, qf) ? -INFINITY : 1.0f;
}

/* --------------------------------------------------------------------------
 * sad_match_score - Compute composite match score
 *
 * Returns a float in [0.0, 1.0] (or negative if a hard constraint
 * is violated, which means the candidate is disqualified).
 * -------------------------------------------------------------------------- */

float sad_match_score(const sad_t *query, const route_entry_t *candidate,
                      const scoring_weights_t *weights)
{
    if (!query || !candidate)
        return -1.0f;

    scoring_weights_t w;
    if (weights) {
        w = *weights;
    } else {
        w = scoring_weights_default();
    }

    const sad_t *cand_sad = &candidate->capabilities;

    /* Wildcard: zero-field query matches everything with score 1.0 */
    if (query->num_fields == 0)
        return 1.0f;

    /* Hard constraints: check first, reject immediately */
    float ctx_score = match_context_window(query, cand_sad);
    if (ctx_score <= 0.0f)
        return -1.0f;

    float trust_score = match_trust(query, candidate->trust_level);
    if (trust_score <= 0.0f)
        return -1.0f;

    float region_excl = match_region_exclude(query, candidate->region_code);
    if (region_excl < 0.0f)
        return -1.0f;

    /* Soft constraints: weighted sum */
    float arch_score  = match_model_arch(query, cand_sad);
    float cap_score   = match_capability(query, cand_sad);
    float lat_score   = match_latency(query, candidate->latency_us);
    float cost_score  = match_cost(query, candidate->cost_milli);
    float region_pref = match_region_prefer(query, candidate->region_code);

    /*
     * Composite score = weighted sum of all field scores.
     * The architecture score is multiplied with capability weight
     * since it's closely related. Region preference is a bonus.
     */
    float score = 0.0f;
    score += w.capability     * cap_score;
    score += w.latency        * lat_score;
    score += w.cost           * cost_score;
    score += w.context_window * ctx_score;
    score += w.trust          * trust_score;

    /* Modifiers (not part of weight sum, act as multipliers) */
    if (arch_score <= 0.0f)
        return -1.0f;  /* Model arch mismatch is a hard reject */

    /* Region preference adjusts the final score */
    score *= region_pref;

    /* Clamp to [0.0, 1.0] */
    if (score > 1.0f) score = 1.0f;
    if (score < 0.0f) score = 0.0f;

    return score;
}

/* --------------------------------------------------------------------------
 * sad_find_best - Find top-K matches from a flat array of entries
 *
 * Performs a linear scan, keeping the top-K results sorted by score.
 * Returns the number of results written (up to top_k).
 * -------------------------------------------------------------------------- */

int sad_find_best(const sad_t *query, const route_entry_t *table,
                  int table_size, const scoring_weights_t *weights,
                  int top_k, resolve_result_t *results)
{
    if (!query || !table || !results || top_k <= 0 || table_size <= 0)
        return 0;

    int count = 0;

    for (int i = 0; i < table_size; i++) {
        float score = sad_match_score(query, &table[i], weights);
        if (score < 0.0f)
            continue;  /* Disqualified by hard constraint */

        /* Insertion sort into results array */
        if (count < top_k) {
            /* Still room, just insert */
            int pos = count;
            while (pos > 0 && results[pos - 1].score < score) {
                results[pos] = results[pos - 1];
                pos--;
            }
            results[pos].entry = table[i];
            results[pos].score = score;
            count++;
        } else if (score > results[count - 1].score) {
            /* Better than the worst in the list, replace it */
            int pos = count - 1;
            while (pos > 0 && results[pos - 1].score < score) {
                results[pos] = results[pos - 1];
                pos--;
            }
            results[pos].entry = table[i];
            results[pos].score = score;
        }
    }

    return count;
}
