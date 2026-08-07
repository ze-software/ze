---
kind: note
level:
stage:
---
Stages 1-5 run per-phase. The engine loads plugins across up to five phases
(config-path auto-load -> explicit -> family -> event-type -> send-type) serially,
so a plugin's `OnStarted` fires after its own handshake but potentially before
plugins in later phases are loaded.
