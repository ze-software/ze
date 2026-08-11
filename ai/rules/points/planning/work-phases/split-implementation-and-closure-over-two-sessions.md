---
kind: directive
level: MUST
stage:
---
- **A spec whose metadata carries `| Handoff | verify |` is implemented, COMMITTED and stopped by one session, then reviewed and closed by another.** The row is declared before implementation starts, because not every spec is worked this way. Absent, or `-`, closure stays in the implementing session and nothing below applies.
- **The implementing session MUST set `| Status | verification |` before it commits, and MUST stop after the commit.** That status says the code is written, tested and in git, and that the spec awaits an independent review. It MUST NOT be used to park unfinished work: every acceptance criterion is implemented and green first (`ai/rules/completion.md`).
- **The handoff commit MUST carry neither a `plan/learned/` file nor a removal of the spec.** Either one makes `commit_helper.py` read it as a closure commit and demand the Review Gate artifact, which the implementing session MUST NOT produce over its own work.
- **This mode serves review INDEPENDENCE (above), and that is the only thing it buys.** The reviewing session reads a committed diff it did not write. A same-session close cannot give that.
