# 381 — Shell Completion (bash, zsh, fish, nushell)

## Objective

Add contextual tab completion with descriptions for ze across bash, zsh, fish, and nushell shells. Static completions for the CLI dispatch tree, dynamic completions for YANG-driven data (show/run commands, plugin names, schema modules).

## Decisions

- Created `ze completion words show|run [path...]` as a shared data source outputting `word\tdesc` pairs — all four shells consume this instead of parsing `ze show help` output
- Fish completion uses named helper functions (`__ze_complete_plugins`, `__ze_complete_schema_modules`, `__ze_complete_dynamic`) rather than inline `-a '...'` expressions — fish single-quotes have NO escape mechanism (`\'` is literal, not an escape)
- Bash strips descriptions (`| cut -f1`) since bash `compgen -W` doesn't support them; zsh converts tabs to colons (`| sed 's/\t/:/'`) for `_describe`; fish uses tab format natively
- Nushell uses `extern` definitions (not `export extern`) with `source` (not `use`) — nushell's module system conflicts with extern command names
- Nushell accepts `nu` as alias for `nushell` in the CLI dispatch

## Patterns

- All shell helpers share identical global flag-skipping lists: no-arg flags (`-d`, `--debug`, `-V`, `--version`, `--plugins`) and arg-consuming flags (`--plugin`, `--pprof`, `--chaos-seed`, `--chaos-rate`)
- Fish depth calculation: `__ze_depth subcmd` counts positional words after subcommand, subtracting 1 for partial current token (`commandline -ct`) to match bash/zsh semantics
- Multi-level dynamic completion: shell collects path words between subcommand and cursor, passes to `ze completion words <context> <path...>`, which walks `BuildCommandTree()` children
- Nushell custom completers receive `context: string, offset: int` — parse the context string to extract path words for dynamic show/run completion

## Gotchas

- **Fish quoting trap**: `complete -a '...'` with regex containing `[^"]*` breaks — the `\` is literal inside fish single-quotes, leaving `[^` as an unquoted glob pattern. Must use helper functions for any regex
- **Fish `commandline -opc` partial token**: `[-1]` returns the partial token being typed, not the last completed word. Schema module completion must walk the full command line to find the word after `schema`, not just take the last element
- **Tab in descriptions**: Safe because YANG Help fields are human prose (verified: no tabs), command names come from `strings.Fields()` (single words), and synthesized fallback uses commas
- **Nushell module name collision**: A file named `ze.nu` imported with `use ze.nu` creates module `ze`, which prevents `export extern "ze"` (same-name collision). Fix: use `source` with plain `extern` instead of `use` with `export extern`
- **Nushell `use ... *` doesn't export externs**: Even with a differently-named file, `use foo.nu *` does not make `extern "ze"` available in scope. Only `source` works for extern definitions
- **Nushell completion file naming**: The completion file can be named `ze.nu` when using `source` (no module created), but must NOT be `ze.nu` when using `use`

## Files

- `cmd/ze/completion/nushell.go` — new: nushell completion script with extern definitions and custom completers
- `cmd/ze/completion/words.go` — shared `writeWords()` data source
- `cmd/ze/completion/words_test.go` — 6 tests for words output
- `cmd/ze/completion/fish.go` — fish shell completion script
- `cmd/ze/completion/main.go` — dispatch for all four shells + words
- `cmd/ze/completion/bash.go` — bash completion script
- `cmd/ze/completion/zsh.go` — zsh completion script
- `cmd/ze/completion/main_test.go` — 42 tests covering all four shells
