# 876 -- commit_helper session ID was shared across concurrent Claude sessions

## Context

Two Claude sessions worked in the same checkout (2026-06-10). Session A
prepared `tmp/commit-9651503c.sh` with three commits. Session B then ran
`commit_helper.py session`, read the SAME `tmp/commit-session-id`, and its
`create --replace` rewrote `tmp/commit-9651503c.sh` with B's commits. The
user ran the script expecting A's commits and got B's; A's commits were
silently gone from the script (the message files survived because tag
suffixes kept incrementing). The second run aborted with "no changes added
to commit" because B's files were already committed.

## Lesson

A "session" file in shared `tmp/` is not a session file unless it is keyed
by the session. Any per-session artifact created by tooling that multiple
agents invoke concurrently (scripts, locks, scratch state) must embed a
session-unique component in its PATH, not rely on whoever wrote last.

Fix: `commit_helper.py session_id` now stores the ID in
`tmp/commit-session-id-<claude-session-fingerprint>`. The fingerprint is
the access-token `session_id` claim when available, else the PID of the
`claude` ancestor process (the same derivation `.claude/hooks` use for
`tmp/session/session-state-<SID>.md`), else the parent PID. Each session
therefore resolves to its own `tmp/commit-<SESSION>.sh`.

## Symptom catalogue (for recognition)

- User runs your prepared commit script; the commit that lands carries
  another session's subject and files.
- Re-run reports "no changes added to commit" while your files are still
  modified-unstaged: the script on disk is no longer yours -- diff it.
- Related but distinct: a concurrent session's plain `git commit` sweeping
  YOUR `git add`-ed files (shared index). That one is mitigated by scoped
  `git add` lists inside single user-run scripts; this lesson is about the
  script PATH itself colliding.

## Files

- `scripts/dev/commit_helper.py` -- `claude_session_fingerprint`, `session_id`
- `.claude/hooks/pretool-writeedit.py:session_id` -- the reference derivation
- `ai/rules/git-safety.md` -- commit workflow step 1 wording
