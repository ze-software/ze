# Spec: fleet-6 -- Reconnect Hold, Emergency Disable and Config Freeze

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | config |
| Priority | P0 |
| Release reason | Fleet reconnect must not silently overwrite emergency device configuration. |
| RFC gaps | - |
| Depends | - |
| Phase | 5/5 |
| Handoff | - |
| Implementation tier | development tier |
| Updated | 2026-11-19 |

Recovery after compaction: `.claude/rules/post-compaction.md`. Then read this
spec, `docs/architecture/fleet-config.md`, `ai/doors/system.md` and
`ai/doors/hub-engine.md`. The `## Owner Decisions` section at the end holds
D-1 to D-8, which this design is built on. The `## Disposition` section holds
why the skeleton of 2026-06-26 was reworked.

## Task

A managed device loses a local configuration change without a trace. It tells
its hub the hash of whatever configuration it runs; after a restart, or when
the hub's copy changes, the hub answers with its own configuration and the
device applies it. An operator who changed the device in an emergency finds
the change gone.

Four pieces, in the order they land (D-1):

1. **Baseline and hold.** The device stores the hash of the last configuration
   that became active from its hub (the baseline). Before each fetch it
   compares the hash of its active configuration with the baseline. When they
   differ, the device applies nothing the hub sends, keeps running its
   configuration, stays connected, tells the hub it refused and why, and
   reports itself `diverged`. This is the guarantee (D-5).
2. **Leave and rejoin.** `request fleet disable` takes the device out of the
   fleet and ends the hub connection at once. `request fleet enable` puts it
   back. `show fleet status` reports where the device stands.
3. **Freeze.** While the device is managed, an operator's change to its own
   configuration is refused with a message naming `request fleet disable`.
   The freeze is best effort: it tells the operator early. What it misses,
   the hold catches (D-5).
4. **Hub guard.** Every writer of a client's configuration on a hub refuses a
   client no serving block declares. Today three of four do (D-2).

Resolving a diverged device from the hub (a diff, adopt or revert, an up-verb)
is `plan/post-release/features/spec-fleet-7-config-reconnect-resolution.md`.
Until it lands the operator resolves a hold with existing commands, stated in
Known Limitations.

Transport (TLS), authentication (per-client secret) and the four `pkg/fleet`
verbs and their envelopes are unchanged.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/fleet-config.md` - the managed client and hub protocol, the store layout, the two-phase change
  → Constraint: a client that is not connected is owed nothing and fetches when it connects; the hub may write its configuration at any time (D-2 keeps this)
  → Constraint: a configuration with no hub-level `client` block asks for no hub connection; the client stops following its hub and `doctor-hub-client-missing` reports it. This stays (D-3)
  → Decision: the sentence "Toggling it severs or establishes the hub connection without changing the config" is false at the producer today and becomes true through `request fleet disable` and `enable`; the page is corrected in the phase that lands them
- [ ] `docs/guide/fleet-config.md` - the operator walkthrough, the website page
  → Constraint: step 2 writes a client's first configuration before the client has ever connected, and "Change a Client's Configuration" promises a client that is not connected gets the change when it next connects. No hub-side rule may break either
- [ ] `ai/doors/system.md` - what every change must respect
  → Constraint: `pkg/zefs` and `pkg/fleet` are published. A new key in `pkg/zefs/keys.go` is a deliberate addition; no `pkg/fleet` type or verb changes in this spec
  → Constraint: a command is declared once with a YANG registration and reaches every interface; a readiness check is a doctor check registered by the package that owns the dependency
- [ ] `ai/doors/hub-engine.md` - daemon startup, reload, the managed client and server
  → Constraint: one reload path. A hub push and an operator commit both reach `doReload`; the freeze is placed before a change is staged, never as a second reload path
  → Constraint: something built only at startup ignores every later change. The hub client is built once at start today, which is why leave and rejoin need an owner
- [ ] `docs/architecture/cli/command-verbs.md` - what each verb promises
  → Decision: status is `show` (changes nothing). Leave and rejoin are `request`: the command ends or starts a session as well as storing a value, and a compound of two is neither `update` nor `set` (D-8, deciding the L-7 shape for this command)
  → Constraint: a command roots at the object its owner names; the owner is `internal/component/managed`, so the wire prefix is `ze-managed:`
- [ ] `ai/rules/cli.md` - grammar rules
  → Constraint: verb first, keywords before values, no `--flag` reaches the daemon. A grammar change is run through `./le cli grammar`
- [ ] `ai/META-DECISIONS.md`, "Goal and scope" - operator commands
  → Constraint: every operator command runs through the daemon on every surface with the same output as the local `ze` command, and the local route stays for when the daemon is unreachable
- [ ] `docs/architecture/doctor-and-health-checks.md` - doctor phases and coverage
  → Constraint: a check registers at the phase whose process holds the state it reads. A check over the store and the tree is post-config; one over live daemon state is runtime
  → Constraint: a new `Warn` or `Error` log line carries a code some doctor check declares, and a diagnostic carries its fix
- [ ] `ai/rules/user-facing-errors.md` - refusals
  → Constraint: a refusal is an operator error with problem, why and fix; the fix is the exact command alone on the last line
- [ ] `ai/rules/goroutine-lifecycle.md` - long-lived workers
  → Constraint: a goroutine that is started and stopped at runtime has one owner that cancels a context of its own and joins the worker; a replacement joins the previous worker first

**Key insights:**
- The device has no record of what its hub last gave it. That record is the whole fix; everything else is how the operator reaches it.
- About thirty routes write a device's configuration and no single point sits in front of them. Every versioned write does pass the store's candidate staging step, and so do the machine writers.
- The managed flag is read once at start. A daemon started with it off builds no hub client and runs a different start path.
- A "current" answer to a fetch means hub and device hold the same configuration. It is what resets the baseline and clears a hold, and it is what fleet-7's adopt will rely on.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/managed/client.go` - `RunManagedClient` loops over connections and reads `CheckManaged` only before each attempt. `fetchAndProcess` sends the in-memory `Version`, and applies whatever the hub returns through `OnCommit` unless the hub answers `current`. It ends for good on `errHubClientRemoved`. A failed validation or commit is acknowledged with `ok:false` and the connection is kept
  → Constraint: a hold that returns an error from `fetchAndProcess` ends the connection, and the backoff is reset at each successful authentication, so the device would reconnect about once a second. The hold returns no error
- [ ] `cmd/ze/ze_core_start.go` - `cmdStart` tests `isManaged` once, after the web-only branch. `cmdStartManaged` either builds a client from the cached configuration (`extractManagedClientConfig`, whose `Version` is the hash of the cached configuration) or, on a first start, fetches, judges and writes the configuration with a plain store write, then runs the daemon. `extractManagedClientConfig` answers nothing when the configuration has no hub `client` block
  → Constraint: a daemon started with the flag off has no client and takes the unmanaged path (web and MCP on, pushed-config inbox, health revert). A managed start has web and MCP off (D-7)
  → Decision: the first start writes the baseline too, since it does not pass through `OnCommit`
- [ ] `cmd/ze/hub/managed.go` - `wireManagedCommit` sets `OnCommit`: refused while a confirmed-commit window is open, otherwise `applyManagedPush` stages a candidate, reloads, and records an audit entry. The reload promotes the candidate
  → Decision: the baseline is written after the reload returns success, read back from the store
- [ ] `cmd/ze/hub/main.go` - `run` creates one cancel shared by the managed client and the managed server, launches `RunManagedClient` once on a copy of the client configuration, and joins it at shutdown. `InstallDataRPC` publishes the target the data RPCs act on, with `ServesClient` bound to `ManagedServer.Serves`
  → Constraint: the client cannot be stopped alone today, and nothing restarts it
- [ ] `cmd/ze/hub/config_upload.go`, `cmd/ze/hub/main_reload.go` - `configMutationOwner.Reload` is passed by every daemon-authored commit, after the candidate is staged, and carries no origin. The signal reload stages from the source and reloads without passing the owner
  → Constraint: a refusal placed in the reload comes after staging and leaves a candidate behind, which makes the next hub push fail. The freeze is placed before staging
- [ ] `internal/component/config/storage/pointer.go`, `restore.go`, `publication_intent.go`, `source.go` - `WriteCandidateVersionWithGuard` is the staging step every versioned write passes. `refuseConfigIntentLocked` refuses a write while a store key says something is unfinished, reading that key under the held lock. `BindConfigSource` records on the daemon's store handle which configuration is its own and whether it comes from the store, a file or stdin
  → Decision: the freeze extends the retained-intent refusal: one key read under the lock already held, at the staging step, scoped to the device's own stored configuration
- [ ] `internal/component/cli/editor_commit.go`, `internal/component/web/editor.go`, `internal/component/api/config_session.go`, `internal/component/config/storage/cli/data_rpc*.go` - the operator routes: session and console commits, the web commit and its full-content route, the API sessions, import, restore and rollback. All stage a candidate before the reload
  → Constraint: none of them is edited by this spec. The freeze reaches them through the store
- [ ] `cmd/ze/hub/session_factory.go` - `buildSessionModelFactory` lets a session name any stored configuration that exists, and drops the reload when it is not the daemon's own, so the commit is store-only
  → Decision: the declared-client refusal is added here, at session open
- [ ] `internal/component/config/storage/cli/data_rpc_config.go` - `unservedClientRefusal` refuses the import of a client configuration no serving block declares, and one on a daemon that serves no client at all
  → Constraint: the session editor refuses the same two cases with the same wording
- [ ] `internal/component/plugin/server/managed_serve.go`, `managed_doctor.go` - `handleRequest` records a failed acknowledgement as that client's last refusal, warns under `doctor-managed-client-rejected` and answers OK; the next successful acknowledgement clears it. `Serves` means declared
  → Decision: the hold reaches the hub through this record, with no wire change
- [ ] `internal/component/managed/doctor.go`, `register.go` - `hubClientDoctorCheck` is a post-config check over the managed flag and the tree, registered from `init`
  → Decision: the `diverged` check is its sibling, same phase, same registration
- [ ] `internal/plugins/update-cmd/cmd/serve_daemon.go` - `updateServeOwner` starts and stops one worker at runtime and joins it when the daemon stops
  → Decision: the hub client owner copies this shape
- [ ] `pkg/zefs/keys.go`, `pkg/zefs/tree.go` - `meta/instance/managed` is a value key; a key below a value is refused. The bootstrap keys live under `meta/managed/hub/`
  → Decision: the baseline key is `meta/managed/base-version`

**Behavior to preserve:**
- The four verbs, their envelopes, TLS and per-client secrets
- A device whose active configuration equals its baseline, or that has no baseline: fetch, apply, acknowledge, exactly as today
- The hub push while a confirmed-commit window is open: refused, as today
- A configuration with no hub `client` block ends the client and is reported by `doctor-hub-client-missing`
- A device that is not managed: every configuration write works as today
- A hub's writes to the configurations of the clients it declares, connected or not
- The seventeen tests of `test/managed/`

**Behavior to change:**
- A managed device records a baseline and holds when its active configuration differs from it
- `request fleet disable`, `request fleet enable` and `show fleet status` exist, on every surface and locally
- A managed device refuses an operator's change to its own stored configuration
- The hub's session editor refuses a client configuration no serving block declares
- `ze init --managed --force` removes the baseline

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Version hash and the `current` answer | `pkg/fleet` `VersionHash`; `internal/component/plugin/server/managed.go` `HandleConfigFetch` | A content hash; the hub answers `current` when the device already holds its configuration | Extends: the baseline is the same hash, and `current` resets it and clears a hold |
| Hub push wiring | `cmd/ze/hub/managed.go` `wireManagedCommit`, `applyManagedPush` | Stages the pushed configuration, reloads, audits | Extends: writes the baseline once the reload succeeds, and stages through the machine entry of the freeze |
| Failed acknowledgement and the hub's refusal record | `internal/component/managed/client.go` `fetchAndProcess`; `internal/component/plugin/server/managed_doctor.go` `recordRejected` | A device reports a configuration it refused; the hub shows it under `doctor-managed-client-rejected` | Extends: carries the hold and its reason to the hub. fleet-7 gives the hold its own cause |
| Retained-intent refusal at the store | `internal/component/config/storage/publication_intent.go` `refuseConfigIntentLocked` | Refuses a write while a store key says a publication is unfinished | Extends: the freeze is one more key read under the held lock, at candidate staging |
| Candidate staging | `internal/component/config/storage/pointer.go` `WriteCandidateVersionWithGuard` | The step every versioned configuration write passes | Extends: it admits or refuses an operator write; machine writers use a separately named entry |
| Config source binding | `internal/component/config/storage/source.go` `BindConfigSource`, `SourceMode` | Records which configuration is the daemon's own and where it comes from | Extends: it scopes the freeze to the device's own stored configuration |
| Outside-write refusal for an open confirmed commit | `internal/component/config/confirm` `WriteOutside` | Refuses a write no editor makes while a window is open; the hub push passes it | Does not apply as the freeze seam: it misses session and web commits, holds no store, and cannot tell the push from an operator. It stays as it is |
| Daemon mutation owner | `cmd/ze/hub/config_upload.go` `configMutationOwner` | One admission for daemon commits and reloads | Does not apply as the freeze seam: its reload comes after staging and carries no origin. Unchanged |
| Runtime start and stop of one worker | `internal/plugins/update-cmd/cmd/serve_daemon.go` `updateServeOwner` | Starts, stops and joins a listener at runtime | Extends by copy: the hub client owner has the same shape |
| Handler target filled by the hub | `internal/component/config/storage/cli/data_rpc.go` `DataRPCTarget`, `InstallDataRPC` | Lets a command handler reach daemon state without importing `cmd/ze/hub` | Extends by copy: the fleet commands reach the client owner the same way |
| Declared-client guard | `internal/component/config/storage/cli/data_rpc_config.go` `unservedClientRefusal`; `ManagedServer.Serves` | Three hub writers refuse a client no serving block declares | Extends: the session editor joins them |
| Managed doctor check | `internal/component/managed/doctor.go` `hubClientDoctorCheck` | Post-config check on the flag and the tree | Extends: a sibling check reports `diverged` |
| Endpoint re-read | `internal/component/managed/client.go` `adoptEndpoint` | The client follows the hub block of each configuration it applies | Unchanged. A held device applies nothing, so it does not follow a hub block change either (R-5) |
| Local command that asks the daemon first | `internal/component/doctor/local_route.go` `answerLocalWith` | `ze doctor` answers alone and asks a running daemon for more | Extends by copy: `show fleet status` |
| Store write with both routes | `internal/component/config/storage/daemonroute` `Send` | A local mutation needs the store; a busy store sends the line to the daemon | Extends by use: `request fleet disable` and `enable` with the daemon running or stopped |
| Running-config hash key | `pkg/zefs/keys.go` `KeyConfigActiveHash`, `cmd/ze/pushed_config.go` `writeConfigActiveHash` | Hash of the running configuration, written on the unmanaged start path | Does not apply: it tracks what runs, never what the hub last gave. A second meaning on that key would be wrong |
| Editor conflict states | `internal/component/cli/editor_commit.go` `ConflictStale`, `ConflictLive` | Conflicts between concurrent edit sessions | Does not apply. It is why the fleet state is named `diverged` |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A managed device connects to its hub, or its hub notifies it of a change
- An operator runs `request fleet disable`, `request fleet enable` or `show fleet status`, on any surface or locally
- An operator changes a managed device's configuration through any write route
- An operator opens a client's configuration in a hub's session editor

### Transformation Path
1. **Baseline write.** When a hub configuration has become active (a push whose reload succeeded, or the first start's fetched configuration), the device reads its active configuration back, hashes it and stores the hash under `meta/managed/base-version`.
2. **Fetch decision.** Before each fetch the client reads the active hash and the baseline from the store. It sends the active hash as its version.
3. **Clean.** No baseline, or active equals baseline: the answer is handled as today. A `current` answer with no baseline stores one.
4. **Edited, hub agrees.** Active differs from baseline and the hub answers `current`: the baseline is set to the active hash. Nothing is held.
5. **Edited, hub differs.** Active differs from baseline and the hub returns a configuration: the device does not validate, commit or apply it. It sends an acknowledgement with `ok:false` whose error says the device is holding, with both hashes, logs a warning under `doctor-managed-diverged`, keeps the connection and returns no error.
6. **Hub side of a hold.** The hub records the refusal as that client's last refusal and shows it under `doctor-managed-client-rejected`, as for any refused configuration.
7. **Disable.** The handler stores the flag as false, then asks the client owner to stop: the owner cancels the client's own context and joins it, so a push in flight finishes first. With the daemon stopped the local route stores the flag.
8. **Enable.** The handler refuses when the active configuration names no hub `client` block. Otherwise it stores the flag as true and, on a daemon that started managed, asks the owner to build and start a client. On a daemon that started unmanaged it answers that the connection starts at the next `ze start`. The owner exists on every managed start, including one whose configuration named no hub and so started no client: the client is built from the active configuration at the moment it is started, never from what the daemon read at boot.
9. **Status.** The local handler asks a running daemon first; the daemon answers with the store facts plus the client and connection state. With no daemon the store facts are the answer.
10. **Freeze.** The store's staging step, for the device's own stored configuration, reads the flag under the held lock and refuses before any version is written. Machine writers enter through a separately named entry that does not read the flag.
11. **Hub guard.** The session factory, for a name of the client-configuration form, asks the same declared-client predicate the data RPCs use and refuses the session.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Managed client to store | Three callbacks on the client configuration (read active hash, read baseline, store baseline), as `CheckManaged` and `OnCommit` are today | No |
| Device to hub | The existing `config-fetch` and `config-ack` envelopes, unchanged | No |
| Command handler to client owner | A target with function fields, installed by the hub and withdrawn before the store closes | No |
| Operator route to freeze | The store's staging step | No |
| Session editor to managed server | The declared-client predicate the hub already installs for the data RPCs | No |

### Changed-Wire Census

**Search evidence:** Searched the whole tree outside `tmp/`, `bin/` and `.claude/worktrees/`, in every file type, for `meta/instance/managed`, `KeyInstanceManaged`, `meta/managed/`, `config-ack`, `ConfigAck` and `recordRejected`. Positive control: the search finds the known writer `internal/plugins/init/main.go` and the known reader `cmd/ze/ze_core_start.go` `isManaged`. The only non-Go file that touches the flag is `test/managed/init-managed-key.ci`, which reads it through `ze data`. No shell or Python script reads or writes either key. `pkg/fleet` envelopes are not changed, so they have no row: the acknowledgement keeps its three fields and the hold uses the existing `error` text.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| Store key `meta/managed/base-version` (new) | writer | `cmd/ze/hub/managed.go` `applyManagedPush`, after the reload | New; the reload promotes the candidate before it returns | New write |
| same | writer | `cmd/ze/ze_core_start.go` `cmdStartManaged`, first start | New; the first start does not pass `OnCommit` | New write |
| same | writer | `internal/component/managed/client.go` `fetchAndProcess`, on a `current` answer | New | New write, through the store-baseline callback |
| same | writer | `internal/plugins/init/main.go`, `--managed --force` | New; the force path already replaces the stored hub root | Removes the key (D-6) |
| same | reader | `internal/component/managed/client.go` `fetchAndProcess` | New | New read, through the read-baseline callback |
| same | reader | `internal/component/managed/doctor.go`, the new `diverged` check | New | New read |
| same | reader | The `show fleet status` handler | New | New read |
| Store key `meta/instance/managed` (existing) | writer | `internal/plugins/init/main.go`; `internal/appliance/cmd_assemble.go`; `update data key` (`handleDataKeyUpdate`) | Found by the search | Preserved unchanged |
| same | writer | The `request fleet disable` and `enable` handlers, daemon and local | New | New writes of the same values, `true` and `false` |
| same | reader | `cmd/ze/ze_core_start.go` `isManaged`; `internal/component/managed/doctor.go` `checkHubClientDeclared` | Found by the search | Preserved; the readers and the new ones share one helper |
| same | reader | The store's staging step (freeze); the `show fleet status` handler; `test/managed/init-managed-key.ci` | New, and found by the search | New reads; the test is unchanged |
| `config-ack` `error` text (existing field, new content) | writer | `internal/component/managed/client.go` `fetchAndProcess` | The field is free text today | A new text for the hold; the envelope is unchanged |
| same | reader | `internal/component/plugin/server/managed_serve.go` `handleRequest`, then `managed_doctor.go` `recordRejected`, then `show doctor` | The hub stores and shows the text and parses nothing in it | Preserved: no reader depends on its wording |
| Command RPCs `ze-managed:` (new) | writer | Every command surface and the local `ze` route | New | New |
| same | reader | The handlers in `internal/component/managed` | New | New |

### Integration Points
- `managed.ClientConfig` - gains the three store callbacks; `Version` stops being the version a fetch sends
- `cmd/ze/hub` `run` - builds the client owner on a managed start, installs the command target, withdraws it before the store closes
- `cmd/ze` `cmdStartManaged` - hands the hub a builder of the client configuration in place of a built one, so each start of the client reads the endpoint afresh
- `storage` staging - one admission, one machine entry
- `diagnostic` - one new code and its catalog entry

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The hash of the active configuration is stable across a restart when nobody edits: what the daemon reads back as active is byte-identical from one boot to the next | `extractManagedClientConfig` hashes the cached configuration and `test/managed/client-cached-boot.ci` passes today | A device holds after every restart. The baseline is read back from the store, not taken from the pushed bytes, to keep this true even if a write normalises | `TestBaselineStableAcrossRestart`, and `fleet-hold-keeps-local-edit.ci` restarting an unedited device first | confirmed, run 2026-11-19: `TestBaselineStableAcrossRestart` and `fleet-hold-keeps-local-edit.ci` (an unedited device restarts and follows its hub) green. Reading: the boot record (`cmd/ze/hub/config_source.go` `initializeConfigSource`, `storage.EnsureActiveVersion`) versions the bytes the daemon read and does not rewrite the stored configuration, so the plain read `managed.ActiveVersion` hashes is the same from one boot to the next |
| A-2 | The plain read of the configuration name and the store's active-version read answer the same bytes on a stored-source daemon | `extractManagedClientConfig` uses the plain read; the reload promotes through the version pointers | The hold and the doctor check disagree. One helper computes the active hash for every reader | `TestActiveHashOneHelper`: client, doctor and status agree after a push | confirmed, run 2026-11-19: `TestActiveHashOneHelper` and `TestActiveHashReadsThePointedVersion` green. Reading: `storage` `promoteVersionLocked` writes the promoted bytes to the configuration name after it moves the active pointer, so the two reads agree after a push. The exception: that mirror write may fail and is only logged, and a raw write of the configuration key with the daemon stopped changes the plain read and not the pointed version the daemon boots (`storage.ReadActiveConfig`). All readers still agree with each other through `managed.ActiveVersion`; in the first case the device re-applies its hub's configuration at the next fetch, in the second it holds on a configuration the daemon does not run |
| A-3 | Every machine writer of a device's own configuration is one of: the hub push, the confirmed-commit revert (running and at boot), the signal reload, the boot record, schema recovery | Write census of 2026-11-18, thirty routes, read not run | A managed device refuses its own push or its own recovery | AC-13: a test drives each on a managed store | confirmed, run 2026-11-19: `TestMachineWritersOnManagedStore` and `fleet-freeze-refuses-commit.ci` (a hub push still applies on a frozen device) green. Reading: every non-test caller of the store's staging functions was listed with `gopls references`. The machine writers are `applyManagedPush`, `confirmRevert`, `publishRecoveredConfig` (schema recovery and, through `restoreConfirmOnStart`, the boot revert), `initializeConfigSource` (the explicit-file arm of the boot record) and `stageSIGHUPCandidate`, and no other was found. The stored-source boot record uses `storage.EnsureActiveVersion`, which stages no candidate and so never met the freeze |
| A-4 | A daemon-stopped tool that opens the store can tell the device's own configuration by the instance name key, as `internal/core/resolve` `DefaultConfig` does | `DefaultConfig`; stopped tools do not bind a config source | The freeze does not apply to stopped tools, or freezes the wrong name | `TestFreezeScopeStoppedTool`: own name refused, another name admitted | confirmed, run 2026-11-19: `TestFreezeScopeStoppedTool`, `TestFreezeReadsTheNameFirst` green. Reading: the name rule moved to `storage.InstanceConfigName`, which `resolve.DefaultConfig` and the freeze (`storage` `ownConfigNameLocked`) both call. Two limits found: an explicit blob artifact is never frozen (`isBlobArtifact`), and `storage.ImportConfig` of a name the store does not hold yet writes the key without staging, so it is not refused |
| A-5 | The reload's last step can fail after the candidate is promoted, leaving the configuration active and the push reported as failed | Read in `runReloadContext`; the condition was not determined | None for safety: the next fetch sends the active hash, the hub answers `current`, and the baseline is set | `TestCurrentAnswerSetsBaseline` | confirmed at closure by reading, outcome proven by run 2026-11-19 (`TestCurrentAnswerSetsBaseline`, `TestHoldEndedOnCurrentTellsTheHub` green). `cmd/ze/hub/main_reload.go` `runReloadContext` promotes through `promoteReloadCandidate` (`promoteConfigCandidate` for a commit) and then returns `acceptReload(true)`, the acceptance `pluginserver.Server.DeferReloadAcceptance` hands it, which returns an error of its own: a push can be reported failed with its configuration already active. The outcome is covered: such a push writes no baseline (`applyManagedPush` calls `recordPushBaseline` only on a nil reload), and the next fetch sends the active hash, so `current` records it (`managed` `settleBaseline`) |
| A-6 | Cancelling the client's own context ends a live connection promptly | `notificationLoop` selects on the context | `request fleet disable` is not prompt | `TestOwnerStopEndsLiveConnection` | confirmed, run 2026-11-19: `TestOwnerStopEndsLiveConnection` and `fleet-disable-severs.ci` green. Reading: `managed` `runConnection` passes the context to the dial, the handshake, the ping sender and `notificationLoop`, which selects on it, and `RunManagedClient` returns on `ctx.Err()` before any backoff. A client inside `OnCommit` ends only when the reload returns (R-6) |
| A-7 | `ze config edit` against a stopped managed store starts a full managed daemon, so its commit meets the freeze and is refused with the same message | `runStoredEditor` starts a plain `ze start`; read, not run | The operator gets a different or missing refusal | `fleet-freeze-refuses-commit.ci`, stopped-daemon step | confirmed at closure by reading, not run (no test drives `ze config edit` against a stopped managed store; `fleet-freeze-refuses-commit.ci` drives the stopped `ze config import` and `ze data import` routes, and the running rollback RPC). The path, read to the end: `internal/component/config/cli/cmd_edit.go` `runStoredEditor` finds no daemon (`probeDaemonSSH`) and calls `startEphemeralDaemon`, which runs `ze start` with `ZE_SSH_EPHEMERAL` and `ZE_CONFIG_DIR` set to the store folder. `cmd/ze/ze_core_start.go` `cmdStart` binds the store to its stored configuration (`storage.BindConfigSource(..., ConfigSourceStored)`) before it tests `isManaged`, so the daemon is a managed start (`cmdStartManaged` -> `hub.RunWithManagedClient` -> `runYANGConfig`). Both SSH starts of `runYANGConfig` (the no-BGP branch in `cmd/ze/hub/main.go`, the BGP branch in `infra_setup.go`) listen on `127.0.0.1:0` when `ze.ssh.ephemeral` is set and the configuration declares no listener. The session's commit, `internal/component/cli/editor_commit.go` `commitSessionCandidate`, stages through `storage.WriteCandidateVersionWithGuard`, whose `refuseFrozenConfigLocked` takes the stored arm of `configFrozen` and refuses with `ErrConfigFrozen`. The operator sees the freeze's problem, reason and fix (`request fleet disable`, last) under the editor's `write candidate:` prefix, as on every route that prefixes its first line. One side effect: the ephemeral daemon is a full managed start, so it connects to the hub while the editor is open, and a hub push it receives then is applied |
| A-8 | No reader depends on the wording of a failed acknowledgement's error | `handleRequest` stores and shows it | A hold is misread on the hub | The census search above; `TestHubRecordsHoldRefusal` | confirmed, run 2026-11-19: `TestHubRecordsHoldRefusal` and `fleet-hold-keeps-local-edit.ci` (the hub lists the hold text) green. Reading: `managed_serve.go` `handleRequest` passes `ack.Error` to `recordRejected` and to one log attribute and compares nothing in it; no hub code changed for the hold |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A machine writer is missed and a managed device refuses its own hub push or boot recovery | A push acknowledged `ok:false` with the freeze text; a boot that stops | AC-13 drives every machine writer on a managed store. The freeze text names its seam so the cause is plain |
| R-2 | A new operator route is written against the machine entry and escapes the freeze | Review of a caller of the machine entry | The hold still protects the edit (D-5). The machine entry's doc names the permitted callers |
| R-3 | Boot recovery or a confirmed-commit revert changes the active configuration with no operator involved, and the device holds | `diverged` on a device nobody edited | The doctor text says the configuration differs from the last one the hub sent, never that it was edited. Resolution is the same |
| R-4 | A held device floods the hub's log, one warning per hub write | Repeated `doctor-managed-client-rejected` for one client | One refusal per fetch, and a fetch happens once per connection and once per hub write. Bounded by the hub's own writes |
| R-5 | A held device does not follow a change of its hub's address or secret, since it applies nothing, and loses its hub | The device stops reconnecting successfully while diverged | Visible on both sides as diverged before it happens. Resolution restores it |
| R-6 | `request fleet disable` waits on a push in flight, and a reload that hangs makes it hang | The command does not return | The wait is the join the client's contract already requires. The command reports that it is waiting for a configuration in flight |
| R-7 | A hub that is itself a managed device has its client configurations frozen | An import of a client configuration refused on a hub | The freeze is scoped to the device's own configuration; AC-12 proves a hub's client configuration stays writable |
| R-8 | The managed flag cannot be read at the staging step | A write refused with a read error | Refuse, naming the key and the store check. A store that cannot answer one key is not safe to write |
| R-9 | A daemon started unmanaged and then enabled is frozen at once but has no hub connection until its next start | An operator who cannot edit and sees no hub | `request fleet enable` says so in its answer, and `show fleet status` shows client not started. D-7 chose this |
| R-10 | Disable and enable race each other or a shutdown | A second client, or a client that outlives the store | The owner holds one running slot under a lock, refuses a second run, and is joined before the store closes |
| R-11 | Phase 2 lands before the freeze, so any local edit on a managed device makes it hold | A diverged device after an ordinary edit | Accepted by D-1. The hold is still the correct outcome; the freeze follows in the same spec |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A managed device stops taking its hub's configuration (a false hold), or refuses its own hub push or boot recovery (a missed machine writer). Neither drops a running session: the device keeps the configuration it runs |
| How is it reverted? | Single commit revert per phase. The baseline key is ignored by older code. No configuration migration |
| Who else touches this path? | `spec-fleet-7` (resolution, built on the baseline and the hold), `spec-fleet-1` (reads device state), the `fleet-ui` set (renders status), `spec-managed-server-hardening` (the managed listener) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A device with a local edit reconnects to a hub that holds another configuration | → | The fetch decision holds: nothing applied, acknowledgement refused, doctor code raised | `test/managed/fleet-hold-keeps-local-edit.ci` |
| The hub is given the device's configuration | → | A `current` answer resets the baseline and the hold ends | `test/managed/fleet-hold-clears-on-match.ci` |
| `request fleet disable` on a connected device | → | The owner stops the client; the hub sees the connection close | `test/managed/fleet-disable-severs.ci` |
| `request fleet enable` on a device that left at runtime | → | The owner starts the client; the device fetches | `test/managed/fleet-enable-rejoins.ci` |
| `show fleet status` with and without a daemon | → | The status handler, local and daemon | `test/managed/fleet-status.ci` |
| An operator commit on a managed device | → | The store's staging step refuses | `test/managed/fleet-freeze-refuses-commit.ci` |
| A hub push to a managed device | → | The machine entry admits it and the baseline is stored | `test/managed/fleet-hold-keeps-local-edit.ci`, whose first step reads the baseline after a real push. The existing `test/managed/config-push-transactional.ci` keeps its one question, whether the acknowledgement follows the commit |
| A session editor opened on an undeclared client configuration | → | The session factory refuses | `test/managed/fleet-hub-editor-declared.ci` |
| `ze doctor` on a diverged device | → | The `diverged` check | `test/managed/fleet-diverged-doctor.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A hub push is applied by a managed device | The device runs the pushed configuration, acknowledges `ok:true`, and `meta/managed/base-version` holds the hash of its active configuration | A push refused by an open confirmed-commit window or by validation leaves the baseline as it was: nothing from the hub became active |
| AC-2 | A managed device makes its first start from `ze init --managed` | The device runs the fetched configuration and the baseline holds its hash before the client makes its first reconnect | None: every first start that reaches a running daemon stores it |
| AC-3 | A managed device whose active configuration equals its baseline restarts, and the hub's copy has changed meanwhile | The device applies the hub's configuration and stores the new baseline, as today | None: an unedited device always follows its hub |
| AC-4 | A managed device whose active configuration differs from its baseline connects, and the hub holds another configuration | The device keeps running its own configuration, stays connected and answering heartbeats, sends an acknowledgement with `ok:false` naming the hold and both hashes, and logs one warning under `doctor-managed-diverged` | The hub's configuration is fetched but neither validated nor stored on the device: the fetch is how the device learns the hub differs |
| AC-5 | A held device is notified of a further hub change | The device answers the notification OK, holds again, and its connection stays up | One refused acknowledgement per hub write: the hub must see each refusal to show the current one |
| AC-6 | A managed device whose active configuration differs from its baseline connects, and the hub holds the same configuration as the device | The hub answers `current`, the baseline is set to the active hash, `show fleet status` reports the device in sync and `ze doctor` reports no divergence | None |
| AC-7 | A managed device with no stored baseline connects and the hub's copy differs | The device applies the hub's configuration as today and stores the baseline | The one unprotected fetch of a device provisioned before this spec, accepted by D-6 |
| AC-8 | `ze doctor` runs on a managed device whose active configuration differs from its baseline, daemon running or stopped | It reports `doctor-managed-diverged` as a warning, saying the configuration differs from the last one the hub sent, with the fix | A device that is not managed and has local changes reports nothing: outside the fleet a local change is not a problem |
| AC-9 | A hub whose client refused a configuration because it holds | `show doctor` on the hub lists that client under `doctor-managed-client-rejected` with the device's hold text; the entry clears at the client's next successful acknowledgement | The hold shares the "refused configuration" cause until fleet-7 gives it its own |
| AC-10 | `request fleet disable` on a connected managed device | The flag is false, the hub sees the connection close within the join of one configuration in flight and well inside the 90 second heartbeat timeout, the device keeps running its configuration, and a local commit is then accepted | A push already being applied completes and stores its baseline before the connection closes |
| AC-11 | `request fleet enable` on a daemon that started managed, after `request fleet disable` in the same run or after a start whose configuration named no hub and has since gained one | The flag is true, a client built from the active configuration starts, and the device fetches: AC-3 when it made no edit, AC-4 or AC-6 when it did | None |
| AC-12 | `request fleet enable` on a daemon that started with the flag off | The flag is true and the answer states that the hub connection starts at the next `ze start`; `show fleet status` shows managed with the client not started | The daemon keeps the services of its unmanaged start until it restarts (D-7) |
| AC-13 | `request fleet enable` when the active configuration names no hub `client` block | Refused with an operator error whose fix is the hub-side fix `doctor-hub-client-missing` names; the flag is left as it was | `request fleet disable` on the same device succeeds: the exit works with no hub (D-3) |
| AC-14 | `request fleet disable` and `request fleet enable` with the daemon stopped | The local route stores the flag and answers; the next `ze start` follows it | With a daemon holding the store the same line is sent to that daemon |
| AC-15 | `show fleet status` | It reports managed, frozen, the hub name and address the configuration declares, the baseline and active hashes, whether they differ, and from a running daemon the client state, the connection state and whether a fetch is being held. The local command and every daemon surface give the same fields | With no daemon the client and connection fields say that no daemon answered |
| AC-16 | An operator changes a managed device's own stored configuration by a session commit, the web editor, an API session, an import, a restore or a rollback | The change is refused with an operator error: the device is managed, local changes are frozen so a hub push cannot overwrite one, and the fix is `request fleet disable`. No version and no candidate is left in the store | A file-sourced daemon is not frozen: its file is the authority and it runs no hub client. Raw key writes, a full-store restore and a rename with the daemon stopped are outside the freeze and are covered by the hold (D-5) |
| AC-17 | A hub push, a confirmed-commit revert, a signal reload, the boot record and schema recovery each run on a managed device | Each stages and completes as on an unmanaged device | None: a machine writer the freeze refuses is a defect |
| AC-18 | A configuration written to a managed device with the daemon stopped by a route the freeze does not cover, then the device starts and connects to a hub holding another configuration | The device holds as in AC-4 and keeps the written configuration | None: this is the guarantee of D-5 |
| AC-19 | A hub that is itself managed imports or edits the configuration of a client it declares | The write is accepted and the client is notified, as today | Its own configuration is frozen as in AC-16 |
| AC-20 | An operator opens the session editor on a hub for a client configuration no serving block declares, or on a daemon that serves no client | The session is refused with the operator error the import gives for the same case | A declared client's configuration opens and commits as today, connected or not (D-2) |
| AC-21 | `ze init --managed --force` on a device that holds a baseline | The baseline is removed with the other managed bootstrap state, and the next start behaves as AC-2 | None |
| AC-22 | A device that is not managed | Every configuration write works as today and no fleet warning is raised | `show fleet status` reports it as not managed |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Changes a managed router in an emergency, then lets it rejoin | disable -> owner stops client -> commit accepted -> enable -> owner starts client -> fetch decision -> hold | `fleet-disable-severs.ci`, `fleet-hold-keeps-local-edit.ci` |
| 2 | Tries to edit a router that is in a fleet | commit -> store staging -> refusal naming `request fleet disable` | `fleet-freeze-refuses-commit.ci` |
| 3 | Leaves the fleet, changes nothing, rejoins | disable -> enable -> fetch decision -> clean -> hub configuration applied | `fleet-enable-rejoins.ci` |
| 4 | Keeps the router's change by giving it to the hub | import on the hub -> notification -> fetch -> `current` -> baseline reset | `fleet-hold-clears-on-match.ci` |
| 5 | Asks a router where it stands | `show fleet status`, locally or through the daemon | `fleet-status.ci` |
| 6 | Looks at the hub to find the routers that are holding | failed acknowledgement -> hub refusal record -> `show doctor` | `fleet-hold-keeps-local-edit.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestFetchHoldsWhenActiveDiffersFromBaseline` | `internal/component/managed/client_hold_test.go` | A returned configuration is neither validated nor committed when active differs from baseline | |
| `TestHoldAcknowledgesRefusalAndKeepsConnection` | `internal/component/managed/client_hold_test.go` | The hold sends `ok:false` and returns no error | |
| `TestCurrentAnswerSetsBaseline` | `internal/component/managed/client_hold_test.go` | A `current` answer stores the active hash as the baseline | |
| `TestNoBaselineAppliesAsToday` | `internal/component/managed/client_hold_test.go` | An absent baseline is clean | |
| `TestFetchSendsActiveHash` | `internal/component/managed/client_hold_test.go` | The version a fetch sends is read at fetch time | |
| `TestBaselineWrittenAfterPush` | `cmd/ze/hub/managed_test.go` | A successful push stores the hash of the active configuration | |
| `TestBaselineUnchangedWhenPushFails` | `cmd/ze/hub/managed_test.go` | A push whose reload fails leaves the baseline | |
| `TestBaselineWrittenOnFirstStart` | `cmd/ze/ze_core_start_bootstrap_test.go` | The first start stores the baseline | |
| `TestBaselineStableAcrossRestart` | `cmd/ze/ze_core_start_bootstrap_test.go` | An unedited device has active equal to baseline after a restart (A-1) | |
| `TestActiveHashOneHelper` | `internal/component/managed/baseline_test.go` | The client, the doctor check and status compute the same hash (A-2) | |
| `TestDivergedDoctorCheck` | `internal/component/managed/doctor_test.go` | Managed and differing reports the code with its fix | |
| `TestDivergedDoctorSilentWhenUnmanaged` | `internal/component/managed/doctor_test.go` | An unmanaged device with local changes reports nothing | |
| `TestHubRecordsHoldRefusal` | `internal/component/plugin/server/managed_serve_test.go` | A hold acknowledgement is that client's last refusal, with its text | |
| `TestOwnerStopEndsLiveConnection` | `cmd/ze/hub/managed_owner_test.go` | Stop ends a connected client and joins it (A-6) | |
| `TestOwnerRefusesSecondRun` | `cmd/ze/hub/managed_owner_test.go` | Start while running starts nothing | |
| `TestOwnerStartAfterStop` | `cmd/ze/hub/managed_owner_test.go` | A stopped client starts again with a freshly built configuration | |
| `TestOwnerStartsWhenBootHadNoHubBlock` | `cmd/ze/hub/managed_owner_test.go` | A managed start whose configuration named no hub has an owner, and a start after a hub block is added builds a client from the active configuration | |
| `TestOwnerJoinedBeforeStoreCloses` | `cmd/ze/hub/managed_owner_test.go` | Shutdown joins the client before the store is returned | |
| `TestFleetDisableStoresFlagThenStops` | `internal/component/managed/cmd_fleet_test.go` | Disable stores false before it stops the client | |
| `TestFleetEnableRefusesWithoutHubBlock` | `internal/component/managed/cmd_fleet_test.go` | Enable refuses and leaves the flag | |
| `TestFleetEnableOnUnmanagedStart` | `internal/component/managed/cmd_fleet_test.go` | Enable stores true and answers that the connection starts at the next start | |
| `TestFleetStatusStoreOnly` | `internal/component/managed/cmd_fleet_test.go` | Status with no daemon answers the store facts | |
| `TestFreezeRefusesOperatorStaging` | `internal/component/config/storage/freeze_test.go` | Staging the device's own configuration is refused while managed, and no version is written | |
| `TestFreezeAdmitsMachineEntry` | `internal/component/config/storage/freeze_test.go` | The machine entry stages while managed | |
| `TestFreezeAdmitsOtherConfig` | `internal/component/config/storage/freeze_test.go` | Another configuration name is admitted while managed (R-7) | |
| `TestManagedHubImportsDeclaredClientConfig` | `cmd/ze/hub/managed_hub_freeze_test.go` | A managed hub bound to its own stored configuration imports a declared client's configuration through `update data config`, records the version and notifies the client, while its own configuration is refused by the freeze (AC-19) | |
| `TestFreezeAdmitsFileSource` | `internal/component/config/storage/freeze_test.go` | A file-sourced handle is admitted | |
| `TestFreezeScopeStoppedTool` | `internal/component/config/storage/freeze_test.go` | An unbound handle scopes by the instance name (A-4) | |
| `TestFreezeRefusesOnUnreadableFlag` | `internal/component/config/storage/freeze_test.go` | A flag that cannot be read refuses, naming the key (R-8) | |
| `TestMachineWritersOnManagedStore` | `cmd/ze/hub/managed_machine_writers_test.go` | The confirmed-commit revert, the signal reload, the boot record and schema recovery each complete on a managed store (A-3, AC-17) | |
| `TestSessionEditorRefusesUndeclaredClient` | `cmd/ze/hub/session_factory_test.go` | A client configuration no block declares is refused at session open | |
| `TestSessionEditorOpensDeclaredClient` | `cmd/ze/hub/session_factory_test.go` | A declared client's configuration opens | |
| `TestInitForceRemovesBaseline` | `internal/plugins/init/managed_hub_test.go` | `--managed --force` removes the baseline | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| None | The feature takes no numeric input: hashes are compared as text and the commands take no value | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `fleet-hold-keeps-local-edit` | `test/managed/fleet-hold-keeps-local-edit.ci` | An unedited device restarts and follows its hub. The same device, edited with the daemon stopped by a route the freeze misses, starts against a hub holding another configuration: it keeps its edit, the hub's doctor lists the refusal | |
| `fleet-hold-clears-on-match` | `test/managed/fleet-hold-clears-on-match.ci` | The device's configuration is imported on the hub; the device stops holding and reports in sync | |
| `fleet-diverged-doctor` | `test/managed/fleet-diverged-doctor.ci` | `ze doctor` on a diverged device names the code and the fix; on an unmanaged device with local changes it is silent | |
| `fleet-disable-severs` | `test/managed/fleet-disable-severs.ci` | `request fleet disable` on a connected device: the hub sees it leave, a commit is then accepted | |
| `fleet-enable-rejoins` | `test/managed/fleet-enable-rejoins.ci` | Disable then enable with no edit: the device reconnects without a restart and follows its hub | |
| `fleet-status` | `test/managed/fleet-status.ci` | `show fleet status` with the daemon running and stopped, managed and not | |
| `fleet-freeze-refuses-commit` | `test/managed/fleet-freeze-refuses-commit.ci` | A commit on a managed device is refused naming `request fleet disable`; `ze config edit` against the stopped store gives the same refusal; a hub push still applies | |
| `fleet-hub-editor-declared` | `test/managed/fleet-hub-editor-declared.ci` | A hub's session editor refuses an undeclared client configuration and opens a declared one whose client is not connected | |

## Files to Modify
- `internal/component/managed/client.go` - the fetch decision, the three store callbacks, the hold acknowledgement and its warning
- `internal/component/managed/doctor.go` - the `diverged` check and the shared flag helper
- `internal/component/managed/register.go` - register the check and the commands
- `cmd/ze/hub/managed.go` - baseline write after a successful push; stage through the machine entry
- `cmd/ze/hub/main.go` - build and join the client owner, install and withdraw the command target; machine entry for the boot record and schema recovery
- `cmd/ze/hub/main_reload.go`, `cmd/ze/hub/confirm_window.go` - machine entry for the signal reload and the confirmed-commit revert
- `cmd/ze/hub/session_factory.go` - the declared-client refusal
- `cmd/ze/ze_core_start.go` - baseline on the first start; hand the hub a builder of the client configuration; the store callbacks
- `internal/component/config/storage/pointer.go` - admission at staging, and the machine entry
- `internal/component/config/storage/source.go` - the scope the admission reads
- `internal/core/diagnostic/codes_plugin_infra.go` - `doctor-managed-diverged` and its catalog entry
- `internal/plugins/init/main.go` - remove the baseline on `--managed --force`
- `pkg/zefs/keys.go` - register `meta/managed/base-version`
- `internal/component/plugin/all/testdata/wire-methods.snapshot` - the three RPCs
- `docs/architecture/fleet-config.md` - the baseline, the hold, leave and rejoin, the freeze; correct the "Toggling it" sentence
- `docs/guide/fleet-config.md` - sections on the freeze, on leaving and rejoining the fleet, and on a diverged device; the store-keys reference
- `docs/architecture/doctor-and-health-checks.md` - the row for the new check in "External plugin and managed fleet checks"
- `docs/architecture/zefs-format.md` - the new key
- `docs/guide/command-reference.md`, `docs/architecture/api/commands.md` - the three commands
- `features/fleet-management.md` - the real-path tests

## Files to Create
- `internal/component/managed/baseline.go` - the active-hash helper, baseline read and write
- `internal/component/managed/cmd_fleet.go` - the three handlers, the command target, the local routes
- `internal/component/managed/yang/ze-managed-cmd.yang` - the `fleet` container under `request` and `show`
- `internal/component/config/storage/freeze.go` - the admission and its refusal
- `cmd/ze/hub/managed_owner.go` - the hub client owner
- The test files named in the TDD Test Plan
- `test/managed/fleet-hold-keeps-local-edit.ci`, `fleet-hold-clears-on-match.ci`, `fleet-diverged-doctor.ci`, `fleet-disable-severs.ci`, `fleet-enable-rejoins.ci`, `fleet-status.ci`, `fleet-freeze-refuses-commit.ci`, `fleet-hub-editor-declared.ci` - drafted under `test/draft/managed/` and promoted through `/ze-test`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/managed/yang/ze-managed-cmd.yang`: three command RPCs. No config leaf |
| YANG validation constraints | N-A | The commands take no value |
| YANG custom validators | N-A | No leaf |
| CLI commands/flags | Yes | `internal/component/managed/cmd_fleet.go`, registered from `register.go`. No offline flag |
| CLI grammar (keyword before value) | Yes | `request fleet disable`, `request fleet enable`, `show fleet status`; `./le cli grammar`, `./le cli ownership`, `./le cli catalog`, `./le doc yang-contract command-contract` |
| Editor autocomplete | Yes | Automatic from the YANG command tree |
| Functional test for new RPC/API | Yes | `test/managed/fleet-disable-severs.ci`, `fleet-enable-rejoins.ci`, `fleet-status.ci` |
| Pipe completeness | Yes | `show fleet status` registers its shape and columns as a local-data command, with its `internal/test/localdatacoverage` evidence row |
| Env var registration | N-A | No environment value |
| Doctor check for runtime dependencies | N-A | No new file path, socket, port, binary or certificate. The new store key is state, covered by the row below |
| Doctor code per problem class | Yes | Hold on the device: `doctor-managed-diverged`, check in `internal/component/managed/doctor.go`, post-config, with its warning line in `client.go`. Hold seen by the hub: the existing `doctor-managed-client-rejected`, runtime. The freeze refusal and the enable refusal are operator errors with a fix and raise no log line. The session editor refusal is the existing operator error of the import |
| Prometheus counters/metrics | No | The existing `ze_managed_config_fetch_total` and the hub's refusal record show a hold. A gauge of diverged devices belongs to the hub once fleet-7 gives the hold its own cause |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not a protocol change |

Discovery (`ai/rules/repo-maintenance.md`): an agent finds this through the `ai/INDEX.md` row "fleet, managed, server, backup, bootstrap", which names `docs/architecture/fleet-config.md`, and through `ai/doors/hub-engine.md`. Regression is prevented by the functional tests above and by `./le arch doctor-coverage`. Drift is prevented by the command registry and its `wire-methods.snapshot`, the `pkg/zefs` key registry and the doctor check registry. Verification is `./le --name <session> test managed -a`.

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/fleet-management.md`: the new real-path tests |
| 2 | Config syntax changed? | No | No YANG config leaf is added or changed |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | No | The commands belong to the existing managed component |
| 6 | Has a user guide page? | Yes | `docs/guide/fleet-config.md` |
| 7 | Wire format changed? | No | `pkg/fleet` envelopes are unchanged |
| 8 | Plugin SDK/protocol changed? | No | Nothing under `pkg/plugin` changes |
| 9 | RFC behavior implemented, changed, or newly proven? | No | No RFC governs managed configuration |
| 10 | Test infrastructure changed? | No | The tests use the existing managed fixtures |
| 11 | Affects daemon comparison? | No | Other daemons have no hub-managed configuration to compare |
| 12 | Internal architecture changed? | Yes | `docs/architecture/fleet-config.md` |
| 13 | Route metadata keys added/changed? | No | No route metadata |
| 14 | Prometheus counters added/changed? | No | None added |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | The command inventory: `docs/guide/command-reference.md` and the `wire-methods.snapshot` |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Derived at implementation with `./le spec citation anchors spec plan/release/fixes/spec-fleet-6-config-freeze.md`. Known now: `docs/architecture/fleet-config.md` anchors `client.go`, `ze_core_start.go`, `managed.go` and `session_factory.go`; `docs/architecture/doctor-and-health-checks.md` anchors the managed doctor; `docs/architecture/zefs-format.md` anchors `pkg/zefs/keys.go`. Found at closure by the same command, declared by `// Design:` headers of changed files: `docs/architecture/hub-architecture.md` (`cmd/ze/hub/main.go`, `main_reload.go`, `session_factory.go`, `storage/source.go`): its shutdown paragraph ("The hub also cancels and joins the complete outbound managed client") stays true and now names the owner (`managedClientOwner.shutdown`, anchor added at closure); its `update data config` paragraph stays true, the session editor's refusal of an undeclared client is in `docs/architecture/fleet-config.md`. `docs/architecture/system-architecture.md` (`cmd/ze/ze_core_start.go`, `internal/plugins/init/main.go`): unaffected, it names no managed start, no baseline and no fleet command (`grep -n -i 'managed\|fleet\|baseline' docs/architecture/system-architecture.md` answers nothing) |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/fleet-config.md` "Troubleshooting" and "What the Client Keeps in Its Store" gain the new commands and the new key |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the key, the code, the status command
   - Tests: `fleet-status.ci` (failing on the divergence fields), `TestFleetStatusStoreOnly`
   - Files: `pkg/zefs/keys.go`, `codes_plugin_infra.go`, `ze-managed-cmd.yang`, `cmd_fleet.go` (status only), `register.go`, the snapshot
   - Verify: `show fleet status` answers locally and through the daemon; `./le cli grammar` passes
2. **Phase: Baseline and hold** -- closes the release defect
   - Tests: the `client_hold_test.go` set, the baseline tests, `TestActiveHashOneHelper`, `TestDivergedDoctorCheck`, `TestHubRecordsHoldRefusal`, `fleet-hold-keeps-local-edit.ci`, `fleet-hold-clears-on-match.ci`, `fleet-diverged-doctor.ci`
   - Files: `baseline.go`, `client.go`, `doctor.go`, `cmd/ze/hub/managed.go`, `ze_core_start.go`, `internal/plugins/init/main.go`
   - Verify: AC-1 to AC-9, AC-18, AC-21. `docs/architecture/fleet-config.md` and the guide gain the baseline and the diverged device in this phase
3. **Phase: Leave and rejoin** -- the owner and the two commands
   - Tests: the owner tests, the `cmd_fleet_test.go` set, `fleet-disable-severs.ci`, `fleet-enable-rejoins.ci`
   - Files: `managed_owner.go`, `cmd/ze/hub/main.go`, `ze_core_start.go`, `cmd_fleet.go`
   - Verify: AC-10 to AC-15. The "Toggling it" sentence of the design page is corrected and the guide gains "leaving the fleet"
4. **Phase: Freeze** -- the admission and the machine entry
   - Tests: the `freeze_test.go` set, `TestMachineWritersOnManagedStore`, `fleet-freeze-refuses-commit.ci`, and the whole of `test/managed/`
   - Files: `freeze.go`, `pointer.go`, `source.go`, `cmd/ze/hub/managed.go`, `main.go`, `main_reload.go`, `confirm_window.go`
   - Verify: AC-16, AC-17, AC-19, AC-22. The guide gains the freeze
5. **Phase: Hub guard**
   - Tests: `TestSessionEditorRefusesUndeclaredClient`, `TestSessionEditorOpensDeclaredClient`, `fleet-hub-editor-declared.ci`
   - Files: `session_factory.go`
   - Verify: AC-20

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | The hold returns no error and never validates or stores the hub's configuration. The baseline is read back from the store, never taken from the pushed bytes. Disable stores the flag before it stops the client |
| Naming | The state is `diverged`, never `conflict`. The key is `meta/managed/base-version`. The RPCs carry the `ze-managed:` prefix |
| Data flow | The freeze refuses before a version is written. No operator route is edited. The reload stays one path |
| Machine writers | Each of the hub push, the confirmed-commit revert (running and at boot), the signal reload, the boot record and schema recovery calls the machine entry, and nothing else does |
| Operator errors and doctor codes | The freeze, enable and session refusals are operator errors with the fix on the last line. The hold's warning names `doctor-managed-diverged`, and the check carries its fix at the post-config phase |
| Rule: `ai/doors/system.md` | No `pkg/fleet` type changes. The new `pkg/zefs` key is the only public addition |
| Rule: `ai/rules/goroutine-lifecycle.md` | The owner cancels a context of its own, joins the client, refuses a second run and is joined before the store closes |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| The baseline key is registered | `grep -n "meta/managed/base-version" pkg/zefs/keys.go` |
| The doctor code exists and a check declares it | `./le arch doctor-coverage check` |
| The three commands are registered and grammatical | `./le cli list` shows them; `./le cli grammar` passes |
| The managed suite passes with the new tests | `./le --name <session> test managed -a` |
| No operator message is untyped | `./le arch operator-error check`, `./le arch error-inventory check` |
| The pages are current | `docs/architecture/fleet-config.md` and `docs/guide/fleet-config.md` carry the baseline, the hold, the commands and the freeze, each with a source anchor |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Authorization | `request fleet disable` and `enable` are in the edit section by their verb, so a read-only profile is refused; both record an audit action. `show fleet status` is a read |
| Hold bypass | No message from the hub can clear a hold except a `current` answer to the device's own hash. A hub cannot make a device apply a configuration it holds against |
| Baseline integrity | The baseline is written only where a hub configuration became active or the hub answered `current`. No command writes it. `update data key` can, with the edit section, as it can write the flag today |
| Freeze fail direction | A flag that cannot be read refuses the write |
| Refusal content | The hold text sent to the hub carries two hashes and no configuration content or secret |
| Client configuration access on a hub | The declared-client refusal of AC-20 is not an authorization check and must not be read as one (`undeclaredClientSessionRefusal` reads no profile and no user). Authorization of the SSH session editor's config-mode verbs is `spec-ssh-editor-authorization`'s, landed in `cc07d52edb`: `cmd/ze/hub/session_factory.go` `buildSessionModelFactory` installs `sessionConfigAuthorizer` on every editor model, judging each config permission in the edit section and failing closed with no authorizer |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| A machine writer is refused by the freeze | A-3 is broken: add the writer to the machine entry's callers, record it in the Mistake Log, re-run AC-17 |
| An unedited device holds after a restart | A-1 or A-2 is broken: back to DESIGN before the freeze phase |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The hold is the guarantee and the freeze is a courtesy. A design that makes the freeze carry the safety has to close thirty routes; one that makes the hold carry it has to compare two hashes.
- `current` is the one hub answer that proves agreement. Using it to reset the baseline makes a lagging baseline self-healing and gives fleet-7's adopt its mechanism for free.
- Deriving `diverged` from the flag, the baseline and the active hash means no state is stored that could disagree with them.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Detect a local change by comparing the active hash with a stored baseline | A "dirty" mark set by each operator write | The mark is only as complete as the coverage of thirty write routes. The comparison is independent of how the change was made |
| Read the baseline back from the store after the configuration is active | Hash the bytes the hub sent | If any write normalises the text, the pushed hash never equals the active hash and every device holds |
| Report the hold through the existing failed acknowledgement | A new acknowledgement field, or a new verb | A change to a published envelope in a release fix, for a report fleet-7 redoes with its up-verb |
| Derive `diverged` from the store | A stored `diverged` flag | A second record of the same fact drifts from the first |
| Freeze at the store's staging step, with a machine entry | A guard at each operator entry; a guard in the reload | At the entries a new route is unfrozen until someone remembers it. In the reload the refusal comes after staging and leaves a candidate that breaks the next push |
| The hub may always write a declared client's configuration (D-2) | Connected-only, as the skeleton had it | It forbade provisioning a client and the documented delivery of a change to a client that is away |
| A device with no baseline is clean (D-6) | Treat it as edited | Every device provisioned before this spec would hold on its first hub change, with no resolution command yet |
| Rejoin is live only after a live leave (D-7) | Build a client on the unmanaged start path too | It would run a combination no boot produces: the unmanaged services with a hub client attached |

## Known Limitations
- Resolving a diverged device from the hub is `plan/post-release/features/spec-fleet-7-config-reconnect-resolution.md`. Until then: to keep the device's change, import the device's configuration on the hub as that client's configuration, and the hold ends at the next fetch. To discard it, run `request fleet disable`, roll the device back to the version it received from the hub, and run `request fleet enable`.
- The hold is shown on the hub as a refused configuration. Its own cause and code on the hub belong to fleet-7.
- Raw key writes, a full-store restore and a rename with the daemon stopped are not frozen. The hold covers them (D-5).
- The SSH session editor now consults the authorization profile: `spec-ssh-editor-authorization` landed it in `cc07d52edb` (`cmd/ze/hub/session_factory.go` `sessionConfigAuthorizer`, installed by `buildSessionModelFactory` on every editor model), so a user whose profile denies the edit section can no longer commit a hub's client configuration through it. Read at source at closure; that spec's own tests are its evidence.
- During daemon start, between the first publication of the data RPC target and its re-publication once the managed server runs, a session or an import naming a declared client's configuration is refused as "this daemon serves no managed client". The import has had the same window since before this spec; the hub guard shares it (`cmd/ze/hub/main.go`, `InstallDataRPC`). Found in review round 1, 2026-11-19. Recorded as an open row in `plan/journal/session-up-before-its-subscriber.md` (2026-11-19), owned by `plan/release/fixes/spec-daemon-runtime-one-off-defects.md` (Work Not Done).
- A bare configuration name given to `ze config edit`, `ze config rollback` or `ze config history` outside the store folder fails with "no configuration store" even when `ze.config.dir` is set: `internal/core/resolve` `StoreDir` lets any supplied path, a bare name included, outrank the environment. A bare name is also how a loose file in the working directory is named, and 35 callers share the rule, so the fix needs a resolution design of its own and is too big for this spec. The guide tells the operator to type these commands in the store folder. Found while fixing the hub editor test, 2026-11-19. Recorded as an open row in `plan/journal/supplied-path-outranks-the-configured-store.md` (2026-11-19), owned by `plan/release/fixes/spec-daemon-runtime-one-off-defects.md` (Work Not Done).

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-22 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches. No agent runs any `./le verify` command: GitHub CI verifies the pushed commit, and the full verification is Thomas's release check. A red CI reports gets a fix commit
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied: lint fixed rather than disabled, the focused check for the changed behavior run once with its OUTPUT PASTED, and any red named with the one-line reason it is scaffolding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script (commit A preserves the spec in history)

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

## Owner Decisions (2026-11-18)

Put to the owner at the scope gate of the redesign, after the disposition
above. Each replaces what the skeleton above says where the two differ.

| # | Question | Decision | What it replaces |
|---|----------|----------|------------------|
| D-1 | Does one spec keep the freeze, the leave and rejoin commands, the baseline and the hold? | Yes, one spec. The baseline and the reconnect hold land first, because they alone close the release defect: a managed device loses a local edit today with no freeze involved | The skeleton's phase order (freeze first, hold third) |
| D-2 | AC-9: the hub refuses to write the config of a client that is not connected | Dropped. The hub may always write the config of a client it declares. Safety comes from the device: a device that is only offline is still frozen and cannot have edited, and a device that left the fleet and edited holds on its return whatever the hub wrote. The hub-side work is one shared "declared client" guard over every writer of a client config, the session editor included | AC-9, the "hub-side connected-only freeze" row of the inherited decisions, the fifth wiring row, phase 5, and the umbrella's "hub frozen for that device" wording. A hub and a departed device can both change a config; that two-sided divergence is resolved by fleet-7's diff |
| D-3 | A managed device whose applied config names no hub `client` block: managed flag true, client stopped for good | It stays frozen. The managed flag is the single truth for the freeze. The leave command is the exit and works with no hub and no connection, the freeze refusal names it, and the existing `doctor-hub-client-missing` report says why the device is alone. Rejoin refuses while the config names no hub, naming the fix | Nothing in the skeleton, which did not know the state |
| D-4 | Does fleet-7 wait for fleet-1 for its `diverged` record? | No. fleet-7 keeps its own per-client record of the config a device pushed up, stored beside that client's config; its presence is the diverged flag. fleet-1 later reads it. fleet-7 depends on fleet-6 only | The umbrella's "fleet-1 gains a diverged device state ... consumed by fleet-7" |
| D-5 | Is the freeze the guarantee, or the hold? | The hold. About thirty routes write a device's config and no single point sits in front of them, so the freeze is best effort: it tells the operator early. The baseline and the hold are what keep a local edit, and they are tested against a write the freeze missed | The skeleton's reliance on two editor functions (A-1) and its "freeze bypass" security row |
| D-6 | A managed device with no stored baseline | It is clean: it applies the hub config as today and records the baseline from then on. `ze init --managed --force` returns a device to that state | Nothing in the skeleton |
| D-7 | What rejoin does on a running daemon | A daemon that started managed owns its hub client: leave stops it at once and rejoin starts it again, with no restart. A daemon that started unmanaged sets the flag and says the hub connection starts at the next `ze start` | R-2 of the skeleton, which left it open |
| D-8 | Command spelling | `request fleet disable`, `request fleet enable`, `show fleet status`. `request` because the command ends or starts a session as well as storing a value; `disable` and `enable` as `request peer <selector> disable` and `enable` already use them | `ze fleet disable`, `ze fleet enable`, `ze fleet status` |

## Implementation Summary

### What Was Implemented
- Baseline and hold: `internal/component/managed/baseline.go` (`ActiveVersion`, `BaselineVersion`, `StoreBaseline`, `RecordActiveAsBaseline`, `Diverged`, `BaselineAccess`); `client.go` `fetchAgainstBaseline`, `settleBaseline`, `holdFetched`, `refuseUncompared`; baseline written by `cmd/ze/hub/managed.go` `recordPushBaseline` and `cmd/ze/ze_core_start.go` `cacheFirstConfig`; key `pkg/zefs/keys.go` `KeyManagedBaseVersion`; doctor check `managed/doctor.go` `managedDivergedDoctorCheck` under `doctor-managed-diverged`.
- Leave and rejoin: `cmd/ze/hub/managed_owner.go` `managedClientOwner` (`startWith`, `stopWith`, `join`, `shutdown`, `state`), wired in `runYANGConfig`; `managed/cmd_fleet.go` `request fleet disable`, `request fleet enable` (builtin RPCs `ze-managed:fleet-disable`, `ze-managed:fleet-enable` plus local routes) and `show fleet status` (local-data); YANG `ze-managed-cmd.yang`.
- Freeze: `internal/component/config/storage/freeze.go` (`refuseFrozenConfigLocked`, `configFrozen`, `OwnConfigFrozen`, `FreezeRefusal`, `ErrConfigFrozen`), asked by `pointer.go` `WriteCandidateVersionWithGuard`; machine entry `StageMachineCandidate` for `applyManagedPush`, `confirmRevert`, `publishRecoveredConfig`, `initializeConfigSource`; `StageSourceCandidate` for the signal reload. The refusal is kept whole by `daemonConfigImportRefusal`, `configRollbackRefusal`, `storedRollbackRefusal` and `configMutationOwner.Upload`.
- Hub guard: `cmd/ze/hub/session_factory.go` `undeclaredClientSessionRefusal` over `storagecli.UnservedClientConfigRefusal`.

### Bugs Found/Fixed
- A hold ended by `current` sent no acknowledgement, so the hub kept listing the device: fixed in `fetchAgainstBaseline` (`TestHoldEndedOnCurrentTellsTheHub`, `fleet-hold-clears-on-match.ci`).
- Review round 1: disable and enable raced (owner `op` lock, `TestOwnerFlagAndClientAgree`); status blocked behind a disable (`stopping` slot, `TestOwnerStateAnswersWhileStopWaits`); `frozen` true on a file-bound daemon (`OwnConfigFrozen`, `TestFleetStatusFrozenFollowsTheHandle`); wrappers replaced the freeze's fix (`FreezeRefusal`, `TestDaemonImportKeepsTheStoresFix` and siblings); the builder read the mirror (`TestManagedClientBuiltFromThePointedVersion`); flag read before name (`TestFreezeReadsTheNameFirst`); doctor fix named no client (`TestDivergedDoctorFixNamesTheClient`); enable's builder failure answered "next ze start" (`TestFleetEnableAnswersWhatFailed`).
- Review round 2: the SSH upload rewrapped the freeze refusal (`39b72f9ccf`, `TestUploadOnManagedDeviceAnswersTheFreeze`).

### Documentation Updates
- `docs/architecture/fleet-config.md` ("The baseline and the hold", "The freeze", client-side storage, the hub-side editor paragraph), `docs/guide/fleet-config.md` ("Leave and Rejoin the Fleet", "When a Client Refuses Your Change", "When a Client Holds", troubleshooting, store keys), `docs/architecture/doctor-and-health-checks.md`, `docs/architecture/storage-backends.md`, `docs/architecture/zefs-format.md`, `docs/architecture/api/commands.md`, `docs/guide/command-reference.md`, `ai/digests/hub-engine.md`, `features/fleet-management.md`: committed in `c9fbf8964c`..`7a7ae9c827`, each claim anchored.
- At closure: `docs/architecture/hub-architecture.md` shutdown paragraph names the owner (anchor `cmd/ze/hub/managed_owner.go -- managedClientOwner, shutdown`).
- `./le doc check all` at closure: Pre-Commit Verification, Documentation Verified.

### Deviations from Plan
- `internal/plugins/init/main.go` is unchanged: `--managed --force` publishes a fresh tree (`storage.ReplacePopulated`), so the baseline goes with it; `TestInitForceRemovesBaseline` pins AC-21.
- `show fleet status` is local-data and registers no RPC, so `wire-methods.snapshot` gained two rows, not three.
- The active configuration is the version the daemon runs (`storage.ReadActiveConfig`), not the plain configuration key (main-thread design change after phase 2A).
- The hub-push wiring row names `fleet-hold-keeps-local-edit.ci`; `config-push-transactional.ci` was not extended.
- The functional test plan said `fleet-freeze-refuses-commit` drives `ze config edit` stopped; it drives `ze config import` and `ze data import` stopped, because a stopped `ze config rollback <N> <file>` edits the loose file and never stages the store. A-7 is settled by reading.
- The SSH config upload (`cmd/ze/hub/config_upload.go`) gained the freeze pass-through, on a route the plan said would not be edited: it rewrapped the refusal under another fix.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The stopped-daemon step of the freeze test used `ze config rollback 1 <file>` | With no store open it edits the loose file (`cmdRollbackThrough` -> `cli.NewLooseFileEditor`) and exits 0 | Draft run red | Step replaced by `ze config import --dir`, which stages the store |
| approach | The hub editor test typed `ze config edit client-edge-99.conf` outside the store folder | `resolve.StoreDir` resolves a bare name to the working directory | Draft run red, "no configuration store" | Test runs in the hub's store folder; defect recorded in `plan/journal/supplied-path-outranks-the-configured-store.md` |
| approach | Pages said the hub keeps the `holding:` row after a hold ends | `handleRequest` clears a refusal only on an `ok:true` ack, which the end of a hold did not send | Phase 2B reading | Product fixed (`fetchAgainstBaseline` acks), pages corrected |
| approach | The Known Limitations line said the SSH editor ignores the authorization profile | `spec-ssh-editor-authorization` landed `sessionConfigAuthorizer` in `cc07d52edb` while this spec was open | Closure re-read of `session_factory.go` | Line and security row rewritten |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| 1. Baseline and hold | Done | `internal/component/managed/client.go` `fetchAgainstBaseline`; `baseline.go` | |
| 2. Leave and rejoin, status | Done | `cmd/ze/hub/managed_owner.go`; `internal/component/managed/cmd_fleet.go` | |
| 3. Freeze | Done | `internal/component/config/storage/freeze.go`; `pointer.go` `WriteCandidateVersionWithGuard` | |
| 4. Hub guard on the session editor | Done | `cmd/ze/hub/session_factory.go` `undeclaredClientSessionRefusal` | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestBaselineWrittenAfterPush`, `TestBaselineUnchangedWhenPushFails`, `fleet-hold-keeps-local-edit.ci` | |
| AC-2 | Done | `TestBaselineWrittenOnFirstStart` | |
| AC-3 | Done | `TestBaselineStableAcrossRestart`, `fleet-hold-keeps-local-edit.ci` (restart step) | |
| AC-4 | Done | `TestFetchHoldsWhenActiveDiffersFromBaseline`, `TestHoldAcknowledgesRefusalAndKeepsConnection`, `fleet-hold-keeps-local-edit.ci` | Heartbeats not asserted: nothing observes them without waiting on time |
| AC-5 | Done | `fleet-hold-keeps-local-edit.ci` `requireHoldsAcrossHubChange` | |
| AC-6 | Done | `TestCurrentAnswerSetsBaseline`, `fleet-hold-clears-on-match.ci` | |
| AC-7 | Done | `TestNoBaselineAppliesAsToday`, `fleet-hold-keeps-local-edit.ci` first step | |
| AC-8 | Done | `TestDivergedDoctorCheck`, `TestDivergedDoctorSilentWhenUnmanaged`, `fleet-diverged-doctor.ci` | |
| AC-9 | Done | `TestHubRecordsHoldRefusal`, `fleet-hold-keeps-local-edit.ci`, `fleet-hold-clears-on-match.ci` (cleared) | |
| AC-10 | Done | `TestFleetDisableStoresFlagThenStops`, `TestOwnerStopEndsLiveConnection`, `fleet-disable-severs.ci` | |
| AC-11 | Done | `TestFleetEnableStartsClient`, `TestOwnerStartAfterStop`, `TestOwnerStartsWhenBootHadNoHubBlock`, `fleet-enable-rejoins.ci` | |
| AC-12 | Done | `TestFleetEnableOnUnmanagedStart`, `fleet-status.ci` (`not-started`) | |
| AC-13 | Done | `TestFleetEnableRefusesWithoutHubBlock`, `TestFleetDisableNeedsNoHub`, `fleet-enable-rejoins.ci` | |
| AC-14 | Done | `TestFleetLocalRouteStoresFlagWithoutDaemon`, `TestFleetLocalRouteSendsBusyStoreToDaemon`, `fleet-enable-rejoins.ci` | |
| AC-15 | Done | `TestFleetStatusStoreOnly`, `TestFleetStatusDaemonRouteReportsClientState`, `TestClientReportsHoldToItsOwner`, `fleet-status.ci` | |
| AC-16 | Done | `TestFreezeRefusesOperatorStaging`, `TestUploadOnManagedDeviceAnswersTheFreeze`, `fleet-freeze-refuses-commit.ci` | Session, web and API commits reach the same staging step; driven end to end through the import, the rollback RPC and the stopped import |
| AC-17 | Done | `TestFreezeAdmitsMachineEntry`, `TestMachineWritersOnManagedStore`, `fleet-freeze-refuses-commit.ci` (the push applies) | |
| AC-18 | Done | `fleet-hold-keeps-local-edit.ci` (raw pointer write with the flag true throughout) | |
| AC-19 | Done | `TestManagedHubImportsDeclaredClientConfig`, `TestFreezeAdmitsOtherConfig` | |
| AC-20 | Done | `TestSessionEditorRefusesUndeclaredClient`, `TestSessionEditorOpensDeclaredClient`, `fleet-hub-editor-declared.ci` | |
| AC-21 | Done | `TestInitForceRemovesBaseline` | |
| AC-22 | Done | `TestFreezeScopeStoppedTool`, `TestFleetStatusStoreOnly`, `TestDivergedDoctorSilentWhenUnmanaged` | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| The five `client_hold_test.go` tests | Done | `internal/component/managed/client_hold_test.go` | plus `TestHoldEndedOnCurrentTellsTheHub`, `TestFirstBaselineSendsNoAck` and two store-failure tests |
| `TestBaselineWrittenAfterPush`, `TestBaselineUnchangedWhenPushFails` | Done | `cmd/ze/hub/managed_test.go` | |
| `TestBaselineWrittenOnFirstStart`, `TestBaselineStableAcrossRestart` | Done | `cmd/ze/ze_core_start_bootstrap_test.go` | |
| `TestActiveHashOneHelper` | Done | `internal/component/managed/baseline_test.go` | plus `TestActiveHashReadsThePointedVersion` |
| `TestDivergedDoctorCheck`, `TestDivergedDoctorSilentWhenUnmanaged` | Done | `internal/component/managed/doctor_test.go` | |
| `TestHubRecordsHoldRefusal` | Done | `internal/component/plugin/server/managed_serve_test.go` | |
| The five `TestOwner...` tests | Done | `cmd/ze/hub/managed_owner_test.go` | plus `TestOwnerFlagAndClientAgree`, `TestOwnerStateAnswersWhileStopWaits` |
| The four `cmd_fleet_test.go` tests | Done | `internal/component/managed/cmd_fleet_test.go`, `cmd_fleet_change_test.go` | |
| The six `freeze_test.go` tests | Done | `internal/component/config/storage/freeze_test.go` | |
| `TestManagedHubImportsDeclaredClientConfig` | Done | `cmd/ze/hub/managed_hub_freeze_test.go` | |
| `TestMachineWritersOnManagedStore` | Done | `cmd/ze/hub/managed_machine_writers_test.go` | |
| `TestSessionEditorRefusesUndeclaredClient`, `TestSessionEditorOpensDeclaredClient` | Done | `cmd/ze/hub/session_factory_test.go` | |
| `TestInitForceRemovesBaseline` | Done | `internal/plugins/init/managed_hub_test.go` | |
| The eight functional tests | Done | `test/managed/fleet-*.ci` | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| Every file of Files to Modify and Files to Create | Done | except `internal/plugins/init/main.go`: Changed, not needed (Deviations) |
| `cmd/ze/hub/config_upload.go`, `config_source.go`, `internal/component/config/storage/config_name.go`, `publication_intent.go`, `internal/core/resolve/resolve.go`, `storage/cli/data_rpc_rollback.go`, `config/cli/cmd_rollback.go` | Changed | Added: the machine entries, the shared name rule, the shared key read and the refusal pass-throughs |

### Audit Summary
- **Total items:** 4 requirements, 22 AC, 33 planned tests, the planned files
- **Done:** all
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 2 (`internal/plugins/init/main.go` unchanged; files added), recorded in Deviations

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A managed device does not lose a local change on reconnect (baseline and hold) | functional, whole path | `test/managed/fleet-hold-keeps-local-edit.ci`: a raw active-pointer write with the flag true, then the device connects to a hub holding another configuration and keeps its own, and the hub's `show doctor` lists the hold; red with the `Diverged` branch of `fetchAgainstBaseline` removed (round 1 handoff). 80 stress iterations, no failure |
| The hold ends when hub and device agree | functional | `test/managed/fleet-hold-clears-on-match.ci`: importing the device's configuration on the hub ends the hold and clears the hub's row |
| An operator can leave and rejoin the fleet at runtime | functional | `test/managed/fleet-disable-severs.ci` (the hub sees the session end, a commit is then accepted), `test/managed/fleet-enable-rejoins.ci` (the same process reconnects and follows) |
| The freeze refuses a local change and names the fix | negative | `test/managed/fleet-freeze-refuses-commit.ci`: the running import and rollback RPC and the stopped import refused with `fix: request fleet disable` last, a hub push still applies; `TestUploadOnManagedDeviceAnswersTheFreeze` red/green proven |
| The hub's session editor refuses an undeclared client | negative | `test/managed/fleet-hub-editor-declared.ci`, `TestSessionEditorRefusesUndeclaredClient`; a declared client's configuration opens |
| Status is reported on every surface | functional | `test/managed/fleet-status.ci` (daemon running and stopped, managed and not); `test/ui/pipe-local-command.ci` 72/72 |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| Resolution of a diverged device from the hub (diff, adopt, revert, the hold's own hub-side cause) | Outside this spec by D-4; built on the baseline and hold delivered here | `plan/post-release/features/spec-fleet-7-config-reconnect-resolution.md` |
| The declared-client guard's startup window (open row `plan/journal/session-up-before-its-subscriber.md`, 2026-11-19) | Pre-existing on the import; fixing it reorders the hub's start publication, a design of its own | `plan/release/fixes/spec-daemon-runtime-one-off-defects.md` |
| `resolve.StoreDir` resolving a bare name to the working directory (open row `plan/journal/supplied-path-outranks-the-configured-store.md`, 2026-11-19) | Pre-existing; 35 callers share the rule, so the fix needs a resolution design | `plan/release/fixes/spec-daemon-runtime-one-off-defects.md` |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fleet-6-config-freeze-1ecdeafe-7c8e-4b32-a486-fac3ee6445a8.md` (16 files: this spec and every file commit A carries) |
| `./le spec review check` | clean, 2026-11-19 at closure, over the same 16 files |
| Rounds | 4: rounds 1-3 during implementation, round 4 at closure, clean |
| Reviewer lenses used | Round 1 logic, wiring and security over the whole diff; round 2 the fixes and the operator routes; round 3 the SSH upload fix; round 4 (closure, a context that wrote none of it) wiring, guard audit, goroutine lifecycle, style pass |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | Disable and enable raced: flag true with the client stopped | `cmd/ze/hub/managed_owner.go` | `op` lock across write and act (`startWith`, `stopWith`) |
| 2 | ISSUE | `show fleet status` blocked behind a disable | `managed_owner.go` `join` | Join outside `mu`, `stopping` slot |
| 3 | ISSUE | `frozen` reported true on a file-bound daemon | `cmd_fleet.go` `readFleetStatus` | `storage.OwnConfigFrozen` |
| 4 | ISSUE | Three wrappers replaced the freeze's fix line | `data_rpc_config.go`, `data_rpc_rollback.go`, `cmd_rollback.go` | `storage.FreezeRefusal` pass-through |
| 5 | ISSUE | The builder read the mirror, enable read the pointed version | `ze_core_start.go` `extractManagedClientConfig` | `storage.ReadActiveConfig` |
| 6 | ISSUE | A bad flag on a stopped store refused every client import | `freeze.go` | Name read first (`unboundConfigFrozen`) |
| 7 | ISSUE | The SSH upload rewrapped the freeze refusal | `cmd/ze/hub/config_upload.go` `Upload` | `39b72f9ccf` |

### Round 4 (closure)
0 BLOCKER, 0 ISSUE. Style pass run over every changed Go file: the two `panic("BUG: ...")` added (`configFrozen` on an unknown source mode, `RunWithManagedClient` with a nil builder) guard states only a Ze defect reaches, with no socket input on the path; the owner's goroutine has one owner that cancels and joins it, and its doc comments state both sides. NOTEs, fixed in one edit: the Known Limitations SSH-editor line and the security row were stale; A-5 and A-7 settled by reading.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/managed/baseline.go`, `cmd_fleet.go`, `yang/ze-managed-cmd.yang`, `internal/component/config/storage/freeze.go`, `cmd/ze/hub/managed_owner.go` | Yes | `ls` at closure, 2026-11-19 |
| The eight `test/managed/fleet-*.ci` | Yes | `ls test/managed/fleet-*.ci` lists all eight |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1..AC-22 | Each demonstrated by the tests of the audit table | Main-thread runs 2026-11-19: managed suite 25/25; each new `.ci` 80 stress iterations; packages managed, config/storage, storage/cli, cmd/ze and the hub fleet tests green; the closure re-run is an owed command |
| AC-16 | The staging step asks the freeze | `pointer.go` `WriteCandidateVersionWithGuard` calls `refuseFrozenConfigLocked` before `stageCandidateLocked` (read at closure) |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| Each Wiring Test row | The `.ci` the row names | Yes: each file exists under `test/managed/` and passed in the 25/25 managed run of 2026-11-19 |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `TestBaselineStableAcrossRestart`, `fleet-hold-keeps-local-edit.ci` |
| A-2 | confirmed | `TestActiveHashOneHelper`, `TestActiveHashReadsThePointedVersion` |
| A-3 | confirmed | `TestMachineWritersOnManagedStore`; the caller census of the freeze phase |
| A-4 | confirmed | `TestFreezeScopeStoppedTool`, `TestFreezeReadsTheNameFirst` |
| A-5 | confirmed | `runReloadContext` returns `acceptReload(true)` after `promoteReloadCandidate`; `TestCurrentAnswerSetsBaseline` |
| A-6 | confirmed | `TestOwnerStopEndsLiveConnection`, `fleet-disable-severs.ci` |
| A-7 | confirmed | By reading: `startEphemeralDaemon` -> managed `ze start` bound `ConfigSourceStored` -> ephemeral SSH -> `commitSessionCandidate` -> `WriteCandidateVersionWithGuard` -> `ErrConfigFrozen` |
| A-8 | confirmed | `TestHubRecordsHoldRefusal`; `handleRequest` compares nothing in `ack.Error` |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `./le doc check all` and `./le spec citation anchors` at closure | 2026-11-19 at closure: `./le spec citation` OK; `./le spec citation anchors` no longer reports an unnamed design document; `./le doc check all` red only on other sessions' surfaces (the wiki catalog drift on `clear bgp rib out-excluding-owner`, the `ze-bgp:rib-source-*` and `ze-cos:cleanup` handler contract, help-shape rules, stale anchors into `web/fleetui`, `l2tp` and `rib`), none naming a managed or fleet command, page or source | Yes |
| `docs/architecture/hub-architecture.md` shutdown paragraph | `runYANGConfig` defers `clientOwner.shutdown()`; `managedClientOwner.shutdown` sets `ended` and joins | Yes |
| `docs/architecture/system-architecture.md` unaffected | It names no managed start, baseline or fleet command | Yes |
