/*
 * p4_runtime.c - P4Runtime / BMv2 Thrift control-plane client for NexRoute
 *
 * Provides table entry management for the NexRoute P4 pipeline running on
 * BMv2 simple_switch.  Two compilation modes:
 *
 *   Default (stub mode, no Thrift dependency):
 *     All operations log to stderr and return P4RT_OK.  This lets the
 *     codebase build and unit-test without a BMv2 installation.
 *
 *   BMv2 Thrift mode (-DBMV2_THRIFT_ENABLED):
 *     Links against the Thrift-generated SimpleSwitch RPC library and
 *     communicates with a running simple_switch process over TCP.
 *
 *     Required libraries (passed via CMake P4_RUNTIME option):
 *       -lthrift -lsimple_switch_thrift
 *
 *     Required include paths:
 *       -I <bmv2_src>/targets/simple_switch/thrift/gen-cpp
 *       -I <thrift_install>/include
 *
 * Table mapping (see nexroute/p4/*.p4):
 *   sad_ternary_match  : (model_arch, capability_flags, context_window) -> node_id
 *   node_id_forward    : (dst_node_id)                                  -> egress_port
 */

#include "nexroute/p4_runtime.h"
#include "nexroute/sad.h"
#include "nexroute/types.h"

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>
#include <pthread.h>
#include <errno.h>

/* --------------------------------------------------------------------------
 * Optional Thrift includes
 *
 * When BMV2_THRIFT_ENABLED is defined, the caller must supply the BMv2
 * simple_switch Thrift-generated header path via -I flags:
 *
 *   -I <bmv2>/targets/simple_switch/thrift/gen-cpp
 *   -I <thrift>/include
 *
 * We guard these includes so the file compiles cleanly without them.
 * -------------------------------------------------------------------------- */

#ifdef BMV2_THRIFT_ENABLED
/*
 * BMv2 Thrift headers.  The path below is the conventional install location;
 * override via -DBMV2_THRIFT_INCLUDE_DIR=<path> in CMake.
 */
#  ifndef BMV2_THRIFT_INCLUDE_DIR
#    define BMV2_THRIFT_INCLUDE_DIR "."
#  endif

/* Thrift C++ bindings -- must be linked with -lthrift */
#  include <thrift/transport/TSocket.h>
#  include <thrift/transport/TBufferTransports.h>
#  include <thrift/protocol/TBinaryProtocol.h>

/* BMv2 simple_switch Thrift-generated client */
#  include "SimpleSwitch.h"

using namespace apache::thrift;
using namespace apache::thrift::protocol;
using namespace apache::thrift::transport;
#endif /* BMV2_THRIFT_ENABLED */

/* --------------------------------------------------------------------------
 * Internal state
 * -------------------------------------------------------------------------- */

/** Global mutex protecting all p4rt_* functions. */
static pthread_mutex_t g_p4rt_lock = PTHREAD_MUTEX_INITIALIZER;

/** Connection metadata. */
static struct {
    int  connected;             /**< 1 if open, 0 if closed. */
    char host[P4RT_MAX_HOST_LEN];
    int  port;
} g_p4rt_state = {
    .connected = 0,
    .host      = P4RT_DEFAULT_HOST,
    .port      = P4RT_DEFAULT_PORT,
};

#ifdef BMV2_THRIFT_ENABLED
/* Thrift client objects (heap-allocated, owned here). */
static std::shared_ptr<TTransport>          g_transport;
static std::shared_ptr<SimpleSwitch::Client> g_client;
#endif

/* --------------------------------------------------------------------------
 * Utility helpers
 * -------------------------------------------------------------------------- */

/**
 * format_node_id - Format a 16-byte node_id as a hex string.
 *
 * @id    16-byte node ID buffer.
 * @buf   Output string buffer; must be at least 33 bytes.
 */
static void format_node_id(const uint8_t id[16], char buf[33])
{
    int i;
    for (i = 0; i < 16; i++) {
        snprintf(buf + i * 2, 3, "%02x", id[i]);
    }
    buf[32] = '\0';
}

/**
 * extract_sad_keys - Pull the three TCAM key fields out of a sad_t.
 *
 * Returns 0 on success, -1 if the SAD pointer is NULL.
 */
static int extract_sad_keys(const nexroute_sad_t *sad,
                             uint32_t *model_arch,
                             uint32_t *capability,
                             uint32_t *context_window)
{
    if (!sad) return -1;

    *model_arch     = sad_get_uint32(sad, SAD_FIELD_MODEL_ARCH);
    *capability     = sad_get_uint32(sad, SAD_FIELD_CAPABILITY);
    *context_window = sad_get_uint32(sad, SAD_FIELD_CONTEXT_WINDOW);
    return 0;
}

/* --------------------------------------------------------------------------
 * Lifecycle
 * -------------------------------------------------------------------------- */

int p4rt_init(const char *host, int port)
{
    int rc = P4RT_OK;

    pthread_mutex_lock(&g_p4rt_lock);

    /* Resolve defaults. */
    if (!host || host[0] == '\0') host = P4RT_DEFAULT_HOST;
    if (port <= 0)                port = P4RT_DEFAULT_PORT;

    strncpy(g_p4rt_state.host, host, P4RT_MAX_HOST_LEN - 1);
    g_p4rt_state.host[P4RT_MAX_HOST_LEN - 1] = '\0';
    g_p4rt_state.port = port;

#ifdef BMV2_THRIFT_ENABLED
    /*
     * Thrift mode: open a buffered TCP connection to simple_switch.
     *
     * TSocket     -- raw TCP transport.
     * TBufferedTransport -- buffers writes until flush; reduces syscalls.
     * TBinaryProtocol    -- BMv2's wire protocol.
     */
    try {
        std::shared_ptr<TSocket> socket =
            std::make_shared<TSocket>(g_p4rt_state.host, g_p4rt_state.port);
        socket->setConnTimeout(2000 /* ms */);
        socket->setRecvTimeout(5000 /* ms */);
        socket->setSendTimeout(5000 /* ms */);

        g_transport = std::make_shared<TBufferedTransport>(socket);
        std::shared_ptr<TProtocol> protocol =
            std::make_shared<TBinaryProtocol>(g_transport);

        g_client    = std::make_shared<SimpleSwitch::Client>(protocol);
        g_transport->open();

        g_p4rt_state.connected = 1;
        fprintf(stderr, "[p4rt] Connected to BMv2 at %s:%d\n",
                g_p4rt_state.host, g_p4rt_state.port);
    } catch (const TException &ex) {
        fprintf(stderr, "[p4rt] ERROR: failed to connect to BMv2 at %s:%d: %s\n",
                g_p4rt_state.host, g_p4rt_state.port, ex.what());
        rc = P4RT_ERR_CONN;
    }
#else
    /*
     * Stub mode: pretend we connected successfully.
     */
    g_p4rt_state.connected = 1;
    fprintf(stderr, "[p4rt] STUB: init called (host=%s port=%d) -- "
            "no BMv2 Thrift connection (recompile with -DBMV2_THRIFT_ENABLED)\n",
            g_p4rt_state.host, g_p4rt_state.port);
#endif /* BMV2_THRIFT_ENABLED */

    pthread_mutex_unlock(&g_p4rt_lock);
    return rc;
}

void p4rt_close(void)
{
    pthread_mutex_lock(&g_p4rt_lock);

    if (!g_p4rt_state.connected) {
        pthread_mutex_unlock(&g_p4rt_lock);
        return;
    }

#ifdef BMV2_THRIFT_ENABLED
    try {
        if (g_transport && g_transport->isOpen()) {
            g_transport->close();
        }
    } catch (const TException &ex) {
        fprintf(stderr, "[p4rt] WARNING: error closing transport: %s\n", ex.what());
    }
    g_client    = nullptr;
    g_transport = nullptr;
#else
    fprintf(stderr, "[p4rt] STUB: close called\n");
#endif

    g_p4rt_state.connected = 0;
    pthread_mutex_unlock(&g_p4rt_lock);
}

/* --------------------------------------------------------------------------
 * SAD table management
 * -------------------------------------------------------------------------- */

int p4rt_sad_table_add(const nexroute_sad_t *sad, const uint8_t node_id[16])
{
    uint32_t model_arch = 0, capability = 0, context_window = 0;
    char node_hex[33];
    int rc = P4RT_OK;

    if (!sad || !node_id) return P4RT_ERR_INVAL;

    if (extract_sad_keys(sad, &model_arch, &capability, &context_window) != 0)
        return P4RT_ERR_INVAL;

    format_node_id(node_id, node_hex);

    pthread_mutex_lock(&g_p4rt_lock);

    if (!g_p4rt_state.connected) {
        pthread_mutex_unlock(&g_p4rt_lock);
        return P4RT_ERR_CONN;
    }

#ifdef BMV2_THRIFT_ENABLED
    /*
     * Install a ternary entry in the sad_ternary_match table.
     *
     * Table:    MyIngress.sad_ternary_match
     * Action:   MyIngress.set_resolved_node
     *
     * Key fields (ternary: value + mask):
     *   meta.model_arch        : 0x%08x &&& 0xFFFFFFFF  (exact)
     *   meta.capability_flags  : 0x%08x &&& 0xFFFFFFFF  (exact)
     *   meta.context_window    : 0x%08x &&& 0xFFFFFFFF  (exact)
     *
     * Action parameters:
     *   node_id : 16 bytes
     *
     * Priority: 10 (higher = higher priority in ternary table; use lower
     *               priority for wildcard / prefix entries).
     */
    try {
        SimpleSwitch::BmMatchParam p_arch, p_cap, p_ctx;
        SimpleSwitch::BmMatchParamTernary t_arch, t_cap, t_ctx;

        /* Pack uint32 as big-endian 4-byte strings (Thrift uses string for bytes) */
        auto pack32 = [](uint32_t v) -> std::string {
            char b[4];
            b[0] = (v >> 24) & 0xFF; b[1] = (v >> 16) & 0xFF;
            b[2] = (v >>  8) & 0xFF; b[3] = (v      ) & 0xFF;
            return std::string(b, 4);
        };
        std::string mask32 = std::string("\xFF\xFF\xFF\xFF", 4);

        t_arch.key   = pack32(model_arch);
        t_arch.mask  = mask32;
        p_arch.__set_ternary(t_arch);

        t_cap.key  = pack32(capability);
        t_cap.mask = mask32;
        p_cap.__set_ternary(t_cap);

        t_ctx.key  = pack32(context_window);
        t_ctx.mask = mask32;
        p_ctx.__set_ternary(t_ctx);

        std::vector<SimpleSwitch::BmMatchParam> match_params = {p_arch, p_cap, p_ctx};

        /* Action data: 16-byte node_id */
        SimpleSwitch::BmActionData action_data;
        action_data.push_back(std::string(reinterpret_cast<const char *>(node_id), 16));

        int32_t entry_handle = g_client->bm_mt_add_entry(
            0 /* cxt_id */,
            "MyIngress.sad_ternary_match",
            match_params,
            "MyIngress.set_resolved_node",
            action_data,
            SimpleSwitch::BmAddEntryOptions()
        );

        (void)entry_handle;
    } catch (const SimpleSwitch::InvalidTableOperation &ex) {
        fprintf(stderr, "[p4rt] ERROR: sad_table_add failed: code=%d\n", ex.code);
        rc = P4RT_ERR_GENERIC;
    } catch (const TException &ex) {
        fprintf(stderr, "[p4rt] ERROR: sad_table_add transport error: %s\n", ex.what());
        rc = P4RT_ERR_CONN;
    }
#else
    /* Stub: log the operation */
    fprintf(stderr,
            "[p4rt] STUB: sad_table_add(model_arch=0x%08x, cap=0x%08x, "
            "ctx_win=0x%08x, node_id=%s)\n",
            model_arch, capability, context_window, node_hex);
#endif /* BMV2_THRIFT_ENABLED */

    pthread_mutex_unlock(&g_p4rt_lock);
    return rc;
}

int p4rt_sad_table_delete(const nexroute_sad_t *sad)
{
    uint32_t model_arch = 0, capability = 0, context_window = 0;
    int rc = P4RT_OK;

    if (!sad) return P4RT_ERR_INVAL;

    if (extract_sad_keys(sad, &model_arch, &capability, &context_window) != 0)
        return P4RT_ERR_INVAL;

    pthread_mutex_lock(&g_p4rt_lock);

    if (!g_p4rt_state.connected) {
        pthread_mutex_unlock(&g_p4rt_lock);
        return P4RT_ERR_CONN;
    }

#ifdef BMV2_THRIFT_ENABLED
    /*
     * To delete an entry we need its handle.  A production implementation
     * would maintain a local (model_arch, cap, ctx) -> handle map.
     *
     * Here we use bm_mt_get_entries to scan the table for a matching entry,
     * then call bm_mt_delete_entry with the retrieved handle.  This is O(n)
     * but acceptable for the control plane.
     */
    try {
        auto pack32 = [](uint32_t v) -> std::string {
            char b[4];
            b[0] = (v >> 24) & 0xFF; b[1] = (v >> 16) & 0xFF;
            b[2] = (v >>  8) & 0xFF; b[3] = (v      ) & 0xFF;
            return std::string(b, 4);
        };

        std::string key_arch = pack32(model_arch);
        std::string key_cap  = pack32(capability);
        std::string key_ctx  = pack32(context_window);

        std::vector<SimpleSwitch::BmMtEntry> entries;
        g_client->bm_mt_get_entries(entries, 0, "MyIngress.sad_ternary_match");

        int32_t handle_to_delete = -1;
        for (const auto &entry : entries) {
            if (entry.match_key.size() == 3 &&
                entry.match_key[0].ternary().key == key_arch &&
                entry.match_key[1].ternary().key == key_cap  &&
                entry.match_key[2].ternary().key == key_ctx) {
                handle_to_delete = entry.entry_handle;
                break;
            }
        }

        if (handle_to_delete < 0) {
            rc = P4RT_ERR_NOT_FOUND;
        } else {
            g_client->bm_mt_delete_entry(0, "MyIngress.sad_ternary_match",
                                         handle_to_delete);
        }
    } catch (const SimpleSwitch::InvalidTableOperation &ex) {
        fprintf(stderr, "[p4rt] ERROR: sad_table_delete failed: code=%d\n", ex.code);
        rc = P4RT_ERR_GENERIC;
    } catch (const TException &ex) {
        fprintf(stderr, "[p4rt] ERROR: sad_table_delete transport error: %s\n", ex.what());
        rc = P4RT_ERR_CONN;
    }
#else
    fprintf(stderr,
            "[p4rt] STUB: sad_table_delete(model_arch=0x%08x, cap=0x%08x, "
            "ctx_win=0x%08x)\n",
            model_arch, capability, context_window);
#endif /* BMV2_THRIFT_ENABLED */

    pthread_mutex_unlock(&g_p4rt_lock);
    return rc;
}

/* --------------------------------------------------------------------------
 * Node ID forwarding table management
 * -------------------------------------------------------------------------- */

int p4rt_node_forward_add(const uint8_t node_id[16], int egress_port)
{
    char node_hex[33];
    int rc = P4RT_OK;

    if (!node_id || egress_port < 0) return P4RT_ERR_INVAL;

    format_node_id(node_id, node_hex);

    pthread_mutex_lock(&g_p4rt_lock);

    if (!g_p4rt_state.connected) {
        pthread_mutex_unlock(&g_p4rt_lock);
        return P4RT_ERR_CONN;
    }

#ifdef BMV2_THRIFT_ENABLED
    /*
     * Install an exact-match entry in the node_id_forward table.
     *
     * Table:    MyIngress.node_id_forward
     * Action:   MyIngress.forward_to_port
     *
     * Key field (exact):
     *   hdr.nexlink.dst_node_id : 16 bytes
     *
     * Action parameter:
     *   port : 9 bits packed into 2 bytes (big-endian)
     */
    try {
        SimpleSwitch::BmMatchParam p_node;
        SimpleSwitch::BmMatchParamExact e_node;
        e_node.key = std::string(reinterpret_cast<const char *>(node_id), 16);
        p_node.__set_exact(e_node);

        std::vector<SimpleSwitch::BmMatchParam> match_params = {p_node};

        /* Pack port as 2-byte big-endian (P4 9-bit field packed into 2 bytes) */
        SimpleSwitch::BmActionData action_data;
        char port_bytes[2];
        port_bytes[0] = (egress_port >> 8) & 0x01;  /* upper 1 bit */
        port_bytes[1] = (egress_port     ) & 0xFF;  /* lower 8 bits */
        action_data.push_back(std::string(port_bytes, 2));

        g_client->bm_mt_add_entry(
            0 /* cxt_id */,
            "MyIngress.node_id_forward",
            match_params,
            "MyIngress.forward_to_port",
            action_data,
            SimpleSwitch::BmAddEntryOptions()
        );
    } catch (const SimpleSwitch::InvalidTableOperation &ex) {
        fprintf(stderr, "[p4rt] ERROR: node_forward_add failed: code=%d\n", ex.code);
        rc = P4RT_ERR_GENERIC;
    } catch (const TException &ex) {
        fprintf(stderr, "[p4rt] ERROR: node_forward_add transport error: %s\n", ex.what());
        rc = P4RT_ERR_CONN;
    }
#else
    fprintf(stderr,
            "[p4rt] STUB: node_forward_add(node_id=%s, egress_port=%d)\n",
            node_hex, egress_port);
#endif /* BMV2_THRIFT_ENABLED */

    pthread_mutex_unlock(&g_p4rt_lock);
    return rc;
}

int p4rt_node_forward_delete(const uint8_t node_id[16])
{
    char node_hex[33];
    int rc = P4RT_OK;

    if (!node_id) return P4RT_ERR_INVAL;

    format_node_id(node_id, node_hex);

    pthread_mutex_lock(&g_p4rt_lock);

    if (!g_p4rt_state.connected) {
        pthread_mutex_unlock(&g_p4rt_lock);
        return P4RT_ERR_CONN;
    }

#ifdef BMV2_THRIFT_ENABLED
    /*
     * Scan node_id_forward table for the entry matching node_id, then delete.
     * Same linear-scan strategy as sad_table_delete; production code should
     * maintain a node_id -> handle map.
     */
    try {
        std::string key_node(reinterpret_cast<const char *>(node_id), 16);

        std::vector<SimpleSwitch::BmMtEntry> entries;
        g_client->bm_mt_get_entries(entries, 0, "MyIngress.node_id_forward");

        int32_t handle_to_delete = -1;
        for (const auto &entry : entries) {
            if (!entry.match_key.empty() &&
                entry.match_key[0].exact().key == key_node) {
                handle_to_delete = entry.entry_handle;
                break;
            }
        }

        if (handle_to_delete < 0) {
            rc = P4RT_ERR_NOT_FOUND;
        } else {
            g_client->bm_mt_delete_entry(0, "MyIngress.node_id_forward",
                                         handle_to_delete);
        }
    } catch (const SimpleSwitch::InvalidTableOperation &ex) {
        fprintf(stderr, "[p4rt] ERROR: node_forward_delete failed: code=%d\n", ex.code);
        rc = P4RT_ERR_GENERIC;
    } catch (const TException &ex) {
        fprintf(stderr, "[p4rt] ERROR: node_forward_delete transport error: %s\n", ex.what());
        rc = P4RT_ERR_CONN;
    }
#else
    fprintf(stderr,
            "[p4rt] STUB: node_forward_delete(node_id=%s)\n", node_hex);
#endif /* BMV2_THRIFT_ENABLED */

    pthread_mutex_unlock(&g_p4rt_lock);
    return rc;
}

/* --------------------------------------------------------------------------
 * Diagnostics
 * -------------------------------------------------------------------------- */

const char *p4rt_strerror(int errcode)
{
    switch (errcode) {
    case P4RT_OK:           return "Success";
    case P4RT_ERR_GENERIC:  return "Generic error";
    case P4RT_ERR_CONN:     return "Connection error (BMv2 not reachable)";
    case P4RT_ERR_NOT_FOUND:return "Entry not found";
    case P4RT_ERR_INVAL:    return "Invalid argument";
    case P4RT_ERR_FULL:     return "Table full";
    default:                return "Unknown error";
    }
}

int p4rt_is_connected(void)
{
    int ret;
    pthread_mutex_lock(&g_p4rt_lock);
    ret = g_p4rt_state.connected;
    pthread_mutex_unlock(&g_p4rt_lock);
    return ret;
}
