# Agent Tooling Contract

**BLOCKING.** All agent-facing CLI output must follow these rules.

## JSON Output Contract

Every new command that produces JSON output for agents must:

1. Use `encoding/json`, never string concatenation.
2. Use lower kebab-case keys (per `json-format.md`).
3. Include `schema-version` in top-level envelopes via `diagnostic.NewValidateResult` or `diagnostic.NewFixPlan`.
4. Emit no ANSI escape sequences when stdout is not a terminal.

## Diagnostic Codes

Every validation error surfaced to agents must carry a stable diagnostic code.

1. Codes are lower-kebab: `config-parse`, `config-yang-type`, etc.
2. Every code must be registered in `internal/core/diagnostic/codes.go` with title, description, and related codes.
3. New validation stages must map errors to diagnostic codes, not pass raw strings.
4. The `ze explain` command must return an explanation for every registered code.

## Repair Plans

Repair metadata is plan-only. Commands must never edit config files.

1. Every repair carries a stable `id` (lower-kebab) and `summary`.
2. Safety labels: `format-only`, `section-local`, `behavior-preserving`, `api-changing`, `target-changing`, `requires-human-review`.
3. If Ze cannot prove a repair is safe, use `requires-human-review` with id `manual-review`.

## Prefer Skills Over Raw Agents

When a skill covers the task (`/ze-rfc`, `/ze-review`, `/ze-implement`, etc.),
use it instead of spawning a raw agent or improvising the workflow. Skills
encode project conventions, gates, and ordering that a raw agent will miss.

## Skills

Version-matched skill content is embedded in the binary.

1. New skills go in `cmd/ze/skills/data/<name>.md` with frontmatter.
2. The skill inventory in `cmd/ze/skills/main.go` must list every embedded file.
3. `ze skills list` must show all bundled skills without a static list elsewhere.

## Validation at Boundaries

Config validation must run at every boundary where config enters the system:

1. `ze config validate` (CLI).
2. `ze config fix --plan --json` (CLI agent surface).
3. Web commit (pre-commit validation of pending changes).
4. Hub API config push (`ValidateContent`).
5. `ze config validate --pending` (validate zefs pending config without committing).

All boundaries use the same diagnostic pipeline.

## Documentation

When adding agent-facing features:

1. Update `docs/features/ai-first.md` with the new command or contract.
2. Update `docs/guide/mcp/overview.md` if MCP users need to discover the feature.
3. Update embedded skill files if the feature changes the agent workflow.
