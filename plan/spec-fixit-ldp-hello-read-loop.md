# Spec: fixit-ldp-hello-read-loop

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | - |
| Phase | - |
| Updated | 2026-07-17 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/plugins/ldp/register.go` (discover loop 587-610, `discoverOnInterface` 540, `sendHello` 665)
4. `internal/plugins/isis/transport/backend_linux.go:196-234` - the correct dedicated-reader model
5. `internal/plugins/ldp/discovery.go` (`DefaultHelloInterval` :18, `AdjacencyTable` :56)

## Task

**[MEDIUM]** LDP Basic Discovery reads inbound Hellos only once per hello-send tick, one packet at a
time, so on a shared segment inbound Hellos are dropped and adjacencies flap. In
`internal/plugins/ldp/register.go:587-610` the per-interface loop's `select` has only two cases,
`<-ctx.Done()` (:589) and `<-helloTicker.C` (:591, calls `sendHello`); the single 1-second-bounded
`udpConn.ReadFromUDP(recvBuf)` (:599, deadline set :595) sits AFTER the `select` and feeds
`processDiscoveryPacket` (:609) exactly once per iteration. Because `DefaultHelloInterval = 5s`
(`internal/plugins/ldp/discovery.go:18`) the loop turns roughly every 5s, so inbound Hellos are
consumed once per 5s, one at a time. On a segment with N neighbors, N Hellos arrive per interval but
only 1 is drained, the socket receive buffer fills, Hellos are dropped, hold timers (`DefaultHelloHoldTime
= 15s`) expire, and adjacencies fail or flap; even one neighbor is timing-fragile (its Hello must land
inside the 1s window that opens once per 5s). The worker runs per interface as a production goroutine
(`internal/plugins/ldp/discovery_manager.go:71` `go m.startFn(...)` -> `discoverOnInterface`, wired
at `register.go:323-329`).

**Fix:** separate the receive path from the send path. Run `ReadFromUDP` continuously in its own
dedicated reader goroutine (model the ISIS `readLoop`, `internal/plugins/isis/transport/backend_linux.go:196-234`),
and drive `sendHello` from `helloTicker` independently. Preserve `processDiscoveryPacket` / adjacency-table
semantics and clean ctx-cancel shutdown.

**Related (do not duplicate):** `plan/spec-improve-5-panic-boundaries` should add a per-message
`recover` boundary around the packet decode in this loop. Note the overlap; that spec owns the recover work.

## Required Reading

### Source
- [ ] `internal/plugins/ldp/register.go` - `discoverOnInterface` (:540), discover loop (:587-610), `sendHello` (:665)
  → Constraint: `ReadFromUDP` (:599) is the only drain and is gated behind the `select` on `helloTicker.C`, so read cadence is coupled to send cadence. The fix decouples them.
  → Decision: keep `sendHello` on `helloTicker`; move only the read into a dedicated goroutine that loops on `ReadFromUDP` without a per-tick gate.
- [ ] `internal/plugins/isis/transport/backend_linux.go:196-234` - `readLoop`, the correct dedicated-reader pattern
  → Constraint: a receiver runs its own loop, checks a stop signal, copies the PDU out of the shared buffer before handing it off, and exits on socket close / ctx cancel.
- [ ] `internal/plugins/ldp/discovery.go` - `DefaultHelloInterval` (:18), `AdjacencyTable` (:56, `sync.RWMutex`)
  → Constraint: `AdjacencyTable.Update/Remove/All/Get/ExpireSweep` are all lock-guarded (:58), so a reader goroutine calling `processDiscoveryPacket` -> `Update` stays race-safe against the existing expiry sweep.
- [ ] `internal/plugins/ldp/discovery_manager.go` - `reconcile` (:45), `go m.startFn` (:71)
  → Constraint: one worker per interface; a config reload cancels its ctx. The new reader goroutine must exit on that same ctx cancel so reload stays leak-free.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/ldp/register.go` - `discoverOnInterface` sets up the multicast conn, sends an initial Hello (:583), then loops: `select { ctx.Done | helloTicker.C -> sendHello }` (:588-593) followed by one deadline-bounded `ReadFromUDP` + `processDiscoveryPacket` (:595-609).
- [ ] `internal/plugins/ldp/discovery.go` - `DefaultHelloInterval` 5s (:18), `DefaultHelloHoldTime` 15s (:19), `AdjacencyTable` RWMutex-guarded (:56-107).
- [ ] `internal/plugins/isis/transport/backend_linux.go` - `readLoop` (:196) dedicated goroutine model to mirror.
- [ ] `internal/plugins/ldp/discovery_manager.go` - per-interface goroutine lifecycle (`reconcile` :45, `stopAll` :79).

**Behavior to preserve:**
- `sendHello` cadence (`helloTicker`, `cfg.HelloInterval`), the initial Hello on entry, multicast egress pinning + TTL 1 (:570-578).
- `processDiscoveryPacket` decode + `AdjacencyTable.Update` + `onNewAdj` session-start semantics, unchanged.
- Clean shutdown on ctx cancel and on `udpConn.Close()`; no goroutine or socket leak across config reloads.

**Behavior to change:**
- Drain inbound Hellos continuously in a dedicated reader goroutine, decoupled from the send tick, so all N neighbors' Hellos per interval are consumed.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Real: `discoverOnInterface` (`internal/plugins/ldp/register.go:540`), spawned per interface by `discoveryManager` (`discovery_manager.go:71`, wired `register.go:323-329`). Inbound: multicast UDP 224.0.0.2:646 Hellos from on-link LSRs.

### Transformation Path
1. A neighbor's Basic Discovery Hello arrives on the interface's multicast UDP socket (`udpConn`).
2. Today: the loop reaches `ReadFromUDP` (:599) only after the `select`, once per iteration, then decodes via `processDiscoveryPacket` (:609).
3. After fix: a dedicated reader goroutine loops on `ReadFromUDP` and calls `processDiscoveryPacket` for every datagram; the main loop only fires `sendHello` on `helloTicker` and returns on ctx cancel.
4. `processDiscoveryPacket` -> `AdjacencyTable.Update` (locked) -> `onNewAdj` starts/refreshes the session.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Network ↔ discovery worker | multicast UDP datagram -> `ReadFromUDP` | [ ] |
| Reader goroutine ↔ adjacency table | `processDiscoveryPacket` -> `AdjacencyTable.Update` (RWMutex) | [ ] |
| ctx cancel ↔ reader/sender exit | reload/shutdown cancels ctx; both goroutines return, socket closes | [ ] |

### Integration Points
- `internal/plugins/ldp/register.go` (`discoverOnInterface`, `sendHello`), `discovery_manager.go` (per-interface ctx lifecycle), `AdjacencyTable` (locked writes), `runAdjacencyExpiry` (hold-timer sweep). Registration over hardcoding: the worker stays wired through `discoveryManager.startFn`, not a new ad-hoc goroutine spawn path.

## Risks & Assumptions

| ID | Assumption / Risk | Basis | If wrong / Mitigation |
|----|-------------------|-------|-----------------------|
| A-1 | `*net.UDPConn` supports one goroutine reading while another writes (`sendHello`) | Go `net.UDPConn` is safe for concurrent Read and Write | If wrong, gate send behind the reader; but the stdlib guarantee holds |
| A-2 | Moving reads into a goroutine that calls `AdjacencyTable.Update` is race-safe | `AdjacencyTable` is RWMutex-guarded (`discovery.go:58`); expiry sweep already runs concurrently | Run with `-race`; no new shared unguarded state introduced |
| A-3 | ctx cancel + `udpConn.Close()` reliably unblocks a blocked `ReadFromUDP` | `discoverOnInterface` already closes on return (:561-565); a closed UDP socket returns an error from `ReadFromUDP` | Reader loop treats close/ctx as exit (ISIS model); keep a read deadline as a backstop so a lost close still wakes the loop |
| A-4 | A reader goroutine per interface does not leak across reloads | one ctx per interface (`discovery_manager.go:69`), cancelled on removal | Reader selects on ctx / exits on socket close; assert no leak in the wiring test |
| R-1 | Reordering could drop the shared `recvBuf` copy semantics | current code decodes in place before next read | Give the reader its own buffer; copy before handoff per the ISIS model |

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| N Hellos arrive within one `HelloInterval` on one socket | -> | dedicated reader drains all N, `AdjacencyTable.Len() == N` | `TestDiscoveryDrainsBurst` |
| ctx cancel / socket close while reader is blocked | -> | reader goroutine returns, no leak | `TestDiscoveryReaderExitsOnCancel` |
| `helloTicker` fires with no inbound traffic | -> | `sendHello` still runs on cadence | `TestDiscoverySendUnaffectedByReads` |
| N LDP neighbors on a shared segment converge and stay adjacent across full intervals (no flap) | -> | reader drains every neighbor's Hello each interval; `show ldp neighbor` lists all N, none age out | `test/ldp/ldp-convergence.ci` |

Concrete test: `TestDiscoveryDrainsBurst` binds a loopback multicast socket, writes N (e.g. 5) valid
Hello datagrams back-to-back into it inside a single `HelloInterval`, runs the reader, and asserts
`adjTable.Len() == 5` well before `DefaultHelloHoldTime`. Under the current once-per-tick drain this
fails (only 1 adjacency appears per interval); after the fix it passes.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | N Hellos from N neighbors arrive within one `HelloInterval` | all N are consumed; N adjacencies created within one interval, none dropped |
| AC-2 | A single neighbor Hello arrives at an arbitrary instant | it is consumed promptly (not only inside a 1s-per-5s window); no hold-timer flap |
| AC-3 | ctx cancel (reload/shutdown) | reader and sender goroutines both exit, socket closes, no goroutine/socket leak (`-race` clean) |
| AC-4 | Steady state | `sendHello` cadence unchanged; `processDiscoveryPacket`/adjacency/session semantics unchanged |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDiscoveryDrainsBurst` | `internal/plugins/ldp/discovery_burst_test.go` | AC-1 | |
| `TestDiscoverySingleHelloPrompt` | `internal/plugins/ldp/discovery_burst_test.go` | AC-2 | |
| `TestDiscoveryReaderExitsOnCancel` | `internal/plugins/ldp/discovery_burst_test.go` | AC-3 | |
| `TestDiscoverySendUnaffectedByReads` | `internal/plugins/ldp/discovery_burst_test.go` | AC-4 | |

### Functional Tests
| Test | File | Validates |
|------|------|-----------|
| Multi-neighbor convergence stays stable (no flap) | `test/ldp/ldp-convergence.ci` | AC-1, AC-2 |
| Session/adjacency lifecycle across reload | `test/ldp/ldp-session.ci`, `test/ldp/ldp-reload.ci` | AC-3, AC-4 |

## Files to Modify
- `internal/plugins/ldp/register.go` - split `discoverOnInterface` into a dedicated reader goroutine (ISIS-style) plus a send-only ticker loop; preserve `sendHello`, `processDiscoveryPacket`, ctx-cancel shutdown.

## Files to Create
- `internal/plugins/ldp/discovery_burst_test.go` - burst-drain, prompt-single, reader-exit, send-cadence tests.

## Implementation Steps
1. **Wiring first** — extract the read path from the `select` loop into a dedicated reader goroutine started inside `discoverOnInterface`, looping on `ReadFromUDP` -> `processDiscoveryPacket` (model `readLoop`), with its own buffer.
2. Reduce the main loop to `select { <-ctx.Done() -> return; <-helloTicker.C -> sendHello }`; keep the initial Hello on entry.
3. Ensure clean shutdown: reader exits on ctx cancel / socket close; keep a read deadline as a backstop so a missed close still wakes it (A-3).
4. Add `discovery_burst_test.go`; confirm `TestDiscoveryDrainsBurst` FAILS on the pre-fix loop, PASSES after.
5. Run `make ze-test` and the LDP `.ci` functional tests under `-race`.
6. Complete spec: audit tables, `plan/learned/NNN-<name>.md`, two-commit closure.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-4 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Reader/sender goroutines both exit on ctx cancel; `-race` clean, no leak

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] Implementation Audit complete
- [ ] Overlap with `plan/spec-improve-5-panic-boundaries` (recover boundary) noted, not duplicated

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Burst / single / cancel / send-cadence cases all present

## Notes
- Skeleton captured from the 2026-07-16 repository audit; lines verified by direct read (`register.go:587-610`, `discovery.go:18`, `discovery_manager.go:71`, `backend_linux.go:196-234`). Related recover-boundary work belongs to `plan/spec-improve-5-panic-boundaries`.
