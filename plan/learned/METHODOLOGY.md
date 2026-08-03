# Knowledge Extraction Methodology

How to extract a summary from a completed spec into `plan/learned/NNN-<name>.md`.

## Summary Format

| Section | Content |
|---------|---------|
| `# NNN -- Name` | Title from spec filename |
| `## Context` | Short paragraph (3-5 sentences): what problem existed, what was the symptom, what was the goal |
| `## Decisions` | Bullet points: what was decided, what was rejected, and why |
| `## Consequences` | Bullet points: what this enables, constrains, or changes going forward |
| `## Gotchas` | Bullet points: what surprised, failed, or trapped |
| `## Files` | List of files modified/created |

**Rules:**
- Each bullet point is one line, max two
- Reference architecture doc when the decision is documented there: "(see encoding-context.md)"
- Decisions must include "over" clauses when alternatives existed: "chose X over Y because Z"
- Gotchas section is the most valuable -- never skip even if empty ("None.")
- All five sections carry content. A section with nothing to say is not filled with a phrase that says so; it is the signal that no summary belongs here (see below)

### Section Quality Checks

| Section | Quality check |
|---------|---------------|
| Context | "Could a future reader reconstruct *why this work was worth doing* from this section alone?" |
| Decisions | "Does each bullet name what was chosen AND what was rejected?" |
| Consequences | "If someone touches this area next, what do they need to know that the code alone won't tell them?" |
| Gotchas | "Would a future session hit the same trap without this warning?" |
| All | "If I deleted this entry, would a future session miss something that code alone cannot tell them?" |

## When No Summary Is Written

A summary is a record of a lesson, not an artifact of closing a spec. Work that
produced no knowledge writes no file: no number is allocated, and
`plan/learned/` does not grow.

Measured on the corpus that made this rule: 229 of 1,285 summaries carried no
gotcha, and 77 stated in words that they carried no lesson. Every one of them
cost a number, an index row, and a future reader's attention.

Write nothing when all of these hold:

| Condition | Test |
|-----------|------|
| The change only relocated content | Moves, file renames, reformatting, and a rename applied everywhere. The words are the same words, in a different place |
| No alternative was rejected | Nothing to put in Decisions with an "over" clause |
| Nothing surprised, failed, or trapped | Gotchas would read "None." with nothing above it |
| Nothing constrains the next session | Consequences would restate what the diff already shows |

Write a summary when any one of these holds, even if the diff is small:

- A decision was made and something else was rejected.
- A constraint was discovered that the code does not state.
- Something failed in a way the next session would repeat.
- An interface, a gate, or a default changed, and the reason is not in the code.

The judgement is yours, and it is asked before the number is allocated
(`/ze-close` step 6a). The commit helper asks the same question of the diff and
refuses commit A when the change adds content and neither a summary nor an
explicit reason is present (`scripts/dev/commit_helper.py` `lesson_worthy`). It
reads what changed, so it cannot tell filler from knowledge: a session that
writes one hollow gotcha to satisfy it has satisfied nothing.

This governs records of COMPLETED WORK. It says nothing about records of
DEFECTS. A `plan/known-failures/` shard and a `plan/deferrals/` row are
governed by `ai/rules/completion.md` and `ai/rules/completion.md`, and
nothing here permits deleting one.

## What Counts as Knowledge

Knowledge is information that code alone cannot convey: WHY a decision was made, WHAT alternatives were considered, WHAT surprised or failed, WHAT constraints were discovered, WHAT this enables or constrains going forward. If the information can be derived by reading the current source code, it is not knowledge -- it is description.

| Knowledge | Not knowledge |
|-----------|---------------|
| "Chose uint16 over pointer: saves 6MB at 1M routes" | "ContextID is uint16" |
| "ADD-PATH is the only asymmetric capability" | "Each peer has recv and send contexts" |
| "subsystem.go is NOT the production handler" | "Modified server_startup.go" |
| "deliverConfig sends to ALL peers with matching cap" | "Plugin receives config" |
| "OpenSent collision not handled -- adding it later needs a new code path, not an extension" | "Collision detection is implemented" |

## Where Knowledge Lives in a Spec

### ALWAYS read -- highest density

| Section | Maps to summary | What to extract |
|---------|----------------|-----------------|
| Task / Problem Statement | Context | Problem, symptom, motivation -- the full "why" behind the work |
| Current Behavior: Behavior to change | Context | What was broken or limited before |
| Core Insight / Key Insight | Decisions | Single most important design revelation (not all specs have this) |
| Key Design Decisions | Decisions | Rationale for choices: "chose X over Y because Z" |
| `-> Decision:` annotations | Decisions | One-line design choices embedded in Required Reading |
| `-> Constraint:` annotations | Consequences | Constraints discovered that affect future work |
| Design Insights | Consequences | Truths about the system that affect what comes next |
| Known Limitations | Consequences | What was deliberately not done and why -- future scope |
| Deviations from Plan | Gotchas | Where plan met reality -- corrections and WHY the plan was wrong |
| Mistake Log | Gotchas | Failure patterns -- highest value-per-line in any spec |
| Implementation Summary: Bugs Found | Gotchas | Bugs that revealed architectural truths |

### NEVER read -- pure process scaffolding

Post-Compaction Recovery, MANDATORY READING box, TDD Test Plan tables, Implementation Steps (procedure), Checklists (TDD, Verification, Design, Completion), Implementation Audit tables, Critical Review table, Estimated Effort.

### SOMETIMES read -- check signals

| Section | Has knowledge when... | Is noise when... |
|---------|----------------------|-------------------|
| Required Reading | Has `-> Constraint:` or `-> Decision:` annotations | Just unchecked `[ ]` boxes with doc names |
| Current Behavior | Documents specific behavior with file:line refs | Says "ALL behavior preserved" or "Not applicable" |
| Data Flow | Has boundary crossings with specific transformations | Says "Not applicable" |
| Implementation Summary | Has "Bugs Found" or insights with content | Just lists files changed |

## Extraction Recipe

For every spec, regardless of how it was committed:

| Step | Read | Write |
|------|------|-------|
| 1 | Task + Current Behavior (behavior to change) + commit message | **Context** (3-5 sentences: problem, symptom, goal) |
| 2 | Core Insight + Key Design Decisions + `-> Decision:` annotations | **Decisions** (bullet points with "over" clauses) |
| 3 | Design Insights + `-> Constraint:` annotations + Known Limitations | **Consequences** (bullet points: enables, constrains, interacts with) |
| 4 | Deviations + Mistake Log + Bugs Found | **Gotchas** (bullet points) |
| 5 | Files to Modify/Create or `git show --stat` | **Files** (list) |
| 6 | Skip everything else | -- |

Knowledge concentrates in post-implementation sections (Design Insights, Deviations, Mistake Log), not in planning sections. Design-focused specs front-load knowledge in Core Insight sections. Implementation-focused specs back-load it into post-mortem sections. The recipe covers both.

## For Future Specs

At spec completion, the Executive Summary (already BLOCKING before commit) maps directly:

| Executive Summary section | Learned summary section |
|--------------------------|------------------------|
| Objective | Context (expand to include problem/symptom from Task section) |
| Design decisions | Decisions |
| Risks & observations | Consequences + Gotchas (forward-looking risks to Consequences, traps to Gotchas) |
| Not done | Consequences (scope boundaries and constraints accepted) |
| Changes table | Files |

One additional step: scan Design Insights + Known Limitations for **Consequences** bullets not already covered by the Executive Summary mapping. This takes 30 seconds when context is fresh.

## Numbering

Reserve the next number with `python3 scripts/dev/commit_helper.py learned-next <name>`: it takes max(existing `plan/learned/NNNN-*.md` prefixes) + 1 and creates the `NNNN-<name>.md` file immediately, so concurrent sessions in one tree cannot allocate the same number. The filenames ARE the record -- there is no separate counter cache to read, bump, or stage.

`make ze-learned-numbers-check` is the uniqueness/consistency backstop (no two summaries share a number, each H1 matches its filename); duplicate numbers across branches surface only on merge/rebase and are resolved with `make ze-learned-numbers-fix`.

Write the summary to `plan/learned/NNN-<name>.md` instead of moving the full spec to `done/`.
