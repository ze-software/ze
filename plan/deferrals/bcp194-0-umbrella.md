# Deferrals: bcp194-0-umbrella

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/planning.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-08-08 | spec-bcp194-0-umbrella (Task, Key Design Decisions) | TCP-AO (RFC 5925) for BGP sessions, which RFC 7454 §5.1 prefers over TCP MD5 | The recommendation is conditional on the implementation existing, and Ze has none. It is a feature rather than a conformance gap, and it is large enough to need its own spec before it can be an obligation. Folding it into child 3 would put a new authentication mechanism inside a session-hardening spec | needs a destination spec | deferred |
| 2026-08-08 | spec-bcp194-0-umbrella (Known Limitations) | RFC 8195 Functions 5, 7 and 8 to 12 | The LOCAL_PREF family is a separate design problem, and both §4.3.3 and RFC 4264 warn about it. Child 1 covers Functions 1 to 4 and 6 | needs a destination spec | deferred |
