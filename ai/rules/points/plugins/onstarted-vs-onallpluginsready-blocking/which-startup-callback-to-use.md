---
kind: table
level:
stage:
---
| Where to put it | Rule |
|-----------------|------|
| `OnStarted(fn)` | Local setup only: start long-lived goroutines, register subscriptions, initialise per-plugin state. |
| `OnAllPluginsReady(fn)` | Any `DispatchCommand` targeting another plugin's command at startup. The callback fires via the event loop once the dispatcher command registry is frozen, so cross-plugin dispatch is guaranteed to resolve. |
