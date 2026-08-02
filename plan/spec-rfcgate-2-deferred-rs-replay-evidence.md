# Spec: rfcgate-2-deferred-rs-replay-evidence

<!-- DESIGN-TIME template: everything that must exist BEFORE code is written.
     The closure half (Implementation Summary, Audit, Goal Validation, Review
     Gate, Pre-Commit Verification, Mistake Log) lives in
     plan/TEMPLATE-CLOSURE.md and is APPENDED by /ze-close at step 1.
     Do not copy it in advance: sections copied 300 lines ahead of their use
     reach closure untouched, the ones created when needed get filled. -->

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | 3/3 |
| Deferral shard | `-` (nothing deferred out of this spec yet) |
| Updated | 2026-08-01 |

<!-- Scope drives which optional blocks below apply. Say which one this is, so
     an absent section reads as "inapplicable" rather than "skipped".
     Deferral shard: every deferred item lands there (ai/rules/deferral-tracking.md)
     and closure must resolve its rows, so name the file from the start. -->

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Deferred out of `plan/spec-rfcgate-2-evidence.md` (see
`plan/deferrals/rfcgate-2-evidence.md`).

### The premise this spec was created on is REFUTED (2026-07-29)

~~**The defect.** Ze configured as a route server prepends its own AS to routes
it re-advertises through the adj-rib-in replay-on-peer-up path. RFC 7947 Section
2.2.2.1 says a route server should not insert itself into the AS_PATH, and Ze
honors that on the bgp-rs FORWARD rail; the REPLAY rail is the generic per-peer
re-advertisement path and does not know the peer belongs to a route server.~~

~~**The symptom, reproduced.** `python3 test/interop/run.py
47-rfc7606-relay-shape-frr` fails at Path 1 with `FAIL: AS 65001 found in
AS_PATH for 10.0.0.0/24`.~~

**There is no such defect.** Verified against the producing code, not the
symptom:

| Claim | Producing code | What it shows |
|-------|----------------|---------------|
| One prepend gate serves BOTH rails | `internal/component/bgp/reactor/reactor_api_forward.go:711` -- `if facts.isEBGP && !facts.rsClient` | The replay rail is not a second, RS-unaware path |
| Both rails reach that gate | `RelayStoredRoute` -> `forwardUpdateCore` (`reactor/reactor_api_relay.go:253`); `ForwardUpdate` -> `forwardUpdateCore` (`reactor/reactor_api_forward.go:358`) | The replay rail IS the forward rail below the entry point |
| `rsClient` has exactly one source | `reactor/peer_forward_facts.go:111` (`rsClient: s.RSClient`) <- `reactor/config.go:266` (the `session/rs-client` leaf) | Nothing else can set it |
| Its default is `false` | `internal/component/bgp/plugins/rs/yang/ze-rs-conf.yang:40-46` | An unconfigured peer is not an RS-client |
| No interop scenario sets it | `grep -rn 'rs-client' test/interop/` returned nothing **at the moment of the refutation** | Scenario 47's peer was, as configured, a plain eBGP peer |

→ Correction (2026-07-29, same day): that grep now returns ten hits. `rs-client true;`
was added to both peers of scenarios `14-route-server-frr` and `47-rfc7606-relay-shape-frr`
later the same day, each with an inline comment recording that the leaf defaults to false.
The row above is kept in the past tense because it is the EVIDENCE for the refutation, not
a live claim: re-running that grep today proves the fix landed, not that the diagnosis was
wrong. A future reader who runs it and finds hits should read this note rather than
concluding the refutation was mistaken.

So Ze prepended **correctly**. Scenario 47's earlier green depended on an RFC
4271 Section 5.1.2 bug that commit `8bb55e509` ("fix(bgp): prepend the local AS
on eBGP announces, RFC 4271 5.1.2", 2026-07-25) fixed; when the bug went away
the scenario's expectation stopped being satisfiable, and the resulting red was
misread as a new route-server defect. The general lesson is recorded in the
parent spec's Known Limitations: **a test that goes red after an unrelated fix
may have been green *because* of the bug.**

### The real gap, which is what this spec now owns

RFC 7947 Section 2.2.2.1's AS_PATH transparency obligation is proven **only by
unit tests**, and no test pins it through the REPLAY entry point at all:

| Fact | Evidence |
|------|----------|
| `RFC7947-x-1` has unit-only evidence | `ai/RFC-REQUIREMENTS.md:4040` -- positive `forward_rs_test.go:431` (unit/verify), negative `forward_rs_test.go:323` (unit/verify). No functional, editor or interop carrier |
| The one relay test asserts nothing about the AS_PATH | `internal/component/bgp/reactor/reactor_api_relay_test.go:92` (`TestRelayStoredRouteForwardsThroughForwardRail`) asserts the destination peer and `assert.NotEmpty(t, item.rawBodies)`, i.e. that the relay reached the forward pool and that buffers are released. It says nothing about the transform |
| No rs-client relay test exists | `grep -n 'RSClient\|rsClient' internal/component/bgp/reactor/reactor_api_relay_test.go` returns nothing: the relay fixture's destination is never an RS-client |
| The requirement is a SHOULD NOT, not a MUST | `rfc/short/rfc7947.md:49` -- `[RFC7947-x-1] [SHOULD NOT]`. The earlier framing as a "violation" of a MUST overstated it |

The gap is therefore **evidence, not behaviour**: nothing proves that a route
relayed to an rs-client peer through `RelayStoredRoute` is byte-identical in
AS_PATH to what arrived, and the parent spec's whole thesis is that a wire
obligation proven only in-process is proven at the wrong altitude.

### The work

1. Add an rs-client **relay** test pinning AS_PATH byte-identity through
   `RelayStoredRoute`, alongside the existing forward-rail coverage. Both rails
   share the gate at `reactor_api_forward.go:711`, so this is a test for a
   behaviour believed correct -- write it RED first by flipping the gate.
2. Decide the carrier for `RFC7947-x-1`'s non-unit evidence. A `.ci` with two
   rs-client peers is the parent spec's AC-18 default (a `.ci` runs on every
   push); an interop scenario with `rs-client` actually set is the stronger
   proof and the nightly-tier option. Do not pick interop by reflex.
3. Only then re-examine scenario 47. Its Path 1 assertion
   (`check_route_no_as("10.0.0.0/24", "65001")`) is testing route-server
   transparency **on a peer that is not configured as an RS-client**. Either the
   scenario config gains `rs-client`, or the assertion is wrong and must change.
   That decision belongs here, with the evidence, not in the tooling spec.
4. `RFC7606-5.1-3 positive` may then be tagged onto scenario 47 and
   mutation-verified RED per `ai/rules/interop-and-goal-validation.md`.

Note for whoever picks this up: `test/interop/scenarios/47-rfc7606-relay-shape-frr/check.py`
still carries a `# NOT YET TAGGED` header repeating the refuted premise. Correct
it in the same change; it was written before the refutation.

### Outcome of the four work items (2026-08-01)

| # | Item | Outcome |
|---|------|---------|
| 1 | rs-client relay test pinning AS_PATH through `RelayStoredRoute` | DONE, and widened. Both polarities added (`TestRelayStoredRouteRSClientPreservesASPath`, `TestRelayStoredRoutePlainEBGPPrependsLocalAS`), each mutation-killed. Writing it "RED first by flipping the gate" was done as an overlay mutation rather than a tree edit, because three sessions share this checkout |
| 2 | Decide the carrier for the non-unit evidence | DONE: a `.ci`, per `ai/rules/testing.md` (a `.ci` runs on every push; interop is nightly and advisory). `test/plugin/bgp-rs-relay-aspath-transparency.ci` carries BOTH polarities. The nightly interop binding on scenario 14 is retained, not replaced |
| 3 | Re-examine scenario 47's Path 1 assertion | ALREADY LANDED before this session. `ze.conf:30,45` now set `rs-client true` on both peers, so `check_route_no_as("10.0.0.0/24", "65001")` is testing an actual RS client. Verified by reading the file, not assumed |
| 4 | Tag `RFC7606-5.1-3 positive` onto scenario 47 | ALREADY LANDED before this session. The tag is at `check.py:28` and appears in the ledger row for `RFC7606-5.1-3` as `(interop/nightly)`. The `# NOT YET TAGGED` header noted above is gone, replaced by a header that records the refutation |

Items 3 and 4 were completed by a later commit than the one that filed this spec.
Nothing was left for this session to do on them beyond confirming they are real,
which is why neither appears in the Implementation Steps below.

## Required Reading

### RFC Summaries (Scope: protocol)
- `rfc/short/rfc7947.md` - the route-server RFC this spec supplies evidence for
  → Constraint: `RFC7947-x-1` is a **SHOULD NOT**, not a MUST. Section 2.2.2.1 says the RS
    should not prepend its own AS nor modify AS_PATH in any other way, and Section 2.2.2.2
    states outright that the weaker form exists only for clients that cannot accept a
    non-adjacent leftmost AS. Do not re-inflate it to MUST NOT (a 2026-07-23 correction
    already did that once).
  → Constraint: x-2 (NEXT_HOP) and x-3 (MED) are `{single-polarity: positive}` and are
    proven byte-identically by `TestReactorForwardRSTransparent`. This spec does not touch
    them.
- `rfc/full/rfc4271.txt` Section 5.1.2 - the obligation the RS exemption is carved out of
  → Constraint: an ordinary eBGP peer MUST still get the local AS prepended. Any change
    that suppresses the prepend must be confined to RS clients, which is why every positive
    assertion here is paired with a confining negative.

**Key insights:**
- The AS_PATH prepend gate `facts.isEBGP && !facts.rsClient` exists in **two** copies, one
  per forwarding rail. Which copy a test drives is decided by the SOURCE peer's
  `behavior/rs-fast-path` leaf.
- `facts.rsClient` has exactly one source: the `session/rs-client` YANG leaf, default false.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go` - `forwardUpdateCore` holds the
  prepend gate (`:724`) for the PLUGIN rail. Reached by `ForwardUpdate` (bgp-rs dispatch,
  `:358`) and by `RelayStoredRoute` (peer-up replay, `reactor_api_relay.go:253`).
- [ ] `internal/component/bgp/reactor/forward_rs.go` - `reactorForwardRS` holds a SECOND,
  textually identical gate (`:387`) for the reactor FAST PATH, taken when the source peer
  sets `behavior/rs-fast-path`.
- [ ] `internal/component/bgp/reactor/reactor_api_relay.go` - `RelayStoredRoute` reconstructs a
  stored route and hands it to `forwardUpdateCore` under the SOURCE peer's receive context.
- [ ] `internal/component/bgp/reactor/peer_forward_facts.go` - `rsClient: s.RSClient` is the
  only writer of the gate's input.
- [ ] `internal/component/bgp/plugins/rs/yang/ze-rs-conf.yang` - `rs-client`, `default false`.

**Behavior to preserve:**
- Everything. This spec adds no product code: the transform is already correct on both
  rails. It supplies the missing evidence.

**Behavior to change:**
- None. Tests only.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- A BGP UPDATE arrives on an established RS-client session as wire bytes, and a second
  client establishes later. Two distinct entry points follow from that:
  1. **Live relay** -- bgp-rs dispatches `ForwardCached`, which reaches `ForwardUpdate`.
  2. **Peer-up replay** -- bgp-rs replays adj-rib-in through `RelayStoredRoute`, whose
     input is a `rpc.StoredRoute` (hex `AttrHex` / `NextHopHex` / `NLRIHex`), not wire.

### Transformation Path
1. Wire parse into `WireUpdate` (`internal/component/bgp/wireu/`), or, on the replay path,
   `buildRelayUpdate` reconstructs an equivalent received-shape `ReceivedUpdate` from the
   stored hex and tags it with the SOURCE peer's receive `ContextID`.
2. `forwardUpdateCore` resolves per-destination `forwardFacts` and runs the ordered egress
   steps (export policy, in-process role/OTC/community filters).
3. **The gate** (`reactor_api_forward.go:724`): `facts.isEBGP && !facts.rsClient` selects
   `getEBGPWire`, which prepends the local AS. An RS client skips it, so `peerWire` stays
   the received wire and the body is written verbatim.
4. `buildFwdBody` emits the body; the per-peer forward pool writes it to the session.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Wire → engine | UPDATE parsed to `WireUpdate`, zero-copy into the read buffer | Yes -- `bgp-rs-relay-aspath-transparency.ci` drives real sessions end to end |
| Plugin ↔ engine | bgp-rs `ForwardCached` / `RelayStoredRoute` over DirectBridge | Yes -- the `.ci` runs bgp-rs as an internal plugin |
| Engine → wire | `buildFwdBody` + forward pool per-peer write | Yes -- the `.ci` asserts exact bytes at two peers |

### Integration Points
- `forwardUpdateCore` - the shared egress transform; both rails' entry points converge here.
- `bgp-rs` (`internal/component/bgp/plugins/rs/`) - the plugin that decides to forward.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The `.ci` reaches the gate through real sessions and the real plugin; mutating the gate reddens it |
| No unintended coupling (components stay isolated) | Yes | No product code changed; no new import |
| No duplicated functionality (extends existing, does not recreate) | Yes | Reuses `relayFixture` and `forwardBodyFindASPath`; adds no third fixture |
| Zero-copy preserved where applicable (refs, not copies) | Yes | Unchanged: the RS-client path still writes the received wire verbatim, which is what the byte-identity assertion pins |
| Registration over hardcoding | N-A | No command, view, family, or handler added |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The relay rail and the live-forward rail share ONE prepend gate | Spec premise, from `reactor_api_relay.go:253` and `reactor_api_forward.go:358` both calling `forwardUpdateCore` | The relay needs its own coverage independent of `ForwardUpdate` | Read both call sites | confirmed |
| A-2 | That shared gate is the SAME gate the existing `RFC7947-x-1` tests already prove | The spec's refutation section, which cites one gate | The tagged evidence does not cover the relay rail at all, and the gap is wider than "the relay entry point" | Mutation: flip the `forwardUpdateCore` gate, re-run the tagged tests | **broken** -- see Design Insights (carry to the Mistake Log "Wrong Assumptions" row at closure). `forward_rs.go:387` is a SECOND copy; both tagged x-1 tests and the pre-existing relay test stayed GREEN under both mutations |
| A-3 | `rs-client` is the only input to the gate, and defaults false | `peer_forward_facts.go:111`, `ze-rs-conf.yang` | A `.ci` could select the transparent path by accident and prove nothing | Read the producer; assert `forwardFacts().rsClient` as a test precondition | confirmed |
| A-4 | A `.ci` with `rs-fast-path` unset drives `forwardUpdateCore`, not `reactorForwardRS` | `reactorForwardRS` runs only for a source peer with `RSFastPath` (`forward_rs_test.go` `TestRSFastPathGateRespectsCapability`) | The new `.ci` would silently re-cover the already-covered rail | Mutation: the `.ci` reddens on the `forwardUpdateCore` gate and stays GREEN on the `forward_rs.go` gate | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Exact-hex `.ci` expectations break on an unrelated attribute-ordering change | The `.ci` fails with a byte diff at conn=2 or conn=3 | The frames are machine-checked for internal consistency (`tmp/check-ci-hex.py`), and the failure names the offending bytes. Byte-equality is deliberate: it is what proves "unchanged" |
| R-2 | A three-peer `.ci` is more load-sensitive than a two-peer one | Intermittent failure under `ze-verify` | Readiness is event-driven (`run_rs_observer(expected_peers=3)`), no sleeps; proven with 24 invocations under 32 burners at 8-way parallel |
| R-3 | The two gate copies drift apart again, silently | Only one of the two mutation sites reddens a test | Both copies now carry tagged coverage in both polarities; a future divergence reddens something |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing user-visible: no product code changes. A wrong test would assert the wrong wire and fail loudly rather than pass silently |
| How is it reverted? | Single commit revert (tests + ledger row) |
| Who else touches this path? | A concurrent session is editing `forward_build.go` / `forward_modify_failure.go` in the same package. Nothing in this spec touches those files |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| bgp-rs peer-up replay of adj-rib-in (`RelayStoredRoute`) to an RS-client destination | → | `forwardUpdateCore` prepend gate, RS-client branch | `TestRelayStoredRouteRSClientPreservesASPath` |
| The same replay to an ordinary eBGP destination | → | `forwardUpdateCore` prepend gate, `getEBGPWire` branch | `TestRelayStoredRoutePlainEBGPPrependsLocalAS` |
| Two RS clients establishing against a running daemon, one route forwarded to both | → | bgp-rs `ForwardCached` → `ForwardUpdate` → `forwardUpdateCore` | `test/plugin/bgp-rs-relay-aspath-transparency.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A stored route with AS_PATH `[64496 64512 64497]` is replayed through `RelayStoredRoute` to an eBGP peer with `rs-client true` | The peer receives AS_PATH `[64496 64512 64497]`, and the local AS 65000 appears nowhere in it |
| AC-2 | The same replay to an eBGP peer with `rs-client` left at its default | The peer receives AS_PATH `[65000 64496 64512 64497]` |
| AC-3 | A running daemon relays one received UPDATE (AS_PATH `[65001]`) to two established clients, one `rs-client true`, one not | The RS client receives the UPDATE byte-identical to the one sent; the ordinary peer receives it with AS_PATH `[65000 65001]` |
| AC-4 | The `forwardUpdateCore` prepend gate is mutated to prepend always, or never | At least one test added by this spec turns RED for each mutation |
| AC-5 | `RFC7947-x-1` is read from the generated ledger | Both polarities carry `functional/verify` evidence, so neither is nightly-only |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs Ze as an IXP route server; a client's session flaps and re-establishes | peer-up → bgp-rs replay → `RelayStoredRoute` → `forwardUpdateCore` → wire | `TestRelayStoredRouteRSClientPreservesASPath` |
| 2 | Peers a router with Ze-as-route-server and checks the received AS_PATH shows the origin AS, not the IXP's | wire → bgp-rs → `ForwardUpdate` → `forwardUpdateCore` → wire | `test/plugin/bgp-rs-relay-aspath-transparency.ci` (conn=2) |
| 3 | Peers an ordinary eBGP session with the same daemon and expects normal transit semantics | same path, `rs-client` unset | `test/plugin/bgp-rs-relay-aspath-transparency.ci` (conn=3) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRelayStoredRouteRSClientPreservesASPath` | `internal/component/bgp/reactor/reactor_api_relay_test.go` | AC-1: replay to an RS client leaves AS_PATH byte-identical; tagged `RFC7947-x-1 positive` | done |
| `TestRelayStoredRoutePlainEBGPPrependsLocalAS` | `internal/component/bgp/reactor/reactor_api_relay_test.go` | AC-2: the exemption is confined to RS clients; tagged `RFC7947-x-1 negative` | done |

Both reuse `relayFixture` and read the AS_PATH the destination actually received
via `relayBodyASPath` (a new helper over the existing `forwardBodyFindASPath`).
"Before" is decoded from the stored `AttrHex`, so each test states the transform
on both sides rather than asserting a shape.

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A: this spec adds no numeric input | - | - | - | - |

The relay payload size boundary is already covered by `TestRelayPayloadSizeBoundary`.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgp-rs-relay-aspath-transparency` | `test/plugin/bgp-rs-relay-aspath-transparency.ci` | AC-3: an operator running Ze as a route server sees an RS client receive the origin's AS_PATH untouched, while an ordinary eBGP neighbour on the same daemon receives the normal prepend | done |

Deliberately does NOT set `rs-fast-path`: that leaf routes the UPDATE to the
other copy of the gate, which already had coverage (A-4).

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `14-route-server-frr` | `test/interop/scenarios/` | FRR | A real FRR sees no 65001 in the AS_PATH of a route relayed by Ze-as-route-server. Already tagged `RFC7947-x-1 positive` (interop/nightly) | pre-existing, unchanged |
| `47-rfc7606-relay-shape-frr` | `test/interop/scenarios/` | FRR | Relay shape under RFC 7606 Section 5.1; carries the `RFC7606-5.1-3 positive` tag | pre-existing, already corrected -- see Deviations |

No NEW interop scenario: `ai/rules/testing.md` prefers a `.ci` when the behaviour
is reachable from both, because a `.ci` runs on every push and interop is nightly
and advisory. The nightly binding is kept, not replaced (the evidence ratchet is
per tier, so removing it would fail `make ze-rfc-check`).

## Files to Modify
- `internal/component/bgp/reactor/reactor_api_relay_test.go` - two tagged tests plus the
  `relayBodyASPath` / `storedRouteASPath` / `relayDispatchedBody` helpers
- `ai/RFC-REQUIREMENTS.md` - regenerated (`make ze-rfc-index`); one row changes

No product code is modified: the transform was already correct on both rails.
This spec closes an EVIDENCE gap, which is why every new test was written to fail
first under a mutation of the code it claims to pin.

## Files to Create
- `test/plugin/bgp-rs-relay-aspath-transparency.ci` - the verify-tier functional carrier,
  drafted in `test/draft/plugin/` and promoted once green and load-proven

### Integration Checklist
<!-- Answer every row Yes / No / N-A. Never leave a bare marker: an unanswered
     row is indistinguishable from a forgotten one. N-A needs a reason. -->
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No config surface added. `rs-client` and `rs-fast-path` already exist and are unchanged |
| YANG validation constraints | N-A | No new leaf |
| YANG custom validators | N-A | No new leaf |
| CLI commands/flags | No | No command added or changed |
| CLI grammar (keyword before value) | N-A | No command |
| Editor autocomplete | N-A | No new leaf |
| Functional test for new RPC/API | Yes | `test/plugin/bgp-rs-relay-aspath-transparency.ci` -- no new RPC, but the behaviour needed a verify-tier carrier |
| Pipe completeness | N-A | No command output |
| Env var registration | N-A | No `environment/` leaf |
| Doctor check for runtime dependencies | No | No file path, socket, port, module, binary, or certificate introduced |
| Prometheus counters/metrics | No | No new observable state; the transform is asserted on the wire instead |
| BGP family surface (new SAFI / capability / attribute) | No | No family, capability, or attribute added. The route is plain ipv4/unicast |

### Documentation Update Checklist (BLOCKING)
<!-- Answer every row Yes / No / N-A. A No must be backed by a source-aware
     check, not a guess: at minimum grep docs/ for source anchors pointing at the
     files you changed. Any factual doc change carries a source anchor. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | No feature added; behaviour unchanged |
| 2 | Config syntax changed? | No | `rs-client` / `rs-fast-path` pre-exist and are untouched |
| 3 | CLI command added/changed? | No | None |
| 4 | API/RPC added/changed? | No | None |
| 5 | Plugin added/changed? | No | bgp-rs unchanged |
| 6 | Has a user guide page? | No | No behaviour change to document |
| 7 | Wire format changed? | No | The emitted wire is byte-for-byte what it was; that is the assertion |
| 8 | Plugin SDK/protocol changed? | No | None |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes -- newly proven, no edit needed | `docs/features/rfc-status.md:50` already states "Transparent RS forwarding for RS clients: no AS_PATH prepend..." at Status `Supported`. The claim was already accurate and its wording does not depend on evidence tier, so strengthening the proof leaves it correct. Verified by reading the row, not assumed. `rfc/short/rfc7947.md` needs no edit either: no requirement was added, retired, or re-levelled, so `check_retired_requirements` and `check_gap_count_agreement` stay satisfied (`make ze-rfc-check` green) |
| 10 | Test infrastructure changed? | No | Used the existing draft incubator and the existing `relayFixture`; added no runner, format, or target |
| 11 | Affects daemon comparison? | No | No capability gained or lost |
| 12 | Internal architecture changed? | No | No product code changed |
| 13 | Route metadata keys added/changed? | No | None |
| 14 | Prometheus counters added/changed? | No | None |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | None |
| 16 | Any changed source file referenced by existing doc source anchors? | No | The only non-test file changed is the generated `ai/RFC-REQUIREMENTS.md`. `grep -rn 'source: internal/component/bgp/reactor/reactor_api_relay' docs/` returns nothing, and no product source file was modified |
| 17 | Existing docs show config/CLI/API examples for this area? | No | The `rs-client` examples in `docs/` describe the leaf's meaning, which is unchanged |

## Implementation Steps

<!-- Concrete phases of work, not a restatement of the /ze-implement stages
     (those live in the skill). Phase 1 is ALWAYS wiring. Order by dependency:
     schema before resolution, resolution before CLI. Each phase follows TDD
     (write test -> fail -> implement -> pass) and ends with a self-critical
     review; fix what it finds before starting the next phase. -->

1. **Phase 1: Wiring** -- pin the entry points that had no transform coverage
   - Tests: `TestRelayStoredRouteRSClientPreservesASPath`,
     `TestRelayStoredRoutePlainEBGPPrependsLocalAS`
   - Files: `internal/component/bgp/reactor/reactor_api_relay_test.go`
   - Verify: each test RED under the mutation of the gate it claims to pin, GREEN on the
     real tree. The entry points already exist, so the "failing stub" step is replaced by
     the mutation, which is the stronger form of the same proof.
2. **Phase 2: Verify-tier functional carrier** -- prove it through the daemon
   - Tests: `test/plugin/bgp-rs-relay-aspath-transparency.ci`
   - Files: drafted in `test/draft/plugin/`, promoted to `test/plugin/`
   - Verify: RED on both mutations of the `forwardUpdateCore` gate, GREEN on a mutation of
     the `forward_rs.go` gate (which proves WHICH rail it drives); load-proven; full
     `plugin` suite green before promotion is claimed.
3. **Phase 3: Ledger** -- publish the evidence
   - Files: `ai/RFC-REQUIREMENTS.md`
   - Verify: `make ze-rfc-index` then `make ze-rfc-check` (every ratchet, including the
     per-tier evidence ratchet)

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-1/AC-2 at `reactor_api_relay_test.go`; AC-3 at the `.ci`; AC-4 by the mutation logs; AC-5 by the regenerated `RFC7947-x-1` row |
| Feature completeness | Each of the three user stories names a test that fails when its own behaviour is broken |
| Correctness (the real risk here) | Does each new test FAIL when the gate it names is mutated? A test asserting a non-empty body, or a shape rather than a value, is the exact failure this spec exists to correct -- do not accept one |
| Rail attribution | Does the `.ci` redden on the `forwardUpdateCore` gate and stay green on the `forward_rs.go` gate? If it reddens on both, it is not proving the uncovered rail |
| Polarity confinement | Is every "no prepend" assertion paired with a peer that DOES get the prepend in the same run? An unpaired positive passes under a never-prepend mutation |
| Tag correctness | Do the `RFC requirement:` tags quote a SHOULD NOT, not a MUST NOT? Re-inflating x-1 was already corrected once (`rfc/short/rfc7947.md` note, 2026-07-23) |
| Rule: `ai/rules/testing.md` | The `.ci` was drafted in `test/draft/plugin/` and promoted, not iterated in place |
| Rule: `ai/rules/rfc-compliance.md` | Nothing was classified `{gap}`, deferred, or annotated past. Evidence was ADDED at every tier; the nightly interop binding is retained |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Two tagged relay-rail unit tests | `go test -tags "ze_core $TAGS" -run 'TestRelayStoredRoute(RSClientPreservesASPath\|PlainEBGPPrependsLocalAS)' ./internal/component/bgp/reactor/` |
| Both mutation-killed | `python3 tmp/mutate-rfcgate2.py` -- expects ALL MUTATIONS KILLED |
| Verify-tier `.ci` | `ze-test bgp plugin --pattern bgp-rs-relay-aspath-transparency` |
| `.ci` proven to drive the uncovered rail | `python3 tmp/mutate-ci-rfcgate2.py` -- RED/RED on `forwardUpdateCore`, GREEN on `forward_rs.go` |
| No regression in the suite | `make ze-plugin-test` (full suite, not a sample) |
| Ledger published and gated | `make ze-rfc-index && make ze-rfc-check` |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | N-A: no new input path. The `.ci` frames are fixtures, and the relay's malformed-input refusals are already covered by `TestRelayStoredRouteRejectsMalformedInput` |
| Fail-open risk | The gate is a policy branch, not a guard: neither branch grants access. The failure it protects against is a wire-visible conformance defect, which is what both polarities now pin |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior; if misunderstood → RESEARCH |
| Lint failure | Fix inline; if architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

**The gap was one rail wider than this spec described, and assumption A-2 is how it
was found.** The spec said "both rails share the gate at `reactor_api_forward.go`",
which is true of the two rails it names (`RelayStoredRoute` and `ForwardUpdate`).
It is not true of the rail the EXISTING evidence drives. `reactorForwardRS`
(`forward_rs.go:387`) carries a second, textually identical copy of
`facts.isEBGP && !facts.rsClient`, and every pre-existing `RFC7947-x-1` binding
drives that copy: the two tagged unit tests call `reactorForwardRS` directly, and
both tagged-adjacent `.ci` files enable `rs-fast-path`.

Measured, not argued: mutating the `forwardUpdateCore` copy to prepend always, and
then to prepend never, left `TestReactorForwardRSTransparent` (the x-1 positive),
`TestReactorForwardRSEBGPPrepend` (the x-1 negative) and
`TestRelayStoredRouteForwardsThroughForwardRail` all GREEN. So RFC 7947 AS_PATH
transparency could be reverted on the plugin rail and on every peer-up replay with
the whole suite passing.

**A duplicated gate needs duplicated evidence.** The lesson generalises past this
requirement: when a conformance obligation is enforced at two sites, tagging one
site is not tagging the requirement. The tag names the requirement, so it reads as
covering the behaviour rather than the copy it happens to sit on.

**"Asserts a non-empty body" is the shape of a vacuous test.** The pre-existing
relay test asserted the destination peer and `assert.NotEmpty(t, item.rawBodies)`.
Both survive any transform. The fix is to assert the VALUE on both sides of the
transform (`before` decoded from the stored attributes, `after` decoded from the
dispatched body), which is what makes the mutation kill it.

**A positive without its confining negative passes under the opposite mutation.**
"No prepend for an RS client" is satisfied by a daemon that prepends for nobody.
Each polarity here is asserted in the same run against a peer that differs only in
`rs-client`, so neither can pass by accident.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Assert the parsed AS_PATH ASN list, "before" and "after" | Exact-hex body equality in the unit tests | The value IS the requirement, and it survives incidental attribute-order changes. Exact hex is still used in the `.ci`, where byte-identity is the user-visible claim |
| Add a `.ci` on the plugin rail rather than a new interop scenario | A third FRR scenario with `rs-client` set | `ai/rules/testing.md`: a `.ci` runs inside `ze-verify` on every push, interop is nightly and advisory. The existing nightly binding (scenario 14) is KEPT, not replaced -- the evidence ratchet is per tier |
| Deliberately omit `rs-fast-path` from the new `.ci` | Mirror `bgp-rs-fastpath-ebgp-shared.ci` and enable it | That leaf routes the UPDATE to the OTHER copy of the gate, which already had coverage. Enabling it would have silently reproduced existing coverage while appearing to add some |
| Put both polarities in ONE `.ci` with three peers | Two `.ci` files, one per polarity | One source UPDATE reaching two destinations that differ only in `rs-client` makes the confinement the test's structure rather than a claim spread across two files |
| Verify mutations with `go -overlay` and copies under `tmp/` | Edit the gate in place, run, revert | Three sessions share this checkout. An in-place edit would have compiled into their runs. This is also why the sibling's broken `forward_build.go` was worked around with an overlay rather than fixed |

## Known Limitations
- The interop scenario for x-1 (`14-route-server-frr`) remains nightly-tier and advisory;
  this spec does not change that, it adds a verify-tier carrier beside it.
- The new `.ci` proves the plugin rail. The reactor fast path keeps its own pre-existing
  coverage. Neither rail is now unproven, but they are proven by different tests, which is
  the honest reflection of there being two gates. Unifying the two copies is a product
  change, not an evidence change, and is not attempted here.
- `RFC7947-x-4` (per-client import/export policy) keeps its existing unit-only bindings;
  this spec was scoped to x-1 and did not widen to it.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-verify` passes (the pre-commit gate; `ai/rules/git-safety.md`)
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
