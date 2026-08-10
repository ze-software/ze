---
kind: directive
level: SHOULD
stage:
---
**Extract-then-gate at subsystem scale (`ze_bgp`).** Gating the BGP subsystem (~59 manifest lines: the whole `internal/component/bgp` tree plus `internal/plugins/flowspec-firewall`) is the same blank-import partitioning, but the one invariant does NOT hold going in: 27 always-on files imported a bgp package. Three techniques clear them, in this order of preference. Implementers SHOULD aim for the FEWEST source-tagged files, not the fewest edits:
