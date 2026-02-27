// tests/frame_test.zig — Integration tests for StrandLink frame encode/decode
//
// Tests roundtrip encoding/decoding, CRC validation, option parsing,
// and edge cases across all frame types.

const std = @import("std");
const testing = std.testing;
const strandlink = @import("strandlink");

const FrameHeader = strandlink.FrameHeader;
const FrameType = strandlink.FrameType;
const QosClass = strandlink.QosClass;
const TensorDtype = strandlink.TensorDtype;
const NodeId = strandlink.NodeId;
const OptionBuilder = strandlink.OptionBuilder;
const OptionType = strandlink.OptionType;
const HEADER_SIZE = strandlink.HEADER_SIZE;
const MIN_FRAME_SIZE = strandlink.MIN_FRAME_SIZE;
const MAX_FRAME_SIZE = strandlink.MAX_FRAME_SIZE;
const CRC_SIZE = strandlink.frame.CRC_SIZE;

// ── Roundtrip tests ──

test "roundtrip: minimal frame (no options, no payload)" {
    var hdr = FrameHeader.init(.heartbeat);
    var buf: [MIN_FRAME_SIZE]u8 = undefined;

    const n = try strandlink.encode(&hdr, &.{}, &.{}, &buf);
    try testing.expectEqual(MIN_FRAME_SIZE, n);

    const f = try strandlink.decode(&buf);
    try testing.expectEqual(FrameType.heartbeat, f.header.frame_type);
    try testing.expectEqual(@as(usize, 0), f.payload.len);
    try testing.expectEqual(@as(usize, 0), f.options.len);
}

test "roundtrip: data frame with payload" {
    const payload = "The quick brown fox jumps over the lazy dog.";
    var hdr = FrameHeader.init(.data);
    hdr.stream_id = 0xCAFEBABE;
    hdr.sequence_number = 999;
    hdr.priority = 10;
    hdr.qos_class = .reliable_ordered;

    var buf: [512]u8 = undefined;
    const n = try strandlink.encode(&hdr, &.{}, payload, &buf);

    const f = try strandlink.decode(buf[0..n]);
    try testing.expectEqual(FrameType.data, f.header.frame_type);
    try testing.expectEqual(@as(u32, 0xCAFEBABE), f.header.stream_id);
    try testing.expectEqual(@as(u32, 999), f.header.sequence_number);
    try testing.expectEqual(@as(u4, 10), f.header.priority);
    try testing.expectEqual(QosClass.reliable_ordered, f.header.qos_class);
    try testing.expectEqualSlices(u8, payload, f.payload);
}

test "roundtrip: tensor_transfer with options" {
    const src_id: NodeId = .{ 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16 };
    const dst_id: NodeId = .{ 0xF0, 0xF1, 0xF2, 0xF3, 0xF4, 0xF5, 0xF6, 0xF7, 0xF8, 0xF9, 0xFA, 0xFB, 0xFC, 0xFD, 0xFE, 0xFF };

    var opts_buf: [256]u8 = undefined;
    var builder = OptionBuilder.init(&opts_buf);
    const dims = [_]u32{ 8, 768, 1024 };
    try builder.putTensorShape(&dims);
    try builder.putGpuHint(2, 0x0001);
    try builder.putHopCount(4);

    const payload = [_]u8{0x42} ** 128;

    var hdr = FrameHeader.init(.tensor_transfer);
    hdr.flags = .{ .tensor_payload = true };
    hdr.source_node_id = src_id;
    hdr.dest_node_id = dst_id;
    hdr.tensor_dtype = .bfloat16;
    hdr.tensor_alignment = 512;
    hdr.timestamp = 1700000000_000000000;

    var buf: [1024]u8 = undefined;
    const n = try strandlink.encode(&hdr, builder.slice(), &payload, &buf);

    const f = try strandlink.decode(buf[0..n]);
    try testing.expectEqual(FrameType.tensor_transfer, f.header.frame_type);
    try testing.expect(f.header.flags.tensor_payload);
    try testing.expectEqual(src_id, f.header.source_node_id);
    try testing.expectEqual(dst_id, f.header.dest_node_id);
    try testing.expectEqual(TensorDtype.bfloat16, f.header.tensor_dtype);
    try testing.expectEqual(@as(u16, 512), f.header.tensor_alignment);
    try testing.expectEqualSlices(u8, &payload, f.payload);

    // Parse options
    var iter = f.optionIterator();
    const opt1 = (try iter.next()).?;
    try testing.expectEqual(OptionType.tensor_shape, opt1.option_type);
    const shape = try strandlink.options.parseTensorShape(opt1.value);
    try testing.expectEqual(@as(u8, 3), shape.ndims);
    try testing.expectEqual(@as(u32, 8), try strandlink.options.readTensorDim(shape.dims_data, 0));

    const opt2 = (try iter.next()).?;
    try testing.expectEqual(OptionType.gpu_hint, opt2.option_type);

    const opt3 = (try iter.next()).?;
    try testing.expectEqual(OptionType.hop_count, opt3.option_type);
    try testing.expectEqual(@as(u8, 4), opt3.value[0]);

    try testing.expectEqual(@as(?strandlink.options.Option, null), try iter.next());
}

// ── CRC validation tests ──

test "crc: corrupted header detected" {
    var hdr = FrameHeader.init(.control);
    var buf: [256]u8 = undefined;
    const n = try strandlink.encode(&hdr, &.{}, "payload", &buf);

    // Flip a bit in the source_node_id region (byte 20), which does not
    // affect frame_length or version validation, so CRC check is reached.
    buf[20] ^= 0x01;
    try testing.expectError(error.CrcMismatch, strandlink.decode(buf[0..n]));
}

test "crc: corrupted payload detected" {
    var hdr = FrameHeader.init(.data);
    var buf: [256]u8 = undefined;
    const n = try strandlink.encode(&hdr, &.{}, "important data", &buf);

    // Corrupt a payload byte
    buf[HEADER_SIZE + 3] ^= 0xFF;
    try testing.expectError(error.CrcMismatch, strandlink.decode(buf[0..n]));
}

test "crc: corrupted CRC field detected" {
    var hdr = FrameHeader.init(.data);
    var buf: [256]u8 = undefined;
    const n = try strandlink.encode(&hdr, &.{}, "data", &buf);

    // Corrupt the CRC bytes directly
    buf[n - 1] ^= 0x01;
    try testing.expectError(error.CrcMismatch, strandlink.decode(buf[0..n]));
}

test "crc: standalone crc32c known vector" {
    const result = strandlink.crc.compute("123456789");
    try testing.expectEqual(@as(u32, 0xE3069283), result);
}

// ── Edge cases ──

test "edge: all frame types roundtrip" {
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
        const n = try strandlink.encode(&hdr, &.{}, "x", &buf);
        const f = try strandlink.decode(buf[0..n]);
        try testing.expectEqual(ft, f.header.frame_type);
    }
}

test "edge: all flags set" {
    var hdr = FrameHeader.init(.data);
    hdr.flags = .{
        .more_fragments = true,
        .compressed = true,
        .encrypted = true,
        .tensor_payload = true,
        .priority_express = true,
        .overlay_encap = true,
    };

    var buf: [256]u8 = undefined;
    const n = try strandlink.encode(&hdr, &.{}, "", &buf);
    const f = try strandlink.decode(buf[0..n]);
    try testing.expect(f.header.flags.more_fragments);
    try testing.expect(f.header.flags.compressed);
    try testing.expect(f.header.flags.encrypted);
    try testing.expect(f.header.flags.tensor_payload);
    try testing.expect(f.header.flags.priority_express);
    try testing.expect(f.header.flags.overlay_encap);
}

test "edge: max priority and qos" {
    var hdr = FrameHeader.init(.data);
    hdr.priority = 15;
    hdr.qos_class = .probabilistic;

    var buf: [256]u8 = undefined;
    const n = try strandlink.encode(&hdr, &.{}, "x", &buf);
    const f = try strandlink.decode(buf[0..n]);
    try testing.expectEqual(@as(u4, 15), f.header.priority);
    try testing.expectEqual(QosClass.probabilistic, f.header.qos_class);
}

test "edge: frame_length field matches actual length" {
    var hdr = FrameHeader.init(.data);
    const payload = "variable length payload";

    var buf: [512]u8 = undefined;
    const n = try strandlink.encode(&hdr, &.{}, payload, &buf);

    const f = try strandlink.decode(buf[0..n]);
    try testing.expectEqual(@as(u32, @intCast(n)), f.header.frame_length);
    try testing.expectEqual(@as(usize, HEADER_SIZE + payload.len + CRC_SIZE), n);
}

test "edge: large payload near max" {
    var hdr = FrameHeader.init(.data);
    // Use a payload that approaches the max but leaves room for header + CRC
    const payload_size = MAX_FRAME_SIZE - HEADER_SIZE - CRC_SIZE;
    const payload_buf = try testing.allocator.alloc(u8, payload_size);
    defer testing.allocator.free(payload_buf);
    @memset(payload_buf, 0xAB);

    const out_buf = try testing.allocator.alloc(u8, MAX_FRAME_SIZE);
    defer testing.allocator.free(out_buf);

    const n = try strandlink.encode(&hdr, &.{}, payload_buf, out_buf);
    try testing.expectEqual(MAX_FRAME_SIZE, n);

    const f = try strandlink.decode(out_buf[0..n]);
    try testing.expectEqual(payload_size, f.payload.len);
}
