---
journal: rfc-counting-history 2026-11-23
topic: RFC Compliance
---

## Before

The RFC compliance ledger counted requirements of obsoleted RFCs as open gaps
even when Ze proved or implemented the replacement requirement in the newer
RFC, treated `RFC4360-6-1` as partial because an operator could write custom
policy that breaks it, and counted operator, network-topology, IANA/IETF
process, and section-scoped non-Ze-role clauses as open software gaps. The
public compliance page did not explain how the compliance percentages are
calculated for an outside engineer, and there was no central counting-history
record for weekly updates to cite when calculation rules change.

## After

Superseded requirements satisfied by a proven or implemented successor RFC
count as proven (`+37` MUSTs); default-path compliance is no longer downgraded
to partial when custom operator policy can override it; and 53 operator,
topology, IANA/RFC-author, and section-scoped non-Ze-role clauses are excluded
with checked evidence (`not owed`), moving the walked 230-RFC totals from
`2,777 / 5,166` (`53.8%` all MUSTs, `76.6%` built MUSTs) to `2,818 / 5,113`
(`55.1%` all MUSTs, `77.0%` built MUSTs). The `/quality/rfc-compliance/` page
and its Markdown mirror now include a plain-language "How compliance is
calculated" guide and a dated Counting history block backed by
`plan/records/rfc-counting-history.md`.

## Corrects

Misclassification of superseded requirements, operator/topology rules,
IANA/RFC-author obligations, and section-scoped non-Ze roles as software
defects, and confusion when reporting compliance-figure movements in weekly
updates.

## Exposure

none

## Upstream

none

## Asked by

Thomas Mangin
