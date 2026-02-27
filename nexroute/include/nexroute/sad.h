/*
 * sad.h - Semantic Address Descriptor (SAD) API
 *
 * Provides encoding, decoding, and validation of SAD binary format.
 * Binary TLV format: version(1B), flags(1B), num_fields(2B),
 * then TLV fields each with type(1B), length(2B), value(variable).
 */

#ifndef NEXROUTE_SAD_H
#define NEXROUTE_SAD_H

#include "nexroute/types.h"

#ifdef __cplusplus
extern "C" {
#endif

/* --------------------------------------------------------------------------
 * SAD Initialization
 * -------------------------------------------------------------------------- */

/**
 * Initialize an empty SAD with default version and zero fields.
 */
void sad_init(sad_t *sad);

/* --------------------------------------------------------------------------
 * SAD Field Builders
 * -------------------------------------------------------------------------- */

/**
 * Add a field to the SAD. Returns 0 on success, -1 if the SAD is full.
 */
int sad_add_field(sad_t *sad, sad_field_type_t type,
                  const void *value, uint16_t length);

/**
 * Convenience: add a uint32 field (MODEL_ARCH, CAPABILITY, CONTEXT_WINDOW, etc).
 */
int sad_add_uint32(sad_t *sad, sad_field_type_t type, uint32_t value);

/**
 * Convenience: add a uint8 field (TRUST_LEVEL).
 */
int sad_add_uint8(sad_t *sad, sad_field_type_t type, uint8_t value);

/**
 * Convenience: add a region list (REGION_PREFER, REGION_EXCLUDE).
 * regions is an array of uint16 region codes; count is the number of entries.
 */
int sad_add_regions(sad_t *sad, sad_field_type_t type,
                    const uint16_t *regions, uint16_t count);

/* --------------------------------------------------------------------------
 * SAD Field Lookup
 * -------------------------------------------------------------------------- */

/**
 * Find a field by type. Returns pointer to field or NULL if not found.
 */
const sad_field_t *sad_find_field(const sad_t *sad, sad_field_type_t type);

/**
 * Extract a uint32 value from a field. Returns 0 if field is not found.
 */
uint32_t sad_get_uint32(const sad_t *sad, sad_field_type_t type);

/**
 * Extract a uint8 value from a field. Returns 0 if field is not found.
 */
uint8_t sad_get_uint8(const sad_t *sad, sad_field_type_t type);

/* --------------------------------------------------------------------------
 * SAD Binary Encoding/Decoding
 *
 * Wire format:
 *   [version:1][flags:1][num_fields:2] (4 bytes header)
 *   For each field:
 *     [type:1][length:2][value:length]
 *
 * All multi-byte integers are in network byte order (big-endian).
 * -------------------------------------------------------------------------- */

/**
 * Encode a sad_t to binary buffer.
 *
 * @param sad     The SAD to encode.
 * @param buf     Output buffer (must be at least SAD_MAX_SIZE bytes).
 * @param buf_len Size of output buffer.
 * @return        Number of bytes written, or -1 on error.
 */
int sad_encode(const sad_t *sad, uint8_t *buf, size_t buf_len);

/**
 * Decode binary buffer into a sad_t.
 *
 * @param buf     Input buffer containing encoded SAD.
 * @param buf_len Length of input data.
 * @param sad     Output SAD struct.
 * @return        Number of bytes consumed, or -1 on error.
 */
int sad_decode(const uint8_t *buf, size_t buf_len, sad_t *sad);

/**
 * Validate an encoded SAD buffer without full decode.
 * Checks: version, field lengths match total_length, known types have
 * correct lengths.
 *
 * @return 0 if valid, negative error code otherwise.
 */
int sad_validate(const uint8_t *buf, size_t buf_len);

#ifdef __cplusplus
}
#endif

#endif /* NEXROUTE_SAD_H */
