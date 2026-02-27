// tests/overlay_test.zig — Integration tests for NexLink UDP overlay encapsulation
//
// Tests encap/decap roundtrip, header validation, MTU calculation,
// and end-to-end overlay wrapping of NexLink frames.

const std = @import("std");
const testing = std.testing;
const mem = std.mem;
const nexlink = @import("nexlink");

const OverlayHeader = nexlink.OverlayHeader;
const FrameHeader = nexlink.FrameHeader;
const FrameType = nexlink.FrameType;

const OVERLAY_MAGIC = nexlink.overlay.OVERLAY_MAGIC;
const OVERLAY_VERSION = nexlink.overlay.OVERLAY_VERSION;
const OVERLAY_HEADER_SIZE = nexlink.overlay.OVERLAY_HEADER_SIZE;
const OVERLAY_OVERHEAD_IPV4 = nexlink.overlay.OVERLAY_OVERHEAD_IPV4;
const OVERLAY_OVERHEAD_IPV6 = nexlink.overlay.OVERLAY_OVERHEAD_IPV6;
const DEFAULT_INNER_MTU = nexlink.overlay.DEFAULT_INNER_MTU;

// ── Overlay header tests ──

test "overlay: header roundtrip default" {
    const oh = OverlayHeader{};
    var buf: [OVERLAY_HEADER_SIZE]u8 = undefined;
    try oh.serialize(&buf);

    const decoded = try OverlayHeader.deserialize(&buf);
    try testing.expectEqual(OVERLAY_MAGIC, decoded.magic);
    try testing.expectEqual(OVERLAY_VERSION, decoded.version);
    try testing.expectEqual(@as(u32, 0), decoded.vni);
}

test "overlay: header with VNI and flags" {
    const oh = OverlayHeader{
        .vni = 0x00FFFFFF,
        .flags = .{ .control = true, .fragment = true },
    };

    var buf: [OVERLAY_HEADER_SIZE]u8 = undefined;
    try oh.serialize(&buf);

    const decoded = try OverlayHeader.deserialize(&buf);
    try testing.expectEqual(@as(u32, 0x00FFFFFF), decoded.vni);
    try testing.expect(decoded.flags.control);
    try testing.expect(decoded.flags.fragment);
}

test "overlay: header max VNI" {
    const oh = OverlayHeader{ .vni = 0xFFFFFFFF };
    var buf: [OVERLAY_HEADER_SIZE]u8 = undefined;
    try oh.serialize(&buf);

    const decoded = try OverlayHeader.deserialize(&buf);
    try testing.expectEqual(@as(u32, 0xFFFFFFFF), decoded.vni);
}

// ── Encapsulation/decapsulation roundtrip ──

test "overlay: encapsulate/decapsulate raw data" {
    const inner = "raw NexLink frame bytes here";
    var buf: [256]u8 = undefined;

    const n = try nexlink.encapsulate(12345, inner, &buf);
    try testing.expectEqual(OVERLAY_HEADER_SIZE + inner.len, n);

    const result = try nexlink.decapsulate(buf[0..n]);
    try testing.expectEqual(@as(u32, 12345), result.header.vni);
    try testing.expectEqualSlices(u8, inner, result.inner_frame);
}

test "overlay: encapsulate/decapsulate NexLink frame" {
    // Step 1: Encode a NexLink frame
    var hdr = FrameHeader.init(.data);
    hdr.stream_id = 77;
    const payload = "tensor gradient data";

    var frame_buf: [512]u8 = undefined;
    const frame_len = try nexlink.encode(&hdr, &.{}, payload, &frame_buf);

    // Step 2: Encapsulate in overlay
    var overlay_buf: [1024]u8 = undefined;
    const overlay_len = try nexlink.encapsulate(9999, frame_buf[0..frame_len], &overlay_buf);

    // Step 3: Decapsulate
    const result = try nexlink.decapsulate(overlay_buf[0..overlay_len]);
    try testing.expectEqual(@as(u32, 9999), result.header.vni);

    // Step 4: Decode inner NexLink frame
    const inner_frame = try nexlink.decode(result.inner_frame);
    try testing.expectEqual(FrameType.data, inner_frame.header.frame_type);
    try testing.expectEqual(@as(u32, 77), inner_frame.header.stream_id);
    try testing.expectEqualSlices(u8, payload, inner_frame.payload);
}

test "overlay: VNI zero" {
    const inner = "data";
    var buf: [64]u8 = undefined;

    const n = try nexlink.encapsulate(0, inner, &buf);
    const result = try nexlink.decapsulate(buf[0..n]);
    try testing.expectEqual(@as(u32, 0), result.header.vni);
}

// ── Error handling ──

test "overlay: invalid magic rejected" {
    var buf: [OVERLAY_HEADER_SIZE]u8 = .{0} ** OVERLAY_HEADER_SIZE;
    mem.writeInt(u16, buf[0..2], 0xDEAD, .big);
    buf[2] = 0x10; // version = 1

    try testing.expectError(error.InvalidMagic, OverlayHeader.deserialize(&buf));
}

test "overlay: version zero rejected" {
    var buf: [OVERLAY_HEADER_SIZE]u8 = undefined;
    mem.writeInt(u16, buf[0..2], OVERLAY_MAGIC, .big);
    buf[2] = 0x00; // version = 0
    buf[3] = 0;
    mem.writeInt(u32, buf[4..8], 0, .big);

    try testing.expectError(error.InvalidVersion, OverlayHeader.deserialize(&buf));
}

test "overlay: buffer too small for encapsulate" {
    const inner = "some data that needs encapsulation";
    var small: [4]u8 = undefined;
    try testing.expectError(error.BufferTooSmall, nexlink.encapsulate(1, inner, &small));
}

test "overlay: buffer too small for decapsulate" {
    var small: [4]u8 = .{0} ** 4;
    try testing.expectError(error.BufferTooSmall, nexlink.decapsulate(&small));
}

// ── MTU calculation tests ──

test "overlay: MTU calculation IPv4" {
    const inner_mtu = nexlink.overlay.maxInnerFrameSize(1500, false);
    try testing.expectEqual(@as(usize, 1450), inner_mtu);
}

test "overlay: MTU calculation IPv6" {
    const inner_mtu = nexlink.overlay.maxInnerFrameSize(1500, true);
    try testing.expectEqual(@as(usize, 1430), inner_mtu);
}

test "overlay: MTU calculation jumbo frame" {
    const inner_mtu = nexlink.overlay.maxInnerFrameSize(9000, false);
    try testing.expectEqual(@as(usize, 8950), inner_mtu);
}

test "overlay: MTU calculation tiny" {
    try testing.expectEqual(@as(usize, 0), nexlink.overlay.maxInnerFrameSize(10, false));
    try testing.expectEqual(@as(usize, 0), nexlink.overlay.maxInnerFrameSize(0, false));
}

test "overlay: overhead constants correct" {
    // IPv4: Ethernet(14) + IP(20) + UDP(8) + Overlay(8) = 50
    try testing.expectEqual(@as(usize, 50), OVERLAY_OVERHEAD_IPV4);
    // IPv6: Ethernet(14) + IPv6(40) + UDP(8) + Overlay(8) = 70
    try testing.expectEqual(@as(usize, 70), OVERLAY_OVERHEAD_IPV6);
}

// ── Wire format verification ──

test "overlay: wire format magic bytes" {
    const oh = OverlayHeader{};
    var buf: [OVERLAY_HEADER_SIZE]u8 = undefined;
    try oh.serialize(&buf);

    // Verify magic bytes are 'N' 'X' in big-endian
    try testing.expectEqual(@as(u8, 0x4E), buf[0]); // 'N'
    try testing.expectEqual(@as(u8, 0x58), buf[1]); // 'X'
}

test "overlay: wire format VNI byte order" {
    const oh = OverlayHeader{ .vni = 0x01020304 };
    var buf: [OVERLAY_HEADER_SIZE]u8 = undefined;
    try oh.serialize(&buf);

    // VNI at bytes 4-7, big-endian
    try testing.expectEqual(@as(u8, 0x01), buf[4]);
    try testing.expectEqual(@as(u8, 0x02), buf[5]);
    try testing.expectEqual(@as(u8, 0x03), buf[6]);
    try testing.expectEqual(@as(u8, 0x04), buf[7]);
}
