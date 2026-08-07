---
kind: directive
level:
stage:
---
**Why not `OnAllPluginsReady`:** `sendPostStartupToAll` fans that callback out on
detached goroutines immediately before peers start, and waiting there deadlocks
(its doc comment records the attempt). A role resolved from it races the first
session by 1-2 ms on an idle host, which inverts under load: the duplicate
peer-up replay in `plan/known-failures/bgp-plugin-rs-forward-duplicate-and-order.md`.
