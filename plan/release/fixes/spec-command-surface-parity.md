# Spec: command-surface-parity

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | cli |
| Priority | P1 |
| Release reason | Shipped operator surfaces offer commands they cannot serve (web admin, SSH completion), omit commands they do serve, and leave a remote operator or AI without the local diagnostics (`ze doctor`, support archive, config inspection); this fixes those product defects by serving every command from the daemon and adds the per-surface check that keeps them fixed. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-01 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Thomas approved, in his words, "a check that every command appears on every
surface: each command is checked for CLI help and completion, the web UI, MCP
and gNMI, and a gap fails the check". The idea came from the BAMA skill set's
menu audit, where one loop walks every command and checks its menu title,
shortcut and icon.

At the approval gate Thomas settled the open questions, in his words:

| Question | Thomas's words | What it decides |
|----------|----------------|-----------------|
| Remove the local-only commands from daemon surfaces, or serve them there (A-2) | "there is no reason why the deamon could not run them and provide the same output"; "yes design so they can run. How can an AI remotely connecting the daemon could debug it without ze doctor for example." | Every local command, the G1 ones and the plain local ones (doctor, support archive, debug profiles, show interface, ping, traceroute, show diagnostic, skills, bgp decode and encode, show version, help, update serve), runs through the daemon with the same registered handler and the same output, and is reachable from MCP, REST, gRPC, SSH and the web. MCP is how an AI gets in, so MCP listing and execution is an acceptance criterion for each. The local `ze` route stays for when the daemon is unreachable (`ze doctor` on a broken daemon) |
| gNMI as a checked surface (A-3) | gNMI was listed in the brief by mistake; Thomas never asked for it | gNMI leaves the check's surface list. It serves the YANG config tree and no command, so it has nothing to check here |
| Approval | "ok we want to fix this gap, make sure the specs are ready" | The spec moves to `ready` |

Ze today has no single answer to "on which surfaces must this command appear".
Each surface builds its command list from a different subset of the command
sources, so surfaces disagree, and nothing compares them. The disagreement is
already user-visible:

| Id | Defect | Evidence (verified 2026-11-01 against source) |
|----|--------|-----------------------------------------------|
| G1 | 25 commands answered only in the caller's own process carry a YANG `ze:command` node with no daemon handler. The web admin nav and form, and SSH completion, are built from the raw YANG tree, so they offer these commands; running one dispatches to the daemon, which never registered it, and fails. The journal row says 26; its own list counts 25 (`show config running`, the ninth node of `ze-config-cli-cmd.yang`, is a daemon command) | `LoadBuiltins` and `loadBuiltinsWithAliases` (`internal/component/plugin/server/command.go`) register only wire methods a Go `RegisterRPCs` entry carries. No Go registration exists for the wire methods of: `show env list`, `get`, `registered` (`internal/plugins/env/yang/ze-env-cmd.yang`); `show plugin list`, `declarations`, `declarations config` (`internal/component/plugin/yang/ze-plugin-show-cmd.yang`); `show schema list`, `methods`, `events`, `handlers`, `protocol` (`internal/plugins/config-schema/yang/ze-schema-cli-cmd.yang`); `show yang tree`, `completion`, `doc` (`internal/plugins/config-yang/yang/ze-yang-cli-cmd.yang`); `show data list`, `cat`, `registered` (`internal/plugins/config-storage/yang/ze-storage-cli-cmd.yang`); `show config dump`, `diff`, `history`, `list`, `cat`, `fmt`, `graph`, `validation` (`internal/plugins/config-cli/yang/ze-config-cli-cmd.yang`). `cmd/ze/hub/service_web.go` passes `yangloader.BuildCommandTree(resolved)` to `HandleAdminView` and `HandleAdminExecute`; `cmd/ze/hub/session_factory.go` `buildCommandTree` starts SSH completion from `yang.BuildCommandTree`. No daemon-side surface calls `ServeLocal`, `LookupLocal` or `InvokeLocal` |
| G5 | 19 plain local commands with no daemon side at all are unreachable from every remote surface, so a remote operator or AI cannot run them: `doctor` (the daemon's `show doctor` is a second, narrower implementation), `request support archive`, the eight debug profile commands, `show skills`, `show skill`, `show diagnostic`, `request wireguard keypair`, `show bgp decode`, `show bgp encode`, `help command`, `help ai`, `update serve` | Each registers only through `command.MustRegisterLocal` or `MustRegisterLocalMeta` (`internal/component/doctor/register.go`, `internal/plugins/support/register.go`, `internal/plugins/debug/register.go`, `internal/plugins/skills/register.go`, `internal/plugins/explain/register.go`, `internal/plugins/diag/register.go`, `internal/component/bgp/cli/register.go`, `cmd/ze/ze_core_dispatch.go` `registerLocalCommands`); no `RegisterRPCs` entry carries any of them |
| G2 | A plugin command declared at Stage 1 with no YANG node never reaches the web admin nav, although the comment above `HandleAdminView` says plugin-contributed commands appear there | `cmd/ze/hub/service_web.go` builds the admin handlers from the YANG tree alone, once, at service start; `VisibleCommandEntries` is merged by SSH (`mergePluginCommands`, per session), the web terminal completer (`web_completer.go`, per request) and the `ze cli` client (`injectPluginCommands`), never by the admin handlers |
| G4 | `ze cli` serves the 20 local-data commands in process (`ServeLocal`, `internal/component/cli/client/main.go`, both `-c` and the interactive executor), but its interactive completion tree omits them | `buildRuntimeTreeFromDispatch` builds the tree from `AllBuiltinRPCs` filtered by `system command list`, plus plugin commands; no local registration feeds it |
| G3 | `./le cli catalog check` labels each local-data command that has a YANG node `daemon` or `read-only` | `Collect` (`internal/le/cli/catalog/catalog.go`) walks `cli.WireToPaths` first and assigns `daemon`/`read-only` by `IsReadOnlyPath`; the later `ListLocal` loop skips the path as already seen. A documentation accuracy defect of the published catalog |

The goal: every command is served by the daemon through the handler that
already answers it locally, so every remote surface, MCP first, can run it and
gets the same output as `ze <verb>`; one declaration answers which surfaces
must offer each command; every surface's own producer derives its list from
that answer; and a check asks each surface's real producer and fails, naming
the command and the surface, on any gap or any offer the surface cannot serve.

### Framing checked against the code

| Brief's claim | What the code says |
|---------------|--------------------|
| Four command sources | Five. Besides builtin RPCs, plugin Stage 1 declarations, local-data commands and streaming handlers, there are plain local commands (`command.MustRegisterLocal`, `MustRegisterLocalMeta`, 30 calls) served only by `ze <verb>`, not by `ze cli` (`ServeLocal` answers local-data paths only). `show yang doc`, `show data cat`, `show config cat`, `fmt`, `graph` are this kind and have YANG nodes, so they are part of G1 |
| 28 production local-data registrations | 20 in `ze` (`command.MustRegisterLocalData`), plus one in `le` (`internal/le/le/root/leroot.go`), which is development tooling and not a daemon command |
| Streaming commands may be listed as MCP tools and fail on one-shot dispatch | All eight streaming commands also register a one-shot builtin RPC, so they are in the dispatcher and in `buildCommandMeta`'s list. They do not fail: `handleMonitorInterfaceRate` (`internal/component/iface/cmd/interface_rate.go`) answers `{"status":"monitor-configured"}` and no data. Whether each one-shot answer is a real answer is AC-9 |
| Interactive or TUI-only is declared nowhere | The CLI live-view registry (`internal/component/cli/view_registry.go` `RegisterView`) declares the three full-screen views from their own `register_view_*.go`. Two of them, `monitor ping` and `monitor traceroute`, have a one-shot daemon RPC (`ze-ping:monitor-ping`, `ze-traceroute:monitor-traceroute`). `monitor bgp` has none: the dashboard drives one-shot pull pollers from the view (`register_view_dashboard.go`), and `internal/le/cli/dispatch/resolver.go` registers it by hand as dispatchable but not a builtin RPC. So `monitor bgp` is the one TUI-only command today |
| gNMI is a surface to check | Withdrawn by Thomas (A-3). gNMI serves the YANG config tree, not commands (`ai/doors/api-surfaces.md`, "Two surfaces do not go through the command registry") |

### Local commands already served by the daemon under the same path

Six local registrations share their path with a daemon RPC. The daemon already
serves them; the local handler is the no-daemon route. "Same output" here means
the two routes call one producer, which AC-14 judges:

| Path | Local handler | Daemon RPC |
|------|---------------|------------|
| `show ping` | `showPingLocal` (`internal/component/ping/cmd/register.go`) | `ze-ping:show-ping` `handleShowPing` |
| `monitor ping` | `monitorPingLocal`, a loop under `signal.NotifyContext(os.Interrupt)` | `ze-ping:monitor-ping` |
| `show traceroute`, `monitor traceroute` | `showTracerouteLocal`, `monitorTracerouteLocal` (`internal/component/traceroute/cmd/register.go`) | `ze-traceroute:show-traceroute`, `monitor-traceroute` |
| `show interface` | `internal/component/iface/cli/register.go` | `ze-iface:show-interface` (`internal/component/iface/cmd/show_interface.go`) |
| `show version` | `cmd/ze/ze_core_dispatch.go` | `ze-cmd:show-version` (`internal/component/cmd/show/show.go`) |

`doctor` is the seventh pair under two paths: the local `doctor` (`internal/component/doctor/doctor.go` `Run`) parses `--json`, `config <file>` and the `kernel-capabilities` mode, while the daemon `show doctor` (`internal/component/doctor/cmd/show.go` `HandleShowDoctor`) runs `diagnostic.RunDoctorChecks` plus the plugin doctor checks and has no kernel-capabilities mode. Two implementations of one question is the drift this spec removes.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/commands.md` - where a command is served; "Offline fallback", "Local-data commands", "Monitor Commands"
  → Decision: today a local-data command has no daemon side and registers no RPC; this spec changes that page to say the daemon also serves it through the same handler
  → Constraint: a hidden plugin command never reaches completion, help, MCP or the API list
- [ ] `docs/architecture/cli/command-completion.md` - how each completion tree is built
  → Constraint: help, usage and completion are derived from the owner's registration and schema, never written twice (`ai/doors/commands-cli.md`)
- [ ] `docs/architecture/web-interface.md` - admin console routes
  → Constraint: the admin console is generated from the command tree, with no static map
- [ ] `docs/architecture/mcp/overview.md` - tool derivation from the live command list
  → Decision: MCP builds tools at `tools/list` time from `buildCommandMeta`, so a local command the dispatcher holds becomes an MCP tool with no MCP edit
- [ ] `docs/architecture/api/architecture.md` - identity per transport
  → Constraint: each surface hands the dispatcher the caller's identity; the daemon route of a local command is authorized like any builtin
- [ ] `docs/architecture/zefs-format.md` - key registry, "Runtime state through statestore"
  → Constraint: a file the daemon keeps goes through `statestore` under a key registered in `pkg/zefs/keys.go`; `./le arch fs-persistence check` refuses a raw write under the config or state directory (`ai/doors/core.md`)
- [ ] `ai/doors/system.md`, `ai/doors/commands-cli.md`, `ai/doors/api-surfaces.md`, `ai/doors/plugin-infra.md`, `ai/doors/hub-engine.md`, `ai/doors/access-security.md`, `ai/doors/core.md`, `ai/doors/config-yang.md`, `ai/doors/system-plugins.md`, `ai/doors/le-tooling.md`, `ai/doors/forwarding.md`, `ai/doors/bgp-engine.md`
  → Constraint: "A command is declared once and reaches every interface" (`system.md`); the local-data exception it names is removed by this spec, and the door text changes with it
  → Constraint: "A handler answers data, never text", and output goes through `ApplyPipes` (`commands-cli.md`)
  → Constraint: authorization and TACACS+ accounting hook into `Dispatcher.Dispatch` (`access-security.md`), so a command the dispatcher serves gets both with no per-command work
  → Constraint: always-on code never imports a gated surface; the check reaches web and MCP through their tags, and needs `ze_bgp` to see BGP commands (`hub-engine.md`, Traps)
  → Constraint: something built only at startup misses what arrives later (`hub-engine.md`, Traps): plugin commands arrive after the hub builds the admin tree, so the admin merge runs per request, as the web completer's overlay does
  → Constraint: a plugin owns its whole surface, so each owner's YANG node and handler stay in its own folder (`system-plugins.md`); a shipped `ze` never links `internal/le` (`le-tooling.md`)
- [ ] `docs/architecture/api/process-protocol.md`, `docs/architecture/cli/command-namespacing.md` - declared by `// Design:` headers of files this spec changes
  → Constraint: a plugin's Stage 1 command declaration is wire (`process-protocol.md`); this spec changes no field of it
  → Constraint: `--flag` is offline tooling only and never reaches the YANG layer or the daemon (`command-namespacing.md`), so a flag a local handler parses (`--output`, `--json`, `--strip-private`) becomes a declared leaf on the daemon route or is refused there
- [ ] `docs/architecture/core-design.md`, `docs/architecture/hub-architecture.md`, `ai/rules/plugins.md` - declared by `// Design:` headers of the hub and command files this spec changes
  → Constraint: registration-based discovery and YANG for every RPC (`ai/rules/plugins.md`): a local command the daemon serves needs a `ze:command` node
  → Decision: these pages change only if the predicate or the local-to-daemon bridge becomes part of the documented command engine (`core-design.md`, "Command engine and composition roots"), which the implementing session checks against its anchors

**Key insights:**
- The surfaces are thin adapters; the fix belongs at the producer of each surface's tree and at the dispatcher's builtin load, not in a per-surface exception list.
- The local handlers already take `command.ValidatedArgs`, the same value the daemon's validator produces, so one handler can sit behind both routes.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/server/command.go` - `LoadBuiltins`, `loadBuiltinsWithAliases`: register only wire methods with both a Go handler and a YANG path; `IsReadOnlyPath` sets `ReadOnly` from the verb; `isAuthorized` (called before execution) consults the caller's authorizer
- [ ] `internal/component/plugin/server/command_registry.go` - `VisibleCommandEntries`: non-hidden plugin commands for completion trees
- [ ] `internal/component/plugin/server/handler.go` - `RegisterStreamingHandler`, `IsStreamingCommand`, `StreamingPrefixes`
- [ ] `internal/component/command/local.go` - `LocalHandler` returns only an exit code and prints itself; offline fallbacks, `ListLocal`, `HasLocal`, `InvokeLocal`
- [ ] `internal/component/command/local_data.go` - `LocalDataHandler` returns a payload and an exit code; its contract says a handler with nothing to say writes its reason to stderr itself; `RegisterLocalData` also registers a plain local handler; `ServeLocal`, `ValidateModelArgs`
- [ ] `internal/component/command/node.go` - `MergeCommandPaths`
- [ ] `cmd/ze/hub/main_servers.go` - `serverDispatcher`: builds the `CommandContext` (Username, RemoteAddr, Surface, Authorizer) every surface dispatches through
- [ ] `cmd/ze/hub/command_meta.go` - `buildCommandMeta`: dispatcher commands plus non-hidden plugin commands, the list behind MCP `tools/list`, REST and gRPC command lists
- [ ] `cmd/ze/hub/session_factory.go` - `buildCommandTree` (raw YANG) and `mergePluginCommands` for SSH
- [ ] `cmd/ze/hub/service_web.go` - admin handlers built from the raw YANG tree; web terminal completer with plugin overlay
- [ ] `internal/component/web/handler_admin.go` - `HandleAdminView`, `HandleAdminExecute`: nav from the tree, execution through the dispatcher
- [ ] `internal/component/cli/client/main.go` - `buildRuntimeTreeFromDispatch`, `injectPluginCommands`, `ServeLocal` callers, `emitLocalResult`
- [ ] `internal/component/cli/view_registry.go`, `register_view_dashboard.go`, `register_view_ping.go`, `register_view_traceroute.go` - the TUI live views
- [ ] `internal/component/authz/authz.go`, `register.go` - profiles, run and edit sections, prefix and regex entries over the canonical command text
- [ ] `internal/le/cli/catalog/catalog.go` - `Collect`: one inventory, mode labels
- [ ] The handler of every local command, listed in "Handlers that cannot run in the daemon unchanged" below

**Behavior to preserve:**
- Every local command keeps answering through `ze <verb>` and `ze cli` with no daemon running; `ze doctor` on a host whose daemon is broken still answers.
- Hidden plugin commands stay off every listing surface and still run when typed in full.
- Offline fallbacks (`show host`, `show crashes`) stay daemon-first.
- `ze:backend` and `ze:os` restrictions keep removing nodes as today.
- MCP tool names, parameters and answer shapes for every command the daemon already serves.
- Authorization: every daemon command, the newly served ones included, passes `Dispatcher.isAuthorized` and accounting before it runs.

**Behavior to change:**
- The daemon serves every local and local-data command through its registered handler (G1, G5), so web admin, SSH, MCP, REST and gRPC can run them.
- Web admin offers non-hidden plugin commands that have no YANG node (G2).
- `ze cli` interactive completion offers the local-data commands it serves (G4).
- The catalog labels a command by its registration kind (G3).
- A new check fails on any surface gap.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Daemon dispatcher builtin load | `internal/component/plugin/server/command.go` `LoadBuiltins`; `Dispatcher` | Holds every builtin RPC with a YANG path | Extends: it also loads each local and local-data registration under its YANG path, wrapping the same handler, so the dispatcher stays the one authority for "the daemon serves this" |
| Local and local-data registries | `internal/component/command/local.go`, `local_data.go` | Process-local handlers; `ServeLocal` answers local data in `ze cli`; `InvokeLocal` for `ze <verb>` | Extends: the handler contract changes so the answer and the refusal are returned values, never writes to the process's stdout or stderr; the local route renders them as today, the daemon route returns them in the response |
| Argument validation | `command.ValidateModelArgs`, `command.ValidateArgs` | One validator for local and daemon routes | Extends: unchanged, the daemon route hands the handler the value the dispatcher's validator produced |
| Per-user authorization and accounting | `internal/component/authz` `Store.AuthorizeWithProfiles` through `StoreAuthorizer` and `boundStoreAuthorizer`; `Dispatcher.isAuthorized`, `beginAccounting`; `serverDispatcher` in `cmd/ze/hub/main_servers.go` | Allows or denies a command for the caller's login profiles by word-boundary prefix or regex over the canonical command text (arguments included), in the run section when `ReadOnly` (from `IsReadOnlyPath`, the verb) and the edit section otherwise; records start and stop | Extends: the newly served commands pass through it unchanged; a command whose daemon form runs a process declares itself not read-only so the edit section judges it (see "Path arguments, stdin and authorization") |
| Runtime state store | `internal/core/statestore`, `pkg/zefs/keys.go`; `show data cat` | The daemon's one store for files it keeps; `show data cat` answers a stored key's bytes | Extends: the daemon's support archive is stored under a registered key and retrieved with `show data cat`, which this spec makes daemon-served |
| Plugin command registry | `command_registry.go` `CommandRegistry`, `VisibleCommandEntries` | Stage 1 commands, hidden ones excluded from completion | Extends: the predicate reads its Hidden flag; web admin gains the merge SSH already does |
| Offline fallback registry | `local.go` `RegisterOfflineFallback` | Second answer when the daemon is down | Does not apply to surface membership: a fallback path is also a daemon command, listed as one. The local route of a newly served command is not turned into a fallback: `ze <verb>` keeps running locally first, as today |
| Streaming handler registry | `plugin/server/handler.go` `RegisterStreamingHandler` | Prefix-matched streaming answers on SSH exec and the TUI | Extends: streaming is a declared surface property; AC-9 judges the one-shot answer |
| CLI live-view registry | `internal/component/cli/view_registry.go` `RegisterView` | Declares the full-screen TUI views | Extends: the TUI-only property is declared on the view registration (Key Design Decisions) |
| `system command list` intersection | `internal/component/cli/client/main.go` `buildRuntimeTreeFromDispatch` | `ze cli` offers only what the daemon lists | Extends: SSH and admin adopt the same answer through the shared predicate instead of a copy |
| `MergeCommandPaths` and the per-request overlay | `internal/component/command/node.go`; `cmd/ze/hub/web_completer.go` | Merges plugin entries into a tree without overwriting, per request on the web terminal | Extends: the admin tree uses the same overlay per request |
| `buildCommandMeta` | `cmd/ze/hub/command_meta.go` | One list for MCP, REST, gRPC | Extends: it reads the dispatcher, which now holds the local commands, so MCP lists them with no MCP edit |
| Command catalog check | `internal/le/cli/catalog/catalog.go` `Collect`, `./le cli catalog check` | One inventory compared with published Markdown | Extends: its mode label derives from the registration kind (G3) |
| Answer shapes | `command.RegisterShape`, `ShapeDoc`, `ShapeTab`, `ParseAnswerShape` (`internal/component/command/declared.go`) | Declares whether an answer is one document or rows | Extends: a text or bytes answer (`show data cat`, `show config cat`, `show yang doc`, `help`) is a declared shape of its own, so pipes refuse it by name; the implementing session adds it to the shape set if no existing shape fits |
| Release surface inventory | `plan/release/validation/spec-release-audit-1-surface-inventory.md` | Design-only inventory of user-visible surfaces | Does not apply: that spec indexes surfaces and changes no code. Its inventory may cite this check as evidence |
| Per-surface tests that exist | `internal/component/mcp` `dispatch_surface_test.go`; `TestRunCompletionTreeCoversEveryCanonicalVerb`; `TestEveryLocalDataRegistrationHasAFunctionalCase`; `./le cli dispatch check`; `./le plugin declarations check` | Each judges one surface or one registry | Does not replace: none walks every command across every surface |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Registrations at `init()`: `RegisterRPCs` plus YANG `ze:command`, `MustRegisterLocal*`, `RegisterStreamingHandler`, `RegisterView`; plugin Stage 1 `CommandDecl` lists at runtime.
- Surface requests: SSH session start and exec, web `/admin/` GET and POST, web `/cli/complete`, MCP `tools/list` and `tools/call`, REST and gRPC command list and execute, `ze cli` interactive start, `ze completion words`, `ze <verb>`.

### Transformation Path
1. `LoadBuiltins` loads, beside the `RegisterRPCs` entries, every local and local-data registration that has a YANG `ze:command` node, under that node's path, with `ReadOnly` from the verb unless the registration declares otherwise.
2. A daemon call of such a path passes `isAuthorized` and accounting, then the dispatcher's validator, then the registered handler with the `ValidatedArgs` value; the handler returns its payload, its exit code and its refusal as values, and the daemon route puts them in the response (payload as `Data`, refusal as the error, a non-zero code as a failed status).
3. The local route (`ze <verb>`, `ServeLocal`) runs the same handler and renders the same values to stdout, stderr and the exit code, as today.
4. One predicate in the command layer answers, for a command path and a surface, whether the surface must offer it, must not, or offers it with a declared qualifier (TUI-only, streaming), and why.
5. Each surface producer (SSH tree, web admin tree, web completer, `buildCommandMeta`, `ze cli` tree) builds its list by asking the predicate.
6. The check enumerates every command from every source, asks every surface producer for its list, and compares each with the predicate's answer.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ Plugin | Stage 1 `CommandDecl` (name, help, Hidden); no wire change | No |
| Daemon ↔ caller | The response of a newly served command carries the payload the local route renders | No |
| Daemon ↔ `ze cli` | `system command list` answer, now including the local commands | No |
| Hub ↔ gated surfaces | web and MCP reached through their build tags; the check runs per tag | No |

### Integration Points
- `internal/component/plugin/server/command.go` `LoadBuiltins`: the local-to-daemon bridge.
- `internal/component/command/local.go`, `local_data.go`: the handler contract.
- `cmd/ze/hub/service_web.go` admin handler construction: tree from the predicate plus a per-request `MergeCommandPaths` overlay.
- `cmd/ze/hub/session_factory.go` `buildCommandTree` and `mergePluginCommands`.
- `cmd/ze/hub/command_meta.go` `buildCommandMeta`.
- `internal/component/cli/client/main.go` `buildRuntimeTreeFromDispatch`.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Newly served commands pass the dispatcher's authorization, accounting and validation like every builtin |
| No unintended coupling (components stay isolated) | Yes | The bridge lives in `plugin/server`, which already imports `command`; the predicate lives in always-on `command`; no surface imports another |
| No duplicated functionality (extends existing, does not recreate) | Yes | One handler behind two entry points; the two doctor implementations and any other same-path pair become one producer (AC-14) |
| No layering | Yes | The local route is not replaced: two entry points call one handler. Nothing old is kept beside a new copy |
| Zero-copy preserved where applicable (refs, not copies) | N-A | No wire path |
| Registration over hardcoding, outbound | Yes | The dispatcher reads the local registries; surface membership derives from registration kind and the view registration; no per-command list |
| Registration over hardcoding, inbound | Yes | Lists searched: `LoadBuiltins`, the admin tree build, SSH `buildCommandTree`, `buildCommandMeta`, `buildRuntimeTreeFromDispatch`, catalog `Collect`, `internal/le/cli/dispatch/resolver.go` (its hand registration of `monitor bgp` derives from the view registry instead). No command name is spelled in any of them |

## Serving local commands from the daemon

### Handlers that cannot run in the daemon unchanged

Read from each handler's code on 2026-11-01. Every row is design work this spec
owns; none is a reason to keep a command off the daemon.

| Commands | What in the handler stops it | Daemon form |
|----------|------------------------------|-------------|
| All 20 local-data handlers (`show env *`, `show plugin *`, `show schema *`, `show yang tree`, `completion`, `show data list`, `registered`, `show config dump`, `diff`, `history`, `list`, `validation`) | The `LocalDataHandler` contract (`local_data.go`) says a handler with nothing to say writes its reason to stderr itself; they do (`dataHistory` and `dataDump` in `internal/component/config/cli/config_data.go`, `withRuntimeStore`, `dataGet` through `writeErr` in `internal/plugins/env/env_data.go`, the `fmt.Fprintln(os.Stderr, ...)` refusals in `internal/component/config/yang/cli/yang_data.go` and `storage/cli/storage_data.go`). In the daemon that text goes to the daemon's stderr and the caller gets an empty failure | The handler returns its refusal as a value; the local route writes it to stderr, the daemon route returns it as the error |
| `show yang doc`, `show data cat`, `show config cat`, `fmt`, `graph` (G1), and the G5 plain handlers | `LocalHandler` returns only an exit code; the answer is printed to `os.Stdout` (`cmdCat` in `storage/cli/main.go` writes the bytes, `formatDocCommand(os.Stdout, ...)` in `yang/cli/main.go`, `Run` of `config/cli`, `support.Run` encodes its manifest to `os.Stdout`, `debug.go` `printShowTable` and its `os.Stdout.WriteString`, `explain`, `skills` and `diag` print) | Each becomes a data handler returning its answer (text and bytes under a declared shape; bytes carried in an encoding the response declares, since a binary blob is not valid JSON text); the local route prints it as today |
| `show config dump`, `validation`, `graph`, `fmt`, `show plugin declarations config`, `show bgp encode`, `doctor config <file>` | Read a file the caller names, and take `-` for stdin through `internal/core/cliio` (`dataDump`, `dataValidate` via `cliio.ReadFile`); `show bgp encode` reads `os.Stdin` when no argument is given (`internal/component/bgp/cli/encode.go`) | On the daemon route a path resolves on the daemon's host inside the confined set below, and `-` or an implicit stdin is refused with a message naming the local `ze` command, since no surface carries a stdin stream to the dispatcher |
| `doctor` (`--json`), `show config dump` (`--strip-private`), `request support archive` (`--module`, `--exclude`, `--since`, `--reason`, `--output`) | The handler parses `--flag` tokens itself (`doctor.go` `Run`, `config_data.go` `dataDump`, `support.go` `Run`); a `--flag` never reaches the YANG layer or the daemon (`command-namespacing.md`) | A flag that selects what is answered becomes a declared leaf of the command's YANG node; a rendering flag (`--json`) has no daemon form because the daemon route answers data and the caller's pipe (`\| json`) renders it, while `ze doctor --json` keeps working locally; `--output` is refused on the daemon route (see support archive) |
| `show plugin declarations config <file>` | `runQuery` (`internal/component/plugin/declarations.go`) runs each external plugin's `run` line from the file through `exec.CommandContext(Shell, "-c", ...)` | In the daemon that is process execution under the daemon's identity. The registration declares the daemon form not read-only, so the authz edit section judges it and the built-in read-only profile (edit default deny) refuses it; the file is confined like any other path |
| `request support archive` | `--output <dir>` names a directory on the caller's host; the archive is written there and the manifest printed (`internal/component/support/support.go` `Run`, `collect`) | The daemon stores the archive through `statestore` under a key registered in `pkg/zefs/keys.go` (a raw file write under the config or state directory is refused by `./le arch fs-persistence check`), answers the manifest with that key, and the caller retrieves it with `show data cat <key>`, daemon-served by this spec. `--output` is refused on the daemon route |
| `update serve` | Blocks in a long-lived HTTP server, toggles pause on `SIGUSR1` and on a file beside the binary (`cmd/ze/update_serve.go` `runUpdateServe`); its comment says it is for build and release infrastructure | The daemon form is a server with a lifecycle under the daemon: a start command answering the listen address, a stop command, and a pause command replacing `SIGUSR1` (a signal sent to the daemon reaches the daemon, not the server). The listener goroutine follows `ai/rules/goroutine-lifecycle.md` |
| `doctor` | Two implementations (Current Behavior, "Local commands already served") | One handler serves `ze doctor` and `show doctor`, carrying the plugin checks and `kernel-capabilities`; the checks answer for the daemon's host, which on an appliance is the same box |
| `monitor ping`, `monitor traceroute` | The local handler loops under `signal.NotifyContext(os.Interrupt)` | Already served by the daemon's own RPC; the local loop stays the no-daemon route. AC-14 judges that both reach one producer |
| `request wireguard keypair` | Answers a private key | The verb `request` is not read-only, so the edit section judges it; the answer carries the key to an authorized caller as it does locally |
| `show interface`, `ping`, `traceroute`, `show version`, `show env *`, `show plugin list` | None: they answer the process or host they run in | On the daemon route they describe the daemon's host and process, which is what a remote caller asks about |

No handler was found that cannot run in the daemon at all.

### Path arguments, stdin and authorization

| Question | Answer from the code | Decision |
|----------|----------------------|----------|
| Does the existing authz scope command access? | Yes. `serverDispatcher` puts the caller's Username, RemoteAddr, Surface and login-bound Authorizer in the `CommandContext`; `Dispatcher.isAuthorized` hands the canonical command (arguments included) to `authz.Store.AuthorizeWithProfiles`, which matches word-boundary prefixes or regexes in the run section when `ReadOnly` and the edit section otherwise | Every newly served command goes through it unchanged, so an operator's profile can allow or deny each one, and accounting records it |
| Does it scope file paths? | No. An entry matches command text; it has no notion of a filesystem location, and the built-in read-only profile allows every `show` | The daemon route confines a path argument to the places the daemon already reads configuration from: the directories `configSearchDirs` returns and the store under `resolve.StoreDir` (the set `show config list` already enumerates). A path outside them is refused with a message naming the local `ze` command that reads any file |
| Stdin (`-`) | No surface passes a stdin stream through `CommandDispatcher` | Refused on the daemon route, by name, with the local command to run instead |
| A command that runs processes | `IsReadOnlyPath` derives read-only from the verb alone | A local registration may declare its daemon form not read-only (`show plugin declarations config`); the declaration is on the registration, not in a list |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every TUI-only command has a YANG node or a view registration, and no plugin without YANG declares a TUI-only command | Views are rendered by the in-process CLI model (`view_registry.go`); a plugin process cannot supply one; `monitor bgp` is the only view with no RPC | A `CommandDecl` field is needed, a `pkg/` contract change | grep of `RegisterView` callers and Stage 1 declarations during implementation | unvalidated |
| A-2 | The daemon serves the G1 and G5 commands through their registered handlers, with the same output as the local route | Thomas: "there is no reason why the deamon could not run them and provide the same output"; "yes design so they can run" | - | Decided at the approval gate | confirmed |
| A-3 | gNMI is not a checked surface | Thomas: listed by mistake; `ai/doors/api-surfaces.md`: gNMI serves the config tree, not commands | - | Decided at the approval gate | confirmed |
| A-4 | `ze completion words` (shell completion) is YANG-only by design and offers local commands, since `ze <verb>` serves them | `docs/architecture/api/commands.md` shell completion section | The predicate needs a shell-surface row that differs | The check's shell-completion arm | unvalidated |
| A-5 | A local answer that carries a secret (an env value, a config secret) is masked the same way on both routes, because the handler is shared | `dataDiff` masks secrets in the handler; env rows come from `envRows` | A daemon caller sees a secret a local caller would not | Reading `envRows` and each handler's secret handling during implementation; a test per secret-bearing command | unvalidated |
| A-6 | Every G5 command can carry a `ze:command` node without changing its local grammar | The G1 commands already carry one beside their local registration | Grammar changes ripple to `ze <verb>` | `./le doc yang-contract command-contract` and `./le cli grammar` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A handler still writes to the process's stdout or stderr, so the daemon caller gets an empty answer and the daemon log gets the text | A daemon-route test of the command sees no payload | A test runs every newly served command through the dispatcher and asserts a payload or a refusal value; the handler contract makes the write impossible to miss |
| R-2 | A path argument reads a file outside the confined set on the daemon host | A daemon-route test with `/etc/passwd` answers | The confinement test per path-taking command |
| R-3 | `show plugin declarations config` forks processes for a read-only caller | The read-only profile test runs it | Declared not read-only; a test under the built-in read-only profile expects refusal |
| R-4 | A handler that blocks (update serve, a long collection) holds a dispatcher slot | A dispatch that never returns | Long-lived work runs as a managed server with start and stop commands; collection honours the request context |
| R-5 | The check builds a dispatcher without the real plugin set and reports false gaps | Gaps on commands that work in a running daemon | Build the dispatcher from the linked registrations under each tag, plus a fixture plugin, as the MCP surface test does |
| R-6 | The check becomes a central list of commands | A literal command path in the check | The check enumerates registries only; a literal path is allowed only in the fixture plugin |
| R-7 | A support archive too large for the store or for one response | Storing or retrieving a real archive fails | The implementing session measures a real archive first; the stored blob and the `show data cat` answer are bounded with a refusal that names the size and the local command |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A local command answers differently through the daemon than locally, a remote caller reads a file or runs a process it should not, or commands vanish from a surface; no routing or config effect |
| How is it reverted? | Single commit revert per phase |
| Who else touches this path? | `spec-declared-commands-without-leaves` (command node arguments, same admin forms); `spec-release-audit-1-surface-inventory` (inventory only) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| MCP `tools/list` then `tools/call` of `show env list` and `doctor` | → | `LoadBuiltins` local bridge, `buildCommandMeta` | `TestMCPServesLocalCommands` (`internal/component/mcp`) and `test/plugin/mcp-local-commands.ci` |
| Web `POST /admin/show/env/list` | → | dispatcher local bridge | `test/web/admin-local-command-runs.wb` |
| SSH exec `show config validation <file>` | → | dispatcher local bridge, path confinement | `test/ui/cli-local-command-daemon-ssh.ci` |
| Web `GET /admin/` with a fixture plugin command lacking YANG | → | per-request `MergeCommandPaths` overlay on the admin tree | `TestAdminTreeMergesVisiblePluginCommands` (`cmd/ze/hub`) |
| `ze cli` interactive completion of `show env` | → | `buildRuntimeTreeFromDispatch` | `TestRuntimeTreeOffersLocalDataCommands` (`internal/component/cli/client`) |
| `ze doctor` with no daemon | → | the local route of the shared handler | `test/ui/doctor-no-daemon.ci` |
| Every surface producer | → | the parity check | `TestEveryCommandOnEverySurface` (`cmd/ze/hub`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | The parity check runs over the linked registrations and a fixture plugin (one hidden command, one visible command with no YANG node) | It asks each surface's own producer (SSH completion and help, web admin nav, web terminal completion, MCP `tools/list`, REST command list, gRPC command list, `ze cli` interactive completion, shell completion) for its command set and compares each with the predicate's answer |
| AC-2 | A command a surface must offer is absent, or a command a surface must not offer is present | The check fails, naming the command path, the surface, the registration kind and the expected answer |
| AC-3 | Run against the tree before the fixes | The check fails on `show env list` for web admin, SSH, MCP and `ze cli` completion (G1, G4), on `doctor` for MCP (G5), and on the fixture plugin command for web admin (G2); the red is recorded in the spec and the commit |
| AC-4 | Each of the 25 G1 commands and the 19 G5 commands, called through the daemon dispatcher with valid arguments | It runs the registered handler and answers the same payload, under `\| json`, as `ze <verb>` with the same arguments on the same host; a test per command compares the two |
| AC-5 | MCP, for every G1 and G5 command | `tools/list` lists it as a tool, and `tools/call` runs it and answers the payload of AC-4 |
| AC-6 | Web admin and SSH, for every G1 and G5 command | The command is offered, and running it answers the payload of AC-4; a `.wb` and a `.ci` test prove it at the real entry point for one local-data, one plain local and one path-taking command |
| AC-7 | A handler refuses (missing argument, unknown key, unreadable store) on the daemon route | The caller receives the refusal text the local route writes to stderr, as the response error; nothing is written to the daemon's stdout or stderr |
| AC-8 | A path argument on the daemon route outside the configuration directories and the store, and `-` for stdin | Refused, naming the local `ze` command that reads any file; inside the confined set the command answers as locally |
| AC-9 | Each streaming command listed on a one-shot surface | The check asserts its one-shot answer carries data, or the command is declared streaming-only and removed from one-shot surfaces; `monitor interface rate` answering only `monitor-configured` is fixed to answer its rates once |
| AC-10 | `./le cli catalog check` | A command is labelled by its registration kind; a local command the daemon serves shows both routes, never `daemon` alone (G3) |
| AC-11 | A new command registers through any source with no edit to the check, a surface or `LoadBuiltins` | The check classifies it from its registration and passes or fails on its own; a new local command with a YANG node is served by the daemon with no further edit |
| AC-12 | `docs/architecture/api/commands.md`, `ai/doors/system.md`, `ai/doors/commands-cli.md` and the `HandleAdminView` comment | State that local and local-data commands are also served by the daemon through the same handler, which surfaces offer each registration kind, and the path and stdin rules, with source anchors |
| AC-13 | `monitor bgp`, the one command with a view and no RPC | It is declared TUI-only on its view registration; the predicate keeps it on SSH and `ze cli` and off MCP, REST, gRPC and web admin; `internal/le/cli/dispatch/resolver.go` derives it from the view registry; an undeclared TUI-only command fails the check |
| AC-14 | Each pair of local and daemon handlers for one question (`doctor` and `show doctor`, `show ping`, `monitor ping`, `show traceroute`, `monitor traceroute`, `show interface`, `show version`) | Both routes reach one producer, and a test compares their answers; `show doctor` gains `kernel-capabilities` and the daemon route of `doctor` carries the plugin checks |
| AC-15 | `request support archive` through MCP | The archive is stored in the daemon's store under a registered key, the answer names the key, and `show data cat <key>` through MCP returns its bytes to the same caller; `--output` is refused on the daemon route |
| AC-16 | `update serve` through the daemon | A start command answers the listen address, the server answers `/version.json`, a pause command toggles the pause, and a stop command ends the listener; no goroutine outlives stop |
| AC-17 | `show plugin declarations config <file>` under the built-in read-only profile through the daemon | Refused by authorization before any process starts; under the admin profile it answers as locally |
| AC-18 | Any local command with no daemon running | `ze <verb>` and `ze cli` answer exactly as before this spec; `ze doctor` on a host with a broken daemon answers |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | An AI connected over MCP runs `doctor` to find why a box misbehaves | MCP `tools/call` → `serverDispatcher` → authz → `LoadBuiltins` bridge → doctor handler → response | `test/plugin/mcp-local-commands.ci` |
| 2 | An operator in the web admin runs `show env list` | `/admin/` POST → dispatcher → env data handler → rendered payload | `test/web/admin-local-command-runs.wb` |
| 3 | An operator over SSH validates a config file in the config directory | SSH exec → dispatcher → path confinement → `dataValidate` | `test/ui/cli-local-command-daemon-ssh.ci` |
| 4 | An AI collects a support archive and retrieves it | MCP `request support archive` → archive stored under a registered key → MCP `show data cat <key>` answers its bytes | `TestMCPSupportArchiveRoundTrip` (`internal/component/mcp`) |
| 5 | An operator runs `ze doctor` when the daemon will not start | `ze doctor` → `InvokeLocal` → the shared handler, local route | `test/ui/doctor-no-daemon.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSurfacePredicateByKind` | `internal/component/command/surface_test.go` | Each registration kind maps to the expected surface set, hidden and TUI-only included | |
| `TestLoadBuiltinsServesLocalCommands` | `internal/component/plugin/server/command_test.go` | AC-4, AC-11: every local registration with a YANG node is in the dispatcher | |
| `TestLocalCommandDaemonMatchesLocal` | `cmd/ze/hub/local_bridge_test.go` (tags `ze_core`, `ze_bgp`) | AC-4: each G1 and G5 command answers the same payload on both routes | |
| `TestLocalCommandRefusalReturned` | `cmd/ze/hub/local_bridge_test.go` | AC-7 | |
| `TestLocalCommandPathConfined` | `cmd/ze/hub/local_bridge_test.go` | AC-8 | |
| `TestDeclarationsConfigRefusedReadOnly` | `cmd/ze/hub/local_bridge_test.go` | AC-17 | |
| `TestEveryCommandOnEverySurface` | `cmd/ze/hub/surface_parity_test.go` (tags `ze_core`, `ze_bgp` plus each surface tag) | AC-1, AC-2, AC-3, AC-9, AC-11, AC-13 | |
| `TestMCPServesLocalCommands` | `internal/component/mcp/local_commands_test.go` | AC-5 | |
| `TestMCPSupportArchiveRoundTrip` | `internal/component/mcp/local_commands_test.go` | AC-15 | |
| `TestDoctorOneProducer` | `internal/component/doctor/cmd/show_test.go` | AC-14 for doctor | |
| `TestSameQuestionPairsOneProducer` | `cmd/ze/hub/local_bridge_test.go` | AC-14 for ping, traceroute, interface, version | |
| `TestUpdateServeLifecycle` | `cmd/ze/update_serve_test.go` | AC-16 | |
| `TestAdminTreeMergesVisiblePluginCommands` | `cmd/ze/hub/service_web_test.go` | G2 | |
| `TestRuntimeTreeOffersLocalDataCommands` | `internal/component/cli/client/main_test.go` | G4 | |
| `TestCollectLabelsLocalByKind` | `internal/le/cli/catalog/catalog_test.go` | AC-10 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | The feature takes no numeric input of its own; each served command keeps its own validated leaves | N-A | N-A | N-A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `mcp-local-commands` | `test/plugin/mcp-local-commands.ci` | An MCP client lists and runs `show env list`, `doctor` and `show config validation` against a running daemon | |
| `admin-local-command-runs` | `test/web/admin-local-command-runs.wb` | The admin console offers and runs `show env list` | |
| `admin-plugin-command-no-yang` | `test/web/admin-plugin-command-no-yang.wb` | A plugin command without YANG is offered and runs from the admin console | |
| `cli-local-command-daemon-ssh` | `test/ui/cli-local-command-daemon-ssh.ci` | SSH completion offers `show env list`, and SSH exec of `show config validation` on a file in the config directory answers; a path outside is refused | |
| `doctor-no-daemon` | `test/ui/doctor-no-daemon.ci` | `ze doctor` answers with no daemon running | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | N-A | N-A | Scope is cli; no wire-visible change | |

## Files to Modify
- `internal/component/command/local.go`, `local_data.go` - handler contract returns answer and refusal as values; daemon form read-only declaration
- `internal/component/plugin/server/command.go` - `LoadBuiltins` loads local registrations under their YANG paths
- Every local handler named in "Handlers that cannot run in the daemon unchanged": `internal/component/config/cli/config_data.go`, `register.go`; `internal/component/config/yang/cli/`; `internal/component/config/storage/cli/`; `internal/component/config/schema/cli/`; `internal/plugins/env/`; `internal/component/plugin/register.go`, `declarations.go`; `internal/component/doctor/`, `doctor/cmd/show.go`; `internal/component/support/support.go`; `internal/plugins/debug/`; `internal/plugins/skills/`; `internal/plugins/explain/`; `internal/plugins/diag/`; `internal/component/bgp/cli/`; `internal/component/ping/cmd/`, `traceroute/cmd/`, `iface/cli/`; `cmd/ze/ze_core_dispatch.go`, `cmd/ze/update_serve.go`
- `pkg/zefs/keys.go` - the support archive key
- YANG `-cmd` modules of the G5 owners - a `ze:command` node for each G5 command
- `cmd/ze/hub/service_web.go` - admin tree from the predicate plus per-request plugin overlay; comment corrected
- `cmd/ze/hub/session_factory.go` - SSH tree from the predicate
- `cmd/ze/hub/command_meta.go` - MCP, REST and gRPC list through the predicate
- `internal/component/web/handler_admin.go` - admin handlers read the per-request tree
- `internal/component/cli/client/main.go` - `ze cli` tree adds local entries
- `internal/component/cli/view_registry.go`, `register_view_dashboard.go` - TUI-only declaration
- `internal/le/cli/dispatch/resolver.go` - `monitor bgp` from the view registry
- `internal/le/cli/catalog/catalog.go` - mode label by registration kind
- `docs/architecture/api/commands.md`, `docs/architecture/cli/command-completion.md`, `docs/architecture/web-interface.md`, `ai/doors/system.md`, `ai/doors/commands-cli.md`

## Files to Create
- `internal/component/command/surface.go` - the surface predicate
- `cmd/ze/hub/surface_parity_test.go` - the parity check
- `cmd/ze/hub/local_bridge_test.go` - daemon and local route comparisons
- `internal/component/mcp/local_commands_test.go`
- `test/plugin/mcp-local-commands.ci`, `test/web/admin-local-command-runs.wb`, `test/web/admin-plugin-command-no-yang.wb`, `test/ui/cli-local-command-daemon-ssh.ci`, `test/ui/doctor-no-daemon.ci`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | A `ze:command` node for each G5 command; update serve start, stop and pause nodes |
| YANG validation constraints | Yes | The arguments of the new nodes are declared leaves |
| YANG custom validators | N-A | No custom validator; path confinement is the daemon route's rule |
| CLI commands/flags | Yes | Update serve start, stop, pause |
| CLI grammar (keyword before value) | Yes | The new commands pass `./le cli grammar` |
| Editor autocomplete | Yes | SSH and `ze cli` trees change |
| Functional test for new RPC/API | Yes | `mcp-local-commands.ci`, `.wb` and `.ci` above |
| Pipe completeness | Yes | Every newly served answer reaches `ApplyPipes`; text and bytes answers declare their shape |
| Env var registration | N-A | None |
| Doctor check for runtime dependencies | N-A | No new runtime dependency |
| Prometheus counters/metrics | N-A | None |
| BGP family surface (new SAFI / capability / attribute) | N-A | None |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | The MCP guide page: the local commands are tools; `docs/guide/command-reference.md` |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md`: local commands run on every surface; update serve lifecycle commands; support archive storage |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` "Local-data commands (answered in the caller's own process)": they are also served by the daemon through the same handler, with the path, stdin and authorization rules; "Offline fallback" contrasted |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | The admin console guide page, if it lists commands |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | Maybe | Only if A-1 breaks |
| 9 | RFC behavior implemented, changed, or newly proven? | No | |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` if the check gets a `./le` command |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/cli/command-completion.md`, `docs/architecture/web-interface.md`, `ai/doors/system.md` (the local-data exception goes), `ai/doors/commands-cli.md` ("A command is served in one of three places") |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | Command catalog labels (G3), the new commands, the support archive zefs key (`docs/architecture/zefs-format.md` key registry) |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Derived by `./le spec citation anchors spec plan/release/fixes/spec-command-surface-parity.md` at implementation |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | Verify admin console, MCP and completion examples |

## Implementation Steps

1. **Phase: Wiring and red check** -- write `TestEveryCommandOnEverySurface` and the fixture plugin; run it under `./le job run` with `ze_core`, `ze_bgp` and each surface tag; record the red on G1, G2, G4 and G5 (AC-3)
   - Files: `cmd/ze/hub/surface_parity_test.go`
2. **Phase: Handler contract** -- local and local-data handlers return answer and refusal as values; the local routes render them exactly as today (AC-7, AC-18); convert every handler in the table, one owner per commit
   - Tests: existing local functional tests stay green; `TestLocalCommandRefusalReturned`
3. **Phase: Daemon bridge** -- `LoadBuiltins` serves local registrations; G5 YANG nodes; path confinement and stdin refusal; read-only declaration for `show plugin declarations config` (AC-4, AC-8, AC-11, AC-17)
   - Tests: `TestLoadBuiltinsServesLocalCommands`, `TestLocalCommandDaemonMatchesLocal`, `TestLocalCommandPathConfined`, `TestDeclarationsConfigRefusedReadOnly`
4. **Phase: One producer per question** -- doctor, ping, traceroute, interface, version pairs (AC-14)
5. **Phase: Daemon forms with storage or a lifecycle** -- support archive in the store and retrieval through `show data cat`, update serve start, stop and pause (AC-15, AC-16)
6. **Phase: Predicate and surface producers** -- `internal/component/command/surface.go`; SSH, admin (per-request overlay), `buildCommandMeta`, `ze cli` tree read it; `monitor bgp` TUI-only on its view (AC-1, AC-2, AC-5, AC-6, AC-13)
   - Verify: `TestEveryCommandOnEverySurface` turns green with no exception list
7. **Phase: Streaming one-shot answers (AC-9)** and catalog labels (AC-10)
8. **Phase: Docs** -- pages, doors and the `HandleAdminView` comment (AC-12), each in the same commit as the code it describes

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Each surface's real producer is the one the check asks, not a copy; every G1 and G5 command runs through MCP |
| Correctness | Both routes of a command answer the same payload; no surface offers a command its dispatcher refuses; hidden commands still run when typed |
| Security | Path confinement, stdin refusal, read-only declaration and authz on every newly served command |
| Naming | The kind names match `commands.md` vocabulary (local data, offline fallback, streaming) |
| Data flow | Surfaces call the predicate; the predicate and `LoadBuiltins` read registries; no command path literal outside the fixture |
| Rule: principles | No central enumeration of commands; a new command needs no edit to the check or the bridge |
| Rule: no-layering | The local route and the daemon route call one handler; no second copy of any handler |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Parity check | `./le job run label surfaces command go test -tags ze_core,ze_bgp,ze_web,ze_mcp,ze_ssh,ze_rest,ze_grpc -run TestEveryCommandOnEverySurface ./cmd/ze/hub/` |
| Daemon serves local commands | `./le job run label bridge command go test -tags ze_core,ze_bgp -run TestLocalCommand ./cmd/ze/hub/` |
| MCP reach | `./le test functional plugin mcp-local-commands` |
| Admin and SSH | `./le test functional web admin-local-command-runs`, `./le test functional ui cli-local-command-daemon-ssh` |
| Catalog labels | `./le cli catalog check` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Hidden commands | A hidden plugin command must not appear on any listing surface after the admin merge |
| Authorization | Every newly served command passes `Dispatcher.isAuthorized` and accounting; the admin merge must not bypass `RequireEditAuthz` |
| File access | No daemon-route path argument reads outside the configuration directories and the store |
| Process execution | `show plugin declarations config` is refused under read-only authority |
| Secrets | A secret masked locally is masked on the daemon route (A-5); the wireguard private key reaches only an edit-authorized caller |

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

## Goal Validation

| Goal | Evidence owed |
|------|---------------|
| A remote AI can debug the box through the daemon | `mcp-local-commands.ci` running `doctor`, `show env list` and `show config validation` over MCP against a running daemon |
| The daemon gives the same output as the local command | `TestLocalCommandDaemonMatchesLocal` over every G1 and G5 command |
| The local route still works with no daemon | `doctor-no-daemon.ci` and the existing local functional tests green |
| Every command appears on every surface it must, and on no surface that cannot serve it | `TestEveryCommandOnEverySurface` green, with the recorded red of AC-3 before the fixes |
| G2 fixed for the operator | `admin-plugin-command-no-yang.wb` passing |
| A gap fails the check | The recorded red of AC-3 and a fixture command added without its declaration failing AC-13 |

## Design Insights

- The local handlers already take the validator's `ValidatedArgs` value, the same one the daemon dispatcher builds, so serving them from the daemon costs a return contract, not a rewrite.
- The `ze cli` client already intersects its tree with what the daemon serves; once the daemon serves the local commands, that intersection includes them.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `inert-registration-offered-as-a-choice.md` | 2026-11-01 | G1: web admin and SSH offer 26 local commands the daemon cannot serve |
| `unwired-feature.md` | 2026-11-01 | G2: web admin never merges plugin commands without YANG |
| `inert-registration-offered-as-a-choice.md` | 2026-11-01 | G4: `ze cli` completion omits local-data commands it serves |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The daemon serves every local and local-data command through the same registered handler, loaded by `LoadBuiltins` | Remove them from daemon surfaces (rejected by Thomas: "there is no reason why the deamon could not run them and provide the same output"); a second daemon implementation per command | One handler behind two entry points keeps the outputs identical and adds no copy; a remote AI needs `doctor` and the rest (Thomas) |
| Handlers return answer, refusal and code as values | Capture the process's stdout and stderr around a daemon call | Capture is process-global and races with concurrent dispatches; returned values are the contract `commands-cli.md` already asks for ("a handler answers data, never text") |
| Path arguments confined on the daemon route to the configuration directories and the store; stdin refused | Rely on authz regex rules; allow any path | Authz matches command text and has no filesystem notion, and the built-in read-only profile allows every `show`; the confined set is the one `show config list` already enumerates |
| A local registration may declare its daemon form not read-only | A list of privileged commands in the dispatcher | A list is a central enumeration; the owner declares it on its registration |
| The daemon's support archive lives in the store and is retrieved with `show data cat` | Write it to a directory and add a download command | The store is the daemon's one place for files it keeps (`ai/doors/core.md`), and `show data cat` already answers a key's bytes, so no new retrieval command is needed |
| `monitor bgp` declared TUI-only on its view registration | A YANG extension; a `RegisterOptions` flag | Decided from the code: it is the only view with no RPC, its view registration already names it, and the `ping` and `traceroute` views have one-shot RPCs, so they are not TUI-only. A YANG extension would be a second declaration; a `RegisterOptions` flag covers builtins only |
| gNMI is not a checked surface | Record it as offering the empty set | Thomas: it was listed by mistake; gNMI serves the config tree and no command |
| The check is a Go test in `cmd/ze/hub` under each surface tag | A `./le` tool reading source | Only the hub links every surface producer; asking the real producers is the point. A `./le cli surfaces check` may wrap the same function |

## Known Limitations

- The check covers command surfaces only. gNMI serves the config tree and no command, and the web editor edits config; each is judged by its own tests.
- On the daemon route, a command that describes a host answers for the daemon's host; on an appliance that is the same box, and from a remote client it is the router, which is the question a remote caller asks.

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
- [ ] The targeted tests for what changed pass: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches, and `./le verify changed` over the closing commits (`/ze-close` step 6f). The full verification is Thomas's release check: no agent runs it, and one that is owed is handed to him as its exact command. A red there gets a fix commit
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
- [ ] **Commit B:** `remove plan/release/fixes/spec-command-surface-parity.md` only, in the same `./le commit create` script (commit A preserves the spec in history)
