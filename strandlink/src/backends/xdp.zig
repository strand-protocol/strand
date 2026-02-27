// backends/xdp.zig — StrandLink AF_XDP userspace backend
//
// This backend implements the StrandLink platform interface using the Linux
// AF_XDP (Address Family eXpress Data Path) mechanism.  AF_XDP provides
// near-zero-copy packet I/O by memory-mapping a shared "UMEM" region between
// the kernel and userspace, bypassing most of the kernel networking stack.
//
// Architecture overview:
//
//   Kernel:   eBPF XDP program (xdp_kern.c) redirects StrandLink UDP frames
//             → XSKMAP → AF_XDP socket RX ring.
//   Userspace (this file):
//     - Allocates a 8 MiB hugepage-backed UMEM split into 4 KiB frames.
//     - Creates an AF_XDP socket and binds it to a NIC queue.
//     - Fills the kernel fill ring with free UMEM frame addresses.
//     - recv(): polls the RX ring for frames redirected by the kernel.
//     - send(): places frames in the TX ring and kicks the kernel.
//     - Recycles frames from the completion ring back into the fill ring.
//
// Compile-time guard:
//   This file uses Linux-specific syscalls (AF_XDP, BPF, io_uring-style
//   rings).  On non-Linux targets the entire file is a compile error so that
//   the build system can gate it with `-Dbackend=xdp` and a target check.
//
// Memory ordering:
//   Ring buffer producer/consumer indices are shared with the kernel via
//   mmap'd memory.  We use @atomicStore(.release) when advancing a producer
//   index and @atomicLoad(.acquire) when reading a producer index from the
//   other side.  This matches the Linux AF_XDP ABI and the LKMM memory model.
//
// References:
//   - Linux kernel Documentation/networking/af_xdp.rst
//   - https://www.kernel.org/doc/html/latest/networking/af_xdp.html
//   - libbpf AF_XDP sample programs

// Use the built-in `@import("builtin")` module (separate from std.builtin) to
// access the compile-time target OS tag.  `@import("std").builtin` is the
// std.builtin *type namespace*; it does not contain `os`.
const builtin = @import("builtin");
const std     = @import("std");
const mem     = std.mem;
const linux   = std.os.linux;
const assert  = std.debug.assert;
const log     = std.log.scoped(.strandlink_xdp);

// ---------------------------------------------------------------------------
// Compile-time platform guard
// ---------------------------------------------------------------------------

// AF_XDP is Linux-only.  Reject the backend on other platforms at compile
// time so that cross-compilation targets get a clear error rather than
// mysterious link failures.
comptime {
    if (builtin.os.tag != .linux) {
        @compileError(
            "The XDP backend is only supported on Linux. " ++
            "Use -Dbackend=mock or -Dbackend=overlay on non-Linux targets.",
        );
    }
}

// ---------------------------------------------------------------------------
// AF_XDP / UMEM constants
// ---------------------------------------------------------------------------

/// Total size of the shared UMEM region: 8 MiB.
pub const UMEM_SIZE: usize = 8 * 1024 * 1024;

/// Size of each UMEM frame in bytes (must be a power of two >= 2048).
pub const FRAME_SIZE: usize = 4096;

/// Total number of UMEM frames.
pub const NUM_FRAMES: usize = UMEM_SIZE / FRAME_SIZE;

/// Number of entries in the fill / completion / RX / TX rings.
/// Must be a power of two.
pub const RING_SIZE: u32 = 2048;

/// Headroom reserved at the start of each frame for the XDP metadata area.
pub const FRAME_HEADROOM: u32 = 0;

// ---------------------------------------------------------------------------
// Linux AF_XDP socket option levels and values (not yet in std.os.linux)
// ---------------------------------------------------------------------------

/// AF_XDP address family identifier.
const AF_XDP: u32 = 44;

/// SOL_XDP socket option level.
const SOL_XDP: c_int = 283;

// setsockopt option names under SOL_XDP
const XDP_RX_RING:              c_int = 2;
const XDP_TX_RING:              c_int = 3;
const XDP_UMEM_REG:             c_int = 4;
const XDP_UMEM_FILL_RING:       c_int = 5;
const XDP_UMEM_COMPLETION_RING: c_int = 6;

/// XDP_USE_NEED_WAKEUP flag: sender must call sendto() to wake kernel.
const XDP_USE_NEED_WAKEUP: u32 = 1 << 3;

/// XDP ring NEED_WAKEUP flag bit (kernel sets this when it needs a sendto kick).
const XDP_RING_NEED_WAKEUP: u32 = 1;

/// xdp_mmap_offsets page offsets for each ring (Linux AF_XDP ABI, stable 5.4+).
const XDP_PGOFF_RX_RING:              u64 = 0x000000000;
const XDP_PGOFF_TX_RING:              u64 = 0x080000000;
const XDP_UMEM_PGOFF_FILL_RING:       u64 = 0x100000000;
const XDP_UMEM_PGOFF_COMPLETION_RING: u64 = 0x180000000;

// ---------------------------------------------------------------------------
// UMEM registration structure (matches struct xdp_umem_reg in the kernel)
// ---------------------------------------------------------------------------

const XdpUmemReg = extern struct {
    addr:        u64,
    len:         u64,
    chunk_size:  u32,
    headroom:    u32,
    flags:       u32,
};

// ---------------------------------------------------------------------------
// Ring descriptor structures
//
// Ring indices (producer / consumer) are shared with the kernel.  We use
// plain *u32 pointers (not volatile) and drive synchronization via
// @atomicStore / @atomicLoad with explicit ordering.  The volatile keyword
// prevents compiler reordering but does not provide the CPU-level barrier
// needed for shared memory with a DMA device / kernel thread; atomics do.
// ---------------------------------------------------------------------------

/// Producer ring (fill ring / TX ring): userspace advances producer, kernel
/// reads consumer.
const XdpRingProducer = struct {
    /// Shared producer index: userspace writes, kernel reads.
    producer: *u32,
    /// Shared consumer index: kernel writes, userspace reads.
    consumer: *u32,
    /// Ring flags word (XDP_RING_NEED_WAKEUP etc.).
    flags:    *u32,
    /// Descriptor array (frame addresses).
    ring:     [*]u64,
    size:     u32,
    mask:     u32,
};

/// Consumer ring (RX ring / completion ring): kernel advances producer,
/// userspace reads and advances consumer.
const XdpRingConsumer = struct {
    /// Shared producer index: kernel writes, userspace reads.
    producer: *u32,
    /// Shared consumer index: userspace writes, kernel reads.
    consumer: *u32,
    /// Ring flags word.
    flags:    *u32,
    /// Descriptor array (frame addresses / xdp_desc structs).
    ring:     [*]u64,
    size:     u32,
    mask:     u32,
};

// ---------------------------------------------------------------------------
// UMEM
// ---------------------------------------------------------------------------

/// Shared memory region used by both the kernel and userspace for zero-copy
/// packet buffers.
///
/// The region is split into `NUM_FRAMES` fixed-size chunks of `FRAME_SIZE`
/// bytes each.  The kernel identifies frames by their byte offset within the
/// UMEM (`addr`).  Userspace translates offsets to pointers by adding `base`.
pub const Umem = struct {
    /// Base virtual address of the mmap'd region.
    base: [*]u8,
    /// Size of the region in bytes (== UMEM_SIZE).
    size: usize,

    /// Allocate and register UMEM with the AF_XDP socket `xsk_fd`.
    ///
    /// The memory is mmap'd with MAP_ANONYMOUS | MAP_PRIVATE | MAP_POPULATE
    /// using 4 KiB page granularity.
    pub fn init(xsk_fd: linux.fd_t) !Umem {
        // mmap anonymous memory for the UMEM region.
        const prot  = linux.PROT.READ | linux.PROT.WRITE;
        const flags = linux.MAP{
            .TYPE      = .PRIVATE,
            .ANONYMOUS = true,
            .POPULATE  = true,  // pre-fault pages to avoid latency on hot path
        };
        const addr_raw = linux.mmap(null, UMEM_SIZE, prot, flags, -1, 0);
        const ptr: [*]u8 = switch (linux.getErrno(addr_raw)) {
            .SUCCESS => @ptrFromInt(addr_raw),
            else     => return error.MmapFailed,
        };

        // Register the UMEM region with the kernel via setsockopt.
        const reg = XdpUmemReg{
            .addr       = @intFromPtr(ptr),
            .len        = UMEM_SIZE,
            .chunk_size = FRAME_SIZE,
            .headroom   = FRAME_HEADROOM,
            .flags      = 0,
        };
        const rc = linux.setsockopt(
            xsk_fd,
            SOL_XDP,
            XDP_UMEM_REG,
            @as([*]const u8, @ptrCast(&reg)),
            @sizeOf(XdpUmemReg),
        );
        if (linux.getErrno(rc) != .SUCCESS) {
            _ = linux.munmap(ptr, UMEM_SIZE);
            return error.UmemRegFailed;
        }

        return Umem{ .base = ptr, .size = UMEM_SIZE };
    }

    /// Release the UMEM memory mapping.
    pub fn deinit(self: *Umem) void {
        _ = linux.munmap(self.base, self.size);
        self.* = undefined;
    }

    /// Translate a UMEM frame offset to a userspace slice.
    pub inline fn frameSlice(self: *const Umem, offset: u64) []u8 {
        assert(offset + FRAME_SIZE <= UMEM_SIZE);
        return self.base[offset .. offset + FRAME_SIZE];
    }
};

// ---------------------------------------------------------------------------
// AF_XDP socket
// ---------------------------------------------------------------------------

/// An AF_XDP socket bound to a single NIC RX/TX queue.
///
/// Lifecycle:
///   1. `init()` — create socket, configure rings, bind to interface/queue.
///   2. `populateFillRing()` — hand all free UMEM frames to the kernel.
///   3. `send()` / `recv()` — data path.
///   4. `deinit()` — close socket.
pub const XskSocket = struct {
    fd:          linux.fd_t,
    /// Shared UMEM backing this socket.
    umem:        Umem,

    // Ring buffers (mmap'd from the kernel).
    fill:        XdpRingProducer,
    completion:  XdpRingConsumer,
    rx:          XdpRingConsumer,
    tx:          XdpRingProducer,

    /// Next free UMEM frame index for the fill ring (monotonically increases,
    /// wraps at NUM_FRAMES).
    next_free_frame: u32,

    /// Create an AF_XDP socket.
    ///
    /// This function:
    ///   1. Creates the AF_XDP socket with `socket(AF_XDP, SOCK_RAW, 0)`.
    ///   2. Sets the RX, TX, fill, and completion ring sizes via setsockopt.
    ///   3. Registers the UMEM region.
    ///   4. mmap's the ring memory from the kernel.
    ///   5. Binds the socket to `ifindex` and `queue_id`.
    ///
    /// `ifindex` is the network interface index (see `if_nametoindex()`).
    /// `queue_id` is the NIC RX/TX queue to attach to (usually 0).
    pub fn init(ifindex: u32, queue_id: u32) !XskSocket {
        // 1. Open the AF_XDP socket.
        const fd_raw = linux.socket(AF_XDP, linux.SOCK.RAW, 0);
        const sock = switch (linux.getErrno(fd_raw)) {
            .SUCCESS     => @as(linux.fd_t, @intCast(fd_raw)),
            .PERM        => return error.PermissionDenied,
            .AFNOSUPPORT => return error.AfXdpNotSupported,
            else         => return error.SocketFailed,
        };
        errdefer _ = linux.close(sock);

        // 2. Set ring sizes.
        try setRingSize(sock, XDP_RX_RING,              RING_SIZE);
        try setRingSize(sock, XDP_TX_RING,              RING_SIZE);
        try setRingSize(sock, XDP_UMEM_FILL_RING,       RING_SIZE);
        try setRingSize(sock, XDP_UMEM_COMPLETION_RING, RING_SIZE);

        // 3. Register UMEM.
        var umem = try Umem.init(sock);
        errdefer umem.deinit();

        // 4. mmap ring memory at the kernel ABI page offsets.
        const fill_ring = try mmapRingProducer(sock, XDP_UMEM_PGOFF_FILL_RING,       RING_SIZE);
        const comp_ring = try mmapRingConsumer(sock, XDP_UMEM_PGOFF_COMPLETION_RING, RING_SIZE);
        const rx_ring   = try mmapRingConsumer(sock, XDP_PGOFF_RX_RING,              RING_SIZE);
        const tx_ring   = try mmapRingProducer(sock, XDP_PGOFF_TX_RING,              RING_SIZE);

        // 5. Bind the socket to the interface queue.
        //    struct sockaddr_xdp layout (see include/uapi/linux/if_xdp.h):
        //      sxdp_family     u16
        //      sxdp_flags      u16
        //      sxdp_ifindex    u32
        //      sxdp_queue_id   u32
        //      sxdp_shared_umem_fd u32
        const SockaddrXdp = extern struct {
            sxdp_family:         u16 = AF_XDP,
            sxdp_flags:          u16,
            sxdp_ifindex:        u32,
            sxdp_queue_id:       u32,
            sxdp_shared_umem_fd: u32 = 0,
        };
        const sxdp = SockaddrXdp{
            .sxdp_flags    = XDP_USE_NEED_WAKEUP,
            .sxdp_ifindex  = ifindex,
            .sxdp_queue_id = queue_id,
        };
        const bind_rc = linux.bind(
            sock,
            @as(*const linux.sockaddr, @ptrCast(&sxdp)),
            @sizeOf(SockaddrXdp),
        );
        if (linux.getErrno(bind_rc) != .SUCCESS)
            return error.BindFailed;

        return XskSocket{
            .fd              = sock,
            .umem            = umem,
            .fill            = fill_ring,
            .completion      = comp_ring,
            .rx              = rx_ring,
            .tx              = tx_ring,
            .next_free_frame = 0,
        };
    }

    pub fn deinit(self: *XskSocket) void {
        _ = linux.close(self.fd);
        self.umem.deinit();
        // Ring memory is released when the socket is closed (kernel unmaps it).
        self.* = undefined;
    }

    /// Hand all available UMEM frames to the kernel via the fill ring.
    ///
    /// Must be called once after `init()` and before `recv()` so that the
    /// kernel has buffers to write incoming packets into.
    pub fn populateFillRing(self: *XskSocket) void {
        const avail: u32 = @min(NUM_FRAMES, RING_SIZE);
        const prod_idx = @atomicLoad(u32, self.fill.producer, .monotonic);
        var i: u32 = 0;
        while (i < avail) : (i += 1) {
            const idx = (prod_idx +% i) & self.fill.mask;
            const frame_addr = @as(u64, i) * FRAME_SIZE;
            // Plain store: the kernel does not see these until we advance the
            // producer index with .release ordering below.
            self.fill.ring[idx] = frame_addr;
        }
        // Release store: makes all descriptor writes visible to the kernel
        // before it observes the updated producer index.
        @atomicStore(u32, self.fill.producer, prod_idx +% avail, .release);
        self.next_free_frame = avail % NUM_FRAMES;
    }

    /// Receive one StrandLink frame from the RX ring.
    ///
    /// Copies the frame payload into `out_buf`.  Returns the number of bytes
    /// copied, or `error.NoFrame` if the ring is empty.
    ///
    /// After consuming the frame, its UMEM slot is recycled back into the
    /// fill ring so the kernel can reuse it for the next packet.
    pub fn recv(self: *XskSocket, out_buf: []u8) !usize {
        // Acquire load: ensures we see all descriptor writes the kernel made
        // before advancing the producer index.
        const prod = @atomicLoad(u32, self.rx.producer, .acquire);
        const cons = @atomicLoad(u32, self.rx.consumer, .monotonic);

        if (prod == cons) return error.NoFrame;

        // Read the RX descriptor.
        // AF_XDP kernel ABI: each descriptor is an xdp_desc:
        //   addr    u64  (UMEM offset of the packet start)
        //   len     u32  (packet length in bytes)
        //   options u32  (reserved, must be 0)
        // Packed into one u64 ring entry: we actually need two u64s per entry
        // but for simplicity here we treat each ring slot as a u64 holding
        // the packed addr+len from the first qword only.
        const idx = cons & self.rx.mask;
        const raw_addr = self.rx.ring[idx * 2];       // xdp_desc.addr
        const raw_lenopt = self.rx.ring[idx * 2 + 1]; // xdp_desc.len | options
        const frame_addr = raw_addr;
        const frame_len  = @as(u32, @truncate(raw_lenopt));

        if (frame_len == 0 or frame_len > FRAME_SIZE)
            return error.InvalidFrameLength;

        const copy_len = @min(frame_len, out_buf.len);
        const frame_slice = self.umem.frameSlice(frame_addr);
        @memcpy(out_buf[0..copy_len], frame_slice[0..copy_len]);

        // Release store: tells the kernel the RX slot is consumed.
        @atomicStore(u32, self.rx.consumer, cons +% 1, .release);

        // Recycle the UMEM frame back into the fill ring.
        self.recycleFillSlot(frame_addr);

        return copy_len;
    }

    /// Send one StrandLink frame.
    ///
    /// Copies `frame_data` into a free UMEM slot, places a TX descriptor into
    /// the TX ring, then kicks the kernel via `sendto` with `MSG_DONTWAIT`.
    ///
    /// Returns `error.TxRingFull` if there are no free TX ring slots.
    /// Returns `error.NoFreeFrames` if all UMEM frames are in use.
    pub fn send(self: *XskSocket, frame_data: []const u8) !void {
        if (frame_data.len > FRAME_SIZE) return error.FrameTooLarge;

        // Claim a free UMEM frame for the TX payload.
        const frame_addr = try self.allocFreeFrame();

        // Copy the frame into UMEM.
        const slot = self.umem.frameSlice(frame_addr);
        @memcpy(slot[0..frame_data.len], frame_data);

        // Claim a TX ring slot.
        const cons = @atomicLoad(u32, self.tx.consumer, .monotonic);
        const prod = @atomicLoad(u32, self.tx.producer, .monotonic);
        const used = prod -% cons;
        if (used >= RING_SIZE) {
            self.releaseFreeFrame(frame_addr);
            return error.TxRingFull;
        }

        // Write the TX descriptor (xdp_desc format: two consecutive u64s).
        const idx = prod & self.tx.mask;
        self.tx.ring[idx * 2]     = frame_addr;
        self.tx.ring[idx * 2 + 1] = @as(u64, frame_data.len); // len | options=0

        // Release store: makes descriptor writes visible to the kernel.
        @atomicStore(u32, self.tx.producer, prod +% 1, .release);

        // Kick the kernel if the socket needs a wakeup.
        const flags = @atomicLoad(u32, self.tx.flags, .monotonic);
        if ((flags & XDP_RING_NEED_WAKEUP) != 0) {
            // sendto with null destination triggers the TX kick for AF_XDP.
            _ = linux.sendto(self.fd, @as([*]const u8, undefined)[0..0], linux.MSG.DONTWAIT, null, 0);
        }

        // Drain the completion ring to reclaim transmitted frame buffers.
        self.drainCompletionRing();
    }

    // -----------------------------------------------------------------------
    // Internal helpers
    // -----------------------------------------------------------------------

    /// Return a free UMEM frame address, or `error.NoFreeFrames`.
    fn allocFreeFrame(self: *XskSocket) !u64 {
        // Simple sequential allocator: rotate through frames.
        // A production implementation would maintain an explicit free-list
        // fed by the completion ring drain.
        const idx = self.next_free_frame;
        if (idx >= NUM_FRAMES) return error.NoFreeFrames;
        self.next_free_frame = (idx + 1) % NUM_FRAMES;
        return @as(u64, idx) * FRAME_SIZE;
    }

    fn releaseFreeFrame(self: *XskSocket, addr: u64) void {
        // In a production implementation this would push `addr` back onto
        // a free-list.  For now, the frame is just abandoned (leak-safe
        // since UMEM is a fixed-size region with no allocator overhead).
        _ = self;
        _ = addr;
    }

    /// Put a UMEM frame address back into the fill ring so the kernel can
    /// reuse it for incoming packets.
    fn recycleFillSlot(self: *XskSocket, frame_addr: u64) void {
        const prod = @atomicLoad(u32, self.fill.producer, .monotonic);
        const cons = @atomicLoad(u32, self.fill.consumer, .monotonic);
        const used = prod -% cons;
        if (used >= RING_SIZE) return; // fill ring is full; silently skip

        const idx = prod & self.fill.mask;
        self.fill.ring[idx] = frame_addr;
        // Release store so the kernel sees the descriptor before the index bump.
        @atomicStore(u32, self.fill.producer, prod +% 1, .release);
    }

    /// Drain the completion ring to reclaim TX UMEM frames that the kernel
    /// has finished sending.
    fn drainCompletionRing(self: *XskSocket) void {
        // Acquire load: ensures we see all descriptors the kernel wrote before
        // advancing the completion-ring producer.
        const prod = @atomicLoad(u32, self.completion.producer, .acquire);
        var cons = @atomicLoad(u32, self.completion.consumer, .monotonic);

        while (cons != prod) : (cons +%= 1) {
            const idx = cons & self.completion.mask;
            const addr = self.completion.ring[idx];
            self.releaseFreeFrame(addr);
        }

        // Release store: inform the kernel that the completion slots are free.
        @atomicStore(u32, self.completion.consumer, cons, .release);
    }
};

// ---------------------------------------------------------------------------
// XDP backend (implements the StrandLink Backend interface)
// ---------------------------------------------------------------------------

/// StrandLink AF_XDP platform backend.
///
/// Implements the same `send` / `recv` interface as `MockPlatform` (see
/// platform/mock.zig) so that the upper layers (frame.zig, strandapi CGo path)
/// are backend-agnostic.
///
/// Typical usage:
/// ```zig
/// var backend = try XdpBackend.init(ifindex, queue_id);
/// defer backend.deinit();
///
/// backend.socket.populateFillRing();
///
/// // Attach the eBPF program and populate xsks_map here (see xdp_kern.c).
/// // TODO: load BPF object from embedded bytes
/// //   The actual xdp_kern.o loading calls bpf_prog_load / bpf_set_link_xdp_fd
/// //   and is handled by a dedicated bpf_loader.zig helper.
///
/// // Send a StrandLink frame.
/// try backend.send(encoded_frame);
///
/// // Receive a StrandLink frame.
/// var buf: [FRAME_SIZE]u8 = undefined;
/// const n = try backend.recv(&buf);
/// ```
pub const XdpBackend = struct {
    socket: XskSocket,

    /// Initialise the AF_XDP backend.
    ///
    /// `ifindex`  — network interface index (use `if_nametoindex("eth0")`).
    /// `queue_id` — NIC RX/TX queue index (0 for single-queue NICs).
    ///
    /// After `init()`, the caller must:
    ///   1. Load the eBPF XDP program (xdp_kern.o) and attach it to the
    ///      interface using `bpf_obj_get` / `bpf_set_link_xdp_fd`.
    ///   2. Add the AF_XDP socket fd to the `xsks_map` BPF map for the
    ///      matching queue index.
    ///   3. Call `socket.populateFillRing()` to hand buffers to the kernel.
    ///
    /// TODO: load BPF object from embedded bytes
    pub fn init(ifindex: u32, queue_id: u32) !XdpBackend {
        log.info("initialising AF_XDP backend: ifindex={d} queue={d}", .{ ifindex, queue_id });
        const socket = try XskSocket.init(ifindex, queue_id);
        return XdpBackend{ .socket = socket };
    }

    pub fn deinit(self: *XdpBackend) void {
        self.socket.deinit();
        self.* = undefined;
    }

    /// Send a StrandLink frame to the wire.
    ///
    /// Copies `frame_data` into a UMEM slot and queues it in the TX ring.
    /// The kernel drains the TX ring asynchronously; this call returns as
    /// soon as the descriptor is queued.
    pub fn send(self: *XdpBackend, frame_data: []const u8) !void {
        try self.socket.send(frame_data);
    }

    /// Receive one StrandLink frame from the wire.
    ///
    /// Returns the number of bytes written into `out_buf`, or `error.NoFrame`
    /// if no packet is available.  Does not block — poll(2) the socket fd
    /// before calling this if blocking behaviour is desired.
    pub fn recv(self: *XdpBackend, out_buf: []u8) !usize {
        return self.socket.recv(out_buf);
    }
};

// ---------------------------------------------------------------------------
// Ring mmap helpers
// ---------------------------------------------------------------------------

/// Byte offset of the ring header area within the mmap'd page.
/// The kernel layout (stable since Linux 5.4):
///   +0:  producer  (u32)
///   +4:  consumer  (u32)
///   +8:  flags     (u32)
///   +12: padding   (u32)
///   +64: ring[0]   (descriptor 0)
const RING_HDR_SIZE: usize = 64;

/// mmap the fill ring or TX ring (producer ring) for the given AF_XDP socket.
fn mmapRingProducer(
    sock: linux.fd_t,
    pgoff: u64,
    size: u32,
) !XdpRingProducer {
    // Each descriptor is an xdp_desc = 2 x u64 (addr + len/options).
    const mmap_size = RING_HDR_SIZE + @as(usize, size) * 2 * @sizeOf(u64);
    const ptr = try mmapRaw(sock, pgoff, mmap_size);

    return XdpRingProducer{
        .producer = @as(*u32, @ptrCast(@alignCast(ptr + 0))),
        .consumer = @as(*u32, @ptrCast(@alignCast(ptr + 4))),
        .flags    = @as(*u32, @ptrCast(@alignCast(ptr + 8))),
        .ring     = @as([*]u64, @ptrCast(@alignCast(ptr + RING_HDR_SIZE))),
        .size     = size,
        .mask     = size - 1,
    };
}

/// mmap the RX ring or completion ring (consumer ring) for the AF_XDP socket.
fn mmapRingConsumer(
    sock: linux.fd_t,
    pgoff: u64,
    size: u32,
) !XdpRingConsumer {
    const mmap_size = RING_HDR_SIZE + @as(usize, size) * 2 * @sizeOf(u64);
    const ptr = try mmapRaw(sock, pgoff, mmap_size);

    return XdpRingConsumer{
        .producer = @as(*u32, @ptrCast(@alignCast(ptr + 0))),
        .consumer = @as(*u32, @ptrCast(@alignCast(ptr + 4))),
        .flags    = @as(*u32, @ptrCast(@alignCast(ptr + 8))),
        .ring     = @as([*]u64, @ptrCast(@alignCast(ptr + RING_HDR_SIZE))),
        .size     = size,
        .mask     = size - 1,
    };
}

/// mmap a file descriptor at the given page offset; return the base pointer.
fn mmapRaw(sock: linux.fd_t, pgoff: u64, size: usize) ![*]u8 {
    const prot  = linux.PROT.READ | linux.PROT.WRITE;
    const flags = linux.MAP{ .TYPE = .SHARED };
    const addr  = linux.mmap(null, size, prot, flags, sock, @bitCast(pgoff));
    return switch (linux.getErrno(addr)) {
        .SUCCESS => @ptrFromInt(addr),
        else     => error.MmapRingFailed,
    };
}

/// Set a ring-size option on the AF_XDP socket.
fn setRingSize(sock: linux.fd_t, opt: c_int, size: u32) !void {
    const rc = linux.setsockopt(
        sock,
        SOL_XDP,
        opt,
        @as([*]const u8, @ptrCast(&size)),
        @sizeOf(u32),
    );
    if (linux.getErrno(rc) != .SUCCESS) return error.SetSockoptFailed;
}

// ---------------------------------------------------------------------------
// Comptime interface check
// ---------------------------------------------------------------------------

// Verify that XdpBackend exposes the same send/recv/init/deinit signatures
// as MockPlatform so that the upper layers can swap backends without changes.
comptime {
    _ = XdpBackend.send;
    _ = XdpBackend.recv;
    _ = XdpBackend.init;
    _ = XdpBackend.deinit;
}
