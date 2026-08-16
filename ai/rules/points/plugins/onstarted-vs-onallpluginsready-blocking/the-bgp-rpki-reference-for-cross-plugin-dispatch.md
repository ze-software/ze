---
kind: note
level:
stage:
---
`bgp-rpki` is the reference example: the `adj-rib-in enable-validation` dispatch lives in `OnAllPluginsReady` (`internal/component/bgp/plugins/rpki/rpki.go`). `OnStarted` can run before `bgp-adj-rib-in` loads and fail with "unknown command".
