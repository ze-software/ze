# Spec: fixit-peer-verdict-and-forward-rail

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/5 |
| Updated | 2026-07-22 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `tmp/session/notes-48807c59-0aa5-4a58-8483-c4cc3cb05b46.md` -- working notes
3. `rfc/short/rfc4271.md` (AS_PATH propagation), `rfc/short/rfc4724.md` (End-of-RIB)
4. Source files under Current Behavior

## Task

Six defects found while diagnosing the four remaining `bgp plugin` functional-test
failures (91, 224, 398, 458). Only ONE of the four is a product bug; the rest are
harness and fixture defects that were masking it.

1. **[BLOCKER] The multi-peer test verdict is fail-open.** `runner_exec.go:1211`
   decides a check-mode peer test with
   `!strings.Contains(rec.PeerOutput, "successful")`, and `rec.PeerOutput` is the
   concatenation of EVERY peer's stdout+stderr (`runner_exec.go:1058-1064`). One
   peer printing `successful` therefore passes the test however many other peers
   failed. This is the same defect class `peer_contract.go:10-17` was written to
   close (a test that cannot fail), one layer further out. It is also why 224 and
   398 read as "flaky 1-in-10" when their underlying failure is deterministic.

2. **[HIGH] 52 malformed BGP frames in two fixtures.** Independently verified by
   byte count (`tmp/checkhex-48807c59.py`):
   `test/plugin/forward-overflow-two-tier.ci` -- 50 of 50 `action=send` frames
   declare Length 48 and carry 49 bytes (a stray `00` after each `/24` NLRI);
   `test/plugin/role-otc-unicast-scope.ci:24` -- declares 58, carries 59, and its
   MP_REACH_NLRI sets the extended-length flag bit (`0x90`) while using a 1-byte
   length field. The trailing byte desynchronises the stream, so the next header
   fails its marker check and the session is torn down. The daemon is behaving
   correctly; the fixtures never tested what they claim.

3. **[HIGH] Intermittent un-prepended AS_PATH to an eBGP peer.** Test 91
   (`test/plugin/bgp-rs-reactor-fastpath.ci`, fixtures verified well-formed) fails
   2 in 8 runs in isolation. The failing frame carries `AS_PATH: [65001]` where
   `[65000, 65001]` is expected. RFC 4271 Section 5.1.2 (`rfc/full/rfc4271.txt:1408-1414`)
   requires the local AS be prepended when advertising to an external peer. Both
   forwarding rails contain the prepend branch (`forward_rs.go:357-364`,
   `reactor_api_forward.go:540-547`) and both gate it on
   `facts.isEBGP && !facts.rsClient`; the config sets `behavior { rs-fast-path }`
   (`reactor/config.go:220`) and NOT `session { rs-client }` (`:267`), so
   `facts.rsClient` is false and the prepend must fire. Root cause not yet
   attributed -- Phase 3 instruments the gate.

4. **[MEDIUM] Two fail-open branches at the fast-path seam.**
   (a) `reactor_notify.go:571-578`: `msg.ReactorForwarded = true` is set only
   inside `if ok` on the `recentUpdates.Get` lookup, with no else. On a miss the
   fast path silently does nothing and bgp-rs takes over via the `!reactorHandled`
   branch (`rs/server_withdrawal.go:71-72`).
   (b) If the fast path runs but matches zero peers, `reactorForwardRS` returns
   early (`forward_rs.go:121-123`) with an empty `skipped`, so bgp-rs takes
   `default: releaseCache` (`rs/server_withdrawal.go:75-76`) and the UPDATE reaches
   nobody.

5. **[LOW] Over-specified End-of-RIB ordering in two sibling tests.**
   `bgp-rs-reactor-fastpath.ci:42-43` asserts EOR-then-UPDATE; its sibling
   `bgp-rs-reactor-fastpath-fallback.ci:26,30` asserts UPDATE-then-EOR for the same
   topology. RFC 4724 constrains only that the EOR follow the initial dump
   (`rfc/short/rfc4724.md:126`), never that nothing precede it, so neither ordering
   is a property ze owes. `checker.go:413-418` already swallows unmatched EORs for
   exactly this reason; both tests opted out by declaring EOR expectations.
   Test 91's justifying comment (`:32-41`) cites the `ShouldQueue` FIFO drain
   (`peer_initial_sync.go:180-188`), which governs the route-injection rail, not the
   forwarding rail under test.

6. **[LOW] Reversed-polarity comment.** `peer_initial_sync.go:318` states HoldWrites
   blocks the RS via `Lock` and the forward pool via `TryLock`. It is the reverse:
   the fast path `TryLock`s (`forward_rs.go:48`), the pool worker blocks
   (`forward_pool.go:131-132`).

Out of scope, recorded for its own spec: the forwarding rail consults
`ShouldQueue()` nowhere (only `reactor_api_batch.go:106`, `:235`,
`reactor_api_forward.go:58`), so a forwarded withdraw can overtake a queued
announce for the same prefix.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/testing/ci-format.md` - `.ci` grammar
  → Constraint: `action=send:conn=N:seq=N:hex=` injects raw bytes; the declared BGP
    Length field must equal the frame's actual byte count or the stream desynchronises.

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4271.md` - AS_PATH propagation
  → Constraint: Section 5.1.2 -- when advertising to an external peer the speaker
    prepends its own AS. The summary extracts only the `MAY` for multiple prepends
    (`:731`); the base obligation is unextracted (defect 9 below).
- [ ] `rfc/short/rfc4724.md` - End-of-RIB
  → Constraint: `[RFC4724-4-1]` (`:126`) -- the EOR MUST be sent once the initial
    routing update completes. One-directional: nothing forbids an UPDATE preceding it.
- [ ] `rfc/short/rfc7947.md` - route server
  → Constraint: `[RFC7947-x-1]` (`:42`) -- an RS MUST NOT prepend its own AS for
    RS-client peers. Applies only when `session { rs-client }` is set.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/test/runner/runner_exec.go:1058-1064` - peer output concatenation
  → Constraint: `rec.PeerOutput` is every peer's stdout then every peer's stderr,
    joined; per-peer attribution is lost at this point.
- [ ] `internal/test/runner/runner_exec.go:1208-1221` - the verdict
  → Constraint: `isSelfValidated` gates it; inside, a single `strings.Contains`
    over the joined output decides pass/fail for ALL peers.
- [ ] `internal/test/runner/peer_contract.go` - the peer contract
  → Constraint: `hasCheckPeer` (`:42-49`) already distinguishes check-mode peers
    from sink/echo/inject scaffolding; only check-mode peers report `successful`.
- [ ] `internal/test/runner/runner_exec_util.go:315-326` - `peerOutput` struct
  → Constraint: one per ze-peer, holding `stdout *syncWriter`, `stderr *lockedBuilder`,
    `proc *exec.Cmd`. Per-peer output IS retained; only the verdict discards it.
- [ ] `internal/test/peer/checker.go:406-427` - message matching
  → Constraint: an unmatched KEEPALIVE or EOR is silently accepted (`:416-418`),
    documented as "EOR may arrive before expected messages due to initial route sync
    racing with the read loop".
- [ ] `internal/component/bgp/reactor/reactor_notify.go:560-590` - fast-path gate
  → Constraint: five conjuncts at `:570`, then `recentUpdates.Get` at `:571`;
    `ReactorForwarded` set at `:574` only when both pass.
- [ ] `internal/component/bgp/reactor/forward_rs.go:28-72,101-123,354-364` - fast path
  → Constraint: `tryDirectWriteNoFlush` gates on session non-nil, Established,
    `writeMu.TryLock()`; selection filters on `forwardFacts() != nil` and
    `exportFilters` only; prepend at `:357` gated on `isEBGP && !rsClient`.
- [ ] `internal/component/bgp/plugins/rs/server_withdrawal.go:64-77` - fallback switch
  → Constraint: three-arm switch on `reactorHandled` and `len(FastPathSkipped)`;
    the `default` arm releases the cache and forwards nothing.
- [ ] `internal/component/bgp/reactor/peer_initial_sync.go:314-346` - EOR emission
  → Constraint: `HoldWrites()` at `:327`, config/plugin routes at `:332`, per-family
    EOR at `:337-342`, `ReleaseWrites()` at `:345`. Correct per RFC 4724; not changed.
- [ ] `internal/component/bgp/reactor/peer_forward_facts.go:78-80,101-152` - facts
  → Constraint: `forwardFacts()` is a plain atomic load; `rsClient` copied from
    `s.RSClient` at `:111`. Non-nil from `setEncodingContexts` (`peer.go:563`)
    until teardown (`peer.go:576`), so it spans the whole initial-sync window.

**Behavior to preserve:**
- `checker.go:416-418` EOR/KEEPALIVE tolerance: it is the mechanism that makes the
  de-specified tests order-independent, and must not be removed.
- `isSelfValidated` semantics (`peer_contract.go:59-73`): a check-mode peer is never
  self-validated; an exit-code assertion must not disable the peer path.
- Sink/echo/inject peers never report completion and must never be required to.
- `peer_initial_sync.go` EOR-after-dump ordering (RFC 4724 `[RFC4724-4-1]`).
- The prepend policy split: prepend for eBGP non-RS-client, never for RS-clients
  (RFC 7947 `[RFC7947-x-1]`).
- Fixture edits change only malformed bytes; every assertion count stays >= today.

**Behavior to change:**
- Defects 1-6 above. Nothing else.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
A `.ci` test launches one or more `ze-peer` processes plus a `ze` daemon; peers
exchange BGP with the daemon and each prints `successful` or a mismatch diff.

### Transformation Path
1. Each `ze-peer` writes to its own `peerOutput` (`runner_exec_util.go:318-326`).
2. `runner_exec.go:1058-1064` concatenates all of them into `rec.PeerOutput`.
3. `runner_exec.go:1208-1221` decides the verdict from that single string.
4. Independently: the daemon's read goroutine calls `notifyMessageReceiver`, which
   may invoke `reactorForwardRS` (`reactor_notify.go:573`) and set
   `ReactorForwarded` (`:574`); bgp-rs later branches on that flag
   (`rs/server_withdrawal.go:70-77`).
5. Whichever rail forwards applies the eBGP prepend at `forward_rs.go:357-364` or
   `reactor_api_forward.go:540-547`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| ze-peer process ↔ runner | stdout/stderr pipes into per-peer `peerOutput`; collapsed to one string at `runner_exec.go:1064` | [ ] |
| read goroutine ↔ bgp-rs delivery goroutine | `RawMessage.ReactorForwarded` / `.FastPathSkipped` (`bgp/types/rawmessage.go:34-37`) | [ ] |
| fast path ↔ forward pool | `writeMu.TryLock()` (`forward_rs.go:48`) vs blocking `Lock` (`forward_pool.go:131`) | [ ] |

### Integration Points
- `internal/test/runner` verdict logic - gains per-peer evaluation.
- `internal/component/bgp/reactor/reactor_notify.go` - gains an explicit decline path.
- `internal/component/bgp/plugins/rs/server_withdrawal.go` - default arm must not
  silently drop.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | Per-peer verdict will turn currently-green multi-peer tests red | `runner_exec.go:1211` passes on any peer's success; 224/398 already observed passing with a failing peer | Blast radius smaller than feared; still correct | Phase 1 measures it by running the full suite before/after | unvalidated |
| A-2 | The 52 frames are the whole fixture defect, not a symptom of a generator | byte-count check over all `hex=` in the three files (`tmp/checkhex-48807c59.py`); test 91's 3 frames are clean | A generator needs fixing too | grep for a fixture generator in Phase 2 | unvalidated |
| A-3 | Removing EOR expectations makes 90/91 order-independent | `checker.go:416-418` swallows unmatched EOR/KEEPALIVE | Tests still flake on ordering | Phase 4 runs each 20x | unvalidated |
| A-4 | The un-prepended frame comes from a rail that skipped the prepend, not from a peer echo | both rails gate prepend identically; `EBGPWire` cannot return `(nil, nil)` (`received_update.go:174-175`, `:203`) | Root cause is elsewhere | Phase 3 instrumentation | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Per-peer verdict turns many tests red at once, blocking the suite | Phase 1 measurement | Land the fix behind measurement first; triage the newly-red set as its own phase before committing |
| R-2 | Fixture repair changes what a test proves | assertions that only ever matched malformed bytes | Byte-count check re-run after edit; declared lengths unchanged |
| R-3 | De-specifying EOR order hides a genuine future ordering regression | none | The `ShouldQueue` gap keeps its own spec and its own test |
| R-4 | Phase 3 finds the AS_PATH race is in shared reactor code | `-race` reports | `make ze-race-reactor` required before claiming done |

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| multi-peer `.ci` where one peer succeeds and another fails | -> | per-peer verdict in `runner_exec.go` | `TestPeerVerdictRequiresAllCheckPeers` |
| `.ci` frame whose declared Length != actual bytes | -> | fixture lint | `TestCIFrameLengthsWellFormed` |
| eBGP non-RS-client destination receives a forwarded route | -> | prepend branch | `test/plugin/bgp-rs-reactor-fastpath.ci` |
| fast path declines after cache miss | -> | explicit decline, no silent rail switch | `TestFastPathDeclineIsExplicit` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Multi-peer test, peer A prints `successful`, peer B prints a mismatch | Test FAILS, and the failure names peer B |
| AC-2 | Every `hex=` frame in `test/**/*.ci` | Declared BGP Length equals actual byte count; a violation fails a gate |
| AC-3 | `forward-overflow-two-tier` and `role-otc-unicast-scope` run | Sessions stay established; no `invalid marker` teardown |
| AC-4 | Test 91 run 20 times | 20 passes; every forwarded frame to the eBGP peer carries `AS_PATH [65000, 65001]` |
| AC-5 | Fast path enabled but declines (cache miss or zero matched peers) | The decline is logged and the UPDATE is still forwarded exactly once; never silently dropped |
| AC-6 | Tests 90 and 91 run 20 times each | No failure attributable to EOR ordering |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | operator peers two eBGP routers through ze with the RS fast path | source UPDATE -> `reactorForwardRS` -> prepend -> destination wire | `test/plugin/bgp-rs-reactor-fastpath.ci` |
| 2 | developer adds a multi-peer `.ci` where one peer is wrong | runner evaluates each check peer | `TestPeerVerdictRequiresAllCheckPeers` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPeerVerdictRequiresAllCheckPeers` | `internal/test/runner/peer_contract_test.go` | AC-1: one success does not mask another peer's failure | |
| `TestPeerVerdictIgnoresScaffoldingPeers` | `internal/test/runner/peer_contract_test.go` | sink/echo/inject peers are not required to report success | |
| `TestCIFrameLengthsWellFormed` | `internal/test/runner/ci_fixture_test.go` | AC-2: every `.ci` frame's declared Length matches its byte count | |
| `TestFastPathDeclineIsExplicit` | `internal/component/bgp/reactor/forward_rs_test.go` | AC-5: a declined fast path routes every peer to the fallback | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgp-rs-reactor-fastpath` | `test/plugin/bgp-rs-reactor-fastpath.ci` | AC-4/AC-6: forwarded route carries the prepended AS_PATH, order-independent | |
| `bgp-rs-reactor-fastpath-fallback` | `test/plugin/bgp-rs-reactor-fastpath-fallback.ci` | AC-6: same, fallback rail | |
| `forward-overflow-two-tier` | `test/plugin/forward-overflow-two-tier.ci` | AC-3: 50 routes delivered through the overflow pool | |
| `role-otc-unicast-scope` | `test/plugin/role-otc-unicast-scope.ci` | AC-3: OTC scoped to unicast; multicast route forwarded intact | |
| `show-l2tp-tunnel-detail` | `test/plugin/show-l2tp-tunnel-detail.ci` | rescoped: L2TP tunnel detail only | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| BGP message Length | 19-4096 (65535 extended) | 4096 | 18 | 4097 |

## Files to Modify
- `internal/test/runner/runner_exec.go` - per-peer verdict instead of joined-string contains
- `internal/test/runner/peer_contract.go` - the per-peer success predicate lives here
- `test/plugin/forward-overflow-two-tier.ci` - 50 malformed frames
- `test/plugin/role-otc-unicast-scope.ci` - 2 malformed frames + MP_REACH flag
- `test/plugin/bgp-rs-reactor-fastpath.ci` - drop EOR expectations + stale comment
- `test/plugin/bgp-rs-reactor-fastpath-fallback.ci` - drop EOR expectations
- `test/plugin/show-l2tp-tunnel-detail.ci` - drop the vestigial BGP EOR expectation
- `internal/component/bgp/reactor/reactor_notify.go` - explicit decline path
- `internal/component/bgp/plugins/rs/server_withdrawal.go` - default arm must not drop
- `internal/component/bgp/reactor/peer_initial_sync.go` - reversed comment
- `rfc/short/rfc4271.md` - extract the eBGP prepend obligation
- `plan/known-failures/bgp-plugin-forward-overflow-two-tier.md` - darwin claim disconfirmed
- `plan/known-failures/` entry for 458 - same

## Files to Create
- `internal/test/runner/ci_fixture_test.go` - frame-length gate
- `internal/test/runner/peer_contract_test.go` - per-peer verdict tests (if absent)

## Implementation Steps

### Implementation Phases
1. **Phase 1: harness verdict** -- write the failing per-peer verdict test, implement,
   then MEASURE the blast radius by running the full `bgp plugin` suite before and
   after. Triage the newly-red set before going further.
2. **Phase 2: fixtures** -- add the frame-length gate as a failing test, repair the 52
   frames, confirm the gate goes green and 224/398 sessions survive.
3. **Phase 3: AS_PATH race** -- instrument the `reactor_notify.go:570` gate, reproduce
   test 91 to failure, attribute the miss, fix at the producer. `make ze-race-reactor`.
4. **Phase 4: EOR de-specification** -- drop the ordering assertions from 90/91, rescope
   458, run each 20x.
5. **Phase 5: seams, docs, records** -- fail-open branches, reversed comment, RFC
   extraction, known-failures corrections, full verification, closure.

### Failure Routing
| Failure | Route To |
|---------|----------|
| Phase 1 blast radius is large | STOP, report the newly-red list, triage before committing |
| Test fails wrong reason | Fix test setup, not the assertion |
| 3 fix attempts fail | STOP. Report all 3. Ask user. |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| D-1: the verdict requires EVERY check-mode peer to succeed | keep any-peer semantics and add a warning | A warning does not fail a test; `ai/rules/fail-closed-guards.md` requires the guard to deny or speak. Any-peer is precisely the vacuous-pass shape `peer_contract.go` exists to prevent |
| D-2: repair fixture bytes rather than relax the assertions they feed | delete the malformed tests | The tests describe real behaviour worth covering; only their input bytes are wrong. Declared lengths stay unchanged so the intended frame is recovered, not redefined |
| D-3: EOR ordering is de-specified in the tests, not enforced in the product | add a readiness gate so nothing precedes the EOR | RFC 4724 does not require it; `opQueue` holds structured `PeerOp` (`peer.go:111-118`), not wire bodies, so enforcing it needs a new per-peer wire queue, and widening `HoldWrites` would block KEEPALIVE (`session_write.go:53`) for up to 2.5s |
| D-4: the frame-length gate is a Go test over `test/**/*.ci`, not a runner-time check | validate at parse time in the runner | A parse-time check only fires when the test runs; a repo-wide gate catches a malformed fixture the moment it is added, including in suites nobody ran |

## Known Limitations
- The forwarding rail's missing `ShouldQueue()` guard is NOT fixed here; it gets its
  own spec. A forwarded withdraw can still overtake a queued announce.
- Phase 3 may find the AS_PATH miss is environmental rather than a code defect; if so
  the finding is recorded and AC-4 is re-scoped with explicit approval.

## RFC Documentation

Add `// RFC 4271 Section 5.1.2: "the local system prepends its own AS number"` above
the prepend branches (`forward_rs.go:357`, `reactor_api_forward.go:540`) if Phase 3
touches them. Add the missing `[RFC4271-5.1.2-N]` requirement row to
`rfc/short/rfc4271.md` per `ai/rules/rfc-compliance.md` extraction completeness.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-test` passes
- [ ] Registration over hardcoding respected

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for BGP message length
