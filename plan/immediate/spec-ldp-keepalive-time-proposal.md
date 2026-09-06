# Spec: ldp-keepalive-time-proposal

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-06 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**What the operator is promised.** The `keepalive-time` leaf of
`internal/plugins/ldp/yang/ze-ldp-conf.yang` carries the description "Session
keepalive interval.", the unit seconds, the range 1 to 65535, and the default 60.
A reader takes it as the KeepAlive Time Ze proposes to its peer and the period
that governs how fast a dead session is detected. RFC 5036 Section 3.5.3 defines
the field the leaf names: "Two octet unsigned non zero integer that indicates the
number of seconds that the sending LSR proposes for the value of the KeepAlive
Time." The same section states what the peer does with it: "The receiving LSR
MUST calculate the value of the KeepAlive Timer by using the smaller of its
proposed KeepAlive Time and the KeepAlive Time received in the PDU."

**What Ze did instead.** `parseLDPConfig` (`internal/plugins/ldp/register.go`)
wrote the value into `ldpConfig.KeepaliveTime`. That field had no reader. The
session was built by `startSessionForAdj` in the same file, which called
`NewSession` (`internal/plugins/ldp/session.go`) with no keepalive argument.
`NewSession` set `keepaliveTime: DefaultKeepaliveTime`, which is 60 seconds, and
`holdTime: 3 * DefaultKeepaliveTime`. `SendInit` in the same file encodes
`s.keepaliveTime` into the Common Session Parameters of the Initialization
message, so Ze proposed 60 seconds to every peer whatever the operator wrote.
`handleInit` then takes the smaller of the two proposals and sets the hold time
to three times the result, which is the RFC 5036 rule and which the operator
could not influence from Ze's side. An operator who set `keepalive-time 15` to
detect a dead peer inside 45 seconds got 60 seconds and a 180-second hold,
unless the peer happened to propose less. The `ze:help` beside the leaf stated
this, in the sentence "Ze proposes 60 seconds in every Initialization message and
reads this leaf nowhere, so a change here moves no timer." The `description` did
not.

**What closing it means.** The implementer either carries the configured value
into the session or refuses the leaf at commit, the way
`unimplementedVRFValidator` (`internal/component/config/validators.go`) refuses
the `vrf` leaf. Building it is the right answer, and the gap between the two is
small. The negotiation, the encoder, the timer and the hold-time derivation all
exist and are correct; only the configured value was missing from the path.
`startSessionForAdj` already receives several scalars taken from `ldpConfig`, so
the change is that path carrying one more value into `NewSession`. `NewSession`
already took eight parameters, so the design had to decide whether the value
arrives as a ninth or whether the constructor takes a settings struct. Refusing
the leaf would leave an operator with a fixed 60-second keepalive and a fixed
180-second hold on every LDP session, which is a real operational limit rather
than a schema cleanup. The design also had to state what happens to a session
that is already up when the leaf changes, because the negotiated value is a
property of the Initialization exchange and cannot be renegotiated without a new
session.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress.
     Capture what you learned as -> Decision: / -> Constraint: annotations, which
     survive compaction; track reading progress in the session state file. -->

### Architecture Docs
- [ ] `docs/architecture/ldp/mpls-ldp.md` - the page every file in the plugin declares in its `// Design:` header
  → Decision: discovery reconciles per interface and leaves a running interface's goroutine alone, so a reload reaches a running interface only through state that goroutine reads at the time it acts
  → Constraint: the discovery start function is an injected field, so the session-open path stays unit-testable with no multicast I/O
- [ ] `docs/guide/mpls.md` - the operator-facing page that documents the `ldp { }` block and its timers
  → Constraint: the page states the defaults 5s / 15s / 60s, so a change to what `keepalive-time` does is a change to this page

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfcNNNN.md` - `rfc/short/rfc5036.md`, the LDP summary; the authority read for this spec is `rfc/full/rfc5036.txt` Section 3.5.3
  → Constraint: the KeepAlive Time is "Two octet unsigned non zero integer", so 1 to 65535 seconds is everything the wire can carry
  → Constraint: "The receiving LSR MUST calculate the value of the KeepAlive Timer by using the smaller of its proposed KeepAlive Time and the KeepAlive Time received in the PDU" (Section 3.5.3), so the exchange is one-shot and a running session cannot renegotiate
  → Decision: RFC 5036 states no default KeepAlive Time anywhere in its text, so Ze's 60 seconds is a Ze choice and not a conformance question

**Key insights:** (minimal context to resume after compaction)
- The negotiation, the encoder, the hold-time derivation and the keepalive sender were all already correct. Only the operator's number was missing from the path.
- The value is negotiated once, in the Initialization exchange, so it is read once, when the session opens.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/plugins/ldp/register.go` - `parseLDPConfig` reads `keepalive-time` into `ldpConfig.KeepaliveTime`; `runLDPEngine` holds the delivered config in `activeCfg` under `mgrMu`; `startSessionForAdj` dials the peer and builds the session; the per-session goroutine sends a KeepAlive every `sess.currentKeepalive() / 3`
- [ ] `internal/plugins/ldp/session.go` - `NewSession` built every session with `DefaultKeepaliveTime`; `SendInit` encodes `uint16(s.keepaliveTime.Seconds())` into the Common Session Parameters; `handleInit` lowers `s.keepaliveTime` to the peer's value when the peer proposes less and sets `holdTime` to three times the result
- [ ] `internal/plugins/ldp/wire.go` - `initMessage.KeepaliveTime uint16`, written at `sessionBuf[2:4]` of the Common Session Parameters TLV and read back by `DecodeInit`
- [ ] `internal/plugins/ldp/yang/ze-ldp-conf.yang` - the leaf, its `range "1..65535"`, its `default 60`, and the `ze:help` that admitted the leaf was read nowhere

**Behavior to preserve:** (unless the user explicitly said to change it)
- `handleInit` keeps taking the smaller of the two proposals and deriving the hold time as three times it. That is the RFC 5036 Section 3.5.3 rule and it was already right.
- The `.ci` suite in `test/ldp/` boots a single daemon with no peer. `ldp-session`, `ldp-reload` and `ldp-convergence` keep passing unchanged.
- The default stays 60 seconds, so an operator who writes no `keepalive-time` sees no change.

**Behavior to change:** (only what the user asked for)
- The KeepAlive Time Ze proposes in its Initialization message is the configured value rather than the fixed 60 seconds.
- The pre-negotiation hold time is three times the configured value rather than a fixed 180 seconds.
- `NewSession` takes a named `SessionConfig` rather than eight positional parameters.
- The engine states the timers it starts with in one log line, so an operator can read back what the leaf did.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- The operator writes `ldp { keepalive-time 15 }` in the config file.
- The config component delivers the `ldp` subtree to the plugin as one JSON section whose leaves are strings: `{"ldp":{"keepalive-time":"15", ...}}`.

### Transformation Path
1. `parseLDPConfig` (`internal/plugins/ldp/register.go`) coerces the string leaf through `configNumber` into `ldpConfig.KeepaliveTime time.Duration`.
2. `runLDPEngine` stores the delivered config in `activeCfg`, guarded by `mgrMu`, and logs the timers it starts with.
3. `discoverOnInterface` forms an adjacency from a neighbor's Hello and calls back into the engine's start function.
4. The start function reads `activeCfg.KeepaliveTime` under `mgrMu` at that moment and hands it to `startSessionForAdj`.
5. `sessionConfigForAdj` builds a `SessionConfig` from the LSR-ID, the timer and the adjacency.
6. `NewSession` validates the value against what two octets can carry and stores it as `s.keepaliveTime`, with `s.holdTime` three times it.
7. `SendInit` writes `uint16(s.keepaliveTime.Seconds())` into the Common Session Parameters TLV of the Initialization message.
8. `handleInit` lowers `s.keepaliveTime` to the peer's proposal when the peer proposes less, and re-derives `s.holdTime`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config → Plugin | One JSON config section rooted at `ldp`, every leaf a string | Yes -- `TestConfiguredKeepaliveReachesInitializationMessage` parses the delivered JSON shape through `parseLDPConfig` |
| Plugin → Wire | The Common Session Parameters TLV of the Initialization message, two octets at TLV offset 2 | Yes -- `proposedKeepalive` reads the bytes back off a `net.Pipe` through `DecodeInit` |
| Ze → FRR | A real LDP session over a veth pair to FRR ldpd | Not run here -- `TestLDPInteropFRR` asserts it and FRR is not installed on this machine |

### Integration Points
- `NewSession` (`internal/plugins/ldp/session.go`) - takes `SessionConfig` instead of eight positional parameters; every existing call site names its fields.
- `startSessionForAdj` (`internal/plugins/ldp/register.go`) - takes the timer as one more scalar beside the LSR-ID and the transport address it already carried.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The value moves config → `parseLDPConfig` → `activeCfg` → `sessionConfigForAdj` → `NewSession` → `SendInit`. No other reader of `ldpConfig.KeepaliveTime` exists |
| No unintended coupling (components stay isolated) | Yes | Every changed file is under `internal/plugins/ldp/`. No component or core package is touched |
| No duplicated functionality (extends existing, does not recreate) | Yes | The negotiation, the encoder and the hold-time derivation are the ones that were already there. Nothing is reimplemented |
| Zero-copy preserved where applicable (refs, not copies) | Yes | `SendInit` still encodes into its own `[256]byte` stack buffer; no allocation is added on any path |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | No new command, view or handler. `SessionConfig` is a field on a plugin-local struct, not on a shared one |

## Risks & Assumptions

<!-- LIVE: written during RESEARCH/DESIGN, statuses updated during implementation.
     Gate answers from /ze-spec (assumption challenge, Failure Mode Analysis)
     land HERE, not only in conversation. -->

### Assumptions
<!-- Every row needs a validation method. `unvalidated` is not a valid final
     status: closure re-checks each one. A broken assumption also gets a
     Mistake Log row and a Deviations entry. -->
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `ldpConfig.KeepaliveTime` had no reader before this change | `grep -n KeepaliveTime internal/plugins/ldp/*.go`: the only non-test mentions were the field, the parser that writes it, and the wire struct of the same name | The gap is somewhere else and this change is not the fix | Grep over the package, plus the revert walk below | Confirmed |
| A-2 | RFC 5036 mandates no default KeepAlive Time, so Ze's 60 seconds is a free choice | `grep -i "180\|default.*keepalive" rfc/full/rfc5036.txt` returns nothing | Ze's default would be a conformance question rather than a config choice | Grep over the RFC text | Confirmed |
| A-3 | The discovery goroutine's captured `ldpConfig` goes stale across a reload that changes only a timer | `newDiscoveryManager.reconcile` starts a goroutine for an added interface and stops one for a removed interface; it leaves a running interface alone | The captured copy would be as good as `activeCfg` and the lock is unneeded | Reading `reconcile` in `internal/plugins/ldp/discovery_manager.go` | Confirmed |
| A-4 | A session already up cannot be moved to a new KeepAlive Time | RFC 5036 Section 3.5.3 carries the value in the Initialization message, which is exchanged one time per session | A reload could retime live sessions and the Known Limitation below is wrong | Reading Section 3.5.3 of `rfc/full/rfc5036.txt` | Confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | An operator sets a keepalive so low that a busy peer cannot answer inside three times it, and every session flaps | Sessions cycling through `StateOperational` and back in the log | The leaf keeps its YANG `range "1..65535"` and its 60-second default. Ze proposes; the peer's own floor still applies through the negotiation |
| R-2 | The two-octet field silently truncates a value it cannot carry, so the peer honors an interval nobody configured | A negotiated keepalive unrelated to the configured one | `NewSession` refuses a value outside 1..65535 seconds, warns with both numbers, and proposes the default. `TestSessionKeepaliveBoundaries` pins all four edges |
| R-3 | Reading `activeCfg` under `mgrMu` from the discovery callback deadlocks against the reconcile path that holds the same lock | A hung engine at reload | The callback takes the lock, copies one field, and releases it before calling `startSessionForAdj`. It calls nothing while holding it |

## Blast Radius

<!-- What a wrong landing costs, and how to get out. A reviewer reads this first. -->
| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | LDP sessions time out too fast or too slowly, and an MPLS LSP goes with the session. Nothing outside `internal/plugins/ldp/` changes behavior |
| How is it reverted? | A single commit revert. No config migration: the leaf, its range and its default are unchanged, so an existing config file parses identically before and after |
| Who else touches this path? | Nobody in this run. `spec-mpls-2-ldp` built the plugin and is closed. The `test/ldp/` suite and `docs/architecture/ldp/mpls-ldp.md` are LDP-only |

## Wiring Test (MANDATORY -- NOT deferrable)

<!-- BLOCKING: proves the feature is reachable from its intended entry point.
     Without it the feature exists in isolation: unit tests pass, nothing calls it.
     Every row needs a concrete test name. "Deferred"/"TODO"/empty is rejected
     by `internal/le/hookruntime/lifecycle.go`, which is the point: an unedited row fails. -->
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ldp { keepalive-time 15 }` in a config file a daemon boots on | → | `parseLDPConfig` → `runLDPEngine`, which reports the timers it starts with | `test/ldp/ldp-keepalive-time.ci` |
| The `ldp` config section as the plugin is handed it, in its delivered JSON string form | → | `parseLDPConfig` → `sessionConfigForAdj` → `NewSession` → `SendInit`, read back off the wire | `TestConfiguredKeepaliveReachesInitializationMessage` |
| A discovered adjacency with a live FRR ldpd peer | → | `startSessionForAdj` → `NewSession` → the negotiated value on an operational session | `TestLDPInteropFRR` |

## Acceptance Criteria

<!-- Define BEFORE implementation. Each row is a testable assertion, stated as
     observable behavior, never as the mechanism used to reach it. -->
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | An operator configures `keepalive-time 15` and Ze opens a session | The Initialization message Ze sends carries 15 in the KeepAlive Time field of its Common Session Parameters |
| AC-2 | The same session, before the peer's Initialization arrives | The session hold time is 45 seconds, three times the configured value, not the 180 seconds three times the default gives |
| AC-3 | The peer proposes a smaller KeepAlive Time than Ze | The session settles on the peer's smaller value and derives its hold time from that, unchanged from before this spec |
| AC-4 | A session is built with a KeepAlive Time two octets cannot carry: zero, or more than 65535 seconds | Ze logs the requested value and the value it will propose, then proposes the 60-second default. It never truncates to a number the peer would read as a different interval |
| AC-5 | A daemon boots with an `ldp { }` block | The engine reports the LSR-ID and the three timers it started with, so an operator can read back what each leaf did |
| AC-6 | An operator changes `keepalive-time` and reloads | Sessions that open after the reload propose the new value. A session that is already up keeps the value it negotiated, because RFC 5036 exchanges it one time |

## End-to-End User Stories

<!-- One row per user-facing operation the feature enables. ACs verify that
     components work; stories verify the chain is connected. A broken link in a
     path is a spec gap: add the missing component to ACs, Files, and Test Plan
     before proceeding. Delete this section when Scope is tooling or docs. -->
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | writes `keepalive-time 15` and boots the daemon | config file → YANG → config section → `parseLDPConfig` → `runLDPEngine` | `test/ldp/ldp-keepalive-time.ci` |
| 2 | wants a dead LDP peer declared dead inside 45 seconds rather than 180 | `sessionConfigForAdj` → `NewSession` → `SendInit` → the Common Session Parameters TLV | `TestConfiguredKeepaliveReachesInitializationMessage` |
| 3 | runs that daemon against another vendor's LSR | discovery → `startSessionForAdj` → `NewSession` → Initialization exchange with FRR ldpd → operational session | `TestLDPInteropFRR` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestConfiguredKeepaliveReachesInitializationMessage` | `internal/plugins/ldp/keepalive_test.go` | AC-1, AC-2: the delivered config JSON reaches the KeepAlive Time bytes of the Initialization message, and the hold time is three times it | Pass |
| `TestSessionConfigForAdjCarriesPeerIdentity` | `internal/plugins/ldp/keepalive_test.go` | The builder fills the peer identity from the adjacency, so a field added beside the timer cannot displace one | Pass |
| `TestSessionKeepaliveBoundaries` | `internal/plugins/ldp/keepalive_test.go` | AC-4 and the four edges of the range RFC 5036 Section 3.5.3 can carry | Pass |
| `TestSessionKeepaliveNegotiation` | `internal/plugins/ldp/session_test.go` | AC-3: the smaller of the two proposals wins. Pre-existing, kept green | Pass |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `keepalive-time` (YANG leaf, seconds) | 1-65535 | 65535 | 0 | 65536 |
| `SessionConfig.KeepaliveTime` (session builder) | 1s-65535s | 65535s | 0s | 65536s |

### Functional Tests
<!-- REQUIRED: a unit test proves the algorithm, a .ci proves the user can reach
     the feature. New RPCs/APIs are never covered by unit tests alone.
     Structure: ai/patterns/functional-test.md -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ldp-keepalive-time` | `test/ldp/ldp-keepalive-time.ci` | An operator writes `keepalive-time 15`, boots, and the engine starts on 15 seconds rather than falling back to the 60-second default | Pass |

### Interop Tests (Scope: protocol)
<!-- REQUIRED when wire-visible behavior changes. See
     ai/rules/interop-and-goal-validation.md, including the vacuity traps: prove
     the test FAILS when the behavior under test is reverted. -->
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `TestLDPInteropFRR` | `internal/plugins/ldp/frr_interop_integration_linux_test.go` (Go integration test, not a `test/interop/scenarios/` directory) | FRR (zebra + ldpd) in a child network namespace over a veth pair | FRR proposes its own 180-second default, so a session that settles on Ze's configured 15 seconds proves Ze put 15 on the wire. An engine reading the leaf nowhere proposes 60 and the assertion reads 60 | Assertion written and type-checked under `-tags integration`; NOT RUN, FRR is not installed on this machine |

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*), not only test files.
     Check each file's // Design: annotation: if the change alters behavior the
     referenced architecture doc describes, list that doc here too. -->
- `internal/plugins/ldp/session.go` - `SessionConfig`, `keepaliveTimeMax`, `NewSession` takes the struct and validates the range, `SendInit` states what it proposes
- `internal/plugins/ldp/register.go` - `sessionConfigForAdj`, `startSessionForAdj` takes the timer, the engine start log line, and the start function reading `activeCfg` under `mgrMu`
- `internal/plugins/ldp/yang/ze-ldp-conf.yang` - the `ze:help` that said the leaf was read nowhere
- `internal/plugins/ldp/adjacency_expiry_test.go`, `internal/plugins/ldp/rfc5036_test.go` - call sites moved to `SessionConfig`
- `internal/plugins/ldp/frr_interop_integration_linux_test.go` - configures 15 seconds and asserts the negotiated value
- `docs/architecture/ldp/mpls-ldp.md` - the decision that the timer is read when the session opens
- `docs/guide/mpls.md` - what `keepalive-time` does and when a change to it applies

## Files to Create
- `internal/plugins/ldp/keepalive_test.go` - the wiring and boundary tests
- `test/ldp/ldp-keepalive-time.ci` - functional test for end-user behavior

### Integration Checklist
<!-- Answer every row Yes / No / N-A. Never leave a bare marker: an unanswered
     row is indistinguishable from a forgotten one. N-A needs a reason. -->
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | The leaf, its type and its default already existed. Only its `ze:help` prose changed |
| YANG validation constraints | No | `range "1..65535"` was already the maximum native validation the leaf can carry |
| YANG custom validators | No | The range is expressible natively, so `ze:validate` buys nothing |
| CLI commands/flags | No | No command is added or changed |
| CLI grammar (keyword before value) | N-A | No command is added or changed |
| Editor autocomplete | No | A `uint16` leaf with a range needs no `CompleteFn` |
| Functional test for new RPC/API | N-A | No RPC or API is added. The operator path is covered by `test/ldp/ldp-keepalive-time.ci` |
| Pipe completeness | N-A | No command output is added |
| Env var registration | N-A | The leaf is not under `environment/` |
| Doctor check for runtime dependencies | No | No new file path, socket, port, module or binary. The existing port 646 check in `internal/plugins/ldp/doctor.go` is unaffected |
| Prometheus counters/metrics | No | No new observable state. The session counters `startSessionForAdj` already updates are unchanged |
| BGP family surface (new SAFI / capability / attribute) | N-A | LDP, not BGP |

### Documentation Update Checklist (BLOCKING)
<!-- Answer every row Yes / No / N-A. A No must be backed by a source-aware
     check, not a guess: at minimum grep docs/ for source anchors pointing at the
     files you changed. Any factual doc change carries a source anchor. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | A leaf that already existed now does what its description promised. `docs/features.md` claims no LDP timer behavior |
| 2 | Config syntax changed? | No | The `ldp { }` block, the leaf name, its range and its default are all unchanged |
| 3 | CLI command added/changed? | No | No command is added or changed |
| 4 | API/RPC added/changed? | No | No RPC is added or changed |
| 5 | Plugin added/changed? | Yes | `docs/guide/mpls.md` -- what `keepalive-time` does, and that a change applies to sessions opened after it |
| 6 | Has a user guide page? | Yes | `docs/guide/mpls.md`, updated |
| 7 | Wire format changed? | No | The Common Session Parameters TLV is byte-for-byte what it was. Only the value in it changes |
| 8 | Plugin SDK/protocol changed? | No | No SDK type, event or command crosses a boundary differently |
| 9 | RFC behavior implemented, changed, or newly proven? | No | The RFC 5036 Section 3.5.3 requirement is "the receiving LSR MUST calculate ... the smaller of the two", which `handleInit` already implemented and which this change does not touch. Ze's own proposal is a configuration choice the RFC leaves free: `grep -i "180\|default.*keepalive" rfc/full/rfc5036.txt` returns nothing, so the RFC names no default |
| 10 | Test infrastructure changed? | No | One `.ci` added to an existing registered suite, no runner change |
| 11 | Affects daemon comparison? | No | `docs/comparison.md` carries no LDP timer row |
| 12 | Internal architecture changed? | Yes | `docs/architecture/ldp/mpls-ldp.md` -- the decision that the timer is read at session open, from the active config |
| 13 | Route metadata keys added/changed? | No | No metadata key is added or changed |
| 14 | Prometheus counters added/changed? | No | No counter is added or changed |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | The plugin's registration, its events and its command set are unchanged |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `internal/plugins/ldp/register.go` and `internal/plugins/ldp/session.go` both declare `// Design: docs/architecture/ldp/mpls-ldp.md`. That page is named above and updated |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/mpls.md` shows an `ldp { }` example listing the three timers. Verified against the YANG: the leaf names, the defaults and the units all still agree |

## Implementation Steps

<!-- Concrete phases of work, not a restatement of the /ze-implement stages
     (those live in the skill). Phase 1 is ALWAYS wiring. Order by dependency:
     schema before resolution, resolution before CLI. Each phase follows TDD
     (write test -> fail -> implement -> pass) and ends with a self-critical
     review; fix what it finds before starting the next phase. -->

1. **Phase: Wiring (MANDATORY FIRST)** -- prove the operator's leaf reaches the code that opens a session
   - Tests: `test/ldp/ldp-keepalive-time.ci`, `TestConfiguredKeepaliveReachesInitializationMessage`
   - Files: `internal/plugins/ldp/register.go` (the engine start log line and the start function), `internal/plugins/ldp/keepalive_test.go`
   - Verify: the `.ci` fails on `keepalive-time=1m0s` while the leaf is not read, and the unit test reads 60 off the wire where 15 was configured
2. **Phase: Carry the value into the session** -- `SessionConfig`, the range check, and the call sites
   - Tests: `TestSessionKeepaliveBoundaries`, `TestSessionConfigForAdjCarriesPeerIdentity`
   - Files: `internal/plugins/ldp/session.go`, `internal/plugins/ldp/register.go`, the three test files whose call sites move to the struct
   - Verify: all four boundary cases hold, and the whole `internal/plugins/ldp` package stays green
3. **Phase: Interop and prose** -- assert the negotiated value against FRR, and correct the pages the change made wrong
   - Tests: `TestLDPInteropFRR`
   - Files: `internal/plugins/ldp/frr_interop_integration_linux_test.go`, `internal/plugins/ldp/yang/ze-ldp-conf.yang`, `docs/guide/mpls.md`, `docs/architecture/ldp/mpls-ldp.md`
   - Verify: the interop assertion type-checks under `-tags integration`, and no page still says the leaf is read nowhere

### Critical Review Checklist

<!-- Feature-SPECIFIC checks. The generic ones in ai/rules/quality.md always
     apply and are not repeated here. A row that would read the same on any spec
     is not worth a row. -->
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-1/AC-2 at `keepalive_test.go` `TestConfiguredKeepaliveReachesInitializationMessage`; AC-3 at `session.go` `handleInit`; AC-4 at `session.go` `NewSession`; AC-5 at `register.go` `runLDPEngine`; AC-6 at `register.go`, the start function reading `activeCfg` |
| Feature completeness | The chain config file → `parseLDPConfig` → `activeCfg` → `sessionConfigForAdj` → `NewSession` → `SendInit` has a test on every link and no gap between two of them |
| Correctness | The proposal is read before `ReadLoop` starts, so no peer message can race `SendInit` reading `s.keepaliveTime`; `handleInit` still lowers and never raises |
| Naming | `SessionConfig` field names match the wire names of RFC 5036 Section 3.5.3, and the YANG leaf, the JSON key and `ldpConfig.KeepaliveTime` all name one value |
| Data flow | The timer is read from `activeCfg` under `mgrMu` at session open, never from the discovery goroutine's captured copy |
| Rule: `ai/rules/principles.md` | `NewSession` never silently accepts an unencodable value: it says what it was given and what it will propose instead |

### Deliverables Checklist

<!-- Every deliverable with a command that proves it. "Looks done" is not a
     verification method. -->
| Deliverable | Verification method |
|-------------|---------------------|
| The configured value is on the wire | `./le job run label ldp-unit command go test -run TestConfiguredKeepaliveReachesInitializationMessage ./internal/plugins/ldp/` |
| The operator path reaches the engine | `./le functional ldp` |
| No call site still builds a session positionally | `grep -n "NewSession(" internal/plugins/ldp/*.go` -- every hit passes a `SessionConfig` |
| The interop assertion compiles | `go vet -tags integration ./internal/plugins/ldp/...` |

### Security Review Checklist

<!-- Feature-specific: untrusted input, injection, resource exhaustion, error
     leakage, authorization that could fail open. -->
| Check | What to look for |
|-------|-----------------|
| Input validation | The configured value is operator input, bounded twice: by the YANG `range "1..65535"` at commit, and by `NewSession` against what two octets carry. The peer's proposal is untrusted input and `handleInit` only ever lowers Ze's value with it, so a peer cannot lengthen Ze's hold time and hide a dead session |
| Resource exhaustion | A one-second keepalive is the floor the wire allows, so the fastest a peer can be made to send is one PDU per second per session. The keepalive goroutine's period is `currentKeepalive() / 3` with a one-second floor, so it cannot spin |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
<!-- LIVE: write immediately when you learn something. At closure these route to
     a subsystem arch doc, a rule, or the learned summary. -->
- A leaf parsed into a field is not a leaf that is read. `parseLDPConfig` wrote `KeepaliveTime` correctly and the value died there. The tell was that the only other mention of the name in the package was a wire struct field that happened to share it, which a grep for the name hides rather than shows.
- Where a reload changes only a scalar, the config copy a long-lived goroutine captured at start is the wrong source. `reconcile` deliberately leaves a running interface's goroutine alone, so anything that goroutine reads from its captured config is frozen at the moment the interface came up.
- The `.ci` suite for LDP boots one daemon with no peer, so the operator-facing proof stops at the engine. Saying that in the `.ci` header, and naming the unit test and the interop test that carry the other half, is what keeps the boundary honest rather than hidden.

## Key Design Decisions
<!-- "Chose X over Y because Z." The rejected alternative is the valuable half. -->
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| `NewSession` takes a `SessionConfig` struct | A ninth positional parameter | Eight positional parameters already included two `[4]byte` LSR-IDs and two `uint16` label spaces, which a caller can transpose with no compiler error. The struct names each at the call site, and the next leaf is a field rather than a tenth position |
| The timer is read from `activeCfg` under `mgrMu` when the session opens | Capture it in the discovery goroutine's `ldpConfig` at interface start | `reconcile` leaves a running interface's goroutine alone, so a reload that changes only a timer would reach no session |
| A session that is up keeps its negotiated value | Tear down and rebuild sessions when the leaf changes | RFC 5036 Section 3.5.3 exchanges the value one time. A bounce to apply a timer drops every LSP the peer carries, which is a large price for a timer change |
| An unencodable value is refused and the default proposed, with a warning naming both numbers | Clamp to the nearest encodable value | A clamp puts a number nobody configured on the wire, and the peer honors it silently. Refusing says so in the log and keeps the documented default |
| The engine states its timers in one log line at start | Add a `show ldp` field carrying the configured timers | The log line needs no new command surface and no peer, so the `.ci` suite, which boots one daemon with no peer, can read it |

## Known Limitations
<!-- Deliberate scope boundaries. Anything here that is actually outstanding work
     is not a limitation: write it as its own spec, in the bucket that item
     belongs to, and name that spec here (ai/rules/planning.md). -->
- A session that is already operational keeps the KeepAlive Time it negotiated when the leaf changes. This is RFC 5036 Section 3.5.3's exchange model rather than unbuilt work: the value travels in the Initialization message, which is sent one time per session.
- The FRR interop assertion is written and type-checks, and it was NOT run: FRR is not installed on the machine this work was done on, and no registered `./le` action runs `internal/plugins/ldp/frr_interop_integration_linux_test.go`. The wire half is proven instead by `TestConfiguredKeepaliveReachesInitializationMessage`, which reads the KeepAlive Time bytes back off a `net.Pipe`.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

| Site | Requirement |
|------|-------------|
| `keepaliveTimeMax` (`internal/plugins/ldp/session.go`) | RFC 5036 Section 3.5.3: "Two octet unsigned non zero integer that indicates the number of seconds that the sending LSR proposes for the value of the KeepAlive Time." |
| `NewSession` range guard (`internal/plugins/ldp/session.go`) | The same sentence: a value outside 1..65535 cannot be proposed, so it is refused rather than truncated |
| `SendInit` (`internal/plugins/ldp/session.go`) | RFC 5036 Section 3.5.3: the field "indicates the number of seconds that the sending LSR proposes for the value of the KeepAlive Time" |
| `handleInit` (`internal/plugins/ldp/session.go`, pre-existing) | RFC 5036 Section 3.5.3: "The receiving LSR MUST calculate the value of the KeepAlive Timer by using the smaller of its proposed KeepAlive Time and the KeepAlive Time received in the PDU." |

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

## Red-Then-Green Evidence

Both proofs were run on 2026-09-06 in this checkout. Each reverts the product
code, observes the red, then restores it and observes the green.

| Test | Revert applied | Observed RED | Restored GREEN |
|------|----------------|--------------|----------------|
| `TestConfiguredKeepaliveReachesInitializationMessage`, `TestSessionKeepaliveBoundaries` | `NewSession` builds the session with `keepaliveTime: DefaultKeepaliveTime` and `holdTime: 3 * DefaultKeepaliveTime`, the state before this spec | `FAIL`: proposed 0x3c where 0x0f was expected; keepalive 1m0s where 15s was expected; hold time 3m0s where 45s was expected. `lowest encodable` read 0x3c for 0x1 and `highest encodable` read 0x3c for 0xffff | `ok github.com/ze-software/ze/internal/plugins/ldp` |
| `test/ldp/ldp-keepalive-time.ci` | `parseLDPConfig` drops its `keepalive-time` branch, so the leaf reaches no field | `FAIL 2 ldp-keepalive-time`, `stderr does not contain "keepalive-time=15s"`; the engine logged `keepalive-time=1m0s`. The other three tests in the suite passed, so the red is specific to this one | `pass 4/4 100.0%` |
