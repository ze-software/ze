---
kind: directive
level: MUST
stage:
rationale: ai/rationale/testing.md
---
- **A `ze.log.<subsystem>` key in a `.ci` test MUST name a real slog subsystem.** An internal plugin's logger name is `CanonicalSubsystemName` of its registry name (`internal/component/plugin/inprocess.go`), which turns every hyphen into a dot, and `getLogEnv` (`internal/core/slogutil/slogutil.go`) reads the key through `env.Get`, which treats a dot, an underscore and a hyphen as one separator (`normalize`, `internal/core/env/env.go`). So a plugin registered `bgp-adj-rib-in` reads `ze.log.bgp.adj.rib.in`, `ze.log.bgp.adj-rib-in` and `ZE_LOG_BGP_ADJ_RIB_IN` alike. A key that names no subsystem in any spelling sets nothing and leaves the level at the WARN default with no error. `checkLogSubsystemKeys` (`internal/le/doc/wiring/checks.go`) refuses a hyphenated key whose reading matches no subsystem the Go source declares.
