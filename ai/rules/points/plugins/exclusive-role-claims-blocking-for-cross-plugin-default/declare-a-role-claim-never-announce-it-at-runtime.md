---
kind: note
level:
stage:
---
When plugin A takes over a role that plugin B performs by default, B must learn
that BEFORE it can receive its first runtime event. **Declare it; never dispatch
a command at `OnAllPluginsReady` to announce it.**
