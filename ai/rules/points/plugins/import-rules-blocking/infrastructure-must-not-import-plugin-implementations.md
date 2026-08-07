---
kind: note
level: MUST NOT
stage:
---
Infrastructure MUST NOT import plugin implementations directly -- use registry lookups.
Plugins MUST NOT import sibling plugin packages -- use text commands via DispatchCommand.
