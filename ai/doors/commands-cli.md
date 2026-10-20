# The commands, CLI and SSH door

<!-- door-base: internal/component/command internal/component/cmd internal/component/cli internal/component/ssh -->

Read `system.md` first. This door covers:

- the command model and its registries (`internal/component/command`,
  `command/registry`, `command/grammar`)
- the central verb packages (`internal/component/cmd/{show,clear,set,...}`)
- the editor and the `ze cli` client (`internal/component/cli`)
- the SSH server (`internal/component/ssh`)

The daemon dispatcher they feed is
`internal/component/plugin/server` `Dispatcher`, which `plugin-infra.md` owns.

## Where it sits

- **The command model is infrastructure.** Every component and plugin registers
  commands into it, so it must hold no knowledge of any one owner: no BGP word, no
  peer grammar, no plugin help string (`docs/architecture/command-ownership.md#what-shared-code-may-carry`).
- **A command is served in one of three places**
  (`docs/architecture/api/commands.md`):
  - by the daemon dispatcher, under its YANG path
  - by an offline fallback (`command.MustRegisterOfflineFallback`), only after
    the daemon is unreachable
  - by a local-data handler (`command.MustRegisterLocalData`), in the caller's
    own process, with no daemon side

  Pick one on purpose. A local-data command is answered by
  `command.ServeLocal`, which only the `ze cli` client calls
  (`internal/component/cli/client`), so REST, gRPC, MCP and the web do not reach it.
- **SSH can be compiled out** (`ze_ssh` in `feature-gates.txt`). The command
  model and the editor cannot.
- **SSH sessions dispatch through the shared dispatcher.** The hub hands the SSH
  server `serverDispatcher(apiServer, audit.SSH)` (`cmd/ze/hub/main.go`), the same
  `plugin.CommandDispatcher` that web, MCP, REST and gRPC receive with their own
  audit surface (`cmd/ze/hub/main_servers.go` `serverDispatcher`).

## Who depends on it from outside

| Outside party | Contract | Breaks when |
|---------------|----------|-------------|
| An operator, and every script that runs `ze cli -c` | The command grammar R1 to R9 (`ai/rules/cli.md`, `docs/architecture/cli/root-namespace-grammar.md`), the verbs (`cli/command-verbs.md`), the error text | A path, a keyword or a verb changes. Ze is unreleased, so a grammar is replaced, but on purpose and with its page |
| A plugin, internal or external, that sends a command by its bare path, and the ExaBGP bridge | The dispatch key is the YANG path, not the wire method | A `ze:command` container moves. That renames the command, and every sender breaks (`commands.md`, "Dispatch keys are YANG paths") |
| A third-party plugin that declares commands, pipe aliases or an answer shape at Stage 1 | `pkg/plugin/sdk` declarations, bounded and control-character free (`ai/rules/plugins.md`), and the declared answer shape (`commands.md`, "A plugin declares its own answer shape") | The declaration format or its checks change |
| A script that reads `\| json`, `\| yaml` or `\| table` | Kebab-case keys from YANG, the `{"status":...}` envelopes (`docs/architecture/api/json-format.md`), the pipe operators (`internal/component/command` `pipe_catalog.go`) | A handler answers finished text, or a key changes spelling |
| A shell with Ze completion | The tab-separated completion format (`ai/rules/plugins.md`) | A one-line help text carries a tab or a newline |
| An SSH client | Public-key and password login (`internal/component/ssh` `pubkey.go`, `passwordauth.go`) | An auth option changes |
| REST, gRPC, MCP, web and LG clients | Each one reads this registry's commands, YANG parameters and typed answers (`api-surfaces.md`) | Anything above changes, on every surface at once |

## Boundaries

- **A handler answers data, never text.** The payload satisfies `ResponseData`,
  and output goes through `ApplyPipes` (`ai/rules/cli.md`).
- **Every RPC has a YANG node, and every node a handler.** The argument is a leaf
  the model declares, never a positional token the handler reads on its own
  (`ai/rules/cli.md`, "CLI Grammar: Keywords Before Values").
- **Dispatch is registration at every level.** A new subcommand is a
  registration, never a `case` (`ai/rules/plugins.md`, "Registration-Based Dispatch").
- **A command lives with its owner.** A central verb package must not register
  or spell a plugin's command, and `cmd/ze/internal/cmdutil` must not special-case
  one (`command-ownership.md`, "What the Removal Test Forbids").
- **`--flag` is offline tooling only.** It never reaches the YANG layer or the
  daemon (`ai/rules/cli.md`, and `docs/architecture/cli/command-namespacing.md`).
- **Help, usage and completion are derived** from the owner's registration and
  schema, never written in a second place.

## Read before code

| Read | It answers |
|------|-----------|
| `docs/architecture/api/commands.md` | Where a command is served, dispatch keys, answer shapes, pipes, pipe aliases, the two help texts |
| `docs/architecture/command-ownership.md` | Which package owns a command, and the self-containment guards |
| `docs/architecture/cli/command-namespacing.md`, `command-verbs.md`, `root-namespace-grammar.md` | Which root, which verb, which token order |
| `docs/architecture/cli/error-surface.md` | Where a fault appears on every surface, and how it reads |
| `docs/architecture/cli/command-completion.md`, `plugin-modes.md` | Completion and the plugin CLI modes |
| `docs/architecture/api/json-format.md` | The JSON keys and envelopes |
| `ai/digests/cli-editor.md` | The traced flow: SSH session, model, keystroke, dispatch, commit |
| `ai/patterns/cli-command.md` | The files a new command needs |
| `ai/rules/cli.md`, `ai/rules/user-facing-errors.md` | The grammar, output and error obligations |

## Traps

- **A handler that reads arguments the model never declares.** The usage line
  and the web admin form are generated from the model, so they show nothing to
  type, and the validator judges nothing
  (`plan/journal/command-takes-an-untyped-positional-value.md`, 2026-08-30 to
  2026-09-15).
- **The same leaf on the `-cmd` container and on the rpc input**, measured on 25
  commands (`plan/journal/argument-leaf-declared-on-container-and-rpc-input.md`).
- **A rename that leaves the parameter schema behind.** The `-api` module keeps
  the old rpc name, the lookup misses with no error, and the command loses its
  parameters on every surface (`plan/learned/RECURRING-PATTERNS.md`, "A renamed
  command leaves its parameter schema keyed to the old rpc name").
- **An error that discards the data.** `dispatchCommandResponse` returned only
  the Go error when a handler gave both
  (`plan/journal/error-path-discards-data-already-received.md`, 2026-09-08).
- **The validator speaks before the handler.** `argvalidate.go` refuses a value
  first, so the handler's better message is unreachable
  (`plan/journal/earlier-guard-hides-the-better-error.md`, 2026-09-03).
- **One surface fixed, the other not.** The web CLI `set` and the SSH `set` built
  a list entry two different ways
  (`plan/journal/guard-added-to-one-half-of-a-pair.md`, 2026-10-13).
- **A moved mechanism, unswept pages.** The pipe chain moved to `execMiddleware`
  (`internal/component/ssh/ssh.go`) and two pages kept the old place
  (`plan/journal/comment-describes-superseded-behaviour.md`, 2026-08-19).

## Gates

| Command | It judges |
|---------|-----------|
| `./le cli grammar` | Every command, root and offline flag against R1 to R9 |
| `./le cli ownership` | One owner per command, owners free of `cmd/ze` |
| `./le cli dispatch check` | Every command string the tree sends still routes |
| `./le cli stdio check` | A user path goes through the helper that reads `-` |
| `./le cli catalog check` | The command catalog matches the live registries |
| `./le doc yang-contract command-contract`, `usage-contract`, `help-shape` | Node and handler pairing, grammar stated in the model, one-sentence summaries |
| `./le plugin declarations check` | A plugin's Stage 1 commands match its registration |
| `./le doc check verify` | The documentation gate. Run it after you touch a `ze:command` declaration (`ai/rules/writing.md`) |
| `./le test functional ui`, `./le test functional editor` | The CLI, completion and editor suites |
