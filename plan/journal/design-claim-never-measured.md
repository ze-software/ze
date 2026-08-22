# A design claim nobody measured, built on for three phases

A spec justifies a shape with a performance claim. The claim reads as a fact,
because it is written in the same voice as the constraints around it, and
nothing in the workflow asks for its measurement. Later phases assume the shape.
When somebody finally measures, the reversal has to unpick every phase that
inherited it, and the same premise usually has to be unpicked twice, because it
spread under a second name.

A design rationale is evidence for its reader exactly as much as a doc comment
defending a symbol is (`ai/rules/evidence.md`): it records what its author
believed. The earlier a claim sits in a dependency chain, the cheaper it is to
check and the more expensive it is to leave.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-08-22 | record-answers-2-only-encoding | the plugin answer wire, the id field | the spec specified a length-prefixed id, `#<len>:<id>`, so a reader reaches the kind token by arithmetic rather than by scanning for a space. Nothing measured it. Phase 3 (`326ce6e96`) built it, phase 4 put the kind tokens at the offsets it produced, and phase 5 (`46c4d0e1e`) applied the same premise to every counted field as a base-36 outer length. Measured, the counted id is SLOWER: 8.1 to 9.2 ns against 3.2 to 3.5 ns for a fused digit loop over plain `#42 `, and 17.9 against 32.1 ns at twenty digits, zero allocations either way, and it is two bytes wider on every line of every walk. The count bought nothing because `cutID` still had to check the space that closes the field and still had to call `ParseUint` on the slice it had just measured, which IS the cost of the plain form | reversed by the owner's measurement in phase 6 (`9313b7d5e`), and the second population unpicked separately in `50468ee34` because the same premise had spread to every counted field under another name. What replaced the shape is a rule rather than a spelling: a count belongs on a field whose value CAN hold the delimiter, and nowhere else. The habit the reversal asks for is that a performance rationale which decides a shape later phases will build on is an ASSUMPTION with an A-N row, and its benchmark is the FIRST phase, before the shape exists. This one took two minutes and was as available before phase 3 as after it |
