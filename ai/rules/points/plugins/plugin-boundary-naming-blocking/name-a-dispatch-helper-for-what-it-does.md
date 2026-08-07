---
kind: note
level:
stage:
---
When a plugin sends commands to the engine via DispatchCommand, name the helper
for what it does (dispatch a command), not where it sends it (to a specific plugin).
The engine routes commands by prefix, so the caller must not encode the destination
in function names, variable names, or type names.
