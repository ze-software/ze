# Spec: le-cli-usability

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Priority | P3 |
| Release reason | Developer tooling only: `le` is never shipped (`TestNormalZeLinksNoInternalLe`), so its usability is release-independent engineering work |
| Depends | - |
| Phase | 2/7 |
| Handoff | - |
| Updated | 2026-10-25 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`le` is the entry point every developer and every agent session uses, and today
it is a CLI a person cannot drive without reading source. It has no shell
completion, its help comes in three page formats and is wrong at several
levels, a usage error dumps a whole command family, 30 areas hand-roll their
own argument parsers that publish no grammar, and nothing answers "what does
le hold" in the shape `ze help ai` answers it for `ze`.

The goal: `le` becomes a CLI a human developer finds usable, matching `ze`.

### Thomas's decisions (the agreed scope)

| # | Decision |
|---|----------|
| D-1 | `le` gets what `ze` has: shell completion for bash, zsh and fish at least, using ze's mechanism (a static script that calls back into the binary for live words); correct and complete help at every level; interactive filling of missing arguments when stdin is a terminal; and self-documentation through `le help ai` and `le help ai --json`, with `--ai` accepted as a flag, in the same shape as `ze help ai` |
| D-2 | ONE spec. It absorbs `spec-le-every-area-dispatches-through-one-table` (every area on the action table, typed grammar published) and `spec-le-one-verb-one-job` (one verb per job, the write/update ruling, exit-code meanings). Every task, finding and acceptance criterion of both is carried here, re-measured against today's tree |
| D-3 | Renames are in scope: `le spec claim spec <file>` becomes `le spec claim <file>`, write/update merge per the recorded ruling, mocks and spawned helpers move out of `test`, noun-verb consistency across areas. Old spellings are removed, never aliased (`ai/rules/no-layering.md`). Every rule, skill, doc, hook and test naming an old spelling is updated in the same change |
| D-4 | `le help ai` becomes the reference. The `le ai` namespace page moves to `le ai help` |
| D-5 | Root help is laid out by command tree, one block per namespace, and each command's purpose (workflow, gate, generated, suite, report) is a tag on its line rather than a section heading |
| D-6 | Parameters become typed: a kind (switch, string, enum, uint, path, revision, spec file, and so on), enum values, a per-keyword description, and a value source for completion. Reuse of `internal/component/command.ArgDef`, `command.Node` and `command.TreeCompleter` is evaluated before any parallel model; ze's completion scripts, TUI launcher and AI-help data layer are evaluated for sharing. One shared implementation is preferred over a copy (`ai/rules/principles.md`, every fact declared once) |

## Superseded specs (removed in this spec's first commit)

These two skeletons are absorbed whole. They are not edited by the research
phase. The first implementation commit of this spec removes both files, and
every citation of either is repointed to this spec in that same commit.

| Spec | What this spec carries from it |
|------|-------------------------------|
| `plan/post-release/engineering/spec-le-every-area-dispatches-through-one-table.md` | Every area dispatches through `leaction.Area`; the migration list in `internal/le/actions_test.go` empties and its exemption branch is deleted; `RegisterActions` becomes mandatory; the conversion inventory (no positional values, structured payloads, flattened sub-verbs, the bare-form verb conflict of `spec status`); the lesson that declaring a table is not the deliverable, dispatching through it is; R-1 (migrate `commit` last, byte-identical exit codes until the exit-code phase), R-2 (renames break citations); its ACs AC-1 to AC-5 and wiring rows, recorded below as carried criteria |
| `plan/post-release/engineering/spec-le-one-verb-one-job.md` | One verb per job (`update` wins over `write` for regenerating a committed artifact, owner ruling, no alias); one meaning per exit code; help that fits a terminal (wrap the description column at render time); `RefuseArgument` and `refuseValue` answer the same code; its ACs AC-1 to AC-7, A-1 to A-3, R-1 to R-3 and wiring rows, recorded below as carried criteria |

### Carried acceptance criteria (to be restated at the DESIGN gate)

The DESIGN phase restates each in final form. None may be dropped.
Restated as: one-table AC-1 as AC-6, AC-2 as AC-14, AC-3 as AC-15, AC-4 as
AC-16, AC-5 as AC-17, wiring as AC-18; one-verb AC-1 and AC-2 as AC-21, AC-3 as
AC-25, AC-4 as AC-28, AC-5 as AC-29, AC-6 as AC-10, AC-7 as AC-30. The one-verb
skeleton's A-1 (rename with no alias, owner-confirmed) is D-3 and A-12, its A-2
(every caller findable) is A-3, its A-3 (no caller branches on 2) is A-6; its
R-1 to R-3 are R-2, R-3 and R-4. The one-table skeleton's R-1 and R-2 are R-1
and R-2.

| From | Carried criterion | Re-measured today |
|------|-------------------|-------------------|
| one-table AC-1 | `le commit create --help` renders every live keyword, marking `subject` required and `body`, `file`, `file-list`, `remove`, `remove-list` repeatable, exit 0 | `createKeywords` holds 23 entries: 20 live plus 3 retired-with-replacement (`push`, `unverified`, `missing-full-verify-ok`). The skeleton's "21" is stale |
| one-table AC-2 | Every formerly hand-rolled area, bare, answers its action listing in the shape every other area answers | 30 areas, not 7 |
| one-table AC-3 | `le '|' json` names every registered area's actions, none exempt | 164 areas: 78 tabled, 56 forwarding, 30 hand-rolled |
| one-table AC-4 | The migration list is empty and the exemption branch is deleted | `areasWithoutAnActionTable` has 30 entries |
| one-table AC-5 | Exit codes stay unchanged through the table migration; exit-code discipline lands in its own phase | unchanged |
| one-table wiring | `le test stress-repro run --help` renders the run grammar from the registered table; `le '|' json` carries every area with no exemption | `test stress-repro` is still on the migration list |
| one-verb AC-1 | Any action regenerating a committed artifact from its source is spelled `update`; none registers `write` for that job | `write` is now a verb literal in 11 packages, `update` in 8 (skeleton said 5 and 7); each `write` row must be judged against the job, since `verify status write` and `ai sync write` may not be regenerations |
| one-verb AC-2 | The old `write` spellings are refused as unknown actions, no alias | unchanged |
| one-verb AC-3 | Every `le` invocation cited in the instruction corpus and in Go string literals resolves against the manifest, verb included | see Consumer surface |
| one-verb AC-4 | A usage error in any area answers one code, distinct from a gate that ran and found a defect | still split: see Current Behavior |
| one-verb AC-5 | An environment failure answers a code distinct from both | `rfc check` answers 2 for both "cannot run" and real violations |
| one-verb AC-6 | A `Why` longer than the terminal wraps under the description column, the name column aligned | unchanged; 75 of 440 single-line `Why` literal lines exceed about 120 characters, and many `Why` texts are concatenated across lines, so the longest rendered text is not measurable statically |
| one-verb AC-7 | `leroot.RefuseArgument` and `leaction.refuseValue` answer the same code for the same mistake | still 1 against 2 |

## Required Reading

<!-- NEVER tick [ ] to [x]. -->

### Architecture Docs
- [ ] `ai/doors/le-tooling.md` - the door for `internal/le`
  → Constraint: a helper both `ze` and `le` need MUST live outside `internal/le`; `TestNormalZeLinksNoInternalLe` goes red otherwise. Any shared completion, help-ai or menu library therefore lives under `internal/component/command`, `internal/core`, or a new non-le package
  → Constraint: every hook in `.claude/settings.json` (14 calls) runs `le ai hooks <name>`; a rename under `le ai` stops every live session at once, so `ai hooks` names are frozen unless the settings file changes in the same commit
  → Constraint: a before-dispatch hook never prints, never waits; completion callbacks go through the same `run`, so they also trigger `RunBeforeDispatch` (the scratch store's hourly trim chance)
- [ ] `docs/architecture/core-design.md` - "Command engine and composition roots", le's composition, groups, namespaces, retired names, help at every depth
  → Decision: le's package sits at the path its command name predicts (`TestEveryCommandIsFoundAtThePathItsNamePredicts`), so every rename of an area moves its directory
  → Decision: a bare namespace token answers its page with exit 0; a non-member word after it exits 1; an unknown first word exits 1
  → Decision: a retired name is never registered; `internal/le/doc/check/retirednames.go` declares the rename map once (126 rows today) and `le doc check retired-commands` finds every tracked line naming an old form. This is the consumer-surface machine for D-3's renames: every rename adds rows there
  → Constraint: help answers from the registry and never calls the handler, because a single-action area runs its gate on a bare invocation. Completion and interactive filling must obey the same rule: neither may call a handler to learn the grammar
- [ ] `docs/architecture/cli/root-namespace-grammar.md` - the R1-R9 grammar and the seven feeders of `./le cli grammar`, feeder 6 being le's surface
  → Constraint: keyword before value; `le spec claim <file>` puts an untyped value in the first slot after the action, which the R-rules may refuse unless the action's grammar types it. The DESIGN phase checks the rename against `cli grammar`
  → Constraint: a rendering flag a command reads by hand is invisible to feeder F3 (the page's "Coverage boundary" names `ze pipe help --json`); `le help ai --json` would be a second such case
- [ ] `docs/architecture/cli/command-verbs.md` - ze's operator verb taxonomy
  → Decision: the page governs the operator CLI only and says nothing about le's verb vocabulary (`check`, `update`, `list`, `run`). The verb set for le has to be stated somewhere, and this page or `running-commands.md` gains a section for it
- [ ] `docs/architecture/cli/tui-launcher.md` - ze's no-argument menu
  → Constraint: outside a terminal, the bare command keeps printing the static help, so scripts and pipes see the same bytes. Any le menu or prompt obeys the same rule
  → Constraint: every level is derived from the registry, never hard-coded; pad before styling
- [ ] `docs/contributing/running-commands.md` - "Finding a command and what it takes"
  → Constraint: documents bare `./le` on stdout exit 0, `<area> <verb> --help` from the table without a handler call, the four help spellings, and that a table-less area hands any other option to its own parser. Every one of these sentences changes in this spec
- [ ] `docs/guide/command-reference.md` - `ze completion`
  → Constraint: shell completion is registered by command name, so `./bin/ze` completes only when reachable as `ze` on PATH. Agents and developers type `./le`, a path; bash and zsh register completion by command word, so the install line must cover `./le` explicitly or the feature does nothing for the common spelling
- [ ] `docs/guide/mcp/overview.md` - `ze help ai` sections (`cli`, `api`, `mcp`, `dispatch`, `all`) and `--json`
  → Decision: `ze help ai` is generated from the binary's registries; `le help ai` follows the same "generated from code, always matches this binary" contract
- [ ] `ai/patterns/cli-command.md` - the CLI command pattern (feature type gate)
  → Constraint: "How `help ai` consumes the registry" names `cmd/ze/help_ai.go:cliSubcommands()`, which no longer exists (it is `aihelp.CLISubcommands`); the page is stale and is corrected in the phase that touches help ai
- [ ] `ai/rules/cli.md` - binding rule for any CLI change
  → Constraint: "a rendering flag MUST NOT exist on any command"; only `--version`, `-V`, `--help` and `-h` are exempt. `le help ai --json` and `--ai` conflict with this as written (see Design questions, Q-1)
  → Constraint: "a second spelling MUST be renamed outright rather than aliased"; `ze` keeps `--ai` as a hidden alias of `help ai`, so copying it into le copies an alias
  → Constraint: "Every user-facing command MUST have tab-completion"; le has none today, so this spec closes a standing violation
  → Constraint: GNU option syntax binds le; a dash-leading token in a value slot is refused by name
- [ ] `ai/rules/no-layering.md`
  → Constraint: every renamed spelling and every replaced parser is deleted, then the replacement written; no hybrid period in which both `commandVerbs` and a table exist for `commit`
- [ ] `ai/rules/principles.md`
  → Constraint: the parameter type, its description and its value source are declared once, in the action table, and help, completion, prompting, the manifest and `help ai` all derive from it

**Key insights:** (minimal context to resume after compaction)
- `bin/le` IS `cmd/ze` built with tags `ze_le` plus the feature tags, not `ze_core`. The personality is chosen by binary name (`defaultDispatch`, `cmd/ze/dispatch.go`). So `cmd/ze/tui_menu.go` and `cmd/ze/help_ai.go` (both `ze_core`) are not in `bin/le`, while `internal/plugins/completion` and `internal/component/aihelp` are linked (through `plugin/all`, which `le cli list` imports)
- `cmd/ze/internal/helpfmt` is only a shim over `internal/core/helpfmt`, which le already uses; the renderer is shared already
- le's commands already live in the shared local-data registry under the path prefix `le ` (`command.MustRegisterLocalData`), so a `command.Node` tree for le can be built from `command.ListLocal()` plus the action tables, and `command.TreeCompleter` can complete it
- `command.Node` already carries `ArgDefs`, `DynamicChildren` and `ValueHints`, so a dynamic value source exists at node level; `ArgDef` has kinds string, enum, uint, union, flag, a mandatory and a rest-of-line marker, short help and description, but no repeat, no pair, and no retired-keyword replacement, all of which le uses
- ze's shell scripts are string literals with `ze` hard-coded (bash 55, zsh 32, fish 94 occurrences); the callback record shape (candidate, tab, summary, newline, `writeCompletionRecord`) is program-neutral and reusable
- No missing-argument prompt exists anywhere in ze today: ze's TUI launcher is a navigation menu, not an argument filler. D-1's interactive filling is new behaviour

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/le/action/leaction.go` (968L) - `Parameter` has exactly `Keyword`, `Value` (a placeholder name, untyped), `Requirement`, `Repeat`. `Action`: `Verb`, `Why`, `Writes`, `Alone`, `WholeTree`, `Parameters`, `Answer`, `AnswerArgs`, `AnswerWords`. `List.Text` pads the verb column and appends `writes`/`checks` and the whole `Why` with no width limit. `refuseVerb` and `refuseValue` answer 2; `refuseValue` hard-codes `usage: le ` even in a `ze le` build
- [ ] `internal/le/le/root/leroot.go` (422L) - `Register(name, group, answer, meta)` panics on a missing group or Meta field; `RegisterActions` optional; `RegisterForwarding`; `RefuseArgument` answers 1 for the mistake `refuseValue` answers 2 for
- [ ] `internal/le/le/root/dispatch.go` (350L) - bare `le` answers the manifest through `Run` on stdout, exit 0; `le help` and `le --help` call `Usage`, which writes the same page to stderr, exit 0. `helpNode` splits `Meta.ResolveSubs()` on " | " into entries with a name and no description, and writes to stderr. So `le spec claim --help` lists `spec <spec>` as an action (its Subs is `spec <spec>`), `le job run --help` lists the whole `usage: le job run label <label> [quiet] command <argv...>` line as one action, and `le commit create --help` (commit has no table) prints commit's node page with bare verb names. `commandWordsMax` is 2. A namespace followed by a non-member exits 1
- [ ] `internal/le/le/root/group.go` (115L) - five groups rendered as section headings in a fixed order; group is a `Register` parameter. `spec` members fall under four headings (workflow: claim, current, release, review, state, wip, model, baseline; gate: citation; report: journal, status; generate: roadmap); the prior "five headings" was not reproduced statically
- [ ] `internal/le/le/root/manifest.go` (143L) - `Manifest{Program, Summary, Areas[]{Name, Group, Description, Actions[]leaction.Row, Forwards}}`; `page()` builds one section per group. `le '|' json` is the nearest thing to `help ai` today
- [ ] `internal/le/actions_test.go` - `areasWithoutAnActionTable` lists 30 areas: `ai digest`, `ai tokens`, `arch iface-resolution`, `build gosum`, `build host-driver`, `cli grammar`, `cli list`, `cli ownership`, `commit`, `config claims`, `doc check`, `doc consistency`, `doc yang-contract`, `go extract`, `job`, `repo inventory`, `repo bootstraps`, `repo working-tree`, `spec citation`, `spec claim`, `spec current`, `spec model`, `spec release`, `spec review`, `spec state`, `spec status`, `spec wip`, `test stress-repro`, `verify summary`, `weekly`. `TestEveryPublishedVerbIsDispatchedThroughItsOwnTable` guards table/dispatch agreement
- [ ] registration census (grep of `leroot.Register(` outside tests, one string constant in `repo/bootstraps/tracked.go` excluded) - 164 areas; 78 files call `RegisterActions`; 56 call `RegisterForwarding`; 78 + 56 + 30 = 164, consistent with the prior count. The prior "321 actions" was not re-derived: 231 `Verb:` literals exist statically, the rest are built in loops
- [ ] `internal/le/commit/actions.go` - its own `commandVerbs` (session, create, message, audit, review-check, push, debt-status and more), its own `parseKeywords` over `createKeywords` (a `keywordRule` with `Value`, `Repeat`, `Replacement`, `Pair`, `Usage`, `Given`, `WithAttest`), its own `commandError`; `commandList.Text` prints verb, mark and `Why` separated by two spaces with no padding, so its columns do not align, unlike `leaction.List.Text`
- [ ] `internal/le/spec/actions.go` - `answerState` hand-matches `current` and `latest spec <stem>`; anything else calls `refuseLifecycle`, which prints the whole spec family as one brace-grammar line (claim, current, release, wip, state, model, review) and answers 2. The skeleton's `spec session` area no longer exists: it is now the `spec` namespace's members
- [ ] `internal/le/verify/register.go` - the `verify` area's ShortHelp reads "the full native verification population against a fixed commit in a detached worktree", which describes `verify worktree`; the area holds `current`, `changed`, `reds`, `list` and more
- [ ] `internal/le/cli/list/commandlist.go` - `le cli list` blank-imports `plugin/all` and inventories ze's commands by verb, not le's
- [ ] `internal/le/mrt/mrt.go` - bare `le mrt` answers `analyze.Targets()` as data, rendered by the table renderer; the prior "columns reversed" finding was not re-checked against rendered output
- [ ] `internal/le/register.go` - `run` calls `ensureCacheLink` (may print to stderr), `activatePatchedGOROOT`, `RunBeforeDispatch`, then `Dispatch`
- [ ] `le` (launcher script) - builds `bin/le` from `./cmd/ze` with tags `ze_le,<feature tags>` when missing or on `--update`; on one run in sixteen (`le_sampled`) it runs `warn_when_stale` AFTER the binary, which walks sources with `find` and `git status` and may print `le: bin/le is older than committed sources; refresh it with './le --update'` to stderr; cache-link repairs also print to stderr
- [ ] `internal/component/command/argdef.go`, `node.go`, `completer.go`, `help.go` - `ArgDef` built only by constructors, private fields, kinds `ArgString`, `ArgEnum`, `ArgUint`, `ArgUnion`, `ArgFlag`; `Node{Name, ShortHelp, Description, Children, ArgDefs, DynamicChildren, ValueHints, Modifier, ...}`; `TreeCompleter.Complete(input)` matches children, dynamic children, value hints and ArgDef names; `HelpEntries` and `FindNode` serve the ze TUI
- [ ] `internal/component/command/local_data.go` - `ArgDefSource` is installed by the YANG package; le paths declare no YANG, so le commands carry no ArgDefs today
- [ ] `internal/component/command/registry/registry.go` - `Meta{ShortHelp, Description, Mode, Section, Subs, SubsFunc}`; `Subs` is free text
- [ ] `internal/plugins/completion/main.go`, `bash.go`, `zsh.go`, `fish.go`, `words.go`, `flags.go` - `ze completion <shell>` prints a static script; `ze completion words <path...>` builds a Node tree and prints records through `writeCompletionRecord`; `flags`, `families`, `peers` are other callbacks. Scripts hard-code `ze`
- [ ] `internal/component/aihelp/aihelp.go` - `Build()` assembles `Reference` (commands, RPCs, plugins, families, services) from ze's YANG and registries; `CLICommand{Name, Mode, ShortHelp, Subs}` is the only program-neutral piece
- [ ] `cmd/ze/help_ai.go` (`ze_core`) - text sections by positional word (`cli`, `api`, `mcp`, `dispatch`, `all`), `--json` read with `slices.Contains`, `--ai` a hidden alias; renders through `cmd/ze/internal/helpfmt`, a shim over `internal/core/helpfmt`
- [ ] `cmd/ze/tui_menu.go` (`ze_core`, 464L) - a bubbletea navigation menu over `command.HelpEntries`; `stdinIsTerminal` via `golang.org/x/term`
- [ ] `internal/le/le/human/human.go` - le already asks whether stdin and stdout are terminals (`IsTerminal`), for the whole-tree verify gate

**Behavior to preserve:**
- `le '|' json`, `| yaml`, `| table` render one structured payload; every area keeps answering structured data
- A help word never reaches a handler; a value spelled `help` that a declared keyword introduced stays a value
- Forwarding areas (`mrt`, `chaos run`, `test harness`, `build gokrazy`, perf forwarding rows) hand help words to the program
- Bare `le` outside a terminal keeps printing the root page on stdout, exit 0; no prompt and no menu ever appears when stdin or stdout is not a terminal (agents run on pipes)
- `le ai hooks <name>` spellings, unless `.claude/settings.json` changes in the same commit
- The whole-tree human gate in `internal/le/le/human` is untouched by prompting: a prompt never answers that gate's question
- The group of every area stays declared at registration, and the group tests in `internal/le/group_test.go` keep their meaning; only its rendering moves from heading to tag
- Two-word maximum area path (`TestNoRegisteredLeCommandExceedsTwoWords`) and the path-predicts-package rule

**Behavior to change:** (Thomas's D-1 to D-6, and the carried criteria)
- Shell completion for le (none today)
- One page format at every level, with correct entries (helpNode's free-text split goes), each action's typed keywords and descriptions, and help on stdout when asked
- Usage errors name the one action's grammar, never the family
- Root page by namespace tree with purpose tags; long `Why` wrapped
- `le help ai` and its JSON form; `le ai` namespace page reached as `le ai help`
- Every area on the action table; typed parameters; renames; one exit-code meaning

### Consumer surface (how to find every old spelling)

| Source | Measured today | How it is found |
|--------|----------------|-----------------|
| Rename map | 126 rows in `internal/le/doc/check/retirednames.go` | Every rename adds its row; `le doc check retired-commands` then names file and line of every tracked old form, including Go argv literals (`runLE(nil, ...)`) |
| Hooks | 14 `le ai hooks` calls in `.claude/settings.json` | grep of `.claude/settings.json` and `.claude/hooks/README.md` |
| Agent instructions | `le ` citations in 237 files under `ai/`, 17 under `.claude/`, plus `.agents/skills` and `.codex/skills` | grep, and the planned citation resolver (`TestNoCitationNamesAVerbTheRegistryDoesNotHold`, carried from one-verb) |
| Docs | 392 files under `docs/` | grep plus `le doc check verify` |
| Specs and journal | 821 files under `plan/` | grep; live specs only, history stays |
| Functional tests | 1613 files under `test/` cite `le test <name>`: `le test fixture` 1398 times, `le test peer` 975, `le test rtr-mock` 60, `le test engine-steps` 31, `le test plugin-external` 22, `le test irr` 19, others fewer | grep over `test/`; these are the mocks and spawned helpers D-3 moves out of `test` |
| Go code | 5581 files under `internal/` and 102 under `cmd/` mention `le <word>` | `retired-commands` covers argv literals; the rest by grep per renamed word |
| CI | 6 files under `.github/` | grep; the door names `test qemu`, `test integration`, `build host-driver`, `setup docker-kernel` |
| `spec claim spec` alone | 15 files | grep |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- argv reaching `cmd/ze` main under the `le` personality (binary name `le`), through the `le` launcher script or `bin/le` directly; for completion, the shell's callback invocation of the same binary
- Format at entry: argv words; `|` splits the operator chain

### Transformation Path
1. Launcher: build if missing, exec `bin/le` (one run in sixteen also runs the stale check after the binary exits)
2. `defaultDispatch` picks the `le` root handler by binary name
3. `internal/le/register.go` `run`: cache link, patched GOROOT, before-dispatch hooks, `leroot.Dispatch`
4. `Dispatch`: bare root, help word, `resolve` (longest registered prefix, at most two words, through `command.LookupLocalData`), trailing-help guard (`asksForUsage`), namespace page, unknown command
5. Area handler: `leaction.Area.Answer` (table) or a hand-rolled parser (30 areas) or a forwarded program (56 areas)
6. `leroot.Run` renders the payload through the shared pipe layer

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Shell to le (completion) | a static script calls back into the binary; records are candidate, tab, summary | No (no le completion exists) |
| Dispatcher to area | `LocalDataHandler` over judged tokens; table areas through `leaction` | Yes, read `dispatch.go`, `leroot.go` |
| le to ze shared code | `internal/component/command`, `internal/core/helpfmt`, `internal/plugins/completion`, `internal/component/aihelp` | Yes, read; directional rule from the door |
| Area to caller | process exit status read by scripts, hooks and CI | Partly: readers enumerated at DESIGN (A-6); per-test judgement in Phase 5 |

### Integration Points
- `command.ListLocal()` and `leroot.ActionsOf` - the source a le `Node` tree is built from
- `command.TreeCompleter` - the completer the callback runs
- `internal/core/helpfmt.Page` - the one page renderer
- `leroot.Manifest` - the payload `le help ai --json` grows from
- `retirednames.go` - the rename map

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes (by design) | dispatch keeps one path (`leroot.Dispatch` to `leaction.Area.Answer`); completion, menu, prompting and `help ai` read the derived tree and never call a handler (core-design, "help answers from the registry") |
| No unintended coupling (components stay isolated) | Yes (by design) | shared code lands in `internal/plugins/completion`, `internal/component/cli/menu` and `internal/component/aihelp/reference`, none of which imports `internal/le`; `TestNormalZeLinksNoInternalLe` stays green |
| No duplicated functionality (extends existing, does not recreate) | Yes (by design) | one completion generator, one menu model, one reference renderer, one parameter declaration (`leaction.Parameter`); the derived `command.Node` tree is a projection, never a second declaration |
| Zero-copy preserved where applicable (refs, not copies) | N-A | no wire path; the derived tree is built once per process, on demand |
| Registration over hardcoding, outbound | Yes (by design) | every area keeps registering through `leroot.Register` plus `RegisterActions`, which becomes mandatory; the tree, help, completion, menu, prompt and `help ai` derive from that registration |
| Registration over hardcoding, inbound | Yes (by design) | lists searched and their fate: ze's completion scripts' `ze` literal (becomes the program descriptor's name), `helpNode`'s Subs split (deleted, pages render from the table), `refuseLifecycle`'s family line (deleted, usage names one action), commit's `commandVerbs` and `parseKeywords` (deleted, commit goes on the table), the `GroupTitle` switch (becomes one declared group table with title and tag), `areasWithoutAnActionTable` (emptied and deleted) |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | No area of the 30 hand-rolled ones takes a positional value | one-table skeleton inventory of 7 areas, 2026-09-12; the 23 others are unmeasured | The flat keyword table cannot describe them, or D-3 renames like `spec claim <file>` introduce positionals the table lacks | read each of the 30 parsers at the start of Phase 3, before its conversion; DESIGN read only `commit` and `spec`. A positional found is declared with the positional rule of D-Q3 or turned into a keyword | unvalidated (Phase 3 entry) |
| A-2 | Every payload of the 30 is already structured data | one-table inventory for 7; `refuseLifecycle` and commit's `Prepared.Text` show text renderers beside the data | `leroot.Run` would need changes | read each Answer at the start of Phase 3; a text-only renderer found keeps its data payload and gains a `Text` method, as `leaction.List` has | unvalidated (Phase 3 entry) |
| A-3 | Agents' instructions depend on today's spellings, and every dependent is findable | 237 `ai/` files, 17 `.claude/`, skills mirrors, 1613 `.ci` files, 14 hook calls | A rename strands an agent or a hook mid-session, and every concurrent session fails | `le doc check retired-commands` plus the citation resolver, run before and after each rename commit | unvalidated (Phase 4, per rename) |
| A-4 | Building a completion tree from the registry is fast enough for a keystroke | le already runs every `init()` including `plugin/all` on every call; no measurement taken | completion lags or times out in the shell | DESIGN measured `bin/le spec current` and `bin/le help` warm on macOS (arm64): 62 to 69 ms wall over six runs, every `init()` included. That leaves about 30 ms for the tree build and completion against the 100 ms budget of AC-21. Linux is unmeasured; `TestCompletionCallbackAnswersWithinBudget` measures both where it runs | partly validated (macOS baseline) |
| A-5 | Hand-rolled parsers like commit's fit the action table | commit's `keywordRule` carries `Pair`, `Replacement` (retired keyword naming its successor), `Given` and `WithAttest`, none of which `leaction.Parameter` or `command.ArgDef` has | The table must grow these features, or commit keeps a parser (forbidden by D-2 and no-layering) | DESIGN mapped every field: `Value` to the parameter kind (switch against valued), `Repeat` to `Repeat`, `Replacement` to the new `Retired` field, `Pair` and `Usage` to the pair arity with a second value name (usage derived, never written), `Given` and `WithAttest` stay in `commit` as its attestation policy keyed by keyword, with a test that every live declared keyword has exactly one entry (today's `TestEveryCreateKeywordReadsOptions` shape) | validated (DESIGN read `internal/le/commit/actions.go` L236-256) |
| A-6 | No caller branches on a specific le exit value in a way a split breaks | carried one-verb A-3; hooks and the commit script read exit codes | A red gate turns green silently | DESIGN grep: `.github/workflows` run `./le ...` steps and read only zero against non-zero; the two Go readers branching on exit 1 (`internal/le/commit/owed.go` `isAncestor`, `internal/le/rfc/rename.go`) read git's code, not le's; `internal/le/test/weakened/selftest.go` asserts its own codes 1 and 2; 231 `_test.go` files under `internal/le` mention exit codes and are judged per producer in Phase 5; `.ci` files asserting `expect=exit:code=` on `le test fixture` read the fixture's code, which passes through (D-Q8 exemption); `le ai hooks` answers the hook protocol (0 allow, 2 block) and is exempt. Phase 5 re-runs the grep against HEAD before changing any producer | partly validated (readers enumerated; per-test judgement in Phase 5) |
| A-7 | The launcher's stderr output does not corrupt completion | the stale check and cache-link messages go to stderr, and completion reads stdout only | bash and zsh still paint stderr over the prompt line; a missing binary makes a keystroke start a full build | decided (D-Q7): the generated script calls the binary beside the launcher (`<dir>/bin/le`) directly, discards stderr, and the `completion` area skips cache-link repair and before-dispatch hooks; `TestCompletionCallbackRunsNoBeforeDispatchHook` and the pty test AC-20 prove it | validated by design, proven in Phase 6 |
| A-8 | `command.ArgDef` can carry le's parameter kinds without weakening its YANG-derived invariants | ArgDef has 5 kinds, private fields, constructors; no path, revision or spec-file kind, no repeat | A parallel model is needed, or ArgDef grows kinds YANG never produces | broken as stated, and not needed (D-Q3): ArgDef is not grown. The derived `Node` maps switch to `ArgFlag`, enum to `ArgEnum`, uint to `ArgUint`, every other kind to `ArgString` with its value source as `ValueHints` or `DynamicChildren`. Repeat, pair and retired stay on the Parameter and are honoured by the le completion callback, never by ArgDef | resolved at DESIGN |
| A-9 | The completion scripts can be parameterised by program name without changing ze's behaviour | scripts are literals with `ze` 32 to 94 times each | Two copies of each script (forbidden by principles) | broken in part: DESIGN read `bash.go` and found ze-only branches (global-flag skipping `_ze_find_subcmd`, `families`, config-file sections, `flags`). So the generator takes a program descriptor: a shared skeleton (registration, the `words` callback loop) parameterised by name, plus a per-program fragment that ze supplies with today's branches and le leaves empty. `TestZeCompletionScriptsUnchanged` pins ze's three scripts byte-for-byte against goldens captured before the refactor | resolved at DESIGN, proven in Phase 6 |
| A-10 | `--json` and `--ai` on `le help ai` are permissible | Thomas's D-1 | `ai/rules/cli.md` bans rendering flags and aliases as written | broken: decided D-Q1, no flag and no alias on `le help ai`; JSON and YAML come through the pipe layer | resolved at DESIGN |
| A-11 | Interactive filling never fires for an agent | agents reach le on pipes (`internal/le/le/human` relies on the same fact) | an agent's tool call hangs on a prompt | require stdin AND stdout terminals (D-Q4, D-Q5), and test the pipe case: `TestBareLeOnAPipePrintsTheRootPage`, `TestMissingRequiredParameterOnAPipeRefusesWithOneUsage` | validated by design, proven in Phases 2 and 7 |
| A-12 | The five `write` areas named by the owner's ruling are all regenerations, and the other six `write` packages found today are judged individually | one-verb skeleton ruling; re-measure found 11 | a non-regeneration action renamed to `update` misdescribes its job | DESIGN read every `write` action's `Why` (11 packages): ten regenerate or refetch a committed artifact and become `update` (`yang glue`, `web assets`, `plugin gated-table`, `plugin imports`, `repo feature-tags`, `data asn-delegation`, `data ip-special-purpose`, `data ietf-reference`, `ai sync`, `doc index`); `verify status write` records a verdict and becomes `verify status record`, matching `spec review record`; `worktree update` rebases a worktree and becomes `worktree rebase`, so `update` keeps one job. Exact area spellings are confirmed against the manifest in Phase 4 | validated (job judged per action) |
| A-13 | `bash`, `zsh` and `fish` are installed on every developer host that runs the completion tests | DESIGN found all three under `/opt/homebrew/bin` on the macOS host | a shell-driven test cannot run there | `TestCompletionScriptsInRealShells` refuses with the missing shell named and the install command, never skips; the Linux host is checked when Phase 6 runs there | partly validated (macOS) |
| A-14 | The existing pty pattern serves the menu and prompt tests | `internal/test/fixture/ui_fixture_display_fill_completion.go` opens a pty with `github.com/creack/pty` (`pty.Open`, `pty.Setsize`) inside a registered compiled fixture driven by a `.ci` file; `internal/le/site/terminaldemo/pty.go` has `startPTY`, `readUntilPTY` | a new pty helper would be a second copy | the Phase 6 and 7 fixtures reuse that pattern; any helper they share is lifted into one place, not copied | validated (pattern exists) |
| A-15 | Prompting with Tab completion needs raw terminal input, which the menu library's bubbletea dependency already gives | `cmd/ze/tui_menu.go` imports `charm.land/bubbletea/v2`; `bin/le` already links bubbletea through `plugin/all` | `bufio` line reading cannot complete on Tab | the prompt is a one-field model in `internal/component/cli/menu`; `TestPromptCompletesFromTheValueSource` | validated by design |
| A-16 | No area named `completion` or `mock` exists under le today | DESIGN read `bin/le` root page: neither word is registered | a name collision at registration | `leroot.Register` panics on a duplicate; the Phase 1 wiring test registers the derived tree with both | validated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | `commit` regresses and no session can commit (carried) | a commit script fails to generate | migrate `commit` last among parsers, exit codes byte-identical until the exit-code phase |
| R-2 | A rename breaks a citation in hooks, CI or instructions (carried) | an agent runs a verb that exits 1 | rename map row plus `retired-commands` and the citation resolver in the same commit |
| R-3 | Splitting exit 2 turns a red gate green (carried) | a workflow passes where it failed | enumerate exit-status readers before changing any producer |
| R-4 | Wrapping help breaks golden tests (carried) | a wave of golden failures | wrap at render time only; update goldens in the same change |
| R-5 | Renaming `le test fixture` and `le test peer` touches about 2,370 lines in `.ci` files at once | huge diffs; concurrent sessions editing `.ci` files conflict | one mechanical commit per moved helper, generated by the rewrite tool, scoped to files citing it |
| R-6 | Completion callback triggers before-dispatch work (store trim) or a rebuild on every keystroke | slow tab, disk activity | a callback path that skips hooks and the launcher; measure (A-4) |
| R-7 | `le help ai` collides with the general rule that `le help <name>` prints that node's page, since `ai` is a namespace | `le help <x>` loses uniformity | DESIGN states the reserved word explicitly and tests both forms |
| R-8 | This spec is large: 30 parser migrations, renames, completion, help, prompting, help ai | phases slip, review rounds balloon | seven phases (D-Q6), each its own commit with its own docs and tests, each standing on its own; Phase 3 may land as several commits (one per area or per namespace), `commit` last |
| R-9 | A prompt or menu opens for an agent and its tool call hangs (journal class `command-waits-for-input-it-was-not-given`) | an agent's `./le` call never returns | both stdin AND stdout must be terminals; any pipe or redirection on either side prints or refuses instead; pipe-case tests in Phases 2 and 7 |
| R-10 | The derived `Node` tree drifts from what dispatch accepts (completion offers a word dispatch refuses, or misses one it accepts) | a completed command line is refused | the tree is derived from the same `leaction.Parameter` declarations dispatch parses; `TestEveryCompletedWordIsAcceptedByDispatch` walks the tree and dispatches every leaf in check-only parse mode |
| R-11 | Moving the menu model out of `cmd/ze` changes ze's launcher | `TestMenuModel_*` in `cmd/ze/tui_menu_test.go` go red, or ze's menu looks different | the ten `TestMenuModel_*` tests move with the model unchanged in their assertions; ze's builders (`buildTopLevel`, `buildYANGLevel`) stay in `cmd/ze`; `docs/architecture/cli/tui-launcher.md` source anchor moves in the same commit |
| R-12 | Parameterising the completion scripts changes ze's completion | a ze user's tab behaves differently | `TestZeCompletionScriptsUnchanged` goldens captured from HEAD before the refactor; `test/ui/root-completion.ci` stays green |
| R-13 | The exit-code split (Phase 5) breaks the hook protocol: Claude Code reads 2 from a hook as "block" | every session's hooks stop blocking, or block everything | `le ai hooks *` is a declared exemption, listed in `le help ai exit-codes`; `TestHookActionsKeepTheHookProtocolCodes` |
| R-14 | Helper exit codes change and `.ci` files asserting `expect=exit:code=` on `le test fixture` or a mock go red or, worse, green | `.ci` failures in many suites | forwarding areas, `test fixture`, `test peer` and every `mock` area pass their program's code through and are declared so; `TestPassThroughAreasKeepTheirProgramsCode` |
| R-15 | `le help ai` swallows `le help <name>` for the `ai` namespace | `le help ai` no longer shows the `ai` namespace page | D-4: the `ai` namespace page is `le ai help` (and `le ai --help`, and bare `le ai`); `le help ai` is the one reserved word, stated in `running-commands.md`; `TestHelpAiIsTheReferenceAndAiHelpIsTheNamespacePage` |
| R-16 | Wrapping width differs between a terminal and a pipe, so goldens depend on the host | goldens pass locally and fail in CI | wrap to the terminal width only when stdout is a terminal; otherwise a fixed 100 columns; goldens run on a pipe |
| R-17 | Typed validation refuses a value an existing caller passes today (a uint keyword given `auto`, a path that does not exist yet) | a hook, script or `.ci` that worked is refused with exit 2 | a kind validates syntax only (uint parses, duration parses, enum member), never existence; `path` and `spec` kinds complete from disk but accept any value; each conversion in Phase 3 greps the consumer surface for the keyword's values |
| R-18 | Many sessions edit `internal/le` while Phases 3 and 4 run; a rename commit lands over another session's new citation of the old spelling | `le doc check retired-commands` red after a pull | each rename commit re-runs `retired-commands` against HEAD at commit time; `retirednames.go` makes any later old spelling fail loudly rather than silently |
| R-19 | `ze` itself contradicts `ai/rules/cli.md`: `ze help ai --json` reads a rendering flag (`cmd/ze/help_ai.go`, `slices.Contains`) and `--ai` is a hidden alias (`aiHelpRequested`, L698) | `le` and `ze` answer `help ai` differently | out of this spec (ze is not in scope). Journal row candidate for the main thread, class `rendering-flag-on-a-command` or the nearest existing class: `ze help ai --json` and `ze --ai`, `cmd/ze/help_ai.go`, contradict `ai/rules/cli.md` "a rendering flag MUST NOT exist" and "renamed outright rather than aliased"; owner spec to be named by the main thread. This spec only makes ze's `help ai` render through the shared reference renderer, with its flags untouched |
| R-20 | The closed verb vocabulary (D-Q9) is large because le has 246 distinct action words over 375 actions in 110 tabled areas (measured 2026-10-25 from `le help ai commands | json`; the earlier "about 184" is stale), 214 of them used once | the vocabulary becomes a list of every word, which proves nothing | Designed in D-P4-vocab: the vocabulary is 36 verbs, each with a kind and one meaning ("Verb vocabulary"); an action word outside it is a name (a suite, scenario, platform, shell, artifact or selection) that declares the vocabulary verb it performs; synonyms and compound words are renamed per "Phase 4 rename map". `TestNoTwoVocabularyVerbsShareAMeaning` refuses two entries with the same meaning text; a verb used by exactly one action stays only if no other entry covers its job |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Every session's development commands: hooks (`le ai hooks`), commits (`le commit`), spec claims, functional tests that spawn `le test` helpers. Nothing in the shipped `ze` |
| How is it reverted? | One commit revert per phase, provided each phase lands as its own commit with its rename-map rows and citation updates |
| Who else touches this path? | Other sessions edit `internal/le` continuously (today: `commit/verification.go`, `go/lint`, `verify/engine`, `test/functional`, `le/action/leaction.go`); every rename commit is judged against HEAD at its time |

## Design questions for the next gate

Every question below is DECIDED (main thread, 2026-10-25, on Thomas's instruction
that implementation starts as soon as the spec is written). Each decision, with
its reason, is the matching `D-Q<n>` entry in "Design", "Decisions".

| Question | Status |
|----------|--------|
| Q-1 | Decided: D-Q1 |
| Q-2 | Decided: D-Q2 |
| Q-3 | Decided: D-Q3 |
| Q-4 | Decided: D-Q4 |
| Q-5 | Decided: D-Q5 |
| Q-6 | Decided: D-Q6 |
| Q-7 | Decided: D-Q7 |
| Q-8 | Decided: D-Q8 |
| Q-9 | Decided: D-Q9 |
| Q-10 | Decided: D-Q10 |
| Q-11 | Decided: D-Q11 |

| # | Question | What each answer changes |
|---|----------|--------------------------|
| Q-1 | `ai/rules/cli.md` bans rendering flags (only `--help`, `-h`, `--version`, `-V` exempt) and aliases. D-1 asks for `le help ai --json` and `--ai`. Either the rule gains a named exception for `help ai` on both binaries, or le answers `le help ai | json` and ze's `--json` and `--ai` are brought into line | The first edits `ai/rules/cli.md` and the F3 coverage note; the second changes ze's documented `ze help ai --json` (MCP overview page) |
| Q-2 | Where the shared completion, help-ai and menu library lives: `internal/component/command` (already holds Node, TreeCompleter, HelpEntries), a new `internal/core` package, or `internal/plugins/completion` parameterised in place | Imports for both binaries; must stay outside `internal/le` |
| Q-3 | Typed parameters: extend `leaction.Parameter` with kind, enum values, description and value source and derive a `command.Node` with `ArgDefs` and `ValueHints` from it, or replace `Parameter` with `command.ArgDef` and grow ArgDef (repeat, pair, retired replacement, path, revision, spec-file kinds) | Reuse versus one more model; the second touches ArgDef's YANG invariants and ze's validators |
| Q-4 | Does bare `le` at a terminal open ze's menu (the TUI launcher moved out of `cmd/ze` package main), or keep printing the root page? `bin/le` is `cmd/ze` without `ze_core`, so a build-tag change inside package main is a third option | Scope of the TUI move; D-1 names interactive filling, not the menu |
| Q-5 | Interactive filling: which arguments are prompted (required only, or every declared one), with what widget (line prompt or bubbletea form), and whether the filled line is echoed so the user learns the command | UX, and whether `charm.land/bubbletea/v2` enters le's path |
| Q-6 | Migration order into phases and commits. Proposed: typed parameter model and Node derivation first (wiring), then help pages, then the 30 parser migrations smallest first and `commit` last, then renames (each with rename-map rows), then exit codes, then completion and `help ai`, then prompting | Review scope per round; D-2 already fixes that the two superseded skeletons go in the first commit |
| Q-7 | Completion install for `./le`: register for both `le` and `./le`, and have the callback invoke `bin/le` directly, bypassing the launcher's build and stale check | Whether the stale warning ever reaches completion users; A-7 |
| Q-8 | Exit-code vocabulary for le: 0 answered, 1 ran and found a defect, 2 usage error, and a distinct code for an environment that could not run (carried one-verb AC-5); which number | Every gate's contract and every hook reading a code |
| Q-9 | Where le's verb vocabulary is declared (`check`, `update`, `list`, `run`, `selftest`, `report`) and enforced: a section in `command-verbs.md` or `running-commands.md`, plus a ratchet over the manifest | D-3 noun-verb consistency needs a stated rule and a check |
| Q-10 | New homes for mocks and spawned helpers now under `test` (`fixture`, `peer`, `rtr-mock`, `irr`, `dns`, `cymru`, `peeringdb`, `radius-mock`, `tacacs-mock`, `rpki`, `engine-steps`, `record-plugin`, `text-plugin`, `plugin-external`, `syslog`, `httpd`, `httpget` and others): one namespace such as `le mock` and `le helper`, within the two-word limit | Directory moves under the path-predicts-package rule and 1613 `.ci` files |
| Q-11 | `ze le` (the `ze_le` build): do completion and `help ai` cover the `ze le` spelling too, given `refuseValue` already hard-codes `usage: le ` | Whether the program name is threaded through every renderer |

## Design

### Alternatives

| Choice | Option | How it works | Gains | Costs | Verdict |
|--------|--------|--------------|-------|-------|---------|
| Parameter model | A. Extend `leaction.Parameter`, derive a `command.Node` tree | Parameter gains kind, enum values, per-keyword description, value source, retired successor, pair arity and the positional rule; a read-only `Node` tree is built from the registry for completion, help, menu and `help ai` | one declaration; ArgDef's YANG invariants untouched; dispatch keeps its parser | a derivation function to maintain, and repeat/pair/retired are honoured beside TreeCompleter rather than inside it | **Recommended** |
| Parameter model | B. Replace Parameter with `command.ArgDef` | grow ArgDef with repeat, pair, retired, path, revision and spec kinds; le declares ArgDefs | one type across ze and le | ArgDef is built from YANG with private fields and constructors; growing it with kinds YANG never produces weakens its invariants and touches ze's validators | Rejected |
| Parameter model | C. A le-only completion table beside Parameter | a second list naming each keyword's candidates | fastest to write | a second declaration of every keyword (`ai/rules/principles.md`) | Rejected |
| Completion | A. Parameterise `internal/plugins/completion` by program | one generator, a program descriptor (name, callback, per-program fragment), one record format (`writeCompletionRecord`) | ze and le share one script skeleton and one callback protocol | ze's fragment must be lifted out of the literal without changing ze's bytes | **Recommended** |
| Completion | B. Copy the three scripts into le | le owns its scripts | no risk to ze | three copies that drift | Rejected |
| Completion | C. le completes through `ze completion` | the shell calls ze for le words | no le code | ze does not hold le's tree outside the `ze_le` build, and the callback would pay two binaries | Rejected |
| `help ai` | A. Shared reference type and renderer in `internal/component/aihelp/reference` | a program-neutral reference (program, sections, commands with typed parameters) rendered through `internal/core/helpfmt`; ze's `aihelp.Build` fills it, le fills it from its tree | one shape and one renderer for both programs | ze's `help_ai.go` is re-plumbed | **Recommended** |
| `help ai` | B. le answers only `le '|' json` (the manifest) | no new command | nothing to build | not the `ze help ai` shape D-1 asks for, and no text summary | Rejected |
| `help ai` | C. Reuse `aihelp.Reference` whole | le fills ze's struct | no new type | the struct is ze-specific (RPCs, plugins, families, services) and le would publish empty fields | Rejected |
| Menu | A. Move the model to `internal/component/cli/menu` | the bubbletea model, navigation and styles move; ze's level builders stay in `cmd/ze` and feed items; le builds items from its tree | one menu, linked into both binaries | ze's tests move with the model | **Recommended** |
| Menu | B. Copy `tui_menu.go` into le | le owns a menu | no ze change | a copy (`ai/rules/principles.md`) | Rejected |
| Menu | C. Change build tags so package main compiles `tui_menu.go` into `bin/le` | `bin/le` gains `ze_core` files | no move | drags ze's whole package-main surface into le, and le's menu would still need le items | Rejected |
| Prompting | A. One-field bubbletea prompt in the menu package, completing from the value source | asks for each missing required parameter, Tab completes, then echoes the full line | completion while typing; shares the menu's terminal handling | needs raw mode | **Recommended** |
| Prompting | B. `bufio` line prompt | read a line per parameter | trivial | no Tab completion, which D-Q5 requires | Rejected |
| Completion callback path | A. Script calls `<dir>/bin/le completion words` directly; the `completion` area skips cache-link repair and before-dispatch hooks | no launcher build, no stale check, no hook | a keystroke never builds or prints | the script resolves the binary path | **Recommended** |
| Completion callback path | B. Teach the launcher to recognise `completion words` | launcher special-cases the words | works for every spelling | a second declaration of the command name in shell, outside the registry | Rejected |
| Exit codes | A. 0 success, 1 found a problem, 2 usage, 3 could not run | four meanings, one number each | matches leaction's original intent and the one-verb skeleton | every producer audited | **Recommended** |
| Exit codes | B. sysexits (64 usage, 69/70 could not run) | BSD numbers | standard numbers | breaks the existing 2-means-usage contract everywhere at once, for no reader that asks for sysexits | Rejected |
| Helper homes | A. `le mock <name>` for mocks and stand-in servers; `test fixture`, `test peer`, `test mcp`, `test replay` stay under `test` with a `tool` tag | mocks leave `test`; the two 2,000-use tools stay | small `.ci` churn for the heavy tools; the `test` namespace lists suites and tools, tagged | about 150 `.ci` lines move for the mocks | **Recommended** |
| Helper homes | B. Move every helper, `fixture` and `peer` included | one rule for every helper | the purest split | about 2,370 `.ci` lines rewritten (R-5) for no reader gain over a tag | Rejected |

### Current behaviour to proposed behaviour

| Surface | Today | After |
|---------|-------|-------|
| Bare `le` | root page on stdout, exit 0, everywhere | at a terminal (stdin and stdout): the menu. Elsewhere: the root page (new tree layout) on stdout, exit 0 |
| `le help`, `le --help` | the root page on stderr | the root page on stdout, exit 0 |
| Root page | five group headings | one block per namespace (first word), single-word areas in one block; each line: name, purpose tag, first sentence of the description, wrapped |
| Area and action pages | three formats; `helpNode` splits free-text Subs | one renderer over the action table: action page lists typed keywords with kind, values, required, repeat and description; whole `Why` wrapped |
| Usage error | whole family (`refuseLifecycle`) or the area page | the one action's usage and keyword lines, on stderr, exit 2 (exit 1 until Phase 5 where today's code is 1) |
| Program name | `usage: le ` hard-coded | the running personality, `le` or `ze le` |
| Hand-rolled areas | 30 own parsers | every area on `leaction.Area`; forwarding areas declare their grammar or declare that words pass through |
| Completion | none | `le completion bash|zsh|fish`, callback `le completion words` |
| Reference | `le '|' json` only | `le help ai` (text), `le help ai <section>`, and `| json`, `| yaml` |
| `le ai` page | `le ai`, `le help ai` | `le ai`, `le ai help`, `le ai --help`; `le help ai` is the reference |
| Missing required parameter | refused | at a terminal: prompted; elsewhere refused with the one action's usage, exit 2 |
| Exit codes | 2 means four things; `RefuseArgument` 1 | 0, 1, 2, 3 as D-Q8; declared exemptions |
| Retired spelling | `unknown command: <word>`, exit 1 | names the new spelling, from the one rename map |

### Preservation list

Everything in "Behavior to preserve" above, plus: `le ai hooks <name>` spellings
(D-hooks), ze's three completion scripts byte-for-byte, ze's menu behaviour
and its ten `TestMenuModel_*` assertions, ze's `help ai` output bytes, the
human gate in `internal/le/le/human`, and every forwarded program's own help and
exit code.

### Decisions

- → Decision D-Q1: `ai/rules/cli.md` bans rendering flags and aliases, and the rule outranks D-1's wording, so `le help ai` takes no `--ai` and no `--json`. The text summary is `le help ai`; JSON and YAML come through le's pipe layer (`le help ai | json`, `| yaml`). Sections are words, shaped by what le holds: `commands` (every area, its actions and typed parameters), `groups` (the purpose tags and their meanings), `verbs` (the closed vocabulary, D-Q9), `exit-codes` (D-Q8 and its exemptions) and `all`. No ze-only section (`api`, `mcp`, `dispatch`) is copied. `le help ai --json` is refused as a dash-leading token in a word slot, naming `le help ai | json`, exit 2. Reason: one rule for both programs, and a pipe answer is visible to grammar feeder F3 where a hand-read flag is not
- → Decision D-Q1 (ze): ze's `--ai` alias and `ze help ai --json` contradict the same rule. They are not changed here (ze is out of scope); R-19 carries the journal row candidate for the main thread
- → Decision D-Q2: shared machinery lives outside `internal/le` (`TestNormalZeLinksNoInternalLe`). Completion: `internal/plugins/completion` gains a program descriptor, so ze and le share one generator and one callback record format; the tree walk stays `command.TreeCompleter`. Menu: the model (items, levels, navigation, scrolling, styles, `truncateDesc`) moves from `cmd/ze/tui_menu.go` to a new library package `internal/component/cli/menu`, which imports only `internal/component/command`, `internal/core/textbuf`, bubbletea, lipgloss and `golang.org/x/term`; ze's `buildTopLevel` and `buildYANGLevel` and its command execution (`cli/client`) stay in `cmd/ze`. `help ai`: a new `internal/component/aihelp/reference` package holds the program-neutral reference type and its text renderer over `internal/core/helpfmt`; `internal/core/helpfmt` cannot host it because the reference names `command` types and `internal/core` is a leaf tier. Reason: one copy of each, importable by both binaries
- → Decision D-Q3: `leaction.Parameter` stays the one declaration. It gains: `Kind` (a closed set declared once in `leaction`, each with a one-line meaning and a syntax validator: switch, string, enum, uint, duration, path, revision, spec, label, area; Phase 1 adds any further kind a converted area needs, to the same set), `Values` (enum members), `Help` (one line per keyword), `Source` (a value source: static from `Values`, or a dynamic function returning candidates with summaries, which never runs a handler), `Retired` (the successor spelling; the keyword is refused naming it and never rendered or completed), and a pair arity with the second value's name (usage derived, never written by hand). A positional value is a Parameter with no keyword: at most one per action, in the first slot after the verb, of a kind other than string. A `command.Node` tree is DERIVED from the registry (`command.ListLocal` under the `le ` prefix plus `leroot.ActionsOf`): switch maps to `ArgFlag`, enum to `ArgEnum`, uint to `ArgUint`, every other kind to `ArgString`, value sources to `ValueHints` or `DynamicChildren`. Derivation boundary: the tree is read-only and serves completion, help, the menu, prompting and `help ai`; dispatch and validation parse only through `leaction`. Repeat, pair and retired are not projected into ArgDef; the le completion callback applies them (a non-repeat keyword already used is not offered again; a retired keyword is never offered). Parameter is not replaced by ArgDef, which lacks repeat, pair and retired-keyword. Reason: one declaration, ArgDef's YANG invariants untouched
- → Decision D-Q4: bare `le` with stdin AND stdout both terminals opens the shared menu over the derived tree; any other case (pipe, agent, CI, redirection) prints the root page to stdout, exit 0, as today. `le help` and `le --help` print the root page to stdout, exit 0. Reason: agents run on pipes (A-11, R-9)
- → Decision D-Q5: at a terminal (stdin and stdout), a missing REQUIRED parameter is prompted for, one by one, with Tab completion from its value source and enum values; then le echoes the full command line it is about to run (to stderr, prefixed so it is never mistaken for output) and runs it. Optional parameters are never prompted. Off a terminal: refuse with the usage of that one action, never the family, exit 2. Ctrl-C or Ctrl-D at a prompt runs nothing and exits 2. A prompt never answers the human gate in `internal/le/le/human`. Reason: the user learns the command, and nothing ever waits on a pipe
- → Decision D-Q6: seven phases, each its own commit, in this order: (1) remove the two superseded skeletons and repoint their citations, typed Parameter model, derived Node tree; (2) help pages: one renderer for the root page (by tree, purpose as a tag), namespace and action pages, per-keyword help, usage errors naming one action, `Why` first sentence in listings and whole on the action page, help on stdout; (3) migrate the 30 hand-rolled areas onto action tables, `commit` last, forwarding areas declaring their grammar or that words pass through (which help shows); (4) renames through `retirednames.go` and `le doc check retired-commands`, every consumer updated in the same commit; (5) exit codes; (6) completion and `le help ai`; (7) menu and prompting. Each phase updates its documentation pages in that phase. Phases 3 and 4 may be several commits (one per area or rename group), each standing on its own
- → Decision D-Q7: `le completion bash|zsh|fish` prints a script that registers both `le` and `./le` (bash and zsh by command word and by path; fish by command name, with the `./le` case documented in the script's install line). The script resolves the typed command word and calls `<that word's directory>/bin/le completion words <path...>` when that file exists, else the word itself, with stderr discarded. The `completion` area is declared as a shell callback at registration, so `run` skips cache-link repair and before-dispatch hooks for it; no launcher change is needed, because the launcher is never invoked. The callback prints only candidate records on stdout. Budget: under 100 ms on a warm binary (AC-37, A-4)
- → Decision D-Q8: exit codes: 0 success; 1 the check ran and found a problem; 2 usage error (unknown command, namespace non-member, unknown verb, bad or missing value, retired spelling, refused flag); 3 could not run (the environment or tree the gate needs is unreadable or absent). `leroot.RefuseArgument` moves from 1 to 2, matching `refuseValue`. `commit review-check` not clean moves from 3 to 1. `rfc check` splits: real violations 1, could not run 3. `cligrammar` unreadable tree 3. `spec status` closure sub-verbs' errors join the area's usage code 2. Declared exemptions, listed by `le help ai exit-codes`: `le ai hooks *` (the hook protocol: 0 allow, 2 block), every forwarding area, `test fixture`, `test peer` and every `mock` area (their program's code passes through). Phase 5 re-greps every reader against HEAD (A-6) before changing a producer
- → Decision D-Q9: le's verb vocabulary is declared once in `leaction` as a closed set, each verb with a one-line meaning; a test refuses a registered verb outside the set and a set entry no action uses. The owner's write/update ruling applies: `update` regenerates a committed artifact from its source; `write` leaves the vocabulary. Judged per action (A-12): ten `write` actions become `update`; `verify status write` becomes `verify status record` (records a verdict, as `spec review record` does); `worktree update` becomes `worktree rebase` (it rebases, which is not regeneration). The vocabulary is documented in `docs/contributing/running-commands.md` and rendered by `le help ai verbs`
- → Decision D-Q10: mocks and stand-in servers move to `le mock <name>`, packages under `internal/le/mock/<name>` (path-predicts-package): `cymru`, `dns`, `irr`, `peeringdb`, `rpki`, `rtr` (was `test rtr-mock`), `radius` (was `test radius-mock`), `tacacs` (was `test tacacs-mock`), `syslog`, `httpd`, `lg`, and the spawned helpers `engine-steps`, `record-plugin`, `text-plugin`, `plugin-external`. The four spawned helpers are hidden: absent from help pages and completion, present in `le help ai` with a hidden marker. `test fixture`, `test peer`, `test mcp` and `test replay` stay under `test`, with a new purpose `tool` instead of `suite`. Exact membership is re-checked against each helper's `ShortHelp` in Phase 4
- → Decision D-Q11: `ze le` derives from the same tree and the same renderers, so completion, help, `help ai`, menu and prompting cover it with no extra code; the program name is threaded from the running personality into every renderer (`refuseValue`'s `usage: le ` literal goes). AC-48 proves parity
- → Decision D-hooks: `le ai hooks` names are referenced 14 times from `.claude/settings.json`. They are kept. A rename is allowed only with `.claude/settings.json` in the same commit; none is planned
- → Decision D-retired: the rename map in `internal/le/doc/check/retirednames.go` moves to a leaf library package `internal/le/le/retired`, read by both `le doc check retired-commands` and `leroot.Dispatch`. A typed retired spelling is refused at runtime naming its successor (`<old> was renamed: run le <new>`), before parameter parsing, exit 1 in Phase 4 and 2 from Phase 5. Reason: one declaration answers both the tracked-file scan and the runtime refusal; today's runtime says only `unknown command`
- → Decision D-help-ai-word: `le help ai` is the one reserved word after `le help`; every other `le help <path>` prints that node's page as today. `le ai`, `le ai help` and `le ai --help` print the `ai` namespace page (D-4)
- → Decision D-keyword-help: per-keyword help is `le help <area> <verb> <keyword>`, extending the existing `le help <path>` form one level; it prints kind, values, required, repeat and description, exit 0. A trailing `--help` after a keyword that takes a value stays a refused dash-leading value (GNU syntax, `ai/rules/cli.md`)
- → Decision D-why: listings show a `Why`'s first sentence (up to the first full stop followed by a space, or the whole text); the action page shows the whole `Why`. Wrapping is at render time: to the terminal width when stdout is a terminal, else 100 columns, the name column aligned (R-16)
- → Decision D-P1-validate (Phase 1): each kind's syntax check is declared and tested (`TestKindValidatorsCheckSyntaxOnly`) but NOT applied by `parseArguments` until Phase 5. Applied at parse, a malformed value answers 2 where several bodies answer 1 today (`test qemu netns-test suites` refuses through `parseClosedCSV` with 1), and Phase 1 keeps exit codes unchanged (AC-17). Phase 5 applies `Kind.Check` in `parseArguments` with the other code changes
- → Decision D-P1-kinds (Phase 1): the set gains `list` (comma-separated items, closed when `Values` are declared: `suites`, `scenarios`, `dut`, `files`) and `words` (a whole le command line). `words` replaces the `leaction.ValueLeWords` Value marker, which is deleted: `NestedLeKeyword` reads `Kind == KindWords`, so the nesting fact is declared once (`ai/rules/no-layering.md`). Enum and closed-list usage renders the members (`<amd64|arm64>`), so no usage line lost the values its old hand-written Value spelled
- → Decision D-P1-tree (Phase 1): R-10 is proven twice: `TestEveryCompletedWordIsAcceptedByDispatch` (`internal/le/le/root/tree_test.go`) over a probe area, and `TestDerivedTreeLinesAreAcceptedForEveryRegisteredArea` (`internal/le/actions_test.go`) over every linked area, because only `internal/le` links them and a root-package test cannot import it. `command.TreeCompleter` offers an `ArgEnum`'s values at the action node, before any keyword; the Phase 6 callback must offer them only after their keyword. The grammar code moved from `leaction.go` (past 1000 lines) to `internal/le/le/action/parameter.go`
- → Decision D-groups: the purpose tag set is one declared table in `group.go` (name, tag, one-line meaning), replacing the `GroupTitle` switch; `tool` is added for test-driving helpers. Every area still declares its group at `leroot.Register`, and `internal/le/group_test.go` keeps its meaning
- → Decision D-P2-required (Phase 2): the parser does NOT refuse a missing Required keyword in Phase 2. 37 Required parameters in 17 areas are answered by their own bodies with their own codes, and Phase 2 keeps exit codes unchanged; that refusal lands with the kind checks in Phase 5. Phase 2 prints the one-action usage on every parse refusal, and `spec claim` refuses missing words in its handler through `leroot.RefuseUsage`. The Phase 2 TDD row is delivered as `TestAParseRefusalOnAPipePrintsOneActionUsage` and `TestClaimWithoutASpecRefusesWithClaimsUsageOnly`; `TestMissingRequiredParameterOnAPipeRefusesWithOneUsage` is owed by Phase 5
- → Decision D-P2-program (Phase 2): the running program's name is set once by `leroot.Dispatch` through `leaction.SetProgram`, default `le`, and every renderer and refusal reads `leaction.Program()`, so `ze le` pages name `ze le`
- → Decision D-P2-tags (Phase 2): a listing's purpose tag is the area's Group string in brackets, and `[ungrouped]` for an area that declares none
- → Decision D-P3-stray (Phase 3): a stray or unknown word given to a migrated area exits 2 from the moment it reaches the table, which is D-Q8's usage code brought forward from Phase 5. Reason: the table's dispatch (`TestEveryPublishedVerbIsDispatchedThroughItsOwnTable`) answers 2 for such a word in every tabled area, and a per-action usage-code override would be a second refusal path kept only to preserve a code D-Q8 retires. The three `TestAnswerRefusesAnArgument` tests (build/gosum, arch/ifaceresolution, cli/ownership) change their expected code from 1 to 2 and keep their assertion that the word is refused; every other refusal of a migrated area keeps its code until Phase 5 (AC-17 amended). Widened after batch B: every refusal the table's PARSER makes (stray or unknown word, missing value, repeated keyword, a dash-leading token in a value slot) exits 2 in a migrated area, and a test asserting 1 for one of those changes its expected code to 2 with its refusal assertion kept; a refusal the action's BODY makes keeps its code until Phase 5
- → Decision D-P3-dash (Phase 3): a dash-leading token in a value slot (`le test stress-repro run suite -help`) is refused by name with exit 2 and starts nothing, even in the trailing position. Reason: `ai/rules/cli.md` says a dash-leading token where a value is expected MUST be refused by name, and `-help` is the cluster `-h -e -l -p`, not `--help`; the trailing-position rule only forbids starting work, which a refusal satisfies. The table parser is fixed to match, and `TestEveryValueSlotRefusesAnOptionWithoutStartingTheRun` stays as it is
- → Decision D-P3-slug (Phase 3): `le ai tokens project` takes a checkout path, and the transcript-store slug is derived from it with `SlugForPath`; a slug typed as the value is refused as a dash-leading token. Reason: `ai/rules/cli.md` says a value MUST NOT begin with a dash, and every slug begins with one, so the keyword's old meaning cannot be expressed in the grammar; a per-slot exemption would make the dash rule depend on the keyword. `TestAProjectSlugBeginningWithAHyphenIsAValue` asserts the forbidden form, so it is replaced by a test that a path selects its project and a slug value is refused; the replacement needs the owner's weakening approval before it is committed, recorded with the Phase 2 one
- → Decision D-P3-refuseargument (Phase 3): once the last area migrates, `leroot.RefuseArgument` has no caller and is deleted (`ai/rules/no-layering.md`). AC-30 then holds by construction: the table's parser is the only refusal path for a stray word. `TestRefuseArgumentAndRefuseValueAgree` is replaced in Phase 5 by `TestEveryStrayWordRefusalComesFromTheTable`, which fails if any area answers a stray word outside the table
- → Decision D-P3-bare (Phase 3): a single-gate area that runs its gate when bare (ai digest, arch iface-resolution, build gosum, cli grammar, cli ownership, config claims, doc consistency and the like) keeps running it when bare, and gains a `check` verb that runs the same gate, following the `doc wiring` precedent. Reason: hooks and verify stages call these bare, and a bare listing would silently turn every such call into exit 0, which `ai/rules/principles.md` forbids. AC-14 is amended to match. A test that asked a tabled area for its listing by calling it bare now asks with `help` (workflowcheck `actionExists`, which otherwise ran `build host-driver`), and the CI workflows name `build host-driver run` rather than the bare area
- → Decision D-P23-one-commit (Phases 2 and 3): Phase 2 (help pages) and Phase 3 (every area on the action table, batches A to E) land as one commit. Reason: five agents did them in parallel in the shared checkout and their hunks share files (`internal/le/le/action`, `internal/le/le/root`, `internal/le/actions_test.go`, `docs/architecture/core-design.md`, `docs/contributing/running-commands.md`), so no split of the working tree leaves either half compiling and green on its own
- → Decision D-order-6-before-4 (while the Phase 2+3 commit waits on owner weakening approvals): Phase 6 (completion and `le help ai`) and Phase 7 (menu and prompting) are built before Phase 4 (renames) and Phase 5 (exit codes). Reason: Phases 6 and 7 add new code derived from the tree and change no spelling, while Phase 4 rewrites the rules, skills and docs every session in this shared checkout reads; uncommitted renames there would tell other sessions to type spellings the binary they run does not accept. Phases 4 and 5 run once 2+3 is committed, and Phase 6's completion and help ai pick the renames up through the derived tree with no extra work
- → Decision D-P3-tablecheck (Phase 3, AC-16): the migration list `areasWithoutAnActionTable` and its exemption branch are deleted, and the first `leroot.Dispatch` or `leroot.Tree` of a process runs `requireEveryAreaHasATable` once, which panics naming the first area with no action table and no pass-through declaration, and the `leroot.RegisterActions` call that fixes it. Reason: `leroot.Register` cannot make the check itself, because an area's init may call `RegisterActions` after `Register` and reordering 164 inits to put the table first is churn with no behaviour; by the first Dispatch every init has run. The test is `TestAnAreaWithoutATableStopsLeNamingIt` (`internal/le/le/root/leroot_test.go`), in place of the planned `TestRegisterWithoutActionsPanics`. A test fixture area that registers without a table now registers its pass-through declaration (verify/dispatch `registerProbe`)
- → Decision D-P23-push-argv (Phase 3, commit): `commit push "<owner authorisation>"` declares its words as a positional of kind `argv`, the action's only parameter, read as typed (a leading dash included). The positional rule (`checkPositional`: never kind string) and `TestPositionalParameterRules` are unchanged from HEAD. `New` refuses a positional argv beside any other parameter, because every word after the verb is its own. The body still takes exactly one word, so `push a b` is refused as before and the script output is unchanged. The dispatch probe (`probesFor`) sends a positional argv no probe, since every probe line would run the action (a real push preparation); `TestCommitRefusesWhatItsTableRefuses` asserts the bare and two-word refusals
- → Decision D-P6-root (Phase 6, verify): le's `completion` area keeps its name, and `TestLeRegistersOneRootAndNoToolRoots` judges who registered a same-named root. Reason: the test exists so that one package registers one root (`le`) and no tool of le's becomes a second one; `ze completion` is the product's own root, registered by `internal/plugins/completion`, and `le completion` lives at `le completion`, so the two never meet, and a rename would only cost users the word D-Q7 gave them. The test now fails when a root sharing a tool's name has a handler defined under `internal/le/`.
- → Decision D-P6-golden (Phase 6, verify): ze's captured completion scripts are `internal/plugins/completion/testdata/ze.<shell>.golden`. Reason: `TestNoPythonLeRemains` reads a tracked `.bash` file as a first-party shell source, and the captures are test data, so the suffix says what they are and no exemption list is needed.
- → Decision D-P6-golden-fixture (Phase 6, verify; replaces D-P6-tags): `ze help ai`'s pin is the functional test `test/ui/ze-help-ai-unchanged.ci`, whose native fixture (`internal/test/fixture/ui_fixture_ze_help_ai_unchanged.go`) runs the ze binary the functional runner built and compares it with `internal/test/fixture/testdata/ze-help-ai-{summary,all,json}.golden`: all three byte for byte, the services' `Env vars:` lines included. The summary golden was captured from `dbf0dbd866^`, before the shared reference type, built under the runner's tag set (`runner.TestBuildTags`). The `all` and `--json` goldens were first compared as sorted lines without the env-var lines, because RPC order and env-var attribution followed map order (journal `reference-output-depends-on-map-order.md`); once `aihelp` ordered its RPCs, notifications and services and gave a service only its own `ze.<service>.*` group, they were recaptured from that producer under the same tags and the comparison became byte exact (a strengthening). The recapture changes two services' env-var lines, which were wrong before: `bmp` listed gnmi's vars and `ntp` listed `ze.metrics.interval`. The cmd/ze unit test `TestZeHelpAiOutputUnchanged`, its goldens and its split feature-tag files are deleted, and no test skips. Reason: the output depends on which features the binary links (it prints the plugin and RPC counts), a unit test sees only the tags of the run that compiles it, and the runner's ze is the one build that always links every feature.
- → Decision D-P7-q (Phase 7): `q` quits the shared menu from any level while no filter is typed, and is a filter letter once one is. `tui-launcher.md` already said "q quits from anywhere" and AC-42 requires it, while ze's model typed it as a filter letter: the code was the side that was wrong. This is the one change to ze's launcher; `TestZeMenuViewUnchanged` (golden captured from the model before the move) pins every screen byte-for-byte
- → Decision D-P7-echo (Phase 7): the echoed line is the line le runs, so AC-44 echoes `running: le spec claim spec <path>`, not `le spec claim <path>`: `spec claim`'s grammar is `le spec claim spec <spec>` and a bare `le spec claim` reaches the area's one action. A bare line of an area with exactly one action prompts for that action; any other bare line is not prompted
- → Decision D-P7-missing (Phase 7): requiredness stays a declaration (`leaction.Requirement`); `List.Missing` derives the absent Required parameters from the same parser, and a line that parser refuses is not prompted, so it is refused as before. A forwarding action and a pass-through area are never prompted. Tab offers `leroot.Complete`'s candidates at that place, plus paths on disk for kinds path and spec. Ctrl-C or Ctrl-D answers 2; a terminal that fails under the prompt answers 1. `TestBareLeOnAPipePrintsTheRootPage` lives in `internal/le/le/root/menu_test.go` beside the menu test, not in `dispatch_test.go`
- → Decision D-P7-namespace (Phase 7, owner report: sub-menus had no description): a namespace (a first word areas share that is no area itself) declares its one-sentence summary once, with `leroot.RegisterNamespace(name, summary)` from `internal/le/<name>/register.go`, the path its name predicts, blank-imported by `internal/le/register.go` (15 namespaces: ai, arch, build, chaos, cli, config, data, doc, go, lab, plugin, spec, test, web, yang; `internal/le/spec` was already a library, the other 14 directories gain a package holding only `register.go`). `treeFrom` gives a namespace node that summary, so the menu shows it; the root page heads each namespace block with its first sentence (`helpfmt.HelpSection.Summary`); the namespace's own page carries it; `le help ai commands` lists every namespace with it (`reference.Reference.Namespaces`). A first word that is also an area (`repo`, `rfc`, `setup`, `site`, `verify`) is described by the area's own summary, because its page, manifest entry and completion already show that one; a `RegisterNamespace` for such a word is a second description of one entry and the startup check refuses it. The startup check (D-P3-tablecheck, renamed `checkRegistrations`) also refuses a namespace with no summary, naming the namespace, the `leroot.RegisterNamespace` call and the file it goes in. `TestEveryCommandIsFoundAtThePathItsNamePredicts` now also holds each namespace's registration to its predicted directory, and `TestEveryCommandRegistersItsOwnAnswerShape` leaves a namespace directory out because a namespace answers nothing and has no shape
- → Decision D-P7-describe-all (Phase 7, owner request "make sure all the sections of le have description"): every word le offers carries text, held as an invariant rather than a one-off scan. `checkRegistrations` gains `requireEveryItemHasText` (`internal/le/le/root/describe.go`), run once on the first Dispatch or Tree: it refuses to start while an area has no summary (Meta ShortHelp), a pass-through area names no program (its passes-to line is its description), an action has no `Why`, a live keyword has no `Help`, a value an enum or closed list declares has no line, or a line describes a value the keyword does not declare; the stop names the item, what to fill in and `internal/le/<area path>/`. Values had no description slot, so `leaction.Parameter` gains `ValueHelp map[string]string` (manifest key `value-help`) beside `Values`, which keeps its type so `Kind.Check`, the usage line and the manifest's `values` array are untouched; `Parameter.ValueCandidates` answers each value with its line, and completion (`valuesOf`), the menu's positional hints (`hintsOf`), the keyword page (`values:` one per line) and `le help ai commands` (`reference.Parameter.ValueHelp`, one indented line per value) all read it. A registry that holds the values fills the map from where each value is declared (`testintegration.StressScenarioHelp` from a `summary` field of each stress scenario; `vrrpScenarioHelp` replaces the `vrrpDescription` switch, which the report header and PASS line now read too). 29 values gained a description: perf step (2), yang path-refactor operation (3), setup docker-host arch (2), test qemu all-tests only (1), vrrp scenarios (5), netns suites (6), stress scenarios (5), rfc evidence-split from (1), rfc audit-stamp mode (2), weekly channel (2)
- → Decision D-P7-short-long (Phase 7, owner: "short in the menu's right column, long above an entered level"): every enterable word declares two texts instead of a derived first sentence, and `leaction.FirstSentence` is deleted (no layering). A namespace declares both in `RegisterNamespace(name, summary, description)`; an area in `registry.Meta` ShortHelp and Description (a pass-through area owes no long: its program prints its own help); an action in `Summary` (new, json `summary`) and `Why`; a keyword's `Help` and each `ValueHelp` line are shorts. The startup check (`requireShort`, `requireShortAndLong`) refuses a short that is blank, over one line or over 60 runes, and a missing long. The manifest and `le help ai` carry `short-help` beside `description`. The menu gains `NewDeclared` and `DeclaredLevelOf`: an item's Desc is its ShortHelp whole, and the level's Description prints wrapped under the breadcrumb only when non-empty, so ze, which keeps `New` and `LevelOf`, is byte-identical. Detail that no longer fits a 60-character Help was dropped or left in the action's `Why`; no per-keyword long text was added, because no keyword page needed one
- → Decision D-P4-vocab (Phase 4, design pass of 2026-10-25 over `le help ai commands | json`: 246 distinct action words, 375 actions, 110 tabled areas, 56 forwarding areas): `leaction` declares the closed vocabulary once, as a table of verb, kind and one-line meaning ("Verb vocabulary" below, 36 verbs). Every action word is one of two things. A VERB is a vocabulary entry and performs that entry's meaning. A NAME is a noun or adjective naming what the action works on (a suite, scenario, platform, shell, architecture, artifact or selection: `test functional bgp`, `go vet-platforms darwin`, `completion zsh`, `verify changed`, `commit message`) and declares the vocabulary verb it performs (`Does`), so every action still has exactly one verb meaning without renaming a noun. Three rules make the split testable rather than a free pass: a word that is a vocabulary verb never declares another verb (this is why `test functional install`, a suite, is renamed); no hyphen-separated part of a name is a vocabulary verb, a retired verb (`write`, `generate`) or `test` (the owner's ruling that `*-test` and `*-update` words such as `ipsec-mobike-test`, `index-update` and `condensed-update` are nouns to restructure); and `all` is the one shared name, meaning every member of the area in table order. `le help ai verbs` renders each verb with its kind, its meaning and the names that declare it. Reason: 214 of the 246 words are used once and most are suite or scenario names; listing them as verbs makes the vocabulary prove nothing (R-20), and renaming nouns into verbs churns 1,600 `.ci` files for no reader gain
- → Decision D-P4-minimal (Phase 4): a name keeps its spelling when it embeds no verb, even where another area spells the same job with a vocabulary verb: `commit message` (preview), `commit session` (ensure), `commit debt-status` (report), `spec model current`, `spec state current` and `spec state latest` (show), `doc ste review` and `review-changed` (report), `test sensitivity tracked` (report), `ai hooks unit` (selftest), `verify current`, `changed` and `worktree` (run), `session scratch` and `seed-store` (ensure), `rfc audit-stamp` (record), `rfc tagged-scope` and `test weakened proposed` (judge), `rfc evidence-split` (migrate), `rfc quote-backfill` (update). A word that is an English verb outside the vocabulary (`validate`, `lint`, `audit`, `append`, `reap`, `bump`, `sync`, `select`, `rename` in `rfc`) is renamed to the vocabulary verb that covers its job, and a vocabulary verb used with a second meaning (`report` in `perf`, `update` in `weekly`) is renamed to its real job. Single-action areas reached bare keep their action word (`spec release run`, `spec current show`, `job run`), except where a ruling makes the area word the whole command (`spec claim <file>`, D-3; `rollback`, D-P4-launcher)
- → Decision D-P4-frozen (Phase 4): the 14 `le ai hooks <name>` words that `.claude/settings.json` calls are declared frozen on the `ai hooks` area, citing D-hooks, and the name rules do not judge them (`session-start` contains `start`, `rule-coverage-report` contains `report`, `block-premature-stop` contains `stop`). `TestFrozenHookWordsAreExactlyTheSettingsCalls` reads `.claude/settings.json` and requires the frozen set to equal its `le ai hooks <name>` calls, so the exemption can never cover a word settings does not call; `ai hooks unit`, which settings does not call, is an ordinary name and passes the rules. No hook word is renamed, so `.claude/settings.json` and `.claude/hooks/README.md` are untouched by Phase 4
- → Decision D-P4-positional (Phase 4): where one area holds several compound words for one verb over different targets, the target becomes an enum positional (D-Q3: one, first slot, not string): `ai rules check <condensed|index|metadata|points-roundtrip|render>`, `ai rules update <condensed|index|render>`, `ai rules report <coverage|gate-map|payload|router>`, `site terminal-demo check <artifacts|release|validation>`, `site terminal-demo build <binaries|image>`, `perf record [evidence|history]`, `repo check [generated|tree]`. The positional is required, except for `perf record` and `repo check`, whose bare form keeps today's job (record HEAD; all six repository checks). Where the targets of one verb take different keywords, Phase 4 reads them first and uses a sub-area instead, because one action declares one parameter list. `rfc extraction-create`, `extraction-classify` and `extraction-status` become the sub-area `rfc extraction` (`create`, `classify`, `report`), registered from `internal/le/rfc/extraction` (path predicts package) and calling the existing `rfc` functions
- → Decision D-P4-launcher (Phase 4, owner rule in `ai/META-DECISIONS.md` "Developer tooling"): every word the `./le` launcher consumes before the binary runs is declared once in `leroot` (word, value, one-line meaning): `--name <name>` and `--name=<name>` (run the build of the working tree under that name; the value `session` names this session's build) and `rollback` (set the newest build aside). The root page renders them in a launcher block, `le help ai launcher` lists them, and completion offers them at the first word. `TestEveryWordTheLauncherConsumesIsInHelp` reads the words from the launcher script itself (the arms of its `case ${1:-} in`, its `[ "${1:-}" = <word> ]` tests and its `[ "$name" = <word> ]` value tests, all before the first `exec`), fails when it finds none so a restructured script cannot make it vacuous, and requires the script's set and the declared set to be equal both ways. `./le rollback <anything>` accepts and ignores the extra words today, which is a word le accepts that help cannot show, so the launcher refuses any word after `rollback` with exit 2 naming `./le rollback`; the binary's `rollback newest` retires and `rollback` becomes an area-only command, matching the launcher
- → Decision D-P4-counts (Phase 4): the consumer counts in "Phase 4 rename map" are tracked files whose text holds the old phrase (`git grep -l`, 2026-10-25; `plan` excludes `plan/learned` and `plan/journal`, which are history). A Go argv literal split into words (`"ai", "rules", "condensed-update"`) is found by `le doc check retired-commands` once the row is in the rename map, not by these counts. The 330 `internal/` files naming `yang glue write` are generated headers that the renamed command rewrites when it runs, so they change in the same commit by running `le yang glue update`, never by hand
- → Decision D-P5-save (Phase 5): a `| save` whose write fails (a missing directory, a full disk, a permission) was a well-formed chain that could not deliver its answer, so le exits 3; every other pipe refusal stays 2. The pipe layer names the case once: `saveError` opens with the constant `saveWriteFailure` and `command.IsSaveWriteFailure` reads the same constant (`internal/component/command/pipe_save.go`), and `leroot.Run` branches on it before `IsPipeError`. Ze's own callers do not call the predicate, so ze's codes are unchanged. Reason: the pipe layer reports both through one string, and a prefix predicate beside the existing `IsPipeError` is the contained change; a typed error would change the formatter's signature at every ze surface
- → Decision D-P5-extraction (Phase 5): `rfc extraction-status` over a tree with no `rfc/` tree, and every other failure that stops it (malformed summary, unreadable enrolled list, malformed artifact), exits 3: the input the count needs is absent
- → Decision D-P5-indexscope (Phase 5): `rfc index-update` over a scope the user wrote invalid stays a usage refusal, exit 2 (`ui_fixture_le_rfc_answers.go` unchanged)
- → Decision D-P4-machinery (Phase 4, part 1 of the batches): the rename map is package `leretired` (`internal/le/le/retired/retired.go`); `Match` answers the longest row whose old words open the typed line, and `leroot.dispatchLine` refuses a match before `resolve`, exit 2, `error: le <old> was renamed: run le <new> <the trailing words>`. `TestRetiredSpellingNamesItsSuccessor` (in `internal/le/le/root/retired_test.go`, the external test package that links every area) drives every row through Dispatch and refuses a row whose old words are still a live area or action, or whose successor action a table does not hold, so a historic row whose successor is renamed later (`docs-to-code update` to `doc index write`) goes red until its `Action` follows the rename
- → Decision D-P4-owed (Phase 4): the vocabulary tests judge the registered tree now, and the 303 findings it holds on 2026-10-25 (223 names without `Does`, 71 names holding a verb, `write` or `test`, 9 vocabulary verbs no action performs yet) are listed in `internal/le/testdata/vocabulary-owed.txt`, each with the rename or declaration that clears it. A finding outside the list fails its rule's test, and `TestPhase4OwedListOnlyShrinks` fails a listed finding the tree no longer holds, so each batch deletes its lines and the last batch deletes the file. Rows are not added to the rename map ahead of their rename: the map holds only spellings le refuses. Reason: the tests are live and red-checked from the first commit, and the work list is derived from the registry rather than copied from this spec
- → Decision D-P4-launcher-scan (Phase 4): `TestEveryWordTheLauncherConsumesIsInHelp` reads the whole `le` script, not only the part before the first `exec`, because the `--name` branch execs early and `rollback` is tested after it. A branch that hands the word on unchanged (`completion words`, exec with `"$@"` and no `shift`) is not a consumed word, and a branch of only `echo` lines and `exit 2` (`--update`) is a refused word, not one le accepts. The launcher table in `leroot` (`launcher.go`) declares `--name`, `--name=` and the value `session`; `rollback` is declared by its own area, so it is not listed twice. The root page shows a `Launcher` block, `le help ai launcher` a section, and completion offers the words first and `session` after `--name` and glued to `--name=`

### Verb vocabulary

The closed set of D-Q9 and D-P4-vocab. Kind: `report` reads and prints, exit 0
whenever it answered; `gate` reads and exits 1 on a defect; `regenerate`
rewrites a committed artifact from its source; `record` appends to a ledger;
`render` produces a published artifact this repository does not commit; `run`
has side effects outside the tree's committed files. 36 verbs, against 246
distinct action words today and no declared vocabulary.

| Verb | Meaning | Kind |
|------|---------|------|
| `report` | print what the area finds in the tree, read-only | report |
| `list` | print the members of a set, one per line | report |
| `show` | print one recorded item: a claim, a certificate, a path, a model, a script | report |
| `preview` | print what another action would do or write, doing and writing nothing | report |
| `compare` | print the difference between two or more inputs | report |
| `track` | print the trend of a history ledger over time; exit 1 on a regression only when asked | report |
| `suggest` | say whether a run is owed and why, never failing | report |
| `discriminate` | print which breaks of the producing code prove a tagged requirement, and what is still unproven | report |
| `combine` | merge several result files into one result file outside the committed tree | report |
| `check` | run the area's gate over the tree; exit 1 on a defect | gate |
| `selftest` | run the gate against its seeded fixtures, proving it catches what it claims | gate |
| `judge` | decide one proposed edit read on stdin; exit 1 when refused | gate |
| `update` | rewrite a committed artifact from its canonical source (owner ruling: never `write`, never `generate`) | regenerate |
| `reseal` | re-pin the recorded fingerprints a mechanical edit moved, keeping every verdict | regenerate |
| `record` | append one result, verdict or measurement to its ledger | record |
| `approve` | record the owner's approval of one named change, in his words | record |
| `render` | produce a page, deck or recording for the published site | render |
| `run` | start the area's job, with its side effects; exit is the job's | run |
| `build` | produce a binary or image that is not committed | run |
| `send` | send traffic at a target | run |
| `install` | install tools, a kernel or a profile on this host | run |
| `start` | start a long-lived service | run |
| `stop` | stop a long-lived service | run |
| `clean` | remove what earlier runs left behind, only what this checkout or run provably owns | run |
| `trim` | bound a store's size by evicting its oldest entries | run |
| `ensure` | create the item when absent and leave it when present, then print it | run |
| `create` | create one new item, such as a commit script, each time | run |
| `migrate` | convert files from a retired layout to the current one | run |
| `rewrite` | apply one mechanical edit to tracked files, previewing unless told to apply | run |
| `move` | relocate code or files to another path, carrying what refers to them | run |
| `rename` | change a name everywhere it is spelled | run |
| `rebase` | rebase a worktree branch onto its upstream | run |
| `push` | prepare the owner-authorised push of a verified commit | run |
| `publish` | post an announcement to an outside channel | run |
| `claim` | claim a spec for this session | run |
| `rollback` | set the newest le build aside so the previous one runs | run |

Retired from the vocabulary: `write` (owner ruling), `generate`, `validate`,
`lint`, `audit`, `append`, `reap`, `bump`, `sync`, `select`, and every compound
word in the map below.

### Phase 4 rename map

Every row adds its old spelling to the rename map (D-retired) in the commit that
updates its consumers. Consumer columns are file counts per D-P4-counts: ai,
docs, .claude, plan (live), test, internal, .github. `cmd/`, `AGENTS.md`,
`CLAUDE.md`, the `le` script, `.agents` and `.codex` hold none of these phrases.

| Old spelling | New spelling | Reason | ai | docs | .claude | plan | test | internal | .github |
|--------------|--------------|--------|----|------|---------|------|------|----------|---------|
| `ai sync write` | `ai sync update` | owner ruling (A-12) | 6 | 0 | 0 | 1 | 0 | 5 | 0 |
| `data asn-delegation write` | `data asn-delegation update` | owner ruling (A-12) | 0 | 1 | 0 | 0 | 0 | 6 | 0 |
| `data ietf-reference write` | `data ietf-reference update` | owner ruling (A-12) | 1 | 0 | 0 | 0 | 0 | 1 | 0 |
| `data ip-special-purpose write` | `data ip-special-purpose update` | owner ruling (A-12) | 0 | 1 | 0 | 0 | 0 | 0 | 0 |
| `doc index write` | `doc index update` | owner ruling (A-12) | 2 | 3 | 0 | 2 | 0 | 4 | 0 |
| `plugin gated-table write` | `plugin gated-table update` | owner ruling (A-12) | 0 | 1 | 0 | 0 | 0 | 4 | 0 |
| `plugin imports write` | `plugin imports update` | owner ruling (A-12) | 1 | 1 | 1 | 1 | 0 | 41 | 0 |
| `repo feature-tags write` | `repo feature-tags update` | owner ruling (A-12) | 0 | 2 | 0 | 0 | 0 | 5 | 1 |
| `web assets write` | `web assets update` | owner ruling (A-12) | 0 | 0 | 0 | 0 | 0 | 5 | 0 |
| `yang glue write` | `yang glue update` | owner ruling (A-12); 330 generated headers, D-P4-counts | 1 | 0 | 0 | 2 | 0 | 330 | 0 |
| `verify status write` | `verify status record` | records a verdict (D-Q9) | 0 | 1 | 0 | 1 | 0 | 1 | 0 |
| `worktree update` | `worktree rebase` | rebases, not a regeneration (D-Q9) | 0 | 0 | 1 | 0 | 0 | 1 | 0 |
| `spec claim spec <file>` | `spec claim <file>` | D-3 | 4 | 1 | 1 | 9 | 0 | 6 | 0 |
| `setup docker-host build|check|start|stop` | `setup docker host build|check|start|stop` | rule R9 of `le cli grammar` (Phase 4 step) | 1 | 5 | 0 | 2 | 0 | 21 | 0 |
| `setup docker-kernel apparmor|check|install` | `setup docker kernel apparmor|check|install` | rule R9 of `le cli grammar` | 2 | 8 | 0 | 11 | 0 | 24 | 1 |
| `test <mock>` (15 mocks and helpers) | `mock <name>` | D-Q10; counts per D-Q10 (about 150 `.ci` lines) | - | - | - | - | - | - | - |
| `ai rules condensed-check` | `ai rules check condensed` | compound word (D-P4-positional) | 0 | 1 | 0 | 0 | 0 | 8 | 0 |
| `ai rules index-check` | `ai rules check index` | compound word | 0 | 0 | 0 | 0 | 0 | 1 | 0 |
| `ai rules points-roundtrip-check` | `ai rules check points-roundtrip` | compound word | 0 | 1 | 0 | 1 | 0 | 4 | 0 |
| `ai rules render-check` | `ai rules check render` | compound word | 0 | 1 | 0 | 0 | 0 | 1 | 0 |
| `ai rules lint` | `ai rules check metadata` | `lint` is a gate, which `check` covers | 2 | 1 | 0 | 1 | 0 | 3 | 0 |
| `ai rules condensed-update` | `ai rules update condensed` | compound word, named by the owner | 5 | 1 | 0 | 6 | 0 | 8 | 0 |
| `ai rules index-update` | `ai rules update index` | compound word, named by the owner | 1 | 1 | 0 | 1 | 0 | 6 | 0 |
| `ai rules render-update` | `ai rules update render` | compound word | 3 | 2 | 0 | 0 | 0 | 1 | 0 |
| `ai rules coverage-report` | `ai rules report coverage` | compound word | 0 | 0 | 0 | 0 | 0 | 0 | 0 |
| `ai rules gate-map-report` | `ai rules report gate-map` | compound word | 0 | 1 | 0 | 1 | 0 | 5 | 0 |
| `ai rules payload-report` | `ai rules report payload` | compound word | 0 | 1 | 0 | 1 | 0 | 4 | 0 |
| `ai rules router-report` | `ai rules report router` | compound word | 1 | 1 | 0 | 0 | 0 | 5 | 0 |
| `test netlab render-check` | `test netlab check` | compound word; the render is the area's only artifact | 1 | 3 | 0 | 0 | 1 | 2 | 0 |
| `test netlab render-update` | `test netlab update` | compound word | 0 | 2 | 0 | 0 | 0 | 2 | 0 |
| `rfc index-update` | `rfc update` | compound word, named by the owner; the index is `rfc`'s only regenerated artifact | 4 | 5 | 0 | 39 | 0 | 14 | 0 |
| `rfc discriminate-record` | `rfc record` | compound word | 9 | 9 | 0 | 92 | 0 | 21 | 0 |
| `rfc extraction-create` | `rfc extraction create` | compound word; sub-area (D-P4-positional) | 2 | 3 | 0 | 4 | 0 | 11 | 0 |
| `rfc extraction-classify` | `rfc extraction classify` | moves with its family into the sub-area | 2 | 3 | 0 | 3 | 0 | 5 | 0 |
| `rfc extraction-status` | `rfc extraction report` | moves with its family; `status` is a report | 3 | 3 | 0 | 5 | 0 | 4 | 0 |
| `rfc rename` | `rfc move` | it moves test files; `rename` means changing a name (`go module rename`) | 4 | 2 | 0 | 3 | 0 | 8 | 0 |
| `perf report` | `perf compare` | it compares result files; `report` reads the tree | 1 | 3 | 0 | 1 | 0 | 7 | 0 |
| `perf evidence-record` | `perf record evidence` | compound word | 0 | 2 | 0 | 3 | 0 | 2 | 0 |
| `perf history-record` | `perf record history` | compound word | 0 | 1 | 0 | 1 | 0 | 3 | 0 |
| `test mutation record-history` | `test mutation record` | compound word; one ledger in the area | 1 | 2 | 0 | 0 | 1 | 4 | 0 |
| `doc yang-contract pipe-operators-update` | `doc yang-contract update` | compound word; one regenerated artifact in the area | 0 | 1 | 0 | 0 | 0 | 7 | 0 |
| `arch tier write-baseline` | `arch tier update` | compound word with a retired verb | 0 | 0 | 0 | 0 | 0 | 5 | 0 |
| `repo generate` | `repo update` | owner ruling: `update` regenerates a committed artifact | 8 | 11 | 0 | 6 | 1 | 5 | 0 |
| `repo generated-check` | `repo check generated` | compound word (D-P4-positional) | 0 | 4 | 0 | 1 | 1 | 5 | 0 |
| `repo tree-check` | `repo check tree` | compound word | 0 | 0 | 1 | 2 | 0 | 3 | 0 |
| `site update-talk` | `site talk` | compound word; a name that declares `update` | 0 | 0 | 0 | 0 | 0 | 2 | 0 |
| `site terminal-demo check-all` | `site terminal-demo check artifacts` | compound word | 0 | 1 | 0 | 0 | 0 | 0 | 0 |
| `site terminal-demo release-check-all` | `site terminal-demo check release` | compound word | 0 | 0 | 0 | 0 | 0 | 3 | 0 |
| `site terminal-demo validation-check-all` | `site terminal-demo check validation` | compound word | 0 | 0 | 0 | 0 | 0 | 3 | 0 |
| `site terminal-demo render-all` | `site terminal-demo render all` | compound word; `all` becomes a switch of `render` | 0 | 1 | 0 | 0 | 0 | 1 | 0 |
| `site terminal-demo binaries-build-ze` | `site terminal-demo build binaries` | compound word | 0 | 1 | 0 | 0 | 0 | 4 | 0 |
| `site terminal-demo image-build` | `site terminal-demo build image` | compound word | 0 | 1 | 0 | 2 | 0 | 2 | 0 |
| `scratch cache-clean` | `scratch clean` | compound word | 2 | 2 | 0 | 12 | 0 | 11 | 0 |
| `scratch store-trim` | `scratch trim` | compound word | 1 | 1 | 0 | 6 | 1 | 3 | 0 |
| `scratch links-ensure` | `scratch ensure` | compound word | 0 | 1 | 0 | 0 | 0 | 9 | 0 |
| `session reap` | `session clean` | `reap` and `clean` share one meaning | 0 | 1 | 1 | 3 | 0 | 3 | 0 |
| `test integration cleanup` | `test integration clean` | `cleanup` and `clean` share one meaning | 1 | 7 | 0 | 4 | 0 | 17 | 0 |
| `verify summary append` | `verify summary record` | `append` and `record` share one meaning | 0 | 0 | 0 | 0 | 0 | 1 | 0 |
| `spec journal validate` | `spec journal check` | `validate` and `check` share one meaning | 0 | 0 | 0 | 1 | 0 | 1 | 0 |
| `doc check verify` | `doc check all` | `verify` here means every check in order, which is `all` | 10 | 8 | 0 | 40 | 0 | 8 | 0 |
| `commit audit` | `test weakened check` | a gate; the weakening tools live in `test weakened` | 3 | 4 | 0 | 8 | 0 | 2 | 0 |
| `commit review-check` | `commit check` | compound word; `commit`'s one gate once `audit` leaves | 0 | 0 | 0 | 1 | 0 | 4 | 0 |
| `test functional select` | `test functional preview` | it prints what a gating run would start | 0 | 3 | 0 | 0 | 1 | 5 | 0 |
| `test functional install` | `test functional installer` | a suite spelled as the vocabulary verb `install` | 1 | 1 | 0 | 0 | 0 | 0 | 0 |
| `test functional exabgp-test` | `test functional exabgp` | `*-test` word, owner ruling | 3 | 6 | 0 | 7 | 0 | 3 | 0 |
| `weekly update` | `weekly publish` | it posts to Discord; `update` regenerates a committed artifact | 0 | 0 | 0 | 0 | 0 | 1 | 0 |
| `lab pin bump` | `lab pin update` | rewrites a committed pin from upstream, as `data * update` refetches | 0 | 2 | 0 | 0 | 2 | 2 | 0 |
| `web vendor sync` | `web vendor update` | rewrites committed copies from `third_party/web/` | 1 | 0 | 0 | 0 | 0 | 5 | 0 |
| `web vendor update-report` | `web vendor report` | compound word | 0 | 0 | 0 | 0 | 0 | 1 | 0 |
| `feature record-run` | `feature run` | compound word | 1 | 6 | 0 | 2 | 0 | 8 | 0 |
| `setup web-test` | `setup web` | `*-test` word | 0 | 0 | 0 | 0 | 0 | 0 | 0 |
| `setup proto-generate` | `setup proto` | compound word with a retired verb; a name that declares `update` | 0 | 1 | 0 | 0 | 0 | 4 | 0 |
| `rollback newest` | `rollback` | D-P4-launcher | 0 | 0 | 0 | 0 | 0 | 0 | 0 |
| `test qemu all-tests`, `install-test`, `install-iso-test`, `install-scenarios-test`, `install-ventoy-test`, `ipsec-mobike-test`, `mpls-boot-test`, `netns-test`, `pppoe-accel-test`, `pppoe-test`, `vpp-hugepages-test`, `vrrp-keepalived-test` | `all`, `installer`, `installer-iso`, `installer-scenarios`, `installer-ventoy`, `ipsec-mobike`, `mpls-boot`, `netns`, `pppoe-accel`, `pppoe`, `vpp-hugepages`, `vrrp-keepalived` | `*-test` words, owner ruling; `install-*` would keep the vocabulary verb `install` inside a name | 8 | 24 | 1 | 26 | 45 | 35 | 1 |
| `test deployment docker-l2tp-ppp-test`, `docker-pppoe-accel-test`, `gokrazy-l2tp-ppp-test`, `ipsec-mobike-test`, `l2tp-ppp-test`, `l2tp-scale-test`, `l2tp-test`, `vpp-iface-test`, `vpp-test` | the same words without `-test` | `*-test` words, owner ruling | 3 | 13 | 1 | 13 | 1 | 5 | 2 |

Totals: 113 retired spellings (12 from the A-12 rulings, 1 from D-3, 7 docker
actions, 15 mocks and helpers, 78 from this design pass: 57 single rows plus the
21 `test qemu` and `test deployment` words). The 21 `test qemu` and
`test deployment` renames each land with their `.ci` and workflow consumers in
one mechanical commit (R-5). Phase 4 re-runs every count against HEAD before
its commit (R-18).

### Data flow impact

| Path | Entry | Transformation | Boundary |
|------|-------|----------------|----------|
| Dispatch (unchanged shape) | argv | `run` to `Dispatch` to retired check (new) to `resolve` to `leaction.Area.Answer` to `leroot.Run` | none new |
| Help | `le help ...`, `--help` | `Dispatch` to the one page renderer over the action table and derived tree, stdout | `internal/core/helpfmt` |
| Completion | the shell's callback | `run` (hooks skipped) to `completion words` to derived tree to `command.TreeCompleter` plus le's repeat/retired filter to `writeCompletionRecord` | `internal/plugins/completion`, `internal/component/command` |
| `help ai` | `le help ai [section]` | derived tree to `aihelp/reference` to `leroot.Run` (pipes apply) | `internal/component/aihelp/reference` |
| Menu | bare `le` at a terminal | derived tree to `cli/menu` model to the chosen action, through prompting when a required parameter is missing, then normal dispatch | `internal/component/cli/menu` |
| Prompt | an action missing a required parameter at a terminal | `leaction` reports the missing parameters to the prompt, which completes from `Source`, echoes the line, then re-dispatches the completed argv | `internal/component/cli/menu` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `le '|' json` | → | `leroot.Manifest` carrying typed parameters from `leaction.Parameter` | `TestManifestCarriesTypedParameters` (`internal/le/le/root/manifest_test.go`) |
| derived tree build | → | `leroot.Tree` from `command.ListLocal` and `leroot.ActionsOf` | `TestDerivedTreeHoldsEveryRegisteredAction` (`internal/le/le/root/tree_test.go`) |
| `le help` on a pipe | → | `Dispatch` root page renderer, stdout | `test/ui/le-help-pages.ci` with fixture `ui/le-help-pages` |
| `le spec claim` on a pipe | → | one-action usage refusal | `TestMissingRequiredParameterOnAPipeRefusesWithOneUsage` (`internal/le/le/action/usage_test.go`) |
| `le commit create --help` | → | action page from the `commit` table | `TestCommitCreateHelpRendersEveryLiveKeyword` (`internal/le/commit/actions_test.go`) |
| `le test stress-repro run --help` | → | action page from the registered table | `TestStressReproRunHelpComesFromTheTable` (`internal/le/test/stressrepro/actions_test.go`) |
| every registered area | → | `leaction.Area` dispatch | `TestEveryRegisteredAreaProvidesActionsOrIsOnTheMigrationList` (list empty, exemption deleted), renamed `TestEveryRegisteredAreaProvidesActions` (`internal/le/actions_test.go`) |
| a retired spelling | → | `internal/le/le/retired` read by `Dispatch` | `TestRetiredSpellingNamesItsSuccessor` (`internal/le/le/root/dispatch_test.go`) |
| a usage error vs a found problem vs could-not-run | → | exit codes 2, 1, 3 | `TestAUsageErrorAndAnExecutionFailureAnswerDifferentCodes` (`internal/le/exitcode_test.go`) |
| `le completion bash` in bash at a pty | → | `internal/plugins/completion` generator, `le completion words`, `command.TreeCompleter` | `test/ui/le-completion-shells.ci` with fixture `ui/le-completion-shells` |
| `le completion words` | → | callback skipping hooks | `TestCompletionCallbackRunsNoBeforeDispatchHook` (`internal/le/completion/words_test.go`) |
| `le help ai | json` | → | `aihelp/reference` from the derived tree | `TestHelpAiJSONListsEveryAreaWithTypedParameters` (`internal/le/le/root/helpai_test.go`) |
| `ze help ai` | → | `aihelp/reference` renderer | `ze-help-ai-unchanged` (`test/ui/ze-help-ai-unchanged.ci`) |
| bare `le` at a pty | → | `internal/component/cli/menu` over the derived tree | `test/ui/le-menu-and-prompt.ci` with fixture `ui/le-menu-and-prompt` |
| `ze le help ai | json` | → | same tree, program `ze le` | `TestZeLeParity` (`internal/le/le/root/parity_test.go`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `le '|' json` | every action's every parameter carries `kind`, `help`, `required`, `repeat`, and `values` for an enum; no parameter has an empty `help` |
| AC-2 | the tree after Phase 1 | `spec-le-every-area-dispatches-through-one-table.md` and `spec-le-one-verb-one-job.md` are absent; `le spec citation` reports no citation of either |
| AC-3 | `le help` and `le --help` on a pipe | the root page on stdout, nothing on stderr, exit 0; bytes equal bare `le` on a pipe |
| AC-4 | the root page | one block per namespace (first word) plus one block for single-word areas; every line is name, purpose tag in brackets, first sentence of the description; no group heading lines; every registered area appears exactly once |
| AC-5 | `le spec --help` | every member of `spec` with tag and description on stdout, exit 0 |
| AC-6 | `le commit create --help` (carried one-table AC-1) | every live keyword (20) with kind and help; `subject` marked required; `body`, `file`, `file-list`, `remove`, `remove-list` marked repeatable; the three retired keywords (`push`, `unverified`, `missing-full-verify-ok`) absent; exit 0 |
| AC-7 | `le spec claim --help`, `le job run --help` | `claim` shows its positional `<spec>` of kind spec, not an action named `spec <spec>`; `job run` shows `label`, `quiet`, `command` as keywords, not one usage line as an action |
| AC-8 | `le help commit create subject` | the keyword's kind, required flag, repeat flag and help, exit 0; `le help commit create nosuch` refuses naming `commit create`'s keywords, usage code |
| AC-9 | `le spec claim` with no spec, on a pipe | stderr carries `usage: le spec claim <spec>` and `claim`'s keyword lines only; no other `spec` member is named; usage code |
| AC-10 | any page rendered on a pipe (carried one-verb AC-6) | no line exceeds 100 columns unless it is one unbreakable token; continuation lines start at the description column |
| AC-11 | a `Why` of several sentences | the listing shows its first sentence; the action page shows the whole text |
| AC-12 | `ze le spec claim` on a pipe | usage names `ze le spec claim`, never `le spec claim` |
| AC-13 | `le verify --help` | the area description names the namespace's job, not `verify worktree`'s |
| AC-14 | each of the 30 formerly hand-rolled areas, bare (carried one-table AC-2) | answers its action listing in the shape every tabled area answers; a single-gate area runs its gate when bare and lists the same actions under `help` (D-P3-bare) |
| AC-15 | `le '|' json` (carried one-table AC-3) | names every registered area's actions, none exempt; each forwarding area carries its declared grammar or `passes-words: true` |
| AC-16 | `internal/le/actions_test.go` (carried one-table AC-4) | `areasWithoutAnActionTable` and its exemption branch are gone; le's first `Dispatch` or `Tree` panics naming any area with no action table and the call that fixes it (D-P3-tablecheck) |
| AC-17 | every refusal case of every migrated area, through Phase 3 (carried one-table AC-5) | the same exit code as before its migration, except a refusal the table's parser makes, which exits 2 (D-P3-stray, D-P3-dash) |
| AC-18 | `le test stress-repro run --help` (carried one-table wiring) | renders the run grammar from the registered table |
| AC-19 | `le help ai commands` for a forwarding area | marks it as forwarding words to its program, with its declared grammar when it has one |
| AC-20 | `le spec claim <file>` and `le spec claim spec <file>` | the first claims; the second is refused naming `le spec claim <file>`; `le cli grammar` passes |
| AC-21 | each old `write` spelling (ten areas), `verify status write`, `worktree update` (carried one-verb AC-1, AC-2) | refused naming the new spelling; the new spelling runs the same job; no area registers `write` |
| AC-22 | `le test cymru` and every other moved mock | refused naming `le mock <name>`; `le mock <name>` serves as before |
| AC-23 | `le help`, `le mock --help`, completion of `le mock ` | the four spawned helpers are absent; `le help ai commands | json` lists them with `hidden: true` |
| AC-24 | `le test --help` | `fixture`, `peer`, `mcp`, `replay` carry the `tool` tag; suites carry `suite` |
| AC-25 | `le doc check retired-commands` and the citation resolver after each rename commit (carried one-verb AC-3) | zero findings: every `le` invocation cited in the instruction corpus and in Go string literals resolves against the manifest, verb included |
| AC-26 | every registered verb | is in the closed vocabulary; every vocabulary entry is used by at least one action; `le help ai verbs` lists each with its meaning |
| AC-27 | `le ai help`, `le ai --help`, bare `le ai` | the `ai` namespace page; every `le ai hooks <name>` spelling in `.claude/settings.json` still dispatches |
| AC-28 | a usage error in any area, an unknown command, a namespace non-member (carried one-verb AC-4) | exit 2 |
| AC-29 | `le rfc check` with its corpus unreadable, and with a real violation (carried one-verb AC-5) | 3 and 1 respectively |
| AC-30 | the same mistake through `leroot.RefuseArgument` and `leaction.refuseValue` (carried one-verb AC-7) | both exit 2; after D-P3-refuseargument `RefuseArgument` is gone, and every stray-word refusal of every area comes from the table with exit 2 |
| AC-31 | `le commit review-check` not clean | exit 1 |
| AC-32 | `le ai hooks <name>` blocking, a forwarding area's program failing, `le test fixture <name>` failing | 2 for the hook; the program's own code for the others; each exemption is listed by `le help ai exit-codes` |
| AC-33 | bash at a pty after `eval "$(./le completion bash)"` | `./le sp` Tab gives `./le spec `; `./le spec cl` Tab gives `claim`; `./le commit create ` Tab offers live keywords and none of the retired three; `subject` is not offered again once used; an enum keyword's values complete; the same for the word `le` when on PATH |
| AC-34 | zsh at a pty after loading `le completion zsh` | the same four completions as AC-33 |
| AC-35 | fish after sourcing `le completion fish` | `complete -C './le sp'` and `complete -C 'le sp'` offer `spec` |
| AC-36 | `le completion words spec cl` with `bin/le` stale | stdout holds only candidate records (candidate, tab, summary); stderr is empty; no before-dispatch hook ran; with `bin/le` absent the script prints nothing and builds nothing |
| AC-37 | the completion callback, warm binary | median wall time under 100 ms over ten runs |
| AC-38 | `le help ai` | a text summary on stdout naming the sections and `le help ai | json`, exit 0 |
| AC-39 | `le help ai commands | json` | lists every registered area (count equal to the manifest's), each action, each parameter with kind, values, required, repeat and help; `le help ai all | yaml` holds every section; `le help ai nosuch` is a usage error listing the sections |
| AC-40 | `le help ai --json`, `le help ai --ai` | refused as dash-leading tokens, naming `le help ai | json`; usage code |
| AC-41 | `ze help ai`, `ze help ai --json` | byte-identical to their output before this spec |
| AC-42 | bare `le` at a pty | the menu opens over the derived tree; arrows and Enter drill into a namespace and an action; an action with every required parameter given is echoed and run; `q` quits with exit 0 |
| AC-43 | bare `le` with stdin a terminal and stdout a pipe, and the reverse | the root page on stdout, exit 0, no menu |
| AC-44 | `le spec claim` at a pty | prompts for `spec`, Tab completes spec file paths, then echoes `le spec claim <path>` to stderr and runs it |
| AC-45 | an action at a pty with a missing optional parameter only | runs without a prompt |
| AC-46 | Ctrl-C or Ctrl-D at a prompt | nothing runs; exit 2 |
| AC-47 | the whole-tree human gate at a pty | asked exactly as today; no prompt answers it |
| AC-48 | `ze le help ai | json`, `ze le completion words spec cl`, `ze le` at a pty | the same payload as `le` but `program` reads `ze le`; the same candidates; the same menu |
| AC-49 | `le commit create`, `message` or `session` failing to execute (git, the index or the tree refuses the operation) rather than refusing a keyword (carried one-verb finding: commit's `commandError(err, 2)`) | exit 1 when the operation ran and failed, 3 when it could not run; a keyword refusal stays 2 |
| AC-50 | any listing rendered at a terminal of width W, for W at 20, 80 and 500 columns (carried one-verb AC-6, terminal branch) | no line exceeds W unless it is one unbreakable token; below the description column plus 20, the description moves to its own indented line; continuation lines start at the description column |
| AC-51 | every word le accepts, including every word the `./le` launcher script consumes before the binary runs (`--name <name>`, `--name=<name>`, the value `session`, `rollback`), owner rule in `ai/META-DECISIONS.md` "Developer tooling" (D-P4-launcher) | each appears on the help page of its level (`le help` for areas and launcher words, the area page for actions, the action page for keywords and enum values), in `le help ai | json`, and among the completion candidates at its place; the launcher's words are read from the script, so a word added to the script without its declaration turns the test red; `./le rollback newest` is refused naming `./le rollback`, exit 2 |
| AC-52 | every registered action word (D-P4-vocab) | is a vocabulary verb, or a name declaring the vocabulary verb it performs; no vocabulary verb declares another verb; no name has a hyphen part that is a vocabulary verb, `write`, `generate` or `test`; the only words exempt are the `ai hooks` words `.claude/settings.json` calls, and that set equals the settings file's calls; every old spelling in "Phase 4 rename map" is refused naming its successor |
| AC-53 | every namespace of the live tree, a first word areas share that is no area itself (D-P7-namespace) | carries the summary its package registered, in the menu item on le's first screen, as the heading of its block on the root page, on its own page and in `le help ai commands`; a word that is both a namespace and an area shows the area's summary, and a namespace registration for it stops le; le refuses to start while a namespace holds no summary, naming the namespace and the `leroot.RegisterNamespace` call that fixes it |
| AC-54 | every area, pass-through area, action, keyword and declared enum or list value of the live registry (D-P7-describe-all) | carries text: the area's summary, the program a pass-through area's words go to, the action's Why, the keyword's Help, and one line per declared value that completion, the menu, the keyword page and `le help ai commands` print beside the value; le refuses to start while any one is blank, or a value line names a value the keyword does not declare, naming the item, what to fill in and the package it goes in |
| AC-55 | every namespace, area, action, keyword and declared value of the live registry (D-P7-short-long) | declares a short text of one line and at most 60 characters, shown whole by listings, completion and the menu's right column, and every namespace, area and action declares a long description that its page and the header of an entered menu level print; le refuses to start while a short is blank, over 60 or over one line, or a long is missing (a pass-through area excepted); ze's menu is unchanged |

## 🧪 TDD Test Plan

Unit tests run per package through `./le job run label unit-pkg command go test -tags <feature tags> ./<pkg>/...`
(a bare `go test` omits the feature tags). Every `.ci` here is accept-only over a
compiled fixture registered in `internal/test/fixture` (the `le-binary-dispatches`
pattern); pty fixtures reuse the `creack/pty` pattern of
`ui_fixture_display_fill_completion.go` (A-14).

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestManifestCarriesTypedParameters` | `internal/le/le/root/manifest_test.go` | AC-1 | |
| `TestEveryParameterHasAKindAndHelp` | `internal/le/actions_test.go` | AC-1 over every registered action | |
| `TestEveryKindIsDeclaredWithAMeaningAndValidator` | `internal/le/le/action/kind_test.go` | D-Q3 kind set closed | |
| `TestKindValidatorsCheckSyntaxOnly` | `internal/le/le/action/kind_test.go` | R-17: uint, duration, enum refuse bad syntax; path and spec accept a non-existent path | |
| `TestPositionalParameterRules` | `internal/le/le/action/leaction_test.go` | at most one positional, first slot, not of kind string; registration panics otherwise | |
| `TestRetiredKeywordIsRefusedNamingItsSuccessor` | `internal/le/le/action/leaction_test.go` | D-Q3 `Retired` | |
| `TestPairKeywordUsageIsDerived` | `internal/le/le/action/leaction_test.go` | pair arity, usage derived | |
| `TestDerivedTreeHoldsEveryRegisteredAction` | `internal/le/le/root/tree_test.go` | Phase 1 wiring | |
| `TestDerivedTreeMapsKindsToArgDefs` | `internal/le/le/root/tree_test.go` | D-Q3 mapping table | |
| `TestEveryCompletedWordIsAcceptedByDispatch` | `internal/le/le/root/tree_test.go` | R-10 | |
| `TestRootPageIsLaidOutByTree` | `internal/le/le/root/dispatch_test.go` | AC-4 | |
| `TestHelpGoesToStdout` | `internal/le/le/root/dispatch_test.go` | AC-3, AC-5 | |
| `TestOneRendererForEveryPage` | `internal/le/le/root/dispatch_test.go` | `helpNode`'s Subs split gone; AC-7 | |
| `TestCommitCreateHelpRendersEveryLiveKeyword` | `internal/le/commit/actions_test.go` | AC-6 | |
| `TestKeywordHelpPage` | `internal/le/le/root/dispatch_test.go` | AC-8 | |
| `TestMissingRequiredParameterOnAPipeRefusesWithOneUsage` | `internal/le/le/action/usage_test.go` | AC-9, AC-46 off a terminal | |
| `TestListingWrapsUnderTheDescriptionColumn` | `internal/le/le/action/leaction_test.go` | AC-10 | |
| `TestListingWrapsAtTheTerminalWidth` | `internal/le/le/action/leaction_test.go` | AC-50: widths 20, 80 and 500 through the width the renderer is given | |
| `TestCommitExecutionFailureIsNotAUsageError` | `internal/le/commit/actions_test.go` | AC-49 | |
| `TestListingShowsTheFirstSentenceAndThePageTheWhole` | `internal/le/le/action/leaction_test.go` | AC-11 | |
| `TestUsageNamesTheRunningProgram` | `internal/le/le/action/usage_test.go` | AC-12 | |
| `TestVerifyAreaDescribesTheNamespace` | `internal/le/verify/register_test.go` | AC-13 | |
| `TestEveryRegisteredAreaProvidesActions` | `internal/le/actions_test.go` | AC-14, AC-16 (renamed from `TestEveryRegisteredAreaProvidesActionsOrIsOnTheMigrationList`) | |
| `TestAnAreaWithoutATableStopsLeNamingIt` | `internal/le/le/root/leroot_test.go` | AC-16 (D-P3-tablecheck, in place of `TestRegisterWithoutActionsPanics`) | |
| `TestEveryPublishedVerbIsDispatchedThroughItsOwnTable` | `internal/le/actions_test.go` | existing guard, now over 164 areas | |
| `TestMigratedAreaKeepsItsExitCodes` | `internal/le/actions_test.go` | AC-17, a per-area golden captured before each conversion | |
| `TestStressReproRunHelpComesFromTheTable` | `internal/le/test/stressrepro/actions_test.go` | AC-18 | |
| `TestEveryCreateKeywordReadsOptions` | `internal/le/commit/actions_test.go` | existing, kept over the new table | |
| `TestEveryForwardingAreaDeclaresItsGrammarOrPassThrough` | `internal/le/actions_test.go` | AC-15, AC-19 | |
| `TestRetiredSpellingNamesItsSuccessor` | `internal/le/le/root/retired_test.go` | AC-20, AC-21, AC-22, D-P4-machinery: every row of the map refused naming its successor, exit 2; no row shadows a live line | written (part 1) |
| `TestNoAreaRegistersWrite` | `internal/le/actions_test.go` | AC-21 | |
| `TestSpawnedHelpersAreHiddenFromHelpAndCompletion` | `internal/le/le/root/tree_test.go` | AC-23 | |
| `TestEveryRegisteredVerbIsInTheVocabulary` | `internal/le/vocabulary_test.go` | AC-26 | written (part 1), owed list D-P4-owed |
| `TestEveryVocabularyVerbIsUsed` | `internal/le/vocabulary_test.go` | AC-26 | written (part 1), owed list D-P4-owed |
| `TestNoTwoVocabularyVerbsShareAMeaning` | `internal/le/vocabulary_test.go` | R-20 | written (part 1) |
| `TestPhase4OwedListOnlyShrinks` | `internal/le/vocabulary_test.go` | D-P4-owed: a listed finding the tree no longer holds fails | written (part 1) |
| `TestOnlyTheHookAreaFreezesAWord` | `internal/le/vocabulary_test.go` | AC-52, D-P4-frozen: `Frozen` outside `ai hooks` fails | written (part 1) |
| `TestEveryWordTheLauncherConsumesIsInHelp` | `internal/le/le/root/launcher_test.go` | AC-51: reads the consumed words from the repository's `le` script (the `case ${1:-} in` arms, `[ "${1:-}" = <word> ]` and `[ "$name" = <word> ]` tests before the first `exec`); fails when it reads none; script set equals the declared set both ways; each word is on `le help`, in `le help ai launcher | json` and offered as a first-word completion | |
| `TestEveryWordLeAcceptsIsInHelp` | `internal/le/actions_test.go` | AC-51: every registered area, action, keyword and enum value appears on the help page of its level, in `le help ai | json` and among completion candidates; a hidden helper only in `help ai`, marked hidden (AC-23) | |
| `le-launcher-words` | `test/ui/le-launcher-words.ci` | AC-51 through the real launcher: `./le rollback newest` refused naming `./le rollback`, exit 2, nothing moved aside; `./le help` names `--name` and `rollback` | |
| `TestEveryActionWordIsAVerbOrADeclaringName` | `internal/le/vocabulary_test.go` | AC-52, D-P4-vocab: a verb, or a name with `Does` set to a vocabulary verb; a vocabulary verb with a different `Does` is refused | |
| `TestNoNameEmbedsAVerbOrTest` | `internal/le/vocabulary_test.go` | AC-52: no hyphen part of a name is a vocabulary verb, `write`, `generate` or `test` | |
| `TestFrozenHookWordsAreExactlyTheSettingsCalls` | `internal/le/ai/hooks/frozen_test.go` | AC-52, D-P4-frozen: the frozen set equals the `le ai hooks <name>` calls read from `.claude/settings.json` | |
| `TestEveryNamespaceIsDescribedInTheMenu` | `internal/le/namespace_test.go` | AC-53 over the real registry: every namespace's tree node and first-screen menu item carry its registered summary | |
| `TestEveryNamespaceIsDescribedOnTheRootPageAndInHelpAI` | `internal/le/namespace_test.go` | AC-53: `le help` heads each namespace block with its summary's first sentence; `le help ai commands \| json` lists each namespace with its summary | |
| `TestANamespaceWithoutASummaryStopsLeNamingIt`, `TestANamespaceThatIsAlsoAnAreaIsDescribedOnce`, `TestTheTreeAndTheRootPageShowANamespaceSummary` | `internal/le/le/root/namespacesummary_test.go` | AC-53: the refusal names the namespace, the call and its file; a namespace registration for an area is refused; the tree and the root page show the summary, an area's own summary winning | |
| `TestEveryWordLeOffersSaysWhatItDoes` | `internal/le/describe_test.go` | AC-54 over the real registry: every node of `leroot.Tree()` and every keyword definition carries text, and every declared value carries its line (29 today) | |
| `TestEveryItemWithoutTextStopsLeNamingIt` | `internal/le/le/root/describe_test.go` | AC-54: one probe per level (area summary, pass-through program, Why, Help, value line, stray value line), each registering its own table, stops le naming the item, the fix and the package | |
| `TestEveryWordLeOffersSaysWhatItDoes` (extended) | `internal/le/describe_test.go` | AC-55 over the real registry: every short text (node summary, keyword Help, value line) is one line of at most 60 characters, and every node but a pass-through area carries a long description | |
| `TestEveryItemWithoutTextStopsLeNamingIt` (extended) | `internal/le/le/root/describe_test.go` | AC-55: an area summary over 60, an area with no Description, an action with no Summary, a Summary over 60 or over one line, a Help over 60 and a ValueHelp line over 60 each stop le naming the item and the fix | |
| `TestLeMenuArrowsAndEnterDrillToAnAction` (extended) | `internal/le/le/root/menu_test.go` | AC-55: the menu le opens (`menu.NewDeclared`) shows an area's short whole in the right column and not its long before Enter; after Enter the area's long heads the level beside the action's short | |
| `TestRootPageIsLaidOutByTree` (changed) | `internal/le/le/root/group_test.go` | AC-55: a root page line is the declared short whole, never the long | |
| `TestZeMenuViewUnchanged` | `cmd/ze` | AC-55: ze's menu (`menu.New`, `LevelOf`) stays byte-identical | |
| `TestNoCitationNamesAVerbTheRegistryDoesNotHold` | `internal/le/doc/check/citation_test.go` | AC-25 (carried from one-verb) | |
| `TestHelpAiIsTheReferenceAndAiHelpIsTheNamespacePage` | `internal/le/le/root/dispatch_test.go` | AC-27, R-15 | |
| `TestAUsageErrorAndAnExecutionFailureAnswerDifferentCodes` | `internal/le/exitcode_test.go` | AC-28, AC-29 | |
| `TestRefuseArgumentAndRefuseValueAgree` | `internal/le/le/root/leroot_test.go` | AC-30 | |
| `TestReviewCheckNotCleanExitsOne` | `internal/le/commit/actions_test.go` | AC-31 | |
| `TestHookActionsKeepTheHookProtocolCodes` | `internal/le/ai/hooks/exitcode_test.go` (new) | AC-32, R-13 | |
| `TestPassThroughAreasKeepTheirProgramsCode` | `internal/le/exitcode_test.go` | AC-32, R-14 | |
| `TestZeCompletionScriptsUnchanged` | `internal/plugins/completion/scripts_test.go` | R-12, goldens captured from HEAD before Phase 6 | |
| `TestLeCompletionScriptRegistersBothSpellings` | `internal/plugins/completion/scripts_test.go` | AC-33, AC-34, AC-35 script side | |
| `TestCompletionCallbackRunsNoBeforeDispatchHook` | `internal/le/completion/words_test.go` | AC-36 | |
| `TestCompletionCallbackHonoursRepeatAndRetired` | `internal/le/completion/words_test.go` | AC-33 | |
| `TestCompletionCallbackAnswersWithinBudget` | `internal/le/completion/budget_test.go` | AC-37: builds the le personality from the tree into a private directory, as the functional fixtures build their subject (D-P6-budget: the shared `bin/le` can predate the change under test, so measuring it judged the wrong binary), warms it once, runs it ten times, median under 100 ms | |
| `TestHelpAiTextSummary` | `internal/le/le/root/helpai_test.go` | AC-38 | |
| `TestHelpAiJSONListsEveryAreaWithTypedParameters` | `internal/le/le/root/helpai_test.go` | AC-39 | |
| `TestHelpAiRefusesRenderingFlags` | `internal/le/le/root/helpai_test.go` | AC-40 | |
| `ze-help-ai-unchanged` | `test/ui/ze-help-ai-unchanged.ci` | AC-41, goldens captured from `dbf0dbd866^` under the runner's tags (D-P6-golden-fixture) | |
| `TestMenuModel_*` (ten tests, moved) | `internal/component/cli/menu/menu_test.go` | R-11, assertions unchanged | |
| `TestLeMenuItemsComeFromTheTree` | `internal/le/le/root/menu_test.go` | AC-42 | |
| `TestLeMenuArrowsAndEnterDrillToAnAction` | `internal/le/le/root/menu_test.go` | AC-42 (arrows and Enter drill into an area and choose an action) | |
| `TestBareLeOnAPipePrintsTheRootPage` | `internal/le/le/root/dispatch_test.go` | AC-43 | |
| `TestPromptCompletesFromTheValueSource` | `internal/component/cli/menu/prompt_test.go` | AC-44 | |
| `TestOptionalParametersAreNeverPrompted` | `internal/le/le/root/prompt_test.go` | AC-45 | |
| `TestPromptCancelRunsNothing` | `internal/component/cli/menu/prompt_test.go` | AC-46 | |
| `TestHumanGateIsNotAnsweredByAPrompt` | `internal/le/le/human/human_test.go` | AC-47 | |
| `TestZeLeParity` | `internal/le/le/root/helpai_test.go` | AC-48, `help ai` half; the completion half is `le-completion-shells` (`ze le completion words`), the menu half Phase 7 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| uint kind value | 0 to max uint64 | 18446744073709551615 | -1 (refused, usage code) | 18446744073709551616 (refused, usage code) |
| wrap width off a terminal | fixed 100 | a 100-column line unbroken | N-A | a 101-column line wraps (one unbreakable token excepted) |
| completion budget | under 100 ms median | 99 ms | N-A | 100 ms fails AC-37 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `le-help-pages` | `test/ui/le-help-pages.ci`, fixture `ui/le-help-pages` | `bin/le help`, `--help`, `spec --help`, `commit create --help`, `help commit create subject`, `spec claim` on a pipe: AC-3 to AC-9, AC-12 through `ze le` | |
| `le-retired-spellings` | `test/ui/le-retired-spellings.ci`, fixture `ui/le-retired-spellings` | every Phase 4 old spelling refused naming its successor; new spelling runs: AC-20 to AC-22 | |
| `le-exit-codes` | `test/ui/le-exit-codes.ci`, fixture `ui/le-exit-codes` | usage, found problem, could not run, exemptions, through the binary: AC-28 to AC-32 | |
| `le-completion-shells` | `test/ui/le-completion-shells.ci`, fixture `ui/le-completion-shells` | bash and zsh in a pty load the script and press Tab; fish answers `complete -C`: AC-33 to AC-36; refuses naming the missing shell and its install command (A-13) | |
| `le-help-ai` | `test/ui/le-help-ai.ci`, fixture `ui/le-help-ai` | `le help ai`, `| json`, `| yaml`, sections, refused flags; `ze le` parity: AC-38 to AC-40, AC-48 | |
| `le-menu-and-prompt` | `test/ui/le-menu-and-prompt.ci`, fixture `ui/le-menu-and-prompt` | bare `le` in a pty opens the menu, drills, runs, quits; `le spec claim` in a pty prompts, completes, echoes and runs; Ctrl-C runs nothing; half-terminal cases print the page: AC-42 to AC-46 | |
| `root-completion` (existing) | `test/ui/root-completion.ci` | ze's completion unchanged after the generator refactor | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | Scope is tooling | N-A | N-A | |

## Files to Modify
<!-- Research list below, with DESIGN's additions after it. -->
- `internal/le/le/action/leaction.go` - typed parameters, wrapped listing, one refusal code, program name in usage
- `internal/le/le/root/dispatch.go`, `leroot.go`, `manifest.go`, `group.go` - one page format, help to stdout when asked, tree layout with purpose tags, `help ai`, mandatory tables, `RefuseArgument` code
- `internal/le/commit/actions.go` and the 29 other areas in `areasWithoutAnActionTable` - onto the table
- `internal/le/spec/actions.go` and `internal/le/spec/claim/` - flattened grammar, `spec claim <file>`
- the `write` packages (`yang/glue`, `web/assets`, `plugin/imports`, `repo/featuretags`, `data/*`, `doc/index`, `plugin/gatedtable`, `ai/sync`, `verify/status`, judged per A-12)
- `internal/le/test/<helper>/` packages moved per Q-10
- `internal/le/verify/register.go` - correct area description
- `internal/le/actions_test.go` - migration list emptied, exemption deleted
- `internal/le/doc/check/retirednames.go` - one row per rename
- `internal/plugins/completion/` and/or the shared library per Q-2; `internal/component/aihelp/` if its program-neutral part is shared
- `cmd/ze/help_ai.go`, `cmd/ze/tui_menu.go` per Q-1, Q-4
- `le` launcher per Q-7
- every consumer in the Consumer surface table

DESIGN additions and resolutions:

| File | Phase | Change |
|------|-------|--------|
| `plan/post-release/engineering/spec-le-every-area-dispatches-through-one-table.md`, `spec-le-one-verb-one-job.md` | 1 | removed through `./le commit create` `remove`; every citation repointed |
| `internal/le/le/action/leaction.go` | 1, 2, 5 | Parameter fields (D-Q3), kind set, verb vocabulary (D-Q9), one-action usage, wrap, first sentence, program name, exit codes |
| `internal/le/le/root/dispatch.go` | 2, 4, 5, 6, 7 | help to stdout, one renderer, `help <path> <keyword>`, retired check, exit 2 for unknown, `help ai` word, menu at a terminal |
| `internal/le/le/root/group.go` | 2 | one declared group table with tag and meaning; `tool` group |
| `internal/le/le/root/leroot.go` | 3, 5, 6 | `RegisterActions` mandatory, `RefuseArgument` 2, shell-callback registration flag |
| `internal/le/le/root/manifest.go` | 1, 6 | typed parameters, hidden marker, exit-code exemptions |
| `internal/le/register.go` | 6 | `run` skips cache link and before-dispatch hooks for a shell-callback area |
| `internal/le/doc/check/retirednames.go` | 4 | its table moves to `internal/le/le/retired`; the check reads it from there |
| `internal/le/rfc/check.go`, `internal/le/cli/grammar/` (the "tree the gate could not read" producer the one-verb skeleton called `cligrammar/actions.go`), `internal/le/spec/status/answer.go`, `internal/le/commit/actions.go` (`review-check`, and the `create`, `message` and `session` execution failures of AC-49) | 5 | exit codes per D-Q8 |
| `internal/le/test/{cymru,dns,irr,peeringdb,rpki,rtrmock,radiusmock,tacacsmock,syslog,httpd,lg,enginesteps,recordplugin,textplugin,pluginexternal}/` | 4 | move to `internal/le/mock/<name>/` |
| `internal/le/test/fixture/register.go`, `internal/le/test/peer/register.go` and the `mcp`, `replay` registrations | 4 | group `tool` |
| `internal/le/worktree/actions.go`, `internal/le/verify/status/answer.go`, the ten `write` packages of A-12 | 4 | verb renames |
| `internal/plugins/completion/main.go`, `bash.go`, `zsh.go`, `fish.go`, `words.go` | 6 | program descriptor; ze fragment lifted out unchanged |
| `internal/component/aihelp/aihelp.go` | 6 | `Build` fills the shared reference |
| `cmd/ze/help_ai.go` | 6 | renders through `aihelp/reference`; sections, `--json` and `--ai` untouched (R-19) |
| `cmd/ze/tui_menu.go`, `cmd/ze/tui_menu_test.go` | 7 | model moves out; builders stay |
| `.claude/settings.json` | - | not modified (D-hooks) |
| `internal/test/fixture/` | 2, 4, 5, 6, 7 | the six new compiled fixtures |
| `test/**/*.ci` citing moved mocks | 4 | mechanical rewrite, one commit per moved helper (R-5) |
| Docs: `docs/contributing/running-commands.md`, `docs/contributing/committing.md`, `docs/architecture/core-design.md`, `docs/architecture/cli/root-namespace-grammar.md`, `docs/architecture/cli/tui-launcher.md`, `docs/guide/command-reference.md`, `docs/functional-tests.md`, `docs/architecture/testing/ci-format.md`, `ai/INDEX.md`, `ai/patterns/cli-command.md`, `ai/doors/le-tooling.md` | the phase that changes each | per the Documentation Update Checklist |
| Docs named by the anchor audit (`./le spec citation anchors`): `docs/architecture/config/environment.md` (retirednames), `docs/architecture/system-architecture.md` (dispatch), `docs/architecture/testing/interop.md` (leaction), `docs/contributing/claude-code-cheatsheet.md` (commit), `docs/features/ai-first.md` (commit, help_ai), `docs/features/introspection.md` (help_ai) | the phase that changes the anchored file | each re-read and corrected if the change makes it wrong |

## Files to Create

| File | Phase | Purpose |
|------|-------|---------|
| `internal/le/le/root/tree.go`, `tree_test.go` | 1 | derived `command.Node` tree |
| `internal/le/le/action/kind.go`, `kind_test.go` | 1 | the closed kind set |
| `internal/le/le/action/verbs.go`, `verbs_test.go` | 4 | the closed verb vocabulary |
| `internal/le/le/action/usage_test.go` | 2 | one-action usage |
| `internal/le/le/retired/retired.go` | 4 | the one rename map |
| `internal/le/exitcode_test.go` | 5 | exit-code discipline across areas |
| `internal/le/completion/register.go`, `words.go`, `words_test.go`, `budget_test.go` | 6 | `le completion` |
| `internal/le/mock/<name>/` (15 packages, moved) | 4 | `le mock` areas |
| `internal/component/aihelp/reference/reference.go`, `reference_test.go` | 6 | shared reference type and renderer |
| `internal/le/le/root/helpai.go`, `helpai_test.go` | 6 | `le help ai` |
| `internal/component/cli/menu/menu.go`, `menu_test.go`, `prompt.go`, `prompt_test.go` | 7 | shared menu and prompt |
| `internal/le/le/root/menu.go`, `menu_test.go`, `prompt_test.go`, `parity_test.go` | 7 | le's menu items, prompting, `ze le` parity |
| `internal/test/fixture/ui_fixture_le_help_pages.go`, `ui_fixture_le_retired_spellings.go`, `ui_fixture_le_exit_codes.go`, `ui_fixture_le_completion_shells.go`, `ui_fixture_le_help_ai.go`, `ui_fixture_le_menu_and_prompt.go` | per functional test | compiled fixtures |
| `test/ui/le-help-pages.ci`, `le-retired-spellings.ci`, `le-exit-codes.ci`, `le-completion-shells.ci`, `le-help-ai.ci`, `le-menu-and-prompt.ci` | per functional test | functional tests |

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | le commands register outside YANG; no daemon RPC |
| YANG validation constraints | N-A | no YANG leaf; typed parameter validation lives in the action table (D-Q3) |
| YANG custom validators | N-A | no YANG leaf; value sources play the `CompleteFn` role (D-6) |
| CLI commands/flags | Yes | le's whole surface; `le help ai`, `le completion`, `le mock`; no flag added (D-Q1) |
| CLI grammar (keyword before value) | Yes | `./le cli grammar` feeder 6 judges every rename, including `spec claim <file>` |
| Editor autocomplete | Yes | shell completion for le, through `command.TreeCompleter` |
| Functional test for new RPC/API | Yes | no RPC; six `.ci` tests over compiled fixtures in `internal/test/fixture` drive the binary, the shells and a pty (TDD plan, Functional Tests) |
| Pipe completeness | Yes | `le help ai` answers a payload through `ApplyPipes`; every migrated area keeps structured payloads |
| Env var registration | N-A | none: wrap width comes from the terminal or the fixed 100 columns, never an env var |
| Doctor check for runtime dependencies | N-A | no new runtime dependency of the shipped product; le is a development tool |
| Prometheus counters/metrics | N-A | no observable runtime state |
| BGP family surface (new SAFI / capability / attribute) | N-A | not protocol work |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | le is development tooling with no `features/<id>.md` entry (`./le feature report` at DESIGN lists no le usability feature); `docs/features/ai-first.md` and `introspection.md` are re-read in Phase 6 because the anchor audit names them |
| 2 | Config syntax changed? | No | no config |
| 3 | CLI command added/changed? | Yes | `docs/contributing/running-commands.md` ("Finding a command and what it takes", completion install), `docs/contributing/committing.md` (commit grammar), `docs/architecture/core-design.md` (le help, groups, namespaces), `docs/architecture/cli/root-namespace-grammar.md` (feeder 6 over the positional rule; no F3 change, D-Q1 adds no flag), `docs/guide/command-reference.md` (`ze completion` if the package is parameterised), `ai/INDEX.md` ("Native command inventory"), `ai/patterns/cli-command.md` (stale `help ai` section) |
| 4 | API/RPC added/changed? | No | no daemon API |
| 5 | Plugin added/changed? | No | `internal/plugins/completion` gains a program descriptor and registers nothing new; ze's `completion` command is unchanged |
| 6 | Has a user guide page? | Yes | `docs/architecture/cli/tui-launcher.md` (the menu model moves, D-Q2); `docs/guide/mcp/overview.md` unchanged because `ze help ai` output is unchanged (AC-41) |
| 7 | Wire format changed? | No | no wire |
| 8 | Plugin SDK/protocol changed? | No | no SDK change |
| 9 | RFC behavior implemented, changed, or newly proven? | No | no RFC |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` and `docs/architecture/testing/ci-format.md` for moved helpers (`le test fixture`, `le test peer`, mocks) |
| 11 | Affects daemon comparison? | No | tooling |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md`, `ai/doors/le-tooling.md`, and `docs/contributing/running-commands.md` for le's verb vocabulary and exit codes (D-Q8, D-Q9) |
| 13 | Route metadata keys added/changed? | No | no route metadata |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | le's command inventory: `ai/INDEX.md` table and the manifest; no ze plugin inventory changes |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `./le spec citation anchors spec plan/post-release/engineering/spec-le-cli-usability.md` at DESIGN named six: `docs/architecture/config/environment.md`, `docs/architecture/system-architecture.md`, `docs/architecture/testing/interop.md`, `docs/contributing/claude-code-cheatsheet.md`, `docs/features/ai-first.md`, `docs/features/introspection.md` (Files to Modify); re-run at each phase for its new files |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | every `./le` example in the Consumer surface; verified by `le doc check retired-commands` and `le doc check verify` |

### Discovery (repo-maintenance mechanical checklist)
| Question | Answer |
|----------|--------|
| Where does an agent look first? | `ai/INDEX.md`, "Native command inventory", which will name `le help ai` as the authority beside `./le '|' json` |
| What rule prevents regression? | `ai/rules/cli.md` (tab-completion for every command, no aliases), plus the closed verb vocabulary in `leaction` (D-Q9) |
| What registry or inventory prevents drift? | the action table per area, read by help, completion, prompting, the manifest and `help ai`; `internal/le/le/retired` for old spellings |
| What verification proves it? | `TestEveryRegisteredAreaProvidesActions`, `TestEveryPublishedVerbIsDispatchedThroughItsOwnTable`, `TestEveryRegisteredVerbIsInTheVocabulary`, `TestEveryCompletedWordIsAcceptedByDispatch`, `le doc check retired-commands`, `le cli grammar` |

## Implementation Steps

Order fixed by D-Q6. Each phase: write its tests, see them fail, implement, see
them pass, update its pages, run its scoped package tests and named `.ci` under
`./le job run`, then one commit (Phases 3 and 4 may be several). Gates the main
thread or a fresh agent owes per phase: `./le go lint run` scoped to the changed
packages, `./le cli grammar`, `./le doc check retired-commands` (Phase 4 on),
`./le spec citation`, and `./le verify changed` over the phase's commits.

1. **Phase 1: Wiring (MANDATORY FIRST)** -- skeletons out, typed model, derived tree
   - Inputs: the two superseded skeletons, `leaction.Parameter`, `command.ListLocal`, `leroot.ActionsOf`
   - Tests: `TestManifestCarriesTypedParameters`, `TestEveryParameterHasAKindAndHelp`, `TestEveryKindIsDeclaredWithAMeaningAndValidator`, `TestKindValidatorsCheckSyntaxOnly`, `TestPositionalParameterRules`, `TestRetiredKeywordIsRefusedNamingItsSuccessor`, `TestPairKeywordUsageIsDerived`, `TestDerivedTreeHoldsEveryRegisteredAction`, `TestDerivedTreeMapsKindsToArgDefs`, `TestEveryCompletedWordIsAcceptedByDispatch`
   - Files: `leaction.go`, `kind.go`, `tree.go`, `manifest.go`; every tabled area's Parameter literals gain kind and help (78 areas)
   - Output: every tabled action's parameters typed and described; the tree builds; exit codes and dispatch unchanged
   - Verify: AC-1, AC-2
2. **Phase 2: Help pages** -- one renderer at every level
   - Inputs: Phase 1 tree and kinds
   - Tests: `TestRootPageIsLaidOutByTree`, `TestHelpGoesToStdout`, `TestOneRendererForEveryPage`, `TestKeywordHelpPage`, `TestMissingRequiredParameterOnAPipeRefusesWithOneUsage`, `TestListingWrapsUnderTheDescriptionColumn`, `TestListingWrapsAtTheTerminalWidth`, `TestListingShowsTheFirstSentenceAndThePageTheWhole`, `TestUsageNamesTheRunningProgram`, `TestVerifyAreaDescribesTheNamespace`, `le-help-pages`
   - Files: `dispatch.go` (`helpNode` deleted), `group.go`, `leaction.go`, `spec/actions.go` (`refuseLifecycle` deleted), `verify/register.go`; pages `running-commands.md`, `core-design.md`
   - Constraint: exit codes unchanged (a one-action usage keeps the code its producer answers today); goldens updated in the same commit (R-4)
   - Verify: AC-3 to AC-13 and AC-50 (AC-6 and AC-7 for `commit` and `job` land in Phase 3, where those areas reach the table)
3. **Phase 3: Every area on the table** -- the 30 hand-rolled areas, `commit` last
   - Inputs: each area's parser, read first (A-1, A-2); `keywordRule` mapping (A-5)
   - Tests: `TestEveryRegisteredAreaProvidesActions`, `TestAnAreaWithoutATableStopsLeNamingIt`, `TestEveryPublishedVerbIsDispatchedThroughItsOwnTable`, `TestMigratedAreaKeepsItsExitCodes`, `TestStressReproRunHelpComesFromTheTable`, `TestCommitCreateHelpRendersEveryLiveKeyword`, `TestEveryCreateKeywordReadsOptions`, `TestEveryForwardingAreaDeclaresItsGrammarOrPassThrough`
   - Files: the 30 areas of `areasWithoutAnActionTable`, `commit/actions.go` (`commandVerbs`, `parseKeywords`, `commandError` deleted before the table replaces them, `ai/rules/no-layering.md`), the 56 forwarding registrations, `actions_test.go`, `leroot.go`; page `committing.md`
   - Output: the migration list deleted, `RegisterActions` mandatory, exit codes byte-identical (AC-17)
   - Verify: AC-6, AC-7, AC-14 to AC-19
4. **Phase 4: Renames** -- through the one rename map
   - Inputs: `retirednames.go` moved to `internal/le/le/retired`; the Consumer surface table
   - Tests: `TestRetiredSpellingNamesItsSuccessor`, `TestNoAreaRegistersWrite`, `TestSpawnedHelpersAreHiddenFromHelpAndCompletion`, `TestEveryRegisteredVerbIsInTheVocabulary`, `TestEveryVocabularyVerbIsUsed`, `TestNoTwoVocabularyVerbsShareAMeaning`, `TestNoCitationNamesAVerbTheRegistryDoesNotHold`, `TestHelpAiIsTheReferenceAndAiHelpIsTheNamespacePage` (namespace half), `TestEveryWordTheLauncherConsumesIsInHelp`, `TestEveryWordLeAcceptsIsInHelp`, `TestEveryActionWordIsAVerbOrADeclaringName`, `TestNoNameEmbedsAVerbOrTest`, `TestFrozenHookWordsAreExactlyTheSettingsCalls`, `le-retired-spellings`, `le-launcher-words`
   - Renames: every row of "Phase 4 rename map" (113 spellings), the vocabulary of "Verb vocabulary" declared in `leaction`, and `Does` declared on every name (D-P4-vocab, D-P4-minimal); in summary `spec claim spec <file>` to `spec claim <file>`; ten `write` to `update`; `verify status write` to `verify status record`; `worktree update` to `worktree rebase`; the 15 mocks and helpers to `le mock <name>`; synonym verbs and compound words per the map (R-20); launcher words declared and `./le rollback` refusing trailing words (D-P4-launcher, AC-51, files: the `le` script, `leroot`, `internal/le/rollback`); `.claude/settings.json` untouched (D-P4-frozen); `ai` namespace page reachable as `le ai help`; `setup docker-host` and `setup docker-kernel` to `setup docker host` and `setup docker kernel` (rule R9 of `le cli grammar`: two siblings sharing the left word `docker` make it a namespace; package move to `internal/le/setup/docker/{host,kernel}`, the `internal/le/interoplab` dockerlab argv, workflows, `workflowcheck` and docs; clears functional `le-checks-answers`)
   - Constraint: each rename's old spelling is deleted and its row added in the same commit as every consumer's update; `le ai hooks` untouched (D-hooks); `.ci` rewrites one commit per moved helper (R-5)
   - Files: per rename; pages `running-commands.md` (verb vocabulary section), `functional-tests.md`, `ci-format.md`, `ai/INDEX.md`
   - Verify: AC-20 to AC-27, AC-51, AC-52; `le doc check retired-commands` zero findings
5. **Phase 5: Exit codes**
   - Inputs: the reader grep re-run against HEAD (A-6)
   - Tests: `TestAUsageErrorAndAnExecutionFailureAnswerDifferentCodes`, `TestRefuseArgumentAndRefuseValueAgree`, `TestReviewCheckNotCleanExitsOne`, `TestHookActionsKeepTheHookProtocolCodes`, `TestPassThroughAreasKeepTheirProgramsCode`, `TestCommitExecutionFailureIsNotAUsageError`, `TestMissingRequiredParameterOnAPipeRefusesWithOneUsage` (moved from Phase 2, D-P2-required), `le-exit-codes`
   - Files: `leaction.go`, `leroot.go`, `dispatch.go`, `rfc/check.go`, `cli/grammar/`, `spec/status/answer.go`, `commit/actions.go`, every `_test.go` asserting a changed code; pages `running-commands.md` (exit-code table) and `core-design.md` (its "unknown first word exits 1" and "non-member exits 1" sentences become 2)
   - Verify: AC-28 to AC-32, AC-49
6. **Phase 6: Completion and `le help ai`**
   - Inputs: goldens of ze's three scripts and `ze help ai` output captured from HEAD before any edit
   - Tests: `TestZeCompletionScriptsUnchanged`, `TestLeCompletionScriptRegistersBothSpellings`, `TestCompletionCallbackRunsNoBeforeDispatchHook`, `TestCompletionCallbackHonoursRepeatAndRetired`, `TestCompletionCallbackAnswersWithinBudget`, `TestHelpAiTextSummary`, `TestHelpAiJSONListsEveryAreaWithTypedParameters`, `TestHelpAiRefusesRenderingFlags`, `ze-help-ai-unchanged`, `le-completion-shells`, `le-help-ai`, `root-completion`
   - Files: `internal/plugins/completion/*`, `internal/le/completion/*`, `internal/le/register.go`, `leroot.go`, `aihelp/reference/*`, `aihelp/aihelp.go`, `cmd/ze/help_ai.go`, `helpai.go`; pages `running-commands.md` (install), `command-reference.md`, `ai/patterns/cli-command.md`, `ai/INDEX.md`, `docs/features/ai-first.md`, `docs/features/introspection.md`
   - Verify: AC-33 to AC-41, AC-48 (help ai and completion half)
7. **Phase 7: Menu and prompting**
   - Inputs: `cmd/ze/tui_menu.go` model; `internal/le/le/human` untouched
   - Tests: `TestMenuModel_*` (moved), `TestLeMenuItemsComeFromTheTree`, `TestLeMenuArrowsAndEnterDrillToAnAction`, `TestBareLeOnAPipePrintsTheRootPage`, `TestPromptCompletesFromTheValueSource`, `TestOptionalParametersAreNeverPrompted`, `TestPromptCancelRunsNothing`, `TestHumanGateIsNotAnsweredByAPrompt`, `TestZeLeParity`, `le-menu-and-prompt`
   - Files: `internal/component/cli/menu/*`, `cmd/ze/tui_menu.go`, `cmd/ze/tui_menu_test.go`, `menu.go`, `dispatch.go`; page `tui-launcher.md` (source anchor moves), `running-commands.md`
   - Verify: AC-42 to AC-48

### Triple Challenge

| Challenge | Answer |
|-----------|--------|
| Simplicity | Yes. Every surface derives from one declaration (`leaction.Parameter`) through one projection (the derived tree). Rejected simpler options each add a second declaration (a le completion table, copied scripts, a copied menu). The prompt reuses the menu's terminal model instead of adding a line editor |
| Uniformity | Yes. Completion follows ze's mechanism (static script, `completion words` callback, `writeCompletionRecord`); help ai follows `ze help ai`'s generated-from-registries contract; renames follow `retirednames.go`; the menu is ze's menu. New patterns: the shell-callback registration flag (the only way a keystroke avoids before-dispatch work) and the positional parameter rule (needed by D-3's `spec claim <file>`) |
| Performance | No wire or hot path. The derived tree is built lazily, once per process, only by help, completion, menu, prompt and `help ai`; dispatch never builds it. The completion callback's budget is measured (AC-37) against a 62 to 69 ms warm baseline |

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every AC-1..AC-48 has an implementation at file:line and its named test |
| Correctness | no handler is called to learn a grammar (help, completion, menu, prompt, help ai); every prompt and menu path requires stdin AND stdout terminals |
| Naming | JSON keys kebab-case (`passes-words`, `exit-codes`); verbs from the closed vocabulary only |
| Data flow | dispatch parses only through `leaction`; the derived tree is read-only |
| Rule: `ai/rules/no-layering.md` | every replaced parser and old spelling deleted before its replacement; no alias, no fallback |
| Rule: `ai/rules/cli.md` | no rendering flag, no alias, completion for every le command |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| migration list gone | `grep -n areasWithoutAnActionTable internal/le/actions_test.go` finds nothing |
| skeletons gone | `ls plan/post-release/engineering/spec-le-every-area-dispatches-through-one-table.md plan/post-release/engineering/spec-le-one-verb-one-job.md` fails |
| completion | `bin/le completion bash` exits 0 and the script names `./le` |
| help ai | `bin/le help ai commands | json` area count equals `bin/le '|' json` area count |
| menu package | `ls internal/component/cli/menu/menu.go`; `cmd/ze/tui_menu.go` holds no model type |
| renames | `./le doc check retired-commands` reports zero findings |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | kind validators refuse malformed values before a handler runs; the completion callback treats every word as data and never executes or expands it |
| Shell script injection | the generated scripts quote every word passed to the callback; candidate records are printed as data, never `eval`ed |
| Resource exhaustion | a dynamic value source is bounded (it lists files or registry entries, never walks the whole tree on a keystroke) |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood, back to RESEARCH |
| Lint failure | Fix inline. If architectural, back to DESIGN |
| Functional test fails | Check the AC: wrong AC, back to DESIGN; correct AC, IMPLEMENT |
| A Phase 3 area takes a positional or emits text only (A-1, A-2 broken) | declare it with the positional rule or add a `Text` method; never keep its parser |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- Declaring a table is not the deliverable; dispatching through it is (carried from the one-table skeleton's third-group findings, where `ste`, `worktree`, `functional`, `doc wiring` and `test-chaos` published grammars they did not honour).

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `committed-tree-does-not-build.md` | 2026-10-25 | `internal/le/register.go` at HEAD |
| `reference-output-depends-on-map-order.md` | 2026-10-25 | `internal/component/aihelp` `matchEnvVars`, RPC order |
| `test-shares-a-process-global.md` | 2026-10-25 | `internal/le/le/root` table-less probe tests, run alone |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Extend `leaction.Parameter`, derive a read-only `command.Node` tree | replace with ArgDef; a le-only completion table | one declaration; ArgDef keeps its YANG invariants (D-Q3) |
| `le help ai | json` through the pipe layer | `--json`, `--ai` | `ai/rules/cli.md` bans rendering flags and aliases (D-Q1) |
| Parameterise `internal/plugins/completion` with a program descriptor and per-program fragment | copy the scripts; complete through ze | one generator; ze's bytes pinned by goldens (D-Q2, A-9) |
| Move the menu model to `internal/component/cli/menu` | copy; build tags in package main | one menu in both binaries (D-Q2) |
| Completion script calls `<dir>/bin/le` directly; area skips hooks | launcher special case | no second declaration of the command name; no build on a keystroke (D-Q7) |
| Exit codes 0, 1, 2, 3 with declared exemptions | sysexits | keeps 2-means-usage; the hook protocol and helpers keep their codes (D-Q8) |
| `le mock <name>`; `fixture`, `peer`, `mcp`, `replay` stay with a `tool` tag | move every helper | about 2,370 `.ci` lines saved for no reader gain (D-Q10) |
| Rename map moves to `internal/le/le/retired`, read at runtime too | keep it in `doc/check` only | one declaration answers the scan and the runtime refusal (D-retired) |

## Known Limitations
- `ze help ai --json` and ze's `--ai` alias keep contradicting `ai/rules/cli.md` after this spec; R-19 carries the journal row candidate, and its owning spec is named by the main thread.
- fish registers completion by command name, so `./le` in fish relies on fish's own path handling; the script's install line states the exact behaviour Phase 6 observes.
- A value literally spelled `spec` after `le spec claim` is refused as the retired form (D-retired); a spec path always ends in `.md`, so no real value is lost.

## Goal Validation (BLOCKING)

<!-- Evidence is filled at closure. Rows are fixed now, one per goal in Task and D-1 to D-6. -->
| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| le has shell completion for bash, zsh and fish through ze's mechanism (D-1) | functional, pty | `test/ui/le-completion-shells.ci` output; `TestZeCompletionScriptsUnchanged`; `TestCompletionCallbackAnswersWithinBudget` timing |
| correct and complete help at every level (D-1, D-5) | functional | `test/ui/le-help-pages.ci` output; `TestRootPageIsLaidOutByTree`; `TestCommitCreateHelpRendersEveryLiveKeyword` |
| interactive filling of missing arguments at a terminal, never on a pipe (D-1) | functional, pty | `test/ui/le-menu-and-prompt.ci` output; `TestMissingRequiredParameterOnAPipeRefusesWithOneUsage` |
| self-documentation through `le help ai` in ze's shape (D-1, D-4) | functional | `test/ui/le-help-ai.ci` output with the area count equal to the manifest's |
| every area on the action table, typed grammar published (D-2, D-6) | unit | `TestEveryRegisteredAreaProvidesActions` with no migration list; `TestEveryParameterHasAKindAndHelp` |
| one verb per job, renames with no alias (D-2, D-3) | functional | `test/ui/le-retired-spellings.ci`; `./le doc check retired-commands` zero findings; `TestEveryRegisteredVerbIsInTheVocabulary` |
| one meaning per exit code (D-2) | functional | `test/ui/le-exit-codes.ci` output |
| shared implementation over copies (D-6) | unit | `TestNormalZeLinksNoInternalLe` green; `TestMenuModel_*` run from `internal/component/cli/menu`; `TestZeLeParity` |
| a human developer finds le usable, matching ze (goal) | functional, pty | the three pty runs above, recorded at closure with their transcripts |

## Review Gate

<!-- BLOCKING (ai/rules/planning.md). Filled by /ze-close through /ze-review
     before closure; left empty at design. -->

| Field | Value |
|-------|-------|
| Artifact | |
| `./le spec review check` | |
| Rounds | |
| Reviewer lenses used | |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test (N-A when Scope is tooling or docs, which delete that section)
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches, and `./le verify changed` over the closing commits (`/ze-close` step 6f). The full verification is Thomas's release check: no agent runs it, and one that is owed is handed to him as its exact command. A red there gets a fix commit
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/post-release/engineering/spec-le-cli-usability.md` only, in the same `./le commit create` script (commit A preserves the spec in history)
