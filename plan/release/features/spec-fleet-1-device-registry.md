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
(`ai/META-DECISIONS.md`, "Goal and scope"). The third was C-7's case: Thomas
had decided that a changed serving certificate takes effect at the next
restart. The revision put it back to him as C-10 with the evidence it found,
and he answered it (the next section): the reload installs the changed
certificate on the running listener (AC-38). A name that does not resolve
refuses the reload (AC-39).

The owner rule that an operator adds a peer, with any of its settings, to the
running configuration without a restart (`ai/META-DECISIONS.md`, "CLI, config,
and daemon surfaces") names routing peers, so a managed client entry is
outside its letter. Its objective holds here all the same, because a restart
of a hub that also routes drops every routing session: no reload is refused
because of what a new client entry or its serving block configures.

This revision changes `## Acceptance Criteria` and the design, and carries
choices Thomas did not make, so the design goes back to the judge and then to
him. This spec is not re-approved by this edit.

### Owner ruling of 2026-11-23: C-10, the certificate changes at the commit

Thomas answered C-10 (voice-transcribed): "Keep the certificate in the
configuration." then "In that case on commit we can perform the certificate
change on the listener." The main session confirmed the reading with him:
option (c). A reload (the commit) that changes the certificate a serving
hub's managed listener presents installs it on the running listener at that
reload, with no rebind and no session closed. That is how the web and looking
glass listeners install a changed certificate
(`docs/architecture/pki/tls-listeners.md`, "Decision: rotation does not rebind
the hub listeners"; `cmd/ze/hub/listener_migrate.go` `updateWebCertificate`,
`updateLGCertificate`, both through `installListenerCertificate`). Since
commit 2984d3eaa5 the comparison also holds when the name is removed: each
listener then presents the certificate a restart with that configuration
would serve. For the web and looking glass listeners that is their
self-signed pair, resolved through `listenerTLSMaterial`
(`cmd/ze/hub/service_tls.go`), and a looking glass that runs with no store
refuses that reload. For the managed listener it is the leaf the daemon's
certificate authority issues (`docs/architecture/fleet-config.md`, "Hub
certificate and client trust": "With no `certificate` the listener serves a
leaf the daemon's own certificate authority issued"). The listeners differ in
what their certificate is when no name is set, and in nothing else. The
certificate stays in the configuration's `pki` block, named by `plugin hub
server <block> certificate`. This reverses C-7 (final gate, 2026-11-19),
which is superseded.

What the ruling removes from the design: the comparison of the configured
certificate with the served one, the pending-restart record, its warning log
line, `doctor-managed-certificate-restart-pending`, the risk and the tests
that covered them, `test/managed/fleet-reload-certificate-pending.ci`, and
every departure from the design pages that existed only because of C-7 (all
five; the departure "Documents departed from" records today is another one,
from the last revision of this section). AC-38 now states that a
changed certificate is served from the next handshake after the commit, with
existing sessions untouched. The ruling is recorded in the `## Design
decisions` section of `docs/architecture/fleet-config.md`, the page the
`// Design:` header of `internal/component/plugin/server/managed_serve.go`
names, which documents the managed listener and its certificate ("Hub
certificate and client trust").

This edit also folds in the three non-blocking notes of the last design
judge: the core-design decision row now says that the managed certificate
name is outside the one verdict of `ExtractHubConfig`; the start and reload
asymmetry of an unresolvable name is stated (Known Limitations, AC-41, and
the planned `docs/architecture/pki/tls-listeners.md` edit); and the system
door's `statestore` row says why the borrowed store handle is used. The
changed text goes to the independent design judge.

### Design revision of 2026-11-23: doctor reports a hub session that cannot open

The independent design judge refused the C-10 text on 2026-11-23 (digest
`39cd9fbc7739`). R-20, the first Known Limitation and the cost cell of C-10
(c) said that a client which cannot verify the certificate a reload installs
is reported by its own `ze doctor` under `doctor-hub-unreachable`. Read at the
producers, that was false on both sides:

| Side | What the producer does | Producer |
|------|------------------------|----------|
| The client's doctor | `doctor-hub-unreachable` is a TCP dial of the first hub block and nothing more: "A bounded dial only: TLS and auth are the running daemon's job". A hub whose port answers is reported reachable whatever its certificate | `internal/component/managed/doctor.go` `checkHubReachable`, `diagnoseHubReachability`, `hubReachable` |
| The client's daemon | A failed handshake returns `tls handshake: ...` to the reconnect loop, which logs `connection lost, reconnecting` at Warn with no code and retries. `doctor-hub-unreachable` is logged only for a failed ack or fetch on a session that opened | `internal/component/managed/client.go` `runConnection`, `RunManagedClient`, `handleConfigChangedRequest` |
| The hub | A connection that ends before it names a client is logged at Warn and recorded nowhere: "there is no name to report it under" | `internal/component/plugin/server/managed_doctor.go` `authRefused` |

C-10 makes this case reachable at every rotation, and it was left to the
logs. The owner rule decides it: "`ze doctor` reports every problem an
operator can hit ... A problem class found in code, a lab or a test that
doctor does not report is a gap owed a doctor check, never left to the logs"
(`ai/META-DECISIONS.md`, "CLI, config, and daemon surfaces"). The check is
owed work and is designed here, on both sides (ACs 43 to 48):

| Side | What doctor reports after this revision |
|------|------------------------------------------|
| The managed client | A runtime check reads the last failed attempt of the daemon's hub client to open its session. The device refusing the hub's certificate is reported under `doctor-hub-certificate-unverified`, with the cause told apart (an issuer the device's anchor does not cover, a certificate that does not name the host the device dials, a certificate outside its validity dates, an anchor the configuration names and `pki` does not define, any other verification failure). A handshake that fails for another reason is `doctor-hub-handshake-failed`, and a hub that refuses the device's name or secret is `doctor-hub-auth-rejected`. Each row gives the cause and the command that fixes it, chosen from the way the device anchors its hub |
| The hub | The managed doctor record keeps, per remote address, the connections that ended before they named a client. An address that ended the handshake with a TLS alert, which is what a client that refuses the hub's certificate sends, is reported under the new `doctor-managed-certificate-refused`; any other such failure under the existing `doctor-managed-client-rejected`, the code its log line already names |

The same site held two neighbours of the same class, found while reading it,
and they are carried here because they are small and one record serves all
three: an authentication the hub refuses, and a `ca` the client's
configuration names and `pki` does not define (`errCANotConfigured`), both
logged by the client as `connection lost, reconnecting` and reported by its
doctor under nothing (`ai/META-DECISIONS.md`, "How agents work": a small
problem found while working is fixed in the work in hand).

This revision changes `## Acceptance Criteria` and the design, and carries
choices Thomas did not make (the codes, the two records and their bounds). It
reopens none of his decisions: C-10 stands as he ruled it. The changed text
goes to the independent design judge and is then presented to him. This spec
is not re-approved by this edit.

### Design revision of 2026-11-23: one declaration of the managed certificate rule

The independent design judge refused the design on 2026-11-23 (digest
`3d23e24337d0`), on four counts. Each is answered here:

| Refusal | What was wrong | What this revision does |
|---------|----------------|-------------------------|
| Nine rows of the Design documents table were missing | Decisions recorded on 2026-11-23 on two pages this spec's files link had no row | One row each, after reading each decision: one of `docs/architecture/api/commands.md`, eight of `docs/architecture/core-design.md` |
| The text called the web and looking glass fix uncommitted | It landed as commit 2984d3eaa5: on a removed name both listeners install their self-signed pair through `listenerTLSMaterial`, and a looking glass with no store refuses that reload | Every sentence that called it in progress now states what HEAD does |
| The row on "rotation does not rebind" quoted a cut-down sentence | The page's sentence goes on to say that a hub listener resolves through `listenerTLSMaterial`, to the named chain or the self-signed pair. The managed listener presents neither when no name is set: it presents the leaf the daemon's authority issues | The rows quote the whole sentences, and the design names which page sentences are scoped to the web and looking glass listeners and what the managed listener's rule is ("Documents departed from") |
| The managed listener's certificate was decided in two places | At start in `managedCertificate`, and at a reload in the gate of `cmd/ze/hub` and again in the apply. The page says a hub listener's precedence is declared once and called by its start and its reload, so the two cannot disagree | One function, `managedCertificate`, decides what the managed listener presents for a configuration. The server calls it at construction, the reload calls it once at the certificate gate, and the apply installs what that call returned and decides nothing |

What the design keeps from the page is its principle: each rule is declared
once, and a start and a reload call the same declaration. What it cannot keep
is the page's wording that the hub has one such declaration,
`listenerTLSMaterial`, whose second branch is the self-signed pair. The
managed listener's second branch is the issued leaf, a certificate the server
renews itself, which `docs/architecture/fleet-config.md` already states and
`managedCertificate` already implements at start. The page is therefore
corrected to name two rules, each declared once. That correction is a
departure from the page as written. It is recorded as three `departs` rows
and under "Documents departed from" with the text before and after, and it is
put to Thomas as C-11.

The revision removes one mechanism the earlier text added, the replacement of
a leaf holder's host list. Calling the one function at a reload builds the
issued leaf for the new hosts the way a start does, so
`internal/component/plugin/leaf.go` is not changed.

This revision changes `## Task` and the design, and carries a choice Thomas
did not make (C-11). It reopens none of his decisions. The changed text goes
to the independent design judge and is then presented to him. This spec is
not re-approved by this edit.

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
  → Constraint: the same trap covers the certificate the managed listener presents, built once by `managedCertificate`: once a reload binds addresses, the leaf's hosts and the certificate the config names must follow it (Design revision of 2026-11-23; C-10).
  → Constraint: the door's "One reload path" boundary says nothing but the external plugins needs a restart. By C-10 the managed certificate follows the reload, so the boundary holds unchanged and the door owes no correction.
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
  → Constraint ("Decision: a named certificate fails closed"): the precedence between a name and the certificate a listener presents with no name is declared once, and the listener's start and its reload both call that declaration, "so neither two surfaces nor a start and a reload can disagree". For the web and looking glass listeners it is `listenerTLSMaterial`. The managed listener's precedence must be one declaration too, called by its start and by its reload.
  → Constraint: the page speaks of "a hub listener" and "both hub listeners" and means the web and looking glass listeners; its intro lists three consumers and the managed listener is none of them. Adding the managed listener to the page makes every such sentence owe a scope, because with no name the managed listener presents the leaf the daemon's authority issues, never a self-signed pair.
- `docs/architecture/fleet-interface.md` - moved with the fleet interface work to `plan/post-release/features/spec-fleet-ui-2-estate-tree-and-device-tables.md`, which carries its two checkpoints (owner ruling of 2026-11-23)
- [ ] `docs/architecture/api/commands.md` - where a command is served, builtin RPCs, answer shapes, pipe filters
  → Constraint: the wire-method prefix of a builtin is owned by the package that calls `RegisterRPCs` (`RPCRegistration.registrar`, `OwnerPrefix`), so the `ze-managed:` commands are registered from `internal/component/managed`, which reads the registry through an exported snapshot of `plugin/server`, as `show fleet divergence` reads the held records.
- [ ] `docs/architecture/zefs-format.md` - key families and the store's refusal of a key below a value
  → Constraint: a new key family must not sit below an existing value key; `meta/fleet/` holds no value today (searched `pkg/zefs/keys.go`).
- [ ] `docs/architecture/doctor-and-health-checks.md` - doctor checks, codes, phases
  → Constraint: every problem class gets a doctor check with its cause and the command that fixes it (`ai/META-DECISIONS.md`); the managed runtime checks live in `internal/component/plugin/doctor/check_managed_runtime.go` and read `server.ManagedDoctorSnapshot`.
  → Constraint ("The runtime phase: checks only the daemon runs"): a fact that exists only in the running daemon is read by a check registered in the `runtime` phase through `diagnostic.RegisterDoctorCheck`; only `show doctor` runs it, and the local `ze doctor` asks the running daemon first ("The local route asks the running daemon first"), so a runtime row reaches `ze doctor` whenever the daemon answers.
  → Constraint ("Recent events: the report bus in show doctor"): a transient problem is raised on the report bus, and a state a check can probe is read by a check. A hub session that cannot open is a state, held until the session opens, so it is a runtime check on both sides (Existing Mechanisms, "Report bus").
  → Constraint ("Every problem site names a doctor code"): a `Warn` line carries a `code` attribute. The client's reconnect line carries none today.

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
- [ ] `cmd/ze/hub/main_reload.go` `runReloadContext` (the certificate gates) - after `zepki.Load` installs the new store, the reload resolves `environment.web.certificate` (`reloadWebCertificate`) and the looking glass name (`lgCertificateName`) with `zepki.ServerTLSMaterial` and refuses the reload through `restorePKIAfter` when one does not resolve; before the candidate is promoted `lm.updateWebCertificate` and `lm.updateLGCertificate` install the certificate the reloaded config describes, and a reload that fails after an install puts the running config's certificate back (`runningListenerCertificates`, `restoreOn`). No step reads the managed certificate name.
- [ ] `cmd/ze/hub/service_tls.go` `listenerTLSMaterial` and `cmd/ze/hub/listener_migrate.go` `installListenerCertificate`, `tlsUpdatable`, `selfSignedSource` (as of commit 2984d3eaa5) - `listenerTLSMaterial` is the one place the web and looking glass listeners' precedence is decided: a set name is `zepki.ServerTLSMaterial` of that name and fails closed, an empty name is the self-signed pair loaded from or generated into the listener's store, and an empty name with no store is refused (`errSelfSignedNeedsStore`). It answers PEM material. `service_web.go` and `service_lg.go` call it at start. Both reload hooks call it through `installListenerCertificate`, which reads the listener's own store and SAN address through `selfSignedSource` and installs the result with `UpdateTLSCertificate`; so a removed name installs the self-signed pair, and a looking glass with no store refuses that reload. `cmd/ze/hub` imports the pki component, and the pki component imports `plugin/server` (the comment in `startManagedServer`, confirmed by a search of its imports), so `plugin/server` can call neither `listenerTLSMaterial` nor the pki component: its resolver is injected (`ManagedServerConfig.TLSMaterialResolver`).
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
- [ ] `internal/component/managed/doctor.go` `checkHubReachable`, `diagnoseHubReachability`, `hubReachable` - the `hub-unreachable` check is post-config and stateless: it dials TCP to the first hub `client` block and reports `doctor-hub-unreachable` when the dial fails. It runs no TLS handshake and reads no state of the running client, so a hub whose port answers is reachable to it whatever certificate it presents.
- [ ] `internal/component/managed/client.go` `RunManagedClient`, `runConnection` - one attempt builds the TLS config (`clientTLSConfig`, which fails with `errCANotConfigured` when the named `ca` is not in `pki`), dials, runs the handshake (`tls handshake: %w`), sends the token and reads the answer (`errAuthRejected` on anything but `ok`). Every such failure returns to the loop, which logs `connection lost, reconnecting` at Warn with the delay and the error, with no `code`, and retries after the backoff. `cfg.reportConnection(true)` runs once authentication succeeds and `false` when that connection ends. `doctor-hub-unreachable` is logged only by `handleConfigChangedRequest` (a failed fetch after a push) and for a failed ack, both on a session that opened.
- [ ] `internal/component/managed/cmd_fleet.go` `ClientState`, `FleetTarget`, `InstallFleetTarget` and `cmd/ze/hub/managed_owner.go` `managedClientRun`, `startWith`, `state` - the daemon's owner of the hub client wires `ClientConfig.OnConnection`, `OnHold` and `OnHubAnswer` to atomics of the run, answers them as a `ClientState` value, and publishes that through the `FleetTarget` the hub installs (`cmd/ze/hub/main.go`). `show fleet status` reads it. Nothing in it says why the client is not connected.
- [ ] `internal/component/managed/register.go` and `register_doctor_codes.go` - the managed component registers its four doctor checks with `diagnostic.RegisterDoctorCheck`, all post-config, and its four codes with `diagnostic.Register`.
- [ ] `internal/component/plugin/server/managed_doctor.go` `authRefused`, `recordUnknown`, `managedUnknownMax` - a refusal that named a client is recorded under the name (a configured name) or in the bounded unknown record (the 16 most recent names, the oldest forgotten first, because whoever connects chooses the name). A connection that failed before it named a client is logged at Warn under `doctor-managed-client-rejected` with its remote address and is recorded nowhere.
- [ ] `internal/component/plugin/server/managed_serve.go` `handleConn` and `internal/component/plugin/ipc/tls.go` `StartListeners`, `AuthenticateWithLookup` - the listeners come from `tls.Listen`, so the handshake runs on the connection's first read, inside `AuthenticateWithLookup`, whose read error is returned wrapped (`read auth request: %w`) to `handleConn`, which hands it to `authRefused` with `conn.RemoteAddr()`.
- [ ] `internal/component/plugin/doctor/register_external.go` - the two managed runtime checks (`managed-listener-unbound`, `managed-client-rejected`) are registered here in `DoctorPhaseRuntime` with the dependency `managed-server`.
- [ ] `internal/component/plugin/ipc/connect_back_refusal.go` `reportConnectBackRefused` - the plugin acceptor reports a refused connect-back as an event on the report bus (`report.RaiseErrorFix`), its subject the claimed name or, when the request named none, the remote address.
- [ ] `internal/core/report/report.go` `RaiseWarningFix`, `ClearWarning`, `RaiseErrorFix`, `RaiseErrorFixOnce`, `defaultWarningCap`, `defaultErrorCap` - an active warning is one entry per source, code and subject until its producer clears it, 1,024 of them by default; an error is an event in a ring of 256 by default, shared by every producer, where the oldest is evicted, and `RaiseErrorFixOnce` leaves one event per subject per window.

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
- The certificate the managed listener presents follows the reload. The issued leaf names the host of every address the hub serves after each reload. A hub with no managed listener bound takes the certificate its new config names when a reload binds the first one. A reload that names a certificate the store does not hold is refused, as it is for the web listener.
- A reload that changes the certificate a serving hub presents (a name set, changed or removed, or the same name whose `pki` material changed) installs it on the running listener: the next handshake presents it, the listener keeps its socket and no session is closed (C-10).
- A device record removed or overwritten through the running daemon's data commands is noticed by the registry (R-4).
- A managed client whose hub session cannot open is reported by its own doctor with the cause and the fix: the hub's certificate refused, a handshake that failed otherwise, or an authentication the hub refused. The reconnect log line names the same code. `doctor-hub-unreachable` keeps its meaning, a hub that does not answer a TCP dial.
- The hub records, per remote address, the connections that end before they name a client, and its doctor reports them: an address that refused the hub's certificate under `doctor-managed-certificate-refused`, the others under `doctor-managed-client-rejected`.
- Not changed here: the fleet interface keeps its single source slot and its sample build. Its split into parts, the estate part over the registry and the not-available state belong to `plan/post-release/features/spec-fleet-ui-2-estate-tree-and-device-tables.md` (owner ruling of 2026-11-23).

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Connected-name set | `internal/component/plugin/server/managed.go` `ManagedConfigService.connected`, `RegisterClient`, `UnregisterClient` | Refuses a second session under one name; forgotten at disconnect | Replaces: the registry's live record holds connected state, and the duplicate refusal reads it. Two holders of "connected" would disagree |
| Managed server records | `managed_doctor.go` `managedDoctorState`; `managed_held.go` `heldRecords` | Per-client facts the server keeps, bounded by declared names | Extends: the registry is a third record of the same server, bounded the same way |
| Published snapshot | `managed_doctor.go` `managedDoctorSource`, `ManagedDoctorSnapshot`, `publishManagedDoctor`, `withdrawManagedDoctor` | Lets doctor read the running server's record with no import of the hub | Extends: the registry publishes through the same pointer and an exported `ManagedRegistrySnapshot`; the doctor record gains the registry's write and read failures, the handshake saturation and the connections that ended before they named a client |
| Push coalescing | `managed_serve.go` `pending`, `pushing`, `notifyWake`, `takePending`, `notifyWorker` | A burst of changes for one client becomes one push; bounded by connected names | Extends the pattern: one long-lived record writer with a dirty set, so a burst of events for one device becomes one store write |
| Borrowed store | `ManagedServerConfig.Store`; `managed_held.go` `HeldRecordKey`, `ReadHeldRecord` | The held records are written and read through the hub's store | Extends: the device records use the same borrowed store, no second handle |
| Runtime-state handle | `internal/core/statestore/statestore.go` `SetStore`, `Write`, `Read`, `Delete`, `List` | A process-wide accessor to the daemon's one `storage.Storage`, for callers that are handed no store | Does not apply as the route, and the store is the same one. The managed server is handed that `storage.Storage` by the hub (`ManagedServerConfig.Store`) and already reads client configs and writes held records through it. Three reasons keep the device records on that handle: the held records sit under `meta/config/`, a configuration key family, so the server needs the store handle whatever the device records use, and two routes to one store would have two lifetimes; `Stop` joins every user of the handle before the hub returns the store, an order the process-wide accessor's `SetStore(nil)` is outside; and a test gives each server its own store (the counting store of AC-20), which a process-wide accessor cannot. The key is registered in `pkg/zefs/keys.go` and no file is written directly, which is what the door's rule protects |
| Managed listener certificate | `managed_serve.go` `managedCertificate`, `withAuthority`; `ManagedServer.getCertificate` | Decides once, at construction, what every handshake presents: the named pki certificate, resolved once, or a leaf the daemon root issues | Extends, and stays the one declaration: `managedCertificate` remains the only place that decides what the managed listener presents for a configuration (a name: the named chain, failing closed; no name: the leaf the daemon's authority issues for the hosts of the serving addresses). It gains a second caller. The server calls it at construction, as today, and the reload calls it once, at the certificate gate, through an exported operation of the managed server that prepares the certificate of a hub config and changes nothing the listener presents. The apply installs what that call returned behind the one `getCertificate` every listener already calls per handshake, on a serving hub as on one with no listener bound, while the listeners keep their sockets. The gate and the apply hold no branch on the name. The fail-closed rule for a name is kept: a name is never answered by the issued leaf, and a config with no name is never answered by a name it dropped |
| Listener certificate precedence, declared once | `cmd/ze/hub/service_tls.go` `listenerTLSMaterial`; `cmd/ze/hub/listener_migrate.go` `installListenerCertificate`, `tlsUpdatable` (`UpdateTLSCertificate`, `selfSignedSource`) (`docs/architecture/pki/tls-listeners.md`, "Decision: a named certificate fails closed") | One function decides what the web and looking glass listeners present for a config (the named chain, or their self-signed pair), called by both at start and by both reload hooks, so a start and a reload cannot disagree. The hook installs the answer on the running server with no rebind | Extends the shape to a third listener, and does not call the function: the managed listener gets the same three parts, one declaration (`managedCertificate`), called at start and at the reload, and an install on the running listener with no rebind. It cannot share `listenerTLSMaterial` itself, for three reasons read in the code. The rule differs in its second branch: with no name the managed listener presents the issued leaf, never a self-signed pair. The answer differs in kind: `listenerTLSMaterial` answers PEM material installed once, and the issued leaf is a holder that reissues as it ages (`ServingLeaf`), which a PEM answer would freeze until it expired. And the managed listener's start runs in `plugin/server`, which can import neither `cmd/ze/hub` nor the pki component. The page is corrected to name both declarations ("Documents departed from") |
| Self-renewing leaf | `internal/component/plugin/leaf.go` `ServingLeaf` (`NewServingLeaf`, `Certificate`, `issueLocked`) | Issues the listener's leaf for a fixed host list and reissues it for the same list as it ages | Does not apply as a change: a holder keeps one host list for its life. When a reload leaves the configuration with no name, `managedCertificate` builds a holder for the new host list, as it does at start (`NewServingLeaf` issues at construction and fails when the authority cannot issue), and the apply puts it in place of the previous one. When the listener already presents the issued leaf and that leaf names every host of the new list, the function answers the holder in place and issues nothing. The plugin acceptor's leaf (`NewHubAcceptor`) is unchanged |
| Leaf host derivation | `managed_serve.go` `managedLeafHosts` | Turns listen addresses into the leaf's hosts: `127.0.0.1` plus each named host, none for an unspecified address | Extends by use: it keeps its one call site, inside `managedCertificate`, which the reload now reaches with every serving address of the new config, before an added one is bound. The function is unchanged |
| Client hostname check | `internal/component/managed/tls.go` `clientTLSConfig`; `client.go` `serverNameFromAddr` | The client verifies the leaf against the host of the address it dials | Does not apply as a change: the client is right, and the hub is what must present a leaf naming that host |
| Reload certificate gate | `cmd/ze/hub/main_reload.go` `runReloadContext`: `zepki.ServerTLSMaterial` on the web and looking glass names after `zepki.Load`, `restorePKIAfter` (`docs/architecture/pki/tls-listeners.md`, "Decision: a named certificate fails closed") | Refuses a reload whose web or looking glass certificate name does not resolve in the new store, before any consumer applies | Extends: the managed certificate of the new hub config is a third one the same gate checks and refuses the same way, before any consumer applies. The gate does not judge the name itself: it asks the managed server to prepare the certificate of the new hub config, which calls `managedCertificate`, and refuses on its error. That one call resolves and parses a name, or issues the leaf when the config names none, so the apply that installs the result after promotion has no failure of its own. The name and the serving addresses of a hub config are read by one reader in `cmd/ze/hub/managed_server.go`, which `startManagedServer` and the gate both call, as `lgCertificateName` serves the looking glass at start and at reload |
| Certificate rotation without rebinding | `cmd/ze/hub/listener_migrate.go` `updateWebCertificate`, `updateLGCertificate`; `UpdateTLSCertificate` (`docs/architecture/pki/tls-listeners.md`, "Decision: rotation does not rebind the hub listeners") | After a reload, installs the re-resolved chain on the running web and looking glass servers; the next handshake serves it | Extends (C-10): the managed listener is a third hub listener rotated the same way. At each reload the apply installs the certificate the gate's call of `managedCertificate` prepared (the named chain, or the leaf ze issues) behind the handshake's certificate source; the listener keeps its socket, no session is closed, and the next handshake presents it. The same install carries the issued leaf's new hosts and a first listener's certificate. On a removed name the three listeners now behave alike (commit 2984d3eaa5): each presents the certificate a restart with that config would serve. `updateWebCertificate` and `updateLGCertificate` install the self-signed pair through `listenerTLSMaterial`, and the managed listener presents the leaf ze issues, its certificate with no name (`docs/architecture/fleet-config.md`, "Hub certificate and client trust"). Two differences remain and both are stated on the page by this spec. The certificate with no name differs, as above. And the web and looking glass hooks install before the candidate is promoted, with an undo, where the managed install runs after promotion beside the client set and cannot fail, because the gate's call already produced what it installs |
| Restart-required refusal | `internal/component/plugin/server/reload_declarations.go` `RefuseChangedDeclarations`, `raiseReloadRefused`, `doctor-config-reload-refused` | Refuses whole a reload that changes the external plugins, with the fix `restart ze` | Does not apply: by C-10 a changed managed certificate needs no restart, so nothing is refused for it. A refused commit is never promoted, so the refusal (option (b) of C-10) would also have left the change unmakeable from a command surface |
| Managed doctor record | `managed_doctor.go` `managedDoctorState`, `bindFailed`; `check_managed_runtime.go` | Unbound addresses and client rejections, read by the runtime checks | Extends: an address whose leaf could not be issued is recorded unbound with that reason |
| Hub reachability probe | `internal/component/managed/doctor.go` `hubDoctorCheck`, `checkHubReachable`, `hubReachable`; code `doctor-hub-unreachable` | A post-config TCP dial of the first hub block, run with the daemon up or down; no TLS, no state of the running client | Does not apply as the carrier, and is kept unchanged: it answers whether the hub's port answers, from any process, and a handshake run from the doctor process would send no token and could not see what the daemon's client met. The new check is its runtime sibling in the same package, reading what the client itself recorded. The code's description is corrected to say what it probes |
| Hub client state seam | `internal/component/managed/client.go` `ClientConfig.OnConnection`, `OnHold`, `OnHubAnswer`, `reportConnection`; `cmd/ze/hub/managed_owner.go` `managedClientRun`, `state`; `cmd_fleet.go` `ClientState`, `FleetTarget`, `InstallFleetTarget` | The running client reports connected, holding and the hub's last answer to its owner, which publishes them as one value the fleet commands read | Extends: a fourth report beside the three, the last attempt that failed to open the session (when, at which step, the cause, the way the hub was anchored), kept by the same owner and answered in the same `ClientState` value. The attempt that authenticates clears it. `show fleet status` keeps its answer: its columns are unchanged |
| Runtime doctor check registration | `internal/core/diagnostic` `RegisterDoctorCheck`, `DoctorPhaseRuntime`; precedent `internal/component/plugin/doctor/register_external.go` (`managed-listener-unbound`, `managed-client-rejected`) | A check over state only the daemon holds; `show doctor` runs it, and the local `ze doctor` asks the daemon first | Extends by one check on each side: `hub-session` in `internal/component/managed` (the first runtime check of that package, registered beside its four post-config ones in `register.go`), and `managed-certificate-refused` beside the two managed runtime checks in `register_external.go` |
| Managed client doctor codes | `internal/component/managed/register_doctor_codes.go` `doctorDiagnosticCodes`, `diagnostic.Register` | The four codes the managed component owns, with the text `ze show diagnostic <code>` prints | Extends: three codes, `doctor-hub-certificate-unverified`, `doctor-hub-handshake-failed`, `doctor-hub-auth-rejected`; the description of `doctor-hub-unreachable` is corrected (a TCP probe of the first hub block, not "none of its management hubs") and names the three as related |
| Unknown-name record | `managed_doctor.go` `recordUnknown`, `managedUnknownMax` | Keeps the 16 most recent names no client entry declares, the oldest forgotten first, because the peer chooses the name | Extends the pattern to the other value a peer chooses, its address: the connections that end before a name are kept per remote address (the port left out, since it changes at every attempt), 16 addresses per class, the oldest forgotten first |
| Pre-identification refusal | `managed_doctor.go` `authRefused`, the branch for an error that is no `AuthRefusal` | Logs at Warn under `doctor-managed-client-rejected` with the remote address; records nothing | Extends: the branch records what it logs. The log line of a connection that ended on a TLS alert names the new code |
| Report bus | `internal/core/report` `RaiseWarningFix`, `ClearWarning`, `RaiseErrorFixOnce`; the `recent-events` check (`docs/architecture/doctor-and-health-checks.md`, "Recent events: the report bus in show doctor"); precedent `internal/component/plugin/ipc/connect_back_refusal.go` `reportConnectBackRefused` | Carries a transient problem to `show doctor`, deduplicated on source, code and subject; the plugin acceptor raises a refused connect-back there under the claimed name or the remote address | Does not apply as the carrier, on either side. On the hub, the subject would be the remote address, and one rotation the fleet cannot verify gives up to 2,000 subjects: more than the 1,024 active warnings and the 256 error events the bus holds for every producer, so the fleet's rows would evict the other owners' events, and the bus cannot clear an error event when that address later authenticates. The managed record carries its own bound and clears. The connect-back precedent differs in scale: its subjects are the plugins the daemon itself started. On the client there is one subject, and the bus could hold it, but its row is the `recent-events` wording and severity for an event that "already happened", where this is a standing state with a cause-specific fix; the client already holds the state in the seam above, so a check reads it and no second copy is kept on the bus |
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
| Reload | `cmd/ze/hub/main_reload.go` `doReloadContext`, `publishNameServers` | The one reload path; publishes post-promotion state | Extends: the managed client set is applied after promotion, beside `publishNameServers`; its extraction is checked before any step applies. The same apply installs the certificate the gate's call of `managedCertificate` prepared for the new config, the issued leaf's new hosts included |
| Held record | `managed_held.go` `HeldClients`, `HeldRecordKey` (`fleet-7`) | The hub's copy of what a holding client runs | Does not apply as a writer: the registry reads presence only, as the owner decided |
| Fleet interface seam | `internal/component/web/fleetui/source.go` `sourceSlot`, `MustRegisterSource`; `register.go` `sourceRegistered` | One source per build, all concerns | Does not apply here: the split into parts is web work, moved to `plan/post-release/features/spec-fleet-ui-2-estate-tree-and-device-tables.md` by the owner ruling of 2026-11-23 |
| Device display state | `internal/component/web/fleetui/state.go` `DeviceDisplayState` | Derives Unknown while `HealthReported` is false | Does not apply here: moved with the seam to `fleet-ui-2` |
| Route deps | `internal/component/web/webroute.go` `RouteDeps.Dispatch` | Gives a route builder the web dispatcher | Does not apply here: the estate part built over it moved to `fleet-ui-2` |
| Command surface parity | `cmd/ze/hub/surface_parity_test.go` `TestEveryCommandOnEverySurface`; `command.AnswerSurface` | Checks every registered command is listed and served on every operator surface | Extends: the two new builtins are covered by it with no edit, and AC-34 names it |
| Perf harness | `cmd/ze/hub/fleet_perf_test.go` `TestFleetManyClientsPerf` | 128 clients, one fetch each | Extends: 2,000 clients held connected at once |
| Typed events | `internal/core/events/typed.go` `Register` | Typed bus events | Does not apply: placement B (a component fed by events) was rejected by the owner |

## Design documents

Derived by `./le spec design documents spec plan/release/features/spec-fleet-1-device-registry.md`, which the last revision of 2026-11-23 re-ran: 41 pages and 15 decision rows (one of `docs/architecture/fleet-config.md`, three of `docs/architecture/api/commands.md`, eleven of `docs/architecture/core-design.md`). That revision added the nine decision rows recorded on 2026-11-23 that had none (one of `commands.md`, eight of `core-design.md`), replaced the two rows of `docs/architecture/pki/tls-listeners.md` whose statements were cut short with rows that quote the whole sentences, and turned three of that page's rows to `departs`. The paragraph below is the history of the earlier revisions, and its counts are theirs. The table carries rows added beyond the derivation: two doors this spec's Required Reading names (`ai/doors/api-surfaces.md`, `ai/doors/commands-cli.md`) and two owner rules of `ai/META-DECISIONS.md`. The design revision of 2026-11-23 added the rows on the managed listener's certificate: two of `ai/doors/hub-engine.md`, two of `docs/architecture/fleet-config.md` ("Hub certificate and client trust"), two of `docs/architecture/pki/tls-listeners.md`, and one each of `docs/architecture/pki/pki-store.md` and `docs/architecture/api/process-protocol.md` for `internal/component/plugin/leaf.go`, which had joined Files to Modify and left it again with the last revision (both rows stay, because the pages still describe the leaf this design builds). The C-10 ruling of 2026-11-23 added a sixth decision row, on `docs/architecture/fleet-config.md`, and turned the five `departs` rows to `agrees`. The revision of 2026-11-23 on doctor coverage re-ran the derivation after it added files (still 41 pages and 6 decision rows, the added files being covered by pages already derived) and added seven rows: one of `ai/META-DECISIONS.md` (the doctor coverage rule), five of `docs/architecture/doctor-and-health-checks.md` (the local route, the report bus, the problem sites, the managed fleet checks, where an owner registers) and one of `docs/architecture/fleet-config.md` ("Hub certificate and client trust", the client's refusal).

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | One schema, one command registry, every interface | System door (always derived) | "A command is declared once and reaches every interface." | agrees | `show fleet devices` and `show fleet device` are builtin RPCs with `ze:command` nodes in `ze-managed-cmd.yang`, registered once from `internal/component/managed/register.go`; no interface package is edited, and `TestEveryCommandOnEverySurface` checks every surface lists and routes them (AC-34) |
| `ai/doors/system.md` | Shared machinery you use rather than rebuild | System door (always derived) | "In the daemon, `statestore` under a key in `pkg/zefs/keys.go`, never `os.WriteFile`." | agrees | The device record is written through the daemon's one store under `KeyFleetDeviceRecord`, registered with `MustRegister` in `pkg/zefs/keys.go`; no file is written directly. It reaches that store through the `storage.Storage` handle the hub gives the managed server (`ManagedServerConfig.Store`), not through the `statestore` accessor. Both name the same one store, so the door's rule (one store, a registered key, no file written) holds either way; the borrowed handle is used for three reasons. First, the server already needs that handle: it reads client configs and writes the held records under `meta/config/`, a configuration family the `statestore` route is not for, and two routes to one store inside one server would carry two lifetimes. Second, `Stop` joins the record writer before the hub returns the store, an order the process-wide `SetStore(nil)` is outside. Third, a test gives each server its own store (the counting store of AC-20), which a process-wide accessor cannot (Existing Mechanisms, "Runtime-state handle") |
| `ai/doors/system.md` | Shared machinery you use rather than rebuild | System door (always derived) | "A long-lived worker on a channel, never one per event" | agrees | One record writer started by `Start` and ended by `Stop` drains a dirty set; no goroutine is started per connect, fetch or ack (Critical Review Checklist, goroutine lifecycle) |
| `ai/doors/hub-engine.md` | Boundaries | Door covering `cmd/ze/hub`, `internal/component/managed`, `pkg/fleet` | "SIGHUP and a managed push both reach `doReload` (`cmd/ze/hub/main_reload.go`). A new trigger joins it rather than building a second path." | agrees | The managed client set joins `doReloadContext`: the hub extraction is checked before any step applies and applied after promotion beside `publishNameServers`; no second reload path is built |
| `ai/doors/hub-engine.md` | Traps | Door covering `cmd/ze/hub` | "Something built only at startup ignores every reload." | agrees | This is the reload defect the spec fixes: the managed server, built only at startup today, gains an apply that the reload calls (AC-8, AC-9, AC-26). The apply also carries what the certificate needs so that a bound address is verifiable: the issued leaf's hosts are re-derived (AC-35, AC-36), a hub with no managed listener bound takes the certificate the new config names (AC-37), and a serving hub installs a changed certificate (AC-38) |
| `ai/doors/hub-engine.md` | Traps | Door covering `cmd/ze/hub` | "so a changed RADIUS or TACACS+ server was accepted and applied to nothing" | agrees | Nothing of the managed listener stays built only at startup. A reload that changes the certificate a serving hub presents (a name set, changed or removed, or the same name whose stored material changed) is applied at that reload: the apply installs it behind the handshake's certificate source, and the next handshake presents it (C-10, AC-38). A changed certificate is never accepted and applied to nothing |
| `ai/doors/hub-engine.md` | Boundaries | Door covering `cmd/ze/hub` | "they change only when `ze` restarts, and nothing else in the config needs one" | agrees | The managed client set and the managed certificate both follow the reload (AC-8, AC-9, AC-26, AC-35 to AC-38), so the external plugins stay the only part of the config that needs a restart, and the door's sentence stays true as written |
| `ai/doors/hub-engine.md` | Traps | Door covering `cmd/ze/hub` | "A check that runs only at start passes validate." | agrees | The label agreement and the 4,096-name refusals sit in `ExtractHubConfig` beside `checkManagedCertificateAgreement`; `validate_semantic.go` calls `ExtractHubConfig`, so `ze config validate`, boot and the reload pre-check give one verdict (AC-23, AC-24). The managed certificate name is outside that verdict, and the other way round from the trap: it is checked only at the reload's certificate gate (AC-39), while boot accepts a name that does not resolve and reports each address unbound (AC-41), and validate does not resolve it. The asymmetry is stated in Known Limitations |
| `ai/doors/plugin-infra.md` | Traps | Door covering `internal/component/plugin/server`, `pkg/plugin/rpc` | "so a payload must be compact `json.Marshal` output." | agrees | The managed channel keeps newline framing; only the hub's reader of it gets a lower maximum, and the bounded ack error travels inside the same compact line |
| `ai/doors/plugin-infra.md` | Boundaries | Door covering `pkg/plugin/rpc` | "The SDK stays generic." | agrees | `pkg/plugin/rpc` gains one additive constructor taking a caller-given maximum; it names no managed or fleet concept, and every existing caller keeps `MaxMessageSize` |
| `ai/doors/core.md` | Boundaries | Door covering `pkg/zefs/keys.go`, `internal/core/diagnostic` | "Runtime state goes through the daemon's one store." | agrees | The device records are runtime state in the daemon's store under a registered key family; the four new hub doctor codes are declared and registered in `internal/component/plugin/server/doctor_codes.go`, where `doctor-managed-client-rejected` and `doctor-managed-listener-unbound` are, and the three client codes in `internal/component/managed/register_doctor_codes.go`: `codes_plugin_infra.go` holds only the codes several packages share, and its header sends a single-owner code to its owner's package |
| `ai/doors/config-yang.md` | Traps | Door covering `internal/component/config/loader_extract.go` | "`ze config validate` never calls a live `OnConfigVerify`." | agrees | No rule of this spec lives in an `OnConfigVerify`: the label and count refusals run in `ExtractHubConfig`, which validate reaches through `validate_semantic.go`, and the label constraints are YANG (`pattern`, `length`, `max-elements`) |
| `ai/doors/le-tooling.md` | Where it sits | Door covering the `test/managed/fleet-*.ci` files | "Functional `.ci` files live under `test/<suite>/` and run through" | agrees | The eleven new functional tests live under `test/managed/` and run through `./le test managed` |
| `ai/doors/commands-cli.md` | Boundaries | Added: Required Reading names this door | "A handler answers data, never text." | agrees | Both handlers answer rows and a document with kebab-case keys, declared by `RegisterShape` and `RegisterColumns`, and every format comes from the pipes |
| `ai/doors/api-surfaces.md` | Boundaries | Added: Required Reading names this door | "Never add a command in an adapter." | agrees | No REST, gRPC, MCP, SSH or web package gains a fleet route or command; each surface reaches the builtins through the dispatcher |
| `ai/META-DECISIONS.md` | Goal and scope | Added: owner rule of 2026-11-23 ("1 b", voice-transcribed) | "Fleet management ships in the first release with its device registry," | agrees | The registry ships reachable through its two commands on every command surface; its web pages moved to `fleet-ui-2` (Task, "Owner ruling of 2026-11-23") |
| `ai/META-DECISIONS.md` | CLI, config, and daemon surfaces | Added: owner rule on command surfaces | "Every operator command runs through the daemon on every command surface (MCP," | agrees | Both commands are daemon builtins served on MCP, REST, gRPC, SSH and the web with one answer; they hold no local route, so no local answer can differ (owner, final gate 2026-11-19, "Daemon-only commands") |
| `docs/architecture/fleet-config.md` | Dedicated managed listener (reuses auth + transport) | `// Design:` page of `managed.go`, `managed_serve.go`, `managed_doctor.go`, `client.go`, `envelope.go`, `managed_server.go`; modified page | "to each `server` block that declares `client` entries." | agrees | A listener still binds only for a block that declares clients; the reload apply binds a block's address when it gains its first client and closes one that loses its last (AC-26, R-8). The server object exists with no listener so the reload can bind, which the page's new "Device registry" text states |
| `docs/architecture/fleet-config.md` | Hub certificate and client trust | `// Design:` page of `managed_serve.go` (`managedCertificate`, `managedLeafHosts`); modified page | "A client anchored on the root needs no change for either." | agrees | At a reload `managedCertificate` is called with every serving address of the new config, derives the hosts through `managedLeafHosts` and issues a leaf for that list before an added address is bound, so a client anchored on the root verifies the hub on an address added by reload with no client change (AC-35, AC-36). When the authority cannot issue, the address is not bound and is reported unbound, and the previous leaf keeps serving the others (AC-40). The page gains that the leaf's hosts follow a reload, and that a reload which adds a served host mints a leaf as a restart does |
| `docs/architecture/fleet-config.md` | Hub certificate and client trust | `// Design:` page of `managed_serve.go`; modified page | "The name wins whenever it is set, and a name that resolves to nothing is an" | agrees | The name wins at the reload that sets it: a hub with no managed listener bound serves the name its new config carries when a reload binds the first one (AC-37), and on a serving hub a name set or changed by reload is presented from the next handshake (C-10, AC-38). A reload naming a certificate the new store does not hold is refused whole, never answered by a fallback (AC-39). The page gains what a reload does to the managed listener's certificate, as an addition: no statement of it is corrected |
| `docs/architecture/fleet-config.md` | Design decisions | Decision of 2026-11-23 | "A reload that changes the certificate a serving hub's managed listener presents installs it on the running listener at that reload: the next handshake presents the certificate the configuration names, the listener keeps its socket and no session is closed, as the web and looking glass listeners do. The certificate stays in the configuration's `pki` block. In Thomas's words (voice-transcribed): "Keep the certificate in the configuration." and "In that case on commit we can perform the certificate change on the listener."" | agrees | This is Thomas's C-10 ruling, recorded on the page by this edit. The apply installs the certificate the new config names behind the handshake's certificate source, with no rebind and no session closed, and the next handshake presents it (AC-38); the name stays in the configuration's `pki` block; no reload is refused and no restart is asked for a changed certificate (AC-39 refuses only a name that does not resolve, unusable material, or a leaf that cannot be issued). The decision's "as the web and looking glass listeners do" holds at HEAD in every case, a removed name included (commit 2984d3eaa5): each listener installs, on its running socket, the certificate a restart with that configuration would serve. What that certificate is with no name differs by listener and is not what the decision compares: the web and looking glass listeners return to their self-signed pair (`listenerTLSMaterial`), and the managed listener presents the leaf the daemon's authority issues ("Hub certificate and client trust" on the same page) |
| `docs/architecture/fleet-config.md` | Per-Client Secrets | `// Design:` page of `managed_serve.go`; modified page | "A client that must be reachable through two blocks is declared under both." | agrees | The registry keys by name, so one record serves both declarations, and two declarations of one name must carry the same labels (R-10, AC-23); admission per block is unchanged |
| `docs/architecture/fleet-config.md` | What doctor reads from the managed listener | `// Design:` page of `managed_doctor.go`, `check_managed_runtime.go`; modified page | "no managed server in the process the answer is `ErrManagedNotServing`" | agrees | The registry snapshot is published through the same pointer and withdrawn by `Stop`; with no server the commands answer an operator error naming its fix (`TestShowFleetDevicesNotServing`), and the doctor table gains the four new codes. One row of the page changes meaning and is edited with the code: a connection that ends before it names a client is no longer "logged only", it is recorded per remote address and reported (AC-47) |
| `docs/architecture/fleet-config.md` | Hub certificate and client trust | `// Design:` page of `internal/component/managed/tls.go`, `client.go`; modified page | "cannot authenticate ends the connection before the token is written." | agrees | The client's verification and its order are unchanged: the handshake still ends before the token is written. What changes is that the end is reported: the client's doctor names the certificate it refused, why, and the fix (AC-43 to AC-45), and the hub's doctor names the address that refused it (AC-47). The page gains both |
| `docs/architecture/fleet-config.md` | Config Storage (Hub Side) | `// Design:` page of `managed_server.go`; modified page | "The hub's `Storage.SetWriteObserver` callback maps a written client-config key" | agrees | The same single observer also marks the written client's hub version stale and still reads no storage; a record key ends in `record`, which `ClientNameFromConfigKey` never maps, so a record write pushes nothing (R-9) |
| `docs/architecture/fleet-config.md` | Connection Lifecycle | `// Design:` page of `client.go`, `managed_serve.go`; modified page | "largest answer, 16 MiB (`rpc.MaxMessageSize`), on a link of about 4.5 Mbit/s." | agrees | The client's reader keeps 16 MiB; only the hub's reader of client lines is lowered, derived from `fleet.HeldConfigMax`, because the hub may serve a configuration larger than any client sends (Changed-Wire Census) |
| `docs/architecture/fleet-config.md` | Connection Lifecycle | `// Design:` page of `managed_serve.go`; modified page | "A panic while the hub serves one client ends that client's connection only." | agrees | The registry marks the device offline in `handleConn`'s deferred cleanup, which runs on a contained panic too, so the name is released and the reconnect admitted |
| `docs/architecture/fleet-config.md` | RPCs | `// Design:` page of `envelope.go`; modified page | "after the first four verbs is optional and omitted when unset" | agrees | No verb and no field is added; `config-ack`'s `error` keeps its type and is cut by the client at `fleet.AckErrorMax` with a marker, a value bound the page's "Protocol" text gains |
| `docs/architecture/fleet-config.md` | Non-Goals | `// Design:` page of the managed server files; modified page | "Hub already exists; extend it" | agrees | The registry is a record inside `ManagedServer` (owner, placement A), not a new server component |
| `docs/architecture/api/commands.md` | Which surfaces offer a command | `// Design:` page of `register.go`; anchored to the managed and hub files; modified page | "Offered: the daemon serves each through its handler" | agrees | Both commands are builtin RPCs, the registration row this statement covers, so every surface offers them with no declaration of their own |
| `docs/architecture/api/commands.md` | Per-command declarations: what a command says about itself | `// Design:` page of `register.go`; modified page | "the pipe operations this command runs in its own handler" | agrees | `\| label` is declared with `RegisterPipeFilters` on `show fleet devices` and runs in its handler, which keeps the matching rows (Data Flow step 11, A-4); the command also registers its own shape and columns, so it inherits none from `show fleet` |
| `docs/architecture/api/commands.md` | Design decisions | Decision of 2026-08-21 on a page this spec modifies | "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included" | agrees | `show fleet devices` names the subject and stops; narrowing by label is the pipe operation `\| label <key>=<value>`, never an argument or a `--label` flag. `show fleet device name <name>` names one device as its subject, a document rather than the table, in the grammar of `show fleet divergence name <name>` |
| `docs/architecture/api/commands.md` | Design decisions | Decision of 2026-11-20 on a page this spec modifies | "A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed."" | agrees | `label` is no operator of the catalog (`pipe_catalog.go`), so it is the command's own operation and overrides nothing; every generic, `match`, `count`, `first` and the formats included, still applies to the rows unchanged |
| `docs/architecture/api/commands.md` | Design decisions | Decision of 2026-11-23 on a page this spec modifies | "A pipe operation works on the answer of the command before it. It may narrow the answer, extend it, transform it, or deliver it somewhere, such as saving it to a file. It never changes what the command does" | agrees | `\| label <key>=<value>` works on the answer of `show fleet devices` and narrows it to the rows whose labels include the pair. It changes nothing the command does: the handler reads the same registry snapshot with or without it, no device is touched, and the rows it keeps are rows the unfiltered answer holds (AC-10). The decision allows a pipe to do more than narrow; this one only narrows, and the spec restricts no other pipe: every generic operation still applies after it |
| `docs/architecture/api/ipc_protocol.md` | Message Framing | `// Design:` page of `framing.go`, `conn.go`; modified page | "All plugin RPC messages are UTF-8, newline-delimited lines:" | agrees | The framing is unchanged; the page gains the constructor with a caller-given maximum and that only the hub's managed reader passes one |
| `docs/architecture/api/process-protocol.md` | Wire Format | `// Design:` page of `types.go`; anchored to `framing.go`, `conn.go` | "Maximum message size is 16 MB." | agrees | Every plugin reader keeps 16 MiB; the framing paragraph states that the plugin protocol keeps it (Documentation Update Checklist row 8) |
| `docs/architecture/api/wire-format.md` | Framing | Anchored to `framing.go`, `conn.go` | "16 MB (16,777,216 bytes)" | agrees | The IPC maximum stays the library default; the managed channel's hub-side bound is stated in `docs/architecture/fleet-config.md`, "Protocol" (row 7) |
| `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` | Both ESP forms on one inbound state | Anchored to `conn.go` (`writeFrame`) | "" | silent | The section cites `writeFrame` for a serialized framed write of IKE report RPCs; this spec changes no write path, only an added read constructor |
| `docs/architecture/api/architecture.md` | Implementation Status | Anchored to `managed_serve.go`, `types.go`, `main.go`, `main_reload.go` | "" | silent | Lists the plugin process, dispatch and BGP API features; silent on the managed channel and the fleet commands |
| `docs/architecture/api/architecture.md` | Design decisions | Decision of 2026-11-23 | "When `\| implied` modifies an existing config line (such as inserting automatic members into a `filters [ ... ];` list), it keeps the line intact and places an inline `# implied: ...` comment on the same line after the statement naming what was added and why, rather than placing a comment only above the line or splitting the list across multiple lines. In Thomas's words (2026-11-23): "No, it should be on the same line and say what was added"." | agrees | The spec adds no implied item, so `\| implied` modifies no line of its configuration: the one config addition, the `label` list under the hub's `client` entry, is written by the operator, a client entry with no label gains none, and the registry's facts are runtime state answered by `show fleet devices` and `show fleet device`, not configuration. No list is expanded or annotated, and the display code is not touched |
| `docs/architecture/core-design.md` | 19. Component Boundaries | Anchored to `managed_serve.go`, `types.go`, `main.go`, `main_reload.go` | "Each component under `internal/component/` is independently removable." | agrees | No import between components is added: `managed` already imports `plugin/server` for `show fleet divergence`, the registry lives in `plugin/server`, and no web package is touched |
| `docs/architecture/core-design.md` | Design decisions | Decision of 2026-11-23 | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | The hub's managed configuration programs neither the kernel nor VPP; the labels and the reload apply add no feature the dataplane guard judges |
| `docs/architecture/core-design.md` | Design decisions | Decision of 2026-11-23 | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | The decision binds the dataplane mismatch, and the spec adds no dataplane verdict. Of its own config refusals, the label agreement and the 4,096-name bound follow the same shape: one verdict in `ExtractHubConfig` at validate, boot and the reload pre-check (AC-23, AC-24). The refusal of a managed certificate name that does not resolve does not: it is reload-only (AC-39), while boot accepts such a name and reports each address unbound (AC-41), and validate does not resolve it, so a hub booted with such a name refuses every later reload until the name is defined in `pki` or removed (Known Limitations). The decision does not reach that refusal, because a certificate name is no dataplane mismatch |
| `docs/architecture/core-design.md` | Design decisions | Decision of 2026-11-23 | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "nft"` annotation." | agrees | No firewall leaf, verifier or `ze:backend` annotation is touched |
| `docs/architecture/core-design.md` | Design decisions | Decision of 2026-11-23 | "Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it"." | agrees | The spec adds no data-plane selector, no backend registration and no `ze:backend` annotation, and none of its configs, tests or pages writes a selector value: the `label` list under the hub `client` entry and the two fleet commands name no plane. The hub and client configs of the new `.ci` tests carry no `backend` leaf, so they hold neither an old value nor a new one |
| `docs/architecture/core-design.md` | Design decisions | Decision of 2026-11-23 | "BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it."" | agrees | The spec configures no BGP session and no export chain: it adds no second surface for egress next-hop rewriting, and the reload path it extends (`doReloadContext`) gains the managed client set and its certificate only, with no BGP peer setting in either |
| `docs/architecture/core-design.md` | Design decisions | Decision of 2026-11-23 | "Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs")." | agrees | No `policy` block, `modify` rule or chain reference is added or read; the one config addition is the `label` list under `plugin hub server <block> client <name>` |
| `docs/architecture/core-design.md` | Design decisions | Decision of 2026-11-23 | "Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)")." | agrees | The spec's one change under `pkg/plugin/rpc` is an added frame reader with a caller-given maximum (`framing.go`, `conn.go`); `rpc.FilterInsertion` is not widened and no plugin RPC is added. Its change to config loading is in `ExtractHubConfig`, which builds no filter chain |
| `docs/architecture/core-design.md` | Design decisions | Decision of 2026-11-23 | "Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self")." | agrees | No UPDATE is built, forwarded or overridden by this spec: the managed channel carries configuration text between a hub and its devices, and no file of the BGP forwarding path is in Files to Modify |
| `docs/architecture/core-design.md` | Design decisions | Decision of 2026-11-23 | "Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load")." | agrees | The refusals this spec adds at config load are the label agreement and the 4,096-name bound of the hub's client entries; neither reads a policy, and no next-hop value is parsed or validated |
| `docs/architecture/core-design.md` | Design decisions | Decision of 2026-11-23 | "Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes")." | agrees | The spec originates and forwards no route. A hub that also routes keeps its BGP sessions through every reload this spec changes, which is why a changed managed certificate is installed with no restart (C-10) |
| `docs/architecture/core-design.md` | Design decisions | Decision of 2026-11-23 | "Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?"." | agrees | The spec inserts no filter-chain member and implies no configuration: a device's labels are written by the operator on its `client` entry, and the registry's facts are runtime state shown by `show fleet devices`, not configuration `\| implied` would display. Nothing is added to or hidden from the implied-filter seam |
| `docs/architecture/hub-architecture.md` | What the daemon does at start | `// Design:` page of `main.go`, `main_reload.go`; anchored to `managed_serve.go`, `managed_server.go`; modified page | "the external plugins and their modules alone: every other part of the config" | agrees | The managed client set now reloads without a restart, and so does everything about the managed listener's certificate: the hosts of the issued leaf, the certificate of a hub with no managed listener bound, and a changed certificate on a serving hub (`plugin hub server <block> certificate`, or the material it names), installed at the reload (C-10, AC-35 to AC-38). Before this work the managed client set was a silent exception to the statement; after it the statement holds for the managed server as written |
| `docs/architecture/hub-architecture.md` | Store ownership and configuration authority | `// Design:` page of `main.go`; modified page | "The daemon owns one writable tree at `<config-folder>/database/` until exit." | agrees | The registry writes through the store the daemon owns, borrowed by the managed server, and its writer is joined by the managed server's `Stop` before the hub returns that store |
| `docs/architecture/doctor-and-health-checks.md` | The runtime phase: checks only the daemon runs | Anchored to `managed_doctor.go`, `check_managed_runtime.go`; `// Design:` page of `internal/component/plugin/server/doctor_codes.go`, `register_external.go`, `internal/component/managed/register_doctor_codes.go`; modified page | "such state registers in the `runtime` phase, through the same" | agrees | `doctor-managed-device-record-unwritable`, `doctor-managed-device-record-unreadable` and `doctor-managed-handshake-saturated` read the running server's record and register in the runtime phase through `diagnostic.RegisterDoctorCheck`, as the existing managed checks do. So does `doctor-managed-certificate-refused`. On the client, the `hub-session` check reads the state the daemon's hub client reported to its owner, which exists in no other process, and registers in the same phase the same way from `internal/component/managed` |
| `docs/architecture/doctor-and-health-checks.md` | The local route asks the running daemon first | Modified page; the route `ze doctor` takes to a runtime row | "running daemon, so the local route asks the daemon before it answers alone" | agrees | The new rows are runtime rows, so `ze doctor` on a device shows them whenever its daemon answers, and `show doctor` shows them on every command surface. With the daemon stopped there is no hub client and no failed attempt to report; the local route then adds its `doctor-runtime-not-checked` row, as it does for every runtime check |
| `docs/architecture/doctor-and-health-checks.md` | Recent events: the report bus in show doctor | Modified page; the alternative carrier | "A transient problem (a peer reset that recovered, one failed retry) is no" | agrees | The page gives the bus to a problem no check can probe after the fact. A hub session that cannot open is a state that lasts until it opens, and each side holds it: the client in the state its owner already publishes, the hub in the managed doctor record. Both are read by a check, and nothing is raised on the bus, which could hold neither 2,000 addresses nor a clearing (Existing Mechanisms, "Report bus") |
| `docs/architecture/doctor-and-health-checks.md` | Every problem site names a doctor code | Modified page; `client.go` and `managed_doctor.go` log lines | "those. Three kinds of call are a problem site, and so is a finding the doctor" | agrees | The client's `connection lost, reconnecting` Warn line gains the `code` of the class it met and its fix. The hub's Warn line for a connection that ended on a TLS alert names `doctor-managed-certificate-refused`; the other pre-identification line keeps `doctor-managed-client-rejected`, which the record now backs |
| `docs/architecture/doctor-and-health-checks.md` | External plugin and managed fleet checks | `// Design:` page of `register_doctor_codes.go`; anchored to `internal/component/managed/doctor.go`, `register.go`; modified page | "registered by `internal/component/managed`. All four are post-config checks," | agrees | The sentence describes the checks that exist and stays true of those four. The component gains a fifth, `hub-session`, in the runtime phase, and the page's two tables gain the new rows; the `hub-unreachable` row keeps its text, a hub that does not answer a TCP probe, which is what the check does |
| `docs/architecture/doctor-and-health-checks.md` | Where each owner registers its check | Modified page | "`diagnostic.RegisterDoctorCheck()` from the owning component's `init()`" | agrees | The managed component owns the device's outbound hub connection, so it registers `hub-session` from its own `init()` in `register.go`; the hub-side check registers beside the managed runtime checks in `internal/component/plugin/doctor/register_external.go`, whose package owns them |
| `ai/META-DECISIONS.md` | CLI, config, and daemon surfaces | Added: owner rule on doctor coverage, the rule the judge's refusal of 2026-11-23 cites | "`ze doctor` reports every problem an operator can hit, so it can be used to" | agrees | C-10 makes one problem reachable at every rotation: a device that refuses the certificate the hub now presents. Before this revision it was in the logs of both sides and in no doctor row. Now the device's doctor reports it with its cause and fix (AC-43 to AC-46), the hub's doctor reports the address (AC-47), and a functional test produces it with a real certificate the client cannot verify (AC-48). The two neighbours at the same site, a refused authentication and an anchor `pki` does not define, are reported by the same check |
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
| `docs/architecture/pki/pki-store.md` | Consequences worth knowing | Anchored to `leaf.go`, `main.go` | "A certificate the operator NAMED is never reissued." | agrees | The reload issues a leaf for new hosts only when the configuration names no certificate. A named certificate is answered unchanged at every handshake, and whether it names a host added by reload is the operator's to arrange (Known Limitations) |
| `docs/architecture/api/process-protocol.md` | A refused connect-back | `// Design:` page of `leaf.go` | "fresh leaf once two thirds of the current one's life is spent." | agrees | `ServingLeaf` is unchanged and keeps its renewal rule. A reload that adds a served host builds a new holder for the new host list, which renews for that list, and the holder it replaces is dropped. The plugin acceptor's leaf is untouched: no reload changes the addresses it serves |
| `docs/architecture/pki/tls-listeners.md` | Decision: rotation does not rebind the hub listeners | Anchored to `main.go`, `main_reload.go` | "On every reload a hub listener resolves the certificate the reloaded config describes through `listenerTLSMaterial`: the named chain from the new store, or the self-signed pair when the config names none." | departs | The decision the section makes holds for the managed listener: it is rotated at the reload with no rebind, the listener keeps its socket, no session is closed and the next handshake serves the new certificate (C-10, AC-38). The quoted sentence does not hold for it, in two places. It does not resolve through `listenerTLSMaterial`, and with no name it presents the leaf the daemon's authority issues, never a self-signed pair. Nor do the two sentences after it: the managed listener has no `selfSignedSource`, and it installs through its own certificate source, not `UpdateTLSCertificate`. The table row "No name, a store: the self-signed pair" is false for it the same way. The design therefore scopes these sentences and that table to the web and looking glass listeners and adds the managed listener's own paragraph and table ("Documents departed from" gives the text before and after). What stays true of all three as written: "A reload never rebinds a listener to plaintext", the refusal of a name the new store does not define, and "Each hook takes its own server handle" |
| `docs/architecture/pki/tls-listeners.md` | Decision: a named certificate fails closed | Anchored to `main.go`, `main_reload.go` | "An EMPTY name is a different case: it means the operator configured no reference, and the established self-signed path is used unchanged." | departs | For the managed listener an empty name means the leaf the daemon's authority issues (`docs/architecture/fleet-config.md`, "Hub certificate and client trust"), and by C-10 a reload that removes the name installs that leaf. No self-signed pair exists on that listener. The rest of the paragraph holds in substance, a commit that removes the leaf returns the listener to the certificate a restart with that config would serve, but its words name the self-signed pair. The design scopes the paragraph to the web and looking glass listeners and states the managed listener's empty-name certificate beside it ("Documents departed from") |
| `docs/architecture/pki/tls-listeners.md` | Decision: a named certificate fails closed | Anchored to `main.go`, `main_reload.go` | "The hub declares that precedence once, in `listenerTLSMaterial`. Both hub listeners call it at start, and both reload hooks call it, so neither two surfaces nor a start and a reload can disagree about which certificate one config asks for." | departs | The principle is kept and the wording is not. The managed listener's precedence is declared once, in `managedCertificate`: the managed server calls it at construction, the reload calls it once at the certificate gate, and the apply installs what that call returned, so a start and a reload of the managed listener cannot disagree either. But that is a second function in the hub's process, where the sentence says the hub has one, and the paragraph ends "A second copy of the rule would be a future disagreement". It is not a copy of the rule: its second branch is another certificate (the issued leaf, renewed by its holder, where `listenerTLSMaterial` answers a self-signed pair as PEM), and both functions resolve a name through the same `pki.ServerTLSMaterial`, which is what fails closed. It cannot be the same function without freezing the leaf's renewal or importing `cmd/ze/hub` from `plugin/server` (Existing Mechanisms, "Listener certificate precedence, declared once"; Alternatives set aside). The page is corrected to say that the hub declares each of its two precedences once and to name both ("Documents departed from"; C-11) |
| `docs/architecture/pki/tls-listeners.md` | Decision: a named certificate fails closed | Anchored to `main.go`, `main_reload.go` | "The reload path refuses the same two names against the just-installed store." | agrees | The managed certificate becomes a third one the same gate refuses after `zepki.Load`, through `restorePKIAfter`, before any consumer applies (AC-39). The gate reaches it through the managed server's preparation, which calls `managedCertificate`, where it reads the web and looking glass names itself. The page's sentence and source anchor gain it, and the page states the asymmetry this leaves: the startup gate of `runYANGConfig` does not read the managed name, because a managed server whose name does not resolve is built at start, binds nothing and reports each address unbound (AC-41), so a hub booted with such a name refuses every later reload until the name is defined in `pki` or removed |
| `docs/architecture/pki/tls-listeners.md` | Decision: rotation does not rebind the hub listeners | Anchored to `main.go`, `main_reload.go` | "Unparseable material leaves the previous certificate serving." | agrees | A leaf the authority cannot issue for the new hosts leaves the previous leaf and host list serving the bound addresses; only the added address is not bound, and it is reported (AC-40) |
| `docs/architecture/pki/tls-listeners.md` | Design decisions | Decision of 2026-11-23; a page this spec modifies | "The looking glass may serve plain HTTP, because what it publishes is public, and one looking glass serves HTTP and HTTPS at the same time. Thomas's words, as voice-transcribed: "We should allow the looking glass to be HTTP. The information is public." and "We should be able to serve on both HTTP and HTTPS, not either." Asked where the choice lives and what happens to `ze.looking-glass.tls`, he answered "1a 2a": the choice is the `tls` leaf of each `server` entry, default true, and the environment variable is removed. The no-store rule applies to each entry: an entry that left `tls` unset serves HTTP with a warning on a daemon with no `database/` store and no named certificate, an entry that says `tls true` there is refused at start and at reload, and a reload does what a restart with the same config does" | agrees | The spec changes nothing of the looking glass's protocol: it adds no `tls` leaf on the `looking-glass` container, no environment variable and no fallback, and its change to `loader_extract.go` is the `label` list alone, leaving `extractLGServerList` and the `LGServerTLS` values as they are. The managed listener it adds to the page is no looking-glass listener and serves TLS only, with no protocol choice. Its page edits scope the web and looking glass sentences and keep the looking glass rules under that scope as today: the per-entry `tls` leaf, the no-store rows of the rotation table, `planLGTransports` and `updateLGTransport`. The reload's certificate gate keeps `lgListenerConfigFrom` and `checkLGTransport` beside the managed certificate it adds (AC-39), so a reload still does what a restart with the same config does on every looking-glass entry |
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
    - Certificate, pre-check: after the new pki store is installed and before any consumer applies, the reload asks the managed server to prepare the certificate of the new hub config, beside its checks of the web and looking glass names. The gate reads the certificate name and the serving addresses of the new config through the reader `startManagedServer` uses, and passes both as they are, an empty name included. The preparation calls `managedCertificate`, the function the server's construction calls, and changes nothing the listener presents. A name is resolved in the new store and its pair parsed, at every reload, so a renewal under the same name is prepared too. No name gives the leaf ze issues for `managedLeafHosts` of the serving addresses: a new leaf holder, issued at the call, or the holder in place when the listener already presents the issued leaf and it names every one of those hosts.
    - Certificate, what the gate does with an error. A name that does not resolve or material that does not parse refuses the reload whole and restores the prior store. A leaf the authority cannot issue is judged by what the listener presents now. Where it presents a named certificate, the reload is refused the same way, because nothing else could replace the name the config dropped (AC-39). Where it presents the issued leaf, or nothing is bound, the reload stands, the leaf in place is kept, and the apply leaves each added address unbound (AC-40). So no listener is left on a certificate the config no longer describes.
    - Certificate, apply, before any address is bound: the server's certificate source becomes what the gate's call prepared, whether a managed listener is bound or not (C-10). The apply decides nothing about the certificate and has no failure of its own for it. Where the leaf could not be issued, each added address is recorded unbound with that reason instead of being bound.
    - The listeners keep their sockets through all of it and no session is closed for a certificate: the next handshake reads the certificate source. A session opened before the reload keeps the certificate it negotiated until it reconnects.
11. Answer: `ManagedRegistrySnapshot` copies every declared and stored record, recomputes stale hub versions through `ReadConfig`, and lists the held clients once (`HeldClients`); the `ze-managed` handlers turn it into rows, apply the `label` pipe filter, and answer data, the same answer on every command surface.
12. A hub session that cannot open, on the client: each attempt of `runConnection` that fails before the session opens is classed where it failed (the trust anchor, the TLS handshake, the authentication answer; a failed TCP connect is classed too and reported by nothing here, because `hub-unreachable` owns it) and reported to the client's owner beside `OnConnection`. The owner keeps the last one and answers it in `ClientState`; the attempt that authenticates clears it. The reconnect log line carries the class's code and fix. The `hub-session` runtime check reads `ClientState` through the installed `FleetTarget` and answers one row, or none.
13. A connection that ends before a name, on the hub: `authRefused`, for an error that carries no claimed name, classes it (the remote ended the handshake with a TLS alert, or anything else) and records it in the managed doctor record under the remote address without its port. An authenticated session from that address removes its entries. The runtime checks read the record through `ManagedDoctorSnapshot`; the row for an address names the devices whose last session came from it, read from the registry.
14. Web: none in this spec. The estate part that reads these answers for the fleet pages is `plan/post-release/features/spec-fleet-ui-2-estate-tree-and-device-tables.md`'s (owner ruling of 2026-11-23).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Managed client to hub | TLS, `#0 auth`, newline-framed requests on `rpc.MuxConn` | No |
| Managed server to store | `storage.Storage` `WriteKey`, `ReadKey`, `ListKeys` on the borrowed store | No |
| Store to hub (observer) | One write-observer function, run after durability | No |
| Config to managed server | `ExtractHubConfig` at boot and at the reload pre-check; an apply call after promotion | No |
| pki store to managed server | The injected `TLSMaterialResolver` (`pki.ServerTLSMaterial`) and the daemon root (`Authority`), both read only inside `managedCertificate`: at construction, and at the reload's certificate gate | No |
| Hub to managed client (TLS) | The certificate source every listener reads per handshake; the client verifies the leaf against the host it dialed | No |
| `plugin/server` to `managed` | Exported snapshot function and types (value copies) | No |
| Hub client to its owner to doctor (client side) | A callback on `ClientConfig` beside `OnConnection`; a value in `ClientState`; the `FleetTarget` the hub installs, read by the `hub-session` check in the same package | No |
| Managed server to doctor (hub side) | `ManagedDoctorSnapshot`, extended with the per-address entries (value copies) | No |
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
- `ManagedServer` (`managed_serve.go`) - gains the registry, the handshake cap, the read bound and an apply method for reload. Its certificate source becomes replaceable behind `getCertificate`, and it gains the preparation the reload's gate calls, which runs `managedCertificate` for a hub config.
- `ServingLeaf` (`internal/component/plugin/leaf.go`) - unchanged; `managedCertificate` builds a holder per host list.
- `runReloadContext` (`cmd/ze/hub/main_reload.go`) - its certificate gate asks the managed server to prepare the managed certificate and refuses on its error; it holds no rule of its own about the name.
- `ManagedConfigService` (`managed.go`) - loses `connected`; `RegisterClient` and `UnregisterClient` move onto the registry.
- `startManagedServer` (`cmd/ze/hub/managed_server.go`) - builds the server whenever the store can serve one, publishes it for reload, extends the observer callback.
- `doReloadContext` (`cmd/ze/hub/main_reload.go`) - checks the hub extraction before any step and applies it after promotion.
- `internal/component/managed/register.go` - registers the two commands, their shapes, columns, pipe filter and completer.
- `internal/component/plugin/doctor/check_managed_runtime.go` - the new checks read the extended doctor record.
- `runConnection` and `RunManagedClient` (`internal/component/managed/client.go`) - class each failed attempt, report it to the owner, and name its code on the reconnect line.
- `managedClientRun` (`cmd/ze/hub/managed_owner.go`) and `ClientState` (`internal/component/managed/cmd_fleet.go`) - keep and answer the last failed attempt.
- `internal/component/managed/doctor.go`, `register.go`, `register_doctor_codes.go` - the `hub-session` runtime check and its three codes.
- `authRefused` (`internal/component/plugin/server/managed_doctor.go`) - records the connection that ended before a name; `handleConn` clears an address when a session from it authenticates.
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
| A-12 | A functional test can give the hub a `pki` certificate with its issuing CA in config, and run a managed client anchored on that CA with verification on, beside one anchored on the hub root | `test/plugin/geodns-dot-pki.ci` serves a `pki` certificate from config; `test/managed/managed-hub-ca-trust.ci` runs a verifying managed client | The proof of AC-38 on the real entry point moves to a Go integration test in `cmd/ze/hub` driving a reload and a real `tls.Dial` | Writing `fleet-reload-certificate-rotated.ci` | unvalidated |
| A-13 | The client's handshake error for a certificate it refuses is a typed value `errors.As` finds through the `tls handshake: %w` wrap: an unknown issuer, a host name the certificate does not carry, and a certificate outside its validity dates each have their own type or reason in `crypto/x509`, carried by the verification error of `crypto/tls` | The Go standard library at the version `go.mod` names (1.27); `runConnection` wraps with `%w` | The causes cannot be told apart by type. The check then reports one cause with the library's own text, never a cause matched from that text, and the design of the per-cause fix goes back to DESIGN | `TestRunConnectionClassesCertificateFailure`, which produces each failure with a real certificate and a real handshake | unvalidated |
| A-14 | When a client refuses the hub's certificate, the hub's first read on that connection fails with a `*net.OpError` whose `Op` is `remote error`, found through `read auth request: %w`, and no other pre-identification failure (a closed connection, bytes that are not TLS, a timeout) has that shape | `tls.Listen` runs the handshake on the first read (`StartListeners`); `AuthenticateWithLookup` wraps the read error with `%w`; the shape of a received alert is the standard library's and is not read from Ze's code | The hub cannot tell a refused certificate from the other failures by type. `doctor-managed-certificate-refused` then has no producer and the design of the hub side goes back to DESIGN: error text is never matched | `TestManagedCertificateRefusedByClientRecorded`, with a real TLS client that refuses the hub's leaf, beside a plain TCP connect that closes | unvalidated |
| A-15 | A functional test can make a connected managed client reconnect without restarting its daemon, and read `show doctor` on the client's daemon and on the hub's | `request fleet disable` ends the hub client and `request fleet enable` builds and starts one (`FleetTarget.StopClient`, `StartClient`); `test/managed/fleet-disable-severs.ci` runs a hub and a client daemon | The proof of AC-48 moves to a Go integration test in `cmd/ze/hub` that runs the real managed server, the real client and both runtime checks in one process | Writing `fleet-certificate-refused-doctor.ci` | unvalidated |

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
| R-15 | A reload binds an address whose host the served leaf does not name, so every verifying client refuses the hub there | A client's handshake error on the new address; before this revision, by design | The reload's call of `managedCertificate` derives the issued leaf's hosts from every serving address of the new config and issues for them, and the apply installs that leaf before it binds (AC-35, AC-36). A named certificate is the operator's material: Ze does not reissue it, and whether it names the new host is the operator's to arrange, as at boot |
| R-16 | The authority cannot issue the leaf for the new hosts at a reload | The issue error at the certificate gate | The previous leaf holder and its host list keep serving the bound addresses; each added address is left unbound and recorded with the issue error under `doctor-managed-listener-unbound`; the rest of the reload stands, as for an address that does not bind (R-8, AC-40) |
| R-18 | A reload names a managed certificate the pki store does not hold, or material that does not parse; accepted, the apply could not install it, and the next restart would start no managed listener, because a name fails closed at construction | The resolver's or the parser's error at the reload's certificate gate | Refused whole at the gate the web and looking glass names use, before any consumer applies, with the prior pki store restored (AC-39). A name removed on a serving hub has its replacement leaf issued at the same gate, by the same call, so the apply never leaves the listener on a name the config dropped |
| R-19 | The certificate source is replaced while a handshake reads it | A race report | The source is read through one synchronised holder per handshake, as the web listener's `atomic.Pointer[tls.Certificate]` is; the apply replaces the holder's content in one store. A leaf holder is never changed after it is built: a new host list is a new holder, so no handshake sees a holder between two host lists |
| R-26 | A start and a reload of the managed listener disagree about the certificate one configuration asks for, because the rule is written in two places | A hub that presents one certificate after a reload and another after a restart with the same configuration | One function decides it, `managedCertificate`, and one reader takes the name and the serving addresses from a hub config; the construction and the reload's gate call both, and the gate and the apply branch on nothing about the name. `TestManagedReloadCertificateMatchesStart` compares, for each kind of configuration, what a reload installs with what a server built from that configuration presents |
| R-20 | A client that cannot verify the certificate a reload installs (a name it has no CA for) | Its handshake error at its next reconnect | The operator's commit decides it, as for the web listener: sessions opened before the reload stay up, and a client that reconnects and cannot verify the new certificate ends the handshake before its token is written. Neither side reported that before this revision: `doctor-hub-unreachable` is a TCP dial and stays silent while the hub's port answers, the client logged `connection lost, reconnecting`, and the hub logged a connection with no name. Now both report it. The device's `ze doctor` raises `doctor-hub-certificate-unverified` with the cause (issuer, host name, validity dates) and the fix for the way it anchors its hub (AC-43, AC-44). The hub's `show doctor` raises `doctor-managed-certificate-refused` for each address that refused its certificate, naming the devices last seen from it, so the operator who committed the certificate sees its effect where the commit was made (AC-47). `test/managed/fleet-certificate-refused-doctor.ci` produces the case with a real certificate (AC-48). Option (a) of C-10 had the same handshake failure after its restart ("Choices for Thomas") |
| R-21 | Noise on the hub's port (a scanner that connects and closes, a peer that speaks no TLS) fills the per-address record and pushes out the addresses that refused the certificate | Rows of `doctor-managed-client-rejected` for addresses that are no device | The two classes are kept apart, 16 addresses each: a connection that ended without a TLS alert never takes the place of one that refused the certificate. Each record is bounded whatever connects, and a repeat from a kept address updates its entry (count, last time, last error) without adding one |
| R-22 | A whole fleet refuses a rotated certificate: up to 2,000 addresses fail, and the record keeps 16 | `doctor-managed-certificate-refused` rows at the bound | The record also counts, per class, the failures and the distinct addresses it has forgotten since the hub started, and the check adds one row saying so when the count is not zero. The full list of devices that are not connected is `show fleet devices`, which every row's fix names. The report bus was set aside for this reason: 2,000 subjects exceed what it holds for every producer (Existing Mechanisms, "Report bus") |
| R-23 | An entry outlives its cause: a device taken out of service after it failed never authenticates, so its address would be reported for as long as the hub runs | A row whose last time is old | An entry whose address has not failed again for `managedUnidentifiedKeep`, one hour, is forgotten when the record is next read. A device that still fails retries at most 60 seconds apart (`newBackoff(1*time.Second, 60*time.Second, 0.1)` in `RunManagedClient`), so it refreshes its entry 60 times within the period, and a row that is shown is at most an hour old |
| R-24 | Several devices behind one address (NAT): one authenticates and clears the entry while another still fails | A row that disappears and comes back | Stated: the hub knows a nameless connection only by its address. The device that still fails records the entry again at its next attempt, at most 60 seconds later, and its own doctor reports it throughout |
| R-25 | The client's failure report is read while the client writes it | A race report | The owner keeps the last failure behind an atomic pointer to an immutable value, as it keeps the hub's last answer (`managedClientRun.hubAnswer`); `ClientState` copies the value |

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
| A reload adding the first serving block on `127.0.0.2`, then a client that verifies the hub's certificate dialing it | → | the leaf the gate's call of `managedCertificate` issues for the new hosts, installed by the apply before the bind | `test/managed/fleet-reload-listener-verified.ci` |
| A reload changing `plugin hub server <block> certificate` on a serving hub, then a client that verifies only the new certificate connecting | → | the reload's certificate gate and the apply's install behind the handshake's certificate source | `test/managed/fleet-reload-certificate-rotated.ci` |
| A reload naming a managed certificate the pki block does not define | → | the reload's certificate gate | `TestReloadRefusesUnresolvableManagedCertificate` (`cmd/ze/hub`) |
| `ze data rm meta/fleet/device/<name>/record` on a running hub | → | the observer's mark and the writer's reconciliation | `TestRegistryOnlineDeleteForgetsRemovedDevice` |
| A managed client's reconnect that meets a certificate it cannot verify, then `show doctor` on that client | → | the failure class in `runConnection`, the owner's `ClientState`, the `hub-session` runtime check | `test/managed/fleet-certificate-refused-doctor.ci` |
| The same reconnect, then `show doctor` on the hub | → | `authRefused`'s per-address record and the `managed-certificate-refused` runtime check | `test/managed/fleet-certificate-refused-doctor.ci` |
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
| AC-38 | On a hub whose managed listener is serving and a client is connected, a reload changes the certificate: a name set where the issued leaf was served, a name changed, a name removed, or the same name whose `pki` material changed | The reload is accepted, and the first handshake after it presents the certificate the new config names: the named chain, or the leaf ze issues when the name was removed. The listener keeps its socket, and the session opened before the reload stays up and keeps answering (its `connected-since` is unchanged). Proven non-vacuously: a client that verifies only the new certificate (anchored on the CA that issued `fleet-hub`, and not on the hub root) is refused before the reload and admitted after it, and the handshake's presented leaf equals the new chain's leaf. Red before this design: the handshake after the reload still presents the old certificate | A session opened before the reload keeps the certificate it negotiated until it reconnects, because TLS negotiates once per connection; the listener closes no session for a certificate (C-10) |
| AC-39 | A reload whose hub config names a managed `certificate` the new `pki` store does not hold, or whose material does not parse as a certificate and key; or a reload that removes the name on a serving hub while the daemon's authority cannot issue the leaf that replaces it | The reload is refused whole before any consumer applies, the prior `pki` store is restored and the running config is kept, so the listener keeps the certificate it presents. For a name, the error names `plugin hub server <block> certificate <name>`, says the name is not defined in `pki` (or that its material does not parse), and gives the fix: define `pki certificate <name>`, or remove the leaf. For the leaf, the error names the authority's issue error and the fix: keep `certificate <name>` in the configuration until the authority issues again | None: a name that resolves to nothing is an error, never a fallback, and a listener is never left on a certificate the config no longer describes |
| AC-40 | At a reload that adds a serving address, the daemon's authority cannot issue the leaf for the new hosts | The added address is not bound and `ze doctor` reports it under `doctor-managed-listener-unbound` with the issue error; the previous leaf keeps serving the addresses already bound, and the rest of the reload stands | The clients of the added block cannot connect there until a reload or restart issues the leaf, because binding it would present a leaf they refuse |
| AC-41 | A hub starts with a serving block whose `certificate` name does not resolve, then a reload defines the name in `pki` | At start the managed server exists, binds no address and reports each one unbound with the unresolved name: nothing is served, which is the fail-closed rule. The reload binds the addresses and presents the named certificate. A reload in between that still names the undefined certificate, whatever else it changes, is refused by AC-39's gate | The start and reload asymmetry: start accepts the name and reports it, while every reload that still carries it is refused until the name is defined in `pki` or removed (Known Limitations). None otherwise: a server that was not built at start would ignore the reload that repairs it |
| AC-42 | `ze data rm meta/fleet/device/<name>/record` (or `delete data key`) on a running hub | For a device no serving block declares, the device leaves `show fleet devices` at once. For a declared device, its history fields are cleared, its live facts stay, and its next event writes a new record. A record overwritten with other bytes is logged and written back from memory | The registry's own writes change nothing, because the writer reads back the bytes it last wrote |
| AC-43 | A managed client's daemon whose hub presents a certificate that the client's anchor did not issue: the hub serves a `pki` certificate from another CA, and the client is anchored on the hub root. Produced by a real handshake against a real certificate, never by a constructed error value | `show doctor` on the client, and `ze doctor` there (which asks the daemon), report `doctor-hub-certificate-unverified`, warning: the device refuses the certificate its hub at `<host:port>` presents because the anchor it holds (named: `pki ca <name>`, the root `ze init` stored, or the system trust store) did not issue it, so it holds no hub session, receives no configuration change and keeps retrying; and the fix for that anchor (the table "The hub session failure"). The reconnect Warn line carries the same `code` and fix. Red before this design: `ze doctor` reports nothing while the port answers, and the line carries no code | `doctor-hub-unreachable` stays silent, because the hub's port answers: it reports a hub that does not answer a TCP dial and nothing else |
| AC-44 | The same daemon, with the hub's certificate refused for another cause, each produced with a real certificate: one that does not carry the host the client dials; one outside its validity dates; and a client whose configuration names a `ca` that `pki` does not define | The same code with the cause told apart and its own fix. Host name: the row gives the host dialed and the names the certificate carries. Validity: the row gives the certificate's two dates and the device's time, so a wrong clock can be told from an expired certificate. Undefined anchor: the row names `pki ca <name>` and says no handshake was tried. A verification failure of any other kind is reported under the same code with the library's text and the unknown-issuer fix | None: every way the client refuses the hub's certificate is one code, and the cause decides the fix |
| AC-45 | The same daemon, where the handshake fails without a verification failure (the hub closes the connection during it, the endpoint speaks no TLS, the handshake times out), and where the handshake succeeds and the hub answers the authentication with anything but `ok` | `doctor-hub-handshake-failed` for the first, with the library's text and the fix `ze show doctor` on the hub at that address, which reports a saturated handshake cap or a listener that did not bind. `doctor-hub-auth-rejected` for the second: the hub refused this device's name or secret, the fix `ze show doctor` on the hub, whose `doctor-managed-client-rejected` row names the refusal, then the `client` entry or the device's secret | A failed TCP connect is kept as the last failure and reported by nothing here, because `doctor-hub-unreachable` owns a hub that does not answer |
| AC-46 | The failing client then authenticates; or the daemon runs no hub client | The row is gone from the next `show doctor`: the attempt that authenticates clears the failure. Only the last failed attempt is kept, so one row at most. A daemon with no hub client (unmanaged, or `request fleet disable`) reports nothing | With the daemon stopped the row cannot exist, because the state is the running client's; the local route's `doctor-runtime-not-checked` row says the runtime checks were not run |
| AC-47 | On a hub, connections that end before they name a client: a real TLS client that refuses the hub's leaf, twice from one address; a TCP connection that closes without TLS; then a client that authenticates from the first address | `show doctor` on the hub reports `doctor-managed-certificate-refused`, warning, for the first address: the address without its port, how many times and when it first and last refused, the alert it sent, the certificate the listener presents now (the name, or the leaf ze issues), the devices whose last session came from that address (from the registry, or that none is recorded), and the fix: `ze doctor` on that device, or have the hub present a certificate its devices verify. The second failure updates the entry and adds none. The TCP connection is reported under `doctor-managed-client-rejected` with its address and error, and never takes a place among the 16 certificate entries. After the authentication from the first address its entries are gone. A 17th address in a class forgets the oldest and is counted in the row that says how many were forgotten. An entry not repeated for one hour is forgotten | Devices behind one address share an entry, because a connection with no name has only its address (R-24). The record is lost at a hub restart, because a device that still fails records it again within 60 seconds |
| AC-48 | Through real daemons: a hub serves `edge-01`, anchored on the hub root, with the leaf ze issues. A reload sets `certificate fleet-hub`, issued by another CA. `edge-01`'s hub client is then stopped and started (`request fleet disable`, `request fleet enable`), so its next handshake meets `fleet-hub`. A second reload removes the name | After the first reload `edge-01` stays connected (AC-38). After its client restarts: `show doctor` on `edge-01` reports `doctor-hub-certificate-unverified` with the unknown-issuer cause, `show doctor` on the hub reports `doctor-managed-certificate-refused` naming `edge-01`'s address and `edge-01`, and `show fleet devices` lists `edge-01` not connected. After the second reload `edge-01` connects at its next attempt and both rows are gone. Red before this design: neither doctor reports anything | None: this is the case C-10 makes reachable, shown on both sides with a real certificate the client cannot verify |

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
| 8 | Changes the hub's certificate by reload, and a device that trusts only the new certificate connects while the devices already connected stay up | reload → certificate gate → apply (install behind the certificate source) → next handshake | `fleet-reload-certificate-rotated.ci` |
| 9 | Rotates the hub's certificate to one a device cannot verify, sees on the hub which address refuses it and which device that is, reads the cause and the fix on the device, and repairs it | reload → device reconnect → handshake refused → client record → `show doctor` on the device; hub record → `show doctor` on the hub; second reload → reconnect → both rows cleared | `fleet-certificate-refused-doctor.ci` |

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
| `TestManagedApplyServingCertificateChangeNextHandshake` | same | AC-38 on a bound server with a client session open: for a name set where the leaf served, a name changed and a name removed, a TLS dial after the apply presents the configured certificate (its leaf bytes equal the new chain's leaf, or, after the removal, a leaf the authority issued that verifies against the root), while the session opened before the apply still answers a ping on the same listener. Red before: the dial after the apply presents the old certificate | |
| `TestManagedApplySameNameNewMaterialNextHandshake` | same | AC-38: the same name with other material in the new store is presented from the next handshake (a renewal) | |
| `TestManagedApplyLeafIssueFailureLeavesAddressUnbound` | same | AC-40, with an authority that refuses to issue | |
| `TestManagedStartUnresolvedCertificateBindsNothingThenReloadBinds` | same | AC-41 | |
| `TestManagedReloadCertificateMatchesStart` | same | R-26, one declaration: for a configuration with a name and for one with none, on each of two host lists, the certificate a reload installs on a running server equals what a server constructed from that configuration presents (the same named leaf bytes; for the issued leaf, the same hosts and the same issuer). Red if the reload decides the certificate anywhere but in the function the construction calls | |
| `TestManagedReloadKeepsLeafThatCoversHosts` | same | A reload with no name, on a server that presents the issued leaf, whose host list is unchanged or only lost a host, issues nothing: the handshake presents the same leaf bytes as before. A reload that adds a host presents a new leaf naming it, and that leaf is reissued for the same hosts once two thirds of its life is spent (a fake clock) | |
| `TestReloadRefusesUnresolvableManagedCertificate` | `cmd/ze/hub/main_reload_test.go` | AC-39: an undefined name, and material that does not parse, are refused before any consumer applies, the prior pki store restored, the message's three parts | |
| `TestReloadRefusesUnissuableManagedLeafOnNameRemoval` | same | AC-39: a name removed on a serving hub with an authority that refuses to issue is refused whole, and the listener keeps presenting the named chain | |
| `TestReloadRefusedWhileBootCertificateUnresolved` | same | AC-41's asymmetry: a hub started with an undefined managed name refuses a reload that still names it, and accepts the one that defines it | |
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
| `TestRunConnectionClassesCertificateFailure` | `internal/component/managed/client_test.go` | AC-43, AC-44 and A-13: a real TLS listener serves, in turn, a leaf from a CA the client is not anchored on, a leaf that does not carry the dialed host, an expired leaf and a leaf not yet valid; `runConnection` with a real handshake reports each to the owner callback with its own cause, the anchor used, and for the validity case the certificate's dates. A client whose `ca` is not in `pki` reports the undefined anchor with no dial. Red before: no report is made | |
| `TestRunConnectionClassesHandshakeAndAuthFailure` | same | AC-45: a listener that closes during the handshake, and one that speaks no TLS, report a handshake failure that is no verification failure; a hub that answers the authentication with an error reports the refused authentication; a refused TCP connect reports the connect step | |
| `TestRunConnectionAuthenticatedClearsFailure` | same | AC-46: after a failed attempt, an attempt that authenticates reports the session open, and the owner's state holds no failure | |
| `TestReconnectLineNamesCode` | same | AC-43: the reconnect Warn line carries the `code` of the class and its fix | |
| `TestManagedClientRunKeepsLastFailure` | `cmd/ze/hub/managed_owner_test.go` | The owner answers the last reported failure in `ClientState`, clears it when the client reports connected, and answers none once the client's goroutine has returned | |
| `TestCheckHubSessionCertificateUnverified` | `internal/component/managed/doctor_test.go` | AC-43, AC-44: from a `ClientState` holding each cause and each anchor, the check answers `doctor-hub-certificate-unverified` with the three parts of the message and the fix of that cause and anchor | |
| `TestCheckHubSessionHandshakeAndAuth` | same | AC-45: the two other codes with their fixes; a failed connect answers no row | |
| `TestCheckHubSessionSilent` | same | AC-46: no target installed, a client that is connected, and a client with no failure each answer no row | |
| `TestHubSessionCheckDeclared` | same | The check is registered in the runtime phase and declares its three codes; `ze show diagnostic` describes each, and the description of `doctor-hub-unreachable` says a TCP probe of the first hub block | |
| `TestManagedCertificateRefusedByClientRecorded` | `internal/component/plugin/server/managed_auth_refused_test.go` | AC-47 and A-14: against a real `ManagedServer`, a TLS client whose root pool lacks the hub's authority fails its handshake; the doctor snapshot holds one entry for its address without the port, in the certificate class, with the alert text. A second failure updates the count and the last time. A TCP connect that closes is held in the other class. Red before: the snapshot holds neither | |
| `TestManagedUnidentifiedBoundPerClass` | same | AC-47, R-21, R-22: 17 addresses (`127.0.0.2` onward as source addresses) in one class keep 16 and count one forgotten; 17 in the other class displace none of the first | |
| `TestManagedUnidentifiedClearedByAuthentication` | same | AC-47: a session that authenticates from an address removes that address's entries in both classes | |
| `TestManagedUnidentifiedForgottenAfterKeep` | same | AC-47, R-23: with the clock moved past `managedUnidentifiedKeep`, the snapshot holds no entry; one second before it, it still does | |
| `TestCheckManagedCertificateRefused` | `internal/component/plugin/doctor/check_managed_runtime_test.go` | AC-47: the row's three parts, the devices last seen from the address (and the wording when none is recorded), the certificate presented, the forgotten-count row | |
| `TestCheckManagedClientRejectedUnidentified` | same | AC-47: a connection that ended without an alert is a `doctor-managed-client-rejected` row with its address and error | |
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
| Addresses kept per class of nameless connection | 0 to 16 | 16 | N/A | the 17th forgets the oldest and is counted |
| Age of a kept address since its last failure | 0 to `managedUnidentifiedKeep` (one hour) | one hour less one second | N/A | one hour: forgotten |
| Error text kept per address, and per client failure | 0 to `managedDetailMax` (1,024 bytes) | 1,024 | N/A | 1,025 is cut |

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
| `fleet-reload-certificate-rotated` | `test/managed/fleet-reload-certificate-rotated.ci` | A hub serves `edge-01` (anchored on the hub root) with the leaf ze issues; `pki` defines `fleet-hub` and the CA that issued it. `edge-02`, anchored on that CA only, is refused by its own handshake. A reload sets `certificate fleet-hub`: `edge-02` connects and is listed connected, and `edge-01` stays listed connected with its `connected-since` unchanged (AC-38, A-12). Red before the apply installs the certificate on a serving hub: `edge-02` is still refused after the reload | |
| `fleet-certificate-refused-doctor` | `test/managed/fleet-certificate-refused-doctor.ci` | AC-48, A-15: a hub rotates by reload to `fleet-hub`, issued by a CA `edge-01` does not hold; `edge-01` leaves and rejoins the fleet so its hub client reconnects; `show doctor` on `edge-01` reports `doctor-hub-certificate-unverified` (unknown issuer) and `show doctor` on the hub reports `doctor-managed-certificate-refused` naming `edge-01`; a second reload removes the name, `edge-01` connects and both rows are gone. Red before the two checks exist: neither `show doctor` holds the code | |
| `TestFleetManyClientsPerf` | `cmd/ze/hub/fleet_perf_test.go` (`fleetperf`) | 2,000 connected devices | |

### Interop Tests (Scope: protocol)
- N/A: the managed channel has no peer implementation other than Ze's own client, which the functional tests run.

## Files to Modify
- `internal/component/plugin/server/managed.go` - remove `connected`; `RegisterClient` and `UnregisterClient` move to the registry.
- `internal/component/plugin/server/managed_serve.go` - handshake cap replacing `managedMaxConns`; registry calls at admit, fetch, ack, disconnect; bounded reader; zero-listener start; apply for reload; `Serves` reads the live served set.
- `internal/component/plugin/server/managed_serve.go` (the certificate) - `managedCertificate` stays the one function that decides the managed listener's certificate for a configuration, and is called at construction and by the preparation the reload's gate calls. It takes the certificate name, the serving addresses and the leaf holder in place, if any, and answers a certificate source. The server keeps that source replaceable, serving or not, read per handshake through the one `getCertificate`; a name that does not resolve at construction leaves the server built with nothing bound and each address reported unbound.
- `internal/component/plugin/server/managed_doctor.go` - record write and read failures per device, handshake saturation, the oversize cause, an address left unbound because its leaf could not be issued.
- `pkg/plugin/rpc/framing.go`, `pkg/plugin/rpc/conn.go` - a reader and a connection with a caller-given maximum.
- `pkg/fleet/envelope.go` - `AckErrorMax`.
- `internal/component/managed/client.go` - bound the ack error.
- `pkg/zefs/keys.go` - `KeyFleetDeviceRecord`.
- `internal/component/plugin/yang/ze-plugin-conf.yang` - `label` list under the hub `client` entry.
- `internal/component/plugin/types.go` - labels on `HubServerConfig`.
- `internal/component/config/loader_extract.go` - extract labels; refuse disagreeing labels and more than 4,096 names.
- `cmd/ze/hub/managed_server.go` - build when the store can serve, publish for reload, observer marks stale; one reader of the certificate name and the serving addresses of a hub config, called by `startManagedServer` and by the reload's gate.
- `cmd/ze/hub/main.go` - install the targets whatever the client count.
- `cmd/ze/hub/main_reload.go` - pre-check extraction, apply after promotion; the certificate gate asks the managed server to prepare the managed certificate of the new config, beside its checks of the web and looking glass names, and refuses on the error (a name that does not resolve, material that does not parse, a leaf that cannot be issued where a named certificate is presented); the prepared certificate is handed to the apply.
- `internal/component/managed/register.go`, `internal/component/managed/yang/ze-managed-cmd.yang` - two commands, shapes, columns, filter, completer.
- `internal/component/plugin/doctor/check_managed_runtime.go` - four checks: the three above and `managed-certificate-refused`; the `managed-client-rejected` check also answers the connections that ended before a name without a TLS alert.
- `internal/component/plugin/doctor/register_external.go` - registers the new runtime checks beside the two managed ones.
- `internal/component/plugin/server/doctor_codes.go` - four codes and their entries (`doctor-managed-device-record-unwritable`, `doctor-managed-device-record-unreadable`, `doctor-managed-handshake-saturated`, `doctor-managed-certificate-refused`), beside `doctor-managed-client-rejected`, the package that owns them.
- `internal/component/plugin/server/managed_doctor.go` (the nameless connections) - `authRefused` classes and records a connection that ended before it named a client, per remote address, in two bounded classes with their forgotten counts and the keep period; the snapshot carries them; an authenticated session clears its address.
- `internal/component/managed/client.go` (the failed attempt) - `runConnection` classes each failure before the session opens and reports it through a callback of `ClientConfig` beside `OnConnection`; the reconnect Warn line names the class's code and fix.
- `internal/component/managed/cmd_fleet.go` - `ClientState` carries the last failed attempt as a value. The answer of `show fleet status` is unchanged.
- `cmd/ze/hub/managed_owner.go` - `managedClientRun` keeps the last failed attempt, clears it when the client reports connected, and answers it in `state`.
- `internal/component/managed/doctor.go`, `register.go` - the `hub-session` runtime check and its registration.
- `internal/component/managed/register_doctor_codes.go` - `doctor-hub-certificate-unverified`, `doctor-hub-handshake-failed`, `doctor-hub-auth-rejected`; the description of `doctor-hub-unreachable` corrected to a TCP probe of the first hub block.
- `docs/architecture/doctor-and-health-checks.md` ("External plugin and managed fleet checks": both tables), `docs/architecture/fleet-config.md` ("What doctor reads from the managed listener", "Hub certificate and client trust") and `docs/guide/fleet-config.md` ("Troubleshooting": the two rows on a client that fails on the hub's certificate name the codes, and a new part says how to change the hub's certificate without cutting devices off) - with the two checks (Phase 7).
- `cmd/ze/hub/fleet_perf_test.go` - 2,000 held sessions.
- `docs/architecture/fleet-config.md`, `docs/guide/fleet-config.md`, `docs/guide/command-reference.md`, `docs/architecture/api/commands.md`, `docs/architecture/zefs-format.md`, `docs/architecture/doctor-and-health-checks.md`, `docs/features/cli-commands.md`, `docs/architecture/api/ipc_protocol.md`, `docs/architecture/hub-architecture.md` - the pages each phase makes wrong.
- `docs/architecture/pki/tls-listeners.md` - what the managed listener's certificate does at a reload (Phase 6). Three of the page's statements are corrected, which is the departure "Documents departed from" records with the text before and after; the rest are additions. The section "The certificate page after this spec", below, lists every edit by place.
- `docs/architecture/fleet-config.md`, "Hub certificate and client trust" - additions, no statement corrected (Phase 6): the issued leaf names every served host after a reload, a reload that adds a served host mints a leaf as a restart does, a changed certificate is presented from the next handshake with no session closed, and the reload's refusals. `docs/guide/fleet-config.md` gains that a certificate changed by reload is served from the next handshake. `docs/architecture/pki/pki-store.md` and `docs/architecture/api/process-protocol.md` owe no edit: `ServingLeaf` is unchanged.
- `docs/architecture/fleet-config.md`, `## Design decisions` - the C-10 row, written by the edit that recorded the ruling (2026-11-23); no further change owed.

## Files to Create
- `internal/component/plugin/server/managed_registry.go` - record, writer, load, snapshot.
- `internal/component/plugin/server/managed_registry_test.go`
- `internal/component/plugin/server/managed_apply.go` - the reload apply.
- `internal/component/plugin/server/managed_apply_test.go`
- `internal/component/managed/cmd_devices.go` - the two handlers and the completer.
- `internal/component/managed/cmd_devices_test.go`
- `test/managed/fleet-devices-connect.ci`, `fleet-device-detail.ci`, `fleet-devices-restart.ci`, `fleet-devices-declared.ci`, `fleet-devices-reload-add.ci`, `fleet-devices-reload-remove.ci`, `fleet-devices-label.ci`, `fleet-devices-diverged.ci`, `fleet-reload-listener-verified.ci`, `fleet-reload-certificate-rotated.ci`, `fleet-certificate-refused-doctor.ci`
- The fixture scenarios those three `.ci` files run, beside `managed/hub-ca-trust` under `internal/test/fixture/`, where a client that verifies the hub needs one (A-10, A-12)

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

### The hub session failure (client side, live only, never persisted)

The running hub client reports each attempt that fails before its session
opens, and its owner keeps the last one. It is cleared by the attempt that
authenticates and dropped with the client. It holds: the time, the hub
address dialed, the step, the cause, the anchor the attempt used (in the
order of `clientTLSConfig`: the `pki ca` the configuration names, the root
`ze init` stored, verification switched off, the system trust store), the
library's error text cut at 1,024 bytes, and, for a certificate the client
refused, the names it carries and its two validity dates.

| Step and cause | Code (warning) | The fix the row gives |
|----------------|----------------|-----------------------|
| TCP connect failed | none from this check | `doctor-hub-unreachable` reports it, as today |
| Anchor: the configuration names a `ca` that `pki` does not define | `doctor-hub-certificate-unverified` | Define `pki ca <name>` in this device's configuration on the hub with the root that issued the hub's certificate, then `ze init --managed --force --hub-ca <file>` on the device so that it fetches it |
| Handshake: the anchor did not issue the certificate | `doctor-hub-certificate-unverified` | By anchor. Stored root: `ze init --managed --force --hub-ca <file>`, the file holding the root that issued the hub's certificate (what `ze show pki local-ca pem` prints on the hub when the hub issues its own leaf). `pki ca <name>`: correct that entry in the device's configuration on the hub, then the same command. System trust store: name a `ca` in the device's configuration on the hub, then the same command. Every row also names the other side of the choice: have the hub present a certificate this anchor verifies (`plugin hub server <block> certificate`) |
| Handshake: the certificate does not carry the host dialed | `doctor-hub-certificate-unverified` | On the hub: give the serving block the `ip` devices dial (a block on `0.0.0.0` puts no host in the leaf ze issues), or name a `certificate` that carries that host; or correct `host` in the device's `client` block |
| Handshake: the certificate is outside its validity dates | `doctor-hub-certificate-unverified` | Renew `pki certificate <name>` on the hub and commit: the next handshake presents it (C-10). The row shows the device's time beside the dates, because a wrong clock gives the same refusal |
| Handshake: any other verification failure | `doctor-hub-certificate-unverified` | As for an issuer the anchor does not cover, with the library's text |
| Handshake: failed with no verification failure | `doctor-hub-handshake-failed` | `ze show doctor` on the hub at that address (a saturated handshake cap, a listener that did not bind); correct `host` and `port` when the address is no managed listener |
| Authentication: the hub answered anything but `ok` | `doctor-hub-auth-rejected` | `ze show doctor` on the hub, whose `doctor-managed-client-rejected` row names the refusal (unknown name, wrong secret, name already connected), then the `client` entry on the hub or the device's secret |

Every row says what is wrong, why it matters (the device holds no hub session,
receives no configuration change and keeps retrying; it keeps running the
configuration it has) and the fix, and the final wording is written under
`ai/rules/user-facing-errors.md` at implementation. The fix line is one
command; where the repair has a step on the hub first, the message carries it.
With verification switched off no certificate is refused, so that anchor
reaches only the last two rows.

### Connections that end before a name (hub side, live only, never persisted)

The managed doctor record gains two bounded lists beside the unknown names,
keyed by the remote address without its port:

| Class | Told by | Code | Bound |
|-------|---------|------|-------|
| The remote refused the hub's certificate | The read fails with a TLS alert the remote sent (A-14) | `doctor-managed-certificate-refused` (warning) | 16 addresses, the oldest forgotten first |
| Any other end before a name | Every other error with no claimed name: a closed connection, bytes that are not TLS, a timeout, a malformed first line | `doctor-managed-client-rejected` (warning), the code its log line already names | 16 addresses, apart from the first class |

An entry holds the address, the listener address that accepted it, the first
and last time, the count, and the last error text cut at `managedDetailMax`.
A repeat from a kept address updates its entry. Each class counts the
addresses it has forgotten since the hub started, and the check adds one row
per class saying so when that count is not zero. An entry is removed when a
session authenticates from its address, and forgotten when it has not been
repeated for `managedUnidentifiedKeep` (one hour), judged when the record is
read, so no timer and no goroutine is added. The certificate row names the
certificate the listener presents now and the devices whose last session came
from that address (the registry's `source-address`), or says that none is
recorded. Its fix is `ze doctor` on that device, which names the cause, with
the other side of the choice: have the hub present a certificate its devices
verify. `show fleet devices` lists every device that is not connected, and
the row names it.

### The certificate page after this spec

Every edit Phase 6 makes to `docs/architecture/pki/tls-listeners.md`. A
"corrected" row changes a statement of the page and is part of the departure
(C-11); a "scoped" row names the listeners a true statement is about; an
"added" row adds text.

| Place on the page | Kind | The edit |
|-------------------|------|----------|
| Intro and its consumer table | added | The managed listener joins the consumers; its leaf is `plugin hub server <block> certificate` |
| "Decision: a named certificate fails closed", the paragraph on an EMPTY name | corrected | Scoped to the web and looking glass listeners. A sentence states the managed listener's case: with no name it presents the leaf the daemon's certificate authority issues, at a reload too |
| Same section, "The hub declares that precedence once, in `listenerTLSMaterial`" | corrected | The hub declares each of its two precedences once: `listenerTLSMaterial` for the web and looking glass listeners, `managedCertificate` for the managed listener. Each is called by its listener's start and by its reload |
| Same section, "Both hub listeners are checked before the daemon starts either one" | scoped | Names the web and looking glass listeners |
| Same section, "The reload path refuses the same two names" | added | The gate also asks the managed server to prepare its certificate and refuses on the error, with the source anchor. The start and reload asymmetry is stated: the startup gate does not read the managed name, a managed server whose name does not resolve starts with nothing bound, and every later reload that still carries the name is refused until it is defined in `pki` or removed |
| "Decision: rotation does not rebind the hub listeners", first paragraph | corrected | The resolution through `listenerTLSMaterial`, `selfSignedSource` and `UpdateTLSCertificate` is scoped to the web and looking glass listeners. A paragraph states the managed listener's rotation: the certificate the gate's call of `managedCertificate` prepared is installed after promotion behind the source `getCertificate` reads |
| Same section, the table of what the next handshake presents | corrected | Scoped to the web and looking glass listeners. A second table gives the managed listener's rows (below) |
| Same section, the consumer and hook table | added | The managed listener's row. Reload hook: the gate's preparation and the apply's install. Running server: the `ManagedServer` certificate source, read per handshake by `getCertificate` |
| Same section, "Both hub servers clear `tls.Config.Certificates`" | scoped | Names the web and looking glass servers. A sentence says the managed listener is built with the callback alone (`StartListeners`) |
| Source anchors of both sections | added | `managedCertificate`, the preparation, and the gate's call |

The managed listener's table on the page:

| Reloaded config | What the next handshake presents |
|-----------------|----------------------------------|
| A name the new store defines | That chain |
| A name the new store does not define, or material that does not parse | The reload is refused, the previous certificate stays |
| No name | The leaf the daemon's authority issues, naming `127.0.0.1` and the host of every served address |
| No name, and the authority cannot issue, on a listener that presents a named certificate | The reload is refused, the named certificate stays |
| No name, and the authority cannot issue, on a listener that presents the issued leaf | The leaf in place stays, and an address the reload added is left unbound and reported |

### Changing the hub's certificate without cutting devices off (guide)

A client holds one anchor, so a rotation to a certificate from another CA has
no period in which both verify. By C-10 the sessions open at the commit stay
up, which gives the order `docs/guide/fleet-config.md` documents: commit the
certificate on the hub, then give each connected device its new anchor in its
configuration through its open session; a device that was offline at the
commit is repaired on the device with the fix its doctor gives. The page is
written after reading, at `errEndpointChanged` in `client.go`, what a changed
`ca` in an applied configuration does to the open session, and it states that
behaviour as the code has it. A renewal under the same CA needs none of this.

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
| Doctor code per problem class | Yes | Record write failure: `doctor-managed-device-record-unwritable`, runtime phase, `checkManagedDeviceRecordUnwritable`. Unreadable stored record: `doctor-managed-device-record-unreadable`, runtime phase. Handshake cap reached: `doctor-managed-handshake-saturated`, runtime phase. Oversize line: existing `doctor-managed-client-rejected` with new cause `ManagedRejectOversize`. Duplicate session: existing `doctor-managed-client-rejected` (`ManagedRejectNameConnected`). Removed client reconnecting: existing `doctor-managed-client-rejected` (`ManagedRejectUnknownName`). Reload address that does not bind: existing `doctor-managed-listener-unbound`, now also recorded by the apply. Reload address left unbound because its leaf could not be issued, and an address left unbound at start because the certificate name did not resolve: the same code, with that reason. Certificate changed by reload on a serving hub: no fault and no code, because the apply installs it (C-10). Reload naming a certificate `pki` does not define, material that does not parse, or a name removed while the authority cannot issue its replacement leaf: a refusal of the reload whose message carries the fix; the config is not applied, so no runtime state remains. Disagreeing labels and too many clients: refusals at verify, whose message carries the fix; no runtime state remains, so no doctor check. `show fleet device` unknown name and not serving: operator errors with their fix; no runtime fault. A device that refuses its hub's certificate (unknown issuer, host name, validity dates, undefined anchor): `doctor-hub-certificate-unverified`, runtime phase, the `hub-session` check of `internal/component/managed`. A hub handshake that fails otherwise: `doctor-hub-handshake-failed`, same check. A hub that refuses the device's name or secret: `doctor-hub-auth-rejected`, same check. On the hub, an address that refused the hub's certificate: `doctor-managed-certificate-refused`, runtime phase, `checkManagedCertificateRefused`. Any other connection that ended before a name: existing `doctor-managed-client-rejected`, now recorded per address. The hub checks are in `internal/component/plugin/doctor/check_managed_runtime.go` and their codes in `internal/component/plugin/server/doctor_codes.go`; the client check is in `internal/component/managed/doctor.go` and its codes in `register_doctor_codes.go`. If A-6 finds a non-blob store case, `doctor-managed-store-unavailable` is added there |
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
| 6 | Has a user guide page? | Yes | `docs/guide/fleet-config.md` (registry, labels, reload, scale, the commands on every command surface; "Troubleshooting" gains the three client codes and `doctor-managed-certificate-refused`, its two rows on a client that fails on the hub's certificate name the code, and a new part says how to change the hub's certificate without cutting devices off). `docs/guide/web-interface.md` (real estate, not-available parts) moved with the web work to `fleet-ui-2` |
| 7 | Wire format changed? | Yes | `docs/architecture/fleet-config.md` "Protocol": hub read bound, ack error bound |
| 8 | Plugin SDK/protocol changed? | Yes | `pkg/plugin/rpc` gains a bounded reader: `docs/architecture/api/process-protocol.md` framing paragraph names that the plugin protocol keeps 16 MiB |
| 9 | RFC behavior implemented, changed, or newly proven? | N/A | No RFC |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` if the perf harness invocation changes (`./le test integration stress-fleet` names 2,000) |
| 11 | Affects daemon comparison? | N/A | No comparison row covers fleet management |
| 12 | Internal architecture changed? | Yes | `docs/architecture/fleet-config.md` (registry, cap, reload, and "Hub certificate and client trust": the leaf's hosts and the certificate under reload); `docs/architecture/pki/tls-listeners.md` (the managed listener as a consumer, its precedence and its rotation, with the page's statements about "a hub listener" scoped to the web and looking glass listeners: "The certificate page after this spec"); `docs/architecture/hub-architecture.md`; `docs/architecture/doctor-and-health-checks.md`, "External plugin and managed fleet checks" (the two tables gain the new checks, and the sentence on the managed component's checks gains its runtime one), and `docs/architecture/fleet-config.md`, "What doctor reads from the managed listener" (the nameless connections, which the page's source says are logged only) and "Hub certificate and client trust" (what each doctor reports when a client refuses the certificate). `ai/doors/hub-engine.md` owes no edit: by C-10 its "One reload path" sentence and its startup trap stay true. `docs/architecture/fleet-interface.md` (parts, join, not available) moved with the web work to `fleet-ui-2` |
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
record that cannot be written or read and a saturated handshake cap. It also
reports, on both sides, a hub session that cannot open: on a device, the hub
certificate it refuses with the cause and the fix, a handshake that fails
otherwise, and an authentication the hub refuses; on the hub, each address
that refused its certificate, with the devices last seen from it. Because
a reload now binds serving addresses, the certificate they present follows
it: the leaf the hub issues names the host of every address served after the
reload, a hub with no managed listener yet serves the certificate its new
config names, a changed certificate on a hub whose managed listener is
already serving is presented from the next handshake with no session closed,
as Thomas decided (C-10), and a name `pki` does not define refuses the
reload. What the managed listener presents for a configuration is decided
by one function, which its start and its reload both call, so a reload and a
restart with the same configuration present the same certificate; the page
on listener certificates is corrected to name that rule beside the one the
web and looking glass listeners share (C-11). The web
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
    ze issued, and nothing says so. After, by Thomas's decision of 2026-11-23
    (C-10): the reload is accepted and the next handshake presents
    `fleet-hub`. `edge-01`, connected before the commit, stays connected; a
    device that trusts only the CA of `fleet-hub` connects from the commit
    on. Renewing `fleet-hub` in `pki` under the same name is the same case,
    with no restart of ze and no routing session dropped. Removing the name
    again makes the next handshake present the leaf ze issues.
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
    define `pki certificate fleet-hbu` or remove the leaf. A hub that was
    started with that name serves no managed client and reports each address
    unbound; every reload that still names `fleet-hbu` is refused the same
    way until the operator defines it or removes it.
14. `ze data rm meta/fleet/device/edge-02/record` on a running hub, for a
    device a reload removed. Before: no record exists. After: `edge-02`
    leaves `show fleet devices` at once. For a declared device the command
    clears its history and keeps its live facts.
15. `ze doctor` on `edge-01` after the hub rotated to `fleet-hub`, issued by
    a CA `edge-01` does not hold, and `edge-01` reconnected. Before:
    nothing. `doctor-hub-unreachable` stays silent because the hub's port
    answers, and the log says `connection lost, reconnecting` every minute.
    After: one warning, `doctor-hub-certificate-unverified`: this device
    refuses the certificate its hub at `192.0.2.1:1791` presents, because
    the hub root it stored at `ze init` did not issue it, so it receives no
    configuration change and keeps retrying; fix
    `ze init --managed --force --hub-ca <file>`, the file holding the root
    that issued the hub's certificate, or have the hub present a
    certificate this root verifies. A certificate that does not name
    `192.0.2.1`, or one past its dates, gives the same code with that cause
    and its own fix.
16. `show doctor` on the hub in the same case. Before: nothing; the hub's
    log holds a connection "refused before it named a client". After: one
    warning, `doctor-managed-certificate-refused`: the device at
    `192.0.2.7` refused the certificate this hub presents (`fleet-hub`) 14
    times since 10:02, last at 10:16; `edge-01` last connected from that
    address; fix `ze doctor` on that device, or present a certificate its
    devices verify; `show fleet devices` lists the devices that are not
    connected.
17. A device whose secret the hub does not accept. Before: its doctor
    reports nothing, and only the hub's does. After: its own `ze doctor`
    reports `doctor-hub-auth-rejected` and sends the operator to the hub's
    row.

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
| Accept a changed serving certificate and keep it until the next restart, reported by a log line and `doctor-managed-certificate-restart-pending` (C-7; option (a) of C-10) | Thomas chose option (c) of C-10 on 2026-11-23. It cost five departures from the design pages, a doctor code, and a restart of ze, dropping every routing session, for each renewal |
| Refuse the reload that changes a serving hub's certificate, with the fix `restart ze`, as a changed external plugin is refused (option (b) of C-10) | Thomas chose option (c). A refused commit is not promoted, so a restart after it boots the old certificate and the operator has no way to make the change from a command surface. It would also refuse a reload over a renewed certificate that the web listener shares and can install |
| Move the managed certificate out of the configuration (a certificate the managed server owns at run time) | Thomas: "Keep the certificate in the configuration." It stays a `pki` name the config carries, read at every reload |
| Close the managed sessions when the certificate changes, so every client re-verifies at once | `docs/architecture/pki/tls-listeners.md` decides a hub listener keeps its socket and the next handshake serves the new chain; a reload never drops the fleet for a change each client meets at its next reconnect |
| Leave the leaf as built and document that a serving address added by reload needs a restart or a named certificate | A reload that binds an address no verifying client can use is a defect of this spec's own reload, and the address would look healthy on the hub |
| Decide the managed certificate at start in `managedCertificate` and at a reload in the hub's gate and the apply (the text the judge refused) | Two declarations of one rule: a start and a reload could come to present different certificates for one configuration, which is what `docs/architecture/pki/tls-listeners.md` forbids for a hub listener |
| One resolver for all three listeners: `listenerTLSMaterial` taking the listener's certificate with no name as a parameter, the web and looking glass listeners passing their self-signed source and the managed listener its issued leaf (option (b) of C-11) | It would keep the page's sentence as written, and it does not fit the code. `listenerTLSMaterial` answers PEM material that the listener installs once; the issued leaf lives 24 hours and is reissued by its holder at the handshake (`ServingLeaf`), so a PEM answer would expire on a hub that is neither reloaded nor restarted for a day. Making the function answer a certificate source instead would rewrite the web and looking glass path that commit 2984d3eaa5 has just settled. And the managed listener's start is `NewManagedServer` in `plugin/server`, which can import neither `cmd/ze/hub` nor the pki component, so the hub would have to resolve the certificate and hand it in, moving the leaf holder out of the server that serves it |
| Install the managed certificate as a third reload hook beside `updateWebCertificate`, before promotion, with the same undo | The undo would need the running configuration's serving addresses to rebuild the leaf it replaced, and the leaf would name the hosts of a configuration not yet promoted while the addresses of the running one are still bound. Preparing at the gate and installing after promotion needs no undo: nothing the listener presents changes before the reload is accepted |
| Replace the host list of the leaf holder in place (the earlier text of this spec) | Done at the gate, it would change the served leaf before the reload is accepted; done at the apply, the reload would decide the certificate in a second place. A new holder per host list is what the construction already builds, and it leaves `leaf.go` unchanged |
| Rebind every managed listener when the leaf's hosts change | Every session would drop for a change the next handshake can read; `docs/architecture/pki/tls-listeners.md` decides a hub listener keeps its socket |
| Issue one leaf per listener, each for its own host | A second leaf holder per block and a certificate chosen per socket, where one leaf naming every served host does the job, as it does at start |
| Bind the first serving block added by reload with the issued leaf whatever name it carries | The name wins whenever it is set (`docs/architecture/fleet-config.md`, "Hub certificate and client trust") |
| Treat an online removal of a device record as invisible until the next start | The fix doctor gives for an unreadable record of a removed device would do nothing on a running hub, where the store accepts the command |
| Leave a client that refuses the hub's certificate to `doctor-hub-unreachable` (the text the judge refused) | That check is a TCP dial and stays silent while the hub's port answers; the case would be in the logs only, against the owner rule on doctor coverage |
| Make `hub-unreachable` run a TLS handshake from the doctor process | It would report what the doctor process meets, not what the daemon's client met; it would run a handshake with no token against a hub that caps unauthenticated handshakes, at every `ze doctor`; and it could not see a refused authentication. The daemon already knows why its session did not open, so it reports that |
| Raise the client's failure on the report bus as an active warning, cleared at the next session | The bus could hold the one subject, but the client already publishes its state to the fleet commands; a second copy on the bus would be worded and ranked as a past event by `recent-events`, with one fix line for every cause |
| Report the hub side on the report bus, the way the plugin acceptor reports a refused connect-back | Its subjects there would be remote addresses, up to 2,000 after one rotation, against 1,024 active warnings and 256 error events shared by every producer; and an error event cannot be cleared when the address authenticates |
| Report the hub side only, or the client side only | The operator who commits the certificate is on the hub and must see its effect there; the cause and the fix are only knowable on the device, which is the side that refused. Each side reports what it knows |
| Put the address that refused the certificate under `doctor-managed-client-rejected` | That code says the hub refused a client. Here the client refused the hub, the fix is the hub's certificate or the device's anchor, and after a rotation it is the one row to look for; `ze show diagnostic` explains one code at a time |
| Keep every nameless connection in one list | A scanner on the port would push out the devices that refused the certificate (R-21) |
| Key the hub's record by address and port | The port changes at every attempt, so one failing device would fill the record |
| One code per verification cause on the client | The causes share what is wrong (the device refuses the hub's certificate) and differ in the fix, which the row carries; three codes for one refusal would triple the registry for no added answer |
| Declare `group` in `fleet-1` | `group` is the template unit `fleet-2` answers (owner, 2026-11-19) |
| Keep the 128-connection cap held for life | 2,000 devices cannot connect; the acceptor's figure of 32 bounds the handshake flood, and configuration bounds sessions |
| A literal read maximum | The largest legal message is declared once, as `fleet.HeldConfigMax` |
| Build the managed server at reload when the first client appears | Two lifecycles, two stops, and the data and divergence targets would need reinstalling |
| Ship the estate part, the source split and the not-available state with the registry | First-release fleet features are reached through commands; the web parts moved to `fleet-ui-2` (owner ruling of 2026-11-23, "1 b") |

### Documents departed from

One page: `docs/architecture/pki/tls-listeners.md`, in three statements
(three `departs` rows of the Design documents table). It is put to Thomas as
C-11 and the page is corrected in Phase 6, in the same work.

The cause is one fact. The page describes the certificate of "a hub
listener" and means the web and looking glass listeners: its intro lists
three consumers and the managed listener is none of them. By C-10 the managed
listener is now rotated at a reload like them, so it joins the page. But with
no name it presents the leaf the daemon's certificate authority issues
(`docs/architecture/fleet-config.md`, "Hub certificate and client trust"),
and C-10 has the reload install the certificate the configuration describes,
so a removed name installs that leaf. Every sentence of the page that says a
hub listener with no name presents a self-signed pair, or that the hub has
one precedence function, is false of it.

| Statement on the page today | After this spec | Why |
|-----------------------------|-----------------|-----|
| "An EMPTY name is a different case: it means the operator configured no reference, and the established self-signed path is used unchanged." | For the web and looking glass listeners, an EMPTY name means the operator configured no reference, and the established self-signed path is used unchanged. For the managed listener an empty name means the leaf the daemon's certificate authority issues. Either way a commit that removes the leaf returns the listener to the certificate a restart with that config would serve | The managed listener has no self-signed pair |
| "The hub declares that precedence once, in `listenerTLSMaterial`. Both hub listeners call it at start, and both reload hooks call it, so neither two surfaces nor a start and a reload can disagree about which certificate one config asks for." | The hub declares each precedence once. The web and looking glass listeners share `listenerTLSMaterial`: both call it at start, and both reload hooks call it. The managed listener's is `managedCertificate`: the managed server calls it at construction, and the reload calls it at the certificate gate and installs its answer. So neither two surfaces nor a start and a reload can disagree about which certificate one config asks a listener for. The paragraph's last sentence, that a second copy of the rule would be a future disagreement, stays, and now says of what: a second copy of either rule | Two rules that differ in the certificate presented with no name, and in its kind (PEM material installed once; a leaf its holder reissues). One function for both is option (b) of C-11 |
| "On every reload a hub listener resolves the certificate the reloaded config describes through `listenerTLSMaterial`: the named chain from the new store, or the self-signed pair when the config names none." With the two sentences after it, and the table row "No name, a store: the self-signed pair, loaded from the store or generated into it" | On every reload the web and looking glass listeners resolve the certificate the reloaded config describes through `listenerTLSMaterial`, and the paragraph and its table go on as today under that scope. A new paragraph and table state the managed listener's rotation: the reload's gate has `managedCertificate` prepare the named chain, or the leaf issued for the hosts of every served address when the config names none, and the apply installs it behind the source the handshake reads | The managed listener resolves through its own function and presents the issued leaf |

What a user sees is unchanged by the correction itself: it changes what the
page says, not what any listener does. What the managed listener does at a
reload is items 10 to 13 of "Before and after". The web and looking glass
listeners behave as commit 2984d3eaa5 left them.

The alternative that would leave the page's second statement as written is
one resolver for all three listeners. It is set aside for the reasons in
"Alternatives set aside", and Thomas decides it as C-11.

History. The five departures of the first revision of 2026-11-23 were one fact,
decided by Thomas as C-7: on a hub whose managed listener is serving, a
changed certificate took effect at the next restart. His answer to C-10
reverses it (Task, "Owner ruling of 2026-11-23: C-10"), and each of the five
rows of the Design documents table agrees since then:

| Statement | Why it agrees now |
|-----------|-------------------|
| `docs/architecture/hub-architecture.md`, "the external plugins and their modules alone: every other part of the config" | The managed certificate reloads without a restart, so the external plugins stay the only exception |
| `docs/architecture/pki/tls-listeners.md`, the sentence that read "A hub listener re-resolves the name in the new store." until commit 2984d3eaa5 rewrote it | The managed listener is rotated at the reload: prepared at the gate, installed with no rebind, served from the next handshake. The sentence that replaced it is one of the three statements above, departed from for another reason |
| `docs/architecture/fleet-config.md`, "The name wins whenever it is set" | A name set by reload wins at that reload |
| `ai/doors/hub-engine.md`, "they change only when `ze` restarts, and nothing else in the config needs one" | Nothing about the managed listener needs a restart |
| `ai/doors/hub-engine.md`, "so a changed RADIUS or TACACS+ server was accepted and applied to nothing" | A changed managed certificate is applied at the reload that accepts it |

The pages also gain text (Files to Modify): what a reload does to the
managed listener's certificate is added to `fleet-config.md` as additions to
statements that stay true, and to `tls-listeners.md` with the three
corrections above. `ai/doors/hub-engine.md` is not edited: both of its
sentences stay true.

### Choices for Thomas

| # | Choice | Options | Decision |
|---|--------|---------|----------|
| C-1 | Where the registry lives | (a) a record inside the hub's managed server; (b) a separate component fed by events | (a), DESIGN gate 2026-11-19 |
| C-2 | When the records are persisted | (a) on connect, disconnect, fetch and ack, under a family outside `meta/config/` and `meta/managed/`; (b) on every event, in place | (a), DESIGN gate 2026-11-19 |
| C-3 | The connection cap | (a) lift it here: a handshake cap, sessions bounded by declared clients, a bounded reader, 2,000 devices proven; (b) a later spec | (a), DESIGN gate 2026-11-19 |
| C-4 | Reload of the client set | (a) fixed here, proven by a `.ci` that adds a client by reload; (b) a later spec | (a), DESIGN gate 2026-11-19 |
| C-5 | Offline answer of the commands | (a) daemon only; (b) an offline route over the stored records | (a), final gate 2026-11-19 |
| C-6 | Removed devices | (a) listed by the commands, the interface lists declared devices only; (b) shown in the interface too | (a), final gate 2026-11-19; the interface half is `fleet-ui-2`'s |
| C-7 | A changed serving certificate | (a) needs a restart, recorded as a Known Limitation, with the page corrections above; (b) applied at reload | (a), final gate 2026-11-19. Superseded by C-10 (2026-11-23): Thomas chose to apply the change at the reload |
| C-8 | `group` and `template` | (a) `fleet-2`'s; (b) declared here | (a), final gate 2026-11-19 |
| C-9 | The fleet web parts | (a) commands in the first release, pages after it; (b) the estate part with the registry | (a), ruling of 2026-11-23 (voice-transcribed "1 b") |
| C-10 | What a reload does when it changes the certificate a serving hub presents, now that the reload must already touch the certificate (the table below) | (a) accepted, served at the next restart, reported by a log line and `ze doctor`: C-7 as decided, with the notice added; (b) the reload is refused with the fix `restart ze`; (c) applied at the reload, as the web and looking glass certificates are | (c), 2026-11-23. Thomas (voice-transcribed): "Keep the certificate in the configuration." then "In that case on commit we can perform the certificate change on the listener." Reading (c) confirmed with him. Supersedes C-7. Recorded in `docs/architecture/fleet-config.md`, "Design decisions". The comparison in option (c), as it was put to him, is the install on the running listener with no rebind. On a removed name each listener presents the certificate a restart with that configuration would serve: the managed listener the leaf ze issues, the web and looking glass listeners their self-signed pair (commit 2984d3eaa5) |
| C-11 | How `docs/architecture/pki/tls-listeners.md` reads once the managed listener is on it. The page says "The hub declares that precedence once, in `listenerTLSMaterial`" and that a hub listener with no name presents its self-signed pair. Evidence: with no name the managed listener presents the issued leaf (`docs/architecture/fleet-config.md`, "Hub certificate and client trust"; `managedCertificate`), and C-10 installs it at the reload; `listenerTLSMaterial` answers PEM installed once, the issued leaf is reissued by its holder every 16 hours of a 24-hour life (`ServingLeaf`); `plugin/server` cannot import `cmd/ze/hub` | (a) Correct the page: two precedences, each declared once and called by its listener's start and reload, `listenerTLSMaterial` for the web and looking glass listeners and `managedCertificate` for the managed listener; the page's sentences about a self-signed pair are scoped to the first two ("Documents departed from"). No code of the web or looking glass path changes. (b) Keep the page's sentence: one resolver for the three listeners, taking the listener's certificate with no name as a parameter. `listenerTLSMaterial`, both servers' install and the managed server's construction change so that all three take a certificate source in place of PEM, and the leaf holder moves to the hub | (a), 2026-11-23. Thomas approved the design with the words "approved, C-11 (a)" (`./le spec design approve`, digest e1897d459b10). Phase 6 writes this decision as a row of a `## Design decisions` section of `docs/architecture/pki/tls-listeners.md`, beside the corrections "Documents departed from" lists |

C-10 in what the operator sees, as it was put to Thomas. The hub `central`
serves `edge-01` with the leaf ze issued, and the operator commits
`certificate fleet-hub`. He chose (c):

| Option | The commit | The listener | What tells the operator | Cost |
|--------|------------|--------------|-------------------------|------|
| (a), C-7 | Accepted | Presents the issued leaf until ze restarts, then `fleet-hub` | The log warning and `doctor-managed-certificate-restart-pending`, fix `restart ze` | Five design statements departed from and corrected. Renewing `fleet-hub` under its name (every year, or every 90 days with an automated CA) needs a restart of ze, which on a hub that routes drops every routing session. Until then the config names one certificate and the listener serves another |
| (b) | Refused: "the new config changes the certificate the managed listener serves, which changes only when ze restarts", fix `restart ze` | Unchanged | The refusal, and `doctor-config-reload-refused` | The same restart, and a refused commit is not promoted, so the restart boots the old certificate: the change can only be made by editing the config file and restarting. A renewal the web listener shares is refused with it |
| (c), chosen | Accepted | Presents `fleet-hub` at the next handshake; `edge-01` stays connected, the listener keeps its socket | Nothing to report about the commit: the config and the listener agree. A device that cannot verify the new certificate is reported, on the device and on the hub (below) | None of the five departures, no pending-restart code, no page correction (as put to Thomas; the later C-11 corrects `docs/architecture/pki/tls-listeners.md` for the managed listener). The apply already replaces the certificate source for a hub with no listener bound and already resolves the name at the reload's gate, so (c) removes the comparison and the pending record. Written in, it adds one thing found while writing it: the gate also parses a name's material and, for a name removed on a serving hub, issues the replacement leaf, so the apply cannot leave the listener on a certificate the config dropped (AC-39). A client that cannot verify `fleet-hub` fails at its next reconnect, as it would after the restart of (a). That failure was reported by no doctor check when this was put to Thomas, and the cell then said `doctor-hub-unreachable` would report it, which was false: that check is a TCP dial. The doctor coverage is owed under every option and is added by the revision of 2026-11-23: `doctor-hub-certificate-unverified` on the device and `doctor-managed-certificate-refused` on the hub (AC-43 to AC-48). It changes nothing in the choice |

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
   - Files: `keys.go`, `managed_registry.go`, `managed_doctor.go`, `check_managed_runtime.go`, `internal/component/plugin/server/doctor_codes.go`
4. **Phase: Commands** - rows, detail, label filter, diverged from `HeldClients`, stale hub versions through the observer (needs A-2)
   - Tests: `TestShowFleet*`, `TestRegistryDivergedFromHeldRecord`, `TestRegistryHubVersionStaleOnConfigWrite`, `fleet-devices-label.ci`, `fleet-devices-diverged.ci`, `fleet-devices-declared.ci`
   - Files: `cmd_devices.go`, `register.go`, `managed_server.go`
5. **Phase: Labels in config** - YANG list, extraction, agreement and count refusals
   - Tests: `TestExtractHubConfig*`
   - Files: `ze-plugin-conf.yang`, `types.go`, `loader_extract.go`
6. **Phase: Reload** - always-built server, apply, pre-check, post-promotion apply; the certificate under reload (C-10): `managedCertificate` called by the construction and by the preparation the reload's gate calls, a certificate source replaced at every reload, serving or not, a leaf issued for the new hosts before a bind, first-bind certificate, a changed certificate installed on a serving listener with no rebind
   - Tests: `TestManagedApply*` (AC-38's `TestManagedApplyServingCertificateChangeNextHandshake` red first: written against the apply before it installs on a serving listener, where the dial after the reload still gets the old certificate), `TestManagedStartUnresolvedCertificateBindsNothingThenReloadBinds`, `TestManagedReloadCertificateMatchesStart`, `TestManagedReloadKeepsLeafThatCoversHosts`, `TestReloadRefusesUnresolvableManagedCertificate`, `TestReloadRefusesUnissuableManagedLeafOnNameRemoval`, `TestReloadRefusedWhileBootCertificateUnresolved`, `fleet-devices-reload-add.ci` (red first), `fleet-devices-reload-remove.ci`, `fleet-reload-listener-verified.ci` (red first: written against the apply before the leaf's hosts follow it, where the client refuses the certificate), `fleet-reload-certificate-rotated.ci` (red first, the same way as AC-38's unit test)
   - Files: `managed_apply.go`, `managed_serve.go`, `managed_doctor.go`, `managed_server.go`, `main.go`, `main_reload.go`; `docs/architecture/fleet-config.md`, `docs/architecture/pki/tls-listeners.md` and `docs/guide/fleet-config.md` for what the managed listener's certificate does at a reload (Files to Modify, "The certificate page after this spec")
7. **Phase: A hub session that cannot open, in doctor** - the client's failed attempt (class, report to the owner, `ClientState`, the `hub-session` runtime check, three codes, the reconnect line's code); the hub's nameless connections (two bounded classes per address, cleared by authentication, forgotten after the keep period, `doctor-managed-certificate-refused`, the rows under `doctor-managed-client-rejected`). After Phase 6, whose reload produces the case
   - Tests: `TestRunConnectionClassesCertificateFailure` (red first: written against `runConnection` before it reports, with real certificates), `TestRunConnectionClassesHandshakeAndAuthFailure`, `TestRunConnectionAuthenticatedClearsFailure`, `TestReconnectLineNamesCode`, `TestManagedClientRunKeepsLastFailure`, `TestCheckHubSession*`, `TestHubSessionCheckDeclared`, `TestManagedCertificateRefusedByClientRecorded` (red first, with a real client that refuses the leaf), `TestManagedUnidentified*`, `TestCheckManagedCertificateRefused`, `TestCheckManagedClientRejectedUnidentified`, `fleet-certificate-refused-doctor.ci` (red first: neither `show doctor` holds the code)
   - Files: `internal/component/managed/client.go`, `cmd_fleet.go`, `doctor.go`, `register.go`, `register_doctor_codes.go`; `cmd/ze/hub/managed_owner.go`; `internal/component/plugin/server/managed_doctor.go`, `managed_serve.go`, `doctor_codes.go`; `internal/component/plugin/doctor/check_managed_runtime.go`, `register_external.go`; `docs/architecture/doctor-and-health-checks.md`, `docs/architecture/fleet-config.md`, `docs/guide/fleet-config.md`
   - Verify: A-13 and A-14 are confirmed or broken by the two red-first tests before any check is written; a broken one returns the per-cause design to DESIGN
8. **Phase: Connection bound** - handshake cap, bounded reader, ack error bound, oversize cause, metrics
   - Tests: `TestHandshake*`, `TestManagedReadMax*`, `TestFrameReaderMax*`, `TestSendConfigAckBoundsError`
   - Files: `managed_serve.go`, `framing.go`, `conn.go`, `envelope.go`, `client.go`, `managed_doctor.go`
9. **Phase: Scale** - 2,000 held sessions with the descriptor limit raised
   - Tests: `TestFleetManyClientsPerf`
   - Files: `fleet_perf_test.go`
10. **Phase: Documentation** - each phase edits its pages as it lands (the Documentation Update Checklist); this phase only checks anchors

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
| Naming | JSON keys kebab-case; hub codes `doctor-managed-*`, client codes `doctor-hub-*` beside `doctor-hub-unreachable`; wire methods `ze-managed:fleet-devices`, `ze-managed:fleet-device` |
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
| A refused hub certificate in doctor, on both sides | `fleet-certificate-refused-doctor.ci`; `ze show diagnostic doctor-hub-certificate-unverified` and `ze show diagnostic doctor-managed-certificate-refused` describe the codes |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Label keys and values by YANG; device names by YANG and at extraction; strict record decode; the read bound on every hub-read line |
| Resource exhaustion | Handshake cap; sessions bounded by declared names; writer pending set bounded by declared names; unknown-name record bounded at 16 (existing); nameless connections bounded at 16 addresses per class, each entry's text at `managedDetailMax`, whatever connects |
| Error leakage | The ack error and record errors are bounded and never carry a secret; a nameless connection's entry holds an address and the library's error text, never bytes the remote sent as a token, and the client's failure never holds its token; `source-address` is shown only to callers allowed to read `show fleet` |
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
| A reload that changes the certificate a serving hub's managed listener presents installs it on the running listener, served from the next handshake, no rebind, no session closed; the certificate stays in the configuration's `pki` block (owner, C-10, 2026-11-23, superseding C-7) | Keep it until the next restart, reported (C-7, option (a)); refuse the reload with `restart ze` (option (b)); close the sessions | Thomas's ruling; it is how the web and looking glass listeners rotate (`docs/architecture/pki/tls-listeners.md`), it removes the five departures C-7 cost, and a renewal needs no restart of a hub that routes |
| What the managed listener presents for a configuration is decided once, in `managedCertificate`, called by the server's construction and by the reload's certificate gate; the apply installs that call's answer | Decide it at start in `managedCertificate` and at a reload in the hub's gate and the apply; one resolver for all three hub listeners; a third reload hook before promotion | A start and a reload cannot disagree when they call one declaration, which is the principle of `docs/architecture/pki/tls-listeners.md`. The function cannot be `listenerTLSMaterial`: the certificate with no name differs, the issued leaf is a holder that reissues where that function answers PEM, and `plugin/server` cannot import the hub. The page is corrected to name both (C-11, open) |
| The gate's one call prepares everything the apply installs: it resolves and parses a name, or issues the leaf when the config names none | Let the apply fail after promotion and keep the previous certificate | The apply runs after promotion and cannot roll back, so every failure of the install is found before any consumer applies; a listener is never left on a certificate the config no longer describes (`docs/architecture/pki/tls-listeners.md`, "Decision: a named certificate fails closed") |
| A name removed from the config makes the managed listener present the leaf ze issues | Leave the named chain serving | The managed listener's certificate with no name is the issued leaf (`docs/architecture/fleet-config.md`), with C-10 the config and the listener always agree after a reload, and the web and looking glass listeners return to their own no-name certificate the same way (commit 2984d3eaa5) |
| A hub session that cannot open is a runtime doctor check on each side, reading state that side already holds: the client's owner-published state, the hub's managed doctor record | The report bus on either side; a TLS handshake run by the `hub-unreachable` probe; one side only | It is a state that lasts until the session opens, not a past event; the bus cannot hold a fleet's addresses nor clear them; the doctor process cannot see what the daemon's client met; the hub is where the certificate was committed and the device is where the cause is known (`ai/META-DECISIONS.md`, the doctor coverage rule) |
| The device's refusal of the hub's certificate is one code with its cause and a fix chosen by the anchor; the hub's view of it is a code of its own, apart from `doctor-managed-client-rejected` | One code per cause; the existing hub code | The causes share the problem and differ in the fix; on the hub the direction is the opposite of a rejected client and the fix is the certificate |
| The hub keeps nameless connections per address, two classes of 16, cleared by an authentication from the address and forgotten after one hour without a repeat | One list; address and port; no expiry | Noise must not push out the devices; the port changes per attempt; a device that still fails repeats within 60 seconds, so an hour is a safe silence |
| The issued leaf's hosts follow the reload with no rebind: a reload that adds a served host builds a new leaf holder for the new list, and a list the leaf in place already covers keeps it | Leave the leaf as built; rebind; one leaf per listener; replace the holder's host list in place | A bound address must be verifiable by the clients it is bound for; the handshake already reads the certificate source each time; a new holder is what the construction builds, so the reload needs no operation the start does not have |
| A hub with no managed listener bound takes the certificate its new config names | Bind with the issued leaf | The name wins whenever it is set (`docs/architecture/fleet-config.md`) |
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
- A session opened before a reload that changes the managed certificate keeps the certificate it negotiated until it reconnects: TLS negotiates once per connection, and by C-10 the listener closes no session for a certificate. A client that cannot verify the new certificate fails at its next reconnect. That is reported, not left to the logs: its own `ze doctor` raises `doctor-hub-certificate-unverified` with the cause and the fix, and the hub's `show doctor` raises `doctor-managed-certificate-refused` for its address (R-20, AC-43 to AC-48). `doctor-hub-unreachable` does not report it: that check is a TCP dial of the hub's port.
- The hub knows a connection that ended before a name only by its remote address, so devices behind one address share one doctor entry, the hub keeps the 16 most recent addresses per class and counts the rest, and the record does not survive a hub restart (R-22 to R-24). The device's own doctor is the complete report for that device.
- The client's doctor reports why its hub session does not open only while its daemon runs: the state is the running client's, so `ze doctor` with the daemon stopped says the runtime checks were not run. A first start that cannot verify its hub stops with its own message and fix, as before (`cmd/ze/ze_core_start.go`).
- Start and reload judge an unresolvable managed certificate name differently. At start the managed server is built with nothing bound and reports each address unbound (AC-41); a reload whose config still carries the name is refused by the certificate gate (AC-39), whatever else it changes, until the name is defined in `pki` or removed. `ze config validate` does not resolve the name. `docs/architecture/pki/tls-listeners.md` states the asymmetry (Files to Modify).
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
- [ ] AC-1..AC-13, AC-16..AC-27 and AC-34..AC-48 all demonstrated (AC-14, AC-15 and AC-28..AC-33 are `fleet-ui-2`'s)
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
