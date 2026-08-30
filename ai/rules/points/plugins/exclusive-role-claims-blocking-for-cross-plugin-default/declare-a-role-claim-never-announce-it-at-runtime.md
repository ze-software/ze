---
kind: directive
level: MUST
stage:
---
- **When plugin A takes over a role plugin B performs by default, A MUST declare it as `Claims` in its `registry.Registration`, and MUST NOT announce it by dispatching a command at runtime.** B has to learn the claim before its first runtime event, and the engine delivers the union of the startup set's claims on every plugin's stage-2 configure callback. B reads `sdk.Plugin.ClaimActive("<role-token>")` from `OnConfigure`. The resolution mechanism is `docs/architecture/plugin/plugin-system.md`, "Exclusive role claims".
