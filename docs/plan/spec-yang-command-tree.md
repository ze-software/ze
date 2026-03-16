# Spec: YANG Command Tree

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/5 |
| Updated | 2026-03-16 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/learned/394-yang-analysis.md` - sibling spec learned summary (analysis tool)
4. `internal/component/config/yang/modules/ze-extensions.yang` - current extensions
5. `internal/component/plugin/server/handler.go` - RPCRegistration struct
6. `internal/component/command/node.go` - current BuildTree
7. `internal/component/cli/model_mode.go` - mode switching + editModeCommands map
8. `internal/component/cli/completer.go` - config completer
9. `internal/component/cli/completer_command.go` - command completer

## Task

Restructure YANG API modules so operational commands are defined as a navigable tree in YANG, eliminating the `CLICommand` string indirection. The command tree is then built by walking YANG entries (same mechanism as config completion), not by splitting strings on spaces.

This is the companion spec to `spec-yang-analysis.md`. The analysis tool (spec-yang-analysis) reports naming collisions on the current tree. This spec restructures the tree so that:
- One YANG walk produces both config and command completions
- Command hierarchy is defined in YANG, not in Go string literals
- Renaming a command = renaming a YANG node (one place, not two)
- The `editModeCommands` hardcoded map is replaced by a YANG extension tag

## Shared Vision with spec-yang-analysis

Both specs work toward: **one YANG tree is the single source of truth for all CLI completions**.

| Spec | Role |
|------|------|
| spec-yang-analysis | Builds tooling to analyze and report on the tree (works on current or restructured architecture) |
| spec-yang-command-tree (this) | Restructures the tree so commands are YANG-native |

The analysis tool (spec-yang-analysis) works on whatever tree exists. After this spec, it walks one unified YANG tree instead of merging two sources.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/yang-config-design.md` - YANG schema system
  -> Decision: goyang loads all node types into `Entry.Dir`. RPCs have `.RPC` field set. `config false` marks operational nodes.
  -> Constraint: goyang has no `ActionEntry` kind. Cannot use YANG 1.1 `action` statement. Must use `config false` containers with custom extension.
- [ ] `docs/architecture/api/commands.md` - current command tree structure
  -> Decision: Commands use target-first syntax. BGP commands strip "bgp " prefix. Peer selector comes after peer keyword.
  -> Constraint: Wire protocol uses `"module:rpc-name"` format. Handler dispatch is by WireMethod. This must be preserved.

### RFC Summaries (MUST for protocol work)
Not applicable -- this is CLI/YANG architecture, not protocol work.

**Key insights:**
- goyang `Entry.Kind` has no `ActionEntry`. Available: `DirectoryEntry`, `LeafEntry`, `NotificationEntry`, `InputEntry`, `OutputEntry`, `CaseEntry`, `ChoiceEntry`, `AnyDataEntry`, `AnyXMLEntry`.
- `config false` containers appear in `Entry.Dir` alongside `config true` containers. The completer can distinguish them via `Entry.Config == TSFalse`.
- Current `editModeCommands` map in `model_mode.go:97-103` hardcodes 15 keywords. These should be derivable from YANG.
- `RPCRegistration` has both `WireMethod` (for dispatch) and `CLICommand` (for tree building). After this spec, `CLICommand` is derived from YANG path. `WireMethod` stays for dispatch.
- `BuildTree()` currently strips `"bgp "` prefix from CLICommand strings. After this spec, the YANG tree already has the right hierarchy -- no prefix stripping needed.
- ~~54 RPCRegistrations currently exist (counted from grep).~~ Superseded: 60 RPCRegistrations across 19 handler files (audited 2026-03-16). 109 RPCs in YANG (includes 20 internal plugin protocol RPCs).

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/server/handler.go` - `RPCRegistration` struct: `WireMethod`, `CLICommand`, `Handler`, `Help`, `ReadOnly`, `RequiresSelector`, `PluginCommand`.
- [ ] `internal/component/command/node.go` - `BuildTree()` takes `[]RPCInfo`, strips "bgp " prefix, splits on spaces, builds `*Node` tree. `RPCInfo` has `CLICommand`, `Help`, `ReadOnly`.
- [ ] `internal/component/cli/model_mode.go` - `editModeCommands` is a hardcoded `map[string]bool` of 15 keywords: set, delete, show, edit, commit, save, discard, compare, rollback, history, load, errors, top, up, who, disconnect. `isEditCommand()` and `isEditCommandWithArgs()` check against this map.
- [ ] `internal/component/cli/model.go:870` - `updateCompletions()` has 4-way switch: edit+run prefix, command+edit-args, command top-level (merge via append), edit mode. No dedup on merge.
- [ ] `internal/component/cli/completer.go` - `Completer` walks `*gyang.Entry.Dir` for config. `mergedRoot()` combines 3 conf modules. `confModules = ["ze-bgp-conf", "ze-hub-conf", "ze-plugin-conf"]`.
- [ ] `internal/component/cli/completer_command.go` - `CommandCompleter` wraps `command.TreeCompleter`, converts `command.Suggestion` to `cli.Completion`.
- [ ] `internal/component/config/yang/modules/ze-extensions.yang` - 6 extensions: `syntax`, `key-type`, `route-attributes`, `allow-unknown-fields`, `sensitive`, `validate`.
- [ ] ~~`internal/component/bgp/schema/ze-bgp-api.yang` - 25 flat RPCs~~ Superseded: 15 YANG API modules with 60 wired RPCRegistrations across per-concern plugins. See full inventory below.

**YANG API Module Inventory (audited 2026-03-16):**

| Module | Path | RPCs | WireMethod prefix | Handler location |
|--------|------|------|-------------------|------------------|
| `ze-system-api` | `internal/core/ipc/schema/` | 11 | `ze-system:` | `plugin/server/system.go` |
| `ze-plugin-api` | `internal/core/ipc/schema/` | 8 | `ze-plugin:` | `plugin/server/session.go`, `plugin_rpc.go` |
| `ze-bgp-api` | `bgp/schema/` | 46 | `ze-bgp:` | Distributed across `cmd/*` plugins |
| `ze-bgp-cmd-peer-api` | `bgp/plugins/cmd/peer/schema/` | 11 | `ze-bgp:` | `cmd/peer/peer.go`, `summary.go` |
| `ze-bgp-cmd-meta-api` | `component/cmd/meta/schema/` | 8 | `ze-bgp:` | `cmd/meta/help.go`, `plugin_config.go` |
| `ze-bgp-cmd-update-api` | `bgp/plugins/cmd/update/schema/` | 2 | `ze-bgp:` | `cmd/update/update_text.go` |
| `ze-bgp-cmd-raw-api` | `bgp/plugins/cmd/raw/schema/` | 1 | `ze-bgp:` | `cmd/raw/raw.go` |
| `ze-bgp-cmd-cache-api` | `component/cmd/cache/schema/` | 1 | `ze-bgp:` | `cmd/cache/cache.go` |
| `ze-bgp-cmd-commit-api` | `component/cmd/commit/schema/` | 1 | `ze-bgp:` | `cmd/commit/commit.go` |
| `ze-bgp-cmd-subscribe-api` | `component/cmd/subscribe/schema/` | 2 | `ze-bgp:` | `cmd/subscribe/subscribe.go` |
| `ze-bgp-cmd-metrics-api` | `component/cmd/metrics/schema/` | 2 | `ze-bgp:` | `cmd/metrics/metrics.go` |
| `ze-bgp-cmd-log-api` | `component/cmd/log/schema/` | 2 | `ze-bgp:` | `cmd/log/log.go` |
| `ze-rib-api` | `bgp/plugins/rib/schema/` | 12 | `ze-rib-api:` | `cmd/rib/rib.go` (forwards to bgp-rib plugin) |
| `ze-route-refresh-api` | `bgp/plugins/route_refresh/schema/` | 4 | `ze-bgp:` | `route_refresh/handler/refresh.go`, `clear_soft.go` |
| `ze-adj-rib-in-api` | `bgp/plugins/adj_rib_in/schema/` | 0 | N/A | Config-only module |
| (no YANG module) | `cli/init.go` | 2 | `ze-editor:` | `cli/init.go` (editor mode switching, no Handler) |

**RPCRegistration Inventory (60 wired, sorted by CLICommand):**

| CLICommand | WireMethod | Handler file | ReadOnly | RequiresSelector | PluginCommand |
|-----------|-----------|-------------|----------|------------------|---------------|
| `bgp cache` | `ze-bgp:cache` | `cmd/cache/cache.go` | No | No | - |
| `bgp command complete` | `ze-bgp:command-complete` | `cmd/meta/help.go` | Yes | No | - |
| `bgp command help` | `ze-bgp:command-help` | `cmd/meta/help.go` | Yes | No | - |
| `bgp command list` | `ze-bgp:command-list` | `cmd/meta/help.go` | Yes | No | - |
| `bgp commit` | `ze-bgp:commit` | `cmd/commit/commit.go` | No | No | - |
| `bgp event list` | `ze-bgp:event-list` | `cmd/meta/help.go` | Yes | No | - |
| `bgp help` | `ze-bgp:help` | `cmd/meta/help.go` | Yes | No | - |
| `bgp log levels` | `ze-bgp:log-levels` | `cmd/log/log.go` | Yes | No | - |
| `bgp log set` | `ze-bgp:log-set` | `cmd/log/log.go` | No | No | - |
| `bgp metrics list` | `ze-bgp:metrics-list` | `cmd/metrics/metrics.go` | Yes | No | - |
| `bgp metrics values` | `ze-bgp:metrics-values` | `cmd/metrics/metrics.go` | Yes | No | - |
| `bgp peer add` | `ze-bgp:peer-add` | `cmd/peer/peer.go` | No | Yes | - |
| `bgp peer borr` | `ze-bgp:peer-borr` | `route_refresh/handler/refresh.go` | No | Yes | - |
| `bgp peer capabilities` | `ze-bgp:peer-capabilities` | `cmd/peer/summary.go` | Yes | No | - |
| `bgp peer clear soft` | `ze-bgp:peer-clear-soft` | `route_refresh/handler/clear_soft.go` | No | Yes | - |
| `bgp peer detail` | `ze-bgp:peer-detail` | `cmd/peer/peer.go` | Yes | No | - |
| `bgp peer eorr` | `ze-bgp:peer-eorr` | `route_refresh/handler/refresh.go` | No | Yes | - |
| `bgp peer list` | `ze-bgp:peer-list` | `cmd/peer/peer.go` | Yes | No | - |
| `bgp peer pause` | `ze-bgp:peer-pause` | `cmd/peer/peer.go` | No | Yes | - |
| `bgp peer plugin session ready` | `ze-plugin:session-peer-ready` | `cmd/peer/session.go` | No | No | - |
| `bgp peer raw` | `ze-bgp:peer-raw` | `cmd/raw/raw.go` | No | Yes | - |
| `bgp peer refresh` | `ze-bgp:peer-refresh` | `route_refresh/handler/refresh.go` | No | Yes | - |
| `bgp peer remove` | `ze-bgp:peer-remove` | `cmd/peer/peer.go` | No | Yes | - |
| `bgp peer resume` | `ze-bgp:peer-resume` | `cmd/peer/peer.go` | No | Yes | - |
| `bgp peer save` | `ze-bgp:peer-save` | `cmd/peer/peer.go` | No | Yes | - |
| `bgp peer statistics` | `ze-bgp:peer-statistics` | `cmd/peer/summary.go` | Yes | No | - |
| `bgp peer teardown` | `ze-bgp:peer-teardown` | `cmd/peer/peer.go` | No | Yes | - |
| `bgp peer update` | `ze-bgp:peer-update` | `cmd/update/update_text.go` | No | Yes | - |
| `bgp plugin ack` | `ze-bgp:plugin-ack` | `cmd/meta/plugin_config.go` | No | No | - |
| `bgp plugin encoding` | `ze-bgp:plugin-encoding` | `cmd/meta/plugin_config.go` | No | No | - |
| `bgp plugin format` | `ze-bgp:plugin-format` | `cmd/meta/plugin_config.go` | No | No | - |
| `bgp rib best` | `ze-rib-api:best` | `cmd/rib/rib.go` | Yes | No | `rib best` |
| `bgp rib best status` | `ze-rib-api:best-status` | `cmd/rib/rib.go` | Yes | No | `rib best status` |
| `bgp rib clear in` | `ze-rib-api:clear-in` | `cmd/rib/rib.go` | No | No | `rib clear in` |
| `bgp rib clear out` | `ze-rib-api:clear-out` | `cmd/rib/rib.go` | No | No | `rib clear out` |
| `bgp rib routes` | `ze-rib-api:routes` | `cmd/rib/rib.go` | Yes | No | `rib show` |
| `bgp rib status` | `ze-rib-api:status` | `cmd/rib/rib.go` | Yes | No | `rib status` |
| `bgp summary` | `ze-bgp:summary` | `cmd/peer/summary.go` | Yes | No | - |
| `daemon reload` | `ze-system:daemon-reload` | `server/system.go` | No | No | - |
| `daemon shutdown` | `ze-system:daemon-shutdown` | `server/system.go` | No | No | - |
| `daemon status` | `ze-system:daemon-status` | `server/system.go` | Yes | No | - |
| `edit` | `ze-editor:mode-edit` | `cli/init.go` | Yes | No | - |
| `plugin command complete` | `ze-plugin:command-complete` | `server/plugin_rpc.go` | Yes | No | - |
| `plugin command help` | `ze-plugin:command-help` | `server/plugin_rpc.go` | Yes | No | - |
| `plugin command list` | `ze-plugin:command-list` | `server/plugin_rpc.go` | Yes | No | - |
| `plugin help` | `ze-plugin:help` | `server/plugin_rpc.go` | Yes | No | - |
| `plugin session bye` | `ze-plugin:session-bye` | `server/session.go` | No | No | - |
| `plugin session ping` | `ze-plugin:session-ping` | `server/session.go` | Yes | No | - |
| `plugin session ready` | `ze-plugin:session-ready` | `server/session.go` | No | No | - |
| `run` | `ze-editor:mode-command` | `cli/init.go` | Yes | No | - |
| `subscribe` | `ze-bgp:subscribe` | `cmd/subscribe/subscribe.go` | No | No | - |
| `system command complete` | `ze-system:command-complete` | `server/system.go` | Yes | No | - |
| `system command help` | `ze-system:command-help` | `server/system.go` | Yes | No | - |
| `system command list` | `ze-system:command-list` | `server/system.go` | Yes | No | - |
| `system dispatch` | `ze-system:dispatch` | `server/system.go` | No | No | - |
| `system help` | `ze-system:help` | `server/system.go` | Yes | No | - |
| `system subsystem list` | `ze-system:subsystem-list` | `server/system.go` | Yes | No | - |
| `system version api` | `ze-system:version-api` | `server/system.go` | Yes | No | - |
| `system version software` | `ze-system:version-software` | `server/system.go` | Yes | No | - |
| `unsubscribe` | `ze-bgp:unsubscribe` | `cmd/subscribe/subscribe.go` | No | No | - |

**Behavior to preserve:**
- Wire protocol `WireMethod` format (`"module:rpc-name"`) used for handler dispatch -- unchanged
- Handler function signatures -- unchanged
- Command execution semantics (ReadOnly, RequiresSelector) -- unchanged
- `run <cmd>` from edit mode executes operational command -- unchanged
- Config commands (set, delete, etc.) from command mode auto-switch to edit mode -- unchanged
- Edit mode shortcuts (commit, save, etc.) are syntactic sugar for `run <cmd>` -- made explicit via YANG tag

**Behavior to change:**
- `CLICommand` string field removed from `RPCRegistration` -- YANG tree is the source of truth for CLI hierarchy
- `BuildTree()` walks YANG entries instead of splitting strings
- Two separate completers merged into one YANG-walking completer with mode awareness
- Per-plugin `-cmd.yang` modules define the command tree hierarchy alongside existing `-api.yang` modules
- `ze-extensions.yang` extended with `ze:command` (with WireMethod handler argument) and `ze:edit-shortcut` (marker, zero current uses)
- ~~`editModeCommands` map derived from YANG~~ Eliminated: all 15 entries are editor-internal, not YANG-derivable
- 6 non-BGP command plugins moved from `bgp/plugins/cmd/` to `component/cmd/`

## Data Flow (MANDATORY)

### Entry Point
- YANG modules loaded at startup via `yang.Loader` (same as today)
- Command tree built from YANG entries instead of from `CLICommand` strings

### Transformation Path

**Current (before this spec):**
1. YANG `-conf` modules loaded -> config `Entry.Dir` tree
2. `RPCRegistration` structs collected from `init()` -> `CLICommand` strings split on spaces -> `command.Node` tree
3. Two separate completers walk two separate trees
4. `updateCompletions()` merges via `append()` in command mode

**After this spec:**
1. YANG `-conf` modules loaded -> config entries in `Entry.Dir`
2. YANG `-cmd` modules loaded -> command entries in `Entry.Dir` (same loader, same tree)
3. One completer walks `Entry.Dir`, using `Entry.Config` and `ze:command` extension to distinguish config nodes from command nodes
4. Mode determines which nodes are offered: edit mode shows `config true` nodes + `ze:edit-shortcut` tagged command nodes; command mode shows `config false` + `ze:command` nodes + config keywords for cross-mode switching
5. `RPCRegistration` still exists for handler dispatch, but `CLICommand` is derived from YANG path

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| YANG loader -> completer | `GetEntry()` returns unified tree from 13 `-cmd` modules | [ ] |
| YANG entry -> WireMethod | Explicit `ze:command "wire-method"` argument on entry (no derivation) | `make ze-validate-commands` (58/58) |
| Completer -> handler dispatch | User input matched against YANG path, `GetCommandExtension()` returns WireMethod, dispatched | [ ] |

### Integration Points
- `internal/component/config/yang/modules/ze-extensions.yang` -- add `ze:command` and `ze:edit-shortcut` extensions
- `internal/component/config/yang.Loader` -- unchanged (already loads all node types)
- `internal/component/cli/completer.go` -- extend to handle `config false` + `ze:command` nodes
- `internal/component/cli/model_mode.go` -- derive `editModeCommands` from YANG instead of hardcoded map
- `internal/component/command/node.go` -- `BuildTree()` rewritten to walk YANG entries
- All `RPCRegistration` call sites -- remove `CLICommand` field

### Architectural Verification
- [ ] No bypassed layers -- uses same YANG loading as config
- [ ] No unintended coupling -- command YANG modules are separate files, loaded via same mechanism
- [ ] No duplicated functionality -- replaces `CLICommand` string splitting with YANG tree walking (same code path as config)
- [ ] Zero-copy preserved -- not applicable (CLI layer, not wire path)

## YANG Extensions

Two new extensions in `ze-extensions.yang`:

| Extension | Argument | Purpose |
|-----------|----------|---------|
| `ze:command` | WireMethod handler (e.g., `"ze-bgp:peer-list"`) | Marks a `config false` container as an executable command and references the handler for dispatch. Eliminates the need for WireMethod derivation conventions -- the reference is explicit. |
| `ze:edit-shortcut` | none | Marks a command node as available in edit mode as a shortcut. When typed in edit mode, implicitly runs as `run <command>`. Replaces the hardcoded `editModeCommands` map. |

Detection in Go: `GetCommandExtension(entry)` returns the WireMethod string (or empty). `HasEditShortcutExtension(entry)` returns bool. Both walk `Entry.Exts`.

~~WireMethod derived from YANG path convention~~ Superseded (2026-03-16): WireMethod is declared explicitly as the `ze:command` argument. Existing WireMethod strings (e.g., `ze-bgp:peer-list`) are preserved unchanged -- no renaming needed.

## YANG Module Restructuring

~~### Current: flat RPCs in `ze-bgp-api.yang`~~ Superseded (2026-03-16).

### Actual current state: 15 per-concern `-api.yang` modules with flat RPCs

Each plugin already has its own YANG API module (e.g., `ze-bgp-cmd-peer-api.yang`). The RPCs within each module are flat (e.g., `peer-list`, `peer-add`). The restructuring adds a parallel `-cmd.yang` module per domain that defines hierarchical `config false` containers matching the CLI tree, while the `-api.yang` modules (with flat RPCs and input/output schemas) are preserved for wire protocol documentation.

### After: per-plugin `-cmd.yang` modules alongside existing `-api.yang` (DONE)

Each plugin gets its own `-cmd.yang` module (proximity principle). The Go tree walker merges overlapping containers (e.g., multiple plugins contribute `peer > ...` nodes). Non-BGP plugins moved from `bgp/plugins/cmd/` to `component/cmd/`.

**13 per-plugin command modules:**

| Module | Plugin location | Commands |
|--------|----------------|----------|
| `ze-peer-cmd` | `bgp/plugins/cmd/peer/schema/` | summary, peer list/detail/add/remove/teardown/pause/resume/save/capabilities/statistics, peer plugin session ready |
| `ze-rib-cmd` | `bgp/plugins/cmd/rib/schema/` | rib status/routes/best/best-status/clear-in/clear-out |
| `ze-refresh-cmd` | `bgp/plugins/route_refresh/schema/` | peer refresh/borr/eorr/clear-soft |
| `ze-raw-cmd` | `bgp/plugins/cmd/raw/schema/` | peer raw |
| `ze-update-cmd` | `bgp/plugins/cmd/update/schema/` | peer update |
| `ze-meta-cmd` | `component/cmd/meta/schema/` | help, command list/help/complete, event list, plugin encoding/format/ack |
| `ze-cache-cmd` | `component/cmd/cache/schema/` | cache |
| `ze-commit-cmd` | `component/cmd/commit/schema/` | commit (+ ze:edit-shortcut) |
| `ze-subscribe-cmd` | `component/cmd/subscribe/schema/` | subscribe, unsubscribe |
| `ze-log-cmd` | `component/cmd/log/schema/` | log levels/set |
| `ze-metrics-cmd` | `component/cmd/metrics/schema/` | metrics values/list |
| `ze-system-cmd` | `core/ipc/schema/` | system help/dispatch/version/subsystem/command, daemon shutdown/status/reload |
| `ze-plugin-cmd` | `core/ipc/schema/` | plugin help/command/session |

~~BGP domain (`ze-bgp-cmd.yang`)~~ Superseded: per-plugin modules replace monolithic file.

**Command-to-YANG mapping (from all plugins):**

| Current CLICommand | New YANG path | Tree depth | Source handler |
|---|---|---|---|
| `bgp help` | `help` | 1 | `cmd/meta/help.go` |
| `bgp summary` | `summary` | 1 | `cmd/peer/summary.go` |
| `bgp commit` | `commit` | 1 | `cmd/commit/commit.go` |
| `bgp cache` | `cache` | 1 | `cmd/cache/cache.go` |
| `bgp command list` | `command > list` | 2 | `cmd/meta/help.go` |
| `bgp command help` | `command > help` | 2 | `cmd/meta/help.go` |
| `bgp command complete` | `command > complete` | 2 | `cmd/meta/help.go` |
| `bgp event list` | `event > list` | 2 | `cmd/meta/help.go` |
| `bgp plugin encoding` | `plugin > encoding` | 2 | `cmd/meta/plugin_config.go` |
| `bgp plugin format` | `plugin > format` | 2 | `cmd/meta/plugin_config.go` |
| `bgp plugin ack` | `plugin > ack` | 2 | `cmd/meta/plugin_config.go` |
| `bgp log levels` | `log > levels` | 2 | `cmd/log/log.go` |
| `bgp log set` | `log > set` | 2 | `cmd/log/log.go` |
| `bgp metrics values` | `metrics > values` | 2 | `cmd/metrics/metrics.go` |
| `bgp metrics list` | `metrics > list` | 2 | `cmd/metrics/metrics.go` |
| `bgp peer list` | `peer > list` | 2 | `cmd/peer/peer.go` |
| `bgp peer detail` | `peer > detail` | 2 | `cmd/peer/peer.go` |
| `bgp peer capabilities` | `peer > capabilities` | 2 | `cmd/peer/summary.go` |
| `bgp peer statistics` | `peer > statistics` | 2 | `cmd/peer/summary.go` |
| `bgp peer add` | `peer > add` | 2 | `cmd/peer/peer.go` |
| `bgp peer remove` | `peer > remove` | 2 | `cmd/peer/peer.go` |
| `bgp peer teardown` | `peer > teardown` | 2 | `cmd/peer/peer.go` |
| `bgp peer pause` | `peer > pause` | 2 | `cmd/peer/peer.go` |
| `bgp peer resume` | `peer > resume` | 2 | `cmd/peer/peer.go` |
| `bgp peer save` | `peer > save` | 2 | `cmd/peer/peer.go` |
| `bgp peer update` | `peer > update` | 2 | `cmd/update/update_text.go` |
| `bgp peer raw` | `peer > raw` | 2 | `cmd/raw/raw.go` |
| `bgp peer refresh` | `peer > refresh` | 2 | `route_refresh/handler/refresh.go` |
| `bgp peer borr` | `peer > borr` | 2 | `route_refresh/handler/refresh.go` |
| `bgp peer eorr` | `peer > eorr` | 2 | `route_refresh/handler/refresh.go` |
| `bgp peer clear soft` | `peer > clear > soft` | 3 | `route_refresh/handler/clear_soft.go` |
| `bgp peer plugin session ready` | `peer > plugin > session > ready` | 4 | `cmd/peer/session.go` |
| `bgp rib status` | `rib > status` | 2 | `cmd/rib/rib.go` |
| `bgp rib routes` | `rib > routes` | 2 | `cmd/rib/rib.go` |
| `bgp rib best` | `rib > best` | 2 | `cmd/rib/rib.go` |
| `bgp rib best status` | `rib > best > status` | 3 | `cmd/rib/rib.go` |
| `bgp rib clear in` | `rib > clear > in` | 3 | `cmd/rib/rib.go` |
| `bgp rib clear out` | `rib > clear > out` | 3 | `cmd/rib/rib.go` |
| `subscribe` | (top-level, no bgp prefix) | 1 | `cmd/subscribe/subscribe.go` |
| `unsubscribe` | (top-level, no bgp prefix) | 1 | `cmd/subscribe/subscribe.go` |

**System domain (`ze-system-cmd.yang` -- new):**

| Current CLICommand | New YANG path | Tree depth |
|---|---|---|
| `system help` | `system > help` | 2 |
| `system version software` | `system > version > software` | 3 |
| `system version api` | `system > version > api` | 3 |
| `system subsystem list` | `system > subsystem > list` | 3 |
| `system command list` | `system > command > list` | 3 |
| `system command help` | `system > command > help` | 3 |
| `system command complete` | `system > command > complete` | 3 |
| `system dispatch` | `system > dispatch` | 2 |
| `daemon shutdown` | `daemon > shutdown` | 2 |
| `daemon status` | `daemon > status` | 2 |
| `daemon reload` | `daemon > reload` | 2 |

**Plugin domain (`ze-plugin-cmd.yang` -- new):**

| Current CLICommand | New YANG path | Tree depth |
|---|---|---|
| `plugin help` | `plugin > help` | 2 |
| `plugin command list` | `plugin > command > list` | 3 |
| `plugin command help` | `plugin > command > help` | 3 |
| `plugin command complete` | `plugin > command > complete` | 3 |
| `plugin session ready` | `plugin > session > ready` | 3 |
| `plugin session ping` | `plugin > session > ping` | 3 |
| `plugin session bye` | `plugin > session > bye` | 3 |

**Top-level commands (in `ze-bgp-cmd.yang` or dedicated module):**

| Current CLICommand | Notes |
|---|---|
| `subscribe` | No domain prefix -- top-level in command tree |
| `unsubscribe` | No domain prefix -- top-level in command tree |

### WireMethod mapping

~~YANG path to WireMethod convention~~ Superseded (2026-03-16): WireMethod is now an explicit argument to `ze:command`, not derived from a naming convention. Existing WireMethod strings are preserved unchanged.

| YANG module | YANG path | ze:command argument (WireMethod) |
|-------------|-----------|----------------------------------|
| `ze-bgp-cmd` | `peer > list` | `ze-bgp:peer-list` |
| `ze-bgp-cmd` | `peer > clear > soft` | `ze-bgp:peer-clear-soft` |
| `ze-bgp-cmd` | `rib > best > status` | `ze-rib-api:best-status` |
| `ze-system-cmd` | `daemon > shutdown` | `ze-system:daemon-shutdown` |
| `ze-system-cmd` | `system > version > software` | `ze-system:version-software` |
| `ze-plugin-cmd` | `plugin > session > ready` | `ze-plugin:session-ready` |

No WireMethod renaming needed. The existing `module:rpc-name` format is preserved verbatim.

### Edit shortcuts

~~Commands tagged with `ze:edit-shortcut` in YANG:~~

~~`commit`, `save`, `rollback`, `load`~~ Superseded (2026-03-16): All 15 commands in the `editModeCommands` map are **editor-internal** commands handled directly by the CLI model (e.g., `cmdCommitConfirmed()`, `cmdLoad()`, `cmdSave()`). They are NOT operational commands dispatched via WireMethod. The `editModeCommands` map controls **mode switching** -- when a command mode user types `commit`, the editor switches to edit mode and executes the editor's internal commit.

**Revised design:** `ze:edit-shortcut` marks operational commands (dispatched via WireMethod) that should also appear in edit mode completions. Currently no operational commands need this -- the `commit` in the YANG tree (`ze-bgp:commit` -- named route commits) is different from the editor's `commit` (config commit).

`ze:edit-shortcut` is defined in YANG but has **zero current uses**. It remains available for future operational commands that should be accessible from edit mode without the `run` prefix. The `editModeCommands` map stays as-is -- it's editor-internal, not YANG-derivable.

**Note:** `ze-commit-cmd.yang` currently has `ze:edit-shortcut` on the `commit` container. This is wrong -- the `ze-bgp:commit` RPC (named route commits) is not the same as the editor's `commit` command. The `ze:edit-shortcut` should be removed from `ze-commit-cmd.yang`.

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Tab completion in command mode | -> | YANG-derived command tree | `test/ui/cli-yang-command-completion.ci` |
| ~~Tab completion in edit mode for shortcuts~~ | ~~->~~ | ~~`ze:edit-shortcut` extension~~ | ~~Eliminated -- editModeCommands is editor-internal~~ |
| `ze yang tree` shows unified tree | -> | Unified YANG walk | `test/ui/cli-yang-tree.ci` (from spec-yang-analysis) |
| Handler dispatch after CLICommand removal | -> | Dispatch still works via WireMethod | `test/plugin/yang-wire-dispatch.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Tab in command mode | Shows command completions from YANG `-cmd` module entries (not from CLICommand strings) |
| AC-2 | `ze:command` on a `config false` container | Completer offers it as an executable command |
| ~~AC-3~~ | ~~`ze:edit-shortcut` on a command~~ | ~~Deferred -- no operational commands currently need edit-mode shortcuts. Extension defined but zero uses.~~ |
| ~~AC-4~~ | ~~`commit` typed in edit mode~~ | ~~Eliminated -- editor `commit` is internal (cmdCommitConfirmed), not the `ze-bgp:commit` RPC~~ |
| AC-5 | `RPCRegistration` struct | `CLICommand` field removed. YANG path is the source of truth. |
| AC-6 | `BuildTree()` or equivalent | Walks YANG `config false` entries instead of splitting CLICommand strings |
| AC-7 | All 58 existing commands (excl. 2 editor-internal) | Still reachable and executable after restructuring. No command lost. `make ze-validate-commands` passes. |
| AC-8 | `ze yang tree` (from spec-yang-analysis) | Shows unified tree with both config and command nodes |
| AC-9 | `ze yang completion` (from spec-yang-analysis) | Finds collisions in the unified tree |
| ~~AC-10~~ | ~~WireMethod format~~ | ~~Eliminated -- WireMethod preserved unchanged via explicit `ze:command` argument~~ |
| ~~AC-11~~ | ~~Handler dispatch after rename~~ | ~~Eliminated -- no rename~~ |
| ~~AC-12~~ | ~~`editModeCommands` map~~ | ~~Eliminated -- all 15 entries are editor-internal commands, not YANG-derivable. Map stays as-is.~~ |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCommandExtension` | `config/yang/command_test.go` | `ze:command` with WireMethod argument parsed from YANG | Done |
| `TestEditShortcutExtension` | `config/yang/command_test.go` | `ze:edit-shortcut` marker parsed from YANG | Done |
| `TestExtensionNilEntry` | `config/yang/command_test.go` | Nil safety for extension functions | Done |
| `TestPeerCmdModule` | `config/yang/command_test.go` | Peer plugin cmd module loads with correct hierarchy + WireMethod refs | Done |
| `TestRibCmdModule` | `config/yang/command_test.go` | RIB plugin cmd module loads | Done |
| `TestRefreshCmdModule` | `config/yang/command_test.go` | Route refresh plugin cmd module loads | Done |
| `TestMetaCmdModule` | `config/yang/command_test.go` | Meta plugin cmd module loads (help, command, event, plugin groups) | Done |
| `TestSimpleCmdModules` | `config/yang/command_test.go` | cache, commit, subscribe modules load | Done |
| `TestCommitEditShortcut` | `config/yang/command_test.go` | commit has ze:edit-shortcut | Done |
| `TestRawCmdModule` | `config/yang/command_test.go` | Raw plugin cmd module loads (peer > raw) | Done |
| `TestUpdateCmdModule` | `config/yang/command_test.go` | Update plugin cmd module loads (peer > update) | Done |
| `TestSystemCmdModuleLoads` | `config/yang/command_test.go` | System cmd module loads (system + daemon groups) | Done |
| `TestPluginCmdModuleLoads` | `config/yang/command_test.go` | Plugin cmd module loads (plugin group) | Done |
| `TestBuildTreeFromYANG` | `command/node_test.go` | Tree built from YANG entries matches expected structure | Phase 2 |
| `TestBuildTreeCommandNodes` | `command/node_test.go` | Only `ze:command`-tagged nodes become executable tree leaves | Phase 2 |
| `TestBuildTreeBranches` | `command/node_test.go` | Non-command containers become navigable branches | Phase 2 |
| `TestEditShortcutsFromYANG` | `cli/completer_test.go` | Edit mode completions include `ze:edit-shortcut` tagged commands | Phase 4 |
| `TestCommandModeCompletionsFromYANG` | `cli/completer_test.go` | Command mode completions come from YANG walk | Phase 5 |
| `TestAllCommandsReachable` | `server/dispatch_test.go` | Every handler reachable after CLICommand removal | Phase 3 |
| `TestNoHardcodedEditModeCommands` | `cli/model_mode_test.go` | No static map -- shortcuts derived from YANG | Phase 4 |

### Boundary Tests (MANDATORY for numeric inputs)
Not applicable -- no numeric inputs in this spec.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `cli-yang-command-completion` | `test/ui/cli-yang-command-completion.ci` | User types `peer ` in command mode, gets YANG-derived completions | |
| `cli-yang-edit-shortcuts` | `test/ui/cli-yang-edit-shortcuts.ci` | User types `commit` in edit mode, it executes as operational command | |
| `yang-wire-dispatch` | `test/plugin/yang-wire-dispatch.ci` | Command dispatches correctly with new WireMethod format | |

### Future (if deferring any tests)
- Dedup logic in merged completions (Phase 3 -- single completer)
- Visual `[cmd]`/`[set]` tags in completion dropdown

## Files to Modify (remaining -- Phase 2-6)

**Core infrastructure (Phase 2-3):**
- `internal/component/plugin/server/handler.go` - remove `CLICommand` from `RPCRegistration`
- `internal/component/command/node.go` - rewrite `BuildTree()` to walk YANG entries

**Handler registrations -- remove CLICommand (Phase 3):**
- `internal/component/plugin/server/system.go` - 11 registrations
- `internal/component/plugin/server/plugin_rpc.go` - 4 registrations
- `internal/component/plugin/server/session.go` - 3 registrations
- `internal/component/cli/init.go` - 2 registrations
- `internal/component/bgp/plugins/cmd/peer/peer.go` - 8 registrations
- `internal/component/bgp/plugins/cmd/peer/summary.go` - 3 registrations
- `internal/component/bgp/plugins/cmd/peer/session.go` - 1 registration
- `internal/component/bgp/plugins/cmd/rib/rib.go` - 6 registrations
- `internal/component/bgp/plugins/cmd/update/update_text.go` - 1 registration
- `internal/component/bgp/plugins/cmd/raw/raw.go` - 1 registration
- `internal/component/bgp/plugins/route_refresh/handler/refresh.go` - 3 registrations
- `internal/component/bgp/plugins/route_refresh/handler/clear_soft.go` - 1 registration
- `internal/component/cmd/meta/help.go` - 5 registrations
- `internal/component/cmd/meta/plugin_config.go` - 3 registrations
- `internal/component/cmd/cache/cache.go` - 1 registration
- `internal/component/cmd/commit/commit.go` - 1 registration
- `internal/component/cmd/subscribe/subscribe.go` - 2 registrations
- `internal/component/cmd/metrics/metrics.go` - 2 registrations
- `internal/component/cmd/log/log.go` - 2 registrations

**Edit shortcuts (Phase 4):**
- `internal/component/cli/model_mode.go` - derive edit shortcuts from YANG

**Completer unification (Phase 5):**
- `internal/component/cli/model.go` - update `updateCompletions()` for unified tree
- `internal/component/cli/completer.go` - extend to walk command nodes
- `internal/component/cli/completer_command.go` - may be absorbed into completer.go

**CLI/tree building (Phase 2-3):**
- `cmd/ze/cli/main.go` - update command tree building
- `cmd/ze/config/cmd_edit.go` - update command tree building
- `cmd/ze/yang/tree.go` - update unified analysis tree

## Files Created (Phase 1 -- DONE)

**YANG extensions:**
- `internal/component/config/yang/modules/ze-extensions.yang` - added `ze:command` (with handler argument) and `ze:edit-shortcut`

**Go extension functions:**
- `internal/component/config/yang/command.go` - `GetCommandExtension()`, `HasCommandExtension()`, `HasEditShortcutExtension()`
- `internal/component/config/yang/command_test.go` - 15 tests covering all 13 modules

**Per-plugin command YANG modules (13 files):**
- `internal/component/bgp/plugins/cmd/peer/schema/ze-peer-cmd.yang`
- `internal/component/bgp/plugins/cmd/rib/schema/ze-rib-cmd.yang`
- `internal/component/bgp/plugins/cmd/raw/schema/ze-raw-cmd.yang`
- `internal/component/bgp/plugins/cmd/update/schema/ze-update-cmd.yang`
- `internal/component/bgp/plugins/route_refresh/schema/ze-refresh-cmd.yang`
- `internal/component/cmd/meta/schema/ze-meta-cmd.yang`
- `internal/component/cmd/cache/schema/ze-cache-cmd.yang`
- `internal/component/cmd/commit/schema/ze-commit-cmd.yang`
- `internal/component/cmd/subscribe/schema/ze-subscribe-cmd.yang`
- `internal/component/cmd/log/schema/ze-log-cmd.yang`
- `internal/component/cmd/metrics/schema/ze-metrics-cmd.yang`
- `internal/core/ipc/schema/ze-system-cmd.yang`
- `internal/core/ipc/schema/ze-plugin-cmd.yang`

**Validation tooling:**
- `scripts/validate-commands.go` - cross-checks YANG `ze:command` vs registered handlers
- `Makefile` - added `make ze-validate-commands` target

**Plugin move (6 non-BGP plugins from `bgp/plugins/cmd/` to `component/cmd/`):**
- `internal/component/cmd/cache/` (was `bgp/plugins/cmd/cache/`)
- `internal/component/cmd/commit/` (was `bgp/plugins/cmd/commit/`)
- `internal/component/cmd/log/` (was `bgp/plugins/cmd/log/`)
- `internal/component/cmd/meta/` (was `bgp/plugins/cmd/meta/`)
- `internal/component/cmd/metrics/` (was `bgp/plugins/cmd/metrics/`)
- `internal/component/cmd/subscribe/` (was `bgp/plugins/cmd/subscribe/`)

## Files Still to Create (Phase 2-6)
- `test/ui/cli-yang-command-completion.ci` - functional test
- `test/ui/cli-yang-edit-shortcuts.ci` - functional test
- `test/plugin/yang-wire-dispatch.ci` - functional test

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Implementation phases below |
| 4. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase 1: YANG extensions + per-plugin command modules** (DONE)
   - Added `ze:command` (with WireMethod handler argument) and `ze:edit-shortcut` (marker) to `ze-extensions.yang`
   - Created 13 per-plugin `-cmd.yang` modules (one per owning plugin, not 3 monolithic)
   - Moved 6 non-BGP plugins from `bgp/plugins/cmd/` to `component/cmd/` (cache, commit, log, meta, metrics, subscribe)
   - Created `scripts/validate-commands.go` and `make ze-validate-commands` -- 58/58 validated
   - Tests: `TestCommandExtension`, `TestEditShortcutExtension`, `TestPeerCmdModule`, `TestRibCmdModule`, `TestRefreshCmdModule`, `TestMetaCmdModule`, `TestSimpleCmdModules`, `TestCommitEditShortcut`, `TestRawCmdModule`, `TestUpdateCmdModule`, `TestSystemCmdModuleLoads`, `TestPluginCmdModuleLoads`
   - Design changes from original spec: `ze:command` takes WireMethod argument (explicit, no naming convention); per-plugin YANG (proximity); non-BGP plugins moved; no WireMethod renaming

2. **Phase 2: Build command tree from YANG** -- rewrite `BuildTree()` to walk YANG `config false` entries with `ze:command`
   - Tests: `TestBuildTreeFromYANG`, `TestBuildTreeCommandNodes`, `TestBuildTreeBranches`
   - Files: `command/node.go`, `config/yang/command.go`
   - Verify: command tree matches current tree structure

3. **Phase 3: Remove CLICommand** -- remove `CLICommand` field from `RPCRegistration`, update all registration sites
   - Tests: `TestAllCommandsReachable`
   - Files: `handler.go`, all handler registration files, `cmd/ze/cli/main.go`, `cmd/ze/config/cmd_edit.go`, `cmd/ze/yang/tree.go`
   - Verify: all handlers dispatch correctly, `make ze-validate-commands` passes

4. ~~**Phase 4: Derive edit shortcuts from YANG**~~ Eliminated: all `editModeCommands` entries are editor-internal, not YANG-derivable

5. **Phase 4: Unify completers** -- extend config completer to walk command nodes, remove separate `CommandCompleter`
   - Tests: `TestCommandModeCompletionsFromYANG`
   - Files: `completer.go`, `completer_command.go`, `model.go`
   - Verify: completions in both modes work correctly

6. **Phase 5: Functional tests, docs, verification**
   - Functional `.ci` tests
   - Documentation updates
   - `make ze-verify`
   - Audit tables, learned summary

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | `make ze-validate-commands` passes (58/58 commands + 2 editor-internal skipped) |
| Correctness | `ze:command` only on executable leaves, not on grouping containers |
| Correctness | `ze:edit-shortcut` only on commands that make sense from edit mode |
| Correctness | `ze:command` handler argument matches the `RPCRegistration.WireMethod` exactly |
| No-layering | Old `CLICommand` field fully removed, not left as dead code |
| No-layering | `-api.yang` modules preserved (wire schema), `-cmd.yang` modules are new (CLI tree). No duplication -- different concerns. |
| Proximity | Each `-cmd.yang` lives in its owning plugin's schema/ directory |
| Proximity | Non-BGP plugins not under `bgp/plugins/` |
| Naming | YANG node names match current CLI tokens exactly (no renames in this spec) |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| `ze-extensions.yang` has `ze:command` with handler argument | `grep "extension command" ze-extensions.yang` |
| 13 per-plugin `-cmd.yang` modules exist | `make ze-validate-commands` finds all 58 |
| Non-BGP plugins moved out of `bgp/` | `ls internal/component/cmd/` shows 6 plugins |
| `CLICommand` field removed from `RPCRegistration` | `grep CLICommand handler.go` returns nothing |
| `editModeCommands` map YANG-derived | `grep editModeCommands model_mode.go` |
| All commands validated | `make ze-validate-commands` passes (exit 0) |
| `BuildTree()` walks YANG | `grep CLICommand command/node.go` returns nothing |
| `.ci` test files exist | `ls test/ui/cli-yang-command-completion.ci test/ui/cli-yang-edit-shortcuts.ci test/plugin/yang-wire-dispatch.ci` |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Extension parsing | Malformed `ze:command` in YANG doesn't crash loader |
| WireMethod injection | YANG path with special characters doesn't create invalid WireMethod |
| No new exec | No new external program execution added |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| goyang doesn't parse extension | Verify extension syntax, check goyang docs |
| Handler not reachable after rename | Trace WireMethod derivation, fix convention |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
| `ze-bgp-api.yang` was monolithic with 25 RPCs | 15 per-concern API modules with 60 registrations already existed | Codebase audit | Spec rewrite of inventory section |
| `ze:command` needs no argument (boolean marker) | WireMethod should be explicit argument -- eliminates renaming risk | User feedback | Eliminated Phase 4 WireMethod rename (60 files) |
| 3 monolithic `-cmd.yang` files (BGP, system, plugin) | Each plugin owns its command tree (proximity principle) | User feedback | 13 per-plugin modules instead of 3 |
| Non-BGP commands (cache, commit, log, etc.) belong in `bgp/plugins/` | They are general daemon commands, not BGP | User feedback | Moved 6 plugins to `component/cmd/` |
| Commands with "bgp" CLICommand prefix are BGP-specific | The "bgp " prefix is an artifact stripped by BuildTree() | User feedback | All `-cmd.yang` modules drop "bgp" from names |

### Failed Approaches
| Approach | Why abandoned | Replacement |
| `ze:command` as boolean marker + WireMethod derived from naming convention | Convention-based is implicit; requires renaming all 60 WireMethod strings | `ze:command "wire-method"` -- explicit handler reference |
| Monolithic `ze-bgp-cmd.yang` with all commands | Violates proximity principle; can't "delete the folder" | Per-plugin `-cmd.yang` modules |
| `ze:edit-shortcut` on `commit` to replace `editModeCommands` map | All 15 `editModeCommands` entries are editor-internal (handled by CLI model methods like `cmdCommitConfirmed`), not operational RPCs. The `ze-bgp:commit` RPC (named route commits) is a different command from the editor's `commit`. | `ze:edit-shortcut` defined but zero uses. `editModeCommands` stays as-is. AC-3/AC-4/AC-12 eliminated. |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
| Assumed historical "bgp" prefix reflects actual domain | Once but fundamental | When CLICommand has a stripped prefix, the command name is what the user types, not the raw string | Added to this spec's Design Insights |

## Design Insights

## RFC Documentation

Not applicable -- this is CLI/YANG architecture, not protocol work.

## Implementation Summary

### What Was Implemented
- (to be filled)

### Bugs Found/Fixed
- (to be filled)

### Documentation Updates
- (to be filled)

### Deviations from Plan
- (to be filled)

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |

### Tests from TDD Plan
| Test | Status | Location | Notes |

### Files from Plan
| File | Status | Notes |

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-12 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `docs/learned/NNN-<name>.md`
- [ ] **Summary included in commit**
