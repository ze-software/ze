---
kind: directive
level: MUST
stage:
---
- **A feature-gated file is still an always-on pin for a DIFFERENT gate, so gated files MUST be cleared too, not only untagged ones.** `dep_audit.file_requires_tag` is per-tag: a `ze_ssh`-on, `ze_bgp`-off build genuinely fails to compile when a `ze_ssh` file imports `bgp/config`.
- **After deleting an always-on import, what that package's `init()` was providing MUST be checked.** Removing the import can unlink an `init()` nobody else pulls in. A `package main` root can never be imported back, so linking it from `cmd/ze/dispatch_<x>.go` is always a safe edge.
