# Spec: managed-server-hardening

| Field | Value |
|-------|-------|
| Status | done |
| Scope | config |
| Priority | P1 |
| Release reason | Trust repairs are historical; the config-change delivery proof (AC-4) and the two device-side guards of fetch and push (AC-5, AC-6) are correctness defects of shipped managed code. |
| RFC gaps | - |
| Depends | - |
| Phase | 3/3 |
| Handoff | - |
| Updated | 2026-11-20 |
| Blocker | - (the AC-4 writer blocker was cleared by the owner on 2026-11-19, see "AC-4 config-changed: no operator-reachable writer") |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. The managed-hub-server record (retired with the learned corpus) - the managed server this hardens
4. `internal/component/plugin/server/managed_serve.go`, `internal/component/managed/client.go`
5. (2026-11-19) "Owner Decisions (2026-11-19)", "Existing Mechanisms", the AC-4..AC-6 rows, "Key Design Decisions"; then `cmd/ze/hub/managed.go`, `internal/component/config/storage/pointer.go` `StageMachineCandidateOver`, `pkg/plugin/rpc/mux.go` `CallRPC`

## Task

This spec owns secure trust for the managed-config hub, a port-collision doctor
check, and a two-daemon proof of both config fetch and change notification.
The trust path, doctor check and fetch proof are implemented, as recorded in
Remaining Work. AC-4's change-notification half remains blocked on the
operator-reachable writer described below.

1. **Secure server-cert verification.** Keep the implemented PKI certificate
   path and client CA verification. The original self-signed-only listener
   required `tls-insecure`; `startManagedServer` now supplies the named
   certificate resolver and local authority to the listener.
2. **Port-collision doctor check.** When a `server` block with `client` entries shares an address
   with the plugin acceptor's bound block, the managed listener cannot bind (handled gracefully:
   it is skipped with an Error log), but managed clients on that block are then dropped by the
   plugin acceptor. Add a `ze doctor` check that flags a server block declaring BOTH plugin usage
   and `client` entries (or a managed block whose address collides with the acceptor).
3. **Two-instance daemon `.ci`.** Add a `test/managed/*.ci` that runs a real `ze` hub serving a
   real `ze` managed client end-to-end (fetch + config-changed), complementing the Go integration
   test `cmd/ze/hub/managed_server_test.go` which exercises `startManagedServer` directly.
4. **Device-side guards of config fetch and change delivery (added 2026-11-19 by the closure of
   spec-fleet-7-config-reconnect-resolution).** Two defects of the managed client, both one half
   of a pair fleet-7 guarded on its other half (Owned journal rows): an ordinary hub push on a
   device that is not holding overwrites a local commit that lands during its fetch, because only
   the revert stages through the under-lock check (`ClientConfig.OnRevert`,
   `storage.StageMachineCandidateOver`); and `FetchConfig` carries no deadline, unlike the
   bounded `config-held` and `config-ack`, so a hub that never answers a fetch holds the device's
   notification loop. Each needs a design of its own: the push must carry the version it was
   decided over through `OnCommit`'s callers, and the fetch bound must admit a 16 MiB answer while
   the loop stays answerable within the hub's `ManagedPushTimeout`. Note for AC-4: fleet-7's
   `test/managed/fleet-divergence-adopt.ci` drives `config-changed` from a write through the
   running hub (`update fleet divergence name <client> adopt version <hash>`), an operator-reachable
   writer, so the AC-4 blocker below should be judged again.
5. **Design of the remainder (added 2026-11-19, owner decisions at the design gate).** The AC-4
   blocker is cleared: three operator-reachable writers of a client's config exist on the running
   hub, so AC-4 owes one functional test of the documented `ze data import` path. The push race
   (AC-5) is fixed by staging every hub push, not only a revert, through the under-lock check, with
   one guarded commit callback replacing `OnCommit` and `OnRevert`. The fetch (AC-6) takes one fixed
   deadline of 30 seconds. The decisions, their rejected alternatives and the design are in
   "Owner Decisions (2026-11-19)" and the sections after it.

### Owner Decisions (2026-11-19)

| # | Date | Decision | Rejected alternatives, and why |
|---|------|----------|--------------------------------|
| OD-1 | 2026-11-19 | AC-4's blocker is cleared. The running hub has operator-reachable writers of a client's config: `ze data import client-<name>.conf` (the path `docs/guide/fleet-config.md` "Change a Client's Configuration" documents), the hub-side `ze config edit client-<name>.conf` (`test/managed/fleet-hub-editor-declared.ci`), and fleet-7's adopt (`test/managed/fleet-divergence-adopt.ci`, which proves config-changed delivery to a live device but whose fetch answers `current`). AC-4 owes one functional test: a running hub with a connected client, `ze data import` of a changed `client-<name>.conf` on the hub, and the device fetches and applies it with no reconnect | Option (a) of 2026-09-05, a new hub-side write command: not needed, the import through the daemon is that command. Option (b), the editor taking a second path: already built (`fleet-hub-editor-declared.ci`) |
| OD-2 | 2026-11-19 | AC-5: the ordinary push stages through the same under-lock check as the revert. The fetch is decided against the active version the device read before it (`fetchAgainstBaseline`); the push's stage refuses, under the config lock, when the active configuration is no longer that version (as `revertCheck` and `storage.StageMachineCandidateOver` do for a revert); the device then fetches once more over the newer edit (as `refetchOverLocalCommit` does, with its once-only bound), and that fetch sees active differ from baseline, holds and uploads its copy (fleet-7). One guarded commit callback replaces `ClientConfig.OnCommit` and `ClientConfig.OnRevert`, and `cmd/ze/hub/managed.go` wires it | Holding the config lock across the fetch: a network round-trip under the lock blocks every local commit for up to the fetch bound. Stage, then roll back when a local commit is seen: not atomic, a commit can land between the stage and the look |
| OD-3 | 2026-11-19 | AC-6: one fixed fetch deadline of about 30 seconds, sized for a 16 MiB answer on a slow link, applied at connect (`runConnection`) and on the notification loop (`handleConfigChangedRequest`). A fetch that runs longer than the hub's `ManagedPushTimeout` (10 s) can cost the hub's next config-changed push: the hub then closes the connection and the device fetches again when it reconnects, which is the self-healing the owner accepted | Fetch off the loop in a worker: a new goroutine lifecycle and notification coalescing. A bound that fits inside 10 s (about 2 s): breaks every large configuration on an ordinary link |
| OD-4 | 2026-11-19 | Status is `design` now, and moves to `ready` only when the main thread reports the owner approved the final spec | - |
| OD-5 | 2026-11-19 | D-3 is owner-confirmed: the no-baseline fetch path of `fetchAndProcess` and `ClientConfig.Version` are deleted, and `RunManagedClient` refuses to start without a `Baseline` | Keeping the path unguarded: it is the overwrite AC-5 removes, reachable by any new caller |
| OD-6 | 2026-11-19 | First boot takes the same 30 s fetch bound: in `fetchInitialConfig` (`cmd/ze/ze_core_start.go`), `ze.managed.connect.timeout` bounds the dial, the TLS handshake and the authentication only, and the first `config-fetch` is bounded by `configFetchTimeout` like every other fetch (one fetch bound everywhere) | Leaving first boot at 5 s: a large first configuration on an ordinary link could never be fetched |
| OD-7 | 2026-11-19 | The owner approved the final spec; Status moves to `ready` | - |

### Certificate decision (2026-08-29)

Two shapes were on the table for AC-1/AC-2. **The hub serves a pki store
certificate (shape a).**

| Shape | Verdict |
|-------|---------|
| (a) The hub serves a certificate from the pki component, and the client verifies it | **Picked.** `pki.ServerTLSMaterial` already resolves a certificate NAME into serving PEM, and the web, DoT and DoH listeners already take it that way. The managed listener was the only TLS server in ze that could not be given a certificate. The hub gains a `certificate` leaf and nothing else |
| (b) The hub keeps a generated self-signed certificate and persists it, and the client pins its fingerprint | **Rejected.** Persisting the generated pair means a second certificate store beside pki: key material, permissions, regeneration when it expires, and a command to read the fingerprint back. It also leaves the operator with a certificate no CA issued and no way to rotate it through config |

The August 29 design also proposed a client fingerprint pin. The implementation
recorded on September 5 instead uses `plugin/hub/client/ca` and
`ClientConfig.CA`. `clientTLSConfig` resolves that named PKI CA and refuses an
unresolved name; with no named CA it uses the system trust pool unless the
operator explicitly selects `TLSInsecure`. The current contract and remaining
proof use that CA path.

The defect this closes: `NewManagedServer` always minted a 24-hour self-signed
certificate whose only SAN was 127.0.0.1, and `runConnection` verified against
the system CA pool. No real deployment could connect without
`ze.managed.tls.insecure`, and with it the client sent its token to whatever
answered on the hub address.

### Post-wave corrections (2026-07-10)

New obligation from the 2026-07 implementation wave (verified against current code): the
plugin RPC connection layer both managed endpoints use gained write-timeout behavior.
`pkg/plugin/rpc/conn.go` applies a default 30s write deadline when the context carries none
(`defaultWriteDeadline`, conn.go; applied in `writeAppended`, conn.go, :309) and
arms a fail-fast write watchdog on transports without `SetWriteDeadline` (armed in `NewConn`
at conn.go; `fireWatchdog` closes the connection, conn.go), with the
`ze_plugin_write_watchdog_total` counter wired in
`internal/component/plugin/server/server.go` and documented in
`docs/plugin-development/metrics.md`.

Relevance to this spec: the managed client wraps its TLS conn in `rpc.NewConn` at
`internal/component/managed/client.go` and the hub's managed listener does the same at
`internal/component/plugin/server/managed_serve.go`. Both are deadline-capable TLS
`net.Conn`s, so they take the 30s-deadline path and the watchdog timer never arms for them
(transport selection at conn.go); the new counter therefore does not observe managed
connections. Obligations: (1) the cert-verification rework (AC-1/AC-2) must keep wrapping the
verified TLS conn in `rpc.NewConn` so the deadline behavior is preserved; (2) the
two-instance daemon `.ci` (AC-4) will implicitly exercise the deadline write path end to end,
and a hang there should be read against this new fail-fast behavior rather than assumed to be
an indefinite block.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/fleet-config.md` - security section (cert verification default), roles
  → Constraint: cert verification is the documented default; the current self-signed cert breaks it.
- [ ] `docs/architecture/api/process-protocol.md` - the plugin hub's own TLS model, declared as the design document by `internal/component/plugin/types.go`
  → Constraint: "Plugin Transport / External Plugins (TLS connect-back)" has the engine create a `PluginAcceptor` holding the certificate authority root that issued the served certificate, and has the forked child validate the engine chain against that root, carried to it as `ZE_PLUGIN_CA_PEM`. The managed listener's trust decision is a second answer to the same question, so this spec states where it diverges rather than inventing a model.
- [ ] The managed-hub-server record (retired with the learned corpus) - what was built and why the dedicated listener

### RFC Summaries (MUST for protocol work)
- [ ] N/A - TLS/cert handling, not an IETF wire protocol.

- [ ] `docs/architecture/fleet-config.md` - "The baseline and the hold", the revert paragraph ("applies it through `ClientConfig.OnRevert`"), "Connection Lifecycle", "Config Change (Two-Phase)", "Config Storage (Hub Side)" (added 2026-11-19)
  → Constraint: a revert stages through `storage.StageMachineCandidateOver`, which re-reads the active configuration under the config's writer lock and stages nothing unless it still hashes to the version the revert discards; a local commit that lands during the fetch wins, and the device fetches once more naming that edit (`refetchOverLocalCommit`), never a third time. The push must reuse exactly this, not a second mechanism.
  → Constraint: a confirmed-commit window open on the device refuses a push through `confirm.WriteOutside` before any stage; the client acks `ok:false` with the window's refusal and a later fetch delivers the push again. The guarded push keeps this order.
  → Constraint: "One upload waits at most 5 seconds (`heldUploadTimeout`), below the hub's 10-second bound on a `config-changed` round-trip (`ManagedPushTimeout`)". The fetch deadline is the first loop bound that does not stay below it, and the page must say so and why.
  → Decision: "Config Storage (Hub Side)" already states that a running hub takes a client config through `ze data import`; the page is not wrong on AC-4, the guide's "Change a Client's Configuration" is the claim AC-4's test proves.
- [ ] `docs/guide/fleet-config.md` - "Change a Client's Configuration", "When a Client Holds", "Troubleshooting" (added 2026-11-19)
  → Constraint: the guide promises that after `ze data import client-edge-01.conf` "the hub tells `edge-01` at once" and the client fetches, checks, applies and tells the hub; AC-4's test asserts exactly that sequence through two daemons.
- [ ] `ai/doors/hub-engine.md`, `ai/doors/plugin-infra.md`, `ai/doors/config-yang.md`, `ai/doors/core.md`, `ai/doors/le-tooling.md` (added 2026-11-19)
  → Constraint: one reload path: a managed push reaches `doReload` through `wireManagedCommit`; the guarded push changes only the stage call inside it, never adds a second apply path.
  → Constraint: a field or method name in `pkg/plugin/rpc` and `pkg/fleet` is wire; this design adds none and renames none (see Changed-Wire Census).

**Key insights:**
- The managed client verifies against a named PKI CA or the system trust pool; the plugin SDK's fingerprint mechanism is a separate surface.
- (2026-11-19) The only production builder of `managed.ClientConfig` with a fetch role, `extractManagedClientConfig` (`cmd/ze/ze_core_start.go`), always sets `Baseline`, so every production fetch runs `fetchAgainstBaseline`; the `cfg.Baseline == nil` branch of `fetchAndProcess` is reached only from tests.
- (2026-11-19) The push race is the revert race with the check missing: `holdOrApply` applies through `applyFetched` and `cfg.OnCommit`, which stages with `storage.StageMachineCandidate`, no check, so a local commit between `access.read()` and the stage is overwritten.
- (2026-11-19) `FetchConfig` calls `mc.CallRPC` with the connection's context, which carries no deadline at connect or on the loop; `pkg/plugin/rpc` then waits for ever (the 30 s default deadline bounds only the request write).

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/server/managed_serve.go` - `managedCertificate` serves a named certificate through the injected resolver, or a leaf issued by the injected authority; it refuses missing material.
- [ ] `internal/component/managed/tls.go` - `clientTLSConfig` verifies against a named PKI CA, otherwise uses explicit insecure mode or the system trust pool.
- [ ] `cmd/ze/hub/managed_server.go` - `startManagedServer` supplies the certificate resolver and authority and installs the client-config write observer.

**Behavior to preserve:**
- The dedicated managed listener, per-client secret auth, config-fetch/ack/ping, and config-changed push all continue to work unchanged.
- `tls-insecure` remains a valid opt-in for development.

**Source files read (added 2026-11-19, for AC-4, AC-5, AC-6):**
- [ ] `internal/component/managed/client.go` - `fetchAndProcess` takes `fetchAgainstBaseline` whenever `cfg.Baseline` is set, and otherwise fetches with `cfg.Version` and applies every answer with no comparison. `fetchAgainstBaseline` reads active and baseline once (`access.read()`), fetches with the active hash, and on an answer with a config either applies a revert (`applyReverted`, through `OnRevert` with `revertCheck(active)`) or calls `holdOrApply`, which holds when active differs from baseline and otherwise calls `cfg.reportHold(false)` and `applyFetched`, which commits through `cfg.OnCommit` with no check. `applyCommitted` answers `errActiveChanged` as is, with no ack. `refetchOverLocalCommit` fetches once more with `refetchNone`, and a second refusal answers `errActiveChangedTwice`. `FetchConfig` calls `mc.CallRPC` with the caller's context and no bound of its own, unlike `SendConfigAck` (`configAckTimeout`, 3 s) and `uploadHeld` (`heldUploadTimeout`, 5 s). At connect a fetch error ends `runConnection` (the connection closes and `RunManagedClient` reconnects with backoff); on the loop `handleConfigChangedRequest` answers the config-changed with `SendOK` first, then fetches, logs a fetch error and keeps the connection.
- [ ] `cmd/ze/hub/managed.go` - `wireManagedCommit` sets `OnCommit` to stage with `storage.StageMachineCandidate` (no check) and `OnRevert` to stage with `storage.StageMachineCandidateOver` (check under the lock), both inside `confirm.WriteOutside` and `applyManagedPush` (stage, reload, `recordPushBaseline`, audit).
- [ ] `cmd/ze/ze_core_start.go` - `extractManagedClientConfig` is the one production `managed.ClientConfig` that runs fetches, and it sets `Baseline: managed.NewBaselineAccess(store)`; the other construction (`fetchInitialConfig`) builds a `ClientConfig` only for `managed.ClientTLSConfig`, and its first-boot `managed.FetchConfig` runs under one context bounded by `ze.managed.connect.timeout` (default 5 s), documented in `docs/architecture/fleet-config.md` ("the first fetch included").
- [ ] `internal/component/config/storage/pointer.go` - `StageMachineCandidateOver` takes the config lock, reads the active configuration (`ReadActiveConfigWithGuard`), hands it to check, and stages only when check accepts; check MUST NOT be nil and MUST NOT use the store. `StageMachineCandidate` stages with no check and stays for its other callers (`config_source.go`, `confirm_window.go`).
- [ ] `internal/component/plugin/server/managed_serve.go` - `pushConfigChanged` calls `config-changed` under `ManagedPushTimeout` (10 s); on any failure other than server stop it closes the client's connection and logs that the client fetches when it reconnects (`notifyDropped`). The push worker keeps one push in flight per client (`pending`, `pushing`).
- [ ] `pkg/plugin/rpc/mux.go`, `pkg/plugin/rpc/conn.go` - `CallRPC` waits on the response, the context, or the reader's end; on the context it deletes its pending entry. A response that arrives later finds no entry and `discardOrphanResponse` logs "mux conn: orphaned response" at Warn and counts it toward `maxConsecutiveBadLines` (100, consecutive, reset by any well-formed line); the connection is kept. A request write takes the context deadline, else `defaultWriteDeadline` (30 s). Inbound requests queue in a 16-entry channel; a request that cannot be queued within a second is dropped with a Warn.
- [ ] `internal/component/config/storage/cli/data_rpc_config.go`, `internal/component/config/storage/cli/main.go` - with the daemon running, `ze data import` is sent to the daemon (`importConfigsThroughDaemon`), which writes a config that is not its own through `importOtherConfig` and `storage.RestoreConfig` on the daemon's store; nothing reloads.
- [ ] `cmd/ze/hub/managed_server.go` - `startManagedServer` installs the store's write observer, which maps a written key to a client name (`pluginserver.ClientNameFromConfigKey`) and calls `NotifyConfigChanged`.
- [ ] `internal/test/fixture/storage_live_client_fixture.go` - `test/managed/data-restore-client-live.ci` already drives config-changed between two real daemons from `request data restore ... config client edge-01`, and asserts the client's active config changes within 10 s; it asserts nothing about reconnects or the baseline.

**Behavior to preserve (added 2026-11-19):**
- A revert keeps its semantics exactly: applied only when `discard-version` is the active hash, staged over the check, refetched once over a local commit, `errActiveChangedTwice` on a second change (`TestDiscardHoldsOverLocalCommitDuringFetch`, `TestDiscardFetchesAgainOnlyOnce`).
- A confirmed-commit window open on the device refuses every push before any stage, with the window's refusal in an `ok:false` ack (`TestWireManagedCommitRefusedDuringConfirmWindow`).
- A push whose reload fails clears the candidate and leaves the baseline (`TestWireManagedCommitClearsCandidateOnReloadFailure`, `TestBaselineUnchangedWhenPushFails`); a push that applied records the baseline read back from the store (`TestBaselineWrittenAfterPush`).
- `heldUploadTimeout` + `configAckTimeout` stays below `ManagedPushTimeout` (`TestHeldUploadDeadlineBelowPushBound`).
- The first-boot fetch keeps its `ze.managed.connect.timeout` bound. (Superseded 2026-11-19 by OD-6: that bound keeps the dial, the handshake and the authentication; the fetch takes `configFetchTimeout`.)

**Behavior to change (added 2026-11-19):**
- An ordinary hub push stages over the check of the version it was decided on; a local commit that lands during its fetch wins, and the device fetches once more and holds over that edit (AC-5).
- `ClientConfig.OnCommit` and `ClientConfig.OnRevert` are replaced by one callback, `ClientConfig.CommitOver`, whose stage always runs the check (AC-5).
- The `cfg.Baseline == nil` branch of `fetchAndProcess`, with `ClientConfig.Version`, is deleted: `Baseline` becomes required (decision D-3 below) (AC-5).
- Every fetch of a running client is bounded by `configFetchTimeout` (30 s) (AC-6).
- (OD-6) The first-boot fetch in `fetchInitialConfig` is bounded by `configFetchTimeout` and no longer by `ze.managed.connect.timeout`, which keeps bounding the dial, the TLS handshake and the authentication of that connection (AC-6).

**Behavior to change:**
- A managed client can verify the hub's identity without `tls-insecure` (CA cert or pinned fingerprint). DONE 2026-08-29.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| The revert's guarded commit | `internal/component/managed/client.go` `ClientConfig.OnRevert`, `applyReverted`, `revertCheck`; `cmd/ze/hub/managed.go` `wireManagedCommit` | A revert commits through `OnRevert(data, check)`, and the daemon stages with `storage.StageMachineCandidateOver`, which runs check on the active configuration under the config lock | Extends: the guarded commit becomes the one commit callback, `CommitOver`, used by the push and the revert alike; `revertCheck` becomes `activeCheck`, the check of the version a fetch was decided on |
| The ordinary push commit | `ClientConfig.OnCommit`, `applyFetched`; `wireManagedCommit` staging with `storage.StageMachineCandidate` | Commits the hub's answer with no check | Replaced: deleted, with every caller and fake (census in Implementation Steps), and replaced by `CommitOver` (`ai/rules/no-layering.md`) |
| The under-lock stage | `internal/component/config/storage/pointer.go` `StageMachineCandidateOver` | Reads active under the config lock, hands it to check, stages only on acceptance; candidate-exists refuses a second stage | Extends its use to the push; unchanged itself. `StageMachineCandidate` keeps its other machine writers |
| The refetch over a local commit | `client.go` `refetchOverLocalCommit`, `revertRefetch` (`refetchOnce`, `refetchNone`), `errActiveChangedTwice` | After a refused revert, fetches once more with the new active hash and decides that answer as any fetch; a second refusal ends the fetch | Extends: a refused push takes the same path with the same once-only bound; the log line and the two error texts stop naming the revert alone |
| The hold and its copy | `client.go` `holdOrApply`, `holdFetched`, `uploadHeld`, hold ack (fleet-7) | A device whose active differs from its baseline applies nothing, uploads a copy (`config-held`) and acks `holding` | Applies unchanged: the refetch after a refused push lands here, since the local edit makes active differ from baseline. `holdOrApply` stops reporting "not holding" before the apply (see Data Flow) |
| The bounded loop round-trips | `client.go` `heldUploadTimeout` (5 s), `configAckTimeout` (3 s) | Bound the upload and the ack, each by a context deadline that is also the write deadline, and together below `ManagedPushTimeout` | Extends the pattern with `configFetchTimeout` (30 s) beside them; unlike them it is deliberately not below the hub's push bound (OD-3), and its comment says so |
| The hub's push bound and drop | `internal/component/plugin/server/managed_serve.go` `ManagedPushTimeout`, `pushConfigChanged` | A config-changed unanswered within 10 s closes the client's connection; the client fetches when it reconnects | Does not change: it is the self-healing OD-3 relies on; its comment ("A managed client bounds every round-trip that keeps its notification loop busy below it") is corrected to name the fetch as the exception |
| The orphaned-response guard | `pkg/plugin/rpc/mux.go` `routeResponse`, `discardOrphanResponse`, `maxConsecutiveBadLines` | A response whose caller gave up is logged and dropped, the connection kept | Does not change: it is what a deadline-expired fetch leaves behind (R-6) |
| `ze data import` on a running hub | `internal/component/config/storage/cli/main.go` `importConfigsThroughDaemon`; `data_rpc_config.go` `importOtherConfig`, `storage.RestoreConfig` | Writes `client-<name>.conf` as a new active version through the daemon's store | Does not change: it is AC-4's writer, driven by the new functional test |
| The write observer | `cmd/ze/hub/managed_server.go` `startManagedServer` (`store.SetWriteObserver`), `pluginserver.ClientNameFromConfigKey`, `NotifyConfigChanged` | Turns a write of a client's config key into a config-changed push to that client | Does not change: AC-4's test proves it is reached from the import |
| The two-daemon fixture harness | `internal/test/fixture/misc_fixture_managed_ca_trust.go` (`caTrustDaemon`, `caTrustStart`, `caTrustImportConfig`, `caTrustPollActive`), `misc_fixture_managed_fleet_hold.go` `fleetHoldRun`, `misc_fixture_managed_fleet_status.go` `fleetStatusAnswer` | Starts a real hub and a real device, imports configs, polls the device's active config and `show fleet status` | Extends: the AC-4 fixture is built on it, not on a new harness |
| The live restore proof | `test/managed/data-restore-client-live.ci`, `storageLiveRestoreClient` | Proves config-changed delivery from `request data restore` between two daemons | Does not apply as AC-4's proof: it drives another writer and asserts neither the reconnect count nor the baseline; it stays as is |

## Data Flow (MANDATORY)

### Entry Point
- Managed client TLS dial to the hub's managed listener (`client.go` `runConnection`).

### Transformation Path
1. The hub serves the named PKI certificate or a leaf issued by its authority.
2. The client resolves its configured PKI CA, or uses the system trust pool.
3. TLS verifies the chain and server name before the client sends its token, unless insecure mode was explicitly selected.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Hub cert material ↔ managed listener | PKI/cert-store load or config-provided cert | [ ] |
| Hub cert identity ↔ client trust | CA chain through the client's named PKI CA or system trust pool | [ ] |

### Entry Points (added 2026-11-19)
- AC-4: the operator's `ze data import client-<name>.conf` beside a running hub, a file whose name is the client's config key.
- AC-5: a `config-fetch` answer carrying a configuration, at connect (`runConnection` to `fetchAndProcess`) or after a `config-changed` (`handleConfigChangedRequest` to `fetchAndProcess`), on a device whose active configuration equals its baseline, while a local commit lands on the device's store.
- AC-6: a `config-fetch` call to a hub that keeps the connection but never answers it, at the same two call sites.

### Transformation Path (added 2026-11-19)

AC-4, the import reaching the device:
1. `ze data import` finds the store busy and sends the file to the hub daemon (`importConfigsThroughDaemon`).
2. The hub writes it as a new active version of `client-<name>.conf` (`importOtherConfig`, `storage.RestoreConfig`).
3. The store's write observer maps the key to the client name and calls `NotifyConfigChanged` (`startManagedServer`).
4. A push worker sends `config-changed` under `ManagedPushTimeout` (`pushConfigChanged`); the device answers it at once (`SendOK`) and fetches.
5. The device's fetch is decided by `fetchAgainstBaseline`: active equals baseline, so `holdOrApply` applies through the guarded commit; the reload makes it active and `recordPushBaseline` records its hash as the baseline; the device acks `ok:true`.

AC-5, the guarded push (the state after this spec):
1. `fetchAgainstBaseline` reads active A and baseline B once (`access.read()`), and fetches with A's hash.
2. The hub answers configuration H. A equals B, so `holdOrApply` calls `applyFetched` with A, and reports no hold yet.
3. `applyFetched` commits through `CommitOver(data, activeCheck(A))`. The daemon (`wireManagedCommit`) runs it inside `confirm.WriteOutside`, then `applyManagedPush`, whose stage is `storage.StageMachineCandidateOver`: under the config lock it reads the active configuration and hands it to the check.
4. When no local commit landed, the active configuration still hashes to A: the stage writes H as the candidate, the reload promotes it, `recordPushBaseline` records H's hash, the ack is `ok:true`, and `holdOrApply` reports no hold.
5. When a local commit L landed after step 1, the check refuses with `errActiveChanged`; nothing is staged; `applyCommitted` answers the refusal with no ack.
6. `holdOrApply` passes the refusal to `refetchOverLocalCommit` with the call's `revertRefetch`: on the first fetch it fetches again with L's hash (`refetchNone`); L differs from B, so the device holds: `uploadHeld` sends a copy of L (`config-held`) unless the hub already holds it, and `holdFetched` acks `holding` with both hashes. The hold is reported once, as holding.
7. On the fetch made again, a further refusal ends the call with `errActiveChangedTwice`: at connect (`runConnection`) and on the loop (`handleConfigChangedRequest`) alike it is logged once, at Info (`logLocalCommitTwice`), and ends the connection; `RunManagedClient` reconnects with its normal backoff and logs no "connection lost" Warn for it. The reconnect's fetch decides again over what the device then runs: a device that still differs holds, uploads its copy and acks holding, and the hold the second fetch reports comes from the store read again, so a device back on its baseline does not report holding.

AC-6, the bounded fetch:
1. Every fetch `fetchAndProcess` makes goes through one unexported fetch that bounds `CallRPC` with `configFetchTimeout` (30 s), whatever deadline the caller's context carries, as `SendConfigAck` bounds the ack. The bound is the context deadline and so also the request's write deadline.
2. At connect, a fetch that reaches the deadline returns `fetch: ...context deadline exceeded`; `runConnection` returns it, the connection closes, and `RunManagedClient` reconnects with backoff (the existing fetch-failure path).
3. On the loop, the same error is logged by `handleConfigChangedRequest` ("fetch after notification failed") and the connection kept; the loop returns to `mc.Requests()` and answers the next `config-changed`.
4. A `config-changed` the hub sent while the fetch waited sits in the request channel. When the fetch waited longer than `ManagedPushTimeout`, the hub's call has already failed: `pushConfigChanged` closes the connection, and the device's next connection fetches again (`runConnection` to `fetchAndProcess`).
5. The hub's answer, if it arrives after the deadline, finds no pending call and is dropped as an orphaned response (`discardOrphanResponse`), the connection kept.

### Boundaries Crossed (added 2026-11-19)
| Boundary | How | Verified |
|----------|-----|----------|
| Operator CLI ↔ hub daemon | `ze data import` over the daemon route (SSH), unchanged | No |
| Hub store ↔ managed server | write observer callback, unchanged | No |
| Hub ↔ device | `config-changed`, `config-fetch`, `config-held`, `config-ack` over the managed TLS listener, unchanged messages | No |
| Managed client ↔ device daemon | the `CommitOver` callback, a Go function value set by `wireManagedCommit` in the same process | No |
| Device daemon ↔ config store | `storage.StageMachineCandidateOver` under the config lock | No |

### Changed-Wire Census

This design changes no wire format. It adds, removes and renames no fleet verb, envelope type, field or JSON tag, and it changes no encoding. What changes on the wire is only when existing messages are sent: a refused push sends no `config-ack` (as a refused revert already does, `applyCommitted`), the device sends a second `config-fetch` on the same connection (as the revert's refetch already does), then the existing fleet-7 `config-held` and holding `config-ack`; a fetch that reached its deadline leaves the hub's answer to arrive and be discarded by the device, and the hub sees the device's next fetch. `ClientConfig.CommitOver` is a Go callback inside one process, not a wire.

**Search evidence:** scope `cmd`, `internal`, `pkg`, `test`, `scripts`, Go, Python, shell, YANG and `.ci` files; terms `config-fetch`, `config-changed`, `config-held`, `config-ack`, `fleet.ConfigFetchRequest`, `fleet.ConfigFetchResponse`, `fleet.ConfigAck`, `fleet.ConfigHeld`. Carriers found: the definitions in `pkg/fleet/envelope.go`; the device writer and reader in `internal/component/managed/client.go` (`FetchConfig`, `SendConfigAck`, `uploadHeld`, `handleConfigChangedRequest`) and `handler.go`; the hub reader and writer in `internal/component/plugin/server/managed_serve.go`, `managed.go`, `managed_held.go`, `managed_doctor.go`; the first-boot reader in `cmd/ze/ze_core_start.go` (`fetchInitialConfig`); fixtures under `internal/test/fixture/` and `.ci` files under `test/managed/`. No Python or shell carrier exists. Positive control: `fleet.VerbConfigFetch` is found at its known writer (`FetchConfig`) and its known reader (the hub's `case fleet.VerbConfigFetch` in `managed_serve.go`). None of these files changes a type, a field or an encoding in this design.

### Integration Points
- `ManagedServerConfig`, `managed.clientTLSConfig`, the hub PKI store, and the `plugin/hub/client/ca` leaf are implemented. The remaining integration is an operator-reachable write through the running hub's store.
- (2026-11-19) `wireManagedCommit` (`cmd/ze/hub/managed.go`) sets `CommitOver`; `applyManagedPush` keeps its stage, reload, baseline and audit steps, with the stage now always `storage.StageMachineCandidateOver`.
- (2026-11-19) `managedClientOwner` (`cmd/ze/hub/managed_owner.go`) applies `wireManagedCommit` to each client it starts; its comment naming `OnCommit, OnRevert` changes.

### Architectural Verification (added 2026-11-19)
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | The push still reaches the daemon through `wireManagedCommit`, `confirm.WriteOutside`, `applyManagedPush` and `doReload`; only the stage call changes |
| No unintended coupling | Yes | The managed component still knows nothing of storage: the daemon supplies the stage; the check is a pure function of the active bytes |
| No duplicated functionality | Yes | The push reuses the revert's check, stage and refetch; the old unchecked path is deleted |
| Zero-copy preserved where applicable | N-A | No buffer or encoding path changes |
| Registration over hardcoding, outbound | N-A | No command, view, family or handler is added |
| Registration over hardcoding, inbound | N-A | No name is introduced that a list, switch or table would have to learn |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | No production path builds a fetching `managed.ClientConfig` with `Baseline` nil, so deleting that branch changes no shipped behavior | `extractManagedClientConfig` (`cmd/ze/ze_core_start.go`) is the one production builder that runs fetches and sets `Baseline`; `fetchInitialConfig` builds one for TLS only; `grep` of `ClientConfig{` outside tests finds these two | A device would refuse to run its client after the change | `grep -rn 'ClientConfig{' cmd internal pkg` excluding tests, repeated at implementation; `TestRunManagedClientRefusesNoBaseline` | confirmed 2026-11-19 (phase 1 re-check: `grep -rn 'ClientConfig{'` outside tests finds only `cmd/ze/ze_core_start.go` `extractManagedClientConfig` (sets `Baseline`) and `fetchInitialConfig` (TLS only)); `TestRunManagedClientRefusesNoBaseline` written and green 2026-11-19 (`./le test unit pkg path ./internal/component/managed/` exit 0, main thread) |
| A-2 | The configuration the client reads as active (`resolve.DefaultConfig(store)` in `activeConfigStamped`) is the one the daemon's stage reads (`configPath` in `wireManagedCommit`) | Fleet-7's revert relies on the same identity, and `TestDiscardHoldsOverLocalCommitDuringFetch` and the `fleet-divergence-revert.ci` pass over it | The check would compare two different configurations and refuse every push | `TestWireManagedCommitOverRefusesLocalCommit` stages over the same store the client reads; the AC-4 `.ci` applies a push end to end | confirmed 2026-11-19: `./le test managed -a` 32/32 on a fresh build at 23:34, including `fleet-config-change-live` and fleet-7's revert and hold tests, which cross the seam between `resolve.DefaultConfig` and `configPath` end to end; `TestWireManagedCommitOver*` green under `go test -race` in `cmd/ze/hub` (main thread) |
| A-3 | A running managed device always has a stored active configuration when its client fetches, because boot writes it (cached boot or `fetchInitialConfig`) before `RunManagedClient` starts | `cmd/ze/ze_core_start.go` boot order; `StageMachineCandidateOver` refuses when `ReadActiveConfigWithGuard` fails | A device with no active configuration would refuse every push with "read the active configuration" | Read the boot order at implementation; `TestGuardedPushWithNoStoredActive` pins what the stage answers then (the push is refused with that error and the device stays connected) | confirmed 2026-11-19 by source read (phase 1): `cmdStartManaged` runs `hub.RunWithManagedClient` only when `store.Exists(configName)`, or after `cacheFirstConfig` wrote the fetched config on first boot; `TestGuardedPushWithNoStoredActive` written and green 2026-11-19 under `go test -race -run 'Managed|WireManaged|Baseline|Guarded|MachineWriters|Owner' ./cmd/ze/hub/` (main thread; that run's one failure, `TestConfiguredPluginConformanceKeepsServerOwnership`, is another session's) |
| A-4 | 30 seconds admits a full 16 MiB answer (`rpc.MaxMessageSize`) on a link of about 4.5 Mbit/s or faster | 16 MiB × 8 / 30 s ≈ 4.5 Mbit/s; owner decision OD-3 | A slower link never completes a large fetch; the device keeps its configuration and reconnects with backoff | Stated in the constant's comment and in `docs/architecture/fleet-config.md`; no test can prove link speed | validated 2026-11-19 by arithmetic |
| A-5 | The hub sends no request to the device that the device must answer within a bound other than `config-changed` | `managed_serve.go` sends only `config-changed`; it answers the device's `ping` (`case fleet.VerbPing`) and never sends one | A hub request queued behind a 30 s fetch would time out on the hub too | `grep CallRPC internal/component/plugin/server/managed*.go` at implementation | validated 2026-11-19 by source read |
| A-6 | Local commits on a managed device come from machine writers and the operator only while the freeze lets them (the confirmed-commit revert `confirmRevert`, the signal reload of an explicit file `StageSourceCandidate`, an operator change after `meta/instance/managed` is disabled), and every one of them takes the config lock `StageMachineCandidateOver` takes | `storage/pointer.go` `StageMachineCandidate` comment lists the machine writers; `freeze.go` refuses operator stages on a managed device | A writer that bypasses the lock would race the check and win silently | Read each writer's lock at implementation; the unit tests write the local commit through the store's locked write | confirmed 2026-11-20 by source read: `storage.StageMachineCandidate` (used by `confirmRevert`, `cmd/ze/hub/confirm_window.go`), `storage.StageSourceCandidate` (the signal reload) and `storage.WriteCandidateVersion` (the operator route, which asks the freeze) each take `store.AcquireLock(configPath)` before they stage, the lock `StageMachineCandidateOver` holds across its read and stage; `stageCandidateLocked` refuses a second candidate with `ErrCandidateExists`, and `PromoteCandidate`, which changes what is active, takes the same lock |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Concurrent local commits during the second fetch: a commit lands while the refetch over the first local edit is in flight | The hold ack names a version the device no longer runs; `errActiveChangedTwice` in the log when the refetch's answer would have applied | The refetch carries `refetchNone`: a hold stages nothing, so a further edit only makes the copy and ack name the version read at the refetch (`uploadHeld` reports `kept` false when the copy's version is not that one); a refetch whose answer would apply is refused by the same check and ends with `errActiveChangedTwice`, no third fetch; the connection ends, and the reconnect's fetch decides over what the device then runs. Tests: `TestPushFetchesAgainOnlyOnce`, `TestLocalCommitTwiceReconnectsQuietly` |
| R-2 | A confirmed-commit window open on the device during the push | `ok:false` ack carrying the window's refusal | Unchanged: `confirm.WriteOutside` runs before the guarded stage, so the window refuses first and nothing reads or checks the active configuration; the client keeps its version and a later fetch delivers the push again. Test: `TestWireManagedCommitRefusedDuringConfirmWindow`, moved onto `CommitOver` |
| R-3 | A reload or another staged change is in progress during the stage (a candidate already exists) | `commit config: ... candidate exists` in the `ok:false` ack | Unchanged: `stageCandidateLocked` refuses with `ErrCandidateExists` after the check, the push is rejected and acked as today, and the next notification or connection fetches again. It is not `errActiveChanged`, so it does not trigger the refetch. Test: `TestWireManagedCommitOverRefusesStagedCandidate` |
| R-4 | A local commit lands after the stage and before the reload promotes the candidate | A local commit refused with `ErrCandidateExists` | Unchanged: once staged, the candidate refuses every other stage until the reload promotes or clears it (`StageMachineCandidateOver` comment), so the push and the local commit cannot interleave |
| R-5 | The deadline cuts a fetch on a slow but live link | "fetch after notification failed ... context deadline exceeded" on the device; on connect, a reconnect loop with backoff | Accepted by OD-3 for answers above what 30 s carries (A-4). The device keeps the configuration it runs; at connect it reconnects with backoff, on the loop it waits for the next notification. Doc: the bound and its consequence in `docs/architecture/fleet-config.md` and the guide's "Troubleshooting" |
| R-6 | An orphaned late fetch answer arrives after the deadline | "mux conn: orphaned response" at Warn on the device | The answer is dropped, never applied: no caller waits for its id. It counts once toward `maxConsecutiveBadLines` (100 consecutive, reset by the next good line), so it cannot end the connection by itself. A 16 MiB answer is read and discarded, a transient memory cost of one frame. Test: `TestFetchDeadlineDropsLateAnswer` |
| R-7 | The hub closes the session because its `config-changed` waited longer than `ManagedPushTimeout` behind a long fetch | "the push failed ... the hub closed its connection" in the hub log; the device reconnects | Accepted by OD-3: the device fetches again when it reconnects (`runConnection` to `fetchAndProcess`), which delivers the change the lost push announced. Hub side proven by `TestManagedServeUnansweredPushDisconnects`; device side by `TestFetchAtConnectAfterHubDropsPush` |
| R-8 | Reporting the hold before the outcome: `holdOrApply` today reports "not holding" before a push it may now refuse | `show fleet status` flickers from not holding to holding | `holdOrApply` reports the hold after the outcome: not holding after an applied push, and the refetch's report after a refused one. Test: `TestPushHoldsOverLocalCommitDuringFetch` asserts the hold reports exactly one value, holding |
| R-9 | Deleting `ClientConfig.Version` and the nil-`Baseline` branch breaks tests that build a client with neither | Compile errors in `internal/component/managed` tests | Every such test gains a `BaselineAccess` over a test store (`holdDevice`); the three `assert.Empty(t, cfg.Version)` assertions become assertions on the store's active version and baseline, which observe more, and go to `ze-test-judge` because they change what a test asserts |
| R-10 | (amended 2026-11-19 by OD-6) First boot today runs dial, handshake, authentication and fetch under one context of `ze.managed.connect.timeout` (5 s default), so moving the fetch to `configFetchTimeout` could leave the fetch still cut at 5 s if it keeps that context, or leave the connect phase unbounded if the context is dropped | A first-boot fetch of a large configuration failing at about 5 s; a first boot hanging on a silent dial | The connect context bounds only the dial, `HandshakeContext`, `SendAuth` and the auth read; the fetch runs under a context not derived from it, bounded by `configFetchTimeout` inside `FetchConfig`. A silent hub at first boot then fails after 30 s with the fetch's deadline error and no configuration is written. Tests: `TestFirstBootFetchOutlivesConnectTimeout`, `TestFirstBootFetchEndsAtFetchDeadline` |
| R-11 | (phase 2, 2026-11-19) A test that runs the real `RunManagedClient` with no `Baseline` and waits for a callback (`CheckManaged`, `CommitOver`) now returns at the entry refusal, so it goes red or, where it only waits for the return, passes without reaching what it names | `TestRunManagedClientStopsWhenUnmanaged` returning before `CheckManaged`; `TestHubManagedClientJoinsStoredRead` and `TestHubManagedServerStopsItsOwner` timing out on the callback | Every such caller found by `gopls references` and `grep` was given a `BaselineAccess` over a test store: `TestRunManagedClientStopsWhenUnmanaged` (`client_test.go`), the two `runClient` tests of `client_endpoint_test.go`, the two clients of `cmd/ze/hub/managed_lifetime_test.go`, and `TestOwnerStopEndsLiveConnection` (`cmd/ze/hub/managed_owner_test.go`). Stand-in runners (`owner.run`) need none |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A managed device refuses hub pushes it should apply (a false `errActiveChanged`, A-2, A-3), or a device on a slow link cannot take a large configuration (A-4). A device never loses its running configuration: every failure keeps it |
| How is it reverted? | A single commit revert; no config, YANG or wire format changes |
| Who else touches this path? | Fleet-7 (`spec-fleet-7-config-reconnect-resolution`, closed) owns the hold and revert code this extends; the hub editor and import paths are other specs' surfaces, used unchanged |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| managed client connects with cert verification (no tls-insecure) | → | `clientTLSConfig` validates the hub chain against the pki ca the client names | `test/managed/managed-hub-ca-trust.ci` |
| a server block declares managed clients on the plugin acceptor's address | → | `diagnoseManagedListener` reports `doctor-hub-managed-collision` | `test/ui/doctor-hub-managed-collision.ci`, and `test/ui/doctor-hub-managed-separate.ci` for the control |
| (2026-11-19) `ze data import client-edge-01.conf` beside a running hub with edge-01 connected | → | `importOtherConfig`, the write observer, `pushConfigChanged`, the device's `fetchAgainstBaseline`, `CommitOver` wired by `wireManagedCommit` | `test/managed/fleet-config-change-live.ci` |
| (2026-11-19) a hub push fetched while a local commit lands on the device | → | `applyFetched` with `activeCheck`, `refetchOverLocalCommit`, `holdFetched` | `TestPushHoldsOverLocalCommitDuringFetch` (`internal/component/managed/client_held_test.go`), and `TestWireManagedCommitOverRefusesLocalCommit` (`cmd/ze/hub/managed_test.go`) for the daemon's stage |
| (2026-11-19) a hub that never answers `config-fetch`, at connect and after `config-changed` | → | the bounded fetch (`configFetchTimeout`) in `fetchAndProcess` | `TestFetchDeadlineAtConnect`, `TestFetchDeadlineOnNotificationLoop` (`internal/component/managed/client_fetch_deadline_test.go`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Managed client with the hub's CA trusted through the named PKI CA or system pool and `tls-insecure` off | Connects and fetches config; TLS verification passes | `tls-insecure` stays a valid explicit opt-in for development, so a client that selects it still connects without verification |
| AC-2 | Managed client with a wrong/absent trust anchor and `tls-insecure` off | Connection refused (verification fails), not silently accepted | None: every untrusted chain is refused, because accepting one sends the token to an unauthenticated hub |
| AC-3 | A `server` block declares both a shared secret (plugins) and `client` entries | `ze doctor` reports a collision/misconfiguration code | The documented central hub, whose managed listener has its own address, stays silent (`doctor-hub-managed-separate.ci`), because it works |
| AC-4 | (refined 2026-11-19) A running hub with client edge-01 connected and applied (its active hash and baseline equal the hash of the config the hub serves); the operator runs `ze data import client-edge-01.conf` beside the hub with a changed config | The import answers `imported`; within 10 s, well inside the 30 s heartbeat, edge-01's `show fleet status` reports `active-hash` and `baseline-hash` both equal to the hash of the imported config and `diverged` false; the hub log names `managed client applied config` for edge-01 with that version; the hub log holds exactly one `managed client connected` line for edge-01 and no `managed client disconnected` line before the test stops the daemons | The import's own write of a new version and its rollback version on the hub, which is what an import does; the hub's other clients, which receive no push because the change is not theirs |
| AC-5 | (refined 2026-11-19) A managed device whose active configuration equals its baseline fetches an ordinary hub push, and a local commit L lands on its store after it read the active hash and before the stage | The device's configuration stays L; it fetches a second time naming L's hash; it sends the hub a `config-held` copy of L and a `config-ack` with `holding` true; its hold is reported once, as holding. Without the local commit (the control), the same push is staged over the check, made active, recorded as the baseline and acked `ok:true` | A push refused for another cause keeps today's answer (a confirmed-commit window, a staged candidate, a reload failure: `ok:false` with the cause), because those refusals are not a newer local edit. A revert keeps its own semantics unchanged. A third fetch is never made: a second local commit during the second fetch ends the call with `errActiveChangedTwice`; the connection ends, and the reconnect's fetch decides over what the device then runs |
| AC-6 | (refined 2026-11-19) A hub that authenticates the device and keeps the connection open but never answers a `config-fetch`; once at connect, and once on the notification loop after a `config-changed` | At connect: the fetch returns its deadline error after `configFetchTimeout` and the connection closes, and the client dials again with backoff. On the loop: the fetch returns its deadline error after `configFetchTimeout`, the device logs it, keeps the connection, and answers the next `config-changed` with `ok`; its next fetch is sent. A hub answer that arrives after the deadline is dropped and the connection kept. At first boot (`ze start` of a device `ze init --managed` prepared, OD-6): a hub that answers the fetch after more than `ze.managed.connect.timeout` but within `configFetchTimeout` gives the device its configuration, which it writes and boots from; a hub that never answers ends first boot with the fetch's deadline error after `configFetchTimeout` | The device keeps the configuration it runs in the connect and loop cases, because no answer arrived to apply; at first boot it writes none, as any failed first boot does today. A `config-changed` that the hub sent while the fetch waited longer than `ManagedPushTimeout` fails on the hub, which closes the connection, and the device fetches again on reconnect (OD-3): this is the accepted self-healing. `ze.managed.connect.timeout` still bounds the dial and the TLS handshake of every connection, and at first boot the authentication too |

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `plan/journal/guard-added-to-one-half-of-a-pair.md` | 2026-11-19 | Managed device apply of an ordinary hub push (`applyFetched`, `OnCommit`): no under-lock check |
| `plan/journal/guard-added-to-one-half-of-a-pair.md` | 2026-11-19 | Managed device `config-fetch` round-trip (`FetchConfig`): no deadline |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestManagedClientValidatesAgainstConfiguredRoot` (was `TestManagedClientPinsCertificate`, renamed when the fingerprint pin became the named pki CA, 8788a29f8b, 2026-09-04) | `internal/component/managed/client_tls_test.go` | AC-1 | PASS |
| `TestManagedClientRefusesAnotherIssuer` (was `TestManagedClientRefusesWrongCertificate`) | `internal/component/managed/client_tls_test.go` | AC-2 | PASS |
| `TestManagedClientRefusesWithNoAnchor` (was `TestManagedClientDefaultFailsClosed`) | `internal/component/managed/client_tls_test.go` | AC-2 | PASS |
| `TestFingerprintConfigIsRefused`, `TestHubCARejectsAnUnusableName` (replace `TestManagedClientFingerprintSources` and `TestHubFingerprintRejectsNonHex`: the fingerprint leaf is gone, and a fingerprint config is now refused) | `internal/component/config/hub_certificate_test.go` | AC-1 (the CA leaf) | PASS |
| `TestManagedServerServesConfiguredCertificate` | `internal/component/plugin/server/managed_cert_test.go` | AC-1 | PASS |
| `TestManagedServerFailsClosedOnCertificate` | `internal/component/plugin/server/managed_cert_test.go` | AC-1 | PASS |
| `TestHubTrustLeavesReachTheStructs` | `internal/component/config/hub_certificate_test.go` | AC-1 (both leaves) | PASS |
| `TestHubRefusesDisagreeingCertificates` | `internal/component/config/hub_certificate_test.go` | AC-1 (one certificate per managed server) | PASS |
| `TestManagedListenerCollisionCases` | `internal/component/plugin/doctor/check_managed_listener_test.go` | AC-3 (each address case, and the documented central hub that must stay silent) | PASS |
| `TestManagedListenerCollisionNamesBothBlocks` | `internal/component/plugin/doctor/check_managed_listener_test.go` | AC-3 (the message names the block that cannot serve and the one that took the address) | PASS |
| `TestManagedListenerCheckReadsTheConfigTree` | `internal/component/plugin/doctor/check_managed_listener_test.go` | AC-3 (config text through ExtractHubConfig to the verdict) | PASS |
| `TestManagedListenerCheckIgnoresAForeignTree` | `internal/component/plugin/doctor/check_managed_listener_test.go` | AC-3 (no tree, a foreign tree, a typed nil tree) | PASS |
| `TestPushHoldsOverLocalCommitDuringFetch` | `internal/component/managed/client_held_test.go` | AC-5, the race reached deterministically: a device whose active equals its baseline; the fake hub's handler for the first `config-fetch` writes local edit L into the store before it answers a push (as `TestDiscardHoldsOverLocalCommitDuringFetch` does for the revert), and `CommitOver` is the fake stage of `holdClient` (it hands check the active bytes the store holds). Asserts: the first fetch names A's hash, the second names L's hash, a `config-held` carrying L's hash follows, the ack is `holding` true and `ok` false, nothing reached the commit, the store's active version is L, the hold was reported exactly once as holding. MUTATION: commit the push with a check that accepts everything, and the hub's configuration is committed over L | |
| `TestPushAppliesWhenActiveUnchanged` | `internal/component/managed/client_held_test.go` | AC-5 control: the same fetch with no local commit commits the hub's configuration through `CommitOver`, acks `ok:true` and reports not holding; one `config-fetch` only. MUTATION: refuse in `activeCheck` always, and the push never applies | |
| `TestPushFetchesAgainOnlyOnce` | `internal/component/managed/client_held_test.go` | AC-5 survivor and R-1: the refetch reaches a commit only when the device runs its baseline again, so the test makes two local commits around the first stage: the fake hub's handler for the first `config-fetch` writes L1, and the fake `CommitOver`, after its check refuses L1, writes A back (a second local commit restoring the baseline content); the refetch then names A, the device applies, and the handler for the second `config-fetch` writes L2, so the second check refuses too. Asserts: the call answers `errActiveChangedTwice`, exactly two `config-fetch` calls were made, nothing was committed, and the hold is reported as not holding, because the wrapper restores A after both refusals and the hold is read from the store again. MUTATION: pass `refetchOnce` on the refetch, and a third fetch appears | |
| `TestPushRefusalOtherThanLocalCommitAcks` | `internal/component/managed/client_held_test.go` | AC-5 survivor: `CommitOver` refusing with an error that is not `errActiveChanged` (a confirmed-commit window) acks `ok:false` with that error, makes no second fetch, and leaves the device not holding. MUTATION: route every commit error to the refetch | |
| `TestRunManagedClientRefusesNoBaseline` | `internal/component/managed/client_test.go` | D-3: `RunManagedClient` with a `ClientConfig` whose `Baseline` is nil returns at once, dials nothing, and logs one Error naming the missing baseline access. MUTATION: drop the entry check, and the client dials | |
| `TestFetchDeadlineAtConnect` | `internal/component/managed/client_fetch_deadline_test.go` | AC-6 at connect, on the existing fakes (`testRoot`, `installCA`, `answerHub` in `client_hold_test.go` for a TLS hub; `heldPipe` in `client_held_test.go` for an in-memory one, which the three tests below use): a silent hub fake (a TLS listener that answers `#0 auth` with `ok`, answers `ping`, and never answers `config-fetch`) and a client whose test-shortened fetch bound is 200 ms; `runConnection` returns an error wrapping `context.DeadlineExceeded` within the bound plus a margin, and the fake sees the connection close. MUTATION: bound the fetch with the caller's context only, and `runConnection` never returns | |
| `TestFetchDeadlineOnNotificationLoop` | `internal/component/managed/client_fetch_deadline_test.go` | AC-6 on the loop: the fake answers the connect fetch `current`, then sends `config-changed` and never answers the fetch that follows, then sends a second `config-changed` under a push bound longer than the fetch bound; the second call returns `ok`, and the device sends a further `config-fetch` after it. MUTATION: as above, and the second `config-changed` fails on its bound | |
| `TestFetchDeadlineDropsLateAnswer` | `internal/component/managed/client_fetch_deadline_test.go` | R-6: the fake answers the loop's fetch only after the bound, with a configuration; nothing is committed and the connection stays open (a following `config-changed` is answered). MUTATION: keep the pending entry past the deadline, and the late answer is routed | |
| `TestFetchAtConnectAfterHubDropsPush` | `internal/component/managed/client_fetch_deadline_test.go` | R-7, the device half: the fake closes the connection while the device's loop fetch waits; the client reconnects and its first request on the new connection is a `config-fetch`. MUTATION: skip `fetchAndProcess` in `runConnection` | |
| `TestFirstBootFetchOutlivesConnectTimeout` | `cmd/ze/ze_core_start_bootstrap_test.go` | OD-6, AC-6 at first boot: `fetchInitialConfig` against a fake hub (the existing fake of that file) that answers auth at once and the `config-fetch` only after a delay longer than a shortened `ze.managed.connect.timeout`; the call returns the hub's configuration. Red before: the fetch shares the connect context and fails on its deadline. MUTATION: pass the connect context to `FetchConfig` | |
| `TestFirstBootFetchEndsAtFetchDeadline` | `internal/component/managed/client_fetch_deadline_test.go` | OD-6 bound, proven where the bound lives so no test waits 30 s: exported `FetchConfig`, which `fetchInitialConfig` calls, delegates to the same unexported bounded fetch as `fetchAndProcess` with `configFetchTimeout`; the test runs that bounded fetch with a 200 ms bound against an in-memory hub (`heldPipe`) that never answers and asserts an error wrapping `context.DeadlineExceeded` after the bound with a caller context that has no deadline. Together with `TestFirstBootFetchOutlivesConnectTimeout` (first boot no longer cut by the connect bound) and `TestFetchTimeoutDocumentsPushBoundTradeOff` (the constant's value), it proves first boot is bounded by `configFetchTimeout`. MUTATION: drop the bound in the unexported fetch | |
| `TestFetchTimeoutDocumentsPushBoundTradeOff` | `internal/component/managed/client_held_test.go` | OD-3: `configFetchTimeout` is at least the time `rpc.MaxMessageSize` takes at 4.5 Mbit/s, and greater than `pluginserver.ManagedPushTimeout` (the trade-off is deliberate and a later edit that shrinks it below the push bound, or below the 16 MiB budget, goes red). MUTATION: set it to 2 s | |
| `TestWireManagedCommitOverStagesAndPromotes` | `cmd/ze/hub/managed_test.go` | AC-5 daemon side, replacing `TestWireManagedCommitStagesCandidateAndPromotes` and `TestWireManagedRevertAppliesOverChecked`: `CommitOver` with a check that accepts stages, reloads, promotes and records the baseline | |
| `TestWireManagedCommitOverRefusesLocalCommit` | `cmd/ze/hub/managed_test.go` | AC-5 daemon side, replacing `TestWireManagedRevertRefusedOverLocalCommit`: with the active configuration changed after the client read it, `CommitOver` with a check that refuses any active configuration other than the old one (the test's own check, as `TestWireManagedRevertRefusedOverLocalCommit` passes today) answers that check's error as is and stages no candidate. MUTATION: stage through `StageMachineCandidate` in `CommitOver` | |
| `TestWireManagedCommitOverRefusesStagedCandidate` | `cmd/ze/hub/managed_test.go` | R-3: with a candidate already staged, `CommitOver` answers `ErrCandidateExists` and leaves both the candidate and the active configuration | |
| `TestFetchMadeAgainReportsHoldWhenDiverged` | `internal/component/managed/client_test.go` | R-1, the holding side: the wrapper restores A after the first refusal only, so L2 stays active; the call answers `errActiveChangedTwice` and the hold is reported as holding. MUTATION: report not holding unconditionally on `refetchNone` | |
| `TestConfigChangedLocalCommitTwiceLogsDecision` | `internal/component/managed/client_test.go` | R-1 on the loop: `handleConfigChangedRequest` answers `errActiveChangedTwice` (the connection ends), with one Info decision line and no Warn under `doctor-hub-unreachable`. MUTATION: drop its `errActiveChangedTwice` branch | |
| `TestLocalCommitTwiceReconnectsQuietly` | `internal/component/managed/client_test.go` | R-1 at connect: through `RunManagedClient`, the double race ends the first connection, a second one connects and fetches, and the log holds one Info decision and no Warn. That the reconnect's fetch holds and uploads on a device still diverged is proven directly by `TestFetchHoldsWhenActiveDiffersFromBaseline` and `TestHoldAckSaysHolding`, beside `TestPushHoldsOverLocalCommitDuringFetch` (a fetch over a diverged device sends its copy and acks holding) and `TestFetchAtConnectAfterHubDropsPush` (a reconnect's first request is a fetch naming the version the device runs). MUTATION: return another error from runConnection's branch, and the reconnect logs the Warn | |
| `TestRunManagedClientRaisesNoBaselineDefect` | `internal/component/managed/client_test.go` | D-3, the report-bus half: one error event, source `managed`, code `doctor-internal-defect`, fix `ze show errors`; resets the bus before and after. MUTATION: drop the `report.RaiseErrorFix` call | |
| `TestGuardedPushWithNoStoredActive` | `cmd/ze/hub/managed_test.go` | A-3: `CommitOver` over a store with no active configuration refuses with the read error and stages nothing, so the behavior A-3 relies on is pinned | |
| existing `TestWireManagedCommitClearsCandidateOnReloadFailure`, `TestWireManagedCommitRefusedDuringConfirmWindow`, `TestBaselineWrittenAfterPush`, `TestBaselineUnchangedWhenPushFails`, `TestManagedACKDeferredUntilCommit`, `TestManagedACKErrorOnCommitFail`, `TestManagedOnCommitNil`, the `client_endpoint_test.go` tests, `TestDiscard*` | `cmd/ze/hub/managed_test.go`, `internal/component/managed/client_test.go`, `client_endpoint_test.go`, `client_held_test.go` | Preservation: moved from `OnCommit`/`OnRevert` onto `CommitOver` and given a `BaselineAccess`, each asserting at least what it asserted (the `cfg.Version` assertions become store assertions, judged by `ze-test-judge`) | |

Each test was proved to discriminate by reverting the behavior it covers: the
pin branch removed, the pin branch replaced by `InsecureSkipVerify`, the default
branch made insecure, the env lookup and the lowercasing dropped, the server
made to ignore its configured certificate name, the two extraction assignments
dropped, the YANG pattern dropped, and the agreement check dropped. Every
mutation turned exactly the covering test red. (Closure note, 2026-11-20: those
mutations were made on the 2026-08-29 fingerprint-pin tests. The CA design of
8788a29f8b replaced them with the CA tests named in the first rows above, and
their end-to-end discrimination is `test/managed/managed-hub-ca-trust.ci`'s
foreign-root control.)

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `managed-hub-ca-trust` | `test/managed/managed-hub-ca-trust.ci` | two real daemons: the client verifies the hub chain against the exported root and fetches, and a foreign root does not (AC-1/AC-2/AC-4 fetch half) | PASS |
| `doctor-hub-managed-collision` | `test/ui/doctor-hub-managed-collision.ci` | `ze doctor --json` reports the collision (AC-3) | PASS |
| `doctor-hub-managed-separate` | `test/ui/doctor-hub-managed-separate.ci` | `ze doctor --json` stays silent on the central hub that works (AC-3 control) | PASS |
| AC-4 `config-changed` half | none | BLOCKED: no operator-reachable producer, see "AC-4 config-changed: no producer" |
| `fleet-config-change-live` (added 2026-11-19; replaces the BLOCKED row above, whose blocker OD-1 cleared) | `test/managed/fleet-config-change-live.ci`, fixture `managed/fleet-config-change-live` in `internal/test/fixture/misc_fixture_managed_config_change_live.go` and its `register_` file | AC-4: two `ze start` daemons, hub and edge-01, built on the `caTrust` and `fleetHoldRun` helpers. edge-01 connects and applies the served config (router-id A). The fixture then writes a changed `client-edge-01.conf` (router-id B) and runs `ze data import client-edge-01.conf` against the running hub's store; it asserts the answer names the import, polls edge-01 until its active config carries B (at most 10 s), reads edge-01's `show fleet status` and asserts `active-hash` and `baseline-hash` both equal the hash of the imported bytes with `diverged` false, and counts the hub's log: exactly one `managed client connected` line for edge-01 (the counting already used by `misc_fixture_managed_fleet_leave.go`, `fleetLeaveHubOpened`), no `managed client disconnected` line, and one `managed client applied config` line naming the new version. Red before: with the write observer not installed, edge-01 never takes B | |

## Remaining Work (2026-09-05)

| Item | State |
|------|-------|
| AC-1/AC-2 mechanism and tests | Done, in `internal/` |
| Hub wiring: `cmd/ze/hub/managed_server.go` passes `Certificate: blk.Certificate`, `TLSMaterialResolver: zepki.ServerTLSMaterial` and `Authority: caRoot` into `ManagedServerConfig` | DONE. Verified in `startManagedServer` |
| Client wiring: `cmd/ze/ze_core_start.go` `extractManagedClientConfig` passes `CA: cli.CA` into `managed.ClientConfig` | DONE. The design landed as a pki ca anchor rather than a fingerprint pin, so the leaf is `plugin/hub/client/ca` and `ClientConfig.CA` reads it |
| First boot: `fetchInitialConfig` (`cmd/ze/ze_core_start.go`) calls `managed.ClientTLSConfig` rather than building a second `tls.Config` | DONE |
| AC-3 port-collision doctor check | DONE. `internal/component/plugin/doctor/check_managed_listener.go`, code `doctor-hub-managed-collision`, `test/ui/doctor-hub-managed-collision.ci` and `test/ui/doctor-hub-managed-separate.ci` |
| AC-4 two-instance daemon `.ci` | HALF DONE, and the other half is BLOCKED. `test/managed/managed-hub-ca-trust.ci` runs a real `ze` hub and a real `ze` client and proves `config-fetch` end to end, with a foreign-root control. `config-changed` cannot be reached from two daemons: the push fires only on a write through the RUNNING hub's own store, and no operator command performs one. See "AC-4 config-changed: no producer" below |
| AC-4 `config-changed` half (2026-11-19) | OPEN, no longer blocked (OD-1). Owed: `test/managed/fleet-config-change-live.ci` through `ze data import` on the running hub, with no reconnect and the baseline recorded |
| AC-5 guarded push (2026-11-19) | OPEN. Designed: `CommitOver`, `activeCheck`, the refetch, the deletion of `OnCommit`, `OnRevert`, `ClientConfig.Version` and the nil-`Baseline` branch |
| AC-6 fetch deadline (2026-11-19) | OPEN. Designed: `configFetchTimeout` (30 s) in the fetch `fetchAndProcess` makes, and (OD-6) in the first-boot fetch of `fetchInitialConfig` |

## AC-4 config-changed: no operator-reachable writer

The 2026-09-05 investigation found that notifications fired only for writes
through the running hub's own blob store. The storage representation has since
changed: `storage.store.WriteFile` writes through the owned tree, and
`guard.Release` invokes the observer after durability and unlock
(`internal/component/config/storage/store.go`). The old claim that the running
store is a `zefs.BlobStore` snapshot no longer describes this path.

The missing producer remains. `startManagedServer`
(`cmd/ze/hub/managed_server.go`) installs the write observer and reads
`ClientConfigKey(name)`, but the callers that write that key are still tests.
The running hub needs an operator entry point that writes a managed client's
configuration through this store. The recorded fetch proof does not prove
that notification path or AC-4 as a whole.

The question for the owner is which way to fix it, not whether:

| Option | Shape |
|--------|-------|
| (a) A hub-side command that writes a managed client's config through the running daemon | New CLI surface plus its YANG. It makes the documented workflow real, and AC-4 then has a producer to drive |
| (b) The in-daemon editor accepts a second config path, the client blob | Reuses the editor, the draft, the history and the validation the operator already has. It widens `NewEditorWithStorage`'s single-path assumption across four call sites |

The documentation was repaired in this work rather than left claiming the
workflow: `docs/architecture/fleet-config.md` "Config Storage (Hub Side)" and
`docs/guide/fleet-config.md` "Config Management" now say the hub must be stopped.

**Resolution (2026-11-19, OD-1).** The blocker is cleared: writers that the
two options above asked for now exist on the running hub, built by other work.
`ze data import client-<name>.conf` beside a running hub sends the file to the
daemon (`importConfigsThroughDaemon`), which writes it through its own store
(`importOtherConfig`, `storage.RestoreConfig`), so the write observer fires;
`docs/guide/fleet-config.md` "Change a Client's Configuration" documents it,
and `docs/architecture/fleet-config.md` "Config Storage (Hub Side)" no longer
says the hub must be stopped for it. The hub-side `ze config edit
client-<name>.conf` (`test/managed/fleet-hub-editor-declared.ci`) and
fleet-7's adopt (`test/managed/fleet-divergence-adopt.ci`) are two more; the
adopt proves config-changed delivery to a live device, but its fetch answers
`current`, so it applies nothing. `test/managed/data-restore-client-live.ci`
also delivers a change from a live `request data restore`, without asserting
the reconnect count or the baseline. AC-4's remaining work is the one
functional test the "fleet-config-change-live" row of the TDD Test Plan
names.

## Files to Modify
- `internal/component/plugin/server/managed_serve.go` - serves the named PKI certificate or authority-issued leaf and fails closed (DONE)
- `internal/component/managed/tls.go` - the client trust decision (DONE)
- `internal/component/managed/client.go` - `CA` on `ClientConfig` (DONE)
- `internal/component/plugin/yang/ze-plugin-conf.yang` - the server `certificate` and client `ca` leaves (DONE)
- `internal/component/plugin/types.go`, `internal/component/config/loader_extract.go` - extraction (DONE)
- `docs/architecture/fleet-config.md` - hub certificate and client trust (DONE)
- `internal/component/plugin/doctor/check_managed_listener.go` and `register.go` + `internal/core/diagnostic/codes.go` (AC-3, DONE)
- `test/ui/doctor-hub-managed-collision.ci`, `test/ui/doctor-hub-managed-separate.ci` (AC-3, DONE)
- `docs/guide/health-checks.md`, `docs/architecture/fleet-config.md`, `docs/guide/fleet-config.md` (DONE)

### Files to Modify (added 2026-11-19, AC-4, AC-5, AC-6)
- `internal/component/managed/client.go` - delete `OnCommit`, `OnRevert` and `Version` from `ClientConfig` and add `CommitOver` (data and a check of the active bytes; nil refuses every configuration as "commit not ready"); document `Baseline` as required; delete the nil-`Baseline` branch of `fetchAndProcess`; `RunManagedClient` refuses a nil `Baseline` at entry (D-3); rename `revertCheck` to `activeCheck` and generalise the texts of `errActiveChanged`, `errActiveChangedTwice` and the `refetchOverLocalCommit` log line to a hub answer, push or revert; `applyFetched` takes the active version and commits through `CommitOver` with `activeCheck`; `holdOrApply` takes the call's `revertRefetch` (the type keeps its name; its comment widens to a refused push), sends an `errActiveChanged` refusal to `refetchOverLocalCommit`, and reports the hold after the outcome (R-8); `applyReverted` commits through `CommitOver`; add `configFetchTimeout` beside `heldUploadTimeout` and `configAckTimeout`, an unexported `fetchTimeout` field on `ClientConfig` (zero means `configFetchTimeout`, a test shortens it, as `ManagedServer.pushTimeout`), and an unexported fetch that bounds `CallRPC` with it, used by both fetches of `fetchAndProcess`; exported `FetchConfig` bounds with `configFetchTimeout` too; the Warn of a failed fetch in `handleConfigChangedRequest` gains `codeHubUnreachable` and its fix; correct the comments of `heldUploadTimeout`, `configAckTimeout`, `heldUploads`, `RunManagedClient` (the callbacks it borrows) and `applyCommitted`
- `cmd/ze/ze_core_start.go` - (OD-6) `fetchInitialConfig`: the `ze.managed.connect.timeout` context bounds the dial, the handshake, `SendAuth` and the auth read only; `managed.FetchConfig` is called with a context not derived from it, so `configFetchTimeout` alone bounds the fetch; its comment says which bound covers which step
- `cmd/ze/ze_core_dispatch.go` - (OD-6) the `ze.managed.connect.timeout` registration's description ("Connection timeout for managed hub") states that it bounds reaching the hub (dial and handshake, and authentication at first boot), not the configuration fetch
- `cmd/ze/ze_core_start_bootstrap_test.go` - (OD-6) `TestFirstBootFetchOutlivesConnectTimeout`
- `internal/component/managed/baseline.go` - the comment naming `OnCommit` beside `CheckManaged`
- `internal/component/managed/handler.go` - the comment "committed through ClientConfig.OnCommit"
- `cmd/ze/hub/managed.go` - `wireManagedCommit` sets `CommitOver` only, staging with `storage.StageMachineCandidateOver` inside `confirm.WriteOutside` and `applyManagedPush`; its comment
- `cmd/ze/hub/managed_owner.go` - the `wire` field comment ("OnCommit, OnRevert")
- `internal/component/config/storage/pointer.go` - the `StageMachineCandidate` comment's caller list: `applyManagedPush` now stages every push through `StageMachineCandidateOver`
- `internal/component/plugin/server/managed_serve.go` - the `ManagedPushTimeout` comment: the client bounds its upload and ack below it, and its fetch deliberately not (OD-3)
- Tests moved onto `CommitOver` and a `BaselineAccess` (census of every `OnCommit` and `OnRevert` caller and fake, 2026-11-19): `cmd/ze/hub/managed_test.go` (all seven tests), `cmd/ze/hub/managed_machine_writers_test.go` (the push writer), `cmd/ze/hub/managed_owner_test.go` (the fake at the `client.OnCommit` assignment), `internal/component/managed/client_test.go` (`TestManagedOnCommitNil`, `TestManagedACKDeferredUntilCommit`, `TestManagedACKErrorOnCommitFail`, and every literal reaching `fetchAndProcess`), `internal/component/managed/client_endpoint_test.go` (six literals), `internal/component/managed/client_hold_test.go` (`holdClient`, and the `cfg.Version` decoy of `TestFetchSendsActiveHash`), `internal/component/managed/client_held_test.go` (the revert tests and the `OnRevert` override in `TestDiscardRefusedApplyStillHolds`)
- `docs/architecture/fleet-config.md` - "The baseline and the hold" and the revert paragraph (`CommitOver`, the push's check and refetch), "Config Change (Two-Phase)" phase 4 (the check under the lock and its refetch), "Connection Lifecycle" (the fetch bound and its trade-off against the push bound), the upload-bound paragraph that names `heldUploadTimeout`, and "Architecture" (the shutdown sentence naming `OnCommit`); (OD-6) the `--connect-timeout` row of "Client Side" ("the first fetch included" becomes: the dial and TLS handshake of each connection and the authentication at first boot, while every configuration fetch, the first one included, is bounded by 30 s), and "First Boot (After `ze init`)" for the same split
- `docs/guide/fleet-config.md` - "Change a Client's Configuration" (a local change made while a push is fetched is kept, and the client holds), "When a Client Holds" (that cause of a hold), "Troubleshooting" (a fetch cut by its 30 s bound on a slow link, and what the client does then); (OD-6) the `--connect-timeout` row of "`ze start` Flags" (the time allowed to reach the hub, for each attempt; the fetch of the configuration has its own 30 s bound). `grep -rn 'ze.managed.connect.timeout\|connect-timeout' docs` on 2026-11-19 found these two pages only for the managed flag; the `--connect-timeout` rows of `docs/guide/command-reference.md` and `docs/guide/benchmarking.md` belong to another tool (a `10s` TCP connection timeout) and do not change

### Files to Create (added 2026-11-19)
- `test/managed/fleet-config-change-live.ci` - AC-4
- `internal/test/fixture/misc_fixture_managed_config_change_live.go`, `internal/test/fixture/register_managed_config_change_live.go` - its compiled fixture and registration
- `internal/component/managed/client_fetch_deadline_test.go` - the AC-6 unit tests and their silent hub fake

## Implementation Steps
1. Keep the implemented PKI/CA trust path; the remaining owner decision is the client-config writer.
2. Serve a verifiable cert from the managed listener.
3. Wire client-side verification.
4. Add the port-collision doctor check + diagnostic code.
5. Add the two-instance functional `.ci`.
6. Full verification + docs.

### Implementation Steps (added 2026-11-19)

Step 6 above is superseded by the owner rule that no agent runs `./le verify`: the evidence is the scoped package tests and the named functional tests the main thread runs.

1. **Phase: Wiring (AC-4 functional test first)** -- write `fleet-config-change-live` (fixture, registration, `.ci`) on the existing `caTrust` and `fleetHoldRun` helpers.
   - Inputs: the hub and client config builders `restoreClientHubConfig`, `restoreClientConfig`; `fleetLeaveHubOpened` counting; `fleetStatusAnswer`.
   - Output: a `.ci` that passes on today's code (AC-4 is an existing behavior's proof), proven to discriminate by removing the `store.SetWriteObserver` call in `startManagedServer` (red: edge-01 never takes B), then restoring it (green). The main thread runs both.
   - Constraint: assert state (the device's status and the hub's log lines), never a sleep (`ai/rules/testing.md`); the `ze-test` skill governs the draft.
2. **Phase: Guarded push (AC-5)** -- replace `OnCommit` and `OnRevert` with `CommitOver` (`ai/rules/no-layering.md`: delete first, then add).
   - Tests first: `TestPushHoldsOverLocalCommitDuringFetch`, `TestPushAppliesWhenActiveUnchanged`, `TestPushFetchesAgainOnlyOnce`, `TestPushRefusalOtherThanLocalCommitAcks`, `TestRunManagedClientRefusesNoBaseline`, the four `TestWireManagedCommitOver*` and `TestGuardedPushWithNoStoredActive`; red before the change (`TestPushHoldsOverLocalCommitDuringFetch` commits over L today).
   - Files: `client.go`, `managed.go`, the comment files, and every test in the census under Files to Modify.
   - Inputs and outputs: `CommitOver` receives the hub's validated bytes and a check; the daemon runs the check on the active bytes under the config lock and answers the check's error as is; the client treats `errActiveChanged` as "refetch", every other error as an `ok:false` ack.
   - Constraint: a revert's behavior is unchanged (its tests pass unmodified except for the callback they set).
3. **Phase: Fetch deadline (AC-6)** -- add `configFetchTimeout` and the bounded fetch.
   - Tests first: `TestFetchDeadlineAtConnect`, `TestFetchDeadlineOnNotificationLoop`, `TestFetchDeadlineDropsLateAnswer`, `TestFetchAtConnectAfterHubDropsPush`, `TestFetchTimeoutDocumentsPushBoundTradeOff`; red before (the first two never return).
   - Constraint: the constant's comment says it is the one loop bound that is not below `ManagedPushTimeout`, why (a 16 MiB answer, A-4), and what a longer fetch costs (the hub closes the connection, the device fetches on reconnect); the comments of `heldUploadTimeout` and `configAckTimeout` keep their "below the push bound" rule and name the fetch as the exception.
   - (OD-6) First boot in the same phase: `TestFirstBootFetchOutlivesConnectTimeout` and `TestFirstBootFetchEndsAtFetchDeadline` first (the first is red today: the fetch shares the 5 s connect context); then split `fetchInitialConfig`'s context so `ze.managed.connect.timeout` bounds the dial, handshake and authentication and `FetchConfig` takes a context not derived from it; correct the env var's registered description and the two pages' `--connect-timeout` rows in this phase.
4. **Phase: Documentation** -- each page edit lands with the phase that makes the page wrong (`ai/rules/documentation.md`): the architecture and guide edits of phase 2 with phase 2, those of phase 3 with phase 3.

### Critical Review Checklist (added 2026-11-19)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-4..AC-6 has an implementation at file:line and its named test |
| Correctness | `activeCheck` compares the version of the bytes read under the lock with the version read before the fetch, never with `resp.Version` or the baseline |
| Correctness | Only `errActiveChanged` reaches the refetch; a window refusal, a staged candidate or a reload failure is acked `ok:false` as today |
| Data flow | No call site stages a hub push through `storage.StageMachineCandidate` after the change (`grep` in `cmd/ze/hub`) |
| No layering | `OnCommit`, `OnRevert`, `ClientConfig.Version` and the nil-`Baseline` branch are gone: `grep -rn 'OnCommit\|OnRevert' cmd internal` finds only unrelated symbols |
| Comments | Every comment that named `OnCommit`, `OnRevert`, "below the push bound" for every loop round-trip, or the unbounded fetch is corrected (`ai/rules/stale-comments.md`) |
| Operator errors and doctor codes | The refetch's log line and the two error texts say what happened, why, and what the device does next (`ai/rules/user-facing-errors.md`) |

### Deliverables Checklist (added 2026-11-19)
| Deliverable | Verification method |
|-------------|---------------------|
| AC-4 functional test | `./le test managed` running `fleet-config-change-live`, plus its recorded red with the observer removed |
| Guarded push | `go test` of `internal/component/managed` and `cmd/ze/hub` through `./le test unit pkg path`, the named tests green and `TestPushHoldsOverLocalCommitDuringFetch` red before |
| Fetch deadline | the same package run, `TestFetchDeadline*` red before |
| Docs | `./le doc check all` over the two edited pages |

### Security Review Checklist (added 2026-11-19)
| Check | What to look for |
|-------|-----------------|
| Resource exhaustion | A hub that never answers can no longer hold the device's loop for ever; a late 16 MiB answer is read once and dropped (R-6) |
| Authorization that could fail open | A nil `CommitOver` refuses every configuration; a nil `Baseline` refuses to run the client; no check is ever nil at the stage (`StageMachineCandidateOver` requires one) |
| Overwrite of operator work | The push can no longer overwrite a local commit that landed during its fetch (AC-5) |

### Integration Checklist (added 2026-11-19, for AC-4..AC-6)
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No config leaf or RPC is added; the fetch bound is fixed by owner decision OD-3, not configured |
| YANG validation constraints | N-A | No leaf added |
| YANG custom validators | N-A | No leaf added |
| CLI commands/flags | No | `ze data import` is used unchanged |
| CLI grammar (keyword before value) | N-A | No command added |
| Editor autocomplete | N-A | No leaf added |
| Functional test for new RPC/API | Yes | No RPC is new; the changed delivery is proven by `test/managed/fleet-config-change-live.ci` |
| Pipe completeness | N-A | No command output changes |
| Env var registration | No | No env var added; the bound is a constant (OD-3) |
| Doctor check for runtime dependencies | N-A | No file path, socket, port, module, binary or certificate is added |
| Doctor code per problem class | Yes | Two problem classes, no new code. (1) A device that holds after a refused push: the existing hold warning under `doctor-managed-diverged` (`holdFetched`) and the doctor check that reads the hold (`internal/component/managed/doctor.go`), post-config, as for any hold; the refetch's Info line carries no code because it reports a decision, not a fault. (2) A fetch cut by its deadline: the loop's Warn "fetch after notification failed" (`handleConfigChangedRequest`) carries no code today; it gains `codeHubUnreachable` (`doctor-hub-unreachable`, declared by `hubDoctorCheck`, post-config, whose comment already says "A hub exchange that fails on a live connection logs under it too", as `sendAck` does) and a fix naming `ze doctor`; at connect the error reaches `RunManagedClient`'s existing reconnect log |
| Prometheus counters/metrics | No | `ze_managed_config_fetch_total` on the hub already counts fetches; no new state |
| BGP family surface | N-A | No SAFI, capability or attribute |

### Documentation Update Checklist (added 2026-11-19, for AC-4..AC-6)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | The fleet configuration feature gains evidence (AC-4's two-daemon test) and a guard; update its `features/<id>.md` evidence row, the id read from `./le feature report` at implementation |
| 2 | Config syntax changed? | No | No leaf changes |
| 3 | CLI command added/changed? | No | `ze data import` unchanged |
| 4 | API/RPC added/changed? | No | No fleet verb or envelope changes (Changed-Wire Census) |
| 5 | Plugin added/changed? | No | No plugin changes |
| 6 | Has a user guide page? | Yes | `docs/guide/fleet-config.md`: "Change a Client's Configuration", "When a Client Holds", "Troubleshooting", and (OD-6) the `--connect-timeout` row of "`ze start` Flags", which documents `ze.managed.connect.timeout` |
| 7 | Wire format changed? | No | Census: none |
| 8 | Plugin SDK/protocol changed? | No | `pkg/plugin/rpc` is used unchanged |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | Not an RFC protocol |
| 10 | Test infrastructure changed? | No | A fixture is added on the existing harness; the runner is unchanged |
| 11 | Affects daemon comparison? | No | No comparison row covers fleet push races |
| 12 | Internal architecture changed? | Yes | `docs/architecture/fleet-config.md`: the push's check and refetch, the fetch bound and its trade-off, and (OD-6) the `--connect-timeout` row of "Client Side" and "First Boot (After `ze init`)": the connect bound no longer covers the first fetch |
| 13 | Route metadata keys added/changed? | N-A | No routes |
| 14 | Prometheus counters added/changed? | No | None |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | None |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/architecture/fleet-config.md` (declared by the `// Design:` headers of `client.go` and `cmd/ze/hub/managed.go`) and `docs/guide/fleet-config.md` (anchors on `client.go`); the complete list is derived at implementation by `./le spec citation anchors spec plan/release/fixes/spec-managed-server-hardening.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | The `ze data import client-edge-01.conf` example of the guide is the one AC-4 drives; it stays as written |

## Key Design Decisions (added 2026-11-19)
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| D-1: one callback, `ClientConfig.CommitOver` (the hub's bytes and a check of the active bytes), for push and revert | Keep `OnCommit` with a new signature beside `OnRevert`; keep the name `OnCommit` | One guarded path (OD-2, `ai/rules/no-layering.md`); a new name makes every old caller a compile error, so the census cannot miss one |
| D-2: the push's check is `activeCheck(A)`, A the version `access.read()` answered before the fetch | Compare with the baseline; compare with `resp.Version` | A is what the decision to apply rested on (active equalled baseline); the baseline could equal a new local edit's content only by coincidence, and `resp.Version` describes the hub's bytes, not the device's |
| D-3 (owner-confirmed 2026-11-19, OD-5): the nil-`Baseline` branch of `fetchAndProcess` and `ClientConfig.Version` are deleted; `Baseline` is required and `RunManagedClient` refuses a nil one at entry with one Error line | Keep the branch unguarded (applies every answer); guard it with `cfg.Version` | No production client takes the branch (A-1). Unguarded, it is the overwrite this spec removes, reachable by any new caller. A `cfg.Version` guard would have no hold, copy or refetch to answer its refusal with, since those all read the store through `Baseline` |
| D-4: the fetch bound lives in the fetch itself, as the ack's does in `SendConfigAck`, with a test-only unexported override on `ClientConfig` | A context deadline set at each of the two call sites; a package variable a test changes | One place covers connect, the loop and the refetch; a package variable races parallel tests; the override follows `ManagedServer.pushTimeout` |
| D-7 (OD-6): first boot's fetch takes `configFetchTimeout` through `FetchConfig`, under a context not derived from the connect context, which keeps the dial, handshake and authentication | Leave first boot under one 5 s context; extend the connect context to 30 s | One fetch bound everywhere (owner); a 30 s connect context would also stretch the dial and handshake bound the operator set |
| D-5: a refused push sends no ack; the refetch's hold ack answers the hub | Ack the refusal, then refetch | The revert already does this (`applyCommitted`); an extra `ok:false` ack would record a rejection the hub must then clear |
| D-6: `holdOrApply` reports the hold after the outcome | Report "not holding" first, as today | A refused push becomes a hold, so an early "not holding" would be false for the length of the refetch (R-8) |
| D-8 (main-thread decision during review round 2, 2026-11-20): a local commit that races both fetches of one push (`errActiveChangedTwice`) ends the connection on both paths, at connect (`runConnection`) and on the loop (`handleConfigChangedRequest`), logged once at Info (`logLocalCommitTwice`), and `RunManagedClient` reconnects with its normal backoff and no "connection lost" Warn; the reconnect's fetch decides again (holds and uploads its copy, or applies). A device whose local commits keep racing reconnects about once a second, because the backoff resets at each successful authentication (`runConnection` `backoff.Reset`) | Keep the connection (round 1's fix): nothing then fetches again, so a diverged device never uploaded its copy or acked holding until the hub's next change; a timer refetch on the loop: a new goroutine lifecycle for a race that needs a local commit inside each of two fetches | The reconnect is the existing path that fetches over what the device runs, so no new mechanism is needed. The cadence needs no Work Not Done row: each round needs a local commit landing inside each of two fetches, and on a managed device only machine writers (a confirmed-commit revert, a signal reload) and an operator change after the freeze is lifted commit locally (A-6), none of them continuous; the round ends at the first fetch no commit races, the cost is one reconnect and one Info line per round, and `docs/architecture/fleet-config.md` "The baseline and the hold" states it. Tests: `TestLocalCommitTwiceReconnectsQuietly`, `TestConfigChangedLocalCommitTwiceLogsDecision` |

## Checklist
- [x] Cert approach decided with the user
- [x] Secure connection works without `tls-insecure`
- [x] Doctor check registered
- [x] Two-instance `.ci` passes
- [x] Tests written
- [x] Tests FAIL (paste output)
- [x] Tests PASS (paste output)
- [ ] `./le verify worktree` passes
- [ ] (2026-11-19) The line above is superseded by the owner rule that no agent runs any `./le verify` command: the evidence owed is the scoped tests of `internal/component/managed` and `cmd/ze/hub`, `test/managed/fleet-config-change-live.ci`, and the recorded red of each new test, run by the main thread

## Implementation Summary

### What Was Implemented
- AC-1, AC-2 (2026-08-29 to 2026-09-05, commits up to 8788a29f8b): the hub serves a named pki certificate or a leaf its local authority issues (`managedCertificate`, `internal/component/plugin/server/managed_serve.go`; wired by `startManagedServer`, `cmd/ze/hub/managed_server.go`), and the client verifies against the pki CA it names, the hub root it kept, or the system pool (`clientTLSConfig`, `internal/component/managed/tls.go`), first boot included (`ClientTLSConfig`, called by `fetchInitialConfig`).
- AC-3: `diagnoseManagedListener` (`internal/component/plugin/doctor/check_managed_listener.go`) reports `doctor-hub-managed-collision`.
- AC-4 (0318255b49): `test/managed/fleet-config-change-live.ci` and its fixture `managedFleetConfigChangeLiveDriver` (`internal/test/fixture/misc_fixture_managed_config_change_live.go`).
- AC-5 (75886ea0cd): `ClientConfig.CommitOver` replaces `OnCommit` and `OnRevert`; `applyFetched` commits every push and revert over `activeCheck(decided)`; `wireManagedCommit` stages through `storage.StageMachineCandidateOver`; `holdOrApply` sends a refusal to `refetchOverLocalCommit` and reports the hold after the outcome; `ClientConfig.Version` and the nil-`Baseline` branch are deleted, and `RunManagedClient` refuses a nil `Baseline` under `doctor-internal-defect`.
- AC-6 (75886ea0cd): `configFetchTimeout` (30 s) in `fetchConfigWithin`, used by `fetchAgainstBaseline` and `FetchConfig`; `fetchInitialConfig` (`cmd/ze/ze_core_start.go`) bounds only dial, handshake and authentication by `ze.managed.connect.timeout`.

### Bugs Found/Fixed
- The unguarded ordinary push (journal `guard-added-to-one-half-of-a-pair.md`, 2026-11-19): `TestPushHoldsOverLocalCommitDuringFetch`.
- The unbounded fetch (same journal, 2026-11-19): `TestFetchDeadlineAtConnect`, `TestFetchDeadlineOnNotificationLoop`.
- Found in review: a double race logged as a hub outage (`TestConfigChangedLocalCommitTwiceLogsDecision`); the no-baseline refusal under the wrong code (`TestRunManagedClientRaisesNoBaselineDefect`); a stale hold report on the fetch made again (`TestPushFetchesAgainOnlyOnce`, `TestFetchMadeAgainReportsHoldWhenDiverged`); a diverged device that never uploaded after a double race (`TestLocalCommitTwiceReconnectsQuietly`).

### Documentation Updates
- `docs/architecture/fleet-config.md`: "The baseline and the hold" (the guarded push, the double race and its reconnect cadence), "Config Change (Two-Phase)", "Connection Lifecycle" (the fetch bound and its trade-off, anchor `client.go -- configFetchTimeout, fetchConfigWithin, FetchConfig, handleConfigChangedRequest`), "First Boot", the "Client Side" `--connect-timeout` row (75886ea0cd).
- `docs/guide/fleet-config.md`: "Change a Client's Configuration", "When a Client Holds", three "Troubleshooting" rows, the `ze start` `--connect-timeout` row, anchor `client.go -- configFetchTimeout, handleConfigChangedRequest` (75886ea0cd).
- `docs/architecture/doctor-and-health-checks.md`: the managed client's `doctor-internal-defect` raise (75886ea0cd).
- `features/fleet-management.md`: real-path test and extra criterion (0318255b49), Defect review row (closure).
- `./le doc check all`: see Pre-Commit Verification.

### Deviations from Plan
- D-8 (Key Design Decisions): a double race ends the connection on both paths instead of keeping it, so the reconnect's fetch decides again.
- The AC-1/AC-2 TDD rows named the 2026-08-29 fingerprint-pin tests; the CA design replaced them, and the rows now name the CA tests.
- The Integration Checklist planned the loop's fetch-failure Warn under `doctor-hub-unreachable`; the double race is carved out of it at Info (round 1).

## Mistake Log
| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | Round 1 kept the connection after a double race, so the log line would not read as a lost connection | Nothing then fetched again, so a diverged device never uploaded its copy or acked holding until the hub's next change | Review round 2 | D-8: end the connection, quiet reconnect |
| approach | The TDD table kept the AC-1/AC-2 test names of the fingerprint-pin design after the CA design renamed and replaced them | The tests are `TestManagedClientValidatesAgainstConfiguredRoot`, `TestManagedClientRefusesAnotherIssuer`, `TestManagedClientRefusesWithNoAnchor`, `TestFingerprintConfigIsRefused`, `TestHubCARejectsAnUnusableName` | Closure grep of every TDD row | Rows corrected in this closure |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| 1. Secure server-cert verification | Done | `internal/component/managed/tls.go` `clientTLSConfig`; `internal/component/plugin/server/managed_serve.go` `managedCertificate`; `cmd/ze/hub/managed_server.go` `startManagedServer` | |
| 2. Port-collision doctor check | Done | `internal/component/plugin/doctor/check_managed_listener.go` `diagnoseManagedListener`; code in `internal/core/diagnostic/codes.go` | |
| 3. Two-instance daemon `.ci` (fetch and config-changed) | Done | `test/managed/managed-hub-ca-trust.ci` (fetch), `test/managed/fleet-config-change-live.ci` (config-changed) | |
| 4. Device-side guards of fetch and change delivery | Done | `internal/component/managed/client.go` `applyFetched`, `activeCheck`, `fetchConfigWithin`; `cmd/ze/hub/managed.go` `wireManagedCommit` | |
| 5. Design of the remainder (OD-1..OD-7) | Done | as above; `cmd/ze/ze_core_start.go` `fetchInitialConfig` (OD-6) | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestManagedClientValidatesAgainstConfiguredRoot`, `TestManagedServerServesConfiguredCertificate`, `managed-hub-ca-trust.ci` | |
| AC-2 | Done | `TestManagedClientRefusesAnotherIssuer`, `TestManagedClientRefusesWithNoAnchor`, the foreign-root half of `managed-hub-ca-trust.ci` | |
| AC-3 | Done | `TestManagedListenerCollisionCases`, `doctor-hub-managed-collision.ci`, `doctor-hub-managed-separate.ci` | |
| AC-4 | Done | `fleet-config-change-live.ci` | red with the write observer removed |
| AC-5 | Done | `TestPushHoldsOverLocalCommitDuringFetch`, `TestPushAppliesWhenActiveUnchanged`, `TestPushFetchesAgainOnlyOnce`, `TestPushRefusalOtherThanLocalCommitAcks`, `TestWireManagedCommitOverRefusesLocalCommit` | |
| AC-6 | Done | `TestFetchDeadlineAtConnect`, `TestFetchDeadlineOnNotificationLoop`, `TestFetchDeadlineDropsLateAnswer`, `TestFetchAtConnectAfterHubDropsPush`, `TestFirstBootFetchOutlivesConnectTimeout`, `TestFirstBootFetchEndsAtFetchDeadline` | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| Every row of the TDD Test Plan | Done | the file each row names; located by `grep -rl "func <name>("` at closure, every name found | the AC-1/AC-2 rows renamed, see Mistake Log |
| `fleet-config-change-live` | Done | `test/managed/fleet-config-change-live.ci` | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| Files to Modify (2026-11-19), every entry | Done | in 75886ea0cd |
| Files to Create (2026-11-19), every entry | Done | in 0318255b49 and 75886ea0cd |
| Files to Modify (AC-1..AC-3) | Done | 2026 commits up to 8788a29f8b |

### Audit Summary
- **Total items:** 5 requirements, 6 ACs
- **Done:** all
- **Partial:** none
- **Skipped:** none
- **Changed:** D-8 and the AC-1/AC-2 test names (Deviations)

## Goal Validation (BLOCKING)
| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| 1. A client verifies the hub without `tls-insecure` | functional, two daemons | `test/managed/managed-hub-ca-trust.ci`: trusted root fetches, foreign root is refused (managed suite 32/32, 2026-11-20 02:27) |
| 2. The collision is reported by `ze doctor` | functional | `test/ui/doctor-hub-managed-collision.ci`, control `doctor-hub-managed-separate.ci` |
| 3. A real hub's change reaches a real device | functional, two daemons | `test/managed/fleet-config-change-live.ci`: passes, red with the hub's write observer removed ("within 10s of the import the device did not run router-id 3.3.3.3 as its baseline"), 40/40 under `./le test stress-repro` |
| 4a. A push never overwrites a local commit made during its fetch | unit, deterministic race | `TestPushHoldsOverLocalCommitDuringFetch` (client) and `TestWireManagedCommitOverRefusesLocalCommit` (daemon stage under the lock), green in `internal/component/managed` and `cmd/ze/hub` |
| 4b. A silent hub cannot freeze a device | fault injection (silent hub fake over TLS) | `TestFetchDeadlineAtConnect`, `TestFetchDeadlineOnNotificationLoop`, `TestFirstBootFetchEndsAtFetchDeadline`, green |
| 5. First boot fetches a large configuration past the connect bound | unit over the real `fetchInitialConfig` | `TestFirstBootFetchOutlivesConnectTimeout` (connect bound 500 ms, hub answers after 1.5 s), green in `cmd/ze` |

## Work Not Done
| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| none | Every AC is done. The reconnect cadence of a device whose local commits keep racing (about once a second) is an accepted cost, not owed work: D-8 gives the reasons | - |

## Review Gate
| Field | Value |
|-------|-------|
| Artifact | `tmp/review/managed-server-hardening-1ecdeafe-7c8e-4b32-a486-fac3ee6445a8.md` (34 files, verdict clean), written by `./le spec review record spec managed-server-hardening verdict CLEAN rounds 4` |
| `./le spec review check` | clean |
| Rounds | 4: three implementation review rounds, then the closure pass |
| Reviewer lenses used | logic and wiring, security and edge cases (guard audit of `activeCheck` and the stage, deadlines and DoS), docs and operator messages |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | A double race was logged at Warn under `doctor-hub-unreachable` with fix `ze doctor`, though the hub answered | `client.go` `handleConfigChangedRequest` | round 1: an Info branch, `logLocalCommitTwice` |
| 2 | ISSUE | The no-baseline refusal named `doctor-managed-diverged`, whose check never reports that state | `client.go` `RunManagedClient` | round 1: `doctor-internal-defect` and a report-bus raise, `TestRunManagedClientRaisesNoBaselineDefect` |
| 3 | ISSUE | A-1, A-2, A-3, A-6 left unvalidated | this spec | round 1: confirmed with evidence |
| 4 | ISSUE | At connect a double race was logged as "connection lost" | `client.go` `runConnection` | round 1 (found by the fixer), then superseded by #6 |
| 5 | ISSUE | The fetch made again reported holding without reading the store | `client.go` `refetchOverLocalCommit` | round 1 (found by the fixer): reads the store again |
| 6 | ISSUE | Keeping the connection after a double race left nothing to fetch again, so a diverged device never uploaded | `client.go` `runConnection`, `handleConfigChangedRequest`, `RunManagedClient` | round 2: D-8, `waitBackoff`, `TestLocalCommitTwiceReconnectsQuietly` |
| 7 | ISSUE | `TestRunManagedClientRaisesNoBaselineDefect` left the report bus dirty; the defect message gave no action | `client_test.go`, `client.go` `noBaselineDefect` | round 2 |
| 8 | ISSUE | Data Flow step 7, the TDD rows and the architecture page described the kept connection | this spec, `docs/architecture/fleet-config.md` | round 2 |

Round 3: wording only (R-1, the AC-5 survivor cell, the reconnect assertion of `TestLocalCommitTwiceReconnectsQuietly`, the page's cadence sentence). Round 4, the closure pass over 0318255b49 and 75886ea0cd: 0 BLOCKER, 0 ISSUE. NOTEs: (a) `revertRefetch` now bounds a push's refetch too, so its name says less than it does; the approved design kept the name and widened the comment (Files to Modify), so it stays. (b) The AC-1/AC-2 TDD rows and the discrimination paragraph named the replaced fingerprint-pin tests: corrected in this closure, one edit. `./le repo check`: 8 findings, none in this change set (unwired exports in `internal/component/cli` and `internal/le`). `./le test weakened check base 0318255b49~1`: no finding in this change set; the 18 listed are other sessions' (`internal/component/command`, `ike`, `plugin/server/command_pipe_refusal_test.go`, `hookruntime`), and every changed test of 75886ea0cd carries its `Weakening-approved:` trailer.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `test/managed/fleet-config-change-live.ci` | Yes | `ls`: 2.2K, 2026-11-19 22:24 |
| `internal/test/fixture/misc_fixture_managed_config_change_live.go`, `register_managed_config_change_live.go` | Yes | `ls`: 7.6K and 353 B |
| `internal/component/managed/client_fetch_deadline_test.go` | Yes | `ls`: 15K |
| `test/managed/managed-hub-ca-trust.ci`, `test/ui/doctor-hub-managed-collision.ci`, `test/ui/doctor-hub-managed-separate.ci` | Yes | `ls`: 1.6K, 1.2K, 1.2K |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1, AC-2 | the client verifies against a named CA, kept root or system pool; insecure only by opt-in | `clientTLSConfig` read at closure (`tls.go`:43-81); managed suite 32/32 at 02:27 on 2026-11-20 on a fresh build, including `managed-hub-ca-trust` |
| AC-3 | doctor reports the collision | `diagnoseManagedListener` (`check_managed_listener.go`:54), code registered (`codes.go`:597) |
| AC-4 | the import reaches the connected device on its session | `fleet-config-change-live` in the 32/32 run; red with `store.SetWriteObserver` removed from `startManagedServer`; 40/40 under stress-repro |
| AC-5 | every push stages over `activeCheck` under the lock | `wireManagedCommit` sets only `CommitOver`, which calls `storage.StageMachineCandidateOver` (`cmd/ze/hub/managed.go`); `grep OnCommit\|OnRevert\|revertCheck` over `cmd internal pkg docs` finds nothing; `internal/component/managed` green, round-2 tests 3x under -race; `cmd/ze/hub` managed tests green apart from `TestConfiguredPluginConformanceKeepsServerOwnership` (another session's) |
| AC-6 | every fetch is bounded by 30 s; first boot's connect bound covers only reaching the hub | `fetchConfigWithin` wraps `CallRPC` in `context.WithTimeout(ctx, bound)`; `fetchInitialConfig` calls `managed.FetchConfig(context.Background(), ...)` after clearing the read deadline; `cmd/ze` green apart from `TestPerfSendAnswersWithoutLauncherEnv` (other sessions' `le` edits) |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| client connects with cert verification | `test/managed/managed-hub-ca-trust.ci` | Yes: two `ze start` daemons, trusted and foreign root |
| managed clients on the acceptor's address | `test/ui/doctor-hub-managed-collision.ci` | Yes: `ze doctor --json` |
| `ze data import client-edge-01.conf` beside a running hub | `test/managed/fleet-config-change-live.ci` | Yes: read at closure; runs `le test fixture managed/fleet-config-change-live`, which imports through the hub daemon and asserts the device's `show fleet status` and the hub's log |
| a push fetched while a local commit lands | none (unit) | Yes: `TestPushHoldsOverLocalCommitDuringFetch` drives `fetchAndProcess`; the daemon half `TestWireManagedCommitOverRefusesLocalCommit` drives `wireManagedCommit`; the end-to-end apply path is crossed by `fleet-config-change-live.ci` |
| a hub that never answers `config-fetch` | none (unit) | Yes: `TestFetchDeadlineAtConnect` drives `runConnection` over TLS |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `grep ClientConfig{` outside tests: `extractManagedClientConfig` (sets `Baseline`, `ze_core_start.go`:505) and `fetchInitialConfig` (TLS only) |
| A-2 | confirmed | managed suite 32/32, fleet-7's revert and hold tests and `fleet-config-change-live` cross the seam |
| A-3 | confirmed | boot order read; `TestGuardedPushWithNoStoredActive` green |
| A-4 | confirmed | arithmetic, pinned by `TestFetchTimeoutDocumentsPushBoundTradeOff` |
| A-5 | confirmed | `managed_serve.go` sends only `config-changed` |
| A-6 | confirmed | every local writer takes `store.AcquireLock(configPath)` |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `docs/architecture/fleet-config.md` fetch bound and cadence | `configFetchTimeout`, `fetchConfigWithin`, `logLocalCommitTwice`, `RunManagedClient` (`backoff.Reset` in `runConnection`) | Yes |
| `docs/guide/fleet-config.md` `--connect-timeout` row and Troubleshooting | `fetchInitialConfig` `connectCtx`; `handleConfigChangedRequest` Warn under `codeHubUnreachable` | Yes |
| `ze.managed.connect.timeout` description | `cmd/ze/ze_core_dispatch.go` registration | Yes |
| No other `connect-timeout` page for this flag | the 2026-11-19 grep in Files to Modify; the `command-reference.md` and `benchmarking.md` rows belong to another tool | Yes |
| `./le doc check all` | run at closure: every failure is in the published `../gh-pages` and `../wiki` command catalogs (fleet divergence and l2tp session rows awaiting `./le site build` and `./le cli catalog update`), none in this change set, which adds and changes no command | Yes |
