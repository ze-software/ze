| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-03-21 | - | parser | unknown keyword fell through to default branch silently | listed all valid cases explicitly and rejected unknown |
| 2026-03-21 | - | parser | parseSAFI fell through to unicast on unknown input | added explicit error for unknown SAFI |
