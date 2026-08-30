---
kind: directive
level: MUST NOT
stage:
---
**The `--flag` form belongs to the offline `cmd/ze/` Go flag tooling that reaches no daemon, and it MUST NOT reach the YANG layer or travel from a client to the daemon.** A flag baked into a YANG description is documentation lying about structure: it is invisible to completion and dispatch, and it couples the shared model to one front-end. A filter (address family, row limit, VRF, table) is grammar, so it MUST be modelled as a keyword-value pair, and every offline flag MUST be declared through the flag registry. The vendor namespacing behind family-as-filter is on `docs/architecture/cli/command-namespacing.md`.
