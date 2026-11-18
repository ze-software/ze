# Spec: fleet-6 -- Config Freeze, Emergency Disable, and Reconnect Safety

| Field | Value |
|-------|-------|
| Status | skeleton |
| Priority | P0 |
| Release reason | Fleet reconnect must not silently overwrite emergency device configuration. |
| Depends | - |
| Phase | - |
| Updated | 2026-11-18 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `docs/architecture/fleet-config.md` -- managed config architecture (transport, auth, managed flag)
4. `internal/component/managed/client.go` -- managed client lifecycle, `fetchAndProcess`, `CheckManaged`
5. `cmd/ze/ze_core_start.go` -- `isManaged`, `ClientConfig.Version` initialization
6. `internal/component/cli/editor_commit.go` -- operator commit chokepoint
7. `cmd/ze/hub/managed.go` -- hub-apply `OnCommit` path (must stay open)
8. `spec-fleet-0-umbrella.md` -- umbrella design decisions and direction update
9. `spec-fleet-7-config-reconnect-resolution.md` -- sibling: the resolution UX built on this spec's baseline + diverged state

## Task

Make a managed device the single writer of its own config while it is part of a fleet, give the
operator a safe emergency way out, and make rejoining the fleet **safe** (never silently
overwrite an emergency change).

Three pieces:

1. **Freeze.** When `meta/instance/managed` is true, operator-initiated local config commits are
   frozen: the editor (CLI, SSH, web) refuses to commit, with an error that names the command to
   leave the fleet. The hub remains authoritative and keeps pushing config down normally; the
   freeze applies only to *operator* edits on the device, never to the hub-apply path. The mirror
   constraint is enforced on the hub: the hub's per-device config editor refuses edits to a device
   whose managed session is not currently connected.

2. **Emergency disable.** `ze fleet disable` sets the device non-managed, **severs the hub
   connection immediately** (today a disable only takes effect on the next reconnect or after the
   ~90 s heartbeat timeout), and unfreezes local edits. `ze fleet enable` rejoins; `ze fleet
   status` reports the current state including divergence.

3. **Reconnect safety (the load-bearing fix).** The device persists a **baseline**: the hash of
   the last config it received from the hub. On (re)connect, before applying anything, it checks
   whether its active config differs from that baseline (it was locally edited while out of the
   fleet). If it differs, the device **holds** -- it keeps running its current config and does NOT
   auto-apply the hub config -- and marks itself `diverged`. If it does not differ, the normal
   fetch/apply path runs unchanged. Without this, `ze fleet enable` would hit the existing
   unconditional-apply path (`client.go`) and silently overwrite the emergency edit.

Resolution of a `diverged` device (the commit-style diff, adopt/revert, the `config-push`
up-verb) is `spec-fleet-7`. Until fleet-7 lands, a diverged device holds safely and the operator
resolves out of band (stay disabled, or reconcile the config to the hub and re-enable).

Transport (TLS) and authentication (pre-declared per-client shared secret) are **unchanged**.

### Decisions inherited from the umbrella direction update

| Decision | Detail |
|----------|--------|
| Keep TLS | No SSH tunnel. Freeze, disable, and the baseline add nothing to the transport |
| Keep shared secret | Pre-declared per-client secret auth unchanged; no enrollment in this spec |
| One flag, two effects | `meta/instance/managed` drives both fleet membership and the local-edit freeze |
| Single-writer | Device-side freeze + hub-side connected-only freeze; editor-level (soft) -- see Risks |
| Persisted baseline | `meta/instance/managed/base-version` distinguishes a local edit from a pending hub update; required because `cfg.Version` is recomputed from the live config at startup |
| Hub-apply is not an operator edit | The `OnCommit` path is exempt from the freeze guard by construction (different code path) |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/fleet-config.md` -- managed config: managed flag, connection lifecycle, two-phase change
  -> Decision: same `meta/instance/managed` flag gates the freeze; baseline persisted alongside it
  -> Constraint: preserve TLS transport, shared-secret auth, and the hub-apply path
- [ ] `docs/architecture/core-design.md` -- component lifecycle, registration pattern
  -> Constraint: `ze fleet` commands register through the normal CLI command path

**Key insights:**
- `cfg.Version` is initialized at startup to `fleet.VersionHash(currentCachedConfig)` (`ze_core_start.go`), so it equals the live config and cannot serve as a baseline. A separate persisted baseline is required.
- `fetchAndProcess` (`client.go`) applies hub config unconditionally when versions differ; the hold check must gate this.
- The managed client runs as `go managed.RunManagedClient(managedCtx, ...)`; cancelling its context is the clean way to sever immediately (`notificationLoop` selects on `ctx.Done()` at `client.go`).
- `ClientConfig.OnCommit` (hub-apply) and the editor commit functions are different code paths; the freeze guard on the editor functions cannot affect hub-apply.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze/ze_core_start.go` -- `isManaged()` reads `meta/instance/managed`; `ClientConfig.Version` set to `fleet.VersionHash(data)` of the cached config at line 325
  -> Constraint: the version token equals the live config at startup; it is NOT a record of the last hub-applied config
- [ ] `internal/component/managed/client.go` -- `RunManagedClient`; `fetchAndProcess` sends `cfg.Version`, applies hub config via `OnCommit` unconditionally on difference; `CheckManaged` consulted only before each reconnect attempt
  -> Decision: add a persisted baseline + a dirty check that holds before `OnCommit`; hold the client `cancel` for immediate sever
- [ ] `internal/component/cli/editor_commit.go` -- `CommitSession` (writes `config.conf` at line 161) and `CommitSessionCandidate` (writes candidate at line 319)
  -> Decision: freeze guard goes here; covers CLI, SSH, and web (web delegates to these). NOTE: the editor already has its own `Conflict`/`ConflictStale`/`ConflictLive` concepts for concurrent edit sessions -- do not reuse "conflict" for the fleet state; use `diverged`
- [ ] `internal/component/cli/editor_commands.go` -- `Rollback` (~:1092), `Save` (~:936) also write config
  -> Decision: guard these too so no operator write path is missed; audit for any other writer of `e.originalPath` / `WriteCandidateVersion`
- [ ] `cmd/ze/hub/managed.go` -- `wireManagedCommit` sets `OnCommit` (writes via `WriteCandidateVersion`, then `reload`)
  -> Constraint: this hub-apply path must NOT be guarded; it is how the mothership pushes config
- [ ] `internal/component/web/editor.go` -- `EditorManager.Commit` delegates to `CommitSession`/`CommitSessionCandidate`
  -> Constraint: covered transitively; verify no independent web write path bypasses the guard

**Behavior to preserve:**
- TLS transport and shared-secret auth unchanged
- Hub-apply path (`OnCommit` -> `WriteCandidateVersion` -> reload) unchanged and unguarded
- Standalone (`managed=false`) devices: editor works exactly as today, no freeze
- `config-fetch`/`config-changed`/`config-ack`/`ping` protocol unchanged
- Normal (non-diverged) reconnect: fetch latest, apply, resume -- unchanged

**Behavior to change:**
- Operator config commits frozen when `managed=true` (guard in `CommitSession`, `CommitSessionCandidate`, `Rollback`, `Save`)
- New `ze fleet disable` / `enable` / `status` commands; `disable` cancels the managed client context for an immediate sever
- Persisted baseline `meta/instance/managed/base-version`, written on every successful hub-apply
- On (re)connect, if the active config differs from the baseline, the device holds (does not auto-apply) and marks itself `diverged`
- Hub-side per-device editor refuses edits to a device that is not currently connected

## Data Flow (MANDATORY)

### Entry Point
- Operator runs a config commit on a managed device (CLI/SSH/web editor)
- Operator runs `ze fleet disable` / `enable` / `status`
- Managed device (re)connects to the hub
- Operator (on the hub) edits a managed device's config

### Transformation Path
1. Editor commit calls `CommitSession`/`CommitSessionCandidate`; guard checks `frozen()` (managed). If frozen: error naming `ze fleet disable`; no write
2. `ze fleet disable`: set `managed=false`, cancel managed client context (sever now), unfreeze
3. Hub-apply (`OnCommit`) writes baseline `base-version := VersionHash(applied config)`
4. On (re)connect: compute `dirty = VersionHash(active config) != base-version`
5. If `dirty`: mark `diverged`, hold (skip `OnCommit`), report state; resolution deferred to fleet-7
6. If not `dirty`: normal `fetchAndProcess` applies any hub update; baseline updated on apply
7. Hub-side editor: before writing a device's config, check that device's session is connected; refuse if not

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Editor commit to freeze guard | `frozen()` predicate read from blob meta | [ ] |
| Disable command to managed client | cancel func cancels `managedCtx` | [ ] |
| Apply path to baseline | `OnCommit` writes `base-version` | [ ] |
| Reconnect to dirty check | active hash vs baseline before apply | [ ] |
| Hub editor to session state | connected-check before write | [ ] |

### Integration Points
- `internal/component/cli/editor_commit.go` -- freeze guard injected as a `frozen func() bool`
- `internal/component/managed/client.go` -- baseline read/dirty check before apply; expose `cancel`
- `cmd/ze/hub/managed.go` -- `OnCommit` writes `base-version`
- `cmd/ze/ze_core_start.go` -- initialize `cfg.Version` from baseline; share the managed-flag predicate
- Hub-side config editor (`internal/component/plugin/server/`) -- connected-only guard
- CLI command tree -- `ze fleet disable/enable/status`

### Architectural Verification
- [ ] No bypassed layers (freeze at the editor chokepoint; hold before `OnCommit`)
- [ ] No unintended coupling (hub-apply path untouched; standalone devices unaffected)
- [ ] No duplicated functionality (reuses `meta/instance/managed`, the editor path, the version hash)
- [ ] Zero-copy preserved where applicable

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | `CommitSession`/`CommitSessionCandidate` are the only operator write paths for the active config (web/SSH delegate to them) | `editor_commit.go`, agent survey | A bypass path stomps the freeze | grep all callers of `WriteFile(e.originalPath)` and `WriteCandidateVersion` | unvalidated |
| A-2 | `OnCommit` is the single point where hub config is applied, so baseline writes there cover all hub-applies | `cmd/ze/hub/managed.go`, `client.go` | Baseline goes stale; false divergence | trace all `OnCommit` invocations | unvalidated |
| A-3 | Cancelling `managedCtx` cleanly drops a live connection | `client.go` selects on `ctx.Done()` | `disable` is not immediate | `TestManagedImmediateSever` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Single-writer is editor-level only; raw `ze data write` to the blob bypasses both guards (device and hub side) | A diverged device whose hub config also changed offline | Document the soft enforcement; the persisted baseline still attributes divergence safely (hold, never stomp). A future spec may guard the blob write path |
| R-2 | `ze fleet enable` runtime re-enable may not restart the client goroutine (started once at `main.go`) | `enable` has no effect until restart | Wire a restart, or have `status`/docs state enable takes effect on next `ze start` |
| R-3 | A diverged device cannot be resolved in-band until fleet-7 | Operator stuck with a held device | Document the out-of-band path (stay disabled, or reconcile to hub then enable); ship fleet-7 |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Operator commit on managed device | -> | Freeze guard rejects with disable hint | `test/managed/fleet-freeze.ci` |
| Hub pushes `config-changed` to managed device | -> | `OnCommit` applies; baseline updated | `test/managed/fleet-freeze.ci` |
| `ze fleet disable` on a live connection | -> | Managed context cancelled, severed, unfrozen | `test/managed/fleet-disable.ci` |
| Re-enable after an emergency edit | -> | Dirty check holds; device marked `diverged`; no stomp | `test/managed/fleet-reconnect-hold.ci` |
| Hub editor edits a disconnected device | -> | Hub-side connected-only guard rejects | `test/managed/fleet-hub-singlewriter.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Operator runs config commit on a `managed=true` device | Commit rejected; error names `ze fleet disable` |
| AC-2 | Hub sends `config-changed`; device applies via `OnCommit` | Applies normally; baseline `base-version` set to the applied hash; freeze does not block it |
| AC-3 | `ze fleet disable` on a managed device | `managed=false`; hub connection severed immediately; local edits allowed |
| AC-4 | `ze fleet enable` with the active config unchanged from baseline | `managed=true`; reconnect; normal fetch/apply; no divergence |
| AC-5 | `ze fleet enable` after an emergency local edit (active != baseline) | Device holds: keeps running its config, does NOT auto-apply hub config, marks itself `diverged` |
| AC-6 | `ze fleet status` | Reports managed on/off, hub address, connection state, frozen state, and `diverged` if applicable |
| AC-7 | Standalone device (`managed=false`) commit | Editor commits normally; no freeze |
| AC-8 | Hub operator edits config for a **connected** managed device | Allowed (existing hub-authoritative push) |
| AC-9 | Hub operator edits config for a **disconnected** managed device | Rejected (editor-level single-writer; see R-1 for the blob backdoor) |
| AC-10 | `ze fleet disable` while the hub connection is healthy | Connection drops within a bounded time, not after the ~90 s heartbeat timeout |
| AC-11 | SSH or web editor commit on a managed device | Rejected (covered transitively via `CommitSession`/`CommitSessionCandidate`) |
| AC-12 | Normal hub update while connected (no local edit), device deferred fetch then reconnects | Device is not dirty (active == baseline); applies the pending hub update; not flagged `diverged` |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Tries to edit a router that is in a fleet | editor commit -> `frozen()` true -> rejected with hint | `fleet-freeze.ci` |
| 2 | Runs `ze fleet disable`, makes an emergency change | disable -> sever + unfreeze -> editor commit allowed | `fleet-disable.ci` |
| 3 | Runs `ze fleet enable` after that change | reconnect -> dirty check -> hold + `diverged`, edit preserved | `fleet-reconnect-hold.ci` |
| 4 | Runs `ze fleet enable` after no local change | reconnect -> not dirty -> normal apply, in sync | `fleet-disable.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestEditorFreezeWhenManaged` | `internal/component/cli/editor_commit_test.go` | Commit rejected when `frozen()` true | |
| `TestEditorNotFrozenStandalone` | `internal/component/cli/editor_commit_test.go` | Commit allowed when `frozen()` false | |
| `TestRollbackSaveFrozen` | `internal/component/cli/editor_commands_test.go` | Rollback and Save also blocked when frozen | |
| `TestManagedImmediateSever` | `internal/component/managed/client_test.go` | Cancelling context drops a live connection | |
| `TestBaselineWrittenOnApply` | `internal/component/managed/client_test.go` | `OnCommit` updates `base-version` | |
| `TestReconnectDirtyHolds` | `internal/component/managed/client_test.go` | active != baseline -> hold, mark diverged, no apply | |
| `TestReconnectCleanApplies` | `internal/component/managed/client_test.go` | active == baseline -> normal apply | |
| `TestHubEditorConnectedOnly` | `internal/component/plugin/server/managed_test.go` | Hub edit rejected for a disconnected device | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `fleet-freeze` | `test/managed/fleet-freeze.ci` | Managed device refuses local commit; hub push still applies | |
| `fleet-disable` | `test/managed/fleet-disable.ci` | `disable` severs now, unfreezes; `enable` (clean) rejoins | |
| `fleet-reconnect-hold` | `test/managed/fleet-reconnect-hold.ci` | `enable` after an edit holds, marks diverged, does not stomp | |
| `fleet-hub-singlewriter` | `test/managed/fleet-hub-singlewriter.ci` | Hub refuses to edit a disconnected device | |

## Files to Modify
- `internal/component/cli/editor_commit.go` -- freeze guard in `CommitSession`/`CommitSessionCandidate`
- `internal/component/cli/editor_commands.go` -- freeze guard in `Rollback`/`Save`
- `internal/component/cli/session_factory.go` -- inject the `frozen func() bool` predicate
- `internal/component/managed/client.go` -- baseline read + dirty check before apply; expose `cancel`
- `cmd/ze/hub/managed.go` -- `OnCommit` writes `base-version`
- `cmd/ze/hub/main.go` -- hold the managed cancel func; register `ze fleet` commands
- `cmd/ze/ze_core_start.go` -- initialize `cfg.Version` from baseline; share the managed-flag predicate
- `internal/component/plugin/server/managed.go` -- hub-side connected-only guard for the per-device editor
- `pkg/zefs/keys.go` -- register `meta/instance/managed/base-version` key

## Files to Create
- `internal/plugins/fleet-cmd/` (or co-located with managed; final home is a design decision) -- `ze fleet disable/enable/status` command provider
- `test/managed/fleet-freeze.ci` -- functional test
- `test/managed/fleet-disable.ci` -- functional test
- `test/managed/fleet-reconnect-hold.ci` -- functional test
- `test/managed/fleet-hub-singlewriter.ci` -- functional test

## Implementation Steps

### Implementation Phases

1. **Phase: Wiring (MANDATORY FIRST)** -- register `ze fleet status` and the freeze predicate; failing wiring tests
   - Tests: `fleet-freeze.ci` (initially failing)
   - Files: `session_factory.go`, `fleet-cmd` skeleton, `cmd/ze/hub/main.go`
   - Verify: `ze fleet status` reachable; guard hook present but stubbed

2. **Phase: Freeze guard** -- block operator commits when managed
   - Tests: `TestEditorFreezeWhenManaged`, `TestEditorNotFrozenStandalone`, `TestRollbackSaveFrozen`, `fleet-freeze.ci`
   - Files: `editor_commit.go`, `editor_commands.go`
   - Verify: operator commit rejected when managed; hub-apply still applies

3. **Phase: Baseline + reconnect hold** -- persist `base-version`; dirty check holds, marks `diverged`
   - Tests: `TestBaselineWrittenOnApply`, `TestReconnectDirtyHolds`, `TestReconnectCleanApplies`, `fleet-reconnect-hold.ci`
   - Files: `client.go`, `cmd/ze/hub/managed.go`, `ze_core_start.go`, `pkg/zefs/keys.go`
   - Verify: re-enable after an edit holds without stomping; clean re-enable applies normally

4. **Phase: Emergency disable/enable** -- immediate sever + unfreeze
   - Tests: `TestManagedImmediateSever`, `fleet-disable.ci`
   - Files: `client.go`, `cmd/ze/hub/main.go`, `fleet-cmd`
   - Verify: `ze fleet disable` drops the live connection and unfreezes

5. **Phase: Hub-side single-writer** -- connected-only edit guard on the hub
   - Tests: `TestHubEditorConnectedOnly`, `fleet-hub-singlewriter.ci`
   - Files: `internal/component/plugin/server/managed.go`
   - Verify: hub refuses to edit a disconnected device

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Hub-apply path never blocked by the freeze; reconnect holds on divergence and never auto-stomps |
| Coverage | CLI, SSH, and web commit paths all pass through the guarded functions (no bypass) -- validate A-1 |
| Baseline | `base-version` written on every hub-apply; read on (re)connect; not changed by a local edit |
| Naming | CLI uses `ze fleet` prefix; the fleet state is `diverged` (not `conflict`, which the editor already uses) |
| Rule: wiring-completeness | `ze fleet` commands and the guard reachable from `cmd/ze/hub/main.go` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Auth scope | `ze fleet disable/enable` require admin authorization |
| Freeze bypass | No operator write path reaches `config.conf` without the guard (A-1); blob backdoor documented (R-1) |
| Audit | `disable`/`enable` recorded in the audit trail (fleet-3 event types) |
| Hub-side guard | Connected-check cannot be raced to edit a device mid-disconnect |

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
<!-- Add a new block per re-run. Final run MUST show zero BLOCKER/ISSUE. -->
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-12 all demonstrated
- [ ] Wiring Test table complete
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Architecture docs updated (`docs/architecture/fleet-config.md`)
- [ ] Risks & Assumptions: every A-N confirmed or broken

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

## Disposition

| Field | Value |
|-------|-------|
| Date | 2026-11-18 |
| Checked at | `0e34a62474` |
| Verdict | REWORK |
| Evidence: defect still occurs | `internal/component/managed/client.go` `fetchAndProcess` calls `OnCommit` whenever the hub does not answer `current`. No baseline, no freeze and no fleet command exist at HEAD, and none of `951251d339`, `02921e203a`, `1c6a148bbc` adds one |
| Evidence: targets gone | `internal/component/cli/session_factory.go` and `internal/plugins/fleet-cmd/` do not exist. The session editor is built in `cmd/ze/hub/session_factory.go` `buildSessionModelFactory`. The hub verb dispatch is `internal/component/plugin/server/managed_serve.go` `handleRequest`; `managed.go` is `ManagedConfigService`, fetch and hashing only. `Editor.Rollback` lives in `internal/component/cli/editor.go`. Every line number in this spec is stale |
| Evidence: the baseline key is illegal | `meta/instance/managed` is a value key (`pkg/zefs/keys.go` `KeyInstanceManaged`), and the store refuses a key below a value: `pkg/zefs/tree.go` `set` answers "path conflict: segment ... is a file" (`TestStoreWriteFileLeafBecomesDirectory`, `TestStoreWriteFileConflictsWithDirectory`). The legal namespace is `meta/managed/`, as the hub bootstrap keys use |
| Evidence: A-1 is false | The device has no single operator chokepoint. Beside the session commit functions, the active config is written by the web editor's full-content route (`internal/component/web/editor.go` `applyCommittedContent`), the SSH config upload and `update bgp config` (`cmd/ze/hub/config_upload.go` `configMutationOwner`), the API config sessions (`internal/component/api/config_session.go`), `update data config`, `request data restore` and the rollback RPC (`internal/component/config/storage/cli/`), a reload of a file-sourced config, and the confirmed-commit revert |
| Evidence: A-2 is false | The first start writes the fetched config with a plain store write in `cmd/ze/ze_core_start.go` `cmdStartManaged`, not through `OnCommit`. A baseline written only on `OnCommit` is absent after the first boot, so the first reconnect would read as diverged |
| Evidence: AC-9 contradicts two design pages | `docs/guide/fleet-config.md` has the operator write a new client's first config before the client has ever connected, and promises that a client that is not connected gets a change when it next connects. `docs/architecture/fleet-config.md`: "A client that is not connected is owed nothing: it fetches when it connects" |
| Evidence: the hub has more than one writer | `client-<name>.conf` on a running hub is written by the session editor (no guard), `handleDataConfigUpdate`, `restoreConfigFromSource` and `handleDataConfigRollback`. The last three share `DataRPCTarget.ServesClient`, bound to `ManagedServer.Serves`, which means declared, not connected. `connectedClients` is unexported |
| Evidence: sever and rejoin have no owner | `cmd/ze/hub/main.go` `run` holds one `managedCancel` for both the managed client and the managed server, and launches `RunManagedClient` once. Nothing can stop the client alone, and nothing restarts it after it returns |
| Evidence: command shape predates the grammar | `docs/architecture/cli/command-verbs.md` puts the verb first, and `ai/META-DECISIONS.md` has every operator command run through the daemon on every surface. `ze fleet disable/enable/status` as a local command tree fits neither |
| Evidence: R-1 is out of date | On a running daemon `update data key` refuses a configuration key (`internal/component/config/storage/store.go` `RuntimeKey`), so a raw write to the config bypasses the guards only with the daemon stopped. The same RPC does accept `meta/instance/managed`, so the flag itself can already be flipped at runtime, with no sever |
| Evidence: no Existing Mechanisms table | The section is absent. Candidates: `confirm.WriteOutside` (refuses a write while a state is open, naming its owner, already applied to the hub push), `configMutationOwner` and its reload gate, the `ServesClient` guard, `fleet.VersionHash`, `zefs.KeyConfigActiveHash`, the doctor checks in `internal/component/managed/doctor.go`, `ClientConfig.adoptEndpoint` |

The goal holds and is still owed by the first release: a managed device that
rejoins its hub after a local emergency edit loses that edit, silently. The
design under it does not fit `main`.

Three things the skeleton took as given are false. It placed the freeze in two
editor functions on the belief that every operator write passes through them;
about ten routes change a device's active config, and they meet only at the
store's config-version functions and at the daemon's reload admission, which
the hub push uses too. So "exempt by construction" holds for the staging step
alone, and the freeze has to be placed where a write is admitted, with the hub
push as the one named exempt caller. It wrote the baseline where the hub push
commits; the first start does not go through that, so the baseline must be
written wherever a hub config becomes active. And it named a store key the
store cannot hold.

The hub-side rule is the part that needs the owner. As written, AC-9 forbids
the documented way to provision a client and the documented behavior of an
edit made while a client is away, and it would guard one of four writers. The
connected-only rule comes from the umbrella's single-writer model, so changing
it changes the umbrella too.

Two states the skeleton does not know. A managed client that applies a config
with no hub `client` block stops following its hub for good while its managed
flag stays true: frozen, with no hub to unfreeze it. And the client cannot be
stopped or restarted alone at runtime, so both the immediate sever and the
rejoin need a lifecycle owner that does not exist yet.

The smallest redesign that fits: a baseline under `meta/managed/`, written at
every point a hub config becomes active; the hold decided before the push is
committed, with a doctor code for `diverged`; the freeze at write admission on
the device; the existing `ServesClient` guard extended to the session editor on
the hub, with the connected-only question settled by the owner; verb-first
daemon commands with a local route; and an owner for the client's own
lifecycle. It adds no wire verb and changes no `pkg/fleet` envelope, and reload
stays one path. A new key in `pkg/zefs/keys.go` is a deliberate addition to a
public package.

Unverified: what `ze config edit` does against a store a running hub holds
when the hub's SSH is unreachable was read, not run. The reading is that the
temporary daemon cannot take the store's exclusive lock and the editor reports
a start timeout. A draft functional test would settle it.

The redesign runs through `/ze-spec`, and its design is put to the owner before
this spec moves to `ready`.
