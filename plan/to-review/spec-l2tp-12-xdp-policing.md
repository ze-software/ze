# Spec: l2tp-12 -- XDP Per-Session Ingress Policing

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | spec-l2tp-8c-shaper |
| Phase | - |
| Updated | 2026-04-23 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `internal/plugins/l2tpshaper/` -- reference plugin (same EventBus pattern)
4. `internal/component/l2tp/events/events.go` -- SessionUp/Down/RateChange events
5. `internal/component/l2tp/kernel_linux.go` -- kernelWorker pattern
6. `internal/component/l2tp/listener.go` -- UDP socket and SocketFD()
7. `internal/component/l2tp/subsystem.go` -- subsystem Start wiring

## Task

Implement XDP-based per-session ingress traffic policing for L2TP. An eBPF
XDP program attaches to operator-specified uplink NIC(s) and applies a
per-session token bucket rate limiter to incoming L2TP data packets (T=0).
Control packets (T=1) and non-L2TP traffic always pass. A Go-side plugin
(`l2tp-policing`) manages the eBPF program lifecycle and BPF map entries,
subscribing to the same EventBus events as the existing l2tp-shaper.

This complements the existing l2tp-shaper plugin (spec-l2tp-8c): the shaper
applies TC qdiscs on pppN interfaces for egress/download shaping; this spec
adds ingress/upload policing at the NIC level before kernel L2TP processing.

### Design Decisions

| Decision | Detail |
|----------|--------|
| XDP on uplink NIC | Attach XDP program to operator-configured physical interface(s). Runs before kernel L2TP `encap_recv` callback. Chosen over TC/BPF (slower, post-sk_buff alloc) and socket BPF (fragile with `encap_recv`). |
| Plugin, not subsystem code | Follows "policy via plugins" design decision from umbrella spec. Same pattern as l2tp-shaper: EventBus subscription, YANG config, independent lifecycle. |
| Shared map + bpf_spin_lock | One BPF hash map entry per session. Token bucket uses `bpf_spin_lock` for cross-CPU safety. Chosen over per-CPU maps (inaccurate rate: effective rate = configured_rate * num_cpus when traffic is CPU-pinned). At 1 Gbps FTTP / 1500-byte frames = ~83k PPS per session, spinlock hold time is ~1.7ms/s: negligible. |
| Map key = (tunnel_id, session_id) | 4-byte key without peer IP. Kernel L2TP module validates source address. Simpler map, no event payload changes. |
| Hierarchical token bucket | Two-level policing per session: an overall line bucket (total upload rate) and a priority sub-bucket (EF/CS6/CS7 cap). Every packet deducts from the line bucket. EF/CS6/CS7 packets additionally deduct from the priority bucket. BE packets only check the line bucket. EF passes only when both buckets have tokens; BE passes when the line bucket has tokens. No separate BE bucket needed: BE is implicitly capped at (line rate minus priority usage). |
| Per-subscriber rates from RADIUS | Rates come from RADIUS Access-Accept and CoA, not static config. `SessionRateChangePayload` extended with `PriorityRate` field. YANG config provides `default-rate` and `default-priority-rate` as fallbacks when RADIUS omits rates. Line rate = `UploadRate` from RADIUS. Priority rate = `PriorityRate` from RADIUS (vendor-specific attribute). |
| Committed eBPF bytecode | bpf2go generates Go bindings from C source at `go generate` time. Generated `.o` files committed to repo (common practice: cilium, Cloudflare). Developers only need clang when modifying the C source. |
| cilium/ebpf dependency | Standard Go eBPF library. Provides bpf2go, program loading, map operations, XDP attach. |
| Map size 4096 | Deployment scale: hundreds to ~1000 sessions per device, 10-100 Gbps uplinks. 4096 max entries provides headroom. |

### Alternatives Considered

| Approach | Why rejected |
|----------|-------------|
| TC/BPF on uplink NIC | Runs after sk_buff allocation. Slower than XDP for high-throughput policing. Ze already uses TC on pppN via l2tp-shaper. |
| Socket-level BPF (SO_ATTACH_BPF) | Fragile interaction with kernel L2TP module's `encap_recv` callback on the same socket. |
| Per-CPU token bucket | Rate accuracy depends on traffic distribution across CPUs. At 1 Gbps per session, a single CPU-pinned flow would see rate/num_cpus instead of the configured rate. |

## Required Reading

### Architecture Docs
- [ ] `ai/patterns/plugin.md` -- plugin file structure
  -> Constraint: register.go with init(), atomic logger, RunXxxPlugin(conn), CLIHandler closure
  -> Constraint: run `make generate` after creating plugin to update all.go
- [ ] `ai/rules/plugin-design.md` -- plugin design rules
  -> Constraint: plugin name hyphen-form (l2tp-policing); YANG required for plugins with config
  -> Constraint: proximity principle: all code in `internal/plugins/l2tppolicing/`
- [ ] `docs/architecture/core-design.md` -- subsystem and plugin patterns
  -> Constraint: plugins discovered via registry; event types registered in events.go
- [ ] `ai/rules/design-context.md` -- design context loading
  -> Constraint: grep ze for existing patterns before proposing new ones
- [ ] `ai/rules/naming.md` -- naming conventions
  -> Constraint: use kernel/standard names; "policing" is the standard term for ingress rate limiting

### RFC Summaries
- [ ] No RFC directly governs XDP policing. Token bucket algorithm is a well-known network primitive (RFC 2697, RFC 2698 for context, not binding).

**Key insights:**
- l2tp-shaper plugin is the structural template: same EventBus events, same YANG pattern, same lifecycle
- XDP program attaches to NIC by ifindex, not to the L2TP socket
- Kernel L2TP module and XDP coexist: XDP runs first (NIC driver level), passes packets to kernel stack where L2TP module intercepts them
- cilium/ebpf bpf2go generates Go type-safe bindings from C at `go generate` time
- Deployments: ~1000 sessions, 10-100 Gbps uplinks, 1 Gbps per FTTP subscriber

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/l2tpshaper/shaper.go` -- TC-based egress shaping plugin
  -> Constraint: subscribes to SessionUp/SessionDown/SessionRateChange via EventBus
  -> Constraint: tracks sessions in sync.Map keyed by (tunnelID, sessionID)
  -> Constraint: uses traffic.GetBackend().Apply() for TC programming
- [ ] `internal/plugins/l2tpshaper/register.go` -- plugin registration
  -> Constraint: ConfigureEventBus callback wires EventBus into singleton
  -> Constraint: ConfigRoots: ["l2tp"], YANG schema from schema/ subpackage
- [ ] `internal/plugins/l2tpshaper/schema/ze-l2tp-shaper-conf.yang` -- YANG config
  -> Constraint: nested under `container l2tp { container shaper { ... } }`
  -> Constraint: uses `zt:rate` type for rate leaves
- [ ] `internal/component/l2tp/events/events.go` -- EventBus event definitions
  -> Constraint: SessionUpPayload has TunnelID, SessionID, Interface (pppN name)
  -> Constraint: SessionDownPayload has TunnelID, SessionID, Username
  -> Constraint: SessionRateChangePayload has TunnelID, SessionID, DownloadRate, UploadRate (bps)
  -> Decision: extend SessionRateChangePayload with PriorityRate (uint64 bps); zero means "not provided by RADIUS, use YANG default"
- [ ] `internal/component/l2tp/listener.go` -- UDP listener
  -> Constraint: binds to netip.AddrPort, SocketFD() returns raw fd for kernel tunnel create
  -> Constraint: XDP does NOT attach here; it attaches to the physical NIC independently
- [ ] `internal/component/l2tp/kernel_linux.go` -- kernel worker pattern
  -> Decision: xdp map management follows same channel-based event dispatch as kernelWorker
- [ ] `internal/component/l2tp/subsystem.go` -- subsystem Start wiring
  -> Decision: plugin runs independently of subsystem; no subsystem code changes needed

**Behavior to preserve:**
- l2tp-shaper TC-based egress shaping is unaffected
- Kernel L2TP module data plane is unaffected (XDP passes all packets through after policing)
- L2TP control messages always reach userspace reactor
- No changes to SessionUp/SessionDown event payloads
- No changes to subsystem.go wiring
- l2tp-shaper continues to work unchanged (PriorityRate field is additive, shaper ignores it)

**Behavior to change:**
- `SessionRateChangePayload` gains a `PriorityRate uint64` field (zero = not provided)
- RADIUS plugin (`l2tpauthradius`) parses a vendor-specific attribute for priority rate and populates PriorityRate in the event; existing rate attributes unchanged

## Data Flow (MANDATORY)

### Entry Point
- L2TP data packets arrive on the uplink NIC as UDP datagrams to port 1701
- Format: Ethernet -> IP -> UDP(dst=1701) -> L2TP header (T=0, tunnel_id, session_id) -> PPP payload

### Transformation Path
1. **NIC driver RX**: packet received by NIC hardware
2. **XDP hook** (NEW): eBPF program runs on raw packet buffer
   - Parse Ethernet/IP/UDP headers; non-UDP or dst port != 1701: XDP_PASS
   - Parse L2TP header; T=1 (control): XDP_PASS
   - Extract tunnel_id, session_id from L2TP header
   - Parse inner PPP protocol field; if IPv4 (0x0021) or IPv6 (0x0057): read inner IP DSCP
   - Look up (tunnel_id, session_id) in BPF hash map; not found: XDP_PASS
   - Acquire bpf_spin_lock; refill both line and priority buckets based on elapsed time
   - Check line bucket (all packets): if line tokens < pkt_len, drop
   - If EF/CS6/CS7: additionally check priority bucket; if prio tokens < pkt_len, drop
   - Deduct from line bucket (always); deduct from priority bucket (EF/CS6/CS7 only)
   - Release lock; XDP_PASS or XDP_DROP accordingly
3. **Kernel network stack**: packet proceeds through normal kernel path (if XDP_PASS)
4. **Kernel L2TP module**: encap_recv callback strips L2TP header, delivers payload to PPP
5. **pppN interface**: IP payload enters kernel routing

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| NIC driver -> XDP | XDP program attached via netlink (cilium/ebpf) | [ ] |
| Go plugin -> BPF maps | cilium/ebpf Map.Put/Map.Delete/Map.Lookup | [ ] |
| EventBus -> Plugin | l2tpevents.SessionUp.Subscribe() callback | [ ] |
| YANG config -> Plugin | OnConfigure callback parses config sections | [ ] |

### Integration Points
- `l2tpevents.SessionUp` -- triggers map entry creation
- `l2tpevents.SessionDown` -- triggers map entry deletion
- `l2tpevents.SessionRateChange` -- triggers map entry rate update
- `registry.Register()` -- plugin registration with YANG config
- `metrics.Registry` -- Prometheus metric registration for XDP counters

### Architectural Verification
- [ ] No bypassed layers (XDP runs before kernel L2TP, does not replace it)
- [ ] No unintended coupling (plugin uses EventBus only, no subsystem imports)
- [ ] No duplicated functionality (ingress policing is new; egress shaping stays in l2tp-shaper)
- [ ] Zero-copy preserved (XDP operates on raw packet buffer, no allocations)

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| YANG config `l2tp { policing { ... } }` | -> | `parsePolicingConfig()` | `TestParsePolicingConfig` |
| `ConfigureEventBus(eb)` | -> | `policingInstance.setEventBus(eb)` | `TestSetEventBus` |
| `SessionUp` event | -> | `onSessionUp()` -> BPF map insert | `TestOnSessionUp_MapInsert` |
| `SessionDown` event | -> | `onSessionDown()` -> BPF map delete | `TestOnSessionDown_MapDelete` |
| `SessionRateChange` event | -> | `onSessionRateChange()` -> BPF map update | `TestOnSessionRateChange_MapUpdate` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Plugin configured with valid interface name | XDP program loaded and attached; `link.XDP` succeeds |
| AC-2 | L2TP BE data packet, line bucket has tokens | XDP_PASS; deduct from line bucket; pass counter incremented |
| AC-3 | L2TP BE data packet, line bucket exhausted | XDP_DROP; drop counter incremented |
| AC-4 | L2TP control packet (T=1) for any tunnel | XDP_PASS always; no map lookup performed |
| AC-5 | Non-L2TP packet (not UDP:1701) | XDP_PASS always; no L2TP parsing |
| AC-5a | L2TP EF data packet, both line and priority buckets have tokens | XDP_PASS; deduct from both buckets |
| AC-5b | L2TP EF data packet, priority bucket exhausted, line bucket has tokens | XDP_DROP; priority cap exceeded even though line has capacity |
| AC-5c | L2TP EF data packet, line bucket exhausted, priority bucket has tokens | XDP_DROP; total line rate exceeded |
| AC-6 | SessionUp event received | BPF map entry created with default line rate + default priority rate from YANG |
| AC-6a | SessionRateChange with UploadRate and PriorityRate from RADIUS | Map entry updated: line bucket = UploadRate, priority bucket = PriorityRate |
| AC-6b | SessionRateChange with UploadRate but PriorityRate = 0 | Line bucket updated; priority bucket keeps YANG default-priority-rate |
| AC-7 | SessionRateChange event (CoA mid-session) | BPF map entry rates updated; existing tokens preserved |
| AC-8 | SessionDown event received | BPF map entry deleted |
| AC-9 | Plugin Stop called | XDP program detached from interface; BPF maps closed |
| AC-10 | YANG config with interface, default-rate, default-priority-rate, optional burst | Config parsed and validated; missing burst defaults to rate_bps/8 bytes; missing default-priority-rate defaults to 0 (no priority bucket until RADIUS provides rate) |
| AC-11 | Interface does not support XDP | Plugin logs error, continues without policing (graceful degradation) |
| AC-12 | Prometheus scrape | Per-session pass/drop packet and byte counters readable via BPF map iteration |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParsePolicingConfig` | `internal/plugins/l2tppolicing/config_test.go` | YANG config parsing: interface, rate, burst, defaults | |
| `TestParsePolicingConfig_MissingInterface` | `internal/plugins/l2tppolicing/config_test.go` | Rejects config without interface leaf | |
| `TestParsePolicingConfig_InvalidRate` | `internal/plugins/l2tppolicing/config_test.go` | Rejects zero or negative rate | |
| `TestSetEventBus` | `internal/plugins/l2tppolicing/policing_test.go` | EventBus subscription wiring (subscribe/unsubscribe) | |
| `TestOnSessionUp_MapInsert` | `internal/plugins/l2tppolicing/policing_test.go` | Session up creates map entry with correct rate/burst | |
| `TestOnSessionDown_MapDelete` | `internal/plugins/l2tppolicing/policing_test.go` | Session down removes map entry | |
| `TestOnSessionRateChange_MapUpdate` | `internal/plugins/l2tppolicing/policing_test.go` | Rate change updates rate/burst, preserves other fields | |
| `TestOnSessionRateChange_UnknownSession` | `internal/plugins/l2tppolicing/policing_test.go` | Rate change for unknown session logs warning, no panic | |
| `TestShowSessions` | `internal/plugins/l2tppolicing/policing_test.go` | CLI show command returns JSON with active sessions | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| default-rate | 1 bps - 100 Gbps | 100000000000 | 0 | N/A (uint64) |
| burst | 1 byte - 2^32-1 bytes | 4294967295 | 0 | N/A (uint32) |
| tunnel_id | 1 - 65535 | 65535 | 0 (reserved) | N/A (uint16) |
| session_id | 1 - 65535 | 65535 | 0 (reserved) | N/A (uint16) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-l2tp-policing-config` | `test/plugin/l2tp-policing-config.ci` | Plugin loads with valid YANG config; show command returns empty sessions | |

### eBPF Program Tests (BPF_PROG_TEST_RUN, Linux 5.10+)
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestXDP_BE_WithinLineRate` | `internal/plugins/l2tppolicing/xdp_test.go` | BE packet, line bucket has tokens: XDP_PASS, line tokens deducted | |
| `TestXDP_BE_OverLineRate` | `internal/plugins/l2tppolicing/xdp_test.go` | BE packet, line bucket exhausted: XDP_DROP | |
| `TestXDP_EF_BothBucketsHaveTokens` | `internal/plugins/l2tppolicing/xdp_test.go` | EF packet, both buckets have tokens: XDP_PASS, both deducted | |
| `TestXDP_EF_PrioBucketExhausted` | `internal/plugins/l2tppolicing/xdp_test.go` | EF packet, priority exhausted but line has tokens: XDP_DROP | |
| `TestXDP_EF_LineBucketExhausted` | `internal/plugins/l2tppolicing/xdp_test.go` | EF packet, line exhausted but priority has tokens: XDP_DROP | |
| `TestXDP_BE_DoesNotConsumePrioBucket` | `internal/plugins/l2tppolicing/xdp_test.go` | BE traffic deducts from line only; priority bucket unchanged | |
| `TestXDP_L2TPControl` | `internal/plugins/l2tppolicing/xdp_test.go` | L2TP control packet (T=1) always returns XDP_PASS | |
| `TestXDP_NonL2TP` | `internal/plugins/l2tppolicing/xdp_test.go` | Non-UDP or non-port-1701 packet returns XDP_PASS | |
| `TestXDP_TruncatedPacket` | `internal/plugins/l2tppolicing/xdp_test.go` | Packet too short for L2TP header returns XDP_PASS (fail-open) | |
| `TestXDP_UnknownSession` | `internal/plugins/l2tppolicing/xdp_test.go` | L2TP data for session not in map returns XDP_PASS | |
| `TestXDP_TokenRefill` | `internal/plugins/l2tppolicing/xdp_test.go` | After time passes, both buckets refill and previously-dropped packets pass | |

These tests use `BPF_PROG_TEST_RUN` (via cilium/ebpf's `prog.Test()`) to feed
crafted raw packets into the loaded XDP program and assert the return action.
They require Linux 5.10+ and CAP_BPF. Guarded by `//go:build linux` and
skipped with `t.Skip` when `BPF_PROG_TEST_RUN` is unavailable.

### Future (if deferring any tests)
- Integration test with real L2TP traffic (requires veth + XDP, kernel 5.8+)

## Files to Modify
- `go.mod` -- add `github.com/cilium/ebpf` dependency
- `go.sum` -- updated by `go mod tidy`
- `internal/component/l2tp/events/events.go` -- add `PriorityRate uint64` field to `SessionRateChangePayload`
- `internal/plugins/l2tpauthradius/` -- parse vendor-specific RADIUS attribute for priority rate; populate `PriorityRate` in `SessionRateChangePayload`

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | [x] | `internal/plugins/l2tppolicing/schema/ze-l2tp-policing-conf.yang` |
| CLI commands/flags | [x] | `l2tp policing show` command via OnExecuteCommand |
| Editor autocomplete | [x] | YANG-driven (automatic) |
| Functional test for new command | [x] | `test/plugin/l2tp-policing-config.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/guide/l2tp.md` -- add policing section |
| 2 | Config syntax changed? | [x] | `docs/guide/l2tp.md` -- policing config example |
| 3 | CLI command added/changed? | [x] | `docs/guide/l2tp.md` -- `l2tp policing show` |
| 4 | API/RPC added/changed? | [ ] | - |
| 5 | Plugin added/changed? | [x] | `docs/guide/plugins.md` -- add l2tp-policing entry |
| 6 | Has a user guide page? | [x] | `docs/guide/l2tp.md` (existing page, add section) |
| 7 | Wire format changed? | [ ] | - |
| 8 | Plugin SDK/protocol changed? | [ ] | - |
| 9 | RFC behavior implemented? | [ ] | - |
| 10 | Test infrastructure changed? | [ ] | - |
| 11 | Affects daemon comparison? | [ ] | - |
| 12 | Internal architecture changed? | [ ] | - |

## Files to Create
- `internal/plugins/l2tppolicing/register.go` -- plugin registration (init, ConfigureEventBus, RunEngine)
- `internal/plugins/l2tppolicing/l2tppolicing.go` -- package doc, logger, Name const
- `internal/plugins/l2tppolicing/policing.go` -- policingPlugin struct, EventBus handlers, BPF map CRUD
- `internal/plugins/l2tppolicing/config.go` -- parsePolicingConfig from YANG sections
- `internal/plugins/l2tppolicing/config_test.go` -- config parsing tests
- `internal/plugins/l2tppolicing/policing_test.go` -- EventBus handler tests
- `internal/plugins/l2tppolicing/xdp_linux.go` -- XDP program loading, attach/detach, map operations (Linux-only)
- `internal/plugins/l2tppolicing/xdp_other.go` -- stub for non-Linux (returns "not supported")
- `internal/plugins/l2tppolicing/bpf/policing.c` -- eBPF XDP program (C source)
- `internal/plugins/l2tppolicing/bpf/policing_bpfel.go` -- bpf2go generated (committed)
- `internal/plugins/l2tppolicing/bpf/policing_bpfel.o` -- bpf2go generated bytecode (committed)
- `internal/plugins/l2tppolicing/bpf/policing_bpfeb.go` -- bpf2go generated (big-endian, committed)
- `internal/plugins/l2tppolicing/bpf/policing_bpfeb.o` -- bpf2go generated bytecode (big-endian, committed)
- `internal/plugins/l2tppolicing/bpf/gen.go` -- `//go:generate` directive for bpf2go
- `internal/plugins/l2tppolicing/schema/ze-l2tp-policing-conf.yang` -- YANG module
- `internal/plugins/l2tppolicing/schema/embed.go` -- `//go:embed` for YANG
- `internal/plugins/l2tppolicing/schema/register.go` -- YANG module registration
- `internal/plugins/l2tppolicing/xdp_test.go` -- BPF_PROG_TEST_RUN tests for XDP program (Linux 5.10+)
- `test/plugin/l2tp-policing-config.ci` -- functional test

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan -- check what exists |
| 3. Implement (TDD) | Implementation phases below |
| 4. /ze-review gate | Review Gate section |
| 5. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 6. Critical review | Critical Review Checklist below |
| 7. Fix issues | Fix every issue from critical review |
| 8. Re-verify | Re-run stage 5 |
| 9. Repeat 6-8 | Max 2 review passes |
| 10. Deliverables review | Deliverables Checklist below |
| 11. Security review | Security Review Checklist below |
| 12. Re-verify | Re-run stage 5 |
| 13. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: YANG + config parsing** -- YANG schema, config parsing, validation
   - Tests: `TestParsePolicingConfig`, `TestParsePolicingConfig_MissingInterface`, `TestParsePolicingConfig_InvalidRate`
   - Files: `schema/`, `config.go`, `config_test.go`, `l2tppolicing.go`
   - Verify: tests fail -> implement -> tests pass

2. **Phase: plugin registration + EventBus** -- register.go, EventBus wiring, session map CRUD
   - Tests: `TestSetEventBus`, `TestOnSessionUp_MapInsert`, `TestOnSessionDown_MapDelete`, `TestOnSessionRateChange_MapUpdate`, `TestOnSessionRateChange_UnknownSession`, `TestShowSessions`
   - Files: `register.go`, `policing.go`, `policing_test.go`
   - Verify: tests fail -> implement -> tests pass

3. **Phase: eBPF C program + XDP tests** -- XDP token bucket program, BPF maps, bpf2go generation, BPF_PROG_TEST_RUN tests
   - Tests: `TestXDP_BE_WithinLineRate`, `TestXDP_BE_OverLineRate`, `TestXDP_EF_BothBucketsHaveTokens`, `TestXDP_EF_PrioBucketExhausted`, `TestXDP_EF_LineBucketExhausted`, `TestXDP_BE_DoesNotConsumePrioBucket`, `TestXDP_L2TPControl`, `TestXDP_NonL2TP`, `TestXDP_TruncatedPacket`, `TestXDP_UnknownSession`, `TestXDP_TokenRefill`
   - Files: `bpf/policing.c`, `bpf/gen.go`, `xdp_test.go`, run `go generate`
   - Verify: generated files compile, `go build` succeeds, XDP tests pass on Linux 5.10+

4. **Phase: XDP attach/detach** -- Linux-specific program loading, NIC attachment, map operations
   - Tests: AC-1 (program loads), AC-9 (program detaches on stop), AC-11 (graceful failure)
   - Files: `xdp_linux.go`, `xdp_other.go`
   - Verify: build tags correct, non-Linux stub compiles

5. **Phase: wire everything** -- connect config -> XDP load -> EventBus -> map CRUD -> metrics
   - Tests: end-to-end plugin lifecycle
   - Files: `register.go` (final wiring), `policing.go` (connect xdp ops to event handlers)
   - Verify: plugin starts, attaches XDP, handles events, detaches on stop

6. **Phase: metrics** -- Prometheus counters from BPF map iteration
   - Tests: AC-12 (counters readable)
   - Files: `policing.go` (metrics registration and collection)
   - Verify: metrics appear in Prometheus scrape

7. **Phase: documentation + functional tests**
   - Tests: `test-l2tp-policing-config`
   - Files: `test/plugin/l2tp-policing-config.ci`, `docs/guide/l2tp.md`, `docs/guide/plugins.md`
   - Verify: `make ze-functional-test` passes

8. **Phase: go.mod + make generate** -- add cilium/ebpf dependency, update all.go
   - Files: `go.mod`, `go.sum`, `internal/component/plugin/all/all.go`
   - Verify: `go mod tidy`, `make generate`, `make ze-lint` all pass

9. **Full verification** -- `make ze-verify`
10. **Complete spec** -- Fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1 through AC-12 has implementation with file:line |
| Correctness | Token bucket refill uses monotonic clock (bpf_ktime_get_ns), not wall clock |
| Correctness | L2TP header parsing handles both short (no L bit) and long (with L bit) headers |
| Correctness | XDP program returns XDP_PASS for malformed/short packets (fail-open) |
| Correctness | EF/CS6/CS7 packets check both line AND priority buckets; BE packets check line only |
| Correctness | Both buckets deducted atomically under same spinlock (no partial deduct on drop) |
| Naming | Plugin name `l2tp-policing`, package `l2tppolicing`, YANG `ze-l2tp-policing-conf` |
| Data flow | EventBus -> policing plugin -> BPF map; no direct subsystem imports |
| Rule: proximity | All code in `internal/plugins/l2tppolicing/`, no scatter |
| Rule: plugin-design | YANG present, register.go has init(), blank import in all.go |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| Plugin compiles on Linux and non-Linux | `GOOS=linux go build ./internal/plugins/l2tppolicing/` and `GOOS=darwin go build ./internal/plugins/l2tppolicing/` |
| YANG schema registered | `grep l2tp-policing internal/component/plugin/all/all.go` |
| eBPF bytecode committed | `ls internal/plugins/l2tppolicing/bpf/policing_bpfel.o` |
| Functional test exists | `ls test/plugin/l2tp-policing-config.ci` |
| Docs updated | `grep -l policing docs/guide/l2tp.md` |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Input validation | YANG config: interface name validated (no path traversal, alphanumeric + dash/dot only) |
| Input validation | Rate and burst values validated as positive, non-zero |
| eBPF safety | XDP program must handle truncated/malformed packets without reading past buffer end (use bounds checks) |
| eBPF safety | Map operations handle ENOENT/EEXIST gracefully (no kernel panic paths) |
| Fail-open | If map lookup fails or packet is too short to parse, XDP_PASS (never silently drop legitimate traffic) |
| Resource cleanup | XDP program detached and maps closed on plugin stop; no leaked kernel resources |
| Privilege | XDP attach requires CAP_BPF + CAP_NET_ADMIN; document in guide |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior -> RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |
| eBPF verifier rejects program | Simplify C code; add explicit bounds checks; reduce loop complexity |
| bpf2go generation fails | Check clang version (>= 11), BTF support, kernel headers |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

### Hierarchical Token Bucket

Two-level policing per session: an overall line bucket and a priority
sub-bucket. Every packet deducts from the line bucket. EF/CS6/CS7 packets
additionally deduct from the priority sub-bucket. BE packets check only
the line bucket. This means:
- Total upload never exceeds line rate (from RADIUS UploadRate)
- Priority traffic is additionally capped at priority rate (from RADIUS
  PriorityRate via vendor-specific attribute)
- BE implicitly gets (line rate minus priority usage), no separate bucket
- EF must pass both buckets; BE must pass line bucket only

Both buckets share a single `bpf_spin_lock` per map entry. Different
sessions never contend. At FTTP rates (1 Gbps, ~83k PPS with 1500-byte
frames), the lock hold time per session is ~1.7ms/s: negligible. Even at
small-packet worst case (~1.5M PPS), the hold time is ~30ms/s per session.
Aggregate throughput at 100 Gbps uplink (~8.3M PPS) is distributed across
~1000 sessions, so per-session contention remains low.

### XDP + Kernel L2TP Coexistence

XDP runs at NIC driver level, before the kernel network stack. The kernel
L2TP module's `encap_recv` callback runs later, after UDP socket delivery.
Both can operate on the same traffic without conflict: XDP polices (pass or
drop), then kernel L2TP handles encap/decap for passed packets. No kernel
module changes required.

### Plugin Independence

The policing plugin has zero coupling to the L2TP subsystem code. It
subscribes to EventBus events (the same ones l2tp-shaper uses) and manages
its own eBPF lifecycle. The subsystem does not know about XDP. This means
the plugin can be independently disabled, and the L2TP data plane works
identically with or without it.

## RFC Documentation

No RFC directly governs XDP policing. The token bucket algorithm draws from
RFC 2697 (srTCM) and RFC 2698 (trTCM) concepts but implements a simpler
single-rate single-bucket variant. L2TP header parsing follows RFC 2661
Section 3 (header format) for T-bit and tunnel/session ID extraction.

## Implementation Summary

### What Was Implemented
- [To be filled after implementation]

### Bugs Found/Fixed
- [To be filled after implementation]

### Documentation Updates
- [To be filled after implementation]

### Deviations from Plan
- [To be filled after implementation]

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- [To be filled]

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-12 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all checks -- no failures)

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] RFC constraint comments added
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

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all checks documented pass in spec
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] Summary included in commit
