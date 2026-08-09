---
kind: note
level:
stage:
---
Generic command plumbing carries **selector scope**, not command spelling.
The dispatcher may extract a typed selector value because a YANG `ArgDef`
declares it (`internal/component/plugin/server/command.go`), but it must not
contain the words `peer`, `bgp`, `bfd`, or any plugin's grammar. The
classification rule is ownership before grammar: shared dispatch may carry
selector scope; it must not own a plugin's command spelling.
