# A refusal of a peer's frame logged at Warn, once per frame

A handler that refuses a frame an unauthenticated peer can send, and logs the
refusal at Warn or above with no rate bound, hands that peer control of the log
volume. The sibling refusals in the same file usually log at Debug with a
`reason` attribute, so the outlier is the one written before that convention.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-09 | spec-pppoe-padt-ends-session | PPPoE AC discovery, `InterfaceServer.handlePADT` (`internal/component/l2tp/pppoe/server.go`) | A PADT naming a live SESSION_ID with a different source MAC logs `pppoe: PADT MAC mismatch` at Warn for every frame, letting any host spam logs at line rate | fixed: PADT MAC mismatch now logs at Debug with reason `padt-mac-mismatch` behind a per-server fixed-window rate limiter; proven by `TestHandlePADTMACMismatchLogsDebugRateLimited` |
