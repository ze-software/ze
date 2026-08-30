---
kind: directive
level: MUST
stage:
---
**A spec whose metadata carries `| Handoff | verify |` is implemented, COMMITTED and stopped by one session, then reviewed and closed by another.** The row is declared before implementation starts; absent, or `-`, closure stays in the implementing session. The implementing session MUST set `| Status | verification |` before it commits, release the claim and stop, and it MUST NOT use that status to park unfinished work.
**The handoff commit MUST carry neither a `plan/learned/` file nor a removal of the spec.** Either makes `internal/le/commit` read it as a closure and demand the Review Gate artifact, which the implementing session MUST NOT produce over its own work.
