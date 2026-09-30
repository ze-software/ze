| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-09-30 | spec-rfc-verdict-fix-bgp (walked into fixing RFC4271-8.2.2-10) | `handlePendingCollision` (`internal/component/bgp/reactor/reactor_connection.go`) | A pending colliding connection whose first header fails `ParseHeader` (bad marker, Length < 19) is closed with Cease 6/7 via `rejectConnectionCollision`, not the RFC 4271 Section 6.1 code 1/1 or 1/2 the session read paths send. | Answer a ParseHeader error there with the Section 6.1 code before closing. |
