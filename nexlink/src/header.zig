// header.zig — NexLink L1 Frame Protocol Header (64 bytes)
//
// Defines the 64-byte fixed frame header for the NexLink AI-native frame protocol.
// The header is serialized in big-endian (network byte order) for wire compatibility.
// We do NOT use a Zig packed struct for the wire format because the spec has sub-byte
// fields (4-bit version, 4-bit priority, 4-bit qos_class) that pack across byte
// boundaries in ways that differ between Zig's packed layout and the network diagram.
// Instead we define a semantic struct and explicit serialize/deserialize functions.

const std = @import("std");
const builtin = std.builtin;
const mem = std.mem;
const testing = std.testing;

/// Protocol version. Current = 1.
pub const NEXLINK_VERSION: u4 = 1;

/// Fixed header size in bytes.
pub const HEADER_SIZE: usize = 64;

/// Maximum TLV options section size.
pub const MAX_OPTIONS_SIZE: usize = 256;

/// Maximum total frame size (64KB, jumbo via fragmentation).
pub const MAX_FRAME_SIZE: usize = 65535;

/// Minimum frame size: header + CRC-32C trailer.
pub const MIN_FRAME_SIZE: usize = HEADER_SIZE + 4;

// ── Flag bits (byte 1 of header, bits [0..7]) ──

pub const FrameFlags = packed struct(u8) {
    more_fragments: bool = false,
    compressed: bool = false,
    encrypted: bool = false,
    tensor_payload: bool = false,
    priority_express: bool = false,
    overlay_encap: bool = false,
    _reserved: u2 = 0,
};

// ── Frame type enumeration ──

pub const FrameType = enum(u16) {
    data = 0x0001,
    control = 0x0002,
    heartbeat = 0x0003,
    route_advertisement = 0x0004,
    trust_handshake = 0x0005,
    tensor_transfer = 0x0006,
    stream_control = 0x0007,
    _,
};

// ── QoS class ──

pub const QosClass = enum(u4) {
    best_effort = 0x0,
    reliable_ordered = 0x1,
    reliable_unordered = 0x2,
    probabilistic = 0x3,
    _,
};

// ── Tensor data types ──

pub const TensorDtype = enum(u8) {
    none = 0x00,
    float16 = 0x01,
    bfloat16 = 0x02,
    float32 = 0x03,
    float64 = 0x04,
    int8 = 0x05,
    int4 = 0x06,
    uint8 = 0x07,
    fp8_e4m3 = 0x08,
    fp8_e5m2 = 0x09,
    _,
};

/// 128-bit node identifier (derived from NexTrust identity key).
pub const NodeId = [16]u8;

/// Semantic representation of a NexLink frame header.
/// NOT a packed struct — use `serialize` / `deserialize` for wire format.
pub const FrameHeader = struct {
    version: u4 = NEXLINK_VERSION,
    flags: FrameFlags = .{},
    frame_type: FrameType = .data,
    frame_length: u32 = 0,
    stream_id: u32 = 0,
    sequence_number: u32 = 0,
    source_node_id: NodeId = .{0} ** 16,
    dest_node_id: NodeId = .{0} ** 16,
    priority: u4 = 0,
    qos_class: QosClass = .best_effort,
    tensor_dtype: TensorDtype = .none,
    tensor_alignment: u16 = 0,
    options_length: u16 = 0,
    timestamp: u64 = 0,

    /// Wire format (64 bytes, big-endian):
    ///
    ///  Byte  0:    version(4 high bits) | flags(4 low bits of flags byte — but flags is 8 bits)
    ///
    /// Spec layout byte 0-3:
    ///   [version:4][flags_hi:4] [flags_lo:4][frame_type_hi:4] [frame_type_lo:8] [pad:4 ... wait]
    ///
    /// Re-reading the spec diagram more carefully:
    ///   Byte 0: version(4) | flags high 4 bits
    ///   Byte 1: flags low 4 bits | frame_type high 4 bits
    ///   Byte 2-3: frame_type low 12 bits  -- that gives 16 bits for frame_type, good.
    ///
    /// Actually the diagram says: Version(4) | Flags(8) | Frame Type(16) which is 28 bits = 3.5 bytes.
    /// With the remaining 4 bits padding to fill byte 3. Let me re-read:
    ///
    /// The ASCII art shows bits 0-31 for bytes 0-3:
    ///   |  Version (4)  | Flags (8)     |        Frame Type (16)        |
    /// That's 4+8+16 = 28 bits in a 32-bit row. The diagram actually shows 4 columns of 8 bits each.
    /// Looking more closely, the version field spans bits 0-3 (half of byte 0), flags spans
    /// bits 4-11 (second half of byte 0 + first half of byte 1), frame_type spans bits 12-27
    /// (second half of byte 1 + bytes 2-3). That's only 28 bits in 32.
    ///
    /// BUT the CLAUDE.md says: Bytes 0-3: version(4b) | flags(8b) | frame_type(16b) | padding(4b)
    /// So there ARE 4 bits of padding to fill the 32-bit word.
    ///
    /// Wire layout (bytes 0-63):
    ///   Byte 0:     [version:4][flags_hi:4]        (version in high nibble, top 4 flag bits in low)
    ///   Byte 1:     [flags_lo:4][frame_type_hi:4]  (bottom 4 flag bits in high nibble, ft high nibble in low)
    ///   Byte 2:     [frame_type_lo:8]
    ///   Byte 3:     [padding:4][0000]  -- Wait, that only gives 12 bits for frame_type.
    ///
    /// Let me use a simpler, unambiguous approach consistent with CLAUDE.md:
    ///   Byte 0:     [version:4 MSB][flags_hi:4 LSB]
    ///   Byte 1:     [flags_lo:4 MSB][pad:4 LSB]
    ///   Byte 2-3:   frame_type (big-endian u16)
    ///
    /// Actually re-reading CLAUDE.md: "Bytes 0-3: version(4b) | flags(8b) | frame_type(16b) | padding(4b)"
    /// 4+8+16+4 = 32 bits. So:
    ///   bits [31..28] = version (4 bits)
    ///   bits [27..20] = flags   (8 bits)
    ///   bits [19..4]  = frame_type (16 bits)
    ///   bits [3..0]   = padding (4 bits)
    ///
    /// In big-endian byte order:
    ///   Byte 0 (bits 31..24): [version:4][flags_hi:4]
    ///   Byte 1 (bits 23..16): [flags_lo:4][frame_type_hi_nibble:4]
    ///   Byte 2 (bits 15..8):  [frame_type_mid:8]
    ///   Byte 3 (bits 7..0):   [frame_type_lo_nibble:4][pad:4]
    ///
    /// This is messy. For clarity and correctness, let's just pack into a u32 and
    /// shift/mask, then write big-endian.

    pub fn serialize(self: *const FrameHeader, buf: []u8) error{BufferTooSmall}!void {
        if (buf.len < HEADER_SIZE) return error.BufferTooSmall;

        // Bytes 0-3: version(4) | flags(8) | frame_type(16) | pad(4)
        const word0: u32 = (@as(u32, self.version) << 28) |
            (@as(u32, @as(u8, @bitCast(self.flags))) << 20) |
            (@as(u32, @intFromEnum(self.frame_type)) << 4) |
            0; // padding
        mem.writeInt(u32, buf[0..4], word0, .big);

        // Bytes 4-7: frame_length
        mem.writeInt(u32, buf[4..8], self.frame_length, .big);

        // Bytes 8-11: stream_id
        mem.writeInt(u32, buf[8..12], self.stream_id, .big);

        // Bytes 12-15: sequence_number
        mem.writeInt(u32, buf[12..16], self.sequence_number, .big);

        // Bytes 16-31: source_node_id (128 bits, already in network byte order)
        @memcpy(buf[16..32], &self.source_node_id);

        // Bytes 32-47: dest_node_id
        @memcpy(buf[32..48], &self.dest_node_id);

        // Bytes 48-51: priority(4) | qos_class(4) | tensor_dtype(8) | padding(16)
        const word12: u32 = (@as(u32, self.priority) << 28) |
            (@as(u32, @intFromEnum(self.qos_class)) << 24) |
            (@as(u32, @intFromEnum(self.tensor_dtype)) << 16) |
            0; // 16-bit padding
        mem.writeInt(u32, buf[48..52], word12, .big);

        // Bytes 52-55: tensor_alignment(16) | options_length(16)
        const word13: u32 = (@as(u32, self.tensor_alignment) << 16) |
            @as(u32, self.options_length);
        mem.writeInt(u32, buf[52..56], word13, .big);

        // Bytes 56-63: timestamp (64-bit)
        mem.writeInt(u64, buf[56..64], self.timestamp, .big);
    }

    pub fn deserialize(buf: []const u8) error{ BufferTooSmall, InvalidVersion }!FrameHeader {
        if (buf.len < HEADER_SIZE) return error.BufferTooSmall;

        const word0 = mem.readInt(u32, buf[0..4], .big);
        const version: u4 = @truncate(word0 >> 28);
        if (version == 0) return error.InvalidVersion;

        const flags: FrameFlags = @bitCast(@as(u8, @truncate(word0 >> 20)));
        const frame_type: FrameType = @enumFromInt(@as(u16, @truncate(word0 >> 4)));

        const frame_length = mem.readInt(u32, buf[4..8], .big);
        const stream_id = mem.readInt(u32, buf[8..12], .big);
        const sequence_number = mem.readInt(u32, buf[12..16], .big);

        var source_node_id: NodeId = undefined;
        @memcpy(&source_node_id, buf[16..32]);

        var dest_node_id: NodeId = undefined;
        @memcpy(&dest_node_id, buf[32..48]);

        const word12 = mem.readInt(u32, buf[48..52], .big);
        const priority: u4 = @truncate(word12 >> 28);
        const qos_class: QosClass = @enumFromInt(@as(u4, @truncate(word12 >> 24)));
        const tensor_dtype: TensorDtype = @enumFromInt(@as(u8, @truncate(word12 >> 16)));

        const word13 = mem.readInt(u32, buf[52..56], .big);
        const tensor_alignment: u16 = @truncate(word13 >> 16);
        const options_length: u16 = @truncate(word13);

        const timestamp = mem.readInt(u64, buf[56..64], .big);

        return FrameHeader{
            .version = version,
            .flags = flags,
            .frame_type = frame_type,
            .frame_length = frame_length,
            .stream_id = stream_id,
            .sequence_number = sequence_number,
            .source_node_id = source_node_id,
            .dest_node_id = dest_node_id,
            .priority = priority,
            .qos_class = qos_class,
            .tensor_dtype = tensor_dtype,
            .tensor_alignment = tensor_alignment,
            .options_length = options_length,
            .timestamp = timestamp,
        };
    }

    /// Create a zeroed header with sensible defaults.
    pub fn init(frame_type: FrameType) FrameHeader {
        return FrameHeader{
            .version = NEXLINK_VERSION,
            .frame_type = frame_type,
        };
    }
};

// ── Unit tests ──

test "header serialize/deserialize roundtrip" {
    const src_id: NodeId = .{ 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10 };
    const dst_id: NodeId = .{ 0xA0, 0xA1, 0xA2, 0xA3, 0xA4, 0xA5, 0xA6, 0xA7, 0xA8, 0xA9, 0xAA, 0xAB, 0xAC, 0xAD, 0xAE, 0xAF };

    const hdr = FrameHeader{
        .version = 1,
        .flags = .{ .tensor_payload = true, .encrypted = true },
        .frame_type = .tensor_transfer,
        .frame_length = 1024,
        .stream_id = 42,
        .sequence_number = 7,
        .source_node_id = src_id,
        .dest_node_id = dst_id,
        .priority = 15,
        .qos_class = .reliable_ordered,
        .tensor_dtype = .bfloat16,
        .tensor_alignment = 128,
        .options_length = 16,
        .timestamp = 1700000000_000000000,
    };

    var buf: [HEADER_SIZE]u8 = undefined;
    try hdr.serialize(&buf);

    const decoded = try FrameHeader.deserialize(&buf);

    try testing.expectEqual(hdr.version, decoded.version);
    try testing.expectEqual(hdr.flags, decoded.flags);
    try testing.expectEqual(hdr.frame_type, decoded.frame_type);
    try testing.expectEqual(hdr.frame_length, decoded.frame_length);
    try testing.expectEqual(hdr.stream_id, decoded.stream_id);
    try testing.expectEqual(hdr.sequence_number, decoded.sequence_number);
    try testing.expectEqual(hdr.source_node_id, decoded.source_node_id);
    try testing.expectEqual(hdr.dest_node_id, decoded.dest_node_id);
    try testing.expectEqual(hdr.priority, decoded.priority);
    try testing.expectEqual(hdr.qos_class, decoded.qos_class);
    try testing.expectEqual(hdr.tensor_dtype, decoded.tensor_dtype);
    try testing.expectEqual(hdr.tensor_alignment, decoded.tensor_alignment);
    try testing.expectEqual(hdr.options_length, decoded.options_length);
    try testing.expectEqual(hdr.timestamp, decoded.timestamp);
}

test "header buffer too small" {
    var small_buf: [32]u8 = undefined;
    const hdr = FrameHeader.init(.data);
    try testing.expectError(error.BufferTooSmall, hdr.serialize(&small_buf));

    try testing.expectError(error.BufferTooSmall, FrameHeader.deserialize(&small_buf));
}

test "header version zero rejected" {
    var buf: [HEADER_SIZE]u8 = .{0} ** HEADER_SIZE;
    // version = 0 in the top nibble of byte 0
    try testing.expectError(error.InvalidVersion, FrameHeader.deserialize(&buf));
}

test "header init sets correct defaults" {
    const hdr = FrameHeader.init(.heartbeat);
    try testing.expectEqual(NEXLINK_VERSION, hdr.version);
    try testing.expectEqual(FrameType.heartbeat, hdr.frame_type);
    try testing.expectEqual(@as(u32, 0), hdr.frame_length);
}

test "header all frame types roundtrip" {
    const types = [_]FrameType{
        .data,
        .control,
        .heartbeat,
        .route_advertisement,
        .trust_handshake,
        .tensor_transfer,
        .stream_control,
    };
    for (types) |ft| {
        var hdr = FrameHeader.init(ft);
        hdr.frame_length = MIN_FRAME_SIZE;
        var buf: [HEADER_SIZE]u8 = undefined;
        try hdr.serialize(&buf);
        const decoded = try FrameHeader.deserialize(&buf);
        try testing.expectEqual(ft, decoded.frame_type);
    }
}

test "header all flags roundtrip" {
    var hdr = FrameHeader.init(.data);
    hdr.flags = .{
        .more_fragments = true,
        .compressed = true,
        .encrypted = true,
        .tensor_payload = true,
        .priority_express = true,
        .overlay_encap = true,
    };
    var buf: [HEADER_SIZE]u8 = undefined;
    try hdr.serialize(&buf);
    const decoded = try FrameHeader.deserialize(&buf);
    try testing.expect(decoded.flags.more_fragments);
    try testing.expect(decoded.flags.compressed);
    try testing.expect(decoded.flags.encrypted);
    try testing.expect(decoded.flags.tensor_payload);
    try testing.expect(decoded.flags.priority_express);
    try testing.expect(decoded.flags.overlay_encap);
}
