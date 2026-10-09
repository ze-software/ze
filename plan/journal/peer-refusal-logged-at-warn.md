# A refusal of a peer's frame logged at Warn, once per frame

A handler that refuses a frame an unauthenticated peer can send, and logs the
refusal at Warn or above with no rate bound, hands that peer control of the log
volume. The sibling refusals in the same file usually log at Debug with a
`reason` attribute, so the outlier is the one written before that convention.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-09 | spec-pppoe-padt-ends-session | PPPoE AC discovery, `InterfaceServer.handlePADT` (`internal/component/l2tp/pppoe/server.go`) | A PADT naming a live SESSION_ID with a different source MAC logs `pppoe: PADT MAC mismatch` at Warn for every frame. Any host on the access segment can repeat it at line rate. The PADI and PADR refusals in the same file log at Debug with `reason` | not fixed: found during the closure review of spec-pppoe-padt-ends-session; the behavior predates it |
