const std = @import("std");

/// Supported platform backends for StrandLink frame I/O.
///
/// - mock:    In-memory loopback ring buffer.  Default.  Works everywhere.
/// - overlay: UDP encapsulation over an existing IP network (port 6477).
/// - xdp:     AF_XDP zero-copy kernel bypass.  Linux only; requires CAP_NET_ADMIN.
/// - dpdk:    DPDK poll-mode driver.  Not yet implemented (placeholder).
const Backend = enum { mock, overlay, xdp, dpdk };

pub fn build(b: *std.Build) void {
    const target = b.standardTargetOptions(.{});
    const optimize = b.standardOptimizeOption(.{});

    // ── Backend selection ──

    const backend = b.option(
        Backend,
        "backend",
        "Platform backend for StrandLink frame I/O (mock | overlay | xdp | dpdk). Default: mock.",
    ) orelse .mock;

    // Reject XDP on non-Linux targets immediately so the error is a friendly
    // build-system message rather than a downstream compile error.
    if (backend == .xdp) {
        const tag = target.result.os.tag;
        if (tag != .linux) {
            std.debug.panic(
                "The XDP backend (-Dbackend=xdp) is only supported on Linux. " ++
                "Current target OS: {s}.",
                .{@tagName(tag)},
            );
        }
    }

    // ── Shared module definition (used by lib, tests, and dependents) ──

    const strandlink_mod = b.createModule(.{
        .root_source_file = b.path("src/root.zig"),
        .target = target,
        .optimize = optimize,
    });

    // Expose the selected backend as a build-time string option so that
    // source files can import it with `@import("options").backend`.
    const build_options = b.addOptions();
    build_options.addOption(Backend, "backend", backend);
    strandlink_mod.addOptions("options", build_options);

    // ── Static library ──

    const lib = b.addLibrary(.{
        .name = "strandlink",
        .root_module = strandlink_mod,
    });
    lib.installHeader(b.path("include/strandlink.h"), "strandlink.h");
    b.installArtifact(lib);

    // ── Unit tests (all source-level tests via root.zig comptime imports) ──

    const unit_test_mod = b.createModule(.{
        .root_source_file = b.path("src/root.zig"),
        .target = target,
        .optimize = optimize,
    });
    unit_test_mod.addOptions("options", build_options);

    const unit_tests = b.addTest(.{ .root_module = unit_test_mod });
    const run_unit_tests = b.addRunArtifact(unit_tests);

    // ── Integration tests ──

    const frame_test_mod = b.createModule(.{
        .root_source_file = b.path("tests/frame_test.zig"),
        .target = target,
        .optimize = optimize,
    });
    frame_test_mod.addImport("strandlink", strandlink_mod);
    const frame_test = b.addTest(.{ .root_module = frame_test_mod });
    const run_frame_test = b.addRunArtifact(frame_test);

    const ring_buffer_test_mod = b.createModule(.{
        .root_source_file = b.path("tests/ring_buffer_test.zig"),
        .target = target,
        .optimize = optimize,
    });
    ring_buffer_test_mod.addImport("strandlink", strandlink_mod);
    const ring_buffer_test = b.addTest(.{ .root_module = ring_buffer_test_mod });
    const run_ring_buffer_test = b.addRunArtifact(ring_buffer_test);

    const overlay_test_mod = b.createModule(.{
        .root_source_file = b.path("tests/overlay_test.zig"),
        .target = target,
        .optimize = optimize,
    });
    overlay_test_mod.addImport("strandlink", strandlink_mod);
    const overlay_test = b.addTest(.{ .root_module = overlay_test_mod });
    const run_overlay_test = b.addRunArtifact(overlay_test);

    // ── Test step ──

    const test_step = b.step("test", "Run all StrandLink tests");
    test_step.dependOn(&run_unit_tests.step);
    test_step.dependOn(&run_frame_test.step);
    test_step.dependOn(&run_ring_buffer_test.step);
    test_step.dependOn(&run_overlay_test.step);

    // ── Backend info step ──
    //
    // `zig build backend-info` prints which backend is compiled in.
    const backend_info = b.step(
        "backend-info",
        "Print the selected StrandLink platform backend",
    );
    const print_backend = b.addSystemCommand(&.{
        "echo",
        b.fmt("StrandLink backend: {s}", .{@tagName(backend)}),
    });
    backend_info.dependOn(&print_backend.step);
}
