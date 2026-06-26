# 1001 -- ze help ai: AI reference as a subcommand, not a flag stack

## Context

The machine-readable AI reference was reached via `ze help --ai --api`
(a `--ai` mode flag plus `--cli`/`--api`/`--mcp`/`--dispatch`/`--all` section
flags, all parsed with `slices.Contains`). Two problems surfaced while an agent
was driving the binary:

1. **Undiscoverable.** Nothing in `ze`, `ze --help`, `ze help`, or the
   unknown-command path named the leaf. An agent's natural guess `ze --ai`
   dead-ended with "unknown command: --ai" and a usage dump that did not mention
   the reference at all. You had to already know the exact string.
2. **Off-grammar.** Ze is a network-OS CLI that is otherwise almost entirely
   positional keywords (`show ip route`, `delete bgp peer`). The flag-stack
   `help --ai --api` was the odd one out, and `ze help command` was already a
   positional subcommand in the same `dispatchHelp` switch -- so `--ai` was
   inconsistent with its own neighbour.

## Decisions

- **Make `ai` a subcommand parallel to `help command`.** Canonical grammar is
  now `ze help ai [cli|api|mcp|dispatch|all] [--json]`. Sections are positional
  keywords matching the house style; `--json` stays a flag because it modifies
  *format*, not *what*. The mental model: `help`=door, `ai`=machine mode,
  section=which chapter, `--json`=rendering.
- **Keep the old form as a hidden alias, do not break it.** Shipped skills data
  (`internal/plugins/skills/data/*.md`), docs, and any agent prompt had learned
  `ze help --ai --json`. Section detection is form-agnostic (`hasSection`
  matches both `api` and `--api`), and `dispatchHelp` keeps the
  `aiHelpRequested` (`--ai`) case. So `ze help --ai --api` still resolves.
- **Discoverability is principled, not bespoke.** `ze help ai` self-lists its
  sections; `ze help` usage Examples name `ze help ai`/`ze help ai api`;
  `help ai` is registered via `MustRegisterLocalMeta` so it appears in
  completion and the help listing. A wrong guess (`ze --ai`) now dumps usage
  that names the leaf.
- **Rejected a self-correcting did-you-mean hint** for `--ai`/section tokens.
  It was redundant with the usage signpost and the wrong altitude: a per-command
  special case bolted onto dispatch instead of the generic `suggest.Command`
  engine. Per-command hints do not scale.

## Consequences

- Adding an AI-help section is a positional keyword, discoverable by tab
  completion and `ze help ai` itself, not a hidden flag.
- The grammar now matches `show ip route`-style commands, so an agent that knows
  the house style guesses it correctly.
- Gated so it cannot regress: `cmd/ze/help_ai_test.go` (form-agnostic section
  parsing + alias) and `test/ui/help-ai.ci` (canonical form, `--ai` alias,
  summary self-listing, and `ze --help` naming the leaf).

## Gotchas

- The shipped skills markdown is the documented agent entry point and embeds the
  exact command string; migrating the grammar means migrating those files too,
  or the alias must keep the old string working. We did both.
- `.ci` `expect=...:contains=` reads to end of line: quoting a value with spaces
  (`contains="help ai"`) searches for the literal quotes. Use unquoted
  (`contains=help ai`), matching the `test/traffic/*.ci` convention.
- General rule reinforced: if the agent operating the tool cannot find a feature
  from its natural first move, the feature effectively does not exist. Name
  AI-facing leaves in the usage/error surfaces, and follow the binary's own
  grammar so guesses land.

## Files

- `cmd/ze/help_ai.go` -- `hasSection`, summary/usage text, doc comment
- `cmd/ze/ze_core_dispatch.go` -- `ai` subcommand case, `help ai` registration
- `cmd/ze/ze_core_usage.go` -- usage Examples signpost
- `cmd/ze/help_ai_test.go`, `test/ui/help-ai.ci` -- gates
- `internal/plugins/skills/data/*.md`, `docs/**`, `ai/patterns/*.md` -- migrated
