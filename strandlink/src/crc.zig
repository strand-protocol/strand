// crc.zig — CRC-32C (Castagnoli) implementation for StrandLink frame integrity
//
// Uses the Castagnoli polynomial 0x1EDC6F41, which is the standard for iSCSI,
// SCTP, and modern storage/networking protocols. Table-based software implementation
// for portability across all targets (freestanding + userspace).

const std = @import("std");
const testing = std.testing;

/// CRC-32C (Castagnoli) polynomial: 0x1EDC6F41
/// Reflected/reversed form used in table: 0x82F63B78
const POLYNOMIAL: u32 = 0x82F63B78;

/// Pre-computed CRC-32C lookup table (256 entries).
/// Generated at comptime from the reflected Castagnoli polynomial.
const crc_table: [256]u32 = blk: {
    @setEvalBranchQuota(10000);
    var table: [256]u32 = undefined;
    for (0..256) |i| {
        var crc: u32 = @intCast(i);
        for (0..8) |_| {
            if (crc & 1 != 0) {
                crc = (crc >> 1) ^ POLYNOMIAL;
            } else {
                crc = crc >> 1;
            }
        }
        table[i] = crc;
    }
    break :blk table;
};

/// Compute CRC-32C over a byte slice.
/// Returns the final CRC value (already XORed with 0xFFFFFFFF).
pub fn compute(data: []const u8) u32 {
    return update(0xFFFFFFFF, data) ^ 0xFFFFFFFF;
}

/// Update a running CRC-32C with additional data.
/// `crc` should be initialized to 0xFFFFFFFF for a new computation.
/// The final result must be XORed with 0xFFFFFFFF after all data is fed.
pub fn update(crc_in: u32, data: []const u8) u32 {
    var crc = crc_in;
    for (data) |byte| {
        const idx: u8 = @truncate(crc ^ byte);
        crc = crc_table[idx] ^ (crc >> 8);
    }
    return crc;
}

/// Verify that a buffer (including a trailing 4-byte CRC) has valid CRC-32C.
/// The trailing 4 bytes must be the CRC in little-endian byte order.
pub fn verify(data: []const u8) bool {
    if (data.len < 4) return false;
    const payload = data[0 .. data.len - 4];
    const expected = std.mem.readInt(u32, data[data.len - 4 ..][0..4], .little);
    return compute(payload) == expected;
}

// ── Unit tests ──

test "crc32c empty input" {
    const result = compute(&.{});
    try testing.expectEqual(@as(u32, 0x00000000), result);
}

test "crc32c known vectors" {
    // Known CRC-32C test vector: "123456789" -> 0xE3069283
    const data = "123456789";
    const result = compute(data);
    try testing.expectEqual(@as(u32, 0xE3069283), result);
}

test "crc32c single byte" {
    // CRC-32C of a single zero byte
    const data = [_]u8{0x00};
    const result = compute(&data);
    // Known: CRC32C(0x00) = 0x527D5351
    try testing.expectEqual(@as(u32, 0x527D5351), result);
}

test "crc32c all ones" {
    const data = [_]u8{ 0xFF, 0xFF, 0xFF, 0xFF };
    const result = compute(&data);
    try testing.expectEqual(@as(u32, 0xFFFFFFFF), result);
}

test "crc32c incremental update matches single-shot" {
    const data = "Hello, StrandLink!";
    const single = compute(data);

    // Split into two parts and compute incrementally
    var crc = update(0xFFFFFFFF, data[0..7]);
    crc = update(crc, data[7..]);
    const incremental = crc ^ 0xFFFFFFFF;

    try testing.expectEqual(single, incremental);
}

test "crc32c verify valid" {
    const payload = "test payload";
    const crc_val = compute(payload);
    var buf: [payload.len + 4]u8 = undefined;
    @memcpy(buf[0..payload.len], payload);
    std.mem.writeInt(u32, buf[payload.len..][0..4], crc_val, .little);
    try testing.expect(verify(&buf));
}

test "crc32c verify invalid" {
    const payload = "test payload";
    var buf: [payload.len + 4]u8 = undefined;
    @memcpy(buf[0..payload.len], payload);
    std.mem.writeInt(u32, buf[payload.len..][0..4], 0xDEADBEEF, .little);
    try testing.expect(!verify(&buf));
}

test "crc32c table correctness spot check" {
    // crc_table[0] = 0 (no bits set, no feedback)
    try testing.expectEqual(@as(u32, 0x00000000), crc_table[0]);
    // crc_table[1] = reflected polynomial (single bit feedback path)
    // For the reflected Castagnoli polynomial 0x82F63B78:
    //   index 1: bit 0 set -> XOR with polynomial -> 0x82F63B78 >> 1 ^ 0x82F63B78
    //   Actually: CRC starts as 1, bit0=1, so (1>>1) ^ POLY = 0x82F63B78
    //   But then 7 more iterations with bit0=0, so just shifts.
    // The actual value is computed; verify it's nonzero and consistent with known vector.
    try testing.expect(crc_table[1] != 0);
    // Cross-check: table must produce correct result for known vector
    try testing.expectEqual(@as(u32, 0xE3069283), compute("123456789"));
}
