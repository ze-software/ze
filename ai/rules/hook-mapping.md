# Hook-to-Rule Mapping

**When:** looking up which check enforces a rule, or why a hook rejected an edit
**Severity:** advisory

## Directives

Quick reference: which checks enforce which rules, and when they trigger.
Consult this BEFORE writing code to proactively comply, rather than
fixing after rejection. For hook false positives and workarounds, see
`plan/learned/HOOK-FRICTION.md`.

## Architecture: checks live in three Python dispatchers

The per-check shell hooks were consolidated into one Python dispatcher per
trigger, so a tool call pays one process instead of dozens. The checks below are
functions inside these files, not separate scripts:

| Dispatcher | Runs on | Contains |
|---|---|---|
| `.claude/hooks/pretool-bash.py` | PreToolUse `Bash` | every Bash check below |
| `.claude/hooks/pretool-writeedit.py` | PreToolUse `Write\|Edit\|MultiEdit\|NotebookEdit` | every Write/Edit check below |
| `.claude/hooks/pretool-agent-skill.py` | PreToolUse `Task\|Agent` | two gates: skills-over-raw-agents (`ai/rules/agent-tooling.md`), and review-runs-on-Opus-5 (`ai/rules/model-selection.md`) |
| `.claude/hooks/posttool-writeedit.py` | PostToolUse `Write\|Edit` | the formatters (gofmt/goimports/golangci, ruff) + cheap advisory checks |

Still standalone (single-purpose or deliberately not folded):
`block-until-lsp.sh`, `validate-spec.sh` (see note below), `mark-lsp-invoked.sh`,
`mark-source-read.sh`, and the session-lifecycle hooks. The Stop hook also shells
out to `scripts/dev/spec-closure-check.py` (the spec-closure detector; also
usable directly as `--list`).

**Changing a check:** edit the function in the relevant dispatcher (not a `.sh`),
then run `python3 scripts/dev/hook-parity-check.py` to confirm no behaviour
changed. If you intentionally changed behaviour, re-bless the golden table with
`python3 scripts/dev/hook-parity-check.py --bless` and paste the result back.
Also satisfy `ai/rules/discovery-updates.md` so future agents can find it.

**Reads never block:** `Read`, `Grep`, `Glob`, `LSP`, `WebFetch`, `WebSearch`
are never rejected. Two of them write a non-blocking freshness marker so the
`design-without-lsp` gate knows the implementation was investigated: `LSP`
(via `mark-lsp-invoked.sh`) and `Read` of a `.go` under `internal/`/`pkg/`/`cmd/`
(via `mark-source-read.sh`). Only mutating/executing tools (`Bash`, `Write`,
`Edit`, `MultiEdit`, `NotebookEdit`, `Task`, `Agent`) and `ToolSearch` (which
loads LSP) are actually gated.

**Every marker is keyed by session id**, and the id is resolved in TWO places that
MUST agree: `.claude/hooks/lib/session-id.sh` (`_session_id`, used by the shell
hooks that WRITE `.lsp-loaded-*` / `.lsp-invoked-*` / `.source-read-*` /
`.session-*`) and a port inside `pretool-writeedit.py` (`session_id()`, which
READS them). Disagreement fails CLOSED -- the reader looks for a file nothing
wrote and blocks work that was actually done. Both read `$CLAUDE_CODE_SESSION_ID`
first; an id that is not a safe filename component is rejected by both rather than
rewritten. `make ze-hook-test` (section `session-id`) locks this. Before
2026-07-16 neither end had an env lookup, so with no `--session-id` in argv and no
access token every concurrent session shared ONE marker set -- `spec-session.sh
claim` then silently overwrote another session's spec claim. If you touch either
resolver, change BOTH and re-run the test.

## PreToolUse Checks (block before the tool runs)

### LSP gate (`block-until-lsp.sh`, standalone)

Enforces `session-start.md`. Triggers on `Bash|Write|Edit|MultiEdit|NotebookEdit|ToolSearch|Task|Agent`.
Blocks those tools until `ToolSearch query="select:LSP"` has run this session. BLOCKING. <!-- severity-note: the LSP gate's severity, not this reference page's -->


### Bash (`pretool-bash.py`)

| Check | Enforces | Triggers on | What it does |
|---|---|---|---|
| destructive-git | `CLAUDE.md` prohibitions | Bash | Blocks git commit/push/reset/restore/clean/merge. Allows `git restore --staged`. BLOCKING. |
| worktree-copy | `CLAUDE.md` prohibitions | Bash | Blocks cp/mv/rsync from `.claude/worktrees/` to main repo. BLOCKING. | <!-- doc-links: ignore (.claude/worktrees/ exists only while a worktree agent is active) -->
| root-build | (build hygiene) | Bash | Blocks `go build` without `-o bin/`. Allows `go build ./...` (check-only). BLOCKING. |
| pipe-tail | `bash-output.md` | Bash | Blocks a lossy filter (`head`/`tail`/`grep`/`awk`/`sed`/`cat`/`less`/`more`) piped from an EXPENSIVE producer: `make`, `go test\|build\|vet\|run`, `golangci-lint`, `bin/ze*` (with or without `./`), `ze-test`, `pytest`, everything under `scripts/evidence/` (QEMU boots, docker interop labs), and the repo's own gates under `scripts/{dev,checks,docvalid,status}/` whose filename contains check/verify/test/audit/lint/stress/repro -- minus a small cheap-probe set (`verify-status.sh`, `verify-lock.sh`, `verify-summary.sh`, `spec-closure-check.py`), which are status readers CLAUDE.md tells you to run. `\| tee` passes, and cheap commands (`git log \| tail`, `scripts/dev/spec-session.sh wip \| head`) are not its business. Judged **per statement** (`;`, `&&`, `\|\|`, newline), so a cheap pipeline beside an expensive command is fine; a trailing `\|` or a `\\` at end of line is a CONTINUATION, not a boundary, and is flattened first (splitting there put the producer in one statement and the filter in the next, so neither tripped). Quote- and `$( )`-blind by design: a `;` inside quotes or a command substitution splits a statement, and a producer inside `bash -c "..."` is not seen. BLOCKING. |
| system-tmp | `testing.md` | Bash | Blocks access to `/tmp`; must use project `tmp/`. BLOCKING. |
| test-deletion | `no-test-deletion.md` | Bash | Blocks `rm`/`git checkout` of test files. BLOCKING. |

The five commit-time gates (spec-audit, deferral-in-diff, deferral-unassigned,
wiring-at-commit, doc-drift) used to sit here but gated on the literal
`git commit` string, which the sanctioned commit path never sends and
`destructive-git` blocks when it does. They are now **creation-time gates in
`scripts/dev/commit_helper.py`** — see "Commit-time gates" below.

`golangci-lint run` also runs standalone on `Bash(git commit:*)`.

### Write/Edit (`pretool-writeedit.py`)

| Check | Enforces | Triggers on | What it does |
|---|---|---|---|
| design-without-lsp | `session-start.md`, `no-fabrication.md` | design/spec `.md` | Blocks edits to `plan/design-*.md` / `plan/spec-*.md` unless the implementation was investigated in the last 30 min: LSP invoked OR a `.go` under `internal/`/`pkg/`/`cmd/` was read. BLOCKING. | <!-- doc-links: ignore (hook trigger patterns, files may not exist) -->
| c_model_phase | `model-selection.md` | code suffixes, never `.md` | Blocks an implementation edit made on a planning/review model. The payload carries no model, so it reads the tail of `transcript_path`. Escape: record the operator's decision in `tmp/session/.model-ack-<sid>`. BLOCKING. |
| pre-write-go | `before-writing-code.md` | `internal/**/*.go` | Blocks without proper session state. BLOCKING. |
| source-edit-spec-not-in-progress | `planning.md` | source/test/learned | Blocks edits when selected spec is not `in-progress`. BLOCKING. |
| encoding-alloc | `buffer-first.md` | wire-encode `.go` | Blocks `make()`/`append()`/`Bytes()`/`Pack()` in wire-facing code. BLOCKING. |
| format-alloc | `buffer-first.md` | BGP format `.go` | Blocks `strings.Join`/`Builder`/`NewReplacer`/`ReplaceAll` (+ `fmt.Sprintf`/`Fprintf`, `strconv.Format*`) in the guarded format files. Comment lines exempt. BLOCKING. |
| sprintf-new | `no-sprintf-alloc.md` | `.go` | Blocks new `fmt.Sprintf`/`Fprintf`/`Printf`. Allows `fmt.Errorf`. BLOCKING. |
| legacy-log | `go-standards.md` | `.go` | Blocks `log.Printf` / legacy `log` package. BLOCKING. |
| panic-error | `go-standards.md` | `.go` | Blocks `panic()` except `unreachable`/`not implemented`/`TODO`/`BUG`/`impossible`. BLOCKING. |
| ignored-errors | `go-standards.md` | `.go` | Blocks `_, _ =` error-swallowing. BLOCKING. |
| silent-ignore | `config-design.md` | `.go` | Blocks empty `default:` cases. BLOCKING. |
| temp-debug | `go-standards.md` | `.go` | Blocks debug-MARKER prints (`DEBUG`/`TRACE`/`>>>`/`<<<`/`***`/`XXX`/`FIXME`) via `fmt.Print*`/`Fprint*`, bare `println(...)`, and short bare `fmt.Println("...")` in production Go. Plain `os.Stderr` output is ALLOWED -- it is the CLI's interface, and `cli-patterns.md` prescribes it. BLOCKING. |
| os-exit | `cli-patterns.md` | `.go` | Blocks `os.Exit()` outside `main.go`/`register.go`/`scripts/`. BLOCKING. |
| layering | `no-layering.md` | `.go` | Blocks backwards-compat/layering patterns. BLOCKING. |
| exabgp-in-engine | `compatibility.md` | `.go` | Blocks ExaBGP awareness outside `exabgp/`. BLOCKING. |
| version-config | `config-design.md` | config files | Blocks version fields in config. BLOCKING. |
| nolint-abuse | `quality.md` | `.go` | Blocks `//nolint:` without justification. BLOCKING. |
| lint-exclusions | `quality.md` | `.golangci.*` | Blocks adding lint exclusions. BLOCKING. |
| and-functions | `design-principles.md` | `.go` | Warns about `func *And*()` names. Advisory. |
| init-register | `design-principles.md` | `.go` | Blocks `init()` outside `register.go`. BLOCKING. |
| yagni-violations | `design-principles.md` | `.go` | Blocks speculative-feature comments. BLOCKING. |
| fake-bufhandle | (pool correctness) | `.go` | Blocks `BufHandle{Buf: make(...)}` outside `testPoolBuf`. BLOCKING. |
| observer-sys-exit | `testing.md` | `.ci` | Warns about `sys.exit(1)` in observers without `runtime_fail`. Advisory. |
| ci-sleep-justification | `ci-sleep-justification.md` | `.ci` | Warns when a `time.sleep(` is introduced with no comment above/trailing it. Advisory (blocking gate is `make ze-verify-wiring-docs`). |
| hardcoded-commands | `derive-not-hardcode.md` | `.go` | Blocks hardcoded command-list literals. BLOCKING. |
| switch-dispatch | `registration-dispatch.md` | `.go` | Blocks `switch args[0]` subcommand dispatch; use `subdispatch.New()` + `Register()`. BLOCKING. |
| json-kebab | `json-format.md` | `.go` | Blocks non-kebab-case JSON tags. BLOCKING. |
| goroutine-lifecycle | `goroutine-lifecycle.md` | hot-path `.go` | Blocks `go func()` in reactor/event/dispatch/hub/wire/message. BLOCKING. |
| require-design-ref | `design-doc-references.md` | `.go` | Blocks Go files without `// Design:` comment. BLOCKING. |
| require-related-refs | `related-refs.md` | `.go` | Blocks missing/stale `// Related:`/`// Detail:`/`// Overview:` refs. BLOCKING. |
| test-weakening (Edit/Write/MultiEdit) | `no-test-deletion.md` | test files | Blocks deleting OR weakening tests: removed funcs/cases/assertions, added `t.Skip`, `require`->`assert` downgrade, commented-out asserts, `ignore` build tag. Escape: `// test-relax: <reason>`. BLOCKING. |
| rfc-tagged-test (Edit/Write/MultiEdit) | `testing.md`, `ai/skills/ze-rfc.md` | any tag CARRIER holding `RFC requirement:` -- `_test.go`, `.ci`, `.et`, an interop `check.py` | The guard is `_rfc_tagged_change_err`, called from `c_test_weakening`. The row label on the left is the registry name, not a function name, so grep for the function. Blocks ANY behavior change to a test that proves an RFC obligation. It runs BEFORE test-weakening. `// test-relax:` does NOT satisfy it, because self-service justification is not user approval. Also blocks DELETING the tag (checked first, since a tag is a comment and the behavior comparison would pass its removal). Scope is the ENCLOSING test function (`_enclosing_tagged_scope`, which now delegates to `scripts/dev/rfc_tagged_scope.py`), not the edited hunk. So a tag on the doc comment still governs a body edit. Untagged functions in the same file are unaffected. A tag outside every function, such as a hoisted table, widens scope to the whole file. Every occurrence of a hunk is considered, so `replace_all` cannot reach a tagged copy unseen. Comment/format edits pass -- `#` counts as the comment syntax for `.ci`, `.et` and `.py`; a rename blocks. A `.go` edit made ONLY of import lines passes too (`_import_only_go_edit`): an import cannot weaken an assertion, and without it GROWING a tagged file always cost an approval, because new tests need new imports and an import block sits outside every function so the scope widens to the whole file. Every non-blank line on both sides must be import-shaped, so an assertion smuggled into the same edit still blocks, and the tag-removal check runs first so a tag cannot ride out on the exemption. Escape: `// rfc-test-change-approved: <date> <what the user approved>`, and only the user may authorize it. **The marker must sit in the replacement text of the edit itself**, since that is the only text the check reads; the same marker elsewhere in the file does not satisfy it. BLOCKING. **The carrier list is derived** from the shared leaf. `TestTaggedScopeCoversEveryCarrier` holds it against the scanner's own `CARRIERS`. Until 2026-07-29 the predicate was a literal covering `_test.go` and a `/test/` `.ci` only. The two interop `check.py` files that `plan/learned/1296-rfcgate-2-evidence.md` admits as evidence therefore carried RFC obligations this guard could not see. |
| system-tmp (path) | `testing.md` | any | Blocks writing to `/tmp`. BLOCKING. |
| generated-files | `canonical-sources.md` | `CLAUDE.md`/`AGENTS.md` | Blocks editing generated files. BLOCKING. |
| claude-plans | `.claude/rules/planning.md` | Write | Blocks `.claude/plans/` and `~/.claude/plan/`. BLOCKING. | <!-- doc-links: ignore (banned location, deliberately nonexistent) -->
| check-existing-patterns | `before-writing-code.md` | new `internal/**/*.go` | Blocks duplicate exported type/func in same package. BLOCKING. |
| check-existing-tests | `before-writing-code.md` | new test files | Warns about similar existing tests. Advisory. |
| enforce-naming | `documentation.md` | new files | Warns on wrong file naming. Advisory. |
| throwaway-tests | `testing.md` | Write | Blocks test files in `/tmp` and throwaway locations. BLOCKING. |
| utils-package | `design-principles.md` | Write `.go` | Blocks `utils/`/`helpers/`/`common/`/`misc/` packages. BLOCKING. |
| require-test-first | `tdd.md` | new `.go` | Warns when creating impl without a test file. Advisory. |
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

## PostToolUse Checks (run after the tool completes)

| Check | File | Enforces | Triggers on | What it does |
|---|---|---|---|---|
| mark-lsp-invoked | `mark-lsp-invoked.sh` | `session-start.md` | LSP | Writes `.lsp-invoked` freshness marker for the design-without-lsp gate. |
| mark-source-read | `mark-source-read.sh` | `no-fabrication.md` | Read | Writes `.source-read` freshness marker when a `.go` under `internal/`/`pkg/`/`cmd/` is read, so reading the producing code satisfies the design-without-lsp gate. Non-blocking. |
| mark-agent-spawned | `mark-agent-spawned.sh` | `spec-delegation.md` | Agent, Task | Writes `.agent-spawned-<sid>` so the Stop hook can tell a supervising main thread from one that ran the phase inline. Fires in the PARENT (subagents inherit its session id), so the marker always lands on the supervising session. Non-blocking. |
| auto-lint | `posttool-writeedit.py` | `go-standards.md` | `.go` Write/Edit | `gofmt`/`goimports -w`, then **one** `golangci-lint --new-from-rev=HEAD` pass (flags only issues this edit introduced). BLOCKING on lint failure. |
| auto-py-format | `posttool-writeedit.py` | (code style) | `.py` Write/Edit | `ruff format` + `ruff check`. Non-blocking. |
| validate-spec | `validate-spec.sh` | `planning.md` | `plan/spec-*.md` | Validates required sections/format. Exit 2 blocks a structurally invalid spec; both `→` and `->` wiring rows accepted. |
| file-size | `posttool-writeedit.py` | `file-modularity.md` | `.go` | Warns >1000 lines. Advisory. |
| warn-deferral | `posttool-writeedit.py` | `deferral-tracking.md` | `.md` | Warns on deferral language in doc edits. Advisory. |
| require-rfc-reference | `posttool-writeedit.py` | `design-doc-references.md` | `.go` | Suggests `// RFC:` header. Advisory. |
| require-test-docs | `posttool-writeedit.py` | `tdd.md` | `_test.go` | Warns about missing `VALIDATES:`/`PREVENTS:`. Advisory. |
| require-fuzz-tests | `posttool-writeedit.py` | `tdd.md` | wire `.go` | Warns about `Parse*` without `Fuzz*` tests. Advisory. |
| vague-names | `posttool-writeedit.py` | `design-principles.md` | `.go` | Warns about `Data`/`Info`/`Result`/... names. Advisory. |
| boundary-tests | `posttool-writeedit.py` | `tdd.md` | `.go` | Warns about numeric validation without boundary tests. Advisory. |

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

`make ze-verify` separately runs `ze-verify-wiring-docs` (wiring/doc-drift gate);
that is a Make target, not a Claude hook.

## Changed-file gates inside `ze-verify-wiring-docs`

Also Make targets, not Claude hooks. All are changed-file scoped: a session
owns the files it touches, not the whole tree.

| Check | Enforces | Triggers on | What it does |
|---|---|---|---|
| `check_ci_sleep_ratchet` | `ci-sleep-justification.md` | changed `test/**/*.ci` | Caps how MANY `time.sleep(` calls exist tree-wide against a committed delta baseline. BLOCKING. |
| `check_ci_sleep_justification` | `ci-sleep-justification.md` | changed `test/**/*.ci` | Caps how many sleeps are UNEXPLAINED: each needs a comment above or trailing it. BLOCKING. |
| `check_known_failure_load_excuses` | `fix-dont-record.md` | changed `plan/known-failures/*.md` | Rejects a shard blaming host load ("under load", "loaded host", "load average", "load-sensitive", "passes in isolation", "resource contention", "contended host"). `README.md` / `RESOLVED.md` exempt. BLOCKING. |
| `check_ci_log_subsystem_keys` | `testing.md`, `config-naming.md` | changed `test/**/*.ci` | Rejects a `ze.log.<subsystem>` key whose subsystem contains a hyphen that is not declared literally in Go. An internal plugin's logger name is `CanonicalSubsystemName` of its registry name (every hyphen becomes a dot) and `getLogEnv` splits on `.` only, so `ze.log.bgp.adj-rib-in` sets nothing and the level silently stays at the WARN default. Scan is tree-wide; `#` comment lines exempt. BLOCKING. |

## Prose gate (ASD-STE100)

Make targets and a commit-time gate, not Claude hooks. HEAD is the baseline and
the comparison is per file, so a document nobody touched can never fail.

| Check | Enforces | Triggers on | What it does |
|---|---|---|---|
| `ste_problems` (`commit_helper.py`) | `simplified-technical-english.md` | `commit_helper.py create` with any `.md`, `.go`, or `.yang` in the commit | Runs `ste_check.py --check` over the FILES OF THAT COMMIT and PRINTS the six banned ASD-STE100 habits that grew against HEAD. Advisory: STE is a guideline, so this never refuses a commit. Commit scope is deliberate: several sessions share this checkout, so a tree-wide prose gate judges a colleague's in-flight sentences and gets switched off. BLOCKING. |
| `make ze-ste-check` | `simplified-technical-english.md` | on demand, before you prepare a commit | The same comparison over every changed file in the working tree. It prints the file, the habit, and only the new findings. Surfaces are Markdown in `docs/`, `ai/`, `plan/`, and the root, prose comments in `.go`, and `description` strings in `.yang`. Renames follow `-M`, so a moved legacy document does not report its inherited content as new. Deliberately NOT in `ze-doc-test`. Whole-tree report: `make ze-ste-review`. |

## Commit-time gates (`scripts/dev/commit_helper.py`)

These are NOT Claude hooks. They run when `commit_helper.py create` generates the
commit script, which is the only sanctioned commit path (`bash tmp/commit-<SID>.sh`).
The helper already knows the exact add/remove set of the commit, so the gates
inspect that instead of the staging area. BLOCK gates raise (exit 2, no script
written); WARN gates print to stderr and let the script be written.

| Gate | Enforces | Severity | What it does |
|---|---|---|---|
| verify-status / structural-gate | `git-safety.md` | BLOCK | Refuses a script over a non-green `ze-verify` (structural reds are unbypassable). |
| ste (`ste_problems`) | `simplified-technical-english.md` | WARN | Runs `ste_check.py --check` over the commit's own `.md`, `.go`, and `.yang` files and prints the six banned ASD-STE100 habits that grew against HEAD. It never refuses. Legacy prose in a file you touched costs nothing, because each file is compared with its own HEAD version. Scoped to the commit for the same reason discovery-index materializes a commit view: a concurrent session's uncommitted prose must not block your commit. Prints the file, the habit, and only the new findings. |
| discovery-index | `discovery-updates.md` | BLOCK | Refuses when a generated index (PACKAGE-MAP / DOCS-TO-CODE / LEARNED-FULL-INDEX) would be left incoherent. Judged on the tree the COMMIT PRODUCES (HEAD + adds - removes), materialized under `tmp/commit-view-*` and checked with the commit's OWN generators via `--root`, never on the working tree: a concurrent session's uncommitted sources must neither block your commit nor be swept into your index. **Every** index whose generator exists is verified, not just the ones the commit visibly feeds -- `package_map` keys its rows on directory existence, so a new `.go` can drift PACKAGE-MAP while feeding only DOCS-TO-CODE. Cost: the view is built on EVERY commit the gate examines, not only ones touching an index source, because the candidate set comes from generator existence -- about 5.5s total (~2s working-tree freshness, ~3.6s to materialize and check). If the view cannot be built it does NOT fail closed: it warns on stderr and falls back to the working-tree verdict, which is the only evidence left. BLOCKING. |
| deferral-unassigned | `deferral-tracking.md` | BLOCK | Folds over every shard in `plan/deferrals/` and flags an open row with no Destination. |
| deferral-in-diff | `deferral-tracking.md` | BLOCK | Blocks when the commit's added lines contain deferral language and no `plan/deferrals/` shard is part of the commit (diff computed in a throwaway git index). |
| spec-audit | `planning.md` | BLOCK | Blocks the closure commit (the one adding the claimed spec's `plan/learned/NNN-<stem>.md`) when that spec's `## Pre-Commit Verification` section is unfilled. Keyed to `spec-session.sh current`; no claim → skips. |
| wiring-at-commit | `integration-completeness.md` | WARN | Warns when `internal/plugins/**/*.go` is committed with no `.ci`. |
| doc-drift | `documentation.md` | WARN | Runs `scripts/docvalid/doc_drift.go`; warns on drift. |

## Hook tests (`make ze-hook-test`)

| Runner | Covers |
|---|---|
| `scripts/dev/hook-parity-check.py` | Golden exit-code regression for the three consolidated dispatchers. `--bless` regenerates the golden; re-bless only intentionally changed cases. Fixture dirs live under `~/.cache` (a `/tmp` or in-repo path trips `system-tmp`/`throwaway-tests` or the module lint and diverges from the golden). |
| `scripts/dev/hook-fixture-check.py` | Behaviour the golden table cannot isolate: `c_format_alloc`, `validate-spec.sh`, the `commit_helper.py` commit-time gates over git-initialized fixtures, and the 35 `delegation` fixtures. Those 35 pin what no other test reaches. The `Stop` array registration and its order. BOTH ends of the claim lifetime: alive past turn one, released at `SessionEnd`, kept across a resume. The two stop-phrase tiers. The markup filter that must fail toward scanning MORE. Deleting the release line once left the whole suite green. Sections selectable with `--only`. |

## Session Lifecycle Hooks

**UserPromptSubmit stdout reaches the model. UserPromptSubmit stderr does not.**
A reminder that must land in the context writes to stdout. A banner that must
cost no context tokens writes to stderr, as `compaction-reminder.sh` does. The
two stdout reminders below fire on every turn, so each one stays a single line.

| Hook | Event | What it does |
|---|---|---|
| `session-start.sh` | SessionStart | Prints status summary. Creates session marker. |
| `compaction-reminder.sh` | UserPromptSubmit | Detects compaction; reminds to read `post-compaction.md`. Writes to **stderr**, so it costs no context tokens. |
| `verify-claim-reminder.sh` | UserPromptSubmit | Emits one **stdout** line per turn. Verify a claim about code by reading the function that PRODUCES the behavior, not the caller. Label an unread claim unverified. Name the file and the symbol, and use a line number only when the line IS the fact. Report the conclusion, not the search. Enforces `ai/rules/no-fabrication.md` and `ai/rules/detail-budget.md`. A banner read once at session start does not survive to the turn that makes the claim, so this lands in fresh context. |
| `delegation-reminder.sh` | UserPromptSubmit | Emits one **stdout** line per turn: subagent delegation needs no permission in this repository. The harness appends the guard "Do not call the AgentTool unless the user requested it" to the END of the system prompt, where it wins on position. `ai/INSTRUCTIONS.md` "STANDING REQUEST: delegate to subagents" IS the request that guard defers to, but it sits far earlier in the same prompt and loses. UserPromptSubmit stdout is the only harness position that lands after the whole system prompt, so the counter goes there. Unconditional by design: a conditional reminder adds a "did the condition fire" failure mode, and the reminder is correct on every turn. Enforces `ai/rules/spec-delegation.md`. Fixtures: `python3 scripts/dev/hook-fixture-check.py --only delegation-reminder`. |
| `block-premature-stop.sh` | Stop (**first**, ahead of `session-end-summary.sh`) | **Live and BLOCKING** since 2026-07-31. Four gates run. All but the first need a session that CLAIMED a spec. (1) **Stop-phrase scan**, exit 2. `PHRASES` covers ownership-dodging, premature handoff and permission-seeking, and it always blocks. `COMPLETION_PHRASES` (`what next`, `what would you like`) blocks ONLY while a claimed spec is `in-progress`, because `.claude/rules/session-start.md` REQUIRES that question once the task is done. A phrase inside backticks or a closed fence is quoted, not used, and does not block. (2) **Spec-closure gate**, exit 2, when `spec-closure-check.py --spec` reports the spec completed but not closed. That check runs well inside the hook's 10s timeout, and a slower one would fail the gate open with no signal. Two escapes: run commit B, or write `tmp/session/.closure-ack-<stem>`. (3) **Spec still in-progress** warning, exit 1. (4) **Delegation nudge**, exit 1, when the session claimed a spec and spawned no agent. A harness retry (`stop_hook_active`) bounds the phrase scan ALONE, whose only escape is rewording. The other three stay armed, because each has an escape of its own. Gates 2 to 4 need the claim marker to outlive turn one, so the release moved from `Stop` to `SessionEnd`. Behaviour is pinned by 35 fixtures: `python3 scripts/dev/hook-fixture-check.py --only delegation`. |
| `session-end-summary.sh` | Stop | Writes session state snapshot. It no longer releases the spec claim: that moved to `SessionEnd` (`session-end-scratch.sh:60`) so the claim survives the turn-by-turn `Stop`. |
| `session-end-deferrals.sh` | Stop | Prints open deferral count. Advisory. |
| `pre-compact-save.sh` | PreCompact | Saves session state before compaction. |
| `subagent-context.sh` | SubagentStart | Injects project context into spawned agents, PLUS the parent session's claimed spec (with its Status) and the subagent contract from `ai/rules/spec-delegation.md`. This exists to make delegating cheap: the rule requires the main thread to hand every agent its spec, phase, and rules, and when that is manual per-spawn work, delegating costs more than working inline and the rule loses. |

## Pre-Flight Checklist by File Type

(All checks below now run inside the dispatchers; behaviour is unchanged.)

### Any `.go` file under `internal/`

pre-write-go, auto-lint, encoding-alloc (wire paths), sprintf-new, legacy-log,
panic-error, ignored-errors, silent-ignore, temp-debug, os-exit, init-register,
yagni-violations, json-kebab, require-design-ref, require-related-refs, file-size.
Hot-path files (reactor/event/dispatch/hub/wire/message) also: goroutine-lifecycle, fake-bufhandle.

### Test files (`_test.go`, `.ci`)

test-weakening (if removing/weakening tests), check-existing-tests (if new), require-test-docs,
boundary-tests, observer-sys-exit (`.ci` with Python).

### Spec files (`plan/spec-*.md`)

validate-spec, design-without-lsp (needs recent implementation investigation:
LSP invoked or a `.go` under `internal/`/`pkg/`/`cmd/` read), require-docs-read
(if new), source-edit-spec-not-in-progress (if spec not `in-progress`).

### Python files (`.py`)

auto-py-format (ruff format + check).

### Commits

A Bash `git commit` is blocked outright by destructive-git. Commit via
`scripts/dev/commit_helper.py create` + `bash tmp/commit-<SID>.sh`; the
creation-time gates (verify-status, discovery-index, deferral-unassigned,
deferral-in-diff, spec-audit block; wiring-at-commit, doc-drift warn) run then —
see "Commit-time gates" above.
