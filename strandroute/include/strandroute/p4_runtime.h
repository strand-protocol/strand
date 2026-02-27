/**
 * p4_runtime.h - P4Runtime / BMv2 Thrift control-plane client for NexRoute
 *
 * Provides a C API for managing P4 table entries in the BMv2 simple_switch
 * process.  Two compilation modes are supported:
 *
 *   Default (stub mode):
 *     Builds without any Thrift dependency.  All operations log to stderr
 *     and return P4RT_OK.  Safe for developer builds and unit tests.
 *
 *   BMv2 Thrift mode (-DBMV2_THRIFT_ENABLED):
 *     Links against the Thrift-generated SimpleSwitch client library and
 *     communicates with a running simple_switch process over TCP.
 *
 * Usage:
 *   // Initialise connection (call once at startup)
 *   if (p4rt_init("localhost", 9090) != P4RT_OK) { ... }
 *
 *   // Install SAD -> node_id mapping
 *   p4rt_sad_table_add(&sad, node_id);
 *
 *   // Install node_id -> egress port mapping
 *   p4rt_node_forward_add(node_id, 1 /* port */);
 *
 *   // Tear down
 *   p4rt_close();
 *
 * Thread safety: all functions acquire an internal mutex; the API is safe
 * to call from multiple threads.
 */

#ifndef NEXROUTE_P4_RUNTIME_H
#define NEXROUTE_P4_RUNTIME_H

#ifdef __cplusplus
extern "C" {
#endif

#include <stdint.h>

/* Forward declaration: defined in nexroute/include/nexroute/sad.h */
typedef struct nexroute_sad nexroute_sad_t;

/* --------------------------------------------------------------------------
 * Return codes
 * -------------------------------------------------------------------------- */

/** Operation succeeded. */
#define P4RT_OK             0

/** Generic error; see errno or p4rt_strerror(). */
#define P4RT_ERR_GENERIC   -1

/** Connection to BMv2 could not be established or was lost. */
#define P4RT_ERR_CONN      -2

/** The requested table entry was not found. */
#define P4RT_ERR_NOT_FOUND -3

/** Invalid argument supplied to the function. */
#define P4RT_ERR_INVAL     -4

/** Table is full; no space for new entries. */
#define P4RT_ERR_FULL      -5

/* --------------------------------------------------------------------------
 * Configuration
 * -------------------------------------------------------------------------- */

/** Default Thrift port used by BMv2 simple_switch. */
#define P4RT_DEFAULT_PORT  9090

/** Default hostname for BMv2 process (may be overridden at p4rt_init time). */
#define P4RT_DEFAULT_HOST  "localhost"

/** Maximum length of the host string (including NUL terminator). */
#define P4RT_MAX_HOST_LEN  256

/* --------------------------------------------------------------------------
 * Lifecycle
 * -------------------------------------------------------------------------- */

/**
 * p4rt_init - Connect to BMv2 simple_switch Thrift interface.
 *
 * @host  Hostname or IP of the BMv2 process.  Pass NULL to use
 *        P4RT_DEFAULT_HOST ("localhost").
 * @port  Thrift TCP port.  Pass 0 to use P4RT_DEFAULT_PORT (9090).
 *
 * Returns P4RT_OK on success, P4RT_ERR_CONN if the connection fails.
 *
 * In stub mode (BMV2_THRIFT_ENABLED not defined) this always returns P4RT_OK
 * and logs a message to stderr.
 */
int p4rt_init(const char *host, int port);

/**
 * p4rt_close - Close the Thrift connection and free resources.
 *
 * Safe to call even if p4rt_init was never called (no-op).
 */
void p4rt_close(void);

/* --------------------------------------------------------------------------
 * SAD table management
 *
 * The SAD ternary-match table (sad_ternary_match in sad_lookup.p4) maps
 * <model_arch, capability_flags, context_window> (with masks) to a resolved
 * destination node_id.
 * -------------------------------------------------------------------------- */

/**
 * p4rt_sad_table_add - Install a SAD -> node_id entry in simple_switch.
 *
 * Extracts model_arch, capability, and context_window from @sad and installs
 * a ternary TCAM entry with exact masks (i.e. exact-value match semantics).
 * For wildcard entries, modify the masks via the lower-level p4rt_raw_entry_*
 * API (future extension).
 *
 * @sad      Pointer to the SAD descriptor.  Must not be NULL.
 * @node_id  16-byte resolved destination node ID.  Must not be NULL.
 *
 * Returns P4RT_OK on success, P4RT_ERR_* on failure.
 */
int p4rt_sad_table_add(const nexroute_sad_t *sad, const uint8_t node_id[16]);

/**
 * p4rt_sad_table_delete - Remove a SAD entry from simple_switch.
 *
 * @sad  Pointer to the SAD descriptor that identifies the entry to remove.
 *
 * Returns P4RT_OK on success, P4RT_ERR_NOT_FOUND if no matching entry exists.
 */
int p4rt_sad_table_delete(const nexroute_sad_t *sad);

/* --------------------------------------------------------------------------
 * Node ID forwarding table management
 *
 * The node_id_forward table (forwarding.p4) maps a 128-bit destination
 * node_id to an egress port number.
 * -------------------------------------------------------------------------- */

/**
 * p4rt_node_forward_add - Install a node_id -> egress_port forwarding entry.
 *
 * @node_id      16-byte destination node ID.  Must not be NULL.
 * @egress_port  Output port index (0-based).  Port 64 is the CPU port.
 *
 * Returns P4RT_OK on success, P4RT_ERR_* on failure.
 */
int p4rt_node_forward_add(const uint8_t node_id[16], int egress_port);

/**
 * p4rt_node_forward_delete - Remove a node_id forwarding entry.
 *
 * @node_id  16-byte destination node ID identifying the entry to remove.
 *
 * Returns P4RT_OK on success, P4RT_ERR_NOT_FOUND if no matching entry exists.
 */
int p4rt_node_forward_delete(const uint8_t node_id[16]);

/* --------------------------------------------------------------------------
 * Diagnostics
 * -------------------------------------------------------------------------- */

/**
 * p4rt_strerror - Return a human-readable string for a P4RT error code.
 *
 * @errcode  One of the P4RT_ERR_* values, or P4RT_OK.
 *
 * Returns a static string; do not free.
 */
const char *p4rt_strerror(int errcode);

/**
 * p4rt_is_connected - Return 1 if a live Thrift connection is open, 0 otherwise.
 */
int p4rt_is_connected(void);

#ifdef __cplusplus
} /* extern "C" */
#endif

#endif /* NEXROUTE_P4_RUNTIME_H */
