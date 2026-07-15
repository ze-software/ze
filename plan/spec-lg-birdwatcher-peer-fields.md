# Spec: lg-birdwatcher-peer-fields

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | Phases 1-3 done; Phase 4 (routes_*) blocked on a design decision |
| Updated | 2026-07-15 |

## Progress (2026-07-15)

**Phases 1-3 IMPLEMENTED** (uncommitted at time of writing):
- `state_changed` and `last_error` now populate end-to-end. `handler_api.go`
  needed NO change: it already read `state-changed` / `last-error`, so emitting
  them from `handleBgpSummary` was sufficient. `transformProtocolsShort`'s
  `since` came along for free from the same key.
- New: `fsmHistory.newest()` + `Peer.LastStateChange()` (`peer_history.go`),
  `PeerInfo.LastStateChange` (`types_bgp.go`), populated at `reactor_api.go`.
- New in `summary.go`: `lastErrorString()` and `stateChangedString()`, emitted as
  `last-error` / `state-changed` in the peer row.
- Tests: `TestBgpSummaryEmitsStateChangedAndLastError`,
  `TestBgpSummaryStateChangedAndLastErrorEmpty`, `TestLastErrorFormat`
  (4 subtests), `TestTransformProtocolsStateChangedAndLastError`,
  `TestTransformProtocolsShortSinceFromRealSummary`. All green.

**Phase 4 (`routes_*`) NOT implemented — see the A-4 resolution below.**

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/web-interface.md` - LG/birdwatcher API surface
4. `internal/component/lg/handler_api.go`, `internal/component/bgp/plugins/cmd/peer/summary.go`, `internal/component/plugin/types_bgp.go`

## Task

`transformProtocols` (`internal/component/lg/handler_api.go`) builds the
birdwatcher protocol object Alice-LG consumes. Three groups of fields it emits
are permanently empty or zero because the producer (`show bgp summary`) does not
emit them:

| birdwatcher field | Currently | Why |
|-------------------|-----------|-----|
| `state_changed` | `""` | reads `state-changed`; summary.go emits no such key |
| `last_error` | `""` | reads `last-error`; summary.go emits no such key |
| `routes_received` / `routes_imported` / `routes_exported` / `routes_filtered` | `0` | read `routes-*`; summary.go emits no such keys |

`transformProtocolsShort` has the same `state_changed` gap (it emits `since`).

Commit `bae4f1956` fixed the wiring bugs in the same function (summary-envelope
navigation, `address` vs `peer-address`, uptime string vs number), so peers,
uptime, `neighbor_address`, `neighbor_as`, `description` and `state` are correct
now. This spec covers only the remaining fields, which are MISSING FEATURES, not
wiring bugs: for two of them no producer emits the value, and for one the data
lives in another component by design.

Goal: Alice-LG shows a real "since" time, a real last-error string, and real
route counts for every peer.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] — checkboxes are template markers, not progress trackers. -->
- [ ] `docs/architecture/web-interface.md` - the birdwatcher REST surface this feeds
  → Constraint: `handler_api.go` is the ONLY file exempt from kebab-case JSON keys; it uses birdwatcher `snake_case` for Alice-LG compatibility (`ai/rules/json-format.md:26`). Field names there are an external contract, not ours to rename.
- [ ] `docs/architecture/api/commands.md` - `show bgp summary` handler contract
  → Constraint: the summary payload is consumed by the CLI dashboard, the web summary page, the LG UI and the LG API. Adding keys is safe; changing existing ones is not.

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4271.md` - NOTIFICATION code/subcode semantics for `last_error`
  → Constraint: RFC 4271 Section 4.5 defines the error code/subcode pairs that must be rendered into a human string.

**Key insights:** (summary of all checkpoint lines — minimal context to resume after compaction)
- The three gaps have very different costs. `last_error` is nearly free (data already on `PeerInfo`), `state_changed` is moderate (data exists behind an optional interface the real Reactor implements), `routes_*` crosses a component boundary that `peer_stats.go` draws deliberately.
- Alice-LG reads these. Getting the shape wrong is an external-compat bug, not just a display bug.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
<!-- Same rule: never tick [ ] to [x]. Write → Constraint: annotations instead. -->
- [ ] `internal/component/lg/handler_api.go` - `transformProtocols` (:521) reads `state-changed` (:544), `last-error` (:548) and `routes-*` (:536-539) per peer row. `transformProtocolsShort` (:570) reads `state-changed` for `since`.
  → Constraint: `getStr` returns `""` and `getNum` returns `0` for a missing key, so every one of these degrades silently rather than failing loudly. That is why the gap stayed invisible.
- [ ] `internal/component/bgp/plugins/cmd/peer/summary.go` - `handleBgpSummary` peer rows (:112-127) emit address, name, description, remote-as, peer-type, state, uptime, updates-*, keepalives-*, eor-*, connections-dropped. No `state-changed`, no `last-error`, no `routes-*`.
  → Constraint: this is the producer. Any new field starts here.
- [ ] `internal/component/plugin/types_bgp.go` - `PeerInfo` (:53).
  → Decision: `LastNotifCode`, `LastNotifSubcode`, `LastNotifRecv`, `LastNotifTime` ALREADY EXIST on PeerInfo (:113-117), commented "lifetime, survives session reset". `last_error` therefore needs NO reactor change, only formatting plus emission.
  → Constraint: PeerInfo carries NO route counters and NO state-change timestamp.
  → Decision: `FSMHistoryProvider` (:269) is an OPTIONAL interface, `PeerFSMHistory(addr) []FSMTransitionRecord`, deliberately not on `ReactorLifecycle` "to avoid widening" it. `FSMTransitionRecord` (:259) carries Timestamp/From/To/Reason.
- [ ] `internal/component/bgp/reactor/reactor.go` - `PeerFSMHistory` (:847) implements the optional interface on the real Reactor, copying `p.FSMHistory()` into `[]plugin.FSMTransitionRecord` under `r.mu.RLock()`; returns nil when the peer is not found.
  → Decision: the provider is real, not hypothetical, so `state_changed` has a live data source. Retention/bounding of `p.FSMHistory()` is still UNVERIFIED (see A-3).
- [ ] `internal/component/bgp/reactor/peer_stats.go` - the counter source.
  → Constraint: :21 and :54 BOTH state "NLRI-level counters (announce vs withdraw) belong in the RIB plugin". Route counts are not the reactor's to report. This is an explicit architectural boundary, not an oversight.
  → Decision: `EstablishedAt()` (:311) returns session establishment time; `ClearStats` (:322) zeroes `establishedAt` on teardown but preserves lifetime counters including last notification. So EstablishedAt alone cannot express "state changed" for a peer that is currently DOWN.
- [ ] `internal/component/bgp/reactor/reactor_api.go` - `info.Uptime = a.r.clock.Now().Sub(estAt)` (:160), guarded by `if estAt := p.EstablishedAt(); !estAt.IsZero()`.
  → Constraint: uptime is already derived from EstablishedAt here; a `state-changed` timestamp would be populated in the same place.
- [ ] `internal/component/lg/handler_ui.go` - the HTML UI peer table (:525-570) reads the same summary payload.
  → Constraint: it has the same empty `routes-received` problem, so fixing the producer fixes both surfaces at once.

**Behavior to preserve:** (unless user explicitly said to change)
- Every existing key in the `show bgp summary` payload keeps its current name, type and meaning. The CLI dashboard (`model_dashboard.go:69,76`), the web summary page (`page_bgp_summary.go`) and `lg/handler_ui.go` all parse it.
- birdwatcher `snake_case` field names in `handler_api.go` are an Alice-LG contract and must not be renamed.
- `uptime` stays a whole-second Go duration string in the payload and a number of seconds in the birdwatcher output (commits `c8e09b26b`, `bae4f1956`).

**Behavior to change:** (only if user explicitly requested)
- Add new keys to the summary peer rows. Populate the three currently-dead birdwatcher field groups.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- HTTP GET `/api/looking-glass/protocols/bgp` -> `handleAPIProtocols` (`handler_api.go:40`)
- Format at entry: none (no request body)

### Transformation Path
1. `s.query("show bgp summary")` (`handler_api.go:41`) -> `dispatch.JSON` (`server.go:541`)
2. `handleBgpSummary` (`summary.go:64`) reads `reactor.Peers()` -> `[]plugin.PeerInfo`
3. Peer rows built (`summary.go:112-127`) -> `plugin.Map{"summary": ...}` (`summary.go:152`)
4. `json.Marshal(resp.Data)` (`plugin/dispatch.go:85`) -> JSON string
5. `parseJSON` (`handler_api.go:379`) -> `map[string]any`
6. `transformProtocols` (`handler_api.go:521`) -> birdwatcher object -> `writeJSON`

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Reactor ↔ plugin handler | `plugin.PeerInfo` value struct via `reactor.Peers()` | [ ] |
| Reactor ↔ handler (FSM history) | optional `plugin.FSMHistoryProvider` type assertion, as `show bgp peer` detail already does | [ ] |
| Engine ↔ LG | `show bgp summary` JSON over the dispatcher | [ ] |
| RIB plugin ↔ summary handler | **DOES NOT EXIST TODAY** — the mechanism for route counts must be designed | [ ] |

### Integration Points
- `plugin.PeerInfo` - carries per-peer data to every command handler
- `plugin.FSMHistoryProvider` (`types_bgp.go:269`), implemented at `reactor.go:847`
- RIB plugin - owns NLRI-level counters per `peer_stats.go:21,54`

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)
- [ ] Registration over hardcoding — no new per-feature field, switch case, or factory in a core/shared package (`ai/rules/plugin-self-containment.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `last_error` needs no reactor change; PeerInfo already carries the notification fields | `types_bgp.go:113-117` read directly | Phase 2 grows to include reactor plumbing | `reactor_api.go:142-145` DOES populate all four LastNotif* from `stats`. Only formatting + emission were needed, as predicted | **confirmed** |
| A-2 | Alice-LG expects `state_changed` as an RFC3339 string, not a number | `handler_api.go:544` uses `getStr`; birdwatcher upstream emits a timestamp string | wrong type breaks the Alice-LG UI silently | read alice-lg/birdwatcher upstream `endpoints.go`; confirm against a real Alice-LG if one is reachable | **unvalidated** — shipped as RFC3339 on the `getStr` evidence alone. Upstream NOT read. See R-4 |
| A-3 | `p.FSMHistory()` retains at least the most recent transition for the life of the peer | `reactor.go:847` copies it wholesale; retention NOT yet read | `state_changed` empty for long-lived/flapping peers; a dedicated timestamp needed | `peer_history.go:11,24-43`: a 32-entry ring; `snapshot()` returns NEWEST FIRST and the ring only evicts the OLDEST, so the newest transition always survives. Appended at `peer_run.go:436` with `p.clock.Now()` | **confirmed** |
| A-4 | The RIB plugin holds per-peer route counts attributable to a peer address | `peer_stats.go:21,54` say NLRI counters live there | `routes_*` needs NEW counters, making Phase 4 much larger | **PARTIALLY BROKEN — see the Phase 4 finding below** | **broken** |
| A-5 | Adding keys to the summary payload breaks no existing consumer | consumers parse named fields into structs (`model_dashboard.go:65-85`) | a strict consumer could fail | `go test` green across cli, web, lg, peer, plugin, cmd/show after adding `state-changed` and `last-error` | **confirmed** |

### A-4 resolution (BLOCKS Phase 4 — read before picking it up)

Only ONE of the four route fields has a correct source. Do not implement Phase 4
until this is decided.

| birdwatcher field | Source | Verdict |
|---|---|---|
| `routes_received` | `AdjRIBInManager.status()` (`adj_rib_in/rib_commands.go:220-236`) returns `{"running":true,"total-routes":N,"peers":{"<addr>":count}}` | AVAILABLE. Adj-RIB-In is exactly "routes received" (RFC 4271 3.2, "unprocessed routing information advertised ... by its peers") |
| `routes_imported` / `routes_accepted` | none | NO SOURCE. Adj-RIB-In is PRE-policy. Mapping its count to "imported/accepted" would OVERSTATE accepted routes whenever policy rejects any, i.e. report a number that is wrong rather than absent. `ai/rules/no-workarounds-for-missing-behavior.md` forbids this |
| `routes_exported` / `routes_sent` | none | NO SOURCE in the adj-RIB-In (it is inbound only) |
| `routes_filtered` | none | NO SOURCE. Ze does not track filtered routes at all (`ai/rules/project-knowledge.md`). Likely permanently zero |

Two further constraints found:
- `bgp-adj-rib-in` is an OPTIONAL plugin (`adj_rib_in/register.go:12-23`, a
  `registry.Registration` with `RunEngine`), so `routes_received` would be zero
  wherever it is not enabled. The feature would be conditional on config.
- Its status is NOT a daemon RPC. It is a plugin command string, `show bgp
  adj-rib-in status` (`rib_commands.go:30`), and its YANG
  (`yang/ze-adj-rib-in-api.yang`) declares no command container at all. The LG
  could reach it with one extra `s.query("show bgp adj-rib-in status")` and merge
  by peer address — that avoids R-1 entirely (no `summary.go` -> RIB import), and
  it is ONE extra query, not N+1.

Decision needed: ship `routes_received` alone (honest, conditional on the plugin,
leaves 3 fields zero), or design real post-policy/outbound counters first. Per
this spec's own Failure Routing ("A-4 proves the RIB has no per-peer counts →
STOP, Phase 4 becomes its own spec"), Phase 4 stopped here.

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | `routes_*` pulls the RIB plugin into the summary handler and violates plugin self-containment | design review shows `summary.go` importing the RIB plugin | do NOT import; the RIB contributes via the existing registration mechanism, or the field stays out of scope |
| R-2 | `state_changed` for a never-established peer has no meaningful value | test with a configured-but-down peer | define the semantic explicitly (empty string) and document it; never emit a zero epoch |
| R-3 | Widening `ReactorLifecycle` to expose FSM history breaks the "avoid widening" intent recorded at `types_bgp.go:268` | code review | use the optional-interface type assertion, as `show bgp peer` already does |
| R-4 | We guess the Alice-LG contract wrong and ship a field that looks populated but is misread | no local Alice-LG to test against | verify against upstream birdwatcher source before implementing; record the evidence in this spec |
| R-5 | A NOTIFICATION string from a hostile peer leaks into a PUBLIC looking glass | security review | map code/subcode through a bounded lookup; never echo peer-supplied bytes |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| GET `/api/looking-glass/protocols/bgp` | → | `transformProtocols` emits non-empty `state_changed` | `TestTransformProtocolsStateChangedFromRealSummary` |
| GET `/api/looking-glass/protocols/bgp` | → | `transformProtocols` emits non-empty `last_error` after a NOTIFICATION | `TestTransformProtocolsLastErrorFromRealSummary` |
| GET `/api/looking-glass/protocols/bgp` | → | `transformProtocols` emits non-zero `routes_received` | `TestTransformProtocolsRouteCountsFromRealSummary` |
| GET `/api/looking-glass/protocols` (short) | → | `transformProtocolsShort` emits non-empty `since` | `TestTransformProtocolsShortSinceFromRealSummary` |
| `show bgp summary` | → | `handleBgpSummary` emits the new keys | `TestBgpSummaryEmitsStateChangedAndLastError` |
| `show bgp summary` over the dispatcher | → | full engine path | `test/plugin/lg-birdwatcher-protocols.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Peer established at T | `show bgp summary` peer row has `state-changed` = T as RFC3339 |
| AC-2 | Peer configured, never established | `state-changed` is the documented "never up" value (empty string), never a bogus zero epoch |
| AC-3 | Peer received NOTIFICATION cease/administrative-shutdown | peer row `last-error` renders a human string naming code and subcode |
| AC-4 | Peer never had a NOTIFICATION | `last-error` is the empty string, not a fabricated "none" |
| AC-5 | Peer has N accepted routes in the RIB | peer row `routes-received`/`routes-accepted` reflect the RIB's per-peer counts |
| AC-6 | Any of the above | `transformProtocols` maps them to `state_changed`, `last_error`, `routes_*` with birdwatcher snake_case names and types |
| AC-7 | `transformProtocolsShort` | `since` is populated from the same `state-changed` source |
| AC-8 | Existing consumers | CLI dashboard, web summary page and LG UI still parse the payload unchanged |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Opens Alice-LG and sees how long each session has been up | HTTP -> handleAPIProtocols -> show bgp summary -> transformProtocols -> `state_changed` | `test/plugin/lg-birdwatcher-protocols.ci` |
| 2 | Opens Alice-LG and sees why a peer went down | reactor NOTIFICATION -> PeerInfo.LastNotif* -> summary `last-error` -> `last_error` | `TestTransformProtocolsLastErrorFromRealSummary` |
| 3 | Opens Alice-LG and sees route counts per peer | RIB counters -> summary `routes-*` -> `routes_received`/`routes_imported` | `TestTransformProtocolsRouteCountsFromRealSummary` |
| 4 | Opens the built-in LG HTML UI | same summary payload -> `handler_ui.go` peer table | existing lg UI tests, extended |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBgpSummaryEmitsStateChangedAndLastError` | `internal/component/bgp/plugins/cmd/peer/summary_test.go` | producer emits the new keys (AC-1..AC-4) | |
| `TestTransformProtocolsStateChangedFromRealSummary` | `internal/component/lg/handler_api_test.go` | AC-6 from the real payload shape | |
| `TestTransformProtocolsShortSinceFromRealSummary` | `internal/component/lg/handler_api_test.go` | AC-7 | |
| `TestTransformProtocolsLastErrorFromRealSummary` | `internal/component/lg/handler_api_test.go` | AC-6 | |
| `TestTransformProtocolsRouteCountsFromRealSummary` | `internal/component/lg/handler_api_test.go` | AC-5, AC-6 | |
| `TestLastErrorFormat` | `internal/component/bgp/plugins/cmd/peer/summary_test.go` | code/subcode -> human string; empty when never errored (AC-3, AC-4) | |

<!-- Build every lg test from the REAL producer payload, extending the existing
     realSummaryJSON const in handler_api_test.go. The original bug survived
     precisely because the old test hand-built a map shape the producer never
     emits. Do not reintroduce that pattern. -->

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `routes-*` counts | 0..uint32 max | 4294967295 | N/A (unsigned) | N/A (saturates at source) |
| NOTIFICATION code | 0..255 | 255 | N/A | N/A (uint8) |
| NOTIFICATION subcode | 0..255 | 255 | N/A | N/A (uint8) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `lg-birdwatcher-protocols` | `test/plugin/lg-birdwatcher-protocols.ci` | Alice-LG hits /protocols/bgp and gets a peer with a real since, last_error and route counts | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A | - | - | No wire protocol change: this is an HTTP/API surface, so interop does not apply. The external contract is Alice-LG, covered by A-2 and R-4 | |

## Files to Modify
- `internal/component/lg/handler_api.go` - map the new keys in `transformProtocols` / `transformProtocolsShort`
- `internal/component/lg/handler_api_test.go` - tests built from the real payload
- `internal/component/bgp/plugins/cmd/peer/summary.go` - emit `state-changed`, `last-error`, `routes-*`
- `internal/component/bgp/plugins/cmd/peer/summary_test.go` - producer tests
- `internal/component/plugin/types_bgp.go` - `PeerInfo.LastStateChange` (Phase 3 only, and only if A-3 breaks)
- `internal/component/bgp/reactor/reactor_api.go` - populate that field (Phase 3 only, same condition)
- `docs/architecture/web-interface.md` - document the now-populated fields

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | [ ] no - no new command or config leaf | - |
| CLI commands/flags | [ ] no - `show bgp summary` gains keys, not arguments | - |
| Functional test for new RPC/API | [ ] yes | `test/plugin/lg-birdwatcher-protocols.ci` |
| Pipe completeness | [ ] no - existing command, output already routed | - |
| Doctor check for runtime dependencies | [ ] no - no new runtime dependency | - |
| Prometheus counters/metrics | [ ] no - counters already exist; this exposes them | - |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] yes | `docs/features.md` (LG peer detail) |
| 3 | CLI command added/changed? | [ ] no - no argument change | - |
| 4 | API/RPC added/changed? | [ ] yes | `docs/architecture/api/commands.md` (summary payload keys) |
| 12 | Internal architecture changed? | [ ] yes, if Phase 4 adds a RIB -> summary path | `docs/architecture/core-design.md` |
| 16 | Any changed source file referenced by doc source anchors? | [ ] check | grep `docs/` for `summary.go`, `handler_api.go` |

## Files to Create
- `test/plugin/lg-birdwatcher-protocols.ci` - functional test over the real dispatch path

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. Full verification | `make ze-verify` |
| 6. Critical review | Critical Review Checklist below |
| 13. /ze-review gate | Review Gate section |
| 14. Present summary + close | Executive Summary; two-commit closure |

### Implementation Phases

<!-- Ordered cheapest-and-most-certain first, so each phase is independently
     shippable and Phase 4 can be dropped without stranding Phases 1-3. -->

1. **Phase: Wiring (MANDATORY FIRST)** — failing tests from the real payload shape
   - Tests: the `TestTransformProtocols*FromRealSummary` set, extending `realSummaryJSON`
   - Files: `handler_api_test.go`, `summary_test.go`
   - Verify: tests fail because the keys are absent, not because the shape is wrong
2. **Phase: last_error (cheapest, no reactor change)** — A-1 says the data is already on PeerInfo (`types_bgp.go:113-117`)
   - Tests: `TestLastErrorFormat`, `TestTransformProtocolsLastErrorFromRealSummary`
   - Files: `summary.go` (format + emit `last-error`), `handler_api.go`
   - Verify: a peer with a recorded NOTIFICATION shows a human string; a clean peer shows ""
3. **Phase: state_changed** — resolve A-3 FIRST by reading `p.FSMHistory()` retention
   - Tests: `TestBgpSummaryEmitsStateChangedAndLastError`, `TestTransformProtocolsStateChangedFromRealSummary`, `TestTransformProtocolsShortSinceFromRealSummary`
   - Files: `summary.go`; plus `types_bgp.go` + `reactor_api.go` only if A-3 breaks
   - Verify: established peer shows its transition time; never-established peer shows the documented "never up" value (AC-2)
4. **Phase: routes_* (largest; resolve A-4 and R-1 first)** — needs a design decision on how the RIB contributes per-peer counts without `summary.go` importing the RIB plugin
   - Tests: `TestTransformProtocolsRouteCountsFromRealSummary`
   - Files: TBD after the A-4 investigation
   - Verify: counts match what `show bgp rib` reports for the same peer
5. **Functional test** → `test/plugin/lg-birdwatcher-protocols.ci`
6. **Full verification** → `make ze-verify`
7. **Complete spec** → audit tables + learned summary; two-commit closure

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Naming | New summary keys are kebab-case (`state-changed`, `last-error`, `routes-received`); birdwatcher output stays snake_case (`ai/rules/json-format.md:26`) |
| Test shape | Every lg test is built from the real producer payload, never a hand-made map. This is the exact defect that hid the original bug |
| Data flow | `summary.go` does NOT import the RIB plugin (R-1); route counts arrive via registration |
| Registration over hardcoding | No new per-feature field or switch case in a core/shared package |
| Rule: no-workarounds | No field is faked, defaulted, or stubbed to make a test green. An unavailable value stays empty and is documented |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| `state_changed` populated | `bin/ze-test bgp plugin lg-birdwatcher-protocols` |
| `last_error` populated after NOTIFICATION | `go test ./internal/component/lg/ -run TestTransformProtocolsLastError` |
| `routes_*` non-zero | `go test ./internal/component/lg/ -run TestTransformProtocolsRouteCounts` |
| No consumer regression | `go test ./internal/component/cli/ ./internal/component/web/ ./internal/component/lg/` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Error leakage | `last_error` is served on a PUBLIC, unauthenticated looking glass (`server.go:538` records that the LG is public and read-only). A NOTIFICATION string must not leak internal addresses, config detail, or anything beyond the RFC 4271 code/subcode meaning |
| Input validation | NOTIFICATION code/subcode come from a remote peer. Map them through a bounded lookup, as `notificationCodeLabel` (`peer_stats.go:186`) already does, so a hostile peer cannot inject an arbitrary string (R-5) |
| Resource exhaustion | `PeerFSMHistory` (`reactor.go:847`) copies the whole history per call under `r.mu.RLock()`. The summary path must not copy every peer's full history on every LG poll |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| A-4 proves the RIB has no per-peer counts | STOP. Phase 4 becomes its own spec. Ship Phases 1-3 |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user |

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

- A test built from a hand-made input map cannot detect a producer/consumer
  contract break. The original `TestTransformProtocolsFields` injected a
  top-level `peers` key, `peer-address`, and a numeric `uptime`, none of which
  the producer emits, and passed for as long as the endpoint was completely
  broken in production. Tests at a component seam must be built from the other
  side's real output.
- `getStr`/`getNum` returning zero values for a missing key turns a contract
  break into silence. Every field in this spec was "working" by that standard.

## Core Insight

The birdwatcher transform was written against an imagined payload rather than
the one `show bgp summary` produces. Fixing the three wiring bugs (`bae4f1956`)
made the endpoint work; this spec closes the remaining fields, which were never
implemented on the producing side at all. The lesson generalises: at a seam
between two components, the test fixture must come from the real producer, or
the seam is untested no matter how many tests pass.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Split into phases ordered by certainty, cheapest first | One combined change | `last_error` needs no reactor change (A-1); `routes_*` may need a new RIB surface (A-4). Bundling would hold the cheap, certain fix hostage to the expensive, uncertain one |
| Fix at the producer (`summary.go`), not in `handler_api.go` | Have the LG query extra commands per peer | The LG HTML UI reads the same payload and has the same gap; one producer fix serves both surfaces, and per-peer queries would N+1 the LG poll |
| Keep `routes_*` out of scope until A-4 is answered | Guess a RIB integration now | `peer_stats.go:21,54` draw the boundary deliberately. Crossing it without design risks R-1 |
| Use the optional `FSMHistoryProvider` type assertion | Widen `ReactorLifecycle` | `types_bgp.go:268` records the intent to avoid widening, and `show bgp peer` already sets the precedent |

## Known Limitations
- Until Phase 4 lands, Alice-LG shows zero route counts. That is the status quo, not a regression.
- No local Alice-LG instance to verify the contract against (R-4); upstream birdwatcher source is the reference.
- `transformProtocols` also emits `routes_filtered` and `routes.filtered`, but Ze does not track filtered routes at all (recorded in `ai/rules/project-knowledge.md`). That field may stay zero permanently regardless of Phase 4, and the spec should not pretend otherwise.

## RFC Documentation

Add `// RFC 4271 Section 4.5: "<quoted requirement>"` above the NOTIFICATION
code/subcode rendering used for `last_error`.

## Implementation Summary

### What Was Implemented
- (not started)

### Bugs Found/Fixed
- (not started)

### Documentation Updates
- (not started)

### Deviations from Plan
- (not started)

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

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Alice-LG shows a real "since" per peer | functional test | `test/plugin/lg-birdwatcher-protocols.ci` (not written) |
| Alice-LG shows why a peer went down | functional test | `test/plugin/lg-birdwatcher-protocols.ci` (not written) |
| Alice-LG shows real route counts | functional test | pending A-4 |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- (not started)

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

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes
- [ ] Feature code integrated (`internal/*`)
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`)

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-lg-birdwatcher-peer-fields.md`
