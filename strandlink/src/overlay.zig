// overlay.zig — UDP overlay encapsulation for NexLink frames (Tier 3 compatibility)
//
// Encapsulates NexLink frames inside UDP datagrams for transport over standard
// IP networks. Modeled after VXLAN (RFC 7348) and Geneve (RFC 8926).
//
// Overlay header (8 bytes):
//   Bytes 0-1: Magic (0x4E58 = "NX")
//   Byte  2:   Version (upper 4 bits) | Flags (lower 4 bits)
//   Byte  3:   Reserved (must be 0)
//   Bytes 4-7: VNI (Virtual Network Identifier, 32 bits)
//
// The standard overlay port is 6477.

const std = @import("std");
const mem = std.mem;
const testing = std.testing;
const crc = @import("crc.zig");

/// Overlay magic bytes: "NX" = 0x4E58
pub const OVERLAY_MAGIC: u16 = 0x4E58;

/// Current overlay version.
pub const OVERLAY_VERSION: u4 = 1;

/// Default UDP destination port for NexLink overlay.
pub const OVERLAY_PORT: u16 = 6477;

/// Overlay header size in bytes.
pub const OVERLAY_HEADER_SIZE: usize = 8;

/// Overlay encapsulation overhead for MTU calculation:
///   Outer Ethernet (14) + IPv4 (20) + UDP (8) + Overlay Header (8) = 50 bytes
pub const OVERLAY_OVERHEAD_IPV4: usize = 14 + 20 + 8 + OVERLAY_HEADER_SIZE;

/// Overlay encapsulation overhead with IPv6:
///   Outer Ethernet (14) + IPv6 (40) + UDP (8) + Overlay Header (8) = 70 bytes
pub const OVERLAY_OVERHEAD_IPV6: usize = 14 + 40 + 8 + OVERLAY_HEADER_SIZE;

/// Default inner MTU for 1500 outer MTU with IPv4
pub const DEFAULT_INNER_MTU: usize = 1500 - OVERLAY_OVERHEAD_IPV4;

/// Overlay flags
pub const OverlayFlags = packed struct(u4) {
    /// Inner frame is a control message (not data)
    control: bool = false,
    /// Overlay-level fragmentation in progress
    fragment: bool = false,
    /// Reserved bits
    _reserved: u2 = 0,
};

/// Overlay header (8 bytes).
pub const OverlayHeader = struct {
    magic: u16 = OVERLAY_MAGIC,
    version: u4 = OVERLAY_VERSION,
    flags: OverlayFlags = .{},
    reserved: u8 = 0,
    vni: u32 = 0,

    /// Serialize the overlay header into a buffer (big-endian).
    pub fn serialize(self: *const OverlayHeader, buf: []u8) error{BufferTooSmall}!void {
        if (buf.len < OVERLAY_HEADER_SIZE) return error.BufferTooSmall;

        // Bytes 0-1: magic
        mem.writeInt(u16, buf[0..2], self.magic, .big);

        // Byte 2: version(4 high bits) | flags(4 low bits)
        buf[2] = (@as(u8, self.version) << 4) | @as(u8, @as(u4, @bitCast(self.flags)));

        // Byte 3: reserved
        buf[3] = self.reserved;

        // Bytes 4-7: VNI
        mem.writeInt(u32, buf[4..8], self.vni, .big);
    }

    /// Deserialize an overlay header from a buffer.
    pub fn deserialize(buf: []const u8) error{ BufferTooSmall, InvalidMagic, InvalidVersion }!OverlayHeader {
        if (buf.len < OVERLAY_HEADER_SIZE) return error.BufferTooSmall;

        const magic = mem.readInt(u16, buf[0..2], .big);
        if (magic != OVERLAY_MAGIC) return error.InvalidMagic;

        const version: u4 = @truncate(buf[2] >> 4);
        if (version == 0) return error.InvalidVersion;

        const flags: OverlayFlags = @bitCast(@as(u4, @truncate(buf[2])));
        const reserved = buf[3];
        const vni = mem.readInt(u32, buf[4..8], .big);

        return OverlayHeader{
            .magic = magic,
            .version = version,
            .flags = flags,
            .reserved = reserved,
            .vni = vni,
        };
    }
};

/// Encapsulate a NexLink frame inside an overlay packet.
///
/// Writes: [OverlayHeader (8B)] [NexLink Frame (variable)]
///
/// Returns the total number of bytes written to `out_buf`.
pub fn encapsulate(
    vni: u32,
    frame_data: []const u8,
    out_buf: []u8,
) error{BufferTooSmall}!usize {
    const total = OVERLAY_HEADER_SIZE + frame_data.len;
    if (out_buf.len < total) return error.BufferTooSmall;

    const oh = OverlayHeader{ .vni = vni };
    try oh.serialize(out_buf[0..OVERLAY_HEADER_SIZE]);

    @memcpy(out_buf[OVERLAY_HEADER_SIZE .. OVERLAY_HEADER_SIZE + frame_data.len], frame_data);

    return total;
}

/// Decapsulate an overlay packet, returning the overlay header and inner frame data.
///
/// The returned inner_frame slice is a view into the input buffer (zero-copy).
pub fn decapsulate(buf: []const u8) error{ BufferTooSmall, InvalidMagic, InvalidVersion }!struct {
    header: OverlayHeader,
    inner_frame: []const u8,
} {
    const oh = try OverlayHeader.deserialize(buf);
    return .{
        .header = oh,
        .inner_frame = buf[OVERLAY_HEADER_SIZE..],
    };
}

/// Calculate the maximum inner frame size given an outer MTU and IP version.
pub fn maxInnerFrameSize(outer_mtu: usize, ipv6: bool) usize {
    const overhead = if (ipv6) OVERLAY_OVERHEAD_IPV6 else OVERLAY_OVERHEAD_IPV4;
    if (outer_mtu <= overhead) return 0;
    return outer_mtu - overhead;
}

// ── Unit tests ──

test "overlay header serialize/deserialize roundtrip" {
    const oh = OverlayHeader{
        .version = 1,
        .flags = .{ .control = true },
        .vni = 0x00ABCDEF,
    };

    var buf: [OVERLAY_HEADER_SIZE]u8 = undefined;
    try oh.serialize(&buf);

    const decoded = try OverlayHeader.deserialize(&buf);
    try testing.expectEqual(OVERLAY_MAGIC, decoded.magic);
    try testing.expectEqual(@as(u4, 1), decoded.version);
    try testing.expect(decoded.flags.control);
    try testing.expect(!decoded.flags.fragment);
    try testing.expectEqual(@as(u32, 0x00ABCDEF), decoded.vni);
}

test "overlay encapsulate/decapsulate roundtrip" {
    const frame = "NexLink frame content here";
    var buf: [256]u8 = undefined;

    const written = try encapsulate(42, frame, &buf);
    try testing.expectEqual(OVERLAY_HEADER_SIZE + frame.len, written);

    const result = try decapsulate(buf[0..written]);
    try testing.expectEqual(@as(u32, 42), result.header.vni);
    try testing.expectEqualSlices(u8, frame, result.inner_frame);
}

test "overlay invalid magic rejected" {
    var buf: [OVERLAY_HEADER_SIZE]u8 = .{0} ** OVERLAY_HEADER_SIZE;
    // Write wrong magic
    mem.writeInt(u16, buf[0..2], 0xBEEF, .big);
    buf[2] = 0x10; // version=1
    try testing.expectError(error.InvalidMagic, OverlayHeader.deserialize(&buf));
}

test "overlay buffer too small" {
    var small: [4]u8 = undefined;
    try testing.expectError(error.BufferTooSmall, OverlayHeader.deserialize(&small));

    const oh = OverlayHeader{};
    try testing.expectError(error.BufferTooSmall, oh.serialize(&small));
}

test "overlay max inner frame size" {
    // Standard 1500 MTU with IPv4
    try testing.expectEqual(DEFAULT_INNER_MTU, maxInnerFrameSize(1500, false));

    // IPv6 has more overhead
    const ipv6_inner = maxInnerFrameSize(1500, true);
    try testing.expect(ipv6_inner < DEFAULT_INNER_MTU);

    // Tiny MTU
    try testing.expectEqual(@as(usize, 0), maxInnerFrameSize(10, false));
}

test "overlay default inner MTU is correct" {
    // 1500 - (14 + 20 + 8 + 8) = 1500 - 50 = 1450
    try testing.expectEqual(@as(usize, 1450), DEFAULT_INNER_MTU);
}
