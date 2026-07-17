# Spec: cli-root-namespace-grammar

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 9/9 |
| Updated | 2026-07-17 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `ai/rules/cli-grammar.md` - R9 (Compound Token vs Namespace Split) and the backward-compat policy
4. `docs/features/formatting.md` - the operator taxonomy that names the `pipe` rename
5. `cmd/ze/ze_core_dispatch.go` (`zeDispatch`, `yangVerbs`), `cmd/ze/ze_core_format.go`, `scripts/checks/cli_grammar.go`

## Task

The registry **root-command namespace is ungoverned**, and it has drifted in two
directions at once.

`ai/rules/cli-grammar.md` defines rules R1-R9 and a static gate enforces them. But the
gate reads **only the YANG command tree**: `scripts/checks/cli_grammar.go` contains no
reference to `registry.ListRoot` or `MustRegisterRootHandler` (`grep -i root
scripts/checks/cli_grammar.go` returns nothing), and `ai/rules/cli-grammar.md:286`
describes the static gate's input as "Every built-in command (YANG command tree)".
Root handlers registered via `registry.MustRegisterRootHandler` never pass through it.

The consequence is visible. The `cli-hyphen-namespace-split` migration (2026-07-13)
split every R9 violation in the YANG tree, leaving `pendingNamespaceSplit` empty
(`scripts/checks/cli_grammar.go:74`). The YANG tree got cleaned. The root namespace
never did, and it still carries both defects:

**Defect 1 - a carrier named after one of its clauses.** `runFormat`
(`cmd/ze/ze_core_format.go:22`) splices its arguments into a synthetic pipe expression
(`"_ | " + args`) and hands the whole thing to `command.ProcessPipesChecked`
(`internal/component/command/pipe.go:674`). Every operator in `knownPipeOps`
(`internal/component/command/pipe.go:52`) is reachable through it, including the filter
operators `match`, `count`, `first`, `last`. `ze format` is the **carrier for the whole
pipe language**, named after one clause of that language, so `ze format match` asks a
"format" command for a filter.

**Defect 2 - compound tokens that are really namespaces.** Four root commands hyphenate
two separate ideas, which R9 (`ai/rules/cli-grammar.md:78`) forbids: `traffic-control`,
`isis-decode`, `ospf-decode`, `update-serve`.

**Goal:** make the root namespace obey the grammar the YANG tree already obeys, and
**extend the gate to the root namespace** so it cannot drift again. The gate is the
load-bearing deliverable: renaming without it fixes today's four commands and prevents
none of tomorrow's.

### Defect 1: why `pipe`

| # | Evidence | Why it matters |
|---|----------|----------------|
| 1 | The command's own help splits `Format Operators` from `Filter Operators` (`cmd/ze/ze_core_format.go:67`, `:74`) | The command contradicts the help it prints |
| 2 | `docs/features/formatting.md:45-61` gives every operator a `Kind`: `format`, `filter`, or `display` | The project's published taxonomy says `match` is not a format |
| 3 | `format` already has two **correct** meanings: `set cli format <name>` (`internal/component/cli/model_keys.go:432` `handleSetCLIFormat`, `:669` `validCLIFormats`) and the editor pipe `\| format tree\|config` (`internal/component/cli/model_load.go:951` `ClassifyShowPipes`, `internal/component/cli/completer.go:440`) | One word, three meanings, two of them right. The carrier is the one that must give |

`pipe` is the word the codebase already uses for this language: `internal/component/command/pipe.go`,
`ParsePipe`, `knownPipeOps`, `ApplyPipes`, `ValidatePipes`, `ai/rules/pipe-completeness.md`,
and the `docs/architecture/api/commands.md -- CLI pipe operators` anchor. It also reads
correctly at the call site, where the command genuinely sits inside a shell pipe.

### Defect 2: the four compounds, and why they do not all move the same way

`zeDispatch` (`cmd/ze/ze_core_dispatch.go:286`) checks `isYANGVerb(arg)` at **:321** and
returns from inside that branch; `dispatchRegisteredRoot` is only reached at **:381**.
A root handler whose name is a YANG verb (`yangVerbs`, `cmd/ze/ze_core_dispatch.go:555`:
show, set, clear, request, delete, update, validate, monitor) is therefore
**unreachable**. This splits the four commands into two groups with two different fixes.

| Root | Registered at | Left segment | Is a YANG verb? | Target form | Mechanism |
|------|--------------|--------------|-----------------|-------------|-----------|
| `traffic-control` | `internal/component/traffic/cli/register.go:11` | `traffic` | No | `ze traffic control` | Root handler `traffic`, sub-dispatch on `control` |
| `ospf-decode` | `internal/plugins/ospf/cli/register.go:8` | `ospf` | No | `ze ospf decode` | Root handler `ospf`, sub-dispatch on `decode` |
| `isis-decode` | `internal/plugins/isis/cli/register.go:16` | `isis` | No | `ze isis decode` | Root handler `isis`, sub-dispatch on `decode` |
| `update-serve` | `cmd/ze/ze_core_dispatch.go:467` | `update` | **Yes** (`:555`) | `ze update serve` | **Local handler** `registry.MustRegisterLocalMeta("update serve", ...)` |

`update-serve` cannot become a root handler: `ze update serve` enters the YANG-verb
branch at `:321` and never reaches root dispatch. It must instead register in the
**local handler registry**, which `RunCommand` consults first
(`cmd/ze/internal/cmdutil/cmdutil.go:71`, "Check local handler registry first (offline
commands like version, completion)") via `registry.LookupLocal`
(`internal/component/command/registry/registry.go:312`), before the YANG tree validity
check. This is the mechanism `help command` and `help ai` already use
(`cmd/ze/ze_core_dispatch.go:498`, `:502`). Offline commands therefore live happily
under a YANG verb, so the split is feasible; it is a **change of registration
mechanism**, not a rename.

R9 evidence per command:

- **`traffic-control`**: `internal/component/traffic/yang/ze-traffic-control-conf.yang:53` opens `container traffic {` and `:56` nests `container control {`. YANG already models this as two tokens. `internal/plugins/traffic-cmd/yang/ze-traffic-cmd.yang:13,17` does the same. R9's own worked example names traffic as the canonical real namespace ("traffic-cmd owns it, trafficusage augments it", `ai/rules/cli-grammar.md:107`).
- **`ospf-decode`**: `container ospf` exists at `internal/plugins/ospf/yang/ze-ospf-conf.yang:212` and `internal/plugins/ospf/yang/ze-ospf-cmd.yang:30`. `ospf` is a real object namespace; `decode` is a member.
- **`isis-decode`**: same shape. **This overrides an earlier deliberate decision**: `internal/plugins/isis/cli/register.go:1-9` documents the hyphen as intentional, "a dedicated OFFLINE verb, intentionally distinct from the `isis` config component root (owned by isis-4) and the `show isis` / `clear isis` command tree (owned by isis-13). Keeping a separate verb avoids any collision with those siblings". That reasoning does not survive inspection: the "collision" it avoids is not a collision. The `isis` config root lives under the `set`/`delete` verbs and the command tree under `show`/`clear`; neither occupies the bare root token `isis`, which is currently unregistered. R9 step 1 applies plainly ("decode an IS-IS PDU"), and the sibling that supposedly collides is exactly what makes `isis` a namespace. User approved the override on 2026-07-17.
- **`update-serve`**: "run a local update server". R9 step 1 splits it. The `update` YANG verb is a mutation verb, which is why the mechanism must change (see above).

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] — checkboxes are template markers, not progress trackers. -->
<!-- Capture insights as → Decision: / → Constraint: annotations — these survive compaction. -->
- [ ] `ai/rules/cli-grammar.md` - R9, the enforcement feeders, and the backward-compat policy
  → Decision: "If the wrong grammar has not shipped, replace it outright. Do not add deprecation branches for unreleased syntax." (`:253-254`). `git tag` returns **zero** tags: every rename here is a hard cut with no alias.
  → Constraint: R9 (`:112-116`) "flags any child token whose left segment is itself a sibling name at the same tree level (`grammar.CheckSiblings`)... deliberately conservative and fires only when the namespace literally exists as a sibling". See A-7: this conservatism means R9 would NOT have caught `traffic-control`, because no `traffic` root existed to be its sibling. The root gate needs a cross-surface check, not just CheckSiblings.
  → Constraint: `pendingNamespaceSplit` (`scripts/checks/cli_grammar.go:74`) is the staging list for shipped-but-unsplit commands. It is empty and must stay empty: nothing here is shipped, so nothing may be staged.
  → Constraint: the three feeders are static gate (YANG tree), registration-time `validateCommandName` (plugin `CommandDecl`), and runtime guard (`TestRuntimeBuiltinSurfaceGrammar`). None sees root handlers. That is the gap this spec closes.
- [ ] `docs/architecture/cli/command-namespacing.md` - why the tree is object-rooted and filters are grammar, not flags
  → Constraint: R9 step 1 says the left part "becomes a container node so the tree stays object-rooted and completion can enumerate the members". After the split, `ze traffic` with no args must enumerate `control`, not error out.
- [ ] `docs/features/formatting.md` - the published operator taxonomy
  → Decision: operators carry a `Kind` of `format` / `filter` / `display`; only `format`-kind operators are formats.
  → Constraint: only one format operator is allowed per chain; filter and display operators chain freely (`:41-43`). The rename must not touch this.
- [ ] `ai/rules/no-layering.md` - replacement discipline
  → Constraint: "When replacing X with Y: DELETE X first, then implement Y. Never keep both." No aliases for any of the five renames.
- [ ] `ai/rules/plugin-self-containment.md` - who owns a command
  → Constraint: R9 step 4, "A split namespace needs one owning module... Never a shared parent that multiple plugins reach up into: that is the plugin-self-containment break the old `show ip` grouping caused." Each new root container (`traffic`, `isis`, `ospf`) must have exactly one owning package. See A-6 and R-5.
- [ ] `ai/rules/pipe-completeness.md` - the canonical operator table
  → Constraint: the operator set is defined once in `knownPipeOps`; this spec renames a caller, never the operator set.

### RFC Summaries (MUST for protocol work)
- N/A - no protocol behavior. `isis decode` and `ospf decode` are offline wire *tools*; their codecs are untouched.

**Key insights:**
- The pipe rename and the hyphen splits are the **same defect**: the root namespace was never subjected to the grammar rules. Fixing the four names without extending the gate leaves the hole open.
- The dispatch order (`isYANGVerb` at `:321` before `dispatchRegisteredRoot` at `:381`) is the hinge fact of this spec. It decides mechanism per command and it is the reason `update-serve` is not a simple rename.
- Completion, usage, and the TUI menu need **no edits**: all derive from `registry.ListRoot`/`ListRootBySection` (`internal/plugins/completion/root_commands.go:21`, `internal/plugins/completion/words.go:94`, `cmd/ze/ze_core_usage.go:31`, `cmd/ze/tui_menu.go:78`).

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
<!-- Same rule: never tick [ ] to [x]. Write → Constraint: annotations instead. -->
- [ ] `cmd/ze/ze_core_dispatch.go` - `zeDispatch` (`:286`) is the ze dispatch loop, installed via `binaryDispatch = zeDispatch` (`:113`). Order: pprof, config-file override, empty-args TUI/usage, then `isYANGVerb(arg)` (`:321`) which handles help paths and calls `cmdutil.RunCommand`, returning unconditionally from that branch; then `-h/--help` mapping; then the `run` deprecation stub; then `dispatchRegisteredRoot` (`:381`). Registers root `format` (`:492`) and root `update-serve` (`:467`). `yangVerbs` at `:555`. Local metas `help command` (`:498`) and `help ai` (`:502`).
  → Constraint: the YANG-verb branch at `:321` returns before root dispatch at `:381`. A root handler named for a YANG verb is dead code. This is what forces `update serve` onto the local-handler path.
  → Constraint: `knownCommands` (`:543`) unions `yangVerbs` and `registry.ListRoot`. It picks up renames for free.
- [ ] `cmd/ze/ze_core_format.go` - `runFormat` (`:22`), `formatUsage` (`:61`). Behavior in order: empty args prints usage, returns 1; `help`/`-h`/`--help` prints usage, returns 0; joins args with spaces, prefixes `"_ | "`, calls `command.ProcessPipesChecked`; on error writes `error: <msg>` to stderr, returns 1; reads stdin through a 256 MB limit reader, returns 1 if exceeded; applies the format function; writes stdout, appending a newline when the result is non-empty and unterminated; returns 0.
  → Constraint: the synthetic `"_ | "` prefix is load-bearing. `foldFilters` (`internal/component/command/pipe.go:110`) calls `lookupPipeFilters("_")`, finds no server-side filter set, and leaves every operator client-side. The placeholder must survive unchanged.
- [ ] `cmd/ze/internal/cmdutil/cmdutil.go` - `RunCommand` (`:53`). Builds the verb tree, extracts selectors, then checks `matchLocalHandler` (`:71`) **before** `ExtractOutputFormat`, before `LookupOfflineFallback`, and before the `IsValidCommand` rejection (`:91`). `matchLocalHandler` (`:37`) adapts `registry.LookupLocal`.
  → Constraint: a local handler short-circuits the YANG tree entirely, so `update serve` needs no YANG container to dispatch. Whether it needs one for *discoverability* is a separate question (see A-4, R-6).
- [ ] `internal/component/command/registry/registry.go` - `RegisterLocal` (`:172`), `RegisterLocalMeta` (`:187`), `MustRegisterLocalMeta` (`:205`), `LookupLocal` (`:312`). Local paths are space-separated strings, looked up by longest-prefix match over the command words.
- [ ] `internal/component/traffic/cli/register.go` - registers root `traffic-control` (`:11`) delegating to `Run(args)`; Meta Description "Linux tc / VPP policer helpers", Mode `offline`, Section `registry.SectionConfiguration`, empty `Subs`. Header comment (`:1-5`) declares this the owner package.
  → Constraint: `Subs` is empty today, so the split must populate it (`control`) or `ze traffic` help lists nothing.
- [ ] `internal/plugins/isis/cli/register.go` - registers root `isis-decode` (`:16`) delegating to `Run(args)`; Meta "Decode a hex IS-IS PDU from stdin to JSON (offline wire tool)", Mode `offline`, Section `registry.SectionConfiguration`, Subs `--pretty`. Header (`:1-9`) documents the deliberate-hyphen decision this spec overrides.
- [ ] `internal/plugins/ospf/cli/register.go` - registers root `ospf-decode` (`:8`); same shape, Design anchor `plan/learned/956-ospf-2-wire.md`.
- [ ] `internal/component/command/pipe.go` - `knownPipeOps` (`:52`), `ParsePipe` (`:68`), `foldFilters` (`:110`), `ApplyPipes` (`:285`), `ValidatePipes` (`:359`), `ProcessPipesChecked` (`:674`).
  → Constraint: NOT modified by this spec. The rename stops at the caller.
- [ ] `scripts/checks/cli_grammar.go` - the static gate. `pendingNamespaceSplit` (`:74`) is empty, documented (`:68-73`) as emptied by the 2026-07-13 `cli-hyphen-namespace-split` migration. Contains **no** reference to root handlers.
  → Constraint: this is the file the gate extension lands in (or beside). The empty `pendingNamespaceSplit` must stay empty.
- [ ] `internal/test/runner/runner_exec_util.go` - `isQuickExitZeCommand` (`:311`) classifies foreground `ze` **by exclusion**: quick-exit unless it carries a config-file arg, uses the web server, or names a daemon verb (`zeDaemonVerbs`: hub/start/cli/monitor). Comment at `:295` names "the 14 `ze format ...` steps in format-operators" in prose.
  → Constraint: no allow-list to update for `pipe`, `traffic`, `isis`, `ospf`. But `update serve` now begins with a YANG verb; confirm `update` is not in `zeDaemonVerbs` (A-5).
- [ ] `internal/component/cli/model_keys.go` - `handleSetCLIFormat` (`:432`), `validCLIFormats` (`:669`): `set cli format <text|table|json|yaml|ndjson>` persists a session default via `ze.cli.format`.
  → Constraint: a **correct** use of the word. Out of scope, must keep working.
- [ ] `internal/component/cli/model_load.go` - `ClassifyShowPipes` (`:951`) validates the editor pipe `| format tree|config`, rejecting anything else with "unknown format: %s (use tree or config)".
  → Constraint: a **correct** use of the word. Out of scope, must keep working.
- [ ] `test/ui/format-operators.ci` - 15 functional steps: json, json compact, yaml, table, text, ndjson, count (json), first 2, last 1, match (text), count (text), resolve (ips), `--help`, no-args, unknown-op.

**Behavior to preserve:**
- Every operator reachable offline today stays reachable: `json [compact]`, `ndjson`, `table`, `text`, `yaml`, `match <pattern>`, `count`, `first <n>`, `last <n>`, `resolve`.
- `runFormat`'s exit codes, 256 MB stdin limit and its error text, trailing-newline behavior, two-section help layout, and the synthetic `"_ | "` placeholder.
- The behavior of `Run(args)` in each of the three split packages: only the registration key and sub-token change, never the tool.
- `runUpdateServe(args)` behavior and its `--listen <addr>` sub.
- `set cli format <name>` and the editor's `| format tree|config`, untouched.
- `pendingNamespaceSplit` stays empty.

**Behavior to change:**
- `ze format <op>` becomes `ze pipe <op>`. `ze format` stops existing (no alias).
- `ze traffic-control` becomes `ze traffic control`; `ze isis-decode` becomes `ze isis decode`; `ze ospf-decode` becomes `ze ospf decode`. Old names stop existing.
- `ze update-serve` becomes `ze update serve`, moving from the root registry to the local-handler registry.
- The grammar gate gains a root-namespace feeder. A new hyphenated root whose left segment names a real namespace fails the gate.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Operator types `ze <token> [<token>] [args]` in a shell. For `ze pipe`, a second input arrives: the upstream command's output on stdin.

### Transformation Path
1. `main` → `binarySetup` → `binaryDispatch` (= `zeDispatch`, `cmd/ze/ze_core_dispatch.go:113`).
2. `zeDispatch` reads `arg = args[0]`.
3. **If `isYANGVerb(arg)`** (`:321`) — the `update serve` path: `cmdutil.RunCommand` builds the verb tree, extracts selectors, then `matchLocalHandler` (`cmd/ze/internal/cmdutil/cmdutil.go:71`) finds the registered local path `update serve` and calls the handler in-process. The YANG tree is never consulted. Returns without reaching `:381`.
4. **Else** (`:381`) — the `pipe` / `traffic` / `isis` / `ospf` path: `dispatchRegisteredRoot` → `registry.LookupRoot(arg)` → handler with `args[1:]`.
5. For `traffic` / `isis` / `ospf`: the handler dispatches on `args[0]` against a closed keyword set (`control`, `decode`), then delegates to the existing `Run`.
6. For `pipe`: `runPipe` joins argv, prefixes `"_ | "`, `ProcessPipesChecked` → `ParsePipe` → `foldFilters` (no server filter set for `_`, so all ops stay client-side) → `ValidatePipes` → format closure; stdin is read through the 256 MB limit reader; `ApplyPipes` runs; result to stdout.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Shell ↔ ze | argv + stdin + exit code | [ ] |
| Root dispatch ↔ handler | `registry.MustRegisterRootHandler` / `registry.LookupRoot` (`:381`) | [ ] |
| YANG-verb dispatch ↔ local handler | `registry.MustRegisterLocalMeta` / `registry.LookupLocal` via `RunCommand` (`:321` → cmdutil `:71`) | [ ] |
| Handler ↔ pipe engine | `command.ProcessPipesChecked` returning a `func(string) string` | [ ] |
| Gate ↔ root registry | new feeder enumerating registered roots (Phase 6) | [ ] |

### Integration Points
- `registry.MustRegisterRootHandler` / `LookupRoot` - four registration-key changes.
- `registry.MustRegisterLocalMeta` / `LookupLocal` - one mechanism change (`update serve`).
- `registry.ListRoot` / `ListRootBySection` - completion, usage, and TUI menu derive from these and pick every rename up for free.
- `command.ProcessPipesChecked` - unchanged contract.
- `scripts/checks/cli_grammar.go` - gains the root feeder.

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)
- [ ] Registration over hardcoding — every command stays registry-registered and core-discovered; the sub-token dispatch in `traffic`/`isis`/`ospf` is a closed keyword switch inside the **owning** package, not a new case in a core/shared package

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `pipe`, `traffic`, `isis`, `ospf` are free as root command names | `grep MustRegisterRootHandler(` over `cmd/` + `internal/` lists 36 roots; none of the four appears | `MustRegisterRootHandler` panics on duplicate at startup | Re-run the grep; the four build-tag root tests (`cmd/ze/build_tag_full_test.go:14` and siblings) must pass | **confirmed** — audit grep over cmd/+internal/ lists 36 `MustRegisterRootHandler`+5 `RegisterRoot`; `grep RegisterRootHandler("(pipe\|traffic\|isis\|ospf)"` returns NONE |
| A-2 | Nothing here has shipped, so hard cuts need no deprecation | `git tag` returns zero tags | A released command needs the old grammar accepted with a one-shot deprecation warning (`ai/rules/cli-grammar.md:255`) | `git tag`; user confirmation | **confirmed** — `git tag \| wc -l` = 0 |
| A-3 | Completion, usage, and the TUI menu need no edits | `internal/plugins/completion/root_commands.go:21`, `internal/plugins/completion/words.go:94`, `cmd/ze/ze_core_usage.go:31`, `cmd/ze/tui_menu.go:78` all derive from `ListRoot`/`ListRootBySection` | Stale root names survive in completion after the rename | Grep for a hardcoded root list; drive completion in the functional test (AC-12) | **confirmed** — all four sites call `registry.ListRoot()`/`ListRootBySection()`; no hardcoded root list found |
| A-4 | A local handler needs no YANG container to dispatch | `RunCommand` checks `matchLocalHandler` (`cmd/ze/internal/cmdutil/cmdutil.go:71`) before `IsValidCommand` (`:91`); `help command` / `help ai` already do this (`cmd/ze/ze_core_dispatch.go:498`) | `update serve` needs a YANG container + `ze:command` annotation, enlarging Phase 5 substantially | Phase 5's failing-then-passing functional test (AC-8) | **confirmed** — `RunCommand` (`cmdutil.go:72`) calls `matchLocalHandler` before `ExtractOutputFormat`/`IsValidCommand` (`:92`); `show version` local meta (`ze_core_dispatch.go:439`) is the exact precedent: a local command under the `show` YANG verb |
| A-5 | `update` is not a daemon verb, so the runner still treats `ze update serve` as quick-exit | `isQuickExitZeCommand` (`internal/test/runner/runner_exec_util.go:311`) excludes only `zeDaemonVerbs` (hub/start/cli/monitor) | The `.ci` step hangs waiting for a daemon that never starts, or races on shared stdout | Read `zeDaemonVerbs`; `test/ui/root-namespace.ci` completing deterministically | **confirmed with refinement** — `zeDaemonVerbs={hub,start,cli,monitor}` (`runner_exec_util.go:270`), `update` absent. BUT `runUpdateServe` blocks on `ListenAndServe`, so the AC-8 `.ci` step uses `cmd=background`+`http=wait` (not foreground quick-exit) — the UI runner supports both (`test/ui/web-commit-reject.ci`); `waitReady` is best-effort (non-fatal), so a background `ze update serve` never hangs |
| A-6 | Each new root container has exactly one owning package | `internal/component/traffic/cli/register.go:1-5` declares itself owner; isis/ospf equivalents likewise | R9 step 4 breaks: a shared parent multiple plugins reach into, the `show ip` failure mode | Grep for any other package registering the same root; `make ze-tier-check` | **confirmed** — audit grep: `traffic-control`/`isis-decode`/`ospf-decode` each registered exactly once, in their owner `cli` packages |
| A-7 | `grammar.CheckSiblings` alone cannot catch these four, so the root feeder needs a cross-surface check | R9 "fires only when the namespace literally exists as a sibling" (`ai/rules/cli-grammar.md:115-116`); no `traffic`/`isis`/`ospf` root existed to be a sibling, which is exactly why the gate stayed green while the defect sat there | The feeder is a thin reuse of CheckSiblings and Phase 6 is much smaller | Phase 6: run the feeder against the **pre-split** tree; it must flag all four. If it flags zero, the check is wrong | **confirmed** — `CheckSiblings` (`grammar/checker.go:157`) checks a token's left segment only against siblings AT THE SAME LEVEL; the roots have no `traffic`/`isis`/`ospf`/`update` root sibling. New pure fn `CheckRootNamespace` checks the left segment against a cross-surface namespace set (YANG containers `traffic`/`isis`/`ospf` confirmed present + the 8 YANG verbs); `TestRootNamespaceGrammar` drives it red-then-green on fixtures |
| A-8 | The R1-R9 static gate does not currently apply to root handlers | `grep -i root scripts/checks/cli_grammar.go` returns nothing; `ai/rules/cli-grammar.md:286` scopes the static gate to the YANG command tree | The gate already covers roots and Phase 6 is redundant | `make ze-cli-grammar-check` on the pre-split tree passes despite four hyphenated roots (that green-on-broken result IS the proof) | **confirmed** — `go run scripts/checks/cli_grammar.go` on the CURRENT (pre-split) tree exits 0 / `cli-grammar: OK` despite four hyphenated roots present; the gate walks only the YANG tree and never sees root handlers |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A stale `ze format` / `ze traffic-control` / `ze isis-decode` / `ze ospf-decode` / `ze update-serve` reference rots in docs, demos, or comments | `grep -rn` for each old name returns a hit that is not enumerated below as confusable | Pre-Commit Verification greps **all five** old names and classifies every hit; the confusable prose hits are enumerated in this spec so they are never mistaken for stragglers |
| R-2 | The gate extension is written to pass on today's tree rather than to catch the defect | Phase 6's feeder flags zero commands when run against the pre-split tree | Phase 6 is ordered **before** the splits are complete on the gate's own test fixture: the feeder must be demonstrated failing on all four old names first (A-7). This is the TDD red that matters most in this spec |
| R-3 | `update serve` silently stops working because a local-handler path is subtly different from a root handler (selector extraction, arg passing) | `ze update serve --listen :8080` passes `--listen` differently than before | AC-8 drives the real flag through the functional test, not just the bare verb. Note `RunCommand` appends the selector as a trailing arg (`cmd/ze/internal/cmdutil/cmdutil.go:43`) — confirm no selector is extracted for this path |
| R-4 | A rename lands but the old name survives via a fallback | `ze format json` still exits 0 after the change | Each rename has a negative AC asserting the old name fails (AC-2, AC-9) |
| R-5 | Creating roots `traffic`/`isis`/`ospf` invites other packages to reach into them later | A second package registers a sub-token under a root it does not own | R9 step 4 and `ai/rules/plugin-self-containment.md`; the owning package's header comment states ownership; A-6 grep at audit time |
| R-6 | `ze traffic` with no args gives a bare error instead of enumerating `control`, so the split loses the discoverability that motivated it | Manual run of `ze traffic` | AC-10 asserts the no-args case enumerates members; `Subs` is populated for each new root (it is empty today for `traffic-control`) |
| R-7 | Renaming files loses git history | `git log --follow` shows one commit | Rename in a single commit with no content shuffling beyond identifiers, so rename detection holds |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze pipe match <pattern>` with text on stdin | → | `runPipe` → `ProcessPipesChecked` → `ApplyPipes` | `test/ui/pipe-operators.ci` match step |
| `ze format json` (removed name) | → | root dispatch miss, no handler | `test/ui/pipe-operators.ci` removed-name step |
| `ze traffic control <args>` | → | root `traffic` → sub-dispatch `control` → `traffic/cli.Run` | `test/ui/root-namespace.ci` traffic step |
| `ze isis decode` with hex on stdin | → | root `isis` → sub-dispatch `decode` → `isis/cli.Run` | `test/ui/root-namespace.ci` isis step |
| `ze ospf decode` with hex on stdin | → | root `ospf` → sub-dispatch `decode` → `ospf/cli.Run` | `test/ui/root-namespace.ci` ospf step |
| `ze update serve --listen <addr>` | → | `isYANGVerb` → `RunCommand` → `matchLocalHandler` → `runUpdateServe` | `test/ui/root-namespace.ci` update-serve step |
| A new hyphenated root whose left segment is a namespace | → | root feeder in the grammar gate | `TestRootNamespaceGrammar` (gate unit test, fixture-driven) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze pipe match reactor` with the multi-line text fixture on stdin | Exit 0; stdout contains only the matching line(s) |
| AC-2 | `ze format json` with JSON on stdin | Non-zero exit; stderr reports an unknown command. The name is gone, not aliased |
| AC-3 | `ze pipe --help` | Exit 0; help titled `ze pipe`; both `Format Operators` and `Filter Operators` sections present; examples use `ze pipe` |
| AC-4 | `ze pipe` with no arguments | Usage printed; exit 1 |
| AC-5 | `ze pipe nosuchop` | `error: ` on stderr; exit 1 |
| AC-6 | Each of `json`, `json compact`, `ndjson`, `table`, `text`, `yaml`, `count`, `first 2`, `last 1`, `resolve` under `ze pipe` | Exit 0, output identical to what the same operator produced under `ze format` before the rename |
| AC-7 | `ze traffic control`, `ze isis decode`, `ze ospf decode` with each tool's existing valid input | Exit code and stdout identical to the pre-split hyphenated command's |
| AC-8 | `ze update serve --listen <addr>` | Behaves exactly as `ze update-serve --listen <addr>` did: the flag reaches `runUpdateServe` and the server listens |
| AC-9 | Each of `ze traffic-control`, `ze isis-decode`, `ze ospf-decode`, `ze update-serve` | Non-zero exit; unknown-command error. All four old names are gone, not aliased |
| AC-10 | `ze traffic`, `ze isis`, `ze ospf` with no arguments | Usage enumerating the members (`control`, `decode`, `decode`); non-zero exit. The container is discoverable (R-6) |
| AC-11 | `ze help command` output | Lists `pipe`, `traffic control`, `isis decode`, `ospf decode`, `update serve`; lists none of the five old names; `pipe`'s description does not call filters "formatting" |
| AC-12 | Root-command completion is requested | Offers `pipe`, `traffic`, `isis`, `ospf`; offers none of the four hyphenated roots |
| AC-13 | The root-namespace feeder runs against a fixture reproducing the pre-split root set | Flags all four hyphenated roots. Run against the post-split set, flags zero |
| AC-14 | `make ze-cli-grammar-check` after the splits | Passes; `pendingNamespaceSplit` still empty; a newly introduced hyphenated-namespace root in a fixture fails the gate |
| AC-15 | `grep -rn` for each of the five old command names | Returns only the confusable prose hits enumerated in this spec |
| AC-16 | `set cli format table` in the interactive CLI; `show \| format config` in the editor | Both still work exactly as before. The legitimate uses of the word are untouched |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Filters captured output offline: `ze debug show \| ze pipe match reactor` | shell → root dispatch (`:381`) → `runPipe` → `ProcessPipesChecked` → `ApplyPipes` → stdout | `test/ui/pipe-operators.ci` match step (AC-1) |
| 2 | Counts plugins in a demo: `ze --plugins \| ze pipe count` | shell → `runPipe` → `ApplyPipes` count → stdout | `demos/terminal/config-views/validate.sh` (AC-6) |
| 3 | Configures traffic shaping offline: `ze traffic control <args>` | shell → root `traffic` → keyword `control` → `traffic/cli.Run` | `test/ui/root-namespace.ci` traffic step (AC-7) |
| 4 | Decodes a captured IS-IS PDU: `ze isis decode` with hex on stdin | shell → root `isis` → keyword `decode` → `isis/cli.Run` | `test/ui/root-namespace.ci` isis step (AC-7) |
| 5 | Serves firmware updates: `ze update serve --listen :8080` | shell → `isYANGVerb("update")` (`:321`) → `RunCommand` → `matchLocalHandler` (`cmdutil.go:71`) → `runUpdateServe` | `test/ui/root-namespace.ci` update-serve step (AC-8) |
| 6 | Discovers what `traffic` offers: types `ze traffic` | shell → root `traffic` → no args → usage from `Subs` | `test/ui/root-namespace.ci` discoverability step (AC-10) |
| 7 | Types an old name out of habit: `ze traffic-control` | shell → root dispatch miss → unknown-command error | `test/ui/root-namespace.ci` removed-names step (AC-9) |
| 8 | A future contributor adds a hyphenated root that is really a namespace | `make ze-cli-grammar-check` → root feeder → CheckSiblings + cross-surface check → fail | `TestRootNamespaceGrammar` (AC-13, AC-14) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRunPipe` | `cmd/ze/ze_core_pipe_test.go` | Exit codes for the operator table, help, and no-args paths (renamed from `cmd/ze/ze_core_format_test.go:24`) | |
| `TestRunPipeUnknownOperator` | `cmd/ze/ze_core_pipe_test.go` | Unknown operator returns 1 (renamed from `cmd/ze/ze_core_format_test.go:32`) | |
| `TestRootsRegistered` | `cmd/ze/ze_core_dispatch_test.go` | `LookupRoot` resolves `pipe`, `traffic`, `isis`, `ospf`; resolves **none** of `format`, `traffic-control`, `isis-decode`, `ospf-decode`, `update-serve`. The negative half is what AC-2/AC-9 rest on | |
| `TestUpdateServeLocalRegistered` | `cmd/ze/ze_core_dispatch_test.go` | `registry.LookupLocal(["update","serve"])` resolves and returns the remaining args unchanged | |
| `TestTrafficSubDispatch` | `internal/component/traffic/cli/register_test.go` | `control` delegates to `Run`; an unknown sub-token errors with a keyword hint, never falls through to `Run` | |
| `TestISISSubDispatch` | `internal/plugins/isis/cli/register_test.go` | as above for `decode` | |
| `TestOSPFSubDispatch` | `internal/plugins/ospf/cli/register_test.go` | as above for `decode` | |
| `TestRootNamespaceGrammar` | `scripts/checks/cli_grammar_test.go` | **Fixture-driven, both directions:** a fixture root set containing the four pre-split names flags exactly those four; the post-split set flags zero; a fresh hyphenated-namespace root flags. This is the gate's own red-then-green (R-2) | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| stdin size (`ze pipe`) | 0 - 256 MB | 256 MB | N/A | 256 MB + 1 byte → exit 1 |
| `first <n>` / `last <n>` | existing operator behavior, unchanged by this spec | N/A | N/A | N/A |

<!-- The stdin limit is preserved behavior, not new: the boundary is enforced in runFormat
     today and carries over verbatim. No new numeric input is introduced by this spec. -->

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `pipe-operators` | `test/ui/pipe-operators.ci` | Every operator, help, no-args, unknown-op, removed-name, completion (renamed and extended from `test/ui/format-operators.ci`) | |
| `root-namespace` | `test/ui/root-namespace.ci` | `traffic control`, `isis decode`, `ospf decode`, `update serve --listen`, the four removed names, and the no-args discoverability of each new container | |

### Interop Tests (MANDATORY for protocol features)
- N/A - no wire protocol behavior changes. `isis decode` and `ospf decode` are offline tools reached by a new token path; their codecs are untouched and their existing wire tests (`test/isis-wire/`) still cover the decode behavior itself.

### Future (if deferring any tests)
- None. Every AC has a test in this plan.

## Files to Modify

- `cmd/ze/ze_core_dispatch.go` - root key `format` → `pipe` at `:492` with an honest `Meta.Description`; remove the `update-serve` root registration at `:467` and re-register as local meta `update serve` beside the existing `help command` / `help ai` local metas (`:498`).
- `internal/component/traffic/cli/register.go` - root key `traffic-control` → `traffic`; add `control` sub-dispatch; populate `Subs` (empty today); update the Design header comment.
- `internal/plugins/isis/cli/register.go` - root key `isis-decode` → `isis`; add `decode` sub-dispatch; `Subs` becomes `decode [--pretty]`; **rewrite the `:1-9` header comment**, which documents the superseded deliberate-hyphen decision.
- `internal/plugins/ospf/cli/register.go` - root key `ospf-decode` → `ospf`; add `decode` sub-dispatch; `Subs` becomes `decode [--pretty]`; update the Design header.
- `scripts/checks/cli_grammar.go` - add the root-namespace feeder; keep `pendingNamespaceSplit` empty.
- `ai/rules/cli-grammar.md` - the enforcement table (`:284-290`) claims three feeders over the YANG tree. Document the root feeder as a fourth, and correct the R9 scope so the next reader does not repeat the assumption that roots are governed.
- `internal/test/runner/runner_exec_util.go` - prose comment at `:295` naming `ze format ...` steps.
- `internal/plugins/host/host.go` - doc comment at `:8` ("pipe the JSON through `ze format table` or `jq`").
- `demos/terminal/config-views/validate.sh` - call sites at `:13`, `:22`, `:24`.
- `demos/terminal/cards.sh` - narration prose at `:211`, `:224`.
- `docs/features.md` - the Resolution CLI and Pipes row at `:75` and its `<!-- source: cmd/ze/ze_core_format.go -- runFormat -->` anchor.
- `docs/guide/command-reference.md` - the `### ze format` section at `:1551-1567`, plus any `traffic-control` / `isis-decode` / `ospf-decode` / `update-serve` entries (grep during Phase 7).
- `docs/features/formatting.md` - "The offline way" section at `:64-81` and its `<!-- source: cmd/ze/ze_core_format.go -- runFormat, formatUsage -->` anchor.
- `ai/DOCS-TO-CODE.md` - rows at `:194-195` mapping the old file paths.

### Files NOT modified (deliberate)

| File | Why |
|------|-----|
| `internal/component/command/pipe.go` | The operator language is correct and stays. This spec renames a caller |
| `internal/component/cli/model_keys.go` | `set cli format <name>` is a genuine format selector |
| `internal/component/cli/model_load.go`, `internal/component/cli/completer.go` | `\| format tree\|config` is a genuine format selector |
| `internal/plugins/completion/*` | Derives from `registry.ListRoot` (A-3) |
| `cmd/ze/ze_core_usage.go`, `cmd/ze/tui_menu.go` | Derive from `registry.ListRootBySection` (A-3) |
| `internal/component/traffic/cli/*.go` (except register.go), isis/ospf `Run` implementations | The tools are correct; only their token path changes |

### Confusable grep hits (NOT callers)

AC-15 greps the five old names. These hits mean "ExaBGP's config converted to ze's own
format" and MUST NOT be rewritten. Enumerated so no future session mistakes them for
stragglers:

| File | Line | Text |
|------|------|------|
| `internal/plugins/exabgp/main.go` | 62, 193, 201 | "Convert ExaBGP config to ze format" |
| `internal/exabgp/migration/migrate.go` | 642 | "into the unified ze format" |
| `test/parse/cli-exabgp-migrate.ci` | 1 | "converts ExaBGP config to ze format" |
| `docs/guide/config-editor.md` | 44 | "Convert ExaBGP config to ze format" |
| `docs/features/exabgp-compatibility.md` | 12 | "converts ExaBGP configs to ze format" |
| `plan/learned/028-spec-api-test-features.md` | 9 | "version 7 (ze format)" |

Separately, `plan/learned/928-isis-2-wire.md` and `plan/learned/956-ospf-2-wire.md` are
the Design anchors for the isis/ospf registrations and **record history**. Learned
summaries are an append-only record of what was decided then; do not rewrite them to
match the new names. The new learned summary for this spec supersedes them, and the
`// Design:` anchors in the register.go files are repointed instead.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | [ ] No - root handlers carry no YANG path; `update serve` dispatches via the local registry ahead of the tree (A-4) | - |
| YANG validation constraints | [ ] No - no leaves added | - |
| YANG custom validators | [ ] No | - |
| CLI commands/flags | [ ] Yes | `cmd/ze/ze_core_dispatch.go`, `cmd/ze/ze_core_pipe.go`, the three `register.go` files |
| CLI grammar (action before identifier) | [ ] Yes - this spec **is** the grammar work; the gate extension is Phase 6 | `ai/rules/cli-grammar.md`, `scripts/checks/cli_grammar.go` |
| Editor autocomplete | [ ] No - derives from `registry.ListRoot` (A-3) | - |
| Functional test for new RPC/API | [ ] Yes | `test/ui/pipe-operators.ci`, `test/ui/root-namespace.ci` |
| Pipe completeness | [ ] No - operator set untouched; this renames the offline carrier of that set | `ai/rules/pipe-completeness.md` |
| Env var registration | [ ] No - `ze.cli.format` is the interactive default, out of scope | - |
| Doctor check for runtime dependencies | [ ] No - no new file path, socket, service, port, or binary. `update serve` already listens and already carries whatever check it had | - |
| Prometheus counters/metrics | [ ] No - no new observable state | - |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] Yes - five renamed surfaces | `docs/features.md:75` |
| 2 | Config syntax changed? | [ ] No - YANG containers untouched | - |
| 3 | CLI command added/changed? | [ ] Yes | `docs/guide/command-reference.md` (`ze format` at `:1551-1567`; grep the other four) |
| 4 | API/RPC added/changed? | [ ] Yes - `// Design:` anchors point at it | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | [ ] Yes - isis/ospf plugin command surfaces | `docs/guide/plugins.md` (verify by grep) |
| 6 | Has a user guide page? | [ ] Yes | `docs/features/formatting.md:64-81` |
| 7 | Wire format changed? | [ ] No | - |
| 8 | Plugin SDK/protocol changed? | [ ] No | - |
| 9 | RFC behavior implemented, changed, or newly proven? | [ ] No | - |
| 10 | Test infrastructure changed? | [ ] Yes - `.ci` renamed and one added | `docs/functional-tests.md` (verify by grep) |
| 11 | Affects daemon comparison? | [ ] No | - |
| 12 | Internal architecture changed? | [ ] Yes - a fourth grammar feeder exists | `ai/rules/cli-grammar.md:284-290` enforcement table |
| 13 | Route metadata keys added/changed? | [ ] No | - |
| 14 | Prometheus counters added/changed? | [ ] No | - |
| 15 | Registered plugin, event type, send type, command, capability, or runtime inventory changed? | [ ] Yes - five registered commands renamed | `docs/plugin-overview.md`, `docs/guide/status.md` (verify by grep) |
| 16 | Any changed source file is referenced by existing doc source anchors? | [ ] Yes - two anchors name `cmd/ze/ze_core_format.go` | `docs/features.md:75`, `docs/features/formatting.md:83` |
| 17 | Existing docs show config/CLI/API examples for this area? | [ ] Yes - every example of the five old names | `docs/features.md`, `docs/guide/command-reference.md`, `docs/features/formatting.md` |

## Files to Create
- `cmd/ze/ze_core_pipe.go` - git-mv of `cmd/ze/ze_core_format.go`; `runFormat` → `runPipe`, `formatUsage` → `pipeUsage`; help `Command` becomes `ze pipe`; usage line and all six examples updated; `// Design:` header updated.
- `cmd/ze/ze_core_pipe_test.go` - git-mv of `cmd/ze/ze_core_format_test.go`.
- `cmd/ze/ze_core_dispatch_test.go` - `TestRootsRegistered`, `TestUpdateServeLocalRegistered` (create if absent; otherwise add to the existing file).
- `internal/component/traffic/cli/register_test.go` - `TestTrafficSubDispatch`.
- `internal/plugins/isis/cli/register_test.go` - `TestISISSubDispatch`.
- `internal/plugins/ospf/cli/register_test.go` - `TestOSPFSubDispatch`.
- `scripts/checks/cli_grammar_test.go` - `TestRootNamespaceGrammar` (create if absent).
- `test/ui/pipe-operators.ci` - git-mv of `test/ui/format-operators.ci`, plus removed-name and completion steps.
- `test/ui/root-namespace.ci` - the four split commands, their removed old names, and container discoverability.

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, TDD Test Plan; validate A-1..A-8 by grep/read before coding. A-8 is validated by observing the gate pass on the **broken** tree |
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

1. **Phase: Wiring (MANDATORY FIRST)** — prove the new entry points and the old names' absence
   - Tests: `TestRootsRegistered`, `TestUpdateServeLocalRegistered`
   - Files: `cmd/ze/ze_core_dispatch_test.go`
   - Verify: both tests fail first (old keys still registered), establishing the negative assertions AC-2/AC-9 rest on

2. **Phase: `format` → `pipe`** — the carrier rename
   - Tests: `TestRunPipe`, `TestRunPipeUnknownOperator`; `test/ui/pipe-operators.ci`
   - Files: `cmd/ze/ze_core_format.go` → `cmd/ze/ze_core_pipe.go`, its test, `cmd/ze/ze_core_dispatch.go:492`, `test/ui/format-operators.ci` → `test/ui/pipe-operators.ci`
   - Verify: git records renames (R-7); `runPipe` is behaviorally identical to `runFormat` on every preserved path; all 15 existing `.ci` steps pass under the new name plus the removed-name and completion steps

3. **Phase: `traffic-control` → `traffic control`** — the clearest split, done first as the pattern for isis/ospf
   - Tests: `TestTrafficSubDispatch`; `test/ui/root-namespace.ci` traffic + discoverability steps
   - Files: `internal/component/traffic/cli/register.go`
   - Verify: `ze traffic control` matches the old command's behavior; `ze traffic` enumerates `control` (R-6); an unknown sub-token errors rather than falling through to `Run`

4. **Phase: `isis-decode` and `ospf-decode`** — same pattern, twice
   - Tests: `TestISISSubDispatch`, `TestOSPFSubDispatch`; `test/ui/root-namespace.ci` isis + ospf steps
   - Files: `internal/plugins/isis/cli/register.go`, `internal/plugins/ospf/cli/register.go`
   - Verify: decode output byte-identical to the pre-split command's for the same hex input; the isis header comment no longer asserts the superseded deliberate-hyphen rationale

5. **Phase: `update-serve` → `update serve`** — the mechanism change, not a rename
   - Tests: `TestUpdateServeLocalRegistered`; `test/ui/root-namespace.ci` update-serve step
   - Files: `cmd/ze/ze_core_dispatch.go` (drop the root at `:467`, add the local meta near `:498`)
   - Verify: `ze update serve --listen <addr>` reaches `runUpdateServe` with `--listen` intact (R-3); confirm no selector extraction mangles the args (`cmd/ze/internal/cmdutil/cmdutil.go:43`); confirm A-4 (no YANG container needed) and A-5 (still quick-exit in the runner)

6. **Phase: the gate (THE LOAD-BEARING PHASE)** — extend grammar enforcement to the root namespace
   - Tests: `TestRootNamespaceGrammar`
   - Files: `scripts/checks/cli_grammar.go`, `scripts/checks/cli_grammar_test.go`, `ai/rules/cli-grammar.md:284-290`
   - Verify: **the feeder must first be demonstrated flagging all four pre-split names on a fixture** (A-7, R-2). A feeder that only passes on the fixed tree proves nothing. Then: post-split fixture flags zero; a fresh hyphenated-namespace root fails; `pendingNamespaceSplit` still empty; `make ze-cli-grammar-check` green
   - Note: R9's `CheckSiblings` alone is insufficient here (A-7) — no `traffic` root existed to be `traffic-control`'s sibling, which is precisely why the gate stayed green while four violations sat in the tree. The feeder needs a cross-surface check: a hyphenated root whose left segment names a YANG container or verb is a namespace violation

7. **Phase: callers and documentation** — demos, comments, every doc example and source anchor
   - Files: per Files to Modify and the Documentation Update Checklist
   - Verify: `make ze-doc-test`; `scripts/dev/check_doc_links.py --design-only` clean; AC-15 grep returns only enumerated confusable hits

8. **Full verification** → `make ze-verify`
9. **Complete spec** → Fill audit tables, write learned summary to `plan/learned/NNN-cli-root-namespace-grammar.md`. TWO commits: commit A saves code + tests + docs + spec + learned summary; commit B does `git rm` of the spec. Repoint the `// Design:` anchors in the isis/ospf register.go files at the new summary before commit B (`ai/rules/planning.md` "Design references survive closure").

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-16 has implementation or test evidence with file:line |
| Feature completeness | Every operator reachable under `ze format` before is reachable under `ze pipe` after — compare against `knownPipeOps` (`internal/component/command/pipe.go:52`), not against the old help text. Each split tool's behavior is unchanged, compared against its pre-split output |
| Correctness | `runPipe` exit codes, 256 MB stdin limit, trailing-newline behavior byte-identical to `runFormat`. `update serve` passes `--listen` through intact |
| Gate honesty | `TestRootNamespaceGrammar` fails on the pre-split fixture. A gate that cannot catch the bug it was written for is not a gate (R-2) |
| Naming | No identifier, comment, file name, or test name still uses an old command name where it means the command. `format` survives ONLY where it means an actual format |
| Data flow | The synthetic `"_ | "` placeholder is intact, so `foldFilters` still leaves every operator client-side |
| Dispatch order | No new root handler is named for a YANG verb (it would be unreachable behind `:321`). `update serve` is a local handler, not a root |
| CLI grammar | `make ze-cli-grammar-check` green; `pendingNamespaceSplit` still empty; `ai/rules/cli-grammar.md` enforcement table now describes four feeders, not three |
| Registration over hardcoding | Every command stays registry-registered; sub-token dispatch is a closed keyword switch **inside the owning package**, never a case added to a core/shared package |
| Plugin self-containment | Each new root container has exactly one owning package (A-6, R-5); no package reaches into a root it does not own |
| Rule: no-layering | All five old names fully deleted. No alias, no fallback, no deprecation branch (A-2) |
| Rule: no-fabrication | Every claim in the learned summary cites the producing function |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| `cmd/ze/ze_core_pipe.go` exists, `ze_core_format.go` does not | `ls` both paths |
| `test/ui/pipe-operators.ci` and `test/ui/root-namespace.ci` exist; `format-operators.ci` does not | `ls` all three |
| Git recorded renames, not rewrites | `git log --follow --oneline cmd/ze/ze_core_pipe.go` shows pre-rename history |
| New roots registered, old ones gone | `TestRootsRegistered` passes |
| `update serve` reachable through the YANG-verb path | `TestUpdateServeLocalRegistered` + the `.ci` step pass |
| The gate catches the class | `TestRootNamespaceGrammar` flags all four on the pre-split fixture |
| `pendingNamespaceSplit` still empty | `grep -A2 'pendingNamespaceSplit = map' scripts/checks/cli_grammar.go` |
| No stale references | AC-15 grep for all five old names |
| Legitimate `format` uses intact | `grep -n 'handleSetCLIFormat' internal/component/cli/model_keys.go`; `grep -n 'cmdFormat' internal/component/cli/model_load.go` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | The 256 MB stdin limit reader survives the `pipe` rename; oversize stdin still exits 1 rather than allocating unbounded |
| Input validation (sub-dispatch) | Each new sub-token switch matches `args[0]` against a **closed keyword set** before doing anything with it (`ai/rules/cli-grammar.md` mechanical check); an unknown token must never reach a lookup/parse function |
| Privilege surface | `update serve` binds a listener. Moving it under the `update` verb must not change who can invoke it or make it reachable from a read-only path — confirm it is not treated as read-only (`IsReadOnlyVerb`/`IsReadOnlyPath`, `internal/component/plugin/server/command.go`) |
| Error leakage | Unknown-operator and unknown-sub-token errors report only the token, never stdin content |
| Resource exhaustion | No new buffering; `runPipe` reads once through the limit reader exactly as `runFormat` did |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior → RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural → DESIGN phase |
| Functional test fails | Check AC; if AC wrong → DESIGN; if AC correct → IMPLEMENT |
| A-4 broken (`update serve` needs a YANG container) | STOP. That enlarges Phase 5 into YANG schema work; present to the user before proceeding |
| A-7 broken (CheckSiblings alone suffices) | Good news, simplify Phase 6 — but only after the fixture demonstrably flags all four |
| The root feeder cannot express the cross-surface check cleanly | STOP and present. Do NOT stage the four in `pendingNamespaceSplit`: they are unshipped, so that list is the wrong tool (`scripts/checks/cli_grammar.go:68-73`) |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| A-6 ("each new root container has exactly one owning package") covered the collision surface | A second package — the `ze-test` suite runner (`internal/test/cli`) — also registers `traffic`/`isis`/`ospf` roots (`ze-test <suite>`), and it pulls the tool roots in via `plugin/all`. Two packages claimed the same root name in one binary once the hyphen was gone | `bin/ze-test ui` panicked on `duplicate root command "traffic"`; `go test ./internal/test/... -tags ze_isis ze_ospf` panicked on `isis` | Added `// codegen:skip` to the three `cli/register.go` so the tools drop from `plugin/all` (Deviations). No design change; `ze` keeps the tools via direct dispatch imports |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights
<!-- LIVE — write IMMEDIATELY when you learn something -->

- A gate that reads one surface silently certifies the surfaces it cannot see. `make ze-cli-grammar-check` has been green this whole time with four R9 violations in the tree, because root handlers are not YANG. Green meant "the YANG tree is clean", and everyone read it as "the CLI is clean".
- R9's conservatism (`CheckSiblings` fires only when the namespace literally exists as a sibling) is correct for avoiding false positives but blind to the case where the namespace exists on a *different surface*: `traffic` was a YANG container the whole time, just never a root. Conservative checks need a cross-surface input, or they only catch the violations that arrive second.
- The offline carrier was named after the operator an early user would reach for, not after the thing it carries. A command named for a use case rather than its contract drifts the moment a second use case arrives; `ze format match` is that drift made visible.
- The repo's docs had the right taxonomy (`docs/features/formatting.md` `Kind`: format / filter / display) before the code caught up. When a doc and a command name disagree about what a thing is, the doc is usually the later, more considered artifact.
- `isis-decode`'s header comment shows how a hyphen gets *rationalized*: the sibling it cited as a collision risk (`isis` config root, `show isis` tree) is exactly the evidence that `isis` is a namespace. The reasoning inverted the rule it should have applied.

## Core Insight

Name a carrier after the language it carries, not one clause of it; and gate every
surface the rule claims to govern, not just the one that was convenient to parse.
Both defects here have the same shape: a rule existed, was correct, and was applied to
a subset of reality that happened to be machine-readable. `format` is a *kind* within
the pipe operator set, so it could never be the name of the set. `traffic` was a YANG
container all along, so `traffic-control` was always two tokens. Nothing new had to be
decided; the existing rules just had to reach the root namespace.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Name the carrier `ze pipe` | `ze view`, `ze out`, `ze render`, `ze fmt`, `ze op` | `pipe` is the word the codebase already uses (`pipe.go`, `ParsePipe`, `knownPipeOps`, `ApplyPipes`, `ai/rules/pipe-completeness.md`, the `commands.md -- CLI pipe operators` anchor), and it reads correctly at a call site that is a literal shell pipe. Chose it over inventing a word because the concept already had a settled in-tree name |
| One carrier for all operator kinds | Split into `ze format <json\|yaml\|table…>` + `ze filter <match\|count…>` | The split gives honest names but breaks one language into two commands, making the common combination two processes. It also has no home for `resolve`/`origin`, which are `display` kind, forcing a third command. One carrier keeps the online and offline operator sets identical, which is the property that justifies the command existing |
| Do not promote operators to root commands | `ze match <pattern>`, `ze count` | Puts generic English verbs beside the YANG verbs (`cmd/ze/ze_core_dispatch.go:555`), inviting permanent collisions |
| `update serve` via the **local handler** registry | Root handler `update` with sub-dispatch; a YANG container + `ze:command` | A root handler named `update` is unreachable: `isYANGVerb` returns at `:321` before root dispatch at `:381`. A YANG container is unnecessary because `RunCommand` checks local handlers first (`cmdutil.go:71`), which is how `help command` / `help ai` already work. Chose the mechanism the codebase already uses for offline-command-under-a-verb |
| Root handler + closed sub-token switch for `traffic`/`isis`/`ospf` | A YANG container per tool | These are offline tools with no config surface; the root registry is where their siblings live (`ze config <sub>`, `ze bgp <sub>` are the same shape). A YANG container would drag them into daemon dispatch for no gain |
| Split `isis-decode` despite the documented deliberate hyphen | Keep it, honoring `register.go:1-9` | The cited collision is not one: the `isis` config root lives under `set`/`delete` and the command tree under `show`/`clear`; the bare root token `isis` is unregistered. The sibling that supposedly collides is what makes `isis` a namespace (R9 step 1). User approved the override on 2026-07-17 |
| Hard cut, no aliases, for all five | One-shot deprecation warnings per `ai/rules/cli-grammar.md:255` | `git tag` returns zero tags: nothing has shipped. `:253` says unreleased grammar is replaced outright; `ai/rules/no-layering.md` forbids keeping both. Negative ACs (AC-2, AC-9) assert the removals |
| Do NOT stage the four in `pendingNamespaceSplit` | Stage them, split later | That list is explicitly for **shipped** commands awaiting migration (`scripts/checks/cli_grammar.go:68-73`) and is currently empty by design. Nothing here is shipped; staging would fake debt and keep the gate green over a hole |
| Extend the gate in the same spec as the renames | Renames now, gate later | Without the gate the next root command reintroduces the defect, and "later" has already happened once: the 2026-07-13 migration cleaned the YANG tree and left the roots because nothing forced the question. The gate is the deliverable; the four renames are its first test case |
| Leave `set cli format` and `\| format tree\|config` alone | Rename them too for a clean sweep of the word | Both genuinely select a representation. They are the reason `format` must be freed, not more instances of the bug |

## Known Limitations

- `--plugins` (`cmd/ze/ze_core_dispatch.go:483`) remains a flag-shaped root command, which reads as an R3 ("no `--flag`") violation in spirit. It is out of scope: it is not a hyphen-namespace defect, and whether root-level flags are legitimate is a distinct question about the root surface's shape. The Phase 6 feeder may surface it; if so, record it and present rather than widening scope unasked.
- `format` still names two different value sets on two surfaces after this spec: `set cli format` accepts `text|table|json|yaml|ndjson` (`internal/component/cli/model_keys.go:669`), while the editor's `| format tree|config` accepts only `tree|config` (`internal/component/cli/model_load.go:951`). Both are legitimate uses of the word, but an operator could reasonably expect one vocabulary. Reconciling them is a question about the editor's rendering vocabulary, deliberately out of scope; this spec only stops `format` from *also* meaning "the pipe language".
- `ze pipe` still cannot chain operators without shell quoting: `runFormat` joins argv into a single pipe expression, so a chain needs the `|` quoted to survive the shell. The rename preserves this rather than changing it. Whether `ze pipe` should accept repeated operators natively is a usability question this spec does not answer.
- The Phase 6 feeder governs the **root** namespace. Local handler paths registered via `MustRegisterLocalMeta` (now including `update serve`) are a third surface, and this spec does not prove they are gated. If the feeder cannot cheaply cover them, record it as a Known Limitation with a destination spec rather than claiming the namespace is now governed (`ai/rules/fail-closed-guards.md`).

## RFC Documentation

N/A - no protocol behavior.

## Implementation Summary

### What Was Implemented
- **`ze format` → `ze pipe`** (carrier rename): `cmd/ze/ze_core_format.go`→`ze_core_pipe.go` (`runFormat`→`runPipe`, `formatUsage`→`pipeUsage`), root key `format`→`pipe` (`ze_core_dispatch.go`), tests renamed (`TestRunPipe`/`TestRunPipeUnknownOperator`), `format-operators.ci`→`pipe-operators.ci` (+ removed-name + completion). Behavior byte-identical.
- **`traffic-control`→`traffic control`, `isis-decode`→`isis decode`, `ospf-decode`→`ospf decode`**: each `cli/register.go` now registers the bare object root (`traffic`/`isis`/`ospf`) with a closed-keyword sub-dispatch (`dispatchTraffic`/`dispatchISIS`/`dispatchOSPF`) routing the single member (`control`/`decode`) to the existing `Run`; bare root and unknown-member are discoverable / rejected (R-6, closed set). Usage strings in `main.go`/`run.go`/`decode.go` updated; the isis header's superseded deliberate-hyphen rationale rewritten. Sub-dispatch unit tests added.
- **`update-serve`→`update serve`** (mechanism change): dropped the root handler; registered `MustRegisterLocalMeta("update serve", runUpdateServe)` beside `help command`/`help ai`. Reached via `isYANGVerb("update")`→`RunCommand`→`matchLocalHandler` (same path as `show version`). `runUpdateServe` unchanged.
- **Root-namespace grammar gate (load-bearing)**: pure fn `grammar.CheckRootNamespace(roots, namespaces)` + fixture red/green `TestRootNamespaceGrammar`; the gate (`scripts/checks/cli_grammar.go`) enumerates roots via AST scan and namespaces = YANG verbs ∪ `container` names, feeds `CheckRootNamespace`, reports "Roots checked". `ai/rules/cli-grammar.md` documents the 4th feeder.
- **codegen:skip** on all three `cli/register.go` (regenerated `all.go`, `all_ze_isis.go`, `all_ze_ospf.go`) to resolve the tool-vs-`ze-test`-suite root collision.
- Caller/doc updates across `docs/`, `internal/`, `test/isis-wire|isis|ospf-wire|ospf/*.ci`, regenerated `ai/` maps.

### Bugs Found/Fixed
- **Root-name collision in the ze-test binary** (traffic/isis/ospf) after the rename — fixed via `codegen:skip` (Deviations, Mistake Log).
- Two pre-existing lint items surfaced on touched groups, fixed incidentally (Deviations).
- validate.py did not treat `scripts/` gates as callers (Review Gate #2).

### Documentation Updates
- `ai/rules/cli-grammar.md` (4th feeder + R9 cross-surface scope); `docs/features.md`, `docs/features/formatting.md`, `docs/guide/command-reference.md`, `docs/guide/command-catalogue.md`, `docs/config-reference.md`, `docs/guide/self-update.md`, `docs/architecture/wire/{isis,ospf}.md` (renames + source anchors `ze_core_format.go`→`ze_core_pipe.go`); regenerated `ai/PACKAGE-MAP.md`, `ai/CODE-TO-DOCS.md`, `ai/DOCS-TO-CODE.md`. `make ze-doc-*` generators stable (`--check` clean).

### Bugs Found/Fixed
- (fill during implementation)

### Documentation Updates
- (fill during implementation)

### Deviations from Plan
- **Files to Modify was missing `scripts/checks/command_ownership.go`.** That gate's `noOwnerAllowlist` (`:39-57`) lists every central `MustRegisterRootHandler` root registered under `cmd/ze`, including `format` and `update-serve`. Because `pipe` is registered centrally via `MustRegisterRootHandler`, `checkRootHandlersAreInternal` REQUIRES `pipe` in that allowlist or `make ze-command-ownership-check` (and `TestNoOwnerAllowlistIsEnforced` in `ze-unit-test`) fails. Fix: rename `format`→`pipe` and drop `update-serve` (now a local handler) in that allowlist. Mechanical necessity for the approved rename, no scope change.
- **`TestRootNamespaceGrammar` moved from `scripts/checks/cli_grammar_test.go` to the `grammar` package.** The spec's TDD plan named `scripts/checks/cli_grammar_test.go`, but every `_test.go` in `scripts/checks/` is `package main` WITHOUT the `//go:build ignore` tag, so it cannot reference symbols from the ignore-tagged `cli_grammar.go` (excluded from `go test` compilation) — those existing tests run the gate as a `go run` subprocess. A fixture-driven red-then-green test needs a pure function, so the feeder lives in `internal/component/command/grammar` (beside `CheckSiblings`) and `TestRootNamespaceGrammar` lives in that package's test file. This matches the grammar package's stated purpose (mechanize the rules as pure functions enforced from one place) and fully satisfies AC-13/AC-14. The `scripts/checks/cli_grammar_test.go` subprocess smoke test (`TestCLIGrammarGateStatic`) still asserts the whole gate is green post-split.
- **Files to Modify doc list was non-exhaustive** (spec anticipated this: "grep the other four" / "verify by grep"). AC-15 grep also surfaces `update-serve` in `docs/config-reference.md`, `docs/guide/command-catalogue.md`, `docs/guide/self-update.md`. Handled in Phase 7 per the spec's grep-driven doc pass.
- **The rename created three root-name COLLISIONS in the `ze-test` binary** (a variant of R-5 the spec did not foresee). `internal/test/cli` registers `ze-test <suite>` roots — including `traffic`, `isis`, `ospf` — via `registerCIRoot`, and it imports `internal/component/plugin/all` (for editor tests). `plugin/all` imported `traffic/cli` (base `all.go`) and, under `ze_isis`/`ze_ospf`, `isis/cli` + `ospf/cli` (gated `all_ze_isis.go`/`all_ze_ospf.go`). While the tool roots were `traffic-control` / `isis-decode` / `ospf-decode` they never clashed with the suite roots; after the rename they became `traffic`/`isis`/`ospf` and panicked on duplicate registration (`bin/ze-test` for traffic; `go test ./internal/test/...` under `ze_isis`/`ze_ospf` for isis/ospf). **Fix:** marked all three `cli/register.go` with `// codegen:skip` (the established mechanism, `scripts/codegen/plugin_imports.go`; precedent: `completion/register.go`) so they drop from `plugin/all` and regenerated `all.go`. `ze` / `ze-appliance` still register the tools through their DIRECT dispatch imports (`ze_core_dispatch.go:52`, `dispatch_isis.go:14`, `dispatch_ospf.go:14`), so `ze traffic` / `ze isis decode` / `ze ospf decode` are unchanged; only `plugin/all` (hence `ze-test`) loses the tool roots. This also makes the three consistent with every other component CLI (firewall/iface/l2tp), which were already dispatch-only, not in `plugin/all`.
- **Two pre-existing lint items surfaced by `ze-lint-changed` on touched package groups**, fixed incidentally (not caused by this spec): `internal/component/l2tp/reactor_kernel_linux_test.go` `behaviour`→`behavior` (misspell, US English), and `internal/plugins/ospf/cli/decode.go` `"area"` hoisted to `const scopeArea` (goconst; the literal is shared with the decode tests).
- **AC-8 `update serve` functional test uses a foreground shell-wrapper + `http=get`** (not `cmd=background`), mirroring `test/ui/web-tool-decode.ci`: a `cmd=background` ze daemon triggers a BGP peer-exchange check the non-BGP HTTP server cannot satisfy, and `http=wait` alone does not make the test self-validated (`isSelfValidated` counts `HTTPChecks`, not `HTTPWaits`). The completion negatives (AC-12) live in their own `test/ui/root-completion.ci` because the `.ci` matcher checks a file's COMBINED output and the removed-name steps emit the hyphenated names.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

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
| The offline carrier is named after the language, not one clause of it | functional test | `test/ui/pipe-operators.ci` drives every operator kind through `ze pipe` |
| No root command hyphenates a namespace | functional test + gate | `test/ui/root-namespace.ci` (AC-7); `make ze-cli-grammar-check` with the root feeder (AC-14) |
| All five old names are gone | functional test + unit test | AC-2 and AC-9 steps; `TestRootsRegistered` negative assertions |
| The root namespace cannot drift again | gate unit test | `TestRootNamespaceGrammar` flags all four on the pre-split fixture and a fresh violation on a new-root fixture (AC-13) |
| The word `format` now means only "choose a representation" | grep + functional test | AC-15 grep; AC-16 proves `set cli format` and `\| format config` still work |
| The operator set is identical online and offline | functional test | AC-6 compares each operator's output against pre-rename behavior |

## Review Gate

### Run 1 (initial)
Pre-checks: `python3 scripts/dev/audit-test-relaxation.py` → 2 `[DELETED]`; `make ze-validate` → 5 `[ISSUE]` unwired-export.

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | NOTE (not a defect) | `[DELETED]` ze_core_format_test.go / format-operators.ci | audit-test-relaxation | RENAMES with replaced coverage: `TestRunFormat`→`TestRunPipe` / `TestRunFormatInvalidOperator`→`TestRunPipeUnknownOperator` (same assertions); `format-operators.ci`→`pipe-operators.ci` (same 15 steps + removed-name + completion). Not weakening. |
| 2 | ISSUE (false positive, FIXED) | `CheckRootNamespace`/`CheckNode`/`CheckSiblings`/`ExemptCategory` "no cross-package non-test caller" | grammar/checker.go | Wired via the `//go:build ignore` gate `scripts/checks/cli_grammar.go`, which `validate.py` did not search. Fixed: added `scripts` to `check_cross_package_wiring` search_dirs; its 26 own tests pass; ze-validate 5→1. |
| 3 | NOTE (pre-existing, advisory) | `WaitFor` "no cross-package non-test caller" | internal/test/runner/runner_exec_util.go:173 | Pre-existing exported method used only intra-package (runner_exec.go:144,781); surfaced only because a comment in the file was edited. `ze-validate` is post-verify advisory, not part of `ze-verify-changed`. Out of scope; left as-is. |

### Fixes applied
- validate.py `check_cross_package_wiring`: `scripts` added to caller-search domains so the grammar gate is recognized as a caller of the grammar helpers (clears the 4 grammar false positives; validate_test.py's 26 tests still pass).

### Run 2+ (re-runs until clean)
Adversarial review (wiring / removed-behavior / logic / security / hot-path) over the full changeset found 0 BLOCKER and 0 ISSUE:
- **Wiring:** `runPipe` (pipe root), `dispatchTraffic`/`dispatchISIS`/`dispatchOSPF` (roots), `update serve` (LookupLocal), `CheckRootNamespace` (gate) each have a production caller AND unit + functional coverage. Proven end-to-end: `ze update serve --listen 127.0.0.1:99998` reaches `runUpdateServe` with the flag intact; the gate flags `traffic-control` when reintroduced then greens when restored.
- **Removed behavior:** `runFormat`→`runPipe` byte-identical (synthetic `"_ | "`, 256 MB limit, exit codes, trailing newline); split tools' `Run` unchanged; `runUpdateServe` unchanged.
- **Logic:** `CheckRootNamespace` mirrors `CheckSiblings` (leading-hyphen `--plugins` safe via `b>0`); sub-dispatch is a closed keyword set that fails closed (unknown token never reaches `Run`).
- **Security:** stdin 256 MB limit preserved; `update` is a mutation verb (not read-only, `IsReadOnlyVerb`); no unbounded allocation; error text names only the token.

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE — MET (Run 2+ adversarial pass clean; ze-validate reduced to 1 pre-existing advisory NOTE outside the commit gate)
- [ ] All NOTEs recorded above (finding #1 rename, #3 WaitFor pre-existing)

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
- [ ] AC-1..AC-16 all demonstrated
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
- [ ] Write learned summary to `plan/learned/NNN-cli-root-namespace-grammar.md`
- [ ] **Commit A:** code + tests + docs + spec (with all edits) + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-cli-root-namespace-grammar.md` only

