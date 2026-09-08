# A mirrored field asserts something about the sender

A test peer, a relay or a proxy answers a message by copying it and editing a
few fields. Every field that describes the SENDER is then wrong by construction,
and the only question is whether anything checks it. A checked field fails
loudly. An unchecked one produces a coverage hole instead: the feature
negotiates to nothing, or the receiver acts on its own value believing it is the
peer's, and no test is red. The tell is a reply built by copying a request.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-09-08 | spec-test-peer-open-inherits-zes-identity | `ze-peer` OPEN, capabilities 64, 70/71, 75 and 76, and the Hold Time | `buildOpen` (`internal/test/peer/open.go`) resolves the AS, the BGP Identifier, the Role (9), the ADD-PATH directions (69) and the FQDN (73), and mirrors every other capability. Five sender-describing values are still mirrored. Graceful Restart is the one verified at its producer: `Negotiate` (`internal/core/bgp/capability/negotiated.go`) stores the REMOTE speaker's capability whole, and `runPeer` (`internal/component/bgp/reactor/peer_run.go`) feeds `neg.GracefulRestart.RestartTime` into `startEORTimer`, so ze times the peer's restart by ze's OWN configured restart time. LLGR stale time (70/71), software version (75, so ze-peer reports ze's build as its own), PATHS-LIMIT (76) and the two-octet Hold Time are the same shape and are NOT producer-verified here | not fixed. The spec's goal statement enumerates the AS, the identifier, the role, the direction and the name, and no acceptance criterion reaches these five. Changing the restart time changes what `startEORTimer` waits for in every graceful-restart test, which is a behavior change no acceptance criterion authorizes |
