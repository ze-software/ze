---
kind: note
level:
stage:
---
Incident: session bgp-reconnect-flap (2026-06-27) claimed the peer reconnect loop
amplified session flaps and recommended a spec, after reading `run()` (the error
*consumer*) and assuming a clean session close returns `err == nil`. Reading
`Session.Run` (the *producer*) showed it never returns nil, so the `err == nil`
branch is dead and the claimed gap did not exist. Root cause: the keystone fact,
what `Run` returns on session end, was inferred from the caller, never read.
