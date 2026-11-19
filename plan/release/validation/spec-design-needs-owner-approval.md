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
| Updated | 2026-11-19 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

On 2026-11-12 the historical spec `spec-plugin-dispatch-pipe-chain-reaches-the-handler`
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
`internal/component/command/pipe.go`, did not carry it. The repair of the
product is `plan/release/fixes/spec-pipe-chain-honored-on-every-command-surface.md`.
This spec is the gate.

Every gate on the way passed, and each passed honestly, because none of them
asks the question that failed:

| Gate | What it proved | What it never asks |
|------|----------------|--------------------|
| Spec door gate (`writeSpecDoorsRead`) | The design context read `ai/doors/system.md` and the covering doors in full | Whether the decisions on the pages the files link (`// Design:`) were read |
| Code door gate (`writeDesignDoorsRead`) | The implementing context read the doors before each code edit | The same: doors only, never the file's own design page |
| Producer-read gate (`designEvidenceProblem` over `designSubjects`) | The context Read some range of each existing source file named in Current Behavior, Files to Modify and Files to Create | Whether the design documentation of those files was read. Markdown is never a subject |
| Design owner check (`hookValidateSpec` over `AuditAnchors`) | The spec text names the first `// Design:` document of each source file it lists | Whether the design agrees with that document. The incident spec named `commands.md`, in its own Files to Modify |
| Existing Mechanisms gate (`mechanismsProblem`, `hasDataRow`) | The section holds a data row | Anything about meaning: it is structural only |
| `./le spec claim` (`Claim`, `transitionReadySpec`) | The WIP cap had room | Whether anybody approved the design |
| `/ze-review`, `/ze-review-spec`, `/ze-close` | The diff matches the spec, and the spec's own claims | Whether the spec agrees with the design documentation |

`ai/rules/planning.md` already says `/ze-spec` and `/ze-design` stay in the
main thread. Nothing enforces it.

Thomas, on this incident: "have an agent investigate how you were allowed to
make this shitty plan without reading the code it must NEVER EVER happen
again." And: "make sure specs authored can not go against design documentation
without a discussion with me with the same rules (before after example etc.)".
On where his decisions live: "I am not keen on ruling. We already have plenty
of registers. you are not going to be able to find that information. it should
be part of the documentation, and the documentation format should be updated.
When we have a file which links to a design, the design should be read and
therefore that way the information should be..."

His rules, committed on 2026-11-12 in `ai/META-DECISIONS.md` (fa219f6120,
f8eac4f0d7), "How agents work":

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
- "A design decision Thomas makes is written into the design page that
  documents its subject, never into a separate register of decisions. Before
  an agent edits a file, or writes a spec that touches it, it reads the design
  page the file's `// Design:` header links."
- "A hook checks what a command, path or file is, never how a message is
  worded."

And, "Test changes": "Owner-only authorization actions require an interactive
terminal and refuse noninteractive invocation before recording
authorization."

The goal:

1. The tooling refuses to start the implementation of a design Thomas has not approved, and refuses a subagent's move of a spec to `ready`.
2. Thomas's decisions are written into the design page that documents their subject, in a `## Design decisions` section the documentation format defines and the existing doc check validates. No separate register.
3. The existing link from a file to its design, the `// Design:` header, makes the decisions on that page owed reading before any code edit or spec edit that touches the file, recorded per agent and owed again when a decision is added.
4. A decision row, once written, changes or leaves its page only with Thomas's approval.
5. A spec lists every design document its files link, and every decision those pages carry, with an `agrees`, `departs` or `silent` verdict. An independent judge confirms the verdicts before the design is presented. A departure reaches implementation only with Thomas's approval of a presentation with before and after examples.

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
  → Decision: its points gain the directive that an owner decision is written into the Design decisions section of the page that documents its subject
- [ ] `docs/architecture/README.md` - the index of architecture pages
  → Decision: it becomes the place the page format is defined (no page defines it today). `docs/architecture/decisions/` holds one decision record and is left as it is
- [ ] `docs/contributing/documentation-testing.md`, "What gets checked" and "Adding a new documentation check"
  → Constraint: a related check joins an existing area; `./le doc consistency` already judges design references
- [ ] `ai/rules/planning.md` - phases, spec lifecycle, review independence
  → Constraint: five skills stay in the main thread, `/ze-spec` and `/ze-design` among them
- [ ] `ai/rules/spec-no-code.md` - this spec carries no code
- [ ] `docs/contributing/spec-workflow.md`, "Spec status vocabulary"
  → Constraint: "Spec approved: design to ready" is today an edit anybody can make
- [ ] `docs/contributing/committing.md` - trailers and the routes that write them
  → Constraint: a body line that starts with a declared trailer key is refused (`declaredTrailers`)
- [ ] `docs/contributing/running-commands.md`, "Issuing owner authorization also requires a terminal"
  → Constraint: no environment variable, keyword or flag skips the terminal gate
- [ ] `ai/owner/machines.md` - the two development machines, worktrees, git as the only carrier
  → Constraint: an untracked record never reaches the other machine; only a commit does

**Key insights:**
- The only route to `in-progress` is already `Claim` (`CheckWIPTransition` refuses an edit to it), so a check in `Claim` stops every unapproved start.
- The code-edit gate and the spec-edit gate share one function, `unreadDoors`; extending its owed set extends both gates at once.
- A whole read of a `// Design:` page cannot be the owed read. Measured on 2026-11-19: 400 distinct pages are named, `docs/architecture/core-design.md` (290 KB, named by 1091 files), `docs/architecture/api/process-protocol.md` (366 KB, 585 files) and `docs/architecture/api/commands.md` (295 KB, 321 files) lead the list, 28 named targets pass 100 KB, the Read tool stops at 25,000 tokens without an override, and a Bash print is credited only up to 25,000 bytes (`plainReadPrintMax`).
- The source-read record (`recordSourceRead`) is already range-bound and digest-bound, so a read record for one section of a page extends an existing shape.
- A commit-time check registers through `changecheck.Register`; the commit code names no check.
- The harness sends `agent_id` in the hook payload of a subagent's call (`Payload.AgentID`). Nothing in the tree states what a fork sends.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/spec/session.go` - `Claim` reads Status; for `ready` it checks the WIP cap, and on room writes the marker, then `transitionReadySpec` rewrites Status and Updated, all under the claim lock. No approval is read. A refusal returns before the marker is written, so the session keeps its previous claim. `CheckWIPTransition` refuses an edit whose result is `in-progress` unless the spec already was
- [ ] `internal/le/hookruntime/specwip.go` - `writeSpecWIP`, the pre-write check that calls `CheckWIPTransition` on the replayed edit
- [ ] `internal/le/hookruntime/writeedit.go` - `writeDesignDoorsRead` owes, before a code edit (`.go`, `.yang`, `.templ`), a whole read of `ai/doors/system.md` and each door covering the file; `unreadDoors` computes the owed set and checks `documentReadMarker` records; `doorRefusal` renders the refusal; `writeSpecStatus` refuses a Go edit while the claimed spec is neither `in-progress` nor `verification`, and passes a session that claims nothing
- [ ] `internal/le/hookruntime/specdoors.go` - `writeSpecDoorsRead`: the same `unreadDoors` over every repository path the spec names, before and after the edit (`specRepoPaths`)
- [ ] `internal/le/hookruntime/readgate.go` - `gatedDocument` admits only the style guide and pages under `ai/doors/` to read records; `documentReadMarker` names the per-agent record, an empty file whose existence is the record; `gatedCheckout` binds a read to the checkout the copy lies in; `loadDoors`, `coveringDoors`
- [ ] `internal/le/hookruntime/lifecycle.go` - `hookSourceRead` writes the document record when `readWholeFile` holds (no offset past 1, and a limit, 2000 lines by default, that covers the file), else calls `recordSourceRead`; `hookValidateSpec` refuses, after the write, a spec past `skeleton` that does not name the `// Design:` document of a file it lists (`AuditAnchors`)
- [ ] `internal/le/hookruntime/bash.go` - `bashDocumentRead` records a plain `cat` or `nl` of a gated document, only when the command names the style guide or `ai/doors/`, and only when the line prints at most 25,000 bytes (`plainReadPrintMax`); `bashGovernedWrite` refuses a shell write into `plan/` and `ai/rules/`
- [ ] `internal/le/hookruntime/designevidence.go` - `designSubjects`, `designEvidenceProblem`: a Read of any range of each source file a spec names, still matching the file, is owed for a spec edit. `recordSourceRead` stores the path, the returned range and the digest of the returned text. Markdown is not a subject
- [ ] `internal/le/hookruntime/specmechanisms.go` - `proposedSpec` replays a Write, Edit or MultiEdit into the text the edit leaves
- [ ] `internal/le/hookruntime/runtime.go` - `nativeHookActions`, the one list of checks, with the call budget last; `Payload.AgentID`; each check needs a `// ze point:` binding
- [ ] `internal/le/doc/consistency/consistency.go` - `checkDesignRefs` warns on a source file with no `// Design:` text. Nothing judges the page a header names, or whether it exists
- [ ] `internal/le/doc/consistency/register.go` - the area's one action, `check`
- [ ] `internal/le/spec/mechanisms/specmechanisms.go` - `Problem`, the structural spec judge at edit and commit; `sectionLines`, `tableRows`, `cellsOf`
- [ ] `internal/le/commit/specmechanisms.go` - `specMechanismsRefusal`, which judges a spec only when the commit changes its text (the precedent for owing a section on next edit)
- [ ] `internal/le/commit/changecheck/changecheck.go`, `internal/le/commit/changechecks.go` - `Register`, `Change` and `runChangeChecks`: a check registers from its own package and judges what the commit adds, reading both sides
- [ ] `internal/le/commit/prepare.go` - `Create`, `checkVerificationGates`, which finds a closure stem (`closureStemSelected`) and its change set (`specWorkPopulation`); the `Spec-work:` trailer is appended after the person-act trailers
- [ ] `internal/le/commit/actions.go` - `declaredTrailers`, each with the one route that writes it
- [ ] `internal/le/commit/authorization.go` - `requireAttestationTerminal`: the `attest` keyword of `commit create` needs the owner terminal
- [ ] `internal/le/spec/citation/anchors.go` - `AuditAnchors`: for the source bullets of Files to Modify and Files to Create, the `// Design:` document (`declaredDesignDocument`, first match in the first 25 lines only) and every document the doc index anchors to the file (`docindex.DocumentsByPath`)
- [ ] `internal/le/le/human/human.go` - `RequireAuthorization`: refuses unless stdin and stdout are terminals, and states that a terminal is an operational restriction, not authenticated authorship
- [ ] `internal/le/spec/review_authorization.go` - an owner keyword guarded by `RequireAuthorization`
- [ ] `internal/le/test/weakened/approve.go` - the judge approval route: an untracked file under `tmp/`, keyed by commit session, written by an agent with no terminal gate, read by hook and commit gate, turned into a trailer
- [ ] `internal/le/ai/hooks/fixtures.go` - the fixture categories `./le ai hooks unit` runs, and the sealed digest of the hook sources, re-sealed with a dated note on each hook change
- [ ] `internal/le/register.go` - the composition root: one blank import per tool
- [ ] `internal/component/plugin/server/command.go`, `internal/component/command/pipe.go` - the incident spec's main files; their `// Design:` headers name `docs/architecture/api/process-protocol.md` and `docs/architecture/api/commands.md`

**Behavior to preserve:**
- `Claim` on a `skeleton`, `design`, `in-progress`, `verification` or `blocked` spec writes only the marker
- The WIP cap check, the claim lock and the refusal of `./le spec claim` when the cap is full
- Every obligation the door gates, the producer-read gate and the design owner check hold today
- A page or spec that predates a new section owes it only when an edit next reaches it, never in another session's commit that does not touch it
- An edit of a spec that is already `ready` and stays `ready`
- `Weakening-approved:` and `RFC-approved:` routes
- `bashDocumentRead` and `gatedDocument`: the whole-read record stays for the style guide and the doors only

**Behavior to change:**
- `Claim` refuses `ready` to `in-progress` without an owner approval of the spec's design as it stands, or an away admission
- An edit that makes a spec `ready` is refused from a subagent, and from the main session without the approval (C-5)
- A code edit and a spec edit owe a read of the `## Design decisions` section of every `// Design:` page of the files they touch, beside the doors
- A design page carries its owner decisions in a `## Design decisions` section; the doc consistency check validates it, and a commit that changes or removes a decision row needs Thomas
- A spec carries `## Design documents` (every linked design page and every decision on them, with a verdict) and `## Design presentation` before Thomas can approve it
- An independent judge records a verdict before the presentation (C-3)
- A `departs` verdict needs Thomas's approval, and closure needs the departed page corrected in the spec's change set

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| The file-to-design link | every source file's `// Design:` header; `<!-- source: -->` anchors rendered into `ai/CODE-TO-DOCS.md` (`docindex.DocumentsByPath`) | Names the page that documents a file; `checkDesignRefs` warns when absent | Extends: the decisions on the `// Design:` page become owed reading. No new index |
| Door read gate, code and spec | `internal/le/hookruntime/writeedit.go` `writeDesignDoorsRead`, `unreadDoors`; `specdoors.go` `writeSpecDoorsRead` | Owes whole reads of the covering doors before a code or spec edit | Extends: `unreadDoors` adds the Design decisions section of each touched file's `// Design:` pages to the owed set, so both gates owe them with one change and one refusal |
| Whole-document read record | `internal/le/hookruntime/readgate.go` `gatedDocument`, `documentReadMarker`; `bash.go` `bashDocumentRead` | Records a whole Read or `cat` of the style guide or a door | Does not apply to design pages: the pages are too large to read whole (Key insights). Kept as it is for the doors |
| Range-bound, digest-bound read record | `internal/le/hookruntime/designevidence.go` `recordSourceRead`, `observedSourceRange`, `currentSourceReads` | Stores the range a Read returned and the digest of its text; a changed file stales it | Extends the shape: a section read record holds the digest of one page section, written when a Read returned the whole section, stale when the section changes |
| Design owner check | `internal/le/hookruntime/lifecycle.go` `hookValidateSpec`; `internal/le/spec/citation/anchors.go` `AuditAnchors` | Refuses a spec that does not name the `// Design:` document of a file it lists | Extends: the same derivation, over every `// Design:` line, yields the rows `## Design documents` owes, and naming a document becomes giving it a verdict |
| Doc consistency gate | `internal/le/doc/consistency/consistency.go` `Check`, `checkDesignRefs`; `./le doc consistency` | Judges design references in source files | Extends: a `design-decisions` check over each page a `// Design:` header names, and a `design-refs` finding for a header whose page is missing. Joins the existing area (`documentation-testing.md`, "Adding a new documentation check") |
| Change-set check seam | `internal/le/commit/changecheck/changecheck.go` `Register`; `internal/le/commit/changechecks.go` | Runs every registered check over what a commit adds, both sides readable | Extends: the consistency package registers the decision-row check, so `prepare.go` learns no page rule |
| Owe-on-next-edit commit judge | `internal/le/commit/specmechanisms.go` `specMechanismsRefusal` | Judges a spec's sections only in a commit that changes its text | Extends the pattern: a decision row is judged only in a commit that adds, changes or removes it |
| Owner terminal gate | `internal/le/le/human/human.go` `RequireAuthorization` | Refuses an owner action without interactive stdin and stdout | Extends: `./le spec design approve`, `away`, `return`, and the `decision-change` keyword of `./le commit create` |
| Owner keyword on `commit create` | `internal/le/commit/authorization.go` `requireAttestationTerminal` | `attest` is admitted only from the owner terminal and writes `Owner-attested:` | Extends the pattern: `decision-change` beside it |
| Judge approval file and trailer | `internal/le/test/weakened/approve.go`; `internal/le/commit/actions.go` `declaredTrailers` | Untracked file read by hook and commit gate; trailer per used row; hand-written trailer refused | Extends the pattern: records keyed by spec under `tmp/spec-design/`, a `Design-approved:` trailer |
| Claim and the ready transition | `internal/le/spec/session.go` `Claim`, `CheckWIPTransition` | The only route to `in-progress` | Extends: checks the approval or the away admission before the cap |
| WIP edit check | `internal/le/hookruntime/specwip.go` `writeSpecWIP` | Refuses an edit that makes a spec `in-progress` | Extends the pattern: a sibling check for the edit that makes a spec `ready` |
| Closure change set | `internal/le/commit/prepare.go` `checkVerificationGates`, `specWorkPopulation` | Finds a closure and every path its `Spec-work:` commits touched | Extends: refuses a `departs` row whose page the change set does not touch |
| Verbatim quote check | `docs/contributing/rfc-conformance-gates.md`, "The row quote" | Refuses an RFC row whose quote is not a span of its section | Extends the idea: an `agrees` or `departs` row quotes a verbatim span of the section it names |
| Independent judge skill | `ai/skills/ze-test-judge.md` (its stub under `.claude/skills/` is generated by `./le ai sync update`) | An agent that did not make a change judges it and records the verdict itself | Extends the pattern: `ai/skills/ze-design-judge.md` (C-3) |
| Spec status listing | `./le spec status`, `internal/le/spec/status` | Lists every spec with bucket and status | Extends: `./le spec design pending` reads the same inventory and adds what each `ready` spec lacks (C-2) |
| Reader-only command judgment | `internal/le/hookruntime/session.go` `bashAgentIdentity`, which asks `commandOutsideReaders` over the one parse of the line | Tells a Bash line made only of readers from one that can write | Extends: the owner-record guard asks the same judgment, and admits a reader-only line that names `tmp/spec-design/` |
| The subagent predicate | `internal/le/hookruntime/session.go` `fromSubagent` | Answers from the payload's `agent_id` alone whether a call is a subagent's; the identity check, the budget and the heavy-work check all read it | Extends: `writeSpecReady` reads the same predicate, so no check judges one caller two ways |
| Doors | `ai/doors/*.md` | Subsystem contracts, boundaries, traps | Does not apply as a home for decisions: a decision lives in the page that documents its subject, which the file links |
| Decision records | `docs/architecture/decisions/`, `plan/records/` | One architecture decision record; audit and investigation history | Does not apply as a home: Thomas rejected a register. A record is the evidence a decision row cites as its Source |
| `ai/META-DECISIONS.md` | owner rules on how agents work and on scope | Loaded by every main session | Does not apply to product design decisions; a row may cite it as Source |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An agent's Write or Edit of a code file or a spec (pre-write hook payload: tool, path, proposed text, `agent_id`)
- An agent's Read of a page (the Read hook payload: path, returned content, start line, line count)
- `./le spec claim <spec-file>`
- `./le spec design documents`, `check`, `judge`, `pending` from an agent; `approve`, `away`, `return` from Thomas's terminal
- `./le commit create` carrying a design page or a spec, or closing a spec
- `./le doc consistency`

### Transformation Path
1. Thomas states a decision; the main session writes it as a row of the `## Design decisions` section of the page that documents the subject, in the same turn, with the record as its Source
2. Before an agent edits a code file or a spec, `unreadDoors` owes the doors and the Design decisions section of each `// Design:` page; the agent reads the section, the Read hook records the section's digest, and the decision is in the agent's context
3. A spec author runs `./le spec design documents spec <path>`, which prints every linked design page and every decision row on those pages; the author gives each a verdict in `## Design documents` and writes `## Design presentation`
4. `./le spec design check spec <path>` judges the table against the same derivation, and the presentation's subsections
5. An independent judge agent reads the section each row names and records a verdict with `./le spec design judge` (C-3); the action refuses a judge whose read records do not cover every row
6. The main session presents the design; Thomas approves from his terminal (`./le spec design approve`), which re-runs the check, requires the judge's approval and writes a digest-bound record
7. The main session sets `ready` (the edit hook admits it on the record); `./le spec claim` admits it; `./le commit create` writes `Design-approved:` from the record, so the approval reaches the other machine
8. During implementation, a commit that changes or removes a decision row is refused unless an approved departure names it
9. At closure, an approved `departs` row owes an edit of its page in the change set; the page's Design decisions section then states the new decision

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Owner terminal to agent sessions | Untracked records under `tmp/spec-design/` | No |
| One checkout to another (Mac, Linux, a worktree) | `Design-approved:` trailer reachable from HEAD | No |
| Hook process to le packages | Library calls into the consistency and spec design packages; no le package imports `hookruntime` | No |
| Agent context to hook | The Read tool's returned content and range, as `recordSourceRead` already reads them | No |

### Changed-Wire Census

**Search evidence:** scope `internal/le/`, `docs/`, `ai/`, `plan/templates/`; terms `## Design decisions`, `// Design:`, `.source-read-`, `.doc-read-`, `Design-approved`, `Decision-changed`, `spec-design`, `## Design documents`, `## Design presentation`, `tmp/commit-`. Positive control: the door read record has its writers (`hookSourceRead`, `bashDocumentRead`) and its reader (`unreadDoors`), and the `// Design:` header has two readers today (`checkDesignRefs`, `declaredDesignDocument`). Finding: `## Design decisions` occurs in no design page (the only hits are an unrelated bold label in `docs/contributing/spec-workflow.md`); `Design-approved`, `Decision-changed`, `tmp/spec-design/` and the two spec headings occur nowhere.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| Design header | writer | the author of each source file, first lines of the file | 13,062 Go files carry one; 35 carry more than one | Preserve: format unchanged |
| Design header | reader | `internal/le/doc/consistency/consistency.go` `checkDesignRefs` | Judges presence only | Change: also reports a `.md` target that does not exist |
| Design header | reader | `internal/le/spec/citation/anchors.go` `declaredDesignDocument` | Returns the first match only | Change: returns every `// Design:` line of the header |
| Design header | reader | `internal/le/hookruntime/writeedit.go` `unreadDoors` | Does not read it today | New reader: from disk and from the proposed text |
| Design decisions section | writer | the main session, in the page that documents the subject | New heading | New; format defined in `docs/architecture/README.md` |
| Design decisions section | reader | `internal/le/doc/consistency/decisions.go` (new): the parser, the `design-decisions` check and the registered change-set check | New | New |
| Design decisions section | reader | `internal/le/hookruntime/sectionread.go` (new) and `unreadDoors` | New | New |
| Design decisions section | reader | `internal/le/spec/design/documents.go` (new), the derivation | New | New |
| Section read record | writer | `internal/le/hookruntime/lifecycle.go` `hookSourceRead`, through `sectionread.go` | A markdown Read records nothing today (`sourceKind`) | New: one append-only file per page and agent under the session root's `tmp/session/` |
| Section read record | reader | `internal/le/hookruntime/writeedit.go` `unreadDoors` | New | New |
| Section read record | reader | `internal/le/spec/design/judge.go` (new) | New | New |
| Approval record | writer | `internal/le/spec/design/approve.go` (new), file `tmp/spec-design/approved/<stem>.md` | No such file today | New |
| Approval record | reader | `internal/le/spec/session.go` `Claim` | Reads only Status and the cap today | Change |
| Approval record | reader | `internal/le/hookruntime/specready.go` (new) | No ready-edit check exists | New |
| Approval record | reader | `internal/le/commit/designapproval.go` (new): trailer writer and closure checks | `declaredTrailers` route list | New |
| Approval record | reader | `internal/le/doc/consistency/decisions.go` (new), through a lookup the spec design package registers | New | New |
| Judge record | writer | `internal/le/spec/design/judge.go` (new), file `tmp/spec-design/judged/<stem>.md` | New | New |
| Judge record | reader | `internal/le/spec/design/approve.go`, `check.go`, `pending.go` (new); `Claim` for an away admission | New | New |
| Away record | writer | `internal/le/spec/design/away.go` (new), file `tmp/spec-design/away.md` | No such file today | New |
| Away record | reader | `internal/le/spec/session.go` `Claim`; `internal/le/hookruntime/specready.go`; `pending.go` | New | New |
| Design-approved trailer | writer | `internal/le/commit/designapproval.go` from the approval record or the away admission | `declaredTrailers` | New key, `OwesNothing` true |
| Design-approved trailer | reader | `internal/le/spec/design/approval.go` (new), one lookup over commits reachable from HEAD, used by `Claim`, the ready hook, the closure checks and the decision-row check | New | New |
| Decision-changed trailer | writer | `internal/le/commit/designapproval.go` from the `decision-change` keyword | New | New key, `OwesNothing` true |
| Decision-changed trailer | reader | `internal/le/commit/actions.go` `declaredTrailers`, through the hand-written trailer refusal; a person reading the history | New | New |
| Design documents table | writer | the spec author, from `./le spec design documents` | New heading | New, owed by `./le spec design approve` |
| Design documents table | reader | `internal/le/spec/design/check.go`, the approve and judge actions, `internal/le/commit/designapproval.go`, the judge and review skills | New | New |
| Design presentation section | writer | the spec author, `/ze-spec` | New heading | New, owed by `./le spec design approve` |
| Design presentation section | reader | `internal/le/spec/design/check.go` and the approve action | New | New |

### Integration Points
- `unreadDoors` - one more source of owed reading: the Design decisions sections of the files' `// Design:` pages, and of any page a spec names
- `hookSourceRead` - a Read of a markdown page records the sections it returned whole
- `docconsistency.Check` - one more check; its parser and judge are exported, and its `register.go` registers the change-set check
- `Claim` - one approval check before the WIP check
- `nativeHookActions` - `writeSpecReady`, `writeOwnerRecord`, `bashOwnerRecord`, each before the call budget
- `declaredTrailers` - two new keys
- `AuditAnchors` - `declaredDesignDocument` returns every header line; the derivation in the spec design package calls it
- `internal/le/register.go` - one blank import for `internal/le/spec/design`

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | Reads flow through the existing gate function; approval through the owner-terminal gate and the claim route |
| No unintended coupling | Yes | The spec design package imports the citation, path and status packages and is imported by `spec`, `commit` and `hookruntime`; it imports neither `spec` nor `hookruntime` |
| No duplicated functionality | Yes | No second gate, no second index, no register: the `// Design:` link and the doc index are the only sources |
| Zero-copy preserved where applicable | N-A | Tooling |
| Registration over hardcoding, outbound | Yes | The actions register from `init()`; the hook checks join the one registry; the commit check registers through `changecheck` |
| Registration over hardcoding, inbound | Yes | No list of pages or decisions exists in code: the pages come from each file's own header, the decisions from each page's own section, the fix-spec bucket from `specpath.Categories` |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Claude Code puts `agent_id` in every subagent hook payload and the agent cannot alter it | `fromSubagent` reads `Payload.AgentID` alone; the comment of `documentReadMarker`; the identity check and the call budget rely on it | Subagent detection fails open for the ready edit only; C-5 still requires the approval | A hook test over a subagent payload plus a live probe | unvalidated |
| A-2 | A fork's payload carries no `agent_id` | No source in the tree states it (an earlier draft cited a comment that does not exist) | A fork is judged as the main session, which C-5 makes harmless: its ready edit needs the approval too | A live probe at implementation | unvalidated |
| A-3 | The owed-set computation is fast enough for a pre-write hook | It reads the first 25 lines of each named code file and scans each named page once; `hookValidateSpec` already derives the whole doc index on every spec write | Every spec edit slows | Time it on this spec and on the incident fixture | unvalidated |
| A-4 | Thomas runs le from a real terminal on both machines | Five owner actions already use the gate | He cannot approve | His first approval | unvalidated |
| A-5 | The Read hook receives the returned content, start line and line count for a markdown page as it does for source | `recordSourceRead` parses them for source files | No section record can be written | `TestSectionReadRecordedForMarkdown` over a real payload | unvalidated |
| A-6 | `./le scratch clean` and the session reaper leave `tmp/spec-design/` alone | The reaper removes `tmp/session` entries; not read for this directory | An approval disappears and a claim is refused until the trailer exists | Read both producers in phase 5 and add the directory to what they keep | unvalidated |
| A-7 | A judge agent runs `./le` under its own `ZE_AGENT_ID` | The identity line every subagent types (`ai/rules/planning.md`) | The judge action cannot find the judge's read records | `TestJudgeRefusesWithoutAgentIdentity` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | 94 specs are `ready` today (25 under `release/fixes/`, counted from the Status rows on 2026-11-19) with no approval; the claim gate stops every new start at landing | `./le spec claim` refusals | C-2: each needs Thomas; `./le spec design pending` prints them by priority with what each lacks. Work already `in-progress` is untouched. The approve action lands before the gate (Implementation Steps) |
| R-2 | The wider read gate refuses code edits in every session the moment it lands | A wave of refusals naming design pages | Inert at landing: no page carries the section, so nothing more is owed. Each decision row then costs every agent that edits a linking file one read of a few lines |
| R-3 | A `// Design:` header names a page that does not exist | A refusal on an edit of that file | Zero such headers on 2026-11-19 (400 distinct `.md` targets checked). The refusal names the header line; `./le doc consistency` reports the class tree-wide |
| R-4 | A page is renamed and its headers are left behind | `design-refs` errors, then refusals on those files | The consistency finding names every stale header; the rename's own work owes the header sweep (`ai/rules/principles.md`, the consumer of the name you renamed) |
| R-5 | An agent fakes a terminal, or writes a record by hand | A record written inside a harness session | `writeOwnerRecord` and `bashOwnerRecord` refuse a write under `tmp/spec-design/`; a faked terminal is the wrapper `ai/META-DECISIONS.md` already bans. Residue in Known Limitations |
| R-6 | The digest stales on harmless edits | Claim refusals right after approval | The digest covers four sections only, and leaves out the Choices subsection, so bookkeeping and the recording of his answers do not stale it (C-8) |
| R-7 | Authors fill `silent` rows by rote | Tables of 15 or more silent rows | A `silent` row needs a reason; the judge reads the section behind every row; a decision row and a page the spec edits cannot be `silent` |
| R-8 | A decision row is added that Thomas never made | A row whose Source does not hold the decision | The Source must resolve; the judge and the review lens read it; Known Limitations |
| R-9 | The hook source seal in `fixtures.go` goes red in another session's run between phases | `./le ai hooks unit` red on `checkHookSources` | Each phase that edits a hook source re-seals in its own commit |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Every session's code edits, spec edits, claims and commits of design pages: nothing user-visible in `ze` |
| How is it reverted? | Single commits per phase; the records are untracked and inert without the readers |
| Who else touches this path? | Every session through the hooks; the commit flow; whoever edits design pages |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Pre-write hook, code edit of a file whose page's decisions are unread | → | `unreadDoors` via `writeDesignDoorsRead` | `TestDesignPageGateOwesTheDecisionsSection` |
| Pre-write hook, spec naming such a file | → | `unreadDoors` via `writeSpecDoorsRead` | `TestSpecDoorGateOwesTheDecisionsSection` |
| Read hook, a Read that returns the whole section | → | the section record writer via `hookSourceRead` | `TestSectionReadRecordedForMarkdown` |
| `./le doc consistency` over a page with a malformed section | → | the `design-decisions` check | `TestDesignDecisionsSectionJudged` |
| `./le commit create` removing a decision row | → | the registered change-set check | `TestCommitRefusesChangedDecisionRow` |
| `./le spec design documents spec <path>` | → | the derivation | `TestDesignDocumentsDerivation` |
| `./le spec design judge` from an agent | → | the judge action | `TestJudgeRefusesUncoveredRow` |
| `./le spec design approve` without a terminal | → | the approve action | `TestApproveRefusesWithoutTerminal` |
| `./le spec claim` on a ready spec | → | `Claim` approval check | `TestClaimRefusesReadySpecWithoutApproval` |
| Pre-write hook, subagent payload, Status to ready | → | `writeSpecReady` | `TestSpecReadyRefusedFromSubagent` |
| Pre-write hook and pre-Bash hook, a write under `tmp/spec-design/` | → | `writeOwnerRecord`, `bashOwnerRecord` | `TestOwnerRecordWriteRefused` |
| `./le commit create` closing a spec with an uncorrected `departs` page | → | closure check | `TestClosureRefusesUncorrectedDeparture` |
| The incident, replayed | → | all of the above | `TestIncidentPipeChainSpecIsRefused` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A context that read the doors edits `internal/component/command/pipe.go` (header page `docs/architecture/api/commands.md`); the page carries a `## Design decisions` section this context holds no current record of | Exits 2 and prints the refusal of example 1 in Design presentation: the file, each page with its section's line range and the Read offset and limit that clear it, the why and the fix | The same edit after the read; a file with no `// Design:` line and a page with no such section owe the doors alone, as today |
| AC-2 | A Write creating a code file whose proposed text carries a `// Design:` line, or an Edit of a file with more than one such line | Exits 2 listing the section of every page the header names, from the file on disk and from the proposed text | An Edit that removes a header line still owes the page the file had, as the spec gate owes the doors of paths an edit drops |
| AC-3 | A spec edit naming a code file whose page's decisions are unread, or naming such a page itself | Exits 2 from `writeSpecDoorsRead` with one list, the sections beside the doors | A spec naming no code file and no such page owes `ai/doors/system.md` alone |
| AC-4 | The context read the section, then a row is added to it | Exits 2 on the next edit of a linking file, owing the section again, because the recorded digest is the old section's | An edit elsewhere in the page leaves the record current, so the page's other churn costs no agent a re-read |
| AC-5 | A Read whose returned range stops inside the section, or a Read of a copy of the page outside its place in the checkout | Writes no record, and the next edit is refused as in AC-1 | A Read of the whole page, or of any range that holds the section whole, records it |
| AC-6 | Agent A read the section; agent B of the same session edits a linking file | Exits 2 for B, which holds its own records, as the door gate does | B's edit after B's own read |
| AC-7 | A code edit whose proposed text carries a `// Design:` line naming a `.md` page that does not exist | Exits 2 naming the file, the header line and `fix: correct the header to name the page that documents this file` | An edit whose proposed text corrects the header passes; a spec edit that names such a file owes nothing for it, since `./le doc consistency` reports the header |
| AC-8 | `./le doc consistency` over a tree where a `// Design:` page carries a `## Design decisions` section with a row whose Decision, It forbids or Source cell is empty, whose Date is not `YYYY-MM-DD`, or whose Source resolves to no tracked path, commit or `ai/META-DECISIONS.md` section; or carries the heading twice; or where a header names a missing page | Reports each as an error, `design-decisions` naming the page and the row, or `design-refs` naming the file and its header | A page with no section is reported as nothing: it records no owner decision |
| AC-9 | `./le commit create` carrying a page whose Design decisions section gains a row that fails AC-8 | Refuses, naming `./le doc consistency`, the page, the row and the cell | A commit that does not change the section's rows, whatever the section already holds |
| AC-10 | `./le commit create` carrying a page from whose Design decisions section a row present at the parent is changed or removed, with no approved departure naming that page and decision | Refuses with the text of example 4: the page, the row, why, and the two routes | Adding a row and reordering rows; a change an approved `departs` row of a spec names (AC-32) |
| AC-11 | The same commit with `decision-change "<what Thomas said>"`, run without a terminal, then from one | Exits 2 with `human.RequireAuthorization`'s refusal naming the command for Thomas; from his terminal the commit is prepared and carries `Decision-changed: <page>: <his words>` | A body line starting `Decision-changed:` is refused as hand-written |
| AC-12 | `docs/architecture/api/commands.md` after phase 2 | Holds one `## Design decisions` section whose first row states the 2026-08-21 pipe decision, sourced to the two records, and `./le doc consistency` reports no `design-decisions` finding for it | Its section "The surface splits the pipe chain; the dispatcher refuses one" is left to `spec-pipe-chain-honored-on-every-command-surface` |
| AC-13 | `./le spec design documents spec <path>` | Prints one row per design page the spec's files link (every `// Design:` page of its Files to Modify and Files to Create, the architecture pages the doc index anchors to them, the covering doors, every design page the spec itself lists for modification) and one row per decision row on those pages, each with what derived it | Guides and feature pages the index anchors are left out: they are not design documentation |
| AC-14 | `./le spec design check spec <path>` on a spec with no `## Design documents` section, a derived row missing, a verdict other than `agrees`, `departs`, `silent`, a section the page does not hold, a quote that is not a verbatim span of that section, a `silent` row with no reason, a page the spec modifies marked `silent`, a decision row marked `silent`, a missing or empty `### What changes`, `### Before and after`, `### Alternatives set aside`, `### Documents departed from` or `### Choices for Thomas`, a `departs` page that `### Documents departed from` does not name, or no current judge approval | Exits 1 and prints one line per problem with the fix, ending `run ./le spec design documents spec <path> to list the owed rows` | Exit 0 with `ready to present` when nothing is owed; `### Documents departed from` may state `none` when no row is `departs` |
| AC-15 | `./le spec design judge spec <path> verdict approve reason "<sentence>"` from an agent whose section read records are current for every row's page and section, on a spec whose table passes AC-14 | Writes `tmp/spec-design/judged/<stem>.md` with the design digest, the verdict, the sentence, the judge's agent fingerprint and the date, and prints the digest | A `refuse` verdict is recorded the same way, and a later `approve` by another judge replaces it |
| AC-16 | The same from the main thread (no agent identity), or from an agent with no current record for some row, or with an empty or multi-line reason | Exits 2 naming what is missing: the agent identity, or each row whose section this agent has not read with the Read range that clears it | A door row is covered by the whole-read record the door gate already keeps |
| AC-17 | `./le spec design approve` with stdin or stdout not a terminal | Exits 2 with `human.RequireAuthorization`'s refusal naming the command for Thomas, and `tmp/spec-design/` is left as it was | From his terminal the action goes on to AC-18 |
| AC-18 | `./le spec design approve` from a terminal on a spec that fails AC-14, or whose judge record is absent, records `refuse`, or carries another digest | Exits 2 naming each failing row, missing subsection, or the judge step to repeat | A spec in Status `design`, `ready`, `in-progress` or `blocked` is accepted, so a departure found during implementation can be approved |
| AC-19 | `./le spec design approve` succeeds | Writes `tmp/spec-design/approved/<stem>.md` (fields in "The records"), prints the digest and the next step | Re-approval replaces that spec's record only |
| AC-20 | Pre-write: an edit making a spec `ready` (from another status or as a new file), payload carries `agent_id` | Exits 2 and prints `BLOCKED: <path>: a subagent cannot move a spec to ready`, why, and `fix: hand the design back to the main session and stop` | A subagent's edits that leave Status as it is, and its edits of a spec already `ready` |
| AC-21 | The same edit from the main session with no approval matching the proposed text, Thomas not away | Exits 2 and prints `<path> cannot be marked ready: its design has no owner approval matching this text`, with the check and approve commands | The same edit once the record or a reachable trailer matches (C-5) |
| AC-22 | `./le spec claim spec-x.md`, Status `ready`, no matching approval record or trailer, Thomas not away | Exits 2 and prints the refusal of example 5; Status stays `ready` and the session keeps its previous claim | A claim of a `skeleton`, `design`, `in-progress`, `verification` or `blocked` spec writes the marker, because none starts implementation |
| AC-23 | Same, with an approval record whose digest matches | Claim proceeds as today, and the report gains `approved: <date> "<owner words>"` | Every claim within the cap that carries an approval |
| AC-24 | A record exists and the spec's Task, Acceptance Criteria, Design documents or Design presentation changed after it | Exits 2 and prints `its design changed after Thomas approved it on <date>` with the commands to check, judge and re-approve | A change outside those four sections, or inside `### Choices for Thomas`, leaves the approval current (C-8) |
| AC-25 | No local record, and a commit reachable from HEAD carries a `Design-approved:` line with this stem and the matching digest | Claim proceeds as AC-23 | A spec approved on the other machine, or in the main checkout before a worktree branch was cut |
| AC-26 | A record that cannot be read or parsed, or a `git log` that fails | Exits 2 naming the file or the git error and the command that repairs it | A missing record is not this case: it falls to the trailer, then to AC-22 |
| AC-27 | `./le spec design away until <YYYY-MM-DD> words "..."` and `./le spec design return` | From a terminal, the first writes `tmp/spec-design/away.md` and prints the admission rule, the second removes it; both refuse without a terminal, and `away` refuses a date before today or a missing `until` (C-4) | The record lapses on its own after `until` |
| AC-28 | Thomas away, a `ready` spec in the fixes category with no `departs` row, a clean check and a current judge approval, no owner approval | The ready edit is admitted from the main session, the claim proceeds and reports `admitted while Thomas is away (<from> to <until>): no design approval; he reviews it on return`, and the next commit carrying the spec writes `Design-approved: <stem> <digest>: away <from>..<until>` | Only fix specs: the away rule covers "a fix spec's design" |
| AC-29 | Thomas away, and a `departs` row, a spec outside the fixes category, a failing check, or no judge approval | Exits 2 as AC-22, adding `while Thomas is away, only a judged fix spec with no departure starts without his approval` | A spec he approved before leaving starts as AC-23 |
| AC-30 | `./le spec design pending` | Prints every `ready` spec with no matching approval, ordered by Priority, then by category in `specpath.Categories` order, then by Updated, each with what it lacks (sections, judge, approval), then every spec started under an away admission he has not approved since, and the away state | A `ready` spec with a matching approval is left out |
| AC-31 | `./le commit create` carrying a spec whose approval record matches the carried text, with no such trailer reachable yet | The message gains `Design-approved: <stem> <digest>: <date> <owner words>` once | A body line starting `Design-approved:` is refused by the hand-written trailer refusal; prose naming the key inside a line is accepted |
| AC-32 | Closing a spec with an approved `departs` row whose page the change set does not modify | Refuses and prints `closure of spec-x: it departs from <page>, and the change set does not correct it`, with the fix | A `departs` row whose page the change set modifies |
| AC-33 | Closing a spec with a `departs` row that no approval of this spec lists | Refuses and prints `spec-x departs from <page> without Thomas's approval`, with the approve command | A departure approved during implementation (AC-18) |
| AC-34 | A Write or Edit whose path lies under `tmp/spec-design/`, or a Bash line that names that directory and is not made only of readers | Exits 2 and prints `BLOCKED: tmp/spec-design/ holds Thomas's approvals, written only by ./le spec design`, why, and the command to hand him | `ls` and `cat` of the records, and every `./le spec design` action, which names no path under it |
| AC-35 | `plan/templates/TEMPLATE.md`, `ai/skills/ze-spec.md`, `ai/skills/ze-design-judge.md`, the review skills and the pages in Files to Modify, after phase 7 | Each states the two sections, the judge step, the approve command and what `ready` now means, and a spec written from the template and filled passes AC-14 | The closure template is unchanged: the lens is in the review skills |
| AC-36 | The incident: the spec text of 2616478e64 with Status `ready`; `docs/architecture/api/commands.md` carrying the row of AC-12 | The claim is refused (AC-22); a subagent's ready edit is refused (AC-20); an edit of `pipe.go` by a context that never read the row is refused (AC-1); `./le spec design documents` lists `docs/architecture/api/commands.md`, `docs/architecture/api/process-protocol.md`, the doors and the decision row; a table that omits the decision row fails the check (AC-14); the commit that rewrote the page's decision is refused (AC-10) | No survivor: this is the case the spec exists to stop |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDesignPageGateOwesTheDecisionsSection` | `internal/le/hookruntime/sectionread_test.go` | AC-1, and a page without the section owes nothing | |
| `TestDesignPageGateReadsEveryHeaderLine` | `internal/le/hookruntime/sectionread_test.go` | AC-2: header from disk and from the proposed text, every line | |
| `TestSpecDoorGateOwesTheDecisionsSection` | `internal/le/hookruntime/specdoors_test.go` | AC-3 | |
| `TestSectionRecordGoesStaleWithTheSection` | `internal/le/hookruntime/sectionread_test.go` | AC-4, both directions | |
| `TestSectionReadRecordedForMarkdown` | `internal/le/hookruntime/sectionread_test.go` | AC-5: whole section recorded, cut section and stray copy not | |
| `TestDesignPageGateHoldsEachSubagentToItsOwnRead` | `internal/le/hookruntime/sectionread_test.go` | AC-6 | |
| `TestDesignPageGateRefusesADanglingHeader` | `internal/le/hookruntime/sectionread_test.go` | AC-7, and the correcting edit passes | |
| `TestDesignDecisionsSectionJudged` | `internal/le/doc/consistency/decisions_test.go` | AC-8 rows, absent section silent, missing page reported | |
| `TestCommitRefusesMalformedAddedDecisionRow` | `internal/le/doc/consistency/decisions_test.go` | AC-9, and an untouched malformed section is accepted | |
| `TestCommitRefusesChangedDecisionRow` | `internal/le/doc/consistency/decisions_test.go` | AC-10: changed, removed, added, reordered | |
| `TestDecisionChangeNeedsTheOwnerTerminal` | `internal/le/commit/designapproval_test.go` | AC-11 | |
| `TestDesignDocumentsDerivation` | `internal/le/spec/design/documents_test.go` | AC-13 | |
| `TestDesignCheckJudgement` | `internal/le/spec/design/check_test.go` | AC-14, one case per problem and the clean case | |
| `TestJudgeRecordsADigestBoundVerdict` | `internal/le/spec/design/judge_test.go` | AC-15 | |
| `TestJudgeRefusesUncoveredRow` | `internal/le/spec/design/judge_test.go` | AC-16 rows | |
| `TestJudgeRefusesWithoutAgentIdentity` | `internal/le/spec/design/judge_test.go` | AC-16 identity | |
| `TestApproveRefusesWithoutTerminal` | `internal/le/spec/design/approve_test.go` | AC-17 | |
| `TestApproveRefusesIncompleteOrUnjudgedSpec` | `internal/le/spec/design/approve_test.go` | AC-18 | |
| `TestApproveWritesDigestBoundRecord` | `internal/le/spec/design/approve_test.go` | AC-19 | |
| `TestDesignDigestCoversFourSections` | `internal/le/spec/design/approval_test.go` | AC-24: each section stales it, bookkeeping and the Choices subsection do not | |
| `TestSpecReadyRefusedFromSubagent` | `internal/le/hookruntime/specready_test.go` | AC-20 | |
| `TestSpecReadyNeedsTheApproval` | `internal/le/hookruntime/specready_test.go` | AC-21, and an edit of an already ready spec passes | |
| `TestClaimRefusesReadySpecWithoutApproval` | `internal/le/spec/session_test.go` | AC-22 | |
| `TestClaimAdmitsMatchingApproval` | `internal/le/spec/session_test.go` | AC-23 | |
| `TestClaimRefusesStaleApproval` | `internal/le/spec/session_test.go` | AC-24 | |
| `TestClaimAdmitsApprovalTrailer` | `internal/le/spec/session_test.go` | AC-25 | |
| `TestClaimFailsClosedOnUnreadableRecord` | `internal/le/spec/session_test.go` | AC-26 | |
| `TestAwayRequiresTerminalAndUntil` | `internal/le/spec/design/away_test.go` | AC-27 | |
| `TestClaimAwayAdmitsJudgedFixSpecOnly` | `internal/le/spec/session_test.go` | AC-28, AC-29, lapse after `until` | |
| `TestPendingListsUnapprovedReadySpecsByPriority` | `internal/le/spec/design/pending_test.go` | AC-30 | |
| `TestDesignApprovedTrailerWrittenFromRecord` | `internal/le/commit/designapproval_test.go` | AC-31 | |
| `TestClosureRefusesUncorrectedDeparture` | `internal/le/commit/designapproval_test.go` | AC-32 | |
| `TestClosureRefusesUnapprovedDeparture` | `internal/le/commit/designapproval_test.go` | AC-33 | |
| `TestOwnerRecordWriteRefused` | `internal/le/hookruntime/ownerrecord_test.go` | AC-34: Write, Edit, a shell write, and the reader lines that pass | |
| `TestIncidentPipeChainSpecIsRefused` | `internal/le/spec/design/incident_test.go`, testdata from 2616478e64 | AC-36: the original failure refused at the code edit, the table, the ready edit, the claim and the page commit | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| away `until` | today onward | today | yesterday (already over) | N/A |
| section read range | a range holding the whole section | first to last line of the section | a range ending one line early | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| fixture category `design-decisions-read` | `internal/le/ai/hooks/fixtures.go`, run by `./le ai hooks unit` | AC-1 and its survivor, through the real hook entry | |
| fixture category `spec-ready` | same | AC-20 and AC-21 survivor | |
| fixture category `owner-record` | same | AC-34 | |
| `le-spec-design-check` | a `.ci` beside `test/ui/le-menu-and-prompt.ci`, drafted under `test/draft/` and promoted by `/ze-test` | A user runs `./le spec design documents` and `./le spec design check` on a fixture spec through the launcher and reads the owed rows | |

### Interop Tests (Scope: protocol)
N-A: tooling, no protocol peer.

## Files to Modify
- `internal/le/hookruntime/writeedit.go` - `unreadDoors` owes the Design decisions sections; `doorRefusal` names them beside the doors
- `internal/le/hookruntime/specdoors.go` - the spec gate's reason text names the sections
- `internal/le/hookruntime/lifecycle.go` - `hookSourceRead` records the sections a markdown Read returned whole
- `internal/le/hookruntime/runtime.go` - register `writeSpecReady`, `writeOwnerRecord`, `bashOwnerRecord`
- `internal/le/ai/hooks/fixtures.go` - three fixture categories; the hook source seal
- `internal/le/doc/consistency/consistency.go` - `checkDesignRefs` reports a missing page; `Check` runs the new check
- `internal/le/doc/consistency/register.go` - registers the change-set check
- `internal/le/spec/session.go` - `Claim`: approval and away check before the WIP check; the report line
- `internal/le/spec/citation/anchors.go` - `declaredDesignDocument` returns every `// Design:` line
- `internal/le/commit/actions.go` - `declaredTrailers` gains `Design-approved:` and `Decision-changed:`; the `decision-change` keyword
- `internal/le/commit/authorization.go` - `decision-change` needs the owner terminal
- `internal/le/commit/prepare.go` - call the trailer writer and the closure departure checks
- `internal/le/register.go` - the blank import of the spec design package
- `docs/architecture/README.md` - new "Page format" section defining `## Design decisions`
- `docs/architecture/api/commands.md` - its first Design decisions row: the 2026-08-21 pipe decision (Source: the two records)
- `docs/architecture/core-design.md` - the hook and gate paragraphs, where they list what an edit owes (the `// Design:` page of most files above)
- `docs/contributing/documentation-testing.md` - the new consistency check in "What gets checked"
- `docs/contributing/spec-workflow.md` - `ready` means approved; "Spec approved" names `./le spec design approve`; the two sections; the judge; away mode
- `docs/contributing/committing.md` - the two trailers, the `decision-change` keyword, the decision-row refusal
- `docs/contributing/running-commands.md` - "Issuing owner authorization also requires a terminal" lists the new routes, and `./le deps approve`, which the code gates and the page omits
- `docs/features/ai-first.md` - the `./le commit create` paragraph, if it lists the trailer routes
- `ai/rules/points/planning/`, `ai/rules/points/documentation/`, `ai/rules/points/architecture/` - the points for the approval, the decision section and the widened read; the rendered rules and the condensed files are regenerated from them, never edited
- `plan/templates/TEMPLATE.md` - `## Design documents` after `## Existing Mechanisms`, `## Design presentation` before `## Implementation Steps`
- `ai/skills/ze-spec.md` - derive, write the two sections, run the judge, present, hand Thomas the approve command, then set `ready`
- `ai/skills/ze-review.md`, `ai/skills/ze-review-spec.md`, `ai/skills/ze-close.md` - the design-documents lens
- `.claude/hooks/README.md` - the new checks in the family table
- `plan/learned/HOOK-FRICTION.md` - known false positives of the widened gate once observed

## Files to Create
- `internal/le/hookruntime/sectionread.go`, `internal/le/hookruntime/sectionread_test.go` - the section read record: writer, lookup, owed set
- `internal/le/hookruntime/specready.go`, `internal/le/hookruntime/specready_test.go` - `writeSpecReady`
- `internal/le/hookruntime/ownerrecord.go`, `internal/le/hookruntime/ownerrecord_test.go` - `writeOwnerRecord`, `bashOwnerRecord`
- `internal/le/doc/consistency/decisions.go`, `internal/le/doc/consistency/decisions_test.go` - the section parser, the tree check, the change-set check
- `internal/le/spec/design/register.go`, `internal/le/spec/design/actions.go` - the `le spec design` area and its seven actions
- `internal/le/spec/design/documents.go`, `internal/le/spec/design/check.go`, `internal/le/spec/design/judge.go`, `internal/le/spec/design/approve.go`, `internal/le/spec/design/approval.go`, `internal/le/spec/design/away.go`, `internal/le/spec/design/pending.go` - derivation, judgment, the three records, the digest, the one approval lookup
- `internal/le/spec/design/documents_test.go`, `internal/le/spec/design/check_test.go`, `internal/le/spec/design/judge_test.go`, `internal/le/spec/design/approve_test.go`, `internal/le/spec/design/approval_test.go`, `internal/le/spec/design/away_test.go`, `internal/le/spec/design/pending_test.go`, `internal/le/spec/design/incident_test.go`
- `internal/le/spec/design/testdata/incident-pipe-chain.md` - the incident spec text (2616478e64, Status set to `ready`)
- `internal/le/commit/designapproval.go`, `internal/le/commit/designapproval_test.go` - trailer writers and closure checks
- `ai/skills/ze-design-judge.md` - the independent judge (C-3); `./le ai sync update` writes its stub

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | le tooling |
| YANG validation constraints | N-A | no leaf |
| YANG custom validators | N-A | no leaf |
| CLI commands/flags | Yes | `./le spec design` with `documents`, `check`, `judge`, `approve`, `pending`, `away`, `return`; the `decision-change` keyword of `./le commit create` |
| CLI grammar (keyword before value) | Yes | `spec <path>`, `verdict <word>`, `reason "<text>"`, `words "<text>"`, `until <date>`; `./le cli grammar`; every word shows in `le help` (`ai/META-DECISIONS.md`, "Developer tooling") |
| Editor autocomplete | N-A | le completion derives from registration |
| Functional test for new RPC/API | Yes | the fixture categories and the `.ci` above |
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
| 10 | Test infrastructure changed? | Yes | `docs/contributing/spec-workflow.md`, `docs/contributing/committing.md`, `docs/contributing/documentation-testing.md`, `docs/contributing/running-commands.md` |
| 11 | Affects daemon comparison? | N-A | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/README.md` (page format), `docs/architecture/core-design.md` |
| 13 | Route metadata keys? | N-A | |
| 14 | Prometheus counters? | N-A | |
| 15 | Registered inventory changed? | N-A | |
| 16 | Changed source files referenced by doc anchors? | Yes | derived by `./le spec citation anchors spec plan/release/validation/spec-design-needs-owner-approval.md` at implementation |
| 17 | Existing docs show examples for this area? | Yes | `docs/contributing/spec-workflow.md` event table |

## The documentation format change

`docs/architecture/README.md` gains a "Page format" section. It defines where
an owner decision lives on any page that a `// Design:` header names:

| Element | Rule |
|---------|------|
| Heading | `## Design decisions`, at most once per page |
| Table | Columns `Decision`, `It forbids`, `Source`, `Date` |
| Decision | The decision as Thomas stated it, about this page's subject |
| It forbids | What a design must not do because of it (the shape a reviewer checks a diff against) |
| Source | A tracked path (the record holding his words), a commit, or `ai/META-DECISIONS.md` with its section |
| Date | `YYYY-MM-DD` |
| No decision recorded | The page carries no such section. An empty table is never written: it would claim that no decision binds the page, which nobody can know |
| Generated pages | A page a generator writes (a rendered rule) carries none; its decisions go to the page its generator reads |

A row is added freely, by the main session, in the turn Thomas states the
decision. A row is changed or removed only with him: through an approved
`departs` row of a spec, or through `./le commit create ... decision-change
"<his words>"` run from his terminal.

The check is a `design-decisions` check in `./le doc consistency`, beside
`checkDesignRefs`. It reads every `// Design:` target and reports a malformed
row or a repeated heading; `design-refs` reports a header whose page is
missing. Tree-wide these are reds owed at release. The same judgment is
registered as a change-set check, so `./le commit create` refuses a commit for
what it adds to or takes from a section, and for nothing the section already
held.

## The records

All three live under `tmp/spec-design/`, untracked, written only by
`./le spec design`, each replaced atomically.

| Record | Field | Description |
|--------|-------|-------------|
| `approved/<stem>.md` | Spec | The spec file name |
| | Digest | The design digest at approval |
| | Date, Machine | When and on which host |
| | Owner words | The `words` value, one line |
| | Judge | The judge record's digest, verdict sentence and date |
| | Departures | One line per `departs` row: page, section, quoted statement |
| `judged/<stem>.md` | Digest, Verdict, Sentence, Judge, Date | The design digest, `approve` or `refuse`, the reason, the judge's agent fingerprint |
| `away.md` | From, Until, Owner words | The dates and what he said |

The design digest is the SHA-256 of four sections of the spec, in order, with
trailing blanks trimmed: `## Task`, `## Acceptance Criteria`,
`## Design documents`, and `## Design presentation` without its
`### Choices for Thomas` subsection. It is what the judge judged and what
Thomas approved. Recording his answers in the Choices subsection, moving
Status, and implementation bookkeeping leave it unchanged; a change of scope,
criteria, verdicts or presentation changes it.

One lookup answers "is this spec's design approved": the local record whose
digest matches, else a `Design-approved:` line reachable from HEAD with the
stem and the digest. The departures of a trailer-only approval are the
`departs` rows of the spec text in the commit that carries the trailer.

## Design documents

Filled by hand for this spec: the derivation does not exist yet, so the judge of C-3 re-checks it once it does.

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | The core knows nothing, and everything registers | every path under `internal/` | "A feature registers from `init()` in its own `register.go` and is discovered." | agrees | The actions and the commit check register; pages and decisions come from each file's own header and each page's own section |
| `ai/doors/le-tooling.md` | Who depends on it from outside | `internal/le/` | "A hook that panics or refuses wrongly blocks every session at once" | agrees | The read gate is inert until a page carries a decision; the approve action lands before the claim gate; an edit of an already ready spec is never judged |
| `ai/doors/le-tooling.md` | Boundaries | `internal/le/` | "The package sits at the path its command name predicts" | agrees | `le spec design` is `internal/le/spec/design` |
| `docs/architecture/core-design.md` | Command engine and composition roots | Design header of `session.go`, `writeedit.go`, `specdoors.go`, `lifecycle.go`, `runtime.go`, `anchors.go`, `consistency.go`, `actions.go`, `fixtures.go`, `internal/le/register.go` | "no second command engine for repository tooling" | agrees | The new area registers like every other |
| `docs/contributing/spec-workflow.md` | Spec status vocabulary | Design header of `designevidence.go`, `specwip.go`; a page this spec modifies | "Design complete, ready for implementation" | departs | `ready` comes to mean "design approved by Thomas, or a judged fix spec admitted while he is away"; the page is corrected in this work |
| `docs/contributing/committing.md` | The keywords | Design header of `authorization.go` and the change-set seam | "hand-written `No-news:` line is refused like any declared key." | agrees | Two more declared trailers, same refusal; one more owner keyword beside `attest` |
| `docs/features/ai-first.md` | Development-Time Discovery | Design header of `prepare.go` | "`./le commit create` prepares a commit" | agrees | `create` still prepares the commit |
| `docs/contributing/documentation-testing.md` | Adding a new documentation check | the doc check this spec extends | "A related check joins an existing area rather than opening another root name." | agrees | The new check joins `./le doc consistency` |
| `docs/contributing/running-commands.md` | Issuing owner authorization also requires a terminal | Design header of `human.go` | "No environment variable, keyword or flag skips it." | agrees | The three owner actions and the keyword call the same gate, with no skip |
| `docs/architecture/README.md` | the page's opening | a page this spec modifies | "Architecture documents describe how the current implementation is wired." | agrees | The Page format section adds where decisions live and changes nothing else |
| `docs/architecture/api/commands.md` | The surface splits the pipe chain; the dispatcher refuses one | a page this spec modifies | "The dispatcher answers data, and a pipe chain renders text, so the chain is applied by the surface that renders the answer." | agrees | This spec adds the decision row and leaves the section to the spec that repairs the incident |
| `ai/rules/planning.md` | Directives | the rule this spec enforces | "Five MUST stay in the main thread" | agrees | Enforced for the ready transition |
| `ai/META-DECISIONS.md` | How agents work | owner rule | "a spec's design is put to him and approved before the spec moves to ready, and the main session presents it, never a subagent" | agrees | AC-20 to AC-26 |
| `ai/META-DECISIONS.md` | How agents work | owner rule | "A spec never departs from the design documentation" | agrees | AC-10, AC-14, AC-32, AC-33 |
| `ai/META-DECISIONS.md` | How agents work | owner rule | "A design decision Thomas makes is written into the design page that documents its subject, never into a separate register of decisions." | agrees | The Page format; no register |
| `ai/META-DECISIONS.md` | How agents work | owner rule | "it reads the design page the file's `// Design:` header links" | departs | The owed read is the page's Design decisions section, since the pages cannot be read whole (C-6). If he chooses the whole page, this row becomes `agrees` |
| `ai/META-DECISIONS.md` | How agents work | owner rule | "A hook checks what a command, path or file is, never how a message is worded." | agrees | Every check reads paths, headers, status cells, verdict tokens, verbatim spans, digests and payload identity |
| `ai/META-DECISIONS.md` | How agents work | owner rule | "While Thomas is away, a fix spec's design needs no approval from him" | agrees | AC-27 to AC-29; the judge still runs, at no cost to him |
| `ai/META-DECISIONS.md` | How agents work | owner rule | "The hook that refuses a shell write covers those two trees and is not widened to the rest of the repository." | agrees | That hook is unchanged. `bashOwnerRecord` is another check over one untracked directory of owner records, which no codebase tool writes |
| `ai/META-DECISIONS.md` | Test changes | owner rule | "Owner-only authorization actions require an interactive terminal and refuse noninteractive invocation before recording authorization." | agrees | AC-11, AC-17, AC-27 |

## Design presentation

### What changes

Thomas's decisions are written into the design page that documents their
subject, in a `## Design decisions` section the page format now defines and
`./le doc consistency` validates. The link a file already carries to its
design, the `// Design:` header, makes that section owed reading before any
code edit or spec edit that touches the file, and owed again when a decision
is added. A decision row changes only with him. A spec lists every design page
its files link and every decision on them, each with a verdict; an independent
judge confirms the verdicts; and the design, with every departure, reaches
implementation only after Thomas approves it from his own terminal.

### Before and after

In the proposed messages below, `<spec-path>` means the spec being reviewed.

**1. A code edit before the file's design decisions are read.**

Before: only the doors are owed, so an agent edits `pipe.go` without ever opening `commands.md`.

After:

> BLOCKED: this session or agent has not read the design decisions that bind internal/component/command/pipe.go:
>   - docs/architecture/api/commands.md, ## Design decisions (lines 4410 to 4416): Read it with offset 4410 and limit 7
>   why: the page a file's // Design: header names holds the decisions Thomas made about it, and an edit made before reading them can undo one
>   fix: read each listed range with the Read tool, then retry. A read that stops before the section's last line records nothing

Once read, the agent has seen:

> | Decision | It forbids | Source | Date |
> | A command path names the subject and stops; scope, presentation and format are pipe operations after it, and the pipe wins over a subcommand | Rewriting the command line to carry a pipe as an argument; refusing a pipe a command declares; a subcommand for scope or presentation | plan/records/audit-command-pipe-vs-subcommand.md, plan/records/audit-presentation-pipes.md | 2026-08-21 |

**2. A spec missing a design page.**

Before: the spec only has to name the page, so nobody checks the design against it or against the decision.

After (`./le spec design check spec <spec-path>`, and the same lines from `approve`):

> spec-plugin-dispatch-pipe-chain-reaches-the-handler.md: ## Design documents is missing 2 rows owed by its files
>   - docs/architecture/api/commands.md, Design decisions, decision of 2026-08-21  (the // Design: page of internal/component/command/pipe.go)
>   - docs/architecture/command-ownership.md  (anchored to internal/component/plugin/server/command.go)
>   why: a spec never departs from the design documentation without a discussion with Thomas, so it gives a verdict on every design page its files link
>   fix: run ./le spec design documents spec <spec-path> and add a row with a verdict for each

**3. A decision row that is malformed, or a header with no page.**

Before: nothing checks either.

After, tree-wide (`./le doc consistency`):

> ERROR design-decisions docs/architecture/api/commands.md: row 2 names no Source
> ERROR design-refs internal/component/command/pipe.go: // Design: names docs/architecture/api/command.md, which does not exist

**4. An edit that rewrites a decision.**

Before: the incident edited `commands.md` to fit its own design, and the commit landed.

After (`./le commit create`):

> ./le doc consistency refused what this commit adds:
> docs/architecture/api/commands.md: the decision of 2026-08-21 is changed
>   why: a decision row holds what Thomas decided, and it changes only with him
>   fix: record the departure in the spec's ## Design documents and have Thomas approve it (./le spec design approve spec <spec-path> words "<what he said>"),
>        or, outside a spec, give Thomas this commit command with decision-change "<what he said>" to run from his terminal

**5. A claim of a spec nobody approved.**

Before: `claimed ...: ready -> in-progress (WIP 9/12)`.

After:

> refusing to start spec-plugin-dispatch-pipe-chain-reaches-the-handler.md: its design has no owner approval, and only Thomas records one
>   why: a design reaches implementation only once Thomas approved it (ai/META-DECISIONS.md, "How agents work")
>   fix: run ./le spec design check spec <spec-path>, have ze-design-judge judge it, present its ## Design presentation to Thomas; he runs, from his own terminal:
>     ./le spec design approve spec <spec-path> words "<what he said>"

**6. A subagent sets the spec ready.**

> BLOCKED: <spec-path>: a subagent cannot move a spec to ready
>   why: only the main session presents a design to Thomas, and he approves it
>   fix: hand the design back to the main session and stop

**7. The judge.**

A judge agent that did not write the spec reads the section behind each row, then:

> $ ./le spec design judge spec <spec-path> verdict approve reason "every verdict holds against its section; the one departure is declared"
> judged spec-x.md (digest 3f9a1c0e72b4): approve, 9 rows, each section read by this agent

A judge that skipped a page:

> refused: this agent has not read 1 section the table gives a verdict on
>   - docs/architecture/command-ownership.md, What the Removal Test Forbids: Read it with offset 212 and limit 31

**8. Thomas approves.**

From an agent, the owner-terminal gate refuses and names the command to hand him. From his terminal:

> $ ./le spec design approve spec <spec-path> words "approved, the pipe stays the operator surface"
> approved spec-x.md (digest 3f9a1c0e72b4): design documents 9 (8 agrees, 0 departs, 1 silent), judged approve on 2026-11-12
> next: the main session sets Status to ready; the next commit carrying the spec records Design-approved

**9. What waits for him.**

> $ ./le spec design pending
> P1 release/fixes  spec-a.md  ready since 2026-10-02  lacks: Design documents, Design presentation, judge, approval
> P2 release/fixes  spec-b.md  ready since 2026-11-01  lacks: approval
> ...
> 94 ready specs wait for an approval. Thomas is not away.

**10. Away.**

> $ ./le spec design away until 2026-11-20 words "away until the 20th"
> Thomas away 2026-11-12 to 2026-11-20: a judged ready fix spec with no departs row starts without approval; ./le spec design pending lists each such start for your return

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| A rulings register (pages, door tables or an index of decisions) | Thomas rejected it: another register nobody finds. The page a file links is where an agent already looks |
| Decisions only in `plan/records/` | The incident: a record no link reaches is never read |
| Decisions in the doors | A door covers a subsystem, not the subject of one page, and a file links its page, not its door's decision |
| Owing a whole read of the `// Design:` page | Measured: `core-design.md` alone is about 72,000 tokens and 1091 files link it; the Read tool stops at 25,000 tokens, a Bash print at 25,000 bytes. Put to Thomas as C-6 |
| The section on every page, empty where no decision is known | An empty table claims no decision binds the page. An agent adding it by rote would publish a claim it cannot know, on 400 pages |
| Owing every page `ai/CODE-TO-DOCS.md` anchors to a file before each code edit | Eleven pages, guides included, for `command.go` alone. The anchored architecture pages enter the spec's Design documents list instead (C-1) |
| A separate gate for design pages | `unreadDoors` already serves both the code and the spec gate; a second gate would be a parallel copy of it |
| An edit-time hook that judges the Design documents table on every spec edit | It would refuse every edit of the 94 specs that are `ready` today until each held a complete table, which blocks the repair it asks for. The check runs at approve, and as a command agents run |
| Approval as a metadata row | An agent can write it; a hook cannot tell the main session from Thomas |
| Approval only as a commit trailer | The incident's `ready` never reached a commit; the record must exist in the working tree |
| A digest over the whole spec | Recording his answers, moving Status and ticking a checklist would each send the spec back to him |
| A Bash check on the words `spec design approve` | The terminal gate already refuses an agent, as it does for the five existing owner actions, and running a refused command through a wrapper is already banned. One command would get a second layer the others lack |
| A hook that judges "contradicts" | It would read meaning from wording; the tooling judges completeness and form, the judge agent and Thomas judge meaning |

### Documents departed from

`docs/contributing/spec-workflow.md`, "Spec status vocabulary": `ready` reads
"Design complete, ready for implementation" and "Spec approved" is an edit
anybody makes. After: `ready` means Thomas approved the design, or it is a
judged fix spec admitted while he was away. Corrected in this work.

`ai/META-DECISIONS.md`, "How agents work": "it reads the design page the
file's `// Design:` header links". After, if he takes C-6 (a): it reads the
Design decisions section of that page. The main session corrects the rule's
wording in the same work.

### Choices for Thomas

Thomas chose the recommendation for each of C-1 to C-5 on 2026-11-19, so each
of those Decision cells is settled. C-6 to C-9 are open.

| # | Choice | Options | Decision |
|---|--------|---------|----------|
| C-1 | Which links owe a read before a code edit | (a) the file's `// Design:` pages only; (b) also every architecture page the doc index anchors to the file | (a): the header is the declared link; the anchored architecture pages still reach every spec through its Design documents list, where each gets a verdict |
| C-2 | The specs already `ready` (94 on 2026-11-19, 25 in `release/fixes/`) | (a) each needs your approval before its claim; (b) a baseline you record from your terminal that approves them as they stand; (c) back to `design` | (a) with a printed list by priority: `./le spec design pending` |
| C-3 | An independent design judge before presentation | (a) required, its read records covering every listed page; (b) optional; (c) none | (a): the judge action refuses a judge with no read record of the section behind each row |
| C-4 | How away is bounded | (a) `until <date>` required; (b) open until you return | (a), no maximum |
| C-5 | Does the main session's own ready edit need the approval | (a) yes; (b) only a subagent's is refused | (a) |
| C-6 | What read of the design page an edit owes | (a) the page's `## Design decisions` section, owed again whenever a row is added; (b) the whole page; (c) the whole page when one Read holds it (2000 lines, 25,000 tokens), else the section | Open. Recommended (a). Under (b) an edit of any hook file costs about 72,000 tokens for `core-design.md`, an edit under `plugin/server` about 91,000, and no existing record route can hold a page that size; under (c) two rules exist and the largest, most linked pages still get the section only |
| C-7 | The decisions you made before today, which live in `plan/records/`, `plan/learned/DESIGN-HISTORY.md` and door lines | (a) a child spec in `release/validation` that walks them and proposes each row for you to confirm; (b) each is written when its page is next edited; (c) this spec carries the walk | Open. Recommended (a): until a past decision is a row, this gate cannot protect it, and the walk needs a design of its own. This spec writes only the pipe decision |
| C-8 | What your approval binds | (a) the spec's Task, Acceptance Criteria, Design documents and Design presentation; (b) the whole text except Status, Updated and Phase | Open. Recommended (a): a spec you approved that is blocked and set `ready` again starts without you while those four are unchanged, and comes back to you when any of them changed. Under (b) it comes back after any edit |
| C-9 | Changing a decision outside a spec | (a) the main session edits the row and hands you one `./le commit create ... decision-change "<your words>"` to run; (b) only through a spec's approved departure | Open. Recommended (a): an order you give directly needs no spec (`ai/META-DECISIONS.md`) |

## Implementation Steps

Order matters twice. The approve, judge and pending actions land before the
claim gate, or every claim is refused with no way to clear it. Each hook change
re-seals the hook source digest in its own commit.

1. **Phase: Wiring (MANDATORY FIRST)** (about 50 calls) -- register the `le spec design` area with its seven actions answering "not built", the three hook checks returning nothing, and the change-set check accepting everything; write the failing wiring tests
   - Tests: every test of the Wiring Test table, red
   - Files: `internal/le/spec/design/register.go`, `internal/le/spec/design/actions.go`, `internal/le/register.go`, `internal/le/hookruntime/runtime.go`, `internal/le/doc/consistency/register.go`
   - Verify: `./le cli grammar`; `./le help spec design` lists the seven actions
2. **Phase: page format and its checks** (about 60 calls) -- the section parser, the tree check, the registered change-set check, the `decision-change` keyword and its trailer, "Page format" in `docs/architecture/README.md`, the first decision row in `docs/architecture/api/commands.md`
   - Tests: `TestDesignDecisionsSectionJudged`, `TestCommitRefusesMalformedAddedDecisionRow`, `TestCommitRefusesChangedDecisionRow`, `TestDecisionChangeNeedsTheOwnerTerminal`
   - Files: `internal/le/doc/consistency/`, `internal/le/commit/actions.go`, `internal/le/commit/authorization.go`, `internal/le/commit/designapproval.go`, the two pages, `docs/contributing/documentation-testing.md`
   - Verify: `./le doc consistency` reports no `design-decisions` finding for `commands.md`
3. **Phase: decisions owed before an edit** (about 60 calls) -- the section read record, `hookSourceRead`, `unreadDoors`, the refusal wording, the dangling header, the point bindings, the fixture category
   - Tests: the seven `sectionread_test.go` tests, `TestSpecDoorGateOwesTheDecisionsSection`, fixture category `design-decisions-read`
   - Files: `internal/le/hookruntime/sectionread.go`, `writeedit.go`, `specdoors.go`, `lifecycle.go`, `internal/le/ai/hooks/fixtures.go`, `ai/rules/points/`
   - Verify: `./le ai hooks unit`; A-3 timing on this spec; A-5
4. **Phase: Design documents, check and judge** (about 60 calls) -- every header line in `declaredDesignDocument`, the derivation, the check, the digest, the judge record, the judge skill, the template sections
   - Tests: `TestDesignDocumentsDerivation`, `TestDesignCheckJudgement`, `TestDesignDigestCoversFourSections`, the three judge tests
   - Files: `internal/le/spec/citation/anchors.go`, `internal/le/spec/design/`, `ai/skills/ze-design-judge.md`, `plan/templates/TEMPLATE.md`
   - Verify: the derivation run on this spec reproduces the hand-filled table, and the judge re-checks it
5. **Phase: approve, away, pending, record guard** (about 60 calls)
   - Tests: the three approve tests, `TestAwayRequiresTerminalAndUntil`, `TestPendingListsUnapprovedReadySpecsByPriority`, `TestOwnerRecordWriteRefused`, fixture category `owner-record`
   - Files: `internal/le/spec/design/`, `internal/le/hookruntime/ownerrecord.go`
   - Verify: A-6; `./le spec design pending` on the real tree
6. **Phase: the gates** (about 60 calls) -- `Claim`, `writeSpecReady`, the `Design-approved:` trailer, the closure checks, the incident test
   - Tests: the six claim tests, the two ready tests, `TestDesignApprovedTrailerWrittenFromRecord`, the two closure tests, `TestIncidentPipeChainSpecIsRefused`, fixture category `spec-ready`
   - Files: `internal/le/spec/session.go`, `internal/le/hookruntime/specready.go`, `internal/le/commit/designapproval.go`, `internal/le/commit/prepare.go`
   - Verify: A-1 and A-2 by live probe
7. **Phase: skills, rules, pages** (about 50 calls)
   - Tests: the `.ci` `le-spec-design-check`; AC-35 by reading each file
   - Files: the skills, `ai/rules/points/`, `docs/contributing/spec-workflow.md`, `docs/contributing/committing.md`, `docs/contributing/running-commands.md`, `docs/architecture/core-design.md`, `.claude/hooks/README.md`
   - Verify: `./le doc check all` on the pages; the rules regenerated with `./le ai rules update render` and `./le ai rules update condensed`; `./le ai sync update`

The judge skill, `ai/skills/ze-design-judge.md`, in the shape of
`ai/skills/ze-test-judge.md`: the judge did not write the spec; its input is
the spec path; it runs the derivation, reads the section behind every row and
the producing code where a row's truth depends on it, and decides. APPROVE
only when every `agrees` row holds against its section, every `silent` row's
page is silent on what the design changes, every departure the design makes is
a `departs` row, and the Existing Mechanisms rows name what the design
extends. REFUSE otherwise, naming the row. It records the verdict itself and
never asks Thomas. A refusal stands: the author fixes the design.

The review lens for `/ze-review`, `/ze-review-spec` and `/ze-close`: for each
Design documents row, read the page at the quoted sentence and the diff. A
diff that breaks what a decision row forbids, under an `agrees` verdict, is a
BLOCKER; a `departs` row whose page and decision row the diff does not correct
is a BLOCKER; a design page the derivation now yields for files the
implementation added is an ISSUE that sends the spec back to Thomas.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | The read gate owes exactly the header's pages' sections; the digest covers exactly the four sections; one lookup answers "approved" for every reader |
| Fail-closed | An unreadable header, page, record, away file or git history refuses, never admits; an absent section owes nothing and says nothing |
| No false refusal | An edit of an already ready spec, a page with no section, a commit that leaves a section's rows alone, and a reader-only Bash line all pass |
| Data flow | One owed-set function serves code and spec edits; no le package imports `hookruntime`; `prepare.go` names no page rule |
| Operator errors | Every refusal states what, why and the exact command |
| Rule: hooks judge form | Paths, headers, status cells, verdict tokens, verbatim spans, digests and payload identity only |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Decisions owed before a code edit | `TestDesignPageGateOwesTheDecisionsSection` |
| Page format defined and checked | `TestDesignDecisionsSectionJudged`; `grep -n "Page format" docs/architecture/README.md` |
| The pipe decision is in its page | `grep -n "## Design decisions" docs/architecture/api/commands.md` |
| Claim gate | `TestClaimRefusesReadySpecWithoutApproval` |
| Judge | `TestJudgeRefusesUncoveredRow`; `ls ai/skills/ze-design-judge.md` |
| Incident refused | `TestIncidentPipeChainSpecIsRefused` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Authorization fail-open | Each reader refuses on an unreadable or malformed record |
| Forgery | Trailers refused in a body; records written only behind the terminal gate or by the judge action; hook refusal of a write under `tmp/spec-design/` |
| Path handling | `<stem>` validated as a file name; a `// Design:` target and a Source path resolved inside the checkout |
| Injection | The owner words and the judge's sentence are one line with no trailer key at its start |

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
- The incident spec listed `docs/architecture/api/commands.md` in its own Files to Modify and changed it to fit its departure. The design owner check passed because the page was named. A page the spec edits can never be `silent`, and a decision row cannot be rewritten by the spec that departs from it.
- `internal/component/command/pipe.go` links `docs/architecture/api/commands.md`; had the 2026-08-21 decision been a row of that page, and the row owed before the edit, the implementing agent would have met it.
- The pages are too large to be the unit of reading. The decision section is the unit: small, defined by the format, and digest-bound, so a decision added mid-session reaches an agent that had already read the page.
- `ai/rules/cli.md` disagrees with the 2026-08-21 decision for selector words (`plan/records/audit-command-pipe-vs-subcommand.md` line 555); only the judge and Thomas can see that.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Decisions live in the page the file links | A register, the doors, the records | Thomas: no new register; the link is what gets read |
| The owed read is the Design decisions section, digest-bound | The whole page; a whole-read marker | Measured page sizes; a marker of existence would miss a decision added later |
| No empty section | A required section on every page | An empty table is a claim nobody can back |
| Widen `unreadDoors` | A parallel gate | One owed set for code and spec edits |
| The table is judged by a command and at approve, not on every spec edit | A pre-write hook over `ready` and later | 94 specs are `ready` without it; a hook would block their repair |
| Approval record keyed by spec, bound to a digest of four sections | Per-session file; metadata row; whole-text digest | Approver and claimant are different sessions; bookkeeping must not stale it |
| One area, `le spec design` | Separate `approve`, `away` and citation actions | One package holds the records, the digest and the one lookup |
| Decision rows protected through the change-set seam | A call in `prepare.go` | The commit code names no check |
| Completeness and form are mechanical, meaning is judged | A wording hook | META forbids a hook reading wording |

## Known Limitations

- The tooling cannot judge whether an `agrees` verdict is true, or whether a decision row states the decision faithfully. The judge is a model; Thomas decides.
- A decision Thomas gives in conversation that nobody writes into a page reaches nothing; the same-turn rule is the only control. Decisions made before this spec are not rows until C-7 is answered.
- A file with no `// Design:` header owes no design page, and a decision written into a page no file links is never owed before a code edit.
- The terminal gate proves a terminal, not Thomas, as for every owner action. A record path built at run time can hide from the Bash guard.
- The judge's independence is not proven by the tooling: the record names the judge agent, and nothing records who wrote the spec.
- A session that claims no spec is not gated: `writeSpecStatus` passes it, and `ai/META-DECISIONS.md` lets ordered work run without a spec.
- The away record is local to a machine. Thomas records it on each machine where sessions run.
- An approval that exists only as a local record is not seen from a worktree or the other machine until a commit carries the spec; the claim refusal there says so.
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
- [ ] AC-1..AC-36 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name
- [ ] The targeted tests pass in the main checkout, run by the main thread: `./le test unit pkg path` for each changed `internal/le/` package and `./le ai hooks unit`. No agent runs any `./le verify` command
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
- [ ] Hook fixtures and the `.ci` for end-to-end behavior
- [ ] Interop tests N-A (tooling)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`
- [ ] **Commit A:** code + tests + docs + edited spec
- [ ] **Commit B:** `remove plan/release/validation/spec-design-needs-owner-approval.md` only, in the same `./le commit create` script
