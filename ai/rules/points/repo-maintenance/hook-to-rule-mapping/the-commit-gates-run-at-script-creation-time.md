---
kind: note
level:
stage:
---
These are NOT Claude hooks. They run when `commit_helper.py create` generates the commit script, which is the only sanctioned commit path (run the script at the path its `script=` line prints). The helper already knows the exact add/remove set of the commit, so the gates inspect that instead of the staging area. BLOCK gates raise (exit 2, no script written); WARN gates print to stderr and let the script be written.
