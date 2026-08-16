# Spec: improve-3 -- Protocol Event Capture and Replay

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 5/5 |
| Updated | 2026-08-04 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `plan/spec-improve-0-umbrella.md` -- set context
4. `plan/deterministic-simulation-analysis.md` -- prior research on replay/determinism
5. `internal/component/bgp/reactor/session_read.go` -- where inbound messages enter

## Task

When a production BGP session misbehaves, Ze has no way to capture what the peer sent
and feed it back into the same state machine on a developer's desk. The global event
ring stores only timestamp/namespace/event-type (a counter trail, not a reproduction
artifact), and adj-rib-in "replay" re-announces stored routes to a peer, which is a
different feature. Bug reproduction currently means reconstructing peer behavior by
hand.

Add an opt-in, per-session JSONL capture of protocol input events, plus a replay
command that feeds a captured stream back into the same processing path with a
deterministic clock. Start narrow: BGP session inbound messages (wire bytes + arrival
metadata) and config transaction events. Capture is off by default and enabled per
peer or globally via config; files are bounded. The existing research in
`plan/deterministic-simulation-analysis.md` (state capture, clock injection) feeds
this design; this spec implements the capture/replay slice only, not full
deterministic simulation.

## Required Reading

### Architecture Docs
- [ ] `plan/deterministic-simulation-analysis.md` - Sections on state capture and clock control
  → Decision: adopt only the Option-D clock-injection slice (Phase 1 of its roadmap) + event capture; the FSM event queue, fault injection, and scheduler layers stay in the analysis doc (read by research agent 2026-07-10; digest in tmp/session/session-state-improve-3-event-replay-56997.md)
  → Constraint: full timer determinism per that doc needs an event queue; replay asserts FSM/RIB outcomes, not exact interleaving (see A-2)
- [ ] `docs/architecture/core-design.md` - session/reactor layering
  → Constraint: Session is owned by Peer; clock wiring flows Peer -> Session at `runOnce` (`peer_run.go`, `Session.SetClock` `session.go`); capture writer must be reactor-owned, format package a leaf (re-verify doc at implementation)
- [ ] `ai/rules/performance.md` - capture path must not allocate per message on hot path
  → Constraint: capture writer uses pooled buffers; disabled capture costs one nil check
- [ ] `ai/rules/config.md` - capture enable knob placement (YANG vs env)
  → Decision: per-peer YANG leaf (operator-facing, per user story 1: operator enables on a live box); read rule in full at implementation for the leaf's naming/validation

### RFC Summaries (MUST for protocol work)
- No new wire behavior; RFC 4271 message handling is exercised, not changed.

**Key insights:**
- Ze already streams synthetic UPDATEs in-process (`ze-test peer --mode inject`);
  replay closes the loop with REAL captured traffic.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/component/plugin/server/event_ring.go` - `EventRecord` holds Timestamp/Namespace/EventType only (:13-17); `Append` stores those three fields (:47-49); useful as a trail, not for reproduction
- [ ] `internal/component/bgp/plugins/adj_rib_in/rib_commands.go` - `replayCommand` re-sends STORED ROUTES from other peers to a target peer (:279-304); route replay, not event replay
- [ ] `internal/component/bgp/reactor/session_read.go` - `readAndProcessMessage` (:57): header read :74, body read :118-125; complete wire message in `buf.Buf[:hdr.Length]` before `processMessage` (:132-134); pooled-buffer lifecycle :59-71 (read directly 2026-07-10)
  → Constraint: tee copies bytes AFTER body read completes and BEFORE processMessage may take buffer ownership (`kept`); capture must never retain the pooled buffer
- [ ] `internal/component/bgp/reactor/session_coalesce.go` - `readAndProcessCoalesced` (:53) is a SECOND independent read path (own header read :69, body :107, slice :119) and coalescing is DEFAULT ON (`ze.bgp.reactor.coalesce=true`) -- A-1 resolved: TWO tee points (research agent)
- [ ] `internal/component/bgp/reactor/reactor_notify.go` - `notifyMessageReceiver` (:218) sees every message from both paths BUT post-RFC7606 short-circuit (`session_read.go,:216`), so the existing observer hook cannot serve raw capture; mrt observes there (`mrt/component.go` via `reactor.MessageObserver` `reactor.go`) (research agent)
- [ ] `internal/component/bgp/reactor/raw_capture.go` - `BGPRawCaptureRing` (:36): in-memory 256x4096 ring, truncates >4096, pcap-snapshot feature (`EnableRawCapture` `reactor.go`) -- adjacent, not reusable for persistent JSONL (research agent)
- [ ] `internal/core/clock/clock.go` + reactor clock chain - `Clock` (:18); `Peer.clock` (`peer.go`), `SetClock` (`peer.go`), `Session.SetClock` (`session.go`), wired `peer_run.go`; grep: ZERO raw time.* in non-test reactor code (research agent)
  → Constraint: verify `internal/bgp/fsm/timer.go` (older path flagged by the analysis doc) during implementation; reactor session/peer path is already clean for deterministic replay
- [ ] `internal/component/bgp/reactor/reactor.go` + `operation.go` - config entry points to capture: `ReconcilePeersWithJournal` (`reactor.go`, called from `bgp/plugin/register.go`) and `ApplyConfigOperation` (`operation.go`, dispatch :33-40) (research agent)
- [ ] `internal/component/config/transaction/orchestrator.go` - txID + phase states (:43-45, :94, :150) enrich captured config events with transaction identity (research agent)

**Behavior to preserve:** (unless user explicitly said to change)
- Zero hot-path cost when capture is disabled (single nil/flag check).
- Event ring, adj-rib-in replay, and `ze-test peer` inject mode unchanged.
- No change to message processing semantics under capture.

**Behavior to change:** (only if user explicitly requested)
- None; capture and replay are additive, opt-in features.

## Data Flow (MANDATORY)

### Entry Point
- Capture: inbound BGP message bytes at BOTH read paths, after each complete
  message is read (`readAndProcessMessage` post-body-read `session_read.go`;
  `readAndProcessCoalesced` post-slice `session_coalesce.go`); config events
  at the reactor boundary (`ReconcilePeersWithJournal`, `ApplyConfigOperation`)
  tagged with orchestrator txID when present.
- Replay: `ze test replay <capture-file>` (ze-test subtree, registerRoot pattern
  `internal/test/cli/register.go`) feeding a session instance in a harness
  process with `SetClock(FakeClock)` + stub `net.Conn`, not the live daemon.
  ~~`ze bgp replay`~~ superseded 2026-07-10: the harness is test infrastructure
  (fake clock, stub conn) and belongs with `ze-test peer`/inject, keeping the NOS
  CLI clean; a developer replays on a dev machine, matching the ze-test host-binary
  family.

### Transformation Path
1. Capture enabled per peer via config: session tees each complete inbound message (header + body bytes, arrival timestamp, peer identity) to a JSONL writer.
2. Writer appends one JSON object per event to a bounded per-session file (size cap + rotation), off the hot path via a buffered channel or equivalent.
3. Config transaction events (verify/apply/commit/rollback with txID) append to the same format under their own namespace.
4. Replay reads the JSONL stream, constructs a session with a deterministic clock and a stub connection, and feeds the captured bytes through the SAME `readAndProcessMessage` path.
5. Replay output (FSM transitions, RIB effect, NOTIFICATIONs) is observable through existing show/diag surfaces for comparison.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Session hot path ↔ capture writer | non-blocking hand-off, drop-with-counter on overflow | [ ] |
| Capture file ↔ replay harness | JSONL schema versioned in the file header line | [ ] |
| Replay ↔ session | stub net.Conn + injected clock (existing `s.clock` seam) | [ ] |

### Integration Points
- `readAndProcessMessage` (`session_read.go`, tee after :125) AND
  `readAndProcessCoalesced` (`session_coalesce.go`, tee after :119) - two tee points.
- Clock chain `Peer.SetClock` (`peer.go`) -> `Session.SetClock` (`session.go`)
  -> wired `peer_run.go`; `FakeClock` (`internal/test/sim/sim.go`) - replay determinism.
- `Reactor.ReconcilePeersWithJournal` (`reactor.go`) + `ApplyConfigOperation`
  (`operation.go`) - config event sources; orchestrator txID (:94) as metadata.
- Replay observation: `Dispatcher.Dispatch` (`server/command.go`), adj-rib-in
  show commands (`rib_commands.go`), FSM history (`peer_run.go`).
- spec-improve-4 conformance fixtures consume this capture format as their event-stream input.

### Architectural Verification
- [ ] No bypassed layers (replay uses the real read/process path, not a parallel decoder)
- [ ] No unintended coupling (capture writer owned by reactor; format pkg shared with replay tool)
- [ ] No duplicated functionality (extends event trail; does not replace event ring)
- [ ] Zero-copy preserved (capture copies bytes once at tee point, only when enabled)
- [ ] Registration over hardcoding -- replay CLI registers via existing dispatch (`ai/rules/plugins.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | ~~One tee point sees all inbound bytes~~ RESOLVED broken as anticipated: coalescing is DEFAULT ON and has its own read path | `readAndProcessCoalesced` (`session_coalesce.go`, header :69, body :107) verified by research agent | - | Design now specifies TWO tee points (Data Flow); shared tee helper so they cannot drift | confirmed (two tees adopted) |
| A-2 | The injected clock seam is sufficient for deterministic replay of timer-driven behavior | grep: zero raw time.* in non-test reactor code; clock chain `peer.go` -> `session.go` -> `peer_run.go` | Replay diverges on hold/keepalive timing; need the analysis doc's event-queue layer | Prototype replay of a captured session with timer expiry; verify `internal/bgp/fsm/timer.go` (older path flagged by analysis doc) | unvalidated (basis strengthened) |
| A-3 | JSONL per-message capture keeps up at stress rates when enabled | buffered writer design | Capture must sample or be documented as debug-rate only | Stress test with `ze-test peer --mode inject` during implementation | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Capture files contain operator config/routing data (sensitive) | design review | document handling; store under a diag directory with clear ownership; no auto-upload |
| R-2 | Format churn breaks old captures | first schema change | version field in header line; replay rejects unknown versions with a clear error |
| R-3 | Scope creep into full deterministic simulation | design review | this spec = capture + single-session replay only; simulation stays in the analysis doc |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| peer config enables capture | → | shared tee helper in BOTH read paths writes JSONL | TestSessionCaptureWritesEvents (parameterized: coalesced on/off) |
| ze test replay <file> | → | replay harness drives session read path with FakeClock | test/replay/bgp-capture-replay.ci |
| config commit with capture on | → | reactor-boundary config events appended with txID | TestTransactionEventCapture |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Capture disabled (default) | No file writes; hot path unchanged (benchmark-guarded) |
| AC-2 | Capture enabled, peer sends OPEN/KEEPALIVE/UPDATE | Each message appears as one JSONL event with bytes + metadata |
| AC-3 | Replay of a captured session | Same FSM transitions and RIB effect as the original run (deterministic clock) |
| AC-4 | Capture file reaches size cap | Rotation/stop per config; daemon unaffected |
| AC-5 | Replay of a truncated/corrupt file | Clear error naming the offending line; no panic |
| AC-6 | Config transactions during capture | verify/apply/commit/rollback events with txID recorded |
| AC-7 | Peer sends a malformed UPDATE (RFC 7606 treat-as-withdraw path) with capture on | Raw bytes captured BEFORE enforcement short-circuits (tee placement guarantees this); replay reproduces the same 7606 handling |
| AC-8 | Capture enabled with coalescing on (default) and off | Both paths produce identical capture streams for identical input |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Operator hits a session bug, enables capture, reproduces, ships the file | capture -> JSONL -> developer replays -> same failure observed | test/replay/bgp-capture-replay.ci |
| 2 | Developer bisects a fix against a captured stream | replay before/after fix | test/replay/bgp-capture-replay.ci |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| TestSessionCaptureWritesEvents | `internal/component/bgp/reactor/capture_test.go` | tee correctness, bytes round-trip | |
| TestCaptureFormatRoundTrip | capture format package test | encode/decode, version handling | |
| TestReplayDrivesSession | replay harness test | captured stream -> FSM transitions | |
| TestTransactionEventCapture | `internal/component/config/transaction/` test | tx events recorded | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| capture file size cap (MiB, YANG leaf) | 1-1024, default 100 | 1024 | 0 | 1025 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| bgp-capture-replay | `test/replay/bgp-capture-replay.ci` | capture a session, replay it, compare outcome | |

### Interop Tests (MANDATORY for protocol features)
- No new wire behavior; capture of an FRR/BIRD peer session can piggyback on an
  existing interop scenario during implementation (decide at design).

## Files to Modify
- `internal/component/bgp/reactor/session_read.go` - tee point (post :125)
- `internal/component/bgp/reactor/session_coalesce.go` - second tee point (post :119)
- `internal/component/bgp/reactor/reactor.go` / `operation.go` - config event emission at reconcile/apply entry points
- BGP peer YANG schema - capture enable knob (per-peer leaf per config-surface decision)
- `internal/test/cli/register.go` - `registerRoot("replay", cmdReplay, ...)` (ze-test subtree)

## Files to Create
- `internal/component/bgp/reactor/capture_replay.go` - capture writer (reactor-owned). ~~`capture.go`~~ (renamed in plan 2026-07-22: `reactor/capture.go` now already exists as an unrelated diagnostic message-capture ring, Design: learned/673, landed for diag-4 -- the planned JSONL writer must not clobber it)
- capture format package under `internal/core/` (leaf tier: imported by both reactor
  and ze-test replay; exact name at implementation per `ai/rules/architecture.md`) - JSONL schema + version header
- `internal/test/cli/cmd_replay.go` + harness (Session + FakeClock + stub conn, feeds `ReadAndProcess` `session_read.go`)
- `test/replay/bgp-capture-replay.ci` - functional test

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | Yes | BGP peer schema: capture enable (boolean) + size-cap leaf (per `ai/patterns/config-option.md`) |
| YANG validation constraints | Yes | size cap `range 1..1024`, default 100 (Boundary Tests table) |
| YANG custom validators | N/A | native constraints suffice |
| CLI commands/flags | Yes | `ze test replay` via `internal/test/cli/register.go` registerRoot |
| CLI grammar (action before identifier) | Yes | verify `test replay <file>` against `ai/rules/cli.md` at implementation |
| Editor autocomplete | N/A | automatic for boolean/range leaves |
| Functional test for new RPC/API | Yes | `test/replay/bgp-capture-replay.ci` |
| Pipe completeness | N/A | replay harness output is a test-tool report, not a NOS CLI command (confirm at implementation) |
| Env var registration | N/A | YANG leaf chosen (config-surface decision above) |
| Doctor check for runtime dependencies | Yes | capture directory writability when capture enabled (file-path dependency): owning-package check + `internal/core/diagnostic/codes.go` + tests per `ai/rules/repo-maintenance.md` |
| Prometheus counters/metrics | Yes | capture-drop counter (writer backpressure drops, Data Flow boundary row); name + labels listed at implementation |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` (capture/replay) |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` (peer capture leaves) |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` (`ze test replay`) |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | reactor + ze-test only |
| 6 | Has a user guide page? | No | features + command-reference suffice; revisit if a debugging guide exists at implementation |
| 7 | Wire format changed? | No | capture observes, never changes wire behavior |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | No | RFC 4271/7606 exercised, not changed |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` (new test/replay suite + harness) |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` (record/replay capability) |
| 12 | Internal architecture changed? | No | additive tee + leaf format package |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | Yes | metrics doc page for the capture-drop counter |
| 15 | Registered plugin, event type, send type, command, capability, or runtime inventory changed? | Yes | command + doctor inventory rows per `ai/rules/repo-maintenance.md` |
| 16 | Any changed source file is referenced by existing doc source anchors? | Check at implementation | grep `docs/` for anchors on session_read/session_coalesce |
| 17 | Existing docs show config/CLI/API examples for this area? | No | none exist yet |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** - capture knob + writer skeleton + replay command registered; failing wiring tests
2. **Phase: capture format + session tee** (including coalesced path)
3. **Phase: replay harness** with deterministic clock via existing seam
4. **Phase: transaction event capture**
5. Functional test, stress check (A-3), `make ze-precommit-verify`, learned summary, two-commit closure

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-1..AC-8 with file:line |
| Correctness | replay uses the real processing path; no parallel decoder |
| Performance | disabled capture adds no allocation on hot path (`ai/rules/performance.md`) |
| Registration over hardcoding | replay command registered via dispatch registry (`ai/rules/plugins.md`) |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | replay parses untrusted files: bounds, version, no panic on corrupt input |
| Resource exhaustion | file size caps, writer backpressure drops with counter |
| Data sensitivity | captured routing data documented; operator-controlled location |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| Doc checklist row 3: the replay command is `ze test replay`, so it belongs in `docs/guide/command-reference.md` | It landed as `ze-test replay`, a root of the ze-test binary. `command-reference.md` documents the `ze` dispatch only (`cmd/ze/main.go`) | Documentation review at implementation | Row 3 is N/A. The command is documented in `docs/functional-tests.md`, "Replaying a captured BGP session", beside the other ze-test tool roots |
| Doc checklist row 10: a new `test/replay/` suite | The `.ci` went into the existing `test/plugin` suite, so no suite was added | Implementation | Row 10 is satisfied by the CLI Reference addition, not a suite inventory row |
| A capture file is written once per PEER | `startCapture` runs once per `runOnce`, so it is once per CONNECTION ATTEMPT | Independent review, 2026-08-04 | `O_TRUNC` erased the capture of the session that failed, seconds after it failed. Fixed: a new session moves the previous file to `<file>.1` |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

## Design Insights
- Holo primary-source verification (2026-07-10, umbrella A-1 for this finding): its
  production `EventRecorder` serializes every instance message to JSONL
  (`holo-protocol/src/event_recorder.rs:30-65`) and `holo-replay` feeds the file
  back through the same harness (`holo-tools/holo-replay/src/main.rs:17-32`); the
  same format seeds its conformance tests. Ze's adaptation keeps the shape
  (JSONL + virtualized time) but tees raw wire bytes pre-enforcement, which Holo
  does not need (it records typed events post-decode).
- Replay asserts OUTCOMES (FSM transitions via `Peer.history` `peer_run.go`,
  RIB effect via dispatch show commands), not goroutine interleavings -- exact
  interleaving reproduction needs the analysis doc's event-queue layer, explicitly
  out of scope (R-3).

## Capture Format (v1) -- field enumeration (added 2026-07-10 at design gate, per user request)

One JSON object per line, kebab-case keys (`ai/rules/cli.md`). Bytes are
base64 (JSONL-safe). `seq` is a per-file monotonic counter so truncation is
detectable (AC-5).

**Header line (first line of every file):**
| Field | Type | Content |
|-------|------|---------|
| format | string | literal "ze-capture" |
| version | int | 1; replay rejects any other value (R-2) |
| peer | string | peer address (`s.settings.Address`) |
| started | string | RFC3339Nano from `s.clock.Now()` |
| daemon-version | string | ze version string |
| coalesce | bool | whether the coalesced read path was active |

**Event lines (common fields):**
| Field | Type | Content |
|-------|------|---------|
| seq | uint64 | monotonic per file, starts 1 |
| ts | string | RFC3339Nano from `s.clock.Now()` at the tee |
| type | string | "message" / "config" / "session" |

**type=message:** `direction` ("recv"; v1 captures inbound only), `msg-type`
(uint8 BGP message type from `hdr.Type`), `len` (uint16 wire length), `data`
(base64 of the FULL wire message including header, `buf.Buf[:hdr.Length]`),
`source-id`/`ctx-id` (when set on the session, `session_read.go` context).

**type=config:** `op` ("reconcile" / "add-peer" / "modify-peer" / "remove-peer",
mirroring `ApplyConfigOperation` dispatch `operation.go`), `tx-id` (orchestrator
transaction ID or empty), `payload` (the operation's JSON as delivered).

**type=session:** `event` ("connect" / "disconnect" / "capture-start" /
"capture-stop" / "drops"), `drops` (cumulative dropped-event counter, emitted when
the writer sheds under backpressure so replay knows the stream has a gap).

Rotation: when the size cap (YANG leaf) is reached, rotate once to `<file>.1` or
stop per config; a rotated or stopped capture emits a final "capture-stop" event.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Capture wire bytes, not decoded structs | decoded-event capture (serialize the internal message types) | bytes replay through the real decoder and survive internal refactors |
| Tee at the two read points post-body-read, NOT at the message-observer hook | reuse `reactor.MessageObserver`/`AddMessageObserver` (mrt's hook) | the observer fires post-RFC7606 short-circuit (`session_read.go,:216`) and post-decode -- it misses exactly the malformed inputs a bug capture exists to record |
| Replay hosts in the ze-test subtree (`ze test replay`) | `ze bgp replay` in the NOS CLI | harness needs FakeClock + stub net.Conn (test infra); ze-test already hosts the peer/inject harness family; NOS CLI stays operator-only |
| Config events captured at the reactor boundary (`ReconcilePeersWithJournal`, `ApplyConfigOperation`), txID as metadata | orchestrator-level phase capture only | single-session replay needs the peer-scoped operations the reactor actually applies; orchestrator phases lack per-peer granularity |
| Timestamps from `s.clock.Now()` at the tee | wall-clock `time.Now()` | keeps capture consistent with the injected-clock world so replay time math is uniform (mrt stamps wall clock at `mrt/component.go` -- plugin-side, not a precedent for reactor code) |

## Known Limitations
- Single-session replay only; multi-peer/topology replay and full deterministic
  simulation remain in `plan/deterministic-simulation-analysis.md` scope.
- A capture file is named for the peer ADDRESS (`captureFileName`), while the
  reactor keys peers by `AddrPort`. Two peers configured on one address with
  different ports write the same file, and the second to start moves the first
  aside. Raised in review as N13. It needs the file name to carry the port, and
  that changes a name an operator and the `.ci` both spell, so it is separable
  work rather than part of this spec.

## Implementation Summary

### What Was Implemented
- **Format package** `internal/core/capture` (leaf tier, standard library plus
  `internal/core/redact`): `capture.go` names the schema, `writer.go` is a
  bounded JSONL encoder that refuses a line WHOLE rather than crossing the cap,
  `reader.go` validates the header and the per-file sequence and names the
  offending line in every error.
- **Capture writer** `internal/component/bgp/reactor/capture_replay.go`:
  `sessionCapture` (one long-lived writer goroutine per capture, pooled items,
  shed-on-full queue), `Session.teeCapture`, `Peer.startCapture` /
  `stopCapture`, and `Reactor.CaptureConfigEvent` over the live-capture set.
- **Tee points** `session_read.go` and `session_coalesce.go`, both on the
  complete wire message before anything consumes it (AC-7, AC-8).
- **Config events** `internal/component/bgp/plugin/register.go` and
  `operation.go` emit verify / commit / rollback / add-peer / modify-peer /
  remove-peer with the transaction id; `reactor.go` emits reconcile (AC-6).
  Payloads pass `redact.JSON`, which was added for this and now backs command
  redaction too.
- **Config surface** `ze-bgp-conf.yang` container `capture`
  (`enabled`, `directory`, `maximum-size` range 1..1024, `on-limit`), parsed by
  `parseCaptureSettings` (`config.go`), defaulted in `NewPeerSettings`.
- **Replay** `internal/test/cli/cmd_replay.go` drives `Session.ReadAndProcess`
  over a stub `net.Conn` and a `FakeClock`. No parallel decoder: prefixes come
  off the `WireUpdate` the real path built (AC-3). The session identity comes
  from the capture header (`replayIdentity.resolve`), so an iBGP capture replays
  as iBGP; the three flags are overrides.
- **Bounds** every line the writer emits is one its own reader accepts
  (`WriteConfig` against `MaxLineLen`), the rotation retry is bounded to one
  attempt, and a new session moves the previous session's file aside rather than
  truncating it.
- **Observability** counter `ze_bgp_capture_dropped_events_total`
  (`reactor_metrics.go`), `ze doctor` check `doctor-bgp-capture-directory`
  (`internal/component/doctor/checks_bgp_capture.go`, landed earlier).
- **Tests** AC-1 and AC-2 and AC-4 through AC-8 in
  `capture_replay_test.go` and `internal/core/capture/*_test.go`, AC-3 and AC-5
  in `cmd_replay_test.go`, end to end in `test/plugin/bgp-capture-replay.ci`
  (mutation-verified 2026-08-03), and two benchmarks pinning zero allocation on
  the disabled and the enabled tee.

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

### Run 1 (initial)

Two independent reviewers over the diff, 2026-08-04. Neither wrote the code.
Reviewer A: the format package, the writer, `redact.JSON`. Reviewer B: the
wiring, the ACs, the replay harness, the `.ci`. They agreed on B1 and B3.

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| B1 | BLOCKER | Unbounded recursion between `write` and `atLimit`: an event larger than an empty file rotates for ever, destroying both generations each turn and ending the daemon on a stack overflow | `capture_replay.go` `atLimit` | fixed: the retry is bounded to one attempt (`writeItem(it, rotated)`) |
| B2 | BLOCKER | The writer emits a config line longer than `MaxLineLen`, which its own reader refuses; the reader stops at the FIRST long line, so one oversized reconcile costs every later event | `writer.go` `WriteConfig` | fixed: an oversized payload is replaced by a marker naming the dropped size |
| B3 | BLOCKER | Every reconnect truncated the previous session's capture, so the file recording the failure was erased seconds later (user story 1) | `capture_replay.go` `openFile` | fixed: `newSessionCapture` moves the previous file to `<file>.1` |
| B4 | BLOCKER | AC-6 wiring untested: `CaptureConfigEvent`, `registerCapture` fan-out and the txID hand-off had no test caller | `capture_replay_test.go` | fixed: `TestReactorCaptureConfigEventReachesOpenCaptures` |
| I5 | ISSUE | `markDrops` advanced the counter before the write, so a refused drops line left the stream claiming there was no gap | `capture_replay.go` `markDrops` | fixed: the counter advances only on success |
| I6 | ISSUE | `mapToJSON(bgpTree)` ran on every config reload even with no capture open | `reactor.go` `ReconcilePeersWithJournal` | fixed: guarded on the new `Reactor.CapturesOpen`, applied at both call sites |
| I7 | ISSUE | The AC-1 benchmarks only `ReportAllocs`; nothing read the number, so an added allocation failed no gate | `capture_replay_test.go` | fixed: `TestSessionTeeCaptureDisabledDoesNotAllocate` asserts `AllocsPerRun == 0` |
| I8 | ISSUE | The header carried no session identity, so replay invented the AS numbers and an iBGP capture replayed as eBGP | `capture.go` `Header`, `cmd_replay.go` | fixed: header records local-as, peer-as, router-id; flags became overrides |
| I9 | ISSUE | The package doc claimed a capture holds no secret; redaction is a name heuristic | `capture.go` package doc | fixed: the doc states the mechanism and its bound |
| I10 | ISSUE | Doc checklist rows 3, 10, 11 unaddressed | `docs/` | fixed for 10 and 11; row 3 is N/A, see Wrong Assumptions |
| N11 | NOTE | `openFile` left a zero-byte file when the header write failed | `capture_replay.go` `openFile` | fixed: the file is removed |
| N12 | NOTE | A verify event is recorded before the verify runs, so a rejected verify looks accepted | `plugin/register.go` | acknowledged: the event records the operation as SUBMITTED, which is what a replay needs; stated in `CaptureConfigEvent`'s doc |
| N13 | NOTE | `captureFileName` keys on the address while the reactor keys peers by `AddrPort` | `capture_replay.go` | acknowledged, open: see Known Limitations |
| N14 | NOTE | `CaptureConfigEvent` redacts once per open capture | `capture_replay.go` | acknowledged: cold path, and moving redaction out of `recordConfig` would take it off the path the redaction test drives |

Found while fixing N12, missed by both reviewers: inserting `parseCaptureSettings`
split `parseTTLSettings`'s RFC 5082 doc comment, leaving the citation on the wrong
function. Restored (`config.go`).

### Fixes applied
- `capture_replay.go`: bounded rotation retry, `rotateAside` on a new session, `markDrops` advances only on success, `CapturesOpen`, empty file removed on header failure.
- `writer.go`: `WriteConfig` bounds an oversized payload against `MaxLineLen`.
- `capture.go`: header carries local-as / peer-as / router-id; the secret claim states its mechanism.
- `reactor.go`, `plugin/operation.go`: the expensive payload is built only when a capture is open.
- `cmd_replay.go`: `replayIdentity` resolves override, then header, then fallback.
- `config.go`: corrected the zero-cap comment, restored the RFC 5082 comment.
- Five tests added, each mutation-verified (see `tmp/capture-land/mutation.log`).

### Run 2+ (re-runs until clean)
<!-- Add a new block per re-run. Final run MUST show zero BLOCKER/ISSUE. -->
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-standard-test` passes

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior
