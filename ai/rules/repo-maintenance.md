# Repository Maintenance

**When:** adding or changing a feature, tool, gate, hook, runtime dependency, or generated file, looking up which check enforces a rule, or reporting development friction
**Severity:** blocking
**Related:** rule-format, writing, evidence, testing

## Directives

- **A change that adds or changes something future agents need to use, verify, document, or avoid MUST update the discovery path in the same work.**
- **Every feature that adds a new runtime dependency must register a `ze doctor` check so agents can verify readiness before starting the daemon.**
- **Never edit a generated file. Edit the canonical source, then sync.**
- **Project behavior rules belong in `ai/rules/` and project startup guidance belongs in `ai/INSTRUCTIONS.md`, so Claude, Codex, and other agents all discover the same rule through generated tool-specific files.**
- **Consult the hook-to-rule mapping BEFORE writing code to comply in advance, rather than to fix after rejection. For hook false positives and workarounds, see `plan/learned/HOOK-FRICTION.md`.**
- **Report a recurring problem pattern, repeated surprise, stale guidance, tooling friction, or wasted effort immediately, and say whether a new or changed rule would prevent it.**

## Discovery Updates

### Trigger

Apply this rule when adding or changing any of these:

| Change | Why agents need it |
|--------|--------------------|
| User-facing feature | Agents must know the feature exists and where users configure or invoke it |
| CLI command, RPC, MCP tool, YANG command, or API contract | Agents must discover the command shape, JSON contract, and wiring |
| Developer tool, script, make target, generator, or inventory command | Agents must know the tool exists before reimplementing it |
| Self-check, verification gate, hook, lint, or doc validator | Agents must run the right check and understand failures |
| Test runner, test format, fixture pattern, or required test category | Agents must place tests in the right suite and run the right target |
| Runtime dependency or readiness condition | Agents must verify the host with `ze doctor` before starting Ze |
| Structural decision, repeated gotcha, or workflow change | Agents must find it through the learned index or a rule before repeating the mistake |
| New BGP family, SAFI, or capability | Agents must update migration schema, route converter, bridge, and compat tests (`ai/patterns/bgp-family.md`) |
| RFC-level protocol behavior added, changed, or newly proven | The standards ledger drives user and design decisions; a stale RFC status misleads both |

**Private refactors with no new surface still trigger this rule when they change a pattern future work must follow.**

### Required Discovery Artifacts

Update every row that applies:

| What changed | Required update |
|--------------|-----------------|
| User-facing behavior | Specific file under `docs/`, with source anchors per `ai/rules/writing.md` |
| RFC support status (protocol behavior implemented, changed, or newly proven) | The matching `docs/features/rfc-status.md` row (Status, Implemented coverage, Remaining) with a source anchor to the producing `file:line`; reconcile `docs/comparison.md` and `docs/features.md` when the support level changes |
| Agent-facing command or contract | `docs/features/ai-first.md`, `docs/guide/mcp/overview.md` if MCP-visible, and `ai/rules/cli.md` if workflow changes |
| CLI command grammar or command availability | `ai/rules/cli.md` or `ai/rules/cli.md`, plus command validation docs if needed |
| New tool or make target | `ai/INDEX.md` Dev Tools or keyword map, plus the owning `docs/contributing/` or `docs/architecture/testing/` page |
| New verification gate or hook | The "Hook-to-Rule Mapping" section below, the rule enforced by the hook, and the relevant make-target documentation |
| New doc or inventory checker | `docs/contributing/documentation-testing.md`, `mk/inventory.mk` quick reference, and `ai/rules/writing.md` if policy changed |
| New test runner or format | `ai/rules/testing.md`, `ai/patterns/functional-test.md` if `.ci`, and the relevant `docs/architecture/testing/` page |
| New runtime dependency | The "Doctor Checks" section below, diagnostic code registration, and a `ze doctor` unit plus functional test |
| New registration or generated inventory | `ai/rules/evidence.md`, `ai/patterns/registration.md`, and registry-backed inventory checks |
| Structural decision or recurring trap | `plan/learned/NNN-*.md`; add `ai/LEARNED-INDEX.md` when the lesson is structural, not task-only |
| New task category or search keyword | `ai/INDEX.md` (task navigation + keyword map) |

**Do not create an isolated rule or doc page that no existing navigation path links to. A rule that agents cannot discover is not a rule.**

### Mechanical Checklist

Before implementation is complete, answer these in the spec, review notes, or handoff:

1. **Where would an agent look first?** Add or update the `ai/INDEX.md` keyword row, `ai/INDEX.md` task row, or both.
2. **What rule prevents regression?** Update the narrowest existing rule. Create a new `ai/rules/*.md` only when no existing rule owns the behavior.
3. **What source of truth prevents drift?** Use a registry, generated inventory, YANG schema, or live binary output. Do not copy static lists.
4. **What verification proves it?** Name the make target, unit test, functional test, hook, or doc validator that catches drift.
5. **What docs explain usage?** Name the exact file and section. Add source anchors for factual `docs/` claims.
6. **What learned record preserves the decision?** Update `ai/LEARNED-INDEX.md` if the learned summary changes future design choices.

### Current Discovery Surfaces

Use these before inventing a new mechanism:

| Need | Existing surface |
|------|------------------|
| Changed-file-aware wiring, doc, command, and inventory gate | `make ze-verify-wiring-docs` |
| Documentation drift and YANG command contracts | `make ze-doc-test` |
| Source-to-document reverse index | `make ze-doc-index`; read `ai/CODE-TO-DOCS.md` |
| RFC MUST requirement to enforcing-test coverage (which tests prove each requirement, plus the backlog) | `make ze-rfc-index`; read `ai/RFC-REQUIREMENTS.md` (the generated two-way ledger; coverage gated by `make ze-rfc-check`, staleness by `make ze-doc-test`) |
| What each package does ("what does what") | `make ze-discovery-index`; read `ai/PACKAGE-MAP.md` |
| Which `.go` files implement a design doc | read `ai/DOCS-TO-CODE.md` (inverse of `// Design:`) |
| Which tests enforce an RFC MUST (and the un-enrolled backlog) | read `ai/RFC-REQUIREMENTS.md` (generated by `make ze-rfc-index`; freshness gated by `make ze-rfc-check` and `make ze-doc-test`) |
| Every learned summary by number (complete) | read `ai/LEARNED-FULL-INDEX.md`; curated by topic: `ai/LEARNED-INDEX.md` |
| Whether a learned summary's cited paths and `plan/learned/NNN` citations still resolve | `make ze-learned-staleness`, folded into `make ze-doc-test`. Its ceiling `plan/.learned-staleness-baseline` shrinks only |
| How to repair a dead learned `## Files` path whose code MOVED rather than went away | `make ze-learned-repath-check` reports, `make ze-learned-repath-apply` writes. It leaves alone any path with several plausible successors or none: a citation repointed at the wrong file reads as true |
| Whether every learned summary spells its section headings the same way | `make ze-learned-normalise-check` reports, `make ze-learned-normalise-fix` rewrites. It is outside `make ze-doc-test` on purpose: heading drift breaks nothing, and a gate that reddens on a colleague's in-flight summary gets switched off |
| Whether every path the instruction corpus names still resolves, and whether a curated-index entry stays a pointer | `make ze-doc-links`, folded into `make ze-doc-test`. It also holds the 120-character pointer budget on `ai/LEARNED-INDEX.md` and rejects a dead `*.sh` or `c_*`/`check_*` name in the hook-describing documents |
| How data flows through a subsystem | read `ai/digests/<subsystem>.md` (living, hand-maintained flow digests; `ai/digests/README.md` lists them); anchors validated by `make ze-digest-check` |
| Plugin, command, YANG, and test inventory | `make ze-inventory`, `make ze-inventory-json` |
| Command inventory | `make ze-command-list`, `make ze-command-list-json` |
| Spec progress | `make ze-spec-status`, `make ze-spec-status-json` |
| Generated plugin imports | `make ze-plugin-imports-check` |
| Whether the tree GIT HOLDS compiles, as opposed to the working tree every other gate reads | `make ze-tracked-build-check` (`REV=<sha>` judges another commit). Runs in `ze-verify`, both modes, and is a structural gate in `scripts/dev/commit_helper.py` |
| Runtime readiness | `ze doctor --json` and `ze explain <diagnostic-code>` |

**If a new feature cannot be found from one of those surfaces or from `ai/INDEX.md`, add the missing discovery link before claiming completion.**

## Doctor Checks

### The Rule

When your implementation introduces any of the following, add a registered doctor check with explicit phase, order, component, dependency, platform, diagnostic-code, and check-function metadata. Ownership is part of the requirement: the package, component, or plugin that owns the dependency MUST own the registration, check function, and unit test.

- **`internal/component/doctor` owns the runner, output contract, functional coverage through the user entry point, and checks that have no narrower owner.**
- **Do not add new runtime dependency checks by appending another direct call to the central `runChecks` list.**
- **Do not add owner-specific registrations in `internal/component/doctor` just because the runner lives there.**
- **Internal plugins (preferred path):** declare doctor checks in `registry.Registration.DoctorChecks`. The doctor runner bridges these at execution time via `checks_plugin_registry.go`. The check function uses `registry.DoctorCheckContext` (Tree and Platform as `any`) and returns `[]rpc.DoctorCheckDiagnostic`. Component is set automatically from the plugin name. See `l2tpauthradius/register.go` for the reference example.
- **Components that are not plugins** (e.g., appliance, web, SSH): use `diagnostic.RegisterDoctorCheck()` from the owning package's init().

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

- **All doctor codes use the `doctor-` prefix: `doctor-<component>-<condition>`.**
- **Register every new code in `internal/core/diagnostic/codes.go` with title, description, and examples. The code must be explainable via `ze explain`.**

### Mechanical Check

- **After implementation, verify the check is registered and explainable: `go test ./internal/component/doctor -run 'TestDoctorCoverageCodesRegistered|TestRunChecksExecutesRegisteredPluginCheck'`**
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

**If no plugin, component, backend, or command package owns the dependency, keep the check and unit test in `internal/component/doctor`. Do not invent an owner package just to satisfy proximity.**

### Test Requirement

Every new doctor check needs both:

| Test type | What it proves | Location |
|-----------|----------------|----------|
| Unit test | The check fires only when the relevant config block is present and emits the registered code | Owning package next to the registration, or `internal/component/doctor` only when there is no narrower owner |
| Functional test | `ze doctor --json <config>` exposes the behavior through the user entry point | `internal/component/doctor` or the existing functional test suite for the user entry |

**Linux-only checks still need Linux-tagged tests and the package must be covered by the QEMU integration target when new `//go:build linux` code is added.**

## Canonical Sources and Sync Direction

### Sync Flows

| Canonical source | Generates | Sync command |
|------------------|-----------|--------------|
| `ai/INSTRUCTIONS.md` | `CLAUDE.md`, `AGENTS.md` | `make ze-ai-instructions` or `make ze-ai-sync` |
| `ai/skills/*.md` | `.claude/skills/*/SKILL.md`, `.codex/skills/*/SKILL.md`, `.agents/skills/*/SKILL.md` | `make ze-ai-sync` |

### Rule Placement

- **Project-wide behavior rules, workflow rules, and agent rules MUST live under `ai/rules/`, not under a tool-specific home directory such as `~/.claude/rules/`.**
- **Tool-specific files are only for behavior that applies exclusively to that tool outside this repository.**
- **`ai/rules/*.md` are tool-agnostic originals. Edit them directly. `.claude/rules/*.md` are Claude-specific originals and must not be used for shared Ze project behavior. These two directories are independent; neither generates the other.**
- **Exception:** `ai/rules/INDEX.md` is generated by `scripts/dev/rules_index.py` from the other rule files' headings and summary lines. Never edit it by hand; run `make ze-rules-index`. To change a rule's one-line overview, edit that rule's `**When:**` line, then regenerate.
- **Exception:** `scripts/dev/rules_condensed.py` generates THREE artifacts from one parse of the other rule files. Never edit any of them by hand; run `make ze-rules-condensed`. To change what they contain, edit the rule itself in the canonical format (`ai/rules/rule-format.md`), then regenerate.

| Artifact | Holds | Imported into every session? |
|----------|-------|------------------------------|
| `ai/rules/TRIGGERS.md` | one routing line per rule: path, severity, `**When:**` trigger. All 97, so no rule is ever invisible | yes |
| `ai/rules/CORE.md` | the condensed directives of the always-on rules. Membership is DERIVED (rungs 1 and 2 of the ladder in `ai/rules/rule-precedence.md`, the ladder itself, any rule with no routable trigger, and any blocking rule no past task description would surface) | yes |

**Membership in `CORE.md` is never edited, because it is never written down.** To make a rule always-on, change what the derivation reads: name it on rung 1 or 2 of the ladder in `ai/rules/rule-precedence.md`. A list of filenames in the generator would read identically until the ladder changed underneath it (`ai/rules/evidence.md`).

### Mechanical Check

**Before editing any file listed in the "Generates" column above, STOP. Find its canonical source in the left column and edit that instead.**

### Drift Detection

**All "Generates" targets above are gitignored, so `git diff` can NEVER show drift for them.** `make ze-ai-check` (also part of `make ze-regen-check`) compares content against a fresh generation; the session-start hook runs it and warns `generated agent files are stale` when a resync is needed. Fix with `make ze-regen`.

### Banned Actions

| Action | Fix |
|--------|-----|
| Editing `CLAUDE.md` directly | Edit `ai/INSTRUCTIONS.md`, run `make ze-ai-instructions` |
| Editing `AGENTS.md` directly | Edit `ai/INSTRUCTIONS.md`, run `make ze-ai-instructions` |
| Editing `.claude/skills/*/SKILL.md` directly | Edit `ai/skills/*.md`, run `make ze-ai-sync` |
| Editing `.codex/skills/*/SKILL.md` directly | Edit `ai/skills/*.md`, run `make ze-ai-sync` |
| Editing `.agents/skills/*/SKILL.md` directly | Edit `ai/skills/*.md`, run `make ze-ai-sync` |

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

**Changing a check:** edit the function in the relevant dispatcher (not a `.sh`), then run `python3 scripts/dev/hook-parity-check.py` to confirm no behaviour changed. If you intentionally changed behaviour, re-bless the golden table with `python3 scripts/dev/hook-parity-check.py --bless` and paste the result back. Also satisfy the "Discovery Updates" section above so future agents can find it.

**Reads never block:** `Read`, `Grep`, `Glob`, `LSP`, `WebFetch`, `WebSearch` are never rejected. Two of them write a non-blocking freshness marker so the `design-without-lsp` gate knows the implementation was investigated: `LSP` (via `mark-lsp-invoked.sh`) and `Read` of a `.go` under `internal/`/`pkg/`/`cmd/` (via `mark-source-read.sh`). Only mutating/executing tools (`Bash`, `Write`, `Edit`, `MultiEdit`, `NotebookEdit`, `Task`, `Agent`) and `ToolSearch` (which loads LSP) are actually gated.

**Every marker is keyed by session id**, and the id is resolved in TWO places that MUST agree: `.claude/hooks/lib/session-id.sh` (`_session_id`, used by the shell hooks that WRITE `.lsp-loaded-*` / `.lsp-invoked-*` / `.source-read-*` / `.session-*`) and a port inside `pretool-writeedit.py` (`session_id()`, which READS them). Disagreement fails CLOSED: the reader looks for a file nothing wrote and blocks work that was actually done. Both read `$CLAUDE_CODE_SESSION_ID` first; an id that is not a safe filename component is rejected by both rather than rewritten. `make ze-hook-test` (section `session-id`) locks this. Before 2026-07-16 neither end had an env lookup, so with no `--session-id` in argv and no access token every concurrent session shared ONE marker set, and `spec-session.sh claim` then silently overwrote another session's spec claim. If you touch either resolver, change BOTH and re-run the test.

### PreToolUse Checks (block before the tool runs)

#### LSP gate (`block-until-lsp.sh`, standalone)

Enforces `session-start.md`. Triggers on `Bash|Write|Edit|MultiEdit|NotebookEdit|ToolSearch|Task|Agent`.
Blocks those tools until `ToolSearch query="select:LSP"` has run this session. BLOCKING. <!-- severity-note: the LSP gate's severity, not this reference page's -->

#### Bash (`pretool-bash.py`)

| Check | Enforces | Triggers on | What it does |
|---|---|---|---|
| destructive-git | `CLAUDE.md` prohibitions | Bash | Blocks git commit/push/reset/restore/clean/merge. Allows `git restore --staged`. BLOCKING. |
| worktree-copy | `CLAUDE.md` prohibitions | Bash | Blocks cp/mv/rsync from `.claude/worktrees/` to main repo. BLOCKING. | <!-- doc-links: ignore (.claude/worktrees/ exists only while a worktree agent is active) -->
| root-build | (build hygiene) | Bash | Blocks `go build` without `-o bin/`. Allows `go build ./...` (check-only). BLOCKING. |
| pipe-tail | `commands.md` | Bash | Blocks a lossy filter (`head`/`tail`/`grep`/`awk`/`sed`/`cat`/`less`/`more`) piped from an EXPENSIVE producer: `make`, `go test\|build\|vet\|run`, `golangci-lint`, `bin/ze*` (with or without `./`), `ze-test`, `pytest`, everything under `scripts/evidence/` (QEMU boots, docker interop labs), and the repo's own gates under `scripts/{dev,checks,docvalid,status}/` whose filename contains check/verify/test/audit/lint/stress/repro -- minus a small cheap-probe set (`verify-status.sh`, `verify-lock.sh`, `verify-summary.sh`, `spec-closure-check.py`), which are status readers CLAUDE.md tells you to run. `\| tee` passes, and cheap commands (`git log \| tail`, `scripts/dev/spec-session.sh wip \| head`) are not its business. Judged **per statement** (`;`, `&&`, `\|\|`, newline), so a cheap pipeline beside an expensive command is fine; a trailing `\|` or a `\\` at end of line is a CONTINUATION, not a boundary, and is flattened first (splitting there put the producer in one statement and the filter in the next, so neither tripped). Quote- and `$( )`-blind by design: a `;` inside quotes or a command substitution splits a statement, and a producer inside `bash -c "..."` is not seen. BLOCKING. |
| poll-loop | `commands.md` | Bash | Blocks an unbounded wait loop: `while`/`until` paired with a `sleep` call, or with `pgrep` in the loop CONDITION. The bound is credited **per loop, in the statement that loop's keyword opens**, so an earlier `timeout` elsewhere on the line does not disarm it, a bounded loop does not cover a later unbounded one, and a `-timeout` FLAG (`go test -timeout 300s`) never counts -- crediting any of those made the guard fail open. A keyword inside a search argument is TEXT (`grep`, `rg`, `git log -S`), so the rule stays auditable from Bash. `while read` and a one-shot `pgrep` are not its business. Two limits are by construction: a loop inside a script file is unseen, and a loop that bounds itself in its own condition (`while [ $SECONDS -lt 300 ]`) is still refused, since the arithmetic is not decidable here. Quoting a loop to RUN-shape is rejected like every text-matched check (`plan/learned/HOOK-FRICTION.md` F22). BLOCKING. |
| system-tmp | `testing.md` | Bash | Blocks access to `/tmp`; must use project `tmp/`. BLOCKING. |
| test-deletion | `testing.md` | Bash | Blocks `rm`/`git checkout` of test files. BLOCKING. |

The five commit-time gates (spec-audit, deferral-in-diff, deferral-unassigned, wiring-at-commit, doc-drift) used to sit here but gated on the literal `git commit` string, which the sanctioned commit path never sends and `destructive-git` blocks when it does. They are now **creation-time gates in `scripts/dev/commit_helper.py`**. See "Commit-time gates" below.

`golangci-lint run` also runs standalone on `Bash(git commit:*)`.

#### Write/Edit (`pretool-writeedit.py`)

| Check | Enforces | Triggers on | What it does |
|---|---|---|---|
| design-without-lsp | `session-start.md`, `evidence.md` | design/spec `.md` | Blocks edits to `plan/design-*.md` / `plan/spec-*.md` unless the implementation was investigated in the last 30 min: LSP invoked OR a `.go` under `internal/`/`pkg/`/`cmd/` was read. BLOCKING. | <!-- doc-links: ignore (hook trigger patterns, files may not exist) -->
| c_model_phase | `planning.md` | code suffixes, never `.md` | Blocks an implementation edit made on a planning/review model. The payload carries no model, so it reads the tail of `transcript_path`. Escape: record the operator's decision in `tmp/session/.model-ack-<sid>`. BLOCKING. |
| pre-write-go | `architecture.md` | `internal/**/*.go` | Blocks without proper session state. BLOCKING. |
| source-edit-spec-not-in-progress | `planning.md` | source/test/learned | Blocks edits when selected spec is not `in-progress`. BLOCKING. |
| encoding-alloc | `performance.md` | wire-encode `.go` | Blocks `make()`/`append()`/`Bytes()`/`Pack()` in wire-facing code. BLOCKING. |
| format-alloc | `performance.md` | BGP format `.go` | Blocks `strings.Join`/`Builder`/`NewReplacer`/`ReplaceAll` (+ `fmt.Sprintf`/`Fprintf`, `strconv.Format*`) in the guarded format files. Comment lines exempt. BLOCKING. |
| sprintf-new | `performance.md` | `.go` | Blocks new `fmt.Sprintf`/`Fprintf`/`Printf`. Allows `fmt.Errorf`. BLOCKING. |
| legacy-log | `go-standards.md` | `.go` | Blocks `log.Printf` / legacy `log` package. BLOCKING. |
| panic-error | `go-standards.md` | `.go` | Blocks `panic()` except `unreachable`/`not implemented`/`TODO`/`BUG`/`impossible`. BLOCKING. |
| ignored-errors | `go-standards.md` | `.go` | Blocks `_, _ =` error-swallowing. BLOCKING. |
| silent-ignore | `config.md` | `.go` | Blocks empty `default:` cases. BLOCKING. |
| temp-debug | `go-standards.md` | `.go` | Blocks debug-MARKER prints (`DEBUG`/`TRACE`/`>>>`/`<<<`/`***`/`XXX`/`FIXME`) via `fmt.Print*`/`Fprint*`, bare `println(...)`, and short bare `fmt.Println("...")` in production Go. Plain `os.Stderr` output is ALLOWED -- it is the CLI's interface, and `cli.md` prescribes it. BLOCKING. |
| os-exit | `cli.md` | `.go` | Blocks `os.Exit()` outside `main.go`/`register.go`/`scripts/`. BLOCKING. |
| layering | `no-layering.md` | `.go` | Blocks backwards-compat/layering patterns. BLOCKING. |
| exabgp-in-engine | `go-standards.md` | `.go` | Blocks ExaBGP awareness outside `exabgp/`. BLOCKING. |
| version-config | `config.md` | config files | Blocks version fields in config. BLOCKING. |
| nolint-abuse | `quality.md` | `.go` | Blocks `//nolint:` without justification. BLOCKING. |
| lint-exclusions | `quality.md` | `.golangci.*` | Blocks adding lint exclusions. BLOCKING. |
| and-functions | `architecture.md` | `.go` | Warns about `func *And*()` names. Advisory. |
| init-register | `architecture.md` | `.go` | Blocks `init()` outside `register.go`. BLOCKING. |
| yagni-violations | `architecture.md` | `.go` | Blocks speculative-feature comments. BLOCKING. |
| fake-bufhandle | (pool correctness) | `.go` | Blocks `BufHandle{Buf: make(...)}` outside `testPoolBuf`. BLOCKING. |
| observer-sys-exit | `testing.md` | `.ci` | Warns about `sys.exit(1)` in observers without `runtime_fail`. Advisory. |
| ci-sleep-justification | `testing.md` | `.ci` | Warns when a `time.sleep(` is introduced with no comment above/trailing it. Advisory (blocking gate is `make ze-verify-wiring-docs`). |
| hardcoded-commands | `evidence.md` | `.go` | Blocks hardcoded command-list literals. BLOCKING. |
| switch-dispatch | `plugins.md` | `.go` | Blocks `switch args[0]` subcommand dispatch; use `subdispatch.New()` + `Register()`. BLOCKING. |
| json-kebab | `cli.md` | `.go` | Blocks non-kebab-case JSON tags. BLOCKING. |
| goroutine-lifecycle | `goroutine-lifecycle.md` | hot-path `.go` | Blocks `go func()` in reactor/event/dispatch/hub/wire/message. BLOCKING. |
| require-design-ref | `go-standards.md` | `.go` | Blocks Go files without `// Design:` comment. BLOCKING. |
| require-related-refs | `go-standards.md` | `.go` | Blocks missing/stale `// Related:`/`// Detail:`/`// Overview:` refs. BLOCKING. |
| test-weakening (Edit/Write/MultiEdit) | `testing.md` | test files | Blocks deleting OR weakening tests: removed funcs/cases/assertions, added `t.Skip`, `require`->`assert` downgrade, commented-out asserts, `ignore` build tag. Escape: `// test-relax: <reason>`. BLOCKING. |
| rfc-tagged-test (Edit/Write/MultiEdit) | `testing.md`, `ai/skills/ze-rfc.md` | any tag CARRIER holding `RFC requirement:` -- `_test.go`, `.ci`, `.et`, an interop `check.py` | The guard is `_rfc_tagged_change_err`, called from `c_test_weakening`. The row label on the left is the registry name, not a function name, so grep for the function. Blocks ANY behavior change to a test that proves an RFC obligation. It runs BEFORE test-weakening. `// test-relax:` does NOT satisfy it, because self-service justification is not user approval. Also blocks DELETING the tag (checked first, since a tag is a comment and the behavior comparison would pass its removal). Scope is the ENCLOSING test function (`_enclosing_tagged_scope`, which now delegates to `scripts/dev/rfc_tagged_scope.py`), not the edited hunk. So a tag on the doc comment still governs a body edit. Untagged functions in the same file are unaffected. A tag outside every function, such as a hoisted table, widens scope to the whole file. Every occurrence of a hunk is considered, so `replace_all` cannot reach a tagged copy unseen. Comment/format edits pass -- `#` counts as the comment syntax for `.ci`, `.et` and `.py`; a rename blocks. A `.go` edit made ONLY of import lines passes too (`_import_only_go_edit`): an import cannot weaken an assertion, and without it GROWING a tagged file always cost an approval, because new tests need new imports and an import block sits outside every function so the scope widens to the whole file. Every non-blank line on both sides must be import-shaped, so an assertion smuggled into the same edit still blocks, and the tag-removal check runs first so a tag cannot ride out on the exemption. Escape: `// rfc-test-change-approved: <date> <what the user approved>`, and only the user may authorize it. **The marker must sit in the replacement text of the edit itself**, since that is the only text the check reads; the same marker elsewhere in the file does not satisfy it. BLOCKING. **The carrier list is derived** from the shared leaf. `TestTaggedScopeCoversEveryCarrier` holds it against the scanner's own `CARRIERS`. Until 2026-07-29 the predicate was a literal covering `_test.go` and a `/test/` `.ci` only. The two interop `check.py` files that `plan/learned/1296-rfcgate-2-evidence.md` admits as evidence therefore carried RFC obligations this guard could not see. |
| system-tmp (path) | `testing.md` | any | Blocks writing to `/tmp`. BLOCKING. |
| generated-files | "Canonical Sources and Sync Direction" above | `CLAUDE.md`/`AGENTS.md` | Blocks editing generated files. BLOCKING. |
| claude-plans | `.claude/rules/planning.md` | Write | Blocks `.claude/plans/` and `~/.claude/plan/`. BLOCKING. | <!-- doc-links: ignore (banned location, deliberately nonexistent) -->
| check-existing-patterns | `architecture.md` | new `internal/**/*.go` | Blocks duplicate exported type/func in same package. BLOCKING. |
| check-existing-tests | `architecture.md` | new test files | Warns about similar existing tests. Advisory. |
| enforce-naming | `writing.md` | new files | Warns on wrong file naming. Advisory. |
| throwaway-tests | `testing.md` | Write | Blocks test files in `/tmp` and throwaway locations. BLOCKING. |
| utils-package | `architecture.md` | Write `.go` | Blocks `utils/`/`helpers/`/`common/`/`misc/` packages. BLOCKING. |
| require-test-first | `testing.md` | new `.go` | Warns when creating impl without a test file. Advisory. |
| require-docs-read | `planning.md` | new spec | Warns when writing a spec without session-state evidence. Advisory. |

> **format-alloc is now live** (enabled 2026-07-09, spec-followup-hooks). It is
> `c_format_alloc` in `.claude/hooks/pretool-writeedit.py`. The retired shell
> version used bash-4 `declare -A`, which the macOS bash
> 3.2 shebang could not run, so it exited 0 and never enforced anything. The
> guarded list is now current (`bgp/attribute/text.go` removed with the attribute
> package in `3e66070f8`; `bgp/format/json.go` added) and comment lines are exempt
> like `sprintf-new`. Its incremental value over `sprintf-new` (which already bans
> `fmt.Sprintf`/`Fprintf` + `strconv.Format*` everywhere) is the `strings.Join`/
> `Builder`/`NewReplacer`/`ReplaceAll` bans. Covered by
> `scripts/dev/hook-fixture-check.py` (`format-alloc-*`).

### PostToolUse Checks (run after the tool completes)

| Check | File | Enforces | Triggers on | What it does |
|---|---|---|---|---|
| mark-lsp-invoked | `mark-lsp-invoked.sh` | `session-start.md` | LSP | Writes `.lsp-invoked` freshness marker for the design-without-lsp gate. |
| mark-source-read | `mark-source-read.sh` | `evidence.md` | Read | Writes `.source-read` freshness marker when a `.go` under `internal/`/`pkg/`/`cmd/` is read, so reading the producing code satisfies the design-without-lsp gate. Non-blocking. |
| mark-agent-spawned | `mark-agent-spawned.sh` | `planning.md` | Agent, Task | Writes `.agent-spawned-<sid>` so the Stop hook can tell a supervising main thread from one that ran the phase inline. Fires in the PARENT (subagents inherit its session id), so the marker always lands on the supervising session. Non-blocking. |
| auto-lint | `posttool-writeedit.py` | `go-standards.md` | `.go` Write/Edit | `gofmt`/`goimports -w`, then **one** `golangci-lint --new-from-rev=HEAD` pass (flags only issues this edit introduced). BLOCKING on lint failure. |
| auto-py-format | `posttool-writeedit.py` | (code style) | `.py` Write/Edit | `ruff format` + `ruff check`. Non-blocking. |
| validate-spec | `validate-spec.sh` | `planning.md` | `plan/spec-*.md` | Validates required sections/format. Exit 2 blocks a structurally invalid spec; both `→` and `->` wiring rows accepted. |
| file-size | `posttool-writeedit.py` | `go-standards.md` | `.go` | Warns >1000 lines. Advisory. |
| warn-deferral | `posttool-writeedit.py` | `planning.md` | `.md` | Warns on deferral language in doc edits. Advisory. |
| require-rfc-reference | `posttool-writeedit.py` | `go-standards.md` | `.go` | Suggests `// RFC:` header. Advisory. |
| require-test-docs | `posttool-writeedit.py` | `testing.md` | `_test.go` | Warns about missing `VALIDATES:`/`PREVENTS:`. Advisory. |
| require-fuzz-tests | `posttool-writeedit.py` | `testing.md` | wire `.go` | Warns about `Parse*` without `Fuzz*` tests. Advisory. |
| vague-names | `posttool-writeedit.py` | `architecture.md` | `.go` | Warns about `Data`/`Info`/`Result`/... names. Advisory. |
| boundary-tests | `posttool-writeedit.py` | `testing.md` | `.go` | Warns about numeric validation without boundary tests. Advisory. |

> **validate-spec.sh is fixed** (2026-07-09, spec-followup-hooks) and kept
> standalone. It previously matched only the Unicode arrow `→` in the Wiring Test
> table, so an ASCII `->` spec produced an empty `grep` pipeline that exited 1 and
> `set -e` aborted the script before the output stage, swallowing every queued
> error (a silent non-blocking exit 1). Both arrow conventions are now accepted
> and the `WIRING_ROWS=` assignment is guarded with `|| true`, so the script
> always reaches its verdict: exit 2 for a structurally invalid spec, exit 0
> otherwise. A survey over all `plan/spec-*.md` (spec-followup-hooks AC-4)
> confirmed zero crashes and zero arrow false-positives. It stays out of the
> dispatcher (see spec Key Design Decisions). Covered by
> `scripts/dev/hook-fixture-check.py` (`validate-spec-*`).

`make ze-verify` separately runs `ze-verify-wiring-docs` (wiring/doc-drift gate); that is a Make target, not a Claude hook.

### Changed-file gates inside `ze-verify-wiring-docs`

Also Make targets, not Claude hooks. All are changed-file scoped: a session owns the files it touches, not the whole tree.

| Check | Enforces | Triggers on | What it does |
|---|---|---|---|
| `check_ci_sleep_ratchet` | `testing.md` | changed `test/**/*.ci` | Caps how MANY `time.sleep(` calls exist tree-wide against a committed delta baseline. BLOCKING. |
| `check_ci_sleep_justification` | `testing.md` | changed `test/**/*.ci` | Caps how many sleeps are UNEXPLAINED: each needs a comment above or trailing it. BLOCKING. |
| `check_known_failure_load_excuses` | `completion.md` | changed `plan/known-failures/*.md` | Rejects a shard blaming host load ("under load", "loaded host", "load average", "load-sensitive", "passes in isolation", "resource contention", "contended host"). `README.md` / `RESOLVED.md` exempt. BLOCKING. |
| `check_ci_log_subsystem_keys` | `testing.md`, `config.md` | changed `test/**/*.ci` | Rejects a `ze.log.<subsystem>` key whose subsystem contains a hyphen that is not declared literally in Go. An internal plugin's logger name is `CanonicalSubsystemName` of its registry name (every hyphen becomes a dot) and `getLogEnv` splits on `.` only, so `ze.log.bgp.adj-rib-in` sets nothing and the level silently stays at the WARN default. Scan is tree-wide; `#` comment lines exempt. BLOCKING. |

### Prose gate (ASD-STE100)

Make targets and a commit-time gate, not Claude hooks. HEAD is the baseline and the comparison is per file, so a document nobody touched can never fail.

| Check | Enforces | Triggers on | What it does |
|---|---|---|---|
| `ste_problems` (`commit_helper.py`) | `writing.md` | `commit_helper.py create` with any `.md`, `.go`, or `.yang` in the commit | Runs `ste_check.py --check` over the FILES OF THAT COMMIT and PRINTS the six banned ASD-STE100 habits that grew against HEAD. Advisory: STE is a guideline, so this never refuses a commit. Commit scope is deliberate: several sessions share this checkout, so a tree-wide prose gate judges a colleague's in-flight sentences and gets switched off. BLOCKING. |
| `make ze-ste-check` | `writing.md` | on demand, before you prepare a commit | The same comparison over every changed file in the working tree. It prints the file, the habit, and only the new findings. Surfaces are Markdown in `docs/`, `ai/`, `plan/`, and the root, prose comments in `.go`, and `description` strings in `.yang`. Renames follow `-M`, so a moved legacy document does not report its inherited content as new. Deliberately NOT in `ze-doc-test`. Whole-tree report: `make ze-ste-review`. |

### Commit-time gates (`scripts/dev/commit_helper.py`)

These are NOT Claude hooks. They run when `commit_helper.py create` generates the commit script, which is the only sanctioned commit path (run the script at the path its `script=` line prints). The helper already knows the exact add/remove set of the commit, so the gates inspect that instead of the staging area. BLOCK gates raise (exit 2, no script written); WARN gates print to stderr and let the script be written.

| Gate | Enforces | Severity | What it does |
|---|---|---|---|
| verify-status / structural-gate | `git-safety.md` | BLOCK | Refuses a script over a non-green `ze-verify` (structural reds are unbypassable). |
| ste (`ste_problems`) | `writing.md` | WARN | Runs `ste_check.py --check` over the commit's own `.md`, `.go`, and `.yang` files and prints the six banned ASD-STE100 habits that grew against HEAD. It never refuses. Legacy prose in a file you touched costs nothing, because each file is compared with its own HEAD version. Scoped to the commit for the same reason discovery-index materializes a commit view: a concurrent session's uncommitted prose must not block your commit. Prints the file, the habit, and only the new findings. |
| discovery-index | "Discovery Updates" above | BLOCK | Refuses when a generated index (PACKAGE-MAP / DOCS-TO-CODE / LEARNED-FULL-INDEX) would be left incoherent. Judged on the tree the COMMIT PRODUCES (HEAD + adds - removes), materialized under `tmp/commit-view-*` and checked with the commit's OWN generators via `--root`, never on the working tree: a concurrent session's uncommitted sources must neither block your commit nor be swept into your index. **Every** index whose generator exists is verified, not just the ones the commit visibly feeds -- `package_map` keys its rows on directory existence, so a new `.go` can drift PACKAGE-MAP while feeding only DOCS-TO-CODE. Cost: the view is built on EVERY commit the gate examines, not only ones touching an index source, because the candidate set comes from generator existence, about 5.5s total (~2s working-tree freshness, ~3.6s to materialize and check). If the view cannot be built it does NOT fail closed: it warns on stderr and falls back to the working-tree verdict, which is the only evidence left. BLOCKING. |
| deferral-unassigned | `planning.md` | BLOCK | Folds over every shard in `plan/deferrals/` and flags an open row with no Destination. |
| deferral-in-diff | `planning.md` | BLOCK | Blocks when the commit's added lines contain deferral language and no `plan/deferrals/` shard is part of the commit (diff computed in a throwaway git index). |
| spec-audit | `planning.md` | BLOCK | Blocks the closure commit (the one adding the claimed spec's `plan/learned/NNN-<stem>.md`) when that spec's `## Pre-Commit Verification` section is unfilled. Keyed to `spec-session.sh current`; no claim → skips. |
| wiring-at-commit | `completion.md` | WARN | Warns when `internal/plugins/**/*.go` is committed with no `.ci`. |
| doc-drift | `writing.md` | WARN | Runs `scripts/docvalid/doc_drift.go`; warns on drift. |

### Hook tests (`make ze-hook-test`)

| Runner | Covers |
|---|---|
| `scripts/dev/hook-parity-check.py` | Golden exit-code regression for the three consolidated dispatchers. `--bless` regenerates the golden; re-bless only intentionally changed cases. Fixture dirs live under `~/.cache` (a `/tmp` or in-repo path trips `system-tmp`/`throwaway-tests` or the module lint and diverges from the golden). |
| `scripts/dev/hook-fixture-check.py` | Behaviour the golden table cannot isolate: `c_format_alloc`, `validate-spec.sh`, the `commit_helper.py` commit-time gates over git-initialized fixtures, and the 35 `delegation` fixtures. Those 35 pin what no other test reaches. The `Stop` array registration and its order. BOTH ends of the claim lifetime: alive past turn one, released at `SessionEnd`, kept across a resume. The two stop-phrase tiers. The markup filter that must fail toward scanning MORE. Deleting the release line once left the whole suite green. Sections selectable with `--only`. |

### Session Lifecycle Hooks

**UserPromptSubmit stdout reaches the model. UserPromptSubmit stderr does not.** A reminder that must land in the context writes to stdout. A banner that must cost no context tokens writes to stderr, as `compaction-reminder.sh` does. The two stdout reminders below fire on every turn, so each one stays a single line.

| Hook | Event | What it does |
|---|---|---|
| `session-start.sh` | SessionStart | Prints status summary. Creates session marker. |
| `compaction-reminder.sh` | UserPromptSubmit | Detects compaction; reminds to read `post-compaction.md`. Writes to **stderr**, so it costs no context tokens. |
| `verify-claim-reminder.sh` | UserPromptSubmit | Emits one **stdout** line per turn. Verify a claim about code by reading the function that PRODUCES the behavior, not the caller. Label an unread claim unverified. Name the file and the symbol, and use a line number only when the line IS the fact. Report the conclusion, not the search. Enforces `ai/rules/evidence.md` and `ai/rules/writing.md`. A banner read once at session start does not survive to the turn that makes the claim, so this lands in fresh context. |
| `delegation-reminder.sh` | UserPromptSubmit | Emits one **stdout** line per turn: subagent delegation needs no permission in this repository. The harness appends the guard "Do not call the AgentTool unless the user requested it" to the END of the system prompt, where it wins on position. `ai/INSTRUCTIONS.md` "STANDING REQUEST: delegate to subagents" IS the request that guard defers to, but it sits far earlier in the same prompt and loses. UserPromptSubmit stdout is the only harness position that lands after the whole system prompt, so the counter goes there. Unconditional by design: a conditional reminder adds a "did the condition fire" failure mode, and the reminder is correct on every turn. Enforces `ai/rules/planning.md`. Fixtures: `python3 scripts/dev/hook-fixture-check.py --only delegation-reminder`. |
| `block-premature-stop.sh` | Stop (**first**, ahead of `session-end-summary.sh`) | **Live and BLOCKING** since 2026-07-31. Four gates run. All but the first need a session that CLAIMED a spec. (1) **Stop-phrase scan**, exit 2. `PHRASES` covers ownership-dodging, premature handoff and permission-seeking, and it always blocks. `COMPLETION_PHRASES` (`what next`, `what would you like`) blocks ONLY while a claimed spec is `in-progress`, because `.claude/rules/session-start.md` REQUIRES that question once the task is done. A phrase inside backticks or a closed fence is quoted, not used, and does not block. (2) **Spec-closure gate**, exit 2, when `spec-closure-check.py --spec` reports the spec completed but not closed. That check runs well inside the hook's 10s timeout, and a slower one would fail the gate open with no signal. Two escapes: run commit B, or write `tmp/session/.closure-ack-<stem>`. (3) **Spec still in-progress** warning, exit 1. (4) **Delegation nudge**, exit 1, when the session claimed a spec and spawned no agent. A harness retry (`stop_hook_active`) bounds the phrase scan ALONE, whose only escape is rewording. The other three stay armed, because each has an escape of its own. Gates 2 to 4 need the claim marker to outlive turn one, so the release moved from `Stop` to `SessionEnd`. Behaviour is pinned by 35 fixtures: `python3 scripts/dev/hook-fixture-check.py --only delegation`. |
| `session-end-summary.sh` | Stop | Writes session state snapshot. It no longer releases the spec claim: that moved to `SessionEnd` (`session-end-scratch.sh`) so the claim survives the turn-by-turn `Stop`. |
| `session-end-deferrals.sh` | Stop | Prints open deferral count. Advisory. |
| `pre-compact-save.sh` | PreCompact | Saves session state before compaction. |
| `subagent-context.sh` | SubagentStart | Injects project context into spawned agents, PLUS the parent session's claimed spec (with its Status) and the subagent contract from `ai/rules/planning.md`. This exists to make delegating cheap: the rule requires the main thread to hand every agent its spec, phase, and rules, and when that is manual per-spawn work, delegating costs more than working inline and the rule loses. |

### Pre-Flight Checklist by File Type

(All checks below now run inside the dispatchers; behaviour is unchanged.)

#### Any `.go` file under `internal/`

pre-write-go, auto-lint, encoding-alloc (wire paths), sprintf-new, legacy-log, panic-error, ignored-errors, silent-ignore, temp-debug, os-exit, init-register, yagni-violations, json-kebab, require-design-ref, require-related-refs, file-size.
Hot-path files (reactor/event/dispatch/hub/wire/message) also: goroutine-lifecycle, fake-bufhandle.

#### Test files (`_test.go`, `.ci`)

test-weakening (if removing/weakening tests), check-existing-tests (if new), require-test-docs, boundary-tests, observer-sys-exit (`.ci` with Python).

#### Spec files (`plan/spec-*.md`)

validate-spec, design-without-lsp (needs recent implementation investigation: LSP invoked or a `.go` under `internal/`/`pkg/`/`cmd/` read), require-docs-read (if new), source-edit-spec-not-in-progress (if spec not `in-progress`).

#### Python files (`.py`)

auto-py-format (ruff format + check).

#### Commits

A Bash `git commit` is blocked outright by destructive-git. Commit via `scripts/dev/commit_helper.py create`, then `bash` on the path its `script=` line prints; the creation-time gates (verify-status, discovery-index, deferral-unassigned, deferral-in-diff, spec-audit block; wiring-at-commit, doc-drift warn) run then. See "Commit-time gates" above.

## Ze Project Knowledge

### Project Knowledge (not in other rules)

- **Family registration** is dynamic via `PluginRegistry.Register()` -- never enumerate, validate format only.
- **Config pipeline**: File -> Tree -> `ResolveBGPTree()` -> `map[string]any` -> `reactor.PeersFromTree()`. Files: `internal/component/bgp/config/{resolve,peers}.go`, `.../reactor/config.go`.
- **Linter hook** (`auto-lint` in `.claude/hooks/posttool-writeedit.py`) runs gofmt + `goimports -format-only` on Edit/Write (imports are no longer auto-removed) -- still add import + usage in the same edit.
- **Arch-0**: 4 components (Engine, ConfigProvider, PluginManager, Subsystem). Subsystem != Plugin (BGP daemon = subsystem; bgp-rib/rs/gr = plugins). Stream system = pub/sub backbone (`internal/component/plugin/server/dispatch.go`). Interfaces in `pkg/ze/`. Umbrella: `plan/learned/425-arch-0-system-boundaries.md`.
- **YANG choice/case**: `mandatory true` and inner-choice exclusivity NOT enforced by the walker. Plugins using `choice` add Go-side validation in their parser. `ze config validate` does not invoke `OnConfigVerify`.
- **Constants for command/status names** -- literals catch typos at compile time. Editor commands: `internal/component/cli/model.go`. Plugin status: `plugin.StatusDone`/`StatusError`.
- **Proximity**: `bgp/handler/` is a middleman; handlers belong in `bgp/plugins/`. ALL RPCs need YANG. See `ai/rules/plugins.md`.
- **LSP** at session start for Go nav -- more precise than grep for call chains and interface impls.
- **Inventory**: `make ze-inventory [--json]` imports `plugin/all` and queries real registries. Use for plugin counts, RPC totals, family coverage.
- **SDK type aliases** (`pkg/plugin/sdk/sdk_types.go` re-exporting `rpc.*`) are intentional -- external plugins import only `sdk`. Not identity wrappers.
- **No filtered/noexport route tracking** -- Ze does not store import-filtered or export-filtered routes (unlike BIRD's "import keep filtered on"): the RIB pipeline has scope keywords (sent/received/sent-received) and filter stages, but no "filtered" scope. The birdwatcher-compatible endpoints `/routes/filtered/{name}` and `/routes/noexport/{name}` return empty lists for compatibility; if filtered tracking ever lands, point them at the real store.
- **Gokrazy appliance owns process lifecycle** -- ze deploys as a gokrazy appliance: no systemd, no init system, no package manager. Any external process ze depends on (VPP or future dependencies) must be exec'd, supervised, and cleaned up by ze itself; never design around an OS-level process manager.
- **Stress injector is in-memory Go** (decision 2026-04-16) -- the BGP UPDATE stream for stress scenarios 01-05 is generated in memory inside `ze-test peer --mode inject` and streamed over the TCP socket after the OPEN handshake; no file on disk, no bngblaster. Extend the Go injector for new scenarios (pool-friendly byte builder, single pre-allocated buffer, single TCP writer with keepalive goroutine). The standalone byte-level oracle and BNG Blaster have been removed now that the Go builder is trusted; `test/stress/` is the Python harness (`harness.py`/`run.py`/`scenarios/`).
- **CLI dispatch discoverability gaps** (2026-03-30 live debugging; spec candidates): (1) no one-shot command against a RUNNING daemon (`ze cli -c "summary"` shape) -- `ze show`/`ze run` use SSH (`sshclient.ExecCommand`) internally but expose no shell one-liner; the offline-config half is covered by `ze config show <file> [path...]` (49f04ffd3). (2) `ze help --ai --api` prints YANG RPC names (`ze-bgp:summary`), not the dispatch strings users type. (3) No way to list the Dispatcher's match keys. `reactor.ExecuteCommand()` accepts strings undiscoverable without reading source; highest-value fix is the one-shot daemon command (SSH port 2222, credentials from the zefs database).

### Mistake Log

One-line lesson + rule pointer. Full root-cause in the linked learned summary.

- **"Linux-only tests can't run on this macOS host / need hardware" is a LIE** (RECURRING, ZERO TOL). Ze HAS a QEMU Alpine-VM harness: `option=needs-linux` `.ci` tests SKIP on native darwin and RUN under `make ze-qemu-needs-linux-test` / `ze-qemu-all-test`; kernel/netlink/nft/veth/loop tests run via `make ze-qemu-integration-test` and the `ze-qemu-<feature>-test` targets. A Linux-only test that FAILS (not skips) on native darwin is missing its `option=needs-linux` marker (fix: add it, then run it in QEMU), never "environmental / unfixable here". NEVER attribute a Linux test red to "darwin env" or "needs docker/qemu we don't have": we HAVE QEMU. Run it. `ai/rules/platform-linux.md`.
- **Feature not wired** (RECURRING, ZERO TOL). Unit tests != wiring. Name the user entry point. `ai/rules/completion.md`.
- **Daemon command without offline CLI** (sysctl-0). Every `CommandDecl` plugin needs `cmd/ze/<name>/` offline entry point.
- **Wrong production path** (rib-04). Grep ALL implementations; trace the consumer's call chain.
- **Count-only test assertions** (addpath-rib). Assert on content (keys/values), not `Len()`.
- **Wrapper struct pattern** (alloc-4). Pass raw bytes + existing iterators. Never wrap data in accessor types.
- **Tests-pass != done** (RECURRING). Tests are step 10 of 12. Continue to docs/spec/summary/audit. `ai/rules/quality.md`.
- **Mechanism-not-behavior test** (prefix-limit). Assert the AC, not a code-path proxy. No-op passes = wrong test. `ai/rules/testing.md`.
- **"Pre-existing" failures** (RESOLVED). Fix in-session after primary task; log to `plan/known-failures/` if >10 min. `ai/rules/completion.md`.
- **Plugin placement anchor bias** (jsonrpc). "Delete the folder" test. Cross-cutting -> `internal/component/`. Domain -> `bgp/plugins/`. Infra -> `internal/core/`.
- **Docs from assumption** (RECURRING). Read source before any factual claim. `ai/rules/writing.md` Source Anchors.
- **Spec deleted without committing** (lg-overhaul, ZERO TOL). TWO commits: (A) code+spec, (B) `git rm` spec + add summary. `ai/rules/planning.md`.
- **Reinventing repo contents** (lg-overhaul). Grep before writing new infra; `third_party/` and components often already have it. `ai/rules/architecture.md`.
- **Spec claimed complete with gaps** (lg-0..4). Learned summary with "future X" = spec NOT done. Audit each AC. `ai/rules/completion.md`.
- **Stale deferrals** (redist-phase2). Grep code before creating phase-N spec from open deferrals. `ai/rules/planning.md`.
- **Worktree copy into main** (ZERO TOL). Commit in worktree; merge/cherry-pick only. `check_worktree_copy` in `.claude/hooks/pretool-bash.py` enforces.
- **Same-day blocker fix** (cmd-4, RECURRING). Real adversarial review: race on reactor code, grep renamed-name consumers, grep sibling call sites, break production to confirm .ci test fails. `ai/rules/quality.md`.
- **Substring collision in bulk edits** (iface-tunnel). Longest prefix first, or add non-name context. Grep for mangled names after.
- **Vendor != upstream** (iface-tunnel). Verify against `vendor/<lib>/`, not upstream docs. Cite vendor path in the spec.
- **Naive reconciliation drops live state** (iface-tunnel). Diff against previous config; act on the delta. Pass `previous` explicitly.
- **Invented config shape** (iface-tunnel). Grep existing `*-conf.yang` for the closest analog before defining new endpoint shapes.
- **Scratch `.go` in `tmp/`** (iface-tunnel). `go test ./...` walks `tmp/`. Research agents use `.txt` or build-tagged dirs.
- **CLI grammar from container nesting, not wire method** (as112-cli-audit). Operator-facing command words come from the YANG `container` tree; `ze:command "ze-X:Y"` is the INTERNAL RPC name and is deliberately different (e.g. `ze-bgp:peer-teardown` = command `request peer teardown`). Never infer command syntax from wire-method names. Top-level operational verb is `request` (`request <object> <action>`); reads are `show`/`monitor`. `ai/rules/writing.md`.
- **ExaBGP migration sync** (exabgp-compat-sync). When ExaBGP adds a new SAFI or route type, three things need updating: (1) `exabgp.yang` schema container, (2) `flexSafis` list or a dedicated `convert*ToUpdate` in `migrate_routes.go`, (3) compat test files (`.ci` + `.conf`). `ai/patterns/bgp-family.md` Section 5b.

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
Proposed fix: [specific ai/rules, ai/INDEX, plan/learned, docs, or hook change]
```

### Timing

- Report as soon as you can describe the pattern. Do not wait until the end of the session.
- **Reporting in chat is not filing.** Chat scrolls away and the next session never sees it, so hook and tooling friction is not reported until it is written to `plan/learned/HOOK-FRICTION.md` in the Format above; a finding you only pass to the next agent in a handoff is folklore, not a record.
- If the user task is still in progress, keep working after reporting unless blocked or the rule change would alter scope.
- If the pattern changes a project workflow, add or update the narrowest rule or learned record before claiming completion.

### Do Not Report

- Things that are simply unfamiliar before reading the relevant docs.
- Intentional deviations already documented in specs or rationale files.
- One-off issues that will not recur and expose no rule gap.
