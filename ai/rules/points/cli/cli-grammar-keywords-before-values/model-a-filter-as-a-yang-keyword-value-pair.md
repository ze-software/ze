---
kind: directive
level: MUST
stage:
---
- MUST model a filter as a container or leaf the command consumes as `keyword value`
  (`... arp family ipv6`, `... route limit 50`). It is then visible to
  completion and RPC dispatch, which are built from the YANG tree.
- A `description` states what the leaf MEANS. It MUST NOT prescribe a CLI spelling
  ("Filter by address family", not "Filter with `--family`"; not "as a positional
  argument" either).
