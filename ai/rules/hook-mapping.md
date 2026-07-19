# Hook-to-Rule Mapping

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
Blocks those tools until `ToolSearch query="select:LSP"` has run this session. BLOCKING.

### Bash (`pretool-bash.py`)

| Check | Enforces | Triggers on | What it does |
|---|---|---|---|
| destructive-git | `CLAUDE.md` prohibitions | Bash | Blocks git commit/push/reset/restore/clean/merge. Allows `git restore --staged`. BLOCKING. |
| worktree-copy | `CLAUDE.md` prohibitions | Bash | Blocks cp/mv/rsync from `.claude/worktrees/` to main repo. BLOCKING. | <!-- doc-links: ignore (.claude/worktrees/ exists only while a worktree agent is active) -->
| root-build | (build hygiene) | Bash | Blocks `go build` without `-o bin/`. Allows `go build ./...` (check-only). BLOCKING. |
| pipe-tail | `bash-output.md` | Bash | Blocks `\| tail` and piping `make ze-*` output. BLOCKING. |
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
| rfc-tagged-test (Edit/Write/MultiEdit) | `testing.md`, `ai/skills/ze-rfc.md` | test files carrying `RFC requirement:` | Blocks ANY behavior change to a test that proves an RFC obligation; runs BEFORE test-weakening, and `// test-relax:` does NOT satisfy it (self-service justification is not user approval). Comment/format edits pass; a rename blocks. Escape: `// rfc-test-change-approved: <date> <what the user approved>`, and only the user may authorize it. BLOCKING. |
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

> **format-alloc is now live** (enabled 2026-07-09, spec-followup-hooks). The
> original `block-format-alloc.sh` used bash-4 `declare -A`, which the macOS bash
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
| auto-lint | `posttool-writeedit.py` | `go-standards.md` | `.go` Write/Edit | `gofmt`/`goimports -w`, then **one** `golangci-lint --new-from-rev=HEAD` pass (flags only issues this edit introduced). BLOCKING on lint failure. |
| auto-py-format | `posttool-writeedit.py` | (code style) | `.py` Write/Edit | `ruff format` + `ruff check`. Non-blocking. |
| validate-spec | `validate-spec.sh` | `planning.md` | `plan/spec-*.md` | Validates required sections/format. Exit 2 blocks a structurally invalid spec; both `→` and `->` wiring rows accepted. |
| file-size | `posttool-writeedit.py` | `file-modularity.md` | `.go` | Warns >600 lines, strong >1000. Advisory. |
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

## Commit-time gates (`scripts/dev/commit_helper.py`)

These are NOT Claude hooks. They run when `commit_helper.py create` generates the
commit script, which is the only sanctioned commit path (`bash tmp/commit-<SID>.sh`).
The helper already knows the exact add/remove set of the commit, so the gates
inspect that instead of the staging area. BLOCK gates raise (exit 2, no script
written); WARN gates print to stderr and let the script be written.

| Gate | Enforces | Severity | What it does |
|---|---|---|---|
| verify-status / structural-gate | `git-safety.md` | BLOCK | Refuses a script over a non-green `ze-verify` (structural reds are unbypassable). |
| discovery-index | `discovery-updates.md` | BLOCK | Refuses when a generated index (PACKAGE-MAP / DOCS-TO-CODE / LEARNED-FULL-INDEX) would be left stale. |
| deferral-unassigned | `deferral-tracking.md` | BLOCK | Blocks when `plan/deferrals.md` has an open row with no Destination. |
| deferral-in-diff | `deferral-tracking.md` | BLOCK | Blocks when the commit's added lines contain deferral language and `plan/deferrals.md` is not part of the commit (diff computed in a throwaway git index). |
| spec-audit | `planning.md` | BLOCK | Blocks the closure commit (the one adding the claimed spec's `plan/learned/NNN-<stem>.md`) when that spec's `## Pre-Commit Verification` section is unfilled. Keyed to `spec-session.sh current`; no claim → skips. |
| wiring-at-commit | `integration-completeness.md` | WARN | Warns when `internal/plugins/**/*.go` is committed with no `.ci`. |
| doc-drift | `documentation.md` | WARN | Runs `scripts/docvalid/doc_drift.go`; warns on drift. |

## Hook tests (`make ze-hook-test`)

| Runner | Covers |
|---|---|
| `scripts/dev/hook-parity-check.py` | Golden exit-code regression for the three consolidated dispatchers. `--bless` regenerates the golden; re-bless only intentionally changed cases. Fixture dirs live under `~/.cache` (a `/tmp` or in-repo path trips `system-tmp`/`throwaway-tests` or the module lint and diverges from the golden). |
| `scripts/dev/hook-fixture-check.py` | Behaviour the golden table cannot isolate: `c_format_alloc` (called directly), `validate-spec.sh` (ASCII/Unicode/malformed specs), and the `commit_helper.py` commit-time gates (git-initialized fixtures). Sections selectable with `--only`. |

## Session Lifecycle Hooks

| Hook | Event | What it does |
|---|---|---|
| `session-start.sh` | SessionStart | Prints status summary. Creates session marker. |
| `compaction-reminder.sh` | UserPromptSubmit | Detects compaction; reminds to read `post-compaction.md`. |
| `block-premature-stop.sh` | Stop | Blocks stop on ownership-dodging phrases. Also runs `scripts/dev/spec-closure-check.py --spec` on the session's claimed spec and blocks (exit 2) if it is completed-but-not-closed. BLOCKING. See `ai/rules/planning.md` "Closure Enforcement". |
| `session-end-summary.sh` | Stop | Writes session state snapshot. Cleans up marker. |
| `session-end-deferrals.sh` | Stop | Prints open deferral count. Advisory. |
| `pre-compact-save.sh` | PreCompact | Saves session state before compaction. |
| `subagent-context.sh` | SubagentStart | Injects project context into spawned agents. |

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
