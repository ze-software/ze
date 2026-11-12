# Spec: a spec's design reaches implementation only with Thomas's approval

| Field | Value |
|-------|-------|
| Status | design |
| Scope | tooling |
| Priority | P2 |
| Release reason | The release must ship behavior that matches its design documentation and Thomas's decisions. On 2026-11-12 a design he never saw, contradicting his own decision, was implemented into the daemon. This spec is the gate that stops that, and it must hold before the first release (`plan/README.md`, `release/validation/`: evidence that shipped behavior works, including accurate claims) |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Design tier | strongest tier |
| Implementation tier | development tier |
| Updated | 2026-11-12 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

On 2026-11-12 the spec `plan/release/fixes/spec-plugin-dispatch-pipe-chain-reaches-the-handler.md`
went from `skeleton` (commit 12531c56ca) to designed, `ready`, claimed,
implemented (2616478e64, 9321d263aa) and closed (ba91ff4dc5) in one day. A
subagent wrote the design, the main session set the spec `ready` in the
working tree (no commit ever held it as `ready`), and Thomas never saw the
design. It made the daemon dispatcher refuse a pipe chain, and it edited
`docs/architecture/api/commands.md` to say so. That contradicts his decision of
2026-08-21 ("the pipe wins", `plan/records/audit-command-pipe-vs-subcommand.md`
line 546, applied to every axis in `plan/records/audit-presentation-pipes.md`
line 59). The decision lived only in those two records. The page that
documents pipes, `docs/architecture/api/commands.md`, the `// Design:` page of
`internal/component/command/pipe.go`, did not carry it.

Every gate on the way passed, and each passed honestly, because none of them
asks the question that failed:

| Gate | What it proved | What it never asks |
|------|----------------|--------------------|
| Spec door gate (`writeSpecDoorsRead`) | The design context read `ai/doors/system.md` and the covering doors in full | Whether the design pages the files link (`// Design:`) were read |
| Code door gate (`writeDesignDoorsRead`) | The implementing context read the doors before each code edit | The same: doors only, never the file's own design page |
| Producer-read gate (`designEvidenceProblem` over `designSubjects`) | The context Read the `.go` files named in Current Behavior and Files to Modify | Whether the design documentation of those files was read |
| Existing Mechanisms gate (`mechanismsProblem`, `hasDataRow`) | The section holds a data row | Anything about meaning: it is structural only |
| `./le spec claim` (`Claim`, `transitionReadySpec`) | The WIP cap had room | Whether anybody approved the design |
| `/ze-review`, `/ze-review-spec`, `/ze-close` | The diff matches the spec, and the spec's own claims | Whether the spec agrees with the design documentation |

`ai/rules/planning.md` line 11 already says `/ze-spec` and `/ze-design` stay in
the main thread. Nothing enforces it.

Thomas, on this incident: "have an agent investigate how you were allowed to
make this shitty plan without reading the code it must NEVER EVER happen
again." And: "make sure specs authored can not go against design documentation
without a discussion with me with the same rules (before after example etc.)".
On where his decisions live: "I am not keen on ruling. We already have plenty
of registers. you are not going to be able to find that information. it should
be part of the documentation, and the documentation format should be updated.
When we have a file which links to a design, the design should be read and
therefore that way the information should be..."

His rules, committed today in `ai/META-DECISIONS.md` (fa219f6120, f8eac4f0d7),
"How agents work":

- "Unless Thomas said he is away, a spec's design is put to him and approved
  before the spec moves to ready, and the main session presents it, never a
  subagent. The design is presented whole: the choice made, the alternatives
  set aside, and every owner ruling it was checked against."
- "A spec never departs from the design documentation (the architecture pages,
  the doors, Thomas's rulings and the records that hold them) without a
  discussion with him. The main session presents the departure the way it
  presents a design: before and after examples of what a user sees, the
  alternatives, and the documentation it contradicts. Once he agrees, the
  documentation is corrected in the same work."
- "A hook checks what a command, path or file is, never how a message is
  worded."

And, "Test changes": "Owner-only authorization actions require an interactive
terminal and refuse noninteractive invocation before recording
authorization."

The goal:

1. The tooling refuses to start the implementation of a design Thomas has not approved, and refuses a subagent's move of a spec to `ready`.
2. Thomas's decisions are written into the design page that documents their subject, in a `## Design decisions` section the documentation format defines and the existing doc check validates. No separate register.
3. The existing link from a file to its design, the `// Design:` header, makes that page owed reading before any code edit or spec edit that touches the file, recorded the way door reads are.
4. A spec lists every design document its files link, and every decision those pages carry, with an `agrees`, `departs` or `silent` verdict. A departure reaches implementation only with Thomas's approval of a presentation with before and after examples.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - the design facts every change keeps
  → Constraint: a fact is declared once and other surfaces derive from it
- [ ] `ai/doors/le-tooling.md` - le, the hooks, the commit flow
  → Constraint: a hook that refuses wrongly blocks every session at once (`plan/learned/HOOK-FRICTION.md`); a guard must not block its own author's repair
  → Constraint: the package sits at the path its command name predicts (`TestEveryCommandIsFoundAtThePathItsNamePredicts`)
  → Constraint: `le` runs the build of HEAD, so a new hook takes effect only once committed
- [ ] `ai/doors/commands-cli.md`, `ai/doors/plugin-infra.md` - the doors the incident spec sat under
  → Decision: no door change; the decision goes into the design page
- [ ] `ai/rules/documentation.md` - read the page before investigating; a change that makes a page wrong carries its page edit
  → Decision: gains the directive that an owner decision is written into the Design decisions section of the page that documents its subject
- [ ] `docs/architecture/README.md` - the index of architecture pages
  → Decision: it becomes the place the architecture page format is defined (no page defines it today)
- [ ] `docs/contributing/documentation-testing.md`, "What gets checked" and "Adding a new documentation check"
  → Constraint: a related check joins an existing area; `./le doc consistency` already judges design references
- [ ] `ai/rules/planning.md` - phases, spec lifecycle, review independence
  → Constraint: line 11, `/ze-spec` and `/ze-design` stay in the main thread
- [ ] `ai/rules/spec-no-code.md` - this spec carries no code
- [ ] `docs/contributing/spec-workflow.md`, "Spec status vocabulary"
  → Constraint: "Spec approved: design to ready" is today an edit anybody can make
- [ ] `docs/contributing/committing.md` - trailers and the routes that write them
  → Constraint: a body line that starts with a declared trailer key is refused (`declaredTrailers`)

**Key insights:**
- The only route to `in-progress` is already `Claim` (`CheckWIPTransition` refuses an edit to it), so a check in `Claim` stops every unapproved start.
- The code-edit gate and the spec-edit gate share one function, `unreadDoors`; extending its owed set with the files' `// Design:` pages extends both gates at once.
- Read records exist only for "gated documents" (`gatedDocument`: the style guide and the doors); the record mechanism itself is generic.
- The harness sends `agent_id` in the hook payload of a subagent's call (`Payload.AgentID`). A fork sends none.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/spec/session.go` - `Claim` reads Status; for `ready` it checks the WIP cap, writes the marker, then `transitionReadySpec` rewrites Status and Updated, under the claim lock. No approval is read. `CheckWIPTransition` refuses an edit that makes a spec `in-progress`
- [ ] `internal/le/hookruntime/specwip.go` - `writeSpecWIP`, the pre-write check that calls `CheckWIPTransition` on the replayed edit
- [ ] `internal/le/hookruntime/writeedit.go` - `writeDesignDoorsRead` owes, before a code edit (`.go`, `.yang`, `.templ`), a whole read of `ai/doors/system.md` and each door covering the file; `unreadDoors` computes the owed set and checks `documentReadMarker` records; `doorRefusal` renders the refusal
- [ ] `internal/le/hookruntime/specdoors.go` - `writeSpecDoorsRead`: the same `unreadDoors` over every repository path the spec names (`specRepoPaths`)
- [ ] `internal/le/hookruntime/readgate.go` - `gatedDocument` admits only the style guide and pages under `ai/doors/` to read records; `documentReadMarker` names the per-agent record; `loadDoors`, `coveringDoors`
- [ ] `internal/le/hookruntime/bash.go` - `bashDocumentRead` records a whole `cat` of a gated document, only when the command names the style guide or `ai/doors/`
- [ ] `internal/le/hookruntime/designevidence.go` - `designSubjects`, `designEvidenceProblem`: source-file reads owed for a spec edit. Markdown is not a subject
- [ ] `internal/le/hookruntime/runtime.go` - `nativeHookActions`, the one list of checks; `Payload.AgentID`; each check needs a `// ze point:` binding
- [ ] `internal/le/doc/consistency/consistency.go` - `checkDesignRefs` warns on a source file with no `// Design:` line. Nothing judges the page a header names
- [ ] `internal/le/spec/mechanisms/specmechanisms.go` - `Problem`, the structural spec judge at edit and commit
- [ ] `internal/le/commit/specmechanisms.go` - `specMechanismsRefusal`, which judges a spec only when the commit changes its text (the precedent for owing a section on next edit)
- [ ] `internal/le/commit/prepare.go` - `checkVerificationGates`, `closureStemSelected`, `specWorkPopulation`
- [ ] `internal/le/commit/actions.go` - `declaredTrailers`
- [ ] `internal/le/spec/citation/anchors.go` - `AuditAnchors`: for Files to Modify and Files to Create, the `// Design:` document (`declaredDesignDocument`, first match only) and every document the doc index anchors to the file (`docindex.DocumentsByPath`)
- [ ] `internal/le/le/human/human.go` - `RequireAuthorization`
- [ ] `internal/le/spec/review_authorization.go` - an owner keyword guarded by `RequireAuthorization`
- [ ] `internal/le/test/weakened/approve.go` - the judge approval route: untracked file under `tmp/`, read by hook and commit gate, turned into a trailer
- [ ] `internal/component/plugin/server/command.go`, `internal/component/command/pipe.go` - the incident spec's main files; their `// Design:` headers name `docs/architecture/api/process-protocol.md` and `docs/architecture/api/commands.md`. The second's header comment states "The DAEMON runs the chain, not the client", which the incident design departed from

**Behavior to preserve:**
- `Claim` on a `skeleton`, `design` or `in-progress` spec writes only the marker
- The WIP cap check, the claim lock and the refusal of `./le spec claim` when the cap is full
- Every obligation the door gates and the producer-read gate hold today
- A page or spec that predates a new section owes it only when an edit next reaches it, never in another session's commit that does not touch it
- `Weakening-approved:` and `RFC-approved:` routes

**Behavior to change:**
- `Claim` refuses `ready` to `in-progress` without an owner approval of the spec as it stands
- A move to `ready` is refused from a subagent, and from the main session without the approval (C-5)
- A code edit and a spec edit owe a whole read of every `// Design:` page of the files they touch, beside the doors
- An architecture page carries a `## Design decisions` section; the doc consistency check validates it, and a commit that edits a design page leaves it valid
- A spec from `ready` onward carries `## Design documents` (every linked design page and every decision on them, with a verdict) and `## Design presentation`
- A `departs` verdict needs Thomas's approval, and closure needs the departed page corrected in the spec's change set

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| The file-to-design link | every source file's `// Design:` header; `<!-- source: -->` anchors rendered into `ai/CODE-TO-DOCS.md` (`docindex.DocumentsByPath`) | Names the page that documents a file; `checkDesignRefs` warns when absent | Extends: the `// Design:` page becomes owed reading, and its Design decisions section is where Thomas's decisions are found. No new index |
| Door read gate, code and spec | `internal/le/hookruntime/writeedit.go` `writeDesignDoorsRead`, `unreadDoors`; `specdoors.go` `writeSpecDoorsRead` | Owes whole reads of the covering doors before a code or spec edit | Extends: `unreadDoors` adds each touched file's `// Design:` pages to the owed set, so both gates owe them with one change and one refusal |
| Document read records | `internal/le/hookruntime/readgate.go` `gatedDocument`, `documentReadMarker`; `bash.go` `bashDocumentRead` | Records a whole Read or `cat` of the style guide or a door | Extends: `gatedDocument` admits any page under `docs/` and `ai/`, so a design page read is recorded the same way |
| Doc consistency gate | `internal/le/doc/consistency/consistency.go` `Check`, `checkDesignRefs`; `./le doc consistency` | Judges design references in source files | Extends: a new check over each page a `// Design:` header names, validating its `## Design decisions` section. Joins the existing area (`documentation-testing.md`, "Adding a new documentation check") |
| Owe-on-next-edit commit judge | `internal/le/commit/specmechanisms.go` `specMechanismsRefusal` | Judges a spec's sections only in a commit that changes its text | Extends the pattern: a commit that changes a design page's text owes that page a valid Design decisions section |
| Owner terminal gate | `internal/le/le/human/human.go` `RequireAuthorization` | Refuses an owner action without interactive stdin and stdout | Extends: `./le spec approve`, `./le spec away` |
| Judge approval file and trailer | `internal/le/test/weakened/approve.go`; `internal/le/commit/actions.go` `declaredTrailers` | Untracked file read by hook and commit gate; trailer per used row; hand-written trailer refused | Extends the pattern: an approval record keyed by spec, a `Design-approved:` trailer |
| Claim and the ready transition | `internal/le/spec/session.go` `Claim`, `CheckWIPTransition` | The only route to `in-progress` | Extends: checks the approval or away admission before the cap |
| WIP edit check | `internal/le/hookruntime/specwip.go` `writeSpecWIP` | Refuses an edit that makes a spec `in-progress` | Extends the pattern: a sibling check for `ready` |
| Doc anchor derivation | `internal/le/spec/citation/anchors.go` `AuditAnchors`, `./le spec citation anchors` | Lists the documents a spec's source files declare or are anchored by | Extends: the same derivation, restricted to design pages and doors, plus each page's Design decisions rows, yields the rows `## Design documents` owes |
| Closure change set | `internal/le/commit/prepare.go` `checkVerificationGates`, `specWorkPopulation` | Finds a closure and every path its `Spec-work:` commits touched | Extends: refuses a `departs` row whose page the change set does not touch |
| Verbatim quote check | `docs/contributing/rfc-conformance-gates.md`, "The row quote" | Refuses an RFC row whose quote is not a span of its section | Extends the idea: an `agrees` or `departs` row quotes a verbatim span of its page |
| Independent judge skill | `.claude/skills/ze-test-judge/SKILL.md` | An agent that did not make a change judges it | Extends the pattern: `ze-design-judge` (C-3) |
| Doors | `ai/doors/*.md` | Subsystem contracts, boundaries, traps | Does not apply as a home for decisions: a decision lives in the page that documents its subject, which the file links |
| Records | `plan/records/` | Audit and investigation history | Does not apply as a home: a record is the evidence a decision row cites as its Source |
| `ai/META-DECISIONS.md` | owner rules on how agents work and on scope | Loaded by every main session | Does not apply to product design decisions; Thomas's new rule (decisions go in the documentation) belongs there, written by the main session |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An agent's Write or Edit of a code file or a spec (pre-write hook payload: tool, path, proposed text, `agent_id`)
- An agent's Read or `cat` of a page (post-read and pre-Bash hooks write the read record)
- `./le spec claim <spec-file>`; `./le spec approve`, `./le spec away` from Thomas's terminal
- `./le commit create` carrying a design page or a spec, or closing a spec
- `./le doc consistency`

### Transformation Path
1. Thomas states a decision; the main session writes it into the `## Design decisions` section of the page that documents the subject, in the same turn, with the record as its Source
2. Before an agent edits a code file, `unreadDoors` owes the doors and the file's `// Design:` pages; reading the page puts the decision in the agent's context
3. A spec author runs `./le spec citation design spec <file>`, which prints every linked design page and every Design decisions row on those pages; the author gives each a verdict in `## Design documents` and writes `## Design presentation`
4. The pre-write hook judges the table against the same derivation once the proposed Status is `ready`
5. An independent judge confirms the verdicts (C-3); Thomas approves from his terminal (`./le spec approve`), which writes a digest-bound record
6. The main session sets `ready`; `./le spec claim` admits it; `./le commit create` writes `Design-approved:` from the record
7. At closure, a `departs` row owes an edit of its page in the change set; the page's Design decisions section then states the new decision

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Owner terminal to agent sessions | Untracked approval record under `tmp/` | No |
| One checkout to another (Mac, Linux) | `Design-approved:` trailer reachable from HEAD | No |
| Hook process to le packages | Library calls; the doc consistency validator exported for the commit gate | No |

### Changed-Wire Census

**Search evidence:** scope `internal/le/`, `docs/`, `.claude/`; terms `## Design decisions`, `gatedDocument`, `documentReadMarker`, `Design-approved`, `owner-away`, `## Design documents`, `## Design presentation`, `tmp/commit-`. Positive control: the door read record has its writers (`hookSourceRead`, `bashDocumentRead`) and its reader (`unreadDoors`). Finding: `## Design decisions`, `Design-approved`, `owner-away` and the two spec headings occur nowhere today; the read record exists and only its admitted documents change.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| Document read record | writer | `internal/le/hookruntime/readgate.go` `gatedDocument` via the Read hook | Admits the style guide and doors only | Change: admits any page under `docs/` and `ai/` |
| Document read record | writer | `internal/le/hookruntime/bash.go` `bashDocumentRead` | Records only commands naming the style guide or `ai/doors/` | Change: also commands naming a page under `docs/` |
| Document read record | reader | `internal/le/hookruntime/writeedit.go` `unreadDoors` | Owes doors only | Change: also owes `// Design:` pages |
| Design decisions section | writer | the main session, in the page that documents the subject | New heading | New; format defined in `docs/architecture/README.md` |
| Design decisions section | reader | `internal/le/doc/consistency/consistency.go` new check; `internal/le/commit/designdecisions.go` (new); `./le spec citation design` | New | New |
| Approval record | writer | `internal/le/spec/approve/` approve action (new), file `tmp/spec-design-approved/<stem>.md` | No such file today | New; Spec, Digest, Departs approved, Owner words, Date, Machine |
| Approval record | reader | `internal/le/spec/session.go` `Claim` | Reads only Status and the cap today | Change |
| Approval record | reader | `internal/le/hookruntime/specready.go` (new) | No ready-edit check exists | New |
| Approval record | reader | `internal/le/commit/designapproval.go` (new) | `declaredTrailers` route list | New |
| Away record | writer | `internal/le/spec/away/` away action (new), file `tmp/owner-away.md` | No such file today | New; from, until, owner words |
| Away record | reader | `internal/le/spec/session.go` `Claim` | New | New |
| Away record | reader | `internal/le/hookruntime/specready.go` (new) | New | New |
| Design-approved trailer | writer | `internal/le/commit/designapproval.go` from the approval record | `declaredTrailers` | New key, `OwesNothing` true |
| Design-approved trailer | reader | `internal/le/spec/session.go` `Claim`, over commits reachable from HEAD | New | New |
| Design-approved trailer | reader | `internal/le/commit/designapproval.go` closure check | New | New |
| Judge record | writer | judge action in `internal/le/spec/approve/`, file `tmp/spec-design-judged/<stem>.md` | New | New; only if C-3 is taken |
| Judge record | reader | the approve action in `internal/le/spec/approve/` | New | New |
| Design documents table | writer | the spec author, from `./le spec citation design` | New heading | New, owed from `ready` |
| Design documents table | reader | `internal/le/hookruntime/specdesigndocs.go` (new), `internal/le/commit/designapproval.go`, the approve action, the judge and review skills | New | New |
| Design presentation section | writer | the spec author, `/ze-spec` | New heading | New, owed by `./le spec approve` |
| Design presentation section | reader | the approve action in `internal/le/spec/approve/` | New | New |

### Integration Points
- `unreadDoors` - one more source of owed pages (the files' `// Design:` lines, from disk and from the proposed text)
- `gatedDocument`, `bashDocumentRead` - wider admission
- `docconsistency.Check` - one more check, exported for the commit gate
- `Claim` - one approval check before the WIP check
- `nativeHookActions` - `writeSpecReady`, `writeSpecDesignDocuments`, `bashOwnerOnly`
- `declaredTrailers` - one new key
- `speccitation` - one derivation and action beside `AuditAnchors`

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | Reads flow through the existing record and door gate; approval through the owner-terminal gate and the claim route |
| No unintended coupling | Yes | The commit gate calls the consistency validator as a library; no le package imports `hookruntime` |
| No duplicated functionality | Yes | No second gate, no second index: the `// Design:` link and the doc index are the only sources |
| Zero-copy preserved where applicable | N-A | Tooling |
| Registration over hardcoding, outbound | Yes | New actions register from `init()`; checks join the one registry |
| Registration over hardcoding, inbound | Yes | No list of pages or decisions exists in code: the pages come from each file's own header, the decisions from each page's own section |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Claude Code puts `agent_id` in every subagent hook payload and the agent cannot alter it | `Payload.AgentID`; the call budget relies on it | Subagent detection fails open | Hook fixture plus a live probe | unvalidated |
| A-2 | A fork's payload carries no `agent_id` | the start-text producer's comment in `lifecycle.go` | A fork is judged as the main session | Covered by C-5 | unvalidated |
| A-3 | `docindex.DocumentsByPath` is fast enough for a pre-write hook | None measured | Every spec edit slows | Time it on the incident spec; fall back to judging at the ready transition and at commit | unvalidated |
| A-4 | Thomas runs le from a real terminal on both machines | five owner actions already use the gate | He cannot approve | His first approval | unvalidated |
| A-5 | Reading a whole design page before a first code edit is affordable | `docs/architecture/api/commands.md` and similar pages are long | Agent context cost grows | Measure the largest `// Design:` targets; the record is per agent, so each page is read once per agent | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | 90 specs are `ready` today with no approval; the claim gate stops all work at landing | `./le spec claim` refusals | Choice C-2 |
| R-2 | The wider read gate refuses every code edit in every session the moment it lands | A wave of refusals naming design pages | The refusal names each page and the read that clears it; fixtures for a file with no header (nothing more owed) and a header naming a missing page (refused, fail-closed, naming the header to fix) |
| R-3 | A `// Design:` header names a page that does not exist | A refusal on an unrelated edit | Refusal names the file and its header line; `./le doc index check` already resolves these, so the count is known before landing |
| R-4 | Hundreds of design pages carry no Design decisions section | `./le doc consistency` warnings | Warning tree-wide (a red owed at release, never a commit refusal), refusal only in a commit that edits the page |
| R-5 | An agent fakes a terminal to run approve | A record written inside a harness session | `bashOwnerOnly`; residue in Known Limitations |
| R-6 | The digest stales on harmless edits | Claim refusals right after approval | Digest blanks Status, Updated and Phase |
| R-7 | Authors fill `silent` rows by rote | Tables of 15+ silent rows | Derivation restricted to design pages; the judge reviews every `silent` row |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Every session's code edits, spec edits and claims: nothing user-visible in `ze` |
| How is it reverted? | Single commits per phase; the records are untracked and inert without the readers |
| Who else touches this path? | Every session through the hooks; the commit flow; whoever edits design pages |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Pre-write hook, code edit of a file whose `// Design:` page is unread | → | `unreadDoors` via `writeDesignDoorsRead` | `./le ai hooks unit` fixture `code-edit-owes-design-page` |
| Pre-write hook, spec naming such a file | → | `unreadDoors` via `writeSpecDoorsRead` | fixture `spec-edit-owes-design-page` |
| `./le doc consistency` over a design page without the section | → | design-decisions check | `TestDesignDecisionsSectionJudged` |
| `./le commit create` editing a design page | → | `internal/le/commit/designdecisions.go` | `TestCommitRefusesEditedDesignPageWithoutDecisions` |
| `./le spec claim` on a ready spec | → | `Claim` approval check | `TestClaimRefusesReadySpecWithoutApproval` |
| `./le spec approve` without a terminal | → | approve action | `TestApproveRefusesWithoutTerminal` |
| Pre-write hook, subagent payload, Status to ready | → | `writeSpecReady` | fixture `spec-ready-from-subagent` |
| Pre-Bash hook, command naming `spec approve` | → | `bashOwnerOnly` | fixture `bash-owner-only-approve` |
| `./le commit create` closing a spec with an uncorrected `departs` page | → | closure check | `TestClosureRefusesUncorrectedDeparture` |
| The incident, replayed | → | all of the above | `TestIncidentPipeChainSpecIsRefused` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A context that read the doors but not `docs/architecture/api/commands.md` edits `internal/component/command/pipe.go` (header `// Design: docs/architecture/api/commands.md`) | Exit 2: `BLOCKED: this session or agent has not read the design that covers internal/component/command/pipe.go:` / `- docs/architecture/api/commands.md (its // Design: page)` / `why: the page a file links holds its design and the decisions Thomas made about it` / `fix: read each listed page whole with the Read tool, or cat it with no pipe, then retry` | The same edit after a whole read; a file with no `// Design:` line owes only its doors, as today |
| AC-2 | A Write creating a new code file whose proposed text carries a `// Design:` line | The page that line names is owed exactly as AC-1 | A new file with no header, which owes its doors (and `./le doc consistency` warns, as today) |
| AC-3 | A spec edit naming a code file whose design page is unread | `writeSpecDoorsRead` refuses with the same list, design pages beside doors | A spec naming no code file, which owes `ai/doors/system.md` alone |
| AC-4 | A `// Design:` header names a page that does not exist | Exit 2 naming the file, the header line and `fix: correct the header to name the page that documents this file` | None: a dangling link would otherwise release the file in silence |
| AC-5 | `./le doc consistency` on a page that some `// Design:` header names | Each page without a `## Design decisions` section, or with a row whose Decision, It forbids or Source cell is empty, or whose Source names a path that does not resolve, is reported as `design-decisions` with the page and the row | A section whose table has a header and no data row: a page that binds no owner decision states so by an empty table |
| AC-6 | `./le commit create` carrying an edit to the text of a page that a `// Design:` header names, where the result lacks a valid Design decisions section | Refused: `<page>: a design page owes a ## Design decisions section (docs/architecture/README.md, "Page format")` with the failing rows | A commit that does not change the page's text; pages never edited stay warnings (AC-5) |
| AC-7 | `./le spec citation design spec <path>` | Prints one row per design page the spec's files link (`// Design:` pages, architecture pages the doc index anchors to them, covering doors) and one row per Design decisions row on those pages, each with what derived it | Guides and feature pages are left out: they are not design documentation |
| AC-8 | A spec edit whose proposed Status is `ready` or later with no `## Design documents` section, a derived row missing, a verdict other than `agrees`, `departs`, `silent`, a quote that is not a verbatim span of its page, a `silent` row with no reason, a page the spec modifies marked `silent`, or a Design decisions row marked `silent` | Exit 2 naming each failing row, with `fix: run ./le spec citation design spec <path> to list the owed rows` | A `design` spec may omit the section or hold it unfinished |
| AC-9 | `./le spec claim spec-x.md`, Status `ready`, no matching approval record or trailer, Thomas not away | Exits 2: `refusing to start spec-x.md: its design has no owner approval, and only Thomas records one` / `why: a design reaches implementation only once Thomas approved it (ai/META-DECISIONS.md, "How agents work")` / `fix: present its ## Design presentation to Thomas; he runs ./le spec approve spec plan/<bucket>/spec-x.md words "<what he said>" from his terminal`. Status stays `ready`, the session keeps its previous claim | A claim of a `skeleton`, `design` or `in-progress` spec writes the marker, because none starts implementation |
| AC-10 | Same, approval record whose digest matches (Status, Updated, Phase blanked) | Claim proceeds as today, and the report gains `approved: <date> "<owner words>"` | Every claim within the cap that carries an approval |
| AC-11 | Record exists, spec text changed after it | Exits 2: `its design changed after Thomas approved it on <date>` with the re-approve command | Changes to the three blanked cells |
| AC-12 | No local record, a commit reachable from HEAD carries a matching `Design-approved:` | Claim proceeds as AC-10 | A spec approved on another machine |
| AC-13 | `./le spec approve` with stdin or stdout not a terminal | Exits 2 with `human.RequireAuthorization`'s refusal naming the command for Thomas; the approval directory is left as it was | None |
| AC-14 | `./le spec approve` from a terminal on a spec lacking a non-empty `### Before and after`, `### Alternatives set aside`, `### Documents departed from` or `### Choices for Thomas`, or whose Design documents table fails AC-8 | Exits 2 naming each missing subsection or failing row | `### Documents departed from` may state "none" when no row is `departs` |
| AC-15 | `./le spec approve` succeeds | Writes `tmp/spec-design-approved/<stem>.md` with digest, words, date, machine and every `departs` row; prints the digest and the next step | Re-approval replaces that spec's record only |
| AC-16 | Pre-write: an edit making a spec `ready` (from another status or as a new file), payload carries `agent_id` | Exit 2: `BLOCKED: <path>: a subagent cannot move a spec to ready` / `why: only the main session presents a design to Thomas, and he approves it` / `fix: hand the design back to the main session and stop` | A subagent's edits that leave Status as it is |
| AC-17 | Same edit from the main session with no matching approval, Thomas not away | Exit 2: `<path> cannot be marked ready: its design has no owner approval matching this text` with the approve command | The same edit once the record matches (C-5) |
| AC-18 | `./le spec away start until <YYYY-MM-DD> words "..."` from a terminal | Writes `tmp/owner-away.md`; `./le spec away end` removes it; both refuse without a terminal | The record lapses on its own after `until` |
| AC-19 | Thomas away, a `ready` spec under `plan/release/fixes/` with no `departs` row, no approval | Claim proceeds and reports `admitted while Thomas is away (<from> to <until>): no design approval; he reviews it on return`; the next commit carrying the spec writes `Design-approved: <stem> <digest>: away <from>..<until>` | Only fix specs: the away rule covers "a fix spec's design" |
| AC-20 | Thomas away, a `departs` row or a spec outside `release/fixes/` | Claim refuses as AC-9, adding `while Thomas is away, only a fix spec with no departure starts without his approval` | None |
| AC-21 | Closing a spec with a `departs` row whose page the change set does not modify | Refused: `closure of spec-x: it departs from <page>, and the change set does not correct it` / `fix: correct <page>, and its Design decisions section, to state the approved design in this work` | A `departs` row whose page the change set modifies |
| AC-22 | Closing a spec with a `departs` row no approval record or trailer of this spec lists | Refused: `spec-x departs from <page> without Thomas's approval` | A departure approved during implementation (approve accepts `in-progress`) |
| AC-23 | An agent's Bash command whose words include `le spec approve` or `le spec away`, wrapped or not | Exit 2: `BLOCKED: ./le spec approve is Thomas's command, run from his own terminal outside any agent session` / `fix: give Thomas the command and stop` | `./le help spec approve`, which records nothing |
| AC-24 | A commit body line starting `Design-approved:` | Refused by the existing hand-written trailer refusal | Prose naming the key inside a line |
| AC-25 | The incident: spec text of 2616478e64 with Status `ready`; `docs/architecture/api/commands.md` carrying a Design decisions row for the 2026-08-21 decision | Claim refused (AC-9); a subagent's ready edit refused (AC-16); an edit of `pipe.go` by a context that never read `commands.md` refused (AC-1); `./le spec citation design` lists `docs/architecture/api/commands.md`, `docs/architecture/api/process-protocol.md`, both doors and the decision row; a table that omits the decision row is refused (AC-8) | None: this is the case the spec exists to stop |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestUnreadDoorsOwesDesignPages` | `internal/le/hookruntime/writeedit_test.go` | AC-1, AC-2 (header read from proposed text), every `// Design:` line of the header, not only the first | |
| `TestUnreadDoorsRefusesDanglingDesign` | `internal/le/hookruntime/writeedit_test.go` | AC-4 | |
| `TestGatedDocumentAdmitsDesignPages` | `internal/le/hookruntime/readgate_test.go` (new) | a Read or `cat` of a page under `docs/` writes the record | |
| `TestDesignDecisionsSectionJudged` | `internal/le/doc/consistency/consistency_test.go` | AC-5 rows, empty table accepted | |
| `TestCommitRefusesEditedDesignPageWithoutDecisions` | `internal/le/commit/designdecisions_test.go` | AC-6, and an unedited page carried by the commit is accepted | |
| `TestDesignDocumentsDerivation` | `internal/le/spec/citation/design_test.go` | AC-7 | |
| `TestDesignDocumentsJudgement` | `internal/le/spec/citation/design_test.go` | AC-8 | |
| `TestClaimRefusesReadySpecWithoutApproval` | `internal/le/spec/session_test.go` | AC-9 | |
| `TestClaimAdmitsMatchingApproval` | `internal/le/spec/session_test.go` | AC-10 | |
| `TestClaimRefusesStaleApproval` | `internal/le/spec/session_test.go` | AC-11 | |
| `TestClaimAdmitsApprovalTrailer` | `internal/le/spec/session_test.go` | AC-12 | |
| `TestClaimAwayAdmitsFixSpecOnly` | `internal/le/spec/session_test.go` | AC-19, AC-20, lapse after `until` | |
| `TestApproveRefusesWithoutTerminal` | `internal/le/spec/approve/approve_test.go` | AC-13 | |
| `TestApproveRefusesIncompleteSpec` | `internal/le/spec/approve/approve_test.go` | AC-14 | |
| `TestApproveWritesDigestBoundRecord` | `internal/le/spec/approve/approve_test.go` | AC-15 | |
| `TestAwayRequiresTerminalAndUntil` | `internal/le/spec/away/away_test.go` | AC-18 | |
| `TestClosureRefusesUncorrectedDeparture` | `internal/le/commit/designapproval_test.go` | AC-21 | |
| `TestClosureRefusesUnapprovedDeparture` | `internal/le/commit/designapproval_test.go` | AC-22 | |
| `TestDesignApprovedTrailerWrittenFromRecord` | `internal/le/commit/designapproval_test.go` | AC-12 writer side, AC-24 | |
| `TestIncidentPipeChainSpecIsRefused` | `internal/le/spec/citation/design_test.go`, testdata from 2616478e64 | AC-25: the original failure refused at the code edit, the table, the ready edit and the claim | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| away `until` | today onward | today | yesterday (already over) | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| fixture `code-edit-owes-design-page` | the hook fixture tree run by `./le ai hooks unit` | AC-1 | |
| fixture `code-edit-after-design-page-read` | same | AC-1 survivor | |
| fixture `spec-edit-owes-design-page` | same | AC-3 | |
| fixture `spec-ready-from-subagent` | same | AC-16 | |
| fixture `spec-ready-main-without-approval` | same | AC-17 | |
| fixture `spec-design-status-without-section` | same | AC-8 survivor: accepted | |
| fixture `bash-owner-only-approve` | same | AC-23, three wrapper shapes | |
| fixture `bash-owner-only-help` | same | AC-23 survivor | |

### Interop Tests (Scope: protocol)
N-A: tooling, no protocol peer.

## Files to Modify
- `internal/le/hookruntime/writeedit.go` - `unreadDoors` owes the `// Design:` pages of each file (from disk and from the proposed text); `doorRefusal` names design pages beside doors
- `internal/le/hookruntime/readgate.go` - `gatedDocument` admits pages under `docs/` and `ai/`
- `internal/le/hookruntime/bash.go` - `bashDocumentRead` records a whole `cat` of such a page
- `internal/le/hookruntime/runtime.go` - register `writeSpecReady`, `writeSpecDesignDocuments`, `bashOwnerOnly`
- `internal/le/doc/consistency/consistency.go` - the design-decisions check, exported for the commit gate
- `internal/le/spec/session.go` - `Claim`: approval and away check before the WIP check
- `internal/le/spec/citation/anchors.go` - `declaredDesignDocument` returns every `// Design:` line
- `internal/le/commit/actions.go` - `declaredTrailers` gains `Design-approved:`
- `internal/le/commit/prepare.go` - call the design-page judge, the trailer writer and the closure departure check
- `docs/architecture/README.md` - new "Page format" section defining `## Design decisions`
- `docs/architecture/api/commands.md` - first Design decisions row: the 2026-08-21 pipe decision (Source: the two records and the META rule of fa219f6120); correct the dispatcher text the incident wrote if it contradicts it (to be judged in phase 3)
- `docs/contributing/documentation-testing.md` - the new consistency check in "What gets checked"
- `ai/rules/documentation.md` - directive: an owner decision is written into the Design decisions section of the page that documents its subject; a code edit owes the file's design page
- `ai/rules/planning.md` - the approval directive
- `ai/rules/points/planning/`, `ai/rules/points/documentation/`, `ai/rules/points/architecture/` - point files binding the new and widened checks (`ai/rules/rule-format.md`)
- `plan/templates/TEMPLATE.md` - `## Design documents` and `## Design presentation`
- `docs/contributing/spec-workflow.md` - `ready` means approved; "Spec approved" names `./le spec approve`; the two sections; away mode
- `docs/contributing/committing.md` - the `Design-approved:` trailer; a design page edit owes its section
- `docs/features/ai-first.md` - the `./le commit create` paragraph (the `// Design:` page of `internal/le/commit/prepare.go`): checked; it changes only if it lists the trailer routes
- `.claude/skills/ze-spec/SKILL.md` - write the two sections, run the judge, present, hand Thomas the approve command, then set `ready`
- `.claude/skills/ze-review/SKILL.md`, `.claude/skills/ze-review-spec/SKILL.md`, `.claude/skills/ze-close/SKILL.md` - the design-documents lens
- `plan/learned/HOOK-FRICTION.md` - known false positives of the widened gate once observed

## Files to Create
- `internal/le/hookruntime/readgate_test.go` - read record admission
- `internal/le/spec/approve/approve.go`, `internal/le/spec/approve/register.go`, `internal/le/spec/approve/approve_test.go` - `./le spec approve`, record format, digest, lookup (no import of `internal/le/spec`)
- `internal/le/spec/away/away.go`, `internal/le/spec/away/register.go`, `internal/le/spec/away/away_test.go` - `./le spec away start|end`
- `internal/le/spec/citation/design.go`, `internal/le/spec/citation/design_test.go` - the derivation, the judgment, `./le spec citation design`
- `internal/le/spec/citation/testdata/incident-pipe-chain.md` - the incident spec text (2616478e64, Status set to `ready`)
- `internal/le/hookruntime/specready.go` - `writeSpecReady`
- `internal/le/hookruntime/specdesigndocs.go` - `writeSpecDesignDocuments`
- `internal/le/hookruntime/owneronly.go` - `bashOwnerOnly`
- `internal/le/commit/designdecisions.go`, `internal/le/commit/designdecisions_test.go` - a commit that edits a design page owes its section
- `internal/le/commit/designapproval.go`, `internal/le/commit/designapproval_test.go` - trailer writer and closure checks
- `.claude/skills/ze-design-judge/SKILL.md` - the independent judge (C-3)

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | le tooling |
| YANG validation constraints | N-A | no leaf |
| YANG custom validators | N-A | no leaf |
| CLI commands/flags | Yes | `./le spec approve`, `./le spec away`, `./le spec citation design` |
| CLI grammar (keyword before value) | Yes | `spec <path> words "<text>"`, `until <date>`; `./le cli grammar` |
| Editor autocomplete | N-A | le completion derives from registration |
| Functional test for new RPC/API | Yes | hook fixtures above |
| Pipe completeness | N-A | shared le renderer |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | N-A | no `ze` runtime dependency |
| Doctor code per problem class | N-A | le refusals |
| Prometheus counters/metrics | N-A | none |
| BGP family surface | N-A | none |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | N-A | developer tooling |
| 2 | Config syntax changed? | N-A | |
| 3 | CLI command added/changed? | N-A | `ze` unchanged |
| 4 | API/RPC added/changed? | N-A | |
| 5 | Plugin added/changed? | N-A | |
| 6 | Has a user guide page? | N-A | |
| 7 | Wire format changed? | N-A | |
| 8 | Plugin SDK/protocol changed? | N-A | |
| 9 | RFC behavior? | N-A | |
| 10 | Test infrastructure changed? | Yes | `docs/contributing/spec-workflow.md`, `docs/contributing/committing.md`, `docs/contributing/documentation-testing.md` |
| 11 | Affects daemon comparison? | N-A | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/README.md` (page format) |
| 13 | Route metadata keys? | N-A | |
| 14 | Prometheus counters? | N-A | |
| 15 | Registered inventory changed? | N-A | |
| 16 | Changed source files referenced by doc anchors? | Yes | derived by `./le spec citation anchors spec plan/release/validation/spec-design-needs-owner-approval.md` at implementation |
| 17 | Existing docs show examples for this area? | Yes | `docs/contributing/spec-workflow.md` event table |

## The documentation format change

`docs/architecture/README.md` gains a "Page format" section. It defines one
required section on every page that a `// Design:` header names:

| Element | Rule |
|---------|------|
| Heading | `## Design decisions`, once per page |
| Table | Columns `Decision`, `It forbids`, `Source`, `Date` |
| Decision | The decision as Thomas stated it, about this page's subject |
| It forbids | What a design must not do because of it (the shape a reviewer checks a diff against) |
| Source | A tracked path (the record holding his words), a commit, or `ai/META-DECISIONS.md` with its section |
| Date | `YYYY-MM-DD` |
| No decision | The table is present with no data row |

The check is a new `design-decisions` check in `./le doc consistency`
(`internal/le/doc/consistency/consistency.go`), beside `checkDesignRefs`. It
reads every `// Design:` target and reports a page with no section, a row with
an empty cell, an unresolvable Source path, or a malformed date. Tree-wide it
is a warning, so the pages that predate the format are a red owed at release.
`./le commit create` refuses a commit that changes a design page's text and
leaves the page failing the check, the way `specMechanismsRefusal` owes a
spec's sections on its next edit. No register is added: the decision lives in
the page the file links, and the link is what makes it read.

## Design documents

Filled by hand for this spec: the derivation does not exist yet, so the judge of C-3 re-checks it once it does.

| Document | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | every path | "A feature registers from `init()` in its own `register.go` and is discovered." | agrees | Actions register; pages and decisions come from each file's own header and each page's own section |
| `ai/doors/le-tooling.md` | `internal/le/` | "Areas register." | agrees | Same |
| `docs/contributing/spec-workflow.md` | the spec lifecycle this spec changes | "Design complete, ready for implementation" | departs | `ready` comes to mean "design approved by Thomas"; the page is corrected in this work |
| `docs/contributing/committing.md` | Design header of `internal/le/commit/specmechanisms.go` | the routes that write each trailer | agrees | One more declared trailer, same refusal |
| `docs/contributing/documentation-testing.md` | the doc check this spec extends | "A related check joins an existing area rather than opening another root name." | agrees | The new check joins `./le doc consistency` |
| `docs/features/ai-first.md` | Design header of `internal/le/commit/prepare.go` | "`./le commit create` prepares a commit" | agrees | `create` still prepares the commit |
| `docs/architecture/core-design.md` | Design header of `internal/le/spec/session.go`, `internal/le/hookruntime/specdoors.go`, `internal/le/spec/citation/anchors.go`, `internal/le/doc/consistency/consistency.go` | "Command engine and composition roots" | agrees | No second command engine |
| `ai/rules/planning.md` | the rule this spec enforces | "Four MUST stay in the main thread" | agrees | Enforced for the ready transition |

## Design presentation

### What changes, in one paragraph

Thomas's decisions are written into the design page that documents their
subject, in a `## Design decisions` section the page format now defines and
`./le doc consistency` validates. The link a file already carries to its
design, the `// Design:` header, makes that page owed reading before any code
edit or spec edit that touches the file, recorded like a door read. A spec
lists every design page its files link and every decision on them, with a
verdict, and a `departs` verdict, like the design as a whole, reaches
implementation only after Thomas approves it from his own terminal.

### Before and after

**1. A code edit before the file's design page is read.**

Before: only the doors are owed, so an agent edits `pipe.go` without ever opening `commands.md`.

After:

> BLOCKED: this session or agent has not read the design that covers internal/component/command/pipe.go:
>   - docs/architecture/api/commands.md  (its // Design: page)
>   why: the page a file links holds its design and the decisions Thomas made about it
>   fix: read each listed page whole with the Read tool, or cat it with no pipe, then retry

Once read, the agent has seen the page's Design decisions section:

> | Decision | It forbids | Source | Date |
> | A command path names the subject and stops; scope, presentation and format are pipe operations after it, and the pipe wins over a subcommand | Rewriting the command line to carry a pipe as an argument; refusing a pipe a command declares; a subcommand for scope or presentation | plan/records/audit-command-pipe-vs-subcommand.md, ai/META-DECISIONS.md "Code and dependencies" | 2026-08-21 |

**2. A spec missing a design page.**

Before: nothing lists the page or the decision, so nobody checks the design against them.

After (the edit that sets Status to ready, or `./le spec approve`):

> BLOCKED: spec-plugin-dispatch-pipe-chain-reaches-the-handler.md: ## Design documents is missing 2 rows owed by its files
>   - docs/architecture/api/commands.md, Design decision of 2026-08-21  (the // Design: page of internal/component/command/pipe.go)
>   - docs/architecture/command-ownership.md  (anchored to internal/component/plugin/server/command.go)
>   why: a spec never departs from the design documentation without a discussion with Thomas, so it gives a verdict on every design page its files link
>   fix: run ./le spec citation design spec plan/release/fixes/spec-plugin-dispatch-pipe-chain-reaches-the-handler.md and add a row with a verdict for each

**3. A design page lacking the decisions section.**

Before: a page can be edited, and decisions left in a record, with no check.

After, tree-wide (`./le doc consistency`, a warning):

> WARN design-decisions docs/architecture/api/process-protocol.md: no ## Design decisions section (named by 41 // Design: headers)

After, in a commit that edits the page:

> refused: docs/architecture/api/process-protocol.md: a design page owes a ## Design decisions section (docs/architecture/README.md, "Page format")
>   fix: add the section; a page that binds no owner decision carries the table with no row

**4. A departs row.**

> | docs/architecture/api/commands.md, decision of 2026-08-21 | Design decisions of the // Design: page of pipe.go | "the pipe wins over a subcommand" | departs | The dispatcher refuses a pipe instead of running it |

Without Thomas's approval the claim is refused (5 below); added during implementation, closure is refused:

> refused: spec-plugin-dispatch-pipe-chain-reaches-the-handler departs from docs/architecture/api/commands.md without Thomas's approval
>   fix: present the departure; Thomas runs ./le spec approve spec <path> words "<what he said>"

With approval, closure still refuses until the page and its Design decisions row are corrected in the same work:

> refused: closure of spec-plugin-dispatch-pipe-chain-reaches-the-handler: it departs from docs/architecture/api/commands.md, and the change set does not correct it

**5. A claim of a spec nobody approved.**

Before: `claimed ...: ready -> in-progress (WIP 9/12)`.

After:

> refusing to start spec-plugin-dispatch-pipe-chain-reaches-the-handler.md: its design has no owner approval, and only Thomas records one
>   why: a design reaches implementation only once Thomas approved it (ai/META-DECISIONS.md, "How agents work")
>   fix: present its ## Design presentation to Thomas; he runs, from his own terminal:
>     ./le spec approve spec plan/release/fixes/spec-plugin-dispatch-pipe-chain-reaches-the-handler.md words "<what he said>"

**6. A subagent sets the spec ready.**

> BLOCKED: plan/release/fixes/spec-plugin-dispatch-pipe-chain-reaches-the-handler.md: a subagent cannot move a spec to ready
>   why: only the main session presents a design to Thomas, and he approves it
>   fix: hand the design back to the main session and stop

**7. Thomas approves.**

From an agent: the Bash hook refuses before le starts (`./le spec approve is Thomas's command, run from his own terminal outside any agent session`), and any non-terminal run gets the owner-terminal refusal naming the command. From his terminal:

> $ ./le spec approve spec plan/release/fixes/spec-x.md words "approved, the pipe stays the operator surface"
> approved spec-x.md (digest 3f9a1c0e72b4): design documents 9 (8 agrees, 0 departs, 1 silent), judge verdict recorded 2026-11-12
> next: the main session sets Status to ready; the next commit carrying the spec records Design-approved

**8. Away.**

> $ ./le spec away start until 2026-11-20 words "away until the 20th"
> Thomas away 2026-11-12 to 2026-11-20: a ready fix spec with no departs row starts without approval; every such start is recorded for your review

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| A rulings register (pages, door tables or an index of decisions) | Thomas rejected it: another register nobody finds. The page a file links is where an agent already looks |
| Decisions only in `plan/records/` | The incident: a record no link reaches is never read |
| Decisions in the doors | A door covers a subsystem, not the subject of one page, and a file links its page, not its door's decision |
| Owing every page `ai/CODE-TO-DOCS.md` anchors to a file before each code edit | Eleven pages, guides included, for `command.go` alone. The `// Design:` link is the declared design; the anchored architecture pages enter the spec's Design documents list instead (C-1) |
| A separate gate for design pages | `unreadDoors` already serves both the code and the spec gate; a second gate would be a parallel copy of it |
| Approval as a metadata row | An agent can write it; a hook cannot tell the main session from Thomas |
| Approval only as a commit trailer | The incident's `ready` never reached a commit; the record must exist in the working tree |
| A hook that judges "contradicts" | It would read meaning from wording; the hook judges completeness and form, the judge agent and Thomas judge meaning |

### Documents departed from

`docs/contributing/spec-workflow.md`, "Spec status vocabulary": `ready` reads
"Design complete, ready for implementation" and "Spec approved" is an edit
anybody makes. After: `ready` means Thomas approved the design, or it is a fix
spec started while he was away. Corrected in this work.

### Choices for Thomas

| # | Choice | Options | Recommendation |
|---|--------|---------|----------------|
| C-1 | Which links owe a whole read before a code edit | (a) the file's `// Design:` pages only; (b) also every architecture page the doc index anchors to the file | (a): the header is the declared link and keeps the cost to one or two pages; the anchored architecture pages still reach every spec through its Design documents list, where each gets a verdict |
| C-2 | The 90 specs already `ready` (26 in `release/fixes/`) | (a) each needs your approval before its claim; (b) a baseline you record from your terminal that approves them as they stand; (c) back to `design` | (a) with a printed list by priority; (b) only if the queue cannot wait |
| C-3 | An independent design judge before presentation | (a) required, its read records covering every listed page; (b) optional; (c) none | (a) |
| C-4 | How away is bounded | (a) `until <date>` required; (b) open until `away end` | (a), no maximum |
| C-5 | Does the main session's own ready edit need the approval | (a) yes; (b) only a subagent's is refused | (a): a fork carries no `agent_id` |

## Implementation Steps

1. **Phase: design pages owed** (about 60 calls) -- `gatedDocument` and `bashDocumentRead` admit design pages; `unreadDoors` owes `// Design:` pages, from disk and proposed text; refusal wording; point bindings
   - Tests: `TestUnreadDoorsOwesDesignPages`, `TestUnreadDoorsRefusesDanglingDesign`, `TestGatedDocumentAdmitsDesignPages`, fixtures `code-edit-owes-design-page`, `code-edit-after-design-page-read`, `spec-edit-owes-design-page`
   - Files: `internal/le/hookruntime/writeedit.go`, `internal/le/hookruntime/readgate.go`, `internal/le/hookruntime/bash.go`, `ai/rules/documentation.md`, `ai/rules/points/`
   - Verify: `./le ai hooks unit`; count of dangling `// Design:` headers from `./le doc index check` is zero before landing (R-3)
2. **Phase: page format and its check** (about 60 calls) -- "Page format" in `docs/architecture/README.md`, the design-decisions check, the commit refusal, the first decision row in `docs/architecture/api/commands.md`
   - Tests: `TestDesignDecisionsSectionJudged`, `TestCommitRefusesEditedDesignPageWithoutDecisions`
   - Files: `internal/le/doc/consistency/consistency.go`, `internal/le/commit/designdecisions.go`, `internal/le/commit/prepare.go`, `docs/architecture/README.md`, `docs/architecture/api/commands.md`, `docs/contributing/documentation-testing.md`
   - Verify: `./le doc consistency` lists the pages owing the section
3. **Phase: claim gate and approval** (about 60 calls) -- approve package, `Claim` check, incident testdata
   - Tests: `TestClaimRefusesReadySpecWithoutApproval`, `TestClaimAdmitsMatchingApproval`, `TestClaimRefusesStaleApproval`, `TestApproveRefusesWithoutTerminal`, `TestApproveWritesDigestBoundRecord`
   - Files: `internal/le/spec/approve/`, `internal/le/spec/session.go`
   - Verify: `./le cli grammar`
4. **Phase: ready edit, away, owner-only Bash** (about 60 calls)
   - Tests: fixtures for AC-16, AC-17, AC-23; `TestAwayRequiresTerminalAndUntil`, `TestClaimAwayAdmitsFixSpecOnly`
   - Files: `internal/le/hookruntime/specready.go`, `internal/le/hookruntime/owneronly.go`, `internal/le/hookruntime/runtime.go`, `internal/le/spec/away/`
5. **Phase: Design documents** (about 60 calls) -- derivation and judgment, `./le spec citation design`, `writeSpecDesignDocuments`, approve pre-checks, template sections
   - Tests: `TestDesignDocumentsDerivation`, `TestDesignDocumentsJudgement`, `TestIncidentPipeChainSpecIsRefused`
   - Files: `internal/le/spec/citation/design.go`, `internal/le/spec/citation/anchors.go`, `internal/le/hookruntime/specdesigndocs.go`, `plan/templates/TEMPLATE.md`
   - Verify: A-3 timing on the incident fixture
6. **Phase: trailer, closure, skills, pages** (about 60 calls)
   - Tests: `TestDesignApprovedTrailerWrittenFromRecord`, `TestClaimAdmitsApprovalTrailer`, `TestClosureRefusesUncorrectedDeparture`, `TestClosureRefusesUnapprovedDeparture`
   - Files: `internal/le/commit/designapproval.go`, `internal/le/commit/actions.go`, the skills, `docs/contributing/spec-workflow.md`, `docs/contributing/committing.md`, `ai/rules/planning.md`
   - Verify: `./le doc check all` on the pages

The review lens for `/ze-review`, `/ze-review-spec` and `/ze-close`: for each
Design documents row, read the page at the quoted sentence and the diff. A
diff that breaks what a Design decisions row forbids, under an `agrees`
verdict, is a BLOCKER; a `departs` row whose page and decision row the diff
does not correct is a BLOCKER; a design page the derivation now yields for
files the implementation added is an ISSUE that sends the spec back to Thomas.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | The widened read gate owes exactly the header's pages; the digest blanks exactly Status, Updated and Phase |
| Fail-closed | An unreadable header, page, record, away file or doc index refuses, never admits |
| Data flow | One owed-set function serves code and spec edits; no le package imports `hookruntime` |
| Operator errors | Every refusal states what, why and the exact command |
| Rule: hooks judge form | Paths, headers, status cells, verdict tokens, verbatim spans and payload identity only |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Design page owed before a code edit | fixture `code-edit-owes-design-page` |
| Page format defined and checked | `TestDesignDecisionsSectionJudged`; `grep -n "Page format" docs/architecture/README.md` |
| Claim gate | `TestClaimRefusesReadySpecWithoutApproval` |
| Incident refused | `TestIncidentPipeChainSpecIsRefused` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Authorization fail-open | Each reader refuses on an unreadable or malformed record |
| Forgery | Trailer refused in a body; record only behind the terminal gate; harness Bash refused for the owner commands |
| Path handling | `<stem>` validated as a file name; a `// Design:` target resolved inside the checkout |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The incident spec's `ready` never reached a commit (12531c56ca `skeleton`, 2616478e64 `in-progress`), so an approval check reading only history would have missed it.
- The incident spec listed `docs/architecture/api/commands.md` in its own Files to Modify and changed it to fit its departure: a page the spec edits can never be `silent`.
- `internal/component/command/pipe.go` links `docs/architecture/api/commands.md`; had the 2026-08-21 decision been in that page, and the page owed before the edit, the implementing agent would have met it.
- `ai/rules/cli.md` disagrees with the 2026-08-21 decision for selector words (`plan/records/audit-command-pipe-vs-subcommand.md` line 555); only the judge and Thomas can see that.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Decisions live in the page the file links | A register, the doors, the records | Thomas: no new register; the link is what gets read |
| Widen `unreadDoors` | A parallel gate | One owed set for code and spec edits |
| Approval record keyed by spec, bound to a digest | Per-session file; metadata row | Approver and claimant are different sessions |
| Completeness and form are mechanical, meaning is judged | A wording hook | META forbids a hook reading wording |

## Known Limitations

- The tooling cannot judge whether an `agrees` verdict is true, or whether a Design decisions row states the decision faithfully. The judge is a model; Thomas decides.
- A decision Thomas gives in conversation that nobody writes into a page reaches nothing; the same-turn rule is the only control.
- A file with no `// Design:` header owes no design page, and a decision written into a page no file links is never owed.
- The terminal gate proves a terminal, not Thomas; a command built at run time can hide `spec approve` from the Bash hook.
- A hook cannot tell the main session from Thomas's editor, and a fork or another harness sends no `agent_id`; C-5 covers both.
- Whether the presentation shown to Thomas was the spec's section is not checked; the digest binds the text he approved.

## RFC Documentation (Scope: protocol)

N-A: tooling.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added

### Goal Gates (MUST pass)
- [ ] AC-1..AC-25 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests pass in the main checkout: `./le test unit pkg path` for each changed `internal/le/` package and `./le ai hooks unit`. No agent runs any `./le verify` command
- [ ] Feature code integrated, not library-only
- [ ] Integration and Documentation checklists answered with evidence
- [ ] Architectural Verification table filled
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Hook fixtures for end-to-end behavior
- [ ] Interop tests N-A (tooling)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`
- [ ] **Commit A:** code + tests + docs + edited spec
- [ ] **Commit B:** `remove plan/release/validation/spec-design-needs-owner-approval.md` only, in the same `./le commit create` script
