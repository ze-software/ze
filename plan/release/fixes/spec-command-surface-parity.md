# Spec: command-surface-parity

| Field | Value |
|-------|-------|
| Status | design |
| Scope | cli |
| Priority | P1 |
| Release reason | Shipped operator surfaces offer commands they cannot serve (web admin, SSH completion) and omit commands they do serve; this fixes those product defects and adds the per-surface check that keeps them fixed. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-01 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Thomas approved, in his words, "a check that every command appears on every
surface: each command is checked for CLI help and completion, the web UI, MCP
and gNMI, and a gap fails the check". This spec goes first among the specs
proposed with it because it fixes product defects; it is implemented in its own
later session, which takes it through the `/ze-spec` approval gate (Thomas
approved the direction, not this text). The idea came from the BAMA skill set's
menu audit, where one loop walks every command and checks its menu title,
shortcut and icon.

Ze today has no single answer to "on which surfaces must this command appear".
Each surface builds its command list from a different subset of the command
sources, so surfaces disagree, and nothing compares them. The disagreement is
already user-visible:

| Id | Defect | Evidence (verified 2026-11-01 against source) |
|----|--------|-----------------------------------------------|
| G1 | 26 commands answered only in the caller's own process carry a YANG `ze:command` node with no daemon handler. The web admin nav and form, and SSH completion, are built from the raw YANG tree, so they offer these commands; running one dispatches to the daemon, which never registered it, and fails | `LoadBuiltins` and `loadBuiltinsWithAliases` (`internal/component/plugin/server/command.go`) register only wire methods a Go `RegisterRPCs` entry carries. No Go registration exists for the wire methods of: `show env list`, `get`, `registered` (`internal/plugins/env/yang/ze-env-cmd.yang`); `show plugin list`, `declarations`, `declarations config` (`internal/component/plugin/yang/ze-plugin-show-cmd.yang`); `show schema list`, `methods`, `events`, `handlers`, `protocol` (`internal/plugins/config-schema/yang/ze-schema-cli-cmd.yang`); `show yang tree`, `completion`, `doc` (`internal/plugins/config-yang/yang/ze-yang-cli-cmd.yang`); `show data list`, `cat`, `registered` (`internal/plugins/config-storage/yang/ze-storage-cli-cmd.yang`); `show config dump`, `diff`, `history`, `list`, `cat`, `fmt`, `graph`, `validation` (`internal/plugins/config-cli/yang/ze-config-cli-cmd.yang`). `cmd/ze/hub/service_web.go` passes `yangloader.BuildCommandTree(resolved)` to `HandleAdminView` and `HandleAdminExecute`; `cmd/ze/hub/session_factory.go` `buildCommandTree` starts SSH completion from `yang.BuildCommandTree`. No daemon-side surface calls `ServeLocal`, `LookupLocal` or `InvokeLocal` |
| G2 | A plugin command declared at Stage 1 with no YANG node never reaches the web admin nav, although the comment above `HandleAdminView` says plugin-contributed commands appear there | `cmd/ze/hub/service_web.go` builds the admin handlers from the YANG tree alone, once, at service start; `VisibleCommandEntries` is merged by SSH (`mergePluginCommands`, per session), the web terminal completer (`web_completer.go`, per request) and the `ze cli` client (`injectPluginCommands`), never by the admin handlers |
| G4 | `ze cli` serves the 20 local-data commands in process (`ServeLocal`, `internal/component/cli/client/main.go`, both `-c` and the interactive executor), but its interactive completion tree omits them | `buildRuntimeTreeFromDispatch` builds the tree from `AllBuiltinRPCs` filtered by `system command list`, plus plugin commands; no local registration feeds it |
| G3 | `./le cli catalog check` labels each local-data command that has a YANG node `daemon` or `read-only` | `Collect` (`internal/le/cli/catalog/catalog.go`) walks `cli.WireToPaths` first and assigns `daemon`/`read-only` by `IsReadOnlyPath`; the later `ListLocal` loop skips the path as already seen. A documentation accuracy defect of the published catalog |

The goal: one declaration answers, for every command, which surfaces must
offer it; every surface's own producer derives its list from that answer; and a
check asks each surface's real producer and fails, naming the command and the
surface, on any gap or any offer the surface cannot serve.

### Framing checked against the code

The brief's framing came from a prior agent's research. Where the code
disagreed, this spec records what the code says:

| Brief's claim | What the code says |
|---------------|--------------------|
| Four command sources | Five. Besides builtin RPCs, plugin Stage 1 declarations, local-data commands and streaming handlers, there are plain local commands (`command.MustRegisterLocal`, `MustRegisterLocalMeta`, 30 calls) served only by `ze <verb>`, not by `ze cli` (`ServeLocal` answers local-data paths only). `show yang doc`, `show data cat`, `show config cat`, `fmt`, `graph` are this kind, and all five have YANG nodes, so they are part of G1 |
| 28 production local-data registrations | 20 in `ze` (`command.MustRegisterLocalData`), plus one in `le` (`internal/le/le/root/leroot.go`) |
| G1 covers show env, plugin, schema, yang, data | It also covers `show config dump`, `diff`, `history`, `list`, `validation` (local data) and the five plain local commands above: 26 commands in all |
| Streaming commands may be listed as MCP tools and fail on one-shot dispatch | All eight streaming commands also register a one-shot builtin RPC, so they are in the dispatcher and in `buildCommandMeta`'s list. They do not fail: `handleMonitorInterfaceRate` (`internal/component/iface/cmd/interface_rate.go`) answers `{"status":"monitor-configured"}` and no data. Whether each one-shot answer is a real answer is AC-9 |
| Interactive or TUI-only is declared nowhere | The CLI live-view registry (`internal/component/cli/view_registry.go` `RegisterView`, `RegisteredViews`) declares the three full-screen views (`monitor bgp` dashboard, `monitor ping`, `monitor traceroute`) from their own `register_view_*.go`. It does not say whether the command is TUI-only, and the dashboard matches by a function, not a path. So a declaration exists to extend; the TUI-only property itself is new |
| gNMI is a surface to check | gNMI serves the YANG config tree, not commands (`ai/doors/api-surfaces.md`, "Two surfaces do not go through the command registry"). Thomas's sentence names it; the check records it as a surface that offers no command, so the answer for gNMI is the empty set by construction, and a test proves gNMI exposes no command path. This is a conflict between the owner's words and the architecture, put to the approval gate |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/commands.md` - where a command is served; "Offline fallback", "Local-data commands", "Monitor Commands"
  → Decision: a local-data command has no daemon side and registers no RPC ("Local-data commands")
  → Constraint: a hidden plugin command never reaches completion, help, MCP or the API list
- [ ] `docs/architecture/cli/command-completion.md` - how each completion tree is built
  → Constraint: help, usage and completion are derived from the owner's registration and schema, never written twice (`ai/doors/commands-cli.md`)
- [ ] `docs/architecture/web-interface.md` - admin console routes
  → Constraint: the admin console is generated from the command tree, with no static map
- [ ] `docs/architecture/mcp/overview.md` - tool derivation from the live command list
  → Decision: MCP builds tools at `tools/list` time from `buildCommandMeta`
- [ ] `ai/doors/system.md`, `ai/doors/commands-cli.md`, `ai/doors/api-surfaces.md`, `ai/doors/plugin-infra.md`, `ai/doors/hub-engine.md`
  → Constraint: "A command is declared once and reaches every interface", the one exception being local data (`system.md`)
  → Constraint: always-on code never imports a gated surface; the check reaches web and MCP through their tags, and needs `ze_bgp` to see BGP commands (`hub-engine.md`, Traps)
  → Constraint: something built only at startup misses what arrives later (`hub-engine.md`, Traps): plugin commands arrive after the hub builds the admin tree, so the admin merge runs per request, as the web completer's overlay does

- [ ] `docs/architecture/core-design.md`, `docs/architecture/hub-architecture.md`, `ai/rules/plugins.md` - declared by `// Design:` headers of the hub and command files this spec changes
  → Constraint: registration-based discovery, YANG for every RPC, and Hidden as the only declared way off completion (`ai/rules/plugins.md`); the hub wires surfaces through gated registration (`hub-architecture.md` covers the plugin model only)
  → Decision: none of these pages changes unless the predicate becomes part of the documented command engine (`core-design.md`, "Command engine and composition roots"), which the implementing session checks against its anchors

**Key insights:**
- The surfaces are thin adapters; the fix belongs at the producer of each surface's tree, not in a per-surface exception list.
- The attached `ze cli` console already intersects its tree with what the daemon serves (`system command list`); SSH and web admin do not.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/server/command.go` - `LoadBuiltins`, `loadBuiltinsWithAliases`: register only wire methods with both a Go handler and a YANG path
- [ ] `internal/component/plugin/server/command_registry.go` - `VisibleCommandEntries`: non-hidden plugin commands for completion trees
- [ ] `internal/component/plugin/server/handler.go` - `RegisterStreamingHandler`, `IsStreamingCommand`, `StreamingPrefixes`
- [ ] `internal/component/command/local.go` - local handlers, offline fallbacks, `ListLocal`, `HasLocal`
- [ ] `internal/component/command/local_data.go` - `RegisterLocalData` (also registers a plain local handler), `ServeLocal`, `HasLocalData`
- [ ] `internal/component/command/node.go` - `MergeCommandPaths`
- [ ] `cmd/ze/hub/command_meta.go` - `buildCommandMeta`: dispatcher commands plus non-hidden plugin commands, the list behind MCP `tools/list`, REST and gRPC command lists
- [ ] `cmd/ze/hub/session_factory.go` - `buildCommandTree` (raw YANG) and `mergePluginCommands` for SSH
- [ ] `cmd/ze/hub/service_web.go` - admin handlers built from the raw YANG tree; web terminal completer with plugin overlay
- [ ] `internal/component/web/handler_admin.go` - `HandleAdminView`, `HandleAdminExecute`: nav from the tree, execution through the dispatcher
- [ ] `internal/component/cli/client/main.go` - `buildRuntimeTreeFromDispatch`, `injectPluginCommands`, `ServeLocal` callers
- [ ] `internal/component/cli/view_registry.go` - `RegisterView`, `RegisteredViews`: the TUI live views
- [ ] `internal/le/cli/catalog/catalog.go` - `Collect`: one inventory, mode labels
- [ ] `internal/component/config/yang/modules/ze-extensions.yang` - `task-support`, `backend`, `os` extensions

**Behavior to preserve:**
- Hidden plugin commands stay off every listing surface and still run when typed in full.
- Offline fallbacks (`show host`, `show crashes`) stay daemon-first.
- `ze:backend` and `ze:os` restrictions keep removing nodes as today.
- Local-data commands keep answering in the caller's process through `ze cli` and `ze <verb>`.
- MCP tool names, parameters and answer shapes for every command the daemon serves.

**Behavior to change:**
- Web admin and SSH stop offering commands the daemon cannot serve (G1).
- Web admin offers non-hidden plugin commands that have no YANG node (G2).
- `ze cli` interactive completion offers the local-data commands it serves (G4).
- The catalog labels a local-data command by its registration kind (G3).
- A new check fails on any surface gap.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Daemon dispatcher command list | `internal/component/plugin/server/command.go` `LoadBuiltins`; `Dispatcher` | Holds every builtin RPC with a YANG path | Extends: it is the authority for "the daemon serves this"; the surface predicate reads it |
| Plugin command registry | `command_registry.go` `CommandRegistry`, `VisibleCommandEntries` | Stage 1 commands, hidden ones excluded from completion | Extends: the predicate reads its Hidden flag; web admin gains the merge SSH already does |
| Local and local-data registries | `internal/component/command/local.go`, `local_data.go` | Process-local handlers; `ServeLocal` answers local data in `ze cli` | Extends: registration kind becomes the declared reason a command is absent from daemon surfaces |
| Offline fallback registry | `local.go` `RegisterOfflineFallback` | Second answer when the daemon is down | Does not apply to surface membership: a fallback path is also a daemon command, listed as one |
| Streaming handler registry | `plugin/server/handler.go` `RegisterStreamingHandler` | Prefix-matched streaming answers on SSH exec and the TUI | Extends: streaming is a declared surface property; AC-9 judges the one-shot answer |
| CLI live-view registry | `internal/component/cli/view_registry.go` `RegisterView` | Declares the full-screen TUI views | Extends (preferred, see Key Design Decisions): the TUI-only property is declared on the view registration rather than in a second place |
| `system command list` intersection | `internal/component/cli/client/main.go` `buildRuntimeTreeFromDispatch` | `ze cli` offers only what the daemon lists | Extends: the pattern SSH and admin adopt, through the shared predicate instead of a copy |
| `MergeCommandPaths` and the per-request overlay | `internal/component/command/node.go`; `cmd/ze/hub/web_completer.go` | Merges plugin entries into a tree without overwriting, per request on the web terminal | Extends: the admin tree uses the same overlay per request |
| `buildCommandMeta` | `cmd/ze/hub/command_meta.go` | One list for MCP, REST, gRPC | Extends: it reads the predicate so these surfaces cannot drift from the others |
| Command catalog check | `internal/le/cli/catalog/catalog.go` `Collect`, `./le cli catalog check` | One inventory compared with published Markdown | Extends: its mode label derives from the registration kind (G3); it stays an inventory, not the parity check |
| Release surface inventory | `plan/release/validation/spec-release-audit-1-surface-inventory.md` | Design-only inventory of user-visible surfaces, no gate, no product change | Does not apply: that spec indexes surfaces for audit and changes no code; this one fixes defects and adds a gate. Its inventory may cite this check as evidence |
| Per-surface tests that exist | `internal/component/mcp` `dispatch_surface_test.go`; `TestRunCompletionTreeCoversEveryCanonicalVerb`; `TestEveryLocalDataRegistrationHasAFunctionalCase`; `./le cli dispatch check`; `./le plugin declarations check` | Each judges one surface or one registry | Does not replace: none walks every command across every surface; the new check is the cross-surface walk |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Registrations at `init()`: `RegisterRPCs` plus YANG `ze:command`, `MustRegisterLocal*`, `RegisterStreamingHandler`, `RegisterView`; plugin Stage 1 `CommandDecl` lists at runtime.
- Surface requests: SSH session start (completion tree), web `/admin/` GET and POST, web `/cli/complete`, MCP `tools/list`, REST and gRPC command list, `ze cli` interactive start, `ze completion words`.

### Transformation Path
1. Each registration is classified by kind: daemon builtin, plugin (visible or hidden), local data, plain local, offline fallback, streaming, TUI view.
2. One predicate in the command layer answers, for a command path and a surface, whether the surface must offer it, must not, or offers it with a declared qualifier, and why.
3. Each surface producer (SSH tree, web admin tree, web completer, `buildCommandMeta`, `ze cli` tree) builds its list by asking the predicate, so removal and merge happen at the producer.
4. The check enumerates every command from every source, asks every surface producer for its list, and compares each with the predicate's answer.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ Plugin | Stage 1 `CommandDecl` (name, help, Hidden); no wire change | No |
| Daemon ↔ `ze cli` | `system command list` answer, unchanged | No |
| Hub ↔ gated surfaces | web and MCP reached through their build tags; the check runs per tag | No |

### Integration Points
- `cmd/ze/hub/service_web.go` admin handler construction: tree from the predicate plus a per-request `MergeCommandPaths` overlay.
- `cmd/ze/hub/session_factory.go` `buildCommandTree` and `mergePluginCommands`.
- `cmd/ze/hub/command_meta.go` `buildCommandMeta`.
- `internal/component/cli/client/main.go` `buildRuntimeTreeFromDispatch`.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Surfaces keep dispatching through `serverDispatcher`; only the listing changes |
| No unintended coupling (components stay isolated) | Yes | The predicate lives in always-on command infrastructure; surfaces call it; no surface imports another |
| No duplicated functionality (extends existing, does not recreate) | Yes | Extends the registries, `MergeCommandPaths` and the `ze cli` intersection pattern (Existing Mechanisms) |
| Zero-copy preserved where applicable (refs, not copies) | N-A | No wire path |
| Registration over hardcoding, outbound | Yes | Surface membership derives from registration kind and the view registration; no per-command list |
| Registration over hardcoding, inbound | Yes | Lists searched: the admin tree build, SSH `buildCommandTree`, `buildCommandMeta`, `buildRuntimeTreeFromDispatch`, catalog `Collect`. Each now derives from the registries above; no command name is spelled in any of them |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every TUI-only command has a YANG node, so a declaration on the view registration or YANG covers it, and no plugin without YANG declares a TUI-only command | Views are rendered by the in-process CLI model (`view_registry.go`); a plugin process cannot supply one | A `CommandDecl` field is needed, a `pkg/` contract change | grep of `RegisterView` callers and Stage 1 declarations during implementation | unvalidated |
| A-2 | The 26 G1 commands should be removed from daemon surfaces, not served by the daemon | `docs/architecture/api/commands.md` "Local-data commands": no daemon side by design; `ai/doors/system.md` names local data as the one exception | If Thomas wants them on web and SSH, the daemon would need to serve them in its own process, which answers the daemon's view, not the operator's | Approval gate of `/ze-spec` | unvalidated |
| A-3 | gNMI exposes no command path | `ai/doors/api-surfaces.md` | gNMI becomes a listed surface the predicate must cover | A test over the gNMI capability and path set | unvalidated |
| A-4 | `ze completion words` (shell completion) is YANG-only by design and offers local commands, since `ze <verb>` serves them | `docs/architecture/api/commands.md` shell completion section | The predicate needs a shell-surface row that differs | The check's shell-completion arm | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Filtering SSH or admin trees by the dispatcher removes a command an operator uses through a path the predicate misclassifies | A `.ci` or `.wb` test that typed it goes red | The check names the command and kind; fix the classification, never add an exception |
| R-2 | The check builds a dispatcher without the real plugin set and reports false gaps | Gaps on commands that work in a running daemon | Build the dispatcher from the linked registrations under each tag, plus a fixture plugin, as the MCP surface test does |
| R-3 | The check becomes a central list of commands | A literal command path in the check | The check enumerates registries only; a literal path is allowed only in the fixture plugin |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Commands vanish from SSH completion or web admin, or appear where they cannot run; no routing or config effect |
| How is it reverted? | Single commit revert per phase |
| Who else touches this path? | `spec-declared-commands-without-leaves` (command node arguments, same admin forms); `spec-release-audit-1-surface-inventory` (inventory only) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Web `GET /admin/show/env` | → | admin tree from the surface predicate | `test/web/admin-local-commands-absent.wb` |
| Web `GET /admin/` with a fixture plugin command lacking YANG | → | per-request `MergeCommandPaths` overlay on the admin tree | `TestAdminTreeMergesVisiblePluginCommands` (`cmd/ze/hub`) |
| SSH tab completion of `show env` | → | SSH tree from the predicate | `test/ui/cli-completion-local-command-absent-ssh.ci` |
| `ze cli` interactive completion of `show env` | → | `buildRuntimeTreeFromDispatch` with local-data entries | `TestRuntimeTreeOffersLocalDataCommands` (`internal/component/cli/client`) |
| Every surface producer | → | the parity check | `TestEveryCommandOnEverySurface` (`cmd/ze/hub`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | The parity check runs over the linked registrations and a fixture plugin (one hidden command, one visible command with no YANG node) | It asks each surface's own producer (SSH completion and help, web admin nav, web terminal completion, MCP `tools/list`, REST command list, gRPC command list, `ze cli` interactive completion, shell completion, gNMI) for its command set and compares each with the predicate's answer |
| AC-2 | A command a surface must offer is absent, or a command a surface must not offer is present | The check fails, naming the command path, the surface, the registration kind and the expected answer |
| AC-3 | Run against the tree before the fixes | The check fails on `show env list` for web admin and SSH (G1), on the fixture plugin command for web admin (G2), and on `show env list` for `ze cli` completion (G4); the red is recorded in the spec and the commit |
| AC-4 | Web admin nav and SSH completion after the fix | None of the 26 G1 commands is offered; a `.wb` and a `.ci` test prove it at the real entry point |
| AC-5 | Web admin nav after the fix, with the fixture plugin declaring after the hub started | The visible command without YANG appears and executes through the dispatcher; the hidden one does not appear |
| AC-6 | `ze cli` interactive completion after the fix | The local-data commands it serves are offered |
| AC-7 | A command is TUI-only | It is declared once, on its existing registration (Key Design Decisions); the predicate removes it from one-shot surfaces (MCP, REST, gRPC, web admin) and keeps it on SSH and `ze cli`; an undeclared TUI-only command fails the check |
| AC-8 | gNMI | The check records gNMI as offering no command and a test proves no command path is exposed there |
| AC-9 | Each streaming command listed on a one-shot surface | The check asserts its one-shot answer carries data, or the command is declared streaming-only and removed from one-shot surfaces; `monitor interface rate` answering only `monitor-configured` is judged here and fixed by whichever declaration the approval gate picks |
| AC-10 | `./le cli catalog check` | A local-data or plain local command is labelled by its registration kind, never `daemon` or `read-only` (G3) |
| AC-11 | A new command registers through any source with no edit to the check or a surface | The check classifies it from its registration and passes or fails on its own |
| AC-12 | `docs/architecture/api/commands.md` and the `HandleAdminView` comment | State which surfaces offer each registration kind, matching the predicate, with source anchors |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSurfacePredicateByKind` | `internal/component/command/surface_test.go` | Each registration kind maps to the expected surface set, hidden and TUI-only included | |
| `TestEveryCommandOnEverySurface` | `cmd/ze/hub/surface_parity_test.go` (tags `ze_core`, `ze_bgp` plus each surface tag) | AC-1, AC-2, AC-3, AC-7, AC-9, AC-11 | |
| `TestAdminTreeMergesVisiblePluginCommands` | `cmd/ze/hub/service_web_test.go` | AC-5 | |
| `TestAdminTreeOmitsLocalCommands` | `cmd/ze/hub/service_web_test.go` | AC-4 at the producer | |
| `TestSSHTreeOmitsLocalCommands` | `cmd/ze/hub/session_factory_test.go` | AC-4 at the producer | |
| `TestRuntimeTreeOffersLocalDataCommands` | `internal/component/cli/client/main_test.go` | AC-6 | |
| `TestGNMIExposesNoCommandPath` | `internal/component/gnmi` | AC-8 | |
| `TestCollectLabelsLocalByKind` | `internal/le/cli/catalog/catalog_test.go` | AC-10 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | The feature takes no numeric input | N-A | N-A | N-A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `admin-local-commands-absent` | `test/web/admin-local-commands-absent.wb` | The admin console offers no `show env` command it cannot run | |
| `admin-plugin-command-no-yang` | `test/web/admin-plugin-command-no-yang.wb` | A plugin command without YANG is offered and runs from the admin console | |
| `cli-completion-local-command-absent-ssh` | `test/ui/cli-completion-local-command-absent-ssh.ci` | SSH tab completion does not offer `show env list` | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | N-A | N-A | Scope is cli; no wire-visible change | |

## Files to Modify
- `cmd/ze/hub/service_web.go` - admin tree from the predicate plus per-request plugin overlay; comment corrected
- `cmd/ze/hub/session_factory.go` - SSH tree from the predicate
- `cmd/ze/hub/command_meta.go` - MCP, REST and gRPC list through the predicate
- `internal/component/web/handler_admin.go` - admin handlers read the per-request tree
- `internal/component/cli/client/main.go` - `ze cli` tree adds local-data entries
- `internal/component/cli/view_registry.go` and `register_view_*.go` - TUI-only declaration (if the approval gate keeps the preferred option)
- `internal/le/cli/catalog/catalog.go` - mode label by registration kind
- `docs/architecture/api/commands.md`, `docs/architecture/cli/command-completion.md`, `docs/architecture/web-interface.md` - surface membership per kind

## Files to Create
- `internal/component/command/surface.go` - the surface predicate
- `cmd/ze/hub/surface_parity_test.go` - the parity check
- `test/web/admin-local-commands-absent.wb`, `test/web/admin-plugin-command-no-yang.wb`
- `test/ui/cli-completion-local-command-absent-ssh.ci`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Maybe | Only if the gate picks a YANG extension for TUI-only over the view registry |
| YANG validation constraints | N-A | No new leaf |
| YANG custom validators | N-A | No new leaf |
| CLI commands/flags | Maybe | An optional `./le cli surfaces check` wrapping the same function |
| CLI grammar (keyword before value) | N-A | No new operator command |
| Editor autocomplete | Yes | SSH and `ze cli` trees change |
| Functional test for new RPC/API | N-A | No new RPC; `.wb` and `.ci` prove the surfaces |
| Pipe completeness | N-A | No new output |
| Env var registration | N-A | None |
| Doctor check for runtime dependencies | N-A | None |
| Prometheus counters/metrics | N-A | None |
| BGP family surface (new SAFI / capability / attribute) | N-A | None |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | Surfaces change membership, no feature level |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md`: where local commands are offered |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md`: surface membership per kind |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | The admin console guide page, if it lists commands |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | Maybe | Only if A-1 breaks |
| 9 | RFC behavior implemented, changed, or newly proven? | No | |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` if the check gets a `./le` command |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/cli/command-completion.md`, `docs/architecture/web-interface.md` |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | Command catalog labels (G3) |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Derived by `./le spec citation anchors spec plan/release/fixes/spec-command-surface-parity.md` at implementation |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | Verify admin console and completion examples |

## Implementation Steps

1. **Phase: Wiring and red check** -- write `TestEveryCommandOnEverySurface` and the fixture plugin; run it under `./le job run` with `ze_core`, `ze_bgp` and each surface tag; record the red on G1, G2 and G4 (AC-3)
   - Tests: `TestEveryCommandOnEverySurface`
   - Files: `cmd/ze/hub/surface_parity_test.go`
   - Verify: the check fails naming the expected commands and surfaces
2. **Phase: Predicate** -- `internal/component/command/surface.go` classifies by registration kind; TUI-only declaration on its existing registration
   - Tests: `TestSurfacePredicateByKind`
   - Verify: tests fail, implement, pass
3. **Phase: Surface producers** -- SSH, admin (per-request overlay), `buildCommandMeta`, `ze cli` tree read the predicate
   - Tests: the producer tests, the `.wb` and `.ci` tests
   - Verify: `TestEveryCommandOnEverySurface` turns green with no exception list
4. **Phase: Streaming one-shot answers (AC-9)** and catalog labels (AC-10)
5. **Phase: Docs** -- pages and the `HandleAdminView` comment (AC-12), in the same commits as the code they describe

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Each surface's real producer is the one the check asks, not a copy |
| Correctness | No surface offers a command its dispatcher refuses; hidden commands still run when typed |
| Naming | The kind names match `commands.md` vocabulary (local data, offline fallback, streaming) |
| Data flow | Surfaces call the predicate; the predicate reads registries; no command path literal outside the fixture |
| Rule: principles | No central enumeration of commands; a new command needs no edit to the check |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Parity check | `./le job run label surfaces command go test -tags ze_core,ze_bgp,ze_web,ze_mcp,ze_ssh,ze_rest,ze_grpc -run TestEveryCommandOnEverySurface ./cmd/ze/hub/` |
| Admin and SSH fixes | `./le test functional web admin-local-commands-absent`, `./le test functional ui cli-completion-local-command-absent-ssh` |
| Catalog labels | `./le cli catalog check` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Hidden commands | A hidden plugin command must not appear on any listing surface after the admin merge |
| Authorization | The admin merge must not bypass `RequireEditAuthz` or per-command authz on execution |

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
| Every command appears on every surface it must, and on no surface that cannot serve it | `TestEveryCommandOnEverySurface` green, with the recorded red of AC-3 before the fixes |
| G1 fixed for the operator | `admin-local-commands-absent.wb` and `cli-completion-local-command-absent-ssh.ci` passing at the real entry points |
| G2 fixed for the operator | `admin-plugin-command-no-yang.wb` passing |
| G4 fixed for the operator | `TestRuntimeTreeOffersLocalDataCommands` over the real `buildRuntimeTreeFromDispatch` |
| A gap fails the check | The recorded red of AC-3 and a fixture command added without its declaration failing AC-7 |

## Design Insights

- The `ze cli` client already does what SSH and the admin console lack: it intersects its tree with what the daemon serves. The defect is that this was done in one producer instead of at a shared answer.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `inert-registration-offered-as-a-choice.md` | 2026-11-01 | G1: web admin and SSH offer 26 local commands the daemon cannot serve |
| `unwired-feature.md` | 2026-11-01 | G2: web admin never merges plugin commands without YANG |
| `inert-registration-offered-as-a-choice.md` | 2026-11-01 | G4: `ze cli` completion omits local-data commands it serves |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| One predicate in `internal/component/command`, read by every surface producer | A per-surface filter in each adapter | Per-surface filters are how the surfaces diverged; the command layer already holds the local registries and is always-on, so every surface can import it |
| Remove G1 commands from daemon surfaces | Serve them in the daemon process | Their answer is the caller's process state by design (`commands.md`); a daemon answer would describe the daemon's binary, a different question. Subject to A-2 at the approval gate |
| TUI-only declared on the existing view registration (preferred) | A YANG extension beside `ze:task-support`; a `RegisterOptions` flag | The view registry already declares the three full-screen views from their own files; a second declaration would be a second copy. A YANG extension would cover every kind with a node and is the fallback if the gate prefers schema-level data; a `RegisterOptions` flag covers builtins only |
| The check is a Go test in `cmd/ze/hub` under each surface tag | A `./le` tool reading source | Only the hub links every surface producer; asking the real producers is the point. A `./le cli surfaces check` may wrap the same function |

## Known Limitations

- The check covers command surfaces only; config-tree surfaces (gNMI, web editor) are recorded as offering no command and are judged by their own tests.

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
