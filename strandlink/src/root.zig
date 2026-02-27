// root.zig — NexLink public API root module
//
// Re-exports all public types and functions for use by dependent modules
// and the C FFI layer.

pub const header = @import("header.zig");
pub const frame = @import("frame.zig");
pub const crc = @import("crc.zig");
pub const options = @import("options.zig");
pub const ring_buffer = @import("ring_buffer.zig");
pub const memory_pool = @import("memory_pool.zig");
pub const overlay = @import("overlay.zig");
pub const mock = @import("platform/mock.zig");

// ── Convenience re-exports (top-level access) ──

pub const FrameHeader = header.FrameHeader;
pub const FrameFlags = header.FrameFlags;
pub const FrameType = header.FrameType;
pub const QosClass = header.QosClass;
pub const TensorDtype = header.TensorDtype;
pub const NodeId = header.NodeId;

pub const Frame = frame.Frame;
pub const encode = frame.encode;
pub const decode = frame.decode;

pub const RingBuffer = ring_buffer.RingBuffer;
pub const MemoryPool = memory_pool.MemoryPool;
pub const MockPlatform = mock.MockPlatform;

pub const OverlayHeader = overlay.OverlayHeader;
pub const encapsulate = overlay.encapsulate;
pub const decapsulate = overlay.decapsulate;

pub const OptionIterator = options.OptionIterator;
pub const OptionBuilder = options.OptionBuilder;
pub const OptionType = options.OptionType;

pub const HEADER_SIZE = header.HEADER_SIZE;
pub const MAX_FRAME_SIZE = header.MAX_FRAME_SIZE;
pub const MIN_FRAME_SIZE = header.MIN_FRAME_SIZE;
pub const MAX_OPTIONS_SIZE = header.MAX_OPTIONS_SIZE;
pub const NEXLINK_VERSION = header.NEXLINK_VERSION;

// ── C FFI exports ──

const std = @import("std");

/// Encode a NexLink frame. Returns 0 on success, negative on error.
export fn nexlink_frame_encode(
    hdr_buf: ?[*]const u8,
    options_ptr: ?[*]const u8,
    options_len: u16,
    payload_ptr: ?[*]const u8,
    payload_len: u32,
    out_buf: ?[*]u8,
    out_buf_len: u32,
    out_frame_len: ?*u32,
) callconv(.c) c_int {
    // Null-guard all required pointer parameters
    const hdr_raw = hdr_buf orelse return -1;
    const out_raw = out_buf orelse return -1;
    const out_len_ptr = out_frame_len orelse return -1;

    // Deserialize header from raw bytes
    const hdr_slice: *const [header.HEADER_SIZE]u8 = @ptrCast(hdr_raw);
    const hdr_result = FrameHeader.deserialize(hdr_slice);
    const hdr = hdr_result catch return -1;

    const opts: []const u8 = if (options_ptr) |p|
        p[0..options_len]
    else
        &.{};

    const pl: []const u8 = if (payload_ptr) |p|
        p[0..payload_len]
    else
        &.{};

    const out_slice = out_raw[0..out_buf_len];

    const written = encode(&hdr, opts, pl, out_slice) catch return -2;
    out_len_ptr.* = @intCast(written);
    return 0;
}

/// Decode a NexLink frame. Returns 0 on success, negative on error.
export fn nexlink_frame_decode(
    buf: ?[*]const u8,
    buf_len: u32,
    out_header_buf: ?[*]u8,
    out_payload_ptr: ?*[*]const u8,
    out_payload_len: ?*u32,
) callconv(.c) c_int {
    // Null-guard all required pointer parameters
    const buf_raw = buf orelse return -1;
    const out_hdr_raw = out_header_buf orelse return -1;
    const out_payload_ptr_nn = out_payload_ptr orelse return -1;
    const out_payload_len_nn = out_payload_len orelse return -1;

    const in_slice = buf_raw[0..buf_len];
    const f = decode(in_slice) catch return -1;

    // Serialize header back into the output buffer
    const out_hdr: *[header.HEADER_SIZE]u8 = @ptrCast(out_hdr_raw);
    f.header.serialize(out_hdr) catch return -2;

    out_payload_ptr_nn.* = if (f.payload.len > 0) f.payload.ptr else buf_raw;
    out_payload_len_nn.* = @intCast(f.payload.len);
    return 0;
}

/// Create a ring buffer. Returns opaque pointer, or null on failure.
export fn nexlink_ring_buffer_create(
    num_slots: u32,
    slot_size: u32,
) callconv(.c) ?*RingBuffer {
    const rb = std.heap.c_allocator.create(RingBuffer) catch return null;
    rb.* = RingBuffer.init(std.heap.c_allocator, num_slots, slot_size) catch {
        std.heap.c_allocator.destroy(rb);
        return null;
    };
    return rb;
}

/// Destroy a ring buffer.
export fn nexlink_ring_buffer_destroy(rb: ?*RingBuffer) callconv(.c) void {
    if (rb) |r| {
        r.deinit();
        std.heap.c_allocator.destroy(r);
    }
}

/// Reserve a slot in the ring buffer. Returns pointer to slot, or null if full.
export fn nexlink_ring_buffer_reserve(rb: ?*RingBuffer) callconv(.c) ?[*]u8 {
    if (rb) |r| {
        if (r.reserve()) |slot| return slot.ptr;
    }
    return null;
}

/// Commit a reserved slot.
export fn nexlink_ring_buffer_commit(rb: ?*RingBuffer) callconv(.c) void {
    if (rb) |r| r.commit();
}

/// Peek at the next readable slot. Returns pointer, or null if empty.
export fn nexlink_ring_buffer_peek(rb: ?*RingBuffer) callconv(.c) ?[*]const u8 {
    if (rb) |r| {
        if (r.peek()) |slot| return slot.ptr;
    }
    return null;
}

/// Release a consumed slot.
export fn nexlink_ring_buffer_release(rb: ?*RingBuffer) callconv(.c) void {
    if (rb) |r| r.release();
}

/// Compute CRC-32C over a buffer. Returns 0 if data is null.
export fn nexlink_crc32c(data: ?[*]const u8, len: u32) callconv(.c) u32 {
    const d = data orelse return 0;
    return crc.compute(d[0..len]);
}

// Force test runner to include all module tests
comptime {
    _ = header;
    _ = frame;
    _ = @import("crc.zig");
    _ = options;
    _ = ring_buffer;
    _ = memory_pool;
    _ = overlay;
    _ = mock;
}
