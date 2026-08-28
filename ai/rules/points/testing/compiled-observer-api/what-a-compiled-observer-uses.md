---
kind: note
level:
stage:
---
Compiled `.ci` observers live under `internal/test/fixture`. They use
`pkg/plugin/sdk` for the five-stage plugin protocol and the local `fixture`
package for registration, dispatch, polling, and failure reporting.
