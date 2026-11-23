# Spec: fleet-10 -- Drift Detection by Hash Chain

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol, plugin, config, cli |
| Priority | P2 |
| Release reason | Thomas put this fleet capability (configuration drift detection by SHA-1 hash chain, repaired by the hub with a diff resync and a full replay fallback) in the first release; existing first-release managed behavior and the fleet-6/fleet-7 hold guarantees remain required. |
| RFC gaps | - |
| Depends | plan/release/features/spec-fleet-1-device-registry.md, spec-fleet-7-config-reconnect-resolution (closed 2026-11-19: the hold, adopt and revert this spec preserves, documented in `docs/architecture/fleet-config.md`, "The baseline and the hold"), plan/release/features/spec-fleet-ui-5-tasks-alarms-notifications.md, plan/release/features/spec-fleet-ui-8-alarm-causes.md |
| Phase | 10/10 of plan/post-release/features/spec-fleet-0-umbrella.md |
| Handoff | - |
| Design tier | strongest tier |
| Implementation tier | development tier |
| Updated | 2026-11-23 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Add continuous drift detection by SHA-1 hash chain for managed fleet configuration deltas. Both the hub and each managed client fold every applied configuration change into a SHA-1 chain and compare the chain head on every reply over the managed TLS channel. When the two ends disagree and the client has not made a local emergency edit (preserving the `fleet-6`/`fleet-7` baseline and hold guarantee), the mismatch triggers a diff-based resync first, with a full configuration replay as fallback.

### Scope Confirmed at Gate (2026-11-23)

| Decision | Confirmed choice |
|----------|------------------|
| What is folded and synced | Managed config deltas: fold each applied config change into the chain, compare on every reply, diff-based resync first, full config replay fallback |
| Interaction with `fleet-6`/`fleet-7` hold | Preserve `fleet-6`/`fleet-7` hold: auto-resync repairs missed/diverged hub changes, while a local device edit (`active != baseline`) still holds for operator `adopt`/`revert` |
| Hash algorithm | SHA-1 chain as requested (non-adversarial sync checksum over authenticated TLS 1.3) |
| Spec bucket and path | `plan/release/features/spec-fleet-10-drift-hash-chain.md` (phase 10 of `plan/post-release/features/spec-fleet-0-umbrella.md`). Confirmed at the gate under `plan/post-release/features/`; moved the same day, when Thomas put drift detection with hub repair in the first release |

### The Configuration resync failed alarm

Added 2026-11-23, after the gate above. When the diff resync and the full
replay both fail, the doctor codes and `show fleet status` say so only to an
operator who asks; the alarm list of `fleet-ui-5` is where the estate tells the
operator what is wrong. `fleet-ui-5` derives each alarm from a fact the hub
holds, and an alarm kind is a registered entry with its condition, its severity
and its sentence, with no central list. This spec registers one kind from its
own package.

| Part | Value |
|------|-------|
| Name | `config-resync-failed`, shown as Configuration resync failed |
| Condition | The last resync of a client that holds no local edit ended with both the diff resync and the full replay failed, and the hub's chain head for the client still differs from the client's last reported head. Read from this spec's per-client chain state on the hub, so nothing new is stored for the alarm |
| Not this alarm | A full replay the client refused at verify is Configuration rejected (`fleet-ui-5`). A client whose active configuration differs from its baseline holds, and is Configuration diverged: no resync runs for it, so this alarm clears |
| Clears | The chain heads agree again: a later diff resync, full replay or fetch succeeded |
| Severity | Warning, as Configuration rejected: the device runs a known configuration, not the hub's latest |
| Sentence | Names the device, the time of the failed attempt, why each of the two steps failed, and the doctor code and command that show it [pending design: exact text, `ai/rules/user-facing-errors.md`] |
| Causes (`fleet-ui-8`) | Device offline. No resync runs while the device is offline, and its reconnect starts one, so the alarm is hidden behind the offline alarm rather than listed beside it (choice C-1) |
| Device tab | Config, registered with the page's alarm tab registry (`mustRegisterAlarmTab`) from this spec's own register file |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/hub-engine.md`, `ai/doors/config-yang.md`, `ai/doors/plugin-infra.md`, `ai/doors/commands-cli.md`, `ai/doors/core.md` - subsystem boundaries, public `pkg/` contracts, and persistence rules
  → Constraint: `pkg/fleet` is public API; wire changes must be intentional and updated in `docs/architecture/fleet-config.md` in the same work.
  → Constraint: runtime state persists only through `statestore` / `storage.Storage` under keys registered in `pkg/zefs/keys.go` (`MustRegister`), never raw files; never nest a key under an existing value key (`meta/instance/managed`, `meta/fleet/device/{name}/record`).
  → Constraint: `ai/doors/config-yang.md` / `ai/rules/config.md` restricts config content manipulation to a parsed YANG tree or `set` lines (no raw text surgery or private config-syntax merge) — any byte-level vs tree-level delta choice must explicitly reconcile this rule.
- [ ] `docs/architecture/fleet-config.md` - managed protocol, `VersionHash`, baseline/hold/revert, non-goals
  → Constraint: `## Non-Goals` currently excludes incremental config updates ("Full config on change; configs are small (< 10KB typically)") — this spec departs from that row and must present the departure to the owner.
  → Constraint: client baseline (`meta/managed/base-version`) is always computed by reading back active bytes after commit (`RecordActiveAsBaseline`), and every hub apply must pass `CommitOver` -> `StageMachineCandidateOver` over `activeCheck(decided)` so a racing local commit wins and holds.
  → Constraint: every loop-blocking client RPC must carry an explicit timeout; loop round-trips other than full `config-fetch` (30s) must stay under `ManagedPushTimeout` (10s).
- [ ] `docs/architecture/api/commands.md` - `## Design decisions`, builtin RPCs, answer shapes, pipes
  → Constraint: command paths name the subject and stop; scope/presentation/format are pipe operations after the path.
- [ ] `docs/architecture/config/syntax.md`, `docs/architecture/config/yang-config-design.md` - serialization, comments, schema stamp, inactive nodes, `UserOrdered` lists, `$9$` secrets, external-plugin schemas
  → Constraint: `ze:sensitive` leaves encode cleartext with a random salt (`secret.Encode`) on `Serialize`/`SerializeSet`; `$9$` strings round-trip identically only when preserved via `Tree.storedSecrets`.
  → Constraint: a hub never executes a client's external plugin binary (`SchemaWithDeclaredPlugins`), so hub-side YANG parsing fails on `unchecked-plugins` sections — diff-based resync must fall back to full replay whenever a tree parse/diff cannot cover a client's config.
- [ ] `plan/release/features/spec-fleet-1-device-registry.md` - ready sibling spec lifting the 128-client cap to 2,000+ devices
  → Constraint: hub-side read limit will drop to ~1.4 MiB (`base64(HeldConfigMax) + 64 KiB`), while client-side read stays 16 MiB; do not depend on `ManagedConfigService.connected`; do not do synchronous fsync store writes on every 30s `ping`; do not install a second `store.SetWriteObserver`.
- [ ] `plan/post-release/features/spec-fleet-0-umbrella.md` - Out of Scope row "Bidirectional config sync", and the Direction Update of 2026-11-19
  → Constraint: "full bidirectional sync stays out of scope"; this spec's sync runs hub to client only, and a local edit still holds for `fleet-7`'s adopt or revert (Design documents)
- [ ] `plan/release/features/spec-fleet-ui-5-tasks-alarms-notifications.md` - "Alarms": the alarm kind registry
  → Constraint: an alarm is derived from a fact the hub holds, never a stored event; a kind is a registered entry with condition, severity and sentence, so this spec registers Configuration resync failed and edits no list
- [ ] `plan/release/features/spec-fleet-ui-8-alarm-causes.md` - the Causes part of a kind's registration
  → Constraint: a kind names its own causes in its own registration; a cause must not be less severe than the kind it hides

**Key insights:**
- Hub already retains every committed version of `client-<name>.conf` under `file/<stamp>/client-<name>.conf` -> `sha256:<64-hex>` -> `object/<64-hex>` with no pruning (`storage/history_revision.go`), so any prior `VersionHash` can resolve its exact historical bytes on the hub.
- Today's `infra.DiffConfigs` is lossy/display-only (resolves BGP inheritance, masks secrets); `SetParser` only parses into a fresh `NewTree()` and lacks `activate` and `UserOrdered` reorder; `Serialize`/`SerializeSet` do NOT reproduce raw stored bytes when comments, format (`FormatSetMeta` vs hierarchical), list order, or freshly encoded `$9$` secrets differ.
- Today's `ping`, `config-ack`, `config-held`, and `config-changed` replies use empty `mc.SendOK`, and client `sendPing` discards the reply payload and sets no per-ping timeout.
- `pkg/zefs.KeyConfigActiveHash` (`meta/config/active-hash`) is legacy unmanaged-only code, unused by managed fleet; managed code uses `meta/managed/base-version` and `storage.ReadActiveConfigStamped`.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `pkg/fleet/envelope.go` - five verbs (`config-fetch`, `config-changed`, `config-ack`, `config-held`, `ping`) and JSON payloads; `HeldConfigMax = 1 MiB`, `HeldRefusedCode`.
- [ ] `pkg/fleet/version.go` - `VersionHash(content []byte) string`: first 8 bytes of SHA-256 as 16 lowercase hex chars over raw config bytes.
- [ ] `internal/component/plugin/server/managed.go` - `ManagedConfigService`: `HandleConfigFetch` compares `req.Version == VersionHash(data)` and returns `"current"` or full base64 config plus `HeldVersion`/`DiscardVersion`; `BuildConfigChanged` returns `ConfigChanged{Version}`.
- [ ] `internal/component/plugin/server/managed_serve.go` - `ManagedServer`: per-connection `managedSession` (`fetched`), `handleRequest` answers `config-fetch` (`SendResult`), `config-ack`/`config-held`/`ping` (`SendOK`); `NotifyConfigChanged`/`pushConfigChanged` coalesces notifications and closes connection on push timeout (`ManagedPushTimeout = 10s`).
- [ ] `internal/component/managed/client.go` - `RunManagedClient`/`runConnection`: auth, initial `fetchAndProcess`, 30s `sendPing` goroutine (discards reply, no per-call timeout) + `Heartbeat` (3 missed = 90s), `notificationLoop` handling `config-changed` (replies `SendOK` then fetches) and `ping` (`SendOK`); `fetchAgainstBaseline` -> `holdOrApply` / `applyReverted` -> `applyFetched` -> `CommitOver(data, activeCheck(decided))` -> `sendAck` -> `adoptEndpoint`.
- [ ] `internal/component/managed/baseline.go` - `ManagedFlag`, `ActiveVersion`, `BaselineVersion`, `Diverged`, `StoreBaseline`, `RecordActiveAsBaseline`, `BaselineAccess`.
- [ ] `internal/component/managed/handler.go` - `Handler.ValidateConfig`: base64-decodes `resp.Config` and runs `Handler.Validate(data)`.
- [ ] `internal/component/managed/cmd_divergence_show.go` - `divergenceDiff` uses `SchemaWithDeclaredPlugins` + `infra.DiffConfigs` (display-only, masked secrets, falls back to `diff-unavailable` on `unchecked-plugins`).
- [ ] `cmd/ze/hub/managed.go` & `cmd/ze/hub/managed_server.go` - `wireManagedCommit` (`confirm.WriteOutside` + `applyManagedPush` + `StageMachineCandidateOver` + `recordPushBaseline`); `startManagedServer` wires `ReadConfig` (`store.ReadFile(ClientConfigKey(name))`) and single `store.SetWriteObserver`.
- [ ] `internal/component/config/prune.go`, `serialize_set.go`, `setparser.go`, `secret_text.go`, `storage/pointer.go`, `storage/history_revision.go` - semantic tree comparison (`SubtreesEqual`/`PruneUnchanged`), `SerializeSet`/`DetectFormat`, `SetParser.Parse` (no public `ApplyTo`, no `activate`, additive leaf-lists), `$9$` secret preservation (`decodeSecretLeaf`/`EncodeSecrets`), guarded staging (`StageMachineCandidateOver`), unpruned dated version history (`versionsForName`/`ListVersions`/`ReadVersion`).
- [ ] `pkg/zefs/keys.go` - `KeyManagedBaseVersion` (`meta/managed/base-version`), `KeyConfigHeld` (`meta/config/{name}/held`), `KeyFileVersion`/`KeyObject`, legacy `KeyConfigActiveHash` (unmanaged only).

**Behavior to preserve:**
- `fleet-6` and `fleet-7` local-edit protection: local freeze, persisted `meta/managed/base-version`, reconnect hold when `Diverged(active, baseline)`, `config-held` upload, `show fleet divergence`, `adopt`, and `revert` (`discard-version`).
- Guarded staging (`CommitOver` -> `StageMachineCandidateOver` over `activeCheck(decided)`) and confirmed-commit exclusion (`confirm.WriteOutside`) on every hub-applied change, delta, diff-resync, and full replay.
- Per-client authentication, TLS trust model, reconnect backoff, endpoint adoption (`adoptEndpoint`), and zero overhead on standalone devices.
- Graceful tolerance of `unchecked-plugins` sections on the hub (never executes client external plugin binaries; falls back cleanly to full replay).

**Behavior to change:**
- Both hub and managed client fold every applied hub-driven config change into a persistent SHA-1 hash chain and exchange/compare the chain head on every managed channel reply.
- Normal hub-to-client updates and mismatch recovery attempt a diff-based delta/resync first, falling back to full configuration replay whenever diff construction, transmission, verification, or application fails or cannot cover the config.
- `show fleet status`, `show doctor`, and Prometheus metrics expose chain state, mismatches, diff resyncs, and full-replay fallbacks.
- When the diff resync and the full replay of a client both fail, the hub opens the alarm Configuration resync failed, a kind this spec registers through `fleet-ui-5`'s alarm kind registry, so the failure reaches the operator's alarm list and not only a doctor code. It clears when the chain heads agree again ("The Configuration resync failed alarm" in the Task section).

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Managed channel envelopes and verbs | `pkg/fleet/envelope.go` (`VerbConfigFetch`, `VerbConfigChanged`, `VerbConfigAck`, `VerbConfigHeld`, `VerbPing`) | Five JSON RPC verbs over newline-framed `rpc.MuxConn`; replies to `ping`, `config-ack`, `config-held`, `config-changed` are empty `ok` | Extends: adds optional `chain-hash` fields to replies and delta/resync fields (or verb) on the same framing |
| Config version hash | `pkg/fleet/version.go` (`VersionHash`) | Truncated SHA-256 (16 hex) over raw active config bytes, used by `config-fetch`, `base-version`, `config-held`, `adopt`, `revert` | Extends alongside: keeps `VersionHash` intact for `fleet-6`/`fleet-7` baseline, hold, and revert identity; adds SHA-1 hash chain (`ChainHash` / fold function) in `pkg/fleet` for continuous applied-change drift detection |
| Managed client sync loop | `internal/component/managed/client.go` (`RunManagedClient`, `fetchAgainstBaseline`, `applyFetched`, `notificationLoop`, `sendPing`) | Fetches full config on connect and `config-changed`, applies through `CommitOver`, sends `ping` every 30s discarding reply | Extends: compares chain head on every reply (`ping`, `config-changed`, `config-fetch`, `config-ack`, `config-held`), applies deltas/diff-resyncs, falls back to full replay, preserves hold when `Diverged(active, baseline)` |
| Client baseline and hold | `internal/component/managed/baseline.go` (`BaselineAccess`, `Diverged`, `RecordActiveAsBaseline`), `pkg/zefs/keys.go` (`KeyManagedBaseVersion`) | Persists `meta/managed/base-version` after each hub apply; holds if active config differs from baseline | Extends: keeps hold guard authoritative for local edits; persists client chain head beside `meta/managed/base-version` under a new `meta/managed/` key |
| Guarded commit and reload wiring | `cmd/ze/hub/managed.go` (`wireManagedCommit`, `applyManagedPush`), `internal/component/config/storage/pointer.go` (`StageMachineCandidateOver`) | Stages hub config under lock over `activeCheck(decided)` inside `confirm.WriteOutside`, reloads daemon, writes baseline | Extends: delta apply, diff resync, and full replay all stage and reload through this one guarded commit path |
| Hub managed server and session | `internal/component/plugin/server/managed_serve.go` (`ManagedServer`, `managedSession`, `handleRequest`, `pushConfigChanged`) and `managed.go` (`ManagedConfigService`) | Serves `config-fetch` (full config or `current`), handles `config-ack`/`config-held`/`ping`, pushes `config-changed` | Extends: tracks and persists per-client hub chain state, includes chain head on every reply, serves diff-based deltas/resyncs with full replay fallback |
| Versioned config object store | `internal/component/config/storage/history_revision.go` (`ListVersions`, `ReadVersion`), `pointer.go` (`ReadActiveConfig`) | Retains unpruned dated versions of `client-<name>.conf` pointing to SHA-256 content-addressed objects | Extends: hub locates the historical base config bytes `A` for a client's last known version to compute a diff to current config `B` |
| Config parsing, tree diff, and serialization | `internal/component/config/` (`PruneUnchanged`, `SubtreesEqual`, `SetParser`, `Serialize`, `SerializeSet`, `EncodeSecrets`, `SchemaWithDeclaredPlugins`) | Semantic tree diff, set-line parser/serializer, `$9$` secret preservation, external-plugin schema handling | Extends (or falls back to full replay on `unchecked-plugins`): provides schema-aware validation and (if tree-delta chosen) delta construction/application |
| Display divergence diff | `internal/component/config/infra/diff.go` (`DiffConfigs`), `internal/component/managed/cmd_divergence_show.go` | Operator-facing diff with resolved BGP inheritance and masked secrets | Does not apply for wire resync: lossy (flattens BGP groups, masks secrets) and cannot reconstruct a target configuration |
| Legacy `KeyConfigActiveHash` | `pkg/zefs/keys.go` (`meta/config/active-hash`), `cmd/ze/pushed_config.go` (`writeConfigActiveHash`) | Written once on unmanaged boot, never read in managed code | Does not apply: legacy unmanaged key; managed fleet uses `meta/managed/` keys |
| Fleet doctor and CLI status | `internal/component/managed/doctor.go`, `register_doctor_codes.go`, `cmd_fleet.go` (`show fleet status`), `internal/component/plugin/server/managed_doctor.go` | Reports managed client/hub health and `show fleet status` | Extends: surfaces chain head, sync/resync status, and doctor codes for drift/fallback/unreadable chain state |
| Alarm kind registry | `plan/release/features/spec-fleet-ui-5-tasks-alarms-notifications.md`, "Alarms"; Causes from `plan/release/features/spec-fleet-ui-8-alarm-causes.md` | Planned, not built: each alarm kind is a registered entry with its condition, severity and sentence, derived from what the hub holds | Extends: registers one kind, Configuration resync failed, with Device offline as its cause; no second alarm mechanism |
| Alarm tab registry | `internal/component/web/fleetui/handler_alarms.go` `mustRegisterAlarmTab` | Maps an alarm kind to the device tab where it is examined | Extends: `config-resync-failed` registers the Config tab from this spec's own register file |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `docs/architecture/fleet-config.md` | Non-Goals | The `// Design:` page of `pkg/fleet/envelope.go` and `internal/component/managed/client.go` | "Full config on change; configs are small (< 10KB typically)" | departs | This spec sends a diff first and a full replay only as fallback, on a hub change and on a chain mismatch. Presented to Thomas at design approval; the row is corrected in this work once he agrees |
| `plan/post-release/features/spec-fleet-0-umbrella.md` | Out of Scope, row "Bidirectional config sync" | The umbrella of this spec's set | "Continuous two-way merge. Replaced for now by single-writer plus an operator-gated `config-push` at divergence (`fleet-7`); full bidirectional sync stays out of scope" | agrees | The sync is continuous but one way: the hub's configuration repairs a client that missed or diverged from a hub change. A client's own change never flows to the hub by it: a local edit (`active != baseline`) still holds, and only `fleet-7`'s operator-gated adopt brings it up. Shown to Thomas beside the departure above, because "continuous sync" reads close to the row |
| `plan/release/features/spec-fleet-ui-5-tasks-alarms-notifications.md` | Alarms | The alarm list this spec's alarm joins | "an alarm kind is a registered entry with its condition, its severity and its sentence, so a later feature adds one without editing the list" | agrees | Configuration resync failed is registered from this spec's package, derived from the chain state the hub already holds |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- [pending research]

### Transformation Path
1. [pending research]

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Hub ↔ Managed Client | TLS + MuxConn RPC | No |

### Changed-Wire Census

**Search evidence:** [pending research]

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| [format] | [role] | [source site] | [search finding] | [change or preserve] |

### Integration Points
- [pending research]

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
| A-1 | Hub retains past versions of `client-<name>.conf` under `file/<stamp>/client-<name>.conf` -> `object/<sha256>` with no automatic pruning | Verified in `internal/component/config/storage/history_revision.go` (`versionsForName`) and `pointer.go` (`removeVersionLocked` only removes failed candidates) | Hub could not reconstruct base bytes `A` for a client's prior version and would always fall back to full replay | Unit test resolving a historical `VersionHash` via `ListVersions` / `ReadVersion` | unvalidated |
| A-2 | Every managed client config apply goes through `CommitOver` -> `StageMachineCandidateOver` over `activeCheck(decided)` and updates `meta/managed/base-version` from the stored active bytes | Verified in `cmd/ze/hub/managed.go` (`wireManagedCommit`, `applyManagedPush`) and `internal/component/managed/baseline.go` (`RecordActiveAsBaseline`) | Delta or resync could silently overwrite a concurrent local edit or leave `VersionHash` out of sync with `base-version` | Unit test racing a local commit against delta apply and diff resync | unvalidated |
| A-3 | Delta application must produce active config bytes whose `fleet.VersionHash` matches the hub's served `client-<name>.conf` bytes (or canonical agreement must be established on both ends) | Verified in `internal/component/plugin/server/managed.go` (`HandleConfigFetch` compares `req.Version == fleet.VersionHash(data)`) and `baseline.go` (`ActiveVersion`) | Next `config-fetch` or `fleet-7` `adopt`/`revert` sees a false version mismatch and loops or falsely holds | Round-trip test asserting `fleet.VersionHash(reconstructed) == fleet.VersionHash(hubTarget)` across hierarchical, set-meta, comments, and `$9$` secrets | unvalidated |
| A-4 | Hub cannot parse external-plugin config sections for plugins the hub itself did not start (`unchecked-plugins`) | Verified in `internal/component/managed/cmd_divergence_show.go` (`divergenceParseRefusal`) and `ai/doors/config-yang.md` | A YANG-tree diff path would fail permanently for clients with external plugins unless full replay fallback handles it | Unit test with an unknown external-plugin root proving fallback to full replay | unvalidated |
| A-5 | `rpc.MuxConn` callers can upgrade empty `SendOK` replies (`ping`, `config-ack`, `config-held`, `config-changed`) to `SendResult` JSON replies with optional `chain-hash` while tolerating empty replies from older peers | Verified in `pkg/plugin/rpc/mux.go` (`SendOK` vs `SendResult`, `CallRPC` returning `json.RawMessage`) and `docs/architecture/fleet-config.md` ("RPCs") | Older peer or test fixture expecting empty reply could fail decoding | Unit test verifying both new JSON replies and legacy empty `ok` replies decode cleanly | unvalidated |
| A-6 | `fleet-1` (`plan/release/features/spec-fleet-1-device-registry.md`, Status: `ready`) will lower hub managed read bound to ~1.4 MiB and target 2,000+ connected clients | Verified in `plan/release/features/spec-fleet-1-device-registry.md` (lines 47, 152, 230, 286) | Large client-to-hub payloads or per-ping synchronous fsyncs would break scale or hit the lowered read bound | Unit test checking all client→hub messages stay small and `ping` performs zero store writes | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Delta representation fails to reproduce byte-identical `VersionHash(B)` due to format (`FormatSetMeta` vs hierarchical), comments, non-`UserOrdered` list order, or random-salt `$9$` secrets | Post-apply `VersionHash` check mismatches target hash | Verify target `VersionHash` before staging; fall back to full replay on any hash mismatch, and choose a delta format at Design Gate that guarantees byte-exact reconstruction |
| R-2 | Hub and client chains diverge permanently across a crash between commit and chain-head persistence | Chain mismatch after restart | Mismatch triggers diff-based resync (re-anchoring the chain), then full replay as fallback (which resets both ends to the same chain anchor) |
| R-3 | Client with a local emergency edit (`Diverged(active, baseline)`) gets overwritten by mismatch auto-resync | Local edit lost on next ping or reply | Check `Diverged(active, baseline)` before any auto-resync; a locally diverged client stays in `fleet-6`/`fleet-7` hold and never auto-resyncs |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | [pending design] |
| How is it reverted? | [pending design] |
| Who else touches this path? | [pending design] |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| [pending design] | → | [pending design] | [pending design] |
| A client whose diff resync and full replay both fail, then `show fleet alarm` on the hub | → | The registered `config-resync-failed` kind derives its alarm from the hub's per-client chain state | `test/managed/fleet-resync-failed-alarm.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A connected client that holds no local edit, whose diff resync and full replay both fail (a diff that cannot be built or verified, a replay that is not delivered or whose applied bytes do not reach the hub's chain head) | The hub opens one Configuration resync failed alarm on that device, Warning, with a sentence naming why each step failed; it is in `show fleet alarm` and the fleet alarm list. When a later resync, replay or fetch makes the chain heads agree, it clears | A replay the client refused at verify opens Configuration rejected instead, because the client answered and `fleet-ui-5` owns that alarm. A client that holds opens none, because no resync runs for it and Configuration diverged says why |
| AC-2 | The device of AC-1 goes offline while the alarm is open | The alarm is hidden behind Device offline (`fleet-ui-8`), and on reconnect it is shown again only if the reconnect's resync fails too | Its history row remains, because the history is the record that it happened. Owed only under choice C-1 (a) |
| AC-3 | [pending design: the chain, delta, resync and replay criteria] | [pending design] | [pending design] |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | [pending design] | [pending design] | [pending design] |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestXxx` | `pkg/fleet/..._test.go` | [pending design] | |
| `TestResyncFailedAlarmOpensAndClears` | [pending design: the package of the hub's per-client chain state] | AC-1: opens when both steps fail on a client that holds nothing; not when the client refused the replay at verify, nor when it holds; clears when the chain heads agree | |
| `TestResyncFailedAlarmHiddenBehindOffline` | [pending design: same package], with the alarm registry of `fleet-ui-5` and the causes of `fleet-ui-8` | AC-2: the kind's registration names Device offline, and the registry's refusal checks pass for it | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| [pending design] | [range] | [last valid] | [below] | [above] |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `fleet-drift-hash-chain` | `test/managed/fleet-drift-hash-chain.ci` | [pending design] | |
| `fleet-resync-failed-alarm` | `test/managed/fleet-resync-failed-alarm.ci` | A real hub and client where both resync steps are made to fail: `show fleet alarm` on the hub lists Configuration resync failed for the client, and after the fault is removed and a resync succeeds it is gone. Red when the failure reaches only the doctor codes | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | N-A | None (Ze-to-Ze managed protocol) | Managed channel is Ze's own hub-to-client protocol; no external peer implementation exists | N-A |

## Files to Modify
- [pending design]

## Files to Create
- [pending design]

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | | |
| YANG validation constraints | | |
| YANG custom validators | | |
| CLI commands/flags | | |
| CLI grammar (keyword before value) | | |
| Editor autocomplete | | |
| Functional test for new RPC/API | | |
| Pipe completeness | | |
| Env var registration | | |
| Doctor check for runtime dependencies | | |
| Doctor code per problem class | | |
| Prometheus counters/metrics | | |
| BGP family surface (new SAFI / capability / attribute) | N-A | Fleet managed config protocol; no BGP SAFI, capability or attribute |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | | |
| 2 | Config syntax changed? | | |
| 3 | CLI command added/changed? | | |
| 4 | API/RPC added/changed? | | |
| 5 | Plugin added/changed? | | |
| 6 | Has a user guide page? | | |
| 7 | Wire format changed? | | |
| 8 | Plugin SDK/protocol changed? | | |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | Ze's own managed protocol; no RFC |
| 10 | Test infrastructure changed? | | |
| 11 | Affects daemon comparison? | | |
| 12 | Internal architecture changed? | | |
| 13 | Route metadata keys added/changed? | N-A | No route metadata keys |
| 14 | Prometheus counters added/changed? | | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | | |
| 16 | Any changed source file referenced by existing doc source anchors? | | |
| 17 | Existing docs show config/CLI/API examples for this area? | | |

## Design presentation

### What changes

[pending design]

### Before and after

[pending design]

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| [pending design] | [pending design] |

### Documents departed from

- `docs/architecture/fleet-config.md`, "Non-Goals", the row "Incremental config updates": "Full config on change; configs are small (< 10KB typically)". After this work the row is removed and "Protocol" describes the hash chain, the diff resync and the full replay fallback. [pending design: the before and after the operator sees]
- Not a departure, presented with it: `plan/post-release/features/spec-fleet-0-umbrella.md`, Out of Scope, "Bidirectional config sync". The sync this spec adds runs hub to client only, and a local edit still holds (Design documents).

### Choices for Thomas

| # | Choice | Options | Decision |
|---|--------|---------|----------|
| C-1 | Does Configuration resync failed name Device offline as its cause? | (a) recommended, Yes: the kind declares the cause in its own registration, so this spec depends on `fleet-ui-8` for the Causes part, and while a device is offline the operator reads one alarm, since its reconnect starts a new resync; (b) No: the kind names no cause, the dependency on `fleet-ui-8` is dropped, and the alarm stays listed beside Device offline | |
| C-2 | [pending design] | [pending design] | |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- [pending design]
   - Tests: `test/managed/fleet-drift-hash-chain.ci`, `test/managed/fleet-resync-failed-alarm.ci`
   - Verify: `./le test managed` runs the managed functional tests named above, red until the chain is compared and the alarm is registered

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| [pending design] | [pending design] |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | [pending design] |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| A resync that fails both ways opens an alarm kind this spec registers | Doctor codes and `show fleet status` alone | Neither reaches an operator who does not ask; the alarm list of `fleet-ui-5` is where the estate says what is wrong, and a registered kind needs no edit outside this spec |

## Known Limitations
- [pending design]

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
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test (N-A when Scope is tooling or docs, which delete that section)
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
