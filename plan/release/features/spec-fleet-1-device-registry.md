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
`plan/post-release/features/spec-fleet-0-umbrella.md` (its three
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

### Owner ruling of 2026-11-23: commands in the first release, pages after it

Thomas ruled (voice-transcribed answer "1 b") that the fleet features of the
first release are reached through commands on every command surface (CLI,
SSH, MCP, REST, gRPC and the web command line), and that their web pages and
views come after the first release (`ai/META-DECISIONS.md`, "Goal and
scope"). This changes the owner decisions above in one place: the "Data seam"
row's last sentence ("The dashboard and the device list go live on real data
when `fleet-1` lands") and the web half of that row no longer belong to this
spec. The split of the fleet interface's `Source` contract into one part per
back end, the estate part over the dispatcher, the join by device name, the
not-available state and the live-estate browser test move, unchanged, to
`plan/post-release/features/spec-fleet-ui-2-estate-tree-and-device-tables.md`,
section "Moved from fleet-1 (owner ruling 2026-11-23)". Every other owner
decision above stands as written.

This edit writes in a ruling Thomas made himself, so it is not put back to him
for approval (`ai/META-DECISIONS.md`, "How agents work"). It changes `## Task`
and `## Acceptance Criteria`, so the design approval of 2026-11-19 is stale:
the independent design judge (`ai/skills/ze-design-judge.md`) re-judges the
changed text before the spec is claimed. This spec is not re-approved by this
edit.

| AC moved out | Destination spec | Its AC there |
|--------------|------------------|--------------|
| AC-14 (a fleet page with only the estate part) | `plan/post-release/features/spec-fleet-ui-2-estate-tree-and-device-tables.md` | AC-8 |
| AC-15 (display state Unknown with no health part) | same | AC-9 |
| AC-28 (`fleetui.DevicesMax` equals `ManagedClientsMax`) | same | AC-10 |
| AC-29 (`test/web/fleet-estate-live.wb`) | same | AC-11 |
| AC-30 (every absent part renders not available) | same | AC-12 |
| AC-31 (the join by device name) | same | AC-13 |
| AC-32 (a second or nil part registration panics) | same | AC-14 |
| AC-33 (the sample build unchanged) | same | AC-15 |

What the moved criteria proved on the command surfaces is kept here: the
devices, their connection state, configuration state, labels and versions by
AC-1 to AC-13 and AC-25, and every command surface by AC-34, added with this
ruling. A device's health on the command surfaces is `fleet-4`'s
(`plan/release/features/spec-fleet-4-inventory-health.md`: AC-8, a device
that has not reported shows "pending" or "not reported", and AC-13, its
inventory and health commands on every command surface).

The goal: on a hub with up to 4,096 declared clients and 2,000 of them
connected, an operator runs `show fleet devices` on any command surface and
sees every declared device with its connection state, its last-seen time,
its labels and where its configuration stands, across a hub restart and a
config reload, with bounded memory and bounded store writes.

### Design revision of 2026-11-23: the managed listener's certificate under reload

The independent design judge refused the design on 2026-11-23 (digest
`c140976692c9`). The reload of the client set (C-4) binds serving addresses
while the hub runs, and the design had left the certificate those addresses
present as it was built at start. Read at the producer, that leaves three
defects, none of which C-7 decided:

| What the operator does | What the refused design did | Producer |
|------------------------|-----------------------------|----------|
| Adds by reload a serving block on a new address, with no `certificate` | The listener binds and presents a leaf issued for the addresses known at start, so a client that dials the new address refuses the hub: the leaf does not name that host | `managedCertificate` issues once for `managedLeafHosts` of the construction-time listeners; `ServingLeaf` keeps those hosts; the client checks the host of its dial address (`serverNameFromAddr`, `clientTLSConfig`) |
| Adds by reload the hub's first serving block, with `certificate fleet-hub` | The listener binds and presents the issued leaf, not `fleet-hub` | `startManagedServer` reads the name once, from the serving blocks of the boot config |
| Changes by reload the certificate a serving hub presents | The reload is accepted, nothing changes, and nothing tells the operator | No reload step reads the name; no log line, no doctor check |

The first two are defects of this spec's own reload, fixed here (ACs 35 to 37
and 40): a logic defect is fixed, never put to Thomas
(`ai/META-DECISIONS.md`, "Goal and scope"). The third is where C-7 applies:
Thomas decided that a changed serving certificate takes effect at the next
restart, and this revision adds what the operator is told (AC-38), and the
refusal of a name that does not resolve (AC-39). "Choices for Thomas" C-10
puts the remaining question to him, with the evidence this revision found.

The owner rule that an operator adds a peer, with any of its settings, to the
running configuration without a restart (`ai/META-DECISIONS.md`, "CLI, config,
and daemon surfaces") names routing peers, so a managed client entry is
outside its letter. Its objective holds here all the same, because a restart
of a hub that also routes drops every routing session: no reload is refused
because of what a new client entry or its serving block configures.

This revision changes `## Acceptance Criteria` and the design, and carries
choices Thomas did not make, so the design goes back to the judge and then to
him. This spec is not re-approved by this edit.

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
| A web page `/fleet` with SSE updates, `page_fleet.go` | None in `fleet-1`: the seam split, its estate part, the pages, events and SSE belong to `fleet-ui` (the seam and the estate part to `plan/post-release/features/spec-fleet-ui-2-estate-tree-and-device-tables.md`) | Umbrella Direction Update of 2026-11-18, row "Web views"; owner ruling of 2026-11-23 (above) |
| EventBus events `fleet.device.online/offline` | None | No consumer: the interface's live events are the `EventSource` of `plan/post-release/features/spec-fleet-ui-9-tasks-notifications-and-alarm-pages.md` |
| A `DeviceRegistry` wired from `cmd/ze/hub/main.go` into `ManagedConfigService` | The registry is part of `ManagedServer`, and replaces the `connected` set of `ManagedConfigService` | Owner placement A; no layering (`ai/rules/no-layering.md`) |
| Health status in the registry record | Answered by `fleet-4`'s part, joined by name | Owner: each back end answers only its own fields |
| "Client protocol unchanged" | One client change: the config-ack error text is bounded | The lowered read bound must hold every legal message (R-6) |
| Goal gate `./le verify worktree` passes | Removed | No agent runs any `./le verify` command (`ai/META-DECISIONS.md`) |
| The 128-connection cap was not mentioned | Lifted here | Umbrella Direction Update of 2026-11-18, row "Scale" |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/hub-engine.md`, `ai/doors/api-surfaces.md`, `ai/doors/commands-cli.md`, `ai/doors/config-yang.md` - the constraints of every subsystem this spec touches
  → Constraint: runtime state is persisted through the daemon's store under a key registered in `pkg/zefs/keys.go` (`MustRegister`), never `os.WriteFile`.
  → Constraint: something built only at startup ignores every reload (hub-engine door, Traps). The managed server is built only at startup today, which is the reload defect fixed here.
  → Constraint: the same trap covers the certificate the managed listener presents, built once by `managedCertificate`: once a reload binds addresses, the leaf's hosts and the certificate name must follow it or the operator must be told (Design revision of 2026-11-23).
  → Constraint: the door's "One reload path" boundary says nothing but the external plugins needs a restart; C-7 keeps one more thing at the restart, so the door is corrected in the same work.
  → Constraint: one reload path, `doReload` (`cmd/ze/hub/main_reload.go`); the managed client set joins it rather than building a second path.
  → Constraint: a handler answers data, never text; output goes through the pipes; a command-specific filter is declared with `command.RegisterPipeFilters` (`internal/component/command/pipe_filter.go`).
  → Constraint: a command is declared once and reaches every interface (`ai/doors/system.md`, "One schema, one command registry, every interface"): the two commands are builtin RPCs, so CLI, SSH, MCP, REST, gRPC and the web command line all serve them with no edit to an interface package, which `TestEveryCommandOnEverySurface` (`cmd/ze/hub/surface_parity_test.go`) checks.
  → Moved: the two constraints on the fleet interface (the `ze_web && ze_fleet_ui` gate of the estate part, and the estate part reading through `web.RouteDeps.Dispatch`) moved with that work to `plan/post-release/features/spec-fleet-ui-2-estate-tree-and-device-tables.md` (owner ruling of 2026-11-23).
- [ ] `docs/architecture/fleet-config.md` - the managed protocol, the dedicated listener, "What doctor reads from the managed listener", per-client secrets
  → Decision: the registry is a third record of the managed server beside the doctor record (`managedDoctorState`) and the held records (`heldRecords`), published the same way (`ManagedDoctorSnapshot`).
  → Constraint: a client is admitted only on the address of the block that declares it; a client reachable through two blocks is declared under both. The registry keys by name, so one record serves both declarations, and two declarations of one name must carry the same labels.
  → Constraint ("Hub certificate and client trust"): the certificate name wins whenever it is set, a name that does not resolve is an error and never a fallback, and a client anchored on the hub's root needs no change when the issued leaf changes. A client verifies the host of the address it dials, so the issued leaf must name the host of every address the hub serves.
- [ ] `docs/architecture/pki/tls-listeners.md` - how a hub listener serves a named certificate, and what a reload does to it
  → Constraint: a reload refuses a certificate name that does not resolve in the just-installed store, before any consumer applies, and restores the prior store (`restorePKIAfter`).
  → Constraint: a new certificate is installed without rebinding: the listener keeps its socket and the next handshake reads the new chain; material that cannot be used leaves the previous certificate serving.
- `docs/architecture/fleet-interface.md` - moved with the fleet interface work to `plan/post-release/features/spec-fleet-ui-2-estate-tree-and-device-tables.md`, which carries its two checkpoints (owner ruling of 2026-11-23)
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
- Reload today never touches the managed server; `hubConfig` is extracted once at boot (`cmd/ze/hub/main.go`, `zeconfig.ExtractHubConfig`, then `startManagedServer`).
- Diverged is presence of `meta/config/client-<name>.conf/held` (`HeldRecordKey`), listed by `HeldClients`.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/server/managed.go` - `ManagedConfigService` holds `readConfig`, `records` (the held records) and `connected`, a set of names. `RegisterClient` refuses a name already in the set with `ErrDuplicateClient`; `UnregisterClient` removes it. `HandleConfigFetch` hashes the client's config with `fleet.VersionHash`, answers `current` when the client's version matches (and clears its held record), otherwise the config, `HeldVersion` and `DiscardVersion`.
- [ ] `internal/component/plugin/server/managed_serve.go` - `managedMaxConns` is 128. `acceptLoop` takes a slot of `sem` before authentication and releases it only when `handleConn` returns, so a slot is held for the connection's life. `handleConn` authenticates with `AuthenticateWithLookup` against the block's clients, calls `RegisterClient`, builds `rpc.NewMuxConn(rpc.NewConn(conn, conn))`, records the mux in `conns`, and serves. `handleRequest` answers config-fetch, config-ack (`handleConfigAck`), config-held and ping. `NewManagedServer` fixes `listens` and `served` at construction. `Start` refuses an empty listener set. `Serves` reads the fixed `served` set.
- [ ] `internal/component/plugin/server/managed_doctor.go` - `managedDoctorState` keeps unbound addresses, per-client rejections (bounded by declared names) and the 16 most recent unknown names. `publishManagedDoctor` and `withdrawManagedDoctor` set and clear the atomic pointer `managedDoctorSource`; `ManagedDoctorSnapshot` answers `ErrManagedNotServing` when none is published. `ManagedRejectCause` lists the refusal causes.
- [ ] `internal/component/plugin/server/managed_held.go` (untracked, `fleet-7`'s work in progress) - `HeldRecordKey(name)` is `zefs.KeyConfigHeld` of `ClientConfigKey(name)`, so `meta/config/client-<name>.conf/held`. `HeldClients(store)` lists the names that have a record. `ReadHeldRecord` treats a record that does not decode as an error, never as absent.
- [ ] `cmd/ze/hub/managed_server.go` - `startManagedServer` returns nil when no server block has client entries, builds one `ManagedListener` per serving block, loads the daemon root (`zepki.LoadOrGenerateRoot`), starts the server and installs the store's single write observer, which calls `NotifyConfigChanged` for a written client config key and reads no storage.
- [ ] `internal/component/plugin/server/managed_serve.go` `managedCertificate`, `managedLeafHosts`, `withAuthority` - the certificate is decided once, in `NewManagedServer`. With `ManagedServerConfig.Certificate` set, the name is resolved once through `TLSMaterialResolver` (`pki.ServerTLSMaterial`) and the same pair answers every handshake; a name that does not resolve fails the construction, and `startManagedServer` logs it and serves no managed client. With no name, `plugin.NewServingLeaf` issues a leaf from the daemon root for `managedLeafHosts(addrs)`: `127.0.0.1` plus the host of each construction-time listen address, an unspecified address (`0.0.0.0`, `::`) adding none. `Start` passes the one `getCertificate` to every listener (`pluginipc.StartListeners`).
- [ ] `internal/component/plugin/leaf.go` - `ServingLeaf` holds `hosts` from construction and reissues the leaf for those same hosts at two thirds of its life (`Certificate`, `issueLocked`). Nothing changes the hosts after `NewServingLeaf`.
- [ ] `internal/component/managed/tls.go` `clientTLSConfig` and `client.go` `serverNameFromAddr` - every trust mode sets `ServerName` to the host of the address the client dials, and every mode but `TLSInsecure` verifies the presented leaf against it. A leaf that does not name that host ends the handshake before the token is written.
- [ ] `cmd/ze/hub/managed_server.go` (the certificate) - `startManagedServer` takes the certificate name from the serving blocks only (a block with no client entry contributes none), and loads the daemon root with `zepki.LoadOrGenerateRoot`.
- [ ] `cmd/ze/hub/main_reload.go` `runReloadContext` (the certificate gates) - after `zepki.Load` installs the new store, the reload resolves `environment.web.certificate` (`reloadWebCertificate`) and the looking glass name (`lgCertificateName`) with `zepki.ServerTLSMaterial` and refuses the reload through `restorePKIAfter` when one does not resolve; after promotion `lm.updateWebCertificate` and `lm.updateLGCertificate` install the new chains. No step reads the managed certificate name.
- [ ] `internal/component/plugin/server/reload_declarations.go` `RefuseChangedDeclarations` - the one reload refusal whose fix is `restart ze`: logged, and raised as `doctor-config-reload-refused` (`raiseReloadRefused`). The candidate config is not promoted.
- [ ] `internal/component/config/storage/store.go` `RuntimeKey` and `docs/guide/command-reference.md`, "delete data key" - a key under `meta/` outside `meta/config/` is runtime state, and the running daemon's `delete data key` and `update data key` accept it unless a running plugin is granted it. `meta/fleet/device/<name>/record` is such a key and no plugin is granted it, so a running hub accepts its removal and its overwrite. The held record differs: it sits under `meta/config/` and is refused.
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
- [ ] `cmd/ze/hub/surface_parity_test.go` - `TestEveryCommandOnEverySurface` builds the daemon dispatcher, walks every registered command and checks that the CLI, SSH, MCP, REST, gRPC, web admin and web completion surfaces each list and serve it.
- Moved: the reads of `internal/component/web/fleetui` (`source.go`, `source_devices.go`, `state.go`, `pages.go`, `register.go`, `sample/register/register.go`) and of the hub-side gates `cmd/ze/hub/register_fleet_ui.go` and `register_fleet_ui_sample.go` moved with the fleet interface work to `plan/post-release/features/spec-fleet-ui-2-estate-tree-and-device-tables.md` (owner ruling of 2026-11-23).
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
- The certificate the managed listener presents follows the reload where C-7 leaves it free. The issued leaf names the host of every address the hub serves after each reload. A hub with no managed listener bound takes the certificate its new config names when a reload binds the first one. A reload that names a certificate the store does not hold is refused, as it is for the web listener.
- A reload that changes the certificate a serving hub presents is accepted and the listener keeps the certificate it serves until the next restart (C-7), and now says so: a log line and a `ze doctor` warning name the certificate served, the one configured and `restart ze`.
- A device record removed or overwritten through the running daemon's data commands is noticed by the registry (R-4).
- Not changed here: the fleet interface keeps its single source slot and its sample build. Its split into parts, the estate part over the registry and the not-available state belong to `plan/post-release/features/spec-fleet-ui-2-estate-tree-and-device-tables.md` (owner ruling of 2026-11-23).

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Connected-name set | `internal/component/plugin/server/managed.go` `ManagedConfigService.connected`, `RegisterClient`, `UnregisterClient` | Refuses a second session under one name; forgotten at disconnect | Replaces: the registry's live record holds connected state, and the duplicate refusal reads it. Two holders of "connected" would disagree |
| Managed server records | `managed_doctor.go` `managedDoctorState`; `managed_held.go` `heldRecords` | Per-client facts the server keeps, bounded by declared names | Extends: the registry is a third record of the same server, bounded the same way |
| Published snapshot | `managed_doctor.go` `managedDoctorSource`, `ManagedDoctorSnapshot`, `publishManagedDoctor`, `withdrawManagedDoctor` | Lets doctor read the running server's record with no import of the hub | Extends: the registry publishes through the same pointer and an exported `ManagedRegistrySnapshot`; the doctor record gains the registry's write and read failures and the handshake saturation |
| Push coalescing | `managed_serve.go` `pending`, `pushing`, `notifyWake`, `takePending`, `notifyWorker` | A burst of changes for one client becomes one push; bounded by connected names | Extends the pattern: one long-lived record writer with a dirty set, so a burst of events for one device becomes one store write |
| Borrowed store | `ManagedServerConfig.Store`; `managed_held.go` `HeldRecordKey`, `ReadHeldRecord` | The held records are written and read through the hub's store | Extends: the device records use the same borrowed store, no second handle |
| Runtime-state handle | `internal/core/statestore/statestore.go` `SetStore`, `Write`, `Read`, `Delete`, `List` | A process-wide accessor to the daemon's one `storage.Storage`, for callers that are handed no store | Does not apply as the route, and the store is the same one. The managed server is handed that `storage.Storage` by the hub (`ManagedServerConfig.Store`) and already reads client configs and writes held records through it. Three reasons keep the device records on that handle: the held records sit under `meta/config/`, a configuration key family, so the server needs the store handle whatever the device records use, and two routes to one store would have two lifetimes; `Stop` joins every user of the handle before the hub returns the store, an order the process-wide accessor's `SetStore(nil)` is outside; and a test gives each server its own store (the counting store of AC-20), which a process-wide accessor cannot. The key is registered in `pkg/zefs/keys.go` and no file is written directly, which is what the door's rule protects |
| Managed listener certificate | `managed_serve.go` `managedCertificate`, `withAuthority`; `ManagedServer.getCertificate` | Decides once, at construction, what every handshake presents: the named pki certificate, resolved once, or a leaf the daemon root issues | Extends: the server keeps the certificate source behind the one `getCertificate` every listener already calls per handshake, so the reload apply can replace what it answers while the listeners keep their sockets. The fail-closed rule for a name is kept: a name is never answered by the issued leaf |
| Self-renewing leaf | `internal/component/plugin/leaf.go` `ServingLeaf` (`NewServingLeaf`, `Certificate`, `issueLocked`) | Issues the listener's leaf for a fixed host list and reissues it for the same list as it ages | Extends: the holder gains one operation that replaces its host list and issues for the new one at the call, keeping the previous leaf and list when the authority cannot issue. The plugin acceptor's leaf (`NewHubAcceptor`) never calls it and is unchanged |
| Leaf host derivation | `managed_serve.go` `managedLeafHosts` | Turns listen addresses into the leaf's hosts: `127.0.0.1` plus each named host, none for an unspecified address | Extends by a second call site: the reload apply calls it with every serving address of the new config, before it binds an added one. The function is unchanged |
| Client hostname check | `internal/component/managed/tls.go` `clientTLSConfig`; `client.go` `serverNameFromAddr` | The client verifies the leaf against the host of the address it dials | Does not apply as a change: the client is right, and the hub is what must present a leaf naming that host |
| Reload certificate gate | `cmd/ze/hub/main_reload.go` `runReloadContext`: `zepki.ServerTLSMaterial` on the web and looking glass names after `zepki.Load`, `restorePKIAfter` (`docs/architecture/pki/tls-listeners.md`, "Decision: a named certificate fails closed") | Refuses a reload whose web or looking glass certificate name does not resolve in the new store, before any consumer applies | Extends: the managed certificate name of the new hub config is a third name the same gate resolves and refuses the same way |
| Certificate rotation without rebinding | `cmd/ze/hub/listener_migrate.go` `updateWebCertificate`, `updateLGCertificate`; `UpdateTLSCertificate` (`docs/architecture/pki/tls-listeners.md`, "Decision: rotation does not rebind the hub listeners") | After a reload, installs the re-resolved chain on the running web and looking glass servers; the next handshake serves it | Extends in part: the managed listener replaces what its handshake answers the same way (no rebind, sessions untouched) for the issued leaf's hosts and for a hub with no managed listener bound. For a changed certificate on a serving hub it does not apply by C-7: the managed listener keeps what it serves, which is the departure the Design documents table records. C-10 asks Thomas whether it should apply there too |
| Restart-required refusal | `internal/component/plugin/server/reload_declarations.go` `RefuseChangedDeclarations`, `raiseReloadRefused`, `doctor-config-reload-refused` | Refuses whole a reload that changes the external plugins, with the fix `restart ze` | Does not apply: C-7 says a changed certificate takes effect at the next restart, so the reload is accepted and the new config is the one a restart boots. A refusal keeps the old config, and a refused commit is never promoted, so a restart after it would boot the old certificate. C-10 lists the refusal as an option |
| Managed doctor record | `managed_doctor.go` `managedDoctorState`, `bindFailed`; `check_managed_runtime.go` | Unbound addresses and client rejections, read by the runtime checks | Extends: the record gains the certificate a restart is pending for, and an address whose leaf could not be issued is recorded unbound with that reason |
| Daemon data commands | `internal/component/config/storage/cli` `delete data key`, `update data key`; `storage.RuntimeKey` | Remove or write one runtime-state key while ze runs; refuse a configuration key and a key a running plugin is granted | Extends by use: `delete data key meta/fleet/device/<name>/record` is how an operator makes the hub forget a device, and the registry reconciles its memory with it (R-4). No grant is added: the managed server is no plugin |
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
| Reload | `cmd/ze/hub/main_reload.go` `doReloadContext`, `publishNameServers` | The one reload path; publishes post-promotion state | Extends: the managed client set is applied after promotion, beside `publishNameServers`; its extraction is checked before any step applies. The same apply carries the certificate name of the new config and re-derives the issued leaf's hosts |
| Held record | `managed_held.go` `HeldClients`, `HeldRecordKey` (`fleet-7`) | The hub's copy of what a holding client runs | Does not apply as a writer: the registry reads presence only, as the owner decided |
| Fleet interface seam | `internal/component/web/fleetui/source.go` `sourceSlot`, `MustRegisterSource`; `register.go` `sourceRegistered` | One source per build, all concerns | Does not apply here: the split into parts is web work, moved to `plan/post-release/features/spec-fleet-ui-2-estate-tree-and-device-tables.md` by the owner ruling of 2026-11-23 |
| Device display state | `internal/component/web/fleetui/state.go` `DeviceDisplayState` | Derives Unknown while `HealthReported` is false | Does not apply here: moved with the seam to `fleet-ui-2` |
| Route deps | `internal/component/web/webroute.go` `RouteDeps.Dispatch` | Gives a route builder the web dispatcher | Does not apply here: the estate part built over it moved to `fleet-ui-2` |
| Command surface parity | `cmd/ze/hub/surface_parity_test.go` `TestEveryCommandOnEverySurface`; `command.AnswerSurface` | Checks every registered command is listed and served on every operator surface | Extends: the two new builtins are covered by it with no edit, and AC-34 names it |
| Perf harness | `cmd/ze/hub/fleet_perf_test.go` `TestFleetManyClientsPerf` | 128 clients, one fetch each | Extends: 2,000 clients held connected at once |
| Typed events | `internal/core/events/typed.go` `Register` | Typed bus events | Does not apply: placement B (a component fed by events) was rejected by the owner |

## Design documents

Derived by `./le spec design documents spec plan/release/features/spec-fleet-1-device-registry.md` (41 pages and 5 decision rows on 2026-11-23), with rows added beyond the derivation: two doors this spec's Required Reading names (`ai/doors/api-surfaces.md`, `ai/doors/commands-cli.md`) and two owner rules of `ai/META-DECISIONS.md`. The design revision of 2026-11-23 added the rows on the managed listener's certificate: two of `ai/doors/hub-engine.md`, two of `docs/architecture/fleet-config.md` ("Hub certificate and client trust"), two of `docs/architecture/pki/tls-listeners.md`, and one each of `docs/architecture/pki/pki-store.md` and `docs/architecture/api/process-protocol.md` for `internal/component/plugin/leaf.go`, which joined Files to Modify.

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | One schema, one command registry, every interface | System door (always derived) | "A command is declared once and reaches every interface." | agrees | `show fleet devices` and `show fleet device` are builtin RPCs with `ze:command` nodes in `ze-managed-cmd.yang`, registered once from `internal/component/managed/register.go`; no interface package is edited, and `TestEveryCommandOnEverySurface` checks every surface lists and routes them (AC-34) |
| `ai/doors/system.md` | Shared machinery you use rather than rebuild | System door (always derived) | "In the daemon, `statestore` under a key in `pkg/zefs/keys.go`, never `os.WriteFile`." | agrees | The device record is written through the daemon's one store under `KeyFleetDeviceRecord`, registered with `MustRegister` in `pkg/zefs/keys.go`; no file is written directly. It reaches that store through the `storage.Storage` handle the hub gives the managed server (`ManagedServerConfig.Store`), not through the `statestore` accessor, which is a process-wide route to the same handle. The held records already go that way and must, because they sit under `meta/config/`; `Stop` joins every user of the handle before the hub returns the store; and a test gives each server its own store (Existing Mechanisms, "Runtime-state handle") |
| `ai/doors/system.md` | Shared machinery you use rather than rebuild | System door (always derived) | "A long-lived worker on a channel, never one per event" | agrees | One record writer started by `Start` and ended by `Stop` drains a dirty set; no goroutine is started per connect, fetch or ack (Critical Review Checklist, goroutine lifecycle) |
| `ai/doors/hub-engine.md` | Boundaries | Door covering `cmd/ze/hub`, `internal/component/managed`, `pkg/fleet` | "SIGHUP and a managed push both reach `doReload` (`cmd/ze/hub/main_reload.go`). A new trigger joins it rather than building a second path." | agrees | The managed client set joins `doReloadContext`: the hub extraction is checked before any step applies and applied after promotion beside `publishNameServers`; no second reload path is built |
| `ai/doors/hub-engine.md` | Traps | Door covering `cmd/ze/hub` | "Something built only at startup ignores every reload." | agrees | This is the reload defect the spec fixes: the managed server, built only at startup today, gains an apply that the reload calls (AC-8, AC-9, AC-26). The apply also carries what the certificate needs so that a bound address is verifiable: the issued leaf's hosts are re-derived (AC-35, AC-36), and a hub with no managed listener bound takes the certificate the new config names (AC-37) |
| `ai/doors/hub-engine.md` | Traps | Door covering `cmd/ze/hub` | "so a changed RADIUS or TACACS+ server was accepted and applied to nothing" | departs | One thing stays built only at startup, by C-7: the certificate a serving hub presents. A reload that changes it (a name set, changed or removed, or the same name whose stored material changed) is accepted and applied at the next restart. It is no longer silent: the apply logs it and `ze doctor` reports `doctor-managed-certificate-restart-pending` with the certificate served, the one configured and the fix `restart ze` (AC-38). The door's trap gains that case ("Documents departed from"); C-10 asks Thomas whether to end the departure |
| `ai/doors/hub-engine.md` | Boundaries | Door covering `cmd/ze/hub` | "they change only when `ze` restarts, and nothing else in the config needs one" | departs | By C-7 a second part of the config needs a restart: a change of the certificate a serving hub's managed listener presents. The reload is not refused for it, unlike the external plugins; it is accepted and the change waits, reported (AC-38). The door's sentence is corrected to name it ("Documents departed from", Files to Modify) |
| `ai/doors/hub-engine.md` | Traps | Door covering `cmd/ze/hub` | "A check that runs only at start passes validate." | agrees | The label agreement and the 4,096-name refusals sit in `ExtractHubConfig` beside `checkManagedCertificateAgreement`; `validate_semantic.go` calls `ExtractHubConfig`, so `ze config validate`, boot and the reload pre-check give one verdict (AC-23, AC-24) |
| `ai/doors/plugin-infra.md` | Traps | Door covering `internal/component/plugin/server`, `pkg/plugin/rpc` | "so a payload must be compact `json.Marshal` output." | agrees | The managed channel keeps newline framing; only the hub's reader of it gets a lower maximum, and the bounded ack error travels inside the same compact line |
| `ai/doors/plugin-infra.md` | Boundaries | Door covering `pkg/plugin/rpc` | "The SDK stays generic." | agrees | `pkg/plugin/rpc` gains one additive constructor taking a caller-given maximum; it names no managed or fleet concept, and every existing caller keeps `MaxMessageSize` |
| `ai/doors/core.md` | Boundaries | Door covering `pkg/zefs/keys.go`, `internal/core/diagnostic` | "Runtime state goes through the daemon's one store." | agrees | The device records are runtime state in the daemon's store under a registered key family; the four new doctor codes register in `codes_plugin_infra.go`, the existing code registry |
| `ai/doors/config-yang.md` | Traps | Door covering `internal/component/config/loader_extract.go` | "`ze config validate` never calls a live `OnConfigVerify`." | agrees | No rule of this spec lives in an `OnConfigVerify`: the label and count refusals run in `ExtractHubConfig`, which validate reaches through `validate_semantic.go`, and the label constraints are YANG (`pattern`, `length`, `max-elements`) |
| `ai/doors/le-tooling.md` | Where it sits | Door covering the `test/managed/fleet-*.ci` files | "Functional `.ci` files live under `test/<suite>/` and run through" | agrees | The ten new functional tests live under `test/managed/` and run through `./le test managed` |
| `ai/doors/commands-cli.md` | Boundaries | Added: Required Reading names this door | "A handler answers data, never text." | agrees | Both handlers answer rows and a document with kebab-case keys, declared by `RegisterShape` and `RegisterColumns`, and every format comes from the pipes |
| `ai/doors/api-surfaces.md` | Boundaries | Added: Required Reading names this door | "Never add a command in an adapter." | agrees | No REST, gRPC, MCP, SSH or web package gains a fleet route or command; each surface reaches the builtins through the dispatcher |
| `ai/META-DECISIONS.md` | Goal and scope | Added: owner rule of 2026-11-23 ("1 b", voice-transcribed) | "Fleet management ships in the first release with its device registry," | agrees | The registry ships reachable through its two commands on every command surface; its web pages moved to `fleet-ui-2` (Task, "Owner ruling of 2026-11-23") |
| `ai/META-DECISIONS.md` | CLI, config, and daemon surfaces | Added: owner rule on command surfaces | "Every operator command runs through the daemon on every command surface (MCP," | agrees | Both commands are daemon builtins served on MCP, REST, gRPC, SSH and the web with one answer; they hold no local route, so no local answer can differ (owner, final gate 2026-11-19, "Daemon-only commands") |
| `docs/architecture/fleet-config.md` | Dedicated managed listener (reuses auth + transport) | `// Design:` page of `managed.go`, `managed_serve.go`, `managed_doctor.go`, `client.go`, `envelope.go`, `managed_server.go`; modified page | "to each `server` block that declares `client` entries." | agrees | A listener still binds only for a block that declares clients; the reload apply binds a block's address when it gains its first client and closes one that loses its last (AC-26, R-8). The server object exists with no listener so the reload can bind, which the page's new "Device registry" text states |
| `docs/architecture/fleet-config.md` | Hub certificate and client trust | `// Design:` page of `managed_serve.go` (`managedCertificate`, `managedLeafHosts`); modified page | "A client anchored on the root needs no change for either." | agrees | The reload apply calls `managedLeafHosts` with every serving address of the new config and has `ServingLeaf` issue for that list before an added address is bound, so a client anchored on the root verifies the hub on an address added by reload with no client change (AC-35, AC-36). When the authority cannot issue, the address is not bound and is reported unbound, and the previous leaf keeps serving the others (AC-40). The page gains that the leaf's hosts follow a reload |
| `docs/architecture/fleet-config.md` | Hub certificate and client trust | `// Design:` page of `managed_serve.go`; modified page | "The name wins whenever it is set, and a name that resolves to nothing is an" | departs | Two cases agree: a hub with no managed listener bound serves the name its new config carries when a reload binds the first one (AC-37), and a reload naming a certificate the new store does not hold is refused whole, never answered by a fallback (AC-39). One departs, by C-7: on a hub whose managed listener is serving, a name set, changed or removed by reload wins only at the next restart, and until then the listener presents what it presented, with the log line and the doctor warning of AC-38. The page gains that rule ("Documents departed from") |
| `docs/architecture/fleet-config.md` | Per-Client Secrets | `// Design:` page of `managed_serve.go`; modified page | "A client that must be reachable through two blocks is declared under both." | agrees | The registry keys by name, so one record serves both declarations, and two declarations of one name must carry the same labels (R-10, AC-23); admission per block is unchanged |
| `docs/architecture/fleet-config.md` | What doctor reads from the managed listener | `// Design:` page of `managed_doctor.go`, `check_managed_runtime.go`; modified page | "no managed server in the process the answer is `ErrManagedNotServing`" | agrees | The registry snapshot is published through the same pointer and withdrawn by `Stop`; with no server the commands answer an operator error naming its fix (`TestShowFleetDevicesNotServing`), and the doctor table gains the four new codes |
| `docs/architecture/fleet-config.md` | Config Storage (Hub Side) | `// Design:` page of `managed_server.go`; modified page | "The hub's `Storage.SetWriteObserver` callback maps a written client-config key" | agrees | The same single observer also marks the written client's hub version stale and still reads no storage; a record key ends in `record`, which `ClientNameFromConfigKey` never maps, so a record write pushes nothing (R-9) |
| `docs/architecture/fleet-config.md` | Connection Lifecycle | `// Design:` page of `client.go`, `managed_serve.go`; modified page | "largest answer, 16 MiB (`rpc.MaxMessageSize`), on a link of about 4.5 Mbit/s." | agrees | The client's reader keeps 16 MiB; only the hub's reader of client lines is lowered, derived from `fleet.HeldConfigMax`, because the hub may serve a configuration larger than any client sends (Changed-Wire Census) |
| `docs/architecture/fleet-config.md` | Connection Lifecycle | `// Design:` page of `managed_serve.go`; modified page | "A panic while the hub serves one client ends that client's connection only." | agrees | The registry marks the device offline in `handleConn`'s deferred cleanup, which runs on a contained panic too, so the name is released and the reconnect admitted |
| `docs/architecture/fleet-config.md` | RPCs | `// Design:` page of `envelope.go`; modified page | "after the first four verbs is optional and omitted when unset" | agrees | No verb and no field is added; `config-ack`'s `error` keeps its type and is cut by the client at `fleet.AckErrorMax` with a marker, a value bound the page's "Protocol" text gains |
| `docs/architecture/fleet-config.md` | Non-Goals | `// Design:` page of the managed server files; modified page | "Hub already exists; extend it" | agrees | The registry is a record inside `ManagedServer` (owner, placement A), not a new server component |
| `docs/architecture/api/commands.md` | Which surfaces offer a command | `// Design:` page of `register.go`; anchored to the managed and hub files; modified page | "Offered: the daemon serves each through its handler" | agrees | Both commands are builtin RPCs, the registration row this statement covers, so every surface offers them with no declaration of their own |
| `docs/architecture/api/commands.md` | Per-command declarations: what a command says about itself | `// Design:` page of `register.go`; modified page | "the pipe operations this command runs in its own handler" | agrees | `\| label` is declared with `RegisterPipeFilters` on `show fleet devices` and runs in its handler, which keeps the matching rows (Data Flow step 11, A-4); the command also registers its own shape and columns, so it inherits none from `show fleet` |
| `docs/architecture/api/commands.md` | Design decisions | Decision of 2026-08-21 on a page this spec modifies | "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included" | agrees | `show fleet devices` names the subject and stops; narrowing by label is the pipe operation `\| label <key>=<value>`, never an argument or a `--label` flag. `show fleet device name <name>` names one device as its subject, a document rather than the table, in the grammar of `show fleet divergence name <name>` |
| `docs/architecture/api/commands.md` | Design decisions | Decision of 2026-11-20 on a page this spec modifies | "A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed."" | agrees | `label` is no operator of the catalog (`pipe_catalog.go`), so it is the command's own operation and overrides nothing; every generic, `match`, `count`, `first` and the formats included, still applies to the rows unchanged |
| `docs/architecture/api/ipc_protocol.md` | Message Framing | `// Design:` page of `framing.go`, `conn.go`; modified page | "All plugin RPC messages are UTF-8, newline-delimited lines:" | agrees | The framing is unchanged; the page gains the constructor with a caller-given maximum and that only the hub's managed reader passes one |
| `docs/architecture/api/process-protocol.md` | Wire Format | `// Design:` page of `types.go`; anchored to `framing.go`, `conn.go` | "Maximum message size is 16 MB." | agrees | Every plugin reader keeps 16 MiB; the framing paragraph states that the plugin protocol keeps it (Documentation Update Checklist row 8) |
| `docs/architecture/api/wire-format.md` | Framing | Anchored to `framing.go`, `conn.go` | "16 MB (16,777,216 bytes)" | agrees | The IPC maximum stays the library default; the managed channel's hub-side bound is stated in `docs/architecture/fleet-config.md`, "Protocol" (row 7) |
| `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` | Both ESP forms on one inbound state | Anchored to `conn.go` (`writeFrame`) | "" | silent | The section cites `writeFrame` for a serialized framed write of IKE report RPCs; this spec changes no write path, only an added read constructor |
| `docs/architecture/api/architecture.md` | Implementation Status | Anchored to `managed_serve.go`, `types.go`, `main.go`, `main_reload.go` | "" | silent | Lists the plugin process, dispatch and BGP API features; silent on the managed channel and the fleet commands |
| `docs/architecture/core-design.md` | 19. Component Boundaries | Anchored to `managed_serve.go`, `types.go`, `main.go`, `main_reload.go` | "Each component under `internal/component/` is independently removable." | agrees | No import between components is added: `managed` already imports `plugin/server` for `show fleet divergence`, the registry lives in `plugin/server`, and no web package is touched |
| `docs/architecture/core-design.md` | Design decisions | Decision of 2026-11-23 | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | The hub's managed configuration programs neither the kernel nor VPP; the labels and the reload apply add no feature the dataplane guard judges |
| `docs/architecture/core-design.md` | Design decisions | Decision of 2026-11-23 | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | The spec adds no dataplane verdict; its own refusals follow the same rule, one verdict in `ExtractHubConfig` at validate, boot and the reload pre-check |
| `docs/architecture/core-design.md` | Design decisions | Decision of 2026-11-23 | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "nft"` annotation." | agrees | No firewall leaf, verifier or `ze:backend` annotation is touched |
| `docs/architecture/hub-architecture.md` | What the daemon does at start | `// Design:` page of `main.go`, `main_reload.go`; anchored to `managed_serve.go`, `managed_server.go`; modified page | "the external plugins and their modules alone: every other part of the config" | departs | The managed client set now reloads without a restart, and so do the hosts of the issued leaf and the certificate of a hub with no managed listener bound. One thing does not: on a hub whose managed listener is serving, a changed certificate (`plugin hub server <block> certificate`, or the material it names) takes effect at the next restart (owner, final gate 2026-11-19, C-7; Known Limitations). The reload is accepted, and a log line and `doctor-managed-certificate-restart-pending` say a restart is pending (AC-38). The page gains that exception ("Documents departed from") |
| `docs/architecture/hub-architecture.md` | Store ownership and configuration authority | `// Design:` page of `main.go`; modified page | "The daemon owns one writable tree at `<config-folder>/database/` until exit." | agrees | The registry writes through the store the daemon owns, borrowed by the managed server, and its writer is joined by the managed server's `Stop` before the hub returns that store |
| `docs/architecture/doctor-and-health-checks.md` | The runtime phase: checks only the daemon runs | Anchored to `managed_doctor.go`, `check_managed_runtime.go`; `// Design:` page of `codes_plugin_infra.go`; modified page | "such state registers in the `runtime` phase, through the same" | agrees | `doctor-managed-device-record-unwritable`, `doctor-managed-device-record-unreadable`, `doctor-managed-handshake-saturated` and `doctor-managed-certificate-restart-pending` read the running server's record and register in the runtime phase through `diagnostic.RegisterDoctorCheck`, as the existing managed checks do |
| `docs/architecture/doctor-and-health-checks.md` | Every finding names the command that fixes it | Modified page | "A doctor diagnostic carries `fix`: the command or config statement the" | agrees | Each new check carries `Fix` (Critical Review Checklist); the unreadable-record fix names `ze data rm meta/fleet/device/<name>/record` (R-4) |
| `docs/architecture/zefs-format.md` | Key Namespaces | `// Design:` page of `pkg/zefs/keys.go`; modified page | "Keys follow a `<namespace>/<qualifier>/<path>` convention to prevent collisions between metadata and config files." | agrees | `meta/fleet/device/{name}/record` is a new family outside `meta/config/` and `meta/managed/` (owner); `meta/fleet/` holds no value, so no key sits below a value; the page's table gains the row |
| `docs/architecture/container/image-store.md` | State keys | Anchored to `pkg/zefs/keys.go` | "" | silent | Covers the container plugin's `meta/container/` keys; silent on fleet keys |
| `docs/architecture/diagnostics/debug-filtering.md` | One ZeFS trap | Anchored to `pkg/zefs/keys.go` | "`List(prefix)` fails when the prefix ends in a slash." | agrees | The registry's load at `Start` lists the record family by the prefix `meta/fleet/device`, written with no trailing slash |
| `docs/architecture/irr-owner.md` | Stored keys and notices | Anchored to `pkg/zefs/keys.go` | "" | silent | Covers the IRR owner's `meta/irr-*` keys and events; silent on fleet keys |
| `docs/architecture/storage-backends.md` | Opening and ownership | Anchored to `pkg/zefs/keys.go`, `main.go` | "A second writer receives `ErrBusy`" | agrees | No second opener: the records use the daemon's borrowed store handle |
| `docs/architecture/api/json-format.md` | Field naming | Anchored to `types.go` | "JSON keys are lowercase kebab-case, never camelCase and never snake_case." | agrees | Every record field and answer key is kebab-case (`connected-since`, `last-fetch-result`, `hub-version`) |
| `docs/architecture/cli/command-verbs.md` | Narrowing and shaping: the pipe layer | Anchored to `types.go` | "An owned filter takes the name of the field it selects on." | agrees | `\| label` selects on the row's `labels` field and names what the operator chooses by, one label; it narrows an answer that exists and chooses nothing the command acts on |
| `docs/architecture/cli/error-surface.md` | The operator error | Anchored to `types.go` | "A refusal an operator reads is an `opererr.Error`" | agrees | The unknown name (AC-12), the hub not serving, and the label and count refusals at verify (AC-23, AC-24) are operator errors with their fix on the last line (Critical Review Checklist) |
| `docs/architecture/config/transaction-protocol.md` | 1. Design Principles | Anchored to `types.go`, `main_reload.go` | "The hub promotes a candidate after runtime reload succeeds." | agrees | The managed client set is applied after promotion, as post-acceptance state like `publishNameServers`; an address that does not bind is recorded and the rest of the reload stands (R-8) |
| `docs/architecture/encoding-context.md` | TL;DR (Read This First) | Anchored to `types.go` | "" | silent | Covers BGP capability-dependent encoding; silent on the managed server and fleet commands |
| `docs/architecture/plugin-manager-wiring.md` | Stage 1 Registration: Config Transaction Fields | Anchored to `types.go` | "" | silent | Covers plugin Stage 1 budget fields; the managed server is no plugin and declares no Stage 1 field |
| `docs/architecture/web-interface.md` | Admin command visibility | Anchored to `types.go`, `main.go`, `main_reload.go` | "Admin command discovery omits schema-hidden nodes and their descendants." | agrees | Neither command is hidden, so the web admin and the web command line list both (AC-34); no web page or route is added |
| `docs/architecture/config/syntax.md` | List (keyed entries) | `// Design:` page of `loader_extract.go`; anchored to `main.go` | "A list entry can also be written on one line, with the key first and the child" | agrees | `label` is a keyed list under the hub `client` entry with one `value` leaf. In a config file it is written as a block, `label site { value lon; }`: on one line the values after the key are assigned by position (`parseListInlineEntry`), so in `label site value lon;` the word `value` would itself be taken as a value. The `set` form keeps the keyword, `set plugin hub server central client edge-01 label site value lon`. The page's hub example gains a label (Documentation Update Checklist row 2) |
| `docs/architecture/appliance/remote-operations.md` | Decisions | Anchored to `main.go` | "" | silent | Covers appliance config preview and push over SSH; silent on the managed channel and the hub's registry |
| `docs/architecture/behavior/signals.md` | Ze Signal Mapping | Anchored to `main.go`, `main_reload.go` | "`handleSIGHUPReload` (`cmd/ze/hub/main_reload.go`), then `reactor.SignalHandler.OnReload`" | agrees | A SIGHUP reaches the same reload, whose post-promotion step now applies the managed client set; no signal is added |
| `docs/architecture/cli/command-completion.md` | The producers | Anchored to `main.go` | "Every producer asks one predicate what its surface owes a command:" | agrees | No producer is edited: each asks `command.AnswerSurface` for the two builtins. The device-name completion is the builtin's `Complete`, answered from the snapshot's names |
| `docs/architecture/config/yang-config-design.md` | API Schemas: `ze-bgp-api.yang` and `ze-*-cmd.yang` | Anchored to `main.go` | "The `-cmd.yang` modules define the CLI navigation hierarchy using" | agrees | The two commands are `config false` containers with `ze:command` in `ze-managed-cmd.yang`, beside `show fleet status` and `show fleet divergence` |
| `docs/architecture/diagnostics/crash-capture.md` | Recovered panics | Anchored to `main.go` | "below that boundary ends only the unit the input belongs to" | agrees | A panic while serving one client ends that connection only; the registry's offline mark runs in the cleanup that follows it |
| `docs/architecture/pki/pki-store.md` | Traps this code exists to avoid | Anchored to `main.go` | "A YANG list is built with the list-entry API, not with nested containers." | agrees | The `label` list's extraction tests build their entries as list entries |
| `docs/architecture/pki/pki-store.md` | Consequences worth knowing | Anchored to `leaf.go`, `main.go` | "A certificate the operator NAMED is never reissued." | agrees | The reload re-derives hosts only for the leaf ze issues. A named certificate is answered unchanged at every handshake, and whether it names a host added by reload is the operator's to arrange (Known Limitations) |
| `docs/architecture/api/process-protocol.md` | A refused connect-back | `// Design:` page of `leaf.go` | "fresh leaf once two thirds of the current one's life is spent." | agrees | `ServingLeaf` keeps its renewal rule, and a renewal after a host replacement issues for the new hosts. The plugin acceptor's leaf never has its hosts replaced: its listener is not rebound by a reload |
| `docs/architecture/pki/tls-listeners.md` | Decision: rotation does not rebind the hub listeners | Anchored to `main.go`, `main_reload.go` | "A hub listener re-resolves the name in the new store." | departs | The managed listener is a hub listener. On a serving hub the reload re-resolves its name in the new store only to compare it with what the listener serves: a difference is reported and waits for the restart (owner, final gate 2026-11-19, C-7; AC-38), where the web and looking glass listeners install the new chain. The page's table gains the managed listener's row ("Documents departed from"). Where C-7 does not reach, the design follows this decision: the issued leaf's new hosts and the first listener's certificate are installed with no rebind, and the next handshake reads them (AC-35 to AC-37) |
| `docs/architecture/pki/tls-listeners.md` | Decision: a named certificate fails closed | Anchored to `main.go`, `main_reload.go` | "The reload path refuses the same two names against the just-installed store." | agrees | The managed certificate name becomes a third name the same gate resolves after `zepki.Load` and refuses through `restorePKIAfter`, before any consumer applies (AC-39). The page's sentence and source anchor gain it |
| `docs/architecture/pki/tls-listeners.md` | Decision: rotation does not rebind the hub listeners | Anchored to `main.go`, `main_reload.go` | "Unparseable material leaves the previous certificate serving." | agrees | A leaf the authority cannot issue for the new hosts leaves the previous leaf and host list serving the bound addresses; only the added address is not bound, and it is reported (AC-40) |
| `docs/architecture/system-architecture.md` | Composition roots | Anchored to `main.go` | "" | silent | Covers the `ze`/`le` composition roots; silent on the managed server |
| `docs/architecture/testing/ci-format.md` | Registered engines launched externally | Anchored to `main.go` | "" | silent | Covers the `le mock plugin-external` launcher; the new `.ci` tests use existing hub and client daemons and no new directive |
| `docs/architecture/aaa-tacacs.md` | Config surface constraints | Anchored to `main_reload.go` | "" | silent | Covers TACACS+ config leaves and probes; silent on the hub client entry and its labels |
| `docs/architecture/plugin/component-boundaries.md` | What this buys | Anchored to `main_reload.go` | "" | silent | Covers the four `pkg/ze` interfaces; the design adds no engine interface, subsystem or plugin |
| `ai/rules/repo-maintenance.md` | Directives | `// Design:` page of `internal/component/managed/register.go` | "A change that adds or changes a surface future agents use, verify, document or avoid MUST update its discovery path in the same work" | agrees | The Repo-maintenance discovery table names `ai/INDEX.md`, the hub-engine door and `docs/architecture/fleet-config.md`'s new "Device registry" section, and the new runtime faults get doctor checks |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A managed client's TLS connection to a serving block's listener: `#0 auth`, then newline-framed requests (`config-fetch`, `config-ack`, `config-held`, `ping`) and answers to `config-changed`.
- A config load or reload carrying `plugin hub server <block> client <name> { secret; label <key> { value <v>; } }` (in `set` form, `plugin hub server <block> client <name> label <key> value <v>`).
- A write of `file/active/client-<name>.conf` (the store's write observer).
- An operator's `show fleet devices [| label <key>=<value>]` or `show fleet device name <name>` on any command surface: CLI, SSH, MCP, REST, gRPC, the web command line.

### Transformation Path
1. Accept: `acceptLoop` takes a handshake slot, `handleConn` authenticates, then releases the slot. A failed or timed-out handshake releases it too.
2. Admit: the registry refuses a name already connected (`ErrDuplicateClient`), otherwise marks it connected with the time and the remote address, and marks its record dirty.
3. Serve: the hub's reader of the connection is a frame reader bounded by the managed read maximum; a longer line ends the connection and records the rejection cause.
4. Fetch: `handleRequest` config-fetch updates the record with the time, the version the client named and the result (`served`, `current`, `error`), and marks it dirty.
5. Ack: `handleConfigAck` updates the record with the time, the result (`applied`, `refused`, `holding`), the acked version and the error text bounded by `managedDetailMax`, and marks it dirty.
6. Disconnect: the deferred cleanup of `handleConn` marks the record offline with the time and marks it dirty.
7. Persist: one long-lived writer takes a dirty name, encodes its record and writes it under `meta/fleet/device/<name>/record`; a failure is logged, kept for doctor, and the record stays dirty until the next event for that device.
8. Config write: the observer, for a written client config, pushes config-changed (unchanged) and marks that client's hub version stale. For a written or removed device record key it marks that device for reconciliation and reads nothing; the record writer then reads the key back and compares it with the bytes it last wrote (R-4).
9. Start: the registry lists the stored records, decodes each strictly, and holds every device offline; a record that does not decode is kept as unreadable.
10. Reload: the pre-check extracts the hub config (refusing an invalid one before any step applies); after promotion, the hub applies the serving blocks to the managed server: added names become served, removed names have their session closed and their record marked removed, a changed secret closes the session it authenticated, an added address is bound, a removed address is closed.
    - Certificate, pre-check: after the new pki store is installed and before any consumer applies, the reload resolves the managed certificate name of the new hub config in that store, beside the web and looking glass names. A name that does not resolve refuses the reload whole and restores the prior store.
    - Certificate, apply, before any address is bound. With no managed listener bound, the server takes the certificate source the new config asks for: the named certificate, or the issued leaf. With a listener bound and the issued leaf serving and no name configured, the leaf's hosts become `managedLeafHosts` of every serving address of the new config, and the leaf is issued for them at once; when the authority cannot issue, the previous leaf stays and each added address is recorded unbound with that reason instead of being bound. With a listener bound and a certificate configured that differs from the one served (another name, a name where the leaf was served, no name where one was served, or the same name resolving to other material), the served certificate stays, the apply records a pending restart with both, and logs it; a later apply whose config matches what is served clears the record.
    - The listeners keep their sockets through all of it and no session is closed for a certificate: the next handshake reads the certificate source.
11. Answer: `ManagedRegistrySnapshot` copies every declared and stored record, recomputes stale hub versions through `ReadConfig`, and lists the held clients once (`HeldClients`); the `ze-managed` handlers turn it into rows, apply the `label` pipe filter, and answer data, the same answer on every command surface.
12. Web: none in this spec. The estate part that reads these answers for the fleet pages is `plan/post-release/features/spec-fleet-ui-2-estate-tree-and-device-tables.md`'s (owner ruling of 2026-11-23).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Managed client to hub | TLS, `#0 auth`, newline-framed requests on `rpc.MuxConn` | No |
| Managed server to store | `storage.Storage` `WriteKey`, `ReadKey`, `ListKeys` on the borrowed store | No |
| Store to hub (observer) | One write-observer function, run after durability | No |
| Config to managed server | `ExtractHubConfig` at boot and at the reload pre-check; an apply call after promotion | No |
| pki store to managed server | The injected `TLSMaterialResolver` (`pki.ServerTLSMaterial`) at construction, at the reload's certificate gate and at the apply; the daemon root (`Authority`) for the issued leaf | No |
| Hub to managed client (TLS) | The certificate source every listener reads per handshake; the client verifies the leaf against the host it dialed | No |
| `plugin/server` to `managed` | Exported snapshot function and types (value copies) | No |
| Daemon to every surface | Builtin RPC through the dispatcher; JSON answer, pipes; CLI, SSH, MCP, REST, gRPC and the web command line | No |

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
| `show fleet devices` answer | reader | New: CLI renderers through the pipes; SSH, REST, gRPC and the web command line through the dispatcher; MCP tools derived from the registry. After the first release, the `internal/component/web/fleetui/estate` part of `plan/post-release/features/spec-fleet-ui-2-estate-tree-and-device-tables.md` | New | New |
| Hub client entry `label` (config) | writer | Operator configs; repository configs under `test/managed/` and `etc/` that declare clients | Additive optional list | Preserved: no existing config carries it |
| Hub client entry `label` (config) | reader | `internal/component/config/loader_extract.go` `ExtractHubConfig`; the YANG parser | New field on `plugin.HubServerConfig` | New |

### Integration Points
- `ManagedServer` (`managed_serve.go`) - gains the registry, the handshake cap, the read bound and an apply method for reload. Its certificate source becomes replaceable behind `getCertificate`.
- `ServingLeaf` (`internal/component/plugin/leaf.go`) - gains the replacement of its host list.
- `runReloadContext` (`cmd/ze/hub/main_reload.go`) - its certificate gate resolves the managed name too.
- `ManagedConfigService` (`managed.go`) - loses `connected`; `RegisterClient` and `UnregisterClient` move onto the registry.
- `startManagedServer` (`cmd/ze/hub/managed_server.go`) - builds the server whenever the store can serve one, publishes it for reload, extends the observer callback.
- `doReloadContext` (`cmd/ze/hub/main_reload.go`) - checks the hub extraction before any step and applies it after promotion.
- `internal/component/managed/register.go` - registers the two commands, their shapes, columns, pipe filter and completer.
- `internal/component/plugin/doctor/check_managed_runtime.go` - the new checks read the extended doctor record.
- The command surfaces - reached through the command registry with no edit to an interface package; `TestEveryCommandOnEverySurface` proves the two commands on each.
- Moved: `internal/component/web/fleetui` (parts, `ReadSnapshot`, the join) and `cmd/ze/hub/register_fleet_ui_estate.go` belong to `plan/post-release/features/spec-fleet-ui-2-estate-tree-and-device-tables.md` (owner ruling of 2026-11-23).

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Facts are written by the server at the point they happen; the commands read a published snapshot; every surface reads through the dispatcher |
| No unintended coupling (components stay isolated) | Yes | `managed` already imports `plugin/server`; no web package is touched |
| No duplicated functionality (extends existing, does not recreate) | Yes | Replaces the connected set; one store, one observer, one reload path, one snapshot pointer |
| Zero-copy preserved where applicable (refs, not copies) | Yes | The snapshot copies value records by design (a cross-boundary value, `ai/doors/system.md`); the hot path (request handling) updates a record in place under a lock |
| Registration over hardcoding, outbound | Yes | Commands, shapes, columns, the pipe filter, the completer and the doctor checks register from `init()` |
| Registration over hardcoding, inbound | Yes | Lists searched for the names this spec adds (`fleet-devices`, `fleet-device`, `label`): the wire-method snapshot (`internal/component/plugin/all/testdata/wire-methods.snapshot`, regenerated, not hand-kept), `command.AnswerSurface` (derived), the MCP tool list (derived at call time), the doctor registry (registration). The part names and `feature-gates.txt` moved with the fleet interface work to `fleet-ui-2` |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A reload does not rebuild the managed client set today | `cmd/ze/hub/main_reload.go` holds no reference to the managed server or `ExtractHubConfig`; `hubConfig` is extracted once in `cmd/ze/hub/main.go` | The reload phase would double an existing path | `fleet-devices-reload-add.ci` is red before Phase 6 and green after | unvalidated |
| A-2 | `fleet-7`'s held record (`KeyConfigHeld`, `HeldClients`) lands before this spec's Phase 4 | `fleet-7` is in progress in another session; its files are untracked on 2026-11-19 | Diverged cannot be read; Phase 4 waits | `git log -- internal/component/plugin/server/managed_held.go` before Phase 4 | unvalidated |
| A-3 | No legal client message other than config-held exceeds 64 KiB once the ack error is bounded | `client.go` `FetchConfig` (version only), `sendPing` (empty), config-changed answer (OK), `SendConfigAck` (bounded here) | The bound closes a healthy client | `TestManagedReadMaxAcceptsLargestHeld`, and a read of every `CallRPC` and `Send*` in `client.go` at implementation | unvalidated |
| A-4 | A builtin RPC handler receives its command's declared pipe filter the way a plugin command does | `RegisterPipeFilters` resolves by command path, not by owner (`pipe_filter.go`) | The label filter needs another route | `TestShowFleetDevicesLabelFilter` through the dispatcher | unvalidated |
| A-6 | `startManagedServer` receiving a nil store is the only "storage cannot serve" case | `startManagedServer` returns nil on `store == nil`; the comment says "storage is blob-backed" | A hub with client entries on non-blob storage would serve nothing silently | Read the store construction in `cmd/ze/hub/main.go` at Phase 6; if another case exists, it gets the `doctor-managed-store-unavailable` check of the Integration Checklist | unvalidated |
| A-7 | A `.ci` can start a hub, a managed client and a reload in one test | `test/managed/fleet-status.ci`, `fleet-disable-severs.ci` run hub and client daemons | The reload proof needs a Go integration test instead | Writing `fleet-devices-reload-add.ci` | unvalidated |
| A-9 | 4,000 file descriptors are available to the perf test process | Linux defaults; macOS soft limit is 256 | The perf test fails on setup, not on the hub | The test raises the soft limit to the hard limit and fails naming both when the hard limit is too low | unvalidated |
| A-10 | A functional test can run a managed client that verifies the hub's certificate against the hub root, on an address other than `127.0.0.1` | `test/managed/managed-hub-ca-trust.ci` runs the fixture `managed/hub-ca-trust`, which proves the trust; most other managed `.ci` files skip the certificate check and say so. Linux routes all of `127.0.0.0/8` to the loopback, so `127.0.0.2` binds with no setup | The proof of AC-35 moves to a Go integration test in `cmd/ze/hub` with a real `tls.Dial` | Writing `fleet-reload-listener-verified.ci` | unvalidated |
| A-11 | The store's write observer runs for a removed key as it does for a written one | `storage/store.go` `guard.Release` runs the observer for the guard's keys after durability and unlock; `RemoveKey` takes a guard | An online delete is not noticed, and R-4's reconciliation needs another signal | `TestRegistryOnlineDeleteForgetsRemovedDevice` through a real store | unvalidated |
| A-12 | Comparing the leaf certificate bytes of the served chain with those the name resolves to in the new store tells whether the material changed | `pki.ServerTLSMaterial` returns the leaf first (`docs/architecture/pki/tls-listeners.md`, "the leaf comes first") | A renewed certificate is not reported as pending, or an unchanged one is | `TestManagedApplySameNameNewMaterialPendsRestart` and its unchanged-material counterpart | unvalidated |

A-5 (the web interface runs inside the hub daemon) and A-8 (the browser test
runner can start a hub with a real managed client) moved with the fleet
interface work to
`plan/post-release/features/spec-fleet-ui-2-estate-tree-and-device-tables.md`,
as its A-1 and A-2 (owner ruling of 2026-11-23).

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Reconnect storm: after a hub restart 2,000 devices connect, fetch and ack within seconds, each event a durable write of two fsyncs, so up to 6,000 writes and 12,000 fsyncs | The perf test's wall clock; `ze_managed_device_records_written_total` rising faster than devices | One writer with a dirty set: events update memory and mark the device; the writer persists one record at a time, so several events for one device before the writer reaches it are one write. Writes are at most one in flight and at most one pending per declared device. A crash loses at most the facts not yet written, which the next event rewrites |
| R-2 | Store write failure (disk full, store read-only) | `WriteKey` error | The in-memory record stays right and answers; the device stays dirty and is retried at its next event; the failure is logged under `doctor-managed-device-record-unwritable` and kept per device; `show fleet device` answers `persisted: false`; doctor reports problem, why (the store's error) and fix |
| R-3 | Reload removes a connected device | The device's session after the reload | The session is closed by the apply, its pending push dropped, its record kept and marked removed with the time; a reconnect is then an unknown name, refused and recorded by the existing doctor record |
| R-4 | Unreadable stored record (corrupt bytes, a field this build does not know) | Strict decode error at `Start` | Never dropped and never read as absent: listed with `unreadable: true` and the device's declared facts; logged under `doctor-managed-device-record-unreadable`; replaced by the device's next event; for a removed device, which has no next event, the fix is `ze data rm meta/fleet/device/<name>/record`, run while the hub runs. The held record is no precedent for it: that key sits under `meta/config/`, which a running daemon refuses to remove, while the device record is a runtime key that `delete data key` accepts (`storage.RuntimeKey`; `ze data rm` goes through the daemon while ze runs). So the registry answers for what an online removal or overwrite does to its memory. The observer marks the device, and the record writer reads the key back. Bytes equal to the ones it last wrote are its own write: nothing happens. A key that is gone, for a device no serving block declares: the device leaves the registry and `show fleet devices` at once, which is how an operator forgets a removed device. A key that is gone, for a declared device: its history fields are cleared, its live facts (connected, since when, source address) stay, and its next event writes a new record. Any other bytes (an `update data key`, or `ze data write`): memory is the authority while the hub runs, so the registry logs the overwrite and writes its own record back. With the hub stopped, `ze data rm` removes the key and the next start loads no record for it |
| R-5 | Unbounded concurrent connections after the cap lift | Goroutine count and memory under a connection flood | Unauthenticated handshakes are capped at `managedMaxHandshakes` (32, the acceptor's figure) and each is timed out at `managedAuthTimeout`; authenticated sessions exist only for declared names, one per name, so sessions are at most the declared clients (at most `ManagedClientsMax`, 4,096) |
| R-6 | The lowered read bound closes a healthy client | `doctor-managed-client-rejected` with the oversize cause | The bound is derived from `fleet.HeldConfigMax` (base64 length plus 64 KiB), the ack error is bounded at the client, and a test sends the largest legal config-held |
| R-7 | Worst-case memory: a bufio scanner keeps the buffer it grew to | Heap after many large config-held uploads | Stated: 64 KiB per connection initially, at most the managed read maximum (about 1.4 MiB) after a device uploads its largest copy; the worst case is declared clients times that maximum, written in `docs/architecture/fleet-config.md` and asserted by a test of the reader's maximum |
| R-8 | A reload apply fails part way (an added address does not bind) | `doctor-managed-listener-unbound` | The apply binds independently per address like `Start`; a failed address is recorded unbound and its clients are not connectable there; the rest of the reload stands, as at boot |
| R-9 | A device name containing `/` or matching the client config key form corrupts a key or triggers a push | Key derivation | Names are bounded by YANG (`1..64`); the record key is `meta/fleet/device/<name>/record`, whose basename `record` never satisfies `ClientNameFromConfigKey`, so the observer never mistakes a record write for a config write; a name with `/` is refused at extraction |
| R-10 | Two serving blocks declare one client with different labels | Extraction | Refused at verify with both blocks and the label named, beside `checkManagedCertificateAgreement` |
| R-12 | `show fleet devices` on a page render reads 4,096 client configs to hash them | Page latency | Hub versions are cached and recomputed only for names the observer marked stale; at `Start` all are stale and are computed once, on the first read |
| R-13 | A secret change by reload leaves a session authenticated with the old secret | Session after reload | The apply closes the session of a client whose secret changed; the client reconnects with the new one |
| R-15 | A reload binds an address whose host the served leaf does not name, so every verifying client refuses the hub there | A client's handshake error on the new address; before this revision, by design | The apply re-derives the issued leaf's hosts from every serving address of the new config and issues before it binds (AC-35, AC-36). A named certificate is the operator's material: Ze does not reissue it, and whether it names the new host is the operator's to arrange, as at boot |
| R-16 | The authority cannot issue the leaf for the new hosts at a reload | The issue error at the apply | The previous leaf and host list keep serving the bound addresses; each added address is left unbound and recorded with the issue error under `doctor-managed-listener-unbound`; the rest of the reload stands, as for an address that does not bind (R-8, AC-40) |
| R-17 | A serving hub's certificate is changed by reload and the operator believes it is served | `doctor-managed-certificate-restart-pending` | By C-7 the listener keeps what it serves. The apply compares the configured certificate with the served one at every reload, records a pending restart with both, logs a warning carrying the code and `restart ze`, and doctor reports it until a restart, or until a reload puts the config back (AC-38). A renewed certificate under an unchanged name is the same case, found by comparing the leaf (A-12) |
| R-18 | A reload names a managed certificate the pki store does not hold; accepted, it would leave the next restart with no managed listener, because a name fails closed at construction | The resolver's error at the reload's certificate gate | Refused whole at the gate the web and looking glass names use, before any consumer applies, with the prior pki store restored (AC-39) |
| R-19 | The certificate source is replaced while a handshake reads it | A race report | The source is read through one synchronised holder per handshake, as the web listener's is; the apply replaces it only when no managed listener is bound, and otherwise changes the leaf holder's hosts under that holder's own lock |

R-11 (the sample build and the real estate part both registering the estate
part) and R-14 (the estate part dispatching as an identity that may not read
`show fleet devices`) moved with the fleet interface work to
`plan/post-release/features/spec-fleet-ui-2-estate-tree-and-device-tables.md`,
as its R-1 and R-2 (owner ruling of 2026-11-23).

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Managed clients refused or disconnected (read bound, handshake cap, reload apply); a hub that no longer starts its managed server |
| How is it reverted? | Single commit revert per phase; the new key family is inert when unread |
| Who else touches this path? | `fleet-7` (held records, `managed.go`, `managed_serve.go`, `keys.go`, closed 2026-11-19), `fleet-4` and `fleet-8` (their reports on the same managed server), `fleet-ui-2` (the fleet interface's estate part that reads these commands, after the first release) |

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
| A reload adding the first serving block on `127.0.0.2`, then a client that verifies the hub's certificate dialing it | → | the apply's leaf host re-derivation before the bind | `test/managed/fleet-reload-listener-verified.ci` |
| A reload changing `plugin hub server <block> certificate` on a serving hub, then `ze doctor` | → | the apply's comparison, the pending-restart record and its doctor check | `test/managed/fleet-reload-certificate-pending.ci` |
| A reload naming a managed certificate the pki block does not define | → | the reload's certificate gate | `TestReloadRefusesUnresolvableManagedCertificate` (`cmd/ze/hub`) |
| `ze data rm meta/fleet/device/<name>/record` on a running hub | → | the observer's mark and the writer's reconciliation | `TestRegistryOnlineDeleteForgetsRemovedDevice` |
| A hub restart | → | the registry's load at `Start` | `test/managed/fleet-devices-restart.ci` |
| A store write failure | → | the record writer's failure path and doctor | `TestRegistryWriteFailureReportedByDoctor` |
| `show fleet devices` and `show fleet device name <name>` on SSH, MCP, REST, gRPC and the web command line | → | the same builtins listed and routed through the daemon dispatcher | `TestEveryCommandOnEverySurface` (`cmd/ze/hub/surface_parity_test.go`, tags `ze_ssh ze_web`) |
| `show fleet devices \| label site=lon` sent to the daemon dispatcher every surface calls | → | the declared `label` pipe filter on the builtin's answer | `TestShowFleetDevicesLabelFilter` (`internal/component/managed/cmd_devices_test.go`) |
| 2,000 clients | → | handshake cap, read bound, registry | `TestFleetManyClientsPerf` (`fleetperf`) |

The two web rows (`test/web/fleet-estate-live.wb`,
`TestEveryAbsentPartRendersNotAvailable`) moved to
`plan/post-release/features/spec-fleet-ui-2-estate-tree-and-device-tables.md`
(owner ruling of 2026-11-23).

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
| AC-34 | `show fleet devices`, `show fleet devices \| label site=lon` and `show fleet device name edge-01` asked on the CLI, SSH, MCP, REST, gRPC and the web command line | Each surface lists both commands and the daemon dispatcher serves them there: `TestEveryCommandOnEverySurface` (`cmd/ze/hub/surface_parity_test.go`, built only under `ze_ssh && ze_web`) proves the listing and the routing, and runs no pipe and compares no answer. The `\| label site=lon` filter is proven where every surface reaches it, at the daemon dispatcher: `TestShowFleetDevicesLabelFilter` sends `show fleet devices \| label site=lon` through the dispatcher and asserts exactly the `site=lon` rows (A-4). Every surface's answer is the one the builtin handler gives the dispatcher, whose content the `.ci` tests of AC-1 to AC-13 assert | None: a command is declared once and reaches every interface (`ai/doors/system.md`); added by the owner ruling of 2026-11-23 so the registry is reachable and tested on every command surface in the first release |
| AC-35 | A hub started with no client entry and no `certificate` gains by reload a serving block on an address whose host is not `127.0.0.1` (the tests use `127.0.0.2`). A client anchored on the hub's root, with certificate verification on, dials that address | The handshake verifies, because the leaf the listener presents names that host; the client authenticates and is listed connected. Red before this design: the leaf named `127.0.0.1` alone, and the client refused it | A listener on an unspecified address (`0.0.0.0`, `::`) adds no host, because it names none a client can verify (`managedLeafHosts`, unchanged). The same reload on `127.0.0.1` proves nothing and is not the test |
| AC-36 | A hub serving clients on one address, with the issued leaf, gains by reload a second serving block on another host | A verifying client dialing the new address is admitted; the sessions on the first address stay up through the reload and its listener keeps its socket | None |
| AC-37 | A hub with no managed listener bound gains by reload its first serving block, which names `certificate fleet-hub`, defined in `pki` | The listener presents the `fleet-hub` chain from its first handshake, never the issued leaf | None: the name wins whenever it is set, and no listener was serving another certificate |
| AC-38 | On a hub whose managed listener is serving, a reload changes the certificate: a name set where the issued leaf was served, a name changed, a name removed, or the same name whose `pki` material changed | The reload is accepted. The listener keeps presenting the certificate it presented and no session is closed. The apply logs a warning, and `ze doctor` reports `doctor-managed-certificate-restart-pending`: what is wrong (the managed listener serves `<served>` while the configuration names `<configured>`), why (the managed listener reads its certificate when ze starts), and the fix `restart ze`. A reload that puts the configured certificate back to the served one clears it; after a restart the listener serves the configured certificate and the warning is gone | The issued leaf is named "the certificate ze issues" in the message, because it has no configured name. The hosts of the issued leaf are not a certificate change and follow the reload (AC-35) |
| AC-39 | A reload whose hub config names a managed `certificate` the new `pki` store does not hold | The reload is refused whole before any consumer applies, the prior `pki` store is restored and the running config is kept. The error names `plugin hub server <block> certificate <name>`, says the name is not defined in `pki`, and gives the fix: define `pki certificate <name>`, or remove the leaf | None: a name that resolves to nothing is an error, never a fallback |
| AC-40 | At a reload that adds a serving address, the daemon's authority cannot issue the leaf for the new hosts | The added address is not bound and `ze doctor` reports it under `doctor-managed-listener-unbound` with the issue error; the previous leaf keeps serving the addresses already bound, and the rest of the reload stands | The clients of the added block cannot connect there until a reload or restart issues the leaf, because binding it would present a leaf they refuse |
| AC-41 | A hub starts with a serving block whose `certificate` name does not resolve, then a reload defines the name in `pki` | At start the managed server exists, binds no address and reports each one unbound with the unresolved name: nothing is served, which is the fail-closed rule. The reload binds the addresses and presents the named certificate | None: a server that was not built at start would ignore the reload that repairs it |
| AC-42 | `ze data rm meta/fleet/device/<name>/record` (or `delete data key`) on a running hub | For a device no serving block declares, the device leaves `show fleet devices` at once. For a declared device, its history fields are cleared, its live facts stay, and its next event writes a new record. A record overwritten with other bytes is logged and written back from memory | The registry's own writes change nothing, because the writer reads back the bytes it last wrote |

AC-14, AC-15 and AC-28 to AC-33 moved to
`plan/post-release/features/spec-fleet-ui-2-estate-tree-and-device-tables.md`
(the table under "Owner ruling of 2026-11-23" in the Task section names each
one's AC there).

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Lists the fleet after a client connects | client auth → `handleConn` → registry → writer → `show fleet devices` | `fleet-devices-connect.ci` |
| 2 | Adds a client by editing the config and reloading | reload pre-check → promotion → apply → client auth → listed | `fleet-devices-reload-add.ci` |
| 3 | Removes a client by reload | apply → session closed → listed removed | `fleet-devices-reload-remove.ci` |
| 4 | Restarts the hub and sees every device offline | `Start` load → snapshot | `fleet-devices-restart.ci` |
| 5 | Finds the London devices | `show fleet devices \| label site=lon` | `fleet-devices-label.ci` |
| 6 | Lists the fleet from an MCP, REST, gRPC or SSH client, or the web command line | surface → dispatcher → `ze-managed:fleet-devices` → snapshot | `TestEveryCommandOnEverySurface` |
| 7 | Turns a running router into a hub by reload: adds a serving block on its own address and a client, and the device connects with certificate verification on | reload → certificate gate → apply (leaf hosts, bind) → client handshake → auth → listed | `fleet-reload-listener-verified.ci` |
| 8 | Changes the hub's certificate by reload and asks doctor what is pending | reload → apply (comparison, pending record) → `ze doctor` | `fleet-reload-certificate-pending.ci` |

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
| `TestManagedApplyNewAddressVerifiesAgainstRoot` | same | AC-35: a server built with no listener, an apply that adds `127.0.0.2`, then a TLS client with the authority root and server name `127.0.0.2` completes the handshake; with the leaf left as built it fails on the host name | |
| `TestManagedApplySecondAddressKeepsSessions` | same | AC-36: the first address's listener and session survive, the second verifies | |
| `TestManagedApplyFirstBindServesNamedCertificate` | same | AC-37: the handshake presents the named chain | |
| `TestManagedApplyCertificateChangePendsRestart` | same | AC-38: name set, changed and removed on a bound server; the served chain is unchanged and the doctor snapshot holds the pending record with both certificates | |
| `TestManagedApplySameNameNewMaterialPendsRestart` | same | AC-38, A-12: other material under one name pends; unchanged material does not | |
| `TestManagedApplyCertificateRestoredClearsPending` | same | AC-38: the record clears when the config matches what is served | |
| `TestManagedApplyLeafIssueFailureLeavesAddressUnbound` | same | AC-40, with an authority that refuses to issue | |
| `TestManagedStartUnresolvedCertificateBindsNothingThenReloadBinds` | same | AC-41 | |
| `TestServingLeafSetHostsIssuesForNewHosts` | `internal/component/plugin/leaf_test.go` | The holder's leaf names the new hosts after the replacement, and reissues for them as it ages | |
| `TestServingLeafSetHostsKeepsLeafOnIssueFailure` | same | The previous leaf and hosts stay when the authority cannot issue | |
| `TestReloadRefusesUnresolvableManagedCertificate` | `cmd/ze/hub/main_reload_test.go` | AC-39: refused before any consumer applies, the prior pki store restored, the message's three parts | |
| `TestCheckManagedCertificateRestartPending` | `internal/component/plugin/doctor/check_managed_runtime_test.go` | AC-38 doctor text: served, configured, why, `restart ze` | |
| `TestRegistryOnlineDeleteForgetsRemovedDevice` | `internal/component/plugin/server/managed_registry_test.go` | AC-42 and A-11, through a real store: a removed device leaves the snapshot | |
| `TestRegistryOnlineDeleteClearsDeclaredHistory` | same | AC-42: history cleared, live facts kept | |
| `TestRegistryOnlineOverwriteWrittenBack` | same | AC-42: foreign bytes are logged and replaced; the registry's own write changes nothing | |
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
| `TestShowFleetDevicesLabelFilter` | same | AC-10, and AC-34's label filter: `show fleet devices \| label site=lon` sent through the daemon dispatcher answers exactly the `site=lon` rows (A-4) | |
| `TestShowFleetDeviceAnswersDetail` | same | AC-11 | |
| `TestShowFleetDeviceUnknownName` | same | AC-12 | |
| `TestShowFleetDevicesNotServing` | same | `ErrManagedNotServing` answers an operator error with its fix | |
| `TestCheckManagedDeviceRecordUnwritable` | `internal/component/plugin/doctor/check_managed_runtime_test.go` | AC-21 doctor text | |
| `TestCheckManagedDeviceRecordUnreadable` | same | AC-22 doctor text | |
| `TestCheckManagedHandshakeSaturated` | same | AC-27 | |
| `TestEveryCommandOnEverySurface` | `cmd/ze/hub/surface_parity_test.go` (existing, `//go:build ze_ssh && ze_web`) | AC-34: both builtins listed and routed on every operator surface; it runs no pipe and compares no answer. Run with both tags: `./le job run label unit-pkg quiet command go test -race -tags "ze_ssh ze_web" -run TestEveryCommandOnEverySurface ./cmd/ze/hub/` | |

The fleet interface tests (`source_parts_test.go`, `estate_test.go`,
`sample/register/register_test.go`, `fleet_ui_bounds_test.go`) moved with
their criteria to
`plan/post-release/features/spec-fleet-ui-2-estate-tree-and-device-tables.md`
(owner ruling of 2026-11-23).

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
| `fleet-reload-listener-verified` | `test/managed/fleet-reload-listener-verified.ci` | A hub with no client entry gains by reload a serving block on `127.0.0.2`; a client anchored on the hub root, certificate verification on, connects there and is listed connected (AC-35). Red before the leaf's hosts follow the reload: the client refuses the hub's certificate. It follows `managed-hub-ca-trust.ci`, the managed test that keeps the certificate check | |
| `fleet-reload-certificate-pending` | `test/managed/fleet-reload-certificate-pending.ci` | A serving hub's reload sets `certificate fleet-hub`; the connected client stays connected, `ze doctor` reports `doctor-managed-certificate-restart-pending` with `restart ze`; after a restart the warning is gone (AC-38) | |
| `TestFleetManyClientsPerf` | `cmd/ze/hub/fleet_perf_test.go` (`fleetperf`) | 2,000 connected devices | |

### Interop Tests (Scope: protocol)
- N/A: the managed channel has no peer implementation other than Ze's own client, which the functional tests run.

## Files to Modify
- `internal/component/plugin/server/managed.go` - remove `connected`; `RegisterClient` and `UnregisterClient` move to the registry.
- `internal/component/plugin/server/managed_serve.go` - handshake cap replacing `managedMaxConns`; registry calls at admit, fetch, ack, disconnect; bounded reader; zero-listener start; apply for reload; `Serves` reads the live served set.
- `internal/component/plugin/server/managed_serve.go` (the certificate) - `managedCertificate` builds a certificate source the server can replace, read per handshake through the one `getCertificate`; a name that does not resolve at construction leaves the server built with nothing bound and each address reported unbound.
- `internal/component/plugin/leaf.go` - `ServingLeaf` gains the replacement of its host list, issuing at the call and keeping the previous leaf on failure.
- `internal/component/plugin/server/managed_doctor.go` - record write and read failures per device, handshake saturation, the oversize cause, the certificate a restart is pending for.
- `pkg/plugin/rpc/framing.go`, `pkg/plugin/rpc/conn.go` - a reader and a connection with a caller-given maximum.
- `pkg/fleet/envelope.go` - `AckErrorMax`.
- `internal/component/managed/client.go` - bound the ack error.
- `pkg/zefs/keys.go` - `KeyFleetDeviceRecord`.
- `internal/component/plugin/yang/ze-plugin-conf.yang` - `label` list under the hub `client` entry.
- `internal/component/plugin/types.go` - labels on `HubServerConfig`.
- `internal/component/config/loader_extract.go` - extract labels; refuse disagreeing labels and more than 4,096 names.
- `cmd/ze/hub/managed_server.go` - build when the store can serve, publish for reload, observer marks stale.
- `cmd/ze/hub/main.go` - install the targets whatever the client count.
- `cmd/ze/hub/main_reload.go` - pre-check extraction, apply after promotion; the certificate gate resolves the managed certificate name beside the web and looking glass names.
- `internal/component/managed/register.go`, `internal/component/managed/yang/ze-managed-cmd.yang` - two commands, shapes, columns, filter, completer.
- `internal/component/plugin/doctor/check_managed_runtime.go` - four checks.
- `internal/core/diagnostic/codes_plugin_infra.go` - four codes and their entries.
- `ai/doors/hub-engine.md` - Boundaries, "One reload path": the sentence "nothing else in the config needs one" names the second thing that waits for a restart, a changed certificate on a serving managed listener, and that its reload is accepted and reported, not refused. Traps, "Something built only at startup ignores every reload": the managed certificate is named as the case kept at startup by decision, with its doctor code. Edited in Phase 6, with the code that makes it true.
- `cmd/ze/hub/fleet_perf_test.go` - 2,000 held sessions.
- `docs/architecture/fleet-config.md`, `docs/guide/fleet-config.md`, `docs/guide/command-reference.md`, `docs/architecture/api/commands.md`, `docs/architecture/zefs-format.md`, `docs/architecture/doctor-and-health-checks.md`, `docs/features/cli-commands.md`, `docs/architecture/api/ipc_protocol.md`, `docs/architecture/hub-architecture.md` - the pages each phase makes wrong.
- `docs/architecture/hub-architecture.md` ("What the daemon does at start"), `docs/architecture/pki/tls-listeners.md` ("Decision: a named certificate fails closed", "Decision: rotation does not rebind the hub listeners") and `docs/architecture/fleet-config.md` ("Hub certificate and client trust") - what the managed listener's certificate does at a reload, as "Documents departed from" states it (Phase 6). `docs/architecture/pki/pki-store.md` ("Consequences worth knowing") and `docs/architecture/api/process-protocol.md` (the `ServingLeaf` paragraph) gain that the leaf holder's hosts can be replaced, and `docs/guide/fleet-config.md` the pending-restart warning.

## Files to Create
- `internal/component/plugin/server/managed_registry.go` - record, writer, load, snapshot.
- `internal/component/plugin/server/managed_registry_test.go`
- `internal/component/plugin/server/managed_apply.go` - the reload apply.
- `internal/component/plugin/server/managed_apply_test.go`
- `internal/component/managed/cmd_devices.go` - the two handlers and the completer.
- `internal/component/managed/cmd_devices_test.go`
- `test/managed/fleet-devices-connect.ci`, `fleet-device-detail.ci`, `fleet-devices-restart.ci`, `fleet-devices-declared.ci`, `fleet-devices-reload-add.ci`, `fleet-devices-reload-remove.ci`, `fleet-devices-label.ci`, `fleet-devices-diverged.ci`, `fleet-reload-listener-verified.ci`, `fleet-reload-certificate-pending.ci`
- The fixture scenarios those two `.ci` files run, beside `managed/hub-ca-trust` under `internal/test/fixture/`, where a client that verifies the hub needs one (A-10)

The fleet interface files (`source_parts.go`, `estate/*`,
`register_fleet_ui_estate.go`, `fleet_ui_bounds_test.go`,
`fleet-estate-live.wb`, `component_unavailable.templ`, the sample's
`register_test.go`, and the edits to `source.go`, `source_devices.go`,
`register.go`, `pages.go` and `sample/register/register.go`) moved to
`plan/post-release/features/spec-fleet-ui-2-estate-tree-and-device-tables.md`
(owner ruling of 2026-11-23).

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
| Doctor code per problem class | Yes | Record write failure: `doctor-managed-device-record-unwritable`, runtime phase, `checkManagedDeviceRecordUnwritable`. Unreadable stored record: `doctor-managed-device-record-unreadable`, runtime phase. Handshake cap reached: `doctor-managed-handshake-saturated`, runtime phase. Oversize line: existing `doctor-managed-client-rejected` with new cause `ManagedRejectOversize`. Duplicate session: existing `doctor-managed-client-rejected` (`ManagedRejectNameConnected`). Removed client reconnecting: existing `doctor-managed-client-rejected` (`ManagedRejectUnknownName`). Reload address that does not bind: existing `doctor-managed-listener-unbound`, now also recorded by the apply. Reload address left unbound because its leaf could not be issued, and an address left unbound at start because the certificate name did not resolve: the same code, with that reason. Certificate changed by reload on a serving hub: `doctor-managed-certificate-restart-pending`, runtime phase, a warning with the fix `restart ze`. Reload naming a certificate `pki` does not define: a refusal of the reload whose message carries the fix; the config is not applied, so no runtime state remains. Disagreeing labels and too many clients: refusals at verify, whose message carries the fix; no runtime state remains, so no doctor check. `show fleet device` unknown name and not serving: operator errors with their fix; no runtime fault. All in `internal/component/plugin/doctor/check_managed_runtime.go` and `internal/core/diagnostic/codes_plugin_infra.go`. If A-6 finds a non-blob store case, `doctor-managed-store-unavailable` is added there |
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
| 6 | Has a user guide page? | Yes | `docs/guide/fleet-config.md` (registry, labels, reload, scale, the commands on every command surface). `docs/guide/web-interface.md` (real estate, not-available parts) moved with the web work to `fleet-ui-2` |
| 7 | Wire format changed? | Yes | `docs/architecture/fleet-config.md` "Protocol": hub read bound, ack error bound |
| 8 | Plugin SDK/protocol changed? | Yes | `pkg/plugin/rpc` gains a bounded reader: `docs/architecture/api/process-protocol.md` framing paragraph names that the plugin protocol keeps 16 MiB |
| 9 | RFC behavior implemented, changed, or newly proven? | N/A | No RFC |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` if the perf harness invocation changes (`./le test integration stress-fleet` names 2,000) |
| 11 | Affects daemon comparison? | N/A | No comparison row covers fleet management |
| 12 | Internal architecture changed? | Yes | `docs/architecture/fleet-config.md` (registry, cap, reload, and "Hub certificate and client trust": the leaf's hosts and the certificate under reload); `docs/architecture/pki/tls-listeners.md` (the managed name at the reload's gate, the managed listener's row); `docs/architecture/pki/pki-store.md` and `docs/architecture/api/process-protocol.md` where they describe `ServingLeaf` (its hosts can be replaced); `docs/architecture/hub-architecture.md`; `ai/doors/hub-engine.md`. `docs/architecture/fleet-interface.md` (parts, join, not available) moved with the web work to `fleet-ui-2` |
| 13 | Route metadata keys added/changed? | N/A | No route metadata |
| 14 | Prometheus counters added/changed? | Yes | `docs/guide/monitoring.md` managed metrics section, `docs/plugin-development/metrics.md` (it mentions `managed_serve.go`) |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/guide/status.md` command inventory rows |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Run `./le spec citation anchors spec plan/release/features/spec-fleet-1-device-registry.md` at each phase; the `// Design:` docs of the changed files are `docs/architecture/fleet-config.md`, `docs/architecture/api/commands.md`, `docs/architecture/zefs-format.md`, plus three the anchors report found on 2026-11-19: `docs/architecture/api/ipc_protocol.md` (declared by `pkg/plugin/rpc/framing.go`: updated, the bounded reader is a framing change), `docs/architecture/hub-architecture.md` (declared by `cmd/ze/hub/main.go` and `main_reload.go`: updated, "What the daemon does at start" and reload now carry the managed client set), `ai/rules/repo-maintenance.md` (declared by `internal/component/managed/register.go`: unaffected, it governs the doctor check registration this spec follows without changing the rule). Advisory mentions (`docs/plugin-development/metrics.md` for `managed_serve.go`, `docs/why-ze.md` for `framing.go`, the radius, tacacs and install guides for the hub files) are checked at each phase and edited only where a stated fact changes |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/fleet-config.md` examples of `server central { client ... }` gain a label; `docs/architecture/fleet-config.md` "Roles" and "Per-Client Secrets" tables checked against the YANG |

### Repo-maintenance discovery
| Question | Answer |
|----------|--------|
| Where an agent looks first | `ai/INDEX.md` fleet row and `ai/doors/hub-engine.md` "managed (fleet) client and server" point to `docs/architecture/fleet-config.md`, which gains a "Device registry" section |
| What prevents regression | The `.ci` tests above, `TestEveryCommandOnEverySurface` for the two commands, the doctor code tests |
| What registry prevents drift | `pkg/zefs/keys.go`, the RPC registry and its wire-method snapshot, the doctor registry |
| What verification proves it | The owed scoped package tests and `./le test managed` of the listed `.ci` |

## Design presentation

### What changes

The hub keeps one record per declared managed client inside its managed
server, written at connect, disconnect, config fetch and config ack, persisted
in the hub's store under `meta/fleet/device/<name>/record` by one coalescing
writer, and loaded at start. An operator runs `show fleet devices` (filtered
with `| label <key>=<value>`) and `show fleet device name <name>` on the CLI,
SSH, MCP, REST, gRPC or the web command line and sees every declared device,
whether it is connected, when it was last seen, its labels, its version and
where its configuration stands, across a hub restart and a config reload. A
client entry gains labels. A config reload now adds and removes managed
clients without a restart. The cap of 128 connections held for life becomes a
cap of 32 unauthenticated handshakes, with sessions bounded by the declared
clients (at most 4,096) and the hub's reader of the managed channel bounded
near 1.4 MiB, so 2,000 connected devices are proven. `ze doctor` reports a
record that cannot be written or read and a saturated handshake cap. Because
a reload now binds serving addresses, the certificate they present follows
it: the leaf the hub issues names the host of every address served after the
reload, a hub with no managed listener yet serves the certificate its new
config names, and a name `pki` does not define refuses the reload. One case
waits for a restart, as Thomas decided (C-7): a changed certificate on a hub
whose managed listener is already serving. That reload is accepted, and a log
line and `ze doctor` say a restart is pending. The web
pages that read this registry come after the first release
(`plan/post-release/features/spec-fleet-ui-2-estate-tree-and-device-tables.md`).

### Before and after

1. `show fleet devices` on any command surface. Before: no such command; the hub
   knows only which names are connected now, in memory. After: one row per
   declared or recorded device, for example `edge-01`, `connected` true,
   `config-state` `in-sync`, `labels` `site=lon`, `hub-version` and
   `device-version` `3f9a1c2e7b4d5a60`, `last-seen` empty while connected.
2. `show fleet devices | label site=lon`. Before: no command. After: exactly the
   rows whose labels include `site=lon`; a removed device never matches.
3. `show fleet device name edge-01`. Before: no command. After: one document with
   every row field plus `first-connected`, `last-ack`, `ack-version`,
   `ack-error`, `persisted`, `unreadable` and `removed-at`.
   `show fleet device name nobody` answers an operator error whose fix names
   `show fleet devices`.
4. A reload that adds `client edge-09`. Before: `edge-09` is refused as an
   unknown name until the hub restarts. After: it is listed at once (never
   connected), can authenticate, and is listed connected once it does.
5. A reload that removes a connected `client edge-02`. Before: its session stays
   up and the hub keeps serving it until a restart. After: its session closes
   within the reload, it is listed `removed` true with `removed-at`, its stored
   facts, held record and config file are kept, and its reconnect is refused as
   an unknown name and recorded by doctor.
6. A hub restart. Before: nothing about any device survives. After: every stored
   device is listed `connected` false with its stored facts until it reconnects.
7. 2,000 devices connect. Before: the 129th connection waits for a slot, because
   each connection holds one of 128 for its life. After: all 2,000 hold sessions;
   only a 33rd unauthenticated handshake waits, for at most the handshake
   timeout.
8. A config that declares one client under two serving blocks with different
   labels, or more than 4,096 client names. Before: a client entry has no
   `label`, and no bound on the number of client names exists in
   `loader_extract.go` or the YANG. After: refused at verify, at
   `ze config validate`, boot and reload alike, naming the client, both blocks
   and the label, or the count and the bound.
9. `ze doctor` after a store write fails. Before: no record exists to fail.
   After: `doctor-managed-device-record-unwritable` names the device, the store
   error and the fix, and `show fleet device` answers `persisted` false with the
   right live facts.
10. A reload that changes `plugin hub server central certificate` on a hub
    whose managed listener is serving, for example from no name to
    `fleet-hub`. Before: the reload is accepted, the listener keeps the leaf
    ze issued, and nothing says so. After: the same, by Thomas's decision at
    the final gate of 2026-11-19 (C-7), and the operator is told. The log
    carries a warning, and `ze doctor` reports
    `doctor-managed-certificate-restart-pending`: "the managed listener serves
    the certificate ze issues while the configuration names fleet-hub", "the
    managed listener reads its certificate when ze starts", fix `restart ze`.
    After the restart the listener serves `fleet-hub` and the warning is
    gone. Renewing `fleet-hub` in `pki` under the same name is the same case.
    This is the departure below.
11. A router that serves no managed client becomes a hub by reload: the
    operator adds `server edge { ip 192.0.2.1; port 1791; client edge-01 {...} }`
    with no `certificate`, and `edge-01` dials `192.0.2.1:1791` anchored on
    the hub's root. Before (today): `edge-01` is an unknown name until a
    restart. With the refused design: the address binds, and `edge-01`
    refuses the hub, because the leaf names `127.0.0.1` alone, with an x509
    error that the certificate is not valid for `192.0.2.1`; it retries for
    ever and the hub reports nothing wrong. After: the leaf is issued for
    `127.0.0.1` and `192.0.2.1` before the address binds, and `edge-01`
    connects. A client already connected on another address stays connected.
12. The same first serving block added by reload with `certificate fleet-hub`.
    With the refused design: the listener presents the leaf ze issued, and a
    client that trusts the public CA of `fleet-hub` refuses it. After: the
    listener presents `fleet-hub` from its first handshake.
13. A reload whose config names `certificate fleet-hbu`, which `pki` does not
    define. With the refused design: accepted, and the next restart starts
    no managed listener. After: the reload is refused whole and the running
    config is kept; the error names `plugin hub server central certificate
    fleet-hbu`, says `pki` defines no such certificate, and gives the fix,
    define `pki certificate fleet-hbu` or remove the leaf.
14. `ze data rm meta/fleet/device/edge-02/record` on a running hub, for a
    device a reload removed. Before: no record exists. After: `edge-02`
    leaves `show fleet devices` at once. For a declared device the command
    clears its history and keeps its live facts.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| A separate registry component fed by events (placement B) | It misses events emitted before it subscribes and adds a second persistence path (owner, 2026-11-19) |
| A synchronous store write per event | A reconnect storm of 2,000 devices would make up to 6,000 durable writes on the request path (R-1) |
| A `fleet { device <name> { label; group; } }` root | A second declaration of a device beside the hub's `client` entry |
| `--label` flag or a `label` argument | A flag never reaches the daemon, and scope is a pipe operation (`docs/architecture/api/commands.md`, decision of 2026-08-21) |
| Builtins registered from `plugin/server` | The wire-method owner prefix belongs to the registering package; `managed` already holds the fleet commands |
| An offline route answering stored records as offline | Every fact but the stored one lives in the daemon; an offline answer would show a fleet that looks dead (owner, 2026-11-19) |
| Removed devices in the fleet interface | `fleetui` has no removed state; adding one is `fleet-ui-2`'s design (owner, 2026-11-19) |
| Apply a changed serving certificate at reload | Deferred by the owner at the final gate of 2026-11-19 (Known Limitations). It is what the design pages describe and it would end every departure below; C-10 puts it back to Thomas with what this revision found |
| Refuse the reload that changes a serving hub's certificate, with the fix `restart ze`, as a changed external plugin is refused | C-7 says the change takes effect at the next restart, which needs the new config to be the one that boots. A refused commit is not promoted, so a restart after it boots the old certificate and the operator has no way to make the change from a command surface. It would also refuse a reload over a renewed certificate that the web listener shares and can install. Listed in C-10 |
| Leave the leaf as built and document that a serving address added by reload needs a restart or a named certificate | A reload that binds an address no verifying client can use is a defect of this spec's own reload, and the address would look healthy on the hub |
| Rebind every managed listener when the leaf's hosts change | Every session would drop for a change the next handshake can read; `docs/architecture/pki/tls-listeners.md` decides a hub listener keeps its socket |
| Issue one leaf per listener, each for its own host | A second leaf holder per block and a certificate chosen per socket, where one leaf naming every served host does the job, as it does at start |
| Bind the first serving block added by reload with the issued leaf whatever name it carries, and report a pending restart | The name wins whenever it is set, and no listener was serving another certificate, so nothing in C-7 asks for it |
| Treat an online removal of a device record as invisible until the next start | The fix doctor gives for an unreadable record of a removed device would do nothing on a running hub, where the store accepts the command |
| Declare `group` in `fleet-1` | `group` is the template unit `fleet-2` answers (owner, 2026-11-19) |
| Keep the 128-connection cap held for life | 2,000 devices cannot connect; the acceptor's figure of 32 bounds the handshake flood, and configuration bounds sessions |
| A literal read maximum | The largest legal message is declared once, as `fleet.HeldConfigMax` |
| Build the managed server at reload when the first client appears | Two lifecycles, two stops, and the data and divergence targets would need reinstalling |
| Ship the estate part, the source split and the not-available state with the registry | First-release fleet features are reached through commands; the web parts moved to `fleet-ui-2` (owner ruling of 2026-11-23, "1 b") |

### Documents departed from

All five departures are one fact, decided by Thomas as C-7 at the final gate
of 2026-11-19: on a hub whose managed listener is serving, a changed
certificate (`plugin hub server <block> certificate`, or the `pki` material
that name resolves to) takes effect at the next restart. C-7 was decided
before the judge showed what else the reload does to the certificate. It is
read here as reaching that one case and no further: the hosts of the leaf ze
issues, and the certificate of a hub with no managed listener bound, are not
a change of a served certificate, and they follow the reload. C-10 asks
Thomas to confirm or change this.

- `docs/architecture/hub-architecture.md`, "What the daemon does at start".
  Statement: "the external plugins and their modules alone: every other part of
  the config reloads without a restart." After this work the page names the
  second exception beside the external plugins, and how it differs: the
  reload is accepted, the managed listener keeps its certificate until the
  restart, and `doctor-managed-certificate-restart-pending` reports it.
- `docs/architecture/pki/tls-listeners.md`, "Decision: rotation does not rebind
  the hub listeners". Statement: "A hub listener re-resolves the name in the new
  store." On a serving hub the managed listener re-resolves the name only to
  compare it with what it serves. After this work the page's table gains a
  row for the managed listener: no rotation hook on a serving listener, the
  pending restart is reported; the issued leaf's hosts and a first listener's
  certificate are installed with no rebind.
- `docs/architecture/fleet-config.md`, "Hub certificate and client trust".
  Statement: "The name wins whenever it is set". On a serving hub a name set,
  changed or removed by reload wins at the next restart. After this work the
  section says so, with the doctor code, and says that the issued leaf names
  every served host after a reload and that a name `pki` does not define
  refuses the reload.
- `ai/doors/hub-engine.md`, Boundaries, "One reload path". Statement: "they
  change only when `ze` restarts, and nothing else in the config needs one".
  After this work the sentence names the second thing that needs a restart
  and that its reload is accepted and reported, where a changed external
  plugin is refused.
- `ai/doors/hub-engine.md`, Traps, "Something built only at startup ignores
  every reload". Statement: "so a changed RADIUS or TACACS+ server was accepted
  and applied to nothing". The certificate of a serving managed listener stays
  built at startup, and a reload that changes it is accepted and applied at
  the restart. After this work the trap names it as the one case kept so by
  decision, with the warning that makes it visible.

What the operator sees for this departure is "Before and after" item 10.

### Choices for Thomas

| # | Choice | Options | Decision |
|---|--------|---------|----------|
| C-1 | Where the registry lives | (a) a record inside the hub's managed server; (b) a separate component fed by events | (a), DESIGN gate 2026-11-19 |
| C-2 | When the records are persisted | (a) on connect, disconnect, fetch and ack, under a family outside `meta/config/` and `meta/managed/`; (b) on every event, in place | (a), DESIGN gate 2026-11-19 |
| C-3 | The connection cap | (a) lift it here: a handshake cap, sessions bounded by declared clients, a bounded reader, 2,000 devices proven; (b) a later spec | (a), DESIGN gate 2026-11-19 |
| C-4 | Reload of the client set | (a) fixed here, proven by a `.ci` that adds a client by reload; (b) a later spec | (a), DESIGN gate 2026-11-19 |
| C-5 | Offline answer of the commands | (a) daemon only; (b) an offline route over the stored records | (a), final gate 2026-11-19 |
| C-6 | Removed devices | (a) listed by the commands, the interface lists declared devices only; (b) shown in the interface too | (a), final gate 2026-11-19; the interface half is `fleet-ui-2`'s |
| C-7 | A changed serving certificate | (a) needs a restart, recorded as a Known Limitation, with the page corrections above; (b) applied at reload | (a), final gate 2026-11-19. The revision of 2026-11-23 reads it as reaching a changed certificate on a serving hub only, and adds what the operator is told; C-10 reopens it with that evidence |
| C-8 | `group` and `template` | (a) `fleet-2`'s; (b) declared here | (a), final gate 2026-11-19 |
| C-9 | The fleet web parts | (a) commands in the first release, pages after it; (b) the estate part with the registry | (a), ruling of 2026-11-23 (voice-transcribed "1 b") |
| C-10 | What a reload does when it changes the certificate a serving hub presents, now that the reload must already touch the certificate (the table below) | (a) accepted, served at the next restart, reported by a log line and `ze doctor`: C-7 as decided, with the notice added; (b) the reload is refused with the fix `restart ze`; (c) applied at the reload, as the web and looking glass certificates are | Open. The spec is written for (a), which is C-7 as Thomas decided it. The author recommends (c) |

C-10 in what the operator sees. The hub `central` serves `edge-01` with the
leaf ze issued, and the operator commits `certificate fleet-hub`:

| Option | The commit | The listener | What tells the operator | Cost |
|--------|------------|--------------|-------------------------|------|
| (a) written here | Accepted | Presents the issued leaf until ze restarts, then `fleet-hub` | The log warning and `doctor-managed-certificate-restart-pending`, fix `restart ze` | Five design statements departed from and corrected. Renewing `fleet-hub` under its name (every year, or every 90 days with an automated CA) needs a restart of ze, which on a hub that routes drops every routing session. Until then the config names one certificate and the listener serves another |
| (b) | Refused: "the new config changes the certificate the managed listener serves, which changes only when ze restarts", fix `restart ze` | Unchanged | The refusal, and `doctor-config-reload-refused` | The same restart, and a refused commit is not promoted, so the restart boots the old certificate: the change can only be made by editing the config file and restarting. A renewal the web listener shares is refused with it |
| (c) recommended | Accepted | Presents `fleet-hub` at the next handshake; `edge-01` stays connected, the listener keeps its socket | Nothing to report: the config and the listener agree | None of the five departures, no doctor code, no page correction. The apply already replaces the certificate source for a hub with no listener bound and already resolves the name at the reload's gate, so (c) removes the comparison and the pending record and adds nothing. A client that cannot verify `fleet-hub` fails at its next reconnect, as it would after the restart of (a) |

Under every option the issued leaf's hosts follow the reload, a first serving
block's certificate is served at once, and a name `pki` does not define
refuses the reload: those are not choices.

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** - YANG nodes for both commands; builtin registrations answering a stub; `ManagedRegistrySnapshot` returning `ErrManagedNotServing`
   - Tests: `fleet-devices-connect.ci`, `fleet-device-detail.ci` (red: stub answers), `TestEveryCommandOnEverySurface` (AC-34)
   - Files: `ze-managed-cmd.yang`, `register.go`, `cmd_devices.go`, `managed_registry.go` skeleton
   - Verify: the commands route on every command surface and fail on content
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
6. **Phase: Reload** - always-built server, apply, pre-check, post-promotion apply; the certificate under reload: replaceable source, leaf hosts re-derived before a bind, first-bind certificate, the reload's certificate gate, the pending-restart record and its doctor check
   - Tests: `TestManagedApply*`, `TestManagedStartUnresolvedCertificateBindsNothingThenReloadBinds`, `TestServingLeafSetHosts*`, `TestReloadRefusesUnresolvableManagedCertificate`, `TestCheckManagedCertificateRestartPending`, `fleet-devices-reload-add.ci` (red first), `fleet-devices-reload-remove.ci`, `fleet-reload-listener-verified.ci` (red first: written against the apply before the leaf's hosts follow it, where the client refuses the certificate), `fleet-reload-certificate-pending.ci`
   - Files: `managed_apply.go`, `managed_serve.go`, `leaf.go`, `managed_doctor.go`, `check_managed_runtime.go`, `codes_plugin_infra.go`, `managed_server.go`, `main.go`, `main_reload.go`; `ai/doors/hub-engine.md` and the three pages of "Documents departed from"
   - If Thomas answers C-10 with (c), this phase drops the comparison, the pending record, `doctor-managed-certificate-restart-pending` and the five page corrections, and the apply replaces the certificate source on a serving hub too; AC-38 then states that the next handshake presents the configured certificate
7. **Phase: Connection bound** - handshake cap, bounded reader, ack error bound, oversize cause, metrics
   - Tests: `TestHandshake*`, `TestManagedReadMax*`, `TestFrameReaderMax*`, `TestSendConfigAckBoundsError`
   - Files: `managed_serve.go`, `framing.go`, `conn.go`, `envelope.go`, `client.go`, `managed_doctor.go`
8. **Phase: Scale** - 2,000 held sessions with the descriptor limit raised
   - Tests: `TestFleetManyClientsPerf`
   - Files: `fleet_perf_test.go`
9. **Phase: Documentation** - each phase edits its pages as it lands (the Documentation Update Checklist); this phase only checks anchors

The former Phase 8, "Fleet interface parts", moved whole to
`plan/post-release/features/spec-fleet-ui-2-estate-tree-and-device-tables.md`,
section "Moved from fleet-1 (owner ruling 2026-11-23)", with the section "The
fleet interface parts after the split" that specified it.

### The fleet interface parts (moved)

The sections "The fleet interface parts after the split", "The join by device
name", "The not-available state", "The estate part on a real build", "What
stays with `fleet-ui-2`" and "The live-estate browser test" moved, with the
criteria, tests and files they specified, to
`plan/post-release/features/spec-fleet-ui-2-estate-tree-and-device-tables.md`,
section "Moved from fleet-1 (owner ruling 2026-11-23)". There the parts that
named a first-release spec as owner are owned by the fleet-ui spec whose page
reads them (`InventoryHealthPart` and `LatestSamplePart` by `fleet-ui-2`,
`HistoryPart` by `fleet-ui-3`), and `DivergencePart`, whose owner
`spec-fleet-7-config-reconnect-resolution` closed without registering it, by
`fleet-ui-6`.


### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | Connected state has one holder; a record write never blocks a request; the observer still reads no storage; a removed client's held record and config survive |
| Naming | JSON keys kebab-case; codes `doctor-managed-*`; wire methods `ze-managed:fleet-devices`, `ze-managed:fleet-device` |
| Data flow | Facts written only in `handleConn`, `handleRequest`, `handleConfigAck` and the apply; every command surface reads only through the dispatcher |
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

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Label keys and values by YANG; device names by YANG and at extraction; strict record decode; the read bound on every hub-read line |
| Resource exhaustion | Handshake cap; sessions bounded by declared names; writer pending set bounded by declared names; unknown-name record bounded at 16 (existing) |
| Error leakage | The ack error and record errors are bounded and never carry a secret; `source-address` is shown only to callers allowed to read `show fleet` |
| Authorization | Both commands are read-only builtins under the dispatcher's authorization, on every command surface |
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
| Removed devices are listed by the CLI (`show fleet devices`, `removed` true, on every command surface) (owner, final gate 2026-11-19) | Removed devices in the interface | A removed entry keeps its record (AC-9), and the commands are this spec's surface. The other half of the owner decision, that the interface's estate part lists declared devices only because `fleetui` has no removed state, is `fleet-ui-2`'s work after the first release: `plan/post-release/features/spec-fleet-ui-2-estate-tree-and-device-tables.md`, section "Moved from fleet-1 (owner ruling 2026-11-23)", row "Removed devices" |
| A serving certificate name change needs a restart (owner, final gate 2026-11-19) | Apply the certificate at reload | Deferred by the owner; recorded in Known Limitations |
| The pending restart is accepted and reported, not refused | Refuse the reload with `restart ze` | The decision is that the change takes effect at the next restart, so the new config must be the one that boots; doctor reports every problem an operator can hit (`ai/META-DECISIONS.md`) |
| The issued leaf's hosts follow the reload through the leaf holder, with no rebind | Leave the leaf as built; rebind; one leaf per listener | A bound address must be verifiable by the clients it is bound for; the handshake already reads the holder each time |
| A hub with no managed listener bound takes the certificate its new config names | Bind with the issued leaf and report a pending restart | The name wins whenever it is set (`docs/architecture/fleet-config.md`); nothing was being served, so C-7 does not reach it |
| The managed certificate name joins the reload's certificate gate | Accept and let the next restart fail closed | One gate for every hub listener's name (`docs/architecture/pki/tls-listeners.md`) |
| The registry reconciles with an online removal or overwrite of a record | Memory only until the next start | The store accepts the command on a running hub, and it is the fix doctor names |
| `group` and `template` stay with `fleet-2` (owner, final gate 2026-11-19) | Declare `group` in `fleet-1` | `group` is the template unit; `fleet-2` answers the `AssignmentPart` |
| The interface split is specified method by method (owner, final gate 2026-11-19: strengthen the split) | Leave each other part to be designed by its own spec | Carried, with the split, to `plan/post-release/features/spec-fleet-ui-2-estate-tree-and-device-tables.md` by the owner ruling of 2026-11-23 |
| One part per owning spec | Keep `ConfigSource` whole | Carried to `fleet-ui-2` with the split (owner ruling of 2026-11-23) |
| The registry ships reachable through its commands on every command surface; its pages come after the first release (owner, 2026-11-23, "1 b") | Ship the estate part and the not-available state with the registry | `ai/META-DECISIONS.md`, "Goal and scope": the first-release fleet features are reached through commands; AC-34 proves every surface |
| Handshake cap 32 | Keep 128 held for life | The acceptor's figure for the same flood; sessions are bounded by configuration |
| Read maximum derived from `HeldConfigMax` | A literal | One declaration of the largest message |
| Always-built managed server on a store that can serve | Build at reload when the first client appears | One lifecycle, one `Stop`, and the data and divergence targets need no reinstall |

## Known Limitations
- On a hub whose managed listener is serving, a changed certificate (`plugin hub server <block> certificate`, or the `pki` material it names, a renewal included) takes effect at the next restart. Owner decision at the final gate, 2026-11-19 (C-7, Key Design Decisions); C-10 is open on it. The reload is accepted, the listener keeps the certificate it serves, a log line and `doctor-managed-certificate-restart-pending` say a restart is pending, and `docs/guide/fleet-config.md` says so with the doctor code.
- A named certificate is the operator's material: when a reload adds a serving address, Ze reissues only the leaf it issues itself. Whether a named certificate names the new host is the operator's to arrange, as at start.
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
- [ ] AC-1..AC-13, AC-16..AC-27 and AC-34..AC-42 all demonstrated (AC-14, AC-15 and AC-28..AC-33 are `fleet-ui-2`'s)
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
