# Spec: fleet-10 -- Drift Detection by Hash Chain

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol, plugin, config, cli |
| Priority | P2 |
| Release reason | Thomas put this fleet capability (configuration drift detection by SHA-1 hash chain, repaired by the hub with a diff resync and a full replay fallback, reached through commands on every command surface per `ai/META-DECISIONS.md`) in the first release; existing first-release managed behavior and the fleet-6/fleet-7 hold guarantees remain required. |
| RFC gaps | - |
| Depends | plan/release/features/spec-fleet-1-device-registry.md, spec-fleet-7-config-reconnect-resolution (closed 2026-11-19: the hold, adopt and revert this spec preserves, documented in `docs/architecture/fleet-config.md`, "The baseline and the hold"), plan/release/features/spec-fleet-ui-5-alarm-list.md, plan/release/features/spec-fleet-ui-8-alarm-causes.md |
| Phase | 10/10 of plan/post-release/features/spec-fleet-0-umbrella.md |
| Handoff | - |
| Design tier | strongest tier |
| Implementation tier | development tier |
| Updated | 2026-11-23 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Add continuous drift detection by SHA-1 hash chain for managed fleet configuration deltas. Both the hub and each managed client fold every applied hub-driven configuration change into a persistent SHA-1 hash chain and exchange and compare the chain head on every reply over the managed TLS channel (`ping`, `config-fetch`, `config-ack`, `config-changed`, `config-held`).

Today a managed client detects hub updates only on reconnect or when a hub `config-changed` notification arrives, and every update transfers the entire configuration blob (`docs/architecture/fleet-config.md`). If a notification is dropped, an out-of-band hub store write occurs, an acknowledgement is lost mid-flight, or applied change history diverges, the two sides remain unaware until the next reconnect or push.

This spec adds:
1. A deterministic SHA-1 hash chain in `pkg/fleet/` that folds every applied hub-driven change (re-anchoring from zero on full replay, folding the base version, target version, and delta payload on every diff apply).
2. Deterministic line-based transport diff/patch in `pkg/fleet/` computed on the hub from its unpruned versioned configuration store (`file/<stamp>/client-<name>.conf` -> `object/<sha256>`), reconstructing byte-identical target configurations so `fleet.VersionHash`, `fleet-6` baseline/hold, `fleet-7` divergence/adopt/revert, `$9$` secrets, comments, `FormatSetMeta`, and `unchecked-plugins` external sections all remain intact, followed by full YANG validation (`Handler.Validate`) and guarded staging (`CommitOver` -> `StageMachineCandidateOver`) before commit.
3. Optional `chain-hash` comparison on every managed channel reply: a mismatch on a non-holding device triggers a diff-based resync first (with `reanchor` when base bytes match but chain heads drifted) and a bounded single full-replay fallback if diff construction, transmission, verification, or validation fails.
4. Strict preservation of `fleet-6`/`fleet-7`: whenever `Diverged(active, baseline)` holds on the client (a local emergency edit) or a local commit races during a fetch (`errActiveChanged`), the client never auto-resyncs and stays in hold for operator `adopt` or `revert`.
5. Persistence of chain state on both ends (`meta/managed/chain-state` on the client, `meta/managed/client/{name}/chain` on the hub) written only when a configuration change is applied (zero store writes on 30-second heartbeats), plus observability in `show fleet status`, `ze doctor`, Prometheus metrics, and the hub `show fleet alarm` list (`fleet-ui-5` / `fleet-ui-8`). Per Thomas's 2026-11-23 ruling (`ai/META-DECISIONS.md`, "Goal and scope"), first-release fleet features ship through commands on every command surface; web pages (`fleet-ui-9`) come after the first release.

### Scope Confirmed at Gates (2026-11-23)

| Gate | Confirmed decision |
|------|--------------------|
| SCOPE | Managed config deltas folded into a SHA-1 chain, compared on every reply, diff-based resync first and full config replay fallback |
| SCOPE | Preserve `fleet-6`/`fleet-7` hold: auto-resync repairs missed/diverged hub changes, while a local device edit (`active != baseline`) still holds for operator `adopt`/`revert` |
| SCOPE | Hash algorithm: SHA-1 chain as requested (non-adversarial sync checksum over authenticated TLS 1.3) |
| SCOPE / First release | Canonical path `plan/release/features/spec-fleet-10-drift-hash-chain.md` (P2, phase 10/10 of `spec-fleet-0-umbrella.md`), moved when Thomas put drift detection with hub repair into the first release (`4d5236a783`, `7af7e45c4d`, `e54bba27d9`) |
| RESEARCH | Confirmed existing mechanisms, assumptions A-1 through A-6, and byte-exact `VersionHash` vs YANG-tree delta tension |
| DESIGN | Approved Approach A: deterministic line-based transport diff in `pkg/fleet/` with full YANG validation and guarded commit on the client, extending existing managed verbs with optional fields (no new RPC verbs) |

### The Configuration resync failed alarm

Added 2026-11-23, after the scope gate above. When the diff resync and the full
replay both fail, the doctor codes and `show fleet status` say so only to an
operator who asks; the alarm list of `fleet-ui-5` (`show fleet alarm`, on every
command surface in the first release) is where the estate tells the operator
what is wrong. `fleet-ui-5` derives each alarm from a fact the hub holds, and
an alarm kind is a registered entry with its condition, its severity and its
sentence, with no central list. This spec registers one kind from its own
package.

| Part | Value |
|------|-------|
| Name | `config-resync-failed`, shown as Configuration resync failed |
| Condition | The last resync of a client that holds no local edit ended with both the diff resync and the full replay failed (non-verify delivery, decode, hash, or commit failure), and the hub's expected chain head for the client still differs from the client's last reported head. Read from this spec's per-client chain state on the hub, so nothing new is stored for the alarm |
| Not this alarm | A full replay the client refused at verify is Configuration rejected (`fleet-ui-5`). A client whose active configuration differs from its baseline holds, and is Configuration diverged: no resync runs for it, so this alarm clears |
| Clears | The chain heads agree again: a later diff resync, full replay or fetch succeeded |
| Severity | Warning, as Configuration rejected: the device runs a known configuration, not the hub's latest |
| Sentence | Names the device, the UTC timestamp of the failed attempt, the bounded reason each of the two steps failed (diff step cause and full-replay step cause, carrying hashes/codes only and never config text), and ends on the fix `ze show doctor` (with code `doctor-managed-chain-drift`), following `ai/rules/user-facing-errors.md` |
| Causes (`fleet-ui-8`) | Device offline. No resync runs while the device is offline, and its reconnect starts one, so the alarm is hidden behind the offline alarm rather than listed beside it (choice C-1) |
| Web device tab (post-release) | `fleet-ui-9` owns the post-release alarm web pages (`ai/META-DECISIONS.md`, 2026-11-23); when built with `ze_web && ze_fleet_ui`, `config-resync-failed` registers the Config tab via `mustRegisterAlarmTab` from a build-gated register file |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/hub-engine.md`, `ai/doors/config-yang.md`, `ai/doors/plugin-infra.md`, `ai/doors/commands-cli.md`, `ai/doors/core.md`, `ai/doors/le-tooling.md` - subsystem boundaries, public `pkg/` contracts, persistence, and test rules
  → Constraint: `pkg/fleet` is public API; wire changes must be intentional and documented in `docs/architecture/fleet-config.md` in the same work.
  → Constraint: runtime state persists only through `storage.Storage` / `statestore` under keys registered in `pkg/zefs/keys.go` (`MustRegister`), never raw files, and never nested beneath an existing value key (`meta/instance/managed`, `meta/fleet/device/{name}/record`).
  → Decision: the line-based transport diff reconstructs exact stored configuration bytes without parsing or mutating config syntax, and the client runs full YANG validation (`Handler.Validate`) and guarded commit (`CommitOver`) before any change takes effect, keeping `ai/rules/config.md` config manipulation rules intact.
- [ ] `docs/architecture/fleet-config.md` - managed protocol, `VersionHash`, baseline/hold/revert, non-goals
  → Constraint: `## Non-Goals` currently excludes incremental config updates ("Full config on change; configs are small (< 10KB typically)") — this spec departs from that row and presents the departure to the owner.
  → Constraint: client baseline (`meta/managed/base-version`) is always read back from stored active bytes (`RecordActiveAsBaseline`), and every hub apply must pass `CommitOver` -> `StageMachineCandidateOver` over `activeCheck(decided)` inside `confirm.WriteOutside`.
  → Constraint: every loop-blocking client RPC must carry an explicit timeout; loop round-trips other than full `config-fetch` (30s) must stay below `ManagedPushTimeout` (10s).
- [ ] `docs/architecture/api/commands.md` - `## Design decisions`, local-data commands, answer shapes, pipe operators
  → Constraint: command paths name the subject and stop; scope, presentation and format stay pipe operations. `show fleet status` adds document fields only and keeps its `ShapeDoc` contract and column registration.
- [ ] `docs/architecture/zefs-format.md` - registered key families, single-process ownership, runtime vs config keys
  → Constraint: hub-side chain keys must not match `client-*.conf` basenames (`ClientNameFromConfigKey`), or the store write observer falsely triggers `NotifyConfigChanged`.
- [ ] `docs/architecture/doctor-and-health-checks.md` - doctor checks, diagnostic codes, runtime vs post-config phases
  → Constraint: every `Warn` or `Error` log line and report-bus event must carry a registered doctor code under attribute `code`, with a registered doctor check whose fix names the exact remediation command.
- [ ] `docs/architecture/config/syntax.md`, `docs/architecture/config/yang-config-design.md` - config formats, `$9$` secrets, external-plugin schemas
  → Constraint: a hub never starts a client's external plugin binary (`SchemaWithDeclaredPlugins`), so hub-side YANG parsing fails on `unchecked-plugins` sections; transport-level line diffing works transparently across all formats (`FormatHierarchical`, `FormatSet`, `FormatSetMeta`), comments, `$9$` secrets, and `unchecked-plugins`.
- [ ] `plan/release/features/spec-fleet-1-device-registry.md` - ready sibling spec lifting the 128-client cap to 2,000+ devices
  → Constraint: hub-side read limit drops to ~1.4 MiB (`base64(HeldConfigMax) + 64 KiB`), while client-side read stays 16 MiB; do not depend on `ManagedConfigService.connected`; do not do synchronous fsync store writes on every 30s `ping`; do not install a second `store.SetWriteObserver`.
- [ ] `plan/post-release/features/spec-fleet-0-umbrella.md` - Out of Scope row "Bidirectional config sync", and the Direction Update of 2026-11-19
  → Constraint: "full bidirectional sync stays out of scope"; this spec's sync runs hub to client only, and a local edit still holds for `fleet-7`'s adopt or revert.
- [ ] `plan/release/features/spec-fleet-ui-5-alarm-list.md` - "Alarms": the alarm kind registry and `show fleet alarm`
  → Constraint: an alarm is derived from a fact the hub holds, never a stored event; a kind is a registered entry with condition, severity and sentence, so this spec registers Configuration resync failed and edits no central list.
- [ ] `plan/release/features/spec-fleet-ui-8-alarm-causes.md` - the Causes part of a kind's registration
  → Constraint: a kind names its own causes in its own registration; a cause must not be less severe than the kind it hides (`Device offline` is Critical >= Warning).

**Key insights:**
- Hub already retains every committed version of `client-<name>.conf` under `file/<stamp>/client-<name>.conf` -> `sha256:<64-hex>` -> `object/<64-hex>` with no automatic pruning (`storage/history_revision.go`), enabling historical base lookup by `VersionHash` prefix with zero per-client config caching in hub RAM.
- All replies other than `config-fetch` (`ping`, `config-ack`, `config-held`, `config-changed`) currently send empty `mc.SendOK`; upgrading them to `mc.SendResult` with `ManagedReply{ChainHash}` (while tolerating empty replies from older peers) enables continuous drift checks on every reply without new RPC verbs.
- `sendPing` runs in a separate goroutine (`client.go:477`) and currently discards replies and has no per-call timeout; adding `pingTimeout` (10s) and signalling `notificationLoop` via a 1-buffered `driftCh` prevents concurrent apply races.
- `pkg/zefs.KeyConfigActiveHash` (`meta/config/active-hash`) is legacy unmanaged-only code, unused by managed fleet.
- Hub per-client chain state already holds the exact facts (`lastDiffError`, `lastFullError`, `lastFailedAt`, `lastClientChain`, `expectedChain`) needed to derive `config-resync-failed` without storing a separate alarm table.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `pkg/fleet/envelope.go` - five verbs (`config-fetch`, `config-changed`, `config-ack`, `config-held`, `ping`) and JSON payloads; `HeldConfigMax = 1 MiB`, `HeldRefusedCode`.
- [ ] `pkg/fleet/version.go` - `VersionHash(content []byte) string`: first 8 bytes of SHA-256 as 16 lowercase hex chars over raw config bytes.
- [ ] `internal/component/plugin/server/managed.go` - `ManagedConfigService`: `HandleConfigFetch` compares `req.Version == VersionHash(data)` and returns `"current"` or full base64 config plus `HeldVersion`/`DiscardVersion`; `BuildConfigChanged` returns `ConfigChanged{Version}`.
- [ ] `internal/component/plugin/server/managed_serve.go` - `ManagedServer`: per-connection `managedSession` (`fetched`), `handleRequest` answers `config-fetch` (`SendResult`), `config-ack`/`config-held`/`ping` (`SendOK`); `NotifyConfigChanged`/`pushConfigChanged` coalesces notifications and closes connection on push timeout (`ManagedPushTimeout = 10s`).
- [ ] `internal/component/plugin/server/managed_doctor.go`, `doctor_codes.go`, `register.go`, `internal/component/plugin/doctor/check_managed_runtime.go`, `register_external.go` - `managedDoctorState`, `ManagedDoctorSnapshot`, doctor code metadata (`doctorDiagnosticCodes`), event-code registration (`serverEventCodes`), runtime doctor checks (`managed-listener-unbound`, `managed-client-rejected`).
- [ ] `internal/component/managed/client.go` - `RunManagedClient`/`runConnection`: auth, initial `fetchAndProcess`, 30s `sendPing` goroutine (discards reply, no timeout, currently unjoined on return) + `Heartbeat` (3 missed = 90s), `notificationLoop` handling `config-changed` (`SendOK` then fetch) and `ping` (`SendOK`); `fetchAgainstBaseline` -> `holdOrApply` / `applyReverted` -> `applyFetched` -> `CommitOver(data, activeCheck(decided))` -> `sendAck` -> `adoptEndpoint`.
- [ ] `internal/component/managed/baseline.go` - `ManagedFlag`, `ActiveVersion`, `BaselineVersion`, `Diverged`, `StoreBaseline`, `RecordActiveAsBaseline`, `BaselineAccess`.
- [ ] `internal/component/managed/handler.go` - `Handler.ValidateConfig`: base64-decodes `resp.Config` and runs `Handler.Validate(data)`.
- [ ] `internal/component/managed/cmd_fleet.go`, `register.go`, `doctor.go`, `register_doctor_codes.go` - `show fleet status` (`fleetStatus`, `ClientState`, column order), managed doctor checks and registered codes.
- [ ] `cmd/ze/hub/managed.go`, `cmd/ze/hub/managed_server.go`, `cmd/ze/hub/managed_owner.go`, `cmd/ze/ze_core_start.go` - `wireManagedCommit` (`confirm.WriteOutside` + `applyManagedPush` + `StageMachineCandidateOver` + `recordPushBaseline`), `startManagedServer` (single `SetWriteObserver`), `managedClientOwner`, `fetchInitialConfig`/`cacheFirstConfig`.
- [ ] `internal/component/config/storage/pointer.go`, `internal/component/config/storage/history_revision.go` - guarded staging (`StageMachineCandidateOver`), unpruned dated version history (`versionsForName`/`ListVersions`/`ReadVersion`).
- [ ] `pkg/zefs/keys.go` - `KeyManagedBaseVersion` (`meta/managed/base-version`), `KeyConfigHeld` (`meta/config/{name}/held`), `KeyFileVersion`/`KeyObject`.

**Behavior to preserve:**
- `fleet-6` and `fleet-7` local-edit protection: local freeze, persisted `meta/managed/base-version`, reconnect hold when `Diverged(active, baseline)`, `config-held` upload, `show fleet divergence`, `adopt`, and `revert` (`discard-version`).
- Guarded staging (`CommitOver` -> `StageMachineCandidateOver` over `activeCheck(decided)`) and confirmed-commit exclusion (`confirm.WriteOutside`) on every hub-applied change, delta, diff-resync, and full replay.
- Per-client authentication, TLS trust model, reconnect backoff, endpoint adoption (`adoptEndpoint`), and zero overhead on standalone devices.
- Graceful handling of `unchecked-plugins` sections on the hub without running client external plugin binaries.
- Existing `show fleet status` fields and JSON keys (additive fields only), and existing Prometheus metrics.

**Behavior to change:**
- Both hub and managed client fold every applied hub-driven config change into a persistent SHA-1 hash chain and exchange/compare the chain head on every managed channel reply.
- Normal hub-to-client config updates and chain-mismatch recoveries attempt a deterministic line-based diff first, falling back to full configuration replay whenever base history is unavailable, diff size meets or exceeds full size, or diff verification/application/validation fails.
- Client `sendPing` enforces a 10-second per-ping timeout and forwards chain mismatches to `notificationLoop` via a non-blocking `driftCh`.
- `show fleet status`, `ze doctor` / `show doctor`, and Prometheus metrics expose chain state, mismatches, diff resyncs, and full-replay fallbacks.
- When both diff resync and full replay fail for a non-holding client (outside a client verify refusal, which opens `Configuration rejected`), the hub derives and opens the `config-resync-failed` (`Configuration resync failed`) alarm in `show fleet alarm` (`fleet-ui-5`), declaring `Device offline` as its cause (`fleet-ui-8`), and clears it when the chain heads agree again.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Managed channel envelopes and verbs | `pkg/fleet/envelope.go` (`VerbConfigFetch`, `VerbConfigChanged`, `VerbConfigAck`, `VerbConfigHeld`, `VerbPing`) | Five JSON RPC verbs over newline-framed `rpc.MuxConn`; replies to `ping`, `config-ack`, `config-held`, `config-changed` are empty `ok` | Extends: adds optional `chain-hash` to all requests/replies and delta/resync fields to `ConfigFetchRequest`/`ConfigFetchResponse` on the same framing with zero new verbs |
| Config version hash | `pkg/fleet/version.go` (`VersionHash`) | Truncated SHA-256 (16 hex) over raw active config bytes, used by `config-fetch`, `base-version`, `config-held`, `adopt`, `revert` | Extends alongside: keeps `VersionHash` intact for `fleet-6`/`fleet-7`; adds `ChainHash` (40-hex SHA-1 fold) and deterministic line diff/patch in `pkg/fleet/` |
| Managed client sync loop | `internal/component/managed/client.go` (`RunManagedClient`, `fetchAgainstBaseline`, `applyFetched`, `notificationLoop`, `sendPing`) | Fetches full config on connect and `config-changed`, applies through `CommitOver`, sends 30s `ping` discarding reply | Extends: compares chain head on every reply, signals `notificationLoop` via `driftCh`, applies diffs with bounded full-replay fallback, preserves hold when `Diverged(active, baseline)` |
| Client baseline and hold | `internal/component/managed/baseline.go` (`BaselineAccess`, `Diverged`, `RecordActiveAsBaseline`), `pkg/zefs/keys.go` (`KeyManagedBaseVersion`) | Persists `meta/managed/base-version` after each hub apply; holds if active config differs from baseline | Extends: keeps hold guard authoritative for local edits; adds `ChainAccess` persisting `meta/managed/chain-state` beside `meta/managed/base-version` |
| Guarded commit and reload wiring | `cmd/ze/hub/managed.go` (`wireManagedCommit`, `applyManagedPush`), `internal/component/config/storage/pointer.go` (`StageMachineCandidateOver`) | Stages hub config under lock over `activeCheck(decided)` inside `confirm.WriteOutside`, reloads daemon, writes baseline | Extends: delta apply, diff resync, and full replay all stage and reload through this one guarded commit path; first boot (`cacheFirstConfig`) also writes initial chain anchor |
| Hub managed server and session | `internal/component/plugin/server/managed_serve.go` (`ManagedServer`, `managedSession`, `handleRequest`, `pushConfigChanged`) and `managed.go` (`ManagedConfigService`) | Serves `config-fetch` (full config or `current`), handles `config-ack`/`config-held`/`ping`, pushes `config-changed` | Extends: tracks and persists per-client hub chain state (`meta/managed/client/{name}/chain`), replies with expected `chain-hash` on every RPC, serves diffs with full replay fallback |
| Versioned config object store | `internal/component/config/storage/history_revision.go` (`ListVersions`, `ReadVersion`), `pointer.go` (`ReadActiveConfig`) | Retains unpruned dated versions of `client-<name>.conf` pointing to SHA-256 content-addressed objects | Extends: hub resolves historical base bytes `A` for a client's reported `VersionHash` to compute `Diff(A, B)` without a new snapshot store or in-RAM cache |
| Config validator | `internal/component/managed/handler.go` (`Handler.ValidateConfig`), `cmd/ze/ze_core_start.go` (`config.ReadConfig`) | Validates decoded config bytes against full YANG schema before commit | Extends: validates reconstructed bytes after diff patch before staging; failure triggers full-replay fallback |
| Display divergence diff | `internal/component/config/infra/diff.go` (`DiffConfigs`), `internal/component/managed/cmd_divergence_show.go` | Operator-facing diff with resolved BGP inheritance and masked secrets | Does not apply for wire resync: lossy (flattens BGP groups, masks secrets) and fails on `unchecked-plugins` |
| Legacy `KeyConfigActiveHash` | `pkg/zefs/keys.go` (`meta/config/active-hash`), `cmd/ze/pushed_config.go` (`writeConfigActiveHash`) | Written once on unmanaged boot, never read in managed code | Does not apply: legacy unmanaged key; managed fleet uses `meta/managed/` keys |
| Fleet status view | `internal/component/managed/cmd_fleet.go`, `register.go` | Reports `show fleet status` (`fleetStatus` `ShapeDoc` + `ColumnOrder`) | Extends: adds `chain-hash`, `chain-state`, `last-resync` fields and columns |
| Report bus, doctor codes, and event-code registry | `internal/core/report/report.go` (`RaiseWarningFix`, `ClearWarning`), `internal/core/diagnostic/registry.go` (`Register`, `RegisterEventCode`), `internal/component/plugin/server/doctor_codes.go` (`doctorDiagnosticCodes`), `register.go` (`serverEventCodes`), `managed_doctor.go`, `internal/component/plugin/doctor/register_external.go`, `check_managed_runtime.go`, `internal/component/managed/doctor.go`, `register.go` | Raises state-based warnings with fixes (`show warnings`), enforces unique `diagnostic.Register` codes (`errDuplicateCode`), registers report-bus event codes, and wires hub/client doctor checks | Extends: declares and registers `CodeDoctorManagedChainDrift` and `CodeDoctorManagedChainUnreadable` once in `internal/component/plugin/server/doctor_codes.go` (referenced by `plugin/server`, `plugin/doctor`, and `managed`, avoiding duplicate `diagnostic.Register` panics), adds `CodeDoctorManagedChainDrift` to `serverEventCodes` in `plugin/server/register.go`, registers hub runtime checks in `plugin/doctor/register_external.go` + `check_managed_runtime.go`, registers client `managed-chain` check in `managed/register.go` + `doctor.go`, and raises/clears transient drift warnings via `report.RaiseWarningFix` / `report.ClearWarning` |
| Daemon Prometheus metrics registry | `internal/component/plugin/registry` (`GetMetricsRegistry`), `cmd/ze/hub/managed_server.go` (`ManagedServerConfig.Metrics`), `cmd/ze/ze_core_start.go` (`managedClientConfig`), `docs/plugin-development/metrics.md` (`Naming Policy`) | Provides the daemon's `*metrics.Registry`, already injected into `ManagedServerConfig.Metrics` on the hub, while `managed.ClientConfig` currently carries no metrics registry | Extends: adds optional `Metrics *metrics.Registry` to `managed.ClientConfig`, wired from `registry.GetMetricsRegistry()` in `cmd/ze/ze_core_start.go` (nil-safe in tests), registering hub counters (`ze_managed_chain_mismatches_total`, `ze_managed_chain_resyncs_total`) and client counters (`ze_managed_client_chain_mismatches_total`, `ze_managed_client_resyncs_total`) on the same registry |
| Alarm kind registry | `plan/release/features/spec-fleet-ui-5-alarm-list.md`, "Alarms"; Causes from `plan/release/features/spec-fleet-ui-8-alarm-causes.md` | Planned in first release (commands-only): each alarm kind is a registered entry with its condition, severity, causes, and sentence, derived from what the hub holds | Extends: registers `config-resync-failed` (`Configuration resync failed`) with `Device offline` as its cause, derived from hub per-client chain state; no second alarm mechanism |
| Alarm tab registry (post-release web) | `internal/component/web/fleetui/handler_alarms.go` `mustRegisterAlarmTab` | Maps an alarm kind to the device tab where it is examined in the post-release web UI (`fleet-ui-9`) | Does not apply to first-release commands (`ai/META-DECISIONS.md`, 2026-11-23); post-release web tab binding lives with `fleet-ui-9` or a `ze_web && ze_fleet_ui` register file |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | Ze is a platform, and other people's code runs against it | System door (always derived) | "`pkg/plugin/sdk`, `pkg/plugin/rpc`, `pkg/ze`, `pkg/zefs` and `pkg/fleet` are imported by code outside this repository." | agrees | Every new field on `pkg/fleet` envelopes is optional (`omitempty`), new `ChainHash`/diff types are additive, and `docs/architecture/fleet-config.md` is updated in the same work. |
| `ai/doors/hub-engine.md` | Boundaries | Door covering `cmd/ze`, `internal/component/managed`, `pkg/fleet` | "SIGHUP and a managed push both reach `doReload` (`cmd/ze/hub/main_reload.go`). A new trigger joins it rather than building a second path." | agrees | Delta apply, diff resync, and full replay all commit through `ClientConfig.CommitOver` -> `applyManagedPush` -> `reloadAfterCommit` -> `doReload`. |
| `ai/doors/plugin-infra.md` | Traps | Door covering `internal/component/plugin` | "so a payload must be compact `json.Marshal` output." | agrees | All extended fleet request and reply payloads are marshaled as compact single-line JSON through `rpc.MuxConn.CallRPC` and `SendResult`. |
| `ai/doors/core.md` | Boundaries | Door covering `pkg/zefs` | "In the daemon, use `statestore` with a key registered in `pkg/zefs/keys.go`" | agrees | Both chain state keys (`meta/managed/chain-state` and `meta/managed/client/{name}/chain`) are registered with `zefs.MustRegister` and written only through the daemon's borrowed `storage.Storage`. |
| `ai/doors/commands-cli.md` | Boundaries | Door covering `internal/component/managed` command registration | "The payload satisfies `ResponseData`, and output goes through `ApplyPipes`" | agrees | `show fleet status` adds structured fields (`chain-hash`, `chain-state`, `last-resync`) to its existing `fleetStatus` `ShapeDoc` payload and column order. |
| `ai/doors/le-tooling.md` | Where it sits | Door covering `test/managed/fleet-drift-hash-chain.ci` and `test/managed/fleet-resync-failed-alarm.ci` | "Functional `.ci` files live under `test/<suite>/` and run through `./le test <suite>`" | agrees | Both end-to-end functional proofs live under `test/managed/` and run through `./le test managed`. |
| `docs/architecture/fleet-config.md` | Non-Goals | `// Design:` header of `pkg/fleet/*`, `internal/component/managed/*`, `internal/component/plugin/server/managed*.go`, `cmd/ze/hub/managed*.go`; modified page | "Full config on change; configs are small (< 10KB typically)" | departs | This spec introduces diff-based managed config deltas and diff-based resync with full config replay as fallback, departing from the full-config-only non-goal while keeping full config replay for first boot, revert, and fallback. `docs/architecture/fleet-config.md` Non-Goals and Protocol sections are updated in the same work. |
| `docs/architecture/fleet-config.md` | The baseline and the hold | `// Design:` header of `internal/component/managed/baseline.go`, `client.go`; modified page | "A managed client must not lose a configuration change made on the client itself. The client therefore records which configuration its hub last gave it, and applies nothing from the hub while it runs a different one." | agrees | Every chain-mismatch resync checks `Diverged(active, baseline)` first and stages through `StageMachineCandidateOver` over `activeCheck(decided)`; a locally edited client stays in hold and never auto-resyncs. On a `current` answer (including `Reanchor: true`), a non-holding client verifies and persists its chain anchor locally without sending `config-ack`, preserving `"A client that was not holding sends none for a current answer"` and `TestFirstBaselineSendsNoAck`. |
| `docs/architecture/fleet-config.md` | Protocol | `// Design:` header of `pkg/fleet/envelope.go`; modified page | "Every field added after the first four verbs is optional and omitted when unset, and the payload of `config-held` names no client: the hub takes the name from the authenticated session." | agrees | All new chain and diff fields on the five existing verbs are `omitempty`, and no request names a client; the hub still scopes every chain lookup and update to the authenticated session name. |
| `plan/post-release/features/spec-fleet-0-umbrella.md` | Scope | Umbrella of this spec's set | "Continuous two-way merge. Replaced for now by single-writer plus an operator-gated `config-push` at divergence (`fleet-7`); full bidirectional sync stays out of scope" | agrees | The sync is continuous but strictly hub-to-client: a local edit (`active != baseline`) still holds and never flows to the hub automatically; only `fleet-7`'s operator-gated `adopt` brings it up. |
| `plan/release/features/spec-fleet-ui-5-alarm-list.md` | Alarms | Alarm list this spec's alarm joins | "an alarm kind is a registered entry with its condition, its severity and its sentence, so a later feature adds one without editing the list." | agrees | `config-resync-failed` (`Configuration resync failed`) is registered from this spec's package and derived from the hub's per-client chain state without editing any central list. |
| `plan/release/features/spec-fleet-ui-8-alarm-causes.md` | Alarm causes | Alarm cause hierarchy this spec declares a cause in | "A kind names its own causes in its own registration. No list of causes exists anywhere else, so a later feature adds a kind and its causes without an edit to this spec's code" | agrees | `config-resync-failed` declares `Device offline` (Critical >= Warning) in its own registration (choice C-1). |
| `docs/architecture/api/commands.md` | Design decisions | `// Design:` header of `internal/component/managed/cmd_fleet.go`, `register.go` | "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included" | agrees | No new subcommand or argument is added; chain state is reported as fields on the existing `show fleet status` document, and resync failure alarms appear in `show fleet alarm`. |
| `docs/architecture/api/commands.md` | Design decisions | `// Design:` header of `internal/component/managed/cmd_fleet.go`, `register.go` | "A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed."" | agrees | No pipe operator is added or overridden; `show fleet status` retains its `ShapeDoc` contract and standard pipe behavior. |
| `docs/architecture/zefs-format.md` | Key Namespaces | `// Design:` header of `pkg/zefs/keys.go`; modified page | "Keys follow a `<namespace>/<qualifier>/<path>` convention to prevent collisions between metadata and config files." | agrees | Both new keys (`meta/managed/chain-state` and `meta/managed/client/{name}/chain`) follow `meta/managed/...` with non-overlapping paths and basenames that never match `client-*.conf`. |
| `docs/architecture/storage-backends.md` | Opening and ownership | Anchored to `pkg/zefs/keys.go`, `cmd/ze/ze_core_start.go` | "`Open(configDir)` opens the live tree and holds `configDir/database.lock` until `Close`." | agrees | Chain state reads and writes use the daemon's single borrowed `storage.Storage` handle under existing store locking, with no second opener or raw file write. |
| `docs/architecture/doctor-and-health-checks.md` | The runtime phase: checks only the daemon runs | `// Design:` header of `internal/component/managed/register_doctor_codes.go`; modified page | "A check over such state registers in the `runtime` phase, through the same `diagnostic.RegisterDoctorCheck` every other check uses" | agrees | Hub and client runtime chain-drift findings register through `diagnostic.RegisterDoctorCheck` and `diagnostic.Register` in their owning packages, while store-readability checks run at post-config phase so `ze doctor` answers with the daemon stopped too. |
| `docs/plugin-development/metrics.md` | Naming Policy | Modified page | "Every metric name is built from four parts in fixed order:" | agrees | New counters (`ze_managed_chain_mismatches_total`, `ze_managed_chain_resyncs_total`, `ze_managed_client_chain_mismatches_total`, `ze_managed_client_resyncs_total`) follow `ze_{scope}_{subject}_{detail}_total` with `{method,result}` labels. |
| `ai/rules/repo-maintenance.md` | Directives | `// Design:` header of `internal/component/managed/register.go`, `doctor.go` | "A change that adds or changes a surface future agents use, verify, document or avoid MUST update its discovery path in the same work" | agrees | Updates `ai/INDEX.md`, `docs/architecture/fleet-config.md`, `docs/architecture/zefs-format.md`, `docs/architecture/doctor-and-health-checks.md`, `docs/plugin-development/metrics.md`, `docs/guide/fleet-config.md`, `features/fleet-management.md`, and registers doctor checks for new store keys. |
| `docs/architecture/core-design.md` | Design decisions | Anchored to `internal/component/plugin/server/managed_serve.go`, `cmd/ze/ze_core_start.go` | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | Managed fleet hash-chain sync touches neither VPP nor Linux kernel forwarding features, and commits reconstructed configs through the existing `Handler.Validate` and `doReload` path so all backend verifiers still run unchanged. |
| `docs/architecture/core-design.md` | Design decisions | Anchored to `internal/component/plugin/server/managed_serve.go`, `cmd/ze/ze_core_start.go` | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | Reconstructed delta and full-replay configs still pass the same `Handler.Validate` and `doReload` verification path, preserving identical refusal verdicts. |
| `docs/architecture/core-design.md` | Design decisions | Anchored to `internal/component/plugin/server/managed_serve.go`, `cmd/ze/ze_core_start.go` | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation." | agrees | No firewall or VPP verifier or YANG backend annotation is modified. |
| `docs/architecture/core-design.md` | Executive Summary | Anchored to `internal/component/plugin/server/managed_serve.go`, `cmd/ze/ze_core_start.go` | "" | silent | Covers core BGP wire/RIB architecture and `ze`/`le` composition roots; silent on the managed fleet hash-chain protocol. |
| `docs/architecture/container/image-store.md` | The data directory and the store layout | Anchored to `pkg/zefs/keys.go` | "" | silent | Covers container image store keys under `meta/container/`; silent on `meta/managed/` keys and fleet config sync. |
| `docs/architecture/diagnostics/debug-filtering.md` | Profiles are one JSON value | Anchored to `pkg/zefs/keys.go` | "" | silent | Covers debug profile keys under `debug/profile/`; silent on managed fleet config sync. |
| `docs/architecture/irr-owner.md` | Where it sits | Anchored to `pkg/zefs/keys.go` | "" | silent | Covers IRR dataset keys under `meta/irr-*`; silent on managed fleet config sync. |
| `docs/architecture/api/architecture.md` | Implementation Status | Anchored to `internal/component/plugin/server/managed_serve.go` | "" | silent | Covers REST/gRPC/API listener architecture; silent on the managed fleet channel protocol and hash chain. |
| `docs/architecture/hub-architecture.md` | Startup Sequence | Anchored to `internal/component/plugin/server/managed_serve.go`, `cmd/ze/hub/managed_owner.go`, `managed_server.go` | "The hub also cancels and joins the complete outbound managed client, because its callbacks can read or write storage before reload admission. A managed start holds that client in one owner, which `request fleet disable` and `enable` also stop and start; at shutdown the owner joins the running client and refuses every later start. It retains the managed server and joins its active config requests and notifications through `Stop`. Both owners finish before the hub returns its store to the launcher." | agrees | Client `ChainAccess` runs inside `managedClientOwner`, and `runConnection` cancels and joins the `sendPing` goroutine before returning so `managedClientOwner.shutdown` joins every ping and store callback; hub `ReadBaseVersion` and chain store reads/writes run inside `ManagedServer.serve` requests joined by `ManagedServer.Stop`, all finishing before the hub returns its store. |
| `docs/architecture/api/process-protocol.md` | Wire Format | `// Design:` header of `internal/component/plugin/server/register.go` | "Every message is a single newline-terminated line:" | agrees | All extended managed-channel request and reply payloads remain single-line `#<id> <method>/<ok> [<json>]\n` frames on `rpc.MuxConn`, and `plugin/server/register.go` only adds `CodeDoctorManagedChainDrift` to `serverEventCodes` with no change to plugin process RPCs. |
| `docs/architecture/system-architecture.md` | Build personalities | `// Design:` header of `cmd/ze/ze_core_start.go` | "" | silent | Legacy hub/orchestrator design note covering `ze`/`le` personalities and system RIB process boundaries; silent on managed fleet hash-chain sync. |
| `docs/architecture/appliance/device-config.md` | Legacy boot inbox | Anchored to `cmd/ze/ze_core_start.go` | "" | silent | Covers appliance boot seeds and legacy unmanaged `meta/config/active-hash`; silent on managed fleet hash-chain sync. |
| `docs/architecture/appliance/kernel-profiles.md` | Go owns the guarantee | Anchored to `cmd/ze/ze_core_start.go` | "" | silent | Covers appliance kernel profile selection; silent on managed fleet config sync. |
| `docs/architecture/fleet-interface.md` | The package | Anchored to `cmd/ze/ze_core_start.go` | "" | silent | Covers `/fleet/` web UI shell and `fleetui.Source` contract; silent on the managed channel hash-chain protocol. |
| `docs/architecture/fleet-config.md` | Design decisions | Managed protocol and command surfaces | "A reload that changes the certificate a serving hub's managed listener presents installs it on the running listener at that reload: the next handshake presents the certificate the configuration names, the listener keeps its socket and no session is closed, as the web and looking glass listeners do. The certificate stays in the configuration's `pki` block. In Thomas's words (voice-transcribed): "Keep the certificate in the configuration." and "In that case on commit we can perform the certificate change on the listener."" | agrees | Managed replies travel over the established connection. Certificate selection for the next handshake stays with the serving listener. |
| `docs/architecture/api/commands.md` | Design decisions | Managed protocol and command surfaces | "A pipe operation works on the answer of the command before it. It may narrow the answer, extend it, transform it, or deliver it somewhere, such as saving it to a file. It never changes what the command does" | agrees | The status handler returns the chain fields in its answer. Piping the answer cannot initiate a resync. |
| `docs/architecture/api/architecture.md` | Design decisions | Managed protocol and command surfaces | "When `\| implied` modifies an existing config line (such as inserting automatic members into a `filters [ ... ];` list), it keeps the line intact and places an inline `# implied: ...` comment on the same line after the statement naming what was added and why, rather than placing a comment only above the line or splitting the list across multiple lines. In Thomas's words (2026-11-23): "No, it should be on the same line and say what was added"." | agrees | The transport diff uses stored configuration bytes. The annotated output of the implied pipe is a display and never serves as a patch base. |
| `docs/architecture/core-design.md` | Design decisions | Managed protocol and command surfaces | "Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it"." | agrees | The patch reconstructs the configured backend names byte for byte before the target validates the complete candidate. |
| `docs/architecture/core-design.md` | Design decisions | Managed protocol and command surfaces | "BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it."" | agrees | The managed client submits the reconstructed configuration to the normal reload path. BGP continues to derive next-hop rewriting from the export chain. |
| `docs/architecture/core-design.md` | Design decisions | Managed protocol and command surfaces | "Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs")." | agrees | Both policy authoring forms survive transport because the reconstructed bytes must match the target version. |
| `docs/architecture/core-design.md` | Design decisions | Managed protocol and command surfaces | "Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)")." | agrees | The fleet protocol carries the operator's configuration. BGP compiles its built-in steps when that configuration loads. |
| `docs/architecture/core-design.md` | Design decisions | Managed protocol and command surfaces | "Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self")." | agrees | Applying a configuration patch preserves policy ordering. The BGP chain evaluator owns next-hop precedence. |
| `docs/architecture/core-design.md` | Design decisions | Managed protocol and command surfaces | "Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load")." | agrees | Validation runs on the reconstructed candidate, including the configured address and chain direction. |
| `docs/architecture/core-design.md` | Design decisions | Managed protocol and command surfaces | "Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes")." | agrees | Fleet resync updates configuration. The BGP export path still distinguishes forwarded and locally originated routes. |
| `docs/architecture/core-design.md` | Design decisions | Managed protocol and command surfaces | "Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?"." | agrees | Transport operates on stored bytes. The implied-filter display continues to derive its entries from the chain registrations. |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Hub config write to `file/active/client-<name>.conf` (triggers `store.SetWriteObserver` -> `ManagedServer.NotifyConfigChanged`).
- Managed client connect and periodic 30-second heartbeat (`ping` with `chain-hash`) or hub-pushed `config-changed`.
- Operator running `show fleet status`, `show fleet alarm` (on any command surface), `ze doctor` / `show doctor`, or scraping `/metrics`.

### Wire Data Structures (`pkg/fleet/`)

#### SHA-1 Chain Fold (`pkg/fleet/chain.go`)

| Function / Constant | Signature / Value | Behavior |
|---------------------|-------------------|----------|
| `ChainHashLen` | `40` | Lowercase hex length of a 20-byte SHA-1 digest. |
| `ChainState` | Struct: `Version string` (`json:"version"`), `Chain string` (`json:"chain"`) | Persisted pair tying a 40-hex SHA-1 chain head to the 16-hex `VersionHash` of the active config it represents. |
| `ValidChainHash(s string) bool` | Returns true iff `len(s) == 40` and all characters are lowercase hex `0-9a-f`. | Strict validation on wire and store decode; malformed strings are rejected, never treated as empty. |
| `FoldFull(config []byte) string` | `SHA-1(20 zero bytes \|\| "full\n" \|\| config)` as 40 lowercase hex chars | Anchors a chain deterministically from zero on initial sync, revert, adopt re-anchor, or full-replay fallback. |
| `FoldDiff(prevHex, baseVer, targetVer string, delta []byte) (string, error)` | Decodes `prevHex` (20 bytes; error if invalid), computes `SHA-1(prev20 \|\| "diff\n" \|\| baseVer \|\| "\n" \|\| targetVer \|\| "\n" \|\| delta)` as 40 lowercase hex chars | Folds one applied delta onto `prevHex`; both ends fold identical bytes. |

#### Deterministic Line Diff / Patch (`pkg/fleet/diff.go`)

| Type / Constant / Function | Field / Behavior | Bounds / Invariants |
|----------------------------|------------------|---------------------|
| `MaxDiffOps` | `65536` | Upper bound on edit operations in one diff; exceeded -> hub falls back to full replay, client rejects diff and falls back to full replay. |
| `DiffOp` | `Op string` (`"copy"` or `"ins"`), `Start int` (0-based source line index, for `"copy"`), `Count int` (line count, for `"copy"`), `Lines []string` (inserted lines with exact line endings preserved, for `"ins"`) | Compact deterministic JSON array (`[]DiffOp`); preserves exact `\n` / `\r\n` / trailing-newline bytes by splitting lines with terminators kept (`strings.SplitAfter`). |
| `ComputeDiff(base, target []byte) (delta []byte, worthSending bool, err error)` | Runs deterministic Myers/LCS line diff on terminator-preserving lines of `base` and `target`, serializes compact JSON `[]DiffOp`. Sets `worthSending = false` if op count exceeds `MaxDiffOps` or `len(delta) >= len(target)`. | Deterministic output for identical `(base, target)`; round-trip verified (`ApplyDiff(base, delta) == target`). |
| `ApplyDiff(base, delta []byte, maxOutput int) ([]byte, error)` | Strictly decodes `[]DiffOp`, validates `len(ops) <= MaxDiffOps`, bounds every `copy` range against `base` lines, bounds total output bytes against `maxOutput` (`rpc.MaxMessageSize`, 16 MiB), and reconstructs exact target bytes. | Rejects unknown op kinds, negative/out-of-bounds indices, oversized output, or trailing garbage. Never modifies `base`. |

#### Extended Envelope Fields (`pkg/fleet/envelope.go`)

| Struct | Existing Fields | New Optional Fields (`omitempty`) | Meaning |
|--------|-----------------|-----------------------------------|---------|
| `ManagedReply` (new shared reply struct) | (none — replaces empty `SendOK` on `ping`, `config-ack`, `config-held`, `config-changed`) | `ChainHash string` (`json:"chain-hash,omitempty"`) | Sender's current expected SHA-1 chain head carried on every reply. Empty `ok` from older peers decodes as empty `ChainHash` (no false drift). |
| `PingRequest` (new request struct for `VerbPing`) | (was `struct{}{}`) | `ChainHash string` (`json:"chain-hash,omitempty"`) | Client's current SHA-1 chain head sent on every 30s heartbeat. |
| `ConfigFetchRequest` | `Version` | `ChainHash string` (`json:"chain-hash,omitempty"`), `Resync string` (`json:"resync,omitempty"`: `""`, `"diff"`, `"full"`) | Carries client's chain head and explicit resync intent (`"diff"` on mismatch first, `"full"` on fallback). |
| `ConfigFetchResponse` | `Version`, `Config`, `Status`, `HeldVersion`, `DiscardVersion` | `Mode string` (`json:"mode,omitempty"`: `""`/`"full"`, `"diff"`), `BaseVersion string` (`json:"base-version,omitempty"`), `Diff string` (`json:"diff,omitempty"`, base64 of `delta`), `Reanchor bool` (`json:"reanchor,omitempty"`), `ChainHash string` (`json:"chain-hash,omitempty"`) | When `Mode == "diff"`, carries base64 line diff over `BaseVersion` (with `Reanchor: true` when base chain is `FoldFull(A)` instead of prior `prev`) and expected `ChainHash`. When `Status == "current"`, carries hub's expected `ChainHash` (and `Reanchor: true` when only chain head drifted). When `Mode == "full"` (or empty), carries full base64 `Config` and `ChainHash = FoldFull(B)`. |
| `ConfigChanged` | `Version` | `ChainHash string` (`json:"chain-hash,omitempty"`) | Hub's expected target chain head when known. |
| `ConfigAck` | `Version`, `OK`, `Error`, `Holding` | `ChainHash string` (`json:"chain-hash,omitempty"`), `RejectKind string` (`json:"reject-kind,omitempty"`: `""`, `"verify"`, `"commit"`, `"resync"`) | Client's SHA-1 chain head after applying (or current chain head on refusal/hold), plus structured refusal class so the hub distinguishes a verify refusal (`Configuration rejected`, `fleet-ui-5`) from a resync/commit failure (`Configuration resync failed`, `fleet-10`). |
| `ConfigHeld` | `Version`, `Config`, `EditedAt` | `ChainHash string` (`json:"chain-hash,omitempty"`) | Client's chain head at hold time (informational). |

### Transformation Path
1. **Steady-state reply comparison:** On every RPC reply (`ping`, `config-ack`, `config-changed`, `config-fetch`, `config-held`), both ends carry `chain-hash`. On `ping`, the hub reads its in-memory/stored `ChainState` for `name` and checks the current active version of `client-<name>.conf`: if `currentVersion == hubChain.Version`, hub replies `ManagedReply{ChainHash: hubChain.Chain}` (zero disk writes). If `currentVersion != hubChain.Version` (an unapplied hub change exists) or `hubChain` is uninitialized, hub replies with its target/anchor chain (`FoldFull(currentBytes)` or pending expected chain), which differs from the client's chain and triggers client resync.
2. **Ping drift trigger (no concurrency race, joined on exit):** Client's ping goroutine calls `sendPing(ctx, mc, clientChain)` with `pingTimeout = 10s`. If `reply.ChainHash != ""` and `reply.ChainHash != clientChain`, it performs a non-blocking send to `driftCh` (buffered capacity 1). `notificationLoop` selects on `<-driftCh` and invokes `handleDriftMismatch` serially on the notification goroutine. `runConnection` cancels a connection-scoped ping context and joins the `sendPing` goroutine before returning, so `managedClientOwner.shutdown` joins every ping and `ChainAccess` callback before the store closes.
3. **Hold precedence (`fleet-6`/`fleet-7` preserved):** Before any resync or delta apply, client reads `(active, baseline)` via `BaselineAccess.read()`. If `Diverged(active, baseline)` is true, client NEVER auto-resyncs and NEVER overwrites local config; it stays in hold (`fleet-6`/`fleet-7` `config-held` + `holding: true` ack, resolved only by operator `adopt` or `revert`). Any hub-side `config-resync-failed` state for a holding client clears immediately because `Configuration diverged` owns that condition.
4. **Diff-based delivery and diff-based resync first:** When not locally diverged, client sends `config-fetch {version: active, chain-hash: clientChain, resync: "diff"}` (or `resync: ""` on normal connect/`config-changed`). Hub handles `HandleConfigFetch`:
   - If `req.Resync == "full"` or a `DiscardVersion` revert is offered: hub serves `Mode: "full"`, `Config: base64(B)`, `Version: V_now`, `ChainHash: FoldFull(B)`, and records `session.pending = {version: V_now, chain: FoldFull(B), resync: req.Resync}`.
   - If `req.Version == V_now`:
     - If `req.ChainHash != "" && req.ChainHash == hubChain.Chain && hubChain.Version == V_now`: hub replies `Status: "current", ChainHash: hubChain.Chain` (converged; clears any in-memory resync-failed state).
     - Otherwise (same config bytes, drifted/missing chain): hub computes `anchor = FoldFull(B)`, persists `ChainState{V_now, anchor}` idempotently (only if changed), and replies `Status: "current", Reanchor: true, ChainHash: anchor`. A non-holding client verifies `FoldFull(activeBytes) == resp.ChainHash`, persists `ChainState{V_now, anchor}`, and sends **no `config-ack`** (preserving `docs/architecture/fleet-config.md`'s rule that a client that was not holding sends none for a `current` answer, and keeping `TestFirstBaselineSendsNoAck` intact; if the reply is lost in flight, the next `ping` sees `clientChain != anchor` and retries the idempotent re-anchor). A holding client whose hold just ended on `current` (e.g., after `adopt`) still sends `config-ack {version: V_now, ok: true, chain-hash: anchor}` as today. If client's `FoldFull(activeBytes) != resp.ChainHash`, client falls back once to `resync: "full"`.
   - If `req.Version != V_now` and `req.Version != ""`: hub looks up historical base bytes `A` for `req.Version` via `storage.ListVersions(ClientConfigKey(name))` + `ReadVersion` (matching the 16-hex prefix of `sha256:<64-hex>`).
     - If `A` is found and `ComputeDiff(A, B)` returns `worthSending == true`:
       - If `req.ChainHash != "" && hubChain.Version == req.Version && req.ChainHash == hubChain.Chain`: normal delta (`Reanchor: false`, `nextChain = FoldDiff(req.ChainHash, req.Version, V_now, delta)`).
       - Else (chain drifted or uninitialized, but base `A` matches `req.Version`): diff-based resync with re-anchor (`Reanchor: true`, `baseAnchor = FoldFull(A)`, `nextChain = FoldDiff(baseAnchor, req.Version, V_now, delta)`).
       - Hub replies `Mode: "diff", BaseVersion: req.Version, Version: V_now, Diff: base64(delta), Reanchor: reanchor, ChainHash: nextChain` and records `session.pending = {version: V_now, chain: nextChain, mode: "diff"}`.
     - Otherwise (base `A` not found in history, or `len(delta) >= len(B)`, or op limit exceeded): hub notes the diff-unavailable reason on `session.lastDiffReason` and serves `Mode: "full"` directly.
5. **Client diff verification, guarded apply, and full-replay fallback:**
   - When client receives `Mode: "diff"`: reads `activeBytes` and verifies `VersionHash(activeBytes) == resp.BaseVersion`; decodes base64 `resp.Diff`; computes `prev = clientChain` (or `FoldFull(activeBytes)` if `resp.Reanchor`); verifies `FoldDiff(prev, resp.BaseVersion, resp.Version, delta) == resp.ChainHash`; runs `ApplyDiff(activeBytes, delta, rpc.MaxMessageSize)` -> `reconstructed`; verifies `VersionHash(reconstructed) == resp.Version`; runs `Handler.Validate(reconstructed)`.
   - If ANY step above fails: client logs at `Info` (incrementing `ze_managed_client_resyncs_total{method="diff",result="fallback"}`), stages nothing, leaves chain untouched, and sends ONE bounded fallback `config-fetch {version: active, chain-hash: clientChain, resync: "full"}`.
   - On validation success (diff or full): client commits through `CommitOver(reconstructed, activeCheck(decided))` (inside `confirm.WriteOutside` and `StageMachineCandidateOver`). On `errActiveChanged`, `refetchOverLocalCommit` runs as today (no chain advance). On commit success: `RecordActiveAsBaseline` runs, client persists `meta/managed/chain-state = {Version: resp.Version, Chain: resp.ChainHash}`, clears any active `report.ClearWarning` for `doctor-managed-chain-drift`, sends `config-ack {version: resp.Version, ok: true, chain-hash: resp.ChainHash}`, and runs `adoptEndpoint`.
   - If full replay fails: client distinguishes `RejectKind: "verify"` (YANG validation refusal -> opens `Configuration rejected` in `fleet-ui-5`, not `config-resync-failed`) from `RejectKind: "commit"` / `"resync"` (non-verify apply, decode, or chain failure -> opens `config-resync-failed` on the hub). Client leaves active config and chain unchanged, logs `Warn` under `doctor-managed-chain-drift`, raises a transient report-bus warning via `report.RaiseWarningFix`, and latches `lastFailedSync = (resp.Version, resp.ChainHash)` so subsequent 30s pings do not loop.
6. **Hub chain commit & alarm derivation:** In `handleConfigAck`, when `ack.OK && ack.Version == session.pending.version && ack.ChainHash == session.pending.chain` (or when a hold ends on `current` with `ack.Version == V_now && ack.ChainHash == anchor`), the hub updates its in-memory chain map, persists `meta/managed/client/<name>/chain` via `Store.WriteKey` only if `(Version, Chain)` changed (never on `ping`), clears any resync-failed state for `name`, and replies `ManagedReply{ChainHash: ack.ChainHash}`. On a matching `ping` (`ping.ChainHash == hubChain.Chain` for `V_now`), the hub performs zero store writes and clears any in-memory resync-failed state (covering recovery via a `current` re-anchor). When a resync ends with both diff and full replay failed (non-verify refusal or failed replay delivery/chain mismatch) on a non-holding client, the hub records the bounded failure tuple (`failedAt`, `diffReason`, `fullReason`, `clientChain`, `expectedChain`) in memory on that client's hub chain state, from which the registered `config-resync-failed` alarm kind (`fleet-ui-5` / `fleet-ui-8`) derives its open Warning alarm (hidden behind `Device offline` under C-1).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Managed client ↔ Hub | TLS 1.3 + `rpc.MuxConn` compact JSON frames on existing 5 verbs | No |
| Managed client → Store | `storage.Storage` `ReadFile` / `WriteKey` on `meta/managed/chain-state` via `ChainAccess` | No |
| Managed client → Daemon reload | `ClientConfig.CommitOver` -> `applyManagedPush` -> `StageMachineCandidateOver` -> `doReload` | No |
| Hub ManagedServer → Store | `storage.Storage` `ListVersions` / `ReadVersion` (base lookup) and `ReadKey` / `WriteKey` (`meta/managed/client/{name}/chain`) | No |
| Hub chain state → Alarm registry | Registered `config-resync-failed` kind (`fleet-ui-5` / `fleet-ui-8`) reads exported snapshot of hub chain state | No |
| Managed client → CLI / Doctor / Metrics | `ClientState` snapshot to `show fleet status`, `diagnostic.RegisterDoctorCheck`, `metrics.Registry` | No |

### Changed-Wire Census

**Search evidence:** Scope `pkg/`, `internal/`, `cmd/`, `test/` and repository shell and Python scripts across Go (`*.go`) and non-Go (`*.ci`, `*.et`, `*.wb`, `.sh`, `.py`). Terms searched: `VerbConfigFetch`, `VerbConfigChanged`, `VerbConfigAck`, `VerbConfigHeld`, `VerbPing`, `ConfigFetchRequest`, `ConfigFetchResponse`, `ConfigChanged`, `ConfigAck`, `ConfigHeld`, `FetchConfig`, `SendConfigAck`, `config-fetch`, `config-changed`, `config-ack`, `config-held`, `meta/managed/`. Positive controls found: `pkg/fleet/envelope.go` (declarations), `internal/component/managed/client.go` (`fetchConfigWithin`, `SendConfigAck`, `sendPing`, `uploadHeld`, `handleConfigChangedRequest`), `internal/component/plugin/server/managed_serve.go` (`handleRequest`, `pushConfigChanged`), `internal/component/plugin/server/managed.go` (`HandleConfigFetch`, `BuildConfigChanged`), `cmd/ze/ze_core_start.go` (`fetchInitialConfig` calling `managed.FetchConfig`). Non-Go scan found zero shell or Python wire speakers; the managed functional fixtures also exercise the real daemons.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| `config-fetch` request | writer | `internal/component/managed/client.go` `fetchConfigWithin` / `FetchConfig`; `cmd/ze/ze_core_start.go` `fetchInitialConfig` | `client.go:980`, `ze_core_start.go:769` | Change: `fetchConfigWithin` sends `ChainHash` and `Resync`; `fetchInitialConfig` sends empty `Version`/`ChainHash` (first boot) and persists returned `ChainHash` via `cacheFirstConfig`. |
| `config-fetch` request | writer (tests) | `cmd/ze/hub/managed_lifetime_test.go`, `fleet_perf_test.go`, `managed_server_test.go`; `internal/component/plugin/server/managed*_test.go` | Grepped across `cmd/` and `server/` | Preserved: new fields are `omitempty`, so existing callers sending only `Version` continue to work (hub serves full replay when `ChainHash` is absent and no base matches). |
| `config-fetch` request | reader | `internal/component/plugin/server/managed_serve.go` `handleRequest`; `managed.go` `HandleConfigFetch` | `managed_serve.go:559-583`, `managed.go:85` | Change: reads optional `ChainHash` and `Resync`, resolves diff/reanchor/full reply. |
| `config-fetch` reply | writer | `internal/component/plugin/server/managed.go` `HandleConfigFetch`, `managed_serve.go` `handleRequest` | `managed.go:98-111`, `managed_serve.go:581` | Change: populates `Mode`, `BaseVersion`, `Diff`, `Reanchor`, `ChainHash` (`omitempty`). |
| `config-fetch` reply | reader | `internal/component/managed/client.go` `fetchConfigWithin` / `fetchAgainstBaseline`; `cmd/ze/ze_core_start.go` `fetchInitialConfig` | `client.go:541-567,990`, `ze_core_start.go:769` | Change: handles `Mode: "diff"`, `Reanchor`, `ChainHash`, and full-replay fallback; `cacheFirstConfig` persists initial `ChainState`. |
| `config-ack` request | writer | `internal/component/managed/client.go` `SendConfigAck` / `sendAck` | `client.go:552,728,838,850,1000` | Change: populates `ChainHash` and `RejectKind` (`omitempty`) and reads `ManagedReply` from `CallRPC`. |
| `config-ack` request | reader | `internal/component/plugin/server/managed_serve.go`, `handleRequest` and `handleConfigAck` | The hub decodes `fleet.ConfigAck` before handling the result. | Compare the reported chain with the pending version and record failure details when resync fails. |
| `config-ack` reply | writer | `internal/component/plugin/server/managed_serve.go`, `handleRequest` | The hub calls `SendOK` after processing the acknowledgement. | Return the expected chain head in the reply. |
| `config-ack` reply | reader | `internal/component/managed/client.go`, `SendConfigAck` | The client waits for `CallRPC` and currently discards its result payload. | Decode the optional chain head on success. An older hub's empty reply still completes the acknowledgement. |
| `ping` request & reply | writer / reader | `internal/component/managed/client.go` `sendPing`, `handleHubRequest` | `client.go:483,917-918,1011-1017` | Change: `sendPing` sends `PingRequest{ChainHash}` with 10s `pingTimeout`, parses `ManagedReply` (empty reply tolerated), signals `driftCh` on mismatch; inbound `VerbPing` replies `SendResult(ManagedReply{ChainHash})`. |
| `ping` request & reply | reader / writer | `internal/component/plugin/server/managed_serve.go` `handleRequest` | `managed_serve.go:594-595` | Change: unmarshals optional `PingRequest`, counts mismatch if client chain differs, replies `SendResult(ManagedReply{ChainHash})`. |
| `config-changed` request & reply | writer / reader | `internal/component/plugin/server/managed.go` `BuildConfigChanged`, `managed_serve.go` `pushConfigChanged` | `managed.go:116`, `managed_serve.go:760-775` | Change: `BuildConfigChanged` adds optional `ChainHash`; `pushConfigChanged` reads `ManagedReply` from client reply and records mismatch metric if chains differ. |
| `config-changed` request & reply | reader / writer | `internal/component/managed/client.go` `handleConfigChangedRequest` | `client.go:933-941` | Change: replies `SendResult(ManagedReply{ChainHash})` instead of empty `SendOK`, then runs `fetchAndProcess`. |
| `config-held` request & reply | writer / reader | `internal/component/managed/client.go` `uploadHeld`; `internal/component/plugin/server/managed_held.go` `handleConfigHeld` | `client.go:793`, `managed_held.go:405-430` | Change: client attaches optional `ChainHash`; hub replies `SendResult(ManagedReply{ChainHash})` on success (refusals keep `SendCodedError`). |
| Client chain store key (`meta/managed/chain-state`) | writer / reader | New: `internal/component/managed/chain.go` (`ChainAccess`); read by `cmd_fleet.go` and `doctor.go` | New key in `pkg/zefs/keys.go`; raw `ze data` readers preserved | New key family; raw `ze data` prints bytes unchanged. |
| Hub client chain store key (`meta/managed/client/{name}/chain`) | writer / reader | New: `internal/component/plugin/server/managed_chain.go`; read by `managed_serve.go`, doctor snapshot, and alarm kind | New key in `pkg/zefs/keys.go`; basename `chain` does not match `client-*.conf` | New key family; write observer ignores it; raw `ze data` prints bytes unchanged. |

### Integration Points
- `pkg/fleet/chain.go` & `pkg/fleet/diff.go` & `pkg/fleet/envelope.go` - SHA-1 fold, deterministic line diff/patch, optional envelope fields.
- `pkg/zefs/keys.go` - `KeyManagedChainState` and `KeyManagedClientChain`.
- `internal/component/plugin/server/managed.go`, `managed_serve.go`, `managed_chain.go`, `managed_doctor.go`, `doctor_codes.go`, `register.go` - hub chain persistence, historical base lookup via `storage.ListVersions`/`ReadVersion`, diff/reanchor/full fetch handling, reply `ChainHash`, resync-failure state for alarm derivation, doctor/event code registration (single owner), and mismatch/resync metrics.
- `internal/component/managed/chain.go`, `client.go`, `cmd_fleet.go`, `doctor.go`, `register.go`, `alarm_resync.go` - `ChainAccess`, `driftCh` + `pingTimeout` + ping goroutine join, diff apply + bounded full-replay fallback, `show fleet status` fields, client doctor check referencing `pluginserver` codes, `report.RaiseWarningFix`/`ClearWarning`, client metrics via injected `Metrics *metrics.Registry`, and registration of `config-resync-failed` alarm kind (with `Device offline` cause) into `fleet-ui-5`/`fleet-ui-8`'s alarm registry.
- `cmd/ze/hub/managed.go`, `cmd/ze/hub/managed_owner.go`, `cmd/ze/hub/managed_server.go`, `cmd/ze/ze_core_start.go` - wire `ChainAccess`, `Metrics: registry.GetMetricsRegistry()`, and `ReadBaseVersion` (hub historical lookup), persist initial chain in `cacheFirstConfig`, wire `OnChainState` callback into `managedClientOwner`.
- `internal/component/plugin/doctor/register_external.go`, `check_managed_runtime.go` - register and run hub-side runtime doctor checks for persistent chain drift and unreadable client chain state.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Diff/replay bytes flow through `Handler.Validate` -> `CommitOver` -> `StageMachineCandidateOver` -> `doReload`; chain persistence flows through `storage.Storage` under registered `pkg/zefs` keys; `config-resync-failed` derives from hub chain state through the `fleet-ui-5` alarm kind registry |
| No unintended coupling (components stay isolated) | Yes | `pkg/fleet` uses stdlib only (`crypto/sha1`, `encoding/json`, `strings`); `plugin/server` and `managed` share only `pkg/fleet` and `pkg/zefs`; no always-on package imports gated `ze_web`/`ze_fleet_ui` packages |
| No duplicated functionality (extends existing, does not recreate) | Yes | Reuses the 5 existing fleet verbs, `storage.ListVersions`/`ReadVersion` history, `CommitOver`/`BaselineAccess`, `managedDoctorState`, `show fleet status`, and `fleet-ui-5`/`fleet-ui-8` alarm/cause registration |
| Zero-copy preserved where applicable (refs, not copies) | Yes | Hub holds no in-RAM copy of client config blobs; `FoldFull`/`FoldDiff` hash slices in-place via `sha1.New()`; `ApplyDiff` pre-sizes output within `maxOutput` |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | Doctor checks/codes, ZeFS keys, `show fleet status` columns, and the `config-resync-failed` alarm kind (with its `Device offline` cause) all register from `init()` in their owning packages |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | Yes | Searched `internal/component/plugin/all/testdata/`, `internal/component/command/`, `internal/core/diagnostic/`, `pkg/zefs/`: no central switch or static list enumerates managed status columns, doctor codes, zefs keys, or alarm kinds — each derives from its owning package's `init()` registration |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Hub retains past versions of `client-<name>.conf` under `file/<stamp>/client-<name>.conf` -> `object/<sha256>` with no automatic pruning | Verified in `internal/component/config/storage/history_revision.go` (`versionsForName`) and `pointer.go` (`removeVersionLocked` only removes failed candidates) | Hub could not reconstruct base bytes `A` for a client's prior version and would always fall back to full replay | `TestManagedServerResolvesHistoricalBaseForDiff` | unvalidated |
| A-2 | Every managed client config apply goes through `CommitOver` -> `StageMachineCandidateOver` over `activeCheck(decided)` and updates `meta/managed/base-version` from stored active bytes | Verified in `cmd/ze/hub/managed.go` (`wireManagedCommit`, `applyManagedPush`) and `internal/component/managed/baseline.go` (`RecordActiveAsBaseline`) | Delta or resync could silently overwrite a concurrent local edit or leave `VersionHash` out of sync with `base-version` | `TestClientDiffResyncPreservesConcurrentLocalCommit` | unvalidated |
| A-3 | Deterministic line-based `ApplyDiff(A, ComputeDiff(A, B))` reproduces byte-identical `B` so `fleet.VersionHash(reconstructed) == fleet.VersionHash(B)` across hierarchical, `FormatSetMeta`, comments, CRLF/LF, trailing-newline variants, and `$9$` secrets | `strings.SplitAfter` preserves exact terminators and bytes per line | Next `config-fetch` or `fleet-7` `adopt`/`revert` sees a false version mismatch | `TestDiffRoundTripByteExactAcrossFormats` | unvalidated |
| A-4 | Hub-side transport line diffing works even when `client-<name>.conf` contains `unchecked-plugins` sections because the hub does not YANG-parse the client config to diff it, while the client runs full YANG `Handler.Validate` with its own external plugin schemas | Verified in `internal/component/managed/cmd_divergence_show.go` and `handler.go` | Clients with external plugins would fall back to full replay unnecessarily | `TestDiffWorksWithUnknownExternalPluginSection` | unvalidated |
| A-5 | `rpc.MuxConn` callers can upgrade empty `SendOK` replies (`ping`, `config-ack`, `config-held`, `config-changed`) to `SendResult(ManagedReply)` while tolerating empty replies from older peers | Verified in `pkg/plugin/rpc/mux.go` (`SendOK` vs `SendResult`, `CallRPC` returning `json.RawMessage`) | Older peer or test fixture expecting empty reply could fail decoding | `TestManagedReplyToleratesLegacyEmptyOK` | unvalidated |
| A-6 | `fleet-1` (`plan/release/features/spec-fleet-1-device-registry.md`, Status: `ready`) lowers hub managed read bound to ~1.4 MiB and targets 2,000+ connected clients | Verified in `plan/release/features/spec-fleet-1-device-registry.md` | Large client→hub payloads or per-ping store fsyncs would break scale or hit the hub read limit | `TestPingMatchPerformsZeroStoreWrites` and payload-size assertions | unvalidated |
| A-7 | `fleet-ui-5` (`show fleet alarm` kind registry) and `fleet-ui-8` (Causes registration) land before Phase 5 (alarm kind wiring) of this spec | Listed in `Depends` (`plan/release/features/spec-fleet-ui-5-alarm-list.md`, `spec-fleet-ui-8-alarm-causes.md`) | Alarm kind registration waits for `fleet-ui-5`/`fleet-ui-8` | `TestResyncFailedAlarmOpensAndClears`, `TestResyncFailedAlarmHiddenBehindOffline` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Delta fails to reconstruct exact `VersionHash(B)` or fails `Handler.Validate` (e.g., corrupted diff, truncated line, stale base) | Pre-commit `VersionHash`, `ChainHash`, or `Handler.Validate` check fails | Stage nothing, leave chain untouched, log at `Info`, increment fallback counter, and trigger ONE bounded `config-fetch {resync: "full"}` fallback |
| R-2 | Hub and client chains diverge across a crash between commit and chain-state write, or a dropped `config-ack` | Chain mismatch on next `ping` or reconnect | Diff-based resync with `reanchor: true` (verifying `FoldFull(activeBytes)` locally); if that fails or base is missing, full replay resets both ends to `FoldFull(B)` |
| R-3 | Client with a local emergency edit (`Diverged(active, baseline)`) gets overwritten by mismatch auto-resync | Local edit lost on next ping or reply | Check `Diverged(active, baseline)` BEFORE any auto-resync; locally diverged client stays in `fleet-6`/`fleet-7` hold and never auto-resyncs |
| R-4 | Infinite resync loop when hub holds an invalid config or client has an open confirmed-commit window | Repeated `config-fetch` every 30s ping | Latch `lastFailedSync = (version, chain)` on full-replay failure; skip ping-triggered resync while hub advertises the same `(version, chain)` until hub changes or reconnect; log `Warn` under `doctor-managed-chain-drift` |
| R-5 | Concurrency race or leaked goroutine between 30s `sendPing` goroutine, `notificationLoop`, and store shutdown | `-race` failure, interleaved `CommitOver` calls, or ping accessing store after `managedClientOwner.shutdown` | `sendPing` reads client chain via atomic/mutex snapshot and signals `notificationLoop` via 1-buffered `driftCh`; only `notificationLoop` executes fetch/resync/commit; `runConnection` cancels a connection-scoped ping context and joins `sendPing` before returning |
| R-6 | Stalled `ping` hangs the ping goroutine forever (today `sendPing` has no per-call timeout) | Heartbeat misses never detected promptly or goroutine leaked | Bound `sendPing` with `pingTimeout = 10s` (< `heartbeatInterval = 30s`) and join `sendPing` before `runConnection` exits |
| R-7 | Hostile or malformed diff triggers excessive memory allocation on client (diff bomb) | Memory spike on `ApplyDiff` | Cap op count at `MaxDiffOps` (65536), validate every `copy` range, enforce `maxOutput = rpc.MaxMessageSize` (16 MiB) during reconstruction, fall back to full replay on error |
| R-8 | Hub `SetWriteObserver` loops when hub writes `meta/managed/client/{name}/chain` | Spurious `config-changed` storm | Key basename is `chain` (not `client-*.conf`), so `ClientNameFromConfigKey` returns false; verified by unit test |
| R-9 | Unreadable or unwritable chain store key (`meta/managed/chain-state` or `meta/managed/client/{name}/chain`) | Strict JSON decode or `WriteKey` error | Never silently ignore as absent: log `Warn` under `doctor-managed-chain-unreadable` with fix `ze show data check`, report in `ze doctor`, and allow next full replay to overwrite the chain key cleanly |
| R-10 | Hub diff is larger than full config (e.g., total config replacement) | Wasted wire bytes | `ComputeDiff` sets `worthSending = false` when `len(delta) >= len(target)`; hub serves `Mode: "full"` directly |
| R-11 | `Configuration resync failed` alarm overlaps with `Configuration rejected` or `Configuration diverged` | Duplicate warning alarms for one failure | Client sets `RejectKind: "verify"` on YANG verify refusal (`Configuration rejected` only); a holding client never runs resync and clears any `config-resync-failed` state (`Configuration diverged` only) |
| R-12 | Burst of hub chain store writes during a 2,000-device fleet-wide config push (`fleet-1` scale) | Elevated fsync latency during fleet-wide rollout | Steady-state 30s `ping` performs zero store writes, and `handleConfigAck` / `current` re-anchor skip `Store.WriteKey` whenever the persisted `(Version, Chain)` tuple is already unchanged |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Managed clients fail to apply hub updates, enter resync loops, misclassify resync alarms, or (if hold guard broke) overwrite a local emergency edit. Standalone devices and BGP data plane are unaffected. |
| How is it reverted? | Single commit revert restores full-config-only `config-fetch` and empty `SendOK` replies; persisted `meta/managed/chain-state` and `meta/managed/client/{name}/chain` keys are harmless if unread. |
| Who else touches this path? | `internal/component/plugin/server/managed*.go` and `internal/component/managed/` (shared with `fleet-1` device registry and `fleet-ui-5`/`fleet-ui-8` alarm registry). Key families and structs are kept disjoint from `fleet-1`. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| First boot `ze start` after `ze init --managed` | → | `fetchInitialConfig` -> `cacheFirstConfig` persists `meta/managed/chain-state = FoldFull(B)` | `TestFirstBootPersistsInitialChainAnchor` |
| Hub `ze data import client-<name>.conf` (V1 -> V2) | → | `ManagedServer.HandleConfigFetch` serves `Mode: "diff"` -> client `ApplyDiff` -> `Handler.Validate` -> `CommitOver` -> both ends fold `FoldDiff` | `test/managed/fleet-drift-hash-chain.ci` |
| Periodic `ping` reply with mismatched `chain-hash` | → | `sendPing` -> `driftCh` -> `notificationLoop` -> diff-based resync first, full-replay fallback if diff fails | `TestClientPingMismatchTriggersDiffResyncThenFullFallback` |
| Local emergency edit (`Diverged(active, baseline)`) + mismatched `chain-hash` reply | → | `handleDriftMismatch` checks `Diverged` -> skips auto-resync, stays in `fleet-6`/`fleet-7` hold | `TestChainMismatchRespectsLocalEditHold` |
| `ze show fleet status` and `ze doctor` | → | `readFleetStatus` reports `chain-hash`/`chain-state`/`last-resync`; `checkManagedChain` reports unreadable/drift codes | `TestShowFleetStatusReportsChainFields` |
| A client whose diff resync and full replay both fail, then `show fleet alarm` on the hub | → | The registered `config-resync-failed` kind derives its alarm from the hub's per-client chain state | `test/managed/fleet-resync-failed-alarm.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A connected client that holds no local edit, whose diff resync and full replay both fail (a diff that cannot be built or verified, a replay that is not delivered or whose applied bytes do not reach the hub's chain head) | The hub opens one `Configuration resync failed` alarm (`config-resync-failed`) on that device, Warning, with a sentence naming the device, time, why each step failed, and fix `ze show doctor` (`doctor-managed-chain-drift`); it appears in `show fleet alarm` on every command surface. When a later resync, replay or fetch makes the chain heads agree, it clears. | A replay the client refused at verify (`RejectKind: "verify"`) opens `Configuration rejected` instead, because the client answered and `fleet-ui-5` owns that alarm. A client that holds opens none, because no resync runs for it and `Configuration diverged` says why. |
| AC-2 | The device of AC-1 goes offline while the `Configuration resync failed` alarm is open | The alarm is hidden behind `Device offline` (`fleet-ui-8`, `hidden-behind` in `show fleet alarm`), and on reconnect it is shown again only if the reconnect's resync fails too. | Its history row remains, because the history is the record that it happened. Owed only under choice C-1 (a). |
| AC-3 | Fresh managed client connects for first boot (`version: ""`) and hub serves config `B` (`V1`) | Hub replies `Mode: "full"`, `Version: V1`, `ChainHash: FoldFull(B)`. Client commits `B`, persists `meta/managed/base-version = V1` and `meta/managed/chain-state = {V1, FoldFull(B)}`, sends `config-ack {ok:true, version:V1, chain-hash:FoldFull(B)}`, and hub persists `meta/managed/client/<name>/chain = {V1, FoldFull(B)}`. | None — both ends must converge to the exact same `FoldFull(B)` anchor on first sync. |
| AC-4 | Converged client running `(V1, C1)` receives hub update `V1 -> V2` where `V1` is in hub history and `len(delta) < len(V2)` | Hub computes `delta = ComputeDiff(V1, V2)` and `C2 = FoldDiff(C1, V1, V2, delta)`, serves `Mode: "diff"`. Client reconstructs byte-identical `V2` (`VersionHash` matches), verifies `C2`, runs `Handler.Validate` and `CommitOver`, persists `{V2, C2}`, sends `config-ack {ok:true, chain-hash:C2}`, and hub persists `{V2, C2}` and replies `chain-hash: C2`. | If `len(delta) >= len(V2)` or op count exceeds `MaxDiffOps`, hub serves `Mode: "full"` directly, because sending a diff larger than the full config wastes bandwidth. |
| AC-5 | Every managed RPC reply (`ping`, `config-fetch`, `config-ack`, `config-changed`, `config-held`) while converged | Every reply carries `chain-hash` equal to the sender's expected chain head; matching `ping` replies perform zero store writes on both hub and client and trigger zero resyncs. | An empty `ok` reply from a peer that omits `chain-hash` is accepted without triggering false drift, preserving tolerance per `fleet-config.md`. |
| AC-6 | Hub config changes out-of-band (or `config-changed` was missed) while client is connected on `(V1, C1)` and not locally edited | On the next `ping` reply, hub returns a `chain-hash` differing from `C1`. Client detects the mismatch within one heartbeat interval, signals `notificationLoop` via `driftCh`, sends `config-fetch {resync: "diff"}`, applies the diff, commits `V2`, and converges both chains to `C2` without reconnecting. | None — drift must be detected and healed on the live connection. |
| AC-7 | Hub and client chains disagree (`clientChain != hubChain`) while client's active config `V1` exists in hub history (e.g., dropped `config-ack` or reset hub chain key) and client is not locally edited | When `V1 != V2`, client's `config-fetch {resync: "diff"}` receives `Mode: "diff", Reanchor: true`, verifies `FoldFull(localActive)` locally, applies the diff, persists `meta/managed/chain-state`, and acks `ok:true`. When `V1 == V2`, hub persists `anchor = FoldFull(B)` idempotently and replies `Status: "current", Reanchor: true, ChainHash: anchor`; a non-holding client verifies `FoldFull(localActive) == anchor` and persists `meta/managed/chain-state` without sending `config-ack` (preserving `TestFirstBaselineSendsNoAck`), and the next `ping` confirms both ends agree without transferring the full config. | None when `V1` exists in hub history and `len(delta) < len(V2)`. |
| AC-8 | Diff-based resync cannot succeed — either hub lacks base `V1` in history, or client receives a corrupted/invalid diff whose `VersionHash`, `ChainHash`, `ApplyDiff`, or `Handler.Validate` check fails | Client stages nothing from the failed diff, logs at `Info`, increments `ze_managed_client_resyncs_total{method="diff",result="fallback"}`, and immediately sends ONE `config-fetch {resync: "full"}`. Hub replies `Mode: "full"`, client commits `B`, re-anchors to `FoldFull(B)`, and both ends converge. | None — fallback to full replay is mandatory whenever diff resync fails for any reason. |
| AC-9 | Managed client has a local emergency edit (`Diverged(active, baseline) == true`) and receives a mismatched `chain-hash` on `ping` or any reply | Client does NOT auto-resync and does NOT overwrite its local configuration; it remains in `fleet-6`/`fleet-7` hold (`holding: true`, `config-held` uploaded). Subsequent `adopt` (`Status: "current", Reanchor: true`) or `revert` (`Mode: "full", DiscardVersion: active`) ends the hold and re-anchors both ends' chains to `FoldFull(finalActive)`. | None — `fleet-6`/`fleet-7` hold protection is an invariant. |
| AC-10 | A local commit lands on the client while a diff or full-replay fetch is in flight | `StageMachineCandidateOver` (`activeCheck(decided)`) refuses staging with `errActiveChanged`; client does NOT advance `meta/managed/chain-state` and runs `refetchOverLocalCommit` once, entering hold over the local edit. | None — concurrent local edits must never be overwritten or folded into the hub chain. |
| AC-11 | Full-replay fallback fails validation or commit on the client (e.g., hub holds invalid config or client has an open confirmed-commit window) | Client sends `config-ack {ok:false, reject-kind:...}`, leaves active config and chain unchanged, logs `Warn` under `doctor-managed-chain-drift` with fix command, raises a transient report-bus warning, latches `lastFailedSync = (version, chain)` so subsequent 30s pings do not loop, and retries only when the hub advertises a different `(version, chain)` or on reconnect. | Retrying after reconnect or after a new hub write is permitted and required so recovery happens once the operator fixes the config or the confirm window closes. |
| AC-12 | `meta/managed/chain-state` (client) or `meta/managed/client/<name>/chain` (hub) holds corrupt JSON or fails store read/write | Strict decode fails closed (never silently treated as empty), logs `Warn` under `doctor-managed-chain-unreadable` naming `ze show data check`, and surfaces in `ze doctor` / `show doctor`; a subsequent successful full replay overwrites and heals the unreadable chain key. `show fleet status` displays `chain-hash`, `chain-state`, and `last-resync`, and Prometheus counters record mismatches and diff/full resyncs. | Standalone devices without managed mode never read or write chain keys and incur zero overhead. |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Imports an updated `client-edge-01.conf` on a running hub while `edge-01` is connected | Hub store write -> `SetWriteObserver` -> `config-changed` -> client `config-fetch` -> hub `ListVersions`/`ReadVersion` + `ComputeDiff` -> `Mode: "diff"` -> client `ApplyDiff` -> `Handler.Validate` -> `CommitOver` -> `RecordActiveAsBaseline` + `StoreChainState` -> `config-ack` -> hub persists chain -> `show fleet status` shows `chain-state: in-sync` | `test/managed/fleet-drift-hash-chain.ci` |
| 2 | Hub's stored chain or notification drifts while `edge-01` stays connected | 30s `ping` reply carries mismatched `chain-hash` -> `driftCh` -> `notificationLoop` -> diff-based resync (`reanchor: true`) -> both ends converge without reconnect or full config transfer | `test/managed/fleet-drift-hash-chain.ci` |
| 3 | Hub serves a config whose base is missing or whose diff cannot verify on `edge-01` | Diff resync fails/unavailable -> automatic bounded fallback `config-fetch {resync: "full"}` -> client applies full replay and re-anchors chain -> `show fleet status` reports `last-resync: full-replay` and `chain-state: in-sync` | `test/managed/fleet-drift-hash-chain.ci` |
| 4 | Both diff resync and full replay fail on `edge-01`; operator runs `show fleet alarm` on the hub | Hub records failed resync state -> registered `config-resync-failed` kind opens Warning alarm in `show fleet alarm` (hidden behind `Device offline` if `edge-01` disconnects) -> clears once fault is removed and resync succeeds | `test/managed/fleet-resync-failed-alarm.ci` |
| 5 | Operator leaves fleet, makes an emergency edit on `edge-01`, and rejoins | `Diverged(active, baseline)` is true -> mismatched `chain-hash` on `ping` does NOT overwrite local edit -> `edge-01` holds -> hub `adopt` or `revert` ends hold and re-anchors chains | `TestChainMismatchRespectsLocalEditHold` + existing `test/managed/fleet-divergence-*.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestFoldFullDeterministicAndDiffersByContent` | `pkg/fleet/chain_test.go` | `FoldFull` outputs 40 lowercase hex SHA-1, deterministic, changes on any byte change | |
| `TestFoldDiffChainsOrderAndPayload` | `pkg/fleet/chain_test.go` | `FoldDiff` depends on `prev`, `baseVer`, `targetVer`, and `delta`; rejects invalid `prevHex` | |
| `TestDiffRoundTripByteExactAcrossFormats` | `pkg/fleet/diff_test.go` | `ApplyDiff(A, ComputeDiff(A, B)) == B` for hierarchical, `FormatSet`, `FormatSetMeta`, comments, CRLF, no trailing newline, `$9$` secrets, empty file | |
| `TestComputeDiffSkipsWhenLargerThanTarget` | `pkg/fleet/diff_test.go` | `worthSending == false` when diff >= target bytes or ops > `MaxDiffOps` | |
| `TestApplyDiffRejectsMalformedAndOversized` | `pkg/fleet/diff_test.go` | Rejects unknown ops, negative/OOB indices, >`MaxDiffOps`, output >`maxOutput` | |
| `TestEnvelopeChainAndDiffRoundTrip` | `pkg/fleet/envelope_test.go` | JSON marshal/unmarshal of `ManagedReply`, `PingRequest`, and new `omitempty` fields; `TestManagedReplyToleratesLegacyEmptyOK` | |
| `TestManagedServerResolvesHistoricalBaseForDiff` | `internal/component/plugin/server/managed_chain_test.go` | Hub locates historical `A` from `storage.ListVersions`/`ReadVersion` by `VersionHash` prefix and serves `Mode: "diff"` | |
| `TestManagedServerChainPersistsOnlyOnConfirmedAck` | `internal/component/plugin/server/managed_chain_test.go` | Hub advances `meta/managed/client/<name>/chain` only on `ack.OK` with matching `ChainHash`, never on `ping` (`TestPingMatchPerformsZeroStoreWrites`) | |
| `TestManagedServerReanchorAndFullFallback` | `internal/component/plugin/server/managed_chain_test.go` | Hub serves `Reanchor: true` when chain drifted with known base, and `Mode: "full"` when base is absent or `resync: "full"` | |
| `TestHubChainKeyDoesNotTriggerWriteObserver` | `internal/component/plugin/server/managed_chain_test.go` | `ClientNameFromConfigKey("meta/managed/client/edge-01/chain")` returns false (R-8) | |
| `TestResyncFailedAlarmOpensAndClears` | `internal/component/managed/alarm_resync_test.go` | AC-1: opens when both steps fail on a client that holds nothing; not when client refused replay at verify (`RejectKind: "verify"`), nor when it holds; clears when chain heads agree | |
| `TestResyncFailedAlarmHiddenBehindOffline` | `internal/component/managed/alarm_resync_test.go` | AC-2: `config-resync-failed` registration names `Device offline` as its cause and passes `fleet-ui-8` cause-registry refusal checks | |
| `TestClientDiffApplyAndChainPersistence` | `internal/component/managed/chain_test.go` | Client applies `Mode: "diff"`, verifies `VersionHash` + `ChainHash`, commits through `CommitOver`, persists `meta/managed/chain-state` | |
| `TestClientPingMismatchTriggersDiffResyncThenFullFallback` | `internal/component/managed/client_chain_test.go` | Mismatch on `ping` reply triggers diff resync via `driftCh`; corrupt diff falls back once to full replay and converges | |
| `TestClientFailedFullReplayLatchesNoLoop` | `internal/component/managed/client_chain_test.go` | Failed full replay latches `(version, chain)`, raises `doctor-managed-chain-drift`, does not loop on subsequent pings, retries when hub version changes | |
| `TestChainMismatchRespectsLocalEditHold` | `internal/component/managed/client_chain_test.go` | `Diverged(active, baseline)` prevents auto-resync on ping/reply mismatch; `adopt` and `revert` re-anchor chain | |
| `TestClientDiffResyncPreservesConcurrentLocalCommit` | `internal/component/managed/client_chain_test.go` | Local commit racing diff apply returns `errActiveChanged`, leaves chain untouched, enters hold | |
| `TestDiffWorksWithUnknownExternalPluginSection` | `internal/component/managed/client_chain_test.go` | Config containing an external-plugin section diffs cleanly on hub and validates on client | |
| `TestFirstBootPersistsInitialChainAnchor` | `cmd/ze/ze_core_start_bootstrap_test.go` | `cacheFirstConfig` persists `meta/managed/chain-state` beside `meta/managed/base-version` | |
| `TestShowFleetStatusReportsChainFields` | `internal/component/managed/cmd_fleet_test.go` | `show fleet status` returns `chain-hash`, `chain-state`, `last-resync` (running and stopped) | |
| `TestDoctorReportsChainUnreadableAndDrift` | `internal/component/managed/doctor_test.go` | `doctor-managed-chain-unreadable` and `doctor-managed-chain-drift` checks report cause and exact fix | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `ChainHash` length | 40 hex chars | 40 lowercase hex chars | 39 chars (or non-hex) | 41 chars |
| `DiffOp` count (`MaxDiffOps`) | 0..65536 | 65536 ops | N-A (slice length) | 65537 ops (rejected / full fallback) |
| `DiffOp.Start` / `Count` | `0 <= Start`, `Count >= 1`, `Start+Count <= len(baseLines)` | `Start+Count == len(baseLines)` | `-1` / `0` | `len(baseLines)+1` |
| `ApplyDiff` output bytes (`maxOutput`) | `0..16777216` (`rpc.MaxMessageSize`) | `16777216` bytes | N-A | `16777217` bytes (rejected) |
| `pingTimeout` | `10s` (< `heartbeatInterval = 30s`) | 10s reply succeeds | N-A | >10s stalls and returns deadline error |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `fleet-drift-hash-chain` | `test/managed/fleet-drift-hash-chain.ci` | Two real `ze` daemons (hub + managed client): first boot anchors SHA-1 chain; live config edit delivers diff and advances both chains; simulated chain drift triggers diff-based resync on reply and converges; unresolvable diff falls back to full replay; `show fleet status` confirms `chain-state: in-sync` | |
| `fleet-resync-failed-alarm` | `test/managed/fleet-resync-failed-alarm.ci` | Real hub and client where both resync steps are made to fail: `show fleet alarm` on the hub lists `Configuration resync failed` for the client (hidden behind `Device offline` when the client disconnects), and after the fault is removed and a resync succeeds it is gone; red when the failure reaches only doctor codes | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | N-A | None | Managed fleet channel is Ze's own hub-to-client TLS protocol; no third-party peer implementation exists | N-A |

## Files to Modify
- `pkg/fleet/envelope.go` - add `ManagedReply`, `PingRequest`, and optional `chain-hash`/diff/resync/`reject-kind` fields to existing envelope structs
- `pkg/zefs/keys.go` - register `KeyManagedChainState` (`meta/managed/chain-state`) and `KeyManagedClientChain` (`meta/managed/client/{name}/chain`)
- `internal/component/plugin/server/managed.go` - extend `ManagedConfigService` with historical base reader, hub chain state, and diff/reanchor/full `HandleConfigFetch` + `BuildConfigChanged`
- `internal/component/plugin/server/managed_serve.go` - add per-session pending chain tracking, `SendResult(ManagedReply)` on all replies, chain verification and resync-failure tracking in `handleConfigAck`, mismatch/resync metrics
- `internal/component/plugin/server/managed_doctor.go` - record hub-side chain drift / unreadable client chain state for runtime doctor snapshot
- `internal/component/plugin/server/doctor_codes.go` - declare and register `CodeDoctorManagedChainDrift` (`doctor-managed-chain-drift`) and `CodeDoctorManagedChainUnreadable` (`doctor-managed-chain-unreadable`) once in `doctorDiagnosticCodes`
- `internal/component/plugin/server/register.go` - add `CodeDoctorManagedChainDrift` to `serverEventCodes` (`diagnostic.RegisterEventCode`)
- `internal/component/plugin/doctor/register_external.go` - register hub-side runtime doctor checks declaring `CodeDoctorManagedChainDrift` and `CodeDoctorManagedChainUnreadable`
- `internal/component/plugin/doctor/check_managed_runtime.go` - report hub-side `doctor-managed-chain-drift` and `doctor-managed-chain-unreadable` diagnostics
- `internal/component/managed/client.go` - add `ChainAccess`, optional `Metrics *metrics.Registry`, `pingTimeout`, `driftCh`, joined `sendPing` goroutine, reply chain comparison, diff apply + bounded full-replay fallback, `RejectKind`, failed-sync latch, hold precedence, `report.RaiseWarningFix`/`ClearWarning`
- `internal/component/managed/cmd_fleet.go` - add `chain-hash`, `chain-state`, `last-resync` to `fleetStatus` and `ClientState`
- `internal/component/managed/register.go` - add new columns to `show fleet status` column order, register client `managed-chain` doctor check (referencing `pluginserver.CodeDoctorManagedChainDrift` / `CodeDoctorManagedChainUnreadable`), and register `config-resync-failed` alarm kind (with `Device offline` cause)
- `internal/component/managed/doctor.go` - add client-side `managed-chain` doctor check (`doctor-managed-chain-unreadable`, `doctor-managed-chain-drift`)
- `cmd/ze/hub/managed.go` - wire client chain persistence alongside baseline on commit wiring
- `cmd/ze/hub/managed_owner.go` - wire `ChainAccess` and `OnChainState` callback into `managedClientOwner`
- `cmd/ze/hub/managed_server.go` - wire historical version lookup (`ReadBaseVersion`) over hub `storage.Storage`
- `cmd/ze/ze_core_start.go` - wire `Metrics: registry.GetMetricsRegistry()` and `ChainAccess` into `managedClientConfig`, and persist initial `ChainState` in `cacheFirstConfig`
- `docs/architecture/fleet-config.md` - update Non-Goals (departure), Protocol RPC table, SHA-1 hash chain, diff-based resync, full-replay fallback, store keys, doctor codes, metrics
- `docs/guide/fleet-config.md` - document `show fleet status` chain fields, drift detection, `Configuration resync failed` alarm, and troubleshooting rows
- `docs/architecture/zefs-format.md` - document `meta/managed/chain-state` and `meta/managed/client/{name}/chain`
- `docs/architecture/doctor-and-health-checks.md` - document `doctor-managed-chain-drift` and `doctor-managed-chain-unreadable` checks
- `docs/plugin-development/metrics.md` - document the new hub and client chain mismatch/resync Prometheus metrics
- `docs/guide/command-reference.md` - document new `show fleet status` fields
- `docs/guide/health-checks.md` - add `doctor-managed-chain-drift` and `doctor-managed-chain-unreadable` to Checks and Report Bus Codes tables
- `docs/features/fleet-management.md` - add hash-chain drift detection and diff-based resync to the feature summary
- `features/fleet-management.md` - add `test/managed/fleet-drift-hash-chain.ci` and `test/managed/fleet-resync-failed-alarm.ci` to real-path tests
- `ai/INDEX.md` - add discovery entry for fleet hash-chain drift detection and diff-based resync

## Files to Create
- `pkg/fleet/chain.go` - `ChainHashLen`, `ChainState`, `ValidChainHash`, `FoldFull`, `FoldDiff`
- `pkg/fleet/chain_test.go` - unit tests for SHA-1 chain folding and validation
- `pkg/fleet/diff.go` - `MaxDiffOps`, `DiffOp`, `ComputeDiff`, `ApplyDiff`
- `pkg/fleet/diff_test.go` - byte-exact round-trip, size cutoff, and boundary/hostile input unit tests
- `internal/component/plugin/server/managed_chain.go` - hub-side per-client chain store (`meta/managed/client/{name}/chain`), resync-failure snapshot for alarm derivation, and historical base resolver over `storage.ListVersions`/`ReadVersion`
- `internal/component/plugin/server/managed_chain_test.go` - hub chain persistence, historical base lookup, reanchor, and zero-write ping tests
- `internal/component/managed/chain.go` - client-side `ChainAccess` (`meta/managed/chain-state`), strict decode, and runtime chain tracker
- `internal/component/managed/chain_test.go` - client `ChainAccess` and diff apply unit tests
- `internal/component/managed/client_chain_test.go` - end-to-end client↔server unit tests for ping mismatch, diff resync, full fallback, hold preservation, racing local commit, latch, and `unchecked-plugins`
- `internal/component/managed/alarm_resync.go` - `config-resync-failed` alarm kind registration (`fleet-ui-5`) with `Device offline` cause (`fleet-ui-8`) derived from hub chain snapshot
- `internal/component/managed/alarm_resync_test.go` - `TestResyncFailedAlarmOpensAndClears`, `TestResyncFailedAlarmHiddenBehindOffline`
- `test/managed/fleet-drift-hash-chain.ci` - functional test over real hub and managed client daemons
- `test/managed/fleet-resync-failed-alarm.ci` - functional test proving `show fleet alarm` opens/hides/clears `Configuration resync failed`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No new CLI commands, RPCs, or config leaves; `show fleet status` and `show fleet alarm` (`fleet-ui-5`) already define their schemas |
| YANG validation constraints | N-A | No new YANG leaves added |
| YANG custom validators | N-A | No new YANG leaves added |
| CLI commands/flags | Yes | `internal/component/managed/cmd_fleet.go` and `register.go` extend `show fleet status` output fields and `ColumnOrder` (`chain-hash`, `chain-state`, `last-resync`); `alarm_resync.go` registers `config-resync-failed` into `show fleet alarm` |
| CLI grammar (keyword before value) | N-A | No new command path or argument added |
| Editor autocomplete | N-A | No new command path or YANG leaf added |
| Functional test for new RPC/API | Yes | `test/managed/fleet-drift-hash-chain.ci` and `test/managed/fleet-resync-failed-alarm.ci` exercise real hub and managed client daemons over TLS MuxConn |
| Pipe completeness | Yes | `show fleet status` routes through existing `command.RenderLocalAnswer` / `ApplyPipes` as `ShapeDoc`; `show fleet alarm` routes through `fleet-ui-5`'s command pipeline |
| Env var registration | N-A | No new environment variables; existing `ze.managed.*` vars suffice |
| Doctor check for runtime dependencies | Yes | Store keys `meta/managed/chain-state` and `meta/managed/client/{name}/chain` checked by `managed-chain` (post-config) in `internal/component/managed/doctor.go` (`managed/register.go`) and hub runtime checks registered in `internal/component/plugin/doctor/register_external.go` (`check_managed_runtime.go`) |
| Doctor code per problem class | Yes | `CodeDoctorManagedChainDrift` (`doctor-managed-chain-drift`, warning, runtime + `report.RaiseWarningFix` transient warning when resync/fallback fails to converge) and `CodeDoctorManagedChainUnreadable` (`doctor-managed-chain-unreadable`, warning, post-config + runtime when chain store key fails decode/write), declared and registered once in `internal/component/plugin/server/doctor_codes.go` (plus `serverEventCodes` in `internal/component/plugin/server/register.go`) and referenced by `plugin/server`, `plugin/doctor`, and `managed` |
| Prometheus counters/metrics | Yes | Hub: `ze_managed_chain_mismatches_total`, `ze_managed_chain_resyncs_total{method,result}` (`managed_serve.go`, using `ManagedServerConfig.Metrics`). Client: `ze_managed_client_chain_mismatches_total`, `ze_managed_client_resyncs_total{method,result}` (`internal/component/managed/chain.go`, using `ClientConfig.Metrics` wired from `registry.GetMetricsRegistry()` in `cmd/ze/ze_core_start.go`) |
| BGP family surface (new SAFI / capability / attribute) | N-A | Fleet managed config protocol; no BGP SAFI, capability, or attribute |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/fleet-management.md` (add `test/managed/fleet-drift-hash-chain.ci` and `test/managed/fleet-resync-failed-alarm.ci` to real-path tests) and `docs/features/fleet-management.md` (add hash-chain drift detection and diff-based resync) |
| 2 | Config syntax changed? | No | Verified: no config syntax or YANG `-conf` leaves change |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` (new `show fleet status` fields: `chain-hash`, `chain-state`, `last-resync`) |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/fleet-config.md` (extended fields on the 5 managed channel verbs) |
| 5 | Plugin added/changed? | No | Verified: no new plugin registered; extends existing `managed` component and `ManagedServer` |
| 6 | Has a user guide page? | Yes | `docs/guide/fleet-config.md` (hash-chain drift detection, `show fleet status` fields, `Configuration resync failed` alarm, troubleshooting rows) and `docs/guide/health-checks.md` (`doctor-managed-chain-drift`, `doctor-managed-chain-unreadable`) |
| 7 | Wire format changed? | Yes | `docs/architecture/fleet-config.md` (SHA-1 chain fold, deterministic line diff/patch, reply `chain-hash`, `reanchor`, fallback) |
| 8 | Plugin SDK/protocol changed? | No | Verified: `pkg/plugin/sdk` and `pkg/plugin/rpc` framing are unchanged; only `pkg/fleet` managed payloads change |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | Managed fleet channel is Ze's own protocol; no RFC governs it |
| 10 | Test infrastructure changed? | No | Verified: reuses existing `test/managed/` `.ci` harness and real-daemon fixtures |
| 11 | Affects daemon comparison? | No | Verified: `docs/comparison.md` compares routing protocols, not fleet hash-chain sync |
| 12 | Internal architecture changed? | Yes | `docs/architecture/fleet-config.md`, `docs/architecture/zefs-format.md`, `docs/architecture/doctor-and-health-checks.md`, `ai/INDEX.md` |
| 13 | Route metadata keys added/changed? | N-A | No route metadata keys touched |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md` (four new chain mismatch/resync metrics) |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | Verified: no new plugin, event type, send type, or command path; only fields on existing `show fleet status` and a registered alarm kind under `fleet-ui-5` |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Derived via `./le spec citation anchors`: updated: `docs/architecture/fleet-config.md`, `docs/architecture/zefs-format.md`, `docs/architecture/doctor-and-health-checks.md`, `docs/plugin-development/metrics.md`, `docs/guide/fleet-config.md`, `docs/guide/health-checks.md`, `docs/guide/command-reference.md`, `docs/features/fleet-management.md`; unaffected anchors: `docs/architecture/api/commands.md` (`show fleet status` path and `ShapeDoc` unchanged), `docs/architecture/api/process-protocol.md` (`plugin/server/register.go` adds one event code only, no plugin RPC change), `docs/architecture/system-architecture.md` (boot order unchanged), `ai/rules/repo-maintenance.md` (rule followed as-is), and `cmd/ze/ze_core_start.go` startup guides (`docs/guide/appliance.md`, `configuration.md`, `docker.md`, `operations.md`, `quickstart.md`, `web-interface.md` — `cacheFirstConfig` persists an internal chain anchor only) |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | Verify and update `show fleet status` and RPC examples in `docs/architecture/fleet-config.md`, `docs/guide/fleet-config.md`, and `docs/guide/command-reference.md` |

## Design presentation

### What changes

Managed clients and their hub now continuously verify that they agree on every applied hub-driven configuration change by folding each change into a persistent 40-hex SHA-1 hash chain and comparing the chain head on every managed channel reply (`ping`, `config-fetch`, `config-ack`, `config-changed`, `config-held`). Normal updates and mismatch recoveries transmit a deterministic line-based diff computed from the hub's existing versioned config history first, reconstructing exact bytes so `fleet-6`/`fleet-7` `VersionHash`, local-edit holds, `$9$` secrets, comments, and `unchecked-plugins` external sections keep working untouched, and validating the full reconstructed configuration through YANG before guarded staging and reload. Whenever a diff cannot be built, exceeds the full config size, or fails verification or validation, the client automatically falls back once to a full configuration replay that re-anchors both ends to `FoldFull(B)`. Operators see the chain head, sync state, and last resync method in `show fleet status`, failures in `ze doctor` / `show doctor`, mismatch/resync counts in Prometheus metrics, and persistent non-verify resync failures as a `Configuration resync failed` Warning alarm in `show fleet alarm` (hidden behind `Device offline` under `fleet-ui-8`, choice C-1). Per the 2026-11-23 first-release ruling, all first-release surfaces are commands; web alarm pages remain in `fleet-ui-9`.

### Before and after

1. **Idle drift detection over heartbeat (`ping`):**
   - **Before:** Client sends `ping {}` every 30 seconds with no per-call timeout; hub replies empty `ok`; a missed `config-changed` notification or dropped `config-ack` goes unnoticed until the next hub edit or connection drop.
   - **After:** Client sends `ping {"chain-hash":"<40-hex>"}` with a 10-second timeout; hub replies `ok {"chain-hash":"<40-hex>"}` (zero store writes on match). Any mismatch on a non-holding device immediately triggers diff-based resync on `notificationLoop`, with a bounded full-replay fallback.

2. **Config update delivery (`config-fetch`):**
   - **Before:** Every hub edit sends the entire base64-encoded configuration in `config-fetch`, even for a one-line change.
   - **After:** When the client's prior version exists in the hub's versioned store and the line diff is smaller than the full config, the hub sends `mode: "diff"` with `base-version`, `diff`, and expected `chain-hash`. Client reconstructs exact target bytes, verifies `VersionHash` and `ChainHash`, validates through YANG, commits via `CommitOver`, and advances its chain. Full config replay remains automatic on first boot, revert, missing base, oversized diff, or failed diff verification.

3. **Operator visibility (`ze show fleet status` and `show fleet alarm`):**
   - **Before:** `show fleet status` shows baseline/active hashes and hold state only; a silent sync failure surfaces only if an operator checks logs or doctor on the device.
   - **After:** `show fleet status` adds `chain-hash` (`<40-hex>` or `none`), `chain-state` (`in-sync`, `mismatched`, `uninitialized`, or `no-daemon`), and `last-resync` (`none`, `diff`, or `full-replay`). On the hub, if both diff resync and full replay fail (outside a YANG verify refusal), `show fleet alarm` opens `Configuration resync failed` (Warning), hidden behind `Device offline` if the client disconnects, and clears automatically once chains agree again.

4. **Local emergency edit protection (`fleet-6`/`fleet-7` hold):**
   - **Before:** If active config differs from `meta/managed/base-version`, client holds and never applies hub pushes until operator `adopt` or `revert`.
   - **After:** Identical guarantee preserved — a `chain-hash` mismatch never overwrites a locally edited device (`Diverged(active, baseline)` is checked before any resync, and `StageMachineCandidateOver` guards against racing local commits). `adopt` and `revert` end the hold and re-anchor both chains.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Schema-driven YANG tree / `set`-line deltas (`internal/component/config/`) | `Serialize`/`SerializeSet` does not reproduce raw stored bytes (drops comments/`FormatSetMeta`, re-salts cleartext `$9$` secrets, non-deterministic inline-list map iteration), breaking `fleet-6`/`fleet-7` raw-byte `VersionHash` equality; `SetParser` lacks `ApplyTo`, `activate`, and `UserOrdered` reorder; hub-side YANG parsing fails on `unchecked-plugins`. |
| New dedicated `drift-sync` / `config-diff` RPC verbs | Unnecessary protocol expansion; extending the existing five verbs with optional `omitempty` fields delivers continuous comparison on every reply and preserves clean fallback on older peers. |
| Caching every client's prior config bytes in hub RAM | Wastes memory at `fleet-1`'s 2,000-device scale; hub `storage.ListVersions`/`ReadVersion` already keeps unpruned content-addressed version history on disk. |
| Auto-resync overwriting local device edits on mismatch | Violates `fleet-6`/`fleet-7`'s core safety promise that an emergency edit made on a router is never silently stomped by its hub. |
| Doctor codes and `show fleet status` alone (no hub alarm kind) | Neither reaches an operator who does not ask per device; registering `config-resync-failed` in `fleet-ui-5` puts failed hub repair in `show fleet alarm` with zero edits to any central list. |

### Documents departed from

- **`docs/architecture/fleet-config.md`, section `Non-Goals`:**
  - **Current binding statement:** `"Full config on change; configs are small (< 10KB typically)"`
  - **What the page says after this work:** Removes incremental config updates from `Non-Goals` and documents diff-based config deltas, SHA-1 hash-chain drift detection, and full-replay fallback under `Protocol` (preserving full config replay for first boot, revert, oversized diffs, missing history, and fallback).
- **Not a departure, presented beside it:** `plan/post-release/features/spec-fleet-0-umbrella.md`, section `Scope` (Out of Scope, "Bidirectional config sync"). The sync this spec adds runs strictly hub to client; a local edit still holds and flows up only through `fleet-7`'s operator-gated `adopt`.

### Choices for Thomas

| # | Choice | Options | Decision |
|---|--------|---------|----------|
| C-1 | Does `Configuration resync failed` name `Device offline` as its cause? | (a) recommended, Yes: the kind declares the cause in its own registration, so this spec depends on `fleet-ui-8` for the Causes part, and while a device is offline the operator reads one alarm, since its reconnect starts a new resync; (b) No: the kind names no cause, the dependency on `fleet-ui-8` is dropped, and the alarm stays listed beside `Device offline` | (a) Yes: declare `Device offline` as its cause (depends on `fleet-ui-8`), confirmed 2026-11-23 |
| C-2 | Confirm departure from `docs/architecture/fleet-config.md` `## Non-Goals` (replacing full-config-only with diff-based deltas + SHA-1 chain + full replay fallback) | (a) recommended, Approve departure and update `docs/architecture/fleet-config.md` in the same work; (b) Keep full-config-only and drop diff-based delivery | (a) Approve departure and update `docs/architecture/fleet-config.md` in the same work, confirmed 2026-11-23 |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- register new ZeFS keys, doctor/event codes and checks, `show fleet status` columns, and envelope fields; write failing wiring tests
   - Tests: `TestEnvelopeChainAndDiffRoundTrip`, `TestShowFleetStatusReportsChainFields`, `TestDoctorReportsChainUnreadableAndDrift`, `TestHubChainKeyDoesNotTriggerWriteObserver`, `test/managed/fleet-drift-hash-chain.ci`, `test/managed/fleet-resync-failed-alarm.ci`
   - Files: `pkg/fleet/envelope.go`, `pkg/zefs/keys.go`, `internal/component/plugin/server/doctor_codes.go`, `internal/component/plugin/server/register.go`, `internal/component/plugin/doctor/register_external.go`, `internal/component/plugin/doctor/check_managed_runtime.go`, `internal/component/managed/register.go`, `internal/component/managed/doctor.go`, `internal/component/managed/cmd_fleet.go`
   - Verify: `./le test managed` runs the managed functional tests named above, red until the chain is compared and the alarm is registered

2. **Phase: SHA-1 Hash Chain & Deterministic Line Diff (`pkg/fleet/`)** -- implement `FoldFull`, `FoldDiff`, `ValidChainHash`, `ChainState`, `ComputeDiff`, and `ApplyDiff`
   - Tests: `TestFoldFullDeterministicAndDiffersByContent`, `TestFoldDiffChainsOrderAndPayload`, `TestDiffRoundTripByteExactAcrossFormats`, `TestComputeDiffSkipsWhenLargerThanTarget`, `TestApplyDiffRejectsMalformedAndOversized`
   - Files: `pkg/fleet/chain.go`, `pkg/fleet/chain_test.go`, `pkg/fleet/diff.go`, `pkg/fleet/diff_test.go`, `pkg/fleet/envelope_test.go`
   - Verify: pure stdlib unit and boundary tests pass with `-race`; byte-exact round-trip verified across all Ze config formats

3. **Phase: Hub Chain Store, Historical Base Lookup & ManagedServer** -- implement hub chain persistence (`meta/managed/client/{name}/chain`), historical base lookup over `storage.ListVersions`/`ReadVersion`, diff/reanchor/full `HandleConfigFetch`, reply `ChainHash` on all 5 verbs, `handleConfigAck` chain commit and resync-failure snapshot, and hub metrics/doctor recording
   - Tests: `TestManagedServerResolvesHistoricalBaseForDiff`, `TestManagedServerChainPersistsOnlyOnConfirmedAck`, `TestPingMatchPerformsZeroStoreWrites`, `TestManagedServerReanchorAndFullFallback`
   - Files: `internal/component/plugin/server/managed_chain.go`, `internal/component/plugin/server/managed_chain_test.go`, `internal/component/plugin/server/managed.go`, `internal/component/plugin/server/managed_serve.go`, `internal/component/plugin/server/managed_doctor.go`, `cmd/ze/hub/managed_server.go`
   - Verify: hub unit tests pass under `-race`; zero store writes on matching pings; zero write-observer loops

4. **Phase: Client Chain Access, Ping Drift Trigger, Diff Apply & Fallback** -- implement `ChainAccess` (`meta/managed/chain-state`), `pingTimeout` + `driftCh`, diff verification/apply with bounded full-replay fallback, `RejectKind`, `lastFailedSync` latch, `fleet-6`/`fleet-7` hold preservation, first-boot anchor, status/doctor/metrics wiring
   - Tests: `TestClientDiffApplyAndChainPersistence`, `TestClientPingMismatchTriggersDiffResyncThenFullFallback`, `TestClientFailedFullReplayLatchesNoLoop`, `TestChainMismatchRespectsLocalEditHold`, `TestClientDiffResyncPreservesConcurrentLocalCommit`, `TestDiffWorksWithUnknownExternalPluginSection`, `TestFirstBootPersistsInitialChainAnchor`
   - Files: `internal/component/managed/chain.go`, `internal/component/managed/chain_test.go`, `internal/component/managed/client.go`, `internal/component/managed/client_chain_test.go`, `cmd/ze/hub/managed.go`, `cmd/ze/hub/managed_owner.go`, `cmd/ze/ze_core_start.go`
   - Verify: client unit tests and `-race` pass; existing `test/managed/` suite remains green

5. **Phase: `Configuration resync failed` Alarm Kind (`fleet-ui-5` & `fleet-ui-8`)** -- register `config-resync-failed` with `Device offline` cause (under C-1) reading hub chain snapshot so `show fleet alarm` opens, hides, and clears it
   - Tests: `TestResyncFailedAlarmOpensAndClears`, `TestResyncFailedAlarmHiddenBehindOffline`, `test/managed/fleet-resync-failed-alarm.ci`
   - Files: `internal/component/managed/alarm_resync.go`, `internal/component/managed/alarm_resync_test.go`, `internal/component/managed/register.go`
   - Verify: unit tests and `fleet-resync-failed-alarm.ci` pass; verify refusal opens `Configuration rejected` and holding client opens none

6. **Phase: End-to-End Functional Proof & Documentation** -- draft, stress-prove, and promote `test/managed/fleet-drift-hash-chain.ci` and `test/managed/fleet-resync-failed-alarm.ci`; update all docs in the Documentation Update Checklist
   - Tests: `./le test managed fleet-drift-hash-chain`, `./le test managed fleet-resync-failed-alarm`
   - Files: `test/managed/fleet-drift-hash-chain.ci`, `test/managed/fleet-resync-failed-alarm.ci`, `docs/architecture/fleet-config.md`, `docs/guide/fleet-config.md`, `docs/guide/health-checks.md`, `docs/architecture/zefs-format.md`, `docs/architecture/doctor-and-health-checks.md`, `docs/plugin-development/metrics.md`, `docs/guide/command-reference.md`, `docs/features/fleet-management.md`, `features/fleet-management.md`, `ai/INDEX.md`
   - Verify: functional tests pass under load; `./le doc check all`, `./le arch doctor-coverage check`, `./le arch operator-error check`, and `./le arch fs-persistence check` pass

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-12 has an implementation at file:line |
| Feature completeness | Every user story (1..5) has a working path, no broken links |
| Correctness | `FoldFull` and `FoldDiff` produce identical 40-hex SHA-1 on both ends; `ApplyDiff` preserves exact bytes and `VersionHash`; `Diverged(active, baseline)` blocks every auto-resync; `RejectKind: "verify"` routes to `Configuration rejected` rather than `config-resync-failed` |
| Naming | JSON keys lowercase kebab-case (`chain-hash`, `base-version`, `chain-state`, `last-resync`, `reject-kind`); ZeFS keys registered in `pkg/zefs/keys.go` |
| Data flow | `sendPing` never invokes `fetchAndProcess` or `CommitOver` directly (always signals `driftCh`); hub never caches full client configs in RAM; zero store writes on matching `ping` |
| Operator errors and doctor codes | Every refusal is an `opererr` error with its fix on the last line, every `Warn`/`Error` line and raise names `doctor-managed-chain-drift` or `doctor-managed-chain-unreadable`, and each doctor check carries `Fix` at the phase that holds its state (`ai/rules/user-facing-errors.md`) |
| Rule: `ai/rules/goroutine-lifecycle.md` | No goroutine spawned per ping or per drift event; `driftCh` feeds existing `notificationLoop`; `runConnection` cancels and joins the `sendPing` goroutine before returning so `managedClientOwner.shutdown` joins every ping before the store closes |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| SHA-1 hash chain and deterministic line diff in `pkg/fleet/` | `./le test unit pkg path ./pkg/fleet/` |
| Hub chain persistence, historical lookup, and reply chain hash | `./le test unit pkg path ./internal/component/plugin/server/` |
| Client chain persistence, ping drift trigger, diff resync, full fallback, hold preservation, alarm kind | `./le test unit pkg path ./internal/component/managed/` |
| Functional two-daemon proofs | `./le test managed fleet-drift-hash-chain` and `./le test managed fleet-resync-failed-alarm` |
| Registered ZeFS keys, doctor codes, and clean doc checks | `./le arch doctor-coverage check`, `./le arch fs-persistence check`, `./le doc check all` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | `ChainHash` validated strictly as 40 lowercase hex chars; `DiffOp` count capped at `MaxDiffOps` (65536); `copy` bounds checked against source line count; reconstructed output capped at `rpc.MaxMessageSize` (16 MiB) before allocating |
| Client isolation | Hub always derives client name from authenticated TLS session, never from request payload; hub chain key scoped to authenticated name |
| Secret handling | Diff, full replay, and `config-resync-failed` alarm sentences never log or print config lines or secrets; `doctor-managed-chain-drift`, `config-ack`, and alarm strings carry only hashes, codes, and bounded causes |
| DoS / amplification | `lastFailedSync` latch prevents 30s ping resync loops; hub skips diff when `len(delta) >= len(target)`; hub performs zero disk writes on `ping`; client→hub payloads stay small (compatible with `fleet-1`'s lowered hub read bound) |

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
| Deterministic line-based transport diff in `pkg/fleet/diff.go` with full YANG validation before commit | Schema-driven YANG tree / `set`-line deltas (`internal/component/config/`) | Guarantees byte-exact `VersionHash` round-trip across hierarchical/`FormatSetMeta`/comments/`$9$` secrets, preserves `fleet-6`/`fleet-7`, works with `unchecked-plugins`, and keeps blast radius small |
| Extend the 5 existing fleet verbs with optional `omitempty` fields (`ManagedReply`, `PingRequest`, `ConfigFetchRequest`/`Response`) | Adding new `config-diff` / `chain-sync` RPC verbs | Delivers chain comparison on every reply with zero new verbs and clean tolerance of older peers |
| Re-anchoring diff resync (`Reanchor: true` with `FoldFull(A)` base) before full replay fallback | Always doing full replay whenever chain heads disagree | Heals chain-only drift (e.g., dropped `config-ack`, reset chain key) without transferring full config bytes while proving both ends hold identical `A` |
| Hub resolves historical base config `A` from unpruned `storage.ListVersions`/`ReadVersion` | Maintaining a separate hub snapshot store or per-client RAM cache | Reuses existing content-addressed ZeFS version history with zero extra RAM at 2,000-device scale |
| Preserve `fleet-6`/`fleet-7` local-edit hold (`Diverged(active, baseline)` checked before auto-resync) | Unconditional auto-resync on any chain mismatch | Protects emergency local edits on routers; operator `adopt`/`revert` resolves holds and re-anchors the chain |
| A resync that fails both ways opens a registered `config-resync-failed` alarm kind (`fleet-ui-5` / `fleet-ui-8`) | Doctor codes and `show fleet status` alone | Neither reaches an operator who does not ask; `show fleet alarm` surfaces failed hub repair estate-wide, hides it behind `Device offline`, and requires no edit to any central list |

## Known Limitations
- First boot (empty client version), `fleet-7` operator `revert`, configs whose line diff is at least as large as the full config, and clients whose prior version is absent from the hub's version history use full configuration replay directly rather than a diff.
- Hub-side diff resync for a client whose first import wrote only `file/active/client-<name>.conf` (before a second hub write created a dated version entry in `file/<stamp>/`) falls back to full replay once until a dated version exists; every subsequent hub write has unpruned dated history.

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| Web Alarms tab / Config device-tab binding (`mustRegisterAlarmTab`) for `config-resync-failed` | Thomas ruled on 2026-11-23 that first-release fleet features ship through commands only, and web alarm pages come after the first release (`ai/META-DECISIONS.md`, "Goal and scope") | `plan/post-release/features/spec-fleet-ui-9-tasks-notifications-and-alarm-pages.md` |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | | | |

### Fixes applied
- (filled during `/ze-review` at closure)

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

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
- [ ] AC-1..AC-12 all demonstrated
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
