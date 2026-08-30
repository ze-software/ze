---
kind: directive
level: MUST
stage:
---
- **Before changing any registered name (plugin name, subsystem name, log subsystem, dispatch key, command prefix, family canonical name), EVERY consumer of that name MUST be grepped.** A registered name is not a single string, and most of its consumers are loose strings that no compiler catches. The consumer list and the grep to run are `docs/architecture/plugin/plugin-system.md`, "A registered name lives in many loose strings".
