# Built-in Diagnostics: /proc readers and show commands

A gokrazy appliance carries no `ss`, `dmesg`, `lsof`, `dig`, `nc` or `pprof`.
The diagnostics an operator needs are built into ze and reachable from the CLI,
from MCP and from the web. `docs/architecture/diagnostics/production-diagnostics.md`
surveys the whole diagnostic surface. This page covers the `/proc` readers and
the show commands that replace those six tools.

<!-- source: internal/core/procfs/reader.go -- types, hex parsing, TCP state names -->
<!-- source: internal/core/procfs/reader_linux.go -- ReadFileLines -->

## The decisions

**`internal/core/procfs/` is a shared package, not inline `/proc` reading per
handler.** Sockets, fd, memory-map and kernel-log all need `/proc`, and the hex
parsing is testable on its own. Every new `/proc` reading handler uses
`procfs.ReadFileLines()` rather than `os.ReadFile`.

**Platform differences are `_linux.go` and `_other.go` pairs, never a runtime
`GOOS` check.** Both files carry an explicit `//go:build` tag. The filename
suffix alone is not enough: the duplicate-detection hook reads the first three
lines for a build tag and flags the pair without one.
<!-- source: internal/component/cmd/show/sockets_linux.go -- socket state from /proc/net -->
<!-- source: internal/component/cmd/show/sockets_other.go -- non-Linux stub -->
<!-- source: internal/component/cmd/show/fd_linux.go -- FD inspection from /proc/self/fd -->
<!-- source: internal/component/cmd/show/memory_map_linux.go -- memory from /proc/self/status -->
<!-- source: internal/plugins/host-cmd/cmd/show_kernel_log_linux.go -- kernel log from /dev/kmsg -->

**DNS lookup uses the stdlib `net.Resolver`, not the `miekg/dns` resolver the
resolve component holds.** The lookup handler must work when the resolver
component was never initialized. Cache statistics come from
`Resolver.CacheStats()`, which counts hits, misses and evictions.
<!-- source: internal/component/resolve/cmd/show_dns.go -- dnsLookupStdlib, getDNSCacheStats -->

**The concurrent-dump guard for `show system goroutines full` is hand-written
from a mutex, a flag and a channel.** Only `errgroup` is vendored from
`golang.org/x/sync`, and vendoring `singleflight` for one use is not justified.
The guard exists because the full dump allocates 16 MB; concurrent callers wait
and share one result.
<!-- source: internal/component/cmd/show/goroutines.go -- goroutineFullGuard, goroutinesFull -->

**The CPU profile guard is `sync.Mutex.TryLock()`.** It gives a clean
"already in progress" error path with no channel bookkeeping.
<!-- source: internal/component/cmd/show/profile.go -- runtime profiling -->

**BFD raw capture is reached through an interface plus a setter, not a type
assertion on the reactor.** BFD is a separate plugin with no reactor context.
The plugin service implements `BFDRawCaptureProvider` and calls
`show.SetBFDRawCaptureProvider()` at startup, aggregating across every engine
loop. Without that call, `capture-raw start bfd` answers "BFD plugin not
loaded". The ring lives on the engine `Loop` behind an atomic pointer, so it
costs nothing when disabled, with hooks in `handleInbound` and `sendLocked`.
<!-- source: internal/component/bfd/engine/raw_capture.go -- RawCaptureRing and the Loop capture methods -->

**The test bypass for the root privilege check is a package variable, not
`testing.Testing()`.** The tests call `Run()` directly, and `testing.Testing()`
is not available in the production binary.

## Traps

**`/proc/net/tcp6` stores each 32-bit word of an IPv6 address in host byte
order.** Reverse the bytes per word before building a `netip.Addr`.

**`fmt.Sprintf` is blocked in new Go files.** Format an address with
`netip.AddrFrom4` or `AddrFrom16` and `textbuf.Addr()`.

**The TCP connectivity check replaces `nc`.**
<!-- source: internal/plugins/diag/cmd/tcp_check.go -- TCP port connectivity check -->
