# format cannot hold a state the tree holds

One of several text forms of the same tree has no spelling for a state the
tree can hold, so its writer emits the nearest state it can spell and the
reader rebuilds a different tree. The value in effect can be unchanged while a
flag an operator set is lost, and nothing fails until the next edit acts on it.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-11 | spec-terminal-demo-showcase (bracket review r7 NOTE 1) | `emitValueOrArrayNop` (config/serialize_set.go), `SetParser.markNopInactive` | Set format cannot hold a wholly inactive plain leaf-list: `inactive: plain [ a ]` is written `nop c plain a` and reads back as member `a` inactive, leaf active. Effective value unchanged; the whole-leaf flag is lost through draft and change files. Since `1abe5f7f0b`. The bracket kind round-trips | not fixed: give the plain kind a whole-leaf set spelling |
