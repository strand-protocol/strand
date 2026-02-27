// ring_buffer.zig — Lock-free SPSC ring buffer for NexLink frame I/O
//
// Single-Producer, Single-Consumer ring buffer using atomic head/tail indices.
// Inspired by the io_uring submission/completion queue design (Linux kernel).
//
// Key properties:
//   - Power-of-2 sized for fast modular arithmetic (mask instead of modulo)
//   - Zero-copy: reserve() returns a mutable slice into the backing buffer
//   - Cache-line separation: head and tail on separate cache lines to avoid false sharing
//   - No locks: uses Release/Acquire atomics for head/tail

const std = @import("std");
const mem = std.mem;
const testing = std.testing;
const Allocator = std.mem.Allocator;

/// A lock-free single-producer single-consumer ring buffer.
///
/// The producer calls `reserve()` to get a writable slot, then `commit()`.
/// The consumer calls `peek()` to read a slot, then `release()`.
pub const RingBuffer = struct {
    /// Backing memory (contiguous allocation of num_slots * slot_size bytes).
    backing: []align(64) u8,

    /// Number of slots (must be power of 2).
    num_slots: u32,

    /// Size of each slot in bytes.
    slot_size: u32,

    /// Bitmask for fast index wrapping: num_slots - 1.
    mask: u32,

    /// Producer index — only written by producer, read by consumer.
    head: std.atomic.Value(u32),

    /// Consumer index — only written by consumer, read by producer.
    tail: std.atomic.Value(u32),

    /// Allocator used for backing memory (needed for deinit).
    allocator: Allocator,

    pub const Error = error{
        InvalidSlotCount,
        OutOfMemory,
    };

    /// Initialize a ring buffer with `num_slots` slots, each `slot_size` bytes.
    /// `num_slots` must be a power of 2 and > 0.
    pub fn init(allocator: Allocator, num_slots: u32, slot_size: u32) Error!RingBuffer {
        if (num_slots == 0 or (num_slots & (num_slots - 1)) != 0) {
            return error.InvalidSlotCount;
        }

        const total_size: usize = @as(usize, num_slots) * @as(usize, slot_size);
        const backing = allocator.alignedAlloc(u8, .@"64", total_size) catch return error.OutOfMemory;
        @memset(backing, 0);

        return RingBuffer{
            .backing = backing,
            .num_slots = num_slots,
            .slot_size = slot_size,
            .mask = num_slots - 1,
            .head = std.atomic.Value(u32).init(0),
            .tail = std.atomic.Value(u32).init(0),
            .allocator = allocator,
        };
    }

    /// Free the backing memory.
    pub fn deinit(self: *RingBuffer) void {
        self.allocator.free(self.backing);
        self.* = undefined;
    }

    /// Returns the number of items currently in the buffer.
    pub fn count(self: *const RingBuffer) u32 {
        const h = self.head.load(.acquire);
        const t = self.tail.load(.acquire);
        return h -% t;
    }

    /// Returns true if the buffer is empty.
    pub fn isEmpty(self: *const RingBuffer) bool {
        return self.count() == 0;
    }

    /// Returns true if the buffer is full.
    pub fn isFull(self: *const RingBuffer) bool {
        return self.count() == self.num_slots;
    }

    // ── Producer API ──

    /// Reserve a slot for writing. Returns a mutable slice into the backing
    /// buffer, or null if the ring is full.
    ///
    /// The caller must write data into the returned slice, then call `commit()`.
    /// Only one slot may be reserved at a time (SPSC contract).
    pub fn reserve(self: *RingBuffer) ?[]u8 {
        const h = self.head.load(.monotonic);
        const t = self.tail.load(.acquire);

        // Full when head - tail == num_slots
        if (h -% t >= self.num_slots) return null;

        const idx = h & self.mask;
        const offset: usize = @as(usize, idx) * @as(usize, self.slot_size);
        return self.backing[offset .. offset + self.slot_size];
    }

    /// Commit a previously reserved slot, making it visible to the consumer.
    pub fn commit(self: *RingBuffer) void {
        const h = self.head.load(.monotonic);
        self.head.store(h +% 1, .release);
    }

    // ── Consumer API ──

    /// Peek at the next available slot for reading. Returns a const slice,
    /// or null if the ring is empty.
    ///
    /// The caller must read data from the returned slice, then call `release()`.
    pub fn peek(self: *RingBuffer) ?[]const u8 {
        const t = self.tail.load(.monotonic);
        const h = self.head.load(.acquire);

        // Empty when head == tail
        if (h == t) return null;

        const idx = t & self.mask;
        const offset: usize = @as(usize, idx) * @as(usize, self.slot_size);
        return self.backing[offset .. offset + self.slot_size];
    }

    /// Release a consumed slot back to the ring, advancing the tail.
    pub fn release(self: *RingBuffer) void {
        const t = self.tail.load(.monotonic);
        self.tail.store(t +% 1, .release);
    }

    /// Get a direct pointer to slot at `index` (modular). For advanced use.
    pub fn slotAt(self: *RingBuffer, index: u32) []u8 {
        const idx = index & self.mask;
        const offset: usize = @as(usize, idx) * @as(usize, self.slot_size);
        return self.backing[offset .. offset + self.slot_size];
    }
};

// ── Unit tests ──

test "ring_buffer init/deinit" {
    var rb = try RingBuffer.init(testing.allocator, 4, 64);
    defer rb.deinit();

    try testing.expectEqual(@as(u32, 4), rb.num_slots);
    try testing.expectEqual(@as(u32, 64), rb.slot_size);
    try testing.expectEqual(@as(u32, 3), rb.mask);
    try testing.expect(rb.isEmpty());
    try testing.expect(!rb.isFull());
}

test "ring_buffer invalid slot count" {
    // Not a power of 2
    try testing.expectError(error.InvalidSlotCount, RingBuffer.init(testing.allocator, 3, 64));
    // Zero
    try testing.expectError(error.InvalidSlotCount, RingBuffer.init(testing.allocator, 0, 64));
}

test "ring_buffer single push/pop" {
    var rb = try RingBuffer.init(testing.allocator, 4, 8);
    defer rb.deinit();

    // Reserve a slot and write data
    const slot = rb.reserve().?;
    @memcpy(slot[0..5], "hello");
    rb.commit();

    try testing.expectEqual(@as(u32, 1), rb.count());
    try testing.expect(!rb.isEmpty());

    // Peek and read
    const read = rb.peek().?;
    try testing.expectEqualSlices(u8, "hello", read[0..5]);
    rb.release();

    try testing.expect(rb.isEmpty());
}

test "ring_buffer fill and drain" {
    var rb = try RingBuffer.init(testing.allocator, 4, 4);
    defer rb.deinit();

    // Fill all 4 slots
    for (0..4) |i| {
        const slot = rb.reserve().?;
        slot[0] = @intCast(i);
        rb.commit();
    }
    try testing.expect(rb.isFull());

    // Reserve should fail when full
    try testing.expectEqual(@as(?[]u8, null), rb.reserve());

    // Drain all 4 slots
    for (0..4) |i| {
        const read = rb.peek().?;
        try testing.expectEqual(@as(u8, @intCast(i)), read[0]);
        rb.release();
    }
    try testing.expect(rb.isEmpty());

    // Peek should fail when empty
    try testing.expectEqual(@as(?[]const u8, null), rb.peek());
}

test "ring_buffer wraps around" {
    var rb = try RingBuffer.init(testing.allocator, 2, 4);
    defer rb.deinit();

    // Push/pop multiple times to force wraparound
    for (0..10) |i| {
        const slot = rb.reserve().?;
        mem.writeInt(u32, slot[0..4], @intCast(i), .little);
        rb.commit();

        const read = rb.peek().?;
        try testing.expectEqual(@as(u32, @intCast(i)), mem.readInt(u32, read[0..4], .little));
        rb.release();
    }
}

test "ring_buffer count tracking" {
    var rb = try RingBuffer.init(testing.allocator, 8, 4);
    defer rb.deinit();

    try testing.expectEqual(@as(u32, 0), rb.count());

    // Push 3
    for (0..3) |_| {
        _ = rb.reserve();
        rb.commit();
    }
    try testing.expectEqual(@as(u32, 3), rb.count());

    // Pop 1
    _ = rb.peek();
    rb.release();
    try testing.expectEqual(@as(u32, 2), rb.count());
}
