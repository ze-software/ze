# Spec: diag-capture-interface

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/6 |
| Updated | 2026-05-19 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/cmd/show/capture_raw.go` - existing pcap export pattern (writePcapHeader/Packet)
4. `internal/component/cmd/show/ping.go` - raw socket handler pattern
5. `internal/component/l2tp/raw_capture.go` - RawCaptureRing mutex pattern for concurrent access

## Task

Add `show capture interface` command that replaces `tcpdump` on gokrazy appliances. Uses AF_PACKET raw sockets with BPF filters for live packet capture, outputting base64-encoded pcap data. Pure Go, no libpcap/cgo dependency.

Related specs:
- `spec-diag-core` - core diagnostic commands (root privilege check, pcap export patterns)
- `spec-diag-netlink-monitor` - streaming netlink monitor
- `spec-diag-traceroute` - traceroute command

### Third-Party Library Stack

-> Decision: `mdlayher/packet` + `go-pcap/filter` chosen. Lighter, composable, no cgo. gopacket/pcapgo rejected (heavier dependency for no clear benefit given existing pcap.go helpers).

| Library | Purpose | cgo? | Notes |
|---------|---------|------|-------|
| `mdlayher/packet` | AF_PACKET raw socket wrapper | No | Clean API, integrates with Go runtime poller via mdlayher/socket. Linux only. |
| `packetcap/go-pcap/filter` | tcpdump expression -> cBPF compilation | No | Only pure-Go tcpdump-syntax-to-BPF compiler. Last published 2025-12. |
| `golang.org/x/net/bpf` | BPF instruction assembly + SO_ATTACH_FILTER | No | Standard library extension. Assembles BPF programs for kernel attachment. |

### Output Modes

-> Decision: Dual output. `format pcap` for Wireshark/programmatic use, `format text` for CLI/AI readability.

**`format pcap`** (default): base64-encoded pcap file. Pipe to `base64 -d > capture.pcap` then open in Wireshark.

**`format text`**: one line per packet, human/AI-readable. Fields:
```
TIMESTAMP PROTO SRC:PORT -> DST:PORT FLAGS LEN PAYLOAD_HEX
```
Example:
```
14:23:01.003 TCP 10.0.0.1:39812 -> 10.0.0.2:179 [SYN] 74 4500004a...
14:23:01.005 TCP 10.0.0.2:179 -> 10.0.0.1:39812 [SYN,ACK] 74 450000460002...
14:23:01.102 TCP 10.0.0.1:39812 -> 10.0.0.2:179 [PSH,ACK] 137 ffffffffffffffff001d01...
```
For non-TCP/UDP: `PROTO SRC -> DST LEN PAYLOAD_HEX` (no port/flags).
Payload hex truncated to first 64 bytes by default (configurable via snap-len).

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - overall system architecture
  -> Constraint: Components register via init(), discovered through registries
- [ ] `ai/patterns/cli-command.md` - CLI command registration pattern
  -> Constraint: WireMethod format ze-<verb>:<noun>, YANG tree required
- [ ] `internal/component/cmd/show/capture_raw.go` - existing pcap export
  -> Decision: Reuse writePcapHeader/writePcapPacket for pcap file format output
- [ ] `internal/component/l2tp/raw_capture.go` - RawCaptureRing
  -> Decision: Mutex-protected ring buffer for concurrent access (reference pattern)

### RFC Summaries (MUST for protocol work)
- Not protocol work; pcap and AF_PACKET are system APIs, not network protocols.

**Key insights:**
- Handler pattern: `func(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error)`
- Existing pcap: `writePcapHeader(snapLen)` + `writePcapPacket(ts, data)` in pcap.go, base64 in capture_raw.go
- AF_PACKET requires CAP_NET_RAW (same as ping; root enforced by diag-core)
- BPF filter: compile tcpdump expression to cBPF bytecode, attach via SO_ATTACH_FILTER
- Concurrency: limit 1 active capture per interface to avoid resource contention
- Max 8 concurrent SSH sessions; each capture holds an AF_PACKET socket + ring buffer

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/cmd/show/show.go` - RPC registration hub
- [ ] `internal/component/cmd/show/capture_raw.go` - BGP/L2TP raw capture ring, pcap export
- [ ] `internal/component/cmd/show/pcap.go` - writePcapHeader, writePcapPacket functions
- [ ] `internal/component/l2tp/raw_capture.go` - RawCaptureRing with sync.Mutex
- [ ] `internal/component/cmd/show/schema/ze-cli-show-cmd.yang` - YANG CLI tree

**Behavior to preserve:**
- All existing show commands unchanged
- Existing capture_raw.go (BGP/L2TP/BFD ring capture) unchanged
- pcap.go helper functions (reuse, not modify)

**Behavior to change:**
- None. Purely additive: one new command.

## Data Flow (MANDATORY)

### Entry Point
- CLI: user types `show capture interface eth0 protocol tcp port 179 count 10 duration 5s format text` via SSH
- MCP: AI calls tool with same parameters
- Web: POST /api/commands

### Transformation Path
1. CLI/MCP/Web input -> command dispatcher -> `handleCaptureInterface`
2. Handler validates interface exists, parses filter expression
3. Handler compiles filter to cBPF bytecode via `go-pcap/filter` + `x/net/bpf`
4. Handler opens AF_PACKET socket via `mdlayher/packet`, attaches BPF filter
5. Handler reads packets in a loop until count/duration/context limit
6a. If `format pcap`: writes pcap file format (header + packets) to buffer, returns base64
6b. If `format text`: decodes each packet inline, writes one text line per packet
7. Handler returns result in `plugin.Response{Status: StatusDone, Data: result}`

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI -> handler | RPC dispatch via WireMethod | [ ] |
| Handler -> kernel | AF_PACKET socket + BPF filter attachment | [ ] |
| Handler -> pcap | writePcapHeader + writePcapPacket (reuse from pcap.go) for format=pcap | [ ] |
| Handler -> text | packet decoder producing one line per packet for format=text | [ ] |
| Handler -> response | base64 pcap or text lines in plugin.Response JSON | [ ] |

### Integration Points
- `pluginserver.RegisterRPCs()` - command registration (existing)
- `ze-cli-show-cmd.yang` - YANG CLI tree (existing, modified)
- `writePcapHeader/Packet` in pcap.go - pcap file format (reuse)
- Third-party: `mdlayher/packet` for AF_PACKET raw sockets
- Third-party: `packetcap/go-pcap/filter` + `golang.org/x/net/bpf` for BPF compilation

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (reuses pcap.go helpers)
- [ ] Zero-copy preserved where applicable (mdlayher/packet read buffer reuse)

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `show capture interface` | -> | `handleCaptureInterface` | `TestCaptureInterface_Wiring` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `show capture interface eth0 protocol tcp port 179 count 10 duration 5s` | Returns base64-encoded pcap with up to 10 TCP packets on port 179, captured over max 5 seconds |
| AC-2 | `show capture interface eth0 count 5` | Captures 5 packets with no protocol filter (all traffic) |
| AC-3 | `show capture interface eth0 duration 3s` | Captures for 3 seconds regardless of packet count |
| AC-4 | `show capture interface eth0 snap-len 128` | Truncates each packet to 128 bytes |
| AC-5 | Invalid interface name | Returns error "interface not found: <name>" |
| AC-6 | Invalid filter expression | Returns error with filter compilation failure message |
| AC-7 | Concurrent capture on same interface | Second request returns error "capture already active on <iface>" |
| AC-8 | `_other.go` stub | Returns "not available on this platform" |
| AC-9 | Pcap output (`format pcap`, default) | Valid pcap file: correct magic number, correct snap-len, timestamps monotonic |
| AC-10 | `format text` | One line per packet: `TIMESTAMP PROTO SRC:PORT -> DST:PORT FLAGS LEN PAYLOAD_HEX`. TCP/UDP show ports+flags; other protocols show `PROTO SRC -> DST LEN HEX`. Payload hex truncated to snap-len bytes. |
| AC-11 | `format text` with no matching traffic | Returns empty output (no error) |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCompileBPFFilter` | `cmd/show/capture_interface_linux_test.go` | Filter expression compiles to valid BPF bytecode | |
| `TestCapturePcapOutput` | `cmd/show/capture_interface_linux_test.go` | Pcap output has correct header, packet records | |
| `TestCaptureArgsParser` | `cmd/show/capture_interface_linux_test.go` | Argument parsing (interface, protocol, port, count, duration, snap-len, format) | |
| `TestCaptureTextOutput` | `cmd/show/capture_interface_linux_test.go` | Text format: one line per packet, correct fields for TCP/UDP/other | |
| `TestCaptureInterfaceValidation` | `cmd/show/capture_interface_linux_test.go` | Rejects invalid interface names | |
| `TestCaptureConcurrencyGuard` | `cmd/show/capture_interface_linux_test.go` | Second capture on same interface returns error | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| count | 1-10000 | 10000 | 0 | 10001 |
| duration | 1s-60s | 60s | <1s | >60s |
| snap-len | 64-65535 | 65535 | 63 | 65536 |
| port | 0-65535 | 65535 | N/A | 65536 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `show-capture-interface` | `test/plugin/show-capture-interface.ci` | CLI captures packets on loopback, verifies pcap output is valid base64 | |

### Future (if deferring any tests)
- None. All tests must be implemented.

## Files to Modify
- `internal/component/cmd/show/schema/ze-cli-show-cmd.yang` - add capture interface container

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [x] | `internal/component/cmd/show/schema/ze-cli-show-cmd.yang` |
| CLI commands/flags | [x] | New handler file (auto-registered via init()) |
| Editor autocomplete | [x] | YANG-driven (automatic) |
| Functional test for new RPC/API | [x] | `test/plugin/show-capture-interface.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` - add to diagnostics section |
| 2 | Config syntax changed? | [ ] | N/A |
| 3 | CLI command added/changed? | [x] | `docs/guide/command-reference.md` - add capture interface |
| 4 | API/RPC added/changed? | [x] | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | [ ] | N/A |
| 6 | Has a user guide page? | [x] | `docs/guide/production-diagnostics.md` (update relevant categories) |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [ ] | N/A |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [ ] | N/A |
| 12 | Internal architecture changed? | [ ] | N/A |

## Files to Create
- `internal/component/cmd/show/capture_interface_linux.go` - AF_PACKET capture handler
- `internal/component/cmd/show/capture_interface_other.go` - stub
- `internal/component/cmd/show/capture_interface_linux_test.go` - unit tests
- `test/plugin/show-capture-interface.ci` - functional test

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

1. **Phase: Wiring + deps** -- register capture handler, add dependencies, write failing wiring test
   - Tests: `TestCaptureInterface_Wiring`
   - Files: handler skeleton, YANG update, `go get mdlayher/packet`, `go get packetcap/go-pcap/filter`
   - Verify: wiring test fails because handler is a stub; dependencies compile

2. **Phase: BPF filter compilation** -- compile tcpdump expressions to kernel BPF
   - Tests: `TestCompileBPFFilter`
   - Files: filter compilation wrapper in `capture_interface_linux.go`
   - Verify: "tcp port 179" compiles to valid BPF bytecode

3. **Phase: Packet capture + pcap output** -- AF_PACKET read loop with count/duration/snap-len limits
   - Tests: `TestCapturePcapOutput`, `TestCaptureArgsParser`, `TestCaptureInterfaceValidation`
   - Files: `capture_interface_linux.go`
   - Verify: captures packets on loopback, produces valid base64 pcap

4. **Phase: Text output** -- decode packets inline, one line per packet
   - Tests: `TestCaptureTextOutput`
   - Files: `capture_interface_linux.go` (packet decoder + text formatter)
   - Verify: TCP packets show SRC:PORT -> DST:PORT FLAGS; non-TCP shows PROTO SRC -> DST

5. **Phase: Concurrency guard** -- per-interface mutex rejecting concurrent captures
   - Tests: `TestCaptureConcurrencyGuard`
   - Files: `capture_interface_linux.go` (sync.Map or similar for per-interface locks)
   - Verify: second capture on same interface returns error; different interfaces are independent

6. **Phase: Platform stub + functional test**
   - Files: `capture_interface_other.go`, `test/plugin/show-capture-interface.ci`
   - Verify: `make ze-functional-test` passes

7. **Full verification** -- `make ze-verify`

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-11 has implementation with file:line |
| Correctness | Pcap output is valid (magic number, endianness, snap-len, timestamps) |
| Naming | YANG uses kebab-case, WireMethod uses ze-show: prefix |
| Data flow | Handler returns via plugin.Response with base64 pcap, no direct stdout |
| Rule: build-tags | _linux.go + _other.go pair |
| Rule: concurrency | Per-interface guard prevents resource contention |
| Rule: dependency | All third-party libraries are pure Go, no cgo |
| Text output | format=text produces parseable one-line-per-packet output, TCP/UDP/other variants correct |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Capture command registered | `grep WireMethod capture_interface_linux.go` |
| YANG container | `grep "capture-interface\|capture interface" ze-cli-show-cmd.yang` |
| _other.go stub | `ls capture_interface_other.go` |
| Functional test | `ls test/plugin/show-capture-interface.ci` |
| No cgo | `go list -f '{{.CgoFiles}}' ./...` returns no unexpected entries |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Interface name validated against net.Interfaces(), count/duration/snap-len bounded |
| BPF filter injection | Filter expression passed to library compiler, not to shell |
| Resource exhaustion | Count max 10000, duration max 60s, snap-len max 65535, per-interface limit 1 |
| Privilege | Requires CAP_NET_RAW (enforced by root check in diag-core) |
| Memory | AF_PACKET ring buffer size bounded; captured packets written to pcap incrementally |
| Concurrent access | Per-interface mutex prevents socket/buffer contention |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Library API insufficient | Switch to alternative library (gopacket vs mdlayher) |
| BPF filter compilation fails for valid expression | Check library limitations, add workaround or restrict filter syntax |
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
- [ ] AC-1..AC-11 all demonstrated
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
- [ ] Write learned summary to `plan/learned/NNN-diag-capture-interface.md`
- [ ] Summary included in commit
