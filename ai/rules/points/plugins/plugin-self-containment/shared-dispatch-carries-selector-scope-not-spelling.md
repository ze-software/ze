---
kind: directive
level: MUST NOT
stage:
---
- **Generic command plumbing MUST NOT carry a plugin's command spelling; it carries selector scope only.** The dispatcher MAY extract a typed selector value because a YANG `ArgDef` declares it, and it contains no plugin grammar: not `peer`, not `bgp`, not `bfd`. The classification rule is ownership before grammar.
