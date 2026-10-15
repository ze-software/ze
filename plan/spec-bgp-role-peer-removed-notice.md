# Spec: bgp-role-peer-removed-notice

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | plugin |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-15 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Owner decision (Thomas, 2026-10-15, "clean up (new plumbing)"): a peer created
with `create bgp peer` and removed with `delete bgp peer` leaves its learned BGP
role in `bgp-role` for the process life, and a re-created peer of the same
address reads it. The reactor announces the removal only as a peer-scoped state
event, which reaches the processes in that peer's `attach process` block, and
`bgp-role` is in none. Journal row:
`plan/journal/invariant-enforced-by-an-absent-call-site.md` (513850c68c).

Goal: every peer the reactor removes frees the per-peer state a filter plugin
keeps for it, through registration, before the removal returns.

## Required Reading

### Architecture Docs

- `docs/architecture/meta/role.md`, "Learned role lifecycle"
  -> Constraint: a session-down clear races validate-open (different loops, no
  session identity). The removal clear MUST run synchronously in the removal
  path, so a later create's OPEN can only follow it.
- `docs/architecture/api/architecture.md`, "What the Plugin Declares and What
  the Peer Grants"
  -> Constraint: a peer-scoped event reaches only attached processes; loading a
  plugin attaches nothing.
- `docs/guide/bgp-role.md`

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/bgp/reactor/reactor_peers.go` - `removePeer` stops the peer, then sends `OnPeerStateChange(down, rpc.ReasonPeerRemoved)`, which reaches `PeerScopedProcs` only
- [ ] `internal/component/bgp/plugins/role/role.go` - `filterRemoteRoles` keyed by `filterKeyLocked(peer name)`; `pruneLearnedRoles` keeps a group-less key no config names, so a created peer's role is never freed (`learnedRolePeerConfiguredLocked`)
- [ ] `internal/component/bgp/reactor/reactor_dynamic.go` - `removeDynamicPeer` removes a dynamic member with no notice
- [ ] `internal/component/bgp/reactor/reactor_api.go` - the undo of a reload's add stops and drops the peer with no notice
- [ ] `internal/component/bgp/filterapi/filterapi.go` - `Filter`, the BGP-owned registration a filter plugin makes from `init()`; the OTC filters that read the learned role run in the engine process
- [ ] `internal/component/bgp/reactor/peer_derived_bindings.go` - `unbindProcesses` also sends reason `removed`, for a detach of a live peer

**Behavior to preserve:**
- The state `down` with reason `removed` and its delivery; `pruneLearnedRoles` at config-committed; the OPEN-driven set and clear of the learned role

**Behavior to change:**
- A peer the reactor removes frees its learned role in bgp-role before the removal returns

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point

`delete bgp peer <name>` -> `handleBgpPeerDelete` -> `Reactor.RemovePeer`.

### Transformation Path

`removePeer` -> `doRemovePeer` (peer leaves `r.peers`; in the same critical
section, before the Cease runs) -> `filterapi.NotifyPeerRemoved(PeerFilterInfo{Address, Name, GroupName})` ->
each registered `Filter.PeerRemoved` -> bgp-role deletes the learned role under
the peer's name key and its address key.

### Boundaries Crossed

Reactor to filter registry (same process, function registered at `init()`).
No plugin protocol change: nothing crosses the plugin RPC boundary.

### Integration Points

`filterapi.Filter` gains `PeerRemoved func(PeerFilterInfo)`. Every path that
takes a peer out of `r.peers` calls `notifyFiltersPeerRemoved`, under `r.mu`
and before the peer's Cease is sent: `doRemovePeer`, `removeDynamicPeer`, the
reload's remove step (`reconcilePeersJournaled`: a dropped, restarted or
replaced peer, including a dynamic member replaced by a configured peer at its
address), and the add-undo of a rolled-back reload. `AddPeer`'s rollback of a
peer it never started is exempt: it validated no OPEN.
-> Decision: the hook runs before the Cease, not after. The Cease can block on a
full send buffer, and a hook run after it deletes the role of a peer created
again at the address whose OPEN arrived during the block (review of 4368fbc99c).

### Architectural Verification

- Registration, no central list: the reactor walks what `filterapi.Register`
  holds and names no plugin.
- The state freed lives where the hook runs: the learned role is read only by
  the OTC filters, which run in the engine process. True when `bgp-role` runs
  in-engine (`RunEngine`); run external (`CLIHandler`), its validate-open writes
  the role in its own process and the engine copy stays empty. That split
  predates this spec and is outside it (`docs/architecture/meta/role.md`).

## Key Design Decisions

| Decision | Chosen | Rejected, and why |
|----------|--------|-------------------|
| Carrier | a `PeerRemoved` hook on `filterapi.Filter` | Widening the state event to unattached processes: changes delivery for every plugin and the external protocol, arrives on the delivery loop and so races validate-open; `unbindProcesses` also sends `removed` for a detach of a live peer, so a role clear keyed on it would drop a live peer's role |
| Ordering | synchronous, inside the removal's critical section, before the Cease and before `delete bgp peer` returns | An asynchronous notice can land after a re-created peer's OPEN and delete its new role |
| Keys cleared | the peer's name (through `filterKeyLocked`) and its address | A name-only clear misses a role stored by address |

Main-thread decision recorded: the hook is a field of the existing filter
registration, not a new event type. No external plugin protocol changes.

## Risks & Assumptions

### Assumptions

- `delete bgp peer` stops the old session before it returns; a validate-open
  already in flight for it may still complete afterwards.

### Risks

- Residual, documented, not closed here: an OPEN validated for the old session
  after the clear writes the role back. The re-created peer's own OPEN replaces
  it, so it is read only when `bgp-role` does not answer that OPEN.

## Blast Radius

`filterapi` (new field, new function), reactor removal paths (four call
sites), bgp-role (one handler). No wire change.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `delete bgp peer <name>` | → | `Reactor.removePeer` -> `filterapi.NotifyPeerRemoved` -> bgp-role `forgetRemovedPeer` | `test/plugin/role-learned-cleared-on-peer-delete.ci` |

The reactor's removal paths read the same design as the other removal
cleanups in `docs/architecture/core-design.md`.

## Acceptance Criteria

| AC | Criterion |
|----|-----------|
| AC-1 | `delete bgp peer` of a created peer whose OPEN carried a Role removes that role from bgp-role before the command returns, and bgp-role logs the drop with the previous role |
| AC-2 | A peer re-created at the same address, whose OPEN carries no Role, is not reported as having withdrawn the previous peer's role, and its role resolves to the configured complement |
| AC-3 | The reactor calls every registered `PeerRemoved` hook for a peer removed by `RemovePeer`, by dynamic-peer removal, by a reload's remove step, and by the undo of a reload's add, with the peer's address, name and group, under `r.mu` and before the Cease is sent |
| AC-4 | A filter registered with only a `PeerRemoved` hook is accepted by `filterapi.Register` |

## End-to-End User Stories

An operator creates a peer, the peer says it is a customer, the operator
deletes it and creates it again at the same address against a router that now
sends no Role: Ze treats it as the new peer it is.

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | AC |
|------|------|----|
| `TestNotifyPeerRemovedCallsEveryHook` | `internal/component/bgp/filterapi/filterapi_test.go` | AC-3, AC-4 |
| `TestRemovePeerNotifiesFilters` | `internal/component/bgp/reactor/peer_removed_test.go` | AC-3 |
| `TestRemoveDynamicPeerNotifiesFilters` | `internal/component/bgp/reactor/peer_removed_test.go` | AC-3 |
| `TestDoRemovePeerNotifiesFiltersBeforeCease` | `internal/component/bgp/reactor/peer_removed_test.go` | AC-3 (ordering) |
| `TestReloadRemoveNotifiesFilters` | `internal/component/bgp/reactor/peer_removed_test.go` | AC-3 |
| `TestReloadReplacedDynamicPeerNotifiesFilters` | `internal/component/bgp/reactor/peer_removed_test.go` | AC-3 |
| `TestReloadAddUndoNotifiesFilters` | `internal/component/bgp/reactor/peer_removed_test.go` | AC-3 |
| `TestPeerRemovedClearsLearnedRole` | `internal/component/bgp/plugins/role/role_removed_test.go` | AC-1, AC-2 |

### Boundary Tests (numeric inputs)

None: no numeric input.

### Functional Tests

`test/plugin/role-learned-cleared-on-peer-delete.ci` with fixture
`plugin/role-learned-cleared-on-peer-delete`: the fixture creates the peer
against a test peer advertising Role customer, deletes it, then creates it
again against a test peer advertising no Role. The .ci requires the drop log
(AC-1) and rejects the "role capability withdrawn" log (AC-2).

### Interop Tests (Scope: protocol)

Not applicable: no wire-visible change. The OPEN and UPDATE bytes are unchanged;
the change frees engine-side state.

## Files to Modify

- `internal/component/bgp/filterapi/filterapi.go`
- `internal/component/bgp/reactor/reactor_peers.go`, `reactor_dynamic.go`, `reactor_api.go`
- `internal/component/bgp/plugins/role/register.go`, `role.go`
- `docs/architecture/meta/role.md`, `docs/guide/bgp-role.md`
- `plan/journal/invariant-enforced-by-an-absent-call-site.md`

## Files to Create

- `internal/component/bgp/plugins/role/role_removed_test.go`
- `internal/test/fixture/role_learned_cleared_fixture.go` and its `register_*.go`
- `test/plugin/role-learned-cleared-on-peer-delete.ci`

### Integration Checklist

| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No config or RPC added |
| YANG validation constraints | N-A | No leaf added |
| YANG custom validators | N-A | No leaf added |
| CLI commands/flags | N-A | `delete bgp peer` unchanged in grammar and output |
| CLI grammar (keyword before value) | N-A | No command added |
| Editor autocomplete | N-A | No leaf or command added |
| Functional test for new RPC/API | Yes | `test/plugin/role-learned-cleared-on-peer-delete.ci` drives `delete bgp peer` end to end |
| Pipe completeness | N-A | No output added |
| Env var registration | N-A | No env var |
| Doctor check for runtime dependencies | N-A | No file, socket, port, module or binary dependency added |
| Prometheus counters/metrics | N-A | No metric added; the drop is an Info log line |
| BGP family surface (new SAFI / capability / attribute) | N-A | No family, capability or attribute added |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | No new feature: a removed peer's state is freed. `docs/features/plugins.md` and `docs/features/bgp-protocol.md` list bgp-role only by name and RFC |
| 2 | Config syntax changed? | No | No YANG or parser change in 4368fbc99c or 6a89d04969 |
| 3 | CLI command added/changed? | No | `delete bgp peer` keeps its grammar and output; only its side effect on bgp-role changed, documented in row 6 |
| 4 | API/RPC added/changed? | No | No RPC added; the hook is an in-process `filterapi` field |
| 5 | Plugin added/changed? | No | `docs/guide/plugins.md` lists bgp-role with no binding; the new hook is engine-internal and not an `attach process` binding |
| 6 | Has a user guide page? | Yes | `docs/guide/bgp-role.md`: deleting a created peer now removes its learned role before `delete bgp peer` returns (4368fbc99c) |
| 7 | Wire format changed? | No | OPEN and UPDATE bytes unchanged |
| 8 | Plugin SDK/protocol changed? | No | `filterapi` is `internal/component/bgp/filterapi`, not `pkg/plugin`; no process-protocol message added |
| 9 | RFC behavior implemented, changed, or newly proven? | No | RFC 9234 Role negotiation unchanged; the change frees engine-side state after the session ends |
| 10 | Test infrastructure changed? | No | New fixture `role_learned_cleared_fixture.go` registers through the existing fixture registry; no harness change |
| 11 | Affects daemon comparison? | No | No capability gained or lost |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md`: filters keeping per-peer state register `PeerRemoved`, called synchronously on every removal path (4368fbc99c) |
| 13 | Route metadata keys added/changed? | Yes | `docs/architecture/meta/role.md`: learned-role lifetime and the `PeerRemoved` clear by name and address, before the Cease (4368fbc99c, 6a89d04969) |
| 14 | Prometheus counters added/changed? | No | No metric added |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | bgp-role registration gains a filter hook only; no event, send type, command or capability added |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes, all unaffected | `./le spec citation anchors` names 8: `docs/DESIGN.md`, `docs/features/plugins.md`, `docs/features/bgp-protocol.md`, `docs/guide/plugins.md` (anchor bgp-role registration: name and RFC only, still true); `docs/architecture/api/process-protocol.md` (anchor `AddPeer`, unchanged); `docs/guide/operational-reports.md` (anchor prefix-stale calls at AddPeer/RemovePeer, unchanged); `docs/architecture/meta/filter-community.md` (anchor `LessOrder` and stage constants, unchanged); `docs/features/configuration.md` (anchor `FilterStageProtocol`, unchanged). grep of all 8 for `filterapi\|PeerRemoved\|delete bgp peer\|learned role` finds only those anchors |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/bgp-role.md` runtime-peer paragraph rewritten to the new behavior; no config example changed |

## Implementation Steps

1. Unit tests first, red.
2. `filterapi`: `PeerRemoved` field, `NotifyPeerRemoved`.
3. Reactor: one helper, called on the four removal paths, under `r.mu` before the Cease.
4. bgp-role: handler clearing name and address keys, with a log of the drop.
5. Functional test, red with the reactor call removed, green restored.
6. Docs and the journal row.

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| `filterapi.Filter.PeerRemoved` field and `filterapi.NotifyPeerRemoved` | `go test -race ./internal/component/bgp/filterapi -run TestNotifyPeerRemovedCallsEveryHook` |
| Reactor notifies on `RemovePeer`, before the Cease | `go test ./internal/component/bgp/reactor -run 'TestRemovePeerNotifiesFilters\|TestDoRemovePeerNotifiesFiltersBeforeCease'` |
| Reactor notifies on dynamic-peer removal | `go test ./internal/component/bgp/reactor -run TestRemoveDynamicPeerNotifiesFilters` |
| Reactor notifies on a reload's remove step and a replaced dynamic peer | `go test ./internal/component/bgp/reactor -run 'TestReloadRemoveNotifiesFilters\|TestReloadReplacedDynamicPeerNotifiesFilters'` |
| Reactor notifies on the undo of a reload's add | `go test ./internal/component/bgp/reactor -run TestReloadAddUndoNotifiesFilters` |
| bgp-role clears the learned role by name and by address | `go test -race ./internal/component/bgp/plugins/role -run 'TestPeerRemovedClearsLearnedRole\|TestPeerRemovedClearsRoleKeyedByAddress'` |
| End-to-end: `delete bgp peer` drops the role, the re-created peer sees no withdrawal | `test/plugin/role-learned-cleared-on-peer-delete.ci` via `./le test bgp plugin --pattern role-learned-cleared` |
| Docs updated | `git show --stat 4368fbc99c 6a89d04969 -- docs/` lists `role.md`, `bgp-role.md`, `core-design.md` |
| Journal row updated | `git show --stat 4368fbc99c -- plan/journal/invariant-enforced-by-an-absent-call-site.md` |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | No new untrusted input: the hook receives the address, name and group of a peer the reactor already holds, never bytes from the wire or the CLI |
| Resource exhaustion | The change removes an unbounded growth: before it, every created-then-deleted peer left a learned-role entry for the process life |
| Stale authorization | A re-created peer at the same address no longer inherits the previous peer's learned role, so OTC and role checks cannot apply a role the new router never sent (AC-2) |
| Lock order and deadlock | Hooks run under `r.mu`; the order is `r.mu` -> `filterapi.mu.RLock` -> bgp-role `filterMu`, and nothing takes them in reverse (reviewed in the Review Gate). A hook must not block or call back into the reactor |
| Error leakage | The drop log carries the peer address and the previous role, the same fields the existing "role capability withdrawn" Info line (`role.go`) already logs; no secret or config value |

## Checklist

### Goal Gates (MUST pass)

- [ ] AC-1..AC-4 each proven by the test named above
- [ ] Functional test red recorded with the reactor call reverted
- [ ] `./le verify worktree` passes (run by the main thread)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

## Review Gate

Independent `/ze-review` over 4368fbc99c and 6a89d04969 (reviewer authored none of it), 2026-10-15.

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/bgp-role-peer-removed-notice-3be8cd06-0062-4950-8b30-09ebec04cbaf.md` |
| `./le spec review check` | clean (12 code files, hashes match) |
| Rounds | 2 (round 2 at closure: 98a5ddd46b, from spec-plugin-reload-applies-or-refuses, added comment-only lines to `role.go` naming `pruneLearnedRoles` and `forgetRemovedPeer` as each other's sibling path, and closure repointed the `role_removed_test.go` spec citation; both read against `config_tx.go` (prune at commit, in the plugin's process) and `register.go` (`PeerRemoved: forgetRemovedPeer`), accurate, 0 BLOCKER, 0 ISSUE; artifact re-recorded, check OK, 12 files) |
| Reviewer lenses used | wiring + functional test, logic + lock order (r.mu -> filterapi.mu.RLock -> role filterMu; nothing takes them in reverse), removed-behavior, docs drift, style pass (no panic, no new loop/queue, names carry the concept) |

Result: 0 BLOCKER, 0 ISSUE. Scoped evidence: `go test -race` filterapi and role ok; reactor `-run 'NotifiesFilters|PeerRemoved|BeforeCease'` ok. `./le commit audit` and `./le repo check` findings are all in other sessions' uncommitted files (rfc8669_error_log_test.go, rfc7606.go, slogutil.go), none in this diff.

Functional test, 2026-10-15, working tree at 22dd96cfcd plus foreign uncommitted hunks (private `./le --name` build, since `bin/le` predated 4368fbc99c):
`./le test bgp plugin --pattern role-learned-cleared` -> `5.4s 1/1 PASS 742 role-learned-cleared-on-peer-delete`, `pass 1/1 100.0%`.

Scoped lint, 2026-10-15: `./le go lint run scope "./internal/component/bgp/reactor ./internal/component/bgp/filterapi ./internal/component/bgp/plugins/role"` reports 4 issues, none in this spec's files: goconst on the `"peer"` literal in `reactor.go` and `session_handlers.go`, gocritic appendCombine in `draft_ietf_bess_mup_safi_t1st_withdraw_test.go`, godot in `session_egress_normalize.go`. filterapi, role, `reactor_peers.go`, `reactor_api.go`, `reactor_dynamic.go` and `peer_removed_test.go` are clean.

NOTEs:
1. The removal hook logs at Info under `r.mu` (`forgetLearnedRoleLocked`, role.go). Cold path, one line per removed peer that held a role; acceptable.
2. The address-key clear (`forgetRemovedPeer`) would drop a role stored for a second peer sharing the same IP (different port). The OTC filters already key the learned role by address (`remoteRoleLocked`), so that conflation predates this spec.

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| - | - | none | - | - |

---

## Implementation Summary

### What Was Implemented
- `filterapi.Filter.PeerRemoved` and `filterapi.NotifyPeerRemoved` (`internal/component/bgp/filterapi/filterapi.go`); a filter registered with only the hook is accepted.
- Reactor helper `notifyFiltersPeerRemoved` (`reactor_peers.go`) called under `r.mu`, before the Cease, from `doRemovePeer` (`reactor_peers.go`), `removeDynamicPeer` (`reactor_dynamic.go`), and in `reconcilePeersJournaled` (`reactor_api.go`) from both the reload's remove step, including a replaced dynamic member, and the add-undo of a rolled-back reload.
- bgp-role `forgetRemovedPeer` (`role.go`), registered as `PeerRemoved` in `register.go`: deletes the learned role under the peer's name key and its address key and logs `learned role dropped: peer removed` with the previous role.
- Fixture `role_learned_cleared_fixture.go` + `register_role_learned_cleared.go`, functional test `test/plugin/role-learned-cleared-on-peer-delete.ci`.
- Commits: 4368fbc99c, 6a89d04969 (review fixes: reload remove step, hook before the Cease), 98a5ddd46b (comment-only, another spec's review NOTE).

### Bugs Found/Fixed
- Reload remove step deleted a peer from `r.peers` without the notice (a dynamic member replaced by a configured peer kept its role): `TestReloadRemoveNotifiesFilters`, `TestReloadReplacedDynamicPeerNotifiesFilters` (6a89d04969).
- Hook ran after the Cease, which can block, so a re-created peer's fresh role could be deleted late: `TestDoRemovePeerNotifiesFiltersBeforeCease` (6a89d04969).

### Documentation Updates
- `docs/architecture/meta/role.md` "Learned role lifecycle", `docs/guide/bgp-role.md` runtime-peer paragraph, `docs/architecture/core-design.md` removal cleanups (4368fbc99c, 6a89d04969, 98a5ddd46b).
- Doc stages of the full verify over 4bd6dd97a2 hold no role page among their reds (see Pre-Commit Verification).

### Deviations from Plan
- Ordering moved from "after stop" to "inside the removal's critical section, before the Cease" (review of 4368fbc99c); the reload remove step was added as a fourth call path. Both recorded in Data Flow.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | First cut called the hook after `stop.run()` and missed the reload's remove step | The Cease can block; the reload remove step also takes a peer out of `r.peers` | Review of 4368fbc99c | Fixed in 6a89d04969, tests above |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Every reactor peer removal frees a filter's per-peer state through registration, before the removal returns | Done | `reactor_peers.go` `notifyFiltersPeerRemoved`; four call paths listed above | AddPeer's rollback of a never-started peer exempt (validated no OPEN) |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestPeerRemovedClearsLearnedRole`; `.ci` expects `learned role dropped: peer removed.*previous-role=customer` | |
| AC-2 | Done | `.ci` rejects `role capability withdrawn` and expects the re-created peer established with no Role | |
| AC-3 | Done | the six `*NotifiesFilters*` tests in `peer_removed_test.go` | ordering: `TestDoRemovePeerNotifiesFiltersBeforeCease` |
| AC-4 | Done | `TestNotifyPeerRemovedCallsEveryHook` | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestNotifyPeerRemovedCallsEveryHook` | Done | `filterapi_test.go` | |
| `TestRemovePeerNotifiesFilters`, `TestRemoveDynamicPeerNotifiesFilters`, `TestDoRemovePeerNotifiesFiltersBeforeCease`, `TestReloadRemoveNotifiesFilters`, `TestReloadReplacedDynamicPeerNotifiesFilters`, `TestReloadAddUndoNotifiesFilters` | Done | `peer_removed_test.go` | red with calls reverted (6a89d04969 body) |
| `TestPeerRemovedClearsLearnedRole` (+ `TestPeerRemovedClearsRoleKeyedByAddress`) | Done | `role_removed_test.go` | |
| `role-learned-cleared-on-peer-delete.ci` | Done | `test/plugin/` | red with reactor call reverted (4368fbc99c body) |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `filterapi.go`, `reactor_peers.go`, `reactor_dynamic.go`, `reactor_api.go`, `role/register.go`, `role/role.go` | Done | 4368fbc99c, 6a89d04969 |
| `role.md`, `bgp-role.md`, journal row | Done | 4368fbc99c |
| `role_removed_test.go`, fixture + register, `.ci` | Done | 4368fbc99c |

### Audit Summary
- **Total items:** 13
- **Done:** 13
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (ordering, in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Every peer the reactor removes frees a filter plugin's per-peer state, through registration, before the removal returns | functional + unit | `test/plugin/role-learned-cleared-on-peer-delete.ci` drives `create bgp peer`/`delete bgp peer` end to end: plugin test 742 passes in the full verify over 4bd6dd97a2 and in the scoped run (`5.4s 1/1 PASS`); red with the reactor call reverted (4368fbc99c). Unit: six reactor tests cover every removal path, red with calls reverted (6a89d04969) |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| none | every AC done; the residual (an OPEN validated for the old session after the clear) and the external-mode split are pre-existing, documented in `role.md`, outside the Task | - |

## Pre-Commit Verification

Full `./le verify worktree` over 4bd6dd97a2 (holds all this spec's code), 2026-10-15, exit 1. Logs: `tmp/session/2026-10-14-3be8cd06-0062-4950-8b30-09ebec04cbaf/scratch/verify6.log`, stages `tmp/verify-worktree-logs/20261015T104346.103322000Z-p23708-r1-4bd6dd97a284/`. Role evidence green: filterapi, plugins/role and reactor pass in the unit, race and ze_core runs; plugin test 742 role-learned-cleared-on-peer-delete passes; no role file is in any red stage. Reds, all other sessions' work (`ai/rules/pre-release.md`): lint on 12 foreign BGP files (none of this spec's), arch tier, rfc2545 records, retired-commands, test health rfc5882, site facts, bfd `.sh` prestart scripts from 08dd26d6b5, race-stage go-cache eviction, plugin 724 (lo0 127.0.0.6 missing), plugin 808 load flake (passed alone), ui 210/212/221/223/224, appliance qemu tests, runner 12, exabgp 4.

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/bgp/plugins/role/role_removed_test.go` | yes | ls 2026-10-15: 3.0K |
| `internal/test/fixture/role_learned_cleared_fixture.go`, `register_role_learned_cleared.go` | yes | ls: 4.9K, 307 |
| `test/plugin/role-learned-cleared-on-peer-delete.ci` | yes | ls: 1.7K |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1, AC-2 | role cleared; re-created peer sees no withdrawal | closure run `go test -count=1 ./internal/component/bgp/plugins/role/ ./internal/component/bgp/filterapi/`: both `ok`; `.ci` expect/reject lines; plugin 742 pass in full verify |
| AC-3 | four removal paths call the hook | grep `notifyFiltersPeerRemoved`: in `doRemovePeer`, `removeDynamicPeer`, and twice in `reconcilePeersJournaled`; reactor package passes in full verify |
| AC-4 | hook-only filter accepted | `TestNotifyPeerRemovedCallsEveryHook` ok (closure run) |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `delete bgp peer <name>` | `test/plugin/role-learned-cleared-on-peer-delete.ci` | read: fixture creates, deletes, re-creates; expects drop log, rejects withdrawal log; passes as plugin 742 |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 (`delete bgp peer` stops the old session before return; an in-flight validate-open may still complete) | confirmed | `doRemovePeer` notifies under `r.mu` before the Cease; residual documented in `role.md` "Learned role lifecycle" |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `role.md` lifecycle: hook before the Cease, name and address keys, prune covers external mode | `reactor_peers.go` `doRemovePeer`, `role.go` `forgetRemovedPeer`, `config_tx.go` `pruneLearnedRoles` calls | yes |
| `bgp-role.md` runtime-peer paragraph | `register.go` `PeerRemoved: forgetRemovedPeer` | yes |
| Citers of this spec | `git grep spec-bgp-role-peer-removed-notice`: `role_removed_test.go` repointed to the bare stem + role.md; journal row already uses the bare stem | yes |
