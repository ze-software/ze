---
kind: directive
level: MUST NOT
stage:
---
- **Infrastructure MUST NOT import a plugin implementation directly; it MUST use a registry lookup.**
- **A plugin MUST NOT import a sibling plugin package; it MUST send a text command through `DispatchCommand`.**
