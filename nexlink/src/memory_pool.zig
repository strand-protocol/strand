// memory_pool.zig — Fixed-size slab allocator for NexLink frame buffers
//
// Pre-allocates N fixed-size blocks at init time. alloc() pops from the freelist,
// free() pushes back. The freelist is implemented as a lock-free atomic stack
// using compare-and-swap (CAS) on the freelist head pointer (encoded as an index).
//
// This avoids any heap allocation on the hot path — alloc/free are O(1).

const std = @import("std");
const mem = std.mem;
const testing = std.testing;
const Allocator = std.mem.Allocator;

/// A fixed-size slab memory pool with lock-free alloc/free.
pub const MemoryPool = struct {
    /// Backing memory: pool_size * block_size bytes.
    backing: []align(64) u8,

    /// Freelist: each slot holds the index of the next free block (or SENTINEL).
    /// freelist[i] = index of next free block after block i, or SENTINEL if end.
    freelist: []std.atomic.Value(u32),

    /// Head of the freelist (atomic). Points to index of first free block, or SENTINEL.
    head: std.atomic.Value(u32),

    /// Size of each block in bytes.
    block_size: u32,

    /// Total number of blocks in the pool.
    pool_size: u32,

    /// Allocator used for the pool memory (needed for deinit).
    allocator: Allocator,

    /// Sentinel value indicating end of freelist or no free blocks.
    pub const SENTINEL: u32 = 0xFFFFFFFF;

    pub const Error = error{
        OutOfMemory,
        PoolExhausted,
        InvalidPointer,
    };

    /// Initialize a memory pool with `pool_size` blocks, each `block_size` bytes.
    pub fn init(allocator: Allocator, pool_size: u32, block_size: u32) Error!MemoryPool {
        const total_backing: usize = @as(usize, pool_size) * @as(usize, block_size);
        const backing = allocator.alignedAlloc(u8, .@"64", total_backing) catch return error.OutOfMemory;
        @memset(backing, 0);

        // Allocate freelist array
        const fl_slice = allocator.alloc(std.atomic.Value(u32), pool_size) catch {
            allocator.free(backing);
            return error.OutOfMemory;
        };

        // Initialize freelist: each block points to the next
        for (0..pool_size) |i| {
            if (i + 1 < pool_size) {
                fl_slice[i] = std.atomic.Value(u32).init(@intCast(i + 1));
            } else {
                fl_slice[i] = std.atomic.Value(u32).init(SENTINEL);
            }
        }

        return MemoryPool{
            .backing = backing,
            .freelist = fl_slice,
            .head = std.atomic.Value(u32).init(0), // first free block is 0
            .block_size = block_size,
            .pool_size = pool_size,
            .allocator = allocator,
        };
    }

    /// Free the memory pool and all backing memory.
    pub fn deinit(self: *MemoryPool) void {
        self.allocator.free(self.freelist);
        self.allocator.free(self.backing);
        self.* = undefined;
    }

    /// Allocate a block from the pool. Returns a mutable slice of `block_size` bytes.
    /// Returns error.PoolExhausted if no blocks are available.
    ///
    /// Lock-free via atomic CAS on the freelist head.
    pub fn alloc(self: *MemoryPool) Error![]u8 {
        while (true) {
            const current_head = self.head.load(.acquire);
            if (current_head == SENTINEL) return error.PoolExhausted;

            const next = self.freelist[current_head].load(.monotonic);

            // CAS: try to swing head from current_head to next
            if (self.head.cmpxchgWeak(current_head, next, .release, .acquire)) |_| {
                // CAS failed, retry
                continue;
            }

            // Success — return pointer to block
            const offset: usize = @as(usize, current_head) * @as(usize, self.block_size);
            return self.backing[offset .. offset + self.block_size];
        }
    }

    /// Return a block to the pool.
    ///
    /// The slice must have been obtained from a prior `alloc()` call on this pool.
    /// Lock-free via atomic CAS on the freelist head.
    pub fn free(self: *MemoryPool, block: []u8) Error!void {
        // Calculate block index from pointer
        const block_addr = @intFromPtr(block.ptr);
        const base_addr = @intFromPtr(self.backing.ptr);

        if (block_addr < base_addr) return error.InvalidPointer;
        const byte_offset = block_addr - base_addr;
        if (byte_offset % self.block_size != 0) return error.InvalidPointer;
        const idx: u32 = @intCast(byte_offset / self.block_size);
        if (idx >= self.pool_size) return error.InvalidPointer;

        // Push onto freelist via CAS
        while (true) {
            const current_head = self.head.load(.acquire);
            self.freelist[idx].store(current_head, .monotonic);

            if (self.head.cmpxchgWeak(current_head, idx, .release, .acquire)) |_| {
                // CAS failed, retry
                continue;
            }
            return;
        }
    }

    /// Returns the number of currently allocated (in-use) blocks.
    /// Note: this is O(n) and walks the freelist, so use sparingly.
    pub fn allocatedCount(self: *MemoryPool) u32 {
        var free_count: u32 = 0;
        var idx = self.head.load(.acquire);
        while (idx != SENTINEL) {
            free_count += 1;
            if (free_count > self.pool_size) break; // safety: detect corruption
            idx = self.freelist[idx].load(.monotonic);
        }
        return self.pool_size - free_count;
    }
};

// ── Unit tests ──

test "memory_pool init/deinit" {
    var pool = try MemoryPool.init(testing.allocator, 8, 128);
    defer pool.deinit();

    try testing.expectEqual(@as(u32, 8), pool.pool_size);
    try testing.expectEqual(@as(u32, 128), pool.block_size);
    try testing.expectEqual(@as(u32, 0), pool.allocatedCount());
}

test "memory_pool alloc and free" {
    var pool = try MemoryPool.init(testing.allocator, 4, 64);
    defer pool.deinit();

    // Allocate a block
    const block = try pool.alloc();
    try testing.expectEqual(@as(usize, 64), block.len);
    try testing.expectEqual(@as(u32, 1), pool.allocatedCount());

    // Write to it
    @memcpy(block[0..5], "hello");

    // Free it
    try pool.free(block);
    try testing.expectEqual(@as(u32, 0), pool.allocatedCount());
}

test "memory_pool exhaust and recover" {
    var pool = try MemoryPool.init(testing.allocator, 2, 32);
    defer pool.deinit();

    const b1 = try pool.alloc();
    const b2 = try pool.alloc();

    // Pool is now exhausted
    try testing.expectError(error.PoolExhausted, pool.alloc());
    try testing.expectEqual(@as(u32, 2), pool.allocatedCount());

    // Free one block
    try pool.free(b1);
    try testing.expectEqual(@as(u32, 1), pool.allocatedCount());

    // Now we can alloc again
    const b3 = try pool.alloc();
    try testing.expectEqual(@as(u32, 2), pool.allocatedCount());

    // Clean up
    try pool.free(b2);
    try pool.free(b3);
}

test "memory_pool all blocks unique" {
    var pool = try MemoryPool.init(testing.allocator, 4, 16);
    defer pool.deinit();

    var blocks: [4][]u8 = undefined;
    for (&blocks) |*b| {
        b.* = try pool.alloc();
    }

    // Verify all blocks are at different addresses
    for (0..4) |i| {
        for (i + 1..4) |j| {
            try testing.expect(@intFromPtr(blocks[i].ptr) != @intFromPtr(blocks[j].ptr));
        }
    }

    for (&blocks) |*b| {
        try pool.free(b.*);
    }
}

test "memory_pool invalid pointer rejected" {
    var pool = try MemoryPool.init(testing.allocator, 4, 64);
    defer pool.deinit();

    // Attempt to free a random stack buffer
    var fake: [64]u8 = undefined;
    try testing.expectError(error.InvalidPointer, pool.free(&fake));
}
