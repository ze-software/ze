---
kind: table
level:
stage:
---
| Check | Enforces | Triggers on | What it does |
|---|---|---|---|
| `c_design_without_lsp` | `.claude/rules/session-start.md`, `evidence.md` | design/spec `.md` | Blocks edits to `plan/design-*.md` / `plan/spec-*.md` unless the implementation was investigated in the last 30 min: LSP invoked OR a `.go` under `internal/`/`pkg/`/`cmd/` was read. BLOCKING. | <!-- doc-links: ignore (hook trigger patterns, files may not exist) -->
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
| `c_ci_sleep_justification` | `testing.md` | `.ci` | Warns when a `time.sleep(` is introduced with no comment above/trailing it. Advisory (blocking gate is `make ze-verify-wiring-docs`). |
| `c_hardcoded_commands` | `evidence.md` | `.go` | Blocks hardcoded command-list literals. BLOCKING. |
| `c_switch_dispatch` | `plugins.md` | `.go` | Blocks `switch args[0]` subcommand dispatch; use `subdispatch.New()` + `Register()`. BLOCKING. |
| `c_json_kebab` | `cli.md`, `go-standards.md` | `.go` | Blocks non-kebab-case JSON tags. BLOCKING. |
| `c_goroutine` | `goroutine-lifecycle.md` | hot-path `.go` | Blocks `go func()` in reactor/event/dispatch/hub/wire/message. BLOCKING. |
| `c_require_design_ref` | `go-standards.md` | `.go` | Blocks Go files without `// Design:` comment. BLOCKING. |
| `c_require_related_refs` | `go-standards.md` | `.go` | Blocks missing/stale `// Related:`/`// Detail:`/`// Overview:` refs. BLOCKING. |
| `c_test_weakening` | `testing.md` | test files | Blocks deleting OR weakening tests: removed funcs/cases/assertions, added `t.Skip`, `require`->`assert` downgrade, commented-out asserts, `ignore` build tag. Escape: `// test-relax: <reason>`, or `# test-relax: <reason>` on a carrier that comments with `#` (`_has_relax_token`). BLOCKING. |
| `_rfc_tagged_change_err` | `testing.md`, `ai/skills/ze-rfc.md` | any tag CARRIER holding `RFC requirement:` -- `_test.go`, `.ci`, `.et`, an interop `check.py` | The guard is `_rfc_tagged_change_err`, called from `c_test_weakening`. Blocks ANY behavior change to a test that proves an RFC obligation. It runs BEFORE test-weakening. `// test-relax:` does NOT satisfy it, because self-service justification is not user approval. Also blocks DELETING the tag (checked first, since a tag is a comment and the behavior comparison would pass its removal). Scope is the ENCLOSING test function (`_enclosing_tagged_scope`, which now delegates to `scripts/dev/rfc_tagged_scope.py`), not the edited hunk. So a tag on the doc comment still governs a body edit. Untagged functions in the same file are unaffected. A tag outside every function, such as a hoisted table, widens scope to the whole file. Every occurrence of a hunk is considered, so `replace_all` cannot reach a tagged copy unseen. Comment/format edits pass -- `#` counts as the comment syntax for `.ci`, `.et` and `.py`; a rename blocks. A `.go` edit made ONLY of import lines passes too (`_import_only_go_edit`): an import cannot weaken an assertion, and without it GROWING a tagged file always cost an approval, because new tests need new imports and an import block sits outside every function so the scope widens to the whole file. Every non-blank line on both sides must be import-shaped, so an assertion smuggled into the same edit still blocks, and the tag-removal check runs first so a tag cannot ride out on the exemption. Escape: `// rfc-test-change-approved: <date> <what the user approved>`, and only the user may authorize it. **The marker must sit in the replacement text of the edit itself**, since that is the only text the check reads; the same marker elsewhere in the file does not satisfy it. BLOCKING. **The carrier list is derived** from the shared leaf. `TestTaggedScopeCoversEveryCarrier` holds it against the scanner's own `CARRIERS`. Until 2026-07-29 the predicate was a literal covering `_test.go` and a `/test/` `.ci` only. The two interop `check.py` files that `plan/learned/1296-rfcgate-2-evidence.md` admits as evidence therefore carried RFC obligations this guard could not see. |
| `c_system_tmp_we` | `testing.md` | any | Blocks writing to `/tmp`. BLOCKING. |
| `c_generated_files` | `repo-maintenance.md`, "Canonical Sources and Sync Direction" above | `CLAUDE.md`/`AGENTS.md` | Blocks editing generated files. BLOCKING. |
| `c_rendered_rules` | `repo-maintenance.md`, "Canonical Sources and Sync Direction" above | any `*.md` sitting DIRECTLY in `ai/rules/` | Blocks editing a rendered rule and names the point to edit instead. Also covers `INDEX.md`, `TRIGGERS.md` and `CORE.md`, which no hook guarded before, and points each at its own generator. `ai/rules/points/**` is the canonical source and is always permitted: a point's parent is `ai/rules/points/<rule>`, so the dirname test lets it through. Matched by realpath against `CLAUDE_PROJECT_DIR`, for the reason `generated-files` records, and it refuses rather than permits when the path cannot be resolved. BLOCKING. |
| `c_point_overwrite` | `never-destroy-work.md` | a `Write` to a path under `ai/rules/points/<rule>/` | Blocks a `Write` over a point file that already exists, and names both non-destructive routes: edit that point, or pick a slug no file uses. A `Write` to a NEW path is how a point is authored and stays permitted, and `Edit`/`MultiEdit` are targeted so neither can silently drop a body. The render gates report the same damage one step too late: the instruction is gone at write time and only git holds it. BLOCKING. |
| `c_line_number_ref` | `evidence.md`, `writing.md` | `.md` under `ai/`, `docs/`, `plan/`, `.claude/` | Blocks a `path:NN` line citation and a `#LNN` permalink anchor in prose. Cite the file and the symbol instead. A fenced block, an `rfc/full/` path, and a file declaring itself generated in its first ten lines are all exempt. `scripts/dev/line_refs.py --apply` sweeps an existing file. BLOCKING. |
| `c_claude_plans` | `.claude/rules/planning.md` (no point) | Write | Blocks `.claude/plans/` and `~/.claude/plan/`. BLOCKING. | <!-- doc-links: ignore (banned location, deliberately nonexistent) -->
| `c_check_existing_patterns` | `architecture.md` | new `internal/**/*.go` | Blocks duplicate exported type/func in same package. BLOCKING. |
| `c_check_existing_tests` | (no point: the function is a no-op) | new test files | Warns about similar existing tests. Advisory. |
| `c_enforce_naming` | (no point: the file-naming convention is unwritten) | new files | Warns on wrong file naming. Advisory. |
| `c_throwaway_tests` | `testing.md` | Write | Blocks test files in `/tmp` and throwaway locations. BLOCKING. |
| `c_utils_package` | `architecture.md` | Write `.go` | Blocks `utils/`/`helpers/`/`common/`/`misc/` packages. BLOCKING. |
| `c_direct_fs_state` | `architecture.md` | `.go` under `internal/plugins/`, `internal/component/`, `cmd/ze/` | Warns on `os.WriteFile`/`Create`/`Rename`/`Symlink`/`Link` and a creating `os.OpenFile`. Runtime state belongs in `internal/core/statestore` or a `storage.Storage` handle, because only `database.zefs` is managed on the appliance. The blocking gate is `make ze-fs-persistence-check`. Advisory. |
| `c_require_test_first` | `testing.md` | new `.go` | Warns when creating impl without a test file. Advisory. |
| `c_require_docs_read` | `.claude/rules/post-compaction.md` (no point) | new spec | Warns when writing a spec without session-state evidence. Advisory. |
