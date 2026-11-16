# Handover: external plugin schema, from phase 5

Written 2026-11-16. The session that ran phases 1 to 4 grew too large to
supervise the next one, so the work crosses a session boundary here.

## Where the work is

| Item | Value |
|------|-------|
| Spec | `plan/post-release/features/spec-plugin-external-yang-at-startup.md`, status in-progress, phase 5 of 9 |
| Branch | `worktree-spec-external-yang`, in the worktree `.claude/worktrees/spec-external-yang` of the server checkout |
| Per-spec state file | `tmp/session/2026-11-15-231106/state/session-state-plugin-external-yang-at-startup-99a3a802-6ea0-45ad-a7f7-a342c1f9baae.md`, untracked: it holds each phase's handoff and is the first thing the next phase agent reads |
| Implementation model | The owner asked for Fable for implementation agents |
| Pushed | Nothing. The branch reaches `main` only when the owner lands it with `git rebase` |

The work is also to be tested on the owner's laptop, which is why it lives in a
worktree (`ai/owner/machines.md`). The laptop takes the branch with
`git fetch claude:Code/github.com/ze-software/ze/main worktree-spec-external-yang`.

## The owner's design, not to be re-opened

The load order is the one `docs/architecture/hub-architecture.md` draws, and the
owner confirmed it on 2026-11-15 and approved the spec on 2026-11-16:

1. Before any protocol, Ze reads the plugin declarations: the `plugin` block of
   the config, and the plugins named on the command line. A plugin compiled into
   `ze` counts as declared with no entry.
2. Ze runs each declared external plugin. Running it is what starts the protocol.
3. Each plugin sends its Stage 1 declaration, which carries its YANG module, and
   waits at the barrier.
4. Ze joins the modules to the schema and parses the full config.
5. Stage 2 and each commit deliver the plugin's section.

A plugin, its declaration and its module change only when `ze` restarts. A first
draft that asked the plugin for its declaration in a separate run before the
parse was set aside by the owner: do not bring it back for boot. Declare mode
serves only the commands that parse a config with no daemon.

## Commits on the branch

| Commit | Content |
|--------|---------|
| `d3c39e6f7a`, `563dcc2527`, `2ade6c2eb0` | The spec: design, the owner's load order, approval |
| `cb4d48a648`, `0f1ea3af39` | Plugin conformance checker, separate work the owner ordered done directly (below) |
| `642ee84a4a` | Phase 2, the audit, in the spec |
| `bedff886c3` | Phase 1, the fixture plugin `plugin/external-yang` |
| `7df08cca17` | Phases 3 and 4a: the join and the first pass |
| `ef2f010a90` | Phase 4b: the publish |

## Outstanding items, every one

| # | Item | Detail |
|---|------|--------|
| 1 | Phase 5, early Stage 1 at boot | Wire `config.FirstPass`, the early start of the declared external plugins, the barrier, `config.PublishExternalModules` and the full parse into `cmd/ze/hub` `runYANGConfig`. Nothing calls the first pass, the join or the publish at boot today, so a config with an external plugin's section is still refused |
| 2 | Blocker for item 1 | `Manager.SpawnMore` refuses until `StartAll`, which `Engine.Start` calls. The manager needs an entry that starts the declared externals earlier |
| 3 | Blocker for item 1 | `runStartupHandshake` reads Stage 1 inside the tier. The early step reads and keeps the declaration and the tier's handshake consumes it (`PluginConn.ReadRequest` already replays `pendingStartup`). Answering OK early would leave a tier-time refusal with no request to carry it (spec R-11). The wording of `TestEarlyStage1HoldsAtBarrier` needs settling |
| 4 | Blocker for item 1 | `runPluginPhaseWithin` would fork an early-started plugin a second time (spec R-12) |
| 5 | Order the publish enforces | The publish must come before `pluginserver.NewServer`, before any gNMI server, and before any full parse (spec R-22). A late publish is refused by name |
| 6 | Three early parses in `cmd/ze` (spec A-10) | `extractManagedClientConfig`: replace its `LoadConfig` with `config.FirstPass` and `Hub.Clients[0]`. `checkPushedConfig` and the first-boot branch of `cmdStartManaged`: each gives a full verdict before the daemon's first pass, and would now make the publish refuse. Move both verdicts into boot after the publish, or publish in `cmd/ze` |
| 7 | A hold inside a tier coordinator fails after 5 seconds | The audit's condition on A-2: the hold at the barrier must not be a wait inside a tier coordinator |
| 8 | A lone plugin silent at Stage 1 is unbounded | Spec R-13, not settled by reading |
| 9 | Phase 6, offline commands | `ze config validate` and the 19 entries of the spec's census take the first pass, the declare-mode run (`plugin.QueryDeclaration` already returns the declaration with its schema), the join and the publish |
| 10 | Phase 7, restart rule | A restarted plugin is compared with the published set (`yang.PublishedModule`); a reload whose plugin declarations differ is refused naming the restart |
| 11 | Phase 8, surfaces and doctor | The editor, the schema CLI, the config-claims check counting external roots, and the three codes `doctor-plugin-schema-undeclared`, `-refused`, `-changed` |
| 12 | Phase 9, measure | Spec Q10, on a host whose performance is known |
| 13 | The 13 draft tests | Under `test/draft/plugin/`, `test/draft/reload/` and `test/draft/editor/`, untracked and git-ignored. Each is promoted with a plain move when it goes green. `external-yang-own-root` was run and is red with `unknown top-level keyword: acme-monitor` |
| 14 | Acceptance criteria no functional test carries yet | AC-13 in the daemon (`show config`, `show yang`, save, the API's JSON); AC-6 "the internal owner of `service` keeps working"; AC-7 "the config under its root stays loaded" |
| 15 | First pass and augments of `plugin` or `environment` | Spec R-18: the first pass reads those two roots strictly, so an external module that augments either is refused before its plugin runs. The simpler fix is a join refusal of such an augment |
| 16 | Hooks the join does not judge | `ze:decorate` and `ze:backend`: their registries are out of reach of `config/yang` (spec R-20) |
| 17 | A non-`-conf` module that augments and also defines top-level nodes | Accepted today, and those nodes never reach the schema. The spec's refusal table does not cover it |
| 18 | `config.JoinExternalModules` after a publish | It joins to the published set and words a clash as "compiled into ze" (spec R-23) |
| 19 | `docs/functional-tests.md` | The fixture plugin is not documented there (spec documentation row 10) |
| 20 | Owner decision D-3 | Whether a plugin's own command argument model is part of this spec, as its last phase. It makes one declaration carry two modules, a wire change. Open; it blocks no phase before the last |
| 21 | Laptop | The branch is not on the laptop. The owner said it would be brought there when done |

## The conformance checker, separate work on the same branch

`show plugin conformance run <command>` and `le plugin conformance check run
<command>` judge a program in declare mode and the content of its Stage 1
declaration. Outstanding for it:

| # | Item |
|---|------|
| C-1 | Owner decision: may the daemon run a program a remote caller names? Today `refuseRemoteCaller` (`internal/component/plugin/conformance/register.go`) refuses every route but the local one, because `ai/allowed-system-commands.md` authorizes the daemon to run only the operator's configured plugin. The owner's rule that every operator command has a daemon form points the other way |
| C-2 | Later slices: the live connect-back, stages 2 to 5, config callbacks, command answer framing, shutdown, the schema. Each is a `not-judged` row today |
| C-3 | The generated wiki command catalog lacks the new command's row |
| C-4 | The red phase of `test/plugin/plugin-declarations-stage1-refusal.ci` was not observed by a run |

## Reds that predate this work

Six tests in `./internal/le/` (`TestNoPythonLeRemains`,
`TestCensusActionsDeclareHeavy`, `TestEveryRegisteredVerbIsInTheVocabulary`,
`TestNoNameEmbedsAVerbOrTest`, `TestHelpAiVerbsRendersTheVocabulary`,
`TestNoAgentSurfaceTellsAnAgentToRunAWholeTreeVerify`), lint findings in files
this work did not touch, two grammar violations (`arch error-inventory`,
`arch error-wrap`), and one dead path in `docs/architecture/testing/interop.md`.

## Tooling defects met, not fixed

| Defect | Effect |
|--------|--------|
| In a worktree session, an edit is refused until the main checkout's copy of the style guide and of the doors was read; a read of the worktree's own copy is not counted | Every phase agent must be told to read the main-checkout copies first |
| The guard that refuses a test deletion prints the whole refused command once per matched path | One refusal exhausted an agent's context; keep any Bash command that names a test path short |
| `./le` in a worktree runs the build of the last commit, so a gate that reads the registered tree does not see uncommitted commands or fixtures | Use `./le --name session <command>` to build `le` from the working tree |
| `gopls` stopped answering in the worktree: its shim points at a checkout that moved | Agents fall back to `grep` and targeted reads |
| A compound `cd && ... && ...` Bash line is refused in a worktree session | One command per call |

## Also on the server's `main`, unpushed

`029984f17d` (the plugin door says an external plugin's YANG arrives at Stage 1)
and `914b25449d` (the VyOS research page corrected). The laptop already holds
the second as `efa194fd37`, with an identical diff.
