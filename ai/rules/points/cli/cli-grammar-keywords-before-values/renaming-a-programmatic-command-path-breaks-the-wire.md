---
kind: note
level:
stage:
---
That is fine for a command an operator types. It is a **wire break** for any command a
plugin or script sends by its bare path, over the plugin CLI protocol
(`dispatch-command` / `dispatch-command-args`) or an interactive `ze <subsystem> plugin cli`
session. A verb-first "rename" of such a command is a protocol break, not a cosmetic
change.
