---
kind: directive
level: MUST
stage:
---
**A claim is daemon-wide and delivery is per-peer, so the engine retracts it per
event.** Each peer-scoped event carries the claimed roles that NO process being
fed that event holds (`UnheldRoles`): `StructuredEvent.UnheldRoles` on the direct
bridge, the `unheld-roles` member of the JSON event otherwise. A plugin that
stood a role down MUST run its own default behaviour for an event that names the
role, because for that peer nothing else will. An absent list means every claim
holds, which is the common case.
