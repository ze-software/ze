# Spec: fleet-1 -- Device Registry

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | plugin, cli, config |
| Priority | P2 |
| Release reason | Thomas put this fleet capability (the hub's device registry) in the first release; existing supported behavior and its obligations remain required. |
| RFC gaps | - |
| Depends | spec-fleet-7-config-reconnect-resolution (closed 2026-11-19: the held record, `KeyConfigHeld`, `HeldClients`, documented in `docs/architecture/fleet-config.md`, "The hub's copy of a hold") |
| Phase | 1/10 of plan/post-release/features/spec-fleet-0-umbrella.md |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | development tier |
| Updated | 2026-11-23 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

**Re-read after compaction:** this file, `docs/architecture/fleet-config.md`,
`docs/architecture/fleet-interface.md`, `plan/post-release/features/spec-fleet-0-umbrella.md` (its three
Direction Update sections), `plan/post-release/features/spec-fleet-ui-2-estate-tree-and-device-tables.md`,
`spec-fleet-7-config-reconnect-resolution`.

## Task

The hub serves managed clients and remembers nothing about them. It knows which
names are connected right now (`ManagedConfigService.connected`, a set in
memory) and forgets everything at disconnect and at restart. An operator cannot
ask the hub which devices it manages, which are online, what each one runs, or
when each was last seen. The fleet web interface (`fleet-ui`) is a prototype
over a sample estate because no hub answers those questions. The hub also holds
each managed connection under a cap of 128 for its whole life
(`managedMaxConns`), so an estate of the size the fleet set targets cannot even
connect, and a config reload does not change the set of clients the hub serves.

This spec builds the hub's device registry and the surfaces that read it. The
owner's decisions, from the SCOPE, RESEARCH and DESIGN gates of 2026-11-19, are
settled and are not reopened here:

| Topic | Owner decision |
|-------|----------------|
| Placement | The registry lives inside the hub's managed server (`internal/component/plugin/server`). It is written at the one point each fact is known: connect, disconnect, config fetch, config ack. It keeps one record per declared client, persisted in the hub's store under its own key family. A separate component fed by events was rejected: it misses events emitted before it subscribes and adds a second persistence path |
| Commands | `show fleet devices` and `show fleet device` are answered by the daemon, as builtin RPCs with `ze:command` YANG nodes, on every surface |
| Persistence | On connect, disconnect, fetch and ack only. The key family sits outside `meta/config/` and `meta/managed/` |
| Device join | The fleet interface joins per-part facts by device name. Each back end answers only its own fields. A part no back end answers leaves its fields "not reported": a connected device's health is Unknown |
| Data seam | The fleet interface's `Source` contract is split so each back end registers the part it owns. `fleet-1` registers the estate part. A part no back end answers shows a plain "not available on this hub yet" state. The dashboard and the device list go live on real data when `fleet-1` lands |
| Connection cap | Lifted inside `fleet-1`: unauthenticated handshakes are capped (as the plugin acceptor caps them), sessions are bounded by the declared clients and the duplicate refusal, and the managed channel's per-connection read buffer is lowered to what its largest message needs, so the memory per device is stated and tested. The perf test proves 2,000 connected devices |
| Reload | Fixed here. The managed client set follows a config reload: an added client becomes connectable and listed, a removed one has its session closed and its record kept, marked removed. Proven by a `.ci` that adds a client by reload |
| Web page | The `/fleet` pages belong to `fleet-ui`. `fleet-1` keeps the data, the commands and the estate part of the seam |
| Diverged | Owned by `fleet-7`: a device is diverged while `fleet-7`'s held record for it exists. The registry only reads that record |
| Hierarchy | Labels stay flat key=value pairs; the interface builds its tree from labels only. `group` stays the unit that assigns a template |
| Strongest concern carried | Bounding sessions by declared clients moves the limit into the configuration; the read-buffer bound is what makes the worst case stated |

The goal: on a hub with up to 4,096 declared clients and 2,000 of them
connected, an operator runs `show fleet devices` (or opens the fleet dashboard)
and sees every declared device with its connection state, its last-seen time,
its labels and where its configuration stands, across a hub restart and a
config reload, with bounded memory and bounded store writes.

### Superseded from the May skeleton

The skeleton of 2026-05-15 is replaced. What changed and why:

| May skeleton | Now | Why |
|--------------|-----|-----|
| A YANG `fleet { device <name> { label; group; } }` tree | Labels are a list on the hub's existing `client` entry (`plugin hub server <block> client <name> label <key> value <v>`); no `fleet` root | The client entry already declares the device. A second device list would be a second declaration that has to agree with the first (`ai/rules/principles.md`, declare once) |
| `group` declared and filtered by `fleet-1` | Not in `fleet-1`. `Device.Group` and `ConfigFacts.Template` are answered by `fleet-2` | Owner: `group` is the template unit. A group with no template is a label |
| `show fleet devices --group <g> --label <k>=<v>` | `show fleet devices \| label <key>=<value>`, a pipe filter the command declares | A `--flag` never reaches the daemon (`ai/rules/cli.md`), and a command path names the subject while scope is a pipe operation (`ai/META-DECISIONS.md`) |
| `show fleet device detail <name>` | `show fleet device name <name>` | The same leaf grammar `show fleet divergence name <name>` uses (`internal/component/managed/yang/ze-managed-cmd.yang`) |
| AC-13: an undeclared device is auto-discovered | Removed | An undeclared name is refused at authentication (`AuthenticateWithLookup` refuses an empty expected token); dynamic enrollment is out of scope (umbrella, Out of Scope) |
| A `diverged` state set by `fleet-6` and cleared by `fleet-7` | Read from `fleet-7`'s held record | Umbrella Direction Update of 2026-11-19, row "`diverged` record on the hub" |
| A web page `/fleet` with SSE updates, `page_fleet.go` | The estate part of the fleet interface's seam; pages, events and SSE belong to `fleet-ui` | Umbrella Direction Update of 2026-11-18, row "Web views" |
| EventBus events `fleet.device.online/offline` | None | No consumer: the interface's live events are `fleet-ui-5`'s `EventSource` |
| A `DeviceRegistry` wired from `cmd/ze/hub/main.go` into `ManagedConfigService` | The registry is part of `ManagedServer`, and replaces the `connected` set of `ManagedConfigService` | Owner placement A; no layering (`ai/rules/no-layering.md`) |
| Health status in the registry record | Answered by `fleet-4`'s part, joined by name | Owner: each back end answers only its own fields |
| "Client protocol unchanged" | One client change: the config-ack error text is bounded | The lowered read bound must hold every legal message (R-6) |
| Goal gate `./le verify worktree` passes | Removed | No agent runs any `./le verify` command (`ai/META-DECISIONS.md`) |
| The 128-connection cap was not mentioned | Lifted here | Umbrella Direction Update of 2026-11-18, row "Scale" |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/hub-engine.md`, `ai/doors/api-surfaces.md`, `ai/doors/commands-cli.md`, `ai/doors/config-yang.md` - the constraints of every subsystem this spec touches
  → Constraint: runtime state is persisted through the daemon's store under a key registered in `pkg/zefs/keys.go` (`MustRegister`), never `os.WriteFile`.
  → Constraint: `cmd/ze/hub` is always on; the fleet interface is gated `ze_web && ze_fleet_ui`, so the real estate part is linked from a hub file under that build constraint, never imported by always-on code.
  → Constraint: something built only at startup ignores every reload (hub-engine door, Traps). The managed server is built only at startup today, which is the reload defect fixed here.
  → Constraint: one reload path, `doReload` (`cmd/ze/hub/main_reload.go`); the managed client set joins it rather than building a second path.
  → Constraint: a web surface never imports a protocol package to read state; it asks the dispatcher. The estate part reads `show fleet devices` through `web.RouteDeps.Dispatch`.
  → Constraint: a handler answers data, never text; output goes through the pipes; a command-specific filter is declared with `command.RegisterPipeFilters` (`internal/component/command/pipe_filter.go`).
- [ ] `docs/architecture/fleet-config.md` - the managed protocol, the dedicated listener, "What doctor reads from the managed listener", per-client secrets
  → Decision: the registry is a third record of the managed server beside the doctor record (`managedDoctorState`) and the held records (`heldRecords`), published the same way (`ManagedDoctorSnapshot`).
  → Constraint: a client is admitted only on the address of the block that declares it; a client reachable through two blocks is declared under both. The registry keys by name, so one record serves both declarations, and two declarations of one name must carry the same labels.
- [ ] `docs/architecture/fleet-interface.md` - the data source seam, "A source answers facts, the package derives", the failures a source reports
  → Constraint: a source MUST answer an operator error wrapping a `source_errors.go` error where one fits, never an empty answer in its place (`internal/component/web/fleetui/source.go` `Source` doc).
  → Decision: the split keeps derivation in `fleetui` (`DeviceDisplayState`, the tree, the table): a part answers facts only.
- [ ] `docs/architecture/api/commands.md` - where a command is served, builtin RPCs, answer shapes, pipe filters
  → Constraint: the wire-method prefix of a builtin is owned by the package that calls `RegisterRPCs` (`RPCRegistration.registrar`, `OwnerPrefix`), so the `ze-managed:` commands are registered from `internal/component/managed`, which reads the registry through an exported snapshot of `plugin/server`, as `show fleet divergence` reads the held records.
- [ ] `docs/architecture/zefs-format.md` - key families and the store's refusal of a key below a value
  → Constraint: a new key family must not sit below an existing value key; `meta/fleet/` holds no value today (searched `pkg/zefs/keys.go`).
- [ ] `docs/architecture/doctor-and-health-checks.md` - doctor checks, codes, phases
  → Constraint: every problem class gets a doctor check with its cause and the command that fixes it (`ai/META-DECISIONS.md`); the managed runtime checks live in `internal/component/plugin/doctor/check_managed_runtime.go` and read `server.ManagedDoctorSnapshot`.

### RFC Summaries (Scope: protocol)
- N/A: the managed channel is Ze's own protocol; no RFC governs it.

**Key insights:**
- The registry is a record inside `ManagedServer`, written by `handleConn`, `handleRequest` (config-fetch) and `handleConfigAck`, persisted through the borrowed `ManagedServerConfig.Store`, published by an atomic pointer like `managedDoctorSource`.
- The commands live in `internal/component/managed` (owner prefix `ze-managed`) beside `show fleet status` and `show fleet divergence`.
- The `fleetui` slot holds one `Source` today; it becomes one slot per part, the estate part being mandatory for the routes to mount.
- Reload today never touches the managed server; `hubConfig` is extracted once at boot (`cmd/ze/hub/main.go`, `zeconfig.ExtractHubConfig`, then `startManagedServer`).
- Diverged is presence of `meta/config/client-<name>.conf/held` (`HeldRecordKey`), listed by `HeldClients`.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/server/managed.go` - `ManagedConfigService` holds `readConfig`, `records` (the held records) and `connected`, a set of names. `RegisterClient` refuses a name already in the set with `ErrDuplicateClient`; `UnregisterClient` removes it. `HandleConfigFetch` hashes the client's config with `fleet.VersionHash`, answers `current` when the client's version matches (and clears its held record), otherwise the config, `HeldVersion` and `DiscardVersion`.
- [ ] `internal/component/plugin/server/managed_serve.go` - `managedMaxConns` is 128. `acceptLoop` takes a slot of `sem` before authentication and releases it only when `handleConn` returns, so a slot is held for the connection's life. `handleConn` authenticates with `AuthenticateWithLookup` against the block's clients, calls `RegisterClient`, builds `rpc.NewMuxConn(rpc.NewConn(conn, conn))`, records the mux in `conns`, and serves. `handleRequest` answers config-fetch, config-ack (`handleConfigAck`), config-held and ping. `NewManagedServer` fixes `listens` and `served` at construction. `Start` refuses an empty listener set. `Serves` reads the fixed `served` set.
- [ ] `internal/component/plugin/server/managed_doctor.go` - `managedDoctorState` keeps unbound addresses, per-client rejections (bounded by declared names) and the 16 most recent unknown names. `publishManagedDoctor` and `withdrawManagedDoctor` set and clear the atomic pointer `managedDoctorSource`; `ManagedDoctorSnapshot` answers `ErrManagedNotServing` when none is published. `ManagedRejectCause` lists the refusal causes.
- [ ] `internal/component/plugin/server/managed_held.go` (untracked, `fleet-7`'s work in progress) - `HeldRecordKey(name)` is `zefs.KeyConfigHeld` of `ClientConfigKey(name)`, so `meta/config/client-<name>.conf/held`. `HeldClients(store)` lists the names that have a record. `ReadHeldRecord` treats a record that does not decode as an error, never as absent.
- [ ] `cmd/ze/hub/managed_server.go` - `startManagedServer` returns nil when no server block has client entries, builds one `ManagedListener` per serving block, loads the daemon root (`zepki.LoadOrGenerateRoot`), starts the server and installs the store's single write observer, which calls `NotifyConfigChanged` for a written client config key and reads no storage.
- [ ] `cmd/ze/hub/main.go` - `hubConfig` is extracted once (`zeconfig.ExtractHubConfig`); `startManagedServer(managedCtx, store, &hubConfig)` runs after the ready file; a non-nil server installs the data RPC target and `managed.InstallDivergenceTarget`.
- [ ] `cmd/ze/hub/main_reload.go` - `doReload` and `doReloadContext` hold no reference to the managed server or to the hub config; after promotion they publish name servers (`publishNameServers`) and the identity chain. Confirmed: a reload does not rebuild the managed client set.
- [ ] `internal/component/config/storage/store.go` - `SetWriteObserver` holds one function, replaced by each call. `WriteKey` and `RemoveKey` acquire a guard and the observer runs after durability and unlock (`guard.Release`) for every key written, runtime keys included.
- [ ] `internal/core/statestore/statestore.go` - the shared runtime-state handle; strict `Read`, `Write`, `Delete`, `List`. The managed server already holds the same store directly (`ManagedServerConfig.Store`) for the held records.
- [ ] `pkg/zefs/keys.go` - key families registered with `MustRegister(KeyEntry{Pattern, Description, Private})`; `KeyConfigHeld` (uncommitted, `fleet-7`) is `meta/config/{name}/held`.
- [ ] `pkg/plugin/rpc/framing.go` - `NewFrameReader` gives every connection a scanner with a 64 KiB initial buffer (`initialBufSize`) that grows to `MaxMessageSize+1`, 16 MiB. `pkg/plugin/rpc/conn.go` `NewConn` always uses it.
- [ ] `pkg/fleet/envelope.go` - the verbs, `ConfigAck` (`Error` is an unbounded string), `ConfigHeld`, `HeldConfigMax` = 1 MiB raw before base64.
- [ ] `internal/component/managed/client.go` - the client writes `FetchConfig` (config-fetch), `SendConfigAck` (config-ack), `uploadHeld` (config-held, refusing a copy above `HeldConfigMax`), `sendPing`, and answers config-changed in `handleConfigChangedRequest`.
- [ ] `internal/component/managed/register.go` and `yang/ze-managed-cmd.yang` - the fleet-6 and fleet-7 command pattern: `show fleet status` (local data), `show fleet divergence name <name>` as a builtin `ze-managed:fleet-divergence-show` with a completer, `show fleet divergence list`, `command.RegisterShape` and `command.RegisterColumns` per command.
- [ ] `internal/component/managed/cmd_divergence.go` - `DivergenceTarget` and `InstallDivergenceTarget`: the hub installs what the handlers act on.
- [ ] `internal/component/plugin/ipc/tls.go` - the plugin acceptor caps unauthenticated connections at `maxPendingConns` = 32 and releases the slot when the handshake ends.
- [ ] `internal/component/plugin/yang/ze-plugin-conf.yang` - `plugin hub server <name> { ... list client { key name; leaf name (1..64); leaf secret } }`.
- [ ] `internal/component/config/loader_extract.go` - `ExtractHubConfig` builds `plugin.HubConfig`; `checkManagedCertificateAgreement` refuses two serving blocks naming different certificates.
- [ ] `internal/component/web/fleetui/source.go` - `Source` is the union of six interfaces; `sourceSlot` holds one; `MustRegisterSource` panics on a second.
- [ ] `internal/component/web/fleetui/source_devices.go` - `RegistrySource` (`Now`, `Estate`, `Devices`, `Device`, `TreeViews`, `SavedFilters`), `Device` and its fact structs, `DevicesMax` = 4096, `LabelsMax` = 32.
- [ ] `internal/component/web/fleetui/state.go` - `DeviceDisplayState`: offline when not connected, then diverged, down, degraded, and Unknown while `HealthReported` is false. `ConfigState`: in-sync, pending, rejected, diverged, never-fetched.
- [ ] `internal/component/web/fleetui/pages.go` - `ReadSnapshot` fails the whole page when `Alarms`, `Tasks` or `TreeViews` errors.
- [ ] `internal/component/web/fleetui/register.go` - every route mounts while `sourceRegistered`; `mountedSource` panics on an empty slot; a route's `Build` receives `web.RouteDeps`, which carries `Dispatch`.
- [ ] `internal/component/web/fleetui/sample/register/register.go` - the sample registers one source for every concern (`MustRegisterSource(newSource(time.Now))`).
- [ ] `cmd/ze/hub/register_fleet_ui.go` (`ze_web && ze_fleet_ui`) and `register_fleet_ui_sample.go` (`... && ze_fleet_ui_sample`) - the hub-side gates.
- [ ] `cmd/ze/hub/fleet_perf_test.go` - `TestFleetManyClientsPerf` (`fleetperf` tag) drives 128 clients through auth and one config fetch each through `startManagedServer`.
- [ ] `internal/core/events/typed.go` - `events.Register[T]` declares a typed event. Not used here: placement B was rejected.

**Behavior to preserve:**
- Authentication, per-block admission and one session per name (`ErrDuplicateClient` is the refusal; its doctor cause `ManagedRejectNameConnected` stays).
- Every managed RPC and its answer: config-fetch, config-ack, config-held, ping, config-changed push, and the held-record behavior of `fleet-7`.
- The write observer's push of config-changed, and its rule that the callback reads no storage.
- The doctor record and its codes (`doctor-managed-listener-unbound`, `doctor-managed-client-rejected`, `doctor-managed-client-notify-dropped`).
- The metrics `ze_managed_clients_connected`, `ze_managed_config_fetch_total`, `ze_managed_config_changed_pushed_total`, `ze_managed_config_held_total`.
- `show fleet status`, `show fleet divergence`, `show fleet divergence list` and their answers.
- The sample build of the fleet interface: every page of the review build behaves as today.
- The client's 16 MiB read bound: only the hub's reader of the managed channel is lowered.

**Behavior to change:**
- The hub keeps a persistent record per declared client and answers `show fleet devices` and `show fleet device name <name>`.
- `ManagedConfigService.connected` is removed; the registry's live record answers the duplicate refusal.
- The connection cap held for a connection's life is removed; unauthenticated handshakes are capped instead; the hub's managed reader is bounded below 16 MiB; a config-ack error is bounded by the client.
- A client entry carries labels.
- A reload changes the client set the managed server serves, binding and closing serving addresses as needed; the managed server exists whenever the store can serve one, even with no client entry yet.
- The fleet interface's single source slot becomes one slot per part; absent parts are shown as not available; the estate part is answered from the registry on a real build.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Connected-name set | `internal/component/plugin/server/managed.go` `ManagedConfigService.connected`, `RegisterClient`, `UnregisterClient` | Refuses a second session under one name; forgotten at disconnect | Replaces: the registry's live record holds connected state, and the duplicate refusal reads it. Two holders of "connected" would disagree |
| Managed server records | `managed_doctor.go` `managedDoctorState`; `managed_held.go` `heldRecords` | Per-client facts the server keeps, bounded by declared names | Extends: the registry is a third record of the same server, bounded the same way |
| Published snapshot | `managed_doctor.go` `managedDoctorSource`, `ManagedDoctorSnapshot`, `publishManagedDoctor`, `withdrawManagedDoctor` | Lets doctor read the running server's record with no import of the hub | Extends: the registry publishes through the same pointer and an exported `ManagedRegistrySnapshot`; the doctor record gains the registry's write and read failures and the handshake saturation |
| Push coalescing | `managed_serve.go` `pending`, `pushing`, `notifyWake`, `takePending`, `notifyWorker` | A burst of changes for one client becomes one push; bounded by connected names | Extends the pattern: one long-lived record writer with a dirty set, so a burst of events for one device becomes one store write |
| Borrowed store | `ManagedServerConfig.Store`; `managed_held.go` `HeldRecordKey`, `ReadHeldRecord` | The held records are written and read through the hub's store | Extends: the device records use the same borrowed store, no second handle |
| Key registry | `pkg/zefs/keys.go` `MustRegister` | Declares every store key family | Extends: one new family, `meta/fleet/device/{name}/record` |
| Store write observer | `internal/component/config/storage/store.go` `SetWriteObserver`; `cmd/ze/hub/managed_server.go` observer | One function, installed by `startManagedServer`, pushes config-changed for a written client config | Extends: the same function also marks that client's hub version stale in the registry. No second observer: a second `SetWriteObserver` would replace the first |
| Version hash | `pkg/fleet/version.go` `VersionHash` | 16 hex of SHA-256 | Extends: the registry's hub version is `VersionHash` of the client's config, recomputed only when stale |
| Handshake cap | `internal/component/plugin/ipc/tls.go` `maxPendingConns`, `PluginAcceptor.sem` | Caps unauthenticated connections at 32, slot released after the handshake | Extends the pattern to the managed listener, replacing `managedMaxConns` |
| Frame reader | `pkg/plugin/rpc/framing.go` `NewFrameReader`, `MaxMessageSize`; `conn.go` `NewConn` | One 16 MiB bound for every reader | Extends: a constructor with a caller-given maximum; only the hub's managed connections pass a lower one |
| Builtin RPC registration | `internal/component/plugin/server/rpc_register.go` `RegisterRPCs`; `handler.go` `RPCRegistration` (`Complete`, owner prefix) | Daemon-answered commands on every surface | Extends: `ze-managed:fleet-devices` and `ze-managed:fleet-device`, registered from `internal/component/managed/register.go` |
| Fleet command module | `internal/component/managed/yang/ze-managed-cmd.yang` `show fleet` | `status`, `divergence`, `divergence list` | Extends: `show fleet devices` and `show fleet device` in the same container |
| Command-owned pipe filter | `internal/component/command/pipe_filter.go` `RegisterPipeFilters`; precedent `internal/component/bgp/plugins/cmd/rib/rib.go` `registerPipeFilters` | A command declares pipe segments of its own | Extends: `\| label <key>=<value>` on `show fleet devices` |
| Answer shape and columns | `internal/component/command` `RegisterShape`, `RegisterColumns` | Table versus document, column order | Extends: `show fleet devices` is a table, `show fleet device` a document |
| Hub client entry | `internal/component/plugin/yang/ze-plugin-conf.yang` `list client`; `loader_extract.go` `ExtractHubConfig`, `checkManagedCertificateAgreement` | Name and secret per client per serving block; cross-block agreement check | Extends: a `label` list on the entry, validated by YANG and by an agreement check beside the certificate one |
| Reload | `cmd/ze/hub/main_reload.go` `doReloadContext`, `publishNameServers` | The one reload path; publishes post-promotion state | Extends: the managed client set is applied after promotion, beside `publishNameServers`; its extraction is checked before any step applies |
| Held record | `managed_held.go` `HeldClients`, `HeldRecordKey` (`fleet-7`) | The hub's copy of what a holding client runs | Does not apply as a writer: the registry reads presence only, as the owner decided |
| Fleet interface seam | `internal/component/web/fleetui/source.go` `sourceSlot`, `MustRegisterSource`; `register.go` `sourceRegistered` | One source per build, all concerns | Replaces the single slot with one slot per part; the sample registers every part |
| Device display state | `internal/component/web/fleetui/state.go` `DeviceDisplayState` | Derives Unknown while `HealthReported` is false | Extends unchanged: an absent health part leaves `HealthReported` false, which is the owner's "health Unknown" |
| Route deps | `internal/component/web/webroute.go` `RouteDeps.Dispatch` | Gives a route builder the web dispatcher | Extends: the estate part is built over it |
| Perf harness | `cmd/ze/hub/fleet_perf_test.go` `TestFleetManyClientsPerf` | 128 clients, one fetch each | Extends: 2,000 clients held connected at once |
| Typed events | `internal/core/events/typed.go` `Register` | Typed bus events | Does not apply: placement B (a component fed by events) was rejected by the owner |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A managed client's TLS connection to a serving block's listener: `#0 auth`, then newline-framed requests (`config-fetch`, `config-ack`, `config-held`, `ping`) and answers to `config-changed`.
- A config load or reload carrying `plugin hub server <block> client <name> { secret; label <key> value <v>; }`.
- A write of `file/active/client-<name>.conf` (the store's write observer).
- An operator's `show fleet devices [| label <key>=<value>]` or `show fleet device name <name>` on any surface.
- A fleet web page request on a build with `ze_web && ze_fleet_ui`.

### Transformation Path
1. Accept: `acceptLoop` takes a handshake slot, `handleConn` authenticates, then releases the slot. A failed or timed-out handshake releases it too.
2. Admit: the registry refuses a name already connected (`ErrDuplicateClient`), otherwise marks it connected with the time and the remote address, and marks its record dirty.
3. Serve: the hub's reader of the connection is a frame reader bounded by the managed read maximum; a longer line ends the connection and records the rejection cause.
4. Fetch: `handleRequest` config-fetch updates the record with the time, the version the client named and the result (`served`, `current`, `error`), and marks it dirty.
5. Ack: `handleConfigAck` updates the record with the time, the result (`applied`, `refused`, `holding`), the acked version and the error text bounded by `managedDetailMax`, and marks it dirty.
6. Disconnect: the deferred cleanup of `handleConn` marks the record offline with the time and marks it dirty.
7. Persist: one long-lived writer takes a dirty name, encodes its record and writes it under `meta/fleet/device/<name>/record`; a failure is logged, kept for doctor, and the record stays dirty until the next event for that device.
8. Config write: the observer, for a written client config, pushes config-changed (unchanged) and marks that client's hub version stale.
9. Start: the registry lists the stored records, decodes each strictly, and holds every device offline; a record that does not decode is kept as unreadable.
10. Reload: the pre-check extracts the hub config (refusing an invalid one before any step applies); after promotion, the hub applies the serving blocks to the managed server: added names become served, removed names have their session closed and their record marked removed, a changed secret closes the session it authenticated, an added address is bound, a removed address is closed.
11. Answer: `ManagedRegistrySnapshot` copies every declared and stored record, recomputes stale hub versions through `ReadConfig`, and lists the held clients once (`HeldClients`); the `ze-managed` handlers turn it into rows, apply the `label` pipe filter, and answer data.
12. Web: the estate part dispatches `show fleet devices` as the request's account, decodes the rows into `fleetui.Device` facts, and the interface joins any registered device-fact parts by name; an absent part leaves its fields not reported.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Managed client to hub | TLS, `#0 auth`, newline-framed requests on `rpc.MuxConn` | No |
| Managed server to store | `storage.Storage` `WriteKey`, `ReadKey`, `ListKeys` on the borrowed store | No |
| Store to hub (observer) | One write-observer function, run after durability | No |
| Config to managed server | `ExtractHubConfig` at boot and at the reload pre-check; an apply call after promotion | No |
| `plugin/server` to `managed` | Exported snapshot function and types (value copies) | No |
| Daemon to every surface | Builtin RPC through the dispatcher; JSON answer, pipes | No |
| Web to daemon | `web.RouteDeps.Dispatch` with the request's caller identity | No |
| Back ends to fleet interface | One registration per part from `init()`; a join by device name | No |

### Changed-Wire Census

**Search evidence:** scope `internal/`, `pkg/`, `cmd/`, `test/` (Go, `.ci`, `.et`, `.wb`, shell and Python). Terms: `rpc.NewConn(`, `NewFrameReader(`, `NewMuxConn(`, `VerbConfigFetch`, `VerbConfigAck`, `VerbConfigHeld`, `config-held`, `config-fetch`, `config-ack`, `SendConfigAck`, `ConfigAck`, `meta/fleet`. Commands run: a recursive grep for `rpc\.NewConn(\|NewFrameReader(\|NewMuxConn(` over Go in `internal pkg cmd`; for `VerbConfigFetch\|VerbConfigHeld\|VerbConfigAck` over Go; for `config-held\|config-fetch\|config-ack` over `*.sh *.py *.ci *.et *.wb` in `test scripts internal`; for `meta/fleet` over Go and Markdown. Positive control: `managed_serve.go` `handleConn` (a known reader) and `client.go` `FetchConfig` (a known writer) were both found. No shell or Python writer or reader of the managed channel exists; one draft `.ci` (`test/draft/managed/fleet-divergence-upload.ci`) drives it through real daemons. `meta/fleet` has no hit: the key family is new.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| Managed channel request lines | reader | `internal/component/plugin/server/managed_serve.go` `handleConn` (`rpc.NewConn(conn, conn)`) | The only hub reader of the channel | Change: built with the managed read maximum, derived from `fleet.HeldConfigMax` |
| Managed channel request lines | writer | `internal/component/managed/client.go` `FetchConfig`, `SendConfigAck`, `uploadHeld`, `sendPing`, `handleConfigChangedRequest` | Every client message the hub reads | `uploadHeld` preserved (already bounded by `HeldConfigMax`); `SendConfigAck` changed to bound `Error` at `fleet.AckErrorMax`; the others are fixed small payloads, preserved |
| Managed channel request lines | writer | `cmd/ze/hub/fleet_perf_test.go` `fleetInitialSync` | Test client, config-fetch only | Preserved; extended to hold 2,000 sessions |
| Managed channel request lines | writer | `internal/component/plugin/server/managed_serve_test.go`, `managed_held_test.go`, `managed_panic_test.go`, `managed_auth_refused_test.go`; `cmd/ze/hub/managed_server_test.go`, `managed_lifetime_test.go` | Test clients writing the verbs | Preserved: every payload is below the new bound; a new test writes the largest legal config-held and one line over the bound |
| Managed channel request lines | writer | `internal/test/fixture/misc_fixture_managed_fleet_*.go` and `test/managed/*.ci`, `test/draft/managed/fleet-divergence-upload.ci` | Real client daemons | Preserved: they use `client.go` |
| Managed channel answer lines | writer | `internal/component/plugin/server/managed_serve.go` `handleConn` answers on the same `rpc.Conn` (config-fetch, config-ack, config-held, ping) | The only hub writer of answers | Preserved: answers are unchanged; only the hub's read side gains a bound |
| Managed channel answer lines | reader (client side) | `internal/component/managed/client.go` (`rpc.NewConn(conn, conn)` at line 383) | Reads config-fetch answers carrying whole configs | Preserved at 16 MiB: the hub may serve a config larger than `HeldConfigMax` |
| Frame reader bound | writer of the bound (library) | `pkg/plugin/rpc/framing.go` `NewFrameReader`; `conn.go` `NewConn` | Every other caller (`adhoc.go`, `ipc/rpc.go`, `process.go`, `sdk.go`, `ze_core_start.go`, `cli/reload.go`, test engines) | Preserved: they keep `MaxMessageSize`; the new constructor is additive |
| Frame reader bound | reader | Every caller of `NewFrameReader` and `NewConn` other than the managed hub reader: `adhoc.go`, `ipc/rpc.go`, `process.go`, `sdk.go`, `ze_core_start.go`, `cli/reload.go`, the test engines | They keep the library default | Preserved: only the managed hub reader passes the new maximum |
| `config-ack` `error` field | writer | `internal/component/managed/client.go` `SendConfigAck` callers | Today unbounded | Change: truncated to `fleet.AckErrorMax` with a marker that it was cut |
| `config-ack` `error` field | reader | `managed_serve.go` `handleConfigAck` → `recordRejected` (truncates at `managedDetailMax`) | The hub keeps at most `managedDetailMax` | Preserved |
| Device record | writer | New: the registry's record writer (`meta/fleet/device/{name}/record`, JSON) in `internal/component/plugin/server/managed_registry.go` | New family, no prior writer | New |
| Device record | reader | New: the registry's load at `Start`; generic raw readers `ze data` (prints keys and bytes, decodes nothing) | No other reader | New; `ze data` preserved |
| `show fleet devices` answer | writer | New: `internal/component/managed/cmd_devices.go` handler | New command | New |
| `show fleet devices` answer | reader | New: `internal/component/web/fleetui/estate` part; CLI renderers through the pipes; MCP tools derived from the registry | New | New |
| Hub client entry `label` (config) | writer | Operator configs; repository configs under `test/managed/` and `etc/` that declare clients | Additive optional list | Preserved: no existing config carries it |
| Hub client entry `label` (config) | reader | `internal/component/config/loader_extract.go` `ExtractHubConfig`; the YANG parser | New field on `plugin.HubServerConfig` | New |

### Integration Points
- `ManagedServer` (`managed_serve.go`) - gains the registry, the handshake cap, the read bound and an apply method for reload.
- `ManagedConfigService` (`managed.go`) - loses `connected`; `RegisterClient` and `UnregisterClient` move onto the registry.
- `startManagedServer` (`cmd/ze/hub/managed_server.go`) - builds the server whenever the store can serve one, publishes it for reload, extends the observer callback.
- `doReloadContext` (`cmd/ze/hub/main_reload.go`) - checks the hub extraction before any step and applies it after promotion.
- `internal/component/managed/register.go` - registers the two commands, their shapes, columns, pipe filter and completer.
- `internal/component/plugin/doctor/check_managed_runtime.go` - the new checks read the extended doctor record.
- `internal/component/web/fleetui` - one slot per part; `ReadSnapshot` tolerates absent parts; the join.
- `cmd/ze/hub/register_fleet_ui_estate.go` - links the estate part on a real fleet build.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Facts are written by the server at the point they happen; the commands read a published snapshot; the web reads through the dispatcher |
| No unintended coupling (components stay isolated) | Yes | `managed` already imports `plugin/server`; the web part imports only `fleetui` and `web`; `cmd/ze/hub` links the gated part from a gated file |
| No duplicated functionality (extends existing, does not recreate) | Yes | Replaces the connected set; one store, one observer, one reload path, one snapshot pointer |
| Zero-copy preserved where applicable (refs, not copies) | Yes | The snapshot copies value records by design (a cross-boundary value, `ai/doors/system.md`); the hot path (request handling) updates a record in place under a lock |
| Registration over hardcoding, outbound | Yes | Commands, shapes, columns, the pipe filter, the completer and the doctor checks register from `init()`; each fleet-interface part registers from its own package |
| Registration over hardcoding, inbound | Yes | Lists searched for the names this spec adds (`fleet-devices`, `fleet-device`, `label`, the part names): the wire-method snapshot (`internal/component/plugin/all/testdata/wire-methods.snapshot`, regenerated, not hand-kept), `command.AnswerSurface` (derived), the MCP tool list (derived at call time), the doctor registry (registration), `feature-gates.txt` (the estate part sits under the `ze_fleet_ui` row's package prefix). The `ReadSnapshot` part list is the one central enumeration in `fleetui`; it is the seam's own definition of which parts a page reads, owned by `fleetui` |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A reload does not rebuild the managed client set today | `cmd/ze/hub/main_reload.go` holds no reference to the managed server or `ExtractHubConfig`; `hubConfig` is extracted once in `cmd/ze/hub/main.go` | The reload phase would double an existing path | `fleet-devices-reload-add.ci` is red before Phase 6 and green after | unvalidated |
| A-2 | `fleet-7`'s held record (`KeyConfigHeld`, `HeldClients`) lands before this spec's Phase 4 | `fleet-7` is in progress in another session; its files are untracked on 2026-11-19 | Diverged cannot be read; Phase 4 waits | `git log -- internal/component/plugin/server/managed_held.go` before Phase 4 | unvalidated |
| A-3 | No legal client message other than config-held exceeds 64 KiB once the ack error is bounded | `client.go` `FetchConfig` (version only), `sendPing` (empty), config-changed answer (OK), `SendConfigAck` (bounded here) | The bound closes a healthy client | `TestManagedReadMaxAcceptsLargestHeld`, and a read of every `CallRPC` and `Send*` in `client.go` at implementation | unvalidated |
| A-4 | A builtin RPC handler receives its command's declared pipe filter the way a plugin command does | `RegisterPipeFilters` resolves by command path, not by owner (`pipe_filter.go`) | The label filter needs another route | `TestShowFleetDevicesLabelFilter` through the dispatcher | unvalidated |
| A-5 | The web interface runs inside the hub daemon, so the process clock and host name are the hub's | `fleetui` routes are mounted by the daemon's web server through `web.RegisterWebRoute` | `Now` or `Estate` would answer another host's values | `TestEstatePartAnswersHubClock` | unvalidated |
| A-6 | `startManagedServer` receiving a nil store is the only "storage cannot serve" case | `startManagedServer` returns nil on `store == nil`; the comment says "storage is blob-backed" | A hub with client entries on non-blob storage would serve nothing silently | Read the store construction in `cmd/ze/hub/main.go` at Phase 6; if another case exists, it gets the `doctor-managed-store-unavailable` check of the Integration Checklist | unvalidated |
| A-7 | A `.ci` can start a hub, a managed client and a reload in one test | `test/managed/fleet-status.ci`, `fleet-disable-severs.ci` run hub and client daemons | The reload proof needs a Go integration test instead | Writing `fleet-devices-reload-add.ci` | unvalidated |
| A-8 | The browser test runner can start a hub daemon with a serving block and a real managed client beside it once it gains a server kind for it | `internal/test/cli/cmd_web.go` already starts a second process for a server kind (`zeTestStartLGServer` starts `ze-test test peer` beside the daemon); kinds are declared in `internal/component/web/testing/parser.go` (`WBServerLG`, `WBServerDaemon`); `option=needs-build-tag` accepts `state=absent` (`internal/test/runner/build_tags.go`) | The harness shape changes; the test stays a Phase 8 deliverable either way (AC-29) | Writing the `fleet-hub` kind and `test/web/fleet-estate-live.wb` in Phase 8 | unvalidated |
| A-9 | 4,000 file descriptors are available to the perf test process | Linux defaults; macOS soft limit is 256 | The perf test fails on setup, not on the hub | The test raises the soft limit to the hard limit and fails naming both when the hard limit is too low | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Reconnect storm: after a hub restart 2,000 devices connect, fetch and ack within seconds, each event a durable write of two fsyncs, so up to 6,000 writes and 12,000 fsyncs | The perf test's wall clock; `ze_managed_device_records_written_total` rising faster than devices | One writer with a dirty set: events update memory and mark the device; the writer persists one record at a time, so several events for one device before the writer reaches it are one write. Writes are at most one in flight and at most one pending per declared device. A crash loses at most the facts not yet written, which the next event rewrites |
| R-2 | Store write failure (disk full, store read-only) | `WriteKey` error | The in-memory record stays right and answers; the device stays dirty and is retried at its next event; the failure is logged under `doctor-managed-device-record-unwritable` and kept per device; `show fleet device` answers `persisted: false`; doctor reports problem, why (the store's error) and fix |
| R-3 | Reload removes a connected device | The device's session after the reload | The session is closed by the apply, its pending push dropped, its record kept and marked removed with the time; a reconnect is then an unknown name, refused and recorded by the existing doctor record |
| R-4 | Unreadable stored record (corrupt bytes, a field this build does not know) | Strict decode error at `Start` | Never dropped and never read as absent: listed with `unreadable: true` and the device's declared facts; logged under `doctor-managed-device-record-unreadable`; replaced by the device's next event; for a removed device the fix is `ze data rm meta/fleet/device/<name>/record` with the hub stopped, the precedent `HeldRecordKey` documents |
| R-5 | Unbounded concurrent connections after the cap lift | Goroutine count and memory under a connection flood | Unauthenticated handshakes are capped at `managedMaxHandshakes` (32, the acceptor's figure) and each is timed out at `managedAuthTimeout`; authenticated sessions exist only for declared names, one per name, so sessions are at most the declared clients (at most `ManagedClientsMax`, 4,096) |
| R-6 | The lowered read bound closes a healthy client | `doctor-managed-client-rejected` with the oversize cause | The bound is derived from `fleet.HeldConfigMax` (base64 length plus 64 KiB), the ack error is bounded at the client, and a test sends the largest legal config-held |
| R-7 | Worst-case memory: a bufio scanner keeps the buffer it grew to | Heap after many large config-held uploads | Stated: 64 KiB per connection initially, at most the managed read maximum (about 1.4 MiB) after a device uploads its largest copy; the worst case is declared clients times that maximum, written in `docs/architecture/fleet-config.md` and asserted by a test of the reader's maximum |
| R-8 | A reload apply fails part way (an added address does not bind) | `doctor-managed-listener-unbound` | The apply binds independently per address like `Start`; a failed address is recorded unbound and its clients are not connectable there; the rest of the reload stands, as at boot |
| R-9 | A device name containing `/` or matching the client config key form corrupts a key or triggers a push | Key derivation | Names are bounded by YANG (`1..64`); the record key is `meta/fleet/device/<name>/record`, whose basename `record` never satisfies `ClientNameFromConfigKey`, so the observer never mistakes a record write for a config write; a name with `/` is refused at extraction |
| R-10 | Two serving blocks declare one client with different labels | Extraction | Refused at verify with both blocks and the label named, beside `checkManagedCertificateAgreement` |
| R-11 | The sample build and the real estate part both register the estate part | Panic at start | The real part's hub file is constrained `!ze_fleet_ui_sample`; the second registration of a part panics as a composition defect, as today |
| R-12 | `show fleet devices` on a page render reads 4,096 client configs to hash them | Page latency | Hub versions are cached and recomputed only for names the observer marked stale; at `Start` all are stale and are computed once, on the first read |
| R-13 | A secret change by reload leaves a session authenticated with the old secret | Session after reload | The apply closes the session of a client whose secret changed; the client reconnects with the new one |
| R-14 | The estate part dispatches as an identity that may not read `show fleet devices` | 403 on the page | It dispatches as the request's account; a refusal is an operator error the page shows (`FailureUnauthorized`) |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Managed clients refused or disconnected (read bound, handshake cap, reload apply); a hub that no longer starts its managed server; fleet pages that no longer mount |
| How is it reverted? | Single commit revert per phase; the new key family is inert when unread |
| Who else touches this path? | `fleet-7` (held records, `managed.go`, `managed_serve.go`, `keys.go`, uncommitted), `fleet-ui-2` (the seam's other parts, the web mode), `fleet-4`, `fleet-8`, `fleet-9` (their device-fact parts) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A managed client connects and authenticates | → | registry admit in `handleConn` | `test/managed/fleet-devices-connect.ci` |
| The client disconnects | → | registry offline in `handleConn`'s cleanup | `test/managed/fleet-devices-connect.ci` |
| `show fleet devices` on the daemon | → | `ze-managed:fleet-devices` over `ManagedRegistrySnapshot` | `test/managed/fleet-devices-connect.ci` |
| `show fleet device name <name>` | → | `ze-managed:fleet-device` | `test/managed/fleet-device-detail.ci` |
| `show fleet devices \| label site=lon` | → | the declared `label` pipe filter | `test/managed/fleet-devices-label.ci` |
| A reload adding a client | → | the hub's apply after promotion | `test/managed/fleet-devices-reload-add.ci` |
| A reload removing a connected client | → | the apply closes the session and marks removed | `test/managed/fleet-devices-reload-remove.ci` |
| A hub restart | → | the registry's load at `Start` | `test/managed/fleet-devices-restart.ci` |
| A store write failure | → | the record writer's failure path and doctor | `TestRegistryWriteFailureReportedByDoctor` |
| A fleet page on a real `ze_fleet_ui` build (no `ze_fleet_ui_sample`) with one real managed client | → | the estate part over the dispatcher, the join, the not-available component | `test/web/fleet-estate-live.wb` |
| A page asking an unregistered part | → | the not-available component | `TestEveryAbsentPartRendersNotAvailable` |
| 2,000 clients | → | handshake cap, read bound, registry | `TestFleetManyClientsPerf` (`fleetperf`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A declared client connects for the first time | `show fleet devices` lists it with `connected` true, `connected-since` set, `ever-connected` true, its `source-address`; its record is persisted at `meta/fleet/device/<name>/record` | `last-fetch` empty until its fetch arrives, because the fetch is a later fact |
| AC-2 | The client disconnects | Its row shows `connected` false and `last-seen` the disconnect time; the record is persisted | `connected-since` keeps the last session's start, because it is history the detail view shows |
| AC-3 | The client reconnects | `connected` true, a new `connected-since`; `first-connected` unchanged | The previous `last-seen` stays in the record, because it is the last disconnect |
| AC-4 | The client fetches its config | `last-fetch` is the time, `device-version` the version it named, `last-fetch-result` `served` or `current` (`error` with the error when the hub could not read the config), `hub-version` the `VersionHash` of its config | `device-version` empty on a first fetch with no version, because the client named none |
| AC-5 | The client acks | `ack-result` `applied` with `ack-version`, or `refused` with `ack-error` (at most `managedDetailMax` bytes), or `holding` when the hub keeps the copy (`holdShownFromCopy`); `config-state` derives from it (in-sync, pending, rejected) | A held client shows `diverged` instead of `rejected`, because `fleet-7`'s record names the hold |
| AC-6 | The hub restarts with stored records | Every stored device is listed `connected` false with its stored facts until it reconnects | None connected, because no session survives a restart |
| AC-7 | A declared client never connected | It is listed with `ever-connected` false, `config-state` `never-fetched`, its labels and `hub-version` | No record is written for it, because no fact has happened |
| AC-8 | A reload adds a client entry | After the reload the new client is listed (never connected), can authenticate, and once connected is listed connected | Other clients' sessions stay up, because their entries did not change |
| AC-9 | A reload removes the entry of a connected client | Its session is closed within the reload; it is listed with `removed` true and `removed-at`, its stored facts kept; its reconnect is refused as an unknown name and recorded by doctor | Its held record (`fleet-7`) and config file stay, because removing a client entry deletes no operator data |
| AC-10 | `show fleet devices \| label site=lon` | Exactly the devices whose labels include `site=lon`, as rows | Removed devices carry no labels and never match, because their entry is gone |
| AC-11 | `show fleet device name edge-01` | One document: every field of the row plus `first-connected`, `last-ack`, `ack-version`, `ack-error`, `persisted`, `unreadable`, `removed-at` | None |
| AC-12 | `show fleet device name nobody` | Operator error: no client `nobody` is declared or recorded, the fix names `show fleet devices` | None |
| AC-13 | A client whose held record exists (`fleet-7`) | Its row has `diverged` true and `config-state` `diverged`; when the record is removed, `diverged` false on the next answer | An unreadable held record still counts as diverged, because presence is the flag |
| AC-14 | A fleet page on a real `ze_fleet_ui` build with the estate part registered and no other part | The dashboard and devices table show the declared devices from the registry; alarms, tasks, history, device tabs and declared tree views show "not available on this hub yet"; built-in tree views work | The live event stream is absent (the page works without live updates), because `fleet-ui-5` owns the events part |
| AC-15 | A connected device with no health part registered | Its display state is Unknown, never Healthy | None |
| AC-16 | 33 unauthenticated connections held open, then a declared client connects | The client authenticates once a handshake slot frees (at most `managedAuthTimeout`); 129 authenticated sessions coexist | The 33rd unauthenticated connection waits in the backlog, because the cap is the flood bound |
| AC-17 | A client sends config-held of exactly `fleet.HeldConfigMax` raw bytes | Accepted and stored as today | None |
| AC-18 | A client sends a line one byte over the managed read maximum | The hub closes that connection, records `doctor-managed-client-rejected` with the oversize cause, and keeps serving others | The device's record shows the disconnect, because it did disconnect |
| AC-19 | 2,000 declared clients connect at once (`TestFleetManyClientsPerf`) | All 2,000 hold sessions at the same time, each fetched its own config, `ManagedRegistrySnapshot` counts 2,000 connected | Latency figures are logged only, because a figure needs a known host |
| AC-20 | A storm of events for one device before the writer reaches it | One store write carries all of them; across 2,000 devices connecting, fetching and acking, writes never exceed events and never more than one write per device is pending | A crash loses facts not yet written, because coalescing trades that for the bound; the next event rewrites them |
| AC-21 | A store write of a record fails | `show fleet device` answers the right live facts with `persisted` false; `ze doctor` reports `doctor-managed-device-record-unwritable` naming the device, the store error, and the fix; the next successful write clears it | None |
| AC-22 | A stored record does not decode at start | The device is listed with `unreadable` true; `ze doctor` reports `doctor-managed-device-record-unreadable` with the key and the fix; the device's next event replaces the record | None: the record is never dropped silently |
| AC-23 | A client entry declares labels in two serving blocks that differ | The config is refused at verify, naming the client, both blocks and the label | None |
| AC-24 | A config declares more than 4,096 distinct client names | Refused at verify, naming the count and the bound | None |
| AC-25 | A client config file is written | The next `show fleet devices` shows the new `hub-version` for that client and `config-state` `pending` until it acks the new version | Other clients' hub versions are not recomputed, because only the written one was marked stale |
| AC-26 | A hub started with no client entry gains one by reload | The managed server binds the block's address and the client can connect | None |
| AC-27 | The handshake cap is reached | `ze doctor` reports `doctor-managed-handshake-saturated` with the time, the count, the remote addresses holding slots and the fix | None |
| AC-28 | `fleetui.DevicesMax` and the registry's `ManagedClientsMax` | Equal, checked by a test | None |
| AC-29 | `test/web/fleet-estate-live.wb` on a build with `ze_fleet_ui` and without `ze_fleet_ui_sample`, one real managed client `edge-01` connected | The estate devices table shows `edge-01` connected (state Unknown), Config "In sync", label `site=lon`, and its config version hash in the side panel; the estate dashboard's Alarms card shows `fleet-not-available-activity` with "Not available on this hub yet." | No sample marker anywhere, because the sample registers nothing on this build |
| AC-30 | Each part of the split table unregistered, with the estate part registered | Every page, card and section the not-available table names for that part renders "Not available on this hub yet." in `fleet-not-available-<part>`, with status 200, and every other part's content still renders | The event part shows no text, because a page without live updates is its stated absent state |
| AC-31 | A device-fact part registered that answers facts for some estate devices and one name the estate does not hold | Each estate device carries exactly that part's fields for its name; devices it did not answer keep them not reported; the extra name appears nowhere; no field of another part changes | None |
| AC-32 | A second registration of any one part, or a nil part | Panics at process start naming the part | None: it is a build composition defect |
| AC-33 | The sample build (`ze_fleet_ui_sample`) | Every existing `test/web/fleet-*.wb` passes unchanged, and no not-available element appears | None: the sample registers every part |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Lists the fleet after a client connects | client auth → `handleConn` → registry → writer → `show fleet devices` | `fleet-devices-connect.ci` |
| 2 | Adds a client by editing the config and reloading | reload pre-check → promotion → apply → client auth → listed | `fleet-devices-reload-add.ci` |
| 3 | Removes a client by reload | apply → session closed → listed removed | `fleet-devices-reload-remove.ci` |
| 4 | Restarts the hub and sees every device offline | `Start` load → snapshot | `fleet-devices-restart.ci` |
| 5 | Finds the London devices | `show fleet devices \| label site=lon` | `fleet-devices-label.ci` |
| 6 | Opens the fleet dashboard on a real hub | web route → estate part → dispatcher → snapshot | `fleet-estate-live.wb` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRegistryFirstConnectRecordsConnected` | `internal/component/plugin/server/managed_registry_test.go` | AC-1 connected, since, source address | |
| `TestRegistryDisconnectRecordsLastSeen` | same | AC-2 | |
| `TestRegistryReconnectKeepsFirstConnected` | same | AC-3 | |
| `TestRegistryDuplicateRefused` | same | A second admit of a connected name answers `ErrDuplicateClient` | |
| `TestRegistryFetchRecordsVersionAndResult` | same | AC-4 | |
| `TestRegistryAckAppliedRecorded` | same | AC-5 applied | |
| `TestRegistryAckRefusedBoundsError` | same | AC-5 refused, error bounded | |
| `TestRegistryAckHoldingRecorded` | same | AC-5 holding | |
| `TestRegistryLoadMarksEveryDeviceOffline` | same | AC-6 | |
| `TestRegistryListsDeclaredNeverConnected` | same | AC-7 | |
| `TestRegistryUndeclaredStoredRecordListedRemoved` | same | A stored record with no entry is removed | |
| `TestRegistryUnreadableRecordKept` | same | AC-22 listing | |
| `TestRegistryWriterCoalescesPerDevice` | same | AC-20, with a counting store | |
| `TestRegistryWriteFailureKeepsLiveFacts` | same | AC-21 `persisted` false, memory right | |
| `TestRegistryDivergedFromHeldRecord` | same | AC-13 | |
| `TestRegistryHubVersionStaleOnConfigWrite` | same | AC-25 | |
| `TestManagedApplyAddsClient` | `internal/component/plugin/server/managed_apply_test.go` | AC-8 at the server | |
| `TestManagedApplyRemovesConnectedClient` | same | AC-9 at the server | |
| `TestManagedApplySecretChangeClosesSession` | same | R-13 | |
| `TestManagedApplyBindsNewAddress` | same | AC-26 | |
| `TestHandshakeSlotReleasedAfterAuth` | `internal/component/plugin/server/managed_serve_test.go` | AC-16 sessions beyond the cap | |
| `TestHandshakeCapHoldsUnauthenticated` | same | AC-16 the 33rd waits | |
| `TestManagedReadMaxAcceptsLargestHeld` | same | AC-17 | |
| `TestManagedReadMaxClosesOversize` | same | AC-18 | |
| `TestFrameReaderMaxAcceptsExact` | `pkg/plugin/rpc/framing_test.go` | A line of exactly the maximum is read | |
| `TestFrameReaderMaxRefusesLonger` | same | One byte over fails the read | |
| `TestSendConfigAckBoundsError` | `internal/component/managed/client_test.go` | The ack error is cut at `fleet.AckErrorMax` with its marker | |
| `TestExtractHubConfigLabels` | `internal/component/config/loader_extract_test.go` | Labels reach `HubServerConfig` | |
| `TestExtractHubConfigRefusesDisagreeingLabels` | same | AC-23 | |
| `TestExtractHubConfigRefusesTooManyClients` | same | AC-24 | |
| `TestShowFleetDevicesAnswersRows` | `internal/component/managed/cmd_devices_test.go` | Row fields and kebab-case keys | |
| `TestShowFleetDevicesLabelFilter` | same | AC-10 through the dispatcher | |
| `TestShowFleetDeviceAnswersDetail` | same | AC-11 | |
| `TestShowFleetDeviceUnknownName` | same | AC-12 | |
| `TestShowFleetDevicesNotServing` | same | `ErrManagedNotServing` answers an operator error with its fix | |
| `TestCheckManagedDeviceRecordUnwritable` | `internal/component/plugin/doctor/check_managed_runtime_test.go` | AC-21 doctor text | |
| `TestCheckManagedDeviceRecordUnreadable` | same | AC-22 doctor text | |
| `TestCheckManagedHandshakeSaturated` | same | AC-27 | |
| `TestEstatePartRequiredToMount` | `internal/component/web/fleetui/source_parts_test.go` | Routes mount only with the estate part | |
| `TestSecondPartRegistrationPanics` | same | R-11 | |
| `TestReadSnapshotMarksAbsentActivity` | same | AC-14 alarms and tasks not available | |
| `TestReadSnapshotBuiltinViewsWithoutViewsPart` | same | AC-14 tree views | |
| `TestAbsentHealthPartLeavesUnknown` | same | AC-15 | |
| `TestJoinFillsDeviceFactsByName` | same | AC-31: a registered device-fact part fills only its fields | |
| `TestJoinIgnoresNameOutsideEstate` | same | AC-31: a name the estate does not hold appears nowhere | |
| `TestDeviceFactPartErrorLeavesFieldsNotReported` | same | A part answering an error leaves its fields not reported and the page renders | |
| `TestEveryAbsentPartRendersNotAvailable` | same | AC-30, one subtest per part of the split table, each asserting its `fleet-not-available-<part>` element on the pages the table names and status 200 | |
| `TestDeviceSystemJoinsHardwareFromInventoryPart` | same | `DeviceSystem.Hardware` from `InventoryHealthPart`, resources from `DeviceViewPart` | |
| `TestSampleRegistersEveryPart` | `internal/component/web/fleetui/sample/register/register_test.go` | AC-33: every slot holds the sample | |
| `TestEstatePartMapsRowsToDevices` | `internal/component/web/fleetui/estate/estate_test.go` | Row to `fleetui.Device` mapping | |
| `TestEstatePartDispatchErrorIsOperatorError` | same | A refused dispatch is an operator error, never empty | |
| `TestEstatePartAnswersHubClock` | same | A-5 | |
| `TestFleetDevicesMaxAgreesWithManagedClientsMax` | `cmd/ze/hub/fleet_ui_bounds_test.go` (`ze_web ze_fleet_ui`) | AC-28 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Declared client names | 0 to 4,096 | 4,096 | N/A | 4,097 |
| Labels per client | 0 to 32 | 32 | N/A | 33 |
| Label key length | 1 to 63 | 63 | 0 | 64 |
| Label value length | 1 to 128 | 128 | 0 | 129 |
| Managed read line | 0 to managed read maximum | the maximum | N/A | maximum + 1 |
| Concurrent unauthenticated handshakes | 0 to 32 | 32 | N/A | 33 waits |
| Ack error bytes kept by the client | 0 to `fleet.AckErrorMax` | `AckErrorMax` | N/A | `AckErrorMax` + 1 is cut |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `fleet-devices-connect` | `test/managed/fleet-devices-connect.ci` | Connect, fetch, ack, list; disconnect, list offline | |
| `fleet-device-detail` | `test/managed/fleet-device-detail.ci` | Detail document of one device | |
| `fleet-devices-restart` | `test/managed/fleet-devices-restart.ci` | Restart the hub, every device offline with stored facts | |
| `fleet-devices-declared` | `test/managed/fleet-devices-declared.ci` | A declared, never-connected client is listed | |
| `fleet-devices-reload-add` | `test/managed/fleet-devices-reload-add.ci` | Reload adds a client which connects and is listed (red before Phase 6) | |
| `fleet-devices-reload-remove` | `test/managed/fleet-devices-reload-remove.ci` | Reload removes a connected client, closed, listed removed | |
| `fleet-devices-label` | `test/managed/fleet-devices-label.ci` | Label filter | |
| `fleet-devices-diverged` | `test/managed/fleet-devices-diverged.ci` | A holding client is listed diverged | |
| `fleet-estate-live` | `test/web/fleet-estate-live.wb` (`option=server:kind=fleet-hub`, `ze_fleet_ui` present, `ze_fleet_ui_sample` absent) | AC-29: a real managed client listed online with its config version; the Alarms card not available | |
| `TestFleetManyClientsPerf` | `cmd/ze/hub/fleet_perf_test.go` (`fleetperf`) | 2,000 connected devices | |

### Interop Tests (Scope: protocol)
- N/A: the managed channel has no peer implementation other than Ze's own client, which the functional tests run.

## Files to Modify
- `internal/component/plugin/server/managed.go` - remove `connected`; `RegisterClient` and `UnregisterClient` move to the registry.
- `internal/component/plugin/server/managed_serve.go` - handshake cap replacing `managedMaxConns`; registry calls at admit, fetch, ack, disconnect; bounded reader; zero-listener start; apply for reload; `Serves` reads the live served set.
- `internal/component/plugin/server/managed_doctor.go` - record write and read failures per device, handshake saturation, the oversize cause.
- `pkg/plugin/rpc/framing.go`, `pkg/plugin/rpc/conn.go` - a reader and a connection with a caller-given maximum.
- `pkg/fleet/envelope.go` - `AckErrorMax`.
- `internal/component/managed/client.go` - bound the ack error.
- `pkg/zefs/keys.go` - `KeyFleetDeviceRecord`.
- `internal/component/plugin/yang/ze-plugin-conf.yang` - `label` list under the hub `client` entry.
- `internal/component/plugin/types.go` - labels on `HubServerConfig`.
- `internal/component/config/loader_extract.go` - extract labels; refuse disagreeing labels and more than 4,096 names.
- `cmd/ze/hub/managed_server.go` - build when the store can serve, publish for reload, observer marks stale.
- `cmd/ze/hub/main.go` - install the targets whatever the client count.
- `cmd/ze/hub/main_reload.go` - pre-check extraction, apply after promotion.
- `internal/component/managed/register.go`, `internal/component/managed/yang/ze-managed-cmd.yang` - two commands, shapes, columns, filter, completer.
- `internal/component/plugin/doctor/check_managed_runtime.go` - three checks.
- `internal/core/diagnostic/codes_plugin_infra.go` - three codes and their entries.
- `internal/component/web/fleetui/source.go`, `source_devices.go`, `register.go`, `pages.go` - parts, join, not-available markers.
- `internal/component/web/fleetui/sample/register/register.go` - register every part.
- `cmd/ze/hub/fleet_perf_test.go` - 2,000 held sessions.
- `docs/architecture/fleet-config.md`, `docs/architecture/fleet-interface.md`, `docs/guide/fleet-config.md`, `docs/guide/command-reference.md`, `docs/architecture/api/commands.md`, `docs/architecture/zefs-format.md`, `docs/architecture/doctor-and-health-checks.md`, `docs/features/cli-commands.md`, `docs/architecture/api/ipc_protocol.md`, `docs/architecture/hub-architecture.md` - the pages each phase makes wrong.

## Files to Create
- `internal/component/plugin/server/managed_registry.go` - record, writer, load, snapshot.
- `internal/component/plugin/server/managed_registry_test.go`
- `internal/component/plugin/server/managed_apply.go` - the reload apply.
- `internal/component/plugin/server/managed_apply_test.go`
- `internal/component/managed/cmd_devices.go` - the two handlers and the completer.
- `internal/component/managed/cmd_devices_test.go`
- `internal/component/web/fleetui/source_parts.go` - one slot per part and the join.
- `internal/component/web/fleetui/source_parts_test.go`
- `internal/component/web/fleetui/estate/estate.go`, `register.go`, `estate_test.go` - the estate part over the dispatcher.
- `cmd/ze/hub/register_fleet_ui_estate.go` - `ze_web && ze_fleet_ui && !ze_fleet_ui_sample` blank import.
- `cmd/ze/hub/fleet_ui_bounds_test.go`
- `test/managed/fleet-devices-connect.ci`, `fleet-device-detail.ci`, `fleet-devices-restart.ci`, `fleet-devices-declared.ci`, `fleet-devices-reload-add.ci`, `fleet-devices-reload-remove.ci`, `fleet-devices-label.ci`, `fleet-devices-diverged.ci`
- `test/web/fleet-estate-live.wb` - the live-estate browser test (AC-29)
- `internal/component/web/fleetui/component_unavailable.templ` - the not-available component
- `internal/component/web/fleetui/sample/register/register_test.go`

### The device record (persisted, JSON, strict decode)

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | The client name; must equal the key's name |
| `first-connected` | time | First admit ever |
| `connected-since` | time | Start of the last session |
| `last-seen` | time | End of the last session |
| `source-address` | address | Remote address of the last session |
| `last-fetch` | time | Last config-fetch |
| `device-version` | string | Version the client named in that fetch |
| `last-fetch-result` | enum | `served`, `current`, `error` |
| `last-fetch-error` | string | The error, at most `managedDetailMax` |
| `last-ack` | time | Last config-ack |
| `ack-result` | enum | `applied`, `refused`, `holding` |
| `ack-version` | string | Version acked |
| `ack-error` | string | At most `managedDetailMax` |
| `removed-at` | time | When a reload removed the entry; absent while declared |

Live only (never persisted): `connected`, `persisted`, `unreadable`, `hub-version` (cached, stale-marked), `diverged` (read at answer time), `labels` and `declared` (from the configuration).

### The `show fleet devices` row (kebab-case keys, table shape)

Columns in order: `name`, `connected`, `config-state`, `labels`, `hub-version`, `device-version`, `last-fetch`, `last-fetch-result`, `connected-since`, `last-seen`, `source-address`, `ever-connected`, `diverged`, `removed`. The `show fleet device name <name>` document adds the remaining record fields plus `persisted` and `unreadable`.

`config-state` derivation, first match wins:

| Condition | config-state |
|-----------|--------------|
| A held record exists | `diverged` |
| Never fetched | `never-fetched` |
| Last ack `refused` | `rejected` |
| Last ack `applied` and `ack-version` equals `hub-version` | `in-sync` |
| Otherwise | `pending` |

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/managed/yang/ze-managed-cmd.yang` (`show fleet devices`, `show fleet device` with `leaf name`); `internal/component/plugin/yang/ze-plugin-conf.yang` (`list label` under `client`) |
| YANG validation constraints | Yes | `ze-plugin-conf.yang`: label key `pattern` lower-case letter then letters, digits, hyphen, `length 1..63`; value `length 1..128` printable; `max-elements 32`; command `leaf name` `length 1..64` |
| YANG custom validators | Yes | Cross-block label agreement and the 4,096-name bound in `loader_extract.go` (no native constraint spans blocks); completer `Complete` on `ze-managed:fleet-device` from the snapshot's names |
| CLI commands/flags | Yes | `internal/component/managed/register.go`; no flags |
| CLI grammar (keyword before value) | Yes | `show fleet device name <name>`; `\| label <key>=<value>` (`ai/rules/cli.md`) |
| Editor autocomplete | Yes | Label leaves from YANG; device names by the completer |
| Functional test for new RPC/API | Yes | `test/managed/fleet-devices-*.ci`, `fleet-device-detail.ci` |
| Pipe completeness | Yes | Answers are data through `ApplyPipes`; the `label` filter via `command.RegisterPipeFilters` |
| Env var registration | N/A | No env var: the bounds are constants and the labels are config |
| Doctor check for runtime dependencies | N/A | No new path, socket, port, module or binary: the listener addresses are the existing serving blocks, already checked by `doctor-managed-listener-unbound` and `hub-managed-listener` |
| Doctor code per problem class | Yes | Record write failure: `doctor-managed-device-record-unwritable`, runtime phase, `checkManagedDeviceRecordUnwritable`. Unreadable stored record: `doctor-managed-device-record-unreadable`, runtime phase. Handshake cap reached: `doctor-managed-handshake-saturated`, runtime phase. Oversize line: existing `doctor-managed-client-rejected` with new cause `ManagedRejectOversize`. Duplicate session: existing `doctor-managed-client-rejected` (`ManagedRejectNameConnected`). Removed client reconnecting: existing `doctor-managed-client-rejected` (`ManagedRejectUnknownName`). Reload address that does not bind: existing `doctor-managed-listener-unbound`, now also recorded by the apply. Disagreeing labels and too many clients: refusals at verify, whose message carries the fix; no runtime state remains, so no doctor check. `show fleet device` unknown name and not serving: operator errors with their fix; no runtime fault. All in `internal/component/plugin/doctor/check_managed_runtime.go` and `internal/core/diagnostic/codes_plugin_infra.go`. If A-6 finds a non-blob store case, `doctor-managed-store-unavailable` is added there |
| Prometheus counters/metrics | Yes | `ze_managed_clients_declared` (gauge), `ze_managed_handshakes_pending` (gauge), `ze_managed_device_records_written_total{result="ok\|failed"}` (counter), registered in `NewManagedServer` beside the existing four |
| BGP family surface (new SAFI / capability / attribute) | N/A | No BGP change |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | The managed configuration feature file under `features/` (its id read from `./le feature report` at Phase 4) |
| 2 | Config syntax changed? | Yes | `docs/guide/fleet-config.md`, `docs/architecture/config/syntax.md` (client `label`) |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md`, `docs/features/cli-commands.md` |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | N/A | No plugin added; the managed component's commands are documented by rows 3 and 4 |
| 6 | Has a user guide page? | Yes | `docs/guide/fleet-config.md` (registry, labels, reload, scale), `docs/guide/web-interface.md` (real estate, not-available parts) |
| 7 | Wire format changed? | Yes | `docs/architecture/fleet-config.md` "Protocol": hub read bound, ack error bound |
| 8 | Plugin SDK/protocol changed? | Yes | `pkg/plugin/rpc` gains a bounded reader: `docs/architecture/api/process-protocol.md` framing paragraph names that the plugin protocol keeps 16 MiB |
| 9 | RFC behavior implemented, changed, or newly proven? | N/A | No RFC |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` if the perf harness invocation changes (`./le test integration stress-fleet` names 2,000) |
| 11 | Affects daemon comparison? | N/A | No comparison row covers fleet management |
| 12 | Internal architecture changed? | Yes | `docs/architecture/fleet-config.md` (registry, cap, reload), `docs/architecture/fleet-interface.md` (parts, join, not available) |
| 13 | Route metadata keys added/changed? | N/A | No route metadata |
| 14 | Prometheus counters added/changed? | Yes | `docs/guide/monitoring.md` managed metrics section, `docs/plugin-development/metrics.md` (it mentions `managed_serve.go`) |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/guide/status.md` command inventory rows |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Run `./le spec citation anchors spec plan/release/features/spec-fleet-1-device-registry.md` at each phase; the `// Design:` docs of the changed files are `docs/architecture/fleet-config.md`, `docs/architecture/fleet-interface.md`, `docs/architecture/api/commands.md`, `docs/architecture/zefs-format.md`, plus three the anchors report found on 2026-11-19: `docs/architecture/api/ipc_protocol.md` (declared by `pkg/plugin/rpc/framing.go`: updated, the bounded reader is a framing change), `docs/architecture/hub-architecture.md` (declared by `cmd/ze/hub/main.go` and `main_reload.go`: updated, "What the daemon does at start" and reload now carry the managed client set), `ai/rules/repo-maintenance.md` (declared by `internal/component/managed/register.go`: unaffected, it governs the doctor check registration this spec follows without changing the rule). Advisory mentions (`docs/plugin-development/metrics.md` for `managed_serve.go`, `docs/why-ze.md` for `framing.go`, the radius, tacacs and install guides for the hub files) are checked at each phase and edited only where a stated fact changes |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/fleet-config.md` examples of `server central { client ... }` gain a label; `docs/architecture/fleet-config.md` "Roles" and "Per-Client Secrets" tables checked against the YANG |

### Repo-maintenance discovery
| Question | Answer |
|----------|--------|
| Where an agent looks first | `ai/INDEX.md` fleet row and `ai/doors/hub-engine.md` "managed (fleet) client and server" point to `docs/architecture/fleet-config.md`, which gains a "Device registry" section |
| What prevents regression | The `.ci` tests above, `TestEveryCommandOnEverySurface` for the two commands, the doctor code tests |
| What registry prevents drift | `pkg/zefs/keys.go`, the RPC registry and its wire-method snapshot, the doctor registry, the `fleetui` part slots |
| What verification proves it | The owed scoped package tests and `./le test managed` of the listed `.ci` |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** - YANG nodes for both commands; builtin registrations answering a stub; `ManagedRegistrySnapshot` returning `ErrManagedNotServing`; `cmd/ze/hub/register_fleet_ui_estate.go` and an estate part that answers the not-available error
   - Tests: `fleet-devices-connect.ci`, `fleet-device-detail.ci` (red: stub answers)
   - Files: `ze-managed-cmd.yang`, `register.go`, `cmd_devices.go`, `managed_registry.go` skeleton
   - Verify: the commands route on every surface and fail on content
2. **Phase: Registry core** - live record per declared name; admit, fetch, ack, disconnect; replace `connected`
   - Tests: `TestRegistryFirstConnectRecordsConnected` to `TestRegistryAckHoldingRecorded`, `TestRegistryDuplicateRefused`, `TestRegistryListsDeclaredNeverConnected`
   - Files: `managed_registry.go`, `managed.go`, `managed_serve.go`
3. **Phase: Persistence** - key family, coalescing writer, load at `Start`, unreadable records, write failure, doctor codes and checks
   - Tests: `TestRegistryLoadMarksEveryDeviceOffline`, `TestRegistryUndeclaredStoredRecordListedRemoved`, `TestRegistryUnreadableRecordKept`, `TestRegistryWriterCoalescesPerDevice`, `TestRegistryWriteFailureKeepsLiveFacts`, the three doctor tests, `fleet-devices-restart.ci`
   - Files: `keys.go`, `managed_registry.go`, `managed_doctor.go`, `check_managed_runtime.go`, `codes_plugin_infra.go`
4. **Phase: Commands** - rows, detail, label filter, diverged from `HeldClients`, stale hub versions through the observer (needs A-2)
   - Tests: `TestShowFleet*`, `TestRegistryDivergedFromHeldRecord`, `TestRegistryHubVersionStaleOnConfigWrite`, `fleet-devices-label.ci`, `fleet-devices-diverged.ci`, `fleet-devices-declared.ci`
   - Files: `cmd_devices.go`, `register.go`, `managed_server.go`
5. **Phase: Labels in config** - YANG list, extraction, agreement and count refusals
   - Tests: `TestExtractHubConfig*`
   - Files: `ze-plugin-conf.yang`, `types.go`, `loader_extract.go`
6. **Phase: Reload** - always-built server, apply, pre-check, post-promotion apply
   - Tests: `TestManagedApply*`, `fleet-devices-reload-add.ci` (red first), `fleet-devices-reload-remove.ci`
   - Files: `managed_apply.go`, `managed_serve.go`, `managed_server.go`, `main.go`, `main_reload.go`
7. **Phase: Connection bound** - handshake cap, bounded reader, ack error bound, oversize cause, metrics
   - Tests: `TestHandshake*`, `TestManagedReadMax*`, `TestFrameReaderMax*`, `TestSendConfigAckBoundsError`
   - Files: `managed_serve.go`, `framing.go`, `conn.go`, `envelope.go`, `client.go`, `managed_doctor.go`
8. **Phase: Fleet interface parts** - one slot per part, join, not-available markers in `ReadSnapshot` and the pages, sample registers every part, estate part over the dispatcher
   - Tests: `source_parts_test.go` (every test of AC-30 to AC-32), `estate_test.go`, `TestSampleRegistersEveryPart`, `TestFleetDevicesMaxAgreesWithManagedClientsMax`, `fleet-estate-live.wb` (AC-29), the existing sample `.wb` suite unchanged (AC-33)
   - Files: `source.go`, `source_parts.go`, `source_devices.go`, `source_activity.go`, `source_config.go`, `source_history.go`, `source_views.go`, `source_events.go` (interfaces regrouped per part, bounds unchanged), `register.go`, `pages.go`, every page handler that renders a part's answer (the not-available component), a new `component_unavailable.templ`, `sample/register/register.go`, `estate/*`, `internal/component/web/testing/parser.go` and `internal/test/cli/cmd_web.go` (the `fleet-hub` server kind)
   - Verify: the browser test is red before the estate part is registered (the routes do not mount) and green after
9. **Phase: Scale** - 2,000 held sessions with the descriptor limit raised
   - Tests: `TestFleetManyClientsPerf`
   - Files: `fleet_perf_test.go`
10. **Phase: Documentation** - each phase edits its pages as it lands (the Documentation Update Checklist); this phase only checks anchors

### The fleet interface parts after the split

The single `Source` of `internal/component/web/fleetui/source.go` becomes one
interface per back end, each held by its own slot in `source_parts.go` and
registered from `init()` by one call per part (`MustRegisterEstatePart`,
`MustRegisterViewsPart`, and so on, one name per row below). A nil part or a
second registration of one part panics, as `MustRegisterSource` does today.
`MustRegisterSource` is deleted, not kept beside the parts (no layering).
Method names, argument lists, answer types and bounds are the ones
`source_*.go` declares today (read on 2026-11-19); the split moves them, it
does not change them, so every page handler keeps its calls. The sample
(`internal/component/web/fleetui/sample/source.go`, one `estate` value that
answers every method today) registers itself as every part from
`sample/register/register.go`. The owner column names each spec by path.

| Part (interface) | Method: inputs → answer, bound | Owner |
|------------------|-------------------------------|-------|
| `EstatePart` | `Now(ctx)` → `time.Time`; `Estate(ctx)` → `Estate`; `Devices(ctx)` → `[]Device`, at most `DevicesMax` (4,096), each with `Labels` at most `LabelsMax` (32); `Device(ctx, name)` → `Device`, an unknown name is `DeviceUnknownError`. The estate part fills only `Name`, `Labels`, `SecretSet`, `Connection.Connected`, `Since`, `EverConnected`, `LastSeen`, `SourceAddr`, and `Config.State`, `HubVersion`, `DeviceVersion`, `Fetched`, `LastFetch`, `LastFetchResult` | `plan/release/features/spec-fleet-1-device-registry.md` |
| `ViewsPart` | `TreeViews(ctx)` → `[]TreeView`, at most `TreeViewsMax` (32), each at most `TreeLevelsMax` (8) levels; `SavedFilters(ctx, user)` → `[]SavedFilter`, at most `SavedFiltersMax` (64) | `plan/post-release/features/spec-fleet-ui-2-estate-tree-and-device-tables.md` |
| `InventoryHealthPart` (device facts) | `InventoryHealth(ctx)` → one entry per device name holding `Inventory`, `HealthReported`, `Health` (at most `HealthComponentsMax`, 32) and the hardware list of `DeviceSystem.Hardware` (at most `HardwareItemsMax`, 64), at most `DevicesMax` entries; `InventoryHealthOf(ctx, name)` → the same entry for one device, an absent device answering the entry with `Reported` and `HealthReported` false | `plan/release/features/spec-fleet-4-inventory-health.md` |
| `LatestSamplePart` (device facts) | `LatestSamples(ctx)` → one `LatestSample` per device name, at most `DevicesMax`; `LatestSampleOf(ctx, name)` → `LatestSample` | `plan/release/features/spec-fleet-8-metric-history.md` |
| `RelayGrantPart` (device facts) | `RelayGrants(ctx)` → per device name `RelayGranted` and `RelayProfile`, at most `DevicesMax`; `RelayGrantOf(ctx, name)` → the same for one | `plan/post-release/features/spec-fleet-9-device-command-relay.md` |
| `AssignmentPart` (device facts) | `Assignments(ctx)` → per device name `Group` and `Config.Template`, at most `DevicesMax`; `AssignmentOf(ctx, name)` → the same for one | `plan/post-release/features/spec-fleet-2-config-templates.md` |
| `HistoryPart` | `DeviceSeries(ctx, device, metric Metric, span TimeSpan)` → `Series`, at most `SeriesPointsMax` (20,160) points; `EstateSeries(ctx, devices []string, metric, span)` → `Series`, at most `SeriesPointsMax`, `devices` at most `DevicesMax`; `HistoryUsage(ctx)` → `HistoryUsage`, `Largest` at most `HistoryDevicesMax` (10) | `plan/release/features/spec-fleet-8-metric-history.md` |
| `ActivityPart` | `Alarms(ctx)` → `[]Alarm`, at most `AlarmsMax` (1,024); `AlarmHistory(ctx, device)` → `[]AlarmEvent`, at most `AlarmHistoryMax` (512), newest first; `Tasks(ctx)` → `[]Task`, at most `TasksMax` (512), newest first; `Task(ctx, id)` → `TaskDetail`, each answer at most `TaskAnswerBytesMax` (16,384), an unknown id wraps `ErrObjectUnknown` | `plan/release/features/spec-fleet-ui-5-tasks-alarms-notifications.md` |
| `DeviceConfigPart` | `DeviceConfig(ctx, device)` → `DeviceConfig`, `Text` at most `ConfigTextBytesMax` (1 MiB) | `plan/post-release/features/spec-fleet-2-config-templates.md` (the source comment names `fleet-1` and `fleet-2`; the owner gave `fleet-1` the estate part only, so `fleet-2`, which renders what the hub serves, owns it) |
| `ConfigHistoryPart` | `ConfigVersions(ctx, device)` → `[]ConfigVersion`, at most `ConfigVersionsMax` (100), newest first; `ConfigText(ctx, device, version)` → text, at most `ConfigTextBytesMax`, an unknown version wraps `ErrObjectUnknown`; `AuditEntries(ctx)` → `[]AuditEntry`, at most `AuditEntriesMax` (1,000), newest first | `plan/post-release/features/spec-fleet-3-audit-trail.md` |
| `DivergencePart` | `Divergence(ctx, device)` → `Divergence`, texts at most `ConfigTextBytesMax`; a device that is not diverged wraps `ErrObjectUnknown` | `spec-fleet-7-config-reconnect-resolution` |
| `RolloutPart` | `Rollouts(ctx)` → `[]Rollout`, at most `RolloutsMax` (128), newest first; `Rollout(ctx, name)` → `RolloutDetail`, at most `RolloutBatchesMax` (100) batches, an unknown name wraps `ErrObjectUnknown` | `plan/post-release/features/spec-fleet-5-staged-rollout.md` |
| `TemplatePart` | `Templates(ctx)` → `[]Template`, at most `TemplatesMax` (256) | `plan/post-release/features/spec-fleet-2-config-templates.md` |
| `AccessPart` | `AccessProfiles(ctx)` → `[]AccessProfile`, at most `AccessProfilesMax` (64) | `plan/post-release/features/spec-fleet-ui-6-config-rollout-audit-admin.md` (the source comment names `fleet-1`; the access administration page is `fleet-ui-6`'s) |
| `SettingsPart` | `Settings(ctx)` → `Settings` | `plan/release/features/spec-fleet-ui-5-tasks-alarms-notifications.md` |
| `DeviceViewPart` | `DeviceSystem(ctx, device)` → `DeviceSystem` (`Resources`, `Time`; `Hardware` comes from `InventoryHealthPart` by the join); `DeviceInterfaces(ctx, device)` → `[]Interface`, at most `InterfacesMax` (1,024); `DevicePeers(ctx, device)` → `[]Peer`, at most `PeersMax` (4,096); `DeviceFamilies(ctx, device)` → `[]FamilySummary`, at most `FamiliesMax` (64); `DeviceAdjacencies(ctx, device)` → `[]ProtocolAdjacencies`, at most `ProtocolsMax` (16) of at most `AdjacenciesMax` (1,024) rows; `DeviceLogs(ctx, device)` → `[]LogLine`, at most `LogLinesMax` (500), newest first; `Console(ctx, device, user, command)` → `ConsoleAnswer`, at most `ConsoleOutputBytesMax` (65,536) | `plan/post-release/features/spec-fleet-9-device-command-relay.md` |
| `EventPart` | `Subscribe(ctx)` → `*Subscription`, at most `EventSubscriptionsMax` (64) at once, each holding at most `EventBufferSize` (64) unread events; beyond the maximum wraps `ErrStreamsFull` | `plan/release/features/spec-fleet-ui-5-tasks-alarms-notifications.md` |

**The join by device name.** `fleetui` answers a page's `Devices` and
`Device` by asking the estate part first: its answer is the set of devices
and their order, and a device the estate does not hold is not a device of the
page. For each registered device-fact part, it asks the batch method once per
request (or the `...Of` method for one device) and copies into each device
exactly the fields the table gives that part, matched by `Name`. A name a part
answers that the estate does not hold is ignored, because the estate is the
set; a device a part does not answer keeps that part's fields at their zero
value, which every fact struct already reads as "not reported"
(`Inventory.Reported`, `HealthReported`, `LatestSample.Reported` false,
`RelayGranted` false, `Group` and `Template` empty). No field is written by
two parts. A part that is not registered leaves its fields not reported for
every device; a registered part that answers an error leaves them not
reported and the page shows that part's error text in place of its fields
(an operator error, `FailureOf`), never failing the page. `DeviceDisplayState`
is unchanged, so a connected device with no health part is Unknown.

**The not-available state.** While no back end has registered a part, every
place that shows that part's answer renders one component instead: the text
"Not available on this hub yet." in `--text-secondary`, inside an element
whose id is `fleet-not-available-<part>` (`<part>` is the part's kebab-case
name: `views`, `inventory-health`, `latest-sample`, `relay-grant`,
`assignment`, `history`, `activity`, `device-config`, `config-history`,
`divergence`, `rollout`, `template`, `access`, `settings`, `device-view`,
`events`). It is a state of the page, not an error: the HTTP status stays 200,
and `ReadSnapshot` records which parts are absent instead of failing. Where
each part shows:

| Part | Pages, cards and sections that show its not-available state |
|------|--------------------------------------------------------------|
| `ViewsPart` | The tree view menu lists the built-in views only, with the declared section showing the state; the saved filter menu of every table; the tree views and filters administration pages |
| `InventoryHealthPart` | The Software card of the estate and group dashboards; the Version column cell of the devices table shows a dash; the Identity and Health sections of the side panel; the hardware section of the System tab |
| `LatestSamplePart` | The Busiest devices card; the CPU and Memory cells of the devices table show a dash; the Now section of the side panel |
| `RelayGrantPart` | The relay grant row of the side panel's Connection section |
| `AssignmentPart` | The group level of a tree view and the template row of the Config tab show a dash |
| `HistoryPart` | Every chart of the estate, group and device dashboards; the history administration page |
| `ActivityPart` | The Alarms card; the alarm count of the Needs attention card; the alarm badges of the tree; the Alarms tabs; the task panel, the Tasks tabs and the Tasks page; the header task count; the Open alarms section of the side panel |
| `DeviceConfigPart` | The device Config tab |
| `ConfigHistoryPart` | The history section of the Config tab; the Recent changes card; the audit administration page and the audit section of the device Tasks tab |
| `DivergencePart` | The divergence banner and diff of the device Dashboard and Config tabs (the diverged state itself still comes from the estate part) |
| `RolloutPart` | The Rollouts card; the estate Rollouts tab and the rollout page; the rollout count of the Needs attention card |
| `TemplatePart` | The templates administration page |
| `AccessPart` | The access administration page |
| `SettingsPart` | The settings administration page |
| `DeviceViewPart` | The System (resources and clock), Interfaces, Routing, Logs and Console tabs of a device |
| `EventPart` | Not shown as text: the page works without live updates, as it does today when the stream answers 503 |

The estate part is the one part without a not-available state: the fleet
routes mount only while it is registered (`sourceRegistered` reads its slot),
and the two prototype-only routes mount only while the event part is a
`Stepper`.

**The estate part on a real build.** `internal/component/web/fleetui/estate`
registers it from `init()` with a builder over `web.RouteDeps.Dispatch`,
built once when the first fleet route is built. `Devices` dispatches
`show fleet devices`, `Device` dispatches `show fleet device name <name>`, as
the request's account; a refusal or an error is an operator error wrapping
the `source_errors.go` error that fits. `Now` is the hub process's clock and
`Estate.Name` the hub's host name (A-5); a host name that cannot be read is an
operator error, never an empty name. `cmd/ze/hub/register_fleet_ui_estate.go`
links it under `ze_web && ze_fleet_ui && !ze_fleet_ui_sample`.

**What stays with `fleet-ui-2`.** The views part, the filter grammar over the
estate rows, the sort and page of the tables, the web mode switch, sign-out
and the CI stage for the fleet browser tests (its "Carried from the
prototype" table). Its row "Register the real source, built over the command
dispatcher" is amended (append-only, 2026-11-19) to say that `fleet-1` splits
the contract and registers the estate part.

**The live-estate browser test (A-8 made a deliverable).** The browser test
runner gains a server kind, `fleet-hub` (`WBServerFleetHub` in
`internal/component/web/testing/parser.go`, started by a new
`zeTestStartFleetHubServer` in `internal/test/cli/cmd_web.go`, the way
`zeTestStartLGServer` starts a peer beside the daemon). It starts the hub
daemon with web enabled and a serving block declaring one client, `edge-01`,
with a fixed secret, the label `site lon`, and a fixed client config written to
`file/active/client-edge-01.conf`; then it starts a second `ze` as the managed
client `edge-01` against that block and waits until the hub answers
`show fleet device name edge-01` with `connected` true and `ack-result`
`applied`. `test/web/fleet-estate-live.wb` carries these option lines:

| Option line | Why |
|-------------|-----|
| `option=needs-build-tag:value=ze_fleet_ui:state=present` | The fleet routes are linked |
| `option=needs-build-tag:value=ze_fleet_ui_sample:state=absent` | A real build: the sample would register every part |
| `option=server:kind=fleet-hub` | The hub with one real managed client |
| `option=timeout:value=90s` | Two daemons and a TLS handshake |
| `option=viewport:width=1600:height=1000` | The devices table shows every column |

It opens `/fleet/` and then the estate Devices tab, and expects: a row named
`edge-01`; its state cell "Unknown" (connected, no health part registered);
its Config cell "In sync"; its Labels cell `site=lon`; the side panel's
Connection section naming the client config's version hash, the 16 hex
characters `fleet.VersionHash` gives for the fixed config (computed when the
test is written and stated in its header with the bytes it hashes); on the
estate dashboard, the element `fleet-not-available-activity` inside the Alarms
card with the text "Not available on this hub yet."; and no element with the
sample marker "Sample data". The fleet browser tests run in CI once
`fleet-ui-2`'s verifier stage builds the fleet tags; until then the main
thread runs this one locally as an owed command.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | Connected state has one holder; a record write never blocks a request; the observer still reads no storage; a removed client's held record and config survive |
| Naming | JSON keys kebab-case; codes `doctor-managed-*`; wire methods `ze-managed:fleet-devices`, `ze-managed:fleet-device` |
| Data flow | Facts written only in `handleConn`, `handleRequest`, `handleConfigAck` and the apply; the web reads only through the dispatcher |
| Operator errors and doctor codes | Every refusal is an `opererr` error with its fix on the last line, every `Warn`/`Error` line names the code the Integration Checklist lists, and each doctor check carries `Fix` |
| Rule: goroutine lifecycle | One long-lived record writer started by `Start`, ended by `Stop`; no goroutine per event |
| Rule: no layering | `ManagedConfigService.connected` and `managedMaxConns` deleted, not kept beside the replacements |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Registry in the managed server | `gopls symbols internal/component/plugin/server/managed_registry.go` |
| Key family | `grep -n KeyFleetDeviceRecord pkg/zefs/keys.go` |
| Commands on every surface | `TestEveryCommandOnEverySurface` |
| Reload follows the client set | `fleet-devices-reload-add.ci`, `fleet-devices-reload-remove.ci` |
| Cap lifted, memory bounded | `TestHandshake*`, `TestManagedReadMax*`, `TestFleetManyClientsPerf` |
| Estate part live | `fleet-estate-live.wb` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Label keys and values by YANG; device names by YANG and at extraction; strict record decode; the read bound on every hub-read line |
| Resource exhaustion | Handshake cap; sessions bounded by declared names; writer pending set bounded by declared names; unknown-name record bounded at 16 (existing) |
| Error leakage | The ack error and record errors are bounded and never carry a secret; `source-address` is shown only to callers allowed to read `show fleet` |
| Authorization | Both commands are read-only builtins under the dispatcher's authorization; the estate part dispatches as the request's account |
| Secret rotation | A secret changed by reload ends the session it authenticated |

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
| Registry inside `ManagedServer` (owner, placement A) | A component fed by events (B) | B misses events emitted before it subscribes and adds a second persistence path |
| Coalescing writer | A synchronous write per event | Bounds a reconnect storm (R-1) and keeps the store off the request path |
| Labels on the client entry | A `fleet { device }` root | One declaration of a device |
| `\| label` pipe filter | `--label` flag, a `label` argument | Flags are offline only; scope is a pipe operation |
| Commands as `ze-managed` builtins over a snapshot | Builtins in `plugin/server` | The owner prefix belongs to the registering package; `managed` already holds the fleet commands |
| Daemon-only commands, no offline fallback (owner, final gate 2026-11-19) | A local route answering stored records as offline | Every fact but the stored one is in the daemon; an offline answer would show a fleet that looks dead |
| Removed devices are listed by the CLI only; the interface's estate part lists declared devices only (owner, final gate 2026-11-19) | Removed devices in the interface | `fleetui` has no removed state; adding one is `fleet-ui-2`'s design |
| A serving certificate name change needs a restart (owner, final gate 2026-11-19) | Apply the certificate at reload | Deferred by the owner; recorded in Known Limitations |
| `group` and `template` stay with `fleet-2` (owner, final gate 2026-11-19) | Declare `group` in `fleet-1` | `group` is the template unit; `fleet-2` answers the `AssignmentPart` |
| The interface split is specified method by method here (owner, final gate 2026-11-19: strengthen the split) | Leave each other part to be designed by its own spec | Every later back end implements a stated contract, and the not-available state of every part is built and proven now |
| One part per owning spec, so `ConfigSource` splits into device config, config history, divergence, rollout, template, access and settings parts, and `DeviceSystem` is joined from `DeviceViewPart` and `InventoryHealthPart` | Keep `ConfigSource` whole | `ConfigSource` today names six owners; "each back end registers the part it owns" needs one owner per part |
| Handshake cap 32 | Keep 128 held for life | The acceptor's figure for the same flood; sessions are bounded by configuration |
| Read maximum derived from `HeldConfigMax` | A literal | One declaration of the largest message |
| Always-built managed server on a store that can serve | Build at reload when the first client appears | One lifecycle, one `Stop`, and the data and divergence targets need no reinstall |

## Known Limitations
- The serving certificate name is read at boot only: a change of `plugin hub server <block> certificate` takes effect at the next restart. Owner decision at the final gate, 2026-11-19 (Key Design Decisions). The reload apply leaves the served certificate as it is, and `docs/guide/fleet-config.md` says a certificate change needs a restart.
- No command answers the label keys and values in use (`show fleet labels`, listed by `fleet-ui-2` as missing); owned by `fleet-ui-2`, which designs its admin page.

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-close's /ze-review gate. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied

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
- [ ] AC-1..AC-33 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests N/A (reason above)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`
- [ ] **Commit A:** code + tests + docs + edited spec
- [ ] **Commit B:** `remove plan/release/features/spec-fleet-1-device-registry.md` only
