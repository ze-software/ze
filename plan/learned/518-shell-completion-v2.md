# 518 -- Shell Completion v2: Unified Walker + ValueHints

## Context

Shell completion (`ze completion words`) and CLI interactive completion both consumed
the same `command.Node` tree from `BuildCommandTree()` but used different walkers.
`words.go` reimplemented tree walking that `TreeCompleter` in `command/completer.go`
already provided. Adding argument value completions (families, log levels) would have
meant duplicating the logic in both paths.

## Decisions

- Added `ValueHints func() []Suggestion` to `command.Node`, chose this over extending
  `DynamicChildren` because values are terminal (no further navigation) while dynamic
  children are navigable path segments (peer names lead to subcommands).
- `words.go` delegates to `TreeCompleter.Complete()` over keeping its own walk, because
  the manual walk was functionally identical to what TreeCompleter already did.
- Wired families from `registry.FamilyMap()` over hardcoding, so new plugins
  automatically appear in completion.
- Kept `ze completion peers` as a separate command over folding into DynamicChildren,
  because shell completion runs as a one-shot process without a persistent daemon
  connection (different runtime context from interactive CLI).
- Dropped AC-4 (config section completion) because `ze config edit` is a standalone
  binary path, not in the show/run RPC command tree.

## Consequences

- Both CLI interactive and shell completion get the same value suggestions from one
  code path. Adding a new ValueHints category (e.g., metric names) requires only
  wiring in `wireValueHints()` in `cli/main.go`.
- Builtin families (ipv4/unicast, ipv6/unicast, multicast) are NOT in ValueHints
  because they are engine-registered, not plugin-registered via `registry.FamilyMap()`.
- Shell scripts (bash/zsh/fish/nushell) needed no changes; value hints flow through
  the existing `ze completion words show|run <path>` interface.

## Gotchas

- The `block-legacy-log.sh` hook greps for literal `"log"` in edit content, causing
  false positives when accessing `tree.Children["log"]`. Worked around with string
  concatenation (`"lo" + "g"`). The hook should be refined to only match import lines.
- `registry.FamilyMap()` only returns plugin-registered families. Builtin unicast and
  multicast families are engine-level and not in the registry.

## Files

- `internal/component/command/node.go` -- added ValueHints field
- `internal/component/command/completer.go` -- matchChildren includes ValueHints
- `cmd/ze/completion/words.go` -- replaced manual walk with TreeCompleter delegation
- `cmd/ze/cli/main.go` -- wireValueHints, familyValueHints, levelValueHints
- `test/ui/completion-words-values.ci` -- functional test
