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

`removePeer` -> `doRemovePeer` (peer leaves `r.peers`) ->
`filterapi.NotifyPeerRemoved(PeerFilterInfo{Address, Name, GroupName})` ->
each registered `Filter.PeerRemoved` -> bgp-role deletes the learned role under
the peer's name key and its address key.

### Boundaries Crossed

Reactor to filter registry (same process, function registered at `init()`).
No plugin protocol change: nothing crosses the plugin RPC boundary.

### Integration Points

`filterapi.Filter` gains `PeerRemoved func(PeerFilterInfo)`. Every path that
takes a peer out of `r.peers` calls `notifyFilterPeerRemoved`: `doRemovePeer`,
`removeDynamicPeer`, and the add-undo of a rolled-back reload.

### Architectural Verification

- Registration, no central list: the reactor walks what `filterapi.Register`
  holds and names no plugin.
- The state freed lives where the hook runs: the learned role is read only by
  the OTC filters, which run in the engine process.

## Key Design Decisions

| Decision | Chosen | Rejected, and why |
|----------|--------|-------------------|
| Carrier | a `PeerRemoved` hook on `filterapi.Filter` | Widening the state event to unattached processes: changes delivery for every plugin and the external protocol, arrives on the delivery loop and so races validate-open; `unbindProcesses` also sends `removed` for a detach of a live peer, so a role clear keyed on it would drop a live peer's role |
| Ordering | synchronous, inside the removal, before `delete bgp peer` returns | An asynchronous notice can land after a re-created peer's OPEN and delete its new role |
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

`filterapi` (new field, new function), reactor removal paths (three call
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
| AC-3 | The reactor calls every registered `PeerRemoved` hook for a peer removed by `RemovePeer`, by dynamic-peer removal, and by the undo of a reload's add, with the peer's address, name and group |
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

## Implementation Steps

1. Unit tests first, red.
2. `filterapi`: `PeerRemoved` field, `NotifyPeerRemoved`.
3. Reactor: one helper, called on the three removal paths.
4. bgp-role: handler clearing name and address keys, with a log of the drop.
5. Functional test, red with the reactor call removed, green restored.
6. Docs and the journal row.

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
