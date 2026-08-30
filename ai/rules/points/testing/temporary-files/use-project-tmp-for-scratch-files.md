---
kind: directive
level: MUST
stage:
---
- **A scratch file MUST go under this session's own directory, and the system `/tmp` MUST NOT be used.** `dir=$(./le session scratch ensure)` prints the `scratch/` subdirectory of `tmp/session/<YYYY-MM-DD>-<session-id>/`. A fixed name at the `tmp/` root is the failure this replaces: it names the same file for every session in the checkout. `bashScratch` and the Write/Edit scratch check in `internal/le/hookruntime` refuse that path.
