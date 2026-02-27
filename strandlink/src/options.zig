// options.zig — TLV option parser/encoder for StrandLink frame options section
//
// Each option follows Type-Length-Value encoding:
//   Type:   1 byte  (option type identifier)
//   Length: 1 byte  (length of value in bytes, NOT including type+length header)
//   Value:  variable (0..254 bytes)
//
// The spec diagram shows Type(8) + Length(8) + Value(var).
// Options section max size: 256 bytes.

const std = @import("std");
const mem = std.mem;
const testing = std.testing;
const header = @import("header.zig");

/// TLV option header size: type(1) + length(1) = 2 bytes
pub const TLV_HEADER_SIZE: usize = 2;

/// Maximum value length in a single TLV option
pub const MAX_VALUE_LEN: usize = 254;

/// Option type identifiers per spec section 3.3
pub const OptionType = enum(u8) {
    fragment_info = 0x01,
    compression_alg = 0x02,
    encryption_tag = 0x03,
    tensor_shape = 0x04,
    trace_id = 0x05,
    hop_count = 0x06,
    semantic_addr = 0x07,
    gpu_hint = 0x08,
    _,
};

/// Compression algorithm identifiers
pub const CompressionAlg = enum(u8) {
    lz4 = 0x01,
    zstd = 0x02,
    snappy = 0x03,
    _,
};

/// A parsed TLV option — a view into the underlying buffer (zero-copy).
pub const Option = struct {
    option_type: OptionType,
    value: []const u8,
};

/// Error set for option parsing
pub const OptionError = error{
    BufferTooSmall,
    InvalidLength,
    OptionsTruncated,
    OptionsTooLarge,
};

/// Iterator over TLV-encoded options in a byte buffer.
pub const OptionIterator = struct {
    data: []const u8,
    pos: usize,

    pub fn init(data: []const u8) OptionIterator {
        return .{ .data = data, .pos = 0 };
    }

    /// Returns the next option, or null if no more options remain.
    pub fn next(self: *OptionIterator) OptionError!?Option {
        if (self.pos >= self.data.len) return null;

        // Need at least 2 bytes for type + length
        if (self.pos + TLV_HEADER_SIZE > self.data.len) return error.OptionsTruncated;

        const opt_type: OptionType = @enumFromInt(self.data[self.pos]);
        const value_len: usize = self.data[self.pos + 1];

        if (self.pos + TLV_HEADER_SIZE + value_len > self.data.len) {
            return error.OptionsTruncated;
        }

        const value = self.data[self.pos + TLV_HEADER_SIZE .. self.pos + TLV_HEADER_SIZE + value_len];
        self.pos += TLV_HEADER_SIZE + value_len;

        return Option{
            .option_type = opt_type,
            .value = value,
        };
    }

    /// Reset the iterator to the beginning.
    pub fn reset(self: *OptionIterator) void {
        self.pos = 0;
    }
};

/// Builder for encoding TLV options into a buffer.
pub const OptionBuilder = struct {
    buf: []u8,
    pos: usize,

    pub fn init(buf: []u8) OptionBuilder {
        return .{ .buf = buf, .pos = 0 };
    }

    /// Append a TLV option. Returns error if buffer space is insufficient.
    pub fn put(self: *OptionBuilder, opt_type: OptionType, value: []const u8) OptionError!void {
        if (value.len > MAX_VALUE_LEN) return error.InvalidLength;
        const needed = TLV_HEADER_SIZE + value.len;
        if (self.pos + needed > self.buf.len) return error.BufferTooSmall;
        if (self.pos + needed > header.MAX_OPTIONS_SIZE) return error.OptionsTooLarge;

        self.buf[self.pos] = @intFromEnum(opt_type);
        self.buf[self.pos + 1] = @intCast(value.len);
        if (value.len > 0) {
            @memcpy(self.buf[self.pos + TLV_HEADER_SIZE .. self.pos + TLV_HEADER_SIZE + value.len], value);
        }
        self.pos += needed;
    }

    /// Convenience: append a FRAGMENT_INFO option (offset:u32 + total_fragments:u16 = 6 bytes).
    pub fn putFragmentInfo(self: *OptionBuilder, offset: u32, total_fragments: u16) OptionError!void {
        var val: [6]u8 = undefined;
        mem.writeInt(u32, val[0..4], offset, .big);
        mem.writeInt(u16, val[4..6], total_fragments, .big);
        return self.put(.fragment_info, &val);
    }

    /// Convenience: append a COMPRESSION_ALG option (1 byte).
    pub fn putCompressionAlg(self: *OptionBuilder, alg: CompressionAlg) OptionError!void {
        const val = [1]u8{@intFromEnum(alg)};
        return self.put(.compression_alg, &val);
    }

    /// Convenience: append a HOP_COUNT option (1 byte).
    pub fn putHopCount(self: *OptionBuilder, count: u8) OptionError!void {
        const val = [1]u8{count};
        return self.put(.hop_count, &val);
    }

    /// Convenience: append a TRACE_ID option (16 bytes).
    pub fn putTraceId(self: *OptionBuilder, trace_id: [16]u8) OptionError!void {
        return self.put(.trace_id, &trace_id);
    }

    /// Convenience: append a TENSOR_SHAPE option.
    /// ndims(1 byte) followed by dims[] (4 bytes each, big-endian).
    pub fn putTensorShape(self: *OptionBuilder, dims: []const u32) OptionError!void {
        if (dims.len > 62) return error.InvalidLength; // 1 + 62*4 = 249 < 254
        var val: [253]u8 = undefined;
        val[0] = @intCast(dims.len);
        for (dims, 0..) |d, i| {
            mem.writeInt(u32, val[1 + i * 4 ..][0..4], d, .big);
        }
        const total_len = 1 + dims.len * 4;
        return self.put(.tensor_shape, val[0..total_len]);
    }

    /// Convenience: append a GPU_HINT option (device_id:u16 + pool_hint:u16 = 4 bytes).
    pub fn putGpuHint(self: *OptionBuilder, device_id: u16, pool_hint: u16) OptionError!void {
        var val: [4]u8 = undefined;
        mem.writeInt(u16, val[0..2], device_id, .big);
        mem.writeInt(u16, val[2..4], pool_hint, .big);
        return self.put(.gpu_hint, &val);
    }

    /// Returns the total number of bytes written so far.
    pub fn len(self: *const OptionBuilder) u16 {
        return @intCast(self.pos);
    }

    /// Returns the encoded options as a slice.
    pub fn slice(self: *const OptionBuilder) []const u8 {
        return self.buf[0..self.pos];
    }
};

/// Parse a FRAGMENT_INFO value (6 bytes) into offset and total_fragments.
pub fn parseFragmentInfo(value: []const u8) error{InvalidLength}!struct { offset: u32, total_fragments: u16 } {
    if (value.len != 6) return error.InvalidLength;
    return .{
        .offset = mem.readInt(u32, value[0..4], .big),
        .total_fragments = mem.readInt(u16, value[4..6], .big),
    };
}

/// Parse a TENSOR_SHAPE value into ndims and a slice of dimensions.
pub fn parseTensorShape(value: []const u8) error{InvalidLength}!struct { ndims: u8, dims_data: []const u8 } {
    if (value.len < 1) return error.InvalidLength;
    const ndims = value[0];
    const expected_len: usize = 1 + @as(usize, ndims) * 4;
    if (value.len != expected_len) return error.InvalidLength;
    return .{
        .ndims = ndims,
        .dims_data = value[1..],
    };
}

/// Read a single dimension from parsed tensor shape dims_data at index i.
pub fn readTensorDim(dims_data: []const u8, index: usize) error{InvalidLength}!u32 {
    const offset = index * 4;
    if (offset + 4 > dims_data.len) return error.InvalidLength;
    return mem.readInt(u32, dims_data[offset..][0..4], .big);
}

// ── Unit tests ──

test "option builder and iterator roundtrip" {
    var buf: [256]u8 = undefined;
    var builder = OptionBuilder.init(&buf);

    try builder.putHopCount(5);
    try builder.putCompressionAlg(.zstd);
    try builder.putTraceId(.{ 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10 });

    const encoded = builder.slice();
    var iter = OptionIterator.init(encoded);

    // Option 1: hop_count
    const opt1 = (try iter.next()).?;
    try testing.expectEqual(OptionType.hop_count, opt1.option_type);
    try testing.expectEqual(@as(usize, 1), opt1.value.len);
    try testing.expectEqual(@as(u8, 5), opt1.value[0]);

    // Option 2: compression_alg
    const opt2 = (try iter.next()).?;
    try testing.expectEqual(OptionType.compression_alg, opt2.option_type);
    try testing.expectEqual(@as(u8, @intFromEnum(CompressionAlg.zstd)), opt2.value[0]);

    // Option 3: trace_id
    const opt3 = (try iter.next()).?;
    try testing.expectEqual(OptionType.trace_id, opt3.option_type);
    try testing.expectEqual(@as(usize, 16), opt3.value.len);

    // No more options
    try testing.expectEqual(@as(?Option, null), try iter.next());
}

test "option fragment_info roundtrip" {
    var buf: [256]u8 = undefined;
    var builder = OptionBuilder.init(&buf);

    try builder.putFragmentInfo(1024, 8);

    var iter = OptionIterator.init(builder.slice());
    const opt = (try iter.next()).?;
    try testing.expectEqual(OptionType.fragment_info, opt.option_type);

    const frag = try parseFragmentInfo(opt.value);
    try testing.expectEqual(@as(u32, 1024), frag.offset);
    try testing.expectEqual(@as(u16, 8), frag.total_fragments);
}

test "option tensor_shape roundtrip" {
    var buf: [256]u8 = undefined;
    var builder = OptionBuilder.init(&buf);

    const dims = [_]u32{ 16, 768, 3072 };
    try builder.putTensorShape(&dims);

    var iter = OptionIterator.init(builder.slice());
    const opt = (try iter.next()).?;
    try testing.expectEqual(OptionType.tensor_shape, opt.option_type);

    const shape = try parseTensorShape(opt.value);
    try testing.expectEqual(@as(u8, 3), shape.ndims);
    try testing.expectEqual(@as(u32, 16), try readTensorDim(shape.dims_data, 0));
    try testing.expectEqual(@as(u32, 768), try readTensorDim(shape.dims_data, 1));
    try testing.expectEqual(@as(u32, 3072), try readTensorDim(shape.dims_data, 2));
}

test "option gpu_hint roundtrip" {
    var buf: [256]u8 = undefined;
    var builder = OptionBuilder.init(&buf);

    try builder.putGpuHint(3, 0x00FF);

    var iter = OptionIterator.init(builder.slice());
    const opt = (try iter.next()).?;
    try testing.expectEqual(OptionType.gpu_hint, opt.option_type);
    try testing.expectEqual(@as(usize, 4), opt.value.len);
    try testing.expectEqual(@as(u16, 3), mem.readInt(u16, opt.value[0..2], .big));
    try testing.expectEqual(@as(u16, 0x00FF), mem.readInt(u16, opt.value[2..4], .big));
}

test "option iterator truncated data" {
    // Only 1 byte — can't read type+length
    const data = [_]u8{0x01};
    var iter = OptionIterator.init(&data);
    try testing.expectError(error.OptionsTruncated, iter.next());
}

test "option iterator truncated value" {
    // Type=0x01, Length=10, but only 2 bytes of value
    const data = [_]u8{ 0x01, 10, 0xAA, 0xBB };
    var iter = OptionIterator.init(&data);
    try testing.expectError(error.OptionsTruncated, iter.next());
}

test "option builder buffer too small" {
    var buf: [3]u8 = undefined;
    var builder = OptionBuilder.init(&buf);

    // HopCount needs 3 bytes (type + len + 1 value byte) — should succeed
    try builder.putHopCount(1);

    // Now buffer is full — next should fail
    try testing.expectError(error.BufferTooSmall, builder.putHopCount(2));
}

test "option empty iteration" {
    const data = [_]u8{};
    var iter = OptionIterator.init(&data);
    try testing.expectEqual(@as(?Option, null), try iter.next());
}

test "option zero-length value" {
    var buf: [256]u8 = undefined;
    var builder = OptionBuilder.init(&buf);

    // A custom option with empty value
    try builder.put(.hop_count, &.{});

    var iter = OptionIterator.init(builder.slice());
    const opt = (try iter.next()).?;
    try testing.expectEqual(OptionType.hop_count, opt.option_type);
    try testing.expectEqual(@as(usize, 0), opt.value.len);
}
