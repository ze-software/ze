# 832 -- Self-Improving Discovery

## Context

Ze had several discovery mechanisms already: generated command inventories, documentation drift checks, changed-file wiring gates, and learned summaries. The gap was policy cohesion: future agents could add a feature, tool, self-check, verification gate, or test runner without updating the route that lets the next agent find and use it. The goal was to make discoverability itself a required deliverable, then review recent work so structural additions landed in the lookup surfaces.

## Decisions

- Chose one blocking discovery rule over scattered reminders because every feature/tool/check/test change needs the same decision tree: update docs, rules, indexes, and verification paths.
- Chose to link through `ai/INDEX.md`, `ai/NAVIGATION.md`, and existing rules over creating a standalone doc because a rule that is not on the agent's normal path is effectively invisible.
- Chose registry-backed/generated inventory targets over static lists because command, plugin, YANG, and documentation surfaces already have drift checks that can prove discoverability.
- Chose to index recent structural lessons directly over relying on numbered learned files alone because keyword lookup is what prevents agents from repeating command, pipe, raw JSON, RIB, plugin, and auth mistakes.

## Consequences

- Any future feature, tooling, self-check, verification, testing, or agent workflow change must identify where an agent looks first, which rule prevents drift, which source of truth is generated, and which check proves it.
- `make ze-verify-wiring-docs` is now documented as the changed-file-aware wiring/doc/inventory gate that backs the discovery policy during normal verification.
- Agent-facing docs now distinguish runtime AI self-description from development-time discovery; the binary describes commands, while repository docs describe tools, checks, rules, and workflows.
- Learned summaries for structural changes must be indexed in `ai/LEARNED-INDEX.md` when they affect future design choices, not just written to `plan/learned/`.

## Gotchas

- Generated agent instruction files are ignored and must be regenerated from `ai/INSTRUCTIONS.md`; do not add `AGENTS.md` or `CLAUDE.md` to commit scripts.
- The discovery gate can expose stale generated plugin imports even in documentation work; run `make generate` and keep the check read-only.
- A check target is not discoverable until its owning docs, rule mapping, and keyword/navigation entries point to it.
- Commit scripts must add the learned summary and bump `.counter`; the learned file alone is not enough.

## Files

- `ai/rules/repo-maintenance.md`
- `ai/INSTRUCTIONS.md`
- `ai/INDEX.md`
- `ai/NAVIGATION.md`
- `ai/LEARNED-INDEX.md`
- `ai/rules/cli.md`
- `ai/rules/architecture.md`
- `ai/rules/writing.md`
- `ai/rules/repo-maintenance.md`
- `ai/rules/testing.md`
- `docs/contributing/documentation-testing.md`
- `docs/features/ai-first.md`
- `scripts/codegen/plugin_imports_test.go`
