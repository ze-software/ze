---
kind: note
level:
stage:
---
`plan/deferrals/` -- a sharded directory, **one file per source**, holding all
deferred work. There is NO single `plan/deferrals.md` and no committed aggregate: <!-- doc-links: ignore (the single file is deliberately retired) -->
the live backlog is a fold over the directory, computed on read (`/ze-status`) and
never stored. A stored aggregate would be a shared file every session appends to,
exactly the cross-commit hazard this layout removes (`ai/rules/git-safety.md`).
