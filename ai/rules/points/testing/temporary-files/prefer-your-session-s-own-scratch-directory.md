---
kind: directive
level: MUST
stage:
---
**MUST write it under your session's own directory**: `dir=$(scripts/dev/session-scratch.sh)`
gives the `scratch/` subdirectory of `tmp/session/<YYYY-MM-DD>-<session-id>/`, so scratch
never collides with a sibling session (`ai/rules/commands.md`). Nothing removes it for you:
the date in the directory name is what lets the operator find it later, with
`make ze-session-clean BEFORE=<YYYY-MM-DD>`. A fixed name at
the `tmp/` ROOT is the failure this replaces: `tmp/` is keyed per checkout, so
`tmp/out.log` names the same file for every session in it.
A file at that root is refused: `check_scratch_path`
(`.claude/hooks/pretool-bash.py`) on a redirect, `c_scratch_path_we`
(`.claude/hooks/pretool-writeedit.py`) on Write and Edit.
