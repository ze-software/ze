---
kind: note
level:
stage:
---
Add a new entry to `internal/le/plugin/boundary/pluginboundary.go`'s `dangerousCalls` list whenever a new instance of this class is found and fixed, so the check stays current. Add a new `allowlist` entry only for a package's own legitimate calls to its own function.
