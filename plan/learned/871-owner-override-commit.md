# 871 -- Owner override for commit verification

## Context
An OpenAI session blocked Thomas from preparing a commit script because `ze-verify` was stale and a verify run had been cancelled. Thomas owns the repository and wanted a user-run commit script without waiting for the long gate. The agent treated an agent safety rule as if it constrained the owner, which turned a useful guardrail into friction.

## Decisions
- Added a Thomas Owner Override section to `ai/rules/git-safety.md`.
- The override requires an explicit instruction to both prepare a commit and skip tests or verify. Urgency alone is not enough.
- The override affects commit-script preparation only. Agents still must not run prohibited git commands from tools.
- Generated scripts still use the normal `scripts/dev/commit_helper.py` path and must not disable hooks, signing, or verification hooks with `--no-verify` or `--no-gpg-sign`.
- The rule records that it was added for OpenAI behavior, not for Anthropic, so future friction is attributed correctly.

## Consequences
- When Thomas explicitly invokes the override, agents stop running late gates and prepare the user-run commit script with a clear `Verification skipped by Thomas owner override` note.
- Agents must not claim skipped tests, lint, `ze-verify`, or integrations passed.
- The existing safety boundary remains: no direct `git add`, `git commit`, `git rm`, `git stash`, or destructive git command from an AI tool.

## Gotchas
- Owner override is not a general permission to bypass hooks. It only bypasses the agent-side requirement to run a fresh gate before creating the script.
- Do not argue after the override is explicit. Stage only the requested files and report the skip reason.
- This is a project-wide rule because shared behavior belongs in `ai/rules/`, even though the incident came from OpenAI.

## Files
- `ai/rules/git-safety.md`
- `ai/INDEX.md`
- `ai/LEARNED-INDEX.md`
- `plan/learned/871-owner-override-commit.md`
