# 462 -- YANG Analysis Tool

## Objective

Build `ze yang` CLI tool to automate prefix collision detection and command documentation across the YANG config tree and operational command tree. Runs after every schema change to find naming problems before users hit them.

## Decisions

- **Unified analysis tree in Go (approach A):** goyang already puts config containers, RPCs, and notifications in the same `Entry.Dir` map. The runtime splits them into two completers (`cli/completer.go` for config, `command/completer.go` for commands). The analysis tool recombines them into one tree with `Source` tags ("config", "command", "both") to catch cross-domain collisions.
- **Separate from `ze schema`:** `ze schema` is discovery-oriented (list modules, show YANG text). `ze yang` is analysis-oriented (find naming problems, generate docs). Different concerns.
- **Generic prefix analyzer:** `FindCollisions()` takes `[]SiblingInfo` and groups by shared first character. Domain-independent, unit-testable without YANG loading. The tree builder feeds it sibling groups at each level.
- **RPC registrations as command source:** Command tree comes from `RPCRegistration.CLICommand` strings (same as `BuildTree()` in runtime), not from YANG RPC names. This ensures collisions match what users actually experience.
- **YANG-derived parameters for doc:** `ze yang doc <command>` uses `ExtractRPCs()` to get input/output leaf metadata from API YANG modules, matched by WireMethod.
- **Companion spec for YANG command tree restructuring:** `spec-yang-command-tree.md` proposes making commands YANG-native (hierarchical `config false` containers with `ze:command` extension), eliminating the `CLICommand` string indirection. The analysis tool works on either architecture.

## Patterns

- **Blank imports for YANG registration:** The `cmd/ze/yang/tree.go` file needs blank imports for all YANG schema packages and RPC handler packages to trigger `init()` registration. Same pattern as `cmd/ze/cli/main.go`.
- **Double YANG loader creation:** `BuildUnifiedTree()` creates a loader for config, `AllRPCDocs()` creates another for API RPCs. Acceptable for CLI tool (not hot path). Could be shared if needed.
- **`config false` distinguishes domains:** goyang `Entry.Config == TSFalse` marks operational state. RPCs have `.RPC != nil`. Notifications have `.Kind == NotificationEntry`. The tree walker uses these to skip non-config entries from conf modules.
- **Constants for domain strings:** `SourceConfig`, `SourceCommand`, `SourceBoth`, `FilterCommands` prevent the goconst linter from flagging repeated string literals across files.

## Gotchas

- **goimports removes imports on individual edits:** The auto-linter hook runs goimports on each file write. If you add an import without simultaneously adding its usage, goimports removes it. Always add import + usage in the same edit.
- **goyang has no `ActionEntry`:** Cannot use YANG 1.1 `action` statement. The future command tree spec uses `config false` containers with `ze:command` extension instead.
- **`editModeCommands` is hardcoded:** The `model_mode.go` map of 15 keywords (commit, save, rollback, etc.) that auto-switch from command mode to edit mode is a hardcoded Go map, not derived from YANG. The companion spec proposes replacing it with a `ze:edit-shortcut` YANG extension.
- **Mutually exclusive flags need explicit check:** `flag.NewFlagSet` doesn't support mutually exclusive flags natively. Must check `*commands && *config` manually after parsing.

## Files

- `cmd/ze/yang/prefix.go` -- prefix collision algorithm
- `cmd/ze/yang/tree.go` -- unified analysis tree builder + RPC doc extraction
- `cmd/ze/yang/format.go` -- text and JSON formatters
- `cmd/ze/yang/doc.go` -- per-command documentation
- `cmd/ze/yang/main.go` -- CLI dispatch
- `cmd/ze/main.go` -- added `yang` dispatch case
- `test/ui/cli-yang-*.ci` -- 3 functional tests
- `plan/spec-yang-analysis.md` -- this tool's spec
- `plan/spec-yang-command-tree.md` -- companion spec for YANG-native commands
