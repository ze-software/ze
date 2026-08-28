---
kind: note
level:
stage:
---
The session and lifecycle actions remain separate from the four registered check groups because they return hook protocol output directly rather than a check verdict. They still run in the native process through `runLifecycleHook`; `nativeHookActions` owns the Bash, Write/Edit, post-write, and Task/Agent check rosters.
