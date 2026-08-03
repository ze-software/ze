# 1009 -- selected-spec-to-per-session-marker

## Context

Spec selection used to be tracked in a single shared file `tmp/session/selected-spec`
(one spec filename per line), with a hand-discipline rule: append your line when you
pick a spec, remove it after writing the learned summary. In the normal Ze workflow
of many agents editing main concurrently this never worked: the file was usually empty
despite many in-progress specs, sessions died without removing their lines (a stale
line pointing at a nonexistent spec was found in a release audit, RA-PROC-001), and the
heuristic that claimed a spec from the shared list into a per-session marker was fragile
(documented in plan/learned/438). The real per-session state already lived in the
`tmp/session/.session-<SID>` markers; `selected-spec` was a redundant, manually
maintained seed for them. Its one enforcement consumer, the `check_spec_audit` commit
gate, only fired on a literal `git commit` from Bash, which `check_destructive_git`
blocks first, so it ran on a dead path (plan/deferrals.md). Goal: remove the shared
file and its bookkeeping rule without losing per-session recovery or auto status
transition.

## Decisions

- Replaced the shared `selected-spec` file with per-session claims into each session's
  OWN marker, over keeping a shared list: no two sessions write the same path, so there
  is no append/remove discipline and no concurrency races.
- Added `scripts/dev/spec-session.sh` (`claim`/`current`/`release`) that sources the
  existing `.claude/hooks/lib/state-file.sh`, over reimplementing SID derivation: a claim
  written by the AI's Bash call reads back identically in the hooks because both derive
  the SID from `CLAUDE_CODE_SESSION_ACCESS_TOKEN` (or the same process-tree walk).
- Moved the `ready -> in-progress` auto-transition out of `session-start.sh` (a start-of-
  session heuristic) and into `spec-session.sh claim`, over leaving it at session start:
  the transition now fires exactly when work begins, which is also more correct.
- Removed the dead `check_spec_audit` gate (and its only-helpers `_sed_range`,
  `_count_rows`) from `pretool-bash.py`, over leaving dead code in place.
- Dropped the `- Spec:` line from `subagent-context.sh`, over trying to propagate it: a
  subagent may not share the parent's SID, and it receives its task explicitly anyway.

## Consequences

- A brand-new session has no claimed spec until a skill (ze-spec on create, ze-implement
  on start) runs `spec-session.sh claim`; the marker is no longer seeded at session start.
- Per-session state-file naming and post-compaction recovery still work, driven by the
  marker the helper writes.
- Skills call the helper instead of reading a file; the friction (an unmaintained shared
  file) is replaced by a claim wired into skill steps the agent already follows.
- Historical records that mention `selected-spec` (closed specs, deferrals.md,
  learned/438, the release audit) were left intact as records of past state.

## Gotchas

- `spec-session.sh` must NOT use `set -u`: the shared `session-id.sh` reads
  `$CLAUDE_CODE_SESSION_ACCESS_TOKEN` without a default, and the hooks never run under
  `set -u`. Using it makes every SID lookup fail. Use `set -eo pipefail`.
- `spec-session.sh claim` has a real side effect (it edits the spec's Status/Updated).
  Testing `claim` against a real `ready` spec mutates it; revert the spec afterward.
- The skill mirrors (`.claude`/`.codex`/`.agents/skills`, `CLAUDE.md`, `AGENTS.md`) are
  gitignored and generated from `ai/skills/*.md`; edit the canonical source and run
  `make ze-ai-sync`, never the mirrors, and never add the mirrors to a commit.

## Files

- `scripts/dev/spec-session.sh` (new): per-session claim/current/release helper
- `.claude/hooks/session-start.sh`: read this session's marker instead of the shared file
- `.claude/hooks/compaction-reminder.sh`: read marker via `_session_id`
- `.claude/hooks/subagent-context.sh`: dropped the spec line
- `.claude/hooks/pretool-bash.py`: removed dead `check_spec_audit` + helpers
- `.claude/rules/planning.md`, `session-start.md`, `post-compaction.md`: helper-based steps
- `ai/rationale/session-start.md`, `ai/rules/architecture.md`: updated references
- `ai/skills/ze-spec.md`, `ze-implement.md`, `ze-status.md`, `ze-progress.md`,
  `ze-audit.md`, `ze-debrief.md`, `ze-handoff.md`, `ze-review-spec.md`, `ze-review-deep.md`
- `plan/README.md`: updated tracking description
