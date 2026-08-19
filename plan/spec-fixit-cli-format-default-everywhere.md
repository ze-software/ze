# Spec: cli-format-default-everywhere

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | cli |
| Depends | - |
| Phase | 5/5 |
| Deferral shard | - |
| Handoff | - |
| Updated | 2026-08-19 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`environment cli format default` promises one output format for the CLI. Three
surfaces answer differently, and an operator sees it inside one recording: the
quickstart demonstration runs `ssh ze-demo show bgp summary` and gets indented
JSON, commits `set environment cli format default table`, then runs
`ze cli -c 'show bgp summary'` and gets YAML. Neither command consults the
setting just committed.

Owner decision, 2026-08-18: honor the configured default everywhere.

A second defect sits on the same path. `ssh host 'show bgp peer list | table'`
does not run the pipe. `execMiddleware` hands the whole string to the
dispatcher, and `tokenize` splits on space and tab only, so `|` and `table`
arrive as ordinary command arguments.

Goal: one authority decides the format, every surface obeys it, and an explicit
`--format` flag or format pipe still wins over the configured default.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/cli/color-system.md` - CLI output surfaces
  → Constraint: operator-facing output keeps the semantic role palette.

**Key insights:**
- The daemon is the only process of the pair that holds the config. The client
  cannot resolve the default itself, so the daemon must be the one that formats.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/command/pipe.go` - `configuredDefault` reads
  `ze.cli.format` (registered default `text`) and is reached only through
  `ProcessPipesDefaultFormatChecked` and `ProcessPipesDetectLog`.
- [ ] `internal/component/ssh/ssh.go` - `execMiddleware` joins argv, hands the
  whole string to the executor, prints `result.Output` verbatim. No pipe
  processing and no configured default.
- [ ] `cmd/ze/hub/service_ssh.go` - the executor factory answers
  `params.FormatResponseData(resp.Data)`.
- [ ] `internal/component/bgp/config/loader.go` - `formatResponseData` passes a
  string through and `json.MarshalIndent`s everything else. This is the JSON an
  SSH exec caller sees.
- [ ] `internal/component/cli/client/main.go` - `runBGP` defaults `--format` to
  `yaml`; `Execute` and `executeWithTranscript` call `ProcessPipesChecked`, then
  `renderCommandOutput` and `printFormatted` format locally, whose `default`
  arm is yaml.
- [ ] `internal/component/plugin/server/command.go` - `tokenize` splits on space
  and tab, honors quotes, and gives `|` no meaning.
- [ ] `internal/component/cli/transcript.go` - `(*TranscriptWriter).Record`
  writes the string it is given verbatim.
- [ ] `internal/component/cli/model_mode.go` and
  `internal/component/web/cli_terminal.go` - the two surfaces that already
  honor the default, through the helper this spec reuses.

→ Constraint: nothing on the `ze cli` client startup path (`RunCommand` →
`Run` → `runBGP` → `sshclient.LoadCredentialsWithFlags`) calls
`config.LoadConfig` or `ParseTreeWithYANG`, so `applyParsedEnvironment` never
runs in that process and `env.Get("ze.cli.format")` there can only ever answer
the registered default. The `--remote` branch reads the same blobs. This is why
the daemon formats and the client does not.

**Behavior to preserve:**
- An explicit format pipe (`| json`, `| table`, `| yaml`, `| text`, `| ndjson`)
  wins over the configured default, on every surface.
- `ze cli -c '<cmd>' --format json` keeps answering JSON.
- Empty output still prints `OK` on the no-format-pipe branch.
- The interactive `set cli format <x>` override stays session-scoped and keeps
  winning over the configured default.
- The offline fallback path (`runOfflineFallback`) keeps working with no daemon.
- Streaming commands (`monitor ...`) keep the registered monitor formatter.

**Behavior to change:**
- `ssh host '<cmd>'` answers the configured default format, not indented JSON.
- `ssh host '<cmd> | table'` runs the pipe instead of passing `|` as an argument.
- `ze cli -c '<cmd>'` with no `--format` answers the configured default, not YAML.
- The transcript records what the daemon returned, which is now formatted text
  rather than raw JSON, on the `-c` path. An interactive session's transcript
  keeps the dispatcher's JSON, because the Model renders after the executor.
- `<cmd> | raw` is a new operator answering the dispatcher's JSON unchanged, and
  every in-tree caller that parses an exec-channel answer asks for it through
  `sshclient.ExecCommandRaw`.

## Data Flow (MANDATORY)

### Entry Point
- An operator command string, from an SSH exec channel or from `ze cli -c`.
- Format at entry: plain text, possibly carrying pipe operators.

### Transformation Path
1. `execMiddleware` splits the input with `ProcessPipesDefaultFormatChecked`,
   which answers the command without pipes plus a formatter.
2. The command without pipes reaches the executor, so `tokenize` never sees `|`.
3. The executor answers `formatResponseData`'s JSON.
4. The formatter renders that JSON in the requested format, or in the configured
   default when the input carried no format pipe.
5. `ze cli` sends the operator's command unchanged and prints the answer
   verbatim. A `--format <x>` flag is appended to the command as `| <x>`, so the
   daemon applies it and one implementation decides every format.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Client ↔ daemon | SSH exec: command text up, formatted text down | No |
| SSH exec ↔ dispatcher | command text with the pipe chain removed | No |

### Integration Points
- `command.ProcessPipesDefaultFormatChecked` - already used by the interactive
  CLI and the web terminal. This spec adds the third caller rather than writing
  a second formatter.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | No test under `test/` depends on the current default format | exploration counted 6 shape-asserting files and all 6 pin the format, with `--format json` or a `json compact` pipe | tests go red and the count was wrong | `make ze-functional-test` | broken, then repaired. `make ze-functional-ui-test` is 180/180 green with phases 1-3 applied. It is NOT green with phase 3 reverted: `cli-verb-daemon-dispatch` fails there, because phase 2 alone makes the daemon format an answer the client then formats again. The three suites an earlier phase could not run are now answered, 2026-08-19. `test/parse` names `ze cli` in no file. `test/static` names it only in comments that say why those fixtures dispatch through a plugin instead. `make ze-functional-plugin-test` is 618/628, and every one of its 10 reds is a wall-budget blow-up under load rather than a shape assertion: 7 pass on a serial rerun of the failed set, `api-raw` and `interface-rate-show` are already journaled as budget-versus-load, and `aaa-radius-admin` passes 3 of 3 alone and fails before it sends any command. `audit-auth-fail`, which greps `ze cli -c "show audit action auth-fail"` for `auth-fail` and `admin`, passes against the new `text` rendering. Re-run on a settled tree, 2026-08-19: `make ze-functional-parse-test` 311/311 exit 0; `make ze-functional-plugin-test` 626/628, and both reds pass alone, `plugin-nexthop` 1/1 and `remove-private-as-replace-peer` 1/1 in 15.4s, the full-run failure having shown two BGP connections arriving where one was expected; `make ze-functional-static-test` 3/8, with every failure examined reporting `running without root; missing capabilities` and `static: apply route failed ... operation not permitted`, which is what its own Makefile line calls release evidence. That suite needs root and is not an outstanding gap. A-1 is CONFIRMED for the two suites that can run here |
| A-2 | Every pipe operator can run daemon-side | every arm of `ApplyPipes` in `internal/component/command/pipe.go` transforms the output string alone: `applyMatch`, `applyCount`, `applyFirst`, `applyLast`, `ApplyJSON`, `applyNDJSON`, `ApplyTable`, `applyText`, `applyYAML` and `injectPipeMeta` read no clock, no local file and no client state. The two world-touching arms, `applyResolve` in `pipe_resolve.go` and `applyOrigin` in `pipe_origin.go`, reach the world through resolvers registered in-process by `SetPTRResolver` and `SetOriginResolver`, whose only non-test callers are `runYANGConfig` and `runWebOnly` in `cmd/ze/hub`, both in the daemon. In the client process neither is registered, so `ReverseLookup` falls back to `net.DefaultResolver` and `LookupOrigin` answers an empty `OriginResult`. The daemon holds the configured resolvers, so daemon-side is the better view of the world, not a lost one | a client-side-only operator silently changes meaning | read every operator in `ApplyPipes` before moving formatting | confirmed |
| A-3 | `--format` is expressible as a pipe | `validCLIFormats` names text, table, json, yaml and ndjson, each of which is a pipe operator | `--format` needs a protocol field instead | unit test over the composed command string | confirmed. `commandWithFormat` (`internal/component/cli/client/main.go`) composes `<command> | <format>` and `TestCLIFormatFlagBecomesAPipe` pins all four cases. `knownPipeOps` (`internal/component/command/pipe.go`) names every one of the five formats, and a format it does not name is refused by `ValidatePipes` rather than reaching the dispatcher: `ze cli -c 'show version' --format bogus` exits 1 with `unknown pipe operator: bogus` |
| A-4 | an identity operator can be added to the pipe grammar without a second place needing to know it | the format-operator set was written out by hand in five places: `ApplyPipes`, `hasFormatOp`, `ValidatePipes`, `isFormatOp` and `foldFilters` | a new operator is dropped by whichever list forgot it | read all five, then a test over the one that fails silently | broken, then repaired. `foldFilters`'s switch is `//nolint:exhaustive`, so an operator it does not name falls out of BOTH the server-arg list and the client-op list: the chain then names no format, the configured default is appended, and an RPC caller gets a rendering with no error anywhere. The other four now call `isFormatOp`, so the set is stated once. `TestRawPipeSurvivesFilterFolding` covers the `foldFilters` arm, which no compile error can catch |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A script parsing SSH exec JSON breaks | interop or plugin tests go red | the caller adds `\| raw`, which answers the dispatcher's JSON unchanged and survives any future default. `\| json` is NOT the answer: `unwrapSingleKeyArray` reshapes a single-key wrapper, so it is a renderer |
| R-2 | The transcript's recorded shape changes | transcript tests go red | record what the operator saw; update the fixtures and say so |
| R-3 | `resolve` and `origin` pipes behave differently daemon-side | a resolved name differs between surfaces | A-2 reads them before the move |
| R-4 | a future exec-channel caller that parses the answer forgets to ask for raw, and degrades silently like the six this phase repaired | nothing: that is the whole problem | one helper carries the knowledge (`sshclient.ExecCommandRaw`), its doc comment states the obligation, and `docs/features/formatting.md` says the same for an out-of-tree script. No gate enforces it, so this risk is REDUCED, not closed |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Every scripted `ze cli -c` and `ssh host <cmd>` caller reads a different shape. No routing, wire, or config-apply behavior is touched. |
| How is it reverted? | Single commit revert. |
| Who else touches this path? | The web terminal and the interactive CLI already call the same helper and are not edited here. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ssh host 'show version'` over a real SSH exec channel | → | `execMiddleware` calling `ProcessPipesDefaultFormatChecked` | `TestSSHExecAppliesConfiguredFormatDefault` |
| `ssh host 'show version \| json'` | → | the pipe split in `execMiddleware` | `TestSSHExecRunsAFormatPipe` |
| `ze cli -c 'show version'` against a running daemon | → | `runBGP` into `Execute` | `test/ui/cli-format-default.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ssh host 'show version'`, no `cli format default` in the config | output is `text`, the registered default, not indented JSON |
| AC-2 | `cli format default table` committed, then `ssh host 'show version'` | output is the table rendering |
| AC-3 | `ssh host 'show version \| json'` | output is JSON, whatever the configured default is |
| AC-4 | `cli format default table` committed, then `ze cli -c 'show version'` | output is the table rendering |
| AC-5 | `ze cli -c 'show version' --format json` | output is JSON, whatever the configured default is |
| AC-6 | `ze cli -c 'show version \| yaml'` | output is YAML |
| AC-7 | a command whose response data is empty, no format pipe | output is `OK`, unchanged |
| AC-8 | `set cli format json` in an interactive session while `cli format default table` is committed | that session prints JSON and another session prints the table |
| AC-9 | an in-tree caller that parses the exec channel's answer as structured data, with `cli format default table` committed | it receives the dispatcher's JSON unchanged, and never the table |
| AC-10 | every in-tree caller of the exec channel is classified as structured-data or human-output | the classification is recorded, and no call site is left unclassified |

## AC-10: Every Exec-Channel Call Site, Classified

`gopls references` on `ExecCommand` (`internal/core/ssh/client/client.go`) and on
`(*cliClient).SendCommand` (`internal/component/cli/client/main.go`) names
seventeen call sites, and grep over the same two symbols names the same
seventeen. None is left unclassified.

| # | Call site | Command | What it does with the answer | Class | Moved to raw |
|---|-----------|---------|------------------------------|-------|--------------|
| 1 | `(*cliClient).modelExecutor` (`cli/client/main.go`) | the operator's, pipes already stripped by the Model | the Model applies `ProcessPipesDefaultFormatChecked` to it (`cli/model_mode.go`, `executeOperationalCommand`) | structured | yes |
| 2 | `(*cliClient).dashboardPoller` (`cli/client/main.go`) | `show bgp summary` | `parseDashboardSnapshot` unmarshals it (`cli/model_dashboard.go`) | structured | yes |
| 3 | `runBGP`'s daemon-reachability probe (`cli/client/main.go`) | `show version` | discards the answer; reads only the error | discarded | no |
| 4 | `(*cliClient).Execute` (`cli/client/main.go`) | the operator's, plus `--format` as a pipe | prints it | human | no |
| 5 | `(*cliClient).executeWithTranscript` (`cli/client/main.go`) | same | prints it and records it | human | no |
| 6 | `(*cliClient).SendCommand` (`cli/client/main.go`) | - | the transport wrapper itself | n/a | n/a |
| 7 | `buildRuntimeTree` (`cli/client/main.go`) | `system command list` | `json.Unmarshal` into `{"commands":[...]}` | structured | yes |
| 8 | `fetchPeerSelectors` (`cli/client/main.go`) | `show bgp peer list` | `json.Unmarshal` into `{"peers":{...}}` | structured | yes |
| 9 | `writePeers` (`plugins/completion/peers.go`) | `show bgp peer list` | `formatPeerCompletions` unmarshals it | structured | yes |
| 10 | `cmdSSHExec` (`plugins/signal/main.go`) | whatever `ze signal` was given | prints it | human | no |
| 11 | `forwardToDaemon` (`component/l2tp/cli/show.go`) | the operator's show command | prints it | human | no |
| 12 | `execReloadCommand` (`config/cli/reload_notify.go`) | `request reload` | discards the answer | discarded | no |
| 13 | the archive command (`config/cli/cmd_archive.go`) | `request config archive <name>` | prints it on stderr | human | no |
| 14 | `stopEphemeralDaemon` (`config/cli/cmd_edit.go`) | `stop` | discards the answer | discarded | no |
| 15 | `sshCommandExecutor` (`config/cli/cmd_edit.go`) | the operator's, from the editor's Model | the Model renders it, as row 1 | structured | yes |
| 16 | the editor's reload notifier (`config/cli/cmd_edit.go`) | `request reload` | discards the answer | discarded | no |
| 17 | the iface migrate command (`component/iface/cli/migrate.go`) | `request iface migrate ...` | prints it | human | no |

Six moved. A caller that PRINTS must not move, or the change this spec makes is
undone; `TestExecuteKeepsTheOperatorSurface` pins row 4. A caller that DISCARDS
its answer is left alone, because it asks for nothing to parse and the pipe
would only add a token for `execMiddleware` to strip. Rows 3, 12, 14 and 16 are
that case, and rows 14 and 16 never reach the pipe split at all: `execMiddleware`
matches `stop` and the lifecycle verbs on the whole input before it (see Design
Insights).

One dead surface found and left alone: `cmd/ze/internal/ssh/client/client.go`
re-exports `ExecCommand` and has no importer in the tree (`gopls references` on
its `ExecCommand` returns nothing; the only mention of the path anywhere is a
`// Related:` line in `pkg/zefs/store.go`). It is not extended with a raw
re-export, which would be an unwired export.

## End-to-End User Stories

| Story | Path | Test |
|-------|------|------|
| An operator commits a display default and the next command obeys it, over SSH and through `ze cli` | config commit → `ze.cli.format` → `configuredDefault` → both one-shot surfaces | `test/ui/cli-format-default.ci` |
| A script asks for JSON and keeps getting JSON whatever the operator committed | `--format json` → `\| json` → `ApplyPipes` | `test/ui/cli-format-default.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSSHExecAppliesConfiguredFormatDefault` | `internal/component/ssh/ssh_test.go` | AC-1, AC-2: an exec command with no pipe renders in the configured default, not indented JSON | green |
| `TestSSHExecRunsAFormatPipe` | `internal/component/ssh/ssh_test.go` | AC-3: `show version \| json` renders JSON and the dispatcher never sees `\|` | green |
| `TestCLIFormatFlagBecomesAPipe` | `internal/component/cli/client/main_test.go` | AC-5: `--format json` reaches the daemon as a `\| json` pipe, and no flag adds none | green |
| `TestPrintDaemonOutputPrintsWhatTheDaemonRendered` | `internal/component/cli/client/main_test.go` | AC-7: an empty answer prints `OK` on the no-format-pipe branch and nothing on the other; a non-empty answer is printed unchanged with one trailing newline | green |
| `TestRenderYAMLScalarFields`, `TestRenderYAMLNestedData`, `TestRenderYAMLStringList` | `internal/component/command/format_test.go` | the YAML shape rules the deleted client renderer used to reach, now tested against `RenderYAML` itself, which had no direct test | green |
| `TestRawPipeReturnsTheDispatcherJSONUnchanged` | `internal/component/command/pipe_test.go` | AC-9: `\| raw` answers the dispatcher's JSON byte for byte with `ze.cli.format=table`, keeps a single-key wrapper that `\| json` would unwrap, and passes non-JSON through | green |
| `TestRawPipeSuppressesPipeMetadata` | `internal/component/command/pipe_test.go` | AC-9: `\| raw` injects no `pipe` key, while the same chain ending `\| json compact` still does | green |
| `TestRawPipeSurvivesFilterFolding` | `internal/component/command/pipe_test.go` | AC-9: `foldFilters` keeps `pipeRaw` as a client-side operator for a command owning registered filters, so raw does not vanish there | green |
| `TestRawPipeIsRefusedBesideAnotherFormat` | `internal/component/command/pipe_test.go` | `\| raw \| json` is refused by `ValidatePipes`, so raw joined the mutually exclusive format set | green |
| `TestExecCommandRawAnswersTheDispatcherJSON` | `internal/component/ssh/ssh_test.go` | AC-9: `sshclient.ExecCommandRaw` over a REAL SSH exec channel answers JSON while `ExecCommand` on the same command answers the table | green |
| `TestBuildRuntimeTreeAsksForTheDispatcherJSON` | `internal/component/cli/client/main_test.go` | AC-10: the runtime command tree is built from the daemon's answer, not the static fallback | green |
| `TestFetchPeerSelectorsAsksForTheDispatcherJSON` | `internal/component/cli/client/main_test.go` | AC-10: `peer <TAB>` still finds its selectors | green |
| `TestModelExecutorAsksForTheDispatcherJSON` | `internal/component/cli/client/main_test.go` | AC-10: the interactive executor and the dashboard poller both send `\| raw` | green |
| `TestExecuteKeepsTheOperatorSurface` | `internal/component/cli/client/main_test.go` | AC-10: `ze cli -c` does NOT ask for raw, so the human surface keeps the configured default | green |
| `TestEditorExecutorAsksForTheDispatcherJSON` | `internal/component/config/cli/cmd_edit_test.go` | AC-10: `ze config edit --daemon`'s executor sends `\| raw` | green |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A: this spec adds no numeric input | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `cli-format-default` | `test/ui/cli-format-default.ci` | an operator commits `cli format default table`; `ze cli -c` answers the table, `--format json` still answers JSON, `\| raw` answers the dispatcher's JSON, and `ze completion peers` still finds its peers | green. Covers AC-4 to AC-7, AC-9 and AC-10 in nine checks. Proven to discriminate three ways: with `internal/component/cli/client/main.go` at HEAD it fails at check 2; with `raw` removed from `knownPipeOps` check 8 fails with `unknown pipe operator: raw`; with `internal/plugins/completion/peers.go` back on `ExecCommand` check 9 fails with `completion lost the 192.0.2.1 selector` |

### Interop Tests (Scope: protocol)
N/A: Scope is cli, and no wire-visible behavior changes.

## Files to Modify
- `internal/component/ssh/ssh.go` - `execMiddleware` splits the pipe chain with `ProcessPipesDefaultFormatChecked`, dispatches the command without pipes, and applies the returned formatter to the executor's output
- `internal/component/cli/client/main.go` - `--format` defaults to empty; when set it is appended to the command as a pipe; `renderCommandOutput` and `printFormatted` stop re-formatting the daemon's answer and keep `OK` for empty output
- `internal/component/cli/transcript.go` - the recorded text is now what the daemon rendered; correct the comment that says otherwise
- `demos/terminal/zefs-config/transcript.txt` - its claim about the committed default becomes true
- `internal/component/command/pipe.go` - a `pipeRaw` kind, `"raw"` in `knownPipeOps`, `pipeRaw` in `isFormatOp` and in `foldFilters`'s client-side arm, and `ApplyPipes` clearing `meta` when the chain names raw
- `internal/component/command/completer.go` - `raw` in `PipeOperators`, so it completes after `|`
- `internal/core/ssh/client/client.go` - `RawCommand` and `ExecCommandRaw`, the one place `| raw` is spelled
- `internal/component/cli/client/main.go` - a `send` transport field, `SendCommandRaw`, `modelExecutor`, `dashboardPoller`, and the four structured-data call sites moved
- `internal/plugins/completion/peers.go` - `writePeers` asks for raw
- `internal/component/config/cli/cmd_edit.go` - `sshCommandExecutor` and the `execEditorCommand` seam; the editor's Model executor asks for raw
- `docs/features/formatting.md`, `docs/architecture/api/commands.md` - the `raw` operator and why `| json` cannot serve

## Files to Create
- `test/ui/cli-format-default.ci` - functional test for the operator-visible behavior
- `internal/component/cli/client/main_test.go` - if absent, for `TestCLIFormatFlagBecomesAPipe`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | the leaf `environment cli format default` already exists |
| YANG validation constraints | No | no leaf added |
| YANG custom validators | No | no leaf added |
| CLI commands/flags | Yes | the `--format` default changes in `internal/component/cli/client/main.go` |
| CLI grammar (keyword before value) | N-A | no new grammar |
| Editor autocomplete | No | no new leaf or value |
| Functional test for new RPC/API | Yes | `test/ui/cli-format-default.ci` |
| Pipe completeness | Yes | this spec routes two more surfaces through `ProcessPipes` |
| Env var registration | No | `ze.cli.format` is already registered |
| Doctor check for runtime dependencies | No | no new path, socket, port, or binary |
| Prometheus counters/metrics | No | no observable state added |
| BGP family surface | N-A | not a BGP change |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | a behavior correction, not a feature |
| 2 | Config syntax changed? | No | the leaf is unchanged |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md`, for the `--format` default |
| 4 | API/RPC added/changed? | No | no API change |
| 5 | Plugin added/changed? | No | no plugin change |
| 6 | Has a user guide page? | Yes | whichever guide documents the display default; grep before answering |
| 7 | Wire format changed? | No | no wire change |
| 8 | Plugin SDK/protocol changed? | No | no SDK change |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | not protocol work |
| 10 | Test infrastructure changed? | No | one new `.ci` in an existing suite |
| 11 | Affects daemon comparison? | No | not a comparison claim |
| 12 | Internal architecture changed? | Yes | the client stops formatting; say so where the CLI/daemon split is described |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event, command, or capability changed? | No | none |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | grep `docs/` for the modified files and correct each stale claim |
| 17 | Existing docs show CLI examples for this area? | Yes | any example showing `ze cli -c` output must match the new default |
| 18 | `docs/architecture/system-architecture.md`, declared by `internal/component/ssh/ssh.go` and `internal/core/ssh/client/client.go` | No | it describes the hub/plugin process split. Its only "pipe" is the plugin IPC transport (`net.Pipe`, line 42), not the operator's `\|` chain, and it states nothing about output format |
| 19 | `docs/architecture/core-design.md`, declared by `internal/component/cli/client/main.go` and `internal/component/cli/transcript.go` | No | its "Pipe communication" row and every `format` mention are plugin IPC and BGP wire encoding. It makes no claim about which process renders an operator's answer |
| 20 | `docs/architecture/config/syntax.md`, declared by `internal/component/config/cli/cmd_edit.go` | No | the edit to `cmd_edit.go` is one executor asking for `\| raw`. No config syntax changed. Its own `format parsed \| raw \| full` leaf is the MRT dump format and is unrelated |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- prove the entry point reaches the helper
   - Tests: `TestSSHExecAppliesConfiguredFormatDefault`, `TestSSHExecRunsAFormatPipe`
   - Files: `internal/component/ssh/ssh.go`
   - Verify: both tests fail against today's code, one on the JSON shape and one because the dispatcher receives the pipe as an argument
2. **Phase: The daemon formats** -- `execMiddleware` splits pipes and applies the formatter
   - Tests: the two above go green
   - Files: `internal/component/ssh/ssh.go`
   - Verify: answer A-2 first by reading every operator in `ApplyPipes`, so nothing that needed the client's own view of the world moves
3. **Phase: The client stops formatting** -- `--format` becomes a pipe, local rendering is deleted
   - Tests: `TestCLIFormatFlagBecomesAPipe`, `test/ui/cli-format-default.ci`
   - Files: `internal/component/cli/client/main.go`, `internal/component/cli/transcript.go`
   - Verify: `OK` on empty output survives, and the offline fallback still formats locally because no daemon answered it
5. **Phase: The RPC channel keeps its JSON** -- an identity `raw` pipe and one helper
   - Tests: `TestRawPipeReturnsTheDispatcherJSONUnchanged`, and one per reclassified call site
   - Files: `internal/component/command/pipe.go` (`knownPipeOps`, a `pipeRaw` kind), `internal/core/ssh/client/client.go` (the JSON-asking helper), every in-tree caller that parses the answer
   - Verify: enumerate EVERY `ExecCommand` and `SendCommand` call site and classify each as structured-data or human-output. There are more than the six the phase 3 report named, and some ignore their output entirely. A caller left unclassified is AC-10 unmet
4. **Phase: Truth in the recording and the docs**
   - Files: `demos/terminal/zefs-config/transcript.txt` and every doc row answered Yes above
   - Verify: re-render the demonstration and read the recording

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | An explicit pipe still beats the configured default on all four surfaces, and the session override still beats both |
| Data flow | The dispatcher never receives a pipe token; `tokenize` is unchanged |
| Naming | The flag stays `--format` and its values stay the `validCLIFormats` names |
| Rule: `ai/rules/cli.md` | The agent-facing contract keeps a way to demand a machine format that no operator setting can override |
| Rule: `ai/rules/no-layering.md` | The client's local formatting is DELETED, never left beside the daemon's |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| SSH exec honors the default | `go test ./internal/component/ssh/ -run TestSSHExec` |
| `ze cli` honors the default | the `.ci` runs green under `make ze-functional-test` |
| No client-side re-formatting remains | `grep -n "printFormatted" internal/component/cli/client/main.go` names only the offline path |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | A command string carrying a pipe now reaches `ParsePipe` from a remote SSH caller; `ValidatePipes` must refuse an unknown operator rather than hand it to the dispatcher |
| Error leakage | A pipe error must not echo daemon internals to the caller |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails on behavior mismatch | Re-read the source named in Current Behavior |
| A functional test asserts an old shape | Check the AC first: the shape change is intended, the assertion is not |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

The pipe split sits AFTER the branches that answer without the command
dispatcher, and it must stay there. `execMiddleware` matches the lifecycle
commands (`stop`, `restart`, `reboot`) and `plugin protocol` on the whole
lowercased input before that point: they write their own text or hand the
channel to the plugin transport, so they carry no dispatcher output to format,
and a pipe chain on them would only change the string those exact-match
comparisons see. Streaming commands keep the registered monitor formatter,
which this spec preserves. Only the ordinary-command tail is formatted.

The pipe chain runs in the DAEMON on every surface, and the two published docs
that said otherwise are corrected at closure. A generic operator filters or
renders what the command already produced; a command-owned filter resolves into
command arguments (`foldFilters`), so the rows are never produced. Both halves
run daemon-side. Neither has run in the client since `renderCommandOutput` and
`printFormatted` were deleted from `internal/component/cli/client/main.go`.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The daemon formats, the client prints | the client resolves `ze.cli.format` itself | nothing on the client startup path loads the config, so the client can only ever see the registered default, and `--remote` names a different daemon whose setting the client could never hold |
| `--format` travels as a pipe | a new protocol field on the SSH exec channel | the pipe grammar already carries every format name, so the flag needs no wire change and one implementation renders every surface |
| An identity `raw` pipe, and one helper that asks for it | revert the daemon-side formatting so the exec channel stays JSON | The exec channel is Ze's OWN RPC transport as well as an operator surface: in-tree callers parse its answer as structured data, and phase 2 made every one of them degrade SILENTLY, because each has a graceful fallback. Owner decision, 2026-08-18: honor the configured default everywhere, so the channel keeps formatting and a caller that wants machine output says so. `\| json` cannot serve: `unwrapSingleKeyArray` reshapes the payload, so it is not identity. ONE helper asks for `raw`, so there is a single place to be right rather than a rule every future caller must remember |

## Known Limitations
- The offline fallback path still formats locally, because no daemon answered
  it. Its shape is whatever the fallback handler produces.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-precommit-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated (`internal/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Each new test proven to discriminate: revert the change, watch it go red

## Deviations

| # | Planned | Done instead | Why |
|---|---------|--------------|-----|
| 1 | `pipeRaw` named explicitly in `foldFilters`'s client-side arm | a `default:` arm that carries EVERY unnamed kind to the chain | naming raw fixes raw and leaves the next operator to fall out the same way. A later commit generalized it and `docs/architecture/api/commands.md` records the arm as load-bearing. `TestRawPipeSurvivesFilterFolding` still pins raw |
| 2 | `docs/architecture/api/commands.md` updated for the `raw` operator only | that, plus two false "client-side pipe" claims corrected there and two more in `docs/features/formatting.md` | Documentation checklist rows 12 and 16 asked for exactly this and the implementation phase paid only half of it. Found at closure, fixed at closure |

---

## Implementation Summary

### What Was Implemented
- `execMiddleware` (`internal/component/ssh/ssh.go`) splits the pipe chain with `ProcessPipesDefaultFormatChecked`, dispatches the command without pipes, and applies the returned formatter. The dispatcher's `tokenize` never sees `|` again.
- `runBGP` (`internal/component/cli/client/main.go`) defaults `--format` to empty. `commandWithFormat` appends the flag as a pipe when the command names no format. `printDaemonOutput` prints what came back and keeps `OK` for an empty answer. `renderCommandOutput` and `printFormatted` are DELETED, not left beside the daemon's renderer.
- `pipeRaw` (`internal/component/command/pipe.go`) is an identity operator in `knownPipeOps`, `isFormatOp` and `PipeOperators`. `ApplyPipes` clears `meta` when a chain names it.
- `RawCommand` and `ExecCommandRaw` (`internal/core/ssh/client/client.go`) are the one place `| raw` is spelled. `cliClient.SendCommandRaw` is its in-package twin. Six structured-data call sites moved to them.
- Four user guides and two architecture docs corrected.

### Bugs Found/Fixed
- `foldFilters` dropped any pipe kind its switch did not name, for a command owning registered filters. The chain then named no format, the configured default was appended, and an RPC caller got a rendering with no error anywhere. Covered by `TestRawPipeSurvivesFilterFolding`, and generalized to a `default:` arm.
- Two published architecture docs said the pipe chain runs client-side. Found by this closure's documentation lens, corrected in commit A.

### Documentation Updates
- `docs/features/formatting.md`: the `| raw` section, anchored `<!-- source: internal/core/ssh/client/client.go -- RawCommand, ExecCommandRaw -->`; two "client-side" claims corrected.
- `docs/architecture/api/commands.md`: the generic-pipe paragraph now names the daemon as the process that runs the chain, anchored on `execMiddleware` and on `Execute, commandWithFormat, printDaemonOutput`; the `foldFilters` paragraph corrected.
- `docs/features.md`, `docs/guide/command-reference.md`, `docs/guide/rpki.md`, `docs/guide/flow-export.md`, `docs/guide/traffic-usage.md`, `docs/guide/netlab.md`.
- `python3 scripts/dev/code_to_docs.py --check` exits 0.

### Deviations from Plan
See the Deviations table above. Two rows, neither reducing scope.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-1 assumed no test under `test/` depended on the current default format | six shape-asserting files pinned it, and phase 2 alone made the daemon format an answer the client formatted again | `make ze-functional-ui-test` went red at `cli-verb-daemon-dispatch` | phases 2 and 3 land together; A-1 recorded broken then repaired |
| assumption | A-4 assumed a new pipe operator could be added without a second place knowing it | `foldFilters` carried `//nolint:exhaustive`, so an unnamed kind fell out of BOTH its lists and vanished silently | reading all five hand-written format sets | four sets now call `isFormatOp`, and `foldFilters` grew a `default:` arm |
| approach | the implementation phase read the Documentation Update Checklist as satisfied once the feature pages were edited | two architecture docs stated the opposite of the change, and row 12 named exactly that gap | this closure's documentation lens | corrected in commit A; the lesson is the journal row |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| One authority decides the format | Done | `configuredDefault`, `ProcessPipesDefaultFormatChecked` (`internal/component/command/pipe.go`) | the daemon is the only process of the pair holding the config |
| Every surface obeys it | Done | `execMiddleware` (`internal/component/ssh/ssh.go`), `Execute` (`internal/component/cli/client/main.go`), `executeOperationalCommand` (`internal/component/cli/model_mode.go`), `executeTerminalOperational` (`internal/component/web/cli_terminal.go`) | four surfaces, one helper |
| An explicit `--format` or format pipe still wins | Done | `commandWithFormat` (`internal/component/cli/client/main.go`), `hasFormatOp` (`internal/component/command/pipe.go`) | the flag is skipped when the command already names a format |
| `ssh host '<cmd> \| table'` runs the pipe | Done | `execMiddleware` (`internal/component/ssh/ssh.go`) | `TestSSHExecRunsAFormatPipe` |
| Ze's own RPC callers keep the dispatcher JSON | Done | `RawCommand`, `ExecCommandRaw` (`internal/core/ssh/client/client.go`) | six call sites moved |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestSSHExecAppliesConfiguredFormatDefault` (`internal/component/ssh/ssh_test.go`) | registered default is `text` |
| AC-2 | Done | same test | over a real SSH exec channel |
| AC-3 | Done | `TestSSHExecRunsAFormatPipe` (`internal/component/ssh/ssh_test.go`) | the dispatcher never sees `\|` |
| AC-4 | Done | `test/ui/cli-format-default.ci` check 2 | fails against the client at the parent commit |
| AC-5 | Done | `.ci` check 3, `TestCLIFormatFlagBecomesAPipe` (`internal/component/cli/client/main_test.go`) | four cases including pipe-beats-flag |
| AC-6 | Done | `.ci` check 4 | `\| yaml` inside the quoted command |
| AC-7 | Done | `TestPrintDaemonOutputPrintsWhatTheDaemonRendered` (`internal/component/cli/client/main_test.go`), `.ci` check 5 | `OK` only when the command names no format |
| AC-8 | Done | `executeOperationalCommand` passes `m.cliFormat` (`internal/component/cli/model_mode.go`); `handleSetCLIFormat` (`internal/component/cli/model_keys.go`) | `execMiddleware` passes an empty session format, so the exec channel never inherits one |
| AC-9 | Done | `TestRawPipeReturnsTheDispatcherJSONUnchanged`, `TestRawPipeSuppressesPipeMetadata`, `TestExecCommandRawAnswersTheDispatcherJSON` | raw keeps a single-key wrapper that `\| json` unwraps |
| AC-10 | Done | the 17-row table above, re-enumerated at closure | 7 `ExecCommand` + 3 non-raw `SendCommand` remain, 6 moved, 1 wrapper |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestSSHExecAppliesConfiguredFormatDefault`, `TestSSHExecRunsAFormatPipe`, `TestExecCommandRawAnswersTheDispatcherJSON` | green | `internal/component/ssh/ssh_test.go` | package `ok` in 1.727s |
| `TestCLIFormatFlagBecomesAPipe`, `TestPrintDaemonOutputPrintsWhatTheDaemonRendered`, `TestBuildRuntimeTreeAsksForTheDispatcherJSON`, `TestFetchPeerSelectorsAsksForTheDispatcherJSON`, `TestModelExecutorAsksForTheDispatcherJSON`, `TestExecuteKeepsTheOperatorSurface` | green | `internal/component/cli/client/main_test.go` | package `ok` in 1.791s |
| the four `TestRawPipe*` and the three `TestRenderYAML*` | green | `internal/component/command/pipe_test.go`, `internal/component/command/format_test.go` | scoped run exit 0 |
| `TestEditorExecutorAsksForTheDispatcherJSON` | green | `internal/component/config/cli/cmd_edit_test.go` | package `ok` in 238.177s |
| `cli-format-default` | green | `test/ui/cli-format-default.ci` | in `make ze-functional-ui-test`, 191 passed / 1 skipped of 192 |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| every file under "Files to Modify" | Done | landed in the implementation commit |
| `test/ui/cli-format-default.ci`, `internal/component/cli/client/main_test.go` | Done | both exist |
| `demos/terminal/zefs-config/transcript.txt` | Done | re-recorded; it now demonstrates the committed default and `\| raw` |
| `docs/architecture/api/commands.md` | Changed | the `raw` row landed later; two false client-side claims corrected at closure |

### Audit Summary
- **Total items:** 24
- **Done:** 23
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (Deviations row 2)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| One authority decides the format and every surface obeys it | functional | `test/ui/cli-format-default.ci`, nine checks against a running daemon with `cli format default table` committed. In `make ze-functional-ui-test`: 191 passed, 1 skipped, of 192 |
| An explicit `--format` or format pipe still wins | functional + unit | `.ci` checks 3, 4 and 6; `TestCLIFormatFlagBecomesAPipe` case "an explicit format pipe beats the flag" |
| `ssh host '<cmd> \| table'` runs the pipe | unit over a real SSH exec channel | `TestSSHExecRunsAFormatPipe` asserts the dispatcher received the command without `\|` |
| Ze's own RPC callers keep the dispatcher JSON | functional + unit | `.ci` checks 8 and 9 (`ze completion peers` still finds `192.0.2.1`); `TestExecCommandRawAnswersTheDispatcherJSON` over a real channel |
| Discrimination (the tests would fail if the behavior were reverted) | proven three ways | the client at the parent commit fails check 2; `raw` removed from `knownPipeOps` fails check 8 with `unknown pipe operator: raw`; `peers.go` back on `ExecCommand` fails check 9 with `completion lost the 192.0.2.1 selector` |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| none: the spec declares `Deferral shard: -` | n/a | `ls plan/deferrals/` names no `cli-format` shard. No shard to remove, and no foreign shard was emptied by this closure |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fixit-cli-format-default-everywhere-2e38eb27-078b-4f5b-a456-56437e962d09.md` |
| `review_gate.py check` | clean (0 code files in commit A, hashes match) |
| Rounds | 2. Round 1 found one ISSUE across two files and two NOTEs; round 2 over the fixes found nothing |
| Reviewer lenses used | wiring + logic + removed-behavior; security + edge cases + allocation; documentation drift + style |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | "Generic pipes ... remain client-side pipe operators" and "a kind the `foldFilters` switch does not name stays client-side ... carries every other kind to the client". Both false since `execMiddleware` took the chain | `docs/architecture/api/commands.md` | rewritten to name the daemon, with source anchors on `execMiddleware` and on `Execute, commandWithFormat, printDaemonOutput` |
| 2 | ISSUE | the same false dichotomy twice, in the command-specific-filters section | `docs/features/formatting.md` | rewritten: a command filter stops the rows being produced, a generic pipe filters what the command already produced, both daemon-side |
| 3 | NOTE | three paragraphs about `publishAcceptedLocalIdentity` and break-glass revocation, belonging to an AAA spec | this spec's Design Insights | replaced with the insight this work leaves |
| 4 | NOTE | two open verification-debt rows for the implementation commit | `plan/verification-debt/06f056c4.md` | left open. They are session `06f056c4`'s rows, and verification debt is paid at the push, which this closure does not perform |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `test/ui/cli-format-default.ci` | yes | `ls -l` gives 11089 bytes, dated 2026-08-18 |
| `internal/component/cli/client/main_test.go` | yes | `ls -1` lists it; `grep -n "^func Test"` names 5 of this spec's tests |
| `docs/architecture/api/commands.md`, `docs/features/formatting.md` | yes | both clean in `git status --porcelain` before this closure edited them |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1, AC-2, AC-3 | the exec channel honors the default and runs a format pipe | `make ze-unit-pkg-test PKG=./internal/component/ssh/...` -> `ok github.com/ze-software/ze/internal/component/ssh 1.727s` |
| AC-4, AC-5, AC-6, AC-7 | `ze cli -c` honors the default, the flag travels as a pipe, `OK` survives | `ok .../internal/component/cli/client 1.791s`; `test/ui/cli-format-default.ci` green inside 191/192 |
| AC-8 | the session override still wins | `grep -n ProcessPipesDefaultFormatChecked internal/component/cli/model_mode.go` shows `executeOperationalCommand` passing `m.cliFormat`; `execMiddleware` passes `""` |
| AC-9 | `\| raw` is identity | `make ze-unit-pkg-test PKG=./internal/component/command RUN='TestRawPipe\|TestRenderYAML\|...'` exit 0 |
| AC-10 | every call site classified | fresh grep at closure: 7 `sshclient.ExecCommand(` + 3 non-raw `.SendCommand(` + 6 raw + 1 wrapper = 17 |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `ssh host 'show version'` over a real SSH exec channel | `TestSSHExecAppliesConfiguredFormatDefault` (`internal/component/ssh/ssh_test.go`) | yes: it starts a `Server`, connects over SSH, and asserts the table rendering rather than indented JSON |
| `ssh host 'show version \| json'` | `TestSSHExecRunsAFormatPipe` (same file) | yes: it also asserts the command the executor received carries no `\|` |
| `ze cli -c 'show version'` against a running daemon | `test/ui/cli-format-default.ci` | yes: read at closure. It starts a daemon with `environment cli format default table` in the config file, blocks `ZE_CLI_FORMAT` from the ambient environment, and launches `ze` nine times |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | broken, then repaired | six shape-asserting files pinned the format. `make ze-functional-ui-test` 191/192 with all phases applied; `ze-functional-parse-test` 311/311; `ze-functional-plugin-test` reds each pass alone; `ze-functional-static-test` needs root |
| A-2 | confirmed | every arm of `ApplyPipes` transforms the output string alone. The two world-touching arms reach registered resolvers whose only non-test registrars are in `cmd/ze/hub`, so the daemon holds the better view |
| A-3 | confirmed | `commandWithFormat` composes `<command> \| <format>`; `knownPipeOps` names all five formats; an unknown one is refused by `ValidatePipes` with `unknown pipe operator: <name>` |
| A-4 | broken, then repaired | `foldFilters` dropped an unnamed kind out of both lists. Four sets now call `isFormatOp`, and the switch grew a `default:` arm. `TestRawPipeSurvivesFilterFolding` covers it |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Row 3, `--format` default | `docs/guide/command-reference.md` reads "The flag has no default of its own", matching `runBGP`'s `fs.String("format", "", ...)` | yes |
| Row 6, user guide pages | four guides corrected. A scan of every `ze cli -c` example in `docs/` followed by a JSON block finds one hit, in `docs/guide/rpki.md`, and it carries an explicit `\| json compact` | yes |
| Row 12, internal architecture | `docs/architecture/api/commands.md` and `docs/features/formatting.md` corrected at closure. This was the review's ISSUE | yes |
| Row 16, source anchors | `python3 scripts/dev/code_to_docs.py --check` exits 0 over the whole tree | yes |
| Row 17, CLI examples | `grep -rn "ze cli -c.*jq" docs/` and `grep -rn "ssh .*jq" docs/` outside a `json`/`ndjson`/`raw` pipe both return nothing | yes |
| Rows 18, 19, 20, declared design docs | none of the three states anything about CLI output format or which process runs the chain. Their "pipe" and "format" mentions are plugin IPC, BGP wire encoding, and the MRT dump leaf | yes |
| `make ze-precommit-verify` | NOT run, and deliberately so: the owner has the test harness mid-rewrite and the tree is broken suite-wide. Scoped equivalents recorded above. `make ze-repository-tracked-build-check` passes | recorded, not green |

## Core Insight

The SSH exec channel is two surfaces wearing one name. It is the operator's
one-shot command line AND Ze's own RPC transport, and a change that serves one
breaks the other in silence, because every in-tree caller of it has a graceful
fallback. Completion simply stops offering peers. The dashboard simply shows
nothing. Nothing logs, nothing exits non-zero, and no test that checks an exit
code notices.

What made the fix safe was not the `raw` operator. It was enumerating all 17
call sites and asking of each one what it DOES with the answer, then giving the
six that parse it a single helper to ask through. A rule that every future
caller must remember would have been the same defect with a delay.
