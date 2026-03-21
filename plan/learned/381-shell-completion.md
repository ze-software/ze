# 381 — Shell Completion (bash, zsh, fish)

## Objective

Add contextual tab completion with descriptions for ze across bash, zsh, and fish shells. Static completions for the CLI dispatch tree, dynamic completions for YANG-driven data (show/run commands, plugin names, schema modules).

## Decisions

- Created `ze completion words show|run [path...]` as a shared data source outputting `word\tdesc` pairs — all three shells consume this instead of parsing `ze show help` output
- Fish completion uses named helper functions (`__ze_complete_plugins`, `__ze_complete_schema_modules`, `__ze_complete_dynamic`) rather than inline `-a '...'` expressions — fish single-quotes have NO escape mechanism (`\'` is literal, not an escape)
- Bash strips descriptions (`| cut -f1`) since bash `compgen -W` doesn't support them; zsh converts tabs to colons (`| sed 's/\t/:/'`) for `_describe`; fish uses tab format natively

## Patterns

- All shell helpers share identical global flag-skipping lists: no-arg flags (`-d`, `--debug`, `-V`, `--version`, `--plugins`) and arg-consuming flags (`--plugin`, `--pprof`, `--chaos-seed`, `--chaos-rate`)
- Fish depth calculation: `__ze_depth subcmd` counts positional words after subcommand, subtracting 1 for partial current token (`commandline -ct`) to match bash/zsh semantics
- Multi-level dynamic completion: shell collects path words between subcommand and cursor, passes to `ze completion words <context> <path...>`, which walks `BuildCommandTree()` children

## Gotchas

- **Fish quoting trap**: `complete -a '...'` with regex containing `[^"]*` breaks — the `\` is literal inside fish single-quotes, leaving `[^` as an unquoted glob pattern. Must use helper functions for any regex
- **Fish `commandline -opc` partial token**: `[-1]` returns the partial token being typed, not the last completed word. Schema module completion must walk the full command line to find the word after `schema`, not just take the last element
- **Tab in descriptions**: Safe because YANG Help fields are human prose (verified: no tabs), command names come from `strings.Fields()` (single words), and synthesized fallback uses commas

## Files

- `cmd/ze/completion/words.go` — new: shared `writeWords()` data source
- `cmd/ze/completion/words_test.go` — new: 6 tests for words output
- `cmd/ze/completion/fish.go` — new: fish shell completion script
- `cmd/ze/completion/main.go` — modified: fish + words dispatch
- `cmd/ze/completion/bash.go` — modified: use `ze completion words` for show/run
- `cmd/ze/completion/zsh.go` — modified: use `ze completion words` for show/run, add fish to shells list
- `cmd/ze/completion/main_test.go` — modified: 32 tests covering all three shells
