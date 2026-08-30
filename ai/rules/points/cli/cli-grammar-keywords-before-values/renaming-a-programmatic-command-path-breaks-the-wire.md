---
kind: directive
level: MUST
stage:
---
A rename is harmless for a command an operator types. It is a **wire break** for any
command a plugin or script sends by its bare path, over the plugin CLI protocol
(`dispatch-command` / `dispatch-command-args`) or an interactive
`ze <subsystem> plugin cli` session. A verb-first rename of such a command MUST be
treated as a protocol break, not a cosmetic change.
