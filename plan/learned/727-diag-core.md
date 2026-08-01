# 727 -- diag-core

## Context

Ze on gokrazy appliances has no external Linux tools (ss, dmesg, lsof, dig, nc, pprof). Operators needed built-in diagnostic commands accessible via CLI, MCP, and web. This spec added 9 show commands, a shared procfs package, BFD capture ring support, DNS cache counters, and root privilege enforcement.

## Decisions

- Chose `internal/core/procfs/` as a shared package over inline /proc reading in each handler, because multiple handlers (sockets, fd, memory-map, kernel-log) need /proc access and the hex parsing logic is testable in isolation.
- Chose `_linux.go` / `_other.go` build-split pairs over runtime `GOOS` checks, following the existing privilege/drop pattern. Both files need explicit `//go:build` tags (not just filename suffix) because the `check-existing-patterns` hook only excludes build-tagged files from duplicate detection.
- Chose `net.Resolver` (stdlib) for DNS lookup over the existing `miekg/dns` resolver in `resolve/cmd/`, because the show handler should be self-contained and not depend on the resolver component being initialized.
- Chose a package-level `dnsStatsProvider` callback over importing the resolve package directly, to avoid circular dependencies between `cmd/show` and `resolve/dns`.
- Chose manual singleflight implementation (mutex + channel) over `golang.org/x/sync/singleflight` because only `errgroup` was vendored, and adding a new vendored subpackage was not justified for one use.
- Chose `sync.Mutex.TryLock()` for CPU profile mutex over a channel-based approach, matching the Go 1.18+ API and making the "already in progress" error path clean.
- Chose `BFDRawCaptureProvider` as an interface + `SetBFDRawCaptureProvider()` setter over type assertion on the reactor (like BGP), because BFD is a separate plugin with no reactor context. The `pluginService` in `internal/plugins/bfd/bfd.go` implements the interface, aggregating across all engine loops. The capture ring is on the engine `Loop` (atomic pointer, zero overhead when disabled) with hooks in `handleInbound` and `sendLocked`.
- Chose `skipRootCheck` package var for test bypass over `testing.Testing()` check, because the tests call `Run()` directly and `testing.Testing()` would not be available in the production binary.

## Consequences

- Every new `/proc`-reading handler should use `procfs.ReadFileLines()` rather than inline `os.ReadFile`.
- BFD plugin must call `show.SetBFDRawCaptureProvider()` at startup to enable BFD capture; without it, `capture-raw start bfd` returns "BFD plugin not loaded."
- DNS cache stats are available via `Resolver.CacheStats()`. The `resolve/dns` cache now tracks hits, misses, and evictions. The show handler uses a provider callback to access them.
- The root privilege check in `cmd/ze/hub/main.go` runs before config reading. Tests set `skipRootCheck = true` in `init()`.

## Gotchas

- The `check-existing-patterns.sh` hook rejects build-split files if the `_linux.go` file lacks an explicit `//go:build linux` tag. The hook checks `head -3` for build tags to exclude intentional duplicates.
- The `block-sprintf-alloc.sh` hook blocks all `fmt.Sprintf` in new Go files. IPv4/IPv6 address formatting must use `netip.AddrFrom4`/`AddrFrom16` + `textbuf.Addr()`.
- The `dupl` linter flags start/stop function pairs when they exceed ~30 lines. The `capture_raw.go` start/stop required `nolint:dupl` after adding BFD as a third protocol.
- The `goconst` linter requires extracting string literals used 3+ times into constants, even for mode strings like "summary"/"blocked"/"full".
- `/proc/net/tcp6` stores IPv6 addresses with each 32-bit word in host byte order (little-endian), requiring per-word byte reversal before constructing `netip.Addr`.

## Files

### Created
- `internal/core/procfs/reader.go` -- types, hex parsing, TCP state names
- `internal/core/procfs/reader_linux.go` -- ReadFileLines
- `internal/core/procfs/reader_other.go` -- stub
- `internal/core/procfs/reader_test.go` -- unit tests
- `internal/plugins/diag/cmd/tcp_check.go` -- TCP connectivity check
- `internal/component/cmd/show/goroutines.go` -- goroutine dump with singleflight
- `internal/component/cmd/show/sockets_linux.go` -- socket state
- `internal/component/cmd/show/sockets_other.go` -- stub
- `internal/plugins/host-cmd/cmd/show_kernel_log_linux.go` -- kernel log
- `internal/plugins/host-cmd/cmd/show_kernel_log_other.go` -- stub
- `internal/component/cmd/show/fd_linux.go` -- FD inspection
- `internal/component/cmd/show/fd_other.go` -- stub
- `internal/component/resolve/cmd/show_dns.go` -- DNS lookup and cache stats
- `internal/component/cmd/show/profile.go` -- runtime profiling
- `internal/component/cmd/show/memory_map_linux.go` -- process memory map
- `internal/component/cmd/show/memory_map_other.go` -- stub
- `internal/component/cmd/show/goroutines_test.go` -- goroutine tests
- `internal/component/cmd/show/tcp_check_test.go` -- tcp-check tests
- `internal/component/resolve/cmd/show_dns_test.go` -- DNS wiring tests
- `internal/component/cmd/show/profile_test.go` -- profile tests
- `internal/component/resolve/dns/cache_stats_test.go` -- cache counter tests
- `docs/guide/production-diagnostics.md` -- symptom-based guide
- `test/plugin/show-system-goroutines.ci` -- functional test
- `test/plugin/show-tcp-check.ci` -- functional test
- `test/plugin/show-dns-lookup.ci` -- functional test
- `test/plugin/show-dns-cache.ci` -- functional test
- `test/plugin/show-system-profile.ci` -- functional test
- `test/plugin/show-system-sockets.ci` -- functional test
- `test/plugin/show-system-kernel-log.ci` -- functional test
- `test/plugin/show-system-fd.ci` -- functional test
- `test/plugin/show-system-memory-map.ci` -- functional test

### Created (BFD capture ring)
- `internal/component/bfd/engine/raw_capture.go` -- RawCaptureRing + Loop capture methods

### Modified
- `internal/component/bfd/bfd.go` -- pluginService implements BFDRawCaptureProvider, wires SetBFDRawCaptureProvider at OnStarted
- `internal/component/bfd/engine/engine.go` -- rawCapture atomic pointer on Loop struct
- `internal/component/bfd/engine/loop.go` -- captureRx/captureTx hooks in handleInbound/sendLocked
- `internal/component/cmd/show/yang/ze-cli-show-cmd.yang` -- 9 new containers + dns parent
- `internal/component/cmd/show/show.go` -- added `capBFD` constant
- `internal/plugins/diag/cmd/capture_raw.go` -- BFD capture ring support
- `internal/component/resolve/dns/cache.go` -- hit/miss/eviction counters + Stats()
- `internal/component/resolve/dns/resolver.go` -- CacheStats() method
- `cmd/ze/hub/main.go` -- root privilege check + skipRootCheck for tests
- `cmd/ze/hub/main_test.go` -- init() sets skipRootCheck
- `docs/features.md` -- Core Diagnostics entry
- `docs/guide/command-reference.md` -- 9 new command entries
- `docs/architecture/api/commands.md` -- 9 new RPC entries
