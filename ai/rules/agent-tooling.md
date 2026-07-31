# Agent Tooling Contract

**When:** adding or changing any CLI output, JSON envelope, diagnostic code, or repair plan that an agent consumes
**Severity:** blocking

## Directives

All agent-facing CLI output must follow these rules.

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

- **`.claude/hooks/pretool-agent-skill.py` BLOCKS the spawn** when the agent prompt asks for something a skill covers. It matches the ASK, never the subject: "review this diff" is routed, "explain how review works" is not.
- **Naming the skill in the prompt satisfies the gate**, so a subagent that must follow `/ze-explore` is spawned by saying so.
- The map it enforces: research is `/ze-explore`, review is `/ze-review`, spec conformance is `/ze-review-spec`, a red test is `/ze-debug`, spec work is `/ze-implement`, bug classes are `/ze-hunt`, spec audit is `/ze-audit`.
- A hand-written prompt reproduces a worse version of the skill and drops every gate it carries. That is what the gate exists to stop.

## Commit Script Generation

Use `scripts/dev/commit_helper.py` for commit script preparation. It owns
session ID reuse, message file creation, executable `tmp/commit-SESSION.sh`
generation, ignored-path rejection, `git commit -F`, and the learned-summary
gate for workflow/tooling/rule changes. Hand-write a commit script only when the
helper cannot express the commit shape, and keep the same generated-script
contract from `ai/rules/git-safety.md`.

On explicit commit requests, commit-helper invocation is the work. Do not run
late completeness checks, health checks, recent-commit style reviews, or
remaining-work tables unless the user explicitly asks for them. Before any
verify target, run `scripts/dev/verify-status.sh check`; a FRESH result
forbids rerunning `make ze-verify` or `make ze-verify-changed`.

## Skills

Version-matched skill content is embedded in the binary.

1. New skills go in `internal/plugins/skills/data/<name>.md` with frontmatter.
2. The skill inventory in `internal/plugins/skills/main.go` must list every embedded file.
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
4. Satisfy `ai/rules/discovery-updates.md`: add the keyword/task path in
   `ai/INDEX.md`, and document the verification target
   that proves the feature is discoverable.
