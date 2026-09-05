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
- [ ] `docs/architecture/testing/ci-format.md` - the `.ci` test file format: embedded files, options, expectations and commands
- [ ] `plan/spec-improve-3-event-replay.md` - this spec: protocol event capture and replay
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
| A-2 | The injected clock seam is sufficient for deterministic replay of timer-driven behavior | grep: zero raw time.* in non-test reactor code; clock chain `peer.go` -> `session.go` -> `peer_run.go` | Replay diverges on hold/keepalive timing; need the analysis doc's event-queue layer | Prototype replay of a captured session with timer expiry; verify `internal/bgp/fsm/timer.go` (older path flagged by analysis doc) | confirmed for message-driven replay; timer EXPIRY is Work Not Done (see Assumptions Resolved) |
| A-3 | JSONL per-message capture keeps up at stress rates when enabled | buffered writer design | Capture must sample or be documented as debug-rate only | Stress test with `ze-test peer --mode inject` during implementation | broken; the design removed the need (see Assumptions Resolved) |

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
5. Functional test, stress check (A-3), `./le verify current mode full`, learned summary, two-commit closure

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
| A-3: the writer must keep up at inject rates, so a stress run is what validates it | The writer is not required to keep up at all. `offer` sheds on a full queue, counts the loss, and `markDrops` writes the gap into the stream, so a replay reads a gap as a gap | Closure gate, 2026-09-05, reading `sessionCapture.offer` | A-3 is resolved BROKEN rather than validated. The stress run it asked for is not owed, and no code relies on the claim it made |
| A best-effort path may drop the error it could not act on | A dropped `json.Marshal` error left a config event with no payload, which is also how "this phase carries no detail" is spelled, so the file could not tell a lost payload from an absent one | Closure gate review round 2, 2026-09-05 | Fixed: `captureBGPConfigEvent` logs the error with the operation and the transaction id before it drops the payload |

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

### Bugs Found/Fixed
- Every reconnect truncated the previous session's capture, so the file that
  recorded a failure was erased seconds later. `newSessionCapture` now moves the
  previous file to `<file>.1` (`TestSessionCapturePreservesThePreviousSession`).
- `write` and `atLimit` recursed without a bound, so an event larger than an
  empty file rotated for ever and ended the daemon on a stack overflow. The
  retry is bounded to one attempt
  (`TestSessionCaptureStopsWhenAnEventCannotEverFit`).
- `WriteConfig` emitted a line its own `Reader` refuses, and the reader stops at
  the FIRST long line, so one oversized reconcile cost every later event
  (`TestWriterBoundsAnOversizeConfigPayload`).
- `markDrops` advanced the counter before the write, so a refused drops line
  left the stream claiming there was no gap. The counter advances on success only.
- `captureBGPConfigEvent` discarded the `json.Marshal` error and wrote no
  payload, which is also how a config event with no detail is spelled. The error
  is now logged with the operation and the transaction id
  (`internal/component/bgp/plugin/operation.go`, `captureBGPConfigEvent`).

### Documentation Updates
- `docs/comparison.md`: the "Session capture and replay" row and its paragraph,
  anchored `<!-- source: internal/component/bgp/reactor/capture_replay.go -- sessionCapture, teeCapture -->`.
- `docs/features.md`: the `doctor-bgp-capture-directory` code in the `ze doctor` row.
- `docs/functional-tests.md`: "Replaying a captured BGP session", the `ze-test replay`
  usage line and the `-` stdin form, anchored on `test/plugin/bgp-capture-replay.ci`.
- `internal/component/bgp/yang/ze-bgp-conf.yang`: the `capture` container carries
  a `ze:help` per leaf, which is where the config syntax is documented.
- `./le doc check verify` FAILS on this tree, on findings that touch no file of
  this spec: the `ze-bgp-conf:bgp/defaults/attribute` AIGP summary over char-cap
  and word-cap, and four source anchors naming `getHelpExtension`, `Node.Help`
  and `answerZeroTunnelIDSCCRQ`. Several sessions share this checkout
  (`ai/rules/principles.md`), and those belong to another one.

### Deviations from Plan
- The replay command landed as `ze-test replay`, a root of the ze-test binary,
  not `ze test replay` under the `ze` dispatch. `docs/guide/command-reference.md`
  documents the `ze` dispatch only, so the command is documented in
  `docs/functional-tests.md` beside the other ze-test tool roots.
- The `.ci` landed in the existing `test/plugin` suite rather than a new
  `test/replay/` one, so no suite was added.
- A-3 was not validated as written. The design removed the need for it: see the
  Mistake Log row and Assumptions Resolved.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Opt-in per-session JSONL capture of protocol input events | Done | `internal/core/capture` (`Writer`, `Reader`), `reactor/capture_replay.go` (`sessionCapture`) | off by default, `CaptureSettings.Enabled` |
| Capture BGP inbound wire bytes with arrival metadata | Done | `Session.teeCapture` (`capture_replay.go`), called from `session_read.go` and `session_coalesce.go` | the full message, 19-byte header included |
| Capture config transaction events | Done | `Reactor.CaptureConfigEvent` (`capture_replay.go`), `captureBGPConfigEvent` (`bgp/plugin/operation.go`), `ReconcilePeersWithJournal` (`reactor.go`) | verify, commit, rollback, add-peer, modify-peer, remove-peer, reconcile |
| Replay command feeding the same processing path with a deterministic clock | Done | `runReplay` (`internal/test/cli/cmd_replay.go`) drives `Session.ReadAndProcess` under `sim.NewFakeClock` | no parallel decoder |
| Enabled per peer via config | Done | `container capture` (`ze-bgp-conf.yang`), `parseCaptureSettings` (`reactor/config.go`) | per peer; no AC asked for a global knob |
| Files are bounded | Done | `capture.Writer` limit (`writer.go` `flush`), `sessionCapture.atLimit` | one file at the cap, two under rotate |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestSessionTeeCaptureDisabledDoesNotAllocate` asserts `AllocsPerRun == 0`; `BenchmarkSessionTeeCaptureDisabled` | a disabled capture is one nil compare |
| AC-2 | Done | `TestSessionCaptureWritesEvents`; the `.ci` barrier `capture-has-messages` | OPEN, KEEPALIVE and UPDATE each one line |
| AC-3 | Done | `TestReplayDrivesSession`; `.ci` seq=6 asserts `ESTABLISHED` and `announce=[10.0.0.0/24]` | prefixes come off the real path's `WireUpdate` |
| AC-4 | Done | `TestSessionCaptureRotatesAtLimit`, `TestSessionCaptureStopsAtLimit` | both `on-limit` values |
| AC-5 | Done | `TestReplayRejectsCorruptCapture`, `TestReplayRejectsUnknownVersion`, `TestReaderCorruptInput` | every error names the line |
| AC-6 | Done | `TestSessionCaptureRecordsConfigEvents`, `TestReactorCaptureConfigEventReachesOpenCaptures` | the txID is carried from the plugin callback |
| AC-7 | Done | `TestSessionCaptureRecordsPreEnforcementBytes` | the tee sits ahead of RFC 7606 enforcement |
| AC-8 | Done | `TestSessionCaptureIdenticalAcrossReadPaths` | both read paths, identical stream |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestSessionCaptureWritesEvents | Done | `internal/component/bgp/reactor/capture_replay_test.go` | the plan named `capture_test.go`; that name was already taken by the diagnostic ring |
| TestCaptureFormatRoundTrip | Changed | `internal/core/capture/writer_test.go` `TestWriterRoundTripMessage` and `TestWriterRoundTripConfigAndSession`; `reader_test.go` `TestReaderRejectsUnknownVersion` | split by event kind rather than one test |
| TestReplayDrivesSession | Done | `internal/test/cli/cmd_replay_test.go` | |
| TestTransactionEventCapture | Changed | `TestSessionCaptureRecordsConfigEvents` and `TestReactorCaptureConfigEventReachesOpenCaptures` (`capture_replay_test.go`) | the events are emitted at the reactor boundary, so the test lives there and not under `config/transaction` |
| Boundary: capture size cap 1..1024 | Done | `TestParseCaptureSettingsBoundaries` (`capture_replay_test.go`) | 0 and 1025 refused, 1 and 1024 accepted |
| bgp-capture-replay (functional) | Done | `test/plugin/bgp-capture-replay.ci` | PASS in 7.0s, case 101 of 741 in `./le functional plugin`, 2026-09-05 |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/bgp/reactor/session_read.go` | Done | tee in `readAndProcessMessage`, after the body read |
| `internal/component/bgp/reactor/session_coalesce.go` | Done | tee in `readAndProcessCoalesced`, at the same logical point |
| `internal/component/bgp/reactor/reactor.go` and `bgp/plugin/operation.go` | Done | config event emission, guarded on `CapturesOpen` |
| BGP peer YANG schema | Done | `container capture` in `internal/component/bgp/yang/ze-bgp-conf.yang` |
| `internal/test/cli/register.go` | Changed | registered as the `ze-test replay` root, not `ze test replay` |
| `internal/component/bgp/reactor/capture_replay.go` | Done | 708 lines |
| capture format package under `internal/core/` | Done | `internal/core/capture` |
| `internal/test/cli/cmd_replay.go` | Done | 392 lines |
| `test/replay/bgp-capture-replay.ci` | Changed | landed as `test/plugin/bgp-capture-replay.ci` |

### Audit Summary
- **Total items:** 29
- **Done:** 24
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 5, each recorded in its row or in Deviations

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| An operator can capture what a misbehaving peer sent and ship the file | functional | `test/plugin/bgp-capture-replay.ci` PASS: the daemon runs with `capture { enabled true; }`, the fixture barrier `capture-has-messages` reads seq=3 out of `./capture/bgp-127.0.0.1.jsonl`, and `capture-closed` proves the file is terminated on SIGTERM. Mutation-verified 2026-08-03: with `Session.teeCapture` disabled the seq=3 barrier goes red |
| A developer feeds that file back into the SAME state machine | functional | The same `.ci`, seq=6: `ze-test replay ./capture/bgp-127.0.0.1.jsonl` prints `OPEN`, `UPDATE`, `announce=[10.0.0.0/24]` and `ESTABLISHED`. `runReplay` calls `Session.ReadAndProcess`, the function the daemon's read loop calls, and reads the prefixes off the `WireUpdate` that path built, so there is no second decoder to diverge |
| Replay is deterministic | functional | `runReplay` calls `session.SetClock(sim.NewFakeClock(replayEpoch))`, and `replayEpoch` is a fixed date, so the replay reports the same times whenever it runs. `internal/bgp/fsm/timer.go`, the older raw-time path the spec asked to check, does not exist in the tree |
| Zero cost when capture is off | benchmark | `TestSessionTeeCaptureDisabledDoesNotAllocate` asserts `testing.AllocsPerRun(...) == 0` over `teeCapture` with `captureWriter == nil`. The assertion is what gates it: `BenchmarkSessionTeeCaptureDisabled` only reports |
| A capture cannot fill a disk | unit | `TestSessionCaptureRotatesAtLimit` and `TestSessionCaptureStopsAtLimit` drive a cap through both `on-limit` values; `TestWriterLimitIsHard` and `TestWriterPayloadBoundIsExact` prove the encoder refuses a line WHOLE rather than crossing the bound |
| A capture cannot carry a local secret | unit (negative) | `TestRedactConfigPayload` and `TestRedactPayloadFailsClosed` (`internal/core/capture/capture_test.go`): a payload that will not parse is replaced entirely and the error is returned, so a caller that ignores it still cannot leak |
| A malformed UPDATE is captured as the peer sent it | unit | `TestSessionCaptureRecordsPreEnforcementBytes`: the tee sits ahead of the RFC 7606 short-circuit, which is what the existing `MessageObserver` hook could not do |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| Deterministic replay of TIMER-driven behavior (hold expiry, keepalive), and multi-peer replay | The harness feeds messages and never advances the fake clock, so a timer expiry is not reproduced. Full timer determinism needs an event queue, which R-3 put out of scope | `plan/spec-improve-3-event-replay-deferred-deterministic-scheduler.md` |
| A capture file name that carries the peer PORT | `captureFileName` keys on the address while the reactor keys peers by `AddrPort`, so two peers on one address with different ports share a file. Changing the name changes what an operator and the `.ci` both spell | Not yet homed. Recorded under Known Limitations and raised as N13; needs the owner to schedule a spec |
| A stress run of the capture writer at inject rates | Superseded: `offer` sheds on a full queue and writes the gap into the stream, so the writer is not required to keep up | none; see the A-3 row under Assumptions Resolved |

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/improve-3-event-replay-zeclose-replay.md` |
| `./le spec session review check` | `OK (0 code files, clean, hashes match ...)`. It also NOTEs that the running model could not be determined, so the review-model boundary is unchecked |
| Rounds | 3. Round 2 found the discarded `json.Marshal` error in `captureBGPConfigEvent`; round 3 read the fix and found nothing above NOTE |
| Reviewer lenses used | encoder bounds; capture lifecycle and goroutine ownership; replay harness against untrusted input; wiring against AC-1..AC-8; the `docs/contributing/ze-go-style.md` style pass; the spec's Security Review Checklist |

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

### Run 2 (closure gate, 2026-09-05)

One reviewer over the whole committed diff (`025a74b72` plus the gate repair
`ca53400d4`), reading source rather than the Run 1 record. Lenses: the encoder's
bounds, the capture lifecycle and its one goroutine, the replay harness against
untrusted input, the wiring against AC-1..AC-8, the `ze-go-style.md` style pass,
and the spec's own Security Review Checklist.

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| I15 | ISSUE | `json.Marshal(detail)` failing set `payload = nil` and said nothing. A config event with no payload is also how "this phase carries no detail" is spelled, so a lost payload and an empty one read the same way in the file (`ai/rules/principles.md`, a value that is silently wrong) | `internal/component/bgp/plugin/operation.go` `captureBGPConfigEvent` | fixed: the error is logged on `bgp.capture` with the operation and the transaction id before the payload is dropped |
| N16 | NOTE | `Writer.quoted` truncates at `maxFieldLen` BYTES, which can cut a multi-byte rune. Go's JSON decoder coerces the broken sequence to U+FFFD rather than failing, and every value written this way is an ASCII operation name, direction or transaction id | `internal/core/capture/writer.go` `quoted` | acknowledged: no reachable caller supplies a non-ASCII value |
| N17 | NOTE | `Writer.flush` documents "nothing partial ever reaches the file". That holds for the LIMIT refusal; a short write from the OS under ENOSPC can still leave a partial line | `internal/core/capture/writer.go` `flush` | acknowledged: the outcome is reported rather than silent, and `Reader.Next` names the line |
| N18 | NOTE | `--local-as`, `--peer-as` and `--router-id` are `flag.Uint`, so a value above 2^32 truncates on a 64-bit host | `internal/test/cli/cmd_replay.go` `cmdReplay` | acknowledged: a developer-facing test tool, and 0 falls back to the capture header |
| N19 | NOTE | `internal/core/capture` carries no fuzz target. Its parsing beyond the standard library is the version check, the sequence check and the len-versus-data check; the bytes then go to the BGP decoder, which is fuzzed | `internal/core/capture/reader.go` `Reader.Next` | acknowledged |

### Run 3 (2026-09-05)

Read the I15 fix and the function around it. Zero BLOCKER, zero ISSUE. The four
Run 2 NOTEs stand as recorded.

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/bgp/reactor/capture_replay.go` | Yes | 708 lines, declares `sessionCapture` |
| `internal/core/capture/capture.go` | Yes | declares `Header` and `RedactPayload` |
| `internal/core/capture/writer.go` | Yes | declares `Writer` |
| `internal/core/capture/reader.go` | Yes | declares `Reader` and `MaxLineLen` |
| `internal/test/cli/cmd_replay.go` | Yes | declares `runReplay` |
| `test/plugin/bgp-capture-replay.ci` | Yes | 128 lines, seven `cmd=` steps |
| `internal/component/doctor/checks_bgp_capture.go` | Yes | emits `doctor-bgp-capture-directory` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | a disabled capture allocates nothing | `go test -run 'Capture\|Tee\|Replay' ./internal/component/bgp/reactor/...` returned `ok ... 1.045s`, which runs `TestSessionTeeCaptureDisabledDoesNotAllocate` |
| AC-2 | one JSONL event per message | the same run, `TestSessionCaptureWritesEvents`; the `.ci` PASS at 7.0s |
| AC-3 | replay reproduces the run | `go test ./internal/test/cli/...` returned `ok ... 1.932s` (`TestReplayDrivesSession`); the `.ci` asserts `ESTABLISHED` and `announce=[10.0.0.0/24]` |
| AC-4 | the cap rotates or stops | the same reactor run, `TestSessionCaptureRotatesAtLimit` and `TestSessionCaptureStopsAtLimit` |
| AC-5 | a corrupt file names the line and does not panic | `go test ./internal/core/capture/...` returned `ok ... 0.011s` (`TestReaderCorruptInput`); `TestReplayRejectsCorruptCapture` in the cli run |
| AC-6 | config events carry the txID | the reactor run, `TestReactorCaptureConfigEventReachesOpenCaptures` |
| AC-7 | pre-enforcement bytes | the reactor run, `TestSessionCaptureRecordsPreEnforcementBytes` |
| AC-8 | both read paths identical | the reactor run, `TestSessionCaptureIdenticalAcrossReadPaths` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| peer config `capture { enabled true; }` tees at both read paths | `test/plugin/bgp-capture-replay.ci` | Yes: the config block sets `enabled true` and seq=3 waits on the fixture that reads seq=3 out of the file. `Session.teeCapture` is called from `readAndProcessMessage` (`session_read.go`) and `readAndProcessCoalesced` (`session_coalesce.go`) |
| `ze-test replay <file>` reaches `Session.ReadAndProcess` | `test/plugin/bgp-capture-replay.ci` seq=6 | Yes: read the file. The expectations name `OPEN`, `UPDATE`, `announce=[10.0.0.0/24]` and `ESTABLISHED`, which only the real path produces |
| `ze-test replay -` reads stdin | `test/plugin/bgp-capture-replay.ci` seq=7 | Yes: `stdin-replay-ok`; mutation-verified 2026-08-04 by swapping `cliio.OpenReader` for `os.Open` |
| a config commit with capture on reaches `CaptureConfigEvent` | unit | `TestReactorCaptureConfigEventReachesOpenCaptures`, plus the `var _ bgpCaptureHandle = (*reactor.Reactor)(nil)` assertion in `bgp/config/register.go` that makes a lost method set a build error |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | broken, as anticipated; the design changed | Coalescing has its own read path and is default on, so there are TWO tees, in `readAndProcessMessage` and `readAndProcessCoalesced`, held identical by `TestSessionCaptureIdenticalAcrossReadPaths` |
| A-2 | confirmed for this spec's scope | `runReplay` injects `sim.NewFakeClock(replayEpoch)` and message-driven replay is deterministic (`TestReplayDrivesSession`, `.ci` seq=6). The older raw-time path the spec asked to verify, `internal/bgp/fsm/timer.go`, does not exist in the tree. Timer EXPIRY is not exercised and is Work Not Done |
| A-3 | broken; the design removed the need | The writer is not required to keep up. `sessionCapture.offer` sheds on a full 1024-deep queue, counts the loss, and `markDrops` writes the gap into the stream, so a replay never mistakes a gap for a quiet peer (`TestSessionCaptureDropsUnderBackpressure`). No stress run was made and none is relied on |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Row 1, feature list | the `ze doctor` row of `docs/features.md` names `doctor-bgp-capture-directory`, which `internal/core/diagnostic/codes.go` declares | Yes |
| Row 2, config syntax | `container capture` in `internal/component/bgp/yang/ze-bgp-conf.yang` carries a `ze:help` per leaf, and `parseCaptureSettings` (`reactor/config.go`) enforces the same `1..1024` range the YANG declares | Yes |
| Row 3, CLI reference | N/A: the command is `ze-test replay`, a ze-test root, and `docs/guide/command-reference.md` documents the `ze` dispatch only | Yes |
| Row 10, test infrastructure | `docs/functional-tests.md`, "Replaying a captured BGP session", with the usage line and the `-` form, anchored on `test/plugin/bgp-capture-replay.ci` | Yes |
| Row 11, comparison table | the "Session capture and replay" row and paragraph of `docs/comparison.md`, anchored `<!-- source: internal/component/bgp/reactor/capture_replay.go -- sessionCapture, teeCapture -->` | Yes |
| Row 14, Prometheus counter | `ze_bgp_capture_dropped_events_total`, registered in `newReactorMetrics` (`reactor_metrics.go`), labelled by peer | Yes |
| Row 15, doctor inventory | `internal/component/doctor/checks_bgp_capture.go` and `internal/core/diagnostic/codes.go` | Yes |
| Rows 4, 5, 7, 8, 9, 12, 13, 16, 17 | No change: capture observes the wire and never writes it, adds no RPC, no plugin and no route metadata key. `./le repository check` passes, which resolves the source anchors over the changed files | Yes |
| `./le doc check verify` | FAILS on findings that touch no file of this spec: the `ze-bgp-conf:bgp/defaults/attribute` AIGP summary over char-cap and word-cap, and four anchors naming `getHelpExtension`, `Node.Help` and `answerZeroTunnelIDSCCRQ` | Foreign |

## Core Insight

A capture that records DECODED events cannot record the inputs a bug capture
exists for. Ze already had a message-observer hook, and it fires after RFC 7606
enforcement has tombstoned attributes and synthesized withdrawals, so the one
message an operator wants to ship is the one the hook cannot see. The tee had to
go where the bytes are still the peer's: after the body read, before anything
consumes the buffer, and on BOTH read paths, because coalescing is default on
and has its own.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Wiring Test table complete
- [ ] `./le verify worktree` passes

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior
