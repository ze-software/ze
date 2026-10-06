# A gate counts a population without the attribute that splits it

A check totals rows to compare against a published figure, but the rows carry
an attribute (an obligation level, a role) that the published figure splits on.
The total is then wrong for every row whose attribute differs, and authors route
around the check instead of recording the row honestly.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-09-30 | rfc-verdict-fix-access | `checkGapCountAgreement` (`internal/le/rfc`), via `./le rfc check` | It counts every `{gap}` row as a MUST-level gap whatever the row's level, so a SHOULD gap (RFC3579-3-2) breaks the Support cell's MUST count; an author disclosed the gap only in prose to avoid the refusal. Committed in 189db75308 with the cell reworded to "fourteen at the MUST level and one SHOULD" | not fixed: count gaps per level and compare each level with the cell's figure for it |
| 2026-10-06 | spec-rfc-verdict-fix-bgp | `./le rfc check`, RFC9256 and RFC9552 Support remaining cells | After explicit optional gap annotations, the gate reported seven gaps against six MUST gaps and three against two MUST gaps. The MAY/RECOMMENDED rows must not be relabeled or hidden. | Gate repair remains open. Removed duplicate cached counts from the Support cells while preserving every requirement-level gap annotation; no new feature or gate change. |
