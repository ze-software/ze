---
kind: directive
level: MUST
stage:
---
- **Every flag an offline command accepts MUST be declared once through `registry.RegisterCommandFlags` (`internal/component/command/registry/flags.go`).** A flag declared only in `Meta.Subs` prose is invisible to completion, and prose drifts from the parser in both directions: a flag the handler parses and the help never names, and a flag the help names and the handler never reads.
