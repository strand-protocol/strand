// tests/ring_buffer_test.zig — Integration tests for StrandLink SPSC ring buffer
//
// Tests push/pop semantics, wraparound, full/empty conditions, and
// multi-threaded SPSC correctness.

const std = @import("std");
const testing = std.testing;
const mem = std.mem;
const strandlink = @import("strandlink");

const RingBuffer = strandlink.RingBuffer;

test "ring_buffer: basic reserve/commit/peek/release cycle" {
    var rb = try RingBuffer.init(testing.allocator, 8, 128);
    defer rb.deinit();

    // Reserve and write
    const slot = rb.reserve().?;
    @memcpy(slot[0..4], "test");
    rb.commit();

    try testing.expectEqual(@as(u32, 1), rb.count());

    // Peek and verify
    const read = rb.peek().?;
    try testing.expectEqualSlices(u8, "test", read[0..4]);
    rb.release();

    try testing.expectEqual(@as(u32, 0), rb.count());
}

test "ring_buffer: fill to capacity" {
    var rb = try RingBuffer.init(testing.allocator, 4, 16);
    defer rb.deinit();

    // Fill all slots
    for (0..4) |i| {
        const slot = rb.reserve().?;
        slot[0] = @intCast(i);
        rb.commit();
    }

    try testing.expect(rb.isFull());
    try testing.expectEqual(@as(?[]u8, null), rb.reserve());

    // Verify FIFO order
    for (0..4) |i| {
        const read = rb.peek().?;
        try testing.expectEqual(@as(u8, @intCast(i)), read[0]);
        rb.release();
    }

    try testing.expect(rb.isEmpty());
}

test "ring_buffer: wraparound correctness" {
    var rb = try RingBuffer.init(testing.allocator, 4, 8);
    defer rb.deinit();

    // Do many more iterations than slots to exercise wraparound
    for (0..100) |i| {
        const slot = rb.reserve().?;
        mem.writeInt(u64, slot[0..8], @intCast(i), .little);
        rb.commit();

        const read = rb.peek().?;
        try testing.expectEqual(@as(u64, @intCast(i)), mem.readInt(u64, read[0..8], .little));
        rb.release();
    }
}

test "ring_buffer: interleaved push/pop pattern" {
    var rb = try RingBuffer.init(testing.allocator, 8, 4);
    defer rb.deinit();

    // Push 3, pop 1, push 2, pop 4 — exercises various head/tail positions
    for (0..3) |i| {
        const s = rb.reserve().?;
        s[0] = @intCast(i);
        rb.commit();
    }
    try testing.expectEqual(@as(u32, 3), rb.count());

    // Pop 1
    const r0 = rb.peek().?;
    try testing.expectEqual(@as(u8, 0), r0[0]);
    rb.release();
    try testing.expectEqual(@as(u32, 2), rb.count());

    // Push 2 more
    for (3..5) |i| {
        const s = rb.reserve().?;
        s[0] = @intCast(i);
        rb.commit();
    }
    try testing.expectEqual(@as(u32, 4), rb.count());

    // Pop remaining 4
    for (1..5) |i| {
        const r = rb.peek().?;
        try testing.expectEqual(@as(u8, @intCast(i)), r[0]);
        rb.release();
    }
    try testing.expect(rb.isEmpty());
}

test "ring_buffer: slot data isolation" {
    var rb = try RingBuffer.init(testing.allocator, 4, 32);
    defer rb.deinit();

    // Write different patterns to each slot
    for (0..4) |i| {
        const slot = rb.reserve().?;
        @memset(slot, @intCast(i * 0x11));
        rb.commit();
    }

    // Verify each slot has its own distinct pattern
    for (0..4) |i| {
        const read = rb.peek().?;
        const expected: u8 = @intCast(i * 0x11);
        for (read) |byte| {
            try testing.expectEqual(expected, byte);
        }
        rb.release();
    }
}

test "ring_buffer: power-of-2 enforcement" {
    try testing.expectError(error.InvalidSlotCount, RingBuffer.init(testing.allocator, 0, 64));
    try testing.expectError(error.InvalidSlotCount, RingBuffer.init(testing.allocator, 3, 64));
    try testing.expectError(error.InvalidSlotCount, RingBuffer.init(testing.allocator, 5, 64));
    try testing.expectError(error.InvalidSlotCount, RingBuffer.init(testing.allocator, 6, 64));
    try testing.expectError(error.InvalidSlotCount, RingBuffer.init(testing.allocator, 7, 64));

    // Valid power-of-2 values should work
    var rb1 = try RingBuffer.init(testing.allocator, 1, 64);
    rb1.deinit();

    var rb2 = try RingBuffer.init(testing.allocator, 2, 64);
    rb2.deinit();

    var rb4 = try RingBuffer.init(testing.allocator, 4, 64);
    rb4.deinit();

    var rb16 = try RingBuffer.init(testing.allocator, 16, 64);
    rb16.deinit();
}

test "ring_buffer: large slot size" {
    // Simulate a ring buffer for tensor frames
    var rb = try RingBuffer.init(testing.allocator, 4, 4096);
    defer rb.deinit();

    const slot = rb.reserve().?;
    try testing.expectEqual(@as(usize, 4096), slot.len);

    // Write a pattern across the entire slot
    @memset(slot, 0xBE);
    rb.commit();

    const read = rb.peek().?;
    try testing.expectEqual(@as(usize, 4096), read.len);
    try testing.expectEqual(@as(u8, 0xBE), read[0]);
    try testing.expectEqual(@as(u8, 0xBE), read[4095]);
    rb.release();
}

test "ring_buffer: empty state operations" {
    var rb = try RingBuffer.init(testing.allocator, 4, 16);
    defer rb.deinit();

    try testing.expect(rb.isEmpty());
    try testing.expect(!rb.isFull());
    try testing.expectEqual(@as(u32, 0), rb.count());
    try testing.expectEqual(@as(?[]const u8, null), rb.peek());
}

test "ring_buffer: spsc with threads" {
    // Actual multi-threaded test: producer pushes N items, consumer drains them.
    const NUM_ITEMS: u32 = 1000;
    var rb = try RingBuffer.init(testing.allocator, 16, 8);
    defer rb.deinit();

    // Producer thread
    const producer = try std.Thread.spawn(.{}, struct {
        fn run(ring: *RingBuffer) void {
            var i: u32 = 0;
            while (i < NUM_ITEMS) {
                if (ring.reserve()) |slot| {
                    mem.writeInt(u32, slot[0..4], i, .little);
                    ring.commit();
                    i += 1;
                }
                // If full, spin-wait (acceptable for test)
            }
        }
    }.run, .{&rb});

    // Consumer in the main thread
    var received: u32 = 0;
    while (received < NUM_ITEMS) {
        if (rb.peek()) |read| {
            const val = mem.readInt(u32, read[0..4], .little);
            // Values should arrive in order (SPSC FIFO guarantee)
            std.debug.assert(val == received);
            rb.release();
            received += 1;
        }
        // If empty, spin-wait
    }

    producer.join();
    try testing.expectEqual(NUM_ITEMS, received);
    try testing.expect(rb.isEmpty());
}
