# A field is read after the lock that guards it is released

Code copies a pointer out of a structure under the lock that guards it, releases
the lock, then reads the fields of the object the pointer names. A writer that
changes those fields under the same lock races every such read, so `-race` fails
the reader's package and an unsynchronized reader can see a half-written value.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-11-12 | spec-ipsec-dual-form-receive-spd-bypass (found) | `PeerSession.Info` (`ike/engine/reconcile.go`), via `PeerInfoMap` | `Info` copies `ps.childSA` under `ps.mu`, unlocks, then reads the Child SA's fields; `migrateMobikePair` (`engine/mobike.go`) writes its `LocalAddr`, `RemoteAddr` and `UDPEncap` under `ps.mu`, so a listing during a MOBIKE move races it | open: read the fields inside `ps.mu`, as `espFormSharedSPIPeer` does, with a race-detector test |
