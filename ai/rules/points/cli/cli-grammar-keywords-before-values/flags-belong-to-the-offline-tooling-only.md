---
kind: directive
level: MUST NOT
stage:
---
The `--flag` form (`--json`, `--socket`, `--limit`) is a presentation artifact of the
offline `cmd/ze/` Go flag tooling (`flag.NewFlagSet`) and it MUST NOT reach the YANG
layer. A `--flag` baked into a YANG description is documentation lying about
structure: it is invisible to completion and dispatch, and it couples the shared model
to one front-end.
