# Spec Workflow

Interactive spec creation and resumption with hard gates between phases.
Every gate includes a mandatory challenge — surface concerns, not just summaries.

## Instructions

### Step 0: Detect Mode

1. Read `.claude/selected-spec`
2. If set AND spec file exists in `docs/plan/`: **RESUME mode** — go to Step R
3. If empty or spec doesn't exist: **NEW mode** — go to Step 1

---

### Step R: Resume Existing Spec

1. Read the spec file from `docs/plan/`
2. Read `.claude/session-state.md` for digests
3. Determine current phase by checking spec completeness:
   - No `→ Decision:` / `→ Constraint:` annotations → still in RESEARCH
   - Data Flow section empty → still in RESEARCH
   - No AC table entries → still in DESIGN
   - Spec complete, audit empty → ready for IMPLEMENT
4. Present status to user:
   - Current phase detected
   - Summary of what's been captured so far (annotations, data flow, AC)
   - What's missing
5. **GATE:** ASK user: "Resuming at [PHASE]. Is this right, or should we back up?"
6. Continue from the detected phase gate

---

### Step 1: SCOPE (Hard Gate)

**Goal:** Agree on what we're building.

1. ASK the user what feature/task they want to spec (if not provided as argument)
2. Search `docs/plan/spec-*.md` for related active specs
3. Search `docs/plan/done/*.md` for completed related work
4. Check `.claude/INDEX.md` for relevant architecture docs
5. Present:
   - Related specs found (if any)
   - Whether this overlaps or extends existing work
   - Suggested spec filename
6. **Mandatory challenge:** Before presenting the gate, raise at least one concern:
   - Does this overlap with something that already exists?
   - Is the scope too broad for one spec? Too narrow to be useful?
   - Is there a simpler framing of the same goal?
7. **GATE:** ASK user to confirm scope and direction. Present your concern alongside the recommendation. Do not proceed until confirmed.
8. Write spec filename to `.claude/selected-spec`

---

### Step 2: RESEARCH (Hard Gate)

**Goal:** Understand existing code and architecture before designing anything.

#### Checkpoint Rules

- **Never change `[ ]` to `[x]` in spec files** — checkboxes are template markers
- Capture every insight as `→ Decision:` or `→ Constraint:` under the reading entry
- Track what you've read in `session-state.md` with digests
- The annotations ARE the knowledge — they survive compaction, file contents don't

#### Annotation Quality Standard

Annotations must be **actionable constraints** — specific enough to change a design decision.

| Bad (too vague) | Good (actionable) |
|---|---|
| `→ Constraint: uses buffer-first encoding` | `→ Constraint: WriteTo(buf, off) int required — no Pack() or returning []byte` |
| `→ Decision: pools are used` | `→ Decision: buildBufPool is 4096 bytes — attribute encoding must fit in one pool buffer` |
| `→ Constraint: follows plugin pattern` | `→ Constraint: register via init() in register.go — RunEngine(connA, connB) int signature` |

If an annotation wouldn't help someone make a design choice, it's too vague. Rewrite it.

#### Process

1. Read `.claude/INDEX.md` — identify relevant architecture docs
2. For each relevant doc:
   - Read it
   - Write `→ Decision:` or `→ Constraint:` annotation in spec under reading entry
   - Write one-line digest to `session-state.md`
3. Read ACTUAL source files — document current behavior:
   - What each file does, key functions, patterns used
   - Behavior that must be preserved (unless user says otherwise)
   - Write `→ Constraint:` noting preservation requirements
4. Trace data flow per `rules/data-flow-tracing.md`
5. RFC check: verify `rfc/short/rfcNNNN.md` summaries exist for referenced RFCs
6. Fill the spec's **Key Insights** summary (minimal context to resume after compaction)
7. Present research findings to user:
   - Current behavior (what the code does now)
   - Architectural constraints discovered
   - Data flow through the system
   - Related specs/work that affects this
8. **Mandatory challenge:** Before presenting the gate, raise at least one concern:
   - What is still unclear or ambiguous in the existing code?
   - What assumption are we making that could be wrong?
   - What existing behavior might this feature accidentally break?
9. **GATE:** ASK user: "Is my understanding correct? Here's what concerns me: [concern]."
   Do not proceed to DESIGN until user confirms research is complete.

---

### Step 3: DESIGN (Hard Gate)

**Goal:** Agree on changes, acceptance criteria, and test plan before writing.

#### Alternatives (MANDATORY — present at least 2 approaches)

1. Identify at least 2 distinct approaches to the problem
2. For each approach, present:
   - How it works (1-2 sentences)
   - Trade-offs (what it gains, what it costs)
3. Recommend one and explain why it wins

#### Design

4. Present proposed changes for the recommended approach:
   - Current behavior → proposed behavior (what changes)
   - What stays the same (preservation list)
5. Discuss data flow impact:
   - Entry point, transformation path, boundaries crossed
   - Integration points with existing code
6. Draft acceptance criteria (AC-1, AC-2, ...) — each must be testable
7. Draft wiring test table — how is feature reachable from entry point?
8. Draft TDD test plan — unit tests, boundary tests, functional tests
9. Identify files to modify/create

#### Failure Mode Analysis (MANDATORY)

10. Enumerate what could go wrong:
    - What inputs or states break this?
    - What existing behavior could this accidentally change?
    - What happens if a dependency (pool, channel, config) is missing or full?
    - What happens under concurrent access?

#### Triple Challenge (MANDATORY)

Answer all three before presenting the gate. If any answer is "no", redesign.

| Challenge | Question |
|---|---|
| **Simplicity** | Is this the minimum change that achieves the goal? If not, what's simpler and why was it rejected? |
| **Uniformity** | Does this follow the same pattern as similar features in the codebase? If it introduces a new pattern, why? |
| **Performance** | Does this respect ze's performance constraints (zero-copy, pool buffers, no per-event allocations)? Where are the allocations? |

#### Gate

11. **Mandatory challenge:** Present your strongest concern about the design — the thing most likely to cause rework.
12. **GATE:** ASK user to review:
    - Recommended approach vs alternatives
    - Failure modes identified
    - Triple challenge answers
    - AC criteria and test plan

    Do not proceed until user approves design direction.

---

### Step 4: WRITE (Hard Gate)

**Goal:** Produce the spec file using `docs/plan/TEMPLATE.md`.

1. Write (or update) spec at `docs/plan/spec-<name>.md` using template format
2. Fill all sections from research and design phases:
   - Required Reading with `→ Decision:` / `→ Constraint:` annotations
   - Current Behavior (from research)
   - Data Flow (from research)
   - Wiring Test table (from design)
   - AC table (from design)
   - TDD Test Plan (from design)
   - Files to Modify/Create (from design)
   - Implementation Steps
3. Run Pre-Spec Verification:
   - All checkboxes `[ ]` (never `[x]`)
   - No code snippets
   - Tables not prose for structured data
   - AC-N rows with testable assertions
   - All reading entries have `→ Decision:` or `→ Constraint:`
   - Wiring test rows all have concrete test names

#### Spec Independence Test (MANDATORY)

4. Before presenting the gate, answer honestly:
   - **Could a different Claude session implement this spec without additional context?**
   - For each AC: can a test be written from the AC text alone, without guessing?
   - For each implementation step: are the inputs, outputs, and constraints stated?
   - Is there anything "we discussed" that isn't captured in the spec?
   If any answer is "no", fix the spec before presenting it.

#### Gate

5. **Mandatory challenge:** Name the weakest part of this spec — the section most likely to cause confusion or rework during implementation.
6. Present spec to user for final review.
7. **GATE:** ASK user: "Ready to save? The weakest part is [X] — should we strengthen it?"
   Iterate on feedback until approved.
8. Save and `git add docs/plan/spec-<name>.md`

---

## Rules

- **Never tick `[ ]` to `[x]` in spec files** — track progress in session-state.md
- **`→ Decision:` / `→ Constraint:` annotations are the knowledge** — they survive compaction
- Each GATE must use `AskUserQuestion` — never auto-proceed past a gate
- Style: tables and prose, never code snippets in specs (`rules/spec-no-code.md`)
- All research findings go into spec exhaustively (`rules/planning.md`)
- Append-only editing for existing specs (`rules/spec-preservation.md`)
- One spec at a time — `.claude/selected-spec` tracks which
