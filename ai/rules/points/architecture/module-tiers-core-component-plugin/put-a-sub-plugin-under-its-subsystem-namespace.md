---
kind: directive
level: MUST
stage:
---
**A sub-plugin of an existing subsystem (e.g. a BGP capability or NLRI codec) MUST go under that subsystem's own plugin namespace (`internal/component/bgp/plugins/<x>`), not at the top level. Those nested namespaces are listed in the generator's `pluginDirs` (`internal/le/plugin/imports/pluginimports.go`).**
