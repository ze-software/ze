---
kind: directive
level: MUST
stage:
---
**MUST write it under your session's own directory**: `dir=$(./le session scratch ensure)`
gives the `scratch/` subdirectory of `tmp/session/<YYYY-MM-DD>-<session-id>/`, so scratch
never collides with a sibling session (`ai/rules/commands.md`). Nothing removes
the live session's directory automatically. `./le session reap` removes only
directories whose owners are provably gone. A fixed name at the `tmp/` root is
the failure this replaces: it names the same file for every session in the
checkout. `bashScratch` and the Write/Edit scratch check in
`internal/le/hookruntime` refuse that path.
