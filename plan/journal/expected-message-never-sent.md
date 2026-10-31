# A message the peer waits for is never sent

The session starts and nothing errors, but a message the protocol or the test
expects next never reaches the wire, so the run ends on a timeout that names
no cause.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-31 | route-policy-0-umbrella (found, not caused) | `test/plugin/rpki-aspa-*`, all 7, QEMU netns on `claude` | Each times out at 25s with `received messages: 0` while the test peer waits for Ze's EOR (message 1); the mock RTR and peer start and Ze logs no ERROR. Red alike at 2e08f27eb5 and at 7c45216f1e (before route policy, booted on its own 7.2.9 kernel) | open: cause not traced; read Ze's debug log for whether the session reaches Established and why no EOR follows (the peers carry no export policy, so check RFC 8212 default reject's effect on EOR), fix it, then rerun the 7 |
