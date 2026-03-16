# Spec: YANG Command Tree

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | - |
| Phase | - |
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
- ~~54 RPCRegistrations currently exist (counted from grep).~~ Superseded: 57 RPCRegistrations across 15 handler files (audited 2026-03-16).

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/server/handler.go` - `RPCRegistration` struct: `WireMethod`, `CLICommand`, `Handler`, `Help`, `ReadOnly`, `RequiresSelector`, `PluginCommand`.
- [ ] `internal/component/command/node.go` - `BuildTree()` takes `[]RPCInfo`, strips "bgp " prefix, splits on spaces, builds `*Node` tree. `RPCInfo` has `CLICommand`, `Help`, `ReadOnly`.
- [ ] `internal/component/cli/model_mode.go` - `editModeCommands` is a hardcoded `map[string]bool` of 15 keywords: set, delete, show, edit, commit, save, discard, compare, rollback, history, load, errors, top, up, who, disconnect. `isEditCommand()` and `isEditCommandWithArgs()` check against this map.
- [ ] `internal/component/cli/model.go:870` - `updateCompletions()` has 4-way switch: edit+run prefix, command+edit-args, command top-level (merge via append), edit mode. No dedup on merge.
- [ ] `internal/component/cli/completer.go` - `Completer` walks `*gyang.Entry.Dir` for config. `mergedRoot()` combines 3 conf modules. `confModules = ["ze-bgp-conf", "ze-hub-conf", "ze-plugin-conf"]`.
- [ ] `internal/component/cli/completer_command.go` - `CommandCompleter` wraps `command.TreeCompleter`, converts `command.Suggestion` to `cli.Completion`.
- [ ] `internal/component/config/yang/modules/ze-extensions.yang` - 6 extensions: `syntax`, `key-type`, `route-attributes`, `allow-unknown-fields`, `sensitive`, `validate`.
- [ ] ~~`internal/component/bgp/schema/ze-bgp-api.yang` - 25 flat RPCs~~ Superseded: 15 YANG API modules with 57 wired RPCRegistrations across per-concern plugins. See full inventory below.

**YANG API Module Inventory (audited 2026-03-16):**

| Module | Path | RPCs | WireMethod prefix | Handler location |
|--------|------|------|-------------------|------------------|
| `ze-system-api` | `internal/core/ipc/schema/` | 11 | `ze-system:` | `plugin/server/system.go` |
| `ze-plugin-api` | `internal/core/ipc/schema/` | 8 | `ze-plugin:` | `plugin/server/session.go`, `plugin_rpc.go` |
| `ze-bgp-api` | `bgp/schema/` | 46 | `ze-bgp:` | Distributed across `cmd/*` plugins |
| `ze-bgp-cmd-peer-api` | `bgp/plugins/cmd/peer/schema/` | 11 | `ze-bgp:` | `cmd/peer/peer.go`, `summary.go` |
| `ze-bgp-cmd-meta-api` | `bgp/plugins/cmd/meta/schema/` | 8 | `ze-bgp:` | `cmd/meta/help.go`, `plugin_config.go` |
| `ze-bgp-cmd-update-api` | `bgp/plugins/cmd/update/schema/` | 2 | `ze-bgp:` | `cmd/update/update_text.go` |
| `ze-bgp-cmd-raw-api` | `bgp/plugins/cmd/raw/schema/` | 1 | `ze-bgp:` | `cmd/raw/raw.go` |
| `ze-bgp-cmd-cache-api` | `bgp/plugins/cmd/cache/schema/` | 1 | `ze-bgp:` | `cmd/cache/cache.go` |
| `ze-bgp-cmd-commit-api` | `bgp/plugins/cmd/commit/schema/` | 1 | `ze-bgp:` | `cmd/commit/commit.go` |
| `ze-bgp-cmd-subscribe-api` | `bgp/plugins/cmd/subscribe/schema/` | 2 | `ze-bgp:` | `cmd/subscribe/subscribe.go` |
| `ze-bgp-cmd-metrics-api` | `bgp/plugins/cmd/metrics/schema/` | 2 | `ze-bgp:` | `cmd/metrics/metrics.go` |
| `ze-bgp-cmd-log-api` | `bgp/plugins/cmd/log/schema/` | 2 | `ze-bgp:` | `cmd/log/log.go` |
| `ze-rib-api` | `bgp/plugins/rib/schema/` | 12 | `ze-rib-api:` | `cmd/rib/rib.go` (forwards to bgp-rib plugin) |
| `ze-route-refresh-api` | `bgp/plugins/route_refresh/schema/` | 4 | `ze-bgp:` | `route_refresh/handler/refresh.go`, `clear_soft.go` |
| `ze-adj-rib-in-api` | `bgp/plugins/adj_rib_in/schema/` | 0 | N/A | Config-only module |

**RPCRegistration Inventory (57 wired, sorted by CLICommand):**

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
| `plugin command complete` | `ze-plugin:command-complete` | `server/plugin_rpc.go` | Yes | No | - |
| `plugin command help` | `ze-plugin:command-help` | `server/plugin_rpc.go` | Yes | No | - |
| `plugin command list` | `ze-plugin:command-list` | `server/plugin_rpc.go` | Yes | No | - |
| `plugin help` | `ze-plugin:help` | `server/plugin_rpc.go` | Yes | No | - |
| `plugin session bye` | `ze-plugin:session-bye` | `server/session.go` | No | No | - |
| `plugin session ping` | `ze-plugin:session-ping` | `server/session.go` | Yes | No | - |
| `plugin session ready` | `ze-plugin:session-ready` | `server/session.go` | No | No | - |
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
- `CLICommand` string field removed from `RPCRegistration` -- derived from YANG path
- `BuildTree()` walks YANG entries instead of splitting strings
- `editModeCommands` map derived from YANG `ze:edit-shortcut` extension instead of hardcoded
- Two separate completers merged into one YANG-walking completer with mode awareness
- YANG API modules restructured from flat RPCs to hierarchical `config false` containers
- `ze-extensions.yang` extended with `ze:command` and `ze:edit-shortcut`

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
| YANG loader -> completer | `GetEntry()` returns unified tree | [ ] |
| YANG path -> WireMethod | Convention: path `peer > list` in module `ze-bgp-cmd` -> `"ze-bgp:peer.list"` | [ ] |
| Completer -> handler dispatch | User input matched against YANG path, resolved to WireMethod, dispatched | [ ] |

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
| `ze:command` | none | Marks a `config false` container as an executable command (not just a grouping node). Leaf children are parameters. |
| `ze:edit-shortcut` | none | Marks a command node as available in edit mode as a shortcut. When typed in edit mode, implicitly runs as `run <command>`. Replaces the hardcoded `editModeCommands` map. |

Detection in Go: walk `Entry.Exts` for `ze:command` / `ze:edit-shortcut` statements.

## YANG Module Restructuring

~~### Current: flat RPCs in `ze-bgp-api.yang`~~ Superseded (2026-03-16).

### Actual current state: 15 per-concern `-api.yang` modules with flat RPCs

Each plugin already has its own YANG API module (e.g., `ze-bgp-cmd-peer-api.yang`). The RPCs within each module are flat (e.g., `peer-list`, `peer-add`). The restructuring adds a parallel `-cmd.yang` module per domain that defines hierarchical `config false` containers matching the CLI tree, while the `-api.yang` modules (with flat RPCs and input/output schemas) are preserved for wire protocol documentation.

### After: hierarchical `-cmd.yang` modules alongside existing `-api.yang`

Each domain gets a `-cmd.yang` module with `config false` containers defining the command tree. The CLI command hierarchy is the tree structure itself. The `-api.yang` RPCs remain as the wire protocol schema (input/output leaf definitions).

**BGP domain (`ze-bgp-cmd.yang` -- new, replaces tree from all BGP plugins' CLICommand strings):**

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

YANG path to WireMethod convention:

| YANG module | YANG path | WireMethod |
|-------------|-----------|------------|
| `ze-bgp-cmd` | `peer > list` | `ze-bgp:peer.list` |
| `ze-bgp-cmd` | `peer > clear > soft` | `ze-bgp:peer.clear.soft` |
| `ze-bgp-cmd` | `rib > best > status` | `ze-rib-api:best.status` |
| `ze-system-cmd` | `daemon > shutdown` | `ze-system:daemon.shutdown` |
| `ze-system-cmd` | `system > version > software` | `ze-system:version.software` |
| `ze-plugin-cmd` | `plugin > session > ready` | `ze-plugin:session.ready` |

The `.` separator in WireMethod replaces the current `-` (e.g., `ze-bgp:peer-list` becomes `ze-bgp:peer.list`). Handler dispatch uses WireMethod unchanged. This is a naming convention change that must be applied consistently across all 57 registrations.

### Edit shortcuts

Commands tagged with `ze:edit-shortcut` in YANG:

| Command | Why it's a shortcut |
|---------|-----|
| `commit` | Applies config changes -- natural from edit mode |
| `save` | Persists config -- natural from edit mode |
| `rollback` | Restores config -- natural from edit mode |
| `load` | Loads config -- natural from edit mode |

These replace the hardcoded `editModeCommands` map. The remaining entries in that map (`set`, `delete`, `edit`, `show`, `top`, `up`, `discard`, `compare`, `errors`, `history`, `who`, `disconnect`) are editor-internal commands, not operational commands -- they stay as they are (not YANG-derived).

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Tab completion in command mode | -> | YANG-derived command tree | `test/ui/cli-yang-command-completion.ci` |
| Tab completion in edit mode for shortcuts | -> | `ze:edit-shortcut` extension | `test/ui/cli-yang-edit-shortcuts.ci` |
| `ze yang tree` shows unified tree | -> | Unified YANG walk | `test/ui/cli-yang-tree.ci` (from spec-yang-analysis) |
| Handler dispatch via new WireMethod | -> | Dispatch still works after rename | `test/plugin/yang-wire-dispatch.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Tab in command mode | Shows command completions from YANG `-cmd` module entries (not from CLICommand strings) |
| AC-2 | `ze:command` on a `config false` container | Completer offers it as an executable command |
| AC-3 | `ze:edit-shortcut` on a command | Command appears in edit mode completions without `run` prefix |
| AC-4 | `commit` typed in edit mode | Executes as `run commit` (same behavior as today, now driven by YANG tag) |
| AC-5 | `RPCRegistration` struct | `CLICommand` field removed. YANG path is the source of truth. |
| AC-6 | `BuildTree()` or equivalent | Walks YANG `config false` entries instead of splitting CLICommand strings |
| AC-7 | All 57 existing commands | Still reachable and executable after restructuring. No command lost. |
| AC-8 | `ze yang tree` (from spec-yang-analysis) | Shows unified tree with both config and command nodes |
| AC-9 | `ze yang completion` (from spec-yang-analysis) | Finds collisions in the unified tree |
| AC-10 | WireMethod format | Uses `.` separator for path hierarchy (`ze-bgp:peer.list`) |
| AC-11 | Handler dispatch | All handlers still dispatch correctly with new WireMethod format |
| AC-12 | `editModeCommands` map | Removed or derived from YANG `ze:edit-shortcut` tags. No hardcoded list. |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCommandExtension` | `internal/component/config/yang/extension_test.go` | `ze:command` extension parsed from YANG | |
| `TestEditShortcutExtension` | `internal/component/config/yang/extension_test.go` | `ze:edit-shortcut` extension parsed from YANG | |
| `TestBuildTreeFromYANG` | `internal/component/command/node_test.go` | Tree built from YANG entries matches expected structure | |
| `TestBuildTreeCommandNodes` | `internal/component/command/node_test.go` | Only `ze:command`-tagged nodes become executable tree leaves | |
| `TestBuildTreeBranches` | `internal/component/command/node_test.go` | Non-command containers become navigable branches | |
| `TestWireMethodFromPath` | `internal/component/config/yang/rpc_test.go` | YANG path `peer > list` in module `ze-bgp-cmd` produces `ze-bgp:peer.list` | |
| `TestEditShortcutsFromYANG` | `internal/component/cli/completer_test.go` | Edit mode completions include `ze:edit-shortcut` tagged commands | |
| `TestCommandModeCompletionsFromYANG` | `internal/component/cli/completer_test.go` | Command mode completions come from YANG walk, not CLICommand strings | |
| `TestAllCommandsReachable` | `internal/component/plugin/server/dispatch_test.go` | Every handler is reachable via new WireMethod after restructuring | |
| `TestNoHardcodedEditModeCommands` | `internal/component/cli/model_mode_test.go` | No static map -- shortcuts derived from YANG | |

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

## Files to Modify

**YANG extensions:**
- `internal/component/config/yang/modules/ze-extensions.yang` - add `ze:command` and `ze:edit-shortcut` extensions

**Core infrastructure:**
- `internal/component/plugin/server/handler.go` - remove `CLICommand` from `RPCRegistration`
- `internal/component/command/node.go` - rewrite `BuildTree()` to walk YANG entries
- `internal/component/cli/model_mode.go` - derive edit shortcuts from YANG instead of hardcoded map
- `internal/component/cli/model.go` - update `updateCompletions()` for unified tree
- `internal/component/cli/completer.go` - extend to walk command nodes (`config false` + `ze:command`)
- `internal/component/cli/completer_command.go` - may be absorbed into completer.go

**System/plugin handlers (WireMethod + CLICommand):**
- `internal/component/plugin/server/system.go` - 11 registrations
- `internal/component/plugin/server/plugin_rpc.go` - 4 registrations
- `internal/component/plugin/server/session.go` - 3 registrations

**BGP command handlers (WireMethod + CLICommand):**
- `internal/component/bgp/plugins/cmd/meta/help.go` - 5 registrations
- `internal/component/bgp/plugins/cmd/meta/plugin_config.go` - 3 registrations
- `internal/component/bgp/plugins/cmd/peer/peer.go` - 8 registrations
- `internal/component/bgp/plugins/cmd/peer/summary.go` - 3 registrations
- `internal/component/bgp/plugins/cmd/peer/session.go` - 1 registration
- `internal/component/bgp/plugins/cmd/rib/rib.go` - 6 registrations
- `internal/component/bgp/plugins/cmd/update/update_text.go` - 1 registration
- `internal/component/bgp/plugins/cmd/raw/raw.go` - 1 registration
- `internal/component/bgp/plugins/cmd/subscribe/subscribe.go` - 2 registrations
- `internal/component/bgp/plugins/cmd/cache/cache.go` - 1 registration
- `internal/component/bgp/plugins/cmd/commit/commit.go` - 1 registration
- `internal/component/bgp/plugins/cmd/metrics/metrics.go` - 2 registrations
- `internal/component/bgp/plugins/cmd/log/log.go` - 2 registrations
- `internal/component/bgp/plugins/route_refresh/handler/refresh.go` - 3 registrations
- `internal/component/bgp/plugins/route_refresh/handler/clear_soft.go` - 1 registration

**CLI/tree building:**
- `cmd/ze/cli/main.go` - update command tree building
- `cmd/ze/config/cmd_edit.go` - update command tree building
- `cmd/ze/yang/tree.go` - update unified analysis tree for new YANG structure

**Existing YANG API modules (preserved, not deleted -- wire protocol schema):**
- All 15 `-api.yang` modules remain as documentation of RPC input/output schemas

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new `-cmd` modules) | Yes | 3 new `-cmd.yang` modules alongside existing `-api.yang` |
| RPC count in architecture docs | Yes | `docs/architecture/api/commands.md` |
| CLI commands/flags | No | Commands unchanged, just YANG source changes |
| CLI usage/help text | No | Help text comes from YANG description |
| API commands doc | Yes | `docs/architecture/api/commands.md` - update tree structure |
| Plugin SDK docs | No | SDK unaffected |
| Editor autocomplete | Yes | Completers restructured |
| Functional test for new RPC/API | Yes | `.ci` tests in wiring table |

## Files to Create
- `internal/component/bgp/schema/ze-bgp-cmd.yang` - hierarchical BGP command tree (all 40 BGP CLICommand paths as `config false` containers)
- `internal/core/ipc/schema/ze-system-cmd.yang` - hierarchical system command tree (11 system/daemon commands)
- `internal/core/ipc/schema/ze-plugin-cmd.yang` - hierarchical plugin command tree (7 plugin commands)
- `internal/component/config/yang/command.go` - YANG command tree walker (extracts `ze:command` nodes, builds `command.Node` tree)
- `internal/component/config/yang/command_test.go` - tests for command tree walker
- `test/ui/cli-yang-command-completion.ci` - functional test
- `test/ui/cli-yang-edit-shortcuts.ci` - functional test
- `test/plugin/yang-wire-dispatch.ci` - functional test

~~`internal/component/bgp/plugins/rib/schema/ze-rib-cmd.yang`~~ Superseded: RIB commands are in the BGP domain (`bgp rib ...`) so they go in `ze-bgp-cmd.yang` alongside other BGP commands.

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

1. **Phase: YANG extensions** -- add `ze:command` and `ze:edit-shortcut` to ze-extensions.yang, verify goyang parses them
   - Tests: `TestCommandExtension`, `TestEditShortcutExtension`
   - Files: `ze-extensions.yang`, `extension_test.go`
   - Verify: tests fail -> implement -> tests pass

2. **Phase: Command YANG modules** -- create 3 new `-cmd.yang` modules (`ze-bgp-cmd.yang`, `ze-system-cmd.yang`, `ze-plugin-cmd.yang`) with hierarchical `config false` containers matching the CLI tree. Existing `-api.yang` modules preserved as wire protocol schema.
   - Tests: `TestBuildTreeFromYANG`, `TestWireMethodFromPath`
   - Files: new `-cmd.yang` modules + register.go, `command.go` walker
   - Verify: YANG loads and resolves, tree has expected hierarchy

3. **Phase: Build command tree from YANG** -- rewrite `BuildTree()` to walk YANG entries instead of splitting strings
   - Tests: `TestBuildTreeCommandNodes`, `TestBuildTreeBranches`
   - Files: `command/node.go`, `config/yang/command.go`
   - Verify: command tree matches current tree structure

4. **Phase: Update handler registrations** -- remove `CLICommand` from `RPCRegistration`, update all 57 WireMethod strings to use `.` separator, update dispatch across 18 handler files
   - Tests: `TestAllCommandsReachable`
   - Files: `handler.go`, 18 handler registration files (see Files to Modify)
   - Verify: all handlers dispatch correctly

5. **Phase: Derive edit shortcuts from YANG** -- replace hardcoded `editModeCommands` with YANG `ze:edit-shortcut` lookup
   - Tests: `TestEditShortcutsFromYANG`, `TestNoHardcodedEditModeCommands`
   - Files: `model_mode.go`, `-cmd.yang` modules
   - Verify: edit shortcuts work as before

6. **Phase: Unify completers** -- extend config completer to walk command nodes, remove separate CommandCompleter
   - Tests: `TestCommandModeCompletionsFromYANG`, `TestEditShortcutsFromYANG`
   - Files: `completer.go`, `completer_command.go`, `model.go`
   - Verify: completions in both modes work correctly

7. **Functional tests** -- create `.ci` tests
   - Files: `test/ui/cli-yang-command-completion.ci`, `test/ui/cli-yang-edit-shortcuts.ci`, `test/plugin/yang-wire-dispatch.ci`

8. **Documentation** -- update `docs/architecture/api/commands.md` tree structure

9. **Full verification** -- `make ze-verify`

10. **Complete spec** -- fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All 57 commands still reachable after restructuring |
| Correctness | WireMethod convention consistently applied (`.` separator) |
| Correctness | `ze:command` only on executable leaves, not on grouping containers |
| Correctness | `ze:edit-shortcut` only on commands that make sense from edit mode |
| No-layering | Old `CLICommand` field fully removed, not left as dead code |
| No-layering | `-api.yang` modules preserved (wire schema), `-cmd.yang` modules are new (CLI tree). No duplication -- different concerns. |
| Naming | YANG node names match current CLI tokens exactly (no renames in this spec -- renames come from analysis tool findings) |
| Data flow | YANG path -> WireMethod derivation is deterministic and reversible |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| `ze-extensions.yang` has `ze:command` and `ze:edit-shortcut` | `grep "extension command" ze-extensions.yang` |
| `ze-bgp-cmd.yang` exists with hierarchical structure | `ls internal/component/bgp/schema/ze-bgp-cmd.yang` |
| `ze-system-cmd.yang` exists | `ls internal/core/ipc/schema/ze-system-cmd.yang` |
| `ze-plugin-cmd.yang` exists | `ls internal/core/ipc/schema/ze-plugin-cmd.yang` |
| `CLICommand` field removed from `RPCRegistration` | `grep CLICommand handler.go` returns nothing |
| `editModeCommands` map removed or YANG-derived | `grep editModeCommands model_mode.go` |
| All commands reachable | `go test ./internal/component/plugin/server/... -run TestAllCommandsReachable` |
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

### Failed Approaches
| Approach | Why abandoned | Replacement |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |

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
