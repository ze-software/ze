# 833 -- Commit Helper Tooling

## Context

Commit preparation had become repetitive and error-prone: agents had to invent or rediscover an 8-character session ID, create matching message files, remember the exact `git commit -F` script shape, avoid ignored generated files, and manually decide whether a structural change needed a learned summary. The goal was to make the safe path mechanical while keeping the user-triggered commit boundary intact.

## Decisions

- Chose a repository script (`scripts/dev/commit_helper.py`) over another prose-only rule because the failure mode was mechanical and should be prevented before the user runs anything.
- Chose persisted `tmp/commit-session-id` over per-script random IDs so all logical commits in one session naturally share the same user-run script.
- Chose separate message files plus `git commit -F` over heredocs or `git commit -m`, matching the macOS bash-safe rule and making message content inspectable.
- Chose explicit `--file` and `--remove` arguments over broad staging so the script cannot silently absorb unrelated work from shared sessions.
- Chose a lesson gate for workflow/tooling/rule paths, with either a `plan/learned/NNN-*.md` plus `.counter` or an explicit no-lesson reason, because structural changes are where missing lessons hurt future agents most.

## Consequences

- `/ze-commit`, `/ze-commit-check`, and `/ze-implement` now route commit script generation through the helper instead of asking agents to hand-roll session IDs and heredoc-free scripts.
- Generated scripts are executable, start from the repository root, use `set -euo pipefail`, add only explicit paths, use `git rm` only for tracked removals, and commit via the generated message file.
- Ignored generated paths such as `tmp/`, `AGENTS.md`, and `CLAUDE.md` are rejected before the script is written.
- Workflow/tooling/rule commits cannot pass through the helper without either adding a learned summary and counter bump or recording why no reusable lesson is useful.

## Gotchas

- The helper does not invent commit prose; the agent still drafts the subject and body from the actual changes, then asks the helper to materialize the message and script.
- `--append` is deliberate: without it, an existing `tmp/commit-SESSION.sh` is not overwritten unless `--replace` is passed.
- New learned summaries must include `plan/learned/.counter`; use `--lesson-existing` only when editing an existing learned file.
- `--repo` is a global option and must appear before the `create` or `session` subcommand.

## Files

- `scripts/dev/commit_helper.py`
- `scripts/dev/commit_helper_test.go`
- `ai/rules/git-safety.md`
- `ai/rules/cli.md`
- `ai/skills/ze-commit.md`
- `ai/skills/ze-commit-check.md`
- `ai/skills/ze-implement.md`
- `ai/INDEX.md`
- `ai/NAVIGATION.md`
- `docs/features/ai-first.md`
- `docs/contributing/claude-code-cheatsheet.md`
