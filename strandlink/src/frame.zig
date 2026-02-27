// frame.zig — StrandLink Frame encoding/decoding
//
// A StrandLink frame consists of:
//   [Header (64 bytes)] [Options (0-256 bytes)] [Payload (variable)] [CRC-32C (4 bytes)]
//
// The CRC-32C covers the entire frame excluding the CRC field itself.
// All multi-byte fields are big-endian (network byte order).

const std = @import("std");
const mem = std.mem;
const testing = std.testing;
const header_mod = @import("header.zig");
const crc_mod = @import("crc.zig");
const options_mod = @import("options.zig");

pub const FrameHeader = header_mod.FrameHeader;
pub const FrameFlags = header_mod.FrameFlags;
pub const FrameType = header_mod.FrameType;
pub const QosClass = header_mod.QosClass;
pub const TensorDtype = header_mod.TensorDtype;
pub const NodeId = header_mod.NodeId;
pub const HEADER_SIZE = header_mod.HEADER_SIZE;
pub const MAX_OPTIONS_SIZE = header_mod.MAX_OPTIONS_SIZE;
pub const MAX_FRAME_SIZE = header_mod.MAX_FRAME_SIZE;
pub const MIN_FRAME_SIZE = header_mod.MIN_FRAME_SIZE;
pub const STRANDLINK_VERSION = header_mod.STRANDLINK_VERSION;

pub const CRC_SIZE: usize = 4;

pub const OptionIterator = options_mod.OptionIterator;
pub const OptionBuilder = options_mod.OptionBuilder;
pub const OptionType = options_mod.OptionType;

/// Errors that can occur during frame encoding/decoding.
pub const FrameError = error{
    BufferTooSmall,
    InvalidVersion,
    InvalidFrameLength,
    CrcMismatch,
    OptionsTruncated,
    FrameTooLarge,
    OptionsTooLarge,
};

/// A decoded frame — views into the original buffer (zero-copy).
pub const Frame = struct {
    header: FrameHeader,
    options: []const u8,
    payload: []const u8,

    /// Iterate over TLV options in this frame.
    pub fn optionIterator(self: *const Frame) OptionIterator {
        return OptionIterator.init(self.options);
    }
};

/// Encode a frame into `out_buf`.
///
/// Writes: [header (64B)] [options] [payload] [CRC-32C (4B)]
///
/// The header's `frame_length` and `options_length` fields are set automatically.
/// Returns the total number of bytes written.
pub fn encode(
    hdr: *const FrameHeader,
    options_data: []const u8,
    payload: []const u8,
    out_buf: []u8,
) FrameError!usize {
    // Validate options size before any arithmetic to prevent bypassing OptionBuilder's check
    if (options_data.len > MAX_OPTIONS_SIZE) return error.OptionsTooLarge;
    // Overflow-safe length: options_data.len is bounded above (≤256), so only the payload
    // dimension can cause integer overflow on adversarial inputs.
    const partial = HEADER_SIZE + options_data.len + CRC_SIZE;
    const total_len = std.math.add(usize, partial, payload.len) catch return error.FrameTooLarge;
    if (total_len > MAX_FRAME_SIZE) return error.FrameTooLarge;
    if (out_buf.len < total_len) return error.BufferTooSmall;

    // Copy header and patch frame_length + options_length
    var h = hdr.*;
    h.frame_length = @intCast(total_len);
    h.options_length = @intCast(options_data.len);

    // Serialize header
    h.serialize(out_buf[0..HEADER_SIZE]) catch return error.BufferTooSmall;

    // Copy options
    if (options_data.len > 0) {
        @memcpy(out_buf[HEADER_SIZE .. HEADER_SIZE + options_data.len], options_data);
    }

    // Copy payload
    const payload_offset = HEADER_SIZE + options_data.len;
    if (payload.len > 0) {
        @memcpy(out_buf[payload_offset .. payload_offset + payload.len], payload);
    }

    // Compute CRC-32C over everything except the CRC field
    const crc_offset = payload_offset + payload.len;
    const crc_val = crc_mod.compute(out_buf[0..crc_offset]);
    mem.writeInt(u32, out_buf[crc_offset..][0..4], crc_val, .little);

    return total_len;
}

/// Decode a frame from `buf`.
///
/// Validates: version, frame_length consistency, CRC-32C.
/// Returns a Frame with views into the original buffer.
pub fn decode(buf: []const u8) FrameError!Frame {
    if (buf.len < MIN_FRAME_SIZE) return error.BufferTooSmall;

    // Deserialize header
    const hdr = FrameHeader.deserialize(buf[0..HEADER_SIZE]) catch |err| switch (err) {
        error.BufferTooSmall => return error.BufferTooSmall,
        error.InvalidVersion => return error.InvalidVersion,
    };

    // Validate frame_length
    const frame_len: usize = hdr.frame_length;
    if (frame_len < MIN_FRAME_SIZE or frame_len > MAX_FRAME_SIZE) return error.InvalidFrameLength;
    if (frame_len > buf.len) return error.BufferTooSmall;

    // Validate options_length
    const opts_len: usize = hdr.options_length;
    if (HEADER_SIZE + opts_len + CRC_SIZE > frame_len) return error.InvalidFrameLength;

    // Compute and verify CRC
    const crc_offset = frame_len - CRC_SIZE;
    const expected_crc = crc_mod.compute(buf[0..crc_offset]);
    const stored_crc = mem.readInt(u32, buf[crc_offset..][0..4], .little);
    if (expected_crc != stored_crc) return error.CrcMismatch;

    // Extract slices
    const options = buf[HEADER_SIZE .. HEADER_SIZE + opts_len];
    const payload_offset = HEADER_SIZE + opts_len;
    const payload = buf[payload_offset..crc_offset];

    return Frame{
        .header = hdr,
        .options = options,
        .payload = payload,
    };
}

// ── Unit tests ──

test "frame encode/decode roundtrip - no options, no payload" {
    var hdr = FrameHeader.init(.heartbeat);

    var buf: [MIN_FRAME_SIZE]u8 = undefined;
    const written = try encode(&hdr, &.{}, &.{}, &buf);
    try testing.expectEqual(MIN_FRAME_SIZE, written);

    const frame = try decode(&buf);
    try testing.expectEqual(FrameType.heartbeat, frame.header.frame_type);
    try testing.expectEqual(@as(usize, 0), frame.options.len);
    try testing.expectEqual(@as(usize, 0), frame.payload.len);
}

test "frame encode/decode roundtrip - with payload" {
    const payload = "Hello, StrandLink protocol!";
    var hdr = FrameHeader.init(.data);
    hdr.stream_id = 42;
    hdr.sequence_number = 1;
    hdr.priority = 7;

    var buf: [1024]u8 = undefined;
    const written = try encode(&hdr, &.{}, payload, &buf);
    try testing.expectEqual(HEADER_SIZE + payload.len + CRC_SIZE, written);

    const frame = try decode(buf[0..written]);
    try testing.expectEqual(@as(u32, 42), frame.header.stream_id);
    try testing.expectEqual(@as(u32, 1), frame.header.sequence_number);
    try testing.expectEqual(@as(u4, 7), frame.header.priority);
    try testing.expectEqualSlices(u8, payload, frame.payload);
}

test "frame encode/decode roundtrip - with options and payload" {
    var opts_buf: [256]u8 = undefined;
    var builder = OptionBuilder.init(&opts_buf);
    try builder.putHopCount(3);
    try builder.putCompressionAlg(.lz4);
    const opts = builder.slice();

    const payload = [_]u8{ 0xDE, 0xAD, 0xBE, 0xEF } ** 16;
    var hdr = FrameHeader.init(.tensor_transfer);
    hdr.flags = .{ .tensor_payload = true, .compressed = true };
    hdr.tensor_dtype = .float32;
    hdr.tensor_alignment = 64;

    var buf: [1024]u8 = undefined;
    const written = try encode(&hdr, opts, &payload, &buf);

    const frame = try decode(buf[0..written]);
    try testing.expectEqual(FrameType.tensor_transfer, frame.header.frame_type);
    try testing.expect(frame.header.flags.tensor_payload);
    try testing.expect(frame.header.flags.compressed);
    try testing.expectEqual(@as(usize, opts.len), frame.options.len);
    try testing.expectEqualSlices(u8, &payload, frame.payload);

    // Parse options from decoded frame
    var iter = frame.optionIterator();
    const opt1 = (try iter.next()).?;
    try testing.expectEqual(OptionType.hop_count, opt1.option_type);
    try testing.expectEqual(@as(u8, 3), opt1.value[0]);

    const opt2 = (try iter.next()).?;
    try testing.expectEqual(OptionType.compression_alg, opt2.option_type);
}

test "frame decode CRC mismatch" {
    var hdr = FrameHeader.init(.data);
    const payload = "test";

    var buf: [1024]u8 = undefined;
    const written = try encode(&hdr, &.{}, payload, &buf);

    // Corrupt a byte in the payload area
    buf[HEADER_SIZE + 1] ^= 0xFF;
    try testing.expectError(error.CrcMismatch, decode(buf[0..written]));
}

test "frame decode buffer too small" {
    var buf: [10]u8 = .{0} ** 10;
    try testing.expectError(error.BufferTooSmall, decode(&buf));
}

test "frame encode too large" {
    var hdr = FrameHeader.init(.data);
    // Payload that would exceed MAX_FRAME_SIZE
    const big_payload: []const u8 = &(.{0xAA} ** (MAX_FRAME_SIZE));
    var buf: [MAX_FRAME_SIZE + 100]u8 = undefined;
    try testing.expectError(error.FrameTooLarge, encode(&hdr, &.{}, big_payload, &buf));
}

test "frame frame_length field is set correctly" {
    var hdr = FrameHeader.init(.data);
    const payload = "payload";

    var buf: [1024]u8 = undefined;
    const written = try encode(&hdr, &.{}, payload, &buf);

    const frame = try decode(buf[0..written]);
    try testing.expectEqual(@as(u32, @intCast(written)), frame.header.frame_length);
}

test "frame all frame types encode/decode" {
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
        var buf: [256]u8 = undefined;
        const written = try encode(&hdr, &.{}, "x", &buf);
        const frame = try decode(buf[0..written]);
        try testing.expectEqual(ft, frame.header.frame_type);
    }
}

// ── Security tests ──

test "frame encode rejects options_data exceeding MAX_OPTIONS_SIZE" {
    var hdr = FrameHeader.init(.data);
    // Raw options_data bypassing OptionBuilder must be rejected
    const oversized = [_]u8{0} ** (MAX_OPTIONS_SIZE + 1);
    var buf: [MAX_FRAME_SIZE + 100]u8 = undefined;
    try testing.expectError(error.OptionsTooLarge, encode(&hdr, &oversized, &.{}, &buf));
}

test "frame encode accepts options_data exactly at MAX_OPTIONS_SIZE" {
    var hdr = FrameHeader.init(.data);
    const exact = [_]u8{0} ** MAX_OPTIONS_SIZE;
    var buf: [MAX_FRAME_SIZE + 100]u8 = undefined;
    // Must NOT return OptionsTooLarge when at exact limit
    _ = try encode(&hdr, &exact, &.{}, &buf);
}

test "frame decode frame_length field lying about actual data size" {
    var hdr = FrameHeader.init(.data);
    const payload = "security test";
    var buf: [1024]u8 = undefined;
    const written = try encode(&hdr, &.{}, payload, &buf);

    // Corrupt frame_length in the wire buffer to claim a larger size than the
    // buffer actually contains.  frame_length is bytes 4-7 (big-endian u32).
    const fake_len: u32 = @intCast(written + 100);
    std.mem.writeInt(u32, buf[4..8], fake_len, .big);

    // Decoder detects the lie: declared size > supplied buffer → BufferTooSmall.
    // (The CRC check is never reached, which is correct — we refuse to read
    // beyond the supplied buffer regardless of what the header claims.)
    try testing.expectError(error.BufferTooSmall, decode(buf[0..written]));
}

test "frame decode unknown TLV option type is gracefully skipped" {
    // Build a frame with a raw TLV using an undefined option type (0xFF)
    var hdr = FrameHeader.init(.data);
    var opts_buf: [8]u8 = .{ 0xFF, 2, 0xAB, 0xCD, 0, 0, 0, 0 }; // type=0xFF, len=2, value
    var buf: [1024]u8 = undefined;
    const written = try encode(&hdr, opts_buf[0..4], "payload", &buf);
    const f = try decode(buf[0..written]);
    // Frame decodes successfully; iterator encounters unknown type
    var iter = f.optionIterator();
    const opt = try iter.next();
    // Unknown type is returned (decoder does not reject unknown types)
    try testing.expect(opt != null);
}
