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
and the session-lifecycle hooks.

**Changing a check:** edit the function in the relevant dispatcher (not a `.sh`),
then run `python3 scripts/dev/hook-parity-check.py` to confirm no behaviour
changed. If you intentionally changed behaviour, re-bless the golden table with
`python3 scripts/dev/hook-parity-check.py --bless` and paste the result back.
Also satisfy `ai/rules/discovery-updates.md` so future agents can find it.

**Reads are free:** `Read`, `Grep`, `Glob`, `LSP`, `WebFetch`, `WebSearch` and
other read-only tools trigger NO hooks. Only mutating/executing tools
(`Bash`, `Write`, `Edit`, `MultiEdit`, `NotebookEdit`, `Task`, `Agent`) and
`ToolSearch` (which loads LSP) are gated.

## PreToolUse Checks (block before the tool runs)

### LSP gate (`block-until-lsp.sh`, standalone)

Enforces `session-start.md`. Triggers on `Bash|Write|Edit|MultiEdit|NotebookEdit|ToolSearch|Task|Agent`.
Blocks those tools until `ToolSearch query="select:LSP"` has run this session. BLOCKING.

### Bash (`pretool-bash.py`)

| Check | Enforces | Triggers on | What it does |
|---|---|---|---|
| destructive-git | `CLAUDE.md` prohibitions | Bash | Blocks git commit/push/reset/restore/clean/merge. Allows `git restore --staged`. BLOCKING. |
| worktree-copy | `CLAUDE.md` prohibitions | Bash | Blocks cp/mv/rsync from `.claude/worktrees/` to main repo. BLOCKING. |
| root-build | (build hygiene) | Bash | Blocks `go build` without `-o bin/`. Allows `go build ./...` (check-only). BLOCKING. |
| pipe-tail | `bash-output.md` | Bash | Blocks `\| tail` and piping `make ze-*` output. BLOCKING. |
| system-tmp | `testing.md` | Bash | Blocks access to `/tmp`; must use project `tmp/`. BLOCKING. |
| test-deletion | `no-test-deletion.md` | Bash | Blocks `rm`/`git checkout` of test files. BLOCKING. |
| spec-audit | `implementation-audit.md` | Bash (git commit) | Verifies spec obligations before commit. BLOCKING. |
| deferral-in-diff | `deferral-tracking.md` | Bash (git commit) | Blocks commit when staged diff has deferral language without `plan/deferrals.md` update. BLOCKING. |
| deferral-unassigned | `deferral-tracking.md` | Bash (git commit) | Blocks commit when open deferrals have no destination. BLOCKING. |
| wiring-at-commit | `integration-completeness.md` | Bash (git commit) | Warns about plugin code staged without `.ci` tests. Advisory. |
| doc-drift | `documentation.md` | Bash (git commit) | Warns about docs drifting from live registry. Advisory. |

`golangci-lint run` also runs standalone on `Bash(git commit:*)`.

### Write/Edit (`pretool-writeedit.py`)

| Check | Enforces | Triggers on | What it does |
|---|---|---|---|
| design-without-lsp | `session-start.md` | design/spec `.md` | Blocks edits to `plan/design-*.md` / `plan/spec-*.md` unless LSP invoked in last 30 min. BLOCKING. | <!-- doc-links: ignore (hook trigger patterns, files may not exist) -->
| pre-write-go | `before-writing-code.md` | `internal/**/*.go` | Blocks without proper session state. BLOCKING. |
| source-edit-spec-not-in-progress | `planning.md` | source/test/learned | Blocks edits when selected spec is not `in-progress`. BLOCKING. |
| encoding-alloc | `buffer-first.md` | wire-encode `.go` | Blocks `make()`/`append()`/`Bytes()`/`Pack()` in wire-facing code. BLOCKING. |
| format-alloc | `buffer-first.md` | BGP format `.go` | **No-op** (see note below). |
| sprintf-new | `no-sprintf-alloc.md` | `.go` | Blocks new `fmt.Sprintf`/`Fprintf`/`Printf`. Allows `fmt.Errorf`. BLOCKING. |
| legacy-log | `go-standards.md` | `.go` | Blocks `log.Printf` / legacy `log` package. BLOCKING. |
| panic-error | `go-standards.md` | `.go` | Blocks `panic()` except `unreachable`/`not implemented`/`TODO`/`BUG`/`impossible`. BLOCKING. |
| ignored-errors | `go-standards.md` | `.go` | Blocks `_, _ =` error-swallowing. BLOCKING. |
| silent-ignore | `config-design.md` | `.go` | Blocks empty `default:` cases. BLOCKING. |
| temp-debug | `go-standards.md` | `.go` | Blocks `fmt.Print*`/`println` in production Go. BLOCKING. |
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
| hardcoded-commands | `derive-not-hardcode.md` | `.go` | Blocks hardcoded command-list literals. BLOCKING. |
| switch-dispatch | `registration-dispatch.md` | `.go` | Blocks `switch args[0]` subcommand dispatch; use `subdispatch.New()` + `Register()`. BLOCKING. |
| json-kebab | `json-format.md` | `.go` | Blocks non-kebab-case JSON tags. BLOCKING. |
| goroutine-lifecycle | `goroutine-lifecycle.md` | hot-path `.go` | Blocks `go func()` in reactor/event/dispatch/hub/wire/message. BLOCKING. |
| require-design-ref | `design-doc-references.md` | `.go` | Blocks Go files without `// Design:` comment. BLOCKING. |
| require-related-refs | `related-refs.md` | `.go` | Blocks missing/stale `// Related:`/`// Detail:`/`// Overview:` refs. BLOCKING. |
| test-deletion (Edit) | `no-test-deletion.md` | test files | Blocks removing test funcs/cases/assertions. BLOCKING. |
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

> **format-alloc is a no-op.** The original `block-format-alloc.sh` used bash-4
> `declare -A`, but the `#!/bin/bash` shebang is macOS bash 3.2, so it errored
> and exited 0 — it never enforced anything. The port preserves that (the real
> check sits disabled in `pretool-writeedit.py`, ready to enable).

## PostToolUse Checks (run after the tool completes)

| Check | File | Enforces | Triggers on | What it does |
|---|---|---|---|---|
| mark-lsp-invoked | `mark-lsp-invoked.sh` | `session-start.md` | LSP | Writes freshness marker for the design-without-lsp gate. |
| auto-lint | `posttool-writeedit.py` | `go-standards.md` | `.go` Write/Edit | `gofmt`/`goimports -w`, then **one** `golangci-lint --new-from-rev=HEAD` pass (flags only issues this edit introduced). BLOCKING on lint failure. |
| auto-py-format | `posttool-writeedit.py` | (code style) | `.py` Write/Edit | `ruff format` + `ruff check`. Non-blocking. |
| validate-spec | `validate-spec.sh` | `planning.md` | `plan/spec-*.md` | Validates required sections/format. **Currently broken** (see note). |
| file-size | `posttool-writeedit.py` | `file-modularity.md` | `.go` | Warns >600 lines, strong >1000. Advisory. |
| warn-deferral | `posttool-writeedit.py` | `deferral-tracking.md` | `.md` | Warns on deferral language in doc edits. Advisory. |
| require-rfc-reference | `posttool-writeedit.py` | `design-doc-references.md` | `.go` | Suggests `// RFC:` header. Advisory. |
| require-test-docs | `posttool-writeedit.py` | `tdd.md` | `_test.go` | Warns about missing `VALIDATES:`/`PREVENTS:`. Advisory. |
| require-fuzz-tests | `posttool-writeedit.py` | `tdd.md` | wire `.go` | Warns about `Parse*` without `Fuzz*` tests. Advisory. |
| vague-names | `posttool-writeedit.py` | `design-principles.md` | `.go` | Warns about `Data`/`Info`/`Result`/... names. Advisory. |
| boundary-tests | `posttool-writeedit.py` | `tdd.md` | `.go` | Warns about numeric validation without boundary tests. Advisory. |

> **validate-spec.sh is broken** and kept standalone for that reason. It greps the
> Wiring Test table for the Unicode arrow `→`, but real specs use ASCII `->`, so
> an unguarded `grep -v` pipeline returns 1 and `set -e` aborts the script at
> exit 1 (non-blocking) before validation finishes. It therefore does NOT block
> most real specs today. It was NOT folded into the dispatcher because doing so
> would either replicate the crash or silently turn it into a blocking gate. Fix
> the `→`/`->` mismatch (and the `set -e` fragility) before relying on it.

`make ze-verify` separately runs `ze-verify-wiring-docs` (wiring/doc-drift gate);
that is a Make target, not a Claude hook.

## Session Lifecycle Hooks

| Hook | Event | What it does |
|---|---|---|
| `session-start.sh` | SessionStart | Prints status summary. Creates session marker. |
| `compaction-reminder.sh` | UserPromptSubmit | Detects compaction; reminds to read `post-compaction.md`. |
| `block-premature-stop.sh` | Stop | Blocks stop on ownership-dodging phrases. BLOCKING. |
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

test-deletion (if removing tests), check-existing-tests (if new), require-test-docs,
boundary-tests, observer-sys-exit (`.ci` with Python).

### Spec files (`plan/spec-*.md`)

validate-spec, design-without-lsp (needs recent LSP), require-docs-read (if new),
source-edit-spec-not-in-progress (if spec not `in-progress`).

### Python files (`.py`)

auto-py-format (ruff format + check).

### Commits (Bash `git commit`)

destructive-git (blocks), spec-audit, deferral-in-diff, deferral-unassigned,
wiring-at-commit, doc-drift.
