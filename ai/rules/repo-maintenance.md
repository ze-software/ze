# Repository Maintenance

**When:** adding or changing a feature, tool, gate, hook, runtime dependency, or generated file, looking up which check enforces a rule, or reporting development friction
**Severity:** blocking
**Related:** rule-format, writing, evidence, testing

## Directives

- **A change that adds or changes a surface future agents use, verify, document, or avoid MUST update the discovery path in the same work.** A private implementation change requires no prose only when it meets none of these triggers:
  - it changes user or agent behavior
  - it changes an architecture contract, an invariant, or a documented data flow
  - it makes existing documentation stale
  - it adds a discoverable surface, or sets a pattern future work MUST follow
- **Every feature that adds a new runtime dependency MUST register a `ze doctor` check so agents can verify readiness before starting the daemon.**
- **A generated file MUST NOT be edited. Edit the canonical source, then sync.**
- **Project behavior rules MUST belong in `ai/rules/` and project startup guidance MUST belong in `ai/INSTRUCTIONS.md`, so Claude, Codex, and other agents all discover the same rule through generated tool-specific files.**
- **The hook-to-rule mapping MUST be consulted BEFORE writing code, to comply in advance rather than to fix after rejection. For hook false positives and workarounds, see `plan/learned/HOOK-FRICTION.md`.**
- **A recurring problem pattern, repeated surprise, stale guidance, tooling friction, or wasted effort MUST be reported immediately, and you MUST say whether a new or changed rule would prevent it.**

## Discovery Updates

### Trigger

Apply this rule when adding or changing any of these:

| Change | Why agents need it |
|--------|--------------------|
| Changed user-facing or agent-facing behavior | Agents must know the behavior exists and where users or agents configure or invoke it |
| CLI command, RPC, MCP tool, YANG command, or API contract | Agents must discover the command shape, JSON contract, and wiring |
| Architecture contract, invariant, or documented data flow | Agents must find the current contract before changing or relying on it |
| Developer tool, native action, generator, or inventory command | Agents must know the tool exists before reimplementing it |
| Self-check, verification gate, hook, lint, or doc validator | Agents must run the right check and understand failures |
| Test runner, test format, fixture pattern, or required test category | Agents must place tests in the right suite and run the right target |
| Runtime dependency or readiness condition | Agents must verify the host with `ze doctor` before starting Ze |
| Recurring trap | Agents must find its journal record first, then any rule or gate that prevents it |
| New BGP family, SAFI, or capability | Agents must update migration schema, route converter, bridge, and compat tests (`ai/patterns/bgp-family.md`) |
| RFC-level protocol behavior added, changed, or newly proven | The standards ledger drives user and design decisions; a stale RFC status misleads both |
| Existing documentation made stale by the change | Agents must not discover an obsolete claim |

**Private refactors with no new surface still trigger this rule when they change a pattern future work MUST follow.**

### Required Discovery Artifacts

Update every row that applies:

| What changed | Required update |
|--------------|-----------------|
| Changed user-facing behavior | Specific file under `docs/`, with source anchors per `ai/rules/writing.md` |
| RFC support status (protocol behavior implemented, changed, or newly proven) | The matching `docs/features/rfc-status.md` row (Status, Implemented coverage, Remaining) with a source anchor to the producing `file:line`; reconcile `docs/comparison.md` and `docs/features.md` when the support level changes |
| Changed agent-facing command or contract | `docs/features/ai-first.md`, `docs/guide/mcp/overview.md` if MCP-visible, and `ai/rules/cli.md` if workflow changes |
| Architecture contract, invariant, or documented data flow | The owning `docs/architecture/` page or flow digest, with source anchors per `ai/rules/writing.md` |
| CLI command grammar or command availability | `ai/rules/cli.md` or `ai/rules/cli.md`, plus command validation docs if needed |
| New tool or native action | `ai/INDEX.md` Dev Tools or keyword map, plus the owning `docs/contributing/` or `docs/architecture/testing/` page |
| New verification gate or hook | The "Hook-to-Rule Mapping" section below, the rule enforced by the hook, and the relevant native-action documentation |
| New doc or inventory checker | `docs/contributing/documentation-testing.md`, the owning `internal/le/<area>/actions.go`, and `ai/rules/writing.md` if policy changed |
| New test runner or format | `ai/rules/testing.md`, `ai/patterns/functional-test.md` if `.ci`, and the relevant `docs/architecture/testing/` page |
| New runtime dependency | The "Doctor Checks" section below, diagnostic code registration, and a `ze doctor` unit plus functional test |
| New registration or generated inventory | `ai/rules/evidence.md`, `ai/patterns/registration.md`, and registry-backed inventory checks |
| Existing documentation made stale by the change | Repair the stale claim in its current file and keep its source anchor valid |
| Recurring trap | `plan/journal/<class>.md` -- one row per occurrence; recurrence is the row count |
| New task category or search keyword | `ai/INDEX.md` (task navigation + keyword map) |
| Private implementation change that meets no trigger above and sets no pattern future work MUST follow | No prose update |

**An isolated rule or doc page that no existing navigation path links to MUST NOT be created. A rule that agents cannot discover is not a rule.**

### Mechanical Checklist

Before implementation is complete, answer these in the spec, review notes, or handoff:

1. **Where would an agent look first?** The `ai/INDEX.md` keyword row, the `ai/INDEX.md` task row, or both MUST be added or updated.
2. **What rule or gate prevents regression?** Name the current rule or gate when one covers the behavior. Update it when this change makes it wrong. A NEW `ai/rules/*.md` MUST wait for a recurrence that exposes a missing instruction no current rule or gate gives.
3. **What source of truth prevents drift?** A registry, generated inventory, YANG schema, or live binary output MUST be used. A static list MUST NOT be copied.
4. **What verification proves it?** The native action, unit test, functional test, hook, or doc validator that catches drift MUST be named.
5. **What docs explain usage?** The exact file and section MUST be named. Source anchors MUST be added for factual `docs/` claims.
6. **What journal record preserves the decision?** A row MUST first be appended to the matching `plan/journal/<class>.md` when a recurring trap is hit. The row is the record, never the fix: a blocking or related defect MUST still be fixed (`ai/rules/completion.md`).

### Current Discovery Surfaces

Use these before inventing a new mechanism:

| Need | Existing surface |
|------|------------------|
| Changed-file-aware wiring, doc, command, and inventory gate | `./le doc-wiring` |
| Documentation drift and YANG command contracts | `./le doc-check verify` |
| Source-to-document reverse index | `./le docs-to-code index-update`; read `ai/CODE-TO-DOCS.md` |
| RFC MUST requirement to enforcing-test coverage (which tests prove each requirement, plus the backlog) | `./le rfc index-update`. Read `rfc/requirements/<stem>.md` for one RFC's requirement to test rows. Read `ai/RFC-REQUIREMENTS.md` for the counts, the coverage rollup and the backlog over all of them. Both are generated. Coverage is gated by `./le rfc check`, staleness by `./le doc-check verify` |
| What each package does ("what does what") | `./le discovery-index update`; read `ai/PACKAGE-MAP.md` |
| Which `.go` files implement a design doc | read `ai/DOCS-TO-CODE.md` (inverse of `// Design:`) |
| Which tests enforce an RFC MUST | read `rfc/requirements/<stem>.md` after `./le rfc index-update`. `./le rfc check` and `./le doc-check verify` gate its freshness |
| The un-enrolled backlog, and how much each RFC still owes | read `ai/RFC-REQUIREMENTS.md`, the index over the per-RFC files (same generator, same gates) |
| Which problems recur | `./le journal report`; read `plan/journal/` (one file per class, row count is recurrence) |
| Whether every path the instruction corpus names still resolves | `./le doc-check links`. It is its own `./le verify current mode full` stage. The check also rejects a dead retired tool path or hook check name in hook documentation |
| Whether a `doc-links: ignore` marker states a reason, anywhere in the tree | `./le doc-check links` (`check_ignore_reasons` in `internal/le/doccheck/links.go`). The sweep is over every TRACKED file, not the walked corpus, so a marker nobody's gate reads is still audited |
| Whether every path a TRACKED file names resolves, outside the instruction corpus | `./le doc-check links` (`check_tracked_citations` in `internal/le/doccheck/links.go`). A dead path in any tracked file fails the gate. Repair the reference, or mark its line with a `doc-links: ignore` marker that states why the path cannot resolve. `vendor/` and `third_party/` are excluded because their references point into another repository, and `plan/handover/` because it records the tree as it was. `internal/le/doc_citation_baseline.txt` grandfathers the pairs that predate the check. `check_baseline_growth` compares the pairs against HEAD and refuses each pair HEAD does not hold, so that file only shrinks |
| Whether every symbol a `docs/` source anchor names is declared in the file that anchor points at | `./le doc-check verify` (`check_anchor_symbols` in `internal/le/docstocode/codetodocs.go`). It resolves the tokens after the anchor's `--` against that file's own top-level declarations, and the `report=` argument `main()` passes decides whether a finding is emitted |
| How data flows through a subsystem | read `ai/digests/<subsystem>.md` (living, hand-maintained flow digests; `ai/digests/README.md` lists them); anchors validated by `./le digest` |
| Plugin, command, YANG, and test inventory | `./le inventory`; use `./le inventory | json` for machine-readable output |
| Command inventory | `./le command-list`; use `./le command-list | json` for machine-readable output |
| Spec progress | `./le spec-status`; use `./le spec-status | json` for machine-readable output |
| Generated plugin imports | `./le plugin-imports check` |
| Whether the tree GIT HOLDS compiles | `./le repository-tracked-build check`. It runs in both full verification modes and is a structural gate in `internal/le/commit` |
| Runtime readiness | `ze doctor --json` and `ze explain <diagnostic-code>` |

**If a new feature cannot be found from one of those surfaces or from `ai/INDEX.md`, the missing discovery link MUST be added before claiming completion.**

## Doctor Checks

### The Rule

When your implementation introduces any of the following, add a registered doctor check with explicit phase, order, component, dependency, platform, diagnostic-code, and check-function metadata. Ownership is part of the requirement: the package, component, or plugin that owns the dependency MUST own the registration, check function, and unit test.

- **`internal/component/doctor` owns the runner, output contract, functional coverage through the user entry point, and checks that have no narrower owner.**
- **New runtime dependency checks MUST NOT be added by appending another direct call to the central `runChecks` list.**
- **Owner-specific registrations MUST NOT be added in `internal/component/doctor` just because the runner lives there.**
- **Internal plugins (preferred path) MUST declare doctor checks in `registry.Registration.DoctorChecks`.** The doctor runner bridges these at execution time via `checks_plugin_registry.go`. The check function uses `registry.DoctorCheckContext` (Tree and Platform as `any`) and returns `[]rpc.DoctorCheckDiagnostic`. Component is set automatically from the plugin name. See `l2tpauthradius/register.go` for the reference example.
- **Components that are not plugins** (e.g., appliance, web, SSH) MUST use `diagnostic.RegisterDoctorCheck()` from the owning package's init().

| New dependency | Doctor check needed |
|----------------|---------------------|
| Config leaf that references a file path (cert, key, binary) | File existence check |
| Config leaf that names an external service or socket | Reachability probe |
| Kernel module requirement | `/proc/modules` check (Linux) |
| New listen address/port | Port bind probe |
| New UDP listener | UDP `ListenPacket` bind probe |
| New service with TLS | Certificate validity + expiry check |
| Embedded certificate material | Parse certificate and check validity window |
| External binary (plugin, helper) | `exec.LookPath` or `os.Stat` check |
| Procfs/sysctl dependency | Read/write probe for the exact `/proc` path |
| Netlink dependency | Open the specific netlink family/handle |

### Diagnostic Code Convention

- **All doctor codes MUST use the `doctor-` prefix: `doctor-<component>-<condition>`.**
- **Every new code MUST be registered in `internal/core/diagnostic/codes.go` with title, description, and examples. The code MUST be explainable via `ze explain`.**

### Mechanical Check

- **After implementation, the check MUST be verified as registered and explainable: `go test ./internal/component/doctor -run 'TestDoctorCoverageCodesRegistered|TestRunChecksExecutesRegisteredPluginCheck'`**
- **If you added a runtime dependency and no registered doctor check declares its `doctor-*` code, you missed the readiness check or its diagnostic metadata.**

### Where to Register Checks

| Dependency owner | Registration mechanism |
|------------------|-----------------------|
| Internal plugin (registered via `registry.Register`) | `Registration.DoctorChecks` field; bridge runs at doctor execution time |
| Web, MCP, looking-glass, or other listener component | `diagnostic.RegisterDoctorCheck()` from owning component |
| SSH host-key dependency | `diagnostic.RegisterDoctorCheck()` from SSH component |
| Interface backend | `diagnostic.RegisterDoctorCheck()` from backend owner |
| Kernel module, procfs, sysctl, netlink, VPP, or platform-specific backend | `diagnostic.RegisterDoctorCheck()` from owning backend/component, with build-tagged files where needed |
| Blob storage, platform detection, generic runner state, or dependency with no narrower owner | `internal/component/doctor`, with a comment or test name making the lack of owner explicit |

**If no plugin, component, backend, or command package owns the dependency, the check and unit test MUST stay in `internal/component/doctor`. An owner package MUST NOT be invented just to satisfy proximity.**

### Test Requirement

Every new doctor check needs both:

| Test type | What it proves | Location |
|-----------|----------------|----------|
| Unit test | The check fires only when the relevant config block is present and emits the registered code | Owning package next to the registration, or `internal/component/doctor` only when there is no narrower owner |
| Functional test | `ze doctor --json <config>` exposes the behavior through the user entry point | `internal/component/doctor` or the existing functional test suite for the user entry |

**Linux-only checks MUST still have Linux-tagged tests, and the package MUST be covered by the QEMU integration target when new `//go:build linux` code is added.**

## Canonical Sources and Sync Direction

### Sync Flows

| Canonical source | Generates | Sync command |
|------------------|-----------|--------------|
| `ai/INSTRUCTIONS.md` | `CLAUDE.md`, `AGENTS.md` | `./le ai skills-sync` |
| `ai/skills/*.md` | `.claude/skills/*/SKILL.md`, `.codex/skills/*/SKILL.md`, `.agents/skills/*/SKILL.md` | `./le ai skills-sync` |
| `ai/rules/points/<rule>/` | `ai/rules/<rule>.md` | `./le rules render-update` |

### Rule Placement

- **Project-wide behavior rules, workflow rules, and agent rules MUST live under `ai/rules/`, not under a tool-specific home directory such as `~/.claude/rules/`.**
- **Tool-specific files are only for behavior that applies exclusively to that tool outside this repository.**
- **`ai/rules/*.md` are tool-agnostic and RENDERED from `ai/rules/points/<rule>/`. It MUST NOT be edited by hand. Edit the point file that carries the instruction, or the manifest that carries the title, the trigger and the reading order. Then run `./le rules render-update`. `.claude/rules/*.md` are Claude-specific originals and MUST NOT be used for shared Ze project behavior. These two directories are independent; neither generates the other.**
- **One instruction is one file, and its PATH is its id.** `ai/rules/points/<rule>/<slug>.md` holds one block of the rule, verbatim, behind a small frontmatter header. `ai/rules/points/<rule>/manifest.md` holds the rule's title, its `**When:**` trigger, its severity, and the ordered slug list the renderer concatenates. A point on disk that the manifest does not list is a hard render error, never a silent drop.
- **Second generation:** `ai/rules/INDEX.md` is generated by `internal/le/rules/index.go` from the RENDERED rule files' headings and summary lines. It MUST NOT be edited by hand; run `./le rules index-update`. To change a rule's one-line overview, edit the `when:` field in that rule's manifest, run `./le rules render-update`, then regenerate.
- **Second generation:** `internal/le/rules/artifacts.go` generates TWO artifacts from one parse of the RENDERED rule files. They MUST NOT be edited by hand; run `./le rules condensed-update`. To change what they contain, edit the rule's points, run `./le rules render-update`, then regenerate.

| Artifact | Holds | Imported into every session? |
|----------|-------|------------------------------|
| `ai/rules/TRIGGERS.md` | one routing line per rule: path, severity, `**When:**` trigger. Every rule, so none is ever invisible. The generator prints the count; do not copy it here | yes |
| `ai/rules/CORE.md` | the condensed directives of the always-on rules. Membership is DERIVED (rungs 1 and 2 of the ladder in `ai/rules/rule-precedence.md`, the ladder itself, any rule with no routable trigger, and any blocking rule no past task description would surface) | yes |

**Membership in `CORE.md` MUST NOT be edited, because it is never written down.** To make a rule always-on, change what the derivation reads: name it on rung 1 or 2 of the ladder in `ai/rules/rule-precedence.md`. A list of filenames in the generator would read identically until the ladder changed underneath it (`ai/rules/evidence.md`).

### Mechanical Check

**Before editing any file listed in the "Generates" column above, STOP. You MUST find its canonical source in the left column and edit that instead.**

**Every native write action derives its output from the WORKING TREE, so in a shared checkout it can pick up other sessions' uncommitted sources. You MUST diff a regenerated artifact before you name it in a commit.** The bare `./le <area>` listing marks each write action explicitly.

The output is correct for the tree it read. It is wrong for the commit you are about to make, because that commit does not carry the sources the regeneration saw. What lands is a derived file that describes code nobody can see.

`internal/le/commit` refuses a commit whose regenerated artifact was derived from a tree holding sources the commit does not carry. That refusal is the only thing that catches this.

**The safe regeneration is HEAD plus your own files.** When an artifact is fully generated and yours was the only edit, `git show HEAD:<path>` written back over it restores the committed state, and the gate then agrees.

**The mirror image is worse and no gate catches it: committing a document that DESCRIBES uncommitted code.** A committed document that names a symbol still sitting in the working tree reddens `./le doc-check links` for every session until that code lands. A check that you have not swept somebody's work IN does not check the other direction: prose you committed about work still sitting OUT.

### Drift Detection

**The `CLAUDE.md`, `AGENTS.md`, and skill mirrors are gitignored, so `git diff` can NEVER show drift for them.** `./le ai sync-check` compares them against a fresh generation; the session-start hook warns `generated agent files are stale` when a resync is needed. Fix them with `./le ai skills-sync`. `ai/rules/<rule>.md` is the one generated rule surface that IS tracked, so `git diff` shows its drift, and `./le rules render-check` reaches the same verdict without writing.

### Banned Actions

| Action | Fix |
|--------|-----|
| Editing `CLAUDE.md` directly | Edit `ai/INSTRUCTIONS.md`, run `./le ai skills-sync` |
| Editing `AGENTS.md` directly | Edit `ai/INSTRUCTIONS.md`, run `./le ai skills-sync` |
| Editing `.claude/skills/*/SKILL.md` directly | Edit `ai/skills/*.md`, run `./le ai skills-sync` |
| Editing `.codex/skills/*/SKILL.md` directly | Edit `ai/skills/*.md`, run `./le ai skills-sync` |
| Editing `.agents/skills/*/SKILL.md` directly | Edit `ai/skills/*.md`, run `./le ai skills-sync` |
| Editing `ai/rules/<rule>.md` directly | Edit the point under `ai/rules/points/<rule>/`, run `./le rules render-update` |
| Editing `ai/rules/INDEX.md` directly | Edit the rule's point or manifest, run `./le rules render-update`, then `./le rules index-update` |
| Editing `ai/rules/TRIGGERS.md` or `ai/rules/CORE.md` directly | Edit the rule's point or manifest, run `./le rules render-update`, then `./le rules condensed-update` |

## Hook-to-Rule Mapping

Quick reference: which checks enforce which rules, and when they trigger.

### Architecture: checks live in native Go hookruntime

The consolidated checks run inside `internal/le/hookruntime`, and `nativeHookActions` is the registry that connects each trigger to its Go functions. A tool call pays one native process, and the checks below are functions rather than separate scripts.

| Go source | Runs on | Contains |
|---|---|---|
| `internal/le/hookruntime/bash.go` | PreToolUse `Bash` | every registered Bash check below |
| `internal/le/hookruntime/writeedit.go` | PreToolUse `Write\|Edit\|MultiEdit\|NotebookEdit` | every registered Write/Edit check below |
| `internal/le/hookruntime/agent.go` | PreToolUse `Task\|Agent` | skill routing, review-model enforcement, and the Go style-guide reminder |
| `internal/le/hookruntime/postwrite.go` | PostToolUse `Write\|Edit` | formatting and post-edit advisory checks |

The session and lifecycle actions remain separate from the four registered check groups because they return hook protocol output directly rather than a check verdict. They still run in the native process through `runLifecycleHook`; `nativeHookActions` owns the Bash, Write/Edit, post-write, and Task/Agent check rosters.

**Changing a check:** the Go function in `internal/le/hookruntime` MUST be edited, and its entry in `nativeHookActions`, `// ze point:` binding, and published row MUST stay in agreement. `./le hook-check unit` MUST run afterwards. An intentional fixture change MUST update the owned native golden in the same change, and the "Discovery Updates" section above MUST also be satisfied.

**Reads never block.** `hookSourceRead` and the LSP lifecycle action in `internal/le/hookruntime/lifecycle.go` write non-blocking, session-scoped evidence markers. `writeDesignEvidence` in `internal/le/hookruntime/writeedit.go` consumes those markers before a design or spec write. A Read MUST return implementation content to count; an empty response, failed read, or a window under the native depth threshold records nothing.

**Every marker is keyed by session ID**, and every native hook consumer MUST use
the resolver in `internal/le/hookruntime/session.go`.

Session-start and subagent-context actions read hook JSON and validate the raw
session string before they publish an ID or derive a state path. An absent ID
and an invalid ID are distinct results. Invalid IDs are rejected, never
rewritten, and dot entries are forbidden.

The hook MUST NOT persist `$ZE_SESSION_ID`. Native session and spec lifecycle
commands resolve the current harness session themselves. `./le hook-check
session-id` locks this behavior.

### PreToolUse Checks (block before the tool runs)

#### LSP gate (`hookUntilLSP` in `internal/le/hookruntime/lifecycle.go`)

Enforces `session-start.md`. Triggers on `Bash|Write|Edit|MultiEdit|NotebookEdit|ToolSearch|Task|Agent`.
Blocks those tools until `ToolSearch query="select:LSP"` has run this session. BLOCKING. <!-- severity-note: the LSP gate's severity, not this reference page's -->

#### Bash (`internal/le/hookruntime/bash.go`)

| Check | Enforces | Triggers on | What it does |
|---|---|---|---|
| `bashWorktreeCopy` | `ai/INSTRUCTIONS.md` prohibition, no rule point | Bash | Blocks copies, moves, installs, rsync, and redirects from `.claude/worktrees`. BLOCKING. |
| `bashDestructiveGit` | `git-safety.md` | Bash | Blocks destructive and repository-changing git verbs. `git restore --staged` remains permitted. BLOCKING. |
| `bashRootBuild` | build hygiene, no rule point | Bash | Blocks `go build` that would place a binary at the repository root, while explicit session or `bin/` outputs pass. BLOCKING. |
| `bashLossyPipe` | `commands.md` | Bash | Blocks a lossy filter after an expensive command and directs the output to a log. BLOCKING. |
| `bashRawHeavy` | `commands.md` | Bash | Blocks raw Go tests, lint analysis, and functional runners outside `./le job run`. BLOCKING. |
| `bashPollLoop` | `commands.md` | Bash | Blocks an unbounded `while` or `until` polling loop with sleep or pgrep. BLOCKING. |
| `bashSystemTmp` | `testing.md` | Bash | Blocks access to `/tmp` and names the session scratch action. BLOCKING. |
| `bashScratch` | `commands.md` | Bash | Blocks ad-hoc writes at the project `tmp/` root while permitting owned subdirectories and governed shared names. BLOCKING. |
| `bashTestDeletion` | `testing.md` | Bash | Blocks deletion or checkout of test files outside `test/draft/`. BLOCKING. |
| `bashGovernedWrite` | `commands.md` | Bash | Blocks shell writes under `plan/` or `ai/rules/`, where the native Write/Edit checks own the policy. BLOCKING. |

The five commit-time gates (spec-audit, deferral-in-diff, deferral-unassigned, wiring-at-commit, doc-drift) belong in **creation-time gates in `internal/le/commit`** because the sanctioned commit path does not send the literal `git commit` string to this hook. See "Commit-time gates" below.

#### Write/Edit (`internal/le/hookruntime/writeedit.go`)

| Check | Enforces | Triggers on | What it does |
|---|---|---|---|
| `writeLineCitation` | `evidence.md`, `writing.md` | governed prose | Blocks source line-number citations outside quoted code and generated files. BLOCKING. |
| `writeGenerated` | `repo-maintenance.md` | generated root instructions and tool-specific `.claude/` content | Blocks writes to generated root instructions and warns when shared agent guidance is written into a tool-specific tree. |
| `writeRenderedRule` | `repo-maintenance.md` | rendered files directly under `ai/rules/` | Blocks edits to rendered rules and directs the author to the canonical point source. BLOCKING. |
| `writePointOverwrite` | `never-destroy-work.md` | Write or replacement MultiEdit on an existing rule point | Blocks replacement of an existing canonical point file. BLOCKING. |
| `writePointLanguage` | `rule-format.md` | canonical directive points | Blocks lowercase obligation words and a new directive with no RFC 2119 level. BLOCKING. |
| `writeDesignEvidence` | `evidence.md` | design and spec files | Blocks a design or spec write until this session has read producing source or invoked LSP. BLOCKING. |
| `writeSpecStatus` | `planning.md` | source Go edits | Blocks implementation while the selected spec has the wrong lifecycle status. BLOCKING. |
| `writeGoPatterns` | `architecture.md`, `cli.md`, `go-standards.md`, `performance.md`, `quality.md`, `plugins.md`, `goroutine-lifecycle.md` | production Go writes and edits | Applies the native forbidden-pattern checks for handlers, panic, legacy logging, allocating formatting, nolint, init registration, switch dispatch, anonymous goroutines, and fake buffer handles. |
| `writeFilePatterns` | `architecture.md`, `commands.md`, `config.md`, `quality.md`, `testing.md` | writes and edits selected by path or file type | Applies native path, package-name, scratch, lint-exclusion, config-version, and CI observer checks. |
| `writeWeakening` | `testing.md` | tests and test-harness evidence | Runs the native weakening analyser and blocks unauthorised evidence changes. BLOCKING. |
| `writeCISleep` | `testing.md` | `test/**/*.ci` | Blocks `time.sleep(` without a recognised justification marker. BLOCKING. |

> `writeGoPatterns` is the live edit-time allocation-pattern check. Its registered
> function blocks `fmt.Sprintf`, `fmt.Fprintf`, `fmt.Printf`, and
> `strconv.FormatInt` or `strconv.FormatUint` in production Go. The broader
> allocation audit stays with its native verification action rather than an
> undocumented hook branch.

#### Task/Agent (`internal/le/hookruntime/agent.go`)

| Check | Enforces | Triggers on | What it does |
|---|---|---|---|
| `agentReviewModel` | `planning.md` | Task/Agent review prompts | Blocks a review agent spawned off Opus 5 and reports the accepted model evidence. BLOCKING. |
| `agentSkill` | `cli.md` | Task/Agent prompts covered by a ze skill | Blocks a raw agent spawn when a named skill owns the workflow. BLOCKING. |
| `agentStyleGuide` | `go-standards.md` | Task/Agent briefs that will produce Go | Warns when the brief does not name `docs/contributing/ze-go-style.md`. Advisory. |

#### Post Write/Edit (`internal/le/hookruntime/postwrite.go`)

| Check | Enforces | Triggers on | What it does |
|---|---|---|---|
| `postFormatGo` | `quality.md` | existing Go Write/Edit | Runs gofmt, goimports, and changed-code lint, then blocks on reported lint issues. |
| `postFileSize` | Go style guidance, no rule point | production Go | Warns when a source file exceeds 1,000 lines. Advisory. |
| `postDeferral` | heuristic advisory, no rule point | governed Markdown | Warns when an edit adds deferral language outside the accepted ledgers. Advisory. |
| `postJournal` | journal format, no rule point | `plan/journal/*.md` | Runs the native journal row validator. Advisory and fail-speak. |
| `postRFCHeader` | Go style guidance, no rule point | production Go | Suggests an RFC design header when a source repeatedly references RFCs. Advisory. |
| `postTestDocs` | Go style guidance, no rule point | Go tests | Warns when a test file has tests and no VALIDATES or PREVENTS header. Advisory. |
| `postFuzz` | advisory, no rule point | wire parsing Go | Warns when a package with a Parse function has no fuzz test. Advisory. |
| `postVague` | Go style guidance, no rule point | production Go | Warns on the configured vague variable-name forms. Advisory. |
| `postBoundary` | advisory, no rule point | production Go with numeric validation | Warns when the companion test file has no boundary evidence. Advisory. |

`hookValidateSpec` in `internal/le/hookruntime/lifecycle.go` validates the Wiring
Test table and returns the hook protocol verdict. `runLifecycleHook` dispatches
the `validate-spec` action, and `./le hook-check unit` covers its accepted and
refused forms.

`./le verify worktree` separately runs the native `./le doc-wiring` action. `internal/le/docwiring` owns the wiring and documentation drift checks.

### Changed-file gates inside `./le doc-wiring`

These are native `./le` actions rather than Claude hooks. All are changed-file scoped: a session owns the files it touches, not the whole tree.

| Check | Enforces | Triggers on | What it does |
|---|---|---|---|
| `check_ci_sleep_ratchet` | `testing.md` | changed `test/**/*.ci` | Caps how MANY `time.sleep(` calls exist tree-wide against a committed delta baseline. BLOCKING. |
| `check_ci_sleep_justification` | `testing.md` | changed `test/**/*.ci` | Caps how many sleeps are UNEXPLAINED: each needs a comment above or trailing it. BLOCKING. |
| `check_known_failure_load_excuses` | `completion.md` | changed `plan/known-failures/*.md` | Rejects a shard blaming host load ("under load", "loaded host", "load average", "load-sensitive", "passes in isolation", "resource contention", "contended host"). `README.md` / `RESOLVED.md` exempt. BLOCKING. |
| `check_ci_log_subsystem_keys` | `testing.md`, `config.md` | changed `test/**/*.ci` | Rejects a `ze.log.<subsystem>` key whose subsystem contains a hyphen that is not declared literally in Go. An internal plugin's logger name is `CanonicalSubsystemName` of its registry name (every hyphen becomes a dot) and `getLogEnv` splits on `.` only, so `ze.log.bgp.adj-rib-in` sets nothing and the level silently stays at the WARN default. Scan is tree-wide; `#` comment lines exempt. BLOCKING. |

`./le verify worktree` separately runs `./le repository tree-check`, which executes the three tree-wide checks in `internal/le/repository`. The other two checks are changed-file scoped; `./le repository check` runs all five over the current tree.

| Check | Enforces | Triggers on | What it does |
|---|---|---|---|
| `check_source_anchor_line_numbers` | `writing.md` | every `docs/**/*.md` | Rejects a `<!-- source: -->` anchor that carries a line number, because line numbers rot. Gated: `./le repository tree-check`. BLOCKING. |
| `check_source_anchor_stale_paths` | `evidence.md` | every `docs/**/*.md` | Rejects a repo-relative anchor path that no file or directory answers. It resolves ANY root, including anchors outside the `PATH_PREFIX` roots that `internal/le/docstocode/codetodocs.go` walks. Gated: `./le repository tree-check`. BLOCKING. |
| `check_spec_ac_completeness` | `completion.md`, `planning.md` | every `plan/spec-*.md` whose Status is `in-progress` | Rejects an acceptance-criterion row whose `Demonstrated By` cell is empty, so an in-flight spec cannot claim an AC with no named evidence. Gated: `./le repository tree-check`. BLOCKING. |
| `check_cross_package_wiring` | `completion.md` | `git diff HEAD` plus untracked `.go` files under `internal/` or `cmd/` | Reports an exported symbol with no cross-package non-test caller. UNENFORCED: no gate runs it (owner decision, 2026-08-09). Run `./le repository check` by hand. |
| `check_cli_handler_coverage` | `testing.md` | the same changed-file list, `.go` files under the CLI paths only | Reports a newly registered command that no `.ci` test names. UNENFORCED: no gate runs it (owner decision, 2026-08-09). Run `./le repository check` by hand. |

The two changed-file checks take `changed_files` as their subject, which is `git diff HEAD` plus untracked files. Several sessions share this checkout, so that list includes other sessions' half-written work. Both checks demand completeness that a file in the middle of an edit cannot show. They therefore stay out of the gate.

### Prose gate (ASD-STE100)

Native `./le ste` actions and the commit-time gate own this check. HEAD is the baseline and the comparison is per file, so a document nobody touched can never fail.

| Check | Enforces | Triggers on | What it does |
|---|---|---|---|
| `steProblems` (`internal/le/commit`) | `writing.md` | `./le commit create` with any `.md`, `.go`, or `.yang` in the commit | Calls the native `internal/le/ste` ratchet over the commit's files and prints the six ASD-STE100 habits that grew against HEAD. Advisory: STE is a guideline, so this never refuses a commit. |
| `./le ste check` | `writing.md` | on demand, before you prepare a commit | The same comparison over every changed file in the working tree. It prints the file, the habit, and only the new findings. Surfaces are Markdown in `docs/`, `ai/`, `plan/`, and the root, prose comments in `.go`, and `description` strings in `.yang`. Renames follow `-M`, so a moved legacy document does not report its inherited content as new. Deliberately NOT in `./le doc-check verify`. Whole-tree report: `./le ste review`. |

### Commit-time gates (`internal/le/commit`)

These are NOT Claude hooks. They run when `./le commit create` generates the commit script, which is the only sanctioned commit path (run the script at the path its `script=` line prints). The helper already knows the exact add/remove set of the commit, so the gates inspect that instead of the staging area. BLOCK gates raise (exit 2, no script written); WARN gates print to stderr and let the script be written.

| Gate | Enforces | Severity | What it does |
|---|---|---|---|
| go-design-ref (`goDesignRefProblems`) | `go-standards.md` | BLOCK | Refuses a commit whose non-exempt `.go` files carry no `// Design:` header. `writeGoPatterns` checks edit proposals, while this gate checks the exact tree that the commit will produce. |
| test-coverage (`testCoverageProblems`) | `testing.md` | BLOCK | Refuses a commit that carries non-exempt production Go with no test path, unless `./le commit create` receives the explicit no-test reason accepted by the native commit parser. |
| verify-status / structural-gate | `precommit-verify.md` | BLOCK | Refuses a script over a non-green `./le verify current mode full` (structural reds are unbypassable). |
| ste (`steProblems`) | `writing.md` | WARN | Calls `internal/le/ste.Ratchet` over the commit's own `.md`, `.go`, and `.yang` files and prints only habits that grew against HEAD. Legacy prose in untouched files is outside the population. |
| discovery-index | "Discovery Updates" above | BLOCK | Refuses when a generated index (PACKAGE-MAP) would be left incoherent. Judged on the tree the COMMIT PRODUCES (HEAD + adds - removes), materialized under `tmp/commit-view-*` and checked with the commit's OWN generators via `--root`, never on the working tree: a concurrent session's uncommitted sources must neither block your commit nor be swept into your index. **Every** index whose generator exists is verified, not just the ones the commit visibly feeds: `package_map` keys its rows on directory existence, so a new `.go` can drift PACKAGE-MAP without carrying a `// Package` header at all. Cost: the view is built on EVERY commit the gate examines, not only ones touching an index source, because the candidate set comes from generator existence, about 5.5s total (~2s working-tree freshness, ~3.6s to materialize and check). If the view cannot be built it does NOT fail closed: it warns on stderr and falls back to the working-tree verdict, which is the only evidence left. BLOCKING. |
| rfc-changed (`rfcChangedProblems`) | `testing.md` | BLOCK | Refuses a commit that changes an RFC-tagged test and carries no matching row in `test/rfc-changed.md`, or carries a stale row. `internal/le/weakened` supplies the shared changed-unit population used by edit-time and commit-time checks. |
| deferral-unassigned | `planning.md` | BLOCK | Folds over every shard in `plan/deferrals/` and flags an open row with no Destination. |
| deferral-in-diff | `planning.md` | BLOCK | Blocks when the commit's added lines contain deferral language and no `plan/deferrals/` shard is part of the commit (diff computed in a throwaway git index). |
| review-rounds (`ROUND_CAP`, `cmd_record` in `internal/le/speclifecycle/review.go`) | `planning.md` | BLOCK | Runs at `./le spec-session review record`, not at commit time, and is listed here beside the `./le spec-session review check` half that commit_helper runs. Refuses an artifact claiming more than three review rounds unless `--rounds-reason` names the PRODUCT defect a later round found, and refuses fewer than one. The count is written into the artifact header as `rounds=N`, so a closure's review cost is auditable after the fact. It does NOT cap rounds that find real defects: it prices the fourth at one sentence, which is the sentence a loop auditing its own bookkeeping cannot write. |
| journal-row (`journal_row_problems`) | `planning.md` | BLOCK | Refuses a commit that ADDS a `plan/journal/<class>.md` row not holding the five cells `\| Date \| Spec \| Surface \| Symptom \| Fix \|`. Not cosmetic: `_journal_added_spec_stems` reads the same rows for the closure stem, so a row the parser cannot read yields no stem, `spec_closure_stem` returns None, and the review gate stops firing on the commit carrying the code. `./le journal report` cannot cover it, because that reads HEAD and the bad row is only visible before the commit lands. |
| spec-audit | `planning.md` | BLOCK | Blocks the closure commit (the one ADDING a `plan/journal/*.md` row whose `Spec` cell names the spec, or `plan/learned/NNN-<stem>.md`) when that spec's `## Pre-Commit Verification` section is unfilled. ADDING is load-bearing: the row must be new to this commit, which is what `_journal_added_spec_stems` answers, so a class file's months-old first row is not read as today's closure. Keyed to `./le spec-session current`; no claim → skips. |
| wiring-at-commit | `completion.md` | WARN | Warns when `internal/plugins/**/*.go` is committed with no `.ci`. |
| doc-drift | `writing.md` | WARN | Runs `internal/le/docvalid/drift.go`; warns on drift. |

### Hook tests (`./le hook-check unit`)

| Runner | Covers |
|---|---|
| `internal/le/hookcheck/parity.go` | Golden exit-code regression for the four native hook check groups registered by `nativeHookActions`. An intentional fixture change updates the native golden with the owned bless path. |
| `./le hook-check unit` | Native hookruntime fixtures that need isolated repositories or lifecycle state, including spec validation, source-read evidence, commit-time gates, delegation, and the registered write and Bash checks. |

### Session Lifecycle Hooks

**UserPromptSubmit stdout reaches the model. UserPromptSubmit stderr does not.** A reminder that MUST land in the context writes to stdout. A banner that MUST cost no context tokens writes to stderr, as the native lifecycle actions in `internal/le/hookruntime/lifecycle.go` do. The two stdout reminders below fire on every turn, so each one stays a single line.

| Native kind | Event | What it does |
|---|---|---|
| `session-start` | SessionStart | Validates the raw session ID, publishes the accepted ID, and prints status. Deletes nothing; `./le session reap` owns proof-based cleanup. |
| `compaction-reminder` | UserPromptSubmit | Detects compaction and reminds the session to read the post-compaction rule. |
| `verify-claim-reminder` | UserPromptSubmit | Reminds the session to read the producing function before making a code claim. |
| `delegation-reminder` | UserPromptSubmit | States that requested parallel delegation needs no permission. |
| `block-premature-stop` | Stop | Runs the native stop-phrase and spec-closure checks. Blocking. |
| `session-end-summary` | Stop | Calls `./le session end-summary`; preserves handoffs and never releases a spec claim. |
| `session-end-deferrals` | Stop | Prints the open deferral count. Advisory. |
| `pre-compact-save` | PreCompact | Saves session state before compaction. |
| `subagent-context` | SubagentStart | Validates the parent session ID and emits the parent context through the hook protocol. |

### Pre-Flight Checklist by File Type

(All checks below now run inside the dispatchers; behaviour is unchanged.)

#### Any `.go` file under `internal/`

pre-write-go, auto-lint, encoding-alloc (wire paths), sprintf-new, legacy-log, panic-error, ignored-errors, silent-ignore, temp-debug, os-exit, init-register, yagni-violations, json-kebab, require-design-ref, require-related-refs, file-size.
Hot-path files (reactor/event/dispatch/hub/wire/message) also: goroutine-lifecycle, fake-bufhandle.

#### Test files (`_test.go`, `.ci`)

`writeWeakening` (when removing or weakening tests), `postTestDocs` (for a new Go test), `postBoundary`, and the `.ci` observer checks in `writeFilePatterns`.

#### Spec files (`plan/spec-*.md`)

validate-spec, design-without-lsp (needs recent investigation of EVERY source kind the spec's own Files to Modify and Files to Create name, where the kind is the file's extension: for Go, LSP invoked or a `.go` read; for a tooling, hooks, YANG or build subject, that file read, and read more than a 20-line window of it, or the whole of it -- a Read that showed nothing, such as a second whole Read the harness answers with `file_unchanged`, records nothing), require-docs-read (if new), source-edit-spec-not-in-progress (if spec not `in-progress`), design-doc-owner (past `skeleton`: every `// Design:` document declared by a file the spec names must itself be named in the spec).

design-without-lsp and design-doc-owner are the two halves of one question and neither substitutes for the other: the first says the CODE was read, the second says the code's own DESIGN DOCUMENT was accounted for.

#### Commits

A Bash `git commit` is blocked outright by destructive-git. Commit via `./le commit create`, then `bash` on the path its `script=` line prints; the creation-time gates (verify-status, discovery-index, deferral-unassigned, deferral-in-diff, journal-row, spec-audit block; wiring-at-commit, doc-drift warn) run then. See "Commit-time gates" above.

## Gate Population

**A gate's green is only as wide as the population it reads. Before you trust
one, you MUST know which files it opens. A gate whose NAME promises a
population wider than it reads produces a green that answers a question nobody
asked, and it will be over-trusted by whoever is tired.**

**Where a gate is named, you MUST state what it cannot see, not only what it
checks. A rule that says "also run the other check" is followed on the day
somebody remembers it. A rule that names the blind spot is followed by whoever
reads the gate and wonders what its green covers.**

**A check that reads another artifact's STRUCTURE MUST anchor on a marker that
artifact guarantees, never on a position inside it, and MUST resolve the
indirection that artifact's own format permits. A positional window stops seeing
data the moment the data moves past it. A reader blind to indirection reports
"not wired" for a subject that is wired. Both failures present as a verdict
about the subject when they are a verdict about the reader, which is why neither
is caught by re-running the check.**

**The agreement MUST be pinned by feeding a real, canonical instance of that
artifact through the reader in a test.** A reader and the artifact it parses
drift apart silently whenever nothing exercises the two together, and the drift
surfaces as a wrong answer rather than as an error. Where the read can fail
outright, the failure MUST stay distinguishable from a value that is legitimately
absent (`ai/rules/evidence.md`, "Fail-Closed Guards").

**When a second gate reuses a check another gate already runs, it MUST supply
that check the same INPUT SHAPE the first gate supplies, and the shared shape
MUST be stated where the check is defined.** Sharing one implementation is what
keeps two gates from disagreeing about a rule. It does not keep them from
disagreeing about the SUBJECT, because a check reads its subject through the
values its caller passes. A caller that builds those values differently gets
different answers out of identical code, and the sharing hides it: both gates
cite the same function, so the difference looks impossible.

**The failure MUST be treated as blocking rather than cosmetic, because a
later gate that refuses what an earlier gate allowed leaves no way forward.**
The earlier gate has already passed, its verdict cannot be revisited, and the
later refusal names a rule the author did not break. An exemption a check grants
is part of its contract exactly as much as the violation it reports, so an input
shape that silently voids an exemption converts a permitted subject into a
blocked one.

**A shared check MUST be exercised through the NEW caller, with the values that
caller really constructs.** A test that calls the check directly, or that
rebuilds its input by hand, proves the check and not the wiring. Reusing a
subject the check is known to exempt is what makes the test discriminate: an
input shape that agrees with the original caller reports the exemption, and one
that does not reports the violation.

**Where the shape is derived rather than passed through, the derivation MUST NOT
import context from outside the artifact under test.** Widening a value until
the check accepts it can make the check depend on the environment the tool runs
in, which turns one gate's verdict into a property of the machine.

**`./le repository-tracked-build check` is the only gate that compiles what git holds,
and it compiles no `_test.go`. Its green therefore says nothing about the test
build. Before you treat work as committable, you MUST also compile the test
binaries of every package you touched, without running them.**

**A gate that refuses a COMMIT MUST derive its verdict from the paths that
commit names, never from the state of the working tree or of a file on disk. A
gate that reads the repository answers a question nobody asked: it refuses work
that is correct, for a fact about somebody else's work, and it does so in a
checkout several sessions share.**

The failure is not a false positive about the subject. It is a verdict about the
wrong subject, and the author it blocks usually has no permitted action: the
offending state is not theirs to commit, not theirs to delete, and not theirs to
carry.

Two readings of the same kind of ledger, measured on 2026-08-24, show the whole
distinction:

| Reader | Derives from | Effect |
|--------|--------------|--------|
| `check_weakened_tests` | recomputes the weakenings of the paths the commit NAMES | refuses only a commit that actually weakens a test |
| `rfc_changed_problems` | reads `test/rfc-changed.md` from DISK | one open row refuses every commit in the repository until its author lands it |

Both files are per-commit by their own contract. Only the second turns that
contract into a repository-wide lock. On the day it was measured it held a
227-path change hostage to a five-line assertion in another session's package,
and the blocked author's three available moves were all forbidden: adding a
foreign hunk, deleting an owner-approved row, or waiting.

**The same defect wears a second face: a gate that infers INTENT from a
by-product instead of reading the act.** `spec_audit_problems`
(`internal/le/commit`) asks whether a journal row names the claimed
spec, and treats that as the spec's closure. A row naming a spec is mandated for
every defect an agent finds, and an agent finds most of them inside its own
spec, so the ordinary mid-spec commit is read as a closure and refused for
lacking a closure section. The act itself is in the same function's arguments:
`remove_paths` already says whether the commit removes `plan/<spec>.md`, and a
commit that adds no learned summary and removes no spec closes nothing.

**Before adding or changing a commit gate, its doc comment MUST answer one
question: what does this refuse that the commit did not do?** A gate that cannot
answer it is reading the world in place of the diff.

Three consequences follow, and each MUST hold:

- **A shared per-commit ledger MUST be keyed to the commit that carries it.** A
  row about file A MUST NOT refuse a commit touching only file B.
- **An escape hatch MUST reach the gate that fires.** `--review-override`
  clears `review_gate_problems` and `spec_audit_problems` fires afterwards, so
  the documented escape does not reach the refusal an author actually meets.
  A gate with no reachable escape and no permitted action is a stop, not a gate.
- **A gate MUST NOT be satisfiable only by a statement its author knows to be
  false.** Where the routes past a refusal are gaming it (releasing a spec claim
  so the gate returns early) or asserting something untrue (filling a
  verification section for a spec that is not verified), the gate is asking the
  wrong question, and the author's correct move is to leave the work
  uncommitted and say so (`ai/rules/completion.md`).

## Ze Project Knowledge

### Project Knowledge (not in other rules)

- **Family registration** is dynamic via `PluginRegistry.Register()` -- never enumerate, validate format only.
- **Config pipeline**: File -> Tree -> `ResolveBGPTree()` -> `map[string]any` -> `reactor.PeersFromTree()`. Files: `internal/component/bgp/config/{resolve,peers}.go`, `.../reactor/config.go`.
- **Linter hook**: `postFormatGo` in `internal/le/hookruntime/postwrite.go` runs gofmt, `goimports -format-only`, and changed-code lint on Edit/Write. Imports are not auto-removed, so add an import and its use in the same edit.
- **Arch-0**: 4 components (Engine, ConfigProvider, PluginManager, Subsystem). Subsystem != Plugin (BGP daemon = subsystem; bgp-rib/rs/gr = plugins). Stream system = pub/sub backbone (`internal/component/plugin/server/dispatch.go`). Interfaces in `pkg/ze/`.
- **YANG choice/case**: `mandatory true` and inner-choice exclusivity NOT enforced by the walker. Plugins using `choice` add Go-side validation in their parser. `ze config validate` does not invoke `OnConfigVerify`.
- **Constants for command/status names** -- literals catch typos at compile time. Editor commands: `internal/component/cli/model.go`. Plugin status: `plugin.StatusDone`/`StatusError`.
- **Proximity**: `bgp/handler/` is a middleman; handlers belong in `bgp/plugins/`. ALL RPCs need YANG. See `ai/rules/plugins.md`.
- **LSP** at session start for Go nav -- more precise than grep for call chains and interface impls.
- **Inventory**: `./le inventory [--json]` imports `plugin/all` and queries real registries. Use for plugin counts, RPC totals, family coverage.
- **SDK type aliases** (`pkg/plugin/sdk/sdk_types.go` re-exporting `rpc.*`) are intentional -- external plugins import only `sdk`. Not identity wrappers.
- **No filtered/noexport route tracking** -- Ze does not store import-filtered or export-filtered routes (unlike BIRD's "import keep filtered on"): the RIB pipeline has scope keywords (sent/received/sent-received) and filter stages, but no "filtered" scope. The birdwatcher-compatible endpoints `/routes/filtered/{name}` and `/routes/noexport/{name}` return empty lists for compatibility; if filtered tracking ever lands, point them at the real store.
- **Gokrazy appliance owns process lifecycle** -- ze deploys as a gokrazy appliance: no systemd, no init system, no package manager. Any external process ze depends on (VPP or future dependencies) MUST be exec'd, supervised, and cleaned up by ze itself; ze MUST NOT be designed around an OS-level process manager.
- **Stress tooling is native Go**: `internal/le/integration/stress.go` owns stress orchestration, and the BGP UPDATE stream is generated inside `ze-test peer --mode inject`. Extend the Go injector for a new scenario with a pool-friendly byte builder, one pre-allocated buffer, one TCP writer, and a keepalive goroutine. Run it through `./le integration stress`.
- **CLI dispatch discoverability gaps**: (1) no one-shot command against a RUNNING daemon (`ze cli -c "summary"` shape). `ze show` and `ze run` use SSH (`sshclient.ExecCommand`) internally but expose no shell one-liner. The offline-config half is covered by `ze config show <file> [path...]`. (2) `ze help --ai --api` prints YANG RPC names (`ze-bgp:overview`), not the dispatch strings users type. (3) No way to list the Dispatcher's match keys. `reactor.ExecuteCommand()` accepts strings undiscoverable without reading source. The highest-value fix is the one-shot daemon command (SSH port 2222, credentials from the zefs database).

### Mistake Log

One-line lesson + rule pointer. Full root-cause in the linked journal row's Fix cell.

- **"Linux-only tests cannot run on this macOS host" is false** (RECURRING, ZERO TOL). Mark kernel-dependent `.ci` cases with `option=needs-linux`, use `./le qemu netns-test suites <names>` for a focused pass, and use `./le qemu all-tests` for the full runtime-kernel guest proof. A Linux-only test that fails on native Darwin needs the correct marker and a QEMU run, never an "environmental" dismissal. `ai/rules/platform-linux.md`.
- **Feature not wired** (RECURRING, ZERO TOL). Unit tests != wiring. The user entry point MUST be named. `ai/rules/completion.md`.
- **Daemon command without offline CLI** (sysctl-0). Every `CommandDecl` plugin MUST have a `cmd/ze/<name>/` offline entry point.
- **Wrong production path** (rib-04). ALL implementations MUST be grepped; the consumer's call chain MUST be traced.
- **Count-only test assertions** (addpath-rib). Assertions MUST be on content (keys/values), not `Len()`.
- **Wrapper struct pattern** (alloc-4). Raw bytes and existing iterators MUST be passed. Data MUST NOT be wrapped in accessor types.
- **Tests-pass != done** (RECURRING). Tests are step 10 of 12. Work MUST continue to docs/spec/summary/audit. `ai/rules/quality.md`.
- **Mechanism-not-behavior test** (prefix-limit). The AC MUST be asserted, not a code-path proxy. No-op passes = wrong test. `ai/rules/testing.md`.
- **"Pre-existing" failures** (RESOLVED). Blocks your goal: it MUST be fixed now. Does not: spec it, close the work in hand, ask Thomas whether that spec runs. `ai/rules/completion.md`.
- **Plugin placement anchor bias** (jsonrpc). "Delete the folder" test. Cross-cutting -> `internal/component/`. Domain -> `bgp/plugins/`. Infra -> `internal/core/`.
- **Docs from assumption** (RECURRING). Source MUST be read before any factual claim. `ai/rules/writing.md` Source Anchors.
- **Spec deleted without committing** (lg-overhaul, ZERO TOL). TWO commits MUST be made: (A) code+spec, (B) `git rm` spec + add summary. `ai/rules/planning.md`.
- **Reinventing repo contents** (lg-overhaul). Existing code MUST be grepped before writing new infra; `third_party/` and components often already have it. `ai/rules/architecture.md`.
- **Spec claimed complete with gaps** (lg-0..4). Learned summary with "future X" = spec NOT done. Each AC MUST be audited. `ai/rules/completion.md`.
- **Stale deferrals** (redist-phase2). Code MUST be grepped before a phase-N spec is created from open deferrals. `ai/rules/planning.md`.
- **Worktree copy into main** (ZERO TOL). Work MUST be committed in the worktree, and it MUST reach main only via merge or cherry-pick. `bashWorktreeCopy` in `internal/le/hookruntime/bash.go` enforces.
- **Same-day blocker fix** (cmd-4, RECURRING). A real adversarial review MUST race on reactor code, grep renamed-name consumers, grep sibling call sites, and break production to confirm the `.ci` test fails. `ai/rules/quality.md`.
- **Substring collision in bulk edits** (iface-tunnel). The longest prefix MUST be matched first, or non-name context MUST be added. Mangled names MUST be grepped for afterward.
- **Vendor != upstream** (iface-tunnel). Behavior MUST be verified against `vendor/<lib>/`, not upstream docs. The vendor path MUST be cited in the spec.
- **Naive reconciliation drops live state** (iface-tunnel). The new config MUST be diffed against the previous config, and the delta MUST be acted on. `previous` MUST be passed explicitly.
- **Invented config shape** (iface-tunnel). Existing `*-conf.yang` files MUST be grepped for the closest analog before new endpoint shapes are defined.
- **Scratch `.go` in `tmp/`** (iface-tunnel). `go test ./...` walks `tmp/`. Research agents MUST use `.txt` or build-tagged dirs.
- **CLI grammar from container nesting, not wire method** (as112-cli-audit). Operator-facing command words come from the YANG `container` tree; `ze:command "ze-X:Y"` is the INTERNAL RPC name and is deliberately different (e.g. `ze-bgp:peer-teardown` = command `request peer teardown`). Command syntax MUST NOT be inferred from wire-method names. Top-level operational verb is `request` (`request <object> <action>`); reads are `show`/`monitor`. `ai/rules/writing.md`.
- **ExaBGP migration sync** (exabgp-compat-sync). When ExaBGP adds a new SAFI or route type, three things MUST be updated: (1) `exabgp.yang` schema container, (2) `flexSafis` list or a dedicated `convert*ToUpdate` in `migrate_routes.go`, (3) compat test files (`.ci` + `.conf`). `ai/patterns/bgp-family.md` Section 5b.

## Your Own Mistakes

**When you find that you made a mistake, you MUST fix the SYSTEM that let it
through, not only the instance. Correcting the instance and moving on leaves the
next session to make the same mistake.**

**The rule you write MUST state the root cause and the general practice that
prevents it. It MUST NOT carry the example, the file, or the specifics of the
occurrence that produced it.**

A rule carrying its originating case is read as being about that case. The next
reader checks whether their situation matches the example, decides it does not,
and moves on. The shape recurs in surfaces the original occurrence never
touched, so the example is the part of the rule that ages fastest and costs the
most: it converts a general obligation into a specific one nobody thinks applies
to them.

**When preventing the recurrence needs code rather than prose, you MUST home
that work as a spec under `plan/future/` in the same session. A rule binds
whoever reads it; a gate binds everyone.**

## Friction Reporting

### Report immediately when

| Category | Examples |
|----------|----------|
| **Problem pattern** | The same mistake, rejected edit, missing wiring, misunderstood boundary, or unexpected failure appears more than once or is likely to recur |
| **Rule gap** | Existing rules did not say what to do, gave conflicting guidance, or made the wrong path look valid |
| **Missing docs** | Had to investigate something that should have been documented: file purpose, data flow step, registration pattern, gate behavior |
| **Stale info** | Rule or doc references deleted/renamed files, describes a pattern the code no longer follows |
| **Tooling friction** | Hook rejects valid code, linter config does not match rules, native action behaves unexpectedly |
| **Wasted effort** | Searched in the wrong place, duplicated existing functionality, misunderstood a layer boundary |

### Format

```
Friction: [what happened]
Pattern: [why this is likely to recur, or "one-off" if unsure]
Impact: [time/effort wasted, bug risk, or review risk]
Rule decision: [new rule needed / update existing rule / no rule, because...]
Proposed fix: [specific ai/rules, ai/INDEX, plan/journal, docs, or hook change]
```

### Timing

- The pattern MUST be reported as soon as you can describe it. You MUST NOT wait until the end of the session.
- **Reporting in chat is not filing.** Chat scrolls away and the next session never sees it, so hook and tooling friction is not reported until it is written to `plan/learned/HOOK-FRICTION.md` in the Format above; a finding you only pass to the next agent in a handoff is folklore, not a record.
- If the user task is still in progress, work MUST continue after reporting unless blocked or the rule change would alter scope.
- **When a pattern recurs, a row MUST first be appended to the matching `plan/journal/<class>.md`.** A rule MUST be added or updated only when the recurrence exposes a missing actionable instruction and no current rule or gate covers it.
- Blocking and related defects MUST still be fixed. This recording order changes no defect-fix obligation.

### Do Not Report

The following MUST NOT be treated as friction:
- Things that are simply unfamiliar before reading the relevant docs.
- Intentional deviations already documented in specs or rationale files.
- One-off issues that will not recur and expose no rule gap.
