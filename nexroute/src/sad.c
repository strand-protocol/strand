/*
 * sad.c - SAD encoding, decoding, and validation
 *
 * Binary TLV wire format (all multi-byte integers big-endian):
 *   Header (4 bytes):
 *     [version:1][flags:1][num_fields:2]
 *   Per field:
 *     [type:1][length:2][value:length]
 */

#include "nexroute/sad.h"

#include <string.h>

/* --------------------------------------------------------------------------
 * Byte-order helpers (big-endian on the wire)
 * -------------------------------------------------------------------------- */

static inline void put_be16(uint8_t *p, uint16_t v)
{
    p[0] = (uint8_t)(v >> 8);
    p[1] = (uint8_t)(v);
}

static inline uint16_t get_be16(const uint8_t *p)
{
    return (uint16_t)((uint16_t)p[0] << 8 | (uint16_t)p[1]);
}

static inline void put_be32(uint8_t *p, uint32_t v)
{
    p[0] = (uint8_t)(v >> 24);
    p[1] = (uint8_t)(v >> 16);
    p[2] = (uint8_t)(v >> 8);
    p[3] = (uint8_t)(v);
}

static inline uint32_t get_be32(const uint8_t *p)
{
    return ((uint32_t)p[0] << 24) | ((uint32_t)p[1] << 16) |
           ((uint32_t)p[2] << 8)  | ((uint32_t)p[3]);
}

/* --------------------------------------------------------------------------
 * Header size: version(1) + flags(1) + num_fields(2) = 4 bytes
 * Per-field overhead: type(1) + length(2) = 3 bytes
 * -------------------------------------------------------------------------- */

#define SAD_HEADER_SIZE  4
#define SAD_FIELD_HDR    3

/* --------------------------------------------------------------------------
 * sad_init
 * -------------------------------------------------------------------------- */

void sad_init(sad_t *sad)
{
    memset(sad, 0, sizeof(*sad));
    sad->version = SAD_VERSION;
}

/* --------------------------------------------------------------------------
 * sad_add_field
 * -------------------------------------------------------------------------- */

int sad_add_field(sad_t *sad, sad_field_type_t type,
                  const void *value, uint16_t length)
{
    if (!sad || !value)
        return -1;
    if (sad->num_fields >= SAD_MAX_FIELDS)
        return -1;
    if (length > SAD_MAX_FIELD_VALUE)
        return -1;

    sad_field_t *f = &sad->fields[sad->num_fields];
    f->type   = type;
    f->length = length;
    memcpy(f->value, value, length);
    sad->num_fields++;
    return 0;
}

/* --------------------------------------------------------------------------
 * Convenience builders
 * -------------------------------------------------------------------------- */

int sad_add_uint32(sad_t *sad, sad_field_type_t type, uint32_t value)
{
    uint8_t buf[4];
    put_be32(buf, value);
    return sad_add_field(sad, type, buf, 4);
}

int sad_add_uint8(sad_t *sad, sad_field_type_t type, uint8_t value)
{
    return sad_add_field(sad, type, &value, 1);
}

int sad_add_regions(sad_t *sad, sad_field_type_t type,
                    const uint16_t *regions, uint16_t count)
{
    if (!regions || count == 0)
        return -1;
    uint16_t byte_len = count * 2;
    if (byte_len > SAD_MAX_FIELD_VALUE)
        return -1;

    uint8_t buf[SAD_MAX_FIELD_VALUE];
    for (uint16_t i = 0; i < count; i++) {
        put_be16(&buf[i * 2], regions[i]);
    }
    return sad_add_field(sad, type, buf, byte_len);
}

/* --------------------------------------------------------------------------
 * sad_find_field
 * -------------------------------------------------------------------------- */

const sad_field_t *sad_find_field(const sad_t *sad, sad_field_type_t type)
{
    if (!sad)
        return NULL;
    for (uint16_t i = 0; i < sad->num_fields; i++) {
        if (sad->fields[i].type == type)
            return &sad->fields[i];
    }
    return NULL;
}

/* --------------------------------------------------------------------------
 * sad_get_uint32 / sad_get_uint8
 * -------------------------------------------------------------------------- */

uint32_t sad_get_uint32(const sad_t *sad, sad_field_type_t type)
{
    const sad_field_t *f = sad_find_field(sad, type);
    if (!f || f->length < 4)
        return 0;
    return get_be32(f->value);
}

uint8_t sad_get_uint8(const sad_t *sad, sad_field_type_t type)
{
    const sad_field_t *f = sad_find_field(sad, type);
    if (!f || f->length < 1)
        return 0;
    return f->value[0];
}

/* --------------------------------------------------------------------------
 * sad_encode
 * -------------------------------------------------------------------------- */

int sad_encode(const sad_t *sad, uint8_t *buf, size_t buf_len)
{
    if (!sad || !buf)
        return -1;

    /* Calculate total size */
    size_t total = SAD_HEADER_SIZE;
    for (uint16_t i = 0; i < sad->num_fields; i++) {
        total += SAD_FIELD_HDR + sad->fields[i].length;
    }

    if (total > buf_len || total > SAD_MAX_SIZE)
        return -1;

    /* Write header */
    size_t off = 0;
    buf[off++] = sad->version;
    buf[off++] = sad->flags;
    put_be16(&buf[off], sad->num_fields);
    off += 2;

    /* Write each field */
    for (uint16_t i = 0; i < sad->num_fields; i++) {
        const sad_field_t *f = &sad->fields[i];
        buf[off++] = (uint8_t)f->type;
        put_be16(&buf[off], f->length);
        off += 2;
        memcpy(&buf[off], f->value, f->length);
        off += f->length;
    }

    return (int)off;
}

/* --------------------------------------------------------------------------
 * sad_decode
 * -------------------------------------------------------------------------- */

int sad_decode(const uint8_t *buf, size_t buf_len, sad_t *sad)
{
    if (!buf || !sad)
        return -1;
    if (buf_len < SAD_HEADER_SIZE)
        return -1;

    memset(sad, 0, sizeof(*sad));

    size_t off = 0;
    sad->version = buf[off++];
    sad->flags   = buf[off++];
    sad->num_fields = get_be16(&buf[off]);
    off += 2;

    if (sad->version != SAD_VERSION)
        return -1;
    if (sad->num_fields > SAD_MAX_FIELDS)
        return -1;

    for (uint16_t i = 0; i < sad->num_fields; i++) {
        if (off + SAD_FIELD_HDR > buf_len)
            return -1;

        sad_field_t *f = &sad->fields[i];
        f->type   = (sad_field_type_t)buf[off++];
        f->length = get_be16(&buf[off]);
        off += 2;

        if (f->length > SAD_MAX_FIELD_VALUE)
            return -1;
        if (off + f->length > buf_len)
            return -1;

        memcpy(f->value, &buf[off], f->length);
        off += f->length;
    }

    sad->total_length = (uint16_t)off;
    return (int)off;
}

/* --------------------------------------------------------------------------
 * sad_validate
 * -------------------------------------------------------------------------- */

int sad_validate(const uint8_t *buf, size_t buf_len)
{
    if (!buf || buf_len < SAD_HEADER_SIZE)
        return -1;

    uint8_t version = buf[0];
    if (version != SAD_VERSION)
        return -2;

    uint16_t num_fields = get_be16(&buf[2]);
    if (num_fields > SAD_MAX_FIELDS)
        return -3;

    size_t off = SAD_HEADER_SIZE;
    for (uint16_t i = 0; i < num_fields; i++) {
        if (off + SAD_FIELD_HDR > buf_len)
            return -4;

        uint8_t ftype = buf[off];
        uint16_t flen = get_be16(&buf[off + 1]);
        off += SAD_FIELD_HDR;

        if (flen > SAD_MAX_FIELD_VALUE)
            return -5;
        if (off + flen > buf_len)
            return -6;

        /* Validate known field lengths */
        switch (ftype) {
        case SAD_FIELD_MODEL_ARCH:
        case SAD_FIELD_CAPABILITY:
        case SAD_FIELD_CONTEXT_WINDOW:
        case SAD_FIELD_MAX_LATENCY_MS:
        case SAD_FIELD_MAX_COST_MILLI:
        case SAD_FIELD_MIN_BENCHMARK:
            if (flen != 4)
                return -7;
            break;
        case SAD_FIELD_TRUST_LEVEL:
            if (flen != 1)
                return -7;
            break;
        case SAD_FIELD_PUBLISHER_ID:
            if (flen != 16)
                return -7;
            break;
        case SAD_FIELD_REGION_PREFER:
        case SAD_FIELD_REGION_EXCLUDE:
            if (flen == 0 || (flen % 2) != 0)
                return -7;
            break;
        case SAD_FIELD_CUSTOM:
            /* Any length is valid for custom */
            break;
        default:
            /* Unknown type - skip but don't reject (forward compat) */
            break;
        }

        off += flen;
    }

    return 0;
}
