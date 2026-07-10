# Spec: startup-resilience

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 2/5 (fixes; audit done) |
| Updated | 2026-07-10 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (Classification Table + Implementation Phases are the design)
2. `.claude/rules/planning.md`
3. `internal/component/radius/client.go` - the verified-good reference pattern (lazy resolve per exchange)
4. `internal/plugins/ntp/ntp.go` + `internal/plugins/ntp/register.go` - FIX 1 (reload block)
5. `internal/component/l2tp/plugins/authradius/register.go` - FIX 2 (unbounded DNS on apply)
6. `internal/component/managed/client.go` - health-signal phase (hub connection state)

## Task

**AUDIT COMPLETE (2026-07-10). All eight touchpoints classified with producer
evidence; see the Classification Table under Current Behavior.**

Invariant to establish and enforce: **daemon startup and config apply must never
block on, or fail because of, an unreachable external service.** An appliance
must boot to a functional state (CLI reachable, config committed, local
forwarding up) even when its RADIUS/TACACS servers, RPKI validators, BMP
collectors, or management hub are down, and must converge when they return.

Reference: osvbng 424e2d0/2259ce6 fixed exactly this class (an unreachable
RADIUS server blocked daemon startup; BGP re-apply panicked; VRF table re-apply
was non-idempotent) as one "startup and config re-apply resilience" effort.

Audit outcome (details in Classification Table): the invariant ALREADY HOLDS for
daemon startup. All six externally-connecting subsystems either dial lazily per
request (RADIUS, TACACS) or dial inside detached background goroutines with
bounded timeouts and retry loops (RPKI, BMP, managed hub, NTP), and the managed
client is only spawned AFTER `WaitForStartupComplete` and the ready file
(`cmd/ze/hub/main.go:887-911`). Two config-APPLY weaknesses remain:

1. **NTP reload block (real, reachable today):** a config apply that touches the
   `environment` root while NTP servers are unreachable synchronously waits out
   the in-flight sync (`startWorker` -> `stopAndWait`,
   `internal/plugins/ntp/register.go:113` -> `internal/plugins/ntp/ntp.go:90-93`).
   With N dead servers the wait is up to N x 5s (serial `ntp.Query`,
   `internal/plugins/ntp/ntp.go:154-188`; beevik/ntp v1.5.0 default timeout 5s,
   module `ntp.go:67,482-483`), and the 1s initial-sync retry loop
   (`internal/plugins/ntp/ntp.go:113-124`) keeps a sync in flight essentially
   always while servers are dark. NTP declares `ApplyBudget: 5`
   (`internal/plugins/ntp/register.go:184`); the transaction coordinator aborts
   the whole apply on deadline
   (`internal/component/config/transaction/orchestrator.go:423-424`). So enough
   dead NTP servers can FAIL an unrelated config commit.
2. **L2TP authradius unbounded DNS on apply (latent):** `serverIPs`
   (`internal/component/l2tp/plugins/authradius/register.go:225-245`) calls
   `resolver.LookupIPAddr(context.Background(), host)` (register.go:235-236)
   with NO deadline, inside `activateRadiusConfig` which runs on OnConfigure and
   OnConfigApply (register.go:127, :139). Latent because the gating `coa-port`
   knob is parsed (`config.go:93-100`) but has no YANG leaf (grep of
   `internal/**/*.yang` finds none) and the config parser rejects unknown fields
   (`internal/component/config/parser.go:372-380`), so production configs cannot
   reach it today. The moment a coa-port leaf lands, a hostname server address
   plus a dead resolver blocks apply (plugin declares `ApplyBudget: 1`,
   register.go:156).

The work: fix both apply-path weaknesses, add the one missing health surface
(managed hub is log-only today), and pin the boot invariant with a functional
test using blackholed service addresses.

Audit surface (all rows audited; classification below):

| Touchpoint | Where audited | Verdict |
|-----------|----------------|---------|
| TACACS+ | `internal/component/tacacs/` | lazy |
| RPKI (RTR) | `internal/component/bgp/plugins/rpki/` | lazy |
| BMP collectors | `internal/component/bgp/plugins/bmp/` | lazy |
| Managed-node client | `internal/component/managed/` | lazy (health gap) |
| NTP | `internal/plugins/ntp/` | lazy at boot, blocks reload apply (FIX) |
| RADIUS (component) | `internal/component/radius/client.go` | lazy (reference pattern) |
| L2TP authradius plugin | `internal/component/l2tp/plugins/authradius/` | latent eager-blocking on apply (FIX) |
| DNS in config apply | full-tree sweep | only the authradius hit; everything else off-path |

Scope boundary: osvbng's sibling theme (idempotent re-apply of dataplane objects
on restart, their bond/subinterface/VRF-table fixes) is related but distinct;
this spec is reachability-only (see R-1). The audit did not hunt for idempotency
gaps and none were incidentally observed.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - component/engine startup ordering.
  → Constraint: plugins receive config in Stage 2 (OnConfigure) of the 5-stage startup protocol; anything spawned there must detach before returning.
  → Constraint: readiness (`ze.ready.file`) is signaled by `WaitForStartupComplete` in `cmd/ze/hub/main.go:887-904`; the managed client starts after it (main.go:911), so hub reachability is off the readiness path.
- [ ] `ai/rules/doctor-checks.md` - unreachable-service detection belongs in doctor/health, not in startup failures.
  → Constraint: components that are not plugins register checks via `diagnostic.RegisterDoctorCheck` from their register.go (radius pattern, `internal/component/radius/doctor.go:1-33`); plugins use `registry.DoctorCheckDef` (authradius pattern, `internal/component/l2tp/plugins/authradius/register.go:63-71`). New checks need a code in `internal/core/diagnostic/codes.go`, a unit test, and a functional test.
- [ ] `ai/rules/qemu-testing.md` - boot-with-blackholed-services evidence is Linux/QEMU territory.
  → Constraint: the .ci functional suite runs the daemon for real; RFC 5737 addresses (203.0.113.0/24) give drop (timeout) semantics without firewall setup, and 127.0.0.1:closed-port gives reject semantics. Use drop for the invariant test so timeouts are exercised.

**Key insights:**
- The invariant is testable cheaply: point every external-service address at a
  blackhole (drop, not reject, so timeouts are exercised) and assert the daemon
  reaches ready + commits config within its normal budget.
- The apply deadline is real and enforced: `runApply` aborts the transaction
  when the computed tiered deadline expires
  (`internal/component/config/transaction/orchestrator.go:401-427`; budgets
  capped, 30s default when zero, orchestrator.go:524-566). A plugin blocking in
  OnConfigApply is therefore a user-visible commit failure, not just latency.

## Current Behavior (MANDATORY)

**Source files read:** (all verified 2026-07-10; annotations are the audit evidence)
- [ ] `internal/component/radius/client.go` - REFERENCE PATTERN. `NewClient` (58-88) only binds a local UDP socket (`net.ListenUDP`, :74); `Client.Exchange` (:116) resolves the server per call (`net.ResolveUDPAddr`, :134) and retries with exponential backoff bounded by Timeout x Retries (:145-192; defaults 3s/3, :63-68). `SendToServers` fails over across servers (:302-316). No startup/apply-time network activity.
  → Constraint: this is the target shape for every touchpoint: first network syscall on the request path or in a detached loop, never in a constructor or apply callback.
- [ ] `internal/component/radius/doctor.go` - existing health surface: `radius-admin-unreachable` doctor check probes servers with Timeout+Retries:1 (:42-93, Exchange at :93), registered per ai/rules/doctor-checks.md.
- [ ] `internal/component/tacacs/client.go` - `NewTacacsClient` (88-103) does no I/O (bufpool only). First dial is `TacacsClient.dial` (488-494, `net.Dialer{Timeout: c.config.Timeout}` :489, default 5s :93-95), reached only through Authenticate/SendAuthorization/SendAccounting -> `sendToServers` (208-235) -> `sendReceive`/`trySend` (257-399). Per-exchange deadline on the conn (:304). Lazy.
- [ ] `internal/component/tacacs/register.go` - AAA backend `Build` (24-71) constructs the client and bridges; no network on the apply path. `Close` drains pooled conns on bundle swap (62-68).
- [ ] `internal/component/tacacs/config.go` - timeout from YANG, default 5s (78-83); addresses via `net.JoinHostPort` (69-71).
- [ ] `internal/component/aaa/types.go` - chain semantics: explicit rejection stops; connection error tries the next backend (103-112), so dead TACACS/RADIUS degrades to local auth after bounded timeouts (pinned by `test/plugin/aaa-radius-fallback.ci`).
- [ ] `internal/component/bgp/plugins/rpki/rpki.go` - `OnConfigure` (189-202) parses config and calls `startSessions` (256-282), which spawns one detached goroutine per cache server (`rp.sessionWg.Go(session.Run)`, :275) and returns immediately. Lazy.
- [ ] `internal/component/bgp/plugins/rpki/rtr_session.go` - `RTRSession.Run` (88-108) is the retry loop (retryInterval default 600s, :77); `connectAndSync` (121-176) dials with a 30s bound (:123, DialContext :143) and a stop-cancel watcher (133-144). Health surface: `show bgp rpki status`/`cache` (rpki.go:876, :914) expose per-session state.
- [ ] `internal/component/bgp/plugins/bmp/bmp.go` - `OnConfigure` (212-240) -> `startSender` (336-346) spawns `ss.run` per collector (:343) and returns. Receiver listeners are local binds (296-320). Lazy.
- [ ] `internal/component/bgp/plugins/bmp/sender.go` - `run` (80-130) dials with 10s bound (:91, DialContext :97) and reconnect backoff 30s..720s (:25-29, :103-107). Event-path `writeMsg` returns `errNotConnected` instead of blocking when the collector is down (:191-201). Health surface: `show bmp collectors` (cmd_show.go:16).
- [ ] `internal/component/managed/client.go` - `RunManagedClient` (55-84) is a reconnect loop (backoff 1s..60s jittered, :56); `runConnection` dials + TLS-handshakes under a 5s context (`connectTimeout` :33, WithTimeout :113, DialContext :120, HandshakeContext :126). Health: LOG-ONLY today ("connected to hub" :162, "connection lost, reconnecting" :71-73); no doctor/status surface found in `internal/component/managed/` or the hub CLI (grep).
- [ ] `cmd/ze/hub/main.go` - startup ordering: `WaitForStartupComplete` (887-898) then ready file (900-904) then `go managed.RunManagedClient(...)` (911). An unreachable hub cannot delay readiness.
- [ ] `internal/plugins/ntp/ntp.go` - worker is detached (`start` spawns `run`, 85-87). `run` phase 2 retries initial sync every 1s (113-124); `doSync` (138-249) queries all servers SERIALLY with no stop-check inside the loop (154-188; `ntp.Query` :158, library default timeout 5s: beevik/ntp v1.5.0 `ntp.go:67`, applied :482-483). `stopAndWait` (90-93) blocks until the in-flight `doSync` finishes.
- [ ] `internal/plugins/ntp/register.go` - `startWorker` (111-134) calls `stopAndWait` (:113) BEFORE starting the replacement worker; it runs inside `OnConfigure` (139-152) and `OnConfigApply` (169-177). Declared `ApplyBudget: 5` (:184). First boot is safe (worker==nil, no wait); reload with dead servers blocks up to N x 5s. Health surface: `show system ntp` / `show system ntp peers` (226-285) expose synced flag, per-server reach bitmask and last-error.
- [ ] `internal/component/config/transaction/orchestrator.go` - the enforcement: `runApply` (374-431) aborts the transaction with "apply timeout" at the computed deadline (:423-424); deadline = sum of per-tier max budgets, capped, 30s default when zero (508-571).
- [ ] `internal/component/l2tp/plugins/authradius/register.go` - `activateRadiusConfig` (168-212) runs on OnConfigure (:127) and OnConfigApply (:139). `radius.NewClient` (:169) is local-only (ListenUDP). The CoA branch (:200-210) calls `serverIPs` (225-245) which does `resolver.LookupIPAddr(context.Background(), host)` (:235-236) for hostname servers: UNBOUNDED DNS on the apply path. Declared `ApplyBudget: 1` (:156). Doctor check `l2tp-auth-radius-servers` exists (:63-71, probe in doctor.go:96).
- [ ] `internal/component/l2tp/plugins/authradius/config.go` - `coa-port` parsed (93-100) gates the CoA branch.
- [ ] `internal/component/l2tp/plugins/authradius/yang/ze-l2tp-auth-radius-conf.yang` - NO coa-port leaf (whole file read); `server/address` is `type string` "IP address or hostname" (43-47), so hostnames are legal.
- [ ] `internal/component/config/parser.go` - unknown fields are rejected at parse unless the container has `ze:allow-unknown-fields` (372-380); the authradius container does not, so `coa-port` cannot arrive via production config today (latent gate).
- [ ] `internal/component/resolve/dns/resolver.go` - `NewResolver` (54-89) is network-free (resolv.conf file read only, 118-130); queries are per-call with bounded timeout (client built with Timeout, :80-83, default 5s :56-58; Exchange :263). Constructed at hub startup by `newResolvers` (`cmd/ze/hub/main_system.go:73-92`) without issuing any query.
- [ ] `internal/component/ike/engine/reconcile.go` - out-of-list confirmation for the DNS sweep: IKE resolves peer addresses (`fsm.go:102`) inside per-peer session goroutines (`go ps.run(...)` reconcile.go:284, runOnce :300), off the apply path.

**Behavior to preserve:**
- All touchpoints keep their current lazy/detached connect semantics, retry
  intervals, and timeouts (RADIUS 3s/3 client.go:63-68; TACACS 5s client.go:93-95;
  RTR 30s dial / 600s retry rtr_session.go:77,123; BMP 10s dial / 30-720s backoff
  sender.go:25-29,91; managed 5s dial / 1-60s backoff client.go:33,56).
- NTP single-writer invariant: at most one syncWorker adjusts the clock at a
  time (the synchronous handoff in startWorker guarantees it; the fix must keep
  the handoff synchronous and shorten only the in-flight-query wait).
- AAA chain fallthrough on infra error (aaa/types.go:103-112) and the
  `test/plugin/aaa-radius-fallback.ci` expectations.
- Legitimate hard failures (invalid config, missing local resources) still fail
  fast; this spec is about UNREACHABLE PEERS only.

**Behavior to change:**
- NTP: config re-apply with unreachable servers must stop the old worker within
  one in-flight query (~5s), not N x 5s (`doSync` gets stop-checks between
  per-server queries; declared ApplyBudget raised to cover the residual wait).
- L2TP authradius: the CoA `serverIPs` DNS lookup gets a bounded context so a
  dead resolver can never exceed the apply budget, even after a coa-port YANG
  leaf lands.
- Managed hub: connection state becomes operator-visible (doctor check),
  closing the only touchpoint with a log-only surface (AC-5).

### Classification Table (AC-1 deliverable)

| # | Touchpoint | Classification | First network contact (producer file:line) | Needs fix |
|---|-----------|----------------|--------------------------------------------|-----------|
| 1 | TACACS+ | lazy (good) | `TacacsClient.dial` `internal/component/tacacs/client.go:488-494` (bounded :489, default 5s :93-95), reached only from the auth/authz/acct request path (client.go:208-235); constructor client.go:88-103 and AAA `Build` register.go:24-71 do no I/O | N |
| 2 | RPKI RTR | lazy (good) | dial inside detached per-server goroutine: `startSessions` `internal/component/bgp/plugins/rpki/rpki.go:275` spawns `RTRSession.Run` `rtr_session.go:88-108`; dial bounded 30s `rtr_session.go:123,143`; OnConfigure returns without waiting (rpki.go:189-202) | N |
| 3 | BMP collectors | lazy (good) | dial inside detached goroutine: `startSender` `internal/component/bgp/plugins/bmp/bmp.go:343` spawns `run` `sender.go:80-130`; dial bounded 10s `sender.go:91,97`; backoff `sender.go:25-29`; event path non-blocking (`writeMsg` -> errNotConnected `sender.go:196-198`) | N |
| 4 | Managed-node client | lazy (good) | goroutine spawned AFTER readiness: `cmd/ze/hub/main.go:911` (after WaitForStartupComplete :887-898 + ready file :900-904); dial+TLS bounded 5s `internal/component/managed/client.go:33,113,120,126`; reconnect backoff 1-60s :56 | N (health gap: log-only, client.go:71-73,162 -> Phase 4) |
| 5 | NTP | eager-bounded on reload -> FIX (boot itself is lazy) | worker detached `internal/plugins/ntp/ntp.go:85-87`; BUT `OnConfigApply` `register.go:169-177` -> `startWorker` :113 -> `stopAndWait` `ntp.go:90-93` waits out in-flight `doSync` `ntp.go:154-188` (serial `ntp.Query` :158, 5s each per beevik/ntp v1.5.0 ntp.go:67,482-483; 1s retry loop ntp.go:113-124); apply aborts at deadline `orchestrator.go:423-424` | Y |
| 6 | RADIUS component | lazy (good) - REFERENCE | `Client.Exchange` resolves per call `internal/component/radius/client.go:134`; bounded retries :145-192 (3s/3 defaults :63-68); constructor only local ListenUDP :74 | N |
| 7 | L2TP authradius | eager-blocking, LATENT -> FIX | `serverIPs` `internal/component/l2tp/plugins/authradius/register.go:235-236`: `LookupIPAddr(context.Background(), ...)` unbounded, on apply path via `activateRadiusConfig` :127,:139,:200-210; gated by coa-port (config.go:93-100) which has no YANG leaf and is parser-rejected today (`internal/component/config/parser.go:372-380`) | Y (bound it now; production-unreachable until a coa-port leaf exists) |
| 8 | DNS in config apply | lazy (good) elsewhere | sweep `rg 'net\.Lookup|LookupIPAddr|LookupHost|Resolve(TCP|UDP|IP)Addr' internal/ cmd/` leaves only row 7 on an apply path; resolvers constructed without network (`dns/resolver.go:54-89`, `cmd/ze/hub/main_system.go:73-92`), queried per-call bounded (`dns/resolver.go:56-58,80-83,263`); IKE resolves in session goroutines (`reconcile.go:284,300` -> `fsm.go:102`) | N (folded into row 7) |

Per-touchpoint health surfaces (AC-5 inventory):

| Touchpoint | Surface today | Producer |
|-----------|---------------|----------|
| RADIUS | doctor `radius-admin-unreachable` | `internal/component/radius/doctor.go:23-33,42-93` |
| L2TP authradius | doctor `l2tp-auth-radius-servers` | `internal/component/l2tp/plugins/authradius/register.go:63-71`, doctor.go:96 |
| TACACS | `ze tacacs show <config>` offline probe + warn logs | `internal/component/tacacs/cli/main.go:85,190,209-210`; client.go:226-228 |
| RPKI | `show bgp rpki status` / `cache` session states | `internal/component/bgp/plugins/rpki/rpki.go:876,914` |
| BMP | `show bmp collectors` | `internal/component/bgp/plugins/bmp/cmd_show.go:16` |
| NTP | `show system ntp` / `show system ntp peers` (reach, last-error) | `internal/plugins/ntp/register.go:226-285` |
| Managed hub | NONE (logs only) -> Phase 4 adds doctor check | `internal/component/managed/client.go:71-73,162` |

## Data Flow (MANDATORY)

### Entry Point
- Daemon startup sequence (`cmd/ze/hub/main.go`: engine start -> plugin 5-stage
  startup -> `WaitForStartupComplete` :887-898 -> ready file :900-904 -> managed
  client :911) and config-apply transactions
  (`internal/component/config/transaction/orchestrator.go` runVerify/runApply)
  that reach external-service client setup in plugin OnConfigure/OnConfigApply
  callbacks and AAA bundle builds.

### Transformation Path
1. Audit classified each touchpoint (Classification Table): lazy / eager-bounded / eager-blocking.
2. FIX 1 (NTP): `doSync` becomes stop-aware between per-server queries so `stopAndWait` returns within one in-flight query; declared ApplyBudget raised to cover the residual worst case.
3. FIX 2 (authradius): `serverIPs` lookup gets a bounded context (derived timeout well under ApplyBudget) and degrades by skipping unresolved hostnames with a warning.
4. Health: managed hub connection state exported through a doctor check registered in the owning package (radius doctor pattern); unreachable peers surface via doctor/show, never as boot or apply failures.
5. Functional pin: `test/plugin/startup-unreachable-services.ci` boots with all service addresses blackholed and asserts ready + commit within budget.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| startup ↔ external services | per-touchpoint connect timing (Classification Table, producer file:line each) | [ ] |
| config apply ↔ plugin callbacks | OnConfigApply deadline enforced by orchestrator.go:423-424; NTP/authradius fixes keep callbacks under budget | [ ] |
| health ↔ operator | doctor checks + show commands per touchpoint (AC-5 inventory table) | [ ] |

### Integration Points
- `internal/plugins/ntp/ntp.go` `syncWorker.doSync` / `stopAndWait` - stop-awareness lands here; `register.go` budget declaration.
- `internal/component/l2tp/plugins/authradius/register.go` `serverIPs` - bounded context lands here.
- `internal/component/managed/client.go` `RunManagedClient`/`runConnection` - connection-state tracking read by the new doctor check.
- `internal/core/diagnostic/codes.go` - new `doctor-hub-unreachable` code (doctor-checks rule).

### Architectural Verification
- [ ] No bypassed layers (fixes stay inside the owning component/plugin)
- [ ] No unintended coupling (no central "connection manager" invented; each touchpoint keeps its own retry loop)
- [ ] No duplicated functionality (reuse existing backoff/retry helpers: managed `Backoff`, sender reconnect consts, worker stop channels)
- [ ] Registration over hardcoding - the managed-hub doctor check registers via `diagnostic.RegisterDoctorCheck` from the owning package (radius pattern), not via a hardcoded list

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | TACACS+ dials lazily or with bounded timeout off the startup path | `dial` client.go:488-494 bounded :489 (default 5s :93-95), reached only from request path client.go:208-235; Build register.go:24-71 no I/O | fix in scope | read producers (done 2026-07-10) | confirmed |
| A-2 | RPKI RTR connects in a background loop that cannot block config apply | `startSessions` rpki.go:275 detaches `Run` rtr_session.go:88-108; dial bounded rtr_session.go:123 | fix in scope | read producers (done 2026-07-10) | confirmed |
| A-3 | BMP collector dial cannot block session establishment or apply | `startSender` bmp.go:343 detaches `run` sender.go:80-130; `writeMsg` non-blocking sender.go:196-198 | fix in scope | read producers (done 2026-07-10) | confirmed |
| A-4 | Managed-node client tolerates an unreachable hub at boot | spawned after readiness cmd/ze/hub/main.go:887-911; dial bounded client.go:33,113,120 | fix in scope | read producers (done 2026-07-10) | confirmed |
| A-5 | NTP plugin startup is decoupled from server reachability | BROKEN for re-apply: `stopAndWait` ntp.go:90-93 called from OnConfigApply path register.go:113,169-177 waits out serial 5s queries ntp.go:154-188 (first boot itself IS decoupled, worker detached ntp.go:85-87) | fix in scope (Phase 2) | read producers (done 2026-07-10) | broken |
| A-6 | No config-apply path blocks on DNS resolution of a peer hostname | BROKEN (latent): `serverIPs` register.go:235-236 unbounded LookupIPAddr on apply path; unreachable via production config today because coa-port has no YANG leaf and parser rejects unknown fields (parser.go:372-380) | fix in scope (Phase 3) | full-tree sweep of net.Lookup*/Resolve* + producer read (done 2026-07-10) | broken |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Scope creep into re-apply idempotency | audit findings unrelated to reachability | file findings into a separate skeleton spec; keep this one reachability-only (none observed during this audit) |
| R-2 | Making a dial lazy hides a real misconfiguration | operators miss dead servers | every touchpoint has a named health surface (AC-5 inventory); Phase 4 closes the managed-hub gap; both FIX phases pair with existing surfaces |
| R-3 | Background retry loops leak goroutines across config reloads | goroutine growth in tests | reload-cycle unit test per fixed touchpoint (NTP: `TestSyncWorkerReloadNoGoroutineLeak`; authradius adds no goroutine - N/A recorded) |
| R-4 | NTP fix breaks the single-writer clock invariant (two workers stepping the clock) | flapping clock in tests | keep the startWorker handoff synchronous; only shorten the in-flight-query wait via stop-checks between queries |
| R-5 | Blackhole test flakes on slow CI (timeouts near budget) | intermittent .ci failures | assert readiness against the runner's existing ready-file budget, not bespoke sleeps; use RFC 5737 drop addresses so nothing depends on firewall state |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| daemon boots with TACACS/RPKI/BMP/NTP/hub addresses blackholed (203.0.113.x) | → | lazy/detached connect paths (Classification Table rows 1-6); readiness at cmd/ze/hub/main.go:887-904 | `test/plugin/startup-unreachable-services.ci` |
| config commit touching `environment` while NTP servers blackholed | → | stop-aware `doSync` + `stopAndWait` (internal/plugins/ntp/ntp.go) under apply deadline | `test/plugin/startup-unreachable-services.ci` (commit step) + `TestStartWorkerReloadBoundedWait` |
| config apply with hostname RADIUS server + coa-port (once YANG leaf exists) | → | bounded `serverIPs` lookup (internal/component/l2tp/plugins/authradius/register.go) | `TestServerIPsBoundedTimeout` (unit; the coa-port leaf is not settable today, see Known Limitations, so the .ci for this row belongs to the coa-port spec) |
| `ze doctor` with hub down | → | new managed-hub doctor check (internal/component/managed/doctor.go) | `test/plugin/startup-unreachable-services.ci` (doctor step) + `TestHubDoctorCheckUnreachable` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | audit complete | every touchpoint row classified with producer `file:line` evidence in this spec (Classification Table above) |
| AC-2 | all configured external services blackholed (drop) | daemon startup reaches ready (ready file) within the test runner's normal budget; CLI answers |
| AC-3 | config commit touching `environment` root while NTP servers (and all other services) are blackholed | commit succeeds within the apply deadline; NTP worker handoff waits at most one in-flight query (~5s), not N x 5s |
| AC-4 | service becomes reachable later | touchpoint converges without restart: NTP unit test proves worker syncs once a server starts answering; RPKI/BMP/managed convergence rests on their audited retry loops (rtr_session.go:88-108, sender.go:80-130, client.go:55-84) plus existing unit suites |
| AC-5 | unreachable peer | surfaced via health/doctor, not silent: per-touchpoint inventory table filled; managed hub gains a doctor check (`doctor-hub-unreachable`) |
| AC-6 | authradius CoA config with hostname server and dead DNS | `serverIPs` returns within its bounded context; apply never exceeds the plugin's declared budget; unresolved hostnames logged and skipped |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | boots an appliance while the RADIUS/TACACS/RPKI/BMP/NTP/hub network is down | startup → lazy clients / detached loops → ready file; show commands + doctor report degraded peers | `test/plugin/startup-unreachable-services.ci` |
| 2 | commits a config change during the outage | commit → verify/apply transaction → NTP stop-aware handoff → committed under deadline | `test/plugin/startup-unreachable-services.ci` (commit step) |
| 3 | restores the NTP server network | worker retry loop (1s) → sync → `show system ntp` synced=true | `TestSyncWorkerConvergesWhenServerAppears` (in-process UDP NTP responder) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDoSyncStopChecksBetweenServers` | `internal/plugins/ntp/ntp_test.go` | with 3+ unresponsive servers, closing stop mid-doSync abandons remaining queries (returns before querying server 2) | |
| `TestStartWorkerReloadBoundedWait` | `internal/plugins/ntp/ntp_test.go` | startWorker handoff with unreachable servers returns within one query timeout + margin, not N x timeout | |
| `TestSyncWorkerReloadNoGoroutineLeak` | `internal/plugins/ntp/ntp_test.go` | N startWorker cycles leave exactly zero live worker goroutines (R-3) | |
| `TestSyncWorkerConvergesWhenServerAppears` | `internal/plugins/ntp/ntp_test.go` | worker pointed at in-process UDP responder that starts answering later reaches synced=true without restart (AC-4) | |
| `TestServerIPsBoundedTimeout` | `internal/component/l2tp/plugins/authradius/register_test.go` (or config_test.go) | lookup of a non-resolving hostname returns within the bounded context deadline; IP-literal servers never invoke the resolver | |
| `TestServerIPsSkipsUnresolvedHostname` | `internal/component/l2tp/plugins/authradius/register_test.go` | unresolved hostname is skipped with the remaining servers still in the allow list | |
| `TestHubDoctorCheckUnreachable` | `internal/component/managed/doctor_test.go` | doctor check reports `doctor-hub-unreachable` when the client state says disconnected, healthy when connected, skip when unmanaged | |
| `TestHubConnectionStateTransitions` | `internal/component/managed/client_test.go` | connection-state value transitions connecting→connected→disconnected as runConnection progresses/fails | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| (no new numeric config inputs) existing timeout leaves already YANG-bounded: tacacs timeout uint16 (config.go:79-83), authradius timeout 1..30 (yang:21-25 via config.go:66-73), ntp interval 60..86400 / max-step 0..86400 / slew-threshold 0..1000 (ntp.go:396-421) | - | - | - | - |
| authradius `serverIPs` total lookup bound (`coaResolveTimeout`, 750ms, SHARED across all servers) | < declared ApplyBudget (750ms < 1s) | `TestServerIPsBoundedTimeout` + `TestServerIPsSharedDeadlineAcrossServers` | N/A (constant) | N/A (constant) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `startup-unreachable-services` | `test/plugin/startup-unreachable-services.ci` | daemon configured with TACACS (203.0.113.1:49), RPKI cache (203.0.113.2:323), BMP collector (203.0.113.3:11019), NTP (203.0.113.4) all blackholed: boots to ready within runner budget; CLI answers; a commit touching `environment` succeeds; `show bgp rpki cache` shows non-established sessions, `show bmp collectors` shows disconnected, `show system ntp` shows synced=false; `ze doctor` (or doctor RPC) reports hub/service degradation without failing boot | |
| (existing, referenced) `aaa-radius-fallback` | `test/plugin/aaa-radius-fallback.ci` | RADIUS unreachable → local auth fallback (pins the auth-path story) | exists |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A - reachability/lifecycle behaviour, no wire format change | - | - | functional + unit coverage above | - |

### Future (if deferring any tests)
- authradius CoA end-to-end .ci: deferred to the coa-port YANG spec (the leaf is
  not settable via production config today, parser.go:372-380). The unit tests
  above pin the bounded-lookup behavior now. Requires user approval at
  implementation if this is treated as a deferral rather than out-of-scope.

## Files to Modify
- `internal/plugins/ntp/ntp.go` - stop-aware `doSync` (stop-check between per-server queries); no change to clock logic; add `ntpQueryFn`/`setClockFn` test seams (matches radius `radiusAdminProbe` convention)
- `internal/plugins/ntp/register.go` - raise declared `ApplyBudget` from 5 to 10 so the residual one-query wait plus handoff fits the budget
- `internal/component/l2tp/plugins/authradius/register.go` - `serverIPs` takes a bounded context (compile-time constant timeout well under the apply budget) via a `lookupIPAddr` seam; unresolved hostnames logged + skipped
- `internal/core/diagnostic/codes.go` - add `doctor-hub-unreachable` code

**DEVIATION from readied design (Phase 4):** the managed-hub health signal is a
STATELESS config-tree reachability probe, NOT an in-process connection-state
snapshot. `DoctorCheckContext` (internal/core/diagnostic/doctor_registry.go:33)
carries only `{Tree, ConfigDir, Plugins, Store, Platform}` and `ze doctor` runs
as a SEPARATE process from the running daemon, so an atomic snapshot in
`client.go` would always read "disconnected" under `ze doctor` (false positive).
The hub address is available from the config tree via
`config.ExtractHubConfig(tree)` (loader_extract.go:462 -> `hubCfg.Clients[i].Address()`,
the same source `extractManagedClientConfig` uses at cmd/ze/ze_core_start.go:296),
so the check mirrors `radius/doctor.go` (read tree, probe servers). Consequence:
`client.go` is NOT modified; `TestHubConnectionStateTransitions` is dropped in
favour of a probe-seam test.

## Files to Create
- `internal/component/managed/doctor.go` - hub-connection doctor check (radius component pattern: `diagnostic.RegisterDoctorCheck` from the owning package; skip when no managed client is configured)
- `internal/component/managed/doctor_test.go` - unit test for the check
- `test/plugin/startup-unreachable-services.ci` - invariant functional test (see Functional Tests row)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan — check what exists |
| 3. Wiring phase | Wiring Test table — the .ci test is the wiring proof for the boot invariant |
| 4. Implement (TDD) | Implementation phases below (write-test-fail-implement-pass per phase) |
| 5. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 6. Critical review | Critical Review Checklist below |
| 7. Fix issues | Fix every issue from critical review |
| 8. Re-verify | Re-run stage 5 |
| 9. Repeat 6-8 | Until clean |
| 10. Deliverables review | Deliverables Checklist below |
| 11. Security review | Security Review Checklist below |
| 12. Documentation review | Documentation Update Checklist below |
| 13. /ze-review gate | Review Gate section — run `/ze-review`; fix every BLOCKER/ISSUE; re-run until 0 BLOCKER/0 ISSUE |
| 14. Present summary + close | Executive Summary Report; two-commit closure per `ai/rules/planning.md` |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: AUDIT (DONE 2026-07-10)** — every touchpoint's producer read and
   classified; the Classification Table above IS the output (AC-1). A-1..A-6
   resolved (4 confirmed, 2 broken). No code change.
2. **Phase: NTP reload block fix** — make the worker handoff bounded.
   - Change: `internal/plugins/ntp/ntp.go` `doSync` checks the stop channel
     between per-server `ntp.Query` calls (loop at ntp.go:154-188) and returns
     early when stopping; `internal/plugins/ntp/register.go` raises
     `ApplyBudget` to 10 (worst residual wait = one in-flight query 5s + jitter
     + handoff, comfortably inside budget).
   - Keep synchronous handoff (R-4): `stopAndWait` still waits; only the wait
     shrinks.
   - Health signal (R-2): existing `show system ntp peers` per-server reach +
     last-error (register.go:255-285) — verify the .ci asserts synced=false.
   - Goroutine-leak guard (R-3): `TestSyncWorkerReloadNoGoroutineLeak`.
   - Tests: `TestDoSyncStopChecksBetweenServers`, `TestStartWorkerReloadBoundedWait`,
     `TestSyncWorkerConvergesWhenServerAppears` (fail first against current code
     for the first two; convergence test passes before and after — pins AC-4).
3. **Phase: authradius bounded DNS fix** — bound the latent apply-path lookup.
   - Change: `internal/component/l2tp/plugins/authradius/register.go`
     `serverIPs` uses `context.WithTimeout` (new package constant, ~2s total)
     for `LookupIPAddr`; on error/timeout log a warning naming the hostname and
     continue (CoA source filtering degrades to the resolvable subset — same
     failure mode as today's silent skip, now bounded and logged).
   - Health signal (R-2): existing doctor `l2tp-auth-radius-servers`
     (register.go:63-71) already probes reachability post-config.
   - Goroutine-leak guard (R-3): N/A — no goroutine added; record in audit.
   - Tests: `TestServerIPsBoundedTimeout`, `TestServerIPsSkipsUnresolvedHostname`
     (fail first: current signature takes no context/deadline).
   - NOTE: the coa-port YANG gap itself (leaf parsed at config.go:93-100 but
     absent from the YANG) is a separate wiring gap — routed out, see Known
     Limitations. Do NOT add the leaf here.
4. **Phase: managed-hub health signal** — close the AC-5 gap.
   - Change (CORRECTED, see Deviation under Files to Modify): stateless probe.
     `internal/component/managed/doctor.go` (new, with `init()`) registers a
     `hub-unreachable` doctor check via `diagnostic.RegisterDoctorCheck`
     (radius component pattern, not the plugin-registry path — managed is a
     component); the check reads `ctx.Tree` (`config.Tree`), extracts hub client
     blocks via `config.ExtractHubConfig`, and TCP-dials each `Address()` with a
     bounded timeout through a probe seam; when servers exist but none answers it
     warns `doctor-hub-unreachable`; no-op when no client block is configured.
     `internal/core/diagnostic/codes.go` gains the code. `client.go` unchanged.
   - Tests: `TestHubDoctorCheckUnreachable` (unreachable→warn, reachable→clean,
     no-client→skip, via the probe seam), fail first (check does not exist).
5. **Phase: functional invariant pin** — `test/plugin/startup-unreachable-services.ci`.
   - Scenario (see Functional Tests table): all services at RFC 5737 drop
     addresses; assert ready file within runner budget, CLI answers, commit
     touching `environment` succeeds, per-touchpoint show/doctor surfaces report
     degraded-not-fatal.
   - This test also serves as the wiring proof (Wiring Test rows 1-2, 4).
6. **Full verification** → `make ze-verify` (lint + all ze tests except fuzz)
7. **Complete spec** → Fill audit tables, write learned summary to `plan/learned/NNN-<name>.md`. TWO commits: commit A saves code + tests + spec + learned summary; commit B does `git rm` of the spec.

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-6 has implementation + test with file:line |
| Feature completeness | Every End-to-End User Story path works; story 2 (commit during outage) exercised by the .ci commit step |
| Correctness | NTP handoff stays synchronous (single clock writer, R-4); stop-check placement covers BOTH the phase-2 initial-sync loop and periodic doSync; authradius bounded context cancels the lookup, does not orphan it |
| Timing | worst-case OnConfigApply wait for NTP measured < declared ApplyBudget; authradius lookup bound < its ApplyBudget |
| Naming | doctor code `doctor-hub-unreachable` follows existing `doctor-radius-unreachable` convention; test names match TDD table |
| Data flow | fixes stay inside owning packages; no cross-component connection manager introduced |
| Registration over hardcoding | managed doctor check registered via `diagnostic.RegisterDoctorCheck` in the owning package, discovered by the doctor runner; nothing added to a central check list by hand |
| Doctor checks | new check has: code in `internal/core/diagnostic/codes.go`, unit test, functional assertion in the .ci (per `ai/rules/doctor-checks.md`) |
| YANG validation | no YANG changes in this spec (coa-port leaf explicitly out of scope) — verify none crept in |
| Rule: no-workarounds | the .ci must not weaken assertions to pass (e.g., no removal of the commit step if it flakes — fix the source) |
| Rule: buffer-first / no-sprintf-alloc | untouched hot paths stay untouched; new code is control-plane only |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| Stop-aware NTP doSync | `go test ./internal/plugins/ntp/ -run 'TestDoSyncStopChecksBetweenServers|TestStartWorkerReloadBoundedWait'` passes; grep ntp.go for stop-check inside the server-query loop |
| NTP ApplyBudget raised | grep `ApplyBudget` internal/plugins/ntp/register.go shows 10 |
| Bounded authradius serverIPs | `go test ./internal/component/l2tp/plugins/authradius/ -run TestServerIPs` passes; grep register.go shows `context.WithTimeout` (no bare `context.Background()` passed to LookupIPAddr) |
| Managed connection state + doctor check | `go test ./internal/component/managed/ -run 'TestHub'` passes; grep `doctor-hub-unreachable` in internal/core/diagnostic/codes.go and internal/component/managed/doctor.go |
| Reload leak guard | `go test ./internal/plugins/ntp/ -run TestSyncWorkerReloadNoGoroutineLeak` passes |
| Functional invariant test | `ls test/plugin/startup-unreachable-services.ci`; suite run passes it |
| Convergence evidence (AC-4) | `go test ./internal/plugins/ntp/ -run TestSyncWorkerConvergesWhenServerAppears` passes |
| Spec audit tables filled | this file: Implementation Audit + Pre-Commit Verification complete |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Input validation | doctor check output must not leak the hub auth token or TLS material; connection-state snapshot carries address + error string only |
| Resource exhaustion | bounded lookup context is always cancelled (defer); no goroutine spawned per apply in authradius; NTP reload cannot stack workers (leak test) |
| Degradation semantics | skipping an unresolved CoA hostname SHRINKS the allow list (fail-closed for CoA source filtering, never fail-open); verify the skip cannot admit packets from unlisted sources |
| Error leakage | doctor/show surfaces report reachability, not shared secrets or tokens; log lines follow existing slog patterns (no secret fields) |
| Timing | shortened NTP handoff must not allow two workers to call setClock concurrently (R-4): assert handoff remains synchronous |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No (resilience hardening; no new config surface) | - |
| 2 | Config syntax changed? | No (no YANG changes) | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No user-facing surface | behavior-only (apply timing); grep `docs/guide/plugins.md` found no apply-time NTP/radius claim to update |
| 6 | Has a user guide page? | No stale claim | grep `docs/guide/` for ntp/radius apply semantics: none describe the fixed internal timing |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented/changed? | No | - |
| 10 | Test infrastructure changed? | No new category | `docs/functional-tests.md` is category-level (Plugin = `test/plugin/*.ci`); the new .ci fits it, no per-test entry exists |
| 11 | Affects daemon comparison? | No | grep `docs/comparison.md` for startup/resilience: no such row exists |
| 12 | Internal architecture changed? | No (no new subsystems) | - |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No (doctor check only) | - |
| 15 | Registered plugin/event/command/capability inventory changed? | Yes (new doctor check) | DONE: `docs/guide/health-checks.md` doctor-code table + source anchor |
| 16 | Changed source files referenced by doc source anchors? | Verified none stale | `docs/features/fleet-management.md` anchors `client.go -- RunManagedClient` (unchanged) and `plugin/server/managed.go` (unchanged); no other anchors on changed files |
| 17 | Existing docs show config/CLI/API examples for this area? | Unchanged | `show system ntp` output unchanged (no field/format change); doctor examples still valid |

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| A-5: NTP fully decoupled from server reachability | first boot is decoupled, but config re-apply blocks in `stopAndWait` (ntp.go:90-93) for up to N x 5s while servers are dark, risking transaction abort (orchestrator.go:423-424) | reading the producer chain OnConfigApply -> startWorker -> stopAndWait -> doSync | Phase 2 fix |
| A-6: no apply path resolves DNS | authradius `serverIPs` (register.go:235-236) does an unbounded lookup on apply; only saved today by a missing coa-port YANG leaf (parser rejects unknown fields, parser.go:372-380) | full-tree sweep of net.Lookup*/Resolve* + producer read | Phase 3 fix (bounded now, before the leaf lands) |

## Known Limitations
- Re-apply idempotency (osvbng's sibling theme) is explicitly out of scope
  (R-1); the audit did not hunt for such gaps and none were incidentally found.
- coa-port wiring gap: `internal/component/l2tp/plugins/authradius/config.go:93-100`
  parses a `coa-port` knob that has no YANG leaf, so the whole CoA listener
  branch (register.go:200-210) is unreachable via production config. Route to a
  separate spec ("authradius coa-port YANG leaf + end-to-end CoA test"); this
  spec only bounds the DNS lookup so the branch is safe whenever it goes live.
- AC-4 convergence is pinned end-to-end only for NTP (in-process UDP responder
  unit test). RPKI (600s retry), BMP (30s min reconnect) and managed hub
  (TLS+token handshake) converge via their audited retry loops
  (rtr_session.go:88-108, sender.go:80-130, client.go:55-84) but are not
  .ci-timed; adding fast-retry knobs solely for tests was rejected as
  test-driven config surface.
- TACACS has no doctor check (only the `ze tacacs show` offline probe,
  `internal/component/tacacs/cli/main.go:85-210`, and warn logs). Acceptable
  for AC-5 (not silent); a doctor check would mirror radius but is not required
  by any fix here — note for a future doctor-coverage pass.

## Design Insights
- The startup half of the invariant already held everywhere; the residual risk
  in Ze concentrates in OnConfigApply callbacks because the transaction
  coordinator enforces a real deadline (orchestrator.go:423-424) — a blocking
  apply is a FAILED COMMIT, which is worse than slow boot.
- "Detached goroutine + bounded dial + backoff + non-blocking event path" is the
  established Ze pattern (RPKI, BMP, managed); the RADIUS "resolve per exchange"
  pattern covers request-path clients. New external clients should copy one of
  the two, never dial in a constructor or apply callback.

## Core Insight
The dangerous surface is not startup (already lazy everywhere) but synchronous
teardown of workers that may be mid-network-call inside OnConfigApply: the NTP
`stopAndWait` blocks on an uncancellable in-flight query. Any worker whose
reload path joins a goroutine that performs blocking network I/O needs either
cancellable I/O or stop-checks between bounded calls.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| NTP: stop-checks between serial queries, keep synchronous handoff | (a) async stopAndWait (detach old worker); (b) cancellable Query via library options/context | (a) breaks single-clock-writer invariant (R-4); (b) beevik/ntp v1.5.0 Query has no context parameter — wrapping in a goroutine orphans the query; stop-checks bound the wait to one 5s query with zero concurrency risk |
| authradius: bound the lookup, do not move it off the apply path | background re-resolution loop for CoA allow list | branch is latent (no YANG leaf); a background loop adds a goroutine + reload lifecycle for a feature that cannot be enabled — bounding is proportionate now, revisit in the coa-port spec |
| managed health: doctor check reading an atomic state snapshot | new `show hub` CLI command; Prometheus gauge | doctor is the mandated surface for unreachable dependencies (ai/rules/doctor-checks.md); a show command can layer on the same state later |
| .ci uses RFC 5737 drop addresses | nftables drop rules in the test namespace | no firewall setup, deterministic timeout semantics, works in any runner |
| ApplyBudget 10 for ntp | keep 5 and shrink query timeout | shrinking the library timeout changes sync behavior on slow WANs; budget raise is honest about the residual one-query wait |

## RFC Documentation

No new RFC-mandated behavior. Existing references in touched files stay:
RFC 5905 jitter note (ntp.go:146), RFC 5176 CoA (authradius), RFC 8907 TACACS+
(client.go), RFC 8210 RTR (rtr_session.go), RFC 7854 BMP (sender.go).

## Implementation Summary

### What Was Implemented
- **FIX 1 (NTP reload block):** `doSync` now checks the stop channel before each
  per-server `ntp.Query` (`internal/plugins/ntp/ntp.go` server loop), bounding a
  reload/shutdown handoff to at most one in-flight query. Added `ntpQueryFn` /
  `setClockFn` test seams. Raised the plugin `ApplyBudget` from 5 to 10 seconds
  (`internal/plugins/ntp/register.go`). Tests: `TestDoSyncStopChecksBetweenServers`,
  `TestStartWorkerReloadBoundedWait`, `TestSyncWorkerReloadNoGoroutineLeak`,
  `TestSyncWorkerConvergesWhenServerAppears`.
- **FIX 2 (authradius unbounded DNS):** `serverIPs` now resolves hostname RADIUS
  servers through `resolveCoAHost` with a bounded `context.WithTimeout`
  (`coaResolveTimeout`, 2s) via a `lookupIPAddr` seam; unresolved hostnames are
  logged and skipped (CoA allow list degrades to the resolvable subset).
  Tests: `TestServerIPsBoundedTimeout`, `TestServerIPsSkipsUnresolvedHostname`,
  `TestServerIPsIPLiteralNoResolver`.
- **FIX 3 (managed hub health signal):** new `internal/component/managed/doctor.go`
  + `register.go` register a stateless `hub-unreachable` doctor check that reads
  the hub client blocks from the config tree (`config.ExtractHubConfig`) and
  TCP-probes each with a bounded dial (`doctor-hub-unreachable`). New code added to
  `internal/core/diagnostic/codes.go`. Tests: `TestHubDoctorCheckUnreachable`,
  `TestCheckHubReachableNonTreeContext`, `TestHubReachableProbe`.
- **Functional pin:** `test/plugin/startup-unreachable-services.ci` proves the
  invariant end-to-end (ze doctor reports all five services unreachable incl.
  the new hub check; daemon boots to ready; a SIGHUP reload of environment/ntp
  re-applies within budget) — all with RFC 5737 blackhole addresses.

### Bugs Found/Fixed
- NTP reload block (real) and latent authradius unbounded DNS-on-apply — see Mistake Log.

### Documentation Updates
- `docs/guide/health-checks.md`: added an external-service-reachability row to the
  doctor-code table documenting `doctor-hub-unreachable` and its siblings, with a
  `source:` anchor to `internal/component/managed/doctor.go`. Regenerated
  `ai/DOCS-TO-CODE.md` / `ai/CODE-TO-DOCS.md`. `make ze-doc-test` passes.

### Deviations from Plan
- **Phase 4 mechanism (managed hub):** the readied design proposed an in-process
  connection-state snapshot in `client.go` read by the doctor check. Corrected to
  a STATELESS config-tree reachability probe: `DoctorCheckContext` carries only
  the parsed config (`internal/core/diagnostic/doctor_registry.go:33`) and
  `ze doctor` runs as a separate process, so an in-process snapshot would always
  read disconnected under `ze doctor`. `client.go` is unmodified;
  `TestHubConnectionStateTransitions` was dropped for a probe-seam test. Deliverable
  (AC-5) unchanged. Verified the hub address is available from the tree via
  `config.ExtractHubConfig` (loader_extract.go:462).
- **A-4 strengthened:** the only synchronous hub fetch on the boot path
  (`fetchInitialConfig`) runs solely on first boot with NO cached config
  (`cmd/ze/ze_core_start.go:272`); with any cached config the daemon starts
  immediately and connects to the hub in the background. Managed hub is even less
  of a boot risk than the audit stated.
- **AC-3 coverage split:** the .ci proves the initial apply (boot) and a real
  SIGHUP re-apply both succeed with services blackholed; the precise per-query
  boundedness of the NTP handoff is pinned deterministically by
  `TestStartWorkerReloadBoundedWait` / `TestDoSyncStopChecksBetweenServers`.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Audit every touchpoint with producer evidence (AC-1) | Done | Classification Table | 8 rows, all producer-cited |
| Fix touchpoints that block startup/apply | Done | ntp.go, authradius/register.go | NTP reload + authradius DNS |
| Add missing health surface (managed hub) | Done | managed/doctor.go | doctor-hub-unreachable |
| Pin invariant with a functional test | Done | startup-unreachable-services.ci | boot + doctor + reload |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | Classification Table (this file) | producer file:line per row |
| AC-2 | Done | startup-unreachable-services.ci (boot step) | ready file with all services blackholed |
| AC-3 | Done | .ci (boot + SIGHUP reload) + TestStartWorkerReloadBoundedWait | apply succeeds; handoff bounded |
| AC-4 | Done | TestSyncWorkerConvergesWhenServerAppears | NTP converges without restart |
| AC-5 | Done | .ci doctor step + TestHubDoctorCheckUnreachable | doctor-hub-unreachable + siblings |
| AC-6 | Done | TestServerIPsBoundedTimeout | bounded lookup < budget |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestDoSyncStopChecksBetweenServers | PASS | ntp_test.go | stop-check bounds queries |
| TestStartWorkerReloadBoundedWait | PASS | ntp_test.go | one query after reload |
| TestSyncWorkerReloadNoGoroutineLeak | PASS | ntp_test.go | R-3 |
| TestSyncWorkerConvergesWhenServerAppears | PASS | ntp_test.go | AC-4 |
| TestServerIPsBoundedTimeout | PASS | authradius/register_test.go | AC-6 |
| TestServerIPsSkipsUnresolvedHostname | PASS | authradius/register_test.go | fail-closed subset |
| TestServerIPsIPLiteralNoResolver | PASS | authradius/register_test.go | IP literals skip DNS |
| TestHubDoctorCheckUnreachable | PASS | managed/doctor_test.go | AC-5 |
| TestCheckHubReachableNonTreeContext | PASS | managed/doctor_test.go | defensive |
| TestHubReachableProbe | PASS | managed/doctor_test.go | real TCP probe |
| startup-unreachable-services (.ci) | PASS | test/plugin/ | AC-2/3/5 end-to-end |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| internal/plugins/ntp/ntp.go | Modified | seams + stop-check |
| internal/plugins/ntp/register.go | Modified | ApplyBudget 5->10 |
| internal/plugins/ntp/ntp_test.go | Modified | 4 tests |
| internal/component/l2tp/plugins/authradius/register.go | Modified | bounded serverIPs |
| internal/component/l2tp/plugins/authradius/register_test.go | Created | 3 tests |
| internal/component/managed/doctor.go | Created | hub check (probe) |
| internal/component/managed/register.go | Created | RegisterDoctorCheck |
| internal/component/managed/doctor_test.go | Created | 3 tests |
| internal/core/diagnostic/codes.go | Modified | doctor-hub-unreachable |
| test/plugin/startup-unreachable-services.ci | Created | invariant test |
| docs/guide/health-checks.md | Modified | doctor code table |
| internal/component/managed/client.go | NOT modified | Phase 4 deviation (stateless probe) |

### Audit Summary
- **Total items:** 6 ACs, 11 tests, 12 files
- **Done:** all
- **Partial:** none
- **Skipped:** none
- **Changed:** Phase 4 mechanism (client.go not modified) — see Deviations

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| daemon boots to functional state with all external services down | functional test | `test/plugin/startup-unreachable-services.ci`: ready file within budget + CLI answers with TACACS/RPKI/BMP/NTP/hub blackholed |
| config apply succeeds during the outage | functional test | same .ci: commit touching `environment` succeeds under the apply deadline |
| apply never blocks N x query-timeout on dead NTP | unit test | `TestStartWorkerReloadBoundedWait` (bounded to ~one query) |
| convergence when service returns | unit test | `TestSyncWorkerConvergesWhenServerAppears` (NTP); audited retry loops cited for RPKI/BMP/managed (Classification Table rows 2-4) |
| unreachable peers operator-visible, not boot failures | functional + unit | .ci doctor/show assertions + `TestHubDoctorCheckUnreachable` |
| latent DNS-on-apply bounded before it can go live | unit test | `TestServerIPsBoundedTimeout` |

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | ISSUE | Managed hub check used "reachable-if-any"; the daemon connects only to `hubCfg.Clients[0]` (ze_core_start.go:318), so a down primary + up secondary reported healthy (false-negative, defeats R-2) | internal/component/managed/doctor.go diagnoseHubReachability | fixed: probe the daemon-selected first client only; regression subtest "reachable secondary does not mask a down primary" |
| 2 | ISSUE | Bounded CoA lookup was 2s PER server but authradius `ApplyBudget` is 1s; a dead resolver (or several hostname servers) would still overrun the apply deadline once coa-port goes live (violates AC-6) | internal/component/l2tp/plugins/authradius/register.go serverIPs/coaResolveTimeout | fixed: single shared deadline (750ms < 1s budget) across all lookups; regression test TestServerIPsSharedDeadlineAcrossServers |
| - | pre-existing | ze-validate flags VPP exported symbols + a VPP doc anchor (internal/component/vpp/*, docs/guide/vpp.md) | not in this diff | out of scope (other effort's uncommitted files; no new exported symbols in this diff) |

### Fixes applied
- `internal/component/managed/doctor.go`: `diagnoseHubReachability` probes only `clients[0]` (the hub the daemon connects to); message singularized; test rewritten with the false-healthy regression subtest.
- `internal/component/l2tp/plugins/authradius/register.go`: `serverIPs` creates ONE `context.WithTimeout(coaResolveTimeout)` shared across all per-server lookups; `coaResolveTimeout` 2s -> 750ms (< 1s ApplyBudget); `resolveCoAHost` takes the shared ctx; added `TestServerIPsSharedDeadlineAcrossServers`.

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| - | none | fresh pass over the fixed diff (wiring, removed-behavior, logic, allocation, security, race) found no further findings; `-race` clean; lint 0 issues; .ci re-passes | - | - |

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

<!-- BLOCKING: Do NOT trust the audit above. Re-verify everything independently. -->

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| internal/component/managed/doctor.go | yes | created + tested |
| internal/component/managed/register.go | yes | created (init registers check) |
| internal/component/managed/doctor_test.go | yes | 3 tests PASS |
| internal/component/l2tp/plugins/authradius/register_test.go | yes | 3 tests PASS |
| test/plugin/startup-unreachable-services.ci | yes | suite PASS (37.6s) |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | every touchpoint classified | Classification Table (8 rows, producer file:line) |
| AC-2 | boot to ready blackholed | `.ci` "daemon reached ready with all external services blackholed" |
| AC-3 | apply/reload succeeds | `.ci` "environment/ntp reload applied within budget" + TestStartWorkerReloadBoundedWait PASS |
| AC-4 | converges when back | TestSyncWorkerConvergesWhenServerAppears PASS |
| AC-5 | surfaced via doctor | `ze doctor --json` emits doctor-hub-unreachable (+rpki/bmp/ntp) |
| AC-6 | bounded DNS lookup | TestServerIPsBoundedTimeout PASS (returns at ~50ms bound) |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| daemon boot with services blackholed | startup-unreachable-services.ci | yes (ready file) |
| config commit (SIGHUP) touching environment | startup-unreachable-services.ci | yes (no apply timeout) |
| ze doctor with hub down | startup-unreachable-services.ci | yes (doctor-hub-unreachable) |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | tacacs dial lazy (client.go:488) |
| A-2 | confirmed | rpki detached loop (rpki.go:275) |
| A-3 | confirmed | bmp detached run (bmp.go:343) |
| A-4 | confirmed (strengthened) | managed after-ready + first-boot-only sync fetch (ze_core_start.go:272) |
| A-5 | broken -> fixed | NTP stop-check (ntp.go) + TestDoSyncStopChecksBetweenServers |
| A-6 | broken -> fixed | bounded serverIPs (register.go) + TestServerIPsBoundedTimeout |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| doctor-hub-unreachable documented | docs/guide/health-checks.md + source anchor | make ze-doc-test PASS |
| discovery index fresh | ai/DOCS-TO-CODE.md / CODE-TO-DOCS.md regenerated | make ze-doc-test PASS |
| functional test category | docs/functional-tests.md (category-level, no per-test entry) | verified not applicable |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`)
- [ ] Every touchpoint classified with producer evidence (Classification Table)
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`); broken ones in Mistake Log

### Quality Gates (SHOULD pass)
- [ ] Health/doctor signal exists for every tolerated-unreachable peer (AC-5 inventory table complete, managed gap closed)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No speculative features (coa-port leaf explicitly NOT added)
- [ ] Single responsibility per fix (each phase touches one owning package)
- [ ] Explicit > implicit behavior (skipped hostnames logged, budgets declared)
- [ ] Minimal coupling (no central connection manager)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior
- [ ] Goal Validation table filled with concrete evidence
