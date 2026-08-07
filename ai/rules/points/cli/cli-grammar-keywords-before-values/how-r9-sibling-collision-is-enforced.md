---
kind: directive
level:
stage:
---
**Enforcement (R9).** Inside the YANG command tree, the static gate flags any child
token whose left segment is itself a sibling name at the same tree level
(`grammar.CheckSiblings`). R9-by-sibling needs sibling context, so ONLY the static gate
runs it: per-command registration cannot see siblings. It is deliberately conservative
and fires only when the namespace literally exists as a sibling, so a genuine compound
is never flagged. **Root commands are a separate surface:** they register via
`registry.MustRegisterRootHandler` / `RegisterRoot` and never enter the YANG tree, so
`CheckSiblings` cannot see them. The root-namespace feeder (`grammar.CheckRootNamespace`,
the fourth feeder below) governs them with a cross-surface check: a hyphenated root whose
left segment names a YANG verb or container is the same R9 violation (`traffic-control`
vs the `traffic` container). Do not assume a root is ungoverned because it is not in the
tree.
Shipped commands awaiting the agreed rename are listed in `pendingNamespaceSplit`
(`scripts/checks/cli_grammar.go`) and reported as tracked debt, so the gate stays green
while a NEW collision still fails. Migrating one is a dispatch-key change (see
"Migrating a Built-in Command's Path" below): add the split path, keep the old form per
"Backward Compatibility", update `.ci` senders, and remove its `pendingNamespaceSplit`
entry.
