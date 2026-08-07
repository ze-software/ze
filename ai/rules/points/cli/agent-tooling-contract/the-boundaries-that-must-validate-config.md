---
kind: directive
level:
stage:
---
1. `ze config validate` (CLI).
2. `ze config fix --plan --json` (CLI agent surface).
3. Web commit (pre-commit validation of pending changes).
4. Hub API config push (`ValidateContent`).
5. `ze config validate --pending` (validate zefs pending config without committing).
