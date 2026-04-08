# Spec: report-bus

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 14/14 |
| Updated | 2026-04-08 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/api/commands.md` - RPC contract conventions
4. `internal/component/bgp/reactor/session_prefix.go` - current prefix-warning state machine
5. `internal/component/bgp/plugins/cmd/peer/peer_warnings.go` - current handler being replaced
6. `internal/component/cmd/show/show.go` - show verb RPC registration site
7. `internal/core/family/registry.go` - reference for the small-core-package pattern

## Task

Ze has no central place for subsystems to report user-facing operational issues. Today, BGP prefix warnings live in per-peer state on `reactor.Peer` and are queried by a single noun-first BGP handler. The path is BGP-only, the YANG is partially wired (`ze-bgp:warnings` registered without a YANG `ze:command`), and there is no equivalent for errors. Other subsystems (interface, plugin, healthcheck, config) have no query path at all, issues surface only in logs, login banners, or per-object status fields.

This spec introduces a single cross-cutting **report bus** package at `internal/core/report/` that any subsystem can push warnings and errors into, plus two new verb-first user-facing RPCs (`ze-show:warnings`, `ze-show:errors`) that read from it. The existing BGP prefix warnings move to the bus as the first producer; BGP NOTIFICATIONs sent and received become the first error producer. The old `ze-bgp:warnings` registration and the `ze-show:bgp-warnings` show entry are deleted (replaced by the new flat top-level paths). The login banner is migrated to read from the bus so there is one source of truth.

Goal: every Ze subsystem reports operational issues to a single API; one operator command (`ze show warnings`, `ze show errors`) returns the full picture.

## Severity Semantics

Two distinct kinds of operator-visible information, with different lifecycle and intent. The distinction is part of the bus contract: producers MUST pick the right severity, the bus does not auto-promote.

| Severity | Meaning | Lifecycle | Storage |
|----------|---------|-----------|---------|
| `warning` | Something is approaching a problem state but nothing has actually gone wrong yet. Operationally fine right now. Predictive, advisory. The condition can resolve, in which case the warning disappears from the active set. | State-based. Producer raises when the condition starts, clears when the condition ends. Re-raising is idempotent (deduped on `Source+Code+Subject`). | Active-set map keyed by `(Source, Code, Subject)`, bounded by `warningCap`, oldest-by-Updated evicted at cap. |
| `error` | Something already happened that the operator should know about. Operationally relevant. Reactive, observed. The event itself does not resolve (although a follow-up state may improve). | Event-based. Producer raises once at the moment of occurrence. There is no clear API for errors. | Bounded ring buffer of size `errorCap`, oldest evicted on overflow. |

Boundary test: "did anything actually fail or behave unexpectedly?" If yes: error. If no, but it might soon: warning.

### Initial vocabulary

Day-one warning codes:

| Source | Code | Subject | Raised when | Cleared when |
|--------|------|---------|-------------|--------------|
| `bgp` | `prefix-threshold` | peer address | Per-family prefix count crosses the warning threshold upward | Per-family count drops back below threshold |
| `bgp` | `prefix-stale` | peer address | `peer.PrefixUpdated` parses to a date older than 180 days | Stale scan ticks and the peer is no longer stale |

Day-one error codes:

| Source | Code | Subject | Raised when |
|--------|------|---------|-------------|
| `bgp` | `notification-sent` | peer address | ze sends a NOTIFICATION to the peer (any code/subcode) |
| `bgp` | `notification-received` | peer address | ze receives a NOTIFICATION from the peer (any code/subcode) |
| `bgp` | `session-dropped` | peer address | FSM leaves Established to Idle without a NOTIFICATION exchange (hold-timer expiry, TCP loss, peer-initiated FIN without NOTIFICATION) |
| `config` | `commit-aborted` | transaction id | Verify phase fails, orchestrator publishes AbortEvent |
| `config` | `commit-rollback` | transaction id | Apply phase fails after partial application, orchestrator publishes RollbackEvent |
| `config` | `commit-save-failed` | transaction id | Apply succeeds but the engine fails to write the resulting config file (`AppliedEvent.Saved == false`) |

A warning that escalates to an error is two distinct entries with two distinct codes. Example: `bgp/prefix-threshold` (warning, count nearing the per-family max) and a future `bgp/prefix-exceeded` (error, count actually hit the max and routes were rejected) coexist independently. The bus does not collapse them.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/commands.md` - how show-verb RPCs are registered and dispatched
  → Decision: handlers register via `pluginserver.RegisterRPCs` with kebab-case wire methods
  → Constraint: every wire method MUST have a matching `ze:command` in a YANG `*-cmd.yang` module
- [ ] `docs/architecture/core-design.md` - small-core-plus-registration pattern
  → Constraint: cross-cutting registries live under `internal/core/<name>/`, never under a single component
- [ ] `.claude/rules/json-format.md` - JSON conventions for output
  → Constraint: all keys kebab-case, error responses use `{"error": "...", "parsed": false}`
- [ ] `.claude/rules/integration-completeness.md` - feature reachability
  → Constraint: `ze-show:errors` MUST have a real source on day one or the spec is blocked, not done
- [ ] `.claude/rules/design-principles.md` - YAGNI + no premature abstraction
  → Constraint: only one source registered today (BGP) means no abstract `Reporter` interface; concrete package functions
- [ ] `.claude/rules/no-layering.md` - replace, do not layer
  → Constraint: `peer.PrefixWarnings` field, `peer.SetPrefixWarned`, `peer.clearPrefixWarned`, `peer.PrefixWarnedFamilies`, `peer_warnings.go`, `peer_warnings_test.go`, and the `ze-bgp:warnings` registration are all deleted, not kept alongside

### RFC Summaries
- [ ] `rfc/short/rfc4271.md` - NOTIFICATION message format (for the error producer)
  → Constraint: NOTIFICATION code/subcode are 8-bit fields; error Detail must capture both

**Key insights:**
- Cross-cutting registry pattern is established (`internal/core/family/`, `internal/core/metrics/`); report fits the same shape
- Push API is forced by errors being inherently event-style, there is nothing to "pull" after a plugin crashes or a NOTIFICATION is sent
- Login banner already filters by peer when displaying, it can use the bus filtered by source/subject equally well
- Handler today walks live reactor state under read locks; replacing with a snapshot from the bus is strictly cheaper

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/session_prefix.go` - prefix counter state machine; calls `peer.SetPrefixWarned(family)` when count exceeds threshold and `peer.clearPrefixWarned(family)` when it drops back; uses `prefixCounts.warned` map keyed by uint32 family-key
  → Constraint: warning state changes happen on UPDATE message processing, not periodically
- [ ] `internal/component/bgp/reactor/peer.go` - `Peer` struct fields `PrefixWarnings` (slice), `prefixWarnedMap` (map under mutex), `PrefixUpdated` (date string); methods `SetPrefixWarned`, `clearPrefixWarned`, `PrefixWarnedFamilies`
  → Constraint: `PrefixWarnings` is a snapshot field populated by `Peers()` API method, not by direct write
- [ ] `internal/component/bgp/reactor/reactor_api.go` - `Peers()` populates `PeerInfo.PrefixWarnings` from `PrefixWarnedFamilies()`
- [ ] `internal/component/bgp/plugins/cmd/peer/peer_warnings.go` - 84-line file; `init()` registers `ze-bgp:warnings` (no YANG entry); `HandleBgpWarnings` walks `ctx.Reactor().Peers()`, builds `[]map[string]any` with two warning kinds (stale-data, threshold-exceeded), uses local `isPrefixStale()` (180 days, duplicated from reactor)
  → Constraint: function reads `peer.PrefixUpdated` for stale check and `peer.PrefixWarnings` for threshold check; both must continue to work after migration
- [ ] `internal/component/bgp/plugins/cmd/peer/peer_warnings_test.go` - four unit tests directly calling `HandleBgpWarnings` with seeded `Peer` objects
- [ ] `internal/component/cmd/show/show.go` - registers `ze-show:bgp-warnings` (active), `ze-show:bgp-peer`, `ze-show:version`, `ze-show:interface`; the `bgp-warnings` registration calls `peer.HandleBgpWarnings`
- [ ] `internal/component/cmd/show/schema/ze-cli-show-cmd.yang` - declares `show > bgp > warnings` (`ze-show:bgp-warnings`); 27 ze-show:* entries total
- [ ] `internal/component/bgp/plugins/cmd/peer/schema/ze-peer-cmd.yang` - `summary` and `peer/*` entries; no `warnings` entry (the missing wiring `validate-commands` flagged)
- [ ] `internal/component/bgp/config/loader.go` - `collectPrefixWarnings` walks reactor peers for the login banner; reads `PrefixWarnings` slice
- [ ] `internal/component/bgp/config/loader_test.go` - tests for banner generation including warning lines
- [ ] `internal/component/plugin/types_bgp.go` - `PeerInfo.PrefixWarnings` field declaration
- [ ] `internal/core/family/registry.go` - reference for cross-cutting package layout; immutable snapshot pattern with `sync.RWMutex` writes / atomic reads
- [ ] `test/plugin/show-bgp-warnings.ci` - functional test exercising `show bgp warnings` end-to-end
- [ ] `scripts/validate-commands.go` - tool that flagged `ze-bgp:warnings` (orphan handler); the spec must leave it returning zero issues for this RPC

**Behavior to preserve:**
- BGP prefix-threshold warnings raised when `prefixCounts.add(fk, delta) >= warningThreshold(family)` and cleared when count drops below
- BGP prefix-stale warnings reported when `peer.PrefixUpdated` parses to a date >180 days old
- Login banner shows one warning in detail and "N warnings" for multiple
- JSON output of `ze show warnings` uses kebab-case keys; the response is a structured object with a `warnings` array and a `count` integer
- Functional test `show-bgp-warnings.ci` (renamed) continues to exercise the user-visible behavior

**Behavior to change:**
- `ze-show:bgp-warnings` and `show > bgp > warnings` deleted; replaced by top-level `ze-show:warnings` and `show > warnings`
- `ze-bgp:warnings` registration in `peer_warnings.go` deleted; the file itself deleted
- `peer.PrefixWarnings`, `peer.SetPrefixWarned`, `peer.clearPrefixWarned`, `peer.PrefixWarnedFamilies`, `peer.prefixWarnedMap` removed; report bus owns the state
- New top-level `ze-show:errors` / `show > errors` returning recent BGP NOTIFICATION events
- Login banner reads from `report.Warnings()` filtered by source `bgp` and subject prefix-matching the peer address

## Data Flow (MANDATORY)

### Entry Point
Five entry points feed the report bus on day one:

1. UPDATE message processing on the reactor session loop (existing prefix counter state machine), raises and clears `bgp/prefix-threshold`
2. Periodic stale-data scan (new), ticks alongside existing reactor housekeeping, walks peers, raises and clears `bgp/prefix-stale`
3. BGP FSM NOTIFICATION send and receive sites (new), raise `bgp/notification-sent` and `bgp/notification-received`
4. BGP FSM Established-to-Idle transitions without NOTIFICATION exchange (new), raise `bgp/session-dropped`
5. Config transaction orchestrator at `AbortEvent`, `RollbackEvent`, and `AppliedEvent` emission sites (new), raise `config/commit-aborted`, `config/commit-rollback`, `config/commit-save-failed`

### Transformation Path

| Stage | What happens |
|-------|--------------|
| 1 | UPDATE arrives at reactor session, `prefixCounts.add(fk, delta)` returns new count |
| 2 | If count crosses warning threshold upward, producer calls `report.RaiseWarning("bgp", "prefix-threshold", peerAddr, msg, detail)` with `detail = {"family": "<afi>/<safi>"}`. Subject family disambiguation is via the detail map plus a derived dedup key (see Stage 3). |
| 3 | If count crosses warning threshold downward, producer calls `report.ClearWarning("bgp", "prefix-threshold", peerAddr, detail)` with the same family detail. Bus dedup key is `(Source, Code, Subject, family)` so per-family clears do not affect other families on the same peer. |
| 4 | Periodic stale scan: for each peer with parseable `PrefixUpdated` older than 180 days, raise `bgp/prefix-stale` with subject = peer address. For peers no longer stale, clear. |
| 5 | NOTIFICATION sent: at the FSM site calling the writer, `report.RaiseError("bgp", "notification-sent", peerAddr, msg, detail)` with `detail = {"code": N, "subcode": M, "data": "<hex>"}` |
| 6 | NOTIFICATION received: at the parser site, `report.RaiseError("bgp", "notification-received", peerAddr, msg, detail)` with the same shape |
| 7 | Session dropped without NOTIFICATION: at the FSM Established-to-Idle transition that did NOT come from a notification path, `report.RaiseError("bgp", "session-dropped", peerAddr, msg, detail)` with `detail = {"reason": "<hold-timer-expired|tcp-reset|peer-fin|local-error>"}` |
| 8 | Config commit aborts in verify: orchestrator publishes AbortEvent and calls `report.RaiseError("config", "commit-aborted", txID, msg, detail)` with `detail = {"reason": <abort reason>, "failing-plugin": <name>}` |
| 9 | Config commit rolls back in apply: orchestrator publishes RollbackEvent and calls `report.RaiseError("config", "commit-rollback", txID, msg, detail)` with `detail = {"reason": ..., "failing-plugin": ...}` |
| 10 | Config save fails after successful apply: orchestrator publishes AppliedEvent with `Saved == false` and calls `report.RaiseError("config", "commit-save-failed", txID, msg, detail)` with `detail = {"path": <config file path>, "error": <io error>}` |
| 11 | Operator runs `ze show warnings`: `ze-show:warnings` handler calls `report.Warnings()`, returns snapshot as JSON with kebab-case keys |
| 12 | Operator runs `ze show errors`: `ze-show:errors` handler calls `report.Errors(limit)`, returns most-recent-first JSON list |
| 13 | Login banner construction: `loader.go` calls `report.Warnings()`, filters by source `bgp` and subject equal to peer address, formats one-or-many message |

The key concurrency point: warning state lives in the bus, not on `Peer`. The prefix counter state machine still owns `prefixCounts`, but the **warned** flag becomes the bus's responsibility, the state machine reads back through `report.IsRaised(source, code, subjectKey)` to decide whether to raise (debounce) or skip (already raised).

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Reactor (UPDATE handler) to report bus | `report.RaiseWarning` / `ClearWarning` from `session_prefix.go` | [ ] |
| Reactor (housekeeping ticker) to report bus | `report.RaiseWarning` / `ClearWarning` from stale scanner in `peer_run.go` | [ ] |
| BGP FSM (notification sites) to report bus | `report.RaiseError` from notification send/parse paths | [ ] |
| BGP FSM (Established-to-Idle) to report bus | `report.RaiseError` from session-drop transition | [ ] |
| Config transaction orchestrator to report bus | `report.RaiseError` at AbortEvent, RollbackEvent, AppliedEvent emission sites | [ ] |
| Report bus to show RPC handler | `report.Warnings()` / `report.Errors(limit)` snapshot read | [ ] |
| Config loader to report bus (banner) | `report.Warnings()` filtered locally by source and subject | [ ] |
| Plugin SDK to report bus (future) | `pkg/ze/report/` re-export, deferred until first external producer | [ ] |

### Integration Points

- `pluginserver.RegisterRPCs` in `internal/component/cmd/show/show.go`: register `ze-show:warnings` and `ze-show:errors`
- `ze-cli-show-cmd.yang`: top-level `warnings` and `errors` containers under `show`
- `reactor/session_prefix.go`: replace direct `peer.SetPrefixWarned` / `clearPrefixWarned` calls with `report.RaiseWarning` / `ClearWarning`
- `reactor/peer_run.go` (or wherever the housekeeping ticker lives): add stale-scan tick
- BGP FSM notification send site (locate via grep on `BuildNotification` callers): add `RaiseError`
- BGP wire NOTIFICATION parse site (locate via grep on `parseNotification` or message-type dispatch): add `RaiseError`
- BGP FSM Established-to-Idle transition (locate in fsm/state files): add `RaiseError` for `session-dropped` paths that did not already raise notification-sent or notification-received
- `internal/component/config/transaction/orchestrator.go`: add `report.RaiseError` calls at the AbortEvent, RollbackEvent, and AppliedEvent (Saved=false) emission sites
- `config/loader.go`: replace `collectPrefixWarnings` body with `report.Warnings()` filter

### Architectural Verification

- [ ] No bypassed layers, reactor only writes to the bus, never reads peer warning state
- [ ] No unintended coupling, `internal/core/report` imports nothing from `internal/component/*`
- [ ] No duplicated functionality, `peer.PrefixWarnings` and friends fully removed; bus is single source
- [ ] Concurrency safe, bus uses `sync.RWMutex`; readers take RLock and copy

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| BGP UPDATE crossing threshold during `.ci` run | -> | `report.RaiseWarning` from `session_prefix.go` | `test/plugin/show-warnings.ci` (verifies `ze show warnings` output) |
| BGP UPDATE dropping below threshold | -> | `report.ClearWarning` from `session_prefix.go` | `test/plugin/show-warnings-clear.ci` |
| `peer.PrefixUpdated` older than 180 days during stale-scan tick | -> | periodic stale scanner in reactor | `test/plugin/show-warnings-stale.ci` |
| Mid-session NOTIFICATION sent by ze | -> | `report.RaiseError` from FSM send site | `test/plugin/show-errors-sent.ci` |
| Mid-session NOTIFICATION received from peer | -> | `report.RaiseError` from FSM parse site | `test/plugin/show-errors-received.ci` |
| Hold-timer expiry on a peer in Established | -> | `report.RaiseError` from FSM session-drop path | `test/plugin/show-errors-session-dropped.ci` |
| Config commit with a verify-failing diff | -> | `report.RaiseError` from orchestrator AbortEvent site | `test/plugin/show-errors-config-abort.ci` |
| Config commit where a plugin fails apply mid-transaction | -> | `report.RaiseError` from orchestrator RollbackEvent site | `test/plugin/show-errors-config-rollback.ci` |
| Config commit where the resulting file write fails (read-only fs) | -> | `report.RaiseError` from orchestrator AppliedEvent (Saved=false) site | `test/plugin/show-errors-config-save.ci` |
| Login banner build with active warning | -> | `report.Warnings()` filter in `loader.go` | `internal/component/bgp/config/loader_test.go::TestBannerWithReportBusWarnings` |

## Acceptance Criteria

### Bus mechanics

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `report.RaiseWarning("bgp", "prefix-threshold", "10.0.0.1", "ipv4/unicast over limit", {"family":"ipv4/unicast"})` called once | `report.Warnings()` returns one entry with matching fields, `Raised == Updated` |
| AC-2 | Same call repeated with a new message | `report.Warnings()` returns one entry (deduped on `Source+Code+Subject+familyKey`), `Updated > Raised`, latest message wins |
| AC-3 | `report.ClearWarning(...)` called for an active key | Subsequent `report.Warnings()` does not contain the entry |
| AC-4 | `report.ClearSource("bgp")` called with N active BGP warnings | `report.Warnings()` returns 0 BGP entries, non-BGP entries (if any) untouched |
| AC-5 | `report.RaiseError` called 257 times when ring buffer cap is 256 | `report.Errors(0)` returns exactly 256 entries, oldest evicted, most recent at index 0 |
| AC-14 | Concurrent goroutines call `RaiseWarning` and `Warnings()` simultaneously | Race detector reports no data races, snapshot returned by `Warnings()` is consistent (no torn reads) |
| AC-15 | `report.Warnings()` would exceed `warningCap` because raises kept piling on with distinct keys | Oldest by `Updated` evicted to keep map at cap, eviction logged at warn level once per minute |
| AC-16 | `report.RaiseError(...)` called for a code that already exists in the ring | A new entry is appended (errors are events, no dedup), both visible in `report.Errors(0)` |
| AC-17 | `report.ClearWarning` called for a key that does not exist | No-op, no panic, no error, no log noise |
| AC-18 | `RaiseWarning` or `RaiseError` called with empty `Source`, `Code`, or `Subject` | Returns early without storing, logs at debug level. Bus rejects malformed entries at the boundary. |

### Warning producers (BGP)

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-6 | BGP session receives UPDATE that pushes count to `warningThreshold` | `ze show warnings` JSON contains an entry with `source=bgp`, `code=prefix-threshold`, `subject=<peer-addr>`, `severity=warning`, `detail.family=<family>` |
| AC-7 | Same session withdraws routes back below threshold | Subsequent `ze show warnings` does NOT contain the prior threshold entry for that peer/family |
| AC-8 | Peer configured with `PrefixUpdated` set to a date older than 180 days, stale scan ticks | `ze show warnings` contains an entry with `code=prefix-stale`, `subject=<peer-addr>`, `severity=warning` |

### Error producers (BGP)

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-9 | ze sends a NOTIFICATION (cease/admin-shutdown, code 6 subcode 2) to a peer | `ze show errors` JSON contains an entry with `code=notification-sent`, `subject=<peer-addr>`, `severity=error`, `detail.code=6`, `detail.subcode=2` |
| AC-10 | ze receives a NOTIFICATION from a peer | `ze show errors` contains entry with `code=notification-received`, peer info, code/subcode in detail |
| AC-19 | Peer in Established state has its hold-timer expire | `ze show errors` contains entry with `code=session-dropped`, `subject=<peer-addr>`, `detail.reason=hold-timer-expired` |
| AC-20 | Peer in Established state loses TCP connection without exchanging NOTIFICATION | `ze show errors` contains entry with `code=session-dropped`, `detail.reason=tcp-loss` |

### Error producers (config)

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-21 | Operator commits a config change that fails verify in at least one plugin | `ze show errors` contains entry with `source=config`, `code=commit-aborted`, `subject=<txID>`, `detail.failing-plugin=<name>`, `detail.reason=<msg>` |
| AC-22 | Apply succeeds for one plugin then fails for another, triggering rollback | `ze show errors` contains entry with `source=config`, `code=commit-rollback`, `subject=<txID>`, `detail.failing-plugin=<name>` |
| AC-23 | Apply succeeds for all plugins but the engine cannot write the resulting config file (read-only filesystem) | `ze show errors` contains entry with `source=config`, `code=commit-save-failed`, `subject=<txID>`, `detail.path=<file path>`, `detail.error=<io error>` |

### Banner, dispatch, hygiene

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-11 | Login banner constructed with one active prefix-threshold warning | Banner text contains the warning detail line, not a count line |
| AC-12 | Login banner constructed with three active prefix-threshold warnings | Banner text contains "3 warnings" summary line, not full detail |
| AC-13 | `make ze-validate-commands` run with the spec applied | Reports zero orphans for `ze-show:warnings`, `ze-show:errors`, reports zero "handlers without YANG" overall (after the iface and monitor tooling-import fixes are in) |
| AC-24 | Operator runs `ze show warnings` against a daemon with zero active warnings | Returns `{"warnings": [], "count": 0}` exit code 0, not an error |
| AC-25 | Operator runs `ze show errors` against a daemon that has not raised any errors | Returns `{"errors": [], "count": 0}` exit code 0 |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRaiseWarningNew` | `internal/core/report/report_test.go` | AC-1, single raise creates one entry | |
| `TestRaiseWarningDedup` | `internal/core/report/report_test.go` | AC-2, repeat raise updates `Updated` only | |
| `TestClearWarning` | `internal/core/report/report_test.go` | AC-3, clear removes entry by key | |
| `TestClearSource` | `internal/core/report/report_test.go` | AC-4, clear by source | |
| `TestErrorsRingBufferEviction` | `internal/core/report/report_test.go` | AC-5, ring eviction at cap |  |
| `TestWarningsCapEviction` | `internal/core/report/report_test.go` | AC-15, oldest-by-Updated eviction |  |
| `TestRaiseClearConcurrent` | `internal/core/report/report_test.go` | AC-14, concurrent raise/clear/snapshot, run with `-race` | |
| `TestSnapshotIsCopy` | `internal/core/report/report_test.go` | snapshot mutation does not affect bus state | |
| `TestRaiseErrorAppendsNoDedup` | `internal/core/report/report_test.go` | AC-16, errors append, no dedup | |
| `TestClearWarningMissingKey` | `internal/core/report/report_test.go` | AC-17, no-op for missing key | |
| `TestRaiseRejectsEmptyFields` | `internal/core/report/report_test.go` | AC-18, malformed input rejected | |
| `TestSessionPrefixThresholdRaisesReport` | `internal/component/bgp/reactor/session_prefix_test.go` | AC-6, UPDATE crossing threshold raises bus entry | |
| `TestSessionPrefixThresholdClearsReport` | `internal/component/bgp/reactor/session_prefix_test.go` | AC-7, withdraw below threshold clears | |
| `TestStaleScanRaisesAndClears` | `internal/component/bgp/reactor/peer_run_test.go` (or new file) | AC-8, stale scanner toggles entries | |
| `TestNotificationSentRaisesError` | `internal/component/bgp/reactor/<fsm-test>.go` | AC-9, error raised at send | |
| `TestNotificationReceivedRaisesError` | `internal/component/bgp/reactor/<fsm-test>.go` | AC-10, error raised at receive | |
| `TestSessionDroppedHoldTimerRaisesError` | `internal/component/bgp/reactor/<fsm-test>.go` | AC-19, hold-timer expiry raises session-dropped | |
| `TestSessionDroppedTCPLossRaisesError` | `internal/component/bgp/reactor/<fsm-test>.go` | AC-20, TCP loss raises session-dropped | |
| `TestCommitAbortRaisesError` | `internal/component/config/transaction/orchestrator_test.go` | AC-21, abort raises commit-aborted | |
| `TestCommitRollbackRaisesError` | `internal/component/config/transaction/orchestrator_test.go` | AC-22, rollback raises commit-rollback | |
| `TestCommitSaveFailedRaisesError` | `internal/component/config/transaction/orchestrator_test.go` | AC-23, save failure raises commit-save-failed | |
| `TestBannerWithReportBusWarnings_Single` | `internal/component/bgp/config/loader_test.go` | AC-11, single warning detail line | |
| `TestBannerWithReportBusWarnings_Multi` | `internal/component/bgp/config/loader_test.go` | AC-12, multi-warning count line | |
| `TestShowWarningsHandler` | `internal/component/cmd/show/show_test.go` | handler returns kebab-case JSON snapshot | |
| `TestShowWarningsEmpty` | `internal/component/cmd/show/show_test.go` | AC-24, empty snapshot returns `{"warnings": [], "count": 0}` | |
| `TestShowErrorsHandler` | `internal/component/cmd/show/show_test.go` | handler returns kebab-case JSON list with limit honoured | |
| `TestShowErrorsEmpty` | `internal/component/cmd/show/show_test.go` | AC-25, empty ring returns `{"errors": [], "count": 0}` | |

### Boundary Tests

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `report.Errors(limit)` argument | 0..warningCap | warningCap | N/A (negative treated as 0 per design) | warningCap+1 (returns warningCap, no error) |
| Warning map size before eviction | 0..warningCap (default 1024) | 1024 | N/A | 1025 (eviction triggers) |
| Error ring buffer size | 0..errorCap (default 256) | 256 | N/A | 257 (oldest evicted) |
| Stale-data threshold (existing) | 180 days | 180 days | 179 days (not stale) | 181 days (stale) |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `show-warnings` | `test/plugin/show-warnings.ci` | Peer receives routes exceeding threshold, operator runs `ze show warnings`, JSON contains the entry | |
| `show-warnings-clear` | `test/plugin/show-warnings-clear.ci` | Same as above, then peer withdraws routes, operator re-runs, JSON empty | |
| `show-warnings-stale` | `test/plugin/show-warnings-stale.ci` | Persisted peer with old `PrefixUpdated`, operator runs `ze show warnings`, sees stale entry | |
| `show-warnings-empty` | `test/plugin/show-warnings-empty.ci` | Daemon with no warnings, operator runs `ze show warnings`, gets empty JSON, exit 0 | |
| `show-errors-sent` | `test/plugin/show-errors-sent.ci` | Operator triggers admin-shutdown of a peer, runs `ze show errors`, sees notification-sent entry | |
| `show-errors-received` | `test/plugin/show-errors-received.ci` | Test peer sends NOTIFICATION, ze receives, operator runs `ze show errors`, sees notification-received entry | |
| `show-errors-session-dropped` | `test/plugin/show-errors-session-dropped.ci` | Peer hold-timer expires, operator runs `ze show errors`, sees session-dropped with reason hold-timer-expired | |
| `show-errors-config-abort` | `test/plugin/show-errors-config-abort.ci` | Commit a config change that fails verify, operator runs `ze show errors`, sees commit-aborted entry | |
| `show-errors-config-rollback` | `test/plugin/show-errors-config-rollback.ci` | Commit a change that succeeds for one plugin and fails for another (forced via test plugin), operator runs `ze show errors`, sees commit-rollback entry | |
| `show-errors-config-save` | `test/plugin/show-errors-config-save.ci` | Commit succeeds in all plugins but file save fails (read-only fs in test sandbox), operator runs `ze show errors`, sees commit-save-failed entry | |
| `show-errors-empty` | `test/plugin/show-errors-empty.ci` | Daemon with no errors, operator runs `ze show errors`, gets empty JSON, exit 0 | |

### Future
- `pkg/ze/report/` re-export for external plugin authors, deferred until the first external plugin needs to push warnings/errors. Tracked in `plan/deferrals.md` after this spec ships.
- `--source <name>` and `--since <duration>` filters on `ze show warnings|errors`, deferred until operators ask. Add as a separate spec.
- Streaming `ze monitor warnings|errors`, deferred. Existing `ze event monitor` already exists for general events; report bus integration with it is a separate spec.

## Files to Modify

- `internal/component/bgp/reactor/session_prefix.go`, replace `peer.SetPrefixWarned` / `clearPrefixWarned` calls with `report.RaiseWarning` / `ClearWarning`, remove the local `warned` map (the bus owns dedup)
- `internal/component/bgp/reactor/peer.go`, delete `PrefixWarnings` field, `prefixWarnedMap`, `SetPrefixWarned`, `clearPrefixWarned`, `PrefixWarnedFamilies`
- `internal/component/bgp/reactor/peer_run.go`, add stale scan tick (or extend existing housekeeping ticker if one exists)
- `internal/component/bgp/reactor/reactor_api.go`, remove `PrefixWarnings` population in `Peers()`
- `internal/component/bgp/reactor/session.go` (and/or sibling FSM files), add `report.RaiseError` at NOTIFICATION send and receive sites and at the Established-to-Idle transition for session-dropped (when no NOTIFICATION was exchanged)
- `internal/component/plugin/types_bgp.go`, delete `PeerInfo.PrefixWarnings` field
- `internal/component/bgp/config/loader.go`, replace `collectPrefixWarnings` body with `report.Warnings()` filter
- `internal/component/config/transaction/orchestrator.go`, add `report.RaiseError` calls at AbortEvent, RollbackEvent, and AppliedEvent (Saved=false) emission sites
- `internal/component/cmd/show/show.go`, register `ze-show:warnings` and `ze-show:errors`, remove `ze-show:bgp-warnings`
- `internal/component/cmd/show/schema/ze-cli-show-cmd.yang`, add top-level `warnings` and `errors` containers, remove `show > bgp > warnings`
- `internal/component/bgp/config/loader_test.go`, update banner tests to seed bus instead of `peer.PrefixWarnings`
- `internal/component/bgp/reactor/session_prefix_test.go`, update existing prefix-threshold tests to assert via bus
- `internal/component/config/transaction/orchestrator_test.go`, add tests for the three config error producers
- `test/plugin/show-bgp-warnings.ci`, renamed (see Files to Create), old path deleted
- `scripts/validate-commands.go` (or its renamed location `scripts/docvalid/commands.go` per concurrent reorg), add missing imports for `internal/component/iface/cmd` and `internal/component/bgp/plugins/cmd/monitor/schema` (these are tooling bugs surfaced by the same investigation, including here so the doc-drift baseline is clean before the bus work)
- `docs/architecture/api/commands.md`, add report-bus contract section
- `docs/features.md`, add user-facing description of `ze show warnings` / `ze show errors`
- `docs/architecture/core-design.md`, add `internal/core/report/` to the cross-cutting registries section
- `docs/guide/operational-reports.md`, new operator guide page describing warnings vs errors, the day-one vocabulary, and how to clear / triage entries

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [ ] | `internal/component/cmd/show/schema/ze-cli-show-cmd.yang` |
| CLI commands/flags | [ ] | None, `show` verb dispatch is YANG-driven |
| Editor autocomplete | [ ] | YANG-driven (automatic) |
| Functional test for new RPC | [ ] | `test/plugin/show-warnings*.ci`, `test/plugin/show-errors*.ci` |

### Documentation Update Checklist

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md`, add report-bus operator commands |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md`, `ze show warnings`, `ze show errors` |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md`, `ze-show:warnings`, `ze-show:errors`, push API |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | Yes | `docs/guide/troubleshooting.md` (or new `docs/guide/operational-reports.md`), operator workflow for diagnosing live issues |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No (deferred) | `pkg/ze/report/` re-export future spec |
| 9 | RFC behavior implemented? | No | - |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md`, operator-visible warning/error query (compare with bird `show protocols all`, frr `show bgp summary`) |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md`, add `internal/core/report/` to the cross-cutting registries section |

## Files to Create

- `internal/core/report/report.go`, package doc, `Severity` enum, `Entry` struct, package-level store with `sync.RWMutex`, `Raise*` / `Clear*` / `Warnings` / `Errors` functions
- `internal/core/report/store.go`, the warning map plus error ring storage (split if `report.go` exceeds 600 lines per `rules/file-modularity.md`)
- `internal/core/report/report_test.go`, unit tests covering AC-1 through AC-5, AC-14 through AC-18
- `test/plugin/show-warnings.ci`, functional test for prefix-threshold raise plus query
- `test/plugin/show-warnings-clear.ci`, functional test for raise then clear
- `test/plugin/show-warnings-stale.ci`, functional test for stale-data warning
- `test/plugin/show-warnings-empty.ci`, functional test for empty snapshot
- `test/plugin/show-errors-sent.ci`, functional test for notification-sent error
- `test/plugin/show-errors-received.ci`, functional test for notification-received error
- `test/plugin/show-errors-session-dropped.ci`, functional test for hold-timer expiry session-dropped error
- `test/plugin/show-errors-config-abort.ci`, functional test for verify-phase commit abort error
- `test/plugin/show-errors-config-rollback.ci`, functional test for apply-phase commit rollback error
- `test/plugin/show-errors-config-save.ci`, functional test for config save failure after successful apply
- `test/plugin/show-errors-empty.ci`, functional test for empty error ring
- `docs/guide/operational-reports.md`, new operator guide page

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Implementation phases below |
| 4. Full verification | `make ze-verify` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report |

### Implementation Phases

1. **Phase: report package skeleton**, create `internal/core/report/` with types and store
   - Tests: `TestRaiseWarningNew`, `TestRaiseWarningDedup`, `TestClearWarning`, `TestClearSource`, `TestErrorsRingBufferEviction`, `TestWarningsCapEviction`, `TestRaiseClearConcurrent` (with `-race`), `TestSnapshotIsCopy`, `TestRaiseErrorAppendsNoDedup`, `TestClearWarningMissingKey`, `TestRaiseRejectsEmptyFields`
   - Files: `internal/core/report/report.go`, `internal/core/report/report_test.go` (and `store.go` if split needed)
   - Verify: `go test -race ./internal/core/report/...` passes

2. **Phase: validate-commands tooling fix**, add the two missing imports first so the doc-drift baseline is zero before bus migration
   - Tests: none (script change)
   - Files: `scripts/validate-commands.go` (or `scripts/docvalid/commands.go` after concurrent rename lands)
   - Verify: `make ze-validate-commands` reports only `ze-bgp:warnings` orphan handler (1 issue), not 32

3. **Phase: BGP prefix-threshold migration to bus**, replace peer state with bus calls
   - Tests: `TestSessionPrefixThresholdRaisesReport`, `TestSessionPrefixThresholdClearsReport`
   - Files: `internal/component/bgp/reactor/session_prefix.go`, `internal/component/bgp/reactor/peer.go` (delete fields and methods), `internal/component/bgp/reactor/reactor_api.go`, `internal/component/plugin/types_bgp.go`, `internal/component/bgp/reactor/session_prefix_test.go`
   - Verify: targeted unit tests pass, full reactor package builds

4. **Phase: BGP prefix-stale scanner**, periodic raise / clear of stale entries
   - Tests: `TestStaleScanRaisesAndClears`
   - Files: `internal/component/bgp/reactor/peer_run.go`
   - Verify: targeted unit test passes

5. **Phase: BGP NOTIFICATION error producers**, raise errors at FSM send and receive sites
   - Tests: `TestNotificationSentRaisesError`, `TestNotificationReceivedRaisesError`
   - Files: BGP FSM and wire send / parse paths (locate via grep on existing notification handling)
   - Verify: targeted unit tests pass

6. **Phase: BGP session-dropped error producer**, raise errors at Established-to-Idle transitions that did not exchange a NOTIFICATION
   - Tests: `TestSessionDroppedHoldTimerRaisesError`, `TestSessionDroppedTCPLossRaisesError`
   - Files: BGP FSM state-transition file(s)
   - Verify: targeted unit tests pass

7. **Phase: config commit error producers**, raise errors at orchestrator AbortEvent, RollbackEvent, AppliedEvent (Saved=false) sites
   - Tests: `TestCommitAbortRaisesError`, `TestCommitRollbackRaisesError`, `TestCommitSaveFailedRaisesError`
   - Files: `internal/component/config/transaction/orchestrator.go`, `internal/component/config/transaction/orchestrator_test.go`
   - Verify: targeted unit tests pass

8. **Phase: show RPC handlers**, register `ze-show:warnings` and `ze-show:errors`, delete `ze-show:bgp-warnings`
   - Tests: `TestShowWarningsHandler`, `TestShowWarningsEmpty`, `TestShowErrorsHandler`, `TestShowErrorsEmpty`
   - Files: `internal/component/cmd/show/show.go`, `internal/component/cmd/show/schema/ze-cli-show-cmd.yang`
   - Verify: targeted unit tests pass, `make ze-validate-commands` reports zero orphans

9. **Phase: peer_warnings.go deletion**, remove the dead handler file and its tests
   - Tests: none new, the functional tests in the next phase cover the user-visible path
   - Files: delete `internal/component/bgp/plugins/cmd/peer/peer_warnings.go` and `peer_warnings_test.go`
   - Verify: `go build ./...` passes

10. **Phase: login banner migration**, banner reads from bus instead of peer fields
    - Tests: `TestBannerWithReportBusWarnings_Single`, `TestBannerWithReportBusWarnings_Multi`
    - Files: `internal/component/bgp/config/loader.go`, `internal/component/bgp/config/loader_test.go`
    - Verify: targeted unit tests pass

11. **Phase: functional tests**, rename old `.ci`, add new ones
    - Tests: `show-warnings.ci`, `show-warnings-clear.ci`, `show-warnings-stale.ci`, `show-warnings-empty.ci`, `show-errors-sent.ci`, `show-errors-received.ci`, `show-errors-session-dropped.ci`, `show-errors-config-abort.ci`, `show-errors-config-rollback.ci`, `show-errors-config-save.ci`, `show-errors-empty.ci`
    - Files: `test/plugin/show-bgp-warnings.ci` deleted, new files created
    - Verify: `bin/ze-test plugin show-warnings`, etc.

12. **Phase: documentation updates**, fill the Documentation Update Checklist
    - Files: `docs/features.md`, `docs/guide/command-reference.md`, `docs/architecture/api/commands.md`, `docs/comparison.md`, `docs/architecture/core-design.md`, `docs/guide/operational-reports.md` (new)
    - Verify: read each file post-edit, source anchors added where claims are made

13. **Full verification**, `make ze-verify` clean
14. **Complete spec**, fill audit and verification tables, write learned summary, two-commit sequence per `rules/spec-preservation.md`

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line; no TODO/FIXME in new code |
| Correctness | Threshold raise/clear preserves existing 4-test prefix-limit semantics; banner formatting unchanged |
| Naming | JSON keys kebab-case; YANG paths kebab-case; package-level functions follow `internal/core/family/` style |
| Data flow | Bus is single source of truth, `peer.PrefixWarnings` and friends fully gone, no shadow state |
| Rule: no-layering | Old paths fully deleted: `ze-bgp:warnings` registration, `ze-show:bgp-warnings`, `peer_warnings.go`, peer warning fields |
| Rule: integration-completeness | `ze-show:errors` has at least one real producer wired on day one (NOTIFICATION sent + received) |
| Rule: spec-preservation | Two-commit sequence used: code+spec, then `git rm` spec + add learned summary |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| `internal/core/report/report.go` exists | `ls internal/core/report/report.go` |
| Package-level `RaiseWarning` function | `grep -n 'func RaiseWarning' internal/core/report/report.go` |
| `ze-show:warnings` registered | `grep -n 'ze-show:warnings' internal/component/cmd/show/show.go` |
| `ze-show:errors` registered | `grep -n 'ze-show:errors' internal/component/cmd/show/show.go` |
| YANG `warnings` top-level container | `grep -n 'container warnings' internal/component/cmd/show/schema/ze-cli-show-cmd.yang` |
| YANG `errors` top-level container | `grep -n 'container errors' internal/component/cmd/show/schema/ze-cli-show-cmd.yang` |
| Old `ze-show:bgp-warnings` gone | `! grep -n 'ze-show:bgp-warnings' internal/component/cmd/show/show.go internal/component/cmd/show/schema/ze-cli-show-cmd.yang` |
| Old `peer_warnings.go` gone | `! ls internal/component/bgp/plugins/cmd/peer/peer_warnings.go` |
| Old `peer.PrefixWarnings` gone | `! grep -n 'PrefixWarnings' internal/component/bgp/reactor/peer.go internal/component/plugin/types_bgp.go` |
| `make ze-validate-commands` clean for these RPCs | run target, grep for `ze-show:warnings`, `ze-show:errors`, `ze-bgp:warnings` in output, none should appear as orphan |
| Five new `.ci` tests | `ls test/plugin/show-warnings*.ci test/plugin/show-errors-*.ci` |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | `Source`, `Code`, `Subject` are short, bounded strings; reject empty values; reject strings >256 bytes |
| Resource exhaustion | Warning map cap (1024) and error ring cap (256) enforced; cannot grow unbounded; eviction logged |
| Error leakage | `Detail` map values are not echoed into log lines (operator-visible JSON only); no PII in `Subject` beyond what is already logged |
| Concurrency safety | All reads under RLock, writes under Lock; snapshot returned by `Warnings()` is a copy not a reference into the map |
| Source spoofing | Package-level functions trust callers; no authentication needed since callers are in-process; document this clearly |
| Env var registration | If caps are configurable via env (`ze.report.warnings.max`, `ze.report.errors.max`), MUST register via `env.MustRegister` per `rules/go-standards.md` |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior; revisit spec if mismatch is real |
| Lint failure | Fix inline; if architectural, back to DESIGN |
| Functional test fails | Check AC; if AC wrong → DESIGN; if AC correct → IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| `ze-bgp:warnings` was a stale duplicate registration | It was an implemented-but-never-wired feature; YANG entry was missing | User pushed back; checked `ze-peer-cmd.yang` and found pattern of paired registrations elsewhere (`peer-detail`) | Reframed the fix from "delete dead code" to "wire the missing path", then escalated to full report-bus design |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Add a single `container warnings` to `ze-peer-cmd.yang` (minimal fix) | Operator wanted one cross-subsystem reporting place, not BGP-specific noun-first | Top-level `ze-show:warnings`/`ze-show:errors` + report bus |
| Pull-style `WarningSource` interface with subsystems implementing `Warnings()` | Lock contention on every query; errors are inherently event-style and have nothing to "pull" | Push API: subsystems call `Raise*`/`Clear*` |
| Layer `report.RaiseWarning` alongside existing `peer.SetPrefixWarned` for migration | Violates `rules/no-layering.md`; double bookkeeping on hot path | Single migration: replace peer fields with bus calls in one phase |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|
| Treating `validate-commands` orphans as tooling bugs without checking each orphan against its actual code | First time this session | When `validate-commands` reports an orphan, grep for the wire method and inspect the registration site BEFORE assuming it is stale | Add to `rules/before-writing-code.md` if it recurs |

## Design Insights

- The validate-commands tool's "orphan" categorization conflates three different bug shapes: (1) stale registration, (2) missing YANG, (3) handler in a package the tool forgot to import. Investigation must distinguish all three before proposing fixes.
- The pattern `peer-detail` in `ze-peer-cmd.yang` (with both noun-first RPC + show shadow) was the precedent that proved `ze-bgp:warnings` was a missed wiring, not stale code.
- Push API beats pull for cross-cutting reporting because errors are events, not state. Warnings can be pull-style in principle but get unified under push for consistency and to allow lock-free reads from queries.
- `internal/core/report/` slots into the existing cross-cutting registry pattern (`family`, `metrics`, `env`, `clock`), no new architectural concept, just a new instance of an established one.

## RFC Documentation

NOTIFICATION error format (RFC 4271 §6) is referenced when implementing the error producer at the FSM send/parse sites. Add `// RFC 4271 §6: NOTIFICATION error code/subcode` comment near the error-raising calls.

## Implementation Summary

### What Was Implemented
- `internal/core/report/` package: 30 unit tests, concurrent-safe store, env-configurable caps, length-bounded Raise/Clear helpers, Severity enum with JSON marshaling
- `ze-show:warnings` / `ze-show:errors` RPCs registered with YANG in `internal/component/cmd/show/`, dispatched via the canonical `pluginserver.RegisterRPCs` path
- BGP producers wired end-to-end: `session_prefix.go` raises/clears `prefix-threshold` and `prefix-stale`, `peer_stats.go` raises `notification-sent`/`notification-received`, `peer_run.go` raises `session-dropped` on Established-to-Idle transitions that did not go through a NOTIFICATION exchange
- Config commit producers wired in `transaction/orchestrator.go`: `publishAbort` raises `commit-aborted`, `publishRollback` raises `commit-rollback`, `writeConfigFile` raises `commit-save-failed`. Unit tests added in `orchestrator_test.go` (`TestCommitAbortRaisesReportError`, `TestCommitRollbackRaisesReportError`, `TestCommitSaveFailedRaisesReportError`) cover all three paths via the testGateway fake without needing a running server
- Banner migrated: `config/loader.go` `collectPrefixWarnings` now reads from `report.Warnings()` and filters by source/subject
- Old code removed: `peer_warnings.go`, `PrefixWarnings` field on `Peer` and `PeerInfo`, `SetPrefixWarned` / `clearPrefixWarned` / `PrefixWarnedFamilies` / `prefixWarnedMap`, the `ze-bgp:warnings` registration, and the `ze-show:bgp-warnings` show entry
- Python SDK extension: `test/scripts/ze_api.py` now supports `on_config_verify`, `on_config_apply`, and `on_config_rollback` handlers so Python `.ci` test plugins can exercise the RPC bridge's config transaction path (used by `show-errors-config-abort.ci`)
- Functional tests: `show-warnings.ci` (stale path), `show-errors-sent.ci` (notification-sent), `show-errors-received.ci` (notification-received), `show-errors-config-abort.ci` (commit-aborted end-to-end via Python test plugin that rejects verify)
- Documentation: `docs/features.md`, `docs/guide/operational-reports.md` (new), `docs/architecture/api/commands.md`, `docs/architecture/core-design.md`, `docs/comparison.md` updated with report-bus entries

### Bugs Found/Fixed
- `validate-commands` orphan categorization conflated stale-registration with missing-import (fixed by adding iface/monitor imports to `scripts/docvalid/commands.go` so the tooling baseline is zero before the bus migration)
- 9 findings from the first-pass review resolved in commit `f6da6a28` (details in that commit message)

### Documentation Updates
- `docs/features.md`: `ze show warnings` / `ze show errors` listed under operator commands
- `docs/guide/operational-reports.md` (new): operator workflow for diagnosing live issues via the report bus
- `docs/guide/command-reference.md`: added both new show commands
- `docs/architecture/api/commands.md`: `ze-show:warnings`/`ze-show:errors` RPC contract and the push API rationale
- `docs/architecture/core-design.md`: `internal/core/report/` listed in the cross-cutting registries section alongside family/metrics/env/clock
- `docs/comparison.md`: comparison row for operator-visible warning/error query vs bird/frr

### Deviations from Plan
- Python SDK `on_config_verify` / `on_config_apply` / `on_config_rollback` handlers added to `test/scripts/ze_api.py` as a scope expansion (not in the original spec) to unblock `show-errors-config-abort.ci`. The Go SDK already supports these callbacks; the spec originally assumed `.ci` plugins would use Go test helpers, but the existing Python test plugin pattern is simpler and covers the same ground.
- The original `show-warnings.ci` test is the stale-data scenario (it sets `updated 2024-01-01`) so a separate `show-warnings-stale.ci` would duplicate coverage. Consolidated to a single .ci test.
- `show-warnings-clear.ci`, `show-errors-config-rollback.ci`, `show-errors-config-save.ci` deferred: triggering apply-phase failure and config-file-write failure from a .ci test plugin requires additional infra (multi-phase toggling and read-only filesystem handling) that is not yet in place. All three ACs have unit tests that assert the producer path; the missing coverage is end-to-end wiring only. Tracked in `plan/deferrals.md`.
- `show-warnings-empty.ci` and `show-errors-empty.ci` deferred: both hit the same empty-bus daemon-shutdown hang tracked in `plan/deferrals.md` as a focused debugging item. Unit tests `TestHandleShowWarningsEmpty` and `TestHandleShowErrorsEmpty` cover AC-24/AC-25 deterministically at the handler level.
- `show-errors-session-dropped.ci` deferred: requires ze-peer to expose an abrupt-close action (TCP reset without NOTIFICATION). Unit test `TestRaiseSessionDropped` covers the producer directly.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Cross-cutting report bus package | Done | `internal/core/report/report.go` | 30 unit tests, concurrent-safe atomic store |
| Severity vocabulary (warning/error) | Done | `internal/core/report/report.go` (Severity enum) | JSON marshal/unmarshal round-trips |
| BGP prefix-threshold producer | Done | `internal/component/bgp/reactor/session_prefix.go:84/101` | Raises + clears on UPDATE processing |
| BGP prefix-stale producer | Done | `internal/component/bgp/reactor/session_prefix.go:114/123/129` | Scanner-driven raise/clear |
| BGP notification-sent/received producers | Done | `internal/component/bgp/reactor/peer_stats.go:147/165` via `raiseNotificationError` | Called from FSM send/receive sites |
| BGP session-dropped producer | Done | `internal/component/bgp/reactor/peer_run.go:353` via `raiseSessionDropped` | Suppressed when NOTIFICATION already raised |
| Config commit-aborted producer | Done | `internal/component/config/transaction/orchestrator.go:562` in `publishAbort` | Alongside AbortEvent emit |
| Config commit-rollback producer | Done | `internal/component/config/transaction/orchestrator.go:581` in `publishRollback` | Alongside RollbackEvent emit |
| Config commit-save-failed producer | Done | `internal/component/config/transaction/orchestrator.go:721` in `writeConfigFile` | Fires when config writer returns error |
| `ze-show:warnings` RPC | Done | `internal/component/cmd/show/show.go:29` + YANG `ze-cli-show-cmd.yang:18` | `handleShowWarnings` reads snapshot |
| `ze-show:errors` RPC | Done | `internal/component/cmd/show/show.go:33` + YANG `ze-cli-show-cmd.yang:24` | `handleShowErrors` reads ring |
| Legacy code deletion | Done | `peer_warnings.go`, `PrefixWarnings` field, `ze-bgp:warnings`, `ze-show:bgp-warnings` gone | Verified via grep (see Pre-Commit Verification) |
| Login banner migration | Done | `internal/component/bgp/config/loader.go:525` | Banner reads `report.Warnings()` filtered by source/subject |
| Documentation | Done | `docs/features.md`, `docs/guide/operational-reports.md`, `docs/architecture/api/commands.md`, `docs/architecture/core-design.md`, `docs/comparison.md` | 5 files updated |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestRaiseWarningNew` (`internal/core/report/report_test.go:14`) | Single raise creates one entry with Raised==Updated |
| AC-2 | Done | `TestRaiseWarningDedup` (`report_test.go:41`) | Repeat raise updates Updated only |
| AC-3 | Done | `TestClearWarning` (`report_test.go:78`) | Clear removes entry |
| AC-4 | Done | `TestClearSource` (`report_test.go:92`) | Clear-by-source removes all matching |
| AC-5 | Done | `TestErrorsRingBufferEviction` (`report_test.go:109`) | 257 raises on cap 256 yields 256 |
| AC-6 | Done | `TestSessionPrefixThresholdRaisesAndClearsReport` (`session_prefix_test.go:394`) | UPDATE crossing threshold raises |
| AC-7 | Done | Same test (above) asserts clear on withdraw below threshold | Raise and clear tested together |
| AC-8 | Done | `TestPrefixStalenessCheck` (`session_prefix_test.go:315`) + `show-warnings.ci` functional test | Stale >180 days triggers raise |
| AC-9 | Done | `TestIncrNotificationSentRaisesReport` (`peer_stats_test.go:302`) + `show-errors-sent.ci` | NOTIFICATION send raises error |
| AC-10 | Done | `TestIncrNotificationReceivedRaisesReport` (`peer_stats_test.go:329`) + `show-errors-received.ci` | NOTIFICATION receive raises error |
| AC-11 | Done | `TestCollectPrefixWarningsOneStale` (`loader_test.go:1541`) | Single warning in banner |
| AC-12 | Done | `TestCollectPrefixWarningsMultiple` (`loader_test.go:1567`) | Multi-warning summary line |
| AC-13 | Done | `make ze-validate-commands` run during `make ze-verify` | Zero orphans after iface/monitor import fixes |
| AC-14 | Done | `TestRaiseClearConcurrent` (`report_test.go:169`) | -race concurrent raise/clear/snapshot |
| AC-15 | Done | `TestWarningsCapEviction` (`report_test.go:148`) | Oldest-by-Updated evicted at cap |
| AC-16 | Done | `TestRaiseErrorAppendsNoDedup` (`report_test.go:253`) | Errors are events, no dedup |
| AC-17 | Done | `TestClearWarningMissingKey` (`report_test.go:267`) | Missing-key clear is no-op |
| AC-18 | Done | `TestRaiseRejectsEmptyFields` (`report_test.go:277`) | Empty fields rejected |
| AC-19 | Done | `TestRaiseSessionDropped` (`peer_stats_test.go:354`) | Hold-timer expiry path |
| AC-20 | Partial | Same test covers the raise helper; TCP loss path uses the same code but has no dedicated unit test | Producer wired at peer_run.go:353; raiseSessionDropped is called for every non-NOTIFICATION Idle transition |
| AC-21 | Done | `TestCommitAbortRaisesReportError` (`orchestrator_test.go` new) + `show-errors-config-abort.ci` functional test | Verify-fail triggers commit-aborted |
| AC-22 | Done | `TestCommitRollbackRaisesReportError` (`orchestrator_test.go` new) | Apply-fail triggers commit-rollback |
| AC-23 | Done | `TestCommitSaveFailedRaisesReportError` (`orchestrator_test.go` new) | ConfigWriter error triggers commit-save-failed |
| AC-24 | Done | `TestHandleShowWarningsEmpty` (`show_test.go:21`) | Handler returns `{"warnings":[],"count":0}` on empty |
| AC-25 | Done | `TestHandleShowErrorsEmpty` (`show_test.go:100`) | Handler returns `{"errors":[],"count":0}` on empty |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestRaiseWarningNew` | Done | `internal/core/report/report_test.go` | Passing |
| `TestRaiseWarningDedup` | Done | `internal/core/report/report_test.go` | Passing |
| `TestClearWarning` | Done | `internal/core/report/report_test.go` | Passing |
| `TestClearSource` | Done | `internal/core/report/report_test.go` | Passing |
| `TestErrorsRingBufferEviction` | Done | `internal/core/report/report_test.go` | Passing |
| `TestWarningsCapEviction` | Done | `internal/core/report/report_test.go` | Passing |
| `TestRaiseClearConcurrent` | Done | `internal/core/report/report_test.go` | Passing with -race |
| `TestSnapshotIsCopy` | Done | `internal/core/report/report_test.go` | Passing |
| `TestRaiseErrorAppendsNoDedup` | Done | `internal/core/report/report_test.go` | Passing |
| `TestClearWarningMissingKey` | Done | `internal/core/report/report_test.go` | Passing |
| `TestRaiseRejectsEmptyFields` | Done | `internal/core/report/report_test.go` | Passing |
| `TestSessionPrefixThresholdRaisesReport` | Done | `session_prefix_test.go` (renamed to `TestSessionPrefixThresholdRaisesAndClearsReport`) | Covers AC-6 and AC-7 |
| `TestSessionPrefixThresholdClearsReport` | Done | Consolidated into the raise+clear test above | Same path |
| `TestStaleScanRaisesAndClears` | Done | `TestPrefixStalenessCheck` (`session_prefix_test.go:315`) | Named differently in implementation |
| `TestNotificationSentRaisesError` | Done | `TestIncrNotificationSentRaisesReport` (`peer_stats_test.go:302`) | Renamed during implementation |
| `TestNotificationReceivedRaisesError` | Done | `TestIncrNotificationReceivedRaisesReport` (`peer_stats_test.go:329`) | Renamed during implementation |
| `TestSessionDroppedHoldTimerRaisesError` | Done | `TestRaiseSessionDropped` (`peer_stats_test.go:354`) | Covers helper directly |
| `TestSessionDroppedTCPLossRaisesError` | Partial | Same helper; TCP loss path exercises the same producer call but has no dedicated test case | Producer is shared |
| `TestCommitAbortRaisesError` | Done | `TestCommitAbortRaisesReportError` (`orchestrator_test.go`) | Added in final pass |
| `TestCommitRollbackRaisesError` | Done | `TestCommitRollbackRaisesReportError` (`orchestrator_test.go`) | Added in final pass |
| `TestCommitSaveFailedRaisesError` | Done | `TestCommitSaveFailedRaisesReportError` (`orchestrator_test.go`) | Added in final pass |
| `TestBannerWithReportBusWarnings_Single` | Done | `TestCollectPrefixWarningsOneStale` (`loader_test.go:1541`) | Renamed during implementation |
| `TestBannerWithReportBusWarnings_Multi` | Done | `TestCollectPrefixWarningsMultiple` (`loader_test.go:1567`) | Renamed during implementation |
| `TestShowWarningsHandler` | Done | `TestHandleShowWarningsPopulated` (`show_test.go:48`) | Renamed |
| `TestShowWarningsEmpty` | Done | `TestHandleShowWarningsEmpty` (`show_test.go:21`) | Renamed |
| `TestShowErrorsHandler` | Done | `TestHandleShowErrorsPopulated` (`show_test.go:127`) | Renamed |
| `TestShowErrorsEmpty` | Done | `TestHandleShowErrorsEmpty` (`show_test.go:100`) | Renamed |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/core/report/report.go` | Done | Package created with full API |
| `internal/core/report/report_test.go` | Done | 30 tests |
| `internal/component/bgp/reactor/session_prefix.go` | Done | Migrated to bus |
| `internal/component/bgp/reactor/peer.go` | Done | Old fields deleted |
| `internal/component/bgp/reactor/peer_run.go` | Done | session-dropped producer wired |
| `internal/component/bgp/reactor/reactor_api.go` | Done | `PeerInfo.PrefixWarnings` removed |
| `internal/component/bgp/reactor/peer_stats.go` | Done | notification producers wired |
| `internal/component/plugin/types_bgp.go` | Done | `PeerInfo.PrefixWarnings` deleted |
| `internal/component/bgp/config/loader.go` | Done | Banner reads from bus |
| `internal/component/config/transaction/orchestrator.go` | Done | Three commit error producers |
| `internal/component/config/transaction/orchestrator_test.go` | Done | Three new unit tests added in final pass |
| `internal/component/cmd/show/show.go` | Done | Two new RPCs, old one removed |
| `internal/component/cmd/show/schema/ze-cli-show-cmd.yang` | Done | Top-level `warnings`/`errors` containers |
| `internal/component/bgp/plugins/cmd/peer/peer_warnings.go` | Deleted | Entire file removed |
| `internal/component/bgp/plugins/cmd/peer/peer_warnings_test.go` | Deleted | Entire file removed |
| `test/scripts/ze_api.py` | Done | Added on_config_verify/apply/rollback handler support |
| `test/plugin/show-warnings.ci` | Done | Covers AC-8 stale-data path |
| `test/plugin/show-errors-sent.ci` | Done | Covers AC-9 |
| `test/plugin/show-errors-received.ci` | Done | Covers AC-10 |
| `test/plugin/show-errors-config-abort.ci` | Done | Covers AC-21 end-to-end via Python test plugin |
| `test/plugin/show-warnings-clear.ci` | Deferred | Requires prefix threshold crossing infra (many-route peer injection) |
| `test/plugin/show-warnings-stale.ci` | Skipped | Duplicates `show-warnings.ci` (same path) |
| `test/plugin/show-warnings-empty.ci` | Deferred | Shares empty-bus shutdown hang with show-errors-empty |
| `test/plugin/show-errors-session-dropped.ci` | Deferred | Requires ze-peer abrupt-close action |
| `test/plugin/show-errors-config-rollback.ci` | Deferred | Requires .ci plugin toggle: accept verify, fail apply mid-transaction |
| `test/plugin/show-errors-config-save.ci` | Deferred | Requires read-only filesystem handling in test runner |
| `test/plugin/show-errors-empty.ci` | Deferred | Empty-bus daemon-shutdown hang, needs focused debug |
| `docs/guide/operational-reports.md` | Done | New operator guide page |
| `docs/features.md` | Done | Added show warnings/errors |
| `docs/guide/command-reference.md` | Done | Added both commands |
| `docs/architecture/api/commands.md` | Done | Added RPC + push API rationale |
| `docs/architecture/core-design.md` | Done | Added `internal/core/report/` |
| `docs/comparison.md` | Done | Added comparison row |
| `scripts/docvalid/commands.go` | Done | Iface/monitor imports added |

### Audit Summary
- **Total items:** 25 ACs, 27 tests, 31 files
- **Done:** 24 ACs (AC-1 through AC-25 except AC-20 partial), 25 tests (2 renamed and consolidated), 25 files created/modified + 2 deleted
- **Partial:** AC-20 (TCP loss path covered by shared raiseSessionDropped helper but no dedicated test case)
- **Skipped:** `show-warnings-stale.ci` (duplicates `show-warnings.ci`)
- **Deferred:** 6 `.ci` tests (`show-warnings-clear`, `show-warnings-empty`, `show-errors-session-dropped`, `show-errors-config-rollback`, `show-errors-config-save`, `show-errors-empty`) tracked in `plan/deferrals.md` with explicit blockers
- **Changed:** Several test names differ from the plan (renamed during implementation); all mapped in the Tests from TDD Plan table above

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/core/report/report.go` | Yes | `ls internal/core/report/report.go` -> 16K file |
| `internal/core/report/report_test.go` | Yes | `ls internal/core/report/report_test.go` -> 20K file |
| `docs/guide/operational-reports.md` | Yes | `ls docs/guide/operational-reports.md` -> 8.8K file |
| `test/plugin/show-warnings.ci` | Yes | `ls test/plugin/show-warnings*.ci` -> 3.4K |
| `test/plugin/show-errors-sent.ci` | Yes | `ls test/plugin/show-errors-sent.ci` -> 5.2K |
| `test/plugin/show-errors-received.ci` | Yes | `ls test/plugin/show-errors-received.ci` -> 5.1K |
| `test/plugin/show-errors-config-abort.ci` | Yes | `ls test/plugin/show-errors-config-abort.ci` (new) |
| `internal/component/bgp/plugins/cmd/peer/peer_warnings.go` | Absent | `ls` returns "No such file" |
| `internal/component/bgp/plugins/cmd/peer/peer_warnings_test.go` | Absent | `ls` returns "No such file" |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-6/7 | prefix-threshold raise/clear | `grep -n "RaiseWarning.*prefix-threshold\|ClearWarning.*prefix-threshold" internal/component/bgp/reactor/session_prefix.go` -> lines 84, 101 |
| AC-8 | prefix-stale scan | `grep -n "prefix-stale" internal/component/bgp/reactor/session_prefix.go` -> lines 40, 114, 123, 129 |
| AC-9/10 | notification producers | `grep -n "raiseNotificationError" internal/component/bgp/reactor/peer_stats.go` -> lines 147 (sent), 165 (received) |
| AC-19/20 | session-dropped producer | `grep -n "raiseSessionDropped" internal/component/bgp/reactor/peer_run.go` -> line 353 |
| AC-21 | commit-aborted producer | `grep -n "RaiseError.*commit-aborted\|reportCodeCommitAborted" internal/component/config/transaction/orchestrator.go` -> publishAbort at line 562 |
| AC-22 | commit-rollback producer | `grep -n "reportCodeCommitRollback" internal/component/config/transaction/orchestrator.go` -> publishRollback at line 581 |
| AC-23 | commit-save-failed producer | `grep -n "reportCodeCommitSaveFail" internal/component/config/transaction/orchestrator.go` -> writeConfigFile at line 721 |
| AC-21/22/23 unit tests | Added in final pass | `grep -n "TestCommit.*RaisesReportError" internal/component/config/transaction/orchestrator_test.go` -> 3 functions |
| AC-13 orphans | No orphans | `make ze-validate-commands` returns zero orphans for `ze-show:warnings`, `ze-show:errors`, `ze-bgp:warnings` |
| Deleted fields | Old state absent | `grep -rn "PrefixWarnings\|SetPrefixWarned\|prefixWarnedMap" internal/component/bgp/reactor/peer.go internal/component/plugin/types_bgp.go` -> no matches |
| Deleted RPC | Old registration absent | `grep -rn "ze-bgp:warnings" internal/` -> only the removal-comment in `peer_test.go` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| Stale peer data via config | `test/plugin/show-warnings.ci` | Yes, `ze-test bgp plugin show-warnings` exit 0 |
| NOTIFICATION sent via teardown command | `test/plugin/show-errors-sent.ci` | Yes, `ze-test bgp plugin show-errors-sent` exit 0 |
| NOTIFICATION received from peer | `test/plugin/show-errors-received.ci` | Yes, `ze-test bgp plugin show-errors-received` exit 0 |
| Config verify rejection via SIGHUP reload | `test/plugin/show-errors-config-abort.ci` | Yes, `ze-test bgp plugin show-errors-config-abort` exit 0 |

## Checklist

### Goal Gates
- [ ] AC-1..AC-15 all demonstrated
- [ ] Wiring Test table complete with concrete test names
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] `make ze-verify` passes
- [ ] Feature code integrated (bus + handlers + producers)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates
- [ ] RFC constraint comments added at NOTIFICATION sites
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (single source today; concrete package functions, no `Reporter` interface)
- [ ] No speculative features (`pkg/ze/report/` deferred until needed)
- [ ] Single responsibility per component (bus stores, producers raise, handlers query)
- [ ] Explicit > implicit
- [ ] Minimal coupling (`internal/core/report` imports nothing from `internal/component/*`)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for caps
- [ ] Functional tests for end-to-end behavior

### Completion
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-report-bus.md`
- [ ] Two-commit sequence: (A) code + tests + spec, (B) `git rm` spec + add learned summary
