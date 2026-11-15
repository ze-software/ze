# Spec: dropped-error-6-routing-protocols

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Priority | P1 |
| Release reason | 28 errors in OSPF, IS-IS, LDP and BFD are caught and logged only below WARN: an OSPF IPsec policy that fails to be removed stays in the kernel, an LDP hello that fails to send, a BFD loop that fails to stop, each with no trace at the default level |
| RFC gaps | - |
| Depends | - |
| Phase | Prove |
| Handoff | - |
| Updated | 2026-11-15 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Child of `plan/release/fixes/spec-dropped-error-0-umbrella.md`, which holds the
resolution policy and the tree-wide acceptance. Door: `ai/doors/routing-protocols.md`.

Population: every `error-dropped-below-warn` finding of
`./le arch refuse-by-default report` under the door's base paths
(`internal/plugins/{ospf,isis,ldp,rsvpte}`, `internal/component/{mpls,bfd}`).
At HEAD on 2026-11-02: **28 instances in 13 files** (OSPF 10, BFD 8, LDP 6,
IS-IS 4).

Each instance is resolved by exactly one answer, judged in this order
(umbrella, Key Design Decisions):

| Order | When | Answer |
|-------|------|--------|
| 1 | Going on is wrong: the caller would act on the failure | Return the error, or fix the behavior the drop hid |
| 2 | Going on is right and an operator can act | WARN with what, why and the fix (`ai/rules/user-facing-errors.md`) naming the interface, neighbor or session; on a per-packet path (LDP discovery, BFD echo), count it and WARN once per condition |
| 3 | Going on is right and nobody can act | `// refuse-by-default: guards <Symbol> -- <why>` on the line above |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/routing-protocols.md`
  → Constraint: each protocol plugin can run forked; a WARN goes to its logger
  → Constraint: the BFD client lookup is in-process only and every caller checks for nil; a returned error must not replace that nil check
  → Constraint: a protocol never writes a kernel route; the OSPF IPsec SA and policy installs go through the IKE dataplane seam (`ike/dataplane`)
- [ ] `docs/architecture/bfd.md` - the BFD service and its clients
  → Constraint: a BFD session failure is reported to its client; the client decides what the protocol does
- [ ] `ai/rules/user-facing-errors.md`
  → Constraint: what, why, fix
- [ ] `docs/contributing/ze-go-style.md` - "Error dropped below WARN"
  → Constraint: return, count, or WARN; the annotation names a guard

**Key insights:**
- `ipsecInstaller.removeLocked` drops each failed policy removal: Ze-installed XFRM state that stays, the same ownership concern as the forwarding child.
- The event-sink emits (`eventSink.emit`, `SessionUp`, `SessionDown`) are how other plugins learn adjacency changes; a lost emit is answer 2 at least.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/arch/refusebydefault/dropped.go` - the shape
- [ ] `internal/plugins/ospf/ipsec_install.go` - a failed `RemovePolicyParams` is logged at Debug and the loop goes on

**Behavior to preserve:**
- Protocol behavior toward FRR, BIRD and other peers.

**Behavior to change:**
- Each listed instance resolved by one answer.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| refuse-by-default dropped-error shape | `internal/le/arch/refusebydefault/dropped.go` `droppedError` | Lists these 28 | Applies: the acceptance check |
| Guard annotation | `internal/le/arch/refusebydefault/annotation.go` | Accepts a named guard | Applies: answer 3 only |
| BFD client API | `internal/component/bfd/api/registry.go` | In-process BFD sessions for OSPF, BGP, static | Applies: failures reach the client through it |
| Protocol metrics | Each protocol's Prometheus families (`docs/plugin-development/metrics.md`) | Counters per protocol | Extends: answer 2 on per-packet paths counts here |
| IKE dataplane seam | `internal/component/ike/dataplane` | XFRM install and remove for OSPF RFC 4552 | Applies: the source of the OSPF IPsec removal errors |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A failed call inside a protocol engine or BFD: a packet send, an XFRM removal, an event emit, a loop start or stop.

### Transformation Path
1. The branch catches the error.
2. It returns it, counts it and WARNs once, WARNs, or carries the annotation.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Protocol ↔ kernel | XFRM through `ike/dataplane`, raw sockets | No |
| Protocol ↔ other plugins | EventBus emits | No |
| Protocol ↔ operator | Log at WARN, metrics | No |

### Integration Points
- The protocol loggers, metric families and the BFD client API.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Existing paths |
| No unintended coupling (components stay isolated) | Yes | No new import |
| No duplicated functionality (extends existing, does not recreate) | Yes | No new reporting path |
| Zero-copy preserved where applicable (refs, not copies) | Yes | Per-packet paths count |
| Registration over hardcoding, outbound | N-A | Nothing registers |
| Registration over hardcoding, inbound | N-A | No list learns a name |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The 28 are still the population | Report of 2026-11-02 | Work from the fresh report | Report filtered to this child's paths before the first edit | validated 2026-11-15: still 28, same files and symbols |
| A-2 | A failed XFRM removal at neighbor teardown leaves the policy installed | Netlink delete error | ENOENT is answer 3 | Distinguish "already gone" in each removal | validated: a delete that fails for any reason but absence leaves the Ze-installed entry in the kernel while the record is forgotten (answer 2); ESRCH from a state delete and ENOENT from a policy delete mean already gone (answer 3, `ipsecEntryAbsent`) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A WARN per LDP hello or BFD echo floods the log | A `test/ldp` or `test/bfd` log grows per packet | Count per packet, WARN once per interface and condition |
| R-2 | A returned error from a BFD loop stop changes what the client does at shutdown | A BFD `.ci` reports a failure at clean stop | Answer 1 only where the caller acts correctly |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Log noise, or a protocol reaction to a newly returned error |
| How is it reverted? | Single commit revert |
| Who else touches this path? | Specs on OSPF, IS-IS, LDP and BFD |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le arch refuse-by-default report` | → | `droppedError` over this child's paths | Report filtered to this child's paths: zero findings of the shape |
| An OSPF neighbor's IPsec policy that fails to be removed | → | `ipsecInstaller.removeLocked` | `TestOSPFIPsecRemovalFailureIsReported` |
| An LDP hello that fails to send | → | `sendHello` | `TestLDPHelloSendFailureIsWarned` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `./le arch refuse-by-default report '|' json`, filtered to this child's paths | No `findings` entry with rule `error-dropped-below-warn` |
| AC-2 | Each answer-1 instance | A unit test where the caller observes the error |
| AC-3 | Each file with an answer-2 WARN | A unit test asserts one WARN line names what, why and the fix |
| AC-4 | Each annotation | Names a guard the file can see |
| AC-5 | Package tests and the `test/ospf`, `test/isis`, `test/ldp`, `test/bfd` `.ci` the change reaches | Green |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestOSPFIPsecRemovalFailureIsReported` | `internal/plugins/ospf/ipsec_install_test.go` | AC-2 or AC-3, by the answer chosen | |
| `TestLDPHelloSendFailureIsWarned` | `internal/plugins/ldp/register_test.go` | AC-3 | |
| One test per other answer-1 instance | The package of the instance | AC-2 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| unannotated findings under this child's paths | 0 | 0 | N/A | 1 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `.ci` the change reaches under `test/ospf`, `test/isis`, `test/ldp`, `test/bfd` | `test/` | Unchanged protocol behavior | |

## Files to Modify
- `internal/component/bfd/bfd.go` - 2: `runtimeState.loopFor` 141,145
- `internal/component/bfd/engine/auth_join.go` - 1: `replaceAuth` 51
- `internal/component/bfd/engine/echo.go` - 1: `Loop.reflectEchoLocked` 216
- `internal/component/bfd/engine/engine.go` - 3: `Loop.Start` 504; `Loop.Stop` 535; `Loop.removeEntryLocked` 808
- `internal/component/bfd/engine/loop.go` - 1: `Loop.handleInbound` 175
- `internal/plugins/isis/circuits.go` - 1: `engine.launchCircuitGoroutine.func` 81
- `internal/plugins/isis/events.go` - 2: `eventSink.SessionUp` 91; `eventSink.SessionDown` 101
- `internal/plugins/isis/lsdb_wiring.go` - 1: `engine.publishLSPChange` 784
- `internal/plugins/ldp/register.go` - 6: `discoverOnInterface.func.func` 712; `discoverOnInterface` 727; `processDiscoveryPacket` 825,855; `sendHello` 883; `runSession` 1043
- `internal/plugins/ospf/bfd_client.go` - 2: `engine.detachBFDClient` 347; `engine.stopBFDSession` 374
- `internal/plugins/ospf/events.go` - 3: `eventSink.emitNeighbor` 111; `eventSink.lastKeyExpired` 125; `eventSink.emit` 134
- `internal/plugins/ospf/ipsec_install.go` - 4: `ipsecInstaller.removeNeighborSALocked` 499; `ipsecInstaller.removeLocked` 511,517,523
- `internal/plugins/ospf/transport/transport.go` - 1: `Transport.addrWorker` 358

Lines are those of HEAD on 2026-11-02; the report is the current list.

## Files to Create
- `internal/component/plugin/issuequeue/queue.go`: the one non-blocking report queue every plugin here raises through (one worker per plugin, bounded depth, one total drain deadline at stop, the retry record of a refused raise).
- `internal/plugins/ldp/problems.go`: LDP's codes, hello-discard log and hello-send health (`register.go` was already past 1000 lines).
- `internal/plugins/ospf/issues.go`: OSPF's queue and its refused-report callbacks.
- `internal/component/bfd/engine/problems.go`: the auth-failure, send-failure and sequence-unsaved reports and their quiet expiry.
- Tests: `internal/plugins/ldp/problems_test.go`, `internal/plugins/ospf/{issues_test.go,issues_refused_test.go,events_emit_test.go,bfd_release_refused_test.go}`, `internal/plugins/isis/{events_refused_warn_test.go,hello_send_warn_test.go}`, `internal/component/bfd/engine/failure_report_test.go` and the BFD queue and persist tests.
- `internal/le/interoplab/bgp/testdata/ldp-backoff-clear-frr/ldp-backoff-clear-frr-X3537EPPM42IHDWQ4R4Z5JDI2J.jsonl`: the re-recorded lab run, replacing the 6PHGHPC5 recording.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No config |
| YANG validation constraints | N-A | No leaf |
| YANG custom validators | N-A | No leaf |
| CLI commands/flags | N-A | No command |
| CLI grammar (keyword before value) | N-A | No command |
| Editor autocomplete | N-A | No command |
| Functional test for new RPC/API | N-A | No RPC |
| Pipe completeness | N-A | No output change |
| Env var registration | N-A | No env var |
| Doctor check for runtime dependencies | N-A | No dependency |
| Prometheus counters/metrics | Yes | New: `ze_bfd_send_failures_total{mode,packet}`, `ze_isis_lsp_change_events_refused_total`, `ze_ldp_hello_discards_total{interface}`, `ze_ldp_hello_send_errors_total{interface}`. Existing, now also counting: `ze_bfd_auth_failures_total` |
| BGP family surface (new SAFI / capability / attribute) | N-A | None |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md` (IS-IS and LDP rows, LDP in its own block), `docs/guide/bfd.md` metrics table |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Declared `// Design:` docs: `docs/architecture/bfd.md`, `docs/architecture/isis/isis-4-component-config.md`, `docs/architecture/isis/isis-5-adjacency.md`, `docs/architecture/isis/isis-6-lsdb.md`, `docs/architecture/ldp/mpls-ldp.md`, `docs/architecture/ospf/bfd-client.md`, `docs/architecture/ospf/ospf-3-ip-transport.md`, `docs/architecture/ospf/ospf-4-component-config.md`, `docs/architecture/ospf/ospf-ext-16-ipsec-auth.md`, `docs/research/bfd-implementation-guide.md`, `rfc/short/rfc5880.md`. A log level changes none of them; confirm with `./le spec citation anchors spec plan/release/fixes/spec-dropped-error-6-routing-protocols.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | No | |

## Implementation Steps

1. **Phase: Wiring** -- run the report filtered to this child's paths; write the OSPF and LDP tests red.
2. **Phase: Classify and resolve** -- every instance gets its answer and reason in Design Insights; tests first for answers 1 and 2; annotations last.
3. **Phase: Prove** -- AC-1 to AC-5.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every instance has its answer at file:line |
| Correctness | No per-packet WARN; no returned error replaces the BFD nil check |
| Rule: user-facing-errors | Each WARN names the interface, neighbor or session, why and the fix |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Zero findings under this child's paths | `./le arch refuse-by-default report '|' json` filtered by path |
| Package tests | `./le test unit plugins` limited to the changed packages, and `./le job run label bfd command go test ./internal/component/bfd/...` |
| Functional | `./le test ospf`, `./le test isis`, `./le test ldp`, `./le test bfd` for the `.ci` the change reaches |
| Change set judged | GitHub CI's `verify` run of the pushed commit (no agent runs `./le verify`) |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Error leakage | A WARN that prints an OSPF or BFD authentication key |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

Every instance, its answer and the reason (population of 2026-11-15, the same 28):

| Instance (file, symbol) | Answer | Reason | Proof |
|-------------------------|--------|--------|-------|
| `bfd/bfd.go` `runtimeState.loopFor` (two stops) | 1 | The two stops repeated `Loop.Start`'s own rollback: a transport whose Start fails opens no socket and `UDP.Stop` is idempotent. Deleted; the real stop failure reaches the caller inside the start error | `TestLoopStartReturnsTheControlStopFailureOfItsRollback` |
| `bfd/engine/engine.go` `Loop.Start` (stop after a failed echo start) | 1 | Returns `errors.Join(echoErr, stopErr)`; the caller already fails on this path, so no clean shutdown changes (R-2) | same test |
| `bfd/engine/engine.go` `Loop.Stop`, `Loop.removeEntryLocked`, `bfd/engine/auth_join.go` `replaceAuth` (`CloseAuth`) | 1 and 2 | The drop hid a real failure: `auth.SeqPersister` latched a failed write of the Meticulous sequence number in a flag nothing read, so after a restart the session could reuse a sequence the peer already saw. `Close` now returns the kept write error; the persister reports the first failure of each failure run through a callback; `closeAuth` replaces the three sites and WARNs under `doctor-bfd-auth-seq-unsaved`, raised as an error event. No key is printed | `TestSeqPersistWriteFailure`, `TestSeqPersistFailureRunEndsWhenAWriteLands`, `TestReportSeqUnsavedWarnsAndRaisesAnErrorEvent` |
| `bfd/engine/echo.go` `Loop.reflectEchoLocked` | 2 | Per packet: every refused send is counted in `ze_bfd_send_failures_total{mode,packet}`; one WARN per packet kind per interval under `doctor-bfd-send-failed`, worded for an echo reflect (the peer's echo function fails), raised as a warning and cleared when a send of that kind lands, the session goes away, or two quiet windows pass | `TestEchoReflectFailureIsCountedAndWarnedOnce`, `TestEchoReflectWarningIsWithdrawnOnceTheFailuresStop` |
| `bfd/engine/loop.go` `Loop.handleInbound` (auth verify) | 2 | Per packet: counted in `ze_bfd_auth_failures_total`, WARN once per interval per session under `doctor-bfd-auth-failed`, raised and cleared on recovery. The rate window and the raise are kept apart, so packets that verify between bad ones do not re-arm the WARN (an injector cannot buy one WARN per packet). The secret is never printed (Verify answers fixed errors) | `TestAuthFailureIsCountedWarnedOnceAndClearedOnRecovery`, `TestInterleavedAuthFailuresWarnAndRaiseOncePerWindow` |
| `isis/events.go` `eventSink.SessionUp`, `SessionDown` | 2 | The bus refuses only an unregistered event pair, a Ze defect; the adjacency is right, other plugins miss the change. One WARN per refusal under `doctor-internal-defect` (adjacency changes, not packets) | `TestISISSessionEventRefusalIsWarned` |
| `isis/lsdb_wiring.go` `engine.publishLSPChange` | 2 | Same defect, per LSP: counted in `ze_isis_lsp_change_events_refused_total`, WARN once | `TestISISLSPChangeRefusalWarnsOnceAndCounts` |
| `isis/circuits.go` `engine.launchCircuitGoroutine` (Hello send) | 2 | A failed Hello costs the adjacency at hold expiry and the next tick retries: WARN once per failure run per level under `doctor-isis-hello-send-failed`, raised as a warning and cleared when Hellos go out again or the circuit is removed, the first success logs the run's length; a send on a circuit already closed by teardown is quiet | `TestISISHelloSendFailureWarnsOncePerRun`, `TestISISHelloSendOnClosedCircuitIsQuiet` |
| `ldp/register.go` `discoverOnInterface` (UDP close in the reader) | 3 | `guards readDiscovery`: discovery is stopping and the 1 s read deadline ends the reader whatever close answers | annotation |
| `ldp/register.go` `discoverOnInterface` (`SetMulticastTTL`) | 3 | `guards multicastAddr`: 224.0.0.2 is link-local (RFC 5771) and the kernel's default multicast TTL is already 1 | annotation |
| `ldp/register.go` `processDiscoveryPacket` (two) | 2 | Per packet, and RFC 5036 Section 3.5.1.2.1 says discard silently on the wire: counted in `ze_ldp_hello_discards_total{interface}`, WARN once per interface per quiet minute under `doctor-ldp-hello-discarded`; the three malformed returns that logged nothing now count too | `TestLDPMalformedHelloIsCountedAndWarnedOnce`, `TestHelloDiscardLogWarnsAgainAfterQuiet` |
| `ldp/register.go` `sendHello` | 2 | Now returns the write error; counted in `ze_ldp_hello_send_errors_total{interface}`, WARN once per failure run under `doctor-ldp-hello-send-failed`, cleared by the next good send or the end of discovery | `TestLDPHelloSendFailureIsWarned` |
| `ldp/register.go` `runSession` | 2 | A session Ze did not stop WARNs with the peer under `doctor-ldp-session-ended`; one Ze stopped stays Info | `TestLDPSessionEndByPeerIsWarned`, interop `ldp-backoff-clear-frr` |
| `ospf/ipsec_install.go` `removeLocked` (policy, interface SA, neighbor SA), `removeNeighborSALocked` | 2, absent: 3 | A failed delete leaves Ze-installed XFRM state in the kernel and the record is forgotten, so only the operator can delete it: WARN under `doctor-ospfv3-ipsec-entry-left`, raised as an error event whose subject is the SA or policy, with the delete command as the fix, never the key. ESRCH (state) and ENOENT (policy) mean already gone: quiet (`ipsecEntryAbsent`) | `TestOSPFIPsecRemovalFailureIsReported`, `TestOSPFIPsecNeighborRemovalFailureIsReported`, `TestOSPFIPsecRemovalOfAbsentEntryIsQuiet` |
| `ospf/events.go` `eventSink.emitNeighbor`, `emit`, `lastKeyExpired` | 2 | The bus refuses only an unregistered pair or an unencodable payload, a Ze defect; OSPF is right, subscribers miss the change. WARN per refusal under `doctor-internal-defect` | `TestOSPFEventEmitRefusalIsWarned` |
| `ospf/bfd_client.go` `engine.detachBFDClient`, `engine.stopBFDSession` | 2 | `ReleaseSession` fails only with `ErrForeignHandle`, a Ze defect; the session stays held in BFD. WARN under `doctor-internal-defect`, fix `ze show bfd sessions`; the `api.GetService` nil check is untouched | `TestOSPFBFDStopReleaseRefusalIsWarned`, `TestOSPFBFDDetachReleaseRefusalIsWarned` |
| `ospf/transport/transport.go` `Transport.addrWorker` | 3 | `guards RescanInterfaces`: a failed open is how a link with no IPv4 address leaves OSPF; the 30 s rescan retries the open and WARNs each failure | annotation |

Every new WARN carries a doctor code a check declares (`./le arch doctor-coverage check`), so the change registers new event codes rather than reusing a log level: `doctor-bfd-auth-failed`, `doctor-bfd-auth-seq-unsaved`, `doctor-bfd-send-failed`, `doctor-isis-hello-send-failed`, `doctor-ldp-hello-send-failed`, `doctor-ldp-hello-discarded`, `doctor-ldp-session-ended`, `doctor-ospfv3-ipsec-entry-left`. A Ze defect (a refused event emit, a refused BFD release) uses the existing `doctor-internal-defect`.

Every raise goes through the plugin SDK (`sdk.Plugin.RaiseIssue` / `ClearIssue`), never `internal/core/report`, because each of these plugins can run forked and a forked plugin's own report bus is a copy nobody reads (`docs/architecture/doctor-and-health-checks.md`). Each plugin starts one `issuequeue.Queue` over its `sdk.Plugin` at its SDK entry and stops it after its engines stop, so final clears are sent; BFD reaches it through `engine.IssueReporter`. A raise only enqueues, so no raise waits on the daemon, under a lock (the OSPF IPsec installer's, which every Hello takes) or on the BFD express loop; a refusal is logged by the queue's callback, and a condition that persists retries a refused raise once per window. A condition that can end raises a warning and clears it when it ends (Hello send recovers, auth verifies again, sends land, two quiet windows pass, the circuit or session goes away); a defect raises an error event. A failed BFD send is coded too (`doctor-bfd-send-failed`), worded per packet kind and rate-limited per kind.

The OSPF IPsec fix line names the policy's interface by index (`dev if<N>`), which `ip` resolves as an index: the name stops resolving once the interface is deleted, and the leftover policy still carries the old index.

The LDP session end is now a coded WARN rather than the Info line `ldp: session ended`, so the interop checker of `ldp-backoff-clear-frr` (`internal/le/interoplab/bgp/check_ldp_backoff_clear.go`, the le tooling) counts the code, proves that info lines are read with `ldp: session operational`, and its replay fixture was recorded again from a passing lab run.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|

The class row for the whole population is owned by the umbrella.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The umbrella's three answers, in order | Raise every line to WARN | A WARN does not make a default correct |
| Per-packet failures count and WARN once | WARN per packet | Discovery and echo paths run per packet |

## Known Limitations
- None.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass; GitHub CI verifies the pushed commit
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason): N-A unless answer 1 changes wire behavior, which then owes the FRR or BIRD scenario

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** code + tests + docs + edited spec
- [ ] **Commit B:** `remove plan/release/fixes/spec-dropped-error-6-routing-protocols.md` only, in the same `./le commit create` script

## Implementation Summary

### What Was Implemented
- The 28 instances resolved as the Design Insights table records; one shared report queue (`internal/component/plugin/issuequeue` `Queue`, `RunRaises`) that every raise of OSPF, IS-IS, LDP and BFD goes through.
- Closure round 1 added the refused-raise tests: `TestOSPFRefusedRaisesAreWarned` (`internal/plugins/ospf/issues_refused_test.go`), `TestISISRefusedDefectRaisesAreWarned`, `TestLDPRefusedReportsAreWarned`, `TestRefusedReportsAreWarned` (bfd/engine), and `issue.Message` checks in the IS-IS and LDP raise tests.

### Bugs Found/Fixed
- OSPF `reportSARemoval` / `reportPolicyRemoval` read a textbuf subject twice (String empties it), so raises had an empty subject: fixed before closure, covered by `TestOSPFIPsecRemovalFailureIsReported`.
- Closure round 1: `internal/plugins/ldp/register.go` `processDiscoveryPacket` sliced `data[msgBodyStart:msgBodyEnd]` with a message Length below 4, end before start, a panic any host on the link could cause (pre-existing, in the function this spec changed). Now discarded and counted under `errHelloShorterThanMessageID` (`problems.go`); `TestLDPHelloShorterThanItsMessageIDIsDiscarded`.
- Closure round 1 (continuation): `internal/le/interoplab/bgp/check_ldp_backoff_clear.go` still described the fatal Notification as `Session.ReadLoop`'s WARN after `Session.processMessages` moved it to INFO, and `requireLDPBackoffReleased`'s first refusal blamed a missing Notification line only on the clear, though a ze.conf without the ldp info level now hides it too. The comment and the refusal name both; `TestRequireLDPBackoffReleased` builds its lines in the producer's current form and gains the "info not logged" case such a log really gives.

### Documentation Updates
- The pages in the diff (`git diff --stat HEAD -- docs`): bfd, doctor-and-health-checks, isis-4/5/6, mpls-ldp, ospf-4, ospf-ext-16, testing/interop row 66, guides bfd, health-checks, isis, mpls, plugin-development/metrics. Every claim of that diff was re-read against its anchored producer at closure (Documentation Verified).
- `./le doc check all` (2026-11-15, worktree): FAILED on four source anchors only, all outside this change and unchanged by it: `docs/architecture/resolve.md` (`store.Open`), `docs/guide/irr-filtering.md` (`clearResolverCache`, `readmitRetryInterval`), `docs/guide/plugins.md` (`ConfigReads`). Every other section passed, digest anchors included.
- Citers: `plan/release/fixes/spec-dropped-error-0-umbrella.md` children table restated as the bare stem; `plan/release/fixes/spec-ospf-isis-bfd-vrrp-one-off-defects.md` AC-20 already cites the bare stem; the `// VALIDATES: spec-dropped-error-6 ...` test comments name no path.

### Deviations from Plan
- The Wiring Test names `TestLDPHelloSendFailureIsWarned` in `register_test.go`; it exists there, and the LDP problem tests live in `problems_test.go` because `register.go` passed 1000 lines.
- `internal/le/interoplab/bgp/check_ldp_backoff_clear.go` (le tooling) changed because the LDP session end moved from an Info line to a coded WARN.

## Mistake Log
| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The refused-raise callbacks of every plugin shipped with no test driving a refusal into them | Each is the only trace of a report the daemon refused | Second review NOTE, treated as ISSUE at closure | Four tests added in closure round 1 |
| approach | `Session.processMessages` moved the fatal Notification line from WARN to INFO after the interop checker had been rewritten for the session-end WARN, and nobody re-read the checker | The checker's comment, its first refusal and its test lines all described the old level, and the committed recording (21:18 run) holds the old forms | Closure review of `check_ldp_backoff_clear.go` against `session.go` | Fixed in closure round 1; a producer whose log line a checker counts is re-read with that checker |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Every `error-dropped-below-warn` instance under the door's paths resolved by one answer | Done | Design Insights table | Main thread's report run: zero in the refused section |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `./le arch refuse-by-default report` run by the main thread | zero `error-dropped-below-warn` under ospf, isis, ldp, bfd |
| AC-2 | Done | `TestLoopStartReturnsTheControlStopFailureOfItsRollback`, `TestSeqPersistWriteFailure` | |
| AC-3 | Done | the Proof column tests, plus the four refused-raise tests | green in the main thread's package runs and in `-race -count=20` |
| AC-4 | Done | annotations name `readDiscovery`, `multicastAddr`, `RescanInterfaces` | accepted section of the report |
| AC-5 | Done on this host, QEMU part owned elsewhere | package tests green; host suites bfd 2/2, ldp 4/4, isis 17/17, ospfv3 29/29, ospf all but `ospf-stub` and `ospf-redist-bgp` | the 17 guest-only `.ci` skipped here and the two stale ospf wordings: Work Not Done |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestOSPFIPsecRemovalFailureIsReported` | Done | `internal/plugins/ospf/ipsec_install_test.go` | |
| `TestLDPHelloSendFailureIsWarned` | Done | `internal/plugins/ldp/register_test.go` | |
| One test per other answer-1 instance | Done | `TestLoopStartReturnsTheControlStopFailureOfItsRollback` (bfd/engine), `TestSeqPersistWriteFailure` (bfd/auth) | |
| `TestRequireLDPBackoffReleased`, `TestLDPBackoffClearReplaysTheLabRecording` | Done, rerun owed | `internal/le/interoplab/bgp/check_ldp_backoff_clear_test.go` | closure changed the checker's refusal and the test's lines |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| The 13 files of Files to Modify | Done | each in `git diff --stat HEAD` |

### Audit Summary
- **Total items:** 5 AC, 28 instances
- **Done:** AC-1 to AC-4; AC-5 for every suite this machine can run
- **Partial:** none in this spec; the QEMU runs of AC-5 are owned by `plan/release/delivery/spec-finish-appliance-qemu-evidence.md` (Work Not Done)
- **Skipped:** none
- **Changed:** the two Deviations above

## Goal Validation (BLOCKING)
| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| No error in OSPF, IS-IS, LDP or BFD is caught and logged below WARN | structural check | `./le arch refuse-by-default report` (main thread, final tree): zero `error-dropped-below-warn` under the four paths, and no new refused finding on a changed line |
| An operator sees each failure with what, why and the fix, and doctor reports it | unit + structural | the Proof column tests assert each WARN's problem, why and fix and each raise's code, subject and Message; the four refused-raise tests (`TestOSPFRefusedRaisesAreWarned`, `TestISISRefusedDefectRaisesAreWarned`, `TestLDPRefusedReportsAreWarned`, bfd/engine `TestRefusedReportsAreWarned`) prove a refused raise still reaches the log; `./le arch doctor-coverage check` exit 0 over the eight new codes |
| A raise never blocks a protocol goroutine and survives a forked plugin | unit, race | `internal/component/plugin/issuequeue` `queue_test.go` (`TestQueueStopBoundsTheDrainAgainstAHungDaemon`); `go test -race -count=20` over the refusal, retry, queue and withdrawal tests: ok |
| A peer cannot panic LDP discovery with a short Hello | security negative | `TestLDPHelloShorterThanItsMessageIDIsDiscarded`: the Length-below-4 Hello is discarded, counted on its interface only, and warned once |
| Protocol behavior toward peers unchanged | interop | `ldp-backoff-clear-frr` (Ze vs FRR ldpd) green three times, the second (image sha256:ee205344c287, recording `ldp-backoff-clear-frr-GE65TPKCHYIAF5ZCWWJXHRIMZH`, verdict passed:true) on the final session-end path (`report=queued`, Notification at INFO), the third (image sha256:a1ae1e71d116) on the final code with the short-Hello length guard; host suites of AC-5 |

## Work Not Done
| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| A forked bfd persists the Meticulous sequence through `statestore.Put`, which saves nothing there | Not the dropped-error shape; needs the SDK state path in the persister | `plan/release/fixes/spec-ospf-isis-bfd-vrrp-one-off-defects.md` AC-20 |
| `ipsecInstaller.removeLocked` skips every delete when the dataplane lookup fails | Not the dropped-error shape | `plan/release/fixes/spec-ospf-isis-bfd-vrrp-one-off-defects.md` AC-21 |
| `ospf-stub.ci`, `ospf-redist-bgp.ci` expect the old validator wording | Red on laptop main before this branch | `plan/release/fixes/spec-ospf-isis-bfd-vrrp-one-off-defects.md` AC-22 |
| The guest-only `.ci` of AC-5 (bfd 2, ldp 2, ospfv3 3, ospf 10) in QEMU | QEMU cannot run on the server: the runtime-kernel 7.3 cache key moved and a rebuild needs 40 GB on a 23 GB disk; GitHub CI's verify run is the evidence meanwhile | `plan/release/delivery/spec-finish-appliance-qemu-evidence.md`, section "From `spec-dropped-error-6-routing-protocols` closure" |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/dropped-error-6-routing-protocols-5a9a4bd9-5348-4116-8dd3-8815135bbc31.md` (71 files, verdict clean, rounds 2), recorded over every file commit A carries |
| `./le spec review check` | passed over the same files (closing agent, 2026-11-15) |
| Rounds | 2 |
| Reviewer lenses used | wiring, logic, concurrency, security (peer-reachable panic, key leakage), style (the six questions of `docs/contributing/ze-go-style.md`), docs against source anchors |

Round 1 ran in two closing agents. The first read the issuequeue package, OSPF, the IS-IS events and LSDB wiring, LDP and the BFD auth, echo and loop paths. The continuation read `isis/circuits.go` `helloSendWarn` and `isis/server.go`, `bfd/engine/engine.go` `Loop.Start`, `Loop.Stop`, `removeEntryLocked`, `IssueReporter` and `issueSink`, `bfd/bfd.go` `loopFor` and `runBFDPlugin`, `bfd/register.go`, `bfd/metrics.go`, the le checker and its test, and the whole docs diff against its anchors. It confirmed at the producers: `loopFor` may drop its two stops because `UDP.Start` opens no socket when it fails and `Loop.Start` joins the control stop's failure; `runBFDPlugin` defers `issues.Stop` before `stopAll`, so the queue stops after every loop; the daemon clears a removed plugin's warnings (`plugin/server/dispatch.go` calls `report.ClearSource`), which the IS-IS `e.ctx` exit and every queue's `Stop` rely on; IS-IS `engine.shutdown` runs only at plugin exit; `RunRaises` is safe across the Hello goroutine and the queue worker (`refused` is atomic, the rest is the owner's). Style pass: no `panic` a peer reaches (the issuequeue `enqueue` panic needs a nil callback, a Ze defect; the BFD `register.go` panics run in `init`), every new goroutine is the queue's one worker with `Stop`, every queue is bounded (256) with a defined refusal.

Round 2 re-read every fix of round 1 (the LDP length guard, the four refused-raise tests, the Message assertions, the main thread's `slogTextValue` helper and the other-interface counter assertion, and the le checker fix): 0 BLOCKER, 0 ISSUE.

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | A Hello with message Length below 4 panics the discovery reader | `internal/plugins/ldp/register.go` `processDiscoveryPacket` | length check, `errHelloShorterThanMessageID`, `TestLDPHelloShorterThanItsMessageIDIsDiscarded` |
| 2 | ISSUE | No test drives a daemon refusal into the refused-raise callbacks of OSPF, IS-IS, LDP and BFD | `ospf/issues.go`, `isis/issues.go`, `ldp/problems.go`, `bfd/engine/problems.go` | the four refused-raise tests |
| 3 | ISSUE | IS-IS and LDP raise tests never check the raised Message | `isis/events_refused_warn_test.go`, `isis/hello_send_warn_test.go`, `ldp/problems_test.go` | Message assertions added |
| 4 | ISSUE | The interop checker's comment named `Session.ReadLoop` (Warn) for the fatal Notification, which `Session.processMessages` now logs at INFO; its first refusal blamed a missing line only on the clear, though a ze.conf without `ldp info` hides it too; its test composed the old level and error text | `internal/le/interoplab/bgp/check_ldp_backoff_clear.go` `requireLDPBackoffReleased`, `check_ldp_backoff_clear_test.go` `TestRequireLDPBackoffReleased` | comment and refusal name both causes; the test lines take the producer's form, and the "info not logged" case is a log holding only the WARN session end (the old case, renamed "no operational session logged", is kept) |

NOTEs: `./le repo check`'s nine unwired-export findings are pre-existing (same symbols at HEAD); `bfd/engine/problems.go` `reportOutcome` copies `issuequeue.Outcome` behind the `IssueReporter` seam; the committed recording `ldp-backoff-clear-frr-X3537EPPM42IHDWQ4R4Z5JDI2J.jsonl` is the 21:18 run, before the Notification moved to INFO and before the report queue (`report=raised`), while the 22:34 run `GE65TPKCHYIAF5ZCWWJXHRIMZH` (passed:true) holds the final forms; the checker counts substrings both carry, so the replay proves the same, and swapping the fixture would need a fresh judge verdict; `slogTextValue` leaves an empty value bare where slog writes `""`, and no test passes an empty subject.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/plugin/issuequeue/queue.go` | Yes | read at closure |
| `internal/component/plugin/issuequeue/raises.go`, `queue_test.go` | Yes | `git status`: untracked, carried by commit A |
| `internal/plugins/ldp/problems.go`, `ospf/issues.go`, `isis/issues.go`, `bfd/engine/problems.go` | Yes | `git status`: untracked, carried by commit A |
| The new tests of Files to Create | Yes | `git status`: `ldp/problems_test.go`, `ospf/{issues_test,issues_refused_test,events_emit_test,bfd_release_refused_test}.go`, `isis/{events_refused_warn_test,hello_send_warn_test}.go`, `bfd/engine/failure_report_test.go` |
| `internal/le/interoplab/bgp/testdata/ldp-backoff-clear-frr/ldp-backoff-clear-frr-X3537EPPM42IHDWQ4R4Z5JDI2J.jsonl` | Yes | the only recording once commit A removes the 6PHGHPC5 one (`TestLDPBackoffClearReplaysTheLabRecording` wants exactly one) |
| `website/changes/fragments/dropped-error-6-routing-protocols.md` | Yes | written at closure |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | Zero `error-dropped-below-warn` under the child's paths | `./le arch refuse-by-default report` (main thread, final tree): zero under ospf, isis, ldp, bfd; no new refused finding on a changed line |
| AC-2 | Each answer-1 caller observes the error | `TestLoopStartReturnsTheControlStopFailureOfItsRollback`, `TestSeqPersistWriteFailure`: green in `./le test unit pkg path` for `./internal/component/bfd/engine/` and `./internal/component/bfd/auth/` |
| AC-3 | One WARN per answer-2 file names what, why and the fix | the Proof column tests and the four refused-raise tests: green in the package runs of ldp, ospf, isis, bfd/engine, issuequeue, and `-race -count=20` ok |
| AC-4 | Each annotation names a guard | `grep refuse-by-default: guards` in `ldp/register.go` and `ospf/transport/transport.go` |
| AC-5 | Package tests and the reached `.ci` green | packages listed above plus `./internal/component/bfd/`, `./internal/component/bfd/session/`, `./internal/plugins/ospf/transport/`, `./internal/le/arch/doctorcoverage/`: green; host suites bfd 2/2, ldp 4/4, isis 17/17, ospfv3 29/29, ospf all but the two of AC-22 |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| LDP session end WARN | interop `ldp-backoff-clear-frr` | green twice (main thread), the second on the final session-end path |
| BFD sessions, auth and echo paths through the plugin | `test/bfd` host suite | 2/2, 2 skipped for capabilities (QEMU: Work Not Done) |
| IS-IS circuits, Hellos and LSDB events through the plugin | `test/isis` host suite | 17/17 |
| OSPF and OSPFv3 engines through the plugin | `test/ospf`, `test/ospfv3` host suites | ospfv3 29/29; ospf all but `ospf-stub` and `ospf-redist-bgp` (AC-22) |
| Every new code is declared by a doctor check or registered as an event code | `./le arch doctor-coverage check`, `./le arch tier check` | exit 0 |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | report of 2026-11-15: 28, same files and symbols |
| A-2 | confirmed | `ipsecEntryAbsent` (ESRCH, ENOENT); `TestOSPFIPsecRemovalOfAbsentEntryIsQuiet` |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Metrics rows | `ldp` registers `ze_ldp_sessions_active`, `ze_ldp_sessions_total`, `ze_ldp_bindings_active`, `ze_ldp_bindings_total`, `ze_ldp_hello_send_errors_total`, `ze_ldp_hello_discards_total`; `isis/server.go` `setMetrics` `ze_isis_lsp_change_events_refused_total`; `bfd/metrics.go` `bindMetricsRegistry` `ze_bfd_send_failures_total{mode,packet}` | Yes, `docs/plugin-development/metrics.md` and `docs/guide/bfd.md` match |
| Report queue numbers (256, 1 s per report, 3 s drain) | `issuequeue/queue.go` `queueDepth`, `reportTimeout`, `stopDrainMax` | Yes, `doctor-and-health-checks.md` and `bfd.md` match |
| Windows (IS-IS 10 s retry, LDP one minute, BFD one minute and two-minute quiet) | `isis/auth_warn.go` `authFailWarnInterval`; `ldp/problems.go` `helloDiscardQuiet`; `bfd/engine/loop.go` `sendWarnInterval`, `warningQuiet` | Yes, `isis-5-adjacency.md`, `mpls-ldp.md`, `guide/bfd.md`, `guide/health-checks.md`, `bfd/register.go` descriptions match |
| LDP log lines (session end WARN, Notification at INFO, `ldp: session closed`) | `ldp/problems.go` `reportSessionEnd`; `ldp/session.go` `processMessages` | Yes, `guide/mpls.md`, `mpls-ldp.md`, interop row 66 match |
| The daemon clears a removed plugin's warnings | `plugin/server/dispatch.go` calls `report.ClearSource` | Yes, `issuequeue.Queue.Stop` comment and the IS-IS `e.ctx` comment match |
| `./le doc check all` | four anchor failures, all in IRR and resolve pages this change does not touch | the change's own pages pass |
