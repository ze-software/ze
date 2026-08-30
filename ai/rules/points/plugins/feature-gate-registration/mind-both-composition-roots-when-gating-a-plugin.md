---
kind: directive
level: MUST
stage:
---
- **BOTH composition roots MUST be minded when gating a plugin: the generated `all.go` AND the hand-written `cmd/ze/ze_core_dispatch.go`.** A protocol with a programmatic `cli` package MUST move its dispatch-root blank import into a per-protocol gated `cmd/ze/dispatch_<proto>.go`; miss that root and the package stays linked. A plugin that registers its CLI through the registry's `CLIHandler` has only the ONE root and needs no dispatch companion.
