---
kind: table
level:
stage:
---
| Field | Purpose | Values |
|-------|---------|--------|
| Status | Current state | `skeleton`, `design`, `ready`, `in-progress`, `verification`, `blocked`, `deferred` |
| Handoff | Who closes this spec | `verify` for the two-session handoff, `-` for closure in the same session |
| Depends | Blocking prerequisite | Spec filename (e.g., `spec-rib-04`) or `-` |
| Phase | Multi-phase progress | `N/M` (e.g., `3/5`) or `-` for single-phase |
| Updated | Date of last status change | `YYYY-MM-DD` -- NOT last file edit |
