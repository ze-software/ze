# Handovers live in the repository, never in the session

Owner decision, 2026-10-17: "handover must NEVER be in the session as it is not
something we can move machine when we change from Linux to Mac and vice-versa".

A handover, a resume point, or an agent's "stop here, continue later" notes are
read on whichever machine picks the work up next. `tmp/session/`, the scratch
directory, `~/.claude/` and any other untracked or machine-local path stay on
the machine that wrote them, so a handover written there is lost on a move.

| Do | Never |
|----|-------|
| Write it to `plan/handover/NN-<slug>.md` (rules in `plan/handover/README.md`) and commit it in the same turn it is written | Put a handover only in `tmp/session/.../state/`, a scratch file, the repo root untracked, or the chat |
| Brief every agent that may run out of budget to append its handoff to the session's `plan/handover/` file (Edit tool), not to the session state file | Rely on the session state file or a task output to carry work across machines |
| Name evidence the next machine can reach: commits, tracked files, CI run ids | Point at scratch logs as the only evidence (summarize what they showed) |

The per-session state file under `tmp/session/` may still hold local digests
for post-compaction recovery on the same machine; it is never the handover.
