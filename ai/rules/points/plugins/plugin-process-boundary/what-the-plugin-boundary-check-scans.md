---
kind: note
level:
stage:
---
`./le plugin boundary check` (wired into `./le verify current mode full`/`./le verify current mode changed`) runs `internal/le/pluginboundary/pluginboundary.go`: it scans every package under the generator's plugin search roots, derived at runtime from `internal/le/pluginimports/pluginimports.go`'s `pluginDirs` + `nestedPluginDomains` (13 namespaces today, including `internal/component/l2tp/plugins/` and `internal/component/firewall/plugins/`), never a second hardcoded list, for calls to a maintained dangerous-call list, and fails if a plugin package contains one with no `.IsInternal()`/`warnIfExternal(` call anywhere in that same package. `--print-roots` shows the derived set. This is a presence heuristic (it does not prove the guard actually covers the call at runtime), the same rigor level as the sibling `ze-iface-resolution-check`.
<!-- source: internal/le/pluginboundary/pluginboundary.go -- loadScanRootsFrom -->
