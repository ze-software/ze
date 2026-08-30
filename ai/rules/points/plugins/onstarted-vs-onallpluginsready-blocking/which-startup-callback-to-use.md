---
kind: directive
level: MUST
stage:
---
- **A `DispatchCommand` that targets another plugin's command at startup MUST be issued from `OnAllPluginsReady`, and MUST NOT be issued from `OnStarted`.** The engine loads plugins across up to five phases in series, so `OnStarted` fires after this plugin's own handshake and before a later phase's plugins load, and the dispatch fails with "unknown command". `OnAllPluginsReady` fires once the dispatcher command registry is frozen.
- **`OnStarted` MUST carry local setup only:** long-lived goroutines, subscriptions, per-plugin state. The phase order is `docs/architecture/plugin/plugin-system.md`, "Startup phases and callbacks".
