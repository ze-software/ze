# Spec: dropped-error-6-routing-protocols

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | protocol |
| Priority | P1 |
| Release reason | 28 errors in OSPF, IS-IS, LDP and BFD are caught and logged only below WARN: an OSPF IPsec policy that fails to be removed stays in the kernel, an LDP hello that fails to send, a BFD loop that fails to stop, each with no trace at the default level |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-02 |

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
| A-1 | The 28 are still the population | Report of 2026-11-02 | Work from the fresh report | Report filtered to this child's paths before the first edit | unvalidated |
| A-2 | A failed XFRM removal at neighbor teardown leaves the policy installed | Netlink delete error | ENOENT is answer 3 | Distinguish "already gone" in each removal | unvalidated |

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
- Test additions in the packages above; no new source file.

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
| Prometheus counters/metrics | Yes, if answer 2 counts | Existing protocol families; name each counter used at implementation |
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
| 14 | Prometheus counters added/changed? | If answer 2 adds a counter | `docs/plugin-development/metrics.md` |
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
| Change set judged | `./le verify changed` after the closing commit |

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
- [ ] The targeted tests for what changed pass, and `./le verify changed` over the closing commits
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
