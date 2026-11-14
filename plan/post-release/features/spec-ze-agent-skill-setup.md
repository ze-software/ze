# Spec: ze-agent-skill-setup

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | cli |
| Priority | P3 |
| Release reason | A new operator convenience in the shipped binary. No release profile needs it, and no shipped behaviour is wrong without it. |
| RFC gaps | - |
| Depends | - |
| Phase | 4/7 |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | development tier |
| Updated | 2026-11-13 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Ze ships guides for an operator's agent inside the binary (`ze show skills`,
`ze show skill <name>`). An agent that drives Ze over MCP is told where to look
on connect. An agent with a shell and the `ze` binary is told nothing: it finds
the guides only when the operator says to run `ze show skills`.

The goal: one `ze` command sets the operator's own agent harnesses up to use
Ze. It installs a stub skill named `ze` that the harness lists, whose body
sends the agent to the guide the installed binary serves, and, where the
harness supports hooks, a session-start hook that tells the agent what it is
standing on. A second command reports what is installed, and a third removes
it. The model is `rex skill setup`, `status` and `remove`.

Owner decisions (2026-11-13):

| Gate | Decision |
|------|----------|
| Scope | The stub and the session-start hook. One stub named `ze`, not one per bundled guide |
| Daemon form | Through the daemon the commands write nothing and answer the content to place |
| Names | `create agent skill`, `show agent skill`, `delete agent skill`; the hook runs `show agent context` |
| Model fit | Decided from who uses it, at the owner's direction ("think about how the user will use the software"). A person sets their agent up at a shell, as they run `ze completion` or `ze init`: there the command installs. Nobody sets their workstation's agent up from inside the router's CLI, so every `ze cli` form and every daemon surface gives the daemon's answer. The daemon's answer is for an agent connected to a router, on a machine that may hold no `ze` binary. The fit is the existing one for `show ping` and `show version`: a plain local command and the owner's daemon command on the same path, one producer |

Out of scope: serving the guides to MCP-connected agents as prompts or
resources; a per-project install; the development skills under `ai/skills/`,
whose stubs `./le ai sync update` writes.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - the design every change respects
  → Constraint: a command is declared once and reaches every interface. All four commands carry a YANG node.
  → Constraint: every message a user sees names the problem, why, and the exact fix.
- [ ] `ai/doors/system-plugins.md`, `ai/doors/plugin-infra.md` - the plugin doors
  → Constraint: a plugin owns its whole surface. The four commands, their YANG, their daemon commands and their help live in `internal/plugins/skills`.
  → Constraint: no plugin imports another plugin's implementation (`./le arch tier check`). The liveness dial `ze status` owns in the signal plugin is therefore not called; the address comes from the client package both already import.
- [ ] `ai/doors/commands-cli.md` - the command model
  → Constraint: a handler answers data, never text; every argument is a leaf the model declares; `--flag` is offline tooling and never reaches the daemon.
  → Constraint: the door says "`ze cli` answers a local-data command in its own process before it asks the daemon". That holds for `ze cli -c` only (`internal/component/cli/client/main.go`, the two `ServeLocal` calls). An interactive `ze cli` session is a shell on the daemon (`sshclient.RunInteractive`), and every typed line is dispatched there. The door is corrected by this work.
  → Constraint: a plain local command (`command.MustRegisterLocalMeta`) is answered only by `ze <command>` at a shell; `ze cli` and every daemon surface reach the builtin RPC its owner registered on the same path.
- [ ] `ai/doors/core.md` - the door for `internal/core`
  → Constraint: a core package is a library with no lifecycle and no registry side effect, importing neither components nor plugins. A core package that reads an env key registered elsewhere ends the process in its own tests, so the moved stub code reads no env key.
- [ ] `ai/doors/le-tooling.md`, `ai/doors/hub-engine.md` - the `le` and `cmd/ze` doors
  → Constraint: a shipped `ze` never links an `internal/le` package (`TestNormalZeLinksNoInternalLe`), so the stub file operations in `internal/le/ai/sync` cannot be imported. They move to core, and `le` calls them there.
- [ ] `docs/architecture/api/commands.md`, "Served by the daemon too" and "Local commands already served by the daemon under the same path"
  → Constraint: the bridge serves only local-data handlers ("a plain local command: it prints, and answers no value" is skipped), so a plain local command reaches the daemon only through the builtin RPC its owner registers for the node. `show ping` is built this way.
  → Constraint: the pair's stated contract is "the two routes call one producer, so they cannot drift". Here both routes call the one plan producer; the local route additionally applies the plan.
  → Constraint: `command.CallerPath` confines a caller-named path to the Ze config and store directories. The harness directories are outside them and are never caller-named: the handler builds them, on the local route only.
- [ ] `docs/architecture/cli/command-verbs.md`, "Choosing the verb"
  → Decision: install is `create` (question 5), status is `show` (question 1), removal is `delete`. The page bans an invented operational `remove`. Authorization places `create` and `delete` in the edit section by their verb.
- [ ] `ai/rules/user-facing-errors.md`, `ai/rules/cli.md`
  → Constraint: a refusal is an operator error with the fix on its last line; output goes through the pipe layer.

### RFC Summaries (Scope: protocol)
Not applicable: Scope is cli.

### Harness formats (outside readers and writers of the files this feature touches)
- [ ] The Rex installer's output on the owner's machine (`~/.claude/skills/rex/SKILL.md`, `~/.agents/skills/rex/SKILL.md`, the Rex entry of `~/.claude/settings.json` and of `~/.codex/hooks.json`), read by a research agent 2026-11-13
  → Constraint: Claude Code reads a session-start hook from `hooks.SessionStart` in its settings file: a list of groups, each holding a `hooks` list of entries with `type` `command`, a `command` string and a `timeout` in seconds. Codex reads the same shape from `hooks.SessionStart` in its `hooks.json`. The Codex format is known here only from Rex's output.
  → Constraint: Codex trusts a hook by position and hash in its own `config.toml` and asks the user before it runs a new or changed one. Ze never writes that trust store; it says the hook needs approving.
  → Constraint: the hook command is wrapped in the system shell so that a missing or failing `ze` never fails the session: it discards the error stream and ends in success. Ze's targets are Linux and macOS, which both have that shell.
- [ ] The proof runs of 2026-11-13 recorded in `docs/contributing/claude-code-cheatsheet.md`, "Where a Skill Lives"
  → Constraint: Claude Code reads stubs from `.claude/skills/`; Codex, OpenCode and Pi read them from `.agents/skills/`. Claude Code and Codex were seen to follow a stub; OpenCode and Pi were not run.

**Key insights:** (minimal context to resume after compaction)
- Four commands in `internal/plugins/skills`, each a plain local handler plus the plugin's own daemon command on the same YANG node, as `show ping` is built: `create agent skill`, `show agent skill`, `delete agent skill`, `show agent context`.
- One plan producer. At a shell: inspects and writes the caller's harness directories. Daemon command (every `ze cli` form, SSH, REST, gRPC, MCP, web): writes and reads nothing and answers the plan for a remote agent, with every state `not-inspected` and default home-relative paths.
- The remote plan is for a machine that may hold no `ze`: its stub tells the agent to ask the router it is connected to for `show skill ze`, and it carries no hook.
- Nothing shared changes: no registry field, no change to the route-comparison test, which never runs a plain local command.
- The stub is owned by an exact marker line, first after the frontmatter. The stub file and its `ze` directory are never written through a symlink. That code moves from `internal/le/ai/sync` to a core leaf, with the marker and the wording supplied by each caller.
- A hook entry is owned by its command: it is Ze's when the command runs `show agent context`.
- A settings file is edited by splicing bytes, never by re-encoding. Any shape Ze does not fully understand is refused untouched.
- One optional `agent <name>` on the three skill commands. No argument on the context command.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/plugins/skills/main.go` (256L) - `inventory` lists five bundled guides; `showSkills` and `showSkill` are local-data handlers; `render` prints the local route.
  → Constraint: the stub's `name` and `description` come from the `ze` row of `inventory`, never from a second literal.
- [ ] `internal/plugins/skills/register.go` (27L) - registers both commands with `command.MustRegisterLocalData` and a text drawing per envelope key.
- [ ] `internal/plugins/skills/yang/ze-skills-cmd.yang` (41L) - declares `show skills` and `show skill`, the second with a mandatory `name` leaf.
  → Decision: status cannot be `show skill installed`: the mandatory `name` leaf would read the keyword as a skill name. The noun is `agent skill`.
- [ ] `internal/component/plugin/server/local_bridge.go`, `loadLocalData` (L47 to L96) - for each local registration with a YANG node: when the node's wire method is a builtin RPC it continues ("this handler is that command's no-daemon route and the builtin already serves the path"); otherwise it serves the local-data handler.
  → Constraint: registering the plugin's own RPC for the node is all it takes for the daemon to answer with that RPC. No edit to the bridge.
- [ ] `internal/component/command/registry/registry.go`, `Meta` and `Artifact` (L109 to L169) - `Artifact` declares a produced file: a local directory argument, the field naming the local path, the field naming the daemon store key, and volatile fields.
  → Decision: `Artifact` does not fit. It means "a file the local route writes under a caller-named directory and the daemon keeps in its store", and the test checks both locations exist. This feature's daemon form keeps nothing.
- [ ] `cmd/ze/hub/local_bridge_match_test.go`, `TestLocalCommandDaemonMatchesLocal` (L60 to L178) - runs every local-data command the dispatcher serves through `command.ServeLocal` and through the dispatcher, for real, and requires the same JSON or the same refusal. It skips a command with no local-data handler (L155 to L158). It does not pin `HOME`.
  → Decision: a local-data registration would be run by this test against the home directory of whoever runs it, and would fail on the differing answers. The commands are plain local, which this test never runs; the file is not edited.
- [ ] `internal/component/cli/sshclient/client.go`, `DaemonAddress` (L559 to L575) - answers the host and port the client dials, from the env pair, then the store's default pointer, then the built-in default, reading no credential. `ze doctor` probes this address.
  → Decision: the context command dials this address. `ze status` and the signal plugin are not touched.
- [ ] `internal/core/resolve/resolve.go`, `StoreDir` (L29 to L42) - answers the config directory, opening nothing.
- [ ] `internal/core/version/version.go`, `Short` (L93 to L100) - the one-line version, in memory.
- [ ] `internal/core/skillstub/skillstub.go` (246L), landed by this spec's core move (commit `55f78701fc`) - `At`, `Path.Write`, `Path.OwnedBy`, `Path.RemoveOwnedBy`, `Frontmatter`: the marker-owned, symlink-safe stub file operations, with the marker supplied by the caller and a typed error for a directory at the file path.
- [ ] `internal/le/ai/sync/ai.go`, `markerFor`, `stubOf`, `skillTreesAreDirectories`, `writeStub`, `removeLeftoverStub` (L221 to L383, L498 to L516) - `le`'s own marker, wording and linked-root refusal, calling `skillstub`. Before the move these held the operations themselves.
  → Constraint: these carry `le` wording: `markerFor` names `./le ai sync update` and `ai/skills/`, and the refusals in `skillTreesAreDirectories` and `writeSkillUnder` say "the sync". The moved code takes the marker line from its caller and answers typed errors carrying the path, and each caller words its own message. `le`'s messages stay byte for byte what they are.

**Measured on 2026-11-13:** the five guides total 6,082 bytes; `ze` is 779 bytes and `ze-full` 1,122. A command that prints one is far below the 30,000 character output limit that ruled a command pointer out for the development skills.

**Behavior to preserve:** (unless the user explicitly said to change it)
- `show skills` and `show skill`: paths, answers, drawings, flags.
- `./le ai sync update` and `check`: every behaviour and message, byte for byte.
- `TestLocalCommandDaemonMatchesLocal`, the command registry and the bridge: untouched.
- `ze status` and the signal plugin: untouched.

**Behavior to change:** (only what the user asked for)
- Four new commands exist, each with a daemon command of the plugin's own.
- The commands door says which `ze cli` form answers in the client's process.
- The bundled `ze` guide names `ze create agent skill`.
- The stub file operations live in core.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| The skills plugin | `internal/plugins/skills/main.go`, `inventory`, `showSkills`, `showSkill` | Serves the bundled guides on every surface | Extends: it owns the four new commands, and the stub is built from its `ze` inventory row |
| A plain local command with its owner's daemon command on the same path | `docs/architecture/api/commands.md`, "Local commands already served by the daemon under the same path": `show ping`, `show traceroute`, `show interface`, `show version` | The local handler answers `ze <command>` at a shell; the owner's builtin RPC answers every daemon surface and `ze cli`; both call one producer | Extends: the four commands are built this way, with the plan producer as the one producer. The page's table gains them |
| The route comparison | `cmd/ze/hub/local_bridge_match_test.go`, `TestLocalCommandDaemonMatchesLocal` | Requires a served local-data command to answer alike on both routes, and skips plain local commands | Does not apply: the commands are plain local, so it neither runs nor compares them |
| A declared difference for a produced file | `internal/component/command/registry/registry.go`, `Artifact` | Says where each route keeps a produced file | Does not apply: the daemon form here keeps nothing, and no declaration is needed for a plain local command |
| The marker-owned, symlink-safe stub file operations | `internal/core/skillstub/skillstub.go`, `At`, `Path.Write`, `Path.OwnedBy`, `Path.RemoveOwnedBy`, `Frontmatter` | Write a stub without following a link at its directory or file, recognize one by the caller's marker line, remove an owned one. Moved out of `internal/le/ai/sync` by this spec's core move, which `le` now calls | Extends its use: the skills plugin calls it with its own marker and words its own messages |
| The daemon address | `internal/component/cli/sshclient/client.go`, `DaemonAddress` | Answers where the client dials, with no credential | Extends its use: the context command dials it, bounded to one second, as `ze doctor` probes it |
| The liveness dial of `ze status` | `internal/plugins/signal/main.go`, `RunStatus` | Dials and prints for `ze status` | Does not apply: it is another plugin's implementation, prints and exits, and ignores the store's default pointer that `DaemonAddress` honours |
| The version line | `internal/core/version/version.go`, `Short` | Answers the one-line version | Does not apply a change: called as is |
| The config directory | `internal/core/resolve/resolve.go`, `StoreDir` | Answers the directory, opening nothing | Does not apply a change: called as is, on the local route only |
| Verb choice and authorization by verb | `docs/architecture/cli/command-verbs.md`, "Choosing the verb" | Decides a command's first word and its authorization section | Does not apply a change: the names follow it |
| `ze install` and `ze init` | The install root handler under `cmd/ze`, and the init plugin's root handler | Root handlers in setup mode that write on the machine and have no daemon form | Does not apply: a root handler has no daemon form, which the owner's rule on operator commands does not allow for a new command |
| The MCP `ze_reference` tool | `docs/features/ai-first.md`, "MCP Transport" | Tells an MCP agent where the reference is | Does not apply: out of scope by the owner's decision |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `ze create agent skill`, `ze show agent skill`, `ze delete agent skill`, `ze show agent context`, typed at a shell (the plain local handler), or typed in `ze cli` or sent through SSH, REST, gRPC, MCP or the web (the plugin's daemon command).
- One optional keyed argument, `agent <name>`, on the three skill commands: one of the harness names the plugin knows. The context command takes none.

### Transformation Path
1. The validator judges the `agent` value against the model's enumeration.
2. The plan producer builds, from the plugin's harness table and the optional `agent`, one item per harness and kind: the stub (its home-relative path and text) and, where the harness has hooks, the hook (the settings file's home-relative path and the entry). The stub text is the `ze` inventory row's name and description as frontmatter, the marker line, and one order. The producer builds the plan in one of two forms. The local form orders `ze show skill ze` and includes the hooks. The remote form is for an agent on a machine that may hold no `ze`: its stub orders the agent to ask the Ze system it is connected to for `show skill ze`, over the connection it already uses, and it includes no hook. Paths in the plan are the harness defaults relative to the home directory and read no environment.
3. Daemon command: answers the remote form of the plan. Every item's state is `not-inspected`; `create` and `show` carry each stub's content, `delete` carries the files to remove; the answer says the files are the caller's to place. No file call is made.
4. Local handler: resolves each harness's real directories (the home directory, `CLAUDE_CONFIG_DIR`, `CODEX_HOME`), then inspects (`show`), applies (`create`) or removes (`delete`) each item of the plan, and answers each item's state and absolute path.
5. A harness is found when its configuration directory exists. With no `agent` named, `create` acts on every harness found and `show` reports every harness the plugin knows. `delete` requires `agent`: `agent <name>` acts on that harness, `agent all` on every harness found, and a bare `delete agent skill` is refused by the validator. The shared stub directory is written once however many harnesses read it.
6. `show agent context`, local: the version line, whether a dial to `DaemonAddress` answered within one second, and the config directory. Daemon command: the version line and that the caller reached the daemon; no path and no dial.
7. The harness runs the hook at session start and puts its output in front of the agent. The agent later loads the `ze` skill, receives the stub, and runs `ze show skill ze`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| `ze` to the operator's harness directories | Files under the home directory: one `SKILL.md` per stub directory, one edited settings file per harness with hooks | No |
| Harness to agent | The harness lists the skill by its frontmatter, delivers the stub on load, and runs the hook command at session start | No |
| Caller to daemon | The dispatcher serves the plugin's daemon commands under the four YANG nodes | No |

### Changed-Wire Census

**Search evidence:** scope `internal/`, `cmd/`, `pkg/`, `test/`, `docs/`, `ai/`, and every `*.sh`, `*.py`, `*.json`, `*.yml`, `*.yaml`, `*.toml` outside `vendor/`, `tmp/` and `bin/`. Terms: `show skill`, `show skills`, `skills/ze`, `SessionStart`, `hooks.json`, `agent skill`, `agent context`, `Artifact`, `DaemonPath`. Positive control: the search finds the two existing commands at `internal/plugins/skills/register.go` and the one `Artifact` user at `internal/plugins/support/register.go`. Findings: no site in the repository reads or writes a file under an operator's home harness directory; `.claude/settings.json` in the repository is the project's own hook file and is not touched; `registry.Meta` is read by the help printer, the catalog, `loadLocalData` and the route-comparison test. The design review of 2026-11-13 repeated the search and added the comparison test and `sshclient.DaemonAddress`.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| Operator stub skill file | writer | `internal/plugins/skills`, the `create agent skill` local handler, through the core stub operations | New: no writer exists today | Change: new writer |
| Operator stub skill file | reader | `internal/plugins/skills`, the `show agent skill` and `delete agent skill` local handlers | New | Change: new readers, which judge ownership by the marker line |
| Operator stub skill file | reader | Claude Code, Codex, OpenCode, Pi (outside the repository) | Each lists a skill by `name` and `description` and delivers the body on load | Change: a new file in their skill directory, in the form they already read |
| Harness settings file | writer | `internal/plugins/skills`, the `create agent skill` and `delete agent skill` local handlers | New. The files belong to the operator and to other installers such as Rex | Change: Ze splices one group in or out; every other byte is preserved |
| Harness settings file | reader | `internal/plugins/skills`, the `show agent skill` local handler | New | Change: new reader, which judges ownership by the entry's command |
| Harness settings file | reader | Claude Code and Codex (outside the repository) | Each runs the commands under `hooks.SessionStart` | Preserve the shape they read; Codex additionally asks the user to approve a new entry |
| Hook output | writer | `internal/plugins/skills`, the `show agent context` local handler | New | Change: new command |
| Hook output | reader | The agent in Claude Code or Codex (outside the repository) | The harness adds a session-start hook's output to the agent's context | Change: plain text lines, no format another program parses |
| Remote plan (the daemon commands' answer) | writer | `internal/plugins/skills`, the four builtin RPC handlers, from the plan producer | New | Change: new answers, row data through the shared renderer |
| Remote plan (the daemon commands' answer) | reader | An agent or operator on SSH, REST, gRPC, MCP, the web or `ze cli` (outside the repository) | Each surface renders a builtin's answer as it does any other | Change: a new answer in the shapes those surfaces already carry |
| Development stub skill file | writer | `internal/le/ai/sync/ai.go`, `Mirror.generateInto` | Existing | Preserve: same bytes; the file operations it calls move to core |
| Development stub skill file | reader | `internal/le/ai/sync/ai.go`, `Mirror.Check`, `Mirror.Sync`; the four harnesses | Existing | Preserve: unchanged |

### Integration Points
- `command.MustRegisterLocalMeta`, the plugin's builtin RPC registration (`pluginserver.RegisterRPCs`), and the `ze-skills-cmd` YANG module - four more of each.
- The core stub operations - called by the skills plugin and by `aisync`.
- `sshclient.DaemonAddress` - called by the context handler.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them | No | |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name | No | |

These rows are filled with evidence by the implementation and judged at closure, as on every spec.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `registry.Artifact` can declare this feature's route difference | The first draft's reading of `internal/plugins/support/register.go` | The design needed another fit | The design review read `Artifact` and the comparison test: it cannot. The pairing and a sibling declaration replace it | broken 2026-11-13, before any code; design changed with the owner |
| A-2 | A system plugin can register a builtin RPC for its own YANG node, as `doctor` and `ping` do from their command packages | `loadLocalData` reads `AllBuiltinRPCs`; the commands page lists owner-registered RPCs | The daemon command is registered another way the plugin model offers; if none exists for a system plugin, STOP and bring it to the main thread | The design review's second round read it: `pluginserver.RegisterRPCs`, called from the `register.go` of the `ntp` and `geodns` plugins and of `doctor`'s command package | confirmed 2026-11-13 by reading; the implementer re-reads one of them before use |
| A-9 | Inside `ze cli`, a local-data command whose path a builtin also holds is answered in the client's own process, so the command installs there | The commands door says `ze cli` answers a local-data command in its own process before it asks the daemon, and `doctor` is such a paired command | Inside `ze cli` the command answers the content instead of installing. The owner asked for both behaviours, so this goes back to him with the evidence before any handler is written | The wiring agent traced it before writing anything: `ze cli -c` answers in the client's process, and an interactive session is a shell on the daemon, where only the builtin answers. `doctor` is no counter-example: its pair sits on two paths | broken 2026-11-13, before any code. Brought to the owner, who directed the design be decided from how the commands are used; see the Model fit row |
| A-3 | A session-start hook's output reaches the agent in Claude Code and in Codex | Rex's hook does this in Claude Code on the owner's machine; Codex unverified, and its hook format is known only from Rex's output | The Codex hook is dropped and its row says unsupported | A live session in each harness, before the hook phase starts | unvalidated: documented for both harnesses ("plain-text stdout is added to the context", the Claude Code hooks reference and the Codex hooks documentation, research of 2026-11-14); the live proof is AC-31 |
| A-4 | Codex runs a newly added hook once the user approves it | Rex's documentation says so. Corrected 2026-11-14: Rex's Codex entry on the owner's machine does have a trust record. Codex identifies a trusted hook by its position and a hash in its own `config.toml`, and skips a new or changed hook, with a notice, until the user trusts it through `/hooks` (Codex source at tag `rust-v0.169.0`) | Codex users get no hook; the note Ze prints is still true | The live Codex session of A-3 | unvalidated: documented as above; the live proof is AC-31 |
| A-5 | OpenCode and Pi load a stub from the shared directory and offer no session-start hook | OpenCode's discovery proof of 2026-11-13; Pi by its source; hook formats not found | A hook there is future work | Recorded as a limitation | unvalidated |
| A-6 | The functional runner can give a test its own home directory | A `.ci` sets an environment variable with the `env` option (`test/ui/cli-argument-pattern-refused.ci`); none sets `HOME` today | The end-to-end proof moves to a compiled fixture | Write the wiring `.ci` first | confirmed: `option=env` with a relative value, read against the test's own work directory (`docs/architecture/testing/ci-format.md`, "expect=file"); the UI run showed both new tests reach the handlers (per-spec state file) |
| A-7 | The stub file operations can move to core with the marker and wording supplied by the caller, leaving `./le ai sync` byte for byte unchanged | The operations are file calls; their `le` wording is in three known places | A sync regression in every session | `./le test unit pkg path ./internal/le/ai/sync/` passes with its tests' assertions unchanged | unvalidated |
| A-8 | Each harness's configuration directory is the one named here: `~/.claude` (or `CLAUDE_CONFIG_DIR`), `~/.codex` (or `CODEX_HOME`), `~/.config/opencode`, `~/.pi` | Rex's documentation for the first two; OpenCode's own paths command; Pi inferred | A harness is reported as not found when it is installed; the operator names it with `agent` | Check each on the owner's machine in the wiring phase | confirmed: `~/.claude`, `~/.codex`, `~/.config/opencode` and `~/.pi` all exist on the owner's machine (per-spec state file, phase 1 handoff) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Ze corrupts the operator's settings file and their agent stops working | A harness that will not start after `ze create agent skill` | The file is read once and its bytes spliced, never re-encoded; any shape Ze does not fully understand is refused untouched; the result is parsed before it replaces the file; the file's size and modification time are compared again just before the replace, and a change refuses the write |
| R-2 | Ze removes or rewrites another installer's hook or skill | Rex's hook, or the operator's own, disappears | A hook entry is Ze's only when its command runs `show agent context`; a stub is Ze's only by its exact marker line; everything else is reported as foreign and left |
| R-3 | A settings file linked into a dotfiles repository is replaced by a regular file | The operator's dotfiles lose the link | The link is resolved in full; the write happens beside the target; a dangling link or a target that is not a regular file is refused |
| R-4 | The hook slows or breaks every agent session | Sessions start slowly on a machine with no daemon | The dial is bounded to one second, the hook entry carries a timeout, and the wrapped command ends in success with no output on any failure |
| R-5 | The agent ignores the stub | The agent works on Ze without the guide | The pattern was proven on Claude Code and Codex for the development stubs; AC-31 checks it for this stub |
| R-6 | A test writes into a developer's real home | Stray `~/.claude/skills/ze` after a test run | Every test of the plugin sets `HOME`, `CLAUDE_CONFIG_DIR` and `CODEX_HOME` to directories of its own; the functional tests run with a home of their own; the route-comparison test never runs a plain local command. A test in the plugin walks the registry and fails if any of the four paths has a local-data handler |
| R-7 | A remote agent places a stub that does not work on its machine | The agent runs `ze show skill ze` where there is no `ze` | The remote plan's stub names no local command: it tells the agent to ask the Ze system it is connected to for `show skill ze` over the connection it already uses. The remote plan carries no hook, which would need a local `ze` |
| R-14 | The context command opens the store read-only at every session start to resolve the daemon's address | A slow or failing hook on a machine with a damaged store | Accepted: `ze doctor` resolves the address the same way and the dial is bounded. An error from the address resolution is answered as the daemon not answering, with the reason, and the command still succeeds. The config file name stays out of the answer because it adds a second read for no use to the agent |
| R-8 | A remote caller reads the daemon's answer as "installed" | An MCP agent reports success with nothing on its machine | Every state is `not-inspected`, and the answer's first field says the files are the caller's to place |
| R-9 | The daemon's answer leaks the daemon host's paths or environment | A read-only user learns the daemon's config directory | The daemon commands build default home-relative paths, read no environment variable of the daemon, and the context answer carries no path |
| R-10 | Two harnesses share one stub and a delete for one removes the other's | Codex loses the skill when `agent pi` is deleted | With an `agent` named, the shared stub is removed only when no other harness that reads the shared directory is found on the machine |
| R-11 | The harness table and the YANG `agent` values drift | A known harness refused, or the reverse | A test compares the model's enumeration with the table |
| R-12 | The harness started from a desktop has no `ze` on its path | The hook prints nothing | Accepted and stated: the hook names `ze` as the stub does, so an upgrade or a moved binary keeps working; the wrapped command is silent when `ze` is not found, and `show agent skill` says the hook is installed, not that it ran |
| R-13 | Claude Code rewrites its settings file between Ze's read and write | A lost setting | The size and modification-time comparison of R-1; the window is the few milliseconds of the splice |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | The operator's agent harness: a bad settings write stops Claude Code or Codex from loading its hooks or starting. Nothing in the daemon, the data plane or the shared command model |
| How is it reverted? | `ze delete agent skill` removes what Ze wrote; reverting the commits removes the commands |
| Who else touches this path? | Rex and other installers edit the same settings files; `internal/le/ai/sync` shares `internal/core/skillstub` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze create agent skill agent claude` then `ze show agent skill`, at a shell, with a home of its own | → | the create and show local handlers, the core stub operations | `test/ui/agent-skill-create-show.ci` |
| `ze show agent context`, at a shell | → | the context local handler | `test/ui/agent-context.ci` |
| `create agent skill` through the daemon dispatcher | → | the plugin's daemon command, the plan producer | `TestCreateAgentSkillThroughTheDaemonAnswersThePlanAndTouchesNoFile` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | `ze create agent skill` in a home holding the Claude Code and Codex configuration directories and nothing installed | A stub exists at `skills/ze/SKILL.md` under the Claude Code directory and one under the shared `.agents` directory, each with the `ze` guide's name and description as frontmatter, the marker line first after it, and a body that orders `ze show skill ze`. The answer has one row per harness and item, the stub rows with state `created` and their absolute path | Files of other skills in those directories are untouched, because Ze writes one directory named `ze` |
| AC-2 | The same command again, after the stub's modification time was set back by an hour | Every row answers `unchanged`, and the stub's bytes and its set-back modification time are what they were | Everything survives, because nothing was out of date |
| AC-3 | A stub with the marker line and a different body | `show agent skill` answers `outdated`; `create agent skill` rewrites it and answers `updated` | The marker survives, because it shows Ze owns the file |
| AC-4 | A `SKILL.md` at the stub path that carries no marker | `show agent skill` answers `foreign`; `create` leaves it byte for byte, answers `foreign`, and says to remove the file and run the command again to replace it; `delete` leaves it | The file survives because Ze cannot show it wrote it |
| AC-5 | The `ze` skill directory or its `SKILL.md` is a symlink | The link is replaced by a real directory or file holding the stub, and the link's target is left byte for byte | The target survives because it is outside what Ze owns |
| AC-6 | The harness's skills directory itself (for example `~/.claude/skills`) is a symlink to another directory | The stub is written inside the directory the link points at, and the link stays | The link survives because the operator placed it, as with a linked settings file. `le` keeps refusing a linked tree root in a checkout; the difference is deliberate and each caller makes its own choice |
| AC-7 | `create agent skill` with a Claude Code settings file holding other hook groups, other events and unrelated keys | The file gains one group at the end of `hooks.SessionStart` whose single entry runs the wrapped `ze show agent context` with a timeout. Every byte outside the inserted text is identical to before | All other content survives byte for byte, because it is the operator's and other installers' |
| AC-8 | A settings file with no `hooks` key, or a `hooks` object with no `SessionStart` | The missing key is inserted with the group, in the file's own indentation; every byte outside the inserted text is identical | As AC-7 |
| AC-9 | A settings file that does not exist, in a harness directory that does | It is created holding only the hook list, with owner-only permissions | Nothing to preserve |
| AC-10 | A settings file Ze does not fully understand: not valid JSON, a byte-order mark, a duplicate `hooks` or `SessionStart` key, a `hooks` value that is not an object, a `SessionStart` value that is not a list | The file is left byte for byte, that harness's hook row answers `refused` with what was found and where, and says to correct the file or add the hook by hand. The stub for that harness is still written | The file survives untouched, because a rewrite could lose content |
| AC-11 | A settings file that is a symlink, possibly through a chain, to a regular file | The link survives and the file it finally points at holds the new content, with its permissions unchanged | The link survives because the operator placed it |
| AC-12 | A settings file that is a dangling symlink, points at something that is not a regular file, or cannot be written | The hook row answers `refused` with the path and the reason, and the fix; nothing is created or changed | Everything survives |
| AC-13 | The settings file changes on disk between Ze's read and its replace | Ze does not replace it; the hook row answers `refused`, saying the file changed and to run the command again | The other writer's content survives |
| AC-14 | `create agent skill` with the Codex directory found | The Codex hook row's note says the hook must be approved in Codex before it runs, and how | Codex's own trust store is untouched, because it belongs to Codex |
| AC-15 | `create agent skill` with the OpenCode or Pi directory found | The shared stub is written once, and their hook rows answer `unsupported` with the reason | No hook file is written for them |
| AC-16 | `create agent skill agent claude` in a home with no Claude Code directory | Claude Code's stub and hook are written, creating the directory; no other harness is touched | Other harnesses are untouched, because the operator named one |
| AC-17 | `create agent skill` in a home with no harness directory and no `agent` | The shared stub is written, and the answer says no harness was found and that the shared directory is the one Codex, OpenCode and Pi read | No settings file is created, because no harness with hooks was found |
| AC-18 | `show agent skill` | One row per harness the plugin knows and per item, with a state of `current`, `outdated`, `missing`, `foreign`, `refused` or `unsupported`, whether the harness was found, and the path inspected. No file is created or changed | Everything survives, because the command only reads |
| AC-19 | `delete agent skill agent all` after an install; a bare `delete agent skill` is refused by the validator, which names the values | The stub and its emptied directory are gone, the Ze group is gone from each settings file, its separating comma included, with every other byte identical, and each row answers `removed`. In Codex's file, a Ze group that is not the last is not removed: its `hooks` list is emptied in place (`{"hooks": []}`) and the row's note says so. A second run answers `absent`. Delete's states are `removed`, `absent`, `kept`, `foreign`, `refused` and `unsupported` | Other skills, other hook groups and other keys survive. A `SessionStart` list or a settings file left empty is kept as it stands, because removing a harness's own structure is not Ze's call. In Codex's file every group after Ze's keeps its index, because Codex keys a hook's trust by its position |
| AC-20 | `delete agent skill agent pi` while the Codex or OpenCode directory is found | The shared stub stays, its row answers `kept` (a correct outcome, exit status 0), and the answer names the harnesses that still read it | The stub survives because another installed harness reads it |
| AC-21 | `create`, `show` or `delete agent skill` through the daemon (SSH, REST, gRPC, MCP, web, `ze cli`) | No file is read or written on the daemon's host. The answer says the files are the caller's to place and holds, per harness, the stub's default home-relative path and state `not-inspected`; `create` and `show` carry the stub to place, `delete` carries the file to remove. The stub's body orders the agent to ask the Ze system it is connected to for `show skill ze`, names no local `ze` command, and the answer holds no hook | Nothing on the daemon's host changes, because the agent's home is the caller's. The paths are the defaults, because the daemon's own environment says nothing about the caller's. No hook survives into the remote answer, because a hook runs a local `ze` the caller may not have |
| AC-22 | `show agent context` at a shell on a machine whose daemon answers | The version line, that the daemon answered and at which address, the config directory, and one line naming the `ze` skill and `ze show skill ze`. Exit status 0 | None |
| AC-23 | `show agent context` at a shell with no daemon listening | The same lines with the daemon reported as not answering, returned within one second of the dial starting. Exit status 0 | None: a hook must never fail a session |
| AC-24 | `show agent context` through the daemon | The version line and that the caller reached the daemon. No directory path and no dial | The path is withheld, because a read-only caller has no other way to learn the daemon's config directory |
| AC-25 | The hook command as installed, run where `ze` is not on the path | It prints nothing and ends in success | None |
| AC-26 | An `agent` value the plugin does not know, or `agent` given twice | The validator refuses it and names the values. `delete` requires `agent` and accepts `all`; `create` and `show` do not accept `all` | None |
| AC-27 | `ze cli -c "create agent skill"` and the same line typed in an interactive `ze cli` session | Both give the daemon's answer of AC-21 and write nothing on the caller's machine | Nothing is installed, because `ze cli` is the router's CLI and a person sets their agent up at a shell |
| AC-28 | The four command paths in the local registry | Each has a plain local handler and no local-data handler, so `TestLocalCommandDaemonMatchesLocal` neither runs nor compares them; that test, the registry and the bridge are byte for byte what they were | The existing comparisons survive unchanged |
| AC-29 | `./le ai sync update` and `check` after the stub operations move to core | Every existing test of `internal/le/ai/sync` passes with its assertions unchanged | The development stubs' bytes and `le`'s messages survive exactly |
| AC-30 | `ze show skill ze` | The guide names `ze create agent skill` as how to set an agent up | The rest of the guide survives |
| AC-31 | A fresh agent session in Claude Code and one in Codex, after `ze create agent skill` on the owner's machine | The agent states the ze version it was told at session start, and when asked to use the `ze` skill its next action is to run `ze show skill ze`. The evidence is each session's event log, recorded in the closure record: the hook output, the skill load, and the command that follows | None: this is the goal. It is a proof on one machine with a model in the loop, and is recorded as such |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Installs Ze on a workstation and runs `ze create agent skill` | local handler → plan producer → core stub operations and settings splice | `test/ui/agent-skill-create-show.ci` |
| 2 | Opens their agent, which starts knowing the ze version and loads the guide when asked | harness hook → `show agent context`; harness skill → stub → `show skill ze` | The live sessions of AC-31 |
| 3 | Upgrades Ze and checks nothing is stale | `show agent skill` → states | `TestShowAgentSkillNamesAnOutdatedStub` |
| 4 | Stops using an agent with Ze | `delete agent skill` | `test/ui/agent-skill-delete.ci` |
| 5 | Asks a remote router through MCP how to set their agent up | dispatcher → the plugin's daemon command → the plan | `TestCreateAgentSkillThroughTheDaemonAnswersThePlanAndTouchesNoFile` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCreateAgentSkillWritesTheStubForEachHarnessFound` | `internal/plugins/skills/agent_local_test.go` | AC-1 | |
| `TestTheStubCarriesTheZeGuidesNameAndDescriptionAndTheMarker` | `internal/plugins/skills/agent_local_test.go` | AC-1: frontmatter from the inventory row, marker first, the order to run `ze show skill ze` | |
| `TestASecondCreateLeavesABackdatedStubUntouched` | `internal/plugins/skills/agent_local_test.go` | AC-2 | |
| `TestShowAgentSkillNamesAnOutdatedStub` | `internal/plugins/skills/agent_local_test.go` | AC-3, show half | |
| `TestCreateRefreshesAnOutdatedStub` | `internal/plugins/skills/agent_local_test.go` | AC-3, create half | |
| `TestAForeignZeSkillIsLeftByCreateShowAndDelete` | `internal/plugins/skills/agent_local_test.go` | AC-4, one row per command | |
| `TestCreateReplacesALinkedSkillDirectoryOrFileAndLeavesItsTarget` | `internal/plugins/skills/agent_local_test.go` | AC-5, one row per link position | |
| `TestCreateWritesInsideALinkedSkillsDirectory` | `internal/plugins/skills/agent_local_test.go` | AC-6 | |
| `TestCreateSplicesOneHookGroupAndLeavesEveryOtherByte` | `internal/plugins/skills/agent_hook_test.go` | AC-7, against a settings file with another installer's group, other events, unrelated keys, tabs and no final newline | |
| `TestCreateInsertsAMissingHooksOrSessionStartKey` | `internal/plugins/skills/agent_hook_test.go` | AC-8, one row per missing key | |
| `TestCreateMakesAMissingSettingsFile` | `internal/plugins/skills/agent_hook_test.go` | AC-9 | |
| `TestASettingsFileZeDoesNotUnderstandIsLeftAndRefused` | `internal/plugins/skills/agent_hook_test.go` | AC-10, one row per shape | |
| `TestALinkedSettingsFileIsWrittenThroughToItsTarget` | `internal/plugins/skills/agent_hook_test.go` | AC-11, a direct link and a chain | |
| `TestASettingsFileThatCannotBeWrittenIsRefused` | `internal/plugins/skills/agent_hook_test.go` | AC-12, one row per case | |
| `TestASettingsFileChangedSinceTheReadIsNotReplaced` | `internal/plugins/skills/agent_hook_test.go` | AC-13, through a seam between the read and the replace | |
| `TestTheCodexHookRowSaysItNeedsApproval` | `internal/plugins/skills/agent_hook_test.go` | AC-14 | |
| `TestHarnessesWithoutHooksAnswerUnsupportedAndShareOneStub` | `internal/plugins/skills/agent_local_test.go` | AC-15 | |
| `TestANamedAgentIsWrittenEvenWhenNotFoundAndNoOtherIs` | `internal/plugins/skills/agent_local_test.go` | AC-16 | |
| `TestWithNoHarnessFoundTheSharedStubIsWritten` | `internal/plugins/skills/agent_local_test.go` | AC-17 | |
| `TestShowAgentSkillCreatesAndChangesNoFile` | `internal/plugins/skills/agent_local_test.go` | AC-18 | |
| `TestDeleteRemovesWhatZeWroteAndASecondDeleteAnswersAbsent` | `internal/plugins/skills/agent_local_test.go` | AC-19 | |
| `TestDeleteSplicesOutOnlyZesHookGroup` | `internal/plugins/skills/agent_hook_test.go` | AC-19, settings half | |
| `TestDeleteKeepsASharedStubAnotherFoundHarnessReads` | `internal/plugins/skills/agent_local_test.go` | AC-20 | |
| `TestCreateAgentSkillThroughTheDaemonAnswersThePlanAndTouchesNoFile` | `internal/plugins/skills/agent_test.go` | AC-21 (wiring): a home with content, the daemon command, and the home's bytes compared before and after | |
| `TestTheDaemonPlanUsesDefaultPathsWhateverTheDaemonsEnvironment` | `internal/plugins/skills/agent_test.go` | AC-21, R-9: with the two foreign variables set in the daemon's process | |
| `TestDeleteThroughTheDaemonAnswersWhatToRemove` | `internal/plugins/skills/agent_test.go` | AC-21, delete | |
| `TestAgentContextNamesTheVersionTheDaemonAndTheConfigDirectory` | `internal/plugins/skills/agent_context_test.go` | AC-22, with a listener standing in for the daemon | |
| `TestAgentContextWithNoDaemonAnswersWithinItsBoundAndSucceeds` | `internal/plugins/skills/agent_context_test.go` | AC-23 | |
| `TestAgentContextThroughTheDaemonCarriesNoPath` | `internal/plugins/skills/agent_context_test.go` | AC-24 | |
| `TestTheInstalledHookCommandSucceedsSilentlyWithoutZe` | `internal/plugins/skills/agent_hook_test.go` | AC-25: the command string run with a path holding no `ze` | |
| `TestTheModelsAgentValuesAreTheHarnessTable` | `internal/plugins/skills/skills_model_test.go` | AC-26, R-11 | |
| `TestTheFourCommandsArePlainLocalWithADaemonCommand` | `internal/plugins/skills/agent_test.go` | AC-27, AC-28 (wiring): each path has a local handler, no local-data handler (so `command.ServeLocal`, which `ze cli -c` asks, does not serve it), and a builtin RPC for its node | |
| `TestTheRemoteStubSendsTheAgentToTheRouterAndTheRemotePlanHasNoHook` | `internal/plugins/skills/agent_local_test.go` | AC-21, R-7: no `ze show skill` order and no hook item in the remote form; both present in the local form | |
| `TestTheZeGuideNamesTheSetupCommand` | `internal/plugins/skills/skills_test.go` | AC-30 | |
| The core stub operations' tests | the core leaf's test file | The symlink and marker guarantees, with a caller-supplied marker and typed errors | |

Preservation tests, green before and after: every test of `internal/le/ai/sync` (AC-29, proven when the core move landed: the package's tests passed with no test file edited) and `TestLocalCommandDaemonMatchesLocal`, which is not edited (AC-28).

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| The dial bound of the context command | fixed at one second, not an input | N-A | N-A | N-A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `agent-skill-create-show` | `test/ui/agent-skill-create-show.ci` | With a home of its own, `ze create agent skill agent claude` then `ze show agent skill` answers Claude Code's skill and hook as `current` | |
| `agent-skill-delete` | `test/ui/agent-skill-delete.ci` | After a create, `ze delete agent skill` then `ze show agent skill` answers `missing` | |
| `agent-context` | `test/ui/agent-context.ci` | `ze show agent context` with no daemon exits 0 and names the version and that no daemon answered | |
| `command-contract-skills` (existing) | `test/ui/command-contract-skills.ci` | The plugin's commands and nodes pair up, the four new ones included | |

### Interop Tests (Scope: protocol)
Not applicable: no wire protocol. The harness proof is AC-31.

## Files to Modify
- `internal/plugins/skills/register.go` - register the four plain local handlers and their daemon commands
- `internal/plugins/skills/yang/ze-skills-cmd.yang` - the four nodes and the `agent` leaf
- `internal/plugins/skills/main.go` - the usage page lists the new commands
- `internal/plugins/skills/data/ze.md`, `internal/plugins/skills/data/ze-full.md` - name the setup command
- `internal/le/ai/sync/ai.go` - calls `internal/core/skillstub`; the moved code is deleted here (done, commit `55f78701fc`; no test file needed an edit)
- `docs/architecture/api/commands.md` - "Local commands already served by the daemon under the same path": the four commands join the table
- `ai/doors/commands-cli.md` - the sentence on `ze cli` and local-data commands says it holds for `ze cli -c`, and that an interactive session is dispatched on the daemon
- `docs/features/ai-first.md` - "Bundled Skills": setting an agent up
- `docs/guide/command-reference.md` - the four commands
- `docs/guide/mcp/overview.md` - what the daemon commands answer
- `features/ai-first-design.md` - the feature's declared scope, when its evidence or level changes
- `docs/contributing/claude-code-cheatsheet.md` - "Where a Skill Lives" names the core package as the stub operations' home
- `ai/INDEX.md` - a keyword row for setting an operator's agent up

## Files to Create
- `internal/plugins/skills/agent.go` - the harness table, the plan producer, the three skill handlers and their daemon commands
- `internal/plugins/skills/agent_hook.go` - reading, splicing and replacing a settings file
- `internal/plugins/skills/agent_context.go` - the context handler and its daemon command
- `internal/plugins/skills/agent_test.go`, `agent_hook_test.go`, `agent_context_test.go` - the tests above
- A core leaf under `internal/core/` for the marker-owned, symlink-safe stub file operations, with its tests
- `test/ui/agent-skill-create-show.ci`, `test/ui/agent-skill-delete.ci`, `test/ui/agent-context.ci`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/plugins/skills/yang/ze-skills-cmd.yang`: four `ze:command` nodes |
| YANG validation constraints | Yes | The `agent` leaf is an enumeration of the harness names; no free string |
| YANG custom validators | No | The enumeration is native; completion follows from it |
| CLI commands/flags | Yes | Four commands registered in `internal/plugins/skills/register.go`; no new offline flag |
| CLI grammar (keyword before value) | Yes | `agent <name>` is a single keyed leaf; `./le cli grammar` judges the paths |
| Editor autocomplete | Yes | Automatic from the enumeration |
| Functional test for new RPC/API | Yes | The three `.ci` files and the existing `command-contract-skills.ci` |
| Pipe completeness | Yes | The answers are row data through the shared renderer, with a text drawing per envelope key |
| Env var registration | No | `CLAUDE_CONFIG_DIR`, `CODEX_HOME` and `HOME` are other programs' variables, read as system variables on the local route only (`ai/rules/go-standards.md`, env access) |
| Doctor check for runtime dependencies | N-A | The feature adds no runtime dependency of the daemon: no socket, port, module or file the daemon needs |
| Doctor code per problem class | N-A | The states an operator can hit (outdated, foreign, a refused settings file, a Codex hook awaiting approval) live in the caller's home. `show agent skill` reports each with its fix. `ze doctor`, when a daemon answers, draws the daemon's answer, which runs on the daemon's account where no harness is set up, so a doctor check would report the wrong home |
| Prometheus counters/metrics | N-A | No daemon runtime state |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not a BGP feature |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/ai-first-design.md`; `./le feature report` for its level |
| 2 | Config syntax changed? | No | No config |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md`: the table of local commands with a daemon command on the same path gains the four |
| 5 | Plugin added/changed? | Yes | `docs/features/ai-first.md`, "Bundled Skills" |
| 6 | Has a user guide page? | Yes | `docs/guide/mcp/overview.md` for the daemon commands; `docs/features/ai-first.md` for the local ones |
| 7 | Wire format changed? | No | No protocol wire |
| 8 | Plugin SDK/protocol changed? | No | None |
| 9 | RFC behavior implemented, changed, or newly proven? | No | None |
| 10 | Test infrastructure changed? | No | Unless A-6 fails and the runner gains a home option; then `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | No | None |
| 12 | Internal architecture changed? | Yes | `ai/doors/commands-cli.md` (which `ze cli` form answers in the client's process); `docs/contributing/claude-code-cheatsheet.md` (the stub operations' home, done with the core move) |
| 13 | Route metadata keys added/changed? | No | None |
| 14 | Prometheus counters added/changed? | No | None |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | The command inventory pages the catalog check names: run `./le cli catalog check` and update what it lists |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `./le spec citation anchors spec plan/post-release/features/spec-ze-agent-skill-setup.md` at implementation. `docs/features/ai-first.md` is declared by the skills plugin's files and by `internal/core/resolve/resolve.go`, and is updated. `docs/architecture/api/commands.md` is declared by `internal/component/plugin/server/local_bridge.go`, which is read and not edited, and is updated for the declaration. `ai/rules/repo-maintenance.md` is declared by `internal/le/ai/sync/ai.go` and is unaffected: the sync's behaviour does not change. `docs/architecture/behavior/signals.md` is declared by `internal/plugins/signal/main.go`, which is read and not edited |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/features/ai-first.md` lines showing `ze show skills`; the usage page in `main.go` |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the four YANG nodes, the four plain local handlers and the four daemon commands, each answering a fixed "not implemented" refusal, the door and page corrections, and the wiring tests written to fail. A-6 validated
   - Tests: `test/ui/agent-skill-create-show.ci`, `test/ui/agent-context.ci`, `TestCreateAgentSkillThroughTheDaemonAnswersThePlanAndTouchesNoFile`, `TestTheFourCommandsArePlainLocalWithADaemonCommand`
   - Files: `register.go`, the YANG module, `agent.go` (skeleton), `docs/architecture/api/commands.md`, `ai/doors/commands-cli.md`
   - Verify: the commands appear in help and completion and are refused as not implemented at a shell and through the dispatcher; the wiring tests fail on the refusal
2. **Phase: The core move** -- done, commit `55f78701fc`: `internal/core/skillstub` holds the stub file operations with a caller-supplied marker and a typed error; `aisync` calls it
   - Evidence: the core package's tests and `./le test unit pkg path ./internal/le/ai/sync/` pass, the second with no test file edited; lint clean on both; `./le arch tier check` accepts the leaf (its one failure is `internal/le/interoplab/bgp/ze_ready.go`, another session's); the launcher built the commit and `./le ai sync update` and `check` are clean on it
3. **Phase: The stub commands** -- the harness table, the plan producer, create, show and delete for the stub, on both routes
   - Tests: the AC-1 to AC-6, AC-15 to AC-21 and AC-26 rows
   - Files: `agent.go`, `agent_test.go`, `skills_model_test.go`
   - Verify: tests fail, implement, tests pass; the create-show `.ci` passes for the stub
4. **Phase: The hook** -- A-3 and A-4 validated by a live session in each harness first; then reading, splicing and replacing a settings file, and the hook rows of the three commands
   - Tests: the AC-7 to AC-14, AC-19 and AC-25 rows
   - Files: `agent_hook.go`, `agent_hook_test.go`, `agent.go`
   - Verify: tests fail, implement, tests pass; both skill `.ci` files pass in full
5. **Phase: The context command** -- version, bounded dial, config directory; the daemon command's reduced answer
   - Tests: the AC-22 to AC-24 rows, `agent-context.ci`
   - Files: `agent_context.go`, `agent_context_test.go`
   - Verify: tests fail, implement, tests pass
6. **Phase: Guides and pages** -- the bundled `ze` guide, the usage page, and every page of the Documentation Update Checklist not already edited by its phase
   - Tests: `TestTheZeGuideNamesTheSetupCommand`
   - Verify: `./le doc check all` reports nothing in a file this work touched; `./le cli grammar`, `./le cli ownership` and `./le cli catalog check` pass for the new commands
7. **Phase: The live proof** -- the main thread runs `ze create agent skill` on the owner's machine with his agreement, then a fresh Claude Code session and a fresh Codex session
   - Verify: AC-31; then `ze delete agent skill` unless the owner wants it kept

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path |
| Correctness | The daemon commands reach no file call and no environment read; a hook group is removed only when its command runs `show agent context`; a stub only by its exact marker line; a settings file is spliced, parsed, and compared again before it is replaced |
| Naming | Answer keys are kebab-case; states are the closed sets the ACs name; the `agent` values match the harness table |
| Data flow | One plan producer feeds both routes; the stub text is built from the `ze` inventory row in one place; the stub file operations exist only in the core leaf; no harness name appears outside the skills plugin |
| Operator errors and doctor codes | Every refusal names the file, what is wrong and the fix on its last line |
| Rule: no-layering | The moved operations are deleted from `internal/le/ai/sync`, not kept beside the core copy |
| Rule: plugins | The commands, YANG, daemon commands and help are in the skills plugin; no central verb package registers or spells them; the plugin imports no other plugin |
| Nothing shared changed | `internal/component/command/`, `internal/component/plugin/server/` and `cmd/ze/hub/local_bridge_match_test.go` carry no edit from this work |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Four commands on every surface | `./le cli catalog check` and the surface test the commands door names |
| The package's tests | `./le test unit pkg path ./internal/plugins/skills/` |
| The commands door | `ai/doors/commands-cli.md` names `ze cli -c` where it said `ze cli` |
| The moved code's user | `./le test unit pkg path ./internal/le/ai/sync/` |
| The functional tests | `./le test functional ui` restricted to the three new files and `command-contract-skills` |
| One home for the stub operations | A search of `internal/le/ai/sync/ai.go` for the moved function names answers nothing |
| No stray files | After the package and functional tests, no `skills/ze` directory exists under the real home that was not there before |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The only caller input is `agent`, an enumeration. No path comes from the caller; every path is built from the home directory, the two foreign environment variables and fixed names |
| Daemon commands | No file call and no environment read is reachable, so a remote caller cannot make the daemon read or write under the daemon account's home, or learn its paths |
| Authorization | `create` and `delete` are in the edit section by their verb, although the daemon commands write nothing; `show agent context` through the daemon answers nothing a read-only caller could not already ask (`show version`) |
| Symlinks | The `ze` directory and its `SKILL.md` are never written through a link. A linked skills directory and a linked settings file are the operator's own placement and are resolved; a dangling or non-regular target is refused |
| What the hook runs | The installed command is a fixed string naming `ze` and its own subcommand; nothing from a file or an answer is interpolated into it |
| What the hook prints | The version, an address and a directory path, to the operator's own agent. No secret, no config content |
| Ownership | Removal and refresh touch only what the marker or the command shows Ze wrote |
| File modes | A stub and a new settings file are owner-only; an existing settings file keeps its mode |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| A system plugin cannot register a plain local handler and a builtin RPC on one path | STOP at the wiring phase and bring it to the main thread, with how `show ping` does it |
| A-3 or A-4 fails: a hook's output does not reach the agent | DESIGN, with the session's event log, put to the owner before the hook phase |
| A-6 fails: a `.ci` cannot have its own home | DESIGN: the wiring proof moves to a compiled fixture; the wiring rows change before code |
| A plain local handler cannot answer through the pipe layer what the ACs name (rows with states) | DESIGN: say what the local route can render, and bring the choice of a `--json` offline flag or a drawing to the main thread |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The development stubs point at a file because six guides exceed the harness's command-output limit. The operator stub points at a command because the bundled guides are tiny and live in the binary, which is Rex's own situation.
- The owner's rule that every operator command runs through the daemon has a real answer here even though the daemon cannot reach the caller's home: the daemon knows exactly what should be installed, and an agent on the other end can place it.
- The command model already had the shape for "acts on the caller's machine, and the daemon has its own answer": `doctor`. What it lacked was a way to say so to the test that expects the two to match.
- Design review, 2026-11-13 (independent agent, development tier, before any code). Verdict: not as written. What it changed: the route difference no longer rests on `Artifact`, which cannot express it; the comparison test pins the home variables, without which it would have edited the home of whoever ran it; `agent` is one optional value, because a repeated keyword is refused by the binder; the liveness dial is not moved to core, because `sshclient.DaemonAddress` already answers the address and a core leaf reading its env keys would end the process in its own tests; the stub operations take their marker and wording from the caller, because `le`'s are baked into them today; the settings edit splices bytes and gained an AC per malformed shape, per link case and for a change between read and replace; "found" is defined; the daemon answers default paths and no config directory; `delete` through the daemon answers what to remove; AC-2 sets the modification time back so that it cannot pass by timing. The owner then asked for both behaviours inside `ze cli` and through the daemon. Second round, same day: yes to implementation; it confirmed A-2 by reading and could not verify that `ze cli` answers a paired command in its own process (A-9).
- A-9 was checked before any handler was written, and it does not hold: only `ze cli -c` answers in the client's process, and an interactive session is a shell on the daemon. Brought the choice, the owner said to decide from how the software is used. A person sets their agent up at a shell; inside the router's CLI nobody does; the daemon's answer serves an agent connected to a router, which may sit on a machine with no `ze`. That gives the plain local command with its daemon command, drops the registry declaration and the comparison-test change, and exposed a flaw in the remote answer: it pointed at a local `ze` command and a hook that needs one. The remote stub now sends the agent back to the router, and the remote plan has no hook.
- The commands door stated the `ze cli` behaviour for both forms when it holds for one. A design was built on that sentence before the client was read; the door is corrected in the wiring phase.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| A plain local command with the plugin's own daemon command on the same path | A data command paired with a daemon command and a declared difference, so that `ze cli -c` installs too; teaching the interactive `ze cli` session to run commands on the client | Decided from use, at the owner's direction. Only a shell has a person setting an agent up, so the forms that would make `ze cli` install serve nobody, and each costs a change to shared infrastructure. The plain form is how `show ping` and `show version` already work |
| The remote plan's stub sends the agent to the router, and the remote plan has no hook | The same stub and hook on both routes | An agent that asks a router how to set itself up may sit on a machine with no `ze`. A stub that orders a local `ze` command, or a hook that runs one, would not work there |
| `create`, `show`, `delete agent skill` | `request agent skill install` and `remove` | Owner decision |
| The daemon commands answer the content to place | Writing into the home of the account the daemon runs as | Owner decision |
| One stub named `ze` | One stub per bundled guide | Owner decision |
| Delete requires `agent <name>` or `agent all` | A bare delete acts on every harness found | The owner's CLI rule (`ai/rules/cli.md`): the everything form of an acting command is an explicit `all` |
| One optional `agent` value | A repeatable `agent` | The argument binder refuses a keyword typed twice and no command model declares a list; running the command once per harness covers the case |
| Move the stub operations to core, marker and wording supplied by the caller | Copy them into the plugin; move them with `le`'s wording | `ze` may not link `internal/le`; a copy is the open-coded-copy defect; `le`'s wording in a `ze` message would be false |
| Dial `sshclient.DaemonAddress` | Move the dial of `ze status` to a core leaf | The producer exists and is what `ze doctor` probes; the signal plugin's dial is another plugin's implementation and ignores the store's default pointer |
| A hook entry is owned by the command it runs | A marker key inside the JSON entry | A harness may refuse an unknown key in a hook entry; the command string is the one field every harness accepts and Ze fully controls |
| Splice bytes, never re-encode | Decode, modify, encode | Re-encoding loses key order, indentation, number forms and the operator's layout, and "leaves the rest as it was" would be untrue |
| Refuse every shape Ze does not fully understand | Best-effort repair | A rewrite of a file Ze cannot read could lose the operator's content |
| Follow a linked skills directory and a linked settings file; never a linked `ze` directory or stub | Refuse all links, as `le` does in a checkout; follow all links | A linked skills directory or settings file is the operator's dotfiles setup and is theirs to place. The `ze` directory and its file are Ze's own, so a link there is replaced. A checkout's generated trees are `le`'s own, so `le` keeps refusing |
| The hook names `ze`, not an absolute path | The absolute path of the binary that ran the install | An upgrade or a moved binary would break an absolute path silently; the stub names `ze` too. The cost is a harness whose environment has no `ze` on its path |
| Never write Codex's trust store | Write the trust record so the hook runs at once | The trust store is Codex asking its user for consent; answering for the user defeats it |
| In Codex's file, delete empties a Ze group that is not the last in place (`{"hooks": []}`) and removes only a last one; in Claude Code's file it always removes the group | Remove the group in both files | Codex's trust key is positional (`<file>:session_start:<group index>:<entry index>`): removing a group shifts every group after it and drops their trust, so the operator would have to trust another installer's hook again |
| No forced overwrite of a foreign `ze` skill | An option that replaces it | No offline flag reaches the daemon grammar, and the case is rare: the answer says to remove the file and run the command again |
| The daemon's context answer carries no path | The same answer on both routes | A read-only caller would learn the daemon's config directory, and the daemon dialling its own port says nothing |

## Known Limitations
- OpenCode and Pi get the stub and no hook; their hook formats were not found.
- Whether a model in OpenCode or Pi follows the stub is unproven: no model was available to either on 2026-11-13.
- The install is per user. A per-project install is not offered.
- A settings file with comments, trailing commas or a byte-order mark is refused, not edited.
- The daemon commands cannot say what is installed on the caller's machine; they say so.
- A harness whose environment does not have `ze` on its path gets a silent hook.
- AC-31 is a proof on one machine with a model in the loop.

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
- [ ] AC-1..AC-31 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: `./le test unit pkg path ./internal/plugins/skills/`, `./le test unit pkg path ./internal/le/ai/sync/`, the core leaf's and `cmd/ze/hub`'s package tests for the named tests, and `./le test functional ui` for the named files. No agent runs any `./le verify` command: GitHub CI verifies the pushed commit
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
- [ ] Boundary tests for all numeric inputs (N-A: the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (N-A: no protocol)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script (commit A preserves the spec in history)
