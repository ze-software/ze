# Hook-to-Rule Mapping

Quick reference: which hooks enforce which rules, and when they trigger.
Consult this BEFORE writing code to proactively comply, rather than
fixing after rejection. For hook false positives and workarounds, see
`plan/learned/HOOK-FRICTION.md`.

## PreToolUse Hooks (block before the tool runs)

### Universal (all tools)

| Hook | Enforces | Triggers on | What it does |
|---|---|---|---|
| `block-until-lsp.sh` | `session-start.md` | `.*` (all tools) | Blocks every tool call until `ToolSearch query="select:LSP"` has been run. BLOCKING. |

### Bash

| Hook | Enforces | Triggers on | What it does |
|---|---|---|---|
| `block-destructive-git.sh` | `CLAUDE.md` prohibitions | Bash | Blocks git commit/push/reset/restore/clean/merge. Allows `git restore --staged`. BLOCKING. |
| `block-worktree-copy.sh` | `CLAUDE.md` prohibitions | Bash | Blocks cp/mv/rsync from `.claude/worktrees/` to main repo. BLOCKING. |
| `block-root-build.sh` | (build hygiene) | Bash | Blocks `go build` without `-o bin/`. Allows `go build ./...` (check-only). BLOCKING. |
| `block-pipe-tail.sh` | `bash-output.md` | Bash | Blocks `\| tail` and piping `make ze-*` output. BLOCKING. |
| `block-system-tmp.sh` | `testing.md` | Bash, Write\|Edit | Blocks access to `/tmp`; must use project `tmp/`. BLOCKING. |
| `pre-commit-spec-audit.sh` | `implementation-audit.md` | Bash (git commit) | Verifies spec obligations before commit: file existence, AC evidence, audit tables. BLOCKING. |
| `check-deferral-in-diff.sh` | `deferral-tracking.md` | Bash (git commit) | Blocks commit when staged diff contains deferral language without `plan/deferrals.md` update. BLOCKING. |
| `check-deferral-unassigned.sh` | `deferral-tracking.md` | Bash (git commit) | Blocks commit when open deferrals have no destination spec. BLOCKING. |

### Write and/or Edit (.go files)

| Hook | Enforces | Triggers on | What it does |
|---|---|---|---|
| `block-design-without-lsp.sh` | `session-start.md` | Write\|Edit\|MultiEdit\|NotebookEdit | Blocks edits to `plan/design-*.md` or `plan/spec-*.md` unless LSP was invoked in the last 30 min. BLOCKING. |
| `pre-write-go.sh` | `before-writing-code.md` | Write\|Edit | Blocks writes to `internal/**/*.go` without proper session state. BLOCKING. |
| `block-source-edit-spec-not-in-progress.sh` | `planning.md` | Write\|Edit\|MultiEdit | Blocks source/test/learned edits when selected spec is not `in-progress`. BLOCKING. |
| `block-encoding-alloc.sh` | `buffer-first.md`, `design-principles.md` | Write\|Edit | Blocks `make()`/`append()`/`Bytes()`/`Pack()` in wire-facing code. BLOCKING. |
| `block-format-alloc.sh` | `buffer-first.md` | Write\|Edit | Blocks `fmt.Sprintf`/`strings.Builder`/`strings.Join` in BGP text/JSON format files. BLOCKING. |
| `block-sprintf-new.sh` | `no-sprintf-alloc.md` | Write\|Edit | Blocks new `fmt.Sprintf`/`Fprintf`/`Printf` in Go production code. Allows `fmt.Errorf`. BLOCKING. |
| `block-legacy-log.sh` | `go-standards.md` | Write\|Edit | Blocks `log.Printf` and legacy log package usage. BLOCKING. |
| `block-panic-error.sh` | `go-standards.md` | Write\|Edit | Blocks `panic()` except approved prefixes: `BUG:`, `unreachable:`, `not implemented`, `TODO:`, `impossible:`. BLOCKING. |
| `block-ignored-errors.sh` | `go-standards.md` | Write\|Edit | Blocks `_, _ =` error-swallowing pattern. BLOCKING. |
| `block-silent-ignore.sh` | `config-design.md`, `go-standards.md` | Write\|Edit | Blocks bare `default:` on its own line. Put body on same line or use if/else. BLOCKING. |
| `block-temp-debug.sh` | `go-standards.md` | Write\|Edit | Blocks `fmt.Print*` in production Go files. Use `slog` for permanent debug logging. BLOCKING. |
| `block-os-exit.sh` | `cli-patterns.md` | Write\|Edit | Blocks `os.Exit()` outside `main.go` and `register.go`. BLOCKING. |
| `block-layering.sh` | `no-layering.md` | Write\|Edit | Blocks layering/compatibility patterns. BLOCKING. |
| `block-exabgp-in-engine.sh` | `compatibility.md` | Write\|Edit | Blocks ExaBGP format awareness outside `exabgp/` package. BLOCKING. |
| `block-version-config.sh` | `config-design.md` | Write\|Edit | Blocks version fields in config-related files. BLOCKING. |
| `block-nolint-abuse.sh` | `quality.md` | Write\|Edit | Blocks `//nolint:` without justification comment. BLOCKING. |
| `block-lint-exclusions.sh` | `quality.md` | Write\|Edit | Blocks adding exclusions to `.golangci.yml`. BLOCKING. |
| `block-and-functions.sh` | `design-principles.md` | Write\|Edit | Warns about `func *And*()` names violating single responsibility. Advisory. |
| `block-init-register.sh` | `design-principles.md` | Write\|Edit | Blocks `init()` outside `register.go`/`register_*.go` files. BLOCKING. |
| `block-yagni-violations.sh` | `design-principles.md` | Write\|Edit | Blocks speculative features (TODO, placeholder, future, stub patterns). BLOCKING. |
| `block-fake-bufhandle.sh` | (pool correctness) | Write\|Edit\|MultiEdit | Blocks `BufHandle{Buf: make(...)}` outside `testPoolBuf`. Prevents pool corruption. BLOCKING. |
| `block-observer-sys-exit.sh` | `testing.md` | Write\|Edit\|MultiEdit | Warns about `sys.exit(1)` in `.ci` Python observers without `runtime_fail`. Advisory. |
| `check-json-kebab.sh` | `json-format.md` | Write\|Edit | Blocks non-kebab-case JSON field tags. BLOCKING. |
| `check-goroutine-lifecycle.sh` | `goroutine-lifecycle.md` | Write\|Edit | Blocks `go func()` in hot-path files (reactor, event, dispatch, hub, wire, message). BLOCKING. |
| `require-test-first.sh` | `tdd.md` | Write\|Edit | Warns when editing Go impl files without a corresponding test file. Advisory. |
| `require-design-ref.sh` | `design-doc-references.md` | Write\|Edit | Blocks Go files without `// Design:` comment. Exempts test/register/embed/doc/gen files. BLOCKING. |
| `require-related-refs.sh` | `related-refs.md` | Write\|Edit | Blocks Go files with missing or stale `// Related:` / `// Detail:` / `// Overview:` cross-references. BLOCKING. |
| `block-test-deletion.sh` | `no-test-deletion.md` | Write\|Edit, Bash | Blocks removing test functions or deleting test files. BLOCKING. |

### Write only (new files)

| Hook | Enforces | Triggers on | What it does |
|---|---|---|---|
| `block-claude-plans.sh` | `.claude/rules/planning.md` | Write | Blocks writes to `.claude/plans/` or `~/.claude/plan/`. Use `plan/spec-*.md`. BLOCKING. |
| `check-existing-patterns.sh` | `before-writing-code.md` | Write | Blocks new `.go` under `internal/` when first exported type/func already exists elsewhere. BLOCKING. |
| `check-existing-tests.sh` | `before-writing-code.md` | Write | Warns when creating a test file similar to an existing one. Advisory. |
| `enforce-naming.sh` | `documentation.md` | Write | Blocks wrong file naming conventions for new files. Advisory. |
| `block-throwaway-tests.sh` | `testing.md` | Write | Blocks test files in `/tmp` or similar throwaway locations. BLOCKING. |
| `block-utils-package.sh` | `design-principles.md` | Write | Blocks creating files in `utils/`, `helpers/`, `common/`, `misc/` packages. BLOCKING. |
| `require-docs-read.sh` | `planning.md` | Write | Warns when writing a spec without evidence of reading architecture docs. Advisory. |

## PostToolUse Hooks (run after the tool completes)

### LSP

| Hook | Enforces | Triggers on | What it does |
|---|---|---|---|
| `mark-lsp-invoked.sh` | `session-start.md` | LSP | Writes freshness marker for `block-design-without-lsp.sh`. |

### Bash

| Hook | Enforces | Triggers on | What it does |
|---|---|---|---|
| Post-verify agent review | `quality.md` | Bash (`make ze-verify*`) | Spawns Sonnet agent to review all uncommitted Go changes for bugs. |
| `check-wiring-at-commit.sh` | `integration-completeness.md` | Bash (git commit) | Warns about plugin code committed without `.ci` tests. Advisory. |
| `check-doc-drift.sh` | `documentation.md` | Bash (git commit) | Warns about doc counts/lists drifting from live registry. Advisory. |

### Write/Edit

| Hook | Enforces | Triggers on | What it does |
|---|---|---|---|
| `auto_linter.sh` | `go-standards.md` | Write\|Edit | Runs `goimports -w` on Go files. BLOCKING on lint failure. |
| `auto_py_format.sh` | (code style) | Write\|Edit | Runs `ruff format` + `ruff check` on Python files. Non-blocking. |
| `validate-spec.sh` | `planning.md` | Write\|Edit | Validates `plan/spec-*.md` against required sections/format. BLOCKING. |
| `warn-deferral-in-edit.sh` | `deferral-tracking.md` | Write\|Edit | Warns when deferral language appears in spec/doc edits. Advisory. |
| `require-rfc-reference.sh` | `design-doc-references.md` | Write\|Edit | Suggests `// RFC:` header when BGP code references RFCs. Advisory. |
| `require-test-docs.sh` | `tdd.md` | Write\|Edit | Warns about test files missing `VALIDATES:`/`PREVENTS:` comments. Advisory. |
| `require-fuzz-tests.sh` | `tdd.md` | Write\|Edit | Warns about wire format parsing code without fuzz tests. Advisory. |
| `block-vague-names.sh` | `design-principles.md` | Write\|Edit | Warns about vague variable names (`data`, `info`, `result`, etc.). Advisory. |
| `require-boundary-tests.sh` | `tdd.md` | Write\|Edit | Warns about numeric validation without boundary tests. Advisory. |
| `check-file-size.sh` | `file-modularity.md` | Write\|Edit | Warns at >600 lines, strong warning at >1000 lines. Advisory. |

## Session Lifecycle Hooks

| Hook | Event | What it does |
|---|---|---|
| `session-start.sh` | SessionStart | Prints status summary (uncommitted files, active sessions, spec assignments). Creates session marker. |
| `compaction-reminder.sh` | UserPromptSubmit | Detects context compaction. Writes marker, reminds to read `post-compaction.md`. |
| `block-premature-stop.sh` | Stop | Blocks stop when last message contains ownership-dodging phrases ("would you like me to", "let me know"). BLOCKING. |
| `session-end-summary.sh` | Stop | Writes session state snapshot to per-spec state file. Cleans up session marker. |
| `session-end-deferrals.sh` | Stop | Prints open deferral count as reminder. Advisory. |
| `pre-compact-save.sh` | PreCompact | Saves session state snapshot before context compaction. |
| `subagent-context.sh` | SubagentStart | Injects compact project context (constraints, patterns, branch, spec) into spawned agents. |

## Pre-Flight Checklist by File Type

Before writing to a file, check which hooks will run:

### Any `.go` file under `internal/`

Will trigger: `pre-write-go`, `auto_linter`, `block-encoding-alloc` (wire paths), `block-format-alloc` (format files), `block-sprintf-new`, `block-legacy-log`, `block-panic-error`, `block-ignored-errors`, `block-silent-ignore`, `block-temp-debug`, `block-os-exit`, `block-init-register`, `block-yagni-violations`, `check-json-kebab`, `require-design-ref`, `require-related-refs`, `check-file-size`.

Additionally for hot-path files (reactor, event, dispatch, hub, wire, message): `check-goroutine-lifecycle`, `block-fake-bufhandle`.

### Test files (`_test.go`, `.ci`)

Will trigger: `block-test-deletion` (if removing tests), `check-existing-tests` (if new), `require-test-docs`, `require-boundary-tests`, `block-observer-sys-exit` (`.ci` with Python).

### Spec files (`plan/spec-*.md`)

Will trigger: `validate-spec`, `block-design-without-lsp` (requires recent LSP invocation), `require-docs-read` (if new), `block-source-edit-spec-not-in-progress` (if spec not `in-progress`).

### Python files (`.py`)

Will trigger: `auto_py_format` (ruff format + check).

### Commits (Bash `git commit`)

Will trigger: `block-destructive-git` (blocks), `pre-commit-spec-audit`, `check-deferral-in-diff`, `check-deferral-unassigned`, `check-wiring-at-commit`, `check-doc-drift`.
