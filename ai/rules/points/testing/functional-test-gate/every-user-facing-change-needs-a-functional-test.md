---
kind: directive
level: MUST
stage:
---
When you add or change user-facing behavior, a corresponding functional test MUST
exist in the correct `test/` directory. "User-facing" means: reachable via CLI command,
config option, API call, web endpoint, plugin event, or wire protocol exchange.
