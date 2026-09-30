| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-09-30 | spec-rfc-verdict-fix-bfd | BFD engine `sharedEntryLocked` | A released session (kept per RFC 5880 6.8.1, alive while its peer sends) still counts, so a narrower request matching it and a live session gets its own session instead of sharing (RFC 5882 4.4). Read-verified only | fixed 2026-09-30: `sharedEntryLocked` counts live matches first and falls back to a lone released one; `TestSharedRequestJoinsLiveSessionOverReleasedOne` (red before), `TestSharedRequestRevivesTheOnlyReleasedMatch` |
