// platform/mock.zig — Mock platform backend for StrandLink unit testing
//
// Provides an in-memory loopback NIC interface. send() places a frame into
// a ring buffer, recv() reads from it. This simulates the NIC TX/RX path
// without requiring any hardware or kernel drivers.

const std = @import("std");
const mem = std.mem;
const testing = std.testing;
const Allocator = std.mem.Allocator;
const RingBuffer = @import("../ring_buffer.zig").RingBuffer;
const frame_mod = @import("../frame.zig");

/// Default number of slots in the mock ring buffer.
const DEFAULT_NUM_SLOTS: u32 = 16;

/// Default slot size (large enough for MAX_FRAME_SIZE).
const DEFAULT_SLOT_SIZE: u32 = 2048;

/// A mock NIC platform for testing.
///
/// Internally uses a ring buffer as a loopback: frames sent via `send()`
/// are placed into the ring and can be received via `recv()`.
pub const MockPlatform = struct {
    ring: RingBuffer,

    pub const Error = error{
        RingFull,
        RingEmpty,
        FrameTooLarge,
        InvalidSlotCount,
        OutOfMemory,
    };

    /// Initialize the mock platform with default ring buffer parameters.
    pub fn init(allocator: Allocator) Error!MockPlatform {
        return initWithConfig(allocator, DEFAULT_NUM_SLOTS, DEFAULT_SLOT_SIZE);
    }

    /// Initialize with custom ring buffer configuration.
    pub fn initWithConfig(allocator: Allocator, num_slots: u32, slot_size: u32) Error!MockPlatform {
        const ring = RingBuffer.init(allocator, num_slots, slot_size) catch |err| switch (err) {
            error.InvalidSlotCount => return error.InvalidSlotCount,
            error.OutOfMemory => return error.OutOfMemory,
        };
        return MockPlatform{ .ring = ring };
    }

    /// Release all resources.
    pub fn deinit(self: *MockPlatform) void {
        self.ring.deinit();
        self.* = undefined;
    }

    /// Send a frame (place it into the loopback ring buffer).
    ///
    /// The frame data is copied into a ring buffer slot. The first 4 bytes
    /// of each slot store the frame length as a little-endian u32, followed
    /// by the frame data.
    pub fn send(self: *MockPlatform, frame_data: []const u8) Error!void {
        if (frame_data.len + 4 > self.ring.slot_size) return error.FrameTooLarge;

        const slot = self.ring.reserve() orelse return error.RingFull;

        // Write length prefix (4 bytes LE) + frame data
        mem.writeInt(u32, slot[0..4], @intCast(frame_data.len), .little);
        @memcpy(slot[4 .. 4 + frame_data.len], frame_data);

        self.ring.commit();
    }

    /// Receive a frame from the loopback ring buffer.
    ///
    /// Copies the frame data into `out_buf` and returns the number of bytes copied.
    /// Returns error.RingEmpty if no frames are available.
    pub fn recv(self: *MockPlatform, out_buf: []u8) Error!usize {
        const slot = self.ring.peek() orelse return error.RingEmpty;

        // Read length prefix
        const frame_len: usize = mem.readInt(u32, slot[0..4], .little);

        if (frame_len > out_buf.len) {
            // Still release the slot to avoid deadlock
            self.ring.release();
            return error.FrameTooLarge;
        }

        @memcpy(out_buf[0..frame_len], slot[4 .. 4 + frame_len]);
        self.ring.release();

        return frame_len;
    }

    /// Returns the number of frames currently in the loopback buffer.
    pub fn pendingCount(self: *MockPlatform) u32 {
        return self.ring.count();
    }

    /// Returns true if there are no pending frames.
    pub fn isEmpty(self: *MockPlatform) bool {
        return self.ring.isEmpty();
    }
};

// ── Unit tests ──

test "mock_platform init/deinit" {
    var mock = try MockPlatform.init(testing.allocator);
    defer mock.deinit();

    try testing.expect(mock.isEmpty());
    try testing.expectEqual(@as(u32, 0), mock.pendingCount());
}

test "mock_platform send/recv loopback" {
    var mock = try MockPlatform.init(testing.allocator);
    defer mock.deinit();

    const data = "Hello, StrandLink!";
    try mock.send(data);

    try testing.expectEqual(@as(u32, 1), mock.pendingCount());

    var buf: [256]u8 = undefined;
    const received_len = try mock.recv(&buf);

    try testing.expectEqual(data.len, received_len);
    try testing.expectEqualSlices(u8, data, buf[0..received_len]);
    try testing.expect(mock.isEmpty());
}

test "mock_platform multiple frames" {
    var mock = try MockPlatform.init(testing.allocator);
    defer mock.deinit();

    // Send 3 frames
    try mock.send("frame_1");
    try mock.send("frame_2");
    try mock.send("frame_3");

    try testing.expectEqual(@as(u32, 3), mock.pendingCount());

    // Receive in FIFO order
    var buf: [256]u8 = undefined;

    var n = try mock.recv(&buf);
    try testing.expectEqualSlices(u8, "frame_1", buf[0..n]);

    n = try mock.recv(&buf);
    try testing.expectEqualSlices(u8, "frame_2", buf[0..n]);

    n = try mock.recv(&buf);
    try testing.expectEqualSlices(u8, "frame_3", buf[0..n]);

    try testing.expect(mock.isEmpty());
}

test "mock_platform recv empty" {
    var mock = try MockPlatform.init(testing.allocator);
    defer mock.deinit();

    var buf: [256]u8 = undefined;
    try testing.expectError(error.RingEmpty, mock.recv(&buf));
}

test "mock_platform full ring" {
    // Small ring: 2 slots
    var mock = try MockPlatform.initWithConfig(testing.allocator, 2, 64);
    defer mock.deinit();

    try mock.send("a");
    try mock.send("b");
    try testing.expectError(error.RingFull, mock.send("c"));

    // Drain one and retry
    var buf: [64]u8 = undefined;
    _ = try mock.recv(&buf);
    try mock.send("c");
}

test "mock_platform frame too large for slot" {
    var mock = try MockPlatform.initWithConfig(testing.allocator, 4, 16);
    defer mock.deinit();

    // Slot is 16 bytes, 4 used for length prefix, so max frame is 12 bytes
    const too_large = "this is definitely more than 12 bytes of data!!!";
    try testing.expectError(error.FrameTooLarge, mock.send(too_large));
}

test "mock_platform end-to-end with frame encode/decode" {
    var mock = try MockPlatform.init(testing.allocator);
    defer mock.deinit();

    // Encode a StrandLink frame
    var hdr = frame_mod.FrameHeader.init(.data);
    hdr.stream_id = 100;
    hdr.sequence_number = 1;

    const payload = "tensor data";
    var frame_buf: [1024]u8 = undefined;
    const frame_len = try frame_mod.encode(&hdr, &.{}, payload, &frame_buf);

    // Send through mock loopback
    try mock.send(frame_buf[0..frame_len]);

    // Receive
    var recv_buf: [1024]u8 = undefined;
    const recv_len = try mock.recv(&recv_buf);

    // Decode
    const frame = try frame_mod.decode(recv_buf[0..recv_len]);
    try testing.expectEqual(frame_mod.FrameType.data, frame.header.frame_type);
    try testing.expectEqual(@as(u32, 100), frame.header.stream_id);
    try testing.expectEqualSlices(u8, payload, frame.payload);
}
