# Spec: fixit-vpp-loopback-recreated-per-apply

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | plugin |
| Depends | - |
| Phase | 3/3 |
| Deferral shard | - |
| Updated | 2026-08-19 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Every config apply creates another VPP loopback. The old one stays in the
dataplane with its addresses and its bridge port, and nothing removes it.

`CreateDummy` (`internal/plugins/iface/vpp/ifacevpp.go`) sends
`interfaces.CreateLoopback`. That message carries one field, a MAC address:
`vendor/go.fd.io/govpp/binapi/interface/interface.ba.go` shows it takes no
instance and no name, so the dataplane allocates a fresh interface and a fresh
`SwIfIndex` on every call. The vendored binapi also holds
`CreateLoopbackInstance`, which carries `is_specified` and `user_instance`
beside the MAC. Nothing in the tree sends it.

`applyConfig` (`internal/component/iface/config_apply.go`) calls `CreateDummy`
for every `cfg.Dummy` entry on every apply. Its tolerance for a name that already
exists is written as a fallback on the ERROR branch: it calls `GetInterface` only
after the create returns an error. This create returns no error, so the tolerance
never runs, and the contract stated above that loop, that every create step
treats a name already held as success and keeps the link, is not met on the VPP
backend.

`b.names.Add` (`internal/plugins/iface/vpp/naming.go`) then rebinds the ze name to
the new index. The previous loopback keeps every address that was programmed on
it and keeps its membership in any bridge domain.

Three consequences follow, and each is reachable by a daemon reload alone.

| Consequence | Producer | What the operator sees |
|-------------|----------|------------------------|
| One leaked interface per apply | `CreateDummy` | interface count grows without bound; addresses live on interfaces no name points at |
| A bridge domain gains a dead port per apply | `BridgeAddPort` (`internal/plugins/iface/vpp/ifacevpp.go`) resolves both sides fresh and adds the new index; nothing removes the old one | L2 forwarding to a port that no longer carries traffic |
| A mirror is disabled on the wrong interface | `SetupMirror` (`internal/plugins/iface/vpp/mirror.go`) records the destination `SwIfIndex` in `b.mirrors`, and `RemoveMirror` replays that stored value | traffic keeps being copied after the operator removed the mirror |

The name map is the only resolver. `resolveIndex`
(`internal/plugins/iface/vpp/ifacevpp.go`) reads the cache seeded once by
`populateNameMap` (`internal/plugins/iface/vpp/query.go`) and mutated afterwards
by the create and delete methods and by the interface event monitor. So a stored
index is stale the moment its interface is recreated, while a name lookup is not.

Goal: creating a loopback that already exists must be a no-op that keeps the
existing interface, and no recorded `SwIfIndex` may outlive the interface it
names.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress.
     Capture what you learned as -> Decision: / -> Constraint: annotations, which
     survive compaction; track reading progress in the session state file. -->

### Architecture Docs
- [ ] `ai/digests/vpp-dataplane.md` - how `iface-vpp` acquires its channel and
  maintains the name map
  → Constraint: the name map is seeded once and mutated by lifecycle calls, so
  every fix must keep it the single source of truth
- [ ] `docs/architecture/iface/management.md` - what an apply promises about an
  interface that already exists

### RFC Summaries (Scope: protocol)
- [ ] Not applicable. No wire format changes.

**Key insights:** (minimal context to resume after compaction)
- The apply loop's idempotence is written as an error fallback, so a create that
  succeeds twice defeats it without ever failing.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/plugins/iface/vpp/ifacevpp.go` - `CreateDummy` sends
  `CreateLoopback`; `BridgeAddPort` resolves both sides fresh; `resolveIndex`
  reads the name map
- [ ] `internal/plugins/iface/vpp/naming.go` - `Add` rebinds a name to a new index
- [ ] `internal/plugins/iface/vpp/query.go` - `populateNameMap` seeds the map once
- [ ] `internal/plugins/iface/vpp/mirror.go` - `SetupMirror` records the
  destination index, `RemoveMirror` replays it
- [ ] `internal/component/iface/config_apply.go` - the dummy phase calls
  `CreateDummy` per apply and tolerates an existing name only on error
- [ ] `vendor/go.fd.io/govpp/binapi/interface/interface.ba.go` - the field sets of
  `CreateLoopback` and `CreateLoopbackInstance`

**Behavior to preserve:**
- The rollback contract: a step that created something must delete exactly that
  thing and nothing else.
- `DeleteInterface`'s current order, which tries the loopback deleter before the
  sub-interface one.
- Address programming, which is keyed by ze name, not by index.

**Behavior to change:**
- A second apply must not create a second dataplane loopback for the same ze name.
- A recorded mirror destination must survive, or be re-resolved after, a
  recreation of its interface.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A config apply carrying `interface dummy <name>` with `interface backend vpp`.
  It runs on daemon start and again on every reload.

### Transformation Path
1. `applyConfig` (`internal/component/iface/config_apply.go`) walks `cfg.Dummy`
   and calls `CreateDummy` for each entry that is not disabled.
2. `CreateDummy` (`internal/plugins/iface/vpp/ifacevpp.go`) sends
   `CreateLoopback` and receives a new `SwIfIndex`.
3. `b.names.Add` (`internal/plugins/iface/vpp/naming.go`) rebinds the ze name to
   that index, dropping the binding to the interface created by the last apply.
4. Later phases program addresses and bridge membership against the new index,
   while the old interface keeps the ones it was given.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| iface component ↔ VPP backend | `iface.Backend` method calls | Read |
| VPP backend ↔ dataplane | govpp binary API request and reply | Read |
| Backend ↔ its own name map | in-process map keyed by ze name | Read |

### Integration Points
- `iface.Backend.CreateDummy` (`internal/component/iface/backend.go`) - the
  contract both backends implement, so the netlink behavior is the reference for
  what "already exists" must mean.
- `StartMonitor` (`internal/plugins/iface/vpp/monitor.go`) - the other writer of
  the name map, and the place a recreation becomes visible.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | `CreateDummy` reads `b.names`, the same resolver `resolveIndex` reads (`internal/plugins/iface/vpp/ifacevpp.go`) |
| No unintended coupling (components stay isolated) | Yes | The backend answers with `iface.ErrInterfaceExists`, an existing sentinel shape beside `iface.ErrBackendNotReady` (`internal/component/iface/backend.go`). The apply loop learns nothing about VPP |
| No duplicated functionality (extends existing, does not recreate) | Yes | The apply loop's create-tolerance branch is extended, not replaced; the netlink EEXIST path is untouched (`internal/component/iface/config_apply.go`) |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Control plane, one map lookup per apply. No wire path |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | No new registration, no plugin name in a shared package. The sentinel names a backend CONDITION, not a backend |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `create_loopback` always allocates a new interface | the message carries only a MAC field in the vendored binapi | the leak does not exist and only the mirror half remains | `test/plugin/vpp-loopback-reapply.ci` with the fix reverted | confirmed -- the reverted run logs two creates, `sw_if_index` 1 and 2 |
| A-2 | `create_loopback_instance` with a pinned instance returns the existing interface rather than an error when that instance exists | the field names `is_specified` and `user_instance` | the fix becomes an existence check before the create | the same QEMU run | not needed -- the fix IS an existence check before the create, so no `create_loopback_instance` is sent and the assumption is never relied on |
| A-3 | Nothing outside `b.mirrors` stores a `SwIfIndex` across an apply | search of the backend for stored index fields | another stale record survives the fix | grep of the backend struct and its writers | broken -- the `b.deleters` closures capture one: `createIPIPTunnel` and `CreateWireguardDevice` close over `reply.SwIfIndex`, `createGRETunnel` over `delTun.SwIfIndex`, and the VXLAN one over `del.Instance` (`internal/plugins/iface/vpp/tunnel.go`, `wireguard.go`, `vxlan.go`). Out of this spec's scope: those creates are also unconditional, so each needs the create-side treatment first |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | An existence check races the interface event monitor, which also writes the name map | flaky test on repeated applies | take the same lock the monitor takes, and assert it in the test |
| R-2 | Reusing an interface created by an earlier daemon run adopts state the operator did not ask for | addresses present that the config does not list | the address phase already reconciles; prove it with the reapply test |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Dataplane interfaces leak on every reload, and bridge domains accumulate dead ports. A wrong fix could adopt a foreign interface of the same instance. |
| How is it reverted? | Single commit revert. No config migration. |
| Who else touches this path? | `plan/spec-vpp-loopback-mac.md` changes the same create call to pass a MAC; `plan/spec-vpp-interface-in-use.md` guards deletion of the same interfaces. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| second config apply with the same `dummy` entry | → | `CreateDummy` | `TestCreateDummyKeepsExistingLoopback` |
| bridge member on a re-applied loopback | → | `BridgeAddPort` | `vpp-loopback-reapply.ci` |
| mirror destination recreated, then mirror removed | → | `RemoveMirror` | `TestRemoveMirrorAfterDestinationRecreated` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Two applies of the same `dummy` interface | The dataplane holds one loopback for that name, and the interface count does not grow |
| AC-2 | The same, with an address configured | The address stays on the interface the name points at, and no address remains on an unnamed interface |
| AC-3 | The same, with the loopback a bridge member | The bridge domain holds exactly one port for that name. **Not reachable from config today:** `list bridge` in `internal/component/iface/yang/ze-iface-conf.yang` carries `ze:backend "netlink"`, and `validateBackendGate` (`internal/component/iface/register.go`) refuses a `bridge` block while `interface { backend vpp; }` is active, so no apply on this backend can make a loopback a bridge member. What the AC is really about is the index: `BridgeAddPort` resolves both sides through `resolveIndex` on every call, so one stable index is one port, and AC-1 is what pins the index |
| AC-4 | A mirror whose destination is recreated, then removed | The mirror is disabled on the live interface, and no request names a deleted index |
| AC-5 | An apply that finds no existing loopback | Creation is unchanged, and the rollback still deletes exactly what it created |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | reloads the daemon with an unchanged config | apply → `CreateDummy` → name map | `vpp-loopback-reapply.ci` |
| 2 | bridges a loopback and reloads | apply → `CreateDummy` → `BridgeAddPort` | `vpp-loopback-reapply.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCreateDummyKeepsExistingLoopback` | `internal/plugins/iface/vpp/apply_test.go` | the second create sends no create request and answers `iface.ErrInterfaceExists`; the name still resolves to the first index | pass; red with the fix reverted (`got <nil>, want iface.ErrInterfaceExists`) |
| `TestCreateDummyFirstCallCreates` | `internal/plugins/iface/vpp/apply_test.go` | the first create is unchanged | pass; green both ways by design, it pins AC-5 |
| `TestRemoveMirrorAfterDestinationRecreated` | `internal/plugins/iface/vpp/apply_test.go` | the disable request carries the live destination index | pass; red with the fix reverted (`got 9, want 21`) |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| loopback instance | 0-4294967295 | 4294967295 | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `vpp-loopback-reapply.ci` | `test/plugin/` | the operator reloads, the interface list is unchanged, and the address stays on the interface the name resolves to | pass, repeated; red with the fix reverted on BOTH criteria: `AC-1: create_loopback count is 2 (sw_if_index [1, 2]), want 1` and `AC-2: address programmed on sw_if_index [2], want 1` |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | - | - | no wire-visible protocol behavior changes | |

## Files to Modify
- `internal/plugins/iface/vpp/ifacevpp.go` - `CreateDummy` answers
  `iface.ErrInterfaceExists` for a name the map already holds and sends nothing;
  `mirrors` is re-typed to hold destination NAMES
- `internal/plugins/iface/vpp/mirror.go` - `recordMirror` stores the destination
  ze name and `RemoveMirror` resolves it fresh
- `internal/component/iface/backend.go` - the `ErrInterfaceExists` sentinel and
  the contract it carries
- `internal/component/iface/config_apply.go` - the dummy create-tolerance branch
  reads the sentinel, so a kept interface leaves `created` false and the undo
  does not delete it; `recreateManagedInterface` tolerates the same answer
- `internal/plugins/iface/vpp/apply_test.go` - the create_loopback reply case in
  the channel fake, with a fresh index per call
- `test/scripts/vpp_stub.py` - a `create_loopback` handler; the generic fallback
  answers 4 bytes and `CreateLoopbackReply` needs 8
- `ai/digests/vpp-dataplane.md` - the (L) lifecycle line, which said the create
  is unconditional

## Files to Create
- `test/plugin/vpp-loopback-reapply.ci` - the reload scenario

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | no config surface changes |
| YANG validation constraints | N-A | no new leaf |
| YANG custom validators | N-A | no new leaf |
| CLI commands/flags | No | no new command |
| CLI grammar (keyword before value) | N-A | no new grammar |
| Editor autocomplete | N-A | no new leaf |
| Functional test for new RPC/API | No | no new RPC |
| Pipe completeness | N-A | no new output |
| Env var registration | No | no environment leaf |
| Doctor check for runtime dependencies | No | The change adds no runtime dependency: no file, socket, module, port, cert or binary. A duplicate-loopback check would report the OLD defect's residue on a machine that ran an earlier build, which is a migration aid rather than a dependency check, and `ze doctor` has no such category |
| Prometheus counters/metrics | No | no new observable state |
| BGP family surface (new SAFI / capability / attribute) | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | Yes | `ai/digests/vpp-dataplane.md`, done: the (L) line now says the create is gated on the name map |
| 6 | Has a user guide page? | No | There is no `docs/guide/interfaces.md`. The interface reference is `docs/features/interfaces.md` (the `Design:` header of `config_apply.go` and `mirror.go`) and the operator page is `docs/guide/vpp.md`. Both already claim what the fix delivers -- the capability matrix rows "Idempotent setup/cleanup: have" and "`CreateDummy` real (CreateLoopback)" -- so the change makes a published claim true and edits nothing |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | No | |
| 10 | Test infrastructure changed? | Yes | `test/scripts/vpp_stub.py` gained a `create_loopback` handler. `docs/functional-tests.md` names suites and targets, not stub messages, and the suite list is unchanged, so nothing there moves |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `ai/digests/vpp-dataplane.md`, the (L) lifecycle line: done |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes, none stale | The anchors on `ifacevpp.go` and `mirror.go` in `docs/features/interfaces.md`, `docs/guide/vpp.md`, `docs/features.md`, `docs/architecture/core-design.md` and `ai/DOCS-TO-CODE.md` name the symbols, not the create's conditions, and every named symbol keeps its name |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes, unchanged | The `create interface dummy name <name>` examples in `docs/features/interfaces.md` describe the CLI ensure path, which was already idempotent and which this change does not touch |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** - prove the defect from the entry point
   - Tests: `TestCreateDummyKeepsExistingLoopback`, `vpp-loopback-reapply.ci`
   - Files: `internal/plugins/iface/vpp/apply_test.go`, `test/plugin/`
   - Verify: both fail on the current code, and the failure names a second
     interface rather than a rejected request
2. **Phase: Idempotent create**
   - Tests: the two create tests
   - Files: `internal/plugins/iface/vpp/ifacevpp.go`
   - Verify: the second apply keeps the first interface, and the rollback still
     deletes only what a first apply created
3. **Phase: No stored index outlives its interface**
   - Tests: `TestRemoveMirrorAfterDestinationRecreated`
   - Files: `internal/plugins/iface/vpp/mirror.go`
   - Verify: the disable request carries the live index

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file plus symbol |
| Correctness | The second apply keeps the interface, and does not delete and recreate it, which would drop traffic |
| Naming | The name map stays the only ze-name to index authority |
| Data flow | The idempotence lives in the backend, so both backends keep one contract |
| Rule: `ai/rules/evidence.md` | The dataplane behavior of both create messages is settled by a run, not by the field names |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| One loopback per name after N applies | the functional test asserts the interface count |
| No stale index in any stored record | grep the backend struct for index-typed fields and show each is re-resolved |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Resource exhaustion | The leak itself: an unbounded interface count is the defect being fixed |
| Adoption of foreign state | Reusing an existing interface must not inherit addresses the config does not list |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| `CreateDummy` answers `iface.ErrInterfaceExists` rather than a bare nil | return nil for a name already held | Returning nil sets the apply loop's `created` flag, and its undo then calls `DeleteInterface` on a loopback an EARLIER apply made, with its addresses and its LCP shadow. That inverts the rollback contract the spec preserves and AC-5 states. The sentinel keeps the create a no-op AND keeps `created` false |
| The sentinel, not `Backend.GetInterface`, settles "it already exists" on VPP | reuse the existing GetInterface fallback the netlink EEXIST path uses | `GetInterface` (`internal/plugins/iface/vpp/query.go`) matches the dump's `InterfaceName`, which VPP sets to `loopN`, while the operator's name is the ze one. So the fallback answers "not found" for a loopback that exists, and the apply would fail rather than tolerate |
| Not `create_loopback_instance` | pin `user_instance` so a re-create returns the same interface | It would send a message on every apply to learn something one map lookup already answers, and its "instance exists" behavior is unverified (A-2). The name map is the only resolver the backend has, so asking it is the smaller answer |
| `b.mirrors` holds destination NAMES | keep the index and invalidate it from the monitor | A name is stable across a recreate and an index is not, and `resolveIndex` already reads the map fresh on the delete path. Invalidation needs a second mechanism to stay correct |

## Known Limitations
- The spec covers the loopback path. Tunnels and VLAN sub-interfaces record their
  own deleters and are created from a different message, so they need their own
  reading before any claim is made about them.
  → Constraint: A-3 is now BROKEN and the shape is confirmed there. The GRE,
  IPIP, VXLAN and WireGuard creates each send unconditionally and each records a
  deleter closing over the returned `SwIfIndex`. One row in
  `plan/journal/identifier-reused-after-its-owner-is-gone.md` records it.
- The fix is per-process, because the name map is. `populateNameMap`
  (`internal/plugins/iface/vpp/query.go`) seeds the map with VPP's own names, so
  after a daemon restart the loopback VPP calls `loop0` is registered as `loop0`
  and the operator's `lo0` resolves to nothing. A restart therefore still creates
  a second dataplane loopback. Closing that needs the interface to carry the ze
  name into VPP, which is the `create_loopback_instance` question A-2 asks and
  which this spec does not answer.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-precommit-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
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

---

## Implementation Summary

### What Was Implemented
- `iface.ErrInterfaceExists` (`internal/component/iface/backend.go`): a sentinel
  that says a create step KEPT an existing interface and made nothing.
- `CreateDummy` (`internal/plugins/iface/vpp/ifacevpp.go`) answers that sentinel
  when `b.names.lookupIndex` already holds the name, and sends no
  `create_loopback`. The check runs after `ensureChannel`, so the name map is
  seeded before it is read.
- `applyConfig` (`internal/component/iface/config_apply.go`) reads the sentinel
  with `errors.Is` before its `GetInterface` fallback and leaves `created`
  false, so the undo does not delete a device an earlier apply made.
  `recreateManagedInterface` tolerates the same answer and still re-establishes
  the LCP shadow.
- `b.mirrors` holds destination ze NAMES, not `SwIfIndex` values.
  `recordMirror` stores the name and `RemoveMirror`
  (`internal/plugins/iface/vpp/mirror.go`) resolves it fresh, so no recorded
  index outlives the interface it names.
- `test/scripts/vpp_stub.py` gained `handle_create_loopback` (a fresh index per
  call, logged) and `handle_sw_interface_add_del_address` (logs the index and
  `is_add`). The generic fallback answers 4 bytes and `CreateLoopbackReply`
  needs 8.

### Bugs Found/Fixed
- The spec's own defect: one leaked VPP loopback per config apply, with its
  addresses and its bridge port left behind. Covered by
  `TestCreateDummyKeepsExistingLoopback` and `test/plugin/vpp-loopback-reapply.ci`.
- `RemoveMirror` disabled SPAN on a destination index recorded at setup time,
  which is dead after a recreate. Covered by
  `TestRemoveMirrorAfterDestinationRecreated`.
- Found by the review gate, round 1: `RemoveMirror` returned on the FIRST
  destination that failed, after `takeMirrors` had already consumed the whole
  record, so the remaining destination kept copying traffic with nothing left to
  replay. Two destinations under one source are reachable whenever the ingress
  and egress mirrors name different interfaces (`setupMirrorSpec`,
  `internal/component/iface/config_mirror.go`). Every destination is now
  attempted in sorted order and the failures are joined. Covered by
  `TestRemoveMirrorDisablesEveryDestination`.
- `_handle_callback` (`test/scripts/ze_api.py`) did not answer
  `ze-plugin-callback:post-startup`, so the copy `_call_engine` queues fell to
  the trailing else, was acked, and lost its flag. Both routes now share one
  handler.

### Documentation Updates
- `ai/digests/vpp-dataplane.md`, the (L) lifecycle line: it said the create was
  unconditional. It now states the name-map gate and the sentinel.
- No `docs/` page moved. Every `<!-- source: -->` anchor over the changed files
  names a symbol rather than a create condition, and every named symbol keeps
  its name: `docs/features/interfaces.md`, `docs/guide/vpp.md`,
  `docs/architecture/core-design.md`, `docs/features.md`.
- `make ze-doc-verify` is a stage of the full verify run; its red is judged in
  Pre-Commit Verification below.

### Deviations from Plan
- The spec's Closure checklist asks for `plan/learned/NNN-<name>.md`. The
  2026-08-10 owner directive REPLACED that artifact with a journal row, so the
  lesson is one row in
  `plan/journal/invariant-enforced-by-an-absent-call-site.md` and no learned
  summary was written.
- The review gate added one product fix the spec did not plan
  (`RemoveMirror` abandoning its remaining destinations) and its test.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-3 assumed nothing outside `b.mirrors` stored a `SwIfIndex` across an apply | the `b.deleters` closures capture one: `createIPIPTunnel`, `CreateWireguardDevice`, `createGRETunnel` and the VXLAN create each close over the index their create returned | grep of the backend struct and its writers | recorded as BROKEN in Risks & Assumptions and as one row in `plan/journal/identifier-reused-after-its-owner-is-gone.md`. Those creates send unconditionally, so each needs the create-side check before its deleter is worth touching |
| approach | The teardown loop in `RemoveMirror` kept the early return it had before the re-keying | `takeMirrors` consumes the record before the first disable, so an early return loses every destination after the failing one | review gate, round 1 | fixed in this spec: sorted iteration, joined errors, and `TestRemoveMirrorDisablesEveryDestination` proven red against the early return |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Creating a loopback that already exists is a no-op that keeps the existing interface | Done | `CreateDummy`, `internal/plugins/iface/vpp/ifacevpp.go` | answers `iface.ErrInterfaceExists`, sends nothing |
| No recorded `SwIfIndex` outlives the interface it names | Done | `recordMirror` and `RemoveMirror`, `internal/plugins/iface/vpp/mirror.go` | `b.mirrors` is keyed by name; `resolveIndex` runs at removal time |
| The rollback contract holds: a step that created something deletes exactly that | Done | `applyConfig`, `internal/component/iface/config_apply.go` | `created` stays false on the sentinel, so the undo is a no-op |
| Every stored `SwIfIndex` the Task names is re-resolved rather than replayed | Done | `recordMirror` and `RemoveMirror`, `internal/plugins/iface/vpp/mirror.go` | A-1 named the mirror record and A-3 found the same shape in the `b.deleters` closures of `tunnel.go`, `vxlan.go` and `wireguard.go`. Those are a separate problem with their own home, one row in `plan/journal/identifier-reused-after-its-owner-is-gone.md`, and their creates send unconditionally, so the create-side check has to land there before any deleter is touched |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestCreateDummyKeepsExistingLoopback`, `test/plugin/vpp-loopback-reapply.ci` | the `.ci` counts `create_loopback` in the stub log across two applies |
| AC-2 | Done | `test/plugin/vpp-loopback-reapply.ci` | the address index equals the index the single create returned |
| AC-3 | Done, by the index | `BridgeAddPort` resolves both sides through `resolveIndex` on every call | not reachable from config: `list bridge` carries `ze:backend "netlink"` and `validateBackendGate` refuses a bridge block under `backend vpp`. One stable index is one port, and AC-1 pins the index |
| AC-4 | Done | `TestRemoveMirrorAfterDestinationRecreated`, `TestRemoveMirrorDisablesEveryDestination` | the disable names the live index, and one failing destination no longer abandons the others |
| AC-5 | Done | `TestCreateDummyFirstCallCreates` | the create path is unchanged and the name binds to the returned index |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestCreateDummyKeepsExistingLoopback` | pass | `internal/plugins/iface/vpp/apply_test.go` | red with the fix reverted: `got <nil>, want iface.ErrInterfaceExists` |
| `TestCreateDummyFirstCallCreates` | pass | same | green both ways by design; it pins AC-5 |
| `TestRemoveMirrorAfterDestinationRecreated` | pass | same | red with the fix reverted: `got 9, want 21` |
| `TestRemoveMirrorDisablesEveryDestination` | pass | same | added by the review gate; red with the early return restored: `disable requests: got 0, want 1` |
| `vpp-loopback-reapply.ci` | pass | `test/plugin/` | red with the fix reverted on both criteria |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/plugins/iface/vpp/ifacevpp.go` | Done | name-map gate, `mirrors` re-typed |
| `internal/plugins/iface/vpp/mirror.go` | Done, plus the review fix | name-keyed record, fresh resolve, every destination attempted |
| `internal/component/iface/backend.go` | Done | sentinel and its contract |
| `internal/component/iface/config_apply.go` | Done | both create-tolerance sites |
| `internal/plugins/iface/vpp/apply_test.go` | Done | fake reply case with a fresh index per call, four tests |
| `test/scripts/vpp_stub.py` | Done | two handlers |
| `test/plugin/vpp-loopback-reapply.ci` | Created | the reload scenario |
| `ai/digests/vpp-dataplane.md` | Done | the (L) lifecycle line |

### Audit Summary
- **Total items:** 4 requirements, 5 AC, 5 tests, 8 files
- **Done:** 4 requirements, 5 AC, 5 tests, 8 files
- **Partial:** none
- **Skipped:** none
- **Changed:** one review-gate fix beyond the plan, recorded in Deviations

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Creating a loopback that already exists keeps the existing interface | functional | `test/plugin/vpp-loopback-reapply.ci`: `OK: one create_loopback across two applies`. With the name check removed the stub log holds two creates, `sw_if_index [1, 2]`, and the run fails `AC-1: create_loopback count is 2` |
| No interface leaks across a daemon reload | functional | the same test's count assertion, driven by SIGHUP with an address added, so a second apply provably reached phase 3 |
| An address stays on the interface the ze name points at | functional | the same test's AC-2 assertion; reverted it reports `address programmed on sw_if_index [2], want 1` |
| No recorded `SwIfIndex` outlives its interface | unit | `TestRemoveMirrorAfterDestinationRecreated`: the disable carries 21, the index the recreated destination holds now. Reverted: `got 9, want 21` |
| A mirror the operator removed stops copying traffic | unit | `TestRemoveMirrorDisablesEveryDestination`: the live destination is disabled even when a sibling destination no longer resolves. With the early return restored: `disable requests: got 0, want 1` |
| The rollback still deletes exactly what it created | unit | `TestCreateDummyFirstCallCreates` plus the `created` flag: `applyConfig` returns nil on the sentinel without setting it |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| none | n/a | The spec metadata names no deferral shard, and `plan/deferrals/fixit-vpp-loopback-recreated-per-apply.md` does not exist |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fixit-vpp-loopback-recreated-per-apply-fa011c6d-dddd-408b-a46c-3ee25189f6a1.md` |
| `review_gate.py check` | `review_gate: OK (8 code files, clean, hashes match ...)`, exit 0 |
| Rounds | 2. Round 1 read the landed diff (`c6cfb80bd`) and the uncommitted digest edit and found one ISSUE. Round 2 read the fix and found one prose defect in a comment, which is a NOTE and does not earn another round |
| Reviewer lenses used | wiring and functional coverage; logic, guard and removed-behavior; style, simplicity and documentation drift |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | `RemoveMirror` returned on the first destination that failed to resolve, after `takeMirrors` had consumed the whole record. A source carries two destinations when the ingress and egress mirrors name different interfaces, so the surviving destination kept copying traffic and no record remained to replay. AC-4 says the mirror is disabled on the live interface | `internal/plugins/iface/vpp/mirror.go`, `RemoveMirror` | every destination is attempted in sorted order, failures joined with `errors.Join`; `TestRemoveMirrorDisablesEveryDestination` proven red against the early return |
| 2 | NOTE | A comment in the fix packed the symptom into one hyphenated compound | the same function's doc comment | rewritten as two plain sentences |
| 3 | NOTE | `test/plugin/vpp-loopback-reapply.ci` is the only vpp-stub `.ci` outside `test/vpp/`. That placement is deliberate: `test/vpp/` runs only under `make ze-functional-vpp-test` (release evidence), while `test/plugin/` runs in every `ze-precommit-verify`. It needs no `needs-linux` because it supplies a working backend | `test/plugin/` | kept; recorded here so the next reader does not "correct" the location |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `test/plugin/vpp-loopback-reapply.ci` | yes | `ls -l` reports 9965 bytes, written 2026-08-19 14:27 |
| `internal/plugins/iface/vpp/mirror.go` | yes | `grep -n "resolveIndex(dst)"` returns the fresh resolve inside `RemoveMirror`'s loop |
| `plan/journal/invariant-enforced-by-an-absent-call-site.md` | yes | the closure row appended 2026-08-19, Spec cell `fixit-vpp-loopback-recreated-per-apply` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | the second apply sends no create | `grep -n ErrInterfaceExists internal/plugins/iface/vpp/ifacevpp.go` returns the wrap inside `CreateDummy`; `make _ze-unit-pkg-test-impl PKG=./internal/plugins/iface/vpp/ RUN='TestRemoveMirror\|TestCreateDummy'` printed `ok ... 1.074s`, race-instrumented |
| AC-2 | the address lands on the surviving index | `test/plugin/vpp-loopback-reapply.ci` asserts `strays` against the single create's index; the run printed `OK: one create_loopback across two applies` |
| AC-3 | one stable index is one bridge port | `BridgeAddPort` calls `resolveIndex` for both sides on every call (`internal/plugins/iface/vpp/ifacevpp.go`); no config path can make a vpp loopback a bridge member |
| AC-4 | the disable names the live index, and every destination is attempted | `TestRemoveMirrorAfterDestinationRecreated` and `TestRemoveMirrorDisablesEveryDestination`, both in the run above |
| AC-5 | the first create is unchanged | `TestCreateDummyFirstCallCreates` in the same run |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| a second config apply with the same `dummy` entry | `test/plugin/vpp-loopback-reapply.ci` | read: it starts `vpp_stub.py`, runs `ze start` with `interface { backend vpp; dummy lo0 }`, rewrites the config to add an address, sends SIGHUP, and counts `create_loopback` in the stub's JSONL log after the address request marks the second apply's phase 3 |
| a bridge member on a re-applied loopback | the same file | the index is what the AC turns on, and the `.ci` pins the index the address is programmed on |
| a mirror destination recreated, then removed | unit, `TestRemoveMirrorAfterDestinationRecreated` | no `.ci` route exists: SPAN is programmed through the backend and the stub carries no span handler. The unit test drives `SetupMirror` then `RemoveMirror` through the real backend methods against the fake channel |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | the reverted `.ci` run logs two creates, `sw_if_index` 1 and 2 |
| A-2 | not relied on | the fix is an existence check before the create, so `create_loopback_instance` is never sent |
| A-3 | broken | the `b.deleters` closures capture a `SwIfIndex` in `tunnel.go`, `vxlan.go` and `wireguard.go`. Recorded in the Mistake Log, in Known Limitations, and in `plan/journal/identifier-reused-after-its-owner-is-gone.md` |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `ai/digests/vpp-dataplane.md` (L) line: the create is gated on the name map | matches `CreateDummy` in `internal/plugins/iface/vpp/ifacevpp.go` | yes |
| No `docs/` page claims the create is unconditional | a grep of `docs/` and `ai/` for source anchors over the four changed Go files returns 11 hits, each naming a symbol; every named symbol keeps its name | yes |
| `docs/features/interfaces.md` capability rows ("Idempotent setup/cleanup", "`CreateDummy` real (CreateLoopback)") | the change makes the published claim true rather than changing it | yes, no edit needed |
| `docs/functional-tests.md` names suites and targets, not stub messages | the suite list is unchanged; `test/plugin` already exists as a suite | yes, no edit needed |

## Core Insight

A tolerance written as an error fallback is a tolerance for the backends that
FAIL. `applyConfig` promised that a create meeting an existing name keeps the
interface, and enforced it only after `CreateDummy` returned an error. The
netlink backend reached that branch through the kernel's EEXIST. The VPP
backend, whose create succeeds at making the WRONG thing, walked past it on
every apply. Where two backends share one contract, the contract has to be
stated where both answer it, which here is a sentinel the caller reads before
any fallback.
