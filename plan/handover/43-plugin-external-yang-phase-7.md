# Handover: external plugin schema, from phase 7

Written 2026-11-16. It replaces `42-plugin-external-yang-phase-5.md` for what
comes next. The session that ran phases 5 and 6 is too large to supervise
phase 7.

## Where the work is

| Item | Value |
|------|-------|
| Spec | `plan/post-release/features/spec-plugin-external-yang-at-startup.md`, status in-progress, phases 1 to 6 done, 7 to 9 open |
| Server | Branch `worktree-spec-external-yang`, worktree `.claude/worktrees/spec-external-yang` of the server checkout |
| Laptop | The work is on the laptop's `main`, carried by hand and signed by the owner: `9b45d7f964` (Python example), `c12ae908a5` (phase 5), `c7f081849d` (phase 6 with the owner's ruling) |
| Per-spec state file | `tmp/session/2026-11-15-231106/state/session-state-plugin-external-yang-at-startup-99a3a802-6ea0-45ad-a7f7-a342c1f9baae.md`, untracked, server worktree only: each phase's handoff with file digests |
| Implementation model | The owner asked for Fable for implementation agents |

## What works, and how it was seen

| Behaviour | Evidence |
|-----------|----------|
| A config holding an external plugin's own section loads at `ze start`; the plugin gets it at Stage 2 | Live with `examples/plugin/python/acme_monitor.py`; `test/plugin/external-yang-own-root.ci` |
| Ze enforces the plugin's module (range, mandatory leaf) | Live (interval 11 refused); unit test at the config load; `external-yang-boot-accepts-what-offline-refuses.ci` |
| A commit on a running Ze reaches verify then apply; a verify refusal fails the reload | Live with the Python example over SIGHUP; `external-yang-commit.ci` |
| With no daemon, Ze starts no external plugin and refuses its section as not available | Live behind a marker-writing wrapper; `external-yang-offline-not-available.ci` |
| Failure paths reach `ze doctor` | Stop record for a plugin that never declares; event `doctor-plugin-stop-unfinished` |

## The owner's rulings, not to be re-opened

1. The load order of `docs/architecture/hub-architecture.md`: declarations
   first (the `plugin` block and the command line), run the declared external
   plugins, Stage 1 brings the module, then the full parse.
2. A plugin, its declaration and its module change only when `ze` restarts.
3. "If a user use an external plugin the security is on him."
4. Given 2026-11-16, verbatim: "plugin command requiring external plugins which
   can not be started from the code when no daemon is running to pass it to are
   not available, for command built-in then yes you should run them as we can
   look them up in the yang hardcoded in the binary". With no daemon running,
   Ze starts no external plugin's program. This reversed the spec's earlier
   decision that offline commands read the declaration through declare mode.
5. He agreed that `show plugin declarations config <file>` follows ruling 4,
   and to the order of work below.

## Outstanding items, every one

| # | Item | Detail |
|---|------|--------|
| 1 | Local command to a running daemon | Ruling 4 says "when no daemon is running to pass it to". `ze config validate` never asks the daemon today, so with Ze running the operator still reads "not available" and must open `ze cli` and type `show config validation <file>`. A design is owed to the owner before any code: how the local command finds the daemon, which files the daemon may read. Research was started in the previous session |
| 2 | `show plugin declarations config <file>` | It still runs a config's programs with no daemon. To follow ruling 4. In progress when this was written: check the branch tip |
| 3 | Phase 7, restart rule | A reload whose plugin declarations differ from the published set is refused, naming the restart. Boot must record which plugins declared, so a daemon stops naming every schema-less plugin beside an unknown name |
| 4 | Phase 8, surfaces and doctor | The schema CLI (`getExternalPluginYANG`), the editor's completion (draft `test/draft/editor/external-yang-completion.et`), the three codes `doctor-plugin-schema-undeclared`, `-refused`, `-changed` |
| 5 | Phase 9, measure | Spec Q10. On a host whose performance is known: the owner's Mac, not the server |
| 6 | Drafts still red | `test/draft/plugin/external-yang-query-failed`, `-invalid-refused`, `-clash-refused` (each waits on a schema doctor code), `-declaration-changed` (phase 7), `test/draft/reload/external-yang-reload-refused` |
| 7 | Laptop: `ze config diff`, `dump`, `graph` | Left out of `c7f081849d`: another session was restructuring them, uncommitted, around a supplied-content route. They start no program and refuse an external section as a plain unknown keyword. Add the shared step's call once that work is committed |
| 8 | Laptop: tests not run | `internal/component/plugin/server` did not compile there (another session's request-cancellation edit), so only config, plugin and test/externalyang were tested on the laptop. `cmd/ze` did not build there either |
| 9 | Lint | Not run on phase 6 or on the ruling's commit. Phase 5's two findings were corrected and not re-linted |
| 10 | `TestNoPythonLeRemains` | Refuses the tracked `.py` files of `examples/plugin/python/`. An exemption for that directory needs the test judge. In progress when this was written |
| 11 | Phase 5 leftovers | The early start precedes the `environment` block being applied (spec R-15); `judgeConfigBeforeBoot` and the two `hub.ConfigRefused` closures have no unit test; a held process the manager removed is no longer joined by a later stop |
| 12 | Pushed config | An invalid pushed config that declares an external plugin now ends the process after restoring the previous config. Told to the owner; he did not object |
| 13 | Conformance checker | Owner decision C-1 of the earlier handover is still open: may the daemon run a program a remote caller names. Until then `show plugin conformance run` is local-only and `TestLocalCommandDaemonMatchesLocal` is red for it in `cmd/ze/hub` |
| 14 | Python library | `rollback` never ran against a live Ze. `ze_plugin.py` is 847 lines |
| 15 | Owner decision D-3 | Whether a plugin's own command argument model is part of this spec. Open; blocks nothing before the last phase |

## Added 2026-11-17: `ze config validate` asks a running ze, and the owner's decision on it

Since this handover was first written:

| Commit | Where | Content |
|--------|-------|---------|
| `33e690ac64` | Server branch; on the laptop as `d4fb5888c7` | `show plugin declarations config <file>` starts no program with no daemon (item 2 done); the Python example is exempt from `TestNoPythonLeRemains` by an exact two-file list (item 10 done, the test stays red on three tests of other work); offline and boot functional tests in their final form |
| `a0656f1c59` | Server branch only, HELD | `ze config validate` asks a running ze first (spec phase 10, item 1); the `ze:keyword-only` argument statement; SSH exec keeps the payload of a refused command |

`a0656f1c59` is proven on the server (unit tests of twelve packages, three
functional tests, live with the Python example). It is not carried to the
laptop, because its daemon side duplicates work another session is doing there.

**Owner decision, 2026-11-17: "yes, keep theirs and point your client at it".**
Another session on the laptop is building, uncommitted when this was written,
`show config supplied validation|dump|fmt|graph|diff`: the caller sends a
config's bytes and the daemon judges them with the existing producers. That is
the daemon side. From `a0656f1c59` only the client side is kept.

What the switch is, to do once that work is committed on the laptop:

| Step | Detail |
|------|--------|
| Remove | The `content` leaf added to `show config validation` (`ze-config-cli-cmd.yang`) and the content read in `dataValidate` (`config_data.go`), with `TestShowConfigValidationJudgesContent` |
| Point the client | `judgeConfig` (`config/cli/validate_route.go`) sends `show config supplied validation content <base64> name <label>` in place of `show config validation file <name> content <base64>` |
| Their contract, read on the laptop | `content` is mandatory, 4 to 261,120 padded base64 characters (195,840 bytes), same pattern as the existing content leaves. `name` is optional, a filename label with no directory and no control character, default `supplied.conf`; the client passes the base name of the operator's file so diagnostics name it. The handler is `validationBytesAnswer(data, name)`, the producer of the file form, so the answer keeps the `ValidateResult` shape the client draws. It refuses positional words itself, and declares its content argument through `registry.Meta.SuppliedInput` |
| Check before relying on it | That the supplied validation, inside a daemon, judges with the module set boot published (an external plugin's section accepted). The functional test `external-yang-validate-asks-running-ze` proves it end to end once pointed at their command |
| Keep | `sshclient.ReachDaemon` and doctor's use of it, the refusal texts (`DaemonUnanswered`, `Unanswered`), `ze config validate --user`, the SSH payload fix with its unit tests, `ze:keyword-only` on the two `update data` content leaves. Consider declaring `ze:keyword-only` on their `supplied-content` grouping with that session's agreement |
| Then | The same client route for `fmt`, `dump`, `diff` and `graph` uses their four other supplied commands: no new daemon side is needed |
| Where | On the laptop, in a detached worktree at the commit that holds their work, not in the shared checkout; the server cannot test it until that commit reaches it |

The spec's phase 10 text still describes the `content` leaf on
`show config validation`: correct it in the same work as the switch.

Tooling notes from this round: an apply to the laptop's working tree was denied
to the agent by the permission system once; the owner ran the apply script
himself. A page edited by another session between assessment and apply made the
plain patch refuse; the apply script now merges such a file three-way and
refuses before writing anything on a real conflict.

## Added 2026-11-17, later: the switch and phase 7 are done and on the laptop

This section replaces what the one above says is pending.

| Piece | Laptop `main` | Server |
|-------|---------------|--------|
| `ze config validate` asks a running ze first, through `show config supplied validation` (item 1, spec phase 10) | `552c08a28a` | `09ad03b19d` on `worktree-validate-client` |
| Phase 7, the restart rule (item 3) | `2210546edb` | `350a334e1c` on `worktree-validate-client`; first built as `b4a5be2c4c` on `worktree-spec-external-yang` |

The session doing `show config supplied` (Codex) confirmed its contract in
writing, said the command, arguments and answer shape will not change, and
took the `diff`, `dump` and `graph` gap itself (item 7): it corrects those
producers to read `config.SchemaWithDeclaredPlugins`.

**Where to work from now.** The laptop's `main` is the truth. The server branch
`worktree-spec-external-yang` has diverged from it: it still holds `a0656f1c59`
with the duplicate `content` leaf, and none of the laptop's other work. Build
further phases in `.claude/worktrees/validate-client` (branch
`worktree-validate-client`), or in a fresh worktree made the same way: on the
laptop, `git bundle create` of `main` from the newest commit the server holds,
`scp` to the server, `git fetch` the bundle there, `git worktree add -b <branch>
<path> FETCH_HEAD`. Run every `./le` there with the working directory set to
that worktree (`env -C <worktree> ./le ...`): invoked by absolute path from
another worktree, the launcher tests the directory it was called from.

**What is left of the spec:** phase 8 (the schema CLI `getExternalPluginYANG`,
the editor's completion with the draft `test/draft/editor/external-yang-completion.et`,
the codes `doctor-plugin-schema-undeclared` and `-refused` that the drafts
`external-yang-query-failed`, `-invalid-refused` and `-clash-refused` wait for;
`doctor-plugin-schema-changed` exists since phase 7), phase 9 (the measurement,
on the owner's Mac), and the same client route for `fmt`, `dump`, `diff` and
`graph` through their supplied commands. Those drafts are git-ignored and exist
only in the server worktree `spec-external-yang`.

**For the owner to pass to the supplied-config session:** its `supplied-content`
grouping does not declare `ze:keyword-only` (added by `552c08a28a`), so a stray
word that is valid base64 binds to `content` before the handler's check of
positional words.

**Reds seen on the laptop's history at `0402601f9d`, not from this work:** in
`plugin/server`, `TestRecognizesCommandUsesDispatchRegistrations`,
`TestRecognizesCommandTracksHiddenPluginsAndSubsystems`,
`TestRelayStoredRouteFromAnUnnamedCallerStaysUnnamed`,
`TestRelayStoredRouteJSONTransport`,
`TestHandleDispatchCommandRPCPreservesPluginIdentity`; in `cmd/ze/hub`, the
`TestGraphicalImport` tests, `TestWebCLIViewActionRouteRefusesUnknownHandle`,
and `TestLocalCommandDaemonMatchesLocal` on the supplied commands (a help hint
printed locally and omitted by the daemon route; a later commit on `main`
appears to fix it); in `config/cli`, the config sweep on 28 configs refused as
`config-bgp-peer` "incomplete destination grants"; in `test/reload`,
`bgp-attach-no-route-refresh-restarts` and `bgp-attach-sent-direction-restarts`
(their reload completes, then a `changed=ProcessBindings` line is missing).

**This server's disk stalls.** A live reload on a disk-backed directory was
seen waiting on the store's boot record and on a file write barrier; on a
memory-backed directory (`/dev/shm`) the same reload completes at once. Run
live daemon checks there. One hub test,
`TestInfraSSHOwnerDrainsBeforeStoreCloseOnDaemonShutdown`, is intermittent on
this host.

## Added 2026-11-17, phase 8: surfaces and doctor, written and not run

Built in the worktree `.claude/worktrees/yang-phase8` (branch
`worktree-yang-phase8`, from the laptop's `main` at `2210546edb`). Nothing was
run: the supervising session runs the owed commands. The spec's step 8 holds
the full account.

| Piece | State |
|-------|-------|
| Schema CLI | `getExternalPluginYANG` deleted. The daemon's schema commands answer an external module from the published set; with no daemon `ze schema` answers compiled-in modules and says the external module is not available, fix `ze cli` |
| Doctor codes | `doctor-plugin-schema-undeclared` and `doctor-plugin-schema-refused` declared, registered and raised by boot through `pluginserver.ReportUndeclaredSchema` and `ReportRefusedSchema` |
| Claims check | Counts the roots each running external plugin declared (`pluginserver.ExternalConfigRoots`) |
| Editor | No product change needed; a Go test proves completion from the published set |
| Drafts | `external-yang-query-failed`, `-invalid-refused`, `-clash-refused` corrected to the current texts; `external-yang-show-config` new |

What is left of phase 8, for its continuation:

| # | Item | Detail |
|---|------|--------|
| 1 | Three tracked tests no longer compile | `TestGetExternalPluginYANG`, `TestExternalPluginEmptySpec` and `TestExternalPluginNonexistent` (`internal/component/config/schema/cli/main_test.go`) call the deleted run of a program with `--yang`. Their removal needs the test judge; until then the package's tests do not build |
| 2 | The editor's functional test | `test/draft/editor/external-yang-completion.et` cannot run: the `.et` runner has no daemon and starts no plugin, so it never holds the module. It needs an `.et` option that publishes a fixture module, or a driver that types into the daemon's SSH session |
| 3 | `show schema`, `show yang` and save in the daemon | No functional test. `ze cli -c` answers these local-data commands in its own process, so the test needs a caller that dispatches through the daemon |
| 4 | AC-6 and AC-7 halves | "The internal owner of `service` keeps working" and "the config under its root stays loaded" have no functional test yet |
| 5 | A refused module after the failed start | Nothing reports it again: the daemon is not running, and a local `ze doctor` starts no plugin. Read on the refusal and in the log only. Whether a record that survives the failed start is wanted is the owner's to say |
| 6 | A hook refusal met | The edit hook refused a change to the `switch` in `Run` of the schema CLI ("switch-based command dispatch"). The switch is unchanged; the notice is written where the commands build their registry (`commandLineRegistry`) |

## Added 2026-11-17, phase 8 after its first test run

The table of leftovers above is superseded by this section. Nothing here is
committed; the supervising session runs the owed commands and commits.

What landed in the worktree, and where each piece is seen:

| Piece | Where it is observable |
|-------|------------------------|
| Schema codes after a failed start | Boot writes the refusal to the last-stop record (`RecordRefusedSchemaStop`, `RecordUndeclaredSchemaStop`). The local `ze doctor` reports it under `doctor-plugin-schema-refused` or `doctor-plugin-schema-undeclared` until ze next starts cleanly. Drafts `external-yang-refused-doctor` (green), `external-yang-undeclared-doctor` |
| Empty `--plugin` | `ze schema` refuses it with exit 1, fix `ze show plugin list` |
| `ze schema` with an external plugin and no daemon | Answers compiled-in modules, exit 0, and warns on stderr. The why now names the two commands a running ze has, typed in a `ze cli` session: `show schema list` lists the module, `show yang tree config` shows its nodes |
| The daemon's schema commands | `show schema list` and `show yang tree config` hold the external module. The daemon has no `show schema show <module>` and no command that prints a module's YANG text. Draft `test/draft/ui/external-yang-daemon-schema.ci` |
| The daemon's editor | Offers the external root and its leaves, shows a leaf's description on `?` after Tab, refuses a value outside the range with the range. Draft `test/draft/ui/external-yang-session-editor.ci`, which supersedes the `.et` draft |
| The owner of a shared container | `show dns resolver` holds the listener the config gives it beside the external augment. Draft `test/draft/plugin/external-yang-augment-owner-keeps-section.ci` |

Green in the first run: unit packages `core/diagnostic` and `plugin/doctor`;
drafts `external-yang-query-failed`, `-invalid-refused`, `-clash-refused`,
`-refused-doctor`, `-show-config`, `-changed-keeps-config`. Red and corrected,
not run again: the schema CLI package (a helper declared twice), the two UI
drafts above.

What is left:

| # | Item | Detail |
|---|------|--------|
| 1 | Run and promote the drafts | The two UI drafts, the owner-keeps-section draft and `external-yang-undeclared-doctor` after their next run; each promotion follows the stress run of the `ze-test` skill |
| 2 | AC-13, save | No functional test: a commit typed in the daemon's session, the stored config holding the external block, and a second start that parses it. The session driver (`sessionScreenAfterKeys`) can type it; the commit grammar in the daemon's window and the second start were not written |
| 3 | The vacuous exit assertion | `await=stderr` beside `expect=exit:code=` compares the peers' wait, never the daemon's exit (journal `green-that-could-not-have-been-red.md`, 2026-11-17). Drafts of `external-yang-own-root`, `-command-line` and `-augment-shared-root` add `then=stop`; each replaces a tracked test, so each needs the test judge. `external-yang-boot-accepts-what-offline-refuses.ci` holds three commands and is not converted, and five tracked tests of other work hold the same pair |
| 4 | Phase 9, measure | Spec Q10, on a host whose performance is known |
| 5 | `ze config fmt`, `dump`, `diff`, `graph` | The client route that sends these commands to a running ze, as `ze config validate` has since phase 10, is not built. Another session was working on `cmd_diff.go`, `cmd_dump.go`, `cmd_graph.go` and `config_supplied.go`; this phase did not touch them |
| 6 | A page that disagrees with the code | `docs/guide/command-reference.md`, near "Command help at the prompt", says `ze cli` with no command runs the interactive model in the client process. `runInteractiveSession` opens the daemon's session in command mode (`sshclient.RunInteractive`). Not corrected here: it is about pipe aliases, outside this phase's files |

## Reds that predate this work

In `./internal/le/`: `TestNoPythonLeRemains`, `TestCensusActionsDeclareHeavy`,
`TestEveryRegisteredVerbIsInTheVocabulary`, `TestNoNameEmbedsAVerbOrTest`,
`TestHelpAiVerbsRendersTheVocabulary`,
`TestNoAgentSurfaceTellsAnAgentToRunAWholeTreeVerify`. In `./cmd/ze/hub/`:
`TestBuildTag_Gate12_AbsentBinaryDropsSymbols` (an L2TP symbol in the bare
`ze_core` binary).

## Carrying work to the laptop

A commit cannot be created on the laptop from a remote pane: signing fails
there (`gpg failed to sign the data`). The routine that worked twice: fetch the
branch on the laptop, apply the change to the working tree with a script the
laptop pulls from the server, test what builds, prepare the commit with
`./le commit create` (files with other sessions' uncommitted edits go in as
`hunks`), and give the owner the one `bash tmp/commit-...sh` line to run. A
plain cherry-pick refuses because the laptop's `main` holds hundreds of
uncommitted files of other sessions. Before applying, check that no file the
change touches is being restructured, uncommitted, by another session: one
apply broke that session's build for a few minutes and had to be reversed.

## Tooling traps met

| Trap | Effect |
|------|--------|
| The functional runner's `-pattern` matches a test's path, and this worktree's directory is named `spec-external-yang` | `-pattern external-yang` runs all 1205 plugin tests. Use a full test name or the numeric ids from `-list` |
| `./ze` keeps the binary it built once | Remove `bin/ze` to rebuild from the working tree |
| The built `ze` binary was damaged on disk once after running correctly | One 4 MiB block differed from a rebuild; a rebuild works. The crash left `crash/.pending/` files: never build a commit's file list from `git status` without reading it |
| `git status --short` prints the branch header on these machines | Use `--porcelain` in scripts |
| An edit is refused until the main checkout's style guide and doors were read | Tell every agent to read the main-checkout copies first |
| The test judge records its own approvals | `./le test weakened approve` runs from the judge agent; the main thread never records a row for it |
