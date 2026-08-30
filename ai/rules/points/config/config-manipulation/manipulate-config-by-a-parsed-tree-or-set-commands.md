---
kind: directive
level: MUST
stage:
---
**Config content MUST be manipulated through one of two methods only: a parsed YANG tree when a loaded tree is in memory, or set command lines when building or merging config text.** Why concatenating two valid config texts is itself valid is `docs/architecture/config/yang-config-design.md`.
