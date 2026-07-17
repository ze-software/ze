# Spec: cli-dash-stdio

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/8 |
| Updated | 2026-07-17 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `ai/rules/cli-patterns.md` (line 43) and `ai/patterns/cli-command.md` (line 314) - the rule this spec enforces
4. `internal/component/config/cli/main.go` (`loadConfigData`), `internal/mrt/reader.go` (`openReader`, `ReadFrom`), `internal/component/cli/editor.go` (`NewEditorWithStorage`)
5. `scripts/checks/direct_fs_persistence.go` - the model for the Phase 7 gate

## Task

Every ze command line that accepts a filename must accept `-` as an alias for stdin
(when reading) and stdout (when writing). Today most do not.

**This is not a new convention. It is drift from a rule the repo already states twice:**

| Source | Text |
|--------|------|
| `ai/rules/cli-patterns.md:43` | "`-` for stdin, `--json` for JSON output" |
| `ai/patterns/cli-command.md:314` | "Stdin \| `-` means stdin. Use `os.Stdin` when filename is `-`" |

Nothing enforces it, so the surface drifted into four incompatible styles:

| Style | Count | Examples |
|-------|-------|----------|
| Uses the shared `loadConfigData` helper (supports `-`) | 6 call sites | `internal/component/config/cli/cmd_completion.go:74`, `cmd_graph.go:41`, `cmd_dump.go:53`, `cmd_fmt.go:86`, `cmd_migrate.go:134`, `:267` |
| Re-implements `if path == "-"` by hand | 3 commands | `internal/component/config/cli/cmd_validate.go:177-190`, `cmd_fix.go:47-50`, `cmd_diff.go:120-127` |
| Reads a user-supplied path with no `-` branch at all | ~24 commands | `internal/component/config/cli/cmd_show.go:30` (the reported case), `cmd_import.go:62`, `internal/mrt/reader.go:55`, `internal/analyze/mrt.go:83`, and the full inventory below |
| Writes a user-supplied path with no `-` branch | ~10 commands | `internal/analyze/convert.go:37`, `internal/perf/cli/cmd_run.go:178`, and the inventory below |

The reported case, `ze config show`: `cmdShow` (`internal/component/config/cli/cmd_show.go:30`)
calls `showConfig` (`:36`), which takes `configFile := fs.Arg(0)` (`:70`) and passes it to
`editor.NewEditorWithStorage` (`:73`), which reads at `internal/component/cli/editor.go:72`.
It never touches `loadConfigData` (`internal/component/config/cli/main.go:115`), the helper
that does implement `-` (`:116-118`).

**Goal:** one shared helper that resolves `-`, every filename-accepting command routed
through it, and **a gate that fails when a new command reads or writes a user-supplied
path without it**. The gate is the load-bearing deliverable: the rule has existed this
whole time and produced ~34 violations because nothing checked it.

### Concurrent Work (READ FIRST)

A sibling spec, **`spec-cli-root-namespace-grammar`**, is being implemented in parallel
elsewhere. It is NOT a blocking prerequisite (`Depends | -`): the two specs touch
disjoint concerns and can land in either order. But it **renames files this spec cites**,
so verify before trusting a citation:

| It renames | To | This spec's affected citations |
|-----------|-----|-------------------------------|
| `cmd/ze/ze_core_format.go` (`runFormat`, `formatUsage`) | `cmd/ze/ze_core_pipe.go` (`runPipe`, `pipeUsage`) | Security Review "Unbounded stdin" row (the 256 MB `maxStdin` cap) |
| root command `format` | root command `pipe` | none (this spec does not touch the pipe carrier) |
| roots `traffic-control`, `isis-decode`, `ospf-decode` | `traffic control`, `isis decode`, `ospf decode` | none — the sweep confirmed these take **no** filename arguments |
| root `update-serve` | local handler `update serve` | none — takes no filename argument |

If a cited path is missing, it has been renamed, not deleted: grep for the **symbol**
(`maxStdin`, `runPipe`) rather than assuming the spec is stale. Nothing in the inventory
below overlaps with that spec's Files to Modify.

### Scope decisions (user, 2026-07-17)

| Question | Decision |
|----------|----------|
| Scope | **One spec, phased.** Not an umbrella set |
| Writes | **Reads + writes.** `-` means stdout for output paths, mirroring `cmd_fmt.go:127` which already does this for `fmt -w -` |
| Editor-backed commands | **All of them.** `set`/`deactivate`/`activate`/`rollback` with `-` read stdin and emit the modified config to **stdout** instead of writing back. This changes what those commands can do (they become pipeline stages), which is the intent |

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] — checkboxes are template markers, not progress trackers. -->
<!-- Capture insights as → Decision: / → Constraint: annotations — these survive compaction. -->
- [ ] `ai/rules/cli-patterns.md` - the rule being enforced
  → Constraint: `:43` "`-` for stdin". This spec does not invent policy; it implements and gates an existing one.
- [ ] `ai/patterns/cli-command.md` - the structural template
  → Constraint: `:314` "`-` means stdin. Use `os.Stdin` when filename is `-`". The pattern says `os.Stdin` directly; this spec centralises that into one helper so the gate has something to check for. The pattern text must change in Phase 8 or it will instruct a practice the gate rejects.
- [ ] `ai/rules/module-tiers.md` - where the new helper package belongs
  → Constraint: `:17` `internal/core/` is for "a library you cannot run as a plugin. Foundational; no config-driven lifecycle." The helper is consumed by `internal/component/*`, `internal/plugins/*`, `internal/analyze`, `internal/perf`, `internal/test`, and `internal/appliance`, so it MUST be a leaf under `internal/core/` or `make ze-tier-check` fails.
- [ ] `ai/rules/fail-closed-guards.md` - the stdin-once guard
  → Constraint: stdin can be consumed exactly once. A second `-` in a multi-arg command MUST error, never silently read empty. "A zero value must never be a valid-looking answer" — an empty second read is exactly that.
- [ ] `ai/rules/no-layering.md` - replacement discipline
  → Constraint: the three ad-hoc `-` branches and the package-private `loadConfigData` are DELETED when their callers move to the shared helper. Never both.
- [ ] `scripts/checks/direct_fs_persistence.go` - the gate model (read the header, `:1-33`)
  → Decision: the shape to copy — AST scan over `internal/`+`cmd/ze` non-test files, allowlist by directory prefix or by file with a stated reason, `--json`/`--selftest`, wired into `make` and `ze-verify` via `scripts/status/verify_run.go`.
  → Constraint: its invariant reads "use `internal/core/statestore`, never a raw os write". The Phase 7 invariant is the direct analogue: "use the path helper, never a raw `os.ReadFile`/`os.Open`/`os.Create`/`os.WriteFile` on a **user-supplied** path".

### RFC Summaries (MUST for protocol work)
- N/A - no protocol behavior. This is a CLI I/O convention.

**Key insights:**
- The fix is one leaf package plus ~34 call-site migrations. The hard parts are not the migrations; they are four specific obstacles: the editor's path reuse, MRT compression sniffing, the stdin-once guard, and the gate's definition of "user-supplied".
- `internal/mrt/reader.go:55` `openReader` is the single highest-leverage site: ten `ze analyze` subcommands funnel through it, it already branches on a URL scheme at `:56`, and `ReadFrom(io.Reader, *Handler)` at `:50` is already the stdin-ready entry point.
- `ze -` (config from stdin) already works and is the in-tree precedent: `zeDispatch` handles it at `cmd/ze/ze_core_dispatch.go:403-408`, short-circuiting **before** `config.ResolveConfigPath(arg)` at `:410`.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
<!-- Same rule: never tick [ ] to [x]. Write → Constraint: annotations instead. -->
- [ ] `internal/component/config/cli/main.go` - `loadConfigData(path)` (`:115`): `if path == "-"` (`:116`) returns `io.ReadAll(os.Stdin)` (`:117`), else `os.ReadFile(path)` (`:119`).
  → Constraint: **package-private**, so `analyze`, `exabgp`, `data`, `perf`, `support`, `doctor`, and `appliance` cannot reach it. That inaccessibility is the direct cause of the drift. It is superseded by the new leaf package and deleted.
- [ ] `internal/component/config/cli/cmd_show.go` - `cmdShow` (`:30`) → `showConfig` (`:36`); `configFile := fs.Arg(0)` (`:70`); `editor.NewEditorWithStorage(store, configFile)` (`:73`). Help/usage/examples at `:39-57`. Header comment `:28-29` says it "reads a config file directly from the filesystem (not the blob store), so a plain path works without `-f`" and says nothing about `-`.
- [ ] `internal/component/cli/editor.go` - `NewEditor(configPath)` (`:64`) delegates to `NewEditorWithStorage(store, configPath)` (`:70`), which reads at `:72` via `store.ReadFile(configPath)`.
  → Constraint: the doc comment at `:69` states "All file I/O (config, draft, backup, lock) goes through the storage interface". `configPath` is retained as `originalPath` and reused to derive `configPath + ".edit"` and draft/backup/lock paths. **`-` cannot simply flow through this constructor**: it is not only a read source, it is an identity. This is the central obstacle of Phase 3.
- [ ] `internal/mrt/reader.go` - `ReadFile(filename, handler)` (`:36`) → `openReader(filename)` (`:37`, defined `:55`) → `ReadFrom(rc, handler)` (`:41`, defined `:50`). `openReader` branches on `http://`/`https://` at `:56-58`, then `os.Open` at `:60`, then sniffs compression **by filename extension**: `lower := strings.ToLower(filename)` (`:64`), `.gz` (`:66`), `.bz2` (`:75`), `default:` returns the raw file (`:77-78`).
  → Constraint: `-` has no extension, so it falls to `default:` (`:77`) and is treated as **uncompressed**. A gzipped stream piped in would be misread as raw MRT. Extension sniffing cannot work for stdin; Phase 4 needs magic-byte sniffing (gzip `1f 8b`, bzip2 `BZh`) via a buffered peek.
  → Constraint: `:59-60` read `cleaned := filename // caller-controlled path; no user input` and `os.Open(cleaned) //nolint:gosec // path is from CLI args, not user input`. The code asserts CLI args are not user input. That belief is the blind spot this spec closes, and the comments must be corrected, not preserved.
- [ ] `internal/analyze/mrt.go` - `processMRTFile` (`:83`) `os.Open(filename)` (`:84`) with its own independent compression sniff (`:89-100`).
  → Constraint: a **second, duplicate MRT reader**. Fixing `openReader` alone does NOT cover the six stats commands (`aspath`, `attributes`, `communities`, `count-attrs`, `dump`, `density`). Phase 4 must handle both or unify them (see Key Design Decisions).
- [ ] `internal/component/config/environment.go` - `ResolveConfigPath(path)` (`:269`): `if path == "-" || strings.HasPrefix(path, "/") { return path }` (`:271`).
  → Constraint: `-` already passes through untouched. Any command routing a path through this inherits `-` safely. The helper must be applied AFTER resolution, never before.
- [ ] `cmd/ze/ze_core_dispatch.go` - `looksLikeConfig(arg)` returns true for `-` (`:578`); `zeDispatch` special-cases `arg == "-"` at `:403-408`, calling `hub.Run` with `-` directly and **skipping** `config.ResolveConfigPath(arg)` at `:410`.
  → Constraint: the working precedent. `ze -` reads a config from stdin today.
- [ ] `internal/component/doctor/doctor.go` - `loadConfigData(store, configPath)` (`:179`) `os.ReadFile(configPath)` (`:181`), no `-` branch.
  → Constraint: **name collision** with `internal/component/config/cli/main.go:115`. Two functions, same name, different signatures, different `-` behavior. Rename this one in Phase 5 so the next reader cannot cite the wrong producer.
- [ ] `internal/component/config/cli/cmd_fmt.go` - `-w` write-back at `:132` `os.WriteFile(configPath, ...)`, with `configPath == "-"` special-cased at `:109` and `:127` to write to stdout instead of erroring; help documents it at `:67`.
  → Constraint: the in-tree precedent for `-` on the WRITE side, and the model for Phase 6.
- [ ] `scripts/checks/direct_fs_persistence.go` - header `:1-33`: scans `internal/plugins`, `internal/component`, `cmd/ze` (non-test) for write primitives, allowlists legitimate writers by directory prefix or by file with a stated reason, `--json`/`--selftest`, wired via `make ze-fs-persistence-check` and `scripts/status/verify_run.go`.
- [ ] `internal/core/paths/doc.go` - "Package paths resolves the ze configuration directory from the running binary's location."
  → Decision: NOT the home for this helper. Different concern. A new leaf package is required.

**Behavior to preserve:**
- `ze -` continues to read a config from stdin (`cmd/ze/ze_core_dispatch.go:403-408`).
- Every currently-working `-` keeps working: the 6 `loadConfigData` callers, the 3 ad-hoc branches, `ze config fmt -w -`, `ze data encode -`.
- `ResolveConfigPath("-")` returns `-` unchanged (`internal/component/config/environment.go:271`).
- Commands whose path is *derived* rather than supplied (`analyze download`, `appliance export`, `appliance push`) are untouched.
- Blob-key arguments (`config cat`, `data cat`) are untouched: a key is not a path.
- All existing exit codes and error text for genuine file errors.

**Behavior to change:**
- ~24 read commands gain `-` → stdin; ~10 write commands gain `-` → stdout.
- `ze config set/deactivate/activate/rollback <file>` with `-` read stdin and emit the modified config to **stdout** instead of writing back (user decision).
- A second `-` among multi-file args becomes a hard error (today: a silent empty read).
- MRT compression detection for `-` switches from extension sniffing to magic bytes.
- `internal/component/doctor/doctor.go:179` `loadConfigData` is renamed to end the collision.
- A new gate fails any command reading/writing a user-supplied path outside the helper.

### Full inventory: READ commands

Producers that consume a user-supplied path. `-` column = supported today. This is
the reference appendix for the summary above; every row cites its producer.

| Command | Producer file:line | `-` | Notes |
|---------|-------------------|-----|-------|
| `ze <config.conf>` / `ze start` / `ze -` | `cmd/ze/hub/main.go:155` → `:211` | **yes** | Precedent. Gated at `cmd/ze/ze_core_dispatch.go:404` |
| `ze config completion` | `internal/component/config/cli/cmd_completion.go:74` | yes | via `loadConfigData` |
| `ze config graph` | `internal/component/config/cli/cmd_graph.go:41` | yes | via `loadConfigData` |
| `ze config dump` | `internal/component/config/cli/cmd_dump.go:53` | yes | via `loadConfigData` |
| `ze config fmt` | `internal/component/config/cli/cmd_fmt.go:86` | yes | via `loadConfigData` |
| `ze config migrate` | `internal/component/config/cli/cmd_migrate.go:134`, `:267` | yes | via `loadConfigData` |
| `ze config validate` | `internal/component/config/cli/cmd_validate.go:177-190`; draft at `:228` | adhoc | hand-rolled branch; `:228` draft path has none |
| `ze config fix` | `internal/component/config/cli/cmd_fix.go:47-50` | adhoc | hand-rolled |
| `ze config diff` | `internal/component/config/cli/cmd_diff.go:120-127` | adhoc | hand-rolled |
| `ze config show <file>` | `internal/component/cli/editor.go:72` via `cmd_show.go:73` | **no** | **the reported case** |
| `ze config edit [file]` | `internal/component/cli/editor.go:72` via `cmd_edit.go:486` | **no** | path reused for draft/backup/lock and passed as argv to a spawned daemon (`cmd_edit.go:85`, `:138`) |
| `ze config set <file> <path> <val>` | `internal/component/cli/editor.go:72` via `cmd_set.go:86` | **no** | read-modify-write-back |
| `ze config deactivate/activate <file> <path>` | `internal/component/cli/editor.go:72` via `cmd_deactivate.go:115` | **no** | read-modify-write-back |
| `ze config history <file>` | `internal/component/cli/editor.go:72` via `cmd_history.go:54` | **no** | read-only |
| `ze config rollback <N> <file>` | `internal/component/cli/editor.go:72` via `cmd_rollback.go:62` | **no** | file is `fs.Arg(1)`, NOT arg 0 |
| `ze config import <file>...` | `internal/component/config/cli/cmd_import.go:62` | **no** | **MULTI-ARG** (`fs.Args()` `:44`); `--name` rejected with >1 file (`:57`); loop `continue`s on read error |
| `ze config cat <key>` | `internal/component/config/cli/cmd_ls.go:84` | n/a | **blob key, not a path** — excluded (not a user filename path) |
| `ze data write <key> <file>` | `internal/component/config/storage/cli/main.go:128` | **no** | path is arg **1** |
| `ze data import <file>...` | `internal/component/config/storage/cli/main.go:171` | **no** | **MULTI-ARG** |
| `ze data cat <key>` | `internal/component/config/storage/cli/main.go:274` | n/a | blob key — excluded (not a user filename path) |
| `ze data encode <string\|->` | `internal/component/config/storage/cli/cmd_integrity.go:116-118` | adhoc | argument is a **literal string**, not a filename — excluded (not a user filename path), but confirms `-` is idiomatic here |
| `ze exabgp migrate <config-file>` | `internal/plugins/exabgp/main.go:249` | **no** | arg at `:246`; output already stdout (`:288`) |
| `ze exabgp migrate --env <env-file>` | `internal/plugins/exabgp/main.go:297` | **no** | separate producer, dispatched `:238` |
| `ze analyze {aspath,attributes,communities,count-attrs,mrt-dump,density}` | `internal/analyze/mrt.go:84` in `processMRTFile` (`:83`) | **no** | **duplicate reader**; `communities.go:95`, `aspath.go:79` are **MULTI-ARG** |
| `ze analyze {show,routes,statistics,convert,export,inject,replay,filter,serve}` | `internal/mrt/reader.go:60` in `openReader` (`:55`) | **no** | **10 subcommands, one choke point**; `statistics.go:59` MULTI-ARG |
| `ze test peer [expect-file]` | `internal/test/peer/expect.go:65` in `LoadExpectFile` (`:64`) via `internal/test/cli/cmd_peer.go:142` | **no** | comment at `:65` says "Path from user input (CLI arg)" |
| `ze test engine-steps <steps.json>` | `internal/test/cli/cmd_engine_steps.go:36` | **no** | runner-materialized, not operator-facing |
| `ze perf report <file>...` | `internal/perf/cli/cmd_report.go:40` | **no** | **MULTI-ARG** (`fs.Args()` `:31`) |
| `ze perf track <history.ndjson>` | `internal/perf/cli/cmd_track.go:45` | **no** | rejects >1 file at `:40` |
| `ze support [config]` | `internal/component/support/support.go:403` | **no** | positional at `:118-123`; falls back to store when empty |
| `ze show doctor [config]` | `internal/component/doctor/doctor.go:181` in `loadConfigData` (`:179`) | **no** | **name collision** (see above); arg at `:48`; RPC path `internal/component/doctor/cmd/show.go:26` |
| `ze appliance init --config <f>` | `internal/appliance/config.go:345` in `LoadConfig` (`:344`) via `cmd_init.go:88` | **no** | |
| `ze appliance init --batch <manifest>` | `internal/appliance/cmd_init.go:329` | **no** | dispatched `:62` |
| `ze appliance init --cert/--key` | `internal/appliance/cmd_init.go:276`, `:280` | **no** | |
| `ze appliance cert --cert/--key` | `internal/appliance/cmd_cert.go:65`, `:70` | **no** | |
| `ze appliance import <archive>` | `internal/appliance/cmd_import.go:89` in `importArchive` (`:88`) | **no** | arg at `:48`; encrypted blob |
| `ze appliance push` | `internal/appliance/cmd_push.go:140` | n/a | `--image` selects *within* the appliance dir, not an arbitrary path — excluded (not a user filename path) |

### Full inventory: WRITE commands (`-` means stdout)

| Command | Producer file:line | `-` | Notes |
|---------|-------------------|-----|-------|
| `ze config fmt -w <file>` | `internal/component/config/cli/cmd_fmt.go:132`; `-` cased at `:109`, `:127` | **adhoc** | the precedent; help at `:67` |
| `ze config migrate -o <file>` | `internal/component/config/cli/cmd_migrate.go:308` | **no** | flag `:18`; empty `-o` already means stdout (`:313`) |
| `ze analyze filter <in> <out>` | `internal/mrt/writer.go:51` `NewWriter` via `internal/analyze/filter.go:165` | **no** | **two file args**: in `:134`, out `:135` |
| `ze analyze convert <in> <out>` | `internal/analyze/convert.go:37` | **no** | two file args (`:34`, `:35`) |
| `ze analyze record bmp <out>` | `internal/mrt/writer.go:51` via `internal/analyze/record_bmp.go:87` | **no** | out `:85` |
| `ze analyze download <date> <slot>` | `internal/analyze/download.go:152` | n/a | path **derived**, not supplied — excluded (not a user filename path) |
| `ze perf run --output <f>` | `internal/perf/cli/cmd_run.go:178` in `writeJSONResult` (`:176`) | **no** | flag `:55`; empty → stdout |
| `ze chaos --config-out <f>` | `internal/chaos/orchestrator/cli.go:552` | **no** | flag `:112`; help says "instead of stdout" |
| `ze chaos --event-log <f>` | `internal/chaos/orchestrator/run.go:386` | **no** | flag `:113` |
| `ze chaos --mrt-file <f>` | `internal/chaos/orchestrator/run.go:411` `report.NewMRTLog` | **no** | flag `cli.go:114`; a **writer** with strftime rotation despite the name — `-` disables rotation, so it must be rejected or documented |
| `ze appliance export <name>` | `internal/appliance/cmd_export.go:127`, `:167` | n/a | path **derived** from name — excluded (not a user filename path) |

### Existing `-` normalisers to be superseded

| Location | Behavior | Fate |
|----------|----------|------|
| `internal/component/config/cli/main.go:115` `loadConfigData` | `-` → `io.ReadAll(os.Stdin)` (`:117`) | **deleted**; callers move to the leaf helper |
| `internal/component/hub/config.go:26` `LoadHubConfig`, `-` branch at `:30` | supports `-` | **dead in production**: only callers are `test/hub/startup_test.go:46` and `internal/component/hub/config_test.go:130`. Confirm (A-6) and handle per the dead-code check |
| `internal/component/bgp/config/loader.go:152` in `loadReactorFile` (`:149`) | supports `-` | reachable via `LoadReactorFile*` (`:129-147`); migrate to the helper |
| `internal/component/plugin/cli/cli.go:177` `resolveHexInput` → `readHexFromStdin` (`:167`) | `-` reads one line | **value, not filename**. Excluded; do NOT migrate |

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Operator supplies a path token in argv (positional or flag value). The token is either a real path, `-`, or (for MRT) a URL.

### Transformation Path
1. Command's flag set parses argv; the path lands in `fs.Arg(n)` or a flag var.
2. Where applicable, `config.ResolveConfigPath` normalises it, passing `-` through untouched (`internal/component/config/environment.go:271`).
3. The command calls the new leaf helper instead of a raw `os` call.
4. **Read path:** helper returns `io.ReadAll(os.Stdin)` (or a reader over stdin) for `-`, else opens/reads the file. The stdin-once guard is claimed here.
5. **Write path:** helper returns a writer over stdout for `-`, else creates the file.
6. **MRT:** `openReader` (`internal/mrt/reader.go:55`) returns a reader for `-`; compression is detected by peeking magic bytes rather than the extension; `ReadFrom` (`:50`) consumes it unchanged.
7. **Editor:** content + a synthetic identity are supplied instead of a path; the save sink is stdout when the input was `-`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Shell ↔ ze | argv path token + stdin bytes + stdout bytes + exit code | [ ] |
| CLI command ↔ helper | the new `internal/core/<helper>` leaf API | [ ] |
| Helper ↔ OS | `os.Stdin` / `os.Stdout` / `os.Open` / `os.Create` | [ ] |
| Command ↔ editor | content + identity instead of a path (Phase 3) | [ ] |
| Gate ↔ source tree | AST scan over `internal/` + `cmd/ze` non-test files (Phase 7) | [ ] |

### Integration Points
- `internal/mrt/reader.go:50` `ReadFrom(io.Reader, *Handler)` - already the stdin-ready entry point; `openReader` (`:55`) is the only thing standing between it and stdin.
- `internal/component/config/environment.go:269` `ResolveConfigPath` - already `-`-safe; the helper composes after it.
- `scripts/status/verify_run.go` - where the Phase 7 gate wires into `ze-verify`.

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate) — explicitly: the helper REPLACES four partial normalisers rather than becoming a fifth
- [ ] Zero-copy preserved where applicable (uses refs, not copies) — the reader API must not force `io.ReadAll` on MRT, which streams multi-GB files
- [ ] Registration over hardcoding — N/A: this is a shared library, not a registered feature

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A new `internal/core/` leaf package is importable by every consumer without a tier violation | `ai/rules/module-tiers.md:17`; consumers span `internal/component`, `internal/plugins`, `internal/analyze`, `internal/perf`, `internal/test`, `internal/appliance` | The helper cannot be shared and the drift is structural, not accidental | `make ze-tier-check` after Phase 1 | confirmed (audit): `ai/rules/module-tiers.md:30` "pure library, no sdk.NewWithConn -> internal/core/<x>"; helper imports only os/io/bufio/compress; core import-direction rule only forbids importing component/plugins, which it does not. Re-checked by `make ze-tier-check` after Phase 1 |
| A-2 | `ReadFrom` (`internal/mrt/reader.go:50`) is sufficient for stdin MRT without buffering the whole stream | It takes `io.Reader` and wraps it in a 64 KB `bufio.Reader` (`:51`) | Stdin MRT would need full buffering, breaking multi-GB replay | Phase 4 functional test piping a large MRT | confirmed: `OpenReader("-")` returns a streaming `io.ReadCloser` over stdin; `SniffDecompress` buffers only the 3-byte magic peek. `ReadFrom` consumes it unchanged. No full-stream buffering. TestOpenReaderDash streams raw + real-file |
| A-3 | Magic-byte sniffing can replace extension sniffing for `-` without changing behavior for real paths | `openReader` sniffs `.gz`/`.bz2` by suffix (`internal/mrt/reader.go:64-79`); gzip is `1f 8b`, bzip2 is `BZh` | Compressed stdin is misread as raw; or worse, real files regress | Phase 4: unit tests over both, plus a functional test piping gzipped MRT | confirmed: `SniffDecompress` detects gzip `1f8b`/bzip2 `BZh` on the stdin stream; real paths keep extension sniff (`wrapByExtension`). TestOpenReaderMagicSniff (gzip round-trip + embedded bzip2 fixture + raw passthrough + `.gz` file). TestProcessMRTFileDash covers the duplicate reader |
| A-4 | The editor can be given content + a synthetic identity without breaking draft/backup/lock | `internal/component/cli/editor.go:69-72` doc comment says all file I/O (config, draft, backup, lock) goes through storage keyed off `configPath` | Phase 3 is much larger: the editor's identity model needs rework, affecting `edit`/`set`/`rollback` beyond `-` | Phase 3: read the full editor, then a failing-then-passing `ze config show -` | confirmed: `NewEditorFromContent(content, identity)` reuses the shared parse path (`newEditor`), skips the on-disk `.edit` probe, and `SetStdoutSink(w)` routes `Save()` to stdout. No identity rework needed for set/deactivate/activate. TestEditorFromContent, TestEditorStdoutSink pass |
| A-5 | `ze config edit -` is coherent, or is explicitly rejected | `cmd_edit.go:85`, `:138` pass the path as argv to a **spawned daemon**; an interactive editor over a consumed stdin has no TTY story | `edit` silently misbehaves | Phase 3: decide and test. Rejecting `edit -` with a clear error is an acceptable outcome and must be an explicit AC, not an omission | confirmed: `edit -` REJECTED with a clear error (user decision 2026-07-17). Same decision extended to `rollback N -` and `history -` (they need on-disk revision history a pipe lacks). TestEditRejectsStdin, TestRollbackRejectsStdin, TestHistoryRejectsStdin pass |
| A-6 | `LoadHubConfig` (`internal/component/hub/config.go:26`) is dead in production | Only callers found are `test/hub/startup_test.go:46` and `internal/component/hub/config_test.go:130` | It is live and must be migrated, not removed | `grep`/LSP `findReferences` at audit; the dead-code check in the Completion Checklist | confirmed (audit): grep over all `.go` finds only `config_test.go:130` + `test/hub/startup_test.go:46` as callers. Dead in production. Fate decided in Phase 5 per dead-code check (ASK before removing) |
| A-7 | The gate can mechanically distinguish a user-supplied path from an internally-derived one | `direct_fs_persistence.go` solves the analogous problem with an allowlist rather than dataflow analysis (`:20-25`) | The gate is either noisy (flags `analyze download`, `appliance export`) or toothless | Phase 7: the gate must flag all ~34 pre-migration sites on a fixture and zero after (R-2) | confirmed: went BEYOND allowlist to light per-package CLI-taint dataflow (direct/alias/range/flag-deref/param-fixpoint). Selftest proves it flags the violation shapes and stays quiet on derived (`p[1:]`), `filepath.Join`, `store.ReadFile`, cliio, and `//cliio:allow`-marked sites. Live tree: 0 false positives, and it CAUGHT 8 real sites the inventory missed |
| A-8 | `-` as a literal filename is not a real use case worth preserving | Universal CLI convention; `ze -` already means stdin (`cmd/ze/ze_core_dispatch.go:404`); `ResolveConfigPath` already reserves it (`:271`) | A user with a file named `-` loses access; conventional escape is `./-` | User confirmation; document `./-` in the guide | confirmed: universal convention; `./-` escape documented in `docs/guide/command-reference.md` Conventions section. `IsStdin` treats ONLY the exact token `-` as stdin (TestIsStdin: `./-`, `-x` are not stdin) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Stdin is consumed twice in a multi-arg command and the second read silently returns empty, which looks like an empty file rather than an error | `ze config import a.conf - -` succeeds with a surprising result | The stdin-once guard is fail-closed (`ai/rules/fail-closed-guards.md`): the second claim returns an error naming the conflict. AC-9 asserts it |
| R-2 | The gate is written to pass on the migrated tree and never proves it catches the class | The gate flags zero sites when run against the pre-migration tree | Phase 7 requires demonstrating the gate flagging the pre-migration sites on a fixture BEFORE it is wired in. A gate that cannot catch the bug it was written for is not a gate |
| R-3 | The editor's stdout-sink mode changes `set`/`rollback` semantics and quietly breaks a script that expected a write-back | A `.ci` test or demo that runs `ze config set <file> ...` and reads the file back | The stdout sink is entered ONLY when the path is `-`; every real path behaves exactly as before. AC-13 asserts the unchanged path |
| R-4 | Compressed MRT on stdin is silently misread as raw, producing garbage records rather than an error | `ze analyze show -` with gzipped input yields zero or corrupt records | A-3's magic-byte sniff; a functional test piping gzipped MRT. If sniffing proves infeasible, reject compressed stdin explicitly rather than misread it |
| R-5 | `-` is added to a command whose path is derived, creating a meaningless surface | Review finds `-` on `analyze download` / `appliance export` | The inventory marks derived-path commands `n/a`; the gate allowlists them with a stated reason |
| R-6 | Migrating ~34 sites in one spec produces a diff too large to review, and a real regression hides in it | The diff crosses 9 subsystems | Phases are per-subsystem and each ends with its own functional test. `ai/rules/git-safety.md` "Disjoint systems get separate commits" applies at commit time even though this is one spec |
| R-7 | `ze chaos --mrt-file -` disables strftime rotation silently (`internal/chaos/orchestrator/run.go:411`) | Rotation stops with no message | Phase 6 either rejects `-` for rotating writers with a clear error, or documents it. Silence is not an option |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze config show -` with a config on stdin | → | new leaf helper → editor content constructor → `showConfig` | `test/ui/dash-stdio.ci` config-show step |
| `ze analyze show -` with MRT on stdin | → | `openReader` `-` branch → `ReadFrom` (`internal/mrt/reader.go:50`) | `test/ui/dash-stdio.ci` analyze step |
| `ze config set - <path> <val>` with a config on stdin | → | editor content constructor → stdout sink | `test/ui/dash-stdio.ci` set-stdout step |
| `ze config migrate -o -` | → | write helper → stdout | `test/ui/dash-stdio.ci` write step |
| `ze config import a.conf - -` (stdin claimed twice) | → | stdin-once guard | `test/ui/dash-stdio.ci` double-dash step |
| A new command reading a user-supplied path raw | → | Phase 7 gate | `TestDashStdioGate` (fixture-driven) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze config show -` with a valid config on stdin | Exit 0; output identical to `ze config show <same-file-on-disk>` |
| AC-2 | `ze config show - bgp peer edge1` (path args after `-`) | Exit 0; subtree output identical to the on-disk equivalent. `-` does not disturb positional parsing |
| AC-3 | Every read command in the inventory marked **no**, invoked with `-` | Exit code and output identical to the same command given an equivalent file on disk |
| AC-4 | Every read command already supporting `-` (the 6 helper callers, the 3 ad-hoc ones) | Behavior unchanged after migration to the shared helper |
| AC-5 | `ze analyze show -` with an uncompressed MRT on stdin | Exit 0; record output identical to the file-based invocation |
| AC-6 | `ze analyze show -` with a **gzipped** MRT on stdin | Either decodes correctly via magic-byte sniffing, or fails with an explicit "compressed stdin not supported" error. **Never silently misreads** (R-4) |
| AC-7 | `ze analyze aspath -` (the duplicate reader at `internal/analyze/mrt.go:83`) | Works. Fixing `openReader` alone does not satisfy this |
| AC-8 | `ze config migrate -o -`, `ze perf run --output -`, and each write command with `-` | Output written to stdout; no file created |
| AC-9 | `ze config import a.conf - -` | Non-zero exit; error names the double-stdin conflict. **Not** a silent empty second read |
| AC-10 | `ze config set - <path> <val>` with a config on stdin | Exit 0; modified config emitted to **stdout**; no file written |
| AC-11 | `ze config rollback 2 -` (file is `fs.Arg(1)`, not 0) | Reads stdin as the config; emits to stdout. Proves the non-zero arg index is handled |
| AC-12 | `ze config edit -` | Per the A-5 decision: either works or is **explicitly rejected** with a clear error. Never a silent misbehavior |
| AC-13 | Every migrated command invoked with a **real path** | Byte-identical behavior to before this spec. The migration is invisible to non-`-` users |
| AC-14 | `ze -` (the existing precedent) | Still reads a config from stdin, unchanged |
| AC-15 | `ze config cat <key>`, `ze data cat <key>` with `-` | Unchanged: `-` is treated as a blob key, not stdin. Keys are not paths |
| AC-16 | The Phase 7 gate run against a fixture reproducing the pre-migration tree | Flags all ~34 raw-path sites. Run against the migrated tree, flags zero |
| AC-17 | `make ze-verify` after all phases | Passes, including the new gate |
| AC-18 | `grep` for the superseded normalisers | `internal/component/config/cli/main.go:115` `loadConfigData` is gone; the 3 ad-hoc `== "-"` branches are gone; `internal/component/doctor/doctor.go:179` is renamed |
| AC-19 | A file literally named `-` | Documented escape `./-` works (A-8) |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Inspects a generated config without a temp file: `generate \| ze config show -` | shell → `showConfig` → helper → editor content constructor | `test/ui/dash-stdio.ci` config-show step (AC-1) |
| 2 | Analyses an MRT streamed from a remote host: `ssh host cat rib.mrt \| ze analyze show -` | shell → `openReader` `-` branch → `ReadFrom` | `test/ui/dash-stdio.ci` analyze step (AC-5) |
| 3 | Builds a config pipeline: `ze config show - \| ze config set - bgp/asn 65001 \| ze config validate -` | each stage: stdin → helper → editor → stdout sink | `test/ui/dash-stdio.ci` pipeline step (AC-10) |
| 4 | Migrates an ExaBGP config from a pipe: `curl ... \| ze exabgp migrate -` | shell → `internal/plugins/exabgp/main.go:249` → helper | `test/ui/dash-stdio.ci` exabgp step (AC-3) |
| 5 | Emits a migrated config to stdout: `ze config migrate -o - old.conf` | write helper → stdout | `test/ui/dash-stdio.ci` write step (AC-8) |
| 6 | Mistakenly claims stdin twice: `ze config import a.conf - -` | stdin-once guard → error | `test/ui/dash-stdio.ci` double-dash step (AC-9) |
| 7 | A contributor adds a command reading a raw user path | `make ze-verify` → Phase 7 gate → fail | `TestDashStdioGate` (AC-16) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestReadPathDash` | `internal/core/<helper>/<helper>_test.go` | `-` reads stdin; a real path reads the file; a missing file errors | |
| `TestWritePathDash` | `internal/core/<helper>/<helper>_test.go` | `-` writes stdout; a real path writes the file | |
| `TestStdinClaimedOnce` | `internal/core/<helper>/<helper>_test.go` | Second claim returns an error naming the conflict; fail-closed, never an empty read (R-1) | |
| `TestOpenReaderDash` | `internal/mrt/reader_test.go` | `-` returns a reader over stdin; the `http://` branch (`:56`) still works; real paths unchanged | |
| `TestOpenReaderMagicSniff` | `internal/mrt/reader_test.go` | gzip (`1f 8b`) and bzip2 (`BZh`) magic detected on a stream; raw passes through; **extension sniffing for real paths unchanged** (A-3) | |
| `TestEditorFromContent` | `internal/component/cli/editor_test.go` | Content constructor yields a tree identical to the path constructor's for the same bytes (A-4) | |
| `TestEditorStdoutSink` | `internal/component/cli/editor_test.go` | Save with a `-` origin writes stdout, not a file; a real path still writes the file (R-3) | |
| `TestShowConfigDash` | `internal/component/config/cli/cmd_show_test.go` | `showConfig` with `-` matches the on-disk result (AC-1, AC-2) | |
| `TestDoctorLoadRenamed` | `internal/component/doctor/doctor_test.go` | The renamed function supports `-`; the collision is gone (AC-18) | |
| `TestDashStdioGate` | `scripts/checks/cli_dash_stdio_test.go` | **Fixture-driven, both directions:** pre-migration fixture flags all raw-path sites; migrated fixture flags zero; a fresh raw `os.ReadFile(fs.Arg(0))` flags (R-2) | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| stdin claims per invocation | 0 - 1 | 1 | N/A | 2 → error (AC-9) |
| `ze config rollback <N> <file>` | existing N validation, unchanged | N/A | N/A | N/A |

<!-- No new numeric input is introduced. The one new boundary is the stdin claim count,
     which is a fail-closed guard, not a numeric range. -->

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `dash-stdio` | `test/ui/dash-stdio.ci` | `config show -`, `config set -` → stdout, the 3-stage pipeline, `analyze show -` (raw + gzipped), `exabgp migrate -`, `migrate -o -`, double-`-` rejection, `config edit -` per A-5, and `ze -` still working | |

### Interop Tests (MANDATORY for protocol features)
- N/A - no wire protocol behavior. The MRT changes alter where bytes are *read from*, not how they are parsed; existing MRT decode tests still cover parsing.

### Future (if deferring any tests)
- None. Every AC has a test in this plan.

## Files to Modify

Grouped by phase. Every path below has a producer cited in the inventory.

- **Helper consumers (config):** `internal/component/config/cli/main.go` (delete `loadConfigData`), `cmd_show.go`, `cmd_completion.go`, `cmd_graph.go`, `cmd_dump.go`, `cmd_fmt.go`, `cmd_migrate.go`, `cmd_validate.go`, `cmd_fix.go`, `cmd_diff.go`, `cmd_import.go`, `cmd_edit.go`, `cmd_set.go`, `cmd_deactivate.go`, `cmd_history.go`, `cmd_rollback.go`
- **Editor:** `internal/component/cli/editor.go` (content constructor + stdout sink)
- **MRT / analyze:** `internal/mrt/reader.go` (`openReader` `-` + magic sniff; fix the misleading `:59-60` comments), `internal/mrt/writer.go`, `internal/analyze/mrt.go` (`processMRTFile`), `internal/analyze/convert.go`, `internal/analyze/filter.go`, `internal/analyze/record_bmp.go`, `internal/analyze/communities.go`, `internal/analyze/aspath.go`, `internal/analyze/statistics.go`
- **Other subsystems:** `internal/component/config/storage/cli/main.go`, `internal/plugins/exabgp/main.go`, `internal/perf/cli/cmd_report.go`, `internal/perf/cli/cmd_track.go`, `internal/perf/cli/cmd_run.go`, `internal/component/support/support.go`, `internal/component/doctor/doctor.go` (rename + migrate), `internal/appliance/config.go`, `internal/appliance/cmd_init.go`, `internal/appliance/cmd_cert.go`, `internal/appliance/cmd_import.go`, `internal/test/peer/expect.go`, `internal/test/cli/cmd_engine_steps.go`, `internal/component/bgp/config/loader.go`, `internal/chaos/orchestrator/cli.go`, `internal/chaos/orchestrator/run.go`
- **Possibly delete:** `internal/component/hub/config.go` `LoadHubConfig` (A-6, dead-code check)
- **Gate wiring:** `scripts/status/verify_run.go`, the `mk/` target file for the new check
- **Rules/docs:** `ai/rules/cli-patterns.md` (point `:43` at the helper), `ai/patterns/cli-command.md` (`:314` says "use `os.Stdin`" — must now say "use the helper", or the pattern contradicts the gate), `ai/rules/hook-mapping.md` (register the gate), `docs/guide/command-reference.md`, `ai/INDEX.md` / `ai/rules/INDEX.md`

### Files NOT modified (deliberate)

| File | Why |
|------|-----|
| `internal/component/plugin/cli/cli.go:167,177` `resolveHexInput` | `-` there means a hex **value** from stdin, not a filename. Different concept sharing a spelling |
| `internal/component/config/cli/cmd_ls.go:84`, `internal/component/config/storage/cli/main.go:274` | Blob **keys**, not paths (AC-15) |
| `internal/analyze/download.go:152`, `internal/appliance/cmd_export.go:127`, `internal/appliance/cmd_push.go:140` | Paths are **derived**, never operator-supplied (R-5) |
| `internal/component/config/archive/cmd/archive.go:51` | Daemon-startup internal, not a CLI arg |
| `internal/component/config/storage/cli/cmd_integrity.go:116-118` | `ze data encode -` takes a literal string; already idiomatic |

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | [ ] No - no config surface | - |
| YANG validation constraints | [ ] No | - |
| YANG custom validators | [ ] No | - |
| CLI commands/flags | [ ] Yes - ~34 commands change accepted input | per Files to Modify |
| CLI grammar (action before identifier) | [ ] No - no token paths change; `-` is a value, not a keyword | `ai/rules/cli-grammar.md` |
| Editor autocomplete | [ ] No - `-` is a free-form value; completion offers paths | - |
| Functional test for new RPC/API | [ ] Yes | `test/ui/dash-stdio.ci` |
| Pipe completeness | [ ] No - `-` feeds a command's file input, unrelated to the pipe operator set | `ai/rules/pipe-completeness.md` |
| Env var registration | [ ] No | - |
| Doctor check for runtime dependencies | [ ] No - no new dependency. (`ze show doctor` is *touched* as a consumer, but adds no check) | `ai/rules/doctor-checks.md` |
| Prometheus counters/metrics | [ ] No - no new observable state | - |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] Yes - `-` across ~34 commands | `docs/features.md` |
| 2 | Config syntax changed? | [ ] No | - |
| 3 | CLI command added/changed? | [ ] Yes - input surface of ~34 commands | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | [ ] No | - |
| 5 | Plugin added/changed? | [ ] Yes - `ze exabgp migrate` | `docs/guide/plugins.md` (verify by grep) |
| 6 | Has a user guide page? | [ ] Yes | `docs/guide/config-editor.md` (documents `ze config` commands) |
| 7 | Wire format changed? | [ ] No | - |
| 8 | Plugin SDK/protocol changed? | [ ] No | - |
| 9 | RFC behavior implemented, changed, or newly proven? | [ ] No | - |
| 10 | Test infrastructure changed? | [ ] Yes - new `.ci` + new gate | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | [ ] No | - |
| 12 | Internal architecture changed? | [ ] Yes - a new core leaf package + a new gate | `docs/architecture/core-design.md`, `ai/rules/hook-mapping.md` |
| 13 | Route metadata keys added/changed? | [ ] No | - |
| 14 | Prometheus counters added/changed? | [ ] No | - |
| 15 | Registered plugin/command/inventory changed? | [ ] No - no command renamed or added | - |
| 16 | Any changed source file referenced by doc source anchors? | [ ] Yes - ~30 source files change | Grep `docs/` for `source: <each changed file>` |
| 17 | Existing docs show CLI examples for this area? | [ ] Yes | Verify every `ze config` / `ze analyze` example still matches |

## Files to Create
- `internal/core/<helper>/<helper>.go` + `doc.go` + `<helper>_test.go` - the leaf package. Name chosen in Phase 1 (see Key Design Decisions); `cliio` is the working proposal.
- `scripts/checks/cli_dash_stdio.go` + `scripts/checks/cli_dash_stdio_test.go` - the gate, modeled on `direct_fs_persistence.go`.
- `test/ui/dash-stdio.ci` - the functional suite.

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, TDD Test Plan; validate A-1..A-8 by grep/read/`make ze-tier-check` before coding |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. Full verification | `make ze-lint-changed && make ze-unit-test && make ze-functional-test` |
| 6. Critical review | Critical Review Checklist below |
| 7. Fix issues | Fix every issue from critical review |
| 8. Re-verify | Re-run stage 5 |
| 9. Repeat 6-8 | Until clean |
| 10. Deliverables review | Deliverables Checklist below |
| 11. Security review | Security Review Checklist below |
| 12. Documentation review | Documentation Update Checklist above |
| 13. /ze-review gate | Review Gate section |
| 14. Present summary + close | Executive Summary Report; two-commit closure per `ai/rules/planning.md` |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.
Phases are per-subsystem so each is independently reviewable and committable (R-6).

1. **Phase: Wiring (MANDATORY FIRST)** — the leaf helper + the reported case end-to-end
   - Tests: `TestReadPathDash`, `TestWritePathDash`, `TestStdinClaimedOnce`, `TestShowConfigDash`; `test/ui/dash-stdio.ci` config-show step
   - Files: `internal/core/<helper>/`, `internal/component/config/cli/cmd_show.go`, `internal/component/cli/editor.go` (minimum viable content constructor)
   - Verify: `ze config show -` fails first, then passes (AC-1). `make ze-tier-check` green (A-1). This phase proves the whole design on the user's reported case before touching 33 other sites

2. **Phase: config/cli reads** — migrate every reader, delete the superseded normalisers
   - Tests: existing config CLI tests must stay green
   - Files: the 16 `internal/component/config/cli/*.go` readers
   - Verify: the 6 helper callers and 3 ad-hoc branches behave identically (AC-4); `loadConfigData` and all three ad-hoc `== "-"` branches are **deleted**, not left beside the helper (`ai/rules/no-layering.md`); `cmd_import` multi-arg claims stdin at most once (AC-9)

3. **Phase: editor content + stdout sink** — the hardest phase (A-4, A-5)
   - Tests: `TestEditorFromContent`, `TestEditorStdoutSink`; `.ci` set-stdout, rollback, pipeline steps
   - Files: `internal/component/cli/editor.go`, `cmd_set.go`, `cmd_deactivate.go`, `cmd_history.go`, `cmd_rollback.go`, `cmd_edit.go`
   - Verify: `ze config set - <path> <val>` emits to stdout (AC-10); `rollback` handles the arg-1 index (AC-11); a real path still writes back byte-identically (AC-13, R-3); `ze config edit -` resolves per A-5 with an explicit AC either way (AC-12)
   - STOP condition: if A-4 breaks (the editor's identity model cannot take a synthetic name), present to the user before reworking it

4. **Phase: MRT / analyze** — the choke point and its duplicate
   - Tests: `TestOpenReaderDash`, `TestOpenReaderMagicSniff`; `.ci` analyze raw + gzipped steps
   - Files: `internal/mrt/reader.go`, `internal/mrt/writer.go`, `internal/analyze/mrt.go`, and the analyze subcommands
   - Verify: `openReader` `-` covers all 10 subcommands (AC-5); `processMRTFile` covers the other 6 (AC-7) — **fixing one does not fix the other**; gzipped stdin decodes or errors explicitly, never misreads (AC-6, R-4); real-path extension sniffing unchanged (A-3); the misleading "not user input" comments at `:59-60` are corrected

5. **Phase: remaining subsystems** — data, exabgp, perf, support, doctor, appliance, test, bgp loader
   - Tests: `TestDoctorLoadRenamed`; `.ci` exabgp step
   - Files: per Files to Modify
   - Verify: every inventory row marked **no** now works with `-` (AC-3); the doctor `loadConfigData` collision is gone (AC-18); `LoadHubConfig` resolved per A-6 and the dead-code check (ASK before removing)

6. **Phase: writes** — `-` means stdout
   - Tests: `.ci` write step
   - Files: `cmd_migrate.go`, `internal/analyze/convert.go`, `filter.go`, `record_bmp.go`, `internal/perf/cli/cmd_run.go`, `internal/chaos/orchestrator/cli.go`, `run.go`
   - Verify: each write command with `-` emits to stdout and creates no file (AC-8); the existing empty-string default (flag omitted → stdout) still works — `-` is additive, not a replacement; `ze chaos --mrt-file -` either rejects `-` or documents the loss of rotation (R-7)

7. **Phase: THE GATE (load-bearing)** — enforce the rule that has been unenforced all along
   - Tests: `TestDashStdioGate`
   - Files: `scripts/checks/cli_dash_stdio.go`, its test, `scripts/status/verify_run.go`, the `mk/` target, `ai/rules/hook-mapping.md`
   - Verify: **the gate must first be demonstrated flagging the pre-migration sites on a fixture** (R-2, A-7). A gate that only passes on the fixed tree proves nothing. Then: migrated tree flags zero; a fresh raw `os.ReadFile(fs.Arg(0))` flags; allowlist entries each carry a stated reason (derived paths, blob keys, hex values); `make ze-verify` green

8. **Phase: rules and documentation**
   - Files: `ai/rules/cli-patterns.md:43`, `ai/patterns/cli-command.md:314`, `docs/`, indexes
   - Verify: `ai/patterns/cli-command.md:314` currently says "use `os.Stdin` when filename is `-`" — following that literally now FAILS the Phase 7 gate. It must point at the helper, or the repo's own pattern contradicts its own gate; `make ze-doc-test`; `scripts/dev/check_doc_links.py --design-only` clean

9. **Full verification** → `make ze-verify`
10. **Complete spec** → Fill audit tables, write learned summary to `plan/learned/NNN-cli-dash-stdio.md`. TWO commits minimum: commit A (code + tests + docs + spec + learned summary), commit B (`git rm` the spec). Disjoint subsystems may warrant more than one implementation commit per `ai/rules/git-safety.md`

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-19 has implementation or test evidence with file:line |
| Feature completeness | Every inventory row is either migrated or explicitly listed as excluded (not a user filename path) with a reason. Count the rows; count the migrations |
| Gate honesty | `TestDashStdioGate` fails on the pre-migration fixture. A gate that cannot catch the bug it was written for is not a gate (R-2) |
| Correctness | Every migrated command with a **real path** behaves byte-identically to before (AC-13). The migration is invisible to non-`-` users |
| Fail-closed | The stdin-once guard errors on the second claim. An empty read is never a valid-looking answer (`ai/rules/fail-closed-guards.md`, R-1) |
| No silent misreads | Compressed stdin decodes or errors. Never garbage (R-4) |
| Rule: no-layering | `loadConfigData` and all three ad-hoc `-` branches are DELETED, not left beside the helper |
| Naming | One helper, one name. The doctor `loadConfigData` collision is resolved |
| Data flow | The helper composes AFTER `ResolveConfigPath`, never before (`internal/component/config/environment.go:271`) |
| Module tiers | The helper is a leaf under `internal/core/`; `make ze-tier-check` green (A-1) |
| Rule consistency | `ai/patterns/cli-command.md:314` no longer instructs a practice the gate rejects |
| Rule: no-fabrication | Every claim in the learned summary cites the producing function |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| The leaf helper exists and is tier-legal | `ls internal/core/<helper>/`; `make ze-tier-check` |
| `ze config show -` works (the reported case) | `test/ui/dash-stdio.ci` config-show step passes |
| Superseded normalisers are gone | `grep -n 'func loadConfigData' internal/component/config/cli/main.go` returns nothing; `grep -rn '== "-"' internal/component/config/cli/` returns nothing |
| The gate catches the class | `TestDashStdioGate` flags the pre-migration fixture |
| The gate is wired | `grep -n 'dash' scripts/status/verify_run.go`; `make ze-verify` runs it |
| Every inventory row is accounted for | Re-read the inventory; each row is migrated or listed in "Files NOT modified" with a reason |
| The pattern doc no longer contradicts the gate | `grep -n 'os.Stdin' ai/patterns/cli-command.md` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Unbounded stdin | `io.ReadAll(os.Stdin)` on a config path is unbounded. The offline pipe carrier already caps stdin at 256 MB — `const maxStdin = 256 << 20` + `io.LimitReader` in `runFormat` (`cmd/ze/ze_core_format.go:42-51`), **but see Concurrent Work: that function is being renamed to `runPipe` in `cmd/ze/ze_core_pipe.go`; if the file is absent, grep for `maxStdin`**. Decide whether the helper needs an equivalent cap and apply it consistently. An unbounded read is a trivial memory exhaustion vector |
| Streaming preserved | MRT must NOT be forced through `io.ReadAll`: `ze analyze replay` handles multi-GB files. The reader API must stay streaming (A-2) |
| Path handling | The helper must not widen path access: `-` is the ONLY special token. No `/dev/*` interpretation, no shell expansion |
| gosec annotations | `internal/mrt/reader.go:59-60` claims CLI args are "not user input". Correct the comments; do not carry a false justification into the new code |
| Error leakage | Errors report the path token and the OS error, never stdin content |
| Privilege | `-` must not let a command read something it otherwise could not. Stdin is the caller's own fd, so this is inherently sound — confirm no command re-opens `/dev/stdin` by path (which would be a different fd with different permissions) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior → RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural → DESIGN phase |
| A-1 broken (tier violation) | STOP. The helper's home is wrong; re-read `ai/rules/module-tiers.md` and present |
| A-4 broken (editor identity model) | STOP. Phase 3 becomes an editor rework; present before proceeding |
| A-3 broken (magic sniffing infeasible) | Reject compressed stdin with an explicit error (AC-6's second branch). Never misread |
| A-7 broken (gate cannot separate user-supplied from derived) | STOP and present. Do NOT ship a gate that flags derived paths — it would be disabled within a week |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| The `.ci` functional harness can pipe real stdin to a `ze <cmd> -` step (spec Wiring Test assumed this) | The runner (`internal/test/runner/runner_exec.go:565-634`) materializes any `stdin=` payload for a `ze` command into a TEMP FILE and rewrites the first `-` arg to that path; it never pipes the process's stdin. 700+ existing steps depend on this. | Phase 1 `.ci` "config show -" passed by reading the temp file; Phase 2 `validate -` printed `valid: <tempfile>` not `valid: -` | The `.ci` cannot exercise the literal stdin fd, double-`-`, or MRT magic-sniff-on-stdin. All true-stdin behavior is proven by UNIT tests via `cliio.SwapStreams`. The runner was NOT modified (sibling-owned, huge blast radius). `.ci` steps prove `-` is accepted + output correct (== real-path AC-13). |
| `ze config rollback N -` can "read stdin, emit stdout" (AC-11 wording) | Rollback restores from the `rollback/` revision history keyed by the file path; a piped config has no history. `history -` has the same problem. | Phase 3 tracing of `ed.ListBackups()` on identity `-` | Deviation: `rollback N -` and `history -` REJECT `-` with a clear error (user decision 2026-07-17), instead of AC-11's "emit to stdout". set/deactivate/activate still emit to stdout. |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Gate the runner's `-`→tempfile rewrite on `zeDaemonConfigArgIndex` so offline `ze <cmd> -` pipes real stdin | Breaks 700+ existing `.ci` steps (505 `ze -`, 217 `ze config validate -`, `ze bgp decode -`) that rely on the file materialization; the runner is being edited by a concurrent sibling session | Prove true-stdin behavior with unit tests (`cliio.SwapStreams`); leave the runner untouched |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights
<!-- LIVE — write IMMEDIATELY when you learn something -->

- The rule was stated twice (`ai/rules/cli-patterns.md:43`, `ai/patterns/cli-command.md:314`) and violated ~34 times. Stating a convention in two places is not enforcement; it is documentation of an intention. The helper existed too (`internal/component/config/cli/main.go:115`), but it was **package-private**, so every subsystem outside `config/cli` had to either re-implement it or skip it. Most skipped it. Accessibility of the shared thing determines whether sharing happens.
- `internal/mrt/reader.go:59-60` is the drift in miniature: `cleaned := filename // caller-controlled path; no user input` and `//nolint:gosec // path is from CLI args, not user input`. A CLI argument IS user input. The comment asserts the opposite to justify silencing a linter, and that assertion is why nobody asked whether the path could be `-`.
- The pattern doc actively works against the fix: `ai/patterns/cli-command.md:314` says "use `os.Stdin` when filename is `-`", which after Phase 7 is precisely what the gate rejects. A pattern that predates its own enforcement mechanism becomes a trap.

## Core Insight

A convention is only real if it is both **reachable** and **checked**. This one was
neither: the helper that implemented it was package-private, and no gate looked for it.
The result was not that people disagreed with the rule; it is that ~24 commands never
encountered it. The fix is therefore not "add `-` to `ze config show`" but "make the
helper reachable from every tier and make its absence fail the build" — the lasting
value is in the gate, not the migrations.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| One leaf package under `internal/core/` | Export the existing `config/cli.loadConfigData`; put it in `internal/core/paths` | `config/cli` is a component: `internal/analyze`, `internal/perf`, and `internal/appliance` importing it would invert the tier direction and fail `make ze-tier-check` (`ai/rules/module-tiers.md:17`). `internal/core/paths` is specifically "resolves the ze configuration directory from the running binary's location" (`doc.go`), a different concern |
| Resolve `-` to `os.Stdin`/`os.Stdout`, NOT to the literal path `/dev/stdin` | Rewrite `-` → `/dev/stdin` so path-taking APIs work unchanged | `/dev/stdin` appears nowhere in the tree today. It is a different fd with its own permissions, is not portable, and re-opening it by path breaks for pipes in ways `os.Stdin` does not. The user's phrasing ("`-` as /dev/stdin alias") describes the *semantics*; `os.Stdin` is the correct mechanism for them |
| Streaming reader API, not just `ReadPath` | `ReadPath` returning `[]byte` only | `ze analyze replay` streams multi-GB MRT through `ReadFrom` (`internal/mrt/reader.go:50`). A bytes-only helper would force `io.ReadAll` and blow memory. The helper needs both a bytes form (configs) and a reader form (MRT) |
| Magic-byte compression sniffing for `-` | Extension sniffing (impossible: `-` has no extension); assume raw; reject compressed stdin | `openReader` sniffs by suffix (`internal/mrt/reader.go:64-79`) and `-` falls to `default:` (`:77`) = raw, so a gzipped pipe is silently misread. Magic bytes (gzip `1f 8b`, bzip2 `BZh`) are cheap via a buffered peek. If that proves infeasible, reject explicitly — silence is the one unacceptable outcome (R-4) |
| Fail closed on a second `-` | Let the second read return empty | Stdin is consumable once. An empty second read is a zero value that looks like a valid empty file, which `ai/rules/fail-closed-guards.md` names as the exact failure to prevent |
| `-` is additive to the existing empty-string stdout default | Migrate the empty default to `-` only | For `ze config migrate -o`, `ze perf run --output`, `ze chaos --config-out`, an omitted flag already means stdout. That is a *default*, not a competing sentinel: flag omitted → stdout, flag `= -` → stdout explicitly. Both coexist without ambiguity, and migrating the default would break every existing caller for no gain |
| Blob keys stay excluded (not a user filename path) | Treat `config cat -` / `data cat -` as stdin | A key is not a filesystem path (`internal/component/config/cli/cmd_ls.go:84` reads `store.ReadFile(args[0])` against the blob store). `-` there would mean "the blob named `-`". Conflating the two namespaces is how `-` stops being predictable |
| Editor gains a content constructor + a stdout sink | Make `store.ReadFile("-")` return stdin inside the storage layer | The storage layer is a general abstraction addressing both files and blob keys; teaching it a CLI convention would make `-` magic in the daemon too, and `store.ReadFile` is called from non-CLI paths (`internal/component/config/archive/cmd/archive.go:51`). Keep the CLI convention at the CLI edge |
| One spec, phased, per-subsystem | Umbrella + children | User decision (2026-07-17). Phases are per-subsystem so each is independently reviewable, and `ai/rules/git-safety.md` still permits separate commits for disjoint systems (R-6) |

## Known Limitations

- A file literally named `-` becomes unreachable without the `./-` escape (A-8). This is the universal cost of the convention and is already paid by `ze -` today (`cmd/ze/ze_core_dispatch.go:404`). It must be documented, not silently accepted.
- `ze config edit -` may end up **rejected** rather than supported (A-5). An interactive editor whose input arrived on a consumed stdin, and whose path is forwarded as argv to a spawned daemon (`cmd_edit.go:85`, `:138`), has no coherent behavior. If it is rejected, that is a deliberate scope boundary and AC-12 records it.
- `ze chaos --mrt-file -` loses strftime rotation by definition (`internal/chaos/orchestrator/run.go:411`). Phase 6 must choose rejection or documentation; either is acceptable, silence is not (R-7).
- The gate governs **file-path** I/O. Commands taking non-path values from stdin (`ze bgp encode`, which reads stdin unconditionally at `internal/component/bgp/cli/encode.go:109`; `resolveHexInput` at `internal/component/plugin/cli/cli.go:177`) are a separate convention and remain ungated by this spec.
- The gate's user-supplied-vs-derived distinction rests on an allowlist (A-7), inheriting `direct_fs_persistence.go`'s tradeoff: a new derived-path writer must be allowlisted by hand or it produces a false positive. That is the accepted cost of not doing dataflow analysis.

## RFC Documentation

N/A - no protocol behavior.

## Implementation Summary

### What Was Implemented
- `internal/core/cliio` leaf: `ReadFile`/`OpenReader`/`Create`/`WriteFile`, `IsStdin`, `ErrStdinClaimed` (fail-closed stdin-once), `SwapStreams` (test seam), 256 MB `MaxStdinBytes` cap on `ReadFile`.
- Editor: `NewEditorFromContent` (Phase 1) + `stdoutSink`/`SetStdoutSink` routing `Save()` to stdout (Phase 3).
- ~40 call sites migrated across config/cli, editor commands, mrt/analyze (+ magic-byte compression sniff via `mrt.SniffDecompress`), data, exabgp, perf, support, doctor (rename `loadConfigData`→`loadDoctorConfig`), appliance, test, bgp loader, hub, tacacs, plugin-test, chaos.
- Writes: mrt.Writer `-`→stdout (no rotation), convert/migrate/perf-run/fmt-`-w`/chaos routed through cliio.
- THE GATE: `scripts/checks/cli_dash_stdio.go` (per-package CLI-taint dataflow with param fixpoint, `//cliio:allow` marker); wired into `make ze-dash-stdio-check` + ze-verify. **It caught 8 sites the manual inventory missed.**

### Bugs Found/Fixed
- The Phase 7 gate flagged 8 raw-path sites absent from the inventory (tacacs, plugin-test, chaos replay/shrink/diff/writeConfig, cmd_edit O_EXCL). Migrated 7; allowlisted cmd_edit's O_EXCL atomic-create (`-` rejected upstream) via the inline marker.
- `internal/mrt/reader.go:59-60` false "not user input" gosec comments corrected.

### Documentation Updates
- Rules: `ai/rules/cli-patterns.md` (`-` bullet → helper + gate), `ai/patterns/cli-command.md` (Stdin row → helper, not `os.Stdin`; resolves the doc-contradicts-gate trap).
- User docs: `docs/features.md` (feature row), `docs/guide/command-reference.md` (Conventions section), `docs/guide/config-editor.md` (`-` for editor cmds + rejections), `docs/config-migration.md` (`-w -`/`-o -` stdout).
- Indexes regenerated: `ai/DOCS-TO-CODE.md` (+editor_stdin.go), `ai/rules/INDEX.md` (synced committed state). Source anchors validated (`make ze-doc-test`: all references valid).
- Source-aware NO: `docs/functional-tests.md`/`ai/rules/hook-mapping.md` — the model gate `direct_fs_persistence` is NOT documented in either (0 hits); verify-gates live in the rule (`cli-patterns.md`) + Makefile, which is where mine is. `docs/architecture/core-design.md` is BGP-architecture-only (no core-helper section); cliio's `doc.go` is its design home (mirrors stringsx). `docs/guide/plugins.md` does not document `ze exabgp migrate` (0 hits); it lives in command-reference/features.

### Deviations from Plan
- **AC-11 (rollback `-`):** user decided 2026-07-17 to REJECT `rollback N -` and `history -` (no on-disk revision history for a pipe) rather than "read stdin, emit stdout". `edit -` also rejected (A-5). set/deactivate/activate still emit to stdout.
- **`.ci` cannot test the literal stdin fd** (harness rewrites `-`→tempfile; see Mistake Log). True-stdin behavior (routing, magic-sniff, double-`-`) proven by unit tests via `cliio.SwapStreams`. `.ci` proves command acceptance + output (== real-path AC-13).
- Pre-existing tree reds (NOT this spec): `RFC-REQUIREMENTS.md` staleness and the config/schema authz/listener test failures come from the concurrent `spec-cli-root-namespace-grammar` session; disjoint from this diff (no RFC/schema code touched).

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestShowConfigDash` (cmd_show_test.go) + `test/ui/dash-stdio.ci` seq=1 | `show -` == on-disk whole tree |
| AC-2 | Done | `TestShowConfigDash` (path after `-`) + `.ci` seq=2 | positional parsing undisturbed |
| AC-3 | Done | `.ci` seq=1-5 + unit tests per subsystem; gate live=0 confirms all migrated | every inventory `no` row routes through cliio |
| AC-4 | Done | config/cli suite green (6 helper callers + 3 ad-hoc migrated); `.ci` validate/dump/fmt | behavior unchanged after migration |
| AC-5 | Done | `TestOpenReaderDash` (raw stdin) + `TestProcessMRTFileDash` | both MRT readers |
| AC-6 | Done | `TestOpenReaderMagicSniff` (gzip+bzip2 magic), `TestProcessMRTFileDash` (gzip) | never silently misread (R-4) |
| AC-7 | Done | `TestProcessMRTFileDash` (the duplicate reader) | fixing openReader alone insufficient — covered |
| AC-8 | Done | `TestMigrateOutputDash`, `TestWriterStdout` (mrt), `Create`/`WriteFile` unit tests | write commands → stdout, no file |
| AC-9 | Done | `TestStdinClaimedOnce` (cliio), `TestImportDoubleStdin` (non-zero exit) | second `-` fails closed |
| AC-10 | Done | `TestSetStdoutSink`, `TestConfigPipelineStdin`, `TestEditorStdoutSink` | set - → stdout pipeline stage |
| AC-11 | **Deviated** | `TestRollbackRejectsStdin` (asserts message) | user decision: `rollback N -` REJECTED (no revision history), not "reads stdin, emits stdout" |
| AC-12 | Done | `TestEditRejectsStdin` (asserts message) | `edit -` explicitly rejected (A-5) |
| AC-13 | Done | Correctness reviewer verified byte-identical real-path behavior; existing suites green | migration invisible to non-`-` users |
| AC-14 | Done | `ze -` path untouched (`cmd/ze/ze_core_dispatch.go:404` not modified) | precedent preserved |
| AC-15 | Done | blob-key commands (`config cat`/`data cat`) untouched; gate treats `store.ReadFile` as excluded (not a user filename path) | keys are not paths |
| AC-16 | Done | `TestDashStdioGate` (--selftest); gate CAUGHT 8 missed sites on the live tree | flags violation shapes, 0 on migrated |
| AC-17 | Partial | `make ze-dash-stdio-check` green; full `ze-verify` blocked only by pre-existing sibling reds (RFC-index, schema tests) | the new gate passes |
| AC-18 | Done | `TestDoctorLoadRenamed`; `grep 'func loadConfigData' main.go` = none; 3 ad-hoc branches gone | collision resolved |
| AC-19 | Done | `TestIsStdin` (`./-` not stdin); documented in command-reference.md Conventions | `./-` escape |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|-----------|-------|
| TestReadPathDash / TestWritePathDash / TestStdinClaimedOnce | Done | `internal/core/cliio/cliio_test.go` | + TestCreateDash, TestOpenReaderDash, TestIsStdin |
| TestOpenReaderDash / TestOpenReaderMagicSniff | Done | `internal/mrt/reader_test.go` | embedded real bzip2 fixture |
| TestWriterStdout | Done | `internal/mrt/writer_test.go` | write side |
| TestProcessMRTFileDash | Done | `internal/analyze/mrt_stdin_test.go` | the duplicate reader (AC-7) |
| TestEditorFromContent / TestEditorStdoutSink | Done | `internal/component/cli/editor_test.go` | |
| TestShowConfigDash | Done | `internal/component/config/cli/cmd_show_test.go` | |
| TestImportSingleStdin / TestImportDoubleStdin | Done | `cmd_import_test.go` | |
| TestSet/Deactivate/Pipeline + Reject tests | Done | `cmd_stdin_test.go` | reject tests assert the message (non-vacuous) |
| TestMigrateOutputDash | Done | `cmd_stdin_test.go` | AC-8 write side |
| TestDoctorLoadRenamed | Done | `doctor_stdin_test.go` | AC-18 |
| TestDashStdioGate / TestCliDashStdioLive | Done | `scripts/checks/cli_dash_stdio_test.go` | selftest fires (R-2) + live clean |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Every filename-accepting command accepts `-` | functional test | `test/ui/dash-stdio.ci` covers each subsystem; AC-3 enumerates the inventory |
| The reported case works | functional test | `ze config show -` step (AC-1) |
| `-` means stdout when writing | functional test | write step (AC-8) |
| The convention cannot drift again | gate unit test | `TestDashStdioGate` flags all pre-migration sites on a fixture (AC-16) |
| No silent misbehavior was introduced | functional test | gzipped stdin (AC-6) and double-`-` (AC-9) both fail loudly |
| Non-`-` users are unaffected | functional test | AC-13: every migrated command with a real path is byte-identical |

## Review Gate

### Run 1 (initial) — 3 independent general-purpose reviewer subagents over the diff

**Reviewer B (security / fail-closed guards, agent a459658a):** 0 BLOCKER, 0 ISSUE.
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| B1 | NOTE | `/dev/stdin`/`/dev/zero` bypass cap+once-guard (only `-` is special) | `internal/core/cliio/cliio.go:44` | acknowledged: already documented in the `StdinToken` const comment ("no /dev/* handling"); operator-supplied, not attacker-reachable |
| B2 | NOTE | `io.ReadAll` transient ~2x cap during final grow | `cliio.go:77` | acknowledged: O(cap) bounded, matches the pipe carrier |
| B3 | NOTE | No boundary test for the 256 MB cap | `cliio_test.go` | acknowledged: a 256 MB allocation is impractical for a unit test; logic reviewer-verified (LimitReader+1, exactly-cap allowed, cap+1 rejected) |
| B4 | NOTE | `analyze/mrt.go` real-path `.gz`/`.bz2` match is case-sensitive; `mrt/reader.go` lowercases | `internal/analyze/mrt.go:108,115` | acknowledged: PRE-EXISTING (not introduced here); A-3 required real-path extension sniffing UNCHANGED, so preserved deliberately |

**Reviewer C (gate soundness / test quality, agent a072e31e):** 1 BLOCKER, 2 ISSUE.
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| C1 | BLOCKER | `Test{Rollback,History,Edit}RejectsStdin` are VACUOUS — assert only `rc != exitOK`; the command fails for an unrelated reason (os.ReadFile("-") / "database not found") even if the guard were deleted | `internal/component/config/cli/cmd_stdin_test.go:120,129,139` | FIX: assert the stdin-specific rejection message (capture stderr) |
| C2 | ISSUE | `store.ReadFile(fs.Arg(0))` bypasses the gate (interface method, not `os.*`) — a future command could drop `-` support undetected | `scripts/checks/cli_dash_stdio.go` | FIX: document the interface-method-read boundary in the gate header |
| C3 | ISSUE | Return-value / struct-field laundering false-negative (`p := f(fs.Arg(0)); os.ReadFile(p)`) | `cli_dash_stdio.go:169-174` | acknowledged: already disclosed in the header ("taint does NOT flow through ... function returns"); accepted false-negative-only boundary; strengthen the header wording |
| C4 | NOTE | `isCLIArgExpr` matches by method NAME not type; an unrelated `.Arg()` could false-positive | `cli_dash_stdio.go:102,118` | FIX: soften the header "near-zero false positives" claim + note the heuristic |
| C5 | NOTE | Selftest doesn't exercise aliased-`os` import or two-hop funnel (both implemented + correct) | `cli_dash_stdio.go` runSelftest | FIX: add fixtures to lock the behavior |
| C-hold | (verified sound) | Gate is honest+sound within documented limits; wired in both stage lists; TestDashStdioGate proves R-2; TestStdinClaimedOnce/TestImportDoubleStdin/TestOpenReaderMagicSniff (real bzip2) non-vacuous | — | no action |

### Fixes applied (after Run 1)
- **C1 (BLOCKER):** rewrote `Test{Rollback,History,Edit}RejectsStdin` (`cmd_stdin_test.go`) to capture stderr (`captureStderr`) and assert the stdin-SPECIFIC message ("revision history" / "cannot read a config from stdin"). Now deleting a guard makes the test fail (fallback errors are "revision N not found"/"no such file"/"database not found"), not pass. All 3 pass; verified non-vacuous.
- **C2/C3/C4 (ISSUE/NOTE):** `scripts/checks/cli_dash_stdio.go` header now documents: the `store.ReadFile` interface-method boundary is excluded (not a user filename path) by design; return-value/struct-field laundering is a disclosed false-negative (lint heuristic, not soundness proof); the `isCLIArgExpr` method-name heuristic means false positives are rare-not-zero.
- **C5 (NOTE):** added selftest fixtures `fxTwoHop` (two-hop param funnel) and `fxAliasedOS` (`import fsys "os"`), each `mustFlag` — locks the fixpoint + aliased-import capabilities against regression. selftest still OK.
- **Correctness NOTE:** `fmt -w -` (`cmd_fmt.go`) now routes through `cliio.WriteFile` instead of `fmt.Print`, so it shares the one swappable stdout sink (testable consistency).
- Re-ran: gate selftest OK, gate live OK (0 findings), config/cli tests green, dash-stdio.ci PASS.

### Run 2 (re-review of the Run-1 fixes, agent a67bd02f) — CLEAN
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| — | (verified) | BLOCKER C1 genuinely RESOLVED: reviewer traced each guard's fallback and confirmed all 3 reject tests FAIL if the guard is deleted (fallback = "cannot read config file: open -" / "database not found", neither contains the asserted substring). Non-vacuous. | `cmd_stdin_test.go` | no action |
| — | (verified) | FIX 2 correct: `fxTwoHop` is genuinely two hops (needs the fixpoint's 2nd iteration); `fxAliasedOS` genuinely exercises `osImportNames` alias resolution. selftest OK. | `cli_dash_stdio.go` | no action |
| — | (verified) | FIX 3 correct: `fmt -w -`→`cliio.WriteFile` (swappable stdout, unconditional emit, no file); real-path `-w file` unchanged. | `cmd_fmt.go` | no action |
| R2-1 | NOTE | `captureStderr` latent deadlock if fn writes > ~64 KB to stderr (no concurrent reader) | `cmd_stdin_test.go` | FIXED: added a doc comment bounding it to small-stderr commands (reject msgs ~100 B) |

**Run 2 verdict: 0 BLOCKER, 0 ISSUE. The review gate has looped to zero.**

### Final status
- Independent review CLEAN: Run 1 found 1 BLOCKER (vacuous reject tests) + 2 ISSUE, all fixed; Run 2 re-review confirmed 0 BLOCKER / 0 ISSUE. `review_gate.py` artifact recorded (`tmp/review/cli-dash-stdio.md`, 60 files, verdict=clean).
- All NOTEs recorded above.

### Commit status — BLOCKED by the concurrent session (owner decision needed)
Implementation is complete and every MY-scope check is green: `dash-stdio.ci` PASS, `ze-dash-stdio-check` (the load-bearing gate) green, `ze-lint-changed` green, `ze-tier-check` green, the wiring check PASSED (after the `SwapStreams` allowlist), my discovery indexes regenerated + green, my packages' unit tests pass under proper feature tags.

`commit_helper.py create` refuses because the STRUCTURAL gate `ze-verify-wiring-docs` is red — that make target runs BOTH the (green) wiring check AND `ze-doc-test`, and `ze-doc-test` fails ONLY on `ai/RFC-REQUIREMENTS.md is stale`, caused by the concurrent `spec-cli-root-namespace-grammar` session's UNTRACKED `rfc/short/rfc792.md` (ICMP RFC — unrelated to this spec). Also red but non-structural: `ze-rfc-check` (same RFC-index), `ze-unit-test-changed` (`-race requires cgo`, environmental), `ze-functional-test` (11 suites in untouched subsystems failing on missing `/proc/net/*` — my `dash-stdio` passed).

I cannot fix the RFC-index without regenerating `RFC-REQUIREMENTS.md` from the sibling's untracked `rfc792.md` (cross-commit, forbidden by CLAUDE.md), and a structural gate is not `--unverified`-eligible (owner-only). 73-file changeset staged in `tmp/my_files.txt`; the two-commit script is ready to generate the moment the gate clears (sibling commits rfc792.md + regens the RFC index, or owner override).

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-19 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md` — no failures)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`); broken ones in Mistake Log; surviving risks copied to Executive Summary

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] RFC constraint comments added (N/A - no protocol behavior)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] Abstract when you can (2+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (N/A - no wire protocol behavior)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `ai/rules/quality.md` documented pass in spec
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-cli-dash-stdio.md`
- [ ] **Commit A:** code + tests + docs + spec (with all edits) + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-cli-dash-stdio.md` only

