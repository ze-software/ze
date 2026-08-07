---
kind: note
level:
stage:
---
`bgp-rpki` is the reference example: the `adj-rib-in enable-validation` dispatch lives in `OnAllPluginsReady` (`internal/component/bgp/plugins/rpki/rpki.go`). Putting it in `OnStarted` used to fail with "unknown command" whenever `bgp-adj-rib-in` loaded in Phase 2 while bgp-rpki auto-loaded in Phase 1.
