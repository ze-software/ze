---
kind: directive
level: MUST
stage:
---
**A `ze.log.<subsystem>` key in a `.ci` test MUST name a real slog subsystem.**
An internal plugin's logger name is `CanonicalSubsystemName` of its registry name
(`internal/component/plugin/inprocess.go`), which turns every hyphen into a dot,
and `getLogEnv` (`internal/core/slogutil/slogutil.go`) splits the subsystem on
`.` only. So a plugin registered `bgp-adj-rib-in` reads `ze.log.bgp.adj.rib.in`;
`ze.log.bgp.adj-rib-in` matches no lookup, sets nothing, and leaves the level at
the WARN default -- with no error, which is why it has recurred three times. A
hyphen in the key is legitimate ONLY when that exact subsystem is declared
literally in Go (`slogutil.LazyLogger("bgp.filter.aspath-length")`). Enforced by
`checkLogSubsystemKeys` in `./le doc wiring`.
