---
kind: directive
level: MUST
stage:
---
**Before fixing a defect, `plan/` MUST be searched for a spec that already
describes it, and any spec found MUST be closed or corrected by the same work.**
Grep `plan/spec-fixit-*.md` first, then the rest of `plan/`, on the symbol, the
file, and the failure's own words. A fix that lands while its spec stays open
leaves the backlog counting work that is already done, and the next session
reads that spec as a task.

**A spec the fix discharges is closed by the fix, not after it.** State in the
commit which spec the work discharges, and close it on the route
`ai/rules/planning.md` gives. A spec the fix only PARTLY discharges is corrected
rather than closed: strike the acceptance criteria the fix met and say what
remains, so the reader inherits a smaller task instead of a stale one.

**A spec whose premise the fix disproves is closed too, with the disproof.** The
premise being wrong is a result, and it is worth more to the next reader than
the spec was.

The same search decides the journal row. A defect that already has a spec needs
no new row; the row belongs to a defect that has none.
