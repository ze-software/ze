---
kind: directive
level: MUST
stage:
---
- **A new instance of this class MUST be added to `dangerousCalls` in `internal/le/plugin/boundary/pluginboundary.go` when it is found and fixed, so `./le plugin boundary check` stays current.** An `allowlist` entry MUST cover only a package's own legitimate calls to its own function.
