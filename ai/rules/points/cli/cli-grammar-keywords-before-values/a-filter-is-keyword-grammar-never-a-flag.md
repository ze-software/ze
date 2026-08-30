---
kind: directive
level: MUST NOT
stage:
---
A **filter** (address family, row limit, VRF, table) is grammar, so it MUST NOT be a
flag. The vendor namespacing logic behind family-as-filter (Cisco `ip` against Nokia
`router` against Juniper `show route`) is on
`docs/architecture/cli/command-namespacing.md`.
