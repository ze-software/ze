# Spec: fixit-mgmt-listener-auth-guard-deferred-reload-auth-rebuild

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-fixit-mgmt-listener-auth-guard |
| Phase | implementation complete, review round 4 findings fixed |
| Updated | 2026-08-08 |

## Implementation (2026-08-07)

The trace in this file held on re-verification: `ReloadListeners`
(`cmd/ze/hub/listener_migrate.go`) read no auth field, and an auth-only edit
produced an empty change set, so the reload returned nil having logged nothing.

**Shape chosen: widen the seam, not drain-and-replace.** The repository already
reconfigures a security property of a running server exactly this way:
`TLSUpdatable` / `UpdateWebCertificate` is a narrow capability interface declared
beside `Reconfigurable`, driven from the migrator, failing the reload rather than
downgrading. `AuthUpdatable` is its twin. Drain-and-replace was rejected because
no service is constructed in a way that survives it: the web server bakes its
auth decision into ~30 mux registrations built once, and replacing the instance
would also replace the event broker, the ring wiring, and the shutdown handle.

| AC | Where | Proven by |
|----|-------|-----------|
| AC-1 | `(*rest.RESTServer).UpdateAuth`, `(*grpc.GRPCServer).UpdateAuth`, `applyAuthIntents` | `TestReloadListenersRebuildsAuthenticationOn` / `...Off`; `TestRESTUpdateAuthTurnsAuthenticationOn` / `...Off`; `TestGRPCUpdateAuthTurnsAuthenticationOn` / `...Off` (a real RPC) |
| AC-2 | `UpdateAuth` returns a restore closure; `migrateListeners` runs it beside the address rollback | `TestReloadListenersRestoresAuthenticationWhenMigrationFails`, `TestRESTUpdateAuthRestoreRevertsCredentials`, `TestGRPCUpdateAuthRestoreRevertsCredentials` |
| AC-3 | `checkReloadExposure` re-runs the classifier over LIVE auth via `runningAuth`; gRPC refuses to unauthenticate a non-loopback listener itself | `TestReloadListenersRefusesRebuiltUnauthenticatedNonLoopback`, `TestGRPCUpdateAuthRefusesToUnauthenticateNonLoopback` |
| AC-4 | `checkAuthRebuildable` refuses the reload, before anything is applied, when a service must change its auth and cannot | `TestReloadListenersFailsWhenAuthCannotBeRebuilt` |
| AC-5 | `resolveAuthIntents` (`cmd/ze/hub/listener_migrate.go`) skips a surface with no handle, so no intent exists for one the daemon never built | `TestUnbuiltSurfaceResolvesNoAuthIntent`, `TestReloadListenersProceedsWhenSiblingTransportWasNeverBuilt`, `test/reload/mgmt-guard-reload-unbuilt-transport.ci` |

→ Correction, round 4 (2026-08-08): **AC-5's handle test moved from MARK time to
RELOAD time, because putting it at mark time opened a fail-open window.** Round 3
gated `markMgmtAuth` on `hasService`, which forced it to run after every
management server was built. The handles are installed EARLIER than that:
`lm.SetWeb` / `lm.SetMCP` through `registerBuiltService`, and `lm.SetREST` /
`lm.SetGRPC`. In that window the migrator held handles with an empty
`authAtBoot`, so `runningAuth` answered `known=false` for web and mcp (neither
implements `AuthUpdatable`) and `checkReloadExposure` SKIPPED them, which is its
permissive branch. Nothing else backstops web: `RESTServer` refuses non-loopback
in two places and `GRPCServer.setAuthLocked` self-guards, but the web server
carries no loopback rule of its own. The window was reachable because commit
paths reach live surfaces before the mark: the SSH `ReloadFn`, the web service
`CommitHook`, `apiServer.SetFullReloadFunc` and `wireManagedCommit`.

`markMgmtAuth` now runs BEFORE any handle reaches the migrator and classifies
every surface, and `resolveAuthIntents` drops the ones with no handle. That
needs no startup ordering at all. The reviewer named `checkAuthRebuildable` as
the place for the test; it is one place too late. `resolveAuthIntents` CALLS each
reloader, and `apiAuthReloader` fails the whole reload when the power-user
credentials stop being readable, so a surface classified but never built would
have failed reloads over a server that does not exist. Skipping the check
without skipping the call does not hold AC-5. `checkAuthRebuildable` needs no
test of its own and has none: it only ever sees intents `resolveAuthIntents`
produced.

`TestMarkMgmtAuthClassifiesOnlyBuiltSurfaces` was DELETED, not left beside the
new arrangement (`ai/rules/no-layering.md`). It pinned the mark-time gate, which
is the fail-open. `TestMarkMgmtAuthClassifiesBeforeAnyHandleExists` replaces it
and drives the window directly: mark, then install an unauthenticated web
handle, then run the exposure check.

→ Note: `SetLG` (`cmd/ze/hub/listener_migrate.go`) calls
`registerAuthUpdater(svcLG, s)`. No looking-glass type implements
`AuthUpdatable` today, so `runningAuth("lg")` stays `known=false` and the
documented "leave the looking glass alone" holds. It holds by accident: the day
an LG type gains `UpdateAuth`, that line makes the reload guard start REFUSING
non-loopback looking-glass migrations, the opposite of an intentionally public
surface. The dependency is now named in `SetLG`'s comment, where whoever
implements the interface will read it.

→ Note: gNMI is classified at boot by `checkMgmtListeners` and is deliberately
absent from `markMgmtAuth`'s map. No exposure follows, because gNMI has no
`ListenerMigrator` slot, so `buildChanges` can never move its listener and there
is no migration for the guard to judge. The asymmetry is stated at the
`markMgmtAuth` call site in `cmd/ze/hub/main.go`.

→ Decision: the boot-time `unauth map[string]bool` snapshot is DELETED, not
layered (`ai/rules/no-layering.md`). `authAtBoot` records both polarities and
`runningAuth` prefers the live server, because a rebuilt server's boot record is
exactly the stale answer this work removes.

→ Constraint: a service the boot guard never classified stays outside the reload
guard. The looking glass is the case, and treating its silence as
unauthenticated would have started refusing its migrations.

→ Decision, corrected after review (2026-08-07): a service that must change its
auth and cannot FAILS the reload. The first implementation logged and dropped
that service's address change, then returned nil. `runReload` therefore ran to
completion and promoted the candidate, so `ze config commit` reported success
over a web server still serving unauthenticated: the exact defect this spec
exists to remove, surviving for the two surfaces that cannot rebuild.

The earlier reasoning for returning nil was that refusing would lock out every
later SIGHUP. That was inferred, never read, and it is wrong.
`storage.PromoteCandidate` runs only after every step succeeds, and the refusal
paths clear the candidate, so a refused reload discards the operator's edit and
the next SIGHUP re-reads the config FILE and sees the unchanged active pointer.
There is no lockout.

Precisely, because the general claim is not true: `clearCandidate` is called on
the refusal paths, NOT on every error path. A `PromoteCandidate` failure calls
neither `clearCandidate` nor `rollbackReload`, so the candidate pointer survives
while the active pointer still names the old config. That is a separate
divergence, it is not this spec's, and it does not revive the lockout claim,
which stays withdrawn.

The refusal is raised in `checkAuthRebuildable`, BEFORE anything is applied,
which is what lets both halves of AC-4 hold at once: no listener has moved, so
none is left on an address the rolled-back config no longer describes. The
review's suggested ordering (migrate the others, then error) was not taken for
that reason: `runReload`'s rollback restores config and PKI but cannot move a
listener back.

→ Constraint: `ReloadListeners` returns an undo for the credentials it
installed, and `runReload` runs it from a DEFER guarded by a success flag, not
from a call on each failure path. `UpdateWebCertificate` and `PromoteCandidate`
both run later; without the undo, a reload the operator was told had FAILED left
REST and gRPC authenticating against the config the daemon had just rolled back.
The first implementation hand-placed the call and had already missed the
promote path, which is why the flag replaced it: a new failure path inherits the
undo instead of having to remember it.

→ Constraint: restoring credentials is a credential CHANGE and is judged
against the addresses in force WHEN IT RUNS. `(*grpc.GRPCServer).UpdateAuth`'s
undo re-runs `checkGRPCListenAddr` and KEEPS the reloaded credentials when
putting the old ones back would expose the listener. Without that, one reload
could install a token, move gRPC to `0.0.0.0` (both guards passing, because the
credentials were in place when each looked), then fail at a later step and
restore the empty credentials onto the public port: the original defect
mirrored, with the operator told the reload FAILED. REST needs no such check
because it refuses every non-loopback address; if that ever changes, its undo
becomes the same defect.

→ Constraint: `checkReloadExposure` judges authentication only. A transport
requirement belongs to the server that knows its transport: `Reconfigure` now
applies `checkGRPCListenAddr`, the same rule `NewGRPCServer` applies, so a
reload cannot move an authenticated plaintext gRPC listener off loopback. That
was reachable before, and it is a state the daemon refuses to boot into.

→ Constraint: a surface the boot guard never classified is skipped in
`resolveAuthIntents`, and its reloader is never called.
`registerMgmtAuthReloaders` registers all four unconditionally, so a binary
built without a surface would otherwise fail every reload over a config block
describing a server it cannot run.

**Not claimed, and now verified real:** gNMI serves its token over plaintext.
`(*gnmi.Server).Serve` (`internal/component/gnmi/server.go`) adds the token
interceptor and the TLS credentials from two independent conditions, so a token
with no cert/key binds and every client sends the bearer in cleartext.
`gnmiBuildImpl` (`cmd/ze/hub/service_gnmi.go`) also downgrades an unreadable
cert path to plaintext with a warning. The API gRPC server pairs the two
(`NewGRPCServer` refuses authenticated non-loopback without TLS); gNMI does not.
This stays out of this spec, per the 2026-07-17 autonomous default.

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
AC-4, and the A-1 drain-and-replace/refuse default; `ai/rules/evidence.md`).
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
4. `spec-fixit-mgmt-listener-auth-guard` - the source spec (if still on disk)

## Task

Make an authentication-mode change take effect on a SIGHUP config reload. Today a
running management server keeps the auth mode it was constructed with; only its
listen addresses are migrated.

**Provenance:** deferred from `spec-fixit-mgmt-listener-auth-guard` Known
Limitations, recorded 2026-07-17. The source spec confirmed AC-7's boot-plus-migration
address guard as its shipped scope and left the auth rebuild here.

**The limitation is verified, not inherited.** `ReloadListeners`
(`cmd/ze/hub/listener_migrate.go+`) builds its change set exclusively from
addresses:

| Service | What reload reads | Producer |
|---------|-------------------|----------|
| web | `endpointsToAddrs(webCfg.Servers)` | `listener_migrate.go` |
| lg | `endpointsToAddrs(lgCfg.Servers)` | `listener_migrate.go` |
| mcp | `endpointsToAddrs(mcpCfg.Servers)` | `listener_migrate.go` |
| rest / grpc | `apiListenToAddrs(apiCfg.REST / .GRPC)` | `listener_migrate.go` |

Every path funnels into `buildChange(name, srv, newAddrs)`
(`listener_migrate.go`), whose only per-service input is `newAddrs`, and the
diff it drives (`listenerDiff`, `:221`) compares address lists. No auth field is
read, compared, or applied, so a reload cannot rebuild a server's auth mode.

## Re-verified 2026-08-05: the gap is real, and it is SILENT

Confirmed at the producer. `buildChange` (`cmd/ze/hub/listener_migrate.go`) takes
`newAddrs` as its only per-service input, runs `listenerDiff` over address lists,
and returns `false` when the two lists agree:

and returns "no change" when the old and new address lists agree.

So an operator who edits config to turn authentication ON, without touching the
listen address, produces NO change set at all. `ReloadListeners` logs nothing:
its three log lines all sit inside the per-change loop that this reload never
enters. The operator sees a clean reload and believes the service is
authenticated. It is not.

**What limits the damage, and what does not.** The parent spec's AC-7 shipped and
holds: `MarkUnauthenticated` records services built without authentication and
`ReloadListeners` refuses to migrate one to a non-loopback address. So the
unauthenticated server cannot be moved somewhere reachable. It is not remotely
exploitable. What survives is the operator's false belief, which nothing corrects.

**Two separable deliverables, and they are not the same size.**

| # | Deliverable | Size |
|---|-------------|------|
| 1 | Say something. Compare the configured auth mode against what the running server was built with, and report the difference on reload | Small. Needs the configured mode plumbed to the migrator, which today only holds the `unauth` name set |
| 2 | Rebuild. Stop and reconstruct the server so the new auth mode takes effect | Large. Servers are constructed once, so this is connection draining, restart ordering, and rollback |

Deliverable 1 satisfies "fail closed OR say something" (`ai/rules/evidence.md`)
and is complete on its own axis. It was NOT landed on 2026-08-05, because taking
it alone would reduce this spec's stated scope without approval
(`ai/rules/completion.md`). **If Thomas wants the cheap half first, say so and it
is a short change.** Otherwise both land together.

Status moved `ready` from the previous value: the research is done, the producer
is verified, and the two deliverables are sized.

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
(`spec-fixit-mgmt-listener-auth-guard` open question 4, Known Limitations).
Folding it in here would widen scope without touching the reload/auth-rebuild path.
Thomas: override if wrong.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] — checkboxes are template markers, not progress trackers. -->
- [ ] `docs/architecture/web-interface.md` - management listener/auth construction and the reload-commit flow (the actual design doc `cmd/ze/hub/listener_migrate.go` points at: "Graceful listener migration on config reload")
  → Constraint: the `Reconfigurable` seam (`cmd/ze/hub/listener_migrate.go`) exposes only `Addresses()` + `Reconfigure(ctx, newAddrs)`; auth is chosen once at construction (web `authWrap`, `cmd/ze/hub/service_web.go`) and baked into one `*http.Server{Handler: mux}` (`internal/component/web/server.go,130`). An auth rebuild must either WIDEN this seam to carry the auth decision or drain-and-replace the instance; it must never mutate a live handler chain in place, and on any failure it must fail closed (rollback), never leave a listener less authenticated than before (`ai/rules/evidence.md`).
- [ ] `docs/architecture/api/architecture.md` - API/gNMI/REST/gRPC server construction and lifecycle (the API surfaces that share the `Reconfigurable` seam)
  → Constraint: REST already fails closed in `Reconfigure` by rejecting any non-loopback address (`internal/component/api/rest/server.go`); the auth-rebuild path must not weaken that, and must extend the same fail-closed posture to the auth dimension.
- [ ] `spec-fixit-mgmt-listener-auth-guard` - the PARENT (security) spec; provenance for this child
  → Constraint: the parent creates the single shared classifier + `checkMgmtListeners` (`cmd/ze/hub/mgmt_guard.go`, parent Files to Create) and gates `ReloadListeners` for address moves (AC-7). This child MUST reuse that classifier to re-run the auth guard on reload — do not create a second classifier (parent Critical Review "Single classifier").

**Key insights:**
- Reload today is address-migration only; auth is fixed at construction.
- Every `Reconfigure` implementation (web/lg/mcp/rest/grpc) is address-only — none reads or rebuilds auth (verified 2026-07-17, see Current Behavior).
- The fail-closed reload rule: after any auth rebuild, re-run the parent's classifier over the resulting (address, auth) set; if a listener would end up non-loopback + unauthenticated, or the rebuild fails, roll back and keep the prior state.

## Current Behavior (MANDATORY)

**Source files read:** (all read/verified 2026-07-17 against the working tree)
- [ ] `cmd/ze/hub/listener_migrate.go` (250L) - `ReloadListeners` (`:94+`) migrates listen addresses only; `buildChange` takes `newAddrs` and nothing else; `listenerDiff` compares address lists; `rollbackAppliedListeners` reverts applied changes in reverse order by re-`Reconfigure`-ing to `oldAddr`. The `Reconfigurable` interface is `Addresses()` + `Reconfigure(ctx, newAddrs)` only — no auth field crosses it.
  → Constraint: the `Reconfigurable` interface is the seam every service is driven through; an auth rebuild either extends it or replaces the server instance.
  → AUTONOMOUS DEFAULT (2026-07-17): WIDEN the seam so the change set carries the resolved (address, authenticated) intent per service, and rebuild by DRAIN-AND-REPLACE where a server cannot swap its handler chain live (web bakes auth into one `*http.Server`, see below). Rationale: fail-closed + smaller blast radius — mutating a live handler chain in place is unproven (A-1) and races in-flight requests; a widened seam keeps ONE collection point and lets the existing rollback revert to the prior instance/auth. A service whose auth genuinely cannot be rebuilt must refuse the reload for that service (fail closed), never apply the address change while silently keeping stale auth. Thomas: override if wrong.
- [ ] `internal/component/web/server.go` (Reconfigure `:262-333`, struct `:70`, one `*http.Server{Handler: mux}` `:86,130`, `serveOne` `:336`) - reconfigure adds/removes listeners on the shared `*http.Server`; the handler/auth chain is never rebuilt.
- [ ] `cmd/ze/hub/service_web.go` - `authWrap` (insecure → `InsecureMiddleware`; else `AuthMiddlewareWithAudit`) is selected ONCE and wrapped around the route handlers registered on the mux. This is the auth state a reload would have to rebuild.
- [ ] `internal/component/api/rest/server.go` (`Reconfigure` `:242-253`) - address-only, and already fail-closed: rejects any non-loopback new address before binding. gRPC mirrors it (`internal/component/api/grpc/server.go`).
- [ ] `internal/component/lg/server.go` (`Reconfigure` `:384-415`) and `cmd/ze/hub/service_mcp.go` (`Reconfigure` `:355-389`) - both address-only; no auth read.
- [ ] `cmd/ze/hub/main_reload.go` (`runReload` `:164-286`) - the reload driver: loads the tree, reloads plugin server + engine, then calls `lm.ReloadListeners(reloadCtx, parsedTree)` inside a 30s timeout; on migration error it invokes `rollbackReload` (`:245`, func at `:352-371`). This is where an auth-rebuild failure must surface and roll back. (Anchors re-verified 2026-07-23 after the origin/main fast-forward to 822029463, which grew this file.)

**Behavior to preserve:**
- AC-7's address guard from the source spec: a running unauthenticated listener must not migrate to a non-loopback address.
- Rollback on partial failure (`rollbackAppliedListeners`, `listener_migrate.go`).

**Behavior to change:**
- An auth-mode change in the reloaded config must take effect, rather than being silently ignored until restart.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- SIGHUP → config reload → `ReloadListeners(ctx, tree)` (`cmd/ze/hub/listener_migrate.go`)

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
- `Reconfigurable` implementations for web, lg, mcp, rest, grpc (`internal/component/web/server.go`, `internal/component/lg/server.go`, `cmd/ze/hub/service_mcp.go`, `internal/component/api/rest/server.go`, `internal/component/api/grpc/server.go`) - each would need to accept an auth-mode change (widened seam) or be drain-and-replaced by the migrator.
- `cmd/ze/hub/mgmt_guard.go` (parent-created) - the shared fail-closed classifier + `checkMgmtListeners`; the reload re-runs it over the rebuilt (address, auth) set so a rebuild cannot leave any listener non-loopback + unauthenticated. THIS IS THE HARD DEPENDENCY — the parent must land first.
- `cmd/ze/hub/listener_migrate.go` (`rollbackAppliedListeners`) and `cmd/ze/hub/main_reload.go` (`rollbackReload`) - rollback must be extended to cover the auth dimension, not just addresses, so a half-applied auth rebuild reverts fully (R-1).

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
| A-2 | The parent's shared classifier + `ReloadListeners` gate exist by the time this is implemented | `spec-fixit-mgmt-listener-auth-guard` (Status ready; creates `cmd/ze/hub/mgmt_guard.go`, gates `ReloadListeners` AC-7) | this child cannot re-run the guard; would have to duplicate the classifier (rejected) | parent spec on disk before implementation | pending parent |
| A-3 | A failed auth rebuild can be fully rolled back via the existing reverse-order reconfigure | `cmd/ze/hub/listener_migrate.go`, `main_reload.go` | a half-rebuilt server is left in an unknown auth state | AC-2 rollback test | unvalidated |

→ AUTONOMOUS DEFAULT (2026-07-17) on A-1: do NOT assume in-place rebuild is safe.
Default to DRAIN-AND-REPLACE (or refuse the per-service reload when replace is not
available), because in-place handler-chain mutation is unproven and can race in-flight
requests. FAIL CLOSED: if neither in-place nor replace can rebuild a service's auth,
that service's reload is refused and it keeps its prior (address, auth) — never the
new address with stale auth (`ai/rules/evidence.md`). Thomas: override if wrong.

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | An auth rebuild that fails halfway leaves a server unauthenticated | rollback test | extend `rollbackAppliedListeners` to cover auth, fail closed (`ai/rules/evidence.md`) |

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

| AC-5 | A config's `api-server` block enables ONE transport, and an edit changes what the API's authentication resolves to | The reload proceeds and rebuilds the running transport. A surface that was never built cannot refuse a migration |

AC-5 added 2026-08-08 from round 3 of the review, which found the reload refused
over a server that does not exist. The defect is real and it is fixed here.

→ Correction (2026-08-08, implementation): **the condition AC-5 was written with
does not reproduce, and the row above states the one that does.** AC-5 first read
"an `api-server` block with NO enabled `rest` or `grpc` transport". Read at the
producer, `ExtractAPIConfig` (`internal/component/config/loader_extract.go`) ends
with `if !cfg.RESTOn && !cfg.GRPCOn { return APIConfig{}, false }`, committed at
`e89c1ca55`. A block with a token and no enabled transport therefore answers
`ok=false`: `apiCfgOK` is false at boot so nothing is classified, and
`apiAuthReloader` returns `ok=false` on reload so nothing is resolved. No refusal
is reachable on that path.

**One transport enabled and the other absent is the shape that reproduces, and it
is the common one.** `cmd/ze/hub/main.go` marked BOTH `rest` and `grpc` from
`apiCfgOK` alone, where the `web` branch immediately above it was gated on
`webFactoryOn && webEnabled`. `registerAuthUpdater`
(`cmd/ze/hub/listener_migrate.go`) records an updater only from the `Set*`
methods, so a gRPC server never built has no entry; `runningAuth` still answered
`known` from `authAtBoot`, and `checkAuthRebuildable` refused the whole SIGHUP.
An operator who enables REST alone and removes the API token was told
`grpc cannot change its authentication while running` by a daemon running no
gRPC server. Reproduced at `cmd/ze/hub/mgmt_auth_reload_test.go`
(`TestReloadListenersProceedsWhenSiblingTransportWasNeverBuilt`, red with the
pre-fix marking) and end to end in
`test/reload/mgmt-guard-reload-unbuilt-transport.ci`.

The asymmetry with the `web` branch is the defect, and the fix is at the marking
site: `markMgmtAuth` (`cmd/ze/hub/mgmt_auth_reload.go`) classifies a surface only
when `ListenerMigrator.hasService` holds a handle for it, and it runs after every
management server is built. The scope is wider than the API, and deliberately so:
`mcp` was marked from `mcpFactoryOn` alone while `buildMCPService` skips an empty
address list, so it carried the same defect and no longer does.

AC-3/AC-4 added during design fill (2026-07-17, AUTONOMOUS DEFAULT) as the fail-closed
corollary of the child's charter: a reload must re-run the auth guard/classifier, and
auth state that cannot be rebuilt must never silently leave a listener unauthenticated
(`ai/rules/evidence.md`). Thomas: override if wrong.

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestReloadListenersAuthChange` | `cmd/ze/hub/listener_migrate_test.go` | an auth-mode change is applied on reload (AC-1) | |
| `TestReloadListenersRerunsGuardFailsClosed` | `cmd/ze/hub/listener_migrate_test.go` | a reload resulting in non-loopback + unauthenticated is refused via the parent classifier (AC-3) | |
| `TestReloadListenersAuthRebuildRollsBack` | `cmd/ze/hub/listener_migrate_test.go` | a mid-way auth-rebuild failure rolls back address+auth (AC-2), extending the existing `TestReloadListenersRollsBackAppliedServiceOnLaterFailure` pattern | |
| `TestReloadListenersRefusesUnbuildableAuth` | `cmd/ze/hub/listener_migrate_test.go` | a service whose auth cannot be rebuilt has its reload refused, address not applied with stale auth (AC-4) | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `mgmt-guard-reload-auth-rebuild` | `test/reload/mgmt-guard-reload-auth-rebuild.ci` | operator turns auth on and SIGHUPs; the listener demands auth afterward without a restart | WRITTEN, PASSES |
| `mgmt-guard-reload-refuses-unauth` | `test/reload/mgmt-guard-reload-refuses-unauth.ci` | operator's reload would leave a listener non-loopback + unauthenticated; reload is refused, daemon keeps prior auth+addrs | WRITTEN, PASSES |
| `mgmt-guard-reload-unbuilt-transport` | `test/reload/mgmt-guard-reload-unbuilt-transport.ci` | AC-5: a transport the daemon never built does not refuse the reload | WRITTEN, PASSES |

Round 3 of the review, 2026-08-08, established that NOTHING was added under
`test/reload/` by this work. The only file there,
`mgmt-guard-reload-refuses-nonloopback.ci`, is untouched and asserts the
address-migration refusal alone. So every user-facing behaviour this spec adds
was proven by Go unit tests only, and the rows above had been open since the TDD
plan was written. `ai/rules/testing.md` maps a config reload to a `.ci` in
`test/reload/`, so these are owed before closure, not optional. All three landed
2026-08-08.

→ The third test is named for the condition that reproduces (see the AC-5
correction above), not for the one AC-5 was first written with. A file called
`mgmt-guard-reload-api-server-without-transport` would name a path
`ExtractAPIConfig` cannot reach.

Each test is sequenced on the reload generation counter (`show reload-status`),
which advances on every processed reload including a rejecting one. An assertion
about a refused reload is evidence only once the reload has run, and this is the
fence `test/plugin/reload-listener-rejected.ci` already established for that.

### Discrimination Evidence (`ai/rules/interop-and-goal-validation.md`)

Each functional test was run against a tree with its own behavior reverted, and
each went red for its own reason. The revert was restored inside the same tool
call that ran it.

| Test | Revert applied | Observed red |
|------|----------------|--------------|
| `mgmt-guard-reload-auth-rebuild` | `applyAuthIntents` (`cmd/ze/hub/listener_migrate.go`) returns before `au.UpdateAuth` | `after SIGHUP: unauthenticated read status=200, want 401`, under a daemon that printed `sighup reload complete` -- the silent no-op this spec removes |
| `mgmt-guard-reload-refuses-unauth` | `runningAuth` (same file) drops its `authUpdaters` branch and answers from `authAtBoot` | `stderr does not contain "refusing to migrate rest to non-loopback listener"`. The daemon printed REST's own address rule instead: `reconfigure rest: REST listen address "0.0.0.0:4713" must be loopback`. That rule is REST's alone, so on gRPC nothing would have stopped the migration |
| `mgmt-guard-reload-unbuilt-transport` | `resolveAuthIntents` (`cmd/ze/hub/listener_migrate.go`) drops its `hasService` test, so a classified surface with no handle resolves an intent | `after SIGHUP: unauthenticated read status=401, want 200`, and the daemon printed `reload error: reload: listener migration: grpc cannot change its authentication while running`. Re-recorded in round 4: the earlier revert of this row, "markMgmtAuth classifies every surface, built or not", is now the SHIPPED behavior and no longer a revert |

| Test | Revert applied | Observed red |
|------|----------------|--------------|
| `TestMarkMgmtAuthClassifiesBeforeAnyHandleExists` | `markMgmtAuth` gated on `lm.hasService(name)` again, the round-3 shape | "Should be true ... web must be classified before its handle is installed" for all four surfaces, then "An error is expected but got nil ... an unauthenticated web server must not migrate to a public address". The second line IS the fail-open: with no record for web, `checkReloadExposure` takes its permissive branch and moves the listener to `0.0.0.0:3443` |
| `TestUnbuiltSurfaceResolvesNoAuthIntent`, `TestReloadListenersProceedsWhenSiblingTransportWasNeverBuilt` | `resolveAuthIntents` drops its `hasService` test | "Received unexpected error: resolve grpc authentication: no live config provider ... a reloader for a server that was never built must not run", and "grpc cannot change its authentication while running ... a transport that was never built cannot refuse a reload". Two failures, one for the reloader being CALLED and one for the refusal it produces |

The unit test at the install seam,
`TestApplyAuthIntentsInstallsAndRestoresCredentials`
(`cmd/ze/hub/listener_migrate_test.go`), pins the first revert directly. The
`.ci` proves the test detects the ORIGINAL defect; the unit test makes the suite
detect the fix being undone later, without a daemon.

### Future (if deferring any tests)
- None yet; this is a skeleton.

## Files to Modify
- `cmd/ze/hub/listener_migrate.go` - widen the `Reconfigurable` seam / `serviceChange` to carry the resolved (address, authenticated) intent; re-run the parent classifier over the rebuilt set in `ReloadListeners` (`:94+`); extend `rollbackAppliedListeners` to revert auth as well as addresses
- `cmd/ze/hub/main_reload.go` - surface auth-rebuild failures through `runReload` and its `rollbackReload`
- `internal/component/web/server.go` - support an auth rebuild (drain-and-replace the handler chain, or a new auth-swap method) since auth is baked into one `*http.Server` today (`:86,130`)
- `cmd/ze/hub/service_web.go` - make the `authWrap` decision reproducible on reload from the reloaded config, not fixed at first construction
- `internal/component/lg/server.go`, `cmd/ze/hub/service_mcp.go`, `internal/component/api/rest/server.go`, `internal/component/api/grpc/server.go` - each `Reconfigure` extended (or its instance replaced) to apply the reloaded auth decision; REST/gRPC keep their existing loopback fail-closed guard (`rest/server.go`)

**Consumes (parent, must exist first):** `cmd/ze/hub/mgmt_guard.go` - the shared fail-closed classifier + `checkMgmtListeners`, created by `spec-fixit-mgmt-listener-auth-guard`. This child reuses it; it does not create a second classifier.

## Implementation Steps

### Implementation Phases
0. **Phase: Dependency gate (BLOCKING)** — confirm the parent's `cmd/ze/hub/mgmt_guard.go` classifier + `checkMgmtListeners` and the AC-7 `ReloadListeners` gate are on disk. If absent, STOP: this child cannot be implemented without duplicating the parent's single classifier (see Metadata READINESS note).
1. **Phase: Wiring (MANDATORY FIRST)** — a failing test (`mgmt-guard-reload-auth-rebuild.ci` + `TestReloadListenersAuthChange`) proving an auth change on reload is currently ignored (server keeps its constructed auth).
2. **Phase: Widen the seam** — extend `serviceChange`/`Reconfigurable` (or add a drain-and-replace path) to carry the resolved (address, authenticated) intent; rebuild web auth from the reloaded config (`service_web.go` reproduced on reload). Tests: `TestReloadListenersAuthChange` passes.
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

## Deliverables Checklist

<!-- ADDED AT CLOSURE. The spec was written before the template carried these
     three tables, so /ze-close filled them from what shipped rather than
     closing without the reviews they gate. -->

| Deliverable | Verification method |
|-------------|---------------------|
| The widened seam exists and is one interface, not a per-service switch | `gopls symbols cmd/ze/hub/listener_migrate.go` shows `AuthUpdatable` (`:57`) with `Authenticated` and `UpdateAuth`, and no second classifier |
| Both API transports implement it | `grep -rn 'func.*UpdateAuth' internal/component/api` returns exactly `rest/auth.go:75` and `grpc/server.go:534` |
| The reload drives it | `ReloadListeners` (`:329`) calls `resolveAuthIntents`, `checkAuthRebuildable`, `applyAuthIntents`, `checkReloadExposure` in that order |
| The undo reaches every failure path | `cmd/ze/hub/main_reload.go:298-304`: a `reloadApplied` flag and one `defer`, not a call per path |
| Functional tests exist | `ls test/reload/mgmt-guard-reload-*.ci` returns three files beside the parent's |
| Unit tests green | `make ze-test-pkg PKG=./cmd/ze/hub` |

## Security Review Checklist

<!-- ADDED AT CLOSURE, same reason. This spec is a security spec: every row
     below is a way the reload could end less authenticated than it began. -->

| Check | What to look for | Answer |
|-------|-----------------|--------|
| A guard failing open on an unknown service | `checkReloadExposure` SKIPS an unknown service, which is its permissive branch | Bounded: `markMgmtAuth` runs at `cmd/ze/hub/main.go:1087`, before `registerBuiltService` (`:1168`), `lm.SetREST` (`:1224`) and `lm.SetGRPC` (`:1238`), so no classified surface can be asked about before its record lands. Round 4 found and closed this window |
| An unresolved authentication mode read as "no authentication" | `resolveAuthIntents` error branch | Fails closed: it returns `resolve %s authentication: %w` and the reload stops |
| A rollback that lowers authentication | `(*GRPCServer).UpdateAuth`'s undo | `setAuthLocked` re-runs `checkGRPCListenAddr` on the restore too, and the undo KEEPS the reloaded credentials with an Error log when putting the old ones back would expose the listener |
| A failed reload leaving reloaded credentials in place | `runReload` | The deferred `restoreAuth()` runs on every non-applied path, the `PromoteCandidate` failure included |
| Transport secrecy read as authentication | `checkReloadExposure` doc comment | Stated explicitly: it judges authentication only, gRPC owns its TLS rule in `checkGRPCListenAddr`, and a pass here is not "safe to expose" |
| Credential material in a log or error | `checkAuthRebuildable`'s message | Names the service and the two MODES (`authWord`), never a token or a hash |
| A partially applied intent set | `applyAuthIntents` | `restoreAll()` runs in reverse before the error returns |

## Documentation Update Checklist

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/guide/authentication.md`, "Authentication on reload". Already landed with the implementation: it states what a reload rebuilds, quotes the refusal message, and names the unbuilt-transport case |
| 2 | Config syntax changed? | No | Same leaves; the reload reads what was already there |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | `UpdateAuth` is an internal Go seam, not an RPC |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | Yes | Same page as #1 |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior changed? | N-A | No RFC governs the reload of a management listener |
| 10 | Test infrastructure changed? | No | Three new `.ci` files in the existing `test/reload/` form |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Yes | `docs/architecture/web-interface.md` describes the `Reconfigurable` seam. `AuthUpdatable` is declared beside it and documented at its declaration; no architecture claim on that page became false, because the address seam is unchanged |
| 13 | Route metadata keys? | No | - |
| 14 | Prometheus counters? | No | The refusal reaches the operator's exit status, not a counter |
| 15 | Registered plugin/command/capability? | No | - |
| 16 | Changed file referenced by doc source anchors? | Yes | `docs/guide/authentication.md:247` anchors `cmd/ze/hub/mgmt_auth_reload.go -- markMgmtAuth, registerMgmtAuthReloaders`; both symbols present at HEAD |
| 17 | Existing docs show examples for this area? | Yes | The refusal message in the guide matches `checkAuthRebuildable`'s format string verbatim |

---

## Implementation Summary

### What Was Implemented

- `AuthUpdatable` (`cmd/ze/hub/listener_migrate.go:57`), a narrow capability
  interface beside `Reconfigurable`, twinned on `TLSUpdatable`. Drain-and-replace
  was rejected: the web server bakes its auth decision into ~30 mux
  registrations, and replacing the instance would replace the broker and the
  shutdown handle with it.
- `(*RESTServer).UpdateAuth` (`internal/component/api/rest/auth.go:75`) and
  `(*GRPCServer).UpdateAuth` (`internal/component/api/grpc/server.go:534`), each
  returning a restore closure.
- Four steps in `ReloadListeners` (`:329`), in a fixed order: resolve, refuse the
  unrebuildable, apply, re-check exposure. Every refusal happens before a
  listener moves.
- `authAtBoot` REPLACES the boot `unauth` name set, recording both polarities;
  `runningAuth` prefers the live server, because a rebuilt server's boot record
  is the stale answer this work removes.
- `markMgmtAuth` (`cmd/ze/hub/mgmt_auth_reload.go:80`) classifies every surface
  at boot, before any handle reaches the migrator; `resolveAuthIntents` drops the
  surfaces with no handle.
- The undo in `runReload` (`cmd/ze/hub/main_reload.go:298-304`) is a deferred
  flag, so a new failure path inherits it.

### Bugs Found/Fixed

- Round 3 gated `markMgmtAuth` on `hasService`, which opened a fail-open window:
  handles are installed before the mark, so `checkReloadExposure` skipped web and
  mcp and would have migrated an unauthenticated web server to `0.0.0.0:3443`.
  Round 4 moved the handle test to reload time.
- The first implementation logged and dropped an unrebuildable service's change
  and returned nil, so `ze config commit` reported success over a web server
  still serving unauthenticated. `checkAuthRebuildable` now fails the reload.
- The undo was hand-placed and the `PromoteCandidate` path had already been
  written without it.
- gRPC's undo could restore empty credentials onto a listener the same reload had
  moved off loopback. `setAuthLocked` re-runs `checkGRPCListenAddr` on the undo.
- AC-5's original condition does not reproduce: `ExtractAPIConfig` returns
  `ok=false` for a block with no enabled transport. ONE transport enabled and the
  sibling absent is the shape that reproduces, and it was common.

### Documentation Updates

- `docs/guide/authentication.md`, "Authentication on reload": landed with the
  implementation. Verified at closure against the producers: the quoted refusal
  matches `checkAuthRebuildable`'s format string, the unbuilt-transport paragraph
  matches `resolveAuthIntents`'s `hasService` skip, and the anchor at `:247`
  names `markMgmtAuth` and `registerMgmtAuthReloaders`, both present.
- No further doc edit was owed. `make ze-doc-test` NOT run in this session.

### Deviations from Plan

- The seam was WIDENED, not drain-and-replace, which the spec's own Autonomous
  Default of 2026-07-17 had preferred. The reason is recorded in-body: no service
  is constructed in a way that survives replacement.
- `internal/component/web/server.go`, `internal/component/lg/server.go` and
  `cmd/ze/hub/service_mcp.go` were NOT modified. Web and MCP implement no
  `AuthUpdatable`, so a mode change on either fails the reload with a message
  naming the service, which is AC-4 rather than a gap.
- AC-5 was added mid-flight from review round 3.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | An unrebuildable service's change was logged and dropped, and the reload returned nil | `runReload` then promoted the candidate, so the operator was told a reload succeeded over an unauthenticated server | Review, 2026-08-07 | `checkAuthRebuildable` fails the reload before anything applies |
| approach | The handle test was placed at MARK time | Handles reach the migrator before the mark, so the window was fail-open for web and mcp | Review round 4 | Moved to `resolveAuthIntents`; `TestMarkMgmtAuthClassifiesOnlyBuiltSurfaces` DELETED, not left beside the new shape |
| assumption | "Refusing a reload would lock out every later SIGHUP" | Inferred, never read. `PromoteCandidate` runs only after every step succeeds and the refusal paths clear the candidate | Reading `runReload` | Claim withdrawn in-body, with the one real divergence (`PromoteCandidate` failure clears nothing) named as separate |

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| An auth-mode change takes effect on SIGHUP | Done | `applyAuthIntents` (`listener_migrate.go:465`) -> `UpdateAuth` | REST and gRPC |
| A service that cannot rebuild says so instead of lying | Done | `checkAuthRebuildable` (`:440`) | Web and MCP |
| The reload cannot end more exposed than it began | Done | `checkReloadExposure` (`:526`) over LIVE auth | Plus gRPC's own address rule |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `applyAuthIntents` calls `au.UpdateAuth(ra.intent.token, ra.intent.authenticator)` unconditionally per intent (`:483`) | Applied even when the MODE is unchanged, because a rotated token changes the material |
| AC-2 | Done | `UpdateAuth` returns a restore closure; `applyAuthIntents` composes them in reverse (`:466-471`); `runReload` runs the composite from a deferred `reloadApplied` flag (`main_reload.go:298-304`) | |
| AC-3 | Done | `checkReloadExposure` reads `runningAuth`, which prefers the live `authUpdaters` entry over `authAtBoot` (`:205-209`) | Refuses before `migrateListeners` and restores the credentials first (`:346-349`) |
| AC-4 | Done | `checkAuthRebuildable` returns before any apply (`ReloadListeners:337`) | The message names the service and both modes |
| AC-5 | Done | `resolveAuthIntents` skips `!m.hasService(name)` (`:399`) BEFORE calling the reloader | Skipping the check alone would still have called `apiAuthReloader` for a server that does not exist |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| AC-1 unit | Done | `TestReloadListenersRebuildsAuthenticationOn` (`cmd/ze/hub/listener_migrate_test.go:135`) | Named differently from the TDD plan's placeholder |
| AC-2 unit | Done | `TestReloadListenersRestoresAuthenticationWhenMigrationFails` (`:173`) | |
| AC-3 unit | Done | `TestReloadListenersRefusesRebuiltUnauthenticatedNonLoopback` (`:201`) | |
| AC-4 unit | Done | `TestReloadListenersFailsWhenAuthCannotBeRebuilt` (`:226`) | |
| AC-5 unit | Done | `TestUnbuiltSurfaceResolvesNoAuthIntent`, `TestReloadListenersProceedsWhenSiblingTransportWasNeverBuilt` (`cmd/ze/hub/mgmt_auth_reload_test.go:224,252`) | |
| Fail-open window | Done | `TestMarkMgmtAuthClassifiesBeforeAnyHandleExists` (`:195`) | Replaces the DELETED `TestMarkMgmtAuthClassifiesOnlyBuiltSurfaces` |
| Install seam | Done | `TestApplyAuthIntentsInstallsAndRestoresCredentials` (`listener_migrate_test.go:462`) | |
| Server-side | Done | `internal/component/api/rest/auth_test.go`, `internal/component/api/grpc/auth_reload_test.go` | The gRPC pair drives a real RPC |
| `mgmt-guard-reload-auth-rebuild` | Done | `test/reload/mgmt-guard-reload-auth-rebuild.ci` | |
| `mgmt-guard-reload-refuses-unauth` | Done | `test/reload/mgmt-guard-reload-refuses-unauth.ci` | |
| `mgmt-guard-reload-unbuilt-transport` | Done | `test/reload/mgmt-guard-reload-unbuilt-transport.ci` | |

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| `cmd/ze/hub/listener_migrate.go` | Done | The seam, the four steps, `authAtBoot`, `runningAuth` |
| `cmd/ze/hub/main_reload.go` | Done | The deferred undo |
| `internal/component/api/rest/server.go` (auth.go) | Done | `UpdateAuth` landed in `auth.go` beside the middleware, not in `server.go` |
| `internal/component/api/grpc/server.go` | Done | `UpdateAuth` plus the `setAuthLocked` address rule |
| `internal/component/web/server.go`, `internal/component/lg/server.go`, `cmd/ze/hub/service_mcp.go` | Changed | Deliberately NOT modified; see Deviations |
| `cmd/ze/hub/service_web.go` | Changed | Same reason: web's auth stays a construction decision, and AC-4 covers the change |
| `cmd/ze/hub/mgmt_auth_reload.go` | Added, not in the plan | `markMgmtAuth`, `registerMgmtAuthReloaders`, `apiAuthReloader` |

### Audit Summary

- **Total items:** 5 AC + 3 task requirements + 11 test rows
- **Done:** 17
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 2 file rows (web/lg/mcp untouched, `mgmt_auth_reload.go` added), both recorded in Deviations

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| An authentication-mode change takes effect on SIGHUP, not at restart | functional | `test/reload/mgmt-guard-reload-auth-rebuild.ci`. Discrimination recorded in the spec: with `applyAuthIntents` returning before `au.UpdateAuth`, the test prints `after SIGHUP: unauthenticated read status=200, want 401` under a daemon that logged `sighup reload complete`, which IS the silent no-op |
| A reload cannot end non-loopback and unauthenticated | functional | `test/reload/mgmt-guard-reload-refuses-unauth.ci`. With `runningAuth` answering from `authAtBoot`, the guard's own message disappears and only REST's private address rule fires, which would not have covered gRPC |
| A server that was never built cannot refuse a reload | functional | `test/reload/mgmt-guard-reload-unbuilt-transport.ci`. With the `hasService` test dropped, the daemon prints `reload error: reload: listener migration: grpc cannot change its authentication while running` for a gRPC server it never started |
| A failed reload leaves nothing authenticating against the rejected config | unit | `TestReloadListenersRestoresAuthenticationWhenMigrationFails`, `TestRESTUpdateAuthRestoreRevertsCredentials`, `TestGRPCUpdateAuthRestoreRevertsCredentials` |
| The fail-open window at boot is closed | unit | `TestMarkMgmtAuthClassifiesBeforeAnyHandleExists` drives it directly: mark, install an unauthenticated web handle, run the exposure check |

## Deferrals Resolved

This spec has no deferral shard of its own: `ls plan/deferrals/` holds no
`fixit-mgmt-listener-auth-guard-deferred-reload-auth-rebuild.md`.

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| (own shard) none | N-A | No shard exists for this spec |
| FOREIGN row homed here: "Rebuild the live AAA bundle on config reload" (`plan/deferrals/fixit-web-auth-deleted-user-survives-reload.md`) | re-homed, still deferred | This spec built the SEAM and not this work. `swapAAABundle` (`cmd/ze/hub/aaa_lifecycle.go`) still has exactly two non-test callers, `infraSetup` and `runYANGConfig`, both startup-only, re-verified 2026-08-12. The row now says the seam exists and the rebuilder does not, and needs its own spec. The shard keeps two live rows, so it is not removed |
| gNMI token over plaintext | out of scope, verified real | Recorded in-body under "Not claimed, and now verified real". `(*gnmi.Server).Serve` adds the token interceptor and the TLS credentials from two independent conditions. It is a secrecy question, not an identity one, and the 2026-07-17 autonomous default keeps it out |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fixit-mgmt-listener-auth-guard-deferred-reload-auth-rebuild-640fa955-f03a-45e8-a58f-4b367f5859e6.md` |
| `review_gate.py check` | clean |
| Rounds | 1 at closure. Four review rounds ran during implementation and are recorded in Findings fixed |
| Reviewer lenses used | AC-to-producer verification at HEAD, fail-open audit of every guard branch, boot-ordering audit, citation audit |

This closure reviewed code ALREADY AT HEAD. `git status --porcelain` over
`cmd/ze/hub`, `internal/component/api`, `internal/component/web`,
`internal/component/lg` and `test/reload` reported no change to any path this
spec names before the review began.

### Findings fixed

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | An unrebuildable service's change was dropped and the reload reported success | `cmd/ze/hub/listener_migrate.go` | `checkAuthRebuildable` fails the reload before any apply |
| 2 | BLOCKER | Handles reach the migrator before `markMgmtAuth`, so the exposure guard took its permissive branch for web and mcp | `cmd/ze/hub/main.go`, `listener_migrate.go` | Mark before any handle; drop unhandled surfaces at reload time |
| 3 | BLOCKER | A refused reload left REST and gRPC authenticating against the rolled-back config | `cmd/ze/hub/main_reload.go` | A deferred `reloadApplied` flag replaces hand-placed undo calls |
| 4 | BLOCKER | gRPC's undo could restore empty credentials onto a listener moved off loopback in the same reload | `internal/component/api/grpc/server.go` | `setAuthLocked` re-runs `checkGRPCListenAddr` on the undo and keeps the reloaded credentials |
| 5 | ISSUE | The reload refused over a gRPC server the daemon never built | `cmd/ze/hub/listener_migrate.go` | AC-5, the `hasService` skip in `resolveAuthIntents` |
| 6 | ISSUE | No `.ci` covered any behaviour this spec adds | `test/reload/` | Three functional tests, each with a recorded revert |
| 7 | NOTE | The spec's destination for the foreign AAA-bundle deferral row became dangling at closure | `plan/deferrals/fixit-web-auth-deleted-user-survives-reload.md` | Re-homed with the re-verified `swapAAABundle` caller set |
| 8 | NOTE | The spec carried no Deliverables, Security or Documentation checklist | this spec | All three added and filled above, from what shipped |

## Pre-Commit Verification

### Files Exist (ls)

| File | Exists | Evidence |
|------|--------|----------|
| `test/reload/mgmt-guard-reload-auth-rebuild.ci` | Yes | `ls test/reload/` listed it |
| `test/reload/mgmt-guard-reload-refuses-unauth.ci` | Yes | same |
| `test/reload/mgmt-guard-reload-unbuilt-transport.ci` | Yes | same |
| `cmd/ze/hub/mgmt_auth_reload.go` | Yes | `gopls`/grep resolved `markMgmtAuth:80`, `registerMgmtAuthReloaders:98`, `apiAuthReloader:146` |
| `internal/component/api/rest/auth.go` | Yes | `UpdateAuth` at `:75` |
| `internal/component/api/grpc/auth_reload_test.go` | Yes | `grep -rln` returned it |

### AC Verified (grep/test)

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | The reload installs the reloaded credentials | `listener_migrate.go:483` `restore, err := au.UpdateAuth(ra.intent.token, ra.intent.authenticator)`, reached from `ReloadListeners:341` |
| AC-2 | Every failure path undoes them | `main_reload.go:298-304`: `reloadApplied := false` and a single `defer` calling `restoreAuth()` |
| AC-3 | The exposure check reads LIVE auth | `listener_migrate.go:205-209`: `runningAuth` returns `au.Authenticated()` when an updater exists, and only then falls back to `authAtBoot` |
| AC-4 | The refusal comes before any apply | `ReloadListeners:337` calls `checkAuthRebuildable` before `applyAuthIntents` at `:341` and `migrateListeners` at `:352` |
| AC-5 | An unbuilt surface produces no intent and its reloader is never called | `listener_migrate.go:399-401`: `if !m.hasService(name) { continue }`, above the `m.authReloaders[name](tree)` call at `:405` |
| Boot ordering | The mark precedes every handle | `cmd/ze/hub/main.go`: `markMgmtAuth` at `:1087`, `registerBuiltService` at `:1168`, `lm.SetREST` at `:1224`, `lm.SetGRPC` at `:1238` |
| All | The package is green | `make ze-test-pkg PKG=./cmd/ze/hub`: `ok github.com/ze-software/ze/cmd/ze/hub 106.382s` |

### Wiring Verified (end-to-end)

| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| SIGHUP with auth turned ON | `test/reload/mgmt-guard-reload-auth-rebuild.ci` | Yes: header line 2 names this spec, and the spec records the revert that makes it red |
| SIGHUP leaving a listener non-loopback and unauthenticated | `test/reload/mgmt-guard-reload-refuses-unauth.ci` | Yes: same header, and its recorded red is the guard's own message disappearing |
| A transport never built | `test/reload/mgmt-guard-reload-unbuilt-transport.ci` | Yes: same header; its red is a refusal over a server that does not exist |
| Auth rebuild fails midway | none (unit) | Yes: `TestReloadListenersRestoresAuthenticationWhenMigrationFails` (`listener_migrate_test.go:173`) |

### Assumptions Resolved

| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | broken | In-place rebuild was assumed unsafe and drain-and-replace preferred. Neither was taken: `UpdateAuth` swaps credentials under the server's own mutex without touching a listener or a handler chain, so no connection is drained. Recorded in Deviations and the Mistake Log |
| A-2 | confirmed | `cmd/ze/hub/mgmt_guard.go` and `checkMgmtListeners` are on disk; this spec reuses that classifier and creates no second one |
| A-3 | confirmed | Rollback is NOT the reverse-order reconfigure the assumption named. It is a composed restore closure plus a deferred flag in `runReload`, which covers the promote path the reverse-order form had missed |

### Documentation Verified

| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| "A config reload rebuilds the credentials of the running REST and gRPC servers" | `applyAuthIntents` calls `UpdateAuth` on every intent; only REST and gRPC implement `AuthUpdatable` | Yes |
| The quoted refusal message | `checkAuthRebuildable`'s format string at `listener_migrate.go:449` matches the guide's block word for word | Yes |
| "An `api-server` block that enables REST alone reloads on the REST server, and says nothing about gRPC" | `resolveAuthIntents:399` | Yes |
| "it reads the rebuilt authentication rather than the mode the server started with" | `runningAuth:205-209` | Yes |
| Anchor `mgmt_auth_reload.go -- markMgmtAuth, registerMgmtAuthReloaders` | both symbols present at `:80` and `:98` | Yes, no drift |

## Gates Not Run in This Closure Session

`make ze-verify`, `ze-verify-changed`, `ze-doc-test`, and every `.ci` suite
including `test/reload/`. The operator scoped this session to review and closure
of code already at HEAD and forbade running a test suite. The one gate run is
`make ze-test-pkg PKG=./cmd/ze/hub`, green. No Go file changes in this closure,
so `make ze-tracked-build-check` is not owed.

## Core Insight

The seam was widened rather than replaced, and the reason generalises: a
capability interface declared beside `Reconfigurable` lets a service opt IN to
being reconfigured along one axis, and a service that does not implement it is
visibly unable rather than silently stale. That is what turns "the reload did
nothing" into "the reload refused and named the service". The fail-open branch
that survived three review rounds was the mirror image: a service the guard
could not classify was skipped, and skipping is permissive. Both halves of this
spec are the same rule, applied to a capability and to a classification.
