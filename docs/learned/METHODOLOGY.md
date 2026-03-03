# Knowledge Extraction Methodology

How to extract a summary from a completed spec into `docs/learned/NNN-<name>.md`.

## Summary Format

| Section | Content |
|---------|---------|
| `# NNN — Name` | Title from spec filename |
| `## Objective` | 1-2 sentences: what was the goal and why |
| `## Decisions` | Bullet points: what was decided and why |
| `## Patterns` | Bullet points: patterns discovered or confirmed |
| `## Gotchas` | Bullet points: what surprised, failed, or trapped |
| `## Files` | List of files modified/created |

**Rules:**
- Each bullet point is one line, max two
- Reference architecture doc when the decision is documented there: "(see encoding-context.md)"
- Gotchas section is the most valuable — never skip even if empty ("None.")
- Mechanical refactors with no design decisions: single line "Mechanical refactor, no design decisions."

## What Counts as Knowledge

Knowledge is information that code alone cannot convey: WHY a decision was made, WHAT alternatives were considered, WHAT surprised or failed, WHAT constraints were discovered. If the information can be derived by reading the current source code, it is not knowledge — it is description.

| Knowledge | Not knowledge |
|-----------|---------------|
| "Chose uint16 over pointer: saves 6MB at 1M routes" | "ContextID is uint16" |
| "ADD-PATH is the only asymmetric capability" | "Each peer has recv and send contexts" |
| "subsystem.go is NOT the production handler" | "Modified server_startup.go" |
| "deliverConfig sends to ALL peers with matching cap" | "Plugin receives config" |

**Quality check:** "If I deleted this entry, would a future session miss something that code alone cannot tell them?" If no, the entry is noise.

## Where Knowledge Lives in a Spec

### ALWAYS read — highest density

| Section | Maps to summary | What to extract |
|---------|----------------|-----------------|
| Task / Problem Statement | Objective | Root cause, motivation — the "why" behind the work |
| Core Insight / Key Insight | Decisions | Single most important design revelation (not all specs have this) |
| Key Design Decisions | Decisions | Rationale for choices: "chose X over Y because Z" |
| `→ Decision:` annotations | Decisions | One-line design choices embedded in Required Reading |
| `→ Constraint:` annotations | Patterns | Constraints discovered by reading source files |
| Design Insights | Patterns | Truths discovered during implementation |
| Deviations from Plan | Gotchas | Where plan met reality — corrections and WHY the plan was wrong |
| Mistake Log | Gotchas | Failure patterns — highest value-per-line in any spec |
| Known Limitations | Gotchas | Protocol gotchas, edge cases, trade-offs |
| Implementation Summary: Bugs Found | Gotchas | Bugs that revealed architectural truths |

### NEVER read — pure process scaffolding

Post-Compaction Recovery, MANDATORY READING box, TDD Test Plan tables, Implementation Steps (procedure), Checklists (TDD, Verification, Design, Completion), Implementation Audit tables, Critical Review table, Estimated Effort.

### SOMETIMES read — check signals

| Section | Has knowledge when... | Is noise when... |
|---------|----------------------|-------------------|
| Required Reading | Has `→ Constraint:` or `→ Decision:` annotations | Just unchecked `[ ]` boxes with doc names |
| Current Behavior | Documents specific behavior with file:line refs | Says "ALL behavior preserved" or "Not applicable" |
| Data Flow | Has boundary crossings with specific transformations | Says "Not applicable" |
| Implementation Summary | Has "Bugs Found" or insights with content | Just lists files changed |

## Extraction Recipe

For every spec, regardless of how it was committed:

| Step | Read | Write |
|------|------|-------|
| 1 | Task + commit message (if available) | **Objective** (1-2 sentences) |
| 2 | Core Insight + Key Design Decisions + `→ Decision:` annotations | **Decisions** (bullet points) |
| 3 | Design Insights + `→ Constraint:` annotations | **Patterns** (bullet points) |
| 4 | Deviations + Mistake Log + Known Limitations + Bugs Found | **Gotchas** (bullet points) |
| 5 | Files to Modify/Create or `git show --stat` | **Files** (list) |
| 6 | Skip everything else | — |

Knowledge concentrates in post-implementation sections (Design Insights, Deviations, Mistake Log), not in planning sections. Design-focused specs front-load knowledge in Core Insight sections. Implementation-focused specs back-load it into post-mortem sections. The recipe covers both.

## For Future Specs

At spec completion, the Executive Summary (already BLOCKING before commit) maps directly:

| Executive Summary section | Learned summary section |
|--------------------------|------------------------|
| Objective | Objective |
| Design decisions | Decisions |
| Risks & observations | Gotchas |
| Changes table | Files |

One additional step: scan Design Insights + Deviations for **Patterns**. This takes 30 seconds when context is fresh.

Write the summary to `docs/learned/NNN-<name>.md` instead of moving the full spec to `done/`.
