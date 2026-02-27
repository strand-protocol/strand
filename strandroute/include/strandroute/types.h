/*
 * types.h - NexRoute core types
 *
 * Defines NodeID, SemanticField, SADCapability, RoutingEntry and related
 * constants used throughout the NexRoute semantic routing layer.
 */

#ifndef NEXROUTE_TYPES_H
#define NEXROUTE_TYPES_H

#include <stdint.h>
#include <stddef.h>
#include <stdbool.h>
#include <string.h>

#include "nexroute/nexlink_compat.h"

#ifdef __cplusplus
extern "C" {
#endif

/* --------------------------------------------------------------------------
 * SAD Constants
 * -------------------------------------------------------------------------- */

#define SAD_VERSION         1
#define SAD_MAX_FIELDS      16
#define SAD_MAX_SIZE        512     /* Maximum encoded SAD size in bytes */
#define SAD_MAX_FIELD_VALUE 64      /* Maximum value length per field */

/* --------------------------------------------------------------------------
 * SAD Field Types (from spec Section 3.2)
 * -------------------------------------------------------------------------- */

typedef enum {
    SAD_FIELD_MODEL_ARCH      = 0x01,
    SAD_FIELD_CAPABILITY      = 0x02,
    SAD_FIELD_CONTEXT_WINDOW  = 0x03,
    SAD_FIELD_MAX_LATENCY_MS  = 0x04,
    SAD_FIELD_MAX_COST_MILLI  = 0x05,
    SAD_FIELD_TRUST_LEVEL     = 0x06,
    SAD_FIELD_REGION_PREFER   = 0x07,
    SAD_FIELD_REGION_EXCLUDE  = 0x08,
    SAD_FIELD_PUBLISHER_ID    = 0x09,
    SAD_FIELD_MIN_BENCHMARK   = 0x0A,
    SAD_FIELD_CUSTOM          = 0x0B,
} sad_field_type_t;

/* --------------------------------------------------------------------------
 * Model Architecture Enums
 * -------------------------------------------------------------------------- */

typedef enum {
    MODEL_ARCH_TRANSFORMER = 0x01,
    MODEL_ARCH_DIFFUSION   = 0x02,
    MODEL_ARCH_MOE         = 0x03,
    MODEL_ARCH_CNN         = 0x04,
    MODEL_ARCH_RNN         = 0x05,
    MODEL_ARCH_RL_AGENT    = 0x06,
} model_arch_t;

/* --------------------------------------------------------------------------
 * Capability Bitfield Flags (from spec Section 3.2)
 * -------------------------------------------------------------------------- */

#define CAP_TEXT_GEN        (1u << 0)
#define CAP_CODE_GEN        (1u << 1)
#define CAP_IMAGE_GEN       (1u << 2)
#define CAP_AUDIO_GEN       (1u << 3)
#define CAP_EMBEDDING       (1u << 4)
#define CAP_CLASSIFICATION  (1u << 5)
#define CAP_TOOL_USE        (1u << 6)
#define CAP_REASONING       (1u << 7)

/* --------------------------------------------------------------------------
 * Trust Levels (from spec Section 3.2)
 * -------------------------------------------------------------------------- */

typedef enum {
    TRUST_NONE        = 0,
    TRUST_IDENTITY    = 1,
    TRUST_PROVENANCE  = 2,
    TRUST_SAFETY_EVAL = 3,
    TRUST_FULL_AUDIT  = 4,
} trust_level_t;

/* --------------------------------------------------------------------------
 * SAD Field
 * -------------------------------------------------------------------------- */

typedef struct {
    sad_field_type_t type;
    uint16_t         length;   /* Length of value in bytes */
    uint8_t          value[SAD_MAX_FIELD_VALUE];
} sad_field_t;

/* --------------------------------------------------------------------------
 * Semantic Address Descriptor (SAD)
 * -------------------------------------------------------------------------- */

typedef struct {
    uint8_t     version;
    uint8_t     flags;
    uint16_t    num_fields;
    uint16_t    total_length;   /* Total encoded length in bytes */
    sad_field_t fields[SAD_MAX_FIELDS];
} sad_t;

/* --------------------------------------------------------------------------
 * Routing Entry - A single route in the capability routing table
 * -------------------------------------------------------------------------- */

typedef struct {
    uint8_t  node_id[NEXLINK_NODE_ID_LEN];  /* NexLink Node ID (16 bytes) */
    sad_t    capabilities;                   /* What this node offers */
    uint32_t latency_us;                     /* Current measured latency (microseconds) */
    float    load_factor;                    /* 0.0 - 1.0 current load */
    uint32_t cost_milli;                     /* Cost per request (millionths of dollar) */
    uint8_t  trust_level;                    /* NexTrust attestation level */
    uint16_t region_code;                    /* ISO 3166-1 numeric */
    uint64_t last_updated;                   /* Timestamp of last gossip update (ns) */
    uint64_t ttl_ns;                         /* Time-to-live for this entry (ns) */
} route_entry_t;

/* --------------------------------------------------------------------------
 * Resolution Result - Returned by the resolver
 * -------------------------------------------------------------------------- */

typedef struct {
    route_entry_t entry;
    float         score;     /* Composite match score [0.0, 1.0] */
} resolve_result_t;

/* --------------------------------------------------------------------------
 * Scoring Weights - Configurable per deployment
 * -------------------------------------------------------------------------- */

typedef struct {
    float capability;       /* Default: 0.30 */
    float latency;          /* Default: 0.25 */
    float cost;             /* Default: 0.20 */
    float context_window;   /* Default: 0.15 */
    float trust;            /* Default: 0.10 */
} scoring_weights_t;

/* Default scoring weights */
static inline scoring_weights_t scoring_weights_default(void)
{
    scoring_weights_t w = {
        .capability     = 0.30f,
        .latency        = 0.25f,
        .cost           = 0.20f,
        .context_window = 0.15f,
        .trust          = 0.10f,
    };
    return w;
}

/* --------------------------------------------------------------------------
 * Utility: compare two node IDs
 * -------------------------------------------------------------------------- */

static inline bool node_id_equal(const uint8_t a[NEXLINK_NODE_ID_LEN],
                                 const uint8_t b[NEXLINK_NODE_ID_LEN])
{
    return memcmp(a, b, NEXLINK_NODE_ID_LEN) == 0;
}

static inline void node_id_copy(uint8_t dst[NEXLINK_NODE_ID_LEN],
                                const uint8_t src[NEXLINK_NODE_ID_LEN])
{
    memcpy(dst, src, NEXLINK_NODE_ID_LEN);
}

static inline bool node_id_is_zero(const uint8_t id[NEXLINK_NODE_ID_LEN])
{
    for (int i = 0; i < NEXLINK_NODE_ID_LEN; i++) {
        if (id[i] != 0) return false;
    }
    return true;
}

#ifdef __cplusplus
}
#endif

#endif /* NEXROUTE_TYPES_H */
