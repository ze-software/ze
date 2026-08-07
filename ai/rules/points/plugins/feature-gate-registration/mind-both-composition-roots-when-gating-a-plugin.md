---
kind: note
level:
stage:
---
The plugin's `.go` files are NOT source-tagged; `make generate` moves their blank
imports into `all_<tag>.go` and dead-code elimination drops the unreferenced
packages when the tag is off (A-1: nothing always-on imports a protocol). Mind the
**two composition roots**: the generated `all.go` AND the hand-written
`cmd/ze/ze_core_dispatch.go` (CLI). Protocols with a programmatic `cli` package
(isis, ospf) move their dispatch-root CLI blank import into a per-protocol gated
companion `cmd/ze/dispatch_<proto>.go` (`//go:build ze_core && ze_<proto>`); miss
that root and the package stays linked. A plugin that registers its CLI through the
plugin registry's `CLIHandler` (not a programmatic `cli` package) has only the ONE
root -- the generated `all.go` -- and needs NO dispatch companion. The shape is not
protocol-specific: `ze_vrrp` (first-hop redundancy; the `vrrp` plugin + its `transport`
sidecar, two manifest lines, no `cli` package) is that single-plugin case, gated purely
by blank-import partitioning like ldp/rsvpte. Routing protocols are also the first gated
packages that are `sdk.NewWithConn` *engines* and multi-package features whose
sub-packages import each other, so `dep_audit.py` (a) counts the generated
`all_<tag>.go` as a registration importer (an engine's blank import there is not a
"feature depends on it" tier violation) and (b) skips same-tag importers in the
disableable check (the engine importing its own `transport` sub-package is
intra-feature -- dropped together, not an always-on pin).
