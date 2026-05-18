# Spec: diag-core

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 13/14 |
| Updated | 2026-05-18 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/core-design.md` - system architecture
4. `docs/guide/operations.md` - existing operational commands
5. `docs/guide/operational-reports.md` - report bus codes
6. `docs/debugging-tools.md` - existing debug tools
7. `internal/component/cmd/show/ping.go` - reference handler pattern
8. `internal/component/cmd/show/capture_raw.go` - pcap export pattern
9. `internal/component/cmd/show/conntrack.go` - /proc reading pattern

## Task

Core diagnostic CLI commands for ze on gokrazy appliances (no external Linux tools). 9 commands replacing ss, dmesg, lsof, dig, nc, and pprof. Plus a procfs shared package, BFD capture ring support, root privilege enforcement at startup, and a production diagnostics guide. Each command auto-appears in MCP and web via existing auto-generation.

Split from the original `spec-diag-debug-plus-plus`. Related specs:
- `spec-diag-netlink-monitor` - streaming netlink event monitor (ip monitor replacement)
- `spec-diag-traceroute` - pure Go traceroute
- `spec-diag-capture-interface` - AF_PACKET interface capture (tcpdump replacement)

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - overall system architecture
  -> Constraint: Components register via init(), discovered through registries
- [ ] `docs/guide/operations.md` - existing diagnostic commands
  -> Decision: All show commands are RPC-based via pluginserver.RegisterRPCs
- [ ] `docs/guide/operational-reports.md` - report bus API
  -> Constraint: Warnings state-based, errors event-based, ring buffer
- [ ] `docs/debugging-tools.md` - existing debug tools
  -> Constraint: ze.log.* env vars for per-subsystem logging
- [ ] `docs/guide/monitoring.md` - Prometheus metrics
  -> Decision: 138 OS metrics via collectors, BGP metrics per-peer
- [ ] `ai/patterns/cli-command.md` - CLI command registration pattern
  -> Constraint: WireMethod format ze-<verb>:<noun>, YANG tree required

### RFC Summaries (MUST for protocol work)
- Not protocol work; no RFC summaries needed.

**Key insights:**
- Handler pattern: `func(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error)`
- Registration: `pluginserver.RegisterRPCs(RPCRegistration{WireMethod, Handler})` in `init()`
- YANG: container in `ze-cli-show-cmd.yang` with `ze:command` extension
- Linux-only convention: `_linux.go` + `_other.go` pairs (establishing this for cmd/show/; conntrack.go reads /proc inline without build split, which is the old pattern)
- MCP auto-generation: no extra code needed, tools derived from command registry
- Existing pcap export: `writePcapHeader/Packet` in pcap.go, base64 encoding in capture_raw.go
- Existing streaming: `StreamingHandler` + `RegisterStreamingHandler` exist (used by diag-netlink-monitor, not this spec)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/cmd/show/show.go` - main RPC registration hub, 95+ show commands; handler signature: `func(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error)`
- [ ] `internal/component/cmd/show/ping.go` - ICMP ping, reference handler pattern (216 lines)
- [ ] `internal/component/cmd/show/capture_raw.go` - BGP/L2TP raw capture ring, pcap export (214 lines)
- [ ] `internal/component/cmd/show/conntrack.go` - /proc/sys reading with readProcSysctl()
- [ ] `internal/component/cmd/show/system.go` - system memory/cpu/date handlers
- [ ] `internal/component/cmd/show/schema/ze-cli-show-cmd.yang` - YANG CLI tree (96 containers)
- [ ] `internal/component/mcp/tools.go` - MCP tool auto-generation from command registry
- [ ] `internal/component/resolve/dns/cache.go` - DNS cache (3.3K, needs counter additions)

**Behavior to preserve:**
- All existing 95+ show commands unchanged
- JSON output format with `plugin.Response{Status, Data}` pattern
- MCP tool auto-generation from command registry
- YANG-driven CLI tree structure
- Report bus codes and semantics
- DNS cache existing API (Stats() is additive)

**Behavior to change:**
- `resolve/dns/cache.go`: add hit/miss/eviction counters and `Stats()` method (non-additive, modifies hot path)
- `capture_raw.go`: add BFD protocol support to existing ring
- `cmd/ze/hub/main.go`: add root privilege check at startup (refuse to start if not root)

## Data Flow (MANDATORY)

### Entry Point
- CLI: user types `show system sockets` via SSH
- MCP: AI calls `ze_show_system { action: "sockets" }` via JSON-RPC
- Web: POST /api/commands with `{"command": "show system sockets"}`

### Transformation Path
1. CLI/MCP/Web input -> command dispatcher -> handler function
2. Handler reads kernel data source (/proc, /dev/kmsg, /proc/self/fd, runtime.Stack, net.Dial, runtime/pprof)
3. Handler builds `map[string]any` result
4. Handler returns `plugin.Response{Status: StatusDone, Data: result}`
5. Dispatcher serializes to JSON for transport

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI -> handler | RPC dispatch via WireMethod | [ ] |
| Handler -> kernel | /proc read, /dev/kmsg read | [ ] |
| Handler -> runtime | runtime.Stack, runtime/pprof | [ ] |
| Handler -> resolver | resolve/dns cache Stats() | [ ] |
| Handler -> response | plugin.Response JSON | [ ] |

### Integration Points
- `pluginserver.RegisterRPCs()` - command registration (existing)
- `ze-cli-show-cmd.yang` - YANG CLI tree (existing, modified)
- `readProcSysctl()` in conntrack.go - /proc reading reference (not reused; new code uses procfs package)
- `writePcapHeader/Packet` in pcap.go - pcap export (reused for BFD capture ring)
- `resolve/dns/cache.go` - DNS cache (modified: add counters + Stats())
- `RawCaptureRing` in l2tp/raw_capture.go - capture ring pattern (reused for BFD)

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `show system sockets` | -> | `handleShowSystemSockets` | `TestShowSystemSockets_Wiring` |
| `show system kernel-log` | -> | `handleShowSystemKernelLog` | `TestShowSystemKernelLog_Wiring` |
| `show system goroutines` | -> | `handleShowSystemGoroutines` | `TestShowSystemGoroutines_Wiring` |
| `show tcp-check` | -> | `handleTCPCheck` | `TestTCPCheck_Wiring` |
| `show system file-descriptors` | -> | `handleShowSystemFD` | `TestShowSystemFD_Wiring` |
| `show dns lookup` | -> | `handleDNSLookup` | `TestDNSLookup_Wiring` |
| `show dns cache` | -> | `handleDNSCache` | `TestDNSCache_Wiring` |
| `show system profile` | -> | `handleShowSystemProfile` | `TestShowSystemProfile_Wiring` |
| `show system memory-map` | -> | `handleShowSystemMemoryMap` | `TestShowSystemMemoryMap_Wiring` |
| `show capture-raw start bfd` | -> | BFD ring in `capture_raw.go` | `TestCaptureRawBFD_Wiring` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `show system sockets tcp state established port 179` | Returns JSON with all TCP sockets in ESTABLISHED state on port 179, fields: protocol, local-addr, local-port, remote-addr, remote-port, state, tx-queue, rx-queue |
| AC-2 | `show system kernel-log level err count 20` | Returns JSON with up to 20 kernel log messages at level err or higher, fields: level, sequence, timestamp-us, message |
| AC-3 | `show system goroutines summary` | Returns JSON with total goroutine count and breakdown by state |
| AC-4 | `show system goroutines blocked` | Returns only goroutines in waiting states (chan receive, select, semacquire, IO wait) |
| AC-5 | `show system goroutines full` | Returns raw goroutine stacks; concurrent requests deduplicated via singleflight (one 16MB alloc shared across callers) |
| AC-6 | `show tcp-check 10.0.0.2 179 timeout 3s` | Returns JSON with result (connected/refused/timeout), latency-ms |
| AC-7 | `show tcp-check` with `source` arg | Binds to specified source IP before connecting |
| AC-8 | `show system file-descriptors summary` | Returns JSON with FD count by type (socket/pipe/file/anon_inode) and soft/hard limits |
| AC-9 | `show system file-descriptors detail` | Returns full list of all open FDs with targets |
| AC-10 | `show dns lookup example.com type AAAA` | Returns structured JSON with records, TTL, query-time-ms |
| AC-11 | `show dns cache stats` | Returns cache entry count, capacity, hit rate, miss rate, eviction count (counters added to cache.go) |
| AC-12 | `show system profile cpu duration 10s` | Returns base64-encoded pprof CPU profile data |
| AC-13 | `show system profile heap` | Returns base64-encoded pprof heap profile |
| AC-14 | CPU profiling rejects concurrent requests | Second concurrent CPU profile request returns error (sync.Mutex) |
| AC-15 | `show system memory-map` | Returns JSON with VmRSS, VmSize, VmSwap, Threads from /proc/self/status |
| AC-16 | `show capture-raw start bfd` / `show capture-raw dump bfd pcap` | BFD raw capture ring works like existing BGP/L2TP capture |
| AC-17 | All `_other.go` stubs | Return `plugin.StatusError` with "not available on this platform" |
| AC-18 | All 9 new commands appear as MCP tools | `ze_commands` output includes new commands without any MCP-specific code |
| AC-19 | `docs/guide/production-diagnostics.md` exists | Symptom-based guide covering 17 enumerated failure categories |
| AC-20 | ze started as non-root | Logs "ze requires root privileges" and exits with code 1 |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseHexAddr` | `internal/core/procfs/reader_test.go` | Hex IP address parsing for /proc/net/tcp | |
| `TestParseHexPort` | `internal/core/procfs/reader_test.go` | Hex port parsing | |
| `TestTCPStateString` | `internal/core/procfs/reader_test.go` | State int to string mapping | |
| `TestParseProcNetTCP` | `cmd/show/sockets_linux_test.go` | Full /proc/net/tcp line parsing | |
| `TestParseKmsgLine` | `cmd/show/kernel_log_linux_test.go` | /dev/kmsg line format parsing | |
| `TestParseGoroutineStacks` | `cmd/show/goroutines_test.go` | runtime.Stack output parsing, grouping by state | |
| `TestParseProcSelfLimits` | `cmd/show/fd_linux_test.go` | /proc/self/limits parsing for FD limits | |
| `TestCategorizeFDTarget` | `cmd/show/fd_linux_test.go` | socket:/pipe:/file target classification | |
| `TestParseProcSelfStatus` | `cmd/show/memory_map_linux_test.go` | VmRSS/VmSize parsing | |
| `TestDNSCacheStats` | `internal/component/resolve/dns/cache_test.go` | Hit/miss/eviction counters | |
| `TestGoroutineSingleflight` | `cmd/show/goroutines_test.go` | Concurrent full dumps share one allocation | |
| `TestProfileCPUMutex` | `cmd/show/profile_test.go` | Second concurrent CPU profile returns error | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| sockets port | 0-65535 | 65535 | N/A | 65536 |
| kernel-log count | 1-10000 | 10000 | 0 | 10001 |
| kernel-log level | 0-7 | 7 (debug) | N/A | N/A |
| tcp-check port | 1-65535 | 65535 | 0 | 65536 |
| tcp-check timeout | 1s-30s | 30s | <1s | >30s |
| goroutines full buffer | 1-16MB | 16MB | N/A | truncated with warning |
| profile cpu duration | 1s-60s | 60s | <1s | >60s returns error |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `show-system-sockets` | `test/plugin/show-system-sockets.ci` | CLI shows sockets with port filter | |
| `show-system-kernel-log` | `test/plugin/show-system-kernel-log.ci` | CLI shows kernel log with count/level filter | |
| `show-system-goroutines` | `test/plugin/show-system-goroutines.ci` | CLI shows goroutine summary and blocked | |
| `show-tcp-check` | `test/plugin/show-tcp-check.ci` | CLI tcp-check against localhost SSH port | |
| `show-system-fd` | `test/plugin/show-system-fd.ci` | CLI shows FD summary with counts | |
| `show-dns-lookup` | `test/plugin/show-dns-lookup.ci` | CLI resolves localhost | |
| `show-dns-cache` | `test/plugin/show-dns-cache.ci` | CLI shows cache stats after a lookup | |
| `show-system-profile` | `test/plugin/show-system-profile.ci` | CLI shows heap profile (base64 output) | |
| `show-system-memory-map` | `test/plugin/show-system-memory-map.ci` | CLI shows VmRSS/VmSize | |

### Future (if deferring any tests)
- CPU profile functional test with duration: timing-sensitive, unit test + heap profile functional test provide coverage

## Files to Modify
- `internal/component/cmd/show/schema/ze-cli-show-cmd.yang` - add 9 new container entries
- `internal/component/cmd/show/capture_raw.go` - add BFD protocol support
- `internal/component/resolve/dns/cache.go` - add hit/miss/eviction counters and Stats() method
- `cmd/ze/hub/main.go` - add root privilege check at startup

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [x] | `internal/component/cmd/show/schema/ze-cli-show-cmd.yang` |
| CLI commands/flags | [x] | New handler files (auto-registered via init()) |
| Editor autocomplete | [x] | YANG-driven (automatic if YANG updated) |
| Functional test for new RPC/API | [x] | `test/plugin/show-system-*.ci`, `test/plugin/show-tcp-check.ci`, `test/plugin/show-dns-*.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` - add diagnostics section |
| 2 | Config syntax changed? | [ ] | N/A |
| 3 | CLI command added/changed? | [x] | `docs/guide/command-reference.md` - add 9 commands |
| 4 | API/RPC added/changed? | [x] | `docs/architecture/api/commands.md` - add new RPCs |
| 5 | Plugin added/changed? | [ ] | N/A |
| 6 | Has a user guide page? | [x] | `docs/guide/production-diagnostics.md` (new) |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [ ] | N/A |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [ ] | N/A |
| 12 | Internal architecture changed? | [ ] | N/A |

## Files to Create

### Shared infrastructure
- `internal/core/procfs/reader.go` - types + interface (ErrUnsupported, ReadFileLines, ParseHexAddr, ParseHexPort, TCPStateString)
- `internal/core/procfs/reader_linux.go` - Linux implementations
- `internal/core/procfs/reader_other.go` - stubs returning ErrUnsupported
- `internal/core/procfs/reader_test.go` - unit tests for hex parsing

### Commands
- `internal/component/cmd/show/sockets_linux.go` - TCP/UDP socket state from /proc/net/tcp,udp (ss replacement)
- `internal/component/cmd/show/sockets_other.go` - stub
- `internal/component/cmd/show/kernel_log_linux.go` - kernel log reader from /dev/kmsg (dmesg replacement)
- `internal/component/cmd/show/kernel_log_other.go` - stub
- `internal/component/cmd/show/goroutines.go` - goroutine dump via runtime.Stack (cross-platform, singleflight for full mode)
- `internal/component/cmd/show/tcp_check.go` - TCP port test via net.DialTimeout (cross-platform, nc replacement)
- `internal/component/cmd/show/fd_linux.go` - FD inspection via /proc/self/fd + /proc/self/limits (lsof replacement)
- `internal/component/cmd/show/fd_other.go` - stub
- `internal/component/cmd/show/dns.go` - DNS lookup via net.Resolver + cache stats (cross-platform, dig replacement)
- `internal/component/cmd/show/profile.go` - runtime profiling via runtime/pprof (cross-platform, sync.Mutex for CPU)
- `internal/component/cmd/show/memory_map_linux.go` - /proc/self/status VmRSS/VmSize/VmSwap/Threads (pmap replacement)
- `internal/component/cmd/show/memory_map_other.go` - stub

### Functional tests
- `test/plugin/show-system-sockets.ci`
- `test/plugin/show-system-kernel-log.ci`
- `test/plugin/show-system-goroutines.ci`
- `test/plugin/show-tcp-check.ci`
- `test/plugin/show-system-fd.ci`
- `test/plugin/show-dns-lookup.ci`
- `test/plugin/show-dns-cache.ci`
- `test/plugin/show-system-profile.ci`
- `test/plugin/show-system-memory-map.ci`

### Documentation
- `docs/guide/production-diagnostics.md` - symptom-based diagnostic guide

### Production Diagnostics Guide: 17 Failure Categories

| # | Category | Primary Ze Commands |
|---|----------|-------------------|
| 1 | BGP session won't establish | tcp-check, sockets, capture-raw, dns lookup |
| 2 | BGP session flapping | kernel-log, sockets, capture-raw, event monitor |
| 3 | BGP routes not received | show bgp neighbor, show bgp routes, capture-raw |
| 4 | BGP routes not advertised | show bgp neighbor, show bgp advertised |
| 5 | High CPU usage | profile cpu, goroutines summary, system metrics |
| 6 | Memory leak / high memory | profile heap, memory-map, goroutines, system memory |
| 7 | File descriptor exhaustion | file-descriptors summary/detail, sockets |
| 8 | Goroutine leak | goroutines summary/blocked, profile goroutine |
| 9 | Process killed (OOM / signal) | kernel-log, memory-map, show warnings/errors |
| 10 | Interface down / link flap | show interfaces, kernel-log |
| 11 | Kernel route missing | show route, sockets |
| 12 | DNS resolution failure | dns lookup, dns cache stats, sockets |
| 13 | Config commit failure | show errors, show config diff |
| 14 | Plugin crash / restart loop | show errors, show warnings, goroutines, kernel-log |
| 15 | CLI / SSH unresponsive | goroutines blocked, sockets, file-descriptors |
| 16 | Web UI unreachable | tcp-check, sockets, goroutines, show errors |
| 17 | Telemetry / metrics gaps | show telemetry, sockets, profile cpu |

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7. Critical review | Critical Review Checklist below |
| 8. Fix issues | Fix every issue from critical review |
| 9. Re-verify | Re-run stage 6 |
| 10. Repeat 7-9 | Until clean |
| 11. Deliverables review | Deliverables Checklist below |
| 12. Security review | Security Review Checklist below |
| 13. Re-verify | Re-run stage 6 |
| 14. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring** -- register all 9 command entry points + BFD ring, write failing wiring tests
   - Tests: all `*_Wiring` tests from Wiring Test table
   - Files: handler skeletons, YANG update, show.go registration
   - Verify: wiring tests fail because handlers are stubs

2. **Phase: procfs package** -- create `internal/core/procfs/` with hex parsing and file reading
   - Tests: `TestParseHexAddr`, `TestParseHexPort`, `TestTCPStateString`
   - Files: `reader.go`, `reader_linux.go`, `reader_other.go`, `reader_test.go`
   - Verify: unit tests pass

3. **Phase: tcp-check** -- simplest feature, validates the handler pattern end-to-end
   - Tests: `TestTCPCheck_Wiring`
   - Files: `tcp_check.go`, YANG update
   - Verify: `show tcp-check 127.0.0.1 22` returns connected/refused

4. **Phase: goroutines** -- non-destructive goroutine dump with singleflight
   - Tests: `TestParseGoroutineStacks`, `TestGoroutineSingleflight`, `TestShowSystemGoroutines_Wiring`
   - Files: `goroutines.go`, `goroutines_test.go`
   - Verify: `show system goroutines summary` returns count by state
   - Concurrency: `singleflight.Group` for `full` mode (dedup concurrent 16MB allocations)

5. **Phase: sockets** -- TCP/UDP socket state from /proc/net
   - Tests: `TestParseProcNetTCP`, `TestShowSystemSockets_Wiring`
   - Files: `sockets_linux.go`, `sockets_other.go`
   - Verify: `show system sockets tcp port 179` shows BGP connections

6. **Phase: kernel-log** -- read /dev/kmsg
   - Tests: `TestParseKmsgLine`, `TestShowSystemKernelLog_Wiring`
   - Files: `kernel_log_linux.go`, `kernel_log_other.go`
   - Verify: `show system kernel-log count 10` returns kernel messages

7. **Phase: file-descriptors** -- FD inspection from /proc/self/fd
   - Tests: `TestCategorizeFDTarget`, `TestParseProcSelfLimits`, `TestShowSystemFD_Wiring`
   - Files: `fd_linux.go`, `fd_other.go`
   - Verify: `show system file-descriptors summary` returns counts + limits

8. **Phase: dns lookup/cache** -- structured DNS queries + cache stats with counters
   - Tests: `TestDNSLookup_Wiring`, `TestDNSCache_Wiring`, `TestDNSCacheStats`
   - Files: `dns.go`, `resolve/dns/cache.go` modification
   - Verify: `show dns lookup localhost type A` returns structured result; `show dns cache stats` returns hit/miss/eviction counts

9. **Phase: profile** -- runtime profiling via CLI
   - Tests: `TestShowSystemProfile_Wiring`, `TestProfileCPUMutex`
   - Files: `profile.go`, `profile_test.go`
   - Verify: `show system profile heap` returns base64 pprof data
   - Concurrency: `sync.Mutex` prevents concurrent CPU profiles (AC-14)

10. **Phase: memory-map** -- process memory map from /proc/self/status
    - Tests: `TestParseProcSelfStatus`, `TestShowSystemMemoryMap_Wiring`
    - Files: `memory_map_linux.go`, `memory_map_other.go`
    - Verify: `show system memory-map` returns VmRSS/VmSize/VmSwap/Threads

11. **Phase: BFD capture ring** -- add BFD protocol to existing capture_raw
    - Tests: `TestCaptureRawBFD_Wiring`
    - Files: `capture_raw.go` modification
    - Verify: `show capture-raw start bfd` starts BFD ring capture

12. **Phase: Root privilege check** -- refuse startup without root
    - Tests: unit test for privilege check function
    - Files: `cmd/ze/hub/main.go` modification
    - Verify: non-root invocation logs message and exits 1

13. **Phase: Functional tests** -- .ci files for all 9 commands
    - Files: all `.ci` files listed in Functional Tests table
    - Verify: `make ze-functional-test` passes

14. **Phase: Documentation** -- write `docs/guide/production-diagnostics.md`
    - Symptom-based guide, 17 failure categories (enumerated in this spec)
    - Maps every scenario to Ze CLI commands (existing + new)
    - Cross-references existing guides

15. **Full verification** -- `make ze-verify`

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | JSON output keys use kebab-case, values match documented types |
| Naming | YANG containers use kebab-case, WireMethods use ze-show: prefix |
| Data flow | All handlers return via plugin.Response, no direct stdout |
| Rule: no-layering | No duplicate /proc readers (reuse procfs package) |
| Rule: build-tags | Every Linux-only file has matching _other.go stub |
| Rule: concurrency | goroutines full uses singleflight, profile cpu uses mutex |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| 9 show commands registered | `grep -r "ze-show:" internal/component/cmd/show/ \| grep -c WireMethod` delta |
| YANG containers for all 9 | `grep -c "ze:command" ze-cli-show-cmd.yang` delta |
| _other.go stubs for Linux-only features | `ls internal/component/cmd/show/*_other.go` |
| procfs package exists | `ls internal/core/procfs/reader.go` |
| DNS cache counters | `grep -n "Stats()" internal/component/resolve/dns/cache.go` |
| BFD capture ring | `grep "bfd" internal/component/cmd/show/capture_raw.go` |
| Root privilege check | `grep "Getuid" cmd/ze/hub/main.go` |
| Diagnostic guide exists | `ls docs/guide/production-diagnostics.md` |
| MCP tools auto-generated | no MCP-specific code needed (auto-generation) |
| 9 functional tests | `ls test/plugin/show-system-*.ci test/plugin/show-tcp-check.ci test/plugin/show-dns-*.ci` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Port numbers validated 0-65535, timeout durations capped, count limits enforced |
| /proc path injection | All /proc paths hardcoded, never derived from user input |
| Resource exhaustion | CPU profile max 60s + mutex, goroutine dump max 16MB + singleflight |
| Information disclosure | Socket state shows all connections (acceptable: CLI requires SSH auth) |
| Concurrent profiling | CPU profile mutex prevents resource contention |
| Root privilege | Startup check prevents running as non-root |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline; if architectural, DESIGN phase |
| Functional test fails | Check AC; if AC wrong, DESIGN; if AC correct, IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |

### Failed Approaches
| Approach | Why abandoned | Replacement |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |

## Design Insights

## RFC Documentation
Not protocol work. No RFC annotations needed.

## Implementation Summary

### What Was Implemented
- [To be filled during implementation]

### Bugs Found/Fixed
- [To be filled during implementation]

### Documentation Updates
- [To be filled during implementation]

### Deviations from Plan
- [To be filled during implementation]

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |

### Tests from TDD Plan
| Test | Status | Location | Notes |

### Files from Plan
| File | Status | Notes |

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |

### Fixes applied

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-20 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-diag-core.md`
- [ ] Summary included in commit
