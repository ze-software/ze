# 373 — Shell Completion (ze completion)

## Objective

Add `ze completion bash` and `ze completion zsh` commands that output shell completion
scripts with static CLI tree completions and dynamic YANG-driven callbacks.

## Decisions

- Static completions for the hardcoded CLI dispatch tree (bgp, config, schema, etc.)
- Dynamic callbacks at tab time for plugin names (`ze --plugins --json`), schema modules (`ze schema list`), and show/run subcommands (`ze show help`, `ze run help`)
- Zero internal imports — completion package only generates shell script text
- Functional `.ci` tests skipped — unit tests thoroughly cover script content; no daemon interaction needed

## Patterns

- `generate(shell, writer)` writes script to any `io.Writer` — testable without stdout capture
- `bashScript()` / `zshScript()` return full scripts as string literals in separate files
- `_ze_find_subcmd` helper in bash skips global flags (--debug, --plugin, etc.) to find the subcommand word
- zsh uses `CURRENT == 2` depth guards to offer subcommands only at the right nesting level
- prev-based completion: `--plugin)` case triggers plugin name completion instead of subcommand completion

## Gotchas

- `ze show` and `ze run` completions must be dynamic (call back to `ze show help` / `ze run help`) since their command trees come from RPC registrations — hardcoding would drift
- Bash `_filedir` may not exist in minimal environments — `_ze_filedir` provides a compgen fallback
- `ze show help` awk pattern must filter within "Available commands:" section to avoid matching "ze" from the Examples section

## Files

- `cmd/ze/completion/main.go` — dispatch: bash/zsh/help/error
- `cmd/ze/completion/bash.go` — bash completion script generation
- `cmd/ze/completion/zsh.go` — zsh completion script generation
- `cmd/ze/completion/main_test.go` — 18 tests covering structure, commands, subcommands, dynamic callbacks, flags
