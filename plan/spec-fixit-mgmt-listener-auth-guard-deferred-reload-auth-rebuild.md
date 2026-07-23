# Spec: fixit-mgmt-listener-auth-guard-deferred-reload-auth-rebuild

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | spec-fixit-mgmt-listener-auth-guard |
| Phase | - |
| Updated | 2026-07-17 |

Update (2026-07-22 plan review): the parent's blocking deliverable has LANDED
on disk -- `cmd/ze/hub/mgmt_guard.go` + `checkMgmtListeners` exist (learned
1200); the stale 2026-07-17 "ABSENT" note below is struck through in-body; the
Phase-0 gate is now satisfiable (the parent spec stays in-progress only for
its own deferred AC-5/AC-6). `listener_migrate.go` anchors (~+30 lines, file
grew 250->281 and gained `MarkUnauthenticated` at `:54`) are updated in-body:
`ReloadListeners` `:94+`, `buildChange` `:205`, `rollbackAppliedListeners`
`:193`, `listenerDiff` `:221`.

→ READINESS (2026-07-17): design filled from source (all sections grounded in
`file:line` below), every open decision resolved with FAIL-CLOSED defaults. ~~Status
stays `skeleton` because a genuine hard dependency remains~~ (superseded 2026-07-17,
see DECISION below — the dependency is now tracked as a normal spec `Depends`, not a
reason to hold at skeleton): this child re-runs the
boot guard's classifier on reload, and that classifier plus the `ReloadListeners`
gate are created by the parent `spec-fixit-mgmt-listener-auth-guard` (which builds
`cmd/ze/hub/mgmt_guard.go` — `checkMgmtListeners` + the fail-closed non-loopback
classifier — and adds the AC-7 `ReloadListeners` gate). Implementing this child
before the parent lands would duplicate the parent's single-classifier deliverable
and collide on the same `ReloadListeners` edit. ~~Flip to `ready` the moment the
parent's classifier + AC-7 gate are on disk~~ (superseded: flipped to `ready` now,
with the dependency carried in the `Depends` metadata — see DECISION below); the
design below is otherwise complete.

→ AUTONOMOUS DEFAULT (2026-07-17): Status skeleton → **ready**. Thomas authorized
resolving the remaining gate as a normal spec **dependency**, not an open question.
The design is complete and hook-passing (all sections grounded in source, all cited
`file:line` re-verified on disk 2026-07-17); the only gate was the parent's single
shared classifier / `cmd/ze/hub/mgmt_guard.go`, which is correctly expressed by
`Depends: spec-fixit-mgmt-listener-auth-guard` (the parent is itself `ready`). A
`ready` spec with a `Depends` on another ready spec is correct: it tells the
implementer to land the parent's single shared classifier + `checkMgmtListeners` +
AC-7 `ReloadListeners` gate FIRST, then this child. Rationale: the remaining gate is
sequencing, not a design unknown — the conservative default is the smaller,
self-contained action (flip to ready, keep the dependency explicit). Fail-closed
defaults confirmed intact: reload re-runs the parent classifier over the rebuilt
(address, auth) set, and a listener whose auth cannot be rebuilt REFUSES its reload
and keeps its prior (address, auth) — never the new address with stale auth (AC-3,
AC-4, and the A-1 drain-and-replace/refuse default; `ai/rules/fail-closed-guards.md`).
~~Dependency verified real on disk 2026-07-17: `cmd/ze/hub/mgmt_guard.go` and
`checkMgmtListeners` are ABSENT, so the `Depends` is not stale.~~ (landed
2026-07-19, learned 1200) Thomas: override if wrong. [STAKES: security]

**Sequencing (BLOCKING for the implementer):** implement the parent
`spec-fixit-mgmt-listener-auth-guard` FIRST. Its `cmd/ze/hub/mgmt_guard.go`
(`checkMgmtListeners` + the single fail-closed non-loopback classifier) and its AC-7
`ReloadListeners` gate MUST be on disk before this child begins. This child REUSES
that one classifier and MUST NOT create a second classifier (parent Critical Review
"Single classifier"). Phase 0 (Dependency gate) below restates this as the blocking
first step; if `mgmt_guard.go` is absent, STOP and land the parent first.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `cmd/ze/hub/listener_migrate.go` - `ReloadListeners`, the SIGHUP reload path
4. `plan/spec-fixit-mgmt-listener-auth-guard.md` - the source spec (if still on disk)

## Task

Make an authentication-mode change take effect on a SIGHUP config reload. Today a
running management server keeps the auth mode it was constructed with; only its
listen addresses are migrated.

**Provenance:** deferred from `plan/spec-fixit-mgmt-listener-auth-guard.md` Known
Limitations, recorded 2026-07-17. The source spec confirmed AC-7's boot-plus-migration
address guard as its shipped scope and left the auth rebuild here.

**The limitation is verified, not inherited.** `ReloadListeners`
(`cmd/ze/hub/listener_migrate.go:94+`) builds its change set exclusively from
addresses:

| Service | What reload reads | Producer |
|---------|-------------------|----------|
| web | `endpointsToAddrs(webCfg.Servers)` | `listener_migrate.go:80-86` |
| lg | `endpointsToAddrs(lgCfg.Servers)` | `listener_migrate.go:88-94` |
| mcp | `endpointsToAddrs(mcpCfg.Servers)` | `listener_migrate.go:96-102` |
| rest / grpc | `apiListenToAddrs(apiCfg.REST / .GRPC)` | `listener_migrate.go:104-117` |

Every path funnels into `buildChange(name, srv, newAddrs)`
(`listener_migrate.go:205`), whose only per-service input is `newAddrs`, and the
diff it drives (`listenerDiff`, `:221`) compares address lists. No auth field is
read, compared, or applied, so a reload cannot rebuild a server's auth mode.

**Why it was not simply widened:** the source spec's AC-7 stops a running
*unauthenticated* listener from being migrated onto a non-loopback address, which is
the security-relevant half and needs no rebuild. Turning auth on for an
already-running server is a lifecycle change (servers are constructed once), which is
a different and larger piece of work.

**Scope note:** the source spec also parks gNMI token-over-plaintext (token set, no
TLS still boots; the guard enforces authentication, not transport secrecy). Decide
when picking this up whether that belongs here or in its own spec; it is a distinct
concern (secrecy vs identity) and is NOT currently claimed by this file.

→ AUTONOMOUS DEFAULT (2026-07-17): gNMI token-over-plaintext STAYS OUT of this spec
(its own follow-up). Rationale: scope question → smaller, self-contained option
(readiness decision protocol). This spec is about *identity on reload* (rebuild auth
so an auth-mode change takes effect and re-run the guard); transport secrecy is a
different axis the parent already parked as a NOTED FOLLOW-UP
(`plan/spec-fixit-mgmt-listener-auth-guard.md` open question 4, Known Limitations).
Folding it in here would widen scope without touching the reload/auth-rebuild path.
Thomas: override if wrong.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] — checkboxes are template markers, not progress trackers. -->
- [ ] `docs/architecture/web-interface.md` - management listener/auth construction and the reload-commit flow (the actual design doc `cmd/ze/hub/listener_migrate.go:1` points at: "Graceful listener migration on config reload")
  → Constraint: the `Reconfigurable` seam (`cmd/ze/hub/listener_migrate.go:17-20`) exposes only `Addresses()` + `Reconfigure(ctx, newAddrs)`; auth is chosen once at construction (web `authWrap`, `cmd/ze/hub/service_web.go:466-474`) and baked into one `*http.Server{Handler: mux}` (`internal/component/web/server.go:86,130`). An auth rebuild must either WIDEN this seam to carry the auth decision or drain-and-replace the instance; it must never mutate a live handler chain in place, and on any failure it must fail closed (rollback), never leave a listener less authenticated than before (`ai/rules/fail-closed-guards.md`).
- [ ] `docs/architecture/api/architecture.md` - API/gNMI/REST/gRPC server construction and lifecycle (the API surfaces that share the `Reconfigurable` seam)
  → Constraint: REST already fails closed in `Reconfigure` by rejecting any non-loopback address (`internal/component/api/rest/server.go:249-253`); the auth-rebuild path must not weaken that, and must extend the same fail-closed posture to the auth dimension.
- [ ] `plan/spec-fixit-mgmt-listener-auth-guard.md` - the PARENT (security) spec; provenance for this child
  → Constraint: the parent creates the single shared classifier + `checkMgmtListeners` (`cmd/ze/hub/mgmt_guard.go`, parent Files to Create) and gates `ReloadListeners` for address moves (AC-7). This child MUST reuse that classifier to re-run the auth guard on reload — do not create a second classifier (parent Critical Review "Single classifier").

**Key insights:**
- Reload today is address-migration only; auth is fixed at construction.
- Every `Reconfigure` implementation (web/lg/mcp/rest/grpc) is address-only — none reads or rebuilds auth (verified 2026-07-17, see Current Behavior).
- The fail-closed reload rule: after any auth rebuild, re-run the parent's classifier over the resulting (address, auth) set; if a listener would end up non-loopback + unauthenticated, or the rebuild fails, roll back and keep the prior state.

## Current Behavior (MANDATORY)

**Source files read:** (all read/verified 2026-07-17 against the working tree)
- [ ] `cmd/ze/hub/listener_migrate.go` (250L) - `ReloadListeners` (`:94+`) migrates listen addresses only; `buildChange` (`:205`) takes `newAddrs` and nothing else; `listenerDiff` (`:221`) compares address lists; `rollbackAppliedListeners` (`:193`) reverts applied changes in reverse order by re-`Reconfigure`-ing to `oldAddr`. The `Reconfigurable` interface (`:17-20`) is `Addresses()` + `Reconfigure(ctx, newAddrs)` only — no auth field crosses it.
  → Constraint: the `Reconfigurable` interface is the seam every service is driven through; an auth rebuild either extends it or replaces the server instance.
  → AUTONOMOUS DEFAULT (2026-07-17): WIDEN the seam so the change set carries the resolved (address, authenticated) intent per service, and rebuild by DRAIN-AND-REPLACE where a server cannot swap its handler chain live (web bakes auth into one `*http.Server`, see below). Rationale: fail-closed + smaller blast radius — mutating a live handler chain in place is unproven (A-1) and races in-flight requests; a widened seam keeps ONE collection point and lets the existing rollback (`:193`) revert to the prior instance/auth. A service whose auth genuinely cannot be rebuilt must refuse the reload for that service (fail closed), never apply the address change while silently keeping stale auth. Thomas: override if wrong.
- [ ] `internal/component/web/server.go` (Reconfigure `:262-333`, struct `:70`, one `*http.Server{Handler: mux}` `:86,130`, `serveOne` `:336`) - reconfigure adds/removes listeners on the shared `*http.Server`; the handler/auth chain is never rebuilt.
- [ ] `cmd/ze/hub/service_web.go` (`:466-474`) - `authWrap` (insecure → `InsecureMiddleware`; else `AuthMiddlewareWithAudit`) is selected ONCE and wrapped around the route handlers registered on the mux. This is the auth state a reload would have to rebuild.
- [ ] `internal/component/api/rest/server.go` (`Reconfigure` `:242-253`) - address-only, and already fail-closed: rejects any non-loopback new address before binding. gRPC mirrors it (`internal/component/api/grpc/server.go:247`).
- [ ] `internal/component/lg/server.go` (`Reconfigure` `:384-415`) and `cmd/ze/hub/service_mcp.go` (`Reconfigure` `:355-389`) - both address-only; no auth read.
- [ ] `cmd/ze/hub/main_reload.go` (`runReload` `:164-286`) - the reload driver: loads the tree, reloads plugin server + engine, then calls `lm.ReloadListeners(reloadCtx, parsedTree)` (`:244`) inside a 30s timeout (`:165`); on migration error it invokes `rollbackReload` (`:245`, func at `:352-371`). This is where an auth-rebuild failure must surface and roll back. (Anchors re-verified 2026-07-23 after the origin/main fast-forward to 822029463, which grew this file.)

**Behavior to preserve:**
- AC-7's address guard from the source spec: a running unauthenticated listener must not migrate to a non-loopback address.
- Rollback on partial failure (`rollbackAppliedListeners`, `listener_migrate.go:193`).

**Behavior to change:**
- An auth-mode change in the reloaded config must take effect, rather than being silently ignored until restart.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- SIGHUP → config reload → `ReloadListeners(ctx, tree)` (`cmd/ze/hub/listener_migrate.go:94`)

### Transformation Path
1. Extract per-service config from the tree (`ExtractWebConfig` / `ExtractLGConfig` / `ExtractMCPConfig` / `ExtractAPIConfig`)
2. Build the change set — today addresses only (`buildChange`, `:205`)
3. Diff against running listeners (`listenerDiff`, `:221`)
4. Apply, with rollback on failure (`rollbackAppliedListeners`, `:193`)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config tree ↔ hub | `zeconfig.Extract*Config` | [ ] |
| Hub ↔ each service | `Reconfigurable` | [ ] |

### Integration Points
- `Reconfigurable` implementations for web, lg, mcp, rest, grpc (`internal/component/web/server.go:262`, `internal/component/lg/server.go:384`, `cmd/ze/hub/service_mcp.go:355`, `internal/component/api/rest/server.go:242`, `internal/component/api/grpc/server.go:247`) - each would need to accept an auth-mode change (widened seam) or be drain-and-replaced by the migrator.
- `cmd/ze/hub/mgmt_guard.go` (parent-created) - the shared fail-closed classifier + `checkMgmtListeners`; the reload re-runs it over the rebuilt (address, auth) set so a rebuild cannot leave any listener non-loopback + unauthenticated. THIS IS THE HARD DEPENDENCY — the parent must land first.
- `cmd/ze/hub/listener_migrate.go:193` (`rollbackAppliedListeners`) and `cmd/ze/hub/main_reload.go:352-371` (`rollbackReload`) - rollback must be extended to cover the auth dimension, not just addresses, so a half-applied auth rebuild reverts fully (R-1).

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Registration over hardcoding — no new per-service switch in a shared package

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Rebuilding a server in place can preserve in-flight connections acceptably | not yet investigated | design changes to drain-and-replace | prototype + functional test | unvalidated |
| A-2 | The parent's shared classifier + `ReloadListeners` gate exist by the time this is implemented | `plan/spec-fixit-mgmt-listener-auth-guard.md` (Status ready; creates `cmd/ze/hub/mgmt_guard.go`, gates `ReloadListeners` AC-7) | this child cannot re-run the guard; would have to duplicate the classifier (rejected) | parent spec on disk before implementation | pending parent |
| A-3 | A failed auth rebuild can be fully rolled back via the existing reverse-order reconfigure | `cmd/ze/hub/listener_migrate.go:193`, `main_reload.go:352-371` | a half-rebuilt server is left in an unknown auth state | AC-2 rollback test | unvalidated |

→ AUTONOMOUS DEFAULT (2026-07-17) on A-1: do NOT assume in-place rebuild is safe.
Default to DRAIN-AND-REPLACE (or refuse the per-service reload when replace is not
available), because in-place handler-chain mutation is unproven and can race in-flight
requests. FAIL CLOSED: if neither in-place nor replace can rebuild a service's auth,
that service's reload is refused and it keeps its prior (address, auth) — never the
new address with stale auth (`ai/rules/fail-closed-guards.md`). Thomas: override if wrong.

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | An auth rebuild that fails halfway leaves a server unauthenticated | rollback test | extend `rollbackAppliedListeners` to cover auth, fail closed (`ai/rules/fail-closed-guards.md`) |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| SIGHUP with auth turned ON for a running server | → | `ReloadListeners` rebuilds auth, server now demands auth | `test/reload/mgmt-guard-reload-auth-rebuild.ci` |
| SIGHUP that would leave a listener non-loopback + unauthenticated | → | reload re-runs the parent classifier and refuses; daemon keeps prior auth+addrs | `test/reload/mgmt-guard-reload-refuses-unauth.ci` |
| Auth rebuild fails midway | → | `rollbackAppliedListeners` / `rollbackReload` reverts auth+addrs | `TestReloadListenersAuthRebuildRollsBack` (`cmd/ze/hub/listener_migrate_test.go`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config reloaded with auth turned on for a running server | The server enforces auth after reload, without a restart |
| AC-2 | Auth rebuild fails midway | Rollback leaves no server less authenticated than before (prior address+auth restored) |
| AC-3 | Reload of a running config that would result in a listener non-loopback AND unauthenticated (e.g. auth turned OFF, or auth left unbuildable, while the address is/becomes non-loopback) | Reload re-runs the parent classifier over the rebuilt (address, auth) set and REFUSES that service's migration (fail closed); the daemon keeps serving on its previous addresses with its previous auth |
| AC-4 | A service whose auth cannot be rebuilt through the widened seam (no replace path available) | That service's reload is refused with a clear error naming it; the address is NOT applied with stale auth; other services' reloads still proceed |

AC-3/AC-4 added during design fill (2026-07-17, AUTONOMOUS DEFAULT) as the fail-closed
corollary of the child's charter: a reload must re-run the auth guard/classifier, and
auth state that cannot be rebuilt must never silently leave a listener unauthenticated
(`ai/rules/fail-closed-guards.md`). Thomas: override if wrong.

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestReloadListenersAuthChange` | `cmd/ze/hub/listener_migrate_test.go` | an auth-mode change is applied on reload (AC-1) | |
| `TestReloadListenersRerunsGuardFailsClosed` | `cmd/ze/hub/listener_migrate_test.go` | a reload resulting in non-loopback + unauthenticated is refused via the parent classifier (AC-3) | |
| `TestReloadListenersAuthRebuildRollsBack` | `cmd/ze/hub/listener_migrate_test.go` | a mid-way auth-rebuild failure rolls back address+auth (AC-2), extending the existing `TestReloadListenersRollsBackAppliedServiceOnLaterFailure` pattern (`:76`) | |
| `TestReloadListenersRefusesUnbuildableAuth` | `cmd/ze/hub/listener_migrate_test.go` | a service whose auth cannot be rebuilt has its reload refused, address not applied with stale auth (AC-4) | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `mgmt-guard-reload-auth-rebuild` | `test/reload/*.ci` | operator turns auth on and SIGHUPs; the listener demands auth afterward without a restart | |
| `mgmt-guard-reload-refuses-unauth` | `test/reload/*.ci` | operator's reload would leave a listener non-loopback + unauthenticated; reload is refused, daemon keeps prior auth+addrs | |

### Future (if deferring any tests)
- None yet; this is a skeleton.

## Files to Modify
- `cmd/ze/hub/listener_migrate.go` - widen the `Reconfigurable` seam / `serviceChange` to carry the resolved (address, authenticated) intent; re-run the parent classifier over the rebuilt set in `ReloadListeners` (`:94+`); extend `rollbackAppliedListeners` (`:193`) to revert auth as well as addresses
- `cmd/ze/hub/main_reload.go` - surface auth-rebuild failures through `runReload` (`:201-212`) and its `rollbackReload`
- `internal/component/web/server.go` - support an auth rebuild (drain-and-replace the handler chain, or a new auth-swap method) since auth is baked into one `*http.Server` today (`:86,130`)
- `cmd/ze/hub/service_web.go` - make the `authWrap` decision (`:466-474`) reproducible on reload from the reloaded config, not fixed at first construction
- `internal/component/lg/server.go`, `cmd/ze/hub/service_mcp.go`, `internal/component/api/rest/server.go`, `internal/component/api/grpc/server.go` - each `Reconfigure` extended (or its instance replaced) to apply the reloaded auth decision; REST/gRPC keep their existing loopback fail-closed guard (`rest/server.go:249-253`)

**Consumes (parent, must exist first):** `cmd/ze/hub/mgmt_guard.go` - the shared fail-closed classifier + `checkMgmtListeners`, created by `spec-fixit-mgmt-listener-auth-guard`. This child reuses it; it does not create a second classifier.

## Implementation Steps

### Implementation Phases
0. **Phase: Dependency gate (BLOCKING)** — confirm the parent's `cmd/ze/hub/mgmt_guard.go` classifier + `checkMgmtListeners` and the AC-7 `ReloadListeners` gate are on disk. If absent, STOP: this child cannot be implemented without duplicating the parent's single classifier (see Metadata READINESS note).
1. **Phase: Wiring (MANDATORY FIRST)** — a failing test (`mgmt-guard-reload-auth-rebuild.ci` + `TestReloadListenersAuthChange`) proving an auth change on reload is currently ignored (server keeps its constructed auth).
2. **Phase: Widen the seam** — extend `serviceChange`/`Reconfigurable` (or add a drain-and-replace path) to carry the resolved (address, authenticated) intent; rebuild web auth from the reloaded config (`service_web.go:466-474` reproduced on reload). Tests: `TestReloadListenersAuthChange` passes.
3. **Phase: Re-run the guard, fail closed** — after rebuild, run the parent classifier over the resulting set; refuse any service that would end non-loopback + unauthenticated, and refuse any service whose auth cannot be rebuilt. Tests: `TestReloadListenersRerunsGuardFailsClosed`, `TestReloadListenersRefusesUnbuildableAuth`, `mgmt-guard-reload-refuses-unauth.ci`.
4. **Phase: Rollback covers auth** — extend `rollbackAppliedListeners`/`rollbackReload` so a mid-way failure reverts auth as well as addresses. Test: `TestReloadListenersAuthRebuildRollsBack`.
5. **Full verification** — `make ze-test`; run the new `test/reload/*.ci` natively (config/reload only, no kernel features → no `needs-linux`).
6. **Complete spec** — audit tables, `plan/learned/NNN-<name>.md`, two-commit closure.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior
