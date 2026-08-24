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
| Developer tool, script, make target, generator, or inventory command | Agents must know the tool exists before reimplementing it |
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
| New tool or make target | `ai/INDEX.md` Dev Tools or keyword map, plus the owning `docs/contributing/` or `docs/architecture/testing/` page |
| New verification gate or hook | The "Hook-to-Rule Mapping" section below, the rule enforced by the hook, and the relevant make-target documentation |
| New doc or inventory checker | `docs/contributing/documentation-testing.md`, `mk/inventory.mk` quick reference, and `ai/rules/writing.md` if policy changed |
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
4. **What verification proves it?** The make target, unit test, functional test, hook, or doc validator that catches drift MUST be named.
5. **What docs explain usage?** The exact file and section MUST be named. Source anchors MUST be added for factual `docs/` claims.
6. **What journal record preserves the decision?** A row MUST first be appended to the matching `plan/journal/<class>.md` when a recurring trap is hit. The row is the record, never the fix: a blocking or related defect MUST still be fixed (`ai/rules/completion.md`).

### Current Discovery Surfaces

Use these before inventing a new mechanism:

| Need | Existing surface |
|------|------------------|
| Changed-file-aware wiring, doc, command, and inventory gate | `make ze-doc-wiring-check` |
| Documentation drift and YANG command contracts | `make ze-doc-verify` |
| Source-to-document reverse index | `make ze-doc-index-update`; read `ai/CODE-TO-DOCS.md` |
| RFC MUST requirement to enforcing-test coverage (which tests prove each requirement, plus the backlog) | `make ze-rfc-index-update`. Read `rfc/requirements/<stem>.md` for one RFC's requirement to test rows. Read `ai/RFC-REQUIREMENTS.md` for the counts, the coverage rollup and the backlog over all of them. Both are generated. Coverage is gated by `make ze-rfc-check`, staleness by `make ze-doc-verify` |
| What each package does ("what does what") | `make ze-discovery-index-update`; read `ai/PACKAGE-MAP.md` |
| Which `.go` files implement a design doc | read `ai/DOCS-TO-CODE.md` (inverse of `// Design:`) |
| Which tests enforce an RFC MUST | read `rfc/requirements/<stem>.md`, or print it with `python3 scripts/dev/rfc_requirements.py --show <stem>`. `make ze-rfc-index-update` generates it. `make ze-rfc-check` and `make ze-doc-verify` gate its freshness |
| The un-enrolled backlog, and how much each RFC still owes | read `ai/RFC-REQUIREMENTS.md`, the index over the per-RFC files (same generator, same gates) |
| Which problems recur | `make ze-journal-report`; read `plan/journal/` (one file per class, row count is recurrence) |
| Whether every path the instruction corpus names still resolves | `make ze-doc-links-check`. It is its OWN `ze-precommit-verify` stage: `make ze-doc-verify` runs no part of it, and `ze-generated-files-reconcile` ends with the `--md-only` subset. It also rejects a dead `*.sh` or `c_*`/`check_*` name in the hook-describing documents |
| Whether a `doc-links: ignore` marker states a reason, anywhere in the tree | `make ze-doc-links-check` (`check_ignore_reasons` in `scripts/dev/check_doc_links.py`). The sweep is over every TRACKED file, not the walked corpus, so a marker nobody's gate reads is still audited |
| Whether every path a TRACKED file names resolves, outside the instruction corpus | `make ze-doc-links-check` (`check_tracked_citations` in `scripts/dev/check_doc_links.py`). A dead path in any tracked file fails the gate. Repair the reference, or mark its line with a `doc-links: ignore` marker that states why the path cannot resolve. `vendor/` and `third_party/` are excluded because their references point into another repository, and `plan/handover/` because it records the tree as it was. `scripts/dev/doc_citation_baseline.txt` grandfathers the pairs that predate the check. `check_baseline_growth` compares the pairs against HEAD and refuses each pair HEAD does not hold, so that file only shrinks |
| Whether every symbol a `docs/` source anchor names is declared in the file that anchor points at | `make ze-doc-verify` (`check_anchor_symbols` in `scripts/dev/code_to_docs.py`). It resolves the tokens after the anchor's `--` against that file's own top-level declarations, and the `report=` argument `main()` passes decides whether a finding is emitted |
| How data flows through a subsystem | read `ai/digests/<subsystem>.md` (living, hand-maintained flow digests; `ai/digests/README.md` lists them); anchors validated by `make ze-digest-check` |
| Plugin, command, YANG, and test inventory | `make ze-inventory`, `make ze-inventory-json` |
| Command inventory | `make ze-command-list`, `make ze-command-list-json` |
| Spec progress | `make ze-spec-status`, `make ze-spec-status-json` |
| Generated plugin imports | `make ze-plugin-imports-check` |
| Whether the tree GIT HOLDS compiles, as opposed to the working tree every other gate reads | `make ze-repository-tracked-build-check` (`REV=<sha>` judges another commit). Runs in `ze-precommit-verify`, both modes, and is a structural gate in `scripts/dev/commit_helper.py` |
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
| `ai/INSTRUCTIONS.md` | `CLAUDE.md`, `AGENTS.md` | `make ze-ai-instructions-generate` or `make ze-ai-skills-sync` |
| `ai/skills/*.md` | `.claude/skills/*/SKILL.md`, `.codex/skills/*/SKILL.md`, `.agents/skills/*/SKILL.md` | `make ze-ai-skills-sync` |
| `ai/rules/points/<rule>/` | `ai/rules/<rule>.md` | `make ze-rules-render-update` |

### Rule Placement

- **Project-wide behavior rules, workflow rules, and agent rules MUST live under `ai/rules/`, not under a tool-specific home directory such as `~/.claude/rules/`.**
- **Tool-specific files are only for behavior that applies exclusively to that tool outside this repository.**
- **`ai/rules/*.md` are tool-agnostic and RENDERED from `ai/rules/points/<rule>/`. It MUST NOT be edited by hand. Edit the point file that carries the instruction, or the manifest that carries the title, the trigger and the reading order. Then run `make ze-rules-render-update`. `.claude/rules/*.md` are Claude-specific originals and MUST NOT be used for shared Ze project behavior. These two directories are independent; neither generates the other.**
- **One instruction is one file, and its PATH is its id.** `ai/rules/points/<rule>/<slug>.md` holds one block of the rule, verbatim, behind a small frontmatter header. `ai/rules/points/<rule>/manifest.md` holds the rule's title, its `**When:**` trigger, its severity, and the ordered slug list the renderer concatenates. A point on disk that the manifest does not list is a hard render error, never a silent drop.
- **Second generation:** `ai/rules/INDEX.md` is generated by `scripts/dev/rules_index.py` from the RENDERED rule files' headings and summary lines. It MUST NOT be edited by hand; run `make ze-rules-index-update`. To change a rule's one-line overview, edit the `when:` field in that rule's manifest, run `make ze-rules-render-update`, then regenerate.
- **Second generation:** `scripts/dev/rules_condensed.py` generates TWO artifacts from one parse of the RENDERED rule files. They MUST NOT be edited by hand; run `make ze-rules-condensed-update`. To change what they contain, edit the rule's points, run `make ze-rules-render-update`, then regenerate.

| Artifact | Holds | Imported into every session? |
|----------|-------|------------------------------|
| `ai/rules/TRIGGERS.md` | one routing line per rule: path, severity, `**When:**` trigger. Every rule, so none is ever invisible. The generator prints the count; do not copy it here | yes |
| `ai/rules/CORE.md` | the condensed directives of the always-on rules. Membership is DERIVED (rungs 1 and 2 of the ladder in `ai/rules/rule-precedence.md`, the ladder itself, any rule with no routable trigger, and any blocking rule no past task description would surface) | yes |

**Membership in `CORE.md` MUST NOT be edited, because it is never written down.** To make a rule always-on, change what the derivation reads: name it on rung 1 or 2 of the ladder in `ai/rules/rule-precedence.md`. A list of filenames in the generator would read identically until the ladder changed underneath it (`ai/rules/evidence.md`).

### Mechanical Check

**Before editing any file listed in the "Generates" column above, STOP. You MUST find its canonical source in the left column and edit that instead.**

**Every `make ze-*-update` target derives its output from the WORKING TREE, so in a shared checkout it picks up other sessions' uncommitted sources. You MUST diff a regenerated artifact before you name it in a commit.** Sixteen such targets exist and not one warns you.

The output is correct for the tree it read. It is wrong for the commit you are about to make, because that commit does not carry the sources the regeneration saw. What lands is a derived file that describes code nobody can see.

`commit_helper.py` refuses a commit whose regenerated artifact was derived from a tree holding sources the commit does not carry. That refusal is the only thing that catches this.

**The safe regeneration is HEAD plus your own files.** When an artifact is fully generated and yours was the only edit, `git show HEAD:<path>` written back over it restores the committed state, and the gate then agrees.

**The mirror image is worse and no gate catches it: committing a document that DESCRIBES uncommitted code.** A committed document that names a symbol still sitting in the working tree reddens `ze-doc-links-check` for every session until that code lands. A check that you have not swept somebody's work IN does not check the other direction: prose you committed about work still sitting OUT.

### Drift Detection

**The `CLAUDE.md`, `AGENTS.md` and skill mirrors are gitignored, so `git diff` can NEVER show drift for them.** `make ze-ai-sync-check` (also part of `make ze-generated-files-reconcile`) compares content against a fresh generation; the session-start hook runs it and warns `generated agent files are stale` when a resync is needed. You MUST fix it with `make ze-generated-files-update`. `ai/rules/<rule>.md` is the one "Generates" target that IS tracked, so `git diff` does show its drift, and `make ze-rules-render-check` reaches the same verdict, but writes nothing.

### Banned Actions

| Action | Fix |
|--------|-----|
| Editing `CLAUDE.md` directly | Edit `ai/INSTRUCTIONS.md`, run `make ze-ai-instructions-generate` |
| Editing `AGENTS.md` directly | Edit `ai/INSTRUCTIONS.md`, run `make ze-ai-instructions-generate` |
| Editing `.claude/skills/*/SKILL.md` directly | Edit `ai/skills/*.md`, run `make ze-ai-skills-sync` |
| Editing `.codex/skills/*/SKILL.md` directly | Edit `ai/skills/*.md`, run `make ze-ai-skills-sync` |
| Editing `.agents/skills/*/SKILL.md` directly | Edit `ai/skills/*.md`, run `make ze-ai-skills-sync` |
| Editing `ai/rules/<rule>.md` directly | Edit the point under `ai/rules/points/<rule>/`, run `make ze-rules-render-update` |
| Editing `ai/rules/INDEX.md` directly | Edit the rule's point or manifest, run `make ze-rules-render-update`, then `make ze-rules-index-update` |
| Editing `ai/rules/TRIGGERS.md` or `ai/rules/CORE.md` directly | Edit the rule's point or manifest, run `make ze-rules-render-update`, then `make ze-rules-condensed-update` |

## Hook-to-Rule Mapping

Quick reference: which checks enforce which rules, and when they trigger.

### Architecture: checks live in three Python dispatchers

The per-check shell hooks were consolidated into one Python dispatcher per trigger, so a tool call pays one process instead of dozens. The checks below are functions inside these files, not separate scripts:

| Dispatcher | Runs on | Contains |
|---|---|---|
| `.claude/hooks/pretool-bash.py` | PreToolUse `Bash` | every Bash check below |
| `.claude/hooks/pretool-writeedit.py` | PreToolUse `Write\|Edit\|MultiEdit\|NotebookEdit` | every Write/Edit check below |
| `.claude/hooks/pretool-agent-skill.py` | PreToolUse `Task\|Agent` | two gates: skills-over-raw-agents (`ai/rules/cli.md`), and review-runs-on-Opus-5 (`ai/rules/planning.md`) |
| `.claude/hooks/posttool-writeedit.py` | PostToolUse `Write\|Edit` | the formatters (gofmt/goimports/golangci, ruff) + cheap advisory checks |

Still standalone (single-purpose or deliberately not folded): `block-until-lsp.sh`, `validate-spec.sh` (see note below), `mark-lsp-invoked.sh`, `mark-source-read.sh`, and the session-lifecycle hooks. The Stop hook also shells out to `scripts/dev/spec-closure-check.py` (the spec-closure detector; also usable directly as `--list`).

**Changing a check:** the function in the relevant dispatcher (not a `.sh`) MUST be edited, then `python3 scripts/dev/hook-parity-check.py` MUST be run to confirm no behaviour changed. If you intentionally changed behaviour, the golden table MUST be re-blessed with `python3 scripts/dev/hook-parity-check.py --bless`, and the result MUST be pasted back. The "Discovery Updates" section above MUST also be satisfied so future agents can find it.

**Reads never block:** `Read`, `Grep`, `Glob`, `LSP`, `WebFetch`, `WebSearch` are never rejected. Two of them write a non-blocking freshness marker so the `design-without-lsp` gate knows the implementation was investigated: `LSP` (via `mark-lsp-invoked.sh`) and `Read` of implementation source (via `mark-source-read.sh`). Source is what a spec can be ABOUT, and the KIND is the file's EXTENSION with no directory anchor: `.go`, `.py`, `.sh`, `.yang`, the `Makefile`, `*.mk`. Each accepted Read records its kind (`go`, `py`, `sh`, `make`, `yang`) in `.source-read-<kind>-<sid>` beside the aggregate marker, which is how the gate asks for the spec's own subject rather than for any file at all. The extension is the whole rule because this writer and `_SUBJECT_PATTERNS` in `pretool-writeedit.py` are two ends of one contract: the gate demands the kind a spec's own file list names, so a path the writer refuses is a spec nobody can ground by reading its own subject. A directory list is a second thing to keep in sync, and it drifted: 11 open specs named `.py` subjects under `test/` and `tools/` and 2 named `.sh` subjects under `packaging/` whose only exit was reading an unrelated file. A Read is also held to a DEPTH bar: a window of under 20 lines records nothing, while a whole file records its kind at any length. A Read that showed NOTHING records nothing, whatever else the payload says: an empty file, a window past the end, a failed Read, and the `file_unchanged` answer the harness gives a repeat WHOLE Read all measure zero, and zero is not the same as unmeasurable. A stale marker MUST therefore be renewed with `Read(path, offset=N, limit>=20)`, which returns content where a second whole Read returns nothing. Only a payload shape the writer does not recognise AT ALL is accepted unmeasured, so an unfamiliar payload still cannot silently disable the evidence path. The count is lines of TEXT, taken from the response body, because `numLines` counts the trailing newline as a line and a 19-line window arrives as 20. Only mutating/executing tools (`Bash`, `Write`, `Edit`, `MultiEdit`, `NotebookEdit`, `Task`, `Agent`) and `ToolSearch` (which loads LSP) are actually gated.

**Every marker is keyed by session id**, and every consumer MUST use
`.claude/hooks/lib/session_id.py`. Bash hooks call it through
`.claude/hooks/lib/session-id.sh` (`_session_id`). Python callers import
`session_id()` and reuse `_sid_safe()` for direct values.

`session-start.sh` and `subagent-context.sh` pass hook JSON to
`--hook-session-id`. This mode validates the decoded raw string before shell
normalization. It returns status 0 for a safe id, status 1 for an absent field,
and status 2 for malformed JSON or an invalid field. SessionStart has an empty
matcher so startup, resume, clear, compact, and fork events republish an
accepted id through `$CLAUDE_ENV_FILE`. SubagentStart falls back to `_session_id`
only for status 1. It emits its complete context as JSON
`hookSpecificOutput.additionalContext`. Status 2 adds no parent id, path, spec,
or state. For a restricted subagent Bash call, `pretool-bash.py` prefixes the
command with the accepted parent id from the PreToolUse payload.

The hook MUST NOT persist `$ZE_SESSION_ID`. `mk/helper-session.mk` derives it. Unsafe
ids are rejected rather than rewritten. The validator rejects dot entries.
`make ze-unit-hook-test` (section `session-id`) locks this behavior.

### PreToolUse Checks (block before the tool runs)

#### LSP gate (`block-until-lsp.sh`, standalone)

Enforces `session-start.md`. Triggers on `Bash|Write|Edit|MultiEdit|NotebookEdit|ToolSearch|Task|Agent`.
Blocks those tools until `ToolSearch query="select:LSP"` has run this session. BLOCKING. <!-- severity-note: the LSP gate's severity, not this reference page's -->

#### Bash (`pretool-bash.py`)

| Check | Enforces | Triggers on | What it does |
|---|---|---|---|
| `check_destructive_git` | `git-safety.md` | Bash | Blocks git commit/push/reset/restore/clean/merge. Allows `git restore --staged`. BLOCKING. |
| `check_worktree_copy` | `ai/INSTRUCTIONS.md` prohibitions (no point) | Bash | Blocks cp/mv/rsync from `.claude/worktrees/` to main repo. BLOCKING. | <!-- doc-links: ignore (.claude/worktrees/ exists only while a worktree agent is active) -->
| `check_root_build` | (build hygiene, no point) | Bash | Blocks `go build` without `-o bin/`. Allows `go build ./...` (check-only). BLOCKING. |
| `check_pipe_tail` | `commands.md` | Bash | Blocks a lossy filter (`head`/`tail`/`grep`/`awk`/`sed`/`cat`/`less`/`more`) piped from an EXPENSIVE producer: `make`, `go test\|build\|vet\|run`, `golangci-lint`, `bin/ze*` (with or without `./`), the same binaries in this session's own directory (`tmp/session/<YYYY-MM-DD>-<id>/bin/ze*`, relative or absolute), `ze-test`, `pytest`, everything under `scripts/evidence/` (QEMU boots, docker interop labs), and the repo's own gates under `scripts/{dev,checks,docvalid,status}/` whose filename contains check/verify/test/audit/lint/stress/repro -- minus a small cheap-probe set (`verify-status.sh`, `verify-lock.sh`, `verify-summary.sh`, `spec-closure-check.py`), which are status readers CLAUDE.md tells you to run. `\| tee` passes, and cheap commands (`git log \| tail`, `scripts/dev/spec-session.sh wip \| head`) are not its business. Judged **per statement** (`;`, `&&`, `\|\|`, newline), so a cheap pipeline beside an expensive command is fine; a trailing `\|` or a `\\` at end of line is a CONTINUATION, not a boundary, and is flattened first (splitting there put the producer in one statement and the filter in the next, so neither tripped). Quote- and `$( )`-blind by design: a `;` inside quotes or a command substitution splits a statement, and a producer inside `bash -c "..."` is not seen. BLOCKING. |
| `check_raw_test_invocation` | `commands.md` | Bash | Blocks a heavy job typed raw, so it cannot reach the machine outside job admission: `go test`, `golangci-lint` (any subcommand but `config`/`version`/`help`/`linters`/`cache`/`completion`, which run no analysis), the `ze-test` runner in `bin/` or in this session's own directory (`tmp/session/<YYYY-MM-DD>-<id>/bin/`, with or without `./` and with the QEMU arch suffix), and a Python test file (`*_test.py`, `test_*.py`) run by hand. Each refusal names the `make` target that runs the same work, so the queued path is on screen. The command WORD decides, per statement (`;`, `&&`, `\|\|`, newline) and per pipeline segment, so a `make` target is never refused whatever its arguments spell (`make ze-qemu-debug RUN='bin/ze-test-linux-arm64 ...'`) and a banned verb in a search PATTERN stays text. `go build` belongs to `check_root_build`, and `bin/ze` is not the runner. Two ways through, both visible in the transcript: `scripts/dev/ze-run.sh <label> <command>` queues the raw command, and `ZE_ADMIT_RAW="<reason>" <command>` admits a one-off (an empty reason admits nothing). Text-matched like every check here, so a job inside a script file or a `bash -c` string is out of reach by construction. BLOCKING. |
| `check_poll_loop` | `commands.md` | Bash | Blocks an unbounded wait loop: `while`/`until` paired with a `sleep` call, or with `pgrep` in the loop CONDITION. The bound is credited **per loop, in the statement that loop's keyword opens**, so an earlier `timeout` elsewhere on the line does not disarm it, a bounded loop does not cover a later unbounded one, and a `-timeout` FLAG (`go test -timeout 300s`) never counts -- crediting any of those made the guard fail open. A keyword inside a search argument is TEXT (`grep`, `rg`, `git log -S`), so the rule stays auditable from Bash. `while read` and a one-shot `pgrep` are not its business. Two limits are by construction: a loop inside a script file is unseen, and a loop that bounds itself in its own condition (`while [ $SECONDS -lt 300 ]`) is still refused, since the arithmetic is not decidable here. Quoting a loop to RUN-shape is rejected like every text-matched check (`plan/learned/HOOK-FRICTION.md` F22). BLOCKING. |
| `check_system_tmp` | `testing.md` | Bash | Blocks access to `/tmp`; must use this session's own scratch directory (`scripts/dev/session-scratch.sh`). BLOCKING. |
| `check_scratch_path` | `commands.md` | Bash | Blocks a redirect or `tee` that names a fixed file at the `tmp/` ROOT (`> tmp/out.log`), which is keyed per checkout and so is the same file for every session in it. A subdirectory passes (`tmp/s/<id>/`, `tmp/session/<YYYY-MM-DD>-<id>/`, any per-task folder), and so do the root names that are session-keyed or shared by design (`ze-precommit-verify*`, `commit-*`, `delete-*`, `mutation*`, `test-timings*`). Which paths those are is decided in `.claude/hooks/lib/scratch_path.py`, shared with `c_scratch_path_we` so a path this check refuses cannot land through the Write tool. Two shapes write nothing and pass. A heredoc body a NON-SHELL reads is data, so a document that quotes the banned shape can be written with `cat >> file <<'EOF'`; fed to `bash` it is a script and still blocks, and the redirect that opens the heredoc is judged on its own. And a QUOTED redirect opening a search command is a search argument (`grep -rn '> tmp/out.log' ai/rules`), which is what keeps the ban auditable from Bash; that shape needs both conditions, since `grep foo ai/rules > tmp/notes.txt` writes for real and `bash -c "... > tmp/x"` is run-shaped quoting (F22). BLOCKING. |
| `check_governed_doc_edit` | `commands.md` | Bash | Blocks a WRITE from Bash to a file under `plan/` or `ai/rules/`; use the Write or Edit tool. The reason is coverage, not taste: `.claude/hooks/pretool-writeedit.py` is wired to the matcher `Write\|Edit\|MultiEdit\|NotebookEdit` (`.claude/settings.json`), so a Bash write reaches NONE of its checks -- `c_design_without_lsp` is the one that bites, but a spec rewritten by a heredoc is asked for no evidence at all. Auto mode tells agents to prefer Bash for file changes, so the bypass is the DEFAULT route rather than an exotic one, which is how it went unnoticed until 2026-08-18. **Two tiers, because one match cannot serve both shapes.** A shell verb binds TIGHTLY, on the verb alone: a redirect into a guarded path, `sed -i`, `tee`, a `cp`/`mv` whose destination is one. Reads therefore stay free (`grep`, `cat`, `sed -n`), and so does `commit_helper.py create --file plan/spec-x.md`, which names those paths constantly and would otherwise refuse itself. An interpreter payload (`python3`, `perl`, `ruby`) binds LOOSELY: a guarded path mentioned anywhere beside any write primitive. Loose ON PURPOSE -- a literal-path match (`Path("plan/x.md").write_text`) misses a path built from a variable in a loop, which is the shape that produced this finding. The cost is that a payload merely READING `plan/` and writing to scratch is refused too. That is answered by `ZE_ADMIT_GOVERNED_WRITE="<reason>"`, mirroring `ZE_ADMIT_RAW`: an empty reason admits nothing and the reason lands in the transcript, so the escape is auditable by reading the session. A false positive costs one env assignment, a false negative costs the guard. A generated artifact written by its own generator (`make ze-rules-render-update`, `commit_helper.py` writing `plan/verification-debt/`) is a tool writing rather than an agent editing, and the command string never carries the path, so it needs no carve-out. Text-matched like every check here, so a write inside a script file is out of reach by construction. BLOCKING. |
| `check_test_deletion` | `testing.md` | Bash | Blocks `rm`/`git checkout` of test files. BLOCKING. |

The five commit-time gates (spec-audit, deferral-in-diff, deferral-unassigned, wiring-at-commit, doc-drift) belong in **creation-time gates in `scripts/dev/commit_helper.py`** because the sanctioned commit path does not send the literal `git commit` string to this hook. See "Commit-time gates" below.

`golangci-lint run` also runs standalone on `Bash(git commit:*)`.

#### Write/Edit (`pretool-writeedit.py`)

| Check | Enforces | Triggers on | What it does |
|---|---|---|---|
| `c_claude_tree_has_ai_home` | `repo-maintenance.md` (canonical sources) | Write/Edit under `.claude/<sub>/` | WARNS that the file is read by this tool alone and names `ai/<sub>/` as the shared home, because `ai/INSTRUCTIONS.md` generates BOTH `CLAUDE.md` and `AGENTS.md`. Never blocks: `.claude/rules/` legitimately holds tool-SPECIFIC extensions -- `planning.md` there opens "Extends `ai/rules/planning.md`" -- so refusing would reject correct work. What the author needs is the question while they can still answer it: does this bind every agent, or only this one. The population is DERIVED rather than listed -- it fires for `.claude/<sub>/` exactly when `ai/<sub>/` exists on disk, which today is `agents`, `rules` and `skills`, and stays silent for `hooks`, `settings`, `plan`, `output-styles`, `docs` and `memory`, so editing a hook is not nagged. A new shared tree earns the reminder by existing, and a hardcoded list cannot drift out of date. Written after a session filed "read ze-style every session" in `.claude/rules/session-start.md` alone, where it reached Claude and never reached Codex. WARN. |
| `c_design_without_lsp` | `.claude/rules/session-start.md`, `evidence.md` | design/spec `.md` | Blocks edits to `plan/design-*.md` / `plan/spec-*.md` unless EVERY kind of source the spec's own `## Files to Modify` and `## Files to Create` name was read in the last 30 min, each kind on its own clock. The kind is the file's extension: a Go spec needs a `.go` or the LSP tool (gopls, so it is Go evidence and no other), a hooks or tooling spec needs its `.py`/`.sh`, a YANG spec a `.yang`, a build spec the `Makefile` or a `.mk`. That spelling matches `mark-source-read.sh` exactly, so reading the file the spec names always clears it. A spec naming several kinds needs all of them, so the author cannot list a cheap file as the evidence for an expensive one. A spec whose subject cannot be read falls back to any implementation source and WARNS that it did; a session that investigated nothing is always refused. BLOCKING. | <!-- doc-links: ignore (hook trigger patterns, files may not exist) -->
| `c_pre_write_go` | `.claude/rules/post-compaction.md` (no point) | `internal/**/*.go` | Blocks without proper session state. BLOCKING. |
| `c_source_edit_spec` | `planning.md` | source/test/learned | Blocks edits when selected spec is not `in-progress`. BLOCKING. |
| `c_encoding_alloc` | `performance.md` | wire-encode `.go` | Blocks `make()`/`append()`/`Bytes()`/`Pack()` in wire-facing code. BLOCKING. |
| `c_format_alloc` | `performance.md` | BGP format `.go` | Blocks `strings.Join`/`Builder`/`NewReplacer`/`ReplaceAll` (+ `fmt.Sprintf`/`Fprintf`, `strconv.Format*`) in the guarded format files. Comment lines exempt. BLOCKING. |
| `c_sprintf_new` | `performance.md` | `.go` | Blocks new `fmt.Sprintf`/`Fprintf`/`Printf`. Allows `fmt.Errorf`. BLOCKING. |
| `c_string_concat` | `performance.md` | `.go` | Blocks `"a" + b` string concatenation in production Go. Exempt: a comment line, a `const` declaration, two adjacent string literals, and `filepath`/`path` `Join`/`Dir`/`Base`. Use `textbuf.Buffer`. BLOCKING. |
| `c_legacy_log` | `go-standards.md` | `.go` | Blocks `log.Printf` / legacy `log` package. BLOCKING. |
| `c_panic` | `go-standards.md` | `.go` | Blocks `panic()` except `unreachable`/`not implemented`/`TODO`/`BUG`/`impossible`. BLOCKING. |
| `c_ignored_errors` | `go-standards.md` | `.go` | Blocks `_, _ =` error-swallowing. BLOCKING. |
| `c_silent_ignore` | `config.md` | `.go` | Blocks empty `default:` cases. BLOCKING. |
| `c_temp_debug` | `go-standards.md` | `.go` | Blocks debug-MARKER prints (`DEBUG`/`TRACE`/`>>>`/`<<<`/`***`/`XXX`/`FIXME`) via `fmt.Print*`/`Fprint*`, bare `println(...)`, and short bare `fmt.Println("...")` in production Go. Plain `os.Stderr` output is ALLOWED -- it is the CLI's interface, and `cli.md` prescribes it. BLOCKING. |
| `c_raw_ansi` | (no point: the palette is in `docs/architecture/cli/color-system.md`) | `.go` | Blocks a raw ANSI escape (`\033[`, `\x1b[`, `\e[`, `\u001b[`). Allowed in `textbuf.go`, `helpfmt.go` and `_test.go` only. Elsewhere use the `helpfmt` constants. BLOCKING. |
| `c_os_exit` | `cli.md` | `.go` | Blocks `os.Exit()` outside `main.go`/`register.go`/`scripts/`. BLOCKING. |
| `c_layering` | `no-layering.md` | `.go` | Blocks backwards-compat/layering patterns. BLOCKING. |
| `c_exabgp` | `go-standards.md` | `.go` | Blocks ExaBGP awareness outside `exabgp/`. BLOCKING. |
| `c_version_config` | `config.md` | config files | Blocks version fields in config. BLOCKING. |
| `c_nolint` | `quality.md` | `.go` | Blocks `//nolint:` without justification. BLOCKING. |
| `c_lint_exclusions` | `quality.md` | `.golangci.*` | Blocks adding lint exclusions. BLOCKING. |
| `c_and_functions` | `architecture.md` | `.go` | Warns about `func *And*()` names. Advisory. |
| `c_init_register` | `go-standards.md` | `.go` | Blocks `init()` outside `register.go`. BLOCKING. |
| `c_yagni` | `architecture.md` | `.go` | Blocks speculative-feature comments. BLOCKING. |
| `c_fake_bufhandle` | `performance.md` (pool correctness) | `.go` | Blocks `BufHandle{Buf: make(...)}` outside `testPoolBuf`. BLOCKING. |
| `c_observer_sys_exit` | `testing.md` | `.ci` | Warns about `sys.exit(1)` in observers without `runtime_fail`. Advisory. |
| `c_ci_sleep_justification` | `testing.md` | `.ci` | Warns when a `time.sleep(` is introduced with no comment above/trailing it. Advisory (blocking gate is `make ze-doc-wiring-check`). |
| `c_hardcoded_commands` | `evidence.md` | `.go` | Blocks hardcoded command-list literals. BLOCKING. |
| `c_switch_dispatch` | `plugins.md` | `.go` | Blocks `switch args[0]` subcommand dispatch; use `subdispatch.New()` + `Register()`. BLOCKING. |
| `c_json_kebab` | `cli.md`, `go-standards.md` | `.go` | Blocks non-kebab-case JSON tags. BLOCKING. |
| `c_goroutine` | `goroutine-lifecycle.md` | hot-path `.go` | Blocks `go func()` in reactor/event/dispatch/hub/wire/message. BLOCKING. |
| `c_require_design_ref` | `go-standards.md` | `.go` | Blocks Go files without `// Design:` comment. BLOCKING. |
| `c_require_related_refs` | `go-standards.md` | `.go` | Blocks missing/stale `// Related:`/`// Detail:`/`// Overview:` refs. BLOCKING. |
| `c_test_weakening` | `testing.md` | `_test.go`, and `.ci`/`.et` under `test/` | Blocks the one-directional weakenings: a deleted `Test`/`Fuzz`/`Benchmark` func, an added `t.Skip`, commented-out assertions, an `ignore` build tag, content replaced with nothing, and on a `.ci`/`.et` an emptied needle, an inverted `reject=`, or an assertion that cannot fail. REPORTS and allows a falling count (removed assertions, `require`->`assert`, dropped `t.Run` cases or table rows, dropped `expect=`/`reject=`/`cmd=` lines), because a count cannot tell a deleted check from three consolidated into one. Escape: a row in `test/weakened.md` naming the test THIS edit weakens, `\| <TestName> \| <reason> \|` under the header `\| Test \| Reason \|`. The row is written BEFORE the edit: the hook reads the file from disk, so a row not yet written buys nothing, and neither does a row naming another test. The file is replaced per commit and never accumulates, so no ceiling caps it. `weakened_problems` (`scripts/dev/commit_helper.py`) recomputes the same judgement over the commit's own paths, count drops included, and also refuses a commit that weakens a test and does not carry `test/weakened.md`. One implementation serves both gates, `scripts/dev/check_weakened_tests.py`, so they cannot disagree about what a diff weakens. BLOCKING. |
| `_rfc_tagged_change_err` | `testing.md`, `ai/skills/ze-rfc.md` | any tag CARRIER holding `RFC requirement:` -- `_test.go`, `.ci`, `.et`, an interop `check.py` | The guard is `_rfc_tagged_change_err`, called from `c_test_weakening`. Blocks ANY behavior change to a test that proves an RFC obligation. It runs BEFORE test-weakening. A row in `test/weakened.md` does NOT satisfy it, because self-service justification is not user approval. Also blocks DELETING the tag (checked first, since a tag is a comment and the behavior comparison would pass its removal). Scope is the ENCLOSING test function (`_enclosing_tagged_scope`, which now delegates to `scripts/dev/rfc_tagged_scope.py`), not the edited hunk. So a tag on the doc comment still governs a body edit. Untagged functions in the same file are unaffected. A tag outside every function, such as a hoisted table, widens scope to the whole file. Every occurrence of a hunk is considered, so `replace_all` cannot reach a tagged copy unseen. Comment/format edits pass -- `#` counts as the comment syntax for `.ci`, `.et` and `.py`; a rename blocks. A `.go` edit made ONLY of import lines passes too (`_import_only_go_edit`): an import cannot weaken an assertion, and without it GROWING a tagged file always cost an approval, because new tests need new imports and an import block sits outside every function so the scope widens to the whole file. Every non-blank line on both sides must be import-shaped, so an assertion smuggled into the same edit still blocks, and the tag-removal check runs first so a tag cannot ride out on the exemption. Escape: a row in `test/rfc-changed.md` naming the test THIS edit changes. Its shape is `\| <TestName> \| <what the owner approved, and why the requirement is still proven> \|` under the header `\| Test \| Reason \|`. Only the OWNER authorizes one. The row is written BEFORE the edit, because the hook reads the file from disk. A row not yet written buys nothing, and neither does a row naming another test. The file is replaced per commit and the commit carries it, so the approval sits in git history beside the change. `rfc_changed_problems` (`scripts/dev/commit_helper.py`) recomputes the same judgement over the commit's own paths. `rfc_changed_units` (`scripts/dev/check_weakened_tests.py`) names the unit for both, so neither gate can ask for a row the other refuses. The old escape was an `// rfc-test-change-approved:` comment in the edit's own replacement text. It is retired, no gate reads one, and the tree holds none: 268 markers across 125 files were swept on 2026-08-19, with 27 `test-relax:` beside them. About one block in six carried a fact about its own test found nowhere else, so 57 of those survive as ordinary comments with the approval framing removed. BLOCKING. **The carrier list is derived** from the shared leaf. `TestTaggedScopeCoversEveryCarrier` holds it against the scanner's own `CARRIERS`. Until 2026-07-29 the predicate was a literal covering `_test.go` and a `/test/` `.ci` only. The two interop `check.py` files that the RFC evidence rules admit therefore carried RFC obligations this guard could not see. |
| `c_system_tmp_we` | `testing.md` | any | Blocks writing to `/tmp`. BLOCKING. |
| `c_scratch_path_we` | `commands.md` | any | Blocks a write to a file at the project `tmp/` ROOT, which is keyed per checkout and so is the same file for every session in it. The Write half of `check_scratch_path`: both call `.claude/hooks/lib/scratch_path.py`, so a path a redirect is refused cannot land through the Write tool instead. A subdirectory passes (`tmp/s/<id>/`, `tmp/session/<YYYY-MM-DD>-<id>/`, any per-task folder), and so do the root names that are session-keyed or shared by design (`ze-precommit-verify*`, `commit-*`, `delete-*`, `mutation*`, `test-timings*`). BLOCKING. |
| `c_generated_files` | `repo-maintenance.md`, "Canonical Sources and Sync Direction" above | `CLAUDE.md`/`AGENTS.md` | Blocks editing generated files. BLOCKING. |
| `c_rendered_rules` | `repo-maintenance.md`, "Canonical Sources and Sync Direction" above | any `*.md` sitting DIRECTLY in `ai/rules/` | Blocks editing a rendered rule and names the point to edit instead. Also covers `INDEX.md`, `TRIGGERS.md` and `CORE.md`, which no hook guarded before, and points each at its own generator. `ai/rules/points/**` is the canonical source and is always permitted: a point's parent is `ai/rules/points/<rule>`, so the dirname test lets it through. Matched by realpath against `CLAUDE_PROJECT_DIR`, for the reason `generated-files` records, and it refuses rather than permits when the path cannot be resolved. BLOCKING. |
| `c_point_overwrite` | `never-destroy-work.md` | a `Write` to a path under `ai/rules/points/<rule>/` | Blocks a `Write` over a point file that already exists, and names both non-destructive routes: edit that point, or pick a slug no file uses. A `Write` to a NEW path is how a point is authored and stays permitted, and `Edit`/`MultiEdit` are targeted so neither can silently drop a body. The render gates report the same damage one step too late: the instruction is gone at write time and only git holds it. BLOCKING. |
| `c_rule_point_rfc_language` | `rule-format.md` | a `Write` or `Edit` to `ai/rules/points/<rule>/<section>/<slug>.md` | Blocks a `kind: directive` point that does not state its obligation in RFC 2119 language. A `Write` carries the whole point, so a missing capitalised keyword is refused there. An `Edit` carries a fragment, so what is decidable is the lowercase `must`/`shall`/`should`/`may` it introduces, and that is refused for both tools. Code spans and fenced blocks are quoted text and are never read. `make ze-rules-lint` reads the finished tree and owns the rest. BLOCKING. |
| `c_line_number_ref` | `evidence.md`, `writing.md` | `.md` under `ai/`, `docs/`, `plan/`, `.claude/` | Blocks a `path:NN` line citation and a `#LNN` permalink anchor in prose. Cite the file and the symbol instead. A fenced block, an `rfc/full/` path, and a file declaring itself generated in its first ten lines are all exempt. `scripts/dev/line_refs.py --apply` sweeps an existing file. BLOCKING. |
| `c_claude_plans` | `.claude/rules/planning.md` (no point) | Write | Blocks `.claude/plans/` and `~/.claude/plan/`. BLOCKING. | <!-- doc-links: ignore (banned location, deliberately nonexistent) -->
| `c_check_existing_patterns` | `architecture.md` | new `internal/**/*.go` | Blocks duplicate exported type/func in same package. BLOCKING. |
| `c_enforce_naming` | (no point: the file-naming convention is unwritten) | new files | Warns on wrong file naming. Advisory. |
| `c_throwaway_tests` | `testing.md` | Write | Blocks test files in `/tmp` and throwaway locations. BLOCKING. |
| `c_utils_package` | `architecture.md` | Write `.go` | Blocks `utils/`/`helpers/`/`common/`/`misc/` packages. BLOCKING. |
| `c_direct_fs_state` | `architecture.md` | `.go` under `internal/plugins/`, `internal/component/`, `cmd/ze/` | Warns on `os.WriteFile`/`Create`/`Rename`/`Symlink`/`Link` and a creating `os.OpenFile`. Runtime state belongs in `internal/core/statestore` or a `storage.Storage` handle, because only `database.zefs` is managed on the appliance. The blocking gate is `make ze-fs-persistence-check`. Advisory. |
| `c_require_test_first` | `testing.md` | new `.go` | Blocks a `Write` that creates a source file in a package holding no `_test.go`. **The unit is the package, not the file.** It asked for `<name>_test.go` beside every source file. That spelling refuses a package which tests itself from one `<pkg>_test.go`, the common shape here. Spelled per-file, the check can never block. It also returned 1, a non-blocking warning, under a message that said `BLOCKED`. It announced an obligation it held nobody to. Both were fixed on 2026-08-19. Exempt: `_test.go`, `_gen.go`, `.pb.go`, `cmd/`. An `Edit` and a write from Bash reach it never. `test_coverage_problems` judges that population at commit time. BLOCKING. |
| `c_require_docs_read` | `.claude/rules/post-compaction.md` (no point) | new spec | Warns when writing a spec without session-state evidence. Advisory. |

> **format-alloc is live.** It is `c_format_alloc` in
> `.claude/hooks/pretool-writeedit.py`. The guarded list is current, and comment
> lines are exempt like `sprintf-new`. Its incremental value over `sprintf-new`
> is the `strings.Join`, `Builder`, `NewReplacer`, and `ReplaceAll` bans.
> Covered by `scripts/dev/hook-fixture-check.py` (`format-alloc-*`).

#### Task/Agent (`pretool-agent-skill.py`)

| Check | Enforces | Triggers on | What it does |
|---|---|---|---|
| `verdict` | `cli.md` | Task/Agent | Blocks a raw agent spawn when a skill covers the task. Name the skill instead. BLOCKING. |
| `review_model_refusal` | `planning.md` | Task/Agent | Blocks a review agent spawned off Opus 5. The other half of the same rule is `_model_refusal` in `scripts/dev/review_gate.py`, which refuses to RECORD a review taken off that model. BLOCKING. |
| `style_guide_reminder` | `go-standards.md` | Task/Agent | Warns when a brief that will produce Go never names `docs/contributing/ze-style.md`. A subagent inherits the session-start style read through no mechanism the main thread can verify, and cannot be audited afterwards either, because subagent transcripts live under `/tmp` and `check_system_tmp` refuses that path. WARN, never blocking: the population is a heuristic over prose, so a block would refuse correct work. |

### PostToolUse Checks (run after the tool completes)

| Check | File | Enforces | Triggers on | What it does |
|---|---|---|---|---|
| mark-lsp-invoked | `mark-lsp-invoked.sh` | `session-start.md` | LSP | Writes `.lsp-invoked` freshness marker for the design-without-lsp gate. |
| mark-source-read | `mark-source-read.sh` | `evidence.md` | Read | Writes the `.source-read` freshness markers when implementation source is read, so reading the producing code satisfies the design-without-lsp gate. Two files per accepted Read: the aggregate `.source-read-<sid>`, and `.source-read-<kind>-<sid>` naming the kind (`go`, `py`, `sh`, `make`, `yang`). The kind is the file's extension, spelled identically here and in `_SUBJECT_PATTERNS`, so the file a spec names is always a file whose Read records the kind that spec demands. A window of under 20 lines records nothing, and so does a Read that showed nothing at all: an empty file, a failed Read, or the `file_unchanged` answer to a repeat Read. Non-blocking. |
| mark-agent-spawned | `mark-agent-spawned.sh` | `planning.md` | Agent, Task | Writes `.agent-spawned-<sid>` so the Stop hook can tell a supervising main thread from one that ran the phase inline. The hook runs in the parent process. The marker uses the supervising session. It does not use the subagent environment. Non-blocking. |
| auto-lint | `posttool-writeedit.py` | `go-standards.md` | `.go` Write/Edit | `gofmt`/`goimports -w`, then **one** `golangci-lint --new-from-rev=HEAD` pass (flags only issues this edit introduced). BLOCKING on lint failure. |
| auto-py-format | `posttool-writeedit.py` | (code style) | `.py` Write/Edit | `ruff format` + `ruff check`. Non-blocking. |
| validate-spec | `validate-spec.sh` | `planning.md` | `plan/spec-*.md` | Validates required sections/format. Exit 2 blocks a structurally invalid spec; both `→` and `->` wiring rows accepted. |
| design-doc-owner | `validate-spec.sh` via `scripts/dev/spec_doc_anchors.py` | `repo-maintenance.md` | `plan/spec-*.md` past `skeleton` | Reads the `// Design:` header of every source file the spec's Files to Modify and Files to Create name, and BLOCKS until each declared document is named somewhere in the spec. Naming it as unaffected, with the reason, satisfies it: the requirement is that the author looked. Docs that only `<!-- source: -->` mention the file are printed as an advisory `note:`, not blocked, because a change can legitimately leave most of them alone. The checker's own absence is an error, never a skip. Answers Documentation Update Checklist row 16 by derivation instead of from memory. |
| file-size | `posttool-writeedit.py` | `go-standards.md` | `.go` | Warns >1000 lines. Advisory. |
| warn-deferral | `posttool-writeedit.py` | `planning.md` | `.md` | Warns on deferral language in doc edits. Advisory. |
| journal-row-shape | `posttool-writeedit.py` | `planning.md` | `plan/journal/*.md`, README.md excluded | Names every line of a journal class file that `journal_row_cells` (`scripts/dev/journal.py`) does not read as the five cells. It imports that parser rather than copying it, and it reads the file from disk, because an Edit can break a row by changing a line that is not the whole row. Two things cause the finding: a raw pipe in the prose, and a second markdown table in the file. `journal_row_problems` (`scripts/dev/commit_helper.py`) refuses the same lines when the commit is prepared, which is the only place the rule was enforced before. Advisory, because the write already landed and one edit clears the finding. Fixtures: `python3 scripts/dev/hook-fixture-check.py --only journal-row-shape`. |
| require-rfc-reference | `posttool-writeedit.py` | `go-standards.md` | `.go` | Suggests `// RFC:` header. Advisory. |
| require-test-docs | `posttool-writeedit.py` | `testing.md` | `_test.go` | Warns about missing `VALIDATES:`/`PREVENTS:`. Advisory. |
| require-fuzz-tests | `posttool-writeedit.py` | `testing.md` | wire `.go` | Warns about `Parse*` without `Fuzz*` tests. Advisory. |
| vague-names | `posttool-writeedit.py` | `architecture.md` | `.go` | Warns about `Data`/`Info`/`Result`/... names. Advisory. |
| boundary-tests | `posttool-writeedit.py` | `testing.md` | `.go` | Warns about numeric validation without boundary tests. Advisory. |

> **validate-spec.sh is standalone.** It accepts both `→` and `->` in the
> Wiring Test table. The `WIRING_ROWS=` assignment is guarded with `|| true`, so
> the script always reaches its verdict: exit 2 for a structurally invalid spec,
> exit 0 otherwise.
> It stays out of the dispatcher (see spec Key Design Decisions). Covered by
> `scripts/dev/hook-fixture-check.py` (`validate-spec-*`).

`make ze-precommit-verify` separately runs `ze-doc-wiring-check` (wiring/doc-drift gate); that is a Make target, not a Claude hook.

### Changed-file gates inside `ze-doc-wiring-check`

Also Make targets, not Claude hooks. All are changed-file scoped: a session owns the files it touches, not the whole tree.

| Check | Enforces | Triggers on | What it does |
|---|---|---|---|
| `check_ci_sleep_ratchet` | `testing.md` | changed `test/**/*.ci` | Caps how MANY `time.sleep(` calls exist tree-wide against a committed delta baseline. BLOCKING. |
| `check_ci_sleep_justification` | `testing.md` | changed `test/**/*.ci` | Caps how many sleeps are UNEXPLAINED: each needs a comment above or trailing it. BLOCKING. |
| `check_known_failure_load_excuses` | `completion.md` | changed `plan/known-failures/*.md` | Rejects a shard blaming host load ("under load", "loaded host", "load average", "load-sensitive", "passes in isolation", "resource contention", "contended host"). `README.md` / `RESOLVED.md` exempt. BLOCKING. |
| `is_docker_exec_source` | `evidence.md`, `testing.md` | changed `test/**/*.py`, the checker, or the floor | Selects `make ze-functional-docker-exec-check` (`scripts/dev/docker_exec_checked.py`). Caps how many test-harness call sites read a fail-open return value without testing it for emptiness: `docker_exec_quiet` answers `""` on any non-zero exit, so an untested read turns a FAILED command into a passing assertion over nothing. The fail-open set is derived to a fixpoint, so a new wrapper is covered the day it is written. The floor in `test/health/docker-exec-baseline.json` may only go DOWN. `test/draft/` exempt; opt out per site with `# fail-open-ok: <reason>`. BLOCKING. |
| `check_ci_log_subsystem_keys` | `testing.md`, `config.md` | changed `test/**/*.ci` | Rejects a `ze.log.<subsystem>` key whose subsystem contains a hyphen that is not declared literally in Go. An internal plugin's logger name is `CanonicalSubsystemName` of its registry name (every hyphen becomes a dot) and `getLogEnv` splits on `.` only, so `ze.log.bgp.adj-rib-in` sets nothing and the level silently stays at the WARN default. Scan is tree-wide; `#` comment lines exempt. BLOCKING. |

`make ze-precommit-verify` separately runs `ze-repository-tree-check`, three of the five checks in `scripts/dev/validate.py`; that is a Make target, not a Claude hook. The other two are changed-file scoped and run NOWHERE automatically: `make ze-repository-check` gives you all five over your own tree, by hand.

| Check | Enforces | Triggers on | What it does |
|---|---|---|---|
| `check_source_anchor_line_numbers` | `writing.md` | every `docs/**/*.md` | Rejects a `<!-- source: -->` anchor that carries a line number, because line numbers rot. Gated: `ze-repository-tree-check`. BLOCKING. |
| `check_source_anchor_stale_paths` | `evidence.md` | every `docs/**/*.md` | Rejects a repo-relative anchor path that no file or directory answers. It resolves ANY root, including anchors outside the `PATH_PREFIX` roots that `scripts/dev/code_to_docs.py` walks. Gated: `ze-repository-tree-check`. BLOCKING. |
| `check_spec_ac_completeness` | `completion.md`, `planning.md` | every `plan/spec-*.md` whose Status is `in-progress` | Rejects an acceptance-criterion row whose `Demonstrated By` cell is empty, so an in-flight spec cannot claim an AC with no named evidence. Gated: `ze-repository-tree-check`. BLOCKING. |
| `check_cross_package_wiring` | `completion.md` | `git diff HEAD` plus untracked `.go` files under `internal/` or `cmd/` | Reports an exported symbol with no cross-package non-test caller. UNENFORCED: no gate runs it (owner decision, 2026-08-09). Run `make ze-repository-check` by hand. |
| `check_cli_handler_coverage` | `testing.md` | the same changed-file list, `.go` files under the CLI paths only | Reports a newly registered command that no `.ci` test names. UNENFORCED: no gate runs it (owner decision, 2026-08-09). Run `make ze-repository-check` by hand. |

The two changed-file checks take `changed_files` as their subject, which is `git diff HEAD` plus untracked files. Several sessions share this checkout, so that list includes other sessions' half-written work. Both checks demand completeness that a file in the middle of an edit cannot show. They therefore stay out of the gate.

### Prose gate (ASD-STE100)

Make targets and a commit-time gate, not Claude hooks. HEAD is the baseline and the comparison is per file, so a document nobody touched can never fail.

| Check | Enforces | Triggers on | What it does |
|---|---|---|---|
| `ste_problems` (`commit_helper.py`) | `writing.md` | `commit_helper.py create` with any `.md`, `.go`, or `.yang` in the commit | Runs `ste_check.py --check` over the FILES OF THAT COMMIT and PRINTS the six banned ASD-STE100 habits that grew against HEAD. Advisory: STE is a guideline, so this never refuses a commit. Commit scope is deliberate: several sessions share this checkout, so a tree-wide prose gate judges a colleague's in-flight sentences and gets switched off. BLOCKING. |
| `make ze-ste-check` | `writing.md` | on demand, before you prepare a commit | The same comparison over every changed file in the working tree. It prints the file, the habit, and only the new findings. Surfaces are Markdown in `docs/`, `ai/`, `plan/`, and the root, prose comments in `.go`, and `description` strings in `.yang`. Renames follow `-M`, so a moved legacy document does not report its inherited content as new. Deliberately NOT in `ze-doc-verify`. Whole-tree report: `make ze-ste-review`. |

### Commit-time gates (`scripts/dev/commit_helper.py`)

These are NOT Claude hooks. They run when `commit_helper.py create` generates the commit script, which is the only sanctioned commit path (run the script at the path its `script=` line prints). The helper already knows the exact add/remove set of the commit, so the gates inspect that instead of the staging area. BLOCK gates raise (exit 2, no script written); WARN gates print to stderr and let the script be written.

| Gate | Enforces | Severity | What it does |
|---|---|---|---|
| go-design-ref (`go_design_ref_problems`) | `go-standards.md` | BLOCK | Refuses a commit whose non-exempt `.go` files carry no `// Design:` header. This is the COMMIT-TIME half of `c_require_design_ref`, which `.claude/settings.json` wires to `Write`, `Edit`, `MultiEdit` and `NotebookEdit` only -- so a `.go` written from Bash reaches it never, and auto mode tells agents to prefer Bash for file changes. The two halves answer different questions and only the second cannot be routed around: the hook asks whether an EDIT is allowed, which depends on the tool; this asks whether the TREE THIS COMMIT PRODUCES holds the header, which depends on nothing but the file. A changed-file set at commit time is a fact, and how each file came to be written is neither recoverable nor needed. Exemptions are restated from the hook and kept in step, because a file the hook waves through and this refuses would leave no route to commit: `_test.go`, `_gen.go`, `register.go`, `embed.go`, `doc.go`, anything under `vendor/`, and a file whose first 500 bytes say `Code generated` or `DO NOT EDIT`. Scoped to what a tree can prove. `c_require_related_refs` gates on session-state markers. Those describe what the AUTHOR did rather than what the commit contains, so it stays hook-only by construction. This row named `c_require_test_first` beside it, and that was wrong. That check reads `isfile()` and no session state, so the tree can answer its question. The claim went unchecked and the gap stayed open until 2026-08-19. Its second half is `test-coverage` below. Fixtures pin both halves, since a gate that never fires looks exactly like a clean tree. |
| test-coverage (`test_coverage_problems`) | `testing.md` | BLOCK | Refuses a commit that carries non-exempt `.go` and no test path. The COMMIT-TIME half of `c_require_test_first`. That check fires on a `Write` of a NEW `.go` file and nothing else. A source file added by `Edit`, or written from a Bash heredoc, meets no test obligation anywhere. Auto mode tells agents to prefer Bash for file changes, so that route is the default one. Either test clears this gate: a `_test.go`, or a `.ci`/`.et` under `test/`. `testing.md` asks for both and says neither substitutes for the other. That is a property of a FEATURE, not of a commit. A commit landing the unit tests before the functional one does the right thing. Exemptions are restated from the hook and from go-design-ref, so no file is caught between two gates. Exempt: `_test.go`, `_gen.go`, `register.go`, `embed.go`, `doc.go`, `vendor/`, `cmd/`, and a file whose first 500 bytes say `Code generated`. It shipped WARN for one session, because this checkout is shared and a gate refusing every refactor commit holds other sessions' work back. The owner armed it on 2026-08-19. `--no-test "<reason>"` answers it: a pure rename, a comment fix, a refactor whose behaviour is unchanged. The reason is echoed to stderr and lands in the transcript, the shape `ZE_ADMIT_GOVERNED_WRITE` uses, and an empty reason admits nothing. It is deliberately NOT a `DEBT_FLAGS` entry. Every gate a debt row names can be re-run, which is what `debt-clear` does, and "this commit carried no test" cannot be re-judged once the commit exists. A row for it stays open forever. It sits outside `commit_gate_problems` behind its flag, the way the RFC-tagged-change gate does. |
| verify-status / structural-gate | `precommit-verify.md` | BLOCK | Refuses a script over a non-green `ze-precommit-verify` (structural reds are unbypassable). |
| ste (`ste_problems`) | `writing.md` | WARN | Runs `ste_check.py --check` over the commit's own `.md`, `.go`, and `.yang` files and prints the six banned ASD-STE100 habits that grew against HEAD. It never refuses. Legacy prose in a file you touched costs nothing, because each file is compared with its own HEAD version. Scoped to the commit for the same reason discovery-index materializes a commit view: a concurrent session's uncommitted prose must not block your commit. Prints the file, the habit, and only the new findings. |
| discovery-index | "Discovery Updates" above | BLOCK | Refuses when a generated index (PACKAGE-MAP) would be left incoherent. Judged on the tree the COMMIT PRODUCES (HEAD + adds - removes), materialized under `tmp/commit-view-*` and checked with the commit's OWN generators via `--root`, never on the working tree: a concurrent session's uncommitted sources must neither block your commit nor be swept into your index. **Every** index whose generator exists is verified, not just the ones the commit visibly feeds: `package_map` keys its rows on directory existence, so a new `.go` can drift PACKAGE-MAP without carrying a `// Package` header at all. Cost: the view is built on EVERY commit the gate examines, not only ones touching an index source, because the candidate set comes from generator existence, about 5.5s total (~2s working-tree freshness, ~3.6s to materialize and check). If the view cannot be built it does NOT fail closed: it warns on stderr and falls back to the working-tree verdict, which is the only evidence left. BLOCKING. |
| rfc-changed (`rfc_changed_problems`) | `testing.md` | BLOCK | Refuses a commit that changes an RFC-tagged test and carries no row for it in `test/rfc-changed.md`. Refuses a stale row too, one naming a test the commit does not change. The row is the OWNER's approval, so a commit holding it in the working tree only is refused as well: the approval belongs in git history beside the change. It borrows the edit-time hook's own `_rfc_tagged_change_err` to judge the change. `rfc_changed_units` (`scripts/dev/check_weakened_tests.py`) names the unit for both gates, so neither can ask for a row the other refuses. Population is the commit's own `--file` and `--remove` lists, which makes the BLOCK tier safe in a shared checkout. It sits behind `--rfc-change-ok "<who approved it, and when>"` rather than inside `commit_gate_problems`, the way the review gate does. |
| deferral-unassigned | `planning.md` | BLOCK | Folds over every shard in `plan/deferrals/` and flags an open row with no Destination. |
| deferral-in-diff | `planning.md` | BLOCK | Blocks when the commit's added lines contain deferral language and no `plan/deferrals/` shard is part of the commit (diff computed in a throwaway git index). |
| review-rounds (`ROUND_CAP`, `cmd_record` in `scripts/dev/review_gate.py`) | `planning.md` | BLOCK | Runs at `review_gate.py record`, not at commit time, and is listed here beside the `review_gate.py check` half that commit_helper runs. Refuses an artifact claiming more than three review rounds unless `--rounds-reason` names the PRODUCT defect a later round found, and refuses fewer than one. The count is written into the artifact header as `rounds=N`, so a closure's review cost is auditable after the fact. It does NOT cap rounds that find real defects: it prices the fourth at one sentence, which is the sentence a loop auditing its own bookkeeping cannot write. |
| journal-row (`journal_row_problems`) | `planning.md` | BLOCK | Refuses a commit that ADDS a `plan/journal/<class>.md` row not holding the five cells `\| Date \| Spec \| Surface \| Symptom \| Fix \|`. Not cosmetic: `_journal_added_spec_stems` reads the same rows for the closure stem, so a row the parser cannot read yields no stem, `spec_closure_stem` returns None, and the review gate stops firing on the commit carrying the code. `make ze-journal-report` cannot cover it, because that reads HEAD and the bad row is only visible before the commit lands. |
| spec-audit | `planning.md` | BLOCK | Blocks the closure commit (the one ADDING a `plan/journal/*.md` row whose `Spec` cell names the spec, or `plan/learned/NNN-<stem>.md`) when that spec's `## Pre-Commit Verification` section is unfilled. ADDING is load-bearing: the row must be new to this commit, which is what `_journal_added_spec_stems` answers, so a class file's months-old first row is not read as today's closure. Keyed to `spec-session.sh current`; no claim → skips. |
| wiring-at-commit | `completion.md` | WARN | Warns when `internal/plugins/**/*.go` is committed with no `.ci`. |
| doc-drift | `writing.md` | WARN | Runs `scripts/docvalid/doc_drift.go`; warns on drift. |

### Hook tests (`make ze-unit-hook-test`)

| Runner | Covers |
|---|---|
| `scripts/dev/hook-parity-check.py` | Golden exit-code regression for the three consolidated dispatchers. `--bless` regenerates the golden; re-bless only intentionally changed cases. Fixture dirs live under `~/.cache` (a `/tmp` or in-repo path trips `system-tmp`/`throwaway-tests` or the module lint and diverges from the golden). **Its cases carry no `plan/spec-*` or `plan/design-*` path, so it never reaches `c_design_without_lsp`.** A green parity run says the OTHER checks did not move; it is not evidence the design gate behaves, and citing it as such is a claim about code the run never executed. That gate's evidence is the `design-gate` and `mark-source-read` sections of `hook-fixture-check.py`. |
| `scripts/dev/hook-fixture-check.py` | Behaviour the golden table cannot isolate: `c_format_alloc`, `validate-spec.sh`, the `commit_helper.py` commit-time gates over git-initialized fixtures, and the 36 `delegation` fixtures. The `design-gate` section drives `mark-source-read.sh` and `pretool-writeedit.py` over ONE fixture project, so the writer's kinds and the reader's subjects cannot drift apart. It holds both directions FOR EACH OF THE FIVE KINDS, which is a claim to keep true rather than to repeat: `<kind>-subject-needs-its-own-kind` makes that kind the subject and grounds it with a foreign read, and `<kind>-subject-cleared-by-its-own-file` grounds the same spec with the file it names. Deleting any one row of `_SUBJECT_PATTERNS` reds its own pair. The earlier version claimed both directions while every rejecting case ran on `go` or `py`, so the `sh`, `yang` and `.mk` rows could be deleted with all 21 fixtures green. `contract-both-ends-agree-*` walks 14 real spec subjects through the writer AND the gate and compares what one supplies with what the other demands, which is what a directory anchor creeping back into either end now trips. Cases exist because reviewers defeated earlier versions of the gate: LSP alone standing in for a Python subject, a cheap kind standing in for an expensive one beside it, a fresh kind renewing a stale one, a `### Checklist` row becoming a subject, a table's description column becoming one, and `Read(file, limit=1)` counting as reading the producer. Each fails against the gate it was written against. Those 36 pin what no other test reaches. The `Stop` array registration and its order. The claim lifetime: alive past turn one, released only by `scripts/dev/spec-session.sh release` from `/ze-close`, and no `SessionEnd` hook registered to delete it again. The two stop-phrase tiers. The markup filter that must fail toward scanning MORE. Deleting the release line once left the whole suite green. Sections selectable with `--only`. |

### Session Lifecycle Hooks

**UserPromptSubmit stdout reaches the model. UserPromptSubmit stderr does not.** A reminder that MUST land in the context writes to stdout. A banner that MUST cost no context tokens writes to stderr, as `compaction-reminder.sh` does. The two stdout reminders below fire on every turn, so each one stays a single line.

| Hook | Event | What it does |
|---|---|---|
| `session-start.sh` | SessionStart (all sources) | Passes the raw payload to the canonical validator. The empty matcher covers startup, resume, clear, compact, and fork. Publishes an accepted id as `CLAUDE_CODE_SESSION_ID` for the hook and appends it to `CLAUDE_ENV_FILE`. Prints the status summary. Deletes NOTHING. `make ze-scratch-clean` and `make ze-session-clean BEFORE=<date>` are the operator's cleanup routes. |
| `compaction-reminder.sh` | UserPromptSubmit | Detects compaction; reminds to read `post-compaction.md`. Writes to **stderr**, so it costs no context tokens. |
| `verify-claim-reminder.sh` | UserPromptSubmit | Emits one **stdout** line per turn. Verify a claim about code by reading the function that PRODUCES the behavior, not the caller. Label an unread claim unverified. Name the file and the symbol, and use a line number only when the line IS the fact. Report the conclusion, not the search. Enforces `ai/rules/evidence.md` and `ai/rules/writing.md`. A banner read once at session start does not survive to the turn that makes the claim, so this lands in fresh context. |
| `delegation-reminder.sh` | UserPromptSubmit | Emits one **stdout** line per turn: subagent delegation needs no permission in this repository. The harness appends the guard "Do not call the AgentTool unless the user requested it" to the END of the system prompt, where it wins on position. `ai/INSTRUCTIONS.md` "STANDING REQUEST: delegate to subagents" IS the request that guard defers to, but it sits far earlier in the same prompt and loses. UserPromptSubmit stdout is the only harness position that lands after the whole system prompt, so the counter goes there. Unconditional by design: a conditional reminder adds a "did the condition fire" failure mode, and the reminder is correct on every turn. Enforces `ai/rules/planning.md`. Fixtures: `python3 scripts/dev/hook-fixture-check.py --only delegation-reminder`. |
| `block-premature-stop.sh` | Stop (**first**, ahead of `session-end-summary.sh`) | **Live and BLOCKING.** Four checks run. (1) The stop-phrase scan exits 2 on a match. `COMPLETION_PHRASES` joins the scan only while the claimed spec is `in-progress`. (2) For a claimed spec, `spec-closure-check.py` exit 3 means implemented but not closed, so the hook exits 2. (3) A claimed spec whose metadata is `in-progress` adds a state warning. (4) A missing `.agent-spawned-${SID}` marker adds a delegation warning. State warnings without a stop phrase exit 1 and do not block. There is no `verification` status branch. |
| `session-end-summary.sh` | Stop | Writes session state snapshot. It does not release the spec claim, and no hook does: `spec-session.sh release` does, from `/ze-close`, so the claim survives the turn-by-turn `Stop` and every session end after it. There is no `SessionEnd` hook at all; the only work that event ever did here was deleting `tmp/session/`. |
| `session-end-deferrals.sh` | Stop | Prints open deferral count. Advisory. |
| `pre-compact-save.sh` | PreCompact | Saves session state before compaction. |
| `subagent-context.sh` | SubagentStart | Validates the parent `session_id` from the hook payload. Only an absent field permits resolver fallback. A malformed value adds no parent id, path, spec, or state. Emits the complete context through JSON `hookSpecificOutput.additionalContext`. For a safe id, that context includes the exact parent id and scratch directory, the parent session's claimed spec and status, and the subagent contract from `ai/rules/planning.md`. |

### Pre-Flight Checklist by File Type

(All checks below now run inside the dispatchers; behaviour is unchanged.)

#### Any `.go` file under `internal/`

pre-write-go, auto-lint, encoding-alloc (wire paths), sprintf-new, legacy-log, panic-error, ignored-errors, silent-ignore, temp-debug, os-exit, init-register, yagni-violations, json-kebab, require-design-ref, require-related-refs, file-size.
Hot-path files (reactor/event/dispatch/hub/wire/message) also: goroutine-lifecycle, fake-bufhandle.

#### Test files (`_test.go`, `.ci`)

test-weakening (if removing/weakening tests), check-existing-tests (if new), require-test-docs, boundary-tests, observer-sys-exit (`.ci` with Python).

#### Spec files (`plan/spec-*.md`)

validate-spec, design-without-lsp (needs recent investigation of EVERY source kind the spec's own Files to Modify and Files to Create name, where the kind is the file's extension: for Go, LSP invoked or a `.go` read; for a tooling, hooks, YANG or build subject, that file read, and read more than a 20-line window of it, or the whole of it -- a Read that showed nothing, such as a second whole Read the harness answers with `file_unchanged`, records nothing), require-docs-read (if new), source-edit-spec-not-in-progress (if spec not `in-progress`), design-doc-owner (past `skeleton`: every `// Design:` document declared by a file the spec names must itself be named in the spec).

design-without-lsp and design-doc-owner are the two halves of one question and neither substitutes for the other: the first says the CODE was read, the second says the code's own DESIGN DOCUMENT was accounted for.

#### Python files (`.py`)

auto-py-format (ruff format + check).

#### Commits

A Bash `git commit` is blocked outright by destructive-git. Commit via `scripts/dev/commit_helper.py create`, then `bash` on the path its `script=` line prints; the creation-time gates (verify-status, discovery-index, deferral-unassigned, deferral-in-diff, journal-row, spec-audit block; wiring-at-commit, doc-drift warn) run then. See "Commit-time gates" above.

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

**`make ze-repository-tracked-build-check` is the only gate that compiles what git holds,
and it compiles no `_test.go`. Its green therefore says nothing about the test
build. Before you treat work as committable, you MUST also compile the test
binaries of every package you touched, without running them.**

## Ze Project Knowledge

### Project Knowledge (not in other rules)

- **Family registration** is dynamic via `PluginRegistry.Register()` -- never enumerate, validate format only.
- **Config pipeline**: File -> Tree -> `ResolveBGPTree()` -> `map[string]any` -> `reactor.PeersFromTree()`. Files: `internal/component/bgp/config/{resolve,peers}.go`, `.../reactor/config.go`.
- **Linter hook** (`auto-lint` in `.claude/hooks/posttool-writeedit.py`) runs gofmt + `goimports -format-only` on Edit/Write (imports are no longer auto-removed) -- still add import + usage in the same edit.
- **Arch-0**: 4 components (Engine, ConfigProvider, PluginManager, Subsystem). Subsystem != Plugin (BGP daemon = subsystem; bgp-rib/rs/gr = plugins). Stream system = pub/sub backbone (`internal/component/plugin/server/dispatch.go`). Interfaces in `pkg/ze/`.
- **YANG choice/case**: `mandatory true` and inner-choice exclusivity NOT enforced by the walker. Plugins using `choice` add Go-side validation in their parser. `ze config validate` does not invoke `OnConfigVerify`.
- **Constants for command/status names** -- literals catch typos at compile time. Editor commands: `internal/component/cli/model.go`. Plugin status: `plugin.StatusDone`/`StatusError`.
- **Proximity**: `bgp/handler/` is a middleman; handlers belong in `bgp/plugins/`. ALL RPCs need YANG. See `ai/rules/plugins.md`.
- **LSP** at session start for Go nav -- more precise than grep for call chains and interface impls.
- **Inventory**: `make ze-inventory [--json]` imports `plugin/all` and queries real registries. Use for plugin counts, RPC totals, family coverage.
- **SDK type aliases** (`pkg/plugin/sdk/sdk_types.go` re-exporting `rpc.*`) are intentional -- external plugins import only `sdk`. Not identity wrappers.
- **No filtered/noexport route tracking** -- Ze does not store import-filtered or export-filtered routes (unlike BIRD's "import keep filtered on"): the RIB pipeline has scope keywords (sent/received/sent-received) and filter stages, but no "filtered" scope. The birdwatcher-compatible endpoints `/routes/filtered/{name}` and `/routes/noexport/{name}` return empty lists for compatibility; if filtered tracking ever lands, point them at the real store.
- **Gokrazy appliance owns process lifecycle** -- ze deploys as a gokrazy appliance: no systemd, no init system, no package manager. Any external process ze depends on (VPP or future dependencies) MUST be exec'd, supervised, and cleaned up by ze itself; ze MUST NOT be designed around an OS-level process manager.
- **Stress injector is in-memory Go**: the BGP UPDATE stream for stress scenarios 01-05 is generated inside `ze-test peer --mode inject` and streamed over the TCP socket after the OPEN handshake. Extend the Go injector for new scenarios with a pool-friendly byte builder, one pre-allocated buffer, one TCP writer, and a keepalive goroutine. `test/stress/` is the Python harness (`harness.py`, `run.py`, `scenarios/`).
- **CLI dispatch discoverability gaps**: (1) no one-shot command against a RUNNING daemon (`ze cli -c "summary"` shape). `ze show` and `ze run` use SSH (`sshclient.ExecCommand`) internally but expose no shell one-liner. The offline-config half is covered by `ze config show <file> [path...]`. (2) `ze help --ai --api` prints YANG RPC names (`ze-bgp:overview`), not the dispatch strings users type. (3) No way to list the Dispatcher's match keys. `reactor.ExecuteCommand()` accepts strings undiscoverable without reading source. The highest-value fix is the one-shot daemon command (SSH port 2222, credentials from the zefs database).

### Mistake Log

One-line lesson + rule pointer. Full root-cause in the linked journal row's Fix cell.

- **"Linux-only tests can't run on this macOS host / need hardware" is a LIE** (RECURRING, ZERO TOL). Ze HAS a QEMU Alpine-VM harness: `option=needs-linux` `.ci` tests SKIP on native darwin and RUN under `make ze-qemu-needs-linux-test` / `ze-qemu-test-all`; kernel/netlink/nft/veth/loop tests run via `make ze-qemu-integration-test` and the `ze-qemu-<feature>-test` targets. A Linux-only test that FAILS (not skips) on native darwin is missing its `option=needs-linux` marker (fix: the marker MUST be added, then the test MUST be run in QEMU), never "environmental / unfixable here". A Linux test red MUST NOT be attributed to "darwin env" or "needs docker/qemu we don't have": we HAVE QEMU, and the test MUST be run there. `ai/rules/platform-linux.md`.
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
- **Worktree copy into main** (ZERO TOL). Work MUST be committed in the worktree, and it MUST reach main only via merge or cherry-pick. `check_worktree_copy` in `.claude/hooks/pretool-bash.py` enforces.
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
| **Tooling friction** | Hook rejects valid code, linter config does not match rules, make target behaves unexpectedly |
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
