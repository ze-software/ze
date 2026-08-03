---
name: ze-spec
description: Spec Workflow
---

# Spec Workflow

Interactive spec creation and resumption with hard gates between phases.
Every gate includes a mandatory challenge -- surface concerns, not just summaries.

See also: `/ze-design` (stress-test a design), `/ze-explore` (research a topic), `/ze-audit` (pre-impl check)

## Delegation

`ai/rules/planning.md` exempts this skill: it runs in the MAIN THREAD.
Its gates require `AskUserQuestion`, and a subagent can neither hold a dialogue
with the user nor use LSP.

Delegate the work around the gates, not the gates themselves: send the research
and the file reading out to agents (`/ze-explore`, `/ze-audit`), then carry every
decision point here yourself. Verify what those agents report against source
before putting it in front of the user (`ai/rules/evidence.md`).

## Instructions

### Step 0: Detect Mode

1. Run `scripts/dev/spec-session.sh current`
2. If set AND spec file exists in `plan/`: **RESUME mode** -- go to Step R
3. If empty or spec doesn't exist: **NEW mode** -- go to Step 1

---

### Step R: Resume Existing Spec

1. Read the spec file from `plan/`
2. Read per-session state (most recent `tmp/session/session-state-<spec-stem>-*.md`) for digests
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
2. Search `plan/spec-*.md` for related active specs
3. Search `plan/learned/*.md` for completed related work
4. Check `ai/INDEX.md` for relevant architecture docs
5. Present:
   - Related specs found (if any)
   - Whether this overlaps or extends existing work
   - Suggested spec filename
6. **Mandatory challenge:** Before presenting the gate, raise at least one concern:
   - Does this overlap with something that already exists?
   - Is the scope too broad for one spec? Too narrow to be useful?
   - Is there a simpler framing of the same goal?
7. **GATE:** ASK user to confirm scope and direction. Present your concern alongside the recommendation. Do not proceed until confirmed.
8. Create spec file with `Status: skeleton`
9. Claim it for this session: `scripts/dev/spec-session.sh claim <spec-file>`

---

### Step 2: RESEARCH (Hard Gate)

**Goal:** Understand existing code and architecture before designing anything.

**Status transition:** When starting research, edit the spec: set `Status` to `design`, `Updated` to today. Do this before reading any source files.

#### Feature Surface Gate (BLOCKING)

A feature is not "specced" until every surface it must touch is enumerated in the
spec. Features that skip this ship in 2-3 commits instead of 1 (lint failures,
unwired symbols, missing doctor checks, stale docs found after the fact). This gate
moves the enumeration into the spec so the work happens in one pass. It applies to
ALL features, not just BGP protocol work.

**1. Identify the feature type(s)** and read the matching pattern doc in full before
other research. A feature can be more than one type -- read every row that applies.

| Feature type | Read (BLOCKING) | Applies when |
|---|---|---|
| BGP family (SAFI / capability / attribute) | `ai/patterns/bgp-family.md` | see BGP Family Gate below |
| System or BGP plugin | `ai/patterns/plugin.md`, `ai/patterns/registration.md` | new `internal/plugins/<name>/` or `bgp/plugins/<name>/` with `register.go` |
| Component | `ai/patterns/registration.md`, `ai/rules/architecture.md` | new `internal/component/<name>/` |
| CLI command | `ai/patterns/cli-command.md` | new verb/subcommand or `ze:command` YANG node |
| Config option / YANG leaf / env var | `ai/patterns/config-option.md`, `ai/rules/config.md` | new YANG leaf, container, or `ze.*` env var |
| Runtime dependency | `ai/rules/repo-maintenance.md` | new file path, socket, listen port, kernel module, external binary, cert, procfs/sysctl, or netlink use |

`ai/INDEX.md` (the "Build / extend" table) is the authoritative feature-type to
pattern map; consult it if the type is unclear.

**2. Answer every row of both checklists in `plan/TEMPLATE.md`** with `Yes` (naming
the file), `No`, or `N-A` (with a reason). Never leave the cell blank: an
unanswered row is indistinguishable from a forgotten one, which is why 286
Documentation rows and 138 Integration rows across the existing specs still carry
an unanswered marker. These are the cross-cutting surfaces forgotten regardless of
feature type:
- **Integration Checklist** -- YANG schema + validation, CLI grammar, completion,
  functional test, env var, **doctor check + diagnostic code**, Prometheus counters,
  and the BGP family surface (answered in `ai/patterns/bgp-family.md`, not inline).
- **Documentation Update Checklist** (17 rows) -- `docs/features.md`, command/API/plugin
  docs, source anchors.

`plan/TEMPLATE.md` is design-time only. The closure sections live in
`plan/TEMPLATE-CLOSURE.md` and are appended by `/ze-close` at step 1. Do not
copy them into a new spec.

**3. Discovery (BLOCKING):** answer the `ai/rules/repo-maintenance.md` Mechanical
Checklist in the spec -- where an agent looks first (`ai/INDEX.md` row), what rule
prevents regression, what registry/inventory prevents drift, what verification proves
it. A feature that cannot be found from `ai/INDEX.md` or a discovery surface is not done.

Real incidents this catches: composition-root regen (geodns), server-side RPC
forwarder for plugin CLI commands (firewall-irr), doctor check registration, and
stale interop fixtures after a config restructure (PATHS-LIMIT) were each a forgotten
surface that became a follow-up commit.

#### BGP Family Gate (BLOCKING)

If the spec involves a new SAFI, capability, or attribute:
1. Read `ai/patterns/bgp-family.md` in full before any other research
2. Copy the 12-section checklist into the spec's Integration Checklist
3. Every section must be answered (Yes with file, or N/A with reason)

Detection: if the task mentions any of these, the gate applies:
- New address family, NLRI type, SAFI
- New BGP capability or capability code
- New BGP attribute or attribute code
- ExaBGP bridge family support

Skipping this gate is how SR-Policy shipped in 3 commits instead of 1.

#### Checkpoint Rules

- **Never change `[ ]` to `[x]` in spec files** -- checkboxes are template markers
- Capture every insight as `→ Decision:` or `→ Constraint:` under the reading entry
- Track what you've read in per-spec session state with digests
- The annotations ARE the knowledge -- they survive compaction, file contents don't

#### Annotation Quality Standard

Annotations must be **actionable constraints** -- specific enough to change a design decision.

| Bad (too vague) | Good (actionable) |
|---|---|
| `→ Constraint: uses buffer-first encoding` | `→ Constraint: WriteTo(buf, off) int required -- no Pack() or returning []byte` |
| `→ Decision: pools are used` | `→ Decision: buildBufPool is 4096 bytes -- attribute encoding must fit in one pool buffer` |
| `→ Constraint: follows plugin pattern` | `→ Constraint: register via init() in register.go -- RunEngine(connA, connB) int signature` |

If an annotation wouldn't help someone make a design choice, it's too vague. Rewrite it.

#### Process

1. Read `ai/INDEX.md` -- identify relevant architecture docs
2. For each relevant doc:
   - Read it
   - Write `→ Decision:` or `→ Constraint:` annotation in spec under reading entry
   - Write one-line digest to per-spec session state file
3. Read ACTUAL source files -- document current behavior:
   - What each file does, key functions, patterns used
   - Behavior that must be preserved (unless user says otherwise)
   - Write `→ Constraint:` noting preservation requirements
4. Trace data flow per `ai/rules/architecture.md`
5. RFC check: verify `rfc/short/rfcNNNN.md` summaries exist for referenced RFCs, and note any `docs/features/rfc-status.md` row the spec will add or change so the standards ledger stays synced (per `ai/rules/repo-maintenance.md`)
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
9. **Record assumptions (BLOCKING):** Every assumption surfaced — in the challenge above
   or anywhere during research — goes into the spec's **Risks & Assumptions** Assumptions
   table (A-N rows) with Basis, "If wrong" impact, and a validation method. Gate
   conversation does not survive the session; the table does.
10. **GATE:** ASK user: "Is my understanding correct? Here's what concerns me: [concern]."
   Do not proceed to DESIGN until user confirms research is complete.

---

### Step 3: DESIGN (Hard Gate)

**Goal:** Agree on changes, acceptance criteria, and test plan before writing.

#### Alternatives (MANDATORY -- present at least 2 approaches)

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
6. Draft acceptance criteria (AC-1, AC-2, ...) -- each must be testable
7. Draft wiring test table -- how is feature reachable from entry point?
8. Draft TDD test plan -- unit tests, boundary tests, functional tests
9. Identify files to modify/create
   - For each code file: check its `// Design:` annotation -- if the change affects
     behavior described in that architecture doc, add the doc to the file list

#### Failure Mode Analysis (MANDATORY)

10. Enumerate what could go wrong:
    - What inputs or states break this?
    - What existing behavior could this accidentally change?
    - What happens if a dependency (pool, channel, config) is missing or full?
    - What happens under concurrent access?

    **Record each failure mode (BLOCKING):** every item enumerated above becomes an
    R-N row in the spec's **Risks & Assumptions** Risks table (with early signal and
    mitigation), or an A-N Assumptions row if it hinges on an unverified belief.
    Failure modes presented only at the gate are lost when the session ends.

#### Triple Challenge (MANDATORY)

Answer all three before presenting the gate. If any answer is "no", redesign.

| Challenge | Question |
|---|---|
| **Simplicity** | Is this the minimum change that achieves the goal? If not, what's simpler and why was it rejected? |
| **Uniformity** | Does this follow the same pattern as similar features in the codebase? If it introduces a new pattern, why? |
| **Performance** | Does this respect ze's performance constraints (zero-copy, pool buffers, no per-event allocations)? Where are the allocations? |

#### Gate

11. **Mandatory challenge:** Present your strongest concern about the design -- the thing most likely to cause rework.
12. **GATE:** ASK user to review:
    - Recommended approach vs alternatives
    - Failure modes identified
    - Triple challenge answers
    - AC criteria and test plan

    Do not proceed until user approves design direction.

---

### Step 4: WRITE (Hard Gate)

**Goal:** Produce the spec file using `plan/TEMPLATE.md`.

1. Write (or update) spec at `plan/spec-<name>.md` using template format
2. Fill all sections from research and design phases:
   - Required Reading with `→ Decision:` / `→ Constraint:` annotations
   - Current Behavior (from research)
   - Data Flow (from research)
   - Risks & Assumptions (assumptions from research, risks from Failure Mode Analysis)
   - Wiring Test table (from design)
   - AC table (from design)
   - TDD Test Plan (from design)
   - Files to Modify/Create (from design)
   - Integration Checklist + Documentation Update Checklist (from the Feature Surface Gate -- every applicable row named with a file; N/A rows justified)
   - Implementation Steps
   - Review Gate section: keep the template's empty Run tables. `/ze-close`'s Review Gate step fills them by running `/ze-review` (the BLOCKING Review Gate) before closure. Never delete this section from the produced spec.
3. Run Pre-Spec Verification:
   - All checkboxes `[ ]` (never `[x]`)
   - No code snippets
   - Tables not prose for structured data
   - AC-N rows with testable assertions
   - Every assumption row has Basis + validation method; every risk row has early signal + mitigation
   - All reading entries have `→ Decision:` or `→ Constraint:`
   - Wiring test rows all have concrete test names
   - Integration Checklist and Documentation Update Checklist present, every row answered Yes (file) or N/A (reason) -- never left as empty template placeholders. The doctor-check row is answered if the feature adds any runtime dependency.
   - Review Gate section present (from the template), Run tables left empty -- it is filled at implementation time by `/ze-review`, not now

#### Spec Independence Test (MANDATORY)

4. Before presenting the gate, answer honestly:
   - **Could a different Claude session implement this spec without additional context?**
   - For each AC: can a test be written from the AC text alone, without guessing?
   - For each implementation step: are the inputs, outputs, and constraints stated?
   - Is there anything "we discussed" that isn't captured in the spec?
   - Are all concerns raised at the SCOPE/RESEARCH/DESIGN gates captured as A-N or R-N rows?
   If any answer is "no", fix the spec before presenting it.

#### Gate

5. **Mandatory challenge:** Name the weakest part of this spec -- the section most likely to cause confusion or rework during implementation.
6. Present spec to user for final review.
7. **GATE:** ASK user: "Ready to save? The weakest part is [X] -- should we strengthen it?"
   Iterate on feedback until approved.
8. **Status transition:** Edit the spec: set `Status` to `ready`, `Updated` to today. Design is done, spec is ready for implementation.
9. Save and `git add plan/spec-<name>.md`

---

## Rules

- **Never tick `[ ]` to `[x]` in spec files** -- track progress in per-spec session state
- **`→ Decision:` / `→ Constraint:` annotations are the knowledge** -- they survive compaction
- Each GATE must use `AskUserQuestion` -- never auto-proceed past a gate
- Style: tables and prose, never code snippets in specs (`ai/rules/spec-no-code.md`)
- All research findings go into spec exhaustively (`ai/rules/planning.md`)
- Append-only editing for existing specs (`ai/rules/planning.md`)
- One spec at a time -- this session's marker (`scripts/dev/spec-session.sh`) tracks which
