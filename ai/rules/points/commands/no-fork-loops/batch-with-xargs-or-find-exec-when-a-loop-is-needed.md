---
kind: directive
level: MUST
stage:
---
**When the loop body genuinely needs per-file logic that one command cannot
express, it MUST be batched with `xargs` or `find -exec +` rather than forked per
file.** One recursive `grep`, one glob, or one `find -exec +` is a single fork.
The measured cost is `docs/contributing/running-commands.md`.
