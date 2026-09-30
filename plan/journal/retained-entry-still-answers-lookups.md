| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-09-30 | spec-rfc-verdict-fix-bfd | BFD engine `sharedEntryLocked` | A released session (kept per RFC 5880 6.8.1, alive while its peer sends) still counts, so a narrower request matching it and a live session gets its own session instead of sharing (RFC 5882 4.4). Read-verified only | open: count live entries first, fall back to a released one |
