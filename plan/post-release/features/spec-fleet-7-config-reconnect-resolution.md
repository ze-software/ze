# Spec: fleet-7 -- Diverged Config Resolution

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | config |
| Priority | P3 |
| Release reason | Adds a capability absent from the supported release; existing supported behavior and its obligations remain required. |
| RFC gaps | - |
| Depends | spec-fleet-6-config-freeze (closed 2026-11-19) |
| Phase | - |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | development tier |
| Updated | 2026-11-19 |

Recovery after compaction: `.claude/rules/post-compaction.md`. Then read this
spec, `docs/architecture/fleet-config.md` ("The baseline and the hold",
"Protocol"), `ai/doors/system.md` and `ai/doors/hub-engine.md`. The
`## Owner Decisions (2026-11-19)

The owner approved the written design on 2026-11-19 and asked for implementation.
` section at the end holds E-1 to E-12, which
this design is built on. The `## Disposition` section holds why the skeleton
was reworked.

## Task

fleet-6 made a managed device hold: when the configuration it runs differs
from the last one its hub gave it (its baseline), it applies nothing the hub
sends, stays connected and reports `diverged`. Nothing ends a hold from the
hub today. The operator has two manual routes: import the device's
configuration on the hub by hand, or leave the fleet on the device, roll back
and rejoin. fleet-7 lets the operator end the hold from the hub after seeing
the difference (E-1).

Six pieces, in the order they land:

1. **The diff producer, and the `ze config diff` fix (E-3).** `ze config diff`
   and `show config diff` compare only the BGP tree today: two configurations
   that differ only in a `plugin hub client` host answer "No differences"
   (verified by a run on 2026-11-19). One exported whole-tree, secret-masked,
   structured diff producer replaces that comparison. `ze config diff` and the
   hub's divergence command both use it. It lands in its own commit with its
   own test.
2. **Upload and record.** A device that holds sends the configuration it holds
   to its hub with a new device-to-hub verb, `config-held` (E-8). The hub
   keeps one record per client in its store, under
   `meta/config/client-<name>.conf/held` (E-11). The hub's `config-fetch`
   answer names the record it holds (`held-version`, E-9), so a device uploads
   only when the hub does not hold its configuration yet. The device's
   acknowledgement of a hold says that it holds (`holding`, E-10), so the hub
   stops showing a hold as a refused configuration.
3. **Show.** `show fleet divergence list` and `show fleet divergence name
   <client>` on the hub: the diff, the change count, both hashes and the time
   the device configuration became active (E-6, E-7).
4. **Adopt.** `update fleet divergence name <client> adopt version <hash>`:
   the device configuration becomes the hub's configuration for that client.
   The device is already running it, so fleet-6's `current` answer ends the
   hold (E-6).
5. **Revert.** `request fleet divergence name <client> revert version <hash>`:
   the device drops its change and takes the hub's configuration. The hub
   offers this in its `config-fetch` answer, bound to the device hash and the
   hub hash the operator saw. No new hub-to-device verb (E-4).
6. **Doctor and status.** A hub doctor code for a diverged client and for an
   orphaned record (E-10, E-11), and two new fields of `show fleet status` on
   the device (E-12).

The web page that renders the divergence belongs to
`plan/post-release/features/spec-fleet-ui-6-config-rollout-audit-admin.md`.
fleet-7 owns the data, the protocol and the commands (E-1). The record is per
client in the hub's store and does not wait for a device registry (E-1,
fleet-6 D-4).

Every `pkg/fleet` change is deliberate and backward compatible: one new verb,
two fields of the fetch answer, one field of the acknowledgement. Each lands
with its page (E-2).

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/fleet-config.md` - the managed protocol, the store layout, "The baseline and the hold", "Protocol"
  → Constraint: "How a hold ends" says only a `current` answer ends a hold and "the hub cannot make a client apply a configuration while the client holds". The revert (E-4) departs from this sentence by owner decision; the revert phase corrects the paragraph in the same work
  → Constraint: the RPC table lists four verbs. `config-held`, `held-version`, the discard field and `holding` are added to it in the phase that lands each one (E-2)
  → Decision: adopt needs no new device mechanism: writing the client's configuration makes the hub answer `current` to the device's own hash, which is how a hold ends today
- [ ] `docs/guide/fleet-config.md` - the operator walkthrough
  → Constraint: "End the Hold" names the two manual routes of fleet-6. It gains the adopt and revert commands, and keeps the manual routes for a configuration over the upload bound
  → Constraint: "Change a Client's Configuration" promises a client that is not connected gets the change when it next connects. Revert keeps that promise: an offline device takes the revert at its next fetch
- [ ] `ai/doors/system.md` - what every change must respect
  → Constraint: `pkg/fleet` and `pkg/zefs` are published contracts. The verb, the three fields and the key change them on purpose, each with its page
  → Constraint: a command is declared once with a YANG registration and reaches every interface; a check is a doctor check registered by the package that owns the state
  → Constraint: runtime state lives in the store under a key of `pkg/zefs/keys.go`, never in memory alone, which is why the record survives a hub restart
- [ ] `ai/doors/hub-engine.md` - the managed client and server
  → Constraint: one reload path. The revert applies on the device through the existing push path (`applyFetched` -> `OnCommit` -> `applyManagedPush` -> `storage.StageMachineCandidate`), never a second one
  → Constraint: the managed server is built in `cmd/ze/hub/managed_server.go` `startManagedServer` over `pluginserver.NewManagedServer`; the commands reach it through an installed target, as the fleet commands reach the client owner
- [ ] `docs/architecture/cli/command-verbs.md` - verb meanings
  → Decision: adopt is `update`: data held for an existing client (its stored configuration) is rewritten. Revert is `request`: the change is on the device, through the wire, and is none of create, delete or update. The two shows are `show` (E-6)
- [ ] `ai/rules/cli.md` - verb first, keywords before values, no `--flag` reaches the daemon
  → Constraint: `name <client>` and `version <hash>` are keyword and value pairs. The grammar is checked with `./le cli grammar`
- [ ] `ai/rules/user-facing-errors.md` - every refusal names the problem, why, and the exact fix on the last line
  → Constraint: a stale hash refusal names the record's current hash and the show command as its fix
- [ ] `docs/architecture/doctor-and-health-checks.md` - phases
  → Constraint: a check over the store and the tree is post-config; the hub's divergence check reads the store record and the serving blocks, so it is post-config and reports with the hub stopped
- [ ] The closed fleet-6 spec (`spec-fleet-6-config-freeze`, deleted at closure; read it with `git show` at commit `65b33f19a2`, under `plan/release/fixes/`) - the closed fleet-6 design
  → Constraint: the hold never returns an error from the fetch path, or the device reconnects about once a second (fleet-6 Current Behavior). The upload keeps that: a failed upload is logged and the connection is kept
  → Constraint: fleet-6 Known Limitations names this spec as the owner of the hold's own cause on the hub

**Key insights:**
- The hub already holds half of the diff: the client's configuration. The device holds the other half, and must send exactly the bytes it hashed.
- Adopt is a write of `client-<name>.conf`; everything after it is fleet-6.
- Revert is the one place a hub makes a holding device apply something. Binding it to the device hash D and the hub hash V makes a stale revert inert on both sides.
- The upload runs on the device's notification loop. While it runs, the loop answers no `config-changed`, so the upload's deadline must be shorter than the hub's push round-trip bound.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/managed/client.go` - `runConnection` fetches once after authentication through `fetchAndProcess`, then runs `notificationLoop`, which handles each hub request in turn on one goroutine. `handleConfigChangedRequest` answers the notification OK first, then fetches. `fetchAgainstBaseline` reads the active hash and the baseline (`BaselineAccess.read`), fetches with the active hash, calls `settleBaseline` on `current` (and acks `ok:true` when it was diverged), calls `holdFetched` when `Diverged`, else `applyFetched`. `holdFetched` sends `config-ack` with `ok:false` and the hold text, logs under `doctor-managed-diverged` with `managedDivergedFix`, and returns nothing. `handleHubRequest` silently drops an unknown hub verb
  → Constraint: the device-side revert check lives in `fetchAgainstBaseline`, beside the hold (E-4)
  → Constraint: an upload placed in the hold branch blocks the loop for its whole duration; a second `config-changed` arriving then is answered only when it ends
- [ ] `internal/component/managed/baseline.go` - `ActiveVersion` hashes the bytes `activeConfig` reads (`storage.ReadActiveConfig` of the name `resolve.DefaultConfig` selects). `BaselineAccess` holds three callbacks, active, baseline and store. `Diverged` is false with no baseline or no stored configuration
  → Constraint: the device holds hashes only. The upload reads bytes and hashes them in one `activeConfig` read, so it uploads exactly what it hashed
- [ ] `pkg/fleet/envelope.go` - four verbs; `ConfigFetchRequest` (version), `ConfigFetchResponse` (version, config, status), `ConfigChanged` (version), `ConfigAck` (version, ok, error). Every field is JSON with `omitempty` except the request version and the ack's version and ok
  → Decision: new fields are `omitempty`, so an old peer decodes them as absent and Go's decoder ignores a field it does not know
- [ ] `internal/component/plugin/server/managed_serve.go` - `handleConn` authenticates per client and takes the name only from the authenticated session. `serve` handles one request at a time. `handleRequest` switches over `config-fetch`, `config-ack` and `ping`; any other verb is answered with an error "unknown method". The `config-ack` arm calls `doctor.clearRejected` on `ok:true` and `doctor.recordRejected(name, ManagedRejectConfigRejected, ack.Error)` otherwise. `pushConfigChanged` bounds each `config-changed` round-trip by `managedPushTimeout` (10 seconds) and closes the connection of a client that does not answer in time
  → Constraint: an old hub answers `config-held` with an error, so a new device learns at once that its hub cannot take an upload
  → Constraint: an upload that keeps the device's loop busy past 10 seconds while a push is in flight closes the connection; the device then reconnects, holds and uploads again
- [ ] `internal/component/plugin/server/managed.go` - `HandleConfigFetch` reads the client's configuration, hashes it, answers `current` when the request version equals it, else the version and the base64 configuration. `BuildConfigChanged` hashes the same read
  → Decision: the fetch answer gains `held-version` and the discard field here, from the record
- [ ] `internal/component/plugin/server/managed_doctor.go` - the in-memory `managedDoctorState` keeps the last refusal per client with a `ManagedRejectCause`; `ManagedDoctorSnapshot` publishes it to the runtime check
  → Constraint: it is memory. The record of a diverged client is not kept there: it must survive a hub restart
- [ ] `cmd/ze/hub/managed.go` - `wireManagedCommit` sets `OnCommit`; `applyManagedPush` stages through `storage.StageMachineCandidate`, reloads, then `recordPushBaseline` reads the active configuration back and stores its hash as the baseline
  → Decision: a revert applies through this unchanged path, so the frozen device takes it and the baseline becomes the hub's hash
- [ ] `cmd/ze/hub/managed_server.go` - `startManagedServer` builds the server with `ReadConfig` over `store.ReadFile(ClientConfigKey(name))` and installs `store.SetWriteObserver`, which calls `NotifyConfigChanged` for each written `client-<name>.conf`
  → Decision: adopt rides this chain: write `client-<name>.conf` -> write observer -> `NotifyConfigChanged` -> `pushConfigChanged` -> device fetches its active hash -> `HandleConfigFetch` answers `current` -> `settleBaseline` stores the baseline and acks `ok:true`
- [ ] `internal/component/config/storage/restore.go`, `rename_config.go`, `rollback_config.go`, `publication_intent.go`, `import_config.go` - `CommitConfigWithGuard` publishes a new active version under a guard the caller holds, and refuses with `ErrCandidateExists` while a candidate is staged. `prepareConfigRename` refuses while `file-commit` or `confirm-pending` exists for the old name. `admitConfigRollbackLocked` and `ImportConfig` refuse on retained intents through `refuseConfigIntentLocked`
  → Decision: adopt is `CommitConfigWithGuard` plus the record's removal under the same guard. The rename refusal list gains the record key; the rollback and import admissions do not (E-11)
- [ ] `internal/component/config/storage/store.go` - `RuntimeKey` refuses every `meta/config/` key, so a running daemon's `update data key` and `delete data key` cannot write or remove the record
  → Constraint: the record is written and removed only by the managed server and the divergence commands, under the writer lock (E-11). With the hub stopped, `ze data rm` removes it
- [ ] `internal/component/config/confirm/store.go` - `StoreRecorder` keeps a JSON record under `meta/config/<name>/confirm-pending`, beside the version pointers
  → Decision: the held record copies this shape: one JSON value per configuration name, decoded strictly, an undecodable record is an error and never "no record"
- [ ] `internal/component/config/cli/cmd_diff.go` - `compareDiffInputs` selects the schema with `config.SchemaWithDeclaredPlugins`, parses both sides, and `diffSources` diffs `infra.ResolveBGPTree` of each side with `config.DiffMaps`, adds schema-declared list orders (`diffListOrders`) and masks secrets (`maskDiffSecrets`). A side with no `bgp` block contributes no values. `artifact_config.go` and `inspect_route.go` `diffAsking` are its two callers
  → Decision: the producer keeps the BGP comparison as it is (resolved values, existing paths) and adds every other top-level root as its `Tree.ToMap` lowering under its root name, so every existing `ze config diff` expectation keeps its output
- [ ] `internal/component/cli/diff_tree.go` - `computeTreeAnnotatedDiff` renders a line diff of two texts for the editor; unexported
  → Decision: not used. The hub answers a structured diff, and the change count is its number of entries (E-7)
- [ ] `internal/component/config/declared_schemas.go` - `SchemaWithDeclaredPlugins` starts no program; in a daemon the module set is fixed to the plugins it started. `DeclaredSchemas.Explain` turns a refused unknown node of a declared external plugin into an operator error naming the plugin
  → Constraint: a client configuration with a section of an external plugin the hub did not start does not parse on the hub. The upload and the show tell that case apart from a malformed configuration
- [ ] `internal/component/managed/cmd_fleet.go`, `register.go`, `yang/ze-managed-cmd.yang`, `doctor.go` - `FleetTarget` and `InstallFleetTarget` publish the client owner to the handlers; `show fleet status` is a local-data command the daemon serves under the same path; `request fleet disable` and `enable` are RPCs `ze-managed:fleet-disable` and `ze-managed:fleet-enable` with an audit action, and local routes through `daemonroute`. `managedDivergedFix` names the two manual routes
  → Decision: the four divergence commands live here, under the same YANG module, with a `DivergenceTarget` installed by the hub
- [ ] `internal/component/web/fleetui/source_config.go` - `ConfigSource.Divergence(ctx, device)` answers a `Divergence` value: Device, HubVersion, HubText, DeviceVersion, DeviceText, Changes ("counts the local edits"), EditedAt ("when the last local edit was made"). `handler_config.go` offers the actions `keep-device` and `restore-hub`
  → Constraint: the page is fleet-ui-6's. This spec's command answers the hashes, the change count and the edited-at time; it answers no configuration text (Security Review)
- [ ] `pkg/zefs/keys.go` - `KeyConfigConfirmPending` is `meta/config/{name}/confirm-pending`, Private; `KeyManagedBaseVersion` is `meta/managed/base-version`
  → Decision: `KeyConfigHeld` is `meta/config/{name}/held`, Private (E-11)
- [ ] `pkg/plugin/rpc/conn.go`, `framing.go` - `writeFrame` takes the write deadline from the context, else `defaultWriteDeadline` (30 seconds); `MaxMessageSize` is 16 MiB
  → Constraint: the upload's context deadline is also its write deadline. A TLS write that hits its deadline ends the connection
- [ ] `internal/test/fixture/misc_fixture_managed.go` - a fake hub with its own copies of the ack and fetch-answer types, driving `config-push-transactional`
  → Constraint: its device is never diverged, so it never receives `config-held`. It is preserved unchanged

**Behavior to preserve:**
- The four verbs and their existing fields, TLS and per-client secrets
- A device that is not diverged: fetch, apply, acknowledge, exactly as fleet-6
- A device that holds against a hub that does not know `config-held`: it holds as fleet-6 does, with the refused-configuration row on that hub
- A device built before this spec against a hub built after it: it holds as fleet-6 does and keeps today's refused-configuration row on the hub (E-10)
- fleet-6's two manual routes to end a hold
- Every existing `ze config diff` and `show config diff` expectation (`test/parse/cli-config-diff.ci`, `cli-config-diff-identical.ci`, `config-diff-list-order.ci`, `test/plugin/external-yang-inspect-*.ci`)
- The hub's writes to a declared client's configuration, connected or not; `TestManagedHubImportsDeclaredClientConfig`
- Every test under `test/managed/`

**Behavior to change:**
- `ze config diff` and `show config diff` compare every root of the configuration, not only BGP
- A holding device uploads its configuration once per hub record, under a 1 MiB bound and its own deadline
- The hub keeps a per-client record in its store, answers `held-version`, and stops recording a hold as a refused configuration when the device says it holds
- Four hub commands, an audit action each for adopt and revert, one hub doctor code
- A holding device applies the hub's configuration when the fetch answer's discard field names its own active hash
- `show fleet status` gains `held-by-hub` and `revert-offered`; `managedDivergedFix` names the hub commands
- A rename of a client configuration that has a record is refused

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| The managed listener and its dispatch | `internal/component/plugin/server/managed_serve.go` `handleRequest` | Answers three verbs per authenticated client, errors on any other | Extends: a `config-held` arm, scoped to the authenticated name |
| Fetch answer and the version hash | `internal/component/plugin/server/managed.go` `HandleConfigFetch`; `pkg/fleet` `VersionHash` | Answers `current` or the configuration | Extends: answers `held-version` and the discard field; clears the record on `current` |
| The `pkg/fleet` envelopes | `pkg/fleet/envelope.go` | Four verbs and four payload types | Extends: one verb, one payload type, three optional fields (E-2) |
| The hold and the `current` answer | `internal/component/managed/client.go` `fetchAgainstBaseline`, `holdFetched`, `settleBaseline` | Holds a diverged device; `current` ends the hold and acks `ok:true` | Extends: upload in the hold branch, the discard check beside it. Adopt reuses `settleBaseline` unchanged |
| Active configuration reader | `internal/component/managed/baseline.go` `activeConfig`, `BaselineAccess` | Reads the bytes the device runs; the client holds callbacks, never the store | Extends: a fourth callback answers bytes, hash and activation time from one read |
| Hub push path on the device | `cmd/ze/hub/managed.go` `applyManagedPush`, `recordPushBaseline` | Stages through the machine entry, reloads, records the baseline | Extends by use: the revert applies through it unchanged |
| Write observer and notification | `cmd/ze/hub/managed_server.go` `SetWriteObserver`; `ManagedServer.NotifyConfigChanged` | A written `client-<name>.conf` pushes `config-changed` | Extends by use: adopt writes the file; revert calls `NotifyConfigChanged` through the target, as it writes no client file |
| Versioned publication under a held guard | `internal/component/config/storage/restore.go` `CommitConfigWithGuard` | Publishes a new active version under the caller's guard | Extends by use: adopt publishes and removes the record under one guard |
| Per-configuration JSON record | `internal/component/config/confirm/store.go` `StoreRecorder` | One JSON record under `meta/config/<name>/` | Extends by copy: the held record has the same shape and the same decode strictness |
| Rename identity fence | `internal/component/config/storage/rename_config.go` `prepareConfigRename` | Refuses a rename while an unfinished intent exists | Extends: the record key joins the list (E-11) |
| Rollback and import intent refusal | `rollback_config.go` `admitConfigRollbackLocked`; `import_config.go` `ImportConfig` | Refuse while an unfinished publication is retained | Does not apply: a record is not an unfinished publication; a rollback or import of the client's configuration is a hub write that voids a revert by changing V (E-11) |
| Two-text diff | `internal/component/config/cli/cmd_diff.go` `compareDiffInputs`, `diffSources` | BGP-only structured diff, secret-masked | Replaced: one exported whole-tree producer; both callers and the hub command use it (E-3) |
| Editor line diff | `internal/component/cli/diff_tree.go` `computeTreeAnnotatedDiff` | Annotated text diff for the editor | Does not apply: unexported, text-shaped, no count; the structured diff is the one fact |
| Declared external schemas | `internal/component/config/declared_schemas.go` `SchemaWithDeclaredPlugins`, `DeclaredSchemas.Explain` | Explains a refused section of a plugin the process holds no schema of | Extends by use: tells an unchecked plugin section from a malformed upload |
| Hub refusal record | `internal/component/plugin/server/managed_doctor.go` `recordRejected`, `clearRejected` | In-memory last refusal per client | Extends: a holding ack with a stored record clears it instead of recording a refusal |
| Fleet command target | `internal/component/managed/cmd_fleet.go` `FleetTarget`, `InstallFleetTarget` | Lets a handler reach daemon state without importing `cmd/ze/hub` | Extends by copy: `DivergenceTarget` |
| Declared-client predicate | `internal/component/config/storage/cli/data_rpc.go` `DataRPCTarget.ServesClient`; `ManagedServer.Serves` | Hub writers refuse a client no serving block declares | Extends by use: adopt and revert refuse an undeclared client |
| Managed doctor checks | `internal/component/managed/doctor.go` `checkManagedDiverged`, `managedDivergedFix` | Device-side diverged check and fix text | Extends: a hub-side sibling over the records; the fix text names the hub commands |
| Device fleet status | `internal/component/managed/cmd_fleet.go` `readFleetStatus`, `ClientConfig.OnHold` | Store facts plus runtime client, connection and hold | Extends: two runtime fields from the last fetch answer |
| Audit actions | `internal/core/audit` `action_config_rename.go` and siblings | One file per action constant | Extends by copy: an action each for adopt and revert |
| Device registry | `plan/post-release/features/spec-fleet-1-device-registry.md` | Skeleton | Does not apply: the record is per client in the store and does not wait for it (E-1) |
| Editor conflict states | `internal/component/cli/editor_commit.go` `ConflictStale`, `ConflictLive` | Concurrent edit sessions | Does not apply. It is why the state is `diverged` and the verb is not `conflict` |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A holding device fetches: after authentication, or after a `config-changed`
- The hub receives `config-held` from an authenticated client
- An operator runs one of the four divergence commands on the hub, on any surface or locally
- `ze config diff` or `show config diff` with two configurations

### Transformation Path
1. **Hold decided.** `fetchAgainstBaseline` reads active A and baseline B, fetches with A. The hub answers configuration V with `held-version` H (absent when it holds no record, or when it is an old hub) and possibly a discard field.
2. **Revert offered.** When `Diverged(A, B)` and the discard field equals A, the device logs that the hub's operator reverted, reports not holding and applies through `applyFetched`. The baseline becomes the active hash after the reload (`recordPushBaseline`), and the ack is `ok:true`. A discard field that names another hash is ignored and reported as a stale offer.
3. **Upload decided.** Otherwise, when diverged, the device uploads unless H equals A, unless this connection already met a refused verb, and unless this client run already failed to upload this hash. It reads bytes, hash and activation time in one read. Over 1 MiB: no upload, a warning under `doctor-managed-diverged` naming the manual routes. Else it sends `config-held` (version, base64 config, edited-at) under a context with its own deadline, `heldUploadTimeout`, set below `managedPushTimeout`.
4. **Hub takes the upload.** The `config-held` arm decodes the payload, refuses a decoded size over 1 MiB, recomputes the hash and refuses a mismatch, parses the text with the hub's schema: a malformed configuration is refused; a section of an external plugin the hub did not start is accepted and noted in the record. Under the writer lock of `client-<name>.conf` it writes the record (bytes, device hash, edited-at, received-at, unchecked plugins), replacing any earlier one and dropping its revert decision. It answers OK. The name is the authenticated one; the payload carries none.
5. **Hold acknowledged.** The device sends `config-ack` with `ok:false`, `holding:true` and the hold text (which also says whether the copy was sent). The hub clears the client's refusal entry when a record exists for it, and records the text as today when none does.
6. **Show.** `show fleet divergence list` reads every record and answers client, device hash, hub hash, edited-at, received-at, revert state and whether a serving block declares the client. `show fleet divergence name <client>` adds the diff of hub text to device text from the whole-tree producer, its entry count, and the plugins the diff could not check.
7. **Adopt.** The handler takes the writer lock of `client-<name>.conf`, refuses an undeclared client, refuses when the record's device hash is not the given hash, publishes the record's bytes with `CommitConfigWithGuard`, removes the record under the same guard, releases, records the audit action. The observer pushes `config-changed`; the device fetches A, the hub answers `current`, `settleBaseline` stores A as baseline and acks `ok:true`.
8. **Revert.** The handler takes the same lock, refuses as adopt does, reads the hub's configuration and hashes it as V, writes the record's revert decision (D, V, user, time), releases, records the audit action, then calls `NotifyConfigChanged` through the target. At the next fetch with version D, `HandleConfigFetch` adds the discard field D only while the hub's configuration still hashes to V and the record's decision names D.
9. **Clear.** The record is removed by adopt, by a `current` answer to that client, and by an `ok:true` ack from that client; it is replaced by a newer upload.
10. **Doctor.** On the hub, a post-config check reads the records and the serving blocks: a declared client's record is a warning under `doctor-managed-client-diverged` whose fix names the show, adopt and revert commands; an undeclared client's record is reported as orphaned under the same code with the fix that declares it again or removes the record with the hub stopped.
11. **Diff fix.** `ze config diff` parses both sides as today, compares the resolved BGP tree as today and every other root as written, adds list orders and masks secrets, in one exported producer.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Device to hub | New verb `config-held`; new optional field `holding` of `config-ack` | No |
| Hub to device | Two new optional fields of the `config-fetch` answer | No |
| Managed server to store | Record read and write under the writer lock, through the store the server already borrows | No |
| Command handler to managed server and store | `DivergenceTarget`, installed by the hub after the server starts and withdrawn before the store closes | No |
| Managed client to store | A fourth `BaselineAccess` callback | No |

### Changed-Wire Census

**Search evidence:** On 2026-11-19 searched the whole tree outside `tmp/`, `bin/` and `.claude/` with `rg`, in every file type, for `ConfigFetchResponse`, `ConfigAck`, `VerbConfigFetch`, `VerbConfigAck`, `VerbConfigChanged`, `fleet.ConfigChanged` (Go), then for `config-fetch`, `config-ack`, `config-changed`, `held-version`, `config-held` and `discard-version` in every file that is not Go. Positive control: the Go search finds the known writer `internal/component/plugin/server/managed.go` `HandleConfigFetch` and the known reader `internal/component/managed/client.go` `FetchConfig`. Go carriers found: `client.go`, `handler.go`, `managed_serve.go`, `managed.go`, `pkg/fleet/envelope.go`, the fake hub `internal/test/fixture/misc_fixture_managed.go`, and tests (`client_*_test.go`, `handler_test.go`, `managed_serve_test.go`, `managed_panic_test.go`, `fleet_perf_test.go`, `managed_lifetime_test.go`, `managed_server_test.go`, `envelope_test.go`). Non-Go carriers: `test/managed/config-change-notify.ci` and `data-restore-client-live.ci` name the verbs in comments only and drive real daemons; no shell, Python or JavaScript file names a verb or field; the rest are docs and plan files. The diff output: `rg -l "config diff" test` finds `test/parse/cli-config-diff.ci`, `cli-config-diff-identical.ci`, `config-diff-list-order.ci`, `test/plugin/external-yang-inspect-asks-running-ze.ci`, `external-yang-inspect-no-daemon.ci`, `test/ui/pipe-local-command.ci`; `test/reload/bmp-route-action-redistribute-refused.ci` and `test/exabgp-compat/encoding/conf-llnh-lla-only.ci` match only in comments. Positive control for the diff: `rg -n "compareDiffInputs"` finds both callers, `artifact_config.go` and `inspect_route.go`. The key: `rg "meta/config/.*held|KeyConfigHeld"` finds nothing today; the generic readers of every key are `show data list`, `show data cat` and `ze data rm`.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| Verb `config-held` and its payload (new) | writer | `internal/component/managed/client.go`, a new upload function called from `fetchAgainstBaseline` | New; the hold branch is where the device knows it holds | New write, under `heldUploadTimeout` |
| same | reader | `internal/component/plugin/server/managed_serve.go` `handleRequest`, a new arm | The `default` arm answers "unknown method" today | New read; stores the record |
| same | reader (old hub) | The `default` arm of `handleRequest` in a hub built before this spec | Found by reading the arm | Preserved: the error stops the device's uploads for that connection (E-9) |
| same | reader (fake hub) | `internal/test/fixture/misc_fixture_managed.go` `handleManagedHubConnection` | Expects auth, fetch, ack in order; its device has no baseline, so never holds | Preserved: never receives the verb |
| `config-fetch` answer field `held-version` (new) | writer | `internal/component/plugin/server/managed.go` `HandleConfigFetch` | New; reads the record | New write, `omitempty` |
| same | reader | `internal/component/managed/client.go` `fetchAgainstBaseline` (through `FetchConfig`) | New | New read; decides the upload and the `held-by-hub` status field |
| same | reader (old device, fake hub types) | A device built before this spec; `managedConfigFetchResponse` in the fixture | Go's decoder ignores an unknown field | Preserved: absent for them |
| `config-fetch` answer discard field `discard-version` (new, the revert flag) | writer | `internal/component/plugin/server/managed.go` `HandleConfigFetch` | New; only while the record's decision names the request version D and the hub's configuration hashes to V | New write, `omitempty` |
| same | reader | `internal/component/managed/client.go` `fetchAgainstBaseline` | New | New read; applies only when it equals the active hash |
| same | reader (old device) | A device built before this spec | Ignores the field | Preserved: it holds, as fleet-6 (E-4) |
| `config-ack` field `holding` (new) | writer | `internal/component/managed/client.go` `holdFetched` | The ack carries only version, ok and error today | New write, `omitempty`; the error text stays for an old hub |
| same | reader | `internal/component/plugin/server/managed_serve.go` `handleRequest`, `config-ack` arm | Records every `ok:false` as `ManagedRejectConfigRejected` today | Changed: with a stored record the refusal entry is cleared instead |
| same | reader (old hub, fake hub) | The same arm before this spec; `managedConfigAck` in the fixture | Ignore the field | Preserved: an old hub shows the hold text as today |
| Store key `meta/config/client-<name>.conf/held` (new, `KeyConfigHeld`) | writer | The `config-held` arm (write, replace); the revert command (decision); the adopt command (remove); `HandleConfigFetch` on `current` (remove); the `config-ack` arm on `ok:true` (remove) | New | New, under the writer lock of `client-<name>.conf` |
| same | reader | `HandleConfigFetch`; the two show commands; the hub doctor check; `prepareConfigRename` | New | New reads |
| same | reader / remover (generic) | `show data list`, `show data cat` (hidden from `show data registered`: Private); `ze data rm` with the hub stopped | Generic key tools | Preserved; `RuntimeKey` keeps the running daemon's `update data key` and `delete data key` off it |
| Command RPCs `ze-managed:fleet-divergence-list`, `-show`, `-adopt`, `-revert` (new) | writer | Every command surface and the local `ze` route | New | New |
| same | reader | Handlers in `internal/component/managed` | New | New; `wire-methods.snapshot` updated |
| `show fleet status` answer fields `held-by-hub`, `revert-offered` (new) | writer | `internal/component/managed/cmd_fleet.go` `readFleetStatus` and the daemon's client report | New runtime fields | New, with the existing no-daemon value when no daemon answered |
| same | reader | `cmd_fleet.go` `decodeDaemonFleetStatus` (the local route reads the daemon's JSON); `test/managed/fleet-status.ci`; the `internal/test/localdatacoverage` evidence row | Found by reading the local route | Changed in step: decoder and test gain the fields |
| `ze config diff` and `show config diff` answer (content change) | writer | `internal/component/config/cli/cmd_diff.go` `diffSources`, moved into the new producer | BGP-only today | Changed: every root compared; the `ConfigDiff` shape is unchanged |
| same | reader | `outputDiffText`; `diffAnswer` payload of `artifact_config.go` and `inspect_route.go`; the six `.ci` files above; `test/ui/pipe-local-command.ci` (coverage line only) | Found by the search | Preserved: every expectation is a `contains` or `reject` on BGP or order lines; AC-22 adds the new case |
| `config-ack` `error` hold text (existing field) | writer / reader | `holdFetched`; `handleRequest` | Free text, parsed by no reader (fleet-6 A-8) | Preserved; gains a clause on whether the copy was sent |

### Integration Points
- `managed.ClientConfig` - `BaselineAccess` gains the held-read callback; a new report callback carries `held-by-hub` and the revert state to the client owner
- `pluginserver.ManagedServerConfig` - gains the store the record lives in, beside `ReadConfig`
- `cmd/ze/hub/managed_server.go` - installs `DivergenceTarget` after the server starts, withdraws it before the store closes
- `internal/component/config/infra` - the exported whole-tree diff producer, beside `ResolveBGPTree`
- `internal/core/diagnostic` - one code with its catalog entry
- `internal/core/audit` - two action constants

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | No | |

The `handleRequest` arm is an addition to a protocol's own verb dispatch, which
lists the verbs of one fixed protocol; it is not a registry of features. The
closure fills this table.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The hash the hub computes over the uploaded bytes equals the device's active hash, because the device hashes exactly the bytes it sends | `fleet.VersionHash` of one `activeConfig` read on the device; the same function on the hub | The record never matches the device and adopt never ends the hold | `TestHeldUploadHashesWhatItSends`; `fleet-divergence-adopt.ci` ends with the device in sync | unvalidated |
| A-2 | `CommitConfigWithGuard` of `client-<name>.conf` writes the plain mirror the hub's `ReadConfig` reads, so the next fetch hashes the adopted bytes | `restore.go` "the plain active file is written with the same bytes" | Adopt leaves the hub serving the old configuration | `TestAdoptServesHeldBytes` | unvalidated |
| A-3 | The write observer fires for the adopt's write of `file/active/client-<name>.conf` | `managed_server.go` observer over every written key | The device is told nothing until it reconnects | `fleet-divergence-adopt.ci`, connected device | unvalidated |
| A-4 | The stamp of the active version pointer is the time that version became active, and a configuration with no pointer has no such time | `storage.ReadActiveConfig` reads the pointer stamp; `StageMachineCandidate` stamps with the time of staging | `edited-at` is wrong or missing | `TestHeldReadAnswersActivationStamp`; an absent stamp sends no `edited-at` | unvalidated |
| A-5 | Go's JSON decoder in an old peer ignores the new fields | `encoding/json` default | An old peer refuses the payload | `TestOldPeerIgnoresNewFields` decodes each new payload into the pre-change types | unvalidated |
| A-6 | The hub's in-daemon parse of a client configuration refuses a section of an external plugin it did not start, and `DeclaredSchemas.Explain` identifies it | `declared_schemas.go` `SchemaWithDeclaredPlugins` comment | A malformed upload is stored, or a good one refused | `TestHeldUploadUncheckedPluginSection`, `TestHeldUploadMalformedRefused` | unvalidated |
| A-7 | `internal/component/config/infra` can host the producer: `internal/component/managed` can import it with no cycle, and `config/cli` (which links `plugin/all`) cannot be imported by `managed` | `config/cli` imports `plugin/all`; `infra` imports `config` | The producer needs another home | Compile of `internal/component/managed` with the import, run by the main thread | unvalidated |
| A-8 | `ze data rm` removes a `meta/config/` key with the hub stopped | `internal/component/config/storage/cli/main.go` registers `rm` | The orphan fix names a command that refuses | `fleet-divergence-orphan.ci` runs it | unvalidated |
| A-9 | The hub's post-config check can read the serving blocks' client names from the tree with the extraction the managed server is built from | `startManagedServer` reads `HubConfig.Servers[].Clients` | Orphans cannot be told from declared clients at post-config | `TestHubDivergedCheckOrphan` | unvalidated |
| A-10 | A 1 MiB configuration, base64 in JSON, crosses a healthy link within `heldUploadTimeout` | 1.4 MB frame; a LAN or WAN link above about 3 Mbit/s | Uploads time out on slow links (R-1) | `TestHeldUploadDeadline` with a stalled hub; `fleet-divergence-upload.ci` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A large upload stalls the device's notification loop, a `config-changed` meanwhile waits past the hub's 10 second push bound, the hub closes the connection, and the device reconnects, holds and uploads again | Repeated "the push failed" on the hub for one client; repeated uploads | The upload has its own context deadline, `heldUploadTimeout`, below `managedPushTimeout`. A hash whose upload failed is not uploaded again for the life of the client run unless the active hash changes; the warning names the manual routes. 1 MiB bound (E-5) |
| R-2 | A stale revert discards a newer local edit | A device applies the hub's configuration after its operator edited it again | The discard field names D; the device applies only when its active hash is D. A newer edit has another hash, holds and uploads, which replaces the record and drops the decision (AC-15) |
| R-3 | A revert applies a hub configuration the operator never saw, because the hub's configuration changed after the decision | The device runs a configuration other than the one shown | The discard field is offered only while the hub's configuration hashes to V. After a hub write the device holds again and `show fleet divergence name` reports the decision void (AC-16) |
| R-4 | An orphaned record: the client is removed from every serving block while a record exists | A record for a client the hub no longer serves | Never served (the client cannot authenticate); the hub doctor reports it orphaned with its fix; adopt and revert refuse it (AC-19) |
| R-5 | A client configuration with a section of an external plugin the hub did not start does not parse on the hub, so the diff cannot be computed | `show fleet divergence name` cannot diff | The upload is stored with the unchecked plugins noted; the show answers hashes, edited-at and an operator error naming the plugins and why. Adopt and revert still work: neither parses (AC-10) |
| R-6 | Version skew, new device and old hub: the hub answers `config-held` with "unknown method" every time | Repeated unknown-method errors on the hub | One attempt per connection; the device stops for that connection and holds as fleet-6 (AC-6) |
| R-7 | Version skew, old device and new hub: the device never uploads, never sends `holding`, ignores the discard field | The hub shows the old refused-config row and no record | Preserved fleet-6 behavior; the guide says to upgrade the device or resolve by hand (AC-7) |
| R-8 | Hub restart loses the record | A diverged client no longer listed after a restart | The record is in the store (E-11); `held-version` is read from it at each fetch (AC-17) |
| R-9 | A rename of `client-<name>.conf` moves or orphans the record | A record under the old name | `prepareConfigRename` refuses while the record exists (AC-18) |
| R-10 | The holding ack arrives with no record (upload refused, too large, or failed) and the hub shows nothing | A holding client invisible on the hub | The ack's `holding` clears the refusal entry only when a record exists; with none the hub records the hold text as today (Key Design Decisions, flagged against E-10) |
| R-11 | A session editor holds a staged candidate of `client-<name>.conf`, so adopt's publication is refused with `ErrCandidateExists` | Adopt refused naming a candidate | Refused with an operator error naming the candidate and its discard; nothing is removed |
| R-12 | The record carries the client's secrets (its hub secret, any password) | A secret in `show` output or a log line | The command never answers configuration text; the diff masks every secret leaf; the key is Private; the hub's store already holds the same secrets in `client-<name>.conf` |
| R-13 | A device whose apply of the reverted configuration fails acks `ok:false`, and the hub offers the revert at every fetch | Repeated refused configurations after a revert | Bounded by fetches, as every refused push is; the refusal shows on the hub with the device's reason, and the record stays for the operator to adopt instead |
| R-14 | The whole-tree diff changes `ze config diff` output for an input whose non-BGP roots differ | A `.ci` expecting "No differences" over such inputs | The census found none; AC-22 pins the new case; every existing diff test stays in the managed and parse runs |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A holding device could take a configuration it should have kept (a wrong revert binding), or a hub could show a wrong diff. Neither drops a running session; the device keeps running a configuration. A wrong diff producer makes `ze config diff` report too much or too little |
| How is it reverted? | Single commit revert per phase. Old peers ignore every new field, and the record key is ignored by older code. The diff producer is its own commit |
| Who else touches this path? | `spec-fleet-ui-6` (renders the divergence through `ConfigSource.Divergence`), `spec-fleet-1` (device registry), `spec-managed-server-hardening` (the managed listener), sessions editing `config/cli` diff code |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A holding device connects to a hub with no record | → | Upload in `fetchAgainstBaseline`; the `config-held` arm writes the record | `test/managed/fleet-divergence-upload.ci` |
| `show fleet divergence list` and `name` on the hub | → | The show handlers over the record and the diff producer | `test/managed/fleet-divergence-upload.ci` |
| `update fleet divergence name <client> adopt version <hash>` | → | Publication, record removal, `current`, baseline | `test/managed/fleet-divergence-adopt.ci` |
| `request fleet divergence name <client> revert version <hash>` | → | Decision, notification, discard field, device apply | `test/managed/fleet-divergence-revert.ci` |
| A local edit or hub write after a revert decision | → | The D and V bindings | `test/managed/fleet-divergence-revert-void.ci` |
| A hub restart with a record | → | `held-version` from the store | `test/managed/fleet-divergence-hub-restart.ci` |
| A record for an undeclared client; a rename of a client configuration with a record | → | Hub doctor check; rename refusal | `test/managed/fleet-divergence-orphan.ci` |
| `ze config diff` of two configurations differing only outside BGP | → | The whole-tree producer | `test/parse/cli-config-diff-whole-tree.ci` |
| `show fleet status` on a holding device | → | The two new fields | `test/managed/fleet-status.ci` (extended) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A device whose active hash D differs from its baseline connects to a hub that holds another configuration V and no record for it | The device uploads its active bytes with `config-held`; the hub's record holds exactly those bytes, D equal to the hash of the bytes, and the edited-at time; the device acks with `holding:true`; `show doctor` on the hub lists no refused configuration for that client; `show fleet divergence list` lists the client with D and V | The device still holds and applies nothing: resolution waits for the operator (E-1) |
| AC-2 | The same device is notified of a further hub write while the hub's record holds D | The fetch answer carries `held-version` D, the device sends no second upload, holds, acks `holding:true` and keeps its connection | One holding ack per hub write: the hub must see each |
| AC-3 | The device's operator edits again (active D2) and the device fetches | The device uploads D2, the record is replaced by D2's bytes, and any revert decision of D is dropped | None: the newest held configuration is the one shown |
| AC-4 | A holding device whose active configuration is larger than 1 MiB | No upload is sent; the device logs a warning under `doctor-managed-diverged` naming the manual routes; its ack says the copy was not sent and why; the hub shows that ack text as a refused configuration; the connection stays | The hold itself: the guarantee holds whatever the size (E-5) |
| AC-5 | A holding device uploads to a hub that accepts the verb and never answers it | The upload ends at `heldUploadTimeout`, below 10 seconds; a `config-changed` sent during it is answered before the hub's push bound; the same hash is not uploaded again in that client run | The connection may end if the write itself hit the deadline: the device reconnects and holds without uploading that hash again |
| AC-6 | A holding device connects to a hub built before this spec | It sends `config-held` once, gets the unknown-method error, sends no further upload on that connection, holds as fleet-6 and keeps its connection | The old hub's refused-config row: that hub knows no other way to show a hold |
| AC-7 | A device built before this spec holds against a hub built after it | The hub records the device's hold text as a refused configuration, keeps no record, and the device ignores `held-version` and any discard field | The old device is never reverted from the hub; fleet-6's manual routes remain |
| AC-8 | A `config-held` whose decoded configuration exceeds 1 MiB, whose hash differs from its bytes, or whose text does not parse for a reason other than an unchecked plugin section | The hub answers an error naming the cause and writes no record; the record of that client, if any, is unchanged; the device logs the refusal and holds | None of these is stored: the hub keeps only a configuration it can show |
| AC-9 | Client X uploads | The record is written under X's authenticated name only; no payload field names a client | None: the name never comes from the payload |
| AC-10 | `show fleet divergence name <client>` for a record | Answers the diff entries (added, removed, changed, every secret masked), their count, D, V, edited-at, received-at and the revert state. For a record whose text holds a section of an external plugin the hub did not start, it answers the hashes, the times and an operator error naming the plugins and why the diff is unavailable | No configuration text is answered: the texts carry secrets |
| AC-11 | `update fleet divergence name <client> adopt version D` while the record holds D, the client is declared and connected | `client-<name>.conf` holds the record's bytes as a new active version, the record is removed, the device is notified, fetches D, gets `current`, stores D as its baseline and acks `ok:true`; `show fleet status` on the device reports in sync; one audit entry of the adopt action names the user and the client | With the client not connected the same happens at its next connection |
| AC-12 | Adopt or revert with a hash that is not the record's device hash, or for a client with no record | Refused with an operator error naming the record's current hash (or that none exists) and, last, the show command; nothing is written | None |
| AC-13 | `request fleet divergence name <client> revert version D` while the record holds D, the hub's configuration hashes to V and the device is connected | The record stores the decision (D, V); the device is notified; its fetch answer carries the discard field D; it applies V through the push path; its baseline becomes the hash of its new active configuration; it acks `ok:true`; the hub removes the record; one audit entry of the revert action | A device with a confirmed-commit window open refuses the push as today and the decision stays for its next fetch |
| AC-14 | A revert is decided while the device is not connected | At its next connection the device takes the revert as in AC-13 | None |
| AC-15 | After a revert decision for D, the device's operator edits again (active D2) before the device fetches | The device holds, uploads D2, the record is replaced and the decision dropped; the device does not apply V | None: a newer local edit always wins over an older decision (E-4) |
| AC-16 | After a revert decision bound to V, the hub's configuration for the client is written again (V2) | The fetch answer carries no discard field, the device holds, and `show fleet divergence name` reports the decision void because the hub configuration changed, with the revert command as its fix | None: the operator saw V, not V2 (E-4) |
| AC-17 | The hub restarts while a record exists | The record is listed after the restart; the next fetch carries `held-version`; adopt and revert work | None |
| AC-18 | A rename of `client-<name>.conf` while its record exists | Refused, naming the record key; a rollback or an import of the same configuration is accepted and voids a revert decision by changing V | The rollback and import intent admissions do not read the record (E-11) |
| AC-19 | A record whose client no serving block declares | `ze doctor` on the hub reports it orphaned under `doctor-managed-client-diverged` with the fix that declares the client again or removes the record with the hub stopped; adopt and revert refuse it as for an undeclared client | The record stays until the operator acts: it is the only copy of the device's configuration |
| AC-20 | `ze doctor` on a hub holding a record for a declared client, daemon running or stopped | A warning under `doctor-managed-client-diverged` naming the client, D, V, and the show, adopt and revert commands; after adopt or revert it reports nothing for that client | A hold with no record (AC-4, AC-7) stays on the runtime refused-config row |
| AC-21 | `show fleet status` and `ze doctor` on a holding device | Status reports `held-by-hub` (the last `held-version`, or that the hub holds none) and `revert-offered` (none, or a stale offer); the doctor fix names the hub commands and, for a configuration over 1 MiB, the manual routes | With no daemon both fields say no daemon answered |
| AC-22 | `ze config diff a.conf b.conf` where the two differ only in a `plugin hub client` host and a secret leaf | Exit 0 and both changes listed, the secret shown masked; the existing BGP and list-order expectations are unchanged | A difference only in plain display order is still not a change, as today |
| AC-23 | A device that is not diverged, and every existing managed flow | Fetch, apply, ack, push, restore and notification behave as before; every test under `test/managed/` passes | None |
| AC-24 | A user whose profile denies the edit section runs adopt or revert | Refused by authorization; the show commands answer | None |
| AC-25 | The hub's configuration for a client is made equal to D by an import (fleet-6's manual route), or the client acks `ok:true` | The record is removed at the `current` answer or the ack | None: either proves hub and device agree |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Reconnects a device edited in an emergency and finds it on the hub | hold -> `config-held` -> record -> `show fleet divergence list` | `fleet-divergence-upload.ci` |
| 2 | Reads what the device changed | `show fleet divergence name` -> whole-tree diff | `fleet-divergence-upload.ci` |
| 3 | Keeps the device's change | adopt -> publication -> `config-changed` -> `current` -> baseline | `fleet-divergence-adopt.ci` |
| 4 | Discards the device's change | revert -> decision -> notification -> discard field -> push path | `fleet-divergence-revert.ci` |
| 5 | Is protected from a decision that went stale | edit or hub write after the decision -> hold | `fleet-divergence-revert-void.ci` |
| 6 | Compares two configurations that differ outside BGP | `ze config diff` -> whole-tree producer | `cli-config-diff-whole-tree.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestConfigHeldEnvelopeRoundTrip` | `pkg/fleet/envelope_test.go` | The payload and the three fields encode with their kebab-case keys | |
| `TestOldPeerIgnoresNewFields` | `pkg/fleet/envelope_test.go` | The new payloads decode into the pre-change types (A-5) | |
| `TestHeldUploadHashesWhatItSends` | `internal/component/managed/client_held_test.go` | One read answers bytes and their hash (A-1) | |
| `TestHeldReadAnswersActivationStamp` | `internal/component/managed/baseline_test.go` | edited-at is the active pointer's stamp; none without a pointer (A-4) | |
| `TestHoldUploadsWhenHubHoldsOther` | `internal/component/managed/client_held_test.go` | Uploads when `held-version` differs or is absent | |
| `TestHoldSkipsUploadWhenHubHoldsActive` | `internal/component/managed/client_held_test.go` | No upload when `held-version` equals the active hash | |
| `TestHoldUploadRefusedOverBound` | `internal/component/managed/client_held_test.go` | Over 1 MiB: no upload, the warning, the ack text | |
| `TestHeldUploadDeadline` | `internal/component/managed/client_held_test.go` | A hub that never answers: the upload ends at its deadline and a following `config-changed` is answered (A-10) | |
| `TestHeldUploadNotRetriedAfterFailure` | `internal/component/managed/client_held_test.go` | A failed hash is not uploaded again in the run; a new hash is | |
| `TestHeldUploadOncePerConnectionOldHub` | `internal/component/managed/client_held_test.go` | Unknown-method error: no further upload on that connection | |
| `TestHoldAckSaysHolding` | `internal/component/managed/client_held_test.go` | The hold ack carries `holding:true` and the hold text | |
| `TestDiscardAppliesWhenActiveMatches` | `internal/component/managed/client_held_test.go` | A discard field equal to the active hash applies through `OnCommit` | |
| `TestDiscardIgnoredWhenActiveDiffers` | `internal/component/managed/client_held_test.go` | Another hash holds and reports a stale offer | |
| `TestHeldArmWritesRecordUnderAuthName` | `internal/component/plugin/server/managed_held_test.go` | The record is written under the authenticated name (AC-9) | |
| `TestHeldArmRefusesOversize` | `internal/component/plugin/server/managed_held_test.go` | Decoded over 1 MiB refused, nothing written | |
| `TestHeldArmRefusesHashMismatch` | `internal/component/plugin/server/managed_held_test.go` | Claimed hash differs from the bytes: refused | |
| `TestHeldUploadMalformedRefused` | `internal/component/plugin/server/managed_held_test.go` | A text that does not parse: refused (A-6) | |
| `TestHeldUploadUncheckedPluginSection` | `internal/component/plugin/server/managed_held_test.go` | An unknown external plugin section: stored, plugins noted (A-6) | |
| `TestHeldArmReplacesAndDropsDecision` | `internal/component/plugin/server/managed_held_test.go` | A newer upload replaces the record and drops the decision | |
| `TestFetchAnswersHeldVersion` | `internal/component/plugin/server/managed_test.go` | `held-version` from the record | |
| `TestFetchOffersDiscardOnlyForDAndV` | `internal/component/plugin/server/managed_test.go` | Discard field only while the decision names the request version and the hub hashes to V | |
| `TestFetchCurrentClearsRecord` | `internal/component/plugin/server/managed_test.go` | `current` removes the record (AC-25) | |
| `TestAckOKClearsRecord` | `internal/component/plugin/server/managed_serve_test.go` | `ok:true` removes the record | |
| `TestHoldingAckWithRecordClearsRefusal` | `internal/component/plugin/server/managed_serve_test.go` | With a record, no `ManagedRejectConfigRejected` | |
| `TestHoldingAckWithoutRecordRecordsText` | `internal/component/plugin/server/managed_serve_test.go` | With none, the hold text is the refusal, as today (R-10) | |
| `TestHeldRecordDecodeStrict` | `internal/component/plugin/server/managed_held_test.go` | An undecodable record is an error, never "no record" | |
| `TestAdoptServesHeldBytes` | `internal/component/managed/cmd_divergence_test.go` | Publication of the held bytes, record removed under one guard (A-2) | |
| `TestAdoptRefusesStaleHash` | `internal/component/managed/cmd_divergence_test.go` | Another hash or no record refused, nothing written | |
| `TestAdoptRefusesUndeclaredClient` | `internal/component/managed/cmd_divergence_test.go` | An undeclared client refused | |
| `TestAdoptRefusesStagedCandidate` | `internal/component/managed/cmd_divergence_test.go` | A staged candidate refused with its fix (R-11) | |
| `TestRevertStoresDecisionAndNotifies` | `internal/component/managed/cmd_divergence_test.go` | Decision D and V stored, notification called | |
| `TestShowDivergenceMasksSecrets` | `internal/component/managed/cmd_divergence_test.go` | No text, secrets masked, count equals the entries | |
| `TestShowDivergenceReportsVoidDecision` | `internal/component/managed/cmd_divergence_test.go` | A decision whose V no longer matches is reported void | |
| `TestHubDivergedCheckDeclared` | `internal/component/managed/doctor_test.go` | A declared client's record: warning with the three commands | |
| `TestHubDivergedCheckOrphan` | `internal/component/managed/doctor_test.go` | An undeclared client's record: orphaned with its fix (A-9) | |
| `TestDivergedFixNamesHubCommands` | `internal/component/managed/doctor_test.go` | `managedDivergedFix` names show, adopt and revert | |
| `TestFleetStatusHeldFields` | `internal/component/managed/cmd_fleet_test.go` | The two fields, daemon and no daemon | |
| `TestRenameRefusedWithHeldRecord` | `internal/component/config/storage/rename_config_test.go` | Rename refused naming the key | |
| `TestRollbackAndImportIgnoreHeldRecord` | `internal/component/config/storage/rollback_config_test.go` | Rollback and import accepted with a record | |
| `TestWholeTreeDiffNonBGPRoot` | `internal/component/config/infra/diff_test.go` | A change outside BGP is an entry | |
| `TestWholeTreeDiffMasksSecrets` | `internal/component/config/infra/diff_test.go` | A secret leaf outside BGP is masked | |
| `TestWholeTreeDiffKeepsBGPPaths` | `internal/component/config/infra/diff_test.go` | BGP entries keep their resolved paths | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Held configuration size (device and hub) | 1 to 1048576 bytes | 1048576 | N/A: an empty configuration has no hash and is never held (`Diverged` is false) | 1048577 |
| Upload deadline `heldUploadTimeout` | A constant below `managedPushTimeout` | 5 seconds | N/A | `managedPushTimeout` or more is refused by `TestHeldUploadDeadlineBelowPushBound` |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `fleet-divergence-upload` | `test/managed/fleet-divergence-upload.ci` | A device edited with its daemon stopped holds; the hub lists it, shows the diff with a masked secret, the doctor names the commands; a further hub write causes no second upload | |
| `fleet-divergence-adopt` | `test/managed/fleet-divergence-adopt.ci` | Adopt with the shown hash: the device reports in sync; a stale hash is refused first | |
| `fleet-divergence-revert` | `test/managed/fleet-divergence-revert.ci` | Revert with the shown hash, device connected, then a second device offline: both run the hub's configuration | |
| `fleet-divergence-revert-void` | `test/managed/fleet-divergence-revert-void.ci` | A local edit after the decision keeps the edit; a hub write after the decision voids it | |
| `fleet-divergence-hub-restart` | `test/managed/fleet-divergence-hub-restart.ci` | The record and the adopt survive a hub restart | |
| `fleet-divergence-orphan` | `test/managed/fleet-divergence-orphan.ci` | Rename refused; an undeclared client's record reported orphaned and removed by `ze data rm` (A-8) | |
| `cli-config-diff-whole-tree` | `test/parse/cli-config-diff-whole-tree.ci` | Two configurations differing in a `plugin hub client` host and secret: both listed, secret masked | |
| `fleet-status` (extended) | `test/managed/fleet-status.ci` | The two new fields | |

Every new `.ci` is drafted under `test/draft/managed/` (and `test/draft/parse/`) at implementation and promoted through `/ze-test`.

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | - | - | The fleet protocol has no other implementation; skew between two Ze builds is covered by AC-6 and AC-7 | |

## Files to Modify
- `pkg/fleet/envelope.go` - `VerbConfigHeld`, the `ConfigHeld` payload, `held-version` and `discard-version` on the fetch answer, `holding` on the ack
- `pkg/zefs/keys.go` - `KeyConfigHeld`
- `internal/component/managed/client.go` - upload, discard check, holding ack, the report callback
- `internal/component/managed/baseline.go` - the held-read callback: bytes, hash, activation stamp from one read
- `internal/component/managed/cmd_fleet.go` - `held-by-hub`, `revert-offered`, decoder
- `internal/component/managed/doctor.go` - the hub check; `managedDivergedFix`
- `internal/component/managed/register.go` - the four commands, the check
- `internal/component/managed/yang/ze-managed-cmd.yang` - `show fleet divergence`, `update fleet divergence`, `request fleet divergence`
- `internal/component/plugin/server/managed_serve.go` - the `config-held` arm; the ack arm's `holding` and clear
- `internal/component/plugin/server/managed.go` - `held-version`, the discard field, clear on `current`
- `cmd/ze/hub/managed_server.go` - pass the store; install and withdraw `DivergenceTarget`
- `cmd/ze/hub/managed_owner.go` - carry the new report into status
- `internal/component/config/storage/rename_config.go` - the record key in the refusal list
- `internal/component/config/storage/pointer.go` - a sibling of `ReadActiveConfig` that answers the bytes and the active pointer's stamp from one pointer read
- `internal/component/config/cli/cmd_diff.go` - use the producer; remove `diffSources` and the masking helpers it moves
- `internal/core/diagnostic/codes_plugin_infra.go` - `doctor-managed-client-diverged` and its catalog entry
- `internal/component/plugin/all/testdata/wire-methods.snapshot` - four RPCs
- `docs/architecture/fleet-config.md` - "Protocol" table and two-phase table, "The baseline and the hold" ("How a hold ends", "What the hub sees"), the store keys
- `docs/guide/fleet-config.md` - "When a Client Holds", "End the Hold", "See Where a Client Stands", "What the Client Keeps in Its Store"
- `docs/architecture/doctor-and-health-checks.md` - the new check row
- `docs/architecture/zefs-format.md` - the new key
- `docs/guide/command-reference.md`, `docs/architecture/api/commands.md` - four commands; the `ze config diff` scope
- `features/fleet-management.md` - the new real-path tests

## Files to Create
- `internal/component/config/infra/diff.go` - the exported whole-tree producer
- `internal/component/plugin/server/managed_held.go` - the record type, its codec and its locked read, write and remove
- `internal/component/managed/cmd_divergence.go` - the four handlers, `DivergenceTarget`, the local routes
- `internal/core/audit/action_fleet_divergence.go` - the adopt and revert actions
- The test files named in the TDD Test Plan
- The `.ci` files of the Functional Tests table, drafted under `test/draft/`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/managed/yang/ze-managed-cmd.yang`: four commands. No config leaf |
| YANG validation constraints | Yes | `name` takes the client-name type the hub `client` list key uses; `version` takes a pattern of 16 lower-case hex characters (`fleet.VersionHash`) |
| YANG custom validators | Yes | `name` completes from the record keys through a `CompleteFn` |
| CLI commands/flags | Yes | `internal/component/managed/cmd_divergence.go`, registered from `register.go`. No flag |
| CLI grammar (keyword before value) | Yes | `show fleet divergence list`, `show fleet divergence name <client>`, `update fleet divergence name <client> adopt version <hash>`, `request fleet divergence name <client> revert version <hash>`; `./le cli grammar`, `./le cli ownership`, `./le cli catalog` |
| Editor autocomplete | Yes | From the YANG command tree, `name` from the `CompleteFn` |
| Functional test for new RPC/API | Yes | The `fleet-divergence-*.ci` set |
| Pipe completeness | Yes | The two shows register their shapes and columns as local-data commands, each with its `internal/test/localdatacoverage` row |
| Env var registration | N-A | No environment value |
| Doctor check for runtime dependencies | N-A | No new file, socket, port, binary or certificate. The record is state, covered below |
| Doctor code per problem class | Yes | Hub: a record for a declared client, and an orphaned record: `doctor-managed-client-diverged`, post-config check in `internal/component/managed/doctor.go`. Hub: an upload refused, and a hold with no record: the existing `doctor-managed-client-rejected`, runtime. Device: an upload over the bound, refused or timed out: the existing `doctor-managed-diverged`, post-config, with its fix. The command refusals are operator errors with a fix and no log line |
| Prometheus counters/metrics | Yes | `ze_managed_config_held_total{result}` (stored, refused) on the hub, beside `ze_managed_config_fetch_total` |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not a BGP change |

Discovery (`ai/rules/repo-maintenance.md`): an agent finds this through the `ai/INDEX.md` row "fleet, managed, server, backup, bootstrap", which names `docs/architecture/fleet-config.md`, and through `ai/doors/hub-engine.md`. Regression is prevented by the functional tests above and `./le arch doctor-coverage`. Drift is prevented by the command registry and its `wire-methods.snapshot`, the `pkg/zefs` key registry and the doctor check registry. Verification is `./le --name <session> test managed -a` and the `test/parse` diff tests.

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/fleet-management.md`: the divergence tests as real-path evidence |
| 2 | Config syntax changed? | No | No config leaf |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md`: four commands; `ze config diff` compares every root |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | No | The commands belong to the existing managed component |
| 6 | Has a user guide page? | Yes | `docs/guide/fleet-config.md`, phase by phase |
| 7 | Wire format changed? | Yes | `docs/architecture/fleet-config.md` "Protocol": the verb and the three fields; the fleet protocol has no page under `docs/architecture/wire/` |
| 8 | Plugin SDK/protocol changed? | No | Nothing under `pkg/plugin` changes |
| 9 | RFC behavior implemented, changed, or newly proven? | No | No RFC governs managed configuration |
| 10 | Test infrastructure changed? | No | Existing managed fixtures |
| 11 | Affects daemon comparison? | No | Other daemons have no hub-managed configuration |
| 12 | Internal architecture changed? | Yes | `docs/architecture/fleet-config.md` |
| 13 | Route metadata keys added/changed? | No | None |
| 14 | Prometheus counters added/changed? | Yes | `docs/architecture/fleet-config.md`, the managed metrics list |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | The command inventory and `wire-methods.snapshot` |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Derived at implementation with `./le spec citation anchors spec plan/post-release/features/spec-fleet-7-config-reconnect-resolution.md`. Known now: `docs/architecture/fleet-config.md` anchors `client.go` (`fetchAgainstBaseline`, `holdFetched`), `baseline.go`, `managed_serve.go`, `cmd_fleet.go` and `doctor.go`. Declared by `// Design:` headers of changed files, run 2026-11-19: `docs/architecture/config/syntax.md` (`cmd_diff.go`) is unaffected: its diff paragraph is about `DiffMaps` attributing inactive intent, which the producer keeps, and it states no BGP-only scope. `docs/architecture/storage-backends.md` (`rename_config.go`) is unaffected: it documents no rename refusal list (`grep -n -i "unfinished identity\|confirm-pending" docs/architecture/storage-backends.md` finds none). Mentioned only: `docs/guide/config-editor.md` "Secrets are masked on every display path" stays true and gains no edit; `docs/features/fleet-management.md` is row 1 |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/fleet-config.md` "End the Hold" examples; the guide's `ze config diff` examples if any name BGP-only scope |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the verb, the fields, the key, the code, the commands as stubs
   - Tests: `fleet-divergence-upload.ci` (failing at the record), `TestConfigHeldEnvelopeRoundTrip`, `TestOldPeerIgnoresNewFields`
   - Files: `envelope.go`, `keys.go`, `codes_plugin_infra.go`, `ze-managed-cmd.yang`, `cmd_divergence.go` stubs, `register.go`, `action_fleet_divergence.go`, the snapshot
   - Verify: the four commands answer through the daemon and locally; `./le cli grammar` passes
2. **Phase: Diff producer and the `ze config diff` fix** -- its own commit (E-3)
   - Tests: the `infra/diff_test.go` set, `cli-config-diff-whole-tree.ci`, the existing `test/parse` and `test/plugin` diff tests
   - Files: `infra/diff.go`, `cmd_diff.go`
   - Verify: AC-22. The command reference states the new scope in this phase
3. **Phase: Upload and record**
   - Tests: the upload and arm unit tests, `TestFetchAnswersHeldVersion`, `TestFetchCurrentClearsRecord`, `TestAckOKClearsRecord`, the holding-ack tests, `TestRenameRefusedWithHeldRecord`, `TestRollbackAndImportIgnoreHeldRecord`
   - Files: `client.go`, `baseline.go`, `pointer.go`, `managed_held.go`, `managed_serve.go`, `managed.go`, `managed_server.go`, `rename_config.go`
   - Verify: AC-1 to AC-9, AC-17, AC-18, AC-23, AC-25. The design page gains the verb, `held-version` and `holding`
4. **Phase: Show**
   - Tests: `TestShowDivergenceMasksSecrets`, `fleet-divergence-upload.ci`
   - Files: `cmd_divergence.go`
   - Verify: AC-10
5. **Phase: Adopt**
   - Tests: the adopt tests, `fleet-divergence-adopt.ci`, `fleet-divergence-hub-restart.ci`
   - Files: `cmd_divergence.go`, `managed_server.go`
   - Verify: AC-11, AC-12, AC-24. The guide's "End the Hold" gains adopt
6. **Phase: Revert**
   - Tests: the discard and revert tests, `TestFetchOffersDiscardOnlyForDAndV`, `TestShowDivergenceReportsVoidDecision`, `fleet-divergence-revert.ci`, `fleet-divergence-revert-void.ci`
   - Files: `client.go`, `managed.go`, `cmd_divergence.go`
   - Verify: AC-13 to AC-16. "How a hold ends" is corrected for the revert (E-4)
7. **Phase: Doctor and status**
   - Tests: the hub check tests, `TestDivergedFixNamesHubCommands`, `TestFleetStatusHeldFields`, `fleet-status.ci`, `fleet-divergence-orphan.ci`
   - Files: `doctor.go`, `cmd_fleet.go`, `managed_owner.go`
   - Verify: AC-19 to AC-21

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | The upload hashes the bytes it sends from one read. The hub recomputes the hash and never trusts the claimed one. The discard field is offered only for D and V and applied only for D. A failed upload returns no error from the fetch path |
| Naming | The verb is `config-held`; the fields `held-version`, `discard-version`, `holding`; the key `meta/config/client-<name>.conf/held`; the state `diverged`, never `conflict` |
| Data flow | The revert applies through `applyManagedPush`, never a second reload path. Adopt writes `client-<name>.conf` and lets fleet-6 end the hold |
| Operator errors and doctor codes | Every refusal is an `opererr` error with its fix on the last line; the upload warnings name `doctor-managed-diverged`; the hub check declares `doctor-managed-client-diverged` at post-config |
| Rule: `ai/doors/system.md` | Each `pkg/fleet` and `pkg/zefs` change lands with its page |
| Rule: `ai/rules/goroutine-lifecycle.md` | The upload runs on the notification loop's goroutine; no goroutine is started for it |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The verb and fields exist | `grep -n "config-held\|held-version\|discard-version\|holding" pkg/fleet/envelope.go` |
| The key is registered | `grep -n "meta/config/{name}/held" pkg/zefs/keys.go` |
| The doctor code exists and a check declares it | `./le arch doctor-coverage check` |
| The four commands are registered and grammatical | `./le cli list`; `./le cli grammar` |
| The managed suite and the diff tests pass | `./le --name <session> test managed -a`; the `test/parse` diff tests |
| No operator message is untyped | `./le arch operator-error check`, `./le arch error-inventory check` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Untrusted upload | The upload is input from an authenticated client. Bounded twice: the frame by `MaxMessageSize`, the decoded configuration by 1 MiB on the hub, whatever the device claims. The hash is recomputed. The text is parsed before storage and a malformed one is refused |
| Name scope | The record is written only under the authenticated session's name; the payload carries no name (AC-9) |
| Adopt and revert authorization | `update` and `request` are in the edit section, so a read-only profile is refused; each records an audit action with user and client |
| Revert binding | Offered only while the hub's configuration hashes to V and applied only while the device's active hash is D. A hub can make a holding device apply only a configuration bound to the device's own hash; a hostile hub could forge it, but a hub can already push to every device that does not hold, so the hold protects against accidental loss, not against the hub |
| Secret exposure | The commands answer no configuration text; every diff entry under a secret leaf is masked; the key is Private and is not reachable by the running daemon's data-key RPCs (`RuntimeKey`) |
| Record growth | One record per declared client name, each at most 1 MiB; replaced, never appended |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| The adopted device stays diverged | A-1 or A-2 is broken: back to DESIGN |
| An import cycle from `managed` to the producer | A-7 is broken: back to DESIGN for the producer's home |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- Adopt costs one write: fleet-6's `current` answer already ends a hold and settles the baseline.
- The upload bound is set by the notification loop, not by storage: the loop is blocked while it runs.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Record per client in the hub's store (E-11) | In memory in the managed server; wait for the fleet-1 registry | Memory loses it on a hub restart; fleet-1 is a skeleton (E-1) |
| Verb `config-held` (E-8) | `config-push` | `config-push` already names hub-to-device delivery |
| Revert as a field of the fetch answer (E-4) | A new hub-to-device verb | An old device silently drops an unknown hub verb, so a new verb would look delivered and do nothing; the fetch field also reaches an offline device at its next fetch |
| `held-version` in the fetch answer (E-9) | Upload at every hold | A held device fetches at every hub write; re-uploading 1 MiB each time loads both sides and the loop |
| `holding` field on the ack (E-10) | Parse the hold text on the hub | No reader parses free text (fleet-6 A-8), and a wording change would break it |
| A holding ack clears the refusal only when a record exists | Never record a holding ack (E-10 as worded) | A hold whose upload was refused or never sent would otherwise be invisible on the hub. This refines E-10 and is flagged for the owner's gate |
| 1 MiB bound and its own deadline (E-5) | `MaxMessageSize` (16 MiB); the 30 second default write deadline | Either keeps the loop busy past the hub's 10 second push bound, which closes the connection and loops |
| Upload before the hold ack | Ack first | With the upload first, the hub knows at the ack whether a record exists |
| Parse at upload, store an unchecked plugin section | Store blindly; refuse every parse failure | Blind storage keeps what the hub cannot show; refusing an external plugin section makes such devices unresolvable from the hub |
| Producer in `internal/component/config/infra` | Export from `config/cli`; a new package | `config/cli` links `plugin/all`, which `managed` cannot import; `infra` already holds `ResolveBGPTree` |
| BGP compared resolved, other roots as written | Every root as written | Keeps every existing `ze config diff` expectation; changes only what was missing |
| No configuration text in the command answer | Both texts, as the fleet-ui `Divergence` value has | The texts carry secrets; the page's texts are fleet-ui-6's to decide |

## Known Limitations
- The divergence web page is `plan/post-release/features/spec-fleet-ui-6-config-rollout-audit-admin.md`. Its `Divergence` value has `HubText` and `DeviceText`, which this spec's command does not answer (secrets); fleet-ui-6 decides how its page shows text.
- A held configuration over 1 MiB is resolved with fleet-6's manual routes (E-5).
- On a hub that did not start an external plugin a client configuration declares, the diff of that client is not available; adopt and revert are.
- A hash whose upload failed is not uploaded again until the device's client restarts or its configuration changes.
- A device built before this spec cannot be reverted from the hub.

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
- [ ] AC-1..AC-25 all demonstrated
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
| Evidence: nothing landed | `pkg/fleet/envelope.go` holds four verbs and none carries a config upstream. No `diverged` state exists in `internal/component/managed` or `internal/component/plugin/server` |
| Evidence: targets gone | `internal/component/config/diff/` has never existed. `internal/component/plugin/server/registry.go` does not exist: fleet-1 is a skeleton. `plugin/server/managed.go` is `ManagedConfigService`, fetch and hashing; the one hub dispatch is `managed_serve.go` `handleRequest`, a switch over `config-fetch`, `config-ack` and `ping` |
| Evidence: the dependency is unbuilt and is being redesigned | `spec-fleet-6-config-freeze` (closed 2026-11-19) carried a REWORK disposition of the same date. The baseline, the hold and how the hub learns `diverged` are not settled |
| Evidence: the candidate pointer is the wrong home | `storage.WriteCandidateVersionWithGuard` (`internal/component/config/storage/pointer.go`) refuses with `ErrCandidateExists` while a candidate is staged, and `restoreConfigLocked` (`restore.go`), the body of `RestoreConfig` and `CommitConfigWithGuard`, stages one first. A pushed config held in that pointer would block every editor commit, import and restore of that client's config, under a refusal that tells the operator to discard a staged change |
| Evidence: the hub keeps nothing about a client | `ManagedServer.conns`, `pending` and the doctor record of `managed_doctor.go` are in memory. The `config-ack` arm of `handleRequest` logs and updates the doctor record, and stores no version |
| Evidence: the web view moved | The umbrella's Direction Update of 2026-11-18 gives every fleet web view to the `fleet-ui` set (`spec-fleet-ui-6-config-rollout-audit-admin.md`). This spec keeps the data, the protocol and the commands |
| Evidence: the command breaks the grammar | `ai/rules/cli.md`: commands are verb-first and no `--flag` reaches the daemon. `ze fleet resolve --adopt` is neither |
| Evidence: the verb name is taken | `config-push` already names delivery from the hub to a device: `test/managed/config-push-transactional.ci`, "Live `config-push`" in `docs/architecture/core-design.md`, `cmd/ze/pushed_config.go` |
| Evidence: A-2 is false as worded | The web editor's diff is bound to one user's session. Two functions take two arbitrary config texts: `compareDiffInputs` (`internal/component/config/cli/cmd_diff.go`), behind `show config diff`, and `computeTreeAnnotatedDiff` (`internal/component/cli/diff_tree.go`), unexported |
| Evidence: transport bounds hold, citations moved | `defaultWriteDeadline` is 30 seconds and is applied in `writeFrame` (`pkg/plugin/rpc/conn.go`); `MaxMessageSize` is 16 MiB (`framing.go`). The line numbers in "Post-wave corrections" are stale |
| Evidence: no Existing Mechanisms table | The section is absent. Candidates: the managed listener and its dispatch, the `pkg/fleet` envelopes and version hash, the hub write observer and `NotifyConfigChanged`, `storage.RestoreConfig`, the per-config intent record of `confirm.StoreRecorder` under `meta/config/{name}/`, the two-text diff, the managed doctor record, the command registry |

The goal holds: once fleet-6 makes a device hold instead of losing a local
edit, somebody has to be able to end the hold from the hub. Nothing landed
does that. The design does not fit `main`, and it cannot be redone before
fleet-6 is, because three of its requirements are requirements on fleet-6's
shape.

Those three. The hub must learn that a device is holding; the skeleton reports
it in the acknowledgement, which today would show in doctor as a rejected
config and needs its own cause and code. A revert must reach a holding device
as a revert: the change notification carries a version and nothing else, so a
device that holds cannot tell the operator's decision from any other hub
write. And after an adopt the device is still dirty under fleet-6's check
unless something resets its baseline to the config it is running.

The record question has a smaller answer than fleet-1. The hub needs the
pushed config to survive a restart, and its presence is the `diverged` flag.
A per-config record under `meta/config/{name}/`, as the confirmed-commit
window keeps one, gives both without a device registry. Whether fleet-7
carries that record itself or waits for fleet-1 is the owner's decision.

Smaller corrections: the up-verb needs a name that does not reverse the
meaning `config-push` already has; the resolve and show commands are
registered daemon commands in the verb grammar, and the web page that renders
them belongs to `fleet-ui`; the hold must not be built as an error, or the
device reconnects about once a second.

Unverified: `internal/component/web/fleetui/` and
`docs/architecture/fleet-interface.md` are another session's uncommitted work.
They name this spec as the answerer of a divergence view and want a time of
the local edit, which only the device can report. That shape is a proposal
until it is committed.

The redesign runs through `/ze-spec` after fleet-6's design is approved.

## Owner Decisions (2026-11-19)

| ID | Gate | Decision |
|----|------|----------|
| E-1 | Scope | Full hub-side resolution. The device sends its held configuration up; the hub keeps one record per client; the hub shows a diff; adopt (the device configuration becomes the hub's for that client) and revert (the device drops its change and takes the hub's). The web page is fleet-ui-6's (`plan/post-release/features/spec-fleet-ui-6-config-rollout-audit-admin.md`); fleet-7 owns data, protocol and commands. The record is per client and does not wait for fleet-1 (fleet-6 D-4) |
| E-2 | Research | `pkg/fleet` changes are deliberate and backward compatible: one new device-to-hub verb, fetch-answer fields, one ack field. Each lands with its page (`docs/architecture/fleet-config.md` "Protocol", "How a hold ends"; the guide's "End the Hold") |
| E-3 | Research | The `ze config diff` / `show config diff` BGP-only defect (verified by run: a changed `plugin hub client` host shows "No differences"; producer `internal/component/config/cli/cmd_diff.go` `compareDiffInputs` / `diffSources` diffs only `infra.ResolveBGPTree`) is fixed inside fleet-7: one exported whole-tree, secret-masked, structured diff producer, used by the hub command and by `ze config diff`, in its own commit with its own test. The Task names it and an AC covers it (AC-22) |
| E-4 | Design | Revert: a flag in the `config-fetch` answer, "discard D", bound to the device hash D and the hub hash V the operator saw; offered only while the hub's configuration for that client still hashes to V. A device whose active hash is exactly D applies the hub configuration through fleet-6's machine path (`applyFetched` -> `OnCommit` -> `applyManagedPush` -> `StageMachineCandidate`, `recordPushBaseline`). A newer local edit (active differs from D) or a newer hub write (hub differs from V) voids it. Works for an offline device at its next fetch; an old device ignores the field and keeps holding. No new hub-to-device verb. The device-side check lives in the same function as the hold (`fetchAgainstBaseline`) |
| E-5 | Design | Size bound of the upload: a fixed 1 MiB of raw configuration. A larger held configuration is refused on the device with an operator error, and the hold is resolved by hand. The upload has its own context deadline |
| E-6 | Design | Hub commands: `show fleet divergence list`; `show fleet divergence name <client>` (diff, change count, both hashes, edited-at); `update fleet divergence name <client> adopt version <hash>`; `request fleet divergence name <client> revert version <hash>`. `<hash>` is the record's device hash the operator saw; a changed record refuses. Owner `internal/component/managed`, wire prefix `ze-managed:`, reached through an installed target as `FleetTarget` / `DataRPCTarget`. The mutating commands are in the edit section by verb; audit actions for adopt and revert |
| E-7 | Design | Page fields: the device sends the time its active version became active (`edited-at`) with the upload; the hub's change count is the number of entries in its diff |
| E-8 | Main thread, owner-approved design | Verb name `config-held` (`config-push` already means hub-to-device delivery) |
| E-9 | Main thread, owner-approved design | The hub's `config-fetch` answer carries `held-version` (the hash of the record it holds for this client). The device uploads only when it differs from its active hash. With an old hub (field absent) the device uploads at most once per connection and stops for that connection when the hub refuses the verb |
| E-10 | Main thread, owner-approved design | The ack gains optional `holding: true`; the hub then records no `ManagedRejectConfigRejected` for it. A new hub doctor code `doctor-managed-client-diverged`: the check reads the store record; its fix names the adopt and revert commands. An old device's hold keeps today's refused-config row |
| E-11 | Main thread, owner-approved design | Record: key `meta/config/client-<name>.conf/held` (new `KeyConfigHeld` in `pkg/zefs/keys.go`), JSON (held bytes, device hash, edited-at, pending revert decision D/V), written with the writer lock. Cleared by adopt (same guard as the write), by the hub answering `current` to that client, by an ack `ok:true`; replaced by a newer upload. `prepareConfigRename` refuses a rename while a record exists (`rename_config.go`). It does not join `admitConfigRollbackLocked` or `ImportConfig`'s intent refusal. A record whose client no serving block declares is reported as orphaned, never served. It survives a hub restart (store, not memory) |
| E-12 | Main thread, owner-approved design | Device status: `show fleet status` adds `held-by-hub` (the hash from `held-version`) and `revert-offered`; `managedDivergedFix` (`internal/component/managed/doctor.go`) names the hub commands |
