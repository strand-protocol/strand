// SPDX-License-Identifier: GPL-2.0
//
// xdp_kern.c — StrandLink eBPF/XDP kernel program
//
// This program runs inside the Linux kernel at the XDP (eXpress Data Path)
// hook — the earliest possible point in the packet receive path, before any
// kernel networking stack processing.
//
// Packet flow:
//   NIC RX → XDP hook → strandlink_xdp_prog()
//               ├── StrandLink frame? → bpf_redirect_map → AF_XDP socket
//               └── Other frame?   → XDP_PASS (hand to kernel stack)
//
// The AF_XDP socket lives in the strandlink userspace backend (xdp.zig).
// The BPF map `xsks_map` is populated by userspace: key = RX queue index,
// value = AF_XDP socket fd.  When a StrandLink frame arrives on queue N, the
// program redirects it directly into the userspace socket, bypassing the
// full kernel TCP/IP stack for near-zero-copy, sub-microsecond dispatch.
//
// Classification strategy:
//   1. Walk the Ethernet → IPv4/IPv6 → UDP header chain.
//   2. Check UDP destination port == STRANDLINK_UDP_PORT (6477).
//      This covers the StrandLink overlay encapsulation (overlay.zig).
//   3. Fall through to XDP_PASS for any non-matching frame so normal
//      traffic (SSH, etc.) continues to work.
//
// Build (requires Linux kernel headers + libbpf):
//   clang -O2 -target bpf -D__TARGET_ARCH_x86 \
//         -I/usr/include/x86_64-linux-gnu \
//         -c xdp_kern.c -o xdp_kern.o
//
// The resulting xdp_kern.o is loaded by the Zig AF_XDP backend at runtime.

#include <linux/bpf.h>
#include <linux/if_ether.h>
#include <linux/ip.h>
#include <linux/ipv6.h>
#include <linux/udp.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_endian.h>

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

/// UDP destination port used by the StrandLink overlay transport.
/// Must match overlay.zig OVERLAY_PORT = 6477.
#define STRANDLINK_UDP_PORT 6477

/// Maximum number of AF_XDP sockets (one per NIC RX queue).
/// 64 queues is generous for real-world NICs; increase if needed.
#define XSKMAP_MAX_ENTRIES 64

// ---------------------------------------------------------------------------
// BPF maps
// ---------------------------------------------------------------------------

/// XSKMAP: maps RX queue index → AF_XDP socket fd.
///
/// Populated by the userspace backend (xdp.zig) before attaching the program.
/// Key:   __u32  rx_queue_index
/// Value: __u32  AF_XDP socket fd (as stored by bpf_map_update_elem)
struct {
    __uint(type, BPF_MAP_TYPE_XSKMAP);
    __uint(key_size, sizeof(__u32));
    __uint(value_size, sizeof(__u32));
    __uint(max_entries, XSKMAP_MAX_ENTRIES);
} xsks_map SEC(".maps");

/// Per-CPU statistics counter for dropped frames (congestion / map full).
struct {
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __uint(key_size, sizeof(__u32));
    __uint(value_size, sizeof(__u64));
    __uint(max_entries, 1);
} xdp_stats_map SEC(".maps");

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/// Increment the per-CPU drop counter (key 0).
static __always_inline void count_drop(void)
{
    __u32 key = 0;
    __u64 *val = bpf_map_lookup_elem(&xdp_stats_map, &key);
    if (val)
        __sync_fetch_and_add(val, 1);
}

// ---------------------------------------------------------------------------
// Main XDP program
// ---------------------------------------------------------------------------

SEC("xdp")
int strandlink_xdp_prog(struct xdp_md *ctx)
{
    // data and data_end are pointers into the packet DMA buffer.
    // All pointer arithmetic must stay within [data, data_end) or the
    // BPF verifier will reject the program.
    void *data     = (void *)(long)ctx->data;
    void *data_end = (void *)(long)ctx->data_end;

    // -----------------------------------------------------------------------
    // Layer 2: Ethernet header
    // -----------------------------------------------------------------------
    struct ethhdr *eth = data;
    if ((void *)(eth + 1) > data_end)
        return XDP_PASS;

    __u16 eth_proto = bpf_ntohs(eth->h_proto);

    // Handle 802.1Q VLAN tag: skip the 4-byte tag to reach the inner EtherType.
    __u16 inner_proto = eth_proto;
    void *l3_start = (void *)(eth + 1);

    if (eth_proto == ETH_P_8021Q) {
        // 4-byte 802.1Q tag: TCI(2) + inner EtherType(2)
        if (l3_start + 4 > data_end)
            return XDP_PASS;
        inner_proto = bpf_ntohs(*(__u16 *)(l3_start + 2));
        l3_start += 4;
    }

    // -----------------------------------------------------------------------
    // Layer 3: IP header
    // -----------------------------------------------------------------------
    __u8 ip_proto;
    void *l4_start;

    if (inner_proto == ETH_P_IP) {
        // IPv4
        struct iphdr *ip = l3_start;
        if ((void *)(ip + 1) > data_end)
            return XDP_PASS;
        // IHL is in 32-bit words; multiply by 4 to get bytes.
        __u32 ip_hdr_len = (ip->ihl & 0x0F) * 4;
        if (ip_hdr_len < sizeof(struct iphdr))
            return XDP_PASS;
        if (l3_start + ip_hdr_len > data_end)
            return XDP_PASS;
        ip_proto = ip->protocol;
        l4_start = l3_start + ip_hdr_len;
    } else if (inner_proto == ETH_P_IPV6) {
        // IPv6: fixed 40-byte header (we do not chase extension headers here;
        // StrandLink overlay uses UDP directly after the fixed IPv6 header).
        struct ipv6hdr *ip6 = l3_start;
        if ((void *)(ip6 + 1) > data_end)
            return XDP_PASS;
        ip_proto = ip6->nexthdr;
        l4_start = (void *)(ip6 + 1);
    } else {
        // Not IPv4 or IPv6 — pass to the kernel stack.
        return XDP_PASS;
    }

    // -----------------------------------------------------------------------
    // Layer 4: UDP header
    // -----------------------------------------------------------------------
    if (ip_proto != IPPROTO_UDP)
        return XDP_PASS;

    struct udphdr *udp = l4_start;
    if ((void *)(udp + 1) > data_end)
        return XDP_PASS;

    // Check destination port against the StrandLink overlay port.
    if (bpf_ntohs(udp->dest) != STRANDLINK_UDP_PORT)
        return XDP_PASS;

    // -----------------------------------------------------------------------
    // StrandLink overlay detected — redirect to AF_XDP socket
    //
    // bpf_redirect_map() atomically looks up ctx->rx_queue_index in xsks_map
    // and steers the frame to the matching AF_XDP socket.  If no socket is
    // registered for this queue (map lookup miss), the fallback action
    // XDP_PASS is used so that the frame reaches the normal kernel UDP stack
    // instead of being silently dropped.
    // -----------------------------------------------------------------------
    int ret = bpf_redirect_map(&xsks_map, ctx->rx_queue_index, XDP_PASS);

    if (ret == XDP_DROP) {
        // Map lookup succeeded but the socket's fill ring was full —
        // the frame was dropped.  Count it for diagnostics.
        count_drop();
    }

    return ret;
}

char _license[] SEC("license") = "GPL";
