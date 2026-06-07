# Spec: Chaos MRT Recording

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-06-07 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/mrt.md` - MRT wire format library architecture
4. `internal/chaos/report/reporter.go` - Consumer interface
5. `internal/chaos/peer/event.go` - Event struct
6. `internal/chaos/peer/simulator.go:260-400` - where wire bytes and events originate

## Task

Add standard MRT recording to ze-chaos. A new `report.Consumer` implementation
(`MRTLog`) writes BGP4MP_MESSAGE_AS4 and BGP4MP_STATE_CHANGE_AS4 records from
peer events, producing MRT files readable by any standard tool (bgpdump,
bgpkit-parser, ze-analyse statistics/filter). Reuses `internal/mrt/` for all
wire encoding and file management.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/mrt.md` - MRT wire format library (shared with daemon component)
  -> Constraint: Encoder uses buffer-first WriteTo(buf, off) int pattern; Writer handles strftime rotation
  -> Decision: MRTLog will use mrt.Writer for file I/O and mrt.WriteBGP4MPMessage/WriteCommonHeader for encoding
- [ ] `docs/architecture/chaos-web-dashboard.md` - chaos reporting architecture
  -> Constraint: Consumer.ProcessEvent runs synchronously on the main event loop goroutine; must be fast
  -> Decision: Pool-based buffer allocation; encode and write in ProcessEvent, no goroutines

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc6396.md` - MRT base format (BGP4MP types and subtypes)
  -> Constraint: BGP4MP_MESSAGE_AS4 (subtype 4) uses 4-byte AS fields; entire BGP message including 16-byte marker is included
  -> Constraint: BGP4MP_STATE_CHANGE_AS4 (subtype 5) uses FSM codes 1-6 from RFC 4271

**Key insights:**
- Consumer interface is `ProcessEvent(ev peer.Event)` + `Close() error`; synchronous, no goroutines
- Reporter fans out to all consumers via `NewReporter(consumers...)`
- Wire bytes (`data := sender.BuildRoute(prefix)`) exist at the same scope as the `emit(Event{...})` call
- Event struct has no BGPMessage field today; adding one is the core data model change
- peer.Event is referenced from 392 call sites across 47 files; backward compatibility is critical
- `internal/mrt/` provides all encoding (WriteCommonHeader, WriteBGP4MPMessage, WriteBGP4MPStateChange) and file I/O (Writer with strftime rotation)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/chaos/peer/event.go` - Event struct with 12 event types; no BGPMessage field
  -> Constraint: Event is a value type (not pointer); adding []byte field increases copy cost but only when non-nil
- [ ] `internal/chaos/peer/simulator.go:260-400` - route sending loop; `data` ([]byte) produced by sender, written to conn, then Event emitted without the bytes
  -> Constraint: `data` is available in scope at every emit() call site; threading it through is mechanical
- [ ] `internal/chaos/peer/simulator_reader.go:65-99` - readLoop receives raw BGP from the wire, parses prefix events
  -> Constraint: Received messages also have wire bytes available at the read site
- [ ] `internal/chaos/report/reporter.go` - Consumer interface, Reporter fan-out multiplexer
  -> Constraint: Consumers run synchronously; must not block I/O
- [ ] `internal/chaos/report/jsonlog.go` - reference Consumer implementation; uses json.Encoder, mutex, first-error tracking
  -> Decision: MRTLog follows the same structural pattern (mutex, first-error, Close returns accumulated error)
- [ ] `internal/chaos/peer/session.go:113` - `SerializeMessage(msg message.Message) []byte` produces wire bytes
  -> Constraint: Returns complete BGP message including marker, length, type
- [ ] `internal/chaos/orchestrator/run.go:525` - `report.NewReporter(consumers...)` wires consumers into the event loop
  -> Constraint: MRTLog is added to the consumer list alongside JSONLog, Dashboard, Metrics

**Behavior to preserve:**
- All existing event processing (JSONLog, Dashboard, Metrics, Summary) unchanged
- Event struct backward-compatible (new field is zero-value when not set)
- No performance regression on event hot path when MRT recording is disabled

**Behavior to change:**
- `peer.Event` gains a `BGPMessage []byte` field (nil when not applicable)
- `simulator.go` populates BGPMessage at each emit() site where wire bytes are available
- `simulator_reader.go` populates BGPMessage for received messages
- New `report/mrtlog.go` Consumer writes MRT records

## Data Flow (MANDATORY)

### Entry Point
- Wire bytes produced by `Sender.BuildRoute()` / `BuildVPNRoute()` / etc. in `simulator.go`
- Wire bytes received by `readLoop()` in `simulator_reader.go`
- State transitions from session establishment/disconnection in `simulator.go`

### Transformation Path
1. `simulator.go`: `data := sender.BuildRoute(prefix)` produces []byte
2. `simulator.go`: `emit(Event{..., BGPMessage: data})` adds wire bytes to event
3. `orchestrator/run.go`: `reporter.Process(ev)` fans out to all consumers
4. `report/mrtlog.go`: `ProcessEvent(ev)` encodes BGP4MP record using `mrt.WriteBGP4MPMessage(buf, off, hdr, true, ev.BGPMessage)`, writes header + body via `mrt.Writer`

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| peer -> report | Event struct with BGPMessage []byte | [ ] |
| report -> mrt library | mrt.WriteBGP4MPMessage + mrt.Writer | [ ] |

### Integration Points
- `internal/mrt/encode.go:WriteBGP4MPMessage` - encodes BGP4MP message body
- `internal/mrt/encode.go:WriteBGP4MPStateChange` - encodes state change body
- `internal/mrt/encode.go:WriteCommonHeader` - encodes 12-byte MRT header
- `internal/mrt/writer.go:Writer` - file I/O with strftime rotation
- `internal/chaos/orchestrator/run.go` - wires consumers into reporter

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (BGPMessage is the same slice produced by sender; no copy until MRT encoding)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `report.NewReporter(mrtlog, ...)` -> `reporter.Process(ev)` | -> | `MRTLog.ProcessEvent` writes BGP4MP record | `TestMRTLogProcessEvent` |
| Orchestrator `--mrt-file` flag | -> | MRTLog added to consumer list | `TestOrchestratorMRTFlag` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | EventEstablished with peer index N | MRTLog writes BGP4MP_STATE_CHANGE_AS4 record with OldState=Idle, NewState=Established, correct peer/local AS and IP |
| AC-2 | EventDisconnected with peer index N | MRTLog writes BGP4MP_STATE_CHANGE_AS4 record with OldState=Established, NewState=Idle |
| AC-3 | EventRouteSent with BGPMessage set | MRTLog writes BGP4MP_MESSAGE_AS4 record containing the exact BGP wire bytes |
| AC-4 | EventRouteReceived with BGPMessage set | MRTLog writes BGP4MP_MESSAGE_AS4 record for the received message |
| AC-5 | EventRouteSent with BGPMessage nil | MRTLog skips the record (no crash, no corrupt output) |
| AC-6 | MRT file opened with strftime pattern `/tmp/chaos-%Y%m%d.mrt` | File created with expanded timestamp in name |
| AC-7 | MRT output file parsed by `mrt.ReadFile` | All records decode without error; header types/subtypes match |
| AC-8 | Event struct with no BGPMessage field set | JSONLog, Dashboard, Metrics, Summary unchanged (backward compatible) |
| AC-9 | Chaos run with `--mrt-file` flag | MRT file produced alongside JSON log |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestMRTLogStateChange` | `internal/chaos/report/mrtlog_test.go` | AC-1, AC-2: state change events produce valid BGP4MP_STATE_CHANGE_AS4 records | |
| `TestMRTLogMessage` | `internal/chaos/report/mrtlog_test.go` | AC-3: route sent events produce valid BGP4MP_MESSAGE_AS4 records | |
| `TestMRTLogReceivedMessage` | `internal/chaos/report/mrtlog_test.go` | AC-4: received messages produce valid MRT records | |
| `TestMRTLogNilMessage` | `internal/chaos/report/mrtlog_test.go` | AC-5: nil BGPMessage skipped gracefully | |
| `TestMRTLogRoundTrip` | `internal/chaos/report/mrtlog_test.go` | AC-7: write events, read back with mrt.ReadFile, verify content matches | |
| `TestEventBGPMessageBackwardCompat` | `internal/chaos/peer/event_test.go` | AC-8: zero-value BGPMessage does not affect existing event processing | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| PeerIndex | 0-65535 | 65535 | N/A (uint) | N/A (uint) |
| BGPMessage length | 0-65535 | 65535 (max BGP msg) | N/A (nil = skip) | N/A (MRT Length field is uint32) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N/A | - | ze-chaos is a testing tool, not a daemon with .ci tests | |

### Interop Tests (MANDATORY for protocol features)
Justification for skip: MRT is an archival format, not a wire protocol exchanged between peers. The round-trip test (write with mrt encoder, read with mrt decoder) validates format correctness.

## Files to Modify
- `internal/chaos/peer/event.go` - add BGPMessage []byte field to Event struct
- `internal/chaos/peer/simulator.go` - populate BGPMessage at each emit() site
- `internal/chaos/peer/simulator_reader.go` - populate BGPMessage for received messages
- `internal/chaos/orchestrator/run.go` - add MRTLog to consumer list when --mrt-file set
- `internal/chaos/orchestrator/cli.go` - add --mrt-file flag

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | N/A (CLI tool, not daemon config) |
| CLI commands/flags | Yes | `internal/chaos/orchestrator/cli.go` -- add `--mrt-file` flag |
| Functional test | No | Testing tool, not daemon |
| Pipe completeness | No | Not a show command |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` -- add MRT recording to chaos features |
| 11 | Affects daemon comparison? | Yes | `docs/research/mrt-implementation-comparison.md` -- update Ze column |

## Files to Create
- `internal/chaos/report/mrtlog.go` - MRTLog consumer implementation
- `internal/chaos/report/mrtlog_test.go` - unit tests with round-trip validation

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `go vet ./internal/chaos/... && go test ./internal/chaos/...` |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Event struct** -- add BGPMessage field to peer.Event
   - Tests: `TestEventBGPMessageBackwardCompat`
   - Files: `internal/chaos/peer/event.go`, `internal/chaos/peer/event_test.go`
   - Verify: existing tests still pass; new field is zero-value compatible

2. **Phase: Wire bytes threading** -- populate BGPMessage at emit() sites
   - Tests: existing chaos tests still pass (no behavioral change)
   - Files: `internal/chaos/peer/simulator.go`, `internal/chaos/peer/simulator_reader.go`
   - Verify: BGPMessage populated for sent/received messages; nil for non-message events

3. **Phase: MRTLog consumer** -- implement report/mrtlog.go
   - Tests: `TestMRTLogStateChange`, `TestMRTLogMessage`, `TestMRTLogNilMessage`, `TestMRTLogRoundTrip`
   - Files: `internal/chaos/report/mrtlog.go`, `internal/chaos/report/mrtlog_test.go`
   - Verify: all unit tests pass; round-trip produces valid MRT

4. **Phase: CLI wiring** -- add --mrt-file flag and wire into orchestrator
   - Tests: `TestOrchestratorMRTFlag`
   - Files: `internal/chaos/orchestrator/run.go`, `internal/chaos/orchestrator/cli.go`
   - Verify: flag accepted; MRTLog created and added to reporter consumer list

5. **Full verification** -- `go vet ./internal/chaos/...`, `go test ./internal/chaos/...`

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | MRT records decode correctly via mrt.ReadFile round-trip |
| Naming | MRTLog, not MrtLog (acronym capitalization per Go convention) |
| Data flow | Wire bytes flow peer->event->consumer->mrt.Writer; no copies beyond the initial BuildRoute allocation |
| Hot path | ProcessEvent uses pooled buffer, no allocations per event when MRT disabled |
| Backward compat | Existing consumers unaffected by new Event field |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| `internal/chaos/report/mrtlog.go` exists | `ls internal/chaos/report/mrtlog.go` |
| MRTLog implements Consumer | `grep 'func.*MRTLog.*ProcessEvent' internal/chaos/report/mrtlog.go` |
| Event.BGPMessage field exists | `grep 'BGPMessage' internal/chaos/peer/event.go` |
| --mrt-file flag accepted | `grep 'mrt-file\|mrt_file\|mrtFile' internal/chaos/orchestrator/cli.go` |
| Round-trip test passes | `go test -run TestMRTLogRoundTrip ./internal/chaos/report/` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | --mrt-file path: only used as file path for mrt.Writer; no injection risk (os.OpenFile) |
| Resource exhaustion | BGPMessage []byte on Event: bounded by BGP max message size (65535); not attacker-controlled beyond what BGP already allows |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Existing chaos tests fail | Event struct change broke something -- check zero-value compatibility |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Add BGPMessage to Event struct | (A) Parallel channel for wire bytes, (B) Callback-based recording in sender | Event struct is the natural carrier; all consumers already receive it; one field addition vs. new infrastructure |
| Use BGP4MP_MESSAGE_AS4 (subtype 4) always | (A) Detect 2-byte vs 4-byte AS from session | Chaos peers always use 4-byte AS; AS4 is the modern default; matches GoBGP and FRR |
| Pool-based buffer in ProcessEvent | (A) Allocate per event, (B) Single persistent buffer | Pool avoids allocation on hot path; persistent buffer would need mutex contention with the existing lock |

## Known Limitations
- Received messages: readLoop reads into a reused buffer, so BGPMessage needs a copy for received events (one allocation per received message)
- No extended timestamp (BGP4MP_ET) initially; can switch type code later
- No per-peer recording filter; all peers go to one file
- No TABLE_DUMP_V2 (chaos has no RIB; it generates and sends routes)
