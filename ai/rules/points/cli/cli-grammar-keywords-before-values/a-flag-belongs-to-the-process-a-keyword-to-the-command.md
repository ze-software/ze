---
kind: directive
level: MUST
stage:
---
- **A `--flag` belongs to the PROCESS that runs a command; a bare keyword belongs to the COMMAND itself.** The daemon states the cut in the error it returns when a flag reaches it: `flags are interpreted by the client, not the daemon` (`firstFlagToken`, `internal/component/plugin/server/command.go`).
- **A filter names part of the question, so it MUST take the first register and never the third.** `family`, `limit`, `vrf` and `table` are keywords: `show bgp neighbor ipv6`, never `--family ipv6`.
