---
kind: directive
level: MUST NOT
stage:
---
- **A role MUST NOT be resolved from `OnAllPluginsReady`.** `sendPostStartupToAll` fans that callback out on detached goroutines immediately before peers start, so waiting there deadlocks, and a role resolved from it races the first session by 1 to 2 milliseconds on an idle host, which inverts under load.
