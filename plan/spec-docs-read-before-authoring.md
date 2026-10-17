# Spec: docs-read-before-authoring

| Field | Value |
|-------|-------|
| Status | design |
| Scope | tooling \| docs |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-17 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Owner decision (Thomas, 2026-10-17, dictated): "you need to correctly document
all the subsystems and make sure you are forced to read the documentation before
authoring any plan, etc."

Two failures on 2026-10-17 motivate it. An agent claimed that a functional test
whose DUT reads its config from stdin runs with no store. That is false: the
runner rewrites the DUT to `ze start <conf>` (`routeStdinBlock`,
`internal/test/runner/runner_exec_util.go`, described in
`docs/functional-tests.md`). And `docs/architecture/api/update-syntax.md`
described per-NLRI overrides that the update-text parser does not accept. In the
first case the page was right and nobody read it. In the second the page was
wrong and nobody checked it against the code.

The owner approved four parts:

| # | Part | What it means |
|---|------|---------------|
| 1 | Enforcement | The native write hook refuses authoring until the pages that cover the subject were READ IN THIS SESSION. Authoring is: a code edit, every write under `plan/` (specs, handovers, journal rows, designs), and an agent brief (the `Agent` / `Task` tool's `prompt`). The covering pages of a file come from its `// Design:` header and the code-to-docs map. For a spec, a handover or a brief, the covered files are the paths it names |
| 2 | No page | A file with NO covering page: the hook refuses the edit and names the page to write, so documenting comes before the work |
| 3 | Audit | Every subsystem gets a page-against-code audit by an agent that reads the producing code, and wrong pages are fixed. This week's subsystems first (config storage and the boot record, IRR resolve with the BGP IRR filter and the firewall IRR plugin, the RIB fastpath, the route-injection grammar and update text with the ExaBGP bridge, GTSM), then the rest of the tree |
| 4 | Rollout | The hook warns first, and blocks once the undocumented list is empty. Blocking on day one would stop every agent |

This spec is design only. It sets status `design` and is not claimed for
implementation (the in-progress WIP cap is full).

## Measured Gap (2026-10-17)

How it was measured, so the phase 1 tool can reproduce it:

1. `./le doc index write` regenerated `ai/CODE-TO-DOCS.md` and
   `ai/DOCS-TO-CODE.md` (it reported 340 design docs, 3215 code paths, 672
   packages).
2. A scratch walk read `git ls-files '*.go'`, dropped `vendor/`,
   `gokrazy/modcache/`, `cache/` and every `testdata/` path (the same trees
   `skipPaths` and `skipDirs` in `internal/le/doc/index/docstocode.go` skip),
   dropped `_test.go` files and files whose first 25 lines say
   `Code generated ... DO NOT EDIT`.
3. For each remaining file it read the `// Design:` lines in its first 25 lines
   (`HeaderLines`) and checked that each named page exists. It read the anchors
   from `ai/CODE-TO-DOCS.md`: a file row, or a bullet for the file's OWN
   directory. An anchor on an ancestor directory was not counted, because
   `test/` -> `docs/guide/chaos-testing.md` would otherwise "cover" every file
   under `test/`.

Definitions used in the table:

| Term | Meaning |
|------|---------|
| Hard-uncovered | No `// Design:` line naming a page that exists, AND no anchor on the file or its own directory |
| Nominal | A `// Design:` page exists, but no page anchors the file or its own directory, so no page is known to describe it |
| Core-design-only | Every existing `// Design:` page the file names is `docs/architecture/core-design.md` (168 KB) |
| Not a Ze page | Every existing `// Design:` page the file names is under `ai/` (120 files) or `rfc/short/` (not counted). The GTSM files are an example: their pages are `rfc/short/rfc5082.md` and `docs/architecture/doctor-and-health-checks.md`, so the RFC summary is the only page for the TTL logic. The gate counts these as covered; the audit treats them as nominal |
| Package | A directory holding at least one production Go file |

| Population | Files | Packages | No `// Design:` header | Header names only missing pages | Hard-uncovered | Nominal | Core-design-only | Packages wholly hard-uncovered | Packages with no own anchor |
|------------|-------|----------|-----------------------|-------------------------------|----------------|---------|------------------|-------------------------------|-----------------------------|
| All production Go | 5264 | 883 | 660 | 86 | 635 | 2224 | 786 | 182 | 646 |
| Without `doc.go` | 4910 | 719 | 331 | (in 86) | 323 | 2207 | 785 | 28 | 488 |
| Without `doc.go` and `internal/test/` | 4438 | 676 | 139 | (in 86) | 141 | 2043 | 774 | 16 | 450 |

Other measured facts:

| Fact | Value |
|------|-------|
| `doc.go` files with no `// Design:` header | 329 |
| Files with a `// Design:` line after line 25 (invisible to the index) | 3 |
| Files whose header says `(none ...)` | 79 |
| Distinct pages named by `// Design:` headers | 335 |
| Files naming `docs/architecture/core-design.md` | 791 |
| Generated Go files excluded | 435 |
| Hard-uncovered by tree: `internal/component` / `internal/plugins` / `internal/component/bgp/plugins` / `internal/le` / `internal/core` / `internal/test` / `cmd` | 138 / 122 / 62 / 42 / 43 / 192 / 5 |
| `plan/journal/documentation-shows-config-the-parser-refuses.md` rows still `open` | 16 |

This week's subsystems, production files (no header / no own anchor /
hard-uncovered):

| Unit | Files | No header | No own anchor | Hard-uncovered |
|------|-------|-----------|---------------|----------------|
| `internal/component/config/` (includes `storage/`, the boot record) | 204 | 10 | 98 | 10 |
| `internal/component/resolve` (IRR store) | 25 | 1 | 16 | 3 |
| `internal/component/bgp/plugins/filter_irr` | 10 | 2 | 1 | 1 |
| `internal/component/firewall` (includes `plugins/irr`) | 39 | 4 | 18 | 4 |
| `internal/component/bgp/plugins/rib` (fastpath forward tracker) | 54 | 5 | 24 | 4 |
| `internal/component/bgp/plugins/cmd/update` (update text grammar) | 11 | 2 | 6 | 2 |
| `internal/exabgp` (bridge) | 26 | 0 | 7 | 0 |
| `internal/component/gtsm` | 5 | 0 | 5 | 0 |

The consistency gate already knows part of this:
`checkDesignRefs` (`internal/le/doc/consistency/consistency.go`) reports a
missing `// Design:` line, at warning severity only.

## Required Reading

### Architecture Docs
- [ ] `.claude/hooks/README.md` - the page for `internal/le/hookruntime`: event wiring, every check, the marker files
  → Constraint: `hookTableProblems` (`internal/le/ai/rules/hooktable.go`) compares the README tables with `nativeHookActions`, so each new check owes a table row with its `Enforces` column in the same change
  → Constraint: `PostToolUse Read` runs `mark-source-read`, which today records only the style guide and a source kind (`.go`, `.sh`, `.yang`, `.mk`); a Read of a `.md` page leaves no record, so the read ledger is new
  → Decision: the `PreToolUse Task|Agent` event is not wired today; only `PostToolUse Task|Agent` (`mark-agent-spawned`) is
- [ ] `docs/contributing/navigating-the-code.md` - how the indexes are built and invalidated
  → Constraint: the three indexes are untracked and DERIVED; `postInvalidateDerived` removes one after a Write or Edit to an input, so the hook MUST NOT read `ai/CODE-TO-DOCS.md` from disk and MUST call the producer
  → Constraint: `// Design:` headers are scanned in the first 25 lines only (`HeaderLines`, `designLine`)
- [ ] `docs/contributing/documentation-testing.md` - the existing doc gates, the `doc-drift` check and `plan/doc-reviewed/`
  → Constraint: `checkDocDrift` (`internal/le/doc/wiring/checks.go`) already refuses a commit that changed an anchored symbol and left its page; the audit's anchors therefore become drift guards for free, and this spec adds no second drift check
  → Decision: `plan/doc-reviewed/` records behavior-preserving reviews keyed by commit; it is not the audit ledger, and the audit record lives in this spec's Audit Record table
- [ ] `ai/rules/documentation.md` - the rule this hook enforces
  → Constraint: "You MUST read the page that covers a surface before you spawn an agent, open a broad search, or read wide code"; the hook enforces the write side and the brief side, never searches
- [ ] `ai/rules/user-facing-errors.md` - refusal text: problem, why, the command on its own line, STE, color off where `slogutil.UseColor` says so
- [ ] `docs/contributing/running-commands.md` - session identity, and what each agent owns
  → Constraint: a subagent's payload carries the parent's `session_id` and its own `agent_id`; per-agent files join them with `lepath.AgentFingerprint` so the session reaper removes them
- [ ] `docs/contributing/ze-go-style.md` - precondition for every Go phase of this spec

**Key insights:**
- Two producers already answer "which pages cover this path": `docindex.DocumentsByPath` (`internal/le/doc/index/codetodocs.go`, anchors from `docs/`, read from the tree not the rendering) and the `// Design:` parser `designRefs` / `designLine` (`internal/le/doc/index/docstocode.go`). A third, `declaredDesignDocument` (`internal/le/spec/citation/anchors.go`), re-parses headers and keeps only the FIRST `// Design:` line.
- `hookruntime` can import `internal/le/doc/index` with no cycle (`go list -deps` of the index package holds no `hookruntime`), and already imports `internal/le/spec/citation`, which imports the index.
- `./le doc index check` runs in 0.96 s wall including process start; the in-process walk is the cost the hook pays per call.
- A `PreToolUse Task|Agent` hook existed until `888e0a7285` (2026-09-26) as `pretool-agent-skill`, which read `tool_input.prompt`. It was removed under the owner rule "a hook checks what a command, path or file is, never how a message is worded". A gate on the PATHS a brief names is a path check, so it is inside that rule.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/hookruntime/runtime.go` - `nativeHookActions` maps `pretool-bash`, `pretool-writeedit`, `posttool-writeedit` to ordered checks; `context` carries root, tool, input, path, content, payload
- [ ] `internal/le/hookruntime/writeedit.go` - `writeDesignEvidence` refuses a spec or `plan/design-*` write unless an LSP marker or a `.source-read-*-<session>` marker exists (session-wide, not per agent); `writeStyleGuideRead` refuses a Go Write/Edit/MultiEdit without the per-agent `.style-guide-read-<fingerprint>` marker, fail-closed when the id is unusable
- [ ] `internal/le/hookruntime/lifecycle.go` - `runLifecycleHook` dispatches `mark-source-read` to `hookSourceRead` and `mark-agent-spawned` to a session marker; `hookSourceRead` writes the style-guide marker only for a whole-file Read (`readWholeFile`)
- [ ] `internal/le/hookruntime/bash.go` - `bashStyleGuideRead` records a Bash print of the style guide (`printsStyleGuide`: `cat` with no pipe); `bashGovernedWrite` refuses a shell write into `plan/` or `ai/rules/`
- [ ] `internal/le/hookruntime/session.go` - `resolvedSessionID`, `sessionMarker`
- [ ] `internal/le/le/path/commitsession.go` - `AgentFingerprint(session, agent)` answers `session` for the main thread and `session` plus separator plus agent for a subagent
- [ ] `internal/le/doc/index/docstocode.go` - `Build`, `walkGo`, `designRefs`, `designLine`, `SkipsDir`, `IsDesignSource`
- [ ] `internal/le/doc/index/codetodocs.go` - `DocumentsByPath`, `buildCodeIndex`
- [ ] `internal/le/doc/index/codetodocs_report.go` - `ClaimsByPath`, `UpdateCodeIndex`
- [ ] `internal/le/spec/citation/anchors.go` - `AuditAnchors`, `declaredDesignDocument` (first header line only), `specSourceFiles` (bullets under Files to Modify/Create), `isSourcePath` (`sourcePrefixes`, `sourceSuffixes`)
- [ ] `internal/le/doc/consistency/consistency.go` - `checkDesignRefs` warns on a missing `// Design:` line
- [ ] `.claude/settings.json` - hook wiring

**Behavior to preserve:**
- Every existing check in `nativeHookActions`, its order and its verdicts
- `writeDesignEvidence` (source read before a spec) stays; the new check is in addition, judging pages rather than source
- `writeStyleGuideRead` and its marker stay; the style guide is also a page, but its whole-read rule is its own
- `./le doc index write` / `check` output and the two index renderings
- `./le spec citation anchors` verdicts, apart from now seeing every `// Design:` line rather than the first

**Behavior to change:**
- A Read or a Bash print of a page records it in a per-context read ledger
- Code edits, `plan/` writes and agent briefs are judged against that ledger
- A file with no covering page is refused with the page to write
- `PreToolUse Task|Agent` is wired to a new `pretool-agent` action
- A coverage report and a shrink-only baseline exist, and the baseline's emptiness switches warn to block

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- The Claude Code harness runs `$CLAUDE_PROJECT_DIR/le ai hooks <action>` with a JSON payload on stdin: `tool_name`, `tool_input` (`file_path`, `content` / `new_string`, or `prompt`), `session_id`, `agent_id`, `agent_type`
- An operator or agent runs `./le doc coverage ...` from Bash

### Transformation Path
1. Read recording: `PostToolUse Read` (`mark-source-read`) and `PreToolUse Bash` (`pretool-bash`) append the page path to `tmp/session/.doc-read-<fingerprint>`
2. Coverage: for a path, the coverage producer answers the declared pages (every `// Design:` line naming an existing page) and the anchor pages (exact path or own directory, from `DocumentsByPath`)
3. Subject: a code edit's subject is its own path; a `plan/` write's and a brief's subjects are the repository paths the proposed text names
4. Verdict: required pages minus the ledger; empty means pass; otherwise exit 1 while the baseline is non-empty, exit 2 when it is empty
5. Uncovered: a code edit whose path has no covering page gets the no-page verdict under the same switch

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Harness to `le` | JSON on stdin, exit code 0/1/2, message on stderr | No |
| Hook to doc index | in-process call to the coverage producer in `internal/le/doc/index` | No |
| Hook to session tree | append-only ledger file under `tmp/session/` | No |

### Integration Points
- `nativeHookActions` - new checks appended to `pretool-writeedit`, new `pretool-agent` group
- `hookSourceRead`, `bashStyleGuideRead`'s sibling in `pretool-bash` - ledger writers
- `docindex.DocumentsByPath` and `designRefs` - the only coverage producers
- `citation.AuditAnchors` - switches from `declaredDesignDocument` to the shared producer
- `.claude/settings.json` - one new `PreToolUse` entry with matcher `Task|Agent`
- `./le doc index check` - runs the baseline check

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | the hook calls the index producer in-process; it never parses the rendered `ai/CODE-TO-DOCS.md` |
| No unintended coupling (components stay isolated) | Yes | only `internal/le` packages change; no product component is touched |
| No duplicated functionality (extends existing, does not recreate) | Yes | `declaredDesignDocument` is deleted and its caller uses the shared parser; the subject classifier `isSourcePath` moves to one place both callers use |
| Zero-copy preserved where applicable (refs, not copies) | N-A | tooling, no wire path |
| Registration over hardcoding, outbound | Yes | the new checks register in `nativeHookActions`, the registry the rule-coverage gate reads |
| Registration over hardcoding, inbound | Yes | page lists are derived from headers and anchors, never listed; searched: `nativeHookActions`, `internal/le/ai/hooks/actions.go` categories, `hookTableProblems`, the parity fixture tables, `.claude/settings.json`; each derives from `nativeHookActions` or is the harness wiring |

## Design

### Covering pages for a path

| Path class | Declared pages (required) | Anchor pages (listed, not required) |
|------------|---------------------------|-------------------------------------|
| Production `.go` | every existing page its `// Design:` lines name (first 25 lines) | `DocumentsByPath` for the exact path and for its own directory |
| `doc.go` | the union of the declared pages of its package's production files | as above |
| `_test.go` | the declared pages of the same-stem production file; with none, the union over its package | as above |
| Generated `.go` | none required: `writeGenerated` already refuses the edit | - |
| `.yang`, `.ci`, `.et`, `.wb`, `.sh`, `.templ` | the anchor pages for the exact path or own directory; with none, the declared pages of the nearest enclosing directory holding production Go | - |
| A directory named in prose | the union of the declared pages of its production files, plus its own-directory anchors | - |
| A path named in prose that does not exist (Files to Create) | the coverage of the nearest existing ancestor directory | - |

A path is **covered** when its declared set is non-empty, or when it has an
exact or own-directory anchor. An ancestor anchor never covers.

### Read ledger

| Field | Value |
|-------|-------|
| File | `tmp/session/.doc-read-<fingerprint>`, fingerprint from `lepath.AgentFingerprint(session, agent_id)` |
| Content | one repository-relative page path per line, appended |
| Writers | `hookSourceRead` for a Read of a tracked `.md` path; a new `pretool-bash` check for `cat`, `sed -n`, `head` or `tail` whose operand is a tracked `.md` path |
| Not a read | `grep`, `rg`, a page named only in a pipe's later stage, a Read of a path outside the checkout |
| Scope | per context: a parent's read does not satisfy a subagent, the same rule as the style guide |
| Removal | the session reaper, by the fingerprint naming |

### The checks

| Check | Group | Subject | Refuses |
|-------|-------|---------|---------|
| `writeDocsCovered` | `pretool-writeedit` | a code edit (classes above) | the path has no covering page |
| `writeDocsRead` | `pretool-writeedit` | a code edit's own path; a `plan/` Write's `content` or Edit's `new_string`, every repository path it names | a required page that is not in the ledger |
| `bashDocRead` | `pretool-bash` | the command | never refuses; records reads |
| `agentDocsRead` | `pretool-agent` (new, `PreToolUse Task\|Agent`) | the `prompt`, every repository path it names | a required page not in the spawner's ledger, or not named in the prompt (see D-3) |

Path extraction from text: a token is a subject when it matches the moved
`isSourcePath` classes, or names a directory under one of `sourcePrefixes`, and
exists in the tree (or its nearest ancestor does, for Files to Create). A path
on a `remove` line of a commit plan is not a subject.

### Rollout switch

| State | Severity of every verdict above |
|-------|---------------------------------|
| `ai/doc-coverage-baseline.txt` lists at least one path | exit 1, message prefixed `WARN:` and ending with the switch line |
| The baseline is empty or absent | exit 2, `BLOCKED:` |

The baseline is written by `./le doc coverage baseline write` and judged by
`./le doc coverage check`, which `./le doc index check` runs:

| Finding | Why it is refused |
|---------|-------------------|
| A listed path is now covered | the list must shrink as pages land; the entry is removed by rerunning the writer |
| A hard-uncovered path is not listed and was not listed at `HEAD` | a new undocumented file (see D-4) |
| A listed path was not listed at `HEAD` | the list can only shrink |

### Refusal texts (text samples, STE)

Read gate, blocking state:

| Line | Text |
|------|------|
| 1 | `BLOCKED: you did not read the page that documents internal/component/gtsm/gtsm.go.` |
| 2 | `  Read the page before you change the code. A change made without it can make the page and the code disagree.` |
| 3 | `  Read this page, then try again:` |
| 4 | `    rfc/short/rfc5082.md` |
| 5 | `  These pages also mention the file:` followed by the anchor pages, one per line |
| 6 | `  ai/rules/documentation.md` |

No page, blocking state:

| Line | Text |
|------|------|
| 1 | `BLOCKED: no page documents internal/x/y.go.` |
| 2 | `  Every file needs a page, so that the next reader can learn the code without reading all of it.` |
| 3 | `  Write the page first. Then name it in the first 25 lines of the file:` |
| 4 | `    // Design: docs/architecture/x/y.md -- what this file does` |
| 5 | `  To see the pages the other files in this package use:` |
| 6 | `    ./le doc coverage path internal/x/y.go` |

The proposed page is the page most cited by the package's other files; with none,
`docs/architecture/<area>/<package>.md`.

Plan write and brief: line 1 becomes `BLOCKED: this spec names <path>, and you
did not read the page that documents it.` (or `this brief`), and the brief form
adds `  Name the page in the brief too, so that the agent reads it.` One verdict
lists every missing page, one per line, grouped by the path that needs it.

Warning state: `WARN:` replaces `BLOCKED:` and a last line reads
`  This becomes a refusal when ./le doc coverage report shows no undocumented file.`

No usable id: `BLOCKED: this session or agent has no usable id, so the hook cannot
find the pages you read.` then `  Set CLAUDE_CODE_SESSION_ID to the parent session
id, then try again.` In the warning state this is a warning.

Color follows the existing verdicts (red and bold on the first line only) and is
off wherever `slogutil.UseColor` turns it off.

### `./le doc coverage`

| Verb | Answer |
|------|--------|
| `report` | the counts of the Measured Gap table and the lists, by unit; `| json` gives rows |
| `report unit <dir>` | one unit's files with their declared and anchor pages, and their class (covered, nominal, core-design-only, hard-uncovered) |
| `path <path>` | the covering pages of one path, required and listed, and whether this context read each |
| `baseline write` | rewrites `ai/doc-coverage-baseline.txt` from the hard-uncovered list |
| `check` | the baseline findings above |

## Open Owner Decisions

| ID | Question | Options | Recommendation |
|----|----------|---------|----------------|
| D-1 | Does a partial read count? | (a) any Read or print of the page; (b) whole page only, like the style guide | (a). `core-design.md` is 168 KB and `functional-tests.md` 216 KB; a whole-read rule makes every edit to the 791 files citing `core-design.md` cost about 47k tokens. Both motivating failures were "never opened", which (a) stops |
| D-2 | Which pages are required? | (a) declared pages only, anchors listed; (b) declared plus every anchor page | (a). `internal/le/hookruntime/writeedit.go` has 1 declared page and 9 anchor pages |
| D-3 | Brief gate: what is required? | (a) the spawner read the pages (your words); (b) the brief names the pages; (c) both | (c). (a) alone puts every page into the main thread, against the delegation rule; (b) alone lets the main thread brief without reading. Both are path checks, so the 2026-09-26 "no wording gates" decision holds |
| D-4 | Is a NEW undocumented file refused at commit from day one? | (a) yes, by `./le doc coverage check`; (b) no, warn only until the baseline is empty | (a). A new file already owes a `// Design:` header under the go-standards rule; without the ratchet the list can grow while the audit shrinks it |
| D-5 | Does a file whose only page is `core-design.md` (786 files), or only an `ai/` or `rfc/short/` page, count as documented for the switch? | (a) yes, the audit retargets them later; (b) no, they join the baseline (786 more entries) | (a) for the switch, with phase 8 retargeting every one. (b) puts the block months away |
| D-6 | Global switch or per-file? | (a) global: all verdicts warn until the baseline is empty (your words); (b) per-file: block at once for files not in the baseline | (a), as you said; (b) would start blocking the read gate on covered code on day one |

## Audit

### Unit

An audit unit is one directory at these levels, with every page that covers a
file in it:

| Tree | Unit |
|------|------|
| `internal/component/<name>/` | one component (46) |
| `internal/component/bgp/plugins/<name>/` | one BGP plugin (32) |
| `internal/plugins/<name>/` | one system plugin (66) |
| `internal/core/<name>/`, `internal/le/<name>/` | one package group |
| `cmd/<name>/`, `pkg/<name>/`, `internal/test/<name>/`, other `internal/<name>/` | one directory |

`./le doc coverage report` lists the units, so the list is derived, never written.

### Per-unit checklist (the agent reads the producing code)

| Step | Action | Output |
|------|--------|--------|
| 1 | Run `./le doc coverage report unit <dir>` | the before row |
| 2 | Read every covering page of the unit | the page list |
| 3 | List every factual claim the pages make about the unit: behavior, syntax, defaults, refusals, examples, file and symbol names | the claim list |
| 4 | For each claim, read the producing function (`gopls`), and give a verdict: true, false, or not checkable, with file and symbol | the claim table |
| 5 | A false claim: read the producer to decide which side is wrong. Wrong page: fix it. Wrong code: a defect row in `plan/journal/<class>.md`, or an RFC gap under `ai/rules/rfc-compliance.md`; never edit the page to describe a bug as intended | page edits, journal rows |
| 6 | Run every runnable example (`ze config validate`, a `.ci` reference, the CLI) | pass or fixed |
| 7 | Add a `<!-- source: <file> -- <symbols> -->` anchor under each verified claim | anchors, which `checkDocDrift` then guards |
| 8 | A hard-uncovered, nominal or core-design-only file: write or extend the page and point its `// Design:` header at it | header edits |
| 9 | Run `./le doc coverage report unit <dir>` and `./le doc index check` | the after row |
| 10 | Commit; then `/ze-review-docs` in another context over that commit | review clean |

### Phase proof

A phase is done when every unit in it has an Audit Record row with a commit SHA
and a clean review; `./le doc coverage report` shows 0 hard-uncovered, 0 nominal,
0 core-design-only and 0 not-a-Ze-page files for those units; `./le doc index check` passes; and
the claim tables show every claim with a verdict.

### Audit Record

| Unit | Pages | Claims | False, fixed | Defects recorded | Before / after | Commit | Review |
|------|-------|--------|--------------|------------------|----------------|--------|--------|

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `PreToolUse` with matcher `Task\|Agent` receives `tool_input.prompt` and exit 2 blocks the spawn | `pretool-agent-skill` read `prompt` until `888e0a7285` removed it | the brief gate cannot exist | a probe: wire it, spawn an agent with an unread page, observe the refusal | unvalidated |
| A-2 | The in-process coverage walk fits the hook's 5 s timeout with margin | `./le doc index check` 0.96 s wall | edits get slow or the harness kills the hook | `BenchmarkCoveragePath`, budget 300 ms on this machine | unvalidated |
| A-3 | `hookSourceRead` sees the path of every Read of a `.md` file | `PostToolUse Read` is wired with no matcher limit | reads go unrecorded and the gate refuses after a real read | `TestDocReadLedgerRecordsReadTool` | unvalidated |
| A-4 | `bashGovernedWrite` stops shell writes into `plan/`, so a plan write cannot skip the Write/Edit hook | its doc comment and `bash_governed_taint.go` | specs bypass the gate | observed 2026-10-17 while writing this spec: a `python3` rewrite of this file was refused with "shell write to plan/ or ai/rules/" | confirmed |
| A-5 | The 86 files whose header names missing pages are mostly parse artifacts (`(RFC`, `DST`, `register.go`) rather than deleted pages | the missing-target list printed by the measurement | the header parser needs a fix before the count is trusted | phase 1 lists them by cause | unvalidated |
| A-6 | Every `// Design:` line counts, not only the first | `designRefs` reads all, `declaredDesignDocument` reads the first | required sets differ between tools | `TestCoverageReadsEveryDesignLine` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Main-thread context grows from reading pages before briefs | `./le ai tokens` per-session cost rises | D-3 (c): the brief names the pages and the agent's own writes are gated; phase 8 retargets catch-all headers to small pages |
| R-2 | Path extraction from prose picks up a path in a quoted command or a removed file | a refusal naming a path the author did not mean | only existing paths count; `remove` lines are skipped; the verdict names the path and the text that named it |
| R-3 | An Edit to a large spec judges the whole spec | refusals on pages unrelated to the edit | an Edit judges `new_string` only |
| R-4 | Agents learn to Read a page and ignore it | wrong claims keep appearing in specs | the audit makes pages right and anchored; `checkDocDrift` keeps them right; the gate only guarantees the page was opened |
| R-5 | Someone adds paths to the baseline to silence the gate | the baseline grows in a diff | the check refuses any entry absent at `HEAD` |
| R-6 | Go written through a Bash heredoc skips the gate | none from the hook | the same limit `writeStyleGuideRead` has; `checkDesignRefs` and the commit-time ratchet still see the file |
| R-7 | Warnings are ignored for weeks | the audit stalls | the ratchet (D-4) holds the line; the Audit Record shows progress per unit |
| R-8 | Concurrent appends to one ledger corrupt it | a line that is not a path | one short line per append in append mode; a reader ignores a line that is not a tracked page |
| R-9 | A wrong page passes the gate because it was read | an audit finds a claim the gate let through | phase order: this week's subsystems first, where wrong pages were found |
| R-10 | Main thread blocked by its own gate when it carries a brief about the hook itself | a refusal on `internal/le/hookruntime` | the hooks README becomes the declared page of the new files, a small page |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | No product behavior. Agent edits, plan writes or spawns get refused or slowed; in the warning state nothing is refused |
| How is it reverted? | single commit revert of the hook change; the settings entry is one block |
| Who else touches this path? | every session's hooks; `internal/le/hookruntime` and `.claude/settings.json` are shared with all agents |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le ai hooks pretool-writeedit` with a Go Edit payload | → | `writeDocsRead` | `TestPretoolWriteeditRunsDocsRead` |
| `./le ai hooks pretool-writeedit` with a Go Write to an uncovered path | → | `writeDocsCovered` | `TestPretoolWriteeditRunsDocsCovered` |
| `./le ai hooks pretool-writeedit` with a spec Write | → | `writeDocsRead` over named paths | `TestPretoolWriteeditJudgesSpecPaths` |
| `./le ai hooks pretool-agent` with an Agent payload | → | `agentDocsRead` | `TestPretoolAgentRunsDocsRead` |
| `.claude/settings.json` `PreToolUse` `Task\|Agent` | → | `pretool-agent` action | `TestSettingsWiresEveryNativeAction` |
| `./le ai hooks mark-source-read` with a `.md` Read | → | ledger append | `TestDocReadLedgerRecordsReadTool` |
| `./le ai hooks pretool-bash` with `cat docs/x.md` | → | `bashDocRead` | `TestDocReadLedgerRecordsBashPrint` |
| `./le doc index check` | → | baseline check | `TestDocIndexCheckRunsCoverageBaseline` |
| `./le doc coverage report` | → | coverage producer | `TestDocCoverageReportAction` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A Read of a tracked `.md` page from a context | the page path is a line of that context's ledger |
| AC-2 | A Bash `cat`, `sed -n`, `head` or `tail` of a tracked page | recorded; a `grep` of the same page is not |
| AC-3 | A subagent edits a file whose page only its parent read | refused (warned in the warning state) |
| AC-4 | A Go Edit whose declared pages are all in the ledger | passes |
| AC-5 | A Go Edit with an unread declared page, baseline non-empty | exit 1, message `WARN:` naming every unread page and the switch line |
| AC-6 | The same, baseline empty | exit 2, `BLOCKED:` |
| AC-7 | A Go Write to a path with no covering page | the no-page verdict naming the proposed page and the `./le doc coverage path` command |
| AC-8 | A file whose only anchor is on an ancestor directory | treated as uncovered |
| AC-9 | A spec Write naming three source paths, one page unread | one verdict listing that page under the path that needs it |
| AC-10 | A spec Edit whose `new_string` names no path | passes, whatever the rest of the spec names |
| AC-11 | A spec naming a path that does not exist yet | judged by the nearest existing directory |
| AC-12 | An Agent spawn whose prompt names a path with an unread page, or does not name that page | refused under the switch (per D-3) |
| AC-13 | Writes under `plan/journal/`, `plan/handover/`, `plan/design/` | judged like a spec |
| AC-14 | No usable session id, baseline empty | refused with the id message; baseline non-empty: warned |
| AC-15 | `./le doc coverage report` on the tree of 2026-10-17 | reproduces the Measured Gap counts, or names each difference and its cause |
| AC-16 | `./le doc coverage check` with a covered path still listed, an unlisted new uncovered path, or a listed path absent at `HEAD` | each is a finding and the check fails |
| AC-17 | `./le spec citation anchors` on a file with two `// Design:` lines | both pages are reported (one parser) |
| AC-18 | Every message above with `NO_COLOR` set or stderr not a terminal | no escape codes |
| AC-19 | Phase 7 complete | the six units of this week have Audit Record rows meeting the Phase proof |
| AC-20 | Phase 8 complete | `./le doc coverage report` shows 0 hard-uncovered, 0 nominal and 0 core-design-only files tree-wide, and the baseline is empty |
| AC-21 | The baseline becomes empty | the next unread-page edit is refused with exit 2, with no code change |

## 🧪 TDD Test Plan

Each test is written first and shown failing before its check exists, then
passing. The discrimination step for each gate: remove the check from
`nativeHookActions`, see its fixtures go red, restore it.

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCoverageReadsEveryDesignLine` | `internal/le/doc/index/coverage_test.go` | AC-17, A-6 | |
| `TestCoverageMissingPageIsUncovered` | `internal/le/doc/index/coverage_test.go` | a header naming a deleted page | |
| `TestCoverageOwnDirAnchorCovers` | `internal/le/doc/index/coverage_test.go` | own-directory anchor | |
| `TestCoverageAncestorAnchorDoesNotCover` | `internal/le/doc/index/coverage_test.go` | AC-8 | |
| `TestCoverageTestFileUsesSiblingPages` | `internal/le/doc/index/coverage_test.go` | `_test.go` rule | |
| `TestCoverageDocGoUsesPackagePages` | `internal/le/doc/index/coverage_test.go` | `doc.go` rule | |
| `TestCoverageNonGoUsesEnclosingPackage` | `internal/le/doc/index/coverage_test.go` | `.yang`, `.ci` rule | |
| `TestCoverageReportCounts` | `internal/le/doc/index/coverage_test.go` | AC-15 over a built tree | |
| `TestBaselineCheckRefusesCoveredEntry` | `internal/le/doc/index/coverage_test.go` | AC-16 | |
| `TestBaselineCheckRefusesNewUncovered` | `internal/le/doc/index/coverage_test.go` | AC-16, D-4 | |
| `TestBaselineCheckRefusesGrowth` | `internal/le/doc/index/coverage_test.go` | AC-16 | |
| `BenchmarkCoveragePath` | `internal/le/doc/index/coverage_test.go` | A-2 | |
| `TestDocReadLedgerRecordsReadTool` | `internal/le/hookruntime/docsread_test.go` | AC-1 | |
| `TestDocReadLedgerRecordsBashPrint` | `internal/le/hookruntime/docsread_test.go` | AC-2 | |
| `TestDocReadLedgerIsPerAgent` | `internal/le/hookruntime/docsread_test.go` | AC-3 | |
| `TestWriteDocsReadPassesAfterRead` | `internal/le/hookruntime/docsread_test.go` | AC-4 | |
| `TestWriteDocsReadWarnsWhileBaselineListed` | `internal/le/hookruntime/docsread_test.go` | AC-5 | |
| `TestWriteDocsReadBlocksWhenBaselineEmpty` | `internal/le/hookruntime/docsread_test.go` | AC-6, AC-21 | |
| `TestWriteDocsCoveredNamesPageToWrite` | `internal/le/hookruntime/docsread_test.go` | AC-7 | |
| `TestWriteDocsReadListsPagesPerPath` | `internal/le/hookruntime/docsread_test.go` | AC-9 | |
| `TestWriteDocsReadEditJudgesNewStringOnly` | `internal/le/hookruntime/docsread_test.go` | AC-10 | |
| `TestWriteDocsReadFilesToCreateUseAncestor` | `internal/le/hookruntime/docsread_test.go` | AC-11 | |
| `TestWriteDocsReadJudgesEveryPlanTree` | `internal/le/hookruntime/docsread_test.go` | AC-13 | |
| `TestAgentDocsReadRefusesUnreadOrUnnamed` | `internal/le/hookruntime/docsread_test.go` | AC-12 | |
| `TestDocsGatesNoSessionID` | `internal/le/hookruntime/docsread_test.go` | AC-14 | |
| `TestDocsGateMessagesHonorNoColor` | `internal/le/hookruntime/docsread_test.go` | AC-18 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `// Design:` line position | 1-25 | 25 | N/A | 26 (not read, file is uncovered) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| behavior fixtures for the four checks | `internal/le/ai/hooks/fixture_catalog.go`, run by `./le ai hooks unit` | an agent payload in, the verdict out, through the dispatcher | |
| parity golden table for `pretool-agent` | `internal/le/ai/hooks/parity.go` | the fifth check group keeps its exit codes | |
| the A-1 probe | recorded in this spec | a real spawn is refused by the harness | |

### Interop Tests (Scope: protocol)
N-A: tooling, no protocol peer.

## Files to Modify
- `internal/le/hookruntime/runtime.go` - register `writeDocsCovered`, `writeDocsRead`, `bashDocRead`, the `pretool-agent` group with `agentDocsRead`
- `internal/le/hookruntime/lifecycle.go` - `hookSourceRead` appends `.md` reads to the ledger
- `internal/le/hookruntime/bash.go` - nothing beyond registration if `bashDocRead` lives in the new file
- `internal/le/ai/hooks/actions.go` - the `pretool-agent` action and its category
- `internal/le/ai/hooks/fixture_catalog.go`, `internal/le/ai/hooks/fixtures.go`, `internal/le/ai/hooks/parity.go` - fixtures and the fifth table
- `internal/le/doc/index/actions.go`, `internal/le/doc/index/register.go` - `./le doc coverage` verbs; `check` runs the baseline check
- `internal/le/spec/citation/anchors.go` - delete `declaredDesignDocument` and `isSourcePath`, call the shared producer and classifier
- `internal/le/doc/consistency/consistency.go` - `checkDesignRefs` reads the shared producer (one header parser)
- `.claude/settings.json` - `PreToolUse` `Task|Agent` -> `pretool-agent`
- `.claude/hooks/README.md` - wiring row, check rows, ledger, rollout switch
- `ai/rules/points/documentation/directives/` - a directive that the hook enforces the read; `./le ai rules condensed-update` regenerates `ai/rules/CORE.md`
- `docs/contributing/navigating-the-code.md` - `./le doc coverage` beside the index table
- `docs/contributing/spec-workflow.md` - a spec write needs its pages read
- `docs/contributing/running-commands.md` - the ledger file in "what each agent owns"
- `ai/INDEX.md` - keyword row: page read gate, doc coverage, undocumented file
- Phase 7 and 8: the pages and `// Design:` headers each unit's audit names

## Files to Create
- `internal/le/doc/index/coverage.go` - the coverage producer, the report and the baseline check
- `internal/le/doc/index/coverage_test.go`
- `internal/le/hookruntime/docsread.go` - ledger, the four checks, the messages (`// Design: .claude/hooks/README.md`)
- `internal/le/hookruntime/docsread_test.go`
- `ai/doc-coverage-baseline.txt` - generated by `./le doc coverage baseline write`, tracked, shrink-only

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | `le` tooling, no YANG |
| YANG validation constraints | N-A | no YANG |
| YANG custom validators | N-A | no YANG |
| CLI commands/flags | Yes | `./le doc coverage` verbs in `internal/le/doc/index/actions.go`; `./le ai hooks pretool-agent` in `internal/le/ai/hooks/actions.go` |
| CLI grammar (keyword before value) | Yes | `report unit <dir>`, `path <path>`, `baseline write`, per `ai/rules/cli.md` |
| Editor autocomplete | N-A | `le` verbs register through the action table |
| Functional test for new RPC/API | Yes | hook behavior fixtures, `./le ai hooks unit` |
| Pipe completeness | Yes | `./le doc coverage report \| json` |
| Env var registration | N-A | no new env var; `CLAUDE_CODE_SESSION_ID` exists |
| Doctor check for runtime dependencies | N-A | no runtime dependency of the product |
| Prometheus counters/metrics | N-A | tooling |
| BGP family surface (new SAFI / capability / attribute) | N-A | no BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | N-A | developer tooling, no operator feature |
| 2 | Config syntax changed? | N-A | none |
| 3 | CLI command added/changed? | N-A | `ze` CLI unchanged; `le` verbs documented in `docs/contributing/navigating-the-code.md` |
| 4 | API/RPC added/changed? | N-A | none |
| 5 | Plugin added/changed? | N-A | none |
| 6 | Has a user guide page? | N-A | none |
| 7 | Wire format changed? | N-A | none |
| 8 | Plugin SDK/protocol changed? | N-A | none |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | none |
| 10 | Test infrastructure changed? | Yes | `.claude/hooks/README.md` (Hook tests table: fixture and table counts) |
| 11 | Affects daemon comparison? | N-A | none |
| 12 | Internal architecture changed? | Yes | `.claude/hooks/README.md`, `docs/contributing/navigating-the-code.md`, `docs/contributing/running-commands.md`, `docs/contributing/spec-workflow.md` |
| 13 | Route metadata keys added/changed? | N-A | none |
| 14 | Prometheus counters added/changed? | N-A | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | N-A | no product registry |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | run `./le spec citation anchors spec plan/spec-docs-read-before-authoring.md` at each phase; `.claude/hooks/README.md` and `docs/contributing/navigating-the-code.md` anchor `writeedit.go`, `lifecycle.go`, `bash.go`, `runtime.go`, `codetodocs_report.go` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/contributing/navigating-the-code.md` index table; `.claude/hooks/README.md` wiring table |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** - the coverage producer skeleton, `./le doc coverage`, the `pretool-agent` action, the settings entry, empty checks registered
   - Tests: every Wiring Test row
   - Files: `coverage.go`, `docsread.go`, `runtime.go`, `actions.go` (both), `.claude/settings.json`
   - Verify: wiring tests fail on the stub verdicts; run the A-1 probe and record its result here
2. **Phase: Coverage producer** - one header parser, the path classes, the report, the baseline writer and check; `citation` and `consistency` use it
   - Tests: the `TestCoverage*` and `TestBaseline*` rows, `BenchmarkCoveragePath`
   - Verify: AC-15, AC-16, AC-17; write the first baseline; README and navigating page edited in this phase
3. **Phase: Read ledger** - `hookSourceRead` and `bashDocRead`
   - Tests: AC-1 to AC-3 rows
4. **Phase: Code-edit gates** - `writeDocsCovered`, `writeDocsRead` for code, the switch, the messages
   - Tests: AC-4 to AC-8, AC-14, AC-18, AC-21 rows; behavior fixtures
5. **Phase: Plan-write gate** - subjects from `plan/` text
   - Tests: AC-9 to AC-11, AC-13 rows
6. **Phase: Brief gate** - `agentDocsRead`
   - Tests: AC-12 row, parity table
7. **Phase: Rule and docs** - the rule point, `CORE.md` regeneration, `ai/INDEX.md`, `spec-workflow.md`, `running-commands.md`
8. **Phase: Audit A (this week)** - units in this order: `internal/component/config` (storage and boot record, with `cmd/ze/hub/config_source.go`, page `docs/architecture/storage-backends.md`); `internal/component/resolve` with `internal/component/bgp/plugins/filter_irr` and `internal/component/firewall/plugins/irr` (pages `docs/architecture/resolve.md`, `docs/architecture/bgp/filter-irr.md`); `internal/component/bgp/plugins/rib` (page `docs/architecture/rib/forward-handle.md`); `internal/component/bgp/plugins/cmd/update` with `internal/exabgp` (pages `docs/architecture/api/update-syntax.md`, `docs/architecture/exabgp-bridge.md`, `docs/guide/route-injection.md`); `internal/component/gtsm`; `internal/test/runner` (page `docs/functional-tests.md`, the stdin DUT claim)
   - Verify: AC-19, Phase proof; the 16 open rows of `plan/journal/documentation-shows-config-the-parser-refuses.md` that fall in these units are fixed or confirmed open with cause
9. **Phase: Audit B (rest of the tree)** - units in the order `./le doc coverage report` gives: hard-uncovered count, then nominal, then core-design-only; one commit and one review per unit, a batch of units per session
   - Verify: AC-20; the baseline shrinks with each commit
10. **Phase: Switch** - the baseline is empty; confirm AC-21 on the live tree

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file and symbol |
| Correctness | One `// Design:` parser in the tree after phase 2; no caller of `declaredDesignDocument` remains |
| Correctness | No check reads `ai/CODE-TO-DOCS.md` from disk |
| Correctness | An ancestor anchor never covers |
| Naming | ledger prefix `.doc-read-`, joined by `AgentFingerprint` like every per-agent file |
| Data flow | the switch reads only the baseline file; no hidden constant |
| Rule: user-facing-errors | every verdict: problem, why, command on its own line, no escape codes when color is off |
| Rule: principles | the unit list and page lists are derived; the baseline is generated and checked, never hand-edited |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Coverage report | `./le doc coverage report` prints the Measured Gap counts |
| Gates registered | `./le ai hooks unit` passes with the new fixtures |
| Brief gate live | A-1 probe recorded |
| Audit A | six Audit Record rows with SHA and review |
| Audit B | `./le doc coverage report` all zeros, baseline empty |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | the agent id and session id go through `AgentFingerprint` before they name a file |
| Path traversal | a path named in a brief or spec is cleaned and must stay under the checkout before it is stat'ed |
| Fail open | an error from the coverage producer is a verdict naming the error, never a pass (fail closed in the blocking state) |
| Resource use | text with thousands of path tokens is bounded: each distinct path is judged once |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| A-1 probe fails | Stop; report to the owner which harness event can see a spawn |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The page gate and the source gate (`writeDesignEvidence`) answer different questions: one asks whether the author read the code, the other whether the author read what the code is for. Both stay.
- Most files are "documented" only nominally: 2224 production files name a page that never mentions them. The gate can switch on long before the docs are good, which is why the audit, not the gate, decides whether the owner's first sentence is met.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Call `DocumentsByPath` and the header parser in-process | read `ai/CODE-TO-DOCS.md` | the rendering is removed after edits and drops paths; the producer is the declaration |
| Per-context ledger | session-wide ledger | matches the style guide gate; a subagent's write proves nothing about its parent's reading |
| Shrink-only tracked baseline as the switch | a constant flipped by hand; computing the tree-wide count on every hook call | the count costs a full walk per call; a constant is a second declaration of the count |
| Ancestor anchors do not cover | count them | `test/` -> `chaos-testing.md` would cover every test file |
| Audit record in this spec | a new ledger directory | no new artifact; history keeps the table after closure |

## Known Limitations
- Go written through a Bash heredoc reaches no Write/Edit hook (R-6); the commit-time ratchet still sees a new uncovered file.
- The gate proves a page was opened, not understood (R-4). Correct pages are the audit's job.

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
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior (N-A: the hook fixtures are the end-to-end path)
- [ ] Interop tests for protocol features (N-A: tooling)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/spec-docs-read-before-authoring.md` only, in the same `./le commit create` script
