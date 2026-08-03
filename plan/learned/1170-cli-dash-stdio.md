# 1170 -- cli-dash-stdio

## Context
The repo stated a CLI convention twice (`ai/rules/cli.md` "`-` for stdin", `ai/patterns/cli-command.md` "use `os.Stdin` when filename is `-`") but enforced it nowhere, so the filename surface drifted into four incompatible styles and ~34+ commands read/wrote user paths with raw `os` calls that ignored `-`. The reported symptom was `ze config show <file>` working but `ze config show -` failing. The goal was one shared, tier-legal helper that resolves `-`, every filename-accepting command routed through it (reads AND writes), and -- the load-bearing deliverable -- a build gate that fails any command reading/writing a user-supplied path without it.

## Decisions
- One leaf `internal/core/cliio` (ReadFile/OpenReader/Create/WriteFile) over exporting the package-private `config/cli.loadConfigData` -- a component helper importing into `analyze`/`perf`/`appliance` would invert the tier direction (`make ze-tier-check`). Accessibility of the shared thing is what determines whether sharing happens.
- Resolve `-` to `os.Stdin`/`os.Stdout`, never to the literal `/dev/stdin` -- a different fd with different permissions, non-portable, breaks for pipes.
- Streaming `OpenReader` alongside bytes `ReadFile` -- `ze analyze replay` streams multi-GB MRT through `ReadFrom`; a bytes-only helper would force `io.ReadAll` and blow memory. `ReadFile` caps stdin at 256 MB; `OpenReader` is uncapped/streaming.
- Fail closed on a second `-`: `stdinClaimed atomic.Bool` + `ErrStdinClaimed`, over a silent empty second read (which reads as a valid empty file). Stdout is not consumable, so writes are unguarded.
- Magic-byte compression sniff for stdin MRT (gzip `1f8b`, bzip2 `BZh`) over extension sniff (`-` has no extension) -- else a gzipped pipe is silently misread as raw. Real paths keep extension sniff (no regression).
- Editor gained `NewEditorFromContent` + a stdout sink (`Save()` emits to a writer) over teaching `store.ReadFile("-")` to return stdin -- keep the CLI convention at the CLI edge, not in the daemon storage layer.
- User decision (2026-07-17): `set`/`deactivate`/`activate -` become pipeline stages (stdin -> modify -> stdout); `edit`/`rollback`/`history -` are REJECTED (no TTY / no on-disk revision history for a pipe). This DEVIATES from the spec's AC-11 "rollback reads stdin, emits stdout", which was incoherent.
- The GATE does light per-package CLI-taint dataflow (direct arg / alias / range / flag-deref / parameter-fixpoint) over `direct_fs_persistence`'s flag-all-then-allowlist -- 122 raw os calls in the scanned trees made a blanket flag too noisy, and the spec forbade a gate that flags derived paths. Taint deliberately does not descend into concat/slice/call, so derived paths are never flagged (false negatives over false positives). A `//cliio:allow <reason>` inline marker exempts a genuine never-`-` site precisely (used for cmd_edit's O_EXCL atomic create) instead of blinding a whole file.

## Consequences
- Every filename-accepting command now accepts `-`; new commands must route through cliio or `make ze-dash-stdio-check` fails them (wired into ze-verify).
- The gate is worth more than the migrations: it CAUGHT 8 raw-path sites the hand-written inventory missed (tacacs, plugin-test, chaos replay/shrink/diff/writeConfig, cmd_edit). A hand inventory of a drifted surface is always incomplete; the mechanical scan is the real coverage.
- `mrt.Writer`'s sink is now `io.WriteCloser` (was `*os.File`); `pattern == "-"` writes stdout unrotated. `mrt.SniffDecompress` is exported and shared by the two MRT readers.
- The pattern doc (`ai/patterns/cli-command.md`) had to change from "use `os.Stdin`" to "use the helper" -- following it literally now fails the gate. A pattern that predates its own enforcement becomes a trap.

## Gotchas
- **The `.ci` functional harness cannot test the literal stdin fd for `ze` commands.** `internal/test/runner/runner_exec.go` materializes any `stdin=` payload into a TEMP FILE and rewrites the first `-` arg to that path (700+ existing steps depend on this: `ze -`, `ze config validate -`, `ze bgp decode -`). So `.ci` proves a command ACCEPTS `-` and its output is correct (== the real-path case), NOT the stdin routing, magic-sniff-on-stdin, or double-`-`. ALL true-stdin behavior must be unit-tested via `cliio.SwapStreams`. Do NOT "fix" the runner -- the blast radius is enormous and it is sibling-owned.
- The gate's param-taint is per-PACKAGE with a fixpoint; it does NOT track taint through return values or cross-package funnels. That is an accepted false-negative boundary (documented in the gate header), justified because the funnels are migrated once and new callers reuse them.
- A hand inventory of "~34 sites" undercounted; trust the gate, not the list.
- Running the gate against the live (migrated) tree is a free self-audit -- it flagged sites the author forgot to migrate.

## Files
- Created: `internal/core/cliio/{cliio.go,doc.go,cliio_test.go}`, `internal/component/config/cli/editor_stdin.go`, `scripts/checks/cli_dash_stdio.go`+`_test.go`, `test/ui/dash-stdio.ci`, plus `_stdin_test.go`/`reader_test.go`/`writer_test.go` in mrt/analyze/doctor/config-cli.
- Editor: `internal/component/cli/editor.go`, `editor_commands.go` (NewEditorFromContent, SetStdoutSink, Save sink).
- MRT: `internal/mrt/reader.go` (openReader, SniffDecompress), `writer.go` (io.WriteCloser sink), `internal/analyze/mrt.go`, `convert.go`.
- ~40 call-site migrations across config/cli, storage/cli, exabgp, perf, support, doctor (rename loadConfigData->loadDoctorConfig), appliance, bgp/config, hub, tacacs, plugin/cli, chaos, test/{peer,cli}.
- Gate wiring: `Makefile` (`ze-dash-stdio-check`), `scripts/status/verify_run.go`.
- Docs/rules: `ai/rules/cli.md`, `ai/patterns/cli-command.md`, `docs/features.md`, `docs/guide/command-reference.md`, `docs/guide/config-editor.md`, `docs/config-migration.md`.
