---
kind: note
level:
stage:
---
Central verb packages (`internal/component/cmd/show`, `internal/component/cmd/delete`, ...) keep ONLY
generic cross-system commands (`show warnings`, `show health`), never a specific
plugin's commands.
