# Deferrals: radius-chap-eap-admin

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/deferral-tracking.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-16 | docs/features.md RADIUS row (pre-existing text, surfaced by an edit to the same line) | CHAP/EAP admin authentication, and admin-session accounting, for the RADIUS admin AAA path. Today only PAP is implemented (`radius/authenticator.go` `Authenticate`) | Pre-existing scope statement, not deferred by this session: the 2026-07-16 edit to that row only added the "Accept resolving to no profile is rejected" clause. Recorded because the deferral gate correctly flags the line, and the work genuinely has no home | `plan/spec-radius-chap-eap-admin.md` (skeleton; RADIUS admin AAA is `Partial` by design, so the spec records a known gap rather than a defect. Lowest priority of the four opened 2026-07-16; demand-driven, pick it up when an operator needs CHAP/EAP) | deferred |

