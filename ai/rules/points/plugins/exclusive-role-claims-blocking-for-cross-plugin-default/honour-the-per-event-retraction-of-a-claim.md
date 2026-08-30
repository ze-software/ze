---
kind: directive
level: MUST
stage:
---
- **A plugin that stood a role down MUST run its own default behaviour for an event whose unheld-roles list names that role.** A claim is daemon-wide and delivery is per-peer, so the engine retracts it per event: for that peer, nothing else will do the work. An absent list means every claim holds, which is the common case.
