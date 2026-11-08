# A gate counts a population without the attribute that splits it

A check totals rows to compare against a published figure, but the rows carry
an attribute (an obligation level, a role) that the published figure splits on.
The total is then wrong for every row whose attribute differs, and authors route
around the check instead of recording the row honestly.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-09-30 | rfc-verdict-fix-access | `checkGapCountAgreement` (`internal/le/rfc`), via `./le rfc check` | It counts every `{gap}` row as a MUST-level gap whatever the row's level, so a SHOULD gap (RFC3579-3-2) breaks the Support cell's MUST count, and an author disclosed the gap only in prose to avoid the refusal | fixed: 697558b8ae retired the hand-written Support cell, and `checkGapCountAgreement` with it, for a derived ledger row; `TestGapLevelAccounting` (37cf1d1d03) proves SHOULD and MAY gaps enter no mandatory count |
| 2026-10-06 | spec-rfc-verdict-fix-bgp | `./le rfc check`, RFC9256 and RFC9552 Support remaining cells | After explicit optional gap annotations, the gate reported seven gaps against six MUST gaps and three against two MUST gaps. The MAY/RECOMMENDED rows must not be relabeled or hidden. | fixed: producer removed by 697558b8ae with the cell it read; a HEAD grep finds no `checkGapCountAgreement` outside test comments, and `TestParseMetaRefusesRetiredSupportCells` covers the refusal. |
