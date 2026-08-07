---
kind: directive
level:
stage:
---
**All RPCs need YANG registration for the CLI.** Any command handler without a YANG schema is a structural issue to fix, not a different category. There is no "command module": everything with RPCs is a plugin and lives under `plugins/<name>/`.
