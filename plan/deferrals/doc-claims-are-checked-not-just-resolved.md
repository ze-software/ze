# Deferrals: doc-claims-are-checked-not-just-resolved

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-08-09 | `plan/spec-problem-journal.md` | Every doc-freshness gate verifies reference integrity and none verifies claim truth: 82 of 1611 anchors named a symbol absent from the file they point at, all passing green | Found while moving 1155 `// Design:` pointers. The goal of that work did not depend on the gate being stronger, so it gets a spec rather than a fix folded into that commit (`ai/rules/completion.md`) | `plan/spec-doc-claims-are-checked-not-just-resolved.md` | open |
| 2026-08-09 | `plan/spec-problem-journal.md` | Full audit of whether the rule corpus keeps design documentation current, beyond the gates read while working | Owner asked for the sweep to run at the end of the work in hand, not during it | this spec's research phase | open |
